---
id: SPEC-AX-PIPE-001
version: 0.1.2
status: draft
created: 2026-05-21
updated: 2026-05-21
author: ircp
priority: high
issue_number: 0
---

# SPEC-AX-PIPE-001 — Python AI 파이프라인 구현

## HISTORY

- 2026-05-21 v0.1.2 (draft): plan-auditor iter2 반영. Fix-D-NEW-1: 검증 매트릭스 coverage 80%→85%. Fix-D-NEW-2: AC-PIPE-009-1 목록 16→19개 (ContentSuggester.suggest, StyleApplier.validate, PromptBuilder.build 추가). MP-2(EARS for AC)/MP-3(labels/created_at) 거부 — 본 프로젝트 canonical schema D4/D5 결정에 따름.
- 2026-05-21 v0.1.1 (draft): plan-auditor iter1 반영. Fix-1: AC-PIPE-009 커버리지 통일(85%). Fix-2: AC-PIPE-008-2 action/resource_id 필드 추가(REQ-UBI-003). Fix-3: AC-PIPE-006-5 한국어 출력 AC 추가(REQ-UBI-002). Fix-4: AC-PIPE-009-1 @MX:ANCHOR → 16개 public 메서드 명시 목록으로 대체.
- 2026-05-21 v0.1.0 (draft): 초안 작성. 부모 SPEC-AX-001 의 5개 도메인 스켈레톤(37 Python 파일) 에 실 로직 + 7개 FastAPI REST 엔드포인트 + TDD 테스트 스위트를 채워 넣는 구현 SPEC.

---

## 1. 개요

본 SPEC 은 KEPCO E&C 경영평가 PoC 의 Python AI 파이프라인(`pipelines/`) 에 대해 다음을 정의한다.

1. **실 로직 구현** — 기존 스켈레톤 인터페이스(PDFParser, HWPParser, VLMProcessor, EmbeddingService, VectorStore, BenchmarkLearner, GradePredictor, LLMClient, GapAnalyzer 등) 의 stub/`NotImplementedError` 부분을 실제 동작 코드로 채운다.
2. **REST API** — `pipelines/main.py` 에 7개 FastAPI 엔드포인트(`/api/documents/upload`, `/api/criteria/index`, `/api/criteria/search`, `/api/simulations/predict`, `/api/reports/generate`, `/api/recommendations/generate`, `/api/recommendations/{id}/feedback`) 를 신규 추가한다.
3. **Pydantic 모델 보강** — `pipelines/config/models.py` 에 Sprint 2-6 요청/응답 모델을 추가한다.
4. **TDD 테스트 스위트** — 단위 테스트(17 모듈) + 통합 테스트(testcontainers PostgreSQL + pgvector) 를 작성한다.
5. **Audit Logging** — 모든 액션에 대해 `audit_logs` 테이블에 기록한다 (REQ-UBI-003).

부모 SPEC-AX-001 의 SLA(인제스션 30s, 검색 p99 100ms, 예측 1s, 생성 5s, 추천 3s) 및 REQ-UBI 제약(localhost LLM, Korean-first, audit logging) 을 상속한다.

---

## 2. 영향받는 파일

