//go:build integration

// score_migration_test.go — T-002: 0004_score_tables.sql DDL 검증
// SPEC-AX-SCORE-001 DC-001-S1 (마이그레이션 정확성):
//   - scores.id = uuid (uuid_generate_v4() DEFAULT)
//   - evaluation_item_id = character varying(64), NOT NULL
//   - evidence_id = uuid, nullable
//   - FK 없음 (evaluation_item_id / evidence_id 모두 FK-less)
//   - level CHECK {raw,item,category}
//   - status CHECK {DRAFT,CONFIRMED,SUPERSEDED}
//   - grade CHECK NULL OR {S,A,B,C,D}
//   - scores_evaluation_item_id_idx, scores_evidence_id_idx, scores_level_idx 인덱스
//   - grade_thresholds 테이블: PK(scope,letter), letter CHECK, boundary_rule CHECK
//
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestScoreMigration -v -count=1
package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applyMigration0004 repo 루트의 .moai/db/schema/migrations/0004_score_tables.sql 을
// testcontainers DB에 멱등 재적용한다 (0001→0003 base는 setupTestDB schema.sql이 이미 부트스트랩).
func applyMigration0004(t *testing.T, db *testDB) {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err, "repo 루트 조회 실패")
	root := strings.TrimSpace(string(out))

	sqlPath := filepath.Join(root, ".moai/db/schema/migrations/0004_score_tables.sql")
	sqlBytes, err := os.ReadFile(sqlPath) //nolint:gosec // 테스트 고정 경로, 사용자 입력 아님
	require.NoError(t, err, "0004 마이그레이션 파일 읽기 실패: %s", sqlPath)

	// 멱등 SQL (CREATE TABLE IF NOT EXISTS / DO $$ EXCEPTION) — 재적용해도 무해
	_, err = db.pool.Exec(context.Background(), string(sqlBytes))
	require.NoError(t, err, "0004 마이그레이션 적용 실패")
}

// TestScoreMigration_FileExists DC-001-S1.1: 마이그레이션 파일 존재
func TestScoreMigration_FileExists(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err)
	root := strings.TrimSpace(string(out))
	p := filepath.Join(root, ".moai/db/schema/migrations/0004_score_tables.sql")
	_, statErr := os.Stat(p)
	require.NoError(t, statErr, "0004_score_tables.sql 이 존재해야 함: %s", p)
}

// TestScoreMigration_ScoresIDColumnType DC-001-S1 / D2:
// scores.id = uuid (uuid_generate_v4() DEFAULT) — audit resource_id 직접 대입 계약
func TestScoreMigration_ScoresIDColumnType(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	var dataType string
	err := db.pool.QueryRow(ctx,
		`SELECT data_type
		 FROM information_schema.columns
		 WHERE table_name='scores' AND column_name='id'`,
	).Scan(&dataType)
	require.NoError(t, err, "scores.id 컬럼이 존재해야 함")
	assert.Equal(t, "uuid", dataType, "scores.id = uuid (D2: audit resource_id 직접 대입)")
}

// TestScoreMigration_EvaluationItemIDColumnType DC-001-S1 / EVAL-ITEM-001 §1.4:
// evaluation_item_id = character varying(64), NOT NULL, FK-less
func TestScoreMigration_EvaluationItemIDColumnType(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	var dataType string
	var maxLen int
	var isNullable string
	err := db.pool.QueryRow(ctx,
		`SELECT data_type, character_maximum_length, is_nullable
		 FROM information_schema.columns
		 WHERE table_name='scores' AND column_name='evaluation_item_id'`,
	).Scan(&dataType, &maxLen, &isNullable)
	require.NoError(t, err, "scores.evaluation_item_id 컬럼이 존재해야 함")
	assert.Equal(t, "character varying", dataType, "evaluation_item_id는 VARCHAR (UUID 아님 — EVAL-ITEM-001 §1.4 타입 호환)")
	assert.Equal(t, 64, maxLen, "evaluation_item_id VARCHAR(64) — 평가항목 id 최대 길이 정합")
	assert.Equal(t, "NO", isNullable, "evaluation_item_id NOT NULL")
}

// TestScoreMigration_EvidenceIDColumnType DC-001-S1 / EVID-001 compat:
// evidence_id = uuid, nullable (FK-less stub)
func TestScoreMigration_EvidenceIDColumnType(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	var dataType, isNullable string
	err := db.pool.QueryRow(ctx,
		`SELECT data_type, is_nullable
		 FROM information_schema.columns
		 WHERE table_name='scores' AND column_name='evidence_id'`,
	).Scan(&dataType, &isNullable)
	require.NoError(t, err, "scores.evidence_id 컬럼이 존재해야 함")
	assert.Equal(t, "uuid", dataType, "evidence_id = uuid (EVID-001 타입 호환)")
	assert.Equal(t, "YES", isNullable, "evidence_id nullable (증빙 없는 점수 허용)")
}

