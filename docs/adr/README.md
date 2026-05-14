# Architecture Decision Records

본 디렉토리는 iroum-ax 프로젝트의 주요 아키텍처 결정을 영구 보존하는 ADR(Architecture Decision Record) 모음입니다.

## 활용 방법

- 모든 ADR은 immutable. 결정이 바뀌면 새 ADR을 생성하고 이전 ADR의 Status를 Superseded로 변경
- 신규 결정 시 ADR 작성 → SPEC 작성에 인용 → 코드 구현
- 형식: [Michael Nygard 표준](https://github.com/joelparkerhenderson/architecture-decision-record)

## ADR 목록

| 번호 | 제목 | 상태 | 날짜 | 관련 SPEC |
|---|---|---|---|---|
| [0001](0001-tdd-with-thorough-harness.md) | TDD + thorough harness 채택 | Accepted | 2026-05-14 | SPEC-AX-001 |
| [0002](0002-self-hosted-llm-data-sovereignty.md) | 자체 호스팅 LLM (Qwen 2.5) + 망분리 정합 | Accepted | 2026-05-14 | SPEC-AX-001 |
| [0003](0003-abstain-3way-softmax-classifier.md) | 2-class softmax + abstain 3-way 출력 | Accepted | 2026-05-14 | SPEC-AX-001 v0.1.2 |
| [0004](0004-celery-from-go-redis-direct.md) | Go → Celery: Redis-direct Kombu v2 JSON envelope | Accepted | 2026-05-14 | SPEC-AX-CTRL-001 |
| [0005](0005-state-machine-4-states.md) | Workflow State Machine: 4 states (RETRYING/CANCELLED 제외) | Accepted | 2026-05-14 | SPEC-AX-CTRL-001 |
| [0006](0006-keycloak-oidc-provider.md) | OIDC Provider: Keycloak 24.x LTS (전자정부 표준 후속 SPEC) | Accepted | 2026-05-14 | SPEC-AX-AUTH-001 |
| [0007](0007-cli-anonymous-fallback.md) | Sandbox 환경 user_id 기본값: `cli-anonymous` | Accepted | 2026-05-14 | SPEC-AX-001 + CTRL-001 |

---

## ADR 참조 (Refer to)

각 ADR은 다음 섹션을 포함합니다:

- **Status**: Proposed, Accepted, Deprecated, Superseded
- **Context**: 결정 배경 및 동기
- **Decision**: 선택한 옵션
- **Consequences**: 결정의 긍정적/부정적 영향
- **References**: 관련 SPEC, 문서, 외부 자료

---

**최종 업데이트**: 2026-05-14  
**프로젝트**: iroum-ax (한국 공공기관 경영평가 AI 플랫폼)  
**Anchor 고객**: KEPCO E&C
