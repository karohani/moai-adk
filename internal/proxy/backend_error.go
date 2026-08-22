package proxy

import (
	"encoding/json"
	"io"
	"net/http"
)

// backendErrorBodyLimit caps how much of an upstream error body is read.
// The body is attacker-influenceable (a backend can be any configured URL)
// and was previously consumed with an unbounded ReadAll.
const backendErrorBodyLimit = 8 << 10 // 8 KiB

// relayBackendError translates an upstream error into an Anthropic-shaped
// error response.
//
// It replaces a single line that did three wrong things at once:
//
//  1. Collapsed EVERY 4xx/5xx into 502. A client cannot tell a 429 (back
//     off and retry) from a 401 (re-authenticate) from a 400 (fix the
//     request) when all three arrive as one generic gateway failure, so
//     correct retry behavior is impossible.
//  2. Echoed the upstream body verbatim. Provider error bodies routinely
//     quote request fragments, account identifiers, and key prefixes
//     ("Incorrect API key provided: sk-…XYZ").
//  3. Emitted text/plain via http.Error. Anthropic SDKs parse a JSON error
//     object, so the caller saw a JSON parse failure instead of the
//     backend's actual error.
//
// The upstream status is preserved so client-side retry logic works, and
// the body is reduced to the provider's own message when it is parseable —
// never the raw envelope.
func relayBackendError(w http.ResponseWriter, upstreamStatus int, body io.Reader) {
	raw, _ := io.ReadAll(io.LimitReader(body, backendErrorBodyLimit))

	status := mapBackendStatus(upstreamStatus)
	writeAnthropicError(w, status, anthropicErrorType(status), backendErrorMessage(raw))
}

// mapBackendStatus decides what the CLIENT sees for a given upstream status.
// Statuses whose semantics the client can act on are preserved; anything
// else becomes 502, which is what a gateway genuinely means.
func mapBackendStatus(upstream int) int {
	switch upstream {
	case http.StatusBadRequest, // client sent something the backend rejected
		http.StatusUnauthorized,   // credentials invalid/expired -> re-login
		http.StatusForbidden,      // credentials valid, access denied
		http.StatusNotFound,       // unknown model/route
		http.StatusRequestTimeout, // retryable
		http.StatusConflict,       // retryable
		http.StatusRequestEntityTooLarge,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests: // rate limited -> back off
		return upstream
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return upstream // transient upstream states the client should retry
	default:
		return http.StatusBadGateway
	}
}

// anthropicErrorType maps an HTTP status onto Anthropic's error `type`
// vocabulary so SDK-side error classification behaves as it would against
// the real API.
func anthropicErrorType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusRequestEntityTooLarge:
		return "request_too_large"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "overloaded_error"
	default:
		return "api_error"
	}
}

// backendErrorMessage extracts the provider's own human-readable message
// from an error body, falling back to a generic string. Only the message
// field is surfaced — the surrounding envelope (which is where identifiers
// and key fragments tend to live) is discarded.
func backendErrorMessage(raw []byte) string {
	if len(raw) == 0 {
		return "backend returned an error with no body"
	}

	// OpenAI shape: {"error":{"message":"...","type":"..."}}
	var openaiShaped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &openaiShaped); err == nil && openaiShaped.Error.Message != "" {
		return openaiShaped.Error.Message
	}

	// Some backends put the message at the top level.
	var flat struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if err := json.Unmarshal(raw, &flat); err == nil {
		if flat.Message != "" {
			return flat.Message
		}
		if flat.Detail != "" {
			return flat.Detail
		}
	}

	return "backend returned an unparseable error response"
}

// writeAnthropicError emits an Anthropic-shaped JSON error object, the
// format every Anthropic SDK expects on a non-2xx.
func writeAnthropicError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    errType,
			"message": "proxy: " + message,
		},
	})
}
