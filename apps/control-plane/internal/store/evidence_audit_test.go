//go:build integration

// evidence_audit_test.go — DC-002/DC-003: 증빙 생성/버전 TX에 audit_logs 1건 (원자적 커밋)
// Recorder.RecordEvidence{Created|Versioned}를 store EvidenceTx 위에서 호출하여
// evidence 행 + audit 행이 동일 TX 커밋으로 영속되는지(부분 커밋 아님) 검증한다.
package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// TestEvidenceAudit_CreateRowAtomicCommit
// RecordEvidenceCreated를 EvidenceTx 위에서 호출 → 커밋 후 evidence 1행 + audit 1행
func TestEvidenceAudit_CreateRowAtomicCommit(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-ubi-002a"

	content := []byte("audit-atomic-content")
	hash := sha256Hex(content)

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)

	evID, err := tx.InsertEvidence(ctx,
		evalItemID, "a.pdf", "application/pdf",
		int64(len(content)), hash, "database_blob", "db://evidences/x",
		nil, content, nil,
	)
	require.NoError(t, err)

	// Recorder가 동일 EvidenceTx(AuditTx 인터페이스 충족) 위에서 감사 기록
	rec := audit.NewRecorder(false)
	require.NoError(t, rec.RecordEvidenceCreated(ctx, tx, evID.String(), evalItemID, hash, 1, ""))

	require.NoError(t, tx.Commit(ctx))

	// 커밋 후 별도 풀 커넥션으로 검증 (동일 TX 외부)
	assert.Equal(t, 1, countEvidences(t, db, evalItemID), "evidence 1행")
	assert.Equal(t, 1, countAuditByAction(t, db, evID, "EVIDENCE_CREATED"), "audit EVIDENCE_CREATED 1행")

	var di, dv, dh, uid string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT details->>'evaluation_item_id', details->>'version', details->>'file_hash_sha256', user_id
		 FROM audit_logs WHERE action='EVIDENCE_CREATED' AND resource_id=$1`, evID,
	).Scan(&di, &dv, &dh, &uid))
	assert.Equal(t, evalItemID, di)
	assert.Equal(t, "1", dv)
	assert.Len(t, dh, 64)
	assert.Equal(t, "cli-anonymous", uid, "AC-EVID-UBI-003 cli-anonymous")

	// created_by와 audit user_id byte-identical (DC-004)
	var createdBy string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT created_by FROM evidences WHERE id=$1`, evID).Scan(&createdBy))
	assert.Equal(t, createdBy, uid, "evidences.created_by == audit_logs.user_id (byte-identical)")
}

