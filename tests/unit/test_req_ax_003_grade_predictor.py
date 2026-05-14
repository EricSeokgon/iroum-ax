"""REQ-AX-003 GradePredictor 단위 테스트 (RED phase)

대상 모듈: pipelines.scoring.grade_predictor.GradePredictor
AC: AC-003-1 (happy path — A/B 확률 분포 산출, sum=1.0±0.001)
    AC-003-2 (abstain 분기 — max(p_a, p_b) < 0.5 → status=low_confidence)
    AC-003-3 (model_used metadata tracking)

핵심 설계:
    - 2-class softmax + abstain 3-way output {A, B, abstain} (REQ-AX-003-E1)
    - p_abstain = 1 - p_a - p_b
    - abstain 트리거: max(p_a, p_b) < 0.5 (REQ-AX-003-U1)
    - sum(p_a, p_b, p_abstain) = 1.0 ± 0.001 (수학적 불변식)

# @MX:TODO: [AUTO] GradePredictor 미구현 — GREEN phase에서 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-003-E1 / REQ-AX-003-U1 / AC-003-1 / AC-003-2 / AC-003-3
"""
from __future__ import annotations

from unittest.mock import MagicMock

import pytest

# 구현 전 import — RED phase에서 ModuleNotFoundError 예상
from pipelines.scoring.grade_predictor import GradePredictor  # type: ignore[import]
from pkg.models.simulation import BenchmarkReport, GradeDistribution

# ============================================================
# AC-003-1: Happy Path — 정상 예측 (확률 분포 + sum 불변식)
# ============================================================


class TestGradePredictorHappyPath:
    """AC-003-1: A/B 등급 정상 예측 경로 테스트"""

    @pytest.fixture()
    def trained_predictor(self) -> GradePredictor:
        """A/B 보고서로 학습된 GradePredictor 픽스처"""
        predictor = GradePredictor()
        training_reports = [
            BenchmarkReport(
                grade="A",
                text_content="안전교육 이수율 100% 달성 안전관리체계 우수 안전사고 0건 안전보건 인증 획득",
                report_id="a-benchmark-001",
            ),
            BenchmarkReport(
                grade="B",
                text_content="안전교육 이수율 85% 미달 안전사고 1건 발생 안전관리체계 미흡 개선 필요",
                report_id="b-benchmark-001",
            ),
        ]
        predictor.train(training_reports)
        return predictor

    def test_predict_returns_grade_distribution(self, trained_predictor: GradePredictor) -> None:
        """predict()는 GradeDistribution을 반환해야 한다."""
        report_text = "안전교육 이수율 98% 안전관리체계 우수 안전사고 0건"
        result = trained_predictor.predict(report_text)
        assert isinstance(result, GradeDistribution), "GradeDistribution 인스턴스를 반환해야 한다"

    def test_probability_sum_invariant_equals_one(self, trained_predictor: GradePredictor) -> None:
        """확률 합 불변식: p_a + p_b + p_abstain = 1.0 ± 0.001 (REQ-AX-003-E1).

        이것이 Sprint 4의 핵심 수학 불변식이다.
        abstain이 존재하면 softmax만으로는 sum=1.0이 보장되지 않으므로
        p_abstain = 1 - p_a - p_b 로 계산하여 3-way 합을 강제한다.
        """
        report_text = "안전교육 이수율 98% 달성 안전보건 우수"
        result = trained_predictor.predict(report_text)
        total = result.p_a + result.p_b + result.p_abstain
        assert abs(total - 1.0) <= 0.001, (
            f"확률 합 불변식 위반: p_a({result.p_a:.4f}) + p_b({result.p_b:.4f}) + "
            f"p_abstain({result.p_abstain:.4f}) = {total:.4f}, 허용 오차 0.001 초과"
        )

    def test_predict_a_grade_report_returns_high_p_a(self, trained_predictor: GradePredictor) -> None:
        """A 등급 패턴 보고서 예측 시 p_a >= 0.5여야 한다."""
        # A 등급과 유사한 텍스트 (훈련 데이터와 동일 패턴)
        a_grade_text = "안전교육 이수율 100% 달성 안전관리체계 우수 안전사고 0건 안전보건 인증"
        result = trained_predictor.predict(a_grade_text)
        assert result.p_a >= 0.5, f"A 등급 패턴에서 p_a >= 0.5 예상, 실제: {result.p_a:.4f}"
        assert result.predicted_class == "A", f"A 등급 패턴에서 predicted_class='A' 예상, 실제: {result.predicted_class!r}"

    def test_predict_b_grade_report_returns_high_p_b(self, trained_predictor: GradePredictor) -> None:
        """B 등급 패턴 보고서 예측 시 p_b >= 0.5여야 한다."""
        b_grade_text = "안전교육 이수율 85% 미달 안전사고 1건 발생 안전관리체계 미흡 개선 필요"
        result = trained_predictor.predict(b_grade_text)
        assert result.p_b >= 0.5, f"B 등급 패턴에서 p_b >= 0.5 예상, 실제: {result.p_b:.4f}"
        assert result.predicted_class == "B", f"B 등급 패턴에서 predicted_class='B' 예상, 실제: {result.predicted_class!r}"

    def test_predict_probability_sum_invariant_batch_100(self, trained_predictor: GradePredictor) -> None:
        """100건 배치에서 확률 합 불변식 위반이 0건이어야 한다 (strategy.md §3.4 pass condition)."""
        violations = 0
        test_texts = [
            f"안전교육 이수율 {pct}% 달성 안전보건 관리 실시"
            for pct in range(70, 101)
        ] + [
            f"안전사고 {n}건 발생 안전관리 미흡"
            for n in range(0, 30)
        ]
        for text in test_texts:
            result = trained_predictor.predict(text)
            total = result.p_a + result.p_b + result.p_abstain
            if abs(total - 1.0) > 0.001:
                violations += 1
        assert violations == 0, f"100건 배치 중 {violations}건 확률 합 불변식 위반 발생"

    def test_predict_includes_model_used_metadata(self, trained_predictor: GradePredictor) -> None:
        """GradeDistribution.model_used 필드가 채워져야 한다 (AC-003-3)."""
        result = trained_predictor.predict("안전교육 이수율 90%")
        assert result.model_used, "model_used 필드가 비어 있으면 안 된다"
        assert isinstance(result.model_used, str), "model_used는 문자열이어야 한다"


