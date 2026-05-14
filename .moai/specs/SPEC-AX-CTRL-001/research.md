# SPEC-AX-CTRL-001 Research Notes

> Phase: Plan (Research sub-phase)
> Author: manager-spec (Opus 4.7, extended reasoning applied to 4 architectural decisions)
> Created: 2026-05-14

본 문서는 SPEC-AX-CTRL-001 spec.md / plan.md 작성을 뒷받침하는 외부 참조 자료, 의사결정 근거, 한국 공공기관 규제 컨텍스트, 그리고 plan.md §5 risk register 상세 분석을 담는다.

---

## 1. Reference Implementations

### 1.1 charmbracelet/crush (Go LSP client patterns)

- 출처: `.claude/rules/moai/core/lsp-client.md` 본 프로젝트가 LSP 통합을 위해 이미 charmbracelet/crush를 참조하고 있다.
- 본 SPEC에서의 활용: crush의 `internal/lsp/transport/connection.go` 패턴이 본 SPEC `apps/control-plane/internal/scheduler/dispatcher.go`의 connection 관리 전략과 유사 — context-aware cancellation, retry with exponential backoff, graceful close. powernap v0.1.4 자체는 LSP 전용이지만 RPC connection 라이프사이클 코드 구조는 재사용 가능.
- 직접 dependency 추가는 하지 않음 (LSP가 아닌 Celery dispatch이므로).

### 1.2 grpc-ecosystem/grpc-gateway

- 버전: v2.20+ (2026 기준 stable major)
- 핵심 패턴: `runtime.NewServeMux` + `runtime.WithErrorHandler` + `RegisterWorkflowServiceHandlerServer` 3단 셋업
- 본 SPEC `cmd/server/server.go`가 gRPC + REST 동시 listening을 errgroup으로 구현.
- proto annotation 패턴:
  ```proto
  import "google/api/annotations.proto";
  rpc CreateWorkflow(CreateWorkflowRequest) returns (CreateWorkflowResponse) {
    option (google.api.http) = {
      post: "/api/v1/workflows"
      body: "*"
    };
  }
  ```
- buf.gen.yaml 필수 plugin: `protoc-gen-grpc-gateway`, `protoc-gen-openapiv2` (선택, OpenAPI 문서 자동 생성)
- 참고: `tech.md` §12.1 의존성 핀 `grpc-gateway/v2 v2.18.0` → 본 SPEC에서는 v2.20.0으로 업데이트 (security patch 누적). Run phase 진입 시 go.sum 핀 필수.

### 1.3 jackc/pgx v5 patterns

- 버전: pgx/v5 v5.6.0 (2026 stable)
- 핵심 패턴:
  - `pgxpool.New(ctx, dsn)` — connection pool 초기화
  - `pool.Acquire(ctx)` → `defer conn.Release()` — connection 임대
  - `tx, err := conn.Begin(ctx)` → `defer tx.Rollback(ctx)` (idempotent) → `tx.Commit(ctx)` — 트랜잭션 패턴
  - `tx.QueryRow(ctx, "SELECT ... FOR UPDATE", id).Scan(...)` — SELECT FOR UPDATE
- 성능 노트: pgx/v5는 simple query protocol과 extended query protocol 모두 지원. prepared statement는 `pool.Exec` 호출 시 자동 캐싱.
- pgx 에러 처리: `errors.Is(err, pgx.ErrNoRows)` 패턴 사용 (sql.ErrNoRows 변환 불필요, pgx/v5는 자체 sentinel).
- Context7 라이브러리 ID: `/jackc/pgx`

### 1.4 redis/go-redis v9 patterns

- 버전: go-redis/v9 v9.6.0
- 핵심 패턴:
  - `redis.NewClient(&redis.Options{Addr: ..., DialTimeout: 5*time.Second, ReadTimeout: 3*time.Second})`
  - `rdb.RPush(ctx, "celery", payload).Result()` — Celery 호환 LIST RPUSH
  - `rdb.Ping(ctx).Err()` — 헬스체크
- Connection pool: go-redis는 내부적으로 pool 관리 (`PoolSize` 옵션, 기본값 10×NumCPU).

### 1.5 asynq (Go-native task queue — 평가 후 REJECTED)

