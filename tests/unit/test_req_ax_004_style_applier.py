"""REQ-AX-004 StyleApplier 단위 테스트 — RED Phase

Sprint 5: 한국어 공문 합니다체 검증기 계약 검증.
StyleApplier.validate(text: str) → StyleReport

AC-004-1: 합니다체 style_applier 검증 통과
AC-004-2: 스타일 위반 감지 → 재시도 루프 트리거
"""
from __future__ import annotations

import pytest
from pipelines.generation.style_applier import StyleApplier  # type: ignore[import]
from pkg.models.report import StyleReport

# ============================================================
# 테스트 픽스처
# ============================================================


@pytest.fixture()
def applier() -> StyleApplier:
    """StyleApplier 인스턴스."""
    return StyleApplier()


# 합니다체 텍스트 모음
HONORIFIC_TEXTS = [
    "안전교육을 실시하였습니다.",
    "안전보건 관리체계를 도입하였습니다.",
    "합니다체 문장입니다. 이렇게 씁니다.",
    "안전교육을 실시하였습니다. 됩니다. 입니다.",
]

# 해체/구어체 텍스트 모음
CASUAL_TEXTS = [
    "실시했어요, 수행했습니다 혼재.",
    "안전교육 실시했어 이번에.",
    "이번에 안전교육 실시했어. 잘 됐어.",
]


# ============================================================
# SA-1, SA-2: 합니다체 → is_official=True, honorific_score >= 0.8
# ============================================================


class TestStyleApplierHonoricTexts:
    """합니다체 텍스트 정상 감지 검증."""

    @pytest.mark.parametrize("text", [
        "안전교육을 실시하였습니다.",
        "안전보건 관리체계를 도입하였습니다.",
    ])
    def test_honorific_text_returns_is_official_true(
        self, applier: StyleApplier, text: str
    ) -> None:
        """합니다체 문장은 is_official=True를 반환해야 한다 (SA-1, SA-2).

        AC-004-1: style_applier 검증 통과 = is_official True.
        """
        result = applier.validate(text)
        assert isinstance(result, StyleReport)
        assert result.is_official is True, (
            f"합니다체 문장인데 is_official=False 반환. text='{text}'"
        )

    @pytest.mark.parametrize("text", [
        "안전교육을 실시하였습니다.",
        "안전보건 관리체계를 도입하였습니다.",
    ])
    def test_honorific_text_has_high_honorific_score(
        self, applier: StyleApplier, text: str
    ) -> None:
        """합니다체 문장은 honorific_score >= 0.8이어야 한다 (SA-1, SA-2).

        점수가 낮으면 경계 케이스에서 오탐이 발생할 수 있다.
        """
        result = applier.validate(text)
        assert result.honorific_score >= 0.8, (
            f"합니다체 문장의 honorific_score가 너무 낮음: {result.honorific_score}. text='{text}'"
        )

    # SA-6: 다중 합니다체 문장
    def test_multiple_honorific_sentences_all_pass(
        self, applier: StyleApplier
    ) -> None:
        """여러 합니다체 문장으로 구성된 텍스트는 is_official=True 반환 (SA-6).

        실제 공문은 다수 문장으로 구성되므로 단일 문장 테스트만으로는 부족하다.
        """
        text = "합니다체 문장입니다. 이렇게 씁니다. 보고합니다."
        result = applier.validate(text)
        assert result.is_official is True
        assert result.honorific_score >= 0.8

    # SA-8: 100% 합니다체 → honorific_score == 1.0
    def test_all_honorific_sentences_score_is_one(
        self, applier: StyleApplier
    ) -> None:
        """모든 문장이 합니다체이면 honorific_score == 1.0 (SA-8).

        100% 합니다체 텍스트의 경우 최고 점수를 반환해야 한다.
        """
        text = "안전교육을 실시하였습니다. 됩니다. 입니다."
        result = applier.validate(text)
        assert result.honorific_score == pytest.approx(1.0, abs=0.05), (
            f"완전 합니다체인데 honorific_score != 1.0: {result.honorific_score}"
        )


# ============================================================
# SA-3, SA-4: 해체/구어체 → is_official=False
# ============================================================


