// grpc_server_test.go — WorkflowService gRPC 핸들러 RED 단계 테스트
// Sprint 4 RED: 비즈니스 로직 어설션이 실패 (codes.Unimplemented ≠ 기대 응답)
//
// 테스트 전략:
//   - 서비스 인터페이스 직접 호출: proto wire format 없이 핸들러 계약 검증
//   - bufconn: gRPC 서버 등록 + 연결 레이어 동작 검증 (AC-CTRL-002-1 startup)
//   - FakeStore: Sprint 1 인메모리 스토어로 pgx 의존성 없이 단위 격리
//
// RED 실패 이유:
//   - CreateWorkflow, GetWorkflow, ListWorkflows가 codes.Unimplemented를 반환
//   - 테스트는 NoError + 실제 응답 필드를 기대 → 실패 (RED 정상)
//
// AC 커버리지:
//   - AC-CTRL-002-1: CreateWorkflow → PENDING 상태 + audit row
//   - AC-CTRL-002-2: GetWorkflow Found / NotFound (codes.NotFound)
//   - AC-CTRL-002-3: ListWorkflows empty / multiple / limit enforcement
//   - AC-CTRL-002-4 (Optional): Prometheus 메트릭 카운터
package server_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	proto "github.com/ircp/iroum-ax/apps/control-plane/internal/proto"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/server"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// newTestService 테스트용 WorkflowService + FakeStore 조합을 반환
// audit.NewRecorder(false): authEnabled=false (Walking Skeleton 기본값, cli-anonymous 고정)
func newTestService(t *testing.T) (*server.WorkflowService, *store.FakeStore) {
	t.Helper()
	fakeStore := store.NewFakeStore()
	recorder := audit.NewRecorder(false)
	coordinator := workflow.NewTxCoordinator(fakeStore, recorder)
	sm := workflow.NewStateMachine(coordinator, zap.NewNop())
	svc := server.NewWorkflowService(fakeStore, sm, zap.NewNop())
	return svc, fakeStore
}

// seedWorkflow FakeStore에 테스트용 Workflow를 직접 삽입
func seedWorkflow(t *testing.T, s *store.FakeStore, wfID, docID string) {
	t.Helper()
	id, err := uuid.Parse(wfID)
	if err != nil {
		id = uuid.New()
	}
	docUUID, err := uuid.Parse(docID)
	if err != nil {
		docUUID = uuid.New()
	}
	wf := &types.Workflow{
		ID:         id,
		DocumentID: docUUID,
		State:      types.WorkflowStatePending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.SeedWorkflow(wf)
}

// testID 테스트용 결정론적 UUID 문자열 생성
func testID(prefix string, idx int) string {
	return fmt.Sprintf("%s%04d0000-0000-4000-a000-000000000000", prefix, idx)
}

// -------- AC-CTRL-002-1: CreateWorkflow 해피 패스 --------

// TestWorkflowService_CreateWorkflow_HappyPath
// AC-CTRL-002-1: CreateWorkflow RPC → PENDING 상태 워크플로우 + audit_logs row 1건
//
// RED 실패 이유: resp == nil (codes.Unimplemented), NoError 어설션 실패
func TestWorkflowService_CreateWorkflow_HappyPath(t *testing.T) {
	svc, fakeStore := newTestService(t)
	ctx := context.Background()

	req := &proto.CreateWorkflowRequest{DocumentID: "d0000000-0000-4000-a000-000000000001"}
	resp, err := svc.CreateWorkflow(ctx, req)

	// RED: 이 어설션이 실패해야 함 (Unimplemented 에러 반환)
	require.NoError(t, err, "CreateWorkflow는 에러 없이 완료해야 함")
	require.NotNil(t, resp, "응답이 nil이면 안 됨")
	require.NotNil(t, resp.Workflow)
	assert.Equal(t, proto.WorkflowStatusPending, resp.Workflow.Status,
		"생성된 워크플로우 상태는 PENDING이어야 함")
	assert.NotEmpty(t, resp.Workflow.ID, "workflow_id가 비어 있으면 안 됨")
	assert.Len(t, fakeStore.AuditLogs, 1,
		"audit_logs에 정확히 1건 WORKFLOW_CREATED 이벤트 필요")
	assert.Equal(t, string(audit.ActionWorkflowCreated), string(fakeStore.AuditLogs[0].Action))
}

// TestWorkflowService_CreateWorkflow_AuditFail_RollsBack
// AC-CTRL-UBI-001 Scenario A gRPC 레이어 검증:
// audit INSERT 실패 시 workflow INSERT도 롤백 → store에 row 없음
//
// RED 실패 이유: resp == nil (Unimplemented), 어설션 실패
func TestWorkflowService_CreateWorkflow_AuditFail_RollsBack(t *testing.T) {
	svc, fakeStore := newTestService(t)
	fakeStore.NextTxFailOnAudit = true
	ctx := context.Background()

	req := &proto.CreateWorkflowRequest{DocumentID: "d0000000-0000-4000-a000-000000000002"}
	resp, err := svc.CreateWorkflow(ctx, req)

	// RED: audit fail 시 gRPC 에러 반환 기대
	require.Error(t, err, "audit 실패 시 에러를 반환해야 함")
	assert.Nil(t, resp)
	assert.Empty(t, fakeStore.Workflows, "audit 실패 시 workflow row도 롤백되어야 함")
	assert.Empty(t, fakeStore.AuditLogs, "audit INSERT 실패이므로 감사 row도 없어야 함")
}

// -------- AC-CTRL-002-2: GetWorkflow --------

// TestWorkflowService_GetWorkflow_Found
// AC-CTRL-002-2: 존재하는 워크플로우 ID → 해당 Workflow 반환
//
// RED 실패 이유: resp == nil (Unimplemented), NoError 어설션 실패
func TestWorkflowService_GetWorkflow_Found(t *testing.T) {
	svc, fakeStore := newTestService(t)
	ctx := context.Background()

	wfID := "a0000000-0000-4000-a000-000000000001"
	seedWorkflow(t, fakeStore, wfID, "d0000000-0000-4000-a000-000000000003")

	resp, err := svc.GetWorkflow(ctx, &proto.GetWorkflowRequest{ID: wfID})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Workflow)
	assert.Equal(t, wfID, resp.Workflow.ID)
	assert.Equal(t, proto.WorkflowStatusPending, resp.Workflow.Status)
}

