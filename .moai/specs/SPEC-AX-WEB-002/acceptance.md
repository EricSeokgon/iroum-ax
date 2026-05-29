# Acceptance: SPEC-AX-WEB-002 — E2E `fixme` 4건 차단 해소

본 문서는 SPEC-AX-WEB-002의 검증 기준을 Given/When/Then 형식으로 정의한다. 모든 시나리오는 `pnpm --filter @iroum-ax/web test:e2e -- flow-admin.spec.ts flow-analyst.spec.ts`로 실행되며, 모든 케이스가 GREEN이어야 SPEC이 완료된 것으로 간주한다.

## Definition of Done

- [ ] AC-FLOW-ADMIN-001, AC-FLOW-ADMIN-002, AC-FLOW-ADMIN-003, AC-FLOW-ANALYST-001의 `test.fixme(...)`가 모두 제거되었다.
- [ ] 위 4개 E2E 시나리오가 GREEN으로 통과한다.
- [ ] `apps/web` 변경 SUT 4파일은 `pnpm lint` 및 `pnpm typecheck` GREEN.
- [ ] WEB-003에서 이미 GREEN이었던 9개 E2E 시나리오에 회귀가 없다.
- [ ] `internal/`, `pipelines/`, `apps/web/src/lib`, `apps/web/src/middleware.ts`, `apps/web/src/app/login`, `apps/web/e2e/fixtures`, `apps/web/playwright.config.ts`, `apps/web/next.config.mjs`, `apps/web/package.json`, 루트 `package.json`, `go.mod`, `pyproject.toml`이 0-diff.
- [ ] `data-testid` 신규 도입 0건.

---

## AC-FLOW-ADMIN-001 — 루브릭 임계값 편집 (REQ-WEB-002-005)

**Given** admin 역할로 로그인한 관리자가 있고, `apps/web/e2e/fixtures/api/rubric-thresholds.json` 픽스처에 "경영성과"를 포함한 임계값 행들이 모킹되어 있다.

**When** 관리자가 `/dashboard/rubric`으로 이동하고, 첫 픽스처 행 ("경영성과")이 가시화된 다음, 첫 행의 편집 버튼을 클릭하여 임계값 숫자 입력에 새 값을 채우고 저장 버튼을 클릭한다.

**Then**:

- 편집 버튼 클릭 시점에 `RubricThresholdTable`이 마운트되어 있고, 픽스처 데이터가 화면에 반영되어 있다.
- 숫자 입력은 사용자가 입력한 값으로 채워진다.
- 저장 버튼 클릭 후, 저장 버튼이 여전히 DOM에 존재한다(모달이 닫혔거나 인라인 편집이 해제됨을 의미).
- 테스트는 예외 없이 통과한다.

**계측 셀렉터**:

- `adminPage.getByText("경영성과").first()` — 가시화 대기.
- `adminPage.getByRole("button", { name: "편집" }).first()` — 첫 편집 버튼.
- `adminPage.locator('input[type="number"]').first()` — 임계값 입력.
- `adminPage.getByRole("button", { name: "저장" })` — 저장 버튼.

---

## AC-FLOW-ADMIN-002 — 리뷰 승인 (REQ-WEB-002-006, REQ-WEB-002-001, REQ-WEB-002-002)

**Given** admin 역할로 로그인했고, `apps/web/e2e/fixtures/api/reviews.json`에 `rv-002`(상태 `UNDER_REVIEW`)가 모킹되어 있으며, `reviews/page.tsx`는 RSC fetch 실패 시 `fetchFailed=true`를 전달하고 `KanbanBoard`는 마운트 시 폴백 fetch를 수행한다.

**When** 관리자가 `/dashboard/reviews`로 이동하고, `UNDER_REVIEW` 상태의 카드(`rv-002`로부터 유래한 고유 텍스트, 예: 리뷰 ID 또는 회사명)가 보일 때까지 대기한 다음, 해당 카드의 "승인" 버튼을 클릭한다.

