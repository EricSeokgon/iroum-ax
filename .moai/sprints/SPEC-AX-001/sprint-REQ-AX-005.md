# Sprint Contract — REQ-AX-005 Gap Recommendation

- SPEC: SPEC-AX-001 v0.1.2
- Sprint: 6
- 작성일: 2026-05-14
- Harness level: thorough (sprint contract 필수)
- Priority dimension: **Completeness** (최종 사용자 대면 출력 — B→A 3-5개 우선순위 추천)

---

## 1. 수락 기준 체크리스트

| AC | 설명 | 검증 방법 |
|----|------|----------|
| AC-005-1 | B→A 3-5개 prioritized 추천 반환 + feasibility_score 내림차순 + 3초 이내 | `test_req_ax_005_gap_analyzer.py`, `test_req_ax_005_prioritizer.py`, `test_req_ax_005_integration.py` |
| AC-005-2 | 벤치마크 데이터 부족 → `benchmark_not_available` + 빈 리스트 (fabricated 콘텐츠 금지) | `test_req_ax_005_content_suggester.py` — empty retriever mock |
| AC-005-3 | 각 추천 항목이 criterion_id로 REQ-AX-002 기준과 역추적 가능 | `test_req_ax_005_gap_analyzer.py`, `test_req_ax_005_content_suggester.py` — criterion_id 단언 |

---

## 2. 우선순위 차원

### 2.1 Primary: Completeness (40%)

- `GapAnalyzer.analyze()` — GradeDistribution(현재 B) + target_grade("A") + ParsedDocument + benchmarks → list[GapItem] 반환
- `ContentSuggester.suggest()` — list[GapItem] + Retriever → list[ContentSuggestion] 반환, 각 항목 criterion_id 보존
- `Prioritizer.prioritize()` — list[ContentSuggestion] + top_k → list[RankedSuggestion], feasibility × score_delta 내림차순, top_k(3-5) 캡

### 2.2 Secondary: Functionality (30%)

- GapItem 생성 시 score_delta 계산 정확성: current_p_a 대비 A 벤치마크 p_a 차이
- feasibility 점수가 0.0~1.0 범위 내 보장
- criterion_id가 빈 문자열이 아닌 유효한 ID여야 함 (AC-005-3)

### 2.3 Security (20%)

- 벤치마크 없을 때 fabricated 콘텐츠 생성 금지 (AC-005-2)
- Retriever가 비어 있을 때 `benchmark_not_available` 상태 반환 (할루시네이션 방지)

### 2.4 Consistency (10%)

- REQ-AX-002 (Criterion.id)와의 타입 일관성
- REQ-AX-003 (GradeDistribution)과의 인터페이스 일관성
- RankedSuggestion의 priority_score = feasibility × score_delta 수학적 불변식

---

## 3. 테스트 시나리오

### 3.1 GapAnalyzer 테스트 시나리오

| 테스트 | 설명 | 입력 | 기대 출력 |
|-------|------|------|----------|
| GA-1 | 정상 B→A 갭 분석 | B 분포 + A 벤치마크 2개 | list[GapItem] len >= 1, criterion_id 보존 |
| GA-2 | 모든 기준이 이미 A 수준 | 현재 보고서 ≈ 벤치마크 | 빈 list[] 반환 |
| GA-3 | 벤치마크 없음 | 빈 benchmarks 목록 | 빈 list[] 반환 |
| GA-4 | score_delta 양수 보장 | B 분포 | 모든 GapItem.score_delta >= 0.0 |
| GA-5 | criterion_id 역추적 | 2개 기준 + 벤치마크 | 각 GapItem.criterion_id가 입력 기준 ID 중 하나 |

### 3.2 ContentSuggester 테스트 시나리오

| 테스트 | 설명 | 입력 | 기대 출력 |
|-------|------|------|----------|
| CS-1 | 정상 제안 생성 | 2개 GapItem + mock retriever | 2개 ContentSuggestion, criterion_id 보존 |
| CS-2 | 빈 retriever — AC-005-2 | GapItem + 빈 retriever | BenchmarkNotAvailableError 또는 빈 list |
| CS-3 | evidence_refs 채워짐 | retriever 반환 1개 결과 | evidence_refs len >= 1 |
| CS-4 | criterion_id 역추적 | criterion_id="crit-001" GapItem | ContentSuggestion.criterion_id == "crit-001" |

### 3.3 Prioritizer 테스트 시나리오

| 테스트 | 설명 | 입력 | 기대 출력 |
|-------|------|------|----------|
| PR-1 | top_k=5 캡 적용 | 7개 제안 | 5개 RankedSuggestion |
| PR-2 | feasibility × score_delta 정렬 | 혼합 점수 | rank 1이 최고 priority_score |
| PR-3 | top_k보다 적은 입력 | 2개 제안 + top_k=5 | 2개 반환 (가용한 만큼) |
| PR-4 | priority_score 수학 불변식 | feasibility=0.8, score_delta=0.5 | priority_score ≈ 0.4 |
| PR-5 | rank 1-based 연속성 | 3개 제안 | rank 1, 2, 3 순차 |
| PR-6 | 빈 입력 | 빈 list | 빈 list 반환 |

### 3.4 통합 테스트 시나리오

| 테스트 | 설명 | 기대 출력 |
|-------|------|----------|
| INT-1 | B→A 전체 파이프라인 | 3-5개 RankedSuggestion, feasibility_score 내림차순 |
| INT-2 | 벤치마크 없음 파이프라인 | BenchmarkNotAvailableError 또는 빈 list |

---

## 4. 패스 조건 (Sprint 6 GREEN 진입 기준)

- Sprint 6 신규 테스트 모두 PASS
- Sprint 1-5 회귀 없음 (156 passed 유지)
- LSP errors=0, type_errors=0, lint_errors=0
- Coverage >= 85%

---

## 5. REQ 의존성 다이어그램

```
REQ-AX-002 (Criterion, CriterionMatch)
    ↓ criterion_id 역추적 (AC-005-3)
REQ-AX-003 (GradeDistribution, ScenarioResult)
    ↓ B 등급 현재 분포
REQ-AX-005 Gap Recommendation
    GapAnalyzer → ContentSuggester → Prioritizer
    → list[RankedSuggestion] (3-5개, feasibility_score 내림차순)
```
