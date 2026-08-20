package proxy

import "fmt"

// ResolveActiveGroups implements the `moai proxy` CLI flag-resolution
// contract for which registry groups are active this session
// (REQ-PROXY-008..011): `-g <name>` and `--set <name>` are mutually
// exclusive; when neither is given, resolution falls through to the
// ALREADY-IMPLEMENTED M1 ResolveDefaultSet order (project default_set ->
// machine sets.default -> hard error listing available sets).
func ResolveActiveGroups(reg *Registry, gFlags []string, setFlag string, projectDefaultSet string) ([]string, error) {
	if len(gFlags) > 0 && setFlag != "" {
		return nil, fmt.Errorf("proxy: -g and --set are mutually exclusive; choose one")
	}

	if len(gFlags) > 0 {
		for _, name := range gFlags {
			if _, ok := reg.Groups[name]; !ok {
				return nil, fmt.Errorf("proxy: unknown group %q (-g); no such group in the machine registry", name)
			}
		}
		return gFlags, nil
	}

	if setFlag != "" {
		names, ok := reg.Sets[setFlag]
		if !ok {
			return nil, fmt.Errorf("proxy: unknown set %q (--set); available sets: %s", setFlag, listSetNames(reg))
		}
		return names, nil
	}

	return ResolveDefaultSet(reg, projectDefaultSet)
}
