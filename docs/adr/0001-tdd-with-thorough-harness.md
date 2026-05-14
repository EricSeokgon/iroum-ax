# ADR 0001: TDD + thorough harness 채택

## Status

Accepted (2026-05-14)

## Context

iroum-ax는 한국 공기업 경영평가 도메인 anchor (KEPCO E&C) — 등급 직결 신뢰성 critical. Greenfield 프로젝트로 시작.

품질 방법론 선택지:
- **TDD (RED-GREEN-REFACTOR)**: 명세 우선, 회귀 보장 강력
- **DDD (ANALYZE-PRESERVE-IMPROVE)**: 기존 코드 보존, characterization tests
- **빠른 프로토타이핑**: 검증 후행

Harness 선택지:
- **minimal**: 빠른 반복, 적대적 감사 생략
- **standard**: 균형 (evaluator-active final-pass)
- **thorough**: per-sprint Sprint Contract + plan-auditor + evaluator-active 교차 검증

## Decision

**TDD 모드 + thorough harness** 채택.

근거:
- Greenfield → 기존 코드 없음 (DDD characterization 불필요)
- KEPCO 신뢰성 → 자동화 테스트 회귀 가드 필수
- 적대적 감사 → 단일 시점 SPEC 결함을 다중 시점에 분산 차단

## Consequences

### 긍정적 영향

- **272개 테스트 / 회귀 0건** (3 SPECs 누적)
- **plan-auditor 단독 PASS 후 evaluator-active 교차 검증**으로 D11/D12 등 미발견 결함 추가 식별
- **Sprint Contract 명시**로 구현자-평가자 sprint 단위 합의 보장
- 테스트 주도 설계로 초기 설계 품질 상향

### 부정적 영향

- 토큰 비용 증가 (per-sprint evaluator + 적대적 감사 추가 호출)
- 초기 SPEC 작성 시간 증가 (research.md + plan-auditor 반복)
- 개발자 훈련 필요 (TDD 규율)

### 대안 평가 (Rejected)

| 대안 | 이유 |
|------|------|
| Minimal harness | 회귀 가드 부족, KEPCO 운영 부적합 |
| Standard harness | abstain mathematical invariant (REQ-AX-003) 같은 cross-cutting 결함 발견 빈도 낮음 |
| DDD 모드 | Greenfield 단계에서 기존 코드 0줄, characterization test 필요 없음 |

## References

- SPEC-AX-001 v0.1.2 (Plan-Run-Sync 완전 종료)
- SPEC-AX-CTRL-001 v0.1.2 (7-sprint Run phase)
- `.moai/config/sections/harness.yaml` (thorough 정의)
- `.moai/config/sections/quality.yaml` (development_mode: tdd)
- `.claude/rules/moai/core/moai-constitution.md` (TRUST 5 framework)

---

**작성자**: ircp  
**날짜**: 2026-05-14
