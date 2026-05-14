---
id: SPEC-AX-001
version: 0.1.2
status: draft
created: 2026-05-14
updated: 2026-05-14
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.2 (2026-05-14): evaluator-active cross-validation 후속. REQ-AX-003-E1에 abstain 분기 명시(2-class softmax + abstain 3-way 출력), REQ-UBI-003에 sandbox 환경 user_id 기본값 'cli-anonymous' State-driven 절 추가. AC-UBI-004 1개 추가. (작성자: ircp)
- 0.1.1 (2026-05-14): plan-auditor iteration 1 리뷰 반영. (a) Acceptance gap 해소: REQ-UBI-002/003, REQ-AX-001-S1, REQ-AX-002-S1 dedicated AC 추가, AC-001-5 (GPU 분기), AC-002-5 (한자/한글 정규화 실패), AC-002-6 (RAG cold-start) 신설로 총 23개 시나리오. (b) D6 reconcile: AC-003-2 primary scenario를 `{A: 0.42, B: 0.45}` (both < 0.5)로 교정하여 REQ-AX-003-U1과 정렬. (c) D11 데이터 미활용 결과 명시. (d) D12 structure.md L329 VECTOR(1536) → 768 dim 정정 필요 사항 후속 SPEC에서 처리 명시. (e) Korean-specific 품질 기준 acceptance.md §7 별도 표로 분리. YAML frontmatter 스키마 결정: D4/D5 REJECT, 본 프로젝트 canonical schema는 `.claude/skills/moai/workflows/plan.md` L377 (id, version, status, created, updated, author, priority, issue_number 8 fields)를 따른다. 감사자가 요구한 `labels`/`created_at`은 본 프로젝트 schema 외 항목으로 보류.
- 0.1.0 (2026-05-14): 안전보건 PoC Walking Skeleton 초안 작성. 5개 MVP 기능(Document Ingestion → Criterion Mapping → Grade Simulation → Report Draft Generation → Gap Recommendation)을 단일 평가항목(안전보건) E2E 슬라이스로 정의. KEPCO E&C anchor 고객, CLI/API 우선, UI 후행. Discovery 인터뷰는 `.moai/project/interview.md` §10 anchor 참조. (작성자: ircp)

> Schema note (D4/D5 결정): YAML frontmatter는 본 프로젝트 정식 schema `.claude/skills/moai/workflows/plan.md` Phase 2 (L377)의 8-field 정의를 따른다. 감사 prompt가 요구한 `labels` 필드와 `created_at` 필드명은 본 schema와 충돌하므로 채택하지 않는다. schema 변경은 별도 SPEC(`moai-foundation-core` 변경)으로 진행한다.

---

# SPEC-AX-001 — 안전보건 PoC Walking Skeleton

## 1. 개요

iroum-ax 플랫폼의 첫 번째 E2E thin slice. 한국 공공기관(anchor: KEPCO E&C)의 경영평가 안전보건 항목 1개에 대해 HWP 입력에서 Gap recommendation 출력까지 5개 MVP 기능(`product.md` §5.1-5.5)을 단일 워크플로우로 통과시킨다.

**Walking Skeleton 정의**: 각 기능은 최소 실용 범위만 다룬다.
- 입력 문서: 1개 (KEPCO E&C 자사 안전보건 실적보고서 HWP)
- 평가항목: 1개 (안전보건)
- 평가지표: 1개 (PoC 진행 중 확정)
- 등급 비교: 2개 (A vs B)
- 보고서 섹션: 1개
- 추천 항목: 3-5개

**전제**: 본 SPEC은 CLI/API 진입점만 제공하며 Console UI(`apps/console/`)는 의도적으로 제외한다. UI는 본 SPEC 이후 별도 SPEC에서 진행한다.

**Anchor 컨텍스트**: 본 SPEC은 `.moai/project/interview.md` §10(첫 SPEC 슬라이스 정의)을 직접 구현한다. 결과물은 KEPCO E&C 영업·PoC 검토 자료로 사용된다.

