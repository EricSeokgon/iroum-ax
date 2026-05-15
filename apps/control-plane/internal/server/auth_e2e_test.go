//go:build integration

// auth_e2e_test.go — 인증 체인 E2E 통합 테스트
// Sprint 7 GREEN: AC-AUTH-E2E-1 / AC-AUTH-E2E-2 / AC-AUTH-E2E-4 + 동시성
//
// 테스트 전략:
//   - testcontainers-go Postgres + Redis (e2e_test.go setupE2EContainers 재사용)
//   - 인메모리 RSA 키 쌍으로 테스트용 JWT 서명 (외부 Keycloak 불필요)
//   - TokenValidator에 mockJWKSProvider 주입하여 실제 검증 체인 검증
//   - AuthEnabled=true: user_id = JWT sub 클레임 → audit_logs 전파 확인
//   - AuthEnabled=false: Authorization 헤더 없음 → user_id = cli-anonymous 확인
//
// AC 커버리지:
//   - AC-AUTH-E2E-1: AuthEnabled=true 전체 인증 체인 (HTTP 201 + DB audit + user_id 전파)
//   - AC-AUTH-E2E-2: AuthEnabled=false 백워드 호환 (cli-anonymous)
//   - AC-AUTH-E2E-4: 잘못된 서명 토큰 → HTTP 401 + workflows 미생성
//   - AC-AUTH-UBI-001-C: cli-anonymous 폴백 확인
//   - TestE2E_Auth_ConcurrentRequests: 5개 goroutine, 서로 다른 user_id → 격리 확인
//
// AC-AUTH-E2E-3 (RBAC Forbidden) 결정:
// Option B 선택 — RBAC 라이브러리(Sprint 5)는 완성, REST 핸들러 연동은 SPEC-AX-AUTH-002 후속
// TestE2E_Auth_RBACForbidden은 t.Skip()으로 마킹, TODO 주석으로 근거 문서화
package server_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/scheduler"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/server"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"
)

// ──────────────────────────────────────────────────────────────────────────────
// mockJWKSProvider — 테스트용 인메모리 JWKS 제공자
// ──────────────────────────────────────────────────────────────────────────────

// mockJWKSProvider 테스트 전용 RSA 공개키를 JWKS Provider 인터페이스로 래핑
// auth.JWKSProvider를 구현하여 TokenValidator에 주입 가능
type mockJWKSProvider struct {
	// kid → 공개키 맵 (테스트 E2E 키 페어)
	keys map[string]*rsa.PublicKey
}

// GetKey kid에 대응하는 RSA 공개키를 반환 (alg=RS256, kty=RSA 고정)
func (m *mockJWKSProvider) GetKey(_ context.Context, kid string) (any, string, string, error) {
	pub, ok := m.keys[kid]
	if !ok {
		return nil, "", "", fmt.Errorf("mock JWKS: kid=%s 없음", kid)
	}
	return pub, "RS256", "RSA", nil
}

// ──────────────────────────────────────────────────────────────────────────────
// 테스트 헬퍼 함수
// ──────────────────────────────────────────────────────────────────────────────

// e2eAuthConfig auth E2E 테스트 설정 파라미터
// fieldalignment: 포인터(8B) 먼저, string(16B) 다음, bool(1B) 마지막
type e2eAuthConfig struct {
	// privateKey RSA 서명 키 (테스트 인메모리 생성)
	privateKey *rsa.PrivateKey
	// issuer JWT iss 클레임 및 TokenValidator 기대 issuer
	issuer string
	// audience JWT aud 클레임 및 TokenValidator 기대 audience
	audience string
	// kid JWT 헤더 kid 필드
	kid string
	// authEnabled TokenValidator 활성화 여부
	authEnabled bool
}

