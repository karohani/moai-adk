# research.md — SPEC-PROXY-001 사전 조사

> 목적: `moai proxy`(로컬 다중 백엔드 LLM 게이트웨이) 설계에 필요한 외부 사실을 **직접 확인**하고, 확인하지 못한 항목을 명시적으로 남긴다.
> 원칙: 확인한 것만 사실로 적는다. 확인하지 못한 것은 §5에 미확인 항목으로 모아 표시한다 (`.claude/rules/moai/core/verification-claim-integrity.md` §1).

---

## §1. 조사 방법과 한계

1차 조사는 **WebFetch만** 사용했다. WebSearch는 그 세션의 도구 목록에 없어서, 공식 문서 URL을 직접 지정해 가져오는 방식만 가능했다. 그래서 "검색으로 찾아냈어야 할 문서"를 놓쳤을 가능성이 남아 있고, 아래 §5의 미확인 항목 중 일부는 그 한계 때문일 수 있다.

**2차 보강(2026-08-20)**: §2.3 Codex 자격 증명 저장소 항목에 한해 **로컬 머신의 실제 파일을 직접 관찰**한 근거를 추가했다. 이건 공식 문서가 아니라 한 대의 머신에서 관찰한 사실이고, 일반화 가능성에 제약이 있다 — §2.3의 단서 항목에 그 제약을 명시했다.

로컬 코드베이스 근거는 Read/Grep으로 직접 확인했다.

확인 등급을 네 가지로 나눈다.

| 등급 | 뜻 |
|---|---|
| 확인됨 | 공식 문서를 실제로 가져와 해당 문장을 읽었다 |
| 로컬 관찰 | 공식 문서에는 없지만 로컬 머신의 실제 파일/명령 출력을 직접 관찰했다. **일반화하려면 별도 확인이 필요하다** |
| 부분 확인 | 일부는 문서에 있었고, 나머지는 문서가 다루지 않았다 |
| 미확인 | 가져온 문서가 그 주제를 아예 다루지 않았다 |

---

## §2. 검증 대상별 결과

### §2.1 LiteLLM 게이트웨이의 Anthropic `/v1/messages` 지원 — 확인됨

출처: https://docs.litellm.ai/docs/anthropic_unified (WebFetch로 직접 확인)

확인한 내용:

- LiteLLM 프록시는 **`/v1/messages` 경로를 Anthropic 호환 엔드포인트로 노출한다.**
- **스트리밍을 지원한다** (문서에 "Streaming ✅" 표기, `stream=True` 예제 제공).
- 이 엔드포인트로 서빙 가능한 백엔드는 "All LiteLLM supported providers" — 문서가 명시한 예: `openai, anthropic, bedrock, vertex_ai, gemini, azure, azure_ai` 등.
- 요청은 Anthropic messages API 규격을 따르고, **어느 백엔드가 처리하든 응답도 같은 형식으로 돌아온다.**

설계 영향: `litellm` 그룹 타입은 **번역 계층이 필요 없다.** 요청 형식과 응답 형식이 이미 우리가 클라이언트에게 노출할 형식과 같기 때문에, 인증 헤더를 바꿔 끼우고 스트림을 중계하는 것으로 끝난다. 이게 M1에서 `litellm`을 먼저 처리해야 하는 근거다.

주의: 이건 "LiteLLM이 그런 엔드포인트를 제공한다"는 사실만 확인한 것이다. 사용자가 실제로 띄워 둔 LiteLLM 인스턴스가 그 경로를 켜 두었는지는 런타임에 확인해야 할 문제다 (§5 NC-4 참조).

### §2.2 Amazon Bedrock — 확인됨

출처 3개 (모두 WebFetch로 직접 확인):
- https://docs.aws.amazon.com/bedrock/latest/userguide/conversation-inference.html
- https://docs.aws.amazon.com/bedrock/latest/userguide/inference-profiles-support.html
- https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/bedrockruntime

확인한 내용:

