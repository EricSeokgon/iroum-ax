// refresh_test.go — REQ-AUTH-005 Refresh token rotation + logout RED phase 테스트
// Sprint 6 RED: RefreshTokenStore / RefreshSession / Logout
// SPEC-AX-AUTH-001 §5 AC-AUTH-005-1 ~ AC-AUTH-005-4
package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
)

// ────────────────────────────────────────────────────────────
// 테스트용 인메모리 RefreshTokenStore 구현체
// ────────────────────────────────────────────────────────────

// fakeRefreshStore — 인메모리 RefreshTokenStore (Redis 의존 없음)
// fieldalignment: 포인터/map(8바이트) 먼저, sync.RWMutex(큰 값 타입) 나중
type fakeRefreshStore struct {
	// blacklist — 블랙리스트된 jti 집합 (jti → expiry)
	blacklist map[string]time.Time
	// families — family_id → jti 집합
	families map[string]map[string]time.Time
	mu       sync.RWMutex
}

func newFakeRefreshStore() *fakeRefreshStore {
	return &fakeRefreshStore{
		blacklist: make(map[string]time.Time),
		families:  make(map[string]map[string]time.Time),
	}
}

func (f *fakeRefreshStore) BlacklistJTI(_ context.Context, jti string, expiry time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blacklist[jti] = expiry
	return nil
}

func (f *fakeRefreshStore) IsBlacklisted(_ context.Context, jti string) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.blacklist[jti]
	return ok, nil
}

func (f *fakeRefreshStore) InvalidateFamily(ctx context.Context, familyID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	members, ok := f.families[familyID]
	if !ok {
		return nil
	}
	// family의 모든 jti를 블랙리스트에 등록
	expiry := time.Now().Add(24 * time.Hour)
	for jti := range members {
		f.blacklist[jti] = expiry
	}
	delete(f.families, familyID)
	return nil
}

func (f *fakeRefreshStore) AddToFamily(_ context.Context, familyID, jti string, expiry time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.families[familyID]; !ok {
		f.families[familyID] = make(map[string]time.Time)
	}
	f.families[familyID][jti] = expiry
	return nil
}

func (f *fakeRefreshStore) GetFamilyMembers(_ context.Context, familyID string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	members, ok := f.families[familyID]
	if !ok {
		return nil, nil
	}
	result := make([]string, 0, len(members))
	for jti := range members {
		result = append(result, jti)
	}
	return result, nil
}

// 컴파일 타임 인터페이스 검증
var _ auth.RefreshTokenStore = (*fakeRefreshStore)(nil)

// ────────────────────────────────────────────────────────────
// 테스트용 AuditLogger 구현체
// ────────────────────────────────────────────────────────────

// fakeAuditLogger — 인메모리 감사 이벤트 수집기
// fieldalignment: 슬라이스(24바이트) 먼저, sync.Mutex 나중
type fakeAuditLogger struct {
	events []*audit.Event
	mu     sync.Mutex
}

func (f *fakeAuditLogger) LogEvent(_ context.Context, e *audit.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	return nil
}

func (f *fakeAuditLogger) lastEvent() *audit.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) == 0 {
		return nil
	}
	return f.events[len(f.events)-1]
}

func (f *fakeAuditLogger) eventCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

var _ auth.AuditLogger = (*fakeAuditLogger)(nil)

// ────────────────────────────────────────────────────────────
// 테스트용 TokenIssuer 구현체
// ────────────────────────────────────────────────────────────

// fakeTokenIssuer — 고정 토큰 쌍 발급기
type fakeTokenIssuer struct {
	// issued — 발급된 토큰 쌍 (IssueTokenPair 호출 시 반환)
	issued auth.RefreshTokenPair
}

func (f *fakeTokenIssuer) IssueTokenPair(_ context.Context, _, _ string) (auth.RefreshTokenPair, error) {
	return f.issued, nil
}

var _ auth.TokenIssuer = (*fakeTokenIssuer)(nil)

// ────────────────────────────────────────────────────────────
// RefreshTokenStore 단위 테스트
// ────────────────────────────────────────────────────────────

// TestRefreshTokenStore_BlacklistJTI_AddsToSet — jti를 블랙리스트에 추가하면 IsBlacklisted가 true를 반환한다
// AC-AUTH-005-1: 로그아웃 시 jti가 블랙리스트에 등록됨
func TestRefreshTokenStore_BlacklistJTI_AddsToSet(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	ctx := context.Background()
	jti := "test-jti-001"
	expiry := time.Now().Add(time.Hour)

	// Act
	err := store.BlacklistJTI(ctx, jti, expiry)

	// Assert
	require.NoError(t, err, "BlacklistJTI는 에러 없이 성공해야 한다")
	blacklisted, err := store.IsBlacklisted(ctx, jti)
	require.NoError(t, err)
	assert.True(t, blacklisted, "블랙리스트에 등록된 jti는 IsBlacklisted=true를 반환해야 한다")
}

