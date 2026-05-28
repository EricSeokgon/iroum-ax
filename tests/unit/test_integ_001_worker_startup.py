"""SPEC-AX-INTEG-001 — Celery worker 부팅 검증 단위 테스트

REQ-INTEG-006 / REQ-INTEG-008 / AC-INTEG-001-7 검증:
- GO_CONTROL_PLANE_URL 누락·빈 문자열 → 부팅 실패
- VLLM_ENDPOINT 외부 URL → 부팅 실패 (allowlist 검증)
- 정상 환경변수 — go_control_plane_url 속성 접근 통과
"""
from __future__ import annotations

import pytest
from pipelines.config.settings import Settings, validate_llm_endpoint
from pkg.errors.custom_errors import ExternalLLMBlockedError


class TestGoControlPlaneURLSetting:
    """REQ-INTEG-006 — GO_CONTROL_PLANE_URL 환경변수 도입"""

    def test_default_value_is_empty(self) -> None:
        """기본값은 빈 문자열 — GO_CONTROL_PLANE_URL 미설정 시 worker 부팅 거부 (REQ-INTEG-006)"""
        s = Settings()
        assert s.go_control_plane_url == ""

    def test_env_var_overrides_default(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """GO_CONTROL_PLANE_URL 환경변수가 기본값을 override"""
        monkeypatch.setenv("GO_CONTROL_PLANE_URL", "http://localhost:9090")
        s = Settings()
        assert s.go_control_plane_url == "http://localhost:9090"


class TestVLLMEndpointAllowlist:
    """REQ-INTEG-008 / REQ-UBI-001 — VLLM_ENDPOINT localhost-only 검증"""

    def test_localhost_url_passes(self) -> None:
        """http://localhost:8000 — allowlist 통과"""
        assert validate_llm_endpoint("http://localhost:8000") is True

    def test_127_0_0_1_passes(self) -> None:
        """http://127.0.0.1:8000 — allowlist 통과"""
        assert validate_llm_endpoint("http://127.0.0.1:8000") is True

    def test_ipv6_localhost_passes(self) -> None:
        """http://[::1]:8000 — IPv6 localhost allowlist 통과"""
        assert validate_llm_endpoint("http://[::1]:8000") is True

    def test_external_host_raises(self) -> None:
        """외부 호스트 — ExternalLLMBlockedError 발생"""
        with pytest.raises(ExternalLLMBlockedError) as exc_info:
            validate_llm_endpoint("http://api.openai.com/v1")
        assert "openai" in str(exc_info.value).lower() or "차단" in str(exc_info.value)

    def test_evil_lookalike_raises(self) -> None:
        """localhost.attacker.com 같은 유사 도메인도 거부"""
        with pytest.raises(ExternalLLMBlockedError):
            validate_llm_endpoint("http://localhost.attacker.com/")


class TestWorkerStartupValidation:
    """AC-INTEG-001-7 — Celery worker 부팅 시점 검증 헬퍼"""

    def test_validate_worker_environment_passes_for_localhost(self) -> None:
        """validate_worker_environment(): localhost VLLM + GO_CONTROL_PLANE_URL 정상 → True"""
        # GREEN 단계 헬퍼 import (모듈은 GREEN에서 작성)
        from pipelines.config.celery_client import validate_worker_environment

        assert (
            validate_worker_environment(
                vlm_endpoint="http://localhost:8000",
                go_control_plane_url="http://localhost:8080",
            )
            is True
        )

    def test_validate_worker_environment_rejects_external_vlm(self) -> None:
        """외부 VLM endpoint → ExternalLLMBlockedError"""
        from pipelines.config.celery_client import validate_worker_environment

        with pytest.raises(ExternalLLMBlockedError):
            validate_worker_environment(
                vlm_endpoint="https://api.openai.com/v1",
                go_control_plane_url="http://localhost:8080",
            )

    def test_validate_worker_environment_rejects_empty_go_url(self) -> None:
        """GO_CONTROL_PLANE_URL 빈 문자열 → ValueError"""
        from pipelines.config.celery_client import validate_worker_environment

        with pytest.raises(ValueError, match=r"GO_CONTROL_PLANE_URL"):
            validate_worker_environment(
                vlm_endpoint="http://localhost:8000",
                go_control_plane_url="",
            )

    def test_validate_worker_environment_accepts_empty_vlm(self) -> None:
        """VLM endpoint 빈 문자열 — transformers 직접 로딩 모드 (CPU 환경) → 통과

        REQ-INTEG-008 검증은 LLM endpoint가 *설정된 경우*에만 발동. 빈 문자열은
        opt-out으로 해석 (settings.py:124-127 — '비어있으면 transformers 직접 로딩').
        """
        from pipelines.config.celery_client import validate_worker_environment

        # 빈 VLM endpoint는 valid — transformers 직접 로딩 모드
        assert (
            validate_worker_environment(
                vlm_endpoint="",
                go_control_plane_url="http://localhost:8080",
            )
            is True
        )

    def test_validate_worker_environment_from_env_rejects_external_vllm(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """env-layer: VLLM_ENDPOINT(두 개 L)에 외부 URL → ExternalLLMBlockedError (REQ-UBI-001)."""
        from pipelines.config.celery_client import validate_worker_environment_from_env
        from pkg.errors.custom_errors import ExternalLLMBlockedError

        monkeypatch.setenv("VLLM_ENDPOINT", "https://external-llm.example.com/v1")
        monkeypatch.setenv("GO_CONTROL_PLANE_URL", "http://localhost:8080")

        with pytest.raises(ExternalLLMBlockedError):
            validate_worker_environment_from_env()

    def test_validate_worker_environment_from_env_rejects_missing_go_url(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """env-layer: GO_CONTROL_PLANE_URL 미설정 → ValueError (REQ-INTEG-006)."""
        from pipelines.config.celery_client import validate_worker_environment_from_env

        monkeypatch.delenv("GO_CONTROL_PLANE_URL", raising=False)
        monkeypatch.setenv("VLLM_ENDPOINT", "http://localhost:8000/v1")

        with pytest.raises(ValueError, match="GO_CONTROL_PLANE_URL"):
            validate_worker_environment_from_env()

    def test_validate_worker_environment_from_env_passes_valid_env(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """env-layer: 유효한 환경변수 조합 → True (정상 부팅 경로)."""
        from pipelines.config.celery_client import validate_worker_environment_from_env

        monkeypatch.setenv("VLLM_ENDPOINT", "http://127.0.0.1:8000/v1")
        monkeypatch.setenv("GO_CONTROL_PLANE_URL", "http://localhost:8080")

        assert validate_worker_environment_from_env() is True
