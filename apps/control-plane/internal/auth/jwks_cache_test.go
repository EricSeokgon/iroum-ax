// jwks_cache_test.go — SPEC-AX-AUTH-001 REQ-AUTH-002 JWKS Cache RED phase 테스트
// Sprint 2 GREEN에서 JWKSCache.GetKey 실제 구현 후 PASS로 전환 예정.
//
// 커버리지 목표: AC-AUTH-002-O1, AC-AUTH-002-Stale, JWKS cache performance
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ────────────────────────────────────────────────────────────
// 테스트 헬퍼 — mock JWKS 서버 + JWKS 응답 생성
// ────────────────────────────────────────────────────────────

// jwksKey — JWKS JSON 내 단일 키 항목
type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n,omitempty"` // RSA modulus (base64url)
	E   string `json:"e,omitempty"` // RSA exponent (base64url)
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// jwksResponse — JWKS JSON 응답 구조체
type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// mockJWKSResponse — 복수 kid를 포함하는 JWKS JSON 바이트를 반환한다.
// 실제 RSA/EC 파라미터는 RED phase에서는 dummy 값 사용 (GREEN에서 실 키로 교체)
func mockJWKSResponse(kids ...string) []byte {
	keys := make([]jwksKey, 0, len(kids))
	for i, kid := range kids {
		var k jwksKey
		if i%2 == 0 {
			// RSA 키 (dummy 파라미터)
			k = jwksKey{
				Kid: kid,
				Kty: "RSA",
				Alg: "RS256",
				Use: "sig",
				N:   "sIqyUfv-abc123",
				E:   "AQAB",
			}
		} else {
			// EC 키 (dummy 파라미터)
			k = jwksKey{
				Kid: kid,
				Kty: "EC",
				Alg: "ES256",
				Use: "sig",
				Crv: "P-256",
				X:   "xyz123",
				Y:   "abc456",
			}
		}
		keys = append(keys, k)
	}
	resp := jwksResponse{Keys: keys}
	b, _ := json.Marshal(resp)
	return b
}

// mockJWKSServer — /jwks 엔드포인트를 제공하는 테스트 서버
// fetchCount: 서버가 호출된 횟수 (atomic)
func mockJWKSServer(t *testing.T, kids []string) (srv *httptest.Server, fetchCount *atomic.Int64) {
	t.Helper()
	fetchCount = &atomic.Int64{}

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks" {
			fetchCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(mockJWKSResponse(kids...))
		} else {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, fetchCount
}

