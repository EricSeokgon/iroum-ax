---
id: SPEC-AX-INTEG-001
title: Python↔Go 통합 — Celery 워크플로우 트리거 및 REST 콜백
version: 0.1.0
status: draft
created: 2026-05-21
updated: 2026-05-21
author: ircp
priority: high
issue_number: 0
parent_specs: [SPEC-AX-CTRL-001, SPEC-AX-PIPE-001, SPEC-AX-SERVER-001]
---

## HISTORY

- 2026-05-21: 초안 작성 (v0.1.0). research.md(2026-05-21) 기반 EARS 8 REQ + 8 AC 정의. Celery + REST callback 패턴 채택 (SPEC-AX-CTRL-001 REQ-CTRL-005 강제 준수).

---

## 1. 개요 (Overview)

### 1.1 목적 (Purpose)

본 SPEC은 iroum-ax 시스템의 두 런타임을 연결하는 **통합 레이어**를 정의한다:

- **Go Control Plane** (gRPC:50051, REST:8080) — 워크플로우 상태 머신, 영속 저장소, 감사 로깅
- **Python AI 파이프라인** (FastAPI:8001, Celery worker) — 문서 수집/매핑/스코어링/생성 등 AI 처리

선행 SPEC(SPEC-AX-CTRL-001, SPEC-AX-SERVER-001, SPEC-AX-PIPE-001)이 각각 한쪽 끝(엔벨로프 생성/REST 서버/Celery 의존성)을 완성했으나, **엔드-투-엔드 워크플로우 사이클(Create→RUNNING→COMPLETED)을 완결하는 통합 경로가 누락된 상태**다. 본 SPEC은 그 누락 경로를 채우는 최소 슬라이스를 정의한다.

### 1.2 범위 (Scope)

**IN scope:**
- Python Celery worker 스켈레톤 (`pipelines/workers/ingestion_worker.py`) — Kombu v2 엔벨로프 수신, 스텁 처리, callback POST 전송
- Go callback handler (`POST /api/v1/workflows/{id}/callback`) — RUNNING→COMPLETED/FAILED 전이 트리거, `result_json` 영속화
- Celery app 초기화 및 환경변수 검증 (`pipelines/config/celery_client.py`)
- 환경변수 `GO_CONTROL_PLANE_URL` 도입 및 부팅 검증
- 워크플로우 단위 통합 테스트(`tests/integration/test_integ_001_workflow_e2e.py`) — docker-compose 기반 풀 사이클
- REQ-UBI-001(로컬호스트-온리 LLM) 부팅 시 검증 로직

**OUT of scope:**
- 비즈니스 로직 본체(ingestion 파싱, VLM OCR, RAG 매핑, 점수 계산, 보고서 생성) — 후속 SPEC
- Celery retry/backoff 정책 커스터마이징 (Celery 기본값 사용)
- 워크플로우 취소 API
- gRPC streaming status push
- Python 측 audit_logs 직접 기록 (후속 SPEC에서 STEP_STARTED/STEP_COMPLETED 추가)
- Dead-letter queue, callback 재시도 정책 (D2 참조)
- 워크플로우 timeout-based cleanup (영구 RUNNING 고착 방지)

### 1.3 선행 SPEC 결정 사항 (Prior Decisions that constrain this SPEC)

본 SPEC은 다음 선행 결정에 **종속**되며, 이를 재논의하지 않는다:

| 선행 SPEC | 결정 사항 | 본 SPEC에 미치는 제약 |
|----------|----------|---------------------|
| SPEC-AX-CTRL-001 REQ-CTRL-005 | Celery dispatch via Redis (고정) | Direct HTTP(Go→Python) 옵션 차단 |
| SPEC-AX-CTRL-001 | State machine: `PENDING→RUNNING→COMPLETED\|FAILED` 4-state | Callback handler는 RUNNING 상태에서만 수용 |
| SPEC-AX-PIPE-001 D7 | Celery over BackgroundTasks (고정) | FastAPI BackgroundTasks fallback 금지 |
| SPEC-AX-PIPE-001 | Python 스택: FastAPI 0.110.0 + Celery 5.3.6 + Redis 5.0.4 | 동일 라이브러리 버전 재사용 |
| SPEC-AX-SERVER-001 | REST 라우터 mount 위치: `apps/control-plane/cmd/server/server.go` | callback route는 기존 server.go에 mount |
| 프로젝트 횡단 REQ-UBI-001 | 로컬호스트-온리 LLM | Python worker 부팅 시 VLLM_ENDPOINT 검증 강제 |
| 프로젝트 횡단 REQ-UBI-003 | audit_logs `cli-anonymous` 기본 user_id | Go callback handler는 워크플로우 record의 user_id 사용 |

