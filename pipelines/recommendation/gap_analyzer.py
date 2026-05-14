"""갭 분석기 — B→A 등급 갭 항목 산출 (REQ-AX-005)

# @MX:ANCHOR: [AUTO] GapAnalyzer.analyze — GapItem 파이프라인의 진입점
# @MX:REASON: ContentSuggester, Prioritizer, 통합 테스트가 analyze() 반환 값을 소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-005 / AC-005-1 / AC-005-2 / AC-005-3
"""
from __future__ import annotations

from pkg.models.criterion import Criterion
from pkg.models.document import ParsedDocument
from pkg.models.recommendation import GapItem
from pkg.models.simulation import BenchmarkReport, GradeDistribution

# A 등급 기준 p_a 임계값 — 이 이상이면 이미 A 수준으로 판단
_A_GRADE_THRESHOLD: float = 0.70

# 기준당 기본 feasibility — 단순 키워드 비교 기반 고정값
_DEFAULT_FEASIBILITY: float = 0.70


class GapAnalyzer:
    """현재 보고서와 A 등급 벤치마크 간 콘텐츠 갭을 분석한다.

    벤치마크 데이터 없이는 갭을 생성하지 않는다 (AC-005-2: fabricated 갭 금지).
    각 GapItem은 criterion_id를 통해 REQ-AX-002 Criterion으로 역추적 가능하다 (AC-005-3).
    """

    def analyze(
        self,
        current: GradeDistribution,
        target_grade: str,
        current_report: ParsedDocument,
        benchmarks: list[BenchmarkReport],
        criteria: list[Criterion],
    ) -> list[GapItem]:
        """현재 등급 분포와 A 등급 벤치마크를 비교해 GapItem 목록을 반환한다.

        Args:
            current: 현재 보고서의 등급 확률 분포
            target_grade: 목표 등급 ('A')
            current_report: 현재 보고서 파싱 결과
            benchmarks: 대상 등급(A) 벤치마크 보고서 목록
            criteria: 평가기준 목록 (criterion_id 역추적용)

        Returns:
            GapItem 목록. 벤치마크 없음 또는 기준 없음 시 빈 리스트.

        # @MX:NOTE: [AUTO] AC-005-2: benchmarks가 빈 리스트이면 반드시 [] 반환 — fabricated 갭 금지
        """
        # AC-005-2: 벤치마크 없으면 빈 리스트 반환
        if not benchmarks:
            return []

        # 평가기준 없으면 갭 분석 불가
        if not criteria:
            return []

        # 현재 보고서 텍스트 토큰 집합 (단순 키워드 비교)
        current_tokens = set(_tokenize(current_report.text))

        # 벤치마크 텍스트 합산 토큰 집합 (A 등급 기준 집합)
        benchmark_tokens: set[str] = set()
        for bm in benchmarks:
            benchmark_tokens.update(_tokenize(bm.text_content))

        # A 등급 p_a — 현재 p_a와의 차이를 score_delta 기준으로 사용
        target_p_a = _A_GRADE_THRESHOLD

        # 현재 이미 A 등급 수준인지 확인
        already_a = current.p_a >= target_p_a

        gaps: list[GapItem] = []

        for criterion in criteria:
            # 기준 키워드를 현재/벤치마크 토큰과 비교
            crit_text = criterion.criterion_name + " " + criterion.criterion_detail
            crit_tokens = set(_tokenize(crit_text))

            # 현재 보고서에서 기준 관련 키워드 커버리지
            current_coverage = _coverage(crit_tokens, current_tokens)
            # 벤치마크에서 기준 관련 키워드 커버리지
            benchmark_coverage = _coverage(crit_tokens, benchmark_tokens)

            # 갭 크기 = 벤치마크 커버리지 - 현재 커버리지
            gap_size = max(0.0, benchmark_coverage - current_coverage)

            # p_a 델타를 갭 크기와 결합해 score_delta 산출
            p_a_delta = max(0.0, target_p_a - current.p_a)
            score_delta = min(1.0, gap_size * 0.5 + p_a_delta * 0.5)

            # score_delta가 너무 작으면 (< 0.05) 갭 없음으로 처리
            if score_delta < 0.05:
                continue

            # 이미 A 등급이면 갭 없음
            if already_a and gap_size < 0.1:
                continue

            # feasibility: 현재 커버리지가 높을수록 실현 가능성 높음
            feasibility = min(1.0, max(0.0, 0.4 + current_coverage * 0.6))

            gaps.append(
                GapItem(
                    criterion_id=criterion.id,
                    current_state=(
                        f"{criterion.criterion_name} 현재 수준: {current_coverage:.0%}"
                    ),
                    target_state=(
                        f"{criterion.criterion_name} 목표 수준:"
                        f" {benchmark_coverage:.0%} (A 등급 기준)"
                    ),
                    score_delta=round(min(1.0, max(0.0, score_delta)), 4),
                    feasibility=round(feasibility, 4),
                )
            )

        return gaps


def _tokenize(text: str) -> list[str]:
    """텍스트를 2자 이상 토큰으로 분리한다 (한국어 어절 기반 단순 분리).

    # @MX:NOTE: [AUTO] 형태소 분석기 미사용 — 의존성 없이 단순 공백 분리
    """
    tokens = []
    for word in text.split():
        # 특수문자 제거 후 2자 이상 토큰만 보존
        cleaned = "".join(ch for ch in word if ch.isalnum() or ord(ch) > 127)
        if len(cleaned) >= 2:  # noqa: PLR2004
            tokens.append(cleaned)
    return tokens


def _coverage(criterion_tokens: set[str], text_tokens: set[str]) -> float:
    """기준 키워드 중 텍스트에서 발견된 비율을 반환한다.

    기준 키워드가 없으면 0.0을 반환한다.
    """
    if not criterion_tokens:
        return 0.0
    matched = sum(
        1
        for ct in criterion_tokens
        if any(ct in tt or tt in ct for tt in text_tokens)
    )
    return matched / len(criterion_tokens)
