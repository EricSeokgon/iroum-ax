// JWT 토큰 검증기 — SPEC-AX-AUTH-001 REQ-AUTH-001 구현
// Sprint 1 GREEN: TokenValidator.Verify 실제 구현
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ────────────────────────────────────────────────────────────
// 인터페이스 정의
// ────────────────────────────────────────────────────────────

// JWKSProvider — JWKS 키 조회 인터페이스 (테스트 mock 주입 가능)
type JWKSProvider interface {
	// GetKey — kid로 서명 검증 키와 메타데이터를 조회한다.
	GetKey(ctx context.Context, kid string) (key any, alg string, kty string, err error)
}

// BlacklistChecker — JTI 블랙리스트 조회 인터페이스
type BlacklistChecker interface {
	// IsBlacklisted — jti가 블랙리스트에 등록되어 있으면 true를 반환한다.
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// ────────────────────────────────────────────────────────────
// 내부 기본 구현체
// ────────────────────────────────────────────────────────────

// noopBlacklist — 블랙리스트 미설정 시 사용하는 no-op 구현체
// 모든 jti를 유효한 것으로 간주한다.
type noopBlacklist struct{}

func (noopBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) { return false, nil }

// inlineJWKSProvider — 테스트 및 기본 동작용 인라인 JWKS 제공자
// 테스트에서 validator_test.go의 testKeys를 직접 참조하여 키를 제공한다.
type inlineJWKSProvider struct {
	// entries — kid → 키 메타데이터 맵
	entries map[string]inlineKeyEntry
}

// inlineKeyEntry — 인라인 JWKS의 단일 키 항목
// fieldalignment: 포인터 먼저, 문자열 마지막
type inlineKeyEntry struct {
	rsaPub *rsa.PublicKey
	ecPub  *ecdsa.PublicKey
	kty    string
	alg    string
}

func (p *inlineJWKSProvider) GetKey(_ context.Context, kid string) (any, string, string, error) {
	e, ok := p.entries[kid]
	if !ok {
		return nil, "", "", fmt.Errorf("%w: kid=%s", ErrJWKSUnavailable, kid)
	}
	switch e.kty {
	case "RSA":
		return e.rsaPub, e.alg, e.kty, nil
	case "EC":
		return e.ecPub, e.alg, e.kty, nil
	default:
		return nil, "", "", fmt.Errorf("%w: 지원하지 않는 kty=%s", ErrAlgorithmNotAllowed, e.kty)
	}
}

// ────────────────────────────────────────────────────────────
// ValidatedToken — 검증 완료 토큰 페이로드
// ────────────────────────────────────────────────────────────

// ValidatedToken — 서명 검증 후 파싱된 JWT 페이로드
// fieldalignment: map 먼저(포인터 크기), slice, string, time.Time 순
type ValidatedToken struct {
	ExpiresAt time.Time
	Claims    map[string]any
	Subject   string
	Issuer    string
	Audience  []string
	Scopes    []string
}

// ────────────────────────────────────────────────────────────
// TokenValidator — JWT 서명 검증기
// ────────────────────────────────────────────────────────────

// TokenValidator — JWT 서명 검증기
// JWKS 엔드포인트에서 공개키를 가져와 RS256/EdDSA/ES256 서명을 검증한다.
//
// @MX:ANCHOR: [AUTO] 모든 인증 경로 단일 진입점
// @MX:REASON: gRPC interceptor / REST middleware / logout / refresh / test 에서 호출 (fan_in >= 5)
type TokenValidator struct { //nolint:govet // fieldalignment: 가독성을 위해 논리적 순서 유지 (인터페이스 96바이트는 압축 불가)
	// jwksProvider — JWKS 키 조회 (테스트 시 mock 주입 가능)
	jwksProvider JWKSProvider
	// blacklist — JTI 블랙리스트 조회
	blacklist BlacklistChecker
	// rejectionObs — JWT reject 시 호출되는 관찰자 (nil-safe, optional)
	// SPEC-AX-OBS-001: circular import 해소를 위한 DI — auth는 observer interface만 알고 있음
	rejectionObs RejectionObserver
	// allowedAlgorithms — 허용 알고리즘 목록
	allowedAlgorithms []string
	// expectedIssuer — 기대 issuer URL (SF-1 per-token iss 검증)
	expectedIssuer string
	// expectedAudience — 기대 aud 클레임 값
	expectedAudience string
	// clockSkew — 허용 clock skew (기본 30초)
	clockSkew time.Duration
}

// ValidatorOption — TokenValidator 생성 옵션 함수 타입
type ValidatorOption func(*TokenValidator)

// WithIssuer — expectedIssuer 설정 옵션
func WithIssuer(iss string) ValidatorOption {
	return func(v *TokenValidator) { v.expectedIssuer = iss }
}

// WithAudience — expectedAudience 설정 옵션
func WithAudience(aud string) ValidatorOption {
	return func(v *TokenValidator) { v.expectedAudience = aud }
}

// WithAllowedAlgs — 허용 알고리즘 목록 설정 옵션
func WithAllowedAlgs(algs []string) ValidatorOption {
	return func(v *TokenValidator) { v.allowedAlgorithms = algs }
}

// WithClockSkew — clock skew 허용 시간 설정 옵션
func WithClockSkew(d time.Duration) ValidatorOption {
	return func(v *TokenValidator) { v.clockSkew = d }
}

// WithJWKSProvider — JWKSProvider 주입 옵션 (테스트 mock 연동)
func WithJWKSProvider(p JWKSProvider) ValidatorOption {
	return func(v *TokenValidator) { v.jwksProvider = p }
}

// WithBlacklistChecker — BlacklistChecker 주입 옵션
func WithBlacklistChecker(c BlacklistChecker) ValidatorOption {
	return func(v *TokenValidator) { v.blacklist = c }
}

// WithRejectionObserver — JWT reject 관찰자 주입 옵션 (SPEC-AX-OBS-001)
// nil을 주입하면 no-op으로 동작한다 (nil-safe).
func WithRejectionObserver(o RejectionObserver) ValidatorOption {
	return func(v *TokenValidator) { v.rejectionObs = o }
}

// defaultAllowedAlgorithms — 기본 허용 알고리즘 목록
// REQ-AUTH-001-U1: 비대칭 키 알고리즘만 허용
var defaultAllowedAlgorithms = []string{"RS256", "ES256", "EdDSA"}

// New — TokenValidator 생성자 (레거시 호환 시그니처)
// Sprint 1 GREEN: ctx, oidcIssuer, audience 파라미터로 기본 validator 생성
//
// 테스트에서는 New(ctx, issuer, audience)로 호출하며,
// 내부적으로 testKeys를 참조하는 inlineJWKSProvider가 기본으로 설정된다.
func New(_ context.Context, oidcIssuer, audience string, opts ...ValidatorOption) (*TokenValidator, error) {
	if oidcIssuer == "" {
		return nil, errors.New("OIDC issuer URL이 비어 있습니다")
	}

	v := &TokenValidator{
		expectedIssuer:    oidcIssuer,
		expectedAudience:  audience,
		allowedAlgorithms: defaultAllowedAlgorithms,
		clockSkew:         30 * time.Second,
		blacklist:         defaultBlacklistProvider(),
		// 기본 JWKS provider: 테스트 패키지 내 testKeys 접근을 위해 패키지 레벨 함수로 위임
		jwksProvider: defaultJWKSProvider(),
	}

	for _, opt := range opts {
		opt(v)
	}
	return v, nil
}

// ────────────────────────────────────────────────────────────
// Verify — 핵심 검증 로직
// ────────────────────────────────────────────────────────────

// Verify — Bearer 토큰 문자열을 받아 서명·클레임을 검증하고 ValidatedToken을 반환한다.
//
// 검증 순서:
//  1. 헤더 파싱 (alg, kid 추출)
//  2. alg 허용 목록 확인 (REQ-AUTH-001-U1)
//  3. JWKS 키 조회
//  4. kty/alg cross-check (SF-2)
//  5. 서명 검증
//  6. 시간 클레임 검증 (exp/iat clock skew 30초)
//  7. aud 검증
//  8. iss 검증 (SF-1)
//  9. JTI 블랙리스트 확인 (REQ-AUTH-001-S1)
//
// @MX:ANCHOR: [AUTO] JWT 검증 단일 진입점 — 모든 인증 경로가 이 함수를 통과함
// @MX:REASON: gRPC interceptor, REST middleware, refresh, logout, 테스트에서 호출 (fan_in >= 5)
func (v *TokenValidator) Verify(ctx context.Context, tokenString string) (*ValidatedToken, error) {
	// a. 헤더 파싱 (서명 검증 없이 alg, kid 추출)
	alg, kid, err := extractHeader(tokenString)
	if err != nil {
		v.recordRejection("invalid_signature")
		return nil, fmt.Errorf("%w: %w", ErrTokenInvalidSignature, err)
	}

	// b. alg 허용 목록 확인
	if !v.isAllowedAlg(alg) {
		v.recordRejection("algorithm_not_allowed")
		return nil, fmt.Errorf("%w: alg=%s", ErrAlgorithmNotAllowed, alg)
	}

	// c. JWKS 키 조회
	pubKey, jwksAlg, kty, err := v.jwksProvider.GetKey(ctx, kid)
	if err != nil {
		v.recordRejection("jwks_unavailable")
		return nil, fmt.Errorf("JWKS 키 조회 실패: %w", err)
	}

	// d. SF-2 kty/alg cross-check
	if err := checkAlgKTYConsistency(alg, kty); err != nil {
		// SPEC-AX-OBS-001 §2.2: reason 레이블 정규 집합 {invalid_issuer, alg_mismatch, expired, blacklist}
		v.recordRejection("alg_mismatch")
		return nil, err
	}
	_ = jwksAlg // jwksAlg는 kty cross-check에 의존하므로 별도 사용 없음

	// e+f. 서명 검증 (jwt.ParseWithClaims 사용)
	mapClaims := jwt.MapClaims{}
	token, parseErr := jwt.ParseWithClaims(tokenString, mapClaims, func(_ *jwt.Token) (any, error) {
		// alg 허용 목록 확인은 이미 위에서 완료됨.
		// keyfunc은 검증된 pubKey를 그대로 반환한다.
		return pubKey, nil
	}, jwt.WithoutClaimsValidation())

	if parseErr != nil || !token.Valid {
		// 서명 실패는 ErrTokenInvalidSignature로 래핑
		v.recordRejection("invalid_signature")
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalidSignature, parseErr)
	}

	// g. 클레임 추출
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("%w: claims 파싱 실패", ErrTokenInvalidSignature)
	}

	now := time.Now()

	// h. 시간 클레임 검증 (clock skew 적용)
	// exp: exp + clockSkew < now → 만료 (clock skew만큼 유예 후 만료 판정)
	// 예: exp = now-25s, skew=30s → exp+skew = now+5s > now → 유효
	// 예: exp = now-100s, skew=30s → exp+skew = now-70s < now → 만료
	if exp, ok := getTimeClaimUnix(claims, "exp"); ok {
		if exp.Add(v.clockSkew).Before(now) {
			v.recordRejection("expired")
			return nil, fmt.Errorf("%w", ErrTokenExpired)
		}
	}

	// iat: iat - clockSkew > now → clock skew 초과 미래 iat → 거부
	// 예: iat = now+100s, skew=30s → iat-skew = now+70s > now → 거부
	// 예: iat = now+25s, skew=30s → iat-skew = now-5s < now → 허용
	if iat, ok := getTimeClaimUnix(claims, "iat"); ok {
		if iat.Add(-v.clockSkew).After(now) {
			v.recordRejection("expired")
			return nil, fmt.Errorf("%w: iat가 현재 시각보다 미래입니다", ErrTokenExpired)
		}
	}

	// nbf: nbf - clockSkew > now → 아직 유효하지 않음
	if nbf, ok := getTimeClaimUnix(claims, "nbf"); ok {
		if nbf.Add(-v.clockSkew).After(now) {
			v.recordRejection("expired")
			return nil, fmt.Errorf("%w: nbf 이전 토큰", ErrTokenExpired)
		}
	}

	// i. aud 검증
	aud := extractAudience(claims)
	if !containsString(aud, v.expectedAudience) {
		v.recordRejection("invalid_audience")
		return nil, fmt.Errorf("%w: aud=%v", ErrTokenInvalidAudience, aud)
	}

	// j. SF-1 iss 검증
	// map type assertion은 (value, bool) 패턴 — 빈 문자열이 반환되면 불일치로 처리
	issRaw := claims["iss"]
	iss, _ := issRaw.(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환, 이후 비교에서 거부
	if iss != v.expectedIssuer {
		v.recordRejection("invalid_issuer")
		return nil, fmt.Errorf("%w: iss=%s", ErrTokenInvalidIssuer, iss)
	}

	// k. JTI 블랙리스트 확인
	jtiRaw := claims["jti"]
	jti, _ := jtiRaw.(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환, 빈 jti는 블랙리스트 건너뜀
	if jti != "" {
		blacklisted, blErr := v.blacklist.IsBlacklisted(ctx, jti)
		if blErr != nil {
			return nil, fmt.Errorf("블랙리스트 조회 실패: %w", blErr)
		}
		if blacklisted {
			v.recordRejection("blacklisted")
			return nil, fmt.Errorf("%w: jti=%s", ErrTokenBlacklisted, jti)
		}
	}

	// l. ValidatedToken 구성
	subRaw := claims["sub"]
	sub, _ := subRaw.(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환
	scopes := extractScopes(claims)
	expTime := getExpTime(claims)

	allClaims := make(map[string]any, len(claims))
	for k, val := range claims {
		allClaims[k] = val
	}

	return &ValidatedToken{
		ExpiresAt: expTime,
		Claims:    allClaims,
		Subject:   sub,
		Issuer:    iss,
		Audience:  aud,
		Scopes:    scopes,
	}, nil
}

// ────────────────────────────────────────────────────────────
// 내부 헬퍼 함수
// ────────────────────────────────────────────────────────────

// extractHeader — JWT 토큰 문자열에서 alg와 kid를 추출한다.
// 서명 검증 없이 헤더만 파싱한다.
func extractHeader(tokenString string) (alg, kid string, err error) {
	parts := strings.SplitN(tokenString, ".", 3)
	if len(parts) < 2 {
		return "", "", errors.New("JWT 형식이 유효하지 않습니다")
	}

	headerBytes, decErr := base64.RawURLEncoding.DecodeString(parts[0])
	if decErr != nil {
		return "", "", fmt.Errorf("JWT 헤더 디코딩 실패: %w", decErr)
	}

	var header map[string]any
	if jsonErr := json.Unmarshal(headerBytes, &header); jsonErr != nil {
		return "", "", fmt.Errorf("JWT 헤더 파싱 실패: %w", jsonErr)
	}

	alg, _ = header["alg"].(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환, 이후 허용 목록에서 거부
	kid, _ = header["kid"].(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환, JWKS 조회에서 처리
	return alg, kid, nil
}

// recordRejection — nil-safe observer 호출 헬퍼.
// rejectionObs가 nil이면 no-op으로 동작한다 (SPEC-AX-OBS-001 §2.0 DI pattern).
func (v *TokenValidator) recordRejection(reason string) {
	if v.rejectionObs != nil {
		v.rejectionObs.IncAuthRejection(reason)
	}
}

// isAllowedAlg — alg가 허용 목록에 포함되는지 확인한다.
func (v *TokenValidator) isAllowedAlg(alg string) bool {
	for _, a := range v.allowedAlgorithms {
		if a == alg {
			return true
		}
	}
	return false
}

// checkAlgKTYConsistency — SF-2: JWT alg와 JWKS kty의 일관성을 검증한다.
// Algorithm Confusion Attack 방어: kty=RSA인 키로 ES256 검증 시도 차단
func checkAlgKTYConsistency(alg, kty string) error {
	var expectedKTY string
	switch {
	case strings.HasPrefix(alg, "RS") || strings.HasPrefix(alg, "PS"):
		expectedKTY = "RSA"
	case strings.HasPrefix(alg, "ES"):
		expectedKTY = "EC"
	case alg == "EdDSA":
		expectedKTY = "OKP"
	default:
		// 허용 목록 체크 이후이므로 여기는 도달하지 않아야 함
		return fmt.Errorf("%w: 알 수 없는 alg=%s", ErrAlgorithmNotAllowed, alg)
	}

	if kty != expectedKTY {
		return fmt.Errorf("%w: alg=%s는 kty=%s를 요구하지만 kty=%s 키가 제공됨",
			ErrAlgorithmKeyMismatch, alg, expectedKTY, kty)
	}
	return nil
}

// getTimeClaimUnix — claims에서 Unix 타임스탬프 클레임을 추출한다.
func getTimeClaimUnix(claims jwt.MapClaims, key string) (time.Time, bool) {
	v, ok := claims[key]
	if !ok {
		return time.Time{}, false
	}
	switch val := v.(type) {
	case float64:
		return time.Unix(int64(val), 0), true
	case json.Number:
		n, err := val.Int64()
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(n, 0), true
	case int64:
		return time.Unix(val, 0), true
	}
	return time.Time{}, false
}

// getExpTime — exp 클레임을 time.Time으로 반환한다. 없으면 zero value.
func getExpTime(claims jwt.MapClaims) time.Time {
	t, _ := getTimeClaimUnix(claims, "exp")
	return t
}

// extractAudience — aud 클레임을 []string으로 추출한다.
// aud는 string 또는 []interface{} 형태로 올 수 있다.
func extractAudience(claims jwt.MapClaims) []string {
	raw, ok := claims["aud"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case string:
		return []string{v}
	case []any:
		result := make([]string, 0, len(v))
		for _, a := range v {
			if s, ok := a.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	}
	return nil
}

// extractScopes — scp 또는 scope 클레임을 []string으로 추출한다.
// scp 클레임은 공백으로 구분된 문자열이다.
func extractScopes(claims jwt.MapClaims) []string {
	// scp 클레임 우선 시도
	for _, key := range []string{"scp", "scope"} {
		raw, ok := claims[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case string:
			if v == "" {
				continue
			}
			return strings.Fields(v)
		case []any:
			result := make([]string, 0, len(v))
			for _, s := range v {
				if str, ok := s.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

// containsString — 문자열 슬라이스에서 target을 찾는다.
func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// ────────────────────────────────────────────────────────────
// 기본 JWKS Provider — 테스트 패키지 내 키 참조
// ────────────────────────────────────────────────────────────

// defaultJWKSProvider — 패키지 레벨 기본 JWKS 제공자 생성자
// 테스트 패키지(auth_test 아닌 auth 패키지 내부)에서 testKeys에 접근하기 위해
// 함수 변수로 오버라이드 가능하도록 설계한다.
//
// @MX:NOTE: [AUTO] 이 변수는 테스트에서 TestMain()으로 오버라이드된다.
// 실제 프로덕션에서는 Sprint 2에서 OIDC Discovery 기반으로 교체 예정.
var defaultJWKSProvider = func() JWKSProvider {
	// 빈 provider — 테스트에서는 테스트 헬퍼가 설정을 대체한다.
	// 실제 사용 시에는 WithJWKSProvider 옵션으로 주입해야 한다.
	return &inlineJWKSProvider{entries: map[string]inlineKeyEntry{}}
}

// defaultBlacklistProvider — 패키지 레벨 기본 블랙리스트 제공자 생성자
// 테스트에서 TestMain()으로 오버라이드하여 사전 등록된 jti를 포함시킨다.
//
// @MX:NOTE: [AUTO] 이 변수는 테스트에서 TestMain()으로 오버라이드된다.
// 실제 프로덕션에서는 Sprint 3에서 Redis 기반으로 교체 예정.
var defaultBlacklistProvider = func() BlacklistChecker {
	return noopBlacklist{}
}
