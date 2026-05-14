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

---

## Sprint 4: REQ-AX-003 — RED Phase COMPLETE

- 진입일: 2026-05-14
- 단계: RED (실패 테스트 작성 완료)

### RED Phase 결과

| 지표 | 값 |
|------|---|
| 수집된 테스트 수 | 29 (4파일) |
| 실패 테스트 수 | 29 (ModuleNotFoundError — 구현 모듈 미존재) |
| 통과 테스트 수 | 0 |
| 실패 원인 | ModuleNotFoundError: pipelines.scoring.{benchmark_learner, grade_predictor, scenario_simulator} |
| Sprint 1+2+3 회귀 | 없음 (89/89 통과 유지) |
| Coverage | 0% (구현 없음, 예상됨) |
| RED 상태 확인 | YES |

### 생성된 테스트 파일

| 파일 | AC | 테스트 수 |
|------|----|----------|
| `tests/unit/test_req_ax_003_benchmark_learner.py` | AC-003-1 (×5), AC-003-3 (×1), AC-003-3 학습 중 차단 (×1) | 7 |
| `tests/unit/test_req_ax_003_grade_predictor.py` | AC-003-1 (×6), AC-003-2 abstain (×6), AC-003-3 metadata (×2) | 14 |
| `tests/unit/test_req_ax_003_scenario_simulator.py` | B→A 시나리오 (×5) | 5 |
| `tests/unit/test_req_ax_003_integration.py` | E2E 통합 (×3) | 3 |

### 추가 생성 파일

| 파일 | 내용 |
|------|------|
| `.moai/sprints/SPEC-AX-001/sprint-REQ-AX-003.md` | Sprint Contract (Priority: Functionality, 수학 불변식 + abstain 분기) |
| `pkg/models/simulation.py` | GradeDistribution, BenchmarkReport, ScenarioResult Pydantic 모델 (불변식 검증 포함) |

### Sprint 4 AC 상태

| AC | 설명 | 상태 |
|----|------|------|
| AC-003-1 | {A, B, abstain} sum=1.0±0.001 + 1s 응답 + simulations 저장 | RED |
| AC-003-2 | max(p_a, p_b) < 0.5 → status=low_confidence, prediction=null, candidates 반환 | RED |
| AC-003-3 | 학습 중 HTTP 503 + model_used 메타데이터 추적 | RED |

- 누적 AC 완료: 0 / 24
- 직전 대비 신규 AC 통과: 0 (RED phase — 예상됨)
- LSP error delta: +0 (테스트/모델 파일만 추가, 구현 없음)
- Coverage delta: 0% → 0% (구현 없음)

### Re-planning Gate 체크

- 연속 zero AC 카운터: 4 (RED phase 네 번째 entry — 정상, RED phase는 zero AC가 정상)
- Stagnation: NO (RED phase 완료, GREEN 진입 예정)

---

---

## Sprint 5: REQ-AX-004 — RED Phase COMPLETE

- 진입일: 2026-05-14
- 단계: RED (실패 테스트 작성 완료)

### RED Phase 결과

| 지표 | 값 |
|------|---|
| 수집된 테스트 수 | 38 (5파일, integration 제외) |
| 실패 테스트 수 | 38 |
| 통과 테스트 수 | 0 |
| 실패 원인 | NotImplementedError (구현 모듈 스텁만 존재) |
| Sprint 1+2+3+4 회귀 | 없음 (118/118 통과 유지) |
| Coverage | 0% (구현 없음, 예상됨) |
| RED 상태 확인 | YES |

### 생성된 파일

