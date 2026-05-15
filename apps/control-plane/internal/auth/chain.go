// chain.go — 미들웨어/인터셉터 체인 조합 헬퍼
// Sprint 0 GREEN: SPEC-AX-AUTH-002 REQ-AUTH2-002-E1 + REQ-AUTH2-003-E1 D7 fix
// 체인 순서를 코드로 강제: auth → authz → handler
package auth

import (
	"net/http"

	"google.golang.org/grpc"
)

// chainBuildOptions — BuildRESTChain 함수형 옵션
// 테스트에서 미들웨어를 교체하기 위한 override 메커니즘
type chainBuildOptions struct {
	// authnMiddleware 인증 미들웨어 override (nil이면 기본 RESTMiddleware 사용)
	authnMiddleware func(http.Handler) http.Handler
	// authzMiddleware 인가 미들웨어 override (nil이면 기본 RESTAuthzMiddleware 사용)
	authzMiddleware func(http.Handler) http.Handler
}

// ChainOption — BuildRESTChain 함수형 옵션 타입
type ChainOption func(*chainBuildOptions)

// WithMiddlewareOverride — 테스트에서 미들웨어를 교체하기 위한 옵션
// 프로덕션 코드에서는 사용하지 않는다.
func WithMiddlewareOverride(authn, authz func(http.Handler) http.Handler) ChainOption {
	return func(o *chainBuildOptions) {
		o.authnMiddleware = authn
		o.authzMiddleware = authz
	}
}

// BuildRESTChain — REST 미들웨어 체인을 조합하여 반환한다.
//
// 체인 순서 (D7 iter-2 fix): auth.RESTMiddleware → RESTAuthzMiddleware → handler
// 이 순서는 코드로 강제되며, 호출자가 순서를 뒤집을 수 없다.
//
// authEnabled=false이면 두 미들웨어 모두 chain에서 제외 (REQ-AUTH2-UBI-001-b 백워드 호환).
// recorder=nil이면 감사 기록 skip (테스트 등 특수 환경).
//
// @MX:ANCHOR: [AUTO] REST 체인 조합 단일 진입점 (fan_in >= 2: 테스트 + SPEC-AX-SERVER-001)
// @MX:REASON: D7 미들웨어 순서 강제 — 호출자가 순서를 뒤집을 수 없게 캡슐화
func BuildRESTChain(
	handler http.Handler,
	validator *TokenValidator,
	recorder auditRecorder,
	authEnabled bool,
	opts ...ChainOption,
) http.Handler {
	// 옵션 적용
	o := &chainBuildOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// authEnabled=false: 미들웨어 없이 handler 직접 반환 (REQ-AUTH2-UBI-001-b)
	if !authEnabled {
		return handler
	}

	// 인가 미들웨어 결정 (override 또는 기본)
	authzMW := o.authzMiddleware
	if authzMW == nil {
		authzMW = RESTAuthzMiddleware(recorder, authEnabled)
	}

	// 인증 미들웨어 결정 (override 또는 기본)
	authnMW := o.authnMiddleware
	if authnMW == nil {
		authnMW = RESTMiddleware(validator)
	}

	// 체인 조합: authn(authz(handler))
	// 실행 순서: authn → authz → handler
	return authnMW(authzMW(handler))
}

// BuildGRPCInterceptorChain — gRPC 단항 인터셉터 체인을 조합하여 ServerOption으로 반환한다.
//
// 체인 순서 (D7 iter-2 fix): auth.UnaryServerInterceptor → UnaryAuthzInterceptor → handler
// grpc.ChainUnaryInterceptor는 전달 순서대로 실행된다.
//
// authEnabled=false이면 빈 체인 ServerOption 반환 (interceptor 없이 handler 직접 호출).
//
// @MX:NOTE: [AUTO] gRPC 체인 조합 단일 진입점, chain order는 단위 테스트 TestBuildGRPCInterceptorChain_Order로 회귀 가드
func BuildGRPCInterceptorChain(
	validator *TokenValidator,
	recorder auditRecorder,
	authEnabled bool,
) grpc.ServerOption {
	if !authEnabled {
		// 빈 체인 반환 — 인터셉터 없이 handler 직접 호출
		return grpc.ChainUnaryInterceptor()
	}

	// 체인 순서: UnaryServerInterceptor(authn) → UnaryAuthzInterceptor(authz) → handler
	return grpc.ChainUnaryInterceptor(
		UnaryServerInterceptor(validator),
		UnaryAuthzInterceptor(recorder, authEnabled),
	)
}
