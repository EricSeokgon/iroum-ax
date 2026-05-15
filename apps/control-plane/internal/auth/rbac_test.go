// rbac_test.go — REQ-AUTH-004 RBAC 3-role 매트릭스 RED phase 테스트
// Sprint 5 RED: ParseRolesFromScope / EffectivePermissions / Authorize / LogForbidden
// SPEC-AX-AUTH-001 §4 AC-AUTH-004-1 ~ AC-AUTH-004-6
package auth_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// ────────────────────────────────────────────────────────────
// ParseRolesFromScope 테스트
// ────────────────────────────────────────────────────────────

// TestParseRolesFromScope_SingleRole — "iroum-ax:admin" → [RoleAdmin]
// AC-AUTH-004-3: admin scope는 RoleAdmin으로 파싱됨
func TestParseRolesFromScope_SingleRole(t *testing.T) {
	t.Parallel()
	// Arrange
	scope := "iroum-ax:admin"

	// Act
	roles := auth.ParseRolesFromScope(scope)

	// Assert
	require.Len(t, roles, 1, "단일 admin scope는 정확히 1개의 역할을 반환해야 한다")
	assert.Equal(t, auth.RoleAdmin, roles[0])
}

// TestParseRolesFromScope_MultipleRoles — "iroum-ax:analyst iroum-ax:viewer" → [RoleAnalyst, RoleViewer]
// AC-AUTH-004-4: 공백 구분 다중 역할은 union 집합으로 파싱됨
func TestParseRolesFromScope_MultipleRoles(t *testing.T) {
	t.Parallel()
	// Arrange
	scope := "iroum-ax:analyst iroum-ax:viewer"

	// Act
	roles := auth.ParseRolesFromScope(scope)

	// Assert
	require.Len(t, roles, 2, "두 개의 iroum-ax 역할 scope는 2개의 역할을 반환해야 한다")
	assert.Contains(t, roles, auth.RoleAnalyst)
	assert.Contains(t, roles, auth.RoleViewer)
}

// TestParseRolesFromScope_InvalidRoleIgnored — "iroum-ax:superadmin foo" → []
// AC-AUTH-004-5: allow-list(admin/analyst/viewer) 외 scope는 무시됨
func TestParseRolesFromScope_InvalidRoleIgnored(t *testing.T) {
	t.Parallel()
	// Arrange — 허용 목록에 없는 역할
	scope := "iroum-ax:superadmin foo"

	// Act
	roles := auth.ParseRolesFromScope(scope)

	// Assert — 미인식 역할은 silently drop
	assert.Empty(t, roles, "허용 목록 외 scope는 파싱 결과에서 제외되어야 한다")
}

// TestParseRolesFromScope_EmptyString — 빈 scope → 빈 슬라이스
func TestParseRolesFromScope_EmptyString(t *testing.T) {
	t.Parallel()
	roles := auth.ParseRolesFromScope("")
	assert.Empty(t, roles, "빈 scope 문자열은 빈 슬라이스를 반환해야 한다")
}

// TestParseRolesFromScope_OtherScopes — OIDC 표준 scope와 iroum-ax role 혼재
// "offline_access openid iroum-ax:admin" → [RoleAdmin]
func TestParseRolesFromScope_OtherScopes(t *testing.T) {
	t.Parallel()
	// Arrange — Keycloak이 발급하는 실제 scope 형식 시뮬레이션
	scope := "offline_access openid iroum-ax:admin"

	// Act
	roles := auth.ParseRolesFromScope(scope)

	// Assert — iroum-ax: 접두사가 없는 표준 OIDC scope는 제외
	require.Len(t, roles, 1)
	assert.Equal(t, auth.RoleAdmin, roles[0])
}

// ────────────────────────────────────────────────────────────
// EffectivePermissions 테스트
// ────────────────────────────────────────────────────────────

// TestEffectivePermissions_Admin — admin 역할은 모든 권한을 가짐
// AC-AUTH-004-3: admin role — all methods allowed
func TestEffectivePermissions_Admin(t *testing.T) {
	t.Parallel()
	// Arrange
	roles := []auth.Role{auth.RoleAdmin}

	// Act
	perms := auth.EffectivePermissions(roles)

	// Assert — admin은 최소한 핵심 권한들을 모두 보유해야 함
	require.NotNil(t, perms, "EffectivePermissions는 nil 맵을 반환하면 안 된다")
	assert.True(t, perms["read:workflow"], "admin은 read:workflow 권한을 가져야 한다")
	assert.True(t, perms["write:workflow"], "admin은 write:workflow 권한을 가져야 한다")
	assert.True(t, perms["delete:workflow"], "admin은 delete:workflow 권한을 가져야 한다")
	assert.True(t, perms["audit:read"], "admin은 audit:read 권한을 가져야 한다")
	assert.True(t, perms["read:recommendation"], "admin은 read:recommendation 권한을 가져야 한다")
	assert.True(t, perms["write:recommendation"], "admin은 write:recommendation 권한을 가져야 한다")
}

