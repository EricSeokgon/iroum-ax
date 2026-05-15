// middleware.go — /metrics 전용 인증/인가 미들웨어 + HTTP instrumentation
// SPEC-AX-OBS-001 Sprint 1 GREEN: REQ-OBS-002 + REQ-OBS-003
// DISPUTE FIX: #4 nested 에러 body, #5 IncAuthzForbidden 호출
package metrics

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
)

// MetricsHandler — 전역 레지스트리에서 prometheus 메트릭을 노출하는 핸들러를 반환한다.
func MetricsHandler() http.Handler {
	return MetricsHandlerForRegistry(Registry())
}

// MetricsHandlerForRegistry — 지정된 Gatherer로 prometheus 핸들러를 생성한다 (테스트 격리용).
func MetricsHandlerForRegistry(reg prometheus.Gatherer) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

// MetricsAuthMiddleware — /metrics 전용 인증+인가 미들웨어.
//
// 처리 순서:
//  1. Authorization Bearer 토큰 파싱
//  2. TokenValidator.Verify → authn 실패 시 401
//  3. IsMetricsAuthorized → authz 실패 시 403 + IncAuthzForbidden 호출
//  4. auth.WithUser로 context 주입 후 next 호출
//
// authEnabled=false이면 인증 없이 통과 (spec §3.3: bypass 허용).
//
// @MX:NOTE: [AUTO] /metrics 경로 전용 authn+authz 미들웨어 — BuildRESTChain 외부에서 독립적으로 동작
func MetricsAuthMiddleware(v *auth.TokenValidator, authEnabled bool) func(http.Handler) http.Handler {
	return metricsAuthMiddlewareWithMetrics(v, authEnabled, global())
}

// metricsAuthMiddlewareWithMetrics — 테스트 격리를 위해 Metrics 인스턴스를 주입받는 내부 구현.
// @MX:NOTE: [AUTO] global() 대신 m 주입 — 테스트에서 isolated registry 사용 시 IncAuthzForbidden 관찰 가능
func metricsAuthMiddlewareWithMetrics(v *auth.TokenValidator, authEnabled bool, m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authEnabled || v == nil {
				// authEnabled=false: 인증 없이 통과
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
			// AC-OBS-002-3: 헤더 없음 / 비-Bearer / 검증실패 — 모두 단일 고정 메시지 401
			if !ok || authHeader == "" {
				writeMetricsError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required for metrics")
				return
			}

			vt, err := v.Verify(r.Context(), tokenStr)
			if err != nil {
				writeMetricsError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required for metrics")
				return
			}

			// ValidatedToken → User 변환
			u := validatedTokenToMetricsUser(vt)
			if !IsMetricsAuthorized(u) {
				// AC-OBS-002-4: 403 브랜치 — PERMISSION_DENIED + details:{"required":"read:metrics"}
				role := ""
				if len(u.Roles) > 0 {
					role = u.Roles[0]
				}
				m.IncAuthzForbidden(role, "/metrics")
				writeMetricsErrorWithDetails(w, http.StatusForbidden, "PERMISSION_DENIED", "insufficient scope",
					map[string]string{"required": "read:metrics"})
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
		})
	}
}

// MetricsAuthMiddlewareWithUserInjector — 테스트용 user injector를 받는 authz 전용 미들웨어.
// authn은 injector가 담당하고, authz(IsMetricsAuthorized)만 미들웨어가 수행한다.
func MetricsAuthMiddlewareWithUserInjector(
	inject func(r *http.Request) (*auth.User, bool),
) func(http.Handler) http.Handler {
	return metricsAuthzOnlyMiddleware(inject, global())
}

// metricsAuthzOnlyMiddleware — 테스트 격리용 authz 전용 미들웨어 (Metrics 주입).
func metricsAuthzOnlyMiddleware(
	inject func(r *http.Request) (*auth.User, bool),
	m *Metrics,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := inject(r)
			if !ok {
				writeMetricsError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required for metrics")
				return
			}
			if !IsMetricsAuthorized(u) {
				role := ""
				if len(u.Roles) > 0 {
					role = u.Roles[0]
				}
				m.IncAuthzForbidden(role, "/metrics")
				// AC-OBS-002-4: acceptance.md L93 정확 body
				writeMetricsErrorWithDetails(w, http.StatusForbidden, "PERMISSION_DENIED", "insufficient scope",
					map[string]string{"required": "read:metrics"})
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
		})
	}
}

