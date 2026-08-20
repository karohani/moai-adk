package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// @MX:ANCHOR: [AUTO] ReadCodexCredentials is the codex group's credential
// acquisition seam — the single place this proxy touches another CLI's
// undocumented, unowned credential store (design.md §5.3's isolation
// point).
// @MX:REASON: fan_in >= 3 once M4 CLI wiring lands — codex_group.go's
// per-request handler AND EvaluateGroupStatusWithCodexProbe's
// activation-time isolation check both call this; a third caller
// (M4's daemon-startup group-status report) reads its result.

// ErrCodexUnknownAuthMode signals that ~/.codex/auth.json (or the OS
// credential store) reports an auth_mode other than the one value
// research.md §2.3 actually observed ("chatgpt"). design.md §5.3 requires
// treating an unknown auth_mode as "deactivate this group" rather than
// guessing how to authenticate with it — this sentinel lets callers
// distinguish that case from a plain read/parse failure if they need to,
// though both ultimately isolate the group the same way (REQ-PROXY-021).
var ErrCodexUnknownAuthMode = errors.New("codex: unrecognized auth_mode")

// ErrCodexOSStoreUnverified signals that the OS-specific credential store
// leg was attempted but cannot succeed: research.md §2.3 documents that
// Codex CAN store credentials in "your OS-specific credential store" as an
// alternative to the plaintext file, but neither the storage location
// (keychain service/account name) nor the on-disk format was ever
// observed. Returning a fixed sentinel here — rather than guessing a
// service name to probe — avoids asserting an unverified premise
// (verification-claim-integrity.md §1.1 surface 4: "a reference existing
// is not evidence the referent is live").
var ErrCodexOSStoreUnverified = errors.New("codex: OS-specific credential store location/format is unobserved (research.md §2.3) — not attempted")

// CodexCredentials is the minimal shape this proxy needs from a Codex
// CLI-managed credential store to authenticate a request.
type CodexCredentials struct {
	AuthMode    string
	AccessToken string
	AccountID   string
}

// codexAuthFile mirrors the plaintext auth.json schema research.md §2.3
// observed by inspecting key names only (never values) on a local machine:
// top-level auth_mode / OPENAI_API_KEY / tokens / last_refresh, with
// tokens carrying id_token / access_token / refresh_token / account_id.
//
// Per design.md §5.3, parsing MUST be lenient: an unrecognized extra field
// must not break parsing (encoding/json already does this by default —
// unmapped JSON keys are silently ignored); only the fields this proxy
// actually needs are validated as required, after unmarshal.
type codexAuthFile struct {
	AuthMode string `json:"auth_mode"`
	Tokens   struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
}

// ReadCodexPlaintextAuthFile reads and lenient-parses the plaintext Codex
// auth file at path (normally ~/.codex/auth.json).
//
// design.md §5.3's three requirements this function implements:
//  1. Lenient parsing — an unknown field never fails parsing; only a
//     missing REQUIRED field (auth_mode, tokens.access_token,
//     tokens.account_id) does.
//  2. auth_mode read as a branch signal — the only observed value is
//     "chatgpt" (research.md §2.3); anything else returns an error
//     wrapping ErrCodexUnknownAuthMode rather than guessing how to
//     authenticate with it.
//  3. Failure messages name the path — every error includes path, so the
//     caller (and ultimately the user, via REQ-PROXY-021's isolation
//     reason) can tell whether this is a "log back in" problem or a
//     schema-drift problem.
func ReadCodexPlaintextAuthFile(path string) (*CodexCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("codex: read %s: %w", path, err)
	}

	var raw codexAuthFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("codex: parse %s: %w", path, err)
	}

	if raw.AuthMode == "" {
		return nil, fmt.Errorf("codex: %s carries no auth_mode field", path)
	}
	if raw.AuthMode != "chatgpt" {
		return nil, fmt.Errorf("%w: %s reports auth_mode %q (only \"chatgpt\" is understood — research.md §2.3 / design.md §5.3)", ErrCodexUnknownAuthMode, path, raw.AuthMode)
	}
	if raw.Tokens.AccessToken == "" {
		return nil, fmt.Errorf("codex: %s has auth_mode=chatgpt but tokens.access_token is empty or absent", path)
	}
	if raw.Tokens.AccountID == "" {
		return nil, fmt.Errorf("codex: %s has auth_mode=chatgpt but tokens.account_id is empty or absent", path)
	}

	return &CodexCredentials{
		AuthMode:    raw.AuthMode,
		AccessToken: raw.Tokens.AccessToken,
		AccountID:   raw.Tokens.AccountID,
	}, nil
}

// ReadCodexOSCredentialStore is the second of the two locations Codex may
// store credentials (research.md §2.3: "~/.codex/auth.json 또는 OS 자격
// 증명 저장소"). It always returns ErrCodexOSStoreUnverified — see that
// sentinel's doc comment for why this is the honest implementation rather
// than a stub that silently does nothing: the attempt is a real, callable
// code path (satisfying plan.md M3 item 5's "try both locations"), it just
// cannot succeed until the storage format is actually observed on a real
// OS-credential-store-using install.
func ReadCodexOSCredentialStore() (*CodexCredentials, error) {
	return nil, ErrCodexOSStoreUnverified
}

// ReadCodexCredentials tries the plaintext auth file first, then the
// OS credential store, combining both failure reasons into one message
// naming both attempted sources when neither succeeds.
func ReadCodexCredentials(plaintextPath string) (*CodexCredentials, error) {
	creds, plaintextErr := ReadCodexPlaintextAuthFile(plaintextPath)
	if plaintextErr == nil {
		return creds, nil
	}
	_, osErr := ReadCodexOSCredentialStore()
	return nil, fmt.Errorf("codex: plaintext auth unavailable (%v); OS credential store: %v", plaintextErr, osErr)
}

// lookOfficialCodexBinary probes PATH for the literal official OpenAI
// Codex CLI binary name "codex" — distinct from "opencodex", which
// research.md §2.3 explicitly flags as NOT the official CLI despite
// sharing the same ~/.codex/ config directory and auth.json schema.
func lookOfficialCodexBinary() (string, error) {
	return exec.LookPath("codex")
}
