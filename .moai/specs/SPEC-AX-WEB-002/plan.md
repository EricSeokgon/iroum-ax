# Plan: SPEC-AX-WEB-002 — E2E `fixme` 4건 차단 해소

## 접근 방식

`apps/web` 한정 외과적 변경. Go/Python/BFF/미들웨어/E2E 인프라는 0-diff. SPEC-AX-WEB-003에서 검증된 `fetchFailed` 폴백 패턴(`rubric` 페이지가 표준 레퍼런스)을 두 추가 페이지/컴포넌트 쌍에 그대로 복제하고, 그 다음에 E2E 슈트의 4개 `fixme`를 한국어 텍스트 셀렉터와 명시적 대기로 재작성한다.

## 페이즈 분할

세 페이즈로 분할. 각 페이즈는 자기 검증 가능하며, 다음 페이즈로 진입하기 전 lint/typecheck를 통과해야 한다.

---

### Phase A — Reviews 도메인 SUT (Priority: High)

`reviews` 페이지와 칸반 보드에 `fetchFailed` 폴백 패턴을 적용한다.

#### 작업 A1 — `apps/web/src/app/dashboard/reviews/page.tsx`

**현재**:

```tsx
{initial.kind === "error"
  ? <p>...에러 문구...</p>
  : <KanbanBoard initial={initial.data} />}
```

**변경**:

- `rubric/page.tsx`를 정확한 레퍼런스로 사용하여 동일 구조로 재작성.
- `initialData = initial.kind === "ok" ? initial.data : { reviews: [] }` 형태로 폴백 빈 데이터 산출.
- `fetchFailed = initial.kind === "error"` 도출.
- `<KanbanBoard initial={initialData} fetchFailed={fetchFailed} />`로 변경. 에러 분기 문단은 제거하거나 보드 위에 작은 배너로 축소(선택).

**검증**: 페이지 SSR에서 fetch 실패 시 보드 컴포넌트가 마운트되어야 한다(에러 문단으로 마운트 자체가 차단되지 않음).

#### 작업 A2 — `apps/web/src/components/review/kanban-board.tsx`

**현재**: `initial` prop만 수용. 내부에 `fetchAll()` 메서드 존재(승인/배정 후 재조회용).

**변경**:

- Props 인터페이스에 `fetchFailed?: boolean` 추가.
- 컴포넌트 본문에 다음 `useEffect` 추가:

  ```tsx
  React.useEffect(() => {
    if (fetchFailed) {
      void fetchAll();
    }
  }, [fetchFailed]);
  ```

- 기존 `fetchAll()` 시그니처/동작은 보존(승인/배정 핸들러가 이미 호출하므로 회귀 금지).

**검증**: `fetchFailed=true`로 마운트 시 `/api/v1/reviews`가 클라이언트에서 1회 호출되어 보드가 채워진다.

---

### Phase B — Evaluation-Items 도메인 SUT (Priority: High)

`evaluation-items` 페이지와 `TwoPanel` 컴포넌트에 동일 패턴을 적용한다.

#### 작업 B1 — `apps/web/src/app/dashboard/evaluation-items/page.tsx`

**현재**:

```tsx
{initial.kind === "error"
  ? <p>...에러 문구...</p>
  : <TwoPanel items={initial.data.items} />}
```

**변경**:

- `initialItems = initial.kind === "ok" ? initial.data.items : []`.
- `fetchFailed = initial.kind === "error"`.
- `<TwoPanel items={initialItems} fetchFailed={fetchFailed} />`.

**검증**: SSR fetch 실패 시 `TwoPanel`이 마운트된다.

#### 작업 B2 — `apps/web/src/components/evaluation/two-panel.tsx`

**현재**: `items` prop을 RSC로부터 받음. 내부 상태로 선택된 항목 추적. 폴백 fetch 없음.

**변경**:

- Props에 `fetchFailed?: boolean` 추가.
- 컴포넌트 내부 상태(`items` 동기화용)를 `React.useState<EvaluationItem[]>(props.items)`로 보유(이미 그러하다면 보존).
- `useEffect` 추가:

  ```tsx
  React.useEffect(() => {
    if (!fetchFailed) return;
    let cancelled = false;
    void (async () => {
      const res = await fetch("/api/v1/evaluation-items");
      if (!res.ok || cancelled) return;
      const json = await res.json();
      if (cancelled) return;
      setItems(json.items ?? []);
      // 첫 루트 자동 선택 로직이 이미 있다면 새 items 기준으로 재실행되어야 함.
    })();
    return () => { cancelled = true; };
  }, [fetchFailed]);
  ```

