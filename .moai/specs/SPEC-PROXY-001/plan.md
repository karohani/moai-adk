# plan.md — SPEC-PROXY-001 구현 계획

> 순서는 **결정 번복 가능성**을 기준으로 잡았다. 나중에 바꾸기 어려운 결정(설정 스키마, 카탈로그 자료구조)을 앞에 두고, 기계적인 배선을 뒤에 둔다. 시간 예측은 쓰지 않는다 — 우선순위 라벨만 쓴다.

---

## §A. 맥락

`moai proxy`는 이 저장소에 **처음 등장하는 상주 프로세스**다. 기존 `moai cc`(75 LOC) / `moai glm`(1051 LOC)은 설정 파일을 고쳐 쓰고 `claude`로 프로세스를 교체하는 얇은 진입점이고, 뜬 채로 남는 것이 없다. 이 차이가 이 SPEC에서 가장 큰 구조적 신규성이다.

관련 설계 문서: `design.md`. 사실 근거: `research.md`.

---

## §B. 알려진 미해소 항목

최초 작성 시 7건이었고, 그중 **3건이 해소됐다.** 남은 4건은 **구현 착수를 막지 않는다** — 전부 그것을 실제로 필요로 하는 마일스톤 안에서 조사할 항목이다. M1은 어느 항목에도 의존하지 않는다.

> **§B.2 네 항목의 처리 상태 (2026-08-20).** 네 항목 모두 **미해결 질문이 아니라 결정이 내려진 항목**이다. 오케스트레이터가 `AskUserQuestion` 라운드를 돌려 네 항목 각각에 대해 사용자의 명시적 승인을 받았고, 네 건 모두 **"소비 마일스톤으로 이월"** 로 결정됐다(즉시 조사 대안은 선택되지 않았다). 종결 형태는 각 항목의 표식을 `[CLARIFICATION-DEFERRED → <마일스톤>, 사용자 승인 2026-08-20: <주제>]` 형식으로 **전환**하는 것이다 — 전환된 표식 하나가 **무엇이 아직 미확인인지**와 **언제·어디서 해소하기로 승인됐는지**를 함께 담으므로, 미해결 표식을 남긴 채 결정을 옆에 덧붙이는 형태는 더 이상 쓰지 않는다. 주제 문구는 전환 과정에서 그대로 보존되며, 각 행의 결정 기록도 아래에 그대로 남는다.

### §B.1 해소됨 (2026-08-20)

| 항목 | 해소 방식 |
|---|---|
| `데몬 범위 — 프로젝트 단위 vs 사용자 단위` | **사용자 결정: 머신(사용자) 단위.** 설정도 함께 머신 단위로 올려 최초의 모순을 제거했다. design.md §3.1 / §3.4 참조. **M1 차단 해제** |
| `Codex auth.json 필드 스키마` | **로컬 관찰로 확보** (research.md §2.3). 최상위 4필드 + `tokens` 하위 4필드. 단, 한 인스턴스 관찰이고 바이너리가 `opencodex`였다는 단서가 붙는다 — 공식 CLI 검증은 M3 안에서 한다 |
| `Copilot 토큰 교환 절차와 추론 엔드포인트` | **범위 결정으로 해소.** `copilot` 그룹을 v1 구현 범위에서 제외한다 (타입은 스키마에 유지). plan.md가 원래 예고했던 우발 대응을 발동한 것이다. 조사 질문 자체는 §B.2의 Copilot 항목으로 이월 |

### §B.2 마일스톤 이월 확정 항목 (4건 — 사용자 승인 완료, 2026-08-20)

