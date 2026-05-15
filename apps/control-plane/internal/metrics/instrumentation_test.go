// instrumentation_test.go — HTTP 계측 미들웨어 + workflow/celery 전이 계측 검증
// SPEC-AX-OBS-001 Sprint 2 RED: instrumentation hooks 동작 확인
// AC-OBS-002-2: HTTP 요청별 레이블(method, path, status) 관찰 가능
// AC-OBS-003-1: workflow 전이 카운터 증가
// AC-OBS-003-2: celery dispatch 카운터 증가
package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/metrics"
)

// ── HTTP 계측 미들웨어 테스트 ──────────────────────────────────────────────────

// TestHTTPInstrumentationMiddleware_Records200 HTTPInstrumentationMiddleware가
// HTTP 200 응답에 대해 히스토그램 관찰을 기록해야 한다 (AC-OBS-002-2).
func TestHTTPInstrumentationMiddleware_Records200(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := metrics.NewMetricsWithRegistry(reg)

	// 200 OK 핸들러
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := metrics.HTTPInstrumentationMiddleware(m)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// 히스토그램에 관찰값이 기록되었는지 확인
	mfs, err := reg.Gather()
	require.NoError(t, err)

	found := findMetricFamily(mfs, "iroum_ax_http_request_duration_seconds")
	require.NotNil(t, found, "iroum_ax_http_request_duration_seconds 메트릭이 존재해야 한다")
	require.NotEmpty(t, found.GetMetric(), "히스토그램에 관찰값이 있어야 한다")

	// 레이블 검증 (method=GET, status=200)
	m0 := found.GetMetric()[0]
	assertLabel(t, m0.GetLabel(), "method", "GET")
	assertLabel(t, m0.GetLabel(), "status", "200")
}

// TestHTTPInstrumentationMiddleware_Records404 404 응답도 계측되어야 한다.
func TestHTTPInstrumentationMiddleware_Records404(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := metrics.NewMetricsWithRegistry(reg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})

	wrapped := metrics.HTTPInstrumentationMiddleware(m)(handler)

	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	found := findMetricFamily(mfs, "iroum_ax_http_request_duration_seconds")
	require.NotNil(t, found)
	require.NotEmpty(t, found.GetMetric())

	m0 := found.GetMetric()[0]
	assertLabel(t, m0.GetLabel(), "status", "404")
}

// ── 워크플로우 전이 카운터 테스트 ───────────────────────────────────────────

// TestIncWorkflowTransition_IncreasesCounter PENDING→RUNNING 전이 시
// workflow_state_transitions_total 카운터가 증가해야 한다 (AC-OBS-003-1).
func TestIncWorkflowTransition_IncreasesCounter(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := metrics.NewMetricsWithRegistry(reg)

	m.IncWorkflowTransition("PENDING", "RUNNING")
	m.IncWorkflowTransition("RUNNING", "COMPLETED")
	m.IncWorkflowTransition("PENDING", "RUNNING") // 두 번째 PENDING→RUNNING

	mfs, err := reg.Gather()
	require.NoError(t, err)

	found := findMetricFamily(mfs, "iroum_ax_workflow_state_transitions_total")
	require.NotNil(t, found, "iroum_ax_workflow_state_transitions_total 메트릭이 존재해야 한다")

	// PENDING→RUNNING = 2, RUNNING→COMPLETED = 1
	pendingRunning := findCounterByLabels(found.GetMetric(), map[string]string{"from": "PENDING", "to": "RUNNING"})
	assert.NotNil(t, pendingRunning, "PENDING→RUNNING 레이블 조합이 있어야 한다")
	assert.Equal(t, float64(2), pendingRunning.GetCounter().GetValue(),
		"PENDING→RUNNING 전이는 2번 계측되어야 한다")

	runningCompleted := findCounterByLabels(found.GetMetric(), map[string]string{"from": "RUNNING", "to": "COMPLETED"})
	assert.NotNil(t, runningCompleted, "RUNNING→COMPLETED 레이블 조합이 있어야 한다")
	assert.Equal(t, float64(1), runningCompleted.GetCounter().GetValue(),
		"RUNNING→COMPLETED 전이는 1번 계측되어야 한다")
}

// ── Celery dispatch 카운터 테스트 ───────────────────────────────────────────

// TestIncCeleryDispatch_Success_IncreasesCounter dispatch 성공 시
// celery_dispatch_total{status="success"} 카운터가 증가해야 한다 (AC-OBS-003-2).
func TestIncCeleryDispatch_Success_IncreasesCounter(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := metrics.NewMetricsWithRegistry(reg)

	m.IncCeleryDispatch("success")
	m.IncCeleryDispatch("success")
	m.IncCeleryDispatch("failure")

	mfs, err := reg.Gather()
	require.NoError(t, err)

	found := findMetricFamily(mfs, "iroum_ax_celery_dispatch_total")
	require.NotNil(t, found, "iroum_ax_celery_dispatch_total 메트릭이 존재해야 한다")

	successMetric := findCounterByLabels(found.GetMetric(), map[string]string{"status": "success"})
	require.NotNil(t, successMetric, "success 레이블이 있어야 한다")
	assert.Equal(t, float64(2), successMetric.GetCounter().GetValue(),
		"dispatch 성공이 2번 계측되어야 한다")

	failureMetric := findCounterByLabels(found.GetMetric(), map[string]string{"status": "failure"})
	require.NotNil(t, failureMetric, "failure 레이블이 있어야 한다")
	assert.Equal(t, float64(1), failureMetric.GetCounter().GetValue(),
		"dispatch 실패가 1번 계측되어야 한다")
}

// ── 테스트 헬퍼 ─────────────────────────────────────────────────────────────

// findMetricFamily Gather() 결과에서 이름으로 MetricFamily를 찾는다.
func findMetricFamily(mfs []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

// findCounterByLabels 레이블 맵과 일치하는 Metric을 찾는다.
func findCounterByLabels(metrics []*dto.Metric, labels map[string]string) *dto.Metric {
	for _, m := range metrics {
		if labelsMatch(m.GetLabel(), labels) {
			return m
		}
	}
	return nil
}

// labelsMatch 주어진 레이블 쌍이 모두 dto.LabelPair 슬라이스에 포함되는지 확인한다.
func labelsMatch(pairs []*dto.LabelPair, want map[string]string) bool {
	found := 0
	for _, lp := range pairs {
		if v, ok := want[lp.GetName()]; ok && v == lp.GetValue() {
			found++
		}
	}
	return found == len(want)
}

// assertLabel 레이블 슬라이스에서 특정 name=value 쌍이 존재하는지 검증한다.
func assertLabel(t *testing.T, pairs []*dto.LabelPair, name, value string) {
	t.Helper()
	for _, lp := range pairs {
		if lp.GetName() == name {
			assert.Equal(t, value, lp.GetValue(), "레이블 %q의 값이 일치해야 한다", name)
			return
		}
	}
	t.Errorf("레이블 %q를 찾을 수 없다", name)
}
