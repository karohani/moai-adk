package proxy

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
)

// @MX:ANCHOR: [AUTO] serveOpenAIShapedMessages is the SINGLE handler every
// OpenAI-shaped group type calls — REQ-PROXY-018 / AC-PROXY-011 require
// exactly one translation implementation regardless of consumer count.
// @MX:REASON: fan_in >= 3 — openai_compatible.go's serveOpenAICompatible,
// codex_group.go's serveCodex, and (per REQ-PROXY-018, when copilot is
// eventually revived) a future copilot adapter all call this SAME
// function; a per-type parallel implementation would fail AC-PROXY-011
// regardless of how few consumers exist.

// openAIAuthenticator applies whatever auth a given OpenAI-shaped group
// needs to an outgoing backend request (a static bearer token for
// openai-compatible, a credential-store-derived bearer token for codex).
// Returning an error means the group's credential could not be obtained —
// serveOpenAIShapedMessages surfaces this as an HTTP error rather than
// sending an unauthenticated request.
type openAIAuthenticator func(req *http.Request) error

// serveOpenAIShapedMessages translates an Anthropic /v1/messages request
// to OpenAI Chat Completions shape (ToOpenAIChatRequest), sends it to
// baseURL + "/chat/completions" with auth applied, and translates the
// response back (FromOpenAIChatResponse for non-streaming,
// TranslateOpenAIStreamToAnthropicSSE for streaming). This is the ONLY
// translation entry point every OpenAI-shaped group's handler calls.
func serveOpenAIShapedMessages(
	w http.ResponseWriter,
	r *http.Request,
	client *http.Client,
	baseURL string,
	auth openAIAuthenticator,
	anthropicBody []byte,
	model string,
	stream bool,
) {
	// Validate the destination BEFORE any credential is attached. This leg
	// carries a live OAuth bearer token (codex), so an unvalidated base_url
	// — a typo, or a planted registry entry pointing at a metadata endpoint
	// or an external host — would exfiltrate it on the first request. The
	// litellm leg already parses and checks its URL; doing it only there
	// left the validation absent from precisely the path that holds secrets.
	endpoint, err := backendChatCompletionsURL(baseURL)
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusInternalServerError)
		return
	}

	translated, err := ToOpenAIChatRequest(anthropicBody, model)
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusBadRequest)
		return
	}

	backendReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(translated))
	if err != nil {
		http.Error(w, "proxy: build backend request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	backendReq.Header.Set("Content-Type", "application/json")
	if err := auth(backendReq); err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusUnauthorized)
		return
	}

	resp, err := client.Do(backendReq)
	if err != nil {
		http.Error(w, "proxy: backend request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		relayBackendError(w, resp.StatusCode, resp.Body)
		return
	}

	if stream {
		messageID := newAnthropicMessageID()
		rc := TranslateOpenAIStreamToAnthropicSSE(resp.Body, messageID, model)
		defer func() { _ = rc.Close() }()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 4096)
		for {
			n, rerr := rc.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if rerr != nil {
				return
			}
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "proxy: read backend response: "+err.Error(), http.StatusBadGateway)
		return
	}
	anthResp, err := FromOpenAIChatResponse(body, model)
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(anthResp)
}

// newAnthropicMessageID mints a proxy-local Anthropic-shaped message ID
// (the backend's own OpenAI "chatcmpl-..." id is not reused — this proxy
// owns the Anthropic-facing message identity).
func newAnthropicMessageID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "msg_proxy"
	}
	return "msg_" + hex.EncodeToString(buf)
}