// TestEffectivePermissions_Analyst — analyst 역할 권한 검증
// AC-AUTH-004-1: analyst는 workflow CRUD + recommendation 허용
func TestEffectivePermissions_Analyst(t *testing.T) {
	t.Parallel()
	roles := []auth.Role{auth.RoleAnalyst}

	perms := auth.EffectivePermissions(roles)

	require.NotNil(t, perms)
	assert.True(t, perms["read:workflow"], "analyst는 read:workflow 권한을 가져야 한다")
	assert.True(t, perms["write:workflow"], "analyst는 write:workflow 권한을 가져야 한다")
	assert.True(t, perms["read:recommendation"], "analyst는 read:recommendation 권한을 가져야 한다")
	assert.True(t, perms["write:recommendation"], "analyst는 write:recommendation 권한을 가져야 한다")
	assert.True(t, perms["read:audit"], "analyst는 read:audit 권한을 가져야 한다")
	// viewer보다 더 많은 권한 보유 — write:workflow 확인
	assert.False(t, perms["delete:workflow"], "analyst는 delete:workflow 권한이 없어야 한다")
}

// TestEffectivePermissions_Viewer — viewer 역할은 읽기 전용 권한만 가짐
// AC-AUTH-004-2 case B: viewer는 GET 허용
func TestEffectivePermissions_Viewer(t *testing.T) {
	t.Parallel()
	roles := []auth.Role{auth.RoleViewer}

	perms := auth.EffectivePermissions(roles)

	require.NotNil(t, perms)
	assert.True(t, perms["read:workflow"], "viewer는 read:workflow 권한을 가져야 한다")
	assert.True(t, perms["read:recommendation"], "viewer는 read:recommendation 권한을 가져야 한다")
	// 쓰기 권한 없음 확인
	assert.False(t, perms["write:workflow"], "viewer는 write:workflow 권한이 없어야 한다")
	assert.False(t, perms["write:recommendation"], "viewer는 write:recommendation 권한이 없어야 한다")
	assert.False(t, perms["delete:workflow"], "viewer는 delete:workflow 권한이 없어야 한다")
	assert.False(t, perms["audit:read"], "viewer는 audit:read 권한이 없어야 한다")
}

// TestEffectivePermissions_Union — analyst+viewer 합집합은 analyst 권한을 포함해야 함
// AC-AUTH-004-4: 다중 역할의 permission set은 union
func TestEffectivePermissions_Union(t *testing.T) {
	t.Parallel()
	// Arrange — analyst가 이미 viewer의 superset이므로 union은 analyst와 동일
	roles := []auth.Role{auth.RoleAnalyst, auth.RoleViewer}

	perms := auth.EffectivePermissions(roles)

	require.NotNil(t, perms)
	// analyst 권한이 포함됨
	assert.True(t, perms["write:workflow"], "analyst+viewer union은 write:workflow를 포함해야 한다")
	assert.True(t, perms["read:audit"], "analyst+viewer union은 read:audit를 포함해야 한다")
}

// ────────────────────────────────────────────────────────────
// Authorize 테스트
// ────────────────────────────────────────────────────────────

// TestAuthorize_AdminHasAllPerms — admin 사용자는 모든 권한 통과
// AC-AUTH-004-3: admin role — all methods allowed (RBAC 거부 0건)
func TestAuthorize_AdminHasAllPerms(t *testing.T) {
	t.Parallel()
	// Arrange — context에 admin user 주입
	u := &auth.User{
		UID:    "admin-uuid",
		Scopes: []string{"iroum-ax:admin"},
	}
	ctx := auth.WithUser(context.Background(), u)

	// Act
	err := auth.Authorize(ctx, "write:workflow")

	// Assert — admin은 어떤 권한도 가짐
	assert.NoError(t, err, "admin 사용자는 write:workflow 권한 검증을 통과해야 한다")
}

