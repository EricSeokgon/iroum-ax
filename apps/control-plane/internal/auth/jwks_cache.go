// JWKS 캐시 — SPEC-AX-AUTH-001 REQ-AUTH-002 JWKS stale-while-revalidate 구현
// Sprint 2 GREEN: JWKSCache.GetKey / refresh 실제 구현
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────
// CachedKey — 캐시된 JWKS 키 단일 항목
// ────────────────────────────────────────────────────────────

// CachedKey — kid로 인덱싱된 JWKS 공개키 캐시 항목
type CachedKey struct {
	// Key — 실제 공개키 (rsa.PublicKey / ecdsa.PublicKey)
	Key any
	// Alg — 서명 알고리즘 (RS256 / ES256 / EdDSA)
	Alg string
	// Kty — 키 타입 (RSA / EC / OKP)
	Kty string
	// Kid — 키 ID
	Kid string
}

// ────────────────────────────────────────────────────────────
// JWK JSON 구조체 (파싱용)
// ────────────────────────────────────────────────────────────

// jwkJSON — JWK 단일 키 JSON 표현
type jwkJSON struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	// RSA 파라미터
	N string `json:"n,omitempty"`
	E string `json:"e,omitempty"`
	// EC 파라미터
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// jwksJSON — JWKS 응답 JSON 표현
type jwksJSON struct {
	Keys []jwkJSON `json:"keys"`
}

// ────────────────────────────────────────────────────────────
// JWKSCache — JWKSProvider 구현체 (stale-while-revalidate)
// ────────────────────────────────────────────────────────────

// JWKSCache — JWKS 엔드포인트 fetch + 캐시 관리 구현체
//
// 캐시 정책:
//   - cache age < TTL: 캐시 히트, 즉시 반환 (< 1ms)
//   - TTL <= cache age < staleMaxAge: stale 반환 + 백그라운드 refresh
//   - cache age >= staleMaxAge: blocking fetch
//   - fetch 실패 + stale 유효: stale 반환 (degraded mode)
//   - fetch 실패 + 캐시 없음: ErrJWKSUnavailable
//
// @MX:ANCHOR: [AUTO] JWKSProvider 인터페이스의 유일한 실 구현체
// @MX:REASON: TokenValidator, OIDC middleware, 테스트에서 참조 (fan_in >= 3)
// @MX:WARN: [AUTO] 백그라운드 refresh goroutine 포함
// @MX:REASON: goroutine이 context 없이 실행됨 — 단발성이며 패닉 전파 없음, mu 보호 하에 실행
type JWKSCache struct { //nolint:govet // fieldalignment: sync.RWMutex는 이동 불가 (non-copyable), 논리적 그룹 유지
	// httpClient — HTTP 클라이언트
	httpClient *http.Client
	// cache — kid → CachedKey 맵
	cache map[string]CachedKey
	// mu — 캐시 RW 뮤텍스
	mu sync.RWMutex
	// fetchMu — blocking fetch 직렬화 뮤텍스 (동시 fetch 방지)
	fetchMu sync.Mutex
	// lastSuccessAt — 마지막 성공적 fetch 시각
	lastSuccessAt time.Time
	// ttl — 캐시 hard TTL (기본 3600초)
	ttl time.Duration
	// staleMaxAge — stale 캐시 최대 허용 기간 (기본 14400초, 4시간)
	staleMaxAge time.Duration
	// jwksURI — JWKS 엔드포인트 URL
	jwksURI string
	// refreshing — 백그라운드 refresh 중복 방지 플래그
	refreshing bool
}

// JWKSCacheOption — JWKSCache 생성 옵션 함수 타입
type JWKSCacheOption func(*JWKSCache)

// WithCacheTTL — 캐시 TTL 설정 옵션
func WithCacheTTL(d time.Duration) JWKSCacheOption {
	return func(c *JWKSCache) { c.ttl = d }
}

// WithStaleMaxAge — stale 캐시 최대 허용 기간 설정 옵션
func WithStaleMaxAge(d time.Duration) JWKSCacheOption {
	return func(c *JWKSCache) { c.staleMaxAge = d }
}

// WithJWKSHTTPClient — HTTP 클라이언트 주입 옵션
func WithJWKSHTTPClient(c *http.Client) JWKSCacheOption {
	return func(j *JWKSCache) { j.httpClient = c }
}

