// validator_test.go — SPEC-AX-AUTH-001 REQ-AUTH-001 RED phase 테스트
// Sprint 1 GREEN에서 TokenValidator.Verify 구현 후 모두 PASS로 전환 예정.
//
// 커버리지 목표: AC-AUTH-001-1~10 + AC-AUTH-001-iss-validation (SF-1) +
//
//	AC-AUTH-001-alg-cross-check (SF-2) + AC-AUTH-001-Performance
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ────────────────────────────────────────────────────────────
// 테스트 헬퍼 — 인라인 JWKS mock + JWT 생성
// ────────────────────────────────────────────────────────────

const (
	// testIssuer — 테스트용 OIDC issuer URL
	testIssuer = "https://keycloak.iroum-ax.internal/realms/iroum-ax"
	// testAudience — 테스트용 audience
	testAudience = "iroum-ax-control-plane"
	// testKIDRSA — RSA 테스트 키 ID
	testKIDRSA = "rsa-key-1"
	// testKIDEC — EC 테스트 키 ID
	testKIDEC = "ec-key-1"
)

// testKeys — 테스트용 키 쌍 (패키지 초기화 시 1회 생성)
var testKeys struct {
	rsaPriv *rsa.PrivateKey
	ecPriv  *ecdsa.PrivateKey
}

func init() {
	var err error
	testKeys.rsaPriv, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("RSA 키 생성 실패: " + err.Error())
	}
	testKeys.ecPriv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic("EC 키 생성 실패: " + err.Error())
	}
}

// jwtOpts — genTestJWT에 전달하는 토큰 옵션
type jwtOpts struct {
	// 알고리즘: "RS256", "ES256", "HS256", "none"
	alg string
	// kid 헤더 값
	kid string
	// 서명 키 — nil이면 alg에 따라 기본 키 사용
	signingKey any
	// 클레임
	sub      string
	issuer   string
	audience []string
	jti      string
	scopes   []string
	// 시간 제어
	expOffset time.Duration // now + offset
	iatOffset time.Duration // now + offset
	nbfOffset time.Duration // now + offset
}

// defaultJWTOpts — 유효한 기본 옵션
func defaultJWTOpts() jwtOpts {
	return jwtOpts{
		alg:       "RS256",
		kid:       testKIDRSA,
		sub:       "user-sub-001",
		issuer:    testIssuer,
		audience:  []string{testAudience},
		jti:       "jti-default",
		scopes:    []string{"iroum-ax:analyst"},
		expOffset: +3600 * time.Second,
		iatOffset: -10 * time.Second,
		nbfOffset: -60 * time.Second,
	}
}

// genTestJWT — 옵션에 따라 서명된 JWT 문자열을 반환한다.
func genTestJWT(t *testing.T, opts jwtOpts) string {
	t.Helper()

	now := time.Now()

	claims := jwt.MapClaims{
		"sub": opts.sub,
		"iss": opts.issuer,
		"aud": opts.audience,
		"jti": opts.jti,
		"scp": strings.Join(opts.scopes, " "),
		"exp": now.Add(opts.expOffset).Unix(),
		"iat": now.Add(opts.iatOffset).Unix(),
		"nbf": now.Add(opts.nbfOffset).Unix(),
	}

	var method jwt.SigningMethod
	var key any

	switch opts.alg {
	case "RS256":
		method = jwt.SigningMethodRS256
		if opts.signingKey != nil {
			key = opts.signingKey
		} else {
			key = testKeys.rsaPriv
		}
	case "ES256":
		method = jwt.SigningMethodES256
		if opts.signingKey != nil {
			key = opts.signingKey
		} else {
			key = testKeys.ecPriv
		}
	case "HS256":
		method = jwt.SigningMethodHS256
		key = []byte("weak-secret")
	case "none":
		// alg=none: 서명 없이 수동으로 토큰 생성
		return buildUnsignedJWT(t, opts.kid, claims)
	default:
		t.Fatalf("지원하지 않는 alg: %s", opts.alg)
		return ""
	}

	token := jwt.NewWithClaims(method, claims)
	if opts.kid != "" {
		token.Header["kid"] = opts.kid
	}

	signed, err := token.SignedString(key)
	require.NoError(t, err, "JWT 서명 실패")
	return signed
}

