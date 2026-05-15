// gRPC / REST 인증 미들웨어 — SPEC-AX-AUTH-001 REQ-AUTH-003
// Sprint 3 GREEN: UnaryServerInterceptor + RESTMiddleware 실제 구현
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// contextKey — context.Context 저장 키 타입 (package-local)
type contextKey struct{}

var userContextKey = contextKey{}

// User — context에 저장되는 인증된 사용자 정보
// fieldalignment-friendly: 문자열 먼저, 슬라이스 마지막
type User struct {
	// UID — JWT sub 클레임 값 (Keycloak user ID)
	UID string
	// Issuer — 검증된 issuer URL
	Issuer string
	// Roles — 파싱된 역할 목록 (admin/analyst/viewer)
	Roles []string
	// Scopes — 파싱된 스코프 목록
	Scopes []string
}

// WithUser — context에 인증된 사용자 정보를 주입한다.
//
// @MX:ANCHOR: [AUTO] context user 주입 단일 진입점
// @MX:REASON: gRPC interceptor / REST middleware / 테스트 helper 에서 호출 (fan_in >= 3)
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext — context에서 인증된 사용자 정보를 추출한다.
// 사용자 정보가 없으면 nil, false를 반환한다.
//
// @MX:ANCHOR: [AUTO] context user 추출 단일 진입점
// @MX:REASON: gRPC handler / REST handler / audit recorder / RBAC check / scheduler dispatch 에서 호출 (fan_in >= 10)
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userContextKey).(*User)
	return u, ok && u != nil
}

// validatedTokenToUser — ValidatedToken을 User로 변환한다.
// 역할 클레임은 "roles" 또는 "realm_access.roles"에서 추출한다.
func validatedTokenToUser(vt *ValidatedToken) *User {
	// 역할 추출: "roles" 클레임 우선, 없으면 "realm_access.roles" 확인
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
	// realm_access.roles (Keycloak 표준 클레임)
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

	return &User{
		UID:    vt.Subject,
		Issuer: vt.Issuer,
		Roles:  roles,
		Scopes: vt.Scopes,
	}
}

// UnaryServerInterceptor — gRPC 단항 RPC 인증 인터셉터
//
// validator가 nil이면 AuthEnabled=false(no-op 통과) 모드로 동작한다.
// /grpc.health.v1.Health/Check RPC는 인증을 우회한다 (REQ-AUTH-003-E1).
//
// @MX:ANCHOR: [AUTO] gRPC 단항 인증 인터셉터 진입점
// @MX:REASON: gRPC 서버 등록 시 단일 진입점으로 등록 (fan_in >= 3)
func UnaryServerInterceptor(validator *TokenValidator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// Health check 우회: 인증 없이 통과 (REQ-AUTH-003-E1)
		if info.FullMethod == "/grpc.health.v1.Health/Check" {
			return handler(ctx, req)
		}

		// AuthEnabled=false: validator가 nil이면 no-op 통과 (backward compat)
		if validator == nil {
			return handler(ctx, req)
		}

		// incoming metadata에서 authorization 헤더 추출
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "인증 메타데이터가 없습니다")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return nil, status.Error(codes.Unauthenticated, "Authorization 헤더가 필요합니다")
		}

		// "Bearer <token>" 파싱
		tokenStr, ok := strings.CutPrefix(authHeaders[0], "Bearer ")
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "Authorization scheme이 올바르지 않습니다 (Bearer 필요)")
		}

		// 토큰 검증
		vt, err := validator.Verify(ctx, tokenStr)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "토큰 검증 실패: %v", err)
		}

		// User 변환 후 context 주입
		u := validatedTokenToUser(vt)
		return handler(WithUser(ctx, u), req)
	}
}

// RESTMiddleware — HTTP REST 인증 미들웨어
//
// validator가 nil이면 AuthEnabled=false(no-op 통과) 모드로 동작한다.
// /health 경로는 인증을 우회한다 (REQ-AUTH-003-E2).
//
// @MX:WARN: [AUTO] writeAuthError에서 WWW-Authenticate 헤더 형식을 직접 문자열로 관리
// @MX:REASON: RFC 6750 §3 요구사항으로 형식이 정해져 있어 추상화 시 가독성이 저하됨
func RESTMiddleware(validator *TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Health check 경로 우회 (REQ-AUTH-003-E2)
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			// AuthEnabled=false: validator가 nil이면 no-op 통과
			if validator == nil {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Authorization 헤더 없음 → 401 + WWW-Authenticate
				writeAuthError(w, http.StatusUnauthorized, "missing_authorization",
					`Bearer realm="iroum-ax"`)
				return
			}

			// "Bearer <token>" 파싱
			tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok {
				// Bearer 접두사 없음 → 401 + invalid_request
				writeAuthError(w, http.StatusUnauthorized, "invalid_request",
					`Bearer realm="iroum-ax", error="invalid_request"`)
				return
			}

			// 토큰 검증
			vt, err := validator.Verify(r.Context(), tokenStr)
			if err != nil {
				// 에러 유형에 따라 WWW-Authenticate 헤더 세분화
				wwwAuth := `Bearer realm="iroum-ax", error="invalid_token"`
				if errors.Is(err, ErrTokenExpired) {
					wwwAuth = `Bearer realm="iroum-ax", error="invalid_token", error_description="token expired"`
				}
				writeAuthError(w, http.StatusUnauthorized, "invalid_token", wwwAuth)
				return
			}

			// User 변환 후 context 주입하여 next 호출
			u := validatedTokenToUser(vt)
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}

// writeAuthError — 인증 실패 응답을 작성한다.
// RFC 6750 §3: WWW-Authenticate 헤더 + JSON 응답 body
func writeAuthError(w http.ResponseWriter, statusCode int, errCode string, wwwAuth string) {
	w.Header().Set("WWW-Authenticate", wwwAuth)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// 인코딩 에러는 응답이 이미 시작된 후이므로 무시한다
	_ = json.NewEncoder(w).Encode(map[string]string{"error": errCode}) //nolint:errcheck // 응답 헤더 전송 후 encode 실패는 복구 불가
}
