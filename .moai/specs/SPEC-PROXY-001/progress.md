# progress.md — SPEC-PROXY-001

> 단계별 진행 기록. §E.1은 plan-phase, §E.2/§E.3은 run-phase, §E.4는 sync-phase 소유.

## §E.1 Plan-phase Audit-Ready Signal

- plan_complete_at: 2026-08-19
- plan_updated_at: 2026-08-20
- plan_status: audit-ready
- tier: L
- spec_version: 0.2.0
- artifacts: research.md, design.md, spec.md, plan.md, acceptance.md, spec-compact.md

**미해소 항목: 7건 → 4건** (2026-08-20 갱신).

해소된 3건 (`plan.md` §B.1):
1. **데몬 범위** — 사용자 결정으로 머신(사용자) 단위 확정. 설정 계층도 함께 머신 단위로 이동 (design.md §3.1 / §3.4). **M1 차단 해제**
2. **Codex `auth.json` 필드 스키마** — 로컬 관찰로 확보 (research.md §2.3). 한 인스턴스 관찰이고 바이너리가 `opencodex`였다는 단서가 붙으며, 공식 CLI 검증은 M3 작업으로 등록
3. **Copilot 토큰 교환 절차와 추론 엔드포인트** — 범위 결정으로 해소. `copilot` 그룹을 v1 구현 범위에서 제외 (타입은 스키마 유지, spec.md §H)

남은 4건 (`plan.md` §B.2): Bedrock 필드 대조표(M2), LiteLLM 헬스체크 방법(M2), Codex 인증 엔드포인트(M3, `auth_mode: "chatgpt"` 관찰로 좁혀짐), Copilot 자격 증명 저장 위치(M3 이후/후속 SPEC).

**이 4건은 구현 착수를 막지 않는다.** 전부 해당 마일스톤 안에서 조사할 항목이며, M1은 어느 것에도 의존하지 않는다.

## §E.2 Run-phase Evidence

**cycle_type: tdd** (RED-GREEN-REFACTOR). Milestone M1 only ("설정 스키마와 카탈로그 자료구조", plan.md §D). M2/M3/M4 not started.

### M1 scope delivered

1. Machine registry schema (`groups` / `sets`) — `internal/proxy/registry.go` (`Registry`, `Group`, `AliasBlock`, `GroupType`).
2. Machine registry loader — `internal/proxy/registry_loader.go` (`LoadRegistry`, `LoadDefaultRegistry`, `DefaultRegistryPath`). File absence → empty registry, nil error (not a failure state).
3. Project pointer field `llm.proxy.default_set` — `internal/config/types.go` (`LLMConfig.Proxy LLMProxyConfig`) + `internal/template/templates/.moai/config/sections/llm.yaml` + `make build`.
4. Flag-less resolution order — `internal/proxy/resolve.go` (`ResolveDefaultSet`): project pointer → machine `sets.default` → error listing available sets. A project pointer to a missing set is a hard error (REQ-PROXY-025), never a silent fallback.
5. Catalog resolver returns `[]Deployment` from the first commit — `internal/proxy/catalog.go` (`Catalog.ResolveAlias`).
6. v1 routing reads index 0 as a single line — `SelectRoute(deployments []Deployment)`.
7. `<group>/<model>` direct resolution — `Catalog.ResolveDirect` / `Catalog.Resolve`.
8. Group type enum carries all 5 types including `copilot`; `EvaluateGroupStatus` isolates any group whose type lacks a v1 adapter (currently only `copilot`) as inactive with a `"type %q not supported in v1"` reason, leaving every other group active.

### RED evidence (verbatim, captured before implementation)

```
$ go test ./internal/proxy/...
# github.com/modu-ai/moai-adk/internal/proxy [github.com/modu-ai/moai-adk/internal/proxy.test]
internal/proxy/catalog_test.go:7:22: undefined: Registry
internal/proxy/catalog_test.go:8:10: undefined: Registry
internal/proxy/catalog_test.go:9:22: undefined: Group
internal/proxy/catalog_test.go:11:11: undefined: GroupTypeBedrock
internal/proxy/catalog_test.go:12:14: undefined: AliasBlock
internal/proxy/catalog_test.go:18:11: undefined: GroupTypeOpenAICompatible
internal/proxy/catalog_test.go:19:14: undefined: AliasBlock
internal/proxy/catalog_test.go:24:11: undefined: GroupTypeCopilot
internal/proxy/catalog_test.go:41:9: undefined: NewCatalog
internal/proxy/resolve_test.go:5:26: undefined: Registry
internal/proxy/catalog_test.go:41:9: too many errors
FAIL	github.com/modu-ai/moai-adk/internal/proxy [build failed]
FAIL
```

