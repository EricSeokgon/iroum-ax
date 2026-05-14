# SPEC-AX-AUTH-001 Acceptance Criteria

> Format: Given / When / Then
> Methodology: TDD — 각 AC가 RED phase 단위 테스트로 1:1 매핑
> Tooling (Go): testify/assert + miniredis + bufconn + httptest + goleak
> Tooling (Python): pytest + pytest-asyncio + httpx_mock + FastAPI TestClient

본 문서는 SPEC-AX-AUTH-001의 6개 REQ(1 Ubiquitous + 5 modal)에 대한 acceptance criteria를 정의한다. 모호한 표현("적절히", "신속하게") 사용 금지.

---

## §0. REQ-AUTH-UBI Transverse Invariant — Acceptance

본 섹션은 §1-§5 modal REQ를 가로지르는 UBI 불변 조건과 SPEC-AX-001 / SPEC-AX-CTRL-001 백워드 호환성을 검증한다.

### AC-AUTH-UBI-001-A (sub Claim → audit_logs.user_id Propagation)

REQ 대응: REQ-AUTH-UBI-001.

**Given**:
- AuthEnabled=true, Keycloak realm fixture로 user `alice` (sub=`uuid-alice`, scope=`iroum-ax:analyst`)에 대한 valid access token 발급
- PostgreSQL `workflows` + `audit_logs` 테이블 clean state

**When**:
- 클라이언트가 `POST /api/v1/workflows` with `Authorization: Bearer <token>`, body `{"document_id":"d-uuid-ubi-001"}`

**Then**:
- HTTP 201 Created
- `workflows.user_id` = `'uuid-alice'` (byte-identical with token sub claim)
- `audit_logs` 테이블에 action=`WORKFLOW_CREATED` row 1개, `user_id=='uuid-alice'`
- 동일 transaction 내에서 commit (AC-CTRL-UBI-001 atomicity invariant 유지)

### AC-AUTH-UBI-001-B (Authentication Rejection Audit)

REQ 대응: REQ-AUTH-UBI-001 (거부 이벤트 audit_logs `AUTH_REJECTED`).

**Given**:
- AuthEnabled=true
- Expired access token (`exp = now - 100s`, clock skew 30초 초과)

**When**:
- 클라이언트가 `POST /api/v1/workflows` with expired token

**Then**:
- HTTP 401 Unauthorized
- 응답 헤더 `WWW-Authenticate: Bearer realm="iroum-ax", error="invalid_token", error_description="token expired"`
- `audit_logs`에 action=`AUTH_REJECTED` row 1개, `user_id` = token sub (검증 실패 전 추출), `details.reason = 'token_expired'`
- `workflows` 테이블 변화 없음 (인증 실패 시 비즈니스 로직 미실행)

### AC-AUTH-UBI-001-C (Backward Compatibility — AuthEnabled=false)

REQ 대응: REQ-AUTH-UBI-001 (WHILE 인증 비활성 시 cli-anonymous 폴백 유지).

**Given**:
- AuthEnabled=false (sandbox 환경, 기존 Walking Skeleton 기본값)
- 클라이언트가 `Authorization` 헤더 없이 요청

**When**:
- 클라이언트가 `POST /api/v1/workflows` with body `{"document_id":"d-uuid-ubi-001c"}` (no auth header)

**Then**:
- HTTP 201 Created (인증 우회)
- `workflows.user_id` = `'cli-anonymous'` (SPEC-AX-001 REQ-UBI-003 + SPEC-AX-CTRL-001 AC-CTRL-UBI-002-C와 byte-identical)
- `audit_logs.user_id` = `'cli-anonymous'`
- AC-CTRL-UBI-002-A (`WORKFLOW_CREATED` 1 row) + AC-CTRL-UBI-002-B (state transition audit) 결과 unchanged
- 이는 R-AUTH-007 (백워드 호환성) regression 가드의 핵심

### AC-AUTH-UBI-001-D (gRPC Path — sub Claim Propagation)

REQ 대응: REQ-AUTH-UBI-001 (gRPC 경로에서도 균일 적용).

**Given**:
- AuthEnabled=true
- Valid token for user `bob` (sub=`uuid-bob`, scope=`iroum-ax:analyst`)
- gRPC metadata에 `authorization: Bearer <token>` 설정

