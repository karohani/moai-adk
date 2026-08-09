package template

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

// Host template render-validity tests.
//
// These cover the Codex and OpenCode template surfaces:
//
//   - the shared project-root instruction file AGENTS.md, which serves BOTH
//     hosts (Codex reads AGENTS.md directly; OpenCode reads it as well and can
//     also load it through the instructions key of opencode.json),
//   - the project-root opencode.json config,
//   - the OpenCode agent wrapper under the plural agents/ directory,
//   - the OpenCode plugin under the plural plugins/ directory.
//
// Exact paths are asserted, not merely "a file exists somewhere": a config
// written to the wrong path still renders cleanly while being inert at the
// host, which is the failure mode these tests exist to catch.

// mustStatTemplate asserts that a template source path exists in the embedded
// tree and returns nothing. It is the "exact path" half of the path assertions.
func mustStatTemplate(t *testing.T, tmplPath string) {
	t.Helper()
	fsys, err := EmbeddedTemplates()
	if err != nil {
		t.Fatalf("EmbeddedTemplates() error: %v", err)
	}
	if _, err := fs.Stat(fsys, tmplPath); err != nil {
		t.Fatalf("required host template %q is absent from the embedded tree: %v", tmplPath, err)
	}
}

// mustNotExistTemplate asserts that a path is absent from the embedded tree.
func mustNotExistTemplate(t *testing.T, tmplPath string) {
	t.Helper()
	fsys, err := EmbeddedTemplates()
	if err != nil {
		t.Fatalf("EmbeddedTemplates() error: %v", err)
	}
	if _, err := fs.Stat(fsys, tmplPath); err == nil {
		t.Errorf("path %q must NOT be present in the embedded tree", tmplPath)
	}
}

// assertNoUnresolvedTemplateVars fails when rendered output still contains Go
// template action delimiters.
func assertNoUnresolvedTemplateVars(t *testing.T, name, rendered string) {
	t.Helper()
	for _, marker := range []string{"{{", "}}"} {
		if strings.Contains(rendered, marker) {
			t.Errorf("%s rendered with an unresolved template variable (found %q):\n%s", name, marker, rendered)
		}
	}
}

// --- AGENTS.md.tmpl (shared instruction file, serves Codex AND OpenCode) ---

func TestAgentsInstructionTemplateAtProjectRoot(t *testing.T) {
	// The shared instruction file renders to the PROJECT ROOT as AGENTS.md.
	// The deployer strips the .tmpl suffix, so a source at templates/AGENTS.md.tmpl
	// lands at <project-root>/AGENTS.md.
	mustStatTemplate(t, "AGENTS.md.tmpl")

	// One file serves both hosts: a second per-host instruction copy would drift.
	mustNotExistTemplate(t, ".codex/AGENTS.md.tmpl")
	mustNotExistTemplate(t, ".opencode/AGENTS.md.tmpl")
}

func TestAgentsInstructionTemplateRendersCleanly(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			output := renderTemplate(t, "AGENTS.md.tmpl", testContext(platform))
			if strings.TrimSpace(output) == "" {
				t.Fatal("rendered AGENTS.md is empty")
			}
			assertNoUnresolvedTemplateVars(t, "AGENTS.md", output)
		})
	}
}

func TestAgentsInstructionTemplateServesBothHosts(t *testing.T) {
	output := renderTemplate(t, "AGENTS.md.tmpl", testContext("darwin"))
	for _, want := range []string{"Codex", "OpenCode", "AGENTS.md"} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered AGENTS.md does not mention %q; it is the shared instruction surface for both hosts", want)
		}
	}
}

// TestCodexTemplateSetHasNoConfigToml asserts the ABSENCE of a Codex
// config.toml. Its absence is a scope decision and is asserted, not merely
// left unmentioned.
func TestCodexTemplateSetHasNoConfigToml(t *testing.T) {
	mustNotExistTemplate(t, ".codex/config.toml")
	mustNotExistTemplate(t, ".codex/config.toml.tmpl")
}

// --- opencode.json.tmpl (project-root OpenCode config) ---

func TestOpenCodeConfigTemplateAtProjectRoot(t *testing.T) {
	// opencode.json belongs at the PROJECT ROOT. A file rendered to
	// .opencode/opencode.json would not be loaded as the project config.
	mustStatTemplate(t, "opencode.json.tmpl")
	mustNotExistTemplate(t, ".opencode/opencode.json.tmpl")
	mustNotExistTemplate(t, ".opencode/opencode.json")
}

