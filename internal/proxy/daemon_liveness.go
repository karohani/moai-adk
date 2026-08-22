package proxy

import (
	"net"
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
	conn, err := net.DialTimeout("tcp", st.Address, daemonLivenessDialTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