| 분류 | 파일 경로 | 변경 유형 |
|------|----------|----------|
| API Entry | `pipelines/main.py` | 7개 엔드포인트 추가 |
| Models | `pipelines/config/models.py` | Sprint 2-6 요청/응답 모델 추가 |
| Ingestion | `pipelines/ingestion/pdf_parser.py` | `_load_pdf()` 실 pypdf 호출 |
| Ingestion | `pipelines/ingestion/hwp_parser.py` | `_load_hwp()` pyhwp + VLM 폴백 분기 |
| Ingestion | `pipelines/ingestion/vlm_processor.py` | `_run_inference()` transformers 통합 (D1 가드) |
| Ingestion | `pipelines/ingestion/table_extractor.py` | stub 유지 (D5) |
| Mapping | `pipelines/mapping/criterion_parser.py` | 실 평가편람 PDF 파싱 |
| Mapping | `pipelines/mapping/embedding_service.py` | `SentenceTransformer.encode` 실 호출 |
| Mapping | `pipelines/mapping/vector_store.py` | psycopg2 + pgvector 실 쿼리 |
| Mapping | `pipelines/mapping/retriever.py` | cold-start + 한자 페널티 검증 |
| Scoring | `pipelines/scoring/benchmark_learner.py` | sklearn 학습 파이프라인 완성 |
| Scoring | `pipelines/scoring/grade_predictor.py` | softmax + abstain 산식 완성 |
| Scoring | `pipelines/scoring/scenario_simulator.py` | coef_ 기반 시뮬레이션 완성 |
| Generation | `pipelines/generation/llm_client.py` | `_call_model()` httpx 호출 (D2 가드) |
| Generation | `pipelines/generation/report_drafter.py` | 3 retry + StyleApplier 통합 |
| Recommendation | `pipelines/recommendation/gap_analyzer.py` | fabrication guard 강화 |
| Recommendation | `pipelines/recommendation/content_suggester.py` | Retriever 통합 |
| Recommendation | `pipelines/recommendation/prioritizer.py` | priority_score 산식 검증 |
| Tests (Unit) | `tests/unit/ingestion/test_pdf_parser.py` | 신규 |
| Tests (Unit) | `tests/unit/ingestion/test_hwp_parser.py` | 신규 |
| Tests (Unit) | `tests/unit/ingestion/test_vlm_processor.py` | 신규 |
| Tests (Unit) | `tests/unit/ingestion/test_language_detector.py` | 신규 |
| Tests (Unit) | `tests/unit/mapping/test_criterion_parser.py` | 신규 |
| Tests (Unit) | `tests/unit/mapping/test_embedding_service.py` | 신규 |
| Tests (Unit) | `tests/unit/mapping/test_vector_store.py` | 신규 |
| Tests (Unit) | `tests/unit/mapping/test_retriever.py` | 신규 |
| Tests (Unit) | `tests/unit/scoring/test_benchmark_learner.py` | 신규 |
| Tests (Unit) | `tests/unit/scoring/test_grade_predictor.py` | 신규 |
| Tests (Unit) | `tests/unit/scoring/test_scenario_simulator.py` | 신규 |
| Tests (Unit) | `tests/unit/generation/test_llm_client.py` | 신규 |
| Tests (Unit) | `tests/unit/generation/test_report_drafter.py` | 신규 |
| Tests (Unit) | `tests/unit/generation/test_style_applier.py` | 신규 |
| Tests (Unit) | `tests/unit/recommendation/test_gap_analyzer.py` | 신규 |
| Tests (Unit) | `tests/unit/recommendation/test_content_suggester.py` | 신규 |
| Tests (Unit) | `tests/unit/recommendation/test_prioritizer.py` | 신규 |
| Tests (Integration) | `tests/integration/test_pipeline_e2e.py` | 신규 (testcontainers E2E) |
| Tests (Shared) | `tests/conftest.py` | 공용 fixtures (fake_db, sample bytes) |

---

## 3. 요구사항 (EARS Format)

### REQ-PIPE-001 — Document Ingestion API

**When** 사용자가 `POST /api/documents/upload` 로 HWP 또는 PDF 파일을 업로드할 때, the system **SHALL** 파일을 파싱하여 `documents` 테이블에 저장하고 `document_id` 와 파싱 메타데이터를 반환한다.

**Acceptance Criteria**:
- AC-PIPE-001-1: PDF 업로드 시 `document_id` 반환, `status=PENDING`, `documents` 테이블에 영속 저장된다.
- AC-PIPE-001-2: HWP 업로드 시 `parsed_text` 추출, `language=korean`, `parse_quality_flag` 가 설정된다.
- AC-PIPE-001-3: HWP OLE 파싱 실패 시 `vlm_processor.ocr()` 폴백이 호출되어 텍스트가 추출된다.
- AC-PIPE-001-4: HWP/PDF 외 파일 타입에 대해 **HTTP 400** 이 반환된다.
- AC-PIPE-001-5: 업로드 1건당 `audit_logs` 1건이 `action='document_upload'` 로 생성된다.

