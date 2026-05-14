"""REQ-AX-004 PromptBuilder 단위 테스트 — RED Phase

Sprint 5: 보고서 초안 생성 프롬프트 빌더 계약 검증.
PromptBuilder.build(criterion, customer_content) → str

AC-004-1: 평가지표 1개 초안 생성 — 프롬프트 구성이 올바른지 검증
"""
from __future__ import annotations

import pytest
from pipelines.generation.prompt_builder import PromptBuilder  # type: ignore[import]
from pkg.models.criterion import Criterion

# ============================================================
# 테스트 픽스처
# ============================================================


@pytest.fixture()
def sample_criterion() -> Criterion:
    """안전보건 평가기준 픽스처."""
    return Criterion(
        id="crit-001",
        criterion_name="안전교육 이수율",
        criterion_detail="전 임직원 대상 안전보건 교육 이수율 평가. 연간 이수율 95% 이상 목표.",
        max_points=5,
        parent_criterion_id=None,
    )


@pytest.fixture()
def minimal_criterion() -> Criterion:
    """criterion_detail 없는 최소 기준 픽스처."""
    return Criterion(
        id="crit-002",
        criterion_name="안전사고 발생건수",
        criterion_detail="",
        max_points=None,
    )


@pytest.fixture()
def builder() -> PromptBuilder:
    """PromptBuilder 인스턴스."""
    return PromptBuilder()


# ============================================================
# PB-1: criterion_name이 프롬프트에 포함되는지 확인
# ============================================================


class TestPromptBuilderCriterionInjection:
    """평가기준 컨텍스트 주입 검증."""

    def test_build_includes_criterion_name(
        self, builder: PromptBuilder, sample_criterion: Criterion
    ) -> None:
        """프롬프트에 평가기준 이름이 포함되어야 한다 (AC-004-1).

        평가기준 이름이 프롬프트에 없으면 LLM이 어떤 항목에 대해
        작성하는지 알 수 없어 초안 품질이 보장되지 않는다.
        """
        prompt = builder.build(sample_criterion, "안전교육 이수율 100% 달성")
        assert "안전교육 이수율" in prompt

    def test_build_includes_criterion_detail(
        self, builder: PromptBuilder, sample_criterion: Criterion
    ) -> None:
        """프롬프트에 평가기준 세부 내용이 포함되어야 한다.

        세부 내용(criterion_detail)은 LLM이 평가 맥락을 이해하는 데 필수다.
        """
        prompt = builder.build(sample_criterion, "안전교육 이수율 100% 달성")
        assert "95%" in prompt or "안전보건 교육" in prompt

    # PB-2: customer_content 포함 확인
    def test_build_includes_customer_content(
        self, builder: PromptBuilder, sample_criterion: Criterion
    ) -> None:
        """프롬프트에 고객 콘텐츠가 포함되어야 한다 (AC-004-1).

        고객 콘텐츠(자사 실적 데이터)가 없으면 LLM이 조작된 내용을
        생성할 수 있어 공문 신뢰도를 해친다.
        """
        customer_content = "KEPCO E&C 2025년 안전교육 이수율 100% 달성. 전 임직원 500명 이수."
        prompt = builder.build(sample_criterion, customer_content)
        assert "KEPCO E&C" in prompt or "안전교육 이수율 100%" in prompt


# ============================================================
# PB-3: 한국어 공문 스타일 지시문 포함 확인
# ============================================================


class TestPromptBuilderStyleInstruction:
    """한국어 공문 합니다체 스타일 지시문 검증."""

    def test_build_includes_korean_official_style_instruction(
        self, builder: PromptBuilder, sample_criterion: Criterion
    ) -> None:
        """프롬프트에 합니다체 또는 공문 작성 지시가 포함되어야 한다 (AC-004-1, AC-004-2).

        스타일 지시가 없으면 LLM이 해체/구어체로 응답할 수 있고
        style_applier 검증 실패 확률이 올라간다.
        """
        prompt = builder.build(sample_criterion, "안전교육 실적")
        # 합니다체, 공문, 경어, 존댓말 중 하나 이상 언급
        style_keywords = ["합니다체", "공문", "경어", "존댓말", "합니다", "입니다"]
        assert any(kw in prompt for kw in style_keywords), (
            f"프롬프트에 한국어 공문 스타일 지시가 없음. 프롬프트: {prompt[:200]}"
        )

    def test_build_returns_nonempty_string(
        self, builder: PromptBuilder, sample_criterion: Criterion
    ) -> None:
        """build() 결과가 빈 문자열이 아니어야 한다.

        빈 프롬프트는 LLM 호출 자체를 무의미하게 만든다.
        """
        prompt = builder.build(sample_criterion, "테스트 콘텐츠")
        assert isinstance(prompt, str)
        assert len(prompt.strip()) > 0


# ============================================================
# PB-4: criterion_detail 없는 경우 — 예외 없이 처리
# ============================================================


class TestPromptBuilderEdgeCases:
    """엣지 케이스 — 빈 입력 처리."""

    def test_build_with_empty_criterion_detail_does_not_raise(
        self, builder: PromptBuilder, minimal_criterion: Criterion
    ) -> None:
        """criterion_detail이 비어 있어도 예외 없이 프롬프트를 생성해야 한다 (PB-4).

        평가편람에 세부 내용이 없는 기준도 존재한다.
        """
        prompt = builder.build(minimal_criterion, "안전사고 발생건수 0건")
        assert isinstance(prompt, str)
        assert len(prompt.strip()) > 0
        assert "안전사고 발생건수" in prompt

    # PB-5: customer_content 빈 문자열
    def test_build_with_empty_customer_content_does_not_raise(
        self, builder: PromptBuilder, sample_criterion: Criterion
    ) -> None:
        """customer_content가 빈 문자열이어도 예외 없이 프롬프트를 생성해야 한다 (PB-5).

        고객 데이터가 아직 없는 초기 상태에서도 프롬프트 구성이 가능해야 한다.
        """
        prompt = builder.build(sample_criterion, "")
        assert isinstance(prompt, str)
        assert len(prompt.strip()) > 0
        assert "안전교육 이수율" in prompt