---

## 2. 배경 및 동기 (Background)

### 2.1 현재 Gap

연구 문서(research.md §1.3)가 식별한 4가지 갭:

1. Go는 Redis `celery` queue에 Kombu v2 envelope을 RPUSH하지만, **Python worker 측에 task `pipelines.workers.ingestion_worker.run`이 미구현**
2. Python이 처리 결과를 Go로 보고할 **callback REST 엔드포인트 미구현** (`POST /api/v1/workflows/{id}/callback`)
3. **공유 메시지 스키마 문서화 부재** — Go envelope 빌더와 Python worker 간 계약 불명
4. **dispatch 실패 시 Python 측 에러 처리 패턴 부재**

### 2.2 구현 목표

본 SPEC 완료 시 다음 시나리오가 통과해야 한다:

```
사용자 → POST /api/v1/workflows {document_id}
       ← {workflow_id, status: PENDING}
Go     → Redis RPUSH (Celery envelope)
       → workflow.Start() → RUNNING
Python ← Celery dequeue → run(document_id, workflow_id)
       → (스텁 처리)
       → POST /api/v1/workflows/{id}/callback {status: completed, result_json}
Go     ← workflow.Complete(result_json) → COMPLETED
       ← audit_logs: CREATED, TRANSITIONED_TO_RUNNING, COMPLETED
사용자 → GET /api/v1/workflows/{id} → {status: COMPLETED, result_json: {...}}
```

비즈니스 로직 본체는 후속 SPEC에서 채워지며, 본 SPEC은 **통합 골격**만 보장한다.

---

## 3. 아키텍처 설계 (Architecture)

### 3.1 시스템 통합 다이어그램

```
┌──────────────┐    POST /api/v1/workflows    ┌──────────────────────┐
│   Client     │ ───────────────────────────▶ │  Go Control Plane    │
│  (REST/gRPC) │ ◀─────────────────────────── │  REST :8080          │
└──────────────┘    {workflow_id, status}     │  gRPC :50051         │
                                              └──────────┬───────────┘
                                                         │
                                                         │ RPUSH (Kombu v2 envelope)
                                                         ▼
                                              ┌──────────────────────┐
                                              │  Redis :6379         │
                                              │  queue: "celery"     │
                                              └──────────┬───────────┘
                                                         │
                                                         │ dequeue
                                                         ▼
                                              ┌──────────────────────┐
                                              │  Python Celery       │
                                              │  ingestion_worker    │
                                              │   .run(doc_id, wf_id)│
                                              └──────────┬───────────┘
                                                         │
                              POST /api/v1/workflows/{id}/callback
                                                         │
                                                         ▼
                                              ┌──────────────────────┐
                                              │  Go callback_handler │
                                              │  → Complete()/Fail() │
                                              │  → audit_logs        │
                                              │  → result_json INSERT│
                                              └──────────────────────┘

         ┌───────────────────────────────────────────────┐
         │  PostgreSQL (shared)                          │
         │  - workflows (status, result_json JSONB)      │
         │  - audit_logs (action, resource_id, details)  │
         └───────────────────────────────────────────────┘
```

### 3.2 인터페이스 계약 요약

- **Go→Python**: Redis Celery queue, Kombu v2 JSON envelope, task name `pipelines.workers.ingestion_worker.run` (§5.1)
- **Python→Go**: HTTP POST `application/json`, fire-and-forget 패턴 (§5.2)
- **공유 스토리지**: PostgreSQL (Go 단독 쓰기), Redis (Celery broker)

### 3.3 상태 전이 다이어그램 (Go 측 워크플로우)

