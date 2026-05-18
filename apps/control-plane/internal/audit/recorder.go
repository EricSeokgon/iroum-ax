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
	"strconv"
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
	// clock 시각 제공자 — 증빙 감사 경로 테스트 친화 (R-EVID-007/T-018)
	// nil이면 defaultClock(systemClock) 사용 — 기존 time.Now().UTC()와 동일 동작
	clock Clock
	// authEnabled false이면 user_id를 DefaultUserID로 강제
	// Walking Skeleton 기본값: false (SPEC §5 Exclusion §2)
	authEnabled bool
}

// RecorderOption Recorder 생성 시 선택적 의존성 주입 (기능 옵션 패턴)
type RecorderOption func(*Recorder)

// WithClock 테스트에서 고정 시각 Clock을 주입 (R-EVID-007 — 증빙 감사 시각 검증)
func WithClock(c Clock) RecorderOption {
	return func(r *Recorder) { r.clock = c }
}

// NewRecorder 새 Recorder 인스턴스를 생성
// authEnabled: 인증 활성화 여부 (config.Config.AuthEnabled 값 전달)
// opts: 선택적 의존성 (WithClock 등) — 미지정 시 기존 동작과 byte-identical
func NewRecorder(authEnabled bool, opts ...RecorderOption) *Recorder {
	r := &Recorder{authEnabled: authEnabled, clock: defaultClock}
	for _, o := range opts {
		o(r)
	}
	if r.clock == nil {
		r.clock = defaultClock
	}
	return r
}

