// state_machine_test.go — REQ-CTRL-001 상태 머신 RED phase 테스트
// Sprint 2: AC-CTRL-001-1~5 + 엣지 케이스
// 모든 테스트는 GREEN 구현 전까지 FAIL 상태여야 함 (RED phase 확인)
//
// AC 매핑:
//   - AC-CTRL-001-1: TestStart_FromPending_Success (PENDING → RUNNING + audit)
//   - AC-CTRL-001-2: TestComplete_FromRunning_Success (RUNNING → COMPLETED + resultJSON)
//   - AC-CTRL-001-3: TestStart_FromCompleted_RejectsTransition, TestComplete_FromPending_RejectsTransition
//   - AC-CTRL-001-4: TestStart_ConcurrentCalls_ExactlyOneSucceeds
//   - AC-CTRL-001-5: TestComplete_TerminalState_NoFurtherTransitions
package workflow_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	cperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 헬퍼: 테스트용 StateMachine 조립 및 단언
// ---------------------------------------------------------------------------

// newTestStateMachine FakeStore + Recorder + TxCoordinator로 StateMachine을 조립
// authEnabled=false (Walking Skeleton 기본값)
func newTestStateMachine(s *store.FakeStore) *workflow.StateMachine {
	recorder := audit.NewRecorder(false)
	coord := workflow.NewTxCoordinator(s, recorder)
	return workflow.NewStateMachine(coord, nil) // logger=nil은 테스트에서 허용
}

// assertWorkflowState FakeStore 영속 저장소에서 워크플로우 상태를 확인
func assertWorkflowState(t *testing.T, s *store.FakeStore, workflowID string, expected types.WorkflowState) {
	t.Helper()
	wf, ok := s.Workflows[workflowID]
	if !ok {
		t.Errorf("assertWorkflowState: workflow %q not found in store", workflowID)
		return
	}
	if wf.State != expected {
		t.Errorf("assertWorkflowState: got state %q, want %q", wf.State, expected)
	}
}

// assertWorkflowResultJSON FakeStore 영속 저장소에서 워크플로우 ResultJSON을 확인
func assertWorkflowResultJSON(t *testing.T, s *store.FakeStore, workflowID string, expected []byte) {
	t.Helper()
	wf, ok := s.Workflows[workflowID]
	if !ok {
		t.Errorf("assertWorkflowResultJSON: workflow %q not found in store", workflowID)
		return
	}
	if string(wf.ResultJSON) != string(expected) {
		t.Errorf("assertWorkflowResultJSON: got %q, want %q", wf.ResultJSON, expected)
	}
}

