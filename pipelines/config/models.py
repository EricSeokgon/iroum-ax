"""Pydantic 데이터 모델 — Sprint 0-6 API 스키마

API 요청/응답 스키마 정의 (REQ-AX-001 ~ REQ-AX-005).
"""
from __future__ import annotations

from enum import StrEnum
from uuid import UUID

from pydantic import BaseModel, Field


class FileType(StrEnum):
    """문서 파일 형식"""

    HWP = "HWP"
    PDF = "PDF"
    IMAGE = "IMAGE"


class WorkflowStatus(StrEnum):
    """워크플로우 실행 상태"""

    PENDING = "PENDING"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"


class Grade(StrEnum):
    """경영평가 등급"""

    A = "A"
    B = "B"
    C = "C"
    D = "D"


class HealthResponse(BaseModel):
    """헬스 체크 응답"""

    status: str
    version: str


class WorkflowResponse(BaseModel):
    """워크플로우 상태 응답 (공통)"""

    workflow_id: UUID = Field(description="워크플로우 고유 ID")
    status: WorkflowStatus = Field(description="현재 실행 상태")
    user_id: str = Field(default="cli-anonymous", description="실행 사용자 ID")


# ---------------------------------------------------------------------------
# Sprint 2: Document Upload (REQ-AX-001)
# ---------------------------------------------------------------------------


class DocumentUploadRequest(BaseModel):
    """문서 업로드 요청"""

    file_type: FileType = Field(description="파일 형식")
    filename: str = Field(description="파일명")
    user_id: str = Field(default="cli-anonymous", description="요청 사용자 ID")


class DocumentUploadResponse(WorkflowResponse):
    """문서 업로드 응답"""

    filename: str = Field(description="업로드된 파일명")


# ---------------------------------------------------------------------------
# Sprint 3: Criterion Index / Search (REQ-AX-002)
# ---------------------------------------------------------------------------


class CriterionIndexRequest(BaseModel):
    """평가기준 인덱싱 요청"""

    document_workflow_id: UUID = Field(description="문서 워크플로우 ID")
    user_id: str = Field(default="cli-anonymous", description="요청 사용자 ID")


class CriterionSearchRequest(BaseModel):
    """평가기준 유사도 검색 요청"""

    query: str = Field(description="검색 질의 문자열")
    top_k: int = Field(default=5, ge=1, le=20, description="반환할 최대 결과 수")
    user_id: str = Field(default="cli-anonymous", description="요청 사용자 ID")


class CriterionSearchItem(BaseModel):
    """평가기준 검색 결과 단일 항목"""

    criterion_id: str = Field(description="평가기준 ID")
    criterion_name: str = Field(description="평가기준 명칭")
    confidence_score: float = Field(description="유사도 점수 (0.0-1.0)")


class CriterionSearchResponse(BaseModel):
    """평가기준 검색 응답"""

    items: list[CriterionSearchItem] = Field(description="검색 결과 목록")
    total: int = Field(description="전체 결과 수")


# ---------------------------------------------------------------------------
# Sprint 4: Simulation (REQ-AX-003)
# ---------------------------------------------------------------------------


class SimulationPredictRequest(BaseModel):
    """B→A 등급 시뮬레이션 요청"""

    report_text: str = Field(description="현재 보고서 텍스트")
    target_grade: str = Field(default="A", description="목표 등급")
    user_id: str = Field(default="cli-anonymous", description="요청 사용자 ID")


class SimulationPredictResponse(BaseModel):
    """B→A 등급 시뮬레이션 응답"""

    current_grade: str = Field(description="현재 예측 등급")
    target_grade: str = Field(description="목표 등급")
    current_p_a: float = Field(description="현재 A등급 확률")
    projected_p_a: float = Field(description="전망 A등급 확률")
    content_changes: list[str] = Field(description="콘텐츠 변경 제안 목록")
    feasible: bool = Field(description="목표 달성 가능 여부")


# ---------------------------------------------------------------------------
# Sprint 5: Report Generation (REQ-AX-004)
# ---------------------------------------------------------------------------


class ReportGenerateRequest(BaseModel):
    """보고서 생성 요청"""

    criteria_workflow_id: UUID = Field(description="평가기준 인덱싱 워크플로우 ID")
    user_id: str = Field(default="cli-anonymous", description="요청 사용자 ID")


class ReportGenerateResponse(WorkflowResponse):
    """보고서 생성 응답"""


# ---------------------------------------------------------------------------
# Sprint 6: Recommendation (REQ-AX-005)
# ---------------------------------------------------------------------------


class RecommendationGenerateRequest(BaseModel):
    """개선 추천 생성 요청"""

    report_workflow_id: UUID = Field(description="보고서 워크플로우 ID")
    user_id: str = Field(default="cli-anonymous", description="요청 사용자 ID")


class RecommendationFeedbackRequest(BaseModel):
    """추천 피드백 요청"""

    feedback: str = Field(description="피드백 내용")
    helpful: bool = Field(description="도움이 됐는지 여부")
    user_id: str = Field(default="cli-anonymous", description="요청 사용자 ID")