```
$ go test ./internal/config/... -run TestLLMConfig_ProxyDefaultSet -v
internal/config/types_test.go:254:3: unknown field Proxy in struct literal of type LLMConfig
internal/config/types_test.go:254:10: undefined: LLMProxyConfig
internal/config/types_test.go:256:9: cfg.Proxy undefined (type LLMConfig has no field or method Proxy)
internal/config/types_test.go:257:53: cfg.Proxy undefined (type LLMConfig has no field or method Proxy)
FAIL	github.com/modu-ai/moai-adk/internal/config [build failed]
```

### GREEN evidence (verbatim, this run, this tree — HEAD after implementation)

```
$ go test ./internal/proxy/... -v -cover
PASS
coverage: 91.7% of statements
ok  	github.com/modu-ai/moai-adk/internal/proxy	0.482s	coverage: 91.7% of statements
```
25/25 test functions PASS, 0 FAIL (`go test ./internal/proxy/... -v | grep -c '^--- PASS'` → 25).

```
$ go test ./internal/config/... -run "TestLLMConfig_ProxyDefaultSet|TestLLMProxyConfig_CarriesNoCredentialFields" -v
--- PASS: TestLLMConfig_ProxyDefaultSet (0.00s)
--- PASS: TestLLMProxyConfig_CarriesNoCredentialFields (0.00s)
PASS
ok  	github.com/modu-ai/moai-adk/internal/config	0.391s
```