| 항목 | 이월 마일스톤 | 결정 기록 (사용자 승인) + 비고 |
|---|---|---|
| `[CLARIFICATION-DEFERRED → M2, 사용자 승인 2026-08-20: Bedrock 경로 — Converse vs InvokeModel]` — Converse가 통합 형태이고 tool use를 지원하는 것은 확인됐으나, Anthropic 스키마와의 필드 단위 차이를 대조하지 않았다 (design.md §5.4) | M2 (Bedrock 어댑터 마일스톤) | **결정: 이월 확정 — 미해결 질문이 아니다.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20)이며, 조사 결과가 아니라 **해소 시점에 대한 결정**이다. 근거: Converse가 유력 경로라는 것은 이미 확인되었고 남은 것은 필드 단위 대조표뿐이므로, 대조표는 착수 전이 아니라 **Bedrock 어댑터를 실제로 구현하는 자리(M2 7·8번)에서** 만든다. M1은 이 결정에 의존하지 않는다 |
| `[CLARIFICATION-DEFERRED → M2, 사용자 승인 2026-08-20: LiteLLM 인스턴스별 /v1/messages 활성 여부 판별 방법]` | M2 | **결정: 이월 확정 — 미해결 질문이 아니다.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20). 근거: 엔드포인트의 **존재는 이미 확인**됐고 남은 것은 인스턴스별 활성 여부 판별(헬스체크)뿐이므로, litellm 어댑터 구현 자리(M2 5·6번)에서 정한다 |
| `[CLARIFICATION-DEFERRED → M3, 사용자 승인 2026-08-20: Codex 토큰이 인증하는 API 엔드포인트]` | M3 | **결정: 이월 확정 — 미해결 질문이 아니다.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20). 근거: `auth_mode: "chatgpt"` 관찰로 범위는 **좁혀졌으나**(research.md §2.3) 정확한 URL을 확인해 주는 공개 문서가 없다 — 실제 요청 관찰 또는 codex CLI 소스 확인으로 M3 착수 시점에 해소한다 |
| `[CLARIFICATION-DEFERRED → M3 이후, 사용자 승인 2026-08-20: Copilot 자격 증명 디스크 저장 위치]` | M3 이후 / 후속 SPEC (Copilot 그룹 구현이 실제로 착수될 때) | **결정: 이월 확정 — 미해결 질문이 아니다.** 오케스트레이터가 `AskUserQuestion`으로 받은 사용자 승인 결정(2026-08-20). 근거: v1이 이미 `copilot`을 제외했으므로(spec.md §H) **지금 해소해도 소비자가 없다.** 기존 v1 제외 결정 뒤에 묶여 있으며, §B.1에서 이월된 토큰 교환·추론 엔드포인트 질문도 이 항목에 함께 묶인다 |

---

## §C. 저장소 기계적 제약

구현이 반드시 따라야 하는 이 저장소 고유의 규칙들. 어기면 CI 또는 리뷰에서 막힌다.

**설정 확장 (Template-First)**
- 설정 변경의 원천은 `internal/template/templates/.moai/config/sections/llm.yaml`이다. 로컬 `.moai/config/`를 먼저 고치면 안 된다 (`CLAUDE.local.md` §2 Template-First Rule).
- 템플릿 편집 후 `make build`로 임베딩을 재생성한다.
- 템플릿 파일에는 SPEC ID, REQ 토큰, 내부 날짜, 커밋 SHA를 넣지 않는다 (`CLAUDE.local.md` §25 Template Internal-Content Isolation). 이 SPEC의 식별자가 템플릿 YAML 주석에 새면 CI 중립성 검사에서 막힌다.
- 프로젝트 `llm.yaml`에는 **`llm.proxy.default_set` 한 필드만** 추가한다 (design.md §3.1의 2계층 결정). 따라서 `LLMConfig` 구조체 확장은 작지만 여전히 필요하다. **신규 프로젝트 섹션 파일을 만들지 않으므로 `settings-management.md`의 5단계 신규 섹션 절차는 해당되지 않는다.** 다만 구조체-YAML 대칭성 검사(`internal/config/audit_struct_yaml_symmetry_test.go`)는 기존 `LLMConfig`를 이미 다루고 있으므로 확장된 필드가 그 검사를 통과해야 한다.

