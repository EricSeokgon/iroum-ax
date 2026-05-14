// 워크플로우 protobuf 메시지 Go 타입 정의
// Code generated (hand-written in absence of protoc). DO NOT EDIT.
//
// 실제 protoc 환경이 갖춰지면 `buf generate` 결과물로 교체 예정.
// 패키지 경로는 workflow.proto의 go_package 옵션과 동일하게 유지:
//
//	option go_package = "github.com/ircp/iroum-ax/internal/proto/ax/v1;axv1";
//
// Sprint 4 RED 단계에서 테스트 컨트랙트 확립 목적으로 최소 타입만 선언.
package proto

import (
	"time"
)

// WorkflowStatus 워크플로우 실행 상태 (proto enum 대응)
type WorkflowStatus int32

const (
	// WorkflowStatusUnspecified 기본값 (proto: WORKFLOW_STATUS_UNSPECIFIED = 0)
	WorkflowStatusUnspecified WorkflowStatus = 0
	// WorkflowStatusPending 생성 직후 초기 상태 (proto: WORKFLOW_STATUS_PENDING = 1)
	WorkflowStatusPending WorkflowStatus = 1
	// WorkflowStatusRunning Celery 태스크 디스패치 후 실행 중 (proto: WORKFLOW_STATUS_RUNNING = 2)
	WorkflowStatusRunning WorkflowStatus = 2
	// WorkflowStatusCompleted 모든 파이프라인 단계 성공 완료 (proto: WORKFLOW_STATUS_COMPLETED = 3)
	WorkflowStatusCompleted WorkflowStatus = 3
	// WorkflowStatusFailed 처리 오류 또는 디스패치 실패 (proto: WORKFLOW_STATUS_FAILED = 4)
	WorkflowStatusFailed WorkflowStatus = 4
)

// String WorkflowStatus 문자열 표현 반환
func (s WorkflowStatus) String() string {
	switch s {
	case WorkflowStatusPending:
		return "PENDING"
	case WorkflowStatusRunning:
		return "RUNNING"
	case WorkflowStatusCompleted:
		return "COMPLETED"
	case WorkflowStatusFailed:
		return "FAILED"
	default:
		return "UNSPECIFIED"
	}
}

// Workflow 워크플로우 엔티티 메시지 (proto: message Workflow)
// 필드 순서: 시간(24B) → 포인터(8B) → 정수(4B) → 문자열들 (fieldalignment 최적화)
type Workflow struct {
	// CreatedAt 생성 시각
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// UpdatedAt 최종 갱신 시각
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	// Status 현재 상태
	Status WorkflowStatus `json:"status"`
	// ID 워크플로우 고유 식별자 (UUID)
	ID string `json:"id"`
	// UserID 사용자 식별자 (기본: "cli-anonymous")
	UserID string `json:"user_id"`
	// DocumentID 연결된 문서 ID (UUID)
	DocumentID string `json:"document_id"`
	// ReportID 생성된 보고서 ID (완료 후 채워짐, 빈 문자열이면 미완료)
	ReportID string `json:"report_id,omitempty"`
	// ResultJSON 최종 결과 (JSON 직렬화, nil이면 미완료)
	ResultJSON []byte `json:"result_json,omitempty"`
}

// CreateWorkflowRequest CreateWorkflow RPC 요청 메시지
type CreateWorkflowRequest struct {
	// DocumentID 처리할 문서의 UUID
	DocumentID string `json:"document_id"`
}

// CreateWorkflowResponse CreateWorkflow RPC 응답 메시지
type CreateWorkflowResponse struct {
	// Workflow 생성된 워크플로우 (상태: PENDING)
	Workflow *Workflow `json:"workflow"`
}

// GetWorkflowRequest GetWorkflow RPC 요청 메시지
type GetWorkflowRequest struct {
	// ID 조회할 워크플로우 UUID
	ID string `json:"id"`
}

// GetWorkflowResponse GetWorkflow RPC 응답 메시지
type GetWorkflowResponse struct {
	// Workflow 조회된 워크플로우
	Workflow *Workflow `json:"workflow"`
}

// ListWorkflowsRequest ListWorkflows RPC 요청 메시지 (기본 페이지네이션)
type ListWorkflowsRequest struct {
	// Limit 반환할 최대 워크플로우 수 (0이면 서버 기본값 적용)
	Limit int32 `json:"limit,omitempty"`
	// Offset 건너뛸 워크플로우 수 (0-based)
	Offset int32 `json:"offset,omitempty"`
}

// ListWorkflowsResponse ListWorkflows RPC 응답 메시지
type ListWorkflowsResponse struct {
	// Workflows 반환된 워크플로우 목록
	Workflows []*Workflow `json:"workflows"`
	// Total 필터 조건에 일치하는 전체 워크플로우 수 (페이지네이션 메타)
	Total int32 `json:"total"`
}
