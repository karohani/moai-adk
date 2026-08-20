package proxy

import (
	"fmt"
	"strings"
)

// Deployment is one {group, model} pairing produced by catalog resolution.
//
// REQ-PROXY-013 ([HARD] forward-compat constraint, design.md §4.2): alias
// resolution ALWAYS returns a []Deployment — never a scalar — even when the
// list has length 1. v2 introduces load-balancing/failover strategies that
// consume this list; keeping the slice shape from v1 means v2 needs no
// schema or call-site migration.
type Deployment struct {
	Group string
	Model string
}

// Catalog resolves model identifiers against a registry and an active,
// ordered group set (the set produced by -g / --set / the default-set
// resolution order — see ResolveDefaultSet).
type Catalog struct {
	registry     *Registry
	activeGroups []string
}

// NewCatalog builds a Catalog scoped to the given active group set.
func NewCatalog(reg *Registry, activeGroups []string) *Catalog {
	return &Catalog{registry: reg, activeGroups: activeGroups}
}

// ResolveAlias resolves a 4-alias (opus/sonnet/haiku/fable) into the ordered
// list of {group, model} deployments declared across the active group set
// (REQ-PROXY-013, AC-PROXY-007a). The list order follows the active-group
// set's order; a group that does not declare the alias is skipped.
func (c *Catalog) ResolveAlias(alias string) ([]Deployment, error) {
	deployments := make([]Deployment, 0, len(c.activeGroups))
	for _, groupName := range c.activeGroups {
		group, ok := c.registry.Groups[groupName]
		if !ok {
			continue
		}
		model := group.Aliases.modelFor(alias)
		if model == "" {
			continue
		}
		deployments = append(deployments, Deployment{Group: groupName, Model: model})
	}
	if len(deployments) == 0 {
		return nil, fmt.Errorf("no active group declares alias %q", alias)
	}
	return deployments, nil
}

// SelectRoute is v1's ENTIRE routing decision: read the first entry of the
// ordered deployment list (REQ-PROXY-014, AC-PROXY-007b). No load balancing,
// no failover — this is deliberately a one-line index-0 read so that v2
// replaces only this function's body with a strategy call (design.md §4.2,
// plan.md M1.6).
func SelectRoute(deployments []Deployment) (Deployment, error) {
	if len(deployments) == 0 {
		return Deployment{}, fmt.Errorf("empty deployment list")
	}
	return deployments[0], nil
}

// ResolveDirect resolves a "<group>/<model>" identifier directly against the
// registry, bypassing alias resolution entirely (REQ-PROXY-015,
// AC-PROXY-008).
func (c *Catalog) ResolveDirect(groupModel string) (Deployment, error) {
	group, model, ok := splitGroupModel(groupModel)
	if !ok {
		return Deployment{}, fmt.Errorf("not a <group>/<model> identifier: %q", groupModel)
	}
	if _, exists := c.registry.Groups[group]; !exists {
		return Deployment{}, fmt.Errorf("unknown group %q", group)
	}
	return Deployment{Group: group, Model: model}, nil
}

// Resolve dispatches on the model identifier's shape (design.md §4.3):
//   - "<group>/<model>" → ResolveDirect (alias resolution bypassed)
//   - anything else     → alias resolution + v1 routing (SelectRoute, index 0)
func (c *Catalog) Resolve(modelID string) (Deployment, error) {
	if _, _, ok := splitGroupModel(modelID); ok {
		return c.ResolveDirect(modelID)
	}
	deployments, err := c.ResolveAlias(modelID)
	if err != nil {
		return Deployment{}, err
	}
	return SelectRoute(deployments)
}

// splitGroupModel splits "<group>/<model>" into its two parts. ok is false
// when id carries no '/', or the '/' is a leading or trailing character
// (empty group or empty model).
func splitGroupModel(id string) (group, model string, ok bool) {
	idx := strings.IndexByte(id, '/')
	if idx <= 0 || idx == len(id)-1 {
		return "", "", false
	}
	return id[:idx], id[idx+1:], true
}

// CatalogName formats a catalog entry name as "<group>/<model>"
// (REQ-PROXY-012, AC-PROXY-006). Group names are user-chosen and unique, so
// this cannot collide the way a "<type>/<model>" name would when two groups
// of the same type coexist.
func CatalogName(group, model string) string {
	return group + "/" + model
}

// GroupStatus reports whether a registry group is active for v1 serving.
type GroupStatus struct {
	Name   string
	Active bool
	// Reason is populated when Active is false.
	Reason string
}

// EvaluateGroupStatus classifies every registry group by v1 type support
// (plan.md M1.8, spec.md §H). A group configured with a type lacking a v1
// adapter (currently only GroupTypeCopilot) is isolated as inactive with a
// clear "type not supported in v1" reason; every other group keeps serving —
// the same per-group isolation shape as the credential-failure isolation in
// REQ-PROXY-021.
func EvaluateGroupStatus(reg *Registry) map[string]GroupStatus {
	statuses := make(map[string]GroupStatus, len(reg.Groups))
	for name, g := range reg.Groups {
		if g.Type.SupportedInV1() {
			statuses[name] = GroupStatus{Name: name, Active: true}
			continue
		}
		statuses[name] = GroupStatus{
			Name:   name,
			Active: false,
			Reason: fmt.Sprintf("group %q: type %q not supported in v1", name, g.Type),
		}
	}
	return statuses
}
