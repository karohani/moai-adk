---
name: harness-moaiadk-patterns
description: >
  4개 harness specialist(cli-template-specialist, quality-specialist,
  workflow-specialist, hook-ci-specialist)를 위한 moai-adk-go domain-patterns
  레퍼런스입니다. CLI/template/config/hook/spec subsystem architecture, 주요
  source path, Pipeline specialist delegation map, Template-First build cycle,
  namespace separation contract, add-a-template / add-a-hook / add-an-agent /
  add-a-SPEC 워크플로우를 다룹니다. moai-adk-go 자체 Go 코드베이스와 템플릿을
  작업할 때 specialist가 로드합니다.
allowed-tools: Read, Grep, Glob, Bash
user-invocable: false
metadata:
  version: "1.0.0"
  category: "harness/domain-patterns"
  status: "active"
  updated: "2026-06-17"
  tags: "moai-adk-go,cli,template,harness,patterns"
progressive_disclosure:
  level_1_tokens: 120
  level_2_tokens: 4500
  level_3_optional: true
triggers:
  agents:
    - cli-template-specialist
    - quality-specialist
    - workflow-specialist
    - hook-ci-specialist
  keywords: moai-adk-go, internal/cli, internal/template, embed.go, make build, go:embed, template-first, harness namespace
paths: "internal/**/*.go,internal/template/templates/**,.claude/**,.moai/**"
---

# moai-adk-go Domain Patterns

## Architecture Quick Reference

moai-adk-go는 `moai`라는 Go binary이며 네 개 subsystem으로 구성됩니다.

1. **CLI** (`internal/cli/*.go`, `cmd/moai/`) — Cobra commands: `init`,
   `update`, `hook`, `build`, `glm`, `cc`, `cg`, `version`, `doctor`, `spec`.
   subcommand handler는 hook용 stdin JSON을 읽고 orchestrator용 structured output을 냅니다.
2. **Template system** (`internal/template/`) — `go:embed` 기반 scaffolding입니다.
   source는 `internal/template/templates/`에 있고, `internal/template/embed.go`의
   `//go:embed all:templates`를 통해 binary에 embed됩니다. generated `.go` file은 없습니다.
   `make build`가 binary를 다시 컴파일합니다. `TemplateContext`
   (`{{.GoBinPath}}` / `{{.HomeDir}}`)는 `moai init` 시점에 렌더링됩니다.
3. **Config** (`internal/config/`) — `defaults.go`는 threshold SSOT,
   `envkeys.go`는 env-var constants, `TemplateContext` renderer를 담당합니다.
4. **Hook + CI** (`.claude/hooks/moai/*.sh`, `.github/workflows/`) — bash wrapper hook이
   `moai hook <event>`를 호출합니다. CI guard는 template neutrality를 강제합니다.

프로젝트 자체 개발을 관리하는 **SPEC lifecycle** (`.moai/specs/`)도 함께 있습니다
(plan→run→sync→Mx).

## 주요 Source Paths

| Subsystem | Path | Notes |
|-----------|------|-------|
| Cobra commands | `internal/cli/*.go` | `cmd/moai/`에서 wiring |
| Template source | `internal/template/templates/**` | 먼저 여기서 수정 |
| Embedded assets | `internal/template/embed.go` | `//go:embed all:templates`, generated file 없음 |
| Config defaults | `internal/config/defaults.go` | threshold SSOT |
| Env constants | `internal/config/envkeys.go` | env name 하드코딩 금지 |
| SPEC docs | `.moai/specs/SPEC-*/` | spec/plan/acceptance/progress |
| Era classifier | `internal/spec/era.go` | `ClassifyEra()` H-1..H-6 |
| Hook scripts | `.claude/hooks/moai/*.sh` | bash only, Python 금지 |
| CI workflows | `.github/workflows/*.yaml` | neutrality guard active |
| Harness agents | `.claude/agents/harness/*.md` | USER-OWNED(this skill) |

## Pipeline Specialist Delegation Map

이 harness는 4-stage pipeline입니다. 각 specialist는 retained agent에 delegate합니다.
archived agent는 쓰지 않으며 retained agent를 대체하지 않습니다.

```
CLI/Template ──→ quality ──→ workflow ──→ hook/CI
  │                │            │            │
  ├─ manager-develop (tdd, backend)
  ├─ Explore (read-only)
  ├─ sync-auditor (4-dim scoring)
  ├─ sync-phase-quality-gate.sh (Stop hook)
  ├─ manager-spec (plan)
  ├─ manager-develop (run)
  ├─ manager-docs (sync)
  ├─ plan-auditor (audit)
  ├─ builder-harness (artifact_type=hook|command|plugin)
  └─ Agent(general-purpose, model: opus, tools: ..., prompt: "...CI specialist...")
```

