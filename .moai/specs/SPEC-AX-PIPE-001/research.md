# SPEC-AX-PIPE-001 — Research

본 문서는 Python AI 파이프라인 SPEC 작성을 위한 사실 검증(C2 Validation) 결과입니다. 부모 SPEC-AX-001 의 SLA·REQ-UBI 제약을 상속하며, 본 SPEC 은 기존 스켈레톤에 **실제 로직 + REST API + TDD 테스트 스위트** 를 채워 넣는 것을 목적으로 합니다.

---

## 1. 현재 존재하는 것 (Skeleton Inventory)

### 1.1 Ingestion (REQ-AX-001)

| 파일 | 클래스/메서드 | 상태 |
|------|--------------|------|
| `pipelines/ingestion/pdf_parser.py` | `PDFParser.parse()` | stub — `_load_pdf()` 실 로직 부재 |
| `pipelines/ingestion/hwp_parser.py` | `HWPParser.parse()` | stub — VLM OCR 폴백 스켈레톤만 존재 |
| `pipelines/ingestion/vlm_processor.py` | `VLMProcessor.ocr()` | stub — `_run_inference()` 가 MagicMock 반환 |
| `pipelines/ingestion/table_extractor.py` | `TableExtractor.extract()` | stub — `_detect_cells()` 가 `[]` 반환 |
| `pipelines/ingestion/language_detector.py` | `LanguageDetector.detect()` | **FUNCTIONAL** — 한글 U+AC00-U+D7AF 비율 > 20% 임계값 |

### 1.2 Mapping (REQ-AX-002)

| 파일 | 클래스/메서드 | 상태 |
|------|--------------|------|
| `pipelines/mapping/criterion_parser.py` | `CriterionParser.parse()` | stub — 더미 3-레벨 계층 반환 |
| `pipelines/mapping/embedding_service.py` | `EmbeddingService.encode()` | lazy-load `ko-sroberta-multitask` (768-dim), 실 호출 미구현 |
| `pipelines/mapping/vector_store.py` | `VectorStore.upsert/query()` | psycopg2 + pgvector, `FakeVectorStore` (in-memory) 단위테스트용 존재 |
| `pipelines/mapping/retriever.py` | `Retriever.search()` | EmbeddingService + VectorStore 조합, `ColdStartResponse` 처리, 한자 페널티 0.8× |

### 1.3 Scoring (REQ-AX-003)

| 파일 | 클래스/메서드 | 상태 |
|------|--------------|------|
| `pipelines/scoring/benchmark_learner.py` | `BenchmarkLearner.learn()` | TF-IDF + LogisticRegression 상태 머신 (idle→training→ready) |
| `pipelines/scoring/grade_predictor.py` | `GradePredictor.predict()` | 2-class softmax + abstain 3-way, `p_a + p_b + p_abstain = 1.0±0.001` |
| `pipelines/scoring/scenario_simulator.py` | `ScenarioSimulator.simulate()` | `coef_` 인트로스펙션 기반 B→A content change |

### 1.4 Generation (REQ-AX-004)

| 파일 | 클래스/메서드 | 상태 |
|------|--------------|------|
| `pipelines/generation/llm_client.py` | `LLMClient.generate()` | EXAONE 3.5→Qwen 2.5 폴백, localhost allowlist (REQ-UBI-001), `_call_model()` 가 `NotImplementedError` |
| `pipelines/generation/prompt_builder.py` | `PromptBuilder.build()` | **FUNCTIONAL** — 합니다체 prompt template |
| `pipelines/generation/report_drafter.py` | `ReportDrafter.draft_section()` | retry 로직 (max 3), 스타일 검증 |
| `pipelines/generation/style_applier.py` | `StyleApplier.validate()` | **FUNCTIONAL** — 정규식 기반 한국어 경어체 검출 |

### 1.5 Recommendation (REQ-AX-005)

| 파일 | 클래스/메서드 | 상태 |
|------|--------------|------|
| `pipelines/recommendation/gap_analyzer.py` | `GapAnalyzer.analyze()` | fabrication guard (벤치마크 공집합 → `[]`) |
| `pipelines/recommendation/content_suggester.py` | `ContentSuggester.suggest()` | Retriever 사용 |
| `pipelines/recommendation/prioritizer.py` | `Prioritizer.prioritize()` | `priority_score = feasibility × score_delta`, top_k |

### 1.6 Config / Auth / Main

- `pipelines/config/settings.py` — Pydantic BaseSettings (DSN, Redis URL, model paths, `auth_enabled=False`)
- `pipelines/config/models.py` — Enums + `HealthResponse`, `WorkflowResponse`; Sprint 2-6 요청/응답 모델 부재
- `pipelines/auth/validator.py` — `TokenValidator` JWT 구조 체크, RS256/ES256/EdDSA only, JWKS deferred
- `pipelines/main.py` — `GET /health` 1개 엔드포인트만 존재, Sprint 2-6 엔드포인트 TODO

### 1.7 Database Schema (initial.sql)

전 7개 테이블 존재 확인: `documents`, `criteria`, `reports`, `workflows`, `simulations`, `recommendations`, `audit_logs`.

---

## 2. 누락된 것 (Gap Analysis)