- 첫 루트 자동 선택 로직이 마운트 1회만 실행되도록 되어 있다면, items 변경 시 빈 선택을 갱신하는 부수 효과를 안전하게 처리(이미 있는 useEffect 활용 권장).

**검증**: `fetchFailed=true` 마운트 시 `/api/v1/evaluation-items`가 클라이언트에서 호출되고, 응답 도착 후 첫 루트가 선택되어 우측 패널에 `ScoreForm`이 표시된다.

---

### Phase C — E2E `fixme` 해소 (Priority: High)

A/B 완료 후 진행. SUT 변경이 없으면 C2/C3의 fetchFailed 폴백이 동작하지 않으므로 순서 엄수.

#### 작업 C1 — `apps/web/e2e/flow-admin.spec.ts` AC-FLOW-ADMIN-001 (루브릭 편집)

`test.fixme(...)` 제거 후 본문 재작성:

1. `await adminPage.goto("/dashboard/rubric")`.
2. `await expect(adminPage.getByText("경영성과").first()).toBeVisible()` — 픽스처 데이터 가시화 대기.
3. 첫 행의 편집 버튼 클릭: `await adminPage.getByRole("button", { name: "편집" }).first().click()`.
4. 임계값 숫자 입력 채우기: `getByRole("spinbutton")` 또는 `locator('input[type="number"]')`로 첫 입력에 값 입력.
5. 저장 버튼 클릭: `await adminPage.getByRole("button", { name: "저장" }).click()`.
6. 저장 버튼이 계속 보이는지(또는 모달이 닫혔는지) 단정.

#### 작업 C2 — `apps/web/e2e/flow-admin.spec.ts` AC-FLOW-ADMIN-002 (리뷰 승인)

1. `await adminPage.goto("/dashboard/reviews")`.
2. `UNDER_REVIEW` 카드 가시화 대기 — `rv-002`의 텍스트(예: 리뷰 ID 또는 상태 라벨)가 보일 때까지 `expect(...).toBeVisible()`.
3. 해당 카드 컨텍스트에서 승인 버튼 클릭: `getByRole("button", { name: "승인" })`.
4. 승인 후 상태 변화 단정: 카드가 다른 컬럼으로 이동하거나 토스트 표시(픽스처 응답에 따름).

#### 작업 C3 — `apps/web/e2e/flow-admin.spec.ts` AC-FLOW-ADMIN-003 (리뷰어 배정)

1. `await adminPage.goto("/dashboard/reviews")`.
2. `SUBMITTED` 카드(`rv-001`) 가시화 대기.
3. 해당 카드의 "배정" 버튼 클릭.
4. 리뷰어 ID 입력 필드 채움: `getByRole("textbox", { name: ... })` 또는 placeholder 텍스트로 락온.
5. 확정 버튼(예: "배정 확정") 클릭.
6. 입력 필드가 채워졌음 / 확정 버튼이 트리거되었음을 단정.

#### 작업 C4 — `apps/web/e2e/flow-analyst.spec.ts` AC-FLOW-ANALYST-001 (점수 제출)

1. URL 교정: `await analystPage.goto("/dashboard/evaluation-items")` (`/dashboard/scores` 제거).
2. `TwoPanel` 마운트 + 폴백 fetch 완료 대기: `await expect(analystPage.getByText("경영목표 달성도").first()).toBeVisible()` (픽스처 ei-001 텍스트).
3. ScoreForm 점수 입력 가시화 대기: `await expect(analystPage.locator("#score-value")).toBeVisible()`.
4. 점수 입력: `await analystPage.locator("#score-value").fill("85")`.
5. 저장 버튼 클릭: `await analystPage.getByRole("button", { name: "저장" }).click()`.
6. 안정 상태 단정: 입력 비워짐 또는 성공 텍스트 노출(픽스처 응답 기준).

---

## 변경 파일 매트릭스

| 파일 | 종류 | 추정 LOC 변화 | Phase |
|---|---|---|---|
| `apps/web/src/app/dashboard/reviews/page.tsx` | SUT | +5 / -3 | A1 |
| `apps/web/src/components/review/kanban-board.tsx` | SUT | +8 / -0 | A2 |
| `apps/web/src/app/dashboard/evaluation-items/page.tsx` | SUT | +5 / -3 | B1 |
| `apps/web/src/components/evaluation/two-panel.tsx` | SUT | +15 / -0 | B2 |
| `apps/web/e2e/flow-admin.spec.ts` | E2E | ±40 (fixme 3개 본문 재작성) | C1–C3 |
| `apps/web/e2e/flow-analyst.spec.ts` | E2E | ±15 (fixme 1개 본문 재작성) | C4 |

