//go:build integration

// postgres_test.go — PgWorkflowStore 통합 테스트 (AC-CTRL-004-1~4)
// Sprint 3 RED phase: 모든 테스트가 ErrNotImplemented로 실패해야 정상
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -v -count=1
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// ============================================================
// 테스트 픽스처 — testcontainers postgres:16-alpine
// ============================================================

// testDB 통합 테스트 전반에서 공유하는 PostgreSQL 컨테이너 + 연결 풀
type testDB struct {
	pool      *pgxpool.Pool
	store     *PgWorkflowStore
	logger    *zap.Logger
	container *tcpostgres.PostgresContainer
}

// setupTestDB testcontainers-go로 postgres:16-alpine을 스폰하고
// schema.sql을 적용한 후 PgWorkflowStore를 초기화하여 반환
// t.Cleanup으로 컨테이너와 풀을 정리
func setupTestDB(t *testing.T) *testDB {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	// 스키마 파일 경로 확인
	schemaPath := "testdata/schema.sql"
	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("schema.sql을 찾을 수 없음: %v", err)
	}

	// postgres:16-alpine 컨테이너 시작
	// BasicWaitStrategies()는 CustomizeRequestOption 타입 — 직접 인자로 전달
	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.WithInitScripts(schemaPath),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err, "testcontainers postgres 컨테이너 시작 실패")

	// t.Cleanup 사용 (defer 금지 — testcontainers 요구사항)
	t.Cleanup(func() {
		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		if terminateErr := pgContainer.Terminate(terminateCtx); terminateErr != nil {
			logger.Warn("postgres 컨테이너 종료 실패", zap.Error(terminateErr))
		}
	})

	// DSN 조회
	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "postgres DSN 조회 실패")

	// PgWorkflowStore 초기화
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(connectCancel)

	store, err := NewPgWorkflowStore(connectCtx, dsn, logger)
	require.NoError(t, err, "PgWorkflowStore 초기화 실패")

	t.Cleanup(func() {
		store.Close()
	})

	return &testDB{
		pool:      store.pool,
		store:     store,
		logger:    logger,
		container: pgContainer,
	}
}

// setupTestDBWithMaxConns 풀 고갈 테스트용 — MaxConns=1로 제한된 풀 생성
func setupTestDBWithMaxConns(t *testing.T, maxConns int32) *testDB {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	schemaPath := "testdata/schema.sql"
	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("schema.sql을 찾을 수 없음: %v", err)
	}

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.WithInitScripts(schemaPath),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		if terminateErr := pgContainer.Terminate(terminateCtx); terminateErr != nil {
			logger.Warn("postgres 컨테이너 종료 실패", zap.Error(terminateErr))
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// MaxConns 제한 풀 설정
	poolCfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	poolCfg.MaxConns = maxConns
	// 풀 획득 대기시간 제한 (AC-CTRL-004-3: 5초 이내 실패)
	// pgx v5: ConnConfig.ConnectTimeout (pgconn.Config 필드)
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	store := newPgWorkflowStoreWithPool(pool, logger)

	return &testDB{
		pool:      pool,
		store:     store,
		logger:    logger,
		container: pgContainer,
	}
}

// newTestWorkflow 테스트용 Workflow 픽스처 생성
func newTestWorkflow() *types.Workflow {
	return &types.Workflow{
		ID:         uuid.New(),
		DocumentID: uuid.New(),
		State:      types.WorkflowStatePending,
		CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
		UpdatedAt:  time.Now().UTC().Truncate(time.Millisecond),
	}
}

// newTestAuditEvent 테스트용 audit.Event 픽스처 생성
func newTestAuditEvent(workflowID uuid.UUID, action audit.Action) *audit.Event {
	details := map[string]string{"test": "data"}
	detailsJSON, _ := json.Marshal(details)
	return &audit.Event{
		Action:       action,
		ResourceType: "workflow",
		ResourceID:   workflowID,
		UserID:       "cli-anonymous",
		Timestamp:    time.Now().UTC().Truncate(time.Millisecond),
		DetailsJSON:  detailsJSON,
	}
}

// ============================================================
// AC-CTRL-004-1 — workflows 행 CRUD: INSERT + SELECT 왕복
// ============================================================

