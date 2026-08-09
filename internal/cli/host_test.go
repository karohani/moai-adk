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

// TestHostMatrixCmd_FeatureGroupSetIsExact asserts that the feature-group
// identifiers emitted by `moai host matrix --features --json` form a set EQUAL
// to the 12 canonical identifiers. Neither a missing identifier nor an extra
// one passes.
//
// The expected identifiers are spelled out as string literals on purpose. A
// test that derived them from agenthost.Features() would be circular: it would
// pass no matter how the constants were renamed. These literals are the
// independent anchor, including the `/` separator in `launcher/runtime`, which
// is the only identifier that does not use `_`.
func TestHostMatrixCmd_FeatureGroupSetIsExact(t *testing.T) {
	wantIdentifiers := []string{
		"launcher/runtime",
		"slash_workflows",
		"spec_lifecycle",
		"role_profiles",
		"tmux_delegation",
		"hooks",
		"quality_gates",
		"harness_lifecycle",
		"state_session",
		"project_config",
		"git_pr",
		"skills_templates",
	}

	cmd := newHostMatrixCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--features", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("host matrix --features --json: %v", err)
	}

	var matrices []agenthost.FeatureMatrix
	if err := json.Unmarshal(out.Bytes(), &matrices); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, out.String())
	}
	if len(matrices) == 0 {
		t.Fatal("host matrix --features --json returned no host matrices")
	}

	want := make(map[string]bool, len(wantIdentifiers))
	for _, id := range wantIdentifiers {
		want[id] = true
	}

	// Every host must expose exactly the canonical set — no more, no fewer.
	for _, matrix := range matrices {
		got := make(map[string]bool, len(matrix.Features))
		for _, feature := range matrix.Features {
			got[string(feature.Feature)] = true
		}

		for id := range want {
			if !got[id] {
				t.Errorf("%s feature set is missing canonical identifier %q", matrix.Host, id)
			}
		}
		for id := range got {
			if !want[id] {
				t.Errorf("%s feature set contains non-canonical identifier %q", matrix.Host, id)
			}
		}
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
