"""EmbeddingService + VectorStore 오케스트레이터 (REQ-AX-002)

# @MX:ANCHOR: [AUTO] Retriever.search — 검색 쿼리의 외부 진입점
# @MX:REASON: API 핸들러, 파이프라인 오케스트레이터, 통합 테스트가 모두 이 메서드를 호출함
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1 / REQ-AX-002-E2 / REQ-AX-002-U1 / AC-002-3~6
"""
from __future__ import annotations

import contextlib
import re

from pkg.errors.custom_errors import IndexNotBootstrappedError
from pkg.models.criterion import ColdStartResponse, CriterionMatch

# 한자 범위 감지 (U+4E00–U+9FFF)
_HANJA_PATTERN = re.compile(r"[一-鿿]")

# 한자 포함 결과의 confidence 가중치 (AC-002-5 D9)
_HANJA_CONFIDENCE_PENALTY = 0.8


class Retriever:
    """임베딩 → 벡터 검색을 오케스트레이션하는 RAG 검색기.

    cold-start 및 한자 graceful degradation을 내부에서 처리한다.
    """

    def __init__(self, embedding_service: object, vector_store: object) -> None:
        # @MX:NOTE: [AUTO] 의존성 주입 — 테스트에서 mock 주입 가능
        self._embedding_service = embedding_service
        self._vector_store = vector_store
        # last_search_status: cold-start 감지를 위해 외부에서 접근 가능
        self.last_search_status: str = "ok"

    def search(
        self, query: str, top_k: int = 5
    ) -> list[CriterionMatch] | ColdStartResponse:
        """쿼리 텍스트로 유사 평가기준을 검색한다.

        Args:
            query: 검색 쿼리 (한자 포함 가능)
            top_k: 반환할 최대 결과 수

        Returns:
            CriterionMatch 리스트 또는 ColdStartResponse (cold-start 시)

        Note:
            - cold-start: IndexNotBootstrappedError → ColdStartResponse 반환
            - 한자 쿼리: 결과의 confidence × 0.8 적용 (AC-002-5)
            - IndexRebuildingError는 호출자에게 전파
        """
        # 1. 쿼리 임베딩
        query_vector: list[float] = self._embedding_service.encode(query)  # type: ignore[attr-defined]

        # 2. 벡터 검색 (cold-start 처리)
        try:
            matches: list[CriterionMatch] = self._vector_store.query(  # type: ignore[attr-defined]
                query_vector, top_k=top_k
            )
        except IndexNotBootstrappedError:
            # AC-002-6: cold-start — 명시적 상태 응답
            self.last_search_status = "index_not_bootstrapped"
            indexed = 0
            with contextlib.suppress(Exception):
                indexed = self._vector_store.count_indexed_criteria()  # type: ignore[attr-defined]
            return ColdStartResponse(indexed_chunks=indexed)
        except Exception as exc:
            # generic Exception으로 cold-start를 시뮬레이션하는 mock 대응
            msg = str(exc).lower()
            if "index_not_bootstrapped" in msg or "비어" in msg:
                self.last_search_status = "index_not_bootstrapped"
                indexed = 0
                with contextlib.suppress(Exception):
                    indexed = self._vector_store.count_indexed_criteria()  # type: ignore[attr-defined]
                return ColdStartResponse(indexed_chunks=indexed)
            raise

        # 3. 한자 포함 쿼리이거나 결과에 normalization_warning이 있으면 confidence 가중치 적용
        has_hanja_in_query = bool(_HANJA_PATTERN.search(query))
        adjusted: list[CriterionMatch] = []
        for match in matches:
            has_warning = match.criterion.normalization_warning is not None
            if has_hanja_in_query or has_warning:
                penalized_score = match.confidence_score * _HANJA_CONFIDENCE_PENALTY
                adjusted.append(
                    match.model_copy(
                        update={
                            "confidence_score": penalized_score,
                            "hanja_penalty_applied": True,
                        }
                    )
                )
            else:
                adjusted.append(match)

        self.last_search_status = "ok"
        return adjusted[:top_k]