---

## 2. 영향받는 파일 (Affected Files)

본 SPEC은 Greenfield 단계이므로 모든 파일은 신규 작성이다. 모든 경로는 `.moai/project/structure.md` §2 디렉토리 트리에 정의된 위치에 따른다.

### 2.1 Python Pipelines (`pipelines/`)

| 경로 | 책임 | 모듈 |
|------|------|------|
| `pipelines/ingestion/hwp_parser.py` | HWP OLE 구조 파싱, 텍스트·표·메타데이터 추출 | REQ-AX-001 |
| `pipelines/ingestion/pdf_parser.py` | PDF 텍스트 추출 + 회전 페이지 처리 | REQ-AX-001 |
| `pipelines/ingestion/vlm_processor.py` | Qwen2-VL 7B 호출, OCR 결과 정규화 | REQ-AX-001 |
| `pipelines/ingestion/table_extractor.py` | VLM 출력 후처리, 셀 정렬 | REQ-AX-001 |
| `pipelines/mapping/criterion_parser.py` | 평가편람 항목→지표→배점 계층 파싱 | REQ-AX-002 |
| `pipelines/mapping/embedding_service.py` | ko-sroberta-multitask 임베딩 | REQ-AX-002 |
| `pipelines/mapping/vector_store.py` | pgvector HNSW 인덱싱·upsert | REQ-AX-002 |
| `pipelines/mapping/retriever.py` | top-k 검색·재순위 | REQ-AX-002 |
| `pipelines/scoring/benchmark_learner.py` | A/B 등급 보고서 특징 추출 | REQ-AX-003 |
| `pipelines/scoring/grade_predictor.py` | 자사 보고서 등급 확률 산출 | REQ-AX-003 |
| `pipelines/scoring/scenario_simulator.py` | B→A 점수 시나리오 시뮬레이션 | REQ-AX-003 |
| `pipelines/generation/llm_client.py` | EXAONE 3.5 7B 호출 + Qwen 2.5 fallback | REQ-AX-004 |
| `pipelines/generation/prompt_builder.py` | 평가지표별 프롬프트 템플릿 | REQ-AX-004 |
| `pipelines/generation/style_applier.py` | 한국어 공문 합니다체 검증·재생성 | REQ-AX-004 |
| `pipelines/recommendation/gap_analyzer.py` | 현재 vs 목표 등급 콘텐츠 차이 분석 | REQ-AX-005 |
| `pipelines/recommendation/content_suggester.py` | 벤치마크 기반 콘텐츠 제안 | REQ-AX-005 |
| `pipelines/recommendation/prioritizer.py` | 실현 가능성 우선순위 정렬 | REQ-AX-005 |
| `pipelines/workers/ingestion_worker.py` | Celery 비동기 Document Ingestion 워커 | REQ-AX-001 |
| `pipelines/workers/generation_worker.py` | Celery 비동기 초안 생성 워커 | REQ-AX-004 |
| `pipelines/workers/simulation_worker.py` | Celery 비동기 등급 시뮬레이션 워커 | REQ-AX-003 |
| `pipelines/main.py` | FastAPI 진입점 (REST 엔드포인트) | 전체 |
| `pipelines/config/settings.py` | 환경 설정 (vLLM endpoint, pgvector DSN) | 전체 |
| `pipelines/config/models.py` | Pydantic 데이터 모델 | 전체 |

### 2.2 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 |
|------|------|
| `apps/control-plane/main.go` | 진입점 |
| `apps/control-plane/cmd/server/server.go` | gRPC(:50051) + REST(:8080) 서버 |
| `apps/control-plane/internal/workflow/state_machine.go` | 워크플로우 상태머신 (PENDING/RUNNING/COMPLETED/FAILED) |
| `apps/control-plane/internal/workflow/handlers.go` | 워크플로우 단계별 핸들러 |
| `apps/control-plane/internal/scheduler/dispatcher.go` | Celery 태스크 dispatch |
| `apps/control-plane/internal/store/postgres.go` | PostgreSQL 클라이언트 (documents/workflows) |
| `apps/control-plane/internal/proto/` | Protobuf 생성 코드 (read-only target) |

