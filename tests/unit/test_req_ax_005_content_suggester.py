"""REQ-AX-005 ContentSuggester 단위 테스트 — Sprint 6 RED phase

AC-005-2: 벤치마크 부족 시 fabricated 콘텐츠 금지 + benchmark_not_available
AC-005-3: criterion_id 역추적 보장

# @MX:TODO: [AUTO] ContentSuggester.suggest 구현 미존재 — GREEN phase에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-005 / AC-005-2 / AC-005-3
"""
from __future__ import annotations

from unittest.mock import MagicMock

import pytest
from pkg.models.criterion import CriterionMatch
from pkg.models.recommendation import ContentSuggestion, GapItem

# ============================================================
# 공통 픽스처
# ============================================================


@pytest.fixture()
def gap_items_with_criteria() -> list[GapItem]:
    """2개의 GapItem 픽스처 — criterion_id 포함"""
    return [
        GapItem(
            criterion_id="crit-safety-edu",
            current_state="안전교육 이수율 85%",
            target_state="안전교육 이수율 100% 달성 필요",
            score_delta=0.3,
            feasibility=0.8,
        ),
        GapItem(
            criterion_id="crit-accident-rate",
            current_state="안전사고 2건 발생",
            target_state="안전사고 0건 목표",
            score_delta=0.25,
            feasibility=0.7,
        ),
    ]


@pytest.fixture()
def mock_retriever_with_results() -> MagicMock:
    """결과를 반환하는 Retriever 모의 객체.

    A 등급 벤치마크 CriterionMatch를 반환한다.
    """
    mock = MagicMock()
    # retrieve() 호출 시 CriterionMatch 목록 반환
    from pkg.models.criterion import Criterion  # noqa: PLC0415

    match = CriterionMatch(
        criterion=Criterion(
            id="crit-safety-edu",
            criterion_name="안전교육 이수율",
            criterion_detail="안전교육 이수율 100% 달성. KOSHA 18001 인증 보유.",
            max_points=5,
        ),
        confidence_score=0.85,
        distance=0.15,
    )
    mock.retrieve.return_value = [match]
    return mock


@pytest.fixture()
def mock_retriever_empty() -> MagicMock:
    """결과가 없는 Retriever 모의 객체 — AC-005-2 테스트용."""
    mock = MagicMock()
    mock.retrieve.return_value = []
    return mock


# ============================================================
# CS-1: 정상 제안 생성 — criterion_id 보존
# ============================================================


def test_content_suggester_returns_suggestions_for_gap_items(
    gap_items_with_criteria: list[GapItem],
    mock_retriever_with_results: MagicMock,
) -> None:
    """CS-1: GapItem 목록 + mock retriever → ContentSuggestion 반환 + criterion_id 보존.

    # @MX:SPEC: SPEC-AX-001 AC-005-1
    """
    from pipelines.recommendation.content_suggester import ContentSuggester  # noqa: PLC0415

    suggester = ContentSuggester()
    suggestions = suggester.suggest(
        gaps=gap_items_with_criteria,
        retriever=mock_retriever_with_results,
    )

    assert isinstance(suggestions, list)
    assert len(suggestions) >= 1
    for sug in suggestions:
        assert isinstance(sug, ContentSuggestion)


# ============================================================
# CS-2: 빈 retriever — AC-005-2 (benchmark_not_available)
# ============================================================


def test_content_suggester_raises_or_returns_empty_when_retriever_empty(
    gap_items_with_criteria: list[GapItem],
    mock_retriever_empty: MagicMock,
) -> None:
    """CS-2: Retriever 결과 없음 → BenchmarkNotAvailableError 또는 빈 list 반환.

    AC-005-2: fabricated 콘텐츠 금지. 빈 결과만 허용.
    """
    from pipelines.recommendation.content_suggester import ContentSuggester  # noqa: PLC0415

    suggester = ContentSuggester()

    try:
        suggestions = suggester.suggest(
            gaps=gap_items_with_criteria,
            retriever=mock_retriever_empty,
        )
        # BenchmarkNotAvailableError를 발생시키지 않으면 빈 리스트여야 함
        assert suggestions == [], (
            "빈 retriever에서 fabricated 콘텐츠가 반환됨: "
            f"{[s.suggested_content for s in suggestions]}"
        )
    except Exception as e:
        # BenchmarkNotAvailableError 또는 유사 예외가 발생해야 함
        error_name = type(e).__name__
        assert "BenchmarkNotAvailable" in error_name or "benchmark_not_available" in str(e).lower(), (
            f"예상치 못한 예외 유형: {error_name}: {e}"
        )


# ============================================================
# CS-3: evidence_refs가 채워짐
# ============================================================


def test_content_suggester_populates_evidence_refs(
    gap_items_with_criteria: list[GapItem],
    mock_retriever_with_results: MagicMock,
) -> None:
    """CS-3: Retriever가 결과를 반환하면 evidence_refs가 빈 목록이 아니어야 한다.

    # @MX:SPEC: SPEC-AX-001 AC-005-1 (source_benchmark_id 구조 요구)
    """
    from pipelines.recommendation.content_suggester import ContentSuggester  # noqa: PLC0415

    suggester = ContentSuggester()
    suggestions = suggester.suggest(
        gaps=gap_items_with_criteria,
        retriever=mock_retriever_with_results,
    )

    # 결과가 있으면 evidence_refs가 채워져야 함
    for sug in suggestions:
        assert isinstance(sug.evidence_refs, list)
        # evidence_refs는 비어있지 않아야 함 (벤치마크 근거 추적)
        assert len(sug.evidence_refs) >= 1


# ============================================================
# CS-4: criterion_id 역추적 — AC-005-3
# ============================================================


def test_content_suggester_preserves_criterion_id(
    mock_retriever_with_results: MagicMock,
) -> None:
    """CS-4: ContentSuggestion의 criterion_id가 GapItem.criterion_id와 동일.

    AC-005-3: 역추적 가능성 — 각 제안은 원본 평가기준으로 추적 가능해야 함.
    """
    from pipelines.recommendation.content_suggester import ContentSuggester  # noqa: PLC0415

    gap = GapItem(
        criterion_id="crit-safety-edu",
        current_state="안전교육 이수율 85%",
        target_state="안전교육 이수율 100% 달성 필요",
        score_delta=0.3,
        feasibility=0.8,
    )

    suggester = ContentSuggester()
    suggestions = suggester.suggest(
        gaps=[gap],
        retriever=mock_retriever_with_results,
    )

    for sug in suggestions:
        assert sug.criterion_id == "crit-safety-edu", (
            f"criterion_id가 보존되지 않음: 기대='crit-safety-edu', 실제='{sug.criterion_id}'"
        )


# ============================================================
# CS-5: suggested_content가 비어 있지 않음
# ============================================================


def test_content_suggester_suggested_content_is_not_empty(
    gap_items_with_criteria: list[GapItem],
    mock_retriever_with_results: MagicMock,
) -> None:
    """CS-5: 각 ContentSuggestion의 suggested_content가 비어 있지 않아야 한다."""
    from pipelines.recommendation.content_suggester import ContentSuggester  # noqa: PLC0415

    suggester = ContentSuggester()
    suggestions = suggester.suggest(
        gaps=gap_items_with_criteria,
        retriever=mock_retriever_with_results,
    )

    for sug in suggestions:
        assert sug.suggested_content.strip(), (
            f"suggested_content가 비어 있음 (criterion_id='{sug.criterion_id}')"
        )
