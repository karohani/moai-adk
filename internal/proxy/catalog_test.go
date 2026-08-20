package proxy

import (
	"testing"
)

func testRegistry() *Registry {
	return &Registry{
		Groups: map[string]Group{
			"bedrock-us": {
				Type: GroupTypeBedrock,
				Aliases: AliasBlock{
					Opus:   "us.anthropic.claude-opus",
					Sonnet: "us.anthropic.claude-sonnet",
				},
			},
			"gpu-cluster-a": {
				Type: GroupTypeOpenAICompatible,
				Aliases: AliasBlock{
					Opus: "qwen-3-72b",
				},
			},
			"github-copilot": {
				Type: GroupTypeCopilot,
			},
		},
		Sets: map[string][]string{
			"default": {"bedrock-us", "gpu-cluster-a"},
		},
	}
}

// TestResolveAlias_ReturnsOrderedDeploymentList verifies AC-PROXY-007a and
// REQ-PROXY-013: alias resolution always returns a []Deployment slice, even
// when only one active group declares the alias. Order follows the active
// group set's order.
func TestResolveAlias_ReturnsOrderedDeploymentList(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	cat := NewCatalog(reg, []string{"bedrock-us", "gpu-cluster-a"})

	got, err := cat.ResolveAlias("opus")
	if err != nil {
		t.Fatalf("ResolveAlias(opus): %v", err)
	}
	want := []Deployment{
		{Group: "bedrock-us", Model: "us.anthropic.claude-opus"},
		{Group: "gpu-cluster-a", Model: "qwen-3-72b"},
	}
	if len(got) != len(want) {
		t.Fatalf("ResolveAlias(opus) length: got %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ResolveAlias(opus)[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestResolveAlias_SingleDeploymentIsStillASlice verifies REQ-PROXY-013's
// forward-compat constraint holds even at length 1 — the AC explicitly notes
// "even at length 1" must remain a slice, not a scalar.
func TestResolveAlias_SingleDeploymentIsStillASlice(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	cat := NewCatalog(reg, []string{"gpu-cluster-a"})

	got, err := cat.ResolveAlias("opus")
	if err != nil {
		t.Fatalf("ResolveAlias(opus): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ResolveAlias(opus) length: got %d, want 1", len(got))
	}
	if got[0] != (Deployment{Group: "gpu-cluster-a", Model: "qwen-3-72b"}) {
		t.Errorf("ResolveAlias(opus)[0]: got %+v", got[0])
	}
}

// TestResolveAlias_NoActiveGroupDeclaresAlias verifies the error path when no
// active group in the set declares the requested alias.
func TestResolveAlias_NoActiveGroupDeclaresAlias(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	cat := NewCatalog(reg, []string{"bedrock-us"})

	if _, err := cat.ResolveAlias("haiku"); err == nil {
		t.Fatal("ResolveAlias(haiku) returned nil error, want error (no group declares haiku)")
	}
}

// TestSelectRoute_ReadsFirstDeployment verifies AC-PROXY-007b and
// REQ-PROXY-014: v1 routing reads deployments[0] exclusively — 100 calls all
// resolve to the first entry, never the second.
func TestSelectRoute_ReadsFirstDeployment(t *testing.T) {
	t.Parallel()

	deployments := []Deployment{
		{Group: "bedrock-us", Model: "opus-model"},
		{Group: "gpu-cluster-a", Model: "qwen-model"},
	}

	for i := 0; i < 100; i++ {
		got, err := SelectRoute(deployments)
		if err != nil {
			t.Fatalf("SelectRoute call %d: %v", i, err)
		}
		if got.Group != "bedrock-us" {
			t.Fatalf("SelectRoute call %d: got group %q, want %q", i, got.Group, "bedrock-us")
		}
	}
}

// TestSelectRoute_EmptyListIsError verifies SelectRoute rejects an empty
// deployment list rather than panicking or returning a zero-value silently.
func TestSelectRoute_EmptyListIsError(t *testing.T) {
	t.Parallel()

	if _, err := SelectRoute(nil); err == nil {
		t.Fatal("SelectRoute(nil) returned nil error, want error")
	}
}

// TestResolveDirect_BypassesAliasResolution verifies AC-PROXY-008 and
// REQ-PROXY-015: a "<group>/<model>" identifier maps directly to the named
// group without consulting alias blocks.
func TestResolveDirect_BypassesAliasResolution(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	cat := NewCatalog(reg, []string{"bedrock-us", "gpu-cluster-a"})

	got, err := cat.Resolve("gpu-cluster-a/custom-model-not-in-aliases")
	if err != nil {
		t.Fatalf("Resolve(direct): %v", err)
	}
	want := Deployment{Group: "gpu-cluster-a", Model: "custom-model-not-in-aliases"}
	if got != want {
		t.Errorf("Resolve(direct): got %+v, want %+v", got, want)
	}
}

// TestResolveDirect_UnknownGroupIsError verifies a direct reference to a
// group absent from the registry fails rather than silently no-op-ing.
func TestResolveDirect_UnknownGroupIsError(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	cat := NewCatalog(reg, []string{"bedrock-us"})

	if _, err := cat.Resolve("does-not-exist/some-model"); err == nil {
		t.Fatal("Resolve(direct, unknown group) returned nil error, want error")
	}
}

// TestResolve_AliasShapeRoutesThroughSelectRoute verifies the end-to-end
// Resolve() dispatch: a non-"<group>/<model>" model identifier resolves via
// alias resolution + v1 index-0 routing (design.md §4.3).
func TestResolve_AliasShapeRoutesThroughSelectRoute(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	cat := NewCatalog(reg, []string{"bedrock-us", "gpu-cluster-a"})

	got, err := cat.Resolve("opus")
	if err != nil {
		t.Fatalf("Resolve(opus): %v", err)
	}
	want := Deployment{Group: "bedrock-us", Model: "us.anthropic.claude-opus"}
	if got != want {
		t.Errorf("Resolve(opus): got %+v, want %+v", got, want)
	}
}

// TestCatalogName_GroupModelFormat verifies AC-PROXY-006 and REQ-PROXY-012:
// catalog names use "<group>/<model>", not "<type>/<model>" — so two groups
// of the same type serving the same model identifier do not collide.
func TestCatalogName_GroupModelFormat(t *testing.T) {
	t.Parallel()

	got := CatalogName("bedrock-us", "anthropic.claude-opus-5")
	want := "bedrock-us/anthropic.claude-opus-5"
	if got != want {
		t.Errorf("CatalogName: got %q, want %q", got, want)
	}
}

// TestGroupType_SupportedInV1 verifies plan.md M1.8 / spec.md §H: all five
// group types are valid enum values, but only four have a v1 adapter.
func TestGroupType_SupportedInV1(t *testing.T) {
	t.Parallel()

	cases := []struct {
		gt   GroupType
		want bool
	}{
		{GroupTypeBedrock, true},
		{GroupTypeCodex, true},
		{GroupTypeLiteLLM, true},
		{GroupTypeOpenAICompatible, true},
		{GroupTypeCopilot, false},
	}
	for _, tc := range cases {
		if got := tc.gt.SupportedInV1(); got != tc.want {
			t.Errorf("GroupType(%q).SupportedInV1(): got %v, want %v", tc.gt, got, tc.want)
		}
	}
}

// TestAliasBlock_AllFourAliasesResolve verifies all four fixed agent
// model-tier aliases (opus/sonnet/haiku/fable — the fixed 5-alias vocabulary
// minus "inherit", which has no backend model) resolve through AliasBlock,
// and an unknown alias name resolves to "".
func TestAliasBlock_AllFourAliasesResolve(t *testing.T) {
	t.Parallel()

	block := AliasBlock{Opus: "opus-model", Sonnet: "sonnet-model", Haiku: "haiku-model", Fable: "fable-model"}

	cases := []struct {
		alias string
		want  string
	}{
		{"opus", "opus-model"},
		{"sonnet", "sonnet-model"},
		{"haiku", "haiku-model"},
		{"fable", "fable-model"},
		{"inherit", ""},
	}
	for _, tc := range cases {
		if got := block.modelFor(tc.alias); got != tc.want {
			t.Errorf("modelFor(%q): got %q, want %q", tc.alias, got, tc.want)
		}
	}
}

// TestResolveDirect_MalformedIdentifierIsError verifies a model identifier
// without a "/" separator is rejected rather than mis-parsed.
func TestResolveDirect_MalformedIdentifierIsError(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	cat := NewCatalog(reg, []string{"bedrock-us"})

	if _, err := cat.ResolveDirect("not-a-group-model-pair"); err == nil {
		t.Fatal("ResolveDirect(malformed) returned nil error, want error")
	}
}

// TestEvaluateGroupStatus_IsolatesUnsupportedTypeGroupsOnly verifies plan.md
// M1.8: a group configured with an unsupported v1 type (copilot) is isolated
// as inactive with a clear reason, while every other group keeps serving —
// the same isolation shape as REQ-PROXY-021.
func TestEvaluateGroupStatus_IsolatesUnsupportedTypeGroupsOnly(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	statuses := EvaluateGroupStatus(reg)

	if len(statuses) != 3 {
		t.Fatalf("EvaluateGroupStatus: got %d entries, want 3", len(statuses))
	}

	bedrock := statuses["bedrock-us"]
	if !bedrock.Active {
		t.Errorf("bedrock-us: got Active=false, want true")
	}
	gpu := statuses["gpu-cluster-a"]
	if !gpu.Active {
		t.Errorf("gpu-cluster-a: got Active=false, want true")
	}

	copilot := statuses["github-copilot"]
	if copilot.Active {
		t.Errorf("github-copilot: got Active=true, want false (type not supported in v1)")
	}
	if copilot.Reason == "" {
		t.Error("github-copilot: Reason is empty, want a clear 'type not supported in v1' explanation")
	}
}