// seedPending PENDING 상태 워크플로우를 FakeStore에 직접 삽입
func seedPending(s *store.FakeStore) *types.Workflow {
	wf := &types.Workflow{
		ID:         uuid.New(),
		State:      types.WorkflowStatePending,
		DocumentID: uuid.New(),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	s.SeedWorkflow(wf)
	return wf
}

// seedRunning RUNNING 상태 워크플로우를 FakeStore에 직접 삽입
func seedRunning(s *store.FakeStore) *types.Workflow {
	wf := &types.Workflow{
		ID:         uuid.New(),
		State:      types.WorkflowStateRunning,
		DocumentID: uuid.New(),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	s.SeedWorkflow(wf)
	return wf
}

// seedCompleted COMPLETED 상태 워크플로우를 FakeStore에 직접 삽입
func seedCompleted(s *store.FakeStore) *types.Workflow {
	wf := &types.Workflow{
		ID:         uuid.New(),
		State:      types.WorkflowStateCompleted,
		DocumentID: uuid.New(),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	s.SeedWorkflow(wf)
	return wf
}

// ---------------------------------------------------------------------------
// AC-CTRL-001-1: PENDING → RUNNING 정상 경로
// ---------------------------------------------------------------------------

// TestStart_FromPending_Success — AC-CTRL-001-1
// PENDING 워크플로우에 Start() 호출 시:
//   - 에러 없이 성공
//   - FakeStore의 워크플로우 상태가 RUNNING으로 갱신됨
//   - 감사 로그에 WORKFLOW_TRANSITIONED_TO_RUNNING 행이 1개 존재
func TestStart_FromPending_Success(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedPending(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	// RED: errStateMachineNotImplemented 반환으로 실패 예정
	err := sm.Start(ctx, wf.ID.String())
	require.NoError(t, err, "PENDING → RUNNING 전이는 성공해야 함")

	// 상태 갱신 확인
	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStateRunning)

	// 감사 로그 확인
	require.Len(t, s.AuditLogs, 1, "감사 로그 행이 정확히 1개여야 함")
	assert.Equal(t, audit.ActionWorkflowTransitionedToRunning, s.AuditLogs[0].Action,
		"감사 액션이 WORKFLOW_TRANSITIONED_TO_RUNNING이어야 함")
	assert.Equal(t, audit.DefaultUserID, s.AuditLogs[0].UserID,
		"감사 이벤트의 user_id가 cli-anonymous여야 함")
}

// ---------------------------------------------------------------------------
// AC-CTRL-001-2: RUNNING → COMPLETED 정상 경로 + resultJSON 저장
// ---------------------------------------------------------------------------

// TestComplete_FromRunning_Success — AC-CTRL-001-2
// RUNNING 워크플로우에 Complete() 호출 시:
//   - 에러 없이 성공
//   - 워크플로우 상태가 COMPLETED로 갱신됨
//   - resultJSON이 올바르게 저장됨
//   - 감사 로그에 WORKFLOW_COMPLETED 행이 1개 존재
func TestComplete_FromRunning_Success(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedRunning(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	resultJSON := []byte(`{"score": 0.95, "pages": 10}`)

	// RED: errStateMachineNotImplemented 반환으로 실패 예정
	err := sm.Complete(ctx, wf.ID.String(), resultJSON)
	require.NoError(t, err, "RUNNING → COMPLETED 전이는 성공해야 함")

	// 상태 갱신 확인
	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStateCompleted)

	// resultJSON 저장 확인
	assertWorkflowResultJSON(t, s, wf.ID.String(), resultJSON)

	// 감사 로그 확인
	require.Len(t, s.AuditLogs, 1)
	assert.Equal(t, audit.ActionWorkflowCompleted, s.AuditLogs[0].Action)
}

// ---------------------------------------------------------------------------
// AC-CTRL-001-3: 유효하지 않은 전이 거부 (ErrInvalidTransition)
// ---------------------------------------------------------------------------

// TestStart_FromCompleted_RejectsTransition — AC-CTRL-001-3
// COMPLETED(종료 상태) 워크플로우에 Start() 호출 시:
//   - ErrInvalidTransition 에러 반환
//   - 워크플로우 상태가 변경되지 않음
func TestStart_FromCompleted_RejectsTransition(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedCompleted(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	// RED: errStateMachineNotImplemented 반환으로 이 단언이 실패 예정
	err := sm.Start(ctx, wf.ID.String())
	assert.ErrorIs(t, err, cperrors.ErrInvalidTransition,
		"종료 상태에서 Start()는 ErrInvalidTransition을 반환해야 함")

	// 상태 불변 확인
	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStateCompleted)
	assert.Empty(t, s.AuditLogs, "실패한 전이 시도에서 감사 로그가 기록되지 않아야 함")
}

// TestComplete_FromPending_RejectsTransition — AC-CTRL-001-3 (변형)
// PENDING → COMPLETED 직접 전이 시도 (RUNNING 건너뜀):
//   - ErrInvalidTransition 에러 반환
//   - 워크플로우 상태가 PENDING으로 유지됨
func TestComplete_FromPending_RejectsTransition(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedPending(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	// RED: errStateMachineNotImplemented 반환으로 이 단언이 실패 예정
	err := sm.Complete(ctx, wf.ID.String(), nil)
	assert.ErrorIs(t, err, cperrors.ErrInvalidTransition,
		"PENDING → COMPLETED 직접 전이는 ErrInvalidTransition을 반환해야 함")

	// 상태 불변 확인
	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStatePending)
	assert.Empty(t, s.AuditLogs)
}

// TestStart_FromFailed_RejectsTransition — AC-CTRL-001-3 (변형: FAILED 종료 상태)
// FAILED(종료 상태) 워크플로우에 Start() 호출 시 ErrInvalidTransition 반환
func TestStart_FromFailed_RejectsTransition(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := &types.Workflow{
		ID:    uuid.New(),
		State: types.WorkflowStateFailed,
	}
	s.SeedWorkflow(wf)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	err := sm.Start(ctx, wf.ID.String())
	assert.ErrorIs(t, err, cperrors.ErrInvalidTransition,
		"FAILED 종료 상태에서 Start()는 ErrInvalidTransition을 반환해야 함")
}

// ---------------------------------------------------------------------------
// AC-CTRL-001-4: 동시 전이 — 정확히 1개만 성공
// ---------------------------------------------------------------------------

// TestStart_ConcurrentCalls_ExactlyOneSucceeds — AC-CTRL-001-4
// 2개의 고루틴이 동시에 동일한 워크플로우에 Start()를 호출할 때:
//   - 정확히 1개만 성공 (nil 에러)
//   - 나머지 1개는 ErrInvalidTransition 반환
//   - 워크플로우 최종 상태가 RUNNING (혼합 상태 금지)
//   - FakeStore 뮤텍스로 직렬화 보장
//
// @MX:WARN: [AUTO] 동시성 테스트 — race detector 필수 (-race 플래그)
// @MX:REASON: AC-CTRL-001-4 SELECT FOR UPDATE 직렬화 불변 조건 검증
func TestStart_ConcurrentCalls_ExactlyOneSucceeds(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedPending(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	const goroutineCount = 2

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		errors = make([]error, 0, goroutineCount)
	)

	// 두 고루틴이 동시에 Start()를 호출
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := sm.Start(ctx, wf.ID.String())
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// 정확히 하나만 성공 (nil 에러)
	successCount := 0
	invalidTransitionCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		} else if assert.ErrorIs(t, err, cperrors.ErrInvalidTransition) {
			invalidTransitionCount++
		}
	}

	assert.Equal(t, 1, successCount,
		"동시 Start() 호출에서 정확히 1개만 성공해야 함")
	assert.Equal(t, 1, invalidTransitionCount,
		"나머지 1개는 ErrInvalidTransition을 반환해야 함")

	// 최종 상태가 RUNNING이어야 함
	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStateRunning)

	// 감사 로그 정확히 1개 (성공한 전이 1개만)
	assert.Len(t, s.AuditLogs, 1,
		"동시 호출 중 성공한 1개만 감사 로그를 생성해야 함")
}

