# SPEC-AX-001 Implementation Strategy — Phase 1 (manager-strategy)

> **Plan mode 활성 상태 안내**: 본 문서는 원래 `.moai/specs/SPEC-AX-001/strategy.md`에 작성될 것이지만, 현재 plan 모드가 활성화되어 있어 지정된 plan 파일 경로(`/home/sklee/moai/iroum-ax/.moai/plans/enumerated-plotting-manatee-agent-af7769714dd066aa2.md`)에 작성한다. plan 모드 해제 후 orchestrator가 동일 내용을 `.moai/specs/SPEC-AX-001/strategy.md`로 이동 또는 재생성해야 한다.

- 작성일: 2026-05-14
- 작성자: manager-strategy (Opus 4.7, UltraThink 활성)
- 대상 SPEC: SPEC-AX-001 v0.1.2 (PASSED plan-auditor 0.86, evaluator-active 0.813)
- 개발 방법론: TDD (RED-GREEN-REFACTOR)
- Harness 레벨: thorough (per-sprint evaluator-active, sprint_contract 필수, playwright 활성)
- 목표: Phase 1 전략 산출 → Phase 2 RED 진입 전 scaffolding sequence·실행 순서·sprint contract·리스크 결정 사항 확정

---

## 0. Phase 1 산출물 요약 (Quick Reference)

| 항목 | 결정 사항 |
|------|----------|
| Sprint 총수 | 9 (Sprint 0 scaffolding + Sprint 1 REQ-UBI + Sprint 2-6 REQ-AX-001~005 + Sprint 7 Control Plane + Sprint 8 E2E) |
| Critical path | T-AX-007 → T-AX-008 → REQ-UBI → REQ-AX-001 → REQ-AX-002 → {REQ-AX-003, REQ-AX-004} → REQ-AX-005 → T-AX-006 → T-AX-009 |
| Top 3 risks | R-AX-004 (EXAONE 접근), R-AX-005 (GPU 부재), R-AX-003 (RAG 콜드스타트) |
| Open question 수 | 4 (orchestrator가 AskUserQuestion으로 사용자에게 확인 필요) |
| Phase 2 RED 진입 | YES (4개 open question은 모두 sensible default 보유 — orchestrator 확인 후 기본값 채택 가능) |

---

## 1. Scaffolding Sequence (Phase 2 RED 진입 전 필수)

### 1.1 원칙

- Greenfield: `.moai/`·`.claude/`·`CLAUDE.md`만 존재. `apps/`·`pipelines/`·`pkg/`·`schemas/`·`deployments/`·`tests/`·`docs/`·`Makefile`·`Dockerfile`·`docker-compose.yml`은 전부 신규 작성.
- structure.md §2 디렉토리 트리만 인용. 새 경로 추가 시 structure.md 우선 갱신.
- Sprint 0(=scaffolding sprint)에서는 RED 테스트가 없다. 빈 디렉토리·placeholder 파일·dependency manifests·CI/CD config skeleton만 생성한다.
- LSP baseline은 scaffolding 직후 0/0/0 (errors/type-errors/lint-errors) 상태에서 캡처한다 (quality.yaml plan.require_baseline=true).

### 1.2 Scaffolding 순서 (atomic step 단위)

#### Step 0 — Repository 루트 placeholder

- `.gitignore` (Go: `*.exe`, `bin/`; Python: `__pycache__/`, `.venv/`, `*.egg-info/`; Node: `node_modules/`, `.next/`; secrets: `.env`, `.env.local`; fixtures: `tests/fixtures/*.hwp` (real HWP gitignored))
- `.env.example` (`LLM_ENDPOINT`, `LLM_ENDPOINT_FALLBACK`, `VLM_ENDPOINT`, `POSTGRES_DSN`, `REDIS_URL`, `CUDA_VISIBLE_DEVICES`, `AUTH_ENABLED=false`, `LOG_LEVEL`)
- `README.md` (placeholder: 프로젝트 소개 + Quick Start docker-compose 시나리오 한 단락 + SPEC-AX-001 링크)
- `Makefile` (target: `setup`, `build`, `test`, `lint`, `docker-build`, `helm-deploy`, `proto-gen`, `db-migrate` — 각 target은 placeholder echo + actual command 주석)
- `docs/architecture.md` (placeholder, 본격 작성은 sync phase)

#### Step 1 — Schema contracts (T-AX-007, plan.md §1)

진입점: `schemas/proto/buf.yaml`

생성 파일:
- `schemas/proto/buf.yaml` (proto lint config)
- `schemas/proto/buf.gen.yaml` (Go/Python code gen)
- `schemas/proto/document.proto` (Document message: id, filename, file_type enum, parsed_text, language, parse_quality_flag, status, metadata)
- `schemas/proto/criterion.proto` (Criterion message: id, criterion_name, criterion_detail, max_points, parent_criterion_id, embedding 차원만 명시 — 실제 vector는 SQL VECTOR(768))
- `schemas/proto/simulation.proto` (Simulation: workflow_id, current_grade, target_grade, probability_a, probability_b, abstain_flag, status)
- `schemas/proto/recommendation.proto` (Recommendation: content, expected_score_delta, feasibility_score, source_benchmark_id, status, feedback)
- `schemas/proto/workflow.proto` (Workflow state machine: PENDING/RUNNING/COMPLETED/FAILED + document_id/report_id 연결)
- `schemas/openapi/openapi.yaml` (skeleton: 7개 endpoint stub — `/api/documents/upload`, `/api/criteria/index`, `/api/criteria/search`, `/api/simulations/predict`, `/api/reports/generate`, `/api/recommendations/generate`, `/api/recommendations/{id}/feedback`)
- `schemas/proto/Makefile` (`protoc-gen-go`, `protoc-gen-python` invoke)

> SPEC-AX-001 spec.md §6 handoff note: structure.md §5 데이터 스키마에 명시된 `VECTOR(1536)`은 ko-sroberta-multitask 차원과 불일치하므로 실제 SQL 작성 시 `VECTOR(768)`로 수정한다 (D12 결정사항).

#### Step 2 — Database schema (T-AX-008, plan.md §1)

진입점: `.moai/db/schema/initial.sql`

