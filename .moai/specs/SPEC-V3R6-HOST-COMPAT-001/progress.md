---
id: SPEC-V3R6-HOST-COMPAT-001
title: "Claude/Codex/OpenCode Compatibility Spine"
version: "0.2.0"
status: in-progress
created: 2026-07-05
updated: 2026-08-09
author: manager-spec
priority: P1
phase: "v3.1.0 target"
module: "internal/agenthost, internal/cli, internal/config, internal/template, .moai/config/sections"
lifecycle: spec-anchored
tags: "agenthost, codex, opencode, claude, tmux, workflow, hooks, templates"
tier: L
---

# Progress: SPEC-V3R6-HOST-COMPAT-001

## 2026-07-05

- Created draft SPEC via `$moai plan` flow.
- Research is based on current repository feature inventory plus official Codex and OpenCode documentation.
- Status: draft, pending human review.

## §F Phase 0.95 Mode Selection

### Plan-phase note (2026-07-05)

- Recommended implementation mode: staged solo execution with targeted subagents for review, or team mode only after file ownership is decomposed.
- Reason: changes cross CLI, config, templates, tmux, docs, and tests. Parallel work is useful after interfaces are frozen.

### Run-phase decision (2026-08-10, Phase 5 + Phase 6 remainder milestone)

Input parameters (measured, not assumed):

| Parameter | Value | How established |
|-----------|-------|-----------------|
| tier | L | `spec.md` frontmatter `tier: L` |
| scope (file count) | ~12-15 | 7 enumerated Phase 5 template files + 3-5 installer/guard test edits + 2 Go files for the AC-AH-017 `--attach` correction |
| domain count | 2 | template tree (embed/deploy) + Go CLI/agenthost source. Docs are deferred to sync. |
| file language mix | Go + JSON/`.tmpl` + markdown | from the Phase 5 file table in `plan.md` |
| concurrency benefit | LOW | coding-heavy authoring, not research fan-out |
| harness level | `standard` | `harness.yaml` sets no pinned `level:`; `auto_detection` rules resolve to `standard` (file_count > 3 / spec_type feature). `thorough` requires security or payment keywords, critical priority, or a domain in [auth, payment, migration, public_api] — none hold. |
| `workflow.team.enabled` | `true` | `.moai/config/sections/workflow.yaml` |
| `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` | `1` | `.claude/settings.json:276`, confirmed in the live environment |

Mode evaluation:

| Mode | Selected | Rationale |
|------|----------|-----------|
| 1 trivial | no | New template authoring plus a typed-field code correction — semantic, not a typo-class change. |
| 2 background | no | The work requires Write/Edit; the background path is read-only by contract. |
| 3 agent-team | no | Capability gate fails on 1 of 3 conditions: harness level resolves to `standard`, not `thorough`. The other two conditions (`team.enabled: true`, env `=1`) do hold. |
| 4 parallel | no | Scope is coding-heavy rather than research-heavy; the Anthropic coding-task parallelism caveat routes this away from concurrent fan-out. |
| 5 sub-agent | **yes** | Default fallback and the correct fit: sequential `manager-develop` spawns, coding-heavy work, single implementation owner over one file tree. |
| 6 workflow | no | Two independent conditions fail: scope is ~12-15 files (below the ~30 entry threshold), and the work is not one uniform mechanical transform — authoring 7 heterogeneous template files and changing a struct field's type are different transformations with an inter-file dependency (the `--attach` fix constrains what the OpenCode agent template may assert). |

**Decision: sub-agent**

Justification: the remaining milestone is coding-heavy authoring concentrated in one file tree with a single owner, which is exactly the case Anthropic's coding-task parallelism caveat assigns to sequential sub-agent execution rather than concurrent fan-out. Agent Teams is additionally unavailable on capability grounds (harness `standard`). Mode 6 was evaluated and rejected on both the volume threshold and the uniform-transform requirement, so it is recorded here as considered-and-rejected rather than unexamined.

## §G IGGDA Kickoff Predicate

Evaluated at the Phase 1 → Phase 2 boundary on 2026-08-10.

| Condition | Value | Evidence |
|-----------|-------|----------|
| (a) intent clarity 100% | PASS | Three AskUserQuestion rounds completed: scope and uncommitted-state disposition; shared-skill / Codex-scope / FAIL-path decisions; kickoff approval with SHOULD-FIX timing and integration strategy. |
| (b) plan-auditor PASS | PASS | iter-2 verdict PASS, aggregate 0.873 vs Tier L threshold 0.85, monotonic +0.373 over iter-1. |
| (c) Tier S or M | **FAIL** | `tier: L`. Tier L is the complexity cutoff; the condition requires S or M. |
| (d) no dangerous keywords / destructive scope | **FAIL** | SPEC scope contains `session` (the OpenCode `--session` / session-mode surface) and REQ-AH-018 governs secrets and credential boundaries — both fall inside the security-domain keyword set. |

