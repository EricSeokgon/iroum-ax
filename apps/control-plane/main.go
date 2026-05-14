// iroum-ax Control Plane 진입점
// Sprint 0 스켈레톤 — 비즈니스 로직은 Sprint 7에서 구현 예정
package main

import (
	"go.uber.org/zap"
)

func main() {
	// 구조화 로거 초기화
	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	logger.Info("iroum-ax control plane starting",
		zap.String("version", "0.1.0-spec-ax-001"),
		zap.String("phase", "scaffolding"),
	)

	// TODO(Sprint 7): gRPC(:50051) + REST(:8080) 서버 시작
	// server.Run(logger)
}
