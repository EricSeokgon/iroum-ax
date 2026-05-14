// gRPC / REST 인증 미들웨어 stub — SPEC-AX-AUTH-001 REQ-AUTH-003
// Sprint 3/4 GREEN에서 실제 구현 예정
package auth

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/grpc"
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
// @MX:TODO Sprint 3 — gRPC interceptor에서 사용
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext — context에서 인증된 사용자 정보를 추출한다.
// 사용자 정보가 없으면 nil, false를 반환한다.
//
// @MX:ANCHOR: [AUTO] context user 추출 단일 진입점
// @MX:REASON: gRPC handler / REST handler / audit recorder / RBAC check / scheduler dispatch 에서 호출 (fan_in >= 10)
// @MX:TODO Sprint 3 — 실제 context 추출 로직 검증
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userContextKey).(*User)
	return u, ok && u != nil
}

// UnaryServerInterceptor — gRPC 단항 RPC 인증 인터셉터 stub
//
// AuthEnabled=false 시 downstream handler를 그대로 통과한다 (backward compat).
// /grpc.health.v1.Health/Check RPC는 인증을 우회한다 (REQ-AUTH-003-E1).
//
// @MX:TODO Sprint 3 — TokenValidator.Verify 연동 후 실제 구현
func UnaryServerInterceptor(validator *TokenValidator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if validator == nil {
			// AuthEnabled=false: no-op 통과 (R-AUTH-007 backward compat)
			return handler(ctx, req)
		}
		return nil, errors.New("구현 예정: Sprint 3 GREEN")
	}
}

// RESTMiddleware — HTTP REST 인증 미들웨어 stub
//
// AuthEnabled=false 시 next handler를 그대로 통과한다 (backward compat).
// /health 경로는 인증을 우회한다 (REQ-AUTH-003-E2).
//
// @MX:TODO Sprint 4 — TokenValidator.Verify 연동 후 실제 구현
func RESTMiddleware(validator *TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if validator == nil {
				// AuthEnabled=false: no-op 통과 (R-AUTH-007 backward compat)
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "구현 예정: Sprint 4 GREEN", http.StatusNotImplemented)
		})
	}
}