// genE2ETestJWT 테스트용 RS256 JWT를 생성하여 반환
//
// claims 파라미터로 sub, scope, exp 등을 지정할 수 있다.
// kid와 iss는 cfg에서 자동 주입된다.
//
// @MX:ANCHOR: [AUTO] E2E 테스트 JWT 생성 단일 진입점
// @MX:REASON: 4개 E2E 테스트 모두 이 함수를 호출 (fan_in >= 4)
func genE2ETestJWT(t *testing.T, cfg *e2eAuthConfig, claims map[string]interface{}) string {
	t.Helper()

	// 기본 시간 클레임 설정 (호출자가 exp를 직접 지정하면 덮어쓴다)
	now := time.Now()
	jwtClaims := jwt.MapClaims{
		"iss": cfg.issuer,
		"aud": cfg.audience,
		"iat": now.Unix(),
		"nbf": now.Add(-10 * time.Second).Unix(),
		"exp": now.Add(3600 * time.Second).Unix(),
	}

	// 호출자 지정 클레임으로 덮어쓰기
	for k, v := range claims {
		jwtClaims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
	token.Header["kid"] = cfg.kid

	signed, err := token.SignedString(cfg.privateKey)
	require.NoError(t, err, "JWT 서명 실패")
	return signed
}

// setupE2EAuthStack testcontainers 위에 Auth 인증 체인이 포함된 전체 스택을 조립
//
// authEnabled=true: RESTMiddleware(validator) 래핑 포함 Mux 반환
// authEnabled=false: RESTMiddleware(nil) → no-op (cli-anonymous 폴백)
//
// @MX:ANCHOR: [AUTO] Auth E2E 스택 초기화 — AC-E2E-1/2/4 및 동시성 테스트에서 사용
// @MX:REASON: 5개 테스트 모두 이 함수를 통해 인프라 연결 (fan_in >= 5)
func setupE2EAuthStack(t *testing.T, cfg *e2eAuthConfig) (*httptest.Server, *redis.Client, func(ctx context.Context, sql string, args ...any) queryRower) {
	t.Helper()

	// testcontainers 인프라 초기화 (e2e_test.go setupE2EContainers 재사용)
	containers := setupE2EContainers(t)
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// PgWorkflowStore 초기화
	pgStore, err := store.NewPgWorkflowStore(ctx, containers.pgDSN, logger)
	require.NoError(t, err, "PgWorkflowStore 초기화 실패")
	t.Cleanup(pgStore.Close)

	// Redis 클라이언트 (Celery dispatch 검증용)
	redisClient := redis.NewClient(&redis.Options{Addr: containers.redisAddr})
	t.Cleanup(func() { _ = redisClient.Close() })

	// CeleryDispatcher + audit.Recorder
	goAdapter := &goRedisAdapter{client: redisClient}
	disp := scheduler.NewCeleryDispatcher(goAdapter, "celery", "test-auth-host")
	_ = disp // dispatch는 이 E2E 범위에서 직접 호출하지 않음

	// Recorder: authEnabled 플래그에 따라 user_id 처리 방식 결정
	recorder := audit.NewRecorder(cfg.authEnabled)
	coordinator := workflow.NewTxCoordinator(pgStore, recorder)
	sm := workflow.NewStateMachine(coordinator, logger)
	svc := server.NewWorkflowService(pgStore, sm, logger)
	handler := server.NewRESTHandler(svc, logger)

	// TokenValidator (authEnabled=true 시 mock JWKS 주입)
	var mux http.Handler
	if cfg.authEnabled {
		jwksMock := &mockJWKSProvider{
			keys: map[string]*rsa.PublicKey{cfg.kid: &cfg.privateKey.PublicKey},
		}
		validator, vErr := auth.New(ctx, cfg.issuer, cfg.audience,
			auth.WithJWKSProvider(jwksMock),
			auth.WithAllowedAlgs([]string{"RS256"}),
		)
		require.NoError(t, vErr, "TokenValidator 초기화 실패")

		// RESTMiddleware로 handler.Mux()를 감싸서 인증 체인 구성
		mux = auth.RESTMiddleware(validator)(handler.Mux())
	} else {
		// AuthEnabled=false: validator=nil → no-op 통과
		mux = auth.RESTMiddleware(nil)(handler.Mux())
	}

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// DB 직접 쿼리 함수 (audit_logs, workflows 검증용)
	queryFn := func(ctx context.Context, sql string, args ...any) queryRower {
		return containers.pgPool.QueryRow(ctx, sql, args...)
	}

	return ts, redisClient, queryFn
}

// queryRower DB 단일 행 조회 인터페이스 (pgx.Row 호환)
type queryRower interface {
	Scan(dest ...any) error
}

// ──────────────────────────────────────────────────────────────────────────────
// E2E 테스트
// ──────────────────────────────────────────────────────────────────────────────

// TestE2E_Auth_FullChainWithValidToken AC-AUTH-E2E-1
//
// AuthEnabled=true 전체 인증 체인 검증:
// POST /api/v1/workflows + valid Bearer → 201 + audit user_id = sub 클레임
//
// Given:
//   - testcontainers Postgres + Redis
//   - 인메모리 RSA 키 쌍, mockJWKSProvider
//   - sub=kepco-analyst-001, scope=iroum-ax:analyst 토큰
//
// When: POST /api/v1/workflows Authorization: Bearer <token>
//
// Then:
//   - HTTP 201 + Location 헤더
//   - workflows 테이블 row 생성
//   - audit_logs.user_id = kepco-analyst-001 (NOT cli-anonymous)
func TestE2E_Auth_FullChainWithValidToken(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	// 인메모리 RSA 키 페어 생성 (2048비트, 결정적이지 않음 — 테스트는 매번 새 키 사용)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "RSA 키 생성 실패")

	cfg := &e2eAuthConfig{
		issuer:      "https://test-keycloak.local/realms/iroum-ax",
		audience:    "iroum-ax-control-plane",
		kid:         "e2e-test-key-v1",
		privateKey:  privateKey,
		authEnabled: true,
	}

	ts, _, queryFn := setupE2EAuthStack(t, cfg)
	ctx := context.Background()

	// sub=kepco-analyst-001 토큰 생성
	const testUserID = "kepco-analyst-001"
	tokenStr := genE2ETestJWT(t, cfg, map[string]interface{}{
		"sub":   testUserID,
		"scope": "iroum-ax:analyst",
	})

	docID := uuid.New().String()
	body, _ := json.Marshal(map[string]string{"document_id": docID})

	// POST /api/v1/workflows with Authorization: Bearer <token>
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/api/v1/workflows", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// HTTP 201 Created 확인
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "AC-AUTH-E2E-1: HTTP 201이어야 함")

	// Location 헤더 확인
	location := resp.Header.Get("Location")
	assert.NotEmpty(t, location, "AC-AUTH-E2E-1: Location 헤더가 있어야 함")

	// 응답 JSON 파싱 → workflow_id 추출
	var wfResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wfResp))
	workflowID, ok := wfResp["workflow_id"].(string)
	require.True(t, ok && workflowID != "", "AC-AUTH-E2E-1: workflow_id 필드가 있어야 함")
	assert.Contains(t, location, workflowID, "Location 헤더에 workflow_id 포함")

	// workflows 테이블 row 확인
	var dbStatus string
	err = queryFn(ctx, "SELECT status FROM workflows WHERE id = $1", workflowID).Scan(&dbStatus)
	require.NoError(t, err, "AC-AUTH-E2E-1: workflows DB row가 있어야 함")
	assert.Equal(t, "PENDING", dbStatus)

	// audit_logs.user_id = kepco-analyst-001 확인 (NOT cli-anonymous)
	var auditUserID string
	err = queryFn(ctx,
		"SELECT user_id FROM audit_logs WHERE resource_id = $1 AND action = 'WORKFLOW_CREATED'",
		workflowID,
	).Scan(&auditUserID)
	require.NoError(t, err, "AC-AUTH-E2E-1: audit_logs WORKFLOW_CREATED row가 있어야 함")
	assert.Equal(t, testUserID, auditUserID,
		"AC-AUTH-E2E-1: audit_logs.user_id = '%s' (JWT sub 클레임 전파)", testUserID)
}

