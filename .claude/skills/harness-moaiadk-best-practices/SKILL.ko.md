---
name: harness-moaiadk-best-practices
description: >
  4개 harness specialist(cli-template-specialist, quality-specialist,
  workflow-specialist, hook-ci-specialist)를 위한 moai-adk-go 베스트 프랙티스
  레퍼런스입니다. TRUST 5 게이트, Go 테스트 격리(t.TempDir, 병렬 테스트에서
  OTEL env 금지), 하드코딩 방지 규칙(envkeys.go의 env 상수, defaults.go의
  threshold), AskUserQuestion orchestrator-only 경계, deferred-tool preload 규칙,
  archived-agent rejection 계약, verification-claim 무결성을 다룹니다.
  moai-adk-go 코드를 작성하거나 리뷰할 때 specialist가 로드합니다.
allowed-tools: Read, Grep, Glob, Bash
user-invocable: false
metadata:
  version: "1.0.0"
  category: "harness/best-practices"
  status: "active"
  updated: "2026-06-17"
  tags: "moai-adk-go,best-practices,trust5,testing,hardcoding"
progressive_disclosure:
  level_1_tokens: 120
  level_2_tokens: 4000
  level_3_optional: true
triggers:
  agents:
    - cli-template-specialist
    - quality-specialist
    - workflow-specialist
    - hook-ci-specialist
  keywords: TRUST 5, t.TempDir, envkeys.go, defaults.go, AskUserQuestion, archived-agent, verification-claim, deferred tool
paths: "internal/**/*.go,**/*_test.go,.claude/rules/**"
---

# moai-adk-go 베스트 프랙티스

## TRUST 5 품질 게이트

모든 변경은 완료 전에 다섯 차원을 모두 통과해야 합니다.

| Pillar | Gate | Failure action |
|--------|------|----------------|
| **Tested** | coverage 포함 `go test ./...` | merge 차단, 누락 테스트 생성 |
| **Readable** | `golangci-lint run` | 경고, 리팩터링 제안 |
| **Unified** | `go fmt` + `goimports` | 자동 포맷 또는 경고 |
| **Secured** | OWASP 정렬 리뷰(per-spawn opus agent) | 차단, 리뷰 요구 |
| **Trackable** | Conventional Commits regex | 형식 제안 |

coverage 목표: 패키지 최소 85%, 중요 패키지(`internal/cli`, `internal/template`,
`internal/hook`)는 90% 이상.

## 테스트 격리

- 임시 디렉터리는 항상 `t.TempDir()`를 사용합니다. 자동 정리되며 `os.TempDir()` 아래에 만들어집니다.
- macOS 경로 함정: `t.TempDir()`는 `/var/folders/...`를 반환합니다. Go의
  `filepath.Join(cwd, absPath)`는 선행 `/`를 제거하지 않습니다.
  `filepath.Join("/a/b", "/var/folders/x")`는 잘못된 `"/a/b/var/folders/x"`가 됩니다.
  CLI 명령에서 사용자 제공 경로를 해석할 때는 `filepath.Abs()`를 사용합니다.
- 병렬 테스트에서 OTEL env를 설정하지 않습니다(`CLAUDE.local.md` §WARN). 병렬 테스트 안에서
  `t.Setenv("OTEL_EXPORTER_*", ...)`를 쓰지 마십시오. OTEL SDK는 env var에서 전역 상태를
  최초 사용 시 초기화하므로 data race가 발생할 수 있습니다. fake/no-op exporter를 쓰거나
  부모 테스트를 non-parallel로 둡니다.
- GLM 통합 테스트에서 `t.Setenv("HOME", tmpDir)`를 쓰지 않습니다. 병렬 테스트 오염을
  피하려면 `t.TempDir()`와 명시적 경로 구성을 사용합니다.
- 테스트를 하나라도 고친 뒤에는 cascading failure를 잡기 위해 전체 suite(`go test ./...`)를
  실행합니다. flaky 디버깅에는 `-count=1`, 동시성 안전성 확인에는 `-race`를 사용합니다.

## 하드코딩 방지

- URL, 모델명, 조직명, API header는 `const`로 추출합니다.
- 환경 변수 이름은 `internal/config/envkeys.go`에 상수로 정의하고 모든 곳에서 해당 상수를
  참조합니다. raw env string을 inline하지 않습니다.
- threshold는 `internal/config/defaults.go`를 단일 출처로 둡니다. 패키지 간 threshold를
  중복하지 않습니다.
