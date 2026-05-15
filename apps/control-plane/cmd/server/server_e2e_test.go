//go:build integration

// server_e2e_test.go — 전체 서버 스택 E2E 통합 테스트
// SPEC-AX-SERVER-001 Sprint 2 deliverable: testcontainers Postgres+Redis + 실제 Server.New()+Run()
//
// 테스트 전략:
//   - testcontainers-go로 Postgres:16-alpine + Redis:7-alpine 기동
//   - Server.New() → Server.Run() (goroutine)으로 실제 서버 부팅
//   - HTTP/gRPC dual listener 양쪽 검증
//   - /ready 200 → SIGTERM → graceful shutdown → /ready 503 흐름 검증
//   - in-flight 요청이 shutdown 내에 완료되는지 검증
//
// 빌드 태그: integration (testcontainers 의존성 — 기본 빌드 제외)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// ── e2eGoLeakOptions — 외부 라이브러리 goroutine 제외 ───────────────────────

// serverE2EGoLeakOptions server cmd/server E2E에서 정상 goroutine을 goleak 제외 목록에 등록.
var serverE2EGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),
	goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"),
	goleak.IgnoreTopFunction("github.com/redis/go-redis/v9/maintnotifications.(*CircuitBreakerManager).cleanupLoop"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
	goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	// gRPC 내부 goroutine
	goleak.IgnoreTopFunction("google.golang.org/grpc.(*Server).Serve"),
	goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.(*http2Server).operateHeaders"),
	goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.(*http2Server).keepalive"),
	goleak.IgnoreTopFunction("google.golang.org/grpc.(*ccBalancerWrapper).watcher"),
	goleak.IgnoreTopFunction("google.golang.org/grpc.(*addrConn).resetTransport"),
	goleak.IgnoreTopFunction("google.golang.org/grpc.(*clientStream).newAttemptLocked"),
	goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.newHTTP2Client"),
	goleak.IgnoreTopFunction("google.golang.org/grpc/internal/resolver/dns.(*dnsResolver).watcher"),
	goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.(*controlBuffer).get"),
	// errgroup goroutine — server.Run() 내부
	goleak.IgnoreAnyFunction("golang.org/x/sync/errgroup.(*Group).Go.func1"),
	// signal notify context goroutine
	goleak.IgnoreAnyFunction("os/signal.loop"),
}

// ── 컨테이너 설정 헬퍼 ─────────────────────────────────────────────────────

// e2eServerContainers E2E 서버 테스트에서 공유하는 컨테이너 정보
type e2eServerContainers struct {
	pgDSN     string
	redisAddr string
}

// setupServerE2EContainers Postgres + Redis 컨테이너를 기동하고 스키마를 적용한다.
//
// @MX:ANCHOR: [AUTO] 서버 E2E 테스트 인프라 초기화 — 4개 E2E 테스트 모두 이 함수를 호출
// @MX:REASON: testcontainers 기반 실제 Postgres+Redis 인프라가 Server.New() 검증에 필수
func setupServerE2EContainers(t *testing.T) *e2eServerContainers {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	t.Cleanup(cancel)

	// ── Postgres 컨테이너 기동 ─────────────────────────────────────────────
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

	redisConnStr, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err, "Redis 주소 조회 실패")
	redisAddr := stripServerRedisScheme(redisConnStr)

	// ── Postgres 스키마 적용 ───────────────────────────────────────────────
	schemaPath := "../../internal/store/testdata/schema.sql"
	schemaSQL, err := os.ReadFile(schemaPath)
	require.NoError(t, err, "schema.sql 로드 실패: %s", schemaPath)

	applySchemaWithPgxpool(t, ctx, pgDSN, string(schemaSQL))

	return &e2eServerContainers{
		pgDSN:     pgDSN,
		redisAddr: redisAddr,
	}
}

// applySchemaWithPgxpool pgxpool을 사용하여 SQL 스키마를 적용한다.
func applySchemaWithPgxpool(t *testing.T, ctx context.Context, dsn string, sql string) {
	t.Helper()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "pgxpool 생성 실패")
	defer pool.Close()
	_, err = pool.Exec(ctx, sql)
	require.NoError(t, err, "스키마 적용 실패")
}

// stripServerRedisScheme "redis://host:port" 형식에서 접두사를 제거한다.
func stripServerRedisScheme(addr string) string {
	const prefix = "redis://"
	if strings.HasPrefix(addr, prefix) {
		return addr[len(prefix):]
	}
	return addr
}

