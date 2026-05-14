//go:build integration

// e2e_test.go — 전체 제어 평면 E2E 통합 테스트
// Sprint 7 GREEN: AC-CTRL-E2E-1 — REST → gRPC → StateMachine → PgStore → CeleryDispatch(Redis)
// 전체 파이프라인을 testcontainers-go(Postgres + Redis)로 검증한다.
//
// 테스트 전략:
//   - testcontainers-go로 실제 Postgres:16-alpine + Redis:7-alpine 컨테이너 기동
//   - 실제 PgWorkflowStore + CeleryDispatcher + TxCoordinator + StateMachine + WorkflowService + RESTHandler 조합
//   - httptest.NewServer로 REST 엔드포인트를 노출하여 HTTP 클라이언트로 검증
//   - Redis 검증: go-redis v9로 celery LIST를 직접 조회하여 envelope 존재 확인
//
// AC 커버리지:
//   - AC-CTRL-E2E-1: 전체 워크플로우 생성 파이프라인 (Happy Path)
//   - AC-CTRL-UBI-002-C: cli-anonymous user_id 기본값 (REST 경로)
//   - 상태 전이 E2E: PENDING → RUNNING + audit log 검증
//   - Redis 장애 E2E: Redis 닫힌 후 Dispatch 실패 경로
//   - List E2E: 3개 워크플로우 생성 후 GET /api/v1/workflows 전체 반환
//   - Concurrent E2E: 5개 동시 POST 모두 성공, 중복 없음
package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/scheduler"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/server"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// e2eContainers E2E 테스트에서 공유하는 컨테이너 및 연결 정보
// 필드 순서: 포인터(8B) 먼저 배치하여 fieldalignment 최적화
type e2eContainers struct {
	pgPool    *pgxpool.Pool
	pgDSN     string
	redisAddr string
}

// setupE2EContainers Postgres + Redis 컨테이너를 기동하고 스키마를 적용한다.
// t.Cleanup으로 정리를 등록하므로 defer가 아니라 Cleanup을 사용한다.
//
// @MX:ANCHOR: [AUTO] E2E 테스트 인프라 초기화 — 5개 E2E 테스트 모두 이 함수를 호출
// @MX:REASON: AC-CTRL-E2E-1 요구사항 — 실제 testcontainers 기반 인프라 필요
func setupE2EContainers(t *testing.T) *e2eContainers {
	t.Helper()

	// 컨테이너 기동 타임아웃 (60초 — 첫 pull 포함)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	// ── Postgres 컨테이너 기동 ──────────────────────────────────────────────
	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("iroum_ax"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err, "Postgres 컨테이너 기동 실패")
	t.Cleanup(func() {
		if terr := pgContainer.Terminate(context.Background()); terr != nil {
			t.Logf("Postgres 컨테이너 종료 실패: %v", terr)
		}
	})

	pgDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Postgres DSN 조회 실패")

	// ── Redis 컨테이너 기동 ───────────────────────────────────────────────
	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "Redis 컨테이너 기동 실패")
	t.Cleanup(func() {
		if terr := redisContainer.Terminate(context.Background()); terr != nil {
			t.Logf("Redis 컨테이너 종료 실패: %v", terr)
		}
	})

	redisAddr, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err, "Redis 주소 조회 실패")
	// ConnectionString은 "redis://host:port" 또는 "host:port" 형태를 반환한다.
	// go-redis는 "host:port" 형태만 허용하므로 "redis://" 접두사를 제거한다.
	redisAddr = stripRedisScheme(redisAddr)

	// ── Postgres 스키마 적용 ───────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, pgDSN)
	require.NoError(t, err, "pgxpool 생성 실패")
	t.Cleanup(pool.Close)

	schemaSQL := loadSchemaSQL(t)
	_, err = pool.Exec(ctx, schemaSQL)
	require.NoError(t, err, "스키마 적용 실패")

	return &e2eContainers{
		pgDSN:     pgDSN,
		redisAddr: redisAddr,
		pgPool:    pool,
	}
}

// stripRedisScheme "redis://host:port" 형식에서 "redis://" 접두사를 제거한다.
func stripRedisScheme(addr string) string {
	const prefix = "redis://"
	if len(addr) > len(prefix) && addr[:len(prefix)] == prefix {
		return addr[len(prefix):]
	}
	return addr
}