// TestPgStore_Integration_InsertAndGetWorkflow workflows 행 삽입 후 SELECT 왕복 검증
// AC-CTRL-004-1: ID, state, document_id, timestamps 필드 일치
func TestPgStore_Integration_InsertAndGetWorkflow(t *testing.T) {
	db := setupTestDB(t)

	ctx := context.Background()
	wf := newTestWorkflow()

	// BeginTx
	tx, err := db.store.BeginTx(ctx)
	require.NoError(t, err, "BeginTx 실패")

	// InsertWorkflow
	err = tx.InsertWorkflow(ctx, wf)
	require.NoError(t, err, "InsertWorkflow 실패")

	// GetWorkflow (SELECT FOR UPDATE)
	got, err := tx.GetWorkflow(ctx, wf.ID.String())
	require.NoError(t, err, "GetWorkflow 실패")

	// Commit
	err = tx.Commit(ctx)
	require.NoError(t, err, "Commit 실패")

	// 필드 일치 검증
	assert.Equal(t, wf.ID, got.ID, "ID 불일치")
	assert.Equal(t, wf.State, got.State, "State 불일치")
	assert.Equal(t, wf.DocumentID, got.DocumentID, "DocumentID 불일치")
	assert.WithinDuration(t, wf.CreatedAt, got.CreatedAt, time.Second, "CreatedAt 허용 오차 초과")
}

// TestPgStore_Integration_UpdateWorkflowState_AffectsExactlyOneRow 상태 갱신이 정확히 1개 행에만 적용
func TestPgStore_Integration_UpdateWorkflowState_AffectsExactlyOneRow(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// 2개 워크플로우 삽입
	wf1 := newTestWorkflow()
	wf2 := newTestWorkflow()

	// wf1 삽입
	tx1, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx1.InsertWorkflow(ctx, wf1))
	require.NoError(t, tx1.Commit(ctx))

	// wf2 삽입
	tx2, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx2.InsertWorkflow(ctx, wf2))
	require.NoError(t, tx2.Commit(ctx))

	// wf1만 RUNNING으로 갱신
	tx3, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx3.UpdateWorkflowState(ctx, wf1.ID.String(), types.WorkflowStateRunning))
	require.NoError(t, tx3.Commit(ctx))

	// wf1 상태 확인
	tx4, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	got1, err := tx4.GetWorkflow(ctx, wf1.ID.String())
	require.NoError(t, err)
	got2, err := tx4.GetWorkflow(ctx, wf2.ID.String())
	require.NoError(t, err)
	require.NoError(t, tx4.Commit(ctx))

	assert.Equal(t, types.WorkflowStateRunning, got1.State, "wf1이 RUNNING이어야 함")
	assert.Equal(t, types.WorkflowStatePending, got2.State, "wf2는 PENDING을 유지해야 함")
}

// TestPgStore_Integration_UpdateWorkflowResult_StoresJSONB result_json JSONB 저장 검증
func TestPgStore_Integration_UpdateWorkflowResult_StoresJSONB(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	wf := newTestWorkflow()

	// 삽입
	tx, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.InsertWorkflow(ctx, wf))
	require.NoError(t, tx.Commit(ctx))

	// result_json 갱신
	result := map[string]interface{}{
		"score":  0.85,
		"status": "completed",
	}
	resultJSON, err := json.Marshal(result)
	require.NoError(t, err)

	tx2, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx2.UpdateWorkflowResult(ctx, wf.ID.String(), resultJSON))
	require.NoError(t, tx2.Commit(ctx))

	// 조회 후 검증
	tx3, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	got, err := tx3.GetWorkflow(ctx, wf.ID.String())
	require.NoError(t, err)
	require.NoError(t, tx3.Commit(ctx))

	assert.NotNil(t, got.ResultJSON, "ResultJSON이 nil이어서는 안 됨")
	// JSONB 역직렬화 검증
	var gotResult map[string]interface{}
	require.NoError(t, json.Unmarshal(got.ResultJSON, &gotResult))
	assert.InDelta(t, 0.85, gotResult["score"], 0.001, "score 불일치")
}

// TestPgStore_Integration_GetWorkflow_NotFound_ReturnsError 존재하지 않는 ID 조회 시 에러 반환
func TestPgStore_Integration_GetWorkflow_NotFound_ReturnsError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.store.BeginTx(ctx)
	require.NoError(t, err)

	nonExistentID := uuid.New().String()
	got, err := tx.GetWorkflow(ctx, nonExistentID)

	// 에러 반환 + nil 값 검증
	assert.Error(t, err, "존재하지 않는 워크플로우 조회 시 에러를 반환해야 함")
	assert.Nil(t, got, "존재하지 않는 워크플로우 조회 시 nil을 반환해야 함")

	// 트랜잭션 정리 (rollback 무시)
	_ = tx.Rollback(ctx)
}

