// 워크플로우 상태 머신
// Sprint 0 스켈레톤 — 상태 전이 로직은 Sprint 7에서 구현
package workflow

// Status 워크플로우 실행 상태
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

// WorkflowID 워크플로우 고유 식별자
type WorkflowID string

// DocumentID 문서 고유 식별자
type DocumentID string

// Workflow 워크플로우 상태 엔티티
// @MX:TODO - Sprint 7에서 PostgreSQL store 연동 및 상태 전이 구현
type Workflow struct {
	ID         WorkflowID `json:"id"`
	UserID     string     `json:"user_id"` // 기본값: "cli-anonymous"
	DocumentID DocumentID `json:"document_id"`
	Status     Status     `json:"status"`
	ResultJSON []byte     `json:"result_json,omitempty"`
}

// StateMachine 워크플로우 상태 전이 관리자
// @MX:TODO - Sprint 7에서 PENDING→RUNNING→COMPLETED/FAILED 전이 구현
type StateMachine struct{}

// NewStateMachine StateMachine 인스턴스 생성
func NewStateMachine() *StateMachine {
	return &StateMachine{}
}

// Transition 상태 전이 수행 (stub)
// TODO(Sprint 7): 실제 전이 유효성 검사 및 PostgreSQL 갱신 구현
func (sm *StateMachine) Transition(_ *Workflow, _ Status) error {
	return nil
}
