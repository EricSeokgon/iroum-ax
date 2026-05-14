// 컨트롤 플레인 설정 — 12팩터 앱 방식으로 환경 변수에서 로드
// 서버 초기화는 Sprint 4/5에서 구현
package config

import (
	"os"
	"strconv"
)

// Config 컨트롤 플레인 전체 설정
// 필드 순서: 문자열 먼저, bool 마지막 (패딩 최소화)
type Config struct {
	// PostgresDSN PostgreSQL 연결 문자열
	// 환경 변수: POSTGRES_DSN
	// 기본값: local Docker Compose 설정과 일치
	PostgresDSN string

	// RedisAddr Redis 서버 주소 (host:port)
	// 환경 변수: REDIS_ADDR
	RedisAddr string

	// GRPCAddr gRPC 서버 리스닝 주소
	// 환경 변수: GRPC_ADDR
	GRPCAddr string

	// RESTAddr HTTP REST 서버 리스닝 주소
	// 환경 변수: REST_ADDR
	RESTAddr string

	// Env 실행 환경 (production / development / test)
	// 환경 변수: ENV
	Env string

	// AuthEnabled 인증 미들웨어 활성화 여부
	// 환경 변수: AUTH_ENABLED (true/false)
	// 기본값: false (로컬 개발 편의성)
	AuthEnabled bool
}

// Load 환경 변수에서 Config를 로드하여 반환
// 환경 변수가 설정되지 않은 경우 로컬 개발 기본값을 사용
func Load() *Config {
	return &Config{
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://iroum:iroum@localhost:5432/iroum_ax?sslmode=disable"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		GRPCAddr:    getEnv("GRPC_ADDR", ":50051"),
		RESTAddr:    getEnv("REST_ADDR", ":8080"),
		Env:         getEnv("ENV", "development"),
		AuthEnabled: getBoolEnv("AUTH_ENABLED", false),
	}
}

// getEnv 환경 변수 값을 반환하고, 없으면 기본값을 반환
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// getBoolEnv 환경 변수를 bool로 파싱하여 반환
// 파싱 실패 시 기본값을 반환
func getBoolEnv(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return parsed
}
