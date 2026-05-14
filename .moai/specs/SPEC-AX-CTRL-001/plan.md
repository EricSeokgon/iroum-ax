# SPEC-AX-CTRL-001 Implementation Plan

> Phase: Plan
> Methodology: TDD (RED-GREEN-REFACTOR) per `quality.yaml` development_mode
> Harness: thorough
> Author: manager-spec (Opus 4.7)
> Created: 2026-05-14

본 plan은 SPEC-AX-CTRL-001 Go Control Plane을 5개 REQ-CTRL 단위로 sprint 분해하고, Sprint 7-S1 ~ Sprint 7-S5의 5개 sub-sprint로 점진적 GREEN을 달성한다. SPEC-AX-001 strategy.md §3.7 Sprint 7을 본 SPEC으로 승격·확장한 것이다.

---

## 1. Task Decomposition (REQ-CTRL-001 ~ REQ-CTRL-005)

### Sub-Sprint S1: REQ-CTRL-001 — Workflow State Machine

**Priority dimension**: Functionality (상태 invariants가 잘못되면 후속 REQ 전체 의미 상실)

**Owner**: expert-backend (Go domain)

**산출물**:
- `apps/control-plane/internal/workflow/state_machine.go` — Sprint 0 stub을 GREEN으로 전환
  - `type StateMachine struct{}`
  - `func (sm *StateMachine) CanTransition(from, to Status) bool` — 4 states × 4 states transition matrix
  - `func (sm *StateMachine) Transition(ctx context.Context, tx pgx.Tx, wf *Workflow, to Status) error` — invariants 검증 + tx 내 UPDATE
  - 유효 전이 3개 + 13개 invalid 전이 거부
- `apps/control-plane/internal/workflow/handlers.go` — Create/Callback 핸들러
- `apps/control-plane/internal/workflow/callback.go` — `POST /api/v1/workflows/{id}/callback` 수신
- `apps/control-plane/internal/workflow/state_machine_test.go` — 테이블 테스트
- `apps/control-plane/internal/workflow/handlers_test.go`

**Pass condition**:
- 4 states × 4 states transition matrix 16개 케이스 모두 단위 테스트 통과
- AC-CTRL-001-1 ~ AC-CTRL-001-4 통과 (acceptance.md §1)
- coverage ≥ 85%, mutation score (참고용) ≥ 70%
- golangci-lint 0 error

**선행 의존**:
- T-AX-007 (proto generated code) — strategy.md §1 Step 1 산출물
- T-AX-008 (DB schema initial.sql) — strategy.md §1 Step 2 산출물

---

### Sub-Sprint S2: REQ-CTRL-004 — PostgreSQL Workflow Store

**Priority dimension**: Functionality (모든 후속 REQ가 store에 의존)

**Owner**: expert-backend (Go + pgx 도메인)

**산출물**:
- `apps/control-plane/internal/store/postgres.go` — Sprint 0 stub을 pgx/v5 기반으로 전환
  - `type Store struct{ pool *pgxpool.Pool }`
  - `New(ctx, cfg) (*Store, error)` — pgxpool.New + SELECT 1 검증, fail-fast
  - `CreateWorkflow(ctx, wf)` — INSERT with RETURNING id
  - `GetWorkflow(ctx, id)` — SELECT
  - `ListWorkflows(ctx, status, limit, cursor)` — 페이지네이션 (cursor = (created_at, id) 복합)
  - `LockWorkflowForUpdate(ctx, tx, id) (*Workflow, error)` — SELECT ... FOR UPDATE
  - `UpdateWorkflowStatus(ctx, tx, id, status, result_json)` — UPDATE
- `apps/control-plane/internal/store/audit.go` — `InsertAuditLog(ctx, tx, action, resource_id, details)`
- `apps/control-plane/internal/store/postgres_test.go` — testcontainers-go(postgres:16) 기반 통합 테스트
- `.moai/db/schema/migrations/0002_workflow_indexes.sql` — `CREATE INDEX workflows_status_created_at_idx ON workflows(status, created_at DESC)`

