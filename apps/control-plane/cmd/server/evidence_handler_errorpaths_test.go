//go:build integration

// evidence_handler_errorpaths_test.go — T-019 커버리지 보강: 핸들러 에러 분기
// BeginEvidenceTx 실패(닫힌 풀), multipart 파싱 실패, 버전 이벤트 경로(RecordEvidenceVersioned)
package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// TestEvidenceHandler_MultipartParseFailure
// Content-Type은 multipart지만 본문이 손상 → multipart 파싱 실패 분기 (400)
func TestEvidenceHandler_MultipartParseFailure(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)

	req, err := http.NewRequest(http.MethodPost, env.server.URL+"/api/v1/evidences",
		strings.NewReader("not-a-valid-multipart-body"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")

	resp, err := env.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "손상된 multipart → 400")

	assert.Equal(t, 0, countEvidencesH(t, env, ""), "파싱 실패 시 evidences 0건")
}

// TestEvidenceHandler_VersionEventPath
// 재업로드로 RecordEvidenceVersioned 경로 + storage_location db:// 형식 (커버리지 보강)
func TestEvidenceHandler_VersionEventPath(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)
	const evalItemID = "eval-h-version-path"

	st1, b1 := postEvidence(t, env, evalItemID, "v1.pdf", []byte("v1"))
	require.Equal(t, http.StatusCreated, st1)
	require.Equal(t, float64(1), b1["version"])

	st2, b2 := postEvidence(t, env, evalItemID, "v2.pdf", []byte("v2"))
	require.Equal(t, http.StatusCreated, st2)
	require.Equal(t, float64(2), b2["version"])
	ev2ID := b2["evidence_id"].(string)

	ctx := context.Background()
	var loc, status string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT storage_location, status FROM evidences WHERE id=$1`, ev2ID).Scan(&loc, &status))
	assert.Contains(t, loc, "db://evidences/")
	assert.Equal(t, "ACTIVE", status, "최신 버전 ACTIVE")

	// EVIDENCE_VERSIONED 감사 1건
	var vN int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE action='EVIDENCE_VERSIONED' AND resource_id=$1`,
		ev2ID).Scan(&vN))
	assert.Equal(t, 1, vN)
}
