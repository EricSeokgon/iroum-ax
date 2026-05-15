// interceptor_test.go — UnaryMetricsInterceptor 동작 검증
// SPEC-AX-OBS-001 DISPUTE #8: coverage ≥85%, lesson #4 — 실제 동작 assert (stub-assert 금지)
// AC-OBS-001-3: gRPC 요청별 레이블(method, code) 관찰 가능
package metrics_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/metrics"
)

// TestUnaryMetricsInterceptor_SuccessObserved gRPC OK 응답(코드=0)이 히스토그램에 기록되어야 한다.
// AC-OBS-001-3: registry.Gather로 iroum_ax_grpc_request_duration_seconds 샘플 관측 증명.
func TestUnaryMetricsInterceptor_SuccessObserved(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := metrics.NewMetricsWithRegistry(reg)

	interceptor := metrics.UnaryMetricsInterceptor(m)

	// OK 핸들러
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/iroum.ax.v1.WorkflowService/GetWorkflow"}
	_, err := interceptor(context.Background(), nil, info, handler)
	require.NoError(t, err)

	mfs, gErr := reg.Gather()
	require.NoError(t, gErr)

	found := findMetricFamily(mfs, "iroum_ax_grpc_request_duration_seconds")
	require.NotNil(t, found, "iroum_ax_grpc_request_duration_seconds 메트릭이 존재해야 한다")
	require.NotEmpty(t, found.GetMetric(), "히스토그램에 관찰값이 있어야 한다")

	// 레이블 검증 (method=/iroum.ax.v1.WorkflowService/GetWorkflow, code=0(OK))
	m0 := found.GetMetric()[0]
	assertLabel(t, m0.GetLabel(), "method", "/iroum.ax.v1.WorkflowService/GetWorkflow")
	assertLabel(t, m0.GetLabel(), "code", "0") // codes.OK = 0

	assert.GreaterOrEqual(t, m0.GetHistogram().GetSampleCount(), uint64(1),
		"최소 1개의 샘플이 기록되어야 한다")
}

// TestUnaryMetricsInterceptor_ErrorObserved gRPC 에러 응답(NotFound=5)이 올바른 code 레이블로 기록되어야 한다.
func TestUnaryMetricsInterceptor_ErrorObserved(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := metrics.NewMetricsWithRegistry(reg)

	interceptor := metrics.UnaryMetricsInterceptor(m)

	// NotFound 에러 핸들러
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.NotFound, "workflow not found")
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/iroum.ax.v1.WorkflowService/GetWorkflow"}
	_, err := interceptor(context.Background(), nil, info, handler)
	require.Error(t, err)

	mfs, gErr := reg.Gather()
	require.NoError(t, gErr)

	found := findMetricFamily(mfs, "iroum_ax_grpc_request_duration_seconds")
	require.NotNil(t, found)
	require.NotEmpty(t, found.GetMetric())

	// codes.NotFound = 5
	m0 := found.GetMetric()[0]
	assertLabel(t, m0.GetLabel(), "code", "5")
}

// TestUnaryMetricsInterceptor_MultipleRequests 여러 요청이 각각 독립 레이블로 기록되어야 한다.
func TestUnaryMetricsInterceptor_MultipleRequests(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := metrics.NewMetricsWithRegistry(reg)

	interceptor := metrics.UnaryMetricsInterceptor(m)

	okHandler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	info1 := &grpc.UnaryServerInfo{FullMethod: "/iroum.ax.v1.WorkflowService/CreateWorkflow"}
	info2 := &grpc.UnaryServerInfo{FullMethod: "/iroum.ax.v1.WorkflowService/ListWorkflows"}

	_, _ = interceptor(context.Background(), nil, info1, okHandler)
	_, _ = interceptor(context.Background(), nil, info1, okHandler)
	_, _ = interceptor(context.Background(), nil, info2, okHandler)

	mfs, gErr := reg.Gather()
	require.NoError(t, gErr)

	found := findMetricFamily(mfs, "iroum_ax_grpc_request_duration_seconds")
	require.NotNil(t, found)

	// CreateWorkflow = 2건, ListWorkflows = 1건
	var createCount, listCount uint64
	for _, metric := range found.GetMetric() {
		for _, lp := range metric.GetLabel() {
			if lp.GetName() == "method" {
				switch lp.GetValue() {
				case "/iroum.ax.v1.WorkflowService/CreateWorkflow":
					createCount += metric.GetHistogram().GetSampleCount()
				case "/iroum.ax.v1.WorkflowService/ListWorkflows":
					listCount += metric.GetHistogram().GetSampleCount()
				}
			}
		}
	}

	assert.Equal(t, uint64(2), createCount, "CreateWorkflow 요청이 2번 기록되어야 한다")
	assert.Equal(t, uint64(1), listCount, "ListWorkflows 요청이 1번 기록되어야 한다")
}
