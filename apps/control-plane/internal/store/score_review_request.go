// score_review_request.go — 평가 검토 요청(ScoreReviewRequest) 도메인 pgx 기반 트랜잭션 구현
// SPEC-AX-REVIEW-001: PgScoreTx 패턴을 미러링한 평가 검토 트랜잭션 구현
// 상태 머신: SUBMITTED → UNDER_REVIEW → {APPROVED|REJECTED} (terminal)
// D2: audit resource_id = score_review_requests.id 직접 대입 (surrogate 금지)
// §A.5: validateRejectionReason (REJECTED 시 rejection_reason non-empty, Layer 1 handler validation)
// §A.6: SELECT FOR UPDATE 비관 락 + validateReviewStatusTransition 이중 방어 (pessimistic)
// UBI-003: userID 파라미터로 created_by/updated_by/audit_logs.user_id 일관 영속화
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// 상태 머신 상수 — 0005 마이그레이션 CHECK 제약과 정합
const (
	reviewStatusSubmitted   = "SUBMITTED"
	reviewStatusUnderReview = "UNDER_REVIEW"
	reviewStatusApproved    = "APPROVED"
	reviewStatusRejected    = "REJECTED"
)

// fallbackUserID userID 빈 문자열이면 'cli-anonymous'로 대체 (UBI-003 fallback).
// audit.DefaultUserID와 일치하나 audit 패키지 비의존을 위해 내부 상수로 보유.
const fallbackUserID = "cli-anonymous"

// allowedReviewTransitions §A.6 state-machine 화이트리스트.
// SUBMITTED→UNDER_REVIEW (AssignReviewer), UNDER_REVIEW→APPROVED/REJECTED (Approve/Reject) 만 허용.
// APPROVED/REJECTED는 terminal (UBI-004).
var allowedReviewTransitions = map[string]map[string]struct{}{
	reviewStatusSubmitted: {
		reviewStatusUnderReview: {},
	},
	reviewStatusUnderReview: {
		reviewStatusApproved: {},
		reviewStatusRejected: {},
	},
	reviewStatusApproved: {}, // terminal
	reviewStatusRejected: {}, // terminal
}

// PgScoreReviewRequestTx pgx.Tx 래퍼 — ScoreReviewRequestTx 인터페이스 구현.
// 단일 PostgreSQL 트랜잭션 내에서 모든 평가 검토 쓰기/조회 연산을 수행한다.
// recorder를 보유하여 mutation(Insert/AssignReviewer/Approve/Reject)이 entity-INSERT/UPDATE 직후
// 동일 tx에 audit-INSERT를 수행한다 (PgScoreTx 동형 — PgScoreReviewRequestTx 자신이 audit.AuditTx 구현).
//
// @MX:WARN: [AUTO] mutation 내 entity-write 후 recorder.RecordScoreReviewRequest* 실패 시
//
//	호출자가 Commit하면 안 됨 — deferred Rollback이 entity+audit 양방향 취소
//
// @MX:REASON: REQ-REVIEW-UBI-002 — 모든 mutation이 동일 t.tx에서 원자적으로 실행. 순서
//
//	(entity-write → audit-INSERT → 호출자 Commit) 변경 시 양방향 원자성 붕괴.
type PgScoreReviewRequestTx struct {
	// tx 래핑된 pgx 트랜잭션
	tx pgx.Tx
	// logger 구조화 로그
	logger *zap.Logger
	// recorder 평가 검토 감사 이벤트 기록기 — 동일 tx에 audit_logs 1건 INSERT.
	// 인터페이스로 노출되지 않고 *audit.Recorder 직접 보유 (consumer-only [HARD] 정합).
	// 테스트는 internal_test.go의 setRecorder 헬퍼로 fault recorder 주입 (whitebox).
	recorder *audit.Recorder
}

