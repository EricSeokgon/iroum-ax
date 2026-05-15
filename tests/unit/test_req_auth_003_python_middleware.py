"""SPEC-AX-AUTH-001 REQ-AUTH-003-E3 Python FastAPI 미들웨어 RED 테스트

Sprint 4 RED phase — 대부분의 테스트가 FAIL 상태여야 함.

대상:
- pipelines/auth/validator.py → TokenValidator.verify()
- pipelines/auth/dependencies.py → verify_token() FastAPI Depends
- REQ-AUTH-UBI-001 backward compat: auth_enabled=False → 'cli-anonymous' 반환

검증 항목:
- SF-1: iss 클레임 검증 (RFC 7519 §4.1.1)
- SF-2: alg/kty cross-check (Algorithm Confusion Attack 방어)
- 허용 알고리즘 allow-list: RS256/EdDSA/ES256 (HS256/none 거부)
- exp/aud/sig 실패 시 HTTPException 401 + WWW-Authenticate Bearer

환경 노트:
- fastapi, PyJWT가 설치되지 않은 경우 sys.modules mock으로 대체
- GREEN phase에서는 실제 패키지 설치 후 실제 JWT 검증으로 전환

# @MX:TODO: [AUTO] Sprint 4 RED — stub NotImplementedError로 FAIL 예상. GREEN에서 제거 예정.
# @MX:SPEC: SPEC-AX-AUTH-001 REQ-AUTH-003-E3
"""
from __future__ import annotations

import asyncio
import base64
import json
import sys
import time
from types import ModuleType
from typing import Any
from unittest.mock import MagicMock, patch

import pytest

# =============================================================================
# FastAPI mock 주입 — fastapi 미설치 환경 대응
# FastAPI HTTPException은 status_code + detail + headers 속성을 가진 예외 클래스
# =============================================================================


def _ensure_fastapi_mock() -> None:
    """fastapi가 없는 경우 sys.modules에 최소 mock 주입."""
    if "fastapi" not in sys.modules:
        class _HTTPException(Exception):
            """HTTPException mock — 실제 fastapi.HTTPException과 동일 인터페이스."""

            def __init__(
                self,
                status_code: int,
                detail: str = "",
                headers: dict[str, str] | None = None,
            ) -> None:
                self.status_code = status_code
                self.detail = detail
                self.headers = headers or {}

        class _Security:
            """fastapi.Security mock."""
            def __call__(self, *args: Any, **kwargs: Any) -> None:
                return None

        class _HTTPBearer:
            """fastapi.security.HTTPBearer mock."""
            def __init__(self, **kwargs: Any) -> None:
                pass

        class _HTTPAuthorizationCredentials:
            """HTTPAuthorizationCredentials mock."""
            def __init__(self, scheme: str = "Bearer", credentials: str = "") -> None:
                self.scheme = scheme
                self.credentials = credentials

        # fastapi 모듈 mock
        fastapi_mod = ModuleType("fastapi")
        fastapi_mod.HTTPException = _HTTPException  # type: ignore[attr-defined]
        fastapi_mod.Security = _Security()  # type: ignore[attr-defined]

        # fastapi.security 서브모듈 mock
        security_mod = ModuleType("fastapi.security")
        security_mod.HTTPBearer = _HTTPBearer  # type: ignore[attr-defined]
        security_mod.HTTPAuthorizationCredentials = _HTTPAuthorizationCredentials  # type: ignore[attr-defined]

        sys.modules["fastapi"] = fastapi_mod
        sys.modules["fastapi.security"] = security_mod


# 모듈 로딩 전에 mock 주입
_ensure_fastapi_mock()

# fastapi.HTTPException을 mock에서 가져옴 (실제 또는 mock)
from fastapi import HTTPException  # noqa: E402  (mock 주입 후 import)

# =============================================================================
# 테스트용 JWT 헬퍼 — PyJWT 없이 구조적 토큰 생성
# =============================================================================

