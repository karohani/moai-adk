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

## Phase 0.95 Mode Selection

- Recommended implementation mode: staged solo execution with targeted subagents for review, or team mode only after file ownership is decomposed.
- Reason: changes cross CLI, config, templates, tmux, docs, and tests. Parallel work is useful after interfaces are frozen.

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