// TestE2E_Auth_AnonymousFallback AC-AUTH-E2E-2
//
// AuthEnabled=false 백워드 호환 확인:
// Authorization 헤더 없이 POST → 201 + audit user_id = cli-anonymous
//
// Given:
//   - testcontainers Postgres + Redis
//   - AuthEnabled=false (validator=nil → RESTMiddleware no-op)
//
// When: POST /api/v1/workflows (Authorization 헤더 없음)
//
// Then:
//   - HTTP 201 + Location 헤더
//   - audit_logs.user_id = cli-anonymous (SPEC-AX-CTRL-001 AC-CTRL-UBI-002-C 동일 결과)
func TestE2E_Auth_AnonymousFallback(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	// AuthEnabled=false: privateKey는 사용되지 않음 (nil 허용하지 않아 더미 생성)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cfg := &e2eAuthConfig{
		issuer:      "https://test-keycloak.local/realms/iroum-ax",
		audience:    "iroum-ax-control-plane",
		kid:         "e2e-test-key-anon",
		privateKey:  privateKey,
		authEnabled: false, // AuthEnabled=false
	}

	ts, _, queryFn := setupE2EAuthStack(t, cfg)
	ctx := context.Background()

	docID := uuid.New().String()
	body, _ := json.Marshal(map[string]string{"document_id": docID})

	// Authorization 헤더 없이 POST
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/api/v1/workflows", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	// Authorization 헤더 의도적으로 제외

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// HTTP 201 Created 확인 (인증 우회)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "AC-AUTH-E2E-2: AuthEnabled=false → HTTP 201이어야 함")

	// Location 헤더 확인
	location := resp.Header.Get("Location")
	assert.NotEmpty(t, location, "AC-AUTH-E2E-2: Location 헤더가 있어야 함")

	// workflow_id 추출
	var wfResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wfResp))
	workflowID, ok := wfResp["workflow_id"].(string)
	require.True(t, ok && workflowID != "", "workflow_id 필드가 있어야 함")

	// audit_logs.user_id = cli-anonymous 확인
	var auditUserID string
	err = queryFn(ctx,
		"SELECT user_id FROM audit_logs WHERE resource_id = $1 AND action = 'WORKFLOW_CREATED'",
		workflowID,
	).Scan(&auditUserID)
	require.NoError(t, err, "AC-AUTH-E2E-2: audit_logs WORKFLOW_CREATED row가 있어야 함")
	assert.Equal(t, "cli-anonymous", auditUserID,
		"AC-AUTH-E2E-2: AuthEnabled=false → audit_logs.user_id = 'cli-anonymous'")
}

