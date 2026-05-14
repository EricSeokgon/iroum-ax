# SPEC-AX-CTRL-001 진행 상황 추적

> Format: [날짜] | [Sprint] | [단계] | AC 완료 수 | 에러 수 Delta | 상태
> 목적: Re-planning Gate 감지를 위한 이터레이션별 AC 완료율 추적

---

## Sprint 0: Foundation

| 날짜 | 단계 | AC 완료 | 에러 Delta | 상태 |
|------|------|---------|-----------|------|
| 2026-05-14 | REFACTOR 완료 | 0/26 (Foundation 전용 AC 없음) | 0 | PASSED |

**Sprint 0 산출물**:
- `apps/control-plane/internal/types/workflow.go` — WorkflowState 타입 + ValidTransitions + IsValidTransition
- `apps/control-plane/internal/errors/errors.go` — sentinel 에러 5종
- `apps/control-plane/internal/audit/audit.go` — Action 8종 + Event struct
- `apps/control-plane/internal/config/config.go` — Config + Load()
- `apps/control-plane/internal/log/log.go` — zap 로거 팩토리
- `apps/control-plane/internal/store/postgres.go` — stub Store struct
- `apps/control-plane/internal/workflow/state_machine.go` — stub StateMachine
- `go.mod` — uuid, zap 핵심 의존성 + testcontainers, pgx, redis 간접 의존성

**Sprint 0 품질 게이트**: go vet PASS, golangci-lint PASS, LSP 0/0/0

---

## Sprint 1: REQ-CTRL-UBI-001/002 (진행 중)

| 날짜 | 단계 | AC 완료 | 에러 Delta | 상태 |
|------|------|---------|-----------|------|
| 2026-05-14 | RED | 0/4 | +0 | COMPLETE |

**Sprint 1 RED 산출물**:
- `.moai/sprints/SPEC-AX-CTRL-001/sprint-REQ-CTRL-UBI.md` — Sprint Contract (Security 우선)
- `apps/control-plane/internal/store/store.go` — WorkflowStore + WorkflowTx 인터페이스
- `apps/control-plane/internal/store/fake_store.go` — FakeStore + FakeTx stub
- `apps/control-plane/internal/store/fake_store_test.go` — 8개 RED 테스트
- `apps/control-plane/internal/audit/recorder.go` — Recorder stub
- `apps/control-plane/internal/audit/recorder_test.go` — 11개 RED 테스트 (enum 검증 1개 PASS 포함)
- `apps/control-plane/internal/workflow/transaction.go` — TxCoordinator stub
- `apps/control-plane/internal/workflow/transaction_test.go` — 7개 RED 테스트

**Sprint 1 목표 AC** (4개):
- [ ] AC-CTRL-UBI-001 (Scenario A/B/C)
- [ ] AC-CTRL-UBI-002-A
- [ ] AC-CTRL-UBI-002-B
- [ ] AC-CTRL-UBI-002-C

**실제 테스트 수**: 26개 (fake_store: 8, recorder: 11, transaction: 7)
**RED 상태 확인**:
- `go build ./apps/control-plane/...` → PASS (컴파일 오류 없음)
- `go vet ./apps/control-plane/...` → PASS (0 오류)
- `golangci-lint run ./apps/control-plane/...` → PASS (0 오류)
- `go test ./apps/control-plane/internal/...` → 22 FAIL / 4 PASS
  - 4 PASS: enum 상수 검증 1개 + "에러가 반환되어야 함" 조건 3개 (stub이 에러 반환하므로 의도적 PASS)
  - 22 FAIL: "not implemented" sentinel 에러로 실패 (정상 RED 상태)

**명명 충돌 해결**: Sprint 0 `postgres.go`의 `Store` struct와 충돌 방지를 위해 인터페이스를 `WorkflowStore`/`WorkflowTx`로 명명

---

---

## Sprint 2: REQ-CTRL-001 Workflow State Machine (진행 중)

| 날짜 | 단계 | AC 완료 | 에러 Delta | 상태 |
|------|------|---------|-----------|------|
| 2026-05-14 | RED | 0/11 | +0 | COMPLETE |

