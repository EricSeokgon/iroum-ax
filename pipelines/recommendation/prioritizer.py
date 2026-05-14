"""우선순위 지정기 — ContentSuggestion을 priority_score 기준으로 정렬 (REQ-AX-005)

# @MX:ANCHOR: [AUTO] Prioritizer.prioritize — RankedSuggestion 파이프라인 최종 출력 진입점
# @MX:REASON: API 레이어, 통합 테스트가 prioritize() 반환 값을 최종 소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-005 / AC-005-1
"""
from __future__ import annotations

from pkg.models.recommendation import ContentSuggestion, GapItem, RankedSuggestion


class Prioritizer:
    """ContentSuggestion + GapItem 쌍을 우선순위로 정렬해 RankedSuggestion 목록을 반환한다.

    우선순위 공식: priority_score = feasibility × score_delta (AC-005-1)
    내림차순 정렬 후 top_k 개수로 캡한다.
    """

    def prioritize(
        self,
        suggestions: list[ContentSuggestion],
        gaps: list[GapItem],
        top_k: int = 5,
    ) -> list[RankedSuggestion]:
        """ContentSuggestion을 우선순위 점수 기준으로 정렬해 RankedSuggestion을 반환한다.

        Args:
            suggestions: ContentSuggester.suggest()가 반환한 ContentSuggestion 목록
            gaps: GapAnalyzer.analyze()가 반환한 GapItem 목록 (criterion_id로 매핑)
            top_k: 반환할 최대 항목 수 (기본값: 5)

        Returns:
            RankedSuggestion 목록 — priority_score 내림차순 정렬, rank 1-based.
            빈 입력이면 빈 리스트 반환.

        # @MX:NOTE: [AUTO] priority_score = feasibility × score_delta — AC-005-1 수학 불변식
        """
        if not suggestions:
            return []

        # criterion_id → GapItem 인덱스 구성
        gap_by_criterion: dict[str, GapItem] = {g.criterion_id: g for g in gaps}

        # (ContentSuggestion, GapItem, priority_score) 튜플 목록 생성
        scored: list[tuple[ContentSuggestion, GapItem, float]] = []
        for sug in suggestions:
            gap = gap_by_criterion.get(sug.criterion_id)
            if gap is None:
                # 매칭 GapItem 없으면 건너뜀
                continue
            # AC-005-1: priority_score = feasibility × score_delta
            score = gap.feasibility * gap.score_delta
            scored.append((sug, gap, score))

        # priority_score 내림차순 정렬
        scored.sort(key=lambda t: t[2], reverse=True)

        # top_k 캡 적용
        scored = scored[:top_k]

        # RankedSuggestion 생성 — rank는 1-based 연속
        ranked: list[RankedSuggestion] = []
        for rank_idx, (sug, gap, score) in enumerate(scored, start=1):
            # source_benchmark_id: evidence_refs의 첫 번째 항목 (없으면 빈 문자열)
            source_benchmark_id = sug.evidence_refs[0] if sug.evidence_refs else ""

            ranked.append(
                RankedSuggestion(
                    rank=rank_idx,
                    suggestion=sug,
                    gap=gap,
                    priority_score=round(score, 6),
                    expected_score_delta=gap.score_delta,
                    feasibility_score=gap.feasibility,
                    source_benchmark_id=source_benchmark_id,
                )
            )

        return ranked
