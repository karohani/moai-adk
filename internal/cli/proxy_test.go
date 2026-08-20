package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modu-ai/moai-adk/internal/config"
	"github.com/modu-ai/moai-adk/internal/proxy"
)

// TestProxyCmd_RegistersGroupAndSetFlags verifies the M4 CLI wiring
// (plan.md M4 item 1): `moai proxy` registers -g/--group and --set,
// matching the flag names the already-implemented M2 ResolveActiveGroups
// wires onto.
func TestProxyCmd_RegistersGroupAndSetFlags(t *testing.T) {
	groupFlag := proxyCmd.Flags().Lookup("group")
	if groupFlag == nil {
		t.Fatal("expected a --group flag")
	}
	if groupFlag.Shorthand != "g" {
		t.Errorf("--group shorthand = %q, want g", groupFlag.Shorthand)
	}
	setFlag := proxyCmd.Flags().Lookup("set")
	if setFlag == nil {
		t.Fatal("expected a --set flag")
	}
}

// TestProxyCmd_RegisteredInLaunchGroup verifies the M4 cobra registration
// pattern (plan.md M4 item 1): GroupID "launch", matching the sibling
// launch-group commands (cc, cg, glm).
func TestProxyCmd_RegisteredInLaunchGroup(t *testing.T) {
	if proxyCmd.GroupID != "launch" {
		t.Errorf("GroupID = %q, want launch", proxyCmd.GroupID)
	}
}

// TestResolveProxyStateDir_DefaultsToDaemonDefault verifies the state-dir
// resolver falls back to proxy.DefaultDaemonStateDir() when no override is
// set.
func TestResolveProxyStateDir_DefaultsToDaemonDefault(t *testing.T) {
	t.Setenv(config.EnvProxyStateDir, "")
	got, err := resolveProxyStateDir()
	if err != nil {
		t.Fatalf("resolveProxyStateDir() error = %v", err)
	}
	want, err := proxy.DefaultDaemonStateDir()
	if err != nil {
		t.Fatalf("proxy.DefaultDaemonStateDir() error = %v", err)
	}
	if got != want {
		t.Errorf("resolveProxyStateDir() = %q, want %q", got, want)
	}
}

// TestResolveProxyStateDir_EnvOverride verifies MOAI_PROXY_STATE_DIR
// overrides the default.
func TestResolveProxyStateDir_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvProxyStateDir, dir)
	got, err := resolveProxyStateDir()
	if err != nil {
		t.Fatalf("resolveProxyStateDir() error = %v", err)
	}
	if got != dir {
		t.Errorf("resolveProxyStateDir() = %q, want override %q", got, dir)
	}
}

// TestResolveProxyRegistryPath_DefaultsToRegistryDefault verifies the
// registry-path resolver falls back to proxy.DefaultRegistryPath() when no
// override is set.
func TestResolveProxyRegistryPath_DefaultsToRegistryDefault(t *testing.T) {
	t.Setenv(config.EnvProxyRegistryPath, "")
	got, err := resolveProxyRegistryPath()
	if err != nil {
		t.Fatalf("resolveProxyRegistryPath() error = %v", err)
	}
	want, err := proxy.DefaultRegistryPath()
	if err != nil {
		t.Fatalf("proxy.DefaultRegistryPath() error = %v", err)
	}
	if got != want {
		t.Errorf("resolveProxyRegistryPath() = %q, want %q", got, want)
	}
}

// TestResolveProxyRegistryPath_EnvOverride verifies
// MOAI_PROXY_REGISTRY_PATH overrides the default.
func TestResolveProxyRegistryPath_EnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-groups.yaml")
	t.Setenv(config.EnvProxyRegistryPath, path)
	got, err := resolveProxyRegistryPath()
	if err != nil {
		t.Fatalf("resolveProxyRegistryPath() error = %v", err)
	}
	if got != path {
		t.Errorf("resolveProxyRegistryPath() = %q, want override %q", got, path)
	}
}

// TestResolveProxyActiveGroups_WiresGFlagThroughToResolveActiveGroups
// verifies the CLI-layer wrapper delegates -g to the ALREADY-IMPLEMENTED M2
// proxy.ResolveActiveGroups (no reimplementation).
func TestResolveProxyActiveGroups_WiresGFlagThroughToResolveActiveGroups(t *testing.T) {
	reg := &proxy.Registry{
		Groups: map[string]proxy.Group{
			"a": {Type: proxy.GroupTypeLiteLLM, BaseURL: "http://x"},
			"b": {Type: proxy.GroupTypeCodex},
		},
	}
	got, err := resolveProxyActiveGroups(reg, []string{"a"}, "", t.TempDir())
	if err != nil {
		t.Fatalf("resolveProxyActiveGroups() error = %v", err)
	}
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("resolveProxyActiveGroups() = %v, want [a]", got)
	}
}

// TestResolveProxyActiveGroups_MutualExclusionPropagates verifies the
// -g/--set mutual-exclusion error (REQ-PROXY-011, M2) surfaces through the
// CLI wrapper unchanged.
func TestResolveProxyActiveGroups_MutualExclusionPropagates(t *testing.T) {
	reg := &proxy.Registry{Groups: map[string]proxy.Group{}}
	_, err := resolveProxyActiveGroups(reg, []string{"a"}, "myset", t.TempDir())
	if err == nil {
		t.Fatal("expected mutual-exclusion error, got nil")
	}
}

