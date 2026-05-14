# SPEC-AX-CTRL-001 Acceptance Criteria

> Format: Given / When / Then
> Methodology: TDD — 각 AC가 RED phase 단위 테스트로 1:1 매핑
> Tooling: Go 1.22 + testify/assert + testcontainers-go + miniredis + bufconn + goleak

본 문서는 SPEC-AX-CTRL-001의 7개 REQ(2 Ubiquitous + 5 modal)에 대한 acceptance criteria를 정의한다. 각 AC는 자동화 테스트로 검증 가능해야 하며, 모호한 표현("적절히", "신속하게")은 사용하지 않는다.

---

## §0. REQ-CTRL-UBI Transverse Invariants — Acceptance

본 섹션은 §1-§5 modal REQ를 가로지르는 두 Ubiquitous 불변 조건의 dedicated AC를 정의한다. SPEC-AX-001 iter 2가 도입한 AC-UBI-* 패턴(AC-UBI-001~004)을 본 SPEC에 동일 구조로 적용한다.

### AC-CTRL-UBI-001 (Transactional Atomicity)

REQ 대응: REQ-CTRL-UBI-001 (상태 불변 — workflow 상태 전이와 audit_logs 기입은 단일 transaction 내에서 atomic).

**Given**:
- PostgreSQL `workflows` 테이블 + `audit_logs` 테이블이 testcontainers fixture로 clean state
- Go handler가 `pgx.Tx`를 열고 (a) workflows INSERT 후 (b) audit_logs INSERT를 동일 transaction으로 시도
- 테스트는 audit_logs.action 컬럼에 fault injection (예: trigger로 CHECK constraint violation 강제) 또는 partial DDL 분리

**When** (Scenario A — audit INSERT 실패):
- workflows INSERT가 성공한다
- audit_logs INSERT가 constraint violation으로 실패한다
- handler가 `tx.Rollback(ctx)`를 호출한다

**Then** (Scenario A):
- workflows 테이블에 row가 **존재하지 않는다** (rollback 완료)
- audit_logs 테이블에 row가 **존재하지 않는다** (애초에 실패)
- handler 반환 에러가 wrapped audit insertion failure
- `goleak.VerifyNone(t)` 통과

**Edge — Scenario B (역방향, workflow INSERT 실패):**
- audit_logs INSERT가 먼저 성공한다 (가상의 reverse-order handler 또는 audit-first 패턴)
- workflows INSERT가 unique violation으로 실패한다
- handler가 `tx.Rollback(ctx)`를 호출한다
- workflows 테이블 + audit_logs 테이블 모두 row 없음 (둘 다 rollback)

**Edge — Scenario C (RUNNING → COMPLETED 전이 중 audit fail):**
- workflow_id `"wf-uuid-ubi-001"`가 status=`RUNNING` 상태
- callback handler가 `tx`를 열고 (a) workflows UPDATE status=`COMPLETED` 후 (b) audit_logs INSERT action=`WORKFLOW_COMPLETED`를 시도
- audit_logs INSERT 실패 → `tx.Rollback(ctx)`
- workflows row의 status가 **여전히 `RUNNING`** (UPDATE도 rollback)
- audit_logs에 `WORKFLOW_COMPLETED` row 없음

이는 plan.md §5 R-CTRL-002 Risk(상태머신 race)와 SPEC-AX-001 AC-UBI-003 (transactional atomicity) 패턴의 Go 측 대응이다.

---

### AC-CTRL-UBI-002-A (Audit Event Completeness — Workflow Creation)

REQ 대응: REQ-CTRL-UBI-002 (감사 일관성 — 모든 workflow creation/transition/dispatch에 audit_logs 기록).

**Given**:
- PostgreSQL `audit_logs` 테이블이 testcontainers fixture로 clean state
- AuthN disabled 상태 (Walking Skeleton 기본값)

**When**:
- 클라이언트가 gRPC `WorkflowService.CreateWorkflow{document_id:"d-uuid-ubi-002a"}` RPC를 정상 호출한다
- handler가 정상 완료한다 (return CreateWorkflowResponse)

