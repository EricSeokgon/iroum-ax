# Research: SPEC-AX-INTEG-001 Python↔Go Integration
Date: 2026-05-21

## 1. Current Architecture (Both Sides)

### 1.1 Go Control Plane — What Exists

**Workflow State Machine** (`apps/control-plane/internal/workflow/state_machine.go`):
- 4-state machine: `PENDING` → `RUNNING` → `{COMPLETED | FAILED}`
- Atomic state transitions via `TxCoordinator` (database transactions)
- Synchronization via `sync.Mutex` to prevent concurrent transition races
- Methods: `Start()` (PENDING→RUNNING), `Complete()` (RUNNING→COMPLETED), `Fail()` (RUNNING→FAILED)

**Celery Dispatcher** (`apps/control-plane/internal/scheduler/dispatcher.go`):
- Direct Redis RPUSH to Celery queue (default: `celery`)
- Kombu protocol v2 JSON envelope serialization (Celery-compatible)
- Task name: `pipelines.workers.ingestion_worker.run`
- Envelope body: `[[document_id], {workflow_id: workflowID}, {callbacks:null,...}]` (base64-encoded)
- RedisClient interface for mock-friendly testing (real impl via go-redis v9)

**Audit Trail** (`apps/control-plane/internal/audit/recorder.go`):
- Actions: `WORKFLOW_CREATED`, `WORKFLOW_TRANSITIONED_TO_RUNNING`, `WORKFLOW_COMPLETED`, `WORKFLOW_FAILED`, `WORKFLOW_FAILED_DISPATCH`
- Table: `audit_logs` (shared schema)
- user_id default: `cli-anonymous` (REQ-UBI-003)

**Database Layer** (`apps/control-plane/internal/store/`):
- PostgreSQL via pgx/v5
- `workflows` table: id(UUID), user_id, status(ENUM), document_id(FK), report_id(FK), result_json(JSONB)
- `audit_logs` table: id, action, resource_id, user_id, details(JSONB), timestamp
- SELECT FOR UPDATE locking for state transitions

**gRPC + REST Server** (SPEC-AX-SERVER-001):
- gRPC: `:50051`, REST: `:8080`
- gRPC service: `WorkflowService` — `CreateWorkflow`, `GetWorkflow`, `ListWorkflows`
- REST: `POST /api/v1/workflows`, `GET /api/v1/workflows/{id}`, `GET /api/v1/workflows`
- **KEY ENDPOINT**: `POST /api/v1/workflows/{id}/callback` — Python worker reports back here

### 1.2 Python Pipeline — What Exists

**FastAPI Entry Point** (`pipelines/main.py`):
- Framework: FastAPI 0.110.0
- Current endpoints: `/health` only (liveness probe)
- TODO stubs for all pipeline endpoints (Sprints 2-6)

**Celery Configuration** (`pyproject.toml`):
- `celery = { version = "^5.3.6", extras = ["redis"] }`
- `redis = "^5.0.4"`
- Worker expected at: `pipelines.workers.ingestion_worker.run` (Celery task name)

**Worker Directory** (`pipelines/workers/`):
- Currently empty (`__init__.py` only)
- Sprint 2 will define Celery task `run(documentID: str, workflow_id: str) -> dict`

### 1.3 Missing Integration Gap