```
        ┌─────────┐
        │ PENDING │  (CreateWorkflow 직후, 초기 상태)
        └────┬────┘
             │ Start() — Dispatcher RPUSH 성공
             ▼
        ┌─────────┐
        │ RUNNING │  (Python worker 처리 중)
        └────┬────┘
             │
        ┌────┴────┐
        │         │
  Complete()    Fail()
        │         │
        ▼         ▼
  ┌─────────┐ ┌─────────┐
  │COMPLETED│ │ FAILED  │  (terminal — 추가 transition 차단)
  └─────────┘ └─────────┘
```

- terminal 상태(`COMPLETED`/`FAILED`)에서 들어오는 callback은 **HTTP 409 Conflict** 반환 (REQ-INTEG-005)
- `PENDING` 상태에서 callback이 도달하는 경우(이론상 dispatcher 실패와 callback 조기 도달의 경쟁) 역시 **409 Conflict**

---

## 4. 요구사항 (Requirements) — EARS 형식

### REQ-INTEG-001: Celery Task Definition

**WHEN** Go dispatcher pushes a Kombu v2 Celery envelope to the Redis `celery` queue with task name `pipelines.workers.ingestion_worker.run`, **THEN** the Python Celery worker SHALL deserialize the envelope and invoke the task function with positional argument `document_id: str` and keyword argument `workflow_id: str`.

근거: research.md §2.3 (envelope body `[[document-id], {workflow_id: 'uuid'}, ...]`); SPEC-AX-CTRL-001 REQ-CTRL-005.

### REQ-INTEG-002: Callback HTTP Contract (Python→Go)

**WHEN** the Python Celery task completes (성공 또는 실패), **THEN** the worker SHALL POST to `{GO_CONTROL_PLANE_URL}/api/v1/workflows/{workflow_id}/callback` with JSON body `{"status": "completed"|"failed", "result_json": {...}}` and handle the HTTP 204 No Content response.

근거: research.md §2.1, §4.

### REQ-INTEG-003: Callback Failure Handling

**IF** the callback POST fails (timeout, 5xx response, or network error), **THEN** the Python worker SHALL log the error at ERROR level **AND** SHALL NOT raise an exception, allowing Celery to ACK the task after the callback attempt.

근거: research.md §5.3 (fire-and-forget); D2 결정사항.

### REQ-INTEG-004: Go Callback Handler — State Transition

- **WHEN** Go receives `POST /api/v1/workflows/{id}/callback` with body `{"status": "completed", "result_json": {...}}` and the workflow is in `RUNNING` state, **THEN** the state machine SHALL transition the workflow to `COMPLETED` **AND** persist `result_json` to the `workflows.result_json` column **AND** record a `WORKFLOW_COMPLETED` audit entry.
- **WHEN** Go receives the same endpoint with body `{"status": "failed", "result_json": {...}}` and the workflow is in `RUNNING` state, **THEN** the state machine SHALL transition the workflow to `FAILED` **AND** persist `result_json` **AND** record a `WORKFLOW_FAILED` audit entry.

근거: research.md §3.1, §6 (state_machine.go Complete()/Fail()).

### REQ-INTEG-005: Invalid Transition Rejection

**IF** a callback arrives for a workflow that is NOT in `RUNNING` state (즉, `PENDING`, `COMPLETED`, or `FAILED`), **THEN** Go SHALL return HTTP `409 Conflict` **AND** SHALL NOT modify the workflow state **AND** SHALL NOT record a state-transition audit entry.

근거: state machine invariant; idempotent endpoint를 위한 명시적 거부.

### REQ-INTEG-006: Configuration — GO_CONTROL_PLANE_URL

- **WHILE** the Python Celery worker initializes, the Celery app SHALL read `GO_CONTROL_PLANE_URL` from the environment.
- **IF** `GO_CONTROL_PLANE_URL` is missing or empty, **THEN** the worker SHALL fail to start with a clear error message identifying the missing variable.

근거: research.md §7.4 (settings.py 수정 항목); 기본값 `http://localhost:8080` (development docker-compose 주입)은 §5.3 환경변수 표 참조.

### REQ-INTEG-007: End-to-End Audit Trail

