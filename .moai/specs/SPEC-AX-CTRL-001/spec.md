---
id: SPEC-AX-CTRL-001
version: 0.1.2
status: draft
created: 2026-05-14
updated: 2026-05-14
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.2 (2026-05-14): evaluator-active 교차 검증 후속 보정. D11(AC 카운트 25→26, acceptance.md §9 + spec-compact.md 일관성) + D12(AC-CTRL-UBI-002-B 액션명 'WORKFLOW_STATE_TRANSITION OR ...' 이중 표기 → 단일 'WORKFLOW_TRANSITIONED_TO_RUNNING' 통일, research.md §2.2 enumeration 정합). D13/F1은 비차단 info → Sprint S3 이전 후속 처리 예정. (작성자: ircp)
- 0.1.1 (2026-05-14): plan-auditor iter 1 리뷰 반영(iter 2 수정). D1/D2 Major 결함 해결을 위해 REQ-CTRL-UBI-001/UBI-002 전용 AC 추가(AC-CTRL-UBI-001 트랜잭션 원자성, AC-CTRL-UBI-002-A/B/C 감사 일관성 + cli-anonymous 기본값). D3 broken citation 수정(plan.md L367 → `.claude/skills/moai/workflows/plan.md:366` Composite domain rules). D4 AC 헤딩 일관성 정리("Edge Case " 접두사 제거). D5 Celery dispatch latency p99 전용 AC(AC-CTRL-005-4) 추가. D6 Prometheus Optional AC(AC-CTRL-002-4) 추가. D7 REST 게이트웨이 startup AC(AC-CTRL-003-4) 추가. D8 PostgreSQL mid-tx 실패 AC(AC-CTRL-004-4) 추가. Total AC: 18 → 25. spec-compact.md 재생성. (작성자: ircp)
- 0.1.0 (2026-05-14): SPEC-AX-001 Sprint 7 (T-AX-006 Control Plane)을 별도 SPEC으로 분리한 첫 초안. iroum-ax Go control plane Walking Skeleton 정의 — gRPC + REST 서버, 워크플로우 상태머신, PostgreSQL store, Celery dispatch via Redis. SPEC-AX-001(Python 파이프라인 PASSED + GREEN)이 정의한 5개 REQ-AX(Document Ingestion → Mapping → Scoring → Generation → Recommendation)의 오케스트레이션 계층. Console UI·인증·멀티테넌트·재시도 정책 의도적 제외. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field 정의(id, version, status, created, updated, author, priority, issue_number)를 따른다.

---

# SPEC-AX-CTRL-001 — Go Control Plane Walking Skeleton

## 1. 개요

`apps/control-plane/`(Go 1.22+)이 SPEC-AX-001의 Python 파이프라인(`pipelines/`)을 오케스트레이션하는 Walking Skeleton을 정의한다. Console UI(`apps/console/`)·인증·멀티테넌트 없이, **CLI/gRPC/REST 클라이언트가 워크플로우를 생성·조회·완료할 수 있는 최소 실행 가능한 오케스트레이션 계층**을 제공한다.

### 1.1 Walking Skeleton의 의미 (본 SPEC 범위)

- 단일 워크플로우 타입: `document_processing` (REQ-AX-001~005 E2E 슬라이스)
- 단일 상태머신: PENDING → RUNNING → COMPLETED | FAILED (4 states, 3 transitions)
- 단일 dispatch 경로: Go → Redis(Celery 프로토콜) → Python Celery worker
- 단일 데이터 스토어: PostgreSQL `workflows` + `audit_logs` 테이블 (documents/reports/criteria/simulations/recommendations 테이블은 Python pipelines가 관리, SPEC-AX-001 범위)
- 인증 없음: 모든 호출은 `user_id="cli-anonymous"` (SPEC-AX-001 REQ-UBI-003 State-driven 절과 정합)

### 1.2 Anchor 컨텍스트

본 SPEC은 `.moai/specs/SPEC-AX-001/strategy.md` §3.7 Sprint 7 (T-AX-006)을 독립 SPEC으로 분리한 결과이다. SPEC-AX-001의 5개 REQ-AX와 REQ-UBI-001~003은 Python 측 GREEN 상태로 가정하며, 본 SPEC은 그 위에 Go orchestration 계층을 얹는다.

