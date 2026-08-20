package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDaemon_HoldsAllRegisteredGroupsRegardlessOfSessionActiveSet is
// AC-PROXY-002 (REQ-PROXY-004, design.md §2.1/§2.2): the daemon holds
// EVERY group in the machine registry, independent of which subset a given
// session activated via -g/--set. Given registry groups A and B, and a
// session that started the daemon with `-g A` (activeGroups = ["A"]
// only), a direct `<group>/<model>` reference to B — a group NOT in that
// session's active set — MUST still route successfully, because the
// daemon's Registry (constructed once at daemon start, shared across every
// project/session on the machine) is the source of truth for "what groups
// exist", while Catalog.activeGroups only scopes ALIAS resolution
// (REQ-PROXY-015 direct references always bypass the active-set filter).
func TestDaemon_HoldsAllRegisteredGroupsRegardlessOfSessionActiveSet(t *testing.T) {
	var groupBHit bool
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groupBHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backendB.Close()

	reg := &Registry{
		Groups: map[string]Group{
			"A": {Type: GroupTypeBedrock, Region: "us-east-1"},
			"B": {Type: GroupTypeLiteLLM, BaseURL: backendB.URL},
		},
	}

	// Session activated ONLY group A via `moai proxy -g A`.
	sessionActive, err := ResolveActiveGroups(reg, []string{"A"}, "", "")
	if err != nil {
		t.Fatalf("ResolveActiveGroups() error = %v", err)
	}
	if len(sessionActive) != 1 || sessionActive[0] != "A" {
		t.Fatalf("session active set = %v, want [A]", sessionActive)
	}

	// The daemon's own Registry — constructed once, shared machine-wide —
	// still names BOTH groups, independent of this session's active set.
	if _, ok := reg.Groups["A"]; !ok {
		t.Error("daemon registry must retain group A")
	}
	if _, ok := reg.Groups["B"]; !ok {
		t.Error("daemon registry must retain group B, even though this session only requested A")
	}

	// A direct reference to B (not in this session's active set) must
	// still route successfully through the daemon's handler, because
	// direct references bypass alias-scoped active-group filtering
	// (REQ-PROXY-015) — proving the daemon actually HOLDS group B, not
	// merely lists it.
	cat := NewCatalog(reg, sessionActive)
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"B/some-model","messages":[]}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("routing to group B (outside session active set) status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !groupBHit {
		t.Error("expected the request to reach group B's backend")
	}
}
