"""FastAPI 진입점 — Sprint 0-6 엔드포인트

# @MX:ANCHOR: [AUTO] FastAPI app — 모든 API 진입점의 루트
# @MX:REASON: 7개 엔드포인트(REQ-AX-001~005)가 모두 여기서 등록됨 (fan_in >= 3)
# @MX:SPEC: SPEC-AX-PIPE-001 REQ-AX-001 ~ REQ-AX-005
"""
from __future__ import annotations

import uuid

from fastapi import BackgroundTasks, FastAPI, status
from fastapi.responses import JSONResponse

from pipelines.config.models import (
    CriterionIndexRequest,
    CriterionSearchResponse,
    DocumentUploadRequest,
    DocumentUploadResponse,
    RecommendationFeedbackRequest,
    RecommendationGenerateRequest,
    ReportGenerateRequest,
    ReportGenerateResponse,
    SimulationPredictRequest,
    SimulationPredictResponse,
    WorkflowResponse,
    WorkflowStatus,
)

app = FastAPI(
    title="iroum-ax Pipelines",
    description="한국 공공기관 경영평가 AI 파이프라인 API",
    version="0.1.0",
    docs_url="/docs",
    redoc_url="/redoc",
)


@app.get("/health", response_class=JSONResponse, tags=["system"])
async def health_check() -> dict[str, str]:
    """서비스 상태 확인 (liveness probe)"""
    return {"status": "ok", "version": "0.1.0"}


@app.post(
    "/api/documents/upload",
    response_model=DocumentUploadResponse,
    status_code=status.HTTP_202_ACCEPTED,
    tags=["documents"],
)
async def upload_document(
    request: DocumentUploadRequest,
    background_tasks: BackgroundTasks,
) -> DocumentUploadResponse:
    """문서 업로드 및 비동기 파싱 시작 (REQ-AX-001)"""
    workflow_id = uuid.uuid4()
    # PoC: BackgroundTasks로 비동기 처리 (D7 — Celery 미사용)
    background_tasks.add_task(_noop_background_task, str(workflow_id))
    return DocumentUploadResponse(
        workflow_id=workflow_id,
        status=WorkflowStatus.PENDING,
        user_id=request.user_id,
        filename=request.filename,
    )


@app.post(
    "/api/criteria/index",
    response_model=WorkflowResponse,
    status_code=status.HTTP_202_ACCEPTED,
    tags=["criteria"],
)
async def index_criteria(
    request: CriterionIndexRequest,
    background_tasks: BackgroundTasks,
) -> WorkflowResponse:
    """평가기준 PDF 파싱 및 pgvector 인덱싱 시작 (REQ-AX-002)"""
    workflow_id = uuid.uuid4()
    background_tasks.add_task(_noop_background_task, str(workflow_id))
    return WorkflowResponse(
        workflow_id=workflow_id,
        status=WorkflowStatus.PENDING,
        user_id=request.user_id,
    )


@app.get(
    "/api/criteria/search",
    response_model=CriterionSearchResponse,
    tags=["criteria"],
)
async def search_criteria(
    query: str,
    top_k: int = 5,
    user_id: str = "cli-anonymous",
) -> CriterionSearchResponse:
    """평가기준 유사도 검색 (REQ-AX-002).

    PoC 스텁: 실제 구현은 VectorStore 연동으로 대체.
    """
    _ = (query, top_k, user_id)  # 인자 사용 (PoC 단계 — 추후 VectorStore 연동)
    return CriterionSearchResponse(items=[], total=0)


@app.post(
    "/api/simulations/predict",
    response_model=SimulationPredictResponse,
    tags=["simulations"],
)
async def predict_simulation(
    request: SimulationPredictRequest,
) -> SimulationPredictResponse:
    """B→A 등급 달성 시나리오 시뮬레이션 (REQ-AX-003).

    PoC 스텁: 기본 응답 반환 (실제 구현은 ScenarioSimulator 연동).
    """
    return SimulationPredictResponse(
        current_grade="B",
        target_grade=request.target_grade,
        current_p_a=0.35,
        projected_p_a=0.65,
        content_changes=["안전교육 이수율 100% 달성 내용을 포함하세요"],
        feasible=True,
    )


@app.post(
    "/api/reports/generate",
    response_model=ReportGenerateResponse,
    status_code=status.HTTP_202_ACCEPTED,
    tags=["reports"],
)
async def generate_report(
    request: ReportGenerateRequest,
    background_tasks: BackgroundTasks,
) -> ReportGenerateResponse:
    """경영평가 보고서 초안 생성 시작 (REQ-AX-004)"""
    workflow_id = uuid.uuid4()
    background_tasks.add_task(_noop_background_task, str(workflow_id))
    return ReportGenerateResponse(
        workflow_id=workflow_id,
        status=WorkflowStatus.PENDING,
        user_id=request.user_id,
    )


@app.post(
    "/api/recommendations/generate",
    response_model=WorkflowResponse,
    status_code=status.HTTP_202_ACCEPTED,
    tags=["recommendations"],
)
async def generate_recommendations(
    request: RecommendationGenerateRequest,
    background_tasks: BackgroundTasks,
) -> WorkflowResponse:
    """개선 추천 생성 시작 (REQ-AX-005)"""
    workflow_id = uuid.uuid4()
    background_tasks.add_task(_noop_background_task, str(workflow_id))
    return WorkflowResponse(
        workflow_id=workflow_id,
        status=WorkflowStatus.PENDING,
        user_id=request.user_id,
    )


@app.patch(
    "/api/recommendations/{recommendation_id}/feedback",
    response_model=dict,
    tags=["recommendations"],
)
async def submit_recommendation_feedback(
    recommendation_id: str,
    request: RecommendationFeedbackRequest,
) -> dict:
    """추천 피드백 제출 (REQ-AX-005)"""
    return {
        "recommendation_id": recommendation_id,
        "feedback_recorded": True,
        "user_id": request.user_id,
    }


def _noop_background_task(workflow_id: str) -> None:
    """PoC BackgroundTask 플레이스홀더 (D7 결정).

    실제 구현에서는 워크플로우별 워커 호출로 대체.
    """
    _ = workflow_id
