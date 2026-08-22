package proxy

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// daemonLivenessDialTimeout bounds the reuse-path liveness probe. The probe
// runs while the state lock is held, so it must be short enough not to push
// concurrent invocations into DaemonLockTimeout (2s).
const daemonLivenessDialTimeout = 250 * time.Millisecond

// daemonStateIsLive reports whether a recorded daemon state describes a
// server that is actually still reachable.
//
// Without this check the reference count alone decides reuse, and a holder
// that dies without running Release (SIGKILL, panic, power loss, or simply
// a SIGTERM — Go runs no deferred functions on signal death) leaves the
// machine-scope state file permanently at RefCount >= 1. Because the file
// outlives reboots, every later `moai proxy` on the machine would then be
// handed an address nothing listens on, with no self-heal path short of the
// user deleting the file by hand.
//
// The probe deliberately dials the recorded address rather than trusting
// the recorded PID: the PID answers "did some process with this number
// survive?" (which PID reuse can answer wrongly), while the dial answers the
// question that actually matters — "is a server accepting connections
// there?". A stale state file therefore degrades to "start a fresh daemon",
// which is the correct recovery.
func daemonStateIsLive(st DaemonState) bool {
	if st.Address == "" {
		return false
	}

	// A bare TCP dial proves only that SOMETHING occupies the port. After a
	// daemon dies any process can bind the freed address, and this state
	// file is what decides where Claude Code's ANTHROPIC_BASE_URL points —
	// accepting an impostor would hand it the whole session.
	//
	// Identity is established by challenge-response rather than by sending
	// the token: presenting a bearer token to an unverified listener gives
	// the token to whatever answers, which is precisely the case being
	// tested for. The daemon returns HMAC(token, nonce); a listener that
	// does not hold the token cannot produce it.
	client := &http.Client{Timeout: daemonLivenessDialTimeout}

	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return false
	}
	nonce := hex.EncodeToString(nonceBytes)

	resp, err := client.Get("http://" + st.Address + healthPath + "?nonce=" + nonce)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	if st.Token == "" {
		// State written before tokens existed carries nothing to verify
		// against, so reachability is all that can be established. Checked
		// BEFORE decoding: such a daemon may not answer with a proof body
		// at all, and refusing to reuse a live daemon over a missing field
		// it was never expected to send would be a false negative.
		return true
	}

	var payload struct {
		Proof string `json:"proof"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&payload); err != nil {
		return false
	}
	return hmac.Equal([]byte(payload.Proof), []byte(healthProof(st.Token, nonce)))
}
