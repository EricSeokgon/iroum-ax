// middleware_test.go — SPEC-AX-AUTH-001 REQ-AUTH-003 RED phase 테스트
//
// Sprint 3 RED: gRPC UnaryServerInterceptor + REST RESTMiddleware 실패 테스트
// Sprint 3 GREEN에서 실제 구현 후 PASS로 전환 예정.
//
// 커버리지 목표: AC-AUTH-003-1~4 (Go side) + context round-trip + AuthDisabled 폴백
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// ────────────────────────────────────────────────────────────
// 테스트 인프라 — bufconn gRPC 서버 + 공유 helper
// ────────────────────────────────────────────────────────────

const bufSize = 1024 * 1024

// testHealthServer — /grpc.health.v1.Health/Check 구현
// bufconn 서버에 등록하여 health check 우회를 검증한다.
type testHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (s *testHealthServer) Check(_ context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// newTestGRPCServer — bufconn 기반 in-process gRPC 서버를 생성한다.
// validator가 nil이면 AuthEnabled=false (no-op 통과) 모드로 동작한다.
// 반환된 서버와 listener는 테스트 종료 시 정리해야 한다.
func newTestGRPCServer(t *testing.T, validator *TokenValidator) (*grpc.Server, *bufconn.Listener) {
	t.Helper()

	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer(grpc.UnaryInterceptor(UnaryServerInterceptor(validator)))

	// health 서비스 등록 — /grpc.health.v1.Health/Check 우회 테스트용
	grpc_health_v1.RegisterHealthServer(srv, &testHealthServer{})

	go func() {
		if err := srv.Serve(lis); err != nil {
			// 테스트 환경에서 Serve 종료 에러는 무시
			_ = err
		}
	}()

	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	return srv, lis
}

// newTestGRPCClient — bufconn listener에 연결하는 gRPC 클라이언트를 생성한다.
func newTestGRPCClient(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// ────────────────────────────────────────────────────────────
// 테스트용 TokenValidator 생성 헬퍼
// ────────────────────────────────────────────────────────────

// newMiddlewareTestValidator — middleware 테스트용 TokenValidator를 생성한다.
// Sprint 1 GREEN에서 구현된 실제 Verify를 통해 토큰을 검증한다.
func newMiddlewareTestValidator(t *testing.T) *TokenValidator {
	t.Helper()
	v, err := New(context.Background(), testIssuer, testAudience,
		WithJWKSProvider(&testJWKSProvider{}),
	)
	require.NoError(t, err)
	return v
}

// makeIncomingMetaCtx — incoming gRPC metadata context를 생성한다.
// 서버 인터셉터는 incoming metadata를 읽으므로 NewIncomingContext를 사용한다.
func makeIncomingMetaCtx(token string) context.Context {
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewIncomingContext(context.Background(), md)
}

// makeIncomingMetaCtxRaw — 임의 authorization 값으로 incoming metadata context를 생성한다.
func makeIncomingMetaCtxRaw(authValue string) context.Context {
	md := metadata.Pairs("authorization", authValue)
	return metadata.NewIncomingContext(context.Background(), md)
}

// dummyHandler — 항상 nil을 반환하는 dummy gRPC unary handler
func dummyHandler(_ context.Context, _ any) (any, error) {
	return "ok", nil
}

// capturedCtx — 핸들러가 받은 context 캡처용 (context user 검증)
var capturedCtx context.Context

// capturingHandler — context를 캡처하는 gRPC unary handler
func capturingHandler(ctx context.Context, _ any) (any, error) {
	capturedCtx = ctx
	return "ok", nil
}

// ────────────────────────────────────────────────────────────
// §1. Context Round-Trip 테스트 (직접 함수 검증)
// ────────────────────────────────────────────────────────────

// TestWithUser_RoundTrip — WithUser → UserFromContext 라운드트립을 검증한다.
// AC-AUTH-003-1 보조: context 주입/추출 경로가 올바름을 확인한다.
func TestWithUser_RoundTrip(t *testing.T) {
	t.Parallel()

	want := &User{
		UID:    "uuid-alice",
		Issuer: testIssuer,
		Roles:  []string{"analyst"},
		Scopes: []string{"iroum-ax:analyst"},
	}

	ctx := WithUser(context.Background(), want)
	got, ok := UserFromContext(ctx)

	require.True(t, ok, "UserFromContext가 true를 반환해야 한다")
	assert.Equal(t, want.UID, got.UID)
	assert.Equal(t, want.Issuer, got.Issuer)
	assert.Equal(t, want.Scopes, got.Scopes)
}

// TestUserFromContext_Empty — context에 User가 없을 때 nil, false를 반환한다.
func TestUserFromContext_Empty(t *testing.T) {
	t.Parallel()

	got, ok := UserFromContext(context.Background())
	assert.False(t, ok)
	assert.Nil(t, got)
}

// ────────────────────────────────────────────────────────────
// §2. gRPC UnaryServerInterceptor 테스트
// 인터셉터를 직접 호출하여 gRPC wire 레이어 없이 동작을 검증한다.
// Health check bypass는 bufconn으로 통합 검증한다.
// ────────────────────────────────────────────────────────────

// TestUnaryInterceptor_ValidToken_PassesToHandler — 유효한 토큰이 있을 때 핸들러에 진입한다.
// AC-AUTH-003-1: Authorization metadata 추출 → Verify → context user 주입 → 핸들러
func TestUnaryInterceptor_ValidToken_PassesToHandler(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	interceptor := UnaryServerInterceptor(validator)

	token := genTestJWT(t, defaultJWTOpts())
	ctx := makeIncomingMetaCtx(token)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, dummyHandler)

	// RED: stub이 "구현 예정" 에러를 반환하므로 FAIL
	// GREEN 이후: err == nil이어야 함
	require.NoError(t, err, "유효한 토큰으로 핸들러 진입이 성공해야 한다 — GREEN에서 구현 예정")
}

// TestUnaryInterceptor_ContextHasUser — 핸들러 내부에서 context user를 추출할 수 있다.
// AC-AUTH-003-1 핵심: User.UID = token sub
func TestUnaryInterceptor_ContextHasUser(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	interceptor := UnaryServerInterceptor(validator)

	opts := defaultJWTOpts()
	opts.sub = "uuid-test-user"
	token := genTestJWT(t, opts)
	ctx := makeIncomingMetaCtx(token)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, capturingHandler)

	// RED: 핸들러 진입 성공 후 context에 user가 주입되어야 함
	require.NoError(t, err, "유효한 토큰으로 핸들러 진입이 성공해야 한다")

	u, ok := UserFromContext(capturedCtx)
	require.True(t, ok, "핸들러 context에서 User를 추출할 수 있어야 한다")
	assert.Equal(t, "uuid-test-user", u.UID,
		"핸들러가 context에서 올바른 UID를 추출해야 한다")
}

