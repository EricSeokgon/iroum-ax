"""보고서 초안 생성 오케스트레이터 스텁 — Sprint 5 GREEN Phase에서 구현 예정 (REQ-AX-004)

# @MX:TODO: [AUTO] ReportDrafter.draft_section() 미구현 — Sprint 5 GREEN에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-004 / AC-004-1 / AC-004-2 / AC-004-3
"""
from __future__ import annotations

from pkg.models.criterion import Criterion
from pkg.models.report import DraftSection
from pipelines.generation.llm_client import LLMClient
from pipelines.generation.prompt_builder import PromptBuilder
from pipelines.generation.style_applier import StyleApplier


class ReportDrafter:
    """보고서 초안 생성 오케스트레이터.

    파이프라인: PromptBuilder → LLMClient → StyleApplier
    스타일 실패 시 최대 2회 재시도 (총 3회 시도).

    # @MX:ANCHOR: [AUTO] ReportDrafter.draft_section — 초안 생성 파이프라인 진입점
    # @MX:REASON: API 레이어, 통합 테스트, E2E 테스트 모두 이 메서드를 호출함
    # @MX:SPEC: SPEC-AX-001 AC-004-1 / AC-004-2
    """

    def __init__(self) -> None:
        """ReportDrafter 초기화 — 내부 컴포넌트 조합."""
        # Sprint 5 GREEN에서 실제 엔드포인트로 초기화
        self._prompt_builder = PromptBuilder()
        self._llm_client = LLMClient("http://localhost:8001/v1")
        self._style_applier = StyleApplier()

    def draft_section(
        self,
        criterion: Criterion,  # noqa: ARG002
        customer_content: str,  # noqa: ARG002
    ) -> DraftSection:
        """평가기준 1개 섹션의 한국어 공문 합니다체 초안을 생성한다.

        Args:
            criterion: 평가기준 (이름, 세부 내용 포함)
            customer_content: 고객사 실적 데이터 텍스트

        Returns:
            DraftSection (text, status, model_used, retry_count)
            - status='ok': 합니다체 검증 통과
            - status='style_violation': 최대 재시도 소진

        Raises:
            NotImplementedError: Sprint 5 GREEN에서 구현 예정
        """
        raise NotImplementedError("Sprint 5 GREEN Phase에서 구현 예정")  # noqa: EM101
