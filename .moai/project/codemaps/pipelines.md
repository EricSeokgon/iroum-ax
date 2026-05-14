# Python 파이프라인 모듈 맵 (pipelines)

5개 도메인 영역 · 17개 핵심 모듈 · REQ-AX 매핑

## 도메인 구조

```
pipelines/
├── ingestion/      (5개 모듈) — Document Ingestion (REQ-AX-001)
├── mapping/        (4개 모듈) — Criterion Mapping & RAG (REQ-AX-002)
├── scoring/        (3개 모듈) — Grade Simulation (REQ-AX-003)
├── generation/     (4개 모듈) — Report Draft Generation (REQ-AX-004)
├── recommendation/ (3개 모듈) — Gap Recommendation (REQ-AX-005)
├── workers/        (2개) — Celery 비동기 워커
├── config/         (3개) — 설정 & 데이터 모델
└── main.py         (FastAPI 진입점)
```

## 도메인별 모듈 맵

### 1. Ingestion (Document Ingestion — REQ-AX-001)

| 모듈 | 책임 | 진입점 | fan_in | 주요 함수 |
|------|------|--------|--------|----------|
| **hwp_parser.py** | HWP OLE 구조 파싱, 텍스트·표·메타데이터 추출 | CLI: parse_hwp(path) | 1 | HWPDocument.extract_text(), extract_tables() |
| **pdf_parser.py** | PDF 텍스트 추출 + 회전 페이지 처리 | CLI: parse_pdf(path) | 1 | PDFDocument.extract_text(), get_page_orientation() |
| **vlm_processor.py** | Qwen2-VL 7B 호출, OCR 결과 정규화 | FastAPI: /api/documents/ocr (Celery) | 1 | process_ocr(image_path, model) — @MX:NOTE: GPU 메모리 공유 |
| **table_extractor.py** | VLM 출력 후처리, 셀 정렬 검증 | CLI/Worker: extract_table_structure(vlm_output) | 2 (vlm_processor, retriever) | align_cells(), validate_schema() |
| **language_detector.py** | 언어 감지 (한국어 enforcement) | CLI: detect_language(text) | 2 (hwp_parser, pdf_parser) | is_korean_text() — @MX:NOTE: REQ-UBI-002 규제 준수 |

### 2. Mapping (Criterion Mapping & RAG — REQ-AX-002)

| 모듈 | 책임 | 진입점 | fan_in | 주요 함수 |
|------|------|--------|--------|----------|
| **criterion_parser.py** | 평가편람 파싱, 항목→지표→배점 계층 추출 | CLI: parse_criterion_handbook(pdf_path) | 1 | parse_hierarchy(), extract_leaf_criteria() |
| **embedding_service.py** | ko-sroberta-multitask 임베딩 | CLI: embed_text(text, model_name) | 3 (criterion_parser, vector_store, retriever) — @MX:ANCHOR | HuggingFaceEmbedding.embed() |
| **vector_store.py** | pgvector HNSW 인덱싱·upsert | FastAPI: /api/criteria/index (Worker) | 2 (embedding_service, retriever) | index_vectors(), upsert_batch() — @MX:ANCHOR |
| **retriever.py** | top-k 검색, 재순위화 (선택) | FastAPI: /api/criteria/search?q={query} | 4 (scoring, generation, recommendation, table_extractor) — @MX:ANCHOR | retrieve(query, top_k=3), rerank() |

### 3. Scoring (Grade Simulation — REQ-AX-003)

| 모듈 | 책임 | 진입점 | fan_in | 주요 함수 |
|------|------|--------|--------|----------|
| **benchmark_learner.py** | A/B 등급 보고서 특징 추출 (학습 데이터) | CLI: learn_from_benchmarks(benchmark_dir) | 1 | fit(reports_by_grade), extract_features() |
| **grade_predictor.py** | 자사 보고서 등급 확률 산출 (2-class softmax + abstain) | FastAPI: /api/simulations/predict (Worker) | 2 (benchmark_learner, scenario_simulator) — @MX:ANCHOR | predict_probabilities(report_text) → {A%, B%, abstain%} |
| **scenario_simulator.py** | B→A 점수 시나리오 시뮬레이션 | FastAPI: /api/simulations/scenarios (Worker) | 1 | simulate_scenarios(current_report, target_grade, benchmark_data) |

### 4. Generation (Report Draft Generation — REQ-AX-004)

