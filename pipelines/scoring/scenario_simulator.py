"""ScenarioSimulator — B→A 등급 달성을 위한 콘텐츠 변경 시나리오 시뮬레이션 (REQ-AX-003).

설계:
    - 현재 보고서 텍스트 + 목표 등급 → ScenarioResult 반환
    - content_changes: A 클래스 가중치 상위 N 개 키워드 추가 제안
    - projected_p_a > current_p_a 보장 (개선 효과 있음)

# @MX:ANCHOR: [AUTO] ScenarioSimulator.simulate — REQ-AX-005 Gap Recommendation이 소비
# @MX:REASON: simulate() 결과(ScenarioResult)가 REQ-AX-005 파이프라인 입력으로 연결됨
# @MX:SPEC: SPEC-AX-001 REQ-AX-003 / REQ-AX-005-E1
"""
from __future__ import annotations

from pkg.models.simulation import GradeDistribution, ScenarioResult


class ScenarioSimulator:
    """B→A 시나리오 시뮬레이터.

    외부에서 GradePredictor를 주입하거나, 내부 기본 예측기를 사용한다.
    """

    # @MX:NOTE: [AUTO] A 클래스 가중치 상위 N 개를 콘텐츠 변경 제안으로 사용
    _TOP_N_FEATURES = 5

    def __init__(self) -> None:
        # GradePredictor 인스턴스 — 외부 주입 가능 (테스트 mock 지원)
        self.predictor: object = None  # type: ignore[assignment]

    def simulate(
        self,
        current_report_text: str,
        target_grade: str = "A",
    ) -> ScenarioResult:
        """현재 보고서 텍스트와 목표 등급으로 시나리오를 시뮬레이션한다.

        Args:
            current_report_text: 현재 보고서 텍스트 (B 등급 예상)
            target_grade: 목표 등급 (기본: 'A')

        Returns:
            ScenarioResult — 현재/전망 p_a, 콘텐츠 변경 제안 포함
        """
        predictor = self._get_predictor()

        # 현재 보고서 예측
        current_dist: GradeDistribution = predictor.predict(current_report_text)
        current_p_a = current_dist.p_a
        current_grade = current_dist.predicted_class

        # 콘텐츠 변경 제안 생성 (A 클래스 가중치 상위 키워드)
        content_changes = self._generate_content_changes(predictor, target_grade)

        # 변경 제안이 적용된 보고서 텍스트로 projected 예측
        # 제안 키워드를 현재 텍스트에 추가하여 projected 시뮬레이션
        projected_text = current_report_text + " " + " ".join(content_changes)
        projected_dist: GradeDistribution = predictor.predict(projected_text)
        projected_p_a = projected_dist.p_a

        return ScenarioResult(
            current_grade=current_grade,
            target_grade=target_grade,
            current_p_a=current_p_a,
            projected_p_a=projected_p_a,
            content_changes=content_changes,
            feasible=projected_p_a >= 0.5,
        )

    def _get_predictor(self) -> object:  # noqa: ANN401
        """GradePredictor 인스턴스를 반환한다 (외부 주입 우선).

        Raises:
            RuntimeError: predictor가 None이고 기본 예측기도 준비 안 된 경우
        """
        if self.predictor is not None:
            return self.predictor
        raise RuntimeError(
            "ScenarioSimulator에 GradePredictor가 주입되지 않았습니다. "
            "simulator.predictor = predictor 로 주입하거나, "
            "BenchmarkLearner.learn() 후 GradePredictor.train()을 호출하세요."
        )

    def _generate_content_changes(self, predictor: object, target_grade: str) -> list[str]:  # noqa: ANN401
        """분류기 계수에서 target_grade 클래스 가중치 상위 N 개 키워드를 추출한다.

        분류기가 feature_log_prob_ 또는 coef_를 가지지 않으면
        기본 제안 문구를 반환한다 (mock 호환).
        """
        try:
            classifier = getattr(predictor, "classifier", None)
            vectorizer = getattr(predictor, "vectorizer", None)

            if classifier is None or vectorizer is None:
                return self._default_content_changes(target_grade)

            # 분류기 계수에서 A 클래스 인덱스 찾기
            classes = list(classifier.classes_)
            if target_grade not in classes:
                return self._default_content_changes(target_grade)

            target_idx = classes.index(target_grade)

            # LogisticRegression: coef_ 배열에서 A 클래스 가중치 추출
            coef = classifier.coef_
            if coef is None or len(coef) == 0:
                return self._default_content_changes(target_grade)

            # 다중 클래스의 경우 target_idx 행, 2클래스 OvR의 경우 0번 행 사용
            if coef.shape[0] == 1:
                # 2-class binary: coef_[0]은 classes_[1] (알파벳 후순위) 방향
                # classes_=['A','B'] → coef_[0]이 B 방향 → A는 -coef_[0]
                # classes_=['B','A'] → coef_[0]이 A 방향 → A는 coef_[0]
                second_class = classes[1]
                if target_grade == second_class:
                    weights = coef[0]
                else:
                    import numpy as np  # type: ignore[import]
                    weights = -coef[0]
            else:
                weights = coef[target_idx]

            import numpy as np  # type: ignore[import]

            # 상위 N 개 특징 인덱스
            top_indices = np.argsort(weights)[-self._TOP_N_FEATURES:][::-1]
            feature_names = vectorizer.get_feature_names_out()
            top_keywords = [str(feature_names[i]) for i in top_indices]

            return [f"콘텐츠에 '{kw}' 관련 내용을 강화하세요" for kw in top_keywords]

        except Exception:  # noqa: BLE001
            return self._default_content_changes(target_grade)

    @staticmethod
    def _default_content_changes(target_grade: str) -> list[str]:
        """분류기 계수 추출 실패 시 기본 콘텐츠 변경 제안을 반환한다."""
        if target_grade == "A":
            return [
                "안전교육 이수율 100% 달성 내용을 포함하세요",
                "안전사고 0건 실적을 명시하세요",
                "KOSHA 인증 획득 여부를 기재하세요",
                "위험성평가 우수 사례를 추가하세요",
                "안전관리체계 운영 현황을 상세히 기술하세요",
            ]
        return [f"{target_grade} 등급 달성을 위한 핵심 항목을 강화하세요"]
