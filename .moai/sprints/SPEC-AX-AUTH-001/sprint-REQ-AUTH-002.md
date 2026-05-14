# Sprint Contract — REQ-AUTH-002: OIDC Discovery + JWKS Cache

**SPEC**: SPEC-AX-AUTH-001
**Sprint**: Sprint 2 — REQ-AUTH-002
**Priority Dimension**: Functionality
**Harness Level**: standard
**Date**: 2026-05-15

---

## Acceptance Checklist

### AC-AUTH-002-1 — Discovery Document Parsing

- [ ] `NewOIDCClient(ctx, issuerURL)` 호출 시 `/.well-known/openid-configuration` fetch
- [ ] `*OIDCClient.metadata.Issuer` = issuerURL (byte-identical)
- [ ] `*OIDCClient.metadata.JWKSUri` 정확히 파싱
- [ ] `*OIDCClient.metadata.TokenEndpoint` 정확히 파싱
- [ ] in-memory 캐시에 저장 (두 번째 호출 시 fetch 없음)

### AC-AUTH-002-2 — Fail-Fast Startup

- [ ] 10초 타임아웃 내 non-200 응답 → 에러 반환
- [ ] httptest hang 서버 → timeout 에러 반환
- [ ] panic 또는 error (main.go가 log.Fatal로 처리)

### AC-AUTH-002-3 — Issuer Mismatch Rejection

- [ ] discovery 응답 `issuer` != 요청 URL → 에러
- [ ] 에러 메시지에 "discovery issuer mismatch" 포함

### AC-AUTH-002-O1 — JWKS Cache TTL + stale-while-revalidate

- [ ] cache age < TTL → 캐시 히트, < 1ms 반환
- [ ] TTL <= cache age < staleMaxAge → stale 반환 + 백그라운드 refresh
- [ ] cache age >= staleMaxAge → blocking fetch
- [ ] fetch 실패 + stale 유효 → stale 반환 (degraded)
- [ ] fetch 실패 + 캐시 없음 → `ErrJWKSUnavailable`

### 동시성 + Race

- [ ] 10 goroutine 동시 GetKey → race detector 에러 없음
- [ ] 동시 fetch는 mutex로 단일화 (fetchCount <= 2)

---

## Pass Conditions

| Criterion | Minimum Score | Method |
|-----------|--------------|--------|
| Functionality (GetKey + Discovery) | PASS | 1 FAIL → 1 PASS (GREEN) |
| Concurrency Safety | PASS | go test -race |
| Performance (cache hit < 1ms) | PASS | elapsed < time.Millisecond |
| Error Handling (ErrJWKSUnavailable) | PASS | errors.Is 검증 |

---

## Test Scenarios (Priority Order)

1. `TestOIDCClient_Discover_Success` — **PRIMARY RED** (require.NoError → stub FAIL)
2. `TestJWKSCache_GetKey_CacheHit` — cache hit < 1ms (currently PASS in stub)
3. `TestJWKSCache_GetKey_TTLExpired_BackgroundRefresh` — stale-while-revalidate
4. `TestJWKSCache_GetKey_JWKSUnavailable_NoCacheError` — ErrJWKSUnavailable
5. `TestJWKSCache_GetKey_Concurrent` — race-free

---

## Sprint 2 GREEN Implementation Plan

### Priority 1: OIDCClient (oidc.go)

1. `Discover(ctx, issuerURL, client)` — HTTP GET + 10s timeout + JSON 파싱
2. `NewOIDCClient(ctx, issuerURL, opts...)` — Discover 호출 + issuer 검증 + 메타데이터 캐시

### Priority 2: JWKSCache (jwks_cache.go)

1. `refresh(ctx)` — HTTP GET jwksURI + JSON 파싱 + 캐시 갱신 (mu.Lock)
2. `GetKey(ctx, kid)` — stale-while-revalidate 로직:
   - 캐시 히트 (age < TTL) → 즉시 반환
   - age >= TTL, < staleMaxAge → stale 반환 + go refresh(ctx)
   - age >= staleMaxAge → blocking refresh
   - 에러 + stale 유효 → stale 반환
   - 에러 + 캐시 없음 → ErrJWKSUnavailable

### Stdlib Only

- `net/http` + `encoding/json` 전용
- 외부 HTTP 라이브러리 금지

---

## File Ownership

| File | Owner |
|------|-------|
| `internal/auth/oidc.go` | Sprint 2 |
| `internal/auth/jwks_cache.go` | Sprint 2 |
| `internal/auth/oidc_test.go` | Sprint 2 |
| `internal/auth/jwks_cache_test.go` | Sprint 2 |

---

## Definition of Done

- `TestOIDCClient_Discover_Success` PASS (현재 RED)
- 전체 새 테스트 PASS (14개 이상)
- Sprint 1 회귀 없음 (47 subtests PASS 유지)
- `go test -race ./apps/control-plane/internal/auth/...` PASS
- golangci-lint 0 issue
- `@MX:TODO` 태그 → `@MX:NOTE` 또는 제거 (GREEN 완료 시)
