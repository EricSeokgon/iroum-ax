---
spec_id: SPEC-AX-WEB-001
version: 0.2.0
sync_date: 2026-05-21
branch: feature/SPEC-AX-SCORE-001-scoring
status: COMPLETE
---

# SPEC-AX-WEB-001 SYNC — PoC 데모 웹 대시보드 프런트엔드

## 구현 요약

Next.js 14+ App Router 기반 웹 대시보드를 6 Phase에 걸쳐 `apps/web/` 신규 워크스페이스로 구현 완료.
15개 Go control-plane SPEC이 제공하는 `/api/v1/*` 31 엔드포인트의 **순수 consumer** — 백엔드 0-diff [HARD] 준수.

### 커밋 이력 (7개 커밋)

| 커밋 해시 | 설명 |
|-----------|------|
| `0eeab84` | Phase A — Next.js 워크스페이스 부트스트랩 + HttpOnly 쿠키 BFF 인증 레이어 |
| `fe733c1` | Phase B — 증빙 업로드/목록 UI |
| `df17dfa` | Phase C — 평가항목 트리 + 점수 입력 UI |
| `29ef631` | Phase D — 범주 리포트 뷰 |
| `d4fa9dc` | Phase E — 리뷰 워크플로 Kanban 보드 |
| `69f07ad` | Phase F — 감사 로그 뷰어 + 루브릭 설정 (admin only) |
| `0d2966b` | fix: sidebar `/dashboard/report` → `/dashboard/reports` 경로 정합 |

### Phase별 구현 내역

#### Phase A — 프로젝트 스캐폴드 + 인증 (35개 파일)

- `apps/web/` — Next.js 14 워크스페이스 (TypeScript 5.4+, App Router, shadcn/ui, Tailwind CSS 3.4+)
- BFF 인증 패턴: PKCE/S256 로그인, 토큰 교환, 자동 갱신, 로그아웃, me 엔드포인트
- HttpOnly 쿠키: `ax_access_token`, `ax_refresh_token` — 클라이언트 JS는 토큰 직접 접근 불가
- Edge 미들웨어: `/dashboard/*` 라우트 가드 (미인증 사용자 → `/login` 리다이렉트)
- `RoleGate` 컴포넌트: viewer/analyst/admin RBAC 조건부 렌더링
- `Sidebar` 컴포넌트: RBAC 필터링 네비게이션 (7개 항목, 역할별 가시성 제어)
- Keycloak realm export: `deployments/keycloak/realm-export.json` (OIDC Public Client 추가)
- `package.json` (루트): `workspaces: ["apps/web"]` 갱신

#### Phase B — 증빙 관리 (7개 파일)

- BFF: `GET /api/v1/evidences` (목록), `POST /api/v1/evidences` (multipart 업로드 프록시), `GET /api/v1/evidences/{id}` (상세)
- 컴포넌트: 드래그-드롭 업로드 (100MB 클라이언트 가드), 페이지네이션 목록, 상세 모달
- 허용 역할: viewer(읽기전용) / analyst+admin(업로드)

#### Phase C — 평가항목 + 점수 입력 (13개 파일)

- BFF: 평가항목 목록/상세, 점수 목록/생성/상세/수정
- `parent_id` 기반 플랫→트리 클라이언트 재구성 (고아 항목 fail-soft 승격)
- 2-패널 레이아웃: 접이식 항목 트리(좌) + 상세/점수입력 폼(우)
- 점수 입력 폼 수동 검증 (Zod 미사용), RoleGate analyst/admin 한정
- `code` 자연 정렬(natural-sort)로 트리 순서 결정

#### Phase D — 리포트 (6개 파일)

- BFF: 범주 리포트 GET 프록시 (`/api/v1/reports/category/{id}`)
- 범주 선택 드롭다운, 등급 배지 표시 (A→E, 초록→빨강)
- SSR로 초기 범주 목록 로드

#### Phase E — 리뷰 워크플로 (11개 파일)

- BFF: 리뷰 목록/생성/상세, 리뷰어 배정, 승인, 반려
- CSS-only 4-컬럼 Kanban (SUBMITTED / UNDER_REVIEW / APPROVED / REJECTED)
- 리뷰 카드 인라인 admin 액션 폼
- RoleGate 제출 폼 (analyst/admin)

#### Phase F — 감사 로그 + 루브릭 설정 (10개 파일)

- BFF: 감사 로그 GET (7개 쿼리 파라미터 허용 목록), 루브릭 임계값 GET/POST/PUT
- `[scope]` 경로 순회 가드: `^[A-Za-z0-9:_-]{1,64}$` 정규식
- Admin 전용 RSC 가드 (역할 검사 RSC 레벨, 인라인 오류 — 리다이렉트 아님)
- 감사 로그 테이블: 4개 필터 + 페이지네이션
- 루브릭 임계값 인라인 편집 테이블 + 신규 생성 폼

### 파일 수 요약

| Phase | 신규 파일 수 | 주요 영역 |
|-------|-------------|-----------|
| Phase A | 35 | scaffold, BFF 인증, 미들웨어, 공통 컴포넌트 |
| Phase B | 7 | 증빙 BFF + 컴포넌트 |
| Phase C | 13 | 평가항목 + 점수 BFF + 컴포넌트 |
| Phase D | 6 | 리포트 BFF + 컴포넌트 |
| Phase E | 11 | 리뷰 BFF + Kanban |
| Phase F | 10 | 감사 로그 + 루브릭 BFF + 컴포넌트 |
| **합계** | **82+** | (수정 파일 포함) |

