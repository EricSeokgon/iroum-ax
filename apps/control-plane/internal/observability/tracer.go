// tracer.go — OpenTelemetry TracerProvider 초기화 (skeleton)
// SPEC-AX-OBS-001 Sprint 2 GREEN: REQ-OBS-004
// 기본: noop (외부 전송 0, 망분리 정합)
// OTEL_EXPORTER_OTLP_ENDPOINT 환경변수 설정 시 OTLP exporter opt-in
package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

// InitTracer — OTel TracerProvider를 초기화한다.
//
// 동작 규칙:
//   - OTEL_EXPORTER_OTLP_ENDPOINT 미설정(default): noop SDK TracerProvider (외부 전송 없음)
//   - OTEL_EXPORTER_OTLP_ENDPOINT 설정 시: SDK TracerProvider (OTLP exporter는 Sprint 3에서 연결)
//
// 반환값:
//   - tp: 초기화된 TracerProvider (caller가 otel.SetTracerProvider로 전역 등록)
//   - shutdown: graceful shutdown 클로저 (server.go의 defer 체인에 등록)
//   - error: 초기화 실패 시 non-nil
//
// 망분리 정합: 기본 exporter는 noop — K8s 내부 OTLP endpoint 환경변수 없이 외부 망 접속 0 (REQ-OBS-UBI-001-a).
//
// @MX:ANCHOR: [AUTO] OTel TracerProvider 초기화 단일 진입점
// @MX:REASON: server.go + 테스트에서 호출 — 전역 otel.SetTracerProvider 등록의 유일한 지점 (fan_in >= 2)
func InitTracer(ctx context.Context) (tp *trace.TracerProvider, shutdown func(context.Context) error, err error) {
	// OTLP endpoint 환경변수 확인 (망분리 opt-in gate)
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	var opts []trace.TracerProviderOption

	if endpoint == "" {
		// noop 모드: AlwaysOffSampler로 span 생성 자체를 차단 (0 overhead)
		// @MX:NOTE: [AUTO] noop은 sdk.NewTracerProvider + AlwaysOff — 진짜 noop provider는 otel/trace.NewNoopTracerProvider()이나
		// SDK TP를 사용해야 Shutdown() 계약이 일관됨
		opts = append(opts, trace.WithSampler(trace.NeverSample()))
	} else {
		// endpoint 설정 시: AlwaysSample (Sprint 3에서 OTLP exporter 연결 예정)
		// @MX:NOTE: [AUTO] OTLP exporter wire는 Sprint 3 범위 — 현재는 sampler만 설정하고 span은 in-memory에 보관됨
		opts = append(opts, trace.WithSampler(trace.AlwaysSample()))
	}

	sdkTP := trace.NewTracerProvider(opts...)

	// 전역 TracerProvider 등록 — otel.Tracer()를 패키지 어디서든 호출 가능하게
	otel.SetTracerProvider(sdkTP)

	shutdownFn := func(ctx context.Context) error {
		return sdkTP.Shutdown(ctx)
	}

	return sdkTP, shutdownFn, nil
}
