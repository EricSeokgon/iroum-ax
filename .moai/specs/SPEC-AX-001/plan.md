# SPEC-AX-001 Implementation Plan — 안전보건 PoC Walking Skeleton

> 본 plan은 `spec.md`의 5개 EARS 모듈(REQ-AX-001 ~ REQ-AX-005)을 9개 구현 태스크로 분해한다.
> 모든 우선순위는 High/Medium/Low 레이블만 사용한다 (시간 추정 금지, `agent-common-protocol.md` Time Estimation 규칙).

- 작성일: 2026-05-14
- 작성자: ircp
- 대상 SPEC: SPEC-AX-001
- 개발 방법론: TDD (`quality.yaml` development_mode: tdd)
- 품질 게이트: TRUST 5 + 85% coverage + LSP zero-error

---

## 1. 태스크 분해 (Task Decomposition)

### T-AX-007 — 공유 스키마 정의 (Schema Contracts)

- **매핑**: 전체 (선행 조건)
- **언어**: Protobuf
- **진입점**: `schemas/proto/buf.yaml`
- **출력 파일**: `schemas/proto/document.proto`, `criterion.proto`, `simulation.proto`, `recommendation.proto`, `workflow.proto`, `schemas/openapi/openapi.yaml`
- **의존성**: 없음
- **선행 조건**: 없음 (가장 먼저 수행)
- **블록 대상**: T-AX-006, T-AX-001 ~ T-AX-005
- **복잡도 우선순위**: High
- **참고 구현(Reference)**: gRPC-Gateway v2 documentation (`tech.md` §12.1)

### T-AX-008 — 데이터베이스 스키마 (Database Schema)

- **매핑**: 전체 (선행 조건)
- **언어**: SQL (PostgreSQL 16 + pgvector 0.5.0+)
- **진입점**: `.moai/db/schema/initial.sql`
- **출력 테이블**: documents, criteria(부모-자식 계층), reports, workflows, simulations, recommendations, audit_logs
- **출력 인덱스**: pgvector HNSW on `criteria.embedding`, B-tree on `documents.created_at`, foreign key on `workflows.document_id`
- **의존성**: 없음
- **선행 조건**: 없음
- **블록 대상**: T-AX-002, T-AX-006
- **복잡도 우선순위**: High
- **참고 구현(Reference)**: pgvector README HNSW indexing (`tech.md` §11.1), `structure.md` §5 스키마 개요

### T-AX-001 — Document Ingestion (REQ-AX-001)

- **매핑**: REQ-AX-001
- **언어**: Python 3.11+
- **진입점**: `pipelines/main.py` `POST /api/documents/upload` 핸들러 → `pipelines/workers/ingestion_worker.py` Celery 태스크
- **출력 파일**: 
  - `pipelines/ingestion/hwp_parser.py`
  - `pipelines/ingestion/pdf_parser.py`
  - `pipelines/ingestion/vlm_processor.py`
  - `pipelines/ingestion/table_extractor.py`
  - `pipelines/workers/ingestion_worker.py`
  - `pkg/models/document.py`
  - `pipelines/ingestion/tests/test_hwp_parser.py`
  - `pipelines/ingestion/tests/test_vlm_processor.py`
- **의존성**: hwp-converter, Qwen2-VL 7B (vLLM endpoint), Celery, FastAPI, Pydantic 2.x
- **선행 조건**: T-AX-007, T-AX-008
- **복잡도 우선순위**: High
- **참고 구현(Reference)**: 
  - hwp-converter README (`tech.md` §14.1)
  - Qwen2-VL 모델 카드: https://huggingface.co/Qwen/Qwen2-VL-7B-Instruct
  - vLLM 성능 튜닝: https://docs.vllm.ai/en/latest/performance_tuning.html

### T-AX-002 — Criterion Mapping (REQ-AX-002)

- **매핑**: REQ-AX-002
- **언어**: Python 3.11+
- **진입점**: `pipelines/main.py` `POST /api/criteria/index`, `GET /api/criteria/search`
- **출력 파일**: 
  - `pipelines/mapping/criterion_parser.py`
  - `pipelines/mapping/embedding_service.py`
  - `pipelines/mapping/vector_store.py`
  - `pipelines/mapping/retriever.py`
  - `pkg/models/criterion.py`
  - `pipelines/mapping/tests/test_retriever.py`
  - `pipelines/mapping/tests/test_criterion_parser.py`