// buildUnsignedJWT — alg=none 토큰을 수동으로 생성한다.
func buildUnsignedJWT(t *testing.T, kid string, claims jwt.MapClaims) string {
	t.Helper()

	header := map[string]string{"alg": "none", "typ": "JWT"}
	if kid != "" {
		header["kid"] = kid
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	h := base64.RawURLEncoding.EncodeToString(headerJSON)
	c := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return h + "." + c + "."
}

// jwksEntry — 단일 JWKS 키 표현 (인라인 mock 용)
// fieldalignment: 포인터 필드 먼저, 문자열 마지막
type jwksEntry struct {
	// RSA 공개키 — nil이면 기본 RSA 공개키 사용
	RSAPub *rsa.PublicKey
	// EC 공개키
	ECPub *ecdsa.PublicKey
	// jwks kid
	KID string
	// kty: "RSA" | "EC" | "OKP"
	KTY string
	// alg: "RS256" | "ES256" | "EdDSA"
	ALG string
}

// mockJWKS — 주어진 키 목록을 나타내는 인라인 JWKS 맵을 반환한다.
// Sprint 1 GREEN의 Verify 구현이 이 맵을 keyfunc으로 사용할 것을 가정한다.
// 현재는 단지 테스트 헬퍼로 선언됨 — mockJWKSToKeyFunc로 연결 예정.
func mockJWKS(entries []jwksEntry) map[string]jwksEntry {
	m := make(map[string]jwksEntry, len(entries))
	for _, e := range entries {
		m[e.KID] = e
	}
	return m
}

// defaultRSAJWKS — 기본 RSA JWKS mock (testKIDRSA → RS256)
func defaultRSAJWKS() map[string]jwksEntry {
	return mockJWKS([]jwksEntry{
		{
			KID:    testKIDRSA,
			KTY:    "RSA",
			ALG:    "RS256",
			RSAPub: &testKeys.rsaPriv.PublicKey,
		},
	})
}

// defaultECJWKS — 기본 EC JWKS mock (testKIDEC → ES256)
// Sprint 1 GREEN에서 EC 키 경로 검증 시 사용된다.
func defaultECJWKS() map[string]jwksEntry {
	return mockJWKS([]jwksEntry{
		{
			ECPub: &testKeys.ecPriv.PublicKey,
			KID:   testKIDEC,
			KTY:   "EC",
			ALG:   "ES256",
		},
	})
}

// newTestValidator — 테스트용 TokenValidator를 생성한다.
// Sprint 1 GREEN에서 New() 시그니처 확정 후 이 헬퍼도 갱신 예정.
func newTestValidator(t *testing.T) *TokenValidator {
	t.Helper()
	v, err := New(context.Background(), testIssuer, testAudience)
	require.NoError(t, err)
	return v
}

// ────────────────────────────────────────────────────────────
// 테스트 케이스
// ────────────────────────────────────────────────────────────

// TestVerify_HappyPath — AC-AUTH-001-1: 유효한 RS256 토큰이 정상 검증된다.
// RED: stub이 error를 반환하므로 FAIL 예정.
func TestVerify_HappyPath(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	tokenStr := genTestJWT(t, opts)

	got, err := v.Verify(context.Background(), tokenStr)

	require.NoError(t, err, "유효한 토큰은 에러 없이 검증되어야 한다")
	require.NotNil(t, got)
	assert.Equal(t, "user-sub-001", got.Subject)
	assert.Contains(t, got.Scopes, "iroum-ax:analyst")
}

// TestVerify_ExpiredToken — AC-AUTH-001-2: exp가 지난 토큰은 ErrTokenExpired를 반환한다.
func TestVerify_ExpiredToken(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.expOffset = -100 * time.Second // 100초 전 만료 (30초 skew 초과)
	tokenStr := genTestJWT(t, opts)

	_, err := v.Verify(context.Background(), tokenStr)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenExpired), "만료된 토큰은 ErrTokenExpired를 반환해야 한다. got: %v", err)
}

