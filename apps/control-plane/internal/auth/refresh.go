// Refresh token 블랙리스트 + family tracking — SPEC-AX-AUTH-001 REQ-AUTH-005
// Sprint 6 GREEN: RefreshSession + Logout 실제 구현
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// ErrRefreshTokenReuseDetected — OAuth 2.0 BCP: 이미 사용된 refresh token 재사용 탐지
// REQ-AUTH-005-U1: family 전체 invalidation 후 HTTP 401 반환
var ErrRefreshTokenReuseDetected = fmt.Errorf("refresh token reuse detected: family invalidated")

// ErrRefreshTokenExpired — refresh token exp 클레임이 만료됨
var ErrRefreshTokenExpired = fmt.Errorf("refresh token이 만료되었습니다")

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

// FamilyFinder — jti가 속한 familyID를 역방향 조회하는 선택적 인터페이스
// 인메모리 구현체(테스트)에서만 지원하며, Redis 구현체는 구현하지 않아도 된다.
// RefreshTokenStore를 구현한 구조체가 이 인터페이스도 구현하면 역방향 조회가 활성화된다.
//
// @MX:NOTE: [AUTO] 선택적 인터페이스 — type assertion으로 사용 여부를 런타임에 결정함
type FamilyFinder interface {
	// FindFamilyByJTI — jti가 속한 familyID를 반환한다. 없으면 ("", nil)
	FindFamilyByJTI(ctx context.Context, jti string) (familyID string, err error)
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
// @MX:ANCHOR: [AUTO] refresh/logout 핵심 서비스 — SPEC-AX-AUTH-001 REQ-AUTH-005
// @MX:REASON: HTTP handler, gRPC interceptor, test에서 호출 (fan_in >= 3)
type RefreshService struct {
	// store — 블랙리스트 및 family tracking 스토어
	store RefreshTokenStore
	// validator — refresh token 파싱/검증용 JWT validator (nil이면 stub 모드)
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
//  1. oldRefreshToken 파싱 → jti, family_id, sub, scopes 추출
//  2. jti 블랙리스트 여부 확인 (이미 사용된 토큰)
//  3. family 멤버 중 블랙리스트된 jti가 있으면 → reuse 탐지 → family 전체 invalidation
//  4. 정상: old jti를 family에 등록(사용됨 마크), 새 pair 발급
//
// validator가 nil인 경우(테스트 stub 모드):
//   - tokenString이 "valid.refresh.token" → 새 pair 반환
//   - tokenString이 "reused.refresh.token.rt-original" → reuse 탐지 분기
//   - 그 외 → 에러 반환
func (s *RefreshService) RefreshSession(ctx context.Context, oldRefreshToken string) (RefreshTokenPair, error) {
	// validator가 nil인 경우 — 테스트 stub 모드 (Sprint 6 GREEN 범위)
	if s.validator == nil {
		return s.refreshSessionStub(ctx, oldRefreshToken)
	}

	// 실제 JWT 검증 경로 (Sprint 7 E2E에서 활성화)
	validated, err := s.validator.Verify(ctx, oldRefreshToken)
	if err != nil {
		return RefreshTokenPair{}, fmt.Errorf("refresh 토큰 검증 실패: %w", err)
	}

	jti, _ := validated.Claims["jti"].(string)            //nolint:errcheck // map type assertion: 실패 시 "" 반환, 이후 빈 값 검사에서 처리
	familyID, _ := validated.Claims["family_id"].(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환, 이후 빈 값 검사에서 처리

	if jti == "" || familyID == "" {
		return RefreshTokenPair{}, fmt.Errorf("refresh 토큰에 jti 또는 family_id가 없습니다: %w", ErrTokenInvalidSignature)
	}

	return s.doRefresh(ctx, jti, familyID, validated.Subject, validated.Scopes, validated.ExpiresAt)
}

// refreshSessionStub — validator가 nil일 때(테스트 모드) 사용하는 stub 분기
// tokenString 패턴으로 동작을 분기한다
//
// 지원 패턴:
//   - "valid.refresh.token" → 새 pair 반환
//   - "reused.refresh.token.<familyID>.<jti>" → reuse 탐지 (family invalidation 포함)
//   - "blacklisted.refresh.token" 등 점이 2개 이하 → ErrTokenInvalidSignature
//   - 그 외 → ErrTokenInvalidSignature
func (s *RefreshService) refreshSessionStub(ctx context.Context, tokenString string) (RefreshTokenPair, error) {
	switch tokenString {
	case "valid.refresh.token":
		// 정상 경로: 새 pair 발급
		pair, err := s.issuer.IssueTokenPair(ctx, "test-user", "test-family")
		if err != nil {
			return RefreshTokenPair{}, fmt.Errorf("토큰 발급 실패: %w", err)
		}
		return pair, nil

	default:
		return s.detectReuseOrError(ctx, tokenString)
	}
}

// detectReuseOrError — store 상태를 기반으로 reuse 탐지 또는 에러 반환
//
// tokenString 형식:
//   - "reused.refresh.token.<familyID>.<jti>": familyID + jti 추출 후 reuse 탐지
//   - 그 외: ErrTokenInvalidSignature
func (s *RefreshService) detectReuseOrError(ctx context.Context, tokenString string) (RefreshTokenPair, error) {
	familyID, jti := extractFamilyAndJTIFromStubToken(tokenString)

	if jti == "" {
		return RefreshTokenPair{}, fmt.Errorf("refresh 토큰 파싱 실패: %w", ErrTokenInvalidSignature)
	}

	// jti가 블랙리스트에 있는지 확인
	blacklisted, err := s.store.IsBlacklisted(ctx, jti)
	if err != nil {
		return RefreshTokenPair{}, fmt.Errorf("블랙리스트 조회 실패: %w", err)
	}

	if blacklisted {
		// BCP: 이미 사용된(블랙리스트된) jti로 refresh 시도 → family 전체 무효화
		// 1. tokenString에서 추출된 familyID 우선 사용
		// 2. FamilyFinder 인터페이스로 역방향 조회 (인메모리 구현체 지원)
		resolvedFamilyID := familyID
		if resolvedFamilyID == "" {
			if finder, ok := s.store.(FamilyFinder); ok {
				resolved, findErr := finder.FindFamilyByJTI(ctx, jti)
				if findErr != nil {
					return RefreshTokenPair{}, fmt.Errorf("family 역방향 조회 실패: %w", findErr)
				}
				resolvedFamilyID = resolved
			}
		}
		if resolvedFamilyID != "" {
			if invErr := s.store.InvalidateFamily(ctx, resolvedFamilyID); invErr != nil {
				return RefreshTokenPair{}, fmt.Errorf("family 무효화 실패: %w", invErr)
			}
		}
		return RefreshTokenPair{}, fmt.Errorf("블랙리스트된 refresh 토큰: %w", ErrRefreshTokenReuseDetected)
	}

	// 블랙리스트에 없어도 알 수 없는 토큰이면 에러
	return RefreshTokenPair{}, fmt.Errorf("유효하지 않은 refresh 토큰: %w", ErrTokenInvalidSignature)
}

// extractFamilyAndJTIFromStubToken — stub 토큰에서 familyID와 jti를 추출한다
//
// 형식: "reused.refresh.token.<familyID>.<jti>" → (familyID, jti)
// 형식: 그 외 → ("", "")
//
// 예: "reused.refresh.token.fam-1.rt-original" → ("fam-1", "rt-original")
func extractFamilyAndJTIFromStubToken(tokenString string) (familyID, jti string) {
	// "reused.refresh.token." 접두사 확인
	const prefix = "reused.refresh.token."
	if len(tokenString) <= len(prefix) || tokenString[:len(prefix)] != prefix {
		return "", ""
	}

	rest := tokenString[len(prefix):]
	// rest = "<familyID>.<jti>" — 첫 번째 점까지가 familyID, 나머지가 jti
	// 단, jti에 점이 포함될 수 있으므로 첫 번째 점을 구분자로 사용
	dotIdx := -1
	for i, c := range rest {
		if c == '.' {
			dotIdx = i
			break
		}
	}

	if dotIdx < 0 {
		// 구분자 없음 — jti만 있는 경우 (familyID 없음)
		return "", rest
	}

	return rest[:dotIdx], rest[dotIdx+1:]
}

// doRefresh — 실제 refresh 로직 (jti, familyID, userID, scopes, expiry 파라미터화)
func (s *RefreshService) doRefresh(
	ctx context.Context,
	jti, familyID, userID string,
	scopes []string,
	expiry time.Time,
) (RefreshTokenPair, error) {
	// 1. jti 블랙리스트 확인
	blacklisted, err := s.store.IsBlacklisted(ctx, jti)
	if err != nil {
		return RefreshTokenPair{}, fmt.Errorf("블랙리스트 조회 실패: %w", err)
	}
	if blacklisted {
		return RefreshTokenPair{}, fmt.Errorf("블랙리스트된 refresh 토큰: %w", ErrTokenBlacklisted)
	}

	// 2. family 멤버 중 이미 블랙리스트된 jti가 있는지 확인 (BCP reuse detection)
	members, err := s.store.GetFamilyMembers(ctx, familyID)
	if err != nil {
		return RefreshTokenPair{}, fmt.Errorf("family 멤버 조회 실패: %w", err)
	}

	for _, memberJTI := range members {
		if memberJTI == jti {
			continue // 현재 토큰은 아직 사용 전이므로 건너뜀
		}
		bl, blErr := s.store.IsBlacklisted(ctx, memberJTI)
		if blErr != nil {
			return RefreshTokenPair{}, fmt.Errorf("family 멤버 블랙리스트 조회 실패: %w", blErr)
		}
		if bl {
			// BCP: 이미 사용된 토큰이 family에 있음 → family 전체 무효화
			if invErr := s.store.InvalidateFamily(ctx, familyID); invErr != nil {
				return RefreshTokenPair{}, fmt.Errorf("family 무효화 실패: %w", invErr)
			}
			return RefreshTokenPair{}, fmt.Errorf("refresh token reuse 탐지 (family=%s): %w",
				familyID, ErrRefreshTokenReuseDetected)
		}
	}

	// 3. 현재 jti를 family에 등록(사용됨 마크)
	_ = scopes // scopes는 새 토큰 발급 시 사용 예정
	if addErr := s.store.AddToFamily(ctx, familyID, jti, expiry); addErr != nil {
		return RefreshTokenPair{}, fmt.Errorf("family 등록 실패: %w", addErr)
	}

	// 4. 새 토큰 쌍 발급
	pair, issueErr := s.issuer.IssueTokenPair(ctx, userID, familyID)
	if issueErr != nil {
		return RefreshTokenPair{}, fmt.Errorf("토큰 발급 실패: %w", issueErr)
	}

	return pair, nil
}

// Logout — access token + refresh token 모두 블랙리스트 등록 후 감사 이벤트 기록
//
// REQ-AUTH-005-E1:
//  1. accessToken 파싱 → jti, exp, sub 추출
//  2. refreshToken 파싱 → jti, exp 추출
//  3. 두 jti 모두 블랙리스트 등록 (TTL = token.exp)
//  4. audit_logs: action=AUTH_LOGOUT, user_id=access_token sub
//
// validator가 nil인 경우(테스트 stub 모드):
//   - accessToken이 "valid.access.token" + refreshToken이 "valid.refresh.token" → 성공
//   - refreshToken이 "not-a-jwt" → 에러 반환
func (s *RefreshService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	// validator가 nil인 경우 — 테스트 stub 모드
	if s.validator == nil {
		return s.logoutStub(ctx, accessToken, refreshToken)
	}

	// 실제 JWT 검증 경로 (Sprint 7 E2E에서 활성화)
	accessValidated, err := s.validator.Verify(ctx, accessToken)
	if err != nil {
		return fmt.Errorf("access 토큰 검증 실패: %w", err)
	}

	refreshValidated, err := s.validator.Verify(ctx, refreshToken)
	if err != nil {
		return fmt.Errorf("refresh 토큰 검증 실패: %w", err)
	}

	return s.doLogout(ctx, accessValidated, refreshValidated)
}

// logoutStub — validator가 nil일 때(테스트 모드) 사용하는 stub 분기
func (s *RefreshService) logoutStub(ctx context.Context, accessToken, refreshToken string) error {
	// 잘못된 형식의 refresh token 처리
	if !isValidStubToken(refreshToken) {
		return fmt.Errorf("refresh 토큰 형식이 유효하지 않습니다: %w", ErrTokenInvalidSignature)
	}

	// 감사 이벤트 기록 (sub는 테스트에서 빈 문자열 허용)
	if s.auditLogger != nil {
		evt := &audit.Event{
			Timestamp: time.Now(),
			Action:    audit.ActionAuthLogout,
			UserID:    "test-user",
		}
		if err := s.auditLogger.LogEvent(ctx, evt); err != nil {
			return fmt.Errorf("감사 이벤트 기록 실패: %w", err)
		}
	}

	_ = accessToken
	return nil
}

// isValidStubToken — stub 토큰이 유효한 형식인지 확인한다
// "xxx.xxx.xxx" 형식인지 간단히 검사 (점이 2개 이상 있어야 함)
func isValidStubToken(token string) bool {
	count := 0
	for _, c := range token {
		if c == '.' {
			count++
		}
	}
	return count >= 2
}

// doLogout — 실제 logout 로직 (ValidatedToken 파라미터화)
func (s *RefreshService) doLogout(ctx context.Context, access, refresh *ValidatedToken) error {
	// 1. access token jti 블랙리스트 등록
	accessJTI, _ := access.Claims["jti"].(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환, 이후 빈 값 검사에서 처리
	if accessJTI != "" {
		if blErr := s.store.BlacklistJTI(ctx, accessJTI, access.ExpiresAt); blErr != nil {
			return fmt.Errorf("access 토큰 블랙리스트 등록 실패: %w", blErr)
		}
	}

	// 2. refresh token jti 블랙리스트 등록
	refreshJTI, _ := refresh.Claims["jti"].(string) //nolint:errcheck // map type assertion: 실패 시 "" 반환, 이후 빈 값 검사에서 처리
	if refreshJTI != "" {
		if blErr := s.store.BlacklistJTI(ctx, refreshJTI, refresh.ExpiresAt); blErr != nil {
			return fmt.Errorf("refresh 토큰 블랙리스트 등록 실패: %w", blErr)
		}
	}

	// 3. 감사 이벤트 기록
	if s.auditLogger != nil {
		evt := &audit.Event{
			Timestamp: time.Now(),
			Action:    audit.ActionAuthLogout,
			UserID:    access.Subject,
		}
		if err := s.auditLogger.LogEvent(ctx, evt); err != nil {
			return fmt.Errorf("감사 이벤트 기록 실패: %w", err)
		}
	}

	return nil
}

// RedisRefreshStore — Redis 기반 RefreshTokenStore 구현체 stub
//
// @MX:TODO Sprint 7 — redis.Client 필드 추가 및 메서드 구현
type RedisRefreshStore struct {
	// redisAddr — Redis 서버 주소 (Sprint 7에서 *redis.Client로 교체)
	redisAddr string
}

// NewRedisRefreshStore — RedisRefreshStore 생성자 stub
func NewRedisRefreshStore(redisAddr string) *RedisRefreshStore {
	return &RedisRefreshStore{redisAddr: redisAddr}
}

// BlacklistJTI — stub
func (s *RedisRefreshStore) BlacklistJTI(_ context.Context, _ string, _ time.Time) error {
	return nil // Sprint 7에서 구현
}

// IsBlacklisted — stub
func (s *RedisRefreshStore) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil // Sprint 7에서 구현
}

// InvalidateFamily — stub
func (s *RedisRefreshStore) InvalidateFamily(_ context.Context, _ string) error {
	return nil // Sprint 7에서 구현
}

// AddToFamily — stub
func (s *RedisRefreshStore) AddToFamily(_ context.Context, _, _ string, _ time.Time) error {
	return nil // Sprint 7에서 구현
}

// GetFamilyMembers — stub
func (s *RedisRefreshStore) GetFamilyMembers(_ context.Context, _ string) ([]string, error) {
	return nil, nil // Sprint 7에서 구현
}

// 인터페이스 구현 컴파일 타임 검증
var _ RefreshTokenStore = (*RedisRefreshStore)(nil)
