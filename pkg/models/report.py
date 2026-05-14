"""공유 실적보고서 데이터 모델 스켈레톤 (REQ-AX-004)"""
from __future__ import annotations

from enum import Enum
from uuid import UUID

from pydantic import BaseModel, Field


class Grade(str, Enum):
    """경영평가 등급"""

    A = "A"
    B = "B"
    C = "C"
    D = "D"


class ReportModel(BaseModel):
    """경영평가 실적보고서 공유 모델 — Sprint 5에서 필드 확정"""

    id: UUID = Field(description="보고서 고유 ID")
    organization_name: str | None = Field(default=None, description="기관명")
    grade: Grade | None = Field(default=None, description="등급")
    # TODO(Sprint 5): content, score, source_benchmark_id 필드 추가 (REQ-AX-004)
