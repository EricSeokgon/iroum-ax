// SPEC-AX-E2E-001 — Playwright 설정 (Chromium 단일, OPEN #3 옵션 A)
// 실행 전제: `npm run dev` 가 별도 터미널에서 실행 중이어야 함 (PLAYWRIGHT_BASE_URL).
// CI 통합은 EXC-3 (별도 SPEC).

import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  // 테스트 디렉터리 — apps/web/e2e/
  testDir: "./e2e",

  // globalSetup: 3개 role 의 storageState 사전 생성 (OPEN #1 옵션 C: 직접 쿠키 주입)
  globalSetup: "./e2e/global-setup.ts",

  // 병렬 실행
  fullyParallel: true,

  // CI 에서는 .only() 금지 (REQ-E2E-NO-303 정신과 일치)
  forbidOnly: !!process.env["CI"],

  // 재시도 — 로컬 0, CI 2 (REQ-E2E-OPT-200 trace 와 연동)
  retries: process.env["CI"] ? 2 : 0,

  // 워커 수 — CI 에서는 1 (순차) 로 안정성 우선
  workers: process.env["CI"] ? 1 : undefined,

  // 리포터 — list (콘솔) + html (파일, 자동 오픈 안함)
  reporter: [["list"], ["html", { open: "never" }]],

  use: {
    // baseURL — 기본 localhost:3000 (Next.js dev 기본 포트)
    baseURL: process.env["PLAYWRIGHT_BASE_URL"] ?? "http://localhost:3000",

    // 첫 재시도 시 trace 보존 (디버깅용, REQ-E2E-OPT-200)
    trace: "on-first-retry",

    // 콘솔/네트워크 캡처 — 실패 시만
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },

  // 단일 브라우저 — Chromium Desktop Chrome (OPEN #3)
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