**Then**:

- `KanbanBoard`가 마운트되고, 폴백 fetch로 `/api/v1/reviews`가 호출되어 모킹된 reviews 데이터로 보드가 채워진다.
- 칸반 보드에 `rv-002` 카드가 표시되고 그 카드에 "승인" 버튼이 보인다.
- 승인 버튼 클릭 시 `POST` 또는 `PATCH`로 승인 API가 호출되고, 응답에 따라 보드 상태가 갱신되거나 성공 신호(토스트 등)가 표시된다.
- 테스트는 예외 없이 통과한다.

**계측 셀렉터**:

- `adminPage.getByText(/rv-002|UNDER_REVIEW.*고유텍스트/)` — `UNDER_REVIEW` 카드 가시화 대기.
- `adminPage.getByRole("button", { name: "승인" })` — 승인 버튼.

---

## AC-FLOW-ADMIN-003 — 리뷰어 배정 (REQ-WEB-002-007, REQ-WEB-002-001, REQ-WEB-002-002)

**Given** admin 역할로 로그인했고, `apps/web/e2e/fixtures/api/reviews.json`에 `rv-001`(상태 `SUBMITTED`)이 모킹되어 있으며, `reviews/page.tsx` + `KanbanBoard`에 `fetchFailed` 폴백이 적용되어 있다.

**When** 관리자가 `/dashboard/reviews`로 이동하고, `SUBMITTED` 상태 카드(`rv-001`로부터 유래한 고유 텍스트)가 보일 때까지 대기한 다음, 해당 카드의 "배정" 버튼을 클릭하고, 리뷰어 ID 입력 필드를 채우고, 확정 버튼을 클릭한다.

**Then**:

- 보드에 `rv-001` 카드가 표시되고, `showAssign` 조건(`isAdmin && status === "SUBMITTED"`)에 따라 "배정" 버튼이 보인다.
- 배정 버튼 클릭 시 리뷰어 ID 입력 필드가 활성/표시되고, 사용자가 입력한 값으로 채워진다.
- 확정 버튼 클릭이 트리거된다(픽스처 응답에 따라 API 호출이 성공으로 모킹되어 있다).
- 테스트는 예외 없이 통과한다.

**계측 셀렉터**:

- `adminPage.getByText(/rv-001|SUBMITTED.*고유텍스트/)` — `SUBMITTED` 카드 가시화 대기.
- `adminPage.getByRole("button", { name: "배정" })` — 배정 버튼.
- 리뷰어 ID 입력: `getByRole("textbox")` 또는 placeholder 텍스트 기반.
- 확정 버튼: `getByRole("button", { name: /배정 확정|확정/ })`.

---

## AC-FLOW-ANALYST-001 — 점수 제출 (REQ-WEB-002-008, REQ-WEB-002-003, REQ-WEB-002-004)

**Given** analyst 역할로 로그인했고, `apps/web/e2e/fixtures/api/evaluation-items.json`에 `ei-001` "경영목표 달성도"(`parent_id=null`)가 모킹되어 있으며, `evaluation-items/page.tsx` + `TwoPanel`에 `fetchFailed` 폴백이 적용되어 첫 루트 항목이 자동 선택되도록 구현되어 있다.

**When** analyst가 `/dashboard/evaluation-items`(`/dashboard/scores`가 아님)로 이동하고, `TwoPanel`의 폴백 fetch가 완료되어 "경영목표 달성도" 항목이 좌측 패널에 보이며 우측 `ScoreForm`(`#score-value` 입력)이 가시화될 때까지 대기한 다음, 숫자 점수를 입력하고 저장 버튼을 클릭한다.

**Then**:

