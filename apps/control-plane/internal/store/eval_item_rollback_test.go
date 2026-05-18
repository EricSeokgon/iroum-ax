//go:build integration

// eval_item_rollback_test.go — T-008: audit INSERT 실패 → 양방향 롤백 (DC-008, E-10/E-16)
// audit_logs에 CHECK(false) 제약을 주입하여 InsertAuditLog가 실패하면
// evaluation_items 행 + audit 행이 모두 롤백되어야 함 (all-or-nothing, REQ-EVALITEM-003-U1).
// create 경로 + update 경로 모두 검증. goroutine leak 0 (TH goleak).
package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// auditEvalItemCount 별도 풀 커넥션으로 details->>'eval_item_id'별 audit_logs 행 수 조회
func auditEvalItemCount(t *testing.T, db *testDB, itemID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE details->>'eval_item_id'=$1`, itemID).Scan(&n))
	return n
}

// TestEvalItem_AuditFailRollback_CreatePath DC-008.1/8.2/8.3/8.4 / E-10/E-16 / AC-003-3:
// 생성 경로 audit INSERT 실패 시 evaluation_items + audit 양방향 롤백, goroutine leak 0
func TestEvalItem_AuditFailRollback_CreatePath(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)

	db := setupTestDB(t)
	ctx := context.Background()
	rec := audit.NewRecorder(false)
	const itemID = "AX-FAULT-ITEM"

	// fault injection: audit_logs INSERT를 무조건 거부하는 CHECK 제약 추가
	_, err := db.pool.Exec(ctx, `ALTER TABLE audit_logs ADD CONSTRAINT ci_fail_ei CHECK (false)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.pool.Exec(context.Background(),
			`ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS ci_fail_ei`)
	})

	// 핸들러가 수행할 TX orchestration 재현:
	// BeginEvalItemTx → defer Rollback (즉시) → InsertEvalItem → RecordEvalItemCreated(audit, 실패) → Commit 미도달
	tx, beginErr := db.store.BeginEvalItemTx(ctx)
	require.NoError(t, beginErr)
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	_, insErr := tx.InsertEvalItem(ctx, itemID, nil, "fault", "", intptr(1), "AX.FAULT", nil, nil, nil)
	require.NoError(t, insErr, "InsertEvalItem 자체는 성공 (audit 단계에서 실패 예정)")

	// audit INSERT — CHECK(false)로 실패. Recorder는 store TX(EvalItemTx)를 AuditTx로 사용
	auditErr := rec.RecordEvalItemCreated(ctx, tx, itemID, "AX.FAULT", "", 1, "")
	require.Error(t, auditErr, "CHECK(false)로 audit INSERT가 실패해야 함 (DC-008.3 wrapped 에러)")

	// 핸들러는 audit 실패 시 Commit하지 않고 반환 → deferred Rollback이 양방향 취소
	require.NoError(t, tx.Rollback(ctx))
	committed = true

	assert.Equal(t, 0, countEvalItems(t, db, itemID), "evaluation_items 행 롤백 (0건, 부분커밋 없음)")
	assert.Equal(t, 0, auditEvalItemCount(t, db, itemID), "audit 행 미삽입 (0건)")
}

// TestEvalItem_AuditFailRollback_UpdatePath DC-008.5:
// 수정 경로 audit INSERT 실패 시 UpdateEvalItem 롤백, audit 부재, leak 0
func TestEvalItem_AuditFailRollback_UpdatePath(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)

	db := setupTestDB(t)
	ctx := context.Background()
	rec := audit.NewRecorder(false)
	const itemID = "AX-UPD-FAULT"

	// 사전: 항목을 정상 커밋 (status='ACTIVE')
	insertRoot(t, db, itemID, "AX.UPD.FAULT")

	// fault injection
	_, err := db.pool.Exec(ctx, `ALTER TABLE audit_logs ADD CONSTRAINT ci_fail_eiu CHECK (false)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.pool.Exec(context.Background(),
			`ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS ci_fail_eiu`)
	})

	tx, beginErr := db.store.BeginEvalItemTx(ctx)
	require.NoError(t, beginErr)
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	newStatus := "DEPRECATED"
	updErr := tx.UpdateEvalItem(ctx, itemID, EvalItemUpdate{Status: &newStatus})
	require.NoError(t, updErr, "UpdateEvalItem 자체는 성공 (audit 단계에서 실패 예정)")

	auditErr := rec.RecordEvalItemUpdated(ctx, tx, itemID, "AX.UPD.FAULT", "", 1, "")
	require.Error(t, auditErr, "CHECK(false)로 audit INSERT 실패")

	require.NoError(t, tx.Rollback(ctx))
	committed = true

	// status 변경이 롤백되어 'ACTIVE' 유지, audit 부재
	var status string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM evaluation_items WHERE id=$1`, itemID).Scan(&status))
	assert.Equal(t, "ACTIVE", status, "UpdateEvalItem 롤백 — status 'ACTIVE' 유지 (부분커밋 없음)")
	assert.Equal(t, 0, auditEvalItemCount(t, db, itemID), "audit 행 미삽입 (0건)")
}
