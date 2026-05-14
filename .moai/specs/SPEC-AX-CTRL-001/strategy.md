# SPEC-AX-CTRL-001 Implementation Strategy — Phase 1 (manager-strategy)

> **Plan mode 활성 안내**: 본 문서는 본래 `.moai/specs/SPEC-AX-CTRL-001/strategy.md`로 작성되어야 하나, 현재 plan 모드가 활성화되어 있어 지정 plan 파일 경로(`/home/sklee/moai/iroum-ax/.moai/plans/enumerated-plotting-manatee-agent-a5b25a6d32551c5c0.md`)에 작성한다. plan 모드 해제 후 orchestrator가 동일 내용을 `.moai/specs/SPEC-AX-CTRL-001/strategy.md`로 이동/재생성해야 한다. 이는 SPEC-AX-001 strategy.md가 따랐던 패턴과 동일.

- 작성일: 2026-05-14
- 작성자: manager-strategy (Opus 4.7, Adaptive Thinking 활성)
- 대상 SPEC: SPEC-AX-CTRL-001 v0.1.1 (draft, plan-auditor iter 2 PASSED 가정)
- 개발 방법론: TDD (RED-GREEN-REFACTOR) per `quality.yaml development_mode`
- Harness 레벨: thorough (per-sprint evaluator-active, sprint_contract 필수)
- 목표: SPEC-AX-CTRL-001 Go Control Plane Walking Skeleton 구현 전략 — Phase 2 RED 진입 전 Sprint 분해·시퀀싱·라이브러리 결정·Sprint Contract scaffold·리스크 mitigation 확정

---

## 0. Phase 1 산출물 요약 (Quick Reference)

| 항목 | 결정 사항 |
|------|----------|
| Sprint 총수 | **8** (Pre-Sprint 0 환경 검증 + Sprint 0 Foundation + Sprint 1 UBI + Sprint 2 State Machine + Sprint 3 Postgres Store + Sprint 4 gRPC Server + Sprint 5 REST Gateway + Sprint 6 Celery Dispatch + Sprint 7 E2E) |
| Critical path REQ ordering | go.mod 정리 → pkg/models 확장 → REQ-CTRL-UBI-001/002 → REQ-CTRL-001(State Machine) → REQ-CTRL-004(Postgres Store) → REQ-CTRL-002(gRPC) → REQ-CTRL-003(REST) → REQ-CTRL-005(Celery Dispatch) → E2E |
| Top 3 risks | R-CTRL-001 Celery envelope 호환성, R-CTRL-003 PENDING 고아 워크플로우, R-CTRL-002 State machine race |
| Open question 수 | 4 (모두 sensible default 보유, orchestrator AskUserQuestion 1라운드 가능) |
| Phase 2 RED entry | **YES** (Pre-Sprint 0 ~ Sprint 7 모두 sensible default 적용 가능) |

> **주요 차이점 vs plan.md S1~S5**: plan.md는 5개 sub-sprint로 분해(S1 State→S2 Store→S3 Dispatch→S4 gRPC→S5 REST)했으나, 본 strategy는 **Postgres Store를 gRPC보다 먼저** 배치(S2→S3)하고 **Celery Dispatch를 가장 뒤로** 이동(S6)한다. 근거는 §1.2 참조 — Sprint Contract artifact 안정성과 testcontainers 의존성 단일화를 위한 보수적 시퀀싱.

---

## 1. Sprint Sequencing (REQ Dependency DAG → Sprint Order)

### 1.1 Topological order

```
[Pre-Sprint 0: Environment Verification & go.mod hydration]
        ↓
[Sprint 0 CTRL: Foundation]
   - go.mod 의존성 추가 (grpc, pgx, redis, zap, testify, testcontainers-go, grpc-gateway)
   - buf codegen 환경 (workflow.proto에 service 정의 추가, internal/proto/ax/v1/ 생성)
   - pkg/models/workflow.go 확장 (Status/Workflow Go 타입 단일 진실 source)
   - apps/control-plane/config/config.go (env 파서)
   - apps/control-plane/internal/audit/ 디렉토리 신규
        ↓
[Sprint 1 CTRL: REQ-CTRL-UBI-001/002 횡단 불변]
   - pkg/logging/audit.go Go-side audit helper (SPEC-AX-001 audit_logs 스키마 단일 source)
   - internal/audit/event.go 8종 액션 enum
   - 트랜잭션 원자성 invariant 테스트 골격
        ↓
[Sprint 2 CTRL: REQ-CTRL-001 Workflow State Machine]
   - internal/workflow/state_machine.go (4 states × 4 states 전이 매트릭스)
   - internal/workflow/handlers.go (Create/Callback)
   - SELECT FOR UPDATE 호출 위치 정의 (실제 pgx 호출은 Sprint 3)
        ↓
[Sprint 3 CTRL: REQ-CTRL-004 PostgreSQL Store]
   - internal/store/postgres.go (pgxpool.New + CRUD + LockWorkflowForUpdate)
   - internal/store/audit.go (InsertAuditLog tx-aware)
   - .moai/db/schema/migrations/0002_workflow_indexes.sql
   - testcontainers-go(postgres:16) 통합 테스트
        ↓
[Sprint 4 CTRL: REQ-CTRL-002 gRPC Server]
   - schemas/proto/workflow.proto 확장 (WorkflowService 서비스 + google.api.http 어노테이션)
   - cmd/server/server.go (errgroup 기반 gRPC :50051)
   - cmd/server/grpc_handlers.go (WorkflowService 구현)
   - cmd/server/middleware.go (zap interceptor + uuid v7 request_id)
   - bufconn + goleak 단위 테스트
        ↓
[Sprint 5 CTRL: REQ-CTRL-003 REST API (gRPC-Gateway)]
   - cmd/server/server.go에 gRPC-Gateway mux 추가
   - cmd/server/health.go (/healthz)
   - cmd/server/rest_handlers_test.go (httptest)
   - schemas/openapi/openapi.yaml 확장
        ↓
[Sprint 6 CTRL: REQ-CTRL-005 Celery Dispatch]
   - internal/scheduler/celery_envelope.go (Celery protocol v2 envelope 빌더)
   - internal/scheduler/dispatcher.go (go-redis/v9 + 3-step backoff retry)
   - testdata/celery_envelope_v2.json (Python kombu 측 생성 후 커밋)
   - miniredis 기반 단위 + 벤치 테스트
        ↓
[Sprint 7 CTRL: E2E Integration]
   - tests/integration/test_control_plane_to_pipelines.py
   - docker-compose 기반 전체 lifecycle 검증 (AC-CTRL-E2E-1)
```

### 1.2 시퀀싱 정당화 (vs plan.md §2 ordering)

plan.md §2는 `S1 State → S2 Store → S3 Dispatch → S4 gRPC → S5 REST` 순으로 명시했다. 본 strategy는 다음 4가지 근거로 **시퀀스를 재배치**한다 — 단, plan.md의 큰 의존 골격은 그대로 보존된다(state machine이 store보다 먼저, gRPC가 dispatch와 무관하게 실행 가능, REST가 gRPC 뒤).

