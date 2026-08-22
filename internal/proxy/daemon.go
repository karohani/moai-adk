// Package proxy implements moai proxy — a local multi-backend LLM gateway.
package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// @MX:ANCHOR: [AUTO] Daemon is the singleton-per-machine runtime lifecycle
// primitive for `moai proxy` — every project on the machine shares one
// daemon process via reference counting.
// @MX:REASON: fan_in >= 3 — Acquire/Release are called from every `moai
// proxy` CLI invocation across every project on the machine (M4 wiring),
// and from the daemon package's own self-termination path.

// DaemonState is the persisted runtime state for the moai proxy daemon.
// It lives at a MACHINE-SCOPE path (design.md §3.4), never under a
// project's .moai/state/ directory: multiple unrelated projects on the
// same machine share one daemon process via this file.
type DaemonState struct {
	PID       int       `json:"pid"`
	Address   string    `json:"address"`
	RefCount  int       `json:"ref_count"`
	StartedAt time.Time `json:"started_at"`
}

// DaemonLockTimeout bounds how long Acquire/Release wait for the advisory
// lock before giving up.
const DaemonLockTimeout = 2 * time.Second

// ErrDaemonLockTimeout is returned when lock acquisition exceeds DaemonLockTimeout.
var ErrDaemonLockTimeout = errors.New("proxy daemon: lock acquisition timed out")

// DefaultDaemonStateDir resolves the machine-scope directory the daemon
// persists its runtime state under: ~/.moai/proxy/. This is DISTINCT from
// any project-scope .moai/state/ directory (design.md §3.4).
func DefaultDaemonStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("proxy: resolve home directory: %w", err)
	}
	return filepath.Join(home, ".moai", "proxy"), nil
}

// daemonStateFileName is the state file's basename. The daemon's recorded
// address is a capability handle — anything that can read it can reach an
// unauthenticated endpoint holding every configured backend's credentials —
// and anything that can WRITE it can redirect Claude Code's
// ANTHROPIC_BASE_URL at an attacker-controlled listener. Both surfaces are
// closed by the 0600/0700 modes below rather than by the loopback bind,
// which only excludes remote peers.
const daemonStateFileName = "daemon.json"

// daemonStateFileMode / daemonStateDirMode keep the state file and its
// directory owner-only. The threat this closes is same-user code (a package
// postinstall script, an editor extension, a compromised dependency), which
// is precisely the actor a credential-holding daemon must defend against.
const (
	daemonStateFileMode = 0o600
	daemonStateDirMode  = 0o700
)

func daemonStateFilePath(stateDir string) string {
	return filepath.Join(stateDir, daemonStateFileName)
}

func daemonLockFilePath(stateDir string) string {
	return filepath.Join(stateDir, daemonStateFileName+".lock")
}

// ServerFactory starts a new server instance and returns its bound address,
// a stop function, and an error. The concrete production factory binds a
// loopback-only listener on an unprivileged port (REQ-PROXY-022/023); tests
// inject a fake factory to keep the lifecycle logic hermetic.
type ServerFactory func() (address string, stop func() error, err error)

// Daemon manages the lifecycle (start / reference-count / self-terminate)
// of the shared moai proxy server for a single machine-scope state
// directory. Multiple Daemon instances pointing at the SAME stateDir model
// multiple independent CLI invocations (e.g. from different project working
// directories) coordinating over the shared state file.
type Daemon struct {
	stateDir    string
	lockTimeout time.Duration
	// liveness decides whether a recorded state describes a still-reachable
	// server. Injected for the same reason ServerFactory is: the production
	// implementation performs real network I/O, which hermetic lifecycle
	// tests must be able to replace.
	liveness func(DaemonState) bool

	// mu and stopFuncs travel TOGETHER through every derived copy. Sharing
	// the map while giving each copy its own zero-value mutex would guard
	// one map with two locks — a genuine data race, and a potential
	// "concurrent map read and map write" fatal error.
	mu        *sync.Mutex
	stopFuncs map[string]func() error // address -> stop, held by whichever Daemon instance started the server
}

