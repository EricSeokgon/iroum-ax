"""SPEC-AX-INTEG-001 — Celery ingestion worker (스켈레톤)

REQ-INTEG-001 — Task name 'pipelines.workers.ingestion_worker.run' (고정).
REQ-INTEG-002 — 처리 완료 시 Go Control Plane으로 callback POST.
REQ-INTEG-003 — callback 실패는 ACK (fire-and-forget, D2).

비즈니스 로직 본체(ingestion 파싱, VLM OCR, RAG 매핑 등)는 후속 SPEC에서
점진 추가. 본 SPEC은 통합 골격만 보장 — _execute는 스텁 처리.
"""
from __future__ import annotations

import logging
from typing import Any

from pipelines.callbacks.control_plane import post_callback
from pipelines.config.settings import settings

logger = logging.getLogger(__name__)

# @MX:NOTE: [AUTO] TASK_NAME — Kombu envelope headers.task와 정확 일치 필수 (마법 상수)
# @MX:SPEC: SPEC-AX-INTEG-001 REQ-INTEG-001 (Go dispatcher도 동일 문자열 사용 — 양쪽 동시 변경)
# REQ-INTEG-001 — Celery task name 고정 (envelope headers.task와 정확 일치 필수)
# Go dispatcher가 이 문자열을 사용하므로 변경 시 양쪽 동시 수정 필요.
TASK_NAME: str = "pipelines.workers.ingestion_worker.run"


def build_dispatch_payload(*, document_id: str, workflow_id: str) -> list[Any]:
    """REQ-INTEG-001 — Celery envelope payload 구조 빌더.

    Kombu v2 envelope body는 base64 디코딩 후 다음 3-tuple 리스트 형태:
        [
          ["<document_id>"],          # positional args
          {"workflow_id": "<uuid>"},  # kwargs
          {"callbacks": None, ...}    # Celery options
        ]

    Go dispatcher와 동일한 payload 구조를 산출하여 contract 정합을 단위 테스트로
    검증 가능하게 한다. 실 envelope 직렬화(base64 + JSON)는 Go scheduler 책임.
    """
    return [
        [document_id],
        {"workflow_id": workflow_id},
        {
            "callbacks": None,
            "chain": None,
            "chord": None,
            "errbacks": None,
        },
    ]


def _execute(*, document_id: str, workflow_id: str) -> dict[str, Any]:
    """REQ-INTEG-002 — task 본문: 스텁 처리 + callback POST.

    본 SPEC 범위에서는 비즈니스 로직 없이 즉시 callback 'completed' 전송.
    후속 SPEC에서 이 함수 내부에 ingestion/VLM/RAG/scoring 단계 점진 주입.

    Returns:
        result_json — Celery task 반환값 (디버그용, callback에 이미 전송됨)
    """
    # SPEC-AX-INTEG-001 §6.3 OUT of scope — 비즈니스 로직 본체는 후속 SPEC.
    # 본 스켈레톤은 즉시 'completed'를 보고하여 통합 골격을 완결한다.
    result_json: dict[str, Any] = {
        "document_id": document_id,
        "stub": True,
        "spec": "SPEC-AX-INTEG-001",
    }

    # REQ-INTEG-002 — callback POST. REQ-INTEG-003 — fire-and-forget (예외 흡수).
    post_callback(
        base_url=settings.go_control_plane_url,
        workflow_id=workflow_id,
        status="completed",
        result_json=result_json,
    )

    logger.info(
        "ingestion worker stub completed (document_id=%s workflow_id=%s)",
        document_id,
        workflow_id,
    )
    return result_json


# ─────────────────────────────────────────────────────────────────────────────
# Celery task decorator — celery 패키지 미설치 환경(단위 테스트)에서는 plain function.
# 설치된 환경에서는 @app.task(name=TASK_NAME)로 등록되어 Redis dequeue 시 호출됨.
# ─────────────────────────────────────────────────────────────────────────────

try:  # noqa: SIM105 — celery 미설치 환경 분기 명시
    from pipelines.config.celery_client import create_celery_app

    _app = create_celery_app()

    @_app.task(name=TASK_NAME, bind=True, max_retries=3)  # type: ignore[misc]
    def run(self, document_id: str, *, workflow_id: str) -> dict[str, Any]:  # noqa: ANN001, ARG001
        """Celery task entry — REQ-INTEG-001 정확 task name 매칭.

        AC-INTEG-001-1: Go dispatcher RPUSH 후 Python worker dequeue → 본 함수 호출.
        """
        return _execute(document_id=document_id, workflow_id=workflow_id)

except ImportError:
    # celery 미설치 — 단위 테스트 격리 모드. run을 plain function으로 노출.
    # 단위 테스트는 _execute을 직접 호출하거나 build_dispatch_payload만 검증.

    def run(document_id: str, *, workflow_id: str) -> dict[str, Any]:  # type: ignore[no-redef]
        """단위 테스트 격리용 plain function (celery 미설치 시)."""
        return _execute(document_id=document_id, workflow_id=workflow_id)
