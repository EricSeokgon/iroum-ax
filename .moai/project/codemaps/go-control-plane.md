# Go Control Plane 코드맵 (SPEC-AX-CTRL-001)

## 개요

`apps/control-plane/` (Go 1.22+) 는 SPEC-AX-001 Python 파이프라인을 오케스트레이션하는 Walking Skeleton이다.
12개 내부 패키지 + 진입점으로 구성되며, gRPC + REST 이중 서버를 통해 워크플로우 생성/조회/완료를 제공한다.

---

## 패키지 아키텍처

### 1. Entrypoint & Server (`cmd/server/`)

**main.go** (OS 진입점 — SPEC-AX-SERVER-001 Sprint 1)
- Environment 변수 파싱 → config.Config 로드
- Zap logger 초기화
- server.New() → 의존성 주입 11단계
- server.Run() → dual listener + graceful shutdown
- @MX:NOTE: 서버 OS 진입점

**server.go** (Server bootstrap + dual listener — SPEC-AX-SERVER-001 Sprint 0-1)
- Server.New(ctx, config) → Server struct 생성 (11-step DI: config → store → redis → oidc → jwks → auth → coordinator → dispatcher → handler → chain)
- Server.Run(ctx) → errgroup 기반 dual listener (gRPC :50051 + REST :8080 concurrent)
- Server.gracefulShutdown(ctx, 30s) → SIGTERM/SIGINT 처리 + sync.Once idempotency + reverse cleanup
- @MX:ANCHOR: Server.New, Server.Run, Server.startListeners (fan_in ≥ 3)
- @MX:WARN: gracefulShutdown (sync.Once 3-component race + double-signal force-kill)

**probes.go** (Health/readiness probe — SPEC-AX-SERVER-001 Sprint 1)
- /health (liveness, 항상 200 OK)
- /ready (readiness, DB ping + Redis ping + JWKS reachable 검증)
- DefaultReadinessProbeFn(ctx) → PgWorkflowStore.Ping + RedisClient.Ping + JWKSCache.Reachable 이순차 검증
- @MX:ANCHOR: DefaultReadinessProbeFn (liveness/readiness 로직 진입점)

**redis_adapter.go** (goRedisAdapter production promotion — SPEC-AX-SERVER-001 Sprint 0)
- Wraps github.com/redis/go-redis/v9 *redis.Client → scheduler.RedisClient interface
- RPush(ctx, key, ...values) → client.RPush().Result() (int64, error)
- Ping(ctx) → client.Ping().Err() (error)
- Close() → client.Close() (error)
- Previously test-only goRedisAdapter in e2e_test.go, promoted to production code

**grpc_handlers.go** (gRPC service 구현 — SPEC-AX-CTRL-001)
- CreateWorkflow RPC
- GetWorkflow RPC
- ListWorkflows RPC + pagination
- @MX:ANCHOR: CreateWorkflow (fan_in 3+: gRPC + REST + dispatch callback)

**rest_handler.go** (REST 엔드포인트)
- POST /api/v1/workflows
- GET /api/v1/workflows/{id}
- GET /api/v1/workflows?limit=10&offset=0
- /healthz (헬스체크)

**health.go** (헬스체크 엔드포인트)
- /healthz 구현 (Kubernetes readiness probe)

**middleware.go** (미들웨어 체인)
- RequestID 미들웨어 (uuid.New())
- 구조화 JSON 로깅 (zap)
- 에러 응답 포맷팅

---

### 2. 워크플로우 상태 머신 (`internal/workflow/`)

**types.go** (정의 생략, `internal/types/` 참조)
- WorkflowState enum (PENDING, RUNNING, COMPLETED, FAILED)
- Workflow struct (id, status, created_at, result)