**Non-Functional**: 30-sec 인제스션 SLA (REQ-AX-001 상속).

---

### REQ-PIPE-002 — Criteria Indexing API

**When** 사용자가 `POST /api/criteria/index` 로 평가편람 PDF 를 업로드할 때, the system **SHALL** 평가기준을 파싱·임베딩(768-dim)하여 pgvector 에 색인한다.

**Acceptance Criteria**:
- AC-PIPE-002-1: PDF 업로드 시 평가기준이 파싱·임베딩되어 `criteria` 테이블에 upsert 된다.
- AC-PIPE-002-2: 색인이 비어있고 criteria 파일이 제공되지 않은 경우 `ColdStartResponse` 가 반환된다.
- AC-PIPE-002-3: 한자→한글 정규화가 적용되면 `normalization_warning` JSONB 가 저장된다.

---

### REQ-PIPE-003 — Criteria Search API

**When** 사용자가 `GET /api/criteria/search?q={query}&top_k={k}` 로 한국어 질의를 요청할 때, the system **SHALL** top-k `CriterionMatch` 결과를 점수 내림차순으로 반환한다.

**Acceptance Criteria**:
- AC-PIPE-003-1: 한국어 질의에 대해 top-k 결과가 score 내림차순으로 반환된다.
- AC-PIPE-003-2: `top_k=3`, 50개 색인 criteria 기준 **p99 < 100ms** (REQ-AX-002 상속).
- AC-PIPE-003-3: 질의에 U+4E00-U+9FFF (CJK 한자) 가 포함되면 confidence 에 0.8× 페널티가 적용된다.
- AC-PIPE-003-4: 색인이 부트스트랩되지 않은 경우(`indexed_chunks=0`) `ColdStartResponse` 가 반환된다.

---

### REQ-PIPE-004 — Grade Simulation API

**When** 사용자가 `POST /api/simulations/predict` 로 A 등급/B 등급 벤치마크 보고서를 제출할 때, the system **SHALL** `GradeDistribution {p_a, p_b, p_abstain}` 을 반환하고 `simulations` 테이블에 기록한다.

**Acceptance Criteria**:
- AC-PIPE-004-1: A+B 벤치마크 입력 시 `GradeDistribution` 이 반환된다.
- AC-PIPE-004-2: `p_a + p_b + p_abstain = 1.0 ± 0.001` (확률 불변식).
- AC-PIPE-004-3: `max(p_a, p_b) < 0.5` 이면 `abstain=True` 가 반환된다.
- AC-PIPE-004-4: A 등급만 단일 입력된 경우(homogeneous) **HTTP 422** 가 반환된다.
- AC-PIPE-004-5: 시뮬레이션 결과가 `simulations` 테이블에 영속된다.

**Non-Functional**: 1-sec 예측 SLA (REQ-AX-003 상속).

---

### REQ-PIPE-005 — Report Draft API

**When** 사용자가 `POST /api/reports/generate` 로 criterion + customer_content 를 제출할 때, the system **SHALL** 한국어 합니다체 `DraftSection` 을 생성하여 반환한다.

**Acceptance Criteria**:
- AC-PIPE-005-1: criterion + content 입력 시 한국어 합니다체 `DraftSection` 이 반환된다.
- AC-PIPE-005-2: LLM 호출은 localhost endpoint 로만 발생한다 (REQ-UBI-001 allowlist 검증).
- AC-PIPE-005-3: `StyleApplier` 가 `style_violation` 을 반환하면 최대 3회 재시도된다.
- AC-PIPE-005-4: LLM endpoint 가 3회 재시도 후에도 도달 불가능하면 **HTTP 502** 가 반환된다.
- AC-PIPE-005-5: 생성된 텍스트는 최소 1개의 경어체 종결(`~습니다`, `~겠습니다`, `~합니다`) 을 포함한다.

