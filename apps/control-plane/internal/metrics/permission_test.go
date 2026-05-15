// permission_test.go — IsMetricsAuthorized 함수 테스트
// SPEC-AX-OBS-001 Sprint 0 RED
package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
)

// TestIsMetricsAuthorized_Admin — admin 역할 사용자는 metrics 접근이 허용돼야 한다.
func TestIsMetricsAuthorized_Admin(t *testing.T) {
	t.Parallel()
	u := &auth.User{
		UID:    "admin-user",
		Scopes: []string{"iroum-ax:admin"},
	}
	assert.True(t, IsMetricsAuthorized(u), "admin 역할 사용자는 metrics 접근이 허용돼야 한다")
}

// TestIsMetricsAuthorized_Viewer — viewer 역할 사용자는 metrics 접근이 거부돼야 한다.
func TestIsMetricsAuthorized_Viewer(t *testing.T) {
	t.Parallel()
	u := &auth.User{
		UID:    "viewer-user",
		Scopes: []string{"iroum-ax:viewer"},
	}
	assert.False(t, IsMetricsAuthorized(u), "viewer 역할 사용자는 metrics 접근이 거부돼야 한다")
}

// TestIsMetricsAuthorized_Analyst — analyst 역할 사용자는 metrics 접근이 거부돼야 한다.
func TestIsMetricsAuthorized_Analyst(t *testing.T) {
	t.Parallel()
	u := &auth.User{
		UID:    "analyst-user",
		Scopes: []string{"iroum-ax:analyst"},
	}
	assert.False(t, IsMetricsAuthorized(u), "analyst 역할 사용자는 metrics 접근이 거부돼야 한다")
}

// TestIsMetricsAuthorized_NoScope — scope 없는 사용자는 metrics 접근이 거부돼야 한다.
func TestIsMetricsAuthorized_NoScope(t *testing.T) {
	t.Parallel()
	u := &auth.User{
		UID:    "no-scope-user",
		Scopes: []string{},
	}
	assert.False(t, IsMetricsAuthorized(u), "scope 없는 사용자는 metrics 접근이 거부돼야 한다")
}