**Then**:
- audit_logs 테이블에 정확히 1개 row 존재하며:
  - `action = 'WORKFLOW_CREATED'`
  - `resource_type = 'workflow'`
  - `resource_id = <반환된 workflow_id>`
  - `user_id = 'cli-anonymous'` (AuthN disabled 기본값)
  - `created_at`이 NOT NULL이며 RPC 호출 시각 ± 50ms 이내
- audit_logs row의 `details` JSONB가 `{document_id:"d-uuid-ubi-002a"}`를 포함

---

### AC-CTRL-UBI-002-B (Audit Event Completeness — State Transition)

REQ 대응: REQ-CTRL-UBI-002 (state transition 별 audit row 1:1 매핑).

**Given**:
- workflow_id `"wf-uuid-ubi-002b"`가 status=`PENDING` 상태로 존재
- 직전 step에서 dispatch RPUSH ack 수신

**When**:
- workflow handler가 `Transition(wf, StatusRunning)`을 commit한다 (PENDING → RUNNING)

**Then**:
- audit_logs 테이블에 정확히 1개 row 존재하며:
  - `action = 'WORKFLOW_TRANSITIONED_TO_RUNNING'` (research.md §2.2 audit enumeration 단일 명칭, v0.1.2 evaluator D12 보정)
  - `resource_type = 'workflow'`
  - `resource_id = 'wf-uuid-ubi-002b'`
  - `details` JSONB가 `{"from":"PENDING", "to":"RUNNING"}` 포함
  - `user_id = 'cli-anonymous'`
- 동일 transaction 내 workflows row의 status가 `RUNNING`으로 변경됨
- (AC-CTRL-UBI-001과 결합: audit fail 시 양쪽 다 rollback)

---

### AC-CTRL-UBI-002-C (cli-anonymous Default for Go path)

REQ 대응: REQ-CTRL-UBI-002 (user_id 기본값 cli-anonymous, SPEC-AX-001 AC-UBI-004 Go 측 평행 검증).

**Given**:
- Walking Skeleton 환경 (`AUTH_ENABLED=false`, JWT 미들웨어 미등록)
- REST 클라이언트가 `Authorization` 헤더 없이 요청

**When**:
- 클라이언트가 `POST /api/v1/workflows` with body `{"document_id":"d-uuid-ubi-002c"}`를 호출한다 (no auth header)

**Then**:
- workflows 테이블 row의 `user_id` 컬럼 = `'cli-anonymous'` (정확히 literal 문자열, NULL 금지)
- audit_logs 테이블 row의 `user_id` 컬럼 = `'cli-anonymous'`
- 두 row의 user_id가 byte-identical (cross-table consistency)
- 이는 SPEC-AX-001 §3.1 REQ-UBI-003 State-driven 절과 schema-level 정합

**Edge — gRPC path 검증**:
- 동일 invariant가 gRPC `CreateWorkflow` 호출에서도 성립 (metadata 없이 호출 시 user_id='cli-anonymous')
- 이는 8개 Go-side audit actions(research.md §2.2 enumeration: WORKFLOW_CREATED, WORKFLOW_TRANSITIONED_TO_RUNNING, WORKFLOW_COMPLETED, WORKFLOW_FAILED_DISPATCH, WORKFLOW_FAILED_CALLBACK, TRANSITION_REJECTED, CALLBACK_REJECTED_TERMINAL, WORKFLOW_CREATE_CANCELLED) 전체에 균일 적용

---

## §1. REQ-CTRL-001 Workflow State Machine — Acceptance

### AC-CTRL-001-1 (Happy Path: 워크플로우 생성)

**Given**:
- PostgreSQL `workflows` 테이블이 비어 있다 (clean state via testcontainers fixture)
- Redis `celery` LIST가 비어 있다 (miniredis fixture)
- 새 document_id `"d-uuid-001"`가 documents 테이블에 존재한다

**When**:
- 클라이언트가 gRPC `WorkflowService.CreateWorkflow{document_id: "d-uuid-001"}`를 호출한다

**Then**:
- 응답이 50ms 이내 도착 (`assert.Less(t, elapsed, 50*time.Millisecond)`)
- 응답 `workflow_id`가 유효한 UUID v7 형식
- `workflows` 테이블에 정확히 1개 row 존재, status=`PENDING` → 이후 dispatch ack → status=`RUNNING`
- `audit_logs` 테이블에 action=`WORKFLOW_CREATED` row 존재 (user_id=`cli-anonymous`)
- Redis `celery` LIST에 정확히 1개 envelope 존재 (LRANGE 0 0)