**Non-Functional**: 5-sec 생성 SLA (REQ-AX-004 상속).

---

### REQ-PIPE-006 — Gap Recommendation API

**When** 사용자가 `POST /api/recommendations/generate` 로 현재 `GradeDistribution` 과 A 등급 벤치마크를 제출할 때, the system **SHALL** 3-5건의 `RankedSuggestion` 리스트를 반환한다.

**Acceptance Criteria**:
- AC-PIPE-006-1: 현재 분포 + A 등급 벤치마크 입력 시 3-5건 `RankedSuggestion` 이 반환된다.
- AC-PIPE-006-2: `priority_score = feasibility_score × expected_score_delta` 산식이 정확히 계산된다.
- AC-PIPE-006-3: A 등급 벤치마크가 공집합이면 빈 리스트가 반환된다 (fabrication guard).
- AC-PIPE-006-4: 추천 결과가 `recommendations` 테이블에 영속된다.
- AC-PIPE-006-5: `RankedSuggestion` 의 `content` 필드는 한국어로 반환된다 (REQ-UBI-002 Korean-first).

**Non-Functional**: 3-sec 추천 SLA (REQ-AX-005 상속).

---

### REQ-PIPE-007 — Recommendation Feedback API

**When** 사용자가 `PATCH /api/recommendations/{id}/feedback` 로 피드백을 제출할 때, the system **SHALL** 피드백을 `recommendations.feedback` JSONB 컬럼에 저장한다.

**Acceptance Criteria**:
- AC-PIPE-007-1: 피드백 payload 가 `recommendations.feedback` JSONB 에 저장된다.
- AC-PIPE-007-2: 존재하지 않는 `recommendation_id` 에 대해 **HTTP 404** 가 반환된다.
- AC-PIPE-007-3: 피드백 액션에 대해 `audit_log` 가 생성된다.

---

### REQ-PIPE-008 — Audit Logging (Ubiquitous)

The system **SHALL** 7개 엔드포인트 전부에 대해 매 요청마다 `audit_logs` 테이블에 감사 기록을 남긴다.

**Acceptance Criteria**:
- AC-PIPE-008-1: 7개 엔드포인트 호출 시 `audit_logs` 항목이 1건 이상 생성되며, sandbox 에서는 `user_id='cli-anonymous'` 가 사용된다.
- AC-PIPE-008-2: `audit_logs.details` JSONB 는 요청 메타데이터(`action`, `resource_id`, `file_type`, `endpoint`, `status_code`) 를 포함한다 (REQ-UBI-003: action + resource_id 필수).

---

### REQ-PIPE-009 — TDD Test Coverage (Ubiquitous)

The system **SHALL** 모든 신규/수정 모듈에 대해 단위·통합 테스트를 제공한다.

**Acceptance Criteria**:
- AC-PIPE-009-1: 다음 19개 public 메서드에 대해 각각 단위 테스트가 작성되며, 모듈당 분기 커버리지 **≥ 85%** 를 달성한다: `PDFParser.parse`, `HWPParser.parse`, `VLMProcessor.ocr`, `TableExtractor.extract`, `CriterionParser.parse`, `EmbeddingService.encode`, `VectorStore.upsert`, `VectorStore.query`, `Retriever.search`, `BenchmarkLearner.learn`, `GradePredictor.predict`, `ScenarioSimulator.simulate`, `LLMClient.generate`, `PromptBuilder.build`, `ReportDrafter.draft_section`, `StyleApplier.validate`, `GapAnalyzer.analyze`, `ContentSuggester.suggest`, `Prioritizer.prioritize`.
- AC-PIPE-009-2: 통합 테스트(testcontainers PostgreSQL + pgvector) 가 E2E 시나리오(upload → index → predict → generate → recommend) 를 커버한다.
- AC-PIPE-009-3: `pytest tests/ -v --cov=pipelines --cov-fail-under=85` 명령이 통과한다.

