package proxy

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// TestMessagesHandler_RoutesToOpenAICompatible verifies a model resolving
// to an openai-compatible group is translated and relayed through the
// shared translator (REQ-PROXY-018), with no auth header applied (v1
// scope: openai-compatible sends no additional auth — plan.md M3 item 4).
func TestMessagesHandler_RoutesToOpenAICompatible(t *testing.T) {
	var gotAuth string
	var gotBody map[string]interface{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`))
	}))
	defer backend.Close()
	_ = gotBody

	reg := &Registry{
		Groups: map[string]Group{
			"gpu": {Type: GroupTypeOpenAICompatible, BaseURL: backend.URL, Models: []string{"local-model"}},
		},
	}
	cat := NewCatalog(reg, []string{"gpu"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	body := `{"model":"gpu/local-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty (openai-compatible sends no auth in v1)", gotAuth)
	}
}

// TestMessagesHandler_RoutesToCodex verifies a model resolving to a codex
// group is translated and relayed through the shared translator, with the
// credential read from the (test-overridden) plaintext auth file.
func TestMessagesHandler_RoutesToCodex(t *testing.T) {
	origPathFn := codexDefaultAuthFilePath
	defer func() { codexDefaultAuthFilePath = origPathFn }()

	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	writeCodexAuthFile(t, dir, `{"auth_mode":"chatgpt","tokens":{"access_token":"acctok","account_id":"acct-1"}}`)
	codexDefaultAuthFilePath = func() (string, error) { return authPath, nil }

	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`))
	}))
	defer backend.Close()

	reg := &Registry{
		Groups: map[string]Group{
			"cx": {Type: GroupTypeCodex, BaseURL: backend.URL, Models: []string{"gpt-5-codex"}},
		},
	}
	cat := NewCatalog(reg, []string{"cx"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	body := `{"model":"cx/gpt-5-codex","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer acctok" {
		t.Errorf("Authorization = %q, want Bearer acctok", gotAuth)
	}
}

// TestMessagesHandler_UnwiredCopilotGroupTypeIsNotImplemented verifies
// copilot — explicitly out of v1 scope (plan.md M3 item 8) — still returns
// 501 through MessagesHandler, replacing the M2-era codex placeholder now
// that codex is wired.
func TestMessagesHandler_UnwiredCopilotGroupTypeIsNotImplemented(t *testing.T) {
	reg := &Registry{
		Groups: map[string]Group{
			"cop": {Type: GroupTypeCopilot},
		},
	}
	cat := NewCatalog(reg, []string{"cop"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	body := `{"model":"cop/whatever","messages":[]}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501 (copilot is v1-out-of-scope)", w.Code)
	}
}
