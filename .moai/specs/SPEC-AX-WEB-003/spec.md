---
id: SPEC-AX-WEB-003
version: 0.1.0
status: completed
created: 2026-05-27
updated: 2026-05-27
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-27): SPEC-AX-E2E-001(merged)이 도입한 21건 Playwright E2E 테스트가 `apps/web/next.config.ts`의 TypeScript 전용 syntax(`import type ...` + `: NextConfig` annotation)로 인해 Next.js 14.2.18이 config를 로드하지 못해 `npm run dev`가 즉시 실패하고, 그 결과 21건 E2E가 전부 차단된 상태를 해결하는 최소 외과적 SPEC. 단일 변경은 `apps/web/next.config.ts` → `apps/web/next.config.mjs` rename + 1줄 type import 삭제 + 1줄 type annotation 제거. Next.js 업그레이드(14 → 15)는 §3 비목표(별도 SPEC). 본 SPEC은 SPEC-AX-WEB-001 / SPEC-AX-E2E-001의 순수 consumer이며, Go control-plane(`apps/control-plane/**`)·Python pipelines(`apps/pipeline/**`)·frontend 본체(`apps/web/src/**`)·E2E 테스트 본체(`apps/web/e2e/**`)·`playwright.config.ts`·루트 `package.json`은 [HARD] 0-diff. (작성자: ircp)

> Schema note: YAML frontmatter는 누적 18 SPEC과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. phantom-API 검증 부담은 단일 파일 텍스트 편집이라는 점에서 최소이며, 변경 대상은 `apps/web/next.config.ts` 1개 파일(44 라인, 직접 Read로 검증)에 한정된다.

---

# SPEC-AX-WEB-003 — `next.config.ts` → `next.config.mjs` 전환으로 E2E 차단 해제 (Unblock E2E by Converting Next.js Config from TypeScript to ESM JavaScript)

## 1. 개요 (Overview)

SPEC-AX-E2E-001가 도입한 21건 Playwright E2E 테스트(5 spec 파일)가 **`npm run dev` 기동 실패**로 전부 차단되었다. 차단 원인은 단일 root cause: `apps/web/next.config.ts`가 TypeScript 전용 syntax(`import type { NextConfig } from "next";` + `const nextConfig: NextConfig = {`)를 포함하지만 Next.js **14.2.18**은 TypeScript config 파일을 네이티브 지원하지 않고, `apps/web/package.json`에 TypeScript 트랜스파일러(`ts-node`/`tsx`/`jiti`)가 부재하기 때문이다.

본 SPEC은 **단일 외과적 변경**으로 이를 해결한다:

1. `apps/web/next.config.ts` → `apps/web/next.config.mjs` 파일 rename (1개 파일)
2. 1번 라인 `import type { NextConfig } from "next";` 삭제
3. 10번 라인 `const nextConfig: NextConfig = {` → `const nextConfig = {` 변경 (type annotation 제거)

나머지 41줄(rewrites proxy + CSP headers + 주석)은 **100% 보존**된다. 설정 객체 형상이 동일하므로 BFF rewrite(`/api/v1/:path*` → `${BACKEND_BASE_URL}/api/v1/:path*`) 및 보안 헤더(`X-Frame-Options: DENY` 외) 런타임 거동에 변화가 없다.

### 1.1 본 SPEC의 의미와 PoC 범위

본 SPEC의 1차 산출물은 **dev 서버 기동 복구 + 21건 E2E 실행 가능 상태 회복**이다. 채점·평가·증빙 등 도메인 기능 변경 없음 — 순전히 빌드/실행 환경 복구.

PoC 데모 보강이므로 다음은 의식적으로 **간소화**한다:
- Next.js 14 → 15 업그레이드 미수행(§3 비목표)
- TypeScript 트랜스파일러(`jiti`/`tsx`) 도입 미수행(§3 비목표)
- 신규 E2E 추가 미수행(§3 비목표 — 21건 그대로 검증)
- CI/CD 통합 미수행(SPEC-AX-E2E-001 §3과 동일 비목표 유지)

### 1.2 Anchor 컨텍스트

본 SPEC은 18번째 SPEC이며, SPEC-AX-WEB-001(merged) + SPEC-AX-E2E-001(merged)의 순수 **consumer**다. SUT는 `apps/web/next.config.ts` 1개 파일이며, **수정 후 산출물도 동일 위치의 `apps/web/next.config.mjs` 1개 파일**이다(rename + 2줄 편집). 이외 모든 영역은 [HARD] 0-diff.

### 1.3 검증 대상 — 단일 파일 카탈로그

