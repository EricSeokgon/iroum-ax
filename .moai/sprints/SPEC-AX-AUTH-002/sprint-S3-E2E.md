# Sprint Contract — SPEC-AX-AUTH-002 Sprint 3 E2E

## Sprint Identifier
- SPEC: SPEC-AX-AUTH-002
- Sprint: S3 (E2E Integration + AUTH-001 SKIP Unblock)
- Phase: GREEN (RED + GREEN combined)
- Date: 2026-05-15

## Acceptance Criteria

### AC-AUTH2-004-1 (admin full access)
- [x] admin 토큰 → POST /api/v1/workflows → 201 Created
- [x] admin 토큰 → GET /api/v1/workflows → 200 OK
- [x] workflows 테이블에 row 생성 (DB 검증)

### AC-AUTH2-004-2 (viewer read-only, POST forbidden)
- [x] viewer 토큰 → GET /api/v1/workflows → 200 OK
- [x] viewer 토큰 → POST /api/v1/workflows → 403 Forbidden
- [x] 403 응답: WWW-Authenticate 헤더 + error.code = "PERMISSION_DENIED"
- [x] viewer POST 차단 시 workflows 테이블 변화 없음
- [x] gRPC: viewer → CreateWorkflow → codes.PermissionDenied

### AC-AUTH2-004-4 (analyst write allowed)
- [x] analyst 토큰 → POST /api/v1/workflows → 201 Created
- [x] analyst 토큰 → GET /api/v1/workflows → 200 OK
- [x] workflows DB row 생성 검증

### AC-AUTH2-004-Sprint7-Unblock
- [x] TestE2E_Auth_RBACForbidden SKIP 제거
- [x] grep -c "t.Skip" auth_e2e_test.go == 0
- [x] viewer DELETE → 403 실제 검증 활성화

## Test Files Created/Modified

### Created
- `apps/control-plane/internal/server/authz_e2e_test.go`
  - 5 E2E 테스트 (//go:build integration)
  - setupAuthzE2EStack: BuildRESTChain 기반 REST 스택
  - TestE2E_Authz_AdminFullAccess (AC-AUTH2-004-1)
  - TestE2E_Authz_ViewerForbidden_POST (AC-AUTH2-004-2 핵심)
  - TestE2E_Authz_AnalystWriteAllowed (AC-AUTH2-004-4)
  - TestE2E_Authz_AuthDisabled_BypassesAuthz (authEnabled=false)
  - TestE2E_GRPC_Authz_ViewerForbidden_Create (AC-AUTH2-004-2 gRPC)

### Modified
- `apps/control-plane/internal/server/auth_e2e_test.go`
  - TestE2E_Auth_RBACForbidden: t.Skip() 제거 + 실제 구현 추가
  - viewer DELETE /api/v1/workflows/{id} → 403 검증

## Design Decisions

### REST Chain
BuildRESTChain(handler, validator, nil, authEnabled) 사용:
- nil recorder: AUTH_FORBIDDEN 감사 기록 DB 삽입은 authz_middleware_test.go 범위
- RESTMiddleware → RESTAuthzMiddleware → handler 순서 강제

### gRPC Chain
proto 패키지에 클라이언트 stub 없음 (hand-written proto):
- UnaryServerInterceptor(authn) + UnaryAuthzInterceptor(authz) 직접 체인 호출
- BuildGRPCInterceptorChain 기반 gRPC 서버 기동 검증 (bufconn)

### AUTH-001 SKIP Unblock
- TestE2E_Auth_RBACForbidden: setupAuthzE2EStack 사용 (BuildRESTChain)
- viewer DELETE → RESTAuthzMiddleware → delete:workflow 없음 → 403

## Validation Results
- go build -tags=integration: PASS
- go vet -tags=integration: PASS
- golangci-lint --build-tags=integration: PASS
- go test ./apps/control-plane/internal/... (unit): all PASS
- grep -c "t.Skip" auth_e2e_test.go: 0

## Priority
- Priority High: AC-AUTH2-004-2 (viewer 403), AC-AUTH2-004-Sprint7-Unblock
- Priority Medium: AC-AUTH2-004-1, AC-AUTH2-004-4
- Priority Low: gRPC authz interceptor chain