// TestE2E_Auth_RBACForbidden AC-AUTH-E2E-3 (SKIP — SPEC-AX-AUTH-002 후속)
//
// 결정: Option B — RBAC 라이브러리(Sprint 5)는 완성, REST 핸들러 연동은 별도 SPEC 범위
//
// TODO(SPEC-AX-AUTH-002): RESTHandler.Mux()에 RBAC Authorize 미들웨어 연동 후 활성화
//
//	구현 위치: rest_handler.go 또는 별도 authz_middleware.go
//	필요 작업:
//	  1. auth.Authorize(ctx, method, path) 를 각 핸들러 진입 시 호출
//	  2. ErrInsufficientPermission → 403 Forbidden + AUTH_FORBIDDEN audit
//	  3. viewer 토큰으로 DELETE → 403 검증
//	이 테스트는 SPEC-AX-AUTH-002 Sprint 1에서 활성화
func TestE2E_Auth_RBACForbidden(t *testing.T) {
	// RBAC-REST 연동은 SPEC-AX-AUTH-002로 이관 (Option B 결정)
	// 이유: Sprint 7 범위는 validator + middleware + audit 체인 검증에 집중
	//       RBAC 핸들러 연동은 독립 SPEC으로 분리하여 scope discipline 유지
	t.Skip("SPEC-AX-AUTH-002: RBAC REST handler wiring deferred — see TODO above")
}

