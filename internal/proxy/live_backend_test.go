package proxy

// Manual live-backend verification harness.
//
// Every other test in this package uses a fake backend, which is precisely
// why 86% coverage once coexisted with a defect that sent the group-prefixed
// model name to the real backend: a mock shaped like the implementation's
// assumptions cannot contradict them. This harness is the counterweight — it
// drives the FULL path (registry -> catalog -> auth -> handler -> credential
// read -> shared translator -> a REAL OpenAI-shaped backend) and asserts on
// what the backend actually returned.
//
// It is skipped unless explicitly enabled, because it needs a reachable
// backend and real credentials:
//
//	MOAI_PROXY_LIVE=1 \
//	MOAI_PROXY_LIVE_BASE_URL=http://127.0.0.1:10100/v1 \
//	MOAI_PROXY_LIVE_MODEL=gpt-5.3-codex-spark \
//	MOAI_PROXY_LIVE_GROUP_TYPE=codex \
//	go test ./internal/proxy/ -run TestLiveBackend -v
//
// Run it after ANY change to the translation layer, the auth path, or
// base_url validation. A green unit suite is not evidence that a real
// backend still answers.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// liveConfig reads the harness parameters from the environment.
func liveConfig(t *testing.T) (baseURL, model string, groupType GroupType) {
	t.Helper()
	baseURL = os.Getenv("MOAI_PROXY_LIVE_BASE_URL")
	model = os.Getenv("MOAI_PROXY_LIVE_MODEL")
	if baseURL == "" || model == "" {
		t.Skip("set MOAI_PROXY_LIVE_BASE_URL and MOAI_PROXY_LIVE_MODEL")
	}
	switch os.Getenv("MOAI_PROXY_LIVE_GROUP_TYPE") {
	case "codex":
		groupType = GroupTypeCodex
	default:
		groupType = GroupTypeOpenAICompatible
	}
	return baseURL, model, groupType
}

const liveGroupName = "live"

func liveServer(t *testing.T, token, baseURL string, groupType GroupType) (addr string, stop func() error) {
	t.Helper()
	reg := &Registry{
		Groups: map[string]Group{
			liveGroupName: {Type: groupType, BaseURL: baseURL},
		},
		Sets: map[string][]string{},
	}
	h, err := NewMessagesHandler(reg, NewCatalog(reg, []string{liveGroupName}), nil)
	if err != nil {
		t.Fatalf("NewMessagesHandler: %v", err)
	}
	addr, stop, err = StartServer(h.WithAuthToken(token))
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	return addr, stop
}

func TestLiveBackend(t *testing.T) {
	if os.Getenv("MOAI_PROXY_LIVE") != "1" {
		t.Skip("live test — set MOAI_PROXY_LIVE=1")
	}
	baseURL, backendModel, groupType := liveConfig(t)
	liveModel := liveGroupName + "/" + backendModel
	const token = "live-test-token"
	addr, stop := liveServer(t, token, baseURL, groupType)
	defer func() { _ = stop() }()
	client := &http.Client{Timeout: 90 * time.Second}

	post := func(body string, withAuth bool) (*http.Response, string) {
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/messages", strings.NewReader(body))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if withAuth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp, string(raw)
	}

	// --- 1. auth is actually enforced on the real serving path ---
	t.Run("1_unauthenticated_is_rejected", func(t *testing.T) {
		resp, body := post(`{"model":"`+liveModel+`","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`, false)
		t.Logf("LIVE-1 status=%d body=%s", resp.StatusCode, liveTruncate(body, 200))
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", resp.StatusCode)
		}
	})

	// --- 2. non-streaming round trip to the real codex relay ---
	t.Run("2_nonstreaming_roundtrip", func(t *testing.T) {
		resp, body := post(`{"model":"`+liveModel+`","max_tokens":64,"messages":[{"role":"user","content":"Reply with exactly: codex routing works"}]}`, true)
		t.Logf("LIVE-2 status=%d", resp.StatusCode)
		t.Logf("LIVE-2 body=%s", liveTruncate(body, 600))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		var got map[string]interface{}
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		// Shape contract added during the fix rounds.
		for _, k := range []string{"content", "model", "stop_reason", "stop_sequence", "usage", "role", "type"} {
			if _, ok := got[k]; !ok {
				t.Errorf("LIVE-2 response missing %q (fix-round contract)", k)
			}
		}
		if got["model"] != backendModel {
			t.Errorf("LIVE-2 model = %v, want the RESOLVED %q (group prefix stripped)", got["model"], backendModel)
		}
		if c, ok := got["content"].([]interface{}); !ok {
			t.Errorf("LIVE-2 content is not an array: %#v", got["content"])
		} else if len(c) == 0 {
			t.Error("LIVE-2 content array is empty — no answer relayed")
		}
		if u, ok := got["usage"].(map[string]interface{}); ok {
			t.Logf("LIVE-2 usage: input=%v output=%v", u["input_tokens"], u["output_tokens"])
		}
	})

	// --- 3. streaming round trip: the heavily-rewritten state machine ---
	t.Run("3_streaming_roundtrip", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/messages",
			strings.NewReader(`{"model":"`+liveModel+`","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"Count: one two three"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("stream POST: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		s := string(raw)
		t.Logf("LIVE-3 status=%d ct=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
		t.Logf("LIVE-3 FULL STREAM:\n%s", s)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		// The Anthropic event sequence the translator must produce.
		for _, ev := range []string{"event: message_start", "event: content_block_start",
			"event: content_block_delta", "event: content_block_stop",
			"event: message_delta", "event: message_stop"} {
			if !strings.Contains(s, ev) {
				t.Errorf("LIVE-3 missing %q", ev)
			}
		}
		// A healthy stream must NOT be reported as truncated.
		if strings.Contains(s, "event: error") {
			t.Errorf("LIVE-3 a healthy stream emitted an error event — truncation detection is over-firing")
		}
		if !strings.Contains(s, `"stop_reason":"end_turn"`) {
			t.Errorf("LIVE-3 healthy stream did not end with end_turn")
		}
		// H8: usage must be reported now that include_usage is requested.
		if i := strings.Index(s, `"input_tokens":`); i >= 0 {
			t.Logf("LIVE-3 usage fragment: %s", liveTruncate(s[i:], 80))
		}
		// message_start ALWAYS carries a zero usage placeholder by protocol;
		// the real figures belong to message_delta. Assert on that frame only.
		if i := strings.Index(s, "event: message_delta"); i >= 0 {
			delta := s[i:]
			if j := strings.Index(delta, "\n\n"); j >= 0 {
				delta = delta[:j]
			}
			t.Logf("LIVE-3 message_delta frame: %s", delta)
			if strings.Contains(delta, `"output_tokens":0`) {
				t.Errorf("LIVE-3 message_delta reports zero output_tokens — include_usage not honored by this backend")
			}
		} else {
			t.Error("LIVE-3 no message_delta frame at all")
		}
	})

	// --- 4. base_url validation must not have broken a legitimate relay ---
	t.Run("4_base_url_validation_accepts_the_relay", func(t *testing.T) {
		got, err := backendChatCompletionsURL(baseURL)
		t.Logf("LIVE-4 %q -> %q err=%v", baseURL, got, err)
		if err != nil {
			t.Errorf("the live relay URL was rejected by validation: %v", err)
		}
	})
}

func liveTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("... (+%d bytes)", len(s)-n)
}