---

## 4. 제약사항 및 결정사항

| ID | 결정 | 근거 |
|----|------|------|
| **D1** | `VLMProcessor._run_inference()` 는 단위테스트에서는 stub 으로 유지. 통합테스트는 `ENABLE_VLM=true` 환경변수가 설정된 경우에만 실제 Qwen2-VL 모델을 사용한다 (기본: skip). | VLM 모델(수 GB) 로딩 비용/CI 환경 제약 |
| **D2** | `LLMClient._call_model()` 단위테스트는 `httpx_mock` 사용. 통합테스트는 `ENABLE_LLM=true` 가 설정된 경우에만 실제 endpoint 호출 (기본: skip). | LLM endpoint 의존성/CI 환경 제약 |
| **D3** | `BenchmarkLearner` 는 sklearn `TfidfVectorizer + LogisticRegression` 사용 (스켈레톤 그대로). 딥러닝 분류 모델 도입하지 않는다. | PoC 단순성, 추론 1s SLA 보장 |
| **D4** | `EmbeddingService.encode()` 단위테스트는 `SentenceTransformer._get_model()` 을 patch. 통합테스트는 실 모델 호출 (`ENABLE_EMBEDDING=true`, 기본 활성, CPU 에서 충분히 빠름). | ko-sroberta-multitask 는 CPU 친화적 (≈400MB) |
| **D5** | `TableExtractor._detect_cells()` 는 stub 으로 유지하고 빈 리스트를 반환한다. 컴퓨터비전 통합은 본 SPEC 범위 외(후속 SPEC). | PoC 우선순위 |
| **D6** | `settings.auth_enabled=False`. JWT 강제는 SPEC-AX-AUTH-002 로 위임한다. | sandbox 모드, REQ-UBI-003 'cli-anonymous' |
| **D7** | SPEC-AX-001 §2.1 의 Celery 워커(`ingestion_worker.py`, `generation_worker.py`, `simulation_worker.py`) 대신 FastAPI `BackgroundTasks` 를 사용한다. Celery + Redis 통합은 후속 SPEC 으로 분리. | PoC 단순성, Redis 의존성 회피 |

---

## 5. 제외 사항 (Exclusions — What NOT to Build)

본 SPEC 범위에서 명시적으로 제외되는 항목:

1. **Celery + Redis 워커 풀 통합** — D7 에 따라 `BackgroundTasks` 로 대체. 후속 SPEC.
2. **VLM / LLM 모델 파일 다운로드 워크플로우** — 모델은 `settings.model_path` 경로에 사전 존재한다고 가정.
3. **Multi-document 배치 업로드** — 단일 파일 업로드만 지원.
4. **C / D 등급 분류** — A / B 2-class + abstain 만 (REQ-AX-003).
5. **Console UI / Web UI** — REST API 만. UI 는 SPEC-AX-WEB-001 범위.
6. **JWT 강제 인증** — D6, SPEC-AX-AUTH-002 위임.
7. **`TableExtractor` 셀 검출 컴퓨터비전 로직** — D5, 후속 SPEC.
8. **외부 LLM (OpenAI, Anthropic 등) 호출** — REQ-UBI-001 위반. localhost only.
9. **운영 환경 모니터링/관측 가능성** — SPEC-AX-OBS-001 범위.
10. **국제화(영어/일본어 등 다국어 보고서 생성)** — REQ-UBI-002 (Korean-first).

---

## 6. TDD 구현 전략

### 6.1 Phase 분할