**머신 레벨 설정 (신규 작업 — 프로젝트 섹션 절차와 다름)**
- 그룹 레지스트리는 `~/.moai/config/proxy-groups.yaml`에 있다 (design.md §3.1). 이건 `.moai/config/sections/` 아래가 **아니므로** 위의 5단계 프로젝트 섹션 절차 대상이 아니고, 템플릿으로 배포되는 파일도 아니다.
- 대신 **이 파일을 읽는 로더가 새로 필요하다.** 프로젝트 설정 로더와 별개 경로이며 M1 작업에 포함된다. 파일이 없을 때(=아직 그룹을 등록하지 않은 사용자)를 정상 상태로 다뤄야 한다 — 없으면 빈 레지스트리로 시작하고, `moai proxy` 호출 시점에 "등록된 그룹이 없다"는 안내로 이어진다. 로더 단계에서 실패시키지 않는다.
- 경로 해석은 `$HOME` 기반이어야 하고 크로스 플랫폼에서 동작해야 한다. init 시점 절대 경로를 굳혀 넣지 않는다 (`CLAUDE.local.md` §14).

**CLI 등록**
- `internal/cli/proxy.go`를 신설하고 `cc.go` / `glm.go` 패턴을 따른다. cobra `GroupID: "launch"`로 등록한다.
- 같은 `Use:` 접두사를 쓰는 서브커맨드를 중복 선언하면 cobra가 런타임 panic한다 (`internal/cli/CLAUDE.md`).
- CLI는 `AskUserQuestion`을 호출하지 않는다. 대화형 프롬프트 대신 위치 인자 + 플래그 기본값 + 구조화된 stderr 오류를 쓴다.
- 종료 코드: 0 성공 / 1 사용자 오류 / 2 시스템 오류. `panic()` 금지.
- stdout은 기계 판독용, stderr는 사람이 읽는 진행/경고/오류. 섞지 않는다.

**환경변수**
- 새 환경변수 이름은 전부 `internal/config/envkeys.go`에 상수로 추가한다. 인라인 `os.Getenv("ANTHROPIC_*")` 문자열은 금지다 (`CLAUDE.local.md` §14).
- 기존 상수 재사용: `EnvAnthropicBaseURL`, `EnvAnthropicAuthToken`, `EnvAnthropicDefaultOpusModel` / `SonnetModel` / `HaikuModel`.

**보안**
- 자격 증명은 git 추적 설정 파일에 쓰지 않는다. 참조만 쓴다 (REQ-PROXY-024).
- 리스닝 소켓은 `127.0.0.1` 전용, 비특권 포트 (REQ-PROXY-022, REQ-PROXY-023).

**크로스 플랫폼**
- 경로 처리는 linux/darwin/windows 모두에서 동작해야 한다. 커밋 전 `GOOS=windows GOARCH=amd64 go build ./...` 확인.
- 사용자 제공 경로는 `filepath.Abs`로 해석한다. `filepath.Join(cwd, userPath)`는 절대 경로 인자를 잘라 내지 않으므로 쓰지 않는다.

**테스트**
- 임시 디렉터리는 `t.TempDir()`만 쓴다.
- 데몬 테스트는 실제 포트를 잡을 수 있으므로 포트 0(자동 할당)을 쓰고 병렬 테스트 간 충돌을 피한다.
- 커버리지 85% 이상. `go vet ./...` + `golangci-lint run` 통과.

---

## §D. 마일스톤

순서는 결정 번복 가능성 순이다. M1이 가장 되돌리기 어렵다.

### M1 — 설정 스키마와 카탈로그 자료구조 (우선순위 High)

**가장 먼저 하는 이유**: 여기서 정한 형태가 나머지 전부를 결정한다. 나중에 바꾸면 마이그레이션이 된다.

**선행 차단 없음.** 데몬 범위 결정이 해소되어(§B.1) M1은 지금 착수 가능하다. §B.2의 남은 4건은 어느 것도 M1이 소비하지 않는다.

