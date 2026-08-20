package proxy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestLoadRegistry_MissingFileIsNormalState verifies AC-PROXY-016 precondition
// and plan.md M1.2: absence of the machine registry is a normal state (a user
// who has not yet registered any groups), not an error.
func TestLoadRegistry_MissingFileIsNormalState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missingPath := filepath.Join(dir, "does-not-exist", "proxy-groups.yaml")

	reg, err := LoadRegistry(missingPath)
	if err != nil {
		t.Fatalf("LoadRegistry(missing) returned error, want nil: %v", err)
	}
	if reg == nil {
		t.Fatal("LoadRegistry(missing) returned nil registry, want empty registry")
	}
	if len(reg.Groups) != 0 {
		t.Errorf("Groups: got %d entries, want 0 (empty registry)", len(reg.Groups))
	}
	if len(reg.Sets) != 0 {
		t.Errorf("Sets: got %d entries, want 0 (empty registry)", len(reg.Sets))
	}
}

// TestLoadRegistry_ParsesGroupsAndSets verifies design.md §3.2/§3.3 schema:
// groups map by name (not type-keyed) and sets map by name to an ordered
// group-name list.
func TestLoadRegistry_ParsesGroupsAndSets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "proxy-groups.yaml")
	content := `
groups:
  bedrock-us:
    type: bedrock
    region: us-east-1
    profile: default
    aliases:
      opus: us.anthropic.claude-opus
      sonnet: us.anthropic.claude-sonnet
      haiku: us.anthropic.claude-haiku
      fable: us.anthropic.claude-fable
    models:
      - us.anthropic.claude-extra
  gpu-cluster-a:
    type: openai-compatible
    base_url: http://10.0.0.5:8000/v1
    aliases:
      opus: qwen-3-72b
      sonnet: qwen-3-32b
sets:
  default: [bedrock-us, gpu-cluster-a]
  cheap: [gpu-cluster-a]
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	if len(reg.Groups) != 2 {
		t.Fatalf("Groups: got %d entries, want 2", len(reg.Groups))
	}
	bedrock, ok := reg.Groups["bedrock-us"]
	if !ok {
		t.Fatal("Groups[\"bedrock-us\"] missing")
	}
	if bedrock.Type != GroupTypeBedrock {
		t.Errorf("bedrock-us.Type: got %q, want %q", bedrock.Type, GroupTypeBedrock)
	}
	if bedrock.Region != "us-east-1" {
		t.Errorf("bedrock-us.Region: got %q, want %q", bedrock.Region, "us-east-1")
	}
	if bedrock.Aliases.Opus != "us.anthropic.claude-opus" {
		t.Errorf("bedrock-us.Aliases.Opus: got %q, want %q", bedrock.Aliases.Opus, "us.anthropic.claude-opus")
	}
	if len(bedrock.Models) != 1 || bedrock.Models[0] != "us.anthropic.claude-extra" {
		t.Errorf("bedrock-us.Models: got %v, want [us.anthropic.claude-extra]", bedrock.Models)
	}

	gpu, ok := reg.Groups["gpu-cluster-a"]
	if !ok {
		t.Fatal("Groups[\"gpu-cluster-a\"] missing")
	}
	if gpu.Type != GroupTypeOpenAICompatible {
		t.Errorf("gpu-cluster-a.Type: got %q, want %q", gpu.Type, GroupTypeOpenAICompatible)
	}
	if gpu.BaseURL != "http://10.0.0.5:8000/v1" {
		t.Errorf("gpu-cluster-a.BaseURL: got %q, want %q", gpu.BaseURL, "http://10.0.0.5:8000/v1")
	}

	if len(reg.Sets) != 2 {
		t.Fatalf("Sets: got %d entries, want 2", len(reg.Sets))
	}
	wantDefault := []string{"bedrock-us", "gpu-cluster-a"}
	if got := reg.Sets["default"]; !equalStrSlices(got, wantDefault) {
		t.Errorf("Sets[\"default\"]: got %v, want %v", got, wantDefault)
	}
	wantCheap := []string{"gpu-cluster-a"}
	if got := reg.Sets["cheap"]; !equalStrSlices(got, wantCheap) {
		t.Errorf("Sets[\"cheap\"]: got %v, want %v", got, wantCheap)
	}
}

// TestLoadRegistry_SameTypeGroupsCoexist verifies AC-PROXY-003: two groups of
// the same type are distinct registry entries — neither overwrites the other.
func TestLoadRegistry_SameTypeGroupsCoexist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "proxy-groups.yaml")
	content := `
groups:
  bedrock-us:
    type: bedrock
    region: us-east-1
  bedrock-eu:
    type: bedrock
    region: eu-west-1