| 변경 | 근거 |
|------|------|
| **Sprint 0 Foundation 분리** (plan.md는 Sprint 0가 SPEC-AX-001 산출물이라고 가정) | SPEC-AX-001 Sprint 0 stub가 이미 존재하나 grpc/pgx/redis/testcontainers-go 의존성이 go.mod에 없음(현재 zap만 존재). foundation을 명시적 Sprint로 분리해야 후속 sprint 의존성 사슬이 명확. |
| **REQ-CTRL-UBI를 Sprint 1로 격상** | spec-compact.md L27-28이 UBI를 modal REQ보다 위에 명시. plan.md는 UBI를 modal REQ 안에 흩어 놓았으나 AC-CTRL-UBI-001~002-C 4개 AC는 횡단 invariant이므로 단일 sprint로 모아 atomicity 패턴을 먼저 확립. |
| **PostgreSQL Store가 gRPC보다 먼저** | plan.md §2 ordering과 일치 (S2→S4). gRPC handler가 store interface에 의존하므로 store interface를 먼저 GREEN 처리해야 gRPC 단위 테스트에서 mock 표면적이 최소화됨. |
| **Celery Dispatch를 마지막에서 두 번째로** | plan.md §2는 S3로 배치했으나 — Dispatch는 (a) Python pipelines 측 Celery serializer 설정 의존, (b) golden file 외부 입력 의존, (c) state machine + store만 완료되면 dispatch는 격리 단위로 추가 가능. 따라서 **gRPC/REST 후 안정된 server에서 dispatch 추가**가 risk minimization 측면에서 우수. **단, AC-CTRL-001-1(happy path)은 RPC 응답 후 dispatch까지 검증하므로** Sprint 2 State Machine과 Sprint 4 gRPC happy path는 dispatch mock으로 검증, 실제 Redis dispatch는 Sprint 6 GREEN 진입 시점에 모든 modal REQ 통합 검증. |

**Re-evaluation**: 만약 Celery dispatch가 가장 큰 risk(R-CTRL-001)이고 외부 의존(Python kombu golden file)이라면 **빨리 검증**하는 것도 합리적이다. 그러나 dispatch가 동작하려면 store에 workflow row가 있어야 하고 state machine이 PENDING→RUNNING 전이를 수행해야 한다 → store + state machine 선행이 dispatch보다 dependency 측면에서 우선. golden file은 sprint 6 시작 전 1회 Python 측 생성 후 커밋이라는 별도 작업으로 분리 가능(SPEC-AX-001 maintenance handoff). 따라서 **현재 ordering 유지**.

### 1.3 병렬화 기회

- Sprint 4(gRPC)와 Sprint 5(REST gateway)는 grpc-gateway가 grpc handler 위에 얇은 변환층이므로 sub-agent mode + 단일 manager-tdd 환경에서는 **순차** 권장. team mode 활성 시 implementer 2명(isolation: worktree)으로 분할 가능하나 sprint contract artifact 충돌 위험 + tutuhal harness evaluator scoring 일관성 문제로 본 SPEC에서는 순차 권장.
- Sprint 2(State Machine) 단위 테스트 작성과 Sprint 3(Store) testcontainers fixture 작성은 RED 단계에서 병렬 가능하나, GREEN 단계에서는 store가 state machine의 Status enum에 의존하므로 순차.

---

## 2. Foundation Setup (Pre-Sprint 0 + Sprint 0)

### 2.1 Pre-Sprint 0 — Environment Verification

- [x] **Go 1.22.10 확인** (`/usr/local/go/bin/go version`) — 환경 검증 완료
- [x] **golangci-lint 확인** (`/home/sklee/go/bin/golangci-lint`) — 환경 검증 완료
- [x] **`go mod tidy` 정상 동작** — 환경 검증 완료
- [ ] **PostgreSQL+Redis 미가동** → testcontainers-go fallback으로 진행 (single-image session-scoped fixture)
- [ ] **Docker 가용성 확인** — testcontainers-go 동작 전제. `docker info` 실행 후 daemon 가용성 확인. 미가용 시 expert-devops 호출하여 환경 정비.

### 2.2 Sprint 0 — Foundation 산출물

#### 2.2.1 go.mod 의존성 추가

현재 `/home/sklee/moai/iroum-ax/go.mod` 상태:
```
module github.com/ircp/iroum-ax
go 1.22
require go.uber.org/zap v1.27.0
require go.uber.org/multierr v1.10.0 // indirect
```

Sprint 0에서 추가 (plan.md §3 의존성 핀 + research.md §1.2~§1.4 검증):

```
require (
    google.golang.org/grpc v1.65.0
    google.golang.org/protobuf v1.34.0
    github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0
    github.com/jackc/pgx/v5 v5.6.0
    github.com/redis/go-redis/v9 v9.6.0
    github.com/google/uuid v1.6.0
    golang.org/x/sync v0.7.0  // errgroup
    google.golang.org/genproto/googleapis/api v0.0.0 // grpc-gateway http annotations
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

> **Go 버전 정책**: `tech.md` §1.1 = Go 1.22+, `.claude/rules/moai/languages/go.md` = Go 1.23+ 권장. plan.md §3 주의사항대로 **본 SPEC은 Go 1.22 baseline** 유지(1.23 전용 기능 사용 금지: range over integers, PGO 2.0). CI matrix는 1.22 + 1.23 양쪽 검증 권장. 본 시점 환경(`go1.22.10`)이 baseline에 정확히 일치.

추가 후 검증: `go mod tidy` → `go.sum` 핀 → `go vet ./...` 0 error.

#### 2.2.2 Proto Codegen Environment

도구: **buf v1.39+** (research.md §1.2 명시, plan.md §1 S4에서 buf.gen.yaml 갱신 명시).

진입점 파일:
- `schemas/proto/buf.yaml` (현재 존재 — Sprint 0 stub 검증)
- `schemas/proto/buf.gen.yaml` (**NEW** — Sprint 0 신규 작성)
- `schemas/proto/workflow.proto` (현재 messages만 존재 — Sprint 4에서 WorkflowService 추가)

buf.gen.yaml 내용 (Sprint 0 작성):
```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go:v1.34.0
    out: apps/control-plane/internal/proto
    opt: paths=source_relative
  - remote: buf.build/grpc/go:v1.5.1
    out: apps/control-plane/internal/proto
    opt: paths=source_relative,require_unimplemented_servers=false
  - remote: buf.build/grpc-ecosystem/gateway:v2.20.0
    out: apps/control-plane/internal/proto
    opt: paths=source_relative
  - remote: buf.build/grpc-ecosystem/openapiv2:v2.20.0
    out: schemas/openapi/generated
