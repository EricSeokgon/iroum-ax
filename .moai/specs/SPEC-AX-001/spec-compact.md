# SPEC-AX-001 (Compact) — 안전보건 PoC Walking Skeleton

> Run phase 로딩용 압축본. 개요·기술 접근 설명·research 인용 등 서술 부분 제거.
> 포함: REQ-AX-XXX 리스트, Given/When/Then 시나리오, 영향받는 파일, Exclusions.

- id: SPEC-AX-001
- version: 0.1.2
- status: draft
- priority: high
- iteration: 3 (evaluator-active cross-validation 후속, 총 24개 AC)

---

## EARS Requirements

### Ubiquitous
- REQ-UBI-001: The system SHALL store all input documents, intermediate artifacts, and output reports exclusively within the customer-controlled infrastructure. External LLM API calls are prohibited.
- REQ-UBI-002: The system SHALL process Korean text as the primary language.
- REQ-UBI-003: The system SHALL record an audit log entry for every document upload, workflow creation, draft generation, and prediction event. WHILE 인증 시스템(SSO/JWT)이 비활성화(Exclusion §12 sandbox)일 때, audit_logs.user_id 필드에 'cli-anonymous' 기본값을 기록한다.

### REQ-AX-001 Document Ingestion
- REQ-AX-001-E1 (Event): WHEN a user uploads a HWP or PDF file via POST /api/documents/upload, THEN the system SHALL parse the file, extract text·tables·metadata, persist a documents record, and return a document_id within 30 seconds for a reference 50-page HWP.
- REQ-AX-001-S1 (State): WHILE the VLM is processing OCR for a single page, THE system SHALL not accept additional OCR requests for that document and SHALL queue concurrent requests through Celery.
- REQ-AX-001-O1 (Optional): WHERE a GPU is available, THE system SHALL use vLLM-accelerated Qwen2-VL 7B inference targeting <2 seconds per page; WHERE GPU is unavailable, THE system MAY fall back to CPU inference with degraded latency.
- REQ-AX-001-U1 (Unwanted): IF hwp-converter detects OLE structure corruption, THEN the system SHALL automatically invoke VLM OCR fallback, mark status `ocr_fallback`, and continue without user intervention.
- REQ-AX-001-U2 (Unwanted): IF both hwp-converter and VLM OCR fail, THEN the system SHALL persist status `parse_failed`, return HTTP 422, and SHALL NOT silently drop the input.

### REQ-AX-002 Criterion Mapping
- REQ-AX-002-E1 (Event): WHEN 평가편람 PDF is uploaded for indexing, THEN the system SHALL parse 항목→지표→배점 hierarchy, chunk leaf criteria into 500-1000 token segments, compute embeddings via ko-sroberta-multitask, and persist them into pgvector HNSW.
- REQ-AX-002-E2 (Event): WHEN GET /api/criteria/search?q={query} is issued, THEN the system SHALL return top-3 results with p99 latency < 100 milliseconds.
- REQ-AX-002-S1 (State): WHILE the pgvector HNSW index is being constructed or rebuilt, THE system SHALL queue retrieval requests and SHALL NOT serve stale or partial index results.
- REQ-AX-002-U1 (Unwanted): IF fewer than 3 results above relevance 0.7 are retrieved, THEN the system SHALL return `insufficient_context` with the candidates and flag the query for manual review.

### REQ-AX-003 Grade Simulation
- REQ-AX-003-E1 (Event): WHEN 자사 보고서가 grade_predictor에 입력되면, the system SHALL 2-class softmax (A vs B) 결과에 abstain 분기(max(p_a, p_b) < 0.5일 때 활성화)를 포함하여 `{A, B, abstain}` 3-way 분포 (sum=1.0 ± 0.001) + abstain 플래그를 1초 이내에 반환한다.
- REQ-AX-003-S1 (State): WHILE benchmark_learner is training, THE system SHALL reject predictions with HTTP 503 `model_training`.
- REQ-AX-003-U1 (Unwanted): IF abstain 플래그가 활성화되면(max(p_a, p_b) < 0.5), THEN the system SHALL mark `low_confidence`, return candidate probabilities including abstain, and request human verification.

