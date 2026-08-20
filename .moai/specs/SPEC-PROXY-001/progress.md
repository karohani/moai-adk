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

## M2 subsection — 데몬 골격 + litellm + bedrock

**cycle_type: tdd** (RED-GREEN-REFACTOR). Milestone M2 ("데몬 골격 + litellm + bedrock", plan.md §D). Continues in the SAME worktree/session as M1, after orchestrator independent re-verification + user approval to proceed. M3/M4 not started.

### Gating investigations resolved this milestone

**Investigation A — LiteLLM per-instance `/v1/messages` activation detection (plan.md §B.2 gate item 6).** research.md §2.1 had already confirmed the endpoint EXISTS on LiteLLM's proxy server; the per-instance ACTIVATION question was deferred here. Method decided and implemented: LiteLLM's proxy returns HTTP 404 for any path with no registered handler, and only registers `/v1/messages` when the Anthropic-unified passthrough is enabled. A minimal probe POST distinguishes "404 → inactive" from "any other status (200/400/401/...) → active, route is registered and being handled" — implemented as `ProbeMessagesEndpoint` (`internal/proxy/litellm_health.go`), tested against both branches with an `httptest` backend (`internal/proxy/litellm_health_test.go`). This extends the same isolate-with-reason mechanism M1's `EvaluateGroupStatus` (REQ-PROXY-021) already established, rather than inventing a second one — full daemon-startup wiring (calling the probe per litellm-typed group at activation and folding the result into `GroupStatus`) is deferred to M4 CLI wiring; the probe function itself is complete and unit-tested.

**Investigation B — Bedrock Converse vs InvokeModel field-mapping table (design.md §5.4, plan.md §B.2 gate item 8).** Decision: **InvokeModel / InvokeModelWithResponseStream**, not Converse.

| Anthropic `/v1/messages` field | Bedrock InvokeModel body | Treatment |
|---|---|---|
| `model` | *(removed)* | Goes in the `ModelId` API parameter (URL path segment), not the body |
| `stream` | *(removed)* | Chosen by which SDK operation is invoked (`InvokeModel` vs `InvokeModelWithResponseStream`), not a body field |
| *(n/a)* | `anthropic_version` | **Added**, fixed literal `"bedrock-2023-05-31"` — required by Bedrock's Anthropic-on-Bedrock contract, no Anthropic-native equivalent |
| `messages` | `messages` | Passthrough, unchanged |
| `system` | `system` | Passthrough, unchanged |
| `max_tokens` | `max_tokens` | Passthrough, unchanged |
| `temperature` / `top_p` / `top_k` | same | Passthrough, unchanged |
| `stop_sequences` | `stop_sequences` | Passthrough, unchanged |
| `tools` / `tool_choice` | same | Passthrough, unchanged |
| *(response)* `content` / `usage` / `stop_reason` | same | **Identity** — Bedrock's InvokeModel response body for Anthropic-family models IS the Anthropic response shape verbatim |
| *(streaming)* | `PayloadPart.Bytes` per EventStream chunk | Each chunk's decoded bytes are themselves Anthropic-shaped SSE event JSON (`message_start`/`content_block_delta`/...); the adapter re-wraps each into a literal `data: <json>\n\n` SSE frame — the EventStream *binary framing* differs from raw SSE bytes even though the JSON payload inside matches |