```

생성 산출물 디렉토리:
- `apps/control-plane/internal/proto/ax/v1/workflow.pb.go` (Sprint 4 codegen)
- `apps/control-plane/internal/proto/ax/v1/workflow_grpc.pb.go` (Sprint 4)
- `apps/control-plane/internal/proto/ax/v1/workflow.pb.gw.go` (Sprint 5)

Sprint 0에서는 buf 환경만 구성, 실제 codegen은 Sprint 4 시점에 WorkflowService 추가 후 실행.

#### 2.2.3 디렉토리 신규 생성

```
apps/control-plane/
├── config/                      # NEW — 환경 변수 파서
│   └── config.go                # Sprint 0
├── cmd/server/                  # EXISTING (Sprint 0 stub: server.go)
│   ├── server.go                # EXISTING (Sprint 4에서 errgroup 기반 재작성)
│   ├── grpc_handlers.go         # NEW — Sprint 4
│   ├── health.go                # NEW — Sprint 5
│   ├── middleware.go            # NEW — Sprint 4
│   ├── grpc_handlers_test.go    # NEW — Sprint 4
│   └── rest_handlers_test.go    # NEW — Sprint 5
├── internal/
│   ├── audit/                   # NEW — Sprint 1
│   │   ├── event.go             # 8종 audit 액션 enum
│   │   └── event_test.go
│   ├── proto/ax/v1/             # NEW (buf 생성, Sprint 4)
│   ├── scheduler/               # EXISTING (Sprint 0 stub: dispatcher.go)
│   │   ├── celery_envelope.go   # NEW — Sprint 6
│   │   ├── celery_envelope_test.go
│   │   ├── dispatcher.go        # EXISTING (Sprint 6 재작성)
│   │   ├── dispatcher_test.go
│   │   └── testdata/
│   │       └── celery_envelope_v2.json  # NEW — Python kombu 측 생성 후 커밋
│   ├── store/                   # EXISTING (Sprint 0 stub: postgres.go)
│   │   ├── postgres.go          # EXISTING (Sprint 3 재작성)
│   │   ├── postgres_test.go     # NEW — testcontainers
│   │   ├── audit.go             # NEW — Sprint 3
│   │   └── audit_test.go
│   └── workflow/                # EXISTING (Sprint 0 stub: state_machine.go)
│       ├── state_machine.go     # EXISTING (Sprint 2 재작성)
│       ├── state_machine_test.go
│       ├── handlers.go          # NEW — Sprint 2
│       ├── handlers_test.go
│       └── callback.go          # NEW — Sprint 2
```

#### 2.2.4 pkg/models 확장

현재 `pkg/models/` 존재. Sprint 0에서 Go 측 단일 진실 source 추가:

- `pkg/models/workflow.go` (NEW):
  ```go
  // Status 워크플로우 실행 상태
  type Status string
  
  const (
      StatusPending   Status = "PENDING"
      StatusRunning   Status = "RUNNING"
      StatusCompleted Status = "COMPLETED"
      StatusFailed    Status = "FAILED"
  )
  
  // Workflow 도메인 엔티티 (workflows 테이블 1:1 매핑)
  type Workflow struct {
      ID         string    // UUID v7
      UserID     string    // 기본값 "cli-anonymous"
      Status     Status
      DocumentID string
      ReportID   *string   // nullable
      ResultJSON []byte    // JSONB
      CreatedAt  time.Time
      UpdatedAt  time.Time
  }
  ```

> 현재 `apps/control-plane/internal/workflow/state_machine.go`에 동일 타입이 stub으로 존재. Sprint 2 RED 단계 시작 시 **이전을 pkg/models로 이전**하고 internal/workflow는 state machine 로직만 보유 (DRY 원칙, SPEC-AX-001 strategy.md §1.3 `pkg/models/` 패턴과 일치).

#### 2.2.5 config/config.go (env 파서)

`apps/control-plane/config/config.go` (NEW):
```go
type Config struct {
    GRPCPort            string  // default ":50051"
    RESTPort            string  // default ":8080"
    PostgresDSN         string
    RedisURL            string
    CeleryQueue         string  // default "celery"
    PrometheusEnabled   bool    // REQ-CTRL-002-O1
    GRPCReflectionEnabled bool  // default false (Exclusion §1)
    DefaultUserID       string  // default "cli-anonymous"
    LogLevel            string  // default "info"
}

func LoadFromEnv() (*Config, error) {
    // env 파싱 + 기본값 적용 + 필수 필드 검증
}
```

### 2.3 Sprint 0 Pass Criteria

- `go mod tidy` 0 warning
- `go vet ./apps/control-plane/...` 0 error
- `golangci-lint run ./apps/control-plane/...` 0 error (default + govet, errcheck, staticcheck, gosec)
- 모든 신규 디렉토리 생성 + `.gitkeep` 또는 placeholder 파일 존재
- `pkg/models/workflow.go` 컴파일 성공
- buf.gen.yaml lint 통과 (`buf lint schemas/proto`)
- LSP baseline 0/0/0 캡처 (`.moai/specs/SPEC-AX-CTRL-001/lsp-baseline.json` 저장)

---

## 3. Test Infrastructure

### 3.1 단위 테스트 (Go default)

- 라이브러리: `github.com/stretchr/testify/assert` + `require`
- 패턴: 테이블 기반 테스트 (`.claude/rules/moai/languages/go.md` "Table-driven tests are preferred" 준수)
- 명령: `go test -cover -race ./apps/control-plane/...`
- 커버리지: ≥ 85% (per package, 모든 sprint pass 조건)
- goroutine leak detection: 모든 테스트 file의 `TestMain`에서 `goleak.VerifyTestMain(m)` 호출 (research.md R-CTRL-004 mitigation)

테스트 명명 규약 (Sprint 0 합의):
- 단위: `func TestStateMachine_Transition(t *testing.T)` (Subject_Method 패턴)
- 통합: `//go:build integration` 빌드 태그 + `func TestPostgresStore_Integration(t *testing.T)`
- 벤치마크: `func BenchmarkDispatchLatency(b *testing.B)` (AC-CTRL-005-4)

### 3.2 통합 테스트 (testcontainers-go)

빌드 태그: `//go:build integration` — CI에서는 별도 job으로 실행 (`go test -tags=integration -race -cover ./...`).

testcontainers-go fixture 패턴 (Sprint 3 + Sprint 6 + Sprint 7 공유):
- **shared per package**: `TestMain`에서 `testcontainers.Container` 1개 부팅, 모든 테스트가 동일 컨테이너 공유 (성능 5-10x 개선)
- **per-test schema isolation**: 각 테스트가 `CREATE SCHEMA test_xxx; SET search_path TO test_xxx;` 후 fixture 데이터 INSERT, 테스트 종료 시 `DROP SCHEMA test_xxx CASCADE` (transaction rollback 대안보다 schema 격리가 더 명확)

이미지:
- PostgreSQL: `pgvector/pgvector:pg16` (.moai/db/schema/initial.sql과 동일 이미지 — SPEC-AX-001 strategy.md §1 Step 6)
- Redis는 miniredis(in-memory Go 구현)로 대체 — 통합 테스트도 miniredis 우선, 실제 Redis 컨테이너는 Sprint 7 E2E에서만 사용

### 3.3 벤치마크 (성능 AC)

- AC-CTRL-002-2 gRPC p99 < 50ms: bufconn + `go test -bench=BenchmarkGRPCCreateWorkflow`
- AC-CTRL-003-1 REST p99 < 100ms: httptest + `go test -bench=BenchmarkRESTCreateWorkflow`
- AC-CTRL-005-4 Dispatch p99 < 100ms: miniredis + `go test -bench=BenchmarkDispatchLatency -benchtime=10000x -count=5`
- p99/p50 추출: custom benchstat 또는 hand-rolled quantile (acceptance.md §7 명시)
- CI 환경 noise 완화: thresholds 1.5× 완화 (acceptance.md §7 표 참조)

### 3.4 Mocking Strategy

**결정: 인터페이스 + 핸드라이튼 fake** (mockgen 미사용)

근거:
- testify mock은 매크로 복잡성 + reflection overhead, plan.md §9 specialist routing은 minimal tooling 선호
- mockgen은 generation step 추가 + Makefile target 부담 + git diff noise
- 핸드라이튼 fake는 ~30-50 LOC per interface로 충분(`store.Store`, `scheduler.Dispatcher`, `audit.Writer` 정도)
- 향후 mockgen 도입은 후속 SPEC에서 별도 평가

인터페이스 위치:
- `internal/workflow/handlers.go`에서 import: `store.Store`, `scheduler.Dispatcher` (interface 정의는 각 패키지 owner)
- test 파일 옆에 `_fake.go` 또는 `fake_test.go`로 fake 구현 위치

### 3.5 Coverage 측정

명령: `go test -cover -coverprofile=coverage.out -tags=integration ./apps/control-plane/...`

각 sub-sprint REFACTOR 종료 시:
- `go tool cover -func=coverage.out` 출력에서 `total: (statements)` ≥ 85% 검증
- per-package coverage ≥ 80% (per-file은 강제하지 않음 — pgx CRUD wrapper는 통합 테스트로만 커버 가능)