**Sprint 2 RED 산출물**:
- `.moai/sprints/SPEC-AX-CTRL-001/sprint-REQ-CTRL-001.md` — Sprint Contract (Functionality + Safety 우선)
- `apps/control-plane/internal/workflow/state_machine.go` — StateMachine 타입 + 4개 메서드 stub (RED)
- `apps/control-plane/internal/workflow/state_machine_test.go` — 14개 RED 테스트
- `apps/control-plane/internal/store/store.go` — WorkflowTx에 GetWorkflow 추가
- `apps/control-plane/internal/store/fake_store.go` — FakeTx.GetWorkflow + FakeStore.SeedWorkflow 추가

**Sprint 2 목표 AC** (5개 + edge cases):
- [ ] AC-CTRL-001-1 (PENDING→RUNNING + audit)
- [ ] AC-CTRL-001-2 (RUNNING→COMPLETED + resultJSON)
- [ ] AC-CTRL-001-3 (ErrInvalidTransition 거부)
- [ ] AC-CTRL-001-4 (동시 전이 직렬화)
- [ ] AC-CTRL-001-5 (종료 상태 불변)

**실제 테스트 수**: 14개 신규 (state_machine_test.go) + Sprint 1 21개 유지
**RED 상태 확인**:
- `go vet ./apps/control-plane/...` → PASS (0 오류)
- `golangci-lint run ./apps/control-plane/...` → PASS (0 오류)
- `go test ./apps/control-plane/internal/workflow/...` → 11 FAIL / 3 PASS
  - 3 PASS: 관대한 단언 구조 (에러 반환 여부만 검사하는 케이스)
  - 11 FAIL: "not implemented" sentinel 에러로 실패 (정상 RED 상태)
- Sprint 1 테스트 회귀 없음: audit PASS, store PASS

---

---

## Sprint 3: REQ-CTRL-004 PostgreSQL Store (진행 중)

| 날짜 | 단계 | AC 완료 | 에러 Delta | 상태 |
|------|------|---------|-----------|------|
| 2026-05-14 | RED | 0/4 (REQ-CTRL-004 AC 기준) | +0 | COMPLETE |