- 검토 결과: plan.md §4 (B) 옵션. SPEC-AX-001이 이미 Python Celery로 worker를 GREEN 상태로 만들었으므로 asynq로 재작성하면 Python 측 worker 5개(ingestion/mapping/scoring/generation/recommendation) 전체 재구현 필요. Reject.
- 미래 시나리오: 만약 Python 워커를 Go로 마이그레이션하는 별도 SPEC이 있다면 asynq는 매력적 — 작업 우선순위, retry 정책, 스케줄링이 모두 Go-native. 본 SPEC 범위 외.

---

## 2. Korean Public Sector Regulatory Constraints

### 2.1 망분리 (Network Segmentation) 정합성

`tech.md` §9.1에 정의된 망분리 요구사항:
- 내부망에 100% 배포, 외부 인터넷 접근 0건
- K8s NetworkPolicy로 namespace 단위 격리

**본 SPEC 영향**:
- Go control-plane은 외부 LLM API 호출 없음 (LLM은 SPEC-AX-001 Python pipelines가 담당). Go 측은 PostgreSQL + Redis만 통신.
- Redis는 동일 K8s namespace 내. Celery 프로토콜 envelope에 외부 URL 포함 금지.
- gRPC reflection 의도적 미등록 (Exclusion §1) — 외부 노출 시 schema fingerprinting risk.
- audit_logs는 모두 내부 PostgreSQL에 저장. 외부 SIEM 전송 없음 (후속 SPEC에서 syslog forwarding 등 다룰 수 있음).

### 2.2 감사 로그 일관성 (REQ-UBI-003 정합)

SPEC-AX-001 §3.1 REQ-UBI-003은 모든 워크플로우 생성, 초안 생성, 예측 이벤트를 `audit_logs` 테이블에 기록할 것을 요구한다. 본 SPEC은 그 테이블 schema를 그대로 사용하되, **Go 측 컨트롤 플레인에서 작성하는 audit 이벤트 종류를 명시적으로 정의**한다:

| Action | Resource Type | 발생 시점 | 작성자 |
|--------|--------------|----------|--------|
| `WORKFLOW_CREATED` | `workflow` | REQ-CTRL-001-E1 INSERT 성공 후 | Go control-plane |
| `WORKFLOW_TRANSITIONED_TO_RUNNING` | `workflow` | REQ-CTRL-005-E1 dispatch ack 후 | Go control-plane |
| `WORKFLOW_COMPLETED` | `workflow` | REQ-CTRL-001-E2 callback 성공 후 | Go control-plane |
| `WORKFLOW_FAILED_DISPATCH` | `workflow` | REQ-CTRL-005-S1 dispatch 최종 실패 후 | Go control-plane |
| `WORKFLOW_FAILED_CALLBACK` | `workflow` | REQ-CTRL-001-E2 callback status=failed | Go control-plane |
| `TRANSITION_REJECTED` | `workflow` | REQ-CTRL-001-U1 invalid transition 시도 시 | Go control-plane |
| `CALLBACK_REJECTED_TERMINAL` | `workflow` | terminal state에 추가 callback 시도 시 | Go control-plane |
| `WORKFLOW_CREATE_CANCELLED` | `workflow` | REQ-CTRL-002-U1 ctx 취소 시 | Go control-plane |
| `DOCUMENT_UPLOADED` | `document` | (Python pipelines 측, SPEC-AX-001 REQ-UBI-003) | Python |
| `DRAFT_GENERATED` | `report` | (Python pipelines 측) | Python |
| `PREDICTION_RECORDED` | `simulation` | (Python pipelines 측) | Python |

**Cross-language consistency**: Go와 Python 모두 동일 `audit_logs` 테이블에 INSERT하므로 컬럼 schema 일치 필수. 본 SPEC의 `internal/store/audit.go`는 SPEC-AX-001 Sprint 0에서 정의한 audit_logs schema (id UUID, user_id VARCHAR(64) DEFAULT 'cli-anonymous', action VARCHAR(64), resource_id UUID, resource_type VARCHAR(32), timestamp TIMESTAMPTZ DEFAULT now(), details JSONB)를 그대로 사용한다.