// validateReviewRequestInput Insert pre-write 입력 검증 — SQL 미실행 후 거부 (fail-closed).
// score.go:79-97 validateScoreInput 동형 패턴. uuid.Nil score_id 거부.
func validateReviewRequestInput(scoreID uuid.UUID) error {
	if scoreID == uuid.Nil {
		return fmt.Errorf("score_id가 비어 있음(uuid.Nil): %w", stderrors.ErrScoreReviewRequestInvalidInput)
	}
	return nil
}

// validateRejectionReason RejectRequest pre-store 검증 — REJECTED 시 rejection_reason non-empty 강제
// (§A.5 Layer 1, EVAL-ITEM-001 이중 방어 동형). SQL 미실행 후 거부 (fail-closed).
func validateRejectionReason(rejectionReason string) error {
	if strings.TrimSpace(rejectionReason) == "" {
		return fmt.Errorf("REJECTED 시 rejection_reason 필수: %w", stderrors.ErrScoreReviewRequestInvalidInput)
	}
	return nil
}

// validateReviewStatusTransition 현재 상태 → 목표 상태 전이 허용 여부 검증 (§A.6).
// 허용 외 전이는 ErrScoreReviewRequestInvalidStatus 래핑 반환 (SQL 미실행 후 거부).
func validateReviewStatusTransition(current, next string) error {
	if _, ok := allowedReviewTransitions[current]; !ok {
		return fmt.Errorf("status=%q 허용 외 현재 상태: %w", current, stderrors.ErrScoreReviewRequestInvalidStatus)
	}
	if _, ok := allowedReviewTransitions[current][next]; !ok {
		return fmt.Errorf("status 전이 %s→%s 허용되지 않음: %w",
			current, next, stderrors.ErrScoreReviewRequestInvalidStatus)
	}
	return nil
}

// marshalReviewMetadata metadata map을 JSONB 바이트로 직렬화 (nil/빈 맵이면 NULL — score.go 동형)
func marshalReviewMetadata(metadata map[string]any) (interface{}, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("review request metadata 직렬화 실패: %w", err)
	}
	return b, nil
}

// resolveUserID userID 빈 문자열이면 'cli-anonymous' fallback (UBI-003).
// auth-disabled Walking Skeleton + handler context 부재 시 핸들러가 ""를 전달 → 본 함수가 fallback.
func resolveUserID(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return fallbackUserID
	}
	return userID
}

// InsertScoreReviewRequest score_review_requests 테이블에 새 SUBMITTED 행을 삽입하고 UUID 반환.
// 검증 실패 시 SQL 미실행 후 ErrScoreReviewRequestInvalidInput 래핑 반환.
// 성공 시 동일 t.tx에 SCORE_REVIEW_REQUEST_CREATED audit 1건을 기록 (REQ-REVIEW-UBI-002).
// userID: created_by/updated_by + audit_logs.user_id에 일관 영속 (UBI-003).
//
// @MX:ANCHOR: [AUTO] 평가 검토 생성 단일 진입점 — 핸들러/통합 테스트/recorder 3곳 이상 호출
// @MX:REASON: InsertScoreReviewRequest → recorder.RecordScoreReviewRequestCreated(동일 t.tx)
//
//	원자성 계약 — REQ-REVIEW-001-E1, AC-REVIEW-UBI-002 동일-TX 단언 대상
func (t *PgScoreReviewRequestTx) InsertScoreReviewRequest(
	ctx context.Context,
	scoreID uuid.UUID,
	comment string,
	metadata map[string]any,
	userID string,
) (uuid.UUID, error) {
	if vErr := validateReviewRequestInput(scoreID); vErr != nil {
		return uuid.Nil, vErr
	}

	metaJSON, mErr := marshalReviewMetadata(metadata)
	if mErr != nil {
		return uuid.Nil, mErr
	}

	// comment는 nil 허용 — empty 문자열이면 NULL로 저장
	var commentArg interface{}
	if strings.TrimSpace(comment) != "" {
		commentArg = comment
	}

	actor := resolveUserID(userID)

	const query = `
		INSERT INTO score_review_requests (
			score_id, status, comment, metadata,
			created_at, created_by, updated_at, updated_by
		) VALUES (
			$1, 'SUBMITTED', $2, $3,
			now(), $4, now(), $4
		)
		RETURNING id
	`
	var id uuid.UUID
	if qErr := t.tx.QueryRow(ctx, query, scoreID, commentArg, metaJSON, actor).Scan(&id); qErr != nil {
		t.logger.Error("InsertScoreReviewRequest 실패",
			zap.String("score_id", scoreID.String()),
			zap.Error(qErr),
		)
		return uuid.Nil, fmt.Errorf("InsertScoreReviewRequest 실패: %w", qErr)
	}

	// REQ-REVIEW-UBI-002: entity-INSERT 직후 동일 t.tx에 audit 1건 (actor 일관 영속).
	if auditErr := t.recorder.RecordScoreReviewRequestCreated(ctx, t, id, scoreID, actor); auditErr != nil {
		t.logger.Error("InsertScoreReviewRequest audit 기록 실패",
			zap.String("review_request_id", id.String()),
			zap.Error(auditErr),
		)
		return uuid.Nil, fmt.Errorf("InsertScoreReviewRequest audit 실패: %w: %w",
			stderrors.ErrScoreReviewRequestAuditWriteFailed, auditErr)
	}
	return id, nil
}

