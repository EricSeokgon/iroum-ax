// registry.go — Prometheus 메트릭 레지스트리 싱글톤 + 7개 core collector
// SPEC-AX-OBS-001 Sprint 0 GREEN: REQ-OBS-001
// default registry 미사용 — 테스트 격리 보장
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// defaultBuckets — HTTP/gRPC latency 히스토그램 버킷 (초 단위)
// tech.md §11.2: API 응답 목표 200ms → p50/p95/p99 구분 가능한 버킷
var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

// Metrics — 레지스트리 + 7개 collector 보유 구조체
// fieldalignment-friendly: 포인터 타입 우선 배치
//
// @MX:ANCHOR: [AUTO] 메트릭 collector 단일 보유 구조체
// @MX:REASON: registry, HTTP/gRPC/workflow/celery/auth/authz collector 7개를 단일 지점에서 관리 (fan_in >= 3: server.go, tests, observer)
type Metrics struct {
	// reg — custom prometheus 레지스트리 (default registry 미사용)
	reg *prometheus.Registry
	// httpDuration — HTTP 요청 레이턴시 히스토그램 (method, path, status)
	httpDuration *prometheus.HistogramVec
	// grpcDuration — gRPC 요청 레이턴시 히스토그램 (method, code)
	grpcDuration *prometheus.HistogramVec
	// workflowTransitions — 워크플로우 상태 전이 카운터 (from, to)
	workflowTransitions *prometheus.CounterVec
	// celeryDispatch — Celery 태스크 디스패치 카운터 (status)
	celeryDispatch *prometheus.CounterVec
	// authRejections — JWT 검증 실패 카운터 (reason)
	authRejections *prometheus.CounterVec
	// authzForbidden — 인가 거부 카운터 (role, method)
	authzForbidden *prometheus.CounterVec
}

// 싱글톤 인스턴스
var (
	globalMetrics *Metrics
	once          sync.Once
)

// Registry — 전역 prometheus.Registry를 반환한다 (싱글톤).
//
// @MX:ANCHOR: [AUTO] 전역 메트릭 레지스트리 단일 접근점
// @MX:REASON: MetricsHandler, server.go wiring, 테스트에서 호출 (fan_in >= 3)
func Registry() *prometheus.Registry {
	return global().reg
}

// global — 전역 Metrics 싱글톤을 반환한다 (lazy init).
func global() *Metrics {
	once.Do(func() {
		globalMetrics = newMetricsWithRegistry(prometheus.NewRegistry())
	})
	return globalMetrics
}

// newMetricsWithRegistry — 지정된 레지스트리로 Metrics를 생성하고 7개 collector를 등록한다.
// 테스트 격리 시 prometheus.NewRegistry()를 주입하여 사용한다.
//
// @MX:NOTE: [AUTO] GaugeFunc(pg_pool_connections)는 server.go wiring 시점에 별도 등록 — registry.go에서는 나머지 6개를 등록한다
func newMetricsWithRegistry(reg *prometheus.Registry) *Metrics {
	m := &Metrics{reg: reg}

	// HTTP 요청 레이턴시 (method, path, status)
	m.httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "iroum_ax_http_request_duration_seconds",
		Help:    "HTTP 요청 처리 시간 (초 단위)",
		Buckets: defaultBuckets,
	}, []string{"method", "path", "status"})

	// gRPC 요청 레이턴시 (method, code)
	m.grpcDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "iroum_ax_grpc_request_duration_seconds",
		Help:    "gRPC 요청 처리 시간 (초 단위)",
		Buckets: defaultBuckets,
	}, []string{"method", "code"})

	// 워크플로우 상태 전이 카운터 (from, to)
	m.workflowTransitions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iroum_ax_workflow_state_transitions_total",
		Help: "워크플로우 상태 전이 횟수",
	}, []string{"from", "to"})

	// Celery 태스크 디스패치 카운터 (status)
	m.celeryDispatch = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iroum_ax_celery_dispatch_total",
		Help: "Celery 태스크 디스패치 횟수",
	}, []string{"status"})

	// JWT 검증 실패 카운터 (reason)
	m.authRejections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iroum_ax_auth_rejections_total",
		Help: "JWT 검증 실패 횟수 (reason 레이블로 분류)",
	}, []string{"reason"})

	// 인가 거부 카운터 (role, method)
	m.authzForbidden = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iroum_ax_authz_forbidden_total",
		Help: "RBAC 인가 거부 횟수 (role, method 레이블로 분류)",
	}, []string{"role", "method"})

	// 6개 collector 레지스트리 등록
	reg.MustRegister(
		m.httpDuration,
		m.grpcDuration,
		m.workflowTransitions,
		m.celeryDispatch,
		m.authRejections,
		m.authzForbidden,
	)

	return m
}

