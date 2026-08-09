//go:build !windows

package cli

import (
	"fmt"
	"os"
	"syscall"
)

// execOrSpawnClaude replaces the current process with the claude binary via
// syscall.Exec (execve(2)). On POSIX hosts this is the canonical launch path:
// the current shell process becomes claude, so no defer() runs after this call
// and the parent process identity is preserved.
//
// REQ-CGH-001: syscall.Exec is POSIX-only. The Windows companion
// (launch_exec_windows.go) spawns a child and propagates its exit code instead,
// mirroring the reexecNewBinary pattern in update.go.
func execOrSpawnClaude(claudeBin string, args, env []string) error {
	return execOrSpawnProcess(claudeBin, args, env, "")
}

func execOrSpawnHost(hostBin string, args, env []string, cwd string) error {
	return execOrSpawnProcess(hostBin, args, env, cwd)
}

func execOrSpawnProcess(bin string, args, env []string, cwd string) error {
	if cwd != "" {
		if err := os.Chdir(cwd); err != nil {
			return fmt.Errorf("chdir to launch cwd %q: %w", cwd, err)
		}
	}
	return syscall.Exec(bin, args, env)
}
