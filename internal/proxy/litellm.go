package proxy

import (
	"fmt"
	"net/http/httputil"
	"net/url"
)

// NewLiteLLMProxy returns a reverse proxy that relays every request to
// baseURL VERBATIM (REQ-PROXY-017: litellm is pure passthrough, no body
// translation — research.md §2.1 confirmed LiteLLM's /v1/messages endpoint
// is already Anthropic-shaped, so the only work is credential-header
// swap-in and stream relay).
//
// FlushInterval: -1 disables output buffering so SSE chunks reach the
// client as they arrive rather than batched — required for streaming
// responses to behave correctly through the proxy.
func NewLiteLLMProxy(baseURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("proxy: parse litellm base URL %q: %w", baseURL, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("proxy: litellm base URL %q must be absolute (scheme + host)", baseURL)
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = -1
	return rp, nil
}