### AC-CTRL-001-2 (Invalid Transition: PENDING → COMPLETED skip RUNNING)

**Given**:
- workflow_id `"wf-uuid-002"`가 status=`PENDING` 상태로 존재한다 (Celery dispatch 직전 시점 fixture)

**When**:
- 내부 핸들러가 (악의적 또는 버그로) `Transition(wf, StatusCompleted)`를 직접 호출한다

**Then**:
- `Transition` 함수가 `ErrInvalidTransition` 에러 반환
- workflows 테이블 row의 status는 여전히 `PENDING` (UPDATE 발생하지 않음)
- audit_logs에 action=`TRANSITION_REJECTED` + details JSONB에 `{from:"PENDING", to:"COMPLETED", reason:"missing_intermediate_state"}` 기록

### AC-CTRL-001-3 (Terminal State Immutability)

**Given**:
- workflow_id `"wf-uuid-003"`가 status=`COMPLETED` 상태로 존재한다

**When**:
- 클라이언트가 worker callback `POST /api/v1/workflows/wf-uuid-003/callback`을 두 번째로 호출한다 (idempotency 위반 시도)

**Then**:
- 응답 HTTP 409 Conflict
- 응답 body `{error: {code: "WORKFLOW_TERMINAL", message: "workflow wf-uuid-003 is already in terminal state COMPLETED"}}`
- workflows 테이블 row의 result_json은 변경되지 않음 (첫 callback의 결과 보존)
- audit_logs에 `CALLBACK_REJECTED_TERMINAL` 기록

### AC-CTRL-001-4 (Concurrent Transition — SELECT FOR UPDATE 직렬화)

**Given**:
- workflow_id `"wf-uuid-004"`가 status=`RUNNING` 상태로 존재한다
- 2개의 goroutine이 동시에 callback을 시도한다: G1=`{status:"completed"}`, G2=`{status:"failed"}`

**When**:
- G1과 G2가 거의 동시(< 1ms 간격)에 `POST /api/v1/workflows/wf-uuid-004/callback`을 호출한다

**Then**:
- 정확히 하나만 성공 (HTTP 204), 다른 하나는 HTTP 409 (이미 terminal state)
- workflows 테이블 row의 status는 G1 또는 G2의 결과 중 정확히 하나만 반영 (mixed state 금지)
- audit_logs에 정확히 1개의 `WORKFLOW_COMPLETED` 또는 `WORKFLOW_FAILED` 기록 + 1개의 `CALLBACK_REJECTED_TERMINAL` 기록
- `goleak.VerifyNone(t)` 통과 (leaked goroutine 없음)

### AC-CTRL-001-5 (Edge — gRPC Client Cancellation Mid-Transaction)

**Given**:
- 클라이언트가 `ctx, cancel := context.WithTimeout(parent, 10*time.Millisecond)`로 deadline을 매우 짧게 설정한다
- 핸들러가 PostgreSQL transaction을 시작한 직후 client deadline이 만료된다

**When**:
- `tx.Rollback(ctx)`가 호출된다

**Then**:
- gRPC 응답 code = `DEADLINE_EXCEEDED`
- workflows 테이블에 부분 row 잔존 없음 (rollback 완료)
- audit_logs에 action=`WORKFLOW_CREATE_CANCELLED` 기록
- `goleak.VerifyNone(t)` 통과

---

## §2. REQ-CTRL-002 gRPC Server — Acceptance

### AC-CTRL-002-1 (gRPC Server Startup)

**Given**:
- Test 환경에서 `GRPC_PORT=0`(자동 할당) + 모든 외부 의존성(PostgreSQL via testcontainers, Redis via miniredis) ready

**When**:
- `server.Run(ctx)`가 호출된다

**Then**:
- 2초 이내 `:50051` 또는 자동할당 포트에 LISTEN 상태
- 클라이언트가 `grpc.NewClient(addr, ...)` 후 `WorkflowService.GetWorkflow(ctx, &GetWorkflowRequest{id: "nonexistent"})` 호출 시 즉시 응답 (서버가 ready)
- 응답은 gRPC code `NOT_FOUND` (NULL → 빈 lookup), but 연결 자체는 성공

