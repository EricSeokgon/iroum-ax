// authz_middleware_test.go — Sprint 1 RED: REST 인가 미들웨어 단위 테스트
// REQ-AUTH2-002-E1/U1/U2, REQ-AUTH2-UBI-001-b/c
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// captureTx — 테스트용 AuditTx 구현: 삽입된 이벤트를 슬라이스에 캡처
type captureTx struct {
	events []*audit.Event
	failAt int // n번째 삽입에서 실패 (-1이면 항상 성공)
	count  int
}

func newCaptureTx() *captureTx { return &captureTx{failAt: -1} }

func (c *captureTx) InsertAuditLog(_ context.Context, e *audit.Event) error {
	c.count++
	if c.failAt >= 0 && c.count == c.failAt {
		return audit.ErrAuditCaptureFail
	}
	c.events = append(c.events, e)
	return nil
}

// captureRecorder — auditRecorder 인터페이스 구현 (captureTx를 내부적으로 사용)
type captureRecorder struct {
	tx *captureTx
}

func newCaptureRecorder() *captureRecorder {
	return &captureRecorder{tx: newCaptureTx()}
}

func (cr *captureRecorder) LogForbiddenEvent(ctx context.Context, method, path, required, userID string, grantedRoles []Role) error {
	// required 필드를 포함하여 직접 audit.Event를 생성하고 captureTx에 삽입
	rolesStr := make([]string, len(grantedRoles))
	for i, r := range grantedRoles {
		rolesStr[i] = string(r)
	}
	detailsRaw, err := json.Marshal(map[string]any{
		"method":        method,
		"path":          path,
		"required":      required,
		"granted_roles": rolesStr,
	})
	if err != nil {
		return err
	}
	e := &audit.Event{
		Timestamp:   time.Now().UTC(),
		Action:      audit.ActionAuthForbidden,
		UserID:      userID,
		DetailsJSON: detailsRaw,
	}
	return cr.tx.InsertAuditLog(ctx, e)
}

// makeUserCtx — 지정된 scope를 가진 User를 context에 주입한 요청 생성
func makeUserCtx(method, path, scope, uid string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	u := &User{
		UID:    uid,
		Scopes: []string{scope},
	}
	return req.WithContext(WithUser(req.Context(), u))
}

// makeAdminCtx — admin scope User 컨텍스트 요청
func makeAdminCtx(method, path string) *http.Request {
	return makeUserCtx(method, path, "iroum-ax:admin", "uid-admin-001")
}

// makeAnalystCtx — analyst scope User 컨텍스트 요청
func makeAnalystCtx(method, path string) *http.Request {
	return makeUserCtx(method, path, "iroum-ax:analyst", "uid-analyst-001")
}

// makeViewerCtx — viewer scope User 컨텍스트 요청
func makeViewerCtx(method, path string) *http.Request {
	return makeUserCtx(method, path, "iroum-ax:viewer", "uid-viewer-001")
}

// okHandler — 항상 200 OK를 반환하는 테스트용 핸들러
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// TestRESTAuthz_AdminAccess_AllPaths — admin은 모든 매핑 경로를 통과
func TestRESTAuthz_AdminAccess_AllPaths(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	paths := []struct{ method, path string }{
		{"POST", "/api/v1/workflows"},
		{"GET", "/api/v1/workflows"},
		{"GET", "/api/v1/workflows/some-id"},
		{"DELETE", "/api/v1/workflows/some-id"},
		{"POST", "/api/v1/recommendations/rec-id/feedback"},
		{"POST", "/api/v1/documents/upload"},
	}

	for _, p := range paths {
		p := p
		t.Run(p.method+" "+p.path, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			mw.ServeHTTP(rr, makeAdminCtx(p.method, p.path))
			assert.Equal(t, http.StatusOK, rr.Code, "admin은 모든 경로 통과해야 함")
		})
	}

	// admin 요청에서 AUTH_FORBIDDEN 이벤트 없음
	assert.Empty(t, recorder.tx.events, "admin 접근 시 AUTH_FORBIDDEN 이벤트 없어야 함")
}

// TestRESTAuthz_AnalystAccess_Workflow — analyst는 write:workflow 경로 통과
func TestRESTAuthz_AnalystAccess_Workflow(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	req := makeAnalystCtx("POST", "/api/v1/workflows")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "analyst는 write:workflow 경로 통과해야 함")
	assert.Empty(t, recorder.tx.events)
}

