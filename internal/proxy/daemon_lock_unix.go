//go:build !windows

package proxy

import (
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

// daemonLock is the unix flock-based advisory lock guarding the machine-scope
// proxy daemon state file (~/.moai/proxy/daemon.json). Mirrors the pattern
// established by internal/session's registryLock (per-package lock copies
// are the established convention in this codebase; see
// internal/session/registry_lock_unix.go).
type daemonLock struct {
	mu sync.Mutex
	fd int
}

func newDaemonLock() *daemonLock {
	return &daemonLock{}
}

// acquire opens the lock companion file at lockPath and applies a
// non-blocking exclusive flock.
func (l *daemonLock) acquire(lockPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return fmt.Errorf("proxy daemon lock open %s: %w", lockPath, err)
	}

	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = unix.Close(fd)
		return fmt.Errorf("proxy daemon lock flock %s: %w", lockPath, err)
	}

	l.fd = fd
	return nil
}

// release releases the flock and closes the underlying fd. Idempotent.
func (l *daemonLock) release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.fd == 0 {
		return nil
	}
	err := unix.Close(l.fd)
	l.fd = 0
	return err
}