---

## 4. Per-Sprint Sprint Contracts (Thorough Harness)

각 Sprint Contract는 `.moai/sprints/SPEC-AX-CTRL-001/sprint-{n}.md`에 evaluator-active가 동적 생성한다. 본 §4는 scaffold(priority dimension, acceptance checklist, test scenarios, pass condition)만 제공.

### 4.1 Sprint 0: Foundation

- **Priority dimension**: **Completeness** (모든 후속 sprint가 의존)
- **Acceptance checklist**:
  - [F0-1] go.mod에 §2.2.1 의존성 모두 추가, `go mod tidy` 0 warning
  - [F0-2] `buf lint schemas/proto` 0 error
  - [F0-3] `apps/control-plane/config/config.go` 환경 변수 파싱 + 기본값 검증 단위 테스트 통과
  - [F0-4] `pkg/models/workflow.go` 컴파일 + 기본 직렬화/역직렬화 테스트 통과
  - [F0-5] `apps/control-plane/internal/audit/event.go` 8종 액션 enum 정의 + 검증 테스트
  - [F0-6] `golangci-lint run ./apps/control-plane/...` 0 error
- **Test scenarios** (table-driven):
  - `TestConfig_LoadFromEnv` — 정상/누락 필드/잘못된 포트 형식 3 케이스
  - `TestWorkflow_StatusValidation` — 4 유효 + 1 invalid
  - `TestAuditAction_String` — 8종 액션 enum string 검증
- **Pass condition**:
  - 모든 acceptance checklist 통과
  - coverage ≥ 85% (`pkg/models`, `config`, `internal/audit`)
  - LSP 0/0/0
- **Evaluator 4-dim**: Completeness 40% / Functionality 30% / Security 20% / Originality 10%

### 4.2 Sprint 1: REQ-CTRL-UBI-001/002 횡단 불변

- **Priority dimension**: **Security** (트랜잭션 원자성 + audit 일관성 = constitutional invariant)
- **Acceptance checklist**:
  - [AC-CTRL-UBI-001] 트랜잭션 원자성: workflow + audit 동시 rollback (Scenario A/B/C)
  - [AC-CTRL-UBI-002-A] WORKFLOW_CREATED audit row 완전성
  - [AC-CTRL-UBI-002-B] WORKFLOW_TRANSITIONED_TO_RUNNING audit row 완전성
  - [AC-CTRL-UBI-002-C] user_id='cli-anonymous' 기본값 (REST + gRPC 양쪽)
- **Test scenarios**:
  - `TestAuditEvent_AllActionsSerialize` — 8종 액션 JSONB 직렬화
  - `TestAuditWriter_TxAware` — pgx.Tx 인자 받는 InsertAuditLog 인터페이스 검증 (Sprint 3 본격 구현 전 인터페이스 정의)
  - `TestUserID_DefaultClianon` — config.DefaultUserID 적용 검증
- **Pass condition**:
  - audit_logs 인터페이스 정의 완료 (Sprint 3 Postgres Store와 호환)
  - 8종 액션 enum + JSONB details schema 검증
  - coverage ≥ 85%
- **Evaluator 4-dim**: Security 45% / Functionality 30% / Completeness 15% / Originality 10%

> **주의**: Sprint 1은 audit 인터페이스 + 인메모리 fake 검증에 한정. 실제 PostgreSQL 트랜잭션 atomicity는 Sprint 3에서 testcontainers로 검증. 이는 RED-GREEN-REFACTOR 의존 사슬을 단순화한다.

### 4.3 Sprint 2: REQ-CTRL-001 Workflow State Machine

- **Priority dimension**: **Functionality** (상태 불변 = 후속 모든 REQ 의미의 전제)
- **Acceptance checklist**:
  - [AC-CTRL-001-1] Happy path workflow 생성 (50ms p99)
  - [AC-CTRL-001-2] Invalid transition PENDING→COMPLETED 거부
  - [AC-CTRL-001-3] Terminal state immutability (409)
  - [AC-CTRL-001-4] Concurrent transition serialization (SELECT FOR UPDATE invariant 정의 — 실제 구현은 Sprint 3 Store 통합)
  - [AC-CTRL-001-5] gRPC client cancellation mid-tx (Sprint 4 통합 검증)
- **Test scenarios**:
  - `TestStateMachine_CanTransition` — 4×4 transition 매트릭스 16 케이스 (3 valid + 13 invalid)
  - `TestStateMachine_Transition_Happy` — fake Store로 PENDING→RUNNING
  - `TestStateMachine_Transition_RejectInvalid` — PENDING→COMPLETED skip RUNNING 거부 + ErrInvalidTransition
  - `TestStateMachine_TerminalImmutability` — COMPLETED 후 RUNNING 시도 거부
- **Pass condition**:
  - 16/16 transition 매트릭스 케이스 통과
  - ErrInvalidTransition + ErrTerminalState 명시적 에러 타입
  - coverage ≥ 85%
- **Evaluator 4-dim**: Functionality 50% / Security 25% / Completeness 15% / Originality 10%

### 4.4 Sprint 3: REQ-CTRL-004 PostgreSQL Store

- **Priority dimension**: **Functionality + Security** (SQL injection + SELECT FOR UPDATE 정확성)
- **Acceptance checklist**:
  - [AC-CTRL-004-1] Pool fail-fast (잘못된 DSN, 5s 이내)
  - [AC-CTRL-004-2] SELECT FOR UPDATE 2-goroutine 직렬화
  - [AC-CTRL-004-3] Pool exhaustion → RESOURCE_EXHAUSTED
  - [AC-CTRL-004-4] Mid-tx PostgreSQL failure → tx.Rollback + SQLSTATE 로그
  - [AC-CTRL-UBI-001] Scenario A/B/C transaction atomicity (Sprint 1에서 인터페이스 정의 → Sprint 3 실제 검증)
- **Test scenarios** (`//go:build integration`):
  - `TestPostgresStore_CreateWorkflow_Integration` — INSERT + RETURNING id
  - `TestPostgresStore_LockForUpdate_Concurrent` — 2 goroutine 동시 락 시도 직렬화 (1 goroutine 차단 확인)
  - `TestPostgresStore_PoolExhaustion` — max_open=2, 3개 long-running tx
  - `TestPostgresStore_TxRollback_OnAuditFail` — audit INSERT 실패 시 workflow rollback (UBI-001 Scenario A)
  - `TestPostgresStore_TxRollback_OnMidTxFailure` — pg_terminate_backend로 connection kill 후 rollback 검증 (AC-CTRL-004-4)
  - 별도 unit: `TestPostgresStore_InvalidDSN_FailFast` — testcontainers 미사용, 5s 이내 에러
- **Pass condition**:
  - testcontainers postgres:16-pgvector 정상 부팅 (또는 plain postgres:16 — Sprint 3 시작 시 결정)
  - 동시성 테스트 100% 직렬화 (deadlock 0건)
  - SQLSTATE 코드 zap 구조화 로그에 정확히 출력
  - coverage ≥ 85%
- **Evaluator 4-dim**: Functionality 40% / Security 30% / Completeness 20% / Originality 10%
- **마이그레이션 산출물**: `.moai/db/schema/migrations/0002_workflow_indexes.sql` (HNSW와 동일 패턴, B-tree 인덱스 `workflows(status, created_at DESC)`)

### 4.5 Sprint 4: REQ-CTRL-002 gRPC Server