Sprint 0 scaffolding 산출물(`apps/control-plane/main.go`, `cmd/server/server.go`, `internal/workflow/state_machine.go`, `internal/scheduler/dispatcher.go`, `internal/store/postgres.go`)은 모두 stub 단계(`@MX:TODO` Sprint 7 표시)이며 비즈니스 로직 0줄이다. 본 SPEC이 그 stub들을 GREEN으로 전환한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `CTRL` (Control Plane sub-domain)
- 따라서 SPEC ID: `SPEC-AX-CTRL-001` (2 domains, `.claude/skills/moai/workflows/plan.md:366` Composite domain rules — "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내)

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 디렉토리 트리의 `apps/control-plane/` 섹션을 그대로 따른다. Sprint 0 stub 파일들은 모두 본 SPEC에서 비즈니스 로직으로 채워진다 (Delta 마커 없음 — stub은 "implementation absent" 상태이므로 brownfield 보존 의무 없음).

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | 모듈 |
|------|------|------|
| `apps/control-plane/main.go` | 진입점, 설정 로드, 로거 초기화, server.Run 호출 | 전체 |
| `apps/control-plane/cmd/server/server.go` | gRPC(:50051) + REST(:8080) 복합 서버, grpc-gateway v2 마운트 | REQ-CTRL-002, REQ-CTRL-003 |
| `apps/control-plane/cmd/server/grpc_handlers.go` | gRPC service 구현 (CreateWorkflow, GetWorkflow, ListWorkflows) | REQ-CTRL-002 |
| `apps/control-plane/cmd/server/health.go` | `/healthz` 헬스체크 엔드포인트 (REST 전용) | REQ-CTRL-003 |
| `apps/control-plane/cmd/server/middleware.go` | 구조화 JSON 로깅 미들웨어 (zap), request ID 발급 | REQ-CTRL-002, REQ-CTRL-003 |
| `apps/control-plane/internal/workflow/state_machine.go` | 상태머신 invariants (CanTransition, Transition) | REQ-CTRL-001 |
| `apps/control-plane/internal/workflow/handlers.go` | 워크플로우 생명주기 핸들러 (Create + Dispatch + Callback) | REQ-CTRL-001, REQ-CTRL-005 |
| `apps/control-plane/internal/workflow/callback.go` | Python worker 완료 콜백 수신 → state transition | REQ-CTRL-001 |
| `apps/control-plane/internal/scheduler/dispatcher.go` | Redis 기반 Celery 프로토콜 v2 envelope 직렬화 + RPUSH | REQ-CTRL-005 |
| `apps/control-plane/internal/scheduler/celery_envelope.go` | Celery JSON envelope 빌더 (body/headers/properties) | REQ-CTRL-005 |
| `apps/control-plane/internal/store/postgres.go` | pgx/v5 pool, workflows 테이블 CRUD, SELECT FOR UPDATE | REQ-CTRL-004 |
| `apps/control-plane/internal/store/audit.go` | audit_logs INSERT helper (SPEC-AX-001 REQ-UBI-003과 schema 공유) | REQ-CTRL-004 |
| `apps/control-plane/internal/proto/ax/v1/` | buf로 생성된 protobuf Go stubs (read-only target, 빌드 산출물) | REQ-CTRL-002 |
| `apps/control-plane/config/config.go` | 환경변수 파서 (POSTGRES_DSN, REDIS_URL, GRPC_PORT, REST_PORT, LOG_LEVEL) | 전체 |

### 2.2 Shared Schemas (`schemas/proto/`, `schemas/openapi/`)

| 경로 | 책임 |
|------|------|
| `schemas/proto/workflow.proto` | **확장**: `service WorkflowService { rpc CreateWorkflow(...); rpc GetWorkflow(...); rpc ListWorkflows(...); }` 추가 (Sprint 0 stub은 message만 보유) |
| `schemas/openapi/openapi.yaml` | **확장**: `/api/v1/workflows` (POST/GET/LIST), `/api/v1/workflows/{id}/callback` (worker callback) 엔드포인트 추가 |
| `schemas/proto/buf.gen.yaml` | grpc-gateway plugin 추가 (`protoc-gen-grpc-gateway` + `protoc-gen-openapiv2`) |

### 2.3 Database (`.moai/db/schema/`)

| 경로 | 책임 |
|------|------|
| `.moai/db/schema/initial.sql` | **참조 only**: `workflows` 및 `audit_logs` 테이블은 SPEC-AX-001 Sprint 0에서 이미 정의됨. 본 SPEC은 schema 변경 없이 기존 스키마를 사용한다. |
| `.moai/db/schema/migrations/0002_workflow_indexes.sql` | **신규**: `workflows(status, created_at DESC)` 인덱스 추가 (ListWorkflows 페이지네이션 성능) |

### 2.4 Tests (`apps/control-plane/`, `tests/`)

| 경로 | 책임 |
|------|------|
| `apps/control-plane/internal/workflow/state_machine_test.go` | 4-state × 3-transition + invalid transition 테이블 테스트 | REQ-CTRL-001 |
| `apps/control-plane/internal/workflow/handlers_test.go` | Create/Dispatch/Callback 핸들러 단위 테스트 | REQ-CTRL-001, REQ-CTRL-005 |
| `apps/control-plane/internal/scheduler/dispatcher_test.go` | Celery envelope 직렬화 검증 + Redis miniredis 기반 RPUSH 검증 | REQ-CTRL-005 |
| `apps/control-plane/internal/scheduler/celery_envelope_test.go` | Kombu 호환 envelope 골든 파일 비교 | REQ-CTRL-005 |
| `apps/control-plane/internal/store/postgres_test.go` | testcontainers-go(postgres) 기반 CRUD + SELECT FOR UPDATE 동시성 테스트 | REQ-CTRL-004 |
| `apps/control-plane/cmd/server/grpc_handlers_test.go` | bufconn 기반 in-memory gRPC 클라이언트로 unary RPC 검증 | REQ-CTRL-002 |
| `apps/control-plane/cmd/server/rest_handlers_test.go` | `httptest.NewRequest` 기반 REST 엔드포인트 검증 | REQ-CTRL-003 |
| `tests/integration/test_control_plane_to_pipelines.py` | Go control-plane → Redis → Python Celery worker E2E (Python pytest, docker-compose 환경) | REQ-CTRL-005 |

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건)

