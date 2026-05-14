"""REQ-AX-005 Prioritizer 단위 테스트 — Sprint 6 RED phase

AC-005-1: feasibility_score 내림차순 정렬 + 3-5개 캡
우선순위 점수 = feasibility × score_delta

# @MX:TODO: [AUTO] Prioritizer.prioritize 구현 미존재 — GREEN phase에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-005 / AC-005-1
"""
from __future__ import annotations

from pkg.models.recommendation import ContentSuggestion, GapItem, RankedSuggestion

# ============================================================
# 공통 헬퍼 — ContentSuggestion + GapItem 조합 생성
# ============================================================


def _make_suggestion_and_gap(
    criterion_id: str,
    feasibility: float,
    score_delta: float,
    content: str = "제안 콘텐츠",
) -> tuple[ContentSuggestion, GapItem]:
    """테스트용 (ContentSuggestion, GapItem) 튜플 생성 헬퍼"""
    sug = ContentSuggestion(
        criterion_id=criterion_id,
        suggested_content=content,
        evidence_refs=["bench-a-001"],
    )
    gap = GapItem(
        criterion_id=criterion_id,
        current_state="현재 상태",
        target_state="목표 상태",
        score_delta=score_delta,
        feasibility=feasibility,
    )
    return sug, gap


# ============================================================
# PR-1: top_k=5 캡 — 7개 입력 → 5개 반환
# ============================================================


def test_prioritizer_caps_at_top_k() -> None:
    """PR-1: top_k=5이고 7개 제안이 있으면 5개만 반환.

    # @MX:SPEC: SPEC-AX-001 AC-005-1 (3-5개 추천 항목)
    """
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    suggestions = []
    gaps = []
    for i in range(7):
        s, g = _make_suggestion_and_gap(
            criterion_id=f"crit-{i:03d}",
            feasibility=0.5 + i * 0.05,
            score_delta=0.3,
        )
        suggestions.append(s)
        gaps.append(g)

    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(suggestions=suggestions, gaps=gaps, top_k=5)

    assert len(ranked) == 5


# ============================================================
# PR-2: feasibility × score_delta 내림차순 정렬
# ============================================================


def test_prioritizer_sorts_by_priority_score_descending() -> None:
    """PR-2: rank 1이 가장 높은 priority_score (feasibility × score_delta).

    # @MX:SPEC: SPEC-AX-001 AC-005-1 (feasibility_score 내림차순)
    """
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    # 명확히 다른 우선순위 점수 설정
    # crit-001: 0.9 × 0.8 = 0.72 (최고)
    # crit-002: 0.5 × 0.5 = 0.25 (중간)
    # crit-003: 0.2 × 0.3 = 0.06 (최저)
    s1, g1 = _make_suggestion_and_gap("crit-001", feasibility=0.9, score_delta=0.8)
    s2, g2 = _make_suggestion_and_gap("crit-002", feasibility=0.5, score_delta=0.5)
    s3, g3 = _make_suggestion_and_gap("crit-003", feasibility=0.2, score_delta=0.3)

    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(
        suggestions=[s3, s1, s2],  # 의도적으로 순서 섞음
        gaps=[g3, g1, g2],
        top_k=3,
    )

    assert len(ranked) == 3
    assert ranked[0].gap.criterion_id == "crit-001", (
        f"rank 1 기대: crit-001, 실제: {ranked[0].gap.criterion_id}"
    )
    assert ranked[1].gap.criterion_id == "crit-002"
    assert ranked[2].gap.criterion_id == "crit-003"


# ============================================================
# PR-3: top_k보다 적은 입력 — 가용한 만큼 반환
# ============================================================


def test_prioritizer_returns_all_when_fewer_than_top_k() -> None:
    """PR-3: 2개 입력 + top_k=5 → 2개만 반환 (가용한 만큼).

    Walking Skeleton 범위: 3개 미만이면 전체 반환.
    """
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    s1, g1 = _make_suggestion_and_gap("crit-001", feasibility=0.8, score_delta=0.5)
    s2, g2 = _make_suggestion_and_gap("crit-002", feasibility=0.6, score_delta=0.4)

    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(
        suggestions=[s1, s2],
        gaps=[g1, g2],
        top_k=5,
    )

    assert len(ranked) == 2


# ============================================================
# PR-4: priority_score 수학 불변식
# ============================================================


def test_prioritizer_priority_score_equals_feasibility_times_score_delta() -> None:
    """PR-4: priority_score = feasibility × score_delta 수학 불변식.

    # @MX:SPEC: SPEC-AX-001 AC-005-1 (우선순위 정렬 기준)
    """
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    feasibility = 0.8
    score_delta = 0.5
    expected_priority_score = feasibility * score_delta  # 0.4

    s, g = _make_suggestion_and_gap(
        "crit-001", feasibility=feasibility, score_delta=score_delta
    )

    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(suggestions=[s], gaps=[g], top_k=5)

    assert len(ranked) == 1
    assert abs(ranked[0].priority_score - expected_priority_score) < 0.001, (
        f"priority_score 불변식 위반: 기대={expected_priority_score:.3f}, "
        f"실제={ranked[0].priority_score:.3f}"
    )


# ============================================================
# PR-5: rank 1-based 연속성
# ============================================================


def test_prioritizer_rank_is_1_based_sequential() -> None:
    """PR-5: rank가 1부터 시작하고 연속적이어야 한다."""
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    suggestions = []
    gaps = []
    for i in range(3):
        s, g = _make_suggestion_and_gap(
            criterion_id=f"crit-{i:03d}",
            feasibility=0.7 - i * 0.1,
            score_delta=0.5,
        )
        suggestions.append(s)
        gaps.append(g)

    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(suggestions=suggestions, gaps=gaps, top_k=5)

    assert len(ranked) == 3
    ranks = [r.rank for r in ranked]
    assert ranks == [1, 2, 3], f"rank 순서가 1-based 연속적이지 않음: {ranks}"


# ============================================================
# PR-6: 빈 입력 — 빈 리스트 반환
# ============================================================


def test_prioritizer_returns_empty_for_empty_input() -> None:
    """PR-6: 빈 suggestions 입력 → 빈 RankedSuggestion 리스트 반환."""
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(suggestions=[], gaps=[], top_k=5)

    assert ranked == []


# ============================================================
# PR-7: RankedSuggestion 필드 AC-005-1 구조 요구사항
# ============================================================


def test_prioritizer_ranked_suggestion_contains_required_fields() -> None:
    """PR-7: RankedSuggestion이 AC-005-1 요구 필드 포함.

    {content, expected_score_delta, feasibility_score, source_benchmark_id}
    """
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    s = ContentSuggestion(
        criterion_id="crit-001",
        suggested_content="안전교육 이수율 100% 달성 방안",
        evidence_refs=["bench-a-001"],
    )
    g = GapItem(
        criterion_id="crit-001",
        current_state="이수율 85%",
        target_state="이수율 100%",
        score_delta=0.3,
        feasibility=0.8,
    )

    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(suggestions=[s], gaps=[g], top_k=5)

    assert len(ranked) == 1
    r = ranked[0]
    # AC-005-1 요구 필드 검증
    assert isinstance(r, RankedSuggestion)
    assert r.expected_score_delta == g.score_delta
    assert r.feasibility_score == g.feasibility
    assert r.source_benchmark_id == "bench-a-001"  # evidence_refs[0]
    assert r.suggestion.suggested_content == s.suggested_content