생성 파일:
- `.moai/db/schema/initial.sql`:
  - `CREATE EXTENSION IF NOT EXISTS vector;`
  - documents (id UUID PK, filename, file_type ENUM, parsed_text, language VARCHAR(8) DEFAULT 'ko', parse_quality_flag VARCHAR(64), status VARCHAR(32), metadata JSONB, created_at)
  - criteria (id UUID PK, criterion_name, criterion_detail, max_points INT, parent_criterion_id UUID FK, embedding VECTOR(768), normalization_warning JSONB, created_at)
  - reports (id UUID PK, organization_name, grade ENUM, content, score, source_benchmark_id UUID, created_at)
  - workflows (id, user_id VARCHAR(64) DEFAULT 'cli-anonymous', status ENUM, document_id FK, report_id FK, result_json JSONB, created_at, updated_at)
  - simulations (id, workflow_id FK, current_grade, target_grade, probability_a DECIMAL(4,3), probability_b DECIMAL(4,3), abstain_flag BOOLEAN, status VARCHAR(32), prediction VARCHAR(16) NULL, created_at)
  - recommendations (id, workflow_id FK, content, expected_score_delta INT, feasibility_score DECIMAL(3,2), source_benchmark_id, status VARCHAR(32), feedback JSONB, priority INT, created_at)
  - audit_logs (id UUID PK, user_id VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous', action VARCHAR(64) NOT NULL, resource_id UUID NOT NULL, resource_type VARCHAR(32), timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), details JSONB)
- HNSW 인덱스: `CREATE INDEX criteria_embedding_hnsw_idx ON criteria USING hnsw (embedding vector_cosine_ops) WITH (m=16, ef_construction=64);`
- B-tree 인덱스: documents(created_at), workflows(document_id), audit_logs(user_id, timestamp)
- `.moai/db/schema/migrations/0001_initial.sql` (Alembic 또는 raw SQL migration — TDD 모드에서 db-docs 스킬이 자동 동기화)
- `.moai/db/seeds/` (디렉토리만, 비어 있음 — Sprint 1 이후 test fixture seed 추가)

#### Step 3 — Go 모듈 (T-AX-006 선행 + T-AX-007 코드 생성 의존)

생성 파일:
- `go.mod`:
  - `module github.com/ircp/iroum-ax`
  - Go 1.22
  - deps (skeleton, 빈 require 블록 + Sprint 7에서 채움):
    - `github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.x` (tech.md §12.1 핀)
    - `google.golang.org/protobuf v1.31.x`
    - `github.com/jackc/pgx/v5` (PostgreSQL driver, structure.md §8.1 require에 명시되지 않았으나 store/postgres.go 구현용 — sprint 7에 추가)
    - `github.com/redis/go-redis/v9` (Celery broker 통신)
    - `go.uber.org/zap` (구조화 로깅)
- `go.sum` (Step 3 시점에는 빈 파일, `go mod tidy` 실행 후 채움)
- `apps/control-plane/main.go` (placeholder: `package main; func main() { /* TODO: Sprint 7 */ }`)
- `apps/control-plane/go.mod` 추가 여부 검토 — 모노레포 단일 module로 운영 권장 (structure.md §8.1 module 이름 단일 사용)

#### Step 4 — Python 모듈 (T-AX-001 ~ T-AX-005 + T-AX-009 선행)

생성 파일:
- `pyproject.toml` (Poetry 또는 PEP 621):
  - python = "^3.11"
  - core: fastapi ^0.110, uvicorn[standard] ^0.29, pydantic ^2.6, sqlalchemy ^2.0, pgvector ^0.2, celery[redis] ^5.3, asyncio
  - ML: transformers ^4.40, sentence-transformers ^2.7 (ko-sroberta-multitask), torch ^2.2 (CPU wheel 또는 cuda12), scikit-learn ^1.4, joblib ^1.4
  - LLM serving: vllm ^0.4 (optional extra `[gpu]` — CPU dev에서는 skip 가능)
  - HWP/PDF: pyhwp (or hwp-converter) ^0.1, pypdf ^4.0, pdfplumber ^0.11, hanja ^0.13.5 (한자→한글 변환)
  - dev: pytest ^8.0, pytest-asyncio ^0.23, pytest-cov ^4.1, httpx ^0.27, ruff ^0.4, mypy ^1.10, testcontainers[postgres] ^4.5
- `poetry.lock` (Step 4 시점에는 생성하지 않음, `poetry lock --no-update` 실행 후 생성)
- `pipelines/__init__.py`
- `pipelines/main.py` (placeholder FastAPI app: `from fastapi import FastAPI; app = FastAPI(title="iroum-ax pipelines")` + 7개 엔드포인트 stub returning 501 Not Implemented)
- `pipelines/config/__init__.py`, `pipelines/config/settings.py` (LLM_ENDPOINT_ALLOWLIST 환경변수 파서 + AUTH_ENABLED + DEFAULT_USER_ID='cli-anonymous'), `pipelines/config/models.py` (Pydantic 데이터 모델 stub), `pipelines/config/logging.py` (구조화 JSON logger)
- 5개 도메인 디렉토리 각각: `pipelines/{ingestion,mapping,scoring,generation,recommendation}/__init__.py` + `tests/__init__.py`
- `pipelines/workers/__init__.py`
- `pkg/models/__init__.py`, `pkg/models/{document,criterion,report,simulation}.py` (Pydantic skeleton)
- `pkg/errors/__init__.py`, `pkg/errors/custom_errors.py` (`ExternalLLMBlockedError`, `OCRConcurrencyError`, `StyleViolationError`, `BenchmarkNotAvailableError`, `IndexRebuildingError` skeleton)
- `pkg/logging/__init__.py`, `pkg/logging/logger.py` (audit_logs INSERT helper — Sprint 1에서 구현)

#### Step 5 — TypeScript 최소 스켈레톤 (Console UI excluded §13 — lint/format tooling만)

> Console UI는 Exclusion §1로 완전 제외. 그러나 모노레포 일관성을 위해 package.json + 기본 lint 도구만 scaffolding한다 (structure.md §8.3 참조).

생성 파일:
- `package.json` (root):
  - name: "iroum-ax-workspace"
  - private: true
  - workspaces: ["apps/console"] (배열만 선언, console/ 자체는 비어 있음)
  - devDependencies: prettier ^3.2, eslint ^8.57
  - scripts: `"lint:js": "echo 'console UI excluded in SPEC-AX-001'"`
- `pnpm-workspace.yaml` (packages: apps/console)
- `apps/console/.gitkeep` (디렉토리 placeholder)

#### Step 6 — Docker Compose (로컬 dev 환경)

생성 파일:
- `docker-compose.yml`:
  - service `postgres`: `pgvector/pgvector:pg16` (port 5432, env POSTGRES_DB=iroum_ax POSTGRES_USER=ax POSTGRES_PASSWORD=devpass, volume db-init: `.moai/db/schema/initial.sql:/docker-entrypoint-initdb.d/01_initial.sql`)
  - service `redis`: `redis:7-alpine` (port 6379)
  - service `vllm-qwen2vl`: placeholder `vllm/vllm-openai:latest` (GPU 미사용 환경에서는 disabled 또는 stub — `profiles: ["gpu"]` 설정으로 opt-in)
  - service `vllm-exaone`: 동일 패턴 (profiles: ["gpu"])
  - service `pipelines`: build from `pipelines/Dockerfile` (sprint 7에서 실제 빌드), port 8000, depends_on: postgres+redis
  - service `control-plane`: build from `apps/control-plane/Dockerfile`, port 8080+50051, depends_on: postgres+redis
- `pipelines/Dockerfile` (placeholder: FROM python:3.11-slim + COPY pyproject.toml + RUN pip install — 실제 빌드는 sprint 진행 중 채움)
- `apps/control-plane/Dockerfile` (placeholder: FROM golang:1.22-alpine + 멀티스테이지 빌드 + final stage scratch)

#### Step 7 — Helm Chart (dev only — prod Exclusion §13)

생성 파일:
- `deployments/helm/iroum-ax/Chart.yaml` (apiVersion v2, name iroum-ax, version 0.1.0, appVersion "0.1.0-spec-ax-001")
- `deployments/helm/iroum-ax/values.yaml` (공통 기본값: postgres image, redis image, autoscaling false, replicas 1)
- `deployments/helm/iroum-ax/values-dev.yaml` (sandbox single-node: GPU disabled, resources minimal)
- ~~`values-prod.yaml`~~ — Exclusion §13으로 생성하지 않음
- `deployments/helm/iroum-ax/templates/_helpers.tpl`
- `deployments/helm/iroum-ax/templates/control-plane-deployment.yaml` (placeholder skeleton)
- `deployments/helm/iroum-ax/templates/postgresql-statefulset.yaml`
- `deployments/helm/iroum-ax/templates/redis-statefulset.yaml`
- `deployments/helm/iroum-ax/templates/configmap.yaml` (LLM_ENDPOINT_ALLOWLIST 등)
- `deployments/helm/iroum-ax/templates/secret.yaml` (placeholder, real secret은 sealed-secrets 별도 관리)
- `deployments/helm/iroum-ax/templates/networkpolicy.yaml` (tech.md §9.1 망분리 — egress 차단 정책)

#### Step 8 — CI/CD skeleton (GitHub Actions — Exclusion §14 따라 deploy 자동화 제외, 테스트 실행까지만)

생성 파일:
- `.github/workflows/ci.yml`:
  - jobs: `lint` (ruff + mypy + buf lint + go vet + golangci-lint), `test-python` (pytest with testcontainers postgres), `test-go` (go test ./...), `proto-check` (buf breaking change detection)
  - matrix: python 3.11, go 1.22
  - **deploy job 없음** (Exclusion §14)

#### Step 9 — `.moai/sprints/SPEC-AX-001/` 디렉토리 생성 (thorough harness sprint contract 저장소)

생성 파일:
- `.moai/sprints/SPEC-AX-001/.gitkeep`
- 실제 sprint contract 파일(`sprint-REQ-UBI.md`, `sprint-REQ-AX-001.md`, …)은 evaluator-active가 sprint 시작 시 동적 생성

#### Step 10 — Scaffolding 검증 (LSP baseline 캡처)

- `make lint` 실행 → 모든 도구가 깨끗하게 통과해야 함 (skeleton 단계이므로 함수 본문이 비어 있어도 ok)
- `go mod tidy`, `poetry lock`, `buf lint` 모두 0 exit
- LSP baseline 기록: `errors=0, type_errors=0, lint_errors=0, warnings=0` (quality.yaml lsp_state_tracking.capture_points.phase_start)

---

## 2. Implementation Order (REQ Dependency DAG)

### 2.1 Topological order (Opus 4.7 adaptive reasoning)

```
[Scaffolding Sprint 0]
        ↓
[REQ-UBI (Sprint 1) — transverse invariants]
        ↓ (audit_logs + Korean lang detect + LLM allowlist 가용)
[REQ-AX-001 Document Ingestion (Sprint 2)]
        ↓ (parsed_text + documents 레코드 가용)
[REQ-AX-002 Criterion Mapping (Sprint 3)]
        ↓ (embed/retrieve API 가용)
        ├─────────────┐
        ↓             ↓
[REQ-AX-003 Grade  [REQ-AX-004 Report Draft
 Simulation         Generation (Sprint 5)]
 (Sprint 4)]              ↓
        ↓             (AX-002 retrieval만 사용)
        └─────────────┘
        ↓ (grade prediction + retrieval 결합)
[REQ-AX-005 Gap Recommendation (Sprint 6)]
        ↓
[T-AX-006 Control Plane (Sprint 7) — Go orchestration]
        ↓
[T-AX-009 E2E (Sprint 8) — Walking Skeleton 검증]
```

### 2.2 Precondition modules per REQ

| Sprint | REQ | 시작 전 GREEN 필요 module | Rationale |
|--------|-----|--------------------------|-----------|
| 1 | REQ-UBI | T-AX-008 (DB schema) + T-AX-007 (proto) | audit_logs 테이블 + Document/Workflow proto 필요 |
| 2 | REQ-AX-001 | REQ-UBI (audit_logs insert helper + language detector + LLM allowlist) | 모든 업로드는 audit 이벤트 기록 (AC-UBI-003), 한국어 감지 (AC-UBI-002) |
| 3 | REQ-AX-002 | REQ-AX-001 (documents 레코드에 평가편람 PDF 적재 가능해야 함) + REQ-UBI | criterion_parser는 documents row 기반, RAG 검색은 audit 기록 |
| 4 | REQ-AX-003 | REQ-AX-002 (retriever — 벤치마크 보고서 RAG 검색) + REQ-UBI | benchmark_learner는 retrieved 벤치마크 청크 입력, prediction은 audit 기록 |
| 5 | REQ-AX-004 | REQ-AX-002 (retriever — 5 context chunks) + REQ-UBI (LLM allowlist + audit) | prompt_builder는 retrieved context, llm_client는 외부 API 차단 게이트 |
| 6 | REQ-AX-005 | REQ-AX-002 (A-grade benchmark retrieval) + REQ-AX-003 (current grade) + REQ-UBI | gap_analyzer는 grade prediction + 벤치마크 retrieval 결합 |
| 7 | T-AX-006 Control Plane | REQ-AX-001~005 (모두 stub 이상) + T-AX-007 (proto generated code) + T-AX-008 | gRPC handler는 모든 pipeline 호출 |
| 8 | T-AX-009 E2E | T-AX-006 + REQ-AX-001~005 모두 GREEN | E2E는 control-plane API 통해 흐름 검증 |

### 2.3 병렬화 기회

- Sprint 4 (REQ-AX-003)와 Sprint 5 (REQ-AX-004)는 의존성이 둘 다 REQ-AX-002 하나라서 이론적으로 병렬 가능. 그러나 thorough harness + per-sprint evaluator scoring + 단일 manager-tdd orchestrator 환경에서는 순차 실행 권장 (병렬 실행 시 sprint contract artifact 충돌 위험).
- 추후 team mode 활성 시 implementer 2명(role: implementer + tester) isolation=worktree 로 분리하여 두 sprint 병렬 가능.

---

## 3. Sprint Contract Outline (Thorough Harness 필수)

> 각 sprint contract는 `.moai/sprints/SPEC-AX-001/sprint-{REQ-ID}.md`에 저장되며 evaluator-active가 sprint 시작 시 acceptance checklist + priority dimension + Playwright test scenario + pass condition을 생성한다. expert-frontend는 본 SPEC에서 console UI 제외 (§1)이므로 Playwright testing은 API 레벨 contract 검증(httpx + FastAPI testclient)로 대체한다.

### 3.1 Sprint 1: REQ-UBI 횡단 불변

- **Priority dimension**: **Security** (constitutional 데이터 주권 invariant)
- **Highest-risk dimension**: Security — REQ-UBI-001 외부 LLM API 차단 위반 시 컴플라이언스 즉시 실패
- **Acceptance checklist**:
  - [AC-UBI-001] `LLM_ENDPOINT_ALLOWLIST` 환경변수 파서 + `pipelines/generation/llm_client.py`에서 호스트 검증
  - [AC-UBI-002] documents.language 자동 감지 + parse_quality_flag 기록 + audit `language_warning`
  - [AC-UBI-003] 4종 액션 audit_logs INSERT 완전성 (user_id/action/resource_id/timestamp 4필드)
  - [AC-UBI-004] AUTH_ENABLED=false 시 user_id 기본값 'cli-anonymous' (NULL 0건)
- **Pass condition**:
  - 모든 4 AC G/W/T 시나리오 자동화 테스트 통과
  - audit_logs 누락 필드 0건
  - 외부 LLM API 호출 시도 시 `ExternalLLMBlockedError` 발생률 100%
  - 단위 테스트 coverage ≥ 85%
- **Evaluator 4-dim 가중치**: Security 40% / Functionality 30% / Completeness 20% / Originality 10%

### 3.2 Sprint 2: REQ-AX-001 Document Ingestion

- **Priority dimension**: **Functionality** (parse 정확도 95% + 폴백 체인)
- **Highest-risk dimension**: Functionality — HWP OLE 손상 시 자동 폴백 실패 = 입력 손실
- **Acceptance checklist**:
  - [AC-001-1] 정상 HWP 30s 이내 + 95% 정확도
  - [AC-001-2] OLE 손상 → VLM OCR 자동 폴백 (status=`ocr_fallback`)
  - [AC-001-3] 회전 PDF 셀 90% 정확도 + rotation metadata
  - [AC-001-4] 동일 문서 OCR 동시 요청 큐잉 (HTTP 202 또는 409, GPU race 방지)
  - [AC-001-5] GPU vs CPU 분기 (env-A vllm_gpu, env-B transformers_cpu, p99 < 20s/page CPU budget)
- **Pass condition**:
  - 5 AC 시나리오 통과
  - 단위/통합 coverage ≥ 85%
  - HWP OLE 손상 복원율 ≥ 80% (10건 샘플) — §7.1 Korean-specific
- **Evaluator 4-dim**: Functionality 40% / Originality 25% (Korean HWP 파싱) / Completeness 20% / Security 15%

### 3.3 Sprint 3: REQ-AX-002 Criterion Mapping (가장 큰 sprint — 6 AC)

- **Priority dimension**: **Completeness** (edge cases가 많음: cold-start, 한자 정규화, reindex)
- **Highest-risk dimension**: Completeness — 6개 AC 중 4개가 edge case
- **Acceptance checklist**:
  - [AC-002-1] top-3 p99 < 100ms + relevance ≥ 0.8
  - [AC-002-2] insufficient_context 명시 상태
  - [AC-002-3] 항목→지표→배점 계층 보존 + 한자/한글 정규화
  - [AC-002-4] HNSW 재구축 중 큐잉 또는 503
  - [AC-002-5] 한자 정규화 실패 graceful (no HTTP 500, raw fallback + confidence × 0.8)
  - [AC-002-6] cold-start 명시 응답 (no silent empty / no 500)
- **Pass condition**:
  - 6 AC 통과
  - 한자/한글 정규화율 ≥ 90%
  - 한자 정규화 실패 graceful 100%
  - cold-start 명시 응답 100%
- **Evaluator 4-dim**: Completeness 35% / Functionality 30% / Originality 20% (한국어 RAG) / Security 15%

### 3.4 Sprint 4: REQ-AX-003 Grade Simulation

- **Priority dimension**: **Functionality** (확률 수학 invariant — sum=1.0 ± 0.001 + abstain 분기)
- **Highest-risk dimension**: Functionality — abstain 로직 잘못 시 mathematical contradiction (NEW-F1 evaluator-active iteration 3 발견)
- **Acceptance checklist**:
  - [AC-003-1] {A, B, abstain} sum=1.0 ± 0.001 + 1s 응답 + simulations 저장
  - [AC-003-2] max(p_a, p_b) < 0.5 → status=low_confidence, prediction=null, candidates 반환, downstream 차단
  - [AC-003-3] 학습 중 HTTP 503 model_training
- **Pass condition**:
  - 3 AC 통과
  - 확률 sum 검증 100건 batch에서 0 위반
  - 등급 예측 일치율 ≥ 80% (사람 검수 벤치마크, A/B 2-class)
- **Evaluator 4-dim**: Functionality 50% / Completeness 25% / Security 15% / Originality 10%

### 3.5 Sprint 5: REQ-AX-004 Report Draft Generation

- **Priority dimension**: **Originality** (한국어 공문 합니다체 enforcement) + Functionality
- **Highest-risk dimension**: Originality — style_applier 검증 미흡 시 KEPCO E&C 도메인 신뢰도 직격
- **Acceptance checklist**:
  - [AC-004-1] 합니다체 + 5s + style_applier 통과 + 주관 4/5
  - [AC-004-2] 스타일 위반 → 최대 3회 재시도 → style_violation
  - [AC-004-3] EXAONE 3회 실패 → Qwen 2.5 자동 fallback (외부 API escalate 금지)
- **Pass condition**:
  - 3 AC 통과
  - 공문 합니다체 일관성 ≥ 95%
  - 외부 API 호출 시도 0건
  - EXAONE 미가용 시 Qwen-only 경로로 AC-004-1 happy path 검증 가능 (decision §4.1 참조)
- **Evaluator 4-dim**: Originality 35% / Functionality 30% / Security 25% (LLM allowlist) / Completeness 10%

### 3.6 Sprint 6: REQ-AX-005 Gap Recommendation

- **Priority dimension**: **Functionality** (3-5개 우선순위 항목 + feasibility 정렬)
- **Highest-risk dimension**: Functionality — fabricated content 생성 금지 (AC-005-2)
- **Acceptance checklist**:
  - [AC-005-1] 3-5 항목 + {content, expected_score_delta, feasibility_score, source_benchmark_id} + 3s 응답 + feasibility desc 정렬
  - [AC-005-2] benchmark 없으면 empty list + benchmark_not_available (fabricated 금지)
  - [AC-005-3] not_feasible 피드백 → priority downgrade + alternative 제안 + DB persist
- **Pass condition**:
  - 3 AC 통과
  - fabricated content 0건 (벤치마크 미존재 시)
- **Evaluator 4-dim**: Functionality 40% / Completeness 30% / Originality 20% / Security 10%

### 3.7 Sprint 7: T-AX-006 Control Plane Workflow (통합)

- **Priority dimension**: **Functionality** (Go state machine + gRPC handler)
- **Highest-risk dimension**: Functionality — Workflow.Transition 잘못 시 entire workflow 멈춤
- **Acceptance checklist**:
  - Workflow state machine 4 state 전이 (PENDING → RUNNING → COMPLETED/FAILED)
  - gRPC/REST handler가 pipelines API 정확히 호출
  - Celery dispatcher가 ingestion/generation/simulation worker 큐 발행
  - postgres.go가 documents/workflows/audit_logs CRUD 수행
- **Pass condition**:
  - state_machine_test.go 통과
  - test_control_plane_integration.py 통합 테스트 통과
  - golangci-lint 0 error
- **Evaluator 4-dim**: Functionality 50% / Security 25% / Completeness 15% / Originality 10%

### 3.8 Sprint 8: T-AX-009 E2E (Walking Skeleton 검증)

- **Priority dimension**: **Completeness** (전체 슬라이스 통과)
- **Highest-risk dimension**: Completeness — 단일 슬라이스가 실패하면 본 SPEC 전체 실패
- **Acceptance checklist**:
  - `tests/e2e/test_document_to_report.py`가 KEPCO E&C 샘플 HWP → 평가편람 indexing → 초안 생성 → 등급 예측 → 추천 전체 E2E 통과
  - 24 AC 시나리오 자동화 테스트 모두 PASS
  - 성능 4종 (OCR, RAG, draft, prediction) 측정·기록
  - Korean-specific 품질 6종 측정·기록
- **Pass condition**:
  - 24/24 AC PASS
  - 단위+통합+E2E coverage ≥ 85%
  - LSP errors=0, type-errors=0, lint-errors=0
  - TRUST 5 모든 차원 통과
- **Evaluator 4-dim**: Completeness 40% / Functionality 30% / Security 20% / Originality 10%

---

## 4. Risk-Driven Decisions (Opus 4.7 Trade-off Reasoning)

### 4.1 R-AX-004: EXAONE 3.5 접근 불확실성

**현황**: tech.md §3.3 EXAONE 3.5는 LG AI 협력 후 공개 (_TBD). 본 strategy 작성 시점에는 가용성 미확정.

**Trade-off matrix** (가중치: Risk 30% / Implementation Cost 25% / Performance 20% / Maintainability 15% / Scalability 10%):

| 선택지 | Risk | Cost | Performance | Maint | Scale | 가중점수 |
|--------|------|------|-------------|-------|-------|---------|
| (A) EXAONE 가용 전제로 RED 작성 | 2/10 | 7/10 | 9/10 | 7/10 | 6/10 | **5.45** |
| (B) Qwen 2.5 only + EXAONE optional path | 8/10 | 8/10 | 7/10 | 8/10 | 8/10 | **7.75** |
| (C) EXAONE 가용 시점까지 sprint 5 보류 | 6/10 | 4/10 | 5/10 | 6/10 | 5/10 | 5.20 |

**결정**: (B) — Qwen 2.5 7B를 1차 GREEN 경로로 채택. EXAONE은 REQ-AX-004-O1 Optional 조건절을 활용하여 `LLM_PRIMARY_MODEL` 환경변수로 가용 시 EXAONE 호출, 미가용 시 Qwen 2.5 fallback.

**GREEN criterion (정확한 정의)**:
- AC-004-1 (Happy)는 환경변수 `EXAONE_AVAILABLE=true`일 때만 EXAONE 경로로 검증. `EXAONE_AVAILABLE=false`일 때는 Qwen 2.5 응답이 합니다체+style_applier 통과+5s를 만족하면 GREEN. response metadata `model_used`가 환경에 따라 다름은 정상 동작.
- AC-004-3 (Fallback)은 EXAONE 가용 여부와 무관하게 EXAONE 엔드포인트 mock 5xx 응답 후 Qwen fallback 동작 검증 — 항상 검증 가능.
- AC-004-2 (Style violation)는 LLM 응답 mock으로 검증 — 항상 검증 가능.

**Cognitive Bias Check (anchoring)**: tech.md가 EXAONE을 1차로 명시하여 anchoring bias 위험. (B)를 채택해도 spec 위반은 아니다 (REQ-AX-004-O1이 fallback을 명시).

### 4.2 R-AX-005: GPU 부재 환경

**현황**: tech.md §6.1 GPU _TBD. 사용자 dev machine GPU 가용성 미확인 (Open Question §8.4).

**결정 트리**:
- AC-001-5는 **명시적으로** env-A (GPU) + env-B (CPU) 분기를 모두 검증해야 함.
- CI/CD 환경 (GitHub Actions 기본 러너)에는 GPU 없음 → env-B 경로 CI에서 자동 검증.
- env-A (GPU) 검증은 dev machine 또는 sandbox K8s GPU 노드에서 수동 실행.

**CPU-fallback test 전략**:
- `pipelines/ingestion/vlm_processor.py`에서 `torch.cuda.is_available()` 체크 후 backend 결정.
- CPU mode: `transformers` library 직접 사용 (vllm 미사용) + `device_map="cpu"` + `torch.float32`.
- CPU 테스트 budget: 5페이지 합성 HWP × 20s/page = 100s. pytest mark `@pytest.mark.slow_cpu` 부여, CI에서는 `--timeout=180` 적용.
- GPU 테스트 (env-A): `pytest -m gpu`로 opt-in, GPU 검출 시에만 실행.

**Performance budget 완화 (CPU)**:
- README에 명시: "GPU 미가용 환경에서는 OCR p99 < 20s/page (5-10× 완화), AC-001-5 env-B branch 참조"
- §7.1 Korean-specific 품질 기준은 GPU 환경 기준이며 CPU 환경에서는 운영 가능성 검증용으로만 사용.

### 4.3 R-AX-001: HWP Parsing Edge Cases

**Fixture 분류**:

| AC | Fixture | 출처 | 커밋 여부 |
|----|---------|------|----------|
| AC-001-1 | 50페이지 KEPCO E&C HWP | 실제 고객 자료 | **gitignore** (`tests/fixtures/*.real.hwp`). CI에서는 합성 5페이지 안전보건 HWP로 대체. |
| AC-001-2 | OLE 구조 손상 HWP | 합성 (정상 HWP의 OLE 헤더를 의도적으로 손상) | **커밋** (`tests/fixtures/corrupted_ole.hwp`) |
| AC-001-3 | 회전 표 PDF | 합성 (matplotlib + pdf rotation) | **커밋** (`tests/fixtures/rotated_table.pdf`) |
| AC-001-4 | (불필요) | Celery mock | N/A |
| AC-001-5 | env-A/B 공통 HWP | 합성 5페이지 안전보건 HWP | **커밋** (`tests/fixtures/synthetic_safety_5page.hwp`) |
| AC-002-1, AC-002-3 | 평가편람 PDF | 실제 또는 합성 | 합성 우선 커밋, 실제 미커밋 |
| AC-002-6 | 빈 인덱스 | 런타임 생성 | N/A |
| AC-003-1, AC-005-1 | A/B 등급 벤치마크 HWP | 합성 (서로 다른 점수 패턴) | 커밋 |

**합성 fixture 작성 책임**: Sprint 8 (T-AX-009) 초입 + Sprint 2 (T-AX-001) RED phase 시점에 expert-testing 에이전트가 fixture generator script(`tests/fixtures/generate_fixtures.py`) 작성.

### 4.4 R-AX-007: pgvector + Korean Embeddings

**문제**: 첫 TDD iteration에 PostgreSQL + pgvector 컨테이너 필요 여부.

**Trade-off**:
- 옵션 A: Pure unit test + `FakeVectorStore` (in-memory dict, top-k는 cosine similarity 계산) → 빠르지만 HNSW behavior 미검증
- 옵션 B: testcontainers-python으로 매 테스트 postgres+pgvector 컨테이너 부팅 → 느리지만 실제 HNSW 검증
- 옵션 C: docker-compose up 전제 + 모든 테스트가 외부 postgres 사용 → 가장 느림, CI 환경 친화 X

**결정**:
- **단위 테스트**: FakeVectorStore (옵션 A) 사용. `pipelines/mapping/vector_store.py`는 추상 인터페이스 + 두 가지 구현(`PgVectorStore`, `FakeVectorStore`).
- **통합 테스트** (`pipelines/mapping/tests/test_vector_store_integration.py`): testcontainers-python (옵션 B). pytest fixture로 session-scoped postgres container 1개 공유.
- **E2E 테스트** (Sprint 8): docker-compose up 전제. `tests/e2e/conftest.py`에서 docker-compose up + wait-for-port 후 fixture 진입.
- **AC-002-1 (p99 < 100ms)**: 통합 테스트(testcontainers)에서만 측정, FakeVectorStore는 latency 검증 제외.
- **AC-002-4 (HNSW reindex)**: 통합 테스트 전용. FakeVectorStore에는 reindex 상태 시뮬레이션 추가.

---

## 5. LSP Baseline Strategy

### 5.1 Greenfield baseline

Scaffolding 직후 시점 (`make lint` 통과 후):
```
errors: 0
type_errors: 0
lint_errors: 0
warnings: 0
```

이를 `.moai/specs/SPEC-AX-001/lsp-baseline.json`에 저장 (quality.yaml `capture_points: phase_start` 트리거).

### 5.2 언어별 threshold (Run phase 동안 0/0/0 유지)

| 언어 | Errors | Type errors | Lint errors | 도구 |
|------|--------|-------------|-------------|------|
| Python | 0 | 0 (mypy strict) | 0 (ruff F+E+W+I+B) | mypy ^1.10 + ruff ^0.4 |
| Go | 0 | 0 (go vet 포함) | 0 (golangci-lint default + gosec) | go vet + golangci-lint v1.59 |
| Protobuf | 0 | N/A | 0 (buf lint default + breaking change detection) | buf ^1.32 |
| TypeScript | 0 | 0 (tsc --noEmit) | 0 (eslint) | tsc ^5.4 + eslint ^8.57 (console excluded이지만 root tooling은 유지) |
| SQL | N/A | N/A | N/A | sqlfluff (optional, dialect=postgres) |

### 5.3 Regression policy (quality.yaml run.allow_regression=false)

- 각 sprint 종료 시 `moai lsp status` 실행 → 0/0/0 유지 확인.
- 실패 시 즉시 차단, 해당 sprint REFACTOR 단계로 회귀.
- LSP cache TTL 5s + timeout 3s (quality.yaml 기본값).

### 5.4 ast-grep gate (quality.yaml ast_grep_gate.enabled=true)

- `.moai/config/astgrep-rules/` 디렉토리는 scaffolding 시점에 존재. 본 SPEC 진행 중 룰 추가는 보수적으로.
- `block_on_error=true` → 에러 매치 0개 유지 필수.

---

## 6. Phase 2 Sub-Sprint Plan

### 6.1 Sprint 순서 (단계 순서 — 시간 추정 금지)

```
Sprint 0: Scaffolding
   ├─ §1 Step 0~10 모두 수행
   ├─ LSP baseline 캡처 (0/0/0)
   └─ NO RED tests (TodoWrite로 manager-strategy 진행 추적)

Sprint 1: REQ-UBI (Priority: High)
   ├─ Sprint Contract 생성 (evaluator-active) → §3.1
   ├─ RED: tests/integration/test_audit_logs_completeness.py, test_external_llm_blocked.py, test_korean_language_detect.py, test_sandbox_user_id_default.py 작성, 실패 확인
   ├─ GREEN: pkg/logging/logger.py audit helper + pipelines/config/settings.py allowlist + pipelines/ingestion 한국어 감지 + pipelines/config DEFAULT_USER_ID
   ├─ REFACTOR: @MX:ANCHOR pkg/logging.audit_event, @MX:NOTE settings.LLM_ENDPOINT_ALLOWLIST 정규화
   ├─ Quality gate: manager-quality + LSP 0/0/0
   └─ Evaluator-active scoring (per-sprint, strict profile) → pass (≥ 0.75)

Sprint 2: REQ-AX-001 (Priority: High)
   ├─ Sprint Contract 생성 → §3.2
   ├─ RED: 5 AC × pytest cases (hwp_parser/vlm_processor/table_extractor/ingestion_worker)
   │      - test_normal_hwp_parsing.py, test_ole_corruption_fallback.py, test_rotated_pdf_table.py,
   │        test_concurrent_ocr_queue.py, test_gpu_cpu_branching.py
   ├─ GREEN: 5개 ingestion 모듈 최소 구현
   ├─ REFACTOR: @MX:ANCHOR hwp_parser.parse_document, @MX:WARN vlm_processor.batch_inference (REASON: GPU memory)
   ├─ Quality + LSP + evaluator-active scoring
   └─ Re-planning gate 체크 (3+ stagnation 시 manager-strategy 재호출)

Sprint 3: REQ-AX-002 (Priority: High, 가장 큰 sprint)
   ├─ Sprint Contract → §3.3
   ├─ RED: 6 AC × tests (criterion_parser/embedding_service/vector_store/retriever)
   │      - test_topk_search.py, test_insufficient_context.py, test_hierarchy_preservation.py,
   │        test_hnsw_rebuild_queueing.py, test_hanja_graceful_degradation.py, test_cold_start_bootstrap.py
   ├─ GREEN: 4개 mapping 모듈 + FakeVectorStore + PgVectorStore (testcontainers)
   ├─ REFACTOR: @MX:ANCHOR embedding_service.embed_text + retriever.search_topk
   ├─ Quality + LSP + evaluator-active scoring
   └─ AC-002-1 p99 측정 (testcontainers 환경)

Sprint 4: REQ-AX-003 (Priority: Medium)
   ├─ Sprint Contract → §3.4
   ├─ RED: 3 AC × tests (benchmark_learner/grade_predictor/scenario_simulator)
   │      - test_probability_sum_invariant.py, test_low_confidence_abstain.py, test_training_state_block.py
   ├─ GREEN: scikit-learn LogisticRegression + abstain 로직 (max < 0.5 → abstain=True)
   ├─ REFACTOR: @MX:NOTE grade_predictor.predict abstain 분기 mathematical invariant
   └─ Quality + LSP + evaluator-active

Sprint 5: REQ-AX-004 (Priority: High)
   ├─ Sprint Contract → §3.5
   ├─ RED: 3 AC × tests
   │      - test_happy_draft_generation.py (EXAONE 또는 Qwen, env-conditional),
   │        test_style_violation_retry.py, test_exaone_qwen_fallback.py
   ├─ GREEN: llm_client (allowlist + fallback chain) + prompt_builder + style_applier
   ├─ REFACTOR: @MX:ANCHOR llm_client.generate, @MX:WARN workers/generation_worker.py retry loop (REASON: max 3 retries per REQ-AX-004-U1)
   └─ Quality + LSP + evaluator-active

Sprint 6: REQ-AX-005 (Priority: Medium)
   ├─ Sprint Contract → §3.6
   ├─ RED: 3 AC × tests (gap_analyzer/content_suggester/prioritizer)
   │      - test_recommendation_happy.py, test_benchmark_not_available.py, test_not_feasible_feedback.py
   ├─ GREEN: gap_analyzer (현재 vs 목표 retrieval diff) + content_suggester + prioritizer (feasibility 정렬)
   ├─ REFACTOR: @MX:NOTE prioritizer 실현 가능성 가중치
   └─ Quality + LSP + evaluator-active

Sprint 7: T-AX-006 Control Plane (Priority: High)
   ├─ Sprint Contract → §3.7
   ├─ RED: Go test (state_machine_test.go) + Python integration test (test_control_plane_integration.py)
   ├─ GREEN: state_machine.go + handlers.go + dispatcher.go + postgres.go
   ├─ REFACTOR: @MX:ANCHOR Transition, @MX:WARN scheduler/dispatcher.go goroutines (REASON: workflow timeout context cancellation)
   └─ Quality + LSP + evaluator-active

Sprint 8: T-AX-009 E2E (Priority: High)
   ├─ Sprint Contract → §3.8
   ├─ RED: tests/e2e/test_document_to_report.py
   ├─ GREEN: docker-compose up 전제 + 전체 슬라이스 통과
   ├─ Performance 측정 4종 + Korean-specific 6종 기록
   └─ Final evaluator-active CONFIRM (≥ 0.75) → sync phase 진입 허가
```

### 6.2 Re-planning Gate (spec-workflow.md 참조)

각 sprint REFACTOR 종료 후 `.moai/specs/SPEC-AX-001/progress.md`에 다음 기록:
- 완료된 AC 수 (cumulative)
- 직전 sprint 대비 신규 AC 통과 수
- LSP error count delta
- Coverage delta

Trigger conditions:
- 3+ 연속 iteration zero new AC met → manager-strategy 재호출 (이번 strategy 갱신)
- Coverage 직전 대비 하락 → 재호출
- new errors > fixed errors per cycle → 재호출
- Agent가 명시적으로 SPEC 충족 불가 보고 → 재호출

### 6.3 Drift Guard

각 sprint REFACTOR 후 자동 실행:
- planned files (본 strategy §1, plan.md §1 출력 파일 목록) vs actual modified files 비교
- 누적 drift > 30% → Re-planning Gate 트리거

---

## 7. Specialist Routing

| Sprint | Owner Agent | 보조 Agent | 비고 |
|--------|------------|-----------|------|
| Sprint 0 (Scaffolding) | manager-strategy (현 agent) | expert-devops (docker-compose + Helm + Makefile), expert-backend (go.mod + pyproject.toml + proto + initial.sql) | 코드 작성 phase 아님 — orchestrator가 직접 호출 또는 본 strategy 기반 spawn |
| Sprint 1 (REQ-UBI) | manager-tdd | expert-backend (Python — DB middleware + audit helper + settings allowlist) | 횡단 invariant — 모든 후속 sprint에서 의존 |
| Sprint 2 (REQ-AX-001) | manager-tdd | expert-backend (Python — Korean HWP parsing 도메인 지식) | hwp-converter + Qwen2-VL transformers/vllm integration |
| Sprint 3 (REQ-AX-002) | manager-tdd | expert-backend (Python — RAG/pgvector), expert-testing (testcontainers integration) | 가장 큰 sprint, 6 AC |
| Sprint 4 (REQ-AX-003) | manager-tdd | expert-backend (Python — ML scikit-learn), expert-performance (확률 sum invariant 검증) | abstain 로직 정확성 |
| Sprint 5 (REQ-AX-004) | manager-tdd | expert-backend (Python — LLM client + fallback), expert-security (LLM allowlist 검증) | 한국어 공문 스타일 |
| Sprint 6 (REQ-AX-005) | manager-tdd | expert-backend (Python — gap analyzer 알고리즘) | fabricated content 방지 |
| Sprint 7 (T-AX-006) | manager-tdd | expert-backend (Go — gRPC + state machine + pgx) | Go specialty |
| Sprint 8 (T-AX-009) | manager-tdd | expert-testing (E2E pytest + fixture generation), expert-performance (성능 측정) | 전체 검증 |
| 매 sprint 종료 후 | manager-quality | (LSP gate + ast-grep gate + TRUST 5 검증) | quality.yaml 강제 |
| 매 sprint 종료 후 | evaluator-active | (per-sprint scoring, strict profile, 4-dim) | thorough harness 필수 |
| 모든 sprint 완료 후 | manager-git | (conventional commit + branch 정리) | 본 SPEC 종료 시 |
| ~~Console UI~~ | ~~expert-frontend~~ | — | **SKIP** — Exclusion §1 |

**Worktree 정책** (sub-agent mode 가정, team mode 비활성):
- expert-backend가 write-heavy로 호출될 때마다 `isolation: "worktree"` 적용 (cross-file 변경)
- expert-testing read-only 분석 시 isolation 없이 호출
- expert-security read-only audit 시 isolation 없이 호출

---

## 8. Open Questions (Phase 2 RED 진입 전 orchestrator 확인 필요)

> [HARD] manager-strategy(본 agent)는 AskUserQuestion 호출 금지. 아래 4개 질문은 orchestrator가 AskUserQuestion으로 사용자에게 확인한다.
>
> 각 질문에 대한 **권장 기본값(default)이 존재**하므로, 사용자 응답 없이도 Phase 2 RED 진입 가능 (Ready=YES). 사용자 응답이 있으면 default 대신 사용자 선택 채택.

### Q1: Local docker-compose 실행 정책

**질문**: 단위 테스트 실행 전 `docker-compose up`을 사용자가 수동으로 띄울 것인지, 또는 testcontainers-python으로 테스트 fixture가 자동 컨테이너 부팅할 것인지?

- 옵션 A (권장 default): **testcontainers-python**. 단위 테스트는 FakeVectorStore. 통합 테스트는 testcontainers가 자동으로 postgres+pgvector 컨테이너 부팅. E2E만 docker-compose up 전제.
- 옵션 B: docker-compose up 사용자 수동. 통합 테스트도 외부 postgres에 연결.
- 옵션 C: Mock 위주. PgVector는 mock 추상 인터페이스로만 검증, 실제 HNSW 검증은 sprint 8 E2E에서만.

### Q2: EXAONE 3.5 가용성

**질문**: EXAONE 3.5 모델 파일 또는 vLLM 엔드포인트가 현재 로컬에서 가용한가?

- 옵션 A (권장 default): **Qwen 2.5 7B만으로 진행** (EXAONE 미가용 전제). REQ-AX-004-O1 fallback path가 1차 경로. AC-004-1 happy path는 Qwen 2.5로 합니다체+5s 검증. EXAONE 가용 시 환경변수 EXAONE_AVAILABLE=true로 활성.
- 옵션 B: EXAONE 3.5 LG 협력 모델 다운로드 시점까지 Sprint 5 보류, Sprint 4까지 진행 후 대기.
- 옵션 C: EXAONE 3.5 가용 (사용자가 endpoint URL 제공) — AC-004-1을 EXAONE 경로로 검증.

### Q3: Test fixture 저장 정책

**질문**: KEPCO E&C 실제 HWP 샘플과 평가편람 PDF의 저장 위치 및 커밋 정책?

- 옵션 A (권장 default): **합성 fixture만 커밋**. 실제 KEPCO E&C HWP/PDF는 `tests/fixtures/*.real.hwp` 패턴으로 gitignore. CI는 합성 5페이지 안전보건 HWP + 합성 평가편람 PDF로 검증. AC-001-1 50페이지 정확도는 사용자 수동 검증 단계로 이관.
- 옵션 B: 실제 KEPCO E&C HWP를 PII 마스킹 후 커밋. AC-001-1 자동화 가능.
- 옵션 C: Git LFS로 실제 fixture 저장. private repo 전제.

### Q4: GPU 가용성

**질문**: 현재 dev machine 또는 sandbox 환경에 NVIDIA GPU가 있는가?

- 옵션 A (권장 default): **GPU 미가용 전제**. CI/dev 모두 CPU 추론. AC-001-5 env-A (GPU) 검증은 `pytest -m gpu` opt-in으로 deferred. README에 CPU 환경 budget 5-10× 완화 명시.
- 옵션 B: GPU 가용 (CUDA 12.x). env-A 자동 검증 + p99 < 2s/page enforced.
- 옵션 C: K8s sandbox에 단일 GPU. dev machine은 CPU. sandbox 배포 후에만 env-A 검증.

---

## 9. Philosopher Framework 적용 (manager-strategy 본질)

### 9.1 Assumption Audit

| Assumption | Confidence | Risk if Wrong |
|------------|-----------|--------------|
| Greenfield — `apps/`·`pipelines/` 등 빈 디렉토리 | High | Low (사용자 직접 확인) |
| TDD mode 채택 (quality.yaml) | High | Low |
| thorough harness 채택 (sprint contract 필수) | High | Medium (sprint contract 미작성 시 evaluator 차단) |
| EXAONE 3.5 미가용 default | Medium | Medium (가용 확인 시 §4.1 (A) 채택 가능) |
| GPU 미가용 default | Medium | Low (env-B 경로가 항상 1차) |
| ko-sroberta-multitask 768차원 (structure.md §5 VECTOR(1536) ≠ 실제) | High | Medium (D12 handoff note 미적용 시 DB schema 불일치) |
| Console UI 완전 제외 (§1) — TS scaffolding은 root tooling만 | High | Low |
| Helm prod values 미작성 (§13) | High | Low |
| CI/CD 배포 자동화 제외 (§14) | High | Low |

### 9.2 First Principles — Five Whys (안전보건 PoC Walking Skeleton)

- Surface: KEPCO E&C 경영평가팀 인건비 절감 + 등급 상향
- Why 1: 1년 풀타임 보고서 작성을 단축 사이클로 단축
- Why 2: 평가기준 매핑 + 초안 생성 + 등급 시뮬 + recommendation 자동화
- Why 3: 한국어 + HWP + 망분리 + 공문 스타일이 기존 솔루션에서 미충족
- Why 4: 도메인 특화 PoC로 anchor 고객 확보 후 platform 확장
- **Root**: 한국 공공 규제 도메인 자동화의 표준 인프라 확보 (제품 비전)

### 9.3 Alternative Generation (구현 순서)

- **Conservative (low risk, incremental)**: REQ-AX-001 → 002 → 003 → 004 → 005 순차 (본 strategy 채택)
- **Balanced (moderate risk)**: REQ-AX-002를 가장 먼저 (RAG가 모든 후속에 필요) + 평가편람 indexing을 REQ-AX-001 전에 → 거부 (AX-001 ingestion이 평가편람 PDF도 처리하므로 의존성 뒤집힘)
- **Aggressive (transformative)**: 5 REQ 병렬 + control-plane 우선 → 거부 (greenfield + thorough harness에서 병렬 sprint는 contract 충돌 위험)

**채택: Conservative** — sprint contract artifact 충돌 방지 + per-sprint evaluator scoring 일관성.

### 9.4 Cognitive Bias Check

- **Anchoring bias**: tech.md가 EXAONE 1차로 명시 → §4.1에서 Qwen-first로 의도적 안티-anchor 결정.
- **Confirmation bias**: spec.md가 통과(0.86 + 0.813)했으므로 별도 검증 생략 위험 → §3 sprint contract마다 evaluator-active re-validation 강제.
- **Sunk cost**: scaffolding 단계에서 잘못된 선택 시 후속 sprint에서 비용 가속 → Sprint 0 종료 시 LSP baseline + drift guard로 조기 감지.
- **Overconfidence**: thorough harness가 자동으로 품질 보장한다는 가정 → §6.2 Re-planning Gate + §6.3 Drift Guard로 명시적 정지 trigger 정의.

**This option might fail because**:
- (a) testcontainers-python이 사용자 dev 환경 (WSL2)에서 docker-in-docker 이슈로 실패 가능 → fallback: docker-compose up 수동.
- (b) Qwen 2.5 7B가 한국어 공문 합니다체 일관성 95% 미달 가능 → mitigation: style_applier 재시도 루프 + fine-tuning 후속 SPEC.
- (c) pgvector HNSW 초기 튜닝 미흡으로 p99 < 100ms 미달 가능 → mitigation: AC-002-1 측정 후 ef_construction/m 재조정.

---

## 10. Definition of Done (Phase 1 본 strategy)

- [x] `.moai/plans/enumerated-plotting-manatee-agent-af7769714dd066aa2.md` (본 파일) 작성 완료
- [ ] (plan mode 해제 후) 본 내용을 `.moai/specs/SPEC-AX-001/strategy.md`로 이전 또는 재생성
- [x] Scaffolding 10단계 정의
- [x] REQ DAG topological order 정의
- [x] 8개 sprint contract outline (REQ-UBI + 5 REQ-AX + 2 통합)
- [x] 4개 risk-driven decision (EXAONE, GPU, HWP fixture, pgvector test 전략)
- [x] LSP baseline strategy
- [x] Phase 2 sub-sprint plan + Re-planning Gate + Drift Guard
- [x] Specialist routing matrix
- [x] 4개 open question with sensible defaults
- [x] Philosopher Framework (Assumption / First Principles / Alternative / Bias check)

---

## 11. RETURN to Orchestrator (≤ 300 words)

- **File created**: `/home/sklee/moai/iroum-ax/.moai/plans/enumerated-plotting-manatee-agent-af7769714dd066aa2.md` (plan mode 활성으로 인해 원래 `.moai/specs/SPEC-AX-001/strategy.md` 대신 plan 파일에 작성. plan mode 해제 후 orchestrator가 동일 내용을 spec 경로로 이동·재생성 필요.)
- **Total sprint count**: 9 (Sprint 0 scaffolding + Sprint 1 REQ-UBI + Sprint 2-6 REQ-AX-001~005 + Sprint 7 Control Plane + Sprint 8 E2E)
- **Critical path REQ ordering**: T-AX-007 (proto) → T-AX-008 (DB) → REQ-UBI → REQ-AX-001 → REQ-AX-002 → {REQ-AX-003, REQ-AX-004 (병렬 가능, but 순차 권장)} → REQ-AX-005 → T-AX-006 Control Plane → T-AX-009 E2E
- **Top 3 risks + mitigation**:
  1. **R-AX-004 EXAONE 3.5 접근**: Qwen 2.5 7B를 1차 GREEN, EXAONE은 환경변수 opt-in (Trade-off matrix B 선택, 점수 7.75).
  2. **R-AX-005 GPU 부재**: CPU 경로가 CI baseline (AC-001-5 env-B), GPU 경로는 `pytest -m gpu` opt-in. CPU budget 5-10× 완화 명시.
  3. **R-AX-003 RAG 콜드스타트**: 합성 평가편람 fixture + AC-002-6 cold-start 명시 응답 + 한자/한글 graceful fallback (AC-002-5).
- **Open questions count**: 4 (모두 sensible default 보유, orchestrator AskUserQuestion 1라운드로 확정 가능):
  1. testcontainers vs docker-compose 수동
  2. EXAONE 가용성
  3. HWP fixture 저장 정책
  4. GPU 가용성
- **Ready for Phase 2 RED entry**: **YES** (4 open question은 sensible default로 진행 가능. orchestrator가 사용자 확정 후 Sprint 0 Scaffolding 직접 spawn — expert-devops + expert-backend 병렬 호출).

