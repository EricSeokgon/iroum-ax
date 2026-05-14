"""프롬프트 빌더 스텁 — Sprint 5 GREEN Phase에서 구현 예정 (REQ-AX-004)

# @MX:TODO: [AUTO] PromptBuilder.build() 미구현 — Sprint 5 GREEN에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-004 / AC-004-1
"""
from __future__ import annotations

from pkg.models.criterion import Criterion


class PromptBuilder:
    """평가기준별 한국어 공문 합니다체 프롬프트 빌더.

    # @MX:ANCHOR: [AUTO] PromptBuilder.build — 프롬프트 구성 핵심 함수
    # @MX:REASON: ReportDrafter, 통합 테스트, 단위 테스트 모두 이 메서드를 호출함
    # @MX:SPEC: SPEC-AX-001 AC-004-1
    """

    def build(self, criterion: Criterion, customer_content: str) -> str:  # noqa: ARG002
        """평가기준 + 고객 콘텐츠 → 한국어 공문 합니다체 프롬프트.

        Args:
            criterion: 평가기준 (이름, 세부 내용, 배점 포함)
            customer_content: 고객사 실적 데이터 텍스트

        Returns:
            LLM 입력용 프롬프트 문자열

        Raises:
            NotImplementedError: Sprint 5 GREEN에서 구현 예정
        """
        raise NotImplementedError("Sprint 5 GREEN Phase에서 구현 예정")  # noqa: EM101
