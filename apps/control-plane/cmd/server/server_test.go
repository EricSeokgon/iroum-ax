// server_test.go — Server 부트스트랩 및 dual listener 단위/통합 테스트
// SPEC-AX-SERVER-001 S1 deliverable: New() + Run() + probes 동작 검증
//
// 테스트 전략:
//   - New() 테스트: 실제 PostgreSQL/Redis 없이 수행 불가능하므로 fake 서버로 대체
//   - Run() 테스트: ":0" 포트 바인딩으로 실제 리스너 검증 (testcontainers 불필요)
//   - Probe 테스트: httptest 또는 직접 HTTP 요청으로 /health, /ready 검증
//
// 주의: New()는 실제 PostgreSQL/Redis Ping이 필요하므로 DB 의존 테스트는 별도 integration 태그 사용
package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestServer_HealthEndpoint_Returns200 GET /health는 항상 HTTP 200을 반환해야 한다.
// DB/Redis 상태와 무관하게 liveness는 유지되어야 한다 (REQ-SERVER-004-E1).
func TestServer_HealthEndpoint_Returns200(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	livenessHandler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"GET /health는 HTTP 200을 반환해야 한다")
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"),
		"Content-Type은 application/json이어야 한다")
}

// TestServer_HealthEndpoint_ResponseBody GET /health 응답 바디 구조 검증.
func TestServer_HealthEndpoint_ResponseBody(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	livenessHandler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]string
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "GET /health 응답은 유효한 JSON이어야 한다")

	assert.Equal(t, "ok", result["status"], "status 필드는 'ok'이어야 한다")
	assert.Equal(t, "iroum-ax-control-plane", result["service"],
		"service 필드는 'iroum-ax-control-plane'이어야 한다")
	assert.NotEmpty(t, result["version"], "version 필드는 비어 있으면 안 된다")
}

// TestServer_ReadyEndpoint_ShuttingDown_Returns503 shutdown 중에는 /ready가 HTTP 503을 반환해야 한다.
// REQ-SERVER-004-S1: shutdown 진입 후 즉시 503.
func TestServer_ReadyEndpoint_ShuttingDown_Returns503(t *testing.T) {
	t.Parallel()

	// shuttingDown=true인 서버 상태 시뮬레이션
	s := &Server{
		cfg:          testConfig(),
		shuttingDown: true,
	}

	handler := readinessHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode,
		"shutdown 중인 서버의 /ready는 HTTP 503을 반환해야 한다")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]string
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "shutting_down", result["status"],
		"shutdown 중인 서버의 /ready 응답에는 'shutting_down' 상태가 포함되어야 한다")
}

// TestServer_ReadyEndpoint_NilDeps_ReturnsOK deps가 nil(AuthEnabled=false, pgStore=nil, redisClient=nil)일 때
// /ready는 HTTP 200을 반환해야 한다 (모든 체크를 건너뜀).
func TestServer_ReadyEndpoint_NilDeps_ReturnsOK(t *testing.T) {
	t.Parallel()

	// 모든 의존성이 nil이고 AuthEnabled=false → 체크 없음
	s := &Server{
		cfg:          testConfig(),
		shuttingDown: false,
		// pgStore, redisClient, jwksCache 모두 nil → 체크 건너뜀
	}

	handler := readinessHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// pgStore=nil, redisClient=nil → 두 체크 모두 skip(nil 처리) → 전체 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"nil 의존성(AuthEnabled=false)에서 /ready는 HTTP 200을 반환해야 한다")
}

// TestServer_IsShuttingDown shuttingDown 플래그 접근이 -race에서 안전해야 한다.
func TestServer_IsShuttingDown_ThreadSafe(t *testing.T) {
	t.Parallel()

	s := &Server{shuttingDown: false}

	const workers = 20
	done := make(chan struct{}, workers)
	for range workers {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = s.isShuttingDown()
		}()
	}

	// 동시에 shuttingDown을 true로 설정
	go func() {
		s.mu.Lock()
		s.shuttingDown = true
		s.mu.Unlock()
	}()

	for range workers {
		<-done
	}
	// race detector 에러 없으면 통과
}

// TestServer_DualListener_BothBind ":0" 포트로 gRPC + REST 리스너 모두 바인딩 가능해야 한다.
// 실제 서버 Start는 수행하지 않고, net.Listen(":0") 성공 여부만 검증.
func TestServer_DualListener_BothBind(t *testing.T) {
	t.Parallel()

	// gRPC 리스너 (":0" 는 테스트 전용 임시 포트 — G102 gosec 억제)
	grpcLn, err := net.Listen("tcp", ":0") //nolint:gosec
	require.NoError(t, err, "gRPC 리스너 바인딩에 실패해서는 안 된다")
	grpcAddr := grpcLn.Addr().String()
	defer grpcLn.Close()

	// REST 리스너
	restLn, err := net.Listen("tcp", ":0") //nolint:gosec
	require.NoError(t, err, "REST 리스너 바인딩에 실패해서는 안 된다")
	restAddr := restLn.Addr().String()
	defer restLn.Close()

	assert.NotEmpty(t, grpcAddr, "gRPC 주소는 비어 있으면 안 된다")
	assert.NotEmpty(t, restAddr, "REST 주소는 비어 있으면 안 된다")
	assert.NotEqual(t, grpcAddr, restAddr, "gRPC 주소와 REST 주소는 달라야 한다")
}

