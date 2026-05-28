"""SPEC-AX-INTEG-001 — Python→Go 콜백 fire-and-forget 회복성 검증

REQ-INTEG-002 / REQ-INTEG-003 / AC-INTEG-001-5 검증:
- post_callback(workflow_id, status, result_json)이 Go 콜백 엔드포인트로 POST
- 네트워크 오류·연결 거부·5xx 응답 시 예외를 raise하지 않음 (fire-and-forget)
- ERROR 레벨 로그가 기록됨

격리: httpx의 MockTransport 또는 unittest.mock으로 외부 통신 차단.
"""
from __future__ import annotations

import logging

import httpx
import pytest

# ── helper imports — GREEN 단계에서 모듈을 생성한다 ──────────────────────────────────
# RED 단계에서는 import 자체가 실패할 수 있다. GREEN에서 모듈 생성 후 테스트 통과.
from pipelines.callbacks.control_plane import post_callback  # noqa: E402


class TestPostCallback:
    """post_callback() — fire-and-forget HTTP POST 전송 회복성 단위 테스트"""

    def test_post_callback_success_returns_none(self) -> None:
        """정상 204 응답 — 예외 없이 None 반환"""
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["url"] = str(request.url)
            captured["body"] = request.read().decode("utf-8")
            return httpx.Response(204)

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client:
            result = post_callback(
                base_url="http://localhost:8080",
                workflow_id="11111111-1111-1111-1111-111111111111",
                status="completed",
                result_json={"pages": 5},
                client=client,
            )

        assert result is None
        # AC: 정확한 URL 조립
        assert "/api/v1/workflows/11111111-1111-1111-1111-111111111111/callback" in captured["url"]
        # AC: body 구조 — status + result_json
        assert '"status"' in captured["body"]
        assert '"completed"' in captured["body"]
        assert '"pages"' in captured["body"]

    def test_post_callback_4xx_no_raise(self, caplog: pytest.LogCaptureFixture) -> None:
        """4xx 응답 — 예외 없이 ERROR 로그만"""

        def handler(_: httpx.Request) -> httpx.Response:
            return httpx.Response(409, json={"error": {"code": "INVALID_TRANSITION"}})

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client, caplog.at_level(logging.ERROR):
            # fire-and-forget — 예외가 raise되지 않아야 함
            result = post_callback(
                base_url="http://localhost:8080",
                workflow_id="11111111-1111-1111-1111-111111111111",
                status="completed",
                result_json={},
                client=client,
            )

        assert result is None
        # ERROR 로그 1건 이상 — message에 status code 표기
        assert any("409" in rec.getMessage() for rec in caplog.records)

    def test_post_callback_5xx_no_raise(self, caplog: pytest.LogCaptureFixture) -> None:
        """5xx 응답 — 예외 없이 ERROR 로그만 (Celery ACK 보장)"""

        def handler(_: httpx.Request) -> httpx.Response:
            return httpx.Response(500, text="internal error")

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client, caplog.at_level(logging.ERROR):
            result = post_callback(
                base_url="http://localhost:8080",
                workflow_id="11111111-1111-1111-1111-111111111111",
                status="failed",
                result_json={"error": "parse"},
                client=client,
            )

        assert result is None
        assert any("500" in rec.getMessage() for rec in caplog.records)

    def test_post_callback_connection_error_no_raise(self, caplog: pytest.LogCaptureFixture) -> None:
        """연결 오류 (서버 미기동) — 예외 없이 ERROR 로그만"""

        def handler(_: httpx.Request) -> httpx.Response:
            raise httpx.ConnectError("connection refused")

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client, caplog.at_level(logging.ERROR):
            # AC-INTEG-001-5 — 네트워크 오류 시 예외 raise 금지
            result = post_callback(
                base_url="http://localhost:8080",
                workflow_id="11111111-1111-1111-1111-111111111111",
                status="completed",
                result_json={},
                client=client,
            )

        assert result is None
        # ERROR 로그가 기록되어야 함
        assert len(caplog.records) >= 1
        # 에러 메시지에 callback 또는 connect 단어 포함
        assert any(
            "callback" in rec.getMessage().lower() or "connect" in rec.getMessage().lower()
            for rec in caplog.records
        )

    def test_post_callback_timeout_no_raise(self, caplog: pytest.LogCaptureFixture) -> None:
        """타임아웃 — 예외 없이 ERROR 로그만"""

        def handler(_: httpx.Request) -> httpx.Response:
            raise httpx.TimeoutException("request timed out")

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client, caplog.at_level(logging.ERROR):
            result = post_callback(
                base_url="http://localhost:8080",
                workflow_id="11111111-1111-1111-1111-111111111111",
                status="completed",
                result_json={},
                client=client,
            )

        assert result is None
        assert len(caplog.records) >= 1

    def test_post_callback_body_structure(self) -> None:
        """request body — JSON 직렬화 정확성: {"status": ..., "result_json": ...}"""
        import json

        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["body"] = json.loads(request.read().decode("utf-8"))
            captured["content_type"] = request.headers.get("content-type", "")
            return httpx.Response(204)

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client:
            post_callback(
                base_url="http://localhost:8080",
                workflow_id="abc",
                status="failed",
                result_json={"error": "parse_error", "step": "ingestion"},
                client=client,
            )

        assert captured["body"] == {
            "status": "failed",
            "result_json": {"error": "parse_error", "step": "ingestion"},
        }
        assert "application/json" in captured["content_type"]
