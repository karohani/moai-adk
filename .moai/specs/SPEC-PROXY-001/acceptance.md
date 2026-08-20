# acceptance.md — SPEC-PROXY-001 인수 기준

> 각 AC는 관찰 가능하고 이분법적으로 판정 가능해야 한다. 형식은 Given-When-Then. 각 AC는 REQ와 마일스톤으로 추적된다.
> 심각도: MUST-FIX / SHOULD-FIX / NICE-TO-HAVE.

## §D. AC 매트릭스

| AC ID | REQ | 마일스톤 | 심각도 | 시나리오 |
|---|---|---|---|---|
| AC-PROXY-001a | REQ-PROXY-001 | M2 | MUST-FIX | 데몬이 없을 때 기동, 참조 계수 1 |
| AC-PROXY-001b | REQ-PROXY-002 | M2 | MUST-FIX | **서로 다른 프로젝트**가 데몬 하나를 공유 (머신 범위 검증) |
| AC-PROXY-001c | REQ-PROXY-003 | M2 | MUST-FIX | 참조 계수 0에서 자기 종료 |
| AC-PROXY-002 | REQ-PROXY-004 | M2 | MUST-FIX | 데몬이 요청되지 않은 그룹도 보유 |
| AC-PROXY-003 | REQ-PROXY-005, 006 | M1 | MUST-FIX | 동일 타입 그룹 둘이 공존 |
| AC-PROXY-004 | REQ-PROXY-007 | M1 | MUST-FIX | 그룹별 별칭 블록이 구체 모델로 사상 |
| AC-PROXY-005a | REQ-PROXY-008 | M2 | MUST-FIX | `-g`가 활성 그룹 집합을 정함 |
| AC-PROXY-005b | REQ-PROXY-009 | M2 | MUST-FIX | `--set`이 저장된 세트를 해석 |
| AC-PROXY-005c | REQ-PROXY-010 | M2 | MUST-FIX | 플래그 없을 때 기본 세트를 쓰고, 기본 세트가 없으면 나열 후 오류, 프로젝트 선호가 머신 기본을 이김 |
| AC-PROXY-005d | REQ-PROXY-011 | M2 | MUST-FIX | `-g`와 `--set` 동시 지정 시 거절 |
| AC-PROXY-005f | REQ-PROXY-025 | M2 | MUST-FIX | 프로젝트 포인터가 가리킨 세트 부재 시 조용한 대체 없이 오류 |
| AC-PROXY-006 | REQ-PROXY-012 | M1 | MUST-FIX | 카탈로그 이름이 `<group>/<model>` |
| AC-PROXY-007a | REQ-PROXY-013 | M1 | MUST-FIX | 별칭 해석이 순서 있는 목록을 반환 |
| AC-PROXY-007b | REQ-PROXY-014 | M1 | MUST-FIX | v1 라우팅이 목록 첫 항목만 사용 |
| AC-PROXY-008 | REQ-PROXY-015 | M1 | MUST-FIX | `<group>/<model>` 직접 지정이 별칭을 우회 |
| AC-PROXY-009 | REQ-PROXY-016 | M2 | MUST-FIX | Claude Code가 스트리밍으로 정상 대화 |
| AC-PROXY-010 | REQ-PROXY-017 | M2 | MUST-FIX | litellm 경로가 본문을 변환하지 않음 |
| AC-PROXY-011 | REQ-PROXY-018 | M3 | MUST-FIX | v1 범위의 OpenAI 형태 두 타입이 번역기 하나를 공유 (`copilot` 제외) |
| AC-PROXY-012a | REQ-PROXY-019 | M3 | MUST-FIX | 스트리밍 이벤트 열 변환 |
| AC-PROXY-012b | REQ-PROXY-019 | M3 | MUST-FIX | 도구 호출 블록 변환 |
| AC-PROXY-013 | REQ-PROXY-020 | M3 | MUST-FIX | 자체 인증 흐름을 수행하지 않음 |
| AC-PROXY-014 | REQ-PROXY-021 | M3 | MUST-FIX | 판독 실패 그룹만 격리, 나머지 서빙 지속 |
| AC-PROXY-015a | REQ-PROXY-022 | M2 | MUST-FIX | 루프백 전용 바인딩 |
| AC-PROXY-015b | REQ-PROXY-023 | M2 | SHOULD-FIX | 비특권 포트 |
| AC-PROXY-016 | REQ-PROXY-024 | M1 | MUST-FIX | 프로젝트 설정 + 머신 레지스트리 **두 파일 모두**에 자격 증명 없음 |

