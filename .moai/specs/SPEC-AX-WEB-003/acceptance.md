# SPEC-AX-WEB-003 — Acceptance Criteria

작성일: 2026-05-27
작성자: ircp
SPEC: SPEC-AX-WEB-003

총 AC: **8건** (Given/When/Then)
연결 REQ: REQ-WEB-CONFIG-001~005, REQ-WEB-E2E-001/002, REQ-WEB-SCOPE-001~004, REQ-WEB-BEHAV-001/002

---

## AC-CONFIG-001 — Config 파일 형식 검증 (연결: REQ-WEB-CONFIG-001, REQ-WEB-CONFIG-005)

**Given** SPEC-AX-WEB-003 Phase A 적용이 완료되었고

**When** 작업자가 다음 명령을 실행하면:
```bash
ls apps/web/next.config.*
grep -cE '^import type|: *NextConfig' apps/web/next.config.mjs
```

**Then** 시스템은 다음을 만족한다:
- `apps/web/next.config.mjs` 파일이 존재
- `apps/web/next.config.ts` 파일은 부재 (`No such file or directory`)
- grep 출력이 `0` (TypeScript 전용 syntax 잔존 없음)

---

## AC-CONFIG-002 — Module 로딩 검증 (연결: REQ-WEB-CONFIG-001, REQ-WEB-CONFIG-002, REQ-WEB-BEHAV-001, REQ-WEB-BEHAV-002)

**Given** Phase A 완료 + Node.js 20+ 환경

**When** 작업자가 다음 명령을 실행하면:
```bash
cd apps/web && node -e "import('./next.config.mjs').then(m => { const c = m.default; console.log(JSON.stringify({reactStrictMode: c.reactStrictMode, poweredByHeader: c.poweredByHeader, hasRewrites: typeof c.rewrites, hasHeaders: typeof c.headers})); })"
```

**Then** 출력은 다음을 만족한다:
- module parse error 0건
- `reactStrictMode: true`
- `poweredByHeader: false`
- `hasRewrites: "function"`
- `hasHeaders: "function"`

---

## AC-DEV-001 — Dev Server 기동 검증 (연결: REQ-WEB-CONFIG-003)

**Given** Phase A 완료 + 포트 :3000 미점유

**When** 작업자가 다음 명령을 실행하면:
```bash
cd apps/web && npm run dev &
# 30초 이내 ready 대기
curl -sf -o /dev/null -w '%{http_code}\n' http://localhost:3000/
```

**Then** 시스템은 다음을 만족한다:
- dev 서버 stdout에 `Ready in Nms` 또는 동등한 ready 시그널 출력 (module parse error 0건)
- HTTP 응답 코드: `200` (또는 `/`가 `/login`으로 redirect되어 `200` 또는 `307`)
- dev 프로세스가 listening 상태 유지 (SIGTERM 전까지 종료 없음)

---

## AC-DEV-002 — Login 페이지 응답 검증 (연결: REQ-WEB-CONFIG-004)

**Given** AC-DEV-001 충족 (dev 서버 기동 상태)

**When** 작업자가 다음 명령을 실행하면:
```bash
curl -sf -o /tmp/login.html -w '%{http_code}\n' http://localhost:3000/login
grep -c 'login' /tmp/login.html
```

**Then** 시스템은 다음을 만족한다:
- HTTP 응답 코드: `200`
- 응답 본문에 로그인 페이지를 식별할 수 있는 키워드 존재 (`grep -c 'login'` ≥ 1)

---

## AC-E2E-001 — Playwright 21건 실행 (연결: REQ-WEB-E2E-001)

**Given** AC-DEV-001 충족 (dev 서버 기동) + Playwright 브라우저 binary 설치 완료

**When** 작업자가 다음 명령을 실행하면:
```bash
cd apps/web && npm run test:e2e
```

**Then** 시스템은 다음을 만족한다:
- Playwright runner가 정상 시작 (config load 성공)
- 21건 테스트 전부 실행됨 (skip 없음)
- 결과: **`21 passed` / `0 failed`** (또는 SPEC-AX-E2E-001 §6 OPEN에 명시된 backend live 의존 시나리오 한정 완화 시 `≥18 passed + 차단 사유 OPEN-AC 기록`)
- 5개 spec 파일별 분포: auth(3) + logout(2) + flow-analyst(4) + flow-admin(5) + rbac-viewer(7) = 21

---