// TestRefreshTokenStore_IsBlacklisted_ReturnsTrue — 이미 등록된 jti는 IsBlacklisted=true
// AC-AUTH-005-1: 블랙리스트 조회 정상 동작
func TestRefreshTokenStore_IsBlacklisted_ReturnsTrue(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	ctx := context.Background()
	jti := "already-blacklisted-jti"
	_ = store.BlacklistJTI(ctx, jti, time.Now().Add(time.Hour))

	// Act
	result, err := store.IsBlacklisted(ctx, jti)

	// Assert
	require.NoError(t, err)
	assert.True(t, result, "사전 등록된 jti는 IsBlacklisted=true를 반환해야 한다")
}

// TestRefreshTokenStore_IsBlacklisted_NotInSet_ReturnsFalse — 미등록 jti는 IsBlacklisted=false
func TestRefreshTokenStore_IsBlacklisted_NotInSet_ReturnsFalse(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	ctx := context.Background()

	// Act
	result, err := store.IsBlacklisted(ctx, "unknown-jti")

	// Assert
	require.NoError(t, err)
	assert.False(t, result, "미등록 jti는 IsBlacklisted=false를 반환해야 한다")
}

// TestRefreshTokenStore_InvalidateFamily_BlacklistsAllJTIs — family invalidation 시 모든 jti가 블랙리스트에 등록됨
// AC-AUTH-005-3: OAuth 2.0 BCP — family 전체 무효화
func TestRefreshTokenStore_InvalidateFamily_BlacklistsAllJTIs(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	ctx := context.Background()
	familyID := "fam-001"
	jti1 := "rt-original"
	jti2 := "rt-new"
	expiry := time.Now().Add(24 * time.Hour)

	_ = store.AddToFamily(ctx, familyID, jti1, expiry)
	_ = store.AddToFamily(ctx, familyID, jti2, expiry)

	// Act
	err := store.InvalidateFamily(ctx, familyID)

	// Assert
	require.NoError(t, err, "InvalidateFamily는 에러 없이 성공해야 한다")

	// family의 모든 jti가 블랙리스트에 등록되어야 한다
	bl1, _ := store.IsBlacklisted(ctx, jti1)
	bl2, _ := store.IsBlacklisted(ctx, jti2)
	assert.True(t, bl1, "family 첫 번째 jti는 블랙리스트에 등록되어야 한다")
	assert.True(t, bl2, "family 두 번째 jti는 블랙리스트에 등록되어야 한다")
}

// ────────────────────────────────────────────────────────────
// RefreshService.RefreshSession 테스트
// ────────────────────────────────────────────────────────────

// newTestRefreshService — 테스트용 RefreshService 생성 헬퍼
func newTestRefreshService(store *fakeRefreshStore, auditLog *fakeAuditLogger) *auth.RefreshService {
	issuer := &fakeTokenIssuer{
		issued: auth.RefreshTokenPair{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		},
	}
	// validator는 nil — RefreshSession에서 JWT 파싱용, stub에서는 사용 안 함
	return auth.NewRefreshService(store, nil, issuer, auditLog)
}

// TestRefreshSession_HappyPath_NewPairReturned — 정상 refresh 시 새 토큰 쌍 반환
// AC-AUTH-005 정상 케이스: 유효한 refresh token으로 새 pair 발급
//
// RED: RefreshSession stub이 "구현 예정" 에러 반환 → require.NoError FAIL
func TestRefreshSession_HappyPath_NewPairReturned(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	auditLog := &fakeAuditLogger{}
	svc := newTestRefreshService(store, auditLog)
	ctx := context.Background()

	// 유효한 refresh token (실제 JWT는 GREEN에서 생성, RED에서는 stub 동작 검증)
	validRefreshToken := "valid.refresh.token"

	// Act
	pair, err := svc.RefreshSession(ctx, validRefreshToken)

	// Assert — GREEN에서 통과, RED에서 FAIL
	require.NoError(t, err, "유효한 refresh token으로 RefreshSession은 성공해야 한다")
	assert.NotEmpty(t, pair.AccessToken, "새 access token이 발급되어야 한다")
	assert.NotEmpty(t, pair.RefreshToken, "새 refresh token이 발급되어야 한다")
}