**Pass condition**:
- testcontainers-go 기반 통합 테스트 모두 통과
- 동시 transition 시도(2 goroutine) → SELECT FOR UPDATE 직렬화 검증
- pool exhaustion 시 5s timeout 후 RESOURCE_EXHAUSTED 반환 검증
- AC-CTRL-004-1 ~ AC-CTRL-004-3 통과
- coverage ≥ 85%

**선행 의존**: S1 완료 (Workflow struct 정의 필요)

---

### Sub-Sprint S3: REQ-CTRL-005 — Celery Dispatch via Redis

**Priority dimension**: Functionality + Originality (Celery 프로토콜 호환성)

**Owner**: expert-backend + expert-devops 보조 (Celery protocol v2 envelope 분석)

**산출물**:
- `apps/control-plane/internal/scheduler/celery_envelope.go` — Celery protocol v2 JSON envelope 빌더
  - `type Envelope struct{ Body, Headers, Properties, ContentType, ContentEncoding ... }`
  - `BuildIngestionTaskEnvelope(workflow_id, document_id, args) ([]byte, error)`
  - 골든 파일 비교 테스트로 Kombu 호환성 검증
- `apps/control-plane/internal/scheduler/dispatcher.go` — Sprint 0 stub을 go-redis/v9 기반으로 전환
  - `type Dispatcher struct{ rdb *redis.Client; queue string }`
  - `Dispatch(ctx, envelope) error` — RPUSH `celery` LIST + 3-step backoff retry (50ms, 200ms, 800ms)
  - `Close()` — graceful shutdown
- `apps/control-plane/internal/scheduler/dispatcher_test.go` — miniredis 기반 단위 테스트
- `apps/control-plane/internal/scheduler/celery_envelope_test.go` — 골든 파일 비교

**Pass condition**:
- 정상 dispatch: RPUSH ack 후 envelope이 Kombu가 기대하는 JSON 구조와 일치 (cross-check: Python `kombu.message.Message.decode()` 호환)
- Redis 미가용 → 3회 재시도 → 최종 실패 시 dispatch error 반환
- 시리얼라이즈 실패 → 즉시 에러, 절대 RPUSH 발생하지 않음
- AC-CTRL-005-1 ~ AC-CTRL-005-3 통과
- coverage ≥ 85%

**선행 의존**: S2 완료 (handler에서 dispatch 호출 시 workflow row 필요)

**Risk**: Celery 프로토콜 envelope 정확성. Mitigation:
1. research.md §3에서 Kombu 1.x source의 `kombu.message.Message` 빌드 코드를 참조 모델로 사용
2. 골든 파일 `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json`을 Python 측에서 생성하여 커밋, Go가 동일하게 생성하는지 비교
3. SPEC-AX-001 Sprint 0의 `pipelines/workers/ingestion_worker.py`가 `accept_content=['json']` 설정 필요 — Run phase 시작 전 확인 필수

---

### Sub-Sprint S4: REQ-CTRL-002 — gRPC Server

**Priority dimension**: Functionality + Security (망분리 환경에서 외부 노출 금지)

**Owner**: expert-backend

**산출물**:
- `schemas/proto/workflow.proto` 확장 — 메시지 + service 추가:
  ```proto
  service WorkflowService {
    rpc CreateWorkflow(CreateWorkflowRequest) returns (CreateWorkflowResponse);
    rpc GetWorkflow(GetWorkflowRequest) returns (Workflow);
    rpc ListWorkflows(ListWorkflowsRequest) returns (ListWorkflowsResponse);
  }
  ```
  (HTTP 옵션 어노테이션 추가 — grpc-gateway용)