**When**:
- 클라이언트가 `WorkflowService.CreateWorkflow{document_id: "d-uuid-ubi-001d"}` 호출

**Then**:
- gRPC 응답 정상 (CreateWorkflowResponse)
- `workflows.user_id` = `'uuid-bob'`
- `audit_logs.user_id` = `'uuid-bob'`
- gRPC + REST 두 경로 모두 동일 invariant 충족 (cross-protocol consistency)

---

## §1. REQ-AUTH-001 JWT Token Validation — Acceptance

### AC-AUTH-001-1 (Happy Path — Valid Token Accepted)

**Given**:
- AuthEnabled=true, JWKS 캐시 ready
- Valid token: alg=RS256, exp=now+3600s, nbf=now-60s, iat=now-10s, aud="iroum-ax-control-plane", iss=`OIDC_ISSUER_URL`

**When**:
- `VerifyToken(ctx, tokenString)` 호출

**Then**:
- 에러 없음
- 반환 `*Claims`의 `sub`, `scope`, `exp` 필드 정확히 추출
- JWKS cache hit 메트릭 증가

### AC-AUTH-001-2 (Expired Token Rejection)

**Given**: alg=RS256, exp=now-100s (30초 skew 초과)
**When**: VerifyToken 호출
**Then**:
- 에러 `ErrTokenExpired` 반환
- `audit_logs` action=`AUTH_REJECTED`, `details.reason='token_expired'`

### AC-AUTH-001-3 (Future iat Rejection)

**Given**: iat=now+100s (30초 skew 초과)
**When**: VerifyToken 호출
**Then**:
- 에러 `ErrTokenInvalid` 반환, message `"future iat"`
- audit reason=`future_iat`

### AC-AUTH-001-4 (Wrong Audience Rejection)

**Given**: aud="some-other-service" (configured audience와 불일치)
**When**: VerifyToken 호출
**Then**:
- 에러 `ErrTokenInvalid` 반환, message contains `"audience"`
- audit reason=`audience_mismatch`

### AC-AUTH-001-5 (Algorithm Allow-list — HS256 Rejection)

REQ 대응: REQ-AUTH-001-U1.

**Given**: alg=HS256 (대칭키, allow-list 위반)
**When**: VerifyToken 호출
**Then**:
- 에러 `ErrTokenInvalid` 반환 (signature verification 시도조차 안 함)
- audit reason=`algorithm_not_allowed`
- **Critical security AC** — Algorithm Confusion Attack 방지

### AC-AUTH-001-6 (Algorithm Allow-list — none Rejection)

**Given**: alg=none, signature 없음
**When**: VerifyToken 호출
**Then**:
- 에러 `ErrTokenInvalid` 반환
- audit reason=`algorithm_not_allowed`

### AC-AUTH-001-7 (Clock Skew Within 30s — Accepted)

**Given**: iat=now+25s (30초 skew 이내)
**When**: VerifyToken 호출
**Then**:
- 에러 없음, 토큰 수용

### AC-AUTH-001-8 (JWKS Cache TTL Refresh)

REQ 대응: REQ-AUTH-001-E2.

**Given**:
- JWKS 캐시 TTL 1초 (테스트용 단축 설정)
- 토큰 알고리즘 RS256, kid="key-v1"
- T+0: VerifyToken 성공 (cache populate)
- T+2: JWKS endpoint가 kid="key-v2"로 회전된 응답 반환

**When**:
- T+2 시점에 kid="key-v2" 토큰으로 VerifyToken 호출

**Then**:
- 캐시 미스 → JWKS 재fetch → cache repopulate → 검증 성공
- fetch latency p99 < 5s (REQ-AUTH-001-E2 invariant)

### AC-AUTH-001-9 (JWKS Unavailable — Stale-While-Revalidate)

**Given**:
- JWKS 캐시 populated (T+0)
- Keycloak Pod down (T+2)
- 토큰은 cache에 있는 kid 사용

**When**:
- T+2 시점에 VerifyToken 호출

**Then**:
- cached keys로 검증 성공 (degraded mode)
- 단, cache가 처음부터 비어 있을 때 JWKS 다운 시: HTTP 503 + `Retry-After: 30` 응답 + audit reason=`jwks_unavailable`

### AC-AUTH-001-10 (Blacklisted Token Rejection)

