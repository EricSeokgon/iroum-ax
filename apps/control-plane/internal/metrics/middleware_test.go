// middleware_test.go — MetricsAuthMiddleware + MetricsHandler 테스트
// SPEC-AX-OBS-001 Sprint 1 RED
package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
)

// TestValidatedTokenToMetricsUser_RolesFromRoles — roles 클레임에서 역할을 추출해야 한다.
// middleware.go 내부 함수 validatedTokenToMetricsUser 검증 (lesson #4: 실제 동작 assert).
func TestValidatedTokenToMetricsUser_RolesFromRoles(t *testing.T) {
	t.Parallel()
	vt := &auth.ValidatedToken{
		Subject: "sub-123",
		Issuer:  "https://example.com",
		Scopes:  []string{"iroum-ax:admin"},
		Claims: map[string]any{
			"roles": []any{"admin", "analyst"},
		},
	}
	u := validatedTokenToMetricsUser(vt)
	assert.Equal(t, "sub-123", u.UID)
	assert.Equal(t, "https://example.com", u.Issuer)
	assert.Equal(t, []string{"admin", "analyst"}, u.Roles)
}

// TestValidatedTokenToMetricsUser_RolesFromRealmAccess — realm_access.roles에서 역할을 추출해야 한다.
func TestValidatedTokenToMetricsUser_RolesFromRealmAccess(t *testing.T) {
	t.Parallel()
	vt := &auth.ValidatedToken{
		Subject: "sub-456",
		Claims: map[string]any{
			"realm_access": map[string]any{
				"roles": []any{"viewer"},
			},
		},
	}
	u := validatedTokenToMetricsUser(vt)
	assert.Equal(t, []string{"viewer"}, u.Roles)
}

// TestMetricsHandler_Returns200_WithRegistry MetricsHandler()가 전역 레지스트리로 200을 반환해야 한다.
func TestMetricsHandler_Returns200_WithRegistry(t *testing.T) {
	t.Parallel()
	h := MetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "MetricsHandler()는 200을 반환해야 한다")
}

// testValidator — 테스트용 TokenValidator 생성 헬퍼
func testValidator(t *testing.T) *auth.TokenValidator {
	t.Helper()
	v, err := auth.New(context.Background(), "https://example.com", "test-aud",
		auth.WithJWKSProvider(&testJWKSProvider{}),
	)
	require.NoError(t, err)
	return v
}

// testJWKSProvider — 테스트용 JWKSProvider (항상 에러 반환 → 토큰 검증 실패)
type testJWKSProvider struct{}

func (t *testJWKSProvider) GetKey(_ context.Context, _ string) (any, string, string, error) {
	return nil, "", "", auth.ErrJWKSUnavailable
}

// dummyHandler — 테스트용 downstream handler
var dummyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// TestMetricsAuthMiddleware_NoToken_Returns401 — Bearer 토큰 없으면 401을 반환해야 한다.
func TestMetricsAuthMiddleware_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	mw := MetricsAuthMiddleware(v, true)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "토큰 없으면 401이어야 한다")
}

// TestMetricsAuthMiddleware_InvalidToken_Returns401 — 잘못된 토큰이면 401을 반환해야 한다.
func TestMetricsAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	mw := MetricsAuthMiddleware(v, true)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer bad.invalid.token")
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "잘못된 토큰이면 401이어야 한다")
}

