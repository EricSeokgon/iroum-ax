"""인증/인가 예외 클래스 — SPEC-AX-AUTH-001 REQ-AUTH-UBI-001

Go 등가물: apps/control-plane/internal/auth/errors.go
"""
from __future__ import annotations


class AuthError(Exception):
    """인증/인가 기본 예외 클래스"""


class TokenExpiredError(AuthError):
    """JWT exp 클레임이 현재 시각(clock skew 허용 후) 이전임."""


class TokenInvalidSignatureError(AuthError):
    """JWT 서명 검증 실패 (잘못된 키 또는 변조)."""


class TokenInvalidIssuerError(AuthError):
    """JWT iss 클레임이 설정된 OIDC_ISSUER_URL과 불일치.

    SF-1 보정: cross-realm token 재사용 공격 차단 (RFC 7519 §4.1.1).
    """


class TokenInvalidAudienceError(AuthError):
    """JWT aud 클레임이 기대값과 불일치."""


class AlgorithmNotAllowedError(AuthError):
    """JWT alg 헤더가 허용 목록(RS256/EdDSA/ES256)에 없음.

    REQ-AUTH-001-U1: HS256, none 등 대칭키/무서명 알고리즘 명시 거부.
    """


class AlgorithmKeyMismatchError(AuthError):
    """JWT alg 헤더와 JWKS kty 필드가 불일치.

    SF-2 보정: Algorithm Confusion Attack 변형 방어 (OWASP JWT cheat sheet).
    """


class TokenBlacklistedError(AuthError):
    """Redis 블랙리스트에 jti가 존재 (로그아웃된 토큰).

    REQ-AUTH-001-S1.
    """


class InsufficientPermissionError(AuthError):
    """인증은 통과했으나 required permission 미달.

    REQ-AUTH-004-U1: HTTP 403 매핑.
    """


class JWKSUnavailableError(AuthError):
    """JWKS 엔드포인트 미가용 + 캐시 없음.

    HTTP 503 + Retry-After: 30 반환용.
    """


class MissingTokenError(AuthError):
    """Authorization 헤더 누락 또는 Bearer 접두사 없음.

    REQ-AUTH-003-U1.
    """
