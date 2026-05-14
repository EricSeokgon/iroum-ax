# Sprint Contract — SPEC-AX-AUTH-001 / REQ-AUTH-001 JWT Validator

> Phase: Sprint 1 RED (완료) → GREEN (다음)
> Priority Dimension: Security
> Harness Level: thorough

---

## 1. 스코프

REQ-AUTH-001 JWT 토큰 검증기 (`TokenValidator.Verify`) 구현.

범위:
- 서명 검증 (RS256 / ES256 / EdDSA)
- 시간 클레임 (exp / nbf / iat) + clock skew 30초
- Issuer 검증 (SF-1: RFC 7519 §4.1.1)
- Audience 검증
- Algorithm allow-list (HS256 / none 거부)
- Algorithm / Key Type cross-check (SF-2: OWASP JWT)
- Redis 블랙리스트 jti 확인 (REQ-AUTH-001-S1)

---

## 2. Acceptance Checklist (Sprint 1)

| # | 항목 | AC 참조 | 우선순위 | 상태 |
|---|------|---------|---------|------|
| 1 | 유효한 RS256 토큰 → ValidatedToken 반환 | AC-AUTH-001-1 | P1 | RED |
| 2 | 만료 토큰 → ErrTokenExpired | AC-AUTH-001-2 | P1 | RED |
| 3 | 미래 iat (30초 초과) → 거부 | AC-AUTH-001-3 | P1 | RED |
| 4 | Clock skew 30초 이내 → 수용 | AC-AUTH-001-7 | P1 | RED |
| 5 | aud 불일치 → ErrTokenInvalidAudience | AC-AUTH-001-4 | P1 | RED |
| 6 | HS256 → ErrAlgorithmNotAllowed | AC-AUTH-001-5 | P1 (Security) | RED |
| 7 | alg=none → ErrAlgorithmNotAllowed | AC-AUTH-001-6 | P1 (Security) | RED |
| 8 | ES256 → 수용 | allow-list | P1 | RED |
| 9 | 변조 서명 → ErrTokenInvalidSignature | AC-AUTH-001-8 | P1 (Security) | RED |
| 10 | jti 블랙리스트 → ErrTokenBlacklisted | AC-AUTH-001-10 | P1 | RED |
| 11 | iss 불일치 → ErrTokenInvalidIssuer (SF-1) | AC-AUTH-001-iss-validation | P1 (Security) | RED |
| 12 | iss 일치 → 수용 | AC-AUTH-001-iss-validation | P1 | RED |
| 13 | kty=RSA + alg=ES256 → ErrAlgorithmKeyMismatch (SF-2) | AC-AUTH-001-alg-cross-check | P1 (Security) | RED |
| 14 | kty=RSA + alg=RS256 → cross-check 통과 | AC-AUTH-001-alg-cross-check | P1 | RED |
| 15 | ValidatedToken 필드 정확성 | AC-AUTH-001-1 | P2 | RED |
| 16 | BenchmarkVerify p99 < 5ms | AC-AUTH-001-Performance | P2 | RED |

---

## 3. Must-Pass Security Criteria

Security 차원 최소 점수: **0.75** (evaluator-active 기준)

필수 통과 항목 (must-pass — 하나라도 실패 시 Sprint FAIL):

- [ ] HS256 거부 — Algorithm Confusion Attack 방어 (OWASP JWT §3)
- [ ] alg=none 거부 — 서명 우회 방어 (RFC 7519 §10.7)
- [ ] iss 검증 (SF-1) — cross-realm token 재사용 방어 (RFC 7519 §4.1.1)
- [ ] kty/alg cross-check (SF-2) — Algorithm Confusion 변형 방어 (OWASP JWT §5)
- [ ] 변조 서명 거부 — token integrity 보장

---

## 4. Test Scenarios (Playwright — 해당 없음, Go 단위 테스트)

| 시나리오 | 파일 | 함수 |
|---------|------|------|
| 유효 토큰 happy path | validator_test.go | TestVerify_HappyPath |
| 만료 토큰 | validator_test.go | TestVerify_ExpiredToken |
| 미래 iat | validator_test.go | TestVerify_FutureIAT_Rejected |
| Clock skew 경계 | validator_test.go | TestVerify_ClockSkew_Within30s_Accepted |
| aud 불일치 | validator_test.go | TestVerify_WrongAudience |
| HS256 거부 | validator_test.go | TestVerify_AlgorithmAllowList_HS256Rejected |
| none 거부 | validator_test.go | TestVerify_AlgorithmAllowList_NoneRejected |
| ES256 수용 | validator_test.go | TestVerify_ES256_Accepted |
| 변조 서명 | validator_test.go | TestVerify_TamperedSignature |
| 블랙리스트 | validator_test.go | TestVerify_Blacklisted |
| iss 불일치 (SF-1) | validator_test.go | TestVerify_IssuerMismatch_Rejected |
| iss 일치 (SF-1) | validator_test.go | TestVerify_IssuerMatch_Accepted |
| kty/alg mismatch (SF-2) | validator_test.go | TestVerify_AlgKeyTypeMismatch_Rejected |
| kty/alg match (SF-2) | validator_test.go | TestVerify_AlgKeyTypeMatch_RSA_Accepted |
| Algorithm table | validator_test.go | TestVerify_AlgorithmAllowList_Table |
| Time claims table | validator_test.go | TestVerify_TimeClaims_Table |
| 필드 정확성 | validator_test.go | TestVerify_ValidatedTokenFields |
| 성능 | validator_test.go | BenchmarkVerify_JWKSCacheHit |

---

## 5. Sprint 1 GREEN 구현 힌트

```
1. golang-jwt/jwt/v5 Parser 사용
   - WithValidMethods([]string{"RS256","ES256","EdDSA"})
   - WithLeeway(30 * time.Second)
   - WithAudience(v.audience)
   - WithIssuer(v.oidcIssuer)

2. Keyfunc:
   - kid 헤더로 JWKS 맵에서 공개키 조회
   - kid 없으면 첫 번째 키 사용 (fallback)
   - kty/alg cross-check:
     RS256/RS384/RS512 → kty=RSA
     ES256/ES384/ES512 → kty=EC
     EdDSA            → kty=OKP
   - 불일치 시 ErrAlgorithmKeyMismatch 반환

3. iat 검증 (jwt/v5는 기본 미검증):
   - 파싱 후 claims에서 iat 추출
   - iat > now+30s → 거부

4. jti 블랙리스트:
   - 단위 테스트용: in-memory map (thread-safe)
   - 실제: Redis SET SISMEMBER auth:blacklist:<jti>

5. Blacklist seeding API:
   - TokenValidator에 AddToBlacklist(jti string) 메서드 추가
   - 또는 생성자에서 blacklist provider 인터페이스 주입
```

---

## 6. Pass Conditions

Sprint 1 GREEN 완료 기준:

- 16개 RED 테스트 → 모두 PASS
- BenchmarkVerify_JWKSCacheHit p99 < 5ms
- go vet 0 오류
- golangci-lint 0 오류
- 기존 79개 테스트 회귀 없음

---

Version: 1.0
Created: 2026-05-15
Status: RED phase complete