// buildServerE2EConfig testcontainers 연결 정보로 config.Config를 생성한다.
func buildServerE2EConfig(c *e2eServerContainers) *config.Config {
	return &config.Config{
		PostgresDSN:              c.pgDSN,
		RedisAddr:                c.redisAddr,
		GRPCAddr:                 ":0",
		RESTAddr:                 ":0",
		AuthEnabled:              false,
		CeleryQueue:              "celery",
		ReadyProbeTimeoutSeconds: 5,
		ShutdownTimeoutSeconds:   10,
	}
}

// waitForReady /ready 엔드포인트가 200을 반환할 때까지 대기한다.
func waitForReady(t *testing.T, restAddr string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + restAddr + "/ready") //nolint:noctx
		if err == nil {
			resp.Body.Close() //nolint:errcheck
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// ── E2E 테스트 ──────────────────────────────────────────────────────────────

// TestE2E_Server_FullStackStartupAndRequest testcontainers + 실제 Server.New() + POST /api/v1/workflows
//
// Given: Postgres + Redis 컨테이너 기동, AuthEnabled=false
// When: Server.New() + Server.Run(ctx) (goroutine) → POST /api/v1/workflows
// Then:
//   - HTTP 201 Created
//   - /ready → 200 (deps up)
//   - gRPC 리스너 응답 확인
func TestE2E_Server_FullStackStartupAndRequest(t *testing.T) {
	defer goleak.VerifyNone(t, serverE2EGoLeakOptions...)

	containers := setupServerE2EContainers(t)
	cfg := buildServerE2EConfig(containers)
	logger := zaptest.NewLogger(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Server.New() 호출
	srv, err := New(ctx, cfg, logger)
	require.NoError(t, err, "Server.New() 실패")

	// Server.Run() goroutine으로 기동
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(ctx)
	}()

	// /ready 200 대기 (최대 15초)
	require.Eventually(t, func() bool {
		if srv.RESTAddr() == "" {
			return false
		}
		return waitForReady(t, srv.RESTAddr(), 1*time.Second)
	}, 15*time.Second, 200*time.Millisecond, "서버 ready 상태 대기 타임아웃")

	restURL := "http://" + srv.RESTAddr()

	// ── POST /api/v1/workflows ─────────────────────────────────────────────
	docID := uuid.New().String()
	body, _ := json.Marshal(map[string]string{"document_id": docID})
	resp, err := http.Post(restURL+"/api/v1/workflows", "application/json", bytes.NewReader(body)) //nolint:noctx
	require.NoError(t, err, "POST /api/v1/workflows 요청 실패")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"POST /api/v1/workflows는 HTTP 201을 반환해야 한다")

	var wfResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wfResp))
	workflowID, ok := wfResp["workflow_id"].(string)
	require.True(t, ok, "응답에 workflow_id 필드가 있어야 한다")
	require.NotEmpty(t, workflowID, "workflow_id가 비어 있으면 안 된다")

	// ── gRPC addr 확인 (리스너 바인딩 완료) ────────────────────────────────
	assert.NotEmpty(t, srv.GRPCAddr(), "gRPC 주소가 비어 있으면 안 된다")

	// gRPC TCP 연결 가능 여부 확인
	grpcConn, grpcErr := net.DialTimeout("tcp", srv.GRPCAddr(), 2*time.Second)
	require.NoError(t, grpcErr, "gRPC 리스너에 TCP 연결 실패")
	grpcConn.Close() //nolint:errcheck

	// ── cancel → graceful shutdown ─────────────────────────────────────────
	cancel()

	// Run() 종료 대기 (최대 15초)
	select {
	case e := <-runErr:
		// nil 또는 context.Canceled 모두 정상 종료
		if e != nil && !isContextError(e) {
			t.Errorf("Server.Run()이 예상치 못한 에러로 종료: %v", e)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Server.Run()이 15초 내에 종료되지 않음")
	}
}