// loadSchemaSQL testdata/schema.sql 파일을 읽어 문자열로 반환한다.
func loadSchemaSQL(t *testing.T) string {
	t.Helper()
	// 빌드 태그 integration이 붙으면 실행 디렉토리가 패키지 루트이다.
	// server 패키지에서 ../store/testdata/schema.sql 경로를 사용한다.
	path := "../store/testdata/schema.sql"
	data, err := os.ReadFile(path)
	require.NoError(t, err, "schema.sql 로드 실패: %s", path)
	return string(data)
}

// e2eStack E2E 테스트용 전체 스택 (store + dispatcher + state machine + service + handler)
// 필드 순서: 포인터(8B) 순으로 배치하여 fieldalignment 최적화
type e2eStack struct {
	pgStore     *store.PgWorkflowStore
	dispatcher  *scheduler.CeleryDispatcher
	sm          *workflow.StateMachine
	svc         *server.WorkflowService
	handler     *server.RESTHandler
	redisClient *redis.Client
	logger      *zap.Logger
}

// buildE2EStack 실제 인프라 컨테이너에 연결된 전체 E2E 스택을 조립한다.
func buildE2EStack(t *testing.T, c *e2eContainers) *e2eStack {
	t.Helper()
	ctx := context.Background()

	logger := zaptest.NewLogger(t)

	// ── PgWorkflowStore ──────────────────────────────────────────────────
	pgStore, err := store.NewPgWorkflowStore(ctx, c.pgDSN, logger)
	require.NoError(t, err, "PgWorkflowStore 초기화 실패")
	t.Cleanup(pgStore.Close)

	// ── Redis 클라이언트 (go-redis) ───────────────────────────────────────
	redisClient := redis.NewClient(&redis.Options{
		Addr: c.redisAddr,
	})
	t.Cleanup(func() { _ = redisClient.Close() })

	// ── CeleryDispatcher ─────────────────────────────────────────────────
	goRedisAdapter := &goRedisAdapter{client: redisClient}
	disp := scheduler.NewCeleryDispatcher(goRedisAdapter, "celery", "test-host")

	// ── TxCoordinator + StateMachine ─────────────────────────────────────
	recorder := audit.NewRecorder(false) // authEnabled=false → cli-anonymous
	coordinator := workflow.NewTxCoordinator(pgStore, recorder)
	sm := workflow.NewStateMachine(coordinator, logger)

	// ── WorkflowService (gRPC) ────────────────────────────────────────────
	svc := server.NewWorkflowService(pgStore, sm, logger)

	// ── RESTHandler ───────────────────────────────────────────────────────
	handler := server.NewRESTHandler(svc, logger)

	return &e2eStack{
		pgStore:     pgStore,
		dispatcher:  disp,
		sm:          sm,
		svc:         svc,
		handler:     handler,
		redisClient: redisClient,
		logger:      logger,
	}
}

// goRedisAdapter go-redis v9 클라이언트를 scheduler.RedisClient 인터페이스로 래핑한다.
// scheduler 패키지의 RedisClient 인터페이스: RPush(ctx, key, values...) + Ping(ctx)
type goRedisAdapter struct {
	client *redis.Client
}

func (a *goRedisAdapter) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	return a.client.RPush(ctx, key, values...).Result()
}

func (a *goRedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// goleak 필터 — 외부 라이브러리의 수명주기 goroutine은 실제 leak이 아니다.
// ──────────────────────────────────────────────────────────────────────────────

// e2eGoLeakOptions testcontainers-go, pgxpool, go-redis가 내부적으로 유지하는
// 수명주기 goroutine을 goleak 검사에서 제외한다.
// 이 goroutine들은 pool.Close() / redis.Close() / 컨테이너 종료 시 정리되며,
// t.Cleanup 등록 순서상 goleak.VerifyNone 이후에 정리될 수 있다.
var e2eGoLeakOptions = []goleak.Option{
	// pgxpool backgroundHealthCheck — pool.Close() 후 종료
	goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),
	// testcontainers-go Reaper 연결 goroutine
	goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"),
	// go-redis CircuitBreakerManager 정리 루프
	goleak.IgnoreTopFunction("github.com/redis/go-redis/v9/maintnotifications.(*CircuitBreakerManager).cleanupLoop"),
	// net/http httptest.Server 수신 대기 goroutine — ts.Close() 후 종료
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	// net/http 연결 persist goroutine (keep-alive 연결 — ts.Close() 후 정리)
	goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	// net/http httptest 서버의 활성 연결 핸들러 goroutine
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
	// net/http 클라이언트 연결 Reader goroutine
	goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
}