**Decision (audit replication strategy)**: 
- 옵션 A: Go에서 audit_logs 직접 INSERT (본 SPEC 채택)
- 옵션 B: Go가 Redis pub/sub로 audit event 발행 → Python audit_event 서비스가 INSERT 
- 옵션 C: Go가 Python의 audit FastAPI endpoint를 호출

**결정: A (Go 직접 INSERT)** 근거:
- 데이터 sovereignty 측면에서 단일 PostgreSQL이 단일 진실 source
- Pub/sub는 fire-and-forget이라 audit 누락 위험
- HTTP 호출은 hop 추가 + Python 측 가용성 의존
- A는 schema만 동일하면 atomicity 보장 (Go tx 내에서 workflow UPDATE + audit INSERT 동시)
- 단점: schema drift 위험 → integration test (`tests/integration/test_audit_consistency.py`)에서 Go·Python 양측이 동일 컬럼셋을 쓰는지 검증

### 2.3 데이터 sovereignty (REQ-UBI-001 정합)

SPEC-AX-001 REQ-UBI-001은 모든 입력 문서/중간 산출물/출력 보고서를 customer-controlled infrastructure에 저장할 것을 요구한다. 본 SPEC은 워크플로우 메타데이터(workflows 테이블)와 audit 로그만 다루며, **외부 cloud-managed service에 어떤 데이터도 송신하지 않는다**. Redis는 K8s 내부 service. PostgreSQL은 K8s StatefulSet (`structure.md` §2).

---

## 3. Celery Protocol v2 Envelope Format (R-CTRL-001 Mitigation)

본 SPEC의 가장 핵심적인 기술적 위험. plan.md §4에서 (A) Redis-direct를 선택했으므로 envelope 형식의 정확성이 GREEN 달성 여부를 결정한다.

### 3.1 Celery Protocol v2 Reference

Celery 5.3+ (`tech.md` 의존성)이 사용하는 wire format은 Kombu의 `Message` 객체로 직렬화된다. Protocol v2 envelope의 정확한 JSON 구조:

```json
{
  "body": "<base64-encoded args/kwargs/embed JSON>",
  "content-encoding": "utf-8",
  "content-type": "application/json",
  "headers": {
    "lang": "py",
    "task": "pipelines.workers.ingestion_worker.run",
    "id": "<task-uuid>",
    "shadow": null,
    "eta": null,
    "expires": null,
    "group": null,
    "group_index": null,
    "retries": 0,
    "timelimit": [null, null],
    "root_id": "<task-uuid>",
    "parent_id": null,
    "argsrepr": "(...)",
    "kwargsrepr": "{...}",
    "origin": "<sender-hostname>",
    "ignore_result": false
  },
  "properties": {
    "correlation_id": "<task-uuid>",
    "reply_to": "<reply-queue-uuid>",
    "delivery_mode": 2,
    "delivery_info": {
      "exchange": "",
      "routing_key": "celery"
    },
    "priority": 0,
    "body_encoding": "base64",
    "delivery_tag": "<delivery-tag-uuid>"
  }
}
```

`body`의 base64 디코드 결과:
```json
[
  [<positional_args>],
  {<keyword_kwargs>},
  {
    "callbacks": null,
    "errbacks": null,
    "chain": null,
    "chord": null
  }
]
```

### 3.2 Go Implementation Strategy

`apps/control-plane/internal/scheduler/celery_envelope.go`:
1. `BuildIngestionTaskEnvelope(workflowID, documentID string, extraArgs map[string]interface{}) ([]byte, error)`
2. positional args = `[documentID]`, kwargs = `{"workflow_id": workflowID, "extra": extraArgs}`
3. embed = `{"callbacks": null, "errbacks": null, "chain": null, "chord": null}`
4. body = base64(JSON([args, kwargs, embed]))
5. headers.task = `"pipelines.workers.ingestion_worker.run"` (SPEC-AX-001 strategy.md §1 Step 4 + scheduler/dispatcher.go Sprint 0 stub의 `TaskIngestion` 상수와 동일)
6. headers.id = workflow_id (UUID v7)
7. properties.correlation_id = workflow_id (idempotency)
8. properties.delivery_mode = 2 (persistent — Redis는 무시하지만 AMQP 호환을 위해 설정)
9. properties.delivery_info.routing_key = `"celery"` (default queue)