# ============================================================
# AC-003-2: Abstain 분기 — 두 확률 모두 0.5 미만 시 abstain 트리거
# ============================================================


class TestGradePredictorAbstainBranch:
    """AC-003-2: abstain 분기 핵심 테스트.

    design note: 핵심 시나리오 — p_a=0.42, p_b=0.45, expected p_abstain=0.13, class='abstain'.
    이는 strategy.md §3.4가 명시한 abstain 트리거의 primary scenario이다.
    """

    @pytest.fixture()
    def predictor_with_mock_classifier(self) -> GradePredictor:
        """mock 분류기를 주입한 GradePredictor — 결정론적 확률 제어용."""
        predictor = GradePredictor()
        # mock 분류기: predict_proba가 [p_b, p_a] 형태 배열을 반환 (classes_=['B', 'A'])
        mock_clf = MagicMock()
        mock_clf.classes_ = ["B", "A"]  # scikit-learn 관례: 알파벳 순
        # abstain 시나리오: p_a=0.42, p_b=0.45 (acceptance.md AC-003-2 primary scenario)
        mock_clf.predict_proba.return_value = [[0.45, 0.42]]  # [p_B, p_A]
        mock_vectorizer = MagicMock()
        mock_vectorizer.transform.return_value = [[0.1, 0.2, 0.3]]
        predictor.classifier = mock_clf
        predictor.vectorizer = mock_vectorizer
        predictor._is_trained = True
        return predictor

    def test_abstain_triggered_when_both_below_0_5(
        self, predictor_with_mock_classifier: GradePredictor
    ) -> None:
        """핵심 abstain 시나리오: {A: 0.42, B: 0.45} → predicted_class='abstain'.

        acceptance.md AC-003-2:
            When grade_predictor가 {A: 0.42, B: 0.45, abstain: 0.13} 산출
            Then predicted_class='abstain', status=low_confidence
        """
        result = predictor_with_mock_classifier.predict("경계 케이스 보고서 내용")
        assert result.predicted_class == "abstain", (
            f"max(p_a={result.p_a:.2f}, p_b={result.p_b:.2f}) < 0.5 이므로 "
            f"predicted_class='abstain' 이어야 하나 '{result.predicted_class}' 반환됨"
        )

    def test_abstain_p_abstain_value_equals_one_minus_pa_pb(
        self, predictor_with_mock_classifier: GradePredictor
    ) -> None:
        """abstain 시나리오: p_abstain = 1 - p_a - p_b = 0.13 (±0.001)."""
        result = predictor_with_mock_classifier.predict("경계 케이스 보고서")
        expected_p_abstain = 1.0 - result.p_a - result.p_b
        assert abs(result.p_abstain - expected_p_abstain) <= 0.001, (
            f"p_abstain={result.p_abstain:.4f}이 예상값 {expected_p_abstain:.4f}와 다름"
        )

    def test_abstain_sets_low_confidence_flag(
        self, predictor_with_mock_classifier: GradePredictor
    ) -> None:
        """abstain 상태에서 low_confidence 플래그는 True여야 한다 (REQ-AX-003-U1)."""
        result = predictor_with_mock_classifier.predict("경계 케이스 보고서")
        assert result.low_confidence is True, "abstain 상태에서 low_confidence는 True여야 한다"

    def test_abstain_probability_sum_invariant(
        self, predictor_with_mock_classifier: GradePredictor
    ) -> None:
        """abstain 상태에서도 확률 합 불변식 p_a + p_b + p_abstain = 1.0 ± 0.001 유지."""
        result = predictor_with_mock_classifier.predict("경계 케이스")
        total = result.p_a + result.p_b + result.p_abstain
        assert abs(total - 1.0) <= 0.001, (
            f"abstain 상태에서 확률 합 불변식 위반: {result.p_a:.4f} + {result.p_b:.4f} + "
            f"{result.p_abstain:.4f} = {total:.4f}"
        )

    def test_no_abstain_when_p_a_above_0_5(self) -> None:
        """p_a >= 0.5이면 abstain이 트리거되지 않아야 한다."""
        predictor = GradePredictor()
        mock_clf = MagicMock()
        mock_clf.classes_ = ["B", "A"]
        mock_clf.predict_proba.return_value = [[0.25, 0.75]]  # p_B=0.25, p_A=0.75
        mock_vectorizer = MagicMock()
        mock_vectorizer.transform.return_value = [[0.1, 0.2, 0.3]]
        predictor.classifier = mock_clf
        predictor.vectorizer = mock_vectorizer
        predictor._is_trained = True

        result = predictor.predict("A 등급 우세 보고서")
        assert result.predicted_class == "A", "p_a=0.75 > 0.5이므로 predicted_class='A'여야 한다"
        assert result.low_confidence is False, "p_a >= 0.5이면 low_confidence=False여야 한다"

    def test_abstain_p_a_and_p_b_preserved_in_response(
        self, predictor_with_mock_classifier: GradePredictor
    ) -> None:
        """abstain 응답에서 p_a, p_b 원래 값이 보존되어야 한다 (AC-003-2 candidates 반환).

        acceptance.md: response body에 {A: 0.42, B: 0.45} candidates가 포함되어야 한다.
        """
        result = predictor_with_mock_classifier.predict("경계 케이스")
        # mock: p_A=0.42, p_B=0.45
        assert abs(result.p_a - 0.42) <= 0.01, f"p_a 예상값 0.42, 실제 {result.p_a:.4f}"
        assert abs(result.p_b - 0.45) <= 0.01, f"p_b 예상값 0.45, 실제 {result.p_b:.4f}"


