package proxy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// codexDefaultAuthFilePath resolves ~/.codex/auth.json. A package-level
// var (not a plain function) so tests can override it without touching the
// real filesystem or $HOME.
var codexDefaultAuthFilePath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("codex: resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "auth.json"), nil
}

// serveCodex is the codex group's per-request handler.
//
// Endpoint (plan.md M3 item 7 — investigated, NOT settled; see
// progress.md §E.2 M3 for the full writeup): research.md §2.3's
// auth_mode:"chatgpt" observation narrows the target toward OpenAI's
// ChatGPT-backend API, but the exact URL was never independently
// confirmed — this delegated session has no WebFetch/WebSearch tool to
// complete the investigation a prior plan-phase agent started with one.
// Hardcoding an unverified URL guess would embed TWO unverified claims at
// once (the URL AND the request shape that endpoint expects), which is
// worse than requiring explicit configuration. So this adapter REQUIRES
// group.BaseURL to be set for codex groups; an empty BaseURL isolates the
// group with a reason naming the open investigation — the SAME isolation
// shape REQ-PROXY-021 already uses for credential failures.
//
// Auth (AC-PROXY-013 — no interactive auth flow): the credential is read
// from the existing Codex CLI's own storage via ReadCodexCredentials; this
// function never launches an interactive sign-in page, never prompts for
// login, and never performs its own token-exchange round trip.
//
// Translation (REQ-PROXY-018 — single shared translator): once
// credentials and a base URL are resolved, this function calls the SAME
// serveOpenAIShapedMessages every other OpenAI-shaped group calls
// (see openai_compatible.go) — no codex-specific translation logic exists
// anywhere in this file.
func serveCodex(w http.ResponseWriter, r *http.Request, client *http.Client, group Group, anthropicBody []byte, model string, stream bool) {
	if group.BaseURL == "" {
		http.Error(w, "proxy: codex group has no base_url configured — the ChatGPT-backend endpoint URL is unverified in this environment "+
			"(plan.md M3 item 7 / research.md §2.3); configure base_url explicitly once the endpoint is confirmed", http.StatusInternalServerError)
		return
	}

	path, err := codexDefaultAuthFilePath()
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusInternalServerError)
		return
	}
	creds, err := ReadCodexCredentials(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("proxy: codex group credential read failed at %s: %v", path, err), http.StatusUnauthorized)
		return
	}

	auth := func(req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		if creds.AccountID != "" {
			req.Header.Set("chatgpt-account-id", creds.AccountID)
		}
		return nil
	}
	serveOpenAIShapedMessages(w, r, client, group.BaseURL, auth, anthropicBody, model, stream)
}

// EvaluateGroupStatusWithCodexProbe extends EvaluateGroupStatus (M1's
// type-support isolation) with M3's credential/config-readiness isolation
// for codex-type groups (REQ-PROXY-021, AC-PROXY-014): a codex group with
// no base_url configured, or whose credential store cannot be read at
// evaluation time, is isolated inactive with a reason naming the group and
// the attempted read path — daemon startup and every OTHER group continue
// unaffected. probe is injected so this function is testable without
// touching the real filesystem/OS credential store; production callers
// pass ReadCodexCredentials.
func EvaluateGroupStatusWithCodexProbe(reg *Registry, probe func(plaintextPath string) (*CodexCredentials, error)) map[string]GroupStatus {
	statuses := EvaluateGroupStatus(reg)

	for name, g := range reg.Groups {
		if g.Type != GroupTypeCodex {
			continue
		}
		st, ok := statuses[name]
		if !ok || !st.Active {
			continue // already inactive from the base (type-support) evaluation
		}

		if g.BaseURL == "" {
			statuses[name] = GroupStatus{
				Name:   name,
				Active: false,
				Reason: fmt.Sprintf("group %q (codex): no base_url configured — the ChatGPT-backend endpoint URL is unverified (plan.md M3 item 7); configure base_url explicitly once confirmed", name),
			}
			continue
		}

		path, err := codexDefaultAuthFilePath()
		if err != nil {
			statuses[name] = GroupStatus{
				Name:   name,
				Active: false,
				Reason: fmt.Sprintf("group %q (codex): %v", name, err),
			}
			continue
		}

		if _, err := probe(path); err != nil {
			statuses[name] = GroupStatus{
				Name:   name,
				Active: false,
				Reason: fmt.Sprintf("group %q (codex): credential read failed at %s: %v", name, path, err),
			}
		}
	}

	return statuses
}
