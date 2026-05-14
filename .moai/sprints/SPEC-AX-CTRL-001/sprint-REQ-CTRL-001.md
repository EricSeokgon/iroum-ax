# Sprint Contract — SPEC-AX-CTRL-001 Sprint 2: REQ-CTRL-001

> Harness level: thorough
> Priority dimension: **Functionality** + **Safety** (상태 머신 불변 조건 + 동시성 직렬화)
> Created: 2026-05-14
> Phase: RED (완료) → GREEN → REFACTOR

---

## 1. Acceptance Checklist

- [ ] [AC-CTRL-001-1] PENDING → RUNNING 정상 전이 + WORKFLOW_TRANSITIONED_TO_RUNNING 감사 이벤트
- [ ] [AC-CTRL-001-2] RUNNING → COMPLETED 정상 전이 + resultJSON 저장 + WORKFLOW_COMPLETED 감사 이벤트
- [ ] [AC-CTRL-001-3] 유효하지 않은 전이 시도 시 ErrInvalidTransition 반환 + 상태 불변
- [ ] [AC-CTRL-001-4] 동시 Start() 호출 시 정확히 1개만 성공 (FakeStore 뮤텍스 직렬화)
- [ ] [AC-CTRL-001-5] 종료 상태(COMPLETED/FAILED) 이후 모든 전이 시도 ErrInvalidTransition 반환
- [ ] [EDGE] RUNNING → FAILED 정상 전이 (dispatch 실패 경로)
- [ ] [EDGE] PENDING → FAILED 정상 전이 (serialization 실패 경로)
- [ ] [EDGE] 존재하지 않는 워크플로우 CurrentState 조회 시 에러 반환
- [ ] [EDGE] 기존 PENDING 워크플로우 CurrentState 조회 시 StatusPending 반환
- [ ] [EDGE] 감사 INSERT 실패 시 워크플로우 상태 롤백 (트랜잭션 원자성)
- [ ] [EDGE] resultJSON 바이트 정확성 저장 검증

---

## 2. Test Scenarios (14 RED tests in state_machine_test.go)

| 테스트 함수 | AC | 검증 내용 |
|------------|-----|---------|
| `TestStart_FromPending_Success` | AC-CTRL-001-1 | PENDING→RUNNING + audit(ActionWorkflowTransitionedToRunning) |
| `TestComplete_FromRunning_Success` | AC-CTRL-001-2 | RUNNING→COMPLETED + resultJSON + audit(ActionWorkflowCompleted) |
| `TestStart_FromCompleted_RejectsTransition` | AC-CTRL-001-3 | ErrInvalidTransition + 상태 COMPLETED 불변 |
| `TestComplete_FromPending_RejectsTransition` | AC-CTRL-001-3 | ErrInvalidTransition + 상태 PENDING 불변 |
| `TestStart_FromFailed_RejectsTransition` | AC-CTRL-001-3 | FAILED 종료 상태에서 ErrInvalidTransition |
| `TestStart_ConcurrentCalls_ExactlyOneSucceeds` | AC-CTRL-001-4 | 2 goroutines, 1 성공, 1 ErrInvalidTransition |
| `TestComplete_TerminalState_NoFurtherTransitions` | AC-CTRL-001-5 | 테이블 서브테스트: Start/Complete/Fail 모두 거부 |
| `TestFail_FromRunning_Success` | EDGE | RUNNING→FAILED + audit 1개 |
| `TestFail_FromPending_Success` | EDGE | PENDING→FAILED |
| `TestCurrentState_NotFound_ReturnsError` | EDGE | 비존재 ID → 에러 반환 |
| `TestCurrentState_ExistingWorkflow_ReturnsPendingState` | EDGE | PENDING → StatusPending |
| `TestStart_AuditRecordFails_Rollback` | EDGE | 감사 실패 → 상태 PENDING 유지 |
| `TestComplete_WithResultJSON_PersistsCorrectly` | EDGE | resultJSON 바이트 정확성 |
| `TestCurrentState_InvalidStateValue_RejectedAtBoundary` | EDGE | 비정상 상태값 경계 처리 |