// RegisterPgPoolGauge — PgWorkflowStore.PoolStats를 GaugeFunc로 레지스트리에 등록한다.
// server.go에서 pgStore 초기화 이후 호출한다.
func RegisterPgPoolGauge(reg *prometheus.Registry, statsFn func() float64) {
	gauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "iroum_ax_pg_pool_connections",
		Help: "PostgreSQL 연결 풀 사용 중인 연결 수",
	}, statsFn)
	// 이미 등록된 경우 무시 (서버 재시작 없는 테스트 환경 대응)
	_ = reg.Register(gauge) //nolint:errcheck // 중복 등록은 무시
}

// ────────────────────────────────────────────────────────────
// 헬퍼 메서드 — collector 조작 (전역 싱글톤 위임)
// ────────────────────────────────────────────────────────────

// ObserveHTTPDuration — HTTP 요청 레이턴시를 히스토그램에 기록한다.
func (m *Metrics) ObserveHTTPDuration(method, path, status string, seconds float64) {
	m.httpDuration.WithLabelValues(method, path, status).Observe(seconds)
}

// ObserveGRPCDuration — gRPC 요청 레이턴시를 히스토그램에 기록한다.
func (m *Metrics) ObserveGRPCDuration(method, code string, seconds float64) {
	m.grpcDuration.WithLabelValues(method, code).Observe(seconds)
}

// IncWorkflowTransition — 워크플로우 상태 전이 카운터를 1 증가시킨다.
func (m *Metrics) IncWorkflowTransition(from, to string) {
	m.workflowTransitions.WithLabelValues(from, to).Inc()
}

// IncCeleryDispatch — Celery 디스패치 카운터를 1 증가시킨다.
func (m *Metrics) IncCeleryDispatch(status string) {
	m.celeryDispatch.WithLabelValues(status).Inc()
}

// IncAuthRejection — JWT 검증 실패 카운터를 1 증가시킨다.
// auth.RejectionObserver 인터페이스를 구조적으로 만족한다.
func (m *Metrics) IncAuthRejection(reason string) {
	m.authRejections.WithLabelValues(reason).Inc()
}

// IncAuthzForbidden — 인가 거부 카운터를 1 증가시킨다.
func (m *Metrics) IncAuthzForbidden(role, method string) {
	m.authzForbidden.WithLabelValues(role, method).Inc()
}

// ────────────────────────────────────────────────────────────
// 전역 싱글톤 헬퍼 — 외부 패키지(workflow, scheduler)에서 직접 호출용
// ────────────────────────────────────────────────────────────

// IncWorkflowTransition — 전역 싱글톤의 워크플로우 전이 카운터를 증가시킨다.
func IncWorkflowTransition(from, to string) {
	global().IncWorkflowTransition(from, to)
}

// IncCeleryDispatch — 전역 싱글톤의 Celery 디스패치 카운터를 증가시킨다.
func IncCeleryDispatch(status string) {
	global().IncCeleryDispatch(status)
}

// GlobalMetrics — 전역 Metrics 인스턴스를 반환한다 (server.go wiring용).
func GlobalMetrics() *Metrics {
	return global()
}

// NewMetricsWithRegistry — 지정된 레지스트리로 Metrics를 생성한다 (테스트 격리용 exported wrapper).
// 테스트에서 prometheus.NewRegistry()를 주입하여 싱글톤과 독립적으로 사용한다.
func NewMetricsWithRegistry(reg *prometheus.Registry) *Metrics {
	return newMetricsWithRegistry(reg)
}