- **의존성**: ko-sroberta-multitask (sentence-transformers), pgvector (psycopg2 또는 SQLAlchemy 2.0 + pgvector binding)
- **선행 조건**: T-AX-008 (pgvector HNSW 인덱스 존재 필요)
- **복잡도 우선순위**: High
- **참고 구현(Reference)**: 
  - ko-sroberta-multitask: https://huggingface.co/jhgan/ko-sroberta-multitask
  - pgvector query performance: https://github.com/pgvector/pgvector#query-performance

### T-AX-003 — Grade Simulation (REQ-AX-003)

- **매핑**: REQ-AX-003
- **언어**: Python 3.11+
- **진입점**: `pipelines/main.py` `POST /api/simulations/train`, `POST /api/simulations/predict`
- **출력 파일**: 
  - `pipelines/scoring/benchmark_learner.py`
  - `pipelines/scoring/grade_predictor.py`
  - `pipelines/scoring/scenario_simulator.py`
  - `pipelines/workers/simulation_worker.py`
  - `pkg/models/simulation.py`
  - `pipelines/scoring/tests/test_grade_predictor.py`
- **의존성**: scikit-learn (LogisticRegression 또는 GradientBoosting), numpy, joblib (모델 직렬화)
- **선행 조건**: T-AX-002 (벤치마크 임베딩이 RAG 인덱스에 존재)
- **복잡도 우선순위**: Medium
- **참고 구현(Reference)**: scikit-learn LogisticRegression docs (PoC 단계 단순 선형 모델 우선)

### T-AX-004 — Report Draft Generation (REQ-AX-004)

- **매핑**: REQ-AX-004
- **언어**: Python 3.11+
- **진입점**: `pipelines/main.py` `POST /api/reports/generate` → `pipelines/workers/generation_worker.py`
- **출력 파일**: 
  - `pipelines/generation/llm_client.py`
  - `pipelines/generation/prompt_builder.py`
  - `pipelines/generation/style_applier.py`
  - `pipelines/workers/generation_worker.py`
  - `pkg/models/report.py`
  - `pipelines/generation/tests/test_llm_client_fallback.py`
  - `pipelines/generation/tests/test_style_applier.py`
- **의존성**: EXAONE 3.5 7B (vLLM endpoint, 1차) / Qwen 2.5 7B (vLLM endpoint, fallback), tiktoken, asyncio
- **선행 조건**: T-AX-002 (검색된 context 필요)
- **복잡도 우선순위**: High
- **참고 구현(Reference)**: 
  - vLLM 공식 문서: https://docs.vllm.ai/
  - Qwen 2.5 모델 카드: https://huggingface.co/Qwen/Qwen2.5-7B-Instruct (fallback)
  - EXAONE 3.5: LG AI 협력 후 공개 자료 (`tech.md` §14.1, _TBD)

### T-AX-005 — Gap Recommendation (REQ-AX-005)

- **매핑**: REQ-AX-005
- **언어**: Python 3.11+
- **진입점**: `pipelines/main.py` `POST /api/recommendations/generate`, `POST /api/recommendations/{id}/feedback`
- **출력 파일**: 
  - `pipelines/recommendation/gap_analyzer.py`
  - `pipelines/recommendation/content_suggester.py`
  - `pipelines/recommendation/prioritizer.py`
  - `pipelines/recommendation/tests/test_gap_analyzer.py`
  - `pipelines/recommendation/tests/test_prioritizer.py`
- **의존성**: T-AX-003 출력(grade prediction), T-AX-002 retriever
- **선행 조건**: T-AX-003 (current grade probability), T-AX-002 (벤치마크 콘텐츠 검색)
- **복잡도 우선순위**: Medium
- **참고 구현(Reference)**: 표준 gap analysis 패턴 (현재 vs 목표 벤치마크 차이 추출)

### T-AX-006 — Control Plane Workflow (워크플로 오케스트레이션)