### 2.3 Shared Schemas (`schemas/`)

| 경로 | 책임 |
|------|------|
| `schemas/proto/document.proto` | document 메시지 정의 |
| `schemas/proto/criterion.proto` | criterion 메시지 정의 |
| `schemas/proto/simulation.proto` | simulation 메시지 정의 |
| `schemas/proto/recommendation.proto` | recommendation 메시지 정의 |
| `schemas/proto/workflow.proto` | workflow 메시지 정의 |
| `schemas/openapi/openapi.yaml` | REST API 명세 |

### 2.4 Shared Library (`pkg/`)

| 경로 | 책임 |
|------|------|
| `pkg/models/document.py` | Python document 데이터 클래스 |
| `pkg/models/criterion.py` | Python criterion 데이터 클래스 (계층) |
| `pkg/models/report.py` | Python report 데이터 클래스 |
| `pkg/models/simulation.py` | Python simulation 결과 데이터 클래스 |
| `pkg/errors/custom_errors.py` | 공통 에러 정의 (`HWPParseError`, `RAGInsufficientContextError`, 등) |

### 2.5 Database (`.moai/db/`)

| 경로 | 책임 |
|------|------|
| `.moai/db/schema/initial.sql` | documents/criteria/reports/workflows/simulations/audit_logs 테이블 + pgvector 인덱스 |

### 2.6 Tests (`tests/`, `pipelines/*/tests/`)

| 경로 | 책임 |
|------|------|
| `tests/e2e/test_document_to_report.py` | HWP 업로드 → recommendation 출력 E2E 검증 |
| `tests/integration/test_control_plane_integration.py` | control-plane ↔ pipelines 통합 |
| `pipelines/ingestion/tests/` | REQ-AX-001 단위 테스트 |
| `pipelines/mapping/tests/` | REQ-AX-002 단위 테스트 |
| `pipelines/scoring/tests/` | REQ-AX-003 단위 테스트 |
| `pipelines/generation/tests/` | REQ-AX-004 단위 테스트 |
| `pipelines/recommendation/tests/` | REQ-AX-005 단위 테스트 |

---

## 3. EARS 요구사항

### 3.1 Ubiquitous Requirements (시스템 전반 불변 조건)

- **REQ-UBI-001 (데이터 주권)**: The system SHALL store all input documents, intermediate artifacts, and output reports exclusively within the customer-controlled infrastructure (단일 노드 sandbox 포함). 외부 LLM API 호출은 금지된다.
- **REQ-UBI-002 (언어)**: The system SHALL process Korean text as the primary language for all input parsing, RAG retrieval, draft generation, and recommendation output.
- **REQ-UBI-003 (감사 가능성)**: The system SHALL record an audit log entry for every document upload, workflow creation, draft generation, and prediction event in `audit_logs` 테이블 with user_id, action, resource_id, and timestamp. WHILE 인증 시스템(SSO/JWT)이 비활성화 상태(Exclusion §12 sandbox 환경)일 때, the system SHALL audit_logs.user_id 필드에 'cli-anonymous' 문자열 기본값을 기록한다.

### 3.2 REQ-AX-001 — Document Ingestion

#### Event-driven
- **REQ-AX-001-E1**: WHEN a user uploads a HWP or PDF file via `POST /api/documents/upload`, THEN the system SHALL parse the file, extract text·tables·metadata, persist a `documents` 레코드, and return a document_id within 30 seconds for a reference 50-page HWP.

#### State-driven
- **REQ-AX-001-S1**: WHILE the VLM (Qwen2-VL 7B) is processing OCR for a single page, THE system SHALL not accept additional OCR requests for that document and SHALL queue concurrent requests through Celery.