// NewDaemon returns a Daemon bound to stateDir. Callers normally pass
// DefaultDaemonStateDir(); tests pass a t.TempDir().
func NewDaemon(stateDir string) *Daemon {
	return &Daemon{
		stateDir:    stateDir,
		lockTimeout: DaemonLockTimeout,
		liveness:    daemonStateIsLive,
		mu:          &sync.Mutex{},
		stopFuncs:   make(map[string]func() error),
	}
}

// WithLockTimeout returns a copy of d with a custom lock-acquisition timeout.
func (d *Daemon) WithLockTimeout(timeout time.Duration) *Daemon {
	return &Daemon{
		stateDir:    d.stateDir,
		lockTimeout: timeout,
		liveness:    d.liveness,
		mu:          d.mu,
		stopFuncs:   d.stopFuncs,
	}
}

// withLiveness returns a copy of d using a custom liveness probe. Test-only:
// lifecycle tests inject fake addresses that no listener is bound to, so the
// real dial-based probe would classify every such state as dead.
func (d *Daemon) withLiveness(probe func(DaemonState) bool) *Daemon {
	return &Daemon{
		stateDir:    d.stateDir,
		lockTimeout: d.lockTimeout,
		liveness:    probe,
		mu:          d.mu,
		stopFuncs:   d.stopFuncs,
	}
}

// isLive applies d's liveness probe, defaulting to the production one when
// the Daemon was constructed as a bare struct literal.
func (d *Daemon) isLive(st DaemonState) bool {
	if d.liveness == nil {
		return daemonStateIsLive(st)
	}
	return d.liveness(st)
}

// Acquire ensures a daemon server is running for d's stateDir and increments
// the shared reference count (REQ-PROXY-001, REQ-PROXY-002). If no server is
// currently running (refcount == 0 or no state recorded), Acquire invokes
// factory to start one and persists the resulting address. If a server is
// already running, Acquire reuses its address WITHOUT invoking factory —
// this is the mechanism by which multiple projects share one daemon
// (AC-PROXY-001b).
func (d *Daemon) Acquire(factory ServerFactory) (string, error) {
	var address string
	err := d.withLock(func(st DaemonState) (DaemonState, error) {
		if st.RefCount > 0 && st.Address != "" && d.isLive(st) {
			st.RefCount++
			address = st.Address
			return st, nil
		}

		addr, stop, ferr := factory()
		if ferr != nil {
			return st, fmt.Errorf("proxy daemon: start server: %w", ferr)
		}

		d.mu.Lock()
		d.stopFuncs[addr] = stop
		d.mu.Unlock()

		address = addr
		return DaemonState{
			PID:       os.Getpid(),
			Address:   addr,
			RefCount:  1,
			StartedAt: time.Now().UTC(),
		}, nil
	})
	if err != nil {
		return "", err
	}
	return address, nil
}

// Release decrements the shared reference count (REQ-PROXY-003). When the
// count reaches zero, Release invokes the stop function captured at start
// time (self-termination, REQ-PROXY-004) and clears the persisted state so
// the next Acquire starts a fresh server.
//
// Self-termination is scoped to THIS Daemon instance (and therefore this
// process): only the Daemon instance that originally started the server
// holds its stop function. In the real multi-process deployment (M4), the
// elected starter process is the one whose Acquire call actually invoked
// factory; a caller that only ever reused an existing daemon (like
// callerB in the two-directories test) has no stop function to invoke and
// simply decrements the shared counter — the starter process observes the
// refcount reaching zero itself and terminates. This in-process test
// therefore verifies REQ-PROXY-004 from the perspective of the process that
// started the server; the cross-process signaling path is a documented Gap
// (see progress.md §E.2).
func (d *Daemon) Release() (int, error) {
	var remaining int
	var stop func() error

	err := d.withLock(func(st DaemonState) (DaemonState, error) {
		if st.RefCount <= 0 {
			remaining = 0
			return DaemonState{}, nil
		}
		st.RefCount--
		remaining = st.RefCount
		if st.RefCount == 0 {
			d.mu.Lock()
			stop = d.stopFuncs[st.Address]
			delete(d.stopFuncs, st.Address)
			d.mu.Unlock()
			return DaemonState{}, nil
		}
		return st, nil
	})
	if err != nil {
		return 0, err
	}
	if stop != nil {
		if serr := stop(); serr != nil {
			return remaining, fmt.Errorf("proxy daemon: stop server: %w", serr)
		}
	}
	return remaining, nil
}

