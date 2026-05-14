// transaction.go — 워크플로우 생성/전이의 트랜잭션 원자성 코디네이터
// Sprint 1 GREEN: BeginTx → 쓰기 → Commit/Rollback 패턴 구현
// REQ-CTRL-UBI-001: 워크플로우 변경과 감사 로그 삽입은 반드시 동일 tx 안에서 atomic하게 수행
package workflow

import (
	"context"
	"fmt"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// TxCoordinator 워크플로우 생성 및 상태 전이를 원자적으로 처리하는 코디네이터
// Store.BeginTx → InsertWorkflow + InsertAuditLog → Commit/Rollback 순서를 강제
//
// @MX:ANCHOR: [AUTO] REQ-CTRL-UBI-001 트랜잭션 원자성의 단일 진입점
// @MX:REASON: gRPC 핸들러, REST 핸들러, callback 핸들러 3곳에서 호출 예정
type TxCoordinator struct {
	store    store.WorkflowStore
	recorder *audit.Recorder
}

// NewTxCoordinator 새 TxCoordinator 인스턴스를 생성
func NewTxCoordinator(s store.WorkflowStore, r *audit.Recorder) *TxCoordinator {
	return &TxCoordinator{store: s, recorder: r}
}

// ExecuteWorkflowCreate 워크플로우 생성을 트랜잭션으로 실행
// 성공: workflow INSERT + WORKFLOW_CREATED audit INSERT → Commit
// 실패: 어느 한쪽이라도 실패하면 tx.Rollback → 양쪽 모두 영속화되지 않음
// (AC-CTRL-UBI-001 Scenario A/B, AC-CTRL-UBI-002-A, AC-CTRL-UBI-002-C)
func (c *TxCoordinator) ExecuteWorkflowCreate(
	ctx context.Context,
	w *types.Workflow,
	documentID string,
	userID string,
) error {
	tx, err := c.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("workflow create: begin tx: %w", err)
	}
	// defer rollback은 commit 이후에는 no-op (FakeTx.Rollback 멱등 보장)
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck

	if err = tx.InsertWorkflow(ctx, w); err != nil {
		return fmt.Errorf("workflow create: insert workflow: %w", err)
	}

	if err = c.recorder.RecordCreated(ctx, tx, w.ID.String(), documentID, userID); err != nil {
		return fmt.Errorf("workflow create: record audit: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("workflow create: commit: %w", err)
	}
	return nil
}

// workflowStateToAction WorkflowState를 감사 Action으로 변환
// to 상태에 따라 적절한 액션 상수를 반환
func workflowStateToAction(state types.WorkflowState) audit.Action {
	switch state {
	case types.WorkflowStateRunning:
		return audit.ActionWorkflowTransitionedToRunning
	case types.WorkflowStateCompleted:
		return audit.ActionWorkflowCompleted
	case types.WorkflowStateFailed:
		return audit.ActionWorkflowFailedDispatch
	default:
		return audit.ActionWorkflowCreated
	}
}

// ExecuteWorkflowTransition 워크플로우 상태 전이를 트랜잭션으로 실행
// 성공: UpdateWorkflowState + transition audit INSERT → Commit
// 실패: 어느 한쪽이라도 실패하면 tx.Rollback → 상태 변경 없음
// (AC-CTRL-UBI-001 Scenario C, AC-CTRL-UBI-002-B)
func (c *TxCoordinator) ExecuteWorkflowTransition(
	ctx context.Context,
	workflowID string,
	from, to types.WorkflowState,
	userID string,
) error {
	tx, err := c.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("workflow transition: begin tx: %w", err)
	}
	// defer rollback은 commit 이후 no-op
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck

	if err = tx.UpdateWorkflowState(ctx, workflowID, to); err != nil {
		return fmt.Errorf("workflow transition: update state: %w", err)
	}

	fromAction := workflowStateToAction(from)
	toAction := workflowStateToAction(to)
	if err = c.recorder.RecordTransition(ctx, tx, workflowID, fromAction, toAction, userID); err != nil {
		return fmt.Errorf("workflow transition: record audit: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("workflow transition: commit: %w", err)
	}
	return nil
}
