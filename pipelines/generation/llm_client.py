"""LLM 클라이언트 — 외부 엔드포인트 차단 포함 (REQ-UBI-001, AC-UBI-001)

실제 HTTP 호출 전에 allowlist 검증을 수행한다.
"""
from __future__ import annotations

from pipelines.config.settings import validate_llm_endpoint


class LLMClient:
    """LLM 엔드포인트 래퍼 — 외부 URL 차단 포함

    # @MX:NOTE: [AUTO] 외부 LLM 차단 — REQ-UBI-001 데이터 주권
    # @MX:SPEC: SPEC-AX-001 REQ-UBI-001 / AC-UBI-001
    """

    def __init__(self, endpoint: str) -> None:
        """LLMClient 초기화.

        Args:
            endpoint: LLM 서버 엔드포인트 URL (내부 서버여야 함)
        """
        self._endpoint = endpoint

    def generate(self, prompt: str, **kwargs: object) -> str:
        """프롬프트로 텍스트를 생성한다.

        내부 allowlist에 없는 외부 엔드포인트이면 실제 HTTP 연결 전에 차단한다.

        Args:
            prompt: 입력 프롬프트 텍스트
            **kwargs: 추가 생성 옵션

        Returns:
            생성된 텍스트

        Raises:
            ExternalLLMBlockedError: 외부 엔드포인트 사용 시도 시
        """
        # 외부 엔드포인트 검증 — ExternalLLMBlockedError 발생 시 전파
        validate_llm_endpoint(self._endpoint)

        # 내부 엔드포인트만 도달 가능한 영역
        # Sprint 2+에서 실제 HTTP 호출 구현 예정
        raise NotImplementedError("실제 LLM 호출은 Sprint 2에서 구현됩니다.")  # noqa: EM101