// NewJWKSCache — JWKSCache 생성자
//
// 지연 초기화(lazy init): 첫 GetKey 호출 시 fetch 수행.
func NewJWKSCache(jwksURI string, opts ...JWKSCacheOption) *JWKSCache {
	c := &JWKSCache{
		jwksURI:     jwksURI,
		cache:       make(map[string]CachedKey),
		ttl:         3600 * time.Second,
		staleMaxAge: 4 * 3600 * time.Second,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetKey — kid로 서명 검증 키를 조회한다. JWKSProvider 인터페이스 구현.
//
// 캐시 히트 시 < 1ms 성능 목표 (AC-AUTH-002-O1, REQ-AUTH-002 NFR).
func (c *JWKSCache) GetKey(ctx context.Context, kid string) (any, string, string, error) {
	if kid == "" {
		return nil, "", "", fmt.Errorf("kid가 비어 있습니다")
	}

	c.mu.RLock()
	cacheAge := c.cacheAge()
	cached, found := c.cache[kid]
	c.mu.RUnlock()

	if found {
		if cacheAge < c.ttl {
			// 캐시 히트 — TTL 내 즉시 반환
			return cached.Key, cached.Alg, cached.Kty, nil
		}

		if cacheAge < c.staleMaxAge {
			// TTL 만료 but staleMaxAge 내 — stale 반환 + 백그라운드 refresh
			c.mu.Lock()
			if !c.refreshing {
				c.refreshing = true
				go func() {
					defer func() {
						c.mu.Lock()
						c.refreshing = false
						c.mu.Unlock()
					}()
					// context.Background()로 백그라운드 refresh (요청 컨텍스트와 독립)
					_ = c.refresh(context.Background()) //nolint:errcheck // 백그라운드 refresh 실패는 무시 (stale 유지)
				}()
			}
			c.mu.Unlock()
			return cached.Key, cached.Alg, cached.Kty, nil
		}
	}

	// 캐시 미스 또는 staleMaxAge 초과 — blocking fetch (직렬화)
	// fetchMu로 동시 fetch를 방지하여 JWKS 서버 부하를 최소화한다.
	c.fetchMu.Lock()
	// Lock 획득 후 캐시가 이미 갱신되었는지 재확인
	c.mu.RLock()
	newAge := c.cacheAge()
	newCached, newFound := c.cache[kid]
	c.mu.RUnlock()

	var fetchErr error
	if newFound && newAge < c.ttl {
		// 다른 고루틴이 이미 refresh 완료 — 재사용
		c.fetchMu.Unlock()
		return newCached.Key, newCached.Alg, newCached.Kty, nil
	}
	fetchErr = c.refresh(ctx)
	c.fetchMu.Unlock()

	if fetchErr != nil {
		if found {
			// staleMaxAge 초과 후 fetch 실패 → 사용 불가 (stale 유효 기간 종료)
			return nil, "", "", fmt.Errorf("%w: fetch 실패 및 stale 만료", ErrJWKSUnavailable)
		}
		// 캐시 없음 + fetch 실패
		return nil, "", "", fmt.Errorf("%w: %v", ErrJWKSUnavailable, fetchErr)
	}

	c.mu.RLock()
	refreshed, ok := c.cache[kid]
	c.mu.RUnlock()

	if !ok {
		return nil, "", "", fmt.Errorf("kid=%q가 JWKS에 존재하지 않습니다", kid)
	}

	return refreshed.Key, refreshed.Alg, refreshed.Kty, nil
}

// cacheAge — 마지막 성공적 fetch 이후 경과 시간을 반환한다.
// 호출자가 mu.RLock을 보유해야 한다.
func (c *JWKSCache) cacheAge() time.Duration {
	if c.lastSuccessAt.IsZero() {
		return c.staleMaxAge + 1 // 초기화 전: 무조건 fetch 유발
	}
	return time.Since(c.lastSuccessAt)
}

// refresh — JWKS 엔드포인트에서 키를 가져와 캐시를 갱신한다.
func (c *JWKSCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURI, nil)
	if err != nil {
		return fmt.Errorf("JWKS 요청 생성 실패: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("JWKS fetch 실패: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS non-200 응답: status=%d", resp.StatusCode)
	}

	var jwks jwksJSON
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("JWKS JSON 파싱 실패: %w", err)
	}

	newCache := make(map[string]CachedKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		parsed, parseErr := parseJWKKey(k)
		if parseErr != nil {
			// 파싱 실패한 개별 키는 건너뜀 (다른 키는 사용 가능)
			continue
		}
		newCache[k.Kid] = CachedKey{
			Key: parsed,
			Alg: k.Alg,
			Kty: k.Kty,
			Kid: k.Kid,
		}
	}

	c.mu.Lock()
	c.cache = newCache
	c.lastSuccessAt = time.Now()
	c.mu.Unlock()

	return nil
}

// parseJWKKey — JWK JSON 항목을 실제 공개키 타입으로 변환한다.
//
// 지원: RSA (n/e 필드), EC (crv/x/y 필드)
// 미지원 kty는 에러 반환 (키 건너뜀)
func parseJWKKey(k jwkJSON) (any, error) {
	switch k.Kty {
	case "RSA":
		return parseRSAKey(k)
	case "EC":
		return parseECKey(k)
	default:
		return nil, fmt.Errorf("지원하지 않는 kty=%s (kid=%s)", k.Kty, k.Kid)
	}
}

// parseRSAKey — JWK RSA 공개키를 *rsa.PublicKey로 변환한다.
func parseRSAKey(k jwkJSON) (*rsa.PublicKey, error) {
	if k.N == "" || k.E == "" {
		// 더미 값이어도 빈 문자열이 아닌 경우만 허용
		return nil, fmt.Errorf("RSA JWK: n 또는 e 필드가 비어 있습니다 (kid=%s)", k.Kid)
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("RSA JWK: n 디코딩 실패 (kid=%s): %w", k.Kid, err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("RSA JWK: e 디코딩 실패 (kid=%s): %w", k.Kid, err)
	}

	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}
	return pub, nil
}

// parseECKey — JWK EC 공개키를 *ecdsa.PublicKey로 변환한다.
func parseECKey(k jwkJSON) (*ecdsa.PublicKey, error) {
	if k.Crv == "" || k.X == "" || k.Y == "" {
		return nil, fmt.Errorf("EC JWK: crv/x/y 필드가 비어 있습니다 (kid=%s)", k.Kid)
	}

	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("EC JWK: 지원하지 않는 crv=%s (kid=%s)", k.Crv, k.Kid)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("EC JWK: x 디코딩 실패 (kid=%s): %w", k.Kid, err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("EC JWK: y 디코딩 실패 (kid=%s): %w", k.Kid, err)
	}

	pub := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
	return pub, nil
}
