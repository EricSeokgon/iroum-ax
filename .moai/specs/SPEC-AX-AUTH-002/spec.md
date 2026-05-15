---
id: SPEC-AX-AUTH-002
version: 0.1.0
status: draft
created: 2026-05-15
updated: 2026-05-15
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-15): SPEC-AX-AUTH-001 v0.1.1 GREEN 후속. AUTH-001 Sprint 5에서 `rbac.go`(`ParseRolesFromScope`/`EffectivePermissions`/`Authorize`/`LogForbidden`) + 18 unit tests + 100% 핵심 함수 커버리지 완성되었으나, REST/gRPC 핸들러 체인에 `Authorize()` 호출이 wiring되지 않아 `viewer` 토큰으로 `DELETE /api/v1/workflows/{id}` 호출이 차단되지 않는 보안 결함 잔존. AUTH-001 Sprint 7 `TestE2E_Auth_RBACForbidden`가 `t.Skip("SPEC-AX-AUTH-002: RBAC REST handler wiring deferred — see TODO above")`로 명시적 위임됨(`auth_e2e_test.go:370`). 본 SPEC은 method/path → Permission 매핑 테이블 + REST Authz Middleware + gRPC Authz Interceptor + AUTH-001 E2E SKIP unblock으로 KEPCO E&C 운영 배포 prerequisite를 충족한다. RBAC 정책 자체는 AUTH-001 §3.5 매트릭스를 신뢰(재정의 금지)하며, 본 SPEC은 wiring 계층만 추가한다. Composite domain: AX + AUTH. Sprint Contract Exclusion 9개 명시(ABAC / 권한 캐싱 / 위임 / 보고서 자동화 / per-resource ACL / row-level / cell-level / 동적 권한 매트릭스 / 권한 재할당 propagation). (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001/SPEC-AX-CTRL-001/SPEC-AX-AUTH-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 같은 변형 필드는 canonical schema에 없으므로 plan-auditor 결함 제기 시 본 메모와 `plan.md` L378을 출처로 거부한다. (lessons_session_2026_05_14 #1 적용)

---

# SPEC-AX-AUTH-002 — RBAC REST/gRPC Handler 통합

## 1. 개요

`apps/control-plane/internal/server/` 계층에 RBAC `Authorize()` 호출 wiring을 도입한다. SPEC-AX-AUTH-001가 정의한 3-role 권한 매트릭스(`admin` / `analyst` / `viewer`)는 라이브러리 형태로 완성되어 있으나, REST `Mux` 및 gRPC `UnaryServerInterceptor` 체인에 연결되지 않아 권한 부족 요청이 차단되지 않는 상태이다. 본 SPEC은 method + path → required Permission 매핑을 코드로 명시하고, REST/gRPC 양 진입점에서 인증 통과 직후·핸들러 진입 전에 `auth.Authorize(ctx, requiredPerm)`를 호출하여 권한 부족 시 `403 Forbidden` / `codes.PermissionDenied`를 반환하며 `AUTH_FORBIDDEN` 감사 이벤트를 기록한다.

### 1.1 본 SPEC의 위상

- SPEC-AX-AUTH-001 §5 Exclusion 영역에 없던 "핸들러 wiring" 항목을 명시적으로 해소한다.
- AUTH-001 Sprint 7 `TestE2E_Auth_RBACForbidden`(`auth_e2e_test.go:354~371`)가 본 SPEC 후속 처리를 명시하며 `t.Skip` 상태이다. 본 SPEC S3에서 unblock한다.
- AUTH-001 §4 NFR "성능 — RBAC 검사 p99 < 1ms"를 재이용하며, 본 SPEC은 lookup table 조회만 추가하므로 추가 NFR 변경 없음.

### 1.2 운영 컨텍스트 (Why now)

| 동인 | 출처 | 본 SPEC 대응 |
|------|------|-------------|
| KEPCO E&C 운영 배포 시 viewer 사용자가 DELETE 호출 가능한 보안 결함 차단 | `product.md` §6.1 Go-Live + AUTH-001 §1.2 PISA/PIPA | REQ-AUTH2-002 / REQ-AUTH2-003 (사전 차단) |
| 감사원 요구: 차단된 접근 시도도 추적 가능 | AUTH-001 REQ-AUTH-UBI-001 + §3.4 감사원 추적성 | REQ-AUTH2-UBI-001 (모든 결정 audit) |
| AuthEnabled=false 환경 backward compatibility 유지 | AUTH-001 REQ-AUTH-UBI-001 + SPEC-AX-001 REQ-UBI-003 | REQ-AUTH2-UBI-001 (권한 체크 skip) |
| 다중 사용자(경영평가팀 5-10명) 권한 격리 | `product.md` §4.1 + AUTH-001 §1.4 3-role 카디널리티 | REQ-AUTH2-001 매트릭스 |
| AUTH-001 E2E SKIP unblock (TestE2E_Auth_RBACForbidden) | `auth_e2e_test.go:354~371` | REQ-AUTH2-004 |

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `AUTH` (인증/인가 sub-domain — AUTH-001과 동일 도메인)
- SPEC ID: `SPEC-AX-AUTH-002` (도메인 카디널리티 2, plan.md L366 권장 범위 내)

### 1.4 한국 공공 도메인 6 제약 영향 평가 (lessons #8 적용)

- 망분리: 본 SPEC은 외부 API 호출 0건 (lookup table은 in-process). 영향 없음.
- PIPA audit_logs: 모든 권한 결정(grant/deny) 기록 (REQ-AUTH2-UBI-001) — 강화.
- 합니다체: 사용자 노출 에러 메시지(`insufficient_scope`)는 영문 표준 + 한국어 audit detail. 합니다체 미해당.
- HWP / 한자한글 / 등급 시뮬레이션: 본 SPEC 무관 (인가 계층).

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 디렉토리 트리를 따른다. 본 SPEC은 **신규 파일 2개**(`apps/control-plane/internal/server/authz.go` + `authz_test.go`)를 추가하며, 기존 파일(`grpc_server.go`, `rest_handler.go`, `cmd/server/server.go`)에는 wiring 줄을 수~십 줄 단위로 추가한다. Delta 마커는 Run phase에서 정확한 라인 단위로 결정.

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | 모듈 | 신규/수정 |
|------|------|------|---------|
| `apps/control-plane/internal/server/authz.go` | method/path → Permission 매핑 lookup 테이블, REST `RESTAuthzMiddleware`, gRPC `UnaryAuthzInterceptor`, 매핑 미정의 시 default-deny 처리 헬퍼 | REQ-AUTH2-001~003 | 신규 |
| `apps/control-plane/internal/server/authz_test.go` | 매핑 테이블 단위 테스트(positive/negative/unknown method) + middleware/interceptor 단위 테스트(httptest + bufconn) | REQ-AUTH2-001~003 + UBI-001 | 신규 |
| `apps/control-plane/internal/server/rest_handler.go` | `Mux()`에서 `auth.RESTMiddleware` 다음 `server.RESTAuthzMiddleware`를 chain하도록 wiring 한 줄 추가. `Mux()` 등록부에 HEAD/OPTIONS(CORS preflight) bypass 처리 명시 | REQ-AUTH2-002 + UBI-001 | 수정 (소규모) |
| `apps/control-plane/internal/server/grpc_server.go` | wiring 변경 없음 — interceptor 등록은 `cmd/server/server.go`에서 처리 | - | 미수정 |
| `apps/control-plane/cmd/server/server.go` | gRPC 서버 등록 시 `grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor(...), server.UnaryAuthzInterceptor(...))` 순서로 wiring | REQ-AUTH2-003 | 수정 (소규모) |
| `apps/control-plane/internal/server/auth_e2e_test.go` | `TestE2E_Auth_RBACForbidden`의 `t.Skip(...)` 제거 + AC-AUTH2-004-1/2/3 시나리오 추가 (viewer DELETE → 403 / analyst GET → 200 / admin DELETE → 200) | REQ-AUTH2-004 | 수정 (소규모) |
| `apps/control-plane/internal/audit/actions.go` | `ActionAuthForbidden` 상수 이미 존재(AUTH-001 도입). 본 SPEC은 재사용만, 신규 정의 없음 | - | 미수정 |

### 2.2 Python Pipelines (`pipelines/`)

본 SPEC은 Go control-plane 핸들러 wiring에 한정한다. FastAPI 측 RBAC wiring은 후속 SPEC(`SPEC-AX-AUTH-PY-001`)으로 분리하여 scope discipline 유지.

### 2.3 Shared (`pkg/`, `schemas/`)

| 경로 | 책임 | 신규/수정 |
|------|------|---------|
| `schemas/openapi/openapi.yaml` | `403 Forbidden` 응답 스키마 추가 (이미 AUTH-001에서 401만 정의) + `WWW-Authenticate` 헤더 명세 보강 | 수정 (소규모) |

### 2.4 Deployments / Database

본 SPEC은 schema 변경 없음. 인프라 변경 없음.

### 2.5 Tests

| 경로 | 책임 | 모듈 |
|------|------|------|
| `apps/control-plane/internal/server/authz_test.go` | 매핑 lookup 테이블 (positive/negative/unknown), middleware 짧은 시나리오 | REQ-AUTH2-001/002/003 + UBI-001 |
| `apps/control-plane/internal/server/rest_handler_test.go` | 기존 파일에 RBAC 통합 시나리오 3건 추가: viewer POST → 403, analyst POST → 201, missing user (auth bypass 우회 시도) → 401 | REQ-AUTH2-002 |
| `apps/control-plane/internal/server/grpc_server_test.go` | bufconn 기반 RBAC 시나리오 3건 추가 | REQ-AUTH2-003 |
| `apps/control-plane/internal/server/auth_e2e_test.go` | SKIP 제거 + viewer DELETE / analyst GET / admin DELETE 3 시나리오 활성화 | REQ-AUTH2-004 |

---

## 3. EARS 요구사항

EARS 5개 패턴(Ubiquitous / Event-driven / State-driven / Optional / Unwanted) 모두 본 SPEC 내 포함.

### 3.1 Ubiquitous Requirements (시스템 전반 불변 조건)

**REQ-AUTH2-UBI-001 (권한 결정 추적성 + 백워드 호환 + 사전 차단)**

본 UBI는 3개 sub-clause를 가지며, 각 sub-clause는 acceptance.md에서 dedicated AC를 가진다(lessons #2 적용).

- **REQ-AUTH2-UBI-001-a (모든 권한 결정 audit)**: The system SHALL record every authorization decision (grant or deny) to `audit_logs` when AuthEnabled=true. Grant 결정은 핸들러 성공 audit row의 부속 정보로 통합되며(별도 row 미생성, 성능 보존), deny 결정은 dedicated `AUTH_FORBIDDEN` row를 1건 INSERT한다. 동일 transaction 내에서 commit하여 atomicity 보장.
- **REQ-AUTH2-UBI-001-b (AuthEnabled=false 권한 체크 skip — 백워드 호환)**: WHILE AuthEnabled=false (validator=nil, sandbox), the system SHALL bypass `auth.Authorize()` 호출 entirely and treat every request as authorized, preserving SPEC-AX-001 REQ-UBI-003 + SPEC-AX-CTRL-001 AC-CTRL-UBI-002-C + SPEC-AX-AUTH-001 AC-AUTH-UBI-001-C의 `cli-anonymous` 폴백 결과를 byte-identical로 유지한다.
- **REQ-AUTH2-UBI-001-c (사전 차단 — 인증 후 · 핸들러 진입 전)**: The system SHALL execute authorization check AFTER authentication middleware/interceptor injects validated user context AND BEFORE the business handler executes. 핸들러 진입 후 권한 거부는 비즈니스 부작용(예: DB write) 발생 후 차단이 되어 transactional integrity를 위협하므로 금지.

### 3.2 REQ-AUTH2-001 — Method-Permission Mapping

#### Ubiquitous

- **REQ-AUTH2-001-U1**: The system SHALL define method/path → Permission mapping in code (Go source), not in runtime configuration file. 매핑 변경은 SPEC 변경을 요구하며, 운영 hot-reload 금지(보안 결정의 immutable한 출처).

#### Event-driven

- **REQ-AUTH2-001-E1 (REST mapping)**: WHEN an HTTP request arrives matching the routes below, THEN the system SHALL resolve the required Permission per the canonical table:

| HTTP Method | Path Pattern | Required Permission | Roles allowed (AUTH-001 §3.5 매트릭스 reference) |
|-------------|--------------|--------------------|--------------------------------------------------|
| POST | `/api/v1/workflows` | `write:workflow` | admin, analyst |
| GET | `/api/v1/workflows/{id}` | `read:workflow` | admin, analyst, viewer |
| GET | `/api/v1/workflows` | `read:workflow` | admin, analyst, viewer |
| DELETE | `/api/v1/workflows/{id}` | `delete:workflow` | admin only |
| POST | `/api/v1/recommendations/{id}/feedback` | `write:recommendation` | admin, analyst |
| POST | `/api/v1/documents/upload` | `write:workflow` | admin, analyst (업로드는 워크플로우 생성 selfsame) |
| GET | `/health` | (no auth required) | bypass (AUTH-001 REQ-AUTH-003-E2와 동일) |
| GET | `/metrics` | (no auth required) | bypass (Prometheus scrape — 내부망 한정, 운영 정책 후속) |
| HEAD / OPTIONS | `*` (any path) | (no auth required) | bypass (CORS preflight; 핸들러는 별도 처리 없음) |

- **REQ-AUTH2-001-E2 (gRPC mapping)**: WHEN a gRPC unary RPC arrives, THEN the system SHALL resolve the required Permission per the canonical table:

| gRPC FullMethod | Required Permission | Roles allowed |
|-----------------|--------------------|---------------|
| `/iroum.ax.v1.WorkflowService/CreateWorkflow` | `write:workflow` | admin, analyst |
| `/iroum.ax.v1.WorkflowService/GetWorkflow` | `read:workflow` | admin, analyst, viewer |
| `/iroum.ax.v1.WorkflowService/ListWorkflows` | `read:workflow` | admin, analyst, viewer |
| `/grpc.health.v1.Health/Check` | (no auth required) | bypass (AUTH-001 REQ-AUTH-003-E1과 동일) |

#### Unwanted

- **REQ-AUTH2-001-U1 (Default-deny safety net)**: IF the incoming method+path (REST) or `info.FullMethod` (gRPC) is NOT defined in the canonical mapping above AND AuthEnabled=true, THEN the system SHALL return HTTP 503 Service Unavailable (REST) / `codes.Unavailable` (gRPC) with response body `{"error":{"code":"AUTHZ_MAPPING_MISSING","message":"authorization mapping not defined for this method"}}`, log `AUTH_FORBIDDEN` with reason `authz_mapping_missing`, AND SHALL NOT execute the handler. (`open-by-default` 패턴을 명시적으로 거부; 매핑 누락은 운영이 아닌 개발 시점 결함이므로 503가 적절 — 401/403은 사용자에게 권한 부족을 시사하여 혼선 유발).

### 3.3 REQ-AUTH2-002 — REST Authorize Wrapper

#### Event-driven

- **REQ-AUTH2-002-E1**: WHEN an HTTP request passes `auth.RESTMiddleware` (AUTH-001 REQ-AUTH-003-E2) successfully and reaches `server.RESTAuthzMiddleware`, THEN the middleware SHALL extract the authenticated `*auth.User` from `r.Context()` via `auth.UserFromContext()`, resolve the required Permission via the mapping table from REQ-AUTH2-001-E1, call `auth.Authorize(ctx, requiredPerm)`, and on success invoke `next.ServeHTTP(w, r)`.

- **REQ-AUTH2-002-E2 (Audit on success)**: WHEN authorization succeeds, THEN the middleware SHALL annotate the request context with `granted_permission` for downstream handler audit logging integration (handler가 작성하는 기존 audit row의 `details.granted_permission` 필드로 합쳐짐), AND SHALL NOT insert a separate audit row (REQ-AUTH2-UBI-001-a grant 경로 적용).

#### Unwanted

- **REQ-AUTH2-002-U1 (Forbidden response)**: IF `auth.Authorize()` returns `auth.ErrInsufficientPermission`, THEN the middleware SHALL return HTTP 403 Forbidden with response body `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"<perm>","granted":["<role>",...]}}}`, set header `WWW-Authenticate: Bearer realm="iroum-ax", error="insufficient_scope", scope="<requiredPerm>"`, insert `AUTH_FORBIDDEN` audit row via `auth.LogForbidden` with `details.method`, `details.path`, `details.required`, `details.granted_roles`, AND SHALL NOT call `next.ServeHTTP`.

- **REQ-AUTH2-002-U2 (Missing user context — defense in depth)**: IF `auth.UserFromContext(ctx)` returns `ok=false` AND AuthEnabled=true (validator non-nil), THEN the middleware SHALL return HTTP 500 Internal Server Error with body `{"error":{"code":"AUTHZ_USER_MISSING","message":"authenticated user context not propagated"}}` AND log `AUTH_FORBIDDEN` reason=`user_context_missing`. (정상 흐름에서는 `auth.RESTMiddleware`가 user를 주입했어야 함 — 본 경로 진입은 미들웨어 wiring 버그 detection.)

### 3.4 REQ-AUTH2-003 — gRPC Authorize Interceptor

#### Event-driven

- **REQ-AUTH2-003-E1**: WHEN a gRPC unary RPC passes `auth.UnaryServerInterceptor` (AUTH-001 REQ-AUTH-003-E1) and reaches `server.UnaryAuthzInterceptor`, THEN the interceptor SHALL extract the authenticated user from context, resolve the required Permission via the mapping from REQ-AUTH2-001-E2 using `info.FullMethod`, call `auth.Authorize(ctx, requiredPerm)`, and on success invoke `handler(ctx, req)`. Interceptor chaining order: `auth.UnaryServerInterceptor` → `server.UnaryAuthzInterceptor` → business handler (`grpc.ChainUnaryInterceptor`).

#### State-driven

- **REQ-AUTH2-003-S1 (Health check bypass)**: WHILE `info.FullMethod == "/grpc.health.v1.Health/Check"`, THE interceptor SHALL bypass authorization (mirrors AUTH-001 REQ-AUTH-003-E1 health bypass).

#### Unwanted

- **REQ-AUTH2-003-U1 (Forbidden response)**: IF `auth.Authorize()` returns `auth.ErrInsufficientPermission`, THEN the interceptor SHALL return `status.Errorf(codes.PermissionDenied, "insufficient scope: required=%s granted=%v", requiredPerm, grantedRoles)`, insert `AUTH_FORBIDDEN` audit row, AND SHALL NOT invoke the downstream handler.

### 3.5 REQ-AUTH2-004 — E2E Forbidden Verification (AUTH-001 SKIP Unblock)

#### Event-driven

- **REQ-AUTH2-004-E1 (viewer DELETE → 403)**: WHEN a request `DELETE /api/v1/workflows/{id}` arrives with a valid `iroum-ax:viewer` token and AuthEnabled=true, THEN the system SHALL return HTTP 403 Forbidden, NOT delete the workflow row in `workflows` table, AND insert exactly 1 `AUTH_FORBIDDEN` row in `audit_logs` with `details.required=delete:workflow` and `details.granted_roles=["viewer"]`.

- **REQ-AUTH2-004-E2 (analyst GET → 200)**: WHEN a request `GET /api/v1/workflows` arrives with a valid `iroum-ax:analyst` token, THEN the system SHALL return HTTP 200 with the workflows list (analyst는 read:workflow 보유).

- **REQ-AUTH2-004-E3 (admin DELETE → 200/204)**: WHEN a request `DELETE /api/v1/workflows/{id}` arrives with a valid `iroum-ax:admin` token AND the workflow exists, THEN the system SHALL return HTTP 200 (or 204 depending on REST contract), delete the workflow, AND insert `WORKFLOW_DELETED` audit row with `user_id` from token sub. (DELETE 핸들러 자체는 본 SPEC 범위 외; 본 AC는 RBAC wiring만 검증하며 DELETE 핸들러는 stub/501 또는 후속 SPEC. 본 SPEC에서는 admin이 RBAC 단을 통과한다는 사실만 검증 — 통과 후 핸들러 부재로 501/404 가능, 단 403은 아니어야 함.)

#### Unwanted

- **REQ-AUTH2-004-U1 (AUTH-001 Sprint 7 SKIP unblock)**: IF `auth_e2e_test.go` `TestE2E_Auth_RBACForbidden`가 본 SPEC 구현 후에도 `t.Skip(...)` 상태이면, RUN phase는 incomplete로 판정. 본 SPEC GREEN 종료 조건에 SKIP 제거가 포함된다(cross-SPEC artifact 변경 — lessons #5 적용).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 성능 — RBAC lookup | method+path → Permission lookup p99 < 100µs (in-memory map) | string ops only |
| 성능 — Authorize wrapper 전체 | middleware 진입~next 호출까지 p99 < 1ms (AUTH-001 REQ-AUTH-004 NFR 재사용) | AUTH-001 §4 |
| 보안 — Default-deny | 매핑 미정의 method는 503 응답, 절대 200/handler 진입 금지 | REQ-AUTH2-001-U1 |
| 보안 — Audit completeness | 모든 deny 결정은 `AUTH_FORBIDDEN` row 1건 생성 (REQ-AUTH2-UBI-001-a) | PIPA |
| 보안 — Order of operations | 권한 체크는 핸들러 진입 전 (REQ-AUTH2-UBI-001-c) | 비즈니스 부작용 차단 |
| 백워드 호환성 | AuthEnabled=false 시 SPEC-AX-AUTH-001 AC-AUTH-UBI-001-C + SPEC-AX-CTRL-001 모든 AC가 unchanged 통과 | regression invariant |
| 망분리 정합 | 외부 API 호출 0건 (lookup table은 in-process) | `tech.md` §9.1 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | DDD 또는 TDD (quality.yaml development_mode 따름; AUTH-001과 동일 모드 유지) | `quality.yaml` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC에서 다룬다 (target ≥ 8 충족: 9개 명시).

1. **ABAC (Attribute-Based Access Control)** — 자원별 ownership(예: "본인이 만든 workflow만 조회"), 시간 기반 권한, 위치 기반 권한 등. 본 SPEC은 RBAC만. 후속 SPEC `SPEC-AX-AUTH-ABAC-001`.
2. **동적 권한 매트릭스 (런타임 변경)** — Keycloak admin UI 혹은 별도 console에서 매트릭스를 hot-reload. 본 SPEC은 Go source 명시만 허용(REQ-AUTH2-001-U1). 후속 SPEC에서 dynamic policy engine 검토 시 OPA 등 별도 평가.
3. **권한 캐싱 (Authorize 호출 결과 캐시)** — 동일 user/method 조합의 결과를 Redis/in-memory에 caching. 본 SPEC은 매 요청마다 `Authorize()` 호출 (in-memory map lookup이 p99 < 100µs이므로 캐시 불필요). 운영 후 측정 기반 후속 최적화.
4. **권한 위임 (Impersonation / Service Account Assumption)** — admin이 다른 사용자 토큰을 가장 발급. 후속 SPEC.
5. **임시 권한 grant (time-bounded elevation)** — 1회성 admin elevation 요청-승인 흐름. 후속 SPEC.
6. **감사 보고서 자동 생성** — `AUTH_FORBIDDEN` row를 집계하여 일/주/월 단위 PDF 보고서 생성, email 발송. AUTH-001 §5 Exclusion #8과 동일 — 본 SPEC도 미해소.
7. **권한 자동 재할당 propagation (사용자 역할 변경 시 활성 토큰 즉시 무효화)** — Keycloak admin에서 사용자 role 변경 시 활성 access token이 자동 invalidate되어야 함. 본 SPEC은 토큰 만료(1h)까지 대기; 즉시 propagation은 Keycloak Admin Events + back-channel logout 필요. 본 SPEC 범위 외(Keycloak 책임).
8. **Per-resource ACL (특정 workflow의 owner만 수정 가능)** — 자원 instance 단위 권한. 본 SPEC은 method/path 단위만. ABAC sub-set으로 후속 SPEC.
9. **라인/셀 레벨 권한 (row-level / cell-level security)** — 동일 테이블 내 row 중 일부만 조회 가능. PostgreSQL RLS 또는 application layer filter. 본 SPEC RBAC 범위 외, 멀티테넌트 SPEC과 함께 후속.

---

## 6. 의존성 및 전제

- **SPEC-AX-AUTH-001 v0.1.1 GREEN 가정**: `rbac.go`의 `ParseRolesFromScope` / `EffectivePermissions` / `Authorize` / `LogForbidden`이 그대로 사용 가능. 본 SPEC은 wiring 계층만 추가.
- **SPEC-AX-CTRL-001 GREEN 가정**: gRPC + REST + TxCoordinator + audit.Recorder가 GREEN. 본 SPEC에서 변경 없음.
- **SPEC-AX-001 GREEN 가정**: `audit_logs` schema + `cli-anonymous` 폴백.
- **AUTH-001 Sprint 7 SKIP 마커**: `auth_e2e_test.go:354~371`에 명시된 TODO를 본 SPEC에서 unblock.
- **Go 의존성**: 신규 의존성 없음 (`google.golang.org/grpc`, `net/http` 표준만 사용).
- **Database**: schema 변경 없음.
- **MX tags**: `authz.go` 매핑 lookup 함수에 `@MX:ANCHOR`(fan_in 예상 ≥ 4: REST/gRPC/테스트/audit), `RESTAuthzMiddleware`에 `@MX:NOTE`, default-deny 분기에 `@MX:WARN` + `@MX:REASON` 명시 (mx-tag-protocol.md 적용; `code_comments: ko` 적용).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- DELETE /api/v1/workflows/{id} 핸들러 구현: 본 SPEC은 RBAC wiring만. DELETE 핸들러 자체는 후속 SPEC(`SPEC-AX-WF-DELETE-001`). REQ-AUTH2-004-E3는 admin이 RBAC 단을 통과하는지만 검증.
- Python FastAPI RBAC wiring: 후속 SPEC `SPEC-AX-AUTH-PY-001`. 본 SPEC은 Go control-plane만.
- gRPC streaming RPC RBAC: 본 SPEC은 unary만. streaming RPC 도입 시(현재 없음) 후속 SPEC `SPEC-AX-AUTH-STREAM-001`.
- Keycloak realm role → JWT scope 매핑 설정: Keycloak admin 책임. 본 SPEC은 JWT scope 클레임이 `iroum-ax:(admin|analyst|viewer)` 패턴이라는 AUTH-001 §3.5 invariant를 신뢰.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/internal/server/authz_test.go` — 매핑 테이블 (positive/negative/unknown method 조합), middleware 짧은 시나리오 (httptest), interceptor (bufconn)
- 통합 테스트: 기존 `rest_handler_test.go` + `grpc_server_test.go`에 RBAC 시나리오 추가
- E2E 테스트: `auth_e2e_test.go` `TestE2E_Auth_RBACForbidden` 활성화 + viewer DELETE / analyst GET / admin DELETE 3 시나리오
- 백워드 호환성 테스트: AuthEnabled=false 시 본 SPEC 진입점 모두 bypass — 기존 `auth_e2e_test.go` `TestE2E_Auth_AnonymousFallback` regression 보존
- 성능 측정: `go test -bench=BenchmarkAuthzMapping` (lookup p99 < 100µs 측정)
- 보안 검증: default-deny 매핑 누락 시나리오 explicit AC (REQ-AUTH2-001-U1), 핸들러 진입 전 차단 검증 (REQ-AUTH2-UBI-001-c)

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