**state_machine.go** (불변성 규칙)
```
상태: PENDING → RUNNING → (COMPLETED | FAILED)
전이:
  - Start: PENDING → RUNNING (audit: WORKFLOW_TRANSITIONED_TO_RUNNING)
  - Complete: RUNNING → COMPLETED (결과 JSON 저장)
  - Fail: RUNNING → FAILED (에러 메시지 저장)
```
- @MX:ANCHOR: CanTransition() (all codepaths verify validity)
- @MX:ANCHOR: Transition() (audit recording + state update atomicity)
- StateMachine struct: Store + AuditRecorder 의존성

**handlers.go** (워크플로우 생명주기)
- CreateWorkflow: 초기 상태 PENDING + audit 1건 + dispatcher 호출
- Start: PENDING → RUNNING 전이 + audit
- Complete: RUNNING → COMPLETED 전이 + result 저장
- Fail: RUNNING → FAILED 전이 + error 저장
- @MX:ANCHOR: CreateWorkflow (3개 호출자)

**callback.go** (Python worker 콜백)
- POST /api/v1/workflows/{id}/callback
- worker가 작업 완료 후 상태 전이 트리거
- RUNNING → COMPLETED | FAILED
- audit 기록 (WORKFLOW_COMPLETED or WORKFLOW_FAILED)

---

### 3. 데이터 저장소 (`internal/store/`)

**store.go** (인터페이스 정의)
```go
type WorkflowStore interface {
  CreateWorkflow(ctx, workflow) error
  GetWorkflow(ctx, id) (*Workflow, error)
  ListWorkflows(ctx, limit, offset) ([]*Workflow, error)
  Tx(ctx) (WorkflowTx, error)  // 트랜잭션 시작
}

type WorkflowTx interface {
  CreateWorkflow(ctx, workflow) error
  GetWorkflow(ctx, id) (*Workflow, error)
  UpdateWorkflow(ctx, id, workflow) error
  Commit(ctx) error
  Rollback(ctx) error
}
```
- @MX:ANCHOR: WorkflowStore.Tx() (fan_in 2: state_machine + dispatcher)

**postgres.go** (pgx/v5 풀 관리)
- PgWorkflowStore struct: *pgxpool.Pool
- NewPgWorkflowStore: DSN 파싱 + Ping + 인덱스 검증
- @MX:WARN: DSN 기본값 "iroum:iroum@localhost" (env 오버라이드 권장)

**pg_store.go** (PostgreSQL 구현)
- CreateWorkflow: INSERT + RETURNING id
- GetWorkflow: SELECT (레이스 조건 없음)
- ListWorkflows: SELECT + ORDER BY created_at DESC LIMIT OFFSET
- UpdateWorkflow: UPDATE (SELECT FOR UPDATE 래핑)
- @MX:ANCHOR: UpdateWorkflow (SELECT FOR UPDATE 직렬화)

**pg_tx.go** (트랜잭션 래퍼)
- pgx.Tx 기반
- CreateWorkflow, GetWorkflow, UpdateWorkflow (트랜잭션 컨텍스트)
- Commit, Rollback

**fake_store.go** (테스트용 인메모리)
- FakeStore struct: sync.Map (workflows)
- FakeTx struct: 임시 변경 맵 + Commit/Rollback 직렬화
- RED 테스트용 stub

**audit.go** (감시 로그 헬퍼)
- InsertAuditLog(ctx, event) error
- audit_logs 테이블 INSERT (JSONB: action, user_id, workflow_id, timestamp)

---

### 4. 감시 및 로깅 (`internal/audit/`)

**audit.go** (감시 이벤트 정의)
```go
const (
  ActionWorkflowCreated = "WORKFLOW_CREATED"
  ActionWorkflowTransitionedToRunning = "WORKFLOW_TRANSITIONED_TO_RUNNING"
  ActionWorkflowCompleted = "WORKFLOW_COMPLETED"
  ActionWorkflowFailed = "WORKFLOW_FAILED"
  // 등등 8개
)

type Event struct {
  Action string
  UserID string
  WorkflowID string
  Timestamp time.Time
  Metadata map[string]interface{}
}
```

