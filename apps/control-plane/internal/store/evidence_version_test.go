//go:build integration

// evidence_version_test.go — T-008/T-009: 버전 관리 + 불변성
// DC-011 (재업로드→v2 체이닝), DC-012 (이전 버전 조회), DC-013 (3-deep 체인 + mutation guard)
// DC-005 (이전 버전 byte-identical 보존), E-01, E-07
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// reuploadNextVersion 단일 TX로 다음 버전 생성 (resolve→insert→supersede→commit) 후 새 ID 반환
func reuploadNextVersion(t *testing.T, db *testDB, evalItemID string, content []byte) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	latest, err := tx.GetLatestVersionByEvalItem(ctx, evalItemID)
	require.NoError(t, err)
	var prevID *uuid.UUID
	if latest != nil {
		id := latest.ID
		prevID = &id
	}
	newID, err := tx.InsertEvidence(ctx,
		evalItemID, "v.pdf", "application/pdf",
		int64(len(content)), sha256Hex(content),
		"database_blob", "", nil, content, prevID,
	)
	require.NoError(t, err)
	if latest != nil {
		require.NoError(t, tx.MarkSuperseded(ctx, latest.ID))
	}
	require.NoError(t, tx.Commit(ctx))
	return newID
}

// TestEvidenceVersion_ReuploadCreatesV2 재업로드 → version=2, prev 체이닝, 직전 SUPERSEDED
// DC-011, E-01 (부분)
func TestEvidenceVersion_ReuploadCreatesV2(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-002-1"

	ev1ID := insertEvidenceV1(t, db, evalItemID, []byte("v1"))
	ev2ID := reuploadNextVersion(t, db, evalItemID, []byte("v2"))

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	ev2, err := tx.GetEvidenceByID(ctx, ev2ID)
	require.NoError(t, err)
	assert.Equal(t, 2, ev2.Version)
	require.NotNil(t, ev2.PreviousVersionID)
	assert.Equal(t, ev1ID, *ev2.PreviousVersionID, "v2.previous_version_id == v1.id")
	assert.Equal(t, "ACTIVE", ev2.Status)

	ev1, err := tx.GetEvidenceByID(ctx, ev1ID)
	require.NoError(t, err)
	assert.Equal(t, "SUPERSEDED", ev1.Status, "직전 버전 status ACTIVE→SUPERSEDED")

	assert.Equal(t, 2, countEvidences(t, db, evalItemID), "물리 삭제 0건 — 2행 모두 보존")
}

// TestEvidenceVersion_PriorVersionRetrievable 이전 버전 조회 가능 + version DESC 정렬
// DC-012
func TestEvidenceVersion_PriorVersionRetrievable(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-002-2"

	ev1ID := insertEvidenceV1(t, db, evalItemID, []byte("v1"))
	reuploadNextVersion(t, db, evalItemID, []byte("v2"))

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	got1, err := tx.GetEvidenceByID(ctx, ev1ID)
	require.NoError(t, err)
	require.NotNil(t, got1, "이전 버전 GetEvidenceByID는 non-nil")

	list, err := tx.ListEvidenceByEvalItem(ctx, evalItemID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, 2, list[0].Version, "version DESC — 첫 요소가 v2")
	assert.Equal(t, 1, list[1].Version, "두 번째가 v1")
}

// TestEvidenceVersion_ThreeDeepChainIntact 3-deep 체인 + 본문 immutable (mutation guard)
// DC-013, E-01, E-07
func TestEvidenceVersion_ThreeDeepChainIntact(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-002-3"

	ev1ID := insertEvidenceV1(t, db, evalItemID, []byte("v1"))
	ev2ID := reuploadNextVersion(t, db, evalItemID, []byte("v2"))
	ev3ID := reuploadNextVersion(t, db, evalItemID, []byte("v3"))

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	ev3, err := tx.GetEvidenceByID(ctx, ev3ID)
	require.NoError(t, err)
	ev2, err := tx.GetEvidenceByID(ctx, ev2ID)
	require.NoError(t, err)
	ev1, err := tx.GetEvidenceByID(ctx, ev1ID)
	require.NoError(t, err)

	require.NotNil(t, ev3.PreviousVersionID)
	assert.Equal(t, ev2ID, *ev3.PreviousVersionID, "v3→v2")
	require.NotNil(t, ev2.PreviousVersionID)
	assert.Equal(t, ev1ID, *ev2.PreviousVersionID, "v2→v1")
	assert.Nil(t, ev1.PreviousVersionID, "v1→NULL (체인 시작)")

	h1 := ev1.FileHashSHA256

	// mutation guard: successor가 있는 v1 본문 컬럼 UPDATE 시도 → 거부 (SQL 미실행)
	guardTx, ok := tx.(*PgEvidenceTx)
	require.True(t, ok, "PgEvidenceTx 타입이어야 mutation guard 호출 가능")
	err = guardTx.UpdateEvidenceBodyColumn(ctx, ev1ID, "file_hash_sha256", "tampered-value")
	require.Error(t, err, "successor 존재 시 본문 컬럼 UPDATE는 거부되어야 함")
	assert.ErrorIs(t, err, stderrors.ErrEvidenceImmutable)

	// v1 file_hash_sha256 불변 확인
	ev1after, err := tx.GetEvidenceByID(ctx, ev1ID)
	require.NoError(t, err)
	assert.Equal(t, h1, ev1after.FileHashSHA256, "mutation guard 거부 후 file_hash_sha256 불변")
}

// TestEvidence_PriorVersionImmutable 이전 버전 byte-identical 보존 + SUPERSEDED 전이 허용
// DC-005, E-07
func TestEvidence_PriorVersionImmutable(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-ubi-004"

	v1Content := []byte("v1-immutable-body")
	ev1ID := insertEvidenceV1(t, db, evalItemID, v1Content)

	// v2 생성 전 v1 스냅샷
	tx0, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	snap, err := tx0.GetEvidenceByID(ctx, ev1ID)
	require.NoError(t, err)
	require.NoError(t, tx0.Rollback(ctx))

	reuploadNextVersion(t, db, evalItemID, []byte("v2-content"))

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	ev1, err := tx.GetEvidenceByID(ctx, ev1ID)
	require.NoError(t, err)
	require.NotNil(t, ev1)

	assert.Equal(t, snap.FileName, ev1.FileName)
	assert.Equal(t, snap.FileSizeBytes, ev1.FileSizeBytes)
	assert.Equal(t, snap.FileHashSHA256, ev1.FileHashSHA256)
	assert.Equal(t, snap.ContentType, ev1.ContentType)
	assert.Equal(t, snap.Metadata, ev1.Metadata)
	assert.Equal(t, "SUPERSEDED", ev1.Status, "status 전이는 허용 (본문 외)")

	var n int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM evidences WHERE id=$1`, ev1ID).Scan(&n))
	assert.Equal(t, 1, n, "DELETE 미발행 — v1 행 보존")
}