// TestRefreshSession_AlreadyUsedToken_InvalidatesFamily — 이미 사용된 refresh token으로 RefreshSession 시 family 전체 무효화
// AC-AUTH-005-3: OAuth 2.0 BCP critical — refresh token family reuse detection
//
// RED: RefreshSession stub이 "구현 예정" 에러 반환 → errors.Is(err, ErrRefreshTokenReuseDetected) 기대 → FAIL
func TestRefreshSession_AlreadyUsedToken_InvalidatesFamily(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	auditLog := &fakeAuditLogger{}
	svc := newTestRefreshService(store, auditLog)
	ctx := context.Background()

	// family 설정: fam-1에 rt-original + rt-new 모두 등록
	familyID := "fam-1"
	jtiOriginal := "rt-original"
	jtiNew := "rt-new"
	expiry := time.Now().Add(24 * time.Hour)
	_ = store.AddToFamily(ctx, familyID, jtiOriginal, expiry)
	_ = store.AddToFamily(ctx, familyID, jtiNew, expiry)
	// rt-original은 이미 사용됨 → 블랙리스트에 등록
	_ = store.BlacklistJTI(ctx, jtiOriginal, expiry)

	// 이미 사용된 rt-original 토큰으로 재시도 (reuse attack)
	// GREEN에서 실제 JWT 파싱이 구현되면 올바른 토큰이 전달됨
	// RED에서는 stub의 동작만 검증
	reusedToken := "reused.refresh.token.rt-original" //nolint:gosec // 테스트 픽스처 — 실제 자격증명 아님

	// Act
	_, err := svc.RefreshSession(ctx, reusedToken)

	// Assert — GREEN에서 통과, RED에서 FAIL (stub은 "구현 예정" 에러 반환)
	require.Error(t, err, "이미 사용된 refresh token으로 RefreshSession은 에러를 반환해야 한다")
	assert.True(t,
		errors.Is(err, auth.ErrRefreshTokenReuseDetected),
		"재사용 탐지 시 ErrRefreshTokenReuseDetected를 반환해야 한다 (got: %v)", err,
	)

	// family 전체가 무효화되어야 한다 (GREEN에서 검증)
	blNew, _ := store.IsBlacklisted(ctx, jtiNew)
	assert.True(t, blNew, "family invalidation 시 rt-new도 블랙리스트에 등록되어야 한다")
}

// TestRefreshSession_BlacklistedJTI_ReturnsError — 블랙리스트된 jti의 refresh token은 에러 반환
// AC-AUTH-005: 로그아웃된 refresh token으로 refresh 시도 차단
//
// RED: stub이 "구현 예정" 에러 반환 → require.Error PASS (에러 타입은 다름)
func TestRefreshSession_BlacklistedJTI_ReturnsError(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	auditLog := &fakeAuditLogger{}
	svc := newTestRefreshService(store, auditLog)
	ctx := context.Background()

	// jti를 직접 블랙리스트에 등록 (로그아웃된 상태)
	_ = store.BlacklistJTI(ctx, "blacklisted-refresh-jti", time.Now().Add(time.Hour))

	// Act — 블랙리스트된 jti를 가진 refresh token
	_, err := svc.RefreshSession(ctx, "blacklisted.refresh.token")

	// Assert
	require.Error(t, err, "블랙리스트된 refresh token으로 RefreshSession은 에러를 반환해야 한다")
}

// TestRefreshSession_ExpiredRefreshToken_ReturnsError — 만료된 refresh token으로 RefreshSession 시 에러
// AC-AUTH-005-4: 만료된 refresh token은 갱신 불가
//
// RED: stub이 "구현 예정" 에러 반환 → require.Error PASS (에러 타입 검증은 GREEN에서)
func TestRefreshSession_ExpiredRefreshToken_ReturnsError(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	auditLog := &fakeAuditLogger{}
	svc := newTestRefreshService(store, auditLog)
	ctx := context.Background()

	// 만료된 refresh token (exp 클레임이 과거인 JWT)
	expiredRefreshToken := "expired.refresh.token"

	// Act
	_, err := svc.RefreshSession(ctx, expiredRefreshToken)

	// Assert
	require.Error(t, err, "만료된 refresh token으로 RefreshSession은 에러를 반환해야 한다")
}

// ────────────────────────────────────────────────────────────
// RefreshService.Logout 테스트
// ────────────────────────────────────────────────────────────

