package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestServeCodex_MissingBaseURLIsIsolated verifies AC-PROXY-014's shape for
// the endpoint-unsettled case (plan.md M3 item 7): a codex group with no
// base_url configured is rejected with a clear reason naming the open
// investigation, rather than guessing an endpoint URL.
func TestServeCodex_MissingBaseURLIsIsolated(t *testing.T) {
	group := Group{Type: GroupTypeCodex} // no BaseURL
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))

	serveCodex(w, req, newTestClient(), group, []byte(`{"model":"m","messages":[]}`), "m", false)

	if w.Code < 400 {
		t.Errorf("status = %d, want a 4xx/5xx error", w.Code)
	}
	if !strings.Contains(w.Body.String(), "base_url") {
		t.Errorf("error body %q does not mention the missing base_url", w.Body.String())
	}
}

// TestServeCodex_CredentialReadFailureIsSurfaced verifies a credential-read
// failure at request time surfaces as an HTTP error naming the path,
// without ever attempting an interactive auth flow (AC-PROXY-013).
func TestServeCodex_CredentialReadFailureIsSurfaced(t *testing.T) {
	origPathFn := codexDefaultAuthFilePath
	defer func() { codexDefaultAuthFilePath = origPathFn }()

	missing := filepath.Join(t.TempDir(), "auth.json")
	codexDefaultAuthFilePath = func() (string, error) { return missing, nil }

	group := Group{Type: GroupTypeCodex, BaseURL: "http://127.0.0.1:1"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	serveCodex(w, req, newTestClient(), group, []byte(`{"model":"m","messages":[]}`), "m", false)

	if w.Code < 400 {
		t.Errorf("status = %d, want a 4xx/5xx error", w.Code)
	}
	if !strings.Contains(w.Body.String(), missing) {
		t.Errorf("error body %q does not name the attempted path %q", w.Body.String(), missing)
	}
}

// TestServeCodex_ValidCredentialsRouteThroughSharedTranslator verifies a
// successfully-read credential is applied as a bearer token (+
// chatgpt-account-id header) and the request flows through the SAME
// shared translator every OpenAI-shaped group uses (REQ-PROXY-018).
func TestServeCodex_ValidCredentialsRouteThroughSharedTranslator(t *testing.T) {
	origPathFn := codexDefaultAuthFilePath
	defer func() { codexDefaultAuthFilePath = origPathFn }()

	dir := t.TempDir()
	authPath := writeCodexAuthFile(t, dir, `{"auth_mode":"chatgpt","tokens":{"access_token":"acctok","account_id":"acct-1"}}`)
	codexDefaultAuthFilePath = func() (string, error) { return authPath, nil }

	var gotAuth, gotAccountID string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccountID = r.Header.Get("chatgpt-account-id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`))
	}))
	defer backend.Close()

	group := Group{Type: GroupTypeCodex, BaseURL: backend.URL}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	serveCodex(w, req, newTestClient(), group, []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`), "m", false)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer acctok" {
		t.Errorf("Authorization = %q, want Bearer acctok", gotAuth)
	}
	if gotAccountID != "acct-1" {
		t.Errorf("chatgpt-account-id = %q, want acct-1", gotAccountID)
	}
}

// TestServeCodex_NoInteractiveAuthFlowSourceCheck is a code-search-shaped
// check for AC-PROXY-013 ("자체 인증 흐름을 수행하지 않음"): the codex
// credential-acquisition path must never invoke a browser-opening or
// interactive-login mechanism — it only reads an existing CLI's storage.
// Verified by grepping the implementation files for the shapes such a flow
// would take. exec.LookPath (used only to detect whether the official
// `codex` binary is present, per plan.md M3 item 6) is explicitly NOT a
// flow-initiator and is allowed.
func TestServeCodex_NoInteractiveAuthFlowSourceCheck(t *testing.T) {
	forbidden := []string{"exec.Command(", "http.Get(\"https://", "browser", "OAuth", "oauth2."}
	for _, path := range []string{"codex_credentials.go", "codex_group.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		content := string(data)
		for _, pattern := range forbidden {
			if strings.Contains(content, pattern) {
				t.Errorf("%s contains %q — a possible interactive/auth-flow-initiating pattern", path, pattern)
			}
		}
	}
}

// TestEvaluateGroupStatusWithCodexProbe_IsolatesCredentialFailureOnly is
// AC-PROXY-014's direct unit-level analogue: group A (bedrock, healthy)
// and group B (codex, broken credential store) are both registered; after
// evaluation, A is active and B is inactive with a reason naming B's name
// and the attempted read path — A is unaffected.
func TestEvaluateGroupStatusWithCodexProbe_IsolatesCredentialFailureOnly(t *testing.T) {
	reg := &Registry{
		Groups: map[string]Group{
			"A": {Type: GroupTypeBedrock, Region: "us-east-1"},
			"B": {Type: GroupTypeCodex, BaseURL: "http://127.0.0.1:1"},
		},
	}
	missingPath := filepath.Join(t.TempDir(), "auth.json")
	probe := func(path string) (*CodexCredentials, error) {
		return ReadCodexPlaintextAuthFile(missingPath)
	}

	statuses := EvaluateGroupStatusWithCodexProbe(reg, probe)

	if !statuses["A"].Active {
		t.Errorf("group A status = %+v, want Active=true (unaffected by B's failure)", statuses["A"])
	}
	stB := statuses["B"]
	if stB.Active {
		t.Fatal("group B status.Active = true, want false (credential read failed)")
	}
	if !strings.Contains(stB.Reason, "B") {
		t.Errorf("group B reason %q does not name the group", stB.Reason)
	}
	if !strings.Contains(stB.Reason, missingPath) {
		t.Errorf("group B reason %q does not name the attempted read path %q", stB.Reason, missingPath)
	}
}

// TestEvaluateGroupStatusWithCodexProbe_MissingBaseURLIsolatesWithReason
// verifies a codex group with no base_url is isolated at evaluation time
// (not only at request time), naming the open endpoint investigation.
func TestEvaluateGroupStatusWithCodexProbe_MissingBaseURLIsolatesWithReason(t *testing.T) {
	reg := &Registry{
		Groups: map[string]Group{
			"nolocation": {Type: GroupTypeCodex}, // no BaseURL
		},
	}
	probe := func(path string) (*CodexCredentials, error) {
		return &CodexCredentials{AccessToken: "x", AccountID: "y"}, nil // credentials would succeed
	}
	statuses := EvaluateGroupStatusWithCodexProbe(reg, probe)
	st := statuses["nolocation"]
	if st.Active {
		t.Fatal("status.Active = true, want false (missing base_url)")
	}
	if !strings.Contains(st.Reason, "base_url") {
		t.Errorf("reason %q does not mention base_url", st.Reason)
	}
}

// TestEvaluateGroupStatusWithCodexProbe_HealthyCodexGroupStaysActive
// verifies a codex group with a valid base_url and a successful probe
// remains active.
func TestEvaluateGroupStatusWithCodexProbe_HealthyCodexGroupStaysActive(t *testing.T) {
	reg := &Registry{
		Groups: map[string]Group{
			"healthy": {Type: GroupTypeCodex, BaseURL: "http://example.invalid"},
		},
	}
	probe := func(path string) (*CodexCredentials, error) {
		return &CodexCredentials{AccessToken: "x", AccountID: "y"}, nil
	}
	statuses := EvaluateGroupStatusWithCodexProbe(reg, probe)
	if !statuses["healthy"].Active {
		t.Errorf("status = %+v, want Active=true", statuses["healthy"])
	}
}

// TestEvaluateGroupStatusWithCodexProbe_NonCodexGroupsUnaffected verifies
// the M1 type-support isolation (e.g. copilot) is preserved unchanged when
// composed with the M3 codex-credential probe.
func TestEvaluateGroupStatusWithCodexProbe_NonCodexGroupsUnaffected(t *testing.T) {
	reg := &Registry{
		Groups: map[string]Group{
			"cop": {Type: GroupTypeCopilot},
			"llm": {Type: GroupTypeLiteLLM, BaseURL: "http://x"},
		},
	}
	probe := func(path string) (*CodexCredentials, error) {
		t.Fatal("probe must not be called for non-codex groups")
		return nil, nil
	}
	statuses := EvaluateGroupStatusWithCodexProbe(reg, probe)
	if statuses["cop"].Active {
		t.Error("copilot group must remain inactive (M1 type-support isolation, unchanged)")
	}
	if !statuses["llm"].Active {
		t.Error("litellm group must remain active (unaffected by the codex probe)")
	}
}