### AC-CTRL-002-2 (gRPC CreateWorkflow Performance)

**Given**:
- 10 concurrent gRPC 클라이언트가 미리 ready 상태 (bufconn pre-warmed)

**When**:
- 100개의 `CreateWorkflow` 요청을 동시에 발사한다 (각 클라이언트당 10개)

**Then**:
- 모든 요청이 성공 응답 (no error)
- p99 응답시간 < 50ms (dispatch overhead 제외, 즉 PENDING 상태까지의 시간)
- p50 응답시간 < 20ms
- 100개의 workflow_id가 모두 unique
- 100개의 audit_logs entry 존재 (action=`WORKFLOW_CREATED`)

### AC-CTRL-002-3 (gRPC Cancellation Propagation)

**Given**:
- 클라이언트가 `ctx, cancel := context.WithCancel(parent)`로 cancel-able context 생성

**When**:
- `WorkflowService.CreateWorkflow(ctx, req)` 호출 직후 1ms 후 `cancel()` 실행

**Then**:
- 클라이언트 측 에러: `context.Canceled` 또는 gRPC code `CANCELED`
- 서버 측: pgx transaction이 `tx.Rollback(ctx)` 호출됨 (mock pgx pool에서 검증)
- workflows 테이블에 row 없음 (insert도 rollback)
- `goleak.VerifyNone(t)` 통과

### AC-CTRL-002-4 (Prometheus Optional — Conditional Activation)

REQ 대응: REQ-CTRL-002-O1 (Optional: `PROMETHEUS_ENABLED=true`일 때 `/metrics` 노출).

**Given (활성화)**:
- 환경변수 `PROMETHEUS_ENABLED=true` 설정
- 서버가 정상 부팅된 상태
- 최소 1건의 gRPC CreateWorkflow 호출이 선행되어 메트릭이 누적된 상태

**When**:
- 클라이언트가 `GET http://<REST_PORT>/metrics` 호출

**Then**:
- HTTP 200 OK
- 응답 Content-Type: `text/plain; version=0.0.4`
- 응답 body가 Prometheus exposition format
- 다음 metric 이름들이 존재:
  - `grpc_server_handled_total{grpc_method="CreateWorkflow"}` (counter, ≥1)
  - `grpc_server_handling_seconds_bucket{...}` (histogram)
  - `grpc_server_started_total{...}` (counter)

**Edge — 비활성화 (Sandbox 기본):**
- `PROMETHEUS_ENABLED` 미설정 또는 `false`
- `GET /metrics` 호출 시 HTTP 404 (route 미등록)
- 다른 endpoint(`/healthz`, `/api/v1/workflows`)는 정상 동작
- 이는 Walking Skeleton sandbox 기본 출고 상태와 일치

---

## §3. REQ-CTRL-003 REST API — Acceptance

### AC-CTRL-003-1 (REST POST Happy Path)

**Given**:
- REST 게이트웨이가 `:8080`에 바인딩됨
- 새 document_id `"d-uuid-rest-001"` 존재

**When**:
- 클라이언트가 `POST /api/v1/workflows` with body `{"document_id":"d-uuid-rest-001"}`, `Content-Type: application/json` 호출

**Then**:
- HTTP 201 Created
- 응답 body `{"workflow_id":"<uuid v7>","status":"PENDING"}`
- 응답 header `Location: /api/v1/workflows/<uuid v7>`
- 응답 시간 < 100ms p99 (10회 반복 측정)
- workflows 테이블에 row 존재

### AC-CTRL-003-2 (REST Bad Request)

**Given**:
- REST 게이트웨이 ready

**When**:
- 클라이언트가 `POST /api/v1/workflows` with body `{"document_id":"not-a-uuid"}` (malformed UUID) 호출

**Then**:
- HTTP 400 Bad Request
- 응답 body `{"error":{"code":"INVALID_ARGUMENT","message":"document_id must be a valid UUID","field":"document_id"}}`
- workflows 테이블 변화 없음
- 서버 로그 레벨 = INFO (ERROR가 아님 — client error는 server defect가 아니므로)

### AC-CTRL-003-3 (Healthcheck Endpoint)

**Given**:
- 서버 ready (DB + Redis 모두 연결)

