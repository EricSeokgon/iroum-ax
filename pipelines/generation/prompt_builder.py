"""프롬프트 빌더 — REQ-AX-004 Sprint 5 GREEN

# @MX:ANCHOR: [AUTO] PromptBuilder.build — 프롬프트 구성 핵심 함수
# @MX:REASON: ReportDrafter, 통합 테스트, 단위 테스트 모두 이 메서드를 호출함
# @MX:SPEC: SPEC-AX-001 AC-004-1
"""
from __future__ import annotations

from pkg.models.criterion import Criterion


class PromptBuilder:
    """평가기준별 한국어 공문 합니다체 프롬프트 빌더."""

    def build(self, criterion: Criterion, customer_content: str) -> str:
        """평가기준 + 고객 콘텐츠 → 한국어 공문 합니다체 프롬프트.

        Args:
            criterion: 평가기준 (이름, 세부 내용, 배점 포함)
            customer_content: 고객사 실적 데이터 텍스트

        Returns:
            LLM 입력용 프롬프트 문자열
        """
        # 세부 내용이 있는 경우 컨텍스트 추가
        detail_section = ""
        if criterion.criterion_detail:
            detail_section = f"\n평가기준 세부 내용:\n{criterion.criterion_detail}\n"

        # 고객 콘텐츠 섹션
        customer_section = ""
        if customer_content:
            customer_section = f"\n고객사 실적 데이터:\n{customer_content}\n"

        style_instruction = (
            "다음 경영평가 실적보고서 섹션을 한국어 공문 합니다체"
            "(습니다, 입니다, 합니다, 됩니다)로 작성하시오."
        )
        prompt = (
            f"{style_instruction}\n"
            "반드시 경어체와 존댓말을 사용하여 공식 문서 형식으로 작성하십시오.\n"
            "\n"
            f"평가기준명: {criterion.criterion_name}\n"
            f"{detail_section}"
            f"{customer_section}"
            "\n"
            "위 내용을 바탕으로 공문 합니다체 초안을 작성하시오."
        )
        return prompt
