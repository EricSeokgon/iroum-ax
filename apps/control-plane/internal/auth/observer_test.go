// observer_test.go — RejectionObserver 인터페이스 + nil-safe 검증 테스트
// SPEC-AX-OBS-001 Sprint 0 RED
package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubObserver — 테스트용 RejectionObserver 구현체
type stubObserver struct {
	calls []string
}

func (s *stubObserver) IncAuthRejection(reason string) {
	s.calls = append(s.calls, reason)
}

// TestRejectionObserver_InterfaceSatisfied — RejectionObserver 인터페이스가 구조적으로 만족되는지 확인한다.
func TestRejectionObserver_InterfaceSatisfied(t *testing.T) {
	t.Parallel()
	// stubObserver가 RejectionObserver를 구현하는지 컴파일 타임 검증
	var _ RejectionObserver = (*stubObserver)(nil)
}

// TestWithRejectionObserver_NilSafe — observer가 nil이어도 Verify가 패닉 없이 동작해야 한다.
func TestWithRejectionObserver_NilSafe(t *testing.T) {
	t.Parallel()
	// observer 없이 validator 생성 — nil-safe 동작 확인
	v, err := New(context.Background(), "https://example.com", "test-aud",
		WithJWKSProvider(&alwaysFailJWKS{}),
	)
	require.NoError(t, err)
	// observer 미설정 상태에서 검증 실패 — 패닉 없어야 함
	_, err = v.Verify(context.Background(), "invalid.token.string")
	assert.Error(t, err, "잘못된 토큰이면 에러가 반환돼야 한다")
}

// TestWithRejectionObserver_Called — Verify reject 분기에서 observer가 호출되어야 한다.
func TestWithRejectionObserver_Called(t *testing.T) {
	t.Parallel()
	obs := &stubObserver{}
	v, err := New(context.Background(), "https://example.com", "test-aud",
		WithJWKSProvider(&alwaysFailJWKS{}),
		WithRejectionObserver(obs),
	)
	require.NoError(t, err)

	// 잘못된 토큰으로 Verify 호출 — observer.IncAuthRejection이 호출돼야 함
	_, err = v.Verify(context.Background(), "bad.token.value")
	assert.Error(t, err)
	assert.NotEmpty(t, obs.calls, "reject 시 observer.IncAuthRejection이 최소 1회 호출돼야 한다")
}

// TestWithRejectionObserver_ReasonPropagated — reject 이유가 observer에 전달되어야 한다.
func TestWithRejectionObserver_ReasonPropagated(t *testing.T) {
	t.Parallel()
	obs := &stubObserver{}
	v, err := New(context.Background(), "https://example.com", "test-aud",
		WithJWKSProvider(&alwaysFailJWKS{}),
		WithRejectionObserver(obs),
	)
	require.NoError(t, err)

	_, _ = v.Verify(context.Background(), "bad.token.value")
	// reason이 빈 문자열이 아니어야 한다
	if len(obs.calls) > 0 {
		assert.NotEmpty(t, obs.calls[0], "reason이 전달돼야 한다")
	}
}

// alwaysFailJWKS — 테스트용 JWKSProvider (키 없음 → 항상 에러 반환)
type alwaysFailJWKS struct{}

func (a *alwaysFailJWKS) GetKey(_ context.Context, _ string) (any, string, string, error) {
	return nil, "", "", ErrJWKSUnavailable
}