---

## §D.1 AC 상세 (Given-When-Then)

### AC-PROXY-001a — 데몬 부재 시 기동 (MUST-FIX, M2)
**Given** 해당 **머신에** 동작 중인 `moai proxy` 데몬이 없다.
**When** 사용자가 `moai proxy -g <group>`을 실행한다.
**Then** 데몬 프로세스가 하나 생기고, `~/.moai/proxy/`의 상태 기록에 참조 계수가 1로 기록된다.
**증거**: 실행 전후 데몬 프로세스 수(0 → 1)와 `~/.moai/proxy/` 상태 기록의 참조 계수 값. 프로젝트의 `.moai/state/`에는 프록시 데몬 상태가 기록되지 않음도 함께 확인한다.

### AC-PROXY-001b — 서로 다른 프로젝트가 데몬 하나를 공유 (MUST-FIX, M2)
**Given** 데몬이 이미 하나 동작 중이고 참조 계수가 1이며, 그 데몬을 띄운 세션은 프로젝트 디렉터리 P1에 있다.
**When** **P1이 아닌 다른 프로젝트 디렉터리 P2**에서 두 번째 세션이 `moai proxy`를 실행한다.
**Then** 데몬 프로세스 수는 여전히 1이고, 참조 계수가 2가 된다. P2용 데몬이 추가로 생기지 않는다.
**증거**: 데몬 프로세스 수(1 유지)와 참조 계수(1 → 2), 그리고 두 호출의 작업 디렉터리가 서로 다른 프로젝트였다는 기록.
**이 AC가 검증하는 것**: 데몬 범위가 프로젝트가 아니라 **머신**이라는 결정 (design.md §3.4). 서로 다른 프로젝트에서 부를 때 데몬이 둘이 되면 이 AC는 실패한다 — 같은 프로젝트에서 두 번 부르는 것만으로는 이 결정을 검증하지 못한다.

### AC-PROXY-001c — 참조 계수 0에서 자기 종료 (MUST-FIX, M2)
**Given** 데몬이 동작 중이고 참조 계수가 2다 (서로 다른 프로젝트의 두 세션).
**When** 연결된 두 세션이 모두 종료된다.
**Then** 참조 계수가 0이 되고, 데몬 프로세스가 사라진다. 한 프로젝트의 세션만 종료된 시점에는 데몬이 살아 있어야 한다.
**증거**: 참조 계수(2 → 1 → 0), 계수 1 시점의 데몬 생존, 최종 데몬 프로세스 수 0.

### AC-PROXY-002 — 데몬이 전체 그룹 보유 (MUST-FIX, M2)
**Given** 레지스트리에 그룹 A와 그룹 B가 등록되어 있다.
**When** 사용자가 `moai proxy -g A`만으로 데몬을 기동한다.
**Then** 데몬이 보유한 그룹 목록에 A와 B가 모두 있고, 그 세션의 활성 그룹 집합에는 A만 있다.
**증거**: 데몬이 보유한 그룹 목록과 세션 활성 집합을 각각 조회한 결과.

