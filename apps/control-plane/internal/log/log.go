// 구조화 JSON 로거 팩토리 — zap 기반
// 서비스 공통 필드(service, version)를 기본으로 포함
package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	serviceName    = "iroum-ax-control-plane"
	serviceVersion = "0.1.0"
)

// New 환경별 zap 로거를 생성하여 반환
// env: "production" → JSON 인코더 + Info 레벨
// env: 그 외(development, test 등) → 콘솔 인코더 + Debug 레벨
//
// 기본 필드: service, version
// UTF-8 한국어 필드 값 지원 (zap은 UTF-8 문자열을 그대로 직렬화)
func New(env string) (*zap.Logger, error) {
	var cfg zap.Config

	switch env {
	case "production":
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	default:
		cfg = zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// JSON 인코딩 시 타임스탬프를 ISO8601 형식으로 출력
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build(
		zap.Fields(
			zap.String("service", serviceName),
			zap.String("version", serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("zap 로거 초기화 실패: %w", err)
	}

	return logger, nil
}
