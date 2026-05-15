// permission.go — metrics 접근 권한 확인 (admin role only)
// SPEC-AX-OBS-001 Sprint 0 GREEN: REQ-OBS-002
// auth.ParseRolesFromScope를 재사용하여 admin 역할 여부를 판단한다.
// metrics → auth 단방향 import (auth → metrics 금지)
package metrics

import (
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
)

// IsMetricsAuthorized — 사용자가 /metrics endpoint에 접근할 수 있는지 확인한다.
//
// 규칙: RoleAdmin 역할을 보유한 사용자만 허용 (REQ-OBS-002).
// Scopes 필드를 ParseRolesFromScope로 변환하여 RoleAdmin 포함 여부를 검사한다.
//
// @MX:ANCHOR: [AUTO] metrics 권한 결정 단일 진입점
// @MX:REASON: MetricsAuthMiddleware + MetricsAuthMiddlewareWithUserInjector + 테스트에서 호출 (fan_in >= 3)
func IsMetricsAuthorized(u *auth.User) bool {
	if u == nil {
		return false
	}
	// Scopes 목록을 공백으로 이어 붙여 ParseRolesFromScope에 전달
	scopeStr := ""
	for i, s := range u.Scopes {
		if i > 0 {
			scopeStr += " "
		}
		scopeStr += s
	}
	roles := auth.ParseRolesFromScope(scopeStr)
	for _, r := range roles {
		if r == auth.RoleAdmin {
			return true
		}
	}
	return false
}
