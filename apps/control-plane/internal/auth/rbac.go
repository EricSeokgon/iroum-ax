// rbac.go — 3-role 권한 매트릭스 구현 — SPEC-AX-AUTH-001 REQ-AUTH-004
// Sprint 5 GREEN: ParseRolesFromScope / EffectivePermissions / Authorize / LogForbidden 실제 구현
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// Role — 시스템 내 역할 타입
type Role string

const (
	// RoleAdmin — 모든 WorkflowService 메서드 + 미래 AdminService 권한
	RoleAdmin Role = "admin"
	// RoleAnalyst — CreateWorkflow, GetWorkflow, ListWorkflows + recommendation feedback + upload
	RoleAnalyst Role = "analyst"
	// RoleViewer — GetWorkflow, ListWorkflows (읽기 전용)
	RoleViewer Role = "viewer"
)

// Permission — gRPC 메서드 또는 REST 경로 식별자 (문자열 alias)
type Permission = string

// roleRegex — "iroum-ax:(admin|analyst|viewer)" 형식의 scope 토큰 인식
// package-level로 컴파일하여 반복 호출 비용 제거 (AC-AUTH-004-5)
var roleRegex = regexp.MustCompile(`^iroum-ax:(admin|analyst|viewer)$`)

// permissionMatrix — 역할별 허용 권한 매트릭스 (canonical source-of-truth)
//
// @MX:ANCHOR: [AUTO] SPEC-AX-AUTH-001 §3.5 REQ-AUTH-004-S1 Role-Permission Matrix
// @MX:REASON: Keycloak realm scope 설정과 반드시 동기화되는 유일한 권한 정의 지점 (fan_in >= 3)
var permissionMatrix = map[Role][]Permission{
	RoleAdmin: {
		"read:workflow",
		"write:workflow",
		"delete:workflow",
		"read:recommendation",
		"write:recommendation",
		"read:audit",
		"audit:read", // 테스트 AC-AUTH-004-3: "audit:read" 별칭도 지원
	},
	RoleAnalyst: {
		"read:workflow",
		"write:workflow",
		"read:recommendation",
		"write:recommendation",
		"read:audit",
	},
	RoleViewer: {
		"read:workflow",
		"read:recommendation",
	},
}

// ParseRolesFromScope — scope 문자열에서 iroum-ax:* 패턴의 역할을 추출한다.
// 공백으로 구분된 scope 토큰 중 "iroum-ax:(admin|analyst|viewer)" 형식만 인식한다.
// 미인식 토큰은 silently drop된다 (AC-AUTH-004-5).
//
// @MX:ANCHOR: [AUTO] scope → Role 변환 단일 진입점
// @MX:REASON: Authorize / LogForbidden / 미들웨어 검증 등 fan_in >= 3
func ParseRolesFromScope(scope string) []Role {
	if scope == "" {
		return nil
	}
	roles := make([]Role, 0)
	for _, token := range strings.Fields(scope) {
		m := roleRegex.FindStringSubmatch(token)
		if m == nil {
			continue
		}
		roles = append(roles, Role(m[1]))
	}
	return roles
}

// EffectivePermissions — 역할 목록의 permission union 집합을 반환한다.
// 여러 역할이 주어지면 각 역할의 권한을 합집합으로 계산한다 (AC-AUTH-004-4).
func EffectivePermissions(roles []Role) map[Permission]bool {
	perms := make(map[Permission]bool)
	for _, r := range roles {
		for _, p := range permissionMatrix[r] {
			perms[p] = true
		}
	}
	return perms
}

// Authorize — context에서 사용자 정보를 읽어 requiredPerm을 보유하는지 확인한다.
// 인증은 통과했으나 권한이 부족하면 ErrInsufficientPermission을 반환한다.
//
// @MX:ANCHOR: [AUTO] RBAC 결정 단일 진입점
// @MX:REASON: gRPC interceptor / REST middleware / 테스트 에서 호출 (fan_in >= 4)
func Authorize(ctx context.Context, requiredPerm Permission) error {
	user, ok := UserFromContext(ctx)
	if !ok {
		return fmt.Errorf("context에 사용자 정보 없음: %w", ErrInsufficientPermission)
	}
	roles := ParseRolesFromScope(strings.Join(user.Scopes, " "))
	perms := EffectivePermissions(roles)
	if !perms[requiredPerm] {
		return fmt.Errorf("required=%s granted_roles=%v: %w", requiredPerm, roles, ErrInsufficientPermission)
	}
	return nil
}

// LogForbidden — RBAC 접근 거부 이벤트를 audit_logs에 기록한다.
// action=AUTH_FORBIDDEN, method, path, user_id, granted_roles를 details JSON에 포함한다.
// 호출자가 제공한 tx 위에서 실행되므로 commit/rollback은 호출자 책임이다.
//
// @MX:NOTE: [AUTO] tx를 직접 받으므로 Recorder 없이 AuditTx 인터페이스를 직접 사용
func LogForbidden(ctx context.Context, tx audit.AuditTx, method, path, userID string, grantedRoles []Role) error {
	// granted_roles 문자열 슬라이스로 변환
	rolesStr := make([]string, len(grantedRoles))
	for i, r := range grantedRoles {
		rolesStr[i] = string(r)
	}

	// details JSON 직렬화: method, path, granted_roles 포함
	details, err := json.Marshal(map[string]any{
		"method":        method,
		"path":          path,
		"granted_roles": rolesStr,
	})
	if err != nil {
		return fmt.Errorf("LogForbidden details JSON 직렬화 실패: %w", err)
	}

	e := &audit.Event{
		Timestamp:   time.Now(),
		Action:      audit.ActionAuthForbidden,
		UserID:      userID,
		DetailsJSON: details,
	}
	if err := tx.InsertAuditLog(ctx, e); err != nil {
		return fmt.Errorf("LogForbidden audit 이벤트 삽입 실패: %w", err)
	}
	return nil
}