- **REQ-CTRL-UBI-001 (상태 불변)**: The control plane SHALL persist every workflow state transition into the `workflows` table within the same transaction as the corresponding business-side change (e.g., dispatch acknowledgement, callback receipt). 트랜잭션 외부에서 발생한 상태 변화는 audit 불가능하므로 금지된다.
- **REQ-CTRL-UBI-002 (감사 일관성)**: The control plane SHALL write an `audit_logs` entry for every workflow creation, state transition, and dispatch event, using the same `audit_logs` 테이블 스키마 정의된 SPEC-AX-001 REQ-UBI-003. user_id 기본값은 `cli-anonymous` (SPEC-AX-001 §3.1 REQ-UBI-003 State-driven 절과 동일).

### 3.2 REQ-CTRL-001 — Workflow State Machine

**상태(4개)**: `PENDING` (생성 직후), `RUNNING` (Celery dispatch ack 후), `COMPLETED` (worker 콜백 성공), `FAILED` (worker 콜백 실패 또는 dispatch 실패).

**유효 전이(3개)**: `PENDING → RUNNING`, `RUNNING → COMPLETED`, `RUNNING → FAILED`.

**터미널 상태**: `COMPLETED`, `FAILED` (이후 모든 전이 시도는 unwanted).

#### Event-driven

- **REQ-CTRL-001-E1**: WHEN a workflow is created via gRPC `CreateWorkflow` or REST `POST /api/v1/workflows`, THEN the control plane SHALL atomically (a) INSERT a `workflows` row with status `PENDING`, (b) INSERT a corresponding `audit_logs` row with action `WORKFLOW_CREATED`, and (c) return the workflow_id within 50ms (gRPC) / 100ms (REST) p99 under reference load (10 concurrent requests).

