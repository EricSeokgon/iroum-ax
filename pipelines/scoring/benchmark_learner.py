"""BenchmarkLearner — 벤치마크 보고서로 A/B 분류기를 학습 (REQ-AX-003 AC-003-1, AC-003-3).

# @MX:ANCHOR: [AUTO] BenchmarkLearner.learn — GradePredictor 초기화 전 반드시 호출되는 진입점
# @MX:REASON: GradePredictor, 통합 테스트, API 핸들러가 모두 learn() 결과를 소비함 (fan_in >= 3)
# @MX:SPEC: SPEC-AX-001 REQ-AX-003 / AC-003-1 / AC-003-3
"""
from __future__ import annotations

from typing import Any

from pkg.models.simulation import BenchmarkReport


class ModelTrainingError(Exception):
    """학습 중 상태에서 예측 시도 시 발생하는 에러 (AC-003-3)."""


class BenchmarkLearner:
    """TF-IDF + LogisticRegression 기반 A/B 벤치마크 분류기 학습기.

    상태 전이: idle → training → ready
    """

    # @MX:NOTE: [AUTO] 상태 상수 — 'idle', 'training', 'ready' 세 가지만 허용
    _VALID_STATES = ("idle", "training", "ready")

    def __init__(self) -> None:
        self.state: str = "idle"
        # TF-IDF 벡터라이저 — learn() 호출 후 채워짐
        self.vectorizer: Any = None
        # 학습된 분류기 — learn() 호출 후 채워짐
        self._classifier: Any = None

    def learn(self, reports: list[BenchmarkReport]) -> object:  # noqa: ANN401
        """벤치마크 보고서 목록으로 TF-IDF+LR 분류기를 학습하고 반환한다.

        Args:
            reports: BenchmarkReport 목록 (A 등급 1개 + B 등급 1개 이상 필요)

        Returns:
            학습된 분류기 (predict_proba 지원)

        Raises:
            ValueError: 보고서가 없거나 A/B 양쪽이 없을 때
            RuntimeError: 학습 실패 시
        """
        # 빈 목록 검증
        if not reports:
            raise ValueError("학습 보고서가 비어 있습니다. 최소 A 등급 1개 + B 등급 1개 필요.")

        # A/B 등급 분포 검증
        grades = {r.grade for r in reports}
        if "A" not in grades or "B" not in grades:
            raise ValueError(
                f"A 등급과 B 등급 보고서가 각 1개 이상 필요합니다. "
                f"현재 등급 목록: {sorted(grades)}"
            )

        # 학습 시작 — 상태 전이
        self.state = "training"

        # sklearn lazy import — 모듈 로드 시점에 강제 설치 불필요
        from sklearn.feature_extraction.text import TfidfVectorizer  # type: ignore[import]
        from sklearn.linear_model import LogisticRegression  # type: ignore[import]

        texts = [r.text_content for r in reports]
        labels = [r.grade for r in reports]

        # TF-IDF 특징 추출 (한국어 공백 분리 기반)
        vectorizer = TfidfVectorizer(
            analyzer="word",
            token_pattern=r"[^\s]+",  # 공백 기준 분리 (한국어 호환)
            max_features=5000,
        )
        x_train = vectorizer.fit_transform(texts)

        # 로지스틱 회귀 학습
        clf = LogisticRegression(
            C=1.0,
            max_iter=1000,
            random_state=42,
            solver="lbfgs",
        )
        clf.fit(x_train, labels)

        # 상태 저장
        self.vectorizer = vectorizer
        self._classifier = clf
        self.state = "ready"

        return clf

    def predict_grade(self, text: str) -> str:
        """학습된 모델로 등급을 예측한다.

        Raises:
            ModelTrainingError: state=='training' 상태에서 호출 시 (AC-003-3)
        """
        if self.state == "training":
            raise ModelTrainingError(
                "BenchmarkLearner가 학습 중(training)입니다. 학습 완료 후 예측 가능합니다."
            )
        if self.state == "idle" or self._classifier is None:
            raise RuntimeError("먼저 learn()을 호출하여 모델을 학습해야 합니다.")

        x_vec = self.vectorizer.transform([text])
        return str(self._classifier.predict(x_vec)[0])
