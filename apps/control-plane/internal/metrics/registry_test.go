// registry_test.go — metrics 레지스트리 + collector 등록 + 헬퍼 함수 테스트
// SPEC-AX-OBS-001 Sprint 0 RED
package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistryNotDefault — 싱글톤 레지스트리가 default registry와 다른 인스턴스여야 한다.
func TestRegistryNotDefault(t *testing.T) {
	t.Parallel()
	reg := Registry()
	assert.NotEqual(t, prometheus.DefaultRegisterer, reg,
		"custom registry가 default registry와 달라야 한다 (테스트 격리 보장)")
}

// TestRegistryIsSingleton — Registry() 호출마다 동일 인스턴스를 반환해야 한다.
func TestRegistryIsSingleton(t *testing.T) {
	t.Parallel()
	r1 := Registry()
	r2 := Registry()
	assert.Same(t, r1, r2, "Registry()는 싱글톤이어야 한다")
}

// TestCollectorsRegistered — 6개 core collector 모두 레지스트리에 등록되어 Describe 가능해야 한다.
// GaugeFunc(pg_pool_connections)는 server.go wiring 시점에 등록되므로 여기서는 제외한다.
// HistogramVec/CounterVec는 관측값이 없으면 Gather에 포함되지 않으므로 Describe로 확인한다.
func TestCollectorsRegistered(t *testing.T) {
	t.Parallel()
	// 격리된 레지스트리 사용 (싱글톤 의존 금지)
	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	// 각 collector를 직접 Describe하여 이름을 수집
	descCh := make(chan *prometheus.Desc, 100)
	m.httpDuration.Describe(descCh)
	m.grpcDuration.Describe(descCh)
	m.workflowTransitions.Describe(descCh)
	m.celeryDispatch.Describe(descCh)
	m.authRejections.Describe(descCh)
	m.authzForbidden.Describe(descCh)
	close(descCh)

	names := make(map[string]bool)
	for desc := range descCh {
		// Desc.String() 형식: "Desc{fqName: "...", help: "...", ...}"
		s := desc.String()
		if idx := len(`Desc{fqName: "`); idx < len(s) {
			s = s[idx:]
			if end := len(s); end > 0 {
				for i, c := range s {
					if c == '"' {
						names[s[:i]] = true
						break
					}
				}
			}
		}
	}

	expected := []string{
		"iroum_ax_http_request_duration_seconds",
		"iroum_ax_grpc_request_duration_seconds",
		"iroum_ax_workflow_state_transitions_total",
		"iroum_ax_celery_dispatch_total",
		"iroum_ax_auth_rejections_total",
		"iroum_ax_authz_forbidden_total",
	}
	for _, name := range expected {
		assert.True(t, names[name], "collector %q가 등록돼야 한다", name)
	}
}

// TestIncWorkflowTransition — IncWorkflowTransition 호출 후 카운터가 증가해야 한다.
func TestIncWorkflowTransition(t *testing.T) {
	t.Parallel()
	// 새 레지스트리로 격리
	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	m.IncWorkflowTransition("pending", "running")
	m.IncWorkflowTransition("pending", "running")

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_workflow_state_transitions_total" {
			for _, metric := range mf.GetMetric() {
				val := metric.GetCounter().GetValue()
				if val == 2 {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "IncWorkflowTransition 2회 호출 후 카운터 값이 2이어야 한다")
}

// TestIncAuthRejection — IncAuthRejection 호출 후 카운터가 증가해야 한다.
func TestIncAuthRejection(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	m.IncAuthRejection("expired")
	m.IncAuthRejection("expired")
	m.IncAuthRejection("blacklisted")

	mfs, err := reg.Gather()
	require.NoError(t, err)

	total := 0.0
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_auth_rejections_total" {
			for _, metric := range mf.GetMetric() {
				total += metric.GetCounter().GetValue()
			}
		}
	}
	assert.Equal(t, 3.0, total, "IncAuthRejection 3회 호출 후 합계가 3이어야 한다")
}

