package cli

// @MX:NOTE: [AUTO] moai proxy launches Claude Code through the local
// multi-backend LLM gateway (SPEC-PROXY-001 M4: CLI wiring). Unlike the
// other launch-group commands (cc/cg/glm), this command does NOT
// syscall.Exec-replace the current process — the daemon reference count
// (proxy.Daemon.Acquire/Release, M2) must be released after Claude Code
// exits, which process replacement makes impossible. It spawns `claude`
// as a child process, waits, and releases the daemon reference on exit.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/modu-ai/moai-adk/internal/config"
	"github.com/modu-ai/moai-adk/internal/proxy"
)

var (
	proxyGroupFlags []string
	proxySetFlag    string
)

var proxyCmd = &cobra.Command{
	Use:   "proxy [-g group[,group...]] [--set name] [-- claude-args...]",
	Short: "Launch Claude Code through the local moai proxy gateway",
	Long: `Launch Claude Code routed through moai proxy — a local, multi-backend
LLM gateway (SPEC-PROXY-001). One daemon serves every project on this
machine (reference-counted; the first invocation starts it, the last exit
stops it).

This command:
  1. Loads the machine-scope group registry (~/.moai/config/proxy-groups.yaml)
  2. Resolves the active group set: -g/--set, else the project's
     llm.proxy.default_set pointer, else the machine registry's default set
  3. Acquires the shared daemon (starts it if not already running)
  4. Launches Claude Code as a child process with ANTHROPIC_BASE_URL
     pointed at the daemon
  5. On exit, releases the daemon reference (self-terminates at refcount 0)

Flags:
  -g, --group <name>[,<name>...]   Activate specific registry group(s) by name
  --set <name>                     Activate a named group set from the machine registry
  (neither flag)                   Falls back to the project pointer, then the machine default

-g and --set are mutually exclusive.

Examples:
  moai proxy                        # Use the resolved default group set
  moai proxy -g work                # Activate only the 'work' group
  moai proxy --set heavy            # Activate the 'heavy' named set
  moai proxy -- --print              # Pass-through args to claude`,
	GroupID: "launch",
	RunE:    runProxy,
}

func init() {
	proxyCmd.Flags().StringSliceVarP(&proxyGroupFlags, "group", "g", nil,
		"Activate specific registry group(s) by name (comma-separated). Mutually exclusive with --set.")
	proxyCmd.Flags().StringVar(&proxySetFlag, "set", "",
		"Activate a named group set from the machine registry. Mutually exclusive with -g.")
	rootCmd.AddCommand(proxyCmd)
}

// proxyExitError carries a child process's exit code through cobra's error
// chain so main.go's ExitCoder handling propagates it verbatim (exit code
// discipline: internal/cli/CLAUDE.md).
type proxyExitError struct{ code int }

func (e *proxyExitError) Error() string { return fmt.Sprintf("claude exited with code %d", e.code) }
func (e *proxyExitError) ExitCode() int { return e.code }

// resolveProxyStateDir resolves the daemon's machine-scope runtime state
// directory: MOAI_PROXY_STATE_DIR overrides proxy.DefaultDaemonStateDir().
func resolveProxyStateDir() (string, error) {
	if override := os.Getenv(config.EnvProxyStateDir); override != "" {
		return override, nil
	}
	return proxy.DefaultDaemonStateDir()
}

// resolveProxyRegistryPath resolves the machine-scope group registry file
// path: MOAI_PROXY_REGISTRY_PATH overrides proxy.DefaultRegistryPath().
func resolveProxyRegistryPath() (string, error) {
	if override := os.Getenv(config.EnvProxyRegistryPath); override != "" {
		return override, nil
	}
	return proxy.DefaultRegistryPath()
}

// resolveProxyActiveGroups combines -g/--set with the project's
// llm.proxy.default_set pointer and delegates the actual resolution
// decision entirely to the already-implemented M2 proxy.ResolveActiveGroups
// — this function's only job is gathering the project pointer input, never
// reimplementing the resolution order or the mutual-exclusion check.
// A project directory carrying no .moai/config at all (config.Loader
// returns defaults, nil error) is not an error here — it simply carries an
// empty default_set pointer, which proxy.ResolveActiveGroups then falls
// through past to the machine registry default (or its own hard error).
func resolveProxyActiveGroups(reg *proxy.Registry, groups []string, set string, projectRoot string) ([]string, error) {
	var projectDefaultSet string
	if cfg, err := config.NewConfigManager().Load(projectRoot); err == nil && cfg != nil {
		projectDefaultSet = cfg.LLM.Proxy.DefaultSet
	}
	return proxy.ResolveActiveGroups(reg, groups, set, projectDefaultSet)
}

