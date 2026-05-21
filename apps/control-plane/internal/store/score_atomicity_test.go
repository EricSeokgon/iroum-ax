//go:build integration

// score_atomicity_test.go — T-009: 동일-TX 원자성 + audit-fault 양방향 rollback + goroutine 누출
// SPEC-AX-SCORE-001:
//
//	REQ-SCORE-UBI-002 / REQ-SCORE-004-U1: InsertScore + InsertAuditLog 동일 TX 원자성
//	DC-UBI-002: 1 TX = 1 score entity = 1 audit row
//	  cond1: InsertScore+Commit → audit_logs SCORE_CREATED COUNT=1 (실 production 경로)
//	  cond2: UpdateScore+Commit → audit_logs SCORE_UPDATED COUNT=1
//	DC-004-U1: audit 장애 주입 시 scores/audit_logs 양방향 rollback + errors.Is 래핑
//	goleak: 테스트 종료 후 TX goroutine 누출 없음 (TH-03)
//
// [TDD 정정] 이전 captureFaultTx 모의(contrived)는 InsertScore의 실제 recorder 와이어링을
// 우회하여 DC-UBI-002(action='SCORE_CREATED')를 검증하지 못했다. 본 파일은 실제
// BeginScoreTx → InsertScore(recorder 자동 호출) → Commit 경로를 검증한다.
//
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestScore -v -count=1
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// auditScoreCount 별도 풀 커넥션으로 resource_id별 audit_logs 행 수를 action 기준 조회
func auditScoreCount(t *testing.T, db *testDB, scoreID uuid.UUID, action string) int {
	t.Helper()
	var n int
	require.NoError(t, db.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_logs
		 WHERE resource_type='score' AND action=$1 AND resource_id=$2`,
		action, scoreID).Scan(&n))
	return n
}

// ── DC-UBI-002 cond1: InsertScore+Commit → SCORE_CREATED audit COUNT=1 ──────────

// TestScoreAtomicity_InsertCreatesAuditRow DC-UBI-002.1 / REQ-SCORE-001-E1:
// 실제 InsertScore (recorder 자동 와이어링) + Commit 후
// audit_logs에 resource_type='score' AND action='SCORE_CREATED' AND resource_id=score.id 정확히 1건
func TestScoreAtomicity_InsertCreatesAuditRow(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)

	scoreID, err := tx.InsertScore(ctx, "AX-AUDIT-CREATE-01", nil, "raw", 80.0, nil, nil)
	require.NoError(t, err, "InsertScore 성공")
	require.NoError(t, tx.Commit(ctx))

	assert.Equal(t, 1, auditScoreCount(t, db, scoreID, "SCORE_CREATED"),
		"InsertScore+Commit → SCORE_CREATED audit COUNT=1 (DC-UBI-002.1, recorder 실 호출)")

	var scoreCnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scores WHERE id=$1`, scoreID).Scan(&scoreCnt))
	assert.Equal(t, 1, scoreCnt, "scores 행 1건 (동일 TX 원자성)")
}

// ── DC-UBI-002 cond2: UpdateScore+Commit → SCORE_UPDATED audit COUNT=1 ──────────