// TestVerify_FutureIAT_Rejected — AC-AUTH-001-3: iat가 30초 초과 미래인 토큰은 거부된다.
// REQ-AUTH-001-U2 대응.
func TestVerify_FutureIAT_Rejected(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.iatOffset = +100 * time.Second // 100초 후 발급 (30초 skew 초과)
	tokenStr := genTestJWT(t, opts)

	_, err := v.Verify(context.Background(), tokenStr)

	require.Error(t, err, "미래 iat 토큰은 거부되어야 한다")
	// ErrTokenExpired 또는 신규 sentinel로 거부 — 어느 쪽이든 에러여야 함
	assert.True(t, errors.Is(err, ErrTokenExpired) || errors.Is(err, ErrTokenInvalidSignature),
		"미래 iat 거부 시 적절한 sentinel 에러를 반환해야 한다. got: %v", err)
}

// TestVerify_ClockSkew_Within30s_Accepted — AC-AUTH-001-7: 30초 이내 clock skew는 허용된다.
func TestVerify_ClockSkew_Within30s_Accepted(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.iatOffset = +25 * time.Second // 25초 미래 iat — skew 허용 범위 내
	tokenStr := genTestJWT(t, opts)

	got, err := v.Verify(context.Background(), tokenStr)

	require.NoError(t, err, "30초 이내 clock skew는 허용되어야 한다")
	require.NotNil(t, got)
}

// TestVerify_WrongAudience — AC-AUTH-001-4: aud 불일치 시 ErrTokenInvalidAudience를 반환한다.
func TestVerify_WrongAudience(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.audience = []string{"some-other-service"}
	tokenStr := genTestJWT(t, opts)

	_, err := v.Verify(context.Background(), tokenStr)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenInvalidAudience),
		"aud 불일치 시 ErrTokenInvalidAudience를 반환해야 한다. got: %v", err)
}

// TestVerify_AlgorithmAllowList_HS256Rejected — AC-AUTH-001-5: HS256 토큰은 즉시 거부된다.
// OWASP Algorithm Confusion Attack 방어.
func TestVerify_AlgorithmAllowList_HS256Rejected(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.alg = "HS256"
	opts.kid = ""
	tokenStr := genTestJWT(t, opts)

	_, err := v.Verify(context.Background(), tokenStr)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlgorithmNotAllowed),
		"HS256 토큰은 ErrAlgorithmNotAllowed를 반환해야 한다. got: %v", err)
}

// TestVerify_AlgorithmAllowList_NoneRejected — AC-AUTH-001-6: alg=none 토큰은 즉시 거부된다.
func TestVerify_AlgorithmAllowList_NoneRejected(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.alg = "none"
	opts.kid = ""
	tokenStr := genTestJWT(t, opts)

	_, err := v.Verify(context.Background(), tokenStr)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlgorithmNotAllowed),
		"alg=none 토큰은 ErrAlgorithmNotAllowed를 반환해야 한다. got: %v", err)
}

// TestVerify_ES256_Accepted — allow-list에 ES256이 포함됨을 검증한다.
// RED: stub이 에러를 반환하므로 FAIL 예정.
func TestVerify_ES256_Accepted(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.alg = "ES256"
	opts.kid = testKIDEC
	tokenStr := genTestJWT(t, opts)

	got, err := v.Verify(context.Background(), tokenStr)

	require.NoError(t, err, "ES256 토큰은 allow-list에 포함되어 있어야 한다")
	require.NotNil(t, got)
}

// TestVerify_TamperedSignature — AC-AUTH-001-8: 변조된 서명은 ErrTokenInvalidSignature를 반환한다.
func TestVerify_TamperedSignature(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	tokenStr := genTestJWT(t, opts)

	// 서명 부분을 변조한다
	parts := strings.Split(tokenStr, ".")
	require.Len(t, parts, 3, "JWT는 3개 파트로 구성되어야 한다")
	parts[2] = base64.RawURLEncoding.EncodeToString([]byte("tampered-signature-bytes"))
	tamperedToken := strings.Join(parts, ".")

	_, err := v.Verify(context.Background(), tamperedToken)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenInvalidSignature),
		"변조된 서명은 ErrTokenInvalidSignature를 반환해야 한다. got: %v", err)
}

