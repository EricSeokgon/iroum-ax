"""REQ-AX-005 GapAnalyzer 단위 테스트 — Sprint 6 RED phase

AC-005-1: B→A 3-5개 우선순위 제안
AC-005-3: criterion_id로 평가기준 역추적 가능

# @MX:TODO: [AUTO] GapAnalyzer.analyze 구현 미존재 — GREEN phase에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-005 / AC-005-1 / AC-005-3
"""
from __future__ import annotations

import pytest
from pkg.models.criterion import Criterion
from pkg.models.document import ParsedDocument
from pkg.models.recommendation import GapItem
from pkg.models.simulation import BenchmarkReport, GradeDistribution

# ============================================================
# 공통 픽스처
# ============================================================


@pytest.fixture()
def b_grade_distribution() -> GradeDistribution:
    """현재 B 등급 분포 픽스처 — p_b가 우세하고 p_a < 0.5"""
    return GradeDistribution(
        p_a=0.35,
        p_b=0.60,
        p_abstain=0.05,
        predicted_class="B",
        low_confidence=False,
    )


@pytest.fixture()
def a_grade_benchmark_reports() -> list[BenchmarkReport]:
    """A 등급 벤치마크 보고서 픽스처"""
    return [
        BenchmarkReport(
            grade="A",
            text_content="안전교육 이수율 100% 달성. KOSHA 18001 인증. 안전사고 0건. 위험성평가 우수 사례.",
            report_id="bench-a-001",
        ),
        BenchmarkReport(
            grade="A",
            text_content="안전관리체계 운영 현황 우수. 근로자 안전보건 교육 연 24시간 이상. 위험성평가 정기 실시.",
            report_id="bench-a-002",
        ),
    ]


@pytest.fixture()
def criterion_list() -> list[Criterion]:
    """평가기준 목록 픽스처 — criterion_id 역추적 테스트용"""
    return [
        Criterion(
            id="crit-safety-edu",
            criterion_name="안전교육 이수율",
            criterion_detail="근로자 안전보건 교육 이수율 평가",
            max_points=5,
        ),
        Criterion(
            id="crit-accident-rate",
            criterion_name="안전사고 발생률",
            criterion_detail="연간 안전사고 발생 건수 평가",
            max_points=5,
        ),
    ]


@pytest.fixture()
def current_report() -> ParsedDocument:
    """현재 보고서 픽스처 — B 등급 수준 콘텐츠"""
    return ParsedDocument(
        text="안전교육 이수율 85%. 안전사고 2건 발생. 위험성평가 미흡.",
        status="ok",
    )


# ============================================================
# GA-1: 정상 B→A 갭 분석 — list[GapItem] 반환
# ============================================================


def test_gap_analyzer_returns_gap_items_for_b_to_a(
    b_grade_distribution: GradeDistribution,
    a_grade_benchmark_reports: list[BenchmarkReport],
    current_report: ParsedDocument,
    criterion_list: list[Criterion],
) -> None:
    """GA-1: B 등급 현재 분포 + A 벤치마크 → 갭 항목 1개 이상 반환.

    # @MX:SPEC: SPEC-AX-001 AC-005-1
    """
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415

    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_report,
        benchmarks=a_grade_benchmark_reports,
        criteria=criterion_list,
    )

    assert isinstance(gaps, list)
    assert len(gaps) >= 1
    for gap in gaps:
        assert isinstance(gap, GapItem)


# ============================================================
# GA-2: 모든 기준이 이미 A 수준 — 빈 리스트 반환
# ============================================================


def test_gap_analyzer_returns_empty_when_already_a_grade(
    criterion_list: list[Criterion],
) -> None:
    """GA-2: 현재 보고서가 이미 A 등급 수준이면 빈 gap list를 반환.

    # @MX:SPEC: SPEC-AX-001 AC-005-1 (edge: 갭 없음)
    """
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415

    # 현재 A 등급 분포
    a_distribution = GradeDistribution(
        p_a=0.80,
        p_b=0.15,
        p_abstain=0.05,
        predicted_class="A",
        low_confidence=False,
    )
    # 이미 A 수준 콘텐츠
    a_report = ParsedDocument(
        text="안전교육 이수율 100% 달성. KOSHA 18001 인증. 안전사고 0건.",
        status="ok",
    )
    benchmarks = [
        BenchmarkReport(
            grade="A",
            text_content="안전교육 이수율 100% 달성. KOSHA 18001 인증. 안전사고 0건.",
            report_id="bench-a-001",
        ),
    ]

    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=a_distribution,
        target_grade="A",
        current_report=a_report,
        benchmarks=benchmarks,
        criteria=criterion_list,
    )

    assert isinstance(gaps, list)
    # A 등급 이미 달성 시 갭이 없거나 최소화됨
    assert len(gaps) == 0 or all(gap.score_delta < 0.1 for gap in gaps)


