"""JWT 토큰 검증기 — PyJWT 기반.

SF-1 iss 검증 + SF-2 alg/kty cross-check 적용.
허용 알고리즘: RS256/ES256/EdDSA (HS256/none 거부 — REQ-AUTH-001-U1).
JWKS 없이 구조 검증만 수행 (Sprint 4 scope: middleware 레이어 테스트).

# @MX:ANCHOR: [AUTO] Python 측 JWT 검증 진입점
# @MX:REASON: 모든 protected endpoint에서 호출 예상 (fan_in >= 5)
# @MX:SPEC: SPEC-AX-AUTH-001 REQ-AUTH-001, REQ-AUTH-003-E3
"""
from __future__ import annotations

import base64
import json
from typing import Any

from pipelines.auth.errors import (
    AlgorithmKeyMismatchError,
    AlgorithmNotAllowedError,
    TokenExpiredError,
    TokenInvalidAudienceError,
    TokenInvalidIssuerError,
    TokenInvalidSignatureError,
)
from pipelines.auth.models import ValidatedToken

# 허용 알고리즘 목록 (REQ-AUTH-001-U1)
ALLOWED_ALGORITHMS: frozenset[str] = frozenset({"RS256", "ES256", "EdDSA"})

# alg → 기대 kty 매핑 (SF-2 cross-check)
_ALG_TO_KTY: dict[str, str] = {
    "RS256": "RSA",
    "RS384": "RSA",
    "RS512": "RSA",
    "ES256": "EC",
    "ES384": "EC",
    "ES512": "EC",
    "EdDSA": "OKP",
}


def _b64url_decode(segment: str) -> bytes:
    """Base64url 패딩 보완 후 디코드."""
    # 패딩 보완: urlsafe_b64decode는 패딩이 맞아야 함
    padding = 4 - len(segment) % 4
    if padding != 4:
        segment += "=" * padding
    return base64.urlsafe_b64decode(segment)