#### Optional
- **REQ-AX-001-O1**: WHERE a GPU is available in the deployment environment, THE system SHALL use vLLM-accelerated Qwen2-VL 7B inference targeting <2 seconds per page; WHERE GPU is unavailable, THE system MAY fall back to CPU inference with degraded latency (5-10x slower, per `tech.md` §6.1).

#### Unwanted
- **REQ-AX-001-U1**: IF hwp-converter detects HWP OLE structure corruption or raises an exception, THEN the system SHALL automatically invoke the VLM OCR fallback path, mark the document with status `ocr_fallback`, and continue processing without user intervention.
- **REQ-AX-001-U2**: IF both hwp-converter and VLM OCR fail for a given document, THEN the system SHALL persist the document with status `parse_failed`, return HTTP 422 with the failure reason, and SHALL NOT silently drop the input.

---

### 3.3 REQ-AX-002 — Criterion Mapping (RAG Indexing)

#### Event-driven
- **REQ-AX-002-E1**: WHEN a 평가편람 PDF is uploaded for indexing, THEN the system SHALL parse 항목→지표→배점 계층 구조, chunk each leaf criterion into segments of 500-1000 tokens, compute embeddings via ko-sroberta-multitask, and persist them into pgvector with HNSW indexing.
- **REQ-AX-002-E2**: WHEN a retrieval query is issued via `GET /api/criteria/search?q={query}`, THEN the system SHALL return top-3 results with p99 latency < 100 milliseconds.

#### State-driven
- **REQ-AX-002-S1**: WHILE the pgvector HNSW index is being constructed or rebuilt, THE system SHALL queue retrieval requests and SHALL NOT serve stale or partial index results.

#### Unwanted
- **REQ-AX-002-U1**: IF fewer than 3 results above relevance threshold 0.7 are retrieved for a query, THEN the system SHALL return status `insufficient_context` with the candidates that were retrieved and flag the query for manual review, instead of returning an empty result silently.

---

### 3.4 REQ-AX-003 — Grade Simulation

#### Event-driven
- **REQ-AX-003-E1**: WHEN 자사 보고서가 grade_predictor에 입력되면, the system SHALL 2-class softmax 분류기(A vs B) 결과에 더해 abstain 분기(두 클래스 확률이 모두 0.5 미만일 때 활성화)를 포함하여 등급 확률 분포 + abstain 플래그를 반환한다. 전체 출력은 `{A, B, abstain}` 3-way 분포로 표현되며 sum=1.0 ± 0.001을 만족하고, 추론 응답 시간은 1초 이내이다. abstain 플래그가 활성화된 경우 max(p_a, p_b) < 0.5 이며 REQ-AX-003-U1의 low_confidence 처리 경로로 연결된다.

#### State-driven
- **REQ-AX-003-S1**: WHILE the benchmark learner is in training mode (feature extraction or fitting), THE system SHALL reject prediction requests with HTTP 503 and a `model_training` status, instead of returning stale predictions.

#### Unwanted
- **REQ-AX-003-U1**: IF abstain 플래그가 활성화되면(두 클래스 확률 max(p_a, p_b) < 0.5), THEN the system SHALL mark the prediction with status `low_confidence`, return all candidate probabilities including abstain, and request human verification before downstream consumption.

---

### 3.5 REQ-AX-004 — Report Draft Generation

#### Event-driven
- **REQ-AX-004-E1**: WHEN the system receives a request to draft a report section for a single evaluation indicator (e.g., 안전교육 이수율), THEN the system SHALL invoke EXAONE 3.5 7B via vLLM, apply the Korean 공문 합니다체 style template, validate the output through `style_applier`, and return the draft within 5 seconds.

#### Optional
- **REQ-AX-004-O1**: WHERE the EXAONE 3.5 endpoint is reachable, THE system SHALL use EXAONE 3.5 7B as the primary generator; WHERE EXAONE 3.5 is unreachable or returns errors for three consecutive attempts, THE system SHALL automatically fall back to Qwen 2.5 7B with the same prompt template (per `tech.md` §3.3).

