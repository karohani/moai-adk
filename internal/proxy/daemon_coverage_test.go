package proxy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDaemon_WithLockTimeout verifies the timeout override actually takes
// effect: an artificially tiny timeout against a pre-held lock must time
// out quickly rather than using the default DaemonLockTimeout.
func TestDaemon_WithLockTimeout(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Hold the lock externally so Acquire below must wait/time out.
	holder := newDaemonLock()
	lockPath := daemonLockFilePath(stateDir)
	if err := holder.acquire(lockPath); err != nil {
		t.Fatalf("holder.acquire() error = %v", err)
	}
	defer func() { _ = holder.release() }()

	d := NewDaemon(stateDir).WithLockTimeout(20 * time.Millisecond)
	start := time.Now()
	_, err := d.Acquire(func() (string, func() error, error) {
		return "127.0.0.1:1", func() error { return nil }, nil
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected lock-timeout error while another holder owns the lock, got nil")
	}
	if elapsed > time.Second {
		t.Errorf("Acquire() took %v, want it to respect the ~20ms WithLockTimeout override", elapsed)
	}
}

// TestDaemon_ReleaseOnEmptyStateIsANoOp verifies Release on a daemon with no
// recorded holders returns remaining=0 without error (the RefCount<=0
// degenerate branch).
func TestDaemon_ReleaseOnEmptyStateIsANoOp(t *testing.T) {
	stateDir := t.TempDir()
	d := NewDaemon(stateDir)

	remaining, err := d.Release()
	if err != nil {
		t.Fatalf("Release() on empty state error = %v", err)
	}
	if remaining != 0 {
		t.Errorf("Release() remaining = %d, want 0", remaining)
	}
}

// TestDaemon_ReadState_MalformedJSONIsError verifies a corrupted state file
// surfaces a parse error rather than silently returning a zero-value state.
func TestDaemon_ReadState_MalformedJSONIsError(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(daemonStateFilePath(stateDir), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	d := NewDaemon(stateDir)
	_, err := d.readState()
	if err == nil {
		t.Fatal("expected parse error for malformed state file, got nil")
	}
}

// TestDaemon_WriteStateAtomic_MkdirFailureIsError verifies a stateDir path
// that cannot be created as a directory (because a regular file already
// occupies it) surfaces an error rather than silently no-op-ing.
func TestDaemon_WriteStateAtomic_MkdirFailureIsError(t *testing.T) {
	parent := t.TempDir()
	blocker := filepath.Join(parent, "blocked-by-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	// stateDir is a path THROUGH the regular file — MkdirAll must fail.
	stateDir := filepath.Join(blocker, "state")

	d := NewDaemon(stateDir)
	err := d.writeStateAtomic(DaemonState{RefCount: 1})
	if err == nil {
		t.Fatal("expected MkdirAll failure error, got nil")
	}
}

// TestDaemonLock_AcquireContentionFails verifies a second acquire on an
// already-locked file returns an error (the flock LOCK_NB contention path).
func TestDaemonLock_AcquireContentionFails(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	first := newDaemonLock()
	if err := first.acquire(lockPath); err != nil {
		t.Fatalf("first.acquire() error = %v", err)
	}
	defer func() { _ = first.release() }()

	second := newDaemonLock()
	if err := second.acquire(lockPath); err == nil {
		t.Error("second.acquire() on an already-locked file expected an error, got nil")
		_ = second.release()
	}
}

// TestDaemonLock_ReleaseIsIdempotent verifies release() is safe to call
// multiple times, including before any successful acquire.
func TestDaemonLock_ReleaseIsIdempotent(t *testing.T) {
	l := newDaemonLock()
	if err := l.release(); err != nil {
		t.Errorf("release() before acquire error = %v, want nil", err)
	}
	if err := l.release(); err != nil {
		t.Errorf("second release() error = %v, want nil", err)
	}
}
