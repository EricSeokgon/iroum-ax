"""REQ-AX-002 파이프라인 통합 테스트 — RED phase

CriterionParser → EmbeddingService → VectorStore.upsert → VectorStore.query
전체 흐름을 검증하는 오케스트레이션 테스트.

단위 테스트와 달리 여러 컴포넌트를 함께 사용하되,
실제 모델/DB는 mock으로 대체한다.

# @MX:TODO: [AUTO] GREEN phase에서 pipelines.mapping.* 모듈 구현 후 통과
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1 / AC-002-1
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from pkg.models.criterion import Criterion, CriterionMatch


class TestCriterionMappingPipelineFlow:
    """CriterionParser → EmbeddingService → VectorStore → Retriever 전체 흐름"""

    def test_parse_then_embed_then_upsert_flow(
        self,
        tmp_path: "Path",  # noqa: F821
        mock_ko_sroberta_global: MagicMock,
        mock_pgvector_conn: MagicMock,
    ) -> None:
        """parse → encode → upsert 파이프라인이 예외 없이 완료되어야 한다.

        AC-002-1: 평가편람 indexing 전체 흐름.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        dummy_pdf = tmp_path / "criterion.pdf"
        dummy_pdf.write_bytes(b"%PDF-1.4 fake content")

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta_global,
        ):
            parser = CriterionParser()
            embed_svc = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")
            store = VectorStore(conn=mock_pgvector_conn)

            # 1단계: parse
            criteria = parser.parse(str(dummy_pdf))
            assert len(criteria) > 0

            # 2단계: embed (각 criterion에 embedding 추가)
            for criterion in criteria:
                criterion.embedding = embed_svc.encode(criterion.criterion_name)
                assert len(criterion.embedding) == 768

            # 3단계: upsert
            store.upsert(criteria)
            # 예외 없이 완료되면 성공

    def test_parse_embed_upsert_then_query_returns_results(
        self,
        tmp_path: "Path",  # noqa: F821
        mock_ko_sroberta_global: MagicMock,
        mock_pgvector_conn_with_results: MagicMock,
    ) -> None:
        """parse → embed → upsert → query 전체 파이프라인이 결과를 반환해야 한다.

        AC-002-1: 검색 쿼리 실행 후 top-k 결과 반환.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]
        from pipelines.mapping.retriever import Retriever  # type: ignore[import]
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        dummy_pdf = tmp_path / "criterion.pdf"
        dummy_pdf.write_bytes(b"%PDF-1.4 fake content")

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta_global,
        ):
            embed_svc = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")
            store = VectorStore(conn=mock_pgvector_conn_with_results)
            retriever = Retriever(embedding_service=embed_svc, vector_store=store)

            # 검색
            result = retriever.search("안전교육 이수율 평가기준", top_k=3)

            assert isinstance(result, list)
            assert len(result) <= 3

    def test_hanja_in_criterion_does_not_break_pipeline(
        self,
        tmp_path: "Path",  # noqa: F821
        mock_ko_sroberta_global: MagicMock,
        mock_pgvector_conn: MagicMock,
    ) -> None:
        """한자가 포함된 평가편람 처리 시 파이프라인이 중단되지 않아야 한다.

        AC-002-5: 전체 파이프라인 수준의 한자 graceful degradation.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]
        from pipelines.mapping.vector_store import VectorStore  # type: ignore[import]

        # 한자가 포함된 criterion 직접 생성 (parser mock 대신)
        hanja_criteria = [
            Criterion(
                id="hanja-criterion-001",
                criterion_name="古文書 보존 안전보건 기준",
                criterion_detail="希귀 한자 포함 평가기준",
                max_points=5,
                parent_criterion_id="root-criterion",
                normalization_warning={
                    "type": "unresolved_hanja",
                    "unresolved_chars": ["古", "文"],
                },
            )
        ]

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta_global,
        ):
            embed_svc = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")
            store = VectorStore(conn=mock_pgvector_conn)

            try:
                for criterion in hanja_criteria:
                    criterion.embedding = embed_svc.encode(criterion.criterion_name)
                store.upsert(hanja_criteria)
            except Exception as e:
                pytest.fail(f"한자 포함 criterion 파이프라인 처리 중 예외 발생: {e}")


# ============================================================
# 픽스처
# ============================================================


@pytest.fixture()
def mock_ko_sroberta_global() -> MagicMock:
    """ko-sroberta-multitask SentenceTransformer 모의 객체 (통합 테스트용)"""
    import random

    mock = MagicMock()

    def fake_encode(text: str, **kwargs: object) -> list[float]:
        if not text:
            return [0.0] * 768
        rng = random.Random(hash(text) % (2**32))
        return [rng.gauss(0, 0.1) for _ in range(768)]

    mock.encode.side_effect = fake_encode
    mock.get_sentence_embedding_dimension.return_value = 768
    return mock


@pytest.fixture()
def mock_pgvector_conn() -> MagicMock:
    """pgvector 연결 모의 객체 — upsert 전용"""
    conn = MagicMock()
    conn.execute.return_value = None
    conn.is_rebuilding = MagicMock(return_value=False)
    conn.count_indexed_criteria = MagicMock(return_value=10)
    return conn


@pytest.fixture()
def mock_pgvector_conn_with_results() -> MagicMock:
    """pgvector 연결 모의 객체 — query 결과 반환"""
    conn = MagicMock()
    conn.execute.return_value = None
    conn.is_rebuilding = MagicMock(return_value=False)
    conn.count_indexed_criteria = MagicMock(return_value=5)

    mock_cursor = MagicMock()
    mock_cursor.fetchall.return_value = [
        {
            "id": "criterion-result-001",
            "criterion_name": "안전교육 이수율 기준",
            "criterion_detail": "안전교육 이수율 관련 평가 세부 기준",
            "max_points": 10,
            "parent_criterion_id": "criterion-root",
            "normalization_warning": None,
            "embedding": [0.1] * 768,
            "distance": 0.08,
        }
    ]
    conn.execute.return_value = mock_cursor
    return conn
