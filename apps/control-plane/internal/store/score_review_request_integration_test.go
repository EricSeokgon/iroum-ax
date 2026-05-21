//go:build integration

// score_review_request_integration_test.go — SPEC-AX-REVIEW-001 DB 의존 통합 테스트
//
// 검증 대상:
//   - T-101 (AC-REVIEW-001-1 / UBI-002): InsertScoreReviewRequest 동일-TX entity+audit
//   - T-101-D1 (AC-REVIEW-UBI-003 정합 강화): userID 영속화 (created_by/updated_by/audit user_id)
//   - T-103 (AC-REVIEW-003-1): AssignReviewer SUBMITTED→UNDER_REVIEW + audit
//   - T-104 (Edge E14): AssignReviewer UNDER_REVIEW → ErrScoreReviewRequestNotSubmitted
//   - T-106 (AC-REVIEW-003-2 / Edge E10): SELECT FOR UPDATE 동시 admin approve race 결정성
//   - T-107 (AC-REVIEW-003-4): terminal 상태 추가 전이 거부
//   - T-108 (AC-REVIEW-003-2): ApproveRequest UNDER_REVIEW→APPROVED + audit
//   - T-109 (Edge E15): ApproveRequest SUBMITTED → ErrScoreReviewRequestNotUnderReview
//   - T-110 (AC-REVIEW-003-3): RejectRequest UNDER_REVIEW→REJECTED with reason + audit
//   - T-111 (AC-REVIEW-004-2 / Edge E16): audit fault injection 양방향 rollback (whitebox helper)
//   - T-Count: CountScoreReviewRequests full count (AC-REVIEW-002-3)
//
// 실행: go test -tags=integration -count=1 -timeout=600s -run TestReview ./apps/control-plane/internal/store/
package store

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// applyMigration0005 0005_score_review_request_tables.sql 을 testcontainers DB에 적용한다.
// 0001~0004 base는 setupTestDB의 schema.sql + applyMigration0004로 부트스트랩.
func applyMigration0005(t *testing.T, db *testDB) {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err, "repo 루트 조회 실패")
	root := strings.TrimSpace(string(out))

	sqlPath := filepath.Join(root, ".moai/db/schema/migrations/0005_score_review_request_tables.sql")
	sqlBytes, err := os.ReadFile(sqlPath) //nolint:gosec // 테스트 고정 경로, 사용자 입력 아님
	require.NoError(t, err, "0005 마이그레이션 파일 읽기 실패: %s", sqlPath)

	_, err = db.pool.Exec(context.Background(), string(sqlBytes))
	require.NoError(t, err, "0005 마이그레이션 적용 실패")
}

// auditReviewCount resource_id별 audit_logs 행 수를 action 기준 조회
func auditReviewCount(t *testing.T, db *testDB, reviewID uuid.UUID, action string) int {
	t.Helper()
	var n int
	require.NoError(t, db.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_logs
		 WHERE resource_type='score_review_request' AND action=$1 AND resource_id=$2`,
		action, reviewID).Scan(&n))
	return n
}

// auditReviewUserID 첫 매치 audit_logs row의 user_id 조회 (UBI-003 영속화 검증용)
func auditReviewUserID(t *testing.T, db *testDB, reviewID uuid.UUID, action string) string {
	t.Helper()
	var userID string
	require.NoError(t, db.pool.QueryRow(context.Background(),
		`SELECT user_id FROM audit_logs
		 WHERE resource_type='score_review_request' AND action=$1 AND resource_id=$2
		 LIMIT 1`,
		action, reviewID).Scan(&userID))
	return userID
}

// reviewCount score_review_requests 테이블 행 수 조회
func reviewCount(t *testing.T, db *testDB, reviewID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, db.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM score_review_requests WHERE id=$1`, reviewID).Scan(&n))
	return n
}

// setupReviewTestDB 0001~0005 모든 마이그레이션이 적용된 testDB 반환
func setupReviewTestDB(t *testing.T) *testDB {
	t.Helper()
	db := setupTestDB(t)
	applyMigration0004(t, db) // scores 테이블 (FK-less stub용)
	applyMigration0005(t, db) // score_review_requests 테이블
	return db
}

