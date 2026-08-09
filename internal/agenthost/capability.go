// Package agenthost describes how MoAI runtime events map to coding-agent
// host surfaces such as Claude Code, Codex, and OpenCode.
package agenthost

import (
	"fmt"
	"slices"
	"strings"
)

// Host is a supported coding-agent host.
type Host string

const (
	HostClaude   Host = "claude"
	HostCodex    Host = "codex"
	HostOpenCode Host = "opencode"
)

// SupportLevel describes how directly a host supports a MoAI event.
type SupportLevel string

const (
	SupportNative      SupportLevel = "native"
	SupportAdapter     SupportLevel = "adapter"
	SupportFallback    SupportLevel = "fallback"
	SupportUnsupported SupportLevel = "unsupported"
)

// Event is a host-neutral MoAI runtime event.
type Event string

const (
	EventSessionStart      Event = "SessionStart"
	EventPreToolUse        Event = "PreToolUse"
	EventPermissionRequest Event = "PermissionRequest"
	EventPostToolUse       Event = "PostToolUse"
	EventUserPromptSubmit  Event = "UserPromptSubmit"
	EventStop              Event = "Stop"
	EventSubagentStart     Event = "SubagentStart"
	EventSubagentStop      Event = "SubagentStop"
	EventPreCompact        Event = "PreCompact"
	EventPostCompact       Event = "PostCompact"
)

// Feature is a MoAI capability group that may need a host-native surface, an
// adapter, or an explicit fallback path.
type Feature string

const (
	FeatureLauncherRuntime Feature = "launcher/runtime"
	FeatureSlashWorkflows  Feature = "slash_workflows"
	FeatureSpecLifecycle   Feature = "spec_lifecycle"
	FeatureRoleProfiles    Feature = "role_profiles"
	FeatureTmuxDelegation  Feature = "tmux_delegation"
	FeatureHooks           Feature = "hooks"
	FeatureQualityGates    Feature = "quality_gates"
	FeatureHarness         Feature = "harness_lifecycle"
	FeatureStateSession    Feature = "state_session"
	FeatureProjectConfig   Feature = "project_config"
	FeatureGitPR           Feature = "git_pr"
	FeatureSkillsTemplates Feature = "skills_templates"
)

// Mapping describes one host's support for one MoAI runtime event.
type Mapping struct {
	Event       Event        `json:"event"`
	HostEvent   string       `json:"host_event"`
	Support     SupportLevel `json:"support"`
	Source      string       `json:"source"`
	Degradation string       `json:"degradation,omitempty"`
	Notes       string       `json:"notes,omitempty"`
}

// Matrix is the complete event mapping for one host.
type Matrix struct {
	Host     Host      `json:"host"`
	Source   string    `json:"source"`
	Mappings []Mapping `json:"mappings"`
}

// FeatureMapping describes one host's support for a MoAI capability group.
type FeatureMapping struct {
	Feature     Feature      `json:"feature"`
	Support     SupportLevel `json:"support"`
	Surface     string       `json:"surface"`
	Source      string       `json:"source,omitempty"`
	Degradation string       `json:"degradation,omitempty"`
	Notes       string       `json:"notes,omitempty"`
}

// FeatureMatrix is the complete capability-group mapping for one host.
type FeatureMatrix struct {
	Host     Host             `json:"host"`
	Source   string           `json:"source"`
	Features []FeatureMapping `json:"features"`
}

var hostOrder = []Host{HostClaude, HostCodex, HostOpenCode}

var featureOrder = []Feature{
	FeatureLauncherRuntime,
	FeatureSlashWorkflows,
	FeatureSpecLifecycle,
	FeatureRoleProfiles,
	FeatureTmuxDelegation,
	FeatureHooks,
	FeatureQualityGates,
	FeatureHarness,
	FeatureStateSession,
	FeatureProjectConfig,
	FeatureGitPR,
	FeatureSkillsTemplates,
}

// Hosts returns the supported hosts in stable display order.
func Hosts() []Host {
	return slices.Clone(hostOrder)
}

// Features returns the feature inventory in stable display order.
func Features() []Feature {
	return slices.Clone(featureOrder)
}

// ParseHost normalizes a host name.
func ParseHost(raw string) (Host, error) {
	host := Host(strings.ToLower(strings.TrimSpace(raw)))
	switch host {
	case HostClaude, HostCodex, HostOpenCode:
		return host, nil
	default:
		return "", fmt.Errorf("unsupported host %q: expected one of: claude, codex, opencode", raw)
	}
}

