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

# Acceptance Criteria: SPEC-V3R6-HOST-COMPAT-001

## AC-AH-001: Backward-Compatible Default

Given an existing project with no `workflow.default_host` and no role `host` fields,
When MoAI loads workflow configuration,
Then all role host resolution behaves as it did before this SPEC.

Verification:

```bash
go test ./internal/config ./internal/cli -run 'Workflow|Role|Team'
```

## AC-AH-002: Role Host Resolution

Given `workflow.team.role_profiles.implementer.host: codex`,
When the role host resolver evaluates `implementer`,
Then it returns `codex`.

Given `workflow.default_host: opencode` and no role-specific host,
When the role host resolver evaluates `reviewer`,
Then it returns `opencode`.

## AC-AH-003: Unknown Host Rejection

Given a workflow config contains `host: unknown-agent`,
When config validation runs,
Then the command fails with a message listing `claude`, `codex`, and `opencode`.

## AC-AH-004: Codex Command Builder

Given a launch request for host `codex` with model, profile, sandbox, and project root,
When the command builder runs in dry-run mode,
Then it returns a structured argv beginning with `codex` and containing the configured Codex flags.

## AC-AH-005: OpenCode Command Builder

Given a launch request for host `opencode` with agent, model, session, and project root,
When the command builder runs in dry-run mode,
Then it returns a structured argv beginning with `opencode` and containing OpenCode-native flags.

## AC-AH-006: No Generic Shell Adapter for OpenCode

Given OpenCode support is enabled,
When tests inspect launch configuration,
Then OpenCode is represented by `HostOpenCode` and `OpenCodeCommandBuilder`, not by a free-form shell command string.

## AC-AH-007: Native Launcher Dry Runs

Given the user runs `moai codex --dry-run`,
When Codex host support is enabled,
Then the CLI prints the calculated Codex command without launching a session.

Given the user runs `moai opencode --dry-run`,
When OpenCode host support is enabled,
Then the CLI prints the calculated OpenCode command without launching a session.

## AC-AH-008: Tmux Role Delegation

Given role profiles map architect to Claude, implementer to Codex, and reviewer to OpenCode,
When tmux launch planning runs,
Then each pane receives the command builder output for its resolved host.

## AC-AH-009: Codex Template Validity

Given templates are rendered for Codex,
When template tests run,
Then `.codex/hooks.json` is valid JSON and contains the supported core hook events.

## AC-AH-010: OpenCode Template Validity

Given templates are rendered for OpenCode,
When template tests run,
Then OpenCode config JSON is valid and plugin/agent templates render without missing variables.

## AC-AH-011: Hook Truthfulness

Given `moai host matrix opencode --json`,
When the matrix includes an adapter or fallback event,
Then the support level is not `native` and a degradation note is present.

## AC-AH-012: Feature Matrix Coverage

Given `moai host matrix --features --json`,
When the output is decoded,
Then it includes every feature group listed in `research.md` section 1.

## AC-AH-013: Shared Skill Policy

Given shared skill mode is `symlink`,
When templates install skills,
Then host skill paths are symlinks to `.agents/skills` where supported.

Given shared skill mode is `copy`,
When templates install skills,
Then host skill paths contain copied skill files and no symlink is required.

## AC-AH-014: Dev-Only Harness Exclusion

Given a user project runs `moai update`,
When templates are installed,
Then dev-only harness commands are not distributed unless an explicit dev flag is set.

## AC-AH-015: Documentation Honesty

Given generated docs describe host support,
When they mention OpenCode or Codex,
Then they distinguish native, adapter, fallback, and unsupported cases.

## AC-AH-016: Full Regression Gate

Given all implementation phases are complete,
When the verification suite runs,
Then these commands pass:

```bash
go test ./internal/agenthost ./internal/cli ./internal/template ./internal/tmux
go test ./...
```