1. 머신 레지스트리 `~/.moai/config/proxy-groups.yaml`의 `groups` / `sets` 스키마 확정 (design.md §3.1, §3.2, §3.3)
2. 머신 레지스트리 로더 신규 작성 — 파일 부재를 정상 상태로 처리, `$HOME` 기반 크로스 플랫폼 경로 해석 (§C)
3. 프로젝트 쪽은 템플릿 `llm.yaml`에 `llm.proxy.default_set` 한 필드만 반영 → `make build`. `LLMConfig`에 대응 필드 추가, 기본값, 구조체-YAML 대칭성 확인
4. 플래그 없는 `moai proxy`의 세트 해석 순서 구현: 프로젝트 `default_set` → 머신 `sets.default` → 나열 후 오류 (design.md §3.3). 프로젝트가 가리킨 이름이 레지스트리에 없으면 조용히 넘어가지 않고 오류
5. **카탈로그 해석기: 반환 타입을 처음부터 `[]Deployment` 슬라이스로 만든다** (REQ-PROXY-013). 단일 값으로 만들고 나중에 슬라이스로 바꾸면 호출부가 전부 깨진다
6. v1 라우팅은 그 슬라이스의 인덱스 0을 읽는 **한 줄**로 둔다 (REQ-PROXY-014). v2에서 그 한 줄만 전략 호출로 바뀌어야 한다
7. `<group>/<model>` 직접 지정 해석 (REQ-PROXY-015)
8. 그룹 타입 열거값에 다섯 타입을 모두 넣는다 — `copilot` 포함. **타입은 스키마에 남고 어댑터만 v1에서 빠진다** (spec.md §H). `copilot` 타입이 설정된 그룹을 만나면 "v1에서 지원하지 않는 타입"으로 그 그룹만 비활성 처리하고 나머지는 계속 서빙한다 (REQ-PROXY-021과 같은 격리 형태)

### M2 — 데몬 골격 + litellm + bedrock (우선순위 High)

**이 순서인 이유**: 번역 부담이 가장 얇은 두 백엔드로 데몬 골격을 검증한다. 번역기가 없어도 끝까지 동작하는 경로를 먼저 만들어 두면, M3의 번역기 작업이 "이미 도는 파이프라인에 어댑터를 끼우는 일"이 된다.

1. 데몬 기동 / 연결 / 참조 계수 / 자기 종료 (REQ-PROXY-001~004). 런타임 상태(PID, 포트/소켓, 참조 계수)는 **머신 범위 `~/.moai/proxy/`**에 둔다 (design.md §3.4). `.moai/state/`는 건드리지 않는다
2. 루프백 전용 바인딩, 비특권 포트 (REQ-PROXY-022, 023)
3. Anthropic `/v1/messages` 클라이언트 표면 + SSE (REQ-PROXY-016)
4. `-g` / `--set` / 기본 세트 / 상호 배타 검증 (REQ-PROXY-008~011). 기본 세트의 2계층 우선순위는 REQ-PROXY-010 본문이 정하고, 프로젝트 포인터 미존재 시 오류는 **REQ-PROXY-025**가 정한다 (REQ-PROXY-010에서 분리)
5. **조사: LiteLLM 인스턴스별 `/v1/messages` 활성 여부 판별 방법** (§B.2). 아래 6번의 선행 작업이다
6. `litellm` 그룹: 통과 중계 (REQ-PROXY-017). 번역 없음
7. **조사: Bedrock Converse ↔ Anthropic messages 필드 대조표 작성 → Converse vs InvokeModel 결정** (§B.2, design.md §5.4). 아래 8번의 선행 작업이다
8. `bedrock` 그룹: `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` 사용. SigV4는 SDK가 처리하므로 직접 구현하지 않는다
9. **여러 프로젝트가 데몬 하나를 공유하는 것 확인** — 서로 다른 프로젝트 디렉터리에서 `moai proxy`를 각각 불렀을 때 데몬이 하나만 뜨고 참조 계수가 2가 되는지. 머신 범위 결정의 핵심 관찰 지점이다 (AC-PROXY-001b)
10. `moai proxy`를 통한 Claude Code 실제 대화 end-to-end 확인

