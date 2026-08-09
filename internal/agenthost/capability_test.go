package agenthost

import (
	"strings"
	"testing"
)

func TestMatrixFor_CodexHookParity(t *testing.T) {
	matrix, err := MatrixFor(HostCodex)
	if err != nil {
		t.Fatalf("MatrixFor(codex): %v", err)
	}

	for _, event := range []Event{
		EventSessionStart,
		EventPreToolUse,
		EventPermissionRequest,
		EventPostToolUse,
		EventUserPromptSubmit,
		EventStop,
		EventSubagentStart,
		EventSubagentStop,
		EventPreCompact,
		EventPostCompact,
	} {
		mapping, ok := matrix.Find(event)
		if !ok {
			t.Fatalf("codex matrix missing %s", event)
		}
		if mapping.Support != SupportNative {
			t.Errorf("codex %s support = %s, want native", event, mapping.Support)
		}
		if mapping.HostEvent == "" {
			t.Errorf("codex %s host event should not be empty", event)
		}
	}
}

func TestMatrixFor_OpenCodeAdapterBoundaries(t *testing.T) {
	matrix, err := MatrixFor(HostOpenCode)
	if err != nil {
		t.Fatalf("MatrixFor(opencode): %v", err)
	}

	cases := []struct {
		event    Event
		want     SupportLevel
		hostPart string
	}{
		{EventPreToolUse, SupportAdapter, "tool.execute.before"},
		{EventPermissionRequest, SupportAdapter, "permission.asked"},
		{EventPostToolUse, SupportAdapter, "tool.execute.after"},
		{EventUserPromptSubmit, SupportFallback, "tui.prompt.append"},
		{EventStop, SupportFallback, "session.idle"},
		{EventSubagentStop, SupportFallback, "session.idle"},
		{EventPreCompact, SupportAdapter, "experimental.session.compacting"},
	}

	for _, tc := range cases {
		mapping, ok := matrix.Find(tc.event)
		if !ok {
			t.Fatalf("opencode matrix missing %s", tc.event)
		}
		if mapping.Support != tc.want {
			t.Errorf("opencode %s support = %s, want %s", tc.event, mapping.Support, tc.want)
		}
		if !strings.Contains(mapping.HostEvent, tc.hostPart) {
			t.Errorf("opencode %s host event = %q, want to contain %q", tc.event, mapping.HostEvent, tc.hostPart)
		}
		if mapping.Support != SupportNative && mapping.Degradation == "" {
			t.Errorf("opencode %s should explain degradation", tc.event)
		}
	}
}

// TestMatrixFor_CodexProjectLayerTrustDegradation covers AC-AH-011: Codex hook
// events are delivered through the project-scoped `.codex/hooks.json`, which
// Codex loads only when the project layer is trusted. The mapping must
// therefore never be reported as *unconditionally* native — every mapping
// carries a trust-conditional degradation note.
func TestMatrixFor_CodexProjectLayerTrustDegradation(t *testing.T) {
	matrix, err := MatrixFor(HostCodex)
	if err != nil {
		t.Fatalf("MatrixFor(codex): %v", err)
	}

	for _, mapping := range matrix.Mappings {
		t.Run(string(mapping.Event), func(t *testing.T) {
			if mapping.Degradation == "" {
				t.Fatalf("codex %s is reported as unconditionally native: degradation note is empty", mapping.Event)
			}
			note := strings.ToLower(mapping.Degradation)
			if !strings.Contains(note, "trust") {
				t.Errorf("codex %s degradation note must state the project-layer trust condition, got %q", mapping.Event, mapping.Degradation)
			}
			if !strings.Contains(note, ".codex") {
				t.Errorf("codex %s degradation note must name the project-scoped .codex layer, got %q", mapping.Event, mapping.Degradation)
			}
		})
	}
}

// TestMatrixFor_NonNativeMappingsCarrySourceAndNote covers AC-AH-011: a mapping
// whose support level is non-native must carry BOTH a non-empty degradation
// note AND a non-empty source field. The source is the anti-overclaiming
// mechanism and is asserted independently of the note.
func TestMatrixFor_NonNativeMappingsCarrySourceAndNote(t *testing.T) {
	for _, host := range Hosts() {
		matrix, err := MatrixFor(host)
		if err != nil {
			t.Fatalf("MatrixFor(%s): %v", host, err)
		}
		for _, mapping := range matrix.Mappings {
			if mapping.Support == SupportNative {
				continue
			}
			t.Run(string(host)+"/"+string(mapping.Event), func(t *testing.T) {
				if mapping.Degradation == "" {
					t.Errorf("%s %s is %s but carries no degradation note", host, mapping.Event, mapping.Support)
				}
				if mapping.Source == "" {
					t.Errorf("%s %s is %s but carries no source or local-evidence field", host, mapping.Event, mapping.Support)
				}
			})
		}
	}
}

