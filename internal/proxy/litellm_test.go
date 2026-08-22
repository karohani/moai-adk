package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewLiteLLMProxy_RelaysBodySemanticallyIdentical verifies REQ-PROXY-017:
// the litellm group is pure passthrough — no body translation. The backend
// must observe the SAME body the client sent.
func TestNewLiteLLMProxy_RelaysBodySemanticallyIdentical(t *testing.T) {
	var gotBody string
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	proxy, err := NewLiteLLMProxy(backend.URL)
	if err != nil {
		t.Fatalf("NewLiteLLMProxy() error = %v", err)
	}

	front := httptest.NewServer(proxy)
	defer front.Close()

	sentBody := `{"model":"anthropic/claude-3-5-sonnet","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(sentBody))
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if gotPath != "/v1/messages" {
		t.Errorf("backend received path %q, want /v1/messages", gotPath)
	}
	if gotBody != sentBody {
		t.Errorf("backend received body %q, want %q (semantically identical — no translation)", gotBody, sentBody)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("front response status = %d, want 200", resp.StatusCode)
	}
}

// TestNewLiteLLMProxy_InvalidBaseURLIsError verifies malformed base URLs
// are rejected at construction time, not silently at request time.
func TestNewLiteLLMProxy_InvalidBaseURLIsError(t *testing.T) {
	_, err := NewLiteLLMProxy("://not-a-url")
	if err == nil {
		t.Fatal("expected error for malformed base URL, got nil")
	}
}