// MatrixFor returns the official-doc-grounded event compatibility matrix for a
// host. Source URLs intentionally point at host docs, not MoAI implementation
// files, because this matrix is the boundary between MoAI and external hosts.
func MatrixFor(host Host) (Matrix, error) {
	switch host {
	case HostClaude:
		return Matrix{
			Host:   HostClaude,
			Source: "https://docs.anthropic.com/en/docs/claude-code/hooks",
			Mappings: []Mapping{
				native(EventSessionStart, "SessionStart", "Claude Code hook event"),
				native(EventPreToolUse, "PreToolUse", "Claude Code hook event"),
				native(EventPermissionRequest, "PermissionRequest", "Claude Code hook event"),
				native(EventPostToolUse, "PostToolUse", "Claude Code hook event"),
				native(EventUserPromptSubmit, "UserPromptSubmit", "Claude Code hook event"),
				native(EventStop, "Stop", "Claude Code hook event"),
				native(EventSubagentStart, "SubagentStart", "Claude Code hook event"),
				native(EventSubagentStop, "SubagentStop", "Claude Code hook event"),
				native(EventPreCompact, "PreCompact", "Claude Code hook event"),
				native(EventPostCompact, "PostCompact", "Claude Code hook event"),
			},
		}, nil
	case HostCodex:
		return Matrix{
			Host:   HostCodex,
			Source: "https://developers.openai.com/codex/hooks",
			Mappings: []Mapping{
				native(EventSessionStart, "SessionStart", "Codex hook event; matcher filters startup|resume|clear|compact"),
				native(EventPreToolUse, "PreToolUse", "Codex hook event; matcher filters Bash, apply_patch/Edit/Write, and MCP tools"),
				native(EventPermissionRequest, "PermissionRequest", "Codex hook event"),
				native(EventPostToolUse, "PostToolUse", "Codex hook event"),
				native(EventUserPromptSubmit, "UserPromptSubmit", "Codex hook event; matcher is ignored"),
				native(EventStop, "Stop", "Codex hook event; matcher is ignored"),
				native(EventSubagentStart, "SubagentStart", "Codex hook event; matcher filters subagent type"),
				native(EventSubagentStop, "SubagentStop", "Codex hook event; matcher filters subagent type"),
				native(EventPreCompact, "PreCompact", "Codex hook event"),
				native(EventPostCompact, "PostCompact", "Codex hook event"),
			},
		}, nil
	case HostOpenCode:
		return Matrix{
			Host:   HostOpenCode,
			Source: "https://opencode.ai/docs/plugins/",
			Mappings: []Mapping{
				adapter(EventSessionStart, "session.created", "OpenCode plugin event", "No direct SessionStart hook payload parity; adapter must synthesize MoAI session fields."),
				adapter(EventPreToolUse, "tool.execute.before", "OpenCode plugin event", "Use plugin mutation/throw plus OpenCode permission rules instead of Claude/Codex permissionDecision JSON."),
				adapter(EventPermissionRequest, "permission.asked", "OpenCode plugin event", "Pair with permission.replied for observation; approval UI semantics differ."),
				adapter(EventPostToolUse, "tool.execute.after|file.edited", "OpenCode plugin events", "Split tool-result observation and file-edit observation."),
				fallback(EventUserPromptSubmit, "tui.prompt.append", "OpenCode TUI event", "No official prompt-submitted lifecycle event found; keep explicit /moai or $skill routing as fallback."),
				fallback(EventStop, "session.idle", "OpenCode session event", "Approximate turn completion; quality gate should also run via explicit moai gate/sync fallback."),
				fallback(EventSubagentStart, "session.created", "OpenCode session event", "Subagent child sessions exist, but no dedicated SubagentStart lifecycle hook is documented."),
				fallback(EventSubagentStop, "session.idle", "OpenCode session event", "Subagent stop must be inferred from child session idle/status metadata."),
				adapter(EventPreCompact, "experimental.session.compacting", "OpenCode experimental plugin hook", "OpenCode exposes a pre-compaction mutation point under an experimental event name."),
				adapter(EventPostCompact, "session.compacted", "OpenCode session event", "Post-compaction observation only; prompt/context mutation belongs to experimental.session.compacting."),
			},
		}, nil
	default:
		return Matrix{}, fmt.Errorf("unsupported host %q", host)
	}
}