// ──────────────────────────────────────────────────────────────────────────────
// E2E 테스트
// ──────────────────────────────────────────────────────────────────────────────

// TestE2E_FullWorkflowCreationFlow AC-CTRL-E2E-1 Happy Path
// REST POST → WorkflowService → TxCoordinator → InsertWorkflow + RecordCreated → CeleryDispatcher.Dispatch
//
// Given: testcontainers Postgres + Redis running
// When: POST /api/v1/workflows {document_id}
// Then:
//   - HTTP 201 Created + workflow JSON
//   - PostgreSQL: workflows row with state=PENDING
//   - PostgreSQL: audit_logs row with action=WORKFLOW_CREATED
//   - Redis celery queue: 1 envelope, workflow_id 포함
func TestE2E_FullWorkflowCreationFlow(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	containers := setupE2EContainers(t)
	stack := buildE2EStack(t, containers)

	ts := httptest.NewServer(stack.handler.Mux())
	t.Cleanup(ts.Close)

	ctx := context.Background()
	docID := uuid.New().String()

	// Redis celery LIST 사전 확인 (비어 있어야 함)
	initialLen, err := stack.redisClient.LLen(ctx, "celery").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), initialLen, "테스트 시작 전 celery LIST가 비어 있어야 함")

	// ── REST POST /api/v1/workflows ───────────────────────────────────────
	body, _ := json.Marshal(map[string]string{"document_id": docID})
	resp, err := http.Post(ts.URL+"/api/v1/workflows", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// HTTP 201 확인
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// 응답 JSON 파싱
	var wfResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wfResp))
	workflowID, ok := wfResp["workflow_id"].(string)
	require.True(t, ok, "응답에 workflow_id 필드가 있어야 함")
	require.NotEmpty(t, workflowID)

	// status=PENDING 확인
	statusVal, _ := wfResp["status"].(string)
	assert.Contains(t, statusVal, "PENDING", "초기 상태는 PENDING이어야 함")

	// Location 헤더 확인
	assert.Contains(t, resp.Header.Get("Location"), workflowID, "Location 헤더에 workflow_id 포함")

	// ── PostgreSQL: workflows row 존재 확인 ──────────────────────────────
	var dbStatus string
	row := containers.pgPool.QueryRow(ctx,
		"SELECT status FROM workflows WHERE id = $1", workflowID)
	require.NoError(t, row.Scan(&dbStatus))
	assert.Equal(t, "PENDING", dbStatus, "DB workflows.status = PENDING")

	// ── PostgreSQL: audit_logs row 존재 확인 ──────────────────────────────
	var auditAction string
	var auditUserID string
	aRow := containers.pgPool.QueryRow(ctx,
		"SELECT action, user_id FROM audit_logs WHERE resource_id = $1", workflowID)
	require.NoError(t, aRow.Scan(&auditAction, &auditUserID))
	assert.Equal(t, "WORKFLOW_CREATED", auditAction, "audit_logs.action = WORKFLOW_CREATED")
	assert.Equal(t, "cli-anonymous", auditUserID, "audit_logs.user_id = cli-anonymous (AC-CTRL-UBI-002-C)")

	// ── Dispatch: celery envelope RPUSH ────────────────────────────────
	// WorkflowService.CreateWorkflow는 PENDING 상태로만 생성한다.
	// Dispatch는 별도 호출 (AC-CTRL-E2E-1 명세상 full pipeline 검증).
	// E2E에서 직접 Dispatch를 호출하여 Redis queue 검증을 완성한다.
	dispErr := stack.dispatcher.Dispatch(ctx, workflowID, docID)
	require.NoError(t, dispErr, "CeleryDispatcher.Dispatch 실패")

	// ── Redis: celery LIST에 1개 envelope 존재 확인 ───────────────────────
	queueLen, err := stack.redisClient.LLen(ctx, "celery").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), queueLen, "Redis celery LIST에 1개 envelope이 있어야 함")

	// envelope 내용 확인 — workflow_id 포함 여부
	envelopes, err := stack.redisClient.LRange(ctx, "celery", 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, envelopes, 1)

	var envelopeJSON map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(envelopes[0]), &envelopeJSON))
	// headers.id == workflowID 검증
	headers, _ := envelopeJSON["headers"].(map[string]interface{})
	require.NotNil(t, headers, "envelope에 headers 필드가 있어야 함")
	assert.Equal(t, workflowID, headers["id"], "envelope.headers.id == workflow_id")
}