### 3.3 Validation Strategy (Golden File)

Python side에서 reference envelope 생성 절차 (개발자 1회 수동 실행):

```python
# scripts/generate_celery_golden.py
from kombu import Connection, Producer, Exchange, Queue
import json, base64

with Connection("redis://localhost:6379") as conn:
    producer = conn.Producer(serializer="json")
    msg = producer.publish(
        body={"args": ["d-fixed-005-001"], 
              "kwargs": {"workflow_id": "fixed-test-uuid-005-001"}},
        routing_key="celery",
        task_id="fixed-test-uuid-005-001",
        ...
    )
    # Intercept the wire format before publishing
```

Go test (`celery_envelope_test.go`)는 동일 입력으로 envelope을 생성한 후 `testdata/celery_envelope_v2.json`과 byte-for-byte 비교 (key 순서 stable JSON marshal 보장 필요 → `encoding/json` 기본 동작은 map key alphabetical sort).

### 3.4 Python Side Configuration Requirement (Handoff Note)

본 SPEC GREEN 진입 전 SPEC-AX-001 Run phase에서 다음 설정 적용 필요:

`pipelines/config/settings.py`:
```python
CELERY_TASK_SERIALIZER = "json"
CELERY_ACCEPT_CONTENT = ["json"]
CELERY_RESULT_SERIALIZER = "json"
CELERY_TIMEZONE = "Asia/Seoul"
CELERY_ENABLE_UTC = False
```

만약 SPEC-AX-001 Sprint 0에서 이 설정이 누락되었다면 본 SPEC Sprint 1(S1) 시작 전 패치 PR 필요 (handoff to SPEC-AX-001 maintenance).

---

## 4. Risk Register Detail

### R-CTRL-001 — Celery Envelope Compatibility

**Likelihood**: Medium (Kombu 내부 format이 minor version에서 변경 가능)

**Impact**: High (전체 dispatch 실패 → SPEC-AX-CTRL-001 전체 실패)

**Detection**:
- Golden file comparison test (`celery_envelope_test.go`) — Run phase에서 즉시 발견
- Python Celery 측 `task_received` signal logging 활성화 시 `Received task: pipelines.workers.ingestion_worker.run[task-uuid]` 메시지 확인 가능

**Mitigation**:
- Kombu/Celery 버전 pinning (`pyproject.toml`: `celery[redis] ^5.3.0`)
- Run phase S3 시작 전 Python 측 reference envelope 생성 스크립트 1회 실행
- 골든 파일을 git에 커밋 (`apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json`)
- Celery 6.x 출시 시 별도 SPEC으로 envelope migration 처리

**Residual Risk**: Low (golden file이 있고 양측 버전 핀이 있다면 GREEN 후 변화 없음)

### R-CTRL-002 — State Machine Race Conditions

**Likelihood**: Medium (callback이 worker에서 비동기로 들어오므로 멀티 callback 가능성)

**Impact**: Medium (audit 일관성 위반, 사용자 신뢰 손상)

**Detection**:
- AC-CTRL-001-4 동시성 단위 테스트
- 통합 테스트에서 2 goroutine 동시 callback 시뮬레이션

**Mitigation**:
- REQ-CTRL-001-S1: `SELECT ... FOR UPDATE` 의무화
- WorkflowID 기반 idempotency: callback handler가 첫 진입 시 status 확인, terminal 상태면 즉시 409 반환
- `pgx.Tx`를 함수 인자로 받는 `Transition` 시그니처 — 호출자가 항상 tx 내부에서 호출하도록 강제

**Residual Risk**: Low

### R-CTRL-003 — Transaction Boundary (PostgreSQL + Celery RPUSH)

**Likelihood**: Medium (네트워크 partition, Redis 재시작 시나리오)

**Impact**: Medium (workflow 고아 PENDING — 사용자 입장에서 "처리 중" 상태가 영원히 유지될 수 있음)

**Detection**:
- AC-CTRL-005-2 (Redis unavailable retry)
- Production monitoring: PENDING 상태 워크플로우 5분 초과 alert (후속 SPEC에서 다룸)