// AllMatrices returns every host matrix in stable order.
func AllMatrices() []Matrix {
	out := make([]Matrix, 0, len(hostOrder))
	for _, host := range hostOrder {
		matrix, err := MatrixFor(host)
		if err == nil {
			out = append(out, matrix)
		}
	}
	return out
}

// FeatureMatrixFor returns the host compatibility matrix at MoAI feature-group
// granularity. It is intentionally more conservative than the event matrix:
// native hooks alone do not imply native support for every MoAI workflow.
func FeatureMatrixFor(host Host) (FeatureMatrix, error) {
	switch host {
	case HostClaude:
		return FeatureMatrix{
			Host:   HostClaude,
			Source: "https://docs.anthropic.com/en/docs/claude-code/overview",
			Features: []FeatureMapping{
				featureNative(FeatureLauncherRuntime, "claude CLI", "Claude is the original MoAI runtime target."),
				featureNative(FeatureSlashWorkflows, "Claude slash commands and skills", "MoAI workflows were authored around Claude-compatible command and skill surfaces."),
				featureNative(FeatureSpecLifecycle, ".moai/specs files", "SPEC files are host-independent and fully driven by the MoAI workflow."),
				featureNative(FeatureRoleProfiles, "Claude agents and team profiles", "Role profiles map directly to the existing Claude team/subagent runtime."),
				featureNative(FeatureTmuxDelegation, "OMX tmux runtime", "Existing OMX team panes are Claude-first."),
				featureNative(FeatureHooks, "Claude Code hooks", "Claude exposes the complete hook event set used by MoAI."),
				featureNative(FeatureQualityGates, "moai gate and language toolchains", "Quality gates are owned by MoAI and already integrated with Claude workflows."),
				featureNative(FeatureHarness, "MoAI harness artifacts", "Harness command, specialist, skill, and workflow artifacts target Claude-compatible paths."),
				featureNative(FeatureStateSession, ".omx and Claude session state", "Existing session state and hook persistence are Claude-first."),
				featureNative(FeatureProjectConfig, ".moai config plus CLAUDE.md/AGENTS.md", "Project configuration is fully represented for Claude operation."),
				featureNative(FeatureGitPR, "git and gh via MoAI", "Git/PR automation is host-independent once driven from Claude workflows."),
				featureNative(FeatureSkillsTemplates, "Claude skills and templates", "Template installation writes Claude-compatible skill and command files."),
			},
		}, nil
	case HostCodex:
		return FeatureMatrix{
			Host:   HostCodex,
			Source: "https://developers.openai.com/codex/cli/reference",
			Features: []FeatureMapping{
				featureNative(FeatureLauncherRuntime, "moai codex -> codex CLI", "MoAI can build native Codex argv without a generic shell shim."),
				featureAdapter(FeatureSlashWorkflows, "AGENTS.md + MoAI CLI workflows", "Codex does not consume Claude slash-command files directly; route through AGENTS.md and moai subcommands."),
				featureNative(FeatureSpecLifecycle, ".moai/specs files", "SPEC artifacts remain host-independent."),
				featureAdapter(FeatureRoleProfiles, "workflow.yaml host + Codex subagent roles", "Role profiles select Codex as host, then rely on Codex-native agent/prompt surfaces."),
				featureAdapter(FeatureTmuxDelegation, "tmux pane running codex", "Delegation is process-native but OMX team runtime still needs host-aware launch glue."),
				featureNative(FeatureHooks, "Codex hooks", "Codex exposes the hook event set needed for MoAI hook parity."),
				featureNative(FeatureQualityGates, "moai gate and language toolchains", "Quality gates are host-independent CLI checks."),
				featureAdapter(FeatureHarness, "MoAI harness artifacts with Codex adapters", "Harness generation must target Codex command/skill paths instead of Claude-only paths."),
				featureAdapter(FeatureStateSession, ".omx/.moai state + Codex sessions", "State files are shared, but session lifecycle integration is host-specific."),
				featureAdapter(FeatureProjectConfig, "AGENTS.md and .codex config", "Project config must be rendered into Codex-native guidance/config locations."),
				featureNative(FeatureGitPR, "git and gh via MoAI", "Git/PR automation is host-independent."),
				featureAdapter(FeatureSkillsTemplates, "shared skills plus .codex projection", "Skills need a Codex projection or symlink strategy."),
			},
		}, nil
	case HostOpenCode:
		return FeatureMatrix{
			Host:   HostOpenCode,
			Source: "https://opencode.ai/docs/cli/",
			Features: []FeatureMapping{
				featureNative(FeatureLauncherRuntime, "moai opencode -> opencode CLI", "MoAI can build native OpenCode argv including run, --dir, --agent, and session flags."),
				featureFallback(FeatureSlashWorkflows, "explicit moai CLI commands", "OpenCode does not document Claude-style slash workflow execution; use moai commands as the stable entry point."),
				featureAdapter(FeatureSpecLifecycle, ".moai/specs files", "SPEC files are usable, but workflow invocation must be bridged into OpenCode."),
				featureNative(FeatureRoleProfiles, "workflow.yaml host + OpenCode agents", "Role profiles can select OpenCode and map roles to OpenCode agents."),
				featureAdapter(FeatureTmuxDelegation, "tmux pane running opencode", "OpenCode can run in tmux, but MoAI team routing still needs host-aware launch/session glue."),
				featureAdapter(FeatureHooks, "OpenCode plugins", "OpenCode plugin events cover key points but do not provide full Claude/Codex hook parity."),
				featureNative(FeatureQualityGates, "moai gate and language toolchains", "Quality gates are host-independent CLI checks."),
				featureAdapter(FeatureHarness, "MoAI harness artifacts with OpenCode adapters", "Harness outputs need OpenCode agent/plugin/config projection."),
				featureFallback(FeatureStateSession, "OpenCode session ids plus .moai state", "Session lifecycle must be reconciled manually until a first-class adapter exists."),
				featureAdapter(FeatureProjectConfig, "opencode config and AGENTS.md", "OpenCode config/agent files can receive projected MoAI role settings."),
				featureNative(FeatureGitPR, "git and gh via MoAI", "Git/PR automation is host-independent."),
				featureAdapter(FeatureSkillsTemplates, "shared skills projected to OpenCode agents/plugins", "Skills require an OpenCode projection rather than direct reuse of Claude command files."),
			},
		}, nil
	default:
		return FeatureMatrix{}, fmt.Errorf("unsupported host %q", host)
	}
}