**recorder.go** (감시 기록)
- Recorder interface: Record(ctx, event) error
- PgAuditRecorder: audit_logs 테이블 INSERT
- @MX:ANCHOR: Record() (fan_in 3+: all handlers)

---

### 5. 스케줄러 및 Dispatch (`internal/scheduler/`)

**dispatcher.go** (Celery 디스패치)
- CeleryDispatcher struct: RedisClient 의존성
- Dispatch(ctx, workflow) error
  1. BuildEnvelope() → Kombu v2 JSON 생성
  2. redis.RPUSH(celery_queue, envelope)
  3. 에러 → workflow.status = FAILED
- @MX:WARN: Redis 연결 실패 시 graceful 에러 처리 필요
- @MX:ANCHOR: Dispatch() (fan_in 2: CreateWorkflow + StateTransition)

**celery_envelope.go** (Kombu v2 빌더)
- BuildEnvelope(workflow) → []byte (JSON)
- 필드:
  - body: base64([args, kwargs, empty])
  - headers: {id, task, exchange, routing_key}
  - properties: {correlation_id, reply_to, delivery_mode}
- @MX:NOTE: base64 인코딩 (Kombu 호환성)

---

### 6. 증빙 관리 (`internal/store/evidence.go`, `internal/storage/`, `internal/audit/recorder.go`, `cmd/server/evidence_handlers.go`)

> SPEC-AX-EVID-001 v0.1.0 — 경영평가 증빙 자료 수집/관리

**evidence.go** (`internal/store/evidence.go` — EvidenceTx 구현)
- `PgEvidenceTx` struct: pgx TX 래퍼, `EvidenceTx` 인터페이스 구현
- `BeginEvidenceTx(ctx) (EvidenceTx, error)`: `PgWorkflowStore.pool`에서 새 pgx TX 시작 (신규 pool 생성 금지 — 단일 pool 재사용)
- `InsertEvidence(ctx, evalItemID, fileName, contentType, fileSize, fileHash, storageStrategy, storageLocation, metadata, fileContent BYTEA, prevVersionID)` → `(uuid.UUID, error)`
- `GetEvidenceByID(ctx, id)` → `(*Evidence, error)` — not found 시 `ErrEvidenceNotFound` 래핑
- `GetLatestVersionByEvalItem(ctx, evalItemID)` → `(*Evidence, error)` — `SELECT ... FOR UPDATE` 직렬화
- `ListEvidenceByEvalItem(ctx, evalItemID)` → `([]*Evidence, error)`
- `MarkSuperseded(ctx, id)` → `error` — 직전 버전 `status=SUPERSEDED` 전이 (본문/파일 메타 불변)
- `InsertAuditLog`, `Commit`, `Rollback`
- @MX:ANCHOR: `BeginEvidenceTx` (핸들러 + 통합 테스트 + 감사 검증 3곳 이상 호출)

**storage.go** (`internal/storage/storage.go` — 저장 전략 추상화)
- `EvidenceBlobStore` interface: `Put(ctx, key, io.Reader) (string, error)`, `Get(ctx, location) (io.ReadCloser, error)`
- `dbBlobStore` struct: database_blob 전략 구현체 — blob bytes는 이 인터페이스를 통과하지 않음; `Put`은 논리 위치 `db://evidences/<key>` 만 반환 (실제 바이너리는 `InsertEvidence(file_content)` 경유)
- `NewDBBlobStore() EvidenceBlobStore`: 외부 의존 0 (자격증명/네트워크 endpoint 없음 — REQ-EVID-UBI-001)
- @MX:NOTE: 저장 전략 database_blob 확정 — 추상화 유지로 filesystem/minio 전환 대비 (REQ-EVID-004)

