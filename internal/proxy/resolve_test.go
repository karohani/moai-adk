package proxy

import "testing"

func registryWithSets() *Registry {
	return &Registry{
		Groups: map[string]Group{
			"bedrock-us":    {Type: GroupTypeBedrock},
			"gpu-cluster-a": {Type: GroupTypeOpenAICompatible},
		},
		Sets: map[string][]string{
			"default": {"bedrock-us"},
			"cheap":   {"gpu-cluster-a"},
		},
	}
}

// TestResolveDefaultSet_ProjectPointerWins verifies AC-PROXY-005c branch 3 and
// REQ-PROXY-010: when both a project default_set and a machine sets.default
// exist with different content, the project preference wins.
func TestResolveDefaultSet_ProjectPointerWins(t *testing.T) {
	t.Parallel()

	reg := registryWithSets()
	got, err := ResolveDefaultSet(reg, "cheap")
	if err != nil {
		t.Fatalf("ResolveDefaultSet: %v", err)
	}
	want := []string{"gpu-cluster-a"}
	if !equalStrSlices(got, want) {
		t.Errorf("ResolveDefaultSet(project=cheap): got %v, want %v", got, want)
	}
}

// TestResolveDefaultSet_FallsBackToMachineDefault verifies AC-PROXY-005c
// branch 1: no project pointer → machine sets.default.
func TestResolveDefaultSet_FallsBackToMachineDefault(t *testing.T) {
	t.Parallel()

	reg := registryWithSets()
	got, err := ResolveDefaultSet(reg, "")
	if err != nil {
		t.Fatalf("ResolveDefaultSet: %v", err)
	}
	want := []string{"bedrock-us"}
	if !equalStrSlices(got, want) {
		t.Errorf("ResolveDefaultSet(project=\"\"): got %v, want %v", got, want)
	}
}

// TestResolveDefaultSet_NoDefaultAnywhereIsError verifies AC-PROXY-005c
// branch 2: neither layer has a default → error listing available sets, not
// a silent empty result.
func TestResolveDefaultSet_NoDefaultAnywhereIsError(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Groups: map[string]Group{"gpu-cluster-a": {Type: GroupTypeOpenAICompatible}},
		Sets:   map[string][]string{"cheap": {"gpu-cluster-a"}},
	}
	_, err := ResolveDefaultSet(reg, "")
	if err == nil {
		t.Fatal("ResolveDefaultSet(no default set) returned nil error, want error")
	}
}

// TestResolveDefaultSet_MissingProjectPointerIsHardError verifies
// AC-PROXY-005f and REQ-PROXY-025: a project pointer to a set name absent
// from the machine registry MUST error — it must NOT silently fall back to
// the machine sets.default even when one exists.
func TestResolveDefaultSet_MissingProjectPointerIsHardError(t *testing.T) {
	t.Parallel()

	reg := registryWithSets() // has sets.default = [bedrock-us]
	_, err := ResolveDefaultSet(reg, "missing")
	if err == nil {
		t.Fatal("ResolveDefaultSet(project=missing) returned nil error, want hard error (no silent fallback)")
	}
}