**Converse API**
- Converse API는 `bedrock-runtime` 엔드포인트에서만 제공된다.
- 연산은 `Converse`(동기)와 `ConverseStream`(스트리밍) 두 가지다.
- 문서 문장: "the Converse API provides a consistent API that works with all Amazon Bedrock models that support messages. This means you can write code once and use it with different models." — 즉 **모델별로 페이로드를 갈아끼울 필요가 없다.**
- 모델 고유 파라미터가 있으면 model-specific 구조체로 따로 넘길 수 있다.
- **tool use를 지원한다** (문서가 tool use / guardrails 구현에 Converse API를 쓰라고 명시).
- 기존 `InvokeModel` / `InvokeModelWithResponseStream`으로도 대화형 앱을 만들 수 있지만, 그 경우 모델별 페이로드를 직접 다뤄야 한다.

**모델 ID 형태 (cross-region inference profile)**
- 문서가 든 실제 예: `us.anthropic.claude-3-haiku-20240307-v1:0`
- 앞의 `us.` 같은 지역 접두사는 cross-Region inference profile을 가리킨다. 소스 리전에서 호출하면 목적지 리전 중 하나로 자동 라우팅된다.
- 같은 프로필 ID라도 **소스 리전에 따라 목적지 리전 집합이 달라진다.** 문서 예: `us.anthropic.claude-3-haiku-20240307-v1:0`를 US East (Ohio)에서 부르면 `us-east-1`/`us-east-2`/`us-west-2`로 갈 수 있지만, US West (Oregon)에서 부르면 `us-east-1`/`us-west-2`로만 간다.
- 지역(US/EU/APAC)에 묶인 프로필의 목적지 리전 목록은 바뀌지 않지만, Global 프로필의 목적지는 AWS가 리전을 추가하면서 **시간에 따라 변할 수 있다.**

**Go SDK**
- import 경로: `github.com/aws/aws-sdk-go-v2/service/bedrockruntime`
- `Client`가 노출하는 연산에 `Converse`, `ConverseStream`, `InvokeModel`, `InvokeModelWithResponseStream`이 모두 있음을 확인했다.
- SigV4 서명은 SDK가 처리한다. `WithSigV4SigningName` / `WithSigV4SigningRegion` 오버라이드가 제공되고, 자격 증명은 `AuthSchemeResolver` + identity resolver 경로로 해결된다. 즉 **SigV4 서명을 우리가 직접 구현할 이유가 없다.**
- 관련 의존성: `github.com/aws/aws-sdk-go-v2/aws`, `github.com/aws/smithy-go`

설계 영향: `bedrock` 그룹은 번역 부담이 얇다. 다만 "얇다"가 "없다"는 아니다 — Converse의 메시지 구조는 Anthropic `/v1/messages` 스키마와 **비슷할 뿐 같지 않다.** 필드 이름과 중첩 구조를 실제로 대조하는 작업은 구현 시점에 필요하다 (§5 NC-3).

### §2.3 Codex CLI 자격 증명 저장소 — 부분 확인 + 로컬 관찰

출처: https://learn.chatgpt.com/docs/auth
(원래 `https://developers.openai.com/codex/auth`로 요청했으나 308 Permanent Redirect로 위 URL을 가리켰고, 리다이렉트된 URL을 다시 가져와 확인했다.)

확인한 내용:

- 저장 위치: **`~/.codex/auth.json`** — 문서 문장: "Codex caches login details locally in a plaintext file at `~/.codex/auth.json` or in your OS-specific credential store."
- 즉 저장 경로가 **하나가 아니다.** 평문 파일이거나, OS 자격 증명 저장소(macOS Keychain 등)일 수 있다. 어느 쪽을 쓸지 결정하는 조건은 문서에 없다.
- 파일 내용: 문서는 "contains access tokens"라고만 한다. **필드 이름은 공개하지 않는다.**
- 보안 경고: "treat `~/.codex/auth.json` like a password... Don't commit it, paste it into tickets, or share it in chat."

#### 로컬 관찰 (2026-08-20) — 필드 스키마 확보

공식 문서가 공개하지 않는 필드 스키마를 **로컬 머신의 실제 `~/.codex/auth.json`을 직접 관찰**해 확보했다. 값은 읽지 않고 **키 이름만** 조회했다 (자격 증명 노출 방지).