# ============================================================
# GA-3: 벤치마크 없음 — 빈 리스트 반환
# ============================================================


def test_gap_analyzer_returns_empty_when_no_benchmarks(
    b_grade_distribution: GradeDistribution,
    current_report: ParsedDocument,
    criterion_list: list[Criterion],
) -> None:
    """GA-3: 빈 benchmarks → 빈 gap list 반환 (fabricated 갭 생성 금지).

    # @MX:SPEC: SPEC-AX-001 AC-005-2 (벤치마크 데이터 부족 대응)
    """
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415

    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_report,
        benchmarks=[],  # 빈 벤치마크
        criteria=criterion_list,
    )

    assert gaps == []


# ============================================================
# GA-4: score_delta 양수 보장
# ============================================================


def test_gap_analyzer_score_delta_is_non_negative(
    b_grade_distribution: GradeDistribution,
    a_grade_benchmark_reports: list[BenchmarkReport],
    current_report: ParsedDocument,
    criterion_list: list[Criterion],
) -> None:
    """GA-4: 모든 GapItem의 score_delta는 0.0 이상이어야 한다.

    # @MX:SPEC: SPEC-AX-001 AC-005-1 (expected_score_delta 요구사항)
    """
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415

    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_report,
        benchmarks=a_grade_benchmark_reports,
        criteria=criterion_list,
    )

    for gap in gaps:
        assert gap.score_delta >= 0.0, f"score_delta 음수 발견: {gap.score_delta}"


# ============================================================
# GA-5: criterion_id 역추적 — AC-005-3
# ============================================================


def test_gap_analyzer_gap_items_carry_criterion_id(
    b_grade_distribution: GradeDistribution,
    a_grade_benchmark_reports: list[BenchmarkReport],
    current_report: ParsedDocument,
    criterion_list: list[Criterion],
) -> None:
    """GA-5: 각 GapItem의 criterion_id가 유효한 기준 ID를 가져야 한다.

    AC-005-3: criterion_id로 REQ-AX-002 기준으로의 역추적 가능.
    """
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415

    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_report,
        benchmarks=a_grade_benchmark_reports,
        criteria=criterion_list,
    )

    valid_criterion_ids = {c.id for c in criterion_list}
    for gap in gaps:
        # criterion_id가 비어 있지 않아야 함
        assert gap.criterion_id, f"criterion_id가 비어 있음: {gap}"
        # criterion_id가 입력된 기준 ID 중 하나여야 함
        assert gap.criterion_id in valid_criterion_ids, (
            f"criterion_id '{gap.criterion_id}'가 입력 기준 목록에 없음: {valid_criterion_ids}"
        )


# ============================================================
# GA-6: feasibility 범위 보장
# ============================================================


def test_gap_analyzer_feasibility_in_valid_range(
    b_grade_distribution: GradeDistribution,
    a_grade_benchmark_reports: list[BenchmarkReport],
    current_report: ParsedDocument,
    criterion_list: list[Criterion],
) -> None:
    """GA-6: 모든 GapItem의 feasibility는 0.0~1.0 범위여야 한다."""
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415

    analyzer = GapAnalyzer()
    gaps = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_report,
        benchmarks=a_grade_benchmark_reports,
        criteria=criterion_list,
    )

    for gap in gaps:
        assert 0.0 <= gap.feasibility <= 1.0, (
            f"feasibility 범위 초과: {gap.feasibility}"
        )


# ============================================================
# GA-7: 빈 criteria 목록 — 안전한 빈 리스트 반환
# ============================================================


def test_gap_analyzer_returns_empty_when_no_criteria(
    b_grade_distribution: GradeDistribution,
    a_grade_benchmark_reports: list[BenchmarkReport],
    current_report: ParsedDocument,
) -> None:
    """GA-7: 평가기준이 없으면 빈 리스트 반환 — 크래시 없음."""
    from pipelines.recommendation.gap_analyzer import GapAnalyzer  # noqa: PLC0415

    analyzer = GapAnalyzer()
    # criteria가 없으면 갭을 분석할 기준이 없으므로 빈 리스트
    result = analyzer.analyze(
        current=b_grade_distribution,
        target_grade="A",
        current_report=current_report,
        benchmarks=a_grade_benchmark_reports,
        criteria=[],
    )

    assert isinstance(result, list)
    assert result == []