- **REQ-CTRL-001-E2**: WHEN a Python Celery worker invokes the callback endpoint `POST /api/v1/workflows/{id}/callback` with `{status: "completed"|"failed", result_json: {...}}`, THEN the control plane SHALL transition the workflow to the corresponding terminal state, persist `result_json`, write an `audit_logs` entry with action `WORKFLOW_COMPLETED` or `WORKFLOW_FAILED`, and return HTTP 204 within 50ms p99.

#### State-driven

- **REQ-CTRL-001-S1**: WHILE a workflow row is being updated (state transition in progress), the control plane SHALL hold a `SELECT ... FOR UPDATE` lock on that row for the duration of the transition transaction. Concurrent transition attempts on the same workflow_id SHALL block until the lock is released.

#### Ubiquitous

- **REQ-CTRL-001-U2** (Ubiquitous invariant): The control plane SHALL reject any transition that does not match the 3 valid transitions defined above. Specifically, terminal states (`COMPLETED`, `FAILED`) SHALL NOT be transitioned to any other state, and `PENDING` SHALL NOT transition directly to `COMPLETED` or `FAILED` without passing through `RUNNING`.

#### Unwanted

- **REQ-CTRL-001-U1**: IF a transition request specifies an invalid source-target pair (e.g., `PENDING → COMPLETED` skipping `RUNNING`, or any transition from a terminal state), THEN the control plane SHALL reject the transition with gRPC code `FAILED_PRECONDITION` (REST HTTP 409 Conflict), persist a rejected-transition audit entry with details, and SHALL NOT mutate the workflow row.

---

### 3.3 REQ-CTRL-002 — gRPC Server

#### Event-driven

- **REQ-CTRL-002-E1**: WHEN the control plane process starts, THEN it SHALL bind a gRPC server to `:50051` (default; configurable via `GRPC_PORT` env), register the `WorkflowService` (rpc methods: `CreateWorkflow`, `GetWorkflow`, `ListWorkflows`), apply structured JSON logging middleware via zap, and become ready to accept connections within 2 seconds of process start.

- **REQ-CTRL-002-E2**: WHEN a gRPC client invokes `WorkflowService.CreateWorkflow` with a valid `CreateWorkflowRequest` (document_id, optional user_id, optional metadata), THEN the control plane SHALL execute REQ-CTRL-001-E1, then trigger REQ-CTRL-005-E1 (Celery dispatch), and return `CreateWorkflowResponse{workflow_id, status: PENDING}` to the client within 50ms p99 (excluding dispatch latency).

#### State-driven

- **REQ-CTRL-002-S1**: WHILE a gRPC unary call is in progress and the client has not cancelled its context, the control plane SHALL honor the client-supplied deadline (via `ctx.Deadline()`) and SHALL propagate cancellation to downstream pgx queries and Redis commands using `context.WithCancel` chains.

#### Optional

- **REQ-CTRL-002-O1**: WHERE the `PROMETHEUS_ENABLED=true` environment variable is set, THE control plane SHALL expose gRPC server metrics (rpc duration histogram, request count by method, error rate) on `/metrics` of the REST port. Sandbox PoC 환경에서는 비활성 상태로 출고된다.

#### Unwanted

- **REQ-CTRL-002-U1**: IF a gRPC client cancels the context mid-RPC (e.g., deadline exceeded or explicit cancel), THEN the control plane SHALL abort the in-flight pgx transaction via `tx.Rollback(ctx)`, release any acquired row locks, write an `audit_logs` entry with action `WORKFLOW_CREATE_CANCELLED`, and SHALL NOT leak goroutines beyond the cancelled request scope.

---

### 3.4 REQ-CTRL-003 — REST API (gRPC-Gateway)

#### Event-driven

- **REQ-CTRL-003-E1**: WHEN the control plane process starts, THEN it SHALL bind a REST/JSON gateway to `:8080` (default; configurable via `REST_PORT` env), mount the grpc-gateway v2 reverse proxy that translates HTTP/JSON to gRPC, expose `/api/v1/workflows` (POST/GET/LIST) and `/api/v1/workflows/{id}/callback`, expose `/healthz` (REST-only, not proxied), and become ready within 2 seconds of process start.

