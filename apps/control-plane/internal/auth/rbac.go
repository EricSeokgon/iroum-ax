// RBAC — 3-role 권한 매트릭스 stub — SPEC-AX-AUTH-001 REQ-AUTH-004
// Sprint 5 GREEN에서 실제 구현 예정
package auth

import (
	"context"
	"errors"
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

// Permission — gRPC 메서드 또는 REST 경로 식별자
type Permission = string

// permissionMatrix — 역할별 허용 권한 매트릭스 (canonical source-of-truth)
//
// @MX:NOTE: [AUTO] SPEC-AX-AUTH-001 §3.5 REQ-AUTH-004-S1 Role-Permission Matrix
// Keycloak realm scope 설정(iroum-ax:admin / iroum-ax:analyst / iroum-ax:viewer)과 반드시 동기화
var permissionMatrix = map[Role][]Permission{
	RoleAdmin: {
		// 모든 WorkflowService gRPC 메서드
		"/workflow.WorkflowService/CreateWorkflow",
		"/workflow.WorkflowService/GetWorkflow",
		"/workflow.WorkflowService/ListWorkflows",
		// 모든 REST /api/v1/* 경로 (와일드카드 — Sprint 5에서 정밀 매핑)
		"REST:*",
	},
	RoleAnalyst: {
		"/workflow.WorkflowService/CreateWorkflow",
		"/workflow.WorkflowService/GetWorkflow",
		"/workflow.WorkflowService/ListWorkflows",
		"REST:POST:/api/v1/workflows",
		"REST:GET:/api/v1/workflows",
		"REST:LIST:/api/v1/workflows",
		"REST:POST:/api/v1/recommendations/{id}/feedback",
		"REST:POST:/api/v1/documents/upload",
	},
	RoleViewer: {
		"/workflow.WorkflowService/GetWorkflow",
		"/workflow.WorkflowService/ListWorkflows",
		"REST:GET:/api/v1/workflows",
		"REST:LIST:/api/v1/workflows",
	},
}

// Authorize — context에서 사용자 정보를 읽어 requiredPerm을 보유하는지 확인한다.
// 인증은 통과했으나 권한이 부족하면 ErrInsufficientPermission을 반환한다.
//
// permissionMatrix는 Sprint 5 GREEN에서 이 함수 내부에서 사용된다.
//
// @MX:ANCHOR: [AUTO] RBAC 결정 단일 진입점
// @MX:REASON: gRPC interceptor / REST middleware / FastAPI Depends / RBAC 테스트 에서 호출 (fan_in >= 4)
// @MX:TODO Sprint 5 — scope union 로직 및 regex `^iroum-ax:(admin|analyst|viewer)$` 파싱 구현
func Authorize(_ context.Context, requiredPerm string) error {
	// Sprint 5에서 permissionMatrix를 사용하는 실제 로직으로 교체
	_ = permissionMatrix
	_ = requiredPerm
	return errors.New("구현 예정: Sprint 5 GREEN")
}