// TestRESTAuthz_ViewerWrite_Returns403_AuditForbidden — viewer가 write 경로 시도 시 403 + 감사 기록
// REQ-AUTH2-002-U1
func TestRESTAuthz_ViewerWrite_Returns403_AuditForbidden(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	req := makeViewerCtx("POST", "/api/v1/workflows")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "viewer write 시도 → 403")

	// 응답 바디 검증
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok, "응답 body에 error 객체 있어야 함")
	assert.Equal(t, "PERMISSION_DENIED", errObj["code"])

	// WWW-Authenticate 헤더 검증
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	assert.Contains(t, wwwAuth, "insufficient_scope")

	// AUTH_FORBIDDEN 감사 이벤트 1건 기록
	require.Len(t, recorder.tx.events, 1)
	assert.Equal(t, audit.ActionAuthForbidden, recorder.tx.events[0].Action)
	assert.Equal(t, "uid-viewer-001", recorder.tx.events[0].UserID)
}

// TestRESTAuthz_MappingMissing_Returns503 — 매핑 미정의 경로 → 503 (default-deny)
// REQ-AUTH2-001-U1
func TestRESTAuthz_MappingMissing_Returns503(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	// 매핑에 없는 경로 (audit recorder가 있는 admin 사용자)
	req := makeAdminCtx("GET", "/api/v1/unknown-endpoint")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "매핑 미정의 → 503")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AUTHZ_MAPPING_MISSING", errObj["code"])
}

// TestRESTAuthz_AuthDisabled_PassesThrough — AuthEnabled=false 시 인가 미들웨어 pass-through
// REQ-AUTH2-UBI-001-b
func TestRESTAuthz_AuthDisabled_PassesThrough(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	// authEnabled=false: 모든 요청 통과
	mw := RESTAuthzMiddleware(recorder, false)(okHandler)

	// Authorization 헤더 없이 (anonymous 요청)
	req := httptest.NewRequest("POST", "/api/v1/workflows", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "authEnabled=false → 통과")
	assert.Empty(t, recorder.tx.events, "AUTH_FORBIDDEN 이벤트 없어야 함")
}

// TestRESTAuthz_HealthBypass — /health 경로는 bypass (인가 체크 없이 통과)
// REQ-AUTH2-001-E1 bypass 규칙
func TestRESTAuthz_HealthBypass(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	// User 없이도 /health는 통과 (bypass)
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "/health는 bypass")
	assert.Empty(t, recorder.tx.events)
}

// TestRESTAuthz_PathParam_Resolved — 경로 파라미터 있는 URL 매핑 정상 처리
// REQ-AUTH2-001-E1 path parameter matching
func TestRESTAuthz_PathParam_Resolved(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	// viewer는 read:workflow 보유 → GET workflows/{id} 통과
	req := makeViewerCtx("GET", "/api/v1/workflows/550e8400-e29b-41d4-a716-446655440000")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "viewer GET workflow/{id} → 통과")
	assert.Empty(t, recorder.tx.events)
}

// TestRESTAuthz_AuditForbidden_RecordsRow — AUTH_FORBIDDEN 감사 이벤트 상세 필드 검증
// REQ-AUTH2-UBI-001-a
func TestRESTAuthz_AuditForbidden_RecordsRow(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	req := makeViewerCtx("DELETE", "/api/v1/workflows/wf-001")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	require.Equal(t, http.StatusForbidden, rr.Code)
	require.Len(t, recorder.tx.events, 1)

	event := recorder.tx.events[0]
	assert.Equal(t, audit.ActionAuthForbidden, event.Action)
	assert.Equal(t, "uid-viewer-001", event.UserID)

	// details JSON 파싱 검증
	var details map[string]any
	require.NoError(t, json.Unmarshal(event.DetailsJSON, &details))
	assert.Equal(t, "DELETE", details["method"])
	// path는 실제 경로 포함
	assert.NotEmpty(t, details["path"])
	assert.Equal(t, "delete:workflow", details["required"])
	// granted_roles에 viewer 포함
	roles, ok := details["granted_roles"].([]any)
	require.True(t, ok)
	require.Len(t, roles, 1)
	assert.Equal(t, "viewer", roles[0])
}

// TestRESTAuthz_NoUserInContext_Returns500 — User context 없는 요청 → 500
// REQ-AUTH2-002-U2: AuthEnabled=true인데 user context 없으면 미들웨어 wiring 버그
func TestRESTAuthz_NoUserInContext_Returns500(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	// User를 context에 주입하지 않은 요청 (인증 미들웨어가 주입했어야 함)
	req := httptest.NewRequest("POST", "/api/v1/workflows", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code, "user context 없으면 500")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AUTHZ_USER_MISSING", errObj["code"])
}

// TestRESTAuthz_LogsMethodAndPath — 감사 이벤트에 method/path 기록 검증
func TestRESTAuthz_LogsMethodAndPath(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	mw := RESTAuthzMiddleware(recorder, true)(okHandler)

	req := makeViewerCtx("POST", "/api/v1/workflows")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	require.Equal(t, http.StatusForbidden, rr.Code)
	require.Len(t, recorder.tx.events, 1)

	var details map[string]any
	require.NoError(t, json.Unmarshal(recorder.tx.events[0].DetailsJSON, &details))
	assert.Equal(t, "POST", details["method"])
	assert.Equal(t, "/api/v1/workflows", details["path"])
}