**Mitigation**:
- 본 SPEC: PENDING→FAILED 즉시 전이 (REQ-CTRL-005-S1) — 단순하고 명확, 클라이언트가 retry
- Outbox 패턴은 §5 Exclusion으로 차후 SPEC
- handlers.go의 transactional sequence:
  1. tx.Begin
  2. INSERT workflow PENDING
  3. INSERT audit_log WORKFLOW_CREATED
  4. tx.Commit  ← 여기까지 PostgreSQL 상태 fix
  5. Dispatch.Dispatch() 호출 (별도 작업)
  6. 성공 시 새 tx: UPDATE workflow status=RUNNING + audit WORKFLOW_TRANSITIONED_TO_RUNNING
  7. 실패 시 새 tx: UPDATE workflow status=FAILED + audit WORKFLOW_FAILED_DISPATCH

이 순서의 risk: step 5와 6 사이 control-plane crash 시 PENDING 고아. **결정: 본 SPEC은 이 risk를 수용**하며, recovery는 후속 SPEC(outbox pattern + reconciler)에서 다룬다. 본 PoC 단계의 단일 노드 sandbox에서는 crash 빈도가 낮으므로 acceptable.

**Residual Risk**: Medium (수용 위험, 모니터링으로 감시)

### R-CTRL-004 — gRPC Client Cancellation Goroutine Leak

**Likelihood**: Low (ctx propagation이 잘 되어있으면 발생 안 함)

**Impact**: Medium (장기 운영 시 메모리 leak)

**Detection**: `goleak.VerifyNone(t)` 모든 테스트에서 실행 (`go.uber.org/goleak`)

**Mitigation**:
- 모든 외부 호출(pgx, go-redis)이 ctx 인자 수용
- `defer cancel()` 패턴 일관 적용
- 단위 테스트에서 goleak 의무화

**Residual Risk**: Low

### R-CTRL-005 — Cross-Language audit_logs Schema Drift

**Likelihood**: Medium (Python 측에서 schema 변경 시 Go가 즉시 알지 못함)

**Impact**: Medium (audit 불완전, 감사 통과 실패)

**Detection**:
- 통합 테스트 `tests/integration/test_audit_consistency.py` — Go·Python 양측 INSERT 후 동일 row 구조인지 검증
- DB schema migration이 추가될 때 양측 코드 동기 변경 (PR review)

**Mitigation**:
- audit_logs schema는 SPEC-AX-001 §3.1 REQ-UBI-003에서 single source. `pkg/models/audit.py` (Python)와 `internal/store/audit.go` (Go) 모두 이 schema 참조.
- Proto에 audit_logs 메시지 정의 추가는 over-engineering (audit_logs는 내부 테이블이지 RPC 인터페이스가 아님). 대신 SQL DDL이 단일 진실 source.

**Residual Risk**: Low (CI에서 통합 테스트 강제)

---

## 5. Design Decision Log (Top 3 — User Review Required)

### Decision 1: Celery Integration Strategy

- 옵션: A (Redis-direct envelope), B (asynq Go-native), C (HTTP→FastAPI→Celery), D (RabbitMQ amqp)
- 선택: **A**
- 근거: plan.md §4 trade-off matrix (A 점수 7.05, C 점수 7.05이나 A는 latency 우위)
- 사용자 검토 요청 사항: 골든 파일 검증을 통한 envelope 호환성 보장 정책이 충분한지, 또는 Python 측에 명시적 wrapper 추가가 안전한지 결정 필요.

### Decision 2: Transaction Boundary (Outbox vs Synchronous Dispatch)

- 옵션: A (Synchronous + immediate FAILED, 본 SPEC 채택), B (Outbox pattern)
- 선택: **A**, Outbox는 Exclusion §13으로 차후 SPEC
- 근거: PoC 단계 단일 노드 sandbox에서 control-plane crash 빈도가 낮음. 운영 단계에서는 Outbox 필수.
- 사용자 검토 요청 사항: PoC를 넘어 production 진입 시점에 Outbox SPEC을 언제 만들지 결정 필요.

### Decision 3: State Machine 4 States Justification

