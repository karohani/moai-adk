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

# Research: SPEC-V3R6-HOST-COMPAT-001

## 1. Current Feature Surface

The current project supports a broad Claude-first MoAI runtime:

- Launcher modes: `moai cc`, `moai glm`, `moai cg`.
- Slash workflows: 13 `/moai` commands: `plan`, `run`, `sync`, `project`, `fix`, `loop`, `review`, `clean`, `codemaps`, `gate`, `mx`, `feedback`, `harness`.
- SPEC lifecycle: plan/run/sync, GEARS requirements, DDD/TDD run methodology, audit gates.
- Agents and role profiles: retained MoAI agents plus workflow role profiles for researcher, analyst, architect, implementer, tester, designer, reviewer.
- Worktree/tmux: worktree CRUD and tmux launch helpers.
- Hooks: internal hook model covers a larger Claude-style event surface, while the host compatibility matrix currently focuses on 10 core lifecycle/tool events.
- Quality and harness: gate, ast-grep, lsp doctor, tool-policy, constitution, harness lifecycle, learning proposals.
- State and reporting: session registry, state dump, inventory, telemetry, preference, research, HTML report generation.
- Templates and skills: `.claude` templates are the primary distribution path; `.agents/skills` exists as a shared skill candidate.

Local evidence:

- CLI root and launcher: `internal/cli/root.go`, `internal/cli/launcher.go`
- Workflow role profiles: `.moai/config/sections/workflow.yaml`, `internal/cli/team_spawn.go`
- Config structs: `internal/config/types.go`
- Tmux worktree integration: `internal/cli/worktree/tmux_integration.go`, `internal/tmux/session.go`
- Hook event model: `internal/hook/types.go`, `internal/hook/coverage_table.go`
- Host matrix: `internal/agenthost/capability.go`, `internal/cli/host.go`
- Codex hook template: `internal/template/templates/.codex/hooks.json.tmpl`
- Template tests: `internal/template/settings_test.go`

## 2. Current Gaps

The repository has a useful compatibility foundation but does not yet provide full native multi-host execution:

1. **Launcher gap**: the launcher path resolves `cc`, `glm`, and `cg`, then launches Claude. There is no native `moai codex` or `moai opencode` path.
2. **Role host gap**: workflow role profiles contain `mode`, `model`, `isolation`, and `description`; they do not declare which coding-agent host owns a role.
3. **Command builder gap**: there is no structured argv builder for Claude, Codex, and OpenCode. Host-specific launch behavior is embedded in launcher code.
4. **Tmux delegation gap**: worktree tmux integration chooses `moai cc` or `moai glm`; it cannot launch a planner in Claude and implementer in Codex/OpenCode.
5. **Template gap**: `.codex/hooks.json.tmpl` exists, but full Codex config/rules/skills/subagent templates are not equivalent to Claude. OpenCode config/plugin/agent templates are absent.
6. **Hook scope gap**: `agenthost` covers 10 host-neutral events; Claude hook coverage is larger. The product needs a clear contract for core parity vs degraded host-specific behavior.
7. **Skill sharing gap**: `.agents/skills` is present in this checkout but not yet formalized as the installed cross-host skill source with copy/symlink policy.
8. **Documentation gap**: user-facing docs still describe MoAI mainly as a Claude Code runtime.

## 3. Official Host References

Codex official docs confirm these implementation surfaces:

- Codex CLI exposes command-line options such as `--model`, `--profile`, `--sandbox`, `--remote`, and related run configuration.
  Source: https://developers.openai.com/codex/cli/reference
- Codex config is TOML-based with user and project config layers.
  Source: https://developers.openai.com/codex/config-basic
- Codex hooks include events such as `SessionStart` and support JSON hook output for additional context.
  Source: https://developers.openai.com/codex/hooks

OpenCode official docs confirm these implementation surfaces:

- OpenCode CLI exposes `opencode run` and agent management commands.
  Source: https://opencode.ai/docs/cli/
- OpenCode config uses `opencode.json` with schema `https://opencode.ai/config.json`, and supports provider, permissions, LSP, MCP, plugins, and instructions.
  Source: https://opencode.ai/docs/config/
- OpenCode agents support primary agents and subagents; built-ins include Build, Plan, General, Explore, Scout, Compaction, Title, and Summary.
  Source: https://opencode.ai/docs/agents/
- OpenCode plugins live in `.opencode/plugins/` or the user config plugin directory and can subscribe to events.
  Source: https://opencode.ai/docs/plugins/

## 4. Planning Conclusion

This should be a Tier L SPEC because it crosses config schema, CLI launcher, tmux orchestration, templates, hooks, docs, and tests. The implementation should be staged so that each stage is independently testable:

1. Define the host model and role-host schema.
2. Extract structured command builders.
3. Add native Codex and OpenCode launcher paths.
4. Extend tmux/worktree role delegation.
5. Promote shared skills/templates and host-specific config templates.
6. Expand host matrix from hook-only compatibility into feature-surface compatibility.
7. Add docs, migration guidance, and regression tests.