**recorder.go** (`internal/audit/recorder.go` — 증빙 감사 메서드 추가)
- `RecordEvidenceCreated(ctx, tx, evidenceID, evalItemID, fileHashSHA256, version, userID)`: `EVIDENCE_CREATED` 액션 audit_logs INSERT — details `{evaluation_item_id, version, file_hash_sha256}`
- `RecordEvidenceVersioned(ctx, tx, evidenceID, evalItemID, fileHashSHA256, version, previousVersionID, userID)`: `EVIDENCE_VERSIONED` 액션 — details에 `previous_version_id` 포함
- 두 메서드 모두 `r.nowUTC()` (Clock 주입) 사용, `r.resolveUserID()` 로 cli-anonymous 기본값 적용
- @MX:ANCHOR: `RecordEvidenceCreated`, `RecordEvidenceVersioned` (핸들러+통합테스트+감사 검증 3곳 이상)

**clock.go** (`internal/audit/clock.go` — 시각 주입 추상화)
- `Clock` interface: `NowUTC() time.Time`
- `systemClock` struct: `time.Now().UTC()` 반환 (기존 동작과 byte-identical)
- `defaultClock Clock = systemClock{}`: Recorder가 명시적 Clock 미주입 시 사용
- `WithClock(c Clock) RecorderOption`: 테스트에서 고정 시각 주입 (R-EVID-007)

**evidence_handlers.go** (`cmd/server/evidence_handlers.go` — 단일 증빙 핸들러)
- `EvidenceHandler` struct: `store.EvidenceStore`, `evidenceRecorder`, `storage.EvidenceBlobStore`, `*zap.Logger`, `maxFileBytes int64`, `dupSignal bool`
- `NewEvidenceHandler(st, rec, blob, logger, maxFileBytes, dupSignal)`: 핸들러 생성
- **단일 라우트**: `Routes() http.Handler` → `mux.HandleFunc("POST /api/v1/evidences", h.handleCreateEvidence)` (GAP-01 — `/evidences/{id}/versions` 별도 라우트 없음)
- **단일 핸들러** `handleCreateEvidence`: Content-Type 검증(SEC-01) → Content-Length 사전 거부(SEC-02.1) → multipart SHA-256 단일 패스 스트리밍(F1, SEC-02.2/3) → pre-TX 입력 검증(SEC-05) → `BeginEvidenceTx` → defer Rollback(SEC-07) → `GetLatestVersionByEvalItem` 버전 결정 → `InsertEvidence(file_content)` → `MarkSuperseded` → `RecordEvidence{Created|Versioned}` → Commit → 201 `{evidence_id, version}`
- `parseAndHashMultipart(r, maxBytes)`: `io.MultiWriter(&buf, sha256.New())` + `io.LimitReader` 단일 패스
- `resolveVersion(ctx, tx, evalItemID, fileHash)`: SELECT FOR UPDATE 직렬화, dupSignal 처리
- @MX:ANCHOR: `EvidenceHandler` (핸들러 테스트 + 서버 마운트 + 통합 테스트 3곳 이상)
- @MX:WARN: `BeginEvidenceTx` 이후 Commit 전 panic/early-return 시 orphan 증빙 행 누출 — defer Rollback이 즉시 등록되어야 함 (SEC-07)

**config.go 추가 항목** (`internal/config/config.go`)
- `EvidenceStorageStrategy string` — env `EVIDENCE_STORAGE_STRATEGY`, 기본 `database_blob`
- `EvidenceMaxFileBytes int64` — env `EVIDENCE_MAX_FILE_BYTES`, 기본 52428800 (50 MiB)
- `EvidenceDuplicateSignalEnabled bool` — env `EVIDENCE_DUPLICATE_SIGNAL_ENABLED`, 기본 `false`
- `Validate()` / `LoadConfig()`: storage strategy 열거 검증 fail-fast

**errors.go 추가 항목** (`internal/errors/errors.go`)
- `ErrEvidenceNotFound`: `GetEvidenceByID` pgx.ErrNoRows 래핑 (GAP-03)
- `ErrEvidenceImmutable`: successor 존재 시 본문 컬럼 변경 시도 차단 (GAP-04, REQ-EVID-UBI-004)

