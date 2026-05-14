// state_machine.go — 워크플로우 상태 머신
// Sprint 2 GREEN: Start/Complete/Fail/CurrentState 실제 구현
// TxCoordinator와 통합된 상태 전이 제어 흐름 제공
package workflow

import (
	"context"
	"fmt"
	"sync"

	cperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
	"go.uber.org/zap"
)

// Status 워크플로우 실행 상태 (Sprint 0 호환성 유지, types.WorkflowState와 병행)
// TODO(Sprint 2): types.WorkflowState로 단일화 후 이 타입은 제거
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

// WorkflowID 워크플로우 고유 식별자 (Sprint 0 호환)
type WorkflowID string

// DocumentID 문서 고유 식별자 (Sprint 0 호환)
type DocumentID string

// Workflow 워크플로우 상태 엔티티 (Sprint 0 호환)
// Sprint 2 이후에는 types.Workflow를 사용하도록 마이그레이션 예정
//
// @MX:TODO: [AUTO] Sprint 3에서 types.Workflow로 단일화 필요
type Workflow struct {
	ID         WorkflowID `json:"id"`
	UserID     string     `json:"user_id"`
	DocumentID DocumentID `json:"document_id"`
	Status     Status     `json:"status"`
	ResultJSON []byte     `json:"result_json,omitempty"`
}

// StateMachine 워크플로우 상태 전이 관리자
// TxCoordinator를 통해 상태 변경과 감사 이벤트를 원자적으로 처리
//
// @MX:ANCHOR: [AUTO] REQ-CTRL-001 상태 머신의 단일 진입점
// @MX:REASON: gRPC 핸들러, REST 핸들러, callback 핸들러 3곳에서 호출 예정 (Sprint 4+5)
type StateMachine struct {
	// coordinator 트랜잭션 원자성 코디네이터 (Sprint 1 GREEN 산출물)
	coordinator *TxCoordinator
	// logger 구조화 zap 로거
	logger *zap.Logger
	// mu 동시 전이 직렬화 — AC-CTRL-001-4 concurrent test 요건
	// GetWorkflow → 검증 → Commit 사이의 TOCTOU 레이스를 방지
	// 포인터 필드(16B) 뒤에 배치하여 fieldalignment 패딩 최소화
	mu sync.Mutex
}

// NewStateMachine StateMachine 인스턴스를 생성
// coordinator: TxCoordinator (Sprint 1 산출물), logger: zap.Logger
func NewStateMachine(coordinator *TxCoordinator, logger *zap.Logger) *StateMachine {
	return &StateMachine{
		coordinator: coordinator,
		logger:      logger,
	}
}

// Coordinator 내부 TxCoordinator를 반환 (Sprint 4 gRPC 핸들러에서 ExecuteWorkflowCreate 호출 시 사용)
func (sm *StateMachine) Coordinator() *TxCoordinator {
	return sm.coordinator
}