- `TwoPanel`이 마운트되고, 폴백 fetch로 `/api/v1/evaluation-items`가 호출되어 응답이 트리에 반영된다.
- 첫 루트 항목(`ei-001`)이 자동 선택되어 우측에 `ScoreForm`이 마운트된다.
- `#score-value` 입력이 가시화되고 사용자 입력으로 채워진다.
- 저장 버튼 클릭 시 점수 제출 API가 호출된다(픽스처가 성공 응답을 반환).
- 제출 후 안정 상태가 관찰된다(입력 비워짐 또는 성공 텍스트 등 픽스처 동작에 부합).
- 테스트는 예외 없이 통과한다.

**계측 셀렉터**:

- `analystPage.goto("/dashboard/evaluation-items")` — 올바른 URL.
- `analystPage.getByText("경영목표 달성도").first()` — `TwoPanel` 로드 대기.
- `analystPage.locator("#score-value")` — `ScoreForm` 점수 입력.
- `analystPage.getByRole("button", { name: "저장" })` — 저장 버튼.

---

## 엣지 케이스 (Edge Cases)

다음 케이스는 본 SPEC에서 GREEN을 강제하지는 않으나, Run 단계에서 발견 시 별도 SPEC으로 분리 또는 OPEN 노트로 기록한다.

- **EC-01**: `KanbanBoard`가 `fetchFailed=true`로 마운트된 직후, 사용자가 빠르게 승인 버튼을 클릭하여 `fetchAll()`이 중복 호출되는 race. 가장 늦은 응답이 보드 상태를 덮는지 검증 필요.
- **EC-02**: `TwoPanel`의 폴백 fetch가 완료되기 전에 사용자가 좌측 트리의 다른 항목을 클릭. 선택 상태가 클릭 항목으로 유지되는지 검증.
- **EC-03**: `evaluation-items.json` 픽스처가 빈 응답을 반환할 때 `TwoPanel`의 첫 루트 자동 선택 로직이 NPE를 일으키지 않아야 함.
- **EC-04**: 픽스처 `reviews.json`에 한국어 텍스트가 여러 카드에서 중복될 때 `.first()` 또는 부모 컨테이너 scope로 명확히 단일 카드를 지정.
- **EC-05**: 루브릭 편집 모달이 키보드 ESC로 닫히는 경우의 저장 버튼 단정 — 테스트는 ESC 사용을 피해야 함.
- **EC-06**: Playwright `expect`의 기본 타임아웃 내에서 폴백 fetch가 완료되지 않는 환경(느린 CI). 필요 시 `expect(...).toBeVisible({ timeout: 10000 })`로 명시.
- **EC-07**: `KanbanBoard.fetchAll()`이 컴포넌트 unmount 후에도 resolve되는 비동기 race — cleanup flag로 setState 호출 방지.
- **EC-08**: 점수 입력에 음수 또는 100 초과 값 입력 시 클라이언트 검증 메시지 노출 — 본 SPEC은 정상 흐름만 검증.

## 회귀 검증 체크리스트

- [ ] WEB-003에서 GREEN이었던 9개 E2E 시나리오가 모두 GREEN 유지.
- [ ] 승인/배정 후 기존 `fetchAll()` 재조회 경로(사용자 액션 후 보드 갱신)가 동작.
- [ ] `TwoPanel`의 RSC 정상 경로(`fetchFailed=false`)에서 기존과 동일하게 첫 루트 자동 선택 + 우측 `ScoreForm` 표시.
- [ ] 루브릭 페이지의 기존 fetchFailed 동작(이미 동작 중)이 회귀 없이 유지.

## 품질 게이트 (Quality Gates)

- **lint**: `pnpm --filter @iroum-ax/web lint` 0 warnings on 변경 파일.
- **typecheck**: `pnpm --filter @iroum-ax/web typecheck` 0 errors.
- **e2e**: `pnpm --filter @iroum-ax/web test:e2e` 모든 시나리오 GREEN (기존 9 + 신규 4 = 13).
- **frozen-scope**: `git diff --quiet -- <frozen paths>` 종료 코드 0.
- **no-testid**: `git diff` 결과에 `data-testid` 신규 도입 0건 (`grep -c "data-testid" <변경된 spec 파일>`이 변경 전과 동일).