// insertSubmittedReview 테스트 헬퍼: 새 SUBMITTED 행 생성 후 id 반환
// userID 파라미터 전달로 D1 (UBI-003) 영속화 검증 가능
func insertSubmittedReview(t *testing.T, db *testDB, ctx context.Context, userID string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	scoreID := uuid.New() // FK-less stub — scores에 실재할 필요 없음
	id, err := tx.InsertScoreReviewRequest(ctx, scoreID, "초기 검토 요청", nil, userID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return id, scoreID
}

// advanceToUnderReview SUBMITTED → UNDER_REVIEW 전이 헬퍼 (admin userID)
func advanceToUnderReview(t *testing.T, db *testDB, ctx context.Context, id uuid.UUID, userID string) {
	t.Helper()
	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.AssignReviewer(ctx, id, "reviewer-user-1", userID))
	require.NoError(t, tx.Commit(ctx))
}

// ════════════════════════════════════════════════════════════════════════════
// T-101 [GREEN] InsertScoreReviewRequest 동일-TX entity+audit
// AC-REVIEW-001-1 / AC-REVIEW-UBI-002 / AC-REVIEW-UBI-003: SUBMITTED row 1건 +
//   audit_logs 1건 + created_by/updated_by/audit user_id 일관 영속화
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_InsertCreatesEntityAndAudit_CliAnonymous(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	scoreID := uuid.New()
	// auth-disabled 시뮬레이션: handler가 "cli-anonymous" fallback을 전달
	id, err := tx.InsertScoreReviewRequest(ctx, scoreID, "검토 요청합니다", nil, "cli-anonymous")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	assert.Equal(t, 1, reviewCount(t, db, id), "entity 1건")

	var status, createdBy, updatedBy string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status, created_by, updated_by FROM score_review_requests WHERE id=$1`, id).Scan(&status, &createdBy, &updatedBy))
	assert.Equal(t, "SUBMITTED", status)
	assert.Equal(t, "cli-anonymous", createdBy, "auth-disabled fallback created_by (UBI-003)")
	assert.Equal(t, "cli-anonymous", updatedBy, "auth-disabled fallback updated_by")

	// audit_logs 1건 + user_id 일관 영속
	assert.Equal(t, 1, auditReviewCount(t, db, id, "SCORE_REVIEW_REQUEST_CREATED"))
	assert.Equal(t, "cli-anonymous", auditReviewUserID(t, db, id, "SCORE_REVIEW_REQUEST_CREATED"),
		"audit_logs.user_id == 'cli-anonymous' (UBI-003 + UBI-002 일관)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-101-D1 [NEW for iteration 2] InsertScoreReviewRequest auth-enabled principal.id 영속화
// AC-REVIEW-UBI-003: auth-enabled 시 principal.id가 DB row + audit_logs.user_id에 영속화되어야 함
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_InsertCreatesEntity_AuthEnabledPrincipalIDPersisted(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	scoreID := uuid.New()
	// auth-enabled 시뮬레이션: handler가 principal.UID를 전달
	id, err := tx.InsertScoreReviewRequest(ctx, scoreID, "user-alice 검토 요청", nil, "user-alice")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	// principal.id가 created_by/updated_by에 정확 영속 (D1 fix — UBI-003 must-pass)
	var createdBy, updatedBy string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT created_by, updated_by FROM score_review_requests WHERE id=$1`, id).Scan(&createdBy, &updatedBy))
	assert.Equal(t, "user-alice", createdBy, "auth-enabled principal.id가 created_by에 영속 (UBI-003)")
	assert.Equal(t, "user-alice", updatedBy, "auth-enabled principal.id가 updated_by에 영속")

	// audit_logs.user_id도 동일 영속
	assert.Equal(t, "user-alice", auditReviewUserID(t, db, id, "SCORE_REVIEW_REQUEST_CREATED"),
		"audit_logs.user_id == principal.id (D1 fix)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-103 AssignReviewer SUBMITTED → UNDER_REVIEW + audit + updated_by 영속화
// AC-REVIEW-003-1 + UBI-003
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_AssignReviewer_FromSubmitted_TransitionsToUnderReview(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	id, _ := insertSubmittedReview(t, db, ctx, "user-alice")

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.AssignReviewer(ctx, id, "reviewer-user-42", "admin-bob"))
	require.NoError(t, tx.Commit(ctx))

	var status, updatedBy string
	var assignedReviewerID *string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status, assigned_reviewer_id, updated_by FROM score_review_requests WHERE id=$1`, id).Scan(&status, &assignedReviewerID, &updatedBy))
	assert.Equal(t, "UNDER_REVIEW", status)
	require.NotNil(t, assignedReviewerID)
	assert.Equal(t, "reviewer-user-42", *assignedReviewerID)
	assert.Equal(t, "admin-bob", updatedBy, "AssignReviewer updated_by = admin principal.id (D1, UBI-003)")

	assert.Equal(t, 1, auditReviewCount(t, db, id, "SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED"))
	assert.Equal(t, "admin-bob", auditReviewUserID(t, db, id, "SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED"))
}

// ════════════════════════════════════════════════════════════════════════════
// T-104 AssignReviewer UNDER_REVIEW → ErrScoreReviewRequestNotSubmitted
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_AssignReviewer_FromUnderReview_ReturnsNotSubmitted(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	id, _ := insertSubmittedReview(t, db, ctx, "user-alice")
	advanceToUnderReview(t, db, ctx, id, "admin-bob")

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	err = tx.AssignReviewer(ctx, id, "reviewer-user-99", "admin-bob")
	_ = tx.Rollback(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreReviewRequestNotSubmitted)

	var assignedReviewerID *string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT assigned_reviewer_id FROM score_review_requests WHERE id=$1`, id).Scan(&assignedReviewerID))
	require.NotNil(t, assignedReviewerID)
	assert.Equal(t, "reviewer-user-1", *assignedReviewerID, "거부 시 기존 값 무변경")
}