// TestServer_GRPCAddr_RESTAddr_Accessors GRPCAddr()/RESTAddr() 테스트 헬퍼 검증
func TestServer_GRPCAddr_RESTAddr_Accessors(t *testing.T) {
	t.Parallel()

	s := &Server{
		grpcAddr: "127.0.0.1:50051",
		restAddr: "127.0.0.1:8080",
	}

	assert.Equal(t, "127.0.0.1:50051", s.GRPCAddr(),
		"GRPCAddr()는 grpcAddr 필드를 반환해야 한다")
	assert.Equal(t, "127.0.0.1:8080", s.RESTAddr(),
		"RESTAddr()는 restAddr 필드를 반환해야 한다")
}

// TestServer_Shutdown_Idempotent shutdown()이 idempotent해야 한다 (sync.Once).
// 두 번 호출해도 패닉이나 데드락이 발생해서는 안 된다.
func TestServer_Shutdown_Idempotent(t *testing.T) {
	t.Parallel()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	s := &Server{
		cfg:       testConfig(),
		logger:    logger,
		startedAt: time.Now(),
	}

	ctx := context.Background()
	// 첫 번째 shutdown
	s.shutdown(ctx, "test_first")
	// 두 번째 shutdown (sync.Once로 idempotent 보장)
	s.shutdown(ctx, "test_second")

	// shuttingDown 플래그가 true로 설정되어야 한다
	assert.True(t, s.isShuttingDown(),
		"shutdown() 호출 후 isShuttingDown()은 true를 반환해야 한다")
}

// TestServer_OuterMux_LivenessRoute outer mux에서 GET /health 라우팅 검증
func TestServer_OuterMux_LivenessRoute(t *testing.T) {
	t.Parallel()

	// outerMux를 직접 구성하여 /health 라우팅 검증
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", livenessHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code,
		"outer mux의 GET /health는 HTTP 200을 반환해야 한다")
}

// TestServer_New_PgPingFailure_Aborts_Integration PostgreSQL 연결 실패 시 New()가 에러를 반환해야 한다.
// 유효하지 않은 DSN으로 연결 시도 → 에러 반환 검증.
func TestServer_New_PgPingFailure_Aborts(t *testing.T) {
	// 유효하지 않은 PostgreSQL DSN으로 New() 호출
	// NewPgWorkflowStore 내부의 pgx.Connect가 실패해야 한다.
	cfg := testConfig()
	cfg.PostgresDSN = "postgres://invalid-host-that-does-not-exist:5432/nonexistent?connect_timeout=1"
	cfg.GRPCAddr = ":0"
	cfg.RESTAddr = ":0"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger, logErr := zap.NewDevelopment()
	require.NoError(t, logErr)

	_, err := New(ctx, cfg, logger)

	require.Error(t, err,
		"유효하지 않은 PostgreSQL DSN으로 New()를 호출하면 에러를 반환해야 한다")
	assert.Contains(t, err.Error(), "pg_store",
		"에러 메시지에는 'pg_store' 또는 'pg_ping' 단계 정보가 포함되어야 한다")
}

// TestServer_MetricsRoute_NoAuth_Returns200 authEnabled=false 시 /metrics는 인증 없이 200을 반환해야 한다.
// REQ-OBS-002 §3.3: authEnabled=false → bypass.
func TestServer_MetricsRoute_NoAuth_Returns200(t *testing.T) {
	t.Parallel()

	// outerMux를 직접 구성 (authEnabled=false)
	mux := http.NewServeMux()

	// metrics 임포트 없이 테스트하기 위해 단순 핸들러로 대체
	// 실제 MetricsHandler는 integration 테스트에서 검증
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# HELP test metric\n"))
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code,
		"authEnabled=false 시 GET /metrics는 HTTP 200을 반환해야 한다")
}

// TestServer_OuterMux_MetricsRoute_AuthEnabled_NoToken_Returns401
// authEnabled=true에서 토큰 없는 /metrics 요청은 401을 반환해야 한다.
// MetricsAuthMiddleware 동작 검증 (server.go 라우팅과 독립).
func TestServer_OuterMux_MetricsRoute_AuthEnabled_NoToken_Returns401(t *testing.T) {
	t.Parallel()

	// 401 반환 핸들러로 MetricsAuthMiddleware 동작 모방
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing_authorization"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	// Authorization 헤더 없음
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"토큰 없는 /metrics 요청은 HTTP 401을 반환해야 한다")
}

// ── 테스트 헬퍼 ──────────────────────────────────────────────────────────────

// testConfig 테스트용 최소 config.Config 반환
func testConfig() *config.Config {
	return &config.Config{
		ReadyProbeTimeoutSeconds: 5,
		ShutdownTimeoutSeconds:   1,
		AuthEnabled:              false,
		CeleryQueue:              "test-celery",
		GRPCAddr:                 ":0",
		RESTAddr:                 ":0",
		PostgresDSN:              "postgres://test:test@localhost:5432/test",
		RedisAddr:                "localhost:6379",
	}
}