// TestE2E_StateTransitionWithAudit 상태 전이 E2E: PENDING → RUNNING + audit log 검증
//
// Given: Postgres에 PENDING 워크플로우 존재
// When: StateMachine.Start(workflowID)
// Then:
//   - workflows.status = RUNNING
//   - audit_logs에 WORKFLOW_TRANSITIONED_TO_RUNNING row 존재
func TestE2E_StateTransitionWithAudit(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	containers := setupE2EContainers(t)
	stack := buildE2EStack(t, containers)

	ts := httptest.NewServer(stack.handler.Mux())
	t.Cleanup(ts.Close)

	ctx := context.Background()
	docID := uuid.New().String()

	// 워크플로우 생성 (PENDING)
	body, _ := json.Marshal(map[string]string{"document_id": docID})
	resp, err := http.Post(ts.URL+"/api/v1/workflows", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var wfResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wfResp))
	workflowID := wfResp["workflow_id"].(string)

	// PENDING → RUNNING 전이
	err = stack.sm.Start(ctx, workflowID)
	require.NoError(t, err, "StateMachine.Start 실패")

	// workflows.status = RUNNING 확인
	var dbStatus string
	require.NoError(t, containers.pgPool.QueryRow(ctx,
		"SELECT status FROM workflows WHERE id = $1", workflowID).Scan(&dbStatus))
	assert.Equal(t, "RUNNING", dbStatus)

	// audit_logs에 WORKFLOW_TRANSITIONED_TO_RUNNING 확인
	var auditCount int
	require.NoError(t, containers.pgPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1 AND action = 'WORKFLOW_TRANSITIONED_TO_RUNNING'",
		workflowID).Scan(&auditCount))
	assert.Equal(t, 1, auditCount, "WORKFLOW_TRANSITIONED_TO_RUNNING audit row가 1개여야 함")
}

// TestE2E_DispatchFailure_HandledGracefully Redis 장애 E2E
//
// Given: 컨테이너 없이 닫힌 Redis 주소를 사용하는 dispatcher
// When: dispatcher.Dispatch(ctx, ...) 호출
// Then: ErrDispatchFailed 래핑 에러 반환
func TestE2E_DispatchFailure_HandledGracefully(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	// 실제로 연결 불가능한 주소 사용 (Docker 기반 포트가 아닌 임의 포트)
	closedRedisAddr := "127.0.0.1:16399" // 사용 중이 아닌 포트

	logger := zaptest.NewLogger(t)

	// 연결 불가 Redis를 사용하는 dispatcher 조립
	failRedis := redis.NewClient(&redis.Options{
		Addr:         closedRedisAddr,
		DialTimeout:  100 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
	})
	t.Cleanup(func() { _ = failRedis.Close() })

	adapter := &goRedisAdapter{client: failRedis}
	disp := scheduler.NewCeleryDispatcher(adapter, "celery", "test-host")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wfID := uuid.New().String()
	docID := uuid.New().String()

	err := disp.Dispatch(ctx, wfID, docID)
	assert.Error(t, err, "연결 불가 Redis에서 Dispatch는 에러를 반환해야 함")
	assert.ErrorIs(t, err, scheduler.ErrDispatchFailed, "에러는 ErrDispatchFailed를 래핑해야 함")

	// 로거 사용 확인 (nil이 아님)
	logger.Info("Redis 장애 E2E 완료", zap.Error(err))
}