// ════════════════════════════════════════════════════════════════════════════
// T-108 ApproveRequest UNDER_REVIEW → APPROVED + audit + updated_by 영속화
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_ApproveRequest_FromUnderReview_TransitionsToApproved(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	id, _ := insertSubmittedReview(t, db, ctx, "user-alice")
	advanceToUnderReview(t, db, ctx, id, "admin-bob")

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.ApproveRequest(ctx, id, "승인합니다", "admin-bob"))
	require.NoError(t, tx.Commit(ctx))

	var status, updatedBy string
	var comment *string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status, comment, updated_by FROM score_review_requests WHERE id=$1`, id).Scan(&status, &comment, &updatedBy))
	assert.Equal(t, "APPROVED", status)
	require.NotNil(t, comment)
	assert.Equal(t, "승인합니다", *comment)
	assert.Equal(t, "admin-bob", updatedBy, "ApproveRequest updated_by 영속 (D1)")

	assert.Equal(t, 1, auditReviewCount(t, db, id, "SCORE_REVIEW_REQUEST_APPROVED"))
	assert.Equal(t, "admin-bob", auditReviewUserID(t, db, id, "SCORE_REVIEW_REQUEST_APPROVED"))
}

// ════════════════════════════════════════════════════════════════════════════
// T-109 ApproveRequest SUBMITTED → ErrScoreReviewRequestNotUnderReview (Edge E15)
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_ApproveRequest_FromSubmitted_ReturnsNotUnderReview(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	id, _ := insertSubmittedReview(t, db, ctx, "user-alice")

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	err = tx.ApproveRequest(ctx, id, "", "admin-bob")
	_ = tx.Rollback(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreReviewRequestNotUnderReview)

	var status string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM score_review_requests WHERE id=$1`, id).Scan(&status))
	assert.Equal(t, "SUBMITTED", status, "거부 시 status 무변경")
}

