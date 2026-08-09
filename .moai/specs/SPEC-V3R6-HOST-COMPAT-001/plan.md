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

# Implementation Plan: SPEC-V3R6-HOST-COMPAT-001

## A. Context

This SPEC turns the current Claude-first MoAI runtime into a host-aware runtime. The key product decision is that OpenCode must be native. That means OpenCode gets a first-class host enum, command builder, config/plugin templates, tests, and matrix entries. It is not modeled as `custom_command: opencode ...`.

## A.1 Decomposition Boundary

This SPEC is the umbrella compatibility spine. The implementation can still be sliced into smaller delivery branches:

- Runtime slice: host schema, command builders, `moai codex`, `moai opencode`, and tmux role delegation.
- Extension slice: Codex templates, OpenCode config/plugin/agent templates, and feature matrix expansion.
- Shared-skill slice: `.agents/skills` copy/symlink migration. This slice should remain optional until ownership and update semantics are reviewed.

The runtime slice should land first. OpenCode templates are required for native support, but they should not block the initial command-builder contract if review pressure is high.

## B. Phase Plan

### Phase 0: Baseline Lock

Goal: prevent regressions while host support is added.

Tasks:

- Record current CLI command surface from `internal/cli/root.go` and command registrations.
- Record current workflow feature surface from `.claude/commands/moai/*.md`.
- Record current hook matrix from `internal/agenthost/capability.go`.
- Run the smallest existing tests that cover templates and host matrix.

Suggested checks:

```bash
go test ./internal/agenthost ./internal/cli ./internal/template
```

### Phase 1: Host Schema and Resolver

Files:

- `internal/agenthost/capability.go`
- `internal/config/types.go`
- `internal/cli/team_spawn.go`
- `.moai/config/sections/workflow.yaml`
- `internal/template/templates/.moai/config/sections/workflow.yaml.tmpl`

Tasks:

- Add `Host` to workflow role profile structs.
- Add `workflow.default_host`.
- Implement role host resolver with precedence:
  role host -> workflow default host -> llm/team compatibility fallback -> claude.
- Reject unsupported hosts with clear errors.
- Keep existing configs backward-compatible.

Tests:

- role with explicit `host: codex`
- role with `host: opencode`
- missing host falls back to current behavior
- unknown host fails

### Phase 2: Structured Command Builders

New or changed files:

- `internal/agenthost/command.go`
- `internal/agenthost/command_claude.go`
- `internal/agenthost/command_codex.go`
- `internal/agenthost/command_opencode.go`
- `internal/agenthost/command_test.go`

Tasks:

- Define `LaunchRequest` and `LaunchCommand`.
- Return argv arrays, env overlays, cwd, and support notes.
- Encode Claude existing behavior without changing output.
- Encode Codex CLI command generation.
- Encode OpenCode CLI command generation.
- Add dry-run formatting.

Tests:

- no shell string concatenation required for normal launches
- Codex model/profile/sandbox flags render correctly
- OpenCode agent/model/session flags render correctly
- missing required fields fail before execution

### Phase 3: Native CLI Launchers

Files:

- `internal/cli/launcher.go`
- `internal/cli/cc.go`
- new `internal/cli/codex.go`
- new `internal/cli/opencode.go`
- `internal/cli/root.go`

Tasks:

- Add `moai codex`.
- Add `moai opencode`.
- Route both through command builders.
- Add `--dry-run` for command calculation.
- Keep `moai cc`, `moai glm`, and `moai cg` behavior unchanged.
- Add binary detection and actionable missing-binary errors.

Tests:

- `moai codex --dry-run` prints Codex command
- `moai opencode --dry-run` prints OpenCode command
- existing launcher tests remain green

### Phase 4: Tmux and Worktree Role Delegation

Files:

- `internal/tmux/session.go`
- `internal/cli/worktree/tmux_integration.go`
- `internal/cli/team_spawn.go`
- related tests under `internal/cli/worktree` and `internal/tmux`

Tasks:

- Replace hardcoded `moai cc/glm` selection with role-host command builder where role context exists.
- Preserve current cc/glm/cg path for backward compatibility.
- Add role-aware pane naming.
- Add JSON/dry-run path for launch plans.

Tests:

- architect -> Claude command
- implementer -> Codex command
- reviewer -> OpenCode command
- CG compatibility still launches GLM where configured

### Phase 5: Templates and Shared Skills

Files:

- `internal/template/templates/.codex/hooks.json.tmpl`
- new Codex config/instruction templates as selected by implementation
- new `internal/template/templates/opencode.json.tmpl` or `.opencode/opencode.json.tmpl`
- new `internal/template/templates/.opencode/agent/*.md.tmpl`
- new `internal/template/templates/.opencode/plugins/moai.*.tmpl`
- template installer tests

Tasks:

- Formalize `.agents/skills` as canonical shared skill source.
- Add `copy` vs `symlink` policy.
- Ensure Claude existing `.claude/skills` behavior is preserved.
- Install Codex hooks/config/instructions.
- Install OpenCode config/agents/plugin/instructions.
- Mark dev-only harness commands as not distributed.

Tests:

- rendered Codex hooks JSON is valid
- rendered OpenCode JSON is valid
- symlink/copy policy produces expected paths
- update preserves user-owned harness artifacts

### Phase 6: Host Matrix Expansion

Files:

- `internal/agenthost/capability.go`
- `internal/cli/host.go`
- `internal/cli/host_test.go`

Tasks:

- Keep existing hook matrix.
- Add feature-surface matrix.
- Add `moai host matrix --features`.
- Add JSON schema stable enough for docs and tests.
- Add source URLs and degradation notes.

Tests:

- Codex reports native for core hooks supported by current template contract
- OpenCode reports adapter/fallback where applicable
- Feature groups include all 12 inventory groups from the research doc

### Phase 7: Documentation and Reports

Files:

- `README.md`
- `README.ko.md`
- docs-site pages if present and in scope
- generated report under `reports/` if requested during sync

Tasks:

- Update product positioning from Claude-only to host-aware, while preserving Claude-first history.
- Document recommended role split: Claude planning, Codex implementation, OpenCode reviewer/native alternative.
- Document exact unsupported cases.
- Add migration notes for existing projects.

## C. Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| Overclaiming OpenCode parity | user confusion | support levels must distinguish native, adapter, fallback, unsupported |
| Breaking existing Claude workflows | high | preserve default host fallback to Claude and keep cc/glm/cg tests green |
| Symlink portability issues | medium | config-controlled `copy` vs `symlink`; default can remain copy if safer |
| Hook event mismatch | medium | feature matrix and hook matrix remain separate |
| Shell quoting bugs | high | structured argv builders and tests |
| Dev-only harness leakage | medium | template installer tests must assert dev-only commands are excluded |

## D. Verification Plan

Minimum local verification:

```bash
go test ./internal/agenthost ./internal/cli ./internal/template ./internal/tmux
```

Broader verification after implementation:

```bash
go test ./...
go test ./internal/template -run 'Codex|OpenCode|Settings|Template'
go test ./internal/agenthost
go test ./internal/cli -run 'Host|Launch|Codex|OpenCode|Team'
```

No real Codex/OpenCode session launch is required for unit acceptance. Real session smoke tests should be manual or gated behind an opt-in integration flag.
