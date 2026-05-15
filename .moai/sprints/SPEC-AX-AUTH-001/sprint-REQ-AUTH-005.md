# Sprint Contract — REQ-AUTH-005 Refresh Token Rotation + Logout

**SPEC**: SPEC-AX-AUTH-001
**REQ**: REQ-AUTH-005-E1 (Logout), REQ-AUTH-005-U1 (Family Invalidation)
**Priority**: Security — OAuth 2.0 BCP critical
**Phase**: Sprint 6 RED (2026-05-15)

---

## Acceptance Checklist

### REQ-AUTH-005-E1 Logout

- [ ] AC-AUTH-005-1: POST /auth/logout 시 access token jti가 Redis 블랙리스트에 등록된다
- [ ] AC-AUTH-005-1: POST /auth/logout 시 refresh token jti가 Redis 블랙리스트에 등록된다
- [ ] AC-AUTH-005-1: 두 jti의 TTL = token.exp - now (EXPIREAT)
- [ ] AC-AUTH-005-1: audit_logs row action=AUTH_LOGOUT, user_id=access_token sub
- [ ] AC-AUTH-005-4: malformed refresh_token으로 logout 시 에러 반환

### REQ-AUTH-005-U1 Refresh Token Family Invalidation

- [ ] AC-AUTH-005-3: Refresh token family_id가 Redis에 추적된다 (auth:refresh_family:<family_id>)
- [ ] AC-AUTH-005-3: 이미 사용된 jti로 refresh 시도 → ErrRefreshTokenReuseDetected 반환
- [ ] AC-AUTH-005-3: family 전체 jti가 블랙리스트에 등록된다 (rt-original + rt-new 모두)
- [ ] AC-AUTH-005-3: audit_logs row action=AUTH_REFRESH_REUSE_DETECTED, details={family_id, reused_jti}
- [ ] AC-AUTH-005: 정상 refresh 시 새 access/refresh token 쌍 반환
- [ ] AC-AUTH-005: 만료된 refresh token으로 refresh 시 ErrRefreshTokenExpired

---

## Test Seams (Sprint 6 RED 기준)

| 인터페이스 | 역할 | 구현 |
|-----------|------|------|
| `RefreshTokenStore` | 블랙리스트 + family tracking | `fakeRefreshStore` (인메모리) |
| `TokenIssuer` | 새 token 쌍 발급 | `fakeTokenIssuer` (고정 반환) |
| `AuditLogger` | 감사 이벤트 기록 | `fakeAuditLogger` (인메모리 수집) |

---

## RED 테스트 목록 (Sprint 6 신규 — 13개)

| 테스트 | 예상 상태 |
|--------|----------|
| TestRefreshTokenStore_BlacklistJTI_AddsToSet | PASS (fakeStore 자체 검증) |
| TestRefreshTokenStore_IsBlacklisted_ReturnsTrue | PASS (fakeStore 자체 검증) |
| TestRefreshTokenStore_IsBlacklisted_NotInSet_ReturnsFalse | PASS (fakeStore 자체 검증) |
| TestRefreshTokenStore_InvalidateFamily_BlacklistsAllJTIs | PASS (fakeStore 자체 검증) |
| TestRefreshSession_HappyPath_NewPairReturned | **FAIL** (stub "구현 예정") |
| TestRefreshSession_AlreadyUsedToken_InvalidatesFamily | **FAIL** (stub "구현 예정" + ErrReuseDetected 불일치) |
| TestRefreshSession_BlacklistedJTI_ReturnsError | PASS (stub 에러 → require.Error 통과) |
| TestRefreshSession_ExpiredRefreshToken_ReturnsError | PASS (stub 에러 → require.Error 통과) |
| TestLogout_BlacklistsAccessAndRefreshTokens | **FAIL** (stub "구현 예정") |
| TestLogout_RecordsAuditEvent | **FAIL** (stub "구현 예정") |
| TestLogout_InvalidToken_ReturnsError | PASS (stub 에러 → require.Error 통과) |
| TestAuditAction_AuthLogout_Defined | PASS (상수 정의 확인) |
| TestAuditAction_AuthRefreshReuseDetected_Defined | PASS (상수 정의 확인) |

**진성 RED 수**: 4개 (HappyPath, ReuseDetected, BlacklistsTokens, RecordsAudit)

---

## GREEN 구현 계획 (Sprint 6 GREEN)

1. `RefreshSession`:
   - `validator.Verify()` → jti, family_id, exp 추출
   - `store.IsBlacklisted(jti)` → true → `store.InvalidateFamily(family_id)` + `ErrRefreshTokenReuseDetected`
   - `store.GetFamilyMembers(family_id)` → jti 포함 → family invalidation
   - 정상: `store.AddToFamily(family_id, jti, exp)` + `issuer.IssueTokenPair()`

2. `Logout`:
   - `validator.Verify(accessToken)` → jti_access, exp_access, sub 추출
   - `validator.Verify(refreshToken)` → jti_refresh, exp_refresh 추출
   - `store.BlacklistJTI(jti_access, exp_access)` + `store.BlacklistJTI(jti_refresh, exp_refresh)`
   - `auditLogger.LogEvent(AUTH_LOGOUT, user_id=sub)`

---

## Pass Conditions

- 진성 RED 4개 → GREEN으로 전환
- Sprint 1-5 회귀 없음 (auth 외 패키지 모두 PASS 유지)
- LSP 에러 0개, 타입 에러 0개
- golangci-lint clean

---

Version: 1.0.0
Created: 2026-05-15