| 영역 | 누락 항목 |
|------|----------|
| Real Logic | `_load_pdf()`, `_load_hwp()`, `_run_inference()`, `_detect_cells()`, `CriterionParser.parse()` 실 파싱, `EmbeddingService.encode()` 실 모델 호출, `LLMClient._call_model()` httpx 호출 |
| REST API | 7개 엔드포인트 (upload, criteria index, criteria search, simulations predict, reports generate, recommendations generate/feedback) |
| Pydantic Models | Sprint 2-6 요청/응답 (`UploadResponse`, `CriterionMatch`, `GradeDistribution`, `DraftSection`, `RankedSuggestion`, `FeedbackRequest` 등) |
| Tests | 단위 테스트 17 모듈 + 통합 테스트 (testcontainers PostgreSQL+pgvector) |
| Audit Logging | 7개 엔드포인트 전부 `audit_logs` insert (REQ-UBI-003) |

---

## 3. 구현 리스크

| ID | 리스크 | 완화 전략 |
|----|--------|----------|
| R1 | VLM 모델(Qwen2-VL) 로딩 비용/메모리 | `ENABLE_VLM=true` 환경변수로 통합테스트만 실제 모델 사용, 기본은 skip |
| R2 | LLM endpoint (localhost:8080) 의존 | `ENABLE_LLM=true` 로 토글, 단위테스트는 `httpx_mock` |
| R3 | pgvector HNSW 인덱스 / p99<100ms SLA | testcontainers 에 `pgvector` 이미지 + 50 criteria seed 로 측정 |
| R4 | `SentenceTransformer` 로딩 (~400MB) | 단위테스트는 `_get_model()` patch, 통합테스트는 실모델 (CPU 도 빠름) |
| R5 | sklearn `LogisticRegression` 학습 비결정성 | `random_state` 고정, 동일 입력 → 동일 출력 보장 |
| R6 | OLE 손상 HWP 파일 분기 (VLM fallback trigger) | `pyhwp` 예외 클래스 mock 하여 fallback 경로 검증 |
| R7 | Celery + Redis 워커 통합 부담 | **D7**: PoC 는 FastAPI `BackgroundTasks` 로 대체, Celery 통합은 후속 SPEC |

---

## 4. TDD 전략

### 4.1 단위 테스트 (mock-friendly)

| 모듈 | Mock 대상 | 검증 항목 |
|------|----------|----------|
| PDFParser | `pypdf.PdfReader` | parsed_text 추출, 페이지 수 |
| HWPParser | `pyhwp`, `VLMProcessor` | OLE 파싱 + VLM 폴백 트리거 |
| VLMProcessor | `transformers` model | `_run_inference()` 호출 횟수 |
| TableExtractor | (stub 유지) | 빈 리스트 반환 보장 (D5) |
| LanguageDetector | (없음) | 한글 비율 ≥20%/<20% 분기 |
| CriterionParser | `pypdf` | 3-레벨 계층 구조 |
| EmbeddingService | `SentenceTransformer._get_model` | 768-dim 벡터 shape |
| VectorStore | (FakeVectorStore 사용) | upsert→query 라운드트립 |
| Retriever | EmbeddingService + FakeVectorStore | cold-start, 한자 페널티 0.8× |
| BenchmarkLearner | (없음, 실 sklearn) | 상태머신 idle→ready, random_state 고정 |
| GradePredictor | BenchmarkLearner | p_a+p_b+p_abstain 불변식, max<0.5 → abstain |
| ScenarioSimulator | LogisticRegression | coef_ 기반 차이 산출 |
| LLMClient | `httpx.AsyncClient` (httpx_mock) | localhost allowlist, EXAONE→Qwen 폴백 |
| ReportDrafter | LLMClient + StyleApplier | 3회 retry 한계 |
| StyleApplier | (없음) | 합니다체 검출 정확도 |
| GapAnalyzer | (없음) | fabrication guard (empty input → []) |
| ContentSuggester | Retriever | top-k 제안 |
| Prioritizer | (없음) | priority_score 산식 |

### 4.2 통합 테스트 (testcontainers)

- `testcontainers.postgres.PostgresContainer` + `pgvector/pgvector:pg16` 이미지
- `initial.sql` 적용 후 7개 테이블 확인
- E2E 시나리오: upload(HWP)→index(criteria PDF)→search→predict→generate→recommend→feedback
- audit_logs 7건 누적 검증

### 4.3 FastAPI 엔드포인트 테스트

- `fastapi.testclient.TestClient` 로 7개 엔드포인트 호출
- 정상/400/404/422/502 분기 모두 커버
- `BackgroundTasks` 는 `asyncio.gather` 대체로 동기 검증

---

## 5. 부모 SPEC 상속 (SPEC-AX-001)

- REQ-AX-001 30s 인제스션 SLA → upload API 응답 30s 이내
- REQ-AX-002 top-k p99 < 100ms → 통합테스트에서 50 criteria seed + 100 query 측정
- REQ-AX-003 1s 예측 SLA → predict API 응답 1s 이내
- REQ-AX-004 5s 생성 SLA + 3 retry → reports/generate 5s 이내
- REQ-AX-005 3s 추천 SLA + 3-5건 → recommendations/generate 3s 이내
- REQ-UBI-001 localhost LLM only → LLMClient allowlist 단위테스트로 검증
- REQ-UBI-002 Korean-first → StyleApplier 합니다체 강제
- REQ-UBI-003 audit logging → 7개 엔드포인트 전부 audit_logs insert