// GetScoreReviewRequestByID 평가 검토 요청을 id로 단건 조회.
// 미존재 시 ErrScoreReviewRequestNotFound 래핑 반환 (raw pgx.ErrNoRows 누출 금지).
func (t *PgScoreReviewRequestTx) GetScoreReviewRequestByID(ctx context.Context, id uuid.UUID) (*ScoreReviewRequest, error) {
	const query = `
		SELECT id, score_id, status, assigned_reviewer_id, rejection_reason, comment,
		       metadata, created_at, created_by, updated_at, updated_by
		FROM score_review_requests
		WHERE id = $1
	`
	r, err := scanReviewRequestRow(t.tx.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetScoreReviewRequestByID id=%s: %w", id, stderrors.ErrScoreReviewRequestNotFound)
		}
		return nil, fmt.Errorf("GetScoreReviewRequestByID scan 실패: %w", err)
	}
	return r, nil
}

// ListScoreReviewRequests 필터 + 페이지네이션 조회. created_at DESC 정렬. 빈 결과는 빈 슬라이스.
func (t *PgScoreReviewRequestTx) ListScoreReviewRequests(
	ctx context.Context,
	status string,
	limit, offset int,
) ([]*ScoreReviewRequest, error) {
	var rows pgx.Rows
	var qErr error
	if strings.TrimSpace(status) == "" {
		const query = `
			SELECT id, score_id, status, assigned_reviewer_id, rejection_reason, comment,
			       metadata, created_at, created_by, updated_at, updated_by
			FROM score_review_requests
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		rows, qErr = t.tx.Query(ctx, query, limit, offset)
	} else {
		const query = `
			SELECT id, score_id, status, assigned_reviewer_id, rejection_reason, comment,
			       metadata, created_at, created_by, updated_at, updated_by
			FROM score_review_requests
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, qErr = t.tx.Query(ctx, query, status, limit, offset)
	}
	if qErr != nil {
		return nil, fmt.Errorf("ListScoreReviewRequests 실패: %w", qErr)
	}
	defer rows.Close()

	result := make([]*ScoreReviewRequest, 0)
	for rows.Next() {
		r, scanErr := scanReviewRequestRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("ListScoreReviewRequests scan 실패: %w", scanErr)
		}
		result = append(result, r)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("ListScoreReviewRequests rows 에러: %w", rowsErr)
	}
	return result, nil
}

