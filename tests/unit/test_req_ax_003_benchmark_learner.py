"""REQ-AX-003 BenchmarkLearner 단위 테스트 (RED phase)

대상 모듈: pipelines.scoring.benchmark_learner.BenchmarkLearner
AC: AC-003-1 (벤치마크 학습 → 분류기 모델 생성)
    AC-003-3 (학습 중 예측 차단 — BenchmarkLearner state=training)

# @MX:TODO: [AUTO] BenchmarkLearner 미구현 — GREEN phase에서 scikit-learn LogisticRegression 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-003 / AC-003-1 / AC-003-3
"""
from __future__ import annotations

import pytest

# 구현 전 import — RED phase에서 ModuleNotFoundError 예상
from pipelines.scoring.benchmark_learner import BenchmarkLearner  # type: ignore[import]
from pkg.models.simulation import BenchmarkReport


class TestBenchmarkLearnerFeatureExtraction:
    """BenchmarkLearner 특징 추출 계약 테스트 (AC-003-1)"""

    def test_learn_requires_at_least_one_a_and_one_b_report(self) -> None:
        """A 등급과 B 등급 보고서가 각 1개 이상 있어야 학습 가능하다."""
        learner = BenchmarkLearner()
        reports = [
            BenchmarkReport(grade="A", text_content="안전교육 이수율 100% 달성, 안전관리체계 우수", report_id="a-001"),
            BenchmarkReport(grade="B", text_content="안전교육 이수율 85%, 안전사고 1건 발생", report_id="b-001"),
        ]
        # 학습이 에러 없이 완료되어야 한다
        model = learner.learn(reports)
        assert model is not None, "학습 후 분류기 모델을 반환해야 한다"

    def test_learn_with_only_a_grade_reports_raises_error(self) -> None:
        """A 등급 보고서만으로는 2-class 분류기를 학습할 수 없다."""
        learner = BenchmarkLearner()
        reports = [
            BenchmarkReport(grade="A", text_content="우수한 안전관리", report_id="a-001"),
            BenchmarkReport(grade="A", text_content="안전교육 완벽 이수", report_id="a-002"),
        ]
        with pytest.raises((ValueError, RuntimeError)):
            learner.learn(reports)

    def test_learn_with_empty_reports_raises_error(self) -> None:
        """빈 보고서 목록으로 학습 시 에러가 발생해야 한다."""
        learner = BenchmarkLearner()
        with pytest.raises((ValueError, RuntimeError)):
            learner.learn([])

    def test_learn_extracts_tfidf_features_from_korean_text(self) -> None:
        """학습 시 한국어 텍스트에서 TF-IDF 특징을 추출한다."""
        learner = BenchmarkLearner()
        reports = [
            BenchmarkReport(grade="A", text_content="안전교육 이수율 100% 안전관리체계 우수 안전사고 0건", report_id="a-001"),
            BenchmarkReport(grade="B", text_content="안전교육 이수율 85% 안전사고 1건 발생 미흡", report_id="b-001"),
        ]
        learner.learn(reports)
        # 특징 추출기(vectorizer)가 내부적으로 학습되어야 한다
        assert learner.vectorizer is not None, "TF-IDF 벡터라이저가 학습되어야 한다"

    def test_learned_model_supports_predict_proba(self) -> None:
        """학습된 모델은 확률 예측(predict_proba)을 지원해야 한다."""
        learner = BenchmarkLearner()
        reports = [
            BenchmarkReport(grade="A", text_content="안전관리체계 우수 안전교육 100% 이수 사고 0건", report_id="a-001"),
            BenchmarkReport(grade="B", text_content="안전교육 미흡 85% 이수 안전사고 1건 발생 미달", report_id="b-001"),
        ]
        model = learner.learn(reports)
        assert hasattr(model, "predict_proba"), "분류기 모델은 predict_proba 메서드를 가져야 한다"

    def test_learner_state_transitions_to_ready_after_learn(self) -> None:
        """학습 완료 후 BenchmarkLearner 상태는 'ready'여야 한다 (AC-003-3 연관)."""
        learner = BenchmarkLearner()
        assert learner.state == "idle", "초기 상태는 'idle'이어야 한다"
        reports = [
            BenchmarkReport(grade="A", text_content="A등급 안전보건 우수 사례", report_id="a-001"),
            BenchmarkReport(grade="B", text_content="B등급 안전보건 기준 미달 사례", report_id="b-001"),
        ]
        learner.learn(reports)
        assert learner.state == "ready", "학습 완료 후 상태는 'ready'여야 한다"


class TestBenchmarkLearnerTrainingStateBlock:
    """BenchmarkLearner 학습 중 예측 차단 계약 (AC-003-3)"""

    def test_predicting_during_training_raises_model_training_error(self) -> None:
        """BenchmarkLearner가 training 상태일 때 예측 시도는 ModelTrainingError를 발생시킨다."""
        from pipelines.scoring.benchmark_learner import ModelTrainingError  # type: ignore[import]

        learner = BenchmarkLearner()
        # 강제로 training 상태로 설정
        learner.state = "training"
        with pytest.raises(ModelTrainingError):
            learner.predict_grade(text="테스트 보고서 내용")