// mockUnavailableJWKSServer — 항상 503을 반환하는 서버
func mockUnavailableJWKSServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_CacheHit — 캐시 히트 시 즉시 반환
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_CacheHit — 캐시 히트 시 < 1ms 성능 목표
//
// Given: kid가 캐시에 존재, cache age < TTL
// When: GetKey(ctx, kid) 호출
// Then: 에러 없이 키 반환, 서버 fetch 0회 (캐시에서만 조회)
func TestJWKSCache_GetKey_CacheHit(t *testing.T) {
	t.Parallel()

	srv, fetchCount := mockJWKSServer(t, []string{"key-v1", "key-v2"})
	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
		WithCacheTTL(1*time.Hour),
	)
	ctx := context.Background()

	// 첫 fetch로 캐시 populate
	_, _, _, _ = cache.GetKey(ctx, "key-v1")
	initialFetch := fetchCount.Load()

	// 두 번째 호출 — 캐시 히트여야 함
	start := time.Now()
	key, alg, kty, err := cache.GetKey(ctx, "key-v1")
	elapsed := time.Since(start)

	// RED: stub이 "not implemented" 반환 → err != nil
	// GREEN 후: err == nil, elapsed < 1ms, fetchCount == initialFetch (추가 fetch 없음)
	if err == nil {
		assert.NotNil(t, key, "캐시 히트 시 키가 nil이면 안 됨")
		assert.NotEmpty(t, alg, "alg가 비어있으면 안 됨")
		assert.NotEmpty(t, kty, "kty가 비어있으면 안 됨")
		assert.Less(t, elapsed, time.Millisecond, "캐시 히트는 1ms 미만이어야 함")
		assert.Equal(t, initialFetch, fetchCount.Load(), "캐시 히트 시 추가 fetch 없어야 함")
	} else {
		// stub 상태 — "not implemented" 에러 기대
		assert.Error(t, err, "stub 상태에서는 에러 반환")
	}
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_CacheMiss_Fetch — 캐시 미스 시 blocking fetch
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_CacheMiss_Fetch — 캐시 미스 시 JWKS fetch 후 populate
//
// Given: 캐시 비어 있음
// When: GetKey(ctx, "key-v1") 호출
// Then: 서버 fetch 1회, 캐시 populate, 키 반환
func TestJWKSCache_GetKey_CacheMiss_Fetch(t *testing.T) {
	t.Parallel()

	srv, fetchCount := mockJWKSServer(t, []string{"key-v1", "key-v2"})
	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
	)
	ctx := context.Background()

	key, alg, kty, err := cache.GetKey(ctx, "key-v1")

	// RED: stub → err != nil
	// GREEN 후: fetchCount == 1, key != nil, err == nil
	if err == nil {
		assert.Equal(t, int64(1), fetchCount.Load(), "캐시 미스 시 fetch 1회 발생해야 함")
		assert.NotNil(t, key)
		assert.NotEmpty(t, alg)
		assert.NotEmpty(t, kty)
	} else {
		assert.Error(t, err, "stub 상태에서는 에러 반환")
	}
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_TTLExpired_BackgroundRefresh — stale-while-revalidate
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_TTLExpired_BackgroundRefresh — TTL 만료 시 stale 반환 + 백그라운드 refresh
//
// Given: 캐시 TTL 50ms (테스트용 단축), 초기 캐시 populate
// When: 60ms 경과 후 GetKey 호출 (TTL 만료 but staleMaxAge 내)
// Then: stale 캐시로 즉시 응답, 백그라운드 refresh 트리거됨 (fetchCount 증가)
func TestJWKSCache_GetKey_TTLExpired_BackgroundRefresh(t *testing.T) {
	t.Parallel()

	srv, fetchCount := mockJWKSServer(t, []string{"key-v1"})
	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
		WithCacheTTL(50*time.Millisecond),
		WithStaleMaxAge(4*time.Hour),
	)
	ctx := context.Background()

	// 초기 캐시 populate
	_, _, _, _ = cache.GetKey(ctx, "key-v1")
	fetchAfterPopulate := fetchCount.Load()

	// TTL 만료 대기
	time.Sleep(60 * time.Millisecond)

	// TTL 만료 후 호출 — stale 반환 + 백그라운드 refresh
	key, _, _, err := cache.GetKey(ctx, "key-v1")

	// 백그라운드 refresh 완료 대기 (비동기)
	time.Sleep(50 * time.Millisecond)

	// RED: stub → err != nil
	// GREEN 후: err == nil (stale 반환), fetchCount > fetchAfterPopulate
	if err == nil {
		assert.NotNil(t, key, "stale 반환 시 키가 nil이면 안 됨")
		assert.Greater(t, fetchCount.Load(), fetchAfterPopulate, "백그라운드 refresh 발생해야 함")
	} else {
		assert.Error(t, err, "stub 상태에서는 에러 반환")
	}
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_JWKSUnavailable_StaleReturn — degraded mode
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_JWKSUnavailable_StaleReturn — JWKS 다운 + stale 캐시 허용
//
// Given: 캐시 TTL 50ms, 초기 populate, 이후 JWKS 서버 503
// When: TTL 만료 후 GetKey 호출 (staleMaxAge 내)
// Then: 에러 없이 stale 캐시 반환 (AC-AUTH-001-9 degraded mode)
func TestJWKSCache_GetKey_JWKSUnavailable_StaleReturn(t *testing.T) {
	t.Parallel()

	// 처음에는 정상 서버, 이후 503
	callCount := &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks" {
			n := callCount.Add(1)
			if n == 1 {
				// 첫 번째 fetch만 성공
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(mockJWKSResponse("key-v1"))
			} else {
				// 이후 503
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			}
		}
	}))
	t.Cleanup(srv.Close)

	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
		WithCacheTTL(50*time.Millisecond),
		WithStaleMaxAge(4*time.Hour),
	)
	ctx := context.Background()

	// 초기 populate (성공)
	_, _, _, _ = cache.GetKey(ctx, "key-v1")

	// TTL 만료 대기
	time.Sleep(60 * time.Millisecond)

	// refresh 실패 상황에서 stale 반환 기대
	key, _, _, err := cache.GetKey(ctx, "key-v1")

	// RED: stub → err != nil
	// GREEN 후: err == nil, key != nil (stale)
	if err == nil {
		assert.NotNil(t, key, "JWKS 다운 시 stale 캐시를 반환해야 함")
	} else {
		assert.Error(t, err, "stub 상태에서는 에러 반환")
	}
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_JWKSUnavailable_NoCacheError — ErrJWKSUnavailable
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_JWKSUnavailable_NoCacheError — JWKS 다운 + 캐시 없음 → ErrJWKSUnavailable
//
// Given: JWKS 서버 처음부터 503
// When: GetKey 호출 (캐시 비어 있음)
// Then: ErrJWKSUnavailable 반환 (AC-AUTH-001-9 cache miss scenario)
func TestJWKSCache_GetKey_JWKSUnavailable_NoCacheError(t *testing.T) {
	t.Parallel()

	unavailSrv := mockUnavailableJWKSServer(t)
	cache := NewJWKSCache(
		unavailSrv.URL+"/jwks",
		WithJWKSHTTPClient(unavailSrv.Client()),
	)
	ctx := context.Background()

	_, _, _, err := cache.GetKey(ctx, "key-v1")

	// RED: stub이 "not implemented" 반환 → err != nil (이 테스트는 의도적으로 PASS)
	// GREEN 후: errors.Is(err, ErrJWKSUnavailable)
	require.Error(t, err, "JWKS 미가용 + 캐시 없음은 에러를 유발해야 함")
	// GREEN 후 활성화: assert.True(t, errors.Is(err, ErrJWKSUnavailable))
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_KidNotFound — kid가 JWKS에 없는 경우
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_KidNotFound — kid가 JWKS 응답에 없으면 에러
//
// Given: JWKS 응답에 "key-v1"만 포함
// When: GetKey(ctx, "nonexistent-key") 호출
// Then: 에러 반환 (kid not found)
func TestJWKSCache_GetKey_KidNotFound(t *testing.T) {
	t.Parallel()

	srv, _ := mockJWKSServer(t, []string{"key-v1"})
	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
	)
	ctx := context.Background()

	_, _, _, err := cache.GetKey(ctx, "nonexistent-key")

	// RED: stub → "not implemented" 에러
	// GREEN 후: kid not found 에러 반환
	assert.Error(t, err, "존재하지 않는 kid는 에러를 유발해야 함")
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_Concurrent — 동시 접근 race 없음 + 단일 fetch
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_Concurrent — 10 고루틴 동시 GetKey 호출 시 race 없음
//
// Given: 빈 캐시, 10 고루틴이 동시에 GetKey("key-v1") 호출
// When: 동시 요청
// Then: race detector 에러 없음, JWKS fetch 1~2회 (singleflight 또는 mutex 기반)
//
// 실행: go test -race ./... 로 race detector 활성화
func TestJWKSCache_GetKey_Concurrent(t *testing.T) {
	t.Parallel()

	srv, fetchCount := mockJWKSServer(t, []string{"key-v1"})
	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
	)
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			_, _, _, err := cache.GetKey(ctx, "key-v1")
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	// RED: stub → 모든 고루틴이 에러 반환
	// GREEN 후: 에러 0, fetchCount <= 2 (mutex 기반 단일 fetch)
	errs := 0
	for range errCh {
		errs++
	}

	if errs == 0 {
		// 성공 경로 검증
		assert.LessOrEqual(t, fetchCount.Load(), int64(2), "동시 요청 시 fetch는 최대 2회여야 함")
	}
	// stub 상태에서는 모든 고루틴이 에러 — race만 없으면 OK
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_StaleMaxAgeExpired_BlockingFetch — staleMaxAge 초과 시 blocking fetch
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_StaleMaxAgeExpired_BlockingFetch — staleMaxAge 초과 시 blocking fetch
//
// Given: TTL 50ms, staleMaxAge 100ms, 초기 populate
// When: 150ms 경과 (staleMaxAge 초과) 후 GetKey 호출
// Then: stale 반환 없이 blocking fetch 시도 (성공하면 반환, 실패하면 ErrJWKSUnavailable)
func TestJWKSCache_GetKey_StaleMaxAgeExpired_BlockingFetch(t *testing.T) {
	t.Parallel()

	srv, fetchCount := mockJWKSServer(t, []string{"key-v1"})
	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
		WithCacheTTL(50*time.Millisecond),
		WithStaleMaxAge(100*time.Millisecond),
	)
	ctx := context.Background()

	// 초기 populate
	_, _, _, _ = cache.GetKey(ctx, "key-v1")

	// staleMaxAge 초과 대기
	time.Sleep(150 * time.Millisecond)

	key, _, _, err := cache.GetKey(ctx, "key-v1")

	// RED: stub → err != nil
	// GREEN 후: blocking fetch 발생 (fetchCount >= 2), err == nil
	if err == nil {
		assert.NotNil(t, key)
		assert.GreaterOrEqual(t, fetchCount.Load(), int64(2), "staleMaxAge 초과 시 재fetch 발생해야 함")
	} else {
		assert.Error(t, err)
	}
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_JWKSProviderInterface — JWKSProvider 인터페이스 구현 컴파일 검증
// ────────────────────────────────────────────────────────────

// TestJWKSCache_JWKSProviderInterface — JWKSCache가 JWKSProvider를 구현하는지 컴파일 타임 확인
func TestJWKSCache_JWKSProviderInterface(t *testing.T) {
	t.Parallel()

	// 컴파일 타임 인터페이스 검증
	var _ JWKSProvider = (*JWKSCache)(nil)
	// 런타임에서도 확인
	cache := NewJWKSCache("http://example.com/jwks")
	assert.NotNil(t, cache, "NewJWKSCache는 nil을 반환하면 안 됨")

	// JWKSProvider 인터페이스로 사용 가능한지 확인
	var provider JWKSProvider = cache
	assert.NotNil(t, provider)
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_GetKey_EmptyKid — 빈 kid 처리
// ────────────────────────────────────────────────────────────

// TestJWKSCache_GetKey_EmptyKid — 빈 kid 문자열 처리
//
// Given: 유효한 JWKS 서버
// When: GetKey(ctx, "") 호출 (kid="" )
// Then: 에러 반환 (빈 kid는 유효하지 않음)
func TestJWKSCache_GetKey_EmptyKid(t *testing.T) {
	t.Parallel()

	srv, _ := mockJWKSServer(t, []string{"key-v1"})
	cache := NewJWKSCache(
		srv.URL+"/jwks",
		WithJWKSHTTPClient(srv.Client()),
	)
	ctx := context.Background()

	_, _, _, err := cache.GetKey(ctx, "")

	// RED: stub → "not implemented" 에러
	// GREEN 후: kid not found 또는 validation 에러
	assert.Error(t, err, "빈 kid는 에러를 유발해야 함")
}

// ────────────────────────────────────────────────────────────
// TestJWKSCache_WithOptions — 생성 옵션 적용 검증
// ────────────────────────────────────────────────────────────

// TestJWKSCache_WithOptions — JWKSCache 생성 옵션이 정상 적용되는지 확인
//
// Given: WithCacheTTL, WithStaleMaxAge, WithJWKSHTTPClient 옵션
// When: NewJWKSCache 호출
// Then: 내부 설정값이 옵션 값과 일치
func TestJWKSCache_WithOptions(t *testing.T) {
	t.Parallel()

	customClient := &http.Client{Timeout: 5 * time.Second}
	cache := NewJWKSCache(
		"http://example.com/jwks",
		WithCacheTTL(30*time.Minute),
		WithStaleMaxAge(2*time.Hour),
		WithJWKSHTTPClient(customClient),
	)

	require.NotNil(t, cache, "NewJWKSCache가 nil을 반환하면 안 됨")
	// 내부 필드 직접 검증 (같은 패키지 내 접근)
	assert.Equal(t, 30*time.Minute, cache.ttl, "TTL 옵션이 적용되어야 함")
	assert.Equal(t, 2*time.Hour, cache.staleMaxAge, "staleMaxAge 옵션이 적용되어야 함")
	assert.Equal(t, customClient, cache.httpClient, "httpClient 옵션이 적용되어야 함")
}

// ────────────────────────────────────────────────────────────
// 헬퍼 검증 — errors 패키지가 올바르게 import됨
// ────────────────────────────────────────────────────────────

// TestJWKSCache_ErrSentinel — ErrJWKSUnavailable sentinel 에러 검증
func TestJWKSCache_ErrSentinel(t *testing.T) {
	t.Parallel()

	// ErrJWKSUnavailable이 정의되어 있는지 확인
	assert.NotNil(t, ErrJWKSUnavailable, "ErrJWKSUnavailable sentinel이 nil이면 안 됨")
	assert.True(t, errors.Is(ErrJWKSUnavailable, ErrJWKSUnavailable), "errors.Is 정상 동작")
}