// TestE2E_Auth_InvalidToken_401 AC-AUTH-E2E-4
//
// 잘못된 서명 토큰 → HTTP 401 + workflows 미생성
//
// Given:
//   - AuthEnabled=true
//   - 다른 키로 서명된 토큰 (signature mismatch)
//
// When: POST /api/v1/workflows Authorization: Bearer <badToken>
//
// Then:
//   - HTTP 401
//   - WWW-Authenticate 헤더 존재
//   - workflows 테이블 변화 없음 (미생성)
func TestE2E_Auth_InvalidToken_401(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	// 서버용 키 (서버가 신뢰하는 키)
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// 서명에 사용할 다른 키 (악의적 클라이언트가 가진 키)
	attackerKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cfg := &e2eAuthConfig{
		issuer:      "https://test-keycloak.local/realms/iroum-ax",
		audience:    "iroum-ax-control-plane",
		kid:         "e2e-test-key-invalid",
		privateKey:  serverKey,
		authEnabled: true,
	}

	ts, _, queryFn := setupE2EAuthStack(t, cfg)
	ctx := context.Background()

	// 공격자 키로 서명한 토큰 생성 (kid는 서버 키 kid로 설정 — 서명 불일치)
	badCfg := &e2eAuthConfig{
		issuer:     cfg.issuer,
		audience:   cfg.audience,
		kid:        cfg.kid,     // 서버 kid를 주장
		privateKey: attackerKey, // 실제 서명은 다른 키로
	}
	badToken := genE2ETestJWT(t, badCfg, map[string]interface{}{
		"sub":   "attacker-001",
		"scope": "iroum-ax:analyst",
	})

	// 테스트 전 workflows 개수 확인
	var beforeCount int
	err = queryFn(ctx, "SELECT COUNT(*) FROM workflows").Scan(&beforeCount)
	require.NoError(t, err)

	docID := uuid.New().String()
	body, _ := json.Marshal(map[string]string{"document_id": docID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/api/v1/workflows", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+badToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// HTTP 401 확인
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "AC-AUTH-E2E-4: 잘못된 서명 → 401이어야 함")

	// WWW-Authenticate 헤더 존재 확인
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	assert.NotEmpty(t, wwwAuth, "AC-AUTH-E2E-4: WWW-Authenticate 헤더가 있어야 함")
	assert.Contains(t, wwwAuth, "Bearer", "WWW-Authenticate 헤더에 Bearer 포함")

	// workflows 테이블 변화 없음 (인증 실패 시 비즈니스 로직 미실행)
	var afterCount int
	err = queryFn(ctx, "SELECT COUNT(*) FROM workflows").Scan(&afterCount)
	require.NoError(t, err)
	assert.Equal(t, beforeCount, afterCount,
		"AC-AUTH-E2E-4: 인증 실패 시 workflows 행이 생성되어서는 안 됨")
}

// TestE2E_Auth_ConcurrentRequests 5개 고루틴 동시 요청 — 사용자 격리 확인
//
// Given:
//   - AuthEnabled=true
//   - 5개 서로 다른 sub (user-001 ~ user-005)
//
// When: 5개 goroutine이 동시에 각자의 토큰으로 POST /api/v1/workflows
//
// Then:
//   - 모두 HTTP 201
//   - 각 audit_logs.user_id가 해당 goroutine의 sub와 일치 (격리 보장)
//   - workflow_id 5개 모두 고유
func TestE2E_Auth_ConcurrentRequests(t *testing.T) {
	defer goleak.VerifyNone(t, e2eGoLeakOptions...)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cfg := &e2eAuthConfig{
		issuer:      "https://test-keycloak.local/realms/iroum-ax",
		audience:    "iroum-ax-control-plane",
		kid:         "e2e-concurrent-key",
		privateKey:  privateKey,
		authEnabled: true,
	}

	ts, _, queryFn := setupE2EAuthStack(t, cfg)
	ctx := context.Background()

	const concurrency = 5

	// 결과 저장 구조체
	// fieldalignment: error(interface=16B) 먼저, string(16B) 다음, int(8B) 마지막
	type concResult struct {
		err        error
		userID     string
		workflowID string
		statusCode int
	}

	results := make([]concResult, concurrency)
	var wg sync.WaitGroup

	for i := range concurrency {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			userID := fmt.Sprintf("user-%03d", idx+1)
			tokenStr := genE2ETestJWT(t, cfg, map[string]interface{}{
				"sub":   userID,
				"scope": "iroum-ax:analyst",
			})

			docID := uuid.New().String()
			body, _ := json.Marshal(map[string]string{"document_id": docID})

			req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost,
				ts.URL+"/api/v1/workflows", bytes.NewReader(body))
			if reqErr != nil {
				results[idx] = concResult{userID: userID, err: reqErr}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenStr)

			resp, doErr := http.DefaultClient.Do(req)
			if doErr != nil {
				results[idx] = concResult{userID: userID, err: doErr}
				return
			}
			defer resp.Body.Close()

			var wfResp map[string]interface{}
			if decErr := json.NewDecoder(resp.Body).Decode(&wfResp); decErr != nil {
				results[idx] = concResult{userID: userID, statusCode: resp.StatusCode, err: decErr}
				return
			}

			wfID, _ := wfResp["workflow_id"].(string)
			results[idx] = concResult{
				userID:     userID,
				workflowID: wfID,
				statusCode: resp.StatusCode,
			}
		}(i)
	}
	wg.Wait()

	// 모든 요청 성공 + workflow_id 고유성 확인
	seen := make(map[string]bool)
	for i, r := range results {
		require.NoError(t, r.err, "goroutine %d 에러: userID=%s", i, r.userID)
		assert.Equal(t, http.StatusCreated, r.statusCode,
			"goroutine %d: HTTP 201이어야 함 (userID=%s)", i, r.userID)
		require.NotEmpty(t, r.workflowID,
			"goroutine %d: workflow_id가 비어 있음 (userID=%s)", i, r.userID)
		assert.False(t, seen[r.workflowID], "workflow_id 중복: %s", r.workflowID)
		seen[r.workflowID] = true
	}

	// audit_logs.user_id = 각 goroutine의 sub 클레임과 일치 확인 (격리 검증)
	for _, r := range results {
		if r.workflowID == "" {
			continue // 이미 에러 확인 완료
		}
		var auditUserID string
		scanErr := queryFn(ctx,
			"SELECT user_id FROM audit_logs WHERE resource_id = $1 AND action = 'WORKFLOW_CREATED'",
			r.workflowID,
		).Scan(&auditUserID)
		require.NoError(t, scanErr, "audit_logs 조회 실패: workflowID=%s", r.workflowID)
		assert.Equal(t, r.userID, auditUserID,
			"동시성 격리: audit_logs.user_id = '%s' (goroutine sub 클레임 일치)", r.userID)
	}

	assert.Len(t, seen, concurrency, "고유 workflow_id 수 = %d", concurrency)
}
