"""REQ-AX-003 통합 테스트 (RED phase)

BenchmarkLearner → GradePredictor → ScenarioSimulator 파이프라인 전체 테스트.

AC:
    AC-003-1 통합: BenchmarkLearner 학습 → GradePredictor 예측 end-to-end
    AC-003-2 통합: abstain 분기 end-to-end 검증
    AC-003-3 통합: model_used 메타데이터 추적 end-to-end

# @MX:TODO: [AUTO] 통합 대상 모듈 미구현 — GREEN phase에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-003-E1 / AC-003-1 / AC-003-2 / AC-003-3
"""
from __future__ import annotations

import pytest

# 구현 전 import — RED phase에서 ModuleNotFoundError 예상
from pipelines.scoring.benchmark_learner import BenchmarkLearner  # type: ignore[import]
from pipelines.scoring.grade_predictor import GradePredictor  # type: ignore[import]
from pkg.models.simulation import BenchmarkReport, GradeDistribution


class TestGradeSimulationPipelineIntegration:
    """BenchmarkLearner → GradePredictor 파이프라인 통합 테스트"""

    @pytest.fixture()
    def a_grade_benchmark(self) -> BenchmarkReport:
        """A 등급 벤치마크 보고서 (acceptance.md AC-003-1 Given)"""
        return BenchmarkReport(
            grade="A",
            text_content=(
                "안전교육 이수율 100% 달성 안전관리체계 우수 KOSHA 인증 안전사고 0건 "
                "안전보건 경영시스템 도입 위험성평가 우수 안전관리자 상주 "
                "비상대응절차 완비 안전점검 주간 실시"
            ),
            report_id="a-kepco-2026",
        )

    @pytest.fixture()
    def b_grade_benchmark(self) -> BenchmarkReport:
        """B 등급 벤치마크 보고서 (acceptance.md AC-003-1 Given)"""
        return BenchmarkReport(
            grade="B",
            text_content=(
                "안전교육 이수율 85% 미달 안전사고 1건 발생 안전관리체계 미흡 "
                "위험성평가 미흡 안전관리 개선 계획 수립 필요 "
                "안전점검 월간 실시 개선 필요"
            ),
            report_id="b-kepco-2026",
        )

    def test_end_to_end_learn_then_predict_returns_grade_distribution(
        self,
        a_grade_benchmark: BenchmarkReport,
        b_grade_benchmark: BenchmarkReport,
    ) -> None:
        """AC-003-1 통합 happy path: 벤치마크 학습 → 예측 → GradeDistribution 반환.

        Given: A 등급 벤치마크 1개 + B 등급 벤치마크 1개
        When: BenchmarkLearner.learn() → GradePredictor.train() → predict()
        Then: GradeDistribution 반환, sum=1.0±0.001
        """
        # BenchmarkLearner로 학습
        learner = BenchmarkLearner()
        reports = [a_grade_benchmark, b_grade_benchmark]
        learner.learn(reports)

        # 학습된 모델로 GradePredictor 초기화
        predictor = GradePredictor()
        predictor.train(reports)

        # 자사 보고서 예측 (A 등급 패턴과 유사)
        company_report = "안전교육 이수율 97% 달성 안전관리체계 우수 안전사고 0건 위험성평가 실시"
        result = predictor.predict(company_report)

        assert isinstance(result, GradeDistribution)
        total = result.p_a + result.p_b + result.p_abstain
        assert abs(total - 1.0) <= 0.001, f"sum={total:.4f} 불변식 위반"

    def test_end_to_end_abstain_branch_with_boundary_report(
        self,
        a_grade_benchmark: BenchmarkReport,
        b_grade_benchmark: BenchmarkReport,
    ) -> None:
        """AC-003-2 통합: 경계 케이스 보고서에서 abstain 분기 검증.

        acceptance.md AC-003-2:
            Given: 특징이 A·B 어느 등급과도 명확히 일치하지 않는 평탄 분포 케이스
            When: grade_predictor가 추론
            Then: status=low_confidence, prediction=null equivalent

        통합 테스트에서는 실제 분류기가 학습 데이터에 없는 패턴에 대해
        확률이 낮게 나오는 경우를 검증하기 위해 mock을 사용한다.
        """
        from unittest.mock import MagicMock

        predictor = GradePredictor()
        # abstain 트리거를 위한 mock 주입
        mock_clf = MagicMock()
        mock_clf.classes_ = ["B", "A"]
        mock_clf.predict_proba.return_value = [[0.45, 0.42]]  # 둘 다 0.5 미만
        mock_vectorizer = MagicMock()
        mock_vectorizer.transform.return_value = [[0.1, 0.2, 0.3]]
        predictor.classifier = mock_clf
        predictor.vectorizer = mock_vectorizer
        predictor._is_trained = True

        result = predictor.predict("경계 케이스 보고서 — A도 B도 아닌 특징")

        assert result.predicted_class == "abstain", "경계 케이스에서 abstain이어야 한다"
        assert result.low_confidence is True, "abstain 시 low_confidence=True여야 한다"
        assert abs(result.p_a + result.p_b + result.p_abstain - 1.0) <= 0.001

    def test_model_used_metadata_propagated_through_pipeline(
        self,
        a_grade_benchmark: BenchmarkReport,
        b_grade_benchmark: BenchmarkReport,
    ) -> None:
        """AC-003-3 통합: model_used 메타데이터가 파이프라인 전체에서 추적된다."""
        predictor = GradePredictor()
        predictor.train([a_grade_benchmark, b_grade_benchmark])
        result = predictor.predict("안전교육 이수율 95% 달성 안전관리체계 우수")
        assert result.model_used, "파이프라인 통합 예측에서 model_used가 비어 있으면 안 된다"
