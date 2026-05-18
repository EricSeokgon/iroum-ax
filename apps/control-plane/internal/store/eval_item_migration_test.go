//go:build integration

// eval_item_migration_test.go — T-001 [NEW]: 0003_eval_item_tables.sql DDL 검증
// SPEC-AX-EVAL-ITEM-001 DC-001 (마이그레이션 정확성):
//   - id = character varying(64) (UUID 아님 — §1.4 HARD, EVID-001 FK type-compat)
//   - parent_id self-FK ON DELETE RESTRICT
//   - status CHECK {ACTIVE,DEPRECATED,ARCHIVED}
//   - hierarchy_code NOT NULL UNIQUE (GAP-03)
//   - parent_id/hierarchy_code/created_at 인덱스
//   - created_by DEFAULT 'cli-anonymous' NOT NULL
//
// 실제 0003 마이그레이션 파일을 멱등 재적용한 뒤 information_schema로 검증한다.
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestEvalItemMigration -v -count=1
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

// applyMigration0003 repo 루트의 .moai/db/schema/migrations/0003_eval_item_tables.sql 을
// testcontainers DB에 멱등 재적용한다 (0001→0002 base는 setupTestDB schema.sql이 이미 부트스트랩).
func applyMigration0003(t *testing.T, db *testDB) {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err, "repo 루트 조회 실패")
	root := strings.TrimSpace(string(out))

	sqlPath := filepath.Join(root, ".moai/db/schema/migrations/0003_eval_item_tables.sql")
	sqlBytes, err := os.ReadFile(sqlPath) //nolint:gosec // 테스트 고정 경로, 사용자 입력 아님
	require.NoError(t, err, "0003 마이그레이션 파일 읽기 실패: %s", sqlPath)

	// 멱등 SQL (CREATE TABLE IF NOT EXISTS / DO $$ EXCEPTION) — 재적용해도 무해
	_, err = db.pool.Exec(context.Background(), string(sqlBytes))
	require.NoError(t, err, "0003 마이그레이션 적용 실패")
}

// TestEvalItemMigration_FileExists DC-001.1: 마이그레이션 파일 존재
func TestEvalItemMigration_FileExists(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err)
	root := strings.TrimSpace(string(out))
	p := filepath.Join(root, ".moai/db/schema/migrations/0003_eval_item_tables.sql")
	_, statErr := os.Stat(p)
	require.NoError(t, statErr, "0003_eval_item_tables.sql 이 존재해야 함: %s", p)
}

// TestEvalItemMigration_IDColumnType DC-001.3 / DC-003.6 / E-15:
// evaluation_items.id = character varying(64) (UUID/integer 아님)
func TestEvalItemMigration_IDColumnType(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0003(t, db)
	ctx := context.Background()

	var dataType string
	var maxLen int
	err := db.pool.QueryRow(ctx,
		`SELECT data_type, character_maximum_length
		 FROM information_schema.columns
		 WHERE table_name='evaluation_items' AND column_name='id'`,
	).Scan(&dataType, &maxLen)
	require.NoError(t, err)
	assert.Equal(t, "character varying", dataType, "id는 VARCHAR (UUID 아님 — §1.4 HARD)")
	assert.Equal(t, 64, maxLen, "id VARCHAR(64) — EVID-001 evidences.evaluation_item_id stub 타입 호환")
}

// TestEvalItemMigration_ParentFKRestrict DC-001.4:
// parent_id self-FK with delete_rule='RESTRICT'
func TestEvalItemMigration_ParentFKRestrict(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0003(t, db)
	ctx := context.Background()

	var deleteRule string
	err := db.pool.QueryRow(ctx,
		`SELECT rc.delete_rule
		 FROM information_schema.referential_constraints rc
		 JOIN information_schema.table_constraints tc
		   ON rc.constraint_name = tc.constraint_name
		 WHERE tc.table_name='evaluation_items' AND tc.constraint_type='FOREIGN KEY'`,
	).Scan(&deleteRule)
	require.NoError(t, err, "evaluation_items self-FK 제약이 존재해야 함")
	assert.Equal(t, "RESTRICT", deleteRule, "parent_id FK는 ON DELETE RESTRICT (orphan 하위계층 방지)")
}