_TEST_ISSUER = "https://keycloak.iroum-ax.internal/realms/iroum-ax"
_TEST_AUDIENCE = "iroum-ax-pipelines"
_OTHER_ISSUER = "https://other-realm.example.com/auth/realms/other"


def _make_jwt_payload(
    sub: str = "kepco-analyst-001",
    iss: str = _TEST_ISSUER,
    aud: str | list[str] = _TEST_AUDIENCE,
    exp_offset: int = 3600,
    alg: str = "RS256",
    kty: str = "RSA",
) -> str:
    """테스트용 구조적 JWT 문자열 생성 (서명 없음 — stub 테스트용).

    RED phase: stub이 서명 검증 전에 NotImplementedError를 던지므로 서명 유효성 무관.
    GREEN phase에서 실제 RSA 서명 토큰으로 대체됨.
    """
    now = int(time.time())
    header: dict[str, Any] = {
        "alg": alg,
        "typ": "JWT",
        "kid": "test-kid-001",
    }
    # SF-2 테스트용: kty를 커스텀 헤더 필드로 포함
    if kty != "RSA":  # RSA는 기본값이므로 다른 경우만 명시
        header["kty"] = kty
    else:
        header["kty"] = kty  # 항상 포함하여 SF-2 cross-check 가능하게

    payload: dict[str, Any] = {
        "sub": sub,
        "iss": iss,
        "aud": aud if isinstance(aud, list) else [aud],
        "exp": now + exp_offset,
        "iat": now - 10,
        "jti": f"jti-{sub}-001",
        "scope": "iroum-ax:analyst",
    }

    def _b64url(data: dict[str, Any]) -> str:
        return base64.urlsafe_b64encode(
            json.dumps(data, separators=(",", ":")).encode()
        ).rstrip(b"=").decode()

    # 가짜 서명 (stub은 이 값을 검증하지 않음 — NotImplementedError 먼저 반환)
    fake_sig = base64.urlsafe_b64encode(b"red-phase-fake-sig").rstrip(b"=").decode()
    return f"{_b64url(header)}.{_b64url(payload)}.{fake_sig}"


# =============================================================================
# 환경 설정 픽스처
# =============================================================================


@pytest.fixture()
def auth_enabled_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """AUTH_ENABLED=true 환경 변수 설정."""
    monkeypatch.setenv("AUTH_ENABLED", "true")
    monkeypatch.setenv("OIDC_ISSUER_URL", _TEST_ISSUER)
    monkeypatch.setenv("OIDC_AUDIENCE", _TEST_AUDIENCE)
    # settings 캐시 무효화 (pydantic-settings 캐시 때문에 필요)
    import importlib
    import pipelines.config.settings as cfg_mod
    importlib.reload(cfg_mod)


@pytest.fixture()
def auth_disabled_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """AUTH_ENABLED=false 환경 변수 설정 (backward compat 테스트)."""
    monkeypatch.setenv("AUTH_ENABLED", "false")
    monkeypatch.setenv("OIDC_ISSUER_URL", _TEST_ISSUER)
    monkeypatch.setenv("OIDC_AUDIENCE", _TEST_AUDIENCE)
    import importlib
    import pipelines.config.settings as cfg_mod
    importlib.reload(cfg_mod)


# =============================================================================
# TokenValidator 단위 테스트
# =============================================================================