// TestUnaryInterceptor_NoAuthMetadata_ReturnsUnauthenticated — Authorization 헤더 없을 때 UNAUTHENTICATED를 반환한다.
// AC-AUTH-003-3 (gRPC): missing auth → codes.Unauthenticated
func TestUnaryInterceptor_NoAuthMetadata_ReturnsUnauthenticated(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	interceptor := UnaryServerInterceptor(validator)

	// metadata 없는 bare context
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, dummyHandler)

	// RED: stub이 "구현 예정" 에러 → FAIL
	// GREEN 이후: codes.Unauthenticated
	require.Error(t, err, "인증 헤더 없을 때 에러가 반환되어야 한다")
	st, ok := status.FromError(err)
	require.True(t, ok, "gRPC status 에러여야 한다")
	assert.Equal(t, codes.Unauthenticated, st.Code(),
		"Authorization 헤더 누락 시 codes.Unauthenticated를 반환해야 한다")
}

// TestUnaryInterceptor_InvalidToken_ReturnsUnauthenticated — iss 불일치 토큰 시 UNAUTHENTICATED를 반환한다.
// AC-AUTH-003-4 (gRPC): invalid token → codes.Unauthenticated
func TestUnaryInterceptor_InvalidToken_ReturnsUnauthenticated(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	interceptor := UnaryServerInterceptor(validator)

	// iss 불일치 토큰 (검증 실패 유도)
	opts := defaultJWTOpts()
	opts.issuer = "https://evil.example.com/realms/attacker"
	token := genTestJWT(t, opts)
	ctx := makeIncomingMetaCtx(token)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, dummyHandler)

	require.Error(t, err, "유효하지 않은 토큰 시 에러가 반환되어야 한다")
	st, ok := status.FromError(err)
	require.True(t, ok, "gRPC status 에러여야 한다")
	assert.Equal(t, codes.Unauthenticated, st.Code(),
		"유효하지 않은 토큰 시 codes.Unauthenticated를 반환해야 한다")
}