// TestEvalItemMigration_StatusCheckConstraint DC-001.5:
// evaluation_items_status_chk CHECK 제약 존재
func TestEvalItemMigration_StatusCheckConstraint(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0003(t, db)
	ctx := context.Background()

	var n int
	err := db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.check_constraints
		 WHERE constraint_name='evaluation_items_status_chk'`,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "evaluation_items_status_chk CHECK 제약이 존재해야 함")

	// 열거 외 값은 거부되어야 함 (DB CHECK 강제)
	_, insErr := db.pool.Exec(ctx,
		`INSERT INTO evaluation_items (id, display_name, hierarchy_code, status)
		 VALUES ('AX-CHK-X', 'chk', 'AX.CHK.X', 'NOT_A_STATUS')`)
	assert.Error(t, insErr, "status CHECK 위반 INSERT는 거부되어야 함")
}

// TestEvalItemMigration_Indexes DC-001.6: parent_id/hierarchy_code/created_at 인덱스 존재
func TestEvalItemMigration_Indexes(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0003(t, db)
	ctx := context.Background()

	for _, idx := range []string{
		"evaluation_items_parent_id_idx",
		"evaluation_items_hierarchy_code_idx",
		"evaluation_items_created_at_idx",
	} {
		var n int
		err := db.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM pg_indexes WHERE tablename='evaluation_items' AND indexname=$1`,
			idx,
		).Scan(&n)
		require.NoError(t, err)
		assert.Equal(t, 1, n, "인덱스 %s 가 존재해야 함", idx)
	}
}

// TestEvalItemMigration_HierarchyCodeUniqueNotNull DC-001.7 / GAP-03:
// hierarchy_code UNIQUE + NOT NULL
func TestEvalItemMigration_HierarchyCodeUniqueNotNull(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0003(t, db)
	ctx := context.Background()

	var isNullable string
	err := db.pool.QueryRow(ctx,
		`SELECT is_nullable FROM information_schema.columns
		 WHERE table_name='evaluation_items' AND column_name='hierarchy_code'`,
	).Scan(&isNullable)
	require.NoError(t, err)
	assert.Equal(t, "NO", isNullable, "hierarchy_code 는 NOT NULL (GAP-03 — AUD-1 surrogate 일관성)")

	// UNIQUE 제약: 동일 hierarchy_code 두 행은 거부
	_, e1 := db.pool.Exec(ctx,
		`INSERT INTO evaluation_items (id, display_name, hierarchy_code) VALUES ('AX-U1','u','AX.UNIQ')`)
	require.NoError(t, e1)
	_, e2 := db.pool.Exec(ctx,
		`INSERT INTO evaluation_items (id, display_name, hierarchy_code) VALUES ('AX-U2','u','AX.UNIQ')`)
	assert.Error(t, e2, "동일 hierarchy_code 중복 INSERT는 UNIQUE 위반으로 거부")
}

// TestEvalItemMigration_CreatedByDefault DC-001.8:
// created_by DEFAULT 'cli-anonymous', NOT NULL
func TestEvalItemMigration_CreatedByDefault(t *testing.T) {
	db := setupTestDB(t)
	applyMigration0003(t, db)
	ctx := context.Background()

	var colDefault, isNullable string
	err := db.pool.QueryRow(ctx,
		`SELECT column_default, is_nullable FROM information_schema.columns
		 WHERE table_name='evaluation_items' AND column_name='created_by'`,
	).Scan(&colDefault, &isNullable)
	require.NoError(t, err)
	assert.Contains(t, colDefault, "cli-anonymous", "created_by DEFAULT 'cli-anonymous'")
	assert.Equal(t, "NO", isNullable, "created_by NOT NULL")
}