// TestAuthorize_ViewerCannotWrite_ReturnsErrInsufficientPermission — viewer는 write 권한 없음
// AC-AUTH-004-2 case A: viewer POST → 403 (ErrInsufficientPermission)
func TestAuthorize_ViewerCannotWrite_ReturnsErrInsufficientPermission(t *testing.T) {
	t.Parallel()
	// Arrange — context에 viewer user 주입
	u := &auth.User{
		UID:    "viewer-uuid",
		Scopes: []string{"iroum-ax:viewer"},
	}
	ctx := auth.WithUser(context.Background(), u)

	// Act
	err := auth.Authorize(ctx, "write:workflow")

	// Assert — viewer는 write:workflow 권한 없음 → ErrInsufficientPermission
	require.Error(t, err, "viewer 사용자는 write:workflow 권한 검증에 실패해야 한다")
	assert.ErrorIs(t, err, auth.ErrInsufficientPermission,
		"에러는 ErrInsufficientPermission이어야 한다 (HTTP 403 매핑)")
}

// TestAuthorize_ViewerCanRead_ReturnsNil — viewer는 read 권한 통과
// AC-AUTH-004-2 case B: viewer GET → 200
func TestAuthorize_ViewerCanRead_ReturnsNil(t *testing.T) {
	t.Parallel()
	u := &auth.User{
		UID:    "viewer-uuid",
		Scopes: []string{"iroum-ax:viewer"},
	}
	ctx := auth.WithUser(context.Background(), u)

	err := auth.Authorize(ctx, "read:workflow")

	assert.NoError(t, err, "viewer 사용자는 read:workflow 권한 검증을 통과해야 한다")
}

// TestAuthorize_NoUserInContext_ReturnsError — context에 사용자가 없으면 에러 반환
// 미들웨어 우회 버그 감지
func TestAuthorize_NoUserInContext_ReturnsError(t *testing.T) {
	t.Parallel()
	// Arrange — 사용자가 주입되지 않은 빈 context
	ctx := context.Background()

	// Act
	err := auth.Authorize(ctx, "read:workflow")

	// Assert — context에 사용자가 없으면 에러
	require.Error(t, err, "context에 사용자가 없으면 Authorize는 에러를 반환해야 한다")
}

// TestAuthorize_UnknownScope_NoPermission — 미인식 scope는 권한 없음 처리
// AC-AUTH-004-5: unknown scope → 403, granted_roles=[]
func TestAuthorize_UnknownScope_NoPermission(t *testing.T) {
	t.Parallel()
	// Arrange — 허용 목록에 없는 역할
	u := &auth.User{
		UID:    "hacker-uuid",
		Scopes: []string{"iroum-ax:hacker", "iroum-ax:superuser"},
	}
	ctx := auth.WithUser(context.Background(), u)

	// Act
	err := auth.Authorize(ctx, "read:workflow")

	// Assert — 미인식 역할은 권한 없음
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInsufficientPermission)
}

// TestAuthorize_MultipleRoles_UnionPerms — analyst+viewer는 analyst 권한으로 통과
// AC-AUTH-004-4: scope union — analyst 권한 포함
func TestAuthorize_MultipleRoles_UnionPerms(t *testing.T) {
	t.Parallel()
	u := &auth.User{
		UID:    "multi-role-uuid",
		Scopes: []string{"iroum-ax:analyst", "iroum-ax:viewer"},
	}
	ctx := auth.WithUser(context.Background(), u)

	// analyst는 write:workflow 권한 보유 → union도 통과해야 함
	err := auth.Authorize(ctx, "write:workflow")

	assert.NoError(t, err, "analyst+viewer union은 write:workflow 권한을 통과해야 한다")
}

// ────────────────────────────────────────────────────────────
// LogForbidden 테스트 (audit 연동)
// ────────────────────────────────────────────────────────────

// fakeForbiddenTx — LogForbidden 테스트용 인메모리 AuditTx 구현체
// FakeStore의 FakeTx를 직접 사용하기 위해 별도 패키지로의 접근이 필요하므로
// 여기서는 간단한 인메모리 captureTx를 정의한다
type fakeForbiddenTx struct {
	Captured []*audit.Event
}

func (f *fakeForbiddenTx) InsertAuditLog(_ context.Context, e *audit.Event) error {
	f.Captured = append(f.Captured, e)
	return nil
}

