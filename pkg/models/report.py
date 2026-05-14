"""공유 실적보고서 데이터 모델 (REQ-AX-004)

# @MX:ANCHOR: [AUTO] DraftSection / StyleReport / GenerationResult — 보고서 초안 파이프라인 핵심 모델
# @MX:REASON: ReportDrafter, LLMClient, StyleApplier, PromptBuilder 모두 이 모델을 반환/소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-004 / AC-004-1 / AC-004-2 / AC-004-3
"""
from __future__ import annotations

from enum import Enum
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class Grade(str, Enum):
    """경영평가 등급"""

    A = "A"
    B = "B"
    C = "C"
    D = "D"


class ReportModel(BaseModel):
    """경영평가 실적보고서 공유 모델"""

    id: UUID = Field(description="보고서 고유 ID")
    organization_name: str | None = Field(default=None, description="기관명")
    grade: Grade | None = Field(default=None, description="등급")
    content: str | None = Field(default=None, description="보고서 본문")
    score: float | None = Field(default=None, description="평가 점수")
    source_benchmark_id: str | None = Field(default=None, description="벤치마크 출처 ID")


class GenerationResult(BaseModel):
    """LLM 생성 결과 — LLMClient.generate() 반환 타입

    # @MX:NOTE: [AUTO] model_used 는 AC-004-3 fallback 추적 필수 필드
    # @MX:SPEC: SPEC-AX-001 AC-004-3
    """

    model_config = ConfigDict(extra="forbid")

    text: str = Field(description="생성된 텍스트")
    model_used: str = Field(description="실제 사용된 모델명 (예: qwen2.5-7b)")
    tokens: int = Field(ge=0, description="생성 토큰 수")
    latency_ms: int = Field(ge=0, description="추론 응답 시간 (밀리초)")


class StyleReport(BaseModel):
    """공문 스타일 검증 결과 — StyleApplier.validate() 반환 타입

    # @MX:NOTE: [AUTO] is_official=False이면 ReportDrafter가 재생성 루프 진입
    # @MX:SPEC: SPEC-AX-001 AC-004-2 / AC-004-1
    """

    model_config = ConfigDict(extra="forbid")

    is_official: bool = Field(description="공문 합니다체 적합 여부")
    honorific_score: float = Field(
        ge=0.0, le=1.0, description="합니다체 종결어미 비율 (0.0~1.0)"
    )
    violations: list[str] = Field(
        default_factory=list,
        description="위반 항목 목록 (예: no_korean, casual_ending, empty_text)",
    )


class DraftSection(BaseModel):
    """초안 섹션 — ReportDrafter.draft_section() 반환 타입

    # @MX:NOTE: [AUTO] status='style_violation'이면 최대 재시도 소진 — 사람 검수 필요
    # @MX:SPEC: SPEC-AX-001 AC-004-2
    """

    model_config = ConfigDict(extra="allow")

    text: str = Field(default="", description="생성된 초안 텍스트")
    status: str = Field(
        default="ok",
        description="생성 상태: ok | style_violation",
    )
    model_used: str = Field(description="실제 사용된 모델명")
    retry_count: int = Field(default=0, ge=0, description="스타일 검증 재시도 횟수")
    style_report: StyleReport | None = Field(
        default=None, description="최종 스타일 검증 결과"
    )
