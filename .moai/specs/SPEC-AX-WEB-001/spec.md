---
id: SPEC-AX-WEB-001
version: 0.2.0
status: complete
created: 2026-05-21
updated: 2026-05-21
completed: 2026-05-21
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.2.0 (2026-05-21): SYNC COMPLETE — 6 Phase 구현 완료, status draft → complete. Phase A(인증/BFF/scaffold 35파일) + Phase B(증빙 7파일) + Phase C(평가항목+점수 13파일) + Phase D(리포트 6파일) + Phase E(리뷰 11파일) + Phase F(감사로그+루브릭 10파일) + sidebar 경로 수정. 총 71개 파일 신규. 모든 AC GREEN. 백엔드 0-diff [HARD] 준수.
- 0.1.1 (2026-05-21): plan-auditor CONDITIONAL PASS 0.77 후 annotation 결정 반영. OPEN #1 RESOLVED: `workspaces: ["apps/web"]` 대체(A). OPEN #2 RESOLVED: `realm-export.json` 수정 허용(백엔드 0-diff 예외 명시). OPEN #3 RESOLVED: HttpOnly 쿠키 BFF 방식 채택(선택지 A — Next.js Route Handler BFF). 누락 AC 4건 추가(AC-052, AC-073, AC-074 + 로그아웃 AC). §10 Affected Files 갱신.
- 0.1.0 (2026-05-21): PoC 데모용 웹 대시보드 프런트엔드(Web Dashboard Frontend) 첫 초안. 15 SPEC 완료된 Go control-plane(`apps/control-plane/`, gRPC :50051 + REST :8080)이 노출한 `/api/v1/*` 엔드포인트(AUTH/EVIDENCE/EVAL-ITEM/SCORE/REPORT/REVIEW/RUBRIC/AUDIT-QUERY)를 한국 공공 기관(KEPCO E&C) 평가자(analyst)·열람자(viewer)·관리자(admin) 3-역할(SPEC-AX-AUTH-001 RBAC 정합)에게 노출하는 **PoC 데모 5-스크린 Next.js 14+ App Router 프런트엔드**를 신규 디렉터리 `apps/web/`에 추가한다. 핵심 화면 5개: (1) 로그인(Keycloak OIDC redirect), (2) 증빙 업로드·목록, (3) 평가 항목 트리 + 점수 입력, (4) 범주 리포트 뷰, (5) 리뷰 워크플로(Kanban). admin 전용 감사 로그 뷰어(7번째). 인증은 Keycloak 24.x SSO/JWT(`iroum-ax:{admin,analyst,viewer}` scope) 위임, 토큰 자동 갱신 투과. **본 SPEC은 Go control-plane API의 순수 consumer이며 어떤 백엔드 코드·schema·API 계약도 변경하지 않는다 — 백엔드 0-diff(HARD)**. 신규 워크스페이스(`apps/web/`) 추가, 루트 `package.json` `workspaces` 필드 확장 1줄(Open #1로 확정), 어떤 백엔드 Go 파일도 수정 0. WebSocket 실시간 알림, Excel/HWP 임포트 UI, 다국어(한국어 외), 모바일 반응형, CI/CD 파이프라인은 §3 비목표에서 의도적 제외. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-AUDIT-QUERY-001 등 7 SPEC 누적과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. 본 SPEC은 신규 프런트엔드 디렉터리(`apps/web/`)를 추가하는 greenfield 작업이므로 기존 코드에 대한 phantom-API 검증 부담은 없으나, 모든 REST 엔드포인트는 사용자 제공 카탈로그(8 도메인 × 31 endpoints)에 명시된 것만 사용한다 — `apps/web/`에서 호출하는 모든 `/api/v1/*` 경로는 본 SPEC §1.3 카탈로그에 등재된 경로로만 한정된다(phantom endpoint 0건 목표).

---

# SPEC-AX-WEB-001 — PoC 데모 웹 대시보드 프런트엔드 (Web Dashboard Frontend for PoC Demo)

## 1. 개요 (Overview)

한국 공공 기관(KEPCO E&C) 경영평가 PoC의 평가자(analyst)·열람자(viewer)·관리자(admin)가 15 SPEC 완료된 Go control-plane(`apps/control-plane/`, gRPC :50051 + REST :8080)이 노출하는 8 도메인 31 REST 엔드포인트(`/api/v1/*`)를 브라우저에서 시연·검증할 수 있도록, **PoC 데모 범위 5-스크린 Next.js 14+ App Router 프런트엔드**를 신규 디렉터리 `apps/web/`에 추가한다. 본 SPEC은 SPEC-AX-AUTH-001(Keycloak 24.x SSO/JWT)·SPEC-AX-AUTH-002(RBAC 3-역할)·SPEC-AX-AUTH-003(경량 ABAC narrowing)·SPEC-AX-EVID-001(증빙 업로드)·SPEC-AX-EVAL-ITEM-001(평가 항목 taxonomy)·SPEC-AX-SCORE-001/SCORE-API-001(점수)·SPEC-AX-REPORT-001(리포트)·SPEC-AX-REVIEW-001(리뷰 워크플로)·SPEC-AX-RUBRIC-001(루브릭 임계값)·SPEC-AX-AUDIT-QUERY-001(감사 로그 검색)이 GREEN(완료) 상태로 가정한다 — 본 SPEC은 그 **consumer**이며 백엔드 코드·schema·audit·Action 상수·RBAC·ABAC을 일절 변경하지 않는다.

### 1.1 본 SPEC의 의미와 PoC 범위

본 SPEC의 1차 산출물은 **시연 가능한 5-스크린 데모 UI + 인증/권한 통합 + 토큰 자동 갱신**이다. 시연 시나리오: (1) viewer/analyst/admin 사용자가 Keycloak SSO로 로그인 → (2) 평가 항목 트리에서 항목 선택 → (3) analyst가 증빙 업로드 + 점수 입력 → (4) admin이 리뷰 제출/승인 → (5) viewer가 범주 리포트 + 등급 확인 → (6) admin이 감사 로그 조회. 이 워크플로 1회 시연이 PoC 성공의 정의이다.

PoC 데모이므로 다음은 의식적으로 **간소화**한다:
- 시각 디자인: shadcn/ui 기본 테마 + Tailwind 유틸리티 기반, 별도 디자인 시스템 구축 없음
- 데스크톱 first(1280×800 기준), 모바일 반응형은 §3 비목표
- 한국어 단일 언어(i18n 라이브러리 도입 없이 정적 한국어 텍스트)
- 클라이언트-사이드 상태는 React Server Components + URL search params + Server Actions 우선, Redux/Zustand 등 전역 상태 라이브러리 미도입(React Query/TanStack Query만 데이터 패칭에 도입)

### 1.2 Anchor 컨텍스트

본 SPEC은 15 SPEC 누적 Go control-plane이 형성한 REST API 표면(`apps/control-plane/cmd/server/server.go`에 마운트된 `/api/v1/*` 31 endpoints)의 **UI consumer**이다. 백엔드 SPEC 15개가 만든 것은 API + 데이터 영속성 + 감사 추적성; 본 SPEC이 만드는 것은 그 API의 시각화 + 사용자 입력 경로. 두 책임은 분리된다.

### 1.3 소비 대상 REST API 카탈로그 (사용자 제공, 8 도메인 31 endpoints)

| 도메인 | Method | Path | 사용 화면 |
|--------|--------|------|----------|
| 인증 | POST | `/api/v1/auth/token` | 로그인 콜백 |
| 인증 | POST | `/api/v1/auth/refresh` | 토큰 자동 갱신 |
| 인증 | POST | `/api/v1/auth/logout` | 로그아웃 |
| 증빙 | POST | `/api/v1/evidences` | 증빙 업로드 |
| 증빙 | GET | `/api/v1/evidences` | 증빙 목록 |
| 증빙 | GET | `/api/v1/evidences/{id}` | 증빙 상세 |
| 평가항목 | GET | `/api/v1/evaluation-items` | 트리 탐색 |
| 평가항목 | GET | `/api/v1/evaluation-items/{id}` | 항목 상세 |
| 평가항목 | POST | `/api/v1/evaluation-items` | (admin) 항목 추가 — 본 PoC는 read-only 우선, 추가는 §3 비목표 |
| 점수 | POST | `/api/v1/scores` | 점수 입력 |
| 점수 | GET | `/api/v1/scores` | 점수 목록 |
| 점수 | GET | `/api/v1/scores/{id}` | 점수 상세 |
| 점수 | PUT | `/api/v1/scores/{id}` | 점수 수정 |
| 점수 | POST | `/api/v1/scores/{id}/supersede` | 점수 supersede |
| 점수 | GET | `/api/v1/scores/rollup` | 범주 rollup |
| 점수 | GET | `/api/v1/scores/grade` | 등급 산출 |
| 리포트 | GET | `/api/v1/reports/category/{id}` | 범주 리포트 |
| 리뷰 | POST | `/api/v1/reviews` | 리뷰 제출 |
| 리뷰 | GET | `/api/v1/reviews` | 리뷰 목록(Kanban) |
| 리뷰 | GET | `/api/v1/reviews/{id}` | 리뷰 상세 |
| 리뷰 | POST | `/api/v1/reviews/{id}/assign-reviewer` | (admin) 리뷰어 배정 |
| 리뷰 | POST | `/api/v1/reviews/{id}/approve` | (admin) 승인 |
| 리뷰 | POST | `/api/v1/reviews/{id}/reject` | (admin) 반려 |
| 루브릭 | GET | `/api/v1/rubric/thresholds` | (admin) 임계값 조회 |
| 루브릭 | POST | `/api/v1/rubric/thresholds` | (admin) 임계값 생성 |
| 루브릭 | PUT | `/api/v1/rubric/thresholds/{scope}` | (admin) 임계값 수정 |
| 감사 | GET | `/api/v1/audit-logs` | (admin) 감사 로그 |
| 메트릭 | GET | `/metrics` | (관리용, UI 비노출) |

phantom endpoint 0건 [HARD]: 본 SPEC `apps/web/`에서 호출하는 모든 REST 경로는 위 카탈로그에 등재된 것으로 한정된다. 새 엔드포인트가 필요하면 별도 백엔드 SPEC을 선행하고 본 SPEC을 후속한다.

### 1.4 권한 경계 — RBAC 3-역할 + ABAC narrowing

본 SPEC은 SPEC-AX-AUTH-001/002/003이 정의한 권한 모델을 UI에서 정확히 반영한다:

- **viewer**: 모든 화면을 **읽기 전용**으로 접근(증빙·점수·리포트·리뷰·평가 항목 트리 GET 만). 입력 폼/버튼은 표시되지 않거나 비활성화(`disabled`).
- **analyst**: viewer 권한 + 증빙 업로드(POST `/evidences`), 점수 입력/수정(POST/PUT `/scores`, `/scores/{id}/supersede`), 리뷰 제출(POST `/reviews`). admin 전용 액션 비표시.
- **admin**: analyst 권한 + 리뷰 승인/반려(POST `/reviews/{id}/approve|reject|assign-reviewer`), 루브릭 임계값 관리(POST/PUT `/rubric/thresholds*`), 감사 로그 조회(GET `/audit-logs`). 평가 항목 추가(POST `/evaluation-items`)는 §3 비목표(향후 SPEC).

UI 권한 표현은 **백엔드를 신뢰의 원천(source of truth)**으로 한다 — 프런트엔드는 JWT의 `scope` 클레임(`iroum-ax:{admin|analyst|viewer}`)을 디코드해 UI를 가시성 제어할 뿐, 모든 mutation 요청의 최종 인가는 백엔드 RBAC/ABAC 미들웨어가 결정한다. 프런트엔드가 권한을 우회해 요청을 보내도 백엔드가 403을 반환하면 UI는 toast로 표시한다. **프런트엔드 권한 검사는 UX 보조이며 보안 경계가 아니다.**

### 1.5 의존성 stub 계약 — consumer-only [핵심, load-bearing]

본 SPEC은 15 SPEC 누적 Go control-plane의 **순수 consumer**이다. 다음이 **HARD 계약**이다:

- **[HARD]** `apps/control-plane/**`(Go 코드, schema 마이그레이션, audit, RBAC/ABAC, OpenAPI/proto 스펙)를 **일절 수정하지 않는다**. 31 endpoints 응답 schema·에러 형식·HTTP status code는 그대로 사용한다. 신규 endpoint·신규 응답 필드 요구 시 별도 백엔드 SPEC을 선행한다.
- **[HARD]** `pipelines/**`(Python AI 파이프라인)도 수정하지 않는다. PoC 데모는 백엔드 API만 통해 데이터를 본다.
- **[HARD]** 신규 백엔드 외부 의존 0건 — `go.mod`/`pyproject.toml` 무수정.
- **[HARD]** Keycloak 24.x 설정(SPEC-AX-AUTH-001) 무수정 — 본 SPEC은 OIDC 클라이언트(Public client + PKCE)로 합류하며, 신규 Realm/Client 구성은 §6 OPEN #2에서 결정한다.
- **신규 워크스페이스만 추가**: `apps/web/` 디렉터리 신규, 루트 `package.json` `workspaces` 배열에 `"apps/web"` 추가(현재 `["apps/console"]` → `["apps/console", "apps/web"]` 또는 `apps/console`을 `apps/web`으로 대체 — §6 OPEN #1 RESOLVED 후 확정). `apps/console` 처리는 OPEN #1에서 결정.

### 1.6 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `WEB` (Web Dashboard sub-domain — 15 SPEC 누적 REST API의 브라우저 UI 계층)
- SPEC ID: `SPEC-AX-WEB-001` (`.claude/skills/moai/workflows/plan.md` Composite domain rules — `AX` + `WEB` 2-domain, 권장 범위 내)

---

## 2. 목표 (Goals)

1. **시연 가능한 5-스크린 PoC 데모 UI 1회 완주**: 로그인 → 증빙 업로드 → 점수 입력 → 리뷰 승인 → 리포트 조회 워크플로를 브라우저에서 끝까지 수행 가능.
2. **3-역할 RBAC UI 가시성**: viewer/analyst/admin 각각의 권한 경계가 UI에 정확히 반영되며, 권한 외 액션은 비표시/비활성/403 toast 처리.
3. **Keycloak SSO + JWT 자동 갱신 투과**: 사용자가 갱신을 의식하지 않는 매끄러운 세션 유지(만료 60초 전 자동 refresh, 갱신 실패 시 로그인 화면으로 안전 복귀).
4. **백엔드 0-diff**: Go control-plane 코드·schema·인증 설정을 한 줄도 수정하지 않고 31 endpoints만 소비.
5. **한국어 UI 일관성**: 모든 사용자 노출 텍스트(라벨/오류/안내)는 한국어. 코드 내 식별자·라이브러리·기술 용어는 영어.

---

## 3. 비목표 (Non-Goals / Exclusions)

본 PoC 데모 SPEC은 다음을 **의도적으로 제외**한다. 향후 SPEC에서 별도 추진:

1. **실시간 알림(WebSocket)**: 리뷰 상태 변경·점수 갱신 푸시 미구현. polling 또는 새로고침으로 갱신.
2. **Excel/HWP 임포트 UI**: HWP 문서 업로드/파싱 UI 미구현 — 백엔드 파이프라인은 별도 채널(CLI/배치)로 트리거. 본 SPEC은 결과(`/evidences`)만 조회.
3. **다국어 지원(영어/일본어 등)**: 한국어 단일. `next-intl`/`react-i18next` 미도입.
4. **모바일 반응형 디자인**: 데스크톱(1280×800 ↑) first. 모바일 뷰포트 최적화·터치 인터랙션 미지원.
5. **CI/CD 파이프라인 통합**: GitHub Actions 워크플로·Docker 이미지 빌드·배포 매니페스트 미구현. 로컬 `npm run dev`/`npm run build`만 동작.
6. **평가 항목 트리 편집 UI** (POST `/evaluation-items` 호출): admin이 신규 평가 항목을 UI로 추가하는 화면 미구현. taxonomy는 백엔드 시드 데이터/CLI로만 관리. 트리는 read-only.
7. **점수 supersede UI** (POST `/scores/{id}/supersede`): 본 PoC는 PUT `/scores/{id}` 단순 수정만 노출. supersede 시나리오는 §6 OPEN #5에서 자동/수동 결정 후 v0.2.0 이상에서 추가.
8. **감사 로그 CSV/XLSX export**: 감사 로그 뷰어는 화면 표시만, 다운로드 미구현(AUDIT-QUERY-001 §5 #6과 정합 — export 미지원).
9. **e2e 자동화 테스트(Playwright)**: 컴포넌트 단위 테스트(Vitest + React Testing Library)만 수행. 브라우저 e2e는 §6 OPEN #6에서 결정 후 별도 SPEC 가능.
10. **성능 부하 테스트·접근성(a11y) 풀 감사**: keyboard navigation·기본 ARIA 라벨만 적용, WCAG 2.1 AA 풀 적합성 검증 미수행.
11. **PWA·오프라인 동작**: Service Worker 미도입.
12. **컴포넌트 스토리북(Storybook)**: 미도입. shadcn/ui 데모 페이지에 의존.

---

## 4. 기술 스택 (Tech Stack)

| 영역 | 선택 | 버전 | 비고 |
|------|------|------|------|
| 프레임워크 | Next.js | 14.2+ (App Router) | React Server Components first, Server Actions 활용 |
| 언어 | TypeScript | 5.4+ | `strict: true`, ES2022 target |
| UI 라이브러리 | React | 18.3+ | RSC + Suspense + use hook |
| 스타일링 | Tailwind CSS | 3.4+ | shadcn/ui 토큰 사용 |
| 컴포넌트 키트 | shadcn/ui | 최신 (cli-installed) | Radix UI primitives 기반, copy-in 방식 |
| 데이터 패칭 | TanStack Query (React Query) | v5 | 서버 상태 캐싱·refetch·optimistic update |
| HTTP 클라이언트 | fetch (Next 표준) + 얇은 wrapper | — | axios 미도입(번들 최소화) |
| 폼·검증 | React Hook Form + Zod | 최신 | 클라이언트 검증, 백엔드 검증 결과 toast 표시 |
| 인증 | OIDC PKCE Public client | — | Keycloak 24.x, `oidc-client-ts` 또는 직접 구현 OPEN #2 |
| JWT 디코드 | `jose` 또는 `jwt-decode` | 최신 | 클라이언트는 디코드만, 검증은 미들웨어/백엔드에 위임 |
| 아이콘 | lucide-react | 최신 | shadcn/ui 기본 |
| 테스트 | Vitest + React Testing Library | 최신 | 단위·컴포넌트 테스트 |
| 린트 | ESLint 9 flat config + Next.js plugin | 최신 | 기존 루트 ESLint 설정 통합 |
| 포맷터 | Prettier | 3.x | 기존 루트 설정 통합 |
| 패키지 매니저 | npm (workspace) | npm 10+ | 루트 `package.json` `workspaces` 확장 |
| Node | Node.js 20 LTS 또는 22 LTS | — | `engines` 필드 명시 |

**번들/성능 가드**:
- 초기 JS 페이로드 목표 < 300KB (gzipped, 로그인 화면 기준)
- Lighthouse 데스크톱 Performance ≥ 80, Accessibility ≥ 90 (PoC 권장치, hard gate 아님)

---

## 5. 요구사항 (Requirements — EARS 형식)

EARS 패턴: Ubiquitous(SHALL) / Event-Driven(WHEN…THEN…SHALL) / State-Driven(WHILE…IF…) / Unwanted(SHALL NOT) / Optional(WHERE…).

### 5.1 인증·세션 (REQ-WEB-001, 008)

- **REQ-WEB-001 (Event-Driven, 로그인)**: 사용자가 `/login` 화면에서 "로그인" 버튼을 클릭하면, 시스템은 Keycloak 24.x OIDC Authorization Code + PKCE 플로우로 redirect하고 콜백 시 `POST /api/v1/auth/token` 으로 코드 교환 후 JWT access/refresh 토큰을 HttpOnly·Secure·SameSite=Lax 쿠키 또는 Next.js Server Action 세션에 저장한다(저장소 결정은 §6 OPEN #3).
- **REQ-WEB-001a (Ubiquitous, scope 디코드)**: 시스템은 access token의 `scope` 클레임에서 `iroum-ax:{admin|analyst|viewer}` 토큰을 파싱하여 클라이언트 측 역할 상태로 보관해야 한다(UI 가시성 제어용, 보안 결정 아님).
- **REQ-WEB-001b (Unwanted, 인증 없는 보호 경로 접근)**: 인증되지 않은 사용자가 `/login` 외의 보호 경로(`/dashboard/**`)에 접근하면 시스템은 자동으로 `/login`으로 리다이렉트해야 하며, 절대로 보호 경로의 데이터를 노출해서는 안 된다(`SHALL NOT`).
- **REQ-WEB-008 (Event-Driven, 토큰 자동 갱신)**: access token 만료 60초 전이 되면, 시스템은 사용자 개입 없이 `POST /api/v1/auth/refresh` 를 자동 호출하여 새 토큰을 받고 갱신해야 한다. 갱신이 실패(refresh token 만료 등)하면 시스템은 사용자를 `/login`으로 리다이렉트하고 현재 작업 컨텍스트(미저장 폼)를 안전하게 폐기한다.
- **REQ-WEB-008a (Event-Driven, 로그아웃)**: 사용자가 "로그아웃" 메뉴를 클릭하면, 시스템은 `POST /api/v1/auth/logout`을 호출하여 토큰을 무효화하고 세션 쿠키를 삭제 후 `/login`으로 redirect한다.

### 5.2 증빙 업로드·목록 (REQ-WEB-002)

- **REQ-WEB-002 (Event-Driven, 증빙 파일 업로드)**: analyst 또는 admin이 `/dashboard/evidences` 화면에서 파일을 드래그-드롭하거나 "파일 선택" 버튼으로 선택한 뒤 "업로드" 버튼을 클릭하면, 시스템은 `multipart/form-data`로 `POST /api/v1/evidences`를 호출해 업로드해야 하며, 단일 파일 최대 100MB(클라이언트 검증)·서버 응답 201 시 목록을 자동 갱신해야 한다.
- **REQ-WEB-002a (Unwanted, 100MB 초과)**: 100MB를 초과하는 파일이 선택되면 시스템은 업로드 요청을 보내지 않고 한국어 오류 toast(`파일 크기는 100MB를 초과할 수 없습니다.`)를 표시해야 한다(`SHALL NOT` 요청 송신).
- **REQ-WEB-002b (Ubiquitous, viewer 입력 차단)**: viewer 역할 사용자에게는 업로드 영역(드롭존·버튼)이 표시되지 않거나 비활성화되어야 한다.
- **REQ-WEB-002c (Event-Driven, 목록 조회)**: 사용자가 `/dashboard/evidences`에 진입하면 시스템은 `GET /api/v1/evidences`를 호출하여 증빙 목록을 페이지네이션(기본 limit=20)으로 표시한다.
- **REQ-WEB-002d (Event-Driven, 상세)**: 사용자가 목록 행을 클릭하면 시스템은 `GET /api/v1/evidences/{id}`를 호출하여 상세 정보를 모달 또는 별도 페이지로 표시한다.

### 5.3 평가 항목 트리 + 점수 입력 (REQ-WEB-003, 004)

- **REQ-WEB-003 (Ubiquitous, 트리 네비게이션)**: 시스템은 `/dashboard/evaluation-items` 화면에 `GET /api/v1/evaluation-items` 응답을 부모-자식 계층 구조로 표시해야 한다(접기/펼치기 가능, 깊이 표시).
- **REQ-WEB-003a (Event-Driven, 항목 선택)**: 사용자가 트리에서 항목을 클릭하면 시스템은 `GET /api/v1/evaluation-items/{id}` 응답을 우측 패널에 표시하고, 해당 항목에 연관된 점수(`GET /api/v1/scores?eval_item_id={id}`)를 함께 로드한다.
- **REQ-WEB-004 (Event-Driven, 점수 입력)**: analyst 또는 admin이 평가 항목 우측 패널의 "점수 입력" 폼에서 점수 값과 코멘트를 입력하고 저장 버튼을 클릭하면, 시스템은 `POST /api/v1/scores`를 호출하여 점수를 등록해야 하며, 201 응답 시 점수 목록을 갱신한다.
- **REQ-WEB-004a (Event-Driven, 점수 수정)**: analyst 또는 admin이 기존 점수 행의 "수정" 버튼을 클릭하고 값을 변경 후 저장하면, 시스템은 `PUT /api/v1/scores/{id}`를 호출하여 갱신한다.
- **REQ-WEB-004b (Unwanted, viewer 입력)**: viewer에게 점수 입력/수정 폼은 표시되지 않거나 비활성화되어야 한다.
- **REQ-WEB-004c (Unwanted, 유효 범위 외 값)**: 백엔드가 정의한 점수 유효 범위(예: 0–100, 실제 범위는 백엔드 응답/검증 메시지에 위임)를 벗어난 값을 입력하면 시스템은 클라이언트에서 즉시 검증 오류를 표시하고 요청을 보내지 않는다.

### 5.4 범주 리포트 뷰 (REQ-WEB-005)

- **REQ-WEB-005 (Event-Driven, 범주 리포트)**: 사용자가 `/dashboard/reports/{categoryId}`에 진입하면 시스템은 `GET /api/v1/reports/category/{categoryId}`를 호출하여 가중치 적용 rollup·등급(grade) 정보를 표시한다. 등급 표시는 백엔드 응답의 `grade` 필드(예: A/B/C/D)를 그대로 렌더링한다.
- **REQ-WEB-005a (Optional, 범주 선택기)**: WHERE 범주 트리가 여러 개인 경우, 사용자는 좌측 사이드바에서 범주를 선택할 수 있어야 한다.
- **REQ-WEB-005b (Ubiquitous, 0건 처리)**: 점수가 0건인 범주는 "데이터 없음" 빈 상태를 표시해야 하며 오류가 아닌 정상 상태로 처리한다.

### 5.5 리뷰 워크플로 보드 (REQ-WEB-006)

- **REQ-WEB-006 (Ubiquitous, Kanban 보드)**: 시스템은 `/dashboard/reviews` 화면에 `GET /api/v1/reviews` 응답을 상태 컬럼(SUBMITTED / UNDER_REVIEW / APPROVED / REJECTED)별 Kanban 보드로 표시해야 한다(SPEC-AX-REVIEW-001 4-state machine 정합).
- **REQ-WEB-006a (Event-Driven, 리뷰 제출)**: analyst 또는 admin이 "리뷰 제출" 버튼을 클릭하면 시스템은 `POST /api/v1/reviews`를 호출하여 SUBMITTED 카드를 보드에 추가한다.
- **REQ-WEB-006b (Event-Driven, 리뷰어 배정)**: admin이 SUBMITTED 상태 카드의 "리뷰어 배정" 액션을 선택하고 리뷰어를 지정하면 시스템은 `POST /api/v1/reviews/{id}/assign-reviewer`를 호출하여 UNDER_REVIEW로 전환한다.
- **REQ-WEB-006c (Event-Driven, 승인)**: admin이 UNDER_REVIEW 상태 카드의 "승인" 버튼을 클릭하면 시스템은 `POST /api/v1/reviews/{id}/approve`를 호출하여 APPROVED 컬럼으로 이동한다(terminal).
- **REQ-WEB-006d (Event-Driven, 반려)**: admin이 UNDER_REVIEW 상태 카드의 "반려" 버튼을 클릭하면 시스템은 반려 사유 입력을 요구하고(필수, 한국어), `POST /api/v1/reviews/{id}/reject`를 호출하여 REJECTED 컬럼으로 이동한다(terminal).
- **REQ-WEB-006e (Unwanted, terminal 상태 변경)**: APPROVED/REJECTED 카드의 상태 변경 액션 버튼은 표시되지 않거나 비활성화되어야 한다(백엔드 state-machine 정합, SHALL NOT 요청 송신).
- **REQ-WEB-006f (Ubiquitous, viewer Kanban 읽기 전용)**: viewer는 보드와 카드를 볼 수 있되 모든 액션 버튼은 비표시.

### 5.6 감사 로그 뷰어 — admin 전용 (REQ-WEB-007)

- **REQ-WEB-007 (Event-Driven, 감사 로그 조회)**: admin이 `/dashboard/audit-logs`에 진입하면 시스템은 `GET /api/v1/audit-logs`를 호출하여 5-필터(`action`/`resource_type`/`resource_id`/`user_id`/time range `since`+`until`) 폼과 결과 테이블을 표시해야 한다.
- **REQ-WEB-007a (Unwanted, viewer/analyst 접근)**: viewer 또는 analyst가 `/dashboard/audit-logs` 경로에 접근하면 시스템은 화면 표시를 차단하고(메뉴 항목 비표시 + 라우트 가드 redirect) `/dashboard`로 안전 복귀해야 한다(`SHALL NOT` 표시). 백엔드도 403을 반환하므로 이중 방어.
- **REQ-WEB-007b (Event-Driven, 필터 적용)**: admin이 필터를 입력하고 "조회" 버튼을 클릭하면 시스템은 query string으로 5-필터 AND 조합을 전달하여 결과 테이블을 재로드한다(AUDIT-QUERY-001 정합).
- **REQ-WEB-007c (Event-Driven, 페이지네이션)**: 결과가 limit(기본 50, 최대 500 — AUDIT-QUERY-001 정합)을 초과하면 다음 페이지/이전 페이지 버튼을 활성화한다.

### 5.7 루브릭 임계값 관리 — admin 전용 (REQ-WEB-009)

- **REQ-WEB-009 (Event-Driven, 임계값 조회/관리)**: admin이 `/dashboard/rubric/thresholds`에 진입하면 시스템은 `GET /api/v1/rubric/thresholds`로 현재 임계값을 표시하고, "수정" 버튼으로 `PUT /api/v1/rubric/thresholds/{scope}`, "신규" 버튼으로 `POST /api/v1/rubric/thresholds`를 호출할 수 있어야 한다.
- **REQ-WEB-009a (Unwanted, viewer/analyst 접근)**: viewer/analyst는 메뉴 비표시 + 라우트 가드 redirect.

### 5.8 공통 횡단 요구사항 (REQ-WEB-CROSS)

- **REQ-WEB-CROSS-001 (Ubiquitous, 에러 한국어 표시)**: 모든 백엔드 에러 응답(`{"error":{"code","message","field"}}`)은 한국어 toast 또는 form-field 인라인 메시지로 표시되어야 한다. `message` 필드를 우선 사용하고 부재 시 `code`에 대한 한국어 사전 매핑을 fallback으로 사용한다.
- **REQ-WEB-CROSS-002 (Ubiquitous, 401/403 처리)**: HTTP 401 응답은 토큰 자동 갱신을 1회 시도하고 실패 시 `/login`으로 redirect. HTTP 403은 toast "이 작업을 수행할 권한이 없습니다."로 표시하고 화면을 그대로 유지한다.
- **REQ-WEB-CROSS-003 (Ubiquitous, 로딩 상태)**: 모든 데이터 fetch 중에는 skeleton 또는 spinner를 표시하고, 사용자 액션이 진행 중일 때 버튼을 비활성화하여 중복 요청을 방지한다.
- **REQ-WEB-CROSS-004 (Optional, 다크 모드)**: WHERE 사용자 OS가 dark mode를 선호하면 시스템은 dark theme을 적용할 수 있다(shadcn/ui 기본 토큰 활용, 별도 토글 UI는 §3 비목표 외).
- **REQ-WEB-CROSS-005 (Unwanted, 시크릿 노출)**: 클라이언트 번들에는 어떤 백엔드 시크릿(DB 비밀번호·서비스 계정 키·Keycloak client secret)도 포함되어서는 안 된다. Public client + PKCE 사용(`client_secret` 없음).

---

## 6. 수락 기준 (Acceptance Criteria — Given/When/Then)

PoC 데모이므로 수동 시연(데모 시나리오 1회 완주) + 컴포넌트 단위 테스트(Vitest) 조합으로 검증한다. e2e 자동화는 §3 비목표.

### 6.1 인증·세션

- **AC-001 (로그인 정상 경로)**: GIVEN 사용자가 `/login` 화면에 있고 Keycloak에 등록된 계정을 보유, WHEN 로그인 버튼 클릭 후 Keycloak에서 자격 증명 입력, THEN 콜백 후 `/dashboard`로 진입하고 우상단에 사용자 이름·역할 배지가 표시된다.
- **AC-002 (보호 경로 가드)**: GIVEN 인증되지 않은 사용자, WHEN 브라우저 주소창에 `/dashboard/evidences` 직접 입력, THEN 응답 본문에 어떤 보호 데이터도 노출되지 않고 `/login`으로 redirect된다.
- **AC-003 (토큰 자동 갱신)**: GIVEN access token이 만료 60초 전인 세션, WHEN 사용자가 그 시점에 API 호출이 필요한 액션 수행, THEN refresh 요청이 자동 송신되고 사용자에게는 갱신 사실이 시각적으로 노출되지 않은 채 액션이 정상 완료된다.
- **AC-004 (refresh 실패 안전 복귀)**: GIVEN refresh token도 만료된 세션, WHEN 자동 갱신 시도 후 실패, THEN `/login`으로 redirect되고 미저장 폼 데이터는 폐기된다.

### 6.2 증빙

- **AC-010 (analyst 업로드 성공)**: GIVEN analyst 로그인, WHEN 50MB PDF 파일을 드롭존에 드래그하고 업로드 클릭, THEN 201 응답 후 목록 첫 행에 새 항목이 표시된다.
- **AC-011 (100MB 초과 차단)**: GIVEN analyst, WHEN 150MB 파일 선택, THEN 업로드 요청이 전송되지 않고 한국어 오류 toast가 표시된다.
- **AC-012 (viewer 업로드 영역 비표시)**: GIVEN viewer 로그인, WHEN `/dashboard/evidences` 진입, THEN 업로드 드롭존·버튼이 화면에 표시되지 않는다.
- **AC-013 (증빙 목록 + 상세)**: GIVEN viewer 또는 analyst, WHEN 목록 행 클릭, THEN 상세 패널/모달에 메타데이터가 표시된다.

### 6.3 평가 항목 + 점수

- **AC-020 (트리 렌더링)**: GIVEN 평가 항목 100개 이상 시드 데이터, WHEN `/dashboard/evaluation-items` 진입, THEN 부모-자식 계층이 접기/펼치기 가능한 트리로 표시되고 초기 렌더링은 2초 이내 완료한다.
- **AC-021 (점수 입력 — analyst)**: GIVEN analyst, WHEN 트리에서 항목 선택 → 점수 입력 폼에 75 입력 → 저장, THEN 201 응답 후 항목의 점수 목록에 새 행이 추가된다.
- **AC-022 (점수 수정 — analyst/admin)**: GIVEN 기존 점수 보유, WHEN 행의 "수정" → 80으로 변경 → 저장, THEN 200 응답 후 행 값이 80으로 갱신된다.
- **AC-023 (viewer 입력 비활성)**: GIVEN viewer, WHEN 항목 선택, THEN 점수 입력 폼이 표시되지 않거나 모든 필드가 disabled이다.
- **AC-024 (백엔드 검증 오류 표시)**: GIVEN 백엔드가 400 + `{"error":{"code":"INVALID_RANGE","message":"점수는 0과 100 사이여야 합니다.","field":"value"}}` 응답, WHEN 사용자가 -5 입력 후 저장, THEN form-field 인라인 메시지로 한국어 오류가 표시된다.

### 6.4 리포트

- **AC-030 (범주 rollup)**: GIVEN admin/analyst/viewer, WHEN `/dashboard/reports/{categoryId}` 진입, THEN 가중치 적용 점수 합계와 등급(A/B/C/D)이 카드 형식으로 표시된다.
- **AC-031 (데이터 없음 빈 상태)**: GIVEN 점수 0건 범주, WHEN 진입, THEN "데이터 없음" 한국어 빈 상태 화면이 표시되고 오류 toast는 발생하지 않는다.

### 6.5 리뷰 워크플로

- **AC-040 (Kanban 표시)**: GIVEN 다양한 상태의 리뷰 데이터, WHEN `/dashboard/reviews` 진입, THEN 4개 상태 컬럼이 표시되고 각 카드가 올바른 컬럼에 위치한다.
- **AC-041 (리뷰 제출 — analyst)**: GIVEN analyst, WHEN "리뷰 제출" 클릭 → 폼 작성 → 제출, THEN SUBMITTED 컬럼에 새 카드 추가.
- **AC-042 (리뷰어 배정 — admin)**: GIVEN admin, WHEN SUBMITTED 카드의 "리뷰어 배정" → 리뷰어 선택, THEN 카드가 UNDER_REVIEW 컬럼으로 이동.
- **AC-043 (승인 — admin)**: GIVEN admin, WHEN UNDER_REVIEW 카드의 "승인" 클릭, THEN APPROVED 컬럼으로 이동.
- **AC-044 (반려 — admin, 사유 필수)**: GIVEN admin, WHEN "반려" 클릭하되 사유 미입력, THEN 폼이 전송되지 않고 사유 필드 인라인 오류 표시. WHEN 사유 입력 후 제출, THEN REJECTED 컬럼으로 이동.
- **AC-045 (terminal 액션 비활성)**: GIVEN admin, WHEN APPROVED/REJECTED 카드 hover, THEN 상태 변경 버튼이 비표시 또는 disabled이다.
- **AC-046 (viewer 읽기 전용)**: GIVEN viewer, WHEN 보드 진입, THEN 모든 액션 버튼이 비표시이며 카드를 열어 상세는 볼 수 있다.

### 6.6 감사 로그 (admin 전용)

- **AC-050 (admin 접근 + 5-필터)**: GIVEN admin, WHEN `/dashboard/audit-logs` 진입 + `action=score.created` 필터 + 시간 범위 입력, THEN 결과 테이블이 5-필터 AND 조합으로 갱신된다.
- **AC-051 (viewer/analyst 차단)**: GIVEN viewer 또는 analyst, WHEN 주소창에 `/dashboard/audit-logs` 직접 입력, THEN 메뉴 비표시이며 라우트 가드가 `/dashboard`로 redirect한다.

### 6.7 루브릭 임계값 (admin 전용)

- **AC-060 (조회 + 수정)**: GIVEN admin, WHEN `/dashboard/rubric/thresholds` 진입, THEN 현재 임계값이 표시되며 "수정" 클릭 시 PUT 호출 후 변경이 반영된다.
- **AC-061 (viewer/analyst 차단)**: AC-051과 동일 패턴.

### 6.8 공통

- **AC-070 (한국어 에러)**: GIVEN 백엔드 4xx/5xx 응답, WHEN 발생, THEN 모든 사용자 노출 메시지는 한국어이다.
- **AC-071 (401 토큰 갱신 1회 + redirect)**: GIVEN 임의 API 호출이 401 반환, WHEN 자동 refresh 1회 시도, THEN 성공 시 원 요청 자동 재시도, 실패 시 `/login` redirect.
- **AC-072 (403 toast)**: GIVEN viewer가 백엔드 우회로 mutation 요청 송신 시도(개발자 도구 등), WHEN 백엔드 403 반환, THEN UI는 한국어 toast 표시하고 화면은 유지한다.

### 6.9 PoC 데모 완주 종합 (Definition of Done)

- **AC-DEMO**: GIVEN 3개 역할 시드 계정(`viewer@iroum`, `analyst@iroum`, `admin@iroum` — 가칭, 실제 ID는 OPEN #2 결정), WHEN `/moai sync` 시점에 시연 시나리오(로그인 → 증빙 업로드 → 점수 입력 → 리뷰 제출/승인 → 리포트 조회 → 감사 로그 조회)를 한 번에 완주, THEN 어떤 단계에서도 콘솔 에러·미처리 promise rejection·5xx 응답이 발생하지 않는다.

---

## 7. 기술 접근법 (Technical Approach)

### 7.1 아키텍처 개요

- **App Router 디렉터리 구조**:
  ```
  apps/web/
    app/
      (auth)/
        login/page.tsx
        callback/page.tsx
      (dashboard)/
        layout.tsx          # 인증 가드 + 좌측 네비 + 우상단 사용자 정보
        page.tsx            # 대시보드 홈
        evidences/page.tsx
        evaluation-items/page.tsx
        scores/...
        reports/[categoryId]/page.tsx
        reviews/page.tsx
        audit-logs/page.tsx (admin only)
        rubric/thresholds/page.tsx (admin only)
      api/
        auth/[...]/route.ts  # Keycloak callback handler (OPEN #3)
    components/
      ui/                    # shadcn/ui 컴포넌트 (button, card, table, dialog 등)
      auth/                  # LoginButton, RoleBadge, SessionGuard
      evidences/             # EvidenceUploader (dropzone), EvidenceList
      eval-items/            # EvalItemTree, EvalItemDetail
      scores/                # ScoreForm, ScoreList
      reports/               # CategoryRollupCard, GradeBadge
      reviews/               # ReviewBoard (Kanban), ReviewCard, ReviewActions
      audit/                 # AuditLogFilters, AuditLogTable
      rubric/                # RubricThresholdsForm
    lib/
      api/                   # fetch wrapper + endpoint clients (typed)
      auth/                  # JWT decode, session helpers
      hooks/                 # useEvidence, useScores, useReviews (React Query)
      i18n/                  # 정적 한국어 메시지 사전 (단일 ko.ts)
    types/                   # backend response 타입 (수동 정의 또는 OpenAPI 생성 — OPEN #4)
    tests/                   # *.test.tsx (Vitest + RTL)
    public/
    package.json
    next.config.mjs
    tsconfig.json
    tailwind.config.ts
    components.json (shadcn/ui)
  ```

### 7.2 인증·세션 처리 흐름

1. 사용자 `/login` → "로그인" 클릭 → Keycloak Authorization Endpoint (PKCE `code_challenge` 생성)
2. Keycloak 자격 증명 입력 → callback `/api/auth/callback?code=…&state=…`
3. callback 라우트가 `POST /api/v1/auth/token`으로 코드 교환(client_id + code_verifier)
4. 응답의 access/refresh token을 HttpOnly·Secure·SameSite=Lax 쿠키에 저장(서버 측, Next.js Server Action 또는 Route Handler) — 클라이언트 JS 접근 불가
5. 클라이언트 측은 `/api/auth/me` (자체 BFF endpoint) 호출하여 role·name만 받아 보관
6. 모든 백엔드 API 호출은 Next.js Route Handler 또는 Server Action을 경유(쿠키 자동 첨부, CSRF: SameSite=Lax + Origin 검증)
   - 또는 — 단순화를 위해 클라이언트가 직접 백엔드 호출하되 토큰은 메모리에만 보관(refresh는 Next BFF endpoint를 경유) — 최종 결정은 OPEN #3
7. 만료 60초 전 자동 refresh: Server-side에서 만료 검사 → refresh 호출 → 쿠키 갱신. 클라이언트에는 투과.
8. 로그아웃: 쿠키 삭제 + `POST /api/v1/auth/logout` 호출 + `/login` redirect.

### 7.3 데이터 패칭 패턴

- **읽기**: React Server Component에서 `fetch` 직접 호출(쿠키 자동 첨부). Suspense + streaming.
- **목록/필터/페이지네이션**: URL search params(`?limit=20&offset=40&action=score.created`) ↔ Server Component re-render. Client Component는 router.push로 URL 갱신.
- **mutation**: Server Action 또는 client에서 React Query useMutation → 성공 시 invalidateQueries 또는 router.refresh().
- **에러 처리**: `lib/api/fetch.ts` wrapper가 4xx/5xx를 표준화된 `ApiError` throw, 컴포넌트는 ErrorBoundary 또는 React Query `onError`로 catch → toast.

### 7.4 RBAC UI 가시성 패턴

```
<RoleGate allow={['admin','analyst']}>
  <ScoreForm ... />
</RoleGate>
```

- `RoleGate` 컴포넌트는 클라이언트 세션의 role을 검사하여 children을 conditional render.
- 라우트 가드는 `(dashboard)/layout.tsx` 또는 admin-only 페이지의 server component 진입부에서 role 검사 후 `redirect('/dashboard')`.
- **이중 방어**: 프런트 가드 우회 시도에도 백엔드 RBAC/ABAC가 403을 반환하므로 데이터는 안전.

### 7.5 한국어 메시지 사전

- `lib/i18n/ko.ts`에 정적 객체 `{ errors: { INVALID_RANGE: '점수는 0과 100 사이여야 합니다.', ... }, ui: { ... } }` 단순 export. 동적 i18n 인프라(`next-intl`) 미도입.
- 백엔드 응답의 `error.message`가 이미 한국어인 경우 그대로 사용, 영어인 경우 `error.code` 기반 사전 매핑 적용.

### 7.6 구현 단계(Phase 분할 — 우선순위 라벨)

- **Phase A — Priority High**: 워크스페이스 부트스트랩(`apps/web/` scaffolding, Next.js + TS + Tailwind + shadcn/ui 초기 설치, ESLint/Prettier 통합, 루트 package.json workspace 갱신), 인증 베이스(REQ-WEB-001/001a/001b/008/008a — `(auth)/login`, callback handler, JWT 디코드, RoleGate, SessionGuard, 401/403 cross-cutting handler). AC-001~004, AC-070~072 통과.
- **Phase B — Priority High**: 증빙 화면(REQ-WEB-002 family). AC-010~013 통과.
- **Phase C — Priority High**: 평가 항목 트리 + 점수 입력(REQ-WEB-003/004 family). AC-020~024 통과.
- **Phase D — Priority Medium**: 리포트(REQ-WEB-005) + 리뷰 워크플로(REQ-WEB-006). AC-030~031, AC-040~046 통과.
- **Phase E — Priority Medium**: 감사 로그(REQ-WEB-007) + 루브릭(REQ-WEB-009). AC-050~051, AC-060~061 통과.
- **Phase F — Priority Low (Sync)**: 데모 시나리오 1회 완주 검증(AC-DEMO), Lighthouse 점검, 문서화(README), `/moai sync`.

각 Phase는 독립 PR 단위 권장. Phase A 완료 후 다음 Phase 병렬화 가능.

### 7.7 테스트 전략

- **컴포넌트 단위(Vitest + RTL)**: 각 화면의 권한별 가시성, 폼 검증, 에러 toast, 빈 상태 렌더링을 컴포넌트 레벨에서 검증.
- **API wrapper 단위**: `lib/api/fetch.ts`의 4xx/5xx 정규화, 401 자동 refresh 1회, 한국어 메시지 매핑.
- **수동 시연**: 3개 시드 계정으로 데모 시나리오(AC-DEMO) 1회 완주.
- **e2e 자동화(Playwright)는 §3 비목표**(OPEN #6에서 후속 SPEC 결정).

### 7.8 보안 가드

- Public OIDC client + PKCE — `client_secret` 클라이언트 번들 포함 0건(REQ-WEB-CROSS-005).
- HttpOnly·Secure·SameSite=Lax 쿠키 사용 시(OPEN #3 결정) XSS 토큰 탈취 방어.
- CSP(Content Security Policy) 헤더는 Next.js `headers()` config로 `default-src 'self'` 기반 보수적 설정 시작.
- 모든 외부 호출은 `/api/v1/*` Go control-plane만, 신규 외부 도메인 호출 0.

---

## 8. 의존성 (Dependencies)

### 8.1 백엔드 SPEC 의존(완료 가정)

- SPEC-AX-001 (Keycloak compose) — Keycloak 24.x 인프라
- SPEC-AX-AUTH-001 — Keycloak SSO/JWT 발급 + scope 형식 `iroum-ax:{admin|analyst|viewer}`
- SPEC-AX-AUTH-002 — RBAC 3-역할 미들웨어
- SPEC-AX-AUTH-003 — 경량 ABAC narrowing
- SPEC-AX-CTRL-001 / SERVER-001 — HTTP :8080 + 라우팅 베이스
- SPEC-AX-EVID-001 — 증빙 업로드 + 목록
- SPEC-AX-EVAL-ITEM-001 — 평가 항목 taxonomy + 트리
- SPEC-AX-SCORE-001 / SCORE-API-001 — 점수 store + HTTP API
- SPEC-AX-REPORT-001 — 범주 rollup + grade
- SPEC-AX-REVIEW-001 — 4-state 리뷰 워크플로
- SPEC-AX-RUBRIC-001 — 임계값 관리
- SPEC-AX-AUDIT-QUERY-001 — 감사 로그 검색

### 8.2 외부 의존(신규 npm 패키지)

- next (14.2+), react (18.3+), react-dom
- typescript (5.4+)
- tailwindcss + postcss + autoprefixer
- shadcn/ui 컴포넌트(설치형, 의존: @radix-ui/* + class-variance-authority + tailwind-merge + clsx)
- @tanstack/react-query (v5)
- react-hook-form + zod
- jose 또는 jwt-decode
- lucide-react
- (dev) vitest + @testing-library/react + jsdom
- (dev) eslint-config-next + prettier-plugin-tailwindcss

### 8.3 도구·런타임

- Node.js 20 LTS 또는 22 LTS
- npm 10+ (workspace)
- Keycloak 24.x (개발 환경에서 실행 중이어야 함)
- Go control-plane(`apps/control-plane/`) :8080에서 실행 중

---

## 9. 미결 사항 (Open Items)

본 PoC SPEC의 자유도가 일부 남아 있으며, Plan annotation 단계 또는 Phase A 초입에 결정해야 한다.

### OPEN #1 — workspace 정합 [RESOLVED 2026-05-21]

- **결정**: 선택지 A — 루트 `package.json` `"workspaces": ["apps/web"]`로 대체. 미존재 `apps/console` 참조 제거.
- Phase A 부트스트랩에서 `package.json` 1줄 수정 포함.

### OPEN #2 — Keycloak Realm/Client 구성 [RESOLVED 2026-05-21]

- **결정**: `deployments/keycloak/realm-export.json` 수정 허용 — 백엔드 0-diff 정책 예외로 명시.
- 신규 OIDC Public Client(`iroum-ax-web`, PKCE, redirect URI `http://localhost:3000/api/auth/callback`)를 realm-export.json에 추가.
- §10 Affected Files `deployments/**` → `[MODIFY — realm-export.json (OIDC web client 추가)]`로 갱신.

### OPEN #3 — 토큰 저장 방식 [RESOLVED 2026-05-21]

- **결정**: 선택지 A — HttpOnly 쿠키 BFF 방식 채택.
- Next.js Route Handler(`/api/auth/**`)가 BFF 역할. access/refresh 토큰을 HttpOnly·Secure·SameSite=Lax 쿠키에 보관. 클라이언트 JS는 토큰 직접 접근 불가. XSS 토큰 탈취 방어.
- 구현 영향: Phase A에서 `/api/auth/login`, `/api/auth/callback`, `/api/auth/refresh`, `/api/auth/logout` Route Handler 4개 구현 필수.

### OPEN #4 — 타입 동기화 전략: 수동 정의 vs OpenAPI 생성 (Priority Medium)

- 백엔드 OpenAPI 스펙(`apps/control-plane/api/openapi.yaml` 등) 존재 여부 확인 필요(Phase A 초입).
- 존재하면 `openapi-typescript` 또는 `orval`로 자동 생성 권장.
- 부재하면 수동으로 31 endpoints 응답 타입 정의(`types/api.ts` 단일 파일).

### OPEN #5 — 점수 supersede UI 처리 방침 (Priority Low — 본 PoC 비목표 §3 #7이나 Phase D 결정으로 확장 가능)

- 본 PoC는 PUT `/scores/{id}` 단순 수정만 제공. supersede 시나리오(이전 점수 보존 + 새 버전 생성)는 v0.2.0 이상에서 별도 SPEC.

### OPEN #6 — e2e 자동화 도입 여부 (Priority Low — §3 비목표 #9, 후속 SPEC 가능)

- Playwright 도입 시 시연 시나리오 자동화 가능. 본 PoC 범위 외.

### OPEN #7 — 다크 모드 토글 UI (Priority Low)

- REQ-WEB-CROSS-004는 OS 선호도 자동 적용 선택사항. 명시적 토글 버튼 추가 여부는 Phase D/E에서 결정.

### OPEN #8 — 컨테이너화·배포 매니페스트 (Priority Low — §3 비목표 #5)

- `Dockerfile`·`docker-compose` 항목 추가는 별도 운영 SPEC에서.

---

## 10. 참고 — 영향받는 파일 요약 (Affected Files Snapshot)

| 경로 | Delta | 비고 |
|------|-------|------|
| `apps/web/**` | [NEW] | 신규 워크스페이스 전체 |
| `package.json` (루트) | [MODIFY] | `workspaces` 배열 1줄 수정(OPEN #1 RESOLVED 후) |
| `apps/control-plane/**` | [UNCHANGED] | **백엔드 0-diff [HARD]** |
| `pipelines/**` | [UNCHANGED] | Python 파이프라인 무수정 |
| `deployments/keycloak/realm-export.json` | [MODIFY] | OIDC Public Client 추가 — 백엔드 0-diff 예외(OPEN #2 RESOLVED) |
| `.moai/specs/SPEC-AX-WEB-001/{spec,plan,acceptance}.md` | [NEW] | 3-file SPEC 구조 |

---

## 11. 본 SPEC 범위 명시 (Scope Boundary)

본 SPEC은 다음에 한정된다:
- `apps/web/` 신규 Next.js 14+ 프런트엔드 워크스페이스 추가
- 5 데모 화면(로그인·증빙·평가 항목/점수·리포트·리뷰) + admin 전용 2 화면(감사 로그·루브릭 임계값) UI 구현
- Keycloak SSO + JWT 자동 갱신
- RBAC 3-역할 UI 가시성
- 한국어 단일 언어

본 SPEC은 다음을 다루지 않는다:
- §3 비목표 12개 항목(WebSocket, Excel/HWP 임포트 UI, 다국어, 모바일, CI/CD, 평가항목 편집 UI, 점수 supersede UI, 감사로그 export, e2e Playwright, a11y 풀 감사, PWA, Storybook)
- 백엔드 코드·schema·인증 인프라 변경
- 운영 배포·모니터링·관측

향후 SPEC 후보: SPEC-AX-WEB-002 (e2e + a11y), SPEC-AX-WEB-003 (실시간/WebSocket), SPEC-AX-WEB-004 (다국어/i18n), SPEC-AX-WEB-005 (HWP 임포트 UI), SPEC-AX-OPS-001 (배포/CI/CD).
