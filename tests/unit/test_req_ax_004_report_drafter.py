"""REQ-AX-004 ReportDrafter 단위 테스트 — RED Phase

Sprint 5: 보고서 초안 생성 오케스트레이터 계약 검증.
ReportDrafter.draft_section(criterion, customer_content) → DraftSection

AC-004-1: 정상 합니다체 초안 생성 → status='ok'
AC-004-2: 스타일 위반 → 최대 2 재시도 → 3회 총 시도 후 status='style_violation'
AC-004-3: model_used 메타데이터 추적

파이프라인 순서: prompt_builder → llm_client → style_applier
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
def sample_criterion() -> Criterion:
    """안전보건 평가기준 픽스처."""
    return Criterion(
        id="crit-001",
        criterion_name="안전교육 이수율",
        criterion_detail="전 임직원 대상 안전보건 교육 이수율 평가.",
        max_points=5,
    )


@pytest.fixture()
def passing_style_report() -> StyleReport:
    """합니다체 검증 통과 StyleReport."""
    return StyleReport(
        is_official=True,
        honorific_score=0.95,
        violations=[],
    )


@pytest.fixture()
def failing_style_report() -> StyleReport:
    """합니다체 검증 실패 StyleReport."""
    return StyleReport(
        is_official=False,
        honorific_score=0.3,
        violations=["casual_ending"],
    )


@pytest.fixture()
def passing_generation_result() -> GenerationResult:
    """합니다체 응답 GenerationResult."""
    return GenerationResult(
        text="안전교육 이수율 100%를 달성하였습니다.",
        model_used="qwen2.5-7b",
        tokens=42,
        latency_ms=320,
    )


@pytest.fixture()
def failing_generation_result() -> GenerationResult:
    """해체 응답 GenerationResult (스타일 실패)."""
    return GenerationResult(
        text="안전교육 실시했어. 이번에 잘 됐어.",
        model_used="qwen2.5-7b",
        tokens=20,
        latency_ms=200,
    )


# ============================================================
# RD-1: 정상 합니다체 → status='ok'
# ============================================================


class TestReportDrafterHappyPath:
    """정상 초안 생성 — AC-004-1."""

    def test_draft_section_returns_draft_section_instance(
        self,
        sample_criterion: Criterion,
        passing_generation_result: GenerationResult,
        passing_style_report: StyleReport,
    ) -> None:
        """draft_section()은 DraftSection 인스턴스를 반환해야 한다 (RD-1).

        반환 타입 계약 위반은 API 레이어를 즉시 깨뜨린다.
        """
        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "모의 프롬프트"
            mock_llm.generate.return_value = passing_generation_result
            mock_sa.validate.return_value = passing_style_report

            result = drafter.draft_section(sample_criterion, "이수율 100%")
        assert isinstance(result, DraftSection)

    def test_draft_section_status_is_ok_on_success(
        self,
        sample_criterion: Criterion,
        passing_generation_result: GenerationResult,
        passing_style_report: StyleReport,
    ) -> None:
        """style_applier 통과 시 status='ok'이어야 한다 (RD-1, AC-004-1).

        status='ok'이어야 downstream이 결과를 소비할 수 있다.
        """
        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "모의 프롬프트"
            mock_llm.generate.return_value = passing_generation_result
            mock_sa.validate.return_value = passing_style_report

            result = drafter.draft_section(sample_criterion, "이수율 100%")
        assert result.status == "ok"

    def test_draft_section_model_used_is_tracked(
        self,
        sample_criterion: Criterion,
        passing_generation_result: GenerationResult,
        passing_style_report: StyleReport,
    ) -> None:
        """DraftSection에 model_used 필드가 채워져야 한다 (RD-4, AC-004-3).

        model_used 없이는 AC-004-3 fallback 추적이 불가능하다.
        """
        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "모의 프롬프트"
            mock_llm.generate.return_value = passing_generation_result
            mock_sa.validate.return_value = passing_style_report

            result = drafter.draft_section(sample_criterion, "이수율 100%")
        assert result.model_used, f"model_used가 비어 있음: '{result.model_used}'"


# ============================================================
# RD-2: 첫 style 실패 → 재시도 → 통과
# ============================================================


class TestReportDrafterRetrySuccess:
    """스타일 실패 후 재시도 성공 — AC-004-2."""

    def test_draft_section_retries_on_style_failure(
        self,
        sample_criterion: Criterion,
        passing_generation_result: GenerationResult,
        failing_generation_result: GenerationResult,
        passing_style_report: StyleReport,
        failing_style_report: StyleReport,
    ) -> None:
        """첫 번째 시도 style 실패 → 재시도 → 성공 시 status='ok' (RD-2, AC-004-2).

        재시도 없으면 일시적 스타일 실패 때마다 초안 생성이 중단된다.
        """
        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "모의 프롬프트"
            # 첫 번째 생성: 해체 응답 / 두 번째 생성: 합니다체 응답
            mock_llm.generate.side_effect = [
                failing_generation_result,
                passing_generation_result,
            ]
            # 첫 번째 검증: 실패 / 두 번째 검증: 통과
            mock_sa.validate.side_effect = [failing_style_report, passing_style_report]

            result = drafter.draft_section(sample_criterion, "이수율 100%")

        assert result.status == "ok"
        assert result.retry_count == 1


# ============================================================
# RD-3: 모든 재시도 실패 → status='style_violation'
# ============================================================


class TestReportDrafterRetryExhausted:
    """최대 재시도 소진 → style_violation — AC-004-2."""

    def test_draft_section_returns_style_violation_after_max_retries(
        self,
        sample_criterion: Criterion,
        failing_generation_result: GenerationResult,
        failing_style_report: StyleReport,
    ) -> None:
        """3회(1초 + 2재시도) 모두 style 실패 시 status='style_violation' (RD-3, AC-004-2).

        예외를 던지지 않고 structured result를 반환해야
        API 레이어가 응답을 클라이언트에 전달할 수 있다.
        """
        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "모의 프롬프트"
            # 3회 모두 해체 응답
            mock_llm.generate.return_value = failing_generation_result
            # 3회 모두 스타일 실패
            mock_sa.validate.return_value = failing_style_report

            result = drafter.draft_section(sample_criterion, "이수율 100%")

        assert result.status == "style_violation", (
            f"max 재시도 후 style_violation이 아님: status='{result.status}'"
        )

    def test_draft_section_max_retry_count_is_two(
        self,
        sample_criterion: Criterion,
        failing_generation_result: GenerationResult,
        failing_style_report: StyleReport,
    ) -> None:
        """최대 재시도 횟수는 2이어야 한다 (RD-3, AC-004-2 + strategy §5).

        v0.1.2 Unwanted 절: 총 3회 시도 = 1초 + 2재시도.
        retry_count > 2이면 SLA를 초과할 수 있다.
        """
        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier") as mock_sa,
        ):
            mock_pb.build.return_value = "모의 프롬프트"
            mock_llm.generate.return_value = failing_generation_result
            mock_sa.validate.return_value = failing_style_report

            result = drafter.draft_section(sample_criterion, "이수율 100%")

        assert result.retry_count == 2, (
            f"최대 재시도 횟수가 2가 아님: retry_count={result.retry_count}"
        )


# ============================================================
# RD-5: LLM 예외 → 재시도 없이 예외 전파
# ============================================================


class TestReportDrafterLLMException:
    """LLM 예외는 재시도 없이 전파 (RD-5)."""

    def test_llm_exception_propagates_without_retry(
        self,
        sample_criterion: Criterion,
    ) -> None:
        """LLM 호출 예외는 재시도 없이 즉시 전파되어야 한다 (RD-5).

        스타일 실패 재시도와 LLM 장애 재시도를 혼동하면
        LLM 장애 시 불필요한 재호출로 부하가 발생한다.
        """
        drafter = ReportDrafter()
        with (
            patch.object(drafter, "_prompt_builder") as mock_pb,
            patch.object(drafter, "_llm_client") as mock_llm,
            patch.object(drafter, "_style_applier"),
        ):
            mock_pb.build.return_value = "모의 프롬프트"
            mock_llm.generate.side_effect = RuntimeError("LLM 서버 연결 실패")

            with pytest.raises(RuntimeError, match="LLM 서버 연결 실패"):
                drafter.draft_section(sample_criterion, "이수율 100%")

        # LLM이 1회만 호출되어야 함 (재시도 없음)
        assert mock_llm.generate.call_count == 1
