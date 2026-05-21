//go:build integration

// evidence_errorpaths_test.go — T-019 커버리지 보강: 에러 분기 경로 테스트
// InsertEvidence SQLSTATE 래핑(pgErrorOf), UpdateEvidenceBodyColumn 성공/거부/미발견,
// MarkSuperseded 미발견, GetLatestVersionByEvalItem 신규(nil,nil), Commit/Rollback 경로
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// TestEvidence_InsertEvidence_SQLStateWrapped
// previous_version_id가 존재하지 않는 UUID → InsertEvidence 직전 버전 조회 실패 경로
func TestEvidence_InsertEvidence_SQLStateWrapped(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	bogusPrev := uuid.New()
	_, insErr := tx.InsertEvidence(ctx,
		"eval-err-1", "x.pdf", "application/pdf",
		3, sha256Hex([]byte("abc")), "database_blob", "",
		nil, []byte("abc"), &bogusPrev,
	)
	require.Error(t, insErr, "존재하지 않는 previous_version_id → 직전 버전 조회 실패")
	assert.ErrorIs(t, insErr, stderrors.ErrEvidenceNotFound)
}

// TestEvidence_InsertEvidence_InvalidStorageStrategy
// 열거 외 storage_strategy → CHECK 제약 위반 → pgErrorOf SQLSTATE 23514 래핑 경로
func TestEvidence_InsertEvidence_InvalidStorageStrategy(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, insErr := tx.InsertEvidence(ctx,
		"eval-err-2", "x.pdf", "application/pdf",
		3, sha256Hex([]byte("abc")), "bad_strategy", "",
		nil, []byte("abc"), nil,
	)
	require.Error(t, insErr, "열거 외 storage_strategy → CHECK 위반")
	assert.Contains(t, insErr.Error(), "SQLSTATE 23514",
		"pgErrorOf가 SQLSTATE를 에러 메시지에 포함")
}

// TestEvidence_UpdateBodyColumn_AllBranches
// invalid column / 미발견 / no-successor 성공 분기 (UpdateEvidenceBodyColumn 커버리지)
func TestEvidence_UpdateBodyColumn_AllBranches(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-err-3"

	id := insertEvidenceV1(t, db, evalItemID, []byte("body-v1"))

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	pgTx := tx.(*PgEvidenceTx)

	// (1) 허용되지 않은 컬럼 → 에러
	err = pgTx.UpdateEvidenceBodyColumn(ctx, id, "id", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "허용되지 않은 컬럼")

	// (2) successor 없음 → 본문 컬럼 갱신 성공 (화이트리스트 컬럼)
	err = pgTx.UpdateEvidenceBodyColumn(ctx, id, "content_type", "text/plain")
	require.NoError(t, err, "successor 없으면 본문 컬럼 갱신 허용")
	got, err := pgTx.GetEvidenceByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", got.ContentType)

	// (3) 존재하지 않는 ID → ErrEvidenceNotFound
	err = pgTx.UpdateEvidenceBodyColumn(ctx, uuid.New(), "content_type", "y")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrEvidenceNotFound)
}

// TestEvidence_MarkSuperseded_NotFound 존재하지 않는 ID → ErrEvidenceNotFound
func TestEvidence_MarkSuperseded_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.MarkSuperseded(ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrEvidenceNotFound)
}

// TestEvidence_GetLatestVersion_NewItemReturnsNilNil 신규 item → (nil, nil)
func TestEvidence_GetLatestVersion_NewItemReturnsNilNil(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	got, err := tx.GetLatestVersionByEvalItem(ctx, "never-seen-item")
	require.NoError(t, err, "신규 item은 에러 없이 (nil,nil)")
	assert.Nil(t, got)
}

// TestEvidence_CommitRollback_Idempotent Commit 후 Rollback no-op + ListByEvalItem 빈 슬라이스
func TestEvidence_CommitRollback_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)

	list, err := tx.ListEvidenceByEvalItem(ctx, "empty-item")
	require.NoError(t, err)
	assert.Empty(t, list, "빈 item은 빈 슬라이스")

	require.NoError(t, tx.Commit(ctx))
	// Commit 후 Rollback은 no-op (pgx.ErrTxClosed 흡수)
	assert.NoError(t, tx.Rollback(ctx))
}

// TestEvidence_OperationsOnClosedTx 커밋된 TX에서 후속 연산 → 에러 래핑 분기 커버
// Commit/MarkSuperseded/ListEvidenceByEvalItem/InsertEvidence의 err != nil 분기
func TestEvidence_OperationsOnClosedTx(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx)) // TX 종료

	// 종료된 TX에서 연산 → pgx.ErrTxClosed → 각 메서드 에러 래핑 경로
	_, insErr := tx.InsertEvidence(ctx, "ei", "f", "ct", 1, "h", "database_blob", "", nil, []byte("x"), nil)
	assert.Error(t, insErr, "닫힌 TX InsertEvidence → 에러")

	listErr := func() error { _, e := tx.ListEvidenceByEvalItem(ctx, "ei"); return e }()
	assert.Error(t, listErr, "닫힌 TX ListEvidenceByEvalItem → 에러")

	msErr := tx.MarkSuperseded(ctx, uuid.New())
	assert.Error(t, msErr, "닫힌 TX MarkSuperseded → 에러")

	// 두 번째 Commit → 에러 래핑 분기 (evidence.go Commit err != nil)
	c2 := tx.Commit(ctx)
	assert.Error(t, c2, "이미 커밋된 TX 재커밋 → 에러 래핑")

	pgTx := tx.(*PgEvidenceTx)
	upErr := pgTx.UpdateEvidenceBodyColumn(ctx, uuid.New(), "content_type", "x")
	assert.Error(t, upErr, "닫힌 TX UpdateEvidenceBodyColumn successor 조회 → 에러")
}