**When**:
- 클라이언트가 `GET /healthz` 호출

**Then**:
- HTTP 200 OK
- 응답 body `{"status":"healthy","postgres":"ok","redis":"ok","timestamp":"<ISO-8601>"}`
- 응답 시간 < 10ms p99

**Variant AC-CTRL-003-3b** (DB unavailable):
- Given: PostgreSQL 연결이 fail-fast로 종료된 상태에서 healthcheck endpoint만 살아있다고 가정
- Then: HTTP 503 Service Unavailable, body `{"status":"degraded","postgres":"error: ...", "redis":"ok"}`

### AC-CTRL-003-4 (REST Gateway Startup <2s)

REQ 대응: REQ-CTRL-003-E1 (REST/JSON 게이트웨이가 process start 후 2초 이내 ready).

**Given**:
- Test 환경에서 `REST_PORT=0`(자동 할당) + 모든 외부 의존성(PostgreSQL via testcontainers, Redis via miniredis) ready
- 측정 시작 시각 `t0 := time.Now()`

**When**:
- `server.Run(ctx)`가 호출되어 gRPC + REST 복합 서버를 부팅한다
- REST 클라이언트가 `GET /healthz`를 100ms 폴링 간격으로 호출한다

**Then**:
- `healthz`가 HTTP 200을 반환하는 첫 시각 `t1`에 대해 `t1 - t0 < 2000ms` (`assert.Less(t, t1.Sub(t0), 2*time.Second)`)
- 동시에 `POST /api/v1/workflows` 핸들러도 ready 상태 (404 아닌 정상 라우팅)
- gRPC-Gateway v2 reverse proxy 라우트 `/api/v1/workflows`, `/api/v1/workflows/{id}/callback`, `/healthz` 모두 등록 확인 (`httptest.NewRequest` 직접 호출 검증)
- 이는 AC-CTRL-002-1(gRPC 서버 startup)과 parallel 검증으로 REQ-CTRL-003-E1을 isolate

---

## §4. REQ-CTRL-004 PostgreSQL Store — Acceptance

### AC-CTRL-004-1 (Pool Initialization Fail-Fast)

**Given**:
- 잘못된 `POSTGRES_DSN` (예: `postgres://nonexistent:nonexistent@127.0.0.1:1/dbnone`)

**When**:
- `store.New(ctx, cfg)` 호출

**Then**:
- 5초 이내 에러 반환 (connection refused 또는 timeout)
- 반환 에러는 `pgx.ConnectError` 또는 wrapped error
- `*Store`는 nil 반환
- 호출자(main.go)는 이 에러를 받아 즉시 `log.Fatal` 또는 panic으로 fail-fast (단위 테스트에서는 에러 반환만 검증)

### AC-CTRL-004-2 (SELECT FOR UPDATE Serialization)

**Given**:
- workflow_id `"wf-pg-001"`가 status=`RUNNING` 상태로 존재
- 2개의 goroutine이 동시에 `store.LockWorkflowForUpdate(ctx, tx, "wf-pg-001")` 호출

**When**:
- G1이 먼저 락 획득 (tx1 BEGIN + SELECT FOR UPDATE), G2가 그 직후 호출

**Then**:
- G2는 G1의 tx1.Commit() 또는 tx1.Rollback()까지 블록
- 두 goroutine 모두 결국 성공 응답 받음 (deadlock 없음)
- 두 goroutine의 응답 순서는 비결정적이나, 두 번째 응답은 첫 번째의 결과 반영 (예: G1이 COMPLETED 변경, G2가 그 후 lock 획득하면 G2에서는 이미 COMPLETED 관찰)

### AC-CTRL-004-3 (Pool Exhaustion)

**Given**:
- `max_open_connections=2` (테스트용 축소 설정)
- 2개의 long-running transaction이 활성 상태 (모든 connection 점유)

**When**:
- 3번째 요청이 `store.CreateWorkflow(ctx, wf)`를 시도 (acquire timeout=5s)

**Then**:
- 5초 이내 응답
- 반환 에러는 acquire timeout 관련 (pgx pool exhausted)
- 호출자는 이를 gRPC `RESOURCE_EXHAUSTED` / REST HTTP 503으로 변환
- audit_logs에 `WORKFLOW_CREATE_FAILED_POOL_EXHAUSTED` 기록 (best-effort, 별도 tx 또는 별도 connection 필요)

