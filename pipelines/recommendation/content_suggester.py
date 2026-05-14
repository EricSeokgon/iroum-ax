"""콘텐츠 제안기 — GapItem 기반 A 등급 벤치마크 콘텐츠 제안 (REQ-AX-005)

# @MX:ANCHOR: [AUTO] ContentSuggester.suggest — ContentSuggestion 파이프라인 진입점
# @MX:REASON: Prioritizer, 통합 테스트가 suggest() 반환 값을 소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-005 / AC-005-2 / AC-005-3
"""
from __future__ import annotations

from typing import Protocol, runtime_checkable

from pkg.models.criterion import CriterionMatch
from pkg.models.recommendation import ContentSuggestion, GapItem


@runtime_checkable
class RetrieverProtocol(Protocol):
    """ContentSuggester가 의존하는 Retriever 인터페이스.

    # @MX:NOTE: [AUTO] mock 호환을 위해 runtime_checkable Protocol 사용
    """

    def retrieve(self, query: str, top_k: int = 3) -> list[CriterionMatch]:
        """쿼리로 유사 평가기준을 검색해 CriterionMatch 목록을 반환한다."""
        ...

# retriever.retrieve() 호출당 반환받을 최대 결과 수
_RETRIEVE_TOP_K: int = 3


class ContentSuggester:
    """GapItem 목록을 받아 Retriever로 A 등급 벤치마크 콘텐츠를 조회하고
    ContentSuggestion 목록을 반환한다.

    의존성 주입: retriever는 suggest() 호출 시 파라미터로 전달받는다.
    이를 통해 테스트에서 mock retriever를 쉽게 주입할 수 있다 (AC-005-2, AC-005-3).
    """

    def suggest(
        self,
        gaps: list[GapItem],
        retriever: RetrieverProtocol,
    ) -> list[ContentSuggestion]:
        """갭 항목마다 Retriever로 관련 A 등급 콘텐츠를 검색해 ContentSuggestion을 반환한다.

        Args:
            gaps: GapAnalyzer.analyze()가 반환한 GapItem 목록
            retriever: retrieve(query, top_k) 메서드를 가진 검색기 객체 (Retriever 또는 mock)

        Returns:
            ContentSuggestion 목록.
            - Retriever 결과가 없으면 빈 리스트 반환 (AC-005-2 fabricated 콘텐츠 금지)

        # @MX:NOTE: [AUTO] AC-005-2: 빈 retriever 결과 시 fabricated 콘텐츠 생성 금지
        # @MX:NOTE: [AUTO] AC-005-3: criterion_id는 GapItem에서 ContentSuggestion으로 전달
        """
        if not gaps:
            return []

        suggestions: list[ContentSuggestion] = []

        for gap in gaps:
            # 기준 이름을 쿼리로 사용해 유사 A 등급 콘텐츠 검색
            query = gap.target_state
            results: list[CriterionMatch] = retriever.retrieve(query, top_k=_RETRIEVE_TOP_K)

            # AC-005-2: 결과 없으면 fabricated 콘텐츠 생성 금지 — 해당 갭은 건너뜀
            if not results:
                continue

            # 첫 번째 결과를 주요 콘텐츠로 사용, 나머지는 evidence_refs에 포함
            primary = results[0]
            evidence_refs = [
                m.criterion.id
                for m in results
                if m.criterion.id  # 빈 ID 제외
            ]

            # 제안 콘텐츠: 검색된 기준의 세부 내용 사용
            suggested_content = (
                primary.criterion.criterion_detail or primary.criterion.criterion_name
            )

            suggestions.append(
                ContentSuggestion(
                    criterion_id=gap.criterion_id,  # AC-005-3: 역추적 가능성 보장
                    suggested_content=suggested_content,
                    evidence_refs=evidence_refs,
                )
            )

        return suggestions
