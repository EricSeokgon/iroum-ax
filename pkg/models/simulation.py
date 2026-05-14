"""공유 등급 시뮬레이션 데이터 모델 (REQ-AX-003)

Sprint 4 RED phase: GradeDistribution, ScenarioResult, BenchmarkReport 모델 추가.

# @MX:ANCHOR: [AUTO] GradeDistribution — GradePredictor/ScenarioSimulator 전반에서 소비되는 핵심 출력 모델
# @MX:REASON: GradePredictor.predict, ScenarioSimulator.simulate, API 핸들러, 통합 테스트가 모두 이 모델을 반환/소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-003-E1 / REQ-AX-003-U1 / AC-003-1 / AC-003-2
"""
from __future__ import annotations

from uuid import UUID

from pydantic import BaseModel, Field, field_validator, model_validator


class GradeDistribution(BaseModel):
    """등급 확률 분포 — 3-way 출력 {A, B, abstain}.

    # @MX:NOTE: [AUTO] p_a + p_b + p_abstain = 1.0 ± 0.001 불변식 — 이 모델이 반환될 때마다 검증됨
    # @MX:SPEC: SPEC-AX-001 REQ-AX-003-E1 (2-class softmax + abstain 3-way output)

    수학 불변식:
        sum(p_a, p_b, p_abstain) = 1.0 ± 0.001
        IF max(p_a, p_b) < 0.5: predicted_class = 'abstain' (REQ-AX-003-U1)
        ELSE: predicted_class = 'A' if p_a >= p_b else 'B'
    """

    p_a: float = Field(ge=0.0, le=1.0, description="A 등급 확률")
    p_b: float = Field(ge=0.0, le=1.0, description="B 등급 확률")
    p_abstain: float = Field(ge=0.0, le=1.0, description="abstain 확률 = 1 - p_a - p_b")
    predicted_class: str = Field(description="예측 등급: 'A', 'B', 또는 'abstain'")
    model_used: str = Field(default="sklearn_lr", description="사용된 모델 식별자 (AC-003-3 metadata)")
    low_confidence: bool = Field(
        default=False,
        description="max(p_a, p_b) < 0.5 이면 True (REQ-AX-003-U1 트리거)",
    )

    @field_validator("predicted_class")
    @classmethod
    def validate_predicted_class(cls, v: str) -> str:
        """predicted_class는 'A', 'B', 'abstain' 중 하나여야 한다."""
        if v not in ("A", "B", "abstain"):
            raise ValueError(f"predicted_class는 'A', 'B', 'abstain' 중 하나여야 합니다. 입력값: {v!r}")
        return v

    @model_validator(mode="after")
    def validate_probability_sum(self) -> "GradeDistribution":
        """확률 합 불변식: p_a + p_b + p_abstain = 1.0 ± 0.001 (REQ-AX-003-E1)"""
        total = self.p_a + self.p_b + self.p_abstain
        if abs(total - 1.0) > 0.001:
            raise ValueError(
                f"확률 합 불변식 위반: p_a({self.p_a}) + p_b({self.p_b}) + p_abstain({self.p_abstain}) = {total:.4f} ≠ 1.0 ± 0.001"
            )
        return self

    @model_validator(mode="after")
    def validate_abstain_consistency(self) -> "GradeDistribution":
        """abstain 분기 일관성:
        - max(p_a, p_b) < 0.5 이면 predicted_class는 'abstain'이어야 함
        - max(p_a, p_b) >= 0.5 이면 predicted_class는 'A' 또는 'B'여야 함
        """
        if max(self.p_a, self.p_b) < 0.5:
            if self.predicted_class != "abstain":
                raise ValueError(
                    f"abstain 불변식 위반: max(p_a={self.p_a}, p_b={self.p_b}) < 0.5이므로 "
                    f"predicted_class는 'abstain'이어야 하나 '{self.predicted_class}'임"
                )
            if not self.low_confidence:
                raise ValueError("abstain 상태에서 low_confidence는 True여야 합니다")
        return self


class BenchmarkReport(BaseModel):
    """A/B 등급 벤치마크 보고서 학습 입력 — BenchmarkLearner.learn() 입력 타입.

    # @MX:NOTE: [AUTO] grade는 'A' 또는 'B'만 허용 (Exclusion §11: C/D 등급 제외)
    # @MX:SPEC: SPEC-AX-001 §5 Exclusion 11 — A vs B 2-class only
    """

    grade: str = Field(description="벤치마크 등급: 'A' 또는 'B'")
    text_content: str = Field(description="보고서 텍스트 내용 (TF-IDF 특징 추출용)")
    report_id: str = Field(default="", description="보고서 식별자 (감사 로그용)")

    @field_validator("grade")
    @classmethod
    def validate_grade(cls, v: str) -> str:
        """grade는 'A' 또는 'B'만 허용 (Exclusion §11)"""
        if v not in ("A", "B"):
            raise ValueError(f"grade는 'A' 또는 'B'여야 합니다. 입력값: {v!r}")
        return v


class ScenarioResult(BaseModel):
    """B→A 시나리오 시뮬레이션 결과 — ScenarioSimulator.simulate() 반환 타입.

    # @MX:NOTE: [AUTO] content_changes는 현재 등급을 target_grade로 올리기 위한 콘텐츠 변경 제안
    # @MX:SPEC: SPEC-AX-001 REQ-AX-005-E1 (Gap Recommendation에서 소비됨)
    """

    current_grade: str = Field(description="현재 예측 등급")
    target_grade: str = Field(description="목표 등급")
    current_p_a: float = Field(ge=0.0, le=1.0, description="현재 A 등급 확률")
    projected_p_a: float = Field(ge=0.0, le=1.0, description="시나리오 적용 후 A 등급 확률")
    content_changes: list[str] = Field(
        default_factory=list,
        description="B→A 달성을 위한 콘텐츠 변경 제안 (최소 1개)",
    )
    feasible: bool = Field(
        default=True,
        description="p_a를 0.5 이상으로 올릴 가능성이 있는지 여부",
    )


class SimulationModel(BaseModel):
    """등급 시뮬레이션 결과 공유 모델 (DB 저장용)"""

    id: UUID = Field(description="시뮬레이션 고유 ID")
    workflow_id: UUID = Field(description="연결된 워크플로우 ID")
    abstain_flag: bool = Field(
        default=False,
        description="기권 플래그: probability_a < 0.5 AND probability_b < 0.5 (REQ-AX-003-E1)",
    )
    # Sprint 4 GREEN phase에서 추가 필드 확정
    # current_grade, target_grade, probability_a/b, prediction 필드 추가 예정
