// grpc_server.go — WorkflowService gRPC 서버 구현체
// Sprint 4 GREEN: CreateWorkflow / GetWorkflow / ListWorkflows 비즈니스 로직 구현
// REQ-CTRL-002: gRPC :50051에서 세 가지 RPC 메서드 제공
package server

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	proto "github.com/ircp/iroum-ax/apps/control-plane/internal/proto"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// defaultListLimit ListWorkflows 기본 limit (요청에 0이 전달된 경우 적용)
	defaultListLimit = 100
	// maxListLimit ListWorkflows 최대 limit (1000 초과 요청은 1000으로 클램핑)
	maxListLimit = 1000
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
	// sm.Coordinator()를 통해 TxCoordinator에 접근
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
func (s *WorkflowService) CreateWorkflow(
	ctx context.Context,
	req *proto.CreateWorkflowRequest,
) (*proto.CreateWorkflowResponse, error) {
	// 컨텍스트 취소 조기 확인
	if ctx.Err() != nil {
		return nil, status.Errorf(codes.Canceled, "요청 컨텍스트가 취소됨: %v", ctx.Err())
	}

	// DocumentID 유효성 검사
	if req.DocumentID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "document_id는 필수 항목입니다")
	}

	docUUID, err := uuid.Parse(req.DocumentID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id 파싱 실패: %v", err)
	}

	// 새 워크플로우 엔티티 구성
	now := time.Now().UTC()
	wf := &types.Workflow{
		ID:         uuid.New(),
		DocumentID: docUUID,
		State:      types.WorkflowStatePending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// TxCoordinator: BeginTx → InsertWorkflow + InsertAuditLog → Commit/Rollback
	// sm.Coordinator()를 통해 상태 머신이 보유한 TxCoordinator 재사용
	if err = s.sm.Coordinator().ExecuteWorkflowCreate(ctx, wf, req.DocumentID, "cli-anonymous"); err != nil {
		// 컨텍스트 취소 여부를 추가로 확인
		if ctx.Err() == context.Canceled {
			return nil, status.Errorf(codes.Canceled, "워크플로우 생성 중 컨텍스트 취소됨")
		}
		s.logger.Error("CreateWorkflow 실패", zap.String("document_id", req.DocumentID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "워크플로우 생성 실패: %v", err)
	}

	return &proto.CreateWorkflowResponse{
		Workflow: workflowToProto(wf),
	}, nil
}

// GetWorkflow 워크플로우 ID로 단건 조회
// AC-CTRL-002-2: 존재하면 Workflow 반환, 없으면 codes.NotFound
func (s *WorkflowService) GetWorkflow(
	ctx context.Context,
	req *proto.GetWorkflowRequest,
) (*proto.GetWorkflowResponse, error) {
	// ID 유효성 검사 (UUID 파싱)
	if _, err := uuid.Parse(req.ID); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "workflow_id 파싱 실패: %v", err)
	}

	// 읽기 전용 트랜잭션으로 조회 (Rollback으로 종료)
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "트랜잭션 시작 실패: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck

	wf, err := tx.GetWorkflow(ctx, req.ID)
	if err != nil {
		if errors.Is(err, stderrors.ErrWorkflowNotFound) {
			return nil, status.Errorf(codes.NotFound, "워크플로우를 찾을 수 없음: id=%s", req.ID)
		}
		s.logger.Error("GetWorkflow 실패", zap.String("id", req.ID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "워크플로우 조회 실패: %v", err)
	}

	return &proto.GetWorkflowResponse{
		Workflow: workflowToProto(wf),
	}, nil
}

// ListWorkflows 페이지네이션 파라미터를 받아 워크플로우 목록 반환
// AC-CTRL-002-3: limit/offset 기반 목록 조회
func (s *WorkflowService) ListWorkflows(
	ctx context.Context,
	req *proto.ListWorkflowsRequest,
) (*proto.ListWorkflowsResponse, error) {
	// limit 기본값/최대값 적용
	limit := int(req.Limit)
	if limit <= 0 {
		limit = defaultListLimit
	} else if limit > maxListLimit {
		limit = maxListLimit
	}
	offset := int(req.Offset)

	wfs, err := s.store.ListWorkflows(ctx, limit, offset)
	if err != nil {
		s.logger.Error("ListWorkflows 실패", zap.Int("limit", limit), zap.Int("offset", offset), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "워크플로우 목록 조회 실패: %v", err)
	}

	protoWFs := make([]*proto.Workflow, 0, len(wfs))
	for _, wf := range wfs {
		protoWFs = append(protoWFs, workflowToProto(wf))
	}

	return &proto.ListWorkflowsResponse{
		Workflows: protoWFs,
		Total:     int32(len(protoWFs)), //nolint:gosec
	}, nil
}

// workflowToProto types.Workflow를 proto.Workflow로 변환하는 헬퍼
// @MX:NOTE: [AUTO] types ↔ proto 변환의 단일 진입점 — 중복 변환 코드 방지
func workflowToProto(wf *types.Workflow) *proto.Workflow {
	createdAt := wf.CreatedAt
	updatedAt := wf.UpdatedAt

	p := &proto.Workflow{
		ID:         wf.ID.String(),
		DocumentID: wf.DocumentID.String(),
		Status:     workflowStateToProtoStatus(wf.State),
		CreatedAt:  &createdAt,
		UpdatedAt:  &updatedAt,
		ResultJSON: wf.ResultJSON,
	}
	if wf.ReportID != nil {
		p.ReportID = wf.ReportID.String()
	}
	return p
}

// workflowStateToProtoStatus types.WorkflowState를 proto.WorkflowStatus로 변환
func workflowStateToProtoStatus(state types.WorkflowState) proto.WorkflowStatus {
	switch state {
	case types.WorkflowStatePending:
		return proto.WorkflowStatusPending
	case types.WorkflowStateRunning:
		return proto.WorkflowStatusRunning
	case types.WorkflowStateCompleted:
		return proto.WorkflowStatusCompleted
	case types.WorkflowStateFailed:
		return proto.WorkflowStatusFailed
	default:
		return proto.WorkflowStatusUnspecified
	}
}
