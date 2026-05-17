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
	// ActionAuthForbidden RBAC 권한 부족으로 접근 거부
	// REQ-AUTH-004-U1: HTTP 403 / gRPC PERMISSION_DENIED 시 기록
	ActionAuthForbidden Action = "AUTH_FORBIDDEN"
	// ActionAuthLogout 사용자 로그아웃 — access token + refresh token 블랙리스트 등록
	// REQ-AUTH-005-E1: POST /api/v1/auth/logout 성공 시 기록
	ActionAuthLogout Action = "AUTH_LOGOUT"
	// ActionAuthRefreshReuseDetected refresh token family reuse 공격 탐지
	// REQ-AUTH-005-U1: OAuth 2.0 BCP — 이미 사용된 refresh token 재사용 시 family 전체 invalidation 후 기록
	ActionAuthRefreshReuseDetected Action = "AUTH_REFRESH_REUSE_DETECTED"
	// ActionABACDenied ABAC 속성 조건 위반으로 접근 거부
	// SPEC-AX-AUTH-003 REQ-ABAC-007 D5: ABAC 거부 시 HTTP 403 + 거부 사유 기록
	ActionABACDenied Action = "ABAC_CONDITION_DENIED"

	// ActionServerStartup 서버 모든 리스너 바인딩 완료 후 기록
	// REQ-SERVER-UBI-001-a: grpc_addr, rest_addr 기록
	ActionServerStartup Action = "SERVER_STARTUP"
	// ActionServerShutdownInitiated SIGTERM/SIGINT 수신 직후 기록
	// REQ-SERVER-UBI-001-a: signal 수신 시점
	ActionServerShutdownInitiated Action = "SERVER_SHUTDOWN_INITIATED"
	// ActionServerShutdownCompleted 모든 리스너 드레인 + 커넥션 종료 후 기록
	// REQ-SERVER-UBI-001-a: uptime_seconds, exit_reason 기록
	ActionServerShutdownCompleted Action = "SERVER_SHUTDOWN_COMPLETED"
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