// Start PENDING → RUNNING 상태 전이를 수행
// 성공 조건: 현재 상태가 PENDING이고 IsValidTransition(PENDING, RUNNING)이 true
// 실패 조건: 현재 상태가 PENDING이 아니면 ErrInvalidTransition 반환
// 전이 성공 시: UpdateWorkflowState + WORKFLOW_TRANSITIONED_TO_RUNNING audit INSERT → Commit
//
// @MX:WARN: [AUTO] mu.Lock()으로 전체 전이를 직렬화하여 AC-CTRL-001-4 동시성 불변 보장
// @MX:REASON: FakeTx.GetWorkflow와 Commit 사이의 TOCTOU 레이스 방지
func (sm *StateMachine) Start(ctx context.Context, workflowID string) error {
	// 동시 호출 직렬화 — FakeTx(및 실제 pgx)에서 TOCTOU 레이스 방지
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tx, err := sm.coordinator.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("state machine start: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck

	// 현재 상태 조회
	wf, err := tx.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("state machine start: get workflow: %w", err)
	}

	// 전이 유효성 검사
	if !types.IsValidTransition(wf.State, types.WorkflowStateRunning) {
		return fmt.Errorf("state machine start: %w", cperrors.ErrInvalidTransition)
	}

	// 상태 갱신
	if err = tx.UpdateWorkflowState(ctx, workflowID, types.WorkflowStateRunning); err != nil {
		return fmt.Errorf("state machine start: update state: %w", err)
	}

	// 감사 이벤트 기록
	if err = sm.coordinator.recorder.RecordTransitionedToRunning(ctx, tx, workflowID, ""); err != nil {
		return fmt.Errorf("state machine start: record audit: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("state machine start: commit: %w", err)
	}
	return nil
}

// Complete RUNNING → COMPLETED 상태 전이를 수행
// resultJSON: Python worker 반환 결과 (JSONB 형태로 workflows 테이블에 저장)
// 실패 조건: 현재 상태가 RUNNING이 아니면 ErrInvalidTransition 반환
func (sm *StateMachine) Complete(ctx context.Context, workflowID string, resultJSON []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tx, err := sm.coordinator.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("state machine complete: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck

	wf, err := tx.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("state machine complete: get workflow: %w", err)
	}

	if !types.IsValidTransition(wf.State, types.WorkflowStateCompleted) {
		return fmt.Errorf("state machine complete: %w", cperrors.ErrInvalidTransition)
	}

	if err = tx.UpdateWorkflowState(ctx, workflowID, types.WorkflowStateCompleted); err != nil {
		return fmt.Errorf("state machine complete: update state: %w", err)
	}

	if err = tx.UpdateWorkflowResult(ctx, workflowID, resultJSON); err != nil {
		return fmt.Errorf("state machine complete: update result: %w", err)
	}

	if err = sm.coordinator.recorder.RecordCompleted(ctx, tx, workflowID, ""); err != nil {
		return fmt.Errorf("state machine complete: record audit: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("state machine complete: commit: %w", err)
	}
	return nil
}

// Fail PENDING 또는 RUNNING → FAILED 상태 전이를 수행
// reason: 실패 사유 문자열 (audit details에 포함)
// 실패 조건: 이미 종료 상태(COMPLETED, FAILED)이면 ErrInvalidTransition 반환
func (sm *StateMachine) Fail(ctx context.Context, workflowID string, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tx, err := sm.coordinator.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("state machine fail: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck

	wf, err := tx.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("state machine fail: get workflow: %w", err)
	}

	if !types.IsValidTransition(wf.State, types.WorkflowStateFailed) {
		return fmt.Errorf("state machine fail: %w", cperrors.ErrInvalidTransition)
	}

	if err = tx.UpdateWorkflowState(ctx, workflowID, types.WorkflowStateFailed); err != nil {
		return fmt.Errorf("state machine fail: update state: %w", err)
	}

	// 현재 상태에 따라 감사 액션 분기
	// PENDING → FAILED: dispatch 실패, RUNNING → FAILED: callback 실패
	switch wf.State {
	case types.WorkflowStatePending:
		if err = sm.coordinator.recorder.RecordFailedDispatch(ctx, tx, workflowID, ""); err != nil {
			return fmt.Errorf("state machine fail: record dispatch audit: %w", err)
		}
	default:
		// WorkflowStateRunning → FAILED
		if err = sm.coordinator.recorder.RecordFailedCallback(ctx, tx, workflowID, reason, ""); err != nil {
			return fmt.Errorf("state machine fail: record callback audit: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("state machine fail: commit: %w", err)
	}
	return nil
}

// CurrentState 현재 워크플로우 상태를 조회
// 워크플로우가 없으면 에러를 반환
// Sprint 3에서 SELECT FOR UPDATE로 교체될 예정이나 Sprint 2에서는 단순 조회
func (sm *StateMachine) CurrentState(ctx context.Context, workflowID string) (Status, error) {
	tx, err := sm.coordinator.store.BeginTx(ctx)
	if err != nil {
		return "", fmt.Errorf("state machine current state: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck

	wf, err := tx.GetWorkflow(ctx, workflowID)
	if err != nil {
		return "", fmt.Errorf("state machine current state: get workflow: %w", err)
	}

	// types.WorkflowState → Status 변환 (Sprint 0 호환 타입)
	return Status(wf.State), nil
}

// Transition Sprint 0 호환 메서드 — 테스트에서 types 레이어 없이 사용하는 경우 대비
// Sprint 2 이후 deprecated: Start/Complete/Fail 메서드를 사용할 것
//
// @MX:TODO: [AUTO] Sprint 3에서 제거 예정
func (sm *StateMachine) Transition(_ *Workflow, _ Status) error {
	return nil
}
