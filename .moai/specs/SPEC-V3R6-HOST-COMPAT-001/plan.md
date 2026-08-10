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

# Implementation Plan: SPEC-V3R6-HOST-COMPAT-001

## A. Context

This SPEC turns the current Claude-first MoAI runtime into a host-aware runtime. The key product decision is that OpenCode must be native. That means OpenCode gets a first-class host enum, command builder, config/plugin templates, tests, and matrix entries. It is not modeled as `custom_command: opencode ...`.

## A.1 Decomposition Boundary

This SPEC is the umbrella compatibility spine. The implementation is sliced into delivery units:

- **Runtime slice** (Phases 0-4, 6) — host schema, command builders, `moai codex`, `moai opencode`, tmux role delegation, and the feature matrix. Covers REQ-AH-001..REQ-AH-009, REQ-AH-013, REQ-AH-016.
- **Extension slice** (Phase 5) — Codex templates and OpenCode config/plugin/agent templates. Covers REQ-AH-011, REQ-AH-012, REQ-AH-015.
- **Documentation slice** (Phase 7) — deferred to the sync phase. Covers REQ-AH-004 (documentation clause) and REQ-AH-015 honesty obligations.
- ~~Shared-skill slice: `.agents/skills` copy/symlink migration.~~ **REMOVED FROM SCOPE.** The ownership and update-semantics review this slice was gated on has not happened, and OpenCode reads `.claude/skills` natively so the mirroring is not load-bearing. REQ-AH-010 and AC-AH-013 are retired; see `spec.md` §2.2 → "Out of Scope — Shared-skill canonicalisation". `.agents/` is left untouched and `.gitignore` is not modified.

The runtime slice landed first. The remaining milestone is **Phase 5 plus the Phase 6 remainder**; Phase 7 is deferred to the sync phase.

## B. Phase Plan

> **Traceability convention.** Every phase below carries a `Covers:` line naming the `REQ-AH-xxx` requirements it implements and the `AC-AH-xxx` criteria that verify it. Phases 0-4 and 6 are landed (commit `361f8fd50`); Phase 5 is the open milestone; Phase 7 is deferred to the sync phase.

### Phase 0: Baseline Lock — LANDED

Covers: (no REQ — regression baseline for all phases). Verified by: AC-AH-016.

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

### Phase 1: Host Schema and Resolver — LANDED

Covers: REQ-AH-001 (host registry), REQ-AH-003 (YAML host precedence), REQ-AH-004 (role-split config pattern), REQ-AH-016 (backward-compatible migration). Verified by: AC-AH-001, AC-AH-002, AC-AH-003.

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

### Phase 2: Structured Command Builders — LANDED (correction applied)

Covers: REQ-AH-005 (structured argv builders), REQ-AH-006 (Codex builder), REQ-AH-007 (OpenCode builder). Verified by: AC-AH-004, AC-AH-005, AC-AH-006, AC-AH-017.

Correction applied (AC-AH-017), commit `2d01cdf95`: `buildOpenCodeCommand` in `internal/agenthost/command.go` previously modelled `--attach` as a boolean and emitted the bare flag with no value, and `hostLaunchFlags.Attach` in `internal/cli/host_launch.go` declared it `bool`. Because `opencode run --attach` takes a string URL value, that builder could emit a malformed command. `LaunchRequest.Attach` is now `string` and the builder emits the `--attach <url>` pair; the CLI flag is now `StringVar`. This correction landed with the Phase 5 milestone because AC-AH-005 and AC-AH-017 were re-tightened at the same time.

New or changed files — the implementation consolidated all three builders into one file rather than splitting them per host, so this list names what actually landed:

- `internal/agenthost/command.go` — all three host builders (Claude, Codex, OpenCode) plus `LaunchRequest` / `LaunchCommand` / `LaunchMode`
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

### Phase 3: Native CLI Launchers — LANDED

Covers: REQ-AH-008 (native launcher routing + missing-binary failure before session mutation), REQ-AH-018 (security and side-effect boundaries on the launch path). Verified by: AC-AH-007, AC-AH-018.

Files — both launchers were implemented in a single shared file rather than one file per host, so this list names what actually landed:

- `internal/cli/host_launch.go` — new; carries both the `moai codex` and `moai opencode` launchers and the shared `hostLaunchFlags`
- `internal/cli/host_launch_test.go` — new
- `internal/cli/launcher.go`
- `internal/cli/cc.go`
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

### Phase 4: Tmux and Worktree Role Delegation — LANDED

Covers: REQ-AH-009 (tmux role delegation through command builders, with `cc/glm/cg` backward compatibility). Verified by: AC-AH-008.

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

### Phase 5: Host Templates — OPEN (this is the milestone)

Covers: REQ-AH-011 (Codex templates), REQ-AH-012 (OpenCode templates and plugin), REQ-AH-015 (dev-only harness boundary), REQ-AH-018 (project-root write boundary). Verified by: AC-AH-009, AC-AH-010, AC-AH-014, AC-AH-018. Also delivers the AC-AH-017 `--attach` correction carried over from Phase 2.