class TestTokenValidatorVerify:
    """TokenValidator.verify() 단위 테스트 — Sprint 4 RED.

    현재 stub 상태: verify()가 NotImplementedError를 raise함.
    RED: 모든 성공 경로 테스트 FAIL, 에러 경로 테스트도 잘못된 예외 타입으로 FAIL.
    """

    def test_validator_verify_returns_validated_token_pydantic(
        self, auth_enabled_env: None
    ) -> None:
        """valid RS256 토큰 → ValidatedToken Pydantic 모델 반환.

        RED: stub이 NotImplementedError를 raise → FAIL.
        GREEN: 실제 PyJWT decode + RS256 검증 후 ValidatedToken 반환.
        """
        from pipelines.auth.validator import TokenValidator
        from pipelines.auth.models import ValidatedToken

        token = _make_jwt_payload(alg="RS256")
        validator = TokenValidator(oidc_issuer=_TEST_ISSUER, audience=_TEST_AUDIENCE)

        # RED: stub은 NotImplementedError → FAIL
        result = validator.verify(token)

        assert isinstance(result, ValidatedToken), "verify() 반환 타입이 ValidatedToken이어야 함"
        assert result.subject == "kepco-analyst-001"
        assert result.issuer == _TEST_ISSUER

    def test_validator_expired_token_raises_token_expired_error(
        self, auth_enabled_env: None
    ) -> None:
        """만료된 토큰(exp=now-100s) → TokenExpiredError 발생.

        RED: stub이 NotImplementedError → 잘못된 예외 타입으로 FAIL.
        GREEN: PyJWT exp 검증 후 TokenExpiredError 발생.
        """
        from pipelines.auth.validator import TokenValidator
        from pipelines.auth.errors import TokenExpiredError

        # exp_offset=-100 → 100초 전에 만료, clock skew 30초 초과
        token = _make_jwt_payload(exp_offset=-100)
        validator = TokenValidator(oidc_issuer=_TEST_ISSUER, audience=_TEST_AUDIENCE)

        with pytest.raises(TokenExpiredError):
            validator.verify(token)

    def test_validator_invalid_audience_raises_error(
        self, auth_enabled_env: None
    ) -> None:
        """aud 불일치 → TokenInvalidAudienceError 발생 (AC-AUTH-001-4).

        RED: stub이 NotImplementedError → 잘못된 예외 타입으로 FAIL.
        GREEN: PyJWT aud 검증 실패 시 TokenInvalidAudienceError 발생.
        """
        from pipelines.auth.validator import TokenValidator
        from pipelines.auth.errors import TokenInvalidAudienceError

        token = _make_jwt_payload(aud="some-other-service")
        validator = TokenValidator(oidc_issuer=_TEST_ISSUER, audience=_TEST_AUDIENCE)

        with pytest.raises(TokenInvalidAudienceError):
            validator.verify(token)

    def test_validator_iss_mismatch_raises_error_sf1(
        self, auth_enabled_env: None
    ) -> None:
        """iss 불일치 → TokenInvalidIssuerError 발생 (SF-1, RFC 7519 §4.1.1).

        RED: stub이 NotImplementedError → 잘못된 예외 타입으로 FAIL.
        GREEN: PyJWT iss 검증 실패 시 TokenInvalidIssuerError 발생.
        """
        from pipelines.auth.validator import TokenValidator
        from pipelines.auth.errors import TokenInvalidIssuerError

        # 다른 realm의 토큰 — cross-realm 공격 시뮬레이션
        token = _make_jwt_payload(iss=_OTHER_ISSUER)
        validator = TokenValidator(oidc_issuer=_TEST_ISSUER, audience=_TEST_AUDIENCE)

        with pytest.raises(TokenInvalidIssuerError):
            validator.verify(token)

    def test_validator_hs256_alg_not_allowed_raises_error(
        self, auth_enabled_env: None
    ) -> None:
        """alg=HS256 → AlgorithmNotAllowedError 발생 (REQ-AUTH-001-U1).

        RED: stub이 NotImplementedError → 잘못된 예외 타입으로 FAIL.
        GREEN: 허용 목록(RS256/EdDSA/ES256) 검사 후 AlgorithmNotAllowedError 발생.
        """
        from pipelines.auth.validator import TokenValidator
        from pipelines.auth.errors import AlgorithmNotAllowedError

        token = _make_jwt_payload(alg="HS256")
        validator = TokenValidator(oidc_issuer=_TEST_ISSUER, audience=_TEST_AUDIENCE)

        with pytest.raises(AlgorithmNotAllowedError):
            validator.verify(token)

    def test_validator_kty_alg_mismatch_raises_error_sf2(
        self, auth_enabled_env: None
    ) -> None:
        """kty=RSA + alg=ES256 → AlgorithmKeyMismatchError 발생 (SF-2).

        Algorithm Confusion Attack 변형 방어: RSA 키로 서명했지만
        alg 헤더가 EC 알고리즘을 주장하는 경우.

        RED: stub이 NotImplementedError → 잘못된 예외 타입으로 FAIL.
        GREEN: kty/alg cross-check 후 AlgorithmKeyMismatchError 발생.
        """
        from pipelines.auth.validator import TokenValidator
        from pipelines.auth.errors import AlgorithmKeyMismatchError

        # kty=RSA + alg=ES256 불일치 조합 (Algorithm Confusion Attack 변형)
        token = _make_jwt_payload(alg="ES256", kty="RSA")
        validator = TokenValidator(oidc_issuer=_TEST_ISSUER, audience=_TEST_AUDIENCE)

        with pytest.raises(AlgorithmKeyMismatchError):
            validator.verify(token)