// TestMetricsAuthMiddleware_ViewerToken_Returns403 — viewer 역할 사용자는 403을 받아야 한다.
func TestMetricsAuthMiddleware_ViewerToken_Returns403(t *testing.T) {
	t.Parallel()
	// viewer user를 context에 직접 주입하는 방식으로 authz만 테스트
	mw := MetricsAuthMiddlewareWithUserInjector(func(r *http.Request) (*auth.User, bool) {
		return &auth.User{UID: "viewer", Scopes: []string{"iroum-ax:viewer"}}, true
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "viewer는 403을 받아야 한다")
}

// TestMetricsAuthMiddleware_AdminToken_Returns200 — admin 역할 사용자는 downstream에 도달해야 한다.
func TestMetricsAuthMiddleware_AdminToken_Returns200(t *testing.T) {
	t.Parallel()
	mw := MetricsAuthMiddlewareWithUserInjector(func(r *http.Request) (*auth.User, bool) {
		return &auth.User{UID: "admin", Scopes: []string{"iroum-ax:admin"}}, true
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "admin은 200을 받아야 한다")
}

// TestMetricsHandler_Returns200 — MetricsHandler가 200을 반환하고 text/plain content-type을 포함해야 한다.
func TestMetricsHandler_Returns200(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	h := MetricsHandlerForRegistry(reg)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "MetricsHandler가 200을 반환해야 한다")
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain",
		"Content-Type이 text/plain이어야 한다")
}

// TestInstrumentationMiddleware_RecordsHistogram — HTTP instrumentation middleware가 히스토그램에 기록해야 한다.
func TestInstrumentationMiddleware_RecordsHistogram(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	mw := HTTPInstrumentationMiddleware(m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_http_request_duration_seconds" {
			for _, metric := range mf.GetMetric() {
				if metric.GetHistogram().GetSampleCount() >= 1 {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "instrumentation middleware가 히스토그램에 관측값을 기록해야 한다")
}

// TestMetricsAuthMiddleware_NoToken_NestedErrorBody — 401 응답 body가 nested JSON 형식이어야 한다.
// AC-OBS-002-3: {"error":{"code":"UNAUTHENTICATED","message":"..."}} (DISPUTE #4 fix)
func TestMetricsAuthMiddleware_NoToken_NestedErrorBody(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)
	v := testValidator(t)

	mw := metricsAuthMiddlewareWithMetrics(v, true, m)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"UNAUTHENTICATED"`, "에러 code가 UNAUTHENTICATED여야 한다")
	assert.Contains(t, body, `"error"`, "nested error key가 있어야 한다")
}

// TestMetricsAuthMiddleware_Forbidden_NestedErrorBody — 403 응답 body가 nested JSON 형식이어야 한다.
// AC-OBS-002-4: {"error":{"code":"FORBIDDEN","message":"..."}} (DISPUTE #4 fix)
func TestMetricsAuthMiddleware_Forbidden_NestedErrorBody(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	mw := metricsAuthzOnlyMiddleware(func(r *http.Request) (*auth.User, bool) {
		return &auth.User{UID: "viewer", Scopes: []string{"iroum-ax:viewer"}, Roles: []string{"viewer"}}, true
	}, m)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"FORBIDDEN"`, "에러 code가 FORBIDDEN이어야 한다")
	assert.Contains(t, body, `"error"`, "nested error key가 있어야 한다")
}

// TestMetricsAuthMiddleware_Forbidden_IncAuthzForbiddenCalled — 403 응답 시
// authzForbidden 카운터가 증가해야 한다 (DISPUTE #5 fix, AC-OBS-002-4).
func TestMetricsAuthMiddleware_Forbidden_IncAuthzForbiddenCalled(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	mw := metricsAuthzOnlyMiddleware(func(r *http.Request) (*auth.User, bool) {
		return &auth.User{UID: "viewer", Scopes: []string{"iroum-ax:viewer"}, Roles: []string{"viewer"}}, true
	}, m)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	// IncAuthzForbidden 호출 확인: registry.Gather로 카운터 값 검증
	mfs, err := reg.Gather()
	require.NoError(t, err)

	var forbiddenCount float64
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_authz_forbidden_total" {
			for _, metric := range mf.GetMetric() {
				forbiddenCount += metric.GetCounter().GetValue()
			}
		}
	}
	assert.Equal(t, float64(1), forbiddenCount,
		"403 응답 시 authzForbidden 카운터가 1 증가해야 한다")
}
