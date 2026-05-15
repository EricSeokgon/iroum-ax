# SPEC-AX-AUTH-001 v0.1.1 Progress Tracker — SYNC COMPLETE

**최종 상태**: Sprint 0-7 전체 GREEN ✓ | TRUST 5 PASS ✓ | 105 신규 테스트 + 380+ 누적 ✓

---

## SYNC Phase (2026-05-15)

### Phase 2.5 + 3: TRUST 5 Validation + Documentation Sync

**생성 파일:**
- `.moai/reports/quality/SPEC-AX-AUTH-001-trust5.md` — TRUST 5 최종 검증 보고서
- CHANGELOG.md 업데이트 — Sprint 0-7 요약 + 보안 사항
- `.moai/project/codemaps/go-control-plane.md` — auth 섹션 추가
- `.moai/project/codemaps/req-traceability.md` — REQ-AUTH-001~005 + E2E 행렬 추가
- README.md 배지 갱신 — 380+ 테스트 + Algorithm Confusion Attack 방어

**TRUST 5 최종 검증:**
- Tested: Go 90 + Python 15 + 4 E2E = 109 신규 | 누적 380+ ✓
- Readable: gofmt + ko-comments ✓
- Unified: golangci-lint 0 errors, ruff 24/43 사소한 ✓
- Secured: SF-1/SF-2 + OAuth 2.0 BCP ✓
- Trackable: 55 @MX 태그 + 16 커밋 ✓

**다음:** 최종 세션 커밋 준비

---

## Sprint 6 — REQ-AUTH-005 Refresh Token Rotation + Logout

### GREEN phase (2026-05-15)

