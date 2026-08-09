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

# Acceptance Criteria: SPEC-V3R6-HOST-COMPAT-001

Every criterion below names the `REQ-AH-xxx` requirement it verifies. Criteria are binary: each Then-clause names the concrete artifact, path, flag, or identifier being asserted, so a tester can decide PASS/FAIL without interpretation.

Three requirements carry no acceptance obligation because they are removed from scope — REQ-AH-010, REQ-AH-014, REQ-AH-017. See `spec.md` §2.2. AC-AH-013 is retired in place and its identifier is never reused.

## AC-AH-001: Backward-Compatible Default

Verifies: REQ-AH-003, REQ-AH-016.

Given an existing project with no `workflow.default_host` and no role `host` fields,
When MoAI loads workflow configuration,
Then all role host resolution behaves as it did before this SPEC, and no error or warning is emitted for the absent fields.

Verification:

```bash
go test ./internal/config ./internal/cli -run 'Workflow|Role|Team'
```

## AC-AH-002: Role Host Resolution

Verifies: REQ-AH-003.

Given `workflow.team.role_profiles.implementer.host: codex`,
When the role host resolver evaluates `implementer`,
Then it returns `codex`.

Given `workflow.default_host: opencode` and no role-specific host,
When the role host resolver evaluates `reviewer`,
Then it returns `opencode`.

Given neither a role host nor `workflow.default_host` is set,
When the resolver evaluates any role,
Then it falls through the documented precedence chain and returns `claude`.

## AC-AH-003: Unknown Host Rejection

Verifies: REQ-AH-001, REQ-AH-003.

Given a workflow config contains `host: unknown-agent`,
When config validation runs,
Then the command fails with a message listing `claude`, `codex`, and `opencode`.

## AC-AH-004: Codex Command Builder

Verifies: REQ-AH-005, REQ-AH-006.

Given a launch request for host `codex` that establishes **all seven** REQ-AH-005 builder inputs — project root, prompt or workflow command, role profile, model, sandbox/approval mode, session or attach mode, and the dry-run flag,
When the command builder runs in dry-run mode,
Then the returned argv begins with `codex` and every one of the following REQ-AH-006 surfaces is asserted individually, each **with its value** where the flag takes one:

| # | Surface | Assertion |
|---|---------|-----------|
| 1 | `codex` interactive launch | argv[0] == `codex`, and no `exec` subcommand when the request is interactive |
| 2 | `codex exec` | argv[1] == `exec` when the request is non-interactive |
| 3 | `--cd` | the pair `--cd <project-root>` appears, value non-empty |
| 4 | `--model` | the pair `--model <model>` appears, value non-empty |
| 5 | `--profile` | the pair `--profile <profile>` appears, value non-empty |
| 6 | `--sandbox` | the pair `--sandbox <sandbox>` appears, value non-empty |
| 7 | `--ask-for-approval` | the pair `--ask-for-approval <approval>` appears, value non-empty |
| 8 | `--config` | the pair `--config <config>` appears, value non-empty |

A bare flag emitted without its value FAILS this criterion. A flag omitted when its corresponding request field is empty PASSES — the assertion binds the configured case.

The builder must not be able to satisfy this criterion by executing the `codex` binary; the binary is not installed on the development machine, so verification is argv-calculation only.

## AC-AH-005: OpenCode Command Builder

Verifies: REQ-AH-005, REQ-AH-007.

Given a launch request for host `opencode` that establishes **all seven** REQ-AH-005 builder inputs — project root, prompt or workflow command, role profile, model, sandbox/approval mode, session or attach mode, and the dry-run flag,
When the command builder runs in dry-run mode,
Then the returned argv begins with `opencode` and every one of the following REQ-AH-007 surfaces is asserted individually, each **with its value** where the flag takes one:

| # | Surface | Arity | Assertion |
|---|---------|-------|-----------|
| 1 | `opencode run` | subcommand | argv[1] == `run` for non-interactive execution |
| 2 | `--dir` | string | the pair `--dir <project-root>` appears, value non-empty |
| 3 | `--agent` | string | the pair `--agent <agent-or-role>` appears, value non-empty |
| 4 | `--model` | string | the pair `--model <model>` appears, value non-empty |
| 5 | `--session` | string | the pair `--session <session>` appears, value non-empty |
| 6 | `--continue` | boolean | the bare flag `--continue` appears with no following value |
| 7 | `--attach` | **string** | the pair `--attach <url>` appears, value non-empty — see AC-AH-017 |
| 8 | `--auto` | boolean | the bare flag `--auto` appears with no following value |

