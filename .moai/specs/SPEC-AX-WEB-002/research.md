# Research: SPEC-AX-WEB-002 — E2E `fixme` 차단 해소

## 배경

SPEC-AX-WEB-001(웹 대시보드) + SPEC-AX-WEB-003(`next.config.ts`→`.mjs` 전환)로 E2E 9건이 GREEN으로 풀린 이후, Playwright 슈트에는 의도적으로 `test.fixme(...)` 처리된 차단 시나리오가 4건 잔존한다. 본 SPEC은 그 4건을 정확히 무엇이 막고 있는지 근본 원인을 추적하고, 최소 외과적 SUT + 테스트 변경으로 `fixme`를 제거하는 것을 목표로 한다.

차단된 4건:

- `apps/web/e2e/flow-admin.spec.ts` → AC-FLOW-ADMIN-001 (루브릭 임계값 편집)
- `apps/web/e2e/flow-admin.spec.ts` → AC-FLOW-ADMIN-002 (리뷰 승인)
- `apps/web/e2e/flow-admin.spec.ts` → AC-FLOW-ADMIN-003 (리뷰어 배정)
- `apps/web/e2e/flow-analyst.spec.ts` → AC-FLOW-ANALYST-001 (점수 제출)

## 근본 원인 분류

세 가지 독립 결함이 4건의 `fixme`를 만들고 있다. 각 결함은 서로 다른 페이지 계층에서 발생하지만 공통적으로 **RSC fetch 실패 후 클라이언트 폴백 부재**라는 동일한 패턴이다.

### Root Cause A — 루브릭 임계값 편집 (AC-FLOW-ADMIN-001)

**위치**: `apps/web/e2e/flow-admin.spec.ts` L50–99

**관련 SUT**: `apps/web/src/app/dashboard/rubric/page.tsx`

**현황**:
- `rubric/page.tsx`는 이미 `fetchFailed` 패턴을 올바르게 사용함. RSC에서 thresholds fetch 실패 시 `fetchFailed=true`를 자식 `RubricThresholdTable`에 전달하고, 자식이 `useEffect`로 브라우저 fetch 폴백을 수행한다. E2E에서는 `page.route()`가 이 브라우저 fetch를 인터셉트하여 픽스처 데이터를 주입한다.
- 따라서 SUT는 정상이고, 데이터는 페이지에 결국 로드된다.

**테스트 버그**:
- 테스트는 페이지 진입 직후 곧바로 편집 버튼을 클릭한다. 그러나 데이터가 브라우저 fetch 폴백으로 들어오는 동안 편집 버튼은 아직 렌더되지 않는다.
- 첫 행의 셀렉터가 명확하지 않고, 어떤 행을 편집할지 의도가 불분명하다.

**수정 방향**:
- SUT 변경 없음.
- 테스트 변경: (1) `await expect(adminPage.getByText("경영성과").first()).toBeVisible()`로 픽스처 데이터가 화면에 보일 때까지 대기, (2) 첫 편집 버튼 클릭, (3) 숫자 입력 채우기, (4) 저장 버튼 클릭, (5) 저장 버튼 존재로 안정 상태 확인.

### Root Cause B — 리뷰 승인 / 리뷰어 배정 (AC-FLOW-ADMIN-002, 003)

**위치**: `apps/web/e2e/flow-admin.spec.ts` L102–178

**관련 SUT**:
- `apps/web/src/app/dashboard/reviews/page.tsx`
- `apps/web/src/components/review/kanban-board.tsx`

**현황 — 핵심 패턴 격차**:

`reviews/page.tsx`는 `fetchFailed` 패턴을 사용하지 않는다.

```tsx
{initial.kind === "error"
  ? <p>...error...</p>
  : <KanbanBoard initial={initial.data} />}
```