- **Priority dimension**: **Functionality + Security** (gRPC interceptor 보안 + reflection 비활성)
- **Acceptance checklist**:
  - [AC-CTRL-002-1] gRPC server startup <2s
  - [AC-CTRL-002-2] CreateWorkflow 10 concurrent p99 < 50ms
  - [AC-CTRL-002-3] Cancellation propagation (no goroutine leak)
  - [AC-CTRL-002-4] Prometheus optional /metrics (Sprint 5 REST gateway와 함께 검증)
  - [AC-CTRL-001-5] gRPC client cancellation mid-tx
- **Test scenarios**:
  - `TestGRPCServer_StartupReady` — bufconn dial + GetWorkflow nonexistent → NOT_FOUND
  - `TestGRPCHandlers_CreateWorkflow_Happy` — bufconn + testcontainers postgres + miniredis (dispatcher mock)
  - `BenchmarkGRPCCreateWorkflow` — 10 concurrent × 100 RPC, p99 < 50ms
  - `TestGRPCHandlers_CtxCancel_Rollback` — context.WithTimeout + tx.Rollback 검증
  - `TestGRPCMiddleware_RequestID_UUIDv7` — interceptor가 uuid v7 발행 + structured log 필드
- **Pass condition**:
  - workflow.proto에 WorkflowService 추가 + buf generate 성공
  - gRPC reflection 비활성 검증 (`grpc.reflection.v1alpha.ServerReflection` 미등록)
  - bufconn 단위 테스트 통과
  - goleak.VerifyNone 모든 테스트 통과
  - coverage ≥ 85%
- **Evaluator 4-dim**: Functionality 45% / Security 25% / Completeness 20% / Originality 10%

### 4.6 Sprint 5: REQ-CTRL-003 REST API

- **Priority dimension**: **Completeness** (REST는 gRPC 위의 변환층 — 일관성)
- **Acceptance checklist**:
  - [AC-CTRL-003-1] POST happy path (201 + Location header, p99 < 100ms)
  - [AC-CTRL-003-2] Bad request (400 + structured error body)
  - [AC-CTRL-003-3] /healthz 200 + variant 503 (DB down)
  - [AC-CTRL-003-4] REST gateway startup <2s (polling)
  - [AC-CTRL-002-4] Prometheus optional /metrics (Sprint 4 metric exporter + Sprint 5 REST mux 통합)
- **Test scenarios**:
  - `TestRESTGateway_PostWorkflow_Happy` — httptest + 201 + Location 헤더
  - `TestRESTGateway_BadRequest_400` — malformed UUID
  - `TestRESTGateway_Healthz_DBDown` — pgxpool.Close → 503
  - `TestRESTGateway_Healthz_AllOk` — 200 + JSON body
  - `BenchmarkRESTCreateWorkflow` — 100 RPC, p99 < 100ms
  - `TestRESTGateway_Startup_LessThan2s` — server.Run + /healthz 폴링
- **Pass condition**:
  - grpc-gateway 어노테이션이 workflow.proto에 정확 정의 (`google.api.http`)
  - `cmd/server/health.go` DB+Redis ping 통합
  - openapi.yaml workflow 엔드포인트 3개 추가
  - coverage ≥ 85%
- **Evaluator 4-dim**: Completeness 40% / Functionality 35% / Security 15% / Originality 10%

### 4.7 Sprint 6: REQ-CTRL-005 Celery Dispatch

- **Priority dimension**: **Originality + Functionality** (Celery protocol v2 envelope 정확성)
- **Acceptance checklist**:
  - [AC-CTRL-005-1] Envelope golden file byte match
  - [AC-CTRL-005-2] Redis unavailable retry → FAILED
  - [AC-CTRL-005-3] Serialization failure → no RPUSH + FAILED
  - [AC-CTRL-005-4] Dispatch latency p99 < 100ms
- **Test scenarios**:
  - `TestCeleryEnvelope_GoldenFileByteMatch` — testdata/celery_envelope_v2.json와 byte-for-byte 비교
  - `TestDispatcher_RedisUnavailable_3Retries` — miniredis.Close() 후 3회 backoff (50/200/800ms)
  - `TestDispatcher_SerializationFailure_NoRPUSH` — make(chan int) metadata → JSON marshal 실패
  - `BenchmarkDispatchLatency` — 10 concurrent × 1000 iter, p99 < 100ms (miniredis 사용)
- **Pass condition**:
  - Python 측 reference envelope 생성 스크립트(`scripts/generate_celery_golden.py`) 실행 + testdata 커밋 (Sprint 6 시작 전 1회 수동)
  - golden file byte match 100%
  - 3-step exponential backoff retry 정상 동작
  - coverage ≥ 85%
- **Evaluator 4-dim**: Originality 35% / Functionality 35% / Completeness 20% / Security 10%
- **Sprint 6 시작 전 handoff**: SPEC-AX-001 `pipelines/config/settings.py`에 `CELERY_TASK_SERIALIZER='json', CELERY_ACCEPT_CONTENT=['json']` 설정 확인. 누락 시 1줄 패치 PR.

### 4.8 Sprint 7: E2E Integration

- **Priority dimension**: **Completeness** (모든 modal REQ가 한 슬라이스로 검증)
- **Acceptance checklist**:
  - [AC-CTRL-E2E-1] docker-compose 전체 lifecycle: POST → 201 → ~5s 내 COMPLETED → audit_logs 3 events
- **Test scenarios**:
  - `tests/integration/test_control_plane_to_pipelines.py` — pytest + docker-compose up + httpx 클라이언트
  - Variant: SPEC-AX-001 Sprint 8 fixture(`tests/fixtures/synthetic_safety_5page.hwp`)와 동일 fixture 사용
- **Pass condition**:
  - docker-compose up 후 Go control-plane이 Python Celery worker 실제 호출
  - workflow COMPLETED 도달
  - audit_logs 3 events 정확히 기록 (WORKFLOW_CREATED, WORKFLOW_TRANSITIONED_TO_RUNNING, WORKFLOW_COMPLETED)
  - 모든 modal AC 통합 검증 (re-run subset)
- **Evaluator 4-dim**: Completeness 50% / Functionality 30% / Security 15% / Originality 5%

---

## 5. Risk-Driven Decisions

본 §5는 plan.md §5 R-CTRL-001~005를 상속하면서 Opus 4.7 trade-off matrix로 Phase 1 단계에서 결정 가능한 5가지 핵심 결정 사항을 명시한다.

### 5.1 Decision 1: Celery Envelope Golden File Testing Strategy (R-CTRL-001 Mitigation)

**Question**: Celery protocol v2 envelope 정확성을 어떻게 보장할 것인가?

**Options**:

| Option | Risk | Cost | Maint | 가중점수 |
|--------|------|------|-------|---------|
| (A) Python kombu로 golden file 1회 생성 + 커밋, Go가 byte 매치 | 3/10 | 7/10 | 8/10 | **6.30** |
| (B) Property-based testing (Go가 N개 random input 생성, Python kombu와 cross-validate) | 5/10 | 4/10 | 5/10 | 4.65 |
| (C) Python kombu를 cgo 또는 subprocess로 invoke (test마다) | 7/10 | 3/10 | 4/10 | 4.55 |

**결정**: **(A)** — research.md §3.3과 일치. 가장 보수적이고 유지보수 비용 최소.

**구현 절차** (Sprint 6 시작 전 1회):
1. Python venv 활성 (SPEC-AX-001 환경)
2. `scripts/generate_celery_golden.py` 작성 (research.md §3.3 패턴)
3. 입력 고정: workflow_id=`fixed-test-uuid-005-001`, document_id=`d-fixed-005-001`
4. `kombu.Message` 시리얼라이즈 결과를 `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json`에 저장
5. git add + commit (Sprint 6 RED phase 진입 전)