// tightenPathMode narrows an existing path's permissions to want, and never
// widens them. Best-effort: a path that does not exist yet, or one this
// process does not own, is left alone — the write that follows carries the
// correct mode for a fresh file anyway.
func tightenPathMode(path string, want os.FileMode) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if cur := info.Mode().Perm(); cur&^want != 0 {
		_ = os.Chmod(path, want)
	}
}

// readState reads the current DaemonState without acquiring the exclusive
// lock (callers that need consistency should go through withLock). File
// absence is normal (no daemon running yet) and returns a zero-value state
// with no error, matching the M1 LoadRegistry file-absence convention.
func (d *Daemon) readState() (DaemonState, error) {
	data, err := os.ReadFile(daemonStateFilePath(d.stateDir))
	if err != nil {
		if os.IsNotExist(err) {
			return DaemonState{}, nil
		}
		return DaemonState{}, fmt.Errorf("proxy daemon: read state: %w", err)
	}
	if len(data) == 0 {
		return DaemonState{}, nil
	}
	var st DaemonState
	if err := json.Unmarshal(data, &st); err != nil {
		return DaemonState{}, fmt.Errorf("proxy daemon: parse state: %w", err)
	}
	return st, nil
}

// writeStateAtomic writes st to the state file via a temp-file-then-rename,
// avoiding partial-write corruption under concurrent readers.
func (d *Daemon) writeStateAtomic(st DaemonState) error {
	if err := os.MkdirAll(d.stateDir, daemonStateDirMode); err != nil {
		return fmt.Errorf("proxy daemon: create state dir: %w", err)
	}
	// MkdirAll/WriteFile only apply their mode when CREATING. An install
	// that predates the tightened modes keeps its 0755 dir and 0644 file
	// forever, so the permissions must be asserted on every write, not
	// merely requested at creation.
	tightenPathMode(d.stateDir, daemonStateDirMode)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("proxy daemon: marshal state: %w", err)
	}
	target := daemonStateFilePath(d.stateDir)
	tmp := target + ".tmp"
	tightenPathMode(daemonStateFilePath(d.stateDir), daemonStateFileMode)
	if err := os.WriteFile(tmp, data, daemonStateFileMode); err != nil {
		return fmt.Errorf("proxy daemon: write temp state: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("proxy daemon: rename state: %w", err)
	}
	return nil
}

// withLock acquires the advisory lock, reads the current state, applies
// mutate, persists the result, and releases the lock. Retries with backoff
// until lockTimeout elapses.
func (d *Daemon) withLock(mutate func(DaemonState) (DaemonState, error)) error {
	if err := os.MkdirAll(d.stateDir, daemonStateDirMode); err != nil {
		return fmt.Errorf("proxy daemon: create state dir: %w", err)
	}
	// MkdirAll/WriteFile only apply their mode when CREATING. An install
	// that predates the tightened modes keeps its 0755 dir and 0644 file
	// forever, so the permissions must be asserted on every write, not
	// merely requested at creation.
	tightenPathMode(d.stateDir, daemonStateDirMode)

	lock := newDaemonLock()
	lockPath := daemonLockFilePath(d.stateDir)

	timeout := d.lockTimeout
	if timeout <= 0 {
		timeout = DaemonLockTimeout
	}
	deadline := time.Now().Add(timeout)
	backoff := 5 * time.Millisecond

	var lastErr error
	for {
		if err := lock.acquire(lockPath); err == nil {
			break
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %v", ErrDaemonLockTimeout, lastErr)
		}
		time.Sleep(backoff)
	}
	defer func() { _ = lock.release() }()

	st, err := d.readState()
	if err != nil {
		return err
	}

	newSt, err := mutate(st)
	if err != nil {
		return err
	}

	return d.writeStateAtomic(newSt)
}
