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

Proposed package boundary:

```go
type Host string

const (
    HostClaude   Host = "claude"
    HostCodex    Host = "codex"
    HostOpenCode Host = "opencode"
)

type LaunchRequest struct {
    Host        Host
    ProjectRoot string
    Role        string
    Agent       string
    Model       string
    Prompt      string
    Interactive bool
    Session     string
    Sandbox     string
    Approval    string
    DryRun      bool
}

type LaunchCommand struct {
    Host Host
    Cwd  string
    Env  map[string]string
    Argv []string
    Notes []string
}
```

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

OpenCode plugin templates handle event forwarding. OpenCode agent templates handle role prompts.

## 4. Template Strategy

Canonical skill source:

```text
.agents/skills/
```

Host mirrors:

```text
.claude/skills/        copy or symlink
.codex/skills/         copy or symlink when enabled
.opencode/agent/       generated agent wrappers
.opencode/plugins/     event adapter plugin
```

The default policy should be conservative. If symlink behavior is unreliable on a platform, use copy mode and expose a clear diagnostic.

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
