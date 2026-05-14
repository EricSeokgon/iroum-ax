"""REQ-AX-005 통합 테스트 — Sprint 6 RED phase

AC-005-1: B→A 전체 파이프라인 3-5개 prioritized 추천
AC-005-2: 벤치마크 없음 파이프라인 — BenchmarkNotAvailableError 또는 빈 리스트

# @MX:TODO: [AUTO] gap_analyzer/content_suggester/prioritizer 구현 미존재 — GREEN phase에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-005 / AC-005-1 / AC-005-2
"""
from __future__ import annotations

from unittest.mock import MagicMock

import pytest
from pkg.models.criterion import Criterion, CriterionMatch
from pkg.models.document import ParsedDocument
from pkg.models.recommendation import RankedSuggestion
from pkg.models.simulation import BenchmarkReport, GradeDistribution

# ============================================================
# 픽스처
# ============================================================


@pytest.fixture()
def b_grade_distribution() -> GradeDistribution:
    """현재 B 등급 분포"""
    return GradeDistribution(
        p_a=0.35,
        p_b=0.60,
        p_abstain=0.05,
        predicted_class="B",
        low_confidence=False,
    )


@pytest.fixture()
def a_grade_benchmarks() -> list[BenchmarkReport]:
    """A 등급 벤치마크 보고서 목록"""
    return [
        BenchmarkReport(
            grade="A",
            text_content="안전교육 이수율 100%. 위험성평가 정기 실시. KOSHA 18001 인증.",
            report_id="bench-a-001",
        ),
        BenchmarkReport(
            grade="A",
            text_content="안전사고 0건. 안전보건 관리체계 우수. 근로자 교육 강화.",
            report_id="bench-a-002",
        ),
    ]


@pytest.fixture()
def criterion_list() -> list[Criterion]:
    """평가기준 목록"""
    return [
        Criterion(
            id="crit-safety-edu",
            criterion_name="안전교육 이수율",
            max_points=5,
        ),
        Criterion(
            id="crit-kosha",
            criterion_name="KOSHA 인증",
            max_points=3,
        ),
        Criterion(
            id="crit-accident",
            criterion_name="안전사고 발생률",
            max_points=5,
        ),
    ]


@pytest.fixture()
def current_b_report() -> ParsedDocument:
    """현재 B 수준 보고서"""
    return ParsedDocument(
        text="안전교육 이수율 85%. 안전사고 2건. 위험성평가 미흡.",
        status="ok",
    )


@pytest.fixture()
def mock_retriever_populated() -> MagicMock:
    """A 등급 벤치마크 결과를 반환하는 Retriever mock"""
    mock = MagicMock()
    match = CriterionMatch(
        criterion=Criterion(
            id="crit-safety-edu",
            criterion_name="안전교육 이수율",
            criterion_detail="안전교육 이수율 100% 달성. KOSHA 18001 인증 보유.",
            max_points=5,
        ),
        confidence_score=0.90,
        distance=0.10,
    )
    mock.retrieve.return_value = [match]
    return mock


# ============================================================
# INT-1: B→A 전체 파이프라인 — 3-5개 RankedSuggestion 반환
# ============================================================


def test_b_to_a_full_pipeline_returns_ranked_suggestions(
    b_grade_distribution: GradeDistribution,
    a_grade_benchmarks: list[BenchmarkReport],
    criterion_list: list[Criterion],
    current_b_report: ParsedDocument,
    mock_retriever_populated: MagicMock,
) -> None:
    """INT-1: B→A 전체 파이프라인 — GapAnalyzer → ContentSuggester → Prioritizer.

    AC-005-1: 3-5개 RankedSuggestion, feasibility_score 내림차순.
    """
    from pipelines.recommendation.content_suggester import ContentSuggester  # noqa: PLC0415
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    # Step 1: Gap 분석
    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_b_report,
        benchmarks=a_grade_benchmarks,
        criteria=criterion_list,
    )

    # Step 2: 콘텐츠 제안 생성
    suggester = ContentSuggester()
    suggestions = suggester.suggest(
        gaps=gaps,
        retriever=mock_retriever_populated,
    )

    # Step 3: 우선순위 정렬 (최대 5개)
    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(
        suggestions=suggestions,
        gaps=gaps,
        top_k=5,
    )

    # AC-005-1: 3-5개 반환 (Walking Skeleton 범위: 최소 1개 이상)
    assert isinstance(ranked, list)
    assert len(ranked) >= 1
    assert len(ranked) <= 5

    # 타입 검증
    for r in ranked:
        assert isinstance(r, RankedSuggestion)

    # feasibility_score 내림차순 검증
    for i in range(len(ranked) - 1):
        assert ranked[i].priority_score >= ranked[i + 1].priority_score, (
            f"rank {i+1}이 rank {i+2}보다 낮은 priority_score: "
            f"{ranked[i].priority_score} < {ranked[i+1].priority_score}"
        )


# ============================================================
# INT-2: 벤치마크 없음 — benchmark_not_available 또는 빈 리스트
# ============================================================


def test_full_pipeline_no_benchmarks_returns_empty_or_raises(
    b_grade_distribution: GradeDistribution,
    criterion_list: list[Criterion],
    current_b_report: ParsedDocument,
) -> None:
    """INT-2: 벤치마크 없음 파이프라인 — fabricated 콘텐츠 절대 금지.

    AC-005-2: benchmark_not_available 상태 또는 빈 리스트 반환.
    """
    from pipelines.recommendation.content_suggester import ContentSuggester  # noqa: PLC0415
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415
    from pipelines.recommendation.prioritizer import Prioritizer  # noqa: PLC0415

    # 빈 retriever
    empty_retriever = MagicMock()
    empty_retriever.retrieve.return_value = []

    # Step 1: Gap 분석 — 벤치마크 없으면 빈 리스트
    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_b_report,
        benchmarks=[],  # 빈 벤치마크
        criteria=criterion_list,
    )

    # 벤치마크 없으면 gaps도 빈 리스트여야 함
    assert gaps == []

    # Step 2: ContentSuggester에 빈 gaps 전달
    suggester = ContentSuggester()
    try:
        suggestions = suggester.suggest(gaps=gaps, retriever=empty_retriever)
        # 빈 리스트 허용
        assert suggestions == []
    except Exception as e:
        # BenchmarkNotAvailableError 허용
        error_name = type(e).__name__
        assert "BenchmarkNotAvailable" in error_name or "benchmark_not_available" in str(e).lower(), (
            f"예상치 못한 예외: {error_name}: {e}"
        )

    # Step 3: Prioritizer에 빈 suggestions 전달 → 빈 리스트 반환
    prioritizer = Prioritizer()
    ranked = prioritizer.prioritize(suggestions=[], gaps=[], top_k=5)
    assert ranked == []
