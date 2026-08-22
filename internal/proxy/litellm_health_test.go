package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestProbeMessagesEndpoint_404IsInactive verifies the investigated
// detection method (research.md M2 gate item 6, progress.md §E.2 M2
// subsection): a LiteLLM instance that has NOT registered the
// Anthropic-compatible /v1/messages route returns 404 for it, and the
// probe reports the group inactive with a stated reason.
func TestProbeMessagesEndpoint_404IsInactive(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	active, reason, err := ProbeMessagesEndpoint(newTestClient(), backend.URL)
	if err != nil {
		t.Fatalf("ProbeMessagesEndpoint() error = %v", err)
	}
	if active {
		t.Error("ProbeMessagesEndpoint() active = true, want false on 404")
	}
	if reason == "" {
		t.Error("expected a non-empty reason when inactive")
	}
}

// TestProbeMessagesEndpoint_NonNotFoundIsActive verifies any non-404 status
// (200 success, 400 bad request from LiteLLM's own body validation, 401
// unauthorized) is treated as evidence the route IS registered and handled
// by LiteLLM's Anthropic-compatible handler — only a 404 means "no such
// route".
func TestProbeMessagesEndpoint_NonNotFoundIsActive(t *testing.T) {
	statuses := []int{http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized}
	for _, status := range statuses {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))

		active, _, err := ProbeMessagesEndpoint(newTestClient(), backend.URL)
		backend.Close()
		if err != nil {
			t.Fatalf("ProbeMessagesEndpoint() status=%d error = %v", status, err)
		}
		if !active {
			t.Errorf("ProbeMessagesEndpoint() status=%d active = false, want true", status)
		}
	}
}

// TestProbeMessagesEndpoint_UnreachableIsError verifies a genuinely
// unreachable base URL (connection refused) surfaces as an error, not a
// silent inactive classification — the caller can distinguish "instance is
// down" from "route not registered".
func TestProbeMessagesEndpoint_UnreachableIsError(t *testing.T) {
	_, _, err := ProbeMessagesEndpoint(newTestClient(), "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for unreachable base URL, got nil")
	}
}
