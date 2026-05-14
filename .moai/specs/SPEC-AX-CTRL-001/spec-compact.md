# SPEC-AX-CTRL-001 Compact

> Auto-extracted token-efficient summary for Run phase consumption (~30% token savings vs full spec.md).
> Source: spec.md v0.1.1 (2026-05-14, iter 2 post plan-auditor review). DO NOT EDIT directly — regenerate from spec.md when REQs change.

---

## ID & Status

- ID: SPEC-AX-CTRL-001
- Version: 0.1.1
- Status: draft
- Methodology: TDD
- Harness: thorough
- Coverage target: 85%

## Scope (one-liner)

iroum-ax Go control plane Walking Skeleton — gRPC + REST 서버, 워크플로우 상태머신, PostgreSQL store, Redis 기반 Celery dispatch. Console UI·인증·멀티테넌트·재시도 정책 제외.

---

## REQ Modules

| ID | Title | Type Coverage |
|----|-------|---------------|
| REQ-CTRL-UBI-001 | 상태 불변 (트랜잭션 내 atomic 전이) | Ubiquitous |
| REQ-CTRL-UBI-002 | 감사 일관성 (audit_logs 필수, user_id='cli-anonymous' 기본) | Ubiquitous |
| REQ-CTRL-001 | Workflow State Machine (4 states + 3 transitions) | Event/State/Ubiquitous/Unwanted |
| REQ-CTRL-002 | gRPC Server (:50051, WorkflowService) | Event/State/Optional/Unwanted |
| REQ-CTRL-003 | REST API (:8080, gRPC-Gateway v2) | Event/Unwanted |
| REQ-CTRL-004 | PostgreSQL Store (pgx/v5, SELECT FOR UPDATE) | Event/State/Unwanted |
| REQ-CTRL-005 | Celery Dispatch via Redis (envelope v2) | Event/State/Unwanted |

EARS 5 types coverage: ✅ Ubiquitous, ✅ Event-driven, ✅ State-driven, ✅ Optional (REQ-CTRL-002-O1 Prometheus), ✅ Unwanted

---

## State Machine Reference

- States (4): PENDING, RUNNING, COMPLETED, FAILED
- Valid transitions (3): PENDING→RUNNING, RUNNING→COMPLETED, RUNNING→FAILED
- Terminal: COMPLETED, FAILED (no further transitions)
- Invariant: SELECT FOR UPDATE on workflow_id during transition (REQ-CTRL-001-S1)

---

## Affected Files (Top-Level)

### Go Control Plane (`apps/control-plane/`)
- `main.go` (Sprint 0 stub → GREEN)
- `cmd/server/{server,grpc_handlers,health,middleware,rest_handlers_test,grpc_handlers_test}.go`
- `internal/workflow/{state_machine,handlers,callback,*_test}.go`
- `internal/scheduler/{dispatcher,celery_envelope,*_test}.go`
- `internal/store/{postgres,audit,*_test}.go`
- `internal/proto/ax/v1/*.pb.go` (buf generated)
- `config/config.go`

### Shared Schemas
- `schemas/proto/workflow.proto` (extend with WorkflowService)
- `schemas/proto/buf.gen.yaml` (add grpc-gateway plugin)
- `schemas/openapi/openapi.yaml` (extend with /workflows endpoints)

### Database
- `.moai/db/schema/initial.sql` (reuse — no changes)
- `.moai/db/schema/migrations/0002_workflow_indexes.sql` (NEW: workflows(status, created_at DESC))

### Tests
- `tests/integration/test_control_plane_to_pipelines.py` (E2E)

---

## Acceptance Criteria (26 total — v0.1.2 evaluator D11 보정)

| ID | REQ | Scenario |
|----|-----|----------|
| AC-CTRL-UBI-001 | REQ-CTRL-UBI-001 | Transactional atomicity (workflow + audit both rollback on either failure) |
| AC-CTRL-UBI-002-A | REQ-CTRL-UBI-002 | WORKFLOW_CREATED audit row end-to-end (action, resource_type, user_id) |
| AC-CTRL-UBI-002-B | REQ-CTRL-UBI-002 | WORKFLOW_STATE_TRANSITION audit row with from/to JSONB details |
| AC-CTRL-UBI-002-C | REQ-CTRL-UBI-002 | user_id='cli-anonymous' default for Go path (REST + gRPC) |
| AC-CTRL-001-1 | REQ-CTRL-001 | Happy path workflow creation (gRPC, 50ms p99) |
| AC-CTRL-001-2 | REQ-CTRL-001 | Invalid transition PENDING→COMPLETED rejected |
| AC-CTRL-001-3 | REQ-CTRL-001 | Terminal state immutability (409 on re-callback) |
| AC-CTRL-001-4 | REQ-CTRL-001 | Concurrent transition serialization (SELECT FOR UPDATE) |
| AC-CTRL-001-5 | REQ-CTRL-001 | gRPC client cancellation mid-tx (rollback + audit) |
| AC-CTRL-002-1 | REQ-CTRL-002 | gRPC server startup (<2s ready) |
| AC-CTRL-002-2 | REQ-CTRL-002 | Performance: 10 concurrent CreateWorkflow p99 <50ms |
| AC-CTRL-002-3 | REQ-CTRL-002 | Cancellation propagation (no goroutine leak) |
| AC-CTRL-002-4 | REQ-CTRL-002 | Prometheus Optional /metrics conditional activation |
| AC-CTRL-003-1 | REQ-CTRL-003 | REST POST happy path (201 + Location header) |
| AC-CTRL-003-2 | REQ-CTRL-003 | Bad request (400, structured error body) |
| AC-CTRL-003-3 | REQ-CTRL-003 | Healthcheck 200 + variant 503 when DB down |
| AC-CTRL-003-4 | REQ-CTRL-003 | REST gateway startup <2s (polling /healthz) |
| AC-CTRL-004-1 | REQ-CTRL-004 | Pool fail-fast on invalid DSN |
| AC-CTRL-004-2 | REQ-CTRL-004 | SELECT FOR UPDATE 2-goroutine serialization |
| AC-CTRL-004-3 | REQ-CTRL-004 | Pool exhaustion → RESOURCE_EXHAUSTED |
| AC-CTRL-004-4 | REQ-CTRL-004 | Mid-tx PostgreSQL failure → tx.Rollback + SQLSTATE log |
| AC-CTRL-005-1 | REQ-CTRL-005 | Celery envelope golden file byte match |
| AC-CTRL-005-2 | REQ-CTRL-005 | Redis unavailable → 3 retries → FAILED |
| AC-CTRL-005-3 | REQ-CTRL-005 | Serialization failure → no RPUSH + FAILED |
| AC-CTRL-005-4 | REQ-CTRL-005 | Dispatch latency p99 <100ms (miniredis 10×1000 bench) |
| AC-CTRL-E2E-1 | All | Full lifecycle docker-compose E2E |