**WHEN** a workflow completes the full success cycle (Create → RUNNING → COMPLETED), **THEN** the `audit_logs` table SHALL contain at minimum the following three actions for that `workflow_id` (as `resource_id`): `WORKFLOW_CREATED`, `WORKFLOW_TRANSITIONED_TO_RUNNING`, **AND** `WORKFLOW_COMPLETED`. The equivalent failure cycle SHALL produce `WORKFLOW_CREATED`, `WORKFLOW_TRANSITIONED_TO_RUNNING`, **AND** `WORKFLOW_FAILED`.

근거: research.md §1.1 (Audit Trail Actions); REQ-UBI-003 부분 충족.

### REQ-INTEG-008: REQ-UBI-001 Compliance in Worker

**WHILE** the Python worker initializes, it SHALL assert that any configured LLM endpoint URL (e.g., `VLLM_ENDPOINT`) starts with one of: `http://localhost`, `http://127.0.0.1`, or `http://[::1]`. **IF** the validation fails, **THEN** the worker SHALL refuse to start with an error identifying the violating URL.

근거: research.md §5.1; 프로젝트 횡단 REQ-UBI-001.

---

## 5. 인터페이스 명세 (Interface Specifications)

### 5.1 Celery Envelope Schema (Go→Python via Redis)

빌더 구현: `apps/control-plane/internal/scheduler/dispatcher.go::BuildEnvelope()`
골든 테스트: `apps/control-plane/internal/scheduler/testdata/`

```json
{
  "body": "<base64-encoded-payload>",
  "content-encoding": "utf-8",
  "content-type": "application/json",
  "headers": {
    "id": "<workflow-uuid>",
    "task": "pipelines.workers.ingestion_worker.run",
    "user_id": "cli-anonymous",
    "lang": "py"
  },
  "properties": {
    "body_encoding": "base64",
    "delivery_mode": 2,
    "correlation_id": "<workflow-uuid>"
  }
}
```

base64 디코딩 후 payload 형식:
```json
[
  ["<document-id>"],
  {"workflow_id": "<workflow-uuid>"},
  {"callbacks": null, "chain": null, "chord": null, "errbacks": null}
]
```

Python Celery는 위 페이로드를 자동 역직렬화하여 다음을 호출:

```
run(document_id: str, workflow_id: str)
# 위치 인자: document_id
# 키워드 인자: workflow_id
```

| 필드 | 타입 | 출처 | 비고 |
|------|------|------|------|
| `headers.id` | UUID string | Go workflow.id | Celery message ID로도 사용 |
| `headers.task` | string | 고정 `pipelines.workers.ingestion_worker.run` | task name routing key |
| `headers.user_id` | string | Go workflow.user_id | 기본 `cli-anonymous` (REQ-UBI-003) |
| `properties.correlation_id` | UUID string | Go workflow.id | callback 추적용 |
| payload[0][0] | string | document_id | 위치 인자 |
| payload[1].workflow_id | UUID string | workflow.id | 키워드 인자 |

### 5.2 Callback REST Schema (Python→Go)

```
POST /api/v1/workflows/{workflow_id}/callback
Content-Type: application/json

Request Body:
{
  "status": "completed" | "failed",
  "result_json": { ... }   // arbitrary JSONB; 스키마 미강제 (D3 참조)
}

Responses:
  204 No Content   — 정상 전이 성공
  409 Conflict     — 워크플로우가 RUNNING 상태가 아님
  404 Not Found    — 워크플로우 ID 미존재
  400 Bad Request  — JSON 파싱 실패 또는 status 필드 값 위반
```

- 요청 본문의 `status` 필드는 정확히 `"completed"` 또는 `"failed"`만 허용 (그 외 값은 400 Bad Request)
- `result_json`은 생략 가능하며, 생략 시 빈 객체 `{}` 로 영속화
- 응답 본문 없음 (204/409/404/400 공통, 단 4xx는 표준 에러 envelope를 추가할 수 있음)

### 5.3 Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `GO_CONTROL_PLANE_URL` | `http://localhost:8080` | Yes (worker) | Go REST base URL (callback 대상) |
| `CELERY_BROKER_URL` | `redis://localhost:6379/0` | Yes (worker) | Celery Redis broker |
| `VLLM_ENDPOINT` | `http://localhost:8000` | Yes (worker) | 로컬 vLLM 서버 (REQ-UBI-001 검증 대상) |

