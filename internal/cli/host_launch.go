package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/modu-ai/moai-adk/internal/agenthost"
)

func init() {
	rootCmd.AddCommand(newCodexCmd(), newOpenCodeCmd())
}

func newCodexCmd() *cobra.Command {
	var flags hostLaunchFlags
	cmd := &cobra.Command{
		Use:     "codex [prompt...]",
		Short:   "Launch Codex with native Codex CLI flags",
		GroupID: "launch",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRootFn()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}
			req := agenthost.LaunchRequest{
				Host:        agenthost.HostCodex,
				ProjectRoot: root,
				Mode:        launchMode(flags.Exec),
				Model:       flags.Model,
				Profile:     flags.Profile,
				Sandbox:     flags.Sandbox,
				Approval:    flags.Approval,
				Config:      flags.Config,
				Prompt:      strings.Join(args, " "),
			}
			return runHostLaunch(cmd, req, flags.DryRun)
		},
	}
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, "Print the host command without launching")
	cmd.Flags().BoolVar(&flags.Exec, "exec", false, "Use non-interactive execution mode")
	cmd.Flags().StringVarP(&flags.Model, "model", "m", "", "Override model")
	cmd.Flags().StringVarP(&flags.Profile, "profile", "p", "", "Use Codex profile")
	cmd.Flags().StringVar(&flags.Sandbox, "sandbox", "", "Codex sandbox policy")
	cmd.Flags().StringVar(&flags.Approval, "ask-for-approval", "", "Codex approval policy")
	cmd.Flags().StringVar(&flags.Config, "config", "", "Codex config override")
	return cmd
}

func newOpenCodeCmd() *cobra.Command {
	var flags hostLaunchFlags
	cmd := &cobra.Command{
		Use:     "opencode [prompt...]",
		Short:   "Launch OpenCode with native OpenCode CLI flags",
		GroupID: "launch",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRootFn()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}
			req := agenthost.LaunchRequest{
				Host:        agenthost.HostOpenCode,
				ProjectRoot: root,
				Mode:        launchMode(flags.Exec),
				Role:        flags.Role,
				Agent:       flags.Agent,
				Model:       flags.Model,
				Session:     flags.Session,
				Continue:    flags.Continue,
				Attach:      flags.Attach,
				Auto:        flags.Auto,
				Prompt:      strings.Join(args, " "),
			}
			return runHostLaunch(cmd, req, flags.DryRun)
		},
	}
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, "Print the host command without launching")
	cmd.Flags().BoolVar(&flags.Exec, "exec", false, "Use opencode run")
	cmd.Flags().StringVar(&flags.Agent, "agent", "", "OpenCode agent")
	cmd.Flags().StringVar(&flags.Role, "role", "", "MoAI role name to map to an OpenCode agent")
	cmd.Flags().StringVarP(&flags.Model, "model", "m", "", "Override model")
	cmd.Flags().StringVar(&flags.Session, "session", "", "OpenCode session id")
	cmd.Flags().BoolVar(&flags.Continue, "continue", false, "Continue the last OpenCode session")
	cmd.Flags().StringVar(&flags.Attach, "attach", "", "Attach to a running OpenCode server URL (e.g. http://localhost:4096)")
	cmd.Flags().BoolVar(&flags.Auto, "auto", false, "Enable OpenCode auto mode")
	return cmd
}

type hostLaunchFlags struct {
	DryRun   bool
	Exec     bool
	Model    string
	Profile  string
	Sandbox  string
	Approval string
	Config   string
	Agent    string
	Role     string
	Session  string
	Continue bool
	// Attach는 OpenCode 서버 URL 문자열이다 (bare boolean 플래그가 아님).
	Attach string
	Auto   bool
}

func launchMode(execMode bool) agenthost.LaunchMode {
	if execMode {
		return agenthost.LaunchExec
	}
	return agenthost.LaunchInteractive
}

func runHostLaunch(cmd *cobra.Command, req agenthost.LaunchRequest, dryRun bool) error {
	launch, err := agenthost.BuildLaunchCommand(req)
	if err != nil {
		return err
	}
	if dryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] %s\n", launch.String())
		return nil
	}
	bin, err := exec.LookPath(launch.Argv[0])
	if err != nil {
		return fmt.Errorf("%s not found in PATH. Install and authenticate %s first", launch.Argv[0], launch.Host)
	}
	return execOrSpawnHost(bin, launch.Argv, os.Environ(), launch.Cwd)
}