// TestScoreMigration_FKAbsence AC-SCORE-BOUNDARY-1 (T-014 중복 검증 포함):
// scores 테이블에 FK 제약이 없어야 함 (evaluation_item_id/evidence_id 모두 FK-less)
func TestScoreMigration_FKAbsence(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	var n int
	err := db.pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM information_schema.table_constraints
		 WHERE table_name='scores' AND constraint_type='FOREIGN KEY'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n,
		"scores 테이블에 FK 제약 없음 (EVAL-ITEM-001 §1.4 + EVID-001 FK-less stub — AC-SCORE-BOUNDARY-1)")
}

// TestScoreMigration_CheckConstraints D1/D4/D3:
// level CHECK, status CHECK, grade CHECK 제약 존재 및 동작 검증
func TestScoreMigration_CheckConstraints(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	// level CHECK 존재
	var n int
	err := db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.check_constraints
		 WHERE constraint_name='scores_level_chk'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "scores_level_chk CHECK 제약이 존재해야 함 (D1)")

	// status CHECK 존재
	err = db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.check_constraints
		 WHERE constraint_name='scores_status_chk'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "scores_status_chk CHECK 제약이 존재해야 함 (D4)")

	// grade CHECK 존재
	err = db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.check_constraints
		 WHERE constraint_name='scores_grade_chk'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "scores_grade_chk CHECK 제약이 존재해야 함 (D3)")

	// level 열거 외 값 거부
	_, insErr := db.pool.Exec(ctx,
		`INSERT INTO scores (evaluation_item_id, level) VALUES ('AX-01', 'invalid_level')`)
	assert.Error(t, insErr, "level CHECK 위반 INSERT는 거부되어야 함 (D1)")

	// status 열거 외 값 거부
	_, insErr = db.pool.Exec(ctx,
		`INSERT INTO scores (evaluation_item_id, level, status) VALUES ('AX-02', 'raw', 'NOT_STATUS')`)
	assert.Error(t, insErr, "status CHECK 위반 INSERT는 거부되어야 함 (D4)")

	// grade 열거 외 값 거부 (NULL은 허용)
	_, insErr = db.pool.Exec(ctx,
		`INSERT INTO scores (evaluation_item_id, level, grade) VALUES ('AX-03', 'raw', 'Z')`)
	assert.Error(t, insErr, "grade CHECK 위반 INSERT는 거부되어야 함 (D3)")
}

// TestScoreMigration_Indexes DDL 인덱스 존재:
// scores_evaluation_item_id_idx, scores_evidence_id_idx, scores_level_idx
func TestScoreMigration_Indexes(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	for _, idx := range []string{
		"scores_evaluation_item_id_idx",
		"scores_evidence_id_idx",
		"scores_level_idx",
	} {
		var cnt int
		err := db.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM pg_indexes WHERE tablename='scores' AND indexname=$1`,
			idx,
		).Scan(&cnt)
		require.NoError(t, err)
		assert.Equal(t, 1, cnt, "인덱스 %s 가 존재해야 함", idx)
	}
}

// TestScoreMigration_GradeThresholdsTable D3:
// grade_thresholds 테이블: PK(scope,letter), letter CHECK, boundary_rule CHECK
func TestScoreMigration_GradeThresholdsTable(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	// 테이블 존재 확인
	var n int
	err := db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.tables
		 WHERE table_name='grade_thresholds'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "grade_thresholds 테이블이 존재해야 함 (D3)")

	// PK(scope,letter) 확인
	err = db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.table_constraints
		 WHERE table_name='grade_thresholds' AND constraint_type='PRIMARY KEY'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "grade_thresholds PK(scope,letter) 존재")

	// letter CHECK 존재
	err = db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.check_constraints
		 WHERE constraint_name='grade_thresholds_letter_chk'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "grade_thresholds_letter_chk 존재 (D3)")

	// boundary_rule CHECK 존재
	err = db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.check_constraints
		 WHERE constraint_name='grade_thresholds_boundary_rule_chk'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "grade_thresholds_boundary_rule_chk 존재 (D3)")

	// 유효 행 삽입 가능
	_, insErr := db.pool.Exec(ctx,
		`INSERT INTO grade_thresholds (scope, letter, min_value) VALUES ('default', 'S', 90)`)
	assert.NoError(t, insErr, "유효한 grade_thresholds 행 삽입 가능")

	// PK 중복 거부
	_, insErr = db.pool.Exec(ctx,
		`INSERT INTO grade_thresholds (scope, letter, min_value) VALUES ('default', 'S', 80)`)
	assert.Error(t, insErr, "PK(scope,letter) 중복 INSERT는 거부")

	// letter 열거 외 값 거부
	_, insErr = db.pool.Exec(ctx,
		`INSERT INTO grade_thresholds (scope, letter, min_value) VALUES ('default', 'Z', 70)`)
	assert.Error(t, insErr, "letter CHECK 위반 INSERT 거부 (D3)")
}