| 항목 | 관찰값 |
|---|---|
| 경로 | `~/.codex/auth.json` |
| 파일 권한 | `0600` (소유자 읽기/쓰기 전용) |
| 최상위 필드 | `auth_mode` (string), `OPENAI_API_KEY` (string 또는 null), `tokens` (object), `last_refresh` (ISO-8601 string) |
| `tokens` 하위 필드 | `id_token`, `access_token`, `refresh_token`, `account_id` (모두 string) |
| 관찰된 `auth_mode` 값 | `"chatgpt"` |
| 관찰된 `OPENAI_API_KEY` | `null` |

`auth_mode: "chatgpt"`는 **ChatGPT 백엔드 OAuth 흐름**으로 로그인했다는 뜻이고, 이는 OpenAI 플랫폼 API 키 인증과 구분된다. 같은 인스턴스에서 `OPENAI_API_KEY`가 `null`인 것이 그 해석을 뒷받침한다 — 이 계정은 API 키 인증을 쓰고 있지 않다.

**[단서] 일반화하기 전에 반드시 확인해야 한다.** 관찰한 머신에 설치된 바이너리는 문자 그대로의 `codex`가 아니라 **`opencodex`** (`/opt/homebrew/bin/opencodex`)였다. 확인한 사실: 이 머신의 PATH에 `codex`는 없고 `opencodex`만 있다. 둘은 `~/.codex/` 설정 디렉터리와 위 `auth.json` 스키마를 공유하지만, **이 SPEC의 `codex` 그룹 판독기 구현은 실제 공식 OpenAI Codex CLI 설치본에 대해 이 스키마를 검증한 뒤에야 권위 있는 것으로 취급해야 한다.** "한 인스턴스에서 관찰됨"을 "보편적으로 확인됨"으로 조용히 승격해서는 안 된다.

여전히 확인하지 못한 것:
- 그 토큰이 **어느 API 엔드포인트를 인증하는지** — `auth_mode: "chatgpt"`라는 관찰이 대상을 **ChatGPT 백엔드 API 쪽으로 강하게 좁히지만**(`api.openai.com` 플랫폼 API가 아닐 가능성이 높다), 정확한 엔드포인트 URL은 여전히 미확인이다. 좁혀졌을 뿐 닫히지 않았다
- ChatGPT 요금제 로그인과 API 키 로그인이 저장 형태에서 어떻게 다른지 (API 키 로그인 인스턴스를 관찰하지 못했다 — 관찰한 인스턴스는 `OPENAI_API_KEY: null`인 ChatGPT 경로 하나뿐이다)
- OS 자격 증명 저장소(macOS Keychain 등)를 쓰는 경우의 저장 형태 — 관찰한 인스턴스는 평문 파일 경로를 썼다

**리포지토리 내부에서 근거를 찾으려 한 시도와 그 결과**: `.claude/rules/moai/core/moai-mcp-tools.md`는 `mcp__moai__codex_setup`을 "Probe local codex install (LookPath + version + auth)"로 기술한다. 이게 사실이라면 로컬에 이미 codex 인증을 확인하는 코드가 있다는 뜻이고, 공개 문서보다 강한 근거가 된다. 그래서 `internal/` 전체를 grep했다:

```
grep -rn "codex_setup\|codex_task\|CodexSetup" internal/ --include="*.go"   → 결과 없음
grep -rln "codex" internal/ --include="*.go"                                → 8개 파일, 전부 무관
```

`internal/agenthost/capability.go`의 `HostCodex Host = "codex"`는 에이전트 호스트 종류를 나타내는 열거값이고 인증 탐침이 아니다. **즉 codex 인증 저장소를 읽는 Go 구현은 이 리포지토리에 없다.** 규칙 문서가 기술한 MCP 도구 표면과 실제 Go 소스가 어긋나 있다 — 이 어긋남 자체는 이 SPEC의 범위 밖이지만, "기존 구현을 재사용하면 된다"는 가정은 성립하지 않는다는 점이 설계에 직접 영향을 준다.

설계 영향: 필드 스키마를 확보했으므로 `codex` 판독기를 **설계할 수는 있게 됐다.** 그러나 취약성의 성격은 바뀌지 않았다 — 이 포맷은 여전히 **공식 문서에 없고**, 관찰 근거는 한 인스턴스뿐이며, OpenAI가 포맷을 바꾸면 조용히 깨진다. M3 설계는 이 취약성을 전제로 해야 한다 (design.md §5.3의 판독 실패 격리 규칙). 스키마가 관찰됐다는 사실이 격리 규칙을 완화할 근거가 되지는 않는다.

