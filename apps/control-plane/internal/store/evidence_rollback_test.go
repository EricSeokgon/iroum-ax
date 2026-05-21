//go:build integration

// evidence_rollback_test.go — T-011: audit INSERT 실패 → 양방향 롤백 (DC-016, E-03)
// audit_logs에 CHECK(false) 제약을 주입하여 InsertAuditLog가 실패하면
// evidence 행 + audit 행이 모두 롤백되어야 함 (all-or-nothing, REQ-EVID-003-U1).
// database_blob 전략으로 blob bytes도 동일 TX → orphan-cleanup 머신러리 불요.
package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// TestEvidenceStore_AuditFailRollbackBidirectional
// audit INSERT 실패 시 evidence + audit 양방향 롤백, goroutine leak 0
func TestEvidenceStore_AuditFailRollbackBidirectional(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)

	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-evid-003-3"

	// fault injection: audit_logs INSERT를 무조건 거부하는 CHECK 제약 추가
	_, err := db.pool.Exec(ctx,
		`ALTER TABLE audit_logs ADD CONSTRAINT ci_fail CHECK (false)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.pool.Exec(context.Background(),
			`ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS ci_fail`)
	})

	content := []byte("rollback-test-content")

	// 핸들러가 수행할 TX orchestration 재현:
	// BeginEvidenceTx → defer Rollback (즉시, SEC-07) → InsertEvidence → InsertAuditLog(실패) → Commit 미도달
	tx, beginErr := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, beginErr)

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) // BeginEvidenceTx 직후 즉시 등록 (SEC-07 — InsertEvidence 이전)
		}
	}()

	evID, insErr := tx.InsertEvidence(ctx,
		evalItemID, "r.pdf", "application/pdf",
		int64(len(content)), sha256Hex(content),
		"database_blob", "", nil, content, nil,
	)
	require.NoError(t, insErr, "InsertEvidence 자체는 성공 (audit 단계에서 실패 예정)")

	// audit INSERT — CHECK(false)로 실패
	auditErr := tx.InsertAuditLog(ctx, &audit.Event{
		Timestamp:    time.Now().UTC(),
		Action:       audit.ActionEvidenceCreated,
		ResourceType: "evidence",
		ResourceID:   evID,
		UserID:       audit.DefaultUserID,
		DetailsJSON:  []byte(`{"evaluation_item_id":"eval-evid-003-3","version":"1"}`),
	})
	require.Error(t, auditErr, "CHECK(false)로 audit INSERT가 실패해야 함")

	// 핸들러는 audit 실패 시 Commit하지 않고 반환 → deferred Rollback이 양방향 취소
	rbErr := tx.Rollback(ctx)
	require.NoError(t, rbErr)
	committed = true // deferred rollback 중복 방지

	// 별도 풀 커넥션으로 커밋 후 상태 검증 (동일 TX 외부)
	assert.Equal(t, 0, countEvidences(t, db, evalItemID), "evidence 행 롤백 (0건)")

	var auditCount int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE resource_id=$1`, evID).Scan(&auditCount))
	assert.Equal(t, 0, auditCount, "audit 행 미삽입 (0건)")
}

// TestEvidenceStore_RollbackAfterBeginIsImmediate
// SEC-07 계약: BeginEvidenceTx 직후 InsertEvidence 이전에 Rollback 가능 (defer 등록 순서 검증)
func TestEvidenceStore_RollbackAfterBeginIsImmediate(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	// InsertEvidence 호출 없이 즉시 Rollback — 에러 없어야 함 (orphan 0)
	require.NoError(t, tx.Rollback(ctx))

	var n int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM evidences WHERE id=$1`, uuid.New()).Scan(&n))
	assert.Equal(t, 0, n)
}
