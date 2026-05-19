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

	// ActionEvidenceCreated 신규 증빙(version=1) 생성 시 기록
	// SPEC-AX-EVID-001 REQ-EVID-003-E1: 증빙 생성과 동일 TX에 audit_logs 1건
	ActionEvidenceCreated Action = "EVIDENCE_CREATED"
	// ActionEvidenceVersioned 기존 증빙 재업로드(version+1) 시 기록
	// SPEC-AX-EVID-001 REQ-EVID-003-E1: 버전 이벤트와 동일 TX에 audit_logs 1건
	ActionEvidenceVersioned Action = "EVIDENCE_VERSIONED"

	// ActionEvalItemCreated 평가항목(루트/자식) 생성 시 기록
	// SPEC-AX-EVAL-ITEM-001 REQ-EVALITEM-003-E1: 항목 생성과 동일 TX에 audit_logs 1건
	ActionEvalItemCreated Action = "EVAL_ITEM_CREATED"
	// ActionEvalItemUpdated 평가항목 속성/상태 변경 시 기록
	// SPEC-AX-EVAL-ITEM-001 REQ-EVALITEM-003-E1 / REQ-EVALITEM-004-O1: 수정과 동일 TX에 audit_logs 1건
	ActionEvalItemUpdated Action = "EVAL_ITEM_UPDATED"

	// ActionScoreCreated 점수 행 생성 시 기록 (SPEC-AX-SCORE-001 REQ-SCORE-004)
	// D2: resource_id = scores.id UUID 직접 대입 (AUD-1 surrogate 미사용)
	ActionScoreCreated Action = "SCORE_CREATED"
	// ActionScoreUpdated 점수 행 수정 시 기록 (SPEC-AX-SCORE-001 REQ-SCORE-004)
	// D2: resource_id = scores.id UUID 직접 대입 (AUD-1 surrogate 미사용)
	ActionScoreUpdated Action = "SCORE_UPDATED"
)

// EvalItemAuditNamespace 평가항목 감사 resource_id surrogate 생성용 고정 UUID namespace.
// SPEC-AX-EVAL-ITEM-001 §6.6 AUD-1 (Human Gate Decision Point 2 확정):
// evaluation_items.id는 VARCHAR(64) 계층코드라 uuid.UUID 컬럼(audit_logs.resource_id NOT NULL)에
// 직접 들어갈 수 없다. RecordEvalItem*는 hierarchy_code를 이 고정 namespace 기반
// 결정적 UUIDv5(uuid.NewSHA1)로 변환해 resource_id에 저장한다.
//
// [HARD] SEC-05 / TH-12: 반드시 컴파일 타임 고정 리터럴이어야 한다. uuid.New() 런타임 생성·
// 환경변수·설정 파일 유래 금지 — namespace가 가변이면 동일 hierarchy_code의 과거/신규
// audit 행 상관관계가 단절되어 감사 추적성이 붕괴한다.
//
// @MX:ANCHOR: [AUTO] RecordEvalItemCreated/RecordEvalItemUpdated가 공유하는 결정적 surrogate namespace
// @MX:REASON: AUD-1 불변식 — 이 값이 바뀌면 모든 평가항목 audit resource_id 결정성이 깨진다 (SEC-05, TH-12)
var EvalItemAuditNamespace = uuid.MustParse("a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b")

// Event 감사 로그 이벤트 엔티티
// 필드 순서: 슬라이스(24바이트) → 시간(24바이트) → UUID(16바이트) → 문자열들
// @MX:NOTE: [AUTO] audit_logs INSERT는 store-layer Tx에 구현됨 (PgWorkflowTx/PgEvidenceTx/
// PgEvalItemTx.InsertAuditLog) — 항목 쓰기와 동일 pgx TX에 atomic 1건 (CTRL-001/EVID-001 운영 중)
type Event struct {
	Timestamp    time.Time `json:"timestamp"`
	Action       Action    `json:"action"`
	ResourceType string    `json:"resource_type"`
	UserID       string    `json:"user_id"`
	DetailsJSON  []byte    `json:"details_json,omitempty"`
	ResourceID   uuid.UUID `json:"resource_id"`
}