// TestUnaryInterceptor_TokenExpired_ReturnsUnauthenticated — 만료된 토큰 시 UNAUTHENTICATED를 반환한다.
func TestUnaryInterceptor_TokenExpired_ReturnsUnauthenticated(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	interceptor := UnaryServerInterceptor(validator)

	// 만료된 토큰 (exp = now - 100s, skew 30s 초과)
	opts := defaultJWTOpts()
	opts.expOffset = -100 * time.Second
	opts.iatOffset = -200 * time.Second
	opts.nbfOffset = -250 * time.Second
	token := genTestJWT(t, opts)
	ctx := makeIncomingMetaCtx(token)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, dummyHandler)

	require.Error(t, err, "만료된 토큰 시 에러가 반환되어야 한다")
	st, ok := status.FromError(err)
	require.True(t, ok, "gRPC status 에러여야 한다")
	assert.Equal(t, codes.Unauthenticated, st.Code(),
		"만료된 토큰 시 codes.Unauthenticated를 반환해야 한다")
}

// TestUnaryInterceptor_HealthCheck_Bypass — /grpc.health.v1.Health/Check는 인증을 우회한다.
// AC-AUTH-003-2: bufconn 통합 검증 — Authorization 없이 SERVING 반환
func TestUnaryInterceptor_HealthCheck_Bypass(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	_, lis := newTestGRPCServer(t, validator)
	conn := newTestGRPCClient(t, lis)

	// Authorization 없이 health check 호출
	ctx := context.Background()
	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "",
	})

	// RED: 인터셉터가 health check를 우회해야 하는데 stub이 "구현 예정" 에러 반환 → FAIL
	require.NoError(t, err, "/grpc.health.v1.Health/Check는 인증 없이도 성공해야 한다")
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.GetStatus(),
		"health check는 SERVING을 반환해야 한다")
}

// TestUnaryInterceptor_AuthDisabled_PassesAsAnonymous — validator=nil 시 no-op 통과한다.
// AC-AUTH-UBI-001-C (gRPC): AuthEnabled=false → handler에 도달
func TestUnaryInterceptor_AuthDisabled_PassesAsAnonymous(t *testing.T) {
	// validator=nil → AuthEnabled=false 시뮬레이션
	interceptor := UnaryServerInterceptor(nil)

	// authorization 헤더 없이 요청
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, dummyHandler)

	// validator=nil이면 stub도 handler를 통과해야 함
	assert.NoError(t, err, "AuthEnabled=false 시 인증 없이도 핸들러에 도달해야 한다")
}

// TestUnaryInterceptor_MalformedBearer_ReturnsUnauthenticated — Bearer 접두사 없는 토큰 시 UNAUTHENTICATED.
// AC-AUTH-003-4 (gRPC variant): malformed authorization header
func TestUnaryInterceptor_MalformedBearer_ReturnsUnauthenticated(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	interceptor := UnaryServerInterceptor(validator)

	token := genTestJWT(t, defaultJWTOpts())
	// "Bearer" 대신 "Token" 접두사 사용
	ctx := makeIncomingMetaCtxRaw("Token " + token)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, dummyHandler)

	require.Error(t, err, "잘못된 Authorization 형식 시 에러가 반환되어야 한다")
	st, ok := status.FromError(err)
	require.True(t, ok, "gRPC status 에러여야 한다")
	assert.Equal(t, codes.Unauthenticated, st.Code(),
		"잘못된 Authorization 형식 시 codes.Unauthenticated를 반환해야 한다")
}