// TestResolveProxyActiveGroups_NoProjectConfigFallsBackGracefully verifies
// a project directory with NO .moai/config present does not error — it
// simply carries no default_set pointer, falling through to the machine
// registry default (or a hard error from THAT layer, unrelated to config
// absence).
func TestResolveProxyActiveGroups_NoProjectConfigFallsBackGracefully(t *testing.T) {
	reg := &proxy.Registry{
		Groups: map[string]proxy.Group{"only": {Type: proxy.GroupTypeLiteLLM, BaseURL: "http://x"}},
		Sets:   map[string][]string{"default": {"only"}},
	}
	got, err := resolveProxyActiveGroups(reg, nil, "", t.TempDir())
	if err != nil {
		t.Fatalf("resolveProxyActiveGroups() error = %v", err)
	}
	if len(got) != 1 || got[0] != "only" {
		t.Errorf("resolveProxyActiveGroups() = %v, want [only] via machine default", got)
	}
}

// TestResolveProxyActiveGroups_ProjectDefaultSetPointerWins verifies a
// real project config's llm.proxy.default_set pointer is read and honored.
func TestResolveProxyActiveGroups_ProjectDefaultSetPointerWins(t *testing.T) {
	reg := &proxy.Registry{
		Groups: map[string]proxy.Group{
			"a": {Type: proxy.GroupTypeLiteLLM, BaseURL: "http://a"},
			"b": {Type: proxy.GroupTypeLiteLLM, BaseURL: "http://b"},
		},
		Sets: map[string][]string{
			"default": {"a"},
			"heavy":   {"a", "b"},
		},
	}
	projectRoot := t.TempDir()
	cfgDir := filepath.Join(projectRoot, ".moai", "config", "sections")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	llmYAML := "llm:\n  proxy:\n    default_set: heavy\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "llm.yaml"), []byte(llmYAML), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := resolveProxyActiveGroups(reg, nil, "", projectRoot)
	if err != nil {
		t.Fatalf("resolveProxyActiveGroups() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("resolveProxyActiveGroups() = %v, want the 2-group 'heavy' set from the project pointer", got)
	}
}

// TestFirstBedrockGroup_FindsABedrockGroup verifies the pure
// bedrock-group-selection helper picks a bedrock-typed group when one
// exists.
func TestFirstBedrockGroup_FindsABedrockGroup(t *testing.T) {
	reg := &proxy.Registry{
		Groups: map[string]proxy.Group{
			"llm": {Type: proxy.GroupTypeLiteLLM, BaseURL: "http://x"},
			"br":  {Type: proxy.GroupTypeBedrock, Region: "us-east-1"},
		},
	}
	g, name, ok := firstBedrockGroup(reg)
	if !ok {
		t.Fatal("expected a bedrock group to be found")
	}
	if name != "br" || g.Region != "us-east-1" {
		t.Errorf("firstBedrockGroup() = %+v/%q, want br/us-east-1", g, name)
	}
}

// TestFirstBedrockGroup_NoneRegisteredReturnsFalse verifies the helper
// reports absence cleanly when no bedrock group is registered.
func TestFirstBedrockGroup_NoneRegisteredReturnsFalse(t *testing.T) {
	reg := &proxy.Registry{
		Groups: map[string]proxy.Group{
			"llm": {Type: proxy.GroupTypeLiteLLM, BaseURL: "http://x"},
		},
	}
	_, _, ok := firstBedrockGroup(reg)
	if ok {
		t.Error("expected ok=false when no bedrock group is registered")
	}
}

// TestProxyExitError_ErrorAndExitCode verifies the exit-code-propagation
// wrapper (internal/cli/CLAUDE.md exit-code discipline: propagate the
// child claude process's real exit code via the main.go ExitCoder
// mechanism, not a flattened 1).
func TestProxyExitError_ErrorAndExitCode(t *testing.T) {
	err := &proxyExitError{code: 7}
	if err.ExitCode() != 7 {
		t.Errorf("ExitCode() = %d, want 7", err.ExitCode())
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("Error() = %q, want it to mention the exit code", err.Error())
	}
}

// TestRunProxy_RegistryLoadFailureReturnsErrorBeforeLaunchingClaude
// verifies runProxy fails fast on a malformed machine registry — it never
// reaches the daemon-acquire or claude-launch steps (which this test
// environment cannot exercise: CLAUDE.local.md §13 prohibits running
// launch-command integration flows, real daemon binds, or real `claude`
// child processes in the dev project).
func TestRunProxy_RegistryLoadFailureReturnsErrorBeforeLaunchingClaude(t *testing.T) {
	dir := t.TempDir()
	badRegistry := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badRegistry, []byte("{not: valid: yaml:"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv(config.EnvProxyRegistryPath, badRegistry)

	err := runProxy(proxyCmd, nil)
	if err == nil {
		t.Fatal("expected an error from a malformed machine registry, got nil")
	}
	if !strings.Contains(err.Error(), "moai proxy") {
		t.Errorf("error = %v, want it prefixed with the moai proxy: context", err)
	}
}

// TestProxyCmd_NoAskUserQuestion is the canonical static guard
// (internal/cli/CLAUDE.md § Subagent boundary): CLI code MUST NOT call
// AskUserQuestion or mcp__askuser__*.
func TestProxyCmd_NoAskUserQuestion(t *testing.T) {
	data, err := os.ReadFile("proxy.go")
	if err != nil {
		t.Fatalf("ReadFile(proxy.go) error = %v", err)
	}
	content := string(data)
	for _, forbidden := range []string{"AskUserQuestion", "mcp__askuser"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("proxy.go contains %q — CLI code must not call the user-question channel", forbidden)
		}
	}
}