### §2.4 GitHub Copilot CLI 인증 — 부분 확인 (가장 불확실)

출처 3개 (모두 WebFetch로 직접 확인):
- https://docs.github.com/en/copilot/how-tos/set-up/install-copilot-cli
- https://docs.github.com/en/copilot/concepts/agents/about-copilot-cli
- https://github.com/github/copilot-cli

확인한 내용:

- 첫 실행 시 `/login` 슬래시 명령으로 인증하도록 안내된다.
- 또는 **fine-grained personal access token**을 쓸 수 있고, 그 토큰에는 **"Copilot Requests" 권한**이 켜져 있어야 한다.
- 토큰 환경변수는 우선순위 순으로 **`COPILOT_GITHUB_TOKEN` → `GH_TOKEN` → `GITHUB_TOKEN`** (install 페이지가 이 세 개를 이 순서로 명시했다. 반면 `github/copilot-cli` README 스냅샷에는 `GH_TOKEN`/`GITHUB_TOKEN` 둘만 나온다 — 두 출처가 어긋난다).

확인하지 못한 것 (중요도 순):
- **디스크상 자격 증명 저장 위치.** 세 출처 모두 다루지 않는다.
- **GitHub 토큰이 별도 Copilot API 토큰으로 교환되는지 여부와 그 교환 절차.**
- **Copilot이 노출하는 추론 API 엔드포인트의 형태와 URL.** 어떤 출처도 엔드포인트 URL을 제시하지 않는다.

**혼동 주의 — 방향이 반대인 발견**: `about-copilot-cli` 페이지에 `COPILOT_PROVIDER_BASE_URL` / `COPILOT_PROVIDER_API_KEY` 환경변수가 나오고 "The `openai` type works with any OpenAI-compatible endpoint, including Ollama and vLLM"이라는 문장이 있다. 이건 **Copilot CLI가 외부 OpenAI 호환 엔드포인트를 소비하는** 설정이다. 우리가 필요한 건 그 반대 방향 — Copilot을 백엔드로 삼아 우리가 호출하는 것 — 이므로 이 환경변수는 우리 문제를 풀어 주지 않는다. 두 방향을 섞으면 설계가 통째로 틀어지므로 여기 명시해 둔다.

설계 영향 (2026-08-20 결정으로 갱신): **`copilot` 그룹은 v1 구현 범위에서 제외한다.** 저장 위치도, 교환 절차도, 엔드포인트도 공개 문서에서 확인되지 않았고, 이 상태로 구현 일정을 잡으면 조사에 막혀 나머지 네 타입까지 함께 지연된다. 그래서 plan.md가 원래 예고했던 자체 우발 대응(선행 조사 게이트 미통과 시 범위 제외)을 **지금 발동한다.**

제외의 형태가 중요하다. **그룹 타입 `copilot`은 스키마와 설계에 그대로 남긴다** — 레지스트리 자료 모델이 이 타입을 유효한 값으로 계속 명명하므로, 나중에 되살릴 때 스키마를 다시 설계할 필요가 없다. 빠지는 것은 **백엔드 어댑터 구현**뿐이다. v1이 실제로 구현하는 그룹 타입은 정확히 넷이다: `bedrock`, `codex`, `litellm`, `openai-compatible`.

재개 조건: 공개 문서가 개선되거나, 실제로 로그인된 Copilot CLI 인스턴스를 가진 사용자가 관찰 근거를 제공하면 (§2.3 Codex에서 방금 한 것과 같은 방식) M3 이후에 착수한다.

---

## §3. 로컬 코드베이스 근거 (직접 확인)