// TestAllMatrices_CoversEveryHost asserts the aggregate accessors return one
// entry per supported host, in the stable display order.
func TestAllMatrices_CoversEveryHost(t *testing.T) {
	hosts := Hosts()

	matrices := AllMatrices()
	if len(matrices) != len(hosts) {
		t.Fatalf("AllMatrices() len = %d, want %d", len(matrices), len(hosts))
	}
	for i, matrix := range matrices {
		if matrix.Host != hosts[i] {
			t.Errorf("AllMatrices()[%d].Host = %q, want %q", i, matrix.Host, hosts[i])
		}
		if matrix.Source == "" {
			t.Errorf("%s matrix carries no source", matrix.Host)
		}
	}

	featureMatrices := AllFeatureMatrices()
	if len(featureMatrices) != len(hosts) {
		t.Fatalf("AllFeatureMatrices() len = %d, want %d", len(featureMatrices), len(hosts))
	}
	for i, matrix := range featureMatrices {
		if matrix.Host != hosts[i] {
			t.Errorf("AllFeatureMatrices()[%d].Host = %q, want %q", i, matrix.Host, hosts[i])
		}
	}
}

// TestMatrixFor_RejectsUnknownHost covers the error paths of both matrix
// accessors.
func TestMatrixFor_RejectsUnknownHost(t *testing.T) {
	if _, err := MatrixFor(Host("shell")); err == nil {
		t.Error("MatrixFor should reject an unsupported host")
	}
	if _, err := FeatureMatrixFor(Host("shell")); err == nil {
		t.Error("FeatureMatrixFor should reject an unsupported host")
	}
}

// TestFind_ReportsMissingEntries covers the not-found branch of both Find
// methods.
func TestFind_ReportsMissingEntries(t *testing.T) {
	matrix, err := MatrixFor(HostClaude)
	if err != nil {
		t.Fatalf("MatrixFor(claude): %v", err)
	}
	if _, ok := matrix.Find(Event("NoSuchEvent")); ok {
		t.Error("Matrix.Find should report a missing event as not found")
	}

	featureMatrix, err := FeatureMatrixFor(HostClaude)
	if err != nil {
		t.Fatalf("FeatureMatrixFor(claude): %v", err)
	}
	if _, ok := featureMatrix.Find(Feature("no_such_feature")); ok {
		t.Error("FeatureMatrix.Find should report a missing feature as not found")
	}
}

func TestParseHost(t *testing.T) {
	host, err := ParseHost(" Codex ")
	if err != nil {
		t.Fatalf("ParseHost: %v", err)
	}
	if host != HostCodex {
		t.Fatalf("host = %q, want %q", host, HostCodex)
	}

	if _, err := ParseHost("unknown"); err == nil {
		t.Fatal("ParseHost should reject unknown host")
	}
}

func TestFeatureMatrixFor_CoversInventory(t *testing.T) {
	wantFeatures := Features()
	for _, host := range Hosts() {
		matrix, err := FeatureMatrixFor(host)
		if err != nil {
			t.Fatalf("FeatureMatrixFor(%s): %v", host, err)
		}
		if matrix.Host != host {
			t.Fatalf("matrix host = %q, want %q", matrix.Host, host)
		}
		if len(matrix.Features) != len(wantFeatures) {
			t.Fatalf("%s feature count = %d, want %d", host, len(matrix.Features), len(wantFeatures))
		}
		for _, feature := range wantFeatures {
			mapping, ok := matrix.Find(feature)
			if !ok {
				t.Fatalf("%s feature matrix missing %s", host, feature)
			}
			if mapping.Support == "" {
				t.Errorf("%s %s support should not be empty", host, feature)
			}
			if mapping.Surface == "" {
				t.Errorf("%s %s surface should not be empty", host, feature)
			}
			if mapping.Support != SupportNative && mapping.Degradation == "" {
				t.Errorf("%s %s should explain non-native degradation", host, feature)
			}
		}
	}
}

func TestFeatureMatrixFor_OpenCodeBoundaries(t *testing.T) {
	matrix, err := FeatureMatrixFor(HostOpenCode)
	if err != nil {
		t.Fatalf("FeatureMatrixFor(opencode): %v", err)
	}

	cases := []struct {
		feature Feature
		want    SupportLevel
	}{
		{FeatureLauncherRuntime, SupportNative},
		{FeatureRoleProfiles, SupportNative},
		{FeatureHooks, SupportAdapter},
		{FeatureSlashWorkflows, SupportFallback},
		{FeatureStateSession, SupportFallback},
	}
	for _, tc := range cases {
		mapping, ok := matrix.Find(tc.feature)
		if !ok {
			t.Fatalf("opencode feature matrix missing %s", tc.feature)
		}
		if mapping.Support != tc.want {
			t.Errorf("opencode %s support = %s, want %s", tc.feature, mapping.Support, tc.want)
		}
	}
}
