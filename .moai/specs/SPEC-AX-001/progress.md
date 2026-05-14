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

---

## Sprint 2: REQ-AX-001 — RED Phase COMPLETE

- 진입일: 2026-05-14
- 단계: RED (실패 테스트 작성 완료)

### RED Phase 결과

| 지표 | 값 |
|------|---|
| 수집된 테스트 수 | 31 (5파일) |
| 실패 테스트 수 | 31 |
| 통과 테스트 수 | 0 |
| 실패 원인 | ModuleNotFoundError (pipelines.ingestion.* 및 pkg.errors.custom_errors 미구현) |
| Sprint 1 회귀 | 없음 (25/25 통과 유지) |
| Coverage | 0% (구현 없음, 예상됨) |
| RED 상태 확인 | YES |

### 생성된 테스트 파일

| 파일 | AC | 테스트 수 |
|------|----|----------|
| `tests/unit/test_req_ax_001_hwp_parser.py` | AC-001-1 (×5), AC-001-2 (×4) | 9 |
| `tests/unit/test_req_ax_001_pdf_parser.py` | AC-001-3 (×3), 정상 경로 (×2) | 5 |
| `tests/unit/test_req_ax_001_vlm_processor.py` | AC-001-5 CPU (×4), GPU @gpu (×2) | 6 |
| `tests/unit/test_req_ax_001_table_extractor.py` | AC-001-3 심층 (×4), 인터페이스 (×2) | 6 |
| `tests/unit/test_req_ax_001_integration.py` | AC-001-1 통합 (×2), AC-001-2 통합 (×1), AC-001-4 (×2) | 5 |

### pytest 출력 요약

```
======================== 31 failed, 25 passed, 1 warning in 0.27s =========================
```

- Sprint 2 테스트 31개: 전부 FAILED (RED 상태 확인)
- Sprint 1 테스트 25개: 전부 PASSED (회귀 없음)

### 추가 생성 파일

| 파일 | 내용 |
|------|------|
| `.moai/sprints/SPEC-AX-001/sprint-REQ-AX-001.md` | Sprint Contract (Thorough harness 필수) |
| `tests/fixtures/synthetic/README.md` | 합성 픽스처 커밋 정책 및 생성 방법 |
| `tests/conftest.py` (수정) | gpu/integration/slow_cpu 마크 + mock_hwp_doc/mock_pdf_doc/mock_qwen2vl 픽스처 추가 |

### Sprint 2 AC 상태

| AC | 설명 | 상태 |
|----|------|------|
| AC-001-1 | 정상 HWP 파싱 (ParsedDocument 반환, 95% 정확도) | RED |
| AC-001-2 | OLE 손상 HWP → VLM OCR 폴백 (status='ocr_fallback') | RED |
| AC-001-3 | 90° 회전 PDF 표 추출 (90% 셀 정확도) | RED |
| AC-001-4 | 동시 OCR 요청 큐잉 (HTTP 409 / OCRConcurrencyError) | RED |
| AC-001-5 | GPU/CPU 분기 메타데이터 기록 (vllm_gpu / transformers_cpu) | RED |

- 누적 AC 완료: 0 / 24
- 직전 대비 신규 AC 통과: 0 (RED phase — 예상됨)
- LSP error delta: +0 (테스트 파일만 추가, 구현 없음)
- Coverage delta: 0% → 0% (구현 없음)

### Re-planning Gate 체크

- 연속 zero AC 카운터: 2 (RED phase 두 번째 entry — 정상, 3회 연속 시 재계획 트리거)
- Stagnation: NO (RED phase는 zero AC가 정상)

---

## Sprint 3: REQ-AX-002 — RED Phase COMPLETE

- 진입일: 2026-05-14
- 단계: RED (실패 테스트 작성 완료)

### RED Phase 결과

| 지표 | 값 |
|------|---|
| 수집된 테스트 수 | 35 (5파일) |
| 실패 테스트 수 | 35 |
| 통과 테스트 수 | 0 |
| 실패 원인 | ModuleNotFoundError (pipelines.mapping.* 미구현) |
| Sprint 1+2 회귀 | 없음 (54/54 통과 유지) |
| Coverage | 0% (구현 없음, 예상됨) |
| RED 상태 확인 | YES |

### 생성된 테스트 파일

| 파일 | AC | 테스트 수 |
|------|----|----------|
| `tests/unit/test_req_ax_002_criterion_parser.py` | AC-002-1, AC-002-3, AC-002-5 | 7 |
| `tests/unit/test_req_ax_002_embedding_service.py` | 임베딩 계약 (vec dim 768, norm > 0, 한자) | 5 |
| `tests/unit/test_req_ax_002_vector_store.py` | AC-002-4, AC-002-6, upsert/query | 10 |
| `tests/unit/test_req_ax_002_retriever.py` | AC-002-3, AC-002-4, AC-002-5, AC-002-6 | 10 |
| `tests/unit/test_req_ax_002_integration.py` | AC-002-1 통합 파이프라인 | 3 |

