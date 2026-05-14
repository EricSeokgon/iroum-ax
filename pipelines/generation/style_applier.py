"""한국어 공문 합니다체 검증기 — REQ-AX-004 Sprint 5 GREEN

# @MX:ANCHOR: [AUTO] StyleApplier.validate — 공문 스타일 검증 핵심 함수
# @MX:REASON: ReportDrafter retry loop, 통합 테스트, 단위 테스트 모두 이 메서드를 호출함
# @MX:SPEC: SPEC-AX-001 AC-004-1 / AC-004-2
"""
from __future__ import annotations

import re

from pkg.models.report import StyleReport

# 합니다체 종결어미 패턴 목록 (문장 내 어디서나 매칭)
# 니다로 끝나는 모든 합니다체를 포괄 (씁니다, 갑니다 등 포함)
_HONORIFIC_ENDINGS = (
    "겠습니다", "하였습니다", "였습니다", "이었습니다",
    "드립니다", "올립니다", "바랍니다",
    "습니다", "입니다", "합니다", "됩니다", "니다",
)

# 해체/구어체 종결어미 패턴 목록
_CASUAL_ENDINGS = (
    "했어요", "이에요", "거예요", "네요", "군요",
    "했어", "이야", "거야", "잖아요", "잖아",
    "았어", "었어", "겠어", "죠", "지요",
    "됐어", "됐어요", "실시했어",
)

# 한국어 문자 존재 여부
_KOREAN_PATTERN = re.compile(r"[가-힣]")


def _split_segments(text: str) -> list[str]:
    """텍스트를 문장 단위로 분리한다 (구두점 기준).

    예: '입니다. 씁니다. 보고합니다.' → ['입니다', '씁니다', '보고합니다']
    """
    # 마침표/느낌표/물음표로 분리, 쉼표는 문장 내부 구분자로 처리
    parts = re.split(r"[.!?]+", text)
    return [p.strip() for p in parts if p.strip()]


def _has_honorific(segment: str) -> bool:
    """문장 세그먼트에 합니다체 종결어미가 포함되어 있는지 확인."""
    return any(ending in segment for ending in _HONORIFIC_ENDINGS)


def _has_casual(segment: str) -> bool:
    """문장 세그먼트에 해체/구어체 종결어미가 포함되어 있는지 확인."""
    return any(ending in segment for ending in _CASUAL_ENDINGS)


class StyleApplier:
    """한국어 공문 합니다체 스타일 검증기."""

    def validate(self, text: str) -> StyleReport:
        """한국어 공문 합니다체 적합 여부 검증.

        검증 항목:
        - 빈 텍스트 거부 (empty_text)
        - 영어 단독 텍스트 거부 (no_korean)
        - 합니다체 종결어미 비율 계산 (honorific_score)
        - 해체/구어체 종결어미 감지 (casual_ending)
        - is_official: honorific_score >= 0.6이고 위반 없는 경우 True

        Args:
            text: 검증할 텍스트

        Returns:
            StyleReport (is_official, honorific_score, violations)
        """
        violations: list[str] = []

        # 빈 텍스트 검사
        if not text or not text.strip():
            return StyleReport(
                is_official=False,
                honorific_score=0.0,
                violations=["empty_text"],
            )

        # 한국어 문자 존재 여부 검사
        if not _KOREAN_PATTERN.search(text):
            violations.append("no_korean")
            return StyleReport(
                is_official=False,
                honorific_score=0.0,
                violations=violations,
            )

        # 문장 단위로 분리
        segments = _split_segments(text)
        total = len(segments)

        if total == 0:
            return StyleReport(
                is_official=False,
                honorific_score=0.0,
                violations=["empty_text"],
            )

        # 합니다체 / 해체 문장 수 계산
        honorific_count = sum(1 for s in segments if _has_honorific(s))
        has_casual_text = any(_has_casual(s) for s in segments)

        honorific_score = honorific_count / total
        # 1.0 초과 방지
        honorific_score = min(honorific_score, 1.0)

        # 해체/구어체 감지
        if has_casual_text:
            violations.append("casual_ending")

        # is_official: 합니다체 비율 >= 0.6이고 위반 없음
        is_official = honorific_score >= 0.6 and len(violations) == 0

        return StyleReport(
            is_official=is_official,
            honorific_score=round(honorific_score, 6),
            violations=violations,
        )
