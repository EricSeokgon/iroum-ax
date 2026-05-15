// authz_middleware.go — REST 인가 미들웨어 + gRPC 인가 인터셉터
// Sprint 1 GREEN: SPEC-AX-AUTH-002 REQ-AUTH2-002-E1/U1/U2, REQ-AUTH2-003-E1/U1
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// auditRecorder — RESTAuthzMiddleware가 사용하는 감사 기록 인터페이스
// audit.Recorder를 직접 의존하지 않고 인터페이스로 추상화 (테스트 용이성)
type auditRecorder interface {
	// LogForbiddenEvent AUTH_FORBIDDEN 감사 이벤트를 기록한다.
	// required: 요구 권한 문자열 (details.required 필드에 기록)
	LogForbiddenEvent(ctx context.Context, method, path, required, userID string, grantedRoles []Role) error
}

// forbiddenDetails — 403 응답 body의 details 필드
type forbiddenDetails struct {
	// Required 요구 권한
	Required string `json:"required"`
	// Granted 사용자가 보유한 역할 목록
	Granted []string `json:"granted"`
}

// forbiddenErrorBody — 403 응답 body
type forbiddenErrorBody struct {
	Error forbiddenErrObj `json:"error"`
}

type forbiddenErrObj struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Details forbiddenDetails `json:"details"`
}

// mappingMissingBody — 503 응답 body (default-deny)
type mappingMissingBody struct {
	Error mappingMissingErr `json:"error"`
}

type mappingMissingErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// userMissingBody — 500 응답 body (user context 없음)
type userMissingBody struct {
	Error userMissingErr `json:"error"`
}

type userMissingErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// contextGrantKey — context에 granted_permission을 주입하기 위한 키
type contextGrantKey struct{}

// WithGrantedPermission — context에 granted permission을 주입한다.
// REQ-AUTH2-002-E2: 성공 시 핸들러 audit row에 통합될 수 있도록 context annotation
func WithGrantedPermission(ctx context.Context, perm string) context.Context {
	return context.WithValue(ctx, contextGrantKey{}, perm)
}

// GrantedPermissionFromContext — context에서 granted permission을 추출한다.
func GrantedPermissionFromContext(ctx context.Context) (string, bool) {
	p, ok := ctx.Value(contextGrantKey{}).(string)
	return p, ok && p != ""
}

// RESTAuthzMiddleware — REST 인가 미들웨어
//
// 동작 순서 (REQ-AUTH2-UBI-001-c: 사전 차단):
//  1. authEnabled=false → next.ServeHTTP 바로 호출 (REQ-AUTH2-UBI-001-b)
//  2. bypass 경로 → next.ServeHTTP 바로 호출
//  3. 매핑 없음 → 503 + audit (REQ-AUTH2-001-U1)
//  4. User context 없음 → 500 (REQ-AUTH2-002-U2, wiring 버그 감지)
//  5. Authorize 실패 → 403 + audit (REQ-AUTH2-002-U1)
//  6. Authorize 성공 → context annotation 후 next 호출 (REQ-AUTH2-002-E1)
//
// @MX:NOTE: [AUTO] 인증 통과 직후 권한 결정의 단일 결정점 (REQ-AUTH2-UBI-001-c 사전 차단)
func RESTAuthzMiddleware(recorder auditRecorder, authEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// AuthEnabled=false: 인가 체크 전체 skip (REQ-AUTH2-UBI-001-b)
			if !authEnabled {
				next.ServeHTTP(w, r)
				return
			}

			method := r.Method
			path := r.URL.Path

			// 매핑 조회
			perm, bypass, found := LookupRESTPermission(method, path)

			// bypass 경로 → 통과 (REQ-AUTH2-001-E1 bypass rules)
			if bypass {
				next.ServeHTTP(w, r)
				return
			}

			// @MX:WARN: [AUTO] 매핑 미정의 시 503으로 fail-closed (open-by-default 패턴 거부)
			// @MX:REASON: 보안 결정 누락 방지; 매핑 hot-patch 금지 (REQ-AUTH2-001-U1)
			if !found {
				writeMappingMissing(w)
				return
			}

			// User context 확인 (REQ-AUTH2-002-U2: user 없으면 500, wiring 버그)
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeUserMissing(w)
				return
			}

			// Authorize 호출 (REQ-AUTH2-002-E1)
			if err := Authorize(r.Context(), perm); err != nil {
				if errors.Is(err, ErrInsufficientPermission) {
					// 403 + audit (REQ-AUTH2-002-U1)
					roles := ParseRolesFromScope(strings.Join(user.Scopes, " "))
					writeForbidden(w, perm, roles)

					// AUTH_FORBIDDEN 감사 이벤트 기록
					if recorder != nil {
						if auditErr := recorder.LogForbiddenEvent(r.Context(), method, path, perm, user.UID, roles); auditErr != nil {
							// 감사 기록 실패 — 403 응답은 이미 전송됨, 복구 불가; 무시
							_ = auditErr
						}
					}
					return
				}
				// 예상치 못한 에러 → 500
				writeUserMissing(w)
				return
			}

			// 성공: context에 granted_permission annotation 후 next 호출 (REQ-AUTH2-002-E2)
			ctx := WithGrantedPermission(r.Context(), perm)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UnaryAuthzInterceptor — gRPC 단항 인가 인터셉터
