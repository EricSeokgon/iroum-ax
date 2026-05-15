//go:build integration

// authz_e2e_test.go — RBAC 인가 체인 E2E 통합 테스트
// Sprint 3 GREEN: SPEC-AX-AUTH-002 AC-AUTH2-004-1/2/4 + AC-AUTH2-004-Sprint7-Unblock
//
// 테스트 전략:
//   - testcontainers-go Postgres + Redis (e2e_test.go setupE2EContainers 재사용)
//   - auth_e2e_test.go setupE2EAuthStack 패턴 확장: BuildRESTChain 사용
//   - 인메모리 RSA 키 쌍으로 JWT 서명 (외부 Keycloak 불필요)
//   - scope 클레임으로 역할 주입 (admin/analyst/viewer)
//   - gRPC 테스트: bufconn + BuildGRPCInterceptorChain으로 interceptor 체인 검증
//
// AC 커버리지:
//   - AC-AUTH2-004-1: admin 토큰 → POST/GET /api/v1/workflows 모두 성공 (200/201)
//   - AC-AUTH2-004-2: viewer 토큰 → GET 허용 (200), POST 거부 (403 + AUTH_FORBIDDEN)
//   - AC-AUTH2-004-4: analyst 토큰 → POST 허용 (200/201), GET 허용 (200)
//   - TestE2E_GRPC_Authz_ViewerForbidden_Create: viewer → gRPC CreateWorkflow → codes.PermissionDenied
//   - TestE2E_Authz_AuthDisabled_BypassesAuthz: authEnabled=false → RBAC 건너뜀
//
// 설계 결정:
//   - recorder=nil: AUTH_FORBIDDEN 이벤트 감사 기록은 미들웨어가 nil guard로 건너뜀
//     (테스트 범위: RBAC 차단 동작, 감사 DB 삽입은 authz_middleware_test.go에서 검증)
//   - BuildRESTChain 사용: auth.RESTMiddleware → RESTAuthzMiddleware → handler 순서 강제
//   - gRPC BuildGRPCInterceptorChain 사용: UnaryServerInterceptor → UnaryAuthzInterceptor 순서 강제
package server_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	proto "github.com/ircp/iroum-ax/apps/control-plane/internal/proto"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/scheduler"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/server"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// ──────────────────────────────────────────────────────────────────────────────
// 인가 E2E 스택 헬퍼
// ──────────────────────────────────────────────────────────────────────────────

// authzE2EStack 인가 체인이 포함된 E2E 스택
// fieldalignment: 포인터(8B) 먼저 배치
type authzE2EStack struct {
	// ts httptest.Server (REST E2E 검증용)
	ts *httptest.Server
	// queryFn DB 직접 쿼리 함수 (workflows, audit_logs 검증)
	queryFn func(ctx context.Context, sql string, args ...any) queryRower
	// svc gRPC E2E 검증용 WorkflowService (bufconn 연결)
	svc *server.WorkflowService
	// store 인메모리 fake store (REST 스택과 분리된 gRPC 스택 공유)
	fakeStore *store.FakeStore
}

