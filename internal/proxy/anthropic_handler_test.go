package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func handlerFixtureRegistry(litellmBaseURL string) *Registry {
	return &Registry{
		Groups: map[string]Group{
			"work": {
				Type:    GroupTypeLiteLLM,
				BaseURL: litellmBaseURL,
				Models:  []string{"claude-3-5-sonnet"},
			},
			"personal": {
				Type:   GroupTypeBedrock,
				Region: "us-east-1",
				Models: []string{"anthropic.claude-3-5-sonnet-20241022-v2:0"},
			},
			"codex-backend": {
				Type:   GroupTypeCodex,
				Models: []string{"gpt-5-codex"},
			},
			"cop": {
				Type:   GroupTypeCopilot,
				Models: []string{"whatever"},
			},
		},
	}
}

// TestMessagesHandler_RoutesToLiteLLM verifies a request naming a
// litellm-group model is passthrough-relayed to that group's backend.
func TestMessagesHandler_RoutesToLiteLLM(t *testing.T) {
	var gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	reg := handlerFixtureRegistry(backend.URL)
	cat := NewCatalog(reg, []string{"work", "personal", "codex-backend"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	body := `{"model":"work/claude-3-5-sonnet","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotBody != body {
		t.Errorf("backend received %q, want %q (litellm is pure passthrough)", gotBody, body)
	}
}

// TestMessagesHandler_RoutesToBedrock verifies a request naming a
// bedrock-group model is translated and delegated to the injected
// BedrockInvoker.
func TestMessagesHandler_RoutesToBedrock(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"work", "personal", "codex-backend"})
	fake := &fakeBedrockInvoker{invokeResp: []byte(`{"id":"msg_1","content":[{"type":"text","text":"hi"}]}`)}
	h, err := NewMessagesHandler(reg, cat, fake)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	body := `{"model":"personal/anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[{"role":"user","content":"hi"}],"max_tokens":100}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if fake.invokeCalls != 1 {
		t.Errorf("Invoke called %d times, want 1", fake.invokeCalls)
	}
	if fake.gotModelID != "anthropic.claude-3-5-sonnet-20241022-v2:0" {
		t.Errorf("gotModelID = %q, want the resolved Bedrock model ID", fake.gotModelID)
	}
	if !bytes.Equal(w.Body.Bytes(), fake.invokeResp) {
		t.Errorf("response body = %s, want %s", w.Body.Bytes(), fake.invokeResp)
	}
}

// TestMessagesHandler_UnknownModelIsBadRequest verifies an unresolvable
// model identifier surfaces as a 4xx client error, not a 5xx or a silent
// misroute.
func TestMessagesHandler_UnknownModelIsBadRequest(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"work"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	body := `{"model":"nonexistent-alias","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code < 400 || w.Code >= 500 {
		t.Errorf("status = %d, want a 4xx client error", w.Code)
	}
}

// TestMessagesHandler_UnwiredGroupTypeIsNotImplemented verifies a group
// type explicitly OUT of v1 scope (copilot — plan.md M3 item 8, spec.md
// §H) returns 501, never a silent success. codex was M2's placeholder for
// this test; now that codex is wired (M3), copilot is the correct
// unwired-type fixture — see also
// TestMessagesHandler_UnwiredCopilotGroupTypeIsNotImplemented in
// anthropic_handler_openai_test.go for the M3-authored duplicate covering
// the same scenario from the M3 delegation's own fixture.
func TestMessagesHandler_UnwiredGroupTypeIsNotImplemented(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"cop"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	body := `{"model":"cop/whatever","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501 (codex group type unwired in M2)", w.Code)
	}
}

// TestEndToEnd_ClaudeCodeConversationViaProxy is the mechanical equivalent
// of AC-PROXY-009 (item 10 — an actual Claude Code conversation routed
// through moai proxy): a full SSE round trip through StartServer +
// MessagesHandler + a litellm-shaped httptest backend that emits the same
// event sequence a real Anthropic-compatible streaming response would
// (message_start -> content_block_delta* -> message_stop), asserting event
// order and assembled text. A REAL Claude Code CLI conversation requires an
// interactive client and a live backend, neither available in this
// environment — that leg is reported as a Gap in progress.md §E.2, not
// claimed as PASS.
func TestEndToEnd_ClaudeCodeConversationViaProxy(t *testing.T) {
	sseBody := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"Hello\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\", world\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseBody))
	}))
	defer backend.Close()

	reg := handlerFixtureRegistry(backend.URL)
	cat := NewCatalog(reg, []string{"work"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	addr, stop, err := StartServer(h)
	if err != nil {
		t.Fatalf("StartServer() error = %v", err)
	}
	defer func() { _ = stop() }()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":    "work/claude-3-5-sonnet",
		"stream":   true,
		"messages": []map[string]string{{"role": "user", "content": "say hi"}},
	})

	resp, err := newTestClient().Post("http://"+addr+"/v1/messages", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST /v1/messages error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response error = %v", err)
	}
	if string(got) != sseBody {
		t.Errorf("SSE round trip body = %q, want %q (verbatim relay through the daemon)", got, sseBody)
	}

	// Assemble the text deltas to confirm event order survived the relay.
	assembled := ""
	for _, line := range strings.Split(string(got), "\n") {
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, "content_block_delta") {
			var evt struct {
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt); err == nil {
				assembled += evt.Delta.Text
			}
		}
	}
	if assembled != "Hello, world" {
		t.Errorf("assembled streamed text = %q, want %q", assembled, "Hello, world")
	}
}
