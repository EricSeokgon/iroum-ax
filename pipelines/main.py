"""FastAPI 진입점 — Sprint 0 스켈레톤

실제 엔드포인트 구현은 Sprint 2-6에서 순차적으로 추가됨.
현재는 /health 엔드포인트만 활성화.
"""
from fastapi import FastAPI
from fastapi.responses import JSONResponse

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


# TODO(Sprint 2): POST /api/documents/upload (REQ-AX-001)
# TODO(Sprint 3): POST /api/criteria/index, GET /api/criteria/search (REQ-AX-002)
# TODO(Sprint 4): POST /api/simulations/predict (REQ-AX-003)
# TODO(Sprint 5): POST /api/reports/generate (REQ-AX-004)
# TODO(Sprint 6): POST /api/recommendations/generate, PATCH /api/recommendations/{id}/feedback (REQ-AX-005)