// TestScoreAtomicity_UpdateCreatesAuditRow DC-UBI-002.2 / REQ-SCORE-001-S2:
// DRAFT 행 UpdateScore (score_value 변경) + Commit 후 SCORE_UPDATED audit 정확히 1건
func TestScoreAtomicity_UpdateCreatesAuditRow(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	scoreID := insertDraftScore(t, db, ctx, "AX-AUDIT-UPDATE-01")
	require.Equal(t, 1, auditScoreCount(t, db, scoreID, "SCORE_CREATED"),
		"insertDraftScore → SCORE_CREATED 1건 (전제)")

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	newVal := 88.0
	require.NoError(t, tx.UpdateScore(ctx, scoreID, ScoreUpdate{ScoreValue: &newVal}))
	require.NoError(t, tx.Commit(ctx))

	assert.Equal(t, 1, auditScoreCount(t, db, scoreID, "SCORE_UPDATED"),
		"UpdateScore+Commit → SCORE_UPDATED audit COUNT=1 (DC-UBI-002.2)")

	var got float64
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT score_value FROM scores WHERE id=$1`, scoreID).Scan(&got))
	assert.InDelta(t, 88.0, got, 0.001, "DRAFT in-place 갱신 (D4)")
}

// TestScoreAtomicity_NoOpUpdateNoAuditRow DC-UBI-002:
// 빈 ScoreUpdate(변경 없음)는 no-op — audit 행을 생성하지 않는다
func TestScoreAtomicity_NoOpUpdateNoAuditRow(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	scoreID := insertDraftScore(t, db, ctx, "AX-AUDIT-NOOP-01")

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.UpdateScore(ctx, scoreID, ScoreUpdate{}), "빈 갱신 = no-op")
	require.NoError(t, tx.Commit(ctx))

	assert.Equal(t, 0, auditScoreCount(t, db, scoreID, "SCORE_UPDATED"),
		"no-op UpdateScore → SCORE_UPDATED audit 0건 (상태 무변경 시 감사 미생성)")
}

// ── DC-001-E1: 정상 경로 — scores + audit_logs 양쪽 행 존재 (동일 TX) ───────────

// TestScoreAtomicity_SuccessCommit REQ-SCORE-UBI-002 / DC-001-E1:
// InsertScore(recorder 자동) → Commit → scores/audit_logs 양쪽에 행 존재
func TestScoreAtomicity_SuccessCommit(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)

	scoreID, err := tx.InsertScore(ctx, "AX-ATOMICITY-02", nil, "raw", 90.0, nil, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	var cnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scores WHERE id=$1`, scoreID).Scan(&cnt))
	assert.Equal(t, 1, cnt, "커밋 후 scores 행 존재")

	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE resource_id=$1`, scoreID).Scan(&cnt))
	assert.Equal(t, 1, cnt, "커밋 후 audit_logs 행 존재 (동일 TX 원자성)")
}

// ── DC-004-U1: audit 장애 → 양방향 rollback + errors.Is 래핑 ────────────────────

// TestScoreAtomicity_AuditFaultBidirectionalRollback DC-004-U1 / EC-02:
// audit_logs INSERT를 DB CHECK(false)로 강제 실패시키면 InsertScore가
// stderrors.ErrScoreAuditWriteFailed를 래핑한 에러를 반환하고
// scores/audit_logs 양쪽에 행이 남지 않는다 (양방향 rollback). goroutine 누출 0.
func TestScoreAtomicity_AuditFaultBidirectionalRollback(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	const evalItemID = "AX-ATOMICITY-FAULT-01"

	// fault injection: audit_logs INSERT를 무조건 거부하는 CHECK 제약 추가
	_, err := db.pool.Exec(ctx,
		`ALTER TABLE audit_logs ADD CONSTRAINT ci_fail_score CHECK (false)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.pool.Exec(context.Background(),
			`ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS ci_fail_score`)
	})

	tx, beginErr := db.store.BeginScoreTx(ctx)
	require.NoError(t, beginErr)

	// InsertScore는 내부적으로 recorder를 통해 audit를 호출 → audit CHECK(false) 위반 →
	// InsertScore가 에러를 반환해야 함 (audit 실패 전파, DC-004-U1 cond3/4)
	_, insErr := tx.InsertScore(ctx, evalItemID, nil, "raw", 70.0, nil, nil)
	require.Error(t, insErr,
		"audit_logs CHECK(false) → InsertScore가 audit 실패를 에러로 전파해야 함 (DC-004-U1)")
	assert.ErrorIs(t, insErr, stderrors.ErrScoreAuditWriteFailed,
		"InsertScore 에러는 audit-insertion 실패를 래핑 (DC-004-U1 cond4 errors.Is)")

	// 핸들러 계층이 에러 시 Rollback (deferred Rollback 모의) → 양방향 취소
	require.NoError(t, tx.Rollback(ctx))

	var scoreCnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scores WHERE evaluation_item_id=$1`, evalItemID).Scan(&scoreCnt))
	assert.Equal(t, 0, scoreCnt, "audit 장애 → scores 행 0건 (DC-004-U1.1 양방향 rollback)")

	var auditCnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE resource_type='score'`).Scan(&auditCnt))
	assert.Equal(t, 0, auditCnt, "audit 장애 → audit_logs 행 0건 (DC-004-U1.2 partial audit 방지)")
}

// ── goroutine 누출 검사 ─────────────────────────────────────────────────────────

// TestScoreAtomicity_NoGoroutineLeak TH-03:
// TX commit/rollback 후 goroutine 누출 없음 (goleak)
func TestScoreAtomicity_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx1, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	_, err = tx1.InsertScore(ctx, "AX-GOROUTINE-01", nil, "raw", 70.0, nil, nil)
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	tx2, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	_, err = tx2.InsertScore(ctx, "AX-GOROUTINE-02", nil, "raw", 65.0, nil, nil)
	require.NoError(t, err)
	require.NoError(t, tx2.Rollback(ctx))
}

// TestScoreAtomicity_RollbackAfterCommitIsNoop TH-03:
// Commit 후 Rollback은 no-op (pgx 보장 — double-rollback 안전)
func TestScoreAtomicity_RollbackAfterCommitIsNoop(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	_, err = tx.InsertScore(ctx, "AX-NOOP-01", nil, "raw", 55.0, nil, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	rollbackErr := tx.Rollback(ctx)
	assert.NoError(t, rollbackErr, "Commit 후 Rollback은 no-op (ErrTxClosed 무시)")
}