// TestEvidenceAudit_VersionRowAtomicCommit — DC-003 (store 계층)
// 재업로드(version=2)가 단일 EvidenceTx에서 (1) evidence 행 삽입,
// (2) 직전 행 status ACTIVE→SUPERSEDED (MarkSuperseded),
// (3) EVIDENCE_VERSIONED audit 행 삽입 — 세 가지가 함께 commit-or-rollback 됨을 검증한다.
// contract.md §1.0 DC-003: store-level (HTTP 변형이 아님). F3+F4 (MarkSuperseded/version>1 경로) 동시 커버.
func TestEvidenceAudit_VersionRowAtomicCommit(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-ubi-002b"

	rec := audit.NewRecorder(false) // AUTH_ENABLED=false → cli-anonymous

	// v1 선행 생성 — 실제 핸들러 흐름과 동일하게 evidence 행 + EVIDENCE_CREATED audit 행을
	// 단일 TX로 원자 커밋 (DC-003 item 4의 "2 audit rows: 1 CREATED + 1 VERSIONED" 전제).
	v1Content := []byte("evidence-v1-body")
	v1Hash := sha256Hex(v1Content)
	tx1, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)
	ev1ID, err := tx1.InsertEvidence(ctx,
		evalItemID, "v1.pdf", "application/pdf",
		int64(len(v1Content)), v1Hash, "database_blob", "db://evidences/x1",
		nil, v1Content, nil,
	)
	require.NoError(t, err)
	require.NoError(t, rec.RecordEvidenceCreated(ctx, tx1, ev1ID.String(), evalItemID, v1Hash, 1, ""))
	require.NoError(t, tx1.Commit(ctx))

	// v2 재업로드 — resolve(FOR UPDATE) → InsertEvidence → MarkSuperseded → RecordEvidenceVersioned
	// 모두 동일 EvidenceTx 내. 커밋 전 별도 커넥션에서는 어떤 v2/VERSIONED 행도 보이지 않아야 한다.
	v2Content := []byte("evidence-v2-body")
	v2Hash := sha256Hex(v2Content)

	tx, err := db.store.BeginEvidenceTx(ctx)
	require.NoError(t, err)

	latest, err := tx.GetLatestVersionByEvalItem(ctx, evalItemID)
	require.NoError(t, err)
	require.NotNil(t, latest, "v1이 선행 존재해야 함")
	require.Equal(t, ev1ID, latest.ID)
	prevID := latest.ID

	ev2ID, err := tx.InsertEvidence(ctx,
		evalItemID, "v2.pdf", "application/pdf",
		int64(len(v2Content)), v2Hash, "database_blob", "db://evidences/x2",
		nil, v2Content, &prevID,
	)
	require.NoError(t, err)

	require.NoError(t, tx.MarkSuperseded(ctx, latest.ID), "직전 행 ACTIVE→SUPERSEDED")
	require.NoError(t, rec.RecordEvidenceVersioned(ctx, tx,
		ev2ID.String(), evalItemID, v2Hash, 2, prevID.String(), ""))

	// 커밋 전 원자성 가드: 별도 풀에서 VERSIONED audit 행 0건 (부분 커밋 아님)
	assert.Equal(t, 0, countAuditByAction(t, db, ev2ID, "EVIDENCE_VERSIONED"),
		"커밋 전에는 별도 커넥션에서 VERSIONED 행이 보이면 안 됨 (원자성)")

	require.NoError(t, tx.Commit(ctx))

	// ── 커밋 후 별도 풀 커넥션 검증 (동일 TX 외부) ──────────────────────────
	// DC-003 item 1: VERSIONED audit 정확히 1건
	assert.Equal(t, 1, countAuditByAction(t, db, ev2ID, "EVIDENCE_VERSIONED"),
		"audit EVIDENCE_VERSIONED 1건")

	// DC-003 item 2: details.version=="2", details.previous_version_id==ev1ID
	// DC-003 item 3: user_id=='cli-anonymous'
	var dVersion, dPrev, dEvalItem, uid string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT details->>'version', details->>'previous_version_id',
		        details->>'evaluation_item_id', user_id
		 FROM audit_logs WHERE action='EVIDENCE_VERSIONED' AND resource_id=$1`, ev2ID,
	).Scan(&dVersion, &dPrev, &dEvalItem, &uid))
	assert.Equal(t, "2", dVersion, "details.version == \"2\"")
	assert.Equal(t, ev1ID.String(), dPrev, "details.previous_version_id == ev1ID")
	assert.Equal(t, evalItemID, dEvalItem)
	assert.Equal(t, "cli-anonymous", uid, "DC-003 item 3 user_id == cli-anonymous")

	// DC-003 item 4: evidences 2행 + audit 2행 (CREATED + VERSIONED) — 별도 커넥션 COUNT
	assert.Equal(t, 2, countEvidences(t, db, evalItemID), "evidences 2행 (v1+v2)")
	var auditTotal int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs
		 WHERE action IN ('EVIDENCE_CREATED','EVIDENCE_VERSIONED')
		   AND resource_id IN ($1,$2)`, ev1ID, ev2ID).Scan(&auditTotal))
	assert.Equal(t, 2, auditTotal, "audit 2행: 1 CREATED + 1 VERSIONED")

	// F4: predecessor status ACTIVE→SUPERSEDED 영속 확인 (별도 커넥션)
	var status1 string
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT status FROM evidences WHERE id=$1`, ev1ID).Scan(&status1))
	assert.Equal(t, "SUPERSEDED", status1, "F4: 직전 버전 status ACTIVE→SUPERSEDED 원자 커밋")
}