E2E 환경에서는 백엔드가 실제로 떠 있지 않으므로 RSC fetch가 실패 → 에러 문단이 표시되고 `KanbanBoard`는 마운트조차 되지 않는다. 따라서 `page.route()`로 `/api/v1/reviews`를 모킹해도 그것을 호출할 컴포넌트가 존재하지 않는다.

`kanban-board.tsx`는 내부적으로 `fetchAll()` 메서드(승인/배정 후 재조회용)는 가지고 있으나, `fetchFailed` prop 또는 초기 마운트 시 fetch를 트리거하는 `useEffect`가 없다.

**픽스처 데이터**: `apps/web/e2e/fixtures/api/reviews.json`에는 `rv-001`(`SUBMITTED`)과 `rv-002`(`UNDER_REVIEW`)가 존재한다. 컴포넌트 로직:
- `showAssign = isAdmin && review.status === "SUBMITTED"` → `rv-001`에 배정 버튼
- `showApproveReject = isAdmin && review.status === "UNDER_REVIEW"` → `rv-002`에 승인 버튼

따라서 보드가 마운트만 되면 양쪽 버튼이 모두 표시 가능하다.

**테스트 버그(부수적)**:
- 테스트는 카드가 보드에 렌더되기 전에 곧바로 승인/배정 버튼을 찾는다. 대기 없음.

**수정 방향**:
- SUT 변경: `reviews/page.tsx`에 `fetchFailed` 패턴 도입; `kanban-board.tsx`에 `fetchFailed?: boolean` prop + 초기 fetch `useEffect` 추가.
- 테스트 변경: 칸반 카드(예: 리뷰 ID 또는 상태 라벨)가 보일 때까지 대기 후 버튼 단정.

### Root Cause C — 점수 제출 (AC-FLOW-ANALYST-001)

**위치**: `apps/web/e2e/flow-analyst.spec.ts` L34–81

**관련 SUT**:
- `apps/web/src/app/dashboard/evaluation-items/page.tsx`
- `apps/web/src/components/evaluation/two-panel.tsx`

**현황 — 두 가지 결함 누적**:

(1) 테스트 URL이 잘못되었다. 테스트는 `/dashboard/scores`로 이동하지만, `ScoreForm`이 마운트되는 실제 경로는 `/dashboard/evaluation-items`이다.

(2) `evaluation-items/page.tsx`는 `fetchFailed` 패턴을 사용하지 않는다.

```tsx
{initial.kind === "error"
  ? <p>...error...</p>
  : <TwoPanel items={initial.data.items} />}
```

E2E에서 RSC fetch 실패 시 `TwoPanel`이 마운트되지 않고, 따라서 그 내부의 `ScoreForm`도 표시되지 않는다.

`two-panel.tsx`는 `items` prop을 RSC로부터 받고, `fetchFailed` prop이나 브라우저 fetch 폴백을 보유하지 않는다.

**픽스처 데이터**: `apps/web/e2e/fixtures/api/evaluation-items.json`에 `ei-001` "경영목표 달성도"(parent_id=null)가 존재. `TwoPanel`은 첫 루트 항목을 자동 선택하므로 analyst 역할에서는 곧바로 `ScoreForm`이 렌더된다.

**ScoreForm 요소**:
- 점수 입력: `id="score-value"`, `type="number"`
- 제출 버튼: `<Button type="submit" disabled={submitting}>저장</Button>` — 제출 중이 아니면 활성.

**수정 방향**:
- SUT 변경: `evaluation-items/page.tsx`에 `fetchFailed` 패턴 도입; `two-panel.tsx`에 `fetchFailed?: boolean` prop + 초기 fetch `useEffect` 추가(브라우저 fetch로 evaluation-items 재조회).
- 테스트 변경: URL을 `/dashboard/evaluation-items`로 변경; `ScoreForm` 가시화 대기; 점수 입력; 저장 클릭; 성공 토스트 또는 빈 입력 상태로 확인.

## 패턴 격차 요약