### AC-CTRL-004-4 (Mid-Transaction PostgreSQL Failure Rollback)

REQ 대응: REQ-CTRL-004-U1 (mid-tx 쿼리 실패 시 `tx.Rollback(ctx)` + 전체 SQL state 로깅 + gRPC `INTERNAL` 반환 + 부분 상태 잔존 금지).

**Given**:
- testcontainers-go(postgres:16-pgvector) 실가용 instance
- workflow_id `"wf-uuid-pg-004"`가 status=`RUNNING` 상태로 존재
- Test harness가 mid-tx fault injection을 준비: 두 번째 쿼리가 실행되기 직전 PostgreSQL connection을 강제 종료 (e.g., `pg_terminate_backend(pid)` for the connection's backend PID), 또는 `pg_advisory_lock`을 이용한 deadlock 시뮬레이션

**When**:
- handler가 `tx, _ := pool.Begin(ctx)` 후
  - (a) `UPDATE workflows SET status='COMPLETED' WHERE id='wf-uuid-pg-004'` 성공
  - (b) `INSERT INTO audit_logs(...)` 직전 connection이 강제 종료됨
- handler가 `tx.Rollback(ctx)`를 호출한다

**Then**:
- handler가 gRPC `INTERNAL` 코드 반환 (또는 REST HTTP 500)
- workflows 테이블 row의 status가 **여전히 `RUNNING`** (UPDATE도 rollback, partial commit 잔존 0건)
- ERROR-level 로그가 zap structured 포맷으로 출력되며 다음 필드를 포함:
  - `pg_sql_state` (SQLSTATE 코드, 예: `57P01` admin_shutdown 또는 `40P01` deadlock_detected)
  - `pgx_err_code` (pgx wrapping error code)
  - `workflow_id = 'wf-uuid-pg-004'`
  - `op = 'state_transition'`
- `goleak.VerifyNone(t)` 통과 (handler goroutine이 connection lost 후에도 leak되지 않음)
- 이는 AC-CTRL-001-5(client cancel mid-tx)와 보완 관계 — server-side 쿼리 실패 경로를 isolate 검증

---

## §5. REQ-CTRL-005 Celery Dispatch — Acceptance

### AC-CTRL-005-1 (Celery Envelope Golden File Comparison)

**Given**:
- 골든 파일 `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json`이 존재
- 이 파일은 Python 측에서 `kombu.Message`로 생성한 reference envelope이다 (수동 생성, 수동 커밋)
- 입력: workflow_id=`fixed-test-uuid-005-001`, document_id=`d-fixed-005-001`, task=`pipelines.workers.ingestion_worker.run`

**When**:
- Go `BuildIngestionTaskEnvelope(...)`가 동일 입력으로 호출됨

**Then**:
- 출력 JSON이 골든 파일과 byte-for-byte 일치 (key 순서는 stable JSON marshal 후 비교)
- envelope의 `headers.task` = `"pipelines.workers.ingestion_worker.run"`
- envelope의 `headers.id` = workflow_id
- envelope의 `content-type` = `"application/json"`
- envelope의 `content-encoding` = `"utf-8"`
- envelope의 `body`가 base64 디코드 시 valid JSON args 복원

### AC-CTRL-005-2 (Redis Unavailable Retry → FAILED)

**Given**:
- workflow_id `"wf-cel-002"`가 status=`PENDING` 상태로 존재
- miniredis instance가 dispatch 직전에 Close() 호출되어 unavailable 상태

**When**:
- `dispatcher.Dispatch(ctx, envelope)` 호출

**Then**:
- 3회 재시도 (50ms, 200ms, 800ms backoff) — 총 약 1050ms 후 실패 반환
- 최종 에러는 wrapped Redis connection error
- 호출자(handlers.go)가 이를 받아 `Transition(wf, StatusFailed)` + `result_json={"dispatch_error":"redis_unavailable","attempts":3}` 저장
- audit_logs에 `WORKFLOW_FAILED_DISPATCH` 기록

### AC-CTRL-005-3 (Envelope Serialization Failure → No RPUSH)

**Given**:
- workflow_id `"wf-cel-003"`가 status=`PENDING` 상태
- metadata 필드에 직렬화 불가능한 값 포함 (예: `make(chan int)` — JSON marshal 실패)

**When**:
- `BuildIngestionTaskEnvelope(workflow_id, document_id, badArgs)` 호출

**Then**:
- 에러 즉시 반환 (`encoding/json: unsupported type` 또는 wrapped)
- Redis `celery` LIST에 RPUSH 발생 0건 (miniredis 검증)
- 호출자가 즉시 `Transition(wf, StatusFailed)` + `result_json={"dispatch_error":"envelope_serialization_failed"}` 저장
- 클라이언트 측 응답 = gRPC `INTERNAL`

### AC-CTRL-005-4 (Dispatch Latency p99 < 100ms)

REQ 대응: REQ-CTRL-005-E1 (PENDING→RUNNING 전이 + RPUSH ack 종합 latency p99 < 100ms).

**Given**:
- miniredis instance가 ready (in-memory, deterministic)
- PostgreSQL testcontainers instance가 ready
- benchmark harness가 10 concurrent dispatch goroutines, 각 1000 iteration (총 10,000 dispatch 시도) 준비

**When**:
- 각 iteration이 다음 사이클을 수행한다:
  - workflow row INSERT status=`PENDING` (transaction 1)
  - `dispatcher.Dispatch(ctx, envelope)` 호출 — Celery envelope build + Redis RPUSH
  - RPUSH ack 후 workflow status UPDATE `RUNNING` (transaction 2)
- 각 사이클의 wall-clock duration을 `time.Since()`로 측정

**Then**:
- p99 latency < 100ms (`assert.Less(t, p99, 100*time.Millisecond)`)
- p50 latency < 30ms (sanity check)
- p99.9 latency < 200ms (tail bound)
- 0 dispatch error (모든 RPUSH 성공)
- Redis LIST 크기 = 10,000 (정확히)
- CI 환경에서는 noise 영향으로 thresholds 1.5× 완화 가능(p99 < 150ms) — 단, 로컬 측정에서는 strict 100ms

실행 방법: `go test -bench=BenchmarkDispatchLatency -benchtime=10000x -count=5 ./apps/control-plane/internal/scheduler/...` 후 custom benchstat으로 p99/p50 추출. 이는 AC-CTRL-005-1(envelope golden file byte 비교)과는 별개의 latency-focused AC로, REQ-CTRL-005-E1의 timing invariant를 isolate 검증한다.

---

## §6. E2E Integration — Acceptance

### AC-CTRL-E2E-1 (Full Workflow Lifecycle)

**Given**:
- docker-compose up 환경 (PostgreSQL + Redis + Python Celery worker + Go control-plane)
- Python `pipelines.workers.ingestion_worker.run` task가 등록됨 (`accept_content=['json']` 확인)
- 새 document fixture `tests/fixtures/synthetic_safety_5page.hwp`가 documents 테이블에 사전 등록

**When**:
- 외부 클라이언트가 `POST /api/v1/workflows` with `{"document_id":"<fixture uuid>"}` 호출

**Then**:
- 즉시 HTTP 201 + workflow_id 반환
- ~5초 이내 (Python worker 처리 시간 포함) workflow status가 `COMPLETED`로 전이됨 (polling GET 검증)
- workflows.result_json에 Python worker가 반환한 결과 포함
- audit_logs에 일련의 이벤트: WORKFLOW_CREATED → WORKFLOW_TRANSITIONED_TO_RUNNING → WORKFLOW_COMPLETED (총 3개)

---

## §7. Performance Acceptance Summary

| Metric | Target | Measurement Method | Reference |
|--------|--------|--------------------|-----------|
| REST `POST /api/v1/workflows` p99 | < 100ms | `go test -bench` + custom benchmark | AC-CTRL-003-1 |
| gRPC `CreateWorkflow` p99 | < 50ms | bufconn benchmark (10 concurrent clients) | AC-CTRL-002-2 |
| State machine transition CPU time | < 5ms | testify timing assertion | AC-CTRL-001-1 |
| Celery dispatch p99 (PENDING→RUNNING + RPUSH) | < 100ms | miniredis-backed benchmark (10×1000) | **AC-CTRL-005-4** |
| `/healthz` p99 | < 10ms | benchmark | AC-CTRL-003-3 |
| REST gateway startup time | < 2s | polling `/healthz` until 200 | **AC-CTRL-003-4** |
| gRPC server startup time | < 2s | bufconn dial wait | AC-CTRL-002-1 |

성능 측정은 `go test -bench=. -benchtime=1000x -count=5 ./apps/control-plane/...` 실행 후 p99/p50 추출 (custom benchstat). CI에서는 noise 영향으로 thresholds 1.5× 완화 (best-effort). AC-CTRL-005-4가 envelope-only golden file 비교(AC-CTRL-005-1)와 분리된 dedicated latency benchmark이다.

---

## §8. Edge Case Catalog

다음 edge case들은 plan.md §5 R-CTRL-001~005 risk register와 매핑된다:

| Edge Case | 대응 AC | Risk ID |
|-----------|--------|---------|
| 동일 workflow_id 동시 transition (PostgreSQL race) | AC-CTRL-001-4, AC-CTRL-004-2 | R-CTRL-002 |
| PostgreSQL connection failure during state update | AC-CTRL-004-1, **AC-CTRL-004-4** | R-CTRL-003 |
| Mid-tx PostgreSQL query failure (deadlock / backend kill) | **AC-CTRL-004-4** | R-CTRL-003 |
| Celery dispatch when Redis unavailable | AC-CTRL-005-2 | R-CTRL-001 |
| gRPC client cancellation mid-RPC | AC-CTRL-001-5, AC-CTRL-002-3 | R-CTRL-004 |
| PENDING → COMPLETED skip RUNNING (invalid transition) | AC-CTRL-001-2 | R-CTRL-002 |
| Worker callback to terminal state (idempotency violation) | AC-CTRL-001-3 | R-CTRL-002 |
| Envelope serialization failure (non-JSON metadata) | AC-CTRL-005-3 | R-CTRL-001 |
| Pool exhaustion under load | AC-CTRL-004-3 | R-CTRL-003 |
| Audit failure mid-tx → both rollback (atomicity) | **AC-CTRL-UBI-001** | R-CTRL-002 |
| Audit completeness for full workflow lifecycle | **AC-CTRL-UBI-002-A, -B, -C** | R-CTRL-005 |
| Cross-language audit_logs schema drift | AC-CTRL-E2E-1, **AC-CTRL-UBI-002-C** | R-CTRL-005 |

---

## §9. Definition of Done (Acceptance Phase)

본 SPEC의 acceptance가 완료되었다고 선언하기 위해 모두 PASS 필요:

- [ ] §0: REQ-CTRL-UBI-001/002 전용 AC 4개(UBI-001, UBI-002-A, UBI-002-B, UBI-002-C) 모두 자동화 테스트 통과
- [ ] §1-§5: 5개 modal REQ × 3-5개 AC = 총 20개 AC 자동화 테스트 통과
- [ ] §6: AC-CTRL-E2E-1 docker-compose 기반 E2E 통합 테스트 통과
- [ ] §7: 7개 성능 지표 모두 target 충족 (CI에서는 1.5× 완화 허용)
- [ ] §8: 12개 edge case 모두 대응 AC로 검증됨
- [ ] coverage ≥ 85% (go test -cover)
- [ ] golangci-lint default + gosec 0 issue
- [ ] `goleak.VerifyNone(t)` 모든 테스트에서 통과
- [ ] @MX 태그 plan.md §6 매핑 완료 (Sprint 0의 TODO 모두 해소)
- [ ] manager-quality TRUST 5 통과
- [ ] evaluator-active per-sprint scoring 모두 ≥ 0.75 (strict profile)

**Total AC count**: 26 (§0 UBI: 4, §1: 5, §2: 4, §3: 5 including 3b variant, §4: 4, §5: 4, §6 E2E: 1 — 정확 enumeration: AC-CTRL-UBI-001, UBI-002-A, UBI-002-B, UBI-002-C, AC-CTRL-001-{1..5}, AC-CTRL-002-{1..4}, AC-CTRL-003-{1,2,3,3b,4}, AC-CTRL-004-{1..4}, AC-CTRL-005-{1..4}, AC-CTRL-E2E-1) — v0.1.2 evaluator D11 보정 (25→26)