### pytest 출력 요약

```
============ 35 failed, 54 passed, 4 deselected, 1 warning in 0.36s ============
```

- Sprint 3 테스트 35개: 전부 FAILED (RED 상태 확인)
- Sprint 1+2 테스트 54개: 전부 PASSED (회귀 없음)

### 추가 생성/수정 파일

| 파일 | 내용 |
|------|------|
| `.moai/sprints/SPEC-AX-001/sprint-REQ-AX-002.md` | Sprint Contract (thorough harness) |
| `pkg/models/criterion.py` | Criterion, CriterionMatch, ColdStartResponse 스텁 확장 |

### Sprint 3 AC 상태

| AC | 설명 | 상태 |
|----|------|------|
| AC-002-1 | top-3 검색, relevance > 0.7, 계층 포함 | RED |
| AC-002-2 | insufficient_context 명시 상태 | RED |
| AC-002-3 | 항목→지표→배점 계층 보존 + 한자/한글 정규화 | RED |
| AC-002-4 | HNSW 재구축 중 큐잉 또는 503 | RED |
| AC-002-5 | 한자 정규화 실패 graceful (no 500, confidence × 0.8) | RED |
| AC-002-6 | cold-start 명시 응답 (no silent empty / no 500) | RED |

- 누적 AC 완료: 0 / 24
- 직전 대비 신규 AC 통과: 0 (RED phase — 예상됨)
- LSP error delta: +0 (테스트/스텁 파일만 추가, 구현 없음)
- Coverage delta: 0% → 0% (구현 없음)

### Re-planning Gate 체크

- 연속 zero AC 카운터: 3 (RED phase 세 번째 entry — 정상, RED phase는 zero AC가 정상)
- Stagnation: NO (RED phase 완료, GREEN 진입 예정)

### 모의 전략 결정

| 컴포넌트 | 단위 테스트 전략 | 통합 테스트 전략 |
|---------|--------------|---------------|
| ko-sroberta-multitask | `unittest.mock.patch("...SentenceTransformer")` | 동일 |
| pgvector DB 연결 | `MagicMock()` (mock_pgvector_conn) | testcontainers PostgreSQL+pgvector |
| IndexNotBootstrappedError | `Exception("index_not_bootstrapped: ...")` 로 대체 (GREEN에서 실제 클래스) | 실제 예외 클래스 사용 |

---

## 다음 단계: Sprint 3 GREEN Phase

GREEN phase에서 구현할 모듈:
1. `pkg/errors/custom_errors.py` — `IndexNotBootstrappedError`, `IndexRebuildingError` 예외 클래스 추가
2. `pipelines/mapping/criterion_parser.py` — `CriterionParser` (평가편람 PDF 파싱, hanja 정규화)
3. `pipelines/mapping/embedding_service.py` — `EmbeddingService` (ko-sroberta-multitask wrapping)
4. `pipelines/mapping/vector_store.py` — `VectorStore` (pgvector HNSW, FakeVectorStore 포함)
5. `pipelines/mapping/retriever.py` — `Retriever` (embed + query 오케스트레이션, cold-start 처리)

GREEN phase 통과 기준:
- 35개 Sprint 3 테스트 모두 PASS
- 54개 Sprint 1+2 테스트 회귀 없음 유지
- LSP errors=0, type_errors=0, lint_errors=0
- Coverage >= 85%

GREEN phase에서 구현할 모듈:
1. `pkg/models/document.py` — `ParsedDocument`, `Table`, `DocumentMetadata` Pydantic 모델 추가
2. `pkg/errors/custom_errors.py` — `OCRConcurrencyError` 예외 클래스 추가
3. `pipelines/ingestion/hwp_parser.py` — `HWPParser` 클래스 (vlm_processor 의존성 주입, OLE 폴백 로직)
4. `pipelines/ingestion/pdf_parser.py` — `PDFParser` 클래스 (회전 페이지 처리)
5. `pipelines/ingestion/vlm_processor.py` — `VLMProcessor` 클래스 (CPU/GPU 분기, last_inference_meta)
6. `pipelines/ingestion/table_extractor.py` — `TableExtractor` 클래스 (회전 보정, 셀 논리 순서)
7. `tests/fixtures/generate_fixtures.py` — 합성 HWP/PDF 픽스처 생성 스크립트

GREEN phase 통과 기준:
- 31개 Sprint 2 테스트 모두 PASS
- 25개 Sprint 1 테스트 회귀 없음 유지
- LSP errors=0, type_errors=0, lint_errors=0
- Coverage >= 85%