### AC-PROXY-003 — 동일 타입 그룹 공존 (MUST-FIX, M1)
**Given** 설정에 `type: bedrock`인 그룹 `bedrock-us`와 `bedrock-eu`가 둘 다 등록되어 있다.
**When** 설정을 적재한다.
**Then** 두 그룹이 모두 서로 다른 항목으로 존재하며, 어느 쪽도 다른 쪽을 덮어쓰지 않는다.
**증거**: 적재된 그룹 수 2와 각 그룹의 리전 값이 서로 다름.

### AC-PROXY-004 — 그룹별 별칭 사상 (MUST-FIX, M1)
**Given** 그룹 `gpu-cluster-a`가 `aliases.opus: qwen-3-72b`를 선언한다.
**When** 활성 그룹이 `gpu-cluster-a`인 상태에서 별칭 `opus`를 해석한다.
**Then** 결과 배포의 모델 식별자가 `qwen-3-72b`이고 그룹이 `gpu-cluster-a`다.
**증거**: 해석 결과의 `{group, model}` 값.

### AC-PROXY-005a — `-g`가 활성 집합을 정함 (MUST-FIX, M2)
**Given** 레지스트리에 그룹 A, B, C가 있다.
**When** 사용자가 `moai proxy -g A,B`를 실행한다.
**Then** 세션의 활성 그룹 집합이 정확히 `[A, B]`이고 순서가 보존된다.
**증거**: 활성 그룹 집합 조회 결과.

### AC-PROXY-005b — `--set`이 저장된 세트를 해석 (MUST-FIX, M2)
**Given** 설정에 `sets.cheap: [gpu-cluster-a]`가 있다.
**When** 사용자가 `moai proxy --set cheap`을 실행한다.
**Then** 활성 그룹 집합이 `[gpu-cluster-a]`다.
**증거**: 활성 그룹 집합 조회 결과.

### AC-PROXY-005c — 플래그 없을 때 기본 세트 사용, 없으면 오류, 프로젝트 선호 우선 (MUST-FIX, M2)
**Given (가지 1)** 프로젝트에 `llm.proxy.default_set`이 없고, 머신 레지스트리에 `sets.default`가 있다.
**When** 사용자가 플래그 없이 `moai proxy`를 실행한다.
**Then** 활성 그룹 집합이 머신 `sets.default`의 내용과 같고 종료 코드가 0이다.
**Given (가지 2)** 프로젝트에 `llm.proxy.default_set`도 없고 머신 레지스트리에 `sets.default`도 없다.
**When** 사용자가 플래그 없이 `moai proxy`를 실행한다.
**Then** 사용 가능한 그룹 이름이 나열되고 종료 코드가 0이 아니다.
**Given (가지 3)** 프로젝트 `llm.proxy.default_set: cheap`이 있고, 머신 레지스트리에 `sets.cheap`과 `sets.default`가 **둘 다** 있으며 두 세트의 내용이 서로 다르다.
**When** 사용자가 그 프로젝트에서 플래그 없이 `moai proxy`를 실행한다.
**Then** 활성 그룹 집합이 `sets.cheap`의 내용과 같고 `sets.default`의 내용과는 다르며, 종료 코드가 0이다.
**증거**: 세 가지 각각의 종료 코드, 활성 그룹 집합, 출력. 가지 3은 두 세트의 내용 대조를 함께 남긴다.
**판정 주의 (가지 3)**: 두 세트의 내용이 같으면 이 가지는 우선순위를 구분하지 못한다 — Given은 **내용이 다른** 두 세트를 요구한다.
**분할 안내**: 프로젝트 포인터가 가리킨 세트가 부재할 때의 처리는 AC-PROXY-005f가 맡는다. 2계층 우선순위는 v0.4.0에서 이 AC의 가지 3으로 병합됐다 (REQ-PROXY-010 본문 재통합에 맞춤).

### AC-PROXY-005d — `-g`와 `--set` 상호 배타 (MUST-FIX, M2)
**Given** 유효한 그룹 `A`와 유효한 세트 `cheap`이 있다.
**When** 사용자가 `moai proxy -g A --set cheap`을 실행한다.
**Then** 데몬이 기동되지 않고, 두 지시가 상충한다는 오류가 나오며, 종료 코드가 0이 아니다.
**증거**: 종료 코드, 오류 메시지, 데몬 프로세스 수 0.

