---
id: SPEC-AX-WEB-002
version: 0.1.0
status: completed
created: 2026-05-29
updated: 2026-05-29
author: ircp
priority: high
issue_number: TBD
---

# SPEC-AX-WEB-002 — E2E `fixme` 4건 차단 해소

## HISTORY

- v0.1.0 (2026-05-29): 초안 작성. `apps/web/e2e/flow-admin.spec.ts` 3건 + `apps/web/e2e/flow-analyst.spec.ts` 1건의 `fixme`를 SUT 4파일 + E2E 2파일의 외과적 변경으로 해소. `fetchFailed` 패턴(SPEC-AX-WEB-003에서 동작 검증)을 `reviews` / `evaluation-items` 페이지에 적용. 자세한 근본 원인 분석은 `research.md` 참조.

## 개요

본 SPEC은 `apps/web/e2e/` 슈트에 `test.fixme(...)`로 차단된 4개 시나리오를 GREEN으로 전환한다. 차단 원인은 (a) 두 페이지(`reviews`, `evaluation-items`)가 SPEC-AX-WEB-003에서 도입된 `fetchFailed` 폴백 패턴을 따르지 않는 점과, (b) 4개 E2E 케이스의 selectors/URL/대기 처리 결함이다. 백엔드/파이프라인/BFF/미들웨어/E2E 인프라는 무변경(0-diff)으로 유지한다.

## 목표 (Goals)

- `apps/web/e2e/flow-admin.spec.ts`의 AC-FLOW-ADMIN-001, 002, 003 GREEN.
- `apps/web/e2e/flow-analyst.spec.ts`의 AC-FLOW-ANALYST-001 GREEN.
- `apps/web/src/app/dashboard/reviews/page.tsx` + `apps/web/src/components/review/kanban-board.tsx`에 `fetchFailed` 패턴 적용.
- `apps/web/src/app/dashboard/evaluation-items/page.tsx` + `apps/web/src/components/evaluation/two-panel.tsx`에 `fetchFailed` 패턴 적용.
- 변경 SUT 파일은 정확히 4개, 변경 E2E 파일은 정확히 2개로 한정.

## 비목표 (Non-Goals)

- 새로운 페이지/라우트 신설.
- 백엔드 API/모델 변경.
- 칸반 보드 드래그-앤-드롭 도입.
- WEB-003에서 OPEN-AC로 남긴 backend-live 의존 E2E 7건 해소.
- 점수 채점/리뷰 워크플로우의 비즈니스 규칙 수정.

## EARS 요구사항

### Ubiquitous (항상 성립)

**REQ-WEB-002-001** — `apps/web/src/app/dashboard/reviews/page.tsx`의 RSC 단계 `fetchReviews()` 호출이 실패(`kind === "error"`)할 때, 페이지는 **shall** `KanbanBoard`에 빈 초기 데이터(`{ reviews: [] }`)와 `fetchFailed={true}`를 전달하여 컴포넌트가 마운트되도록 해야 한다. 에러 문단으로 컴포넌트 마운트를 차단해서는 안 된다.

**REQ-WEB-002-002** — `apps/web/src/components/review/kanban-board.tsx`는 **shall** 선택적 prop `fetchFailed?: boolean`을 수용하고, `fetchFailed === true`로 마운트되면 `useEffect`에서 `fetchAll()`(또는 동등한 브라우저 fetch)을 1회 호출하여 `/api/v1/reviews` 응답으로 보드 상태를 채워야 한다.

**REQ-WEB-002-003** — `apps/web/src/app/dashboard/evaluation-items/page.tsx`의 RSC 단계 fetch가 실패(`kind === "error"`)할 때, 페이지는 **shall** `TwoPanel`에 빈 항목 배열(`items: []`)과 `fetchFailed={true}`를 전달하여 컴포넌트가 마운트되도록 해야 한다.

**REQ-WEB-002-004** — `apps/web/src/components/evaluation/two-panel.tsx`는 **shall** 선택적 prop `fetchFailed?: boolean`을 수용하고, `fetchFailed === true`로 마운트되면 `useEffect`에서 `/api/v1/evaluation-items`를 브라우저 fetch로 1회 조회하여 항목 트리를 채우고 첫 루트 항목을 자동 선택해야 한다.

