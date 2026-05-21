// SPEC-AX-E2E-001 — 그룹 A 인증 흐름 (3 AC)
// AC-AUTH-001: 미인증 보호 페이지 접근 → /login?from= 리다이렉트 (middleware 1차 방어선)
// AC-AUTH-002: storageState 주입 후 보호 페이지 진입 성공
// AC-AUTH-003: 만료 JWT 쿠키 → layout.tsx redirect('/login') (from 파라미터 없음)

import { test, expect } from "@playwright/test";
import path from "node:path";

const AUTH_DIR = path.join(__dirname, ".auth");

/** base64url 인코딩 */
function b64(obj: object): string {
  return Buffer.from(JSON.stringify(obj)).toString("base64url");
}

// AC-AUTH-001
test("AC-AUTH-001 — 미인증 시 /login?from= 리다이렉트 (middleware 1차 방어선)", async ({
  page,
}) => {
  // Given: 쿠키 없는 상태 (default new context)
  // When: 보호 경로 직접 이동
  await page.goto("/dashboard/evidence");

  // Then: /login 으로 리다이렉트 + from 쿼리 파라미터 (apps/web/src/middleware.ts:21)
  await expect(page).toHaveURL(/\/login/);
  const fromParam = new URL(page.url()).searchParams.get("from");
  expect(fromParam).toBe("/dashboard/evidence");
});

// AC-AUTH-002
test("AC-AUTH-002 — viewer storageState 주입 후 /dashboard/evidence 진입 성공", async ({
  browser,
}) => {
  // Given: viewer storageState 사전 주입
  const context = await browser.newContext({
    storageState: path.join(AUTH_DIR, "viewer.json"),
  });
  const page = await context.newPage();

  // BFF 응답 모킹 — page 가 데이터를 못 받으면 에러 페이지로 빠질 수 있으므로 최소 모킹
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, json: { items: [], total: 0 } }),
  );

  // When: 보호 페이지 이동
  await page.goto("/dashboard/evidence");

  // Then: /login 으로 리다이렉트되지 않음 + URL이 evidence 페이지를 유지
  await expect(page).not.toHaveURL(/\/login/);
  await expect(page).toHaveURL(/\/dashboard\/evidence/);

  await context.close();
});

// AC-AUTH-003 — 만료 쿠키 흐름
test("AC-AUTH-003 — 만료 JWT 쿠키 시 /login 리다이렉트 (from 파라미터 없음, layout 2차 방어선)", async ({
  browser,
}) => {
  // Given: exp 가 과거인 mock JWT
  const header = b64({ alg: "RS256", typ: "JWT" });
  const expiredPayload = b64({
    sub: "test-user",
    scope: "openid profile iroum-ax:viewer",
    // 1시간 전 만료
    exp: Math.floor(Date.now() / 1000) - 3600,
    iat: Math.floor(Date.now() / 1000) - 7200,
  });
  const expiredJwt = `${header}.${expiredPayload}.mock-sig`;

  const context = await browser.newContext();
  await context.addCookies([
    {
      name: "ax_access_token",
      value: expiredJwt,
      domain: "localhost",
      path: "/",
      httpOnly: true,
      secure: false,
      sameSite: "Lax",
    },
  ]);
  const page = await context.newPage();

  // When: 보호 페이지 이동
  await page.goto("/dashboard/evidence");

  // Then: middleware 는 cookie 존재만 보므로 통과(apps/web/src/middleware.ts:13-15 주석),
  //       layout.tsx 가 getServerSession() null 감지 → redirect("/login") (line 17-20)
  //       이 redirect 는 `?from=` 파라미터를 부착하지 않음.
  await expect(page).toHaveURL(/\/login(?!\?from=)/);
  // URL pathname 이 정확히 /login 임을 확인 (search 없음)
  expect(new URL(page.url()).pathname).toBe("/login");

  await context.close();
});