- `schemas/proto/buf.gen.yaml` 갱신 — `protoc-gen-grpc-gateway` plugin 추가
- `apps/control-plane/internal/proto/ax/v1/*.pb.go` — buf generate로 재생성 (빌드 산출물, 직접 편집 금지)
- `apps/control-plane/cmd/server/server.go` — Sprint 0 stub 전환
  - `grpc.NewServer(grpc.UnaryInterceptor(zapInterceptor))` 
  - `Run(ctx)`: gRPC :50051 + REST :8080 동시 기동 (errgroup)
- `apps/control-plane/cmd/server/grpc_handlers.go` — WorkflowService 구현
- `apps/control-plane/cmd/server/middleware.go` — zap 구조화 로깅 + request_id (uuid v7)
- `apps/control-plane/cmd/server/grpc_handlers_test.go` — bufconn in-memory gRPC client + miniredis + testcontainers-postgres

**Pass condition**:
- gRPC CreateWorkflow/GetWorkflow/ListWorkflows 모두 호출 성공
- ctx.Cancel() 시 in-flight tx Rollback + lock release 검증 (goroutine leak detector: `goleak.VerifyNone`)
- AC-CTRL-002-1 ~ AC-CTRL-002-3 통과
- coverage ≥ 85%

**선행 의존**: S1, S2, S3 완료 (전체 핸들러 chain 호출 가능해야 함)

---

### Sub-Sprint S5: REQ-CTRL-003 — REST API (gRPC-Gateway)

**Priority dimension**: Completeness (gateway는 단일 진실 source 위에 얇은 변환층)

**Owner**: expert-backend

**산출물**:
- `apps/control-plane/cmd/server/server.go` — gRPC-Gateway v2 mux 추가
  - `runtime.NewServeMux(runtime.WithErrorHandler(...))`
  - `RegisterWorkflowServiceHandlerServer(mux, grpcHandler)`
- `apps/control-plane/cmd/server/health.go` — `/healthz` (DB ping + Redis ping)
- `apps/control-plane/cmd/server/rest_handlers_test.go` — httptest 기반
- `schemas/openapi/openapi.yaml` 확장 — workflow 엔드포인트 추가 (buf openapiv2 자동 생성 또는 수동)

**Pass condition**:
- REST POST/GET/LIST 모두 호환되는 JSON 응답
- 400 Bad Request (잘못된 UUID, 누락 필드)
- 409 Conflict (invalid transition via REST)
- AC-CTRL-003-1 ~ AC-CTRL-003-3 통과
- coverage ≥ 85%

**선행 의존**: S4 완료 (gRPC handler가 GREEN이어야 gateway가 의미 있음)

---

## 2. Implementation Order (DAG)

```
[SPEC-AX-001 GREEN — assumed PASSED]
       ↓
[S1: State Machine] (REQ-CTRL-001)
       ↓
[S2: PostgreSQL Store] (REQ-CTRL-004)
       ↓
[S3: Celery Dispatch] (REQ-CTRL-005)
       ↓
[S4: gRPC Server] (REQ-CTRL-002)
       ↓
[S5: REST Gateway] (REQ-CTRL-003)
       ↓
[E2E Integration] (tests/integration/test_control_plane_to_pipelines.py)
```

순차 실행이 권장된다. S1·S2는 독립적으로 보이지만 S2 테스트가 Workflow struct(S1 정의)에 의존하므로 S1을 먼저 완료한다.

병렬화 가능: S4 gRPC 서버와 S5 REST gateway는 분리해서 작성 가능하지만, gateway가 gRPC handler를 in-process로 호출하므로 단일 사람이 순차 작성하는 것이 일관성에 유리.

---

## 3. Go Module Dependencies

`go.mod` 추가/확정 (Sprint 0에서 stub 형태로 선언됨, S1-S5에서 실제 사용):