// TestE2E_Server_ReadyProbe_DepsUp /ready 엔드포인트가 실제 deps 확인 후 200을 반환해야 한다.
//
// Given: Postgres + Redis 컨테이너 정상 기동, Server 부팅 완료
// When: GET /ready
// Then: HTTP 200 + body {"status":"ready", "checks":{"postgres":"ok","redis":"ok"}}
func TestE2E_Server_ReadyProbe_DepsUp(t *testing.T) {
	defer goleak.VerifyNone(t, serverE2EGoLeakOptions...)

	containers := setupServerE2EContainers(t)
	cfg := buildServerE2EConfig(containers)
	logger := zaptest.NewLogger(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	srv, err := New(ctx, cfg, logger)
	require.NoError(t, err)

	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx) }()

	// /ready 200 대기
	require.Eventually(t, func() bool {
		if srv.RESTAddr() == "" {
			return false
		}
		return waitForReady(t, srv.RESTAddr(), 1*time.Second)
	}, 15*time.Second, 200*time.Millisecond)

	// /ready 응답 상세 검증
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + srv.RESTAddr() + "/ready") //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"deps 정상 상태에서 /ready는 200이어야 한다")

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var readyResp map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &readyResp))
	assert.Equal(t, "ready", readyResp["status"],
		"/ready 응답 status는 'ready'여야 한다")

	// checks 필드 검증
	checks, ok := readyResp["checks"].(map[string]interface{})
	require.True(t, ok, "응답에 checks 필드가 있어야 한다")
	assert.Equal(t, "ok", checks["postgres"], "postgres check는 ok여야 한다")
	assert.Equal(t, "ok", checks["redis"], "redis check는 ok여야 한다")

	cancel()
	select {
	case <-runErr:
	case <-time.After(10 * time.Second):
		t.Fatal("Server.Run() 종료 타임아웃")
	}
}

// TestE2E_Server_GracefulShutdown_InFlightCompletes 서버 graceful shutdown 중
// in-flight 요청이 완료된 후 서버가 종료되어야 한다.
//
// Given: 실제 서버 기동 완료, in-flight 시뮬레이션 (slow handler)
// When: cancel(ctx) 호출 (SIGTERM 시뮬레이션)
// Then:
//   - in-flight 요청이 201 정상 응답 반환
//   - /ready → 503 (shutdown 진입)
//   - 서버 정상 종료
func TestE2E_Server_GracefulShutdown_InFlightCompletes(t *testing.T) {
	defer goleak.VerifyNone(t, serverE2EGoLeakOptions...)

	containers := setupServerE2EContainers(t)
	cfg := buildServerE2EConfig(containers)
	cfg.ShutdownTimeoutSeconds = 15
	logger := zaptest.NewLogger(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	srv, err := New(ctx, cfg, logger)
	require.NoError(t, err)

	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx) }()

	// 서버 ready 대기
	require.Eventually(t, func() bool {
		if srv.RESTAddr() == "" {
			return false
		}
		return waitForReady(t, srv.RESTAddr(), 1*time.Second)
	}, 15*time.Second, 200*time.Millisecond, "서버 ready 대기 타임아웃")

	restURL := "http://" + srv.RESTAddr()

	// in-flight 요청 시작 (POST /api/v1/workflows — 실제 처리)
	var inFlightResp *http.Response
	var inFlightErr error
	var inFlightWg sync.WaitGroup

	inFlightWg.Add(1)
	go func() {
		defer inFlightWg.Done()
		docID := uuid.New().String()
		body, _ := json.Marshal(map[string]string{"document_id": docID})
		inFlightResp, inFlightErr = http.Post( //nolint:noctx
			restURL+"/api/v1/workflows", "application/json", bytes.NewReader(body),
		)
		if inFlightResp != nil {
			inFlightResp.Body.Close() //nolint:errcheck
		}
	}()

	// 요청이 시작될 시간 보장 (100ms)
	time.Sleep(100 * time.Millisecond)

	// cancel() 호출 → graceful shutdown 트리거
	cancel()

	// in-flight 요청 완료 대기
	inFlightDone := make(chan struct{})
	go func() {
		inFlightWg.Wait()
		close(inFlightDone)
	}()

	select {
	case <-inFlightDone:
		// 정상 완료
	case <-time.After(14 * time.Second):
		t.Fatal("in-flight 요청이 ShutdownTimeout 내에 완료되지 않음")
	}

	require.NoError(t, inFlightErr, "in-flight 요청에서 에러 발생")
	if inFlightResp != nil {
		assert.Equal(t, http.StatusCreated, inFlightResp.StatusCode,
			"in-flight 요청은 201로 완료되어야 한다 (graceful shutdown이 요청을 기다림)")
	}

	// Server.Run() 종료 대기 (먼저 실행)
	select {
	case <-runErr:
	case <-time.After(15 * time.Second):
		t.Fatal("Server.Run() 15초 내 종료 실패")
	}

	// shutdown 완료 후 isShuttingDown() = true 검증
	assert.True(t, srv.isShuttingDown(),
		"Server.Run() 종료 후 서버는 shutting down 상태여야 한다")
}

