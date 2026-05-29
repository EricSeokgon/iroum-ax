"""SPEC-AX-001 REQ-AX-004 — Celery 비동기 초안 생성 워커

파이프라인: ReportDrafter.draft_section() → post_callback (Go Control Plane)

REQ-INTEG-001 — Celery task name 'pipelines.workers.generation_worker.run' 보존.
REQ-INTEG-002 — 처리 완료 시 Go Control Plane으로 callback POST.
REQ-INTEG-003 — callback fire-and-forget (예외 흡수, D2).

# @MX:NOTE: [AUTO] generation_worker — REQ-AX-004 초안 생성 Celery 진입점
# @MX:SPEC: SPEC-AX-001 REQ-AX-004
"""
from __future__ import annotations

import logging
from typing import Any

from pkg.models.criterion import Criterion

from pipelines.callbacks.control_plane import post_callback
from pipelines.config.settings import settings
from pipelines.generation.report_drafter import ReportDrafter

logger = logging.getLogger(__name__)

# @MX:NOTE: [AUTO] TASK_NAME — Kombu envelope headers.task와 정확 일치 필수 (마법 상수)
# @MX:SPEC: SPEC-AX-001 REQ-AX-004 (Go dispatcher도 동일 문자열 사용)
TASK_NAME: str = "pipelines.workers.generation_worker.run"


def _execute(
    *,
    document_id: str,
    workflow_id: str,
    criterion_name: str = "",
    criterion_detail: str = "",
    customer_content: str = "",
    **kwargs: Any,  # noqa: ANN401
) -> dict[str, Any]:
    """REQ-AX-004 — 보고서 초안 생성 파이프라인.

    # @MX:ANCHOR: [AUTO] _execute — generation_worker 파이프라인 진입점 (fan_in >= 3)
    # @MX:REASON: run task + 단위 테스트 + 통합 테스트에서 호출
    # @MX:SPEC: SPEC-AX-001 REQ-AX-004

    Args:
        document_id: 처리 대상 문서 ID
        workflow_id: 워크플로우 UUID
        criterion_name: 평가기준 이름
        criterion_detail: 평가기준 세부 내용
        customer_content: 고객사 실적 데이터 텍스트

    Returns:
        result_json — {document_id, section_status, model_used, retry_count, spec}
    """
    result_json: dict[str, Any] = {
        "document_id": document_id,
        "spec": "SPEC-AX-001-REQ-AX-004",
    }
    status: str = "completed"

    try:
        criterion = Criterion(
            id=document_id,
            criterion_name=criterion_name,
            criterion_detail=criterion_detail,
        )
        drafter = ReportDrafter()
        draft = drafter.draft_section(criterion, customer_content)

        result_json["section_status"] = draft.status
        result_json["model_used"] = draft.model_used
        result_json["retry_count"] = draft.retry_count
        result_json["text_length"] = len(draft.text)

        if draft.status == "style_violation":
            logger.warning(
                "초안 스타일 위반 (document_id=%s workflow_id=%s retries=%d)",
                document_id,
                workflow_id,
                draft.retry_count,
            )

    except Exception as exc:  # noqa: BLE001
        status = "failed"
        result_json["error"] = str(exc)
        logger.error(
            "초안 생성 실패 (document_id=%s workflow_id=%s): %s",
            document_id,
            workflow_id,
            exc,
        )

    post_callback(
        base_url=settings.go_control_plane_url,
        workflow_id=workflow_id,
        status=status,
        result_json=result_json,
    )

    logger.info(
        "generation worker 완료 (document_id=%s workflow_id=%s status=%s)",
        document_id,
        workflow_id,
        status,
    )
    return result_json


try:  # noqa: SIM105 — celery 미설치 환경 분기 명시
    from pipelines.config.celery_client import create_celery_app

    _app = create_celery_app()

    @_app.task(name=TASK_NAME, bind=True, max_retries=3)  # type: ignore[misc]
    def run(self, document_id: str, *, workflow_id: str, **kwargs: Any) -> dict[str, Any]:  # noqa: ANN001, ANN401, ARG001
        """Celery task entry — REQ-AX-004 초안 생성."""
        return _execute(document_id=document_id, workflow_id=workflow_id, **kwargs)

except ImportError:

    def run(document_id: str, *, workflow_id: str, **kwargs: Any) -> dict[str, Any]:  # type: ignore[no-redef]  # noqa: ANN401
        """단위 테스트 격리용 plain function (celery 미설치 시)."""
        return _execute(document_id=document_id, workflow_id=workflow_id, **kwargs)
