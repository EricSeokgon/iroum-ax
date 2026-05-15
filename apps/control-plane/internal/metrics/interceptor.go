// interceptor.go — gRPC 요청 레이턴시 계측 UnaryServerInterceptor
// SPEC-AX-OBS-001 Sprint 2 FIX: AC-OBS-001-3 gRPC grpc_request_duration_seconds 관찰
package metrics

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryMetricsInterceptor — gRPC Unary 요청 레이턴시를 grpc_request_duration_seconds 히스토그램에 기록한다.
//
// 체인 최외곽(first interceptor)에 배치 — 인증 실패를 포함한 모든 요청을 계측한다 (REQ-OBS-001).
// server.go의 BuildGRPCInterceptorChain 호출 전에 grpc.ChainUnaryInterceptor에 prepend.
//
// @MX:ANCHOR: [AUTO] gRPC metrics interceptor — server.go + test에서 참조 (fan_in >= 2 예정)
// @MX:REASON: AC-OBS-001-3 달성의 핵심 계측 지점 — 체인 순서 변경 시 gRPC 관찰 누락 위험
func UnaryMetricsInterceptor(m *Metrics) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Since(start).Seconds()

		// gRPC 상태 코드 추출 (에러 없으면 OK=0)
		code := strconv.Itoa(int(status.Code(err)))
		m.ObserveGRPCDuration(info.FullMethod, code, elapsed)

		return resp, err
	}
}