```
require (
    google.golang.org/grpc v1.65.0
    google.golang.org/protobuf v1.34.0
    github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0
    github.com/jackc/pgx/v5 v5.6.0
    github.com/redis/go-redis/v9 v9.6.0
    go.uber.org/zap v1.27.0
    github.com/google/uuid v1.6.0
    golang.org/x/sync v0.7.0  // errgroup
)

require (
    // test-only
    github.com/stretchr/testify v1.9.0
    github.com/alicebob/miniredis/v2 v2.33.0
    github.com/testcontainers/testcontainers-go v0.32.0
    github.com/testcontainers/testcontainers-go/modules/postgres v0.32.0
    go.uber.org/goleak v1.3.0
)
```

`go mod tidy` 후 go.sum 핀.

> 주의: `tech.md` §1은 Go 1.22+로 명시했고 `.claude/rules/moai/languages/go.md`는 1.23+를 권장한다. 본 SPEC은 **Go 1.22 baseline**으로 진행하며, 1.23 전용 기능(range over integers, PGO 2.0)은 사용하지 않는다. CI matrix는 Go 1.22 + Go 1.23 양쪽 검증 권장.

---

## 4. Celery Integration Strategy (Risk-Driven Decision)

본 SPEC의 가장 핵심적인 아키텍처 결정 사항이다. 자세한 의사결정 기록은 `research.md` §3 참조.

**선택지 평가표** (가중치: Risk 30% / Implementation Cost 25% / Performance 20% / Maintainability 15% / Scalability 10%):

| 선택지 | Risk | Cost | Perf | Maint | Scale | 가중점수 |
|--------|------|------|------|-------|-------|---------|
| (A) Redis-direct Celery envelope v2 (JSON) | 6/10 | 7/10 | 9/10 | 6/10 | 8/10 | **7.05** |
| (B) asynq (Go-native) — Python worker 재작성 필요 | 8/10 | 3/10 | 9/10 | 8/10 | 7/10 | 6.70 |
| (C) HTTP trigger to FastAPI → FastAPI re-dispatch | 8/10 | 8/10 | 6/10 | 7/10 | 6/10 | 7.05 |
| (D) RabbitMQ + amqp Go client | 7/10 | 5/10 | 8/10 | 6/10 | 7/10 | 6.55 |

**결정: (A) Redis-direct Celery envelope v2**

근거:
- (B)는 SPEC-AX-001이 이미 Celery로 Python 워커를 GREEN 상태로 만들었으므로 재작성 비용 과대 (sunk cost가 아니라 검증된 작동).
- (C)는 HTTP hop 추가로 latency 2배 증가 + FastAPI 또한 Celery dispatch를 해야 하므로 single point of failure 추가.
- (D)는 Redis가 이미 SPEC-AX-001 stack에 존재하므로 새 인프라 추가 불필요.
- (A)의 risk(envelope 호환성 미스매치)는 골든 파일 비교 테스트로 mitigation.

**Cognitive bias check (anchoring)**: SPEC-AX-001 strategy.md가 Celery를 1차로 명시 → 이를 그대로 수용하는 것이 안전하나, 본 SPEC에서 (B)/(C)/(D) 옵션도 명시적으로 평가하여 anchoring을 회피했다.

---

## 5. Risk Analysis (R-CTRL-001 ~ R-CTRL-005)

