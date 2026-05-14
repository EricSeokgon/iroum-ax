// auth 패키지 — 인증/인가 센티넬 에러 정의
// SPEC-AX-AUTH-001 REQ-AUTH-UBI-001
package auth

import "errors"

// ErrTokenExpired — JWT exp 클레임이 현재 시각(clock skew 허용 후) 이전임
var ErrTokenExpired = errors.New("토큰이 만료되었습니다")

// ErrTokenInvalidSignature — JWT 서명 검증 실패 (잘못된 키 또는 변조)
var ErrTokenInvalidSignature = errors.New("토큰 서명이 유효하지 않습니다")

// ErrTokenInvalidIssuer — JWT iss 클레임이 설정된 OIDC_ISSUER_URL과 불일치
// SF-1 보정: cross-realm token 재사용 공격 차단 (RFC 7519 §4.1.1)
var ErrTokenInvalidIssuer = errors.New("토큰 발급자가 유효하지 않습니다")

// ErrTokenInvalidAudience — JWT aud 클레임이 기대값과 불일치
var ErrTokenInvalidAudience = errors.New("토큰 대상이 유효하지 않습니다")

// ErrAlgorithmNotAllowed — JWT alg 헤더가 허용 목록(RS256/EdDSA/ES256)에 없음
// REQ-AUTH-001-U1: HS256, none 등 대칭키/무서명 알고리즘 명시 거부
var ErrAlgorithmNotAllowed = errors.New("허용되지 않는 JWT 알고리즘입니다")

// ErrAlgorithmKeyMismatch — JWT alg 헤더와 JWKS kty 필드가 불일치
// SF-2 보정: Algorithm Confusion Attack 변형 방어 (OWASP JWT cheat sheet)
var ErrAlgorithmKeyMismatch = errors.New("JWT 알고리즘과 키 타입이 일치하지 않습니다")

// ErrTokenBlacklisted — Redis 블랙리스트에 jti가 존재 (로그아웃된 토큰)
// REQ-AUTH-001-S1
var ErrTokenBlacklisted = errors.New("토큰이 블랙리스트에 등록되어 있습니다")

// ErrInsufficientPermission — 인증은 통과했으나 required permission 미달
// REQ-AUTH-004-U1: HTTP 403 / gRPC PERMISSION_DENIED 매핑
var ErrInsufficientPermission = errors.New("권한이 부족합니다")

// ErrJWKSUnavailable — JWKS 엔드포인트 미가용 + 캐시 없음
// HTTP 503 + Retry-After: 30 반환용
var ErrJWKSUnavailable = errors.New("JWKS 엔드포인트에 접근할 수 없습니다")

// ErrMissingToken — Authorization 헤더 누락 또는 Bearer 접두사 없음
// REQ-AUTH-003-U1
var ErrMissingToken = errors.New("Authorization Bearer 토큰이 누락되었습니다")