| Phase | 범위 | 우선순위 |
|-------|------|---------|
| **Phase A** | Ingestion 단위 테스트 RED→GREEN (PDFParser, HWPParser w/ VLM mock, LanguageDetector) | High |
| **Phase B** | Mapping 단위 테스트 RED→GREEN (CriterionParser, EmbeddingService w/ ST mock, Retriever w/ FakeVectorStore) | High |
| **Phase C** | Scoring 단위 테스트 RED→GREEN (BenchmarkLearner, GradePredictor 확률 산식, ScenarioSimulator) | High |
| **Phase D** | Generation 단위 테스트 RED→GREEN (LLMClient w/ httpx mock, ReportDrafter 재시도, StyleApplier) | High |
| **Phase E** | Recommendation 단위 테스트 RED→GREEN (GapAnalyzer fabrication guard, ContentSuggester, Prioritizer 산식) | Medium |
| **Phase F** | 통합 테스트 (testcontainers) + FastAPI 엔드포인트 테스트 (TestClient) | High |
| **Phase G** | `config/models.py` Sprint 2-6 Pydantic 모델 + `main.py` 7개 엔드포인트 wiring | High |

### 6.2 테스트 파일 구조

```
tests/
├── conftest.py                              # 공용 fixtures: fake_db, fake_vector_store, sample_hwp_bytes, sample_pdf_bytes
├── unit/
│   ├── ingestion/
│   │   ├── test_pdf_parser.py
│   │   ├── test_hwp_parser.py
│   │   ├── test_vlm_processor.py
│   │   └── test_language_detector.py
│   ├── mapping/
│   │   ├── test_criterion_parser.py
│   │   ├── test_embedding_service.py
│   │   ├── test_vector_store.py
│   │   └── test_retriever.py
│   ├── scoring/
│   │   ├── test_benchmark_learner.py
│   │   ├── test_grade_predictor.py
│   │   └── test_scenario_simulator.py
│   ├── generation/
│   │   ├── test_llm_client.py
│   │   ├── test_report_drafter.py
│   │   └── test_style_applier.py
│   └── recommendation/
│       ├── test_gap_analyzer.py
│       ├── test_content_suggester.py
│       └── test_prioritizer.py
└── integration/
    └── test_pipeline_e2e.py                 # testcontainers PostgreSQL + pgvector
```

### 6.3 TDD 사이클

각 Phase 는 RED → GREEN → REFACTOR 순서로 진행:

1. **RED**: 실패하는 테스트부터 작성 (AC 1건당 ≥1 테스트).
2. **GREEN**: 최소 코드로 테스트 통과.
3. **REFACTOR**: 가독성·중복 제거 (TRUST 5 Readable/Unified).

각 Phase 종료 시 `pytest tests/unit/{domain}/ -v --cov=pipelines/{domain} --cov-fail-under=85` 통과 필수.

Phase F 종료 시 `pytest tests/ -v --cov=pipelines --cov-fail-under=85` 통과 필수 (AC-PIPE-009-3).

### 6.4 검증 매트릭스 (요구사항 ↔ 테스트)

| REQ | 단위 테스트 위치 | 통합 테스트 검증 |
|-----|----------------|-----------------|
| REQ-PIPE-001 | `tests/unit/ingestion/` | E2E upload step |
| REQ-PIPE-002 | `tests/unit/mapping/test_criterion_parser.py`, `test_vector_store.py` | E2E index step |
| REQ-PIPE-003 | `tests/unit/mapping/test_retriever.py` | E2E search + p99 측정 |
| REQ-PIPE-004 | `tests/unit/scoring/` | E2E predict step |
| REQ-PIPE-005 | `tests/unit/generation/` | E2E generate step (D2 ENABLE_LLM) |
| REQ-PIPE-006 | `tests/unit/recommendation/` | E2E recommend step |
| REQ-PIPE-007 | `tests/integration/test_pipeline_e2e.py` | feedback step |
| REQ-PIPE-008 | (각 엔드포인트 테스트) | audit_logs 7건 누적 검증 |
| REQ-PIPE-009 | (전체) | coverage 85% gate |
