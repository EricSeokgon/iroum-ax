---
name: 기능 요청
about: 새 기능 또는 개선 사항을 제안합니다 (SPEC 작성 전 단계)
title: "[FEAT] "
labels: ["enhancement", "triage"]
assignees: []
---

## 동기 (Why)

<!-- 이 기능이 왜 필요한지, 어떤 문제를 해결하는지 -->

## 제안 사항 (What)

<!-- 어떤 기능을 추가/변경하고 싶은지 구체적으로 -->

## 사용자 시나리오

<!-- 누가, 언제, 어떻게 이 기능을 사용할지 -->

**페르소나**: <!-- 예: KEPCO E&C 경영평가팀 담당자 -->

**시나리오**:
1. ...
2. ...
3. ...

## 대안 (Alternatives)

<!-- 고려한 다른 방법들과 거부 이유 -->

## 영향 범위

- [ ] Python pipelines (Document Ingestion / Mapping / Scoring / Generation / Recommendation)
- [ ] Go control plane (State Machine / gRPC / REST / Postgres / Celery)
- [ ] Console UI (현재 Exclusions)
- [ ] 인프라 (Docker / K8s / 모니터링)
- [ ] 신규 도메인 (ESG / 감사 / 면허 / 공문 / 금융권)

## SPEC 후보

<!-- 이 기능이 새 SPEC으로 작성될 경우 예상 ID -->

- 후보 ID: SPEC-AX-XXX-NNN
- 후보 도메인: <!-- AX / CTRL / AUTH / 기타 -->
- 우선순위: <!-- High / Medium / Low -->

## 관련 SPEC / Issue

<!-- 기존 SPEC이나 issue 링크 -->

## TRUST 5 사전 평가

<!-- 이 기능이 어떻게 TRUST 5 원칙을 충족할지 -->

- **Tested**: 어떤 테스트 시나리오가 필요한지
- **Readable**: 핵심 도메인 용어 (예: 평가편람, 합니다체)
- **Unified**: 기존 스택과 정합성 (Qwen 2.5 / pgvector / Go / Python)
- **Secured**: 망분리 / PII / 감사 로깅 영향
- **Trackable**: 어떤 @MX 태그가 필요한지

## 추가 컨텍스트

<!-- 참고 자료, 스크린샷, 외부 링크 등 -->