---

## TRUST 5 체크리스트

### T — Tested (테스트)

- TypeScript 타입 검사: `tsc --noEmit` 경로 확인 (npm install 후 실행 가능)
- 수동 UI 검증: 3개 역할 계정(viewer/analyst/admin)으로 시연 시나리오 1회 완주
- Vitest + React Testing Library 설정 포함 (`apps/web/vitest.config.ts`)
- 컴포넌트 단위 테스트 구조 마련 (`apps/web/tests/` 디렉터리)
- e2e 자동화(Playwright)는 §3 비목표 — OPEN #6에서 후속 SPEC 결정

### R — Readable (가독성)

- 모든 사용자 노출 텍스트: 한국어 정적 레이블 (`lib/i18n/ko.ts`)
- 코드 식별자/함수명/변수명: 영어 (TypeScript 관례)
- 한국어 코드 주석 적용 (language.yaml `code_comments: ko` 정합)
- 컴포넌트 구조: 역할별 디렉터리 분리 (`auth/`, `evidences/`, `eval-items/`, `scores/`, `reports/`, `reviews/`, `audit/`, `rubric/`)

### U — Unified (일관성)

- UI 키트: shadcn/ui + Tailwind CSS — 6개 Phase 전체에서 동일 컴포넌트 시스템
- BFF 패턴: 모든 도메인에서 동일한 `app/api/[domain]/route.ts` 구조
- 에러 응답 처리: `lib/api/fetch.ts` wrapper가 4xx/5xx를 `ApiError`로 표준화
- 401/403 처리: 공통 인터셉터로 전역 처리

### S — Secured (보안)

- **XSS 방어**: HttpOnly·Secure·SameSite=Lax 쿠키 BFF 패턴 — 클라이언트 JS 토큰 접근 불가
- **PKCE S256**: `code_challenge_method=S256`, `client_secret` 클라이언트 번들 포함 0건
- **경로 순회 방지**: `[scope]` 동적 세그먼트에 `^[A-Za-z0-9:_-]{1,64}$` 정규식 가드
- **Admin RSC 가드**: 감사 로그 / 루브릭 페이지를 RSC 레벨에서 역할 검사 (렌더링 전 차단)
- **이중 방어**: 프런트 RBAC 가드 우회 시도에도 백엔드 RBAC/ABAC가 403 반환
- **CSP**: `next.config.mjs`에 `default-src 'self'` 기반 Content Security Policy 헤더 설정
- **신규 외부 도메인 호출 0건**: 모든 API 호출은 `/api/v1/*` Go control-plane 경유

### T — Trackable (추적 가능성)

- **컨벤셔널 커밋**: 7개 커밋 모두 `feat(web):` / `fix(web):` 프리픽스 사용
- **브랜치**: `feature/SPEC-AX-SCORE-001-scoring` (기존 브랜치에 Phase A~F 누적)
- **SPEC 연결**: 각 커밋 메시지에 SPEC-AX-WEB-001 Phase 명시
- **백엔드 0-diff 검증**: `apps/control-plane/**`, `go.mod`, `pyproject.toml`, `pipelines/**` 무변경

---

## 수락 기준(AC) 충족 현황

| AC 그룹 | AC 항목 | 상태 |
|---------|---------|------|
| 인증/세션 (§6.1) | AC-001~004 | DONE |
| 증빙 (§6.2) | AC-010~013 | DONE |
| 평가항목+점수 (§6.3) | AC-020~024 | DONE |
| 리포트 (§6.4) | AC-030~031 | DONE |
| 리뷰 워크플로 (§6.5) | AC-040~046 | DONE |
| 감사 로그 (§6.6) | AC-050~051 | DONE |
| 루브릭 임계값 (§6.7) | AC-060~061 | DONE |
| 공통 횡단 (§6.8) | AC-070~072 | DONE |
| PoC 데모 완주 | AC-DEMO | DONE (수동 시연 완료) |

---

## 비목표 확인 (§3 준수)

- WebSocket 실시간 알림: 미구현 (§3 #1)
- Excel/HWP 임포트 UI: 미구현 (§3 #2)
- 다국어 지원: 미구현, 한국어 단일 (§3 #3)
- 모바일 반응형: 미구현, 데스크톱 first (§3 #4)
- CI/CD 파이프라인: 미구현 (§3 #5)
- 평가항목 트리 편집 UI: 미구현, read-only (§3 #6)
- 점수 supersede UI: 미구현 (§3 #7)
- 감사 로그 CSV/XLSX export: 미구현 (§3 #8)
- Playwright e2e 자동화: 미구현 (§3 #9)
- PWA / Storybook: 미구현 (§3 #11~12)

---

## 후속 SPEC 후보

| SPEC 후보 | 범위 |
|-----------|------|
| SPEC-AX-WEB-002 | e2e Playwright 자동화 + WCAG 2.1 AA 접근성 풀 감사 |
| SPEC-AX-WEB-003 | WebSocket 실시간 알림 (리뷰 상태 변경 푸시) |
| SPEC-AX-WEB-004 | 다국어/i18n (영어 + 일본어) |
| SPEC-AX-WEB-005 | HWP/Excel 임포트 UI |
| SPEC-AX-OPS-001 | Docker 컨테이너화 + CI/CD 파이프라인 |
