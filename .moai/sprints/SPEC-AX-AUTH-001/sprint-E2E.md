# Sprint Contract — Sprint 7: E2E Integration Tests

**SPEC:** SPEC-AX-AUTH-001  
**Sprint:** 7 — AC-AUTH-E2E-1/2 E2E Integration  
**Phase:** GREEN (완료)  
**Priority:** Integration — 전체 인증 체인 검증

---

## 수락 기준 (Acceptance Criteria)

| AC | 설명 | 테스트 함수 | 결과 |
|----|------|------------|------|
| AC-AUTH-E2E-1 | AuthEnabled=true, 유효한 JWT sub=kepco-analyst-001 → HTTP 201 + audit_logs.user_id = kepco-analyst-001 | TestE2E_Auth_FullChainWithValidToken | PASS |
| AC-AUTH-E2E-2 | AuthEnabled=false, Authorization 헤더 없음 → HTTP 201 + user_id = cli-anonymous | TestE2E_Auth_AnonymousFallback | PASS |
| AC-AUTH-E2E-3 | RBAC 미승인 → Option B: SKIP (SPEC-AX-AUTH-002로 이관) | TestE2E_Auth_RBACForbidden | SKIP |
| AC-AUTH-E2E-4 | 잘못된 서명 토큰 → HTTP 401, 워크플로우 미생성 | TestE2E_Auth_InvalidToken_401 | PASS |
| AC-AUTH-E2E-CONC | 5개 goroutine 동시 요청, user_id 격리 검증 | TestE2E_Auth_ConcurrentRequests | PASS |

---

## 테스트 인프라

### 컨테이너
- **PostgreSQL 16-alpine**: testcontainers-go, 자동 스키마 로딩 (`loadSchemaSQL`)
- **Redis 7-alpine**: testcontainers-go, goRedisAdapter 래퍼
- **정리 방법**: `t.Cleanup` (defer 미사용)

### 인증 스택
- **RSA 키 쌍**: `crypto/rsa.GenerateKey` (2048-bit), 메모리 전용 — 실제 Keycloak 미사용
- **mockJWKSProvider**: `auth.JWKSProvider` 인터페이스 구현, kid→공개키 인메모리 맵
- **TokenValidator**: `auth.New(ctx, issuer, audience, auth.WithJWKSProvider(mock), auth.WithAllowedAlgs(["RS256"]))`
- **RESTMiddleware**: `auth.RESTMiddleware(validator)(handler.Mux())`; nil validator = no-op (AuthEnabled=false)
- **Recorder**: `audit.NewRecorder(authEnabled bool)` — authEnabled=false 시 cli-anonymous 강제

### JWT 발급
- `golang-jwt/jwt/v5` RS256 서명
- Claims: `sub`, `iss`, `aud`, `exp`, `iat`, `jti`, `kid`
- `genE2ETestJWT` 헬퍼로 테스트별 커스텀 클레임 지원

---

## 스프린트 범위

### 신규 파일
- `apps/control-plane/internal/server/auth_e2e_test.go` — E2E 테스트 파일 (`//go:build integration`)

### 수정 파일
- `apps/control-plane/internal/server/grpc_server.go` — `CreateWorkflow`: `"cli-anonymous"` 하드코딩 제거, `auth.UserFromContext(ctx)` 로 JWT sub 추출

---

## 핵심 버그 수정

**근본 원인:** `WorkflowService.CreateWorkflow`가 `ExecuteWorkflowCreate` 호출 시 `"cli-anonymous"` 를 하드코딩하여 JWT 인증 사용자 정보를 무시

**수정 내용:**
```go
// 수정 전
s.sm.Coordinator().ExecuteWorkflowCreate(ctx, wf, req.DocumentID, "cli-anonymous")

// 수정 후
userID := audit.DefaultUserID
if u, ok := auth.UserFromContext(ctx); ok && u.UID != "" {
    userID = u.UID
}
s.sm.Coordinator().ExecuteWorkflowCreate(ctx, wf, req.DocumentID, userID)
```

**동작 원리:**
- `AuthEnabled=true`: `RESTMiddleware`가 JWT sub를 컨텍스트에 주입 → `UserFromContext` 추출 → `audit_logs.user_id = JWT sub`
- `AuthEnabled=false`: `RESTMiddleware` nil → 컨텍스트 미변경 → `DefaultUserID("cli-anonymous")` → `Recorder.resolveUserID` 동일값 반환

---

## RBAC E2E 이관 결정 (Option B)

**결정:** `TestE2E_Auth_RBACForbidden` — `t.Skip("SPEC-AX-AUTH-002: ...")`

**이유:**
- RBAC 라이브러리 (`rbac.go`) 단위 테스트 완료 (Sprint 5)
- REST 핸들러 → `Authorize()` 엔드-투-엔드 배선은 별도 통합 관심사
- Sprint 7 범위: 인증 체인 검증 (미들웨어 → validator → JWKS → audit_logs)
- RBAC 엔드포인트 배선은 SPEC-AX-AUTH-002에서 구현

---

## 검증 결과

```
=== RUN   TestE2E_Auth_FullChainWithValidToken    --- PASS (7.38s)
=== RUN   TestE2E_Auth_AnonymousFallback          --- PASS (6.36s)
=== RUN   TestE2E_Auth_RBACForbidden              --- SKIP
=== RUN   TestE2E_Auth_InvalidToken_401           --- PASS (6.32s)
=== RUN   TestE2E_Auth_ConcurrentRequests         --- PASS (6.37s)
PASS  ok  github.com/ircp/iroum-ax/apps/control-plane/internal/server  26.442s
```

**단위 테스트 회귀:** audit/auth/scheduler/server/store/workflow 패키지 전체 PASS

---

## @MX Tag Report — Sprint 7 GREEN

### Tags Updated (1)
- `grpc_server.go` `CreateWorkflow`: `"cli-anonymous"` 하드코딩 제거로 `@MX:ANCHOR` 설명 업데이트 필요 없음 (로직 확장이지 계약 변경 아님)

### Attention Required
- `WorkflowService` `@MX:ANCHOR`는 fan_in=3 유지 (grpc_server.go, grpc_server_test.go, auth_e2e_test.go) — ANCHOR 유효
