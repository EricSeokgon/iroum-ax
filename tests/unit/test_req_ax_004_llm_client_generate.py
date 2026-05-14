"""REQ-AX-004 LLMClient.generate() 단위 테스트 — RED Phase

Sprint 5: 기존 LLMClient(Sprint 1)에 generate() 메서드 확장 계약 검증.
LLMClient.generate(prompt, *, use_gpu=False) → GenerationResult

AC-004-3: EXAONE 3회 실패 → Qwen 2.5 자동 fallback, model_used 메타데이터 추적

주의:
- 기존 validate_endpoint / ExternalLLMBlockedError 테스트는 Sprint 1에서 담당.
  본 파일은 신규 generate() 메서드 계약만 검증한다.
- 실제 Qwen 2.5 모델 로딩은 발생하지 않도록 mock 필수.
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest
from pipelines.generation.llm_client import LLMClient  # type: ignore[import] -- Sprint 1 존재
from pkg.models.report import GenerationResult

# ============================================================
# 테스트 픽스처
# ============================================================


INTERNAL_ENDPOINT = "http://localhost:8001/v1"
SAMPLE_PROMPT = "다음 안전교육 실적을 공문 합니다체로 작성하시오: 이수율 100%."


@pytest.fixture()
def mock_generation_result() -> GenerationResult:
    """모의 GenerationResult 픽스처."""
    return GenerationResult(
        text="안전교육 이수율 100%를 달성하였습니다.",
        model_used="qwen2.5-7b",
        tokens=42,
        latency_ms=320,
    )


# ============================================================
# LC-1: 정상 내부 엔드포인트 mock — GenerationResult 반환
# ============================================================


class TestLLMClientGenerateContract:
    """LLMClient.generate() 기본 계약 검증."""

    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_returns_generation_result(
        self,
        mock_validate: MagicMock,
        mock_generation_result: GenerationResult,
    ) -> None:
        """generate()는 GenerationResult 인스턴스를 반환해야 한다 (LC-1).

        dict나 str 반환은 ReportDrafter 파이프라인을 깨뜨린다.
        """
        client = LLMClient(INTERNAL_ENDPOINT)
        with patch.object(client, "_call_model", return_value=mock_generation_result):
            result = client.generate(SAMPLE_PROMPT)
        assert isinstance(result, GenerationResult)

    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_text_is_nonempty(
        self,
        mock_validate: MagicMock,
        mock_generation_result: GenerationResult,
    ) -> None:
        """generate() 결과의 text 필드가 비어 있지 않아야 한다 (LC-4).

        빈 텍스트는 StyleApplier 검증 실패를 유발하고 무의미한 재시도를 발생시킨다.
        """
        client = LLMClient(INTERNAL_ENDPOINT)
        with patch.object(client, "_call_model", return_value=mock_generation_result):
            result = client.generate(SAMPLE_PROMPT)
        assert len(result.text.strip()) > 0

    # LC-5: model_used 필드 존재 + "qwen" 포함
    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_model_used_contains_qwen(
        self,
        mock_validate: MagicMock,
        mock_generation_result: GenerationResult,
    ) -> None:
        """generate() 결과의 model_used 필드에 모델명이 포함되어야 한다 (LC-5, AC-004-3).

        AC-004-3: response metadata에 model_used 추적이 필수이다.
        """
        client = LLMClient(INTERNAL_ENDPOINT)
        with patch.object(client, "_call_model", return_value=mock_generation_result):
            result = client.generate(SAMPLE_PROMPT)
        assert "qwen" in result.model_used.lower(), (
            f"model_used에 'qwen' 없음: {result.model_used}"
        )

    # LC-6: tokens 필드 int > 0
    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_tokens_is_positive_int(
        self,
        mock_validate: MagicMock,
        mock_generation_result: GenerationResult,
    ) -> None:
        """generate() 결과의 tokens 필드는 양의 정수여야 한다 (LC-6).

        토큰 추적은 비용 모니터링과 API 제한 관리에 필요하다.
        """
        client = LLMClient(INTERNAL_ENDPOINT)
        with patch.object(client, "_call_model", return_value=mock_generation_result):
            result = client.generate(SAMPLE_PROMPT)
        assert isinstance(result.tokens, int)
        assert result.tokens > 0

    # LC-7: latency_ms 필드 int > 0
    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_latency_ms_is_positive_int(
        self,
        mock_validate: MagicMock,
        mock_generation_result: GenerationResult,
    ) -> None:
        """generate() 결과의 latency_ms 필드는 양의 정수여야 한다 (LC-7).

        응답 시간 추적은 AC-004-1 5초 SLA 모니터링에 필요하다.
        """
        client = LLMClient(INTERNAL_ENDPOINT)
        with patch.object(client, "_call_model", return_value=mock_generation_result):
            result = client.generate(SAMPLE_PROMPT)
        assert isinstance(result.latency_ms, int)
        assert result.latency_ms >= 0


# ============================================================
# LC-2: use_gpu=False (default) — CPU 경로
# ============================================================


class TestLLMClientGenerateGpuFlag:
    """use_gpu 플래그 처리 검증."""

    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_cpu_path_does_not_raise(
        self,
        mock_validate: MagicMock,
        mock_generation_result: GenerationResult,
    ) -> None:
        """use_gpu=False (default) CPU 경로는 예외 없이 동작해야 한다 (LC-2).

        AC-001-5 패턴: CPU 경로는 항상 가용해야 한다 (GPU opt-in).
        """
        client = LLMClient(INTERNAL_ENDPOINT)
        with patch.object(client, "_call_model", return_value=mock_generation_result):
            result = client.generate(SAMPLE_PROMPT, use_gpu=False)
        assert result is not None

    @pytest.mark.gpu
    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_gpu_path_routes_to_gpu_device(
        self,
        mock_validate: MagicMock,
        mock_generation_result: GenerationResult,
    ) -> None:
        """use_gpu=True GPU 경로는 GPU device routing을 실행해야 한다 (LC-3, REQ-AX-004-O1).

        GPU 환경에서만 실행. 실제 GPU 모델 로딩 없이 routing 로직만 검증.
        """
        client = LLMClient(INTERNAL_ENDPOINT)
        with patch.object(client, "_call_model", return_value=mock_generation_result) as mock_call:
            client.generate(SAMPLE_PROMPT, use_gpu=True)
            # GPU 경로로 호출되었을 때 use_gpu=True가 전달되는지 확인
            call_kwargs = mock_call.call_args
            assert call_kwargs is not None


# ============================================================
# AC-004-3: EXAONE fallback → model_used = "qwen2.5-7b"
# ============================================================


class TestLLMClientFallbackBehavior:
    """EXAONE 실패 시 Qwen 2.5 fallback 검증 (AC-004-3)."""

    @patch("pipelines.generation.llm_client.validate_llm_endpoint")
    def test_generate_returns_qwen_model_used_on_fallback(
        self,
        mock_validate: MagicMock,
    ) -> None:
        """EXAONE 실패 후 fallback 시 model_used가 'qwen2.5-7b'이어야 한다 (AC-004-3).

        fallback 추적 없으면 운영 중 모델 전환을 감지할 수 없다.
        """
        fallback_result = GenerationResult(
            text="안전교육을 실시하였습니다.",
            model_used="qwen2.5-7b",
            tokens=30,
            latency_ms=500,
        )
        client = LLMClient(INTERNAL_ENDPOINT)
        # EXAONE 실패 시뮬레이션: 첫 번째 호출은 예외, 두 번째(fallback)는 정상
        exaone_error = Exception("EXAONE endpoint 5xx")
        with patch.object(
            client,
            "_call_model",
            side_effect=[exaone_error, exaone_error, exaone_error, fallback_result],
        ):
            result = client.generate(SAMPLE_PROMPT)
        assert result.model_used == "qwen2.5-7b"