### AC-PROXY-005f — 프로젝트 포인터가 가리킨 세트 부재 시 오류 (MUST-FIX, M2)
**Given** 프로젝트 `llm.proxy.default_set: missing`이 가리키는 세트가 머신 레지스트리에 **없고**, 머신 `sets.default`는 있다.
**When** 사용자가 플래그 없이 `moai proxy`를 실행한다.
**Then** 종료 코드가 0이 아니고, 오류가 그 세트 이름(`missing`)을 못 찾았음을 말한다. 활성 그룹 집합이 머신 `sets.default`의 내용으로 채워지지 않는다 — **조용한 대체가 없다.**
**증거**: 종료 코드, 오류 메시지에 포함된 세트 이름, 활성 그룹 집합이 비어 있음(=`sets.default`로 채워지지 않음).

### AC-PROXY-006 — 카탈로그 이름 형태 (MUST-FIX, M1)
**Given** 그룹 `bedrock-us`와 `bedrock-eu`가 같은 모델 식별자를 서빙한다.
**When** 카탈로그를 나열한다.
**Then** 두 항목이 서로 다른 이름(`bedrock-us/<model>`, `bedrock-eu/<model>`)으로 나타나고 충돌하지 않는다.
**증거**: 카탈로그 나열 결과의 항목 수 2와 각 이름.

### AC-PROXY-007a — 별칭이 순서 있는 목록을 반환 (MUST-FIX, M1)
**Given** 활성 그룹 집합이 `[A, B]`이고 A와 B 모두 별칭 `opus`를 선언한다.
**When** 별칭 `opus`를 해석한다.
**Then** 반환값이 길이 2의 목록이고, 첫 항목의 그룹이 A, 둘째가 B다.
**증거**: 반환된 목록의 길이와 각 항목의 그룹 이름. **반환 타입이 목록이라는 사실 자체가 이 AC의 대상이다** — 길이 1인 경우에도 목록이어야 한다.

### AC-PROXY-007b — v1 라우팅이 첫 항목만 사용 (MUST-FIX, M1)
**Given** 별칭 `opus`의 배포 목록이 길이 2이고 첫 항목이 그룹 A다.
**When** `opus`를 지정한 요청을 100회 라우팅한다.
**Then** 100회 모두 그룹 A로 간다. 그룹 B로 간 요청이 0건이다.
**증거**: 그룹별 요청 도달 횟수(A=100, B=0).

### AC-PROXY-008 — 직접 지정이 별칭을 우회 (MUST-FIX, M1)
**Given** 활성 그룹 집합이 `[A, B]`이고 별칭 `opus`의 첫 항목이 A다.
**When** 클라이언트가 모델을 `B/<model>`로 지정해 요청한다.
**Then** 요청이 그룹 B로 간다.
**증거**: 요청이 도달한 그룹 이름.

### AC-PROXY-009 — Claude Code 스트리밍 대화 (MUST-FIX, M2)
**Given** `moai proxy`가 동작 중이고 Claude Code가 그 엔드포인트를 바라본다.
**When** 사용자가 Claude Code에서 응답을 유발하는 프롬프트를 보낸다.
**Then** 응답이 스트리밍으로 점진 표시되고 정상 종료된다. 오류로 끊기지 않는다.
**증거**: 세션 로그와 수신된 스트림 이벤트 열의 정상 종료.

### AC-PROXY-010 — litellm 통과 중계 (MUST-FIX, M2)
**Given** 그룹 타입이 `litellm`이다.
**When** 요청을 그 그룹으로 라우팅한다.
**Then** 백엔드가 수신한 요청 본문이 클라이언트가 보낸 본문과 의미상 동일하다(인증 헤더 제외).
**증거**: 클라이언트 요청 본문과 백엔드 수신 본문의 대조.