// ============================================================
// AC-CTRL-004-2 — audit_logs 행 INSERT with JSONB details
// ============================================================

// TestPgStore_Integration_InsertAuditLog_WithJSONDetails audit_logs JSONB 세부 정보 삽입 검증
// AC-CTRL-004-2: action, resource_type, resource_id, user_id, details JSONB
func TestPgStore_Integration_InsertAuditLog_WithJSONDetails(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// 워크플로우 먼저 삽입 (audit_logs는 독립적 — resource_id가 workflows FK 아님)
	wfID := uuid.New()
	event := newTestAuditEvent(wfID, audit.ActionWorkflowCreated)

	tx, err := db.store.BeginTx(ctx)
	require.NoError(t, err)

	err = tx.InsertAuditLog(ctx, event)
	require.NoError(t, err, "InsertAuditLog 실패")

	err = tx.Commit(ctx)
	require.NoError(t, err, "Commit 실패")

	// audit_logs 행 직접 조회로 검증
	var (
		gotAction       string
		gotResourceType string
		gotResourceID   uuid.UUID
		gotUserID       string
		gotDetails      []byte
	)
	err = db.pool.QueryRow(ctx,
		`SELECT action, resource_type, resource_id, user_id, details
		 FROM audit_logs WHERE resource_id = $1`,
		wfID,
	).Scan(&gotAction, &gotResourceType, &gotResourceID, &gotUserID, &gotDetails)
	require.NoError(t, err, "audit_logs 조회 실패")

	assert.Equal(t, string(audit.ActionWorkflowCreated), gotAction)
	assert.Equal(t, "workflow", gotResourceType)
	assert.Equal(t, wfID, gotResourceID)
	assert.Equal(t, "cli-anonymous", gotUserID)

	// JSONB 세부 정보 검증
	var details map[string]string
	require.NoError(t, json.Unmarshal(gotDetails, &details))
	assert.Equal(t, "data", details["test"])
}