// TestVerify_Blacklisted — AC-AUTH-001-10: jti가 블랙리스트에 있으면 ErrTokenBlacklisted를 반환한다.
// REQ-AUTH-001-S1 대응.
// 주의: Sprint 1 GREEN에서 Redis mock 또는 in-memory blacklist를 연동해야 함.
func TestVerify_Blacklisted(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.jti = "jti-blacklisted-001"
	tokenStr := genTestJWT(t, opts)

	// 블랙리스트에 jti 등록 — Sprint 1 GREEN에서 v.BlacklistAdd(jti) 또는 유사 API로 구현
	// 현재는 validator에 블랙리스트 주입 방식이 결정되지 않아 직접 호출 불가.
	// Verify가 내부적으로 블랙리스트를 조회할 것을 가정하고,
	// 테스트 픽스처로 blacklist를 사전 세팅하는 방식을 GREEN에서 확정.
	_ = tokenStr // TODO Sprint 1 GREEN: 블랙리스트 seeding API 연동

	// 현재 stub은 어떤 에러든 반환하므로 이 테스트는 에러 반환 자체를 확인한다.
	// GREEN 이후에는 errors.Is(err, ErrTokenBlacklisted)로 교체됨.
	_, err := v.Verify(context.Background(), tokenStr)
	require.Error(t, err, "블랙리스트 jti 토큰은 에러를 반환해야 한다")
}

// ────────────────────────────────────────────────────────────
// SF-1 — Issuer Validation (AC-AUTH-001-iss-validation)
// ────────────────────────────────────────────────────────────

// TestVerify_IssuerMismatch_Rejected — SF-1: iss 불일치 시 ErrTokenInvalidIssuer를 반환한다.
// RFC 7519 §4.1.1 + OWASP JWT cheat sheet — cross-realm token 재사용 공격 방어.
func TestVerify_IssuerMismatch_Rejected(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	// 다른 realm의 issuer로 토큰 발급 (서명/exp/aud는 모두 유효)
	opts.issuer = "https://other-realm.example.com/auth/realms/other"
	tokenStr := genTestJWT(t, opts)

	_, err := v.Verify(context.Background(), tokenStr)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenInvalidIssuer),
		"iss 불일치 시 ErrTokenInvalidIssuer를 반환해야 한다. got: %v", err)
}

// TestVerify_IssuerMatch_Accepted — SF-1: iss가 설정값과 일치하면 정상 검증된다.
func TestVerify_IssuerMatch_Accepted(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.issuer = testIssuer // 정확히 일치
	tokenStr := genTestJWT(t, opts)

	got, err := v.Verify(context.Background(), tokenStr)

	require.NoError(t, err, "iss가 일치하면 에러 없이 검증되어야 한다")
	require.NotNil(t, got)
	assert.Equal(t, testIssuer, got.Issuer)
}

// ────────────────────────────────────────────────────────────
// SF-2 — Algorithm/Key Type Cross-check (AC-AUTH-001-alg-cross-check)
// ────────────────────────────────────────────────────────────

// TestVerify_AlgKeyTypeMismatch_Rejected — SF-2: JWKS kty=RSA인데 token alg=ES256이면 거부된다.
// Algorithm Confusion Attack 변형 방어 — allow-list는 통과하지만 kty/alg cross-check에서 거부.
//
// 테스트 시나리오:
//   - JWKS: kid=rsa-key-1, kty=RSA, alg=RS256
//   - Token header: kid=rsa-key-1, alg=ES256 (intentional mismatch)
//   - 기대 결과: ErrAlgorithmKeyMismatch
//
// @MX:TODO Sprint 1 GREEN — TokenValidator에 JWKS mock 주입 인터페이스 설계 후 완성
func TestVerify_AlgKeyTypeMismatch_Rejected(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)

	// RSA kid를 헤더에 넣고 alg=ES256 (allow-list 통과)으로 서명
	opts := defaultJWTOpts()
	opts.alg = "ES256"                // ES256은 allow-list 통과
	opts.kid = testKIDRSA             // 그러나 kid는 RSA 키를 가리킴
	opts.signingKey = testKeys.ecPriv // 실제 서명은 EC 키로
	tokenStr := genTestJWT(t, opts)

	_, err := v.Verify(context.Background(), tokenStr)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlgorithmKeyMismatch),
		"kty/alg 불일치 시 ErrAlgorithmKeyMismatch를 반환해야 한다. got: %v", err)
}

