"""pgvector 기반 벡터 스토어 (REQ-AX-002)

PgVectorStore: psycopg2 연결 기반 실제 구현 (통합 테스트용)
VectorStore: mock-friendly 래퍼 — 단위 테스트에서 mock conn 주입 가능

# @MX:ANCHOR: [AUTO] VectorStore.query — 유사도 검색의 외부 시스템 통합 지점
# @MX:REASON: Retriever, 통합 테스트, 파이프라인 오케스트레이터가 모두 이 메서드를 호출함
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1 / REQ-AX-002-S1 / AC-002-4 / AC-002-6
"""
from __future__ import annotations

from typing import Any

from pkg.errors.custom_errors import IndexNotBootstrappedError, IndexRebuildingError
from pkg.models.criterion import Criterion, CriterionMatch

# 지원 벡터 차원
_VECTOR_DIM = 768


class VectorStore:
    """pgvector 연결을 주입받아 CRUD를 수행하는 벡터 스토어.

    단위 테스트에서는 mock_pgvector_conn을 주입하여 실제 DB 없이 테스트한다.
    """

    def __init__(self, conn: object) -> None:
        # @MX:NOTE: [AUTO] conn은 psycopg2 연결 또는 MagicMock (테스트용)
        self._conn = conn

    def upsert(self, criteria: list[Criterion]) -> None:
        """Criterion 리스트를 pgvector 테이블에 upsert한다.

        빈 리스트는 no-op으로 처리한다.
        """
        for criterion in criteria:
            if criterion.embedding:
                embedding_str = str(criterion.embedding)
            else:
                embedding_str = "[" + ",".join(["0.0"] * _VECTOR_DIM) + "]"
            self._conn.execute(
                """
                INSERT INTO criteria (id, criterion_name, criterion_detail,
                    max_points, parent_criterion_id, normalization_warning, embedding)
                VALUES (%(id)s, %(criterion_name)s, %(criterion_detail)s,
                    %(max_points)s, %(parent_criterion_id)s,
                    %(normalization_warning)s, %(embedding)s)
                ON CONFLICT (id) DO UPDATE
                SET criterion_name = EXCLUDED.criterion_name,
                    embedding = EXCLUDED.embedding
                """,
                {
                    "id": criterion.id,
                    "criterion_name": criterion.criterion_name,
                    "criterion_detail": criterion.criterion_detail,
                    "max_points": criterion.max_points,
                    "parent_criterion_id": criterion.parent_criterion_id,
                    "normalization_warning": criterion.normalization_warning,
                    "embedding": embedding_str,
                },
            )

    def query(self, query_vector: list[float], top_k: int = 5) -> list[CriterionMatch]:
        """벡터 유사도 검색을 수행하여 CriterionMatch 리스트를 반환한다.

        Args:
            query_vector: 768차원 쿼리 벡터
            top_k: 반환할 최대 결과 수

        Raises:
            ValueError: query_vector 차원이 768이 아닌 경우
            IndexRebuildingError: HNSW 인덱스 재구축 중인 경우
            IndexNotBootstrappedError: 인덱스가 비어 있는 경우 (cold-start)
        """
        # 차원 검증
        if len(query_vector) != _VECTOR_DIM:
            raise ValueError(
                f"쿼리 벡터 차원 오류: 기대={_VECTOR_DIM}, 실제={len(query_vector)}"
            )

        # 재구축 중 여부 확인
        if self.is_rebuilding():
            raise IndexRebuildingError(
                "HNSW 인덱스가 재구축 중입니다. 재구축 완료 후 재시도하세요."
            )

        # cold-start 확인
        indexed_count = self._conn.count_indexed_criteria()
        if indexed_count == 0:
            raise IndexNotBootstrappedError(
                "평가기준 인덱스가 초기화되지 않았습니다. "
                "POST /api/criteria/index 로 평가편람 PDF를 업로드하세요."
            )

        # pgvector 코사인 거리 쿼리 실행
        cursor = self._conn.execute(
            """
            SELECT id, criterion_name, criterion_detail,
                   max_points, parent_criterion_id,
                   normalization_warning, embedding,
                   embedding <=> %(query_vec)s AS distance
            FROM criteria
            ORDER BY distance ASC
            LIMIT %(top_k)s
            """,
            {"query_vec": str(query_vector), "top_k": top_k},
        )

        rows = cursor.fetchall()
        matches: list[CriterionMatch] = []
        for row in rows:
            criterion = Criterion(
                id=row["id"],
                criterion_name=row["criterion_name"],
                criterion_detail=row.get("criterion_detail", ""),
                max_points=row.get("max_points"),
                parent_criterion_id=row.get("parent_criterion_id"),
                normalization_warning=row.get("normalization_warning"),
                embedding=row.get("embedding"),
            )
            distance: float = float(row["distance"])
            # 코사인 거리 → 유사도 변환 (distance: 0=동일, 2=반대)
            confidence = max(0.0, 1.0 - distance)
            matches.append(
                CriterionMatch(
                    criterion=criterion,
                    confidence_score=confidence,
                    distance=distance,
                    hanja_penalty_applied=False,
                )
            )

        return matches[:top_k]

    def is_rebuilding(self) -> bool:
        """HNSW 인덱스 재구축 진행 여부를 반환한다."""
        return bool(self._conn.is_rebuilding())

    def count_indexed_criteria(self) -> int:
        """인덱싱된 기준 수를 반환한다."""
        return int(self._conn.count_indexed_criteria())