// TestGrantedPermissionFromContext — 성공 시 context에 granted_permission 주입 검증
// REQ-AUTH2-002-E2
func TestGrantedPermissionFromContext(t *testing.T) {
	t.Parallel()

	// context에 주입 전: 없음
	ctx := context.Background()
	_, ok := GrantedPermissionFromContext(ctx)
	assert.False(t, ok, "주입 전에는 found=false")

	// context에 주입 후: 있음
	ctx2 := WithGrantedPermission(ctx, "read:workflow")
	perm, ok2 := GrantedPermissionFromContext(ctx2)
	assert.True(t, ok2, "주입 후에는 found=true")
	assert.Equal(t, "read:workflow", perm)
}

// TestRESTAuthz_SuccessAnnotatesContext — 성공 시 context에 granted_permission 주입 확인
// REQ-AUTH2-002-E2
func TestRESTAuthz_SuccessAnnotatesContext(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()

	var capturedCtx context.Context
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	mw := RESTAuthzMiddleware(recorder, true)(captureHandler)

	req := makeAdminCtx("GET", "/api/v1/workflows")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	perm, ok := GrantedPermissionFromContext(capturedCtx)
	assert.True(t, ok, "성공 시 context에 granted_permission 주입되어야 함")
	assert.Equal(t, "read:workflow", perm)
}

// TestUnaryAuthzInterceptor_AuthDisabled_PassesThrough — gRPC authEnabled=false 시 pass-through
func TestUnaryAuthzInterceptor_AuthDisabled_PassesThrough(t *testing.T) {
	t.Parallel()

	interceptor := UnaryAuthzInterceptor(nil, false)
	ctx := context.Background()
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/iroum.ax.v1.WorkflowService/CreateWorkflow"}

	resp, err := interceptor(ctx, nil, info, handler)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, called, "authEnabled=false → handler 바로 호출")
}

// TestUnaryAuthzInterceptor_HealthBypass — gRPC health check는 bypass
func TestUnaryAuthzInterceptor_HealthBypass(t *testing.T) {
	t.Parallel()

	interceptor := UnaryAuthzInterceptor(nil, true)
	ctx := context.Background()
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}

	resp, err := interceptor(ctx, nil, info, handler)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, called, "health check → bypass")
}

// TestUnaryAuthzInterceptor_MappingMissing_ReturnsUnavailable — gRPC 매핑 미정의 → Unavailable
// REQ-AUTH2-001-U1
func TestUnaryAuthzInterceptor_MappingMissing_ReturnsUnavailable(t *testing.T) {
	t.Parallel()

	interceptor := UnaryAuthzInterceptor(nil, true)
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/unknown.Service/Unknown"}

	_, err := interceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code(), "매핑 미정의 → codes.Unavailable")
}

// TestUnaryAuthzInterceptor_PermissionDenied — gRPC 권한 부족 → PermissionDenied + 감사
// REQ-AUTH2-003-U1
func TestUnaryAuthzInterceptor_PermissionDenied(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	interceptor := UnaryAuthzInterceptor(recorder, true)

	// viewer는 delete:workflow 권한 없음
	u := &User{UID: "uid-viewer-001", Scopes: []string{"iroum-ax:viewer"}}
	ctx := WithUser(context.Background(), u)
	info := &grpc.UnaryServerInfo{FullMethod: "/iroum.ax.v1.WorkflowService/CreateWorkflow"}

	_, err := interceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code(), "권한 부족 → PermissionDenied")

	// 감사 이벤트 기록 확인
	require.Len(t, recorder.tx.events, 1)
	assert.Equal(t, audit.ActionAuthForbidden, recorder.tx.events[0].Action)
	assert.Equal(t, "uid-viewer-001", recorder.tx.events[0].UserID)
}

// TestUnaryAuthzInterceptor_AdminAccess — gRPC admin 접근 → handler 정상 호출
func TestUnaryAuthzInterceptor_AdminAccess(t *testing.T) {
	t.Parallel()

	recorder := newCaptureRecorder()
	interceptor := UnaryAuthzInterceptor(recorder, true)

	u := &User{UID: "uid-admin-001", Scopes: []string{"iroum-ax:admin"}}
	ctx := WithUser(context.Background(), u)
	info := &grpc.UnaryServerInfo{FullMethod: "/iroum.ax.v1.WorkflowService/CreateWorkflow"}
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "created", nil
	}

	resp, err := interceptor(ctx, nil, info, handler)
	assert.NoError(t, err)
	assert.Equal(t, "created", resp)
	assert.True(t, called)
	assert.Empty(t, recorder.tx.events, "admin 성공 시 AUTH_FORBIDDEN 이벤트 없음")
}
