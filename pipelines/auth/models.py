"""JWT 토큰 관련 Pydantic 모델 — SPEC-AX-AUTH-001 §2.2"""
from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field


class ValidatedToken(BaseModel):
    """서명 검증 후 파싱된 JWT 페이로드.

    Go 등가물: apps/control-plane/internal/auth/validator.go ValidatedToken
    """

    # JWT sub 클레임 (Keycloak user ID)
    subject: str = Field(..., description="JWT sub 클레임")
    # JWT iss 클레임 (SF-1 검증 완료된 값)
    issuer: str = Field(..., description="JWT iss 클레임")
    # JWT aud 클레임
    audience: list[str] = Field(default_factory=list, description="JWT aud 클레임")
    # JWT scope 클레임 파싱 결과
    scopes: list[str] = Field(default_factory=list, description="파싱된 스코프 목록")
    # JWT exp 클레임
    expires_at: datetime = Field(..., description="토큰 만료 시각")
    # 원본 클레임 맵 (확장 클레임 접근용)
    claims: dict[str, object] = Field(default_factory=dict, description="원본 클레임 맵")


class Claims(BaseModel):
    """JWT 표준 클레임 + iroum-ax 확장 클레임 공유 구조체.

    Go 등가물: pkg/auth/claims.go

    # @MX:NOTE: [AUTO] Go-Python 공유 클레임 구조체 — 변경 시 pkg/auth/claims.go와 반드시 동기화
    # @MX:SPEC: SPEC-AX-AUTH-001 §2.3
    """

    # JWT sub 클레임 (Keycloak user ID)
    sub: str = Field(..., description="JWT sub 클레임")
    # JWT iss 클레임 (SF-1 검증 대상)
    iss: str = Field(..., description="JWT iss 클레임")
    # JWT jti 클레임 (블랙리스트 lookup 키)
    jti: str | None = Field(default=None, description="JWT jti 클레임")
    # JWT aud 클레임
    aud: list[str] = Field(default_factory=list, description="JWT aud 클레임")
    # space-separated scope 문자열
    scope: str = Field(default="", description="JWT scope 클레임")
    # Keycloak realm_access.roles (파싱 후 주입)
    roles: list[str] = Field(default_factory=list, description="역할 목록")
    # JWT exp (Unix timestamp)
    exp: int = Field(..., description="만료 Unix timestamp")
    # JWT iat (Unix timestamp)
    iat: int = Field(..., description="발급 Unix timestamp")
    # JWT nbf (옵션)
    nbf: int | None = Field(default=None, description="유효 시작 Unix timestamp")
