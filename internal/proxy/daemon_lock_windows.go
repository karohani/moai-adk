//go:build windows

package proxy

import (
	"fmt"
	"sync"

	"golang.org/x/sys/windows"
)

// daemonLock is the Windows LockFileEx-based advisory lock guarding the
// machine-scope proxy daemon state file. Mirrors
// internal/session/registry_lock_windows.go.
type daemonLock struct {
	mu     sync.Mutex
	handle windows.Handle
}

func newDaemonLock() *daemonLock {
	return &daemonLock{handle: windows.InvalidHandle}
}

func (l *daemonLock) acquire(lockPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	pathW, err := windows.UTF16PtrFromString(lockPath)
	if err != nil {
		return fmt.Errorf("proxy daemon lock utf16 %s: %w", lockPath, err)
	}

	handle, err := windows.CreateFile(
		pathW,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return fmt.Errorf("proxy daemon lock CreateFile %s: %w", lockPath, err)
	}

	const (
		lockFlagsExclusive = 0x00000002 // LOCKFILE_EXCLUSIVE_LOCK
		lockFlagsImmediate = 0x00000001 // LOCKFILE_FAIL_IMMEDIATELY
		maxLen             = 0xFFFFFFFF
	)
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(
		handle,
		lockFlagsExclusive|lockFlagsImmediate,
		0,
		maxLen,
		maxLen,
		&overlapped,
	); err != nil {
		_ = windows.CloseHandle(handle)
		return fmt.Errorf("proxy daemon lock LockFileEx %s: %w", lockPath, err)
	}

	l.handle = handle
	return nil
}

func (l *daemonLock) release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.handle == windows.InvalidHandle {
		return nil
	}
	const maxLen = 0xFFFFFFFF
	var overlapped windows.Overlapped
	unlockErr := windows.UnlockFileEx(l.handle, 0, maxLen, maxLen, &overlapped)
	closeErr := windows.CloseHandle(l.handle)
	l.handle = windows.InvalidHandle
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