# =============================================================================
# FastAPI verify_token Depends 단위 테스트
# =============================================================================


class TestVerifyTokenDepends:
    """verify_token() FastAPI Depends 단위 테스트 — Sprint 4 RED.

    asyncio.get_event_loop().run_until_complete()로 async 함수를 동기 실행.
    fastapi.HTTPException은 sys.modules mock으로 주입.
    """

    def _run(self, coro: Any) -> Any:
        """비동기 함수를 동기 컨텍스트에서 실행."""
        try:
            loop = asyncio.get_event_loop()
            if loop.is_closed():
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
        except RuntimeError:
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
        return loop.run_until_complete(coro)

    def test_verify_token_valid_rs256_returns_validated_token(
        self, auth_enabled_env: None
    ) -> None:
        """valid RS256 Bearer 토큰 → ValidatedToken 반환.

        RED: stub이 HTTPException(501) 반환 → FAIL (ValidatedToken이 아님).
        GREEN: 실제 검증 후 ValidatedToken 반환.
        """
        from pipelines.auth.dependencies import verify_token
        from pipelines.auth.models import ValidatedToken

        token_str = _make_jwt_payload(alg="RS256")
        mock_creds = MagicMock()
        mock_creds.credentials = token_str

        result = self._run(verify_token(credentials=mock_creds))

        assert isinstance(result, ValidatedToken), "verify_token()이 ValidatedToken을 반환해야 함"
        assert result.subject == "kepco-analyst-001"

    def test_verify_token_expired_returns_401(
        self, auth_enabled_env: None
    ) -> None:
        """만료 토큰 → HTTPException 401 + WWW-Authenticate Bearer.

        RED: stub이 HTTPException(501) 반환 → status_code 불일치로 FAIL.
        GREEN: TokenExpiredError → HTTPException(401) 변환.
        """
        from pipelines.auth.dependencies import verify_token

        expired_token = _make_jwt_payload(exp_offset=-100)
        mock_creds = MagicMock()
        mock_creds.credentials = expired_token

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=mock_creds))

        assert exc_info.value.status_code == 401, "만료 토큰은 401을 반환해야 함"
        assert "WWW-Authenticate" in exc_info.value.headers

    def test_verify_token_invalid_signature_returns_401(
        self, auth_enabled_env: None
    ) -> None:
        """서명 검증 실패 → HTTPException 401.

        RED: stub이 HTTPException(501) 반환 → status_code 불일치로 FAIL.
        GREEN: TokenInvalidSignatureError → HTTPException(401) 변환.
        """
        from pipelines.auth.dependencies import verify_token

        token = _make_jwt_payload(alg="RS256")
        mock_creds = MagicMock()
        mock_creds.credentials = token

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=mock_creds))

        assert exc_info.value.status_code == 401

    def test_verify_token_invalid_audience_returns_401(
        self, auth_enabled_env: None
    ) -> None:
        """aud 불일치 → HTTPException 401.

        RED: stub이 HTTPException(501) → FAIL.
        """
        from pipelines.auth.dependencies import verify_token

        token = _make_jwt_payload(aud="wrong-service")
        mock_creds = MagicMock()
        mock_creds.credentials = token

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=mock_creds))

        assert exc_info.value.status_code == 401

    def test_verify_token_iss_mismatch_returns_401(
        self, auth_enabled_env: None
    ) -> None:
        """iss 불일치 → HTTPException 401 (SF-1).

        RED: stub이 HTTPException(501) → FAIL.
        GREEN: TokenInvalidIssuerError → HTTPException(401) 변환.
        """
        from pipelines.auth.dependencies import verify_token

        token = _make_jwt_payload(iss=_OTHER_ISSUER)
        mock_creds = MagicMock()
        mock_creds.credentials = token

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=mock_creds))

        assert exc_info.value.status_code == 401

    def test_verify_token_alg_not_allowed_returns_401(
        self, auth_enabled_env: None
    ) -> None:
        """HS256 알고리즘 → HTTPException 401 (REQ-AUTH-001-U1).

        RED: stub이 HTTPException(501) → FAIL.
        GREEN: AlgorithmNotAllowedError → HTTPException(401) 변환.
        """
        from pipelines.auth.dependencies import verify_token

        token = _make_jwt_payload(alg="HS256")
        mock_creds = MagicMock()
        mock_creds.credentials = token

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=mock_creds))

        assert exc_info.value.status_code == 401

    def test_verify_token_missing_header_returns_401(
        self, auth_enabled_env: None
    ) -> None:
        """Authorization 헤더 누락 → HTTPException 401 + WWW-Authenticate.

        이 테스트는 stub에 이미 구현됨 → 현재 PASS.
        backward compat 회귀 가드.
        """
        from pipelines.auth.dependencies import verify_token

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=None))

        assert exc_info.value.status_code == 401
        assert "WWW-Authenticate" in exc_info.value.headers

    def test_verify_token_malformed_bearer_returns_401(
        self, auth_enabled_env: None
    ) -> None:
        """잘못된 JWT 형식(not.a.jwt) → HTTPException 401.

        RED: stub이 HTTPException(501) → FAIL.
        GREEN: JWT 파싱 실패 시 HTTPException(401) 변환.
        """
        from pipelines.auth.dependencies import verify_token

        mock_creds = MagicMock()
        mock_creds.credentials = "not.a.valid.jwt.token"

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=mock_creds))

        assert exc_info.value.status_code == 401

    def test_verify_token_auth_disabled_returns_anonymous(
        self, auth_disabled_env: None
    ) -> None:
        """AUTH_ENABLED=false → ValidatedToken(sub='cli-anonymous') 반환 (backward compat).

        이 경로는 stub에 이미 구현됨 → 현재 PASS.
        R-AUTH-007 backward compat 회귀 가드 — GREEN/REFACTOR에서도 유지 필수.
        """
        from pipelines.auth.dependencies import verify_token
        from pipelines.auth.models import ValidatedToken

        # auth disabled: credentials 없어도 OK
        result = self._run(verify_token(credentials=None))

        assert isinstance(result, ValidatedToken)
        assert result.subject == "cli-anonymous", (
            "AUTH_ENABLED=false 시 subject가 'cli-anonymous'여야 함 (R-AUTH-007)"
        )

    def test_verify_token_www_authenticate_header_format(
        self, auth_enabled_env: None
    ) -> None:
        """401 응답에 WWW-Authenticate: Bearer realm='iroum-ax' 헤더 포함.

        RED: stub이 501 반환 → FAIL.
        GREEN: 검증 실패 시 올바른 WWW-Authenticate 헤더 포함.
        """
        from pipelines.auth.dependencies import verify_token

        token = _make_jwt_payload(exp_offset=-200)
        mock_creds = MagicMock()
        mock_creds.credentials = token

        with pytest.raises(HTTPException) as exc_info:
            self._run(verify_token(credentials=mock_creds))

        assert exc_info.value.status_code == 401
        www_auth = exc_info.value.headers.get("WWW-Authenticate", "")
        assert "Bearer" in www_auth, "WWW-Authenticate 헤더에 'Bearer' 포함되어야 함"
        assert "iroum-ax" in www_auth, "WWW-Authenticate 헤더에 realm='iroum-ax' 포함되어야 함"