REQ 대응: REQ-AUTH-001-S1.

**Given**:
- Valid token with jti=`jti-blacklist-1`
- Redis SET `auth:blacklist:jti-blacklist-1` with EXPIREAT = token.exp

**When**: VerifyToken 호출
**Then**:
- 에러 `ErrTokenBlacklisted` 반환
- audit reason=`blacklist_hit`

### AC-AUTH-001-Performance (Token Verify p99 < 5ms)

**Given**: JWKS cache hit, 1000 iterations
**When**: VerifyToken 호출 (in-process, no I/O)
**Then**:
- p99 < 5ms (`go test -bench`)
- p50 < 1ms

---

## §2. REQ-AUTH-002 OIDC Provider Integration — Acceptance

### AC-AUTH-002-1 (Discovery Document Parsing)

**Given**:
- httptest mock server가 `/.well-known/openid-configuration`에 standard OIDC discovery JSON 반환
- `OIDC_ISSUER_URL=http://localhost:8080/realms/iroum-ax`

**When**:
- `oidc.NewClient(ctx, issuerURL)` 호출

**Then**:
- 반환 `*Client`의 `jwks_uri`, `issuer`, `token_endpoint` 필드 정확히 파싱
- 메모리 캐시에 저장

### AC-AUTH-002-2 (Discovery Failure — Fail-Fast Startup)

REQ 대응: REQ-AUTH-002-E1 (panic during startup).

**Given**:
- `OIDC_ISSUER_URL` 미응답 (timeout 10s)

**When**:
- `oidc.NewClient(ctx, issuerURL)` 호출

**Then**:
- 10초 이내 panic
- panic message에 issuer URL + timeout 정보 포함
- main.go가 이 panic을 받아 `log.Fatal` (운영 환경 fail-fast invariant)

### AC-AUTH-002-3 (Discovery — issuer Mismatch Rejection)

**Given**:
- Discovery 응답 `issuer` 필드 = `"http://other-issuer/realms/x"` (요청 URL과 불일치)

**When**: NewClient 호출

**Then**:
- 에러 반환 (security check)
- 메시지: `"discovery issuer mismatch"`

---

## §3. REQ-AUTH-003 Middleware Integration — Acceptance

### AC-AUTH-003-1 (gRPC Unary Interceptor — Token Extracted + ctx Injected)

REQ 대응: REQ-AUTH-003-E1.

**Given**:
- bufconn gRPC server with auth interceptor registered
- Valid token in metadata `authorization: Bearer <token>`

**When**:
- 클라이언트가 `WorkflowService.GetWorkflow{id: "wf-uuid-001"}` 호출

**Then**:
- 핸들러 진입 시 `auth.UserFromContext(ctx)`가 valid `*User`를 반환
- `User.UID` = token sub
- `User.Scopes` 정확히 추출

### AC-AUTH-003-2 (gRPC Health Check Bypass)

**Given**:
- gRPC server with auth interceptor
- `Authorization` metadata 없음

**When**:
- 클라이언트가 `/grpc.health.v1.Health/Check` 호출

**Then**:
- 정상 응답 (인증 우회)
- 다른 메서드 호출 시는 `UNAUTHENTICATED` 반환 확인 (대조)

### AC-AUTH-003-3 (REST Middleware — `/health` Bypass + `/api/v1/*` Enforce)

REQ 대응: REQ-AUTH-003-E2.

**Given**: AuthEnabled=true, REST server with auth middleware

**When (case A)**: `GET /health` (no auth header)
**Then (case A)**: HTTP 200 (bypass)

**When (case B)**: `POST /api/v1/workflows` (no auth header)
**Then (case B)**:
- HTTP 401 Unauthorized
- `WWW-Authenticate: Bearer realm="iroum-ax", error="invalid_request"` 헤더
- 응답 body `{"error":{"code":"UNAUTHENTICATED","message":"authorization header missing"}}`
- audit reason=`missing_authorization_header`

### AC-AUTH-003-4 (REST Middleware — Malformed Bearer Prefix)

REQ 대응: REQ-AUTH-003-U1.

**Given**: AuthEnabled=true
**When**: `POST /api/v1/workflows` with `Authorization: Token <token>` (Bearer 접두사 누락)
**Then**:
- HTTP 401
- error=`invalid_request`
- audit reason=`malformed_authorization_header`

