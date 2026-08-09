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

# Research: SPEC-V3R6-HOST-COMPAT-001

## 1. Current Feature Surface

The current project supports a broad Claude-first MoAI runtime. This section enumerates the feature surface as **exactly 12 groups**, one per row, in the same order and with the same serialized identifiers as `spec.md` REQ-AH-002 — which is the single source of truth for the set. Earlier revisions of this section carried 9 prose bullets, which collapsed `quality_gates` + `harness_lifecycle` into one entry and omitted `project_config` and `git_pr` entirely; that mismatch is corrected here so no downstream artifact anchors to a divergent count.

| # | Feature group (`identifier`) | Current Claude-first surface |
|---|------------------------------|------------------------------|
| 1 | launcher/runtime (`launcher/runtime`) | Launcher modes `moai cc`, `moai glm`, `moai cg`. |
| 2 | slash workflows (`slash_workflows`) | 13 `/moai` commands: `plan`, `run`, `sync`, `project`, `fix`, `loop`, `review`, `clean`, `codemaps`, `gate`, `mx`, `feedback`, `harness`. |
| 3 | SPEC lifecycle (`spec_lifecycle`) | plan/run/sync phases, GEARS requirements, DDD/TDD run methodology, audit gates. |
| 4 | role profiles (`role_profiles`) | Retained MoAI agents plus workflow role profiles for researcher, analyst, architect, implementer, tester, designer, reviewer. |
| 5 | worktree/tmux (`tmux_delegation`) | Worktree CRUD and tmux launch helpers. |
| 6 | hooks (`hooks`) | The internal hook model covers a larger Claude-style event surface, while the host compatibility matrix focuses on 10 core lifecycle/tool events. |
| 7 | quality gates (`quality_gates`) | gate, ast-grep, lsp doctor, tool-policy, constitution. |
| 8 | harness lifecycle (`harness_lifecycle`) | Harness lifecycle commands and learning proposals. Distinct from group 7 — the two were previously merged into a single "Quality and harness" bullet, but they are separate identifiers in the matrix. |
| 9 | state/session (`state_session`) | Session registry, state dump, inventory, telemetry, preference, research, HTML report generation. |
| 10 | config/project docs (`project_config`) | `.moai/config/sections/*.yaml` typed config loading and the `.moai/project/` product/structure/tech document set. Not represented in the earlier 9-bullet form. |
| 11 | Git/GitHub/PR (`git_pr`) | Branch and PR routing, `gh`-backed PR creation, and the tier-based PR strategy. Not represented in the earlier 9-bullet form. |
| 12 | skills/templates (`skills_templates`) | `.claude` templates are the primary distribution path. `.agents/skills` exists in this checkout as a shared-skill candidate but is **not** formalized by this SPEC — see `spec.md` §2.2. |

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
7. **Skill sharing gap** — *deferred, not addressed by this SPEC*: `.agents/skills` is present in this checkout but not formalized as the installed cross-host skill source with a copy/symlink policy. This gap is knowingly left open; see `spec.md` §2.2 → "Out of Scope — Shared-skill canonicalisation". Two facts make the deferral low-risk: OpenCode reads `.claude/skills` natively (§3), and `.agents/` is fully gitignored (`.gitignore:126`) with no negation for the template tree, so it cannot be distributed today in any case.
8. **Documentation gap**: user-facing docs still describe MoAI mainly as a Claude Code runtime.

## 3. Official Host References

Codex official docs confirm these implementation surfaces:

- Codex CLI exposes command-line options such as `--model`, `--profile`, `--sandbox`, `--remote`, and related run configuration.
  Source: https://developers.openai.com/codex/cli/reference
- Codex config is TOML-based with user and project config layers.
  Source: https://developers.openai.com/codex/config-basic
- Codex hooks include events such as `SessionStart` and support JSON hook output for additional context.
  Source: https://developers.openai.com/codex/hooks
- Codex discovers hooks at `~/.codex/hooks.json`, `~/.codex/config.toml`, `<repo>/.codex/hooks.json`, and `<repo>/.codex/config.toml`. The existing `internal/template/templates/.codex/hooks.json.tmpl` therefore targets a correct path.
  Source: https://developers.openai.com/codex/hooks
