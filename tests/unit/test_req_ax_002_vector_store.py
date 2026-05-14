"""REQ-AX-002 VectorStore 단위/통합 테스트 — RED phase

pgvector HNSW 인터페이스의 계약을 정의한다.
- 단위 테스트: FakeVectorStore (인메모리, mock DB)
- 통합 테스트: testcontainers PostgreSQL+pgvector (@pytest.mark.integration)

# @MX:TODO: [AUTO] GREEN phase에서 pipelines.mapping.vector_store.VectorStore 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1 / REQ-AX-002-S1 / AC-002-4 / AC-002-6
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from pkg.models.criterion import Criterion, CriterionMatch


class TestVectorStoreUpsert:
    """VectorStore.upsert() 계약 검증 (단위 테스트 — mock DB)"""

    def test_upsert_accepts_list_of_criterion_objects(
        self, mock_pgvector_conn: MagicMock
    ) -> None:
        """upsert()가 Criterion 리스트를 받아 예외 없이 처리해야 한다."""
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        criteria = _make_criterion_list(n=3)
        store = VectorStore(conn=mock_pgvector_conn)
        # 예외 없이 완료되어야 함
        store.upsert(criteria)

    def test_upsert_calls_db_execute_for_each_criterion(
        self, mock_pgvector_conn: MagicMock
    ) -> None:
        """upsert()가 각 criterion에 대해 DB execute를 호출해야 한다."""
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        criteria = _make_criterion_list(n=3)
        store = VectorStore(conn=mock_pgvector_conn)
        store.upsert(criteria)

        # 3개 criterion → 3번 이상 DB 호출
        call_count = mock_pgvector_conn.execute.call_count
        assert call_count >= 3, f"DB execute 호출 횟수 부족: {call_count} < 3"

    def test_upsert_empty_list_does_not_raise(
        self, mock_pgvector_conn: MagicMock
    ) -> None:
        """빈 리스트 upsert는 예외를 발생시키지 않아야 한다."""
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        store = VectorStore(conn=mock_pgvector_conn)
        store.upsert([])  # 예외 없이 통과해야 함


class TestVectorStoreQuery:
    """VectorStore.query() 계약 검증 (단위 테스트 — mock DB)"""

    def test_query_returns_list_of_criterion_match(
        self, mock_pgvector_conn: MagicMock
    ) -> None:
        """query()가 CriterionMatch 리스트를 반환해야 한다."""
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        query_vec = [0.1] * 768
        store = VectorStore(conn=mock_pgvector_conn)
        result = store.query(query_vec, top_k=3)

        assert isinstance(result, list)
        assert all(isinstance(m, CriterionMatch) for m in result)

    def test_query_respects_top_k_limit(
        self, mock_pgvector_conn: MagicMock
    ) -> None:
        """query()가 top_k=3이면 최대 3개 결과를 반환해야 한다.

        AC-002-4: k=3 returns exactly 3 (or empty if cold).
        """
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        query_vec = [0.1] * 768
        store = VectorStore(conn=mock_pgvector_conn)
        result = store.query(query_vec, top_k=3)

        assert len(result) <= 3, f"top_k=3 초과: {len(result)}개 반환됨"

    def test_query_with_empty_index_raises_index_not_bootstrapped(
        self, mock_pgvector_conn_empty: MagicMock
    ) -> None:
        """빈 인덱스에 대한 query는 IndexNotBootstrappedError를 발생시켜야 한다.

        AC-002-6: cold-start (빈 인덱스) → 명시적 응답.
        silent empty array 반환 금지.
        """
        from pipelines.mapping.vector_store import IndexNotBootstrappedError, VectorStore  # type: ignore[import]

        query_vec = [0.1] * 768
        store = VectorStore(conn=mock_pgvector_conn_empty)

        with pytest.raises(IndexNotBootstrappedError):
            store.query(query_vec, top_k=3)

    def test_query_vector_must_be_dim_768(
        self, mock_pgvector_conn: MagicMock
    ) -> None:
        """768차원이 아닌 벡터로 query 시 ValueError를 발생시켜야 한다."""
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        wrong_dim_vec = [0.1] * 512  # 512차원 (잘못된 차원)
        store = VectorStore(conn=mock_pgvector_conn)

        with pytest.raises(ValueError, match="768"):
            store.query(wrong_dim_vec, top_k=3)


class TestVectorStoreIndexStatus:
    """VectorStore 인덱스 상태 관리 (AC-002-4)"""

    def test_is_rebuilding_returns_true_when_reindex_in_progress(
        self, mock_pgvector_conn_rebuilding: MagicMock
    ) -> None:
        """HNSW 재구축 중일 때 is_rebuilding()이 True를 반환해야 한다.

        AC-002-4: reindex 중 stale 결과 반환 금지.
        """
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        store = VectorStore(conn=mock_pgvector_conn_rebuilding)
        assert store.is_rebuilding() is True

    def test_query_raises_index_rebuilding_when_in_progress(
        self, mock_pgvector_conn_rebuilding: MagicMock
    ) -> None:
        """HNSW 재구축 중 query 시 IndexRebuildingError를 발생시켜야 한다.

        AC-002-4: stale 또는 partial 결과를 반환하지 않는다.
        """
        from pipelines.mapping.vector_store import IndexRebuildingError, VectorStore  # type: ignore[import]

        query_vec = [0.1] * 768
        store = VectorStore(conn=mock_pgvector_conn_rebuilding)

        with pytest.raises(IndexRebuildingError):
            store.query(query_vec, top_k=3)


@pytest.mark.integration
class TestVectorStoreIntegration:
    """pgvector 통합 테스트 — testcontainers PostgreSQL 사용

    @pytest.mark.integration 마킹: -m "not integration" 으로 기본 제외.
    """

    def test_upsert_and_query_roundtrip(self, pg_connection: object) -> None:  # noqa: ARG002
        """upsert 후 query로 삽입된 criterion을 검색할 수 있어야 한다."""
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        criteria = _make_criterion_list(n=3)
        store = VectorStore(conn=pg_connection)
        store.upsert(criteria)

        query_vec = [0.1] * 768
        result = store.query(query_vec, top_k=3)

        assert len(result) <= 3

    def test_empty_index_raises_on_query(self, pg_connection: object) -> None:  # noqa: ARG002
        """빈 pgvector 인덱스에서 query 시 IndexNotBootstrappedError가 발생해야 한다."""
        from pipelines.mapping.vector_store import IndexNotBootstrappedError, VectorStore  # type: ignore[import]

        store = VectorStore(conn=pg_connection)
        # 빈 인덱스 상태 (upsert 없음)
        with pytest.raises(IndexNotBootstrappedError):
            store.query([0.1] * 768, top_k=3)


# ============================================================
# 헬퍼 함수
# ============================================================


def _make_criterion_list(n: int) -> list[Criterion]:
    """테스트용 Criterion 리스트 생성"""
    return [
        Criterion(
            id=f"criterion-{i:03d}",
            criterion_name=f"안전보건 평가기준 {i}",
            criterion_detail=f"안전보건 관련 세부 평가 기준 {i}",
            max_points=5,
            parent_criterion_id="criterion-root-001" if i > 0 else None,
            embedding=[0.1] * 768,
        )
        for i in range(n)
    ]


# ============================================================
# 픽스처
# ============================================================


@pytest.fixture()
def mock_pgvector_conn() -> MagicMock:
    """pgvector 연결 모의 객체 — 정상 상태 (데이터 있음)"""
    conn = MagicMock()
    conn.execute.return_value = None

    # query 결과 모의: 3개 행 반환
    mock_cursor = MagicMock()
    mock_cursor.fetchall.return_value = [
        {
            "id": f"criterion-{i:03d}",
            "criterion_name": f"안전보건 평가기준 {i}",
            "criterion_detail": f"세부 내용 {i}",
            "max_points": 5,
            "parent_criterion_id": "criterion-root-001",
            "normalization_warning": None,
            "embedding": [0.1] * 768,
            "distance": 0.1 + i * 0.05,
        }
        for i in range(3)
    ]
    mock_cursor.__iter__ = lambda self: iter(self.fetchall())
    conn.execute.return_value = mock_cursor

    # 인덱스 상태: 정상 (재구축 중 아님)
    conn.is_rebuilding = MagicMock(return_value=False)
    conn.count_indexed_criteria = MagicMock(return_value=10)

    return conn


@pytest.fixture()
def mock_pgvector_conn_empty() -> MagicMock:
    """pgvector 연결 모의 객체 — cold-start (데이터 0건)"""
    conn = MagicMock()

    mock_cursor = MagicMock()
    mock_cursor.fetchall.return_value = []
    mock_cursor.__iter__ = lambda self: iter([])
    conn.execute.return_value = mock_cursor
    conn.count_indexed_criteria = MagicMock(return_value=0)
    conn.is_rebuilding = MagicMock(return_value=False)

    return conn


@pytest.fixture()
def mock_pgvector_conn_rebuilding() -> MagicMock:
    """pgvector 연결 모의 객체 — HNSW 재구축 중"""
    conn = MagicMock()
    conn.is_rebuilding = MagicMock(return_value=True)
    conn.count_indexed_criteria = MagicMock(return_value=5)

    return conn


@pytest.fixture()
def pg_connection() -> object:
    """testcontainers PostgreSQL+pgvector 연결 픽스처.

    통합 테스트에서만 사용. @pytest.mark.integration 필수.
    실제 컨테이너 연결은 GREEN phase에서 conftest.py에 추가.
    """
    pytest.skip("통합 테스트용 pgvector 컨테이너 픽스처 — GREEN phase에서 구현")