// CountScoreReviewRequests 필터(status optional)에 해당하는 전체 행 수를 반환.
// pagination total 정확 계산용 (AC-REVIEW-002-3 — limit/offset 적용 전 전체 카운트).
func (t *PgScoreReviewRequestTx) CountScoreReviewRequests(ctx context.Context, status string) (int64, error) {
	var n int64
	if strings.TrimSpace(status) == "" {
		const q = `SELECT COUNT(*) FROM score_review_requests`
		if err := t.tx.QueryRow(ctx, q).Scan(&n); err != nil {
			return 0, fmt.Errorf("CountScoreReviewRequests 실패: %w", err)
		}
		return n, nil
	}
	const q = `SELECT COUNT(*) FROM score_review_requests WHERE status = $1`
	if err := t.tx.QueryRow(ctx, q, status).Scan(&n); err != nil {
		return 0, fmt.Errorf("CountScoreReviewRequests 실패: %w", err)
	}
	return n, nil
}

// lockAndCheckStatus SELECT ... FOR UPDATE로 row lock 획득 후 현재 status 반환 (§A.6).
// 미존재 시 ErrScoreReviewRequestNotFound 반환.
func (t *PgScoreReviewRequestTx) lockAndCheckStatus(ctx context.Context, id uuid.UUID) (string, error) {
	const lockSQL = `SELECT status FROM score_review_requests WHERE id = $1 FOR UPDATE`
	var currentStatus string
	if err := t.tx.QueryRow(ctx, lockSQL, id).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("score_review_request id=%s: %w", id, stderrors.ErrScoreReviewRequestNotFound)
		}
		return "", fmt.Errorf("SELECT FOR UPDATE 실패: %w", err)
	}
	return currentStatus, nil
}

// AssignReviewer SUBMITTED→UNDER_REVIEW 전이 + assigned_reviewer_id 업데이트.
// 1. SELECT FOR UPDATE row lock 획득 (§A.6)
// 2. 현재 status == 'SUBMITTED' 검증 (Edge E14: 아니면 ErrScoreReviewRequestNotSubmitted)
// 3. validateReviewStatusTransition (이중 방어)
// 4. UPDATE + RecordScoreReviewRequestReviewerAssigned (동일 TX)
// userID: updated_by + audit_logs.user_id (UBI-003).
//
// @MX:ANCHOR: [AUTO] 평가 검토 검토자 할당 단일 진입점 — REQ-REVIEW-003-E1 / E10 동시성 결정성 계약
// @MX:REASON: SELECT FOR UPDATE + validateReviewStatusTransition 이중 방어 (§A.6) — 순서 변경 시 race 발생
func (t *PgScoreReviewRequestTx) AssignReviewer(ctx context.Context, id uuid.UUID, reviewerID, userID string) error {
	if strings.TrimSpace(reviewerID) == "" {
		return fmt.Errorf("reviewer_id가 비어 있음: %w", stderrors.ErrScoreReviewRequestInvalidInput)
	}
	currentStatus, lockErr := t.lockAndCheckStatus(ctx, id)
	if lockErr != nil {
		return lockErr
	}
	if currentStatus != reviewStatusSubmitted {
		return fmt.Errorf("AssignReviewer id=%s: 현재 status=%s (SUBMITTED 필요): %w",
			id, currentStatus, stderrors.ErrScoreReviewRequestNotSubmitted)
	}
	if tErr := validateReviewStatusTransition(currentStatus, reviewStatusUnderReview); tErr != nil {
		return tErr
	}

	// score_id 조회 (audit details용)
	scoreID, scoreErr := t.fetchScoreID(ctx, id)
	if scoreErr != nil {
		return scoreErr
	}

	actor := resolveUserID(userID)

	const updateSQL = `
		UPDATE score_review_requests
		SET status = 'UNDER_REVIEW', assigned_reviewer_id = $2,
		    updated_at = now(), updated_by = $3
		WHERE id = $1
	`
	if _, execErr := t.tx.Exec(ctx, updateSQL, id, reviewerID, actor); execErr != nil {
		t.logger.Error("AssignReviewer UPDATE 실패",
			zap.String("id", id.String()),
			zap.Error(execErr),
		)
		return fmt.Errorf("AssignReviewer UPDATE 실패: %w", execErr)
	}

	if auditErr := t.recorder.RecordScoreReviewRequestReviewerAssigned(ctx, t, id, scoreID, reviewerID, actor); auditErr != nil {
		t.logger.Error("AssignReviewer audit 기록 실패", zap.Error(auditErr))
		return fmt.Errorf("AssignReviewer audit 실패: %w: %w",
			stderrors.ErrScoreReviewRequestAuditWriteFailed, auditErr)
	}
	return nil
}