**Residual Risk**: Kombu/Celery 버전이 업그레이드되면 golden file 무효화 → mitigation: SPEC-AX-001 `pyproject.toml`의 `celery[redis] ^5.3.0` 핀 유지 + 정기 dependency audit.

### 5.2 Decision 2: PostgreSQL Test Isolation Strategy

**Question**: testcontainers-go 기반 통합 테스트에서 데이터 격리를 어떻게 처리할 것인가?

**Options**:

| Option | Speed | Isolation | Complexity |
|--------|-------|-----------|------------|
| (A) per-test schema 격리 (CREATE SCHEMA + DROP SCHEMA) | 중 | 높음 | 중 |
| (B) transaction rollback (각 테스트가 BEGIN, 종료 시 ROLLBACK) | 빠름 | 중간 (DDL 미적용) | 낮음 |
| (C) testcontainer per suite (각 _test.go 마다 새 컨테이너) | 매우 느림 | 매우 높음 | 낮음 |

**결정**: **(A) per-test schema 격리**.

근거:
- (B)는 SELECT FOR UPDATE를 검증하는 동시성 테스트에서 트랜잭션 mock이 되어버려 부적절
- (C)는 5-10x 느려지고 CI 시간 폭증
- (A)는 schema-level 격리 + DDL 변경(인덱스 추가 등) 검증 가능
- 패턴: `setupTestSchema(t, pool) → DROP SCHEMA on t.Cleanup`

**구체 구현** (Sprint 3 RED phase):
```go
// testhelpers/postgres.go
func SetupTestSchema(t *testing.T, pool *pgxpool.Pool) {
    schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
    _, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s; SET search_path TO %s", schema, schema))
    require.NoError(t, err)
    // Run migrations on this schema
    // ...
    t.Cleanup(func() {
        pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
    })
}
```

### 5.3 Decision 3: audit_logs Schema Compatibility with SPEC-AX-001

**Question**: Go control-plane이 작성하는 audit_logs row가 SPEC-AX-001 Python pipelines와 schema 정합한가?

**Verification 절차** (Sprint 1 시작 시):
1. SPEC-AX-001 strategy.md §1 Step 2 audit_logs 스키마 인용:
   ```sql
   CREATE TABLE audit_logs (
     id UUID PRIMARY KEY,
     user_id VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
     action VARCHAR(64) NOT NULL,
     resource_id UUID NOT NULL,
     resource_type VARCHAR(32),
     timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
     details JSONB
   );
   ```
2. Go `internal/audit/event.go`에서 동일 컬럼 모두 정의 (no extra, no missing)
3. SPEC-AX-001 `pkg/logging/logger.py audit_event(...)` 함수 시그니처와 동일 필드 셋 검증
4. Sprint 7 E2E 테스트에서 Python + Go 양측 INSERT 결과를 DESCRIBE로 비교

**결정**: SPEC-AX-001 schema를 single source of truth로 사용. Go 측은 컬럼 추가 시도 0건 (확장은 후속 SPEC 별도 SQL migration으로).

**Action item** (Sprint 1 시작 시):
- `.moai/db/schema/initial.sql`을 읽어 audit_logs 컬럼 셋 확인
- `internal/audit/event.go` AuditEvent struct 8 필드 = id, user_id, action, resource_id, resource_type, timestamp, details (+ Workflow 1:1 mapping fields가 details JSONB에)
- `internal/store/audit.go InsertAuditLog(ctx, tx, event)` 시그니처: `tx pgx.Tx`를 첫 인자로 받아 caller가 atomicity 제어

### 5.4 Decision 4: gRPC Reflection Default Policy

**Question**: dev/prod 환경에서 gRPC reflection 정책?

**결정**: **비활성 default + 환경변수 opt-in**

- `cmd/server/server.go` 부팅 시 `config.GRPCReflectionEnabled` 체크
- 기본값 false (망분리 보안 우선, spec.md Exclusion §1 정합)
- dev 편의를 위해 `GRPC_REFLECTION_ENABLED=true` 환경변수 설정 시에만 `reflection.Register(grpcServer)` 호출
- 단위 테스트: `TestServerBootstrap_ReflectionDisabledByDefault` — grpcurl로 reflection request 실패 검증

근거:
- spec.md §5 Exclusion §1이 "gRPC reflection / gRPC-Web tooling" 제외 명시
- prod 환경 기본값과 dev 편의 사이 균형
- security 회귀 방지 (dev에서 활성 후 prod에 누적 적용 위험)

### 5.5 Decision 5: State Machine Concurrency — SELECT FOR UPDATE Isolation Level

**Question**: SELECT FOR UPDATE를 어떤 isolation level에서 실행할 것인가?

**Options**:

| Option | Locking | 적합성 |
|--------|---------|--------|
| (A) READ COMMITTED + SELECT FOR UPDATE | 행 단위 락 | 표준 — PostgreSQL 기본 |
| (B) REPEATABLE READ + SELECT FOR UPDATE | snapshot + 행 락 | 과도 |
| (C) SERIALIZABLE | 전체 트랜잭션 직렬화 | 성능 저하 |

**결정**: **(A) READ COMMITTED + SELECT FOR UPDATE**.

근거:
- PostgreSQL 기본 isolation level이 READ COMMITTED
- SELECT FOR UPDATE만으로 단일 행에 대한 동시 UPDATE를 직렬화하기 충분 (AC-CTRL-001-4, AC-CTRL-004-2 검증 가능)
- workflow_id 단일 행 동시 callback은 race condition의 본질이므로 행 락이면 충분
- REPEATABLE READ나 SERIALIZABLE은 dispatch latency 부담 가중 (AC-CTRL-005-4 p99 < 100ms 위협)

**구현**:
- `pool.Begin(ctx)` 호출 시 default isolation level 사용 (explicit isolation level 설정 없음)
- `tx.QueryRow(ctx, "SELECT ... FROM workflows WHERE id=$1 FOR UPDATE", id)` 호출
- 단위 테스트(`TestStateMachine_Concurrent_Serialization`): 2 goroutine 동시 락 시도 → 1개 차단, 1개 진행

---

## 6. LSP / Lint Baseline

### 6.1 Baseline Capture (Sprint 0 종료 시)

Sprint 0 종료 시점에 다음 명령으로 baseline 캡처:
- `go vet ./apps/control-plane/...` — 0 error
- `golangci-lint run ./apps/control-plane/...` — 0 error
- `buf lint schemas/proto` — 0 error
- `goimports -l apps/control-plane/` — 0 diff

저장 위치: `.moai/specs/SPEC-AX-CTRL-001/lsp-baseline.json`:
```json
{
  "captured_at": "2026-05-14T...",
  "errors": 0,
  "type_errors": 0,
  "lint_errors": 0,
  "warnings": 0,
  "go_version": "1.22.10",
  "golangci_version": "<output of golangci-lint --version>"
}
```

### 6.2 Per-Sprint Quality Gate

각 sub-sprint REFACTOR 종료 시 manager-quality가 다음 검증:
- `go vet ./apps/control-plane/...` 0 error
- `golangci-lint run ./apps/control-plane/...` 0 error (config: §6.3 참조)
- `go test -race -cover ./apps/control-plane/...` 모두 PASS + coverage ≥ 85%
- `goleak.VerifyTestMain` 통과 (모든 테스트)

실패 시 즉시 차단 + 해당 sprint REFACTOR 단계로 회귀. `quality.yaml run.allow_regression=false` 강제.