| ID | Risk | 확률 | 영향 | Mitigation |
|----|------|------|------|-----------|
| **R-CTRL-001** | Celery envelope v2 JSON 직렬화가 Python 측 Kombu와 미스매치 | 중 | 높음 (dispatch 전체 실패) | 골든 파일 비교 테스트(`celery_envelope_test.go`) + Python 측 `accept_content=['json']` 설정 확인 |
| **R-CTRL-002** | 동일 workflow_id 동시 transition으로 인한 race condition (UPDATE 손실) | 중 | 중 (audit 일관성 위반) | SELECT FOR UPDATE 의무화 (REQ-CTRL-001-S1), 2-goroutine 동시 transition 단위 테스트 추가 |
| **R-CTRL-003** | PostgreSQL INSERT + Celery RPUSH 트랜잭션 경계 — RPUSH 실패 시 workflow 고아(PENDING) 발생 | 중 | 중 | (a) Dispatch 실패 시 즉시 PENDING→FAILED 전이 (REQ-CTRL-005-S1/U1), (b) Outbox 패턴은 §5 Exclusion으로 차후 SPEC, (c) handlers.go에서 INSERT→Dispatch→Transition 순서 보장 + dispatch 실패 catch |
| **R-CTRL-004** | gRPC client cancellation 시 goroutine leak | 낮음 | 중 | `go.uber.org/goleak`으로 모든 테스트에서 `goleak.VerifyNone(t)` 실행, ctx 전파 (pgx + go-redis 모두 ctx 지원) |
| **R-CTRL-005** | Go-side audit_logs 작성과 Python-side audit_logs 작성이 schema/format 불일치 | 중 | 중 | audit_logs 테이블은 SPEC-AX-001 Sprint 0에서 단일 schema로 정의됨. Go `store.InsertAuditLog`와 Python `pkg/logging/logger.py audit_event`가 동일 컬럼 셋(user_id, action, resource_id, resource_type, timestamp, details)을 사용. 통합 테스트에서 두 경로 모두 검증 |

**Re-planning Gate** (per `spec-workflow.md`):
- 3+ 연속 sub-sprint zero new AC met → manager-strategy 재호출
- coverage 직전 sprint 대비 하락 → 재호출
- new lint errors > fixed lint errors per cycle → 재호출

---

## 6. MX Tag Plan

`code_comments: ko` (`.moai/config/sections/language.yaml`) — MX 태그 한국어 사용.

### S1 (state_machine.go, handlers.go)

```go
// @MX:ANCHOR: 워크플로우 상태 전이의 단일 진실 공급원
// @MX:REASON: handlers.go, callback.go, gRPC/REST handler 모두 본 함수 경유 (fan_in >= 3)
// @MX:SPEC: SPEC-AX-CTRL-001 REQ-CTRL-001
func (sm *StateMachine) Transition(...) error { ... }

// @MX:WARN: 단일 workflow_id 동시 전이 시 SELECT FOR UPDATE 누락 시 race condition
// @MX:REASON: SPEC-AX-CTRL-001 REQ-CTRL-001-S1 invariant — pgx tx + FOR UPDATE 필수
// @MX:SPEC: SPEC-AX-CTRL-001 REQ-CTRL-001
```

### S2 (postgres.go)

```go
// @MX:ANCHOR: 워크플로우 store의 모든 쓰기 경로 진입점
// @MX:REASON: handlers, callback, dispatcher 모두 호출 (fan_in >= 3)
// @MX:SPEC: SPEC-AX-CTRL-001 REQ-CTRL-004
func (s *Store) LockWorkflowForUpdate(...) (*Workflow, error) { ... }
```

### S3 (dispatcher.go)

```go
// @MX:WARN: Redis 미가용 시 exponential backoff 3회 후 워크플로우 FAILED 전이
// @MX:REASON: 재시도 동안 caller goroutine block — 100ms p99 SLO 초과 가능
// @MX:SPEC: SPEC-AX-CTRL-001 REQ-CTRL-005-S1
func (d *Dispatcher) Dispatch(...) error { ... }
```

### S4 (server.go)

```go
// @MX:NOTE: gRPC + REST gateway 동시 기동, errgroup 기반 graceful shutdown
// @MX:SPEC: SPEC-AX-CTRL-001 REQ-CTRL-002-E1
func (s *Server) Run(ctx context.Context) error { ... }
```

기존 Sprint 0 stub의 `@MX:TODO`(Sprint 7) 표시는 S1-S5 GREEN 종료 시 모두 제거된다.

---

## 7. Sprint Contract Outline (Thorough Harness)

각 sub-sprint S1-S5는 `.moai/sprints/SPEC-AX-CTRL-001/sprint-S{n}.md`에 evaluator-active가 동적 생성한다.