| 카테고리 | 경로/파일 | 본 SPEC 처리 |
|----------|-----------|---------------|
| 설정 파일(원본) | `apps/web/next.config.ts` (44 라인, TypeScript syntax 포함) | **삭제**(git mv로 rename) |
| 설정 파일(신규) | `apps/web/next.config.mjs` (42 라인, ESM JavaScript) | **신규 생성**(rename 결과) |
| 검증 대상 SUT | `apps/web/e2e/{auth,logout,flow-analyst,flow-admin,rbac-viewer}.spec.ts` (총 21 tests) | **읽기 전용 검증** — 본 SPEC은 수정하지 않음 |
| Frozen Go | `apps/control-plane/**` | 0-diff [HARD] |
| Frozen Python | `apps/pipeline/**` | 0-diff [HARD] |
| Frozen frontend src | `apps/web/src/**` | 0-diff [HARD] |
| Frozen Playwright | `apps/web/playwright.config.ts` | 0-diff [HARD] |
| Frozen 루트 manifest | `package.json`, `go.mod`, `pyproject.toml` | 0-diff [HARD] |

phantom path 0건 [HARD]: 본 SPEC의 모든 참조 파일은 직접 Read 또는 `ls` 출력으로 검증된 실재 경로다.

### 1.4 권한 경계

본 SPEC은 단일 빌드 구성 파일 변경으로 권한 모델·인증·RBAC에 영향을 주지 않는다. SPEC-AX-AUTH-001/002/003 + SPEC-AX-WEB-001 §1.4가 정의한 viewer/analyst/admin 3-role 정책은 변경 없이 그대로 보존된다.

---

## 2. 목표 (Goals)

| ID | 목표 | 우선순위 |
|----|------|----------|
| G1 | `apps/web/next.config.mjs`가 Next.js 14.2.18에서 module parse 에러 없이 로드된다. | High |
| G2 | `npm run dev`(in `apps/web`)가 정상 기동하여 `http://localhost:3000/login`이 200 OK를 반환한다. | High |
| G3 | `npm run test:e2e`(in `apps/web`)가 21건 테스트를 모두 실행 가능하게 한다(0 차단). | High |
| G4 | Go control-plane / Python pipelines / frontend 본체 / E2E 테스트 본체 / playwright config / 루트 manifest는 [HARD] 0-diff. | High |
| G5 | BFF rewrite proxy 및 CSP/보안 헤더 동작 100% 보존. | High |

---

## 3. 비목표 (Non-Goals)

본 SPEC은 다음을 의식적으로 **수행하지 않는다**:

1. Next.js 14 → 15 메이저 업그레이드 (별도 SPEC 필요 — App Router breaking change, React 19 요구사항, middleware 시그니처 변경 등 광범위 영향)
2. TypeScript 트랜스파일러(`ts-node`/`tsx`/`jiti`) devDependency 추가 (.mjs 전환이 충분히 가벼움)
3. 신규 E2E 테스트 추가 또는 기존 21건 시나리오 수정
4. `playwright.config.ts`의 `webServer` 정의 변경/추가
5. CI/CD 워크플로 통합 (SPEC-AX-E2E-001 §3과 동일 비목표 유지)
6. `apps/web/tsconfig.json` 변경 (설정 파일은 typecheck include에서 자연 제외됨)
7. 시각 회귀 테스트, 부하 테스트, k6 통합
8. ESLint 규칙 추가/변경 또는 `eslint-config-next` 업그레이드
9. `apps/web/src/**`의 TypeScript 타입 안전성 변경 — `next.config.mjs` 한정 type annotation 제거이므로 application code는 영향 없음
10. SPEC-AX-WEB-001/E2E-001이 정의한 SUT 경로(7 보호 페이지 + 1 로그인 + 12 BFF) 변경

---

## 4. EARS Requirements (요구사항)