### M3 — 공유 OpenAI↔Anthropic 번역 계층 (우선순위 High)

**이 마일스톤이 작업량의 대부분이다.** 번역기는 소비자 수와 무관하게 **하나**다 (REQ-PROXY-018). v1에서 실제로 소비하는 타입은 둘(`codex`, `openai-compatible`)이지만, `copilot`이 나중에 세 번째 소비자로 붙을 때 번역 계층을 손대지 않아야 한다.

1. 비스트리밍 요청/응답 변환
2. **스트리밍 SSE 이벤트 열 재구성** — OpenAI `data: {choices[].delta}` 청크 → Anthropic `message_start` / `content_block_start` / `content_block_delta` / `content_block_stop` / `message_delta` / `message_stop`. 청크 경계가 1:1이 아니므로 상태 기계가 필요하다
3. **tool_use 블록 변환** — OpenAI `tool_calls`(문자열화된 JSON 인자, 스트리밍 중 조각남) → Anthropic `tool_use`(구조화된 `input`). 조각 재조립 필요
4. `openai-compatible` 그룹 (자체 GPU 클러스터). 번역기의 첫 소비자 — 인증이 단순해서 번역 로직만 검증할 수 있다
5. `codex` 그룹 판독기: `~/.codex/auth.json` 또는 OS 자격 증명 저장소 판독. **두 위치를 모두 시도한다** (research.md §2.3). 관찰된 스키마(최상위 `auth_mode`/`OPENAI_API_KEY`/`tokens`/`last_refresh` + `tokens` 하위 4필드)를 전제로 하되, **관대한 파싱**과 **모르는 `auth_mode` 시 그룹 비활성**을 지킨다 (design.md §5.3)
6. **검증: 공식 OpenAI Codex CLI 설치본에 대해 위 스키마 확인.** 관찰 근거가 `opencodex` 인스턴스 하나뿐이므로 이 검증 전까지 스키마를 확정 계약으로 취급하지 않는다 (research.md §2.3 단서)
7. **조사: Codex 토큰이 인증하는 API 엔드포인트** (§B.2). `auth_mode: "chatgpt"` 관찰로 ChatGPT 백엔드 쪽으로 좁혀졌으나 URL은 미확인. 5번 판독기가 얻은 토큰을 실제로 보낼 곳이므로 이 마일스톤 안에서 확정해야 한다
8. `copilot` 그룹: **v1 범위에서 제외** (§B.1, spec.md §H). M3에서 구현하지 않는다. 재개 조건은 §B.2의 Copilot 항목(저장 위치 + 토큰 교환 + 추론 엔드포인트) 세 가지가 모두 답해지는 것이며, 그때 후속 SPEC 또는 M3 이후 작업으로 착수한다. **타입 열거값은 M1에서 이미 들어가 있으므로 되살릴 때 스키마 변경이 필요 없다**
9. 자격 증명 판독 실패의 그룹 단위 격리 (REQ-PROXY-021)

### M4 — CLI 배선과 문서 (우선순위 Medium)

**마지막인 이유**: 전부 기계적이고 되돌리기 쉽다.

1. `internal/cli/proxy.go` cobra 등록 (`GroupID: "launch"`)
2. 새 환경변수 상수를 `envkeys.go`에 추가
3. `--help` 문안, 오류 메시지, 종료 코드 정리
4. 크로스 플랫폼 빌드 확인
5. 사용자 문서

---

## §E. 기술 접근

`design.md`가 SSOT다. 여기서 반복하지 않고 핵심만 가리킨다.

- 프로세스 모델: §2 (싱글턴 + 참조 계수, 데몬은 전체 그룹 보유)
- 설정 위치: §3.1 (`llm.yaml` 안 `llm.proxy.*` — 응집도 근거)
- 카탈로그: §4.2 (별칭 → 배포 목록. **v2가 가산적이기 위한 [HARD] 제약**)
- 번역: §5.2 (OpenAI 형태 셋이 번역기 하나 공유)
- 보안: §6 (루프백, 비특권 포트, 자격 증명 비기록)

