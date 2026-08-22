package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServeOpenAIShapedMessages_NonStreamingTranslatesRequestAndResponse
// verifies the shared handler (REQ-PROXY-018's single translation entry
// point) round-trips a non-streaming Anthropic request through
// ToOpenAIChatRequest -> backend -> FromOpenAIChatResponse.
func TestServeOpenAIShapedMessages_NonStreamingTranslatesRequestAndResponse(t *testing.T) {
	var gotAuth string
	var gotReq map[string]interface{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotReq)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	}))
	defer backend.Close()

	anthropicBody := []byte(`{"model":"gpu-cluster-model","messages":[{"role":"user","content":"hello"}]}`)
	auth := func(req *http.Request) error {
		req.Header.Set("Authorization", "Bearer test-token")
		return nil
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(string(anthropicBody)))
	serveOpenAIShapedMessages(w, req, newTestClient(), backend.URL, auth, anthropicBody, "gpu-cluster-model", false)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("backend received Authorization = %q, want Bearer test-token", gotAuth)
	}
	if gotReq["model"] != "gpu-cluster-model" {
		t.Errorf("backend received model = %v, want gpu-cluster-model", gotReq["model"])
	}

	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	content := got["content"].([]interface{})
	block := content[0].(map[string]interface{})
	if block["text"] != "hi there" {
		t.Errorf("translated response text = %v, want %q", block["text"], "hi there")
	}
}

// TestServeOpenAIShapedMessages_StreamingRelaysTranslatedSSE verifies the
// streaming path relays the backend's OpenAI-shaped SSE through the shared
// translator to the client as Anthropic-shaped SSE.
func TestServeOpenAIShapedMessages_StreamingRelaysTranslatedSSE(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer backend.Close()

	anthropicBody := []byte(`{"model":"m","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	auth := func(req *http.Request) error { return nil }

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(string(anthropicBody)))
	serveOpenAIShapedMessages(w, req, newTestClient(), backend.URL, auth, anthropicBody, "m", true)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", w.Header().Get("Content-Type"))
	}
	events := sseEvents(t, w.Body.Bytes())
	if len(events) == 0 || events[0].Event != "message_start" {
		t.Errorf("first event = %+v, want message_start", events[0])
	}
	if events[len(events)-1].Event != "message_stop" {
		t.Errorf("last event = %+v, want message_stop", events[len(events)-1])
	}
}

// TestServeOpenAIShapedMessages_AuthErrorIsSurfaced verifies an
// authenticator failure (e.g. a credential-read failure) surfaces as an
// HTTP error rather than proceeding with an unauthenticated request.
func TestServeOpenAIShapedMessages_AuthErrorIsSurfaced(t *testing.T) {
	auth := func(req *http.Request) error {
		return errCredentialUnavailableForTest
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	serveOpenAIShapedMessages(w, req, newTestClient(), "http://127.0.0.1:1", auth, []byte(`{"model":"m","messages":[]}`), "m", false)

	if w.Code < 400 {
		t.Errorf("status = %d, want a 4xx/5xx error", w.Code)
	}
}

// TestServeOpenAIShapedMessages_BackendUnreachableIsBadGateway verifies a
// backend connection failure surfaces as 502.
func TestServeOpenAIShapedMessages_BackendUnreachableIsBadGateway(t *testing.T) {
	auth := func(req *http.Request) error { return nil }
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	serveOpenAIShapedMessages(w, req, newTestClient(), "http://127.0.0.1:1", auth, []byte(`{"model":"m","messages":[]}`), "m", false)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

var errCredentialUnavailableForTest = &credentialTestError{"simulated credential failure"}

type credentialTestError struct{ msg string }

func (e *credentialTestError) Error() string { return e.msg }
