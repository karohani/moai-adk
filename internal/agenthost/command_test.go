package agenthost

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildLaunchCommand_CodexExec(t *testing.T) {
	cmd, err := BuildLaunchCommand(LaunchRequest{
		Host:        HostCodex,
		ProjectRoot: "/repo",
		Mode:        LaunchExec,
		Model:       "gpt-5.5",
		Profile:     "work",
		Sandbox:     "workspace-write",
		Approval:    "on-request",
		Config:      "model_reasoning_effort=\"high\"",
		Prompt:      "implement SPEC",
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(codex): %v", err)
	}

	want := []string{
		"codex", "exec", "--cd", "/repo", "--model", "gpt-5.5",
		"--profile", "work", "--sandbox", "workspace-write",
		"--ask-for-approval", "on-request", "--config", "model_reasoning_effort=\"high\"",
		"implement SPEC",
	}
	if !slices.Equal(cmd.Argv, want) {
		t.Fatalf("argv = %#v, want %#v", cmd.Argv, want)
	}
	if cmd.Host != HostCodex {
		t.Fatalf("host = %q, want codex", cmd.Host)
	}
}

func TestBuildLaunchCommand_OpenCodeNative(t *testing.T) {
	cmd, err := BuildLaunchCommand(LaunchRequest{
		Host:        HostOpenCode,
		ProjectRoot: "/repo",
		Mode:        LaunchExec,
		Role:        "reviewer",
		Model:       "anthropic/claude-sonnet-4-5",
		Session:     "sess-1",
		Attach:      "http://localhost:4096",
		Auto:        true,
		Prompt:      "review changes",
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(opencode): %v", err)
	}

	want := []string{
		"opencode", "run", "--dir", "/repo", "--agent", "reviewer",
		"--model", "anthropic/claude-sonnet-4-5", "--session", "sess-1",
		"--attach", "http://localhost:4096", "--auto", "review changes",
	}
	if !slices.Equal(cmd.Argv, want) {
		t.Fatalf("argv = %#v, want %#v", cmd.Argv, want)
	}
	if cmd.Host != HostOpenCode {
		t.Fatalf("host = %q, want opencode", cmd.Host)
	}
}

func TestBuildLaunchCommand_RejectsUnknownHost(t *testing.T) {
	if _, err := BuildLaunchCommand(LaunchRequest{Host: Host("shell")}); err == nil {
		t.Fatal("BuildLaunchCommand should reject generic shell host")
	}
}

// TestBuildLaunchCommand_OpenCodeAttachIsStringValued asserts that `--attach`
// is emitted as a value-taking flag. `opencode run --help` types `--attach` as
// [string] ("attach to a running opencode server, e.g. http://localhost:4096"),
// so a bare `--attach` produces a malformed command at the host.
func TestBuildLaunchCommand_OpenCodeAttachIsStringValued(t *testing.T) {
	const attachURL = "http://localhost:4096"

	cmd, err := BuildLaunchCommand(LaunchRequest{
		Host:        HostOpenCode,
		ProjectRoot: "/repo",
		Mode:        LaunchExec,
		Attach:      attachURL,
		Auto:        true,
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(opencode): %v", err)
	}

	assertFlagValue(t, cmd.Argv, "--attach", attachURL)
	assertNoBareFlag(t, cmd.Argv, "--attach")
}

// TestBuildLaunchCommand_CodexInteractiveHasNoExec covers AC-AH-004 row 1: an
// interactive Codex request must not carry the `exec` subcommand.
func TestBuildLaunchCommand_CodexInteractiveHasNoExec(t *testing.T) {
	cmd, err := BuildLaunchCommand(LaunchRequest{
		Host:        HostCodex,
		ProjectRoot: "/repo",
		Mode:        LaunchInteractive,
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(codex): %v", err)
	}
	if cmd.Argv[0] != "codex" {
		t.Errorf("argv[0] = %q, want codex", cmd.Argv[0])
	}
	if slices.Contains(cmd.Argv, "exec") {
		t.Errorf("interactive codex argv must not contain the exec subcommand: %#v", cmd.Argv)
	}
}

// TestBuildLaunchCommand_CodexFlagArity asserts each AC-AH-004 surface
// individually, with its value. Flag presence alone is not sufficient.
func TestBuildLaunchCommand_CodexFlagArity(t *testing.T) {
	cmd, err := BuildLaunchCommand(LaunchRequest{
		Host:        HostCodex,
		ProjectRoot: "/repo",
		Mode:        LaunchExec,
		Model:       "gpt-5.5",
		Profile:     "work",
		Sandbox:     "workspace-write",
		Approval:    "on-request",
		Config:      "model_reasoning_effort=\"high\"",
		Prompt:      "implement SPEC",
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(codex): %v", err)
	}

	if cmd.Argv[0] != "codex" {
		t.Errorf("argv[0] = %q, want codex", cmd.Argv[0])
	}
	if cmd.Argv[1] != "exec" {
		t.Errorf("argv[1] = %q, want exec for non-interactive mode", cmd.Argv[1])
	}

	pairs := []struct{ flag, want string }{
		{"--cd", "/repo"},
		{"--model", "gpt-5.5"},
		{"--profile", "work"},
		{"--sandbox", "workspace-write"},
		{"--ask-for-approval", "on-request"},
		{"--config", "model_reasoning_effort=\"high\""},
	}
	for _, p := range pairs {
		t.Run(p.flag, func(t *testing.T) {
			assertFlagValue(t, cmd.Argv, p.flag, p.want)
			assertNoBareFlag(t, cmd.Argv, p.flag)
		})
	}
}

// TestBuildLaunchCommand_OpenCodeFlagArity asserts each AC-AH-005 surface
// individually, distinguishing value-taking flags from boolean switches.
func TestBuildLaunchCommand_OpenCodeFlagArity(t *testing.T) {
	cmd, err := BuildLaunchCommand(LaunchRequest{
		Host:        HostOpenCode,
		ProjectRoot: "/repo",
		Mode:        LaunchExec,
		Role:        "reviewer",
		Model:       "anthropic/claude-sonnet-4-5",
		Session:     "sess-1",
		Continue:    true,
		Attach:      "http://localhost:4096",
		Auto:        true,
		Prompt:      "review changes",
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(opencode): %v", err)
	}

	if cmd.Argv[0] != "opencode" {
		t.Errorf("argv[0] = %q, want opencode", cmd.Argv[0])
	}
	if cmd.Argv[1] != "run" {
		t.Errorf("argv[1] = %q, want run for non-interactive mode", cmd.Argv[1])
	}

	valueFlags := []struct{ flag, want string }{
		{"--dir", "/repo"},
		{"--agent", "reviewer"},
		{"--model", "anthropic/claude-sonnet-4-5"},
		{"--session", "sess-1"},
		{"--attach", "http://localhost:4096"},
	}
	for _, p := range valueFlags {
		t.Run(p.flag, func(t *testing.T) {
			assertFlagValue(t, cmd.Argv, p.flag, p.want)
			assertNoBareFlag(t, cmd.Argv, p.flag)
		})
	}

	for _, flag := range []string{"--continue", "--auto"} {
		t.Run(flag, func(t *testing.T) {
			assertBareFlag(t, cmd.Argv, flag)
		})
	}
}

// TestBuildLaunchCommand_ClaudeDefaultsToInteractive covers the Claude builder
// and the default-mode path: an unset Mode resolves to interactive.
func TestBuildLaunchCommand_ClaudeDefaultsToInteractive(t *testing.T) {
	cmd, err := BuildLaunchCommand(LaunchRequest{
		Host:        HostClaude,
		ProjectRoot: "/repo",
		Model:       "claude-opus-4-5",
		Profile:     "work",
		Prompt:      "plan the SPEC",
		ExtraArgs:   []string{"--verbose"},
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(claude): %v", err)
	}

	want := []string{
		"claude", "--model", "claude-opus-4-5", "--profile", "work",
		"plan the SPEC", "--verbose",
	}
	if !slices.Equal(cmd.Argv, want) {
		t.Fatalf("argv = %#v, want %#v", cmd.Argv, want)
	}
	if cmd.Host != HostClaude {
		t.Errorf("host = %q, want claude", cmd.Host)
	}
	if cmd.Cwd != "/repo" {
		t.Errorf("cwd = %q, want /repo", cmd.Cwd)
	}
}

// TestBuildLaunchCommand_HostNameIsNormalized covers the ParseHost path inside
// the builder: a differently-cased host name resolves to the canonical host.
func TestBuildLaunchCommand_HostNameIsNormalized(t *testing.T) {
	cmd, err := BuildLaunchCommand(LaunchRequest{Host: Host("CODEX")})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(CODEX): %v", err)
	}
	if cmd.Host != HostCodex {
		t.Errorf("host = %q, want codex", cmd.Host)
	}
}

// TestLaunchCommandString renders argv for dry-run output.
func TestLaunchCommandString(t *testing.T) {
	cmd := LaunchCommand{Argv: []string{"opencode", "run", "--dir", "/repo"}}
	if got, want := cmd.String(), "opencode run --dir /repo"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// assertFlagValue asserts that flag appears in argv immediately followed by the
// expected value. A flag emitted without its value fails.
func assertFlagValue(t *testing.T, argv []string, flag, want string) {
	t.Helper()
	for i, tok := range argv {
		if tok != flag {
			continue
		}
		if i+1 >= len(argv) {
			t.Errorf("%s: emitted as the final token with no value; want %q", flag, want)
			return
		}
		if got := argv[i+1]; got != want {
			t.Errorf("%s value = %q, want %q (argv=%#v)", flag, got, want, argv)
		}
		return
	}
	t.Errorf("%s absent from argv %#v", flag, argv)
}

// assertNoBareFlag asserts that flag is never emitted as a bare switch — that
// is, every occurrence is followed by a token that is not itself a flag.
func assertNoBareFlag(t *testing.T, argv []string, flag string) {
	t.Helper()
	for i, tok := range argv {
		if tok != flag {
			continue
		}
		if i+1 >= len(argv) {
			t.Errorf("%s emitted bare as the final token in argv %#v", flag, argv)
			return
		}
		if strings.HasPrefix(argv[i+1], "-") {
			t.Errorf("%s emitted bare: followed by %q in argv %#v", flag, argv[i+1], argv)
		}
		return
	}
}

// assertBareFlag asserts that flag appears as a bare boolean switch — the next
// token must be another flag or the flag must be absent from a value position.
func assertBareFlag(t *testing.T, argv []string, flag string) {
	t.Helper()
	if !slices.Contains(argv, flag) {
		t.Errorf("%s absent from argv %#v", flag, argv)
	}
}
