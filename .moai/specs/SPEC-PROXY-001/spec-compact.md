# spec-compact.md — SPEC-PROXY-001 (run-phase 축약본)

> 요구사항 + 인수 기준 + 변경 대상 + 제외 항목만. 배경, 기술 접근, 조사 근거는 `spec.md` / `design.md` / `research.md` 참조.

## 요구사항

> **범위 결정 두 가지 (v0.2.0)**: ① 데몬은 **머신(사용자) 범위** — 같은 머신의 여러 프로젝트가 데몬 하나를 공유한다. 그룹 레지스트리도 머신 계층(`~/.moai/config/proxy-groups.yaml`)에 있고, 프로젝트는 `llm.proxy.default_set` 포인터만 갖는다. ② **`copilot` 그룹은 v1 구현 범위에서 제외** — 타입은 스키마에 남고 어댑터만 빠진다.

### 모듈 1 — 데몬 수명주기 (머신 범위)
- **REQ-PROXY-001** When **해당 머신에** 동작 중인 데몬이 없으면, 프록시는 데몬을 기동하고 참조 계수를 1로 초기화해야 한다.
- **REQ-PROXY-002** When **해당 머신에** 동작 중인 데몬이 있으면, 프록시는 중복 기동 없이 연결하고 참조 계수를 1 증가시켜야 한다. **호출한 프로젝트 디렉터리가 달라도 마찬가지다.**
- **REQ-PROXY-003** When 참조 계수가 0에 도달하면, 데몬은 스스로 종료해야 한다. 참조 계수는 머신 전체의 연결 세션 수를 센다.
- **REQ-PROXY-004** 데몬은 등록된 전체 그룹 집합을 상시 보유해야 한다.

### 모듈 2 — 그룹 레지스트리와 세트 선택
- **REQ-PROXY-005** 레지스트리는 `bedrock` / `codex` / `copilot` / `litellm` / `openai-compatible` 다섯 타입을 등록할 수 있어야 한다. **`copilot`은 유효한 타입 값으로 남되 v1에서 어댑터를 구현하지 않으며, 그 타입의 그룹은 그룹 단위 비활성으로 처리해야 한다.**
- **REQ-PROXY-006** 그룹은 사용자 명명 항목이며, 동일 타입 그룹의 공존을 허용해야 한다.
- **REQ-PROXY-007** 각 그룹은 `opus`/`sonnet`/`haiku`/`fable` 별칭 블록과 선택적 모델 목록을 선언할 수 있어야 한다.
- **REQ-PROXY-008** When `-g`가 지정되면, 그 목록을 활성 그룹 집합으로 삼아야 한다.
- **REQ-PROXY-009** When `--set`이 지정되면, 저장된 세트를 활성 그룹 집합으로 삼아야 한다.
- **REQ-PROXY-010** When 둘 다 없으면 기본 세트를 쓰되, 프로젝트 `llm.proxy.default_set`이 있으면 그것을 머신 `sets.default`보다 우선해 해석하고, 어느 계층에도 기본 세트가 없으면 그룹을 나열하고 오류로 종료해야 한다.
- **REQ-PROXY-011** When `-g`와 `--set` 동시 지정이 검출되면, 한쪽을 임의 채택해서는 안 되며 오류로 종료해야 한다.
- **REQ-PROXY-025** When 프로젝트 포인터가 가리킨 세트가 레지스트리에 없는 것이 검출되면, 머신 `sets.default`로 조용히 대체해서는 안 되며 이름을 못 찾았음을 알리고 오류로 종료해야 한다. *(REQ-PROXY-010에서 분리)*

### 모듈 3 — 카탈로그 해석
- **REQ-PROXY-012** 카탈로그 이름은 `<group>/<model>` 형태여야 한다.
- **REQ-PROXY-013** [HARD] 별칭은 `{group, model}` 쌍의 **순서 있는 목록**으로 해석되어야 하며, 순서는 활성 그룹 집합의 순서와 같아야 한다.
- **REQ-PROXY-014** v1 라우팅은 그 목록의 첫 항목을 사용해야 한다.
- **REQ-PROXY-015** When 요청이 `<group>/<model>` 형태를 담으면, 별칭 해석을 우회해 직접 사상해야 한다.

### 모듈 4 — 클라이언트 표면과 백엔드 번역
- **REQ-PROXY-016** 프록시는 Anthropic `/v1/messages` 호환 표면을 SSE 스트리밍 포함해 노출해야 한다.
- **REQ-PROXY-017** Where 타입이 `litellm`이면, 본문을 변환하지 않고 중계해야 한다.
- **REQ-PROXY-018** Where 타입이 `codex`/`copilot`/`openai-compatible`이면, **단일 공유 번역 계층**을 사용해야 하며 타입별 별도 구현을 두어서는 안 된다. v1에서 실제로 소비하는 타입은 둘(`codex`, `openai-compatible`)이지만, **소비자 수와 무관하게 번역기는 하나**여야 한다.
- **REQ-PROXY-019** 번역 계층은 스트리밍 이벤트 열과 도구 호출 블록을 모두 변환해야 한다.
- **REQ-PROXY-020** Where 타입이 `codex`/`copilot`이면, 기존 CLI 자격 증명 저장소를 읽기 전용으로만 써야 하며 자체 인증 흐름을 수행해서는 안 된다.
- **REQ-PROXY-021** When 자격 증명 판독 실패가 검출되면, 해당 그룹만 비활성화하고 나머지 서빙을 계속하며 그룹 이름과 시도 위치를 알려야 한다.

