---
id: SPEC-AX-E2E-001
version: 0.1.0
status: draft
created: 2026-05-21
updated: 2026-05-21
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-21): SPEC-AX-WEB-001(merged, Next.js 14+ 대시보드, 7 화면, BFF + HttpOnly 쿠키)의 **Playwright E2E 자동화** 첫 초안. 본 SPEC은 `apps/web/`의 골든 패스(인증 흐름·역할별 권한·로그아웃)를 브라우저-레벨에서 검증하는 테스트 슈트를 신규 디렉터리 `apps/web/e2e/`에 추가한다. SUT는 SPEC-AX-WEB-001이 빌드한 Next.js 앱이며, 백엔드 Go control-plane(`apps/control-plane/`) 및 SPEC-AX-WEB-001 앱 소스(`apps/web/src/**`)는 [HARD] 0-diff(consumer-only). Keycloak 24.x 의존 회피를 위해 OIDC 콜백을 Playwright `page.route` 인터셉트로 모킹하고, 역할별 cookie `storageState` fixture로 세션을 주입한다. 시각 회귀·부하·CI/CD 파이프라인 통합은 §3 비목표(별도 SPEC). (작성자: ircp)

> Schema note: YAML frontmatter는 16번째 SPEC(SPEC-AX-WEB-001 포함 누적)과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. 본 SPEC은 frontend 테스트 디렉터리(`apps/web/e2e/`)를 신규 추가하는 greenfield 작업이므로 phantom-API 검증 부담은 SUT 페이지 경로/쿠키 이름/BFF 엔드포인트에 한정되며, 모두 SPEC-AX-WEB-001 실제 코드(`apps/web/src/middleware.ts`, `apps/web/src/app/(dashboard)/**`, `apps/web/src/app/api/**`)에서 직접 검증된 사실에 근거한다.

---

# SPEC-AX-E2E-001 — apps/web 대시보드 Playwright E2E 자동화 (Playwright E2E Test Automation for Web Dashboard)

## 1. 개요 (Overview)

SPEC-AX-WEB-001이 빌드한 Next.js 14+ App Router 대시보드(`apps/web/`)의 **사용자 경로(golden path)를 브라우저-레벨에서 자동 검증**하는 Playwright 기반 E2E 테스트 슈트를 신규 디렉터리 `apps/web/e2e/`에 추가한다. 본 SPEC은 SPEC-AX-WEB-001의 **순수 consumer**이며 SUT 코드(`apps/web/src/**`)와 백엔드 Go control-plane(`apps/control-plane/**`)·Keycloak 설정·DB 스키마를 **일절 수정하지 않는다 — 백엔드/프런트엔드 0-diff(HARD)**. 추가 산출물은 `apps/web/e2e/` 테스트 디렉터리, `apps/web/playwright.config.ts`, `apps/web/.env.test.example`, 그리고 `apps/web/package.json`의 `devDependencies`에 `@playwright/test`(+ 필요 시 `msw`) 추가뿐이다.

### 1.1 본 SPEC의 의미와 PoC 범위

본 SPEC의 1차 산출물은 **자동 재현 가능한 5개 골든 패스 E2E 시나리오 + 역할별 RBAC 가시성 검증 + 로그아웃 흐름 검증**이다. 시나리오 그룹:

1. **인증 흐름**: 미인증 → `/dashboard/evidence` → `/login` 리다이렉트 → PKCE 모의 로그인 콜백 → `/dashboard/evidence` 진입
2. **viewer 역할**: 증빙 목록·평가 항목 트리·리포트·리뷰 화면을 **읽기 전용**으로 시인. 점수 입력 폼·루브릭·감사 로그 메뉴/페이지 비노출 또는 비활성화
3. **analyst 역할**: 증빙 업로드 폼·점수 입력 폼·리뷰 제출 폼 가시 및 제출 성공. 감사 로그/루브릭 페이지 진입 시 차단 또는 비표시
4. **admin 역할**: 감사 로그 페이지 접근·루브릭 임계값 편집·리뷰어 배정·리뷰 승인 흐름 성공
5. **로그아웃**: 쿠키 삭제 후 보호 경로 재접근 시 `/login` 리다이렉트