---

## 3. Implementation Constraints (GREEN 단계)

### StateMachine 메서드 구현 규칙

**Start(ctx, workflowID string) error**
1. `TxCoordinator.ExecuteWorkflowTransition(ctx, workflowID, PENDING, RUNNING, DefaultUserID)` 호출
2. `ExecuteWorkflowTransition` 내부에서:
   - `tx.GetWorkflow(ctx, workflowID)` 로 현재 상태 조회
   - `types.IsValidTransition(current, RUNNING)` 검증 → false면 `ErrInvalidTransition` 반환
   - `tx.UpdateWorkflowState(ctx, workflowID, RUNNING)` 호출
   - `recorder.RecordTransition(ctx, tx, ...)` 로 감사 기록
   - `tx.Commit(ctx)` 호출

**Complete(ctx, workflowID string, resultJSON []byte) error**
- RUNNING → COMPLETED 전이
- `tx.UpdateWorkflowState`는 resultJSON도 함께 저장해야 함
- 또는 별도 `tx.UpdateWorkflowResult(ctx, workflowID, COMPLETED, resultJSON)` 메서드 추가 검토

**Fail(ctx, workflowID string, reason string) error**
- PENDING 또는 RUNNING → FAILED 전이
- reason을 audit details에 포함

**CurrentState(ctx, workflowID string) (Status, error)**
- `TxCoordinator.store.BeginTx` → `tx.GetWorkflow` → `tx.Rollback`
- `types.WorkflowState` → `workflow.Status` 변환 필요

### 동시성 보장 (AC-CTRL-001-4)
- `FakeStore.mu` 뮤텍스가 `BeginTx`를 직렬화
- Sprint 3에서 `SELECT ... FOR UPDATE`로 교체 예정

### 인터페이스 변경 사항
- `store.WorkflowTx.UpdateWorkflowState`에 resultJSON 파라미터 추가 검토
  - 또는 `types.WorkflowState` + `[]byte` 분리 저장용 별도 메서드

---

## 4. RED Phase 완료 확인 (2026-05-14)

| 검증 항목 | 결과 |
|----------|------|
| 신규 테스트 컴파일 성공 | PASS |
| 신규 테스트 FAIL (RED) | 11개 FAIL, 3개 PASS (의도된 관대한 단언) |
| `go vet ./apps/control-plane/...` | 0 errors |
| `golangci-lint run ./apps/control-plane/...` | 0 errors |
| Sprint 1 테스트 회귀 없음 | audit PASS, store PASS |

---

## 5. Pass Conditions (GREEN 완료 시)

- 14개 신규 테스트 모두 PASS
- Sprint 1 테스트 회귀 없음 (21개 PASS 유지)
- `go vet` 0 error
- `golangci-lint` 0 error
- coverage ≥ 85%

---

## 6. Evaluator 4-Dimension Scoring (Sprint 2)

| Dimension | Weight | Target Score |
|-----------|--------|-------------|
| Functionality | 40% | ≥ 0.75 |
| Safety | 30% | ≥ 0.75 |
| Completeness | 20% | ≥ 0.75 |
| Originality | 10% | ≥ 0.75 |

Overall pass threshold: ≥ 0.75 (strict profile per thorough harness)

---

## 7. Sprint 2 Artifact Paths

| 파일 | 역할 |
|------|------|
| `apps/control-plane/internal/workflow/state_machine.go` | StateMachine 타입 + 4개 메서드 stub (RED) |
| `apps/control-plane/internal/workflow/state_machine_test.go` | 14개 RED tests |
| `apps/control-plane/internal/store/store.go` | WorkflowTx에 GetWorkflow 추가 |
| `apps/control-plane/internal/store/fake_store.go` | FakeTx.GetWorkflow + FakeStore.SeedWorkflow 추가 |

---

Sprint Contract version: 1.0.0
