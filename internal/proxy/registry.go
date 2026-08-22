// Package proxy implements the machine-scoped LLM gateway registry and
// catalog resolution for `moai proxy` (SPEC-PROXY-001).
//
// The daemon this package supports is scoped to the machine (user), not the
// project — several projects on the same machine share one daemon and one
// group registry. See design.md §3.1 / §3.4 for the rationale.
package proxy

// GroupType names the backend adapter a registry group is served by.
//
// @MX:ANCHOR: [AUTO] Group type enum consumed by the loader, the catalog resolver, and v1/v2 group-status isolation.
// @MX:REASON: fan_in>=3 — Registry.Groups values, EvaluateGroupStatus, and future backend adapters all switch on this type.
type GroupType string

const (
	// GroupTypeBedrock serves AWS Bedrock Anthropic models via
	// github.com/aws/aws-sdk-go-v2/service/bedrockruntime (M2).
	GroupTypeBedrock GroupType = "bedrock"
	// GroupTypeCodex reads the OpenAI Codex CLI's own credential store
	// read-only and translates through the shared OpenAI<->Anthropic
	// translation layer (M3).
	GroupTypeCodex GroupType = "codex"
	// GroupTypeCopilot is a valid registry type (REQ-PROXY-005) but has NO
	// v1 backend adapter. It is excluded from v1 implementation scope
	// (spec.md §H) while remaining in the schema so a later SPEC can revive
	// it without a schema migration.
	GroupTypeCopilot GroupType = "copilot"
	// GroupTypeLiteLLM passes requests through to an external LiteLLM
	// gateway instance unchanged — no body translation (REQ-PROXY-017).
	GroupTypeLiteLLM GroupType = "litellm"
	// GroupTypeOpenAICompatible serves an OpenAI-compatible endpoint (e.g. a
	// self-hosted GPU cluster) through the shared translation layer (M3).
	GroupTypeOpenAICompatible GroupType = "openai-compatible"
)

// v1SupportedGroupTypes lists the group types with a working v1 backend
// adapter. GroupTypeCopilot is deliberately absent — see GroupTypeCopilot's
// doc comment and spec.md §H.
var v1SupportedGroupTypes = map[GroupType]bool{
	GroupTypeBedrock:          true,
	GroupTypeCodex:            true,
	GroupTypeLiteLLM:          true,
	GroupTypeOpenAICompatible: true,
}

// SupportedInV1 reports whether this group type has a working v1 backend
// adapter. A group configured with an unsupported type is a valid registry
// entry (the enum accepts it) but is isolated as inactive at serve time — see
// EvaluateGroupStatus.
func (t GroupType) SupportedInV1() bool {
	return v1SupportedGroupTypes[t]
}

// AliasBlock maps the four fixed agent model-tier aliases
// (opus/sonnet/haiku/fable) to a group's concrete backend model identifiers.
// This is the same shape as the existing llm.glm.models alias vocabulary
// (design.md §3.2) — it consumes that fixed 4-alias vocabulary, it does not
// extend it.
type AliasBlock struct {
	Opus   string `yaml:"opus,omitempty"`
	Sonnet string `yaml:"sonnet,omitempty"`
	Haiku  string `yaml:"haiku,omitempty"`
	Fable  string `yaml:"fable,omitempty"`
}

// modelFor returns the concrete model identifier for the given alias name,
// or "" when this block does not declare that alias.
func (a AliasBlock) modelFor(alias string) string {
	switch alias {
	case "opus":
		return a.Opus
	case "sonnet":
		return a.Sonnet
	case "haiku":
		return a.Haiku
	case "fable":
		return a.Fable
	default:
		return ""
	}
}

// Group is a single user-named registry entry. Groups are named, not
// type-keyed — the same type MAY appear in several groups (AC-PROXY-003:
// e.g. two Bedrock regions, three GPU clusters).
type Group struct {
	Type GroupType `yaml:"type"`
	// Region is AWS-specific (type: bedrock).
	Region string `yaml:"region,omitempty"`
	// Profile is an AWS profile NAME, never a credential itself
	// (REQ-PROXY-024, design.md §6).
	Profile string `yaml:"profile,omitempty"`
	// BaseURL targets an OpenAI-compatible or LiteLLM endpoint.
	BaseURL string `yaml:"base_url,omitempty"`
	// Aliases maps the 4-alias vocabulary to concrete model identifiers.
	Aliases AliasBlock `yaml:"aliases,omitempty"`
	// Models lists additional models reachable only by direct
	// "<group>/<model>" reference (REQ-PROXY-015), beyond the 4 alias slots.
	Models []string `yaml:"models,omitempty"`
}

// Registry is the machine-scoped SSOT for group definitions and named sets
// (design.md §3.1). It lives at ~/.moai/config/proxy-groups.yaml — NOT under
// .moai/config/sections/, and is never distributed as a template.
type Registry struct {
	// Groups maps a user-chosen group name to its definition.
	Groups map[string]Group `yaml:"groups"`
	// Sets maps a set name to an ordered list of group names.
	Sets map[string][]string `yaml:"sets"`
}
