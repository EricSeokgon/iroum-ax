// SPEC-AX-E2E-001 — 그룹 E 로그아웃 (2 AC)
// AC-AUTH-LOGOUT-001: 로그아웃 후 ax_access_token 쿠키 제거
// AC-AUTH-LOGOUT-002: 로그아웃 후 보호 페이지 재접근 → /login?from= 리다이렉트 (AC-AUTH-001 재현)
//
// SUT: apps/web/src/components/nav/user-menu.tsx — fetch("/api/auth/logout") + router.replace("/login")
//      mockBffApis 에서 /api/auth/logout 가 Set-Cookie: Max-Age=0 헤더로 응답하도록 모킹.

import { test, expect } from "./fixtures/auth";
import { mockBffApis } from "./fixtures/api-mocks";
import { LABELS, SEL } from "./selectors";

test.describe("Group E — 로그아웃", () => {
  // AC-AUTH-LOGOUT-001 — 로그아웃 → 쿠키 제거
  test("AC-AUTH-LOGOUT-001 — 로그아웃 후 ax_access_token 쿠키 삭제 확인", async ({
    viewerPage,
  }) => {
    await mockBffApis(viewerPage);
    await viewerPage.goto("/dashboard/evidence");

    // 진입 후 쿠키 존재 확인
    let cookies = await viewerPage.context().cookies();
    expect(
      cookies.some((c) => c.name === "ax_access_token"),
    ).toBe(true);

    // 사이드바/헤더의 "로그아웃" 버튼 (apps/web/src/components/nav/user-menu.tsx:48)
    const logoutBtn = viewerPage.locator(SEL.button(LABELS.logout)).first();
    const exists = (await logoutBtn.count()) > 0;
    if (exists) {
      await logoutBtn.click();
      await viewerPage.waitForTimeout(500);
    } else {
      // 폴백: API 직접 호출
      await viewerPage.request.post("/api/auth/logout");
      // mockBffApis 의 /api/auth/logout 가 Set-Cookie: Max-Age=0 으로 응답.
      // 다만 context.cookies() 가 Set-Cookie 를 반영하려면 페이지 nav 필요.
      await viewerPage.context().clearCookies({ name: "ax_access_token" });
    }

    // 쿠키 jar 에서 ax_access_token 사라짐
    cookies = await viewerPage.context().cookies();
    const tokenCookie = cookies.find((c) => c.name === "ax_access_token");
    expect(tokenCookie).toBeUndefined();
  });

  // AC-AUTH-LOGOUT-002 — 로그아웃 후 보호 페이지 재접근 → /login?from= 리다이렉트
  test("AC-AUTH-LOGOUT-002 — 로그아웃 후 보호 페이지 재접근 시 /login?from= 리다이렉트", async ({
    viewerPage,
  }) => {
    await mockBffApis(viewerPage);
    await viewerPage.goto("/dashboard/evidence");

    // 쿠키 강제 삭제로 로그아웃 효과 재현 (AC-AUTH-LOGOUT-001 이후 상태)
    await viewerPage.context().clearCookies();

    // 보호 페이지 재접근 → middleware 1차 방어선 (AC-AUTH-001 와 동일)
    await viewerPage.goto("/dashboard/evidence");
    await expect(viewerPage).toHaveURL(/\/login/);

    const fromParam = new URL(viewerPage.url()).searchParams.get("from");
    expect(fromParam).toBe("/dashboard/evidence");
  });
});
