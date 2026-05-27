# SPEC-AX-WEB-003 — Implementation Plan

작성일: 2026-05-27
작성자: ircp
SPEC: SPEC-AX-WEB-003
Methodology: DDD(ANALYZE-PRESERVE-IMPROVE) — 단일 외과적 변경이므로 PRESERVE 단계는 "기존 next.config.ts 형상 캡처 → 새 .mjs와 라인 단위 diff 검증"으로 갈음

---

## 0. 사전 조건 (Preconditions)

- 작업 브랜치: `feature/SPEC-AX-WEB-003-e2e-fix` (현재 브랜치 확인됨)
- 현재 `apps/web/next.config.ts`가 존재하며 직접 Read로 44라인 검증 완료
- `apps/web/package.json`에 `ts-node`/`tsx`/`jiti` 부재 검증 완료
- `apps/web/e2e/` 디렉터리에 5개 spec 파일 / 21 tests 존재 검증 완료
- Go control-plane(8080) 인스턴스 가용성 — Phase B/C에서 `BACKEND_BASE_URL` 프록시 검증 시 필요. 미가용 시 E2E는 page.route 모킹 fixture로 진행(SPEC-AX-E2E-001 §6 OPEN #2 채택안과 동일).

---

## 1. 마일스톤 (Milestones — 우선순위 기준)

### 1.1 Milestone M1 — 설정 파일 전환 (Priority High)

**범위**: `apps/web/next.config.ts` → `apps/web/next.config.mjs` rename + 2줄 텍스트 편집

**산출물**:
- 신규 파일: `apps/web/next.config.mjs` (42 라인)
- 삭제 파일: `apps/web/next.config.ts` (44 라인)

**작업**:
1. `git mv apps/web/next.config.ts apps/web/next.config.mjs` 실행 (history 보존)
2. `apps/web/next.config.mjs` Read로 현재 내용 확인
3. Edit 도구로 라인 1 삭제 (`import type { NextConfig } from "next";`)
4. Edit 도구로 라인 10 변경 (`const nextConfig: NextConfig = {` → `const nextConfig = {`)
5. Read로 결과 파일 확인 — `import type` 부재 + `: NextConfig` 부재 검증

**합격 기준**:
- `git status`에 `apps/web/next.config.ts` deleted + `apps/web/next.config.mjs` new file (또는 renamed) 표시
- `grep -E '^import type|: *NextConfig' apps/web/next.config.mjs` 출력 0건
- 나머지 41 라인 (주석 4 + 본문 37) 100% 보존

---

### 1.2 Milestone M2 — Dev Server 기동 검증 (Priority High)

**범위**: Phase B (B1 + B2 + B3)

**작업**:
1. `cd apps/web && npm run dev` 백그라운드 기동 (Bash `run_in_background: true`)
2. dev 서버 ready 대기 (`Ready in` 로그 패턴 polling, max 30초)
3. `curl -sf -o /dev/null -w '%{http_code}' http://localhost:3000/login` 실행 → `200` 확인
4. (선택) `curl -sf http://localhost:3000/api/v1/health` 등으로 BFF rewrite 동작 확인 (Go control-plane 가용 시)
5. dev 서버 종료 (SIGTERM)

**합격 기준**:
- B1: `Ready in Nms` 로그 출력 + dev 프로세스 listening 상태
- B2: `/login` HTTP 200
- B3: clean shutdown (exit 0)
- module parse error / syntax error 0건

**Fallback**:
- B2 실패 시: Next.js 로그 확인 → config 파일 syntax 재검증 → M1 재실행
- ECONNREFUSED 시: dev 서버 기동 대기 시간 연장 (max 60초)

---

### 1.3 Milestone M3 — E2E 21건 실행 (Priority High)

**범위**: Phase C (C1)

**작업**:
1. dev 서버 백그라운드 재기동 (M2 절차 반복)
2. `cd apps/web && npm run test:e2e` 실행 (Playwright runner)
3. 결과 집계: 21 passed / 0 failed 확인
4. 실패 시 개별 spec 단위 재실행으로 격리:
   - `npx playwright test e2e/auth.spec.ts` (3 tests)
   - `npx playwright test e2e/logout.spec.ts` (2 tests)
   - `npx playwright test e2e/flow-analyst.spec.ts` (4 tests)
   - `npx playwright test e2e/flow-admin.spec.ts` (5 tests)
   - `npx playwright test e2e/rbac-viewer.spec.ts` (7 tests)
5. dev 서버 종료

**합격 기준**:
- Playwright 출력: `21 passed (Xs)` 또는 `21 passed (Xs)` with 0 failed
- HTML 리포트 또는 stdout에서 21건 전부 status: passed

**참고**: E2E 시나리오 자체의 정확성은 SPEC-AX-E2E-001의 책임. 본 SPEC은 "21건이 실행 가능 상태로 회복"만 검증한다. 만약 SPEC-AX-E2E-001 v0.4.0 정정 사항(`URL 유지 in-page error` 등)이 SUT 코드에 반영되어 있다면 21건 모두 PASS가 기대됨. 만약 Go control-plane 의존 시나리오가 backend 미가동으로 실패한다면, M3 합격 기준을 "21 runnable + ≥18 passed + 차단 사유 명확"으로 완화하고 OPEN-AC로 기록한다(아래 §3 위험관리 참조).

---

### 1.4 Milestone M4 — Frozen Scope 검증 (Priority High)

**범위**: REQ-WEB-SCOPE-001~004 검증

**작업**:
1. `git diff --quiet -- apps/control-plane` 실행 → exit 0 확인
2. `git diff --quiet -- apps/pipeline` 실행 → exit 0 확인
3. `git diff --quiet -- apps/web/src` 실행 → exit 0 확인
4. `git diff --quiet -- apps/web/e2e` 실행 → exit 0 확인
5. `git diff --quiet -- apps/web/playwright.config.ts apps/web/package.json package.json go.mod pyproject.toml` 실행 → exit 0 확인
6. `git diff --stat HEAD` 전체 출력 확인 — 변경 파일이 `apps/web/next.config.{ts→mjs}` 1건뿐인지 검증

**합격 기준**: 6개 명령 전부 exit 0 + git diff --stat가 단일 rename만 표시

**[HARD]**: orchestrator는 M1 완료 직후, M2 시작 전, M3 완료 후 총 3회 본 검증을 직접 수행한다(teammate 자가보고 신뢰 금지 — 메모리 `feedback_consumer_only_orchestrator_grep.md` 참조).

---

## 2. 기술 접근 (Technical Approach)

### 2.1 변경 패턴: Edit 우선

`git mv` + 2회 `Edit` 호출이 최소 변경 경로다. `Write`로 전체 파일을 재생성하면 의도치 않은 라인 차이(개행/공백) 유입 위험 → Edit으로 정확히 2줄만 건드린다.

### 2.2 검증 도구: Bash + grep

- `grep -nE '^import type'` — TypeScript syntax 잔존 검출
- `grep -nE ': *NextConfig'` — type annotation 잔존 검출
- `git diff --quiet -- <path>` — frozen scope 0-diff 검증

### 2.3 Dev Server 운영

- 백그라운드 기동: `Bash` 도구의 `run_in_background: true` 사용 (Write 작업이 끝난 후 검증 단계에서)
- 종료: 명시적 SIGTERM (process kill) 또는 Bash 세션 종료
- 포트 충돌 시: 기존 :3000 점유 프로세스 확인 (`lsof -i :3000`) 후 정리

---

## 3. 위험 관리 (Risk Management)

| 위험 | 가능성 | 영향 | 완화책 |
|------|--------|------|--------|
| `git mv` 실패 (Windows path / permission) | Low | Low | `rm` + `Write` fallback (history loss 감수) |
| `npm run dev` 기동 후 다른 라인에서 syntax error | Very Low | Medium | M1 후 즉시 `node -e "import('./next.config.mjs').then(c=>console.log(c.default))"` 사전 검증 |
| Go control-plane 미가동으로 BFF 프록시 E2E 실패 | Medium | Medium | SPEC-AX-E2E-001 `page.route` 모킹 fixture가 활성 — backend live 없이도 21건 통과 가능. 미통과 시 M3 합격 기준 완화 + OPEN-AC 기록 |
| Playwright 브라우저 binary 미설치 | Low | High | `npx playwright install` 사전 실행 (M3 진입 전 dry-run 체크) |
| 포트 :3000 점유 | Low | Low | `lsof -i :3000` 확인 + 기존 dev 프로세스 정리 |
| `apps/web/tsconfig.json`이 `next.config.ts`를 explicit include | Very Low | Low | Read로 사전 검증 (현재 include는 통상 `"src/**/*"` 패턴이므로 영향 없음 예상) |
| eslint가 `.mjs` 설정 파일을 새로 lint하면서 위반 발생 | Low | Low | `next lint`는 `next.config.*` 무시. 위반 시 ESLint disable 주석으로 처리(behavior 영향 0) |

---

## 4. 의존성 및 순서 (Dependencies & Sequencing)

본 SPEC은 다른 미머지 SPEC에 의존하지 않는다:

- SPEC-AX-WEB-001: merged ✓
- SPEC-AX-E2E-001: merged ✓ (본 SPEC이 해제하려는 차단의 원인 위치)

내부 마일스톤 순서:

```
M1 (config 전환) → M2 (dev 서버 검증) → M3 (E2E 실행) → M4 (frozen 검증)
                                                            ↑
                                       M1 직후, M2 직전, M3 직후 3회 실행
```

M4는 M1/M2/M3 사이사이 끼어 들어가는 검증 마일스톤이다.

---

## 5. 산출물 체크리스트

- [ ] `apps/web/next.config.mjs` 신규 생성 (42 라인, ESM)
- [ ] `apps/web/next.config.ts` 저장소에서 제거
- [ ] `apps/web/next.config.mjs`에 `import type` 부재 (grep 검증)
- [ ] `apps/web/next.config.mjs`에 `: NextConfig` 부재 (grep 검증)
- [ ] `npm run dev` 기동 성공 + `/login` 200 OK
- [ ] `npm run test:e2e` 21 passed / 0 failed (또는 합격 기준 완화 + OPEN 기록)
- [ ] Frozen scope 6경로 `git diff --quiet` exit 0
- [ ] `git diff --stat HEAD`가 단일 rename만 표시

---

## 6. /clear 권장 시점

본 SPEC은 단일 외과적 변경이므로 `/clear`가 필수는 아니다. 단, Run phase 직전 `/clear` 1회로 Plan phase 컨텍스트를 정리하면 토큰 효율적.
