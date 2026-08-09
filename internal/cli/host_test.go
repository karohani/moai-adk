package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modu-ai/moai-adk/internal/agenthost"
)

func TestHostMatrixCmd_Text(t *testing.T) {
	cmd := newHostMatrixCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"opencode"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("host matrix opencode: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"Host: opencode",
		"PreToolUse",
		"tool.execute.before",
		"UserPromptSubmit",
		"fallback",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

func TestHostMatrixCmd_JSON(t *testing.T) {
	cmd := newHostMatrixCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"codex", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("host matrix codex --json: %v", err)
	}

	var matrices []agenthost.Matrix
	if err := json.Unmarshal(out.Bytes(), &matrices); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, out.String())
	}
	if len(matrices) != 1 {
		t.Fatalf("len(matrices) = %d, want 1", len(matrices))
	}
	if matrices[0].Host != agenthost.HostCodex {
		t.Fatalf("host = %q, want codex", matrices[0].Host)
	}
	mapping, ok := matrices[0].Find(agenthost.EventPreToolUse)
	if !ok {
		t.Fatal("codex JSON output missing PreToolUse")
	}
	if mapping.Support != agenthost.SupportNative {
		t.Fatalf("codex PreToolUse support = %q, want native", mapping.Support)
	}
}

func TestHostMatrixCmd_FeaturesText(t *testing.T) {
	cmd := newHostMatrixCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"opencode", "--features"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("host matrix opencode --features: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"Host: opencode",
		"launcher/runtime",
		"role_profiles",
		"hooks",
		"adapter",
		"slash_workflows",
		"fallback",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("feature output missing %q:\n%s", want, text)
		}
	}
}

func TestHostMatrixCmd_FeaturesJSON(t *testing.T) {
	cmd := newHostMatrixCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"codex", "--features", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("host matrix codex --features --json: %v", err)
	}

	var matrices []agenthost.FeatureMatrix
	if err := json.Unmarshal(out.Bytes(), &matrices); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, out.String())
	}
	if len(matrices) != 1 {
		t.Fatalf("len(matrices) = %d, want 1", len(matrices))
	}
	if matrices[0].Host != agenthost.HostCodex {
		t.Fatalf("host = %q, want codex", matrices[0].Host)
	}
	if len(matrices[0].Features) != len(agenthost.Features()) {
		t.Fatalf("feature count = %d, want %d", len(matrices[0].Features), len(agenthost.Features()))
	}
	mapping, ok := matrices[0].Find(agenthost.FeatureHooks)
	if !ok {
		t.Fatal("codex feature JSON output missing hooks")
	}
	if mapping.Support != agenthost.SupportNative {
		t.Fatalf("codex hooks support = %q, want native", mapping.Support)
	}
	if strings.Contains(out.String(), `"source": ""`) {
		t.Fatalf("feature JSON should omit empty per-feature source fields:\n%s", out.String())
	}
}

func TestHostCmd_RegistersMatrix(t *testing.T) {
	cmd := newHostCmd()
	if cmd == nil {
		t.Fatal("newHostCmd returned nil")
	}
	if _, _, err := cmd.Find([]string{"matrix"}); err != nil {
		t.Fatalf("host command should register matrix subcommand: %v", err)
	}
}