---

### 7. 설정 및 타입 (`internal/config/`, `internal/types/`, `internal/errors/`, `internal/log/`)

**config.go** (환경변수 파서)
```
PostgresDSN (기본: iroum:iroum@localhost:5432)
RedisURL (기본: redis://localhost:6379)
GrpcPort (기본: 50051)
RestPort (기본: 8080)
LogLevel (기본: info)
```
- Load(): env 파싱 + 검증
- @MX:WARN: PostgresDSN dev 기본값 (production env 강제)

**types.go** (워크플로우 타입)
- WorkflowState enum
- Workflow struct
- @MX:ANCHOR: Workflow (모든 핸들러, store, dispatcher가 참조)

**errors.go** (Sentinel 에러)
- ErrNotFound
- ErrInvalidTransition
- ErrInvalidState
- ErrDispatchFailed
- ErrAuditFailed

**log.go** (zap 로거 팩토리)
- NewLogger(level string) *zap.Logger
- 구조화 JSON 로깅 (productionConfig)

---

### 7. Protobuf 정의 (`internal/proto/`)

**workflow.pb.go** (수동 작성 proto 메시지)
- WorkflowStatus enum
- Workflow message
- CreateWorkflowRequest/Response
- GetWorkflowRequest/Response
- ListWorkflowsRequest/Response

**workflow_grpc.pb.go** (수동 작성 gRPC 서비스)
- WorkflowServiceServer interface
- CreateWorkflow, GetWorkflow, ListWorkflows 메서드

---

## 의존성 그래프

```
main.go
  ↓
server.go (config, log, grpc_handlers, rest_handler)
  ├─→ grpc_handlers.go
  │     ├─→ state_machine.go (Store, AuditRecorder)
  │     └─→ handlers.go (CreateWorkflow, etc.)
  │           ├─→ store/pg_store.go
  │           ├─→ audit/recorder.go
  │           └─→ scheduler/dispatcher.go
  │
  └─→ rest_handler.go
        ├─→ grpc_handlers.go (로직 공유)
        └─→ server.go (HTTP 래퍼)

store/pg_store.go
  ├─→ store.go (인터페이스)
  ├─→ config.go (PostgresDSN)
  └─→ audit.go (InsertAuditLog)

scheduler/dispatcher.go
  ├─→ scheduler/celery_envelope.go
  ├─→ store.go (WorkflowStore 의존)
  └─→ config.go (RedisURL)

workflow/state_machine.go
  ├─→ types.go (WorkflowState)
  ├─→ store.go (WorkflowStore)
  └─→ audit.go (AuditRecorder)
```

---

## Fan-In/Fan-Out 분석

### High Fan-In (>=3 호출자)

| 함수 | 호출자 | @MX 태그 |
|------|--------|----------|
| CreateWorkflow() | REST + gRPC 핸들러 + 테스트 | @MX:ANCHOR |
| Dispatch() | CreateWorkflow + StateTransition + 콜백 | @MX:ANCHOR |
| Record() | 모든 핸들러 (Create/Start/Complete/Fail) | @MX:ANCHOR |
| GetWorkflow() | REST GET + gRPC GET + 콜백 검증 | @MX:ANCHOR |

### Low Fan-In (<3 호출자)

- UpdateWorkflow(): state_machine.Transition + callback
- Start(), Complete(), Fail(): 각각 고유 코드경로
- Health(): REST 핸들러만

---

## 테스트 커버리지

| 패키지 | 테스트 파일 | 테스트 수 | 빌드 태그 |
|--------|-----------|---------|----------|
| internal/workflow | state_machine_test.go | 14 | 없음 |
| internal/store | pg_store_test.go | 11 | integration |
| internal/audit | recorder_test.go | 11 | 없음 |
| cmd/server | grpc_handlers_test.go, rest_handler_test.go | 24 | 없음 |
| internal/scheduler | dispatcher_test.go | 15 | 없음 |
| E2E | e2e_test.go | 5 | integration |
| **총합** | | **80** | mixed |