- 모든 변수는 Python `pipelines/config/settings.py`에서 단일 진입점으로 로드
- 누락 시 Celery worker 부팅 실패 (REQ-INTEG-006, REQ-INTEG-008)

---

## 6. 구현 범위 (Implementation Scope)

### 6.1 신규 생성 파일 (New Files)

| 경로 | 목적 |
|------|------|
| `pipelines/workers/ingestion_worker.py` | Celery task — envelope 수신, 스텁 처리, callback POST |
| `pipelines/config/celery_client.py` | Celery app 초기화, 환경변수 검증 (REQ-INTEG-006/008) |
| `apps/control-plane/internal/handler/callback_handler.go` | Go callback HTTP handler |
| `tests/integration/test_integ_001_workflow_e2e.py` | docker-compose 기반 풀 사이클 통합 테스트 |
| `tests/unit/test_integ_001_celery_envelope.py` | Python Kombu v2 역직렬화 단위 테스트 |
| `tests/unit/test_integ_001_callback_resilience.py` | Python callback 실패 시 예외 미발생 단위 테스트 |
| `tests/unit/test_integ_001_worker_startup.py` | Python 부팅 검증 단위 테스트 |

### 6.2 수정 대상 파일 (Files to Modify)

| 경로 | 변경 내용 |
|------|-----------|
| `apps/control-plane/cmd/server/server.go` | callback route mount (`POST /api/v1/workflows/{id}/callback`) |
| `apps/control-plane/internal/workflow/state_machine.go` | `Complete(result_json)`/`Fail(result_json)` 시그니처가 result_json JSONB를 수용함을 검증 (이미 있을 가능성 — 누락 시 추가) |
| `pipelines/config/settings.py` | `GO_CONTROL_PLANE_URL` 항목 추가 |
| `pipelines/main.py` | (선택) Celery app 임포트만 추가, 라우터는 변경 없음 |

### 6.3 명시적 OUT of Scope

- **비즈니스 로직 본체**: ingestion 파싱, VLM OCR, RAG 매핑, 점수 계산, 보고서 생성 — 후속 SPEC에서 worker 내부에 점진적으로 채워짐
- **Celery retry/backoff 정책 커스터마이징**: Celery 기본값 사용; 재시도/dead-letter는 별도 SPEC
- **워크플로우 취소 API**: 클라이언트가 RUNNING 워크플로우를 중단시키는 인터페이스
- **gRPC streaming for status**: WebSocket/SSE/gRPC streaming 모두 폴링(GET /workflows/{id})으로 대체
- **Python 측 audit 직접 기록**: STEP_STARTED/STEP_COMPLETED 등 Python 단계별 audit은 후속 SPEC
- **Callback 재시도 및 dead-letter queue**: 본 SPEC은 fire-and-forget (D2 참조)
- **Timeout-based RUNNING cleanup**: 영구 RUNNING 고착 회복 메커니즘 (D2 위험 명시)

---

## 7. 인수 조건 (Acceptance Criteria)

### AC-INTEG-001-1 (REQ-INTEG-001): Celery Envelope Deserialization

**WHEN** Go dispatcher builds a Kombu v2 envelope with `document_id="doc-123"` and `workflow_id="wf-456"` and RPUSHes it to the Redis `celery` queue, **THEN** the Python Celery worker SHALL deserialize the envelope and invoke the task function with positional argument `document_id="doc-123"` AND keyword argument `workflow_id="wf-456"`.

Verification: `tests/unit/test_integ_001_celery_envelope.py::test_envelope_deserialization`

### AC-INTEG-001-2 (REQ-INTEG-002, REQ-INTEG-004): Successful Callback

**WHEN** the Python Celery worker POSTs `{"status": "completed", "result_json": {"pages": 5}}` to the Go callback endpoint for a workflow in `RUNNING` state, **THEN** Go SHALL return HTTP 204 AND transition the workflow to `COMPLETED` AND persist `result_json` to the `workflows.result_json` column.

Verification: `tests/integration/test_integ_001_workflow_e2e.py::test_callback_completed`

### AC-INTEG-001-3 (REQ-INTEG-004): Failure Callback