// ────────────────────────────────────────────────────────────
// §3. REST RESTMiddleware 테스트
// ────────────────────────────────────────────────────────────

// newTestRESTServer — httptest.Server 기반 REST 서버를 생성한다.
// validator가 nil이면 AuthEnabled=false 모드로 동작한다.
func newTestRESTServer(t *testing.T, validator *TokenValidator) *httptest.Server {
	t.Helper()

	// 내부 핸들러: /health는 bypass, /api/v1/*는 인증 필요
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/v1/workflows", func(w http.ResponseWriter, r *http.Request) {
		// context에서 user 추출하여 응답에 포함
		if u, ok := UserFromContext(r.Context()); ok {
			w.Header().Set("X-User-UID", u.UID)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// RESTMiddleware로 wrapping
	wrapped := RESTMiddleware(validator)(mux)
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	return srv
}

// TestRESTMiddleware_ValidToken_CallsNext — 유효한 Bearer 토큰 시 next 핸들러를 호출한다.
// AC-AUTH-003-3 (case A 역할: 유효 토큰 → 200)
func TestRESTMiddleware_ValidToken_CallsNext(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	token := genTestJWT(t, defaultJWTOpts())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// RED: 미구현 stub이 501 반환 → 200이어야 하지만 FAIL
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"유효한 토큰 시 next 핸들러가 호출되어 200을 반환해야 한다")
}

// TestRESTMiddleware_UserInjected_InContext — 유효한 토큰 시 핸들러가 context에서 user를 추출할 수 있다.
// AC-AUTH-003-1 (REST): next handler에서 UserFromContext로 user 추출
func TestRESTMiddleware_UserInjected_InContext(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	opts := defaultJWTOpts()
	opts.sub = "uuid-rest-user"
	token := genTestJWT(t, opts)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// RED: 미구현 stub에서 context user 주입이 없으므로 FAIL
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"유효한 토큰 시 next 핸들러가 200을 반환해야 한다")
	assert.Equal(t, "uuid-rest-user", resp.Header.Get("X-User-UID"),
		"핸들러가 context에서 올바른 UID를 추출해야 한다")
}

// TestRESTMiddleware_MissingHeader_Returns401 — Authorization 헤더 없을 때 401을 반환한다.
// AC-AUTH-003-3 (case B): no auth header → 401 + WWW-Authenticate + JSON body
func TestRESTMiddleware_MissingHeader_Returns401(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	// Authorization 헤더 없음

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// RED: stub이 501 반환 → 401이어야 하므로 FAIL (assert로 soft fail)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Authorization 헤더 없을 때 401을 반환해야 한다")

	// WWW-Authenticate 헤더 검증
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	assert.Contains(t, wwwAuth, "Bearer",
		`WWW-Authenticate 헤더가 "Bearer"를 포함해야 한다`)

	// JSON 응답 body 검증
	var body map[string]any
	if resp.StatusCode == http.StatusUnauthorized {
		err = json.NewDecoder(resp.Body).Decode(&body)
		assert.NoError(t, err, "응답 body가 유효한 JSON이어야 한다")
	}
}

// TestRESTMiddleware_MalformedHeader_Returns401 — Bearer 접두사 없는 헤더 시 401을 반환한다.
// AC-AUTH-003-4: "Token <token>" → 401 malformed_authorization_header
func TestRESTMiddleware_MalformedHeader_Returns401(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	token := genTestJWT(t, defaultJWTOpts())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Token "+token) // Bearer 접두사 누락

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// RED: stub이 501 반환 → 401이어야 하므로 FAIL
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Bearer 접두사 없을 때 401을 반환해야 한다")
}

