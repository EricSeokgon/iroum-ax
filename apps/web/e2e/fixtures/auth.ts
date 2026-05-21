// SPEC-AX-E2E-001 — role 별 storageState fixture
// globalSetup 에서 미리 생성된 viewer/analyst/admin storageState 를 로드해
// 각 테스트가 즉시 인증된 page 를 받도록 한다.

import { test as base, type Page, type BrowserContext } from "@playwright/test";
import path from "node:path";

const AUTH_DIR = path.join(__dirname, "..", ".auth");

interface RolePages {
  viewerPage: Page;
  analystPage: Page;
  adminPage: Page;
}

/**
 * 단일 role 의 인증된 context + page 생성 헬퍼.
 * cleanup 은 fixture 의 `use` 패턴이 자동 수행.
 */
async function makeRoleContext(
  browser: BrowserContext["browser"] extends () => infer B ? B : never,
  storageStateFile: string,
): Promise<{ context: BrowserContext; page: Page }> {
  if (browser === null) {
    throw new Error("Browser is null — fixture initialization failed");
  }
  const context = await browser.newContext({ storageState: storageStateFile });
  const page = await context.newPage();
  return { context, page };
}

export const test = base.extend<RolePages>({
  viewerPage: async ({ browser }, use) => {
    const { context, page } = await makeRoleContext(
      browser,
      path.join(AUTH_DIR, "viewer.json"),
    );
    await use(page);
    await context.close();
  },
  analystPage: async ({ browser }, use) => {
    const { context, page } = await makeRoleContext(
      browser,
      path.join(AUTH_DIR, "analyst.json"),
    );
    await use(page);
    await context.close();
  },
  adminPage: async ({ browser }, use) => {
    const { context, page } = await makeRoleContext(
      browser,
      path.join(AUTH_DIR, "admin.json"),
    );
    await use(page);
    await context.close();
  },
});

export { expect } from "@playwright/test";
