"""Pydantic 데이터 모델 스텁

API 요청/응답 스키마 정의.
실제 필드는 Sprint 2-6에서 REQ별로 확장됨.
"""
from __future__ import annotations

from enum import Enum
from uuid import UUID

from pydantic import BaseModel, Field


class FileType(str, Enum):
    """문서 파일 형식"""

    HWP = "HWP"
    PDF = "PDF"
    IMAGE = "IMAGE"


class WorkflowStatus(str, Enum):
    """워크플로우 실행 상태"""

    PENDING = "PENDING"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"


class Grade(str, Enum):
    """경영평가 등급"""

    A = "A"
    B = "B"
    C = "C"
    D = "D"


class HealthResponse(BaseModel):
    """헬스 체크 응답"""

    status: str
    version: str


# TODO(Sprint 2): DocumentUploadRequest, DocumentUploadResponse (REQ-AX-001)
# TODO(Sprint 3): CriterionIndexRequest, CriterionSearchRequest, CriterionSearchResponse (REQ-AX-002)
# TODO(Sprint 4): SimulationPredictRequest, SimulationPredictResponse (REQ-AX-003)
# TODO(Sprint 5): ReportGenerateRequest, ReportGenerateResponse (REQ-AX-004)
# TODO(Sprint 6): RecommendationGenerateRequest, RecommendationFeedbackRequest (REQ-AX-005)


class WorkflowResponse(BaseModel):
    """워크플로우 상태 응답 (공통)"""

    workflow_id: UUID = Field(description="워크플로우 고유 ID")
    status: WorkflowStatus = Field(description="현재 실행 상태")
    user_id: str = Field(default="cli-anonymous", description="실행 사용자 ID")
