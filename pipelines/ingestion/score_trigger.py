"""SPEC-AX-INGEST-001 — Go 채점 API 트리거 클라이언트

# @MX:NOTE: [AUTO] REQ-INGEST-003 fire-and-forget — never raise; INTEG-001 §4 동형
# @MX:SPEC: SPEC-AX-INGEST-001 REQ-INGEST-003b

VectorStore.upsert() 성공 후 Go Control Plane으로 채점 트리거 POST 전송.
503/504/network/timeout 모두 흡수 — Celery ACK 보장 및 status='completed' 유지.
"""
from __future__ import annotations

import logging
from typing import Any

import httpx

logger = logging.getLogger(__name__)

# 채점 트리거 타임아웃 (초) — fire-and-forget 짧은 timeout으로 빠른 fallback
_TRIGGER_TIMEOUT_SECONDS: float = 5.0


class ScoreTrigger:
    """Go Control Plane 채점 API 트리거 클라이언트.

    Args:
        base_url: Go Control Plane base URL (예: 'http://localhost:8080')
        token: SCORE_API_TOKEN — 빈 문자열이면 Authorization 헤더 생략

    Usage:
        trigger = ScoreTrigger(base_url=settings.go_control_plane_url,
                                token=settings.score_api_token)
        ok = trigger.fire(document_id="doc-1", summary={...})
    """

    def __init__(self, base_url: str, token: str = "") -> None:
        self._base_url = base_url.rstrip("/")
        self._token = token

    def fire(
        self,
        document_id: str,
        summary: dict[str, Any],
        *,
        client: httpx.Client | None = None,
    ) -> bool:
        """POST {base_url}/api/v1/scores — fire-and-forget 채점 트리거.

        Args:
            document_id: 처리 완료된 문서 ID
            summary: pages_processed/chunk_count/tokens/ocr_backend 메타데이터
            client: httpx.Client 주입 (테스트용); None이면 내부 생성 후 close

        Returns:
            True — 2xx 응답 수신, False — 4xx/5xx/네트워크/타임아웃 (모두 흡수).
        """
        url = f"{self._base_url}/api/v1/scores"
        body: dict[str, Any] = {"document_id": document_id, **summary}
        headers: dict[str, str] = {"Content-Type": "application/json"}
        if self._token:
            headers["Authorization"] = f"Bearer {self._token}"

        # 외부 client 주입 시 close 책임 없음 (테스트 격리)
        if client is not None:
            return self._do_post(url, body, headers, client, document_id)

        try:
            with httpx.Client(timeout=_TRIGGER_TIMEOUT_SECONDS) as fresh:
                return self._do_post(url, body, headers, fresh, document_id)
        except Exception as exc:  # noqa: BLE001 — fire-and-forget
            # 클라이언트 생성 자체 실패까지 흡수
            logger.error(
                "채점 트리거 클라이언트 생성 실패 (document_id=%s url=%s): %s",
                document_id,
                url,
                exc,
            )
            return False

    @staticmethod
    def _do_post(
        url: str,
        body: dict[str, Any],
        headers: dict[str, str],
        client: httpx.Client,
        document_id: str,
    ) -> bool:
        """단일 POST 시도 — 모든 예외 흡수하여 bool로 반환."""
        try:
            resp = client.post(url, json=body, headers=headers)
        except httpx.TimeoutException as exc:
            logger.error(
                "채점 트리거 타임아웃 (document_id=%s url=%s): %s",
                document_id,
                url,
                exc,
            )
            return False
        except httpx.ConnectError as exc:
            logger.error(
                "채점 트리거 연결 실패 (document_id=%s url=%s): %s",
                document_id,
                url,
                exc,
            )
            return False
        except httpx.HTTPError as exc:
            logger.error(
                "채점 트리거 HTTP 오류 (document_id=%s url=%s): %s",
                document_id,
                url,
                exc,
            )
            return False

        if 200 <= resp.status_code < 300:
            return True

        logger.error(
            "채점 트리거 비정상 응답 (document_id=%s status=%d url=%s body=%s)",
            document_id,
            resp.status_code,
            url,
            resp.text[:500],
        )
        return False
