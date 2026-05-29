"""SPEC-AX-001 REQ-AX-004 — generation_worker 단위 테스트"""
from __future__ import annotations

from typing import Any
from unittest.mock import MagicMock, patch

import pytest

from pipelines.workers.generation_worker import TASK_NAME, _execute


class TestTaskName:
    def test_task_name_constant(self) -> None:
        assert TASK_NAME == "pipelines.workers.generation_worker.run"


class TestExecuteSuccess:
    def test_returns_result_json_on_success(self) -> None:
        mock_draft = MagicMock()
        mock_draft.status = "ok"
        mock_draft.model_used = "exaone-3.5-7b"
        mock_draft.retry_count = 0
        mock_draft.text = "합니다. 달성하였습니다."

        with (
            patch("pipelines.workers.generation_worker.ReportDrafter") as MockDrafter,
            patch("pipelines.workers.generation_worker.post_callback") as mock_cb,
        ):
            MockDrafter.return_value.draft_section.return_value = mock_draft

            result = _execute(
                document_id="doc-001",
                workflow_id="wf-001",
                criterion_name="안전관리",
                criterion_detail="안전사고 예방 활동",
                customer_content="이수율 100%",
            )

        assert result["document_id"] == "doc-001"
        assert result["section_status"] == "ok"
        assert result["model_used"] == "exaone-3.5-7b"
        assert result["retry_count"] == 0
        mock_cb.assert_called_once()
        _, kwargs = mock_cb.call_args
        assert kwargs["status"] == "completed"

    def test_style_violation_still_completed(self) -> None:
        mock_draft = MagicMock()
        mock_draft.status = "style_violation"
        mock_draft.model_used = "exaone-3.5-7b"
        mock_draft.retry_count = 2
        mock_draft.text = "합니다."

        with (
            patch("pipelines.workers.generation_worker.ReportDrafter") as MockDrafter,
            patch("pipelines.workers.generation_worker.post_callback") as mock_cb,
        ):
            MockDrafter.return_value.draft_section.return_value = mock_draft
            result = _execute(document_id="doc-002", workflow_id="wf-002")

        assert result["section_status"] == "style_violation"
        _, kwargs = mock_cb.call_args
        assert kwargs["status"] == "completed"


class TestExecuteFailure:
    def test_drafter_exception_sets_failed_status(self) -> None:
        with (
            patch("pipelines.workers.generation_worker.ReportDrafter") as MockDrafter,
            patch("pipelines.workers.generation_worker.post_callback") as mock_cb,
        ):
            MockDrafter.return_value.draft_section.side_effect = RuntimeError("LLM down")
            result = _execute(document_id="doc-003", workflow_id="wf-003")

        assert "error" in result
        _, kwargs = mock_cb.call_args
        assert kwargs["status"] == "failed"

    def test_callback_always_called_on_exception(self) -> None:
        with (
            patch("pipelines.workers.generation_worker.ReportDrafter") as MockDrafter,
            patch("pipelines.workers.generation_worker.post_callback") as mock_cb,
        ):
            MockDrafter.return_value.draft_section.side_effect = ValueError("bad input")
            _execute(document_id="doc-004", workflow_id="wf-004")

        mock_cb.assert_called_once()
