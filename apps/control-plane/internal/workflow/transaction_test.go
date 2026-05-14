// transaction_test.go — TxCoordinator 단위 테스트 (RED phase)
// AC-CTRL-UBI-001 Scenario A/B/C: 트랜잭션 원자성
// AC-CTRL-UBI-002-B: WORKFLOW_TRANSITIONED_TO_RUNNING 감사
// AC-CTRL-UBI-002-C: cli-anonymous 기본값
// 모든 테스트는 GREEN 단계 구현 전까지 FAIL 상태여야 함
package workflow_test

import (
	"context"
	"testing"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// newTestCoordinator 테스트용 TxCoordinator를 FakeStore로 생성
func newTestCoordinator(s *store.FakeStore) *workflow.TxCoordinator {
	recorder := audit.NewRecorder(false) // authEnabled=false (Walking Skeleton)
	return workflow.NewTxCoordinator(s, recorder)
}

// TestExecuteWorkflowCreate_Success_BothCommitted — 정상 경로: workflow + audit 모두 commit
func TestExecuteWorkflowCreate_Success_BothCommitted(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	coord := newTestCoordinator(s)
	ctx := context.Background()

	wf := &types.Workflow{
		State: types.WorkflowStatePending,
	}

	// RED: errNotImplemented 반환으로 실패 예정
	err := coord.ExecuteWorkflowCreate(ctx, wf, "d-uuid-test", "cli-anonymous")
	require.NoError(t, err)

	// 양쪽 모두 FakeStore에 존재해야 함
	assert.Len(t, s.Workflows, 1, "workflow 행이 commit되어야 함")
	assert.Len(t, s.AuditLogs, 1, "audit 행이 commit되어야 함")
}

// TestExecuteWorkflowCreate_AuditFail_BothRolledBack — audit INSERT 실패 시 workflow도 rollback
// AC-CTRL-UBI-001 Scenario A
func TestExecuteWorkflowCreate_AuditFail_BothRolledBack(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	// FailOnAuditInsert를 BeginTx 결과에 주입하기 위해
	// TxCoordinator가 BeginTx 후 FakeTx를 노출하는 방식 필요
	// → GREEN 단계에서 FakeStore.NextTxFailOnAudit 필드 또는 옵션으로 구현
	s.NextTxFailOnAudit = true

	coord := newTestCoordinator(s)
	ctx := context.Background()

	wf := &types.Workflow{State: types.WorkflowStatePending}

	// RED: errNotImplemented 또는 audit 장애 에러 반환 예정
	err := coord.ExecuteWorkflowCreate(ctx, wf, "d-uuid-test", "cli-anonymous")
	// 에러가 반환되어야 함
	assert.Error(t, err, "audit 실패 시 에러를 반환해야 함")

	// 양쪽 모두 FakeStore에 없어야 함 (원자성)
	assert.Empty(t, s.Workflows, "rollback 후 workflow 행이 없어야 함")
	assert.Empty(t, s.AuditLogs, "rollback 후 audit 행이 없어야 함")
}

// TestExecuteWorkflowCreate_WorkflowFail_BothRolledBack — workflow INSERT 실패 시 양쪽 rollback
// AC-CTRL-UBI-001 Scenario B
func TestExecuteWorkflowCreate_WorkflowFail_BothRolledBack(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	s.NextTxFailOnWorkflow = true

	coord := newTestCoordinator(s)
	ctx := context.Background()

	wf := &types.Workflow{State: types.WorkflowStatePending}

	err := coord.ExecuteWorkflowCreate(ctx, wf, "d-uuid-test", "cli-anonymous")
	assert.Error(t, err, "workflow INSERT 실패 시 에러를 반환해야 함")

	assert.Empty(t, s.Workflows, "rollback 후 workflow 행이 없어야 함")
	assert.Empty(t, s.AuditLogs, "rollback 후 audit 행이 없어야 함")
}

// TestExecuteWorkflowTransition_Success_BothCommitted — 전이 성공 시 workflow 상태와 audit 모두 commit
// AC-CTRL-UBI-002-B
func TestExecuteWorkflowTransition_Success_BothCommitted(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	// 사전 조건: PENDING 상태 workflow 존재
	// GREEN 단계에서 FakeStore.Workflows에 초기 행 설정 방법 구현 필요
	coord := newTestCoordinator(s)
	ctx := context.Background()

	err := coord.ExecuteWorkflowTransition(
		ctx,
		"wf-uuid-ubi-002b",
		types.WorkflowStatePending,
		types.WorkflowStateRunning,
		"cli-anonymous",
	)
	// RED: errNotImplemented 반환으로 실패 예정
	require.NoError(t, err)

	// audit 행이 WORKFLOW_TRANSITIONED_TO_RUNNING 액션으로 존재해야 함
	require.Len(t, s.AuditLogs, 1)
	assert.Equal(t, audit.ActionWorkflowTransitionedToRunning, s.AuditLogs[0].Action)
}

// TestExecuteWorkflowTransition_AuditFail_BothRolledBack — 전이 중 audit INSERT 실패 → 양쪽 rollback
// AC-CTRL-UBI-001 Scenario C: RUNNING→COMPLETED 전이 중 audit fail → UPDATE도 rollback
func TestExecuteWorkflowTransition_AuditFail_BothRolledBack(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	s.NextTxFailOnAudit = true

	coord := newTestCoordinator(s)
	ctx := context.Background()

	err := coord.ExecuteWorkflowTransition(
		ctx,
		"wf-uuid-ubi-001",
		types.WorkflowStateRunning,
		types.WorkflowStateCompleted,
		"cli-anonymous",
	)
	assert.Error(t, err, "audit 실패 시 에러를 반환해야 함")

	// workflow 상태 변경도 rollback되어야 함
	assert.Empty(t, s.AuditLogs, "rollback 후 audit 행이 없어야 함")
}

// TestExecuteWorkflowCreate_CliAnonymous_Default — 인증 없을 때 cli-anonymous 기본값
// AC-CTRL-UBI-002-C
func TestExecuteWorkflowCreate_CliAnonymous_Default(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	coord := newTestCoordinator(s)
	ctx := context.Background()

	wf := &types.Workflow{State: types.WorkflowStatePending}

	// userID를 빈 문자열로 전달 — cli-anonymous로 대체되어야 함
	err := coord.ExecuteWorkflowCreate(ctx, wf, "d-uuid-test", "")
	require.NoError(t, err)

	require.Len(t, s.AuditLogs, 1)
	// RED: resolveUserID stub이 "" 반환하므로 실패 예정
	assert.Equal(t, audit.DefaultUserID, s.AuditLogs[0].UserID,
		"userID 빈 문자열 시 audit 이벤트에 cli-anonymous가 기록되어야 함")
}

// TestExecuteWorkflowCreate_UserID_FromRequest — user_id가 요청에서 전파
func TestExecuteWorkflowCreate_UserID_FromRequest(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	// authEnabled=true인 recorder로 coordinator 생성
	recorder := audit.NewRecorder(true)
	coord := workflow.NewTxCoordinator(s, recorder)
	ctx := context.Background()

	wf := &types.Workflow{State: types.WorkflowStatePending}
	err := coord.ExecuteWorkflowCreate(ctx, wf, "d-uuid-test", "explicit-user-id")
	require.NoError(t, err)

	require.Len(t, s.AuditLogs, 1)
	assert.Equal(t, "explicit-user-id", s.AuditLogs[0].UserID,
		"요청의 user_id가 audit 이벤트에 그대로 기록되어야 함")
}
