//go:build integration

// evidence_storage_strategy_test.go — DC-017: storage_strategy 열거값 강제
// CHECK 제약이 열거 외 값/NULL을 거부하고, database_blob location이
// db://evidences/<UUID> 형식인지 검증한다.
package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvidenceStorage_StrategyEnumEnforced
func TestEvidenceStorage_StrategyEnumEnforced(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// (1) filesystem 전략 영속
	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	fsID, err := tx.InsertEvidence(ctx,
		"eval-strat-fs", "f.pdf", "application/pdf",
		3, sha256Hex([]byte("abc")), "filesystem", "/srv/evidence/x",
		nil, []byte("abc"), nil,
	)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	var strat string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT storage_strategy FROM evidences WHERE id=$1`, fsID).Scan(&strat))
	assert.Equal(t, "filesystem", strat)

	// (2) 열거 외 값 직접 INSERT → CHECK 제약 위반 (23514)
	_, err = db.pool.Exec(ctx,
		`INSERT INTO evidences (id, evaluation_item_id, file_name, storage_strategy)
		 VALUES ($1, 'eval-bad', 'b.pdf', 'external_s3')`, uuid.New())
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23514", pgErr.Code, "check_constraint_violation (23514)")

	// (3) NULL storage_strategy → NOT NULL 위반
	_, err = db.pool.Exec(ctx,
		`INSERT INTO evidences (id, evaluation_item_id, file_name, storage_strategy)
		 VALUES ($1, 'eval-null', 'n.pdf', NULL)`, uuid.New())
	require.Error(t, err, "NULL storage_strategy 거부")

	// (4) database_blob storage_location = db://evidences/<UUID> 형식
	tx2, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	dbID, err := tx2.InsertEvidence(ctx,
		"eval-strat-db", "d.pdf", "application/pdf",
		3, sha256Hex([]byte("xyz")), "database_blob", "db://evidences/"+uuid.New().String(),
		nil, []byte("xyz"), nil,
	)
	require.NoError(t, err)
	require.NoError(t, tx2.Commit(ctx))
	var loc string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT storage_location FROM evidences WHERE storage_strategy='database_blob' AND id=$1`,
		dbID).Scan(&loc))
	assert.Regexp(t, regexp.MustCompile(`^db://evidences/[0-9a-f-]{36}$`), loc)
}