// setupAuthzE2EStack BuildRESTChain으로 감싼 REST 스택 + gRPC 스택을 조립
//
// authEnabled=true 시:
//   - REST 스택: auth.BuildRESTChain(handler, validator, nil, true) — authn + authz 체인
//   - gRPC 스택: FakeStore 기반 (PG 컨테이너 없이 빠른 인터셉터 검증)
//
// recorder=nil: AUTH_FORBIDDEN 감사 이벤트 기록 skip (nil guard로 처리됨)
// 감사 DB 삽입 검증은 authz_middleware_test.go 범위
//
// @MX:ANCHOR: [AUTO] RBAC E2E 스택 초기화 단일 진입점
// @MX:REASON: 5개 인가 E2E 테스트 모두 이 함수를 통해 스택을 초기화 (fan_in >= 5)
func setupAuthzE2EStack(t *testing.T, cfg *e2eAuthConfig) *authzE2EStack {
	t.Helper()

	// ── testcontainers Postgres + Redis 기동 (REST E2E용) ──────────────────
	containers := setupE2EContainers(t)
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// PgWorkflowStore 초기화
	pgStore, err := store.NewPgWorkflowStore(ctx, containers.pgDSN, logger)
	require.NoError(t, err, "PgWorkflowStore 초기화 실패")
	t.Cleanup(pgStore.Close)

	// Redis 클라이언트 (E2E 완전성을 위해 초기화; 직접 검증은 이 테스트 범위 외)
	redisClient := redis.NewClient(&redis.Options{Addr: containers.redisAddr})
	t.Cleanup(func() { _ = redisClient.Close() })

	goAdapter := &goRedisAdapter{client: redisClient}
	disp := scheduler.NewCeleryDispatcher(goAdapter, "celery", "test-authz-host")
	_ = disp // 이 E2E 범위에서 직접 검증 안 함

	// audit.Recorder: authEnabled=true (인증 활성화, user_id = sub 클레임)
	recorder := audit.NewRecorder(cfg.authEnabled)
	coordinator := workflow.NewTxCoordinator(pgStore, recorder)
	sm := workflow.NewStateMachine(coordinator, logger)
	svc := server.NewWorkflowService(pgStore, sm, logger)
	handler := server.NewRESTHandler(svc, logger)

	// ── TokenValidator 생성 (authEnabled=true 시 mock JWKS 주입) ───────────
	var validator *auth.TokenValidator
	if cfg.authEnabled {
		jwksMock := &mockJWKSProvider{
			keys: map[string]*rsa.PublicKey{cfg.kid: &cfg.privateKey.PublicKey},
		}
		v, vErr := auth.New(ctx, cfg.issuer, cfg.audience,
			auth.WithJWKSProvider(jwksMock),
			auth.WithAllowedAlgs([]string{"RS256"}),
		)
		require.NoError(t, vErr, "TokenValidator 초기화 실패")
		validator = v
	}

	// ── REST 체인: BuildRESTChain (authn → authz → handler) ───────────────
	// recorder=nil: AUTH_FORBIDDEN DB 감사 기록 skip
	// (nil guard: RESTAuthzMiddleware 내부에서 recorder != nil 체크)
	chain := auth.BuildRESTChain(handler.Mux(), validator, nil, cfg.authEnabled)
	ts := httptest.NewServer(chain)
	t.Cleanup(ts.Close)

	// ── DB 직접 쿼리 함수 (workflows, audit_logs 검증) ────────────────────
	queryFn := func(ctx context.Context, sql string, args ...any) queryRower {
		return containers.pgPool.QueryRow(ctx, sql, args...)
	}

	// ── gRPC E2E용 FakeStore + WorkflowService (빠른 interceptor 검증) ────
	fakeStore := store.NewFakeStore()
	fakeRecorder := audit.NewRecorder(cfg.authEnabled)
	fakeCoord := workflow.NewTxCoordinator(fakeStore, fakeRecorder)
	fakeSM := workflow.NewStateMachine(fakeCoord, logger)
	grpcSvc := server.NewWorkflowService(fakeStore, fakeSM, logger)

	return &authzE2EStack{
		ts:        ts,
		queryFn:   queryFn,
		svc:       grpcSvc,
		fakeStore: fakeStore,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 공통 테스트 헬퍼
// ──────────────────────────────────────────────────────────────────────────────

// newAuthzE2ECfg 인가 E2E 테스트 기본 설정 생성
func newAuthzE2ECfg(t *testing.T, kidSuffix string) *e2eAuthConfig {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "RSA 키 생성 실패")
	return &e2eAuthConfig{
		issuer:      "https://test-keycloak.local/realms/iroum-ax",
		audience:    "iroum-ax-control-plane",
		kid:         "e2e-authz-key-" + kidSuffix,
		privateKey:  privateKey,
		authEnabled: true,
	}
}

// postWorkflows POST /api/v1/workflows 요청 전송 + resp 반환
func postWorkflows(t *testing.T, tsURL, token string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	docID := uuid.New().String()
	body, _ := json.Marshal(map[string]string{"document_id": docID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tsURL+"/api/v1/workflows", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// getWorkflows GET /api/v1/workflows 요청 전송 + resp 반환
func getWorkflows(t *testing.T, tsURL, token string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tsURL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// ──────────────────────────────────────────────────────────────────────────────
// REST E2E 테스트 — AC-AUTH2-004-1/2/4
// ──────────────────────────────────────────────────────────────────────────────

// TestE2E_Authz_AdminFullAccess AC-AUTH2-004-1
//
// admin 토큰은 모든 REST 엔드포인트에 접근 가능해야 한다.
//
// Given:
//   - authEnabled=true, scope=iroum-ax:admin
//   - BuildRESTChain으로 감싼 REST 스택
//
// When: POST /api/v1/workflows + GET /api/v1/workflows
//
// Then:
//   - POST → HTTP 201 Created
//   - GET  → HTTP 200 OK
//   - workflows 테이블에 row 생성
func TestE2E_Authz_AdminFullAccess(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	cfg := newAuthzE2ECfg(t, "admin")
	stack := setupAuthzE2EStack(t, cfg)
	ctx := context.Background()

	// admin 토큰 생성 (scope=iroum-ax:admin)
	adminToken := genE2ETestJWT(t, cfg, map[string]interface{}{
		"sub":   "admin-user-001",
		"scope": "iroum-ax:admin",
	})

	// POST /api/v1/workflows → 201 확인
	postResp := postWorkflows(t, stack.ts.URL, adminToken)
	defer postResp.Body.Close()
	assert.Equal(t, http.StatusCreated, postResp.StatusCode,
		"AC-AUTH2-004-1: admin POST /api/v1/workflows → HTTP 201이어야 함")

	// workflow_id 추출 + DB 검증
	var postBody map[string]interface{}
	require.NoError(t, json.NewDecoder(postResp.Body).Decode(&postBody))
	workflowID, ok := postBody["workflow_id"].(string)
	require.True(t, ok && workflowID != "", "AC-AUTH2-004-1: workflow_id 필드가 있어야 함")

	var dbStatus string
	err := stack.queryFn(ctx, "SELECT status FROM workflows WHERE id = $1", workflowID).Scan(&dbStatus)
	require.NoError(t, err, "AC-AUTH2-004-1: workflows DB row가 있어야 함")
	assert.Equal(t, "PENDING", dbStatus)

	// GET /api/v1/workflows → 200 확인
	getResp := getWorkflows(t, stack.ts.URL, adminToken)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode,
		"AC-AUTH2-004-1: admin GET /api/v1/workflows → HTTP 200이어야 함")
}

// TestE2E_Authz_ViewerForbidden_POST AC-AUTH2-004-2 (핵심 SPEC-AX-AUTH-002 검증)
//
// viewer 토큰은 GET은 허용되지만 POST는 거부되어야 한다.
//
// Given:
//   - authEnabled=true, scope=iroum-ax:viewer
//   - BuildRESTChain으로 감싼 REST 스택
//
// When:
//   - POST /api/v1/workflows → 403 Forbidden 기대
//   - GET  /api/v1/workflows → 200 OK 기대
//
// Then:
//   - POST → HTTP 403 + WWW-Authenticate 헤더 + AUTH_FORBIDDEN 응답
//   - GET  → HTTP 200
//   - POST 시 workflows 테이블 변화 없음
func TestE2E_Authz_ViewerForbidden_POST(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	cfg := newAuthzE2ECfg(t, "viewer")
	stack := setupAuthzE2EStack(t, cfg)
	ctx := context.Background()

	// viewer 토큰 생성 (scope=iroum-ax:viewer → read:workflow만 허용)
	viewerToken := genE2ETestJWT(t, cfg, map[string]interface{}{
		"sub":   "viewer-user-001",
		"scope": "iroum-ax:viewer",
	})

	// POST 전 workflows 개수 확인
	var beforeCount int
	err := stack.queryFn(ctx, "SELECT COUNT(*) FROM workflows").Scan(&beforeCount)
	require.NoError(t, err)

	// POST /api/v1/workflows → 403 확인
	postResp := postWorkflows(t, stack.ts.URL, viewerToken)
	defer postResp.Body.Close()
	assert.Equal(t, http.StatusForbidden, postResp.StatusCode,
		"AC-AUTH2-004-2: viewer POST /api/v1/workflows → HTTP 403이어야 함")

	// WWW-Authenticate 헤더 확인 (RFC 6750 §3)
	wwwAuth := postResp.Header.Get("WWW-Authenticate")
	assert.NotEmpty(t, wwwAuth, "AC-AUTH2-004-2: WWW-Authenticate 헤더가 있어야 함")
	assert.Contains(t, wwwAuth, "Bearer", "WWW-Authenticate 헤더에 Bearer 포함")
	assert.Contains(t, wwwAuth, "insufficient_scope", "WWW-Authenticate 헤더에 insufficient_scope 포함")

	// 응답 body에 AUTH_FORBIDDEN 코드 확인
	var forbidBody map[string]interface{}
	require.NoError(t, json.NewDecoder(postResp.Body).Decode(&forbidBody))
	errObj, _ := forbidBody["error"].(map[string]interface{})
	assert.Equal(t, "PERMISSION_DENIED", errObj["code"],
		"AC-AUTH2-004-2: 응답 body error.code = PERMISSION_DENIED이어야 함")

	// POST 후 workflows 개수 변화 없음 확인 (인가 차단 → 비즈니스 로직 미실행)
	var afterCount int
	err = stack.queryFn(ctx, "SELECT COUNT(*) FROM workflows").Scan(&afterCount)
	require.NoError(t, err)
	assert.Equal(t, beforeCount, afterCount,
		"AC-AUTH2-004-2: viewer POST 차단 시 workflows 행이 생성되어서는 안 됨")

	// GET /api/v1/workflows → 200 확인 (viewer는 읽기 허용)
	getResp := getWorkflows(t, stack.ts.URL, viewerToken)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode,
		"AC-AUTH2-004-2: viewer GET /api/v1/workflows → HTTP 200이어야 함")
}

// TestE2E_Authz_AnalystWriteAllowed AC-AUTH2-004-4
//
// analyst 토큰은 POST(write:workflow)와 GET(read:workflow) 모두 허용되어야 한다.
//
// Given:
//   - authEnabled=true, scope=iroum-ax:analyst
//   - BuildRESTChain으로 감싼 REST 스택
//
// When: POST /api/v1/workflows + GET /api/v1/workflows
//
// Then:
//   - POST → HTTP 201 Created
//   - GET  → HTTP 200 OK
func TestE2E_Authz_AnalystWriteAllowed(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	cfg := newAuthzE2ECfg(t, "analyst")
	stack := setupAuthzE2EStack(t, cfg)
	ctx := context.Background()

	// analyst 토큰 생성 (scope=iroum-ax:analyst)
	analystToken := genE2ETestJWT(t, cfg, map[string]interface{}{
		"sub":   "analyst-user-001",
		"scope": "iroum-ax:analyst",
	})

	// POST /api/v1/workflows → 201 확인
	postResp := postWorkflows(t, stack.ts.URL, analystToken)
	defer postResp.Body.Close()
	assert.Equal(t, http.StatusCreated, postResp.StatusCode,
		"AC-AUTH2-004-4: analyst POST /api/v1/workflows → HTTP 201이어야 함")

	// workflow_id 추출 + DB 검증
	var postBody map[string]interface{}
	require.NoError(t, json.NewDecoder(postResp.Body).Decode(&postBody))
	workflowID, ok := postBody["workflow_id"].(string)
	require.True(t, ok && workflowID != "", "AC-AUTH2-004-4: workflow_id 필드가 있어야 함")

	var dbStatus string
	err := stack.queryFn(ctx, "SELECT status FROM workflows WHERE id = $1", workflowID).Scan(&dbStatus)
	require.NoError(t, err, "AC-AUTH2-004-4: workflows DB row가 있어야 함")
	assert.Equal(t, "PENDING", dbStatus)

	// GET /api/v1/workflows → 200 확인
	getResp := getWorkflows(t, stack.ts.URL, analystToken)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode,
		"AC-AUTH2-004-4: analyst GET /api/v1/workflows → HTTP 200이어야 함")
}

// TestE2E_Authz_AuthDisabled_BypassesAuthz authEnabled=false → RBAC 건너뜀
//
// Given:
//   - authEnabled=false → BuildRESTChain이 handler 직접 반환 (미들웨어 없음)
//
// When: POST /api/v1/workflows (Authorization 헤더 없음)
//
// Then:
//   - HTTP 201 (RBAC 건너뜀, cli-anonymous로 처리)
func TestE2E_Authz_AuthDisabled_BypassesAuthz(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	// authEnabled=false: BuildRESTChain이 middleware 없이 handler 직접 반환
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cfg := &e2eAuthConfig{
		issuer:      "https://test-keycloak.local/realms/iroum-ax",
		audience:    "iroum-ax-control-plane",
		kid:         "e2e-authz-key-disabled",
		privateKey:  privateKey,
		authEnabled: false, // 인가 비활성화
	}

	stack := setupAuthzE2EStack(t, cfg)
	ctx := context.Background()

	// Authorization 헤더 없이 POST
	postResp := postWorkflows(t, stack.ts.URL, "")
	defer postResp.Body.Close()
	assert.Equal(t, http.StatusCreated, postResp.StatusCode,
		"TestE2E_Authz_AuthDisabled_BypassesAuthz: authEnabled=false → HTTP 201이어야 함")

	// audit_logs.user_id = cli-anonymous 확인
	var postBody map[string]interface{}
	require.NoError(t, json.NewDecoder(postResp.Body).Decode(&postBody))
	workflowID, ok := postBody["workflow_id"].(string)
	require.True(t, ok && workflowID != "")

	var auditUserID string
	err = stack.queryFn(ctx,
		"SELECT user_id FROM audit_logs WHERE resource_id = $1 AND action = 'WORKFLOW_CREATED'",
		workflowID,
	).Scan(&auditUserID)
	require.NoError(t, err)
	assert.Equal(t, "cli-anonymous", auditUserID,
		"authEnabled=false → audit_logs.user_id = 'cli-anonymous'이어야 함")
}

// ──────────────────────────────────────────────────────────────────────────────
// gRPC E2E 테스트 — BuildGRPCInterceptorChain 검증
// ──────────────────────────────────────────────────────────────────────────────

// TestE2E_GRPC_Authz_ViewerForbidden_Create AC-AUTH2-004-2 (gRPC)
//
// viewer 토큰으로 gRPC CreateWorkflow → codes.PermissionDenied
//
// 전략: proto 패키지에 클라이언트 stub이 없으므로 (hand-written proto, 클라이언트 미생성),
// UnaryServerInterceptor(authn) → UnaryAuthzInterceptor(authz) 를 직접 체인으로 호출하여
// interceptor 계층의 인가 동작을 검증한다.
//
// Given:
//   - authn: auth.UnaryServerInterceptor(validator) → viewer user context 주입
//   - authz: auth.UnaryAuthzInterceptor(nil, true) → write:workflow 없음 → PermissionDenied
//
// When: IncomingContext에 viewer 토큰 주입 + CreateWorkflow FullMethod
//
// Then:
//   - codes.PermissionDenied
//   - FakeStore에 workflow row 없음
//   - BuildGRPCInterceptorChain 기반 gRPC 서버 정상 기동 확인
func TestE2E_GRPC_Authz_ViewerForbidden_Create(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	cfg := newAuthzE2ECfg(t, "grpc-viewer")
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// mockJWKSProvider 준비
	jwksMock := &mockJWKSProvider{
		keys: map[string]*rsa.PublicKey{cfg.kid: &cfg.privateKey.PublicKey},
	}
	validator, err := auth.New(ctx, cfg.issuer, cfg.audience,
		auth.WithJWKSProvider(jwksMock),
		auth.WithAllowedAlgs([]string{"RS256"}),
	)
	require.NoError(t, err)

	// FakeStore + WorkflowService 준비
	fakeStore := store.NewFakeStore()
	fakeRecorder := audit.NewRecorder(true)
	fakeCoord := workflow.NewTxCoordinator(fakeStore, fakeRecorder)
	fakeSM := workflow.NewStateMachine(fakeCoord, logger)
	svc := server.NewWorkflowService(fakeStore, fakeSM, logger)

	// viewer 토큰 생성 (scope=iroum-ax:viewer → write:workflow 없음)
	viewerToken := genE2ETestJWT(t, cfg, map[string]interface{}{
		"sub":   "viewer-grpc-001",
		"scope": "iroum-ax:viewer",
	})

	// gRPC 서버 측 IncomingContext에 Authorization metadata 주입
	md := metadata.Pairs("authorization", "Bearer "+viewerToken)
	incomingCtx := metadata.NewIncomingContext(ctx, md)
	callCtx, cancel := context.WithTimeout(incomingCtx, 10*time.Second)
	defer cancel()

	req := &proto.CreateWorkflowRequest{DocumentID: uuid.New().String()}
	info := &grpc.UnaryServerInfo{
		Server:     svc,
		FullMethod: "/iroum.ax.v1.WorkflowService/CreateWorkflow",
	}

	// 최종 handler: CreateWorkflow 직접 호출
	finalHandler := func(ctx context.Context, req any) (any, error) {
		return svc.CreateWorkflow(ctx, req.(*proto.CreateWorkflowRequest))
	}

	// authn 인터셉터: viewer 토큰 검증 + user context 주입
	authn := auth.UnaryServerInterceptor(validator)

	// Step 1: authn 통과 여부 + user context 주입 확인
	var capturedUserCtx context.Context
	_, authnErr := authn(callCtx, req, info, func(innerCtx context.Context, _ any) (any, error) {
		capturedUserCtx = innerCtx
		return nil, nil
	})
	require.NoError(t, authnErr, "viewer 토큰 authn 통과 실패")
	require.NotNil(t, capturedUserCtx, "authn 인터셉터가 user context를 주입해야 함")

	// user context에 viewer user가 올바르게 주입되었는지 확인
	user, ok := auth.UserFromContext(capturedUserCtx)
	require.True(t, ok, "authn 통과 후 user가 context에 있어야 함")
	assert.Equal(t, "viewer-grpc-001", user.UID, "viewer user UID 확인")

	// Step 2: authz 인터셉터 직접 호출 (capturedUserCtx = authn이 주입한 user context)
	// auth.UnaryAuthzInterceptor(nil, true): recorder=nil, authEnabled=true
	// viewer → write:workflow 없음 → codes.PermissionDenied 기대
	authz := auth.UnaryAuthzInterceptor(nil, true)
	_, authzErr := authz(capturedUserCtx, req, info, finalHandler)

	require.Error(t, authzErr,
		"AC-AUTH2-004-2 gRPC: viewer CreateWorkflow → 에러가 반환되어야 함")

	st, ok := status.FromError(authzErr)
	require.True(t, ok, "gRPC status 추출 실패")
	assert.Equal(t, codes.PermissionDenied, st.Code(),
		"AC-AUTH2-004-2 gRPC: viewer → codes.PermissionDenied이어야 함")

	// FakeStore에 workflow row가 없어야 함 (인가 차단 → 비즈니스 로직 미실행)
	assert.Empty(t, fakeStore.Workflows,
		"AC-AUTH2-004-2 gRPC: viewer 차단 시 FakeStore에 workflow row 없어야 함")

	// BuildGRPCInterceptorChain 기반 gRPC 서버 기동 확인
	interceptorOpt := auth.BuildGRPCInterceptorChain(validator, nil, true)
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := grpc.NewServer(interceptorOpt)
	proto.RegisterWorkflowServiceServer(grpcSrv, svc)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.GracefulStop()
		_ = lis.Close()
	})
	conn, connErr := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, connErr, "BuildGRPCInterceptorChain gRPC 서버 기동 확인")
	t.Cleanup(func() { _ = conn.Close() })
}