**공통 가중치**:
- Sprint S1 (state machine): Functionality 50% / Security 25% / Completeness 15% / Originality 10%
- Sprint S2 (store): Functionality 40% / Security 30% (SQL injection / SELECT FOR UPDATE) / Completeness 20% / Originality 10%
- Sprint S3 (celery): Functionality 40% / Originality 30% (Celery 프로토콜 호환성) / Completeness 20% / Security 10%
- Sprint S4 (gRPC): Functionality 45% / Security 25% / Completeness 20% / Originality 10%
- Sprint S5 (REST): Completeness 40% / Functionality 35% / Security 15% / Originality 10%

**Drift Guard**: 각 sub-sprint REFACTOR 후 planned files (본 plan §1) vs actual modified files 비교, 누적 drift > 30% 시 Re-planning Gate.

---

## 8. LSP Baseline Strategy

본 SPEC 시작 시점 baseline (SPEC-AX-001 GREEN 종료 시점 가정):
```
errors: 0
type_errors: 0
lint_errors: 0
warnings: 0 (Go 1.22 deprecation은 없음)
```

각 sub-sprint REFACTOR 종료 시 `moai lsp status` 0/0/0 유지 확인. `quality.yaml run.allow_regression=false` 적용.

Go 도구체인:
- `go vet ./apps/control-plane/...` — 0 issue
- `golangci-lint run ./apps/control-plane/...` — default + gosec, 0 issue
- `goimports -l apps/control-plane/` — 0 diff
- `go test -race -cover ./apps/control-plane/...` — coverage ≥ 85%, race detector clean

---

## 9. Specialist Routing

| Sub-sprint | Owner Agent | 보조 Agent | 비고 |
|-----------|-------------|-----------|------|
| S1 (state machine) | manager-tdd | expert-backend (Go) | 핵심 invariant 로직 |
| S2 (postgres store) | manager-tdd | expert-backend (Go + pgx), expert-testing (testcontainers-go integration) | 동시성 단위 테스트 강화 |
| S3 (celery dispatch) | manager-tdd | expert-backend (Go + Celery), expert-devops (Kombu protocol 분석) | research.md §3 참조 |
| S4 (gRPC server) | manager-tdd | expert-backend (Go + grpc-gateway), expert-security (gRPC interceptor 보안) | bufconn 테스트 |
| S5 (REST gateway) | manager-tdd | expert-backend (Go) | OpenAPI 정합성 |
| 매 sub-sprint 종료 후 | manager-quality | (LSP gate + golangci-lint + TRUST 5) | quality.yaml 강제 |
| 매 sub-sprint 종료 후 | evaluator-active | (per-sprint scoring, strict profile, 4-dim) | thorough harness |
| 모든 sub-sprint 완료 후 | manager-git | (conventional commit + branch 정리) | 본 SPEC 종료 시 |

**Worktree 정책**:
- expert-backend가 write-heavy로 호출될 때 `isolation: "worktree"` 적용
- expert-testing read-only 분석 시 isolation 없이 호출
- expert-security read-only audit 시 isolation 없이 호출

---

## 10. Definition of Done (본 SPEC 전체)

- [ ] S1 ~ S5 모두 GREEN (모든 AC 통과)
- [ ] coverage ≥ 85% (Go 모듈 전체)
- [ ] LSP errors=0, type-errors=0, lint-errors=0
- [ ] TRUST 5 5개 차원 모두 통과
- [ ] `tests/integration/test_control_plane_to_pipelines.py` E2E 통합 테스트 통과 — Go control-plane이 Python Celery worker를 실제로 호출하여 워크플로우가 COMPLETED 상태에 도달
- [ ] @MX 태그: Sprint 0의 TODO 모두 해소 + 신규 ANCHOR/WARN/NOTE 추가
- [ ] evaluator-active per-sprint scoring 모두 ≥ 0.75 (strict profile)
- [ ] plan-auditor 1차 audit 통과 (본 plan + spec.md + acceptance.md + research.md 정합성)