class TestStyleApplierCasualTexts:
    """해체/구어체 텍스트 위반 감지 검증."""

    def test_mixed_casual_and_honorific_returns_is_official_false(
        self, applier: StyleApplier
    ) -> None:
        """해체와 합니다체 혼재 텍스트는 is_official=False 반환 (SA-3, AC-004-2).

        공문은 일관된 합니다체를 요구한다. 혼재는 위반이다.
        """
        text = "실시했어요, 수행했습니다 혼재."
        result = applier.validate(text)
        assert result.is_official is False, (
            f"해체/합니다체 혼재인데 is_official=True 반환. text='{text}'"
        )

    def test_pure_casual_text_returns_is_official_false(
        self, applier: StyleApplier
    ) -> None:
        """순수 해체 텍스트는 is_official=False 반환 (SA-4).

        구어체 단독 사용은 공문 위반이다.
        """
        text = "안전교육 실시했어 이번에."
        result = applier.validate(text)
        assert result.is_official is False, (
            f"해체 텍스트인데 is_official=True 반환. text='{text}'"
        )

    @pytest.mark.parametrize("text", CASUAL_TEXTS)
    def test_casual_texts_have_violations(
        self, applier: StyleApplier, text: str
    ) -> None:
        """해체/구어체 텍스트는 violations 리스트가 비어 있지 않아야 한다.

        violations가 비어 있으면 ReportDrafter가 재시도 이유를 알 수 없다.
        """
        result = applier.validate(text)
        assert len(result.violations) > 0, (
            f"위반 텍스트인데 violations 빈 리스트. text='{text}'"
        )


# ============================================================
# SA-5: 영어 단독 텍스트 → is_official=False, no_korean 위반
# ============================================================


class TestStyleApplierEnglishOnlyTexts:
    """영어 단독 텍스트 거부 검증."""

    def test_english_only_text_returns_is_official_false(
        self, applier: StyleApplier
    ) -> None:
        """영어 단독 텍스트(한국어 0%)는 is_official=False를 반환해야 한다 (SA-5).

        AC-UBI-002: 한국어가 없는 텍스트는 공문으로 인정하지 않는다.
        """
        text = "Safety training was conducted."
        result = applier.validate(text)
        assert result.is_official is False

    def test_english_only_text_has_no_korean_violation(
        self, applier: StyleApplier
    ) -> None:
        """영어 단독 텍스트는 violations에 'no_korean'이 포함되어야 한다 (SA-5).

        no_korean 위반은 ReportDrafter가 프롬프트에 한국어 지시를 강화하도록
        피드백 신호를 제공한다.
        """
        text = "Safety training was conducted."
        result = applier.validate(text)
        assert "no_korean" in result.violations, (
            f"영어 단독 텍스트인데 no_korean 위반 없음. violations={result.violations}"
        )


# ============================================================
# SA-7: 빈 문자열 → is_official=False, honorific_score=0.0
# ============================================================


class TestStyleApplierEmptyText:
    """빈 텍스트 처리 검증."""

    def test_empty_text_returns_is_official_false(
        self, applier: StyleApplier
    ) -> None:
        """빈 문자열은 is_official=False를 반환해야 한다 (SA-7).

        빈 텍스트는 공문 요건을 충족할 수 없다.
        """
        result = applier.validate("")
        assert result.is_official is False

    def test_empty_text_has_zero_honorific_score(
        self, applier: StyleApplier
    ) -> None:
        """빈 문자열은 honorific_score=0.0이어야 한다 (SA-7).

        분석할 텍스트가 없으면 점수가 0이다.
        """
        result = applier.validate("")
        assert result.honorific_score == pytest.approx(0.0)

    def test_empty_text_has_empty_text_violation(
        self, applier: StyleApplier
    ) -> None:
        """빈 문자열은 violations에 'empty_text'가 포함되어야 한다 (SA-7).

        빈 텍스트 위반 코드로 ReportDrafter가 원인을 식별할 수 있어야 한다.
        """
        result = applier.validate("")
        assert "empty_text" in result.violations, (
            f"빈 텍스트인데 empty_text 위반 없음. violations={result.violations}"
        )

    def test_validate_returns_style_report_instance(
        self, applier: StyleApplier
    ) -> None:
        """validate()는 StyleReport 인스턴스를 반환해야 한다.

        반환 타입 계약 검증 — dict나 None 반환은 파이프라인을 깨뜨린다.
        """
        result = applier.validate("안전교육을 실시하였습니다.")
        assert isinstance(result, StyleReport)