A value-taking flag emitted bare FAILS this criterion. A boolean flag emitted with a value also FAILS. Flag arity is asserted, not just flag presence.

Evidence source for the arity column: `opencode run --help` on the installed binary, which types `--attach` as `[string]` with help text "attach to a running opencode server (e.g., http://localhost:4096)".

## AC-AH-006: No Generic Shell Adapter for OpenCode

Verifies: REQ-AH-007.

Given OpenCode support is enabled,
When tests inspect launch configuration,
Then OpenCode is represented by `HostOpenCode` and a dedicated OpenCode command builder, not by a free-form shell command string.

## AC-AH-007: Native Launcher Dry Runs and Missing-Binary Failure

Verifies: REQ-AH-008.

Given the user runs `moai codex --dry-run`,
When Codex host support is enabled,
Then the CLI prints the calculated Codex command without launching a session.

Given the user runs `moai opencode --dry-run`,
When OpenCode host support is enabled,
Then the CLI prints the calculated OpenCode command without launching a session.

Given the resolved host binary is absent from `PATH`,
When the user runs the launcher **without** `--dry-run`,
Then the command fails **before any session mutation** — that is, the binary lookup failure occurs before the process is spawned or the session is replaced — and the error names the missing binary and directs the user to install and authenticate it.

## AC-AH-008: Tmux Role Delegation

Verifies: REQ-AH-009.

Given role profiles map architect to Claude, implementer to Codex, and reviewer to OpenCode,
When tmux launch planning runs,
Then each pane receives the command builder output for its resolved host.

Given a project configured for the existing `cc` / `glm` / `cg` paths,
When tmux launch planning runs,
Then those paths are selected exactly as before this SPEC.

## AC-AH-009: Codex Template Validity

Verifies: REQ-AH-011.

Given templates are rendered for Codex,
When template tests run,
Then both of the following hold:

1. `.codex/hooks.json`, rendered from `internal/template/templates/.codex/hooks.json.tmpl`, is valid JSON and contains the 10 core host-neutral hook events.
2. The shared instruction file `AGENTS.md`, rendered from `internal/template/templates/AGENTS.md.tmpl`, is produced at the **project root** and contains no unresolved template variables.

Given the rendered template set is enumerated,
When it is compared against the Codex file list,
Then **no** `.codex/config.toml` is produced — its absence is asserted, not merely unmentioned (see `spec.md` §2.2).

## AC-AH-010: OpenCode Template Validity

Verifies: REQ-AH-012.

Given templates are rendered for OpenCode,
When template tests run,
Then all of the following hold, each asserted against its exact path:

1. `opencode.json` is produced at the **project root** — not at `.opencode/opencode.json`. A file rendered to `.opencode/opencode.json` FAILS this criterion.
2. The rendered `opencode.json` is valid JSON and declares `"$schema": "https://opencode.ai/config.json"`.
3. The rendered `opencode.json` carries an `instructions` entry referencing `AGENTS.md`.
4. `.opencode/agents/moai-reviewer.md` is produced — under the **plural** `agents` directory. A file rendered to `.opencode/agent/` (singular) FAILS this criterion.
5. `.opencode/plugins/moai-hooks.js` is produced under the plural `plugins` directory.
6. Every file in 1-5 renders with no unresolved template variables.
7. The rendered plugin contains the literal `MOAI_HOOK_HOST=opencode`, so that forwarded events are attributed to the OpenCode host.

## AC-AH-011: Hook Truthfulness

Verifies: REQ-AH-013.

Given `moai host matrix opencode --json`,
When the matrix includes an adapter or fallback event,
Then the support level is not `native`, a degradation note is present and non-empty, and a source URL or local-evidence field is present and non-empty.

Given `moai host matrix codex --json`,
When the mapping for a project-scoped `.codex/hooks.json` event is inspected,
Then it is not reported as unconditionally `native`, and its degradation note states that project-local hooks load only when the project `.codex/` layer is trusted.

A mapping whose support level is non-native but whose degradation note or source field is empty FAILS this criterion. The source field is the anti-overclaiming mechanism and is asserted independently of the note.

## AC-AH-012: Feature Matrix Coverage

Verifies: REQ-AH-002.

Given `moai host matrix --features --json`,
When the output is decoded and the feature-group identifiers are collected into a set,
Then that set is **equal** to the following 12 identifiers — the canonical set defined in `spec.md` REQ-AH-002. Neither a missing identifier nor an extra one passes:

```
launcher/runtime
slash_workflows
spec_lifecycle
role_profiles
tmux_delegation
hooks
quality_gates
harness_lifecycle
state_session
project_config
git_pr
skills_templates
```