| REQ ID | 패턴 | 요구사항 |
|--------|------|----------|
| **REQ-WEB-CONFIG-001** | Ubiquitous | `apps/web/next.config.mjs`는 유효한 ECMAScript Module(ESM)이어야 하며, TypeScript 전용 syntax(`import type`, `:Type` annotation, `as` 단언, `interface` 선언)를 일절 포함하지 않아야 한다. |
| **REQ-WEB-CONFIG-002** | Ubiquitous | `apps/web/next.config.mjs`는 export default로 Next.js 설정 객체를 노출해야 하며, 객체는 기존 `next.config.ts`와 동일한 필드(`reactStrictMode`, `poweredByHeader`, `rewrites`, `headers`)와 동일한 값을 가져야 한다. |
| **REQ-WEB-CONFIG-003** | Event-Driven | **WHEN** `npm run dev`가 `apps/web` 작업 디렉터리에서 실행되었을 때, Next.js dev 서버는 module parse 오류 없이 기동을 완료하고 `http://localhost:3000`에서 HTTP 요청을 수락해야 한다. |
| **REQ-WEB-CONFIG-004** | Event-Driven | **WHEN** dev 서버가 기동된 상태에서 `GET http://localhost:3000/login` 요청이 도달했을 때, 시스템은 HTTP 200 OK와 함께 로그인 페이지 HTML을 반환해야 한다. |
| **REQ-WEB-CONFIG-005** | Ubiquitous | `apps/web/next.config.ts` 파일은 본 SPEC 적용 후 저장소 상에 존재해서는 안 된다(git tracking 상 삭제, working tree 상 부재). |
| **REQ-WEB-E2E-001** | Event-Driven | **WHEN** dev 서버가 기동된 상태에서 `npm run test:e2e`가 `apps/web` 작업 디렉터리에서 실행되었을 때, Playwright는 21건 테스트를 전부 실행하고 0건 실패로 종료해야 한다. |
| **REQ-WEB-E2E-002** | Ubiquitous | E2E 테스트 5개 spec 파일(`auth.spec.ts`, `logout.spec.ts`, `flow-analyst.spec.ts`, `flow-admin.spec.ts`, `rbac-viewer.spec.ts`)의 내용은 본 SPEC 적용 전후로 동일해야 한다(0-diff). |
| **REQ-WEB-SCOPE-001** | Unwanted | 시스템은 `apps/control-plane/**` 하위 어떤 Go 파일도 수정해서는 안 된다(git diff 기준 zero diff). |
| **REQ-WEB-SCOPE-002** | Unwanted | 시스템은 `apps/pipeline/**` 하위 어떤 Python 파일도 수정해서는 안 된다(git diff 기준 zero diff). |
| **REQ-WEB-SCOPE-003** | Unwanted | 시스템은 `apps/web/src/**` 하위 어떤 frontend 소스 파일도 수정해서는 안 된다(git diff 기준 zero diff). |
| **REQ-WEB-SCOPE-004** | Unwanted | 시스템은 `apps/web/playwright.config.ts`, `apps/web/package.json`, 루트 `package.json`, `go.mod`, `pyproject.toml`을 수정해서는 안 된다(git diff 기준 zero diff). |
| **REQ-WEB-BEHAV-001** | Ubiquitous | `next.config.mjs`의 `rewrites()`는 source `"/api/v1/:path*"`를 destination `${BACKEND_BASE_URL}/api/v1/:path*`로 프록시해야 하며, `BACKEND_BASE_URL`은 환경변수 우선, 기본값 `"http://localhost:8080"`를 사용한다(기존 `next.config.ts`와 동일). |
| **REQ-WEB-BEHAV-002** | Ubiquitous | `next.config.mjs`의 `headers()`는 모든 경로(`"/(.*)"`)에 `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` 3개 헤더를 부여해야 한다(기존 `next.config.ts`와 동일). |

---

## 5. 기술 접근 (Technical Approach)

### 5.1 변경 절차 (Phase A 구체화)

```text
1. git mv apps/web/next.config.ts apps/web/next.config.mjs
2. Edit apps/web/next.config.mjs:
   - Line 1 (전): import type { NextConfig } from "next";
   - Line 1 (후): (삭제)
   - Line 10 (전): const nextConfig: NextConfig = {
   - Line 10 (후): const nextConfig = {
3. 나머지 41 라인은 그대로 유지(주석 4 + 본문 37)
```

`git mv`를 사용하면 git history 상 rename으로 인식되어 추후 blame 추적 시 원본 SPEC-AX-WEB-001의 작성 이력이 보존된다.

### 5.2 결과 파일 형상 (예상 `next.config.mjs` 골격)

```javascript
// @MX:NOTE: Next.js 설정 — Go control-plane(:8080)을 위한 rewrite proxy 정의.
// SPEC-AX-WEB-001 §7.2: BFF Route Handler(`/api/auth/**`)는 직접 처리,
// 그 외 `/api/v1/**`는 Go control-plane으로 투과 프록시.

const BACKEND_BASE_URL =
  process.env["BACKEND_BASE_URL"] ?? "http://localhost:8080";

const nextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  async rewrites() { /* ... 보존 ... */ },
  async headers() { /* ... 보존 ... */ },
};