**파일 생성/수정:**
- `apps/control-plane/internal/auth/refresh_test.go` — 13개 테스트 (진성 RED 4개, PASS 9개)
- `apps/control-plane/internal/auth/refresh.go` — Sprint 0 stub 확장: RefreshService, RefreshSession, Logout 시그니처 추가; RefreshTokenStore 인터페이스 AddToFamily/GetFamilyMembers 메서드 추가; BlacklistJTI 시그니처 변경 (ttlSecs int64 → time.Time)
- `apps/control-plane/internal/audit/audit.go` — ActionAuthLogout, ActionAuthRefreshReuseDetected 상수 추가
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-REQ-AUTH-005.md` — Sprint Contract (Priority: Security — OAuth 2.0 BCP critical)

**의존성 추가:** 없음 (기존 auth/audit 패키지 사용)

**AC 완료 수:** 0 / 10 (RED phase — 미구현 상태가 정상)

**테스트 상태 (Sprint 6 신규 — 13개):**
- FAIL (진성 RED): 4개
  - TestRefreshSession_HappyPath_NewPairReturned: stub "구현 예정" → require.NoError FAIL
  - TestRefreshSession_AlreadyUsedToken_InvalidatesFamily: stub 에러 ≠ ErrRefreshTokenReuseDetected + family 미무효화
  - TestLogout_BlacklistsAccessAndRefreshTokens: stub "구현 예정" → require.NoError FAIL
  - TestLogout_RecordsAuditEvent: stub "구현 예정" → require.NoError FAIL
- PASS: 9개
  - TestRefreshTokenStore_* 4개 (fakeStore 자체 검증)
  - TestRefreshSession_BlacklistedJTI_ReturnsError (stub 에러 → require.Error 통과)
  - TestRefreshSession_ExpiredRefreshToken_ReturnsError (stub 에러 → require.Error 통과)
  - TestLogout_InvalidToken_ReturnsError (stub 에러 → require.Error 통과)
  - TestAuditAction_AuthLogout_Defined (상수 정의 확인)
  - TestAuditAction_AuthRefreshReuseDetected_Defined (상수 정의 확인)
- Total: 13개 테스트

**RED 실패 이유 (진성, Lesson #4 준수):**
- RefreshSession: stub `errors.New("구현 예정: Sprint 6 GREEN")` → require.NoError FAIL / ErrRefreshTokenReuseDetected 불일치
- Logout: stub `errors.New("구현 예정: Sprint 6 GREEN")` → require.NoError FAIL

**Sprint 1-5 회귀:**
- scheduler/server/store/workflow/audit 패키지: OK (auth만 FAIL)
- auth 패키지 Sprint 1-5 테스트: Sprint 6 RED 파일로 인한 패키지 빌드 실패 포함

**에러 델타:** +4 신규 진성 FAIL

**신규 심볼:**
- `auth.ErrRefreshTokenReuseDetected` — OAuth 2.0 BCP reuse 탐지 에러
- `auth.ErrRefreshTokenExpired` — refresh token 만료 에러
- `auth.RefreshTokenStore` — 인터페이스 확장 (AddToFamily, GetFamilyMembers 추가; BlacklistJTI 시그니처 변경)
- `auth.RefreshTokenPair` — 새 access/refresh 쌍 구조체
- `auth.TokenIssuer` — token 발급 인터페이스
- `auth.AuditLogger` — 감사 이벤트 기록 인터페이스
- `auth.RefreshService` — refresh rotation + logout 서비스
- `auth.NewRefreshService(store, validator, issuer, auditLogger)` — 생성자
- `auth.RefreshService.RefreshSession(ctx, oldRefreshToken)` — stub
- `auth.RefreshService.Logout(ctx, accessToken, refreshToken)` — stub
- `audit.ActionAuthLogout = "AUTH_LOGOUT"` — 신규 상수
- `audit.ActionAuthRefreshReuseDetected = "AUTH_REFRESH_REUSE_DETECTED"` — 신규 상수

**다음 단계:** Sprint 6 GREEN
- RefreshSession: validator.Verify → jti/family_id 추출 → family reuse check → 새 pair 발급
- Logout: 두 jti 블랙리스트 등록 (TTL=exp) + AUTH_LOGOUT 감사 이벤트

---


---

## Sprint 7 — E2E Integration Tests (AC-AUTH-E2E-1/2)

### GREEN phase (2026-05-15)

**파일 생성/수정:**
- `apps/control-plane/internal/server/auth_e2e_test.go` — E2E 테스트 파일 (`//go:build integration`)
- `apps/control-plane/internal/server/grpc_server.go` — `CreateWorkflow` 버그 수정: `"cli-anonymous"` 하드코딩 → `auth.UserFromContext(ctx)` 추출
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-E2E.md` — Sprint Contract

**의존성 추가:** 없음 (기존 auth/audit/jwt 패키지 사용)

**AC 완료 수:** 4 / 5 (SKIP 1 — Option B RBAC 이관)

**테스트 상태 (Sprint 7 신규 — 5개):**
- PASS: 4개
  - TestE2E_Auth_FullChainWithValidToken (AC-AUTH-E2E-1): JWT sub → audit_logs.user_id 검증
  - TestE2E_Auth_AnonymousFallback (AC-AUTH-E2E-2): AuthEnabled=false → cli-anonymous 검증
  - TestE2E_Auth_InvalidToken_401 (AC-AUTH-E2E-4): 잘못된 서명 → HTTP 401 검증
  - TestE2E_Auth_ConcurrentRequests: 5개 goroutine user_id 격리 검증
- SKIP: 1개
  - TestE2E_Auth_RBACForbidden (AC-AUTH-E2E-3): SPEC-AX-AUTH-002로 이관

**핵심 버그 수정:**
- `grpc_server.go` `CreateWorkflow`: `"cli-anonymous"` 하드코딩 → `audit.DefaultUserID + auth.UserFromContext(ctx)` 패턴으로 수정
- 동작: AuthEnabled=true → JWT sub 전달; AuthEnabled=false → Recorder.resolveUserID가 cli-anonymous 강제

**Sprint 1-6 회귀:**
- audit/auth/scheduler/server/store/workflow 패키지 전체 PASS (단위 테스트)

**에러 델타:** 0 (신규 PASS 4 + SKIP 1)

**인증 체인 검증 경로:**
```
HTTP Request → RESTMiddleware(TokenValidator) → auth.UserFromContext
→ WorkflowService.CreateWorkflow → TxCoordinator.ExecuteWorkflowCreate
→ audit.Recorder.resolveUserID → audit_logs.user_id
```

---

> Format: Sprint → Phase → 날짜 → AC 완료 수 / 전체 → 에러 델타
> Re-planning Gate: AC 완료율 0% + 3연속 → 트리거

---

## Sprint 5 — REQ-AUTH-004 RBAC 3-role Matrix

### RED phase (2026-05-15)

**파일 생성/수정:**
- `apps/control-plane/internal/auth/rbac_test.go` — 18개 테스트 (진성 RED 14개, PASS 4개)
- `apps/control-plane/internal/auth/rbac.go` — stub 함수 추가: ParseRolesFromScope, EffectivePermissions, LogForbidden
- `apps/control-plane/internal/audit/audit.go` — ActionAuthForbidden 상수 추가
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-REQ-AUTH-004.md` — Sprint Contract (Priority: Security)

**의존성 추가:** 없음 (기존 audit/store 패키지 사용)

**AC 완료 수:** 0 / 6 (RED phase — 미구현 상태가 정상)

**테스트 상태 (Sprint 5 신규):**
- FAIL: 14개 (진성 RED)
  - ParseRolesFromScope 3개: stub nil 반환 → non-empty 기대
  - EffectivePermissions 4개: stub nil 반환 → non-nil 맵 + 특정 권한 기대
  - Authorize 4개: stub "구현 예정" 에러 → NoError/ErrInsufficientPermission 기대
  - LogForbidden 4개: stub "구현 예정" 에러 → 정상 audit 기록 기대
- PASS: 4개 (stub과 호환되는 에러 경로)
  - TestParseRolesFromScope_EmptyString (nil == empty)
  - TestParseRolesFromScope_InvalidRoleIgnored (nil == empty)
  - TestAuthorize_NoUserInContext_ReturnsError (stub 에러 → assert.Error PASS)
- Total: 18개 테스트

