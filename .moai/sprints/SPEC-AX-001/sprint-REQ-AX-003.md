# Sprint Contract: REQ-AX-003 — Grade Simulation

- Sprint: 4
- REQ: REQ-AX-003
- Harness: thorough
- Priority Dimension: **Functionality** (확률 수학 불변식 + abstain 분기 정확성)
- Highest-Risk Dimension: Functionality — abstain 로직 오류 시 수학적 모순 발생 (evaluator-active iteration 3 NEW-F1 발견)
- 작성일: 2026-05-14
- 작성자: manager-tdd (Sprint 4 RED phase)

---

## Acceptance Checklist

| # | AC | 검증 항목 | 통과 조건 |
|---|----|---------|---------|
| 1 | AC-003-1 | {A, B, abstain} 3-way 분포 산출 + sum=1.0±0.001 + 1초 응답 + simulations 저장 | `GradeDistribution.p_a + p_b + p_abstain` 오차 < 0.001 |
| 2 | AC-003-2 | max(p_a, p_b) < 0.5 → status=low_confidence, prediction=null, candidates 반환, downstream 차단 | `{A: 0.42, B: 0.45, abstain: 0.13}` 시나리오에서 `predicted_class='abstain'` |
| 3 | AC-003-3 | 학습 중 HTTP 503 model_training (stale prediction 금지) | BenchmarkLearner state=training 시 예측 거부 |

---

## Priority Dimension: Functionality

Sprint 4의 핵심 위험은 **abstain 분기의 수학적 정확성**이다.

- REQ-AX-003-E1: 2-class softmax 출력(p_a, p_b)에 abstain 확률(p_abstain = 1 - p_a - p_b)을 추가하여 3-way 분포를 만들어야 한다
- REQ-AX-003-U1: `max(p_a, p_b) < 0.5` 조건이 abstain 트리거이며, 이 조건 미충족 시 일반 분류 경로를 따른다
- 수학 불변식: `p_a + p_b + p_abstain = 1.0 ± 0.001` (100건 배치 기준 위반 0건)

**AC-003-2의 핵심 시나리오**: `{A: 0.42, B: 0.45}` — 두 확률 모두 0.5 미만이므로 abstain 트리거. `p_abstain = 1 - 0.42 - 0.45 = 0.13`. sum = 0.42 + 0.45 + 0.13 = 1.00.

---

## Test Scenarios (API-Level, httpx + FastAPI TestClient)

### Scenario 1: 정상 예측 (A 등급 우세)

```
Given: A벤치마크 보고서 특징 (TF-IDF 높은 안전보건 점수 패턴)
When: GradePredictor.predict(report)
Then: p_a > 0.5, predicted_class='A', sum=1.0±0.001
```

### Scenario 2: 정상 예측 (B 등급 우세)

```
Given: B벤치마크 보고서 특징
When: GradePredictor.predict(report)
Then: p_b > 0.5, predicted_class='B', sum=1.0±0.001
```

### Scenario 3: Abstain 트리거 (핵심 시나리오)

```
Given: 경계 케이스 보고서 (A 0.42, B 0.45)
When: GradePredictor.predict(report)
Then: predicted_class='abstain', p_abstain=0.13±0.001, recommend_human_review=True
```

### Scenario 4: 학습 중 예측 차단

```
Given: BenchmarkLearner state=training
When: GradePredictor.predict() 호출
Then: ModelTrainingError 발생 (HTTP 503 equivalent)
```

### Scenario 5: B→A 시나리오 시뮬레이션

```
Given: 현재 B 등급 보고서, 목표 A 등급
When: ScenarioSimulator.simulate(current_report, target_grade='A')
Then: ScenarioResult에 content_changes 포함, p_a_projected > p_a_current
```

---

## Pass Conditions

- 3 AC 시나리오 자동화 테스트 전부 통과
- 확률 sum 검증: 100건 배치에서 위반 0건
- 등급 예측 일치율 기준 (사람 검수 대비 ≥ 80%): GREEN phase에서 통합 테스트로 검증
- LSP errors=0, type_errors=0, lint_errors=0

---

## Evaluator 4-dim Weights

| 차원 | 가중치 |
|------|------|
| Functionality | 50% |
| Completeness | 25% |
| Security | 15% |
| Originality | 10% |

---

## Sprint Contract Artifact

- 저장 위치: `.moai/sprints/SPEC-AX-001/sprint-REQ-AX-003.md`
- 다음 단계: Sprint 4 GREEN phase (scikit-learn LogisticRegression + abstain 분기 구현)
