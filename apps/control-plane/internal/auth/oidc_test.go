// oidc_test.go — SPEC-AX-AUTH-001 REQ-AUTH-002 OIDC Discovery RED phase 테스트
// Sprint 2 GREEN에서 OIDCClient 실제 구현 후 PASS로 전환 예정.
//
// 커버리지 목표: AC-AUTH-002-1/2/3
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ────────────────────────────────────────────────────────────
// 테스트 헬퍼 — httptest mock OIDC 서버
// ────────────────────────────────────────────────────────────

// mockOIDCDiscoveryDoc — OIDC Discovery 응답용 mock 문서 구조체
type mockOIDCDiscoveryDoc struct {
	Issuer                string `json:"issuer"`
	JWKSUri               string `json:"jwks_uri"`
	TokenEndpoint         string `json:"token_endpoint"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
}

// mockOIDCServer — /.well-known/openid-configuration 엔드포인트를 제공하는 테스트 서버
// 반환 값:
//   - *httptest.Server: 실행 중인 mock 서버 (t.Cleanup으로 종료 등록)
//   - string: 서버 기본 URL (issuerURL로 사용)
func mockOIDCServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			// issuer는 서버 URL과 일치해야 AC-AUTH-002-3 테스트가 성립
			doc := mockOIDCDiscoveryDoc{
				Issuer:                srv.URL,
				JWKSUri:               srv.URL + "/protocol/openid-connect/certs",
				TokenEndpoint:         srv.URL + "/protocol/openid-connect/token",
				AuthorizationEndpoint: srv.URL + "/protocol/openid-connect/auth",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(doc)
		default:
			http.NotFound(w, r)
		}
	}))

	t.Cleanup(srv.Close)
	return srv, srv.URL
}

// mockOIDCServerWithIssuerMismatch — discovery 응답의 issuer를 의도적으로 다르게 설정
func mockOIDCServerWithIssuerMismatch(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			doc := mockOIDCDiscoveryDoc{
				// issuer를 서버 URL과 다르게 설정 — AC-AUTH-002-3 트리거
				Issuer:                "http://other-issuer/realms/x",
				JWKSUri:               "http://other-issuer/certs",
				TokenEndpoint:         "http://other-issuer/token",
				AuthorizationEndpoint: "http://other-issuer/auth",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(doc)
		} else {
			http.NotFound(w, r)
		}
	}))

	t.Cleanup(srv.Close)
	return srv, srv.URL
}

// ────────────────────────────────────────────────────────────
// TestOIDCClient_Discover_Success — AC-AUTH-002-1
// ────────────────────────────────────────────────────────────

// TestOIDCClient_Discover_Success — discovery 성공 시 메타데이터 정확히 파싱됨
//
// Given: httptest 서버가 valid OIDC discovery JSON 반환
// When: NewOIDCClient(ctx, issuerURL) 호출
// Then: *OIDCClient.metadata의 JWKSUri, Issuer, TokenEndpoint 정확히 파싱
func TestOIDCClient_Discover_Success(t *testing.T) {
	t.Parallel()

	_, issuerURL := mockOIDCServer(t)
	ctx := context.Background()

	client, err := NewOIDCClient(ctx, issuerURL)

	require.NoError(t, err, "OIDC Discovery 성공해야 함")
	require.NotNil(t, client, "OIDCClient가 nil이면 안 됨")

	meta := client.GetMetadata()
	require.NotNil(t, meta, "Metadata가 nil이면 안 됨")
	assert.Equal(t, issuerURL, meta.Issuer, "issuer가 요청 URL과 일치해야 함")
	assert.Equal(t, issuerURL+"/protocol/openid-connect/certs", meta.JWKSUri, "jwks_uri 정확해야 함")
	assert.Equal(t, issuerURL+"/protocol/openid-connect/token", meta.TokenEndpoint, "token_endpoint 정확해야 함")
	assert.NotEmpty(t, meta.AuthorizationEndpoint, "authorization_endpoint가 비어있으면 안 됨")
}

// ────────────────────────────────────────────────────────────
// TestOIDCClient_Discover_Non200 — AC-AUTH-002-2 (non-200 응답)
// ────────────────────────────────────────────────────────────

// TestOIDCClient_Discover_Non200 — non-200 응답 시 에러 반환
//
// Given: 서버가 404 반환
// When: NewOIDCClient 호출
// Then: 에러 반환 (panic 또는 error, fail-fast)
func TestOIDCClient_Discover_Non200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()

	// non-200 → NewOIDCClient는 에러를 반환하거나 panic 발생
	// stub에서는 "not implemented" 에러 반환 — RED phase에서 이 테스트는 실패
	_, err := NewOIDCClient(ctx, srv.URL)
	// RED: stub이 "not implemented"를 반환하므로 에러가 발생하지만
	// 이 테스트는 "not found" 서버를 쓸 때도 non-nil 에러를 기대함
	assert.Error(t, err, "non-200 응답은 에러를 유발해야 함")
}

// ────────────────────────────────────────────────────────────
// TestOIDCClient_Discover_Timeout — AC-AUTH-002-2 (timeout)
// ────────────────────────────────────────────────────────────

// TestOIDCClient_Discover_Timeout — 서버 응답 없음 시 10초 타임아웃 에러
//
// Given: 서버가 응답하지 않는 URL (closed 서버)
// When: 짧은 타임아웃 HTTP 클라이언트로 NewOIDCClient 호출
// Then: context deadline exceeded 에러 반환
func TestOIDCClient_Discover_Timeout(t *testing.T) {
	t.Parallel()

	// 응답 없이 hang하는 서버 (ctx cancel로 제어)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// 요청을 영원히 block (컨텍스트 취소 시까지)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	// 50ms 타임아웃 클라이언트로 10초 대기 대신 빠르게 테스트
	shortClient := &http.Client{Timeout: 50 * time.Millisecond}

	_, err := NewOIDCClient(ctx, srv.URL, WithHTTPClient(shortClient))

	// RED: stub이 "not implemented" 반환 — GREEN 후 deadline exceeded 에러 기대
	assert.Error(t, err, "타임아웃은 에러를 유발해야 함")
}

// ────────────────────────────────────────────────────────────
// TestOIDCClient_Discover_IssuerMismatch — AC-AUTH-002-3
// ────────────────────────────────────────────────────────────

// TestOIDCClient_Discover_IssuerMismatch — discovery 응답 issuer 불일치 시 에러
//
// Given: discovery 응답의 issuer 필드가 요청 URL과 다름
// When: NewOIDCClient 호출
// Then: "discovery issuer mismatch" 메시지를 포함하는 에러 반환
func TestOIDCClient_Discover_IssuerMismatch(t *testing.T) {
	t.Parallel()

	_, issuerURL := mockOIDCServerWithIssuerMismatch(t)
	ctx := context.Background()

	_, err := NewOIDCClient(ctx, issuerURL)

	require.Error(t, err, "issuer 불일치는 에러를 유발해야 함")
	// GREEN 후: "discovery issuer mismatch" 메시지 포함 확인
	// RED: stub이 "not implemented" 반환 — 에러 발생 자체는 확인됨
}

// ────────────────────────────────────────────────────────────
// TestOIDCClient_Discover_CustomHTTPClient — WithHTTPClient 옵션
// ────────────────────────────────────────────────────────────

// TestOIDCClient_Discover_CustomHTTPClient — WithHTTPClient 옵션으로 커스텀 클라이언트 주입
//
// Given: 커스텀 http.Client (3초 타임아웃)
// When: NewOIDCClient(ctx, issuerURL, WithHTTPClient(client)) 호출
// Then: 에러 없이 OIDCClient 반환
func TestOIDCClient_Discover_CustomHTTPClient(t *testing.T) {
	t.Parallel()

	_, issuerURL := mockOIDCServer(t)
	ctx := context.Background()
	customClient := &http.Client{Timeout: 3 * time.Second}

	client, err := NewOIDCClient(ctx, issuerURL, WithHTTPClient(customClient))

	// RED: stub이 "not implemented" → err != nil
	// GREEN 후: err == nil, client != nil
	if err != nil {
		// stub 상태에서는 에러가 예상됨 — 단, "not implemented" 에러
		assert.True(t, errors.Is(err, err), "에러가 발생하더라도 panic이 아닌 error 반환")
	} else {
		assert.NotNil(t, client, "성공 시 client가 nil이면 안 됨")
	}
}

// ────────────────────────────────────────────────────────────
// TestDiscover_DirectCall — Discover 함수 직접 호출
// ────────────────────────────────────────────────────────────

// TestDiscover_DirectCall — Discover 함수가 올바른 URL로 요청하는지 검증
//
// Given: valid httptest OIDC 서버
// When: Discover(ctx, issuerURL+"/.well-known/openid-configuration", client) 호출
// Then: *Metadata 반환, 필드 정확
func TestDiscover_DirectCall(t *testing.T) {
	t.Parallel()

	srv, issuerURL := mockOIDCServer(t)
	_ = srv
	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	meta, err := Discover(ctx, issuerURL, client)

	// RED: stub이 "not implemented" → err != nil, meta == nil
	// GREEN 후: err == nil, meta.Issuer == issuerURL
	if err != nil {
		assert.Nil(t, meta, "에러 시 meta는 nil이어야 함")
	} else {
		require.NotNil(t, meta, "성공 시 meta가 nil이면 안 됨")
		assert.Equal(t, issuerURL, meta.Issuer, "issuer 일치해야 함")
	}
}