// ---------------------------------------------------------------------------
// AC-CTRL-001-5: 종료 상태 불변 — 이후 모든 전이 시도 거부
// ---------------------------------------------------------------------------

// TestComplete_TerminalState_NoFurtherTransitions — AC-CTRL-001-5
// RUNNING → COMPLETED 성공 후 추가 전이 시도:
//   - Start() → ErrInvalidTransition
//   - Complete() → ErrInvalidTransition
//   - Fail() → ErrInvalidTransition
func TestComplete_TerminalState_NoFurtherTransitions(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedRunning(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	// 먼저 COMPLETED로 전이
	err := sm.Complete(ctx, wf.ID.String(), []byte(`{"result":"ok"}`))
	require.NoError(t, err, "초기 RUNNING → COMPLETED 전이는 성공해야 함")

	// COMPLETED 이후 모든 전이 거부
	// fieldalignment: fn(함수 포인터 8B)을 name(string 16B) 앞에 배치하여 구조체 패딩 최소화
	tests := []struct {
		fn   func() error
		name string
	}{
		{func() error { return sm.Start(ctx, wf.ID.String()) }, "Start after Completed"},
		{func() error { return sm.Complete(ctx, wf.ID.String(), nil) }, "Complete after Completed"},
		{func() error { return sm.Fail(ctx, wf.ID.String(), "test") }, "Fail after Completed"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.fn()
			assert.ErrorIs(t, err, cperrors.ErrInvalidTransition,
				"%s: 종료 상태 이후 전이는 ErrInvalidTransition을 반환해야 함", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// 엣지 케이스: Fail 메서드
// ---------------------------------------------------------------------------

// TestFail_FromRunning_Success — RUNNING → FAILED 정상 경로
// RUNNING 워크플로우에 Fail() 호출 시:
//   - 에러 없이 성공
//   - 워크플로우 상태가 FAILED로 갱신됨
//   - 감사 로그에 WORKFLOW_FAILED_* 행이 존재
func TestFail_FromRunning_Success(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedRunning(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	err := sm.Fail(ctx, wf.ID.String(), "redis_unavailable")
	require.NoError(t, err, "RUNNING → FAILED 전이는 성공해야 함")

	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStateFailed)
	require.Len(t, s.AuditLogs, 1)
}

// TestFail_FromPending_Success — PENDING → FAILED 정상 경로 (dispatch 실패 시 사용)
func TestFail_FromPending_Success(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedPending(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	err := sm.Fail(ctx, wf.ID.String(), "envelope_serialization_failed")
	require.NoError(t, err, "PENDING → FAILED 전이는 성공해야 함")

	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStateFailed)
}

// ---------------------------------------------------------------------------
// 엣지 케이스: CurrentState
// ---------------------------------------------------------------------------

// TestCurrentState_NotFound_ReturnsError — 존재하지 않는 워크플로우 조회
// GetWorkflow가 ErrWorkflowNotFound를 반환하면 CurrentState도 에러를 반환해야 함
func TestCurrentState_NotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	sm := newTestStateMachine(s)
	ctx := context.Background()

	nonExistentID := uuid.New().String()

	_, err := sm.CurrentState(ctx, nonExistentID)
	// ErrWorkflowNotFound 또는 래핑된 에러여야 함
	assert.Error(t, err, "존재하지 않는 워크플로우 조회 시 에러를 반환해야 함")
}

// TestCurrentState_ExistingWorkflow_ReturnsPendingState — 기존 워크플로우 상태 조회
func TestCurrentState_ExistingWorkflow_ReturnsPendingState(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedPending(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	state, err := sm.CurrentState(ctx, wf.ID.String())
	require.NoError(t, err, "존재하는 워크플로우 상태 조회는 성공해야 함")
	assert.Equal(t, workflow.StatusPending, state,
		"PENDING 상태 워크플로우의 CurrentState는 StatusPending이어야 함")
}

// ---------------------------------------------------------------------------
// 엣지 케이스: 감사 기록 실패 → 롤백
// ---------------------------------------------------------------------------

// TestStart_AuditRecordFails_Rollback — 감사 INSERT 실패 시 워크플로우 상태 롤백
// AC-CTRL-UBI-001 Scenario C를 상태 머신 레이어에서 검증
//
// 감사 INSERT가 실패하면:
//   - Start()가 에러를 반환해야 함
//   - 워크플로우 상태가 PENDING으로 유지되어야 함 (UPDATE도 롤백)
//   - 감사 로그에 행이 없어야 함
func TestStart_AuditRecordFails_Rollback(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedPending(s)
	// 감사 INSERT 장애 주입
	s.NextTxFailOnAudit = true
	sm := newTestStateMachine(s)
	ctx := context.Background()

	err := sm.Start(ctx, wf.ID.String())
	assert.Error(t, err, "감사 INSERT 실패 시 Start()는 에러를 반환해야 함")

	// 워크플로우 상태가 PENDING으로 유지되어야 함
	assertWorkflowState(t, s, wf.ID.String(), types.WorkflowStatePending)
	assert.Empty(t, s.AuditLogs, "롤백 후 감사 로그가 없어야 함")
}

// ---------------------------------------------------------------------------
// 엣지 케이스: resultJSON 저장 정확성
// ---------------------------------------------------------------------------

// TestComplete_WithResultJSON_PersistsCorrectly — resultJSON이 올바르게 저장됨
// types.Workflow.ResultJSON 필드에 바이트 슬라이스가 정확히 저장되어야 함
func TestComplete_WithResultJSON_PersistsCorrectly(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	wf := seedRunning(s)
	sm := newTestStateMachine(s)
	ctx := context.Background()

	expectedJSON := []byte(`{"pages": 42, "score": 0.99, "model": "iroum-v2"}`)

	err := sm.Complete(ctx, wf.ID.String(), expectedJSON)
	require.NoError(t, err)

	// resultJSON 저장 정확성 확인
	assertWorkflowResultJSON(t, s, wf.ID.String(), expectedJSON)
}

// ---------------------------------------------------------------------------
// 엣지 케이스: 알 수 없는 상태 값 처리
// ---------------------------------------------------------------------------

// TestCurrentState_InvalidStateValue_RejectedAtBoundary — 내부 저장소에 비정상 상태값이 있을 때
// FakeStore에 직접 알 수 없는 상태를 심어 경계 조건을 검증
// 실제 운영에서는 DB의 ENUM 제약으로 발생 불가하나 방어 코드 검증 목적
func TestCurrentState_InvalidStateValue_RejectedAtBoundary(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()

	// 유효하지 않은 상태 값을 직접 삽입
	wf := &types.Workflow{
		ID:    uuid.New(),
		State: types.WorkflowState("INVALID_STATE"),
	}
	s.SeedWorkflow(wf)

	sm := newTestStateMachine(s)
	ctx := context.Background()

	// CurrentState 조회는 성공하지만, 이 상태에서의 전이는 IsValidTransition으로 거부됨
	// (ValidTransitions 맵에 "INVALID_STATE" 항목이 없으므로)
	state, err := sm.CurrentState(ctx, wf.ID.String())
	// 구현에 따라 에러를 반환하거나 원시 값을 반환할 수 있음
	// GREEN 단계에서 구현 선택에 따라 단언 조건을 결정
	if err == nil {
		// 원시 값을 반환한 경우: Start()는 ErrInvalidTransition을 반환해야 함
		_ = state
		err2 := sm.Start(ctx, wf.ID.String())
		// 알 수 없는 상태에서 Start()는 ErrInvalidTransition 또는 다른 에러
		assert.Error(t, err2,
			"알 수 없는 상태에서 전이 시도는 에러를 반환해야 함")
	} else {
		// 조회 자체가 에러인 경우도 유효
		assert.Error(t, err)
	}
}
