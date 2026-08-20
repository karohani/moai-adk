package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

// ProbeMessagesEndpoint checks whether a LiteLLM instance at baseURL has
// its Anthropic-compatible /v1/messages route active.
//
// Detection method (research.md M2 gate item — investigated as part of
// this milestone): LiteLLM's proxy server returns HTTP 404 for any path it
// has not registered a handler for, and registers /v1/messages only when
// the Anthropic-unified passthrough is enabled on that instance
// (docs.litellm.ai/docs/anthropic_unified, confirmed in research.md §2.1
// for the endpoint's EXISTENCE; this probe answers the remaining
// per-instance ACTIVATION question deferred to M2). A request that reaches
// LiteLLM's own request handling — 200 success, 400 malformed body, 401
// unauthorized, or any other non-404 status — is therefore positive
// evidence the route is registered and being served; only 404 means "no
// such route on this instance".
//
// This probe is deliberately minimal (a POST with an empty JSON object): it
// does not need valid credentials or a valid Anthropic body to distinguish
// "route exists" from "route absent" — only the 404-vs-not-404 boundary
// matters. A network-level failure (connection refused, DNS failure,
// timeout) is returned as err so callers can distinguish "instance
// unreachable" from "instance up but route inactive".
func ProbeMessagesEndpoint(client *http.Client, baseURL string) (active bool, reason string, err error) {
	target := strings.TrimRight(baseURL, "/") + "/v1/messages"
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(`{}`))
	if err != nil {
		return false, "", fmt.Errorf("proxy: build litellm health probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("proxy: litellm health probe against %q: %w", baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, fmt.Sprintf("litellm instance %q does not expose /v1/messages (404)", baseURL), nil
	}
	return true, "", nil
}
