// validator_test_setup_test.go — 테스트 환경 초기화
// testKeys를 defaultJWKSProvider에 주입하여 newTestValidator가 올바른 키를 사용하도록 설정한다.
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"fmt"
	"sync"
	"testing"
)

// testJWKSProvider — testKeys를 직접 참조하는 테스트 전용 JWKS 제공자
type testJWKSProvider struct{}

func (p *testJWKSProvider) GetKey(_ context.Context, kid string) (any, string, string, error) {
	switch kid {
	case testKIDRSA:
		return &testKeys.rsaPriv.PublicKey, "RS256", "RSA", nil
	case testKIDEC:
		return &testKeys.ecPriv.PublicKey, "ES256", "EC", nil
	default:
		return nil, "", "", fmt.Errorf("%w: kid=%s", ErrJWKSUnavailable, kid)
	}
}

// testInMemoryBlacklist — 테스트용 인메모리 블랙리스트
// TestVerify_Blacklisted에서 jti-blacklisted-001이 등록된 상태로 사용된다.
// fieldalignment: 포인터(map=8바이트) 먼저, 큰 값 타입(RWMutex) 나중
type testInMemoryBlacklist struct {
	set map[string]struct{}
	mu  sync.RWMutex
}

func newTestInMemoryBlacklist(jtis ...string) *testInMemoryBlacklist {
	s := make(map[string]struct{}, len(jtis))
	for _, jti := range jtis {
		s[jti] = struct{}{}
	}
	return &testInMemoryBlacklist{set: s}
}

func (b *testInMemoryBlacklist) IsBlacklisted(_ context.Context, jti string) (bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.set[jti]
	return ok, nil
}

// TestMain — 테스트 실행 전 defaultJWKSProvider를 testKeys 기반으로 교체하고,
// newTestValidator가 테스트 블랙리스트를 포함하도록 설정한다.
func TestMain(m *testing.M) {
	// testKeys는 init()에서 초기화되므로 여기서 안전하게 참조 가능
	defaultJWKSProvider = func() JWKSProvider {
		return &testJWKSProvider{}
	}
	// newTestValidator가 사용할 기본 블랙리스트: jti-blacklisted-001 사전 등록
	defaultBlacklistProvider = func() BlacklistChecker {
		return newTestInMemoryBlacklist("jti-blacklisted-001")
	}
	m.Run()
}

// 테스트 전용 — 인터페이스 구현 컴파일 타임 검증
var _ JWKSProvider = (*testJWKSProvider)(nil)
var _ BlacklistChecker = (*testInMemoryBlacklist)(nil)
var _ *rsa.PublicKey = (*rsa.PublicKey)(nil)
var _ *ecdsa.PublicKey = (*ecdsa.PublicKey)(nil)