func TestOpenCodeConfigTemplateValid(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			output := renderTemplate(t, "opencode.json.tmpl", testContext(platform))
			assertNoUnresolvedTemplateVars(t, "opencode.json", output)

			trimmed := strings.TrimSpace(output)
			if !json.Valid([]byte(trimmed)) {
				t.Fatalf("rendered opencode.json is not valid JSON:\n%s", trimmed)
			}

			var cfg struct {
				Schema       string   `json:"$schema"`
				Instructions []string `json:"instructions"`
			}
			if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
				t.Fatalf("decode opencode.json: %v", err)
			}

			const wantSchema = "https://opencode.ai/config.json"
			if cfg.Schema != wantSchema {
				t.Errorf("opencode.json $schema = %q, want %q", cfg.Schema, wantSchema)
			}

			// The config must point OpenCode at the shared instruction file.
			found := false
			for _, entry := range cfg.Instructions {
				if strings.Contains(entry, "AGENTS.md") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("opencode.json instructions %v does not reference AGENTS.md", cfg.Instructions)
			}
		})
	}
}

// --- .opencode/agents/ and .opencode/plugins/ (plural directory names) ---

func TestOpenCodeAgentTemplateUsesPluralDirectory(t *testing.T) {
	mustStatTemplate(t, ".opencode/agents/moai-reviewer.md.tmpl")

	// Singular "agent" is the wrong directory name; a wrapper written there
	// renders cleanly but is never loaded.
	mustNotExistTemplate(t, ".opencode/agent/moai-reviewer.md.tmpl")
}

func TestOpenCodePluginTemplateUsesPluralDirectory(t *testing.T) {
	mustStatTemplate(t, ".opencode/plugins/moai-hooks.js.tmpl")
	mustNotExistTemplate(t, ".opencode/plugin/moai-hooks.js.tmpl")
}

func TestOpenCodeAgentTemplateRendersCleanly(t *testing.T) {
	output := renderTemplate(t, ".opencode/agents/moai-reviewer.md.tmpl", testContext("darwin"))
	assertNoUnresolvedTemplateVars(t, ".opencode/agents/moai-reviewer.md", output)

	if !strings.HasPrefix(strings.TrimSpace(output), "---") {
		t.Error("OpenCode agent wrapper must open with YAML frontmatter")
	}
	for _, want := range []string{"description:", "mode:"} {
		if !strings.Contains(output, want) {
			t.Errorf("OpenCode agent wrapper frontmatter missing %q", want)
		}
	}
}

func TestOpenCodePluginTemplateRendersCleanly(t *testing.T) {
	output := renderTemplate(t, ".opencode/plugins/moai-hooks.js.tmpl", testContext("darwin"))
	assertNoUnresolvedTemplateVars(t, ".opencode/plugins/moai-hooks.js", output)
	if strings.TrimSpace(output) == "" {
		t.Fatal("rendered OpenCode plugin is empty")
	}
}

// TestOpenCodePluginAttributesHostToOpenCode asserts the plugin carries the
// host attribution literal, so a forwarded event can be attributed to the
// OpenCode host by the shared wrapper it invokes.
func TestOpenCodePluginAttributesHostToOpenCode(t *testing.T) {
	output := renderTemplate(t, ".opencode/plugins/moai-hooks.js.tmpl", testContext("darwin"))

	const attribution = "MOAI_HOOK_HOST=opencode"
	if !strings.Contains(output, attribution) {
		t.Errorf("OpenCode plugin does not contain the host attribution %q:\n%s", attribution, output)
	}
	// The attribution must be exported into the wrapper's environment, not
	// merely mentioned in a comment.
	if !strings.Contains(output, "export "+attribution) {
		t.Errorf("OpenCode plugin does not export %q into the hook wrapper environment", attribution)
	}
	// No other host may be attributed from the OpenCode plugin.
	for _, foreign := range []string{"MOAI_HOOK_HOST=codex", "MOAI_HOOK_HOST=claude"} {
		if strings.Contains(output, foreign) {
			t.Errorf("OpenCode plugin must not attribute events to another host (%q)", foreign)
		}
	}
}

// TestOpenCodePluginEventNamesMatchCapabilityMatrix keeps the plugin and the
// published host matrix from drifting apart. Every non-native OpenCode event
// name recorded in the matrix must be handled by the plugin, otherwise the
// matrix advertises a mapping the adapter does not actually forward.
func TestOpenCodePluginEventNamesMatchCapabilityMatrix(t *testing.T) {
	output := renderTemplate(t, ".opencode/plugins/moai-hooks.js.tmpl", testContext("darwin"))

	// Host event names as published by `moai host matrix opencode`.
	hostEvents := []string{
		"session.created",
		"tool.execute.before",
		"tool.execute.after",
		"file.edited",
		"permission.asked",
		"tui.prompt.append",
		"session.idle",
		"experimental.session.compacting",
		"session.compacted",
	}
	for _, event := range hostEvents {
		if !strings.Contains(output, event) {
			t.Errorf("OpenCode plugin does not handle host event %q advertised by the capability matrix", event)
		}
	}
}
