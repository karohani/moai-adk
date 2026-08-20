package proxy

import (
	"fmt"
	"sort"
	"strings"
)

// ResolveDefaultSet implements the flag-less `moai proxy` active-group-set
// resolution order (design.md §3.3, REQ-PROXY-010 + REQ-PROXY-025):
//
//  1. projectDefaultSet (the project's llm.proxy.default_set), if non-empty,
//     is looked up in the machine registry's Sets. A project pointer to a
//     set name absent from the registry is a HARD ERROR (REQ-PROXY-025) — it
//     does NOT fall through to step 2's machine default.
//  2. Otherwise, the machine registry's Sets["default"] is used.
//  3. Otherwise, an error listing the available set names is returned.
//
// The project layer only ever supplies a pointer (a set name); it never
// carries group definitions itself (design.md §3.1).
func ResolveDefaultSet(reg *Registry, projectDefaultSet string) ([]string, error) {
	if projectDefaultSet != "" {
		groups, ok := reg.Sets[projectDefaultSet]
		if !ok {
			return nil, fmt.Errorf(
				"project default_set %q not found in machine registry; available sets: %s",
				projectDefaultSet, listSetNames(reg),
			)
		}
		return groups, nil
	}

	if groups, ok := reg.Sets["default"]; ok {
		return groups, nil
	}

	return nil, fmt.Errorf("no default set configured; available sets: %s", listSetNames(reg))
}

// listSetNames returns a sorted, comma-separated list of the registry's set
// names for use in error messages (or "(none)" when the registry has none).
func listSetNames(reg *Registry) string {
	if len(reg.Sets) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(reg.Sets))
	for name := range reg.Sets {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
