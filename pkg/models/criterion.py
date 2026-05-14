"""공유 평가기준 데이터 모델 (REQ-AX-002)

# @MX:ANCHOR: [AUTO] Criterion / CriterionMatch — RAG 파이프라인 전반에서 사용되는 핵심 모델
# @MX:REASON: CriterionParser, EmbeddingService, VectorStore, Retriever 모두 이 모델을 반환/소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1
"""
from __future__ import annotations

from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class CriterionModel(BaseModel):
    """경영평가 기준 공유 모델 — Sprint 2에서 필드 확정"""

    id: UUID = Field(description="기준 고유 ID")
    criterion_name: str = Field(description="평가기준 이름")
    max_points: int | None = Field(default=None, description="최고 배점")


class Criterion(BaseModel):
    """파싱된 평가기준 — CriterionParser 반환 타입

    # @MX:NOTE: [AUTO] 항목→지표→배점 계층 구조를 보존하는 핵심 모델
    # @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1 / AC-002-3
    """

    model_config = ConfigDict(extra="allow")

    id: str = Field(description="기준 고유 ID (UUID 문자열)")
    criterion_name: str = Field(description="평가기준 이름 (한자 포함 가능)")
    criterion_detail: str = Field(default="", description="평가기준 세부 내용")
    max_points: int | None = Field(default=None, description="최고 배점")
    parent_criterion_id: str | None = Field(
        default=None, description="상위 기준 ID (항목→지표 계층 연결)"
    )
    normalization_warning: dict | None = Field(
        default=None,
        description=(
            "한자 정규화 경고: {'type': 'unresolved_hanja', 'unresolved_chars': [...]}"
        ),
    )
    embedding: list[float] | None = Field(
        default=None, description="ko-sroberta-multitask 임베딩 벡터 (dim=768)"
    )


class CriterionMatch(BaseModel):
    """RAG 검색 결과 — VectorStore/Retriever 반환 타입

    # @MX:NOTE: [AUTO] confidence_score는 한자 정규화 실패 시 × 0.8 가중치 적용됨
    # @MX:SPEC: SPEC-AX-001 AC-002-5
    """

    model_config = ConfigDict(extra="allow")

    criterion: Criterion = Field(description="매칭된 평가기준")
    confidence_score: float = Field(
        ge=0.0, le=1.0, description="유사도 점수 (0.0~1.0)"
    )
    distance: float = Field(description="벡터 거리 (코사인 거리)")
    hanja_penalty_applied: bool = Field(
        default=False, description="한자 정규화 실패로 confidence × 0.8 적용 여부"
    )


class ColdStartResponse(BaseModel):
    """cold-start 상태 응답 — 빈 인덱스 시 Retriever가 반환

    # @MX:NOTE: [AUTO] AC-002-6: silent empty 반환 금지, 명시적 상태 신호 필수
    # @MX:SPEC: SPEC-AX-001 AC-002-6
    """

    status: str = Field(default="index_not_bootstrapped", description="상태 코드")
    indexed_chunks: int = Field(default=0, description="현재 인덱싱된 청크 수")
    next_step: str = Field(
        default="POST /api/criteria/index 평가편람 PDF 업로드",
        description="다음 단계 안내",
    )