### REQ-AX-004 Report Draft Generation
- REQ-AX-004-E1 (Event): WHEN a draft request for a single evaluation indicator arrives, THEN the system SHALL invoke EXAONE 3.5 7B, apply Korean 공문 합니다체 style, validate via style_applier, and return within 5 seconds.
- REQ-AX-004-O1 (Optional): WHERE EXAONE 3.5 is reachable, THE system SHALL use it as primary; WHERE EXAONE fails 3 consecutive attempts, THE system SHALL fall back to Qwen 2.5 7B.
- REQ-AX-004-U1 (Unwanted): IF the draft contains 반말, mixed honorifics, or violates 공문 style rules, THEN the system SHALL discard, re-prompt with style reinforcement up to 3 retries, and return `style_violation` if all retries fail.
- REQ-AX-004-U2 (Unwanted): IF EXAONE fallback to Qwen 2.5 is also exhausted, THEN the system SHALL persist failure, return HTTP 503, and SHALL NOT escalate to any external LLM API.

### REQ-AX-005 Gap Recommendation
- REQ-AX-005-E1 (Event): WHEN a workflow has produced a draft, current B grade, and target A grade, THEN the system SHALL run gap analysis and return 3-5 prioritized recommendation items with expected score delta within 3 seconds.
- REQ-AX-005-S1 (State): WHILE A 등급 benchmark content has not been indexed for the requested criterion, THE system SHALL return empty list with `benchmark_not_available` and SHALL NOT fabricate suggestions.
- REQ-AX-005-U1 (Unwanted): IF a customer marks a recommendation as `not_feasible`, THEN the system SHALL downgrade its priority, propose an alternative, and persist the feedback.

---

## Acceptance Scenarios (Given/When/Then)

### AC-UBI-001 (Ubiquitous, Edge: external API blocked)
- Given: pipelines/generation/llm_client.py uses LLM_ENDPOINT env var.
- When: LLM_ENDPOINT=https://api.openai.com/v1/chat/completions is injected.
- Then: System rejects external domain, raises ExternalLLMBlockedError, audit_logs records `external_llm_blocked`.

### AC-UBI-002 (Ubiquitous, Edge: Korean primary language)
- Given: HWP with 90% Korean + 10% English abbreviations (KOSHA/ISO).
- When: Upload + RAG search + draft generation.
- Then: documents.language="ko"; ko-sroberta-multitask embedding; 합니다체 output. If Korean ratio < 20%: parse_quality_flag="low_korean_ratio" + audit_logs `language_warning`. No rejection.

### AC-UBI-003 (Ubiquitous, Happy: 4 audit events recorded)
- Given: Empty audit_logs; full E2E workflow on 1 document.
- When: 4 actions executed: document_upload, workflow_create, draft_generate, prediction.
- Then: audit_logs INSERT exactly 4 rows; each row has user_id + action + resource_id + timestamp (ISO 8601 with tz). 0 missing fields.

### AC-UBI-004 (Ubiquitous, Happy: sandbox user_id default)
- Given: AUTH_ENABLED=false (Exclusion §12 sandbox), no auth headers.
- When: 4 actions (document_upload, workflow_create, draft_generate, prediction) executed without auth context.
- Then: All audit_logs rows have user_id='cli-anonymous' (no NULL/empty). Other fields meet AC-UBI-003 completeness.

### AC-001-1 (Happy)
- Given: KEPCO E&C HWP (50 pages), hwp-converter, Qwen2-VL 7B vLLM running.
- When: POST /api/documents/upload with the HWP.
- Then: Response within 30s; documents record created; parse accuracy ≥ 95%.

### AC-001-2 (Edge: OLE corruption)
- Given: HWP with corrupted header (hwp-converter OLECompoundError).
- When: Upload to same endpoint.
- Then: Auto-fallback to VLM OCR; status=`ocr_fallback`; if OCR also fails, status=`parse_failed` + HTTP 422.

### AC-001-3 (Edge: rotated PDF — OCR quality only)
- Given: PDF with 90° rotated table page.
- When: Upload and Qwen2-VL OCR runs.
- Then: Cells extracted in logical row/column order; rotation metadata recorded; cell accuracy ≥ 90%. (Verifies REQ-AX-001-E1; GPU/CPU branching verified in AC-001-5.)