sets: {}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(reg.Groups) != 2 {
		t.Fatalf("Groups: got %d entries, want 2", len(reg.Groups))
	}
	if reg.Groups["bedrock-us"].Region == reg.Groups["bedrock-eu"].Region {
		t.Error("bedrock-us and bedrock-eu regions must differ to prove distinct entries")
	}
}

// TestLoadRegistry_MalformedYAMLReturnsError verifies a genuinely corrupt
// file is NOT silently treated as an empty registry (only file-absence is).
func TestLoadRegistry_MalformedYAMLReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "proxy-groups.yaml")
	if err := os.WriteFile(path, []byte("groups: [this is not a map"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := LoadRegistry(path); err == nil {
		t.Fatal("LoadRegistry(malformed) returned nil error, want parse error")
	}
}

// TestDefaultRegistryPath_ResolvesUnderHomeDir verifies cross-platform,
// $HOME-based path resolution (plan.md §C, CLAUDE.local.md §14 — no init-time
// absolute paths baked in).
func TestDefaultRegistryPath_ResolvesUnderHomeDir(t *testing.T) {
	// Not t.Parallel(): t.Setenv is incompatible with parallel subtests.
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir on Windows reads USERPROFILE; harmless no-op elsewhere.
	t.Setenv("USERPROFILE", home)

	path, err := DefaultRegistryPath()
	if err != nil {
		t.Fatalf("DefaultRegistryPath: %v", err)
	}
	want := filepath.Join(home, ".moai", "config", "proxy-groups.yaml")
	if path != want {
		t.Errorf("DefaultRegistryPath: got %q, want %q", path, want)
	}
}

// TestLoadDefaultRegistry_ResolvesAndLoads verifies LoadDefaultRegistry wires
// DefaultRegistryPath into LoadRegistry, and that a missing file under the
// resolved home directory still degrades to an empty registry.
func TestLoadDefaultRegistry_ResolvesAndLoads(t *testing.T) {
	// Not t.Parallel(): t.Setenv is incompatible with parallel subtests.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	reg, err := LoadDefaultRegistry()
	if err != nil {
		t.Fatalf("LoadDefaultRegistry: %v", err)
	}
	if reg == nil || len(reg.Groups) != 0 {
		t.Errorf("LoadDefaultRegistry: got %+v, want empty registry (no file yet)", reg)
	}

	// Now write a registry at the resolved default path and confirm the
	// second call actually reads it.
	path, err := DefaultRegistryPath()
	if err != nil {
		t.Fatalf("DefaultRegistryPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("groups:\n  a:\n    type: bedrock\nsets: {}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg2, err := LoadDefaultRegistry()
	if err != nil {
		t.Fatalf("LoadDefaultRegistry (after write): %v", err)
	}
	if len(reg2.Groups) != 1 {
		t.Errorf("LoadDefaultRegistry (after write): got %d groups, want 1", len(reg2.Groups))
	}
}

// TestResolveDefaultSet_EmptyRegistryListsNone verifies the error message's
// available-set listing degrades to a readable placeholder when the registry
// has no sets at all (rather than an empty string).
func TestResolveDefaultSet_EmptyRegistryListsNone(t *testing.T) {
	t.Parallel()

	reg := emptyRegistry()
	_, err := ResolveDefaultSet(reg, "")
	if err == nil {
		t.Fatal("ResolveDefaultSet(empty registry) returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "(none)") {
		t.Errorf("ResolveDefaultSet(empty registry) error = %q, want it to mention \"(none)\"", err.Error())
	}
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestGroup_SchemaCarriesNoCredentialFields verifies AC-PROXY-016 /
// REQ-PROXY-024 at the schema level: neither the machine registry's Group
// struct nor the project-side pointer carries a field shaped like a
// credential (token, API key, secret, password). Only references (region,
// profile NAME, base_url, group/alias names) are representable — a
// credential has nowhere to be written even by mistake.
func TestGroup_SchemaCarriesNoCredentialFields(t *testing.T) {
	t.Parallel()

	forbidden := []string{"token", "api_key", "apikey", "secret", "password", "credential"}
	typ := reflect.TypeOf(Group{})
	for i := 0; i < typ.NumField(); i++ {
		tag := strings.ToLower(typ.Field(i).Tag.Get("yaml"))
		for _, bad := range forbidden {
			if strings.Contains(tag, bad) {
				t.Errorf("Group field %q (yaml tag %q) looks credential-shaped — forbidden by REQ-PROXY-024", typ.Field(i).Name, tag)
			}
		}
	}
}
