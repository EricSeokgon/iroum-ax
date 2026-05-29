"""SPEC-AX-001 REQ-AX-003 — simulation_worker 단위 테스트"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from pipelines.workers.simulation_worker import TASK_NAME, _build_benchmark_reports, _execute
from pkg.models.simulation import BenchmarkReport


class TestTaskName:
    def test_task_name_constant(self) -> None:
        assert TASK_NAME == "pipelines.workers.simulation_worker.run"


class TestBuildBenchmarkReports:
    def test_none_returns_default_samples(self) -> None:
        reports = _build_benchmark_reports(None)
        assert len(reports) >= 2
        grades = {r.grade for r in reports}
        assert "A" in grades
        assert "B" in grades

    def test_raw_list_converted(self) -> None:
        raw = [
            {"text": "우수 성과", "grade": "A"},
            {"text": "보통 성과", "grade": "B"},
        ]
        reports = _build_benchmark_reports(raw)
        assert len(reports) == 2
        assert isinstance(reports[0], BenchmarkReport)
        assert reports[0].grade == "A"
        assert reports[0].text_content == "우수 성과"


class TestExecuteSuccess:
    def test_returns_scenario_fields(self) -> None:
        mock_scenario = MagicMock()
        mock_scenario.current_grade = "B"
        mock_scenario.target_grade = "A"
        mock_scenario.current_p_a = 0.3
        mock_scenario.projected_p_a = 0.65
        mock_scenario.content_changes = ["안전교육 이수율 강화"]
        mock_scenario.feasible = True

        with (
            patch("pipelines.workers.simulation_worker.BenchmarkLearner"),
            patch("pipelines.workers.simulation_worker.GradePredictor"),
            patch("pipelines.workers.simulation_worker.ScenarioSimulator") as MockSim,
            patch("pipelines.workers.simulation_worker.post_callback") as mock_cb,
        ):
            MockSim.return_value.simulate.return_value = mock_scenario
            result = _execute(
                workflow_id="wf-001",
                document_id="doc-001",
                report_text="B등급 보고서 내용",
            )

        assert result["current_grade"] == "B"
        assert result["projected_p_a"] == pytest.approx(0.65)
        assert result["feasible"] is True
        _, kwargs = mock_cb.call_args
        assert kwargs["status"] == "completed"


class TestExecuteFailure:
    def test_simulator_exception_sets_failed(self) -> None:
        with (
            patch("pipelines.workers.simulation_worker.BenchmarkLearner"),
            patch("pipelines.workers.simulation_worker.GradePredictor"),
            patch("pipelines.workers.simulation_worker.ScenarioSimulator") as MockSim,
            patch("pipelines.workers.simulation_worker.post_callback") as mock_cb,
        ):
            MockSim.return_value.simulate.side_effect = RuntimeError("predictor missing")
            result = _execute(workflow_id="wf-002", document_id="doc-002")

        assert "error" in result
        _, kwargs = mock_cb.call_args
        assert kwargs["status"] == "failed"

    def test_callback_always_called(self) -> None:
        with (
            patch("pipelines.workers.simulation_worker.BenchmarkLearner") as MockLearner,
            patch("pipelines.workers.simulation_worker.post_callback") as mock_cb,
        ):
            MockLearner.return_value.learn.side_effect = Exception("db error")
            _execute(workflow_id="wf-003", document_id="doc-003")

        mock_cb.assert_called_once()