### 6.3 golangci-lint Configuration

**결정: 커스텀 `.golangci.yml`을 Sprint 0에서 생성** (sensible default).

내용 (`apps/control-plane/.golangci.yml` 또는 repo root):
```yaml
run:
  go: "1.22"
  timeout: 5m

linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - gosec
    - revive  # readability
    - gocyclo  # complexity (threshold 15)
    - bodyclose  # HTTP response body close
    - contextcheck  # ctx propagation
    - goimports

linters-settings:
  gocyclo:
    min-complexity: 15
  gosec:
    excludes:
      - G104  # 명시적 errcheck로 대체
```

Sprint 0 종료 시 모든 stub이 위 lint를 통과해야 baseline 유지.

---

## 7. Specialist Routing

| Sprint | Owner Agent | 보조 Agent | 비고 |
|--------|------------|-----------|------|
| Pre-Sprint 0 (env verify) | manager-strategy (현 agent) | expert-devops | Docker daemon 가용성 확인, 미가용 시 정비 |
| Sprint 0 Foundation | manager-tdd | expert-backend (Go + buf), expert-devops (testcontainers 환경) | go.mod hydration + buf.gen.yaml 작성 |
| Sprint 1 REQ-UBI | manager-tdd | expert-backend (Go interface + fake), expert-security (audit invariant) | interface 정의 + 인메모리 검증 |
| Sprint 2 State Machine | manager-tdd | expert-backend (Go state machine) | 16 case 매트릭스 테스트 |
| Sprint 3 Postgres Store | manager-tdd | expert-backend (Go + pgx/v5), expert-testing (testcontainers integration) | per-test schema isolation |
| Sprint 4 gRPC Server | manager-tdd | expert-backend (Go + grpc-gateway proto), expert-security (gRPC interceptor) | bufconn + goleak |
| Sprint 5 REST Gateway | manager-tdd | expert-backend (Go + grpc-gateway runtime) | OpenAPI 정합성 |
| Sprint 6 Celery Dispatch | manager-tdd | expert-backend (Go + go-redis + Celery protocol), expert-devops (Kombu protocol cross-validate) | golden file 1회 생성 handoff |
| Sprint 7 E2E | manager-tdd | expert-testing (pytest + docker-compose) | SPEC-AX-001 통합 fixture 활용 |
| 매 sub-sprint 종료 후 | manager-quality | (LSP gate + TRUST 5) | quality.yaml 강제 |
| 매 sub-sprint 종료 후 | evaluator-active | (per-sprint scoring, strict profile, 4-dim) | thorough harness 필수 |
| 모든 sub-sprint 완료 후 | manager-git | (conventional commit + branch 정리) | 본 SPEC 종료 시 |

**Worktree 정책** (sub-agent mode 가정):
- expert-backend가 write-heavy로 호출될 때 `isolation: "worktree"` 적용 (cross-file 변경)
- expert-testing read-only 분석 시 isolation 없이 호출
- expert-security read-only audit 시 isolation 없이 호출
- 모든 isolation: worktree 호출 prompt는 **상대 경로** 사용 (worktree-integration.md §HARD Rules)

**커밋 패턴** (orchestrator 핸들링):
- Sprint 0 종료 시: `feat(SPEC-AX-CTRL-001): sprint 0 foundation — go.mod hydration + buf codegen + pkg/models`
- Sprint 1 종료 시: `feat(SPEC-AX-CTRL-001): sprint 1 REQ-CTRL-UBI — audit invariant interface + 8 action enum`
- ... (각 Sprint Conventional Commit pattern, SPEC-AX-CTRL-001 prefix)

---

## 8. Open Questions

> [HARD] manager-strategy(본 agent)는 AskUserQuestion 호출 금지. 아래 4개 질문은 orchestrator가 AskUserQuestion으로 사용자에게 확인한다.
> 각 질문에 대한 **권장 기본값(default)이 존재**하므로 사용자 응답 없이도 Phase 2 RED 진입 가능 (Ready=YES).

### Q1: golangci-lint 설정 정책

**질문**: golangci-lint를 기본 설정만 사용할지, 커스텀 `.golangci.yml`을 Sprint 0에서 생성할지?

- 옵션 A (권장 default): **커스텀 `.golangci.yml`을 Sprint 0에서 생성** (§6.3 참조). govet, errcheck, staticcheck, gosec, revive, gocyclo (15), bodyclose, contextcheck, goimports 활성. 시작부터 strict 기준 적용.
- 옵션 B: 기본 default linters만 사용. 후속 SPEC에서 strict화.

### Q2: Mock 생성 정책

**질문**: 인터페이스 mock을 mockgen으로 자동 생성할지, 핸드라이튼 fake를 사용할지?

- 옵션 A (권장 default): **핸드라이튼 fake** (§3.4 참조). 인터페이스 수가 적고(<5) 각 fake가 ~30-50 LOC이므로 자동화 부담이 더 큼. 단순성 우선.
- 옵션 B: mockgen 도입. Makefile target 추가 + `//go:generate` 주석 패턴. 향후 인터페이스 수 증가 시 유리.

### Q3: Protocol Buffers 도구 버전

**질문**: buf CLI 버전?

- 옵션 A (권장 default): **buf v1.39+** (research.md §1.2 명시). 가장 최신 안정. remote plugin 지원 (codegen 환경 설정 단순).
- 옵션 B: buf v1.32 (tech.md §12.1 핀 grpc-gateway v2.18.0 시점). 보수적이나 grpc-gateway v2.20 plugin과 미스매치 위험.

### Q4: Test Database Isolation

**질문**: testcontainers-go 통합 테스트 격리 전략?

- 옵션 A (권장 default): **shared per package + per-test schema isolation** (§3.2 + §5.2). 빠르고 격리 명확.
- 옵션 B: per-suite testcontainer (각 _test.go마다 새 컨테이너). 매우 느림. 시간 폭증.
- 옵션 C: transaction rollback (각 테스트 BEGIN → ROLLBACK). SELECT FOR UPDATE 동시성 테스트와 충돌.

---

## 9. Philosopher Framework 적용

### 9.1 Assumption Audit

| Assumption | Confidence | Risk if Wrong |
|------------|-----------|--------------|
| SPEC-AX-001 GREEN 완료 + audit_logs 스키마 단일 source | High | Medium (Sprint 1 schema 검증으로 mitigation) |
| `tests/integration/test_audit_consistency.py` (SPEC-AX-001)에서 audit schema cross-validate | Medium | Medium |
| Docker daemon 가용 | Medium | High (Sprint 3 + 7 testcontainers 의존) |
| Python `CELERY_TASK_SERIALIZER='json'` 설정 완료 | Medium | High (Sprint 6 golden file 무효화) |
| Go 1.22 baseline 유지 가능 (1.23 features 사용 안 함) | High | Low |
| pgx/v5 v5.6.0 안정 | High | Low |
| miniredis가 Celery RPUSH 시나리오 정확 모사 | High | Low (Sprint 6 golden file로 cross-check) |

### 9.2 First Principles Decomposition

- Surface: Go control-plane이 Python pipelines를 비동기로 호출하는 SPOF
- Why 1: HTTP REST/gRPC로 외부 클라이언트 API 노출
- Why 2: 워크플로우 상태 관리(PENDING/RUNNING/COMPLETED/FAILED) + audit
- Why 3: 한국 공공기관 규제 환경에서 비즈니스 로직 + audit 단일 atomic transaction
- Why 4: 데이터 무결성이 컴플라이언스 핵심
- **Root**: 망분리 + 감사 일관성 + 외부 API 차단 = constitutional 요구사항