### AC-AUTH-003-5 (FastAPI Dependency — verify_token)

REQ 대응: REQ-AUTH-003-E3.

**Given**:
- FastAPI app with `Depends(verify_token)` registered globally except `/health` and `/docs`
- Valid token

**When**:
- 클라이언트가 `POST /api/v1/documents/upload` with Bearer token

**Then**:
- 핸들러가 `user: User` 인자를 정상 수신
- `user.uid` = token sub

**When (case B)**: 동일 endpoint with no auth header
**Then (case B)**: HTTP 401

### AC-AUTH-003-6 (Audit recorder.resolveUserID Backward Compat)

REQ 대응: REQ-AUTH-003 + REQ-AUTH-UBI-001 (resolveUserID 확장이 기존 동작 보존).

**Given**: Recorder 인스턴스, 3가지 시나리오

**Scenario A** (authEnabled=true, ctx user_id="alice"):
- resolveUserID(ctx, "") → "alice" (context user_id 우선)

**Scenario B** (authEnabled=false, ctx에 user 없음):
- resolveUserID(ctx, "") → "cli-anonymous" (기존 동작 보존)

**Scenario C** (authEnabled=true, ctx user_id 없음 — 인증 누락 시점):
- resolveUserID(ctx, "") → 에러 또는 빈 문자열 (호출자가 handle)
- Note: 정상 흐름에서는 미들웨어가 ctx user_id를 주입했어야 함 → 이 시나리오는 미들웨어 우회 버그 detection

---

## §4. REQ-AUTH-004 RBAC Foundation — Acceptance

### AC-AUTH-004-1 (analyst Role — CreateWorkflow Allowed)

REQ 대응: REQ-AUTH-004-S1.

**Given**:
- Valid token, scope=`iroum-ax:analyst`
- AuthEnabled=true

**When**: `POST /api/v1/workflows` 호출

**Then**:
- HTTP 201 Created (analyst는 workflow CRUD 허용)
- audit_logs에 `WORKFLOW_CREATED` row, user_id=token sub

### AC-AUTH-004-2 (viewer Role — POST Denied, GET Allowed)

**Given**: scope=`iroum-ax:viewer`

**When (case A)**: `POST /api/v1/workflows`
**Then (case A)**:
- HTTP 403 Forbidden
- 응답 body `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope"}}`
- audit action=`AUTH_FORBIDDEN`, details=`{"method":"POST","path":"/api/v1/workflows","required":"analyst|admin","granted":"viewer"}`

**When (case B)**: `GET /api/v1/workflows/wf-uuid-001`
**Then (case B)**: HTTP 200 (viewer는 GET 허용)

### AC-AUTH-004-3 (admin Role — All Methods Allowed)

**Given**: scope=`iroum-ax:admin`
**When**: 임의 method/path 호출
**Then**: 항상 통과 (RBAC 거부 0건)

### AC-AUTH-004-4 (Multiple Roles — Union Permission Set)

**Given**: scope=`iroum-ax:analyst iroum-ax:viewer` (공백 구분 두 역할)
**When**: `POST /api/v1/workflows`
**Then**:
- HTTP 201 (analyst 권한으로 통과)
- effective permission set = union(analyst, viewer)

### AC-AUTH-004-5 (Unknown Scope — Treated as No Permission)

**Given**: scope=`iroum-ax:hacker iroum-ax:superuser` (allow-list `[admin, analyst, viewer]` 외)
**When**: `POST /api/v1/workflows`
**Then**:
- HTTP 403
- audit reason=`no_recognized_role`
- granted_roles=`[]` (unknown scopes are silently dropped, not error)

### AC-AUTH-004-6 (gRPC Path RBAC Enforcement)

**Given**: scope=`iroum-ax:viewer`
**When**: gRPC `WorkflowService.CreateWorkflow` 호출
**Then**:
- gRPC code `PERMISSION_DENIED`
- audit `AUTH_FORBIDDEN` with method=`/proto.WorkflowService/CreateWorkflow`

---

## §5. REQ-AUTH-005 Token Refresh + Logout — Acceptance

### AC-AUTH-005-1 (Logout — Both Tokens Blacklisted)

REQ 대응: REQ-AUTH-005-E1.

