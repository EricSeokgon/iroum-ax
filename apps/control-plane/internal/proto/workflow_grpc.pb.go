// 워크플로우 gRPC 서비스 스텁 정의
// Code generated (hand-written in absence of protoc). DO NOT EDIT.
//
// 실제 protoc 환경이 갖춰지면 `buf generate` 결과물로 교체 예정.
// WorkflowServiceServer 인터페이스와 RegisterWorkflowServiceServer 헬퍼를 제공한다.
//
// Sprint 4 RED 단계: 테스트가 import하는 최소 gRPC 서비스 계약만 선언.
package proto

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WorkflowServiceServer gRPC 서비스 서버 인터페이스
// 실제 protoc-gen-go-grpc가 생성하는 인터페이스와 동일한 시그니처를 유지한다.
//
// @MX:ANCHOR: [AUTO] Sprint 4 gRPC 핸들러의 구현 계약 — server 패키지가 이 인터페이스를 구현
// @MX:REASON: gRPC 핸들러(WorkflowService), 통합 테스트 클라이언트, bufconn 다이얼러 3곳에서 참조
type WorkflowServiceServer interface {
	// CreateWorkflow 새 워크플로우를 생성하여 PENDING 상태로 반환 (AC-CTRL-002-1)
	CreateWorkflow(context.Context, *CreateWorkflowRequest) (*CreateWorkflowResponse, error)
	// GetWorkflow 워크플로우 ID로 단건 조회 (AC-CTRL-002-2)
	GetWorkflow(context.Context, *GetWorkflowRequest) (*GetWorkflowResponse, error)
	// ListWorkflows 페이지네이션 파라미터를 받아 워크플로우 목록 반환 (AC-CTRL-002-3)
	ListWorkflows(context.Context, *ListWorkflowsRequest) (*ListWorkflowsResponse, error)
	// mustEmbedUnimplementedWorkflowServiceServer 하위 호환성 강제 (protoc-gen-go-grpc 패턴)
	mustEmbedUnimplementedWorkflowServiceServer()
}

// UnimplementedWorkflowServiceServer 모든 메서드가 codes.Unimplemented를 반환하는 기반 구현체
// GREEN 단계 전 스텁으로 사용하며, WorkflowService가 임베딩하여 미구현 메서드를 안전하게 처리한다.
type UnimplementedWorkflowServiceServer struct{}

// CreateWorkflow codes.Unimplemented 반환 (GREEN 단계에서 교체)
func (UnimplementedWorkflowServiceServer) CreateWorkflow(
	_ context.Context,
	_ *CreateWorkflowRequest,
) (*CreateWorkflowResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateWorkflow not implemented")
}

// GetWorkflow codes.Unimplemented 반환 (GREEN 단계에서 교체)
func (UnimplementedWorkflowServiceServer) GetWorkflow(
	_ context.Context,
	_ *GetWorkflowRequest,
) (*GetWorkflowResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetWorkflow not implemented")
}

// ListWorkflows codes.Unimplemented 반환 (GREEN 단계에서 교체)
func (UnimplementedWorkflowServiceServer) ListWorkflows(
	_ context.Context,
	_ *ListWorkflowsRequest,
) (*ListWorkflowsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListWorkflows not implemented")
}

// mustEmbedUnimplementedWorkflowServiceServer 하위 호환성 강제 메서드 (unexported)
func (UnimplementedWorkflowServiceServer) mustEmbedUnimplementedWorkflowServiceServer() {}

// WorkflowServiceDesc gRPC 서비스 설명자
// grpc.Server.RegisterService가 요구하는 ServiceDesc 구조체
var WorkflowServiceDesc = grpc.ServiceDesc{
	ServiceName: "iroum.ax.v1.WorkflowService",
	HandlerType: (*WorkflowServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateWorkflow",
			Handler:    workflowServiceCreateWorkflowHandler,
		},
		{
			MethodName: "GetWorkflow",
			Handler:    workflowServiceGetWorkflowHandler,
		},
		{
			MethodName: "ListWorkflows",
			Handler:    workflowServiceListWorkflowsHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "workflow.proto",
}

// RegisterWorkflowServiceServer grpc.Server에 WorkflowServiceServer를 등록
// protoc-gen-go-grpc가 생성하는 동일 이름의 함수와 시그니처 호환
func RegisterWorkflowServiceServer(s *grpc.Server, srv WorkflowServiceServer) {
	s.RegisterService(&WorkflowServiceDesc, srv)
}

// --- 핸들러 어댑터 함수 (grpc.MethodDesc.Handler 시그니처 충족) ---

func workflowServiceCreateWorkflowHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(CreateWorkflowRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkflowServiceServer).CreateWorkflow(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/iroum.ax.v1.WorkflowService/CreateWorkflow",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WorkflowServiceServer).CreateWorkflow(ctx, req.(*CreateWorkflowRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func workflowServiceGetWorkflowHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(GetWorkflowRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkflowServiceServer).GetWorkflow(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/iroum.ax.v1.WorkflowService/GetWorkflow",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WorkflowServiceServer).GetWorkflow(ctx, req.(*GetWorkflowRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func workflowServiceListWorkflowsHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(ListWorkflowsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkflowServiceServer).ListWorkflows(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/iroum.ax.v1.WorkflowService/ListWorkflows",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WorkflowServiceServer).ListWorkflows(ctx, req.(*ListWorkflowsRequest))
	}
	return interceptor(ctx, in, info, handler)
}