### AC-PROXY-011 — 번역기 단일 공유 (MUST-FIX, M3)
**Given** v1 범위에 포함된 OpenAI 형태 그룹 타입 **둘(`codex`, `openai-compatible`)**이 구현되어 있다. `copilot`은 v1 범위에서 제외되었다 (spec.md §H).
**When** 두 타입의 번역 경로를 추적한다.
**Then** 두 타입 모두 동일한 번역 구현을 호출한다. 타입별 병렬 번역 구현이 존재하지 않는다.
**증거**: 번역 진입점의 호출자 목록과, 중복 번역 구현이 없음을 보이는 코드 검색 결과.
**판정 주의**: 소비자가 둘로 줄었다는 사실이 이 AC를 완화하지 않는다. 타입별 번역 구현이 둘 있으면 **소비자 수와 무관하게 실패**다 (REQ-PROXY-018). 나중에 `copilot`이 세 번째 소비자로 붙을 때 번역 계층을 수정해야 한다면 이 AC의 의도를 놓친 것이다.

### AC-PROXY-012a — 스트리밍 이벤트 열 변환 (MUST-FIX, M3)
**Given** OpenAI 형태 백엔드가 여러 개의 델타 청크로 응답을 스트리밍한다.
**When** 프록시가 그 응답을 클라이언트로 중계한다.
**Then** 클라이언트가 받는 이벤트 열이 Anthropic 스트리밍 규격의 시작·본문·종료 구조를 갖추고, 조립된 최종 텍스트가 백엔드 델타를 이어 붙인 것과 같다.
**증거**: 수신 이벤트 열의 이벤트 종류 순서와, 조립 텍스트 대조.

### AC-PROXY-012b — 도구 호출 블록 변환 (MUST-FIX, M3)
**Given** OpenAI 형태 백엔드가 인자가 여러 조각으로 나뉜 도구 호출을 스트리밍한다.
**When** 프록시가 그 응답을 중계한다.
**Then** 클라이언트가 받는 도구 호출 블록의 인자가 파싱 가능한 구조화 객체이고, 그 내용이 백엔드 조각을 이어 붙여 파싱한 것과 같다.
**증거**: 수신 도구 호출 블록의 인자 객체와 백엔드 조각 재조립 결과의 대조.

### AC-PROXY-013 — 자체 인증 흐름 없음 (MUST-FIX, M3)
**Given** `codex` 그룹이 설정되어 있다 (v1 범위에서 `copilot`은 제외되었으므로 v1 판정 대상은 `codex` 하나다).
**When** 프록시가 그 그룹의 자격 증명을 확보한다.
**Then** 프록시가 인증 URL을 열거나 사용자에게 로그인을 요구하는 동작이 발생하지 않고, 기존 CLI의 저장소를 읽는 것으로 끝난다.
**증거**: 자격 증명 확보 경로에 인증 흐름 개시 동작이 없음을 보이는 코드 검색 결과와, 실행 시 브라우저/프롬프트가 뜨지 않음.

### AC-PROXY-014 — 판독 실패 그룹 격리 (MUST-FIX, M3)
**Given** 그룹 A(자격 증명 정상)와 그룹 B(자격 증명 저장소가 없거나 손상)가 등록되어 있다.
**When** 데몬을 기동한다.
**Then** 데몬이 정상 기동하고, 그룹 A로 가는 요청이 성공하며, 그룹 B는 비활성으로 표시되고, 그룹 B의 이름과 판독 시도 위치가 사용자에게 전달된다.
**증거**: 데몬 기동 성공, 그룹 A 요청 성공, 그룹 B 상태 표시, 오류 메시지에 그룹 이름과 경로 포함.

