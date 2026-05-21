// SPEC-AX-E2E-001 — 그룹 B viewer RBAC (7 AC)
// AC-VIS-VIEWER-001/002/003: 증빙 업로드/점수 입력/리뷰 승인 버튼 비노출
// AC-VIS-ALL-001/002: 평가 항목 트리 / 리포트 가시 (전 역할)
// AC-RBAC-DENY-001/002: viewer 의 audit-logs/rubric 직접 URL → 인-페이지 차단 (redirect 아님)

import { test, expect } from "./fixtures/auth";
import { mockBffApis } from "./fixtures/api-mocks";
import { LABELS, SEL } from "./selectors";

test.describe("Group B — viewer RBAC", () => {
  test.beforeEach(async ({ viewerPage }) => {
    await mockBffApis(viewerPage);
  });

  // AC-VIS-VIEWER-001 — viewer 의 증빙 페이지에서 "업로드" 버튼 비노출
  // SUT: apps/web/src/app/(dashboard)/evidence/page.tsx 의
  //      <RoleGate allow={["analyst","admin"]}> 으로 EvidenceUpload 가 mount 되지 않음
  test('AC-VIS-VIEWER-001 — viewer 의 증빙 페이지에 "업로드" 버튼 비노출', async ({
    viewerPage,
  }) => {
    await viewerPage.goto("/dashboard/evidence");
    await expect(viewerPage).toHaveURL(/\/dashboard\/evidence/);

    // 업로드 버튼이 DOM 에 아예 없거나 disabled
    const uploadBtn = viewerPage.locator(SEL.button(LABELS.upload));
    const count = await uploadBtn.count();
    if (count === 0) {
      // RoleGate 가 mount 자체를 막은 경우 — 정상
      expect(count).toBe(0);
    } else {
      // 혹시 DOM 에 있다면 disabled 여야 함
      await expect(uploadBtn).toBeDisabled();
    }
  });

  // AC-VIS-VIEWER-002 — viewer 의 점수 페이지에 제출 버튼 비노출
  test("AC-VIS-VIEWER-002 — viewer 의 점수 페이지에 제출 폼 비노출", async ({
    viewerPage,
  }) => {
    await viewerPage.goto("/dashboard/scores");
    await expect(viewerPage).toHaveURL(/\/dashboard\/scores/);

    // 제출 버튼이 없거나 disabled
    const submitBtn = viewerPage.locator(SEL.submitButton());
    const count = await submitBtn.count();
    if (count > 0) {
      // 존재하더라도 viewer 는 disabled
      await expect(submitBtn.first()).toBeDisabled();
    } else {
      expect(count).toBe(0);
    }
  });

  // AC-VIS-VIEWER-003 — viewer 의 리뷰 페이지에 승인/반려 버튼 비노출
  // SUT: review-card.tsx line 57-58 isAdmin && status 조건 → viewer 는 절대 표시 안됨
  test("AC-VIS-VIEWER-003 — viewer 의 리뷰 페이지에 승인/반려 버튼 비노출", async ({
    viewerPage,
  }) => {
    await viewerPage.goto("/dashboard/reviews");
    await expect(viewerPage).toHaveURL(/\/dashboard\/reviews/);

    await expect(viewerPage.locator(SEL.button(LABELS.approve))).toHaveCount(0);
    await expect(viewerPage.locator(SEL.button(LABELS.reject))).toHaveCount(0);
    await expect(viewerPage.locator(SEL.button(LABELS.assign))).toHaveCount(0);
  });

  // AC-VIS-ALL-001 — 평가 항목 트리 가시 (viewer)
  test("AC-VIS-ALL-001 — viewer 의 평가 항목 페이지 가시", async ({
    viewerPage,
  }) => {
    await viewerPage.goto("/dashboard/evaluation-items");
    await expect(viewerPage).toHaveURL(/\/dashboard\/evaluation-items/);
    // 차단되지 않음 — login 으로 리다이렉트 또는 인-페이지 차단 텍스트 없음
    await expect(viewerPage).not.toHaveURL(/\/login/);
    await expect(
      viewerPage.locator(SEL.text(LABELS.accessDenied)),
    ).toHaveCount(0);
  });

  // AC-VIS-ALL-002 — 리포트 페이지 가시 (viewer)
  test("AC-VIS-ALL-002 — viewer 의 리포트 페이지 가시", async ({
    viewerPage,
  }) => {
    await viewerPage.goto("/dashboard/reports");
    await expect(viewerPage).toHaveURL(/\/dashboard\/reports/);
    await expect(viewerPage).not.toHaveURL(/\/login/);
    await expect(
      viewerPage.locator(SEL.text(LABELS.accessDenied)),
    ).toHaveCount(0);
  });

  // AC-RBAC-DENY-001 — viewer 의 audit-logs 직접 URL → in-page 차단 (redirect 아님)
  // SUT: apps/web/src/app/(dashboard)/audit-logs/page.tsx:3 코멘트 명시
  //      "비-admin 접근 시 in-page error 표시 (redirect 대신, URL 유지)"
  test("AC-RBAC-DENY-001 — viewer 의 audit-logs 접근 시 in-page 권한 차단 메시지 (URL 유지)", async ({
    viewerPage,
  }) => {
    await viewerPage.goto("/dashboard/audit-logs");

    // in-page 메시지 가시
    await expect(
      viewerPage.locator(SEL.text(LABELS.accessDenied)),
    ).toBeVisible();
    // URL 은 변경되지 않음 — redirect 없음
    expect(new URL(viewerPage.url()).pathname).toBe("/dashboard/audit-logs");
    // /dashboard 로 redirect 되지 않음을 명시적으로 어설션
    await expect(viewerPage).not.toHaveURL(/\/dashboard$/);
    await expect(viewerPage).not.toHaveURL(/\/login/);
  });

  // AC-RBAC-DENY-002 — viewer 의 rubric 직접 URL → in-page 차단
  test("AC-RBAC-DENY-002 — viewer 의 rubric 접근 시 in-page 권한 차단 메시지 (URL 유지)", async ({
    viewerPage,
  }) => {
    await viewerPage.goto("/dashboard/rubric");

    await expect(
      viewerPage.locator(SEL.text(LABELS.accessDenied)),
    ).toBeVisible();
    expect(new URL(viewerPage.url()).pathname).toBe("/dashboard/rubric");
    await expect(viewerPage).not.toHaveURL(/\/dashboard$/);
    await expect(viewerPage).not.toHaveURL(/\/login/);
  });
});
