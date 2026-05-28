# SPEC-AX-WEB-003 — Compact Summary (1-Page)

**ID**: SPEC-AX-WEB-003 | **Version**: 0.1.0 | **Status**: planning | **Priority**: high
**Branch**: feature/SPEC-AX-WEB-003-e2e-fix | **Author**: ircp | **Date**: 2026-05-27

---

## 1줄 요약

`apps/web/next.config.ts` (TypeScript syntax 포함) → `apps/web/next.config.mjs` (ESM JS)로 전환하여 Next.js 14.2.18의 `npm run dev` 기동 실패를 해소하고, 그로 인해 차단된 21건 Playwright E2E 테스트를 재가동한다.

## Root Cause

Next.js 14.2.18은 `.ts` config 파일을 네이티브 지원하지 않으며, `apps/web/package.json`에 `ts-node`/`tsx`/`jiti` 트랜스파일러가 부재. 첫 줄 `import type { NextConfig } from "next";`에서 module parse 실패 → dev 서버 즉시 종료 → 21 E2E 차단.

## 변경 (총 1 파일, 2줄)

```
git mv apps/web/next.config.ts apps/web/next.config.mjs
- Line 1 삭제:  import type { NextConfig } from "next";
- Line 10 변경: const nextConfig: NextConfig = {  →  const nextConfig = {
- Line 2-9, 11-44 (총 41줄) 보존
```

## 핵심 EARS Requirements (13건)

| ID | 요약 |
|----|------|
| REQ-WEB-CONFIG-001 | `.mjs`는 유효한 ESM, TypeScript syntax 일절 없음 |
| REQ-WEB-CONFIG-002 | 설정 객체 필드(rewrites, headers 등) `.ts`와 동일 |
| REQ-WEB-CONFIG-003 | WHEN `npm run dev` THEN dev 서버 기동 + :3000 listening |
| REQ-WEB-CONFIG-004 | WHEN GET `/login` THEN HTTP 200 |
| REQ-WEB-CONFIG-005 | `.ts` 파일 저장소에서 삭제 |
| REQ-WEB-E2E-001 | WHEN `npm run test:e2e` THEN 21 passed / 0 failed |
| REQ-WEB-E2E-002 | `apps/web/e2e/**` 0-diff |
| REQ-WEB-SCOPE-001 | `apps/control-plane/**` 0-diff [HARD] |
| REQ-WEB-SCOPE-002 | `apps/pipeline/**` 0-diff [HARD] |
| REQ-WEB-SCOPE-003 | `apps/web/src/**` 0-diff [HARD] |
| REQ-WEB-SCOPE-004 | `playwright.config.ts`, `package.json`, `go.mod`, `pyproject.toml` 0-diff |
| REQ-WEB-BEHAV-001 | `rewrites()` proxy `/api/v1/:path*` → `${BACKEND_BASE_URL}/api/v1/:path*` 보존 |
| REQ-WEB-BEHAV-002 | `headers()` CSP 3종(`X-Frame-Options: DENY` 외) 보존 |

## 핵심 Acceptance Criteria (8건)

| AC ID | 검증 |
|-------|------|
| AC-CONFIG-001 | `.mjs` 존재 + `.ts` 부재 + grep `import type` 0건 |
| AC-CONFIG-002 | `node -e "import('./next.config.mjs')"` 성공 + 객체 필드 확인 |
| AC-DEV-001 | `npm run dev` 기동 → `/` HTTP 200/307 |
| AC-DEV-002 | `GET /login` HTTP 200 + login 키워드 |
| AC-E2E-001 | `npm run test:e2e` → 21 passed / 0 failed |
| AC-E2E-002 | `git diff -- apps/web/e2e/` 0건 |
| AC-SCOPE-001 | 8개 frozen 경로 `git diff --quiet` exit 0 (orchestrator 3회 검증) |
| AC-SCOPE-002 | `git diff --stat HEAD` 단일 rename만 표시 |

## Phase 순서

```
M1 (config 전환: git mv + 2 Edit)
  └─ M4 검증 #1 (frozen scope)
M2 (dev 서버 기동 + /login 200)
  └─ M4 검증 #2 (frozen scope)
M3 (E2E 21건 실행)
  └─ M4 검증 #3 (frozen scope)
```

## Exclusions (10건 — what NOT to build)

EXC-001 Next 14→15 업그레이드 / EXC-002 jiti/tsx 추가 / EXC-003 신규 E2E 추가 / EXC-004 playwright.config webServer 변경 / EXC-005 CI/CD 통합 / EXC-006 web/src 수정 / EXC-007 control-plane 수정 / EXC-008 pipeline 수정 / EXC-009 시각/부하 테스트 / EXC-010 RBAC 모델 변경

## 위험 (Top 3)

1. **Go control-plane 미가동 → BFF 의존 E2E 실패** (Medium/Medium) → SPEC-AX-E2E-001 `page.route` 모킹으로 회피, 미회피 시 AC-E2E-001 완화
2. **Playwright 브라우저 binary 미설치** (Low/High) → `npx playwright install` 사전 실행
3. **포트 :3000 점유** (Low/Low) → `lsof -i :3000` 확인 + 정리

## Definition of Done

- [ ] AC 8건 전부 PASS
- [ ] M4 frozen scope 검증 3회 모두 exit 0
- [ ] git diff --stat HEAD가 단일 rename만 표시
- [ ] dev 서버 정상 기동 + 21 E2E runnable

## §6 Open Questions

**비어 있음** — 단일 root cause + 단일 외과적 변경, 모든 가정 직접 검증 완료.
