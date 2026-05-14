"""pipelines.auth — SSO/JWT 인증 + RBAC 패키지 (Python)

SPEC-AX-AUTH-001 Sprint 0 Foundation stub.
Sprint 1+ 에서 비즈니스 로직을 구현한다.

공개 API:
    - :class:`TokenValidator` — JWT 서명 검증기
    - :func:`verify_token` — FastAPI Depends dependency
    - :class:`ValidatedToken` — 검증된 토큰 페이로드
"""
from __future__ import annotations

__all__ = [
    "TokenValidator",
    "ValidatedToken",
    "verify_token",
]

from pipelines.auth.models import ValidatedToken
from pipelines.auth.validator import TokenValidator
from pipelines.auth.dependencies import verify_token