// probePaths — REQ-OBS-003-S1: 계측 제외 경로 집합 (probe + self-scrape)
// self-scrape recursion + SLA 메트릭 오염 방지
var probePaths = map[string]struct{}{
	"/health":  {},
	"/ready":   {},
	"/metrics": {},
}

// HTTPInstrumentationMiddleware — HTTP 요청 레이턴시를 히스토그램에 기록하는 최외곽 미들웨어.
// REQ-OBS-003-S1: /health, /ready, /metrics 경로는 observe skip (요청은 정상 통과).
// 인증 실패를 포함한 그 외 모든 요청을 계측한다 (REQ-OBS-003-E1).
//
// @MX:NOTE: [AUTO] probe/metrics 경로는 self-scrape 노이즈 방지 위해 계측 제외 — REQ-OBS-003-S1
func HTTPInstrumentationMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// REQ-OBS-003-S1: probe 경로 skip — 요청은 통과, observe만 제외
			if _, isProbe := probePaths[r.URL.Path]; isProbe {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rw, r)
			elapsed := time.Since(start).Seconds()
			m.ObserveHTTPDuration(r.Method, r.URL.Path, strconv.Itoa(rw.code), elapsed)
		})
	}
}

// responseWriter — 상태 코드를 캡처하는 http.ResponseWriter 래퍼
type responseWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.code = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

// metricsErrorBody — /metrics 에러 응답 nested JSON body 구조
// AC-OBS-002-3: {"error":{"code":"UNAUTHENTICATED","message":"authentication required for metrics"}}
// AC-OBS-002-4: {"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"read:metrics"}}}
type metricsErrorBody struct {
	Error metricsErrorDetail `json:"error"`
}

// metricsErrorDetail — 에러 상세 구조 (fieldalignment: 인터페이스/맵 우선)
type metricsErrorDetail struct {
	// Details — 403 PERMISSION_DENIED에만 존재 (optional, omitempty)
	Details map[string]string `json:"details,omitempty"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
}

// writeMetricsError — /metrics 에러 응답 작성 (nested JSON body)
// 401: code="UNAUTHENTICATED", message="authentication required for metrics" (AC-OBS-002-3 단일 고정값)
// 403: code="PERMISSION_DENIED", message="insufficient scope", details={"required":"read:metrics"} (AC-OBS-002-4)
func writeMetricsError(w http.ResponseWriter, statusCode int, code, message string) {
	writeMetricsErrorWithDetails(w, statusCode, code, message, nil)
}

// writeMetricsErrorWithDetails — details 필드 포함 에러 응답 작성 (AC-OBS-002-4)
func writeMetricsErrorWithDetails(w http.ResponseWriter, statusCode int, code, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	body := metricsErrorBody{Error: metricsErrorDetail{Code: code, Message: message, Details: details}}
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck // 헤더 전송 후 encode 실패는 복구 불가
}

// validatedTokenToMetricsUser — ValidatedToken에서 auth.User를 구성한다.
// metrics 패키지에서 auth 패키지 내부 함수(validatedTokenToUser)에 접근 불가하므로 재구현.
func validatedTokenToMetricsUser(vt *auth.ValidatedToken) *auth.User {
	var roles []string
	if rawRoles, ok := vt.Claims["roles"]; ok {
		if roleSlice, ok := rawRoles.([]any); ok {
			for _, r := range roleSlice {
				if s, ok := r.(string); ok {
					roles = append(roles, s)
				}
			}
		}
	}
	if len(roles) == 0 {
		if ra, ok := vt.Claims["realm_access"].(map[string]any); ok {
			if rawRoles, ok := ra["roles"].([]any); ok {
				for _, r := range rawRoles {
					if s, ok := r.(string); ok {
						roles = append(roles, s)
					}
				}
			}
		}
	}
	return &auth.User{
		UID:    vt.Subject,
		Issuer: vt.Issuer,
		Roles:  roles,
		Scopes: vt.Scopes,
	}
}