**WHEN** the Python Celery worker POSTs `{"status": "failed", "result_json": {"error": "parse error"}}` to the Go callback endpoint for a workflow in `RUNNING` state, **THEN** Go SHALL return HTTP 204 AND transition the workflow to `FAILED` AND persist `result_json` to the `workflows.result_json` column.

Verification: `tests/integration/test_integ_001_workflow_e2e.py::test_callback_failed`

### AC-INTEG-001-4 (REQ-INTEG-005): Invalid Transition Rejected

**IF** a callback POST arrives for a workflow that is in `COMPLETED`, `FAILED`, or `PENDING` state (i.e., any state other than `RUNNING`), **THEN** Go SHALL return HTTP 409 Conflict AND SHALL NOT modify the workflow state AND SHALL NOT record an additional state-transition audit entry.

Verification: `tests/integration/test_integ_001_workflow_e2e.py::test_callback_duplicate_rejected` (COMPLETED), `test_callback_on_pending_rejected` (PENDING)

### AC-INTEG-001-5 (REQ-INTEG-003): Callback Failure Handling

**IF** the Go callback endpoint is unreachable (simulated via invalid port or connection timeout), **THEN** the Python worker SHALL log the error at ERROR level AND SHALL NOT raise an exception AND Celery SHALL ACK the task normally.

Verification: `tests/unit/test_integ_001_callback_resilience.py::test_callback_failure_no_raise`

### AC-INTEG-001-6 (REQ-INTEG-007): End-to-End Audit Trail — Success Cycle

**WHEN** a full workflow success cycle completes (Create → RUNNING → COMPLETED), **THEN** the `audit_logs` table SHALL contain — for that `workflow_id` as `resource_id` — the three actions `WORKFLOW_CREATED`, `WORKFLOW_TRANSITIONED_TO_RUNNING`, AND `WORKFLOW_COMPLETED` in ascending timestamp order.

Verification: `tests/integration/test_integ_001_workflow_e2e.py::test_audit_trail`

### AC-INTEG-001-6b (REQ-INTEG-007): End-to-End Audit Trail — Failure Cycle

**WHEN** a full workflow failure cycle completes (Create → RUNNING → FAILED), **THEN** the `audit_logs` table SHALL contain — for that `workflow_id` as `resource_id` — the three actions `WORKFLOW_CREATED`, `WORKFLOW_TRANSITIONED_TO_RUNNING`, AND `WORKFLOW_FAILED` in ascending timestamp order.

Verification: `tests/integration/test_integ_001_workflow_e2e.py::test_audit_trail_failure`

### AC-INTEG-001-7 (REQ-INTEG-006, REQ-INTEG-008): Worker Startup Validation

**IF** `GO_CONTROL_PLANE_URL` is absent from the environment OR `VLLM_ENDPOINT` does not start with one of `http://localhost`, `http://127.0.0.1`, or `http://[::1]`, **THEN** the Celery worker SHALL refuse to start AND SHALL emit an error message identifying the missing variable name or the violating URL.

Verification: `tests/unit/test_integ_001_worker_startup.py::test_startup_validation`

### AC-INTEG-001-8 (REQ-INTEG-001 through REQ-INTEG-007): End-to-End Workflow

**WHEN** docker-compose brings up Go Control Plane + Python worker + Redis + PostgreSQL AND a client issues `POST /api/v1/workflows` with a valid `document_id`, **THEN** subsequent polling via `GET /api/v1/workflows/{id}` SHALL observe `status: COMPLETED` within 30 seconds.

Verification: `tests/integration/test_integ_001_workflow_e2e.py::test_end_to_end`

---

## 8. 설계 결정 사항 (Design Decisions)

### D1: 통합 메커니즘 — Celery (Redis) + REST callback

- **결정**: Go→Python 트리거는 Redis Celery queue (RPUSH), Python→Go 보고는 HTTP REST callback (POST)
- **근거**:
  - SPEC-AX-CTRL-001 REQ-CTRL-005가 Celery dispatch를 강제 — 자유도 없음
  - gRPC bidirectional streaming 대비 운영 단순성 (REST는 LB/캐시/디버깅 도구 풍부)
  - Celery 자체의 재시도/ACK 메커니즘을 무료로 활용
