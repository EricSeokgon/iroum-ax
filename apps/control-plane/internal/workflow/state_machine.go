// state_machine.go — 워크플로우 상태 머신
// Sprint 2 RED: Start/Complete/Fail/CurrentState 시그니처 정의, 구현은 GREEN 단계
// Sprint 0 stub을 대체하며 TxCoordinator와 통합된 상태 전이 제어 흐름을 제공
package workflow

import (
	"context"
	"errors"

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

// errStateMachineNotImplemented Sprint 2 GREEN 이전 모든 StateMachine 메서드가 반환하는 sentinel
// RED phase에서 테스트가 실패해야 하므로 이 에러로 실패를 유도
//
// @MX:TODO: [AUTO] Start/Complete/Fail/CurrentState GREEN 구현 시 이 에러 제거
var errStateMachineNotImplemented = errors.New("state machine: not implemented — sprint 2 green phase required")

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
}

// NewStateMachine StateMachine 인스턴스를 생성
// coordinator: TxCoordinator (Sprint 1 산출물), logger: zap.Logger
func NewStateMachine(coordinator *TxCoordinator, logger *zap.Logger) *StateMachine {
	return &StateMachine{
		coordinator: coordinator,
		logger:      logger,
	}
}

// Start PENDING → RUNNING 상태 전이를 수행
// 성공 조건: 현재 상태가 PENDING이고 IsValidTransition(PENDING, RUNNING)이 true
// 실패 조건: 현재 상태가 PENDING이 아니면 ErrInvalidTransition 반환
// 전이 성공 시: UpdateWorkflowState + WORKFLOW_TRANSITIONED_TO_RUNNING audit INSERT → Commit
//
// @MX:TODO: [AUTO] Sprint 2 GREEN에서 구현 필요
func (sm *StateMachine) Start(ctx context.Context, workflowID string) error {
	return errStateMachineNotImplemented
}

// Complete RUNNING → COMPLETED 상태 전이를 수행
// resultJSON: Python worker 반환 결과 (JSONB 형태로 workflows 테이블에 저장)
// 실패 조건: 현재 상태가 RUNNING이 아니면 ErrInvalidTransition 반환
//
// @MX:TODO: [AUTO] Sprint 2 GREEN에서 구현 필요
func (sm *StateMachine) Complete(ctx context.Context, workflowID string, resultJSON []byte) error {
	return errStateMachineNotImplemented
}

// Fail PENDING 또는 RUNNING → FAILED 상태 전이를 수행
// reason: 실패 사유 문자열 (audit details에 포함)
// 실패 조건: 이미 종료 상태(COMPLETED, FAILED)이면 ErrInvalidTransition 반환
//
// @MX:TODO: [AUTO] Sprint 2 GREEN에서 구현 필요
func (sm *StateMachine) Fail(ctx context.Context, workflowID string, reason string) error {
	return errStateMachineNotImplemented
}

// CurrentState 현재 워크플로우 상태를 조회
// 워크플로우가 없으면 ErrWorkflowNotFound를 래핑하여 반환
// Sprint 3에서 SELECT FOR UPDATE로 교체될 예정이나 Sprint 2에서는 단순 조회
//
// @MX:TODO: [AUTO] Sprint 2 GREEN에서 구현 필요, Sprint 3에서 SELECT FOR UPDATE로 전환
func (sm *StateMachine) CurrentState(ctx context.Context, workflowID string) (Status, error) {
	return "", errStateMachineNotImplemented
}

// Transition Sprint 0 호환 메서드 — 테스트에서 types 레이어 없이 사용하는 경우 대비
// Sprint 2 이후 deprecated: Start/Complete/Fail 메서드를 사용할 것
//
// @MX:TODO: [AUTO] Sprint 3에서 제거 예정
func (sm *StateMachine) Transition(_ *Workflow, _ Status) error {
	return nil
}