// AllFeatureMatrices returns every host feature matrix in stable order.
func AllFeatureMatrices() []FeatureMatrix {
	out := make([]FeatureMatrix, 0, len(hostOrder))
	for _, host := range hostOrder {
		matrix, err := FeatureMatrixFor(host)
		if err == nil {
			out = append(out, matrix)
		}
	}
	return out
}

// Find returns the mapping for one event.
func (m Matrix) Find(event Event) (Mapping, bool) {
	for _, mapping := range m.Mappings {
		if mapping.Event == event {
			return mapping, true
		}
	}
	return Mapping{}, false
}

// Find returns the mapping for one feature.
func (m FeatureMatrix) Find(feature Feature) (FeatureMapping, bool) {
	for _, mapping := range m.Features {
		if mapping.Feature == feature {
			return mapping, true
		}
	}
	return FeatureMapping{}, false
}

func native(event Event, hostEvent, notes string) Mapping {
	return Mapping{Event: event, HostEvent: hostEvent, Support: SupportNative, Notes: notes}
}

func adapter(event Event, hostEvent, notes, degradation string) Mapping {
	return Mapping{
		Event:       event,
		HostEvent:   hostEvent,
		Support:     SupportAdapter,
		Notes:       notes,
		Degradation: degradation,
	}
}

func featureNative(feature Feature, surface, notes string) FeatureMapping {
	return FeatureMapping{
		Feature: feature,
		Support: SupportNative,
		Surface: surface,
		Notes:   notes,
	}
}

func featureAdapter(feature Feature, surface, degradation string) FeatureMapping {
	return FeatureMapping{
		Feature:     feature,
		Support:     SupportAdapter,
		Surface:     surface,
		Degradation: degradation,
	}
}

func featureFallback(feature Feature, surface, degradation string) FeatureMapping {
	return FeatureMapping{
		Feature:     feature,
		Support:     SupportFallback,
		Surface:     surface,
		Degradation: degradation,
	}
}

func fallback(event Event, hostEvent, notes, degradation string) Mapping {
	return Mapping{
		Event:       event,
		HostEvent:   hostEvent,
		Support:     SupportFallback,
		Notes:       notes,
		Degradation: degradation,
	}
}