def _parse_jwt_header(token: str) -> dict[str, Any]:
    """JWT 헤더 파싱 (서명 검증 없이).

    Args:
        token: raw JWT 문자열

    Returns:
        헤더 딕셔너리

    Raises:
        TokenInvalidSignatureError: JWT 형식 오류
    """
    parts = token.split(".")
    if len(parts) != 3:  # noqa: PLR2004
        raise TokenInvalidSignatureError("JWT 형식 오류: 3개 세그먼트가 필요합니다")
    try:
        header_bytes = _b64url_decode(parts[0])
        return json.loads(header_bytes)  # type: ignore[no-any-return]
    except (ValueError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise TokenInvalidSignatureError(f"JWT 헤더 파싱 실패: {exc}") from exc


def _parse_jwt_payload(token: str) -> dict[str, Any]:
    """JWT 페이로드 파싱 (서명 검증 없이).

    Args:
        token: raw JWT 문자열

    Returns:
        페이로드 딕셔너리

    Raises:
        TokenInvalidSignatureError: JWT 형식 오류 또는 페이로드 파싱 실패
    """
    parts = token.split(".")
    if len(parts) != 3:  # noqa: PLR2004
        raise TokenInvalidSignatureError("JWT 형식 오류: 3개 세그먼트가 필요합니다")
    try:
        payload_bytes = _b64url_decode(parts[1])
        return json.loads(payload_bytes)  # type: ignore[no-any-return]
    except (ValueError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise TokenInvalidSignatureError(f"JWT 페이로드 파싱 실패: {exc}") from exc


class TokenValidator:
    """JWT 서명 검증기.

    Sprint 4 scope: 구조 검증 (alg allow-list, SF-2 kty cross-check, iss/aud/exp 클레임).
    JWKS 서명 검증은 Sprint 1에서 분리 구현.

    # @MX:ANCHOR: [AUTO] Python 측 인증 진입점
    # @MX:REASON: 모든 protected endpoint에서 호출 (fan_in >= 5)
    # @MX:SPEC: SPEC-AX-AUTH-001 REQ-AUTH-001
    """

    def __init__(self, oidc_issuer: str, audience: str, clock_skew: int = 30) -> None:
        """TokenValidator 초기화.

        Args:
            oidc_issuer: 기대 issuer URL (SF-1 per-token iss 검증에 사용)
            audience: 기대 aud 클레임 값 (예: "iroum-ax-pipelines")
            clock_skew: 시간 클레임 허용 오차(초), 기본 30초
        """
        if not oidc_issuer:
            raise ValueError("OIDC issuer URL이 비어 있습니다")
        self._oidc_issuer = oidc_issuer
        self._audience = audience
        self._clock_skew = clock_skew

    def verify(self, token: str) -> ValidatedToken:
        """Bearer 토큰 문자열을 받아 구조·클레임을 검증하고 ValidatedToken을 반환한다.

        검증 순서:
        1. alg 헤더 허용 목록 확인 (REQ-AUTH-001-U1)
        2. kty vs alg cross-check (SF-2)
        3. exp 만료 검증 (clock_skew 허용)
        4. iss 검증 (SF-1)
        5. aud 검증

        Args:
            token: Bearer 토큰 문자열 (접두사 제거된 raw JWT)

        Returns:
            ValidatedToken — 검증된 토큰 페이로드

        Raises:
            TokenInvalidSignatureError: 형식 오류 또는 kid 미발견
            AlgorithmNotAllowedError: 허용되지 않는 알고리즘
            AlgorithmKeyMismatchError: alg/kty cross-check 실패 (SF-2)
            TokenExpiredError: exp 클레임 만료
            TokenInvalidIssuerError: iss 불일치 (SF-1)
            TokenInvalidAudienceError: aud 불일치
        """
        if not token:
            raise TokenInvalidSignatureError("토큰이 비어 있습니다")

        # 1단계: 헤더 파싱
        header = _parse_jwt_header(token)
        alg = header.get("alg", "")
        kty = header.get("kty", "")

        # 2단계: alg 허용 목록 검사 (REQ-AUTH-001-U1)
        if alg not in ALLOWED_ALGORITHMS:
            raise AlgorithmNotAllowedError(f"허용되지 않는 알고리즘: {alg!r}")

        # 3단계: SF-2 alg/kty cross-check (kty가 헤더에 있을 때만)
        if kty:
            expected_kty = _ALG_TO_KTY.get(alg)
            if expected_kty and kty != expected_kty:
                raise AlgorithmKeyMismatchError(
                    f"kty {kty!r} != 알고리즘 {alg!r}에 기대되는 {expected_kty!r} (SF-2)"
                )

        # 4단계: 페이로드 파싱
        payload = _parse_jwt_payload(token)

        # 5단계: exp 만료 검증 (clock_skew 허용)
        import time

        exp = payload.get("exp")
        if exp is not None:
            now = time.time()
            if now > exp + self._clock_skew:
                raise TokenExpiredError(
                    f"토큰 만료됨: exp={exp}, now={now:.0f}, skew={self._clock_skew}s"
                )

        # 6단계: iss 검증 (SF-1)
        iss = payload.get("iss", "")
        if iss != self._oidc_issuer:
            raise TokenInvalidIssuerError(
                f"iss 불일치: got={iss!r}, expected={self._oidc_issuer!r} (SF-1)"
            )

        # 7단계: aud 검증
        aud_claim = payload.get("aud", [])
        if isinstance(aud_claim, str):
            aud_list = [aud_claim]
        else:
            aud_list = list(aud_claim)
        if self._audience not in aud_list:
            raise TokenInvalidAudienceError(
                f"aud 불일치: got={aud_list!r}, expected={self._audience!r}"
            )

        # 8단계: ValidatedToken 구성
        sub = payload.get("sub", "")
        scope_str = payload.get("scope", "")
        scopes = scope_str.split() if isinstance(scope_str, str) and scope_str else []

        from datetime import datetime, timezone

        expires_at = (
            datetime.fromtimestamp(exp, tz=timezone.utc)
            if exp is not None
            else datetime.now(tz=timezone.utc)
        )

        return ValidatedToken(
            subject=sub,
            issuer=iss,
            audience=aud_list,
            scopes=scopes,
            expires_at=expires_at,
            claims=dict(payload),
        )
