"""pipelines.config.models — Pydantic 모델 단위 테스트.

Sprint 0-6 모든 요청/응답 모델 검증.
"""
from __future__ import annotations

import uuid

import pytest
from pydantic import ValidationError

from pipelines.config.models import (
    CriterionIndexRequest,
    CriterionSearchItem,
    CriterionSearchRequest,
    CriterionSearchResponse,
    DocumentUploadRequest,
    DocumentUploadResponse,
    FileType,
    Grade,
    HealthResponse,
    RecommendationFeedbackRequest,
    RecommendationGenerateRequest,
    ReportGenerateRequest,
    ReportGenerateResponse,
    SimulationPredictRequest,
    SimulationPredictResponse,
    WorkflowResponse,
    WorkflowStatus,
)


class TestEnums:
    """StrEnum 값 검증"""

    def test_file_type_hwp(self) -> None:
        assert FileType.HWP == "HWP"

    def test_file_type_pdf(self) -> None:
        assert FileType.PDF == "PDF"

    def test_file_type_image(self) -> None:
        assert FileType.IMAGE == "IMAGE"

    def test_workflow_status_values(self) -> None:
        assert WorkflowStatus.PENDING == "PENDING"
        assert WorkflowStatus.RUNNING == "RUNNING"
        assert WorkflowStatus.COMPLETED == "COMPLETED"
        assert WorkflowStatus.FAILED == "FAILED"

    def test_grade_values(self) -> None:
        assert Grade.A == "A"
        assert Grade.B == "B"
        assert Grade.C == "C"
        assert Grade.D == "D"


class TestHealthResponse:
    def test_health_response_fields(self) -> None:
        r = HealthResponse(status="ok", version="0.1.0")
        assert r.status == "ok"
        assert r.version == "0.1.0"


class TestWorkflowResponse:
    def test_workflow_response_default_user(self) -> None:
        wf_id = uuid.uuid4()
        r = WorkflowResponse(workflow_id=wf_id, status=WorkflowStatus.PENDING)
        assert r.user_id == "cli-anonymous"
        assert r.status == WorkflowStatus.PENDING
        assert r.workflow_id == wf_id

    def test_workflow_response_custom_user(self) -> None:
        r = WorkflowResponse(
            workflow_id=uuid.uuid4(),
            status=WorkflowStatus.COMPLETED,
            user_id="test-user",
        )
        assert r.user_id == "test-user"


class TestDocumentUploadModels:
    def test_upload_request_default_user(self) -> None:
        r = DocumentUploadRequest(file_type=FileType.PDF, filename="report.pdf")
        assert r.file_type == FileType.PDF
        assert r.filename == "report.pdf"
        assert r.user_id == "cli-anonymous"

    def test_upload_request_custom_user(self) -> None:
        r = DocumentUploadRequest(
            file_type=FileType.HWP, filename="x.hwp", user_id="user-001"
        )
        assert r.user_id == "user-001"

    def test_upload_response_includes_filename(self) -> None:
        r = DocumentUploadResponse(
            workflow_id=uuid.uuid4(),
            status=WorkflowStatus.PENDING,
            filename="report.pdf",
        )
        assert r.filename == "report.pdf"
        assert r.user_id == "cli-anonymous"


class TestCriterionModels:
    def test_criterion_index_request(self) -> None:
        wf_id = uuid.uuid4()
        r = CriterionIndexRequest(document_workflow_id=wf_id)
        assert r.document_workflow_id == wf_id
        assert r.user_id == "cli-anonymous"

    def test_criterion_search_request_defaults(self) -> None:
        r = CriterionSearchRequest(query="안전보건")
        assert r.query == "안전보건"
        assert r.top_k == 5
        assert r.user_id == "cli-anonymous"

    def test_criterion_search_request_top_k_bounds(self) -> None:
        # top_k 하한
        r = CriterionSearchRequest(query="q", top_k=1)
        assert r.top_k == 1
        # top_k 상한
        r = CriterionSearchRequest(query="q", top_k=20)
        assert r.top_k == 20
        # 범위 초과
        with pytest.raises(ValidationError):
            CriterionSearchRequest(query="q", top_k=0)
        with pytest.raises(ValidationError):
            CriterionSearchRequest(query="q", top_k=21)

    def test_criterion_search_item(self) -> None:
        item = CriterionSearchItem(
            criterion_id="c-001",
            criterion_name="안전보건",
            confidence_score=0.85,
        )
        assert item.criterion_id == "c-001"
        assert item.confidence_score == 0.85

    def test_criterion_search_response(self) -> None:
        item = CriterionSearchItem(
            criterion_id="c-001", criterion_name="안전보건", confidence_score=0.9
        )
        r = CriterionSearchResponse(items=[item], total=1)
        assert r.total == 1
        assert len(r.items) == 1


class TestSimulationModels:
    def test_simulation_predict_request_defaults(self) -> None:
        r = SimulationPredictRequest(report_text="현재 보고서")
        assert r.target_grade == "A"
        assert r.user_id == "cli-anonymous"

    def test_simulation_predict_response(self) -> None:
        r = SimulationPredictResponse(
            current_grade="B",
            target_grade="A",
            current_p_a=0.35,
            projected_p_a=0.65,
            content_changes=["안전교육 강화"],
            feasible=True,
        )
        assert r.current_grade == "B"
        assert r.feasible is True
        assert len(r.content_changes) == 1


class TestReportModels:
    def test_report_generate_request(self) -> None:
        r = ReportGenerateRequest(criteria_workflow_id=uuid.uuid4())
        assert r.user_id == "cli-anonymous"

    def test_report_generate_response(self) -> None:
        r = ReportGenerateResponse(
            workflow_id=uuid.uuid4(), status=WorkflowStatus.PENDING
        )
        assert r.status == WorkflowStatus.PENDING


class TestRecommendationModels:
    def test_recommendation_generate_request(self) -> None:
        r = RecommendationGenerateRequest(report_workflow_id=uuid.uuid4())
        assert r.user_id == "cli-anonymous"

    def test_recommendation_feedback_request(self) -> None:
        r = RecommendationFeedbackRequest(feedback="유용함", helpful=True)
        assert r.feedback == "유용함"
        assert r.helpful is True
        assert r.user_id == "cli-anonymous"