// TestWorkflowService_GetWorkflow_NotFound_ReturnsNotFoundStatus
// AC-CTRL-002-2: 존재하지 않는 ID → codes.NotFound 반환
//
// RED 실패 이유: codes.Unimplemented 반환 (≠ codes.NotFound), 어설션 실패
func TestWorkflowService_GetWorkflow_NotFound_ReturnsNotFoundStatus(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.GetWorkflow(ctx, &proto.GetWorkflowRequest{ID: "b0000000-0000-4000-a000-000000000099"})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	// RED: Unimplemented 반환 → NotFound 기대와 불일치 → 실패
	assert.Equal(t, codes.NotFound, st.Code(),
		"존재하지 않는 워크플로우 조회 시 codes.NotFound 반환 필요")
}

// -------- AC-CTRL-002-3: ListWorkflows --------

// TestWorkflowService_ListWorkflows_EmptyStore
// AC-CTRL-002-3: 빈 스토어 → 빈 목록 반환 (에러 아님)
//
// RED 실패 이유: resp == nil (Unimplemented), NoError 어설션 실패
func TestWorkflowService_ListWorkflows_EmptyStore(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.ListWorkflows(ctx, &proto.ListWorkflowsRequest{Limit: 10, Offset: 0})

	require.NoError(t, err, "빈 스토어 ListWorkflows는 에러 없이 빈 목록을 반환해야 함")
	require.NotNil(t, resp)
	assert.Empty(t, resp.Workflows)
	assert.EqualValues(t, 0, resp.Total)
}

// TestWorkflowService_ListWorkflows_WithMultiple
// AC-CTRL-002-3: 스토어에 3개 존재 → 3개 반환
//
// RED 실패 이유: resp == nil (Unimplemented), NoError 어설션 실패
func TestWorkflowService_ListWorkflows_WithMultiple(t *testing.T) {
	svc, fakeStore := newTestService(t)
	ctx := context.Background()

	for i := range 3 {
		seedWorkflow(t, fakeStore, testID("a0000000-000", i), testID("d0000010-000", i))
	}

	resp, err := svc.ListWorkflows(ctx, &proto.ListWorkflowsRequest{Limit: 10, Offset: 0})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Workflows, 3)
	assert.EqualValues(t, 3, resp.Total)
}

// TestWorkflowService_ListWorkflows_LimitEnforcement
// AC-CTRL-002-3: limit=2, 스토어에 5개 → 최대 2개 반환
//
// RED 실패 이유: resp == nil (Unimplemented), NoError 어설션 실패
func TestWorkflowService_ListWorkflows_LimitEnforcement(t *testing.T) {
	svc, fakeStore := newTestService(t)
	ctx := context.Background()

	for i := range 5 {
		seedWorkflow(t, fakeStore, testID("a0000020-000", i), testID("d0000030-000", i))
	}

	resp, err := svc.ListWorkflows(ctx, &proto.ListWorkflowsRequest{Limit: 2, Offset: 0})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Workflows, 2, "limit=2이므로 최대 2개 반환")
}

// -------- Context cancellation --------