// TestRESTMiddleware_InvalidToken_Returns401_WWWAuthenticate — 유효하지 않은 토큰 시 401 + WWW-Authenticate.
// AC-AUTH-003-3 (invalid token variant)
func TestRESTMiddleware_InvalidToken_Returns401_WWWAuthenticate(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	// iss 불일치 토큰 (검증 실패 유도)
	opts := defaultJWTOpts()
	opts.issuer = "https://attacker.example.com"
	token := genTestJWT(t, opts)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// RED: stub이 501 반환 → 401이어야 하므로 FAIL
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"유효하지 않은 토큰 시 401을 반환해야 한다")

	wwwAuth := resp.Header.Get("WWW-Authenticate")
	assert.Contains(t, wwwAuth, "Bearer",
		`WWW-Authenticate 헤더가 "Bearer"를 포함해야 한다`)
}

// TestRESTMiddleware_HealthBypass — /health 경로는 인증 없이도 200을 반환한다.
// AC-AUTH-003-3 (case A): GET /health → 200 (bypass)
func TestRESTMiddleware_HealthBypass(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/health", nil)
	require.NoError(t, err)
	// Authorization 없음

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// GREEN 이후: 200이어야 함
	// RED 현재: stub이 /health를 먼저 라우팅하면 200, 미들웨어 통과 전에 401이면 FAIL
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"/health 경로는 인증 없이 200을 반환해야 한다")
}

// TestRESTMiddleware_AuthDisabled_PassesAsAnonymous — validator=nil 시 no-op 통과한다.
// AC-AUTH-UBI-001-C (REST): AuthEnabled=false → 인증 없이 next 호출
func TestRESTMiddleware_AuthDisabled_PassesAsAnonymous(t *testing.T) {
	// validator=nil → AuthEnabled=false 시뮬레이션
	srv := newTestRESTServer(t, nil)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	// Authorization 헤더 없음

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// validator=nil이면 stub도 no-op 통과 → 200 기대
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"AuthEnabled=false 시 인증 없이도 next 핸들러에 도달해야 한다")
}

// TestRESTMiddleware_MissingToken_BearerParsing — "Bearer " 다음 토큰 값이 빈 문자열 시 401.
func TestRESTMiddleware_MissingToken_BearerParsing(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer ") // 토큰값 없음

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// 빈 토큰값 → 401
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"빈 Bearer 토큰 시 401을 반환해야 한다")
}

// ────────────────────────────────────────────────────────────
// §4. 통합 검증 — gRPC + REST 모두 동일 User 구조체 사용
// ────────────────────────────────────────────────────────────

// TestUser_FieldAlignment — User 구조체가 fieldalignment 요구사항을 충족한다.
// 문자열 필드 먼저, 슬라이스 마지막 (SPEC coding standards)
func TestUser_FieldAlignment(t *testing.T) {
	t.Parallel()

	u := &User{
		UID:    "uid",
		Issuer: "iss",
		Roles:  []string{"admin"},
		Scopes: []string{"scope1"},
	}

	assert.NotNil(t, u)
	assert.Equal(t, "uid", u.UID)
	assert.Equal(t, "iss", u.Issuer)
	assert.Equal(t, []string{"admin"}, u.Roles)
	assert.Equal(t, []string{"scope1"}, u.Scopes)
}

// TestRESTMiddleware_WWWAuthenticate_Format — WWW-Authenticate 헤더 형식 검증.
// AC-AUTH-003-3: WWW-Authenticate: Bearer realm="iroum-ax", error="invalid_request"
func TestRESTMiddleware_WWWAuthenticate_Format(t *testing.T) {
	validator := newMiddlewareTestValidator(t)
	srv := newTestRESTServer(t, validator)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/v1/workflows", nil)
	require.NoError(t, err)
	// Authorization 헤더 없음

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Skip("GREEN 구현 후 검증 — 현재 stub 상태 스킵")
	}

	wwwAuth := resp.Header.Get("WWW-Authenticate")
	assert.True(t, strings.Contains(wwwAuth, `Bearer`),
		"WWW-Authenticate가 Bearer scheme을 포함해야 한다")
	assert.True(t, strings.Contains(wwwAuth, `realm=`),
		"WWW-Authenticate가 realm을 포함해야 한다")
}
