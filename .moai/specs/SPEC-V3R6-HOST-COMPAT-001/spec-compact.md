# SPEC-V3R6-HOST-COMPAT-001 Compact

## Goal

Make MoAI host-aware across Claude, Codex, and OpenCode so project YAML can assign planning, implementation, review, and support roles to different coding-agent hosts.

## Core Requirements

- REQ-AH-001: canonical host registry with `claude`, `codex`, `opencode`.
- REQ-AH-002: feature-surface compatibility matrix over exactly 12 groups — `launcher/runtime`, `slash_workflows`, `spec_lifecycle`, `role_profiles`, `tmux_delegation`, `hooks`, `quality_gates`, `harness_lifecycle`, `state_session`, `project_config`, `git_pr`, `skills_templates`.
- REQ-AH-003: project YAML host resolution with role host -> workflow default -> compatibility fallback -> Claude.
- REQ-AH-004: recommended planning-vs-coding split with Claude planning, Codex implementation, OpenCode native support.
- REQ-AH-005: structured argv command builders.
- REQ-AH-006: native Codex builder.
- REQ-AH-007: native OpenCode builder.
- REQ-AH-008: native `moai codex` and `moai opencode` launchers.
- REQ-AH-009: tmux role delegation through command builders.
- REQ-AH-010: ~~shared skill installation from `.agents/skills`~~ — **REMOVED FROM SCOPE** (deferred to a follow-up SPEC).
- REQ-AH-011: Codex templates — `.codex/hooks.json` plus the shared root `AGENTS.md`. No `config.toml`.
- REQ-AH-012: OpenCode templates and plugin — root `opencode.json`, `.opencode/agents/`, `.opencode/plugins/`.
- REQ-AH-013: truthful hook support levels, including the Codex project-layer trust degradation note.
- REQ-AH-014: ~~13 slash workflow coverage story~~ — **REMOVED FROM SCOPE** (feature-group-level coverage remains under REQ-AH-002).
- REQ-AH-015: user-distributed vs dev-only harness boundary (unconditional — no dev flag).
- REQ-AH-016: backward-compatible migration.
- REQ-AH-017: ~~host diagnostics~~ — **REMOVED FROM SCOPE**. The dry-run surface remains under REQ-AH-008.
- REQ-AH-018: security and side-effect boundaries.

The three removed requirements keep their headings in `spec.md` so REQ-AH-001..REQ-AH-018 numbering stays contiguous; each carries a `[REMOVED FROM SCOPE]` marker pointing at the §2.2 rationale.

## Files To Modify

- `internal/agenthost/*`
- `internal/cli/launcher.go`
- `internal/cli/root.go`
- `internal/cli/team_spawn.go`
- `internal/cli/worktree/tmux_integration.go`
- `internal/config/types.go`
- `internal/template/templates/.codex/hooks.json.tmpl` (existing)
- `internal/template/templates/AGENTS.md.tmpl` (new, shared by Codex + OpenCode)
- `internal/template/templates/opencode.json.tmpl` (new, project root)
- `internal/template/templates/.opencode/agents/moai-reviewer.md.tmpl` (new)
- `internal/template/templates/.opencode/plugins/moai-hooks.js.tmpl` (new)
- `internal/template/split_namespace_test.go` (extend the existing guard)
- `.moai/config/sections/workflow.yaml`
- README/docs

## Exclusions

- Do not rewrite the MoAI workflow engine.
- Do not claim full hook parity where host docs do not support it.
- Do not resurrect old CI review bot workflows.
- Do not mutate user-global host config except through explicit setup commands.
- Do not formalize `.agents/skills` or add a copy/symlink policy (REQ-AH-010 removed). OpenCode reads `.claude/skills` natively, so no mirroring is needed for host support.
- Do not ship a Codex `config.toml` template — that surface carries user-owned model/approval/provider settings.
- Do not add a host diagnostic subcommand (REQ-AH-017 removed); `moai host matrix` is the only host CLI surface.
- Do not classify the 13 `/moai` workflows individually (REQ-AH-014 removed).