### 모듈 5 — 보안 경계
- **REQ-PROXY-022** 데몬은 루프백에만 바인딩해야 한다.
- **REQ-PROXY-023** 데몬은 권한 상승이 필요 없는 포트에서 수신해야 한다.
- **REQ-PROXY-024** 자격 증명을 **머신 레지스트리를 포함한 모든 설정 파일**(버전 관리 대상 여부 무관)에 기록해서는 안 되며, 참조만 기록해야 한다.

## 인수 기준 (요약 — 전문은 acceptance.md)

- **AC-PROXY-001a/b/c** 데몬 기동 / **서로 다른 프로젝트가 데몬 하나 공유** / 참조 계수 0에서 자기 종료
- **AC-PROXY-002** 요청되지 않은 그룹도 데몬이 보유
- **AC-PROXY-003** 동일 타입 그룹 둘이 덮어쓰기 없이 공존
- **AC-PROXY-004** 별칭이 구체 모델로 사상
- **AC-PROXY-005a~d, 005f** `-g` / `--set` / 기본 세트 사용·부재 시 오류 + **프로젝트 선호가 머신 기본을 이김(005c 가지 3)** / 동시 지정 거절 / **프로젝트 포인터 부재 시 조용한 대체 없이 오류(005f)**
- **AC-PROXY-006** 같은 모델을 서빙하는 두 그룹이 이름 충돌 없음
- **AC-PROXY-007a** 별칭 해석 반환값이 **길이 1일 때도 목록**
- **AC-PROXY-007b** 100회 라우팅이 100회 모두 첫 항목으로
- **AC-PROXY-008** `<group>/<model>` 직접 지정이 별칭을 우회
- **AC-PROXY-009** Claude Code 스트리밍 대화 정상 종료
- **AC-PROXY-010** litellm 경로에서 요청 본문 무변환
- **AC-PROXY-011** v1 범위의 두 타입이 동일 번역 구현 호출, 병렬 구현 부재
- **AC-PROXY-012a/b** 스트리밍 이벤트 열 변환 / 도구 호출 인자 재조립
- **AC-PROXY-013** 인증 흐름 개시 동작 없음
- **AC-PROXY-014** 판독 실패 그룹만 비활성, 나머지 요청 성공
- **AC-PROXY-015a/b** 루프백 바인딩 / 1024 이상 포트
- **AC-PROXY-016** 프로젝트 설정 + 머신 레지스트리 **두 파일 모두** 비밀 값 검색 0건

## 변경 대상

- `internal/cli/proxy.go` (신규) — cobra 서브커맨드, `GroupID: "launch"`
- `internal/proxy/` (신규 패키지) — 데몬, 카탈로그, 번역 계층, 백엔드 어댑터
- **머신 레지스트리 로더 (신규)** — `~/.moai/config/proxy-groups.yaml` 판독. 프로젝트 섹션 5단계 절차 대상이 **아니고**, 템플릿 배포 대상도 아니다. 파일 부재를 정상 상태로 처리하고, `$HOME` 기반 크로스 플랫폼 경로 해석
- `internal/config/types.go` — `LLMConfig`에 `Proxy` 필드 (**`default_set` 포인터 한 필드만**)
- `internal/config/defaults.go` — 기본값
- `internal/config/envkeys.go` — 신규 환경변수 상수
- `internal/template/templates/.moai/config/sections/llm.yaml` — `llm.proxy.default_set`만 (Template-First 원천). **그룹 정의는 여기 들어가지 않는다**

## 제외 항목 (What NOT to Build)

### Out of Scope — 클라이언트 표면 확장
- OpenAI 호환 클라이언트 표면 (`/v1/chat/completions` 등)
- Claude Code 이외 클라이언트를 위한 인증 / 멀티테넌시
- 웹 UI 또는 관리 콘솔

### Out of Scope — 라우팅 지능 (v2 대상)
- 배포 목록에 대한 부하 분산
- 오류 시 장애 전환
- 비용/지연 기반 자동 라우팅
- 응답 캐싱

### Out of Scope — Copilot 백엔드 그룹 구현 (v1 제외)
- `copilot` 그룹의 백엔드 어댑터 구현. v1이 서빙하는 타입은 넷: `bedrock`, `codex`, `litellm`, `openai-compatible`
- Copilot 자격 증명 저장 위치 판독
- GitHub 토큰 → Copilot 토큰 교환
- Copilot 추론 엔드포인트 호출

> 타입 `copilot` 자체는 스키마에 **남는다** (REQ-PROXY-005). 되살릴 때 스키마 변경이 없도록 하기 위해서다.

### Out of Scope — 인증 흐름 구현
- Codex OAuth device-code 흐름 자체 구현
- Copilot 로그인 흐름 자체 구현
- 토큰 갱신 / 만료 처리 / 재인증 UI

### Out of Scope — 어휘 및 기존 명령
- 에이전트 `model:` 5-별칭 어휘 변경
- `moai cc` / `moai glm` 동작 변경
- `llm.glm.*` 구조 변경