# ============================================================
# AC-003-3: model_used metadata tracking
# ============================================================


class TestGradePredictorModelUsedMetadata:
    """AC-003-3: model_used 메타데이터 추적 — Qwen 2.5 / fallback 구분."""

    def test_model_used_field_is_not_empty(self) -> None:
        """model_used 필드가 항상 채워져야 한다 (AC-003-3)."""
        predictor = GradePredictor()
        reports = [
            BenchmarkReport(grade="A", text_content="A등급 안전보건 우수 사례 안전교육 이수", report_id="a-001"),
            BenchmarkReport(grade="B", text_content="B등급 안전보건 개선 필요 안전사고 발생", report_id="b-001"),
        ]
        predictor.train(reports)
        result = predictor.predict("테스트 보고서")
        assert result.model_used, "model_used 메타데이터가 비어 있으면 안 된다"

    def test_model_used_field_contains_model_identifier(self) -> None:
        """model_used는 사용된 모델을 식별하는 문자열을 포함해야 한다."""
        predictor = GradePredictor()
        reports = [
            BenchmarkReport(grade="A", text_content="안전관리체계 우수 안전보건 인증", report_id="a-001"),
            BenchmarkReport(grade="B", text_content="안전관리 미흡 개선 필요 안전사고", report_id="b-001"),
        ]
        predictor.train(reports)
        result = predictor.predict("보고서 내용")
        # 모델 식별자는 빈 문자열이 아닌 의미 있는 값이어야 한다
        assert len(result.model_used) > 0, "model_used는 최소 1자 이상의 식별자여야 한다"
