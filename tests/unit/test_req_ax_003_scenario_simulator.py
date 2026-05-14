"""REQ-AX-003 ScenarioSimulator 단위 테스트 (RED phase)

대상 모듈: pipelines.scoring.scenario_simulator.ScenarioSimulator
계약: B→A 시나리오 시뮬레이션
    - 현재 B 등급 보고서 + 목표 A 등급 → ScenarioResult 반환
    - content_changes: p_a를 0.5 이상으로 올리기 위한 콘텐츠 변경 제안 최소 1개
    - projected_p_a > current_p_a (전망 개선)

# @MX:TODO: [AUTO] ScenarioSimulator 미구현 — GREEN phase에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-003 / REQ-AX-005-E1 (Gap Recommendation에서 소비됨)
"""
from __future__ import annotations

from unittest.mock import MagicMock

import pytest

# 구현 전 import — RED phase에서 ModuleNotFoundError 예상
from pipelines.scoring.scenario_simulator import ScenarioSimulator  # type: ignore[import]
from pkg.models.simulation import GradeDistribution, ScenarioResult


class TestScenarioSimulatorBtoA:
    """ScenarioSimulator B→A 시나리오 시뮬레이션 테스트"""

    @pytest.fixture()
    def simulator_with_mock_predictor(self) -> ScenarioSimulator:
        """mock GradePredictor가 주입된 ScenarioSimulator.

        current 보고서: p_a=0.35, p_b=0.52 (B 등급 우세)
        projected (content 추가 후): p_a=0.65, p_b=0.35 (A 등급으로 전환)
        """
        simulator = ScenarioSimulator()

        # 현재 보고서 예측 (B 등급 우세)
        current_dist = MagicMock(spec=GradeDistribution)
        current_dist.p_a = 0.35
        current_dist.p_b = 0.52
        current_dist.p_abstain = 0.13
        current_dist.predicted_class = "B"

        # content 추가 후 예측 (A 등급 달성)
        projected_dist = MagicMock(spec=GradeDistribution)
        projected_dist.p_a = 0.65
        projected_dist.p_b = 0.35
        projected_dist.p_abstain = 0.0
        projected_dist.predicted_class = "A"

        mock_predictor = MagicMock()
        mock_predictor.predict.side_effect = [current_dist, projected_dist]
        simulator.predictor = mock_predictor
        return simulator

    def test_simulate_returns_scenario_result(
        self, simulator_with_mock_predictor: ScenarioSimulator
    ) -> None:
        """simulate()는 ScenarioResult를 반환해야 한다."""
        result = simulator_with_mock_predictor.simulate(
            current_report_text="안전교육 이수율 85% 안전사고 1건",
            target_grade="A",
        )
        assert isinstance(result, ScenarioResult), "ScenarioResult 인스턴스를 반환해야 한다"

    def test_simulate_projected_p_a_greater_than_current(
        self, simulator_with_mock_predictor: ScenarioSimulator
    ) -> None:
        """B→A 시나리오에서 projected_p_a > current_p_a여야 한다 (개선 효과 있음)."""
        result = simulator_with_mock_predictor.simulate(
            current_report_text="안전교육 이수율 85% 안전사고 1건 발생",
            target_grade="A",
        )
        assert result.projected_p_a > result.current_p_a, (
            f"projected_p_a({result.projected_p_a:.4f}) > current_p_a({result.current_p_a:.4f}) "
            f"여야 하나 그렇지 않다 (개선 효과 없음)"
        )

    def test_simulate_content_changes_not_empty(
        self, simulator_with_mock_predictor: ScenarioSimulator
    ) -> None:
        """simulate() 결과에는 최소 1개의 콘텐츠 변경 제안이 포함되어야 한다."""
        result = simulator_with_mock_predictor.simulate(
            current_report_text="안전교육 이수율 85%",
            target_grade="A",
        )
        assert len(result.content_changes) >= 1, (
            "content_changes는 최소 1개의 변경 제안을 포함해야 한다"
        )

    def test_simulate_target_grade_reflected_in_result(
        self, simulator_with_mock_predictor: ScenarioSimulator
    ) -> None:
        """시뮬레이션 결과의 target_grade가 입력과 일치해야 한다."""
        result = simulator_with_mock_predictor.simulate(
            current_report_text="현재 B 등급 보고서 내용",
            target_grade="A",
        )
        assert result.target_grade == "A", (
            f"target_grade가 'A'여야 하나 '{result.target_grade}' 반환됨"
        )

    def test_simulate_current_grade_is_b_in_b_to_a_scenario(
        self, simulator_with_mock_predictor: ScenarioSimulator
    ) -> None:
        """B→A 시나리오에서 current_grade는 'B'여야 한다."""
        result = simulator_with_mock_predictor.simulate(
            current_report_text="안전교육 이수율 85% 미달 안전사고 1건 발생",
            target_grade="A",
        )
        assert result.current_grade == "B", (
            f"current_grade가 'B'여야 하나 '{result.current_grade}' 반환됨"
        )