- **REQ-CTRL-003-E2**: WHEN a REST client invokes `POST /api/v1/workflows` with a valid JSON body `{document_id, metadata}`, THEN the gateway SHALL translate it to the underlying gRPC `CreateWorkflow` call, return HTTP 201 Created with `{workflow_id, status: "PENDING"}` JSON body, set `Location: /api/v1/workflows/{workflow_id}` header, and complete within 100ms p99.

#### Unwanted

- **REQ-CTRL-003-U1**: IF a REST request body fails JSON schema validation (e.g., missing `document_id`, malformed UUID), THEN the control plane SHALL return HTTP 400 Bad Request with a structured error body `{error: {code, message, field}}`, SHALL NOT invoke the underlying gRPC service, and SHALL log the rejection at INFO level (not ERROR — client error is not a server defect).

---

### 3.5 REQ-CTRL-004 — PostgreSQL Workflow Store

#### Event-driven

- **REQ-CTRL-004-E1**: WHEN the control plane process starts, THEN it SHALL initialize a pgx/v5 connection pool against `POSTGRES_DSN` with `max_open_connections=25, max_idle_connections=5, conn_max_lifetime=1h`, verify connectivity by executing `SELECT 1`, and SHALL panic during startup if connectivity verification fails (fail-fast).

#### State-driven

- **REQ-CTRL-004-S1**: WHILE all 25 pool connections are checked out by in-flight requests, the control plane SHALL block new acquisitions for up to 5 seconds (acquire timeout) before returning gRPC `RESOURCE_EXHAUSTED` (REST HTTP 503) to the caller. SHALL NOT spawn additional connections beyond the configured maximum.

#### Unwanted

- **REQ-CTRL-004-U1**: IF a PostgreSQL query fails mid-transaction during a state transition (e.g., connection reset, deadlock detected), THEN the control plane SHALL execute `tx.Rollback(ctx)`, log the failure with full SQL state code and pgx error code at ERROR level, return gRPC `INTERNAL` to the caller, and SHALL NOT leave the workflow row in an inconsistent partial state.

---

### 3.6 REQ-CTRL-005 — Celery Dispatch via Redis

Hybrid integration: Go writes Celery-compatible JSON envelopes directly to Redis LIST (`celery` queue by default). Python Celery worker(`pipelines/workers/`)는 자신의 protocol v2 처리 그대로 사용 (Python 측 변경 없음). 단, Celery는 Python측 설정에서 `task_serializer='json', accept_content=['json']`을 강제한다 (pickle 금지).

#### Event-driven

- **REQ-CTRL-005-E1**: WHEN REQ-CTRL-001-E1 has persisted a workflow row in `PENDING` state, THEN the control plane SHALL build a Celery protocol v2 JSON envelope `{body: base64(JSON args), headers: {task: "pipelines.workers.ingestion_worker.run", id: workflow_id, ...}, content-type: "application/json", content-encoding: "utf-8", properties: {...}}`, RPUSH it to Redis LIST `celery` (queue name configurable via `CELERY_QUEUE` env), transition the workflow to `RUNNING` upon successful RPUSH ack, and complete within 100ms p99 (PENDING→RUNNING transition including Redis publish).

#### State-driven

- **REQ-CTRL-005-S1**: WHILE the Redis broker connection is unavailable (TCP RST, timeout, AUTH failure), the control plane SHALL retry the RPUSH up to 3 times with exponential backoff (50ms, 200ms, 800ms), and on final failure SHALL transition the workflow to `FAILED` with `result_json={dispatch_error: "redis_unavailable", attempts: 3}`. This is local transient retry, NOT workflow-level retry policy (the latter is excluded per §5).

#### Unwanted