- 옵션: 4 states (PENDING/RUNNING/COMPLETED/FAILED, 본 SPEC) vs 5+ states (RETRYING/CANCELLED 추가)
- 선택: **4 states**
- 근거: 
  - PENDING: 생성 후 dispatch 전 — 명확
  - RUNNING: dispatch 완료, worker 처리 중 — 명확
  - COMPLETED: 성공 종료 — 터미널
  - FAILED: 실패 종료 (dispatch or worker) — 터미널
  - RETRYING은 Exclusion §6 (single-attempt)이므로 불필요
  - CANCELLED는 Exclusion §10 (cancellation API 없음)이므로 불필요
- 사용자 검토 요청 사항: PoC 후 RETRYING state 추가 시점, 그리고 cancellation API 요구사항 도출 시점.

---

## 6. Alternative Generation (manager-strategy philosopher framework)

본 SPEC의 구현 순서 (S1→S2→S3→S4→S5)에 대한 대안 검토:

- **Conservative (low risk)**: S1 (state machine) → S2 (store) → S3 (dispatch) → S4 (gRPC) → S5 (REST) — 데이터 계층부터 위로 (본 SPEC 채택)
- **Balanced (moderate)**: S4 (gRPC interface 먼저) → S1 (state machine) → S2 → S3 → S5 — interface-first, 그러나 gRPC handler가 빈 state machine을 호출하면 useless test 다수 → 거부
- **Aggressive**: S1+S2+S3 병렬 (state/store/dispatch 동시 작성) — TDD RED 단위 테스트 작성 시 의존 mock이 너무 많아 oversimulated 위험 → 거부

**채택: Conservative** — 각 sub-sprint가 이전 sub-sprint의 GREEN 산출물 위에 빌드되므로 mock 최소화 + 점진적 검증.

---

## 7. Cognitive Bias Check

- **Anchoring bias**: SPEC-AX-001 strategy.md §3.7이 Sprint 7을 "Control Plane Workflow"로 단일 sprint로 묶어 명시 → 본 SPEC은 그 단일 sprint를 5개 sub-sprint로 분해함으로써 anchoring을 의도적으로 깨뜨림.
- **Confirmation bias**: Celery 사용이 이미 strategy.md에서 결정되었으나 plan.md §4에서 (B)/(C)/(D) 옵션 명시 평가 → 검증 우회 회피.
- **Sunk cost bias**: Sprint 0 stub의 `@MX:TODO` 표시를 그대로 채우는 것이 "sunk cost"처럼 보일 수 있으나, stub은 0줄 비즈니스 로직이므로 sunk cost 아님 — full 재작성도 동일 비용.
- **Overconfidence**: "Walking Skeleton" 정의가 너무 좁아서 실제 운영에 부적합한지 검토 → §5 Exclusion 13개 항목으로 명시적으로 보류, 후속 SPEC 경로 제시.

**This option might fail because**:
- (a) Celery envelope v2 형식이 Kombu 6.x에서 breaking change 발생 시 골든 파일이 무효화 → mitigation: Kombu/Celery 버전 핀 + 정기 dependency audit
- (b) Go-side audit_logs INSERT가 Python-side와 schema drift 발생 시 통합 테스트가 감지하지 못할 수 있음 → mitigation: integration test에 schema validation 명시 추가
- (c) PoC 단계에서 PENDING 고아 워크플로우 1건이라도 발생 시 사용자 신뢰 손상 → mitigation: 운영 monitoring SPEC을 신속 후속 작성, 또는 본 SPEC에서 Outbox 포함 재검토 (사용자 결정 필요)

---

## 8. References (External)