#### Exact file list

There is no "as selected by implementation" latitude and no wildcard in this list. Phase 5 creates or modifies **exactly** these files:

| # | Path | Status | Purpose |
|---|------|--------|---------|
| 1 | `internal/template/templates/.codex/hooks.json.tmpl` | EXISTS — keep, do not rewrite | Codex hook adapter. Already at a correct discovery path (`<repo>/.codex/hooks.json`). |
| 2 | `internal/template/templates/AGENTS.md.tmpl` | NEW | Shared instruction file, rendered to project-root `AGENTS.md`. **One file serves both hosts** — Codex reads `AGENTS.md` for project instructions, and OpenCode reads the same file (falling back to `CLAUDE.md` when absent) and can also load it via the `instructions` config key. Do NOT create a second per-host instruction template. |
| 3 | `internal/template/templates/opencode.json.tmpl` | NEW | OpenCode config, rendered to **project-root** `opencode.json`. Declares `"$schema": "https://opencode.ai/config.json"` and an `instructions` entry pointing at `AGENTS.md`. |
| 4 | `internal/template/templates/.opencode/agents/moai-reviewer.md.tmpl` | NEW | OpenCode agent wrapper for the `reviewer` role. Directory name is the canonical **plural** `agents`. |
| 5 | `internal/template/templates/.opencode/plugins/moai-hooks.js.tmpl` | NEW | OpenCode event-adapter plugin. Forwards supported events to the MoAI hook CLI with `MOAI_HOOK_HOST=opencode`. |
| 6 | `internal/template/split_namespace_test.go` | MODIFY | Extend the existing unconditional guard to the new `.codex/` and `.opencode/` trees (see AC-AH-014). |
| 7 | template installer / render tests under `internal/template/` | NEW | Render-validity tests for files 1-5. |

Enumeration rules, so the list stays derivable rather than arbitrary:

- **Agent wrappers (file 4):** one wrapper per role that REQ-AH-004's recommended split assigns to `opencode`. That split assigns exactly one — `reviewer`. No `moai-plan` agent is created: the recommended split routes planning to Claude, so an OpenCode planning agent would contradict the SPEC's own configuration.
- **Plugin extension (file 5):** OpenCode plugins may be JavaScript or TypeScript. JavaScript is chosen so user projects need no build step to load the plugin.
- **No `config.toml`:** no `internal/template/templates/.codex/config.toml.tmpl` is created. See `spec.md` §2.2 → "Out of Scope — Codex `config.toml` project template".

#### `git check-ignore` verification of every named path

`.gitignore:125-129` ignores `.codex/`, `.agents/`, and `AGENTS.md`, then re-includes the template `.codex/` subtree via two negations. Because `git check-ignore`'s exit status is not a reliable ignore signal when the matching rule is a negation, each path was checked with `--no-index -v` and the matched pattern inspected — a leading `!` means the path is trackable. Result (verified 2026-08-09):

| Path | Matched pattern | Trackable? |
|------|-----------------|-----------|
| `internal/template/templates/.codex/hooks.json.tmpl` | `.gitignore:129:!internal/template/templates/.codex/**` | YES (negated) |
| `internal/template/templates/AGENTS.md.tmpl` | (no match) | YES |
| `internal/template/templates/opencode.json.tmpl` | (no match) | YES |
| `internal/template/templates/.opencode/agents/moai-reviewer.md.tmpl` | (no match) | YES |
| `internal/template/templates/.opencode/plugins/moai-hooks.js.tmpl` | (no match) | YES |

The `AGENTS.md` ignore rule matches the **basename** `AGENTS.md`, so a bare root `AGENTS.md` is ignored but the template source `AGENTS.md.tmpl` is not — this is why file 2 is named with the `.tmpl` suffix and is safe to commit. By contrast `internal/template/templates/.agents/skills/...` **is** ignored (`.gitignore:126:.agents/`, no negation exists for it), which independently confirms the shared-skill slice could not ship today even if it were in scope.

#### Tasks

- Create files 2-5 above. (REQ-AH-011, REQ-AH-012)
- Wire `opencode.json.tmpl`'s `instructions` key to the rendered `AGENTS.md`. (REQ-AH-012)
- Emit `MOAI_HOOK_HOST=opencode` from the plugin's event forwarding. (REQ-AH-012)
- Extend `TestSplitHarnessNamespaceNoLeak` to walk `.codex` and `.opencode` in addition to `.claude`. (REQ-AH-015)
- Correct `--attach` from boolean to string-valued in `internal/agenthost/command.go` and `internal/cli/host_launch.go`. (REQ-AH-007)
- Confirm all template writes land under the project root; no user-global path is written. (REQ-AH-018)
- Preserve existing `.claude/skills` behavior unchanged. No `.agents/` work — that slice is out of scope.