Rationale: our client surface IS Anthropic `/v1/messages`, and Anthropic-family Bedrock models accept the Anthropic body almost verbatim via InvokeModel (the table above IS the whole translation). Converse would instead require a bidirectional field remapping (renamed content-block types, camelCase nesting under `inferenceConfig`) AND full streaming-event reconstruction, for a model-agnosticism benefit this v1 scope does not need — v1 `bedrock` groups route Anthropic-shaped model IDs only (a non-Anthropic Bedrock model routed through this proxy is out of v1 scope, consistent with the proxy's whole purpose of unifying backends behind one Anthropic-shaped client surface). Implemented in `internal/proxy/bedrock.go` (`ToBedrockInvokeModelBody`, `FromBedrockInvokeModelBody`, `InvokeModelViaBedrock`, `InvokeModelStreamViaBedrock`) against a narrow `BedrockInvoker` interface, with the real AWS SDK wiring kept thin in `internal/proxy/bedrock_aws.go` (`AWSBedrockInvoker`).

### M2 scope delivered

1. **Daemon lifecycle** (REQ-PROXY-001~004) — `internal/proxy/daemon.go`: `Daemon.Acquire`/`Release`, reference counting, self-termination when the count reaches zero. Runtime state (PID, address, refcount) at the machine-scope path `~/.moai/proxy/daemon.json` (`DefaultDaemonStateDir`) — NEVER under a project's `.moai/state/`. Advisory locking mirrors `internal/session`'s established per-package flock/`LockFileEx` pattern (`daemon_lock_unix.go` / `daemon_lock_windows.go`), reusing the SAME withLock/readState/writeStateAtomic shape as `internal/session/registry.go`.
2. **Loopback-only binding + unprivileged port** (REQ-PROXY-022/023) — `internal/proxy/server.go` `StartServer`: binds `127.0.0.1:0` (never `0.0.0.0`/`[::]`), OS-assigned ephemeral port.
3. **Anthropic `/v1/messages` client surface** (REQ-PROXY-016) — `internal/proxy/anthropic_handler.go` `MessagesHandler`: reads the `model` field, resolves via `Catalog.Resolve` (M1), dispatches to the resolved group's backend adapter; SSE relay for streaming responses (`Flusher`-driven chunked write-through).
4. **`-g`/`--set`/default-set/mutual-exclusion CLI validation** (REQ-PROXY-008~011) — `internal/proxy/group_selection.go` `ResolveActiveGroups`: pure function wiring `-g`/`--set` onto the ALREADY-IMPLEMENTED M1 `ResolveDefaultSet` for the neither-flag path; mutual exclusion enforced; unknown group/set names rejected. Cobra CLI registration itself is M4 scope (library-level wiring only here, per the M2 delegation's explicit instruction).
5. **`litellm` group: pure passthrough** (REQ-PROXY-017) — `internal/proxy/litellm.go` `NewLiteLLMProxy`: `httputil.ReverseProxy` with `FlushInterval: -1`, no body translation.
6. **`bedrock` group** — `internal/proxy/bedrock.go` + `internal/proxy/bedrock_aws.go`, per the Investigation B decision above.
7. **Two-daemon-callers-share-one-daemon test** (AC-PROXY-001b) — `TestDaemon_AcquireFromTwoCallersSharesOneDaemon` (`daemon_test.go`): two `Daemon` instances pointing at the SAME machine-scope state directory (modeling two independent CLI invocations from two different project working directories) share exactly one server — the second caller's factory is NEVER invoked, both addresses are identical, refcount reaches 2. **Scope note**: this is an in-process hermetic test parameterizing the caller rather than spawning two real OS processes — full multi-process CLI-invocation evidence is M4 scope; documented here, not silently claimed as a real cross-process test.
8. **AC-PROXY-002 (daemon holds groups the session did not request)** — `TestDaemon_HoldsAllRegisteredGroupsRegardlessOfSessionActiveSet` (`daemon_holds_all_groups_test.go`): a session that activates only group A via `-g A` can still route a direct `<group>/<model>` reference to group B, proving the daemon's shared `Registry` (not the per-session `Catalog.activeGroups` subset) is what the daemon actually holds.
9. **End-to-end mechanical equivalent of AC-PROXY-009** — `TestEndToEnd_ClaudeCodeConversationViaProxy` (`anthropic_handler_test.go`): a full SSE round trip through `StartServer` + `MessagesHandler` + an `httptest` Anthropic-shaped backend, asserting event order survives the relay and the streamed text deltas assemble correctly. A REAL Claude Code CLI conversation against a live backend is NOT exercised (neither an interactive client nor a live backend is available in this environment) — reported as a Gap below, not claimed as AC-PROXY-009 PASS.

### RED evidence (verbatim, captured before each implementation unit)

```
$ go vet ./internal/proxy/...
# github.com/modu-ai/moai-adk/internal/proxy
# [github.com/modu-ai/moai-adk/internal/proxy]
internal/proxy/daemon_test.go:16:14: undefined: DefaultDaemonStateDir
internal/proxy/daemon_test.go:30:7: undefined: NewDaemon
internal/proxy/daemon_test.go:59:13: undefined: NewDaemon
internal/proxy/daemon_test.go:60:13: undefined: NewDaemon
internal/proxy/daemon_test.go:105:13: undefined: NewDaemon
internal/proxy/daemon_test.go:106:13: undefined: NewDaemon
internal/proxy/daemon_test.go:132:7: undefined: NewDaemon
internal/proxy/daemon_test.go:167:7: undefined: NewDaemon
internal/proxy/daemon_test.go:185:7: undefined: NewDaemon
```
(mirrored for each subsequent unit: `undefined: StartServer`, `undefined: ResolveActiveGroups`, `undefined: ProbeMessagesEndpoint`, `undefined: ToBedrockInvokeModelBody`, `undefined: NewMessagesHandler` — one compile-failure RED captured per file before its implementation, same discipline as M1.)

### GREEN evidence (verbatim, this run, this tree — HEAD after implementation)

```
$ go test ./internal/proxy/... -v -coverprofile=/tmp/proxy_final.out
PASS
ok  	github.com/modu-ai/moai-adk/internal/proxy	<duration>
$ go tool cover -func=/tmp/proxy_final.out | tail -1
total:									(statements)			87.8%
```
80/80 test functions PASS, 0 FAIL (`grep -c '^--- PASS'` → 80, `grep -c '^--- FAIL'` → 0). All 25 M1-era tests remain green — no M1 regression.

### Build / vet / cross-platform (this run)

```
$ go build ./...
(exit 0, no output)
$ go vet ./...
(exit 0, no output)
$ GOOS=windows GOARCH=amd64 go build ./...
(exit 0, no output)
```

### Template neutrality (unchanged — M2 touches no template surface)

```
$ grep -n "PROXY-001\|SPEC-PROXY" internal/template/templates/.moai/config/sections/llm.yaml
(no output — 0 matches, exit 1)
```

### Subagent boundary grep (C-HRA-008 family)

```
$ grep -rn "AskUserQuestion\|mcp__askuser" internal/proxy/ | grep -v "_test.go" | grep -v "// "
(no output — 0 matches, exit 1)
```

### AC PASS/FAIL matrix — M2-scoped AC IDs

| AC ID | REQ | Status | Verification | Actual Output |
|---|---|---|---|---|
| AC-PROXY-001a | REQ-PROXY-001 | PASS | `go test -run TestDaemon_AcquireStartsServerOnFirstCall ./internal/proxy/...` | PASS — first `Acquire` invokes the factory exactly once, refcount 1 |
| AC-PROXY-001b | REQ-PROXY-002 | PASS (in-process scope note) | `go test -run TestDaemon_AcquireFromTwoCallersSharesOneDaemon ./internal/proxy/...` | PASS — two callers, one factory invocation, identical address, refcount reaches 2 |
| AC-PROXY-001c | REQ-PROXY-003 | PASS | `go test -run TestDaemon_ReleaseToZeroSelfTerminates ./internal/proxy/...` | PASS — stop function invoked exactly once when refcount reaches 0, state cleared |
| AC-PROXY-002 | REQ-PROXY-004 | PASS | `go test -run TestDaemon_HoldsAllRegisteredGroupsRegardlessOfSessionActiveSet ./internal/proxy/...` | PASS — direct reference to group B (outside session's `-g A` active set) routes successfully via the shared Registry |
| AC-PROXY-005a | REQ-PROXY-008 | PASS | `go test -run TestResolveActiveGroups_GFlagSelectsNamedGroups ./internal/proxy/...` | PASS — `-g personal,backup` returns exactly `[personal backup]` |
| AC-PROXY-005b | REQ-PROXY-009 | PASS | `go test -run TestResolveActiveGroups_SetFlagSelectsNamedSet ./internal/proxy/...` | PASS — `--set heavy` returns the stored set's group list `[work personal]` |
| AC-PROXY-005c | REQ-PROXY-010 | PASS | `go test -run TestResolveActiveGroups_NeitherFlagFallsBackToResolveDefaultSet ./internal/proxy/...` | PASS — neither flag falls through to `ResolveDefaultSet`; project pointer wins over machine default |
| AC-PROXY-005d | REQ-PROXY-011 | PASS | `go test -run TestResolveActiveGroups_GFlagAndSetFlagAreMutuallyExclusive ./internal/proxy/...` | PASS — both flags set together is a hard error |
| AC-PROXY-005f | REQ-PROXY-025 | PASS | `go test -run TestResolveActiveGroups_PropagatesResolveDefaultSetHardError ./internal/proxy/...` | PASS — a project pointer naming a nonexistent set is a hard error, no silent fallback (M1's `ResolveDefaultSet` behavior, reused unmodified) |
| AC-PROXY-009 | REQ-PROXY-016 | **PASS-WITH-DEBT** (mechanical equivalent only) | `go test -run TestEndToEnd_ClaudeCodeConversationViaProxy ./internal/proxy/...` | PASS on the mechanical equivalent (full SSE round trip through `StartServer`+`MessagesHandler`+`httptest` backend, event order + assembled text verified). A REAL Claude Code CLI conversation against a live backend was NOT exercised — see Gaps |
| AC-PROXY-010 | REQ-PROXY-017 | PASS | `go test -run TestNewLiteLLMProxy_RelaysBodySemanticallyIdentical ./internal/proxy/...` | PASS — backend receives the byte-identical request body and path the client sent |
| AC-PROXY-015a | REQ-PROXY-022 | PASS | `go test -run TestStartServer_BindsLoopbackOnly ./internal/proxy/...` | PASS — bound address's host `net.ParseIP(...).IsLoopback()` is true |
| AC-PROXY-015b | REQ-PROXY-023 | PASS | `go test -run TestStartServer_UsesUnprivilegedPort ./internal/proxy/...` | PASS — bound port >= 1024 |

### Not in M2 scope (remain untested / unimplemented, by design)

- Shared OpenAI↔Anthropic translation layer, codex adapter, credential reading — M3 (plan.md §D). `internal/proxy/anthropic_handler.go`'s `codex`/`openai-compatible` branches return 501, verified by `TestMessagesHandler_UnwiredGroupTypeIsNotImplemented`.
- `internal/cli/proxy.go` Cobra command registration (actual `moai proxy` CLI entrypoint, `-g`/`--set` flag parsing at the CLI layer) — M4 (plan.md §D). This milestone delivers the library-level `ResolveActiveGroups` function the M4 CLI layer will call.
- AC-PROXY-011, 012a, 012b, 013, 014 — M3 scope (translation layer / codex).
- Full multi-process daemon verification (real separate OS processes, not the in-process two-`Daemon`-instance hermetic model) — deferred to M4 actual CLI invocation testing.

### Residual risk

- `bedrock_aws.go`'s `newBedrockEventStreamReader`/`Read`/`Close` (the real AWS EventStream binary-framing → SSE reconstruction) are 0% covered — hand-crafting valid `vnd.amazon.eventstream` binary frames was judged out of proportion to this milestone's scope; `InvokeStream`'s error path IS covered against a local `httptest` backend with static AWS credentials (no real network/IAM). This is the single largest coverage gap in the package and the item most needing live-AWS verification before production use.
- `TestDaemon_AcquireFromTwoCallersSharesOneDaemon` and the self-termination test model multi-process coordination with two `Daemon` Go values in ONE test process, not two real OS processes — the cross-process lock-file coordination code path (the flock/LockFileEx layer itself) is exercised, but true process-crash / signal-interruption scenarios are not.
- `TestEndToEnd_ClaudeCodeConversationViaProxy` verifies the daemon's HTTP relay mechanics, not an actual Claude Code CLI client's behavior against the proxy — the interactive-client leg of AC-PROXY-009 requires operator action outside this environment.
- `golangci-lint` remains absent from this environment (same gap M1 reported); `go vet ./...` is the only static-analysis signal captured this run.

## M3 subsection — 공유 OpenAI↔Anthropic 번역 계층

**cycle_type: tdd** (RED-GREEN-REFACTOR). Milestone M3 ("공유 OpenAI↔Anthropic 번역 계층", plan.md §D — "이 마일스톤이 작업량의 대부분이다"). Continues in the SAME worktree/session as M1/M2, after orchestrator independent re-verification + user approval. This run was interrupted mid-milestone by a host-machine sleep event (infrastructure failure, not a rejection) and resumed from the orchestrator-confirmed on-disk state; the RED/GREEN evidence below spans both the pre-interruption and post-resume portions of the same continuous implementation. M4 not started.

### M3 scope delivered (plan.md items 1-9)

1. **Non-streaming request/response translation** (`translate.go`) — `ToOpenAIChatRequest` / `FromOpenAIChatResponse`. Field mapping: `model` kept in-body (unlike Bedrock's InvokeModel, which takes the model out-of-band); `system` (string OR content-block-array form) becomes a leading system-role message; `stop_sequences` renames to `stop`; `top_k` dropped (no OpenAI equivalent); `tools` translates to the function-tool wrapper; assistant `tool_use` + user `tool_result` blocks map to OpenAI `tool_calls` / `tool` role messages.
2. **Streaming SSE event-stream reconstruction** (`translate_stream.go`) — `openAIStreamTranslator`, a state machine (chunk boundaries are NOT 1:1 with Anthropic events, confirmed the state-machine requirement plan.md anticipated) turning OpenAI delta chunks into `message_start` / `content_block_start` / `content_block_delta` / `content_block_stop` / `message_delta` / `message_stop`. `message_delta`/`message_stop` are **deferred** until the first of: a usage-only trailing chunk, `[DONE]`, or EOF — this eliminates the co-located-vs-trailing-usage gap (handles both OpenAI's `stream_options.include_usage` trailing-chunk shape AND self-hosted-server co-located-usage shape with the same code path).
3. **`tool_use` block translation / fragment reassembly** — OpenAI `tool_calls[].function.arguments` fragments (arriving across many chunks) are relayed immediately as `input_json_delta` chunks; the reassembly happens client-side by concatenation, the SAME mechanism Anthropic's own streaming API uses — the translator does not buffer/reassemble server-side, it relays.
4. **`openai-compatible` group** (`openai_compatible.go`) — the translator's first consumer. No auth header applied in v1 (self-hosted GPU cluster; per plan.md "인증이 단순해서 번역 로직만 검증할 수 있다" — advisor-confirmed: no AC constrains openai-compatible auth, and an api-key config field would collide with REQ-PROXY-024's reference-only credential rule anyway).
5. **`codex` credential reader** (`codex_credentials.go`) — `ReadCodexPlaintextAuthFile` lenient-parses the research.md §2.3 observed schema (top-level `auth_mode`/`OPENAI_API_KEY`/`tokens`/`last_refresh`, `tokens` sub-object with `id_token`/`access_token`/`refresh_token`/`account_id`); unknown fields never break parsing (encoding/json's default unmapped-key tolerance), only the fields actually needed (`auth_mode`, `tokens.access_token`, `tokens.account_id`) are required. An `auth_mode` other than the one observed value `"chatgpt"` returns an error wrapping `ErrCodexUnknownAuthMode` — design.md §5.3's "모르는 auth_mode를 만나면 추측하지 말고 그 그룹을 비활성으로 표시한다" implemented literally: the group deactivates rather than guessing how to authenticate. **Both storage locations are attempted** (`ReadCodexCredentials` tries plaintext first, then `ReadCodexOSCredentialStore`) — the OS-credential-store leg does NOT guess a service/account name to probe (research.md §2.3 never observed that location's format); it returns a fixed, documented `ErrCodexOSStoreUnverified` sentinel. This is the honest implementation of "try both locations": the attempt is a real callable code path, it just cannot succeed until the format is actually observed on a real OS-credential-store-using install — inventing a service name to probe would itself be an unverified-premise claim (verification-claim-integrity.md §1.1 surface 4).
6. **Verification against an official OpenAI Codex CLI install** — **NOT possible in this environment.** `command -v codex opencodex` confirms this machine has ONLY `opencodex` (`/opt/homebrew/bin/opencodex`), NOT the official `codex` binary — the exact caveat research.md §2.3 already flagged. Re-inspecting this machine's `~/.codex/auth.json` key names (values not read) reproduced the identical schema research.md observed — but this is the SAME instance, not independent verification. `internal/proxy/codex_credentials_test.go`'s `TestReadCodexPlaintextAuthFile_AgainstOfficialCodexCLI` encodes this honestly: it probes `exec.LookPath("codex")` and **SKIPS with an explicit reason** naming exactly why (opencodex-only environment) rather than silently passing or being omitted from the suite. If an official `codex` binary with a logged-in `auth.json` is ever present, this test runs for real and confirms or refutes the schema. Reported as a Gap below — NOT upgraded to a claimed PASS.
7. **Investigation: which API endpoint the codex token authenticates against** — **narrowed but not settled.** research.md §2.3's `auth_mode: "chatgpt"` observation already narrowed the target toward OpenAI's ChatGPT-backend API (distinct from the `api.openai.com` platform API). This delegated session has **no WebFetch/WebSearch tool** to complete the investigation a prior plan-phase agent started with WebFetch. A Bash-only attempt was made: this machine's `~/.codex/opencodex.config.toml` / `~/.codex/config.toml` carry a `base_url` pointing at `http://127.0.0.1:10100` — a LOCAL reverse-proxy override specific to this development machine, not the official CLI's real default, and not evidence of anything about the official endpoint (also, this is `opencodex`, not the official CLI — see item 6). **Decision made instead of guessing**: rather than hardcode an unverified URL (which would embed TWO unverified claims — the URL AND the request shape that endpoint expects), `serveCodex` REQUIRES `Group.BaseURL` to be explicitly configured for codex groups; an empty `BaseURL` isolates the group with a reason naming the open investigation (see item 9 below). This satisfies plan.md item 7's "확정하거나, 확정할 수 없었던 이유와 남은 부분을 명시적으로 기록한다" via the second branch.
8. **`copilot` group** — confirmed NOT implemented. `serveCodex`/`serveOpenAICompatible` exist for `codex`/`openai-compatible` only; `MessagesHandler.ServeHTTP`'s switch statement has no `GroupTypeCopilot` case, so a copilot-typed group request falls through to the `default` branch (501, "not yet wired"). `TestMessagesHandler_UnwiredCopilotGroupTypeIsNotImplemented` and the updated `TestMessagesHandler_UnwiredGroupTypeIsNotImplemented` (repointed from a codex fixture to a copilot fixture now that codex is wired, per explicit advisor guidance — the M1 type enum entry needed no schema change) both verify this. `grep -n "GroupTypeCopilot" internal/proxy/*.go | grep -v _test.go` shows the type constant (registry.go, M1) and its `SupportedInV1()` exclusion (M1) only — no adapter code anywhere.
9. **Group-level isolation on credential-read failure** (REQ-PROXY-021, AC-PROXY-014) — `EvaluateGroupStatusWithCodexProbe` (`codex_group.go`) extends M1's `EvaluateGroupStatus` (type-support isolation) with M3's credential/config-readiness isolation for codex-type groups: a codex group with no `base_url` configured, or whose credential store cannot be read, is isolated inactive with a reason naming the group and the attempted read path — daemon startup and every OTHER group continue unaffected. Same isolation shape as M2's unsupported-type isolation, extended per plan.md's explicit instruction ("REQ-PROXY-021과 같은 형태").

### Single-shared-translator evidence (AC-PROXY-011 / REQ-PROXY-018)

```
$ grep -rn "ToOpenAIChatRequest(\|FromOpenAIChatResponse(\|TranslateOpenAIStreamToAnthropicSSE(" internal/proxy/*.go | grep -v "_test.go"
internal/proxy/openai_shared.go:45:	translated, err := ToOpenAIChatRequest(anthropicBody)
internal/proxy/openai_shared.go:77:		rc := TranslateOpenAIStreamToAnthropicSSE(resp.Body, messageID, model)
internal/proxy/openai_shared.go:105:	anthResp, err := FromOpenAIChatResponse(body)
internal/proxy/translate_stream.go:328:func TranslateOpenAIStreamToAnthropicSSE(r io.Reader, messageID, model string) io.ReadCloser {
internal/proxy/translate.go:57:func ToOpenAIChatRequest(anthropicBody []byte) ([]byte, error) {
internal/proxy/translate.go:289:func FromOpenAIChatResponse(openaiBody []byte) ([]byte, error) {

$ grep -n "serveOpenAIShapedMessages(" internal/proxy/*.go | grep -v "_test.go"
internal/proxy/codex_group.go:71:	serveOpenAIShapedMessages(w, r, client, group.BaseURL, auth, anthropicBody, model, stream)
internal/proxy/openai_compatible.go:18:	serveOpenAIShapedMessages(w, r, client, baseURL, noAuth, anthropicBody, model, stream)
internal/proxy/openai_shared.go:35:func serveOpenAIShapedMessages(
```
All three translation entry points have exactly ONE non-test caller site (`openai_shared.go`), and exactly TWO callers of the shared handler (`codex_group.go`, `openai_compatible.go`) — no per-consumer duplicate translation implementation exists anywhere in the package.

### RED evidence (verbatim, captured before each implementation unit — spans pre- and post-interruption)

```
$ go vet ./internal/proxy/...  (before translate.go existed)
vet: internal/proxy/translate_test.go:25:14: undefined: ToOpenAIChatRequest

$ go vet ./internal/proxy/...  (before translate_stream.go existed)
vet: internal/proxy/translate_stream_test.go:40:8: undefined: newOpenAIStreamTranslator

$ go vet ./internal/proxy/...  (before openai_shared.go existed)
vet: internal/proxy/openai_shared_test.go:37:2: undefined: serveOpenAIShapedMessages

$ go vet ./internal/proxy/...  (before codex_credentials.go existed)
vet: internal/proxy/codex_credentials_test.go:44:16: undefined: ReadCodexPlaintextAuthFile

$ go vet ./internal/proxy/...  (before codex_group.go existed, post-resume)
vet: internal/proxy/codex_group_test.go:21:2: undefined: serveCodex

$ go test ./internal/proxy/... -run 'TestMessagesHandler_RoutesToOpenAICompatible|TestMessagesHandler_RoutesToCodex' -v  (before handler wiring, post-resume)
=== RUN   TestMessagesHandler_RoutesToOpenAICompatible
    anthropic_handler_openai_test.go:43: status = 501, want 200; body=proxy: group type "openai-compatible" is not yet wired into the daemon /v1/messages surface
--- FAIL: TestMessagesHandler_RoutesToOpenAICompatible (0.00s)
=== RUN   TestMessagesHandler_RoutesToCodex
    anthropic_handler_openai_test.go:87: status = 501, want 200; body=proxy: group type "codex" is not yet wired into the daemon /v1/messages surface
--- FAIL: TestMessagesHandler_RoutesToCodex (0.00s)
FAIL
```

### GREEN evidence (verbatim, this run, this tree — HEAD after implementation)

```
$ go test ./internal/proxy/... -v 2>&1 | grep -c '^--- PASS'
120
$ go test ./internal/proxy/... -v 2>&1 | grep -c '^--- FAIL'
0
$ go test ./internal/proxy/... -v 2>&1 | grep -c '^--- SKIP'
1
$ go test ./internal/proxy/... -coverprofile=/tmp/proxy_m3_cover.out && go tool cover -func=/tmp/proxy_m3_cover.out | tail -1
ok  	github.com/modu-ai/moai-adk/internal/proxy	0.548s	coverage: 86.1% of statements
total:									(statements)				86.2%
```
120 PASS, 0 FAIL, 1 SKIP (the official-Codex-CLI schema verification — reported honestly, not silently). All 109 pre-existing M1/M2 tests remain green — no regression.

### Build / vet / cross-platform (this run)

```
$ go build ./...
(exit 0, no output)
$ go vet ./...
(exit 0, no output)
$ GOOS=windows GOARCH=amd64 go build ./...
(exit 0, no output)
```

### Subagent boundary + template neutrality (unchanged — M3 touches no template surface)

```
$ grep -rn "AskUserQuestion\|mcp__askuser" internal/proxy/ | grep -v "_test.go" | grep -v "// "
(no output — 0 matches, exit 1)
$ grep -n "PROXY-001\|SPEC-PROXY" internal/template/templates/.moai/config/sections/llm.yaml
(no output — 0 matches, exit 1)
```

### AC PASS/FAIL matrix — M3-scoped AC IDs

| AC ID | REQ | Status | Verification | Actual Output |
|---|---|---|---|---|
| AC-PROXY-011 | REQ-PROXY-018 | PASS | grep of translation-entry-point + shared-handler callers (above) | PASS — exactly 1 caller site per translation function, exactly 2 callers of the single shared handler, no duplicate implementation |
| AC-PROXY-012a | REQ-PROXY-019 | PASS | `go test -run TestOpenAIStreamTranslator_TextOnly_EventOrderAndAssembly ./internal/proxy/...` | PASS — event order `message_start → content_block_start → content_block_delta ×2 → content_block_stop → message_delta → message_stop`; assembled text `"Hello, world"` matches backend delta concatenation |
| AC-PROXY-012b | REQ-PROXY-019 | PASS | `go test -run TestOpenAIStreamTranslator_ToolCallFragmentReassembly ./internal/proxy/...` | PASS — reassembled `input_json_delta` fragments parse to `{"location":"NYC"}`, matching the backend's own concatenation |
| AC-PROXY-013 | REQ-PROXY-020 | PASS | `go test -run TestServeCodex_NoInteractiveAuthFlowSourceCheck ./internal/proxy/...` | PASS — grep of `codex_credentials.go`/`codex_group.go` for `exec.Command(`/`http.Get("https://`/`browser`/`OAuth`/`oauth2.` finds 0 matches; credential acquisition is a pure file read |
| AC-PROXY-014 | REQ-PROXY-021 | PASS | `go test -run TestEvaluateGroupStatusWithCodexProbe_IsolatesCredentialFailureOnly ./internal/proxy/...` | PASS — group A (bedrock, healthy) stays active; group B (codex, broken credential store) isolates inactive with a reason naming "B" and the attempted read path; A is unaffected |

### Not in M3 scope (remain untested / unimplemented, by design)

- `copilot` adapter — explicitly OUT of v1 scope (plan.md M3 item 8, spec.md §H); resumption requires all three of plan.md §B.2's Copilot items answered first (storage location + token exchange + inference endpoint), belonging to a follow-up SPEC.
- M4 CLI wiring/docs — not started (this delegation stops before M4 per the semi-autonomous checkpoint instruction).
- Real official-OpenAI-Codex-CLI live verification (item 6) and the codex endpoint's final confirmation (item 7) — both explicit Gaps below, not silently deferred.

### Gaps (explicitly not observed, M3)

- **Item 6 (official Codex CLI schema verification) — genuinely could not be performed in this environment.** Only `opencodex` is installed, not the official `codex` binary. The re-observed schema on this machine is the SAME instance research.md §2.3 already used, not an independent data point. `TestReadCodexPlaintextAuthFile_AgainstOfficialCodexCLI` SKIPs with this exact reason rather than passing or being silently omitted.
- **Item 7 (codex token target endpoint) — narrowed, not settled.** No WebFetch/WebSearch tool was available to this delegated session to complete the investigation research.md §2.3 started. The mitigation (require explicit `Group.BaseURL`, isolate-with-reason otherwise) is a documented engineering decision, not a resolution of the underlying open question — the endpoint itself remains unconfirmed.
- **Streaming translator real-network coverage** — `translate_stream_test.go` and `openai_shared_test.go` exercise the state machine and the HTTP relay against `httptest` fixtures; no real OpenAI-compatible backend (self-hosted GPU cluster, real LiteLLM, or a real Codex ChatGPT-backend endpoint) was available to validate against actual wire traffic. The shapes tested are drawn from the OpenAI Chat Completions API's publicly documented schema, not observed live traffic.
- **Multi-choice (`n>1`) streaming** — `openAIStreamTranslator` reads `choices[0]` only (v1 scope, documented in `translate_stream.go`'s doc comment); a backend returning `n>1` choices is not handled and is out of this milestone's scope.
- The pre-existing, unrelated `TestRunHookEvent_ReadInputError` panic in `internal/cli/coverage_test.go` was not re-checked this milestone (M1/M2 already confirmed it's out of `internal/proxy` scope via `git status --short`; M3's diff is exclusively `internal/proxy/*` — same non-overlap holds).

### Residual risk (M3)

- The endpoint-unsettled decision (item 7) means a real `codex` group cannot actually serve a request in this build until an operator supplies a verified `base_url` — this is a deliberate scope-narrowing (explicit config over a guessed default), but it does mean M3's codex adapter is not yet end-to-end usable without that external input, unlike `openai-compatible` which works immediately given any real base_url.
- `ReadCodexOSCredentialStore`'s permanent-failure design means codex groups relying on the OS credential store (rather than the plaintext file) can NEVER activate in the current build — this is intentional (no fabricated service-name guess) but is a real functional gap until the OS-store format is independently observed and implemented.
- The streaming state machine's chunk-arrival assumptions (id+name arriving together or id+name-then-arguments, `finish_reason` eventually arriving, at most one usage-bearing chunk) are drawn from the OpenAI Chat Completions API's documented behavior and from vLLM/TGI-style self-hosted servers' typical co-located-usage shape; an OpenAI-shaped backend with a genuinely different chunking discipline (e.g., id and name split across two separate chunks with a content delta between them) has not been tested and could produce a block ordering surprise.
- `golangci-lint` remains absent from this environment (same gap M1/M2 reported); `go vet ./...` is the only static-analysis signal captured this run.

## §E.3 Run-phase Audit-Ready Signal

- run_milestone: M3 of M1-M4 (plan.md §D)
- run_status: milestone-complete — awaiting orchestrator review before M4 (semi-autonomous progression, per spawn instruction). This run was interrupted mid-milestone by a host-machine sleep event and resumed from orchestrator-confirmed on-disk state (build/tests clean, no rejection implied).
- m1_commit_strategy: single milestone commit — `50542c7ff` on branch `worktree-agent-a1ebd35fa5ba3d08b`
- m2_commit_strategy: single milestone commit — `4407cd82a` on branch `worktree-agent-a1ebd35fa5ba3d08b`
- m3_commit_strategy: single milestone commit — `086b9f622` on branch `worktree-agent-a1ebd35fa5ba3d08b` (this agent's isolated worktree; NOT pushed to `origin/main` — Tier L routes through `manager-git`/PR per SPEC Phase Discipline Route B, out of this delegation's scope)
- ac_pass_count (M1+M2+M3-scoped, cumulative): 25 (M1: 7 — AC-PROXY-003, 004, 006, 007a, 007b, 008, 016; M2: 13 — AC-PROXY-001a, 001b, 001c, 002, 005a, 005b, 005c, 005d, 005f, 009 [with-debt], 010, 015a, 015b; M3: 5 — AC-PROXY-011, 012a, 012b, 013, 014)
- ac_fail_count (M1+M2+M3-scoped): 0
- new_files (M3): internal/proxy/{anthropic_handler_openai_test,codex_credentials,codex_credentials_test,codex_group,codex_group_test,openai_compatible,openai_shared,openai_shared_test,translate,translate_stream,translate_stream_test,translate_test}.go (12 files)
- modified_files (M3): internal/proxy/anthropic_handler.go (openai-compatible/codex dispatch wiring), internal/proxy/anthropic_handler_test.go (M2 unwired-type fixture repointed from codex to copilot per advisor guidance now that codex is wired), .moai/specs/SPEC-PROXY-001/progress.md (this file)
- new_warnings_or_lints_introduced: unknown — `golangci-lint` still not installed in this environment; `go vet ./...` is clean (exit 0). Same residual-risk gap as M1/M2, not newly introduced.
- cross_platform_build.linux_darwin: PASS (native `go build ./...`, exit 0)
- cross_platform_build.windows: PASS (`GOOS=windows GOARCH=amd64 go build ./...`, exit 0)
- total_run_phase_files (M3): 14 (12 new + 2 modified)

### Gaps (explicitly not observed, M2)

- Real AWS Bedrock EventStream wire-format reconstruction was never exercised against a live Bedrock endpoint — `bedrock_aws.go`'s streaming reader is untested beyond the SDK-level error path (see Residual risk above).
- A real Claude Code CLI conversation routed through `moai proxy` (the literal AC-PROXY-009 scenario) requires an interactive client and a live backend, neither available in this environment; the mechanical equivalent (full SSE relay through the actual daemon HTTP surface) was verified instead. AC-PROXY-009 is reported PASS-WITH-DEBT, not a bare PASS.
- Multi-process daemon coordination (real separate OS processes sharing the lock file) is modeled in-process; the actual `moai proxy` CLI entrypoint that would launch real separate processes is M4 scope and does not exist yet.
- The pre-existing, unrelated `TestRunHookEvent_ReadInputError` panic in `internal/cli/coverage_test.go` (first observed during M1's regression sweep) was re-confirmed present and unrelated: `git status --short` after the M2 commit shows no changes to `internal/cli/`. Reported again as a known baseline defect, out of scope for this SPEC.

### Residual risk (M2)

See the M2 subsection's "Residual risk" list above (bedrock EventStream reconstruction, in-process multi-daemon-caller modeling, E2E-vs-real-Claude-Code-client gap, golangci-lint absence). M1's residual-risk item "M2's `-g`/`--set` wiring should add a direct test" is now resolved by `TestResolveActiveGroups_*` (7 tests) — no longer an open item.

### Gaps and Residual risk (M3)

See the M3 subsection's "Gaps" and "Residual risk" lists above — the official-Codex-CLI verification (item 6, SKIPped with an explicit reason) and the codex endpoint (item 7, narrowed but not settled, mitigated via required explicit `base_url`) are the two headline items. Neither M1 nor M2's open residual-risk items are affected by M3's changes (M3's diff is exclusively new files plus the M2-authored `anthropic_handler.go`'s dispatch switch and its M2 test file's unwired-type fixture).

## §E.4 Sync-phase Audit-Ready Signal

_<pending sync-phase>_
