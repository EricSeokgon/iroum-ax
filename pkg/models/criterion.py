"""공유 평가기준 데이터 모델 스켈레톤 (REQ-AX-002)"""
from __future__ import annotations

from uuid import UUID

from pydantic import BaseModel, Field


class CriterionModel(BaseModel):
    """경영평가 기준 공유 모델 — Sprint 3에서 필드 확정"""

    id: UUID = Field(description="기준 고유 ID")
    criterion_name: str = Field(description="평가기준 이름")
    max_points: int | None = Field(default=None, description="최고 배점")
    # TODO(Sprint 3): embedding 필드 추가 (VECTOR(768), REQ-AX-002)