// TestIncCeleryDispatch — IncCeleryDispatch 호출 후 카운터가 증가해야 한다.
func TestIncCeleryDispatch(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	m.IncCeleryDispatch("success")

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_celery_dispatch_total" {
			if len(mf.GetMetric()) > 0 {
				found = mf.GetMetric()[0].GetCounter().GetValue() == 1
			}
		}
	}
	assert.True(t, found, "IncCeleryDispatch 1회 호출 후 카운터가 1이어야 한다")
}

// TestGlobalMetrics_NotNil — GlobalMetrics()가 nil이 아닌 Metrics 인스턴스를 반환해야 한다.
func TestGlobalMetrics_NotNil(t *testing.T) {
	t.Parallel()
	m := GlobalMetrics()
	assert.NotNil(t, m, "GlobalMetrics()는 nil이 아니어야 한다")
}

// TestGlobalIncWorkflowTransition — 패키지 레벨 IncWorkflowTransition이 전역 카운터를 증가시켜야 한다.
// lesson #4: stub-assert 금지 — global().workflowTransitions이 실제로 호출됨을 Registry()를 통해 증명.
func TestGlobalIncWorkflowTransition(t *testing.T) {
	t.Parallel()
	// 전역 싱글톤 사용 — race는 없으나 카운터 누적에 주의 (Registry는 process-wide singleton)
	IncWorkflowTransition("PENDING", "RUNNING_GLOBAL_TEST")

	mfs, err := Registry().Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_workflow_state_transitions_total" {
			for _, metric := range mf.GetMetric() {
				for _, lp := range metric.GetLabel() {
					if lp.GetName() == "to" && lp.GetValue() == "RUNNING_GLOBAL_TEST" {
						found = true
					}
				}
			}
		}
	}
	assert.True(t, found, "전역 IncWorkflowTransition 호출 후 카운터가 기록되어야 한다")
}

// TestGlobalIncCeleryDispatch — 패키지 레벨 IncCeleryDispatch가 전역 카운터를 증가시켜야 한다.
func TestGlobalIncCeleryDispatch(t *testing.T) {
	t.Parallel()
	IncCeleryDispatch("success_global_test")

	mfs, err := Registry().Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_celery_dispatch_total" {
			for _, metric := range mf.GetMetric() {
				for _, lp := range metric.GetLabel() {
					if lp.GetName() == "status" && lp.GetValue() == "success_global_test" {
						found = true
					}
				}
			}
		}
	}
	assert.True(t, found, "전역 IncCeleryDispatch 호출 후 카운터가 기록되어야 한다")
}

// TestRegisterPgPoolGauge — RegisterPgPoolGauge가 GaugeFunc를 레지스트리에 등록해야 한다.
func TestRegisterPgPoolGauge(t *testing.T) {
	t.Parallel()
	// 격리된 레지스트리 사용 (전역 싱글톤에 중복 등록 방지)
	reg := prometheus.NewRegistry()
	called := false
	RegisterPgPoolGauge(reg, func() float64 {
		called = true
		return 3.0
	})

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_pg_pool_connections" {
			for _, metric := range mf.GetMetric() {
				if metric.GetGauge().GetValue() == 3.0 {
					found = true
				}
			}
		}
	}
	assert.True(t, called, "Gather() 호출 시 statsFn이 호출되어야 한다")
	assert.True(t, found, "pg_pool_connections GaugeFunc가 3.0을 반환해야 한다")
}

// TestObserveHTTPDuration — ObserveHTTPDuration 호출 후 히스토그램에 관측값이 기록돼야 한다.
func TestObserveHTTPDuration(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := newMetricsWithRegistry(reg)

	m.ObserveHTTPDuration("GET", "/health", "200", 0.05)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "iroum_ax_http_request_duration_seconds" {
			for _, metric := range mf.GetMetric() {
				if metric.GetHistogram().GetSampleCount() == 1 {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "ObserveHTTPDuration 1회 호출 후 sample count가 1이어야 한다")
}