#### Tests

- rendered `.codex/hooks.json` is valid JSON and carries the 10 core host-neutral hook events, enumerated in `spec.md` REQ-AH-011 (AC-AH-009)
- rendered root `opencode.json` is valid JSON, carries the `$schema` URL, and the agent + plugin templates render with no unresolved variables (AC-AH-010)
- the plugin template contains the `MOAI_HOOK_HOST=opencode` forwarding literal (AC-AH-010)
- the extended namespace guard still passes and now covers `.codex/` and `.opencode/` (AC-AH-014)
- `--attach` renders as `--attach <value>` and a bare `--attach` is rejected (AC-AH-017)

> No acceptance criterion in this phase may require executing the `codex` binary — it is not installed on the development machine. All Codex verification is template-render and JSON-validity based. (`opencode` is installed, but tests still must not launch a real session per `spec.md` §2.2.)

### Phase 6: Host Matrix Expansion — LANDED (degradation-note remainder open)

Covers: REQ-AH-002 (12 feature groups), REQ-AH-013 (hook support truthfulness, including the Codex project-layer trust degradation note). Verified by: AC-AH-011, AC-AH-012.

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

- Codex reports native for core hooks supported by the current template contract, and the project-scoped `.codex/hooks.json` mapping carries the trust-conditional degradation note (AC-AH-011)
- OpenCode reports adapter/fallback where applicable, and every non-native mapping carries both a degradation note and a source URL or local evidence (AC-AH-011)
- Feature groups equal exactly the 12 groups defined in `spec.md` REQ-AH-002, compared as a set against the serialized identifiers (AC-AH-012)

> Corrected premise: an earlier revision of this task read "all 12 inventory groups **from the research doc**". That was false — `research.md` §1 previously enumerated 9 prose bullets, not 12. The authoritative source is `spec.md` REQ-AH-002 (12 groups with serialized identifiers); `research.md` §1 has since been amended to match it row-for-row.

### Phase 7: Documentation and Reports — DEFERRED to the sync phase

Covers: REQ-AH-004 (documenting the recommended role split as the starting point), REQ-AH-013 / REQ-AH-015 honesty obligations in user-facing docs. Verified by: AC-AH-015.

Files:

- `README.md`
- `README.ko.md`
- docs-site pages if present and in scope
- generated report under `reports/` if requested during sync

Tasks:

- Update product positioning from Claude-only to host-aware, while preserving Claude-first history. (REQ-AH-004)
- Document the recommended role split — Claude planning, Codex implementation, OpenCode reviewer/native alternative — explicitly as the recommended starting point. (REQ-AH-004, AC-AH-015)
- Document exact unsupported cases, distinguishing native / adapter / fallback / unsupported without overstating parity. (REQ-AH-013, AC-AH-015)
- Document the four scope exclusions recorded in `spec.md` §2.2 so users are not left expecting shared-skill canonicalisation, a Codex `config.toml`, a host diagnostic command, or per-workflow host classification. (REQ-AH-015)
- Add migration notes for existing projects. (REQ-AH-016)

## C. Risk Register

| Risk | Impact | Mitigation | Bound by |
|------|--------|------------|----------|
| Overclaiming OpenCode parity | user confusion | support levels must distinguish native, adapter, fallback, unsupported | REQ-AH-013 / AC-AH-011 |
| Overclaiming Codex hook parity — project `.codex/` hooks load only when the project layer is trusted | user sees a hook silently not fire | the mapping carries a trust-conditional degradation note; it is never reported as unconditionally native | REQ-AH-013 / AC-AH-011 |
| Breaking existing Claude workflows | high | preserve default host fallback to Claude and keep cc/glm/cg tests green | REQ-AH-016 / AC-AH-001 |
| Hook event mismatch | medium | feature matrix and hook matrix remain separate | REQ-AH-002 / AC-AH-012 |
| Shell quoting bugs | high | structured argv builders and tests | REQ-AH-005 / AC-AH-004, AC-AH-005 |
| Malformed argv from a value-taking flag emitted bare (`--attach`) | medium — command fails at the host | ACs assert flag **and value** together, not flag presence alone | REQ-AH-007 / AC-AH-005, AC-AH-017 |
| Dev-only harness leakage into the new `.codex/` + `.opencode/` trees | medium | extend the existing unconditional static guard rather than adding a second, flag-gated one | REQ-AH-015 / AC-AH-014 |
| Wrong OpenCode config path or singular directory name silently produces a non-loading artifact that still renders cleanly | high — passes render tests while being inert | path and directory names pinned from the live docs and asserted literally in the AC, not left to implementation choice | REQ-AH-012 / AC-AH-010 |
| ~~Symlink portability issues~~ | n/a | ~~config-controlled `copy` vs `symlink`~~ — risk withdrawn: the shared-skill slice is out of scope, so no symlink is created | REQ-AH-010 removed |

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
