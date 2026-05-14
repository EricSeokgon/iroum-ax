"""GradePredictor — A/B 등급 확률 분포 예측 + abstain 3-way 출력 (REQ-AX-003).

핵심 설계:
    - 2-class softmax → p_a, p_b 산출
    - abstain 트리거: max(p_a, p_b) < 0.5 → predicted_class = 'abstain'
    - p_abstain = 1 - p_a - p_b (항등식, 합 불변식 보장)
    - model_used 메타데이터 항상 포함 (AC-003-3)

# @MX:ANCHOR: [AUTO] GradePredictor.predict — API, 통합 테스트, ScenarioSimulator가 모두 소비
# @MX:REASON: predict()는 파이프라인 핵심 출력이며 fan_in >= 3 (AC-003-1 primary)
# @MX:SPEC: SPEC-AX-001 REQ-AX-003-E1 / REQ-AX-003-U1 / AC-003-1 / AC-003-2 / AC-003-3
"""
from __future__ import annotations

from typing import Any

from pkg.models.simulation import (
    BenchmarkReport,
    GradeDistribution,
)

# @MX:NOTE: [AUTO] ABSTAIN_THRESHOLD — max(p_a, p_b) < 0.5 이면 abstain 트리거 (REQ-AX-003-U1)
_ABSTAIN_THRESHOLD = 0.5


class GradePredictor:
    """TF-IDF + LogisticRegression 기반 A/B 등급 확률 예측기.

    Usage:
        predictor = GradePredictor()
        predictor.train(reports)
        result = predictor.predict("보고서 텍스트")
    """

    # @MX:NOTE: [AUTO] MODEL_IDENTIFIER — AC-003-3 model_used 메타데이터 식별자
    _MODEL_IDENTIFIER = "sklearn-tfidf-lr-v1"

    def __init__(self) -> None:
        self.classifier: Any = None
        self.vectorizer: Any = None
        self._is_trained: bool = False

    def train(self, reports: list[BenchmarkReport]) -> None:
        """벤치마크 보고서로 내부 분류기를 학습한다.

        Args:
            reports: BenchmarkReport 목록 (A, B 양쪽 필요)
        """
        from sklearn.feature_extraction.text import TfidfVectorizer  # type: ignore[import]
        from sklearn.linear_model import LogisticRegression  # type: ignore[import]

        texts = [r.text_content for r in reports]
        labels = [r.grade for r in reports]

        vectorizer = TfidfVectorizer(
            analyzer="word",
            token_pattern=r"[^\s]+",
            max_features=5000,
        )
        x_train = vectorizer.fit_transform(texts)

        clf = LogisticRegression(C=1.0, max_iter=1000, random_state=42, solver="lbfgs")
        clf.fit(x_train, labels)

        self.vectorizer = vectorizer
        self.classifier = clf
        self._is_trained = True

    def predict(self, report_text: str) -> GradeDistribution:
        """보고서 텍스트에서 등급 확률 분포를 예측한다.

        확률 합 불변식: p_a + p_b + p_abstain = 1.0 ± 0.001 (REQ-AX-003-E1)
        abstain 트리거: max(p_a, p_b) < 0.5 → predicted_class = 'abstain' (REQ-AX-003-U1)

        Returns:
            GradeDistribution — 3-way 출력 {A, B, abstain}
        """
        if not self._is_trained or self.classifier is None or self.vectorizer is None:
            raise RuntimeError("먼저 train()을 호출하여 모델을 학습해야 합니다.")

        # 특징 벡터 추출
        x_vec = self.vectorizer.transform([report_text])

        # sklearn classifier: classes_ = ['A', 'B'] 또는 ['B', 'A'] (알파벳 순 정렬)
        # predict_proba 결과는 classes_ 순서에 대응
        proba = self.classifier.predict_proba(x_vec)[0]
        classes = list(self.classifier.classes_)

        # classes_ 순서에 따라 p_a, p_b 추출
        a_idx = classes.index("A") if "A" in classes else -1
        b_idx = classes.index("B") if "B" in classes else -1

        p_a_raw = float(proba[a_idx]) if a_idx >= 0 else 0.0
        p_b_raw = float(proba[b_idx]) if b_idx >= 0 else 0.0

        # 2-class softmax 결과이므로 p_a_raw + p_b_raw = 1.0 보장됨
        # abstain 확률: p_abstain = 1 - p_a - p_b (항등식)
        # abstain 미발동 시: p_a + p_b = 1.0 → p_abstain = 0.0
        # abstain 발동 시: p_a, p_b 그대로 유지, p_abstain = 1 - p_a - p_b 계산
        return self._build_distribution(p_a_raw, p_b_raw)

    def _build_distribution(self, p_a: float, p_b: float) -> GradeDistribution:
        """p_a, p_b 원시 확률로 GradeDistribution을 구성한다.

        abstain 트리거 판단 → predicted_class, low_confidence, p_abstain 결정
        확률 합 불변식은 GradeDistribution 모델 검증자가 최종 확인함.
        """
        # abstain 트리거 판단: max(p_a, p_b) < 0.5
        if max(p_a, p_b) < _ABSTAIN_THRESHOLD:
            # abstain 분기: p_a, p_b 원래 값 보존, p_abstain 보충
            p_abstain = 1.0 - p_a - p_b
            # 음수 방지 (부동소수점 오차)
            p_abstain = max(0.0, p_abstain)
            predicted_class = "abstain"
            low_confidence = True
        else:
            # 정상 분기: 2-class softmax이므로 p_abstain = 0.0
            p_abstain = 0.0
            # p_a + p_b가 정확히 1.0이 아닐 수 있으므로 정규화
            total_raw = p_a + p_b
            if total_raw > 0:
                p_a = p_a / total_raw
                p_b = p_b / total_raw
            p_abstain = 1.0 - p_a - p_b
            # 부동소수점 오차 보정
            p_abstain = max(0.0, p_abstain)
            predicted_class = "A" if p_a >= p_b else "B"
            low_confidence = False

        return GradeDistribution(
            p_a=round(p_a, 8),
            p_b=round(p_b, 8),
            p_abstain=round(p_abstain, 8),
            predicted_class=predicted_class,
            model_used=self._MODEL_IDENTIFIER,
            low_confidence=low_confidence,
        )