- **매핑**: REQ-AX-001 ~ REQ-AX-005 (전체 통합)
- **언어**: Go 1.22+
- **진입점**: `apps/control-plane/main.go` → `apps/control-plane/cmd/server/server.go`
- **출력 파일**: 
  - `apps/control-plane/main.go`
  - `apps/control-plane/cmd/server/server.go`
  - `apps/control-plane/internal/workflow/state_machine.go`
  - `apps/control-plane/internal/workflow/handlers.go`
  - `apps/control-plane/internal/scheduler/dispatcher.go`
  - `apps/control-plane/internal/store/postgres.go`
  - `apps/control-plane/internal/workflow/state_machine_test.go`
- **의존성**: gRPC-Gateway v2, google.golang.org/protobuf, pgx/v5 (PostgreSQL), redis/go-redis (Celery broker 통신)
- **선행 조건**: T-AX-007, T-AX-008, T-AX-001 ~ T-AX-005 (전부 또는 일부 stub)
- **복잡도 우선순위**: High
- **참고 구현(Reference)**: gRPC-Gateway v2 docs (`tech.md` §12.1)

### T-AX-009 — E2E Integration Test (Walking Skeleton Validation)

- **매핑**: 전체 (Walking Skeleton 통과 검증)
- **언어**: Python 3.11+ (pytest)
- **진입점**: `tests/e2e/test_document_to_report.py`
- **출력 파일**: 
  - `tests/e2e/test_document_to_report.py`
  - `tests/integration/test_control_plane_integration.py`
  - `tests/fixtures/sample-안전보건-report.hwp` (테스트 픽스처)
  - `tests/fixtures/평가편람-안전보건.pdf`
  - `tests/fixtures/benchmark-a-grade.hwp`, `benchmark-b-grade.hwp`
- **의존성**: pytest, pytest-asyncio, httpx (FastAPI 테스트 클라이언트)
- **선행 조건**: T-AX-001 ~ T-AX-006 전부 완료
- **복잡도 우선순위**: High
- **참고 구현(Reference)**: FastAPI 테스트 가이드, pytest E2E 패턴

---

## 2. 의존성 및 단계 순서 (Phase Ordering)

본 SPEC은 시간 추정을 사용하지 않고 단계 순서와 우선순위로 기록한다.

### Phase 1 — 기반 (Foundation, 모두 High)

- T-AX-007 (Schema Contracts)
- T-AX-008 (Database Schema)

Phase 1 완료 후 Phase 2 시작.

### Phase 2 — 코어 파이프라인 (Core Pipelines, 병렬 가능)

병렬 그룹 A (RAG 기반):
- T-AX-001 (Document Ingestion, High)
- T-AX-002 (Criterion Mapping, High)

병렬 그룹 B (Generation):
- T-AX-004 (Report Draft Generation, High) — T-AX-002 완료 후 시작

순차:
- T-AX-003 (Grade Simulation, Medium) — T-AX-002 완료 후 시작
- T-AX-005 (Gap Recommendation, Medium) — T-AX-003 완료 후 시작

### Phase 3 — 오케스트레이션 (Orchestration)

- T-AX-006 (Control Plane Workflow, High) — T-AX-001 ~ T-AX-005가 최소 stub 단계 이상이면 병렬 시작 가능

### Phase 4 — 검증 (Validation)

- T-AX-009 (E2E Integration Test, High) — Phase 1-3 완료 후 수행

---

## 3. 기술 접근 방식 (Technical Approach)

### 3.1 TDD 사이클 적용 (`workflow-modes.md` TDD Mode)

각 태스크는 RED-GREEN-REFACTOR 사이클을 따른다.

- RED: 실패하는 테스트를 먼저 작성. `@MX:TODO` 태그를 해당 함수에 부착.
- GREEN: 테스트 통과를 위한 최소 구현. `@MX:TODO` 제거.
- REFACTOR: 코드 품질 개선. 복잡 로직에 `@MX:NOTE` 추가.

테스트 커버리지: 85% 이상 (`quality.yaml` test_coverage_target).

### 3.2 LSP Quality Gate (`quality.yaml` lsp_quality_gates)

- Run phase: max_errors=0, max_type_errors=0, max_lint_errors=0 (zero tolerance)
- 각 태스크 종료 시 `mypy` (Python), `go vet` + `golangci-lint` (Go), `buf lint` (Protobuf) 통과.

