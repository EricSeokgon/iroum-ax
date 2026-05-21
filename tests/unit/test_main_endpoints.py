"""FastAPI 엔드포인트 단위 테스트 — TestClient 사용.

SPEC: SPEC-AX-PIPE-001 REQ-AX-001 ~ REQ-AX-005
"""
from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from pipelines.main import app

client = TestClient(app)


class TestHealthEndpoint:
    def test_health_returns_ok(self) -> None:
        r = client.get("/health")
        assert r.status_code == 200
        body = r.json()
        assert body["status"] == "ok"
        assert body["version"] == "0.1.0"


class TestDocumentUploadEndpoint:
    def test_upload_returns_202(self) -> None:
        r = client.post(
            "/api/documents/upload",
            json={"file_type": "PDF", "filename": "report.pdf"},
        )
        assert r.status_code == 202
        data = r.json()
        assert data["status"] == "PENDING"
        assert data["filename"] == "report.pdf"
        assert data["user_id"] == "cli-anonymous"

    def test_upload_custom_user_id(self) -> None:
        r = client.post(
            "/api/documents/upload",
            json={
                "file_type": "HWP",
                "filename": "report.hwp",
                "user_id": "user-001",
            },
        )
        assert r.status_code == 202
        assert r.json()["user_id"] == "user-001"


class TestCriteriaIndexEndpoint:
    def test_index_returns_202(self) -> None:
        r = client.post(
            "/api/criteria/index",
            json={"document_workflow_id": str(uuid.uuid4())},
        )
        assert r.status_code == 202
        assert r.json()["status"] == "PENDING"


class TestCriteriaSearchEndpoint:
    def test_search_returns_200_with_items(self) -> None:
        r = client.get(
            "/api/criteria/search",
            params={"query": "안전보건", "top_k": 3},
        )
        assert r.status_code == 200
        data = r.json()
        assert "items" in data
        assert "total" in data
        assert data["total"] == 0
        assert data["items"] == []


class TestSimulationPredictEndpoint:
    def test_predict_returns_200(self) -> None:
        r = client.post(
            "/api/simulations/predict",
            json={"report_text": "안전교육 이수율 85%", "target_grade": "A"},
        )
        assert r.status_code == 200
        data = r.json()
        assert "current_grade" in data
        assert "projected_p_a" in data
        assert "content_changes" in data
        assert data["target_grade"] == "A"
        assert data["feasible"] is True


class TestReportGenerateEndpoint:
    def test_generate_report_returns_202(self) -> None:
        r = client.post(
            "/api/reports/generate",
            json={"criteria_workflow_id": str(uuid.uuid4())},
        )
        assert r.status_code == 202
        assert r.json()["status"] == "PENDING"


class TestRecommendationsEndpoints:
    def test_generate_recommendations_returns_202(self) -> None:
        r = client.post(
            "/api/recommendations/generate",
            json={"report_workflow_id": str(uuid.uuid4())},
        )
        assert r.status_code == 202
        assert r.json()["status"] == "PENDING"

    def test_submit_feedback_returns_200(self) -> None:
        rec_id = "rec-001"
        r = client.patch(
            f"/api/recommendations/{rec_id}/feedback",
            json={"feedback": "유용함", "helpful": True},
        )
        assert r.status_code == 200
        data = r.json()
        assert data["recommendation_id"] == rec_id
        assert data["feedback_recorded"] is True
