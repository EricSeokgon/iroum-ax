"""한국어 공문 합니다체 검증기 스텁 — Sprint 5 GREEN Phase에서 구현 예정 (REQ-AX-004)

# @MX:TODO: [AUTO] StyleApplier.validate() 미구현 — Sprint 5 GREEN에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-004 / AC-004-1 / AC-004-2
"""
from __future__ import annotations

from pkg.models.report import StyleReport


class StyleApplier:
    """한국어 공문 합니다체 스타일 검증기.

    # @MX:ANCHOR: [AUTO] StyleApplier.validate — 공문 스타일 검증 핵심 함수
    # @MX:REASON: ReportDrafter retry loop, 통합 테스트, 단위 테스트 모두 이 메서드를 호출함
    # @MX:SPEC: SPEC-AX-001 AC-004-1 / AC-004-2
    """

    def validate(self, text: str) -> StyleReport:  # noqa: ARG002
        """한국어 공문 합니다체 적합 여부 검증.

        검증 항목:
        - 합니다체 종결어미 (습니다, 입니다, 됩니다, 합니다) 감지
        - 해체/구어체 종결 (해, 이야, 했어, 했어요) 감지
        - 영어 단독 텍스트 거부 (no_korean)
        - 빈 텍스트 거부 (empty_text)

        Args:
            text: 검증할 텍스트

        Returns:
            StyleReport (is_official, honorific_score, violations)

        Raises:
            NotImplementedError: Sprint 5 GREEN에서 구현 예정
        """
        raise NotImplementedError("Sprint 5 GREEN Phase에서 구현 예정")  # noqa: EM101