// TestVerify_AlgKeyTypeMatch_RSA_Accepted — SF-2: kty=RSA + alg=RS256은 cross-check 통과.
func TestVerify_AlgKeyTypeMatch_RSA_Accepted(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.alg = "RS256"
	opts.kid = testKIDRSA
	tokenStr := genTestJWT(t, opts)

	got, err := v.Verify(context.Background(), tokenStr)

	require.NoError(t, err, "kty=RSA + alg=RS256은 cross-check를 통과해야 한다")
	require.NotNil(t, got)
}

// ────────────────────────────────────────────────────────────
// 테이블 드리븐 — 알고리즘 allow-list 경계 검증
// ────────────────────────────────────────────────────────────

// TestVerify_AlgorithmAllowList_Table — 여러 알고리즘에 대한 allow-list 동작을 검증한다.
func TestVerify_AlgorithmAllowList_Table(t *testing.T) {
	t.Parallel()

	// fieldalignment: 포인터(error) 먼저, bool, string 마지막
	cases := []struct {
		wantErr     error
		name        string
		alg         string
		kid         string
		wantAllowed bool
	}{
		{
			name:        "RS256은 허용된다",
			alg:         "RS256",
			kid:         testKIDRSA,
			wantAllowed: true,
		},
		{
			name:        "ES256은 허용된다",
			alg:         "ES256",
			kid:         testKIDEC,
			wantAllowed: true,
		},
		{
			name:        "HS256은 거부된다",
			alg:         "HS256",
			kid:         "",
			wantAllowed: false,
			wantErr:     ErrAlgorithmNotAllowed,
		},
		{
			name:        "alg=none은 거부된다",
			alg:         "none",
			kid:         "",
			wantAllowed: false,
			wantErr:     ErrAlgorithmNotAllowed,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := newTestValidator(t)
			opts := defaultJWTOpts()
			opts.alg = tc.alg
			opts.kid = tc.kid
			tokenStr := genTestJWT(t, opts)

			_, err := v.Verify(context.Background(), tokenStr)

			if tc.wantAllowed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				if tc.wantErr != nil {
					assert.True(t, errors.Is(err, tc.wantErr),
						"알고리즘 %s: 기대 에러 %v, 실제 %v", tc.alg, tc.wantErr, err)
				}
			}
		})
	}
}

// ────────────────────────────────────────────────────────────
// 테이블 드리븐 — 시간 클레임 경계값 검증
// ────────────────────────────────────────────────────────────

// TestVerify_TimeClaims_Table — exp/iat clock skew 경계값 테스트.
func TestVerify_TimeClaims_Table(t *testing.T) {
	t.Parallel()

	// fieldalignment: Duration 먼저(int64), error 인터페이스, string, bool 마지막
	cases := []struct {
		wantErr   error
		name      string
		expOffset time.Duration
		iatOffset time.Duration
		wantOK    bool
	}{
		{
			name:      "만료 100초 전 — 거부",
			expOffset: -100 * time.Second,
			iatOffset: -10 * time.Second,
			wantOK:    false,
			wantErr:   ErrTokenExpired,
		},
		{
			name:      "만료 25초 전(skew 내) — 수용",
			expOffset: -25 * time.Second,
			iatOffset: -60 * time.Second,
			wantOK:    true,
		},
		{
			name:      "iat 25초 미래(skew 내) — 수용",
			expOffset: +3600 * time.Second,
			iatOffset: +25 * time.Second,
			wantOK:    true,
		},
		{
			name:      "iat 100초 미래(skew 초과) — 거부",
			expOffset: +3600 * time.Second,
			iatOffset: +100 * time.Second,
			wantOK:    false,
		},
		{
			name:      "미래 만료 + 정상 iat — 수용",
			expOffset: +7200 * time.Second,
			iatOffset: -300 * time.Second,
			wantOK:    true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := newTestValidator(t)
			opts := defaultJWTOpts()
			opts.expOffset = tc.expOffset
			opts.iatOffset = tc.iatOffset
			tokenStr := genTestJWT(t, opts)

			_, err := v.Verify(context.Background(), tokenStr)

			if tc.wantOK {
				assert.NoError(t, err, "케이스 [%s]: 수용 기대", tc.name)
			} else {
				assert.Error(t, err, "케이스 [%s]: 거부 기대", tc.name)
				if tc.wantErr != nil {
					assert.True(t, errors.Is(err, tc.wantErr),
						"케이스 [%s]: 기대 에러 %v, 실제 %v", tc.name, tc.wantErr, err)
				}
			}
		})
	}
}

