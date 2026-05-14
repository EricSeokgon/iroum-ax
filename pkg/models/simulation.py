"""공유 등급 시뮬레이션 데이터 모델 스켈레톤 (REQ-AX-003)"""
from __future__ import annotations

from uuid import UUID

from pydantic import BaseModel, Field


class SimulationModel(BaseModel):
    """등급 시뮬레이션 결과 공유 모델 — Sprint 4에서 필드 확정"""

    id: UUID = Field(description="시뮬레이션 고유 ID")
    workflow_id: UUID = Field(description="연결된 워크플로우 ID")
    abstain_flag: bool = Field(
        default=False,
        description="기권 플래그: probability_a < 0.5 AND probability_b < 0.5 (REQ-AX-003-E1)",
    )
    # TODO(Sprint 4): current_grade, target_grade, probability_a/b, prediction 필드 추가
