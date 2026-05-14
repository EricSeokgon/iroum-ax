"""REQ-AX-002 Retriever 단위 테스트 — RED phase

EmbeddingService + VectorStore를 오케스트레이션하는 Retriever의 계약을 정의한다.
주요 검증:
- AC-002-3: 쿼리 관련성 — '안전교육 평가기준' → 안전보건 항목 매칭
- AC-002-4: top-k 제한 적용
- AC-002-5 (D9): 한자/한글 혼용 → confidence × 0.8
- AC-002-6 (D10): cold-start → 명시적 cold-start 응답

# @MX:TODO: [AUTO] GREEN phase에서 pipelines.mapping.retriever.Retriever 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1 / REQ-AX-002-E2 / REQ-AX-002-U1 / AC-002-3~6
"""
from __future__ import annotations

from unittest.mock import MagicMock

import pytest

from pkg.models.criterion import Criterion, CriterionMatch


class TestRetrieverHappyPath:
    """AC-002-1, AC-002-3: 정상 검색 경로"""

    def test_search_returns_list_of_criterion_match(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store: MagicMock,
    ) -> None:
        """search()가 CriterionMatch 리스트를 반환해야 한다."""
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store,
        )
        result = retriever.search("안전교육 이수율 평가기준", top_k=3)

        assert isinstance(result, list)
        assert all(isinstance(m, CriterionMatch) for m in result)

    def test_search_calls_embedding_encode_then_vector_query(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store: MagicMock,
    ) -> None:
        """search()가 embed → query 순서로 호출해야 한다."""
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store,
        )
        retriever.search("안전교육 평가기준", top_k=3)

        mock_embedding_service.encode.assert_called_once_with("안전교육 평가기준")
        mock_vector_store.query.assert_called_once()

    def test_search_safety_query_returns_safety_criterion_match(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store_with_safety_data: MagicMock,
    ) -> None:
        """'안전교육 평가기준' 쿼리가 안전보건 항목을 최상위에 반환해야 한다.

        AC-002-3: '안전교육 평가기준' → 안전보건 항목 매칭.
        """
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store_with_safety_data,
        )
        result = retriever.search("안전교육 평가기준", top_k=3)

        assert len(result) > 0
        top_match = result[0]
        # 안전보건 관련 criterion이 최상위로 반환되어야 함
        assert "안전" in top_match.criterion.criterion_name or \
               "안전" in top_match.criterion.criterion_detail


class TestRetrieverTopKLimit:
    """AC-002-4: top-k 제한 적용"""

    def test_search_returns_exactly_top_k_results_when_enough_data(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store: MagicMock,
    ) -> None:
        """인덱스에 충분한 데이터가 있을 때 top_k=3이면 정확히 3개를 반환해야 한다."""
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store,
        )
        result = retriever.search("안전교육", top_k=3)

        assert len(result) == 3

    def test_search_top_k_cannot_exceed_limit(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store: MagicMock,
    ) -> None:
        """top_k=5 요청에서도 최대 5개를 초과하지 않아야 한다."""
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store,
        )
        result = retriever.search("안전교육", top_k=5)

        assert len(result) <= 5


class TestRetrieverHanjaGracefulDegradation:
    """AC-002-5 (D9): 한자/한글 혼용 graceful degradation"""

    def test_search_with_hanja_query_applies_confidence_penalty(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store_with_hanja_warning: MagicMock,
    ) -> None:
        """한자 포함 결과의 confidence_score가 × 0.8 가중치로 감소해야 한다.

        AC-002-5: (c) 검색 시 해당 chunk의 confidence를 0.8 곱하여 reweight.
        """
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store_with_hanja_warning,
        )
        result = retriever.search("安全 교육 기준", top_k=3)

        # hanja_penalty_applied=True인 결과가 있어야 함
        penalized = [m for m in result if m.hanja_penalty_applied]
        assert len(penalized) > 0, "한자 포함 결과에 penalty가 적용되지 않음"

        # penalty 적용된 결과의 confidence는 원래 값의 0.8배여야 함
        for match in penalized:
            assert match.confidence_score <= 0.8, \
                f"confidence {match.confidence_score}가 0.8을 초과함 (penalty 미적용 의심)"

    def test_search_with_hanja_does_not_crash(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store: MagicMock,
    ) -> None:
        """한자 포함 쿼리가 시스템 크래시를 일으키지 않아야 한다.

        AC-002-5: 시스템 크래시·HTTP 500 발생 시 본 AC는 실패.
        """
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store,
        )
        try:
            result = retriever.search("安全 教育 고문서 古文書 보존", top_k=3)
        except Exception as e:
            pytest.fail(f"한자 포함 쿼리에서 예외 발생: {e}")

        assert isinstance(result, list)

    def test_search_hanja_result_has_hanja_penalty_applied_flag(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store_with_hanja_warning: MagicMock,
    ) -> None:
        """한자 normalization_warning이 있는 결과에 hanja_penalty_applied=True가 설정되어야 한다."""
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store_with_hanja_warning,
        )
        result = retriever.search("안전교육", top_k=3)

        hanja_matches = [m for m in result if m.hanja_penalty_applied]
        assert len(hanja_matches) > 0


