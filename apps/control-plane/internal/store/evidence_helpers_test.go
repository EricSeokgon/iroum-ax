//go:build integration

// evidence_helpers_test.go — 증빙 통합 테스트 공통 픽스처
// SPEC-AX-EVID-001: postgres_test.go의 setupTestDB 패턴을 재사용하며
// 증빙 테스트 전용 헬퍼(SHA-256 계산, evidence 행 카운트 등)를 제공한다.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// sha256Hex 바이트의 SHA-256을 64자 소문자 hex로 반환 (stdlib only — 데이터 주권)
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// countEvidences 특정 evaluation_item_id의 evidences 행 수를 별도 풀 커넥션으로 조회
// (커밋 후 검증용 — 동일 TX 내부가 아님을 보장)
func countEvidences(t *testing.T, db *testDB, evalItemID string) int {
	t.Helper()
	var n int
	err := db.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM evidences WHERE evaluation_item_id = $1`, evalItemID,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

// countAuditByAction resource_id + action별 audit_logs 행 수를 별도 풀 커넥션으로 조회
func countAuditByAction(t *testing.T, db *testDB, resourceID uuid.UUID, action string) int {
	t.Helper()
	var n int
	err := db.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1 AND action = $2`,
		resourceID, action,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

// insertEvidenceV1 신규 증빙(version=1)을 단일 TX로 삽입하고 커밋한 뒤 ID 반환
// database_blob 전략 기본값으로 file_content를 BYTEA에 저장
func insertEvidenceV1(t *testing.T, db *testDB, evalItemID string, content []byte) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)

	id, err := tx.InsertEvidence(ctx,
		evalItemID, "report.pdf", "application/pdf",
		int64(len(content)), sha256Hex(content),
		"database_blob", "", // storage_location은 핸들러가 db://evidences/<id>로 채움; store 단위 테스트는 빈 값 허용
		map[string]string{"origin": "unit-test"},
		content, nil,
	)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return id
}