**Verdict: explicit-gate.**

Conditions (c) and (d) each independently force the explicit-gate branch, so the auto-proceed branch was never reachable. A mandatory blocking `AskUserQuestion` kickoff gate was issued and the user approved run-phase entry, selected deferred remediation for the two SHOULD-FIX defects, and chose PR-based integration at sync. No `/goal` autonomy was set for this milestone.

## §E.1 Plan-phase Audit-Ready Signal

- plan_status: audit-ready
- plan_complete_at: 2026-08-09

### Plan-audit remediation history

| Date | Event | Outcome |
|------|-------|---------|
| 2026-08-09 | plan-auditor iteration 1 | **FAIL**, overall 0.50 vs Tier L threshold 0.85 (Clarity 0.50 / Completeness 0.70 / Testability 0.50 / Traceability 0.40). All four must-pass criteria passed; the failure was threshold-driven. Report: `.moai/reports/plan-audit/SPEC-V3R6-HOST-COMPAT-001-2026-08-09.md`. |
| 2026-08-09 | manager-spec amendment (v0.2.0) | Artifacts amended across `spec.md`, `plan.md`, `acceptance.md`, `research.md`, `design.md`, `spec-compact.md`. Awaiting re-audit. |

Amendment summary (v0.1.0 → v0.2.0):

- Pinned the OpenCode config path to project-root `opencode.json` (was an unresolved "or").
- Reconciled the feature-group count at 12 across `spec.md` REQ-AH-002, `research.md` §1, and `acceptance.md` AC-AH-012, anchored to REQ-AH-002 as the single source of truth.
- Enumerated the exact Phase 5 template file list with `git check-ignore` trackability evidence for every named path.
- Corrected `.opencode/agent` to the canonical plural `.opencode/agents`; confirmed `.opencode/plugins` was already correct.
- Removed the AC-AH-014 dev-flag contradiction; the criterion now extends the standing unconditional guard `TestSplitHarnessNamespaceNoLeak`.
- Removed three requirements from scope with written rationale (REQ-AH-010 shared skills, REQ-AH-014 per-workflow classification, REQ-AH-017 diagnostic command), retained in place as `[REMOVED FROM SCOPE]` markers so REQ numbering stays contiguous.
- Added `REQ-AH-xxx` back-references to every acceptance criterion and `Covers:` lines to every plan phase.
- Added AC-AH-017 (`--attach` is string-valued) and AC-AH-018 (security and side-effect boundaries).

Note on `plan_status: audit-ready` — this records that the artifacts are ready for plan-auditor re-execution. It is not a claim that the re-audit has passed; no re-audit has been run against v0.2.0.

## Run Phase Log

### 2026-07-05 — Runtime Compatibility Slice

- status: PASS
- implemented:
  - `internal/agenthost` structured launch command builders for Claude, Codex, and OpenCode.
  - `moai codex` and `moai opencode` native dry-run/launch commands.
  - host compatibility feature matrix in addition to the existing hook event matrix.
  - `workflow.default_host` and `workflow.team.role_profiles.<role>.host` typed config loading.
  - team role-profile snapshots now persist resolved host per role.
  - workflow.yaml runtime/template defaults preserve existing Claude behavior with `host: claude`.
  - TUI schema bridge fallback for expanded schema fields.
  - stale hook coverage tests aligned with the current graceful malformed-stdin policy.
- validation:
  - `env GOCACHE=/private/tmp/moai-gocache go test ./internal/agenthost ./internal/cli ./internal/config -run 'BuildLaunchCommand|Host|Codex|OpenCode|LoadRoleProfiles|WorkflowConfig|TeamConfig|RoleProfile|FeatureMatrix|RootCmd'` PASS.
  - `env GOCACHE=/private/tmp/moai-gocache GOMODCACHE=/private/tmp/moai-modcache go test ./internal/cli -run 'I18nKeySetParity|BridgeFieldDefResolver|TUIRendersSchemaFieldSet|RunHookEvent_ReadInputError|RunAgentHook_ReadInputError'` PASS.
  - `GOCACHE=/private/tmp/moai-gocache GOMODCACHE=/private/tmp/moai-modcache go test ./...` PASS with external permission for module cache, home-scoped test fixtures, and local listener tests.
