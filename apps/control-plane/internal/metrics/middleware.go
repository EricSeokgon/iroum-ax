// middleware.go — /metrics 전용 인증/인가 미들웨어 + HTTP instrumentation
// SPEC-AX-OBS-001 Sprint 1 GREEN: REQ-OBS-002 + REQ-OBS-003
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
//  3. IsMetricsAuthorized → authz 실패 시 403
//  4. auth.WithUser로 context 주입 후 next 호출
//
// authEnabled=false이면 인증 없이 통과 (spec §3.3: bypass 허용).
//
// @MX:NOTE: [AUTO] /metrics 경로 전용 authn+authz 미들웨어 — BuildRESTChain 외부에서 독립적으로 동작
func MetricsAuthMiddleware(v *auth.TokenValidator, authEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authEnabled || v == nil {
				// authEnabled=false: 인증 없이 통과
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeMetricsError(w, http.StatusUnauthorized, "missing_authorization")
				return
			}

			tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok {
				writeMetricsError(w, http.StatusUnauthorized, "invalid_request")
				return
			}

			vt, err := v.Verify(r.Context(), tokenStr)
			if err != nil {
				writeMetricsError(w, http.StatusUnauthorized, "invalid_token")
				return
			}

			// ValidatedToken → User 변환
			u := validatedTokenToMetricsUser(vt)
			if !IsMetricsAuthorized(u) {
				writeMetricsError(w, http.StatusForbidden, "forbidden")
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
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := inject(r)
			if !ok {
				writeMetricsError(w, http.StatusUnauthorized, "unauthenticated")
				return
			}
			if !IsMetricsAuthorized(u) {
				writeMetricsError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
		})
	}
}

// HTTPInstrumentationMiddleware — HTTP 요청 레이턴시를 히스토그램에 기록하는 최외곽 미들웨어.
// 인증 실패를 포함한 모든 요청을 계측한다 (REQ-OBS-003).
func HTTPInstrumentationMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// writeMetricsError — /metrics 에러 응답 작성 (JSON body)
func writeMetricsError(w http.ResponseWriter, code int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": errMsg}) //nolint:errcheck // 헤더 전송 후 encode 실패는 복구 불가
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