| 확인 대상 | 확인 방법 | 결과 |
|---|---|---|
| `moai cc` 구조 | `internal/cli/cc.go` 전문 읽음 | 75 LOC. cobra `GroupID: "launch"`, `DisableFlagParsing: true`, `runCC` → `unifiedLaunch(profile, "claude", args)` 위임. 상주 프로세스 없음 |
| `llm.glm.models` 별칭 매핑 선례 | `.moai/config/sections/llm.yaml` 읽음 | `high/medium/low/fable/opus/sonnet/haiku` 키가 각각 구체 모델 ID로 매핑됨. `moai proxy`의 그룹별 별칭 블록이 확장할 정확한 형태 |
| `ANTHROPIC_*` 환경변수 상수 | `internal/config/envkeys.go` grep | `EnvAnthropicBaseURL`, `EnvAnthropicAuthToken`, `EnvAnthropicDefaultOpusModel/SonnetModel/HaikuModel` 존재. 인라인 문자열 금지 규칙(`internal/cli/CLAUDE.md`)의 대상 |
| 새 YAML 섹션 추가 절차 | `.claude/rules/moai/core/settings-management.md` 읽음 | 5단계 확정: ① 템플릿에 `<name>.yaml` ② `types.go`에 struct+wrapper ③ `defaults.go` ④ `loader_<name>.go` ⑤ `Loader.Load()` 배선 + `audit_struct_yaml_symmetry_test.go` 등록 |
| 기존 섹션 목록 | `internal/template/templates/.moai/config/sections/` ls | 26개 섹션 존재. `proxy.yaml`은 없음 |
| 기존 프록시/게이트웨이 코드 | grep | 없음 확인 |
| SPEC ID 중복 | `grep -rn "SPEC-PROXY" .moai/specs/` | 매치 없음. `SPEC-PROXY-001`은 신규 |

---

## §4. 번역 부담 비대칭 — 조사에 근거한 정리

조사 결과를 그룹 타입별 구현 비용으로 환산하면 이렇게 갈린다.

| 그룹 타입 | 요청/응답 형태 | 번역 필요 | 근거 |
|---|---|---|---|
| `litellm` | 이미 Anthropic messages | **없음** (헤더 교체 + 스트림 중계) | §2.1 확인됨 |
| `bedrock` | Converse 통합 형태, tool use 지원 | **얇음** (필드 재배치 수준) | §2.2 확인됨 |
| `codex` | OpenAI 형태 | **두꺼움** | §2.3 부분 확인 + 로컬 관찰 (필드 스키마 확보) |
| `copilot` | OpenAI 형태로 추정 | **두꺼움** | §2.4 미확인 — 추정임. **v1 범위 제외** |
| `openai-compatible` | OpenAI 형태 (정의상) | **두꺼움** | 타입 정의 자체가 OpenAI 규격 |

여기서 나오는 설계 결론 하나: **OpenAI 형태 그룹이 셋(`codex`, `copilot`, `openai-compatible`)이고 이들이 같은 번역기를 쓴다.** 번역 계층을 세 번 만들면 안 되고, 한 번 만들어 셋이 소비해야 한다. 이 결론이 M2/M3 경계를 정한다.

`copilot`이 v1에서 빠져도 이 결론은 유지된다 — v1에서 번역기를 소비하는 타입은 둘(`codex`, `openai-compatible`)이지만, 공유 번역기는 **하나**여야 한다. 나중에 `copilot`이 돌아올 때 세 번째 소비자로 붙기만 하면 되도록 설계한다. 소비자가 둘이라는 이유로 타입별 구현으로 되돌아가는 것은 REQ-PROXY-018 위반이다.

번역 작업량의 대부분은 두 곳에 몰린다:
1. **스트리밍 SSE 변환** — OpenAI의 `data: {choices[].delta}` 청크 열을 Anthropic의 `message_start` / `content_block_start` / `content_block_delta` / `content_block_stop` / `message_delta` / `message_stop` 이벤트 열로 재구성해야 한다. 청크 경계가 1:1이 아니다.
2. **tool_use 블록 변환** — OpenAI `tool_calls`(인자가 문자열화된 JSON, 스트리밍 중 조각나서 도착)를 Anthropic `tool_use` 콘텐츠 블록(구조화된 `input` 객체)으로 바꿔야 한다. 조각 재조립이 필요하다.

이 두 가지가 "번역 계층이 M2 전체를 차지한다"는 판단의 근거다.

---

## §5. 미확인 항목

### §5.1 해소된 항목 (2026-08-20)

최초 작성 시 7건이던 미해소 항목 중 3건이 해소됐다.