- **REQ-CTRL-005-U1**: IF the Celery envelope JSON serialization fails for any reason (e.g., non-serializable metadata field), THEN the control plane SHALL NOT publish to Redis, SHALL transition the workflow to `FAILED` synchronously with `result_json={dispatch_error: "envelope_serialization_failed", detail: ...}`, write an audit entry, and return gRPC `INTERNAL` to the original caller. The PENDING state SHALL NOT persist if dispatch is structurally impossible.

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 성능 — REST workflow CRUD | p99 < 100ms (10 동시 요청 기준) | §3.4 REQ-CTRL-003-E2 |
| 성능 — gRPC unary | p99 < 50ms (CreateWorkflow 제외 dispatch overhead) | §3.3 REQ-CTRL-002-E2 |
| 성능 — 상태머신 전이 | < 5ms (CPU 시간, DB lock 시간 제외) | §3.2 |
| 성능 — Celery dispatch | p99 < 100ms (PENDING→RUNNING 전이 + RPUSH ack) | §3.6 REQ-CTRL-005-E1 |
| 가용성 — 헬스체크 | `/healthz`는 항상 200 OK 반환 (DB 미가용 시에도 503 명시), p99 < 10ms | §3.4 |
| 동시성 | 단일 workflow_id 동시 transition 시도 시 SELECT FOR UPDATE로 직렬화 | §3.2 REQ-CTRL-001-S1 |
| 로깅 | 구조화 JSON 로그 (zap), request_id correlation, level=info 기본 | `tech.md` §8.2, `structure.md` §11.2 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR) | `quality.yaml` development_mode |
| Go 도구 | go vet, golangci-lint (default + gosec), goimports | `.claude/rules/moai/languages/go.md` |
| 망분리 | 외부 API 호출 0건 (Redis + PostgreSQL 내부망만), 외부 패키지 fetch는 go.sum 핀 | `tech.md` §9.1 |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **gRPC reflection / gRPC-Web tooling** — `grpc.reflection` 서비스 미등록. grpcurl/Postman을 위한 reflection은 후속 dev tooling SPEC에서 다룬다.
2. **Authentication / Authorization / SSO / JWT** — 모든 호출은 `user_id="cli-anonymous"`. JWT 검증 미들웨어, OIDC 통합, RBAC 일체 제외. SPEC-AX-AUTH-001(미래)에서 처리.
3. **Multi-tenant workflow isolation** — 단일 테넌트(KEPCO E&C) 전제. tenant_id 컬럼 추가, 워크플로우 격리, 테넌트 RLS 정책 제외.
4. **Distributed tracing (OpenTelemetry)** — 트레이싱 SDK 통합, OTLP 익스포터, span propagation 제외. 로그 기반 request_id correlation만 제공.
5. **Rate limiting / Circuit breakers** — gRPC interceptor 기반 rate limit, sony/gobreaker 통합 등 일체 제외.
6. **Workflow retry policies** — 워크플로우 실패 시 자동 재시도 없음. 단일 시도(single-attempt) 후 FAILED 종료. 재시도는 클라이언트가 새 workflow를 생성하여 수행. (단, REQ-CTRL-005-S1의 Redis 전송 transient retry는 dispatch-level 재시도이며 workflow-level 재시도가 아님 — 별개 개념.)
7. **Admin UI / dashboards** — 운영자 콘솔, 워크플로우 모니터링 UI, Grafana 대시보드 설정 일체 제외.
8. **Migration management (alembic / golang-migrate)** — DB schema는 `.moai/db/schema/initial.sql` 수동 적용 + `migrations/0002_workflow_indexes.sql` 단일 패치. 마이그레이션 도구 통합 제외.
9. **Helm production values** — `values-prod.yaml` 작성 제외 (SPEC-AX-001 Exclusion §13과 동일 정책). `values-dev.yaml` + docker-compose 환경만 검증.
10. **Workflow cancellation API** — 진행 중 workflow를 외부에서 강제 종료하는 API 제외. `RUNNING` 상태에서 클라이언트가 cancel 요청해도 거부.
11. **Workflow query 고급 필터링** — `ListWorkflows`는 `status` + `created_at` 기준 단순 페이지네이션만 지원. 자유 텍스트 검색, 다중 필드 조합, full-text 검색 제외.
12. **Concurrent workflow batch creation** — 단일 요청 = 단일 workflow. 배치 API(`BatchCreateWorkflows`) 제외.
13. **Outbox pattern for at-least-once delivery** — Redis 전송 실패 시 즉시 FAILED 처리(REQ-CTRL-005-S1). PostgreSQL outbox 테이블 + poller goroutine을 통한 보장된 dispatch는 후속 SPEC에서 다룬다.