### AC-001-4 (Edge: same-document OCR concurrency queued)
- Given: doc-101 OCR in progress; Celery queue has 1 task for doc-101.
- When: Second OCR request for doc-101 is issued.
- Then: Either (a) queued via Celery → HTTP 202 + {status:"queued"}, or (b) HTTP 409 {status:"ocr_already_in_progress"}. No concurrent inference on the same page. audit_logs records `ocr_concurrent_request_queued`.

### AC-001-5 (Edge: GPU vs CPU branching)
- Given: env-A with CUDA_VISIBLE_DEVICES=0 (1 GPU); env-B with CUDA_VISIBLE_DEVICES="" (no GPU).
- When: POST /api/documents/upload on same HWP in both envs.
- Then: env-A picks vllm_gpu → p99 < 2s/page + meta {inference_backend:"vllm_gpu"}. env-B picks transformers_cpu → p99 < 20s/page (5-10× relaxed per tech.md §6.1) + meta {inference_backend:"transformers_cpu"}. README documents the relaxed budget.

### AC-002-1 (Happy)
- Given: 안전보건 평가편람 indexed in pgvector HNSW.
- When: GET /api/criteria/search?q=안전교육 이수율 평가기준.
- Then: top-3 within 100ms p99; avg relevance ≥ 0.8; hierarchy 항목→지표→배점 included.

### AC-002-2 (Edge: insufficient context)
- Given: No chunks for "재해 통계 분석 방법론" in index.
- When: Search with that query.
- Then: status=`insufficient_context`; 0-2 candidates returned; query flagged for review.

### AC-002-3 (Happy: hierarchy preservation)
- Given: "안전보건 → 안전교육 → 5점" leaf criterion.
- When: criterion_parser parses the PDF.
- Then: parent_criterion_id correctly linked; max_points=5; 한자/한글 mixed text normalized before embedding.

### AC-002-4 (Edge: HNSW rebuild blocks retrieval)
- Given: pgvector HNSW in `reindex_status: rebuilding` (partial index).
- When: GET /api/criteria/search?q=안전교육.
- Then: No stale/partial results served. Either (a) internal queue → respond after rebuild (ETA < 10s), or (b) HTTP 503 + {status:"index_rebuilding", retry_after_seconds}. audit_logs `retrieval_blocked_during_reindex`.

### AC-002-5 (Edge: hanja/hangul normalization failure graceful degradation)
- Given: 평가편람 leaf criterion contains rare hanja not in hanja→hangul mapping.
- When: criterion_parser normalizes and embeds.
- Then: (a) Unresolved tokens kept as-is (raw fallback) into embedding; (b) chunk metadata `normalization_warning:"unresolved_hanja", unresolved_chars:[...]`; (c) search confidence reweighted ×0.8. No crash, no HTTP 500. audit_logs `embedding_normalization_warning`.

### AC-002-6 (Edge: RAG cold-start — empty index)
- Given: criteria table empty (COUNT=0); HNSW index created but no data.
- When: GET /api/criteria/search?q=<any>.
- Then: HTTP 503 or HTTP 200 + {status:"index_not_bootstrapped", indexed_chunks:0, next_step:"POST /api/criteria/index ..."}. No silent empty `[]`, no HTTP 500. Distinct from AC-002-2 (insufficient_context = has data but below threshold).

### AC-003-1 (Happy)
- Given: A grade + B grade benchmark reports trained; KEPCO E&C report.
- When: POST /api/simulations/predict.
- Then: Response within 1s; probabilities sum to 1.0 ± 0.001; prediction recorded.

### AC-003-2 (Edge: low confidence — both probs < 0.5, D6 reconciled)
- Given: Borderline report; flat prediction distribution.
- When: Predictor outputs {A: 0.42, B: 0.45, abstain: 0.13} (max(p_a, p_b) < 0.5).
- Then: status=`low_confidence`; response body {prediction: null, candidates: {A:0.42, B:0.45}, reason:"both_below_0.5", recommend_human_review: true}. simulations row prediction=NULL. Downstream gap recommendation blocked. (No 0.55 threshold introduced; near-tie majority deferred to SPEC-AX-002.)