- **대안 검토**:
  - **Direct HTTP (Go→Python)**: CTRL-001 위반으로 즉시 기각
  - **gRPC extension (proto 신규 service)**: schemas/proto에 Python gRPC service 정의 필요 — proto 변경 영향이 과도하며, 본 SPEC의 통합 골격 목적에 비해 over-engineering
  - **Kafka/NATS**: 인프라 추가 비용 과다; 단일 워커 시나리오에서 정당화 불가

### D2: Callback 실패 시 정책 — 로그 후 ACK (no-raise, fire-and-forget)

- **결정**: Python worker는 callback POST 실패 시 예외를 raise하지 않고 Celery task를 ACK
- **근거**:
  - 예외 raise → Celery 자동 재시도 → 전체 처리(ingestion/scoring 등) 재실행 — 비용 大
  - Callback 자체의 재전송은 별도 dead-letter queue로 처리하는 것이 깔끔 (후속 SPEC)
- **위험**:
  - Callback 단발 실패 시 Go가 RUNNING 상태로 영구 고착될 수 있음
  - 완화책(본 SPEC 범위 외): 향후 timeout-based cleanup SPEC에서 stale RUNNING 워크플로우를 FAILED로 강제 전이

### D3: result_json 스키마 — JSONB 자유 형식

- **결정**: Go callback handler는 `result_json` 스키마를 검증하지 않고 JSONB로 그대로 저장
- **근거**:
  - 후속 SPEC들이 단계별 결과 형식(VLM OCR 출력 / 매핑 결과 / 스코어 / 보고서 등)을 자유롭게 정의해야 함
  - 매 Sprint마다 schema migration 강제 시 통합 진입 장벽이 큼
- **제약**:
  - Python worker는 최소한 `{"status": "completed"|"failed"}` 필드를 callback body에 포함해야 함 (status는 enum-validated, result_json은 free-form)

### D4: user_id 전파 — 엔벨로프 헤더만, callback body 미포함

- **결정**: Go envelope의 `headers.user_id`를 통해 Python worker가 user_id를 읽을 수 있으나, callback POST body에는 user_id를 포함하지 않음
- **근거**:
  - Go callback handler는 `workflows` 테이블에서 `workflow_id`로 user_id를 직접 조회 가능 — 중복 전달 불필요
  - Callback body 스키마를 최소화하여 보안 표면적(spoofing 가능성) 축소

---

## 9. 테스트 전략 (Test Strategy)

### 9.1 단위 테스트 (Unit Tests)

| 테스트 파일 | 검증 대상 |
|------------|----------|
| `tests/unit/test_integ_001_celery_envelope.py` | Kombu v2 envelope 역직렬화 (mock Redis) — REQ-INTEG-001 |
| `tests/unit/test_integ_001_callback_resilience.py` | 네트워크/HTTP 오류 시 예외 미발생 — REQ-INTEG-003 |
| `tests/unit/test_integ_001_worker_startup.py` | 환경변수 누락/위반 시 부팅 실패 — REQ-INTEG-006/008 |
| `apps/control-plane/internal/handler/callback_handler_test.go` | Go callback handler 라우팅, 4xx 응답, 상태 전이 호출 검증 (DB mocked) |

### 9.2 통합 테스트 (Integration Tests)

| 테스트 파일 | 검증 대상 |
|------------|----------|
| `tests/integration/test_integ_001_workflow_e2e.py` | docker-compose 풀 사이클 — AC-INTEG-001-2/3/4/6/8 |

테스트 인프라:
- `pytest-docker` 또는 `testcontainers-python` 으로 Redis + PostgreSQL + Go 서버 + Python worker 기동
- DB는 매 테스트마다 truncate (격리 보장)
- 30초 타임아웃 (REQ 외 시나리오 hang 방지)

### 9.3 커버리지 목표

| 영역 | 목표 | 측정 도구 |
|------|------|---------|
| Python `pipelines/workers/` | 85%+ | `pytest --cov=pipelines.workers` |
| Python `pipelines/config/` | 85%+ | `pytest --cov=pipelines.config` |
| Go `internal/handler/callback_handler.go` | 90%+ | `go test -coverprofile` |
| Go `internal/workflow/state_machine.go` (수정 영역만) | 기존 커버리지 유지 | 동일 |

---

## 10. 의존성 및 전제조건 (Dependencies)

