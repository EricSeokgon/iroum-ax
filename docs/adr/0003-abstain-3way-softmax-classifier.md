# ADR 0003: 2-class softmax + abstain 3-way 출력

## Status

Accepted (2026-05-14)

## Context

SPEC-AX-001 Plan phase에서 evaluator-active가 2-class 접근의 수학적 결함을 식별:

**문제**: 2-class softmax에서 P(A) + P(B) = 1.0 (정의상 항상)
- 따라서 "둘 다 < 0.5" 상황이 수학적으로 불가능
- 예: P(A)=0.3이면 자동으로 P(B)=0.7
- Threshold 기반 Abstain 메커니즘 구현 불가

**REQ-AX-003 분석 요구사항**:
- 등급 분류: A vs B (2-class, 벤치마크 데이터로만 학습 가능)
- Confidence 표현: 낮은 신뢰도 시 abstain (보류) 필요

## Decision

**3-way softmax {A, B, abstain}** 구현 (sum=1.0±0.001).

근거:
- 수학적 명확성: 3개 class의 합 = 1.0, 각 확률 0.0-1.0
- "불확실함" 표현 가능: P(abstain)이 0.3 이상이면 신뢰도 낮음 표현
- 이해 용이: 사용자 입장에서 "A: 35%, B: 42%, 보류: 23%" 직관적
- 구현 간단: scikit-learn MulticlassClassifier 또는 softmax 3D

## Consequences

### 긍정적 영향

- **신뢰도 정량화**: P(abstain)으로 모델 확신도 수치화
- **AC-AX-003-1 통과**: "2-class vs abstain 분류" acceptance criterion 충족
- **사용자 신뢰**: "보류" 상태 명시로 과도한 추천 회피

### 부정적 영향

- **데이터 요구**: A/B 등급 벤치마크 외에 "분류 불확실" 샘플 학습 필요
  - 현재: A 3개 + B 3개 벤치마크
  - 향후: abstain threshold 튜닝 필요
- **인터페이스 확장**: GradeDistribution 모델에 `p_abstain` 필드 추가 필수

## Implementation

### 모델 구조

```python
class GradePredictor:
    def predict_probabilities(self, features: np.ndarray) -> Dict[str, float]:
        # softmax(logits) → 3-way distribution
        logits = self.model.forward(features)  # shape: (3,)
        probs = softmax(logits)  # [P(A), P(B), P(abstain)]
        
        return {
            "A": float(probs[0]),
            "B": float(probs[1]),
            "abstain": float(probs[2])
        }
    
    def classify(self, probs: Dict) -> str:
        max_class = max(probs, key=probs.get)
        if max_class == "abstain":
            return "ABSTAIN"
        return max_class
```

### Acceptance Criteria

| ID | 기준 |
|----|------|
| AC-AX-003-1 | 3-way softmax 예측 결과 sum=1.0±0.001 |
| AC-AX-003-2 | P(abstain) > 0.5일 때 ABSTAIN 분류 |
| AC-AX-003-3 | 벤치마크 3개 A등급 + 3개 B등급으로 학습 후 정확도 ≥ 75% |

## References

- SPEC-AX-001 research.md §4 (위험 등록부 R-AX-006)
- SPEC-AX-001 spec.md REQ-AX-003 (Grade Simulation)
- `.moai/project/structure.md` §5 (scoring module)
- scikit-learn MulticlassClassifier: https://scikit-learn.org/

---

**작성자**: ircp  
**날짜**: 2026-05-14