**Given**:
- AuthEnabled=true
- Valid access_token (jti=`at-1`, exp=now+3600s)
- Valid refresh_token (jti=`rt-1`, exp=now+86400s)

**When**:
- 클라이언트가 `POST /api/v1/auth/logout` with `Authorization: Bearer <access_token>` + body `{"refresh_token":"<refresh_token>"}`

**Then**:
- HTTP 204 No Content
- Redis SET `auth:blacklist:at-1` exists with TTL ≈ 3600s
- Redis SET `auth:blacklist:rt-1` exists with TTL ≈ 86400s
- audit_logs row action=`AUTH_LOGOUT`, user_id=access_token sub

### AC-AUTH-005-2 (Logout — Subsequent Use Rejected)

**Given**: AC-AUTH-005-1 후속 상태 (token 블랙리스트 진입)

**When**: 클라이언트가 access_token으로 `POST /api/v1/workflows`

**Then**:
- HTTP 401
- audit reason=`blacklist_hit`

### AC-AUTH-005-3 (Refresh Token Reuse — Family Invalidation)

REQ 대응: REQ-AUTH-005-U1.

**Given**:
- Refresh token family_id=`fam-1`
- T+0: family에 첫 발급 jti=`rt-original`
- T+1: rt-original을 정상 사용 → 새 토큰 jti=`rt-new` 발급, family에 추가
- T+2: 공격자가 rt-original을 다시 제시 (재사용 시도)

**When**: T+2 시점에 `POST /api/v1/auth/refresh` with rt-original

**Then**:
- HTTP 401
- Redis `auth:blacklist:rt-original` + `auth:blacklist:rt-new` 모두 진입 (family 전체 invalidation)
- audit_logs row action=`AUTH_REFRESH_REUSE_DETECTED`, details=`{"family_id":"fam-1","reused_jti":"rt-original"}`

### AC-AUTH-005-4 (Logout — Malformed refresh_token)

**Given**: Valid access_token but body refresh_token is malformed JWT
**When**: POST logout
**Then**:
- HTTP 400 Bad Request (do not partial-blacklist)
- access_token NOT yet blacklisted (transaction atomicity)

---

## §6. E2E Integration — Acceptance

### AC-AUTH-E2E-1 (Full Authenticated Workflow Lifecycle)

**Given**:
- docker-compose up: PostgreSQL + Redis + Keycloak + Python Celery worker + Go control-plane
- Keycloak realm `iroum-ax` fixture: user `alice` (scope=`iroum-ax:analyst`), client `iroum-ax-cli` (Authorization Code Flow + PKCE)
- `AUTH_ENABLED=true`

**When**:
- 테스트가 Keycloak token endpoint로 토큰 발급 (resource owner password grant — test only)
- `POST /api/v1/workflows` with Bearer token, body `{"document_id":"<fixture uuid>"}`
- 5초 polling으로 workflow 완료 대기

**Then**:
- HTTP 201 + workflow_id 반환
- `workflows.user_id` = `'alice-sub-uuid'`
- audit_logs 일련의 row: `WORKFLOW_CREATED` (user_id=alice) → `WORKFLOW_TRANSITIONED_TO_RUNNING` (user_id=alice) → `WORKFLOW_COMPLETED` (user_id=alice) — 3개 모두 동일 user_id
- Python worker side audit_logs도 user_id=alice 기록 (envelope `headers.user_id` propagation 검증)

### AC-AUTH-E2E-2 (Backward Compat — AuthEnabled=false E2E)

**Given**:
- docker-compose up (Keycloak 포함되지만 사용 안 함)
- `AUTH_ENABLED=false`

**When**:
- 클라이언트가 `Authorization` 헤더 없이 `POST /api/v1/workflows`

**Then**:
- HTTP 201
- 모든 audit_logs row의 user_id=`cli-anonymous`
- SPEC-AX-CTRL-001 AC-CTRL-E2E-1 결과와 byte-identical

---

## §7. Performance Acceptance Summary