---

## §F. 위험과 완화

| 위험 | 완화 |
|---|---|
| ~~Copilot 근거 부족으로 M3에서 막힘~~ → **현실화됨, 처리 완료** | 예고했던 우발 대응을 발동해 v1 범위에서 제외했다 (§B.1). 타입은 스키마에 남으므로 되살리는 비용이 낮다 |
| Codex 자격 증명 포맷이 상대 CLI 변경으로 깨짐 | 판독기를 격리 지점으로. 그룹 단위 비활성 (REQ-PROXY-021) |
| **관찰한 Codex 스키마가 공식 CLI와 다를 수 있음** (근거가 `opencodex` 인스턴스 하나) | M3 6번에서 공식 설치본 검증을 선행 작업으로 명시. 그 전까지 관대한 파싱 + 모르는 `auth_mode` 시 그룹 비활성 (design.md §5.3) |
| **머신 단위 데몬이라 장애 반경이 프로젝트 전체** — 데몬이 죽거나 레지스트리를 잘못 고치면 그 머신의 모든 프로젝트가 영향을 받는다 | 그룹 단위 격리(REQ-PROXY-021)가 그대로 적용되어 한 그룹의 판독 실패는 나머지를 죽이지 않는다. 데몬 자체의 수명주기는 M2에서 얇은 백엔드로 먼저 검증한다 |
| 카탈로그를 단일 배포로 만들어 v2에서 마이그레이션 발생 | M1에서 슬라이스 반환 타입 강제. 이 SPEC의 [HARD] 제약 |
| 첫 상주 프로세스라 수명주기 버그의 선례가 없음 | M2에서 얇은 백엔드로 골격 먼저 검증 |
| 데몬이 여러 자격 증명 보유 | 루프백 전용 바인딩 |

---

## §G. 안티패턴

- 타입별로 번역기를 따로 만드는 것 (REQ-PROXY-018 위반). 버그가 세 벌 생긴다
- 카탈로그 해석기가 단일 배포를 반환하게 만드는 것. v2가 마이그레이션이 된다
- `-g`와 `--set`이 동시에 왔을 때 한쪽을 임의로 채택하는 것 (REQ-PROXY-011 위반)
- Codex / Copilot을 위한 OAuth 흐름을 직접 구현하는 것 (범위 이탈)
- 자격 증명을 설정 파일에 기록하는 것 — 프로젝트 `.moai/config/sections/*.yaml`이든 머신 `~/.moai/config/proxy-groups.yaml`이든 마찬가지다. 머신 레지스트리가 git 추적 대상이 아니라는 이유로 예외를 두면 원본과 사본의 만료 시점이 어긋난다 (design.md §6)
- 그룹 정의를 프로젝트 설정에 두는 것. 데몬이 머신 하나이므로 어느 프로젝트의 레지스트리를 따를지가 임의 규칙이 된다 (design.md §3.1). 프로젝트가 가질 수 있는 것은 세트 이름 **포인터**뿐이다
- `copilot` 타입을 열거값에서 빼는 것. 구현만 빠지고 타입은 남아야 나중에 스키마 변경 없이 되살릴 수 있다 (M1 8번)
- Copilot CLI의 `COPILOT_PROVIDER_BASE_URL`을 우리 문제의 해답으로 착각하는 것 — **방향이 반대다** (research.md §2.4)
- 로컬 `.moai/config/`를 먼저 고치고 템플릿을 나중에 맞추는 것 (Template-First 위반)

---

## §H. 상호 참조

- `design.md` — 구조적 결정과 근거
- `research.md` — 외부 사실 확인 결과와 미확인 항목
- `acceptance.md` — 검증 기준
- `.claude/rules/moai/core/settings-management.md` — 설정 섹션 절차
- `internal/cli/CLAUDE.md` — CLI 모듈 규약
- `CLAUDE.local.md` §2 / §14 / §25 — Template-First, 하드코딩 방지, 템플릿 중립성
