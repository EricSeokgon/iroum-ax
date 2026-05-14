"""보고서 초안 생성 오케스트레이터 — REQ-AX-004 Sprint 5 GREEN

파이프라인: PromptBuilder → LLMClient → StyleApplier
스타일 실패 시 최대 2회 재시도 (총 3회 시도).

# @MX:ANCHOR: [AUTO] ReportDrafter.draft_section — 초안 생성 파이프라인 진입점
# @MX:REASON: API 레이어, 통합 테스트, E2E 테스트 모두 이 메서드를 호출함
# @MX:SPEC: SPEC-AX-001 AC-004-1 / AC-004-2
"""
from __future__ import annotations

from pkg.models.criterion import Criterion
from pkg.models.report import DraftSection, GenerationResult, StyleReport

from pipelines.generation.llm_client import LLMClient
from pipelines.generation.prompt_builder import PromptBuilder
from pipelines.generation.style_applier import StyleApplier

# 스타일 검증 최대 재시도 횟수 (AC-004-2: 총 3회 = 1초 + 2재시도)
_MAX_RETRIES = 2


class ReportDrafter:
    """보고서 초안 생성 오케스트레이터."""

    def __init__(self) -> None:
        """ReportDrafter 초기화 — 내부 컴포넌트 조합."""
        self._prompt_builder = PromptBuilder()
        self._llm_client = LLMClient("http://localhost:8001/v1")
        self._style_applier = StyleApplier()

    def draft_section(
        self,
        criterion: Criterion,
        customer_content: str,
    ) -> DraftSection:
        """평가기준 1개 섹션의 한국어 공문 합니다체 초안을 생성한다.

        파이프라인:
        1. PromptBuilder.build() → 프롬프트 생성
        2. LLMClient.generate() → 텍스트 생성 (LLM 예외는 즉시 전파)
        3. StyleApplier.validate() → 합니다체 검증
        4. 실패 시 최대 _MAX_RETRIES회 재시도
        5. 재시도 소진 시 status='style_violation' 반환

        Args:
            criterion: 평가기준 (이름, 세부 내용 포함)
            customer_content: 고객사 실적 데이터 텍스트

        Returns:
            DraftSection (text, status, model_used, retry_count)
            - status='ok': 합니다체 검증 통과
            - status='style_violation': 최대 재시도 소진

        Raises:
            Exception: LLM 호출 예외는 재시도 없이 즉시 전파 (RD-5)
        """
        prompt = self._prompt_builder.build(criterion, customer_content)

        last_generation: GenerationResult | None = None
        last_style: StyleReport | None = None
        retry_count = 0

        for attempt in range(_MAX_RETRIES + 1):
            # LLM 호출 — 예외는 재시도 없이 즉시 전파 (RD-5)
            generation = self._llm_client.generate(prompt)
            last_generation = generation

            # 스타일 검증
            style_report = self._style_applier.validate(generation.text)
            last_style = style_report

            if style_report.is_official:
                # 합니다체 검증 통과
                return DraftSection(
                    text=generation.text,
                    status="ok",
                    model_used=generation.model_used,
                    retry_count=retry_count,
                    style_report=style_report,
                )

            # 마지막 시도가 아니면 재시도
            if attempt < _MAX_RETRIES:
                retry_count += 1

        # 최대 재시도 소진 → style_violation 반환
        return DraftSection(
            text=last_generation.text if last_generation else "",
            status="style_violation",
            model_used=last_generation.model_used if last_generation else "unknown",
            retry_count=retry_count,
            style_report=last_style,
        )
