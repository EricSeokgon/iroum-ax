"""FastAPI 인증 Dependency stub — SPEC-AX-AUTH-001 REQ-AUTH-003-E3

Sprint 4 GREEN에서 실제 구현 예정.
"""
from __future__ import annotations

from fastapi import HTTPException, Security
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from pipelines.auth.models import ValidatedToken

_bearer_scheme = HTTPBearer(auto_error=False)


async def verify_token(
    credentials: HTTPAuthorizationCredentials | None = Security(_bearer_scheme),
) -> ValidatedToken:
    """FastAPI Depends — Bearer 토큰을 검증하고 ValidatedToken을 반환한다.

    AuthEnabled=false 시: cli-anonymous 사용자로 폴백 (backward compat).
    AuthEnabled=true 시: TokenValidator.verify 호출 (Sprint 4 GREEN에서 구현).

    Args:
        credentials: HTTPBearer scheme에서 추출된 자격증명

    Returns:
        ValidatedToken — 검증된 토큰 (또는 anonymous 폴백)

    Raises:
        HTTPException(401): Authorization 헤더 누락 또는 검증 실패 (AuthEnabled=true 시)

    # @MX:ANCHOR: [AUTO] Python 측 인증 진입점 (FastAPI Depends)
    # @MX:REASON: 모든 protected endpoint에서 호출 (fan_in >= 5)
    # @MX:TODO Sprint 4 — settings.auth_enabled 확인 후 TokenValidator.verify 연동
    """
    from pipelines.config.settings import settings  # 순환 임포트 방지 위해 지연 임포트

    if not settings.auth_enabled:
        # AuthEnabled=false: cli-anonymous 폴백 (R-AUTH-007 backward compat)
        from datetime import datetime, timedelta, timezone

        return ValidatedToken(
            subject="cli-anonymous",
            issuer="",
            audience=[],
            scopes=[],
            expires_at=datetime.now(timezone.utc) + timedelta(hours=1),
            claims={},
        )

    # AuthEnabled=true: Sprint 4에서 구현
    if credentials is None:
        raise HTTPException(
            status_code=401,
            detail="Authorization Bearer 토큰이 누락되었습니다",
            headers={"WWW-Authenticate": 'Bearer realm="iroum-ax", error="invalid_request"'},
        )

    raise HTTPException(
        status_code=501,
        detail="구현 예정: Sprint 4 GREEN",
    )