| 원래 항목 | 해소 방식 | 근거 |
|---|---|---|
| `Codex auth.json 필드 스키마` | **로컬 관찰로 확보** — 최상위 4필드 + `tokens` 하위 4필드 확정. 단, 한 인스턴스 관찰이며 바이너리가 `opencodex`였다는 단서가 붙는다 | §2.3 로컬 관찰 |
| `데몬 범위 — 프로젝트 단위 vs 사용자 단위` | **사용자 결정 — 머신(사용자) 단위.** 그룹 레지스트리 항목이 프로젝트 자원이 아니라 개인/머신 인프라이고, 싱글턴+참조 계수 설계가 여러 프로젝트의 데몬 공유를 전제할 때만 의미를 갖는다 | design.md §3.4 |
| `Copilot 토큰 교환 절차와 추론 엔드포인트` | **범위 결정으로 해소** — 조사가 아니라 결정으로 닫았다. `copilot` 그룹을 v1 구현 범위에서 제외한다(타입은 스키마에 유지). 조사 질문 자체는 아래 §5.2의 Copilot 항목에 합쳐 이월한다 | §2.4 설계 영향, spec.md §H |

### §5.2 마일스톤 이월 확정 항목 (4건 — 사용자 승인 완료, 2026-08-20)

**이 4건은 구현 착수를 막지 않는다.** M1(설정 스키마 + 카탈로그 자료구조)은 이 중 어느 것에도 의존하지 않으며, 각 항목은 그것을 실제로 필요로 하는 마일스톤 안에서 해소하면 된다. 항목별로 어느 마일스톤이 소비하는지는 plan.md §B에 정리했다.

> **처리 상태 — 네 항목 모두 결정이 내려져 있다(미해결 질문 아님).** 오케스트레이터가 `AskUserQuestion` 라운드를 돌려 네 항목 각각에 대해 사용자의 명시적 승인을 받았고, 네 건 모두 **"소비 마일스톤으로 이월"** 로 결정됐다(즉시 조사 대안은 선택되지 않았다). 아래 표식은 종결 형태인 `[CLARIFICATION-DEFERRED → <마일스톤>, 사용자 승인 2026-08-20: <주제>]` 형식으로 **전환**되어 있다 — 전환된 표식 하나가 **무엇이 아직 미확인인지**와 **언제 해소하기로 승인됐는지**를 함께 담는다. 주제 문구는 전환 과정에서 그대로 보존되며, 각 항목 뒤의 결정 기록도 그대로 남는다.

- `[CLARIFICATION-DEFERRED → M3, 사용자 승인 2026-08-20: Codex 토큰이 인증하는 API 엔드포인트]` — **좁혀졌으나 닫히지 않았다.** §2.3의 로컬 관찰에서 `auth_mode: "chatgpt"`이고 `OPENAI_API_KEY`가 `null`인 것이 확인됐으므로, 대상은 **ChatGPT 백엔드 API 쪽일 가능성이 높고** `api.openai.com` 플랫폼 API는 아닐 가능성이 높다. 그러나 정확한 엔드포인트 URL과 요청 형태는 여전히 미확인이다. (소비: M3) — **결정 기록: M3로 이월 확정.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20)이며 조사 결과가 아니다. 근거: `auth_mode: "chatgpt"` 관찰이 범위를 좁혔지만 정확한 URL을 확인해 주는 공개 문서가 없으므로, **실제 요청 관찰 또는 codex CLI 소스 확인**으로 M3 구현 착수 시점에 해소한다
- `[CLARIFICATION-DEFERRED → M3 이후, 사용자 승인 2026-08-20: Copilot 자격 증명 디스크 저장 위치]` — 공개 문서 3곳 모두 미기술. 환경변수 경로(`COPILOT_GITHUB_TOKEN`/`GH_TOKEN`/`GITHUB_TOKEN`)만 확인됐다. 환경변수만으로 충분한지, 아니면 `/login` 산출물을 디스크에서 읽어야 하는지 결정 필요. **§5.1에서 이월된 토큰 교환 절차와 추론 엔드포인트 질문도 이 항목에 함께 묶인다** — `copilot`을 되살리려면 세 가지가 모두 답해져야 한다. `copilot`이 v1 범위 밖이므로 이 항목은 v1의 어느 마일스톤도 막지 않는다. (소비: M3 이후 또는 후속 SPEC) — **결정 기록: M3 이후 / Copilot 그룹 구현이 실제로 착수될 때로 이월 확정.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20). 근거: v1이 이미 `copilot`을 제외했으므로(spec.md §H) **지금 해소해도 소비자가 없다** — 기존 v1 제외 결정 뒤에 묶인 항목이다
- `[CLARIFICATION-DEFERRED → M2, 사용자 승인 2026-08-20: Bedrock Converse ↔ Anthropic messages 필드 대조표]` — Converse가 "통합 형태"라는 것과 tool use를 지원한다는 것은 확인했으나, Anthropic `/v1/messages` 스키마와의 필드 단위 차이는 대조하지 않았다. 구현 전 대조표가 필요하다. (소비: M2) — **결정 기록: M2(Bedrock 어댑터 마일스톤)로 이월 확정.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20). 근거: Converse가 유력 경로라는 것은 이미 확인됐고 남은 것은 필드 단위 대조뿐이므로, 대조표는 착수 전이 아니라 **Bedrock 어댑터 구현 작업의 일부로** 만든다
- `[CLARIFICATION-DEFERRED → M2, 사용자 승인 2026-08-20: LiteLLM 인스턴스별 /v1/messages 활성 여부 판별]` — LiteLLM이 그 엔드포인트를 제공한다는 것은 확인했으나, 사용자가 띄운 특정 인스턴스에서 켜져 있는지 확인하는 방법(헬스체크 경로 등)은 조사하지 않았다. (소비: M2) — **결정 기록: M2로 이월 확정.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20). 근거: 엔드포인트의 **존재는 이미 확인**됐고 남은 것은 인스턴스별 활성 여부 판별뿐이므로 litellm 어댑터 구현 자리에서 정한다

