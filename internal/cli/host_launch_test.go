package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCodexCmd_DryRun(t *testing.T) {
	origFind := findProjectRootFn
	defer func() { findProjectRootFn = origFind }()
	findProjectRootFn = func() (string, error) { return "/repo", nil }

	cmd := newCodexCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--dry-run",
		"--exec",
		"--model", "gpt-5.5",
		"--profile", "work",
		"--sandbox", "workspace-write",
		"--ask-for-approval", "on-request",
		"implement SPEC",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("codex dry-run: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"[dry-run] codex exec",
		"--cd /repo",
		"--model gpt-5.5",
		"--profile work",
		"--sandbox workspace-write",
		"--ask-for-approval on-request",
		"implement SPEC",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, text)
		}
	}
}

func TestOpenCodeCmd_DryRun(t *testing.T) {
	origFind := findProjectRootFn
	defer func() { findProjectRootFn = origFind }()
	findProjectRootFn = func() (string, error) { return "/repo", nil }

	cmd := newOpenCodeCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--dry-run",
		"--exec",
		"--agent", "reviewer",
		"--model", "anthropic/claude-sonnet-4-5",
		"--session", "sess-1",
		"--attach",
		"--auto",
		"review changes",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("opencode dry-run: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"[dry-run] opencode run",
		"--dir /repo",
		"--agent reviewer",
		"--model anthropic/claude-sonnet-4-5",
		"--session sess-1",
		"--attach",
		"--auto",
		"review changes",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, text)
		}
	}
}