// firstBedrockGroup returns the first bedrock-typed group found in reg, if
// any. moai proxy's daemon constructs at most ONE shared BedrockInvoker
// (the region/profile of whichever bedrock group is found first) — a
// pre-existing simplification inherited from M2/M3's MessagesHandler
// design (a single BedrockInvoker field, not one per bedrock group); not
// addressed in M4, which is CLI wiring only. See progress.md §E.2 M4
// Residual risk.
func firstBedrockGroup(reg *proxy.Registry) (proxy.Group, string, bool) {
	for name, g := range reg.Groups {
		if g.Type == proxy.GroupTypeBedrock {
			return g, name, true
		}
	}
	return proxy.Group{}, "", false
}

// buildProxyBedrockInvoker constructs the daemon's shared BedrockInvoker
// from the first registered bedrock group, or returns (nil, nil) when no
// bedrock group is registered — nil is a valid "no bedrock configured"
// state per proxy.NewMessagesHandler's contract.
func buildProxyBedrockInvoker(ctx context.Context, reg *proxy.Registry) (proxy.BedrockInvoker, error) {
	g, name, ok := firstBedrockGroup(reg)
	if !ok {
		return nil, nil
	}
	inv, err := proxy.NewAWSBedrockInvoker(ctx, g.Region, g.Profile)
	if err != nil {
		return nil, fmt.Errorf("build bedrock invoker for group %q: %w", name, err)
	}
	return inv, nil
}

// runProxy is the RunE entry point. Standard cobra flag parsing (not
// DisableFlagParsing) handles -g/--group and --set; args carries only the
// remaining pass-through arguments (everything after a literal `--`, or
// any unrecognized positional args), forwarded to the claude child process
// verbatim.
func runProxy(cmd *cobra.Command, args []string) error {
	claudeArgs := args

	registryPath, err := resolveProxyRegistryPath()
	if err != nil {
		return fmt.Errorf("moai proxy: %w", err)
	}
	reg, err := proxy.LoadRegistry(registryPath)
	if err != nil {
		return fmt.Errorf("moai proxy: load machine registry %s: %w", registryPath, err)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("moai proxy: resolve working directory: %w", err)
	}

	activeGroups, err := resolveProxyActiveGroups(reg, proxyGroupFlags, proxySetFlag, projectRoot)
	if err != nil {
		return fmt.Errorf("moai proxy: %w", err)
	}

	stateDir, err := resolveProxyStateDir()
	if err != nil {
		return fmt.Errorf("moai proxy: %w", err)
	}

	ctx := context.Background()
	bedrockInvoker, err := buildProxyBedrockInvoker(ctx, reg)
	if err != nil {
		// A misconfigured bedrock group does not block the daemon from
		// starting for every OTHER group type — same isolation shape as
		// REQ-PROXY-021 — it just means bedrock-typed requests will fail
		// per-request instead of at startup.
		fmt.Fprintf(os.Stderr, "moai proxy: bedrock invoker unavailable, bedrock-typed groups will be inactive: %v\n", err)
		bedrockInvoker = nil
	}

	daemon := proxy.NewDaemon(stateDir)
	addr, err := daemon.Acquire(func() (string, func() error, error) {
		cat := proxy.NewCatalog(reg, activeGroups)
		handler, herr := proxy.NewMessagesHandler(reg, cat, bedrockInvoker)
		if herr != nil {
			return "", nil, herr
		}
		return proxy.StartServer(handler)
	})
	if err != nil {
		return fmt.Errorf("moai proxy: acquire daemon: %w", err)
	}

	fmt.Fprintf(os.Stderr, "moai proxy: daemon listening at %s (active groups: %v)\n", addr, activeGroups)

	defer func() {
		if _, rerr := daemon.Release(); rerr != nil {
			fmt.Fprintf(os.Stderr, "moai proxy: release daemon reference: %v\n", rerr)
		}
	}()

	child := exec.Command("claude", claudeArgs...)
	child.Env = append(os.Environ(), config.EnvAnthropicBaseURL+"=http://"+addr)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	runErr := child.Run()
	if runErr == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return &proxyExitError{code: exitErr.ExitCode()}
	}
	return fmt.Errorf("moai proxy: launch claude: %w", runErr)
}