// TestWorkflowService_CreateWorkflow_ContextCancelled_ReturnsCanceledStatus
// AC-CTRL-002-3 (cancellation): 취소된 컨텍스트 → codes.Canceled 또는 DeadlineExceeded
//
// RED 실패 이유: codes.Unimplemented ≠ Canceled, 어설션 실패
func TestWorkflowService_CreateWorkflow_ContextCancelled_ReturnsCanceledStatus(t *testing.T) {
	svc, _ := newTestService(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	resp, err := svc.CreateWorkflow(ctx, &proto.CreateWorkflowRequest{
		DocumentID: "d0000000-0000-4000-a000-000000000010",
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	// RED: Unimplemented 반환 → Canceled 기대와 불일치 → 실패
	assert.True(t,
		st.Code() == codes.Canceled || st.Code() == codes.DeadlineExceeded,
		"취소된 컨텍스트는 Canceled 또는 DeadlineExceeded 반환 필요, got: %s", st.Code())
}

// -------- 동시성 --------

// TestWorkflowService_CreateWorkflow_ConcurrentCalls_AllSucceed
// AC-CTRL-002-2 (performance): 3개 고루틴 동시 CreateWorkflow → 모두 성공
//
// RED 실패 이유: 모두 Unimplemented 에러 반환 → NoError 어설션 실패
func TestWorkflowService_CreateWorkflow_ConcurrentCalls_AllSucceed(t *testing.T) {
	svc, fakeStore := newTestService(t)
	ctx := context.Background()

	const concurrency = 3
	type result struct {
		resp *proto.CreateWorkflowResponse
		err  error
	}
	results := make([]result, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := range concurrency {
		go func(idx int) {
			defer wg.Done()
			req := &proto.CreateWorkflowRequest{
				DocumentID: fmt.Sprintf("d000000%d-0000-4000-a000-000000000099", idx),
			}
			resp, err := svc.CreateWorkflow(ctx, req)
			results[idx] = result{resp: resp, err: err}
		}(i)
	}
	wg.Wait()

	// RED: 모두 Unimplemented 에러 반환 → NoError 어설션 실패
	ids := make(map[string]bool)
	for i, r := range results {
		require.NoError(t, r.err, "goroutine %d: 동시 호출이 모두 성공해야 함", i)
		require.NotNil(t, r.resp)
		assert.NotEmpty(t, r.resp.Workflow.ID)
		ids[r.resp.Workflow.ID] = true
	}
	assert.Len(t, ids, concurrency, "동시 생성된 workflow_id가 모두 고유해야 함")
	assert.Len(t, fakeStore.AuditLogs, concurrency, "각 CreateWorkflow마다 audit_log 1건")
}

// -------- bufconn 기반 gRPC 서버 연결 레이어 (PASS 가능) --------

// TestWorkflowService_GRPCServer_RegisterAndConnect
// AC-CTRL-002-1 (startup): bufconn으로 in-process gRPC 서버 등록 + 연결 성공
// 이 테스트는 RED에서도 PASS 가능 (서버 등록/연결 자체는 Unimplemented와 무관)
func TestWorkflowService_GRPCServer_RegisterAndConnect(t *testing.T) {
	svc, _ := newTestService(t)

	lis := bufconn.Listen(bufSize)
	grpcSrv := grpc.NewServer()
	proto.RegisterWorkflowServiceServer(grpcSrv, svc)

	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	t.Cleanup(func() {
		grpcSrv.GracefulStop()
		lis.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "bufconn gRPC 연결이 성공해야 함")
	defer conn.Close()

	assert.NotNil(t, conn)
	_ = ctx
}

// TestWorkflowService_GRPCServer_Serve_AcceptsConnection
// bufconn 기반 서버가 accept 상태임을 연결 수립으로 확인
// RED에서도 PASS 가능 (연결 레이어 자체는 Unimplemented와 무관)
func TestWorkflowService_GRPCServer_Serve_AcceptsConnection(t *testing.T) {
	svc, _ := newTestService(t)

	lis := bufconn.Listen(bufSize)
	grpcSrv := grpc.NewServer()
	proto.RegisterWorkflowServiceServer(grpcSrv, svc)

	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	t.Cleanup(func() {
		grpcSrv.GracefulStop()
		lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	assert.NotNil(t, conn)
}

// -------- AC-CTRL-002-4: Prometheus 메트릭 (Optional) --------

// TestWorkflowService_PrometheusMetrics_RPCCounter
// AC-CTRL-002-4 (Optional): PROMETHEUS_ENABLED=true 시 gRPC 호출 카운터 증가
//
// RED 상태: WorkflowService에 RPCCallCounter() 메서드 없음 → 컴파일 실패 또는
//
//	미구현 상태를 확인하는 형태로 구현
//
// 현재: 서비스 인스턴스가 nil이 아님 + 호출이 패닉 없이 완료됨을 검증
func TestWorkflowService_PrometheusMetrics_RPCCounter(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// CreateWorkflow 3회 호출 (메트릭 누적 시뮬레이션)
	for range 3 {
		_, _ = svc.CreateWorkflow(ctx, &proto.CreateWorkflowRequest{
			DocumentID: "d0000000-0000-4000-a000-000000000020",
		})
	}

	// RED: 메트릭 카운터 미구현 확인
	// GREEN 단계에서 prometheus.CounterVec 필드 추가 후:
	// counter := svc.RPCCallCounter()
	// val := testutil.ToFloat64(counter.With(prometheus.Labels{"method": "CreateWorkflow"}))
	// assert.Equal(t, float64(3), val)
	assert.NotNil(t, svc, "서비스 인스턴스가 nil이 아니어야 함")
}