| 페이지 | `fetchFailed` 패턴 | 컴포넌트 폴백 fetch | 상태 |
|---|---|---|---|
| `rubric/page.tsx` | ✅ 사용 중 | ✅ `RubricThresholdTable.useEffect` | 정상 (테스트 버그만 존재) |
| `reviews/page.tsx` | ❌ 부재 | ❌ `KanbanBoard`에 트리거 없음 | SUT 수정 필요 |
| `evaluation-items/page.tsx` | ❌ 부재 | ❌ `TwoPanel`에 트리거 없음 | SUT 수정 필요 |

## 참조 패턴 (Working Example: rubric)

**page.tsx**:

```tsx
const initial = await fetchThresholds();
const initialData = initial.kind === "ok" ? initial.data : { thresholds: [] };
const fetchFailed = initial.kind === "error";
return <RubricThresholdTable initialData={initialData} fetchFailed={fetchFailed} />;
```

**component**:

```tsx
interface Props {
  initialData: ...;
  fetchFailed?: boolean;
}

React.useEffect(() => {
  if (fetchFailed) { void fetchAll(); }
}, [fetchFailed]);
```

## 영향 범위 (Frozen 영역)

본 SPEC은 다음 영역에 **0-diff**를 유지한다:

- `internal/...` (Go 백엔드 전반)
- `pipelines/...` (Python AI 파이프라인)
- `apps/web/src/lib/`, `apps/web/src/middleware.ts`, `apps/web/src/app/login/...` (BFF, 미들웨어, 인증 흐름)
- `apps/web/e2e/fixtures/**` (픽스처/모킹 인프라)
- `apps/web/playwright.config.ts`, `apps/web/next.config.mjs`, `apps/web/package.json` (E2E 인프라 설정)
- 루트 `package.json`, `go.mod`, `pyproject.toml`

변경 대상은 정확히 6개 파일:

| # | 경로 | 종류 |
|---|---|---|
| 1 | `apps/web/src/app/dashboard/reviews/page.tsx` | SUT |
| 2 | `apps/web/src/components/review/kanban-board.tsx` | SUT |
| 3 | `apps/web/src/app/dashboard/evaluation-items/page.tsx` | SUT |
| 4 | `apps/web/src/components/evaluation/two-panel.tsx` | SUT |
| 5 | `apps/web/e2e/flow-admin.spec.ts` | E2E |
| 6 | `apps/web/e2e/flow-analyst.spec.ts` | E2E |

## 의사결정 결과

- **D1 [RESOLVED]**: `reviews/page.tsx`와 `evaluation-items/page.tsx`를 `rubric/page.tsx`와 동일한 `fetchFailed` 패턴으로 통일한다. (대안: 페이지마다 다른 폴백 전략 사용 → 일관성 손상이므로 기각)
- **D2 [RESOLVED]**: `KanbanBoard`와 `TwoPanel`에는 `fetchFailed` prop + `useEffect` 폴백 fetch를 추가한다. (대안: 부모에서 `useEffect` 처리 → 컴포넌트 응집도 손상이므로 기각)
- **D3 [RESOLVED]**: `flow-analyst.spec.ts`의 잘못된 URL `/dashboard/scores`를 실제 라우트 `/dashboard/evaluation-items`로 교정. 새 페이지/라우트는 만들지 않는다.
- **D4 [RESOLVED]**: 테스트의 클릭/단정 전에 한국어 텍스트(픽스처에서 유래) 가시화로 대기를 명시한다. `data-testid` 미사용 원칙(SPEC-AX-E2E-001 lesson)을 유지한다.

## 비목표 (Out of Scope)

- 백엔드 실 호출이 가능해지는 라이브 모드 E2E (모킹 전략 유지).
- 새로운 페이지/라우트(예: 실제 `/dashboard/scores`) 신설.
- 칸반 보드의 드래그-앤-드롭 상호작용 검증.
- 루브릭 편집 후 서버 측 검증 로직 수정.
- WEB-003에서 OPEN-AC로 남긴 backend-live 의존 E2E 7건의 해소.