## Template-First Build Cycle

사용자 프로젝트로 배포되는 항목을 추가하거나 수정할 때:

1. 먼저 `internal/template/templates/<path>`를 수정합니다.
2. `make build`를 실행해 binary를 재컴파일합니다. template은 `embed.go`의
   `//go:embed all:templates`로 embed되며 generated `.go` file은 없습니다.
3. local에 sync합니다: `moai update` 또는 manual copy.
4. local `.claude/` / `.moai/`가 template을 반영하는지 확인합니다.
5. `go test ./internal/template/...`를 실행합니다(neutrality audit 포함).

template source 없이 `.claude/` 또는 `.moai/`를 직접 수정하지 마십시오. source of truth는
`templates/`입니다. 먼저 그곳을 수정하고 `make build`합니다.

## Namespace Separation Contract

`moai update`가 강제하는 두 namespace:

| Namespace | Location | Owner | `moai update` behavior |
|-----------|----------|-------|------------------------|
| Template-managed | `internal/template/templates/**` → `.claude/agents/{core,expert,meta}/`, `moai-*` skills | MoAI-ADK distribution | sync 시 local overwrite |
| User-owned(this harness) | `.claude/agents/harness/`, `harness-*` skills, `.moai/harness/` | Project developer | 절대 삭제/수정하지 않음, update 전 backup |

canonical user-owned skill prefix는 `harness-*`입니다(namespace catch-up 이후 Go enforcement가 인식,
SPEC-V3R6-HARNESS-NAMESPACE-V2-001). legacy `my-harness-*` 형식은 backward-compat
deprecation window 동안 유지됩니다. 신규 skill은 bare `harness-*` prefix를 사용해야 합니다.

## Common Workflows

### Template 추가

1. `internal/template/templates/<path>`에 파일을 생성합니다.
2. `make build`.
3. `moai update` 또는 `./moai init /tmp/test-project`로 테스트합니다.
4. `go test ./internal/template/... -run TestTemplateNeutralityAudit`.

### Hook 추가

1. `.claude/hooks/moai/handle-<event>.sh`를 작성합니다. bash, stdin JSON 읽기,
   `moai hook <event>` 호출.
2. `.claude/settings.json`에 `"$CLAUDE_PROJECT_DIR/..."` quoting과 `timeout: 5`로 연결합니다.
3. hook이 template-distributable이면 wrapper template source와 settings.json entry를
   `internal/template/templates/`에 추가합니다.

### Agent 추가(harness specialist)

1. `.claude/agents/harness/<role>-specialist.md`를 만들고 `name`,
   trigger-shaped `description`, `skills:` array(companion skill), `tools:` CSV string을 둡니다.
2. companion `harness-*` skill이 존재하는지 확인합니다. 없으면 self-activation smoke gate가 실패합니다.

### SPEC 추가

1. `/moai plan "<description>"` → `manager-spec`가 plan-phase artifacts 작성.
2. `plan-auditor` independent audit gate.
3. **Implementation Kickoff Approval** human gate(orchestrator가 `AskUserQuestion` 실행).
4. `/moai run SPEC-<ID>` → `manager-develop`(cycle_type은 quality.yaml 기준).
5. `/moai sync SPEC-<ID>` → `manager-docs`.
6. `sync-auditor` 4-dimension gate.
7. 단일 sync commit에서 3-phase close를 수행합니다. §E.4의 `sync_commit_sha`를 채우고,
   sync commit이 `implemented → completed` transition을 포함합니다. SPEC-V3R6-LIFECYCLE-REDESIGN-001에 따라
   예전 별도 `mx_commit_sha` / §E.5 Mx-phase step은 retired되었고, MX Tag validation은 sync sub-step입니다.

## Cross-References

- CLAUDE.local.md §2(Template-First Rule), §7(hooks), §21(dev-only commands)
- `.claude/rules/moai/development/agent-authoring.md` — agent frontmatter schema
- `.claude/rules/moai/development/skill-authoring.md` — skill frontmatter schema
- `.claude/rules/moai/workflow/archived-agent-rejection.md` §C — migration table
- `.claude/skills/moai-meta-harness/SKILL.md` § Namespace Separation