class FakeVectorStore:
    """인메모리 벡터 스토어 — 단위 테스트 및 개발 환경용.

    실제 pgvector 없이 동작하는 in-memory 폴백 구현.
    """

    def __init__(self) -> None:
        self._data: list[dict[str, Any]] = []
        self._rebuilding: bool = False

    def upsert(self, criteria: list[Criterion]) -> None:
        """Criterion을 인메모리 저장소에 저장한다."""
        for criterion in criteria:
            # 기존 항목 덮어쓰기
            self._data = [d for d in self._data if d["id"] != criterion.id]
            self._data.append(
                {
                    "id": criterion.id,
                    "criterion": criterion,
                    "embedding": criterion.embedding or [0.0] * _VECTOR_DIM,
                }
            )

    def query(self, query_vector: list[float], top_k: int = 5) -> list[CriterionMatch]:
        """인메모리 선형 검색으로 유사도 결과를 반환한다."""
        if len(query_vector) != _VECTOR_DIM:
            raise ValueError(
                f"쿼리 벡터 차원 오류: 기대={_VECTOR_DIM}, 실제={len(query_vector)}"
            )

        if self._rebuilding:
            raise IndexRebuildingError("인메모리 스토어 재구축 중")

        if not self._data:
            raise IndexNotBootstrappedError("인메모리 스토어가 비어 있습니다")

        # 코사인 유사도 계산 (선형 탐색)
        def _dot(a: list[float], b: list[float]) -> float:
            return sum(x * y for x, y in zip(a, b, strict=False))

        def _norm(v: list[float]) -> float:
            return sum(x * x for x in v) ** 0.5

        results: list[CriterionMatch] = []
        q_norm = _norm(query_vector)
        for item in self._data:
            emb: list[float] = item["embedding"]
            e_norm = _norm(emb)
            if q_norm == 0.0 or e_norm == 0.0:
                cosine = 0.0
            else:
                cosine = _dot(query_vector, emb) / (q_norm * e_norm)
            distance = 1.0 - cosine
            confidence = max(0.0, cosine)
            results.append(
                CriterionMatch(
                    criterion=item["criterion"],
                    confidence_score=confidence,
                    distance=distance,
                    hanja_penalty_applied=False,
                )
            )

        results.sort(key=lambda m: m.distance)
        return results[:top_k]

    def is_rebuilding(self) -> bool:
        """재구축 상태를 반환한다."""
        return self._rebuilding

    def count_indexed_criteria(self) -> int:
        """인덱싱된 기준 수를 반환한다."""
        return len(self._data)