### AC-003-3 (Edge: training state)
- Given: benchmark_learner in `state: training`.
- When: Prediction endpoint called.
- Then: HTTP 503 with `model_training`; no stale prediction served.

### AC-004-1 (Happy)
- Given: 안전교육 이수율 indicator, 5 retrieved chunks, EXAONE 3.5 7B available.
- When: POST /api/reports/generate.
- Then: Response within 5s; output in 합니다체; style_applier passes; subjective rating ≥ 4/5.

### AC-004-2 (Edge: style violation → retry)
- Given: LLM outputs mixed 합니다체 + 해체.
- When: style_applier validates.
- Then: Discard and re-prompt up to 3 times; if all fail, status=`style_violation`.

### AC-004-3 (Edge: EXAONE → Qwen 2.5 fallback)
- Given: EXAONE endpoint times out/5xx for 3 consecutive attempts.
- When: llm_client.generate() called.
- Then: Auto-switch to Qwen 2.5 7B with same prompt; response metadata `model_used: qwen2.5-7b`; no external API escalation.

### AC-005-1 (Happy)
- Given: Workflow with B prediction, target A, A benchmark indexed.
- When: POST /api/recommendations/generate.
- Then: Response within 3s; 3-5 items with {content, expected_score_delta, feasibility_score, source_benchmark_id}; sorted by feasibility_score desc.

### AC-005-2 (Edge: benchmark missing)
- Given: No A grade benchmark for 안전교육 indicator.
- When: Same endpoint called.
- Then: Empty list + status=`benchmark_not_available`; no fabricated content.

### AC-005-3 (Edge: not_feasible feedback)
- Given: User marks rec-123 as not_feasible.
- When: POST /api/recommendations/rec-123/feedback then subsequent recommendation call.
- Then: rec-123 priority downgraded; alternative proposed if available; feedback persisted.

---

## Affected Files (from structure.md §2 directory tree)

### Python Pipelines
- pipelines/ingestion/hwp_parser.py
- pipelines/ingestion/pdf_parser.py
- pipelines/ingestion/vlm_processor.py
- pipelines/ingestion/table_extractor.py
- pipelines/mapping/criterion_parser.py
- pipelines/mapping/embedding_service.py
- pipelines/mapping/vector_store.py
- pipelines/mapping/retriever.py
- pipelines/scoring/benchmark_learner.py
- pipelines/scoring/grade_predictor.py
- pipelines/scoring/scenario_simulator.py
- pipelines/generation/llm_client.py
- pipelines/generation/prompt_builder.py
- pipelines/generation/style_applier.py
- pipelines/recommendation/gap_analyzer.py
- pipelines/recommendation/content_suggester.py
- pipelines/recommendation/prioritizer.py
- pipelines/workers/ingestion_worker.py
- pipelines/workers/generation_worker.py
- pipelines/workers/simulation_worker.py
- pipelines/main.py
- pipelines/config/settings.py
- pipelines/config/models.py

### Go Control Plane
- apps/control-plane/main.go
- apps/control-plane/cmd/server/server.go
- apps/control-plane/internal/workflow/state_machine.go
- apps/control-plane/internal/workflow/handlers.go
- apps/control-plane/internal/scheduler/dispatcher.go
- apps/control-plane/internal/store/postgres.go

### Schemas
- schemas/proto/document.proto
- schemas/proto/criterion.proto
- schemas/proto/simulation.proto
- schemas/proto/recommendation.proto
- schemas/proto/workflow.proto
- schemas/openapi/openapi.yaml

### Shared Library
- pkg/models/document.py
- pkg/models/criterion.py
- pkg/models/report.py
- pkg/models/simulation.py
- pkg/errors/custom_errors.py

### Database
- .moai/db/schema/initial.sql

### Tests
- tests/e2e/test_document_to_report.py
- tests/integration/test_control_plane_integration.py
- pipelines/ingestion/tests/
- pipelines/mapping/tests/
- pipelines/scoring/tests/
- pipelines/generation/tests/
- pipelines/recommendation/tests/

---

## Exclusions (What NOT to Build)