---

## §6. 출처 목록

### §6.1 웹 출처 (전부 WebFetch로 실제 가져와 확인)

1. https://docs.litellm.ai/docs/anthropic_unified — LiteLLM `/v1/messages` 엔드포인트, 스트리밍 지원, 지원 백엔드 목록
2. https://learn.chatgpt.com/docs/auth — Codex CLI 자격 증명 저장 경로 (`developers.openai.com/codex/auth`에서 308 리다이렉트)
3. https://github.com/openai/codex — Codex CLI 저장소 (인증 상세는 없었고 위 2번 문서를 가리킴)
4. https://docs.github.com/en/copilot/how-tos/set-up/install-copilot-cli — Copilot CLI 토큰 환경변수 3종과 우선순위, "Copilot Requests" 권한
5. https://docs.github.com/en/copilot/concepts/agents/about-copilot-cli — `COPILOT_PROVIDER_BASE_URL` (방향이 반대인 설정)
6. https://github.com/github/copilot-cli — Copilot CLI 저장소 README
7. https://docs.aws.amazon.com/bedrock/latest/userguide/conversation-inference.html — Converse / ConverseStream, tool use
8. https://docs.aws.amazon.com/bedrock/latest/userguide/inference-profiles-support.html — cross-region inference profile ID 형태
9. https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/bedrockruntime — Go SDK 연산 목록과 SigV4 처리

접근 실패로 확인하지 못한 URL: `https://docs.github.com/en/copilot/how-tos/set-up/authenticate-copilot-cli` (HTTP 404), `https://docs.github.com/en/copilot/reference/copilot-cli` (HTTP 404).

### §6.2 로컬 관찰 출처 (2026-08-20)

10. **로컬 머신의 `~/.codex/auth.json`** — 파일 권한(`0600`), 최상위 키 목록, `tokens` 하위 키 목록, `auth_mode` 값, `OPENAI_API_KEY`의 null 여부를 직접 조회했다. **키 이름과 `auth_mode` 값만 읽었고 토큰 값은 읽지 않았다** (공식 문서의 "비밀번호처럼 다루라"는 경고 준수). 같은 머신에서 PATH상의 codex 계열 바이너리를 조회해 `codex`는 없고 `opencodex`(`/opt/homebrew/bin/opencodex`)만 있음을 확인했다 — §2.3의 단서가 여기서 나온다.

이 출처는 공식 문서가 아니라 **한 대의 머신에서 관찰한 사실**이다. §1의 등급 표에서 "로컬 관찰"에 해당하며, 일반화하려면 공식 Codex CLI 설치본에 대한 별도 확인이 필요하다.
