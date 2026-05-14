// grpc_server.go — WorkflowService gRPC 서버 구현체 스텁
// Sprint 4 RED: 모든 메서드는 codes.Unimplemented 반환 (GREEN 단계에서 실제 로직으로 교체)
// REQ-CTRL-002: gRPC :50051에서 CreateWorkflow / GetWorkflow / ListWorkflows 제공
package server

import (
	"context"

	proto "github.com/ircp/iroum-ax/apps/control-plane/internal/proto"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WorkflowService WorkflowServiceServer 인터페이스 구현체
// store, 상태 머신, 로거를 조합하여 gRPC 요청을 처리한다.
//
// @MX:ANCHOR: [AUTO] REQ-CTRL-002 gRPC 핸들러의 단일 진입점
// @MX:REASON: gRPC 등록(grpc_server.go), 단위 테스트(grpc_server_test.go), REST-to-gRPC 변환(Sprint 5)
type WorkflowService struct {
	proto.UnimplementedWorkflowServiceServer
	// logger 구조화 zap 로거
	logger *zap.Logger
	// store 워크플로우 영속성 스토어
	store store.WorkflowStore
	// sm 워크플로우 상태 머신 (Sprint 2 산출물)
	sm *workflow.StateMachine
}

// NewWorkflowService WorkflowService 인스턴스를 생성하여 반환
//
// @MX:ANCHOR: [AUTO] gRPC 서버 초기화의 진입점
// @MX:REASON: cmd/server/server.go, main.go, grpc_server_test.go 3곳에서 호출 예정
func NewWorkflowService(
	s store.WorkflowStore,
	sm *workflow.StateMachine,
	logger *zap.Logger,
) *WorkflowService {
	return &WorkflowService{
		store:  s,
		sm:     sm,
		logger: logger,
	}
}

// CreateWorkflow 새 워크플로우를 생성하여 PENDING 상태로 반환
// AC-CTRL-002-1: gRPC unary 호출 → 워크플로우 생성 + audit_logs row 삽입
//
// @MX:TODO: [AUTO] Sprint 4 GREEN에서 실제 TxCoordinator.ExecuteWorkflowCreate 호출로 교체
func (s *WorkflowService) CreateWorkflow(
	_ context.Context,
	_ *proto.CreateWorkflowRequest,
) (*proto.CreateWorkflowResponse, error) {
	// Sprint 4 RED: Unimplemented 반환 — 테스트가 이 상태를 실패로 관찰해야 함
	return nil, status.Errorf(codes.Unimplemented, "method CreateWorkflow not implemented")
}

// GetWorkflow 워크플로우 ID로 단건 조회
// AC-CTRL-002-2: 존재하면 Workflow 반환, 없으면 codes.NotFound
//
// @MX:TODO: [AUTO] Sprint 4 GREEN에서 store.GetWorkflow 호출로 교체
func (s *WorkflowService) GetWorkflow(
	_ context.Context,
	_ *proto.GetWorkflowRequest,
) (*proto.GetWorkflowResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetWorkflow not implemented")
}

// ListWorkflows 페이지네이션 파라미터를 받아 워크플로우 목록 반환
// AC-CTRL-002-3: limit/offset 기반 기본 목록 조회 (Sprint 4 범위: 단순 전체 순회)
//
// @MX:TODO: [AUTO] Sprint 4 GREEN에서 store.ListWorkflows 호출로 교체
func (s *WorkflowService) ListWorkflows(
	_ context.Context,
	_ *proto.ListWorkflowsRequest,
) (*proto.ListWorkflowsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListWorkflows not implemented")
}