- cross-platform 경로는 `$HOME`, `HOMEBREW_PREFIX` 등을 우선합니다. `.sh.tmpl` fallback
  경로에서는 `.HomeDir`가 아니라 `$HOME`을 사용합니다. `.HomeDir`는 `moai init` 시점에
  고정되어 비표준 layout 사용자에게 깨질 수 있습니다.
- 하드코딩은 `CLAUDE.local.md`, `settings.local.json`, 그리고 `t.TempDir()` 내부의
  `_test.go` 파일에서만 허용됩니다.

## AskUserQuestion 경계(orchestrator-only)

- `AskUserQuestion`은 유일한 사용자-facing 질문 채널이며 MoAI orchestrator(main session)에
  예약되어 있습니다.
- Subagent, including these harness specialists,는 `AskUserQuestion`을 호출하면 안 됩니다.
  사용자 입력이 필요하면 `.claude/rules/moai/core/askuser-protocol.md`의 Blocker Report
  Format에 맞춰 orchestrator에 구조화된 blocker report를 반환합니다.
- Deferred-tool preload: `AskUserQuestion`, `TaskCreate`, `TaskUpdate`, `TaskList`, `TaskGet`은
  deferred tool입니다. session start 시 schema가 로드되지 않습니다. orchestrator는 첫 사용 전에
  `ToolSearch(query: "select:AskUserQuestion,TaskCreate,...")`를 호출해야 합니다.
  subagent도 이 제약을 상속합니다.
- 응답 본문에 free-form prose 질문을 쓰는 것은 금지됩니다. 질문은 항상
  AskUserQuestion(orchestrator) 또는 blocker report(subagent)로 라우팅합니다.

## Archived-Agent Rejection 계약

12개 agent는 ARCHIVED이며 생성되는 harness 파일 어디에서도 참조하면 안 됩니다. `delegates-to`,
prose, 예시 모두 금지입니다. archived 이름의 전체 목록은 canonical SSOT인
`.claude/rules/moai/workflow/archived-agent-rejection.md` §B에 있습니다. 이 skill은 이름을
반복하지 않습니다. 모든 생성 파일에 그 token을 다시 심는 일을 피하기 위해서입니다.

유효한 delegation target은 다음 8개 RETAINED agent뿐입니다.

```
manager-spec, manager-develop, manager-docs, manager-git,
plan-auditor, sync-auditor, builder-harness, Explore (Anthropic built-in)
```

기존 archived domain-expert agent가 제공하던 domain expertise가 필요하면 delegation 시점에
per-spawn 패턴을 사용합니다.
`Agent(subagent_type: "general-purpose", model: "opus", tools: "<whitelist>", prompt: "...<domain> specialist: <conventions>...")`
전체 migration table(rows #1-#12)은
`.claude/rules/moai/workflow/archived-agent-rejection.md` §C를 봅니다.

## Verification-Claim 무결성

`.claude/rules/moai/core/verification-claim-integrity.md` 기준:

- 관찰하지 않은 claim 금지. "tests pass", "coverage 87%", "lint clean" 같은 주장은 actor가
  명령을 실행하고 출력을 관찰했을 때만 유효합니다. 실행하지 않은 명령은 pass가 아니라 gap입니다.
- 관찰하지 않은 defect claim 금지. frontmatter text나 grep match만으로 defect/debt/drift를
  추론하는 것은 hypothesis입니다. `moai spec audit`, `go test -cover`, `golangci-lint` 같은
  domain 전용 도구 없이 verified defect라고 부르지 않습니다. 2026-06-17 incident
  (29개 SPEC이 "Mx-close debt"로 오탐됐고 `moai spec audit` 결과 모두 grandfather-protected였던 사례)가
  canonical worked example입니다.
- Baseline attribution: 모든 verification claim은 이번 tree에서 이번 run에 실행한 command와
  관찰한 verbatim output을 명시합니다. 다른 SPEC/package/time의 숫자는 baseline이 아니라 carry-over입니다.
- 5-section report format: Claim / Evidence / Baseline-attribution / Gaps / Residual-risk.
  Gaps section이 방어선입니다. 관찰하지 않은 것을 반드시 열거합니다.

## Cross-References

- CLAUDE.local.md §6(testing), §14(hardcoding), §19(AskUserQuestion)
- `.claude/rules/moai/core/verification-claim-integrity.md` v1.1.0
- `.claude/rules/moai/core/askuser-protocol.md`
- `.claude/rules/moai/workflow/archived-agent-rejection.md`
- `.claude/rules/moai/development/coding-standards.md`