| 모듈 | 책임 | 진입점 | fan_in | 주요 함수 |
|------|------|--------|--------|----------|
| **llm_client.py** | EXAONE 3.5 7B 호출 + Qwen 2.5 fallback | CLI: call_llm(prompt, model="exaone") | 1 | generate_text(prompt, max_tokens, temperature) — @MX:NOTE: Fallback 전략 |
| **report_drafter.py** | 보고서 초안 생성 오케스트레이션 | FastAPI: /api/reports/generate (Worker) | 1 | draft_report_section(criterion, content_context) |
| **prompt_builder.py** | 평가지표별 프롬프트 템플릿 | CLI: build_prompt(criterion, instruction, examples) | 2 (llm_client, style_applier) | generate_prompt(), inject_context() |
| **style_applier.py** | 한국어 공문 합니다체 검증·재생성 | FastAPI: /api/reports/validate-style | 1 | validate_korean_style(text), enforce_formal_tone() — @MX:ANCHOR |

### 5. Recommendation (Gap Recommendation — REQ-AX-005)

| 모듈 | 책임 | 진입점 | fan_in | 주요 함수 |
|------|------|--------|--------|----------|
| **gap_analyzer.py** | 현재 vs 목표 등급 콘텐츠 Gap 분석 | FastAPI: /api/recommendations/analyze (Worker) | 1 | analyze_gap(current_report, target_grade, benchmark_content) |
| **content_suggester.py** | 벤치마크 기반 콘텐츠 제안 | CLI: suggest_content(gap_analysis, benchmark_index) | 1 | extract_improvement_items(), match_similar_content() |
| **prioritizer.py** | 실현 가능성 우선순위 정렬 | FastAPI: /api/recommendations/prioritize | 1 | rank_by_feasibility(suggestions), estimate_impact() |

### 6. Workers (Celery 비동기 처리)

| 모듈 | 책임 | 타스크 명 | Queue |
|------|------|----------|-------|
| **ingestion_worker.py** | Document Ingestion 비동기 워커 | `tasks.process_document` | celery.ingestion |
| **generation_worker.py** | Report Draft Generation 비동기 워커 | `tasks.generate_draft` | celery.generation |
| **simulation_worker.py** | Grade Simulation 비동기 워커 | `tasks.predict_grade` | celery.scoring |

### 7. Config (설정 & 데이터 모델)

| 모듈 | 책임 | 주요 구성 |
|------|------|----------|
| **settings.py** | 환경 설정 | DATABASE_URL, VLLM_ENDPOINT, HF_MODEL_NAME, EMBEDDING_DIM=768 |
| **models.py** | Pydantic 데이터 모델 | Document, Criterion, Report, Simulation, Recommendation (pkg/models/와 동일) |
| **logging.py** | 로깅 설정 | JSON 구조화 로깅, audit_log 별도 채널 |

## 모듈 간 의존성 그래프

```
hwp_parser ──┐
             ├─→ language_detector
pdf_parser ──┤
             └─→ table_extractor ──→ retriever
                 
criterion_parser ──→ embedding_service ──→ vector_store
                                           ↓
                                        retriever
                                           ↑
generation ←─────────────────────────────┘
   ↓
llm_client ──→ style_applier
   ↓
report_drafter

scoring:
  benchmark_learner ──→ grade_predictor ──→ scenario_simulator

recommendation:
  gap_analyzer ──→ content_suggester ──→ prioritizer
                       ↓
                    retriever (벤치마크 검색)
```

## @MX:ANCHOR 위치 (High Fan-In 함수)

| 함수 | 위치 | fan_in | 이유 |
|------|------|--------|------|
| `embedding_service.embed()` | embedding_service.py | 3 | 모든 RAG 파이프라인이 임베딩 필요 |
| `vector_store.{index, upsert}()` | vector_store.py | 2 | 인덱싱·검색 동시 의존 |
| `retriever.retrieve()` | retriever.py | 4 | generation·scoring·recommendation 모두 참조 |
| `grade_predictor.predict()` | grade_predictor.py | 2 | simulation·recommendation 타겟 |
| `style_applier.validate()` | style_applier.py | 1 | report_drafter 품질 보증 |

변경 시 단위 테스트 강화 필수.

---

**최종 업데이트**: 2026-05-14 (Sprint 6 완료)  
**Test Coverage**: 177 unit tests passing (pipelines/ 모듈별)  
**Acceptance**: REQ-AX-001~005 + REQ-UBI 완료