// ────────────────────────────────────────────────────────────
// ValidatedToken 필드 검증
// ────────────────────────────────────────────────────────────

// TestVerify_ValidatedTokenFields — 검증 성공 시 ValidatedToken 필드가 정확히 채워진다.
func TestVerify_ValidatedTokenFields(t *testing.T) {
	t.Parallel()

	v := newTestValidator(t)
	opts := defaultJWTOpts()
	opts.sub = "uuid-alice-001"
	opts.scopes = []string{"iroum-ax:analyst"}
	opts.jti = "jti-fields-test"
	tokenStr := genTestJWT(t, opts)

	got, err := v.Verify(context.Background(), tokenStr)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "uuid-alice-001", got.Subject, "Subject가 token sub와 일치해야 한다")
	assert.Contains(t, got.Scopes, "iroum-ax:analyst", "Scopes에 scp 클레임 값이 포함되어야 한다")
	assert.Equal(t, testIssuer, got.Issuer, "Issuer가 설정 issuer와 일치해야 한다")
	assert.True(t, got.ExpiresAt.After(time.Now()), "ExpiresAt은 현재 시각 이후여야 한다")
}

// ────────────────────────────────────────────────────────────
// 성능 벤치마크 — AC-AUTH-001-Performance
// ────────────────────────────────────────────────────────────

// BenchmarkVerify_JWKSCacheHit — JWKS 캐시 hit 시 p99 < 5ms 목표.
// Sprint 1 GREEN 이후 실제 성능 측정용.
// 현재는 stub이므로 측정값이 무의미하나 벤치마크 형태를 미리 확정.
func BenchmarkVerify_JWKSCacheHit(b *testing.B) {
	v, err := New(context.Background(), testIssuer, testAudience)
	if err != nil {
		b.Fatalf("TokenValidator 생성 실패: %v", err)
	}

	opts := defaultJWTOpts()
	tokenStr := genTestJWT(&testing.T{}, opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = v.Verify(context.Background(), tokenStr)
	}
}

// ────────────────────────────────────────────────────────────
// 유틸리티 — JWKS mock 검증 헬퍼 (Sprint 1 GREEN 연동 준비)
// ────────────────────────────────────────────────────────────

// bigIntToBase64URL — *big.Int을 JWT base64url 인코딩으로 변환 (RSA n/e 인코딩용)
func bigIntToBase64URL(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

// rsaPubToJWK — RSA 공개키를 JWK JSON 바이트로 변환 (통합 테스트 mock 서버용)
func rsaPubToJWK(pub *rsa.PublicKey, kid string) map[string]any {
	eBytes := make([]byte, 4)
	eBytes[0] = byte(pub.E >> 24)
	eBytes[1] = byte(pub.E >> 16)
	eBytes[2] = byte(pub.E >> 8)
	eBytes[3] = byte(pub.E)
	// 앞의 0 바이트 제거
	i := 0
	for i < len(eBytes)-1 && eBytes[i] == 0 {
		i++
	}
	return map[string]any{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"use": "sig",
		"n":   bigIntToBase64URL(pub.N),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes[i:]),
	}
}

// TestHelpers_GenTestJWT_ProducesValidStructure — 헬퍼 함수 자기검증.
// genTestJWT가 올바른 3-part JWT를 생성하는지 확인한다.
func TestHelpers_GenTestJWT_ProducesValidStructure(t *testing.T) {
	t.Parallel()

	opts := defaultJWTOpts()
	tokenStr := genTestJWT(t, opts)

	parts := strings.Split(tokenStr, ".")
	assert.Len(t, parts, 3, "JWT는 정확히 3개 파트여야 한다")

	// 헤더 디코딩 검증
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var header map[string]any
	require.NoError(t, json.Unmarshal(headerBytes, &header))
	assert.Equal(t, "RS256", header["alg"])
	assert.Equal(t, testKIDRSA, header["kid"])
}