// ════════════════════════════════════════════════════════════════════════════
// T-110 RejectRequest UNDER_REVIEW → REJECTED with reason + audit + updated_by 영속화
// AC-REVIEW-003-3 + UBI-003
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_RejectRequest_FromUnderReview_WithReason_TransitionsToRejected(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	id, _ := insertSubmittedReview(t, db, ctx, "user-alice")
	advanceToUnderReview(t, db, ctx, id, "admin-bob")

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.RejectRequest(ctx, id, "근거 부족", "", "admin-bob"))
	require.NoError(t, tx.Commit(ctx))

	var status, updatedBy string
	var rejectionReason *string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status, rejection_reason, updated_by FROM score_review_requests WHERE id=$1`, id).Scan(&status, &rejectionReason, &updatedBy))
	assert.Equal(t, "REJECTED", status)
	require.NotNil(t, rejectionReason)
	assert.Equal(t, "근거 부족", *rejectionReason)
	assert.Equal(t, "admin-bob", updatedBy, "RejectRequest updated_by 영속 (D1)")

	assert.Equal(t, 1, auditReviewCount(t, db, id, "SCORE_REVIEW_REQUEST_REJECTED"))
	assert.Equal(t, "admin-bob", auditReviewUserID(t, db, id, "SCORE_REVIEW_REQUEST_REJECTED"))
}

// ════════════════════════════════════════════════════════════════════════════
// T-107 terminal 상태 → 어떤 전이도 거부 (AC-REVIEW-003-4)
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_StatusTransition_FromApproved_AlwaysReturnsInvalidStatus(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	id, _ := insertSubmittedReview(t, db, ctx, "user-alice")
	advanceToUnderReview(t, db, ctx, id, "admin-bob")

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.ApproveRequest(ctx, id, "", "admin-bob"))
	require.NoError(t, tx.Commit(ctx))

	for _, op := range []string{"assign", "approve", "reject"} {
		t.Run(op, func(t *testing.T) {
			tx, err := db.store.BeginScoreReviewRequestTx(ctx)
			require.NoError(t, err)
			var opErr error
			switch op {
			case "assign":
				opErr = tx.AssignReviewer(ctx, id, "another-reviewer", "admin-bob")
			case "approve":
				opErr = tx.ApproveRequest(ctx, id, "", "admin-bob")
			case "reject":
				opErr = tx.RejectRequest(ctx, id, "재반려", "", "admin-bob")
			}
			_ = tx.Rollback(ctx)
			require.Error(t, opErr, "%s on APPROVED는 거부되어야 한다", op)
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-106 SELECT FOR UPDATE 동시 admin approve race 결정성 (Edge E10)
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_ConcurrentAdminApprove_OnlyFirstSucceeds(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	id, _ := insertSubmittedReview(t, db, ctx, "user-alice")
	advanceToUnderReview(t, db, ctx, id, "admin-bob")

	var wg sync.WaitGroup
	results := make([]error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			tx, err := db.store.BeginScoreReviewRequestTx(ctx)
			if err != nil {
				results[idx] = err
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			err = tx.ApproveRequest(ctx, id, "", "admin-bob")
			if err != nil {
				results[idx] = err
				return
			}
			if cErr := tx.Commit(ctx); cErr != nil {
				results[idx] = cErr
				return
			}
			results[idx] = nil
		}(i)
	}
	wg.Wait()

	successCount := 0
	failureCount := 0
	for _, err := range results {
		if err == nil {
			successCount++
		} else {
			failureCount++
			assert.ErrorIs(t, err, stderrors.ErrScoreReviewRequestNotUnderReview)
		}
	}
	assert.Equal(t, 1, successCount, "정확히 1명만 성공 (SELECT FOR UPDATE 직렬화)")
	assert.Equal(t, 1, failureCount, "정확히 1명만 실패")

	var status string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM score_review_requests WHERE id=$1`, id).Scan(&status))
	assert.Equal(t, "APPROVED", status)
	assert.Equal(t, 1, auditReviewCount(t, db, id, "SCORE_REVIEW_REQUEST_APPROVED"),
		"audit row 정확히 1건 (race 결정성)")
}

// ════════════════════════════════════════════════════════════════════════════
// §A.5 Layer 2 DB CHECK constraint — REJECTED 시 reject reason DB-level 강제
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_DBCheckRejectReasonConstraint_RawSQLInsertFails(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		INSERT INTO score_review_requests (score_id, status, rejection_reason)
		VALUES ($1, 'REJECTED', NULL)
	`, uuid.New())
	require.Error(t, err, "REJECTED + NULL rejection_reason은 DB CHECK 위반")
	assert.Contains(t, err.Error(), "score_review_requests_reject_reason_chk")
}

// ════════════════════════════════════════════════════════════════════════════
// T-111 [D3-evaluator iteration 2] audit fault injection 양방향 rollback
// AC-REVIEW-004-2 / Edge E16: audit INSERT 실패 시 호출자 Rollback → entity+audit 0 row
// EVAL-ITEM-001 eval_item_rollback_test.go 동형 패턴 — audit_logs에 CHECK(false) 제약 주입.
// SCORE-001/EVAL-ITEM-001 선례 정확 미러 (acceptance.md amend 불필요).
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_InsertAuditFault_RollbackBothEntities(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	// fault injection: audit_logs INSERT를 무조건 거부하는 CHECK 제약 추가
	// (eval_item_rollback_test.go:40 동형 패턴)
	_, err := db.pool.Exec(ctx, `ALTER TABLE audit_logs ADD CONSTRAINT ci_fail_review CHECK (false)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.pool.Exec(context.Background(),
			`ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS ci_fail_review`)
	})

	tx, beginErr := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, beginErr)
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	scoreID := uuid.New()
	id, insertErr := tx.InsertScoreReviewRequest(ctx, scoreID, "audit fault test", nil, "user-alice")

	require.Error(t, insertErr, "audit fault 발생 시 InsertScoreReviewRequest는 에러 반환")
	assert.ErrorIs(t, insertErr, stderrors.ErrScoreReviewRequestAuditWriteFailed,
		"ErrScoreReviewRequestAuditWriteFailed 래핑되어야 함 (AC-REVIEW-004-2)")
	assert.Equal(t, uuid.Nil, id, "실패 시 반환 UUID는 uuid.Nil")

	// 호출자가 Rollback (defer 패턴) — entity+audit 양방향 취소
	require.NoError(t, tx.Rollback(ctx))
	committed = true

	// Rollback 후 검증: score_review_requests + audit_logs 양방향 0 row (양방향 원자성, AC-REVIEW-UBI-002)
	var entityCount, auditCount int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM score_review_requests WHERE score_id=$1`, scoreID).Scan(&entityCount))
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE resource_type='score_review_request'
		 AND action='SCORE_REVIEW_REQUEST_CREATED'
		 AND (details::jsonb)->>'score_id' = $1`, scoreID.String()).Scan(&auditCount))
	assert.Equal(t, 0, entityCount, "Rollback 후 score_review_requests 0 row (양방향 원자성)")
	assert.Equal(t, 0, auditCount, "Rollback 후 audit_logs 0 row")
}

