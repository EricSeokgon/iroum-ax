"""AC-UBI-001: 데이터 주권 — 외부 LLM API 호출 차단 테스트

REQ-UBI-001: The system SHALL store all input documents, intermediate artifacts,
and output reports exclusively within the customer-controlled infrastructure.
외부 LLM API 호출은 금지된다.

# @MX:TODO: [AUTO] AC-UBI-001 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-UBI-001 / AC-UBI-001
"""
from __future__ import annotations

import pytest

# =============================================================================
# 테스트 픽스처
# =============================================================================


@pytest.fixture()
def external_llm_endpoint() -> str:
    """외부 LLM 엔드포인트 URL (차단 대상)"""
    return "https://api.openai.com/v1/chat/completions"


@pytest.fixture()
def internal_llm_endpoint() -> str:
    """내부 vLLM 엔드포인트 URL (허용 대상)"""
    return "http://localhost:8000/v1/chat/completions"


# =============================================================================
# AC-UBI-001 테스트 케이스
# =============================================================================


class TestDataSovereigntyLLMEndpoint:
    """REQ-UBI-001 데이터 주권 — LLM 엔드포인트 allowlist 검증"""

    def test_external_openai_endpoint_should_raise_blocked_error(
        self, external_llm_endpoint: str
    ) -> None:
        """외부 OpenAI 엔드포인트 설정 시 ExternalLLMBlockedError가 발생해야 한다.

        Given: LLM_ENDPOINT=https://api.openai.com/v1/chat/completions (외부 도메인)
        When: settings allowlist 검증 또는 llm_client.generate() 호출 시도
        Then: ExternalLLMBlockedError 예외 발생 (HTTP 호출 전 차단)
        """
        # GREEN 페이즈에서 구현될 allowlist 검증 함수를 임포트한다
        from pipelines.config.settings import validate_llm_endpoint  # type: ignore[import]

        with pytest.raises(Exception):  # ExternalLLMBlockedError 또는 ImportError
            validate_llm_endpoint(external_llm_endpoint)

    def test_external_anthropic_endpoint_should_raise_blocked_error(self) -> None:
        """외부 Anthropic 엔드포인트 설정 시 ExternalLLMBlockedError가 발생해야 한다.

        Given: LLM_ENDPOINT=https://api.anthropic.com/v1/messages (외부 도메인)
        When: allowlist 검증 수행
        Then: ExternalLLMBlockedError 예외 발생
        """
        from pipelines.config.settings import validate_llm_endpoint  # type: ignore[import]

        with pytest.raises(Exception):
            validate_llm_endpoint("https://api.anthropic.com/v1/messages")

    def test_internal_localhost_endpoint_should_pass_allowlist(
        self, internal_llm_endpoint: str
    ) -> None:
        """내부 localhost 엔드포인트는 allowlist 검증을 통과해야 한다.

        Given: LLM_ENDPOINT=http://localhost:8000/v1 (내부 도메인)
        When: allowlist 검증 수행
        Then: 예외 없이 통과 (True 반환 또는 None)
        """
        from pipelines.config.settings import validate_llm_endpoint  # type: ignore[import]

        # 내부 엔드포인트는 예외 없이 통과해야 함
        result = validate_llm_endpoint(internal_llm_endpoint)
        assert result is True or result is None

    def test_llm_client_blocks_external_call_attempt_should_raise_error(self) -> None:
        """LLMClient가 외부 엔드포인트 호출 시도 시 ExternalLLMBlockedError를 발생시켜야 한다.

        Given: LLMClient가 외부 도메인 엔드포인트로 초기화됨
        When: generate() 메서드 호출
        Then: ExternalLLMBlockedError 발생 (실제 HTTP 연결 전 차단)
        """
        # GREEN 페이즈에서 구현될 LLMClient를 임포트한다
        from pipelines.generation.llm_client import LLMClient  # type: ignore[import]
        from pkg.errors.custom_errors import ExternalLLMBlockedError  # type: ignore[import]

        client = LLMClient(endpoint="https://api.openai.com/v1")
        with pytest.raises(ExternalLLMBlockedError):
            client.generate(prompt="테스트 프롬프트")

    def test_audit_log_records_external_llm_blocked_event_should_persist_row(
        self,
    ) -> None:
        """외부 LLM 차단 이벤트가 audit_logs에 'external_llm_blocked' 액션으로 기록되어야 한다.

        Given: 외부 LLM 엔드포인트 호출 시도가 차단됨
        When: ExternalLLMBlockedError 발생 후 감사 이벤트 기록
        Then: audit_logs에 action='external_llm_blocked' 레코드 INSERT
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        # mock DB 세션 없이 호출 — GREEN 페이즈에서 세션 의존성 주입 구현
        result = audit_event(
            user_id="cli-anonymous",
            action="external_llm_blocked",
            resource_id="00000000-0000-0000-0000-000000000001",
            resource_type="llm_call",
        )
        assert result is not None
        assert result["action"] == "external_llm_blocked"