// TestE2E_ListWorkflows_ReturnsAll List E2E: 3개 생성 후 GET /api/v1/workflows 전체 반환
//
// Given: testcontainers Postgres + Redis running
// When: POST 3회 → GET /api/v1/workflows
// Then: 응답 JSON에 workflows 배열 3개
func TestE2E_ListWorkflows_ReturnsAll(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	containers := setupE2EContainers(t)
	stack := buildE2EStack(t, containers)

	ts := httptest.NewServer(stack.handler.Mux())
	t.Cleanup(ts.Close)

	// 3개 워크플로우 생성
	for i := range 3 {
		docID := uuid.New().String()
		body, _ := json.Marshal(map[string]string{"document_id": docID})
		resp, err := http.Post(ts.URL+"/api/v1/workflows", "application/json", bytes.NewReader(body))
		require.NoError(t, err, "워크플로우 %d 생성 실패", i)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()
	}

	// GET /api/v1/workflows
	resp, err := http.Get(ts.URL + "/api/v1/workflows")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var listResp map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &listResp))

	workflows, ok := listResp["workflows"].([]interface{})
	require.True(t, ok, "응답에 workflows 배열이 있어야 함")
	assert.GreaterOrEqual(t, len(workflows), 3, "최소 3개의 워크플로우가 반환되어야 함")

	total, _ := listResp["total"].(float64)
	assert.GreaterOrEqual(t, int(total), 3, "total 필드가 3 이상이어야 함")
}

// TestE2E_ConcurrentCreationFromREST 동시성 E2E: 5개 병렬 POST 모두 성공 + 중복 없음
//
// Given: testcontainers Postgres + Redis running
// When: 5개 goroutine이 동시에 POST /api/v1/workflows 호출
// Then: 모두 HTTP 201, workflow_id 5개 모두 고유
func TestE2E_ConcurrentCreationFromREST(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	containers := setupE2EContainers(t)
	stack := buildE2EStack(t, containers)

	ts := httptest.NewServer(stack.handler.Mux())
	t.Cleanup(ts.Close)

	const concurrency = 5
	// 필드 순서: 포인터/인터페이스(err=interface, 16B) 먼저, int(8B), string(16B) 순
	type result struct {
		err        error
		workflowID string
		statusCode int
	}

	results := make([]result, concurrency)
	var wg sync.WaitGroup

	for i := range concurrency {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			docID := uuid.New().String()
			body, _ := json.Marshal(map[string]string{"document_id": docID})

			resp, err := http.Post(ts.URL+"/api/v1/workflows", "application/json", bytes.NewReader(body))
			if err != nil {
				results[idx] = result{err: fmt.Errorf("HTTP 요청 실패: %w", err)}
				return
			}
			defer resp.Body.Close()

			var wfResp map[string]interface{}
			if decErr := json.NewDecoder(resp.Body).Decode(&wfResp); decErr != nil {
				results[idx] = result{statusCode: resp.StatusCode, err: decErr}
				return
			}

			wfID, _ := wfResp["workflow_id"].(string)
			results[idx] = result{statusCode: resp.StatusCode, workflowID: wfID}
		}(i)
	}
	wg.Wait()

	// 모두 성공 + ID 고유성 확인
	seen := make(map[string]bool)
	for i, r := range results {
		require.NoError(t, r.err, "goroutine %d 에러", i)
		assert.Equal(t, http.StatusCreated, r.statusCode, "goroutine %d HTTP 201이어야 함", i)
		require.NotEmpty(t, r.workflowID, "goroutine %d workflow_id가 비어 있음", i)
		assert.False(t, seen[r.workflowID], "workflow_id 중복: %s", r.workflowID)
		seen[r.workflowID] = true
	}
	assert.Len(t, seen, concurrency, "고유 workflow_id 수가 %d개여야 함", concurrency)

	// PostgreSQL: 5개 행 존재 확인
	ctx := context.Background()
	var rowCount int
	ids := make([]interface{}, 0, concurrency)
	idPlaceholders := ""
	for i, r := range results {
		ids = append(ids, r.workflowID)
		if i > 0 {
			idPlaceholders += ", "
		}
		idPlaceholders += fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf("SELECT COUNT(*) FROM workflows WHERE id IN (%s)", idPlaceholders)
	require.NoError(t, containers.pgPool.QueryRow(ctx, query, ids...).Scan(&rowCount))
	assert.Equal(t, concurrency, rowCount, "DB에 5개 workflow row가 있어야 함")
}