export default nextConfig;
```

### 5.3 검증 절차 (Phase B + C)

| 단계 | 명령 | 합격 기준 |
|------|------|-----------|
| B1 | `cd apps/web && npm run dev` (백그라운드 기동) | stdout에 `Ready in ...ms` 출력, exit 없이 listening |
| B2 | `curl -sf -o /dev/null -w '%{http_code}' http://localhost:3000/login` | `200` 반환 |
| B3 | dev 서버 종료(SIGTERM) | clean exit |
| C1 | `cd apps/web && npm run test:e2e` | 21 passed, 0 failed |
| Frozen | `git diff --quiet -- apps/control-plane apps/pipeline apps/web/src apps/web/e2e apps/web/playwright.config.ts apps/web/package.json package.json go.mod pyproject.toml` | exit code 0 |

### 5.4 Type Safety 영향 평가

`next.config`에서 `: NextConfig` annotation을 제거하면 IDE의 자동 완성 / 컴파일 타임 검사가 해당 객체에서만 비활성화된다. 단:

- `apps/web/tsconfig.json`은 `next.config.*`를 typecheck 대상에서 자연스럽게 제외(설정 파일이므로 `include` 패턴 비매칭)
- 설정 객체 구조는 안정적이며 변경 빈도가 낮음
- 필요 시 JSDoc `/** @type {import('next').NextConfig} */`로 정적 타입 힌트 보강 가능(본 SPEC 적용 후 선택적 follow-up)

본 SPEC은 JSDoc 보강을 **명시적으로 비목표화**하지 않는다 — Phase A 구현자가 필요 시 1줄 JSDoc 추가는 허용한다(behavior 영향 0, line count 영향 +1).

---

## 6. Open Questions

본 SPEC은 단일 root cause + 단일 외과적 변경이며, 모든 가정이 직접 검증되었으므로 **§6 비어 있음**.

---

## 7. Exclusions (What NOT to Build)

| Exclusion ID | 제외 사항 | 근거 |
|--------------|----------|------|
| EXC-001 | Next.js 14 → 15 업그레이드 | breaking change 광범위(App Router/React 19/middleware), 별도 SPEC 필요 |
| EXC-002 | `ts-node`/`tsx`/`jiti` devDependency 추가 | `.mjs` 전환이 충분히 가벼움 — 의존성 증가 회피 |
| EXC-003 | 신규 E2E 시나리오 추가 | 21건 기존 테스트로 회복 검증 충분 |
| EXC-004 | `playwright.config.ts` `webServer` 정의 변경 | 본 SPEC 범위 밖 — frozen scope |
| EXC-005 | CI/CD 워크플로 통합 | SPEC-AX-E2E-001 §3과 동일 비목표 |
| EXC-006 | `apps/web/src/**` 코드/타입 수정 | frozen scope [HARD] |
| EXC-007 | `apps/control-plane/**` Go 코드 수정 | frozen scope [HARD] consumer-only |
| EXC-008 | `apps/pipeline/**` Python 코드 수정 | frozen scope [HARD] consumer-only |
| EXC-009 | 시각 회귀 / 부하 / k6 테스트 도입 | SPEC-AX-E2E-001 §3 비목표 유지 |
| EXC-010 | RBAC 모델/권한 정책 변경 | 단일 빌드 설정 변경이므로 권한 시스템 영향 없음 |

---

## 8. 영향 분석 요약

| 차원 | 영향 |
|------|------|
| 코드 변경량 | 파일 1개 rename + 2줄 편집 (1줄 삭제 + 1줄 변경) |
| 런타임 거동 변화 | 없음 — 설정 객체 형상 100% 동일 |
| 의존성 변경 | 없음 — package.json 0-diff |
| TypeScript 검사 영향 | 설정 파일 1개에 한정 — application code 무영향 |
| 보안 헤더 변화 | 없음 — CSP 3종 헤더 동일 부여 |
| BFF rewrite 변화 | 없음 — `/api/v1/:path*` proxy 동일 |
| E2E 회복 효과 | 21건 차단 해제 (100% 회복) |
| Go control-plane 영향 | 0-diff [HARD] |
| Python pipelines 영향 | 0-diff [HARD] |

---

## 9. 성공 기준 (Definition of Done)

본 SPEC은 다음 조건이 모두 충족될 때 완료(Definition of Done)로 간주한다:

1. `apps/web/next.config.mjs` 존재 + `apps/web/next.config.ts` 부재
2. Phase B(B1+B2+B3) 전부 합격
3. Phase C(21 passed / 0 failed) 합격
4. Frozen scope `git diff --quiet` exit 0
5. acceptance.md의 4개 AC 모두 PASS
