// 감사 이벤트 타입 정의 — Python audit_event 열거형에 대응
// Sprint 0: 타입 선언만 포함, 실제 DB 퍼시스턴스는 Sprint 1 GREEN 단계에서 구현
package audit

import (
	"time"

	"github.com/google/uuid"
)

// Action 감사 이벤트 액션 유형 (문자열 기반 열거형)
// research.md §2.2 에서 정의한 8가지 액션을 정확히 반영
type Action string

const (
	// ActionWorkflowCreated 새 워크플로우 생성
	ActionWorkflowCreated Action = "WORKFLOW_CREATED"
	// ActionWorkflowTransitionedToRunning PENDING → RUNNING 전이 성공
	ActionWorkflowTransitionedToRunning Action = "WORKFLOW_TRANSITIONED_TO_RUNNING"
	// ActionWorkflowCompleted 워크플로우 최종 완료
	ActionWorkflowCompleted Action = "WORKFLOW_COMPLETED"
	// ActionWorkflowFailedDispatch Celery 디스패치 실패로 인한 실패
	ActionWorkflowFailedDispatch Action = "WORKFLOW_FAILED_DISPATCH"
	// ActionWorkflowFailedCallback 콜백 처리 실패로 인한 실패
	ActionWorkflowFailedCallback Action = "WORKFLOW_FAILED_CALLBACK"
	// ActionTransitionRejected 유효하지 않은 상태 전이 거부
	ActionTransitionRejected Action = "TRANSITION_REJECTED"
	// ActionCallbackRejectedTerminal 종료 상태에 대한 콜백 거부
	ActionCallbackRejectedTerminal Action = "CALLBACK_REJECTED_TERMINAL"
	// ActionWorkflowCreateCancelled 워크플로우 생성 요청 취소
	ActionWorkflowCreateCancelled Action = "WORKFLOW_CREATE_CANCELLED"
)

// Event 감사 로그 이벤트 엔티티
// 필드 순서: 슬라이스(24바이트) → 시간(24바이트) → UUID(16바이트) → 문자열들
// @MX:TODO - Sprint 1에서 PostgreSQL audit_logs 테이블에 INSERT 구현
type Event struct {
	Timestamp    time.Time `json:"timestamp"`
	Action       Action    `json:"action"`
	ResourceType string    `json:"resource_type"`
	UserID       string    `json:"user_id"`
	DetailsJSON  []byte    `json:"details_json,omitempty"`
	ResourceID   uuid.UUID `json:"resource_id"`
}
