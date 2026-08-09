package template

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// @MX:NOTE: [AUTO] split_namespace_test.go — SPEC-V3R6-DEV-HARNESS-SPLIT-001
// REQ-DHS-004 / REQ-DHS-005 embedded-tree-absence guard for the three split
// dev-only maintainer harnesses (release-update / github / release).
//
// The split harnesses (thin command entries, the release-update manifest +
// Runner, the three specialist sub-agents) are DEV-ONLY maintainer assets in the
// user-owned harness namespace. They MUST NOT leak into
// internal/template/templates/ (the user-facing distribution tree).
//
// History: this guard replaces the SPEC-V3R6-DEV-HARNESS-CONSOLIDATION-001
// unified-harness namespace guard. That consolidation's single unified harness
// was split into three independent harnesses by SPEC-V3R6-DEV-HARNESS-SPLIT-001,
// so the guard now asserts the three split artifact prefixes are absent instead
// of the single legacy unified-harness prefix.
//
// The actual CI protection for these artifacts is "absence-from-embedded-tree",
// modeled on embedded_namespace_test.go (TestTemplateAgentsStructure:
// {moai}-only allowlist on .claude/agents/). The dev_only_skill_test.go walker is
// .claude/skills/-only and therefore cannot protect commands/agents/workflows
// artifacts. This test fills that gap: it walks EmbeddedTemplates() and asserts
// the absence of every split-harness artifact shape.
//
// Sentinel on failure: SPLIT_HARNESS_NAMESPACE_LEAK
// Cross-reference: .moai/docs/dev-only-commands-isolation.md, CLAUDE.local.md §21/§2.
//
// RED/GREEN: planting a `.claude/commands/harness/` path (or a
// `harness-{release-update,github,release}*` agent, or a
// `.claude/workflows/harness-{release-update,github,release}-*` file) under
// internal/template/templates/ and running `make build` re-generates embedded.go
// with the leak compiled in — this test then FAILS (RED). Removing the leak and
// rebuilding restores PASS (GREEN).

// splitHarnessAgentPrefixes is the canonical set of dev-only split-harness
// artifact-name prefixes that MUST NOT appear under .claude/agents/ or
// .claude/workflows/ in the embedded template tree.
var splitHarnessAgentPrefixes = []string{
	"harness-release-update",
	"harness-github",
	"harness-release",
}

// splitHarnessGuardedTrees is the set of host template trees walked by the
// guard. Every host surface MoAI installs into a user project is covered:
// `.claude/` (Claude Code), `.codex/` (Codex), and `.opencode/` (OpenCode).
//
// The guard is UNCONDITIONAL across all three — it consults no flag, no
// environment variable, and no configuration. Making dev-only distribution
// conditional would contradict the guard whenever the condition were set.
var splitHarnessGuardedTrees = []string{".claude", ".codex", ".opencode"}

// scanSplitHarnessLeak walks one template tree and reports every dev-only
// split-harness artifact shape it finds:
//
//	(a) commands/harness/   path (thin commands + release-update manifest)
//	(b) harness-{release-update,github,release}* files under agents/
//	(c) workflows/harness-{release-update,github,release}-* files (the Runner)
//
// report receives the offending path and a human-readable reason. Extracting
// the scan lets the same detector cover every guarded tree and lets a negative
// control prove the detector actually fires.
func scanSplitHarnessLeak(fsys fs.FS, root string, report func(filePath, reason string)) error {
	return fs.WalkDir(fsys, root, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			// A guarded subtree may be absent in some build states; absence is
			// the success case, so skip rather than fail.
			return nil //nolint:nilerr // tolerate partial trees; absence is the success case
		}

		// (a) commands/harness/ path MUST NOT appear (the split thin commands +
		//     the release-update manifest live there; the whole harness commands
		//     subdir is dev-only).
		if strings.Contains(filePath, root+"/commands/harness") {
			report(filePath, "dev-only harness command path `"+root+"/commands/harness/` is a maintainer namespace and MUST NOT be distributed")
		}

		base := path.Base(filePath)

		// (c) workflows/harness-{release-update,github,release}-* files MUST NOT
		//     appear (the Runner; only release-update has one, but all three
		//     prefixes are guarded for completeness).
		if strings.Contains(filePath, root+"/workflows") {
			for _, prefix := range splitHarnessAgentPrefixes {
				if strings.HasPrefix(base, prefix+"-") {
					report(filePath, "dev-only split-harness Runner `"+root+"/workflows/"+prefix+"-*` is a maintainer asset and MUST NOT be distributed")
					break
				}
			}
		}

		// (b) harness-{release-update,github,release}* agent files MUST NOT appear
		//     anywhere under agents/ (the specialist sub-agents).
		if strings.Contains(filePath, root+"/agents") {
			for _, prefix := range splitHarnessAgentPrefixes {
				if strings.HasPrefix(base, prefix) {
					report(filePath, "dev-only split-harness specialist `"+prefix+"*` is a maintainer asset and MUST NOT be distributed")
					break
				}
			}
		}

		return nil
	})
}

// TestSplitHarnessNamespaceNoLeak asserts that NONE of the split-harness
// artifact shapes appear in ANY guarded host template tree (.claude, .codex,
// .opencode). Extending this existing guard — rather than adding a second,
// separately-maintained one — keeps a single detector authoritative for every
// host surface.
func TestSplitHarnessNamespaceNoLeak(t *testing.T) {
	t.Parallel()

	fsys, err := EmbeddedTemplates()
	if err != nil {
		t.Fatalf("EmbeddedTemplates() error = %v, want nil", err)
	}

	for _, root := range splitHarnessGuardedTrees {
		t.Run(root, func(t *testing.T) {
			walkErr := scanSplitHarnessLeak(fsys, root, func(filePath, reason string) {
				t.Errorf("SPLIT_HARNESS_NAMESPACE_LEAK: %q found in embedded template tree. %s.", filePath, reason)
			})
			if walkErr != nil {
				t.Fatalf("WalkDir(%s) error = %v, want nil", root, walkErr)
			}
		})
	}
}

// TestSplitHarnessLeakDetectorFiresOnPlantedLeak is the negative control for
// the guard above. A guard that can never fire is worse than no guard, so this
// plants one leak of each shape in each guarded tree and asserts the detector
// reports it. It exercises the detector against an in-memory tree and never
// touches the real embedded templates.
func TestSplitHarnessLeakDetectorFiresOnPlantedLeak(t *testing.T) {
	t.Parallel()

	for _, root := range splitHarnessGuardedTrees {
		for _, planted := range []string{
			root + "/commands/harness/release-update.md",
			root + "/agents/harness-github-specialist.md",
			root + "/workflows/harness-release-update-run.js",
		} {
			t.Run(planted, func(t *testing.T) {
				fsys := fstest.MapFS{
					planted:              &fstest.MapFile{Data: []byte("planted leak")},
					root + "/keep-me.md": &fstest.MapFile{Data: []byte("legitimate file")},
				}

				var found []string
				if err := scanSplitHarnessLeak(fsys, root, func(filePath, _ string) {
					found = append(found, filePath)
				}); err != nil {
					t.Fatalf("scanSplitHarnessLeak(%s) error = %v, want nil", root, err)
				}

				if !slices.Contains(found, planted) {
					t.Errorf("leak detector did not fire on planted leak %q; reported %v", planted, found)
				}
			})
		}
	}
}