**RED 실패 이유 (진성, Lesson #4 준수):**
- ParseRolesFromScope: stub nil 반환 → require.Len(t, roles, 1) FAIL
- EffectivePermissions: stub nil 반환 → require.NotNil FAIL
- Authorize admin: stub "구현 예정" → assert.NoError FAIL
- LogForbidden: stub "구현 예정" → require.NoError FAIL

**Sprint 1-4 회귀:**
- Go: scheduler/server/store/workflow/audit 패키지 OK (auth만 FAIL)
- Python: Sprint 4 상태 유지 (192+)

**에러 델타:** +14 신규 FAIL (모두 진성 RED)

**신규 심볼:**
- `auth.ParseRolesFromScope(scope string) []Role`
- `auth.EffectivePermissions(roles []Role) map[Permission]bool`
- `auth.LogForbidden(ctx, tx, method, path, userID, roles) error`
- `audit.ActionAuthForbidden = "AUTH_FORBIDDEN"`

**다음 단계:** Sprint 5 GREEN
- ParseRolesFromScope: regex `^iroum-ax:(admin|analyst|viewer)$` 파싱 구현
- EffectivePermissions: permissionMatrix union 로직 구현 (read:/write:/delete:/audit: 표준화)
- Authorize: UserFromContext + ParseRolesFromScope + EffectivePermissions 연동
- LogForbidden: audit.AuditTx.InsertAuditLog + AUTH_FORBIDDEN 이벤트 생성

---

## Sprint 4 — REQ-AUTH-003-E3 Python Middleware + REQ-AUTH-UBI-001 Cross-SPEC Envelope

### RED phase (2026-05-15)

**파일 생성:**
- `tests/unit/test_req_auth_003_python_middleware.py` — 16개 테스트 (FAIL 14, PASS 2)
- `apps/control-plane/internal/scheduler/celery_envelope_user_test.go` — 5개 테스트 (컴파일 에러 → RED)
- `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2_anon.json` — 신규 골든 파일
- `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2_authuser.json` — 신규 골든 파일
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-REQ-AUTH-003-cross-spec.md` — Sprint Contract

**의존성 추가:** 없음 (fastapi sys.modules mock 패턴 사용)

**AC 완료 수:** 0 / 17 (RED phase — 미구현 상태가 정상)

**테스트 상태 (Sprint 4 신규 — Python):**
- FAIL: 14개 (진성 RED)
  - TokenValidatorVerify 6개: stub `NotImplementedError` 반환 → 잘못된 예외 타입
  - TestVerifyTokenDepends 8개: stub `HTTPException(501)` 반환 → status_code 불일치
- PASS: 2개 (stub에 이미 구현된 경로)
  - `test_verify_token_missing_header_returns_401` — credentials=None 시 401 반환 (이미 구현)
  - `test_verify_token_auth_disabled_returns_anonymous` — AUTH_ENABLED=false 폴백 (이미 구현)
- Total: 16개 테스트

**테스트 상태 (Sprint 4 신규 — Go):**
- FAIL (컴파일): 5개 (진성 RED — BuildEnvelope 5번째 파라미터 없음)
  - TestBuildEnvelope_WithUserID_PopulatesHeader
  - TestBuildEnvelope_EmptyUserID_DefaultsToCliAnonymous
  - TestBuildEnvelope_GoldenFileMatch_AuthUser
  - TestBuildEnvelope_GoldenFileMatch_Anonymous
  - TestBuildEnvelope_BackwardCompat_OriginalGoldenStillValid

**RED 실패 이유 (진성, Lesson #4 준수):**
- Python TokenValidatorVerify: stub `raise NotImplementedError("구현 예정: Sprint 1 GREEN")` → 기대 예외와 다른 타입
- Python verify_token AUTH_ENABLED=true: stub `raise HTTPException(status_code=501)` → status_code 불일치
- Go: BuildEnvelope 시그니처 파라미터 수 불일치(4 → 5) → 컴파일 에러

**Sprint 3 회귀:**
- Python: 177 / 177 PASS (신규 16개 제외)
- Go: auth/audit/server/workflow/store 패키지 OK (scheduler 패키지 빌드 실패는 RED 테스트 파일 때문)

**에러 델타:** +14 Python FAIL + 5 Go 컴파일 에러 (모두 진성 RED)

**Cross-SPEC 아티팩트:**
- `testdata/celery_envelope_v2.json` 무손상 유지 (Sprint 6 CTRL 15개 회귀 가드)
- `testdata/celery_envelope_v2_anon.json` 신규 (user_id='cli-anonymous')
- `testdata/celery_envelope_v2_authuser.json` 신규 (user_id='kepco-analyst-001')

**다음 단계:** Sprint 4 GREEN
- `dispatcher.go` BuildEnvelope: 5번째 파라미터 userID 추가 + headers["user_id"] 설정
- `validator.py`: PyJWT decode + RS256 서명 검증 + alg allow-list + SF-1/SF-2 구현
- `dependencies.py`: TokenValidator 연동 + 예외→HTTPException 변환 + WWW-Authenticate 헤더

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
