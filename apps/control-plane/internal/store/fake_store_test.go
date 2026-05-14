// fake_store_test.go — FakeStore + FakeTx 단위 테스트 (RED phase)
// AC-CTRL-UBI-001: 트랜잭션 원자성 검증 (Scenario A/B/C)
// 모든 테스트는 GREEN 단계 구현 전까지 FAIL 상태여야 함
package store_test

import (
	"context"
	"testing"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestFakeStore_BeginTx_ReturnsNewTx — BeginTx가 에러 없이 WorkflowTx를 반환해야 함
func TestFakeStore_BeginTx_ReturnsNewTx(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	tx, err := s.BeginTx(ctx)

	// RED: errFakeNotImplemented 반환으로 실패 예정
	require.NoError(t, err)
	assert.NotNil(t, tx)
}

// TestFakeStore_InsertWorkflow_Success — InsertWorkflow가 정상 처리되어야 함
func TestFakeStore_InsertWorkflow_Success(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	tx, err := s.BeginTx(ctx)
	require.NoError(t, err, "BeginTx 실패")

	wf := &types.Workflow{
		State: types.WorkflowStatePending,
	}
	err = tx.InsertWorkflow(ctx, wf)

	// RED: errFakeNotImplemented 반환으로 실패 예정
	assert.NoError(t, err)
}

// TestFakeStore_InsertAuditLog_Success — InsertAuditLog가 정상 처리되어야 함
func TestFakeStore_InsertAuditLog_Success(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	tx, err := s.BeginTx(ctx)
	require.NoError(t, err, "BeginTx 실패")

	ev := &audit.Event{
		Action:       audit.ActionWorkflowCreated,
		ResourceType: "workflow",
		UserID:       "cli-anonymous",
	}
	err = tx.InsertAuditLog(ctx, ev)

	// RED: errFakeNotImplemented 반환으로 실패 예정
	assert.NoError(t, err)
}

// TestFakeStore_Tx_Commit_PersistsBoth — Commit 후 workflow와 audit 양쪽이 FakeStore에 반영되어야 함
func TestFakeStore_Tx_Commit_PersistsBoth(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	tx, err := s.BeginTx(ctx)
	require.NoError(t, err, "BeginTx 실패")

	wf := &types.Workflow{
		State: types.WorkflowStatePending,
	}
	require.NoError(t, tx.InsertWorkflow(ctx, wf))

	ev := &audit.Event{
		Action:       audit.ActionWorkflowCreated,
		ResourceType: "workflow",
		UserID:       "cli-anonymous",
	}
	require.NoError(t, tx.InsertAuditLog(ctx, ev))

	// RED: Commit도 errFakeNotImplemented 반환 예정
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// commit 후 FakeStore에 반영 확인
	assert.Len(t, s.Workflows, 1)
	assert.Len(t, s.AuditLogs, 1)
}

// TestFakeStore_Tx_Rollback_RemovesBoth — Rollback 후 양쪽 행이 FakeStore에 없어야 함
// AC-CTRL-UBI-001 핵심 검증
func TestFakeStore_Tx_Rollback_RemovesBoth(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	tx, err := s.BeginTx(ctx)
	require.NoError(t, err, "BeginTx 실패")

	wf := &types.Workflow{
		State: types.WorkflowStatePending,
	}
	require.NoError(t, tx.InsertWorkflow(ctx, wf))

	ev := &audit.Event{
		Action: audit.ActionWorkflowCreated,
		UserID: "cli-anonymous",
	}
	require.NoError(t, tx.InsertAuditLog(ctx, ev))

	// Rollback 호출
	err = tx.Rollback(ctx)
	// RED: errFakeNotImplemented 반환으로 실패 예정
	require.NoError(t, err)

	// rollback 후 FakeStore에 아무것도 없어야 함
	assert.Empty(t, s.Workflows, "rollback 후 workflow 행이 없어야 함")
	assert.Empty(t, s.AuditLogs, "rollback 후 audit 행이 없어야 함")
}

// TestFakeTx_FailOnAuditInsert_RollsBack — audit INSERT 실패 시 양쪽 다 rollback
// AC-CTRL-UBI-001 Scenario A: workflow INSERT 성공 → audit INSERT 실패 → 양쪽 rollback
func TestFakeTx_FailOnAuditInsert_RollsBack(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	// BeginTx에서 FailOnAuditInsert=true인 FakeTx를 반환해야 함
	// GREEN 단계에서 BeginTx에 옵션 전달 또는 FakeTx 직접 생성으로 구현
	tx, err := s.BeginTx(ctx)
	require.NoError(t, err, "BeginTx 실패")

	// FailOnAuditInsert 주입 — FakeTx 타입 단언 필요 (GREEN에서 구현)
	fakeTx, ok := tx.(*store.FakeTx)
	require.True(t, ok, "BeginTx는 *store.FakeTx를 반환해야 함")
	fakeTx.FailOnAuditInsert = true

	wf := &types.Workflow{State: types.WorkflowStatePending}
	require.NoError(t, tx.InsertWorkflow(ctx, wf), "workflow INSERT는 성공해야 함")

	ev := &audit.Event{Action: audit.ActionWorkflowCreated, UserID: "cli-anonymous"}
	auditErr := tx.InsertAuditLog(ctx, ev)
	// audit INSERT는 실패해야 함
	assert.Error(t, auditErr)

	// 핸들러는 audit 실패 시 Rollback을 호출해야 함
	rollbackErr := tx.Rollback(ctx)
	assert.NoError(t, rollbackErr)

	// 양쪽 모두 FakeStore에 없어야 함 (원자성 검증)
	assert.Empty(t, s.Workflows, "audit 실패로 rollback 시 workflow 행도 없어야 함")
	assert.Empty(t, s.AuditLogs, "audit INSERT가 실패했으므로 audit 행도 없어야 함")
}

// TestFakeTx_FailOnWorkflowInsert_RollsBack — workflow INSERT 실패 시 양쪽 다 rollback
// AC-CTRL-UBI-001 Scenario B: audit INSERT 성공 → workflow INSERT 실패 → 양쪽 rollback
func TestFakeTx_FailOnWorkflowInsert_RollsBack(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	tx, err := s.BeginTx(ctx)
	require.NoError(t, err, "BeginTx 실패")

	fakeTx, ok := tx.(*store.FakeTx)
	require.True(t, ok, "BeginTx는 *store.FakeTx를 반환해야 함")
	fakeTx.FailOnWorkflowInsert = true

	wf := &types.Workflow{State: types.WorkflowStatePending}
	workflowErr := tx.InsertWorkflow(ctx, wf)
	// workflow INSERT는 실패해야 함
	assert.Error(t, workflowErr)

	rollbackErr := tx.Rollback(ctx)
	assert.NoError(t, rollbackErr)

	// 양쪽 모두 FakeStore에 없어야 함
	assert.Empty(t, s.Workflows, "workflow 실패로 rollback 시 workflow 행 없어야 함")
	assert.Empty(t, s.AuditLogs, "rollback 시 audit 행도 없어야 함")
}

// TestFakeStore_MultiTx_Independent — 두 Tx가 서로 독립적으로 동작해야 함
func TestFakeStore_MultiTx_Independent(t *testing.T) {
	t.Parallel()
	s := store.NewFakeStore()
	ctx := context.Background()

	tx1, err := s.BeginTx(ctx)
	require.NoError(t, err)
	tx2, err := s.BeginTx(ctx)
	require.NoError(t, err)

	wf1 := &types.Workflow{State: types.WorkflowStatePending}
	wf2 := &types.Workflow{State: types.WorkflowStatePending}

	require.NoError(t, tx1.InsertWorkflow(ctx, wf1))
	require.NoError(t, tx2.InsertWorkflow(ctx, wf2))

	// tx1은 commit, tx2는 rollback
	require.NoError(t, tx1.Commit(ctx))
	require.NoError(t, tx2.Rollback(ctx))

	// FakeStore에 tx1의 행만 존재해야 함
	// RED: 현재 Commit/Rollback이 미구현이므로 실패 예정
	assert.Len(t, s.Workflows, 1, "commit된 tx1의 행만 존재해야 함")
}