**합계**: 6파일. 추정 ±90 LOC.

## 마일스톤

- **M1 (Phase A 완료)**: `reviews` 도메인 SUT 변경 후 `lint` + `typecheck` GREEN. AC-FLOW-ADMIN-002/003 수동 검증 가능 상태.
- **M2 (Phase B 완료)**: `evaluation-items` 도메인 SUT 변경 후 `lint` + `typecheck` GREEN. AC-FLOW-ANALYST-001 수동 검증 가능 상태.
- **M3 (Phase C 완료)**: 4건 `fixme` 제거. `pnpm test:e2e -- flow-admin.spec.ts flow-analyst.spec.ts` 4개 시나리오 GREEN.
- **M4 (Frozen 검증)**: `git diff --quiet` 명령으로 Frozen 영역 0-diff 확인. SPEC 완료.

## 기술 접근

- **레퍼런스 패턴**: `apps/web/src/app/dashboard/rubric/page.tsx` + `apps/web/src/components/rubric/rubric-threshold-table.tsx`(가정명)의 `fetchFailed` 폴백을 정확히 동일한 구조로 복제.
- **셀렉터 전략**: SPEC-AX-E2E-001 lesson 준수 — `data-testid` 금지, 한국어 텍스트(`getByText`, `getByRole({ name: "한국어" })`, placeholder)만 사용.
- **대기 전략**: Playwright `expect(...).toBeVisible()` 자동 대기. 임의 `waitForTimeout` 금지.
- **폴백 fetch 멱등성**: `useEffect` 의존성 배열에 `fetchFailed`만 두고, `fetchFailed`가 한 번 `true`로 마운트되면 컴포넌트 lifetime 동안 1회만 실행되도록 보장.

## 검증 (Validation)

각 Phase 종료 시:

```bash
pnpm --filter @iroum-ax/web lint
pnpm --filter @iroum-ax/web typecheck
pnpm --filter @iroum-ax/web test:e2e -- flow-admin.spec.ts flow-analyst.spec.ts
```

최종(Phase C 종료 후):

```bash
git diff --quiet -- \
  internal/ pipelines/ \
  apps/web/src/lib apps/web/src/middleware.ts apps/web/src/app/login \
  apps/web/e2e/fixtures apps/web/playwright.config.ts apps/web/next.config.mjs \
  apps/web/package.json package.json go.mod pyproject.toml
# exit code 0 이어야 함.
```

## 리스크

| 리스크 | 영향 | 완화 |
|---|---|---|
| `KanbanBoard.fetchAll()` 멱등성 결함으로 중복 호출 시 race 발생 | 보드 데이터 깜빡임 | useEffect 의존성 최소화, cancel flag 사용 |
| `TwoPanel`의 첫 루트 자동 선택 로직이 빈 트리에서 NPE | 컴포넌트 충돌 | 폴백 fetch 응답 도착 전에는 선택 미수행, optional chaining 유지 |
| 한국어 텍스트 셀렉터의 다중 매칭 → 잘못된 요소 클릭 | E2E flaky | `.first()`, scope 좁히기(부모 카드 컨테이너 내부 검색), 명시적 `getByRole` 활용 |
| `rubric` 페이지가 실제로는 다른 컴포넌트 구조를 가질 가능성 | A/B 패턴 복제 오류 | Phase A 진입 직전 `rubric/page.tsx`와 자식 컴포넌트를 직접 Read하여 정확한 시그니처 확인 |
| 픽스처 `reviews.json`에 `UNDER_REVIEW` 카드 텍스트가 모호하여 셀렉터 충돌 | C2 flaky | 셀렉터로 리뷰 ID(`rv-002`) 또는 회사명 같은 고유 텍스트 사용 |

## 회귀 검증

- 기존 GREEN E2E(WEB-003에서 풀린 9건): `pnpm test:e2e`로 회귀 없음 확인.
- 칸반 보드의 기존 승인/배정 후 재조회 흐름(`fetchAll()` 호출): 변경 없음. Phase A2의 `useEffect`는 새 폴백 경로일 뿐, 기존 호출자(승인 핸들러, 배정 핸들러)는 수정 금지.
- `TwoPanel`의 RSC 정상 경로(`fetchFailed=false`): 기존과 동일 동작 보장.