//
// 동작 순서 (REQ-AUTH2-003-E1):
//  1. authEnabled=false → handler 바로 호출
//  2. bypass 메서드 → handler 바로 호출
//  3. 매핑 없음 → codes.Unavailable (REQ-AUTH2-001-U1)
//  4. Authorize 실패 → codes.PermissionDenied + audit (REQ-AUTH2-003-U1)
//  5. Authorize 성공 → handler 호출
//
// @MX:NOTE: [AUTO] gRPC 권한 결정 단일 결정점 (REQ-AUTH2-UBI-001-c 사전 차단)
func UnaryAuthzInterceptor(recorder auditRecorder, authEnabled bool) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// AuthEnabled=false: 인가 체크 전체 skip
		if !authEnabled {
			return handler(ctx, req)
		}

		perm, bypass, found := LookupGRPCPermission(info.FullMethod)

		// bypass 메서드 (Health Check 등)
		if bypass {
			return handler(ctx, req)
		}

		// 매핑 없음 → codes.Unavailable (default-deny, REQ-AUTH2-001-U1)
		if !found {
			return nil, status.Errorf(codes.Unavailable,
				"authorization mapping not defined for this method: %s", info.FullMethod)
		}

		// Authorize 호출
		if err := Authorize(ctx, perm); err != nil {
			// codes.PermissionDenied + audit (REQ-AUTH2-003-U1)
			if recorder != nil {
				if user, ok := UserFromContext(ctx); ok {
					roles := ParseRolesFromScope(strings.Join(user.Scopes, " "))
					if auditErr := recorder.LogForbiddenEvent(ctx, info.FullMethod, "", perm, user.UID, roles); auditErr != nil {
						// 감사 기록 실패 — PermissionDenied 응답은 이미 반환 예정, 무시
						_ = auditErr
					}
				}
			}
			return nil, status.Errorf(codes.PermissionDenied,
				"insufficient scope: required=%s", perm)
		}

		return handler(ctx, req)
	}
}

// writeForbidden — 403 Forbidden 응답 전송 (REQ-AUTH2-002-U1)
// RFC 6750 §3: WWW-Authenticate 헤더 포함
func writeForbidden(w http.ResponseWriter, required string, grantedRoles []Role) {
	grantedStrs := make([]string, len(grantedRoles))
	for i, r := range grantedRoles {
		grantedStrs[i] = string(r)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate",
		`Bearer realm="iroum-ax", error="insufficient_scope", scope="`+required+`"`)
	w.WriteHeader(http.StatusForbidden)

	body := forbiddenErrorBody{
		Error: forbiddenErrObj{
			Code:    "PERMISSION_DENIED",
			Message: "insufficient scope",
			Details: forbiddenDetails{
				Required: required,
				Granted:  grantedStrs,
			},
		},
	}
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck
}

// writeMappingMissing — 503 Service Unavailable 응답 전송 (REQ-AUTH2-001-U1 default-deny)
func writeMappingMissing(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	body := mappingMissingBody{
		Error: mappingMissingErr{
			Code:    "AUTHZ_MAPPING_MISSING",
			Message: "authorization mapping not defined for this method",
		},
	}
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck
}

// writeUserMissing — 500 Internal Server Error 응답 전송 (REQ-AUTH2-002-U2)
func writeUserMissing(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	body := userMissingBody{
		Error: userMissingErr{
			Code:    "AUTHZ_USER_MISSING",
			Message: "authenticated user context not propagated",
		},
	}
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck
}
