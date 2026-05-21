// SPEC-AX-E2E-001 — Playwright globalSetup
// 3 개 role(viewer/analyst/admin) storageState 사전 생성 (OPEN #1 옵션 C, OPEN #5)
// 실제 Keycloak 의존 없이 mock JWT + 직접 쿠키 주입.
//
// SUT 의존 사실 (apps/web/src/lib/auth.ts 에서 확인):
// - 쿠키 이름: ax_access_token (line 14, COOKIE_ACCESS_TOKEN)
// - JWT 디코드: decodeJwt(jose) — 서명 검증 없음 (line 60)
// - scope 형식: "iroum-ax:{admin|analyst|viewer}" (line 89-97)
// - 만료 검사: getServerSession() exp 비교 (line 112-113)

import { chromium, type FullConfig } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const AUTH_DIR = path.join(__dirname, ".auth");

/** base64url 인코딩 — Node.js Buffer 표준 */
function b64(obj: object): string {
  return Buffer.from(JSON.stringify(obj)).toString("base64url");
}

/**
 * Mock JWT 생성 — header.payload.dummysignature.
 * apps/web/src/lib/auth.ts:decodeJwt 는 서명을 검증하지 않고 페이로드만 base64 디코드한다.
 */
function makeMockJwt(scope: string): string {
  const header = b64({ alg: "RS256", typ: "JWT" });
  const payload = b64({
    sub: "test-user",
    email: "test@example.com",
    preferred_username: "E2E Test User",
    scope,
    exp: Math.floor(Date.now() / 1000) + 3600, // +1h, 만료 안된 상태
    iat: Math.floor(Date.now() / 1000),
  });
  return `${header}.${payload}.mock-signature-not-verified`;
}

/**
 * 하나의 role 에 대해 storageState 파일을 생성.
 * BrowserContext 에 mock JWT 쿠키를 주입 → storageState 저장.
 */
async function createStorageState(
  jwt: string,
  filePath: string,
  baseURL: string,
): Promise<void> {
  const browser = await chromium.launch();
  try {
    const context = await browser.newContext();
    const hostname = new URL(baseURL).hostname;
    await context.addCookies([
      {
        name: "ax_access_token",
        value: jwt,
        domain: hostname,
        path: "/",
        httpOnly: true,
        secure: false,
        sameSite: "Lax",
      },
    ]);
    await context.storageState({ path: filePath });
  } finally {
    await browser.close();
  }
}

export default async function globalSetup(config: FullConfig): Promise<void> {
  if (!fs.existsSync(AUTH_DIR)) {
    fs.mkdirSync(AUTH_DIR, { recursive: true });
  }

  const baseURL =
    config.projects[0]?.use?.baseURL ?? "http://localhost:3000";

  // 3 개 role 병렬 생성 — 독립이므로 동시 실행 가능
  await Promise.all([
    createStorageState(
      makeMockJwt("openid profile iroum-ax:viewer"),
      path.join(AUTH_DIR, "viewer.json"),
      baseURL,
    ),
    createStorageState(
      makeMockJwt("openid profile iroum-ax:analyst"),
      path.join(AUTH_DIR, "analyst.json"),
      baseURL,
    ),
    createStorageState(
      makeMockJwt("openid profile iroum-ax:admin"),
      path.join(AUTH_DIR, "admin.json"),
      baseURL,
    ),
  ]);
}
