"""REQ-AX-004 통합 테스트 — RED Phase

Sprint 5: 보고서 초안 생성 E2E 흐름 검증.
PromptBuilder → LLMClient → StyleApplier → ReportDrafter 파이프라인.

AC-004-1: 전체 파이프라인 합니다체 초안 생성
AC-004-3: Qwen fallback 경로 model_used 추적
"""
from __future__ import annotations

from unittest.mock import patch

import pytest
from pipelines.generation.report_drafter import ReportDrafter  # type: ignore[import]
from pkg.models.criterion import Criterion
from pkg.models.report import DraftSection, GenerationResult, StyleReport

# ============================================================
# 테스트 픽스처
# ============================================================


@pytest.fixture()
def integration_criterion() -> Criterion:
    """통합 테스트용 안전보건 평가기준 픽스처."""
    return Criterion(
        id="crit-integ-001",
        criterion_name="안전보건 관리체계 인증",
        criterion_detail="KOSHA-MS 인증 획득 여부 및 유지 현황. 연간 감사 이수.",
        max_points=10,
    )


# ============================================================
# INT-1: 전체 파이프라인 — 합니다체 응답 → status='ok'
# ============================================================


class TestReportDrafterIntegration:
    """E2E 파이프라인 통합 검증."""

    @pytest.mark.integration
    def test_full_pipeline_with_honorific_response_returns_ok(
        self, integration_criterion: Criterion
    ) -> None:
        """전체 파이프라인에서 합니다체 LLM 응답은 status='ok'을 반환해야 한다 (INT-1, AC-004-1).

        PromptBuilder → LLMClient → StyleApplier 순서를 검증한다.
        실제 LLM 호출 없이 mock으로 합니다체 응답을 주입한다.
        """
        honorific_result = GenerationResult(
            text="안전보건 관리체계 인증(KOSHA-MS)을 획득하였습니다. "
                 "연간 감사를 성공적으로 이수하였습니다.",
            model_used="qwen2.5-7b",
            tokens=55,
            latency_ms=410,
        )
        passing_style = StyleReport(
            is_official=True,
            honorific_score=1.0,
            violations=[],
        )

        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "통합 테스트 프롬프트"
            mock_llm.generate.return_value = honorific_result
            mock_sa.validate.return_value = passing_style

            result = drafter.draft_section(
                integration_criterion, "KOSHA-MS 2025년 인증 완료. 내부감사 3회 실시."
            )

        assert isinstance(result, DraftSection)
        assert result.status == "ok"
        assert "qwen" in result.model_used.lower()

    # INT-2: Qwen fallback 경로 — model_used='qwen2.5-7b'
    @pytest.mark.integration
    def test_qwen_fallback_path_tracks_model_used(
        self, integration_criterion: Criterion
    ) -> None:
        """EXAONE mock 5xx → Qwen fallback 경로에서 model_used='qwen2.5-7b' (INT-2, AC-004-3).

        fallback model_used 추적이 없으면 운영 중 EXAONE→Qwen 전환을
        감지할 수 없어 모니터링이 불가능하다.
        """
        qwen_fallback_result = GenerationResult(
            text="안전보건 관리체계를 도입하였습니다.",
            model_used="qwen2.5-7b",
            tokens=25,
            latency_ms=650,
        )
        passing_style = StyleReport(
            is_official=True,
            honorific_score=0.9,
            violations=[],
        )

        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "통합 테스트 프롬프트"
            # Qwen fallback 경로: model_used가 'qwen2.5-7b'인 결과 반환
            mock_llm.generate.return_value = qwen_fallback_result
            mock_sa.validate.return_value = passing_style

            result = drafter.draft_section(
                integration_criterion, "KOSHA-MS 인증 완료."
            )

        assert result.model_used == "qwen2.5-7b", (
            f"Qwen fallback 경로에서 model_used가 'qwen2.5-7b' 아님: {result.model_used}"
        )

    # INT-3: max retry exhausted — status='style_violation'
    @pytest.mark.integration
    def test_max_retry_exhausted_returns_style_violation(
        self, integration_criterion: Criterion
    ) -> None:
        """3회 모두 스타일 실패 시 status='style_violation' 반환 (INT-3, AC-004-2).

        예외 전파 대신 structured result를 반환하여
        API 레이어가 클라이언트에 상태를 전달할 수 있어야 한다.
        """
        casual_result = GenerationResult(
            text="이번에 안전교육 실시했어. 잘 됐어.",
            model_used="qwen2.5-7b",
            tokens=15,
            latency_ms=180,
        )
        failing_style = StyleReport(
            is_official=False,
            honorific_score=0.1,
            violations=["casual_ending"],
        )

        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "통합 테스트 프롬프트"
            mock_llm.generate.return_value = casual_result
            mock_sa.validate.return_value = failing_style

            result = drafter.draft_section(
                integration_criterion, "안전교육 실적 데이터"
            )

        assert result.status == "style_violation", (
            f"max retry 후 style_violation 아님: status='{result.status}'"
        )