## AC-E2E-002 — E2E 테스트 파일 0-diff 검증 (연결: REQ-WEB-E2E-002)

**Given** Phase A 완료

**When** 작업자가 다음 명령을 실행하면:
```bash
git diff --stat HEAD -- apps/web/e2e/
git diff --quiet HEAD -- apps/web/e2e/ && echo "ZERO-DIFF" || echo "MODIFIED"
```

**Then** 시스템은 다음을 만족한다:
- `git diff --stat` 출력이 비어 있음 (변경 0건)
- `ZERO-DIFF` 출력

---

## AC-SCOPE-001 — Consumer-Only Frozen Scope 0-diff (연결: REQ-WEB-SCOPE-001~004)

**Given** Phase A 완료 (M1 직후, M2 직전, M3 직후 3회 검증)

**When** orchestrator가 다음 명령을 실행하면:
```bash
git diff --quiet HEAD -- apps/control-plane && \
git diff --quiet HEAD -- apps/pipeline && \
git diff --quiet HEAD -- apps/web/src && \
git diff --quiet HEAD -- apps/web/playwright.config.ts && \
git diff --quiet HEAD -- apps/web/package.json && \
git diff --quiet HEAD -- package.json && \
git diff --quiet HEAD -- go.mod && \
git diff --quiet HEAD -- pyproject.toml && \
echo "ALL-FROZEN-OK"
```

**Then** 시스템은 다음을 만족한다:
- 모든 8개 명령이 exit 0
- `ALL-FROZEN-OK` 출력
- (HARD) 본 검증은 orchestrator 직접 수행 — teammate 자가보고 신뢰 금지

---

## AC-SCOPE-002 — 전체 변경 단일 rename 검증 (연결: REQ-WEB-CONFIG-005)

**Given** Phase A 완료

**When** 작업자가 다음 명령을 실행하면:
```bash
git diff --stat HEAD
git status --porcelain | wc -l
```

**Then** 시스템은 다음을 만족한다:
- `git diff --stat HEAD` 출력이 단일 파일 변경만 표시 — `apps/web/next.config.ts` deleted + `apps/web/next.config.mjs` added (또는 `git mv`로 인한 rename 통합 표시 R100 또는 R99 등)
- `git status --porcelain | wc -l` ≤ 2 (rename 추적 모드에 따라 1 또는 2)
- 다른 디렉터리(`apps/control-plane/`, `apps/pipeline/`, `apps/web/src/`, `apps/web/e2e/` 등)는 일절 표시 없음

---

## Edge Cases (참고 — AC 외 검증 항목)

| EC ID | 시나리오 | 처리 |
|-------|---------|------|
| EC-001 | 포트 :3000 사전 점유 | M2 전 `lsof -i :3000` 확인 + 정리 |
| EC-002 | Go control-plane 미가동 → BFF rewrite 의존 E2E 실패 | SPEC-AX-E2E-001 `page.route` 모킹으로 회피. 미회피 시 AC-E2E-001 완화 + OPEN-AC |
| EC-003 | Playwright 브라우저 binary 미설치 | `npx playwright install` 사전 실행 |
| EC-004 | `npm run dev`가 30초 내 ready 미달성 | timeout 60초로 연장 후 재시도 |
| EC-005 | dev 서버 백그라운드 좀비 프로세스 | 명시적 SIGTERM + `wait`로 정리 |
| EC-006 | `.mjs`가 eslint 신규 lint 대상 진입 | `next lint` 출력 확인 — `next.config.*` 무시 정책 유지. 위반 시 ESLint disable 주석 |
| EC-007 | tsconfig.json이 `next.config.ts` explicit include | Phase A 전 Read로 확인 (현재 include 패턴 통상 `src/**/*`) |
| EC-008 | git mv가 history rename 감지 실패 → 별도 파일로 표시 | functional 영향 없음 (blame 추적만 손실) |

---

## Definition of Done (요약)

- [ ] AC-CONFIG-001 PASS
- [ ] AC-CONFIG-002 PASS
- [ ] AC-DEV-001 PASS
- [ ] AC-DEV-002 PASS
- [ ] AC-E2E-001 PASS (또는 완화 조건 + OPEN-AC 기록)
- [ ] AC-E2E-002 PASS
- [ ] AC-SCOPE-001 PASS (3회 검증 모두)
- [ ] AC-SCOPE-002 PASS
- [ ] EC-001~008 중 실제 발생 항목 해결
- [ ] plan.md M1~M4 마일스톤 완료 체크