### 10.1 선행 SPEC 완료 필요

| SPEC | 상태 | 본 SPEC에 제공하는 산출물 |
|------|------|------------------------|
| SPEC-AX-CTRL-001 | 완료 | Go state machine, Celery dispatcher, audit recorder |
| SPEC-AX-SERVER-001 | 완료 | Go REST 서버, gRPC gateway, 라우터 mount 진입점 |
| SPEC-AX-PIPE-001 | PR #9 미병합 | FastAPI base, Celery 5.3.6 의존성, 프로젝트 구조 (`pipelines/`, `workers/` 디렉터리 존재) |

**전제 위험**: SPEC-AX-PIPE-001이 미병합 상태이므로, 본 SPEC의 Run phase 진입 전 PIPE-001 머지 또는 cherry-pick 필요.

### 10.2 런타임 의존성 (개발 + 운영 공통)

- **Redis 7.x** — Celery broker, 로컬 docker-compose 또는 호스트 설치
- **PostgreSQL 16 + pgvector** — 공유 DB; `workflows`, `audit_logs` 테이블 (선행 SPEC이 생성)
- **Go Control Plane** — `GO_CONTROL_PLANE_URL`로 도달 가능해야 함
- **Python 3.11+** — Celery 5.3.6 호환 요건

### 10.3 개발 의존성 (Python `pyproject.toml`)

- `celery[redis]>=5.3.6` (PIPE-001에서 이미 추가)
- `redis>=5.0.4` (PIPE-001에서 이미 추가)
- `requests>=2.31.0` — Python callback HTTP client (신규 추가)
- `pytest-docker` 또는 `testcontainers>=4.0` — 통합 테스트 infra (신규 추가)

### 10.4 횡단 제약 (Cross-cutting Constraints)

본 SPEC은 다음 횡단 제약을 명시적으로 준수해야 한다:

- **REQ-UBI-001 (Localhost-Only LLM)**: REQ-INTEG-008로 구체화 — 부팅 시 VLLM_ENDPOINT 검증
- **REQ-UBI-002 (Local Embedding)**: 본 SPEC 범위 외 (worker 본체 로직에서 다룸 — 후속 SPEC)
- **REQ-UBI-003 (Audit Default user_id)**: REQ-INTEG-007이 부분 충족 — Go 단의 3대 액션 보장; Python 측 단계별 audit는 후속 SPEC

---

## 11. 제외 사항 (Exclusions — What NOT to Build)

명시적으로 본 SPEC에서 제외되는 항목:

1. **AI 비즈니스 로직 본체** — 문서 파싱, VLM OCR, RAG 매핑, 스코어링, 생성 — 후속 5개 SPEC에서 worker 내부로 점진적 주입
2. **Celery 재시도 정책 커스터마이징** — `max_retries`, `retry_backoff`, `acks_late` 등 모두 Celery 기본값 유지
3. **워크플로우 취소 API** — `DELETE /api/v1/workflows/{id}` 또는 `POST .../cancel` 미정의
4. **gRPC bidirectional streaming** — `WorkflowService`에 `StreamStatus` rpc 추가하지 않음
5. **Callback 재시도 및 dead-letter queue** — 본 SPEC은 fire-and-forget (D2)
6. **Timeout-based RUNNING cleanup** — 영구 RUNNING 고착 회복 (후속 SPEC)
7. **Python 측 audit_logs 직접 기록** — STEP_STARTED/STEP_COMPLETED 등 worker 내부 단계별 감사
8. **gRPC service for Python** — schemas/proto에 Python service 정의 없음 (REST callback만 사용)
9. **WebSocket/SSE status push** — 클라이언트는 폴링(`GET /workflows/{id}`)만 사용
10. **Multi-tenant user 권한 분리** — `cli-anonymous` 기본 user_id 그대로 사용 (REQ-UBI-003)
11. **Callback 엔드포인트 출처 인증** — `POST /api/v1/workflows/{id}/callback`는 내부 서비스 간 신뢰 경계(localhost docker network) 내에서만 호출되며, 본 SPEC 범위에서는 별도 발신자 인증(mTLS, pre-shared key 등)을 정의하지 않는다. 외부망 노출 시 후속 SPEC에서 별도 게이트 정의 필요.
