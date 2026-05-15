// jwks_cache_reachable_test.go — JWKSCache.Reachable() 단위 테스트
// SPEC-AX-SERVER-001 S0 deliverable: readiness probe 전용 신선도 판정 메서드 검증
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJWKSCache_Reachable_BeforeFirstFetch 최초 fetch 전에는 Reachable이 false를 반환해야 한다.
func TestJWKSCache_Reachable_BeforeFirstFetch(t *testing.T) {
	t.Parallel()
	// 실제 JWKS 서버 없이 URI만 설정
	jc := NewJWKSCache("http://nonexistent-jwks.example.com/.well-known/jwks.json")

	ctx := context.Background()
	// lastSuccessAt이 zero이므로 Reachable은 false 반환
	assert.False(t, jc.Reachable(ctx), "최초 fetch 전에는 Reachable()이 false를 반환해야 한다")
}

// TestJWKSCache_Reachable_AfterSuccessfulFetch 성공적인 fetch 후에는 Reachable이 true를 반환해야 한다.
func TestJWKSCache_Reachable_AfterSuccessfulFetch(t *testing.T) {
	t.Parallel()

	// 최소한의 JWKS JSON을 반환하는 테스트 HTTP 서버
	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// keys 배열이 비어 있어도 fetch 자체는 성공 (lastSuccessAt 갱신됨)
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer jwkServer.Close()

	jc := NewJWKSCache(jwkServer.URL,
		WithCacheTTL(5*time.Minute),
		WithStaleMaxAge(1*time.Hour),
	)

	ctx := context.Background()
	// fetch 수행 (kid 미존재 에러는 무시)
	_, _, _, _ = jc.GetKey(ctx, "any-kid") //nolint:errcheck

	// fetch 성공 후 Reachable은 true
	assert.True(t, jc.Reachable(ctx), "JWKS 서버 fetch 성공 후 Reachable()은 true를 반환해야 한다")
}

// TestJWKSCache_Reachable_StaleExpiry stale 만료 후에는 Reachable이 false를 반환해야 한다.
// lastSuccessAt을 강제로 과거로 설정하여 staleMaxAge 초과 시뮬레이션.
func TestJWKSCache_Reachable_StaleExpiry(t *testing.T) {
	t.Parallel()

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer jwkServer.Close()

	// staleMaxAge를 1초로 설정하여 빠른 만료 테스트
	jc := NewJWKSCache(jwkServer.URL,
		WithCacheTTL(100*time.Millisecond),
		WithStaleMaxAge(100*time.Millisecond),
	)

	ctx := context.Background()
	// fetch 수행으로 lastSuccessAt 갱신
	_, _, _, _ = jc.GetKey(ctx, "any-kid") //nolint:errcheck

	// staleMaxAge 이후로 시간 진행 시뮬레이션: lastSuccessAt을 과거로 강제 설정
	jc.mu.Lock()
	jc.lastSuccessAt = time.Now().Add(-200 * time.Millisecond) // staleMaxAge(100ms)를 초과
	jc.mu.Unlock()

	require.False(t, jc.Reachable(ctx),
		"lastSuccessAt이 staleMaxAge를 초과하면 Reachable()은 false를 반환해야 한다")
}

// TestJWKSCache_Reachable_RaceConditionSafe Reachable()이 -race 플래그에서 안전한지 검증.
// mu.RLock()이 올바르게 사용되면 race detector가 에러를 보고하지 않아야 한다.
func TestJWKSCache_Reachable_RaceConditionSafe(t *testing.T) {
	t.Parallel()

	jc := NewJWKSCache("http://unreachable.example.com/.well-known/jwks.json")
	ctx := context.Background()

	// 다수의 goroutine에서 동시 호출하여 race 감지
	const workers = 10
	done := make(chan struct{}, workers)
	for range workers {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = jc.Reachable(ctx)
		}()
	}

	for range workers {
		<-done
	}
	// race detector가 에러를 보고하지 않으면 테스트 통과
}
