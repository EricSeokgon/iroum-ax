// recorder.go — 감사 이벤트 기록 헬퍼
// Sprint 1 GREEN: Recorder 메서드 실제 동작 구현
// Recorder는 Store.Tx를 통해 audit_logs 테이블에 INSERT를 수행하며,
// 호출자가 제공하는 Tx 위에서 실행됨으로써 트랜잭션 원자성(REQ-CTRL-UBI-001)을 보장
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DefaultUserID 인증이 비활성화된 경우 감사 로그에 기록되는 기본 사용자 식별자
// SPEC-AX-001 REQ-UBI-003 State-driven 절 및 REQ-CTRL-UBI-002와 동일
const DefaultUserID = "cli-anonymous"

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
// @MX:ANCHOR: [AUTO] 8종 감사 액션 기록의 단일 진입점 (fan_in: gRPC 핸들러, 워크플로우 핸들러, callback)
// @MX:REASON: REQ-CTRL-UBI-002 AC-UBI-002-A/B/C 모두 이 Recorder를 통해 검증
type Recorder struct {
	// authEnabled false이면 user_id를 DefaultUserID로 강제
	// Walking Skeleton 기본값: false (SPEC §5 Exclusion §2)
	authEnabled bool
}

// NewRecorder 새 Recorder 인스턴스를 생성
// authEnabled: 인증 활성화 여부 (config.Config.AuthEnabled 값 전달)
func NewRecorder(authEnabled bool) *Recorder {
	return &Recorder{authEnabled: authEnabled}
}

// resolveUserID 요청에서 전달된 userID를 반환하거나,
// 인증이 비활성화되어 있거나 userID가 비어 있으면 DefaultUserID를 반환
func (r *Recorder) resolveUserID(userID string) string {
	if userID == "" || !r.authEnabled {
		return DefaultUserID
	}
	return userID
}

// parseResourceID workflowID 문자열을 uuid.UUID로 변환
// 파싱 실패 시 uuid.Nil을 반환 (런타임 검증은 상위 계층 책임)
func parseResourceID(workflowID string) uuid.UUID {
	id, err := uuid.Parse(workflowID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// RecordCreated WORKFLOW_CREATED 감사 이벤트를 기록
// AC-CTRL-UBI-002-A: action='WORKFLOW_CREATED', resource_type='workflow', user_id 전파
func (r *Recorder) RecordCreated(ctx context.Context, tx AuditTx, workflowID, documentID, userID string) error {
	details, err := json.Marshal(map[string]string{
		"document_id": documentID,
	})
	if err != nil {
		return fmt.Errorf("recorder: marshal details: %w", err)
	}

	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       ActionWorkflowCreated,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordTransition 상태 전이 감사 이벤트를 기록
// AC-CTRL-UBI-002-B: action='WORKFLOW_TRANSITIONED_TO_RUNNING',
// details JSONB = {"from":"PENDING","to":"RUNNING"}
func (r *Recorder) RecordTransition(ctx context.Context, tx AuditTx, workflowID string, from, to Action, userID string) error {
	details, err := json.Marshal(map[string]string{
		"from": string(from),
		"to":   string(to),
	})
	if err != nil {
		return fmt.Errorf("recorder: marshal transition details: %w", err)
	}

	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       to,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordCompleted WORKFLOW_COMPLETED 감사 이벤트를 기록
func (r *Recorder) RecordCompleted(ctx context.Context, tx AuditTx, workflowID, userID string) error {
	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       ActionWorkflowCompleted,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordFailedDispatch WORKFLOW_FAILED_DISPATCH 감사 이벤트를 기록
func (r *Recorder) RecordFailedDispatch(ctx context.Context, tx AuditTx, workflowID, userID string) error {
	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       ActionWorkflowFailedDispatch,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordTransitionRejected TRANSITION_REJECTED 감사 이벤트를 기록
// details JSONB = {"from":"...", "to":"...", "reason":"..."}
func (r *Recorder) RecordTransitionRejected(ctx context.Context, tx AuditTx, workflowID string, from, to Action, reason, userID string) error {
	details, err := json.Marshal(map[string]string{
		"from":   string(from),
		"to":     string(to),
		"reason": reason,
	})
	if err != nil {
		return fmt.Errorf("recorder: marshal rejection details: %w", err)
	}

	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       ActionTransitionRejected,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordTransitionedToRunning WORKFLOW_TRANSITIONED_TO_RUNNING 감사 이벤트를 기록
// AC-CTRL-001-1: PENDING → RUNNING 전이 성공 시 호출
func (r *Recorder) RecordTransitionedToRunning(ctx context.Context, tx AuditTx, workflowID, userID string) error {
	details, err := json.Marshal(map[string]string{
		"from": "PENDING",
		"to":   "RUNNING",
	})
	if err != nil {
		return fmt.Errorf("recorder: marshal transition details: %w", err)
	}
	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       ActionWorkflowTransitionedToRunning,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordFailedCallback WORKFLOW_FAILED_CALLBACK 감사 이벤트를 기록
// RUNNING → FAILED 전이 시 호출 (콜백 처리 실패)
func (r *Recorder) RecordFailedCallback(ctx context.Context, tx AuditTx, workflowID, reason, userID string) error {
	details, err := json.Marshal(map[string]string{
		"reason": reason,
	})
	if err != nil {
		return fmt.Errorf("recorder: marshal callback failure details: %w", err)
	}
	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       ActionWorkflowFailedCallback,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordCreateCancelled WORKFLOW_CREATE_CANCELLED 감사 이벤트를 기록
func (r *Recorder) RecordCreateCancelled(ctx context.Context, tx AuditTx, workflowID, userID string) error {
	e := &Event{
		Timestamp:    time.Now().UTC(),
		Action:       ActionWorkflowCreateCancelled,
		ResourceType: "workflow",
		ResourceID:   parseResourceID(workflowID),
		UserID:       r.resolveUserID(userID),
	}
	return tx.InsertAuditLog(ctx, e)
}