### 3.3 망분리 정합 (`tech.md` §9.1)

- 모든 vLLM 호출은 클러스터 내부 endpoint만 사용 (예: `http://vllm-qwen2vl:8000`, `http://vllm-exaone:8000`).
- HTTP egress는 K8s NetworkPolicy로 차단 (sandbox 환경에서도 docker-compose network 격리).
- 외부 API client(`openai`, `anthropic` 등) 의존성 추가 금지.

### 3.4 한국어 공문 스타일 검증 (`style_applier`)

- 규칙 기반 1차 검증: 합니다체 패턴 (`-습니다`, `-입니다`, `-하였습니다`) 매칭
- 위반 발견 시 LLM 재생성 (최대 3회)
- 통과 기준: 검증 100%, 주관 평가 4/5 이상 (`product.md` §6.1)

### 3.5 RAG 인덱싱 전략 (`tech.md` §4.3)

- 청킹: 평가편람 leaf criterion 기준, 500-1000 토큰
- 임베딩: ko-sroberta-multitask (170M 파라미터, 768차원)
- 인덱스: pgvector HNSW (m=16, ef_construction=64 초기값, 측정 후 고정)
- 검색: top-5 후 cross-encoder 재순위는 본 PoC에서는 비활성 (Out of scope, TBD)

---

## 4. 위험 분석 매트릭스 (Risk Analysis)

`tech.md` §13 위험 분석을 본 SPEC 컨텍스트로 재정의.

| ID | 위험 | 확률 | 영향 | 완화 전략 | 담당 태스크 |
|----|------|------|------|----------|-------------|
| R-AX-001 | HWP OLE 구조 손상 → 파싱 실패 | 중 | Document Ingestion 차단 | VLM OCR 자동 폴백 (REQ-AX-001-U1) | T-AX-001 |
| R-AX-002 | Qwen2-VL 7B 테이블 인식 정확도 < 95% | 중 | OCR 품질 목표 미달 | 1차: 7B 벤치마크; 미달 시 72B production gate(`tech.md` §11.1); 정확도 측정은 T-AX-009 E2E에서 자동화 | T-AX-001, T-AX-009 |
| R-AX-003 | RAG 콜드스타트 (안전보건 학습 코퍼스 제한) | 높음 | top-3 relevance < 0.8 | 청크 오버샘플링, 사람 검수 라벨링 1회, HNSW ef_search 튜닝 | T-AX-002 |
| R-AX-004 | EXAONE 3.5 액세스 불확실 (`tech.md` §3.3) | 중 | 초안 생성 차단 | REQ-AX-004-O1 자동 fallback to Qwen 2.5 7B; PoC 시작 시 EXAONE 가용성 1회 확인 | T-AX-004 |
| R-AX-005 | K8s GPU 자원 부족 (`tech.md` §6.1) | 중 | 추론 5-10배 지연 | sandbox 단일 GPU 권장; 부재 시 CPU 추론 경로, 성능 목표 일시 완화 명시 | T-AX-001, T-AX-004 |
| R-AX-006 | 공문 합니다체 위반 (`product.md` §5.4) | 중 | 사용자 만족도 저하 | style_applier 재생성 루프 (REQ-AX-004-U1, 최대 3회) | T-AX-004 |
| R-AX-007 | pgvector HNSW p99 > 100ms | 낮음 | RAG 성능 목표 미달 | 초기 인덱싱 후 ef_search/ef_construction 측정·고정, 100K vector까지는 HNSW 충분 | T-AX-002 |
| R-AX-008 | 한자/한글 혼용 임베딩 품질 저하 | 중 | criterion 매핑 부정확 | 정규화 전처리 (한자→한글 변환 옵션), 평가편람 베이스라인 측정 | T-AX-002 |
| R-AX-009 | A/B 2-class 데이터 불균형 | 중 | grade prediction 편향 | class weight 조정, 검증 셋 stratified split | T-AX-003 |
| R-AX-010 | Celery + Redis broker 안정성 (단일 노드 sandbox) | 낮음 | 워커 태스크 손실 | Redis persistence(AOF) 활성, retry policy 3회, dead-letter 큐 | T-AX-006 |

---

## 5. MX Tag Plan (`mx-tag-protocol.md` 참조)

