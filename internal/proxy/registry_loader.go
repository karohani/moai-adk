package proxy

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultRegistryPath returns the cross-platform machine-registry path
// ~/.moai/config/proxy-groups.yaml, resolved via os.UserHomeDir() at call
// time. It is never baked in as an init-time absolute path (CLAUDE.local.md
// §14) — callers re-resolve it whenever they need the path.
func DefaultRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".moai", "config", "proxy-groups.yaml"), nil
}

// emptyRegistry returns a Registry with initialized (non-nil), empty maps.
func emptyRegistry() *Registry {
	return &Registry{
		Groups: map[string]Group{},
		Sets:   map[string][]string{},
	}
}

// LoadRegistry loads the machine-scoped group registry from path.
//
// A missing file is a NORMAL state — a user who has not yet registered any
// groups — and returns an empty registry with a nil error rather than
// failing the loader (plan.md §D M1.2, design.md §3.1). Any other read or
// parse failure is reported as an error.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyRegistry(), nil
		}
		return nil, fmt.Errorf("read machine registry %s: %w", path, err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse machine registry %s: %w", path, err)
	}
	if reg.Groups == nil {
		reg.Groups = map[string]Group{}
	}
	if reg.Sets == nil {
		reg.Sets = map[string][]string{}
	}
	return &reg, nil
}

// LoadDefaultRegistry resolves DefaultRegistryPath and loads the registry
// from it. A missing file is a normal state — see LoadRegistry.
func LoadDefaultRegistry() (*Registry, error) {
	path, err := DefaultRegistryPath()
	if err != nil {
		return nil, err
	}
	return LoadRegistry(path)
}