| Metric | Target | Measurement Method | Reference |
|--------|--------|--------------------|-----------|
| Token verify p99 (JWKS cache hit) | < 5ms | `go test -bench=BenchmarkTokenVerify` | AC-AUTH-001-Performance |
| JWKS fetch p99 (Keycloak local) | < 200ms | benchmark + httptest | REQ-AUTH-001-E2 |
| RBAC scope parse + lookup p99 | < 1ms | benchmark | REQ-AUTH-004 NFR |
| Logout endpoint p99 | < 50ms | benchmark + miniredis | REQ-AUTH-005-E1 NFR |
| OIDC Discovery startup | < 10s (fail-fast) | timing assertion | AC-AUTH-002-2 |
| Backward compat regression | 0 AC failure in SPEC-AX-CTRL-001 with AUTH_ENABLED=false | test_auth_backward_compat.py | AC-AUTH-UBI-001-C |

---

## §8. Edge Case Catalog

| Edge Case | 대응 AC | Risk ID |
|-----------|--------|---------|
| Expired token (clock skew 초과) | AC-AUTH-001-2 | R-AUTH-003 |
| Future iat (clock 미동기 공격) | AC-AUTH-001-3 | R-AUTH-003 |
| Wrong audience (cross-service token misuse) | AC-AUTH-001-4 | - |
| HS256 algorithm confusion attack | AC-AUTH-001-5 | R-AUTH-006 |
| `alg: none` attack | AC-AUTH-001-6 | R-AUTH-006 |
| JWKS endpoint unavailable (cache miss) | AC-AUTH-001-9 | R-AUTH-001 |
| Blacklisted token reuse | AC-AUTH-001-10 | - |
| Discovery 미응답 (startup) | AC-AUTH-002-2 | R-AUTH-001 |
| Issuer mismatch in discovery | AC-AUTH-002-3 | - |
| Missing Authorization header | AC-AUTH-003-3 (case B) | - |
| Malformed Bearer prefix | AC-AUTH-003-4 | - |
| RBAC scope insufficient | AC-AUTH-004-2 | - |
| Unknown scope (forged token) | AC-AUTH-004-5 | R-AUTH-006 |
| Refresh token reuse (family attack) | AC-AUTH-005-3 | R-AUTH-004 |
| Logout with malformed refresh | AC-AUTH-005-4 | - |
| Backward compat regression | AC-AUTH-UBI-001-C, AC-AUTH-E2E-2 | R-AUTH-007 |
| Token leakage in logs | (lint rule + AC-AUTH-003-* 로그 검사) | R-AUTH-002 |
| Keycloak SPOF (single instance) | (수용 risk, runbook 후속) | R-AUTH-005 |

---

## §9. Definition of Done (Acceptance Phase)

본 SPEC의 acceptance가 완료되었다고 선언하기 위해 모두 PASS 필요:

- [ ] §0: REQ-AUTH-UBI-001 전용 AC 4개 (UBI-001-A/B/C/D) 자동화 테스트 통과
- [ ] §1: REQ-AUTH-001 AC 10개 + performance benchmark 1개
- [ ] §2: REQ-AUTH-002 AC 3개
- [ ] §3: REQ-AUTH-003 AC 6개
- [ ] §4: REQ-AUTH-004 AC 6개
- [ ] §5: REQ-AUTH-005 AC 4개
- [ ] §6: E2E AC 2개 (인증 활성 + 백워드 호환)
- [ ] §7: 6개 성능 지표 모두 target 충족 (CI에서는 1.5× 완화 허용)
- [ ] §8: 17개 edge case 모두 대응 AC로 검증됨
- [ ] coverage ≥ 85% (`go test -cover` + `pytest --cov`)
- [ ] golangci-lint default + gosec 0 issue (특히 G401/G402 crypto warnings)
- [ ] ruff check 0 error
- [ ] `goleak.VerifyNone(t)` 모든 Go 테스트에서 통과
- [ ] MX 태그 plan.md §6 매핑 완료 (S0 stub의 TODO 모두 해소)
- [ ] manager-quality TRUST 5 통과 (특히 Secured: OWASP JWT cheat sheet 항목별 검증)
- [ ] evaluator-active per-sprint scoring: S1/S4/S5 ≥ 0.80 (security-critical), 나머지 ≥ 0.75
- [ ] Token leakage 검사: zap log + Python logging에 `Bearer ` 접두 문자열이 검출되지 않음 (R-AUTH-002 mitigation)

**Total AC count**: 36 (UBI: 4, §1: 10+1, §2: 3, §3: 6, §4: 6, §5: 4, §6 E2E: 2)