The comparison is verbatim, including the `/` separator in `launcher/runtime` (the only identifier not using `_`).

`spec.md` REQ-AH-002 is the single source of truth for this set. An earlier revision of this criterion anchored to `research.md` §1, which then enumerated 9 prose bullets rather than 12 identifiers; `research.md` §1 has since been amended to match REQ-AH-002 row-for-row, but REQ-AH-002 remains the anchor.

## AC-AH-013: Shared Skill Policy — [RETIRED]

**This criterion is retired.** Its requirement (REQ-AH-010, shared-skill installation from `.agents/skills`) is removed from the scope of this SPEC and deferred to a follow-up SPEC. See `spec.md` §2.2 → "Out of Scope — Shared-skill canonicalisation".

The identifier `AC-AH-013` is reserved and MUST NOT be reused for a different criterion. New criteria continue from AC-AH-017.

## AC-AH-014: Dev-Only Harness Exclusion

Verifies: REQ-AH-015.

Given the embedded template tree is walked by the existing static guard `TestSplitHarnessNamespaceNoLeak` in `internal/template/split_namespace_test.go` (sentinel `SPLIT_HARNESS_NAMESPACE_LEAK`),
When that guard is extended to walk the new `.codex/` and `.opencode/` template trees in addition to `.claude/`,
Then no dev-only harness artifact is found in any of the three trees, and the guard fails if one appears.

The guard is **unconditional**: it consults no flag, environment variable, or configuration. An implementation that makes dev-only distribution conditional on a flag FAILS this criterion, because it would contradict the standing guard whenever the flag were set.

This criterion **extends** the existing guard rather than adding a parallel one — a second, separately-maintained guard FAILS this criterion.

## AC-AH-015: Documentation Honesty and Recommended Role Split

Verifies: REQ-AH-004, REQ-AH-013.

Given generated docs describe host support,
When they mention OpenCode or Codex,
Then they distinguish native, adapter, fallback, and unsupported cases.

Given generated docs describe multi-host configuration,
When the role-host configuration is presented,
Then the Claude-plans / Codex-implements / OpenCode-reviews split of REQ-AH-004 is documented explicitly as the **recommended starting point**, not merely shown as one example among several.

Given generated docs describe what MoAI provides for Codex and OpenCode,
When the four scope exclusions of `spec.md` §2.2 are relevant,
Then the docs state that shared-skill canonicalisation, a Codex `config.toml`, a host diagnostic command, and per-workflow host classification are not provided by this release.

## AC-AH-016: Full Regression Gate

Verifies: all in-scope requirements (REQ-AH-001..REQ-AH-009, REQ-AH-011..REQ-AH-013, REQ-AH-015, REQ-AH-016, REQ-AH-018).

Given all in-scope implementation phases are complete,
When the verification suite runs,
Then these commands pass:

```bash
go test ./internal/agenthost ./internal/cli ./internal/template ./internal/tmux
go test ./...
```

And the cross-platform build succeeds:

```bash
GOOS=windows GOARCH=amd64 go build ./...
```

## AC-AH-017: `--attach` Is String-Valued

Verifies: REQ-AH-007.

Given a launch request for host `opencode` whose attach target is the URL `http://localhost:4096`,
When the OpenCode command builder produces argv,
Then argv contains the adjacent pair `--attach`, `http://localhost:4096`.

Given the same request,
When argv is inspected,
Then a bare `--attach` token with no following value is **absent**. Emitting `--attach` as a boolean switch FAILS this criterion.

Rationale and evidence: `opencode run --help` types `--attach` as `[string]` ("attach to a running opencode server (e.g., http://localhost:4096)"). At the time this criterion was written, `buildOpenCodeCommand` in `internal/agenthost/command.go` appended the bare flag from a `bool` field, and `hostLaunchFlags.Attach` in `internal/cli/host_launch.go` declared it `bool` — so the builder could emit a malformed command. This criterion is the correction obligation; it FAILS against the pre-correction implementation and PASSES after the field becomes string-valued.

## AC-AH-018: Security and Side-Effect Boundaries

Verifies: REQ-AH-018.

Given any host launcher runs with `--dry-run`,
When the printed output is inspected,
Then it contains the calculated argv only, and does not print the process environment or any credential, token, or API-key value.

Given any host launcher runs,
When the filesystem effects are inspected,
Then no credential file is written and no user-global host configuration (for example `~/.codex/config.toml` or `~/.config/opencode/`) is created or mutated. User-global setup remains opt-in through an explicit command.

Given `moai init` or `moai update` installs host templates,
When the written paths are inspected,
Then every rendered artifact lands under the project root. A write outside the project root FAILS this criterion.