```
$ go test ./internal/config/... -cover
ok  	github.com/modu-ai/moai-adk/internal/config	0.629s	coverage: 80.2% of statements
ok  	github.com/modu-ai/moai-adk/internal/config/toolpolicy	0.709s	coverage: 91.1% of statements
```
(80.2% is the whole `internal/config` package's pre-existing baseline aggregate — the new `LLMProxyConfig` addition is a plain struct plus one covered test; no regression attributable to this change. `internal/proxy` is the new package subject to the 85% threshold and is at 91.7%.)

### Build / vet / cross-platform

```
$ go build ./...
(exit 0, no output)
$ go vet ./...
(exit 0, no output)
$ GOOS=windows GOARCH=amd64 go build ./...
(exit 0, no output)
$ make build
... catalog.yaml updated successfully (10325 bytes)
go build -ldflags ... -o bin/moai ./cmd/moai
(exit 0)
```

### Template neutrality

```
$ grep -n "PROXY-001\|SPEC-PROXY" internal/template/templates/.moai/config/sections/llm.yaml
(no output — 0 matches, exit 1)
$ go test ./internal/template/... -run "Neutrality" -v
PASS
ok  	github.com/modu-ai/moai-adk/internal/template	0.651s
```

### Subagent boundary grep (C-HRA-008 family)

```
$ grep -rn "AskUserQuestion" internal/proxy/
(no output — 0 matches, exit 1)
```

### AC PASS/FAIL matrix — M1-scoped AC IDs only

| AC ID | REQ | Status | Verification | Actual Output |
|---|---|---|---|---|
| AC-PROXY-003 | REQ-PROXY-005, 006 | PASS | `go test -run TestLoadRegistry_SameTypeGroupsCoexist ./internal/proxy/...` | PASS — two `bedrock` groups (`bedrock-us`, `bedrock-eu`) load as 2 distinct entries with differing `Region` |
| AC-PROXY-004 | REQ-PROXY-007 | PASS | `go test -run TestResolveAlias_ReturnsOrderedDeploymentList\|TestAliasBlock_AllFourAliasesResolve ./internal/proxy/...` | PASS — alias `opus` on `gpu-cluster-a` resolves to `{gpu-cluster-a, qwen-3-72b}`; all 4 aliases resolve |
| AC-PROXY-006 | REQ-PROXY-012 | PASS | `go test -run TestCatalogName_GroupModelFormat ./internal/proxy/...` | PASS — `CatalogName("bedrock-us", "anthropic.claude-opus-5")` == `"bedrock-us/anthropic.claude-opus-5"` |
| AC-PROXY-007a | REQ-PROXY-013 | PASS | `go test -run TestResolveAlias_ReturnsOrderedDeploymentList\|TestResolveAlias_SingleDeploymentIsStillASlice ./internal/proxy/...` | PASS — return type is `[]Deployment` at length 2 AND at length 1 |
| AC-PROXY-007b | REQ-PROXY-014 | PASS | `go test -run TestSelectRoute_ReadsFirstDeployment ./internal/proxy/...` | PASS — 100/100 calls to `SelectRoute` return the first deployment, 0 to the second |
| AC-PROXY-008 | REQ-PROXY-015 | PASS | `go test -run TestResolveDirect_BypassesAliasResolution ./internal/proxy/...` | PASS — `Resolve("gpu-cluster-a/custom-model-not-in-aliases")` reaches `gpu-cluster-a` without consulting `Aliases` |
| AC-PROXY-016 | REQ-PROXY-024 | PASS | `go test -run TestGroup_SchemaCarriesNoCredentialFields\|TestLLMProxyConfig_CarriesNoCredentialFields ./internal/proxy/... ./internal/config/...` | PASS — reflection-based scan of `Group` and `LLMProxyConfig` yaml tags finds no `token`/`api_key`/`secret`/`password`/`credential`-shaped field; schema structurally admits only references (region, profile name, base_url, group/set names) |

### Not in M1 scope (remain untested / unimplemented, by design)

AC-PROXY-001a/b/c, 002, 005a-005f, 009, 010 (M2); AC-PROXY-011, 012a, 012b, 013, 014 (M3); AC-PROXY-015a/b (M2). plan.md §B.2's 4 carried-forward clarification items are untouched — M1 depends on none of them (plan.md §D M1: "선행 차단 없음").

## §E.3 Run-phase Audit-Ready Signal

- run_milestone: M1 of M1-M4 (plan.md §D)
- run_status: milestone-complete — awaiting orchestrator review before M2 (semi-autonomous progression, per spawn instruction)
- m1_commit_strategy: single milestone commit — `50542c7ff` on branch `worktree-agent-a1ebd35fa5ba3d08b` (this agent's isolated worktree; NOT pushed to `origin/main` — Tier L routes through `manager-git`/PR per SPEC Phase Discipline Route B, out of this delegation's scope)
- ac_pass_count (M1-scoped): 7 (AC-PROXY-003, 004, 006, 007a, 007b, 008, 016)
- ac_fail_count (M1-scoped): 0
- new_files: internal/proxy/registry.go, internal/proxy/registry_loader.go, internal/proxy/catalog.go, internal/proxy/resolve.go, internal/proxy/registry_test.go, internal/proxy/catalog_test.go, internal/proxy/resolve_test.go
- modified_files: internal/config/types.go, internal/config/types_test.go, internal/template/templates/.moai/config/sections/llm.yaml, .moai/specs/SPEC-PROXY-001/spec.md (frontmatter `status: draft` → `in-progress`), .moai/specs/SPEC-PROXY-001/progress.md (this file)
- new_warnings_or_lints_introduced: unknown — `golangci-lint` is not installed in this environment (`golangci-lint run` → `command not found`); `go vet ./...` is clean (exit 0). This is a residual-risk gap, not a claimed pass.
- cross_platform_build.linux_darwin: PASS (native `go build ./...`, exit 0)
- cross_platform_build.windows: PASS (`GOOS=windows GOARCH=amd64 go build ./...`, exit 0)
- total_run_phase_files: 11 (7 new + 4 modified)

### Gaps (explicitly not observed)

- `golangci-lint run` could not be executed — binary absent from PATH in this environment. `go vet` is the only static-analysis signal captured.
- No PR/branch created; per SPEC Phase Discipline this SPEC is Tier L (Route B / PR route), which is `manager-git`'s scope, not this delegation's. Commit is local to the current worktree branch; push/PR creation is deferred to the orchestrator's downstream routing.
- The whole-package `internal/config` coverage figure (80.2%) is a pre-existing baseline, not newly measured against a clean prior state — reported as observed-this-run, not as evidence this SPEC improved or regressed it.
- A pre-existing, unrelated test failure was observed during a broader regression sweep: `TestRunHookEvent_ReadInputError` in `internal/cli/coverage_test.go` (nil-pointer panic on a hook stdin-read-error path). `git status --short` confirms this SPEC's changes touch only `internal/config/types.go`, `internal/config/types_test.go`, `internal/template/templates/.moai/config/sections/llm.yaml`, and the new `internal/proxy/` package — none overlap `internal/cli/coverage_test.go` or the hook-input-reading code path it exercises. Reported as a known baseline defect, out of M1 scope, not fixed here.

### Residual risk

- The registry loader's `os.UserHomeDir()` failure branch and a handful of `LoadRegistry` I/O-error branches (permission-denied, not-a-directory) are not exercised by tests — these are OS-failure edge cases with low practical likelihood on the supported platforms, contributing to the 91.7% (not 100%) coverage figure.
- `ResolveAlias`'s "group in active set but absent from registry" skip-branch is exercised only indirectly (via `TestResolveAlias_NoActiveGroupDeclaresAlias`'s all-groups-absent-of-alias path, not a genuinely-unregistered-group path) — low risk, since `ResolveDefaultSet`'s hard-error-on-missing-pointer already prevents most such states, but M2's `-g`/`--set` wiring should add a direct test when it lands.

## §E.4 Sync-phase Audit-Ready Signal

_<pending sync-phase>_
