# SPEC-V3R6-HOST-COMPAT-001 Compact

## Goal

Make MoAI host-aware across Claude, Codex, and OpenCode so project YAML can assign planning, implementation, review, and support roles to different coding-agent hosts.

## Core Requirements

- REQ-AH-001: canonical host registry with `claude`, `codex`, `opencode`.
- REQ-AH-002: feature-surface compatibility matrix for launcher, slash workflows, SPEC lifecycle, role profiles, tmux, hooks, quality, harness, state, config, GitHub/PR, skills/templates.
- REQ-AH-003: project YAML host resolution with role host -> workflow default -> compatibility fallback -> Claude.
- REQ-AH-004: recommended planning-vs-coding split with Claude planning, Codex implementation, OpenCode native support.
- REQ-AH-005: structured argv command builders.
- REQ-AH-006: native Codex builder.
- REQ-AH-007: native OpenCode builder.
- REQ-AH-008: native `moai codex` and `moai opencode` launchers.
- REQ-AH-009: tmux role delegation through command builders.
- REQ-AH-010: shared skill installation from `.agents/skills`.
- REQ-AH-011: Codex templates.
- REQ-AH-012: OpenCode templates and plugin.
- REQ-AH-013: truthful hook support levels.
- REQ-AH-014: 13 slash workflow coverage story.
- REQ-AH-015: user-distributed vs dev-only harness boundary.
- REQ-AH-016: backward-compatible migration.
- REQ-AH-017: host diagnostics and dry-run.
- REQ-AH-018: security and side-effect boundaries.

## Files To Modify

- `internal/agenthost/*`
- `internal/cli/launcher.go`
- `internal/cli/root.go`
- `internal/cli/team_spawn.go`
- `internal/cli/worktree/tmux_integration.go`
- `internal/config/types.go`
- `internal/template/templates/**`
- `.moai/config/sections/workflow.yaml`
- README/docs

## Exclusions

- Do not rewrite the MoAI workflow engine.
- Do not claim full hook parity where host docs do not support it.
- Do not resurrect old CI review bot workflows.
- Do not mutate user-global host config except through explicit setup commands.