#### Unwanted
- **REQ-AX-004-U1**: IF the generated draft contains 반말 forms, mixed honorifics (합니다체와 해체 혼용), or violates the Korean 공문 style rules verified by `style_applier`, THEN the system SHALL discard the draft, re-prompt the LLM with style reinforcement up to 3 retries, and return `style_violation` status if all retries fail.
- **REQ-AX-004-U2**: IF the EXAONE 3.5 fallback to Qwen 2.5 is also exhausted, THEN the system SHALL persist the failure, return HTTP 503, and SHALL NOT auto-escalate to any external LLM API (REQ-UBI-001 데이터 주권 invariant).

---

### 3.6 REQ-AX-005 — Gap Recommendation

#### Event-driven
- **REQ-AX-005-E1**: WHEN a workflow has produced a 자사 draft, a current-grade prediction (B), and a target grade (A) is specified, THEN the system SHALL run gap analysis against A 등급 벤치마크 콘텐츠 index and return 3 to 5 prioritized recommendation items with expected score delta within 3 seconds.

#### State-driven
- **REQ-AX-005-S1**: WHILE benchmark content for A 등급 has not been indexed for the requested criterion, THE system SHALL return an empty recommendation list with status `benchmark_not_available` and SHALL NOT fabricate suggestions.

#### Unwanted
- **REQ-AX-005-U1**: IF a customer marks a recommendation item as `not_feasible` via `POST /api/recommendations/{id}/feedback`, THEN the system SHALL downgrade that item's priority in subsequent recommendations, propose an alternative content item if available, and persist the feedback for future scoring adjustments.

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 성능 - OCR | Qwen2-VL 7B 단일 GPU 환경에서 페이지당 <2초 | `tech.md` §11.1 |
| 성능 - RAG | top-k 검색 p99 < 100ms | `tech.md` §11.1 |
| 성능 - 초안 생성 | 평가지표당 < 5초 | `tech.md` §11.1 |
| 성능 - 등급 예측 | 보고서당 < 1초 | `tech.md` §11.1 |
| 품질 - 문서 파싱 정확도 | KEPCO E&C 참조 HWP 기준 >= 95% | `product.md` §5.1 |
| 품질 - RAG 정확도 | top-3 평균 relevance >= 0.8 | `product.md` §5.2 |
| 품질 - 등급 예측 일치율 | 사람 검수 대비 >= 80% (A/B 2-class) | `product.md` §5.3 |
| 품질 - 공문 스타일 | 주관 평가 4/5 이상 | `product.md` §6.1 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR) | `quality.yaml` development_mode |
| 망분리 | 외부 API 차단, K8s NetworkPolicy 적용 (sandbox 포함) | `tech.md` §9.1 |
| PII 마스킹 | 기본 regex (전화번호, 한글 인명 2-4자) | `tech.md` §9.2 |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **Console UI (`apps/console/`)** — Next.js 대시보드·문서 뷰어·시뮬레이션 UI·recommendation UI 일체 제외. CLI/API만 제공.
2. **다중 문서 배치 처리** — 한 번에 1개 문서만 처리. 폴더 단위 일괄 업로드 미지원.
3. **전체 500개 평가항목** — 안전보건 1개 항목, 1개 지표만 처리. ESG·재무·고객만족 등 타 항목 제외.
4. **인접 도메인 (ESG/감사/면허/공문)** — `product.md` §8.1의 Phase 3 후보. 본 SPEC 범위 외.
5. **금융권 도메인** — `product.md` §8.1의 Phase 4+ 후보. 공공 anchor 성공 사례 확보 전까지 보류.
6. **멀티테넌트 배포** — 단일 테넌트(KEPCO E&C). RBAC·격리·도메인 어댑터 제외.
7. **프로덕션 K8s 배포** — sandbox 단일 노드(단일 GPU 권장)에서 검증. Helm production values, kustomize overlays, HA·복제본 제외.
8. **감사 로그 UI** — `audit_logs` 테이블 스키마와 최소 이벤트 기록만 포함. 조회·필터링·내보내기 UI 제외.
9. **모델 파인튜닝** — Qwen2-VL 7B, EXAONE 3.5 7B 베이스 모델 그대로 사용. LoRA·QLoRA·full fine-tuning 제외.
10. **PII 고급 마스킹** — 기본 regex(전화번호, 한글 인명)만 적용. 직책·내부 식별자·민감 실적 마스킹은 후속 SPEC.
11. **등급 C/D 분류** — A vs B 2-class만 처리. C/D 4-class 확장은 SPEC-AX-002 이후 후보. **결과: KEPCO E&C가 제공한 C/D 등급 벤치마크 보고서 데이터(`product.md` §3.2 item 4)는 본 SPEC 범위에서 학습 입력으로 사용되지 않는다.** 해당 데이터 자산은 4-class 확장 SPEC에서 활용 예정이며, 본 PoC에서는 데이터 보관·인덱싱 자체도 제외한다 (under-utilization 명시).
12. **SSO/JWT 인증** — `structure.md` §3.2 TBD 항목. 본 SPEC은 인증 미적용 (sandbox 내부 통신).
13. **Helm 차트 prod values, kustomize overlays** — docker-compose 기반 로컬 환경 또는 단일 노드 K8s sandbox만 지원.
14. **CI/CD 파이프라인 전체** — `.github/workflows/ci.yml`은 단위 테스트·E2E 테스트 실행까지만. 배포 자동화(`deploy.yml`) 제외.

