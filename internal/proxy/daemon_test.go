package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultDaemonStateDir_ResolvesUnderHomeDir verifies the machine-scope
// daemon runtime state directory resolves to ~/.moai/proxy (design.md §3.4:
// daemon runtime state is machine-scope, distinct from .moai/state/).
func TestDefaultDaemonStateDir_ResolvesUnderHomeDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir, err := DefaultDaemonStateDir()
	if err != nil {
		t.Fatalf("DefaultDaemonStateDir() error = %v", err)
	}
	want := filepath.Join(tmpHome, ".moai", "proxy")
	if dir != want {
		t.Errorf("DefaultDaemonStateDir() = %q, want %q", dir, want)
	}
}

// TestDaemon_AcquireStartsServerOnFirstCall verifies the first Acquire call
// invokes the ServerFactory exactly once and returns its address (REQ-PROXY-001).
func TestDaemon_AcquireStartsServerOnFirstCall(t *testing.T) {
	stateDir := t.TempDir()
	d := NewDaemon(stateDir)

	calls := 0
	factory := func() (string, func() error, error) {
		calls++
		return "127.0.0.1:9999", func() error { return nil }, nil
	}

	addr, err := d.Acquire(factory)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if addr != "127.0.0.1:9999" {
		t.Errorf("Acquire() addr = %q, want 127.0.0.1:9999", addr)
	}
	if calls != 1 {
		t.Errorf("factory called %d times, want 1", calls)
	}
}

// TestDaemon_AcquireFromTwoCallersSharesOneDaemon is the key observation
// point for AC-PROXY-001b: two Daemon instances pointing at the SAME
// machine-scope state directory (simulating two different project working
// directories invoking `moai proxy` independently) MUST share exactly one
// underlying server — the second Acquire MUST NOT invoke its own factory,
// MUST return the SAME address, and refcount MUST reach 2.
func TestDaemon_AcquireFromTwoCallersSharesOneDaemon(t *testing.T) {
	stateDir := t.TempDir() // shared machine-scope state dir

	callerA := NewDaemon(stateDir) // simulates project dir 1
	callerB := NewDaemon(stateDir) // simulates project dir 2 (different cwd)

	callsA := 0
	factoryA := func() (string, func() error, error) {
		callsA++
		return "127.0.0.1:8001", func() error { return nil }, nil
	}
	callsB := 0
	factoryB := func() (string, func() error, error) {
		callsB++
		return "127.0.0.1:8002", func() error { return nil }, nil
	}

	addrA, err := callerA.Acquire(factoryA)
	if err != nil {
		t.Fatalf("callerA.Acquire() error = %v", err)
	}
	addrB, err := callerB.Acquire(factoryB)
	if err != nil {
		t.Fatalf("callerB.Acquire() error = %v", err)
	}

	if addrA != addrB {
		t.Errorf("addresses diverged: A=%q B=%q, want identical (one shared daemon)", addrA, addrB)
	}
	if callsA != 1 {
		t.Errorf("factoryA called %d times, want exactly 1 (the elected starter)", callsA)
	}
	if callsB != 0 {
		t.Errorf("factoryB called %d times, want 0 (caller B must reuse the existing daemon, never start its own)", callsB)
	}

	st, err := callerB.readState()
	if err != nil {
		t.Fatalf("readState() error = %v", err)
	}
	if st.RefCount != 2 {
		t.Errorf("RefCount = %d, want 2 after two Acquire calls from two callers", st.RefCount)
	}
}

// TestDaemon_ReleaseDecrementsRefCount verifies Release decrements the
// shared refcount without stopping the server while other holders remain.
func TestDaemon_ReleaseDecrementsRefCount(t *testing.T) {
	stateDir := t.TempDir()
	callerA := NewDaemon(stateDir)
	callerB := NewDaemon(stateDir)

	factory := func() (string, func() error, error) {
		return "127.0.0.1:8003", func() error { return nil }, nil
	}
	if _, err := callerA.Acquire(factory); err != nil {
		t.Fatalf("callerA.Acquire() error = %v", err)
	}
	if _, err := callerB.Acquire(factory); err != nil {
		t.Fatalf("callerB.Acquire() error = %v", err)
	}

	remaining, err := callerA.Release()
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if remaining != 1 {
		t.Errorf("Release() remaining = %d, want 1", remaining)
	}
}

// TestDaemon_ReleaseToZeroSelfTerminates verifies that when the last holder
// releases, the daemon's stop function is invoked (self-termination,
// REQ-PROXY-004) and the state file is cleared.
func TestDaemon_ReleaseToZeroSelfTerminates(t *testing.T) {
	stateDir := t.TempDir()
	d := NewDaemon(stateDir)

	stopped := false
	factory := func() (string, func() error, error) {
		return "127.0.0.1:8004", func() error { stopped = true; return nil }, nil
	}
	if _, err := d.Acquire(factory); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	remaining, err := d.Release()
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if remaining != 0 {
		t.Errorf("Release() remaining = %d, want 0", remaining)
	}
	if !stopped {
		t.Error("expected the daemon's stop function to be invoked on last release (self-termination)")
	}

	// State file should read back as empty/absent after full release.
	st, err := d.readState()
	if err != nil {
		t.Fatalf("readState() after full release error = %v", err)
	}
	if st.RefCount != 0 {
		t.Errorf("RefCount after full release = %d, want 0", st.RefCount)
	}
}

// TestDaemon_AcquireErrorPropagatesFromFactory verifies a factory error does
// not corrupt daemon state and is returned to the caller.
func TestDaemon_AcquireErrorPropagatesFromFactory(t *testing.T) {
	stateDir := t.TempDir()
	d := NewDaemon(stateDir)

	wantErr := os.ErrPermission
	factory := func() (string, func() error, error) {
		return "", nil, wantErr
	}

	_, err := d.Acquire(factory)
	if err == nil {
		t.Fatal("Acquire() expected error, got nil")
	}
}

// TestDaemon_StateDirIsNotProjectState verifies the daemon state dir is
// independent of any project-scope .moai/state/ directory — a hard
// constraint from design.md §3.4 (machine-scope, not project-scope).
func TestDaemon_StateDirIsNotProjectState(t *testing.T) {
	stateDir := t.TempDir()
	d := NewDaemon(stateDir)
	if d.stateDir == "" {
		t.Fatal("daemon stateDir must not be empty")
	}
	if filepath.Base(filepath.Dir(d.stateDir)) == "state" {
		t.Errorf("daemon stateDir %q must not live under a project .moai/state/ directory", d.stateDir)
	}
}
