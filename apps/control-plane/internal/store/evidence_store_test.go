//go:build integration

// evidence_store_test.go — T-005: InsertEvidence + GetEvidenceByID 왕복
// DC-006 (store-level happy path), DC-008 (eval_item stub, FK 미강제),
// DC-012-gap (ErrEvidenceNotFound 센티널, E-10)
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestEvidenceStore -v -count=1
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// TestEvidenceStore_CreateHappyPath InsertEvidence → GetEvidenceByID 왕복
// version=1, previous_version_id=NULL, file_content BYTEA round-trip, storage_strategy 영속
func TestEvidenceStore_CreateHappyPath(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	content := []byte("KEPCO 경영평가 증빙 본문 — 데이터 주권 PoC")
	wantHash := sha256Hex(content)

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err, "BeginEvidenceTx 실패")

	id, err := tx.InsertEvidence(ctx,
		"eval-001-1", "evidence.pdf", "application/pdf",
		int64(len(content)), wantHash,
		"database_blob", "db://evidences/placeholder",
		map[string]string{"k": "v"},
		content, nil,
	)
	require.NoError(t, err, "InsertEvidence 실패")
	require.NotEqual(t, uuid.Nil, id)

	got, err := tx.GetEvidenceByID(ctx, id)
	require.NoError(t, err, "GetEvidenceByID 실패")
	require.NoError(t, tx.Commit(ctx))

	assert.Equal(t, id, got.ID)
	assert.Equal(t, "eval-001-1", got.EvaluationItemID)
	assert.Equal(t, 1, got.Version, "신규 증빙은 version=1")
	assert.Nil(t, got.PreviousVersionID, "version=1은 previous_version_id=NULL")
	assert.Equal(t, wantHash, got.FileHashSHA256, "SHA-256 64자 소문자 hex 영속")
	assert.Len(t, got.FileHashSHA256, 64)
	assert.Equal(t, "database_blob", got.StorageStrategy)
	assert.Equal(t, "ACTIVE", got.Status)
	assert.Equal(t, int64(len(content)), got.FileSizeBytes)
	assert.Equal(t, "cli-anonymous", got.CreatedBy, "AC-EVID-UBI-003 기본값")

	// file_content BYTEA round-trip — 별도 풀 커넥션으로 직접 검증
	var stored []byte
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT file_content FROM evidences WHERE id=$1`, id).Scan(&stored))
	assert.Equal(t, content, stored, "file_content BYTEA byte-identical 왕복")
}

// TestEvidenceStore_EvalItemStubNoFKConstraint 임의 evaluation_item_id 허용 (FK 미강제)
// DC-008
func TestEvidenceStore_EvalItemStubNoFKConstraint(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	id, err := tx.InsertEvidence(ctx,
		"nonexistent-item-xyz-999", "x.pdf", "application/pdf",
		3, sha256Hex([]byte("abc")), "database_blob", "",
		nil, []byte("abc"), nil,
	)
	require.NoError(t, err, "존재하지 않는 evaluation_item_id도 FK 위반 없이 INSERT 성공해야 함")
	require.NoError(t, tx.Commit(ctx))

	var got string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT evaluation_item_id FROM evidences WHERE id=$1`, id).Scan(&got))
	assert.Equal(t, "nonexistent-item-xyz-999", got)

	// information_schema: evidences→evaluation_items FK 제약이 0건이어야 함
	var fkCount int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.referential_constraints
		 WHERE constraint_name LIKE '%evidences%' AND unique_constraint_name LIKE '%evaluation_items%'`,
	).Scan(&fkCount))
	assert.Equal(t, 0, fkCount, "evidences→evaluation_items FK 제약 0건 (경량 stub)")
}

// TestEvidenceStore_GetByID_NotFound 존재하지 않는 ID → ErrEvidenceNotFound 센티널
// DC-012-gap, E-10
func TestEvidenceStore_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	got, err := tx.GetEvidenceByID(ctx, uuid.New())
	assert.Nil(t, got)
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrEvidenceNotFound,
		"존재하지 않는 증빙 조회는 raw pgx.ErrNoRows가 아닌 ErrEvidenceNotFound를 래핑해야 함")
}
