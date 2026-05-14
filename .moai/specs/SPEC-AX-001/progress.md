# SPEC-AX-001 Implementation Progress

- SPEC: SPEC-AX-001 v0.1.2
- 개발 방법론: TDD (RED-GREEN-REFACTOR)
- Harness level: thorough
- 총 AC 목표: 24개 (acceptance.md 기준)
- 작성일: 2026-05-14

---

## Phase 0: Scaffolding — COMPLETE

- 완료 내용: pyproject.toml, pipelines/ 스켈레톤, settings.py, conftest.py 생성
- LSP baseline: errors=0, type_errors=0, lint_errors=0 (scaffolding 직후)
- 완료 AC: 0 / 24

---

## Phase 1: Plan — COMPLETE

- plan-auditor 점수: 0.86 (PASS)
- evaluator-active 점수: 0.813 (PASS)
- SPEC version: 0.1.2 (iteration 3 — AC-UBI-004 추가)
- 완료 AC: 0 / 24

---

## Sprint 1: REQ-UBI — RED Phase IN PROGRESS

- 진입일: 2026-05-14
- 단계: RED (실패 테스트 작성 완료)

### RED Phase 결과

| 지표 | 값 |
|------|---|
| 수집된 테스트 수 | 25 (4파일) |
| 실패 테스트 수 | 25 |
| 통과 테스트 수 | 0 |
| 실패 원인 | ModuleNotFoundError (pydantic 미설치 또는 구현 모듈 미존재) |
| Coverage | 0% (구현 없음, 예상됨) |
| RED 상태 확인 | YES |

### 생성된 테스트 파일

| 파일 | AC | 테스트 수 |
|------|----|----------|
| `tests/unit/test_req_ubi_data_sovereignty.py` | AC-UBI-001 | 5 |
| `tests/unit/test_req_ubi_korean_language.py` | AC-UBI-002 | 6 |
| `tests/unit/test_req_ubi_audit_logging.py` | AC-UBI-003 | 7 |
| `tests/unit/test_req_ubi_sandbox_user_default.py` | AC-UBI-004 | 7 |

### pytest 출력 (마지막 10줄)

```
FAILED tests/unit/test_req_ubi_data_sovereignty.py::TestDataSovereigntyLLMEndpoint::test_external_openai_endpoint_should_raise_blocked_error
FAILED tests/unit/test_req_ubi_korean_language.py::TestKoreanLanguagePrimary::test_korean_text_detection_should_return_ko_lang_code
FAILED tests/unit/test_req_ubi_audit_logging.py::TestAuditLoggingCompleteness::test_document_upload_audit_event_should_include_all_required_fields
FAILED tests/unit/test_req_ubi_sandbox_user_default.py::TestSandboxUserIdDefault::test_document_upload_with_sso_disabled_should_use_cli_anonymous_user_id
[... 21개 추가 FAILED ...]
======================== 25 failed, 1 warning in 0.20s =========================
```

### Sprint 1 완료 AC

| AC | 상태 |
|----|------|
| AC-UBI-001 (데이터 주권) | RED (테스트 작성 완료, 구현 미존재) |
| AC-UBI-002 (한국어 우선) | RED (테스트 작성 완료, 구현 미존재) |
| AC-UBI-003 (감사 로깅) | RED (테스트 작성 완료, 구현 미존재) |
| AC-UBI-004 (sandbox user_id) | RED (테스트 작성 완료, 구현 미존재) |

- 누적 AC 완료: 0 / 24
- 직전 대비 신규 AC 통과: 0 (RED phase — 예상됨)
- LSP error delta: +0 (테스트 파일만 추가, 구현 없음)
- Coverage delta: 0% → 0% (구현 없음)

### Re-planning Gate 체크

- 연속 zero AC 카운터: 1 (RED phase 첫 entry — 정상, 3회 연속 시 재계획 트리거)
- Stagnation: NO (RED phase는 zero AC가 정상)

---

## 다음 단계: Sprint 1 GREEN Phase

GREEN phase에서 구현할 모듈:
1. `pkg/logging/logger.py` — `audit_event()` 함수 (DB 세션 의존성 주입)
2. `pkg/errors/custom_errors.py` — `ExternalLLMBlockedError` 예외 클래스
3. `pipelines/config/settings.py` — `validate_llm_endpoint()` 함수 + allowlist 로직
4. `pipelines/generation/llm_client.py` — `LLMClient` 클래스 (allowlist 검증 포함)
5. `pipelines/ingestion/language_detector.py` — `detect_language()` 함수

GREEN phase 통과 기준:
- 25개 테스트 모두 PASS
- LSP errors=0, type_errors=0, lint_errors=0
- Coverage >= 85%