---

## 6. 의존성 및 전제

- **SPEC-AX-001 PASSED 가정**: REQ-AX-001~005 Python 파이프라인 + REQ-UBI-001~003 횡단 invariants가 모두 GREEN 상태이고 `pipelines.workers.ingestion_worker.run` Celery task가 호출 가능해야 한다.
- **SPEC-AX-001 Sprint 0 산출물**: `apps/control-plane/` 5개 stub 파일, `schemas/proto/workflow.proto` (messages만), `.moai/db/schema/initial.sql` (workflows + audit_logs 테이블)이 이미 존재한다.
- **`workflows` 테이블 스키마 검증**: SPEC-AX-001 strategy.md §1 Step 2에 정의된 workflows 스키마(id UUID, user_id VARCHAR(64) DEFAULT 'cli-anonymous', status ENUM, document_id FK, report_id FK, result_json JSONB, created_at, updated_at)를 그대로 사용한다. status ENUM이 정확히 `('PENDING', 'RUNNING', 'COMPLETED', 'FAILED')`인지 SPEC-AX-001 Run phase에서 확인되어야 한다.
- **Python Celery 설정 의존**: `pipelines/config/settings.py`에서 `task_serializer='json', accept_content=['json'], result_serializer='json'` 설정 필수. pickle 사용 시 본 SPEC의 Go envelope과 비호환. (SPEC-AX-001 Sprint 0에서 미설정 시 본 SPEC Sprint 1에서 패치 요청 필요 — handoff note.)
- **Go 1.22+** (`tech.md` §1, `structure.md` §7.2). go.mod module 이름: `github.com/ircp/iroum-ax` (모노레포 단일 module, strategy.md §1 Step 3).
- **주요 Go 의존성**: `google.golang.org/grpc v1.65+`, `github.com/grpc-ecosystem/grpc-gateway/v2 v2.20+`, `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`, `go.uber.org/zap`, `github.com/stretchr/testify`, `github.com/alicebob/miniredis/v2` (test), `github.com/testcontainers/testcontainers-go` (test).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- K8s Operator (`apps/control-plane/cmd/operator/`, `structure.md` §2): Operator 패턴(CRD + Reconciler)은 Helm chart 배포 후 별도 SPEC.
- Console UI에서의 워크플로우 표시: `apps/console/`는 SPEC-AX-001 Exclusion §1로 완전 제외, 본 SPEC도 동일.
- 워크플로우 결과의 후처리(downstream notification, webhook): 현재는 `result_json`을 PostgreSQL에 저장하고 클라이언트 polling으로만 조회.
- Schema 변경: `criteria.embedding VECTOR(1536)` → `VECTOR(768)` 패치는 SPEC-AX-001 Sprint 0에서 이미 처리 (strategy.md §1 Step 2). 본 SPEC은 schema 변경 없음.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/internal/{workflow,scheduler,store}/*_test.go` — 테이블 테스트, testify/assert, t.Parallel
- 통합 테스트: `apps/control-plane/internal/store/postgres_test.go` — testcontainers-go(postgres:16-pgvector); `apps/control-plane/internal/scheduler/dispatcher_test.go` — miniredis
- gRPC 핸들러 테스트: `apps/control-plane/cmd/server/grpc_handlers_test.go` — bufconn in-memory gRPC + testify
- REST 핸들러 테스트: `apps/control-plane/cmd/server/rest_handlers_test.go` — httptest.NewRequest
- E2E 통합 테스트: `tests/integration/test_control_plane_to_pipelines.py` — Python pytest with docker-compose, Go control-plane이 Python Celery worker를 실제로 트리거하는지 검증
- 성능 측정: bench 테스트(`go test -bench .`) + `iroum_ax_api_request_latency_seconds` 메트릭(REQ-CTRL-002-O1 Optional 활성 시)

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
