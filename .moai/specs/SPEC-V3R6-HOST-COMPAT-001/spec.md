---
id: SPEC-V3R6-HOST-COMPAT-001
title: "Claude/Codex/OpenCode Compatibility Spine"
version: "0.3.0"
status: in-progress
created: 2026-07-05
updated: 2026-08-10
author: manager-spec
priority: P1
phase: "v3.1.0 target"
module: "internal/agenthost, internal/cli, internal/config, internal/template, .moai/config/sections"
lifecycle: spec-anchored
tags: "agenthost, codex, opencode, claude, tmux, workflow, hooks, templates"
tier: L
---

# SPEC-V3R6-HOST-COMPAT-001: Claude/Codex/OpenCode Compatibility Spine

## HISTORY

| Version | Date | Author | Description |
|---------|------|--------|-------------|
| 0.1.0 | 2026-07-05 | manager-spec | Initial draft for making the existing MoAI feature surface work across Claude, Codex, and OpenCode with role-level host routing. |
| 0.2.0 | 2026-08-09 | manager-spec | Plan-audit remediation (audit 2026-08-09, FAIL 0.50 vs Tier L 0.85). Pinned the OpenCode config path to root `opencode.json`; corrected `.opencode/agent` to the canonical plural `.opencode/agents`; enumerated the Phase 5 template file set; deleted the unfalsifiable escape clauses in REQ-AH-011 and REQ-AH-015; added the Codex project-layer trust degradation note to REQ-AH-013. Removed the shared-skill slice (REQ-AH-010) and the two unbuilt-surface requirements (REQ-AH-014, REQ-AH-017) from scope — each retained in place as a `[REMOVED FROM SCOPE]` marker so REQ numbering stays contiguous, with rationale in §2.2. |

## 1. Goal

Make the current MoAI feature surface usable across three coding-agent hosts:

- Claude for planning/orchestration when selected.
- Codex for implementation work when selected.
- OpenCode as a first-class native host, not a generic shell fallback.

The core user scenario is: a project YAML can say which host owns each role, tmux can delegate sessions to those hosts, and the same MoAI workflow concepts remain available through host-appropriate templates, commands, hooks, and skills.

## 2. Scope

### 2.1 In Scope

- Host model: `claude`, `codex`, `opencode`.
- Project YAML schema for default host and role-level host selection.
- Structured command builders for Claude, Codex, and OpenCode.
- Native launcher commands or aliases for Codex and OpenCode.
- Tmux/worktree role delegation based on role host.
- Codex project templates: hooks plus a shared `AGENTS.md` instruction file. No Codex `config.toml` is shipped (see §2.2).
- OpenCode project templates: root `opencode.json`, `.opencode/agents`, `.opencode/plugins`, and the shared `AGENTS.md` instruction file.
- Host matrix that covers both hooks and feature groups.
- Tests proving command calculation, config parsing, template rendering, hook matrix behavior, and tmux launch selection.
- Documentation explaining exact native/adapted/fallback/unsupported behavior.

### 2.2 Out of Scope

- Rewriting the internal MoAI workflow engine.
- Full parity for every Claude-only hook where the target host has no equivalent official event.
- CI review bot resurrection from old Codex/Gemini/GLM workflows.
- Adding new external dependencies unless explicitly approved.
- Executing real Codex/OpenCode sessions in unit tests. Tests use command calculation and smoke stubs.
- Shared-skill canonicalisation from `.agents/skills` — see the deferral sub-section below.
- A Codex `config.toml` project template — see the deferral sub-section below.
- A `moai host doctor`-style diagnostic command — see the deferral sub-section below.
- Per-workflow host classification of the 13 `/moai` slash workflows — see the deferral sub-section below.

### Out of Scope — Shared-skill canonicalisation (REQ-AH-010, AC-AH-013 removed)

- `.agents/skills` is NOT formalized as the canonical cross-host skill source by this SPEC. REQ-AH-010 and AC-AH-013 are removed from scope and recorded in §2.3 Deferred Backlog → entry D-1.
- Rationale 1: `plan.md` §A.1 already classified this slice as optional pending an ownership and update-semantics review. That review has not happened, so the config key, its default, and the Windows symlink fallback remain uncontracted.
- Rationale 2: the mirroring is not load-bearing for OpenCode. OpenCode reads Claude Code skills natively from `.claude/skills` (opt-out via `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS`; the broader opt-outs are `OPENCODE_DISABLE_CLAUDE_CODE` and `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT`). Source: https://opencode.ai/docs/
- Consequence: `.agents/` is left untouched by this SPEC — no template tree entry, no `.gitignore` change, no copy/symlink policy. Existing `.claude/skills` behavior is preserved unchanged.

