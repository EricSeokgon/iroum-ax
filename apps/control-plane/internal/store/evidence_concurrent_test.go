//go:build integration

// evidence_concurrent_test.go — T-007: 동시 재업로드 직렬화 (DC-009, E-02)
// SELECT FOR UPDATE가 동일 evaluation_item_id 동시 재업로드를 직렬화하여
// 중복 version / orphan 체인 / deadlock(40P01)이 발생하지 않음을 검증한다.
// barrier-synchronized (timing-based 아님) — startCh 닫힘으로 동시 시작 보장.
package store

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// createNextVersionTx 핸들러가 수행할 버전 결정 시퀀스를 store 프리미티브로 재현:
// BeginEvidenceTx → GetLatestVersionByEvalItem(FOR UPDATE) → InsertEvidence(prev) → MarkSuperseded(prev) → Commit
func createNextVersionTx(ctx context.Context, db *testDB, evalItemID string, content []byte) error {
	tx, err := db.store.BeginEvidenceTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // BeginEvidenceTx 직후 즉시 등록 (SEC-07)

	latest, err := tx.GetLatestVersionByEvalItem(ctx, evalItemID)
	if err != nil {
		return err
	}
	var prevID *uuid.UUID
	if latest != nil {
		id := latest.ID
		prevID = &id
	}
	if _, err = tx.InsertEvidence(ctx,
		evalItemID, "v.pdf", "application/pdf",
		int64(len(content)), sha256Hex(content),
		"database_blob", "", nil, content, prevID,
	); err != nil {
		return err
	}
	if latest != nil {
		if err = tx.MarkSuperseded(ctx, latest.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// infraGoleakOptions testcontainers Reaper / pgxpool backgroundHealthCheck 등
// 라이브러리 소유 + t.Cleanup이 정리하는 인프라 goroutine을 goleak 검사에서 제외한다.
// (defer goleak.VerifyNone은 t.Cleanup보다 먼저 실행되므로 cleanup 전 시점엔 살아있음)
// 이 옵션 적용 후에도 본 테스트의 TX/goroutine 누출은 여전히 엄격히 탐지된다 (TH-03 의도 충족).
func infraGoleakOptions() []goleak.Option {
	return []goleak.Option{
		goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"),
		goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	}
}

// TestEvidenceStore_ConcurrentReupload_SerializedBySelectForUpdate
// v1 존재 상태에서 2개 goroutine이 barrier 동기화로 동시에 재업로드 →
// SELECT FOR UPDATE 직렬화로 version 1,2,3 (gap/dup 없음), deadlock 없음
func TestEvidenceStore_ConcurrentReupload_SerializedBySelectForUpdate(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)

	db := setupTestDB(t)
	ctx := context.Background()
	const evalItemID = "eval-001-4"

	// v1 셋업
	insertEvidenceV1(t, db, evalItemID, []byte("v1-content"))

	var wg sync.WaitGroup
	startCh := make(chan struct{})
	errs := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh // barrier: 두 goroutine 동시 시작
			errs[idx] = createNextVersionTx(ctx, db, evalItemID, []byte("reupload"))
		}(i)
	}
	close(startCh) // 동시 해제
	wg.Wait()

	for i, e := range errs {
		require.NoError(t, e, "goroutine %d: 직렬화 실패 또는 deadlock", i)
		if e != nil {
			var pgErr *pgconn.PgError
			if assert.ErrorAs(t, e, &pgErr) {
				assert.NotEqual(t, "40P01", pgErr.Code, "deadlock(40P01) 발생 금지")
			}
		}
	}

	var maxV, distinctV, chained int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT MAX(version) FROM evidences WHERE evaluation_item_id=$1`, evalItemID).Scan(&maxV))
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT version) FROM evidences WHERE evaluation_item_id=$1`, evalItemID).Scan(&distinctV))
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM evidences WHERE evaluation_item_id=$1 AND previous_version_id IS NOT NULL`, evalItemID).Scan(&chained))

	assert.Equal(t, 3, maxV, "MAX(version)==3 (v1 + 동시 재업로드 2건 직렬화)")
	assert.Equal(t, 3, distinctV, "version 1,2,3 — gap/duplicate 없음")
	assert.Equal(t, 2, chained, "v2,v3 각각 previous_version_id 보유 (체인 단절 없음)")
}
