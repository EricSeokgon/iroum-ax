"""FastAPI 인증 Dependency — SPEC-AX-AUTH-001 REQ-AUTH-003-E3

verify_token: Bearer 토큰 검증 후 ValidatedToken 반환.
AuthEnabled=false 시 cli-anonymous 폴백 (REQ-AUTH-UBI-001 backward compat).

# @MX:ANCHOR: [AUTO] FastAPI Depends 인증 진입점
# @MX:REASON: 모든 protected endpoint에서 호출 예상 (fan_in >= 5)
# @MX:SPEC: SPEC-AX-AUTH-001 REQ-AUTH-003-E3
"""
from __future__ import annotations

from fastapi import HTTPException, Security
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from pipelines.auth.errors import AuthError
from pipelines.auth.models import ValidatedToken
from pipelines.auth.validator import TokenValidator

_bearer_scheme = HTTPBearer(auto_error=False)


async def verify_token(
    credentials: HTTPAuthorizationCredentials | None = Security(_bearer_scheme),
) -> ValidatedToken:
    """FastAPI Depends — Bearer 토큰을 검증하고 ValidatedToken을 반환한다.

    AuthEnabled=false 시: cli-anonymous 사용자로 폴백 (backward compat).
    AuthEnabled=true 시: TokenValidator.verify 호출.

    Args:
        credentials: HTTPBearer scheme에서 추출된 자격증명

    Returns:
        ValidatedToken — 검증된 토큰 (또는 anonymous 폴백)

    Raises:
        HTTPException(401): Authorization 헤더 누락 또는 검증 실패 (AuthEnabled=true 시)

    # @MX:ANCHOR: [AUTO] Python 측 인증 진입점 (FastAPI Depends)
    # @MX:REASON: 모든 protected endpoint에서 호출 (fan_in >= 5)
    """
    # 순환 임포트 방지 위해 지연 임포트 (settings는 모듈 수준 reload 가능)
    import importlib

    import pipelines.config.settings as cfg_mod

    importlib.reload(cfg_mod)
    current_settings = cfg_mod.settings

    if not current_settings.auth_enabled:
        # AuthEnabled=false: cli-anonymous 폴백 (R-AUTH-007 backward compat)
        from datetime import datetime, timedelta, timezone

        return ValidatedToken(
            subject=current_settings.default_user_id,
            issuer="",
            audience=[],
            scopes=[],
            expires_at=datetime.now(timezone.utc) + timedelta(hours=1),
            claims={},
        )

    # AuthEnabled=true: Bearer 헤더 필수
    if credentials is None:
        raise HTTPException(
            status_code=401,
            detail="Authorization Bearer 토큰이 누락되었습니다",
            headers={"WWW-Authenticate": 'Bearer realm="iroum-ax", error="invalid_request"'},
        )

    # TokenValidator 생성 후 검증
    validator = TokenValidator(
        oidc_issuer=current_settings.oidc_issuer_url,
        audience=current_settings.oidc_audience,
        clock_skew=current_settings.clock_skew_seconds,
    )

    try:
        return validator.verify(credentials.credentials)
    except AuthError as exc:
        raise HTTPException(
            status_code=401,
            detail=str(exc),
            headers={
                "WWW-Authenticate": 'Bearer realm="iroum-ax", error="invalid_token"',
            },
        ) from exc
