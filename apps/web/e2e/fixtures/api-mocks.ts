// SPEC-AX-E2E-001 — BFF API 응답 모킹 헬퍼 (OPEN #2 옵션 D: page.route 모킹 + 옵션 라이브)
// 각 spec 에서 `await mockBffApis(page)` 한 줄로 모든 /api/v1/* 응답을 fixtures/api/ JSON 으로 대체.
// 시나리오별 override 가 필요한 경우 spec 내부에서 `page.route` 를 더 좁은 패턴으로 재정의.

import type { Page, Route } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";

const FIXTURES_DIR = path.join(__dirname, "api");

/** JSON fixture 로드 — sync (테스트 시작 시 1회만) */
function loadFixture<T>(name: string): T {
  const filePath = path.join(FIXTURES_DIR, name);
  const raw = fs.readFileSync(filePath, "utf-8");
  return JSON.parse(raw) as T;
}

/**
 * 표준 BFF 응답 모킹 — 7개 도메인 + auth/logout.
 *
 * 사용:
 *   test.beforeEach(async ({ viewerPage }) => {
 *     await mockBffApis(viewerPage);
 *   });
 *
 * 재정의 필요 시 본 호출 후 더 좁은 page.route 를 추가하면 우선순위 적용.
 */
// @MX:ANCHOR: [AUTO] mockBffApis — BFF API 목 진입점. 5개 spec 파일(auth/rbac-viewer/flow-analyst/flow-admin/logout)에서 호출(fan_in=5).
// @MX:REASON: fan_in=5 ≥ 3 임계값 초과. 이 함수 서명·동작 변경 시 5개 spec 파일 전부 영향.
// @MX:SPEC: SPEC-AX-E2E-001
export async function mockBffApis(page: Page): Promise<void> {
  const evidences = loadFixture("evidences.json");
  const evaluationItems = loadFixture("evaluation-items.json");
  const scores = loadFixture("scores.json");
  const reviews = loadFixture("reviews.json");
  const auditLogs = loadFixture("audit-logs.json");
  const rubricThresholds = loadFixture("rubric-thresholds.json");

  // GET/POST /api/v1/evidences (※ /evidences/{id} 도 동일 매처에 잡힘)
  await page.route("**/api/v1/evidences**", (route: Route) => {
    if (route.request().method() === "GET") {
      return route.fulfill({ status: 200, json: evidences });
    }
    return route.fulfill({
      status: 201,
      json: { id: "ev-new-001", message: "업로드 성공" },
    });
  });

  // GET /api/v1/evaluation-items
  await page.route("**/api/v1/evaluation-items**", (route: Route) =>
    route.fulfill({ status: 200, json: evaluationItems }),
  );

  // GET/POST /api/v1/scores
  await page.route("**/api/v1/scores**", (route: Route) => {
    if (route.request().method() === "GET") {
      return route.fulfill({ status: 200, json: scores });
    }
    return route.fulfill({
      status: 201,
      json: { id: "sc-new-001", message: "저장됨" },
    });
  });

  // GET /api/v1/reports/category/{id}
  await page.route("**/api/v1/reports/**", (route: Route) =>
    route.fulfill({
      status: 200,
      json: { category: "경영성과", items: [] },
    }),
  );

  // POST /api/v1/reviews/{id}/approve — 더 좁은 패턴 먼저
  await page.route("**/api/v1/reviews/*/approve", (route: Route) =>
    route.fulfill({ status: 200, json: { status: "APPROVED" } }),
  );

  // POST /api/v1/reviews/{id}/assign-reviewer
  await page.route("**/api/v1/reviews/*/assign-reviewer", (route: Route) =>
    route.fulfill({ status: 200, json: { reviewer_id: "reviewer-001" } }),
  );

  // POST /api/v1/reviews/{id}/reject
  await page.route("**/api/v1/reviews/*/reject", (route: Route) =>
    route.fulfill({ status: 200, json: { status: "REJECTED" } }),
  );

  // GET/POST /api/v1/reviews (목록/제출)
  await page.route("**/api/v1/reviews**", (route: Route) => {
    if (route.request().method() === "GET") {
      return route.fulfill({ status: 200, json: reviews });
    }
    return route.fulfill({
      status: 201,
      json: { id: "rv-new-001", status: "SUBMITTED" },
    });
  });

  // GET /api/v1/audit-logs
  await page.route("**/api/v1/audit-logs**", (route: Route) =>
    route.fulfill({ status: 200, json: auditLogs }),
  );

  // GET/PUT /api/v1/rubric/thresholds
  await page.route("**/api/v1/rubric/thresholds**", (route: Route) => {
    if (route.request().method() === "PUT") {
      return route.fulfill({
        status: 200,
        json: { message: "임계값이 저장되었습니다." },
      });
    }
    return route.fulfill({ status: 200, json: rubricThresholds });
  });

  // POST /api/auth/logout — 쿠키 만료 헤더 반환
  await page.route("**/api/auth/logout", (route: Route) =>
    route.fulfill({
      status: 200,
      headers: {
        "Set-Cookie":
          "ax_access_token=; Max-Age=0; Path=/; HttpOnly; SameSite=Lax",
      },
      body: "{}",
    }),
  );
}