---

## REQ-CTRL 매핑

| REQ | 핵심 구현 | 테스트 | 상태 |
|-----|---------|-------|------|
| REQ-CTRL-UBI-001 | AuditRecorder, Event | recorder_test.go | ✓ PASS |
| REQ-CTRL-UBI-002 | 8 Action enum, audit_logs INSERT | audit_test.go, handler tests | ✓ PASS |
| REQ-CTRL-001 | StateMachine, CanTransition, Transition | state_machine_test.go | ✓ PASS |
| REQ-CTRL-002 | gRPC RPC × 3, bufconn test | grpc_handlers_test.go | ✓ PASS |
| REQ-CTRL-003 | REST endpoints, httptest | rest_handler_test.go | ✓ PASS |
| REQ-CTRL-004 | PgWorkflowStore, SELECT FOR UPDATE | pg_store_test.go (integration) | ✓ PASS |
| REQ-CTRL-005 | Dispatcher, celery envelope | dispatcher_test.go | ✓ PASS |
| AC-CTRL-E2E-1 | Full flow: create → transition → dispatch | e2e_test.go | ✓ PASS (5/5) |

---

## 설정 및 로깅

### 환경변수

```bash
POSTGRES_DSN=postgres://user:pass@host:5432/db  # 기본: iroum:iroum@localhost:5432
REDIS_URL=redis://host:6379                      # 기본: redis://localhost:6379
GRPC_PORT=50051                                   # 기본: 50051
REST_PORT=8080                                    # 기본: 8080
LOG_LEVEL=info                                    # 기본: info
```

### 로깅 형식

```json
{
  "level": "info",
  "ts": 1715708414.123,
  "caller": "server/grpc_handlers.go:45",
  "msg": "CreateWorkflow",
  "request_id": "uuid-xxx",
  "workflow_id": "uuid-yyy",
  "status": "PENDING"
}
```

---

## 주요 불변성

1. **상태 전이 불변성**: PENDING → RUNNING → (COMPLETED | FAILED) 만 허용
2. **원자성**: audit INSERT 실패 시 workflow 생성 롤백
3. **Dispatch 원자성**: RPUSH 실패 시 workflow.status = FAILED (보정)
4. **직렬화**: SELECT FOR UPDATE로 동시 상태 전이 직렬화

---

## 성능 특성

| 작업 | 목표 | 달성 | 비고 |
|------|------|------|------|
| Dispatch (Celery envelope) | < 100ms p99 | 7μs/op avg | Redis RPUSH 경량 |
| GetWorkflow | < 50ms p99 | ~5ms (network 의존) | SELECT 인덱스 최적화 |
| CreateWorkflow | < 200ms p99 | ~50ms (audit포함) | 트랜잭션 오버헤드 |
| ListWorkflows (N=100) | < 500ms p99 | ~100ms | 페이지네이션 인덱스 |

---

## 8. 인증 및 권한 관리 (`internal/auth/`, `pipelines/auth/`) — SPEC-AX-AUTH-001

### Go 인증 모듈 (`apps/control-plane/internal/auth/`)

| 파일 | 책임 | fan_in | @MX 태그 |
|------|------|--------|----------|
| validator.go | TokenValidator: JWT sig + iss(SF-1) + alg/kty(SF-2) + aud/exp 검증 | 4 | ANCHOR |
| oidc.go | OIDCClient: Discovery + JWKS 지원 | 2 | - |
| jwks_cache.go | JWKSCache: TTL 3600s + max-age 4h stale-while-revalidate | 3 | - |
| middleware.go | UnaryInterceptor(gRPC) + RESTMiddleware + Health bypass | 5 | ANCHOR |
| rbac.go | ParseRolesFromScope + EffectivePermissions + Authorize + LogForbidden | 4 | ANCHOR |
| refresh.go | RefreshService: RefreshSession + Logout + OAuth 2.0 BCP family invalidation | 2 | ANCHOR |
| errors.go | 11 sentinel errors (ErrTokenExpired, InvalidIssuer/SF-1, AlgorithmKeyMismatch/SF-2, ...) | many | - |
| context.go | WithUser(ctx, user) + UserFromContext(ctx) | 3+ | - |

