// main.go — 서버 진입점
// SPEC-AX-SERVER-001 S1 deliverable: signal.NotifyContext + Server.New + Server.Run
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/config"
)

// main OS 진입점.
//
// @MX:NOTE: [AUTO] 서버 OS 진입점 — config.Load() + logger 초기화 후 Server.Run() 호출
func main() {
	// (b) logger 초기화 (infallible)
	logger, err := zap.NewProduction()
	if err != nil {
		// zap.NewProduction()이 실패하는 경우는 매우 드물지만 처리
		os.Exit(1) //nolint:gocritic
	}
	defer logger.Sync() //nolint:errcheck

	// (a) config 로드 (infallible)
	cfg := config.Load()

	// SIGTERM/SIGINT 수신 시 ctx 취소
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Server 초기화 (의존성 wiring)
	srv, err := New(ctx, cfg, logger)
	if err != nil {
		logger.Error("서버 초기화 실패", zap.Error(err))
		os.Exit(1) //nolint:gocritic
	}

	// Dual listener 시작 (블로킹)
	if err := srv.Run(ctx); err != nil {
		logger.Error("서버 종료 오류", zap.Error(err))
		os.Exit(1) //nolint:gocritic
	}

	os.Exit(0) //nolint:gocritic
}
