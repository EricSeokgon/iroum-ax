// recorder.go — 감사 이벤트 기록 헬퍼
// Sprint 1 RED: Recorder 타입과 메서드 시그니처만 정의, 실제 동작은 GREEN에서 구현
// Recorder는 Store.Tx를 통해 audit_logs 테이블에 INSERT를 수행하며,
// 호출자가 제공하는 Tx 위에서 실행됨으로써 트랜잭션 원자성(REQ-CTRL-UBI-001)을 보장
package audit

import (
	"context"
	"errors"
)

// DefaultUserID 인증이 비활성화된 경우 감사 로그에 기록되는 기본 사용자 식별자
// SPEC-AX-001 REQ-UBI-003 State-driven 절 및 REQ-CTRL-UBI-002와 동일
const DefaultUserID = "cli-anonymous"

// errNotImplemented 미구현 메서드 sentinel 에러
var errNotImplemented = errors.New("not implemented")

// ErrAuditCaptureFail 테스트용 captureTx에서 InsertAuditLog 장애 주입 시 반환
var ErrAuditCaptureFail = errors.New("audit capture injected failure")

// AuditTx Recorder가 감사 이벤트를 삽입하는 데 사용하는 트랜잭션 인터페이스
// store.Tx의 InsertAuditLog 서브셋으로, 의존 역전을 위해 별도 선언
//
// @MX:NOTE: [AUTO] store 패키지 순환 참조 방지를 위해 로컬 인터페이스로 정의
type AuditTx interface {
	// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입
	InsertAuditLog(ctx context.Context, e *Event) error
}

// Recorder 워크플로우 생명주기 이벤트를 audit_logs 테이블에 기록하는 헬퍼
// 모든 메서드는 호출자 제공 AuditTx 위에서 동작하여 원자성을 보장
//
// @MX:ANCHOR: [AUTO] 8종 감사 액션 기록의 단일 진입점 (fan_in 예정: gRPC 핸들러, 워크플로우 핸들러, callback)
// @MX:REASON: REQ-CTRL-UBI-002 AC-UBI-002-A/B/C 모두 이 Recorder를 통해 검증
type Recorder struct {
	// authEnabled false이면 user_id를 DefaultUserID로 강제
	// Walking Skeleton 기본값: false (SPEC §5 Exclusion §2)
	authEnabled bool
}

// NewRecorder 새 Recorder 인스턴스를 생성
// authEnabled: 인증 활성화 여부 (config.Config.AuthEnabled 값 전달)
//
// @MX:TODO - Sprint 1 GREEN: 구현 추가
func NewRecorder(authEnabled bool) *Recorder {
	return &Recorder{authEnabled: authEnabled}
}

// resolveUserID 요청에서 전달된 userID를 반환하거나,
// 인증이 비활성화되어 있거나 userID가 비어 있으면 DefaultUserID를 반환
//
// @MX:TODO - Sprint 1 GREEN: 구현 추가
//
//nolint:unused
func (r *Recorder) resolveUserID(userID string) string {
	_ = userID
	return ""
}

// RecordCreated WORKFLOW_CREATED 감사 이벤트를 기록
// AC-CTRL-UBI-002-A: action='WORKFLOW_CREATED', resource_type='workflow', user_id 전파
//
// @MX:TODO - Sprint 1 GREEN: AuditTx.InsertAuditLog 호출 구현
func (r *Recorder) RecordCreated(ctx context.Context, tx AuditTx, workflowID, documentID, userID string) error {
	_, _ = ctx, tx
	_, _, _ = workflowID, documentID, userID
	return errNotImplemented
}

// RecordTransition 상태 전이 감사 이벤트를 기록
// AC-CTRL-UBI-002-B: action='WORKFLOW_TRANSITIONED_TO_RUNNING',
// details JSONB = {"from":"PENDING","to":"RUNNING"}
//
// @MX:TODO - Sprint 1 GREEN: AuditTx.InsertAuditLog 호출 구현
func (r *Recorder) RecordTransition(ctx context.Context, tx AuditTx, workflowID string, from, to Action, userID string) error {
	_, _ = ctx, tx
	_, _ = workflowID, userID
	_, _ = from, to
	return errNotImplemented
}

// RecordCompleted WORKFLOW_COMPLETED 감사 이벤트를 기록
//
// @MX:TODO - Sprint 1 GREEN: AuditTx.InsertAuditLog 호출 구현
func (r *Recorder) RecordCompleted(ctx context.Context, tx AuditTx, workflowID, userID string) error {
	_, _ = ctx, tx
	_, _ = workflowID, userID
	return errNotImplemented
}

// RecordFailedDispatch WORKFLOW_FAILED_DISPATCH 감사 이벤트를 기록
//
// @MX:TODO - Sprint 1 GREEN: AuditTx.InsertAuditLog 호출 구현
func (r *Recorder) RecordFailedDispatch(ctx context.Context, tx AuditTx, workflowID, userID string) error {
	_, _ = ctx, tx
	_, _ = workflowID, userID
	return errNotImplemented
}

// RecordTransitionRejected TRANSITION_REJECTED 감사 이벤트를 기록
// details JSONB = {"from":"...", "to":"...", "reason":"..."}
//
// @MX:TODO - Sprint 1 GREEN: AuditTx.InsertAuditLog 호출 구현
func (r *Recorder) RecordTransitionRejected(ctx context.Context, tx AuditTx, workflowID string, from, to Action, reason, userID string) error {
	_, _ = ctx, tx
	_, _ = workflowID, userID
	_ = reason
	_, _ = from, to
	return errNotImplemented
}

// RecordCreateCancelled WORKFLOW_CREATE_CANCELLED 감사 이벤트를 기록
//
// @MX:TODO - Sprint 1 GREEN: AuditTx.InsertAuditLog 호출 구현
func (r *Recorder) RecordCreateCancelled(ctx context.Context, tx AuditTx, workflowID, userID string) error {
	_, _ = ctx, tx
	_, _ = workflowID, userID
	return errNotImplemented
}
