"""SPEC-AX-INGEST-001 — ScoreTrigger 단위 테스트 (RED 단계)

REQ-INGEST-003 / REQ-INGEST-003b — VectorStore 성공 후 Go 채점 API 호출.
fire-and-forget: 503/504/network/timeout 모두 no-raise + ERROR 로그 + return False.

격리: httpx.MockTransport로 외부 통신 차단.
"""
from __future__ import annotations

import httpx
import pytest

from pipelines.ingestion.score_trigger import ScoreTrigger


class TestScoreTrigger:
    """ScoreTrigger.fire() — fire-and-forget HTTP POST 단위 테스트"""

    def _build_summary(self) -> dict:
        """공통 summary payload 픽스처"""
        return {
            "pages_processed": 5,
            "chunk_count": 12,
            "tokens": 5000,
            "ocr_backend": "transformers_cpu",
        }

    def test_fire_returns_true_on_2xx(self) -> None:
        """HTTP 201 → True 반환"""
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["url"] = str(request.url)
            captured["body"] = request.read().decode("utf-8")
            return httpx.Response(201, json={"score_id": "abc"})

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            result = trigger.fire(
                document_id="doc-001",
                summary=self._build_summary(),
                client=client,
            )

        assert result is True
        # 정확한 URL 조립
        assert "/api/v1/scores" in captured["url"]
        # body는 document_id + summary 키들 포함
        assert "doc-001" in captured["body"]
        assert "transformers_cpu" in captured["body"]

    def test_fire_returns_true_on_200(self) -> None:
        """HTTP 200 → True 반환 (2xx 범위)"""

        def handler(_: httpx.Request) -> httpx.Response:
            return httpx.Response(200)

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            result = trigger.fire(
                document_id="doc-002",
                summary=self._build_summary(),
                client=client,
            )

        assert result is True

    def test_fire_503_returns_false_no_raise(
        self, caplog: pytest.LogCaptureFixture
    ) -> None:
        """HTTP 503 → False, 예외 없이 ERROR 로그"""

        def handler(_: httpx.Request) -> httpx.Response:
            return httpx.Response(503, json={"error": "service unavailable"})

        transport = httpx.MockTransport(handler)
        with caplog.at_level("ERROR"), httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            result = trigger.fire(
                document_id="doc-003",
                summary=self._build_summary(),
                client=client,
            )

        assert result is False
        # ERROR 로그 기록 확인
        assert any("ERROR" == r.levelname for r in caplog.records)

    def test_fire_504_returns_false_no_raise(
        self, caplog: pytest.LogCaptureFixture
    ) -> None:
        """HTTP 504 → False, 예외 없이 ERROR 로그"""

        def handler(_: httpx.Request) -> httpx.Response:
            return httpx.Response(504)

        transport = httpx.MockTransport(handler)
        with caplog.at_level("ERROR"), httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            result = trigger.fire(
                document_id="doc-504",
                summary=self._build_summary(),
                client=client,
            )

        assert result is False

    def test_fire_connect_error_returns_false_no_raise(
        self, caplog: pytest.LogCaptureFixture
    ) -> None:
        """ConnectError → False, 예외 없이 ERROR 로그"""

        def handler(_: httpx.Request) -> httpx.Response:
            raise httpx.ConnectError("connection refused")

        transport = httpx.MockTransport(handler)
        with caplog.at_level("ERROR"), httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            result = trigger.fire(
                document_id="doc-err",
                summary=self._build_summary(),
                client=client,
            )

        assert result is False
        assert any("ERROR" == r.levelname for r in caplog.records)

    def test_fire_timeout_returns_false_no_raise(
        self, caplog: pytest.LogCaptureFixture
    ) -> None:
        """TimeoutException → False, 예외 없이 ERROR 로그"""

        def handler(_: httpx.Request) -> httpx.Response:
            raise httpx.TimeoutException("read timeout")

        transport = httpx.MockTransport(handler)
        with caplog.at_level("ERROR"), httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            result = trigger.fire(
                document_id="doc-timeout",
                summary=self._build_summary(),
                client=client,
            )

        assert result is False

    def test_fire_includes_bearer_token_when_set(self) -> None:
        """token 비어있지 않으면 Authorization: Bearer 헤더 포함"""
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["auth"] = request.headers.get("Authorization", "")
            return httpx.Response(201)

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(
                base_url="http://localhost:8080", token="secret-token-123"
            )
            trigger.fire(
                document_id="doc-auth",
                summary=self._build_summary(),
                client=client,
            )

        assert captured["auth"] == "Bearer secret-token-123"

    def test_fire_omits_auth_header_when_token_empty(self) -> None:
        """token 빈 문자열이면 Authorization 헤더 생략"""
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["auth"] = request.headers.get("Authorization")
            return httpx.Response(201)

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            trigger.fire(
                document_id="doc-no-auth",
                summary=self._build_summary(),
                client=client,
            )

        # 헤더 자체가 없음
        assert captured["auth"] is None

    def test_fire_body_contains_document_id_and_summary_fields(self) -> None:
        """body 스키마 — document_id + summary 4-필드 포함"""
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            import json
            captured["body"] = json.loads(request.read())
            return httpx.Response(201)

        transport = httpx.MockTransport(handler)
        with httpx.Client(transport=transport) as client:
            trigger = ScoreTrigger(base_url="http://localhost:8080", token="")
            trigger.fire(
                document_id="doc-schema",
                summary=self._build_summary(),
                client=client,
            )

        body = captured["body"]
        assert body["document_id"] == "doc-schema"
        assert body["pages_processed"] == 5
        assert body["chunk_count"] == 12
        assert body["tokens"] == 5000
        assert body["ocr_backend"] == "transformers_cpu"
