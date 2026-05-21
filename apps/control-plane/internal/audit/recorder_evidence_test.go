// recorder_evidence_test.go — T-010: RecordEvidenceCreated/Versioned 단위 테스트
// DC-014 (RecordEvidenceCreated 감사 row), DC-015 (RecordEvidenceVersioned 감사 row),
// DC-004 (cli-anonymous 기본값)
// 기존 recorder_test.go의 captureTx 패턴 재사용 (동일 패키지 audit_test)
package audit_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// fixedClock T-018 — 테스트용 고정 시각 Clock
type fixedClock struct{ t time.Time }

func (f fixedClock) NowUTC() time.Time { return f.t }

// TestRecorder_RecordEvidenceCreated EVIDENCE_CREATED 감사 이벤트 기록 검증
// DC-014
func TestRecorder_RecordEvidenceCreated(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false) // authEnabled=false → cli-anonymous
	tx := &captureTx{}
	ctx := context.Background()

	evidenceID := uuid.New()
	const (
		evalItemID = "eval-evid-003-1"
		hash       = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	)

	err := recorder.RecordEvidenceCreated(ctx, tx, evidenceID.String(), evalItemID, hash, 1, "")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1, "정확히 1개의 감사 이벤트")
	ev := tx.Captured[0]
	assert.Equal(t, audit.ActionEvidenceCreated, ev.Action)
	assert.Equal(t, "evidence", ev.ResourceType)
	assert.Equal(t, evidenceID, ev.ResourceID)
	assert.Equal(t, audit.DefaultUserID, ev.UserID, "authEnabled=false → cli-anonymous")
	assert.False(t, ev.Timestamp.IsZero(), "Timestamp NOT NULL")

	var details map[string]interface{}
	require.NoError(t, json.Unmarshal(ev.DetailsJSON, &details))
	assert.Equal(t, evalItemID, details["evaluation_item_id"])
	assert.Equal(t, "1", details["version"], "version은 문자열 \"1\"")
	assert.Equal(t, hash, details["file_hash_sha256"])
	assert.Len(t, details["file_hash_sha256"].(string), 64)
	_, hasPrev := details["previous_version_id"]
	assert.False(t, hasPrev, "version=1은 previous_version_id 키 부재")
}

// TestRecorder_RecordEvidenceVersioned EVIDENCE_VERSIONED 감사 이벤트 기록 검증
// DC-015
func TestRecorder_RecordEvidenceVersioned(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	newID := uuid.New()
	prevID := uuid.New()
	const (
		evalItemID = "eval-evid-003-2"
		hash       = "0011223344556677889900112233445566778899001122334455667788990011"
	)

	err := recorder.RecordEvidenceVersioned(ctx, tx, newID.String(), evalItemID, hash, 2, prevID.String(), "")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	ev := tx.Captured[0]
	assert.Equal(t, audit.ActionEvidenceVersioned, ev.Action)
	assert.Equal(t, "evidence", ev.ResourceType)
	assert.Equal(t, newID, ev.ResourceID)
	assert.Equal(t, audit.DefaultUserID, ev.UserID)

	var details map[string]interface{}
	require.NoError(t, json.Unmarshal(ev.DetailsJSON, &details))
	assert.Equal(t, evalItemID, details["evaluation_item_id"])
	assert.Equal(t, "2", details["version"])
	assert.Equal(t, prevID.String(), details["previous_version_id"], "버전 이벤트는 previous_version_id 포함")
	assert.Equal(t, hash, details["file_hash_sha256"])
}

// TestRecorder_EvidenceDefaultUserID cli-anonymous 기본값 (authEnabled=false, 빈 userID)
// DC-004
func TestRecorder_EvidenceDefaultUserID(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordEvidenceCreated(ctx, tx, uuid.New().String(), "ei", "h", 1, "")
	require.NoError(t, err)
	require.Len(t, tx.Captured, 1)
	assert.Equal(t, "cli-anonymous", tx.Captured[0].UserID,
		"authEnabled=false + 빈 userID → cli-anonymous (literal)")

	// authEnabled=true + 실제 userID는 전파
	rec2 := audit.NewRecorder(true)
	tx2 := &captureTx{}
	require.NoError(t, rec2.RecordEvidenceVersioned(ctx, tx2, uuid.New().String(), "ei", "h", 2, uuid.New().String(), "real-user"))
	require.Len(t, tx2.Captured, 1)
	assert.Equal(t, "real-user", tx2.Captured[0].UserID)
}

// TestRecorder_EvidenceInjectedClock T-018: 주입된 Clock이 증빙 감사 Timestamp를 제어
// REFACTOR 증명 — 동작 변경 없이 시각 주입 가능 (R-EVID-007)
func TestRecorder_EvidenceInjectedClock(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	recorder := audit.NewRecorder(false, audit.WithClock(fixedClock{t: fixed}))
	tx := &captureTx{}
	ctx := context.Background()

	require.NoError(t, recorder.RecordEvidenceCreated(ctx, tx, uuid.New().String(), "ei", "h", 1, ""))
	require.Len(t, tx.Captured, 1)
	assert.True(t, tx.Captured[0].Timestamp.Equal(fixed),
		"주입된 Clock 시각이 audit Timestamp에 반영되어야 함 (R-EVID-007)")

	// 기본 생성자(Clock 미주입)는 기존 동작 — Timestamp non-zero (byte-identical refactor)
	rec2 := audit.NewRecorder(false)
	tx2 := &captureTx{}
	require.NoError(t, rec2.RecordEvidenceVersioned(ctx, tx2, uuid.New().String(), "ei", "h", 2, uuid.New().String(), ""))
	assert.False(t, tx2.Captured[0].Timestamp.IsZero(), "기본 systemClock — 기존 동작 보존")
}
