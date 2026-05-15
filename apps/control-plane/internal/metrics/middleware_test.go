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

// ── AC-OBS-003-3 probe 경로 제외 테스트 (RED → GREEN) ────────────────────────

// TestHTTPInstrumentationMiddleware_ProbePathsSkipped — REQ-OBS-003-S1
// /health, /ready, /metrics 요청은 iroum_ax_http_request_duration_seconds에 기록되지 않아야 한다.
// 요청은 정상 통과하되 latency observe만 skip (self-scrape SLA 오염 방지).
func TestHTTPInstrumentationMiddleware_ProbePathsSkipped(t *testing.T) {
	t.Parallel()

	probePaths := []string{"/health", "/ready", "/metrics"}

	for _, path := range probePaths {
		path := path
		t.Run("probe_skip:"+path, func(t *testing.T) {
			t.Parallel()

			reg := prometheus.NewRegistry()
			m := newMetricsWithRegistry(reg)

			// probe 경로 핸들러 — 200 OK 반환
			probe := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			mw := HTTPInstrumentationMiddleware(m)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			mw(probe).ServeHTTP(w, req)

			// 요청은 정상 통과
			assert.Equal(t, http.StatusOK, w.Code, path+"는 200으로 정상 통과해야 한다")

			// REQ-OBS-003-S1: probe 경로 샘플 부재 assert
			mfs, err := reg.Gather()
			require.NoError(t, err)

			for _, mf := range mfs {
				if mf.GetName() == "iroum_ax_http_request_duration_seconds" {
					for _, metric := range mf.GetMetric() {
						for _, lp := range metric.GetLabel() {
							if lp.GetName() == "path" && lp.GetValue() == path {
								t.Errorf("probe 경로 %q가 iroum_ax_http_request_duration_seconds에 기록됨 — REQ-OBS-003-S1 위반", path)
							}
						}
					}
				}
			}
		})
	}
}

// TestHTTPInstrumentationMiddleware_NonProbePathRecorded — REQ-OBS-003-E1
// 일반 경로(/api/v1/...)는 iroum_ax_http_request_duration_seconds에 정상 기록되어야 한다.
func TestHTTPInstrumentationMiddleware_NonProbePathRecorded(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := HTTPInstrumentationMiddleware(m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

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
	assert.True(t, found, "일반 경로는 iroum_ax_http_request_duration_seconds에 기록되어야 한다 (REQ-OBS-003-E1)")
}

// ── 기존 테스트 ────────────────────────────────────────────────────────────────

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

// TestMetricsAuthMiddleware_NoToken_NestedErrorBody — AC-OBS-002-3 정확 body 검증
// acceptance.md L87: {"error":{"code":"UNAUTHENTICATED","message":"authentication required for metrics"}}
// 토큰 없음/비-Bearer/검증실패 등 모든 401 케이스 동일 단일 고정값 (spec SSOT)
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
	// acceptance.md L87 정확 리터럴 검증
	assert.Contains(t, body, `"UNAUTHENTICATED"`, "AC-OBS-002-3: code=UNAUTHENTICATED")
	assert.Contains(t, body, `"authentication required for metrics"`,
		"AC-OBS-002-3: message 단일 고정값 'authentication required for metrics'")
	assert.Contains(t, body, `"error"`, "nested error key가 있어야 한다")
}

// TestMetricsAuthMiddleware_NonBearer_Returns401_FixedMessage — AC-OBS-002-3 (ii) sub-case
// Bearer scheme 아닌 토큰도 동일 고정 메시지 401이어야 한다.
func TestMetricsAuthMiddleware_NonBearer_Returns401_FixedMessage(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)
	v := testValidator(t)

	mw := metricsAuthMiddlewareWithMetrics(v, true, m)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Token xyz") // Bearer scheme 아님
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"authentication required for metrics"`,
		"AC-OBS-002-3 (ii): 비-Bearer도 단일 고정 메시지")
}

// TestMetricsAuthMiddleware_InvalidToken_Returns401_FixedMessage — AC-OBS-002-3 (iii) sub-case
// 검증 실패 토큰도 동일 고정 메시지 401이어야 한다.
func TestMetricsAuthMiddleware_InvalidToken_Returns401_FixedMessage(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)
	v := testValidator(t)

	mw := metricsAuthMiddlewareWithMetrics(v, true, m)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer bad.invalid.token")
	w := httptest.NewRecorder()
	mw(dummyHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"authentication required for metrics"`,
		"AC-OBS-002-3 (iii): 검증 실패도 단일 고정 메시지")
}

// TestMetricsAuthMiddleware_Forbidden_NestedErrorBody — AC-OBS-002-4 정확 body 검증 (false-green 교정)
// acceptance.md L93: {"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"read:metrics"}}}
// 구 false-green: code="FORBIDDEN" assert → spec oracle: code="PERMISSION_DENIED" + details 검증
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
	// acceptance.md L93 정확 리터럴 검증 (구: "FORBIDDEN" → 신: "PERMISSION_DENIED")
	assert.Contains(t, body, `"PERMISSION_DENIED"`,
		"AC-OBS-002-4: code=PERMISSION_DENIED (acceptance.md L93 oracle)")
	assert.Contains(t, body, `"insufficient scope"`,
		"AC-OBS-002-4: message='insufficient scope'")
	assert.Contains(t, body, `"required"`,
		"AC-OBS-002-4: details.required 필드 존재")
	assert.Contains(t, body, `"read:metrics"`,
		"AC-OBS-002-4: details.required='read:metrics'")
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
