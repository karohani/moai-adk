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

# Design: SPEC-V3R6-HOST-COMPAT-001

## 1. Architecture

```text
.moai/config/sections/workflow.yaml
        |
        v
RoleHostResolver
        |
        v
agenthost.LaunchRequest
        |
        +--> ClaudeCommandBuilder
        +--> CodexCommandBuilder
        +--> OpenCodeCommandBuilder
        |
        v
CLI launcher / tmux pane / worktree session
```

## 2. Core Interfaces

Package boundary, reconciled against the landed implementation in `internal/agenthost/command.go` and `capability.go`:

```go
type Host string

const (
    HostClaude   Host = "claude"
    HostCodex    Host = "codex"
    HostOpenCode Host = "opencode"
)

type LaunchMode string

const (
    LaunchInteractive LaunchMode = "interactive"
    LaunchExec        LaunchMode = "exec"
)

type LaunchRequest struct {
    Host        Host
    ProjectRoot string
    Mode        LaunchMode
    Role        string
    Agent       string
    Model       string
    Prompt      string
    Profile     string
    Sandbox     string
    Approval    string
    Config      string
    Session     string
    // Attach carries the URL of a running OpenCode server. `opencode run --attach`
    // is a value-taking [string] flag, so this is modelled as a string, not a bool.
    Attach    string
    Continue  bool
    Auto      bool
    ExtraArgs []string
}

type LaunchCommand struct {
    Host  Host              `json:"host"`
    Cwd   string            `json:"cwd,omitempty"`
    Env   map[string]string `json:"env,omitempty"`
    Argv  []string          `json:"argv"`
    Notes []string          `json:"notes,omitempty"`
}
```

Three deltas from the earlier revision of this block are worth naming, because each one is load-bearing elsewhere in this document:

- `Attach string` (not `bool`) — required by §3's OpenCode flag-arity note and by AC-AH-017. This is the correction that landed in commit `2d01cdf95`.
- `Continue` / `Auto` / `Profile` / `Config` — the fields §3 lists as OpenCode and Codex builder inputs. The earlier block declared none of them, so §2 and §3 contradicted each other.
- `Mode LaunchMode` replaces the earlier `Interactive bool`, and `DryRun` is not a field of the request — dry-run is a CLI-level concern that decides whether the calculated `LaunchCommand` is printed or executed, not an input to argv calculation.

## 3. Host-Specific Builders

### Claude

The Claude builder preserves current behavior and wraps the existing launcher logic. This avoids rewriting `cc/glm/cg` before the Codex/OpenCode path is stable.

### Codex

The Codex builder owns Codex-specific argv generation. It should prefer:

- `codex` for interactive host sessions.
- `codex exec` for non-interactive workflow runs.
- project cwd through `--cd`.
- model/profile/sandbox/approval through official CLI flags.

### OpenCode

The OpenCode builder owns OpenCode-specific argv generation. It should prefer:

- `opencode run` for non-interactive execution.
- `--dir` for project root.
- `--agent` for role mapping.
- `--model`, `--session`, `--continue`, `--attach`, `--auto` where configured.

Flag arity matters here and is not uniform. `--continue` and `--auto` are boolean switches, but **`--attach` is string-valued**: `opencode run --help` types it `[string]` with the help text "attach to a running opencode server (e.g., http://localhost:4096)". A builder that models `--attach` as a boolean and emits the bare flag produces a malformed command. See `acceptance.md` AC-AH-017 for the correction obligation.

OpenCode plugin templates handle event forwarding — the plugin passes `MOAI_HOOK_HOST=opencode` to the MoAI hook CLI so downstream hook handling can attribute the event to the OpenCode host. OpenCode agent templates handle role prompts.

## 4. Template Strategy

### 4.1 Shared skill source — DEFERRED (not part of this SPEC)

An earlier revision of this section declared `.agents/skills/` the canonical cross-host skill source with `.claude/skills` / `.codex/skills` copy-or-symlink mirrors. **That strategy is deferred out of scope** (REQ-AH-010 removed; see `spec.md` §2.2 → "Out of Scope — Shared-skill canonicalisation"). No canonical source is designated, no mirror is generated, and no `copy` vs `symlink` policy is contracted by this SPEC.

Two facts make the deferral safe rather than merely postponed:

- Existing `.claude/skills` behavior is preserved exactly as-is — nothing about Claude skill distribution changes.
- OpenCode reads Claude Code skills natively from `.claude/skills` (opt-out `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS`; broader opt-outs `OPENCODE_DISABLE_CLAUDE_CODE`, `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT`). Source: https://opencode.ai/docs/ — so OpenCode receives MoAI skills without any mirroring work.

The open questions this section previously left uncontracted — the config key name and its YAML location, its enum values, its default, and the Windows symlink fallback behavior — are recorded in `spec.md` §2.3 Deferred Backlog → entry D-1, together with the trigger that should cause someone to pick the work up.

### 4.2 Host template surfaces in scope

```text
.codex/hooks.json      Codex hook adapter (existing template)
AGENTS.md              shared instruction file — read by BOTH Codex and OpenCode
opencode.json          OpenCode config, PROJECT ROOT (not .opencode/opencode.json)
.opencode/agents/      generated agent wrappers (canonical PLURAL directory name)
.opencode/plugins/     event adapter plugin
```

`AGENTS.md` is deliberately a single root file rather than a per-host pair: Codex reads `AGENTS.md` for project instructions, and OpenCode reads the same file (falling back to `CLAUDE.md` when it is absent) while also accepting it through the `instructions` config key. Duplicating it per host would create two sources of truth for one contract.

The `.opencode/` subdirectory names are **plural** per the OpenCode docs (`agents/`, `commands/`, `modes/`, `plugins/`, `skills/`, `tools/`, `themes/`); singular forms such as `agent/` exist only for backwards compatibility and are not used here.

## 5. Compatibility Contracts

The implementation must keep these contracts separate:

- Hook matrix: event-level support and degradation.
- Feature matrix: workflow/runtime/template-level support.
- Command builders: executable host commands.
- Template installer: files placed into a project.

This separation prevents a host from being marked as "fully supported" merely because hooks are partially mapped.

## 6. Data Flow Example

User config:

```yaml
workflow:
  default_host: claude
  team:
    role_profiles:
      architect:
        host: claude
      implementer:
        host: codex
      reviewer:
        host: opencode
```

Execution:

1. planner role resolves to Claude.
2. implementer role resolves to Codex.
3. reviewer role resolves to OpenCode.
4. tmux creates panes with argv from each host builder.
5. hook/plugin adapters report events with `MOAI_HOOK_HOST`.
6. `moai host matrix --features --json` reports exact support levels.
