# Sprint Contract: REQ-UBI (Sprint 1)

- SPEC: SPEC-AX-001 v0.1.2
- Sprint: 1 (REQ-UBI — 시스템 전반 불변 조건)
- Harness level: thorough
- Priority dimension: **Security** (데이터 주권 + 감사 가능성 = 컴플라이언스 constitutional invariant)
- 작성일: 2026-05-14
- 작성자: manager-tdd (RED phase)

---

## Acceptance Checklist

- [ ] AC-UBI-001: 데이터 주권 — 시스템이 외부 LLM 엔드포인트 호출을 거부/차단한다
- [ ] AC-UBI-002: 한국어 우선 처리 — 출력이 conversation_language=ko를 준수한다
- [ ] AC-UBI-003: 감사 로깅 — 모든 상태 변경 작업이 audit_logs 행을 영속화한다
- [ ] AC-UBI-004: sandbox user_id 기본값 — SSO 비활성화 시 audit_logs.user_id = 'cli-anonymous'

---

## Priority Dimension: Security

**선정 근거**: tech.md §9의 한국 공공기관 보안·컴플라이언스 요건을 구현하는 invariant다.
- REQ-UBI-001 위반(외부 API 호출)은 망분리 규정 즉시 위반 → PoC 전체 실패
- REQ-UBI-003/004 위반(감사 공백)은 공공기관 감사 추적 의무 위반

evaluator-active 4-dim 가중치:
- Security 40% / Functionality 30% / Completeness 20% / Originality 10%

---

## Test Scenarios (Pytest, RED Phase)

### AC-UBI-001: 외부 LLM API 차단

파일: `tests/unit/test_req_ubi_data_sovereignty.py`

```
시나리오: LLM_ENDPOINT가 외부 도메인(api.openai.com)으로 설정된 경우
Given: 악의적/잘못된 설정으로 외부 LLM 엔드포인트가 주입됨
When: llm_client 또는 settings allowlist 검증이 수행됨
Then: ExternalLLMBlockedError 예외 발생 (HTTP 호출 전 차단)
```

### AC-UBI-002: 한국어 우선 처리

파일: `tests/unit/test_req_ubi_korean_language.py`

```
시나리오: 언어 감지 서비스가 문서 언어를 'ko'로 반환
Given: 한국어 텍스트가 포함된 문서
When: 언어 감지 함수 호출
Then: lang_code == 'ko' AND 순수 영문 입력 시 low_korean_ratio 플래그 기록
```

### AC-UBI-003: 감사 이벤트 완전성

파일: `tests/unit/test_req_ubi_audit_logging.py`

```
시나리오: 상태 변경 작업 호출 시 audit_logs 행 삽입
Given: 감사 로거 인터페이스와 mock DB
When: audit_event() 호출 (document_upload, workflow_create 등)
Then: user_id, action, resource_id, timestamp 4필드 모두 포함된 레코드 삽입
```

### AC-UBI-004: sandbox user_id 기본값

파일: `tests/unit/test_req_ubi_sandbox_user_default.py`

```
시나리오: AUTH_ENABLED=false 상태에서 audit_event 호출
Given: settings.auth_enabled=False 픽스처
When: 사용자 컨텍스트 없이 audit_event() 호출
Then: audit_logs.user_id == 'cli-anonymous' (NULL 또는 빈 문자열 불허)
```

---

## Pass Conditions (RED Phase 종료 기준)

RED phase 검증 기준 (GREEN phase 진입 전):
- 4개 테스트 모두 **FAIL** 상태여야 함
- 실패 원인: `ModuleNotFoundError`, `ImportError`, `AttributeError`, 또는 assertion failure
- pytest collection 성공 (syntax error 없음)
- 실패가 "구현 미존재"로 인한 것임을 확인

GREEN phase 통과 기준 (이후 적용):
- 4개 AC G/W/T 시나리오 자동화 테스트 통과
- audit_logs 누락 필드 0건
- 외부 LLM API 호출 시도 시 ExternalLLMBlockedError 발생률 100%
- 단위 테스트 coverage >= 85%

---

## Implementation Seam Notes

RED phase에서 결정한 테스트 접합점:

1. **AC-UBI-001 seam**: `pipelines.generation.llm_client.LLMClient.generate()` 또는
   `pipelines.config.settings.Settings.validate_llm_endpoint()` — allowlist 검증이 발생하는
   가장 작은 단위. httpx.MockTransport 또는 monkeypatch로 외부 호출 시도를 시뮬레이션.

2. **AC-UBI-002 seam**: `pipelines.ingestion.language_detector.detect_language()` (GREEN phase
   신규 함수). 입력 텍스트 → `{"lang_code": "ko", "korean_ratio": 0.95}` 반환 구조.

3. **AC-UBI-003 seam**: `pkg.logging.logger.audit_event()` (GREEN phase 신규 함수). DB 세션을
   의존성 주입으로 받아 mock 처리. 반환값: 삽입된 레코드 dict.

4. **AC-UBI-004 seam**: settings.auth_enabled + settings.default_user_id 조합. monkeypatch로
   AUTH_ENABLED=false 설정 오버라이드.

---

## Sprint Contract Version

- Contract revision: 1.0 (RED phase draft)
- Evaluator-active scoring: pending (GREEN phase 완료 후)
- Next milestone: GREEN phase — `pkg/logging/logger.py` + `pipelines/config/settings.py` allowlist 구현
