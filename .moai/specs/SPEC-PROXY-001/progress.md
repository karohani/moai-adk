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

_<pending run-phase>_

## §E.3 Run-phase Audit-Ready Signal

_<pending run-phase>_

## §E.4 Sync-phase Audit-Ready Signal

_<pending sync-phase>_