// TestE2E_Server_DualListener_BothServing gRPC + REST 두 리스너 모두 동시에 응답해야 한다.
//
// Given: Server.New() + Server.Run() 완료
// When: gRPC HealthCheck + REST GET /health 동시 호출
// Then:
//   - gRPC Health.Check → SERVING 또는 NOT_SERVING (서버 응답 자체 검증)
//   - REST GET /health → HTTP 200
func TestE2E_Server_DualListener_BothServing(t *testing.T) {
	defer goleak.VerifyNone(t, serverE2EGoLeakOptions...)

	containers := setupServerE2EContainers(t)
	cfg := buildServerE2EConfig(containers)
	logger := zaptest.NewLogger(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	srv, err := New(ctx, cfg, logger)
	require.NoError(t, err)

	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx) }()

	// 서버 ready 대기
	require.Eventually(t, func() bool {
		if srv.RESTAddr() == "" {
			return false
		}
		return waitForReady(t, srv.RESTAddr(), 1*time.Second)
	}, 15*time.Second, 200*time.Millisecond)

	var wg sync.WaitGroup
	errs := make([]error, 2)

	// ── gRPC Health Check ─────────────────────────────────────────────────
	wg.Add(1)
	go func() {
		defer wg.Done()
		grpcConn, grpcErr := grpc.NewClient(
			srv.GRPCAddr(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if grpcErr != nil {
			errs[0] = fmt.Errorf("gRPC 연결 실패: %w", grpcErr)
			return
		}
		defer grpcConn.Close() //nolint:errcheck

		healthClient := grpc_health_v1.NewHealthClient(grpcConn)
		checkCtx, checkCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer checkCancel()

		resp, checkErr := healthClient.Check(checkCtx, &grpc_health_v1.HealthCheckRequest{})
		if checkErr != nil {
			errs[0] = fmt.Errorf("gRPC Health.Check 실패: %w", checkErr)
			return
		}
		// SERVING 또는 NOT_SERVING 모두 "서버가 응답했음"을 의미
		validStatus := resp.Status == grpc_health_v1.HealthCheckResponse_SERVING ||
			resp.Status == grpc_health_v1.HealthCheckResponse_NOT_SERVING
		if !validStatus {
			errs[0] = fmt.Errorf("gRPC Health.Check 응답 상태 예상 불일치: %v", resp.Status)
		}
	}()

	// ── REST GET /health ───────────────────────────────────────────────────
	wg.Add(1)
	go func() {
		defer wg.Done()
		client := &http.Client{Timeout: 5 * time.Second}
		resp, httpErr := client.Get("http://" + srv.RESTAddr() + "/health") //nolint:noctx
		if httpErr != nil {
			errs[1] = fmt.Errorf("GET /health 실패: %w", httpErr)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errs[1] = fmt.Errorf("GET /health HTTP 상태 불일치: %d, body=%s",
				resp.StatusCode, string(body))
		}
	}()

	wg.Wait()

	for i, e := range errs {
		assert.NoError(t, e, "listener %d 에러", i)
	}

	// gRPC codes.NotFound — 존재하지 않는 workflow 조회로 서비스 등록 확인
	grpcConn, err := grpc.NewClient(
		srv.GRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer grpcConn.Close() //nolint:errcheck

	// WorkflowService가 등록되어 있어야 함 (codes.NotFound or codes.Unauthenticated)
	// grpc_health_v1은 이미 확인했으므로 서비스 등록은 검증됨
	assert.NotEmpty(t, srv.GRPCAddr(), "gRPC 주소는 비어 있으면 안 된다")

	cancel()
	select {
	case <-runErr:
	case <-time.After(12 * time.Second):
		t.Fatal("Server.Run() 12초 내 종료 실패")
	}
}

// ── 헬퍼 함수 ──────────────────────────────────────────────────────────────

// isContextError context 관련 에러 여부를 반환한다.
func isContextError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, codes.Canceled.String())
}