| 파일 | 내용 | 테스트 수 |
|------|------|----------|
| `.moai/sprints/SPEC-AX-001/sprint-REQ-AX-004.md` | Sprint Contract (Originality + Functionality) | — |
| `tests/unit/test_req_ax_004_prompt_builder.py` | PB-1~PB-5: PromptBuilder 계약 | 7 |
| `tests/unit/test_req_ax_004_style_applier.py` | SA-1~SA-8: 합니다체 검증 | 15 |
| `tests/unit/test_req_ax_004_llm_client_generate.py` | LC-1~LC-7 + fallback: generate() 계약 | 9 (1 @gpu 제외) |
| `tests/unit/test_req_ax_004_report_drafter.py` | RD-1~RD-5: 오케스트레이터 계약 | 7 |
| `tests/unit/test_req_ax_004_integration.py` | INT-1~INT-3: E2E 파이프라인 (@integration) | (3, @integration 제외) |
| `pkg/models/report.py` (수정) | GenerationResult, StyleReport, DraftSection 모델 추가 | — |
| `pipelines/generation/prompt_builder.py` | PromptBuilder 스텁 | — |
| `pipelines/generation/style_applier.py` | StyleApplier 스텁 | — |
| `pipelines/generation/report_drafter.py` | ReportDrafter 스텁 | — |

### pytest 출력 요약

```
============ 38 failed, 118 passed, 8 deselected, 1 warning in 2.47s ============
```

- Sprint 5 테스트 38개: 전부 FAILED (RED 상태 확인)
- Sprint 1+2+3+4 테스트 118개: 전부 PASSED (회귀 없음)
- @integration/@gpu 테스트 8개: deselected (별도 마크)

### Sprint 5 AC 상태

| AC | 설명 | 상태 |
|----|------|------|
| AC-004-1 | 합니다체 초안 생성 + style_applier 통과 | RED |
| AC-004-2 | 스타일 위반 → 최대 2 재시도 → style_violation | RED |
| AC-004-3 | Qwen 2.5 fallback + model_used 메타데이터 추적 | RED |

- 누적 AC 완료: 0 / 24
- 직전 대비 신규 AC 통과: 0 (RED phase — 예상됨)
- LSP error delta: +0 (테스트/스텁 파일만 추가, 구현 없음)
- Coverage delta: 0% → 0% (구현 없음)

### Re-planning Gate 체크

- 연속 zero AC 카운터: 5 (RED phase 다섯 번째 entry — 정상, RED phase는 zero AC가 정상)
- Stagnation: NO (RED phase 완료, GREEN 진입 예정)

---

---

## Sprint 6: REQ-AX-005 — RED Phase COMPLETE

- 진입일: 2026-05-14
- 단계: RED (실패 테스트 작성 완료)

### RED Phase 결과

| 지표 | 값 |
|------|---|
| 수집된 테스트 수 | 21 (4파일) |
| 실패 테스트 수 | 21 |
| 통과 테스트 수 | 0 |
| 실패 원인 | ModuleNotFoundError (pipelines.recommendation.{gap_analyzer,content_suggester,prioritizer} 미구현) |
| Sprint 1+2+3+4+5 회귀 | 없음 (156/156 통과 유지) |
| Coverage | 0% (구현 없음, 예상됨) |
| RED 상태 확인 | YES |

### 생성된 파일

| 파일 | AC | 테스트 수 |
|------|----|----------|
| `.moai/sprints/SPEC-AX-001/sprint-REQ-AX-005.md` | Sprint Contract (Priority: Completeness) | — |
| `pkg/models/recommendation.py` | GapItem, ContentSuggestion, RankedSuggestion 스텁 | — |
| `tests/unit/test_req_ax_005_gap_analyzer.py` | GA-1~GA-7: GapAnalyzer.analyze 계약 | 7 |
| `tests/unit/test_req_ax_005_content_suggester.py` | CS-1~CS-5: ContentSuggester.suggest + AC-005-2/3 | 5 |
| `tests/unit/test_req_ax_005_prioritizer.py` | PR-1~PR-7: Prioritizer.prioritize 정렬·캡·불변식 | 7 |
| `tests/unit/test_req_ax_005_integration.py` | INT-1~INT-2: B→A 전체 파이프라인 | 2 |

### pytest 출력 요약

```
============ 21 failed, 156 passed, 8 deselected, 1 warning in 1.75s ============
```

- Sprint 6 테스트 21개: 전부 FAILED (RED 상태 확인)
- Sprint 1+2+3+4+5 테스트 156개: 전부 PASSED (회귀 없음)
- @integration/@gpu 테스트 8개: deselected (별도 마크)

