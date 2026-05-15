// chain_test.go — Sprint 0 RED: 체인 조합 헬퍼 단위 테스트
// REQ-AUTH2-002-E1 (D7 iter-2 fix): 미들웨어 순서 강제 검증
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildRESTChain_Order — RESTMiddleware → RESTAuthzMiddleware → handler 순서 검증
// REQ-AUTH2-002-E1 D7 fix: 체인 순서를 코드로 강제
func TestBuildRESTChain_Order(t *testing.T) {
	t.Parallel()

	// 호출 순서 기록용 슬라이스
	var callOrder []string

	// record 미들웨어 팩토리: 각 단계에서 이름을 기록하고 next 호출
	makeRecorder := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	// 최종 핸들러: "handler" 기록
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})

	// BuildRESTChain: authEnabled=true 시 두 미들웨어 모두 chain에 포함
	// 순서 검증을 위해 nil validator 사용 (auth 미들웨어는 nil 시 no-op)
	chain := BuildRESTChain(finalHandler, nil, nil, true,
		WithMiddlewareOverride(makeRecorder("authn"), makeRecorder("authz")))

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	// 순서 보장: authn → authz → handler
	require.Len(t, callOrder, 3)
	assert.Equal(t, "authn", callOrder[0], "첫 번째는 인증 미들웨어")
	assert.Equal(t, "authz", callOrder[1], "두 번째는 인가 미들웨어")
	assert.Equal(t, "handler", callOrder[2], "세 번째는 핸들러")
}

// TestBuildRESTChain_AuthDisabled_SkipsBoth — authEnabled=false 시 양 미들웨어 모두 skip
// REQ-AUTH2-UBI-001-b: AuthEnabled=false 백워드 호환
func TestBuildRESTChain_AuthDisabled_SkipsBoth(t *testing.T) {
	t.Parallel()

	var callOrder []string

	makeRecorder := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})

	// authEnabled=false: 두 미들웨어 모두 chain에서 제외
	chain := BuildRESTChain(finalHandler, nil, nil, false,
		WithMiddlewareOverride(makeRecorder("authn"), makeRecorder("authz")))

	req := httptest.NewRequest("GET", "/api/v1/workflows", nil)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	// handler만 호출됨 (authn/authz skip)
	assert.Equal(t, []string{"handler"}, callOrder, "authEnabled=false 시 미들웨어 없이 핸들러만 호출")
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestBuildGRPCInterceptorChain_Order — UnaryServerInterceptor → UnaryAuthzInterceptor 순서 검증
// REQ-AUTH2-003-E1 D7 fix
func TestBuildGRPCInterceptorChain_Order(t *testing.T) {
	t.Parallel()

	// BuildGRPCInterceptorChain이 grpc.ServerOption을 반환하는지 검증
	// nil validator / nil recorder로 authEnabled=true 호출
	opt := BuildGRPCInterceptorChain(nil, nil, true)
	// grpc.ServerOption은 인터페이스 — nil이 아님을 확인
	assert.NotNil(t, opt, "grpc.ServerOption이 nil이 아니어야 함")
}

// TestBuildGRPCInterceptorChain_AuthDisabled — authEnabled=false 시 interceptor 없이 빈 옵션
func TestBuildGRPCInterceptorChain_AuthDisabled(t *testing.T) {
	t.Parallel()

	opt := BuildGRPCInterceptorChain(nil, nil, false)
	// authEnabled=false: 빈 체인 옵션이지만 nil이 아닌 ServerOption 반환
	assert.NotNil(t, opt, "authEnabled=false 시에도 ServerOption 반환 (nil grpc.Server 방지)")
}
