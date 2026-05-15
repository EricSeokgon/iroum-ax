// OIDC Discovery 클라이언트 — SPEC-AX-AUTH-001 REQ-AUTH-002 구현
// Sprint 2 GREEN: NewOIDCClient / Discover 실제 구현
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ────────────────────────────────────────────────────────────
// Metadata — OIDC provider 메타데이터 (discovery document)
// ────────────────────────────────────────────────────────────

// Metadata — /.well-known/openid-configuration 파싱 결과
type Metadata struct {
	// Issuer — OIDC provider issuer URL
	Issuer string `json:"issuer"`
	// JWKSUri — JWKS 엔드포인트 URL
	JWKSUri string `json:"jwks_uri"`
	// TokenEndpoint — 토큰 발급 엔드포인트
	TokenEndpoint string `json:"token_endpoint"`
	// AuthorizationEndpoint — 인증 코드 발급 엔드포인트 (선택)
	AuthorizationEndpoint string `json:"authorization_endpoint,omitempty"`
}

// ────────────────────────────────────────────────────────────
// OIDCClient — OIDC Discovery 클라이언트
// ────────────────────────────────────────────────────────────

// OIDCClient — OIDC Discovery 문서를 가져오고 메타데이터를 캐시한다.
//
// @MX:ANCHOR: [AUTO] OIDC Provider 연동 단일 진입점
// @MX:REASON: validator, jwks_cache, main.go 등 다수에서 참조 예정 (fan_in >= 3)
type OIDCClient struct {
	// httpClient — HTTP 클라이언트 (테스트 시 mock 서버 주입)
	httpClient *http.Client
	// metadata — 캐시된 OIDC provider 메타데이터
	metadata *Metadata
	// issuerURL — OIDC provider issuer URL
	issuerURL string
}

// OIDCClientOption — OIDCClient 생성 옵션 함수 타입
type OIDCClientOption func(*OIDCClient)

// WithHTTPClient — http.Client 주입 옵션 (테스트 시 httptest 서버 연동)
func WithHTTPClient(c *http.Client) OIDCClientOption {
	return func(o *OIDCClient) { o.httpClient = c }
}

// oidcDiscoveryTimeout — OIDC Discovery 기본 타임아웃 (AC-AUTH-002-2: 10초 내 fail-fast)
const oidcDiscoveryTimeout = 10 * time.Second

// NewOIDCClient — OIDC Discovery를 수행하고 OIDCClient를 반환한다.
//
// 동작:
//  1. issuerURL + "/.well-known/openid-configuration" 요청
//  2. 10초 타임아웃 기본값 (WithHTTPClient로 재정의 가능)
//  3. non-200 응답 → 에러 반환 (AC-AUTH-002-2 fail-fast)
//  4. issuer 필드 불일치 → 에러 반환 (AC-AUTH-002-3 security check)
//  5. 메타데이터 메모리 캐시 저장
//
// @MX:ANCHOR: [AUTO] OIDC 클라이언트 팩토리 — 애플리케이션 시작점
// @MX:REASON: main, validator 초기화, 인증 미들웨어에서 호출 (fan_in >= 3)
func NewOIDCClient(ctx context.Context, issuerURL string, opts ...OIDCClientOption) (*OIDCClient, error) {
	if issuerURL == "" {
		return nil, fmt.Errorf("OIDC issuer URL이 비어 있습니다")
	}

	c := &OIDCClient{
		issuerURL: issuerURL,
		httpClient: &http.Client{
			Timeout: oidcDiscoveryTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	meta, err := Discover(ctx, issuerURL, c.httpClient)
	if err != nil {
		return nil, err
	}

	// AC-AUTH-002-3: issuer 불일치 보안 검증
	if meta.Issuer != issuerURL {
		return nil, fmt.Errorf("discovery issuer mismatch: 기대=%s, 실제=%s", issuerURL, meta.Issuer)
	}

	c.metadata = meta
	return c, nil
}

// Discover — issuerURL에서 OIDC Discovery 문서를 가져온다.
//
// URL: issuerURL + "/.well-known/openid-configuration"
// 반환: *Metadata (성공), error (non-200 응답, 파싱 실패, 필수 필드 누락)
func Discover(ctx context.Context, issuerURL string, client *http.Client) (*Metadata, error) {
	discoveryURL := issuerURL + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("OIDC Discovery 요청 생성 실패: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OIDC Discovery 요청 실패: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // 응답 바디 닫기 실패는 무시

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC Discovery non-200 응답: status=%d", resp.StatusCode)
	}

	var meta Metadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("OIDC Discovery JSON 파싱 실패: %w", err)
	}

	// 필수 필드 검증
	if meta.Issuer == "" {
		return nil, fmt.Errorf("OIDC Discovery: issuer 필드가 비어 있습니다")
	}
	if meta.JWKSUri == "" {
		return nil, fmt.Errorf("OIDC Discovery: jwks_uri 필드가 비어 있습니다")
	}

	return &meta, nil
}

// GetMetadata — 캐시된 OIDC 메타데이터를 반환한다.
func (c *OIDCClient) GetMetadata() *Metadata {
	return c.metadata
}