---

## Performance Targets

| Metric | Target | AC |
|--------|--------|-----|
| REST workflow CRUD p99 | <100ms | AC-CTRL-003-1 |
| gRPC unary p99 | <50ms | AC-CTRL-002-2 |
| State transition CPU | <5ms | AC-CTRL-001-1 |
| Celery dispatch p99 (PENDING→RUNNING+RPUSH) | <100ms | AC-CTRL-005-4 |
| /healthz p99 | <10ms | AC-CTRL-003-3 |
| REST gateway startup | <2s | AC-CTRL-003-4 |
| gRPC server startup | <2s | AC-CTRL-002-1 |

---

## Exclusions (13 entries)

1. gRPC reflection / gRPC-Web tooling
2. Authentication / Authorization / SSO / JWT
3. Multi-tenant workflow isolation
4. Distributed tracing (OpenTelemetry)
5. Rate limiting / Circuit breakers
6. Workflow retry policies (single-attempt only)
7. Admin UI / dashboards
8. Migration management (alembic / golang-migrate)
9. Helm production values
10. Workflow cancellation API
11. Workflow query advanced filtering (status + pagination only)
12. Concurrent workflow batch creation
13. Outbox pattern for at-least-once delivery

---

## Implementation Order (5 Sub-Sprints)

1. **S1** REQ-CTRL-001 (state machine)
2. **S2** REQ-CTRL-004 (postgres store)
3. **S3** REQ-CTRL-005 (celery dispatch)
4. **S4** REQ-CTRL-002 (gRPC server)
5. **S5** REQ-CTRL-003 (REST gateway)

Then: E2E integration test (`tests/integration/test_control_plane_to_pipelines.py`)

---

## Dependencies

- SPEC-AX-001 PASSED + GREEN (Python pipelines + REQ-UBI 횡단)
- SPEC-AX-001 Sprint 0 산출물: 5 stub files + workflow.proto messages + .moai/db/schema/initial.sql
- Python `pipelines/config/settings.py`: `task_serializer='json', accept_content=['json']` 필수 (Handoff Note)
- Go 1.22 baseline (1.23 features 사용 금지)
- Major deps: grpc v1.65, grpc-gateway/v2 v2.20, pgx/v5 v5.6, go-redis/v9 v9.6, zap v1.27

---

## Risk Top 3

| ID | Risk | Mitigation |
|----|------|-----------|
| R-CTRL-001 | Celery envelope v2 mismatch with Kombu | Golden file test (AC-005-1) + dispatch latency bench (AC-005-4) + version pin |
| R-CTRL-003 | PENDING orphan on RPUSH failure | Immediate FAILED transition + mid-tx rollback verified (AC-004-4) |
| R-CTRL-002 | State machine race on concurrent callback | SELECT FOR UPDATE + atomicity invariant (AC-UBI-001) |

---

## MX Tag Targets

- `@MX:ANCHOR` `StateMachine.Transition` (fan_in=4: handlers, callback, gRPC, REST)
- `@MX:ANCHOR` `Store.LockWorkflowForUpdate` (fan_in=3: handlers, callback, dispatcher)
- `@MX:WARN` `Dispatcher.Dispatch` (Redis retry loop, REASON: 100ms SLO 위험 — AC-005-4로 검증)
- `@MX:WARN` State machine concurrent UPDATE (REASON: SELECT FOR UPDATE 필수 — AC-UBI-001로 검증)
- `@MX:NOTE` `Server.Run` (errgroup + graceful shutdown)

---

## Open Questions (4 — sensible defaults available)

1. Python Celery serializer 설정 — default: SPEC-AX-001 GREEN 종료 시 점검
2. CI docker-compose E2E 가용성 — default: 합성 fixture로 검증
3. Outbox 패턴 시점 — default: 별도 SPEC
4. gRPC reflection 정책 — default: 비활성 (망분리)

Ready for plan-auditor: **YES (iter 2)**
