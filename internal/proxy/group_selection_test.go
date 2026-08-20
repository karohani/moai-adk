package proxy

import "testing"

func groupSelectionFixtureRegistry() *Registry {
	return &Registry{
		Groups: map[string]Group{
			"work":     {Type: GroupTypeLiteLLM, BaseURL: "http://localhost:4000"},
			"personal": {Type: GroupTypeBedrock, Region: "us-east-1"},
			"backup":   {Type: GroupTypeCodex},
		},
		Sets: map[string][]string{
			"default": {"work"},
			"heavy":   {"work", "personal"},
		},
	}
}

// TestResolveActiveGroups_GFlagAndSetFlagAreMutuallyExclusive verifies
// REQ-PROXY-011: passing both -g and --set is a hard error.
func TestResolveActiveGroups_GFlagAndSetFlagAreMutuallyExclusive(t *testing.T) {
	reg := groupSelectionFixtureRegistry()
	_, err := ResolveActiveGroups(reg, []string{"work"}, "heavy", "")
	if err == nil {
		t.Fatal("expected mutual-exclusion error, got nil")
	}
}

// TestResolveActiveGroups_GFlagSelectsNamedGroups verifies REQ-PROXY-008:
// -g <name> activates exactly the named group(s).
func TestResolveActiveGroups_GFlagSelectsNamedGroups(t *testing.T) {
	reg := groupSelectionFixtureRegistry()
	got, err := ResolveActiveGroups(reg, []string{"personal", "backup"}, "", "")
	if err != nil {
		t.Fatalf("ResolveActiveGroups() error = %v", err)
	}
	if len(got) != 2 || got[0] != "personal" || got[1] != "backup" {
		t.Errorf("ResolveActiveGroups() = %v, want [personal backup]", got)
	}
}

// TestResolveActiveGroups_GFlagUnknownGroupIsError verifies -g rejects a
// group name that does not exist in the machine registry.
func TestResolveActiveGroups_GFlagUnknownGroupIsError(t *testing.T) {
	reg := groupSelectionFixtureRegistry()
	_, err := ResolveActiveGroups(reg, []string{"nonexistent"}, "", "")
	if err == nil {
		t.Fatal("expected unknown-group error, got nil")
	}
}

// TestResolveActiveGroups_SetFlagSelectsNamedSet verifies REQ-PROXY-009:
// --set <name> activates the named set's group list.
func TestResolveActiveGroups_SetFlagSelectsNamedSet(t *testing.T) {
	reg := groupSelectionFixtureRegistry()
	got, err := ResolveActiveGroups(reg, nil, "heavy", "")
	if err != nil {
		t.Fatalf("ResolveActiveGroups() error = %v", err)
	}
	if len(got) != 2 || got[0] != "work" || got[1] != "personal" {
		t.Errorf("ResolveActiveGroups() = %v, want [work personal]", got)
	}
}

// TestResolveActiveGroups_SetFlagUnknownSetIsError verifies --set rejects
// an unknown set name.
func TestResolveActiveGroups_SetFlagUnknownSetIsError(t *testing.T) {
	reg := groupSelectionFixtureRegistry()
	_, err := ResolveActiveGroups(reg, nil, "nonexistent-set", "")
	if err == nil {
		t.Fatal("expected unknown-set error, got nil")
	}
}

// TestResolveActiveGroups_NeitherFlagFallsBackToResolveDefaultSet verifies
// REQ-PROXY-010/025: with neither -g nor --set, ResolveActiveGroups wires
// through to the ALREADY-IMPLEMENTED M1 ResolveDefaultSet resolution order
// (project default_set -> machine sets.default -> error).
func TestResolveActiveGroups_NeitherFlagFallsBackToResolveDefaultSet(t *testing.T) {
	reg := groupSelectionFixtureRegistry()

	// No project pointer -> falls back to machine sets.default ("default" -> [work]).
	got, err := ResolveActiveGroups(reg, nil, "", "")
	if err != nil {
		t.Fatalf("ResolveActiveGroups() error = %v", err)
	}
	if len(got) != 1 || got[0] != "work" {
		t.Errorf("ResolveActiveGroups() = %v, want [work] via machine default", got)
	}

	// Project pointer naming a real set wins over the machine default.
	got2, err := ResolveActiveGroups(reg, nil, "", "heavy")
	if err != nil {
		t.Fatalf("ResolveActiveGroups() error = %v", err)
	}
	if len(got2) != 2 || got2[0] != "work" || got2[1] != "personal" {
		t.Errorf("ResolveActiveGroups() = %v, want [work personal] via project pointer", got2)
	}
}

// TestResolveActiveGroups_PropagatesResolveDefaultSetHardError verifies the
// fallback path still surfaces ResolveDefaultSet's hard-error behavior for
// a project pointer naming a nonexistent set (no silent fallback).
func TestResolveActiveGroups_PropagatesResolveDefaultSetHardError(t *testing.T) {
	reg := groupSelectionFixtureRegistry()
	_, err := ResolveActiveGroups(reg, nil, "", "nonexistent")
	if err == nil {
		t.Fatal("expected hard error for missing project pointer, got nil")
	}
	if err.Error() == "" {
		t.Error("expected a non-empty error message")
	}
}
