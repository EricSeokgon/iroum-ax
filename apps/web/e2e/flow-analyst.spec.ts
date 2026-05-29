// SPEC-AX-E2E-001 — 그룹 C analyst 흐름 (4 AC)
// AC-VIS-ANALYST-001: 증빙 업로드 버튼 가시
// AC-FLOW-ANALYST-001: 점수 제출 → POST /api/v1/scores 네트워크 확인
// AC-FLOW-ANALYST-002: 리뷰 제출 → POST /api/v1/reviews 네트워크 확인
// AC-FLOW-ANALYST-DENY-001: analyst 의 audit-logs → in-page 차단 (redirect 아님)
//
// 주의: 일부 흐름 테스트는 SUT 폼 구조(EvidenceUpload/ScoreForm/SubmitReviewForm) 의
//       구체 입력 필드에 의존한다. data-testid 가 없으므로 (EXC-6/REQ-E2E-NO-300)
//       SUT 가 변경되면 test.fixme 로 마킹하고 SPEC-AX-WEB-002 후속 SPEC 트리거 필요.

import { test, expect } from "./fixtures/auth";
import { mockBffApis } from "./fixtures/api-mocks";
import { LABELS, SEL } from "./selectors";

test.describe("Group C — analyst 흐름", () => {
  test.beforeEach(async ({ analystPage }) => {
    await mockBffApis(analystPage);
  });

  // AC-VIS-ANALYST-001 — analyst 의 증빙 업로드 버튼 가시
  // SUT: evidence/page.tsx <RoleGate allow={["analyst","admin"]}> → EvidenceUpload 마운트됨
  test('AC-VIS-ANALYST-001 — analyst 의 증빙 페이지에 "업로드" 버튼 가시', async ({
    analystPage,
  }) => {
    await analystPage.goto("/dashboard/evidence");
    await expect(analystPage).toHaveURL(/\/dashboard\/evidence/);

    const uploadBtn = analystPage.locator(SEL.button(LABELS.upload));
    await expect(uploadBtn.first()).toBeVisible();
  });

  // AC-FLOW-ANALYST-001 — 점수 폼 제출 → POST /api/v1/scores 발생
  // SUT 폼 구조 의존 — 실패 시 fixme + SPEC-AX-WEB-002 트리거 (REQ-E2E-NO-300)
  test("AC-FLOW-ANALYST-001 — 점수 제출 후 POST /api/v1/scores 네트워크 호출 발생", async ({
    analystPage,
  }) => {
    let scorePostCalled: boolean = false;
    analystPage.on("request", (req) => {
      if (
        req.method() === "POST" &&
        req.url().includes("/api/v1/scores") &&
        !req.url().includes("/api/v1/scores/")
      ) {
        scorePostCalled = true;
      }
    });

    await analystPage.goto("/dashboard/evaluation-items");
    // TwoPanel fetchFailed → 브라우저 fetch → "경영목표 달성도" 항목 자동 선택 대기
    await expect(analystPage.getByText("경영목표 달성도").first()).toBeVisible();

    // ItemDetail ScoreForm → #score-value 입력
    const scoreInput = analystPage.locator("#score-value");
    await expect(scoreInput).toBeVisible();
    await scoreInput.fill("85");

    // "저장" 버튼 클릭
    const saveBtn = analystPage.locator(SEL.button(LABELS.scoreSubmit)).first();
    await expect(saveBtn).toBeEnabled();
    await saveBtn.click();

    // 네트워크 호출 검증
    await analystPage.waitForTimeout(500);
    expect(scorePostCalled).toBe(true);
  });

  // AC-FLOW-ANALYST-002 — 리뷰 제출 → POST /api/v1/reviews 발생
  // SUT: SubmitReviewForm 의 "+ 리뷰 제출" 버튼 (RoleGate allow analyst/admin → 항상 렌더됨)
  // mockBffApis 가 POST /api/v1/reviews → 201 { id, status:"SUBMITTED" } 모킹.
  test("AC-FLOW-ANALYST-002 — 리뷰 제출 후 POST /api/v1/reviews 네트워크 호출 발생", async ({
    analystPage,
  }) => {
    let reviewPostCalled: boolean = false;
    analystPage.on("request", (req) => {
      if (
        req.method() === "POST" &&
        req.url().match(/\/api\/v1\/reviews(\?|$)/)
      ) {
        reviewPostCalled = true;
      }
    });

    await analystPage.goto("/dashboard/reviews");

    // SubmitReviewForm: analyst 역할 → RoleGate 통과 → 항상 마운트됨
    const submitBtn = analystPage.locator(SEL.button(LABELS.reviewSubmit));
    await expect(submitBtn.first()).toBeVisible();
    await submitBtn.first().click();

    // 폼 열림 → 제목 입력 후 제출
    const titleInput = analystPage.locator("#review-title");
    await expect(titleInput).toBeVisible();
    await titleInput.fill("E2E 테스트 리뷰");

    const innerSubmit = analystPage.locator(SEL.submitButton());
    await expect(innerSubmit.first()).toBeEnabled();
    await innerSubmit.first().click();

    await analystPage.waitForTimeout(500);
    expect(reviewPostCalled).toBe(true);
  });

  // AC-FLOW-ANALYST-DENY-001 — analyst 의 audit-logs 접근 시 in-page 차단
  // (viewer 와 동일 패턴 — audit-logs/page.tsx 의 session.role !== "admin" 분기)
  test("AC-FLOW-ANALYST-DENY-001 — analyst 의 audit-logs 접근 시 in-page 권한 차단 (URL 유지)", async ({
    analystPage,
  }) => {
    await analystPage.goto("/dashboard/audit-logs");

    await expect(
      analystPage.locator(SEL.text(LABELS.accessDenied)),
    ).toBeVisible();
    expect(new URL(analystPage.url()).pathname).toBe("/dashboard/audit-logs");
    await expect(analystPage).not.toHaveURL(/\/login/);
  });
});