| Source | Topic | URL/Path |
|--------|-------|----------|
| `tech.md` §3.3 / §6.1 / §9 / §11 / §12 | Tech stack, K8s, security, perf, version pinning | `.moai/project/tech.md` |
| `structure.md` §2 / §3 / §5 / §11 | Module layout, layer responsibilities, DB schema, logging | `.moai/project/structure.md` |
| `.moai/specs/SPEC-AX-001/spec.md` | REQ-AX-001~005 + REQ-UBI 정의 | `.moai/specs/SPEC-AX-001/spec.md` |
| `.moai/specs/SPEC-AX-001/strategy.md` §3.7 | Sprint 7 Control Plane outline | `.moai/specs/SPEC-AX-001/strategy.md` |
| `.moai/project/codemaps/data-flow.md` | E2E pipeline flow | `.moai/project/codemaps/data-flow.md` |
| `.moai/project/codemaps/pipelines.md` | Python pipelines module map | `.moai/project/codemaps/pipelines.md` |
| Celery 5.3 Protocol v2 | Wire format documentation | https://docs.celeryq.dev/en/v5.3.6/internals/protocol.html |
| grpc-gateway v2 | REST/JSON gateway over gRPC | https://github.com/grpc-ecosystem/grpc-gateway |
| pgx v5 | PostgreSQL driver patterns | https://github.com/jackc/pgx |
| go-redis v9 | Redis client patterns | https://github.com/redis/go-redis |
| testcontainers-go | Integration testing | https://golang.testcontainers.org/ |
| miniredis | In-memory Redis for tests | https://github.com/alicebob/miniredis |
| go.uber.org/goleak | Goroutine leak detection | https://github.com/uber-go/goleak |

---

## 9. Open Questions (Phase 2 RED 진입 전 orchestrator 확인 필요)

> [HARD] manager-spec(본 agent)은 AskUserQuestion 호출 금지. 아래 질문은 orchestrator가 AskUserQuestion으로 사용자에게 확인한다.
> 각 질문에 sensible default가 존재하므로 응답 없이도 Phase 2 RED 진입 가능 (Ready=YES).

### Q1: Python Celery 설정 확인

- 질문: SPEC-AX-001 Sprint 0/1에서 `pipelines/config/settings.py`에 `CELERY_TASK_SERIALIZER="json", CELERY_ACCEPT_CONTENT=["json"]`이 설정되어 있는가?
- 옵션 A (권장 default): SPEC-AX-001 GREEN 종료 시 점검 — 설정 누락 시 본 SPEC S3 시작 전 1줄 패치 PR로 처리
- 옵션 B: SPEC-AX-001 maintenance SPEC을 별도로 작성한 후 본 SPEC 시작

### Q2: docker-compose 기반 E2E 환경

- 질문: AC-CTRL-E2E-1을 실행할 docker-compose 환경이 CI에서 가용한가?
- 옵션 A (권장 default): CI에서는 docker-compose up이 가능하나 GPU 미가용 → Python worker는 CPU 모드로 운영, AC-CTRL-E2E-1은 합성 5페이지 HWP fixture로 검증
- 옵션 B: CI에서는 E2E skip, 로컬 dev에서만 수동 실행 — Sprint 8 통합 점검 시 점검

### Q3: Outbox 패턴 시점

- 질문: PoC 단계 PENDING 고아 risk를 본 SPEC에서 outbox로 mitigation할 것인지, 별도 SPEC으로 분리할 것인지?
- 옵션 A (권장 default): 별도 SPEC (Exclusion §13) — 본 SPEC은 walking skeleton, outbox는 production hardening
- 옵션 B: 본 SPEC에 포함 — REQ-CTRL-006 추가, scope 증가

### Q4: gRPC reflection 활성화 정책

- 질문: dev 환경에서 grpcurl 사용 편의를 위해 gRPC reflection을 활성화할지?
- 옵션 A (권장 default): 비활성 (Exclusion §1) — 망분리 security 우선
- 옵션 B: dev profile만 활성, prod profile 비활성 (환경변수 `GRPC_REFLECTION_ENABLED`로 제어)

---

## 10. Definition of Done (Research Phase)

- [x] 5개 reference implementation 문서화 (charmbracelet/crush, grpc-gateway, pgx v5, go-redis v9, asynq 평가)
- [x] Korean public sector regulatory constraints (망분리, 감사로그 일관성, 데이터 sovereignty) 매핑
- [x] Celery Protocol v2 envelope format 상세 (§3)
- [x] 5개 Risk (R-CTRL-001~005) detail + likelihood/impact/mitigation/residual
- [x] Top 3 design decisions (Celery integration, transaction boundary, state machine cardinality) trade-off 기록
- [x] Cognitive bias check 4종 (anchoring/confirmation/sunk cost/overconfidence)
- [x] 4개 open question with sensible defaults
- [x] External references table
