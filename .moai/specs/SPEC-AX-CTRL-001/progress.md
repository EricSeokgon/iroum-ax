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

## 향후 Sprint

| Sprint | REQ | 예상 시작 |
|--------|-----|---------|
| Sprint 2 GREEN | REQ-CTRL-001 State Machine 구현 | Sprint 2 RED 완료 후 |
| Sprint 3 | REQ-CTRL-004 PostgreSQL Store (testcontainers) | Sprint 2 완료 후 |
| Sprint 4 | REQ-CTRL-002 gRPC Server | Sprint 3 완료 후 |
| Sprint 5 | REQ-CTRL-003 REST API (gRPC-Gateway) | Sprint 4 완료 후 |
| Sprint 6 | REQ-CTRL-005 Celery Dispatch (miniredis) | Sprint 5 완료 후 |
| Sprint 7 | E2E Integration (docker-compose) | Sprint 6 완료 후 |