### Python 인증 모듈 (`pipelines/auth/`)

| 파일 | 책임 | 테스트 | 비고 |
|------|------|--------|------|
| validator.py | TokenValidator: 동기 JWT 검증 (SF-1/SF-2) | 7 | FastAPI 통합 |
| errors.py | TokenError, InvalidIssuerError, AlgorithmKeyMismatchError | - | - |
| celery_auth.py | async user_id 추출 (envelope.headers → context) | 8 | Celery 콜백 |

### 보안 방어

**SF-1 (Issuer Spoofing)**:
- RFC 7519 §4.1.1 발행자 검증
- 각 토큰의 'iss' 클레임을 Keycloak 자체 호스트 URL과 정확히 비교
- Cross-realm 토큰 재사용 공격 차단

**SF-2 (Algorithm Confusion Attack)**:
- JWT alg 헤더 추출 → 허용 목록(RS256/EdDSA/ES256) 확인
- JWKS 키의 kty(RSA/EC/OKP) 추출
- alg ↔ kty 교차 검증 (예: alg=RS256 → kty=RSA 요구)
- OWASP JWT cheat sheet 준수

**OAuth 2.0 BCP 준수**:
- Refresh token rotation: 각 갱신 시 새 토큰 발급
- Family invalidation: 재사용된 refresh_token 발견 시 entire family 무효화
- Token blacklist: Logout 시 refresh_token_family 기록

### 테스트 매트릭스

| 범주 | 테스트 | 커버리지 |
|------|--------|----------|
| JWT Validator | 19개 (SF-1/SF-2/alg/aud/exp/kid) | 92% |
| OIDC + JWKS | 17개 (Discovery/TTL/stale-while-revalidate/concurrent) | 85% |
| Middleware | 20개 (gRPC/REST/Health/AuthDisabled/malformed) | 95%+ |
| RBAC | 18개 (3역할 매트릭스/admin/analyst/viewer) | 85% |
| Refresh/Logout | 13개 (family invalidation/reuse detection) | 90% |
| Python FastAPI | 7개 (TokenValidator 동기 호출) | 80% |
| Celery Cross-SPEC | 8개 (envelope.headers.user_id 전파) | 85% |
| E2E | 4 PASS + 1 SKIP (REST handler SPEC-AX-AUTH-002 연기) | - |
| **합계** | 105 신규 테스트 | ~87% avg |

### 의존성

**Go**:
- github.com/golang-jwt/jwt/v5 (JWT parsing + validation)
- github.com/coreos/go-oidc/v3 (OIDC Discovery)
- github.com/MicahParks/keyfunc/v3 (JWKS fetching)

**Python**:
- PyJWT[cryptography] (JWT validation)
- authlib (OIDC client, OAuth 2.0)

---

## 후속 SPEC 후보

- **SPEC-AX-AUTH-002**: RBAC REST handler 통합 (E2E SKIP 항목)
- **SPEC-AX-AUTH-EGOV-001**: 전자정부 표준 인증 (KEPCO 요구 시)
- **SPEC-AX-AUTH-MFA-001**: 다단계 인증 (2FA/TOTP)
- **SPEC-AX-COV-001**: 통합 커버리지 측정 도구
- **SPEC-AX-AUD-001**: internal/audit 에러 경로 보강
- **SPEC-AX-RETRY-001**: Celery retry 정책 (exponential backoff)