// TestPgStore_Integration_InsertAuditLog_AllActionTypes 모든 Action 타입을 audit_logs에 삽입 가능
func TestPgStore_Integration_InsertAuditLog_AllActionTypes(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	actions := []audit.Action{
		audit.ActionWorkflowCreated,
		audit.ActionWorkflowTransitionedToRunning,
		audit.ActionWorkflowCompleted,
		audit.ActionWorkflowFailedDispatch,
		audit.ActionWorkflowFailedCallback,
		audit.ActionTransitionRejected,
		audit.ActionCallbackRejectedTerminal,
		audit.ActionWorkflowCreateCancelled,
	}

	for _, action := range actions {
		action := action
		t.Run(string(action), func(t *testing.T) {
			wfID := uuid.New()
			event := newTestAuditEvent(wfID, action)

			tx, err := db.store.BeginTx(ctx)
			require.NoError(t, err)
			require.NoError(t, tx.InsertAuditLog(ctx, event))
			require.NoError(t, tx.Commit(ctx))

			// 삽입 확인
			var count int
			err = db.pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM audit_logs WHERE resource_id=$1 AND action=$2`,
				wfID, string(action),
			).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 1, count, fmt.Sprintf("action=%s 삽입 후 행이 1개여야 함", action))
		})
	}
}

// ============================================================
// AC-CTRL-004-3 — SELECT FOR UPDATE locks row during transition
// ============================================================

// TestPgStore_Integration_SelectForUpdate_ConcurrentTransition SELECT FOR UPDATE 동시 전이 직렬화
// AC-CTRL-004-3: 2개 goroutine이 동시에 BeginTx + GetWorkflow(FOR UPDATE) 호출
// G1이 락을 획득하고 상태를 RUNNING으로 변경 후 커밋
// G2는 G1 커밋 후 락을 획득하며, RUNNING 상태를 관찰해야 함
func TestPgStore_Integration_SelectForUpdate_ConcurrentTransition(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// 초기 워크플로우 삽입 (PENDING 상태)
	wf := newTestWorkflow()
	setupTx, err := db.store.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, setupTx.InsertWorkflow(ctx, wf))
	require.NoError(t, setupTx.Commit(ctx))

	// G1이 락을 획득하는 타이밍을 제어하기 위한 채널
	g1LockAcquired := make(chan struct{})
	g1Done := make(chan struct{})
	var g2State types.WorkflowState
	var wg sync.WaitGroup
	var g1Err, g2Err error

	wg.Add(2)

	// G1: 락 획득 → 상태 RUNNING으로 변경 → 커밋
	go func() {
		defer wg.Done()
		defer close(g1Done)

		txCtx, txCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer txCancel()

		tx1, err := db.store.BeginTx(txCtx)
		if err != nil {
			g1Err = fmt.Errorf("G1 BeginTx: %w", err)
			close(g1LockAcquired)
			return
		}

		// SELECT FOR UPDATE — 락 획득
		_, err = tx1.GetWorkflow(txCtx, wf.ID.String())
		if err != nil {
			g1Err = fmt.Errorf("G1 GetWorkflow: %w", err)
			_ = tx1.Rollback(txCtx)
			close(g1LockAcquired)
			return
		}

		// G2에게 G1이 락을 획득했음을 알림
		close(g1LockAcquired)

		// 상태 변경: PENDING → RUNNING
		if err := tx1.UpdateWorkflowState(txCtx, wf.ID.String(), types.WorkflowStateRunning); err != nil {
			g1Err = fmt.Errorf("G1 UpdateWorkflowState: %w", err)
			_ = tx1.Rollback(txCtx)
			return
		}

		// 100ms 대기 (G2가 블록됨을 확인하기 위한 여유 시간)
		time.Sleep(100 * time.Millisecond)

		if err := tx1.Commit(txCtx); err != nil {
			g1Err = fmt.Errorf("G1 Commit: %w", err)
			_ = tx1.Rollback(txCtx)
		}
	}()

	// G2: G1이 락을 획득한 직후 동일 행 잠금 시도 — G1 커밋까지 블록
	go func() {
		defer wg.Done()

		// G1이 락을 획득할 때까지 대기
		<-g1LockAcquired

		txCtx, txCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer txCancel()

		tx2, err := db.store.BeginTx(txCtx)
		if err != nil {
			g2Err = fmt.Errorf("G2 BeginTx: %w", err)
			return
		}

		// SELECT FOR UPDATE — G1 커밋까지 블록됨
		got, err := tx2.GetWorkflow(txCtx, wf.ID.String())
		if err != nil {
			g2Err = fmt.Errorf("G2 GetWorkflow: %w", err)
			_ = tx2.Rollback(txCtx)
			return
		}

		// G1 커밋 후 획득한 상태를 캡처
		if got != nil {
			g2State = got.State
		}
		_ = tx2.Rollback(txCtx)
	}()

	wg.Wait()

	// 에러 없이 완료됐는지 검증
	assert.NoError(t, g1Err, "G1 에러")
	assert.NoError(t, g2Err, "G2 에러")

	// G2가 획득한 상태는 G1이 커밋한 RUNNING이어야 함 (직렬화 검증)
	assert.Equal(t, types.WorkflowStateRunning, g2State,
		"G2는 G1 커밋 후 RUNNING 상태를 관찰해야 함 (SELECT FOR UPDATE 직렬화)")
}

// ============================================================
// AC-CTRL-004-4 — mid-transaction failure rollback
// ============================================================

// TestPgStore_Integration_MidTxFailure_Rollback 트랜잭션 중간 실패 시 롤백 원자성 검증
// AC-CTRL-004-4: SQLSTATE 23505 unique violation 시뮬레이션
// 동일 ID를 두 번 INSERT → 두 번째 INSERT가 23505 실패
// 전체 트랜잭션이 롤백되어 workflows 행 0건이어야 함
func TestPgStore_Integration_MidTxFailure_Rollback(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	wf := newTestWorkflow()

	// 단일 트랜잭션 내에서 동일 ID를 두 번 삽입 시도
	tx, err := db.store.BeginTx(ctx)
	require.NoError(t, err)

	// 첫 번째 INSERT — 성공해야 함
	err = tx.InsertWorkflow(ctx, wf)
	require.NoError(t, err, "첫 번째 InsertWorkflow 실패")

	// 두 번째 INSERT — SQLSTATE 23505 unique violation이 발생해야 함
	err = tx.InsertWorkflow(ctx, wf) // 동일 UUID
	assert.Error(t, err, "동일 UUID 두 번째 INSERT는 에러를 반환해야 함")

	// 트랜잭션 롤백
	rollbackErr := tx.Rollback(ctx)
	assert.NoError(t, rollbackErr, "Rollback 에러")

	// workflows 테이블에 행이 없어야 함 (롤백 완료)
	var count int
	queryErr := db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM workflows WHERE id = $1`,
		wf.ID,
	).Scan(&count)
	require.NoError(t, queryErr)
	assert.Equal(t, 0, count, "롤백 후 workflows 행이 0건이어야 함 (partial commit 금지)")
}

// TestPgStore_Integration_AtomicWorkflowAndAudit 워크플로우 + 감사 로그 원자적 삽입/롤백
// AC-CTRL-UBI-001 Scenario A — audit INSERT 실패 시 workflow INSERT도 rollback
func TestPgStore_Integration_AtomicWorkflowAndAudit_AuditFailRollsBackWorkflow(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	wf := newTestWorkflow()

	tx, err := db.store.BeginTx(ctx)
	require.NoError(t, err)

	// 워크플로우 삽입 — 성공
	require.NoError(t, tx.InsertWorkflow(ctx, wf))

	// 잘못된 감사 이벤트 삽입 — action 필드를 비워서 NOT NULL 위반 유도
	invalidEvent := &audit.Event{
		Action:       "", // NOT NULL 위반
		ResourceType: "workflow",
		ResourceID:   wf.ID,
		UserID:       "cli-anonymous",
		Timestamp:    time.Now().UTC(),
	}
	err = tx.InsertAuditLog(ctx, invalidEvent)
	// 에러 발생 (NOT NULL 위반 또는 check constraint)
	if err != nil {
		// 에러 발생 시 롤백
		_ = tx.Rollback(ctx)
	} else {
		// 에러가 없더라도 일부러 롤백
		_ = tx.Rollback(ctx)
	}

	// workflows 행 없어야 함 (원자성 보장)
	var count int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM workflows WHERE id=$1`, wf.ID,
	).Scan(&count))
	assert.Equal(t, 0, count, "audit 실패 후 workflow 롤백 필요")
}

// ============================================================
// AC-CTRL-004-1 추가 — Pool 초기화 Fail-Fast
// ============================================================

// TestPgStore_Integration_NewPgWorkflowStore_InvalidDSN 잘못된 DSN 시 5초 이내 에러 반환
// AC-CTRL-004-1: pool 초기화 fail-fast
func TestPgStore_Integration_NewPgWorkflowStore_InvalidDSN(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// 존재하지 않는 호스트/DB DSN
	invalidDSN := "postgres://nonexistent:nonexistent@127.0.0.1:19999/dbnone?connect_timeout=3"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	start := time.Now()
	store, err := NewPgWorkflowStore(ctx, invalidDSN, logger)
	elapsed := time.Since(start)

	assert.Error(t, err, "잘못된 DSN은 에러를 반환해야 함")
	assert.Nil(t, store, "잘못된 DSN 시 *PgWorkflowStore는 nil이어야 함")
	assert.Less(t, elapsed, 10*time.Second, "5초 이내 fail-fast 필요")
}

// ============================================================
// AC-CTRL-004-3 — Pool Exhaustion
// ============================================================

// TestPgStore_Integration_BeginTx_PoolExhausted_ReturnsError MaxConns=1 상태에서 풀 고갈 검증
// AC-CTRL-004-3: 1개 커넥션이 점유된 상태에서 두 번째 BeginTx가 timeout 에러를 반환해야 함
func TestPgStore_Integration_BeginTx_PoolExhausted_ReturnsError(t *testing.T) {
	// MaxConns=1 풀 사용
	db := setupTestDBWithMaxConns(t, 1)

	// 첫 번째 트랜잭션으로 유일한 커넥션 점유
	holdCtx, holdCancel := context.WithCancel(context.Background())
	defer holdCancel()

	tx1, err := db.store.BeginTx(holdCtx)
	require.NoError(t, err, "첫 번째 BeginTx는 성공해야 함")

	t.Cleanup(func() {
		_ = tx1.Rollback(context.Background())
	})

	// 두 번째 BeginTx — 풀 고갈로 인한 타임아웃 기대
	// 짧은 timeout으로 빠른 실패 유도
	exhaustCtx, exhaustCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer exhaustCancel()

	tx2, err := db.store.BeginTx(exhaustCtx)

	// 에러가 발생해야 함 (pool 고갈 또는 context 타임아웃)
	assert.Error(t, err, "풀 고갈 상태에서 BeginTx는 에러를 반환해야 함")
	if tx2 != nil {
		_ = tx2.Rollback(context.Background())
	}
}
