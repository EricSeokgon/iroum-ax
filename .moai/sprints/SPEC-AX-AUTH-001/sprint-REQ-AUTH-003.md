# Sprint Contract — REQ-AUTH-003 (Go: gRPC + REST Middleware)

> SPEC-AX-AUTH-001 Sprint 3 — RED phase 완료 2026-05-15

---

## Priority Dimensions

**Priority 1 — Security**: Authorization 헤더 파싱 실패 시 codes.Unauthenticated / 401 반환, WWW-Authenticate 헤더 포함
**Priority 2 — Functionality**: 유효한 토큰 시 context user 주입, health check bypass, AuthDisabled no-op 폴백

---

## Acceptance Checklist (Sprint 3 GREEN 기준)

### gRPC Interceptor (AC-AUTH-003-1, 2, 3, 4)

- [ ] AC-AUTH-003-1: 유효한 token → handler 진입 + context에 `*User` 주입 (UID=sub, Scopes 추출)
- [ ] AC-AUTH-003-2: `/grpc.health.v1.Health/Check` → 인증 우회 (Authorization 없이 SERVING 반환)
- [ ] AC-AUTH-003-3 (gRPC): Authorization metadata 누락 → `codes.Unauthenticated`
- [ ] AC-AUTH-003-4 (gRPC): 유효하지 않은 토큰 (iss 불일치, 만료, malformed) → `codes.Unauthenticated`
- [ ] AuthDisabled (validator=nil) → handler no-op 통과

### REST Middleware (AC-AUTH-003-3, 4)

- [ ] 유효한 Bearer 토큰 → next 핸들러 호출 (200) + context user 주입
- [ ] Authorization 헤더 없음 → 401 + `WWW-Authenticate: Bearer realm="iroum-ax", error="invalid_request"`
- [ ] Bearer 접두사 없음 (e.g., `Token xxx`) → 401
- [ ] 유효하지 않은 토큰 → 401 + WWW-Authenticate
- [ ] `/health` 경로 → 인증 우회 (200)
- [ ] AuthDisabled (validator=nil) → next no-op 통과 (200)

---

## Test Scenarios (Sprint 3 RED 완료)

| 테스트 함수 | AC 대응 | 예상 결과 (GREEN 후) |
|-----------|--------|---------------------|
| TestWithUser_RoundTrip | AC-AUTH-003-1 보조 | PASS (현재 PASS) |
| TestUserFromContext_Empty | 헬퍼 검증 | PASS (현재 PASS) |
| TestUnaryInterceptor_ValidToken_PassesToHandler | AC-AUTH-003-1 | PASS |
| TestUnaryInterceptor_ContextHasUser | AC-AUTH-003-1 핵심 | PASS |
| TestUnaryInterceptor_NoAuthMetadata_ReturnsUnauthenticated | AC-AUTH-003-3 | PASS |
| TestUnaryInterceptor_InvalidToken_ReturnsUnauthenticated | AC-AUTH-003-4 | PASS |
| TestUnaryInterceptor_TokenExpired_ReturnsUnauthenticated | AC-AUTH-003-4 | PASS |
| TestUnaryInterceptor_HealthCheck_Bypass | AC-AUTH-003-2 | PASS |
| TestUnaryInterceptor_AuthDisabled_PassesAsAnonymous | AC-AUTH-UBI-001-C | PASS (현재 PASS) |
| TestUnaryInterceptor_MalformedBearer_ReturnsUnauthenticated | AC-AUTH-003-4 | PASS |
| TestRESTMiddleware_ValidToken_CallsNext | AC-AUTH-003-3 happy | PASS |
| TestRESTMiddleware_UserInjected_InContext | AC-AUTH-003-1 REST | PASS |
| TestRESTMiddleware_MissingHeader_Returns401 | AC-AUTH-003-3 case B | PASS |
| TestRESTMiddleware_MalformedHeader_Returns401 | AC-AUTH-003-4 | PASS |
| TestRESTMiddleware_InvalidToken_Returns401_WWWAuthenticate | AC-AUTH-003-4 | PASS |
| TestRESTMiddleware_HealthBypass | AC-AUTH-003-3 case A | PASS |
| TestRESTMiddleware_AuthDisabled_PassesAsAnonymous | AC-AUTH-UBI-001-C | PASS (현재 PASS) |
| TestRESTMiddleware_MissingToken_BearerParsing | AC-AUTH-003-U1 | PASS |
| TestUser_FieldAlignment | 구조체 검증 | PASS (현재 PASS) |
| TestRESTMiddleware_WWWAuthenticate_Format | AC-AUTH-003-3 format | PASS (SKIP → GREEN 후) |

---

## Pass Conditions

- GREEN 후 신규 FAIL 0개 (현재 13개 FAIL → 0개)
- Sprint 1+2 37개 테스트 회귀 없음
- `go vet` 0 에러
- `golangci-lint run` 0 에러
- `go build ./apps/control-plane/...` 성공

---

## Implementation Notes (GREEN phase 참고)

### gRPC Interceptor 구현 시 핵심 로직

```
1. metadata.FromIncomingContext(ctx) 로 incoming metadata 추출
2. md["authorization"][0] 에서 "Bearer <token>" 파싱
3. strings.TrimPrefix("Bearer ") 로 token 추출
4. info.FullMethod == grpc_health_v1.Health_Check_FullMethodName 이면 bypass
5. validator.Verify(ctx, token) 호출
6. 에러 시: status.Error(codes.Unauthenticated, ...) 반환
7. 성공 시: ValidatedToken → User 변환 → WithUser(ctx, user) → handler(newCtx, req)
```

### REST Middleware 구현 시 핵심 로직

```
1. r.URL.Path == "/health" 이면 bypass
2. authHeader := r.Header.Get("Authorization")
3. authHeader == "" → 401 + WWW-Authenticate
4. !strings.HasPrefix(authHeader, "Bearer ") → 401
5. token := strings.TrimPrefix(authHeader, "Bearer ")
6. token == "" → 401
7. validator.Verify(r.Context(), token) 호출
8. 에러 시: WWW-Authenticate 헤더 설정 + 401 JSON body
9. 성공 시: ValidatedToken → User 변환 → r = r.WithContext(WithUser(r.Context(), user)) → next.ServeHTTP
```

### ValidatedToken → User 변환

```go
// Verify 결과를 middleware용 User로 변환
user := &User{
    UID:    vt.Subject,
    Issuer: vt.Issuer,
    Scopes: vt.Scopes,
    // Roles: 추후 RBAC scope 파싱으로 추출
}
```

---

## Constraints

- Python FastAPI는 Sprint 4에서 처리 (본 Sprint 제외)
- StreamServerInterceptor는 stub 유지 (본 Sprint 제외)
- audit recorder.resolveUserID 확장은 Sprint 3 GREEN 이후 별도 AC-AUTH-003-6으로 처리

---

Version: 1.0.0
Sprint: 3 — RED phase
Date: 2026-05-15