1. Console UI (apps/console/) — Next.js 대시보드·뷰어·시뮬레이션 UI·recommendation UI 일체 제외.
2. 다중 문서 배치 처리 — 1 문서만 처리.
3. 전체 500개 평가항목 — 안전보건 1개·1개 지표만.
4. 인접 도메인 (ESG/감사/면허/공문) — Phase 3 후보.
5. 금융권 도메인 — Phase 4+ 후보.
6. 멀티테넌트 분리 — 단일 테넌트.
7. 프로덕션 K8s 배포 — sandbox 단일 노드.
8. 감사 로그 UI — 스키마 + 최소 기록만.
9. 모델 파인튜닝 — 베이스 모델 그대로.
10. PII 고급 마스킹 — 기본 regex만.
11. 등급 C/D 분류 — A/B 2-class만. **결과**: KEPCO E&C 제공 C/D 등급 데이터는 본 SPEC에서 학습 입력으로 사용되지 않음 (under-utilization 명시; 4-class SPEC 후보).
12. SSO/JWT 인증 — TBD.
13. Helm prod values, kustomize overlays — docker-compose + dev values만.
14. CI/CD 배포 자동화 — 테스트 실행까지만.

---

## Performance Targets

| Target | Threshold |
|--------|-----------|
| OCR per page (Qwen2-VL 7B, single GPU) | p99 < 2s |
| RAG retrieval | p99 < 100ms |
| Draft generation per indicator | p99 < 5s |
| Grade prediction | p99 < 1s |

## Quality Gates

- Document parse accuracy ≥ 95% (KEPCO E&C reference HWP)
- RAG top-3 avg relevance ≥ 0.8
- Grade prediction agreement ≥ 80% vs human review (A/B 2-class)
- 공문 style subjective rating ≥ 4/5
- Test coverage ≥ 85% (quality.yaml)
- LSP zero-error (errors/type-errors/lint-errors = 0)
- TDD: test_first_required = true
- TRUST 5 all dimensions pass

## Korean-specific Quality (acceptance.md §7.1)

- HWP OLE 손상 복원율 ≥ 80% (10건 샘플) — AC-001-2
- 공문 합니다체 final 일관성 ≥ 95% — AC-004-1, AC-004-2
- 한자/한글 정규화 성공률 ≥ 90% — AC-002-3
- 한자 정규화 실패 graceful (no HTTP 500) = 100% — AC-002-5
- 한국어 비율 < 20% 경고 기록 = 100% — AC-UBI-002
- RAG cold-start 명시 응답 (no silent empty / no 500) = 100% — AC-002-6

---

## Iteration 2 Defects Addressed (plan-auditor review #1)

- D1: REQ-UBI-002, REQ-UBI-003 dedicated AC 추가 (AC-UBI-002, AC-UBI-003)
- D2: REQ-AX-001-S1 dedicated AC 추가 (AC-001-4)
- D3: REQ-AX-002-S1 dedicated AC 추가 (AC-002-4)
- D4/D5: REJECTED — canonical 8-field schema (.claude/skills/moai/workflows/plan.md L377) 유지
- D6: AC-003-2 primary scenario를 both < 0.5로 reconcile (Option B)
- D7: AC-001-5 GPU/CPU 분기 dedicated AC 추가
- D8: Scenario total 16 → 23
- D9: AC-002-5 한자 정규화 실패 graceful
- D10: AC-002-6 RAG cold-start
- D11: Exclusions item 11 데이터 미활용 명시
- D12: spec.md §6 structure.md VECTOR(1536→768) handoff note
- D13: spec.md HISTORY interview.md anchor 추가
- D14: §7.1 Korean-specific quality criteria 별도 표

## Iteration 3 Findings Addressed (evaluator-active cross-validation)

- NEW-F1: REQ-AX-003-E1에 abstain 분기 명시 (2-class softmax + abstain 3-way 출력) — mathematical contradiction 해소 (P(A)+P(B)=1.0 vs both < 0.5)
- NEW-F2: REQ-UBI-003에 sandbox user_id 기본값 'cli-anonymous' State-driven 절 추가 — Exclusion §12 sandbox 환경 implementability 확보
- AC-UBI-004 신규 추가 — sandbox user_id 기본값 검증
- Scenario total 23 → 24