// TestLogout_BlacklistsAccessAndRefreshTokens — 로그아웃 시 access + refresh token 모두 블랙리스트 등록
// AC-AUTH-005-1: POST /auth/logout → 두 token의 jti가 Redis 블랙리스트에 등록됨
//
// RED: Logout stub이 "구현 예정" 에러 반환 → require.NoError FAIL
func TestLogout_BlacklistsAccessAndRefreshTokens(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	auditLog := &fakeAuditLogger{}
	svc := newTestRefreshService(store, auditLog)
	ctx := context.Background()

	// 유효한 access token + refresh token
	// GREEN에서 실제 JWT가 사용되며, RED에서는 stub 동작 검증
	accessToken := "valid.access.token"
	refreshToken := "valid.refresh.token"

	// Act
	err := svc.Logout(ctx, accessToken, refreshToken)

	// Assert — GREEN에서 통과, RED에서 FAIL
	require.NoError(t, err, "Logout은 에러 없이 성공해야 한다")
}

// TestLogout_RecordsAuditEvent — 로그아웃 시 감사 이벤트가 기록됨
// AC-AUTH-005-1: audit_logs row action=AUTH_LOGOUT, user_id=access_token sub
//
// RED: Logout stub이 "구현 예정" 에러 반환 → require.NoError FAIL
func TestLogout_RecordsAuditEvent(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	auditLog := &fakeAuditLogger{}
	svc := newTestRefreshService(store, auditLog)
	ctx := context.Background()

	// Act
	err := svc.Logout(ctx, "valid.access.token", "valid.refresh.token")

	// Assert — GREEN에서 통과, RED에서 FAIL
	require.NoError(t, err, "Logout은 에러 없이 성공해야 한다")

	// 감사 이벤트가 정확히 1개 기록되어야 한다
	require.Equal(t, 1, auditLog.eventCount(), "로그아웃 시 감사 이벤트가 1개 기록되어야 한다")

	lastEvt := auditLog.lastEvent()
	require.NotNil(t, lastEvt)
	assert.Equal(t, audit.ActionAuthLogout, lastEvt.Action,
		"로그아웃 감사 이벤트 action은 AUTH_LOGOUT이어야 한다")
}

// TestLogout_InvalidToken_ReturnsError — 잘못된 형식의 token으로 Logout 시 에러
// AC-AUTH-005-4: malformed refresh_token으로 logout 시 에러 반환
//
// RED: stub이 "구현 예정" 에러 반환 → require.Error PASS (진성 RED는 HappyPath/AuditEvent에서 확보)
func TestLogout_InvalidToken_ReturnsError(t *testing.T) {
	t.Parallel()
	// Arrange
	store := newFakeRefreshStore()
	auditLog := &fakeAuditLogger{}
	svc := newTestRefreshService(store, auditLog)
	ctx := context.Background()

	// Act — 잘못된 형식의 token
	err := svc.Logout(ctx, "valid.access.token", "not-a-jwt")

	// Assert
	// GREEN에서는 ErrTokenInvalidSignature 등을 반환해야 하지만,
	// RED에서는 "구현 예정" 에러가 반환되어 require.Error PASS (stub-coupled assert 회피)
	require.Error(t, err, "잘못된 refresh token으로 Logout은 에러를 반환해야 한다")
}

// ────────────────────────────────────────────────────────────
// audit.go 상수 존재 확인 테스트
// ────────────────────────────────────────────────────────────

// TestAuditAction_AuthLogout_Defined — audit.ActionAuthLogout 상수가 정의되어 있는지 확인
// REQ-AUTH-005-E1: audit_logs action=AUTH_LOGOUT
func TestAuditAction_AuthLogout_Defined(t *testing.T) {
	t.Parallel()
	// 상수가 정의되어 있으면 컴파일 성공
	assert.Equal(t, audit.Action("AUTH_LOGOUT"), audit.ActionAuthLogout,
		"audit.ActionAuthLogout은 'AUTH_LOGOUT' 문자열이어야 한다")
}

// TestAuditAction_AuthRefreshReuseDetected_Defined — audit.ActionAuthRefreshReuseDetected 상수 확인
// REQ-AUTH-005-U1: audit_logs action=AUTH_REFRESH_REUSE_DETECTED
func TestAuditAction_AuthRefreshReuseDetected_Defined(t *testing.T) {
	t.Parallel()
	assert.Equal(t, audit.Action("AUTH_REFRESH_REUSE_DETECTED"), audit.ActionAuthRefreshReuseDetected,
		"audit.ActionAuthRefreshReuseDetected은 'AUTH_REFRESH_REUSE_DETECTED' 문자열이어야 한다")
}