### 9.3 Alternative Generation (Sprint Ordering)

- **Conservative (low risk, incremental)**: State → Store → gRPC → REST → Dispatch (본 strategy 채택) — store가 gRPC handler 의존성 표면 최소화
- **Balanced (moderate risk)**: gRPC interface 먼저 → store → state machine → dispatch — interface-first, 그러나 빈 store 위에 gRPC RED test는 mock overhead 과대
- **Aggressive (transformative)**: 5 sub-sprint 병렬 (state + store + dispatch + gRPC + REST) — sprint contract artifact 충돌 위험 + thorough harness evaluator scoring 일관성 손상

**채택: Conservative** — testcontainers-go shared per package 모델과 정합 + sprint contract 안정.

### 9.4 Trade-off Matrix (전체 SPEC 진행 전략)

가중치: Risk 30% / Implementation Cost 25% / Maintainability 20% / Performance 15% / Scalability 10%.

| 옵션 | Risk | Cost | Maint | Perf | Scale | 가중점수 |
|------|------|------|-------|------|-------|---------|
| (A) plan.md §2 순서 그대로 (S1→S2→S3→S4→S5) | 6/10 | 7/10 | 7/10 | 7/10 | 7/10 | **6.65** |
| (B) 본 strategy 순서 (UBI 분리 + Dispatch 뒤로) | 7/10 | 6/10 | 8/10 | 7/10 | 7/10 | **6.95** |
| (C) gRPC 먼저 (B-first) | 5/10 | 5/10 | 6/10 | 7/10 | 7/10 | 5.65 |

**결정: (B) 채택** — UBI invariant를 일찍 확립하여 후속 sprint atomicity 검증 자연스러움. Dispatch를 뒤로 미루어 golden file external dependency를 늦게 도입 → Sprint 2-5가 외부 입력 의존 없이 단위 테스트만으로 GREEN 도달 가능.

### 9.5 Cognitive Bias Check

- **Anchoring bias**: plan.md §2가 S1~S5 순서를 명시 → 본 strategy가 의도적으로 UBI 분리 + Dispatch 후행. plan.md ordering이 anchor가 되지 않도록 §9.4 trade-off matrix로 명시 평가.
- **Confirmation bias**: spec-compact.md + plan.md + acceptance.md + research.md 모두 검토하여 단일 문서에 confirmation 의존하지 않음.
- **Sunk cost bias**: Sprint 0 stub(state_machine.go, dispatcher.go, postgres.go)이 이미 있으나 비즈니스 로직 0줄이므로 재작성 비용 0 — sunk cost 아님.
- **Overconfidence**: thorough harness가 자동 품질 보장이라는 가정 회피 → §6.2 per-sprint quality gate + §3.3 벤치마크 명시.

**This option might fail because**:
- (a) testcontainers-go가 WSL2 Docker daemon에서 docker-in-docker 이슈 발생 가능 → mitigation: Pre-Sprint 0에서 `docker info` 확인, 실패 시 expert-devops 호출
- (b) Sprint 6 Celery golden file 생성을 SPEC-AX-001 maintenance에 외주하므로 1주일 이상 지연 가능 → mitigation: 본 SPEC Sprint 0~5는 dispatch 의존 없이 GREEN 도달 가능하도록 의도적으로 sequencing(§1.2)
- (c) Go 1.22 baseline 유지가 라이브러리 업그레이드로 깨질 수 있음 → mitigation: go.mod 핀 + `tech.md §12.1` 정책 + CI Go 1.22+1.23 matrix

---

## 10. Definition of Done (Phase 1 본 strategy)

- [x] `.moai/plans/enumerated-plotting-manatee-agent-a5b25a6d32551c5c0.md` (본 파일) 작성 완료
- [ ] (plan mode 해제 후) 본 내용을 `.moai/specs/SPEC-AX-CTRL-001/strategy.md`로 이전 또는 재생성
- [x] Sprint 시퀀싱 (Pre-Sprint 0 + 8 sprints) + 시퀀싱 정당화 (vs plan.md §2)
- [x] Foundation Setup (go.mod 의존성 + buf codegen + 디렉토리 구조 + pkg/models 확장 + config)
- [x] Test Infrastructure (testify + testcontainers-go + miniredis + benchmark + mocking)
- [x] Per-Sprint Sprint Contract scaffold (8 sprints 각각의 priority dimension + checklist + scenarios + pass condition + evaluator 4-dim)
- [x] Risk-Driven Decisions (Celery golden file, Postgres isolation, audit schema, gRPC reflection, isolation level)
- [x] LSP / Lint Baseline (capture + per-sprint gate + golangci-lint config)
- [x] Specialist Routing (sprint-by-sprint owner + assist + worktree policy + commit pattern)
- [x] Open Questions (4개, 모두 sensible default 보유)
- [x] Philosopher Framework (Assumption / First Principles / Alternative / Trade-off matrix / Bias check)

---

## 11. RETURN to Orchestrator (≤ 300 words)

- **File created**: `/home/sklee/moai/iroum-ax/.moai/plans/enumerated-plotting-manatee-agent-a5b25a6d32551c5c0.md` (plan mode 활성으로 인해 원래 `.moai/specs/SPEC-AX-CTRL-001/strategy.md` 대신 plan 파일에 작성. plan mode 해제 후 orchestrator가 동일 내용을 spec 경로로 이동·재생성 필요. SPEC-AX-001 strategy.md가 따른 동일 패턴.)
- **Total sprint count**: **8** (Pre-Sprint 0 환경 검증 + Sprint 0 Foundation + Sprint 1 UBI + Sprint 2 State Machine + Sprint 3 Postgres Store + Sprint 4 gRPC + Sprint 5 REST + Sprint 6 Celery Dispatch + Sprint 7 E2E)
- **Critical path REQ ordering**: go.mod 정리 → pkg/models 확장 → REQ-CTRL-UBI-001/002 → REQ-CTRL-001(State Machine) → REQ-CTRL-004(Postgres Store) → REQ-CTRL-002(gRPC) → REQ-CTRL-003(REST) → REQ-CTRL-005(Celery Dispatch) → E2E. plan.md §2의 S1~S5 순서에서 UBI를 Sprint 1로 격상하고 Dispatch를 마지막에서 두 번째로 이동(§1.2 정당화).
- **Top 3 risks + mitigation**:
  1. **R-CTRL-001 Celery envelope 호환성**: Python kombu 측에서 reference envelope 1회 생성 후 testdata로 커밋(§5.1 Decision 1), Sprint 6 byte-match 검증
  2. **R-CTRL-003 PENDING 고아 워크플로우**: handlers.go에서 PostgreSQL tx commit 후 dispatch 시도, 실패 시 즉시 PENDING→FAILED 전이(research.md R-CTRL-003 acceptance). Outbox 패턴은 후속 SPEC
  3. **R-CTRL-002 State machine race**: SELECT FOR UPDATE 의무화 + READ COMMITTED + 2-goroutine 직렬화 단위 테스트 강제(§5.5 Decision 5)
- **Open questions count**: **4** (모두 sensible default 보유): (Q1) golangci-lint custom config, (Q2) hand-written fakes vs mockgen, (Q3) buf v1.39+, (Q4) per-test schema isolation
- **Ready for Run Phase 2 Sprint 0 entry**: **YES** (4 open question은 모두 default로 진행 가능. orchestrator가 Pre-Sprint 0 환경 검증 — `docker info` — 완료 후 Sprint 0 Foundation을 manager-tdd → expert-backend(isolation: worktree) → expert-devops 순으로 spawn 가능. Docker daemon 미가용 시 expert-devops가 정비 후 진입.)
