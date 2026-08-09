package agenthost

import (
	"slices"
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
		Attach:      true,
		Auto:        true,
		Prompt:      "review changes",
	})
	if err != nil {
		t.Fatalf("BuildLaunchCommand(opencode): %v", err)
	}

	want := []string{
		"opencode", "run", "--dir", "/repo", "--agent", "reviewer",
		"--model", "anthropic/claude-sonnet-4-5", "--session", "sess-1",
		"--attach", "--auto", "review changes",
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