### Event-Driven (이벤트 트리거)

**REQ-WEB-002-005** — **WHEN** `flow-admin.spec.ts` AC-FLOW-ADMIN-001 시나리오가 `/dashboard/rubric`으로 이동하면, the test **shall** (i) 첫 픽스처 행의 텍스트("경영성과")가 가시화될 때까지 대기, (ii) 첫 편집 버튼을 클릭, (iii) 임계값 숫자 입력을 채움, (iv) 저장 버튼을 클릭, (v) 저장 버튼이 여전히 존재함을 단정해야 한다.

**REQ-WEB-002-006** — **WHEN** `flow-admin.spec.ts` AC-FLOW-ADMIN-002 시나리오가 `/dashboard/reviews`로 이동하면, the test **shall** (i) `UNDER_REVIEW` 상태 카드(rv-002에서 유래한 텍스트)가 가시화될 때까지 대기, (ii) 해당 카드의 "승인" 버튼을 클릭, (iii) 승인 후 보드 상태 변화(또는 토스트/네트워크 응답)를 단정해야 한다.

**REQ-WEB-002-007** — **WHEN** `flow-admin.spec.ts` AC-FLOW-ADMIN-003 시나리오가 `/dashboard/reviews`로 이동하면, the test **shall** (i) `SUBMITTED` 상태 카드(rv-001에서 유래한 텍스트)가 가시화될 때까지 대기, (ii) 해당 카드의 "배정" 버튼을 클릭, (iii) 리뷰어 ID 입력 필드를 채움, (iv) 확정 버튼을 클릭해 배정 요청을 트리거해야 한다.

**REQ-WEB-002-008** — **WHEN** `flow-analyst.spec.ts` AC-FLOW-ANALYST-001 시나리오가 실행되면, the test **shall** (i) `/dashboard/evaluation-items`(잘못된 `/dashboard/scores` 아님)로 이동, (ii) `ScoreForm`의 점수 입력(`#score-value`)이 가시화될 때까지 대기, (iii) 숫자 점수를 입력, (iv) 저장 버튼을 클릭, (v) 제출 후 안정 상태(빈 입력 또는 성공 신호)를 단정해야 한다.

### Unwanted (금지)

**REQ-WEB-002-009** — The system **shall not** `apps/web/e2e/`에 `data-testid` 기반 셀렉터를 도입한다. 한국어 텍스트(`getByText` / `getByRole({ name })`) 셀렉터만 사용해야 한다. (SPEC-AX-E2E-001 lesson 유지.)

**REQ-WEB-002-010** — The system **shall not** 다음 경로를 수정한다 (0-diff 영역):
- `internal/...`, `pipelines/...`
- `apps/web/src/lib/...`, `apps/web/src/middleware.ts`, `apps/web/src/app/login/...`
- `apps/web/e2e/fixtures/...`
- `apps/web/playwright.config.ts`, `apps/web/next.config.mjs`, `apps/web/package.json`
- 루트 `package.json`, `go.mod`, `pyproject.toml`

## Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외하는 작업:

- **EXC-001**: 백엔드 실 호출 기반 E2E 라이브 모드. 모든 API는 계속 `page.route()` 모킹으로 처리한다.
- **EXC-002**: 실제 `/dashboard/scores` 경로/페이지 신설. 점수 입력은 `/dashboard/evaluation-items` 내부의 `ScoreForm`을 통해 수행한다.
- **EXC-003**: 칸반 보드의 드래그-앤-드롭, 다중 선택, 일괄 처리 등 신규 인터랙션.
- **EXC-004**: 루브릭 편집 후 서버 측 검증/저장 로직(이미 픽스처로 200을 반환).
- **EXC-005**: 점수 제출 후 워크플로우 자동 전이(예: SUBMITTED → UNDER_REVIEW 자동).
- **EXC-006**: `data-testid` 도입.
- **EXC-007**: SPEC-AX-WEB-003에서 OPEN-AC로 남긴 backend-live 의존 E2E 7건 해소.
- **EXC-008**: `apps/web/e2e/fixtures/` 수정(픽스처/모킹 인프라 0-diff).
- **EXC-009**: Playwright 설정/타임아웃/리트라이 변경.
- **EXC-010**: TypeScript/Next.js 메이저 업그레이드.

## 의존성