1. Go has no code that calls Python endpoints or waits for Python responses
2. Python has no Celery worker defined (task `pipelines.workers.ingestion_worker.run` doesn't exist)
3. No callback mechanism from Python to Go
4. No shared message schema documented for Go envelope → Python task
5. No error handling for dispatch failures on Python side

---

## 2. Existing Interface Contracts

### 2.1 Proto Definitions Found

File: `schemas/proto/workflow.proto`
- Enum `WorkflowStatus`: `PENDING(1)`, `RUNNING(2)`, `COMPLETED(3)`, `FAILED(4)`
- Message `Workflow`: id, user_id, status, document_id, report_id, result_json(bytes)
- Package: `iroum.ax.v1`
- Gap: No Python gRPC service definition (integration uses REST callback, not gRPC)

### 2.2 REST/HTTP Interfaces

**Callback endpoint** (KEY INTEGRATION POINT):
```
POST /api/v1/workflows/{id}/callback
Body: {"status": "completed"|"failed", "result_json": {...}}
Response: HTTP 204 No Content
```

**Status polling**:
```
GET /api/v1/workflows/{id}
Response: {"id": "uuid", "status": "PENDING|RUNNING|COMPLETED|FAILED", ...}
```

### 2.3 Celery Envelope (Go → Python via Redis)

Built by: `apps/control-plane/internal/scheduler/dispatcher.go` BuildEnvelope()

```json
{
  "body": "base64([['document-id'], {'workflow_id': 'uuid'}, {'callbacks':null,'chain':null,'chord':null,'errbacks':null}])",
  "content-encoding": "utf-8",
  "content-type": "application/json",
  "headers": {
    "id": "workflow-uuid",
    "task": "pipelines.workers.ingestion_worker.run",
    "user_id": "cli-anonymous",
    "lang": "py",
    ...
  },
  "properties": {
    "body_encoding": "base64",
    "delivery_mode": 2,
    "correlation_id": "workflow-uuid",
    ...
  }
}
```

Python Celery auto-deserializes; task receives: `run(document_id: str, workflow_id: str)`

---

## 3. State Machine & Workflow Analysis

### 3.1 Go Workflow States

```
PENDING ──(Start/Dispatch)──→ RUNNING ──(Complete)──→ COMPLETED
                                   └──(Fail)──────→ FAILED
```

State transition flow:
1. `POST /api/v1/workflows` → Go creates PENDING row
2. Go dispatcher immediately RPUSH to Redis → on success, transitions to RUNNING
3. Python worker processes → POSTs callback → Go transitions to COMPLETED/FAILED
4. Client polls `GET /workflows/{id}` for status

### 3.2 Python Job Lifecycle (To Be Implemented)

1. Celery worker dequeues from Redis `celery`
2. Celery auto-deserializes → calls `run(document_id, workflow_id)`
3. Business logic (ingestion/mapping/scoring/generation)
4. POST callback to Go: `http://{GO_CONTROL_PLANE_URL}/api/v1/workflows/{workflow_id}/callback`
5. Celery ACKs message regardless of callback success/failure

---

## 4. Integration Pattern: Chosen Architecture

**CHOSEN: Celery Queue + REST Callback (Option C)**

Already partially implemented (Go side). This is the only compliant option given SPEC-AX-CTRL-001 REQ-CTRL-005 mandating Celery dispatch.

```
[Client]
  → POST /api/v1/workflows
  ← {workflow_id, status: "PENDING"}

[Go Control Plane]
  → RPUSH Celery envelope to Redis
  → transition to RUNNING
  ← {workflow_id, status: "RUNNING"}

[Redis broker]
  → dequeue to Python worker

[Python Celery Worker]
  → process document
  → POST /api/v1/workflows/{workflow_id}/callback {status: "completed", result_json: {...}}

[Go Control Plane /callback]
  → transition to COMPLETED
  → persist result_json

[Client]
  → GET /api/v1/workflows/{id}
  ← {status: "COMPLETED", result_json: {...}}
```

---

## 5. Key Risks & Constraints

### 5.1 REQ-UBI-001 (Localhost-Only LLM)
- Python worker must NOT call external LLM APIs
- `VLLM_ENDPOINT` must start with `http://localhost` or `http://127.0.0.1`
- Enforcement: startup assertion in celery_client.py

### 5.2 REQ-UBI-003 (Audit Logging)
- Go records: CREATE, RUNNING, COMPLETED, FAILED
- Python steps currently audit-silent (gap)
- INTEG-001 scope: Python worker SHOULD log STEP_STARTED/STEP_COMPLETED to audit_logs (or emit structured log for Go to ingest)

### 5.3 Concurrency
- Celery worker processes each job in a separate process/thread (no BackgroundTasks)
- Python callback POST is fire-and-forget (must not block on Go response)
- Celery ACKs after callback attempt (not after Go confirms)

---

## 6. Reference Implementations

| Reference | File | Purpose |
|-----------|------|---------|
| Celery envelope builder | `apps/control-plane/internal/scheduler/dispatcher.go` | Golden reference for envelope format |
| State machine transitions | `apps/control-plane/internal/workflow/state_machine.go` | Transition methods Complete()/Fail() |
| Audit recorder | `apps/control-plane/internal/audit/recorder.go` | Audit pattern (user_id propagation) |
| Golden test | `apps/control-plane/internal/scheduler/testdata/` | Expected envelope JSON |
| Callback handler stub | `apps/control-plane/cmd/server/workflow_handlers.go` | Location for callback implementation |

---

## 7. SPEC Design Recommendations

### 7.1 Scope Boundaries

**IN scope (SPEC-AX-INTEG-001):**
1. Python Celery worker skeleton (`pipelines/workers/ingestion_worker.py`) — receive envelope, call stub, post callback
2. Go callback handler (`POST /api/v1/workflows/{id}/callback`) — transition state machine
3. Integration tests: Python task deserialization + Go callback receipt
4. `GO_CONTROL_PLANE_URL` env var wiring

**OUT of scope (future SPECs):**
- Full business logic in Python worker (ingestion/mapping/scoring etc.)
- Celery chain/chord for multi-step pipeline
- Workflow cancellation
- WebSocket/gRPC streaming for status
- Retry policy beyond Celery defaults

### 7.2 EARS REQs Recommended

| REQ | EARS Pattern | Statement |
|-----|-------------|-----------|
| REQ-INTEG-001 | Event-driven | WHEN Go dispatcher pushes Celery envelope, THEN Python worker SHALL receive document_id and workflow_id as function arguments |
| REQ-INTEG-002 | Event-driven | WHEN Python worker finishes, THEN it SHALL POST callback to Go `POST /api/v1/workflows/{id}/callback` with status and result_json |
| REQ-INTEG-003 | Unwanted | IF callback POST fails, THEN worker SHALL log error and NOT retry (let Celery broker retry the full task) |
| REQ-INTEG-004 | State-driven | WHILE workflow is RUNNING, the Go callback endpoint SHALL accept any JSONB result_json (no schema enforcement) |
| REQ-INTEG-005 | Optional | WHERE available, user_id SHALL be propagated from Go envelope headers to Python worker context |

### 7.3 Acceptance Criteria Recommended

| AC | Given | When | Then | Evidence |
|----|-------|------|------|----------|
| AC-INTEG-001-1 | Go envelope in Redis | Python worker dequeues | Worker receives document_id + workflow_id | Unit test: mock Redis, verify task args |
| AC-INTEG-001-2 | Python worker completes | POST /callback with status=completed | Go returns 204, workflow→COMPLETED | Integration test: docker-compose |
| AC-INTEG-001-3 | Python worker fails | POST /callback with status=failed | Go returns 204, workflow→FAILED | Integration test: exception in worker |
| AC-INTEG-001-4 | End-to-end via REST client | CreateWorkflow → Celery → callback | Client sees COMPLETED in GET /workflows/{id} | E2E test: full cycle |
| AC-INTEG-001-5 | Workflow lifecycle complete | All transitions | audit_logs has CREATE+RUNNING+COMPLETED entries | DB assertion |

### 7.4 Files to Create/Modify

**New files:**
- `pipelines/workers/ingestion_worker.py` — Celery task + callback POST
- `pipelines/config/celery_client.py` — Celery app init + settings
- `apps/control-plane/internal/handler/callback_handler.go` (or existing handlers file)
- `tests/integration/test_integ_001_workflow.py` — Integration test suite

**Modified files:**
- `apps/control-plane/cmd/server/server.go` — Mount callback route
- `apps/control-plane/internal/workflow/state_machine.go` — Ensure Complete()/Fail() handle result_json
- `pipelines/config/settings.py` — Add GO_CONTROL_PLANE_URL
