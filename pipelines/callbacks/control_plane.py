"""SPEC-AX-INTEG-001 — Python→Go Control Plane 콜백 HTTP 클라이언트

REQ-INTEG-002 / REQ-INTEG-003 (fire-and-forget):
- POST {GO_CONTROL_PLANE_URL}/api/v1/workflows/{workflow_id}/callback
- body: {"status": "completed"|"failed", "result_json": {...}}
- 네트워크 오류·4xx·5xx 응답 시 예외를 raise하지 *않고* ERROR 로그만 남긴다.
  Celery worker가 task ACK를 정상 수행하도록 보장 (D2).

D2 위험 명시: callback 단발 실패 시 Go가 RUNNING 상태로 영구 고착될 수 있다.
완화책(본 SPEC 범위 외): 향후 timeout-based cleanup SPEC에서 stale RUNNING
워크플로우를 FAILED로 강제 전이.
"""
from __future__ import annotations

import logging
from typing import Any

import httpx

logger = logging.getLogger(__name__)

# 콜백 타임아웃 (초) — Go 콜백 핸들러는 단일 TX 내 GetWorkflow+Update+Audit+Commit
# 4단계 + p99 100ms 미만 예상. 5초는 RTT + DB hiccup 여유분.
_CALLBACK_TIMEOUT_SECONDS: float = 5.0


def post_callback(
    *,
    base_url: str,
    workflow_id: str,
    status: str,
    result_json: dict[str, Any],
    client: httpx.Client | None = None,
) -> None:
    """Go Control Plane 워크플로우 콜백 엔드포인트로 POST 전송 (fire-and-forget).

    Args:
        base_url: Go Control Plane base URL (예: 'http://localhost:8080')
        workflow_id: 워크플로우 UUID 문자열
        status: 'completed' 또는 'failed' (Go 핸들러가 enum-validate)
        result_json: free-form JSONB payload (생략 가능, 빈 dict는 빈 객체로 영속화)
        client: httpx.Client 인스턴스 (테스트 주입용) — None이면 default client 생성

    Returns:
        None — 항상 None. 실패 시에도 예외 raise 없음 (D2 fire-and-forget).

    Side effects:
        - 성공: 정보 없음 (204 No Content)
        - 실패: ERROR 로그 기록 (Celery ACK 보장)
    """
    url = f"{base_url.rstrip('/')}/api/v1/workflows/{workflow_id}/callback"
    body: dict[str, Any] = {
        "status": status,
        "result_json": result_json,
    }
    headers = {"Content-Type": "application/json"}

    # 외부 client 주입 시 close 책임 없음 (테스트 격리). 내부 생성 시 with-context로 close.
    if client is not None:
        _do_post(url, body, headers, client, workflow_id)
        return

    try:
        with httpx.Client(timeout=_CALLBACK_TIMEOUT_SECONDS) as fresh:
            _do_post(url, body, headers, fresh, workflow_id)
    except Exception as exc:  # noqa: BLE001 — fire-and-forget, log-only
        # 클라이언트 생성 자체의 실패까지 흡수 — Celery ACK 보장
        logger.error(
            "워크플로우 콜백 클라이언트 생성 실패 (workflow_id=%s url=%s): %s",
            workflow_id,
            url,
            exc,
        )


def _do_post(
    url: str,
    body: dict[str, Any],
    headers: dict[str, str],
    client: httpx.Client,
    workflow_id: str,
) -> None:
    """httpx.Client 한 인스턴스로 단일 POST 시도 — 모든 예외 흡수."""
    try:
        resp = client.post(url, json=body, headers=headers)
    except httpx.TimeoutException as exc:
        logger.error(
            "워크플로우 콜백 타임아웃 (workflow_id=%s url=%s): %s",
            workflow_id,
            url,
            exc,
        )
        return
    except httpx.ConnectError as exc:
        logger.error(
            "워크플로우 콜백 연결 실패 (workflow_id=%s url=%s): %s",
            workflow_id,
            url,
            exc,
        )
        return
    except httpx.HTTPError as exc:
        logger.error(
            "워크플로우 콜백 HTTP 오류 (workflow_id=%s url=%s): %s",
            workflow_id,
            url,
            exc,
        )
        return

    # 4xx/5xx — 예외 대신 ERROR 로그만 (fire-and-forget)
    if resp.status_code >= 400:
        logger.error(
            "워크플로우 콜백 비정상 응답 (workflow_id=%s status=%d url=%s body=%s)",
            workflow_id,
            resp.status_code,
            url,
            resp.text[:500],
        )