// nowUTC Recorder의 주입된 Clock에서 현재 UTC 시각을 반환
// 증빙 감사 메서드 전용 — 기존 워크플로우 메서드는 직접 time.Now().UTC() 유지 (scope 최소화)
func (r *Recorder) nowUTC() time.Time {
	if r.clock == nil {
		return time.Now().UTC()
	}
	return r.clock.NowUTC()
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

// RecordEvidenceCreated EVIDENCE_CREATED 감사 이벤트를 기록 (SPEC-AX-EVID-001)
// 신규 증빙(version=1) 생성과 동일 AuditTx에 audit_logs 1건 (REQ-EVID-UBI-002 / REQ-EVID-003-E1)
// details JSONB: {evaluation_item_id, version, file_hash_sha256} (previous_version_id 없음)
//
// @MX:ANCHOR: [AUTO] 증빙 생성 감사 단일 진입점 — REQ-EVID-UBI-002-A AC가 이 메서드 경유
// @MX:REASON: 핸들러 + 통합 테스트 + 감사 검증 등 3곳 이상에서 호출 — 증빙 감사 완결성 계약
func (r *Recorder) RecordEvidenceCreated(ctx context.Context, tx AuditTx, evidenceID, evalItemID, fileHashSHA256 string, version int, userID string) error {
	details, err := json.Marshal(map[string]string{
		"evaluation_item_id": evalItemID,
		"version":            strconv.Itoa(version),
		"file_hash_sha256":   fileHashSHA256,
	})
	if err != nil {
		return fmt.Errorf("recorder: marshal evidence created details: %w", err)
	}
	e := &Event{
		Timestamp:    r.nowUTC(),
		Action:       ActionEvidenceCreated,
		ResourceType: "evidence",
		ResourceID:   parseResourceID(evidenceID),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordEvidenceVersioned EVIDENCE_VERSIONED 감사 이벤트를 기록 (SPEC-AX-EVID-001)
// 기존 증빙 재업로드(version+1)와 동일 AuditTx에 audit_logs 1건 (REQ-EVID-UBI-002 / REQ-EVID-003-E1)
// details JSONB: {evaluation_item_id, version, file_hash_sha256, previous_version_id}
//
// @MX:ANCHOR: [AUTO] 증빙 버전 감사 단일 진입점 — REQ-EVID-UBI-002-B AC가 이 메서드 경유
// @MX:REASON: 핸들러 + 통합 테스트 + 감사 검증 등 3곳 이상에서 호출 — 버전 감사 완결성 계약
func (r *Recorder) RecordEvidenceVersioned(ctx context.Context, tx AuditTx, evidenceID, evalItemID, fileHashSHA256 string, version int, previousVersionID, userID string) error {
	details, err := json.Marshal(map[string]string{
		"evaluation_item_id":  evalItemID,
		"version":             strconv.Itoa(version),
		"file_hash_sha256":    fileHashSHA256,
		"previous_version_id": previousVersionID,
	})
	if err != nil {
		return fmt.Errorf("recorder: marshal evidence versioned details: %w", err)
	}
	e := &Event{
		Timestamp:    r.nowUTC(),
		Action:       ActionEvidenceVersioned,
		ResourceType: "evidence",
		ResourceID:   parseResourceID(evidenceID),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// evalItemResourceID 계층코드를 AUD-1 결정적 UUIDv5 surrogate로 변환한다.
// evaluation_items.id는 VARCHAR(64) 계층코드라 audit_logs.resource_id(uuid.UUID NOT NULL)에
// 직접 들어갈 수 없다 (parseResourceID는 uuid.Nil 반환 — E-07 회피). 고정 namespace 기반
// uuid.NewSHA1로 동일 hierarchy_code는 항상 byte-identical UUID를 산출한다 (§6.6 AUD-1).
//
// uuid.NewSHA1은 RFC 4122 v5 정의상 SHA-1을 사용한다 — 식별자 파생이지 암호 해시가 아님.
//
//nolint:gosec // RFC 4122 UUID v5 name-based, not cryptographic (SEC-07)
func evalItemResourceID(hierarchyCode string) uuid.UUID {
	return uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))
}

// evalItemDetails 평가항목 감사 DetailsJSON을 직렬화한다.
// 실 식별자(eval_item_id, hierarchy_code, parent_id?, level?)는 resource_id surrogate가
// 보유할 수 없으므로 details에 기록한다 (REQ-EVALITEM-003-E1). parentID 빈 문자열이면 키 부재.
func evalItemDetails(itemID, hierarchyCode, parentID string, level int) ([]byte, error) {
	d := map[string]string{
		"eval_item_id":   itemID,
		"hierarchy_code": hierarchyCode,
		"level":          strconv.Itoa(level),
	}
	if parentID != "" {
		d["parent_id"] = parentID
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("recorder: marshal eval_item details: %w", err)
	}
	return b, nil
}

// RecordEvalItemCreated EVAL_ITEM_CREATED 감사 이벤트를 기록 (SPEC-AX-EVAL-ITEM-001)
// 항목(루트/자식) 생성과 동일 AuditTx에 audit_logs 1건 (REQ-EVALITEM-UBI-002 / REQ-EVALITEM-003-E1)
// resource_id = AUD-1 결정적 UUIDv5 surrogate, 실 식별자는 DetailsJSON (§6.6)
//
// @MX:ANCHOR: [AUTO] 평가항목 생성 감사 단일 진입점 — REQ-EVALITEM-UBI-002 AC가 이 메서드 경유
// @MX:REASON: 핸들러 + 통합 테스트 + 감사 검증 등 3곳 이상에서 호출 — AUD-1 결정성 계약 (SEC-05)
func (r *Recorder) RecordEvalItemCreated(ctx context.Context, tx AuditTx, itemID, hierarchyCode, parentID string, level int, userID string) error {
	details, err := evalItemDetails(itemID, hierarchyCode, parentID, level)
	if err != nil {
		return err
	}
	e := &Event{
		Timestamp:    r.nowUTC(),
		Action:       ActionEvalItemCreated,
		ResourceType: "evaluation_item",
		ResourceID:   evalItemResourceID(hierarchyCode),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}

// RecordEvalItemUpdated EVAL_ITEM_UPDATED 감사 이벤트를 기록 (SPEC-AX-EVAL-ITEM-001)
// 항목 속성/상태 변경과 동일 AuditTx에 audit_logs 1건 (REQ-EVALITEM-UBI-002 / REQ-EVALITEM-004-O1)
// RecordEvalItemCreated와 동일 resource_id 산출 로직 — 동일 hierarchy_code 상관관계 유지
//
// @MX:ANCHOR: [AUTO] 평가항목 수정 감사 단일 진입점 — REQ-EVALITEM-UBI-002 AC가 이 메서드 경유
// @MX:REASON: 핸들러 + 통합 테스트 + 감사 검증 등 3곳 이상에서 호출 — AUD-1 결정성 계약 (SEC-05)
func (r *Recorder) RecordEvalItemUpdated(ctx context.Context, tx AuditTx, itemID, hierarchyCode, parentID string, level int, userID string) error {
	details, err := evalItemDetails(itemID, hierarchyCode, parentID, level)
	if err != nil {
		return err
	}
	e := &Event{
		Timestamp:    r.nowUTC(),
		Action:       ActionEvalItemUpdated,
		ResourceType: "evaluation_item",
		ResourceID:   evalItemResourceID(hierarchyCode),
		UserID:       r.resolveUserID(userID),
		DetailsJSON:  details,
	}
	return tx.InsertAuditLog(ctx, e)
}