PoC 데모 보강이므로 다음은 의식적으로 **간소화**한다:
- 시각 회귀(visual snapshot diff)·픽셀 비교 미도입(§3 비목표)
- 부하·동시성·k6 등 성능 측정 미도입(§3 비목표)
- CI/CD 워크플로 파일(.github/workflows/*) 통합은 §3 비목표(별도 SPEC)
- 다국어 테스트는 한국어 단일(SUT가 한국어 단일 언어이므로)

### 1.2 Anchor 컨텍스트

본 SPEC은 16번째 SPEC(SPEC-AX-WEB-001) 완료(main 머지) 후 후속이며, **SUT는 `apps/web/`의 빌드된 Next.js 앱**이다. SPEC-AX-WEB-001의 71개 신규 파일이 형성한 7개 보호 경로(`/dashboard/{evidence,evaluation-items,scores,reports,reviews,audit-logs,rubric}`)·1개 로그인 경로(`/login`)·12개 BFF Route Handler(`/api/auth/{login,callback,logout,refresh}` + `/api/v1/{evidences,evaluation-items,scores,reports/category/[id],reviews,audit-logs,rubric/thresholds}` 외)가 본 SPEC의 검증 대상 표면이다. 두 SPEC의 책임은 분리된다: SPEC-AX-WEB-001은 화면을 만들고, 본 SPEC은 화면이 명세대로 동작함을 자동 증명한다.

### 1.3 검증 대상 SUT 경로 카탈로그 (SPEC-AX-WEB-001 실제 코드에서 검증)

| 카테고리 | 경로/파일 | 본 SPEC 사용처 |
|----------|-----------|----------------|
| 공개 페이지 | `/login` (apps/web/src/app/login/page.tsx) | AC-AUTH-001/002, 인증 흐름 진입점 |
| 보호 페이지 | `/dashboard/evidence` | AC-VIS-VIEWER-001, AC-VIS-ANALYST-001 |
| 보호 페이지 | `/dashboard/evaluation-items` | AC-VIS-ALL-001 |
| 보호 페이지 | `/dashboard/scores` | AC-VIS-VIEWER-002(폼 비노출), AC-FLOW-ANALYST-001(제출) |
| 보호 페이지 | `/dashboard/reports` | AC-VIS-ALL-002 |
| 보호 페이지 | `/dashboard/reviews` | AC-VIS-VIEWER-003(승인 버튼 비노출), AC-FLOW-ADMIN-002 |
| 보호 페이지 | `/dashboard/audit-logs` | AC-VIS-ADMIN-001, viewer/analyst 차단 AC-RBAC-DENY-001 |
| 보호 페이지 | `/dashboard/rubric` | AC-VIS-ADMIN-002, viewer/analyst 차단 AC-RBAC-DENY-002 |
| 미들웨어 가드 | `apps/web/src/middleware.ts` (`COOKIE_ACCESS_TOKEN` 부재 시 `/login` 리다이렉트, matcher `/dashboard/:path*`) | AC-AUTH-001(미인증 리다이렉트) |
| 인증 BFF | `POST /api/auth/login` (PKCE 시작) | 인증 흐름 시작 |
| 인증 BFF | `GET /api/auth/callback` (토큰 교환 → HttpOnly 쿠키 set) | Playwright `page.route` 모킹 대상 |
| 인증 BFF | `POST /api/auth/logout` (쿠키 삭제) | AC-AUTH-LOGOUT-001 |
| 데이터 BFF | `GET/POST /api/v1/evidences`, `GET /api/v1/evaluation-items`, `GET/POST /api/v1/scores`, `GET /api/v1/reports/category/{id}`, `GET/POST /api/v1/reviews` 외 | analyst/admin 흐름의 백엔드 호출 — 본 SPEC은 실제 backend·mock 둘 다 §6 OPEN #2에서 결정 |

phantom path 0건 [HARD]: 본 SPEC `apps/web/e2e/`에서 참조하는 모든 URL/쿠키 이름/페이지 셀렉터는 SPEC-AX-WEB-001 실제 코드에서 직접 확인된 것으로 한정된다. SUT 코드 변경 없이 셀렉터가 필요하면 SPEC-AX-WEB-001의 후속 SPEC(예: `data-testid` 도입)을 선행해야 한다 — 본 SPEC은 그 후속이 아니라 **현재 SUT 상태 그대로** 검증한다.

### 1.4 권한 경계 — RBAC 3-역할 검증 매트릭스

본 SPEC은 SPEC-AX-AUTH-001/002/003·SPEC-AX-WEB-001 §1.4가 정의한 권한 모델이 UI에서 정확히 반영됨을 검증한다:

| 역할 | 가시 | 비가시/차단 | 검증 AC |
|------|------|--------------|----------|
| viewer | evidence/evaluation-items/reports/reviews(읽기 전용), 사이드바 viewer 메뉴 | 점수 입력 폼, 증빙 업로드 버튼, 리뷰 제출 버튼, 리뷰 승인/반려, audit-logs 메뉴/페이지, rubric 메뉴/페이지 | AC-VIS-VIEWER-001~003, AC-RBAC-DENY-001/002 |
| analyst | viewer 가시 항목 + 증빙 업로드, 점수 입력/수정, 리뷰 제출 | 리뷰 승인/반려 버튼, audit-logs 메뉴/페이지, rubric 메뉴/페이지 | AC-VIS-ANALYST-001, AC-FLOW-ANALYST-001/002 |
| admin | analyst 가시 항목 + 리뷰 승인/반려/배정, audit-logs 페이지, rubric 임계값 편집 | (없음 — 모든 화면 접근) | AC-VIS-ADMIN-001/002, AC-FLOW-ADMIN-001~003 |

권한 강제는 **백엔드가 신뢰의 원천**이라는 SPEC-AX-WEB-001 §1.4 원칙을 그대로 따른다. 본 SPEC은 그 결과(UI 가시성 + 403 toast)를 검증할 뿐, 권한 모델을 새로 정의하지 않는다.

### 1.5 의존성 stub 계약 — consumer-only [핵심, load-bearing]

본 SPEC은 SPEC-AX-WEB-001 + 백엔드 15 SPEC 누적의 **순수 테스트 consumer**이다. 다음이 **HARD 계약**이다:

- **[HARD]** `apps/control-plane/**`(Go 코드, 마이그레이션, audit, RBAC/ABAC, OpenAPI/proto)를 **일절 수정하지 않는다**.
- **[HARD]** `pipelines/**`(Python AI 파이프라인)도 수정하지 않는다.
- **[HARD]** `apps/web/src/**`(SPEC-AX-WEB-001이 빌드한 Next.js 앱 소스 71개 파일)도 **수정하지 않는다**. `data-testid` 누락으로 인한 셀렉터 불안정성은 본 SPEC이 텍스트/role 기반 셀렉터로 우회하며, 셀렉터 안정화가 필수가 되면 SPEC-AX-WEB-002(가칭)를 선행한다.
- **[HARD]** `apps/web/src/middleware.ts`의 matcher(`/dashboard/:path*`)·쿠키 이름(`COOKIE_ACCESS_TOKEN`)·BFF 경로(`/api/auth/*`, `/api/v1/*`)를 본 SPEC이 변경 요구하지 않는다.
- **[HARD]** Keycloak 24.x 설정(SPEC-AX-AUTH-001) 무수정 — 본 SPEC은 OIDC 콜백을 Playwright `page.route`로 인터셉트해 모킹하므로 **테스트 실행 시 라이브 Keycloak 불필요**(§6 OPEN #1 RESOLVED 후 확정).
- **[HARD]** 신규 백엔드/프런트 외부 의존 0건 — `apps/control-plane/go.mod`·`apps/web/src/**` 무수정. 신규 의존은 `apps/web/package.json` `devDependencies`에 한정(`@playwright/test`, 선택적 `msw`).
- **신규 디렉터리/파일만 추가**: `apps/web/e2e/` 디렉터리, `apps/web/playwright.config.ts`, `apps/web/.env.test.example`, 그리고 `apps/web/package.json`에 `test:e2e` 스크립트 1줄 + `devDependencies` 1~2줄 추가. 루트 `package.json` 무수정.

### 1.6 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `E2E` (End-to-End 테스트 자동화)
- 3차 책임: SPEC-AX-WEB-001의 골든 패스를 Playwright로 자동 재현 — 시각 회귀·부하·CI 통합은 비포함

---

## 2. 범위 (Scope)

### 2.1 IN SCOPE — 본 SPEC이 만든다

1. **Playwright 러너 설정**: `apps/web/playwright.config.ts`(브라우저 1종 우선 — Chromium; Firefox/WebKit은 §6 OPEN #3에서 결정), 베이스 URL(`PLAYWRIGHT_BASE_URL`, 기본 `http://localhost:3000`), `apps/web/e2e/` 테스트 디렉터리 인식, 리포터 설정(list + html).
2. **인증 모킹 인프라**: Keycloak OIDC 콜백(`/api/auth/callback`) 응답 인터셉트 또는 직접 쿠키 주입을 통한 역할별 `storageState` 생성. 3개 storage state 파일(`apps/web/e2e/.auth/{viewer,analyst,admin}.json`) — git ignore.
3. **공유 fixture**: 역할별 인증된 `page` fixture(`viewerPage`/`analystPage`/`adminPage`), BFF 응답 모킹 헬퍼(§6 OPEN #2 결정).
4. **5개 시나리오 그룹 테스트**:
   - 그룹 A — 인증 흐름(`apps/web/e2e/auth.spec.ts`): 미인증 리다이렉트, 콜백 후 쿠키 set, 보호 페이지 진입
   - 그룹 B — viewer RBAC(`apps/web/e2e/rbac-viewer.spec.ts`): 7개 화면 가시성 + 차단된 페이지 직접 URL 접근
   - 그룹 C — analyst 흐름(`apps/web/e2e/flow-analyst.spec.ts`): 증빙 업로드, 점수 입력, 리뷰 제출
   - 그룹 D — admin 흐름(`apps/web/e2e/flow-admin.spec.ts`): 감사 로그 조회, 루브릭 임계값 편집, 리뷰어 배정/승인
   - 그룹 E — 로그아웃(`apps/web/e2e/logout.spec.ts`): 로그아웃 후 쿠키 삭제 검증 + 보호 페이지 재접근 차단
5. **환경 변수 템플릿**: `apps/web/.env.test.example`(커밋 가능) — `PLAYWRIGHT_BASE_URL`, `E2E_MOCK_MODE`(`network` 또는 `inject`), Keycloak 토큰 시드(테스트 전용 JWT 페이로드 — 실제 Keycloak 시크릿 미포함). 실제 `apps/web/.env.test`는 `.gitignore`(별도 수정 불필요 — Next.js 표준이 이미 제외).
6. **package.json 스크립트**: `apps/web/package.json`에 `"test:e2e": "playwright test"`, `"test:e2e:ui": "playwright test --ui"` 추가, `devDependencies`에 `@playwright/test` 추가. 루트 `package.json` 무수정.
7. **README**: `apps/web/e2e/README.md`(테스트 실행 방법, 모킹 모드 설명, 새 시나리오 추가 가이드).

### 2.2 OUT OF SCOPE — 본 SPEC이 만들지 않는다

1. **시각 회귀 테스트** (Playwright `toHaveScreenshot()`, Percy, Chromatic 등) — 별도 SPEC.
2. **부하/성능 테스트** (k6, Artillery) — 별도 SPEC.
3. **CI/CD 통합** (.github/workflows/playwright.yml, GitHub Actions 매트릭스, Docker 컴포즈 기반 백엔드 자동 부트스트랩) — 별도 SPEC. 본 SPEC은 **로컬 실행 가능성**만 보장한다.
4. **다국어/i18n 테스트** — SUT가 한국어 단일이므로 불필요.
5. **모바일 뷰포트/반응형 테스트** — SUT가 데스크톱 1280×800 first이므로 §3 비목표.
6. **백엔드 라이브 의존 통합 테스트** — Go control-plane·PostgreSQL·MinIO·Keycloak을 모두 띄우는 통합 시나리오는 본 SPEC의 책임이 아님. 본 SPEC은 OIDC 콜백 + 백엔드 API 응답을 §6 OPEN #2 결정에 따라 모킹한다.
7. **접근성(a11y) 자동 검사** (axe-playwright) — 별도 SPEC.
8. **데이터 시드/팩토리 도구** (faker, factory-bot 등) — 본 SPEC은 inline 픽스처 JSON 또는 hand-written 모의 응답으로 충분.
9. **SUT 코드 변경**(`apps/web/src/**`) — `data-testid` 추가 요구가 발견되어도 본 SPEC은 추가하지 않으며, 별도 SPEC(SPEC-AX-WEB-002 가칭)으로 이관한다.
10. **Storybook/컴포넌트 단위 테스트** — Vitest는 SPEC-AX-WEB-001 이미 도입 완료, 본 SPEC은 E2E만 다룬다.

---

## 3. 목표 / 비목표 (Goals / Non-Goals)

### 3.1 Goals

- **G1**: SPEC-AX-WEB-001의 5개 골든 패스(인증·viewer·analyst·admin·로그아웃)를 Playwright로 100% 자동 재현 가능하게 만든다.
- **G2**: 라이브 Keycloak 의존 없이 로컬 머신에서 단일 명령(`pnpm --filter @iroum-ax/web test:e2e` 또는 `npm run test:e2e`)으로 모든 E2E 시나리오를 실행할 수 있다.
- **G3**: 백엔드/프런트 SUT 코드 0-diff 유지 — 본 SPEC 머지 후 `apps/web/src/**`·`apps/control-plane/**` 변경 0건임을 git diff로 입증 가능.
- **G4**: 새 시나리오 추가 비용을 낮춘다 — 역할별 `storageState`와 BFF 모킹 헬퍼를 fixture로 공통화하여, 단일 spec 파일 추가로 시나리오 1건 확장.
- **G5**: 테스트가 SUT의 한국어 텍스트(예: "로그인", "증빙 업로드", "감사 로그")를 셀렉터로 사용해도 안정적으로 동작 — `data-testid` 부재를 텍스트/role 기반 셀렉터로 우회.

### 3.2 Non-Goals (재강조)

- **NG1**: SUT(`apps/web/src/**`) 셀렉터 안정화를 위한 `data-testid` 추가 — 본 SPEC은 0-diff를 유지하며 셀렉터 안정성은 페이지 텍스트/`getByRole`/`getByLabel`로 우회.
- **NG2**: 실제 PostgreSQL·MinIO 인스턴스를 띄워 증빙 파일 업로드를 검증 — BFF 응답 모킹으로 대체.
- **NG3**: 백엔드 Go 에러 응답 형식(예: 403/404 본문 구조) 신규 정의 — 백엔드가 반환하는 그대로 사용하며, 본 SPEC에서 변경 요구하지 않음.
- **NG4**: 시각적 픽셀 비교(visual regression).
- **NG5**: 보안 침투 테스트(SSRF, XSS, CSRF 페이로드 주입 검증) — 별도 SPEC(SPEC-AX-SEC-E2E-001 가칭).

---

## 4. 요구사항 (Requirements — EARS 형식)

### 4.1 Ubiquitous Requirements (시스템 상시)

- **REQ-E2E-001a (Ubiquitous)**: The E2E 테스트 슈트 **shall** reside in `apps/web/e2e/` and use `@playwright/test` as the runner.
- **REQ-E2E-001b (Ubiquitous)**: The E2E 테스트 슈트 **shall not** modify any file under `apps/control-plane/**`, `pipelines/**`, or `apps/web/src/**` — 0-diff against SUT.
- **REQ-E2E-001c (Ubiquitous)**: The E2E 테스트 슈트 **shall** be runnable locally without a live Keycloak or live Go control-plane instance (via OIDC callback interception and BFF response mocking per §6 OPEN #2).
- **REQ-E2E-001d (Ubiquitous)**: All page selectors **shall** rely on Playwright's text/role/label-based selectors (`getByRole`, `getByLabel`, `getByText`) — `data-testid` MUST NOT be required of the SUT.
- **REQ-E2E-001e (Ubiquitous)**: Test-time secrets (e.g., Keycloak realm export, BFF mock JWT private keys if any) **shall not** be committed; `apps/web/.env.test` is git-ignored and `apps/web/.env.test.example` is the public template.

### 4.2 Event-Driven Requirements (트리거-응답)

- **REQ-E2E-AUTH-010 (Event)**: **WHEN** Playwright opens `http://localhost:3000/dashboard/evidence` without the `ax_access_token` cookie, **THEN** the test **shall** observe an HTTP redirect to `/login?from=%2Fdashboard%2Fevidence` (matching `apps/web/src/middleware.ts` line 22-26 behavior).
- **REQ-E2E-AUTH-011 (Event)**: **WHEN** the Playwright test completes the mock OIDC callback flow, **THEN** the browser context **shall** hold an HttpOnly `ax_access_token` cookie and the next navigation to `/dashboard/evidence` **shall** render the evidence list page (status 200, no redirect).
- **REQ-E2E-VIEWER-020 (Event)**: **WHEN** a Playwright test with the `viewer` `storageState` navigates to `/dashboard/evidence`, **THEN** the page **shall** render the evidence list **without** an "업로드" / "Upload" submit button visible to viewers.
- **REQ-E2E-ANALYST-030 (Event)**: **WHEN** a Playwright test with the `analyst` `storageState` submits the score form on `/dashboard/scores` with a valid mocked BFF response, **THEN** the test **shall** observe a success toast (or equivalent confirmation element) and the network log **shall** include a `POST /api/v1/scores` request.
- **REQ-E2E-ADMIN-040 (Event)**: **WHEN** a Playwright test with the `admin` `storageState` clicks the "승인" button on a pending review at `/dashboard/reviews`, **THEN** the test **shall** observe a `POST /api/v1/reviews/{id}/approve` request and a UI state transition reflecting the approved status (matching SPEC-AX-REVIEW-001 4-state machine).
- **REQ-E2E-LOGOUT-050 (Event)**: **WHEN** a Playwright test triggers logout (sidebar logout button or `POST /api/auth/logout`), **THEN** the `ax_access_token` cookie **shall** be cleared and any subsequent navigation to `/dashboard/*` **shall** trigger the `/login` redirect from REQ-E2E-AUTH-010.

### 4.3 State-Driven Requirements (조건적 동작)

- **REQ-E2E-VIS-100 (State)**: **WHILE** the active session is `viewer`, the test **shall** assert that the sidebar navigation does not link to `/dashboard/audit-logs` and that direct URL access to `/dashboard/audit-logs` results in either a 403 toast, a "권한 없음" message, or a redirect — whichever SPEC-AX-WEB-001 implemented (the test reads the actual SUT behavior, not a prescribed contract).
- **REQ-E2E-VIS-101 (State)**: **WHILE** the active session is `analyst`, the test **shall** assert that the rubric page (`/dashboard/rubric`) link is hidden or disabled in navigation and that direct access yields the same SUT-implemented denial behavior as REQ-E2E-VIS-100.
- **REQ-E2E-VIS-102 (State)**: **WHILE** the active session is `admin`, the test **shall** assert that the rubric and audit-logs pages render without any access-denial UI.

### 4.4 Optional Requirements (Where-feature-exists)

- **REQ-E2E-OPT-200 (Optional, Where)**: **WHERE** Playwright's trace viewer is enabled in config (`trace: 'on-first-retry'`), the test **shall** retain a trace file on the first failure for local debugging — production runs may disable traces.
- **REQ-E2E-OPT-201 (Optional, Where)**: **WHERE** an environment variable `E2E_LIVE_BACKEND=1` is set, the test **may** be re-run against a live Go control-plane (per §6 OPEN #2 dual-mode resolution) — this mode is best-effort and not part of the default `pnpm test:e2e` invocation.

### 4.5 Unwanted Behavior Requirements (금지)

- **REQ-E2E-NO-300 (Unwanted)**: The test suite **shall not** require any modification to `apps/web/src/**` (no `data-testid` additions, no helper exports for tests) — if a selector cannot be stabilized without SUT changes, the failing test **shall** be marked `test.fixme` with an explicit reference to a follow-up SPEC issue, never silently dropped.
- **REQ-E2E-NO-301 (Unwanted)**: The test suite **shall not** commit real Keycloak client secrets, real backend JWT signing keys, or PII test fixtures.
- **REQ-E2E-NO-302 (Unwanted)**: The test suite **shall not** introduce new backend dependencies (`go.mod`, `pyproject.toml`) or modify `apps/web/src/middleware.ts` matcher / cookie name to make tests easier.
- **REQ-E2E-NO-303 (Unwanted)**: The test suite **shall not** include time predictions (e.g., `expect(...).toCompleteWithin(2000)`) tied to real network latency — only deterministic mocks may set timing expectations.

### 4.6 Exclusions (What NOT to Build) [HARD]

본 SPEC은 다음을 **만들지 않으며 후속 SPEC에서도 본 SPEC 범위로 흡수하지 않는다**:

- **EXC-1**: 시각 회귀 테스트(`toHaveScreenshot`, Percy, Chromatic)
- **EXC-2**: 부하/성능 테스트(k6, Artillery, Lighthouse CI)
- **EXC-3**: GitHub Actions/GitLab CI 워크플로 파일(`.github/workflows/**`) — 본 SPEC은 로컬 실행만 보장
- **EXC-4**: 모바일 뷰포트/반응형 검증(iPhone/iPad/Android 디바이스 에뮬레이션)
- **EXC-5**: 접근성(a11y) 자동 검사(axe-core, Pa11y)
- **EXC-6**: SUT(`apps/web/src/**`) 셀렉터 안정화를 위한 `data-testid` 추가 — 별도 SPEC(SPEC-AX-WEB-002 가칭) 필요
- **EXC-7**: 라이브 Keycloak/Postgres/MinIO/Go control-plane을 부트스트랩하는 docker-compose 통합 (`E2E_LIVE_BACKEND=1` 모드는 REQ-E2E-OPT-201처럼 best-effort 옵션이며, 본 SPEC의 합격 조건이 아님)
- **EXC-8**: 영구 테스트 데이터베이스 시드/마이그레이션 도구
- **EXC-9**: 침투 테스트(XSS/CSRF/SSRF 페이로드 주입)
- **EXC-10**: 다국어/i18n 테스트(SUT가 한국어 단일이므로)

---

## 5. 수락 기준 (Acceptance Criteria)

각 AC는 Given-When-Then 형식이며, 모든 AC는 `apps/web/e2e/` 하위 spec 파일 실행으로 자동 검증 가능해야 한다.

### 5.1 그룹 A — 인증 흐름 (auth.spec.ts)

- **AC-AUTH-001 — 미인증 보호 페이지 접근 시 로그인 리다이렉트**
  - **Given** Playwright 브라우저 컨텍스트에 `ax_access_token` 쿠키가 없다
  - **When** `/dashboard/evidence` 로 직접 이동한다
  - **Then** 응답은 `/login?from=%2Fdashboard%2Fevidence` 로 리다이렉트되고, 페이지 URL은 `/login` 으로 끝나며, "로그인" 버튼 또는 Keycloak SSO 진입 요소가 가시화된다

- **AC-AUTH-002 — 모의 OIDC 콜백 후 보호 페이지 진입**
  - **Given** Playwright가 `/api/auth/callback?code=...&state=...` 요청을 인터셉트해 모의 토큰 응답으로 답한다 (또는 `storageState`로 viewer 쿠키를 사전 주입한다)
  - **When** `/dashboard/evidence` 로 이동한다
  - **Then** HTTP 상태 200으로 evidence 목록 페이지가 렌더되며 `/login` 으로의 리다이렉트는 발생하지 않는다

- **AC-AUTH-003 — 만료 쿠키 시 재로그인 유도**
  - **Given** `ax_access_token` 쿠키 값이 만료된 JWT 페이로드를 담고 있다 (BFF가 만료 검사 시)
  - **When** `/dashboard/evidence` 로 이동한다
  - **Then** SUT-implemented 동작에 따라 `/login` 리다이렉트 또는 `/api/auth/refresh` 호출 후 정상 진입 중 하나가 발생한다 (테스트는 둘 중 어느 것이든 허용하며, 결과를 트레이스에 기록한다)

### 5.2 그룹 B — viewer RBAC (rbac-viewer.spec.ts)

- **AC-VIS-VIEWER-001 — 증빙 목록 읽기 전용**
  - **Given** Playwright 세션이 `viewer` storageState로 로드되었다
  - **When** `/dashboard/evidence` 로 이동한다
  - **Then** 증빙 목록 테이블이 렌더되고, "업로드" 버튼은 가시되지 않거나 `disabled` 속성을 갖는다

- **AC-VIS-VIEWER-002 — 점수 입력 폼 비노출**
  - **Given** Playwright 세션이 `viewer` storageState로 로드되었다
  - **When** `/dashboard/scores` 로 이동한다
  - **Then** 점수 목록(있다면)은 가시되나 "점수 입력" / "제출" 버튼·폼은 가시되지 않는다

- **AC-VIS-VIEWER-003 — 리뷰 승인 버튼 비노출**
  - **Given** Playwright 세션이 `viewer` storageState로 로드되었다
  - **When** `/dashboard/reviews` 로 이동한다
  - **Then** 리뷰 목록은 가시되나 "승인" / "반려" 버튼은 가시되지 않는다

- **AC-RBAC-DENY-001 — viewer의 감사 로그 직접 URL 접근 차단**
  - **Given** Playwright 세션이 `viewer` storageState로 로드되었다
  - **When** `/dashboard/audit-logs` 로 직접 URL 이동한다
  - **Then** SUT가 구현한 차단 방식 중 하나가 관찰된다: (a) 페이지 진입은 되었으나 "권한 없음" 메시지 표시, (b) `/dashboard/evidence` 등 기본 경로로 리다이렉트, (c) 사이드바에서 메뉴 자체가 비노출되어 직접 URL 시 빈 결과 — 테스트는 셋 중 어느 것이든 허용하되, 입력 폼·관리 액션이 가시되지 않음만 확실히 검증한다

- **AC-RBAC-DENY-002 — viewer의 루브릭 페이지 차단**
  - AC-RBAC-DENY-001과 동일 패턴, 대상은 `/dashboard/rubric`

### 5.3 그룹 C — analyst 흐름 (flow-analyst.spec.ts)

- **AC-VIS-ANALYST-001 — 증빙 업로드 폼 가시**
  - **Given** Playwright 세션이 `analyst` storageState로 로드되었다
  - **When** `/dashboard/evidence` 로 이동한다
  - **Then** "업로드" 버튼/`<input type="file">` 폼이 가시되고 활성 상태이다

- **AC-FLOW-ANALYST-001 — 점수 제출 흐름**
  - **Given** `analyst` 세션 + `POST /api/v1/scores` BFF 응답 모킹(`200 OK` + 모의 score id)
  - **When** `/dashboard/scores` 에서 evaluation_item_id·점수·코멘트를 입력하고 제출 버튼을 누른다
  - **Then** 네트워크 트레이스에 `POST /api/v1/scores` 요청이 기록되고, UI는 성공 toast 또는 목록 갱신을 표시한다

- **AC-FLOW-ANALYST-002 — 리뷰 제출 흐름**
  - **Given** `analyst` 세션 + `POST /api/v1/reviews` 모킹
  - **When** `/dashboard/reviews/new` 또는 동등한 진입점에서 리뷰를 제출한다
  - **Then** `POST /api/v1/reviews` 요청이 발생하고, 응답 상태가 4-state machine 의 `SUBMITTED`(SPEC-AX-REVIEW-001 정의)로 반영된다

- **AC-FLOW-ANALYST-DENY-001 — analyst의 audit-logs 차단**
  - AC-RBAC-DENY-001과 동일 패턴, viewer 자리에 analyst — analyst도 audit-logs 접근 차단이 SPEC-AX-AUDIT-QUERY-001 admin-only narrowing에 의해 강제됨

### 5.4 그룹 D — admin 흐름 (flow-admin.spec.ts)

- **AC-VIS-ADMIN-001 — 감사 로그 페이지 접근**
  - **Given** `admin` storageState + `GET /api/v1/audit-logs` BFF 응답 모킹(샘플 5건)
  - **When** `/dashboard/audit-logs` 로 이동한다
  - **Then** 감사 로그 테이블이 렌더되고 모킹된 5건이 표시된다

- **AC-VIS-ADMIN-002 — 루브릭 페이지 접근**
  - **Given** `admin` storageState + `GET /api/v1/rubric/thresholds` 모킹
  - **When** `/dashboard/rubric` 으로 이동한다
  - **Then** 임계값 목록/편집 폼이 렌더된다

- **AC-FLOW-ADMIN-001 — 루브릭 임계값 편집**
  - **Given** `admin` 세션 + `PUT /api/v1/rubric/thresholds/{scope}` 모킹(`200 OK`)
  - **When** 기존 임계값을 편집하고 저장 버튼을 누른다
  - **Then** `PUT /api/v1/rubric/thresholds/{scope}` 요청이 발생하고, 성공 toast 또는 갱신된 값이 표시된다

- **AC-FLOW-ADMIN-002 — 리뷰 승인**
  - **Given** `admin` 세션 + 모킹된 pending 리뷰 1건 + `POST /api/v1/reviews/{id}/approve` 모킹
  - **When** "승인" 버튼을 누른다
  - **Then** approve 요청이 발생하고, UI 상의 리뷰 상태가 `APPROVED`로 전환된다

- **AC-FLOW-ADMIN-003 — 리뷰어 배정**
  - **Given** `admin` 세션 + `POST /api/v1/reviews/{id}/assign-reviewer` 모킹
  - **When** 리뷰어를 선택하고 배정 버튼을 누른다
  - **Then** assign-reviewer 요청이 발생하고, UI가 배정된 리뷰어를 표시한다

### 5.5 그룹 E — 로그아웃 (logout.spec.ts)

- **AC-AUTH-LOGOUT-001 — 로그아웃 후 쿠키 삭제**
  - **Given** 임의의 역할(viewer/analyst/admin) storageState로 보호 페이지에 진입했다
  - **When** 사이드바의 "로그아웃" 버튼을 누르거나 `POST /api/auth/logout` 을 직접 호출한다
  - **Then** `ax_access_token` 쿠키가 응답 헤더 `Set-Cookie`의 `Max-Age=0` 또는 삭제 지시로 제거되고, 브라우저 컨텍스트의 쿠키 jar에서 사라진다

- **AC-AUTH-LOGOUT-002 — 로그아웃 후 보호 페이지 재접근 차단**
  - **Given** AC-AUTH-LOGOUT-001 이후 상태
  - **When** `/dashboard/evidence` 로 이동한다
  - **Then** `/login?from=%2Fdashboard%2Fevidence` 로 리다이렉트된다 (AC-AUTH-001 재현)

### 5.6 인프라 AC

- **AC-INF-001 — 0-diff 입증**
  - **Given** 본 SPEC 머지 직전 커밋
  - **When** `git diff main -- apps/control-plane/ apps/web/src/ pipelines/` 을 실행한다
  - **Then** 출력은 비어 있다(0 줄 diff)

- **AC-INF-002 — 단일 명령 실행**
  - **Given** `apps/web/` 에서 `npm install` (또는 `pnpm install`) 후 `npx playwright install chromium` 완료 상태
  - **When** `npm run test:e2e` (또는 동등 명령) 을 실행한다
  - **Then** 모든 그룹 A~E 시나리오가 통과하고 종료 코드 0을 반환한다 (백엔드 라이브 의존 없이)

- **AC-INF-003 — 시크릿 미커밋**
  - **Given** 본 SPEC PR
  - **When** `git diff` 를 검사한다
  - **Then** 실제 JWT signing key·실제 Keycloak client secret·실제 사용자 이메일/이름이 포함되지 않으며, `apps/web/.env.test.example` 만 커밋되고 `apps/web/.env.test` 는 git ignore 상태이다

총 AC 수: **20건** (그룹 A: 3, 그룹 B: 5, 그룹 C: 4, 그룹 D: 5, 그룹 E: 2, 인프라: 3 — 단, 그룹 B 1건이 RBAC-DENY-002를 포함하여 5건이며, 그룹 D는 5건임을 명시)

---

## 6. OPEN ITEMS (수락 전 결정 필요)

### OPEN #1 — OIDC 콜백 모킹 방식 [draft] [Resolve before /moai run]

선택지:
- **A (권장)**: Playwright `page.route('/api/auth/callback*', ...)` 인터셉트로 BFF 응답을 모킹하고, BFF가 `Set-Cookie: ax_access_token=...` 헤더를 반환하도록 위장한다. `apps/web/src/app/api/auth/callback/route.ts` 는 변경하지 않는다.
- **B**: `msw`(Mock Service Worker)를 Playwright 컨텍스트에 부착해 콜백뿐 아니라 `/api/v1/*` BFF 호출까지 통합 모킹한다. 추가 의존(`msw`) 1개.
- **C**: 직접 쿠키 주입 — `BrowserContext.addCookies([{name: 'ax_access_token', value: '<mock-jwt>', httpOnly: true, ...}])` 만 사용하고 `/api/auth/*` 는 호출 안 한다. 콜백 흐름(AC-AUTH-002) 자체 검증은 부분 포기.

권고: **A** — 콜백 흐름까지 검증 가능하면서 의존 추가 0. B는 BFF 데이터 호출 모킹까지 필요해지면 §6 OPEN #2 결정과 함께 재검토. C는 AC-AUTH-002 충실도가 낮아 부적합.

### OPEN #2 — BFF 데이터 호출(`/api/v1/*`) 모킹 vs 라이브 [draft] [Resolve before /moai run]

선택지:
- **A (권장)**: Playwright `page.route('/api/v1/**', ...)` 으로 BFF 응답을 모킹. 신규 의존 0. 시나리오별 모의 응답을 `apps/web/e2e/fixtures/api/*.json` 으로 관리.
- **B**: `msw` 도입(OPEN #1 B와 일관). 단일 도구로 콜백 + 데이터 호출 통합 모킹.
- **C**: `E2E_LIVE_BACKEND=1` 모드만 지원하고 백엔드 라이브 실행 의존. REQ-E2E-001c 위반 — 기각.
- **D**: 듀얼 모드 — 기본은 A(모킹), 옵션으로 라이브 백엔드 모드 지원(REQ-E2E-OPT-201). 기본 명령은 A로 통과되어야 함.

권고: **D** — A를 기본 동작으로 보장하면서 라이브 모드는 best-effort 옵션. C는 G2에 위배.

### OPEN #3 — Playwright 브라우저 매트릭스 [draft] [Resolve before /moai run]

선택지:
- **A (권장)**: Chromium 단일 — PoC 데모용으로 충분, 실행 시간 최소.
- **B**: Chromium + Firefox — 브라우저 호환성 일부 검증.
- **C**: Chromium + Firefox + WebKit — 풀 매트릭스, 실행 시간 약 3배.

권고: **A** — PoC 범위 일치. CI 통합 SPEC에서 매트릭스 확장.

### OPEN #4 — analyst의 audit-logs 차단 동작 검증 강도 [draft] [Resolve before /moai run]

배경: SPEC-AX-WEB-001은 viewer/analyst의 audit-logs 메뉴를 사이드바에서 비노출 처리할 수도 있고, 직접 URL 시 403 toast를 띄울 수도 있다. 실제 구현이 어느 쪽인지 본 SPEC 작성 시점에 확정되지 않음. AC-RBAC-DENY-001/002·AC-FLOW-ANALYST-DENY-001은 SUT-implemented behavior를 허용 범위로 두는데, 이 유연성이 너무 넓은지 결정 필요.

선택지:
- **A (권장)**: 현재 허용 범위 유지(3가지 차단 방식 중 SUT가 구현한 것 무엇이든 통과). 단, 어느 방식이든 입력 폼·관리 액션은 가시되지 않아야 함을 명시.
- **B**: SUT 실제 구현을 사전 조사(Read `apps/web/src/app/(dashboard)/audit-logs/page.tsx` 등)해 단일 차단 방식을 AC에 못박는다. 0-diff 유지 가능하지만 AC가 SUT 구현 세부에 묶임.

권고: **A** — UX 차단 다양성을 SPEC이 강제하지 않고 SUT 결정을 존중. SPEC-AX-WEB-001이 향후 차단 방식을 변경해도 본 E2E SPEC AC가 자동으로 따라감.

### OPEN #5 — 테스트 모의 JWT 생성 전략 [draft] [Resolve before /moai run]

배경: 미들웨어(`apps/web/src/middleware.ts`)는 쿠키 존재만 검사하지만, 보호 페이지의 RSC가 `getServerSession()` 등에서 JWT를 디코드해 역할(`scope`)을 읽는다. 모의 토큰은 JWT 구조(`header.payload.signature`)는 갖추되 SUT가 서명 검증을 어느 강도로 하는지에 따라 전략이 갈린다.

선택지:
- **A (권장)**: SUT가 서명 검증을 우회 가능한 dev 모드(`KEYCLOAK_DEV_MOCK=true` 등 환경변수가 SPEC-AX-WEB-001 §1.4에 있다면)로 동작하고, 모의 토큰은 base64로 인코드된 JSON 페이로드 + 더미 서명. `apps/web/.env.test` 에서 dev 모드 활성화.
- **B**: 실제 RSA/ES256 키 쌍을 테스트용으로 생성(`apps/web/e2e/fixtures/keys/`, git ignore)하고 SUT 표준 JWKS 엔드포인트도 모킹. 의존 증가.
- **C**: 서명 검증 자체를 BFF 레이어에서 모킹하여 토큰 페이로드만 신뢰. 추가 가드 페이지의 검증 강도에 따라 부분 실패 가능.

권고: **A** 시도 후 SUT가 dev 모드 미지원이면 **B**로 폴백. 결정은 plan-auditor 단계에서 SPEC-AX-WEB-001 코드 조사 후 확정.

### OPEN #6 — 한국어 텍스트 셀렉터의 유지보수성 [draft] [Defer to Run phase observation]

배경: REQ-E2E-001d에 따라 `data-testid` 없이 텍스트/role 기반 셀렉터를 사용한다. SPEC-AX-WEB-001이 한국어 라벨(예: "업로드", "승인", "감사 로그")을 변경하면 본 SPEC 테스트가 깨진다.

선택지:
- **A (권장)**: 텍스트 셀렉터를 `apps/web/e2e/selectors.ts` 단일 모듈에 상수화해 라벨 변경 시 1곳 수정으로 흡수.
- **B**: SPEC-AX-WEB-002(가칭)을 선행해 SUT에 `data-testid` 추가 — 본 SPEC의 0-diff 원칙(REQ-E2E-001b) 깨짐. 기각.

권고: **A** — Run phase에서 자연스럽게 도출.

---

## 7. 기술 설계 (Technical Design)

### 7.1 디렉터리 구조 (신규 추가만)

```
apps/web/
├── playwright.config.ts                 (NEW)
├── .env.test.example                    (NEW, committed)
├── .env.test                            (NEW, .gitignore)
├── e2e/
│   ├── README.md                        (NEW)
│   ├── selectors.ts                     (NEW, 한국어 라벨 상수화)
│   ├── fixtures/
│   │   ├── auth.ts                      (NEW, role 별 storageState 생성 헬퍼)
│   │   ├── api-mocks.ts                 (NEW, BFF page.route 헬퍼)
│   │   └── api/
│   │       ├── evidences.json           (NEW, GET /api/v1/evidences 모의 응답)
│   │       ├── evaluation-items.json    (NEW)
│   │       ├── scores.json              (NEW)
│   │       ├── reviews.json             (NEW)
│   │       ├── audit-logs.json          (NEW)
│   │       └── rubric-thresholds.json   (NEW)
│   ├── .auth/                           (NEW, .gitignore — storageState 캐시)
│   │   ├── viewer.json
│   │   ├── analyst.json
│   │   └── admin.json
│   ├── auth.spec.ts                     (NEW, 그룹 A)
│   ├── rbac-viewer.spec.ts              (NEW, 그룹 B)
│   ├── flow-analyst.spec.ts             (NEW, 그룹 C)
│   ├── flow-admin.spec.ts               (NEW, 그룹 D)
│   └── logout.spec.ts                   (NEW, 그룹 E)
└── package.json                          (MOD: scripts + devDependencies 추가만)
```

수정 0건: `apps/web/src/**`, `apps/control-plane/**`, `pipelines/**`, 루트 `package.json`.

### 7.2 Playwright 설정 개요

`playwright.config.ts` 핵심:
- `testDir: './e2e'`
- `baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000'`
- `projects: [{ name: 'chromium', use: devices['Desktop Chrome'] }]` (OPEN #3 A 가정)
- `reporter: [['list'], ['html', { open: 'never' }]]`
- `webServer`: `{ command: 'npm run dev', url: 'http://localhost:3000', reuseExistingServer: true }` — 로컬 dev 서버 자동 기동
- `use.trace: 'on-first-retry'` (REQ-E2E-OPT-200)

### 7.3 fixture 설계

`apps/web/e2e/fixtures/auth.ts`:
- `createAuthState(role: 'viewer' | 'analyst' | 'admin'): Promise<StorageState>` — 모의 JWT 생성(§6 OPEN #5 결정) → `BrowserContext.addCookies` → `storageState.json` 저장
- 글로벌 setup(`globalSetup` config 옵션)에서 3개 storageState 사전 생성하여 각 spec 빠른 시작

`apps/web/e2e/fixtures/api-mocks.ts`:
- `mockBffEndpoints(page: Page, overrides?: Partial<MockMap>): Promise<void>` — `page.route` 로 `/api/v1/**` 응답 모킹
- 기본 응답은 `fixtures/api/*.json` 에서 로드, 시나리오별 override 가능

### 7.4 0-diff 가드

본 SPEC 머지 PR 체크리스트:
- [ ] `git diff main -- apps/control-plane/` → 0 lines
- [ ] `git diff main -- apps/web/src/` → 0 lines (단, `apps/web/package.json`은 허용)
- [ ] `git diff main -- pipelines/` → 0 lines
- [ ] `git diff main -- package.json` (루트) → 0 lines

`apps/web/package.json` 의 허용된 변경 범위는 §2.1 #6 (scripts 2줄 + devDependencies 1~2줄 추가). 그 외 필드 변경 금지.

---

## 8. 테스트 계획 (Test Plan)

### 8.1 본 SPEC 자체의 검증

본 SPEC은 테스트 슈트를 만드는 SPEC이므로 "테스트의 테스트"는 다음으로 대체한다:

- **셀프 검증 1**: `npm run test:e2e` 첫 실행에서 모든 AC가 GREEN으로 통과한다 (AC-INF-002).
- **셀프 검증 2**: SUT의 임의 페이지를 일시적으로 깨뜨리는 mutation을 가하면(예: viewer storageState로 점수 폼이 보이도록 SUT 수정 — 로컬 검증용, 커밋 안 함), 해당 AC가 RED로 떨어진다 (mutation testing 정신). PR에는 0-diff 상태만 커밋.
- **셀프 검증 3**: 0-diff PR 체크(§7.4) 모두 통과.

### 8.2 회귀 방지

- SPEC-AX-WEB-001 후속 SPEC(예: UI 텍스트 변경)이 본 SPEC 셀렉터를 깨뜨리면 `apps/web/e2e/selectors.ts` 1곳 수정으로 회복(§6 OPEN #6 A).
- 백엔드 SPEC(예: REVIEW-001 v2)이 4-state machine을 5-state로 확장하면 본 SPEC AC-FLOW-ADMIN-002의 기대 상태 전이를 업데이트 — 그 시점은 본 SPEC v0.2.0.

### 8.3 실행 명령 (사용자 manual run)

```bash
# 1회 셋업
cd apps/web
npm install                          # @playwright/test devDep 설치
npx playwright install chromium      # 브라우저 바이너리

# E2E 실행
npm run test:e2e                     # 헤드리스, list + html 리포트
npm run test:e2e:ui                  # UI 모드, 디버깅용
```

---

## 9. 영향 파일 (Affected Files — 본 SPEC 머지 시 변경 범위)

### 9.1 NEW (신규 추가)
- `apps/web/playwright.config.ts`
- `apps/web/.env.test.example`
- `apps/web/.env.test` (.gitignore)
- `apps/web/e2e/README.md`
- `apps/web/e2e/selectors.ts`
- `apps/web/e2e/fixtures/auth.ts`
- `apps/web/e2e/fixtures/api-mocks.ts`
- `apps/web/e2e/fixtures/api/*.json` (6개)
- `apps/web/e2e/auth.spec.ts`
- `apps/web/e2e/rbac-viewer.spec.ts`
- `apps/web/e2e/flow-analyst.spec.ts`
- `apps/web/e2e/flow-admin.spec.ts`
- `apps/web/e2e/logout.spec.ts`
- `apps/web/.gitignore` (또는 기존 파일에 `.env.test`, `e2e/.auth/` 라인 추가 — 추가 1~2줄만 허용)

### 9.2 MOD (수정, 최소 범위만)
- `apps/web/package.json`:
  - `scripts`에 `test:e2e`, `test:e2e:ui` 2줄 추가
  - `devDependencies`에 `@playwright/test` 1줄 추가 (선택적으로 `msw` 추가 — §6 OPEN #1/#2 결정)

### 9.3 0-DIFF (절대 변경 금지) [HARD]
- `apps/control-plane/**` 전체
- `pipelines/**` 전체
- `apps/web/src/**` 전체 (SUT 소스)
- 루트 `package.json`
- `infra/keycloak/**` (있다면)
- 모든 DB 마이그레이션 파일

---

## 10. 의존 SPEC 목록

본 SPEC은 다음 SPEC들이 모두 GREEN(완료, main 머지) 상태로 가정한다:

- SPEC-AX-WEB-001 (Next.js 14+ 대시보드) — 1차 의존, SUT 자체
- SPEC-AX-AUTH-001/002/003 (Keycloak OIDC + RBAC 3-역할 + ABAC narrowing)
- SPEC-AX-EVID-001 (증빙 관리) — analyst 흐름 데이터
- SPEC-AX-EVAL-ITEM-001 (평가 항목 taxonomy)
- SPEC-AX-SCORE-001/SCORE-API-001 (점수)
- SPEC-AX-REPORT-001 (범주 리포트)
- SPEC-AX-REVIEW-001 (리뷰 워크플로 4-state)
- SPEC-AX-RUBRIC-001 (루브릭 임계값)
- SPEC-AX-AUDIT-QUERY-001 (감사 로그 검색, admin-only)
- SPEC-AX-OBS-001 (관측성) — 필수 아님, REQ-E2E-OPT-201 라이브 모드에서 metric 검증 시 활용

본 SPEC의 변경이 위 SPEC에 역방향 영향: **0건** (consumer-only).

---

## 11. 마무리 (Closing)

본 SPEC v0.1.0은 SPEC-AX-WEB-001이 만든 5-스크린 PoC 대시보드를 Playwright E2E로 자동 검증하는 첫 SPEC이다. 핵심 원칙은:

1. **0-diff against SUT** — `apps/web/src/**`, `apps/control-plane/**`, `pipelines/**` 전부 변경 0건
2. **라이브 의존 없이 로컬 실행** — Keycloak/Postgres/MinIO/Go 백엔드 부트스트랩 불필요
3. **PoC 범위 일치** — 시각 회귀·부하·CI/CD·a11y 모두 §3 비목표

§6 OPEN 6건은 `/moai plan` annotation cycle 또는 `/moai run` 시작 전 plan-auditor 단계에서 SPEC-AX-WEB-001 SUT 코드 조사를 통해 해결한다. 모든 OPEN이 RESOLVED 처리되면 본 SPEC은 v0.2.0으로 승격되어 `/moai run SPEC-AX-E2E-001` 진입한다.