### AC-PROXY-015a — 루프백 전용 바인딩 (MUST-FIX, M2)
**Given** 데몬이 동작 중이다.
**When** 리스닝 소켓의 바인딩 주소를 확인하고, 루프백이 아닌 인터페이스 주소로 접속을 시도한다.
**Then** 바인딩 주소가 루프백이고, 외부 인터페이스로의 접속이 실패한다.
**증거**: 리스닝 소켓 목록의 바인딩 주소와 외부 접속 시도 결과.

### AC-PROXY-015b — 비특권 포트 (SHOULD-FIX, M2)
**Given** 데몬이 동작 중이다.
**When** 리스닝 포트 번호를 확인한다.
**Then** 포트 번호가 1024 이상이고, 기동에 권한 상승이 필요하지 않다.
**증거**: 리스닝 포트 번호와, 비특권 사용자로 기동 성공.

### AC-PROXY-016 — 설정에 자격 증명 없음 (MUST-FIX, M1)
**Given** 다섯 타입의 그룹이 모두 **머신 레지스트리 `~/.moai/config/proxy-groups.yaml`에** 설정된 상태다 (design.md §3.1).
**When** 아래 **두 파일 모두**를 검사한다.
1. 프로젝트의 버전 관리 대상 설정 파일 (`.moai/config/sections/*.yaml`)
2. 머신 레지스트리 `~/.moai/config/proxy-groups.yaml`
**Then** 두 파일 어디에도 토큰, API 키, 비밀 값이 없고, 백엔드 관련 값은 경로·프로필 이름·그룹 이름·base URL 같은 참조뿐이다. 프로젝트 파일에는 `llm.proxy.default_set` 포인터 외에 프록시 그룹 정의가 없다.
**증거**: 두 파일 각각에 대한 비밀 값 검색 결과 0건.
**판정 주의**: 머신 레지스트리가 git 추적 대상이 아니라는 이유로 이 AC의 대상에서 빼면 안 된다 (design.md §6). 자격 증명 사본은 git 유출과 무관하게 원본과 만료 시점이 어긋나는 문제를 만든다.

---

## §D.2 품질 게이트

- 신규 패키지 커버리지 85% 이상
- `go vet ./...` 무경고
- `golangci-lint run` 통과
- `GOOS=windows GOARCH=amd64 go build ./...` 성공
- `make build` 후 템플릿 임베딩 반영 확인
- 템플릿 중립성 검사 통과 (템플릿 파일에 SPEC ID / REQ 토큰 / 내부 날짜 / 커밋 SHA 없음)

---

## §D.3 완료 정의 (Definition of Done)

1. MUST-FIX 심각도의 모든 AC가 통과한다.
2. `plan.md` §B.2의 **이월 확정 항목**(사용자 승인 완료)이 각자의 이월 마일스톤 안에서 해소되었거나, 해소되지 않은 항목에 걸린 범위가 명시적으로 제외되었다. **`copilot` 그룹은 이미 제외로 확정되었다** (spec.md §H) — v1 완료 판정에서 `copilot` 구현 여부는 묻지 않는다.
3. §D.2 품질 게이트를 전부 통과한다.
4. Claude Code가 `moai proxy`를 통해 서로 다른 타입의 그룹 둘 이상을 한 세션에서 사용하는 것이 실제로 확인되었다.
5. `moai cc` / `moai glm`의 기존 동작이 회귀하지 않았다.

---

## §D.4 범위 밖 검증 (이 SPEC에서 확인하지 않음)

- 부하 분산 / 장애 전환 동작 (v2 대상)
- OpenAI 호환 클라이언트 표면
- Claude Code 이외 클라이언트의 호환성
- 다중 사용자 / 멀티테넌시 시나리오
- **`copilot` 그룹의 백엔드 동작** — v1 범위 제외 (spec.md §H). v1에서 검증하는 것은 `copilot` 타입이 열거값으로 수용되고 그룹 단위 비활성으로 처리된다는 것까지이며, 실제 Copilot 백엔드 호출은 검증하지 않는다
