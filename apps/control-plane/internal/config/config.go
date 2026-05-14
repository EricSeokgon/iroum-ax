// 컨트롤 플레인 설정 — 12팩터 앱 방식으로 환경 변수에서 로드
// 서버 초기화는 Sprint 4/5에서 구현
package config

import (
	"os"
	"strconv"
)

// Config 컨트롤 플레인 전체 설정
// 필드 순서: 문자열 먼저, int, bool 마지막 (패딩 최소화)
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

	// OIDCIssuerURL — Keycloak realm issuer URL
	// 환경 변수: OIDC_ISSUER_URL
	// 예: http://keycloak.iroum-ax.svc.cluster.local:8080/realms/iroum-ax
	OIDCIssuerURL string

	// OIDCAudience — JWT aud 클레임 기대값
	// 환경 변수: OIDC_AUDIENCE
	// 기본값: iroum-ax-control-plane
	OIDCAudience string

	// JWKSCacheTTLSeconds — JWKS 캐시 hard TTL (초 단위)
	// 환경 변수: JWKS_CACHE_TTL_SECONDS
	// 기본값: 3600 (1시간)
	JWKSCacheTTLSeconds int

	// JWKSStaleMaxAgeSeconds — JWKS fetch 실패 시 stale 캐시 최대 허용 기간 (초)
	// 환경 변수: JWKS_STALE_MAX_AGE_SECONDS
	// 기본값: 14400 (4시간)
	JWKSStaleMaxAgeSeconds int

	// ClockSkewSeconds — JWT 시간 클레임 검증 시 허용 오차 (초)
	// 환경 변수: CLOCK_SKEW_SECONDS
	// 기본값: 30 (OAuth 2.0 BCP RFC 9700 권장치)
	ClockSkewSeconds int

	// AuthEnabled 인증 미들웨어 활성화 여부
	// 환경 변수: AUTH_ENABLED (true/false)
	// 기본값: false (로컬 개발 편의성, backward compat)
	AuthEnabled bool
}

// Load 환경 변수에서 Config를 로드하여 반환
// 환경 변수가 설정되지 않은 경우 로컬 개발 기본값을 사용
func Load() *Config {
	return &Config{
		PostgresDSN:            getEnv("POSTGRES_DSN", "postgres://iroum:iroum@localhost:5432/iroum_ax?sslmode=disable"),
		RedisAddr:              getEnv("REDIS_ADDR", "localhost:6379"),
		GRPCAddr:               getEnv("GRPC_ADDR", ":50051"),
		RESTAddr:               getEnv("REST_ADDR", ":8080"),
		Env:                    getEnv("ENV", "development"),
		OIDCIssuerURL:          getEnv("OIDC_ISSUER_URL", ""),
		OIDCAudience:           getEnv("OIDC_AUDIENCE", "iroum-ax-control-plane"),
		JWKSCacheTTLSeconds:    getIntEnv("JWKS_CACHE_TTL_SECONDS", 3600),
		JWKSStaleMaxAgeSeconds: getIntEnv("JWKS_STALE_MAX_AGE_SECONDS", 14400),
		ClockSkewSeconds:       getIntEnv("CLOCK_SKEW_SECONDS", 30),
		AuthEnabled:            getBoolEnv("AUTH_ENABLED", false),
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

// getIntEnv 환경 변수를 int로 파싱하여 반환
// 파싱 실패 시 기본값을 반환
func getIntEnv(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return parsed
}