// ApproveRequest UNDER_REVIEW→APPROVED terminal 전이 (§A.6 + §A.2).
// SELECT FOR UPDATE + validateReviewStatusTransition 이중 방어로 race 결정성 보장 (E10).
// userID: updated_by + audit_logs.user_id (UBI-003).
//
// @MX:ANCHOR: [AUTO] 평가 검토 승인 단일 진입점 — REQ-REVIEW-003-E2 / E10 결정성 계약
// @MX:REASON: terminal 전이 + SELECT FOR UPDATE pessimistic lock — 동시 admin 승인 시 직렬화 보장
func (t *PgScoreReviewRequestTx) ApproveRequest(ctx context.Context, id uuid.UUID, comment, userID string) error {
	currentStatus, lockErr := t.lockAndCheckStatus(ctx, id)
	if lockErr != nil {
		return lockErr
	}
	if currentStatus != reviewStatusUnderReview {
		return fmt.Errorf("ApproveRequest id=%s: 현재 status=%s (UNDER_REVIEW 필요): %w",
			id, currentStatus, stderrors.ErrScoreReviewRequestNotUnderReview)
	}
	if tErr := validateReviewStatusTransition(currentStatus, reviewStatusApproved); tErr != nil {
		return tErr
	}

	scoreID, scoreErr := t.fetchScoreID(ctx, id)
	if scoreErr != nil {
		return scoreErr
	}

	var commentArg interface{}
	if strings.TrimSpace(comment) != "" {
		commentArg = comment
	}
	actor := resolveUserID(userID)

	const updateSQL = `
		UPDATE score_review_requests
		SET status = 'APPROVED', comment = COALESCE($2, comment),
		    updated_at = now(), updated_by = $3
		WHERE id = $1
	`
	if _, execErr := t.tx.Exec(ctx, updateSQL, id, commentArg, actor); execErr != nil {
		return fmt.Errorf("ApproveRequest UPDATE 실패: %w", execErr)
	}

	if auditErr := t.recorder.RecordScoreReviewRequestApproved(ctx, t, id, scoreID, comment, actor); auditErr != nil {
		return fmt.Errorf("ApproveRequest audit 실패: %w: %w",
			stderrors.ErrScoreReviewRequestAuditWriteFailed, auditErr)
	}
	return nil
}

// RejectRequest UNDER_REVIEW→REJECTED terminal 전이 (§A.5 Layer 1 + §A.6 + §A.2).
// rejection_reason empty 시 SQL 미실행 후 ErrScoreReviewRequestInvalidInput 래핑.
// userID: updated_by + audit_logs.user_id (UBI-003).
//
// @MX:ANCHOR: [AUTO] 평가 검토 반려 단일 진입점 — REQ-REVIEW-003-E3 / §A.5 이중 방어 계약
// @MX:REASON: rejection_reason validation Layer 1 (handler-side) + Layer 2 (DB CHECK) — invariant 보호
func (t *PgScoreReviewRequestTx) RejectRequest(ctx context.Context, id uuid.UUID, rejectionReason, comment, userID string) error {
	// Layer 1: pre-store validation — rejection_reason non-empty 강제 (§A.5)
	if vErr := validateRejectionReason(rejectionReason); vErr != nil {
		return vErr
	}

	currentStatus, lockErr := t.lockAndCheckStatus(ctx, id)
	if lockErr != nil {
		return lockErr
	}
	if currentStatus != reviewStatusUnderReview {
		return fmt.Errorf("RejectRequest id=%s: 현재 status=%s (UNDER_REVIEW 필요): %w",
			id, currentStatus, stderrors.ErrScoreReviewRequestNotUnderReview)
	}
	if tErr := validateReviewStatusTransition(currentStatus, reviewStatusRejected); tErr != nil {
		return tErr
	}

	scoreID, scoreErr := t.fetchScoreID(ctx, id)
	if scoreErr != nil {
		return scoreErr
	}

	var commentArg interface{}
	if strings.TrimSpace(comment) != "" {
		commentArg = comment
	}
	actor := resolveUserID(userID)

	const updateSQL = `
		UPDATE score_review_requests
		SET status = 'REJECTED', rejection_reason = $2, comment = COALESCE($3, comment),
		    updated_at = now(), updated_by = $4
		WHERE id = $1
	`
	if _, execErr := t.tx.Exec(ctx, updateSQL, id, rejectionReason, commentArg, actor); execErr != nil {
		return fmt.Errorf("RejectRequest UPDATE 실패: %w", execErr)
	}

	if auditErr := t.recorder.RecordScoreReviewRequestRejected(ctx, t, id, scoreID, rejectionReason, comment, actor); auditErr != nil {
		return fmt.Errorf("RejectRequest audit 실패: %w: %w",
			stderrors.ErrScoreReviewRequestAuditWriteFailed, auditErr)
	}
	return nil
}

