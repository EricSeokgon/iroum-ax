// authz_mapping_test.go — Sprint 0 RED: 권한 매핑 테이블 단위 테스트
// REQ-AUTH2-001-E1 (REST 매핑), REQ-AUTH2-001-E2 (gRPC 매핑), REQ-AUTH2-001-U1 (default-deny)
package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLookupRESTPermission_PositivePaths — 명세된 경로별 required permission 정상 매핑 검증
func TestLookupRESTPermission_PositivePaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		wantPerm   string
		wantFound  bool
		wantBypass bool
	}{
		// REQ-AUTH2-001-E1 REST 매핑 테이블
		{
			name:      "POST workflows → write:workflow",
			method:    "POST",
			path:      "/api/v1/workflows",
			wantPerm:  "write:workflow",
			wantFound: true,
		},
		{
			name:      "GET workflows list → read:workflow",
			method:    "GET",
			path:      "/api/v1/workflows",
			wantPerm:  "read:workflow",
			wantFound: true,
		},
		{
			name:      "GET workflow by id → read:workflow",
			method:    "GET",
			path:      "/api/v1/workflows/some-uuid-here",
			wantPerm:  "read:workflow",
			wantFound: true,
		},
		{
			name:      "DELETE workflow by id → delete:workflow",
			method:    "DELETE",
			path:      "/api/v1/workflows/some-uuid-here",
			wantPerm:  "delete:workflow",
			wantFound: true,
		},
		{
			name:      "POST recommendations feedback → write:recommendation",
			method:    "POST",
			path:      "/api/v1/recommendations/rec-id-123/feedback",
			wantPerm:  "write:recommendation",
			wantFound: true,
		},
		{
			name:      "POST documents upload → write:workflow",
			method:    "POST",
			path:      "/api/v1/documents/upload",
			wantPerm:  "write:workflow",
			wantFound: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			perm, bypass, found := LookupRESTPermission(tc.method, tc.path)
			assert.Equal(t, tc.wantFound, found, "found 불일치")
			assert.False(t, bypass, "bypass는 false여야 함")
			if tc.wantFound {
				assert.Equal(t, tc.wantPerm, perm, "permission 불일치")
			}
		})
	}
}

// TestLookupRESTPermission_BypassPaths — bypass 경로(인증/인가 불필요) 검증
func TestLookupRESTPermission_BypassPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "GET /health bypass", method: "GET", path: "/health"},
		{name: "HEAD any path bypass", method: "HEAD", path: "/api/v1/workflows"},
		{name: "OPTIONS any path bypass", method: "OPTIONS", path: "/api/v1/workflows"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			perm, bypass, found := LookupRESTPermission(tc.method, tc.path)
			assert.True(t, bypass, "bypass=true여야 함")
			assert.True(t, found, "bypass 경로는 found=true")
			assert.Empty(t, perm, "bypass 경로는 perm 빈 문자열")
		})
	}
}

// TestLookupRESTPermission_UnknownPath_DefaultDeny — 매핑 미정의 경로 default-deny 검증
// REQ-AUTH2-001-U1: 매핑 없으면 found=false → 호출자는 503 반환
func TestLookupRESTPermission_UnknownPath_DefaultDeny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "Unknown path", method: "GET", path: "/api/v1/unknown"},
		{name: "Unknown method on known path", method: "PATCH", path: "/api/v1/workflows"},
		{name: "Completely unknown", method: "PUT", path: "/api/v2/foo"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			perm, bypass, found := LookupRESTPermission(tc.method, tc.path)
			assert.False(t, found, "매핑 없는 경로는 found=false (default-deny)")
			assert.False(t, bypass, "bypass=false")
			assert.Empty(t, perm, "perm 빈 문자열")
		})
	}
}

// TestLookupRESTPermission_PathParam — 경로 파라미터({id}) 매칭 검증
// REQ-AUTH2-001-E1: /api/v1/workflows/{id} 형식의 경로 파라미터 처리
func TestLookupRESTPermission_PathParam(t *testing.T) {
	t.Parallel()

	// UUID, 숫자, 문자열 등 다양한 id 형식 검증
	ids := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"12345",
		"workflow-abc",
	}

	for _, id := range ids {
		id := id
		t.Run("GET workflows/"+id, func(t *testing.T) {
			t.Parallel()
			path := "/api/v1/workflows/" + id
			perm, bypass, found := LookupRESTPermission("GET", path)
			assert.True(t, found, "경로 파라미터 있는 경로는 found=true")
			assert.False(t, bypass)
			assert.Equal(t, "read:workflow", perm)
		})
		t.Run("DELETE workflows/"+id, func(t *testing.T) {
			t.Parallel()
			path := "/api/v1/workflows/" + id
			perm, bypass, found := LookupRESTPermission("DELETE", path)
			assert.True(t, found)
			assert.False(t, bypass)
			assert.Equal(t, "delete:workflow", perm)
		})
	}
}

// TestLookupGRPCPermission_PositiveMethods — gRPC FullMethod 매핑 검증
// REQ-AUTH2-001-E2
func TestLookupGRPCPermission_PositiveMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fullMethod string
		wantPerm   string
	}{
		{
			fullMethod: "/iroum.ax.v1.WorkflowService/CreateWorkflow",
			wantPerm:   "write:workflow",
		},
		{
			fullMethod: "/iroum.ax.v1.WorkflowService/GetWorkflow",
			wantPerm:   "read:workflow",
		},
		{
			fullMethod: "/iroum.ax.v1.WorkflowService/ListWorkflows",
			wantPerm:   "read:workflow",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.fullMethod, func(t *testing.T) {
			t.Parallel()
			perm, bypass, found := LookupGRPCPermission(tc.fullMethod)
			assert.True(t, found)
			assert.False(t, bypass)
			assert.Equal(t, tc.wantPerm, perm)
		})
	}
}

// TestLookupGRPCPermission_HealthBypass — Health Check RPC bypass 검증
func TestLookupGRPCPermission_HealthBypass(t *testing.T) {
	t.Parallel()
	perm, bypass, found := LookupGRPCPermission("/grpc.health.v1.Health/Check")
	assert.True(t, bypass)
	assert.True(t, found)
	assert.Empty(t, perm)
}

// TestLookupGRPCPermission_UnknownMethod_DefaultDeny — gRPC default-deny 검증
// REQ-AUTH2-001-U1
func TestLookupGRPCPermission_UnknownMethod_DefaultDeny(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/unknown.Service/Unknown",
		"/iroum.ax.v1.WorkflowService/DeleteWorkflow",
		"",
	}

	for _, m := range tests {
		m := m
		t.Run("unknown:"+m, func(t *testing.T) {
			t.Parallel()
			perm, bypass, found := LookupGRPCPermission(m)
			assert.False(t, found, "미매핑 gRPC 메서드는 default-deny")
			assert.False(t, bypass)
			assert.Empty(t, perm)
		})
	}
}

// BenchmarkLookupRESTPermission — 매핑 lookup 성능 측정 (target p99 < 100µs)
func BenchmarkLookupRESTPermission(b *testing.B) {
	b.ReportAllocs()
	for i := range b.N {
		_ = i
		_, _, _ = LookupRESTPermission("GET", "/api/v1/workflows/550e8400-e29b-41d4-a716-446655440000")
	}
}
