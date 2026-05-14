---
id: SPEC-AX-AUTH-001
version: 0.1.1
status: draft
created: 2026-05-14
updated: 2026-05-14
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.1 (2026-05-14): plan-auditor iter 1 PASS 0.88 + evaluator-active CONFIRM 0.782 후속 보정. SF-1 (REQ-AUTH-001-E1에 `iss` 클레임 검증 추가, RFC 7519 §4.1.1 + OWASP JWT cheat sheet 요구) + SF-2 (AC-AUTH-001-alg-cross-check 신규 — JWKS key 타입과 token `alg` 헤더 cross-check, RSA key + ES256 claim 변형 검증) + D1 (§2.1 affected files: `internal/scheduler/dispatcher.go` → `internal/scheduler/celery_envelope.go` — Celery envelope builder boundary 정합, plan.md S6 + research.md Decision 3과 cross-doc 일관성). SF-3/D2~D6는 비차단 advisory로 Sprint S1 RED 시작 시 함께 처리 예정. (작성자: ircp)
- 0.1.0 (2026-05-14): SPEC-AX-001 + SPEC-AX-CTRL-001 후속. 두 선행 SPEC이 `cli-anonymous` 폴백을 sandbox 정합으로 도입한 상태에서, KEPCO E&C 운영 배포 전제(감사원 추적성, PISA/PIPA 준수)를 위한 SSO/JWT 인증 + 기초 RBAC 도입. Composite domain: AX + AUTH. Keycloak 24.x LTS를 OIDC provider로 선정(망분리 정합 + 한국 운영 사례 다수). 외부 OAuth(Google/Microsoft)·MFA·SAML·전자정부 표준 인증은 후속 SPEC. AuthEnabled=false 시 기존 cli-anonymous 폴백 유지(backward compatible). (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001/SPEC-AX-CTRL-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field 정의(id, version, status, created, updated, author, priority, issue_number)를 따른다.

---

# SPEC-AX-AUTH-001 — SSO/JWT 인증 + 기초 RBAC

## 1. 개요

`apps/control-plane/`(Go)와 `pipelines/`(Python) 양 계층에 OIDC(Keycloak) 기반 SSO + JWT 검증 + 3-role RBAC을 도입한다. 본 SPEC은 **운영 배포 전제** 충족을 위한 보안 계층이며, SPEC-AX-001 / SPEC-AX-CTRL-001가 정의한 `cli-anonymous` 폴백 경로를 **호환 유지**하면서 인증이 활성화된 환경에서는 검증된 토큰 `sub` 클레임을 `audit_logs.user_id`까지 propagation한다.

### 1.1 본 SPEC의 위상

- SPEC-AX-001 §5 Exclusion #12 (`SSO/JWT 인증`)을 명시적으로 해소한다.
- SPEC-AX-CTRL-001 §5 Exclusion #2 (`Authentication / Authorization / SSO / JWT`)를 명시적으로 해소한다.
- 두 선행 SPEC의 Sprint Contract Exclusion 명시 항목을 본 SPEC 도입으로 cross-link 완결한다.

### 1.2 운영 컨텍스트 (Why now)

| 동인 | 출처 | 본 SPEC 대응 |
|------|------|-------------|
| 감사원 추적성 (모든 audit_logs row가 실 사용자 식별 필요) | `tech.md` §9.4 PISA | REQ-AUTH-UBI-001 + REQ-AUTH-003 |
| PIPA(개인정보보호법) 접근 제어 의무 | `tech.md` §9.4 | REQ-AUTH-004 (RBAC) |
| 망분리 정합 (외부 OAuth 금지) | `tech.md` §9.1 | §5 Exclusion #1 (외부 OAuth providers) |
| KEPCO E&C 운영 배포 prerequisite | `product.md` §6.1 Go-Live | 전체 |
| 다중 사용자 동시 접근 (경영평가팀 5-10명) | `product.md` §4.1 | REQ-AUTH-004 분리된 역할 |

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `AUTH` (인증/인가 sub-domain)
- 따라서 SPEC ID: `SPEC-AX-AUTH-001` (2 domains, plan.md L366 "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내)

### 1.4 Walking Skeleton 정의 (본 SPEC 범위)

- 단일 OIDC provider: Keycloak 24.x LTS (자체 호스팅, 망분리 정합)
- 토큰 알고리즘: RS256 또는 EdDSA(EC) — HS256 대칭키 제외
- 역할 카디널리티: 3개 (`admin` / `analyst` / `viewer`)
- 인증 강제 범위: Go control-plane (gRPC + REST) + Python FastAPI 보호된 endpoint
- Celery worker user_id propagation: envelope `headers.user_id` 헤더 사용 (Python 측 추가 인자 없이 task signal handler에서 추출)

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 디렉토리 트리를 따른다. 본 SPEC은 **신규 디렉토리 2개**(`apps/control-plane/internal/auth/`, `pipelines/auth/`)를 추가하며, 기존 파일에는 **인증 미들웨어 등록 hook**만 1줄~수 줄 단위로 추가한다 (Delta 마커는 Run phase에서 정확한 라인 단위로 결정).

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | 모듈 | 신규/수정 |
|------|------|------|---------|
| `apps/control-plane/internal/auth/verifier.go` | JWT 서명 검증(RS256/EdDSA), exp/nbf/iat/aud 클레임 검증, clock skew 30초 허용 | REQ-AUTH-001 | 신규 |
| `apps/control-plane/internal/auth/jwks_cache.go` | JWKS endpoint fetch + 1시간 TTL 캐시 + 키 회전 처리 | REQ-AUTH-001 | 신규 |
| `apps/control-plane/internal/auth/oidc_client.go` | OIDC Discovery (`/.well-known/openid-configuration`) + provider metadata 캐시 | REQ-AUTH-002 | 신규 |
| `apps/control-plane/internal/auth/blacklist.go` | Redis 기반 토큰 블랙리스트 (logout 시), TTL = 토큰 exp - now | REQ-AUTH-001, REQ-AUTH-005 | 신규 |
| `apps/control-plane/internal/auth/rbac.go` | 역할-권한 매트릭스, scope 클레임 파싱, AuthorizeRBAC 함수 | REQ-AUTH-004 | 신규 |
| `apps/control-plane/internal/auth/context.go` | context.Context에 검증된 user 정보(uid, roles, scopes) 주입/추출 헬퍼 | REQ-AUTH-003 | 신규 |
| `apps/control-plane/internal/auth/errors.go` | 공통 인증 에러 정의 (ErrTokenExpired, ErrTokenInvalid, ErrTokenBlacklisted, ErrInsufficientScope) | REQ-AUTH-UBI-001 | 신규 |
| `apps/control-plane/internal/server/grpc_interceptor.go` | gRPC UnaryServerInterceptor + StreamServerInterceptor — 모든 RPC에 토큰 검증 | REQ-AUTH-003 | 신규 |
| `apps/control-plane/internal/server/rest_middleware.go` | REST `http.Handler` 미들웨어 — `/api/v1/*` 경로 토큰 검증, `/health` 면제 | REQ-AUTH-003 | 신규 |
| `apps/control-plane/internal/server/grpc_server.go` | CreateWorkflow / GetWorkflow / ListWorkflows 핸들러가 context에서 검증된 user_id 추출하여 TxCoordinator로 전달 | REQ-AUTH-003 | 수정 (소규모) |
| `apps/control-plane/internal/server/rest_handler.go` | Mux에 미들웨어 chain 적용, hardcoded `"cli-anonymous"` 제거하고 context user_id 사용 | REQ-AUTH-003 | 수정 (소규모) |
| `apps/control-plane/internal/audit/recorder.go` | `resolveUserID` 로직 확장: AuthEnabled=true + context user_id 존재 시 그것을 사용, 아니면 기존 cli-anonymous 폴백 | REQ-AUTH-UBI-001 | 수정 (소규모) |
| `apps/control-plane/internal/config/config.go` | OIDC 관련 필드 추가: OIDCIssuerURL, OIDCAudience, OIDCJWKSCacheTTL, RBACScopePrefix | REQ-AUTH-002 | 수정 (필드 추가) |
| `apps/control-plane/internal/scheduler/celery_envelope.go` | Celery envelope `headers.user_id` 필드 추가 — context에서 user_id 읽어 propagation (v0.1.1 D1 보정: envelope builder boundary는 dispatcher.go가 아닌 celery_envelope.go) | REQ-AUTH-UBI-001 | 수정 (소규모) |
| `apps/control-plane/cmd/server/main.go` | Auth 컴포넌트 초기화 + 미들웨어 chain wiring | 전체 | 수정 |

### 2.2 Python Pipelines (`pipelines/`)

| 경로 | 책임 | 모듈 | 신규/수정 |
|------|------|------|---------|
| `pipelines/auth/__init__.py` | 모듈 초기화 | 전체 | 신규 |
| `pipelines/auth/verifier.py` | JWT 서명 검증 (python-jose 또는 PyJWT), python-jose JWKS client 사용 | REQ-AUTH-001 | 신규 |
| `pipelines/auth/oidc_client.py` | OIDC Discovery 클라이언트 | REQ-AUTH-002 | 신규 |
| `pipelines/auth/dependencies.py` | FastAPI `Depends(verify_token)` dependency, `Depends(require_role("analyst"))` factory | REQ-AUTH-003, REQ-AUTH-004 | 신규 |
| `pipelines/auth/blacklist.py` | Redis 토큰 블랙리스트 (Go와 동일 키 스페이스 공유) | REQ-AUTH-001, REQ-AUTH-005 | 신규 |
| `pipelines/auth/context.py` | Celery task signal handler — envelope `headers.user_id`에서 user_id 추출하여 task-local context에 주입 | REQ-AUTH-UBI-001 | 신규 |
| `pipelines/auth/errors.py` | TokenExpiredError, TokenInvalidError, InsufficientScopeError | REQ-AUTH-UBI-001 | 신규 |
| `pipelines/config/settings.py` | OIDC 관련 필드 추가: OIDC_ISSUER_URL, OIDC_AUDIENCE, OIDC_JWKS_CACHE_TTL, RBAC_SCOPE_PREFIX | REQ-AUTH-002 | 수정 (필드 추가) |
| `pipelines/main.py` | FastAPI 앱에 글로벌 dependency `Depends(verify_token)` 등록, `/health` 면제 | REQ-AUTH-003 | 수정 |
| `pipelines/workers/ingestion_worker.py` | Celery task entrypoint — context.user_id를 audit_event INSERT 시 사용 | REQ-AUTH-UBI-001 | 수정 (소규모) |
| `pipelines/workers/generation_worker.py` | 동일 | REQ-AUTH-UBI-001 | 수정 (소규모) |
| `pipelines/workers/simulation_worker.py` | 동일 | REQ-AUTH-UBI-001 | 수정 (소규모) |

### 2.3 Shared (`pkg/`, `schemas/`)

| 경로 | 책임 | 신규/수정 |
|------|------|---------|
| `pkg/auth/claims.go` | Go-Python 공통 JWT 클레임 구조체 (sub, scope, roles, aud, exp, iat, nbf, iss) | 신규 |
| `pkg/auth/claims.py` | 동일 (Python Pydantic) | 신규 |
| `schemas/openapi/openapi.yaml` | `Authorization: Bearer <token>` 헤더 명세 추가, 401/403 에러 응답 정의 추가 | 수정 |
| `schemas/proto/auth.proto` | (선택) 향후 gRPC metadata 표준화를 위한 placeholder. 본 SPEC에서는 미생성 — gRPC는 metadata `authorization` 헤더 직접 사용 | 미생성 |

### 2.4 Deployments (`deployments/`)

| 경로 | 책임 | 신규/수정 |
|------|------|---------|
| `deployments/helm/iroum-ax/templates/keycloak-statefulset.yaml` | Keycloak 24.x LTS StatefulSet (PoC 단일 인스턴스, HA는 후속) | 신규 |
| `deployments/helm/iroum-ax/templates/keycloak-realm-configmap.yaml` | Keycloak realm 사전 정의 (clients, roles, scopes) | 신규 |
| `deployments/helm/iroum-ax/templates/secret.yaml` | OIDC client secret, JWKS endpoint URL 등 | 수정 |
| `deployments/helm/iroum-ax/values.yaml` | `auth.enabled`, `auth.oidc.issuer`, `auth.oidc.audience` 기본값 | 수정 |
| `deployments/helm/iroum-ax/values-dev.yaml` | dev 환경 `auth.enabled: false` (sandbox 호환) | 수정 |
| `docker-compose.yml` | Keycloak 컨테이너 추가 (dev 편의용) | 수정 |

### 2.5 Database (`.moai/db/`)

본 SPEC은 schema 변경 없음. `audit_logs.user_id` 컬럼은 SPEC-AX-001에서 이미 `VARCHAR(64)` 정의되어 있으며, JWT `sub` 클레임(UUID 또는 Keycloak user ID) 길이를 수용한다.

### 2.6 Tests

| 경로 | 책임 | 모듈 |
|------|------|------|
| `apps/control-plane/internal/auth/verifier_test.go` | 토큰 만료/서명 실패/aud 불일치/clock skew 테이블 테스트 | REQ-AUTH-001 |
| `apps/control-plane/internal/auth/jwks_cache_test.go` | JWKS fetch + TTL 캐시 + 키 회전 단위 테스트 | REQ-AUTH-001 |
| `apps/control-plane/internal/auth/oidc_client_test.go` | Discovery 응답 파싱 단위 테스트 (httptest 기반) | REQ-AUTH-002 |
| `apps/control-plane/internal/auth/blacklist_test.go` | Redis (miniredis) 기반 블랙리스트 단위 테스트 | REQ-AUTH-001 |
| `apps/control-plane/internal/auth/rbac_test.go` | 3-role 매트릭스 권한 매핑 테이블 테스트 | REQ-AUTH-004 |
| `apps/control-plane/internal/server/grpc_interceptor_test.go` | bufconn 기반 unary/stream interceptor 단위 테스트 | REQ-AUTH-003 |
| `apps/control-plane/internal/server/rest_middleware_test.go` | httptest 기반 미들웨어 단위 테스트 + `/health` bypass 검증 | REQ-AUTH-003 |
| `pipelines/auth/tests/test_verifier.py` | python-jose 기반 검증 + httpx_mock JWKS endpoint | REQ-AUTH-001 |
| `pipelines/auth/tests/test_dependencies.py` | FastAPI `TestClient` + dependency override | REQ-AUTH-003, REQ-AUTH-004 |
| `pipelines/auth/tests/test_context.py` | Celery task signal handler에서 user_id 추출 검증 | REQ-AUTH-UBI-001 |
| `tests/integration/test_auth_e2e.py` | docker-compose + Keycloak realm fixture, 실 토큰 발급 → REST → gRPC → Celery → audit_logs.user_id 전파 E2E | 전체 |
| `tests/integration/test_auth_backward_compat.py` | `AUTH_ENABLED=false`로 부팅 시 모든 경로가 기존대로 `cli-anonymous`로 동작 (regression 방지) | REQ-AUTH-UBI-001 |

---

## 3. EARS 요구사항

EARS 5개 패턴(Ubiquitous / Event-driven / State-driven / Optional / Unwanted) 모두 본 SPEC 내에 포함된다.

### 3.1 Ubiquitous Requirements (시스템 전반 불변 조건)

- **REQ-AUTH-UBI-001 (사용자 식별 일관성)**: The system SHALL propagate the verified `sub` claim from JWT to `audit_logs.user_id` for every audited event when authentication is enabled. WHILE 인증 시스템(AuthEnabled=false)이 비활성화 상태일 때, the system SHALL preserve the existing `cli-anonymous` fallback semantics defined in SPEC-AX-001 REQ-UBI-003 and SPEC-AX-CTRL-001 REQ-CTRL-UBI-002. 토큰 만료 / 서명 실패 / 재사용 시 즉시 거부하고, 거부 이벤트는 `audit_logs`에 `AUTH_REJECTED` 액션으로 기록되어야 한다(거부도 추적 가능).

### 3.2 REQ-AUTH-001 — JWT Token Validation

#### Event-driven

- **REQ-AUTH-001-E1**: WHEN a request arrives with `Authorization: Bearer <token>` header and AuthEnabled=true, THEN the system SHALL verify the token signature using RS256 or EdDSA public keys fetched from the configured JWKS endpoint, validate `exp` / `nbf` / `iat` time claims with 30-second clock skew tolerance, validate `aud` claim equals `iroum-ax-control-plane` (Go) or `iroum-ax-pipelines` (Python), validate `iss` claim equals the configured `OIDC_ISSUER_URL` (v0.1.1 SF-1 보정: per-token issuer 검증으로 cross-realm token 재사용 공격 차단, RFC 7519 §4.1.1 + OWASP JWT cheat sheet), and reject the request with HTTP 401 / gRPC `UNAUTHENTICATED` if any check fails.

- **REQ-AUTH-001-E2**: WHEN the JWKS cache TTL (default 3600 seconds) expires, THEN the system SHALL refresh the JWKS document from the OIDC provider's `/protocol/openid-connect/certs` endpoint within 5 seconds and continue accepting in-flight requests using the previous cached keys until refresh completes.

#### State-driven

- **REQ-AUTH-001-S1**: WHILE a token's `jti` claim exists in the Redis blacklist (key `auth:blacklist:<jti>`), THE system SHALL reject every authenticated request bearing that token regardless of expiration validity, and log `AUTH_REJECTED` with reason `blacklist_hit`.

#### Unwanted

- **REQ-AUTH-001-U1**: IF the JWT algorithm header (`alg`) is `HS256`, `none`, or any algorithm other than `RS256` / `EdDSA` / `ES256`, THEN the system SHALL reject the token immediately without signature verification and log `AUTH_REJECTED` with reason `algorithm_not_allowed`. (대칭키 알고리즘은 본 SPEC에서 명시적 거부 대상.)

- **REQ-AUTH-001-U2**: IF the token's `iat` claim is more than 30 seconds in the future (future-dated token), THEN the system SHALL reject the token with HTTP 401 and log `AUTH_REJECTED` with reason `future_iat`.

### 3.3 REQ-AUTH-002 — OIDC Provider Integration

#### Event-driven

- **REQ-AUTH-002-E1**: WHEN the control-plane / pipelines process starts and AuthEnabled=true, THEN the system SHALL fetch `OIDC_ISSUER_URL/.well-known/openid-configuration` once, extract `jwks_uri` / `issuer` / `token_endpoint` fields, persist them in an in-memory provider metadata cache, and SHALL fail-fast (panic during startup) if discovery returns non-200 within 10 seconds.

#### Optional

- **REQ-AUTH-002-O1**: WHERE `OIDC_DISCOVERY_REFRESH_INTERVAL` is set (default: unset = no refresh), THE system MAY periodically re-fetch the OIDC discovery document at that interval to handle provider URL changes. WHERE unset, the discovery document is fetched once at startup and not refreshed during the process lifetime (manual restart required).

### 3.4 REQ-AUTH-003 — Middleware Integration

#### Event-driven

- **REQ-AUTH-003-E1 (Go gRPC)**: WHEN a gRPC unary or stream RPC is invoked on the control-plane server and AuthEnabled=true, THEN the gRPC `UnaryServerInterceptor` / `StreamServerInterceptor` SHALL extract the `authorization` metadata key, verify the bearer token via REQ-AUTH-001-E1, inject the validated user object (`uid`, `roles`, `scopes`, `claims`) into `context.Context` via `auth.WithUser(ctx, user)`, and pass the augmented context to the downstream handler. WHERE the RPC method is `/grpc.health.v1.Health/Check`, the interceptor SHALL bypass authentication.

- **REQ-AUTH-003-E2 (Go REST)**: WHEN an HTTP request arrives at `/api/v1/*` and AuthEnabled=true, THEN the REST middleware SHALL extract the `Authorization: Bearer <token>` header, verify the token, inject the user object into `r.Context()`, and call `next.ServeHTTP(w, r.WithContext(ctx))`. WHERE the path is `/health`, the middleware SHALL bypass authentication (health checks are unauthenticated by REQ-CTRL-003-E1).

- **REQ-AUTH-003-E3 (Python FastAPI)**: WHEN a FastAPI request arrives at any endpoint registered with `Depends(verify_token)` and AuthEnabled=true, THEN the dependency SHALL extract `Authorization: Bearer <token>` from headers, verify the token, and return a `User` object that downstream handlers receive as an argument. `/health` and `/docs` SHALL be excluded from this dependency.

#### Unwanted

- **REQ-AUTH-003-U1**: IF the `Authorization` header is missing or malformed (not starting with `Bearer ` prefix) and AuthEnabled=true, THEN the system SHALL reject the request with HTTP 401 / gRPC `UNAUTHENTICATED` and the error body SHALL include `WWW-Authenticate: Bearer realm="iroum-ax", error="invalid_request"`. SHALL NOT silently fall back to `cli-anonymous` when AuthEnabled=true (fail-secure).

### 3.5 REQ-AUTH-004 — RBAC Foundation (3 Roles)

#### State-driven

- **REQ-AUTH-004-S1**: WHILE a JWT contains the scope claim with value matching the regex `^iroum-ax:(admin|analyst|viewer)$`, THE system SHALL grant the corresponding role permissions per the matrix defined below. A token may contain multiple roles separated by space (`scope: "iroum-ax:analyst iroum-ax:viewer"`); the effective permission set is the union of all granted roles' permissions.

**Role-Permission Matrix** (canonical, source-of-truth):

| Role | gRPC Methods | REST Methods | Description |
|------|-------------|--------------|-------------|
| `admin` | all WorkflowService methods + future AdminService | all `/api/v1/*` | 모든 권한 (시스템 관리자) |
| `analyst` | CreateWorkflow, GetWorkflow, ListWorkflows | POST/GET/LIST `/api/v1/workflows`, POST `/api/v1/recommendations/{id}/feedback`, POST `/api/v1/documents/upload` | 워크플로우 CRUD + recommendation 피드백 |
| `viewer` | GetWorkflow, ListWorkflows | GET only | 읽기 전용 |

#### Unwanted

- **REQ-AUTH-004-U1**: IF the verified token's effective permission set does NOT include the required permission for the invoked method, THEN the system SHALL reject with HTTP 403 Forbidden / gRPC `PERMISSION_DENIED`, log `AUTH_FORBIDDEN` to audit_logs with the attempted `method` + `path` + `user_id` + `granted_roles`, and SHALL NOT execute the underlying handler. (`PERMISSION_DENIED`는 인증은 통과했으나 권한이 부족한 경우만 사용; 인증 실패는 `UNAUTHENTICATED`.)

### 3.6 REQ-AUTH-005 — Token Refresh + Logout

#### Event-driven

- **REQ-AUTH-005-E1 (Logout)**: WHEN a client invokes `POST /api/v1/auth/logout` with both `Authorization: Bearer <access_token>` header and JSON body `{refresh_token: "<refresh_token>"}`, THEN the system SHALL extract both tokens' `jti` claims, persist each into Redis blacklist with TTL = `token.exp - now()` (so blacklist entry expires when token would naturally expire), record `AUTH_LOGOUT` in audit_logs, and return HTTP 204 No Content. If either token is malformed, return HTTP 400 (do not partial-blacklist).

#### Unwanted

- **REQ-AUTH-005-U1 (Refresh Token Reuse Detection)**: IF a refresh token is presented for token refresh and that token's `jti` is already in the blacklist OR has previously been used to issue a new access token (refresh token family tracking via Redis key `auth:refresh_family:<family_id>`), THEN the system SHALL invalidate the entire refresh token family by blacklisting all `jti` values in that family, log `AUTH_REFRESH_REUSE_DETECTED` with `family_id`, and reject with HTTP 401. (재사용 감지는 OAuth 2.0 BCP 권고 패턴.)

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 성능 — 토큰 검증 | JWKS 캐시 hit 시 p99 < 5ms (CPU 시간) | OAuth 2.0 BCP industry baseline |
| 성능 — JWKS fetch | JWKS endpoint fetch p99 < 200ms (Keycloak 동일 K8s namespace 기준) | 측정 baseline |
| 성능 — Logout | logout endpoint p99 < 50ms (Redis SET 1회) | Redis 호환 |
| 성능 — RBAC 검사 | scope 파싱 + 매트릭스 lookup p99 < 1ms | string ops only |
| 가용성 — JWKS unavailable | JWKS endpoint 미가용 시 cached keys 사용; cache 미존재 시 HTTP 503 + `Retry-After: 30` 헤더 | REQ-AUTH-001-E2 |
| 보안 — 알고리즘 | RS256 / EdDSA / ES256만 허용, HS256 / none 명시 거부 | OWASP JWT |
| 보안 — Algorithm Confusion Attack | `alg` 헤더 검증을 JWKS 응답의 `alg` 필드와 매칭 (allow-list) | OWASP JWT |
| 보안 — Token Leakage | access/refresh token을 zap/structured log에 출력 금지 (redaction middleware) | OWASP A09 |
| 보안 — Clock Skew | ±30초 허용 (NTP 미동기화 환경 대비) | OAuth 2.0 BCP |
| 망분리 정합 | Keycloak은 동일 K8s namespace 내, 외부 인터넷 fetch 0건 | `tech.md` §9.1 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR) | `quality.yaml` development_mode |
| 백워드 호환성 | AuthEnabled=false 시 SPEC-AX-001 / SPEC-AX-CTRL-001 모든 AC가 unchanged 통과 | regression invariant |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC에서 다룬다.

1. **외부 OAuth providers (Google / Microsoft / GitHub / Apple)** — 망분리 정합 위반. 후속 SPEC에서 외부 IdP federation 별도 검토.
2. **다단계 인증 (MFA / TOTP / Hardware Token / WebAuthn)** — Keycloak 단에서 옵션으로 활성 가능하나 본 SPEC 범위 외. 후속 SPEC `SPEC-AX-AUTH-MFA-001`.
3. **비밀번호 정책 (해시 / 저장 / 만료 / 복잡도)** — Keycloak 책임. 본 SPEC은 OIDC 클라이언트만 구현.
4. **SAML 2.0 / WS-Federation** — OIDC만 지원. SAML은 후속 SPEC 검토 (전자정부 표준 인증 통합 시 필요할 수 있음).
5. **전자정부 표준 인증 (e-Gov SSO) 통합** — 공공기관 표준이지만 별도 통합 SDK 및 인증서 필요. 후속 SPEC `SPEC-AX-AUTH-EGOV-001`로 분리.
6. **권한 위임 (Impersonation / Service Account Assumption)** — admin이 다른 사용자로 가장 실행. 후속 SPEC.
7. **세션 관리 콘솔 UI** — Keycloak Admin Console에 위임. iroum-ax Console UI는 SPEC-AX-CONSOLE-001 후속.
8. **감사 보고서 자동 생성** — `audit_logs` 쿼리 API만 노출 가능. 보고서 PDF 생성 / email 발송은 후속.
9. **토큰 암호화 (JWE / Encrypted JWT)** — 서명(JWS)만 사용. JWE는 토큰 payload 기밀성 요구 시 후속.
10. **권한 캐싱 최적화** — 운영 후 측정 기반으로 후속 결정. 본 SPEC은 매 요청마다 scope 파싱 (스코프 파싱이 < 1ms이므로 OK).
11. **ABAC (Attribute-Based Access Control)** — 본 SPEC은 role-based만. 자원별 ownership (예: "본인이 만든 workflow만 조회") 같은 ABAC 규칙은 후속.
12. **Workflow / Document level 권한** — 본 SPEC RBAC은 method/path 단위만. row-level security (RLS)는 멀티테넌트 SPEC과 함께 후속.
13. **gRPC mTLS** — 본 SPEC은 JWT bearer만. mTLS는 service-to-service 통신 보안 후속 SPEC.
14. **OIDC Logout Endpoint (RP-Initiated Logout / End-Session)** — 본 SPEC은 client-side 블랙리스트만. Keycloak end-session endpoint 호출은 후속.
15. **JWT Token Revocation Endpoint (RFC 7009)** — 본 SPEC은 logout endpoint를 통한 블랙리스트만. RFC 7009 standard revocation은 후속.

---

## 6. 의존성 및 전제

- **SPEC-AX-001 PASSED 가정**: REQ-UBI-003 audit_logs schema + cli-anonymous 폴백이 GREEN 상태.
- **SPEC-AX-CTRL-001 PASSED 가정**: Go control-plane gRPC + REST + TxCoordinator + Celery dispatch가 GREEN 상태. `audit.Recorder.resolveUserID` 로직이 본 SPEC에서 확장된다.
- **Keycloak 24.x LTS 설치 가능**: docker-compose dev / Helm chart prod 양쪽에서. 망분리 환경에서 사전 다운로드 후 배포.
- **Redis 가용**: 블랙리스트 + refresh token family tracking이 Redis 의존. SPEC-AX-CTRL-001 Redis 인스턴스 공유.
- **PostgreSQL `audit_logs.user_id VARCHAR(64)`**: Keycloak user ID UUID (36 chars) 충분히 수용.
- **Go 의존성**: `github.com/golang-jwt/jwt/v5 v5.2+`, `github.com/coreos/go-oidc/v3 v3.10+`, `github.com/redis/go-redis/v9` (기존).
- **Python 의존성**: `python-jose[cryptography] ^3.3.0` 또는 `PyJWT ^2.8.0` + `httpx ^0.27.0` (JWKS fetch), `fastapi-security` 미사용 (자체 dependency 사용).
- **Helm dependency**: Keycloak StatefulSet + Postgres backing store (Keycloak own DB, iroum-ax PostgreSQL과 별도 DB 권장).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- Keycloak realm 설정 자동화: 본 SPEC은 ConfigMap에 정적 realm.json 제공. 동적 realm provisioning은 후속.
- 토큰 발급 자체: 본 SPEC은 검증만 담당. 토큰 발급은 Keycloak Authorization Code Flow (PKCE). Console UI에서 처리 — SPEC-AX-CONSOLE-001 범위.
- CLI 클라이언트 인증: 본 SPEC은 서버 측만. CLI가 device authorization grant로 토큰 획득하는 흐름은 후속.
- 권한 변경 감사: 사용자 role 변경 시 audit 기록은 Keycloak Admin Events 활용 — iroum-ax 측 audit_logs 통합은 후속.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: Go `apps/control-plane/internal/auth/*_test.go` (verifier, jwks_cache, oidc_client, blacklist, rbac), Python `pipelines/auth/tests/test_*.py`
- 통합 테스트: bufconn gRPC interceptor + httptest REST middleware
- E2E 통합 테스트: `tests/integration/test_auth_e2e.py` — docker-compose + Keycloak realm fixture, 실 토큰 발급 → REST → gRPC → Celery → audit_logs.user_id 전파
- 백워드 호환성 테스트: `tests/integration/test_auth_backward_compat.py` — `AUTH_ENABLED=false`로 부팅 시 SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C가 unchanged 통과
- 성능 측정: `go test -bench=BenchmarkTokenVerifyCacheHit` + custom benchstat
- 보안 검증: OWASP JWT cheat sheet 항목별 체크 (algorithm confusion, none alg, weak secret, missing claims)

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