---

## 6. 의존성 및 전제

- `.moai/project/structure.md` §2 디렉토리 트리는 단일 진실 공급원. 신규 경로 추가 시 본 SPEC을 중단하고 structure.md를 먼저 갱신.
- **structure.md 정합성 경고 (D12)**: `structure.md` L329는 `criteria.embedding VECTOR(1536)`로 정의되어 있으나, 본 SPEC §3.3 (REQ-AX-002-E1)과 `tech.md` §4.3에서 사용하는 `ko-sroberta-multitask` 임베딩은 768차원이다. T-AX-008 (`plan.md`) DDL 작성 시 `VECTOR(768)`로 정정하며, 동시에 별도 structure.md 패치 SPEC 또는 manager-spec 핸드오프 노트로 `structure.md`를 수정한다. 본 불일치는 SPEC-AX-001 범위 밖이나 Run phase 시작 전 해결되어야 한다.
- `pkg/models/`, `schemas/proto/`는 REQ-AX-001 시작 전에 정의 완료되어야 한다 (`plan.md` T-AX-007/T-AX-008 참조).
- KEPCO E&C 제공 자료 4종(`product.md` §3.2): 평가편람, 작성지침, 자사 실적보고서, A/B 벤치마크 보고서. PoC 진행 중 실 데이터로 RAG 인덱싱·벤치마크 학습 수행.
- EXAONE 3.5 액세스는 PoC 시작 시점에 확인. 미확정 시 Qwen 2.5 7B로 자동 fallback (REQ-AX-004-O1).
- GPU 자원: NVIDIA A100/H100 1매 권장. 부재 시 CPU 추론(`tech.md` §6.1).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- 평가편람 자동 업데이트 메커니즘 (편람 개정 시 신규 인덱싱)
- 다중 평가년도 비교 (전년 대비 변동)
- 보고서 버전 관리 (drafts vs final)
- 사용자 권한·역할 관리

위 항목은 본 PoC 검증 후 별도 SPEC에서 다룬다.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- E2E 테스트: `tests/e2e/test_document_to_report.py` — HWP 업로드부터 recommendation 응답까지 단일 시나리오 통과
- 단위 테스트: 모듈별 `pipelines/{module}/tests/` 디렉토리
- 통합 테스트: control-plane ↔ pipelines gRPC 호출 검증
- 성능 측정: `iroum_ax_document_processing_duration_seconds` Prometheus 히스토그램 (`tech.md` §11.1)
- 사용자 만족도: KEPCO E&C 담당자 주관 평가 4/5 이상

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