**Sprint 3 RED 산출물**:
- `.moai/sprints/SPEC-AX-CTRL-001/sprint-REQ-CTRL-004.md` — Sprint Contract (Functionality + Security 우선)
- `apps/control-plane/internal/store/pg_store.go` — PgWorkflowStore + PgWorkflowTx stub (ErrNotImplemented)
- `apps/control-plane/internal/store/postgres_test.go` — 11개 통합 테스트 (//go:build integration)
- `apps/control-plane/internal/store/testdata/schema.sql` — 통합 테스트용 PostgreSQL 스키마

**Sprint 3 목표 AC** (4개):
- [ ] AC-CTRL-004-1 (Pool 초기화 Fail-Fast + CRUD 왕복)
- [ ] AC-CTRL-004-2 (audit_logs JSONB INSERT)
- [ ] AC-CTRL-004-3 (SELECT FOR UPDATE 직렬화 + 풀 고갈)
- [ ] AC-CTRL-004-4 (mid-tx 실패 롤백)

**실제 통합 테스트 수**: 11개 (//go:build integration)
**RED 상태 확인**:
- `go build -tags=integration ./apps/control-plane/internal/store/...` → PASS
- `go vet -tags=integration ./apps/control-plane/internal/store/...` → PASS
- `golangci-lint run --build-tags=integration ./apps/control-plane/...` → PASS
- `go test -tags=integration ./apps/control-plane/internal/store/ -v -count=1`:
  - 10 FAIL (ErrNotImplemented — 정상 RED 상태)
  - 1 PASS (TestPgStore_Integration_NewPgWorkflowStore_InvalidDSN — DSN 실패는 stub에도 동작)
- Sprint 1+2 단위 테스트 회귀 없음: audit PASS, store PASS, workflow PASS

**testcontainers**: Docker 사용 가능, postgres:16-alpine 컨테이너 정상 스폰 확인

---

## Sprint 4: REQ-CTRL-002 gRPC Server

| 날짜 | 단계 | AC 완료 | 에러 Delta | 상태 |
|------|------|---------|-----------|------|
| 2026-05-14 | RED | 0/9 (Must Pass 기준) | +0 | COMPLETE |

**Sprint 4 RED 산출물**:
- `.moai/sprints/SPEC-AX-CTRL-001/sprint-REQ-CTRL-002.md` — Sprint Contract (Functionality 우선)
- `apps/control-plane/internal/proto/workflow.pb.go` — hand-written 프로토 메시지 타입 (WorkflowStatus enum, Workflow/Create/Get/List Request/Response)
- `apps/control-plane/internal/proto/workflow_grpc.pb.go` — hand-written gRPC 서비스 스텁 (WorkflowServiceServer 인터페이스, UnimplementedWorkflowServiceServer, RegisterWorkflowServiceServer, ServiceDesc, 핸들러 어댑터)
- `apps/control-plane/internal/server/grpc_server.go` — WorkflowService 구현체 (모든 메서드 codes.Unimplemented 반환)
- `apps/control-plane/internal/server/grpc_server_test.go` — 12개 테스트 (8 FAIL RED, 4 PASS)

**Sprint 4 목표 AC** (9개 Must Pass):
- [ ] AC-CTRL-002-1: CreateWorkflow → PENDING 상태 + audit 1건 + UUID 반환
- [ ] AC-CTRL-002-1 atomicity: audit INSERT 실패 시 rollback
- [ ] AC-CTRL-002-1 concurrent: 3 goroutine 동시 호출 모두 성공 + ID 고유
- [ ] AC-CTRL-002-2: GetWorkflow → 존재하는 ID 정상 반환
- [ ] AC-CTRL-002-2: GetWorkflow → 없는 ID codes.NotFound
- [ ] AC-CTRL-002-3: ListWorkflows 빈 목록 반환
- [ ] AC-CTRL-002-3: ListWorkflows N개 반환
- [ ] AC-CTRL-002-3: Limit 강제 (limit=2)
- [ ] AC-CTRL-002-3: context 취소 → codes.Canceled/DeadlineExceeded
- (Optional) AC-CTRL-002-4: Prometheus RPCCallCounter 메트릭

**실제 테스트 수**: 12개 신규 (grpc_server_test.go)
- 8 FAIL (정상 RED 상태): CreateWorkflow_HappyPath, GetWorkflow_Found, GetWorkflow_NotFound_ReturnsNotFoundStatus, ListWorkflows_EmptyStore, ListWorkflows_WithMultiple, ListWorkflows_LimitEnforcement, CreateWorkflow_ContextCancelled_ReturnsCanceledStatus, CreateWorkflow_ConcurrentCalls_AllSucceed
- 4 PASS (허용): CreateWorkflow_AuditFail_RollsBack (에러 동작 검증), GRPCServer_RegisterAndConnect, GRPCServer_Serve_AcceptsConnection, PrometheusMetrics_RPCCounter

**RED 상태 확인**:
- `go build ./apps/control-plane/...` → PASS
- `go vet ./apps/control-plane/internal/server/...` → PASS (0 오류)
- `golangci-lint run ./apps/control-plane/internal/server/...` → PASS (0 오류)
- `go test ./apps/control-plane/internal/server/...` → 8 FAIL / 4 PASS (정상 RED)
- Sprint 1+2+3 단위 테스트 40개 회귀 없음: audit PASS, store PASS, workflow PASS

**Proto Codegen 결정**: protoc/buf 미설치 환경에서 hand-written 방식 채택. GREEN 단계에서 protoc-gen-go 설치 후 buf generate 로 교체 예정.

**에러 Delta**: +0 (기존 LSP baseline 변동 없음)

---

## 향후 Sprint

| Sprint | REQ | 예상 시작 |
|--------|-----|---------|
| Sprint 4 GREEN | REQ-CTRL-002 gRPC Server 비즈니스 로직 구현 | Sprint 4 RED 완료 후 |
| Sprint 5 | REQ-CTRL-003 REST API (gRPC-Gateway) | Sprint 4 완료 후 |
| Sprint 6 | REQ-CTRL-005 Celery Dispatch (miniredis) | Sprint 5 완료 후 |
| Sprint 7 | E2E Integration (docker-compose) | Sprint 6 완료 후 |