`tech.md` §9 + `moai-constitution.md` MX Tag Quality Gates 적용.

### 5.1 @MX:ANCHOR (fan_in >= 3, 불변 계약)

- `pipelines/ingestion/hwp_parser.parse_document()` — REQ-AX-001 main entry, called by ingestion_worker + E2E test + control-plane handler
- `pipelines/mapping/embedding_service.embed_text()` — called by criterion_parser + retriever + benchmark_learner (3+ callers)
- `pipelines/mapping/retriever.search_topk()` — called by REQ-AX-004 prompt_builder + REQ-AX-005 gap_analyzer + control-plane handler
- `pipelines/generation/llm_client.generate()` — called by generation_worker + style_applier (재생성 루프) + recommendation 콘텐츠 제안 fallback
- `apps/control-plane/internal/workflow/state_machine.Transition()` — workflow lifecycle 핵심
- Protobuf message `Workflow`, `Document`, `Criterion` — schema-level invariants

### 5.2 @MX:WARN (위험 패턴, @MX:REASON 필수)

- `apps/control-plane/internal/scheduler/dispatcher.go` goroutines — Celery dispatch concurrency. @MX:REASON: "context cancellation on workflow timeout, see REQ-AX-001-S1"
- `pipelines/ingestion/vlm_processor.batch_inference()` — VLM 배치 동시성. @MX:REASON: "GPU memory exhaustion risk, see tech.md §13.2"
- `pipelines/workers/generation_worker.py` retry loop — EXAONE→Qwen fallback. @MX:REASON: "max 3 retries per REQ-AX-004-U1"

### 5.3 @MX:NOTE (한국 비즈니스 규칙)

- `pipelines/generation/style_applier.py` 합니다체 검증 함수 — 공문 honorific rule
- `pipelines/mapping/criterion_parser.parse_hierarchy()` — 평가편람 항목→지표→배점 계층
- `pipelines/recommendation/prioritizer.py` 실현 가능성 score 가중치

### 5.4 @MX:TODO (RED phase 표식)

- 모든 RED phase 실패 테스트에 부착
- GREEN phase 통과 시 자동 제거

---

## 6. 테스트 전략 (`quality.yaml` test_quality)

- specification_based: 행위 기반 테스트 (구현 세부 비결합)
- meaningful_assertions: 모든 assertion은 사양과 직접 매핑
- avoid_implementation_coupling: 내부 함수 호출 검증 금지, 출력 검증 우선

테스트 픽스처:
- KEPCO E&C 자사 실적보고서 1개 (HWP, 익명화 또는 대체 샘플)
- 평가편람 안전보건 항목 1개 (PDF)
- A 등급 벤치마크 보고서 1개, B 등급 벤치마크 보고서 1개

테스트 위치는 §1 각 태스크 출력 파일 목록 참고. 통합 테스트는 `tests/integration/`, E2E는 `tests/e2e/`.

---

## 7. 배포 전제 (`tech.md` §6.2)

- 로컬 개발: `docker-compose up`으로 PostgreSQL + Redis + vLLM(2개) + control-plane + pipelines 기동
- Sandbox 배포: `helm install iroum-ax ./deployments/helm/iroum-ax -f values-dev.yaml`
- 프로덕션 배포: 본 SPEC 범위 외

---

## 8. 검증 종료 조건 (Definition of Done 요약)

상세는 `acceptance.md`. 본 plan에서는 단순화한 목록만 기록.

- 모든 EARS 요구사항(REQ-UBI-001~003, REQ-AX-001~005)에 대해 acceptance.md G/W/T 시나리오 통과
- 단위 테스트 커버리지 >= 85%
- LSP zero-error (`quality.yaml` lsp_quality_gates.run)
- E2E 테스트 `test_document_to_report.py` 통과
- 성능 목표 4종(`spec.md` §4)이 sandbox 환경에서 측정·기록됨

---

## 9. 후속 단계 (Out of This SPEC)

본 SPEC 통과 후 검토 후보:
- SPEC-AX-002: 평가항목 확장 (안전보건 외 1-2개 추가)
- SPEC-AX-003: Console UI (apps/console)
- SPEC-AX-004: 등급 C/D 분류 (4-class)
- SPEC-AX-005: 다중 문서 배치 처리
