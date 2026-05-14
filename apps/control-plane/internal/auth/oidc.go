// OIDC Discovery 클라이언트 — SPEC-AX-AUTH-001 REQ-AUTH-002 구현
// Sprint 2 RED: stub — 메서드는 errors.New("not implemented") 반환
package auth

import (
	"context"
	"errors"
	"net/http"
)

// ────────────────────────────────────────────────────────────
// Metadata — OIDC provider 메타데이터 (discovery document)
// ────────────────────────────────────────────────────────────

// Metadata — /.well-known/openid-configuration 파싱 결과
//
// @MX:TODO: [AUTO] Sprint 2 GREEN에서 실제 JSON 파싱 구현 필요
type Metadata struct {
	// Issuer — OIDC provider issuer URL
	Issuer string `json:"issuer"`
	// JWKSUri — JWKS 엔드포인트 URL
	JWKSUri string `json:"jwks_uri"`
	// TokenEndpoint — 토큰 발급 엔드포인트
	TokenEndpoint string `json:"token_endpoint"`
	// AuthorizationEndpoint — 인증 코드 발급 엔드포인트
	AuthorizationEndpoint string `json:"authorization_endpoint"`
}

// ────────────────────────────────────────────────────────────
// OIDCClient — OIDC Discovery 클라이언트
// ────────────────────────────────────────────────────────────

// OIDCClient — OIDC Discovery 문서를 가져오고 메타데이터를 캐시한다.
//
// @MX:ANCHOR: [AUTO] OIDC Provider 연동 단일 진입점
// @MX:REASON: validator, jwks_cache, main.go 등 다수에서 참조 예정 (fan_in >= 3)
// @MX:TODO: [AUTO] Sprint 2 GREEN — NewOIDCClient 실제 구현 필요
//
//nolint:govet // fieldalignment: Sprint 2 GREEN에서 최적화 예정 (stub 단계)
type OIDCClient struct {
	// metadata — 캐시된 OIDC provider 메타데이터
	metadata *Metadata
	// httpClient — HTTP 클라이언트 (테스트 시 mock 서버 주입)
	httpClient *http.Client
	// issuerURL — OIDC provider issuer URL
	issuerURL string //nolint:unused // Sprint 2 GREEN에서 사용 예정
}

// OIDCClientOption — OIDCClient 생성 옵션 함수 타입
type OIDCClientOption func(*OIDCClient)

// WithHTTPClient — http.Client 주입 옵션 (테스트 시 httptest 서버 연동)
func WithHTTPClient(c *http.Client) OIDCClientOption {
	return func(o *OIDCClient) { o.httpClient = c }
}

// oidcDiscoveryTimeout — OIDC Discovery 기본 타임아웃 (AC-AUTH-002-2: 10초 내 fail-fast)
// Sprint 2 GREEN에서 NewOIDCClient 구현 시 사용
const oidcDiscoveryTimeout = "10s" //nolint:unused // Sprint 2 GREEN에서 사용 예정

// NewOIDCClient — OIDC Discovery를 수행하고 OIDCClient를 반환한다.
//
// 동작:
//  1. issuerURL + "/.well-known/openid-configuration" 요청
//  2. 10초 타임아웃 — 초과 시 panic (AC-AUTH-002-2 fail-fast invariant)
//  3. non-200 응답 → panic (AC-AUTH-002-2 fail-fast)
//  4. issuer 필드 불일치 → 에러 반환 (AC-AUTH-002-3 security check)
//  5. 메타데이터 메모리 캐시 저장
//
// @MX:TODO: [AUTO] Sprint 2 GREEN — 실제 HTTP fetch + JSON 파싱 구현
func NewOIDCClient(_ context.Context, _ string, _ ...OIDCClientOption) (*OIDCClient, error) {
	return nil, errors.New("not implemented")
}

// Discover — issuerURL에서 OIDC Discovery 문서를 가져온다.
//
// @MX:TODO: [AUTO] Sprint 2 GREEN — 실제 HTTP fetch 구현
func Discover(_ context.Context, _ string, _ *http.Client) (*Metadata, error) {
	return nil, errors.New("not implemented")
}

// GetMetadata — 캐시된 OIDC 메타데이터를 반환한다.
func (c *OIDCClient) GetMetadata() *Metadata {
	return c.metadata
}
