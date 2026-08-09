package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
		"--attach", "http://localhost:4096",
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
		"--attach http://localhost:4096",
		"--auto",
		"review changes",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, text)
		}
	}
}

// TestOpenCodeCmd_AttachFlagIsStringValued covers AC-AH-017 at the CLI layer:
// `--attach` must be declared as a string flag, so a bare `--attach` with no
// value is rejected by the flag parser instead of producing a malformed argv.
func TestOpenCodeCmd_AttachFlagIsStringValued(t *testing.T) {
	cmd := newOpenCodeCmd()
	flag := cmd.Flags().Lookup("attach")
	if flag == nil {
		t.Fatal("opencode command does not declare an --attach flag")
	}
	if flag.Value.Type() != "string" {
		t.Errorf("--attach flag type = %q, want string", flag.Value.Type())
	}

	origFind := findProjectRootFn
	defer func() { findProjectRootFn = origFind }()
	findProjectRootFn = func() (string, error) { return "/repo", nil }

	bare := newOpenCodeCmd()
	var out, errOut bytes.Buffer
	bare.SetOut(&out)
	bare.SetErr(&errOut)
	bare.SilenceUsage = true
	bare.SetArgs([]string{"--dry-run", "--exec", "--attach"})
	if err := bare.Execute(); err == nil {
		t.Errorf("bare --attach with no value must be rejected; got output:\n%s", out.String())
	}
}

// TestHostLaunch_DryRunDoesNotLeakEnvironment covers AC-AH-018: dry-run output
// carries the calculated argv only — never the process environment or any
// credential, token, or API-key value.
func TestHostLaunch_DryRunDoesNotLeakEnvironment(t *testing.T) {
	origFind := findProjectRootFn
	defer func() { findProjectRootFn = origFind }()
	findProjectRootFn = func() (string, error) { return "/repo", nil }

	const sentinel = "moai-secret-sentinel-value"
	t.Setenv("ANTHROPIC_AUTH_TOKEN", sentinel)
	t.Setenv("OPENAI_API_KEY", sentinel)

	for name, factory := range map[string]func() *cobra.Command{
		"codex":    newCodexCmd,
		"opencode": newOpenCodeCmd,
	} {
		t.Run(name, func(t *testing.T) {
			cmd := factory()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--dry-run", "--exec", "do the thing"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("%s dry-run: %v", name, err)
			}

			text := out.String()
			if strings.Contains(text, sentinel) {
				t.Errorf("%s dry-run output leaked a credential value:\n%s", name, text)
			}
			for _, forbidden := range []string{"ANTHROPIC_AUTH_TOKEN", "OPENAI_API_KEY", "PATH="} {
				if strings.Contains(text, forbidden) {
					t.Errorf("%s dry-run output leaked environment key %q:\n%s", name, forbidden, text)
				}
			}
			if !strings.HasPrefix(strings.TrimSpace(text), "[dry-run] "+name) {
				t.Errorf("%s dry-run output should be the argv line only, got:\n%s", name, text)
			}
		})
	}
}
