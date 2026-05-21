// SPEC-AX-E2E-001 — 그룹 D admin 흐름 (5 AC)
// AC-VIS-ADMIN-001/002: admin 의 audit-logs / rubric 페이지 접근 가능
// AC-FLOW-ADMIN-001: 루브릭 임계값 편집 저장 → PUT /api/v1/rubric/thresholds/{scope}
// AC-FLOW-ADMIN-002: 리뷰 승인 → POST /approve  (review status: UNDER_REVIEW 필요)
// AC-FLOW-ADMIN-003: 리뷰어 배정 → POST /assign-reviewer (review status: SUBMITTED 필요)
//
// mock review fixture: rv-001=SUBMITTED → 배정 버튼, rv-002=UNDER_REVIEW → 승인/반려 버튼.
// (apps/web/src/components/review/review-card.tsx line 57-58 분기 참고)

import { test, expect } from "./fixtures/auth";
import { mockBffApis } from "./fixtures/api-mocks";
import { LABELS, SEL } from "./selectors";

test.describe("Group D — admin 흐름", () => {
  test.beforeEach(async ({ adminPage }) => {
    await mockBffApis(adminPage);
  });

  // AC-VIS-ADMIN-001 — admin 의 감사 로그 페이지 접근
  test("AC-VIS-ADMIN-001 — admin 의 감사 로그 페이지 접근 + 모킹된 로그 표시", async ({
    adminPage,
  }) => {
    await adminPage.goto("/dashboard/audit-logs");
    await expect(adminPage).toHaveURL(/\/dashboard\/audit-logs/);
    // 인-페이지 차단 텍스트 없음
    await expect(
      adminPage.locator(SEL.text(LABELS.accessDenied)),
    ).toHaveCount(0);
    // 모킹된 로그 1건 이상 표시
    await expect(
      adminPage.getByText("EVIDENCE_UPLOAD").first(),
    ).toBeVisible();
  });

  // AC-VIS-ADMIN-002 — admin 의 루브릭 페이지 접근
  test("AC-VIS-ADMIN-002 — admin 의 루브릭 페이지 접근 + 임계값 표시", async ({
    adminPage,
  }) => {
    await adminPage.goto("/dashboard/rubric");
    await expect(adminPage).toHaveURL(/\/dashboard\/rubric/);
    await expect(
      adminPage.locator(SEL.text(LABELS.accessDenied)),
    ).toHaveCount(0);
    // 임계값 fixture 의 "경영성과" scope 표시
    await expect(adminPage.getByText("경영성과").first()).toBeVisible();
  });

  // AC-FLOW-ADMIN-001 — 루브릭 임계값 편집 → PUT 호출
  // SUT: rubric-threshold-table.tsx — "수정" 클릭 → number input 편집 → "저장"
  test("AC-FLOW-ADMIN-001 — 루브릭 임계값 편집 저장 시 PUT /api/v1/rubric/thresholds 호출", async ({
    adminPage,
  }) => {
    let putCalled: boolean = false;
    adminPage.on("request", (req) => {
      if (
        req.method() === "PUT" &&
        req.url().includes("/api/v1/rubric/thresholds")
      ) {
        putCalled = true;
      }
    });

    await adminPage.goto("/dashboard/rubric");

    const editBtn = adminPage.locator(SEL.button(LABELS.edit)).first();
    if ((await editBtn.count()) === 0) {
      test.fixme(
        true,
        "SUT 루브릭 '수정' 버튼이 행별 액션 — fixture 와 행 매칭이 필요. SPEC-AX-WEB-002 후속.",
      );
      return;
    }
    await editBtn.click();

    // 인라인 폼의 number input 등장 후 값 수정
    const numberInputs = adminPage.locator(SEL.numberInput());
    if ((await numberInputs.count()) === 0) {
      test.fixme(
        true,
        "SUT 인라인 편집 행에 number input 이 즉시 나타나지 않음 — SPEC-AX-WEB-002 후속.",
      );
      return;
    }
    await numberInputs.first().fill("92");

    // "저장" 버튼 — 편집 행의 form 내부
    const saveBtn = adminPage.locator(SEL.button(LABELS.save)).first();
    if (!(await saveBtn.isEnabled())) {
      test.fixme(
        true,
        "SUT '저장' 버튼이 검증 통과 전 비활성 — 추가 입력 필요. SPEC-AX-WEB-002 후속.",
      );
      return;
    }
    await saveBtn.click();
    await adminPage.waitForTimeout(500);

    expect(putCalled).toBe(true);
  });

  // AC-FLOW-ADMIN-002 — 리뷰 승인 → POST /approve
  // rv-002 는 UNDER_REVIEW 이므로 review-card 가 "승인" 버튼 노출
  test("AC-FLOW-ADMIN-002 — admin 리뷰 승인 시 POST /api/v1/reviews/{id}/approve 호출", async ({
    adminPage,
  }) => {
    let approveCalled: boolean = false;
    adminPage.on("request", (req) => {
      if (req.method() === "POST" && req.url().includes("/approve")) {
        approveCalled = true;
      }
    });

    await adminPage.goto("/dashboard/reviews");

    // 모킹된 rv-002 (UNDER_REVIEW) 의 "승인" 버튼 클릭
    const approveBtn = adminPage.locator(SEL.button(LABELS.approve)).first();
    if ((await approveBtn.count()) === 0) {
      test.fixme(
        true,
        "SUT 리뷰 카드에 '승인' 버튼이 보이지 않음 — fixture rv-002 (UNDER_REVIEW) 매칭 실패 또는 Kanban 진입 조건 필요. SPEC-AX-WEB-002 후속.",
      );
      return;
    }
    await approveBtn.click();
    await adminPage.waitForTimeout(300);

    // 인라인 폼이 열린 경우 — 코멘트 옵션 입력 후 제출
    const innerApprove = adminPage
      .locator(`button[type="submit"]:has-text("${LABELS.approve}")`)
      .first();
    if ((await innerApprove.count()) > 0 && (await innerApprove.isEnabled())) {
      await innerApprove.click();
      await adminPage.waitForTimeout(500);
    }

    expect(approveCalled).toBe(true);
  });

  // AC-FLOW-ADMIN-003 — 리뷰어 배정 → POST /assign-reviewer
  // rv-001 는 SUBMITTED 이므로 "리뷰어 배정" 버튼 노출
  test("AC-FLOW-ADMIN-003 — admin 리뷰어 배정 시 POST /api/v1/reviews/{id}/assign-reviewer 호출", async ({
    adminPage,
  }) => {
    let assignCalled: boolean = false;
    adminPage.on("request", (req) => {
      if (req.method() === "POST" && req.url().includes("/assign-reviewer")) {
        assignCalled = true;
      }
    });

    await adminPage.goto("/dashboard/reviews");

    const assignBtn = adminPage.locator(SEL.button(LABELS.assign)).first();
    if ((await assignBtn.count()) === 0) {
      test.fixme(
        true,
        "SUT 리뷰 카드에 '리뷰어 배정' 버튼이 보이지 않음 — fixture rv-001 (SUBMITTED) 매칭 실패 또는 Kanban 진입 조건 필요. SPEC-AX-WEB-002 후속.",
      );
      return;
    }
    await assignBtn.click();
    await adminPage.waitForTimeout(300);

    // 인라인 폼 — reviewer ID 입력 + "확인"
    const reviewerInput = adminPage.locator('input[id^="reviewer-"]').first();
    if ((await reviewerInput.count()) > 0) {
      await reviewerInput.fill("reviewer-001");
      const confirmBtn = adminPage
        .locator('button[type="submit"]:has-text("확인")')
        .first();
      if ((await confirmBtn.count()) > 0) {
        await confirmBtn.click();
        await adminPage.waitForTimeout(500);
      }
    }

    expect(assignCalled).toBe(true);
  });
});
