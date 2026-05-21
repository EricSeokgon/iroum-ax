# SPEC-AX-E2E-001 — Playwright E2E 테스트 슈트

`apps/web/` Next.js 14+ 대시보드의 **골든 패스(인증·역할별 RBAC·로그아웃)** 를 브라우저-레벨에서 자동 검증한다. SUT 코드(`apps/web/src/**`, `apps/control-plane/**`, `pipelines/**`)는 **0-diff** — 본 디렉터리만 추가/수정한다.

## 1. 사전 요구사항

- Node.js 20 이상
- `apps/web/` 에서 `npm install` 완료
- Playwright Chromium 브라우저 설치: `npx playwright install chromium`
- Next.js dev 서버가 **별도 터미널에서 실행 중**이어야 함:
  ```bash
  cd apps/web
  npm run dev   # http://localhost:3000
  ```

### 1.1 알려진 SUT 블로커 (2026-05-21 시점)

현재 `apps/web/next.config.ts` (SPEC-AX-WEB-001 산출물)가 Next.js 14.2.18 의 `loadConfig` 에서 거부됨: `Configuring Next.js via 'next.config.ts' is not supported`. 이 문제는 SUT 측 픽스가 필요하며 (`next.config.ts` → `next.config.mjs` 변환 또는 Next.js 15 업그레이드) 본 SPEC 의 [HARD] 0-diff 제약(`apps/web/src/**`, 루트 `apps/web/` SUT 파일) 때문에 본 SPEC 에서 해결할 수 없다. **SPEC-AX-WEB-003 (가칭) 후속 SPEC 트리거 필요**.

해결 전 임시 우회: dev 서버가 다른 방식으로 실행 가능하면 (`E2E_LIVE_BACKEND=1` + 외부 서버 시) 본 슈트를 그대로 사용 가능.

## 2. 실행 방법

```bash
cd apps/web

# 헤드리스 실행 + list + html 리포트 (기본)
npm run test:e2e

# UI 모드 (디버깅용)
npm run test:e2e:ui

# 단일 spec 만
npx playwright test e2e/auth.spec.ts

# 특정 AC 만
npx playwright test --grep "AC-AUTH-001"
```

## 3. 시나리오 구성 (24 AC)

| 그룹 | 파일 | AC 수 | 핵심 검증 |
|------|------|-------|-----------|
| A 인증 | `auth.spec.ts` | 3 | 미인증 리다이렉트, storageState 주입 진입, 만료 쿠키 차단 |
| B viewer RBAC | `rbac-viewer.spec.ts` | 7 | 가시성 제한 + audit-logs/rubric 인-페이지 차단 |
| C analyst 흐름 | `flow-analyst.spec.ts` | 4 | 업로드 폼 가시 + 점수/리뷰 제출 + audit-logs 차단 |
| D admin 흐름 | `flow-admin.spec.ts` | 5 | audit-logs/rubric 접근 + 임계값 편집 + 리뷰 승인/배정 |
| E 로그아웃 | `logout.spec.ts` | 2 | 쿠키 삭제 + 보호 페이지 재차단 |
| 인프라 (수동) | — | 3 | 0-diff / 단일 명령 실행 / 시크릿 미커밋 |

## 4. 모킹 전략 (OPEN #1/#2/#5 결정)

- **인증 모킹** (OPEN #1 옵션 C): `global-setup.ts` 가 viewer/analyst/admin 역할별 mock JWT 쿠키를 `.auth/{role}.json` 에 저장. 라이브 Keycloak 불필요.
- **JWT 서명 검증 없음** (OPEN #5): SUT `apps/web/src/lib/auth.ts:60` 의 `decodeJwt` 는 페이로드만 디코드. base64 인코드된 payload + 더미 서명으로 충분.
- **BFF API 모킹** (OPEN #2 옵션 D): `fixtures/api-mocks.ts` 의 `mockBffApis(page)` 가 `page.route` 로 `/api/v1/**`, `/api/auth/logout` 응답을 `fixtures/api/*.json` 으로 대체. 라이브 백엔드 불필요.

## 5. 라이브 백엔드 모드 (선택, REQ-E2E-OPT-201)

```bash
# Go control-plane + Postgres + MinIO + Keycloak 가 실행 중인 상태에서:
E2E_LIVE_BACKEND=1 npm run test:e2e
```

본 모드는 best-effort 이며 본 SPEC 합격 조건이 아님 (EXC-7). 라이브 모드 시 mock 응답이 실제 백엔드 응답과 다르면 일부 AC 가 깨질 수 있음.

## 6. 새 시나리오 추가 가이드

1. `selectors.ts` 의 `LABELS` 에 SUT 한국어 텍스트 추가
2. 필요 시 `fixtures/api/*.json` 에 mock 응답 추가 + `api-mocks.ts` 에 라우트 등록
3. 새 `*.spec.ts` 작성 — 기존 spec 의 패턴 참고:
   - `import { test, expect } from "./fixtures/auth"` (role 별 page)
   - `test.beforeEach(async ({ viewerPage }) => await mockBffApis(viewerPage))`
4. 셀렉터는 텍스트/role 기반만 — `data-testid` 추가 금지 (EXC-6)

## 7. 셀렉터 안정성 정책

- **[HARD]** SUT(`apps/web/src/**`) 수정 금지 (REQ-E2E-NO-300). 셀렉터가 깨지면 `test.fixme` 로 마킹하고 후속 SPEC(SPEC-AX-WEB-002 가칭) 트리거.
- 현재 일부 흐름 테스트(`flow-analyst.spec.ts` AC-FLOW-ANALYST-001/002, `flow-admin.spec.ts` AC-FLOW-ADMIN-001/002/003)는 SUT 폼 진입 경로/필드 구조에 의존하므로 SUT 변경 시 fixme 가 활성화될 수 있음.

## 8. 0-diff 검증 (AC-INF-001)

본 SPEC 머지 직전 다음 명령으로 0-diff 확인:

```bash
cd /home/sklee/moai/iroum-ax
git diff main -- apps/control-plane/ apps/web/src/ pipelines/
# 출력이 비어 있어야 통과
```

## 9. 알려진 제약 (EXC-1 ~ EXC-10)

- 시각 회귀(EXC-1), 부하 테스트(EXC-2), CI 통합(EXC-3), 모바일 뷰포트(EXC-4), 접근성(EXC-5), `data-testid` 추가(EXC-6), 라이브 백엔드 통합(EXC-7), DB 시드 도구(EXC-8), 침투 테스트(EXC-9), 다국어(EXC-10) — 모두 별도 SPEC.

## 10. 디렉터리 구조

```
apps/web/e2e/
├── README.md                  (본 파일)
├── global-setup.ts            (3 role storageState 사전 생성)
├── selectors.ts               (한국어 텍스트 상수)
├── fixtures/
│   ├── auth.ts                (role 별 page fixture)
│   ├── api-mocks.ts           (BFF page.route 헬퍼)
│   └── api/
│       ├── evidences.json
│       ├── evaluation-items.json
│       ├── scores.json
│       ├── reviews.json
│       ├── audit-logs.json
│       └── rubric-thresholds.json
├── .auth/                     (.gitignore — storageState 캐시)
├── auth.spec.ts               (그룹 A — 3 AC)
├── rbac-viewer.spec.ts        (그룹 B — 7 AC)
├── flow-analyst.spec.ts       (그룹 C — 4 AC)
├── flow-admin.spec.ts         (그룹 D — 5 AC)
└── logout.spec.ts             (그룹 E — 2 AC)
```