// ════════════════════════════════════════════════════════════════════════════
// T-Count [NEW] CountScoreReviewRequests 정확성 검증 (AC-REVIEW-002-3 — pagination total)
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_CountScoreReviewRequests_FilterByStatus(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	// 5개 SUBMITTED 생성
	for i := 0; i < 5; i++ {
		_, _ = insertSubmittedReview(t, db, ctx, "user-alice")
	}
	// 2개는 UNDER_REVIEW로 전이
	for i := 0; i < 2; i++ {
		id, _ := insertSubmittedReview(t, db, ctx, "user-alice")
		advanceToUnderReview(t, db, ctx, id, "admin-bob")
	}

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	// 전체 count = 7
	totalAll, err := tx.CountScoreReviewRequests(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int64(7), totalAll, "전체 행 = 5 SUBMITTED + 2 UNDER_REVIEW")

	// SUBMITTED filter count = 5
	totalSubmitted, err := tx.CountScoreReviewRequests(ctx, "SUBMITTED")
	require.NoError(t, err)
	assert.Equal(t, int64(5), totalSubmitted)

	// UNDER_REVIEW filter count = 2
	totalUnder, err := tx.CountScoreReviewRequests(ctx, "UNDER_REVIEW")
	require.NoError(t, err)
	assert.Equal(t, int64(2), totalUnder)

	// 없는 status filter count = 0
	totalApproved, err := tx.CountScoreReviewRequests(ctx, "APPROVED")
	require.NoError(t, err)
	assert.Equal(t, int64(0), totalApproved)
}

// ════════════════════════════════════════════════════════════════════════════
// 0005 마이그레이션 멱등성 검증
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_Migration0005_IsIdempotent(t *testing.T) {
	db := setupReviewTestDB(t)
	applyMigration0005(t, db) // 재적용 시 duplicate_object 발생 없음
	applyMigration0005(t, db)

	var tableName string
	require.NoError(t, db.pool.QueryRow(context.Background(),
		`SELECT tablename FROM pg_tables WHERE tablename='score_review_requests'`).Scan(&tableName))
	assert.Equal(t, "score_review_requests", tableName)
}

// ════════════════════════════════════════════════════════════════════════════
// AC-REVIEW-005-2 [D6 NEW] pgx.ErrNoRows 누출 0 검증
// GetScoreReviewRequestByID는 ErrScoreReviewRequestNotFound 래핑, pgx.ErrNoRows 직접 노출 금지
// ════════════════════════════════════════════════════════════════════════════

func TestReviewIntegration_GetByID_PgxNoRowsWrappedAsNotFound(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupReviewTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginScoreReviewRequestTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, gErr := tx.GetScoreReviewRequestByID(ctx, uuid.New()) // 존재하지 않는 UUID
	require.Error(t, gErr)
	assert.True(t, errors.Is(gErr, stderrors.ErrScoreReviewRequestNotFound),
		"errors.Is(err, ErrScoreReviewRequestNotFound) == true")
	assert.False(t, errors.Is(gErr, pgx.ErrNoRows),
		"errors.Is(err, pgx.ErrNoRows) == false — raw pgx 에러 누출 0 (AC-REVIEW-005-2)")
}
