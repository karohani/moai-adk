package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// backendChatCompletionsURL validates a group's configured base_url and
// joins the Chat Completions path onto it.
//
// The previous implementation was a bare string concatenation
// (baseURL + "/chat/completions") performed immediately before a live
// bearer token was attached. That is the wrong order of operations for a
// credential-bearing request: whatever string the registry happens to hold
// becomes the destination. Two failure classes follow.
//
// Credential exfiltration / SSRF. A typo'd or planted base_url —
// "http://169.254.169.254" (cloud metadata), an attacker-controlled host, a
// non-HTTP scheme — receives the Codex OAuth token on the first request.
// This is sharpest for the codex group specifically, whose documentation
// asks the USER to hand-author base_url because the real endpoint is
// unverified, maximizing the chance the value is wrong.
//
// URL-joining corruption. Concatenation mangles ordinary inputs:
// "https://h/v1/" yields "//chat/completions"; "https://h/v1?k=x" puts the
// path into the query string and sends the request to "/".
//
// Rejecting rather than repairing is deliberate: a base_url the operator
// did not intend should surface as a configuration error, not be silently
// normalized into something that "works" against an unintended host.
func backendChatCompletionsURL(baseURL string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("group has no base_url configured")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("base_url %q is not a valid URL: %w", baseURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("base_url %q must use http or https, got scheme %q", baseURL, parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("base_url %q has no host", baseURL)
	}
	if parsed.User != nil {
		// Credentials embedded in the URL are silently dropped by some
		// transports and logged by others; a group's auth belongs in its
		// credential source, never in the registry (design.md §6).
		return "", fmt.Errorf("base_url must not embed credentials (user info in URL)")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		// A query or fragment on a base URL cannot survive path joining in
		// any meaningful way — concatenation silently moved the path into
		// the query string.
		return "", fmt.Errorf("base_url %q must not carry a query string or fragment", baseURL)
	}

	if isLinkLocalHost(parsed.Hostname()) {
		// No LLM backend lives on the link-local range; what does live
		// there is the cloud instance-metadata service. This check cannot
		// establish that an arbitrary host is TRUSTED — the whole point of
		// an openai-compatible group is that the operator names their own
		// cluster at an arbitrary address — but it does remove the one
		// destination class that is never a legitimate backend and is the
		// classic SSRF target.
		return "", fmt.Errorf("base_url %q targets the link-local/metadata range, which is never a valid LLM backend", baseURL)
	}

	joined := *parsed
	joined.Path = strings.TrimSuffix(parsed.Path, "/") + "/chat/completions"
	return joined.String(), nil
}

// isLinkLocalHost reports whether host is an IPv4/IPv6 link-local address
// (169.254.0.0/16, fe80::/10) — the cloud instance-metadata range.
func isLinkLocalHost(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
