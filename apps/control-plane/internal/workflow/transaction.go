// transaction.go — 워크플로우 생성/전이의 트랜잭션 원자성 코디네이터
// Sprint 1 RED: 함수 시그니처만 정의, 실제 동작은 GREEN에서 구현
// REQ-CTRL-UBI-001: 워크플로우 변경과 감사 로그 삽입은 반드시 동일 tx 안에서 atomic하게 수행
package workflow

import (
	"context"
	"errors"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// errTxNotImplemented 미구현 경로에서 반환하는 sentinel 에러
var errTxNotImplemented = errors.New("not implemented")

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
//
// @MX:TODO - Sprint 1 GREEN: 구현 완성
func NewTxCoordinator(s store.WorkflowStore, r *audit.Recorder) *TxCoordinator {
	return &TxCoordinator{store: s, recorder: r}
}

// ExecuteWorkflowCreate 워크플로우 생성을 트랜잭션으로 실행
// 성공: workflow INSERT + WORKFLOW_CREATED audit INSERT → Commit
// 실패: 어느 한쪽이라도 실패하면 tx.Rollback → 양쪽 모두 영속화되지 않음
// (AC-CTRL-UBI-001 Scenario A/B, AC-CTRL-UBI-002-A, AC-CTRL-UBI-002-C)
//
// @MX:TODO - Sprint 1 GREEN: BeginTx → InsertWorkflow → RecordCreated → Commit 구현
func (c *TxCoordinator) ExecuteWorkflowCreate(
	ctx context.Context,
	w *types.Workflow,
	documentID string,
	userID string,
) error {
	_, _ = ctx, w
	_, _ = documentID, userID
	return errTxNotImplemented
}

// ExecuteWorkflowTransition 워크플로우 상태 전이를 트랜잭션으로 실행
// 성공: UpdateWorkflowState + transition audit INSERT → Commit
// 실패: 어느 한쪽이라도 실패하면 tx.Rollback → 상태 변경 없음
// (AC-CTRL-UBI-001 Scenario C, AC-CTRL-UBI-002-B)
//
// @MX:TODO - Sprint 1 GREEN: BeginTx → UpdateWorkflowState → RecordTransition → Commit 구현
func (c *TxCoordinator) ExecuteWorkflowTransition(
	ctx context.Context,
	workflowID string,
	from, to types.WorkflowState,
	userID string,
) error {
	_, _ = ctx, workflowID
	_, _ = from, to
	_ = userID
	return errTxNotImplemented
}
