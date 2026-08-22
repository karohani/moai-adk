package proxy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCodexAuthFile writes a fixture auth.json to dir/auth.json and
// returns its path, mirroring the schema research.md §2.3 observed:
// top-level auth_mode/OPENAI_API_KEY/tokens/last_refresh, tokens sub-object
// carrying id_token/access_token/refresh_token/account_id.
func writeCodexAuthFile(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

// TestReadCodexPlaintextAuthFile_ObservedSchemaParsesLeniently verifies the
// research.md §2.3-observed schema parses successfully, extracting exactly
// the fields the proxy needs (access_token, account_id) — extra/unknown
// fields (e.g. a hypothetical future field) do NOT break parsing, per
// design.md §5.3's lenient-parsing requirement.
func TestReadCodexPlaintextAuthFile_ObservedSchemaParsesLeniently(t *testing.T) {
	dir := t.TempDir()
	path := writeCodexAuthFile(t, dir, `{
		"auth_mode": "chatgpt",
		"OPENAI_API_KEY": null,
		"tokens": {
			"id_token": "idtok",
			"access_token": "acctok",
			"refresh_token": "refreshtok",
			"account_id": "acct-123"
		},
		"last_refresh": "2026-08-20T10:00:00Z",
		"a_hypothetical_future_field": "should not break parsing"
	}`)

	creds, err := ReadCodexPlaintextAuthFile(path)
	if err != nil {
		t.Fatalf("ReadCodexPlaintextAuthFile() error = %v", err)
	}
	if creds.AuthMode != "chatgpt" {
		t.Errorf("AuthMode = %q, want chatgpt", creds.AuthMode)
	}
	if creds.AccessToken != "acctok" {
		t.Errorf("AccessToken = %q, want acctok", creds.AccessToken)
	}
	if creds.AccountID != "acct-123" {
		t.Errorf("AccountID = %q, want acct-123", creds.AccountID)
	}
}

// TestReadCodexPlaintextAuthFile_UnknownAuthModeDeactivates verifies an
// auth_mode other than the one observed value ("chatgpt") is treated as
// "deactivate this group" (design.md §5.3: "모르는 auth_mode를 만나면
// 추측하지 말고 그 그룹을 비활성으로 표시한다") — the function returns an
// error wrapping ErrCodexUnknownAuthMode rather than guessing how to
// authenticate.
func TestReadCodexPlaintextAuthFile_UnknownAuthModeDeactivates(t *testing.T) {
	dir := t.TempDir()
	path := writeCodexAuthFile(t, dir, `{"auth_mode":"api_key","tokens":{"access_token":"x","account_id":"y"}}`)

	_, err := ReadCodexPlaintextAuthFile(path)
	if err == nil {
		t.Fatal("expected error for unrecognized auth_mode, got nil")
	}
	if !errors.Is(err, ErrCodexUnknownAuthMode) {
		t.Errorf("error = %v, want it to wrap ErrCodexUnknownAuthMode", err)
	}
}

// TestReadCodexPlaintextAuthFile_MissingFileIsError verifies a missing
// file surfaces a clear error naming the path (design.md §5.3: failure
// messages must say WHERE the read failed).
func TestReadCodexPlaintextAuthFile_MissingFileIsError(t *testing.T) {
	_, err := ReadCodexPlaintextAuthFile(filepath.Join(t.TempDir(), "nonexistent-auth.json"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestReadCodexPlaintextAuthFile_MissingRequiredFieldsIsError verifies a
// structurally-valid-but-incomplete file (auth_mode present, but no
// access_token) fails with a specific reason rather than succeeding with
// an empty credential.
func TestReadCodexPlaintextAuthFile_MissingRequiredFieldsIsError(t *testing.T) {
	dir := t.TempDir()
	path := writeCodexAuthFile(t, dir, `{"auth_mode":"chatgpt","tokens":{}}`)

	_, err := ReadCodexPlaintextAuthFile(path)
	if err == nil {
		t.Fatal("expected error for missing tokens.access_token, got nil")
	}
}

// TestReadCodexPlaintextAuthFile_MalformedJSONIsError verifies malformed
// JSON is rejected explicitly.
func TestReadCodexPlaintextAuthFile_MalformedJSONIsError(t *testing.T) {
	dir := t.TempDir()
	path := writeCodexAuthFile(t, dir, `{not json`)
	_, err := ReadCodexPlaintextAuthFile(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestReadCodexOSCredentialStore_AlwaysReturnsUnverifiedSentinel verifies
// the OS-credential-store leg does NOT guess a service/account name to
// probe — research.md §2.3 explicitly leaves the OS-store location/format
// unobserved, so inventing one would be an unverified premise
// (verification-claim-integrity.md §1.1 surface 4). This function exists
// so the "try both locations" requirement (plan.md M3 item 5) has a real
// code path to call, not so it can succeed today.
func TestReadCodexOSCredentialStore_AlwaysReturnsUnverifiedSentinel(t *testing.T) {
	_, err := ReadCodexOSCredentialStore()
	if !errors.Is(err, ErrCodexOSStoreUnverified) {
		t.Errorf("error = %v, want ErrCodexOSStoreUnverified", err)
	}
}

// TestReadCodexCredentials_TriesPlaintextFirstThenOSStore verifies the
// combined reader: plaintext success short-circuits (OS store never
// consulted in that case is not directly observable here, but the
// resulting credential must be the plaintext one).
func TestReadCodexCredentials_TriesPlaintextFirstThenOSStore(t *testing.T) {
	dir := t.TempDir()
	path := writeCodexAuthFile(t, dir, `{"auth_mode":"chatgpt","tokens":{"access_token":"acctok","account_id":"acct-1"}}`)

	creds, err := ReadCodexCredentials(path)
	if err != nil {
		t.Fatalf("ReadCodexCredentials() error = %v", err)
	}
	if creds.AccessToken != "acctok" {
		t.Errorf("AccessToken = %q, want acctok", creds.AccessToken)
	}
}

// TestReadCodexCredentials_BothSourcesFailedNamesBoth verifies the
// combined error message names BOTH attempted sources (plaintext path +
// the OS-store's documented unverified status), satisfying design.md
// §5.3's "which path, what wasn't found" requirement.
func TestReadCodexCredentials_BothSourcesFailedNamesBoth(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "auth.json")
	_, err := ReadCodexCredentials(missingPath)
	if err == nil {
		t.Fatal("expected error when both sources fail, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, missingPath) {
		t.Errorf("error message %q does not name the attempted plaintext path %q", msg, missingPath)
	}
}

// TestReadCodexPlaintextAuthFile_AgainstOfficialCodexCLI is plan.md M3
// item 6: verification against an OFFICIAL OpenAI Codex CLI install. This
// environment has ONLY `opencodex` on PATH (confirmed via `command -v
// codex opencodex`), not the official `codex` binary — the schema this
// package assumes was re-observed on the same opencodex-only machine
// research.md §2.3 originally used, which does NOT constitute independent
// verification against the official CLI. This test SKIPS with an explicit
// reason rather than silently passing or being omitted — see progress.md
// §E.2 M3 Gaps for the full writeup. If an official `codex` binary with a
// logged-in ~/.codex/auth.json is ever present, this test runs for real and
// either confirms or refutes the schema.
func TestReadCodexPlaintextAuthFile_AgainstOfficialCodexCLI(t *testing.T) {
	if _, err := lookOfficialCodexBinary(); err != nil {
		t.Skip("official OpenAI Codex CLI ('codex' binary) not found on PATH in this environment " +
			"(only 'opencodex' is present, which research.md §2.3 explicitly flags as NOT the official CLI) " +
			"— plan.md M3 item 6 schema verification cannot run here; reported as a Gap in progress.md §E.2 M3, not silently skipped")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}
	path := filepath.Join(home, ".codex", "auth.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("official codex CLI found on PATH but %s does not exist (not logged in) — cannot verify schema: %v", path, err)
	}

	creds, err := ReadCodexPlaintextAuthFile(path)
	if err != nil {
		t.Fatalf("schema verification against the OFFICIAL Codex CLI FAILED: %v — the research.md §2.3 observed schema does not hold for this install", err)
	}
	if creds.AccessToken == "" {
		t.Error("verification parsed structurally but AccessToken is empty")
	}
}
