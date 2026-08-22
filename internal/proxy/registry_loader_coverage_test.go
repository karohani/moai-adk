package proxy

import "testing"

// TestDefaultRegistryPath_HomeDirUnsetIsError verifies the error branch:
// when os.UserHomeDir() cannot resolve a home directory, DefaultRegistryPath
// surfaces the error rather than silently returning an empty path.
//
// t.Setenv forbids t.Parallel() (documented codebase convention — see
// TestDefaultDaemonStateDir_ResolvesUnderHomeDir and CLAUDE.local.md §6).
func TestDefaultRegistryPath_HomeDirUnsetIsError(t *testing.T) {
	t.Setenv("HOME", "")
	_, err := DefaultRegistryPath()
	if err == nil {
		t.Fatal("expected error when HOME is unset, got nil")
	}
}

// TestDefaultDaemonStateDir_HomeDirUnsetIsError mirrors the above for the
// M2 daemon state directory resolver.
func TestDefaultDaemonStateDir_HomeDirUnsetIsError(t *testing.T) {
	t.Setenv("HOME", "")
	_, err := DefaultDaemonStateDir()
	if err == nil {
		t.Fatal("expected error when HOME is unset, got nil")
	}
}

// TestLoadDefaultRegistry_HomeDirUnsetIsError verifies LoadDefaultRegistry
// propagates the DefaultRegistryPath error rather than reading a bogus path.
func TestLoadDefaultRegistry_HomeDirUnsetIsError(t *testing.T) {
	t.Setenv("HOME", "")
	_, err := LoadDefaultRegistry()
	if err == nil {
		t.Fatal("expected error when HOME is unset, got nil")
	}
}
