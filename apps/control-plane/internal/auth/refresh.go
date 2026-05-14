// Refresh token 블랙리스트 + family tracking stub — SPEC-AX-AUTH-001 REQ-AUTH-005
// Sprint 6 GREEN에서 실제 구현 예정
package auth

import "context"

// RefreshTokenStore — refresh token 블랙리스트 및 family tracking 인터페이스
//
// @MX:TODO Sprint 6 — Redis Lua script 기반 atomic family invalidation 구현
type RefreshTokenStore interface {
	// BlacklistJTI — jti를 Redis 블랙리스트에 등록한다 (TTL = token.exp - now)
	// Redis key: auth:blacklist:<jti>
	BlacklistJTI(ctx context.Context, jti string, ttlSecs int64) error

	// IsBlacklisted — jti가 블랙리스트에 있는지 확인한다
	IsBlacklisted(ctx context.Context, jti string) (bool, error)

	// InvalidateFamily — refresh token family 전체를 무효화한다
	// Redis key: auth:refresh_family:<familyID>
	// Lua script atomic check-and-blacklist 사용 (REQ-AUTH-005-U1)
	//
	// @MX:WARN: [AUTO] Lua script atomic 조건문 복잡도 예상, fallback 전략 필수
	// @MX:REASON: Redis race condition mitigation — Get family + Validate jti + Blacklist family를 atomic 수행, eval 실패 시 보수적 family 전체 blacklist
	InvalidateFamily(ctx context.Context, familyID string) error
}

// RedisRefreshStore — Redis 기반 RefreshTokenStore 구현체 stub
//
// @MX:TODO Sprint 6 — redis.Client 필드 추가 및 메서드 구현
type RedisRefreshStore struct {
	// redisAddr — Redis 서버 주소 (Sprint 6에서 *redis.Client로 교체)
	redisAddr string
}

// NewRedisRefreshStore — RedisRefreshStore 생성자 stub
func NewRedisRefreshStore(redisAddr string) *RedisRefreshStore {
	return &RedisRefreshStore{redisAddr: redisAddr}
}

// BlacklistJTI — stub
func (s *RedisRefreshStore) BlacklistJTI(_ context.Context, _ string, _ int64) error {
	return nil // Sprint 6에서 구현
}

// IsBlacklisted — stub
func (s *RedisRefreshStore) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil // Sprint 6에서 구현
}

// InvalidateFamily — stub
func (s *RedisRefreshStore) InvalidateFamily(_ context.Context, _ string) error {
	return nil // Sprint 6에서 구현
}

// 인터페이스 구현 컴파일 타임 검증
var _ RefreshTokenStore = (*RedisRefreshStore)(nil)
