package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
)

// erroringBedrockInvoker always fails, letting tests exercise the
// serveBedrock error-response branches (both non-streaming and streaming).
type erroringBedrockInvoker struct{}

func (erroringBedrockInvoker) Invoke(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, errors.New("bedrock: simulated failure")
}

func (erroringBedrockInvoker) InvokeStream(_ context.Context, _ string, _ []byte) (io.ReadCloser, error) {
	return nil, errors.New("bedrock: simulated streaming failure")
}

func newBodyReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

// TestMessagesHandler_RoutesToBedrockStreaming verifies the streaming
// branch of serveBedrock: a stream:true request resolving to a bedrock
// group is relayed via InvokeStream and flushed to the client as
// text/event-stream.
func TestMessagesHandler_RoutesToBedrockStreaming(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"personal"})
	fake := &fakeBedrockInvoker{streamResp: "event: message_start\ndata: {}\n\n"}
	h, err := NewMessagesHandler(reg, cat, fake)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":  "personal/anthropic.claude-3-5-sonnet-20241022-v2:0",
		"stream": true,
	})
	req := httptest.NewRequest("POST", "/v1/messages", newBodyReader(reqBody))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if fake.invokeStreamCalls != 1 {
		t.Errorf("InvokeStream called %d times, want 1", fake.invokeStreamCalls)
	}
	if w.Body.String() != fake.streamResp {
		t.Errorf("streamed body = %q, want %q", w.Body.String(), fake.streamResp)
	}
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", w.Header().Get("Content-Type"))
	}
}

// TestMessagesHandler_BedrockNonStreamingErrorIsBadGateway verifies a
// bedrock invoker failure on the non-streaming path surfaces as 502.
func TestMessagesHandler_BedrockNonStreamingErrorIsBadGateway(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"personal"})
	h, err := NewMessagesHandler(reg, cat, erroringBedrockInvoker{})
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	reqBody := `{"model":"personal/anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[]}`
	req := httptest.NewRequest("POST", "/v1/messages", newBodyReader([]byte(reqBody)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 502 {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

// TestMessagesHandler_BedrockStreamingErrorIsBadGateway verifies a bedrock
// invoker failure on the streaming path surfaces as 502.
func TestMessagesHandler_BedrockStreamingErrorIsBadGateway(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"personal"})
	h, err := NewMessagesHandler(reg, cat, erroringBedrockInvoker{})
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	reqBody := `{"model":"personal/anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[],"stream":true}`
	req := httptest.NewRequest("POST", "/v1/messages", newBodyReader([]byte(reqBody)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 502 {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

// TestMessagesHandler_BedrockGroupWithNoInvokerConfigured verifies routing
// to a bedrock group when the handler was constructed with a nil
// BedrockInvoker surfaces a clear 500, not a panic.
func TestMessagesHandler_BedrockGroupWithNoInvokerConfigured(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"personal"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	reqBody := `{"model":"personal/anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[]}`
	req := httptest.NewRequest("POST", "/v1/messages", newBodyReader([]byte(reqBody)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestMessagesHandler_MethodNotAllowed verifies a non-POST request is
// rejected with 405.
func TestMessagesHandler_MethodNotAllowed(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"work"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/messages", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

// TestMessagesHandler_MissingModelFieldIsBadRequest verifies a body with no
// "model" field is rejected at the boundary rather than reaching Resolve.
func TestMessagesHandler_MissingModelFieldIsBadRequest(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"work"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/messages", newBodyReader([]byte(`{"messages":[]}`)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestMessagesHandler_InvalidJSONBodyIsBadRequest verifies malformed JSON
// is rejected before any routing attempt.
func TestMessagesHandler_InvalidJSONBodyIsBadRequest(t *testing.T) {
	reg := handlerFixtureRegistry("")
	cat := NewCatalog(reg, []string{"work"})
	h, err := NewMessagesHandler(reg, cat, nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler() error = %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/messages", newBodyReader([]byte(`{not json`)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
