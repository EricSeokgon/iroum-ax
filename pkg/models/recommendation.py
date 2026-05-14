"""추천 시스템 데이터 모델 (REQ-AX-005)

Sprint 6 RED phase: GapItem, ContentSuggestion, RankedSuggestion 모델 스텁.
GREEN phase에서 완전한 검증 로직이 추가된다.

# @MX:ANCHOR: [AUTO] GapItem — Gap 파이프라인 핵심 모델
# @MX:REASON: analyze/suggest/prioritize 및 통합 테스트 모두 이 모델을 소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-005-E1 / AC-005-1 / AC-005-2 / AC-005-3
"""
from __future__ import annotations

from pydantic import BaseModel, Field


class GapItem(BaseModel):
    """현재 등급과 목표 등급 간의 콘텐츠 차이 항목.

    # @MX:NOTE: [AUTO] criterion_id는 REQ-AX-002 Criterion과의 역추적(traceability) 고리 — AC-005-3
    # @MX:SPEC: SPEC-AX-001 AC-005-3 (각 항목은 criterion_id로 평가기준과 연결됨)
    """

    criterion_id: str = Field(description="연관 평가기준 ID (REQ-AX-002 Criterion.id)")
    current_state: str = Field(description="현재 보고서 해당 기준의 상태 설명")
    target_state: str = Field(description="A 등급 달성을 위한 목표 상태 설명")
    score_delta: float = Field(
        ge=0.0,
        le=1.0,
        description="현재→목표 등급 확률 차이 (0.0~1.0, 클수록 개선 효과 큼)",
    )
    feasibility: float = Field(
        ge=0.0,
        le=1.0,
        description="실현 가능성 점수 (0.0~1.0, 클수록 실현 쉬움)",
    )


class ContentSuggestion(BaseModel):
    """벤치마크 기반 콘텐츠 제안 항목.

    # @MX:NOTE: [AUTO] criterion_id로 역추적 가능 — AC-005-3 traceability 요구사항
    # @MX:SPEC: SPEC-AX-001 AC-005-3
    """

    criterion_id: str = Field(description="연관 평가기준 ID (GapItem.criterion_id와 동일)")
    suggested_content: str = Field(description="A 등급 벤치마크 기반 제안 콘텐츠")
    evidence_refs: list[str] = Field(
        default_factory=list,
        description="근거 벤치마크 보고서 ID 목록 (source_benchmark_id)",
    )


class RankedSuggestion(BaseModel):
    """우선순위가 지정된 추천 항목 — Prioritizer.prioritize() 반환 타입.

    # @MX:NOTE: [AUTO] rank는 1-based (1이 최우선), priority_score = feasibility × score_delta
    # @MX:SPEC: SPEC-AX-001 AC-005-1 (feasibility_score 내림차순 정렬)
    """

    rank: int = Field(ge=1, description="우선순위 순위 (1이 가장 높음)")
    suggestion: ContentSuggestion = Field(description="콘텐츠 제안 상세")
    gap: GapItem = Field(description="연결된 갭 항목 (역추적용)")
    priority_score: float = Field(
        ge=0.0,
        description="우선순위 점수 = feasibility × score_delta",
    )
    # AC-005-1: expected_score_delta, feasibility_score, source_benchmark_id 구조 요구
    expected_score_delta: float = Field(
        ge=0.0,
        le=1.0,
        description="예상 점수 개선폭 (gap.score_delta 미러)",
    )
    feasibility_score: float = Field(
        ge=0.0,
        le=1.0,
        description="실현 가능성 점수 (gap.feasibility 미러)",
    )
    source_benchmark_id: str = Field(
        default="",
        description="주요 근거 벤치마크 ID (evidence_refs[0])",
    )
