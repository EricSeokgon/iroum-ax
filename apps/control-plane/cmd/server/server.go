// gRPC + REST 서버 스텁
// Sprint 0 스켈레톤 — 실제 구현은 Sprint 7(T-AX-006) 예정
package server

import (
	"go.uber.org/zap"
)

// Config 서버 설정
type Config struct {
	GRPCPort int    // gRPC 리스닝 포트 (기본: 50051)
	RESTPort int    // REST 리스닝 포트 (기본: 8080)
	LogLevel string // 로그 레벨
}

// Server gRPC + REST 복합 서버
// @MX:TODO - Sprint 7에서 gRPC-Gateway v2 기반으로 구현
type Server struct {
	cfg    Config
	logger *zap.Logger
}

// New 서버 인스턴스 생성
func New(cfg Config, logger *zap.Logger) *Server {
	return &Server{
		cfg:    cfg,
		logger: logger,
	}
}

// Run 서버 시작 (블로킹)
// TODO(Sprint 7): gRPC 서버 + HTTP 리버스 프록시(grpc-gateway) 실제 구현
func (s *Server) Run() error {
	s.logger.Info("server stub — not yet implemented",
		zap.Int("grpc_port", s.cfg.GRPCPort),
		zap.Int("rest_port", s.cfg.RESTPort),
	)
	return nil
}