### Out of Scope — Codex `config.toml` project template

- No `.codex/config.toml` template is shipped. Codex config is TOML-based with user and project layers (source: https://developers.openai.com/codex/config-basic), and those layers carry user-owned model, approval, and provider settings.
- Rationale: REQ-AH-018 confines user-global host configuration changes to explicit opt-in. Shipping a project `config.toml` template would write opinionated model/approval defaults into a surface the user owns.
- The Codex `.codex/hooks.json` template remains in scope (REQ-AH-011).

### Out of Scope — Host diagnostic command (REQ-AH-017 removed)

- No `moai host doctor` / `moai host validate` diagnostic subcommand is added. The command was never named in any artifact and no such surface exists — the implemented CLI surface is `moai host matrix` only.
- Rationale: naming and building a new diagnostic subcommand expands the current milestone (Phase 5 templates + Phase 6 remainder) into new CLI surface area.
- The dry-run half of the original requirement is retained and remains in scope under REQ-AH-008 (`moai codex --dry-run`, `moai opencode --dry-run`), verified by AC-AH-007.

### Out of Scope — Per-workflow slash-workflow classification (REQ-AH-014 removed)

- The 13 `/moai` workflows are NOT individually classified as native / shared-instruction / fallback / unsupported by this SPEC.
- Rationale: the feature-surface matrix delivered under REQ-AH-002 already carries a `slash_workflows` group at feature-group granularity. Per-workflow granularity is a different axis and would re-open the completed Phase 6 matrix work.
- Recorded in §2.3 Deferred Backlog → entry D-2, together with the host-coverage documentation axis it shares a pickup trigger with.

### 2.3 Deferred Backlog

This section is the single named tracking location for every requirement this SPEC descoped. It exists because "deferred to a follow-up SPEC" with no named successor is how scope is silently dropped rather than genuinely deferred. No successor SPEC ID is allocated here: the three entries are heterogeneous (skill distribution, documentation axis, CLI surface), and each one's gating question is unanswered — allocating an ID now would invent a scope that nobody has reviewed. `spec.md` is `lifecycle: spec-anchored` and is therefore maintained after this SPEC closes, so this section remains a live handle.

Each entry names its origin requirement, the open questions that must be answered before it can be planned, and the concrete trigger that should cause someone to pick it up.

#### D-1: Shared-skill canonicalisation

- Origin: REQ-AH-010 (removed from scope), AC-AH-013 (retired in place). Rationale: §2.2 → "Out of Scope — Shared-skill canonicalisation".
- Open questions, all uncontracted: the config key name and its YAML location; the key's enum values; the key's default; the Windows symlink fallback behavior; and the ownership and update semantics of a mirrored skill tree (which side wins when `.claude/skills` and a mirror diverge).
- Pickup trigger: an ownership and update-semantics review concludes, **or** OpenCode's native `.claude/skills` reading (opt-out `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS`) stops being sufficient for a host MoAI wants to support. Until then the mirroring is not load-bearing and the deferral is safe rather than merely postponed.

#### D-2: Per-workflow slash-workflow classification and host-coverage documentation

- Origin: REQ-AH-014 (removed from scope). Rationale: §2.2 → "Out of Scope — Per-workflow slash-workflow classification".
- Open questions: whether per-workflow granularity is the right axis at all given that REQ-AH-002's matrix already carries a `slash_workflows` feature group; and how a 13-entry per-workflow table would be kept from drifting against that group-level matrix.
- Pickup trigger: the feature-group-level `slash_workflows` entry proves too coarse for a real user question — that is, someone needs to know whether a *specific* workflow runs on a given host and the group-level matrix cannot answer it.

#### D-3: Host diagnostic command

- Origin: REQ-AH-017 (removed from scope). Rationale: §2.2 → "Out of Scope — Host diagnostic command".
- Open questions: the subcommand's name (no surface was ever named in any artifact — `moai host doctor` and `moai host validate` were both hypothetical); what it would check beyond what `moai host matrix` already reports; and whether it is a new subcommand at all rather than a flag on the existing `moai host` surface.
- Pickup trigger: users report that `moai host matrix` plus the `--dry-run` surfaces retained under REQ-AH-008 leave a diagnosable failure mode uncovered.

## 3. Requirements

### REQ-AH-001: Canonical Host Registry

[Ubiquitous] The system SHALL define a canonical coding-agent host registry with exactly these initial host IDs:

- `claude`
- `codex`
- `opencode`

The registry SHALL expose parsing, display names, supported feature groups, and support levels: `native`, `adapter`, `fallback`, `unsupported`.

### REQ-AH-002: Feature-Surface Compatibility Matrix

[Ubiquitous] The system SHALL track host compatibility for exactly these **12** feature groups. This list is the single source of truth for the feature-group set; `research.md` §1 and `acceptance.md` AC-AH-012 both anchor to it, and the serialized identifier column is the exact string emitted in the matrix JSON.

| # | Feature group | Canonical serialized identifier |
|---|---------------|--------------------------------|
| 1 | launcher/runtime | `launcher/runtime` |
| 2 | slash workflows | `slash_workflows` |
| 3 | SPEC lifecycle | `spec_lifecycle` |
| 4 | role profiles | `role_profiles` |
| 5 | worktree/tmux | `tmux_delegation` |
| 6 | hooks | `hooks` |
| 7 | quality gates | `quality_gates` |
| 8 | harness lifecycle | `harness_lifecycle` |
| 9 | state/session | `state_session` |
| 10 | config/project docs | `project_config` |
| 11 | Git/GitHub/PR | `git_pr` |
| 12 | skills/templates | `skills_templates` |

Note that group 1 is the only identifier using a `/` separator; the remaining 11 use `_`. This asymmetry is intentional to record — a set-equality assertion must reproduce it verbatim.

The existing hook matrix SHALL remain, but the CLI SHALL also be able to report this broader feature-surface matrix.

### REQ-AH-003: Project YAML Host Selection

[State-driven] While `.moai/config/sections/workflow.yaml` declares host settings, the system SHALL resolve the host for each role using this precedence:

1. `workflow.team.role_profiles.<role>.host`
2. `workflow.default_host`
3. existing `llm.team_mode` compatibility fallback
4. `claude`

The resolver SHALL reject unknown hosts with an actionable error listing the supported hosts.

### REQ-AH-004: Planning-vs-Coding Role Split

[Ubiquitous] The system SHALL support this project-level configuration pattern:

```yaml
workflow:
  default_host: claude
  team:
    role_profiles:
      architect:
        host: claude
      analyst:
        host: claude
      implementer:
        host: codex
      tester:
        host: codex
      reviewer:
        host: opencode
```

This pattern SHALL be documented as the recommended starting point for "Claude plans, Codex works, OpenCode is natively available".

### REQ-AH-005: Structured Command Builders

[Ubiquitous] Host launch commands SHALL be generated by structured argv builders, not by ad hoc shell string concatenation.

Each builder SHALL accept:

- project root
- prompt or workflow command
- role profile
- model
- sandbox/approval mode
- session or attach mode
- dry-run flag

The builder output SHALL be testable without executing the host binary.

### REQ-AH-006: Native Codex Command Builder

[Event-driven] When a role resolves to `codex`, the command builder SHALL produce a Codex-native command using official Codex CLI surfaces.

The builder SHALL support, where configured:

- `codex` interactive launch
- `codex exec` for non-interactive workflow execution
- `--cd`
- `--model`
- `--profile`
- `--sandbox`
- `--ask-for-approval`
- `--config`

The builder SHALL not claim support for a Codex option that is not backed by official documentation or an explicit compatibility test.

### REQ-AH-007: Native OpenCode Command Builder

[Event-driven] When a role resolves to `opencode`, the command builder SHALL produce an OpenCode-native command using official OpenCode CLI surfaces.

The builder SHALL support, where configured:

- `opencode run`
- `--dir`
- `--agent`
- `--model`
- `--session`
- `--continue`
- `--attach`
- `--auto`

OpenCode support SHALL be implemented through an explicit adapter, enum, and command builder. It SHALL NOT be represented as a generic shell command.

### REQ-AH-008: Native Launcher Commands

[Event-driven] When the user runs a host launcher, the CLI SHALL route natively:

- `moai cc` remains Claude-compatible.
- `moai codex` launches the Codex host path.
- `moai opencode` launches the OpenCode host path.

If the host binary is missing, the command SHALL fail before session mutation and print install/configuration guidance.

### REQ-AH-009: Tmux Role Delegation

[Event-driven] When tmux delegation starts a role session, the launch command SHALL come from the resolved role host command builder.

Examples:

- `architect` host `claude` launches Claude-compatible planning.
- `implementer` host `codex` launches Codex in the target worktree.
- `reviewer` host `opencode` launches OpenCode with the configured agent.

Existing `cc/glm/cg` behavior SHALL remain backward-compatible.

### REQ-AH-010: Shared Skill Installation — [REMOVED FROM SCOPE]

**This requirement is removed from the scope of this SPEC** and recorded in §2.3 Deferred Backlog → entry D-1, which carries its open questions and pickup trigger. See §2.2 → "Out of Scope — Shared-skill canonicalisation" for the full rationale. Its acceptance criterion (AC-AH-013) is retired in place.

The heading is retained so REQ-AH-001..REQ-AH-018 numbering stays contiguous and so downstream references resolve to an explicit removal record rather than to a gap.

No implementation work is authorized under this requirement. `.claude/skills` behavior is preserved exactly as it is today; `.agents/skills` is untouched.

### REQ-AH-011: Codex Templates

[Event-driven] When `moai init` or `moai update` installs Codex support, the project SHALL receive exactly these Codex-native artifacts:

- `.codex/hooks.json` — rendered from the existing `internal/template/templates/.codex/hooks.json.tmpl`.
- `AGENTS.md` at the project root — the shared instruction file. Codex reads `AGENTS.md` for project instructions; OpenCode reads the same file (falling back to `CLAUDE.md` when `AGENTS.md` is absent). One template therefore serves both hosts and SHALL NOT be duplicated per host. Source: https://developers.openai.com/codex/ and https://opencode.ai/docs/
- tests proving the rendered `.codex/hooks.json` is valid JSON.

No Codex `config.toml` template SHALL be shipped (see §2.2 → "Out of Scope — Codex `config.toml` project template").

The existing `.codex/hooks.json.tmpl` SHALL remain compatible with the 10 core host-neutral hook events. That set is **exactly** the following, and this enumeration is the single source of truth every other artifact binds to:

| # | Event |
|---|-------|
| 1 | `SessionStart` |
| 2 | `PreToolUse` |
| 3 | `PermissionRequest` |
| 4 | `PostToolUse` |
| 5 | `UserPromptSubmit` |
| 6 | `Stop` |
| 7 | `SubagentStart` |
| 8 | `SubagentStop` |
| 9 | `PreCompact` |
| 10 | `PostCompact` |

Count: 10 — no more, no fewer. The set is enumerated here rather than left as a bare cardinal so that a test author can write the assertion without reading the implementation. Evidence: the `Event` constants declared in `internal/agenthost/capability.go`. Any change to this event set is a separate SPEC — a change made here without a corresponding SPEC would silently desynchronise the template, the matrix, and AC-AH-009.

### REQ-AH-012: OpenCode Templates and Plugin

[Event-driven] When `moai init` or `moai update` installs OpenCode support, the project SHALL receive exactly these OpenCode-native artifacts:

- `opencode.json` at the **project root**. This path is pinned, not deferred: the OpenCode docs state "Add `opencode.json` in your project root. Project config has the highest precedence among standard config files." `opencode.jsonc` (JSON with comments) is an accepted alternate format of the same root file; `.opencode/opencode.json` is NOT a config location. The file SHALL declare `"$schema": "https://opencode.ai/config.json"`. Source: https://opencode.ai/docs/config/
- `.opencode/agents/moai-reviewer.md` for the MoAI-facing agent. The directory name is the canonical **plural** `agents` — the OpenCode docs state that `.opencode` and `~/.config/opencode` use plural subdirectory names (`agents/`, `commands/`, `modes/`, `plugins/`, `skills/`, `tools/`, `themes/`), with singular forms retained only for backwards compatibility. The rule for which agent wrappers exist: one wrapper per role that REQ-AH-004's recommended split assigns to `opencode` — currently `reviewer` only. Source: https://opencode.ai/docs/agents/
- `.opencode/plugins/moai-hooks.js` for hook/event adaptation. OpenCode plugins live in `.opencode/plugins/` (plural, project scope) and may be JavaScript or TypeScript; JavaScript is chosen so user projects need no build step. Source: https://opencode.ai/docs/plugins/
- the shared root `AGENTS.md` instruction file (the same artifact required by REQ-AH-011 — one file, not one per host), referenced through the `instructions` config key in `opencode.json`.

The OpenCode plugin SHALL forward supported events to the MoAI hook CLI with `MOAI_HOOK_HOST=opencode`. "Supported events" is defined as exactly the set of events that the hook matrix (REQ-AH-013) records for host `opencode` at support level `native` or `adapter` — that is, adapter-or-better. Events the matrix records as `fallback` or `unsupported` are not forwarded, so the forwarded set is enumerable from the matrix rather than left to implementation choice.

### REQ-AH-013: Hook Support Truthfulness

[Unwanted] If a host event is adapter-based, fallback-based, or unsupported, the system SHALL NOT report it as native.

The matrix SHALL include, for every mapping:

- host event name
- support level
- degradation note where support is not native
- source URL or local evidence

Specifically, a project-scoped `.codex/hooks.json` mapping SHALL NOT be reported as unconditionally `native`. Codex loads project-local hooks **only when the project `.codex/` layer is trusted**; in an untrusted project Codex still loads user and system hooks from their own active config layers, but the project hooks do not load. That conditionality SHALL be surfaced as the mapping's degradation note. Source: https://developers.openai.com/codex/hooks

### REQ-AH-014: Slash Workflow Coverage — [REMOVED FROM SCOPE]

**This requirement is removed from the scope of this SPEC** and recorded in §2.3 Deferred Backlog → entry D-2, which carries its open questions and pickup trigger. See §2.2 → "Out of Scope — Per-workflow slash-workflow classification" for the full rationale.

The heading is retained so REQ-AH-001..REQ-AH-018 numbering stays contiguous.

Feature-group-level coverage of slash workflows remains in scope under REQ-AH-002 via the `slash_workflows` feature group; only the per-workflow (13-entry) classification axis is deferred.

### REQ-AH-015: Harness and Dev-Only Boundary

[Ubiquitous] User-distributed harness lifecycle features SHALL be separated from repository-only harness development commands.

The multi-host template installer SHALL NOT distribute dev-only harness commands. This prohibition is unconditional — there is no configuration, environment variable, or flag that enables distribution. It is already enforced for the `.claude/` tree by the standing static guard `TestSplitHarnessNamespaceNoLeak` in `internal/template/split_namespace_test.go` (sentinel `SPLIT_HARNESS_NAMESPACE_LEAK`), which consults no flag; this SPEC extends the same unconditional guard to the new `.codex/` and `.opencode/` template trees.

### REQ-AH-016: Configuration Migration

[Event-driven] When existing projects are updated, the migration SHALL add host fields without breaking existing `workflow.yaml` files.

Existing projects without host fields SHALL behave exactly as before.

### REQ-AH-017: Validation and Dry Run — [REMOVED FROM SCOPE]

**This requirement is removed from the scope of this SPEC** and recorded in §2.3 Deferred Backlog → entry D-3, which carries its open questions and pickup trigger. See §2.2 → "Out of Scope — Host diagnostic command" for the full rationale.

The heading is retained so REQ-AH-001..REQ-AH-018 numbering stays contiguous.

The dry-run surface remains in scope under REQ-AH-008 (`moai codex --dry-run` and `moai opencode --dry-run`), verified by AC-AH-007. Only the unbuilt diagnostic subcommand is deferred.

### REQ-AH-018: Security and Side-Effect Boundaries

[Ubiquitous] Host launchers SHALL not print secrets, write credentials, or mutate user-global host config unless the command explicitly says it is doing so.

Project templates SHALL be written under the project root. User-global setup remains opt-in.

## 4. Acceptance Summary

The implementation is accepted when:

- host schema parsing tests pass
- command builder tests pass for Claude, Codex, and OpenCode
- tmux role launch tests prove host-specific commands are selected
- Codex and OpenCode templates render valid config files
- host matrix JSON covers both hooks and all 12 feature groups of REQ-AH-002
- existing `cc/glm/cg` tests remain green
- docs explain native, adapter, fallback, and unsupported behavior without overstating parity

The full enumeration lives in `acceptance.md` as AC-AH-001..AC-AH-018, each carrying an explicit `REQ-AH-xxx` back-reference. AC-AH-013 is retired in place (its requirement, REQ-AH-010, is removed from scope); the three requirements removed from scope (REQ-AH-010, REQ-AH-014, REQ-AH-017) have no acceptance obligation in this SPEC.