// fetchScoreID 락 획득 이후 score_id 조회 (audit details용)
func (t *PgScoreReviewRequestTx) fetchScoreID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	const q = `SELECT score_id FROM score_review_requests WHERE id = $1`
	var scoreID uuid.UUID
	if err := t.tx.QueryRow(ctx, q, id).Scan(&scoreID); err != nil {
		return uuid.Nil, fmt.Errorf("score_id 조회 실패: %w", err)
	}
	return scoreID, nil
}

// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입 (Recorder가 호출)
// D2: resource_id = score_review_requests.id UUID 직접 대입 — surrogate 미사용
func (t *PgScoreReviewRequestTx) InsertAuditLog(ctx context.Context, e *audit.Event) error {
	const query = `
		INSERT INTO audit_logs (id, action, resource_type, resource_id, user_id, details, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	id := uuid.New()
	var details interface{}
	if len(e.DetailsJSON) > 0 {
		details = e.DetailsJSON
	}
	_, err := t.tx.Exec(ctx, query,
		id, string(e.Action), e.ResourceType, e.ResourceID, e.UserID, details, e.Timestamp,
	)
	if err != nil {
		t.logger.Error("InsertAuditLog(score_review_request) 실패",
			zap.String("action", string(e.Action)),
			zap.String("resource_id", e.ResourceID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("InsertAuditLog(score_review_request) 실패: %w", err)
	}
	return nil
}

// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
func (t *PgScoreReviewRequestTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("Commit(score_review_request) 실패: %w", err)
	}
	return nil
}

// Rollback 현재 트랜잭션을 롤백 (Commit 후 호출 시 pgx가 무시)
func (t *PgScoreReviewRequestTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return nil
		}
		return fmt.Errorf("Rollback(score_review_request) 실패: %w", err)
	}
	return nil
}

// scanReviewRequestRow pgx.Row/pgx.Rows 공통 스캔 헬퍼 (DAMP — 컬럼 순서 단일 정의)
func scanReviewRequestRow(row pgx.Row) (*ScoreReviewRequest, error) {
	var (
		r                  ScoreReviewRequest
		assignedReviewerID *string
		rejectionReason    *string
		comment            *string
		metaRaw            []byte
	)
	if err := row.Scan(
		&r.ID, &r.ScoreID, &r.Status, &assignedReviewerID, &rejectionReason, &comment,
		&metaRaw, &r.CreatedAt, &r.CreatedBy, &r.UpdatedAt, &r.UpdatedBy,
	); err != nil {
		return nil, err
	}
	r.AssignedReviewerID = assignedReviewerID
	r.RejectionReason = rejectionReason
	r.Comment = comment
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &r.Metadata) //nolint:errcheck // 손상된 메타데이터는 nil로 graceful
	}
	return &r, nil
}
