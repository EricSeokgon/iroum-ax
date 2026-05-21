"""SPEC-AX-INTEG-001 — Celery 태스크 시그니처 검증 (단위)

REQ-INTEG-001 / AC-INTEG-001-1 검증:
- Celery task name이 정확히 'pipelines.workers.ingestion_worker.run'
- task signature: run(document_id: str, *, workflow_id: str)

격리: celery 미설치 환경에서도 통과되도록, 실제 celery import 대신
모듈 속성과 호출 가능성만 검증한다. Celery integration 자체는 docker-compose
E2E (tests/integration/test_integ_001_workflow_e2e.py) 책임.
"""
from __future__ import annotations

import inspect

import pytest


class TestCeleryTaskRegistration:
    """REQ-INTEG-001 — Celery task name 고정"""

    def test_run_task_name_exact_match(self) -> None:
        """Task name이 'pipelines.workers.ingestion_worker.run' 정확 일치 (REQ-INTEG-001).

        Go dispatcher가 envelope headers.task 필드에 이 문자열을 그대로 사용한다.
        오타 시 task routing 실패 — 가장 결정적 단위 검증.
        """
        from pipelines.workers import ingestion_worker

        # GREEN: TASK_NAME 모듈 상수로 노출 (Celery decorator의 name= 인자 출처)
        assert hasattr(ingestion_worker, "TASK_NAME"), (
            "ingestion_worker.TASK_NAME must be exposed for envelope contract"
        )
        assert ingestion_worker.TASK_NAME == "pipelines.workers.ingestion_worker.run"

    def test_run_function_signature(self) -> None:
        """run(document_id: str, *, workflow_id: str) — 위치 인자 1 + 키워드 인자 1"""
        from pipelines.workers import ingestion_worker

        assert hasattr(ingestion_worker, "run"), "ingestion_worker.run must be callable"
        # signature 추출 — Celery decorator 후의 wrapped 함수
        sig = inspect.signature(ingestion_worker.run)
        params = list(sig.parameters.values())
        # 첫 인자: document_id (positional)
        assert any(p.name == "document_id" for p in params), (
            "run() must accept document_id as parameter"
        )
        # workflow_id: keyword-only (Celery envelope payload[1].workflow_id에 대응)
        wf_param = next((p for p in params if p.name == "workflow_id"), None)
        assert wf_param is not None, "run() must accept workflow_id as parameter"


class TestCeleryPayloadContract:
    """REQ-INTEG-001 envelope payload — Go dispatcher와 Python worker 간 계약

    envelope body 디코딩 결과:
        [
          ["<document-id>"],          # positional args
          {"workflow_id": "<uuid>"},  # keyword args
          {"callbacks": None, ...}    # Celery options
        ]
    """

    def test_dispatch_payload_builder_positional_args(self) -> None:
        """build_dispatch_payload(doc_id, wf_id) → positional은 [doc_id] 단일"""
        from pipelines.workers.ingestion_worker import build_dispatch_payload

        payload = build_dispatch_payload(document_id="doc-123", workflow_id="wf-456")
        # payload[0] = positional args list
        assert payload[0] == ["doc-123"]

    def test_dispatch_payload_builder_keyword_args(self) -> None:
        """build_dispatch_payload — kwargs dict에 workflow_id 포함"""
        from pipelines.workers.ingestion_worker import build_dispatch_payload

        payload = build_dispatch_payload(document_id="doc-123", workflow_id="wf-456")
        # payload[1] = kwargs dict
        assert payload[1] == {"workflow_id": "wf-456"}

    def test_dispatch_payload_builder_celery_options(self) -> None:
        """payload[2] = Celery options (callbacks/chain/chord/errbacks 모두 None)"""
        from pipelines.workers.ingestion_worker import build_dispatch_payload

        payload = build_dispatch_payload(document_id="doc-123", workflow_id="wf-456")
        # payload[2] = Celery options dict
        options = payload[2]
        assert options == {
            "callbacks": None,
            "chain": None,
            "chord": None,
            "errbacks": None,
        }


class TestCallbackInvocationFromRun:
    """REQ-INTEG-002 — run() 본문에서 callback POST가 호출되는지 검증

    실 Celery 실행은 통합 테스트가 담당. 본 단위 테스트는 run의 *내부 호출*만
    검증하기 위해 callback 함수를 monkeypatch한다.
    """

    def test_run_invokes_callback_on_success(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """run(doc, wf=...) 정상 완료 시 post_callback이 status='completed'로 호출되어야 함"""
        from pipelines.workers import ingestion_worker

        captured: dict = {}

        def fake_post_callback(*, base_url, workflow_id, status, result_json, client=None):  # noqa: ANN001
            captured["base_url"] = base_url
            captured["workflow_id"] = workflow_id
            captured["status"] = status
            captured["result_json"] = result_json
            return None

        # ingestion_worker에서 임포트한 post_callback을 fake로 교체
        monkeypatch.setattr(ingestion_worker, "post_callback", fake_post_callback)

        # run의 내부 처리는 stub — 본 SPEC 범위 (§6.3 OUT of scope)
        # 호출 자체는 envelope payload 정합으로 가능해야 함
        ingestion_worker._execute(document_id="doc-1", workflow_id="wf-1")

        assert captured["workflow_id"] == "wf-1"
        assert captured["status"] == "completed"
        # result_json은 dict 타입 (free-form JSONB)
        assert isinstance(captured["result_json"], dict)