- **선행**: SPEC-AX-WEB-001(웹 대시보드 PoC) + SPEC-AX-WEB-003(`next.config.mjs` 전환) merged.
- **블로킹**: 없음.
- **연관**: SPEC-AX-E2E-001(E2E 인프라/스타일 규약 — `data-testid` 금지, 한국어 텍스트 셀렉터 원칙).

## 검증 도구

- `pnpm --filter @iroum-ax/web test:e2e -- flow-admin.spec.ts flow-analyst.spec.ts`
- `pnpm --filter @iroum-ax/web lint` (변경 SUT 4파일 대상)
- `pnpm --filter @iroum-ax/web typecheck`
- `git diff --quiet -- internal/ pipelines/ apps/web/src/lib apps/web/src/middleware.ts apps/web/src/app/login apps/web/e2e/fixtures apps/web/playwright.config.ts apps/web/next.config.mjs apps/web/package.json package.json go.mod pyproject.toml`

## OPEN 결정

본 SPEC 단계에서 OPEN으로 남기는 결정은 없다(연구 단계에서 D1–D4 모두 RESOLVED). Run 단계에서 다음을 의식적으로 모니터링한다:

- **O1 (관찰)**: `KanbanBoard.fetchAll()`의 멱등성 — `fetchFailed=true` 마운트 후 사용자 클릭으로 인한 추가 `fetchAll()` 호출이 race를 유발하는지. 발견 시 별도 SPEC으로 분리.
- **O2 (관찰)**: `TwoPanel`의 첫 루트 자동 선택 — 빈 트리(fetchFailed 직후) 상태에서 자동 선택이 NPE를 유발하는지. 발견 시 별도 SPEC으로 분리.

## Implementation Notes

**구현 완료** (commit `244e47c`, 2026-05-29, branch `main`)

### 변경 파일 (6개)

| 파일 | 변경 유형 | 내용 요약 |
|------|----------|----------|
| `apps/web/src/app/dashboard/reviews/page.tsx` | SUT 수정 | error 분기 제거 → `fetchFailed` prop 전달, 빈 초기 데이터 |
| `apps/web/src/components/review/kanban-board.tsx` | SUT 수정 | `fetchFailed?: boolean` prop 추가, useEffect로 `fetchAll()` 트리거 |
| `apps/web/src/app/dashboard/evaluation-items/page.tsx` | SUT 수정 | error 분기 제거 → `fetchFailed` prop 전달, 빈 items 배열 |
| `apps/web/src/components/evaluation/two-panel.tsx` | SUT 수정 | `fetchFailed?: boolean` prop, 로컬 `items` state, cancel flag 패턴 async useEffect |
| `apps/web/e2e/flow-admin.spec.ts` | E2E 수정 | AC-FLOW-ADMIN-001/002/003 `fixme` 제거 → `expect(...).toBeVisible()` 비동기 대기 |
| `apps/web/e2e/flow-analyst.spec.ts` | E2E 수정 | AC-FLOW-ANALYST-001 URL 수정(`/scores`→`/evaluation-items`), 비동기 대기 추가 |

### 핵심 패턴

- **fetchFailed 패턴**: RSC fetch 실패 시 에러 문단이 아닌 빈 데이터 + `fetchFailed=true`로 컴포넌트를 마운트. 클라이언트 useEffect가 브라우저 fetch 실행 → Playwright `page.route()`가 인터셉트.
- **cancel flag 패턴** (TwoPanel): async useEffect 클린업에서 `cancelled = true` 설정으로 언마운트 후 setState 방지.
- **첫 루트 자동 선택**: `parent_id === null` 조건으로 첫 루트 항목 선택, 없으면 첫 항목으로 폴백.

### 품질 게이트 결과

- TypeScript `noEmit`: ✓ 오류 없음
- ESLint `--max-warnings 0`: ✓ 경고/오류 없음
- Frozen scope 0-diff: ✓ `git diff --quiet` 확인 완료
- REQ-WEB-002-001 ~ REQ-WEB-002-010: 전 항목 구현/준수

### OPEN-AC (잔여)

- `flow-analyst.spec.ts` AC-FLOW-ANALYST-002/003/004 (`test.fixme`): backend-live 의존성으로 본 SPEC 범위 외 (EXC-007 준수).