// TestHelpers_MockJWKS_Structure — mockJWKS 헬퍼가 올바른 구조를 반환하는지 확인.
func TestHelpers_MockJWKS_Structure(t *testing.T) {
	t.Parallel()

	rsaJWKS := defaultRSAJWKS()
	entry, ok := rsaJWKS[testKIDRSA]
	require.True(t, ok, "testKIDRSA 키가 JWKS에 존재해야 한다")
	assert.Equal(t, "RSA", entry.KTY)
	assert.Equal(t, "RS256", entry.ALG)
	assert.NotNil(t, entry.RSAPub)

	// EC JWKS 검증 — defaultECJWKS 사용
	ecJWKS := defaultECJWKS()
	ecEntry, ok := ecJWKS[testKIDEC]
	require.True(t, ok, "testKIDEC 키가 EC JWKS에 존재해야 한다")
	assert.Equal(t, "EC", ecEntry.KTY)
	assert.Equal(t, "ES256", ecEntry.ALG)
	assert.NotNil(t, ecEntry.ECPub)

	// rsaPubToJWK 헬퍼 검증
	jwkMap := rsaPubToJWK(entry.RSAPub, entry.KID)
	assert.Equal(t, "RSA", jwkMap["kty"])
	assert.Equal(t, testKIDRSA, jwkMap["kid"])
}

// ────────────────────────────────────────────────────────────
// SPEC-AX-OBS-001 §2.2 reason 레이블 정규화 검증
// ────────────────────────────────────────────────────────────

// captureObserver — IncAuthRejection 호출을 캡처하는 mock RejectionObserver
// auth 패키지 내부에서 metrics 패키지 import 없이 reason 레이블 검증
type captureObserver struct {
	reasons []string
}

func (c *captureObserver) IncAuthRejection(reason string) {
	c.reasons = append(c.reasons, reason)
}

// TestVerify_AlgMismatch_RecordsAlgMismatchReason — SPEC-AX-OBS-001 §2.2
// kty/alg cross-check 실패 시 reason 레이블이 "alg_mismatch"여야 한다.
// ES256 토큰 + RSA kty 키 → checkAlgKTYConsistency 실패 → alg_mismatch
func TestVerify_AlgMismatch_RecordsAlgMismatchReason(t *testing.T) {
	t.Parallel()

	obs := &captureObserver{}

	// ES256 토큰을 생성하되, JWKS에는 RSA 키를 등록 (kty mismatch 유도)
	// inlineJWKSProvider를 직접 구성: kid=testKIDEC, kty=RSA, alg=RS256
	mismatched := &inlineJWKSProvider{
		entries: map[string]inlineKeyEntry{
			testKIDEC: {
				rsaPub: &testKeys.rsaPriv.PublicKey,
				kty:    "RSA",
				alg:    "RS256",
			},
		},
	}

	v, err := New(context.Background(), testIssuer, testAudience,
		WithJWKSProvider(mismatched),
		WithRejectionObserver(obs),
	)
	require.NoError(t, err)

	// ES256으로 서명된 토큰 — alg=ES256이지만 JWKS kty=RSA → cross-check 실패
	opts := defaultJWTOpts()
	opts.alg = "ES256"
	opts.kid = testKIDEC
	tokenStr := genTestJWT(t, opts)

	_, verErr := v.Verify(context.Background(), tokenStr)

	require.Error(t, verErr, "alg/kty 불일치 시 에러를 반환해야 한다")
	assert.True(t, errors.Is(verErr, ErrAlgorithmKeyMismatch),
		"ErrAlgorithmKeyMismatch를 반환해야 한다. got: %v", verErr)

	// SPEC-AX-OBS-001 §2.2: reason 레이블이 정확히 "alg_mismatch"여야 한다
	require.Len(t, obs.reasons, 1, "rejection이 정확히 1회 기록되어야 한다")
	assert.Equal(t, "alg_mismatch", obs.reasons[0],
		"reason 레이블이 spec §2.2 정규값 'alg_mismatch'여야 한다")
}
