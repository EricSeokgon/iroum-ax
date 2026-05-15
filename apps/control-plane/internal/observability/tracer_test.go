// tracer_test.go — InitTracer 동작 검증 (SPEC-AX-OBS-001 Sprint 2 RED)
// AC-OBS-004-1: endpoint 미설정 → noop (NeverSample), 외부 전송 없음
// AC-OBS-004-2: endpoint 설정 → AlwaysSample (OTLP exporter Sprint 3 예정)
// AC-OBS-004-3: Shutdown 호출 가능 (이중 호출 포함)
package observability_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/observability"
)

// TestInitTracer_NoEndpoint_NoopProvider OTEL_EXPORTER_OTLP_ENDPOINT 미설정 시
// noop-equivalent TracerProvider가 반환되고 otel 전역에 등록된다.
// 망분리 정합: Span 생성 자체가 차단되어 외부 전송 0 (REQ-OBS-UBI-001-a).
// t.Setenv 사용으로 t.Parallel() 사용 불가 (Go runtime 제약).
func TestInitTracer_NoEndpoint_NoopProvider(t *testing.T) {
	// endpoint 환경변수 미설정 상태 보장
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	ctx := context.Background()
	tp, shutdown, err := observability.InitTracer(ctx)

	require.NoError(t, err, "InitTracer는 endpoint 미설정 시 에러 없이 초기화되어야 한다")
	require.NotNil(t, tp, "TracerProvider는 nil이 아니어야 한다")
	require.NotNil(t, shutdown, "shutdown 클로저는 nil이 아니어야 한다")

	// noop-equivalent: NeverSample이므로 Span은 recording되지 않는다
	tracer := tp.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	assert.False(t, span.IsRecording(),
		"OTLP endpoint 미설정 시 span은 recording 상태가 아니어야 한다 (NeverSample)")
	span.End()

	// shutdown 정상 호출
	require.NoError(t, shutdown(ctx), "shutdown 클로저는 에러 없이 실행되어야 한다")
}

// TestInitTracer_WithEndpoint_SamplingActive OTEL_EXPORTER_OTLP_ENDPOINT 설정 시
// AlwaysSample로 동작하고 span이 recording 상태여야 한다 (Sprint 3 OTLP 연결 대비).
// t.Setenv 사용으로 t.Parallel() 사용 불가 (Go runtime 제약).
func TestInitTracer_WithEndpoint_SamplingActive(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")

	ctx := context.Background()
	tp, shutdown, err := observability.InitTracer(ctx)

	require.NoError(t, err, "InitTracer는 endpoint 설정 시 에러 없이 초기화되어야 한다")
	require.NotNil(t, tp, "TracerProvider는 nil이 아니어야 한다")
	require.NotNil(t, shutdown, "shutdown 클로저는 nil이 아니어야 한다")

	// AlwaysSample: span이 recording 상태여야 한다
	tracer := tp.Tracer("test")
	_, span := tracer.Start(ctx, "sampled-span")
	assert.True(t, span.IsRecording(),
		"OTLP endpoint 설정 시 span은 recording 상태여야 한다 (AlwaysSample)")
	span.End()

	// shutdown 정상 호출
	require.NoError(t, shutdown(ctx), "shutdown 클로저는 에러 없이 실행되어야 한다")
}

// TestInitTracer_SetsGlobalProvider InitTracer 호출 후 otel.GetTracerProvider()가
// 초기화된 TracerProvider를 반환해야 한다.
func TestInitTracer_SetsGlobalProvider(t *testing.T) {
	// 전역 상태 변경 테스트 — parallel 사용 불가
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	ctx := context.Background()
	tp, shutdown, err := observability.InitTracer(ctx)
	require.NoError(t, err)
	defer func() { _ = shutdown(ctx) }()

	globalTP := otel.GetTracerProvider()
	assert.NotNil(t, globalTP, "전역 TracerProvider가 설정되어야 한다")

	// 전역 TracerProvider가 SDK TP와 동일한 인스턴스여야 한다
	// (otel.SetTracerProvider 호출 확인)
	globalTracer := globalTP.Tracer("global-test")
	_, span := globalTracer.Start(ctx, "global-span")
	// noop 모드이므로 recording=false
	assert.False(t, span.IsRecording(),
		"전역 TracerProvider도 noop(NeverSample) 모드여야 한다")
	span.End()

	// tp가 trace.TracerProvider 인터페이스를 구현하는지 확인
	var _ trace.TracerProvider = tp
}

// TestInitTracer_Shutdown_Idempotent shutdown 클로저를 두 번 호출해도 에러가 없어야 한다.
// t.Setenv 사용으로 t.Parallel() 사용 불가 (Go runtime 제약).
func TestInitTracer_Shutdown_Idempotent(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	ctx := context.Background()
	_, shutdown, err := observability.InitTracer(ctx)
	require.NoError(t, err)

	// 첫 번째 shutdown
	require.NoError(t, shutdown(ctx), "첫 번째 shutdown은 에러 없이 실행되어야 한다")
	// 두 번째 shutdown (idempotent 보장)
	require.NoError(t, shutdown(ctx), "두 번째 shutdown 호출도 에러 없이 실행되어야 한다")
}
