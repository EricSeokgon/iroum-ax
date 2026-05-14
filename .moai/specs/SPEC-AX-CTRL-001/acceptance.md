# SPEC-AX-CTRL-001 Acceptance Criteria

> Format: Given / When / Then
> Methodology: TDD — 각 AC가 RED phase 단위 테스트로 1:1 매핑
> Tooling: Go 1.22 + testify/assert + testcontainers-go + miniredis + bufconn + goleak

본 문서는 SPEC-AX-CTRL-001 5개 REQ에 대한 acceptance criteria를 정의한다. 각 AC는 자동화 테스트로 검증 가능해야 하며, 모호한 표현("적절히", "신속하게")은 사용하지 않는다.

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

### Edge Case AC-CTRL-001-5 (gRPC Client Cancellation Mid-Transaction)

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
| Celery dispatch p99 (PENDING→RUNNING) | < 100ms | miniredis-backed benchmark | AC-CTRL-005-1 |
| `/healthz` p99 | < 10ms | benchmark | AC-CTRL-003-3 |

성능 측정은 `go test -bench=. -benchtime=1000x -count=5 ./apps/control-plane/...` 실행 후 p99/p50 추출 (custom benchstat). CI에서는 noise 영향으로 thresholds 1.5× 완화 (best-effort).

---

## §8. Edge Case Catalog

다음 edge case들은 plan.md §5 R-CTRL-001~005 risk register와 매핑된다:

| Edge Case | 대응 AC | Risk ID |
|-----------|--------|---------|
| 동일 workflow_id 동시 transition (PostgreSQL race) | AC-CTRL-001-4, AC-CTRL-004-2 | R-CTRL-002 |
| PostgreSQL connection failure during state update | AC-CTRL-004-1 | R-CTRL-003 |
| Celery dispatch when Redis unavailable | AC-CTRL-005-2 | R-CTRL-001 |
| gRPC client cancellation mid-RPC | AC-CTRL-001-5, AC-CTRL-002-3 | R-CTRL-004 |
| PENDING → COMPLETED skip RUNNING (invalid transition) | AC-CTRL-001-2 | R-CTRL-002 |
| Worker callback to terminal state (idempotency violation) | AC-CTRL-001-3 | R-CTRL-002 |
| Envelope serialization failure (non-JSON metadata) | AC-CTRL-005-3 | R-CTRL-001 |
| Pool exhaustion under load | AC-CTRL-004-3 | R-CTRL-003 |
| Cross-language audit_logs schema drift | AC-CTRL-E2E-1 | R-CTRL-005 |

---

## §9. Definition of Done (Acceptance Phase)

본 SPEC의 acceptance가 완료되었다고 선언하기 위해 모두 PASS 필요:

- [ ] §1-§5: 5개 REQ × 3-5개 AC = 총 18개 AC 자동화 테스트 통과
- [ ] §6: AC-CTRL-E2E-1 docker-compose 기반 E2E 통합 테스트 통과
- [ ] §7: 5개 성능 지표 모두 target 충족 (CI에서는 1.5× 완화 허용)
- [ ] §8: 9개 edge case 모두 대응 AC로 검증됨
- [ ] coverage ≥ 85% (go test -cover)
- [ ] golangci-lint default + gosec 0 issue
- [ ] `goleak.VerifyNone(t)` 모든 테스트에서 통과
- [ ] @MX 태그 plan.md §6 매핑 완료 (Sprint 0의 TODO 모두 해소)
- [ ] manager-quality TRUST 5 통과
- [ ] evaluator-active per-sprint scoring 모두 ≥ 0.75 (strict profile)

**Total AC count**: 18 (§1: 5, §2: 3, §3: 3+1variant, §4: 3, §5: 3, §6: 1)