// TestLogForbidden_RecordsAuditEvent — LogForbidden 호출 시 AUTH_FORBIDDEN 이벤트가 기록됨
// AC-AUTH-004-2 case A: audit action=AUTH_FORBIDDEN 기록
func TestLogForbidden_RecordsAuditEvent(t *testing.T) {
	t.Parallel()
	// Arrange
	tx := &fakeForbiddenTx{}
	roles := []auth.Role{auth.RoleViewer}

	// Act
	err := auth.LogForbidden(context.Background(), tx, "POST", "/api/v1/workflows", "viewer-uuid", roles)

	// Assert
	require.NoError(t, err)
	require.Len(t, tx.Captured, 1, "LogForbidden은 정확히 1개의 감사 이벤트를 삽입해야 한다")
	assert.Equal(t, audit.ActionAuthForbidden, tx.Captured[0].Action,
		"감사 이벤트 action은 AUTH_FORBIDDEN이어야 한다")
}

// TestLogForbidden_IncludesMethodAndPath — details에 method + path 포함
// AC-AUTH-004-2: details={"method":"POST","path":"/api/v1/workflows",...}
func TestLogForbidden_IncludesMethodAndPath(t *testing.T) {
	t.Parallel()
	// Arrange
	tx := &fakeForbiddenTx{}
	roles := []auth.Role{auth.RoleViewer}

	// Act
	err := auth.LogForbidden(context.Background(), tx, "POST", "/api/v1/workflows", "viewer-uuid", roles)

	// Assert
	require.NoError(t, err)
	require.Len(t, tx.Captured, 1)

	// details JSON 파싱
	var details map[string]any
	require.NoError(t, json.Unmarshal(tx.Captured[0].DetailsJSON, &details),
		"DetailsJSON은 유효한 JSON이어야 한다")
	assert.Equal(t, "POST", details["method"], "details.method는 호출 HTTP method여야 한다")
	assert.Equal(t, "/api/v1/workflows", details["path"], "details.path는 요청 경로여야 한다")
}

// TestLogForbidden_UserIDFromContext — user_id가 context에서 추출된 ID와 일치
// AC-AUTH-004-2: user_id=token sub
func TestLogForbidden_UserIDFromContext(t *testing.T) {
	t.Parallel()
	// Arrange
	tx := &fakeForbiddenTx{}
	roles := []auth.Role{auth.RoleViewer}
	userID := "viewer-uuid-from-token-sub"

	// Act
	err := auth.LogForbidden(context.Background(), tx, "POST", "/api/v1/workflows", userID, roles)

	// Assert
	require.NoError(t, err)
	require.Len(t, tx.Captured, 1)
	assert.Equal(t, userID, tx.Captured[0].UserID,
		"감사 이벤트 user_id는 토큰 sub 클레임에서 추출된 ID와 일치해야 한다")
}

// TestLogForbidden_IncludesGrantedRoles — details에 granted_roles 포함
// AC-AUTH-004-2: details.granted=viewer
func TestLogForbidden_IncludesGrantedRoles(t *testing.T) {
	t.Parallel()
	// Arrange
	tx := &fakeForbiddenTx{}
	roles := []auth.Role{auth.RoleViewer}

	// Act
	err := auth.LogForbidden(context.Background(), tx, "POST", "/api/v1/workflows", "viewer-uuid", roles)

	// Assert
	require.NoError(t, err)
	require.Len(t, tx.Captured, 1)

	var details map[string]any
	require.NoError(t, json.Unmarshal(tx.Captured[0].DetailsJSON, &details))

	// granted_roles 필드가 존재해야 함
	grantedRoles, ok := details["granted_roles"]
	assert.True(t, ok, "details.granted_roles 필드가 존재해야 한다")
	assert.NotNil(t, grantedRoles)
}

// TestLogForbidden_WithFakeStore_AuditIntegration — FakeStore를 사용한 통합 확인
// FakeStore의 AuditLogs에 AUTH_FORBIDDEN 이벤트가 기록됨을 검증
func TestLogForbidden_WithFakeStore_AuditIntegration(t *testing.T) {
	t.Parallel()
	// Arrange — FakeStore로 AuditTx 생성
	fs := store.NewFakeStore()
	ctx := context.Background()

	tx, err := fs.BeginTx(ctx)
	require.NoError(t, err)

	roles := []auth.Role{auth.RoleViewer}

	// Act
	logErr := auth.LogForbidden(ctx, tx, "DELETE", "/api/v1/workflows/wf-001", "viewer-uuid", roles)

	// Assert
	require.NoError(t, logErr)
	require.NoError(t, tx.Commit(ctx))

	// FakeStore AuditLogs에서 AUTH_FORBIDDEN 이벤트 확인
	// (store.WorkflowTx는 audit.AuditTx를 구현하므로 직접 사용 가능)
	require.Len(t, fs.AuditLogs, 1)
	assert.Equal(t, audit.ActionAuthForbidden, fs.AuditLogs[0].Action)
}
