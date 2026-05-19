//go:build integration

// score_supersede_test.go — T-012 D4 CONFIRMED 정정 경로
// SPEC-AX-SCORE-001:
//
//	DC-UBI-004 cond3/4: CONFIRMED 정정 = 신규 행 INSERT + 구 행 CONFIRMED→SUPERSEDED
//	  (동일 ScoreTx) — 2 score 행(구=SUPERSEDED, 신=CONFIRMED) + 2 audit 행
//	  (구: SCORE_UPDATED, 신: SCORE_CREATED), 물리 DELETE 0건
//	EC-13: 정정 = 신규 행 + SUPERSEDED 동일 TX
//	EC-ADD-1: 정정 도중 한쪽 실패 시 전체 TX rollback (원자성)
//
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestScore -v -count=1
package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// ── DC-UBI-004 cond3/4: CONFIRMED 정정 = 신규행 + 구행 SUPERSEDED (동일 TX) ──────

// TestScoreSupersede_ConfirmedCorrectionCreatesNewRowAndSupersedesOld
// DC-UBI-004.3/4 / EC-13:
// CONFIRMED 행 정정 → SupersedeAndReplaceScore →
//   - 동일 evaluation_item_id의 scores 행 2건 (구=SUPERSEDED, 신=CONFIRMED)
//   - audit_logs score 행 2건 (구: SCORE_UPDATED, 신: SCORE_CREATED)
//   - 물리 DELETE 0건 (구 행이 그대로 존재, 상태만 SUPERSEDED)
func TestScoreSupersede_ConfirmedCorrectionCreatesNewRowAndSupersedesOld(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	const evalItemID = "AX-SUPERSEDE-01"

	// 구 행: DRAFT → CONFIRMED
	// 주의: insertDraftScore → SCORE_CREATED 1건, confirmScore(UpdateScore CONFIRMED)
	//       → SCORE_UPDATED 1건 (F1 — 실 상태 변경마다 audit). 이는 정상 동작이므로
	//       정정 TX가 추가로 만드는 audit는 baseline 대비 delta로 측정한다.
	oldID := insertDraftScore(t, db, ctx, evalItemID)
	confirmScore(t, db, ctx, oldID)

	// 정정 TX 직전 baseline: 구 행 SCORE_UPDATED 수 (confirm 단계의 1건 포함)
	var baseOldUpdated int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs
		 WHERE resource_type='score' AND action='SCORE_UPDATED' AND resource_id=$1`,
		oldID).Scan(&baseOldUpdated))

	// 정정: 신규 행 INSERT (corrected score_value) + 구 행 CONFIRMED→SUPERSEDED, 동일 TX
	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	correctedValue := 91.5
	correctedWeight := 0.45
	newID, err := tx.SupersedeAndReplaceScore(ctx, oldID, correctedValue, &correctedWeight, nil)
	require.NoError(t, err, "CONFIRMED 정정 성공 (D4)")
	require.NotEqual(t, oldID, newID, "신규 행 id는 구 행과 달라야 함")
	require.NoError(t, tx.Commit(ctx))

	// scores 행 2건 (구=SUPERSEDED, 신=CONFIRMED)
	var totalRows int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scores WHERE evaluation_item_id=$1`, evalItemID).Scan(&totalRows))
	assert.Equal(t, 2, totalRows, "정정 후 scores 행 2건 (DC-UBI-004.3 — 구+신)")

	var oldStatus, newStatus string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM scores WHERE id=$1`, oldID).Scan(&oldStatus))
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM scores WHERE id=$1`, newID).Scan(&newStatus))
	assert.Equal(t, "SUPERSEDED", oldStatus, "구 행 status=SUPERSEDED (DC-UBI-004.3)")
	assert.Equal(t, "CONFIRMED", newStatus, "신규 행 status=CONFIRMED (DC-UBI-004.3)")

	// 신규 행에 corrected 값이 반영됨
	var gotVal, gotWeight float64
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT score_value, weight FROM scores WHERE id=$1`, newID).Scan(&gotVal, &gotWeight))
	assert.InDelta(t, correctedValue, gotVal, 0.001, "신규 행 corrected score_value")
	assert.InDelta(t, correctedWeight, gotWeight, 0.0001, "신규 행 corrected weight")

	// DC-UBI-004.4: 정정 TX가 추가로 생성한 audit = 구 SCORE_UPDATED 1건 (delta) +
	//               신 SCORE_CREATED 1건.

	// 구 행 SCORE_UPDATED delta = 1 (CONFIRMED→SUPERSEDED 전이, baseline 제외)
	var afterOldUpdated int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs
		 WHERE resource_type='score' AND action='SCORE_UPDATED' AND resource_id=$1`,
		oldID).Scan(&afterOldUpdated))
	assert.Equal(t, 1, afterOldUpdated-baseOldUpdated,
		"정정 TX → 구 행 SCORE_UPDATED delta=1 (CONFIRMED→SUPERSEDED, DC-UBI-004.4)")

	// 신규 행 SCORE_CREATED 정확히 1건 (신규 resource_id는 정정 TX 이전 audit 없음)
	var newAuditCnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs
		 WHERE resource_type='score' AND action='SCORE_CREATED' AND resource_id=$1`,
		newID).Scan(&newAuditCnt))
	assert.Equal(t, 1, newAuditCnt,
		"신규 행 → SCORE_CREATED audit 1건 (DC-UBI-004.4)")

	// 정정 TX 순수 산출 = 2 audit 행 (구 SCORE_UPDATED delta 1 + 신 SCORE_CREATED 1)
	assert.Equal(t, 2, (afterOldUpdated-baseOldUpdated)+newAuditCnt,
		"정정 TX → 정확히 2 audit 행 (구 SCORE_UPDATED + 신 SCORE_CREATED, DC-UBI-004.4)")
}

// TestScoreSupersede_NoPhysicalDelete EC-14 / DC-UBI-004:
// 정정 후 구 행이 물리적으로 존재 (SELECT 가능) — DELETE 0건
func TestScoreSupersede_NoPhysicalDelete(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	oldID := insertDraftScore(t, db, ctx, "AX-SUPERSEDE-NODEL-01")
	confirmScore(t, db, ctx, oldID)

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	_, err = tx.SupersedeAndReplaceScore(ctx, oldID, 77.0, nil, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	// 구 행이 물리적으로 그대로 존재 (DELETE 0건, 상태만 SUPERSEDED)
	old, err := func() (*Score, error) {
		rtx, e := db.store.BeginScoreTx(ctx)
		require.NoError(t, e)
		defer func() { _ = rtx.Rollback(ctx) }()
		return rtx.GetScoreByID(ctx, oldID)
	}()
	require.NoError(t, err, "구 행은 물리 삭제되지 않음 (EC-14 물리 DELETE 0)")
	assert.Equal(t, "SUPERSEDED", old.Status, "구 행 상태만 SUPERSEDED로 전이 (물리 보존)")
}

// TestScoreSupersede_RejectsNonConfirmed D4:
// CONFIRMED 아닌 행(DRAFT)에 SupersedeAndReplaceScore → 구조화 에러, DB 무변경
func TestScoreSupersede_RejectsNonConfirmed(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	const evalItemID = "AX-SUPERSEDE-DRAFT-01"
	draftID := insertDraftScore(t, db, ctx, evalItemID) // DRAFT 상태

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.SupersedeAndReplaceScore(ctx, draftID, 50.0, nil, nil)
	require.Error(t, err, "DRAFT 행 정정 시도 → 구조화 에러")
	assert.ErrorIs(t, err, stderrors.ErrScoreNotConfirmed,
		"정정 대상은 CONFIRMED여야 함 (D4 — ErrScoreNotConfirmed)")
	_ = tx.Rollback(ctx)

	// DB 무변경: scores 행 1건 (구 DRAFT), 상태 DRAFT 유지
	var cnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scores WHERE evaluation_item_id=$1`, evalItemID).Scan(&cnt))
	assert.Equal(t, 1, cnt, "정정 거부 → 신규 행 생성 안 됨 (DB 무변경)")
	var st string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM scores WHERE id=$1`, draftID).Scan(&st))
	assert.Equal(t, "DRAFT", st, "DRAFT 행 상태 불변")
}

// TestScoreSupersede_AtomicityFailureRollsBackEntireTX EC-ADD-1:
// 정정 도중 audit 장애 주입(CHECK false) → 전체 TX rollback →
// scores 행 1건(구 행 여전히 CONFIRMED), audit score 행 무증가
func TestScoreSupersede_AtomicityFailureRollsBackEntireTX(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	const evalItemID = "AX-SUPERSEDE-ATOMIC-01"
	oldID := insertDraftScore(t, db, ctx, evalItemID)
	confirmScore(t, db, ctx, oldID)

	// audit_logs INSERT를 무조건 거부 → 정정 TX 내 audit 단계에서 실패.
	// NOT VALID: 기존 행(insertDraftScore/confirmScore 단계의 audit)을 재검증하지 않고
	// 신규 INSERT(정정 TX의 audit)에만 CHECK(false)를 적용한다.
	_, err := db.pool.Exec(ctx,
		`ALTER TABLE audit_logs ADD CONSTRAINT ci_fail_sup CHECK (false) NOT VALID`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.pool.Exec(context.Background(),
			`ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS ci_fail_sup`)
	})

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)

	_, supErr := tx.SupersedeAndReplaceScore(ctx, oldID, 88.0, nil, nil)
	require.Error(t, supErr, "audit 장애 → 정정 실패 (EC-ADD-1)")
	require.NoError(t, tx.Rollback(ctx))

	// 전체 TX rollback: scores 행 1건 (구 행, 여전히 CONFIRMED)
	var cnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scores WHERE evaluation_item_id=$1`, evalItemID).Scan(&cnt))
	assert.Equal(t, 1, cnt, "정정 실패 → 신규 행 미생성, 구 행 1건만 (EC-ADD-1)")
	var st string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM scores WHERE id=$1`, oldID).Scan(&st))
	assert.Equal(t, "CONFIRMED", st, "정정 실패 → 구 행 CONFIRMED 유지 (SUPERSEDED 미전이)")
}
