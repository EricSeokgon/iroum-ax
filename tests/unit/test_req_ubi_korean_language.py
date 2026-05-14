"""AC-UBI-002: 한국어 우선 처리 — 언어 감지 및 출력 언어 검증 테스트

REQ-UBI-002: The system SHALL process Korean text as the primary language for all
input parsing, RAG retrieval, draft generation, and recommendation output.

# @MX:TODO: [AUTO] AC-UBI-002 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-UBI-002 / AC-UBI-002
"""
from __future__ import annotations

import pytest


# =============================================================================
# 테스트 픽스처
# =============================================================================


@pytest.fixture()
def korean_dominant_text() -> str:
    """한국어 비율 90% 이상 텍스트 (정상 케이스)"""
    return (
        "안전보건 관리체계 구축 및 운영에 관한 실적보고서입니다. "
        "KEPCO E&C는 ISO 45001 인증을 획득하였으며, "
        "안전교육 이수율 및 재해율 감소를 위한 다양한 활동을 수행하였습니다. "
        "본 보고서는 경영평가 안전보건 항목에 대한 자사 실적을 종합한 것입니다."
    )


@pytest.fixture()
def english_only_text() -> str:
    """순수 영문 텍스트 (한국어 비율 0%)"""
    return (
        "Safety Management System Report. "
        "This document describes ISO 45001 compliance activities. "
        "All training completion rates are above the target threshold."
    )


@pytest.fixture()
def low_korean_ratio_text() -> str:
    """한국어 비율 15% 텍스트 (low_korean_ratio 경고 대상)"""
    return (
        "Safety Report Q1 2025. Training: KOSHA, ISO 45001. "
        "Completion: 98%. 안전 OK."
    )


# =============================================================================
# AC-UBI-002 테스트 케이스
# =============================================================================


class TestKoreanLanguagePrimary:
    """REQ-UBI-002 한국어 우선 처리 검증"""

    def test_korean_text_detection_should_return_ko_lang_code(
        self, korean_dominant_text: str
    ) -> None:
        """한국어 지배적 텍스트 입력 시 lang_code가 'ko'를 반환해야 한다.

        Given: 한국어 비율 90% 이상 텍스트
        When: detect_language() 호출
        Then: 반환값의 lang_code == 'ko'
        """
        from pipelines.ingestion.language_detector import detect_language  # type: ignore[import]

        result = detect_language(korean_dominant_text)
        assert result["lang_code"] == "ko"

    def test_korean_text_detection_should_return_korean_ratio_above_threshold(
        self, korean_dominant_text: str
    ) -> None:
        """한국어 지배적 텍스트의 korean_ratio가 0.2 초과를 반환해야 한다.

        Given: 한국어 비율 90% 이상 텍스트
        When: detect_language() 호출
        Then: 반환값의 korean_ratio > 0.2
        """
        from pipelines.ingestion.language_detector import detect_language  # type: ignore[import]

        result = detect_language(korean_dominant_text)
        assert result["korean_ratio"] > 0.2

    def test_english_only_input_should_set_low_korean_ratio_flag(
        self, english_only_text: str
    ) -> None:
        """순수 영문 입력 시 parse_quality_flag에 'low_korean_ratio'가 설정되어야 한다.

        Given: 한국어 비율 0% 순수 영문 텍스트
        When: detect_language() 호출
        Then: 반환값의 parse_quality_flag == 'low_korean_ratio'
             (시스템은 거부하지 않고 degraded confidence flag 처리)
        """
        from pipelines.ingestion.language_detector import detect_language  # type: ignore[import]

        result = detect_language(english_only_text)
        assert result.get("parse_quality_flag") == "low_korean_ratio"

    def test_low_korean_ratio_input_should_set_low_korean_ratio_flag(
        self, low_korean_ratio_text: str
    ) -> None:
        """한국어 비율 20% 미만 입력 시 low_korean_ratio 플래그가 설정되어야 한다.

        Given: 한국어 비율 15% 텍스트
        When: detect_language() 호출
        Then: parse_quality_flag == 'low_korean_ratio'
        """
        from pipelines.ingestion.language_detector import detect_language  # type: ignore[import]

        result = detect_language(low_korean_ratio_text)
        assert result.get("parse_quality_flag") == "low_korean_ratio"

    def test_low_korean_ratio_should_not_reject_input_should_return_result(
        self, english_only_text: str
    ) -> None:
        """한국어 비율 낮은 입력도 거부하지 않고 결과를 반환해야 한다.

        Given: 순수 영문 텍스트 (한국어 0%)
        When: detect_language() 호출
        Then: 예외 발생 없이 결과 dict 반환 (degraded confidence flag만 기록)
        """
        from pipelines.ingestion.language_detector import detect_language  # type: ignore[import]

        # 거부하지 않고 결과를 반환해야 함 — 예외 발생 시 테스트 실패
        result = detect_language(english_only_text)
        assert isinstance(result, dict)
        assert "lang_code" in result

    def test_language_warning_audit_event_should_persist_for_low_korean_ratio(
        self, english_only_text: str
    ) -> None:
        """한국어 비율 낮은 입력 감지 시 audit_logs에 'language_warning' 이벤트가 기록되어야 한다.

        Given: 순수 영문 텍스트 감지 결과
        When: 언어 경고 감사 이벤트 기록
        Then: audit_logs에 action='language_warning' 레코드 INSERT
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            user_id="cli-anonymous",
            action="language_warning",
            resource_id="00000000-0000-0000-0000-000000000002",
            resource_type="document",
            details={"parse_quality_flag": "low_korean_ratio"},
        )
        assert result is not None
        assert result["action"] == "language_warning"
