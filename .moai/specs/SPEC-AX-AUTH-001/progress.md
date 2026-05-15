# SPEC-AX-AUTH-001 Progress Tracker

> Format: Sprint → Phase → 날짜 → AC 완료 수 / 전체 → 에러 델타
> Re-planning Gate: AC 완료율 0% + 3연속 → 트리거

---

## Sprint 3 — REQ-AUTH-003 gRPC + REST Middleware (Go)

### RED phase (2026-05-15)

**파일 생성:**
- `apps/control-plane/internal/auth/middleware_test.go` — 20개 테스트 함수 (FAIL 13, PASS 4, SKIP 1, PASS-always 2)
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-REQ-AUTH-003.md` — Sprint Contract

**의존성 추가:** 없음 (bufconn + health proto는 grpc v1.81.0 내장)

**AC 완료 수:** 0 / 10 (RED phase — 미구현 상태가 정상)

**테스트 상태 (Sprint 3 신규):**
- FAIL: 13개 (진성 RED — stub "구현 예정: Sprint 3 GREEN" 에러 + status code 불일치)
- PASS: 4개 (context round-trip, AuthDisabled no-op, User struct — 이미 구현된 경로)
- SKIP: 1개 (TestRESTMiddleware_WWWAuthenticate_Format — GREEN 후 검증)
- Total: 20개 테스트 함수

**RED 실패 이유 (진성, Lesson #4 준수):**
- gRPC ValidToken/ContextHasUser: stub이 `errors.New("구현 예정: Sprint 3 GREEN")` 반환 → `require.NoError` FAIL
- gRPC HealthCheck_Bypass: stub이 health check도 차단 → `require.NoError` FAIL
- gRPC NoAuth/InvalidToken/Expired/Malformed: stub이 gRPC status 에러가 아닌 일반 에러 반환 → `status.FromError(err)` → `ok=false` → `require.True` FAIL
- REST ValidToken/UserInjected: stub이 501 반환 → `assert.Equal(200, ...)` FAIL
- REST Missing/Malformed/Invalid/Empty: stub이 501 반환 → `assert.Equal(401, ...)` FAIL
- REST HealthBypass: stub이 모든 요청에 501 반환 → `/health` 200 기대 FAIL

**Sprint 1+2 회귀:**
- PASS: 37/37 (전체 유지)

**에러 델타:** +13 신규 FAIL

**gRPC 테스트 인프라:**
- `bufconn.Listen(1MB)` — in-process gRPC, 실제 포트 없음
- `health.RegisterHealthServer` — /grpc.health.v1.Health/Check 우회 검증용
- 인터셉터 직접 호출 패턴 — `dummyHandler` + `capturingHandler`로 context user 검증

**REST 테스트 인프라:**
- `httptest.NewServer(RESTMiddleware(v)(mux))` — net/http httptest 기반

**다음 단계:** Sprint 3 GREEN — UnaryServerInterceptor + RESTMiddleware 구현

---

## Sprint 2 — REQ-AUTH-002 OIDC Discovery + JWKS Cache

### RED phase (2026-05-15)

**파일 생성:**
- `apps/control-plane/internal/auth/oidc.go` — OIDCClient stub (NewOIDCClient, Discover, GetMetadata)
- `apps/control-plane/internal/auth/jwks_cache.go` — JWKSCache stub (NewJWKSCache, GetKey, refresh)
- `apps/control-plane/internal/auth/oidc_test.go` — 6개 테스트 함수
- `apps/control-plane/internal/auth/jwks_cache_test.go` — 11개 테스트 함수
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-REQ-AUTH-002.md` — Sprint Contract

**의존성 추가:** 없음 (stdlib net/http + httptest 전용)

**AC 완료 수:** 0 / 8 (RED phase — 미구현 상태가 정상)

**테스트 상태 (Sprint 2 신규):**
- FAIL: 1개 (`TestOIDCClient_Discover_Success` — require.NoError로 stub FAIL 유도)
- PASS: 16개 (에러 확인 테스트 / 인터페이스 검증 / 옵션 검증)
- 이유: OIDC/JWKS 테스트 대부분 `assert.Error` 패턴 → stub 에러도 수용
- 핵심 RED 신호: `TestOIDCClient_Discover_Success`가 SUCCESS path 검증

**Sprint 1 회귀:**
- PASS: 47 subtests (Sprint 1 전체 유지)

**에러 델타:** +1 신규 FAIL (TestOIDCClient_Discover_Success — 진성 RED)

**Lesson #4 적용:**
- `ErrNotImplemented` sentinel 패턴 회피
- 성공 경로 테스트(`Discover_Success`)만 `require.NoError` 사용 → stub에서 자연 FAIL
- 에러 경로 테스트는 `assert.Error` 사용 → stub에서 의도적 PASS (RED 상태 명확화)

**testcontainers 의존:** 없음 (httptest.NewServer 전용)

**다음 단계:** Sprint 2 GREEN — OIDCClient.Discover + JWKSCache.GetKey 구현

---

## Sprint 1 — REQ-AUTH-001 JWT Validator

### RED phase (2026-05-15)

**파일 생성:**
- `apps/control-plane/internal/auth/validator_test.go` — 19개 테스트 함수 (테이블 포함 실제 케이스 25+)
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-REQ-AUTH-001.md` — Sprint Contract
- `.moai/specs/SPEC-AX-AUTH-001/progress.md` — 이 파일

**의존성 추가:**
- `github.com/golang-jwt/jwt/v5 v5.2.2` → go.mod

**AC 완료 수:** 0 / 16 (RED phase — 미구현 상태가 정상)

**테스트 상태:**
- FAIL: 16개 (stub `errors.New("구현 예정: Sprint 1 GREEN")` 반환)
- PASS: 3개 (헬퍼 자기검증 2 + Blacklisted 에러 반환 확인 1)
- 기존 회귀: 0 (audit/scheduler/server/store/workflow 패키지 OK)

**SF-1 전용 테스트:**
- `TestVerify_IssuerMismatch_Rejected` — iss 불일치 → ErrTokenInvalidIssuer
- `TestVerify_IssuerMatch_Accepted` — iss 일치 → 정상 통과

**SF-2 전용 테스트:**
- `TestVerify_AlgKeyTypeMismatch_Rejected` — kty=RSA + token alg=ES256 → ErrAlgorithmKeyMismatch
- `TestVerify_AlgKeyTypeMatch_RSA_Accepted` — kty=RSA + alg=RS256 → cross-check 통과

**Lesson #4 적용:**
- `ErrNotImplemented` sentinel 패턴 회피
- stub은 `errors.New("구현 예정: Sprint 1 GREEN")` 반환
- GREEN 직후 자연스럽게 PASS로 전환되는 구조

**에러 델타:** +16 신규 FAIL (기대값, RED phase 정상)

**다음 단계:** Sprint 1 GREEN — TokenValidator.Verify 구현

---

## Sprint 0 — Foundation Stubs (완료)

- `validator.go` stub 생성
- `errors.go` sentinel 에러 정의
- `middleware.go` stub 생성
- `rbac.go` stub 생성
- `refresh.go` stub 생성
- `pkg/auth/claims.go` 공유 Claims 구조체