### Sprint 6 AC 상태

| AC | 설명 | 상태 |
|----|------|------|
| AC-005-1 | B→A 3-5개 우선순위 제안 + feasibility_score 내림차순 | RED |
| AC-005-2 | 벤치마크 없음 → benchmark_not_available + 빈 리스트 (fabricated 금지) | RED |
| AC-005-3 | criterion_id 역추적 (GapItem → ContentSuggestion → RankedSuggestion) | RED |

- 누적 AC 완료: 0 / 24
- 직전 대비 신규 AC 통과: 0 (RED phase — 예상됨)
- LSP error delta: +0 (테스트/모델 스텁 파일만 추가, 구현 없음)
- Coverage delta: 0% → 0% (구현 없음)

### Re-planning Gate 체크

- 연속 zero AC 카운터: 6 (RED phase 여섯 번째 entry — 정상, RED phase는 zero AC가 정상)
- Stagnation: NO (RED phase 완료, GREEN 진입 예정)

---

## 다음 단계: Sprint 6 GREEN Phase

GREEN phase에서 구현할 모듈 (Sprint 6 신규):
1. `pipelines/recommendation/gap_analyzer.py` — `GapAnalyzer.analyze()` (현재 분포 + 벤치마크 → list[GapItem])
2. `pipelines/recommendation/content_suggester.py` — `ContentSuggester.suggest()` (GapItem + Retriever → list[ContentSuggestion])
3. `pipelines/recommendation/prioritizer.py` — `Prioritizer.prioritize()` (feasibility × score_delta 정렬, top_k 캡)
4. `pkg/errors/custom_errors.py` — `BenchmarkNotAvailableError` 추가 (AC-005-2)

GREEN phase 통과 기준 (Sprint 6):
- 21개 Sprint 6 테스트 모두 PASS
- 156개 Sprint 1+2+3+4+5 테스트 회귀 없음 유지
- LSP errors=0, type_errors=0, lint_errors=0
- Coverage >= 85%

---

## 다음 단계: Sprint 5 GREEN Phase (이전 목록 보존)

GREEN phase에서 구현할 모듈 (Sprint 4 신규):
1. `pipelines/scoring/benchmark_learner.py` — `BenchmarkLearner` 클래스 (TF-IDF + scikit-learn LogisticRegression, state machine: idle→training→ready)
2. `pipelines/scoring/grade_predictor.py` — `GradePredictor` 클래스 (2-class softmax + abstain 3-way output, p_abstain=1-p_a-p_b)
3. `pipelines/scoring/scenario_simulator.py` — `ScenarioSimulator` 클래스 (B→A 시나리오 시뮬레이션)
4. `pkg/errors/custom_errors.py` — `ModelTrainingError` 추가 (학습 중 예측 차단용)

GREEN phase 통과 기준 (Sprint 4):
- 29개 Sprint 4 테스트 모두 PASS
- 89개 Sprint 1+2+3 테스트 회귀 없음 유지
- 확률 합 불변식 100건 배치 위반 0건
- LSP errors=0, type_errors=0, lint_errors=0
- Coverage >= 85%

Sprint 3 GREEN phase에서 구현할 모듈 (이전 목록 보존):
1. `pkg/errors/custom_errors.py` — `IndexNotBootstrappedError`, `IndexRebuildingError`
2. `pipelines/mapping/criterion_parser.py`
3. `pipelines/mapping/embedding_service.py`
4. `pipelines/mapping/vector_store.py`
5. `pipelines/mapping/retriever.py`

Sprint 2 GREEN phase에서 구현할 모듈 (이전 목록 보존):
1. `pkg/models/document.py`
2. `pkg/errors/custom_errors.py` — `OCRConcurrencyError`
3. `pipelines/ingestion/hwp_parser.py`
4. `pipelines/ingestion/pdf_parser.py`
5. `pipelines/ingestion/vlm_processor.py`
6. `pipelines/ingestion/table_extractor.py`
7. `tests/fixtures/generate_fixtures.py`