- **Project-local hooks load only when the project `.codex/` layer is trusted.** In untrusted projects Codex still loads user and system hooks from their own active config layers, but the project-local hooks do not load. This conditionality is why REQ-AH-013 forbids reporting a project `.codex/hooks.json` mapping as unconditionally `native`, and requires the condition to be surfaced as a degradation note.
  Source: https://developers.openai.com/codex/hooks

OpenCode official docs confirm these implementation surfaces:

- OpenCode CLI exposes `opencode run` and agent management commands.
  Source: https://opencode.ai/docs/cli/
- OpenCode config uses `opencode.json` with schema `https://opencode.ai/config.json`, and supports provider, permissions, LSP, MCP, plugins, and instructions.
  **The file lives at the PROJECT ROOT.** The docs state: "Add `opencode.json` in your project root. Project config has the highest precedence among standard config files." `opencode.jsonc` (JSON with comments) is also accepted. `.opencode/opencode.json` is **not** a config location — an earlier revision of this SPEC left the location as an unresolved "or", which is now pinned to root `opencode.json`.
  Source: https://opencode.ai/docs/config/
- The `instructions` config key takes an array of paths/globs, combined with `AGENTS.md`.
  Source: https://opencode.ai/docs/config/
- **`.opencode/` subdirectories use PLURAL names.** The docs state that `.opencode` and `~/.config/opencode` use plural subdirectory names — `agents/`, `commands/`, `modes/`, `plugins/`, `skills/`, `tools/`, `themes/` — with singular names (e.g. `agent/`) supported only for backwards compatibility. The canonical form is therefore `.opencode/agents/`, not `.opencode/agent/`.
  Source: https://opencode.ai/docs/agents/
- OpenCode agents support primary agents and subagents; built-ins include Build, Plan, General, Explore, Scout, Compaction, Title, and Summary. Project agents are markdown files in `.opencode/agents/`; the filename becomes the agent name, and frontmatter carries `description`, `mode` (`all`/`primary`/`subagent`), `model`, `temperature`, and `permission`.
  Source: https://opencode.ai/docs/agents/
- OpenCode plugins live in `.opencode/plugins/` (project) or `~/.config/opencode/plugins/` (global), may be JavaScript **or** TypeScript, are auto-loaded at startup, and can subscribe to events. The plural `plugins/` form recorded in the previous revision of this section was **verified correct** against the live docs during the 2026-08-09 plan-audit remediation; it needs no change.
  Source: https://opencode.ai/docs/plugins/
- **OpenCode natively reads Claude Code files.** Project rules fall back to `CLAUDE.md` when no `AGENTS.md` exists, and skills are read from `.claude/skills`. This is disableable via `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS` (also `OPENCODE_DISABLE_CLAUDE_CODE` and `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT`). This is the load-bearing reason the shared-skill mirroring slice is **not** required for OpenCode support and is deferred out of scope (`spec.md` §2.2).
  Source: https://opencode.ai/docs/
- `opencode run` flags, confirmed against the installed binary via `opencode run --help`: `--command`, `--continue`/`-c`, `--session`/`-s`, `--fork`, `--share`, `--model`/`-m`, `--agent`, `--file`/`-f`, `--format`, `--title`, `--attach`, `--password`/`-p`, `--username`/`-u`, `--dir`, `--port`, `--variant`, `--thinking`, `--auto`, `--interactive`/`-i`.
  **`--attach` is typed `[string]`** — its help text reads "attach to a running opencode server (e.g., http://localhost:4096)". It takes a URL **value**; it is not a boolean flag. See `acceptance.md` AC-AH-017 for the resulting correction obligation.
  Local evidence: `opencode run --help` (the `opencode` binary is installed on the development machine; the `codex` binary is not, so no Codex acceptance criterion may require executing `codex`).

## 4. Planning Conclusion

This should be a Tier L SPEC because it crosses config schema, CLI launcher, tmux orchestration, templates, hooks, docs, and tests. The implementation should be staged so that each stage is independently testable:

1. Define the host model and role-host schema.
2. Extract structured command builders.
3. Add native Codex and OpenCode launcher paths.
4. Extend tmux/worktree role delegation.
5. Add host-specific config/instruction templates. (The shared-skill promotion originally bundled into this stage is deferred out of scope — see `spec.md` §2.2.)
6. Expand host matrix from hook-only compatibility into feature-surface compatibility.
7. Add docs, migration guidance, and regression tests.
