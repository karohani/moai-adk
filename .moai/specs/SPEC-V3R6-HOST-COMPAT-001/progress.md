---
id: SPEC-V3R6-HOST-COMPAT-001
title: "Claude/Codex/OpenCode Compatibility Spine"
version: "0.1.0"
status: in-progress
created: 2026-07-05
updated: 2026-07-05
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

## Audit-Ready Signal

- plan_complete_at: pending
- plan_status: in-progress

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