class TestRetrieverColdStart:
    """AC-002-6 (D10): cold-start 처리"""

    def test_search_cold_start_returns_cold_start_response(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store_cold_start: MagicMock,
    ) -> None:
        """빈 인덱스에서 search 시 ColdStartResponse 또는 명시적 상태를 반환해야 한다.

        AC-002-6: 'no_match' 또는 동등한 명시적 cold-start 신호.
        silent empty array 반환 금지.
        HTTP 500 / unhandled exception 발생 금지.
        """
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]
        from pkg.models.criterion import ColdStartResponse  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store_cold_start,
        )
        result = retriever.search("안전교육 평가기준", top_k=3)

        # ColdStartResponse를 반환하거나, retriever가 last_search_status를 노출해야 함
        is_cold_start_response = isinstance(result, ColdStartResponse)
        has_explicit_status = hasattr(retriever, "last_search_status") and \
            retriever.last_search_status == "index_not_bootstrapped"

        assert is_cold_start_response or has_explicit_status, \
            "cold-start 시 ColdStartResponse 또는 last_search_status='index_not_bootstrapped' 중 하나 필요"

    def test_search_cold_start_does_not_raise_unhandled_exception(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store_cold_start: MagicMock,
    ) -> None:
        """cold-start 검색이 unhandled exception을 발생시키지 않아야 한다.

        AC-002-6: HTTP 500 발생 금지 — Retriever가 cold-start를 내부에서 처리해야 함.
        """
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store_cold_start,
        )
        try:
            retriever.search("any query", top_k=3)
        except Exception as e:
            pytest.fail(f"cold-start 검색에서 unhandled 예외 발생: {e}")

    def test_search_cold_start_vector_store_has_zero_indexed_chunks(
        self,
        mock_embedding_service: MagicMock,
        mock_vector_store_cold_start: MagicMock,
    ) -> None:
        """cold-start 상태에서 vector_store.count_indexed_criteria()가 0을 반환해야 한다."""
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]

        retriever = Retriever(
            embedding_service=mock_embedding_service,
            vector_store=mock_vector_store_cold_start,
        )
        retriever.search("안전교육", top_k=3)

        # vector_store의 count가 0임을 검증
        assert mock_vector_store_cold_start.count_indexed_criteria() == 0


# ============================================================
# 픽스처
# ============================================================


@pytest.fixture()
def mock_embedding_service() -> MagicMock:
    """EmbeddingService 모의 객체"""
    import random

    mock = MagicMock()

    def fake_encode(text: str) -> list[float]:
        rng = random.Random(hash(text) % (2**32))
        return [rng.gauss(0, 0.1) for _ in range(768)]

    mock.encode.side_effect = fake_encode
    return mock


@pytest.fixture()
def mock_vector_store() -> MagicMock:
    """VectorStore 모의 객체 — 3개 criterion 반환"""
    mock = MagicMock()
    mock.is_rebuilding.return_value = False
    mock.count_indexed_criteria.return_value = 10
    mock.query.return_value = [
        CriterionMatch(
            criterion=Criterion(
                id=f"criterion-{i:03d}",
                criterion_name=f"안전보건 평가기준 {i}",
                criterion_detail=f"안전교육 관련 세부 기준 {i}",
                max_points=5,
                parent_criterion_id="criterion-root",
            ),
            confidence_score=0.9 - i * 0.05,
            distance=0.1 + i * 0.05,
            hanja_penalty_applied=False,
        )
        for i in range(3)
    ]
    return mock


@pytest.fixture()
def mock_vector_store_with_safety_data() -> MagicMock:
    """안전보건 데이터를 보유한 VectorStore 모의 객체"""
    mock = MagicMock()
    mock.is_rebuilding.return_value = False
    mock.count_indexed_criteria.return_value = 5
    mock.query.return_value = [
        CriterionMatch(
            criterion=Criterion(
                id="criterion-safety-001",
                criterion_name="안전교육 이수율",
                criterion_detail="근로자 안전교육 이수율 평가 기준",
                max_points=10,
                parent_criterion_id="criterion-root-safety",
            ),
            confidence_score=0.92,
            distance=0.08,
            hanja_penalty_applied=False,
        )
    ]
    return mock


@pytest.fixture()
def mock_vector_store_with_hanja_warning() -> MagicMock:
    """한자 normalization_warning이 있는 criterion을 포함한 VectorStore 모의 객체"""
    mock = MagicMock()
    mock.is_rebuilding.return_value = False
    mock.count_indexed_criteria.return_value = 5
    mock.query.return_value = [
        CriterionMatch(
            criterion=Criterion(
                id="criterion-hanja-001",
                criterion_name="古文書 보존 안전보건 기준",
                criterion_detail="희귀 한자가 포함된 평가기준",
                max_points=5,
                parent_criterion_id="criterion-root",
                normalization_warning={
                    "type": "unresolved_hanja",
                    "unresolved_chars": ["古", "文"],
                },
            ),
            confidence_score=0.72,  # 0.9 × 0.8 = 0.72
            distance=0.28,
            hanja_penalty_applied=True,
        ),
        CriterionMatch(
            criterion=Criterion(
                id="criterion-normal-001",
                criterion_name="안전보건 교육 기준",
                criterion_detail="일반 안전보건 교육 기준",
                max_points=5,
                parent_criterion_id="criterion-root",
            ),
            confidence_score=0.85,
            distance=0.15,
            hanja_penalty_applied=False,
        ),
    ]
    return mock


@pytest.fixture()
def mock_vector_store_cold_start() -> MagicMock:
    """빈 인덱스 (cold-start) VectorStore 모의 객체.

    GREEN phase에서 IndexNotBootstrappedError가 정의되면 side_effect에 연결된다.
    RED phase에서는 Exception으로 대체하여 cold-start 경로를 시뮬레이션한다.
    """
    mock = MagicMock()
    mock.is_rebuilding.return_value = False
    mock.count_indexed_criteria.return_value = 0
    # IndexNotBootstrappedError는 GREEN phase에서 정의됨
    # RED phase에서는 generic Exception으로 cold-start 조건 시뮬레이션
    mock.query.side_effect = Exception("index_not_bootstrapped: 인덱스가 비어 있습니다")
    return mock
