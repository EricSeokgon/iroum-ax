// iroum-ax Control Plane 진입점
// Sprint 0: 로거 및 설정 초기화만 포함 — 서버 시작은 Sprint 4/5에서 구현
package main

import (
	"github.com/ircp/iroum-ax/apps/control-plane/internal/config"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/log"
	"go.uber.org/zap"
)

func main() {
	// 설정 로드 + 검증 (EVIDENCE_STORAGE_STRATEGY 등 fail-fast — DC-017-gap)
	cfg, err := config.LoadConfig()
	if err != nil {
		// config 검증 실패는 복구 불가능한 오류 (startup fail-fast)
		panic("config 검증 실패: " + err.Error())
	}

	// 환경별 구조화 로거 초기화
	logger, err := log.New(cfg.Env)
	if err != nil {
		// 로거 초기화 실패는 복구 불가능한 오류
		panic("로거 초기화 실패: " + err.Error())
	}
	defer logger.Sync() //nolint:errcheck

	logger.Info("iroum-ax 컨트롤 플레인 시작 중",
		zap.String("grpc_addr", cfg.GRPCAddr),
		zap.String("rest_addr", cfg.RESTAddr),
		zap.Bool("auth_enabled", cfg.AuthEnabled),
	)

	// TODO(Sprint 4): gRPC(:50051) 서버 시작
	// TODO(Sprint 5): REST(:8080) HTTP 서버 시작 (grpc-gateway)
	logger.Info("Sprint 0 초기화 완료 — 서버 구현은 Sprint 4/5에서 추가 예정")
}
