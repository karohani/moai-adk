package proxy

import "net/http"

// serveOpenAICompatible is the openai-compatible group's handler — the
// shared translation layer's FIRST consumer (plan.md M3 item 4): a
// self-hosted GPU-cluster backend, deliberately chosen first because its
// auth is trivial (v1 sends no additional auth header — see
// progress.md §E.2 M3 for the REQ-PROXY-024 rationale), letting this
// group verify translation-logic correctness in isolation from
// credential-handling complexity.
//
// It calls the SAME serveOpenAIShapedMessages every OpenAI-shaped group
// calls (REQ-PROXY-018 / AC-PROXY-011) — no group-specific translation
// logic exists here or anywhere else in this file.
func serveOpenAICompatible(w http.ResponseWriter, r *http.Request, client *http.Client, baseURL string, anthropicBody []byte, model string, stream bool) {
	noAuth := func(req *http.Request) error { return nil }
	serveOpenAIShapedMessages(w, r, client, baseURL, noAuth, anthropicBody, model, stream)
}
