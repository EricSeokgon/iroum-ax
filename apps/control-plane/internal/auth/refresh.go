// Refresh token 블랙리스트 + family tracking stub — SPEC-AX-AUTH-001 REQ-AUTH-005
// Sprint 6 GREEN에서 실제 구현 예정
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// ErrRefreshTokenReuseDetected — OAuth 2.0 BCP: 이미 사용된 refresh token 재사용 탐지
// REQ-AUTH-005-U1: family 전체 invalidation 후 HTTP 401 반환
var ErrRefreshTokenReuseDetected = errors.New("refresh token reuse detected: family invalidated")

// ErrRefreshTokenExpired — refresh token exp 클레임이 만료됨
var ErrRefreshTokenExpired = errors.New("refresh token이 만료되었습니다")

// RefreshTokenStore — refresh token 블랙리스트 및 family tracking 인터페이스
//
// @MX:ANCHOR: [AUTO] refresh/logout 블랙리스트 단일 진입점
// @MX:REASON: RefreshSession, Logout, BlacklistChecker에서 호출 (fan_in >= 3)
type RefreshTokenStore interface {
	// BlacklistJTI — jti를 블랙리스트에 등록한다 (TTL = expiry - now)
	// Redis key: auth:blacklist:<jti>
	BlacklistJTI(ctx context.Context, jti string, expiry time.Time) error

	// IsBlacklisted — jti가 블랙리스트에 있는지 확인한다
	IsBlacklisted(ctx context.Context, jti string) (bool, error)

	// InvalidateFamily — refresh token family 전체를 무효화한다
	// Redis key: auth:refresh_family:<familyID>
	// Lua script atomic check-and-blacklist 사용 (REQ-AUTH-005-U1)
	//
	// @MX:WARN: [AUTO] Lua script atomic 조건문 복잡도 예상, fallback 전략 필수
	// @MX:REASON: Redis race condition mitigation — Get family + Validate jti + Blacklist family를 atomic 수행, eval 실패 시 보수적 family 전체 blacklist
	InvalidateFamily(ctx context.Context, familyID string) error

	// AddToFamily — refresh token jti를 family에 추가한다
	// Redis key: auth:refresh_family:<familyID> (SADD)
	AddToFamily(ctx context.Context, familyID, jti string, expiry time.Time) error

	// GetFamilyMembers — family에 속한 모든 jti를 반환한다 (SMEMBERS)
	GetFamilyMembers(ctx context.Context, familyID string) ([]string, error)
}

// RefreshTokenPair — 새로 발급된 access/refresh token 쌍
// fieldalignment: string 필드 순서 — 알파벳 정렬
type RefreshTokenPair struct {
	// AccessToken — 새 access token (JWT)
	AccessToken string
	// RefreshToken — 새 refresh token (JWT)
	RefreshToken string
}

// TokenIssuer — 새 access/refresh token 쌍 발급 인터페이스 (테스트 주입용)
//
// @MX:TODO Sprint 6 GREEN — JWTIssuer 구현체 연동
type TokenIssuer interface {
	// IssueTokenPair — userID, familyID로 새 access/refresh token 쌍 발급
	IssueTokenPair(ctx context.Context, userID, familyID string) (RefreshTokenPair, error)
}

// AuditLogger — 감사 이벤트 기록 인터페이스 (refresh/logout 전용)
// audit.AuditTx 대신 간단한 직접 기록 인터페이스 사용
type AuditLogger interface {
	// LogEvent — 감사 이벤트를 기록한다
	LogEvent(ctx context.Context, e *audit.Event) error
}

// RefreshService — refresh token rotation + logout 서비스
//
// @MX:TODO Sprint 6 GREEN — Redis Lua script 기반 atomic family invalidation 구현
type RefreshService struct {
	// store — 블랙리스트 및 family tracking 스토어
	store RefreshTokenStore
	// validator — refresh token 파싱/검증용 JWT validator
	validator *TokenValidator
	// issuer — 새 token 쌍 발급기
	issuer TokenIssuer
	// auditLogger — 감사 이벤트 기록기
	auditLogger AuditLogger
}

// NewRefreshService — RefreshService 생성자
func NewRefreshService(
	store RefreshTokenStore,
	validator *TokenValidator,
	issuer TokenIssuer,
	auditLogger AuditLogger,
) *RefreshService {
	return &RefreshService{
		store:       store,
		validator:   validator,
		issuer:      issuer,
		auditLogger: auditLogger,
	}
}

// RefreshSession — refresh token rotation (OAuth 2.0 BCP family invalidation 포함)
//
// REQ-AUTH-005-U1:
// 1. oldRefreshToken 파싱 → jti, family_id, exp 추출
// 2. jti 블랙리스트 여부 확인 (이미 사용된 토큰)
// 3. family에 이미 사용된 jti가 있으면 → family 전체 invalidation + ErrRefreshTokenReuseDetected
// 4. 정상: old jti를 사용됨으로 마크, 새 pair 발급
//
// @MX:TODO Sprint 6 GREEN — 실제 구현 예정
func (s *RefreshService) RefreshSession(ctx context.Context, oldRefreshToken string) (RefreshTokenPair, error) {
	_ = ctx
	_ = oldRefreshToken
	// Sprint 6 GREEN에서 구현
	return RefreshTokenPair{}, errors.New("구현 예정: Sprint 6 GREEN")
}

// Logout — access token + refresh token 모두 블랙리스트 등록
//
// REQ-AUTH-005-E1:
// 1. accessToken 파싱 → jti, exp, sub 추출
// 2. refreshToken 파싱 → jti, exp 추출
// 3. 두 jti 모두 블랙리스트 등록 (TTL = token.exp)
// 4. audit_logs: action=AUTH_LOGOUT, user_id=access_token sub
//
// @MX:TODO Sprint 6 GREEN — 실제 구현 예정
func (s *RefreshService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	_ = ctx
	_ = accessToken
	_ = refreshToken
	// Sprint 6 GREEN에서 구현
	return errors.New("구현 예정: Sprint 6 GREEN")
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
func (s *RedisRefreshStore) BlacklistJTI(_ context.Context, _ string, _ time.Time) error {
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

// AddToFamily — stub
func (s *RedisRefreshStore) AddToFamily(_ context.Context, _, _ string, _ time.Time) error {
	return nil // Sprint 6에서 구현
}

// GetFamilyMembers — stub
func (s *RedisRefreshStore) GetFamilyMembers(_ context.Context, _ string) ([]string, error) {
	return nil, nil // Sprint 6에서 구현
}

// 인터페이스 구현 컴파일 타임 검증
var _ RefreshTokenStore = (*RedisRefreshStore)(nil)
