// 컨트롤 플레인 설정 — 12팩터 앱 방식으로 환경 변수에서 로드
// 서버 초기화는 Sprint 4/5에서 구현
package config

import (
	"fmt"
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

	// CeleryQueue Celery 태스크 큐 이름
	// 환경 변수: CELERY_QUEUE
	// 기본값: celery
	CeleryQueue string

	// EvidenceStorageStrategy 증빙 저장 전략 (SPEC-AX-EVID-001)
	// 환경 변수: EVIDENCE_STORAGE_STRATEGY
	// 기본값: database_blob (Run Phase 1 확정). 열거 {filesystem,database_blob,minio} 검증 fail-fast
	// 필드 순서: string 블록 끝에 배치하여 fieldalignment 최적화
	EvidenceStorageStrategy string

	// EvidenceMaxFileBytes 증빙 파일 최대 바이트 (pre-TX 거부 임계 — REQ-EVID-001-U1)
	// 환경 변수: EVIDENCE_MAX_FILE_BYTES
	// 기본값: 52428800 (50 MiB). int64 블록 — int64(8B)
	EvidenceMaxFileBytes int64

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

	// ShutdownTimeoutSeconds SIGTERM 수신 후 graceful shutdown 최대 대기 시간 (초)
	// 환경 변수: SHUTDOWN_TIMEOUT_SECONDS
	// 기본값: 30
	ShutdownTimeoutSeconds int

	// ReadyProbeTimeoutSeconds /ready 프로브 개별 체크 타임아웃 (초)
	// 환경 변수: READY_PROBE_TIMEOUT_SECONDS
	// 기본값: 5
	ReadyProbeTimeoutSeconds int

	// AuthEnabled 인증 미들웨어 활성화 여부
	// 환경 변수: AUTH_ENABLED (true/false)
	// 기본값: false (로컬 개발 편의성, backward compat)
	AuthEnabled bool

	// EvidenceDuplicateSignalEnabled 중복 SHA-256 비차단 신호 활성화 (REQ-EVID-001-O1 Optional)
	// 환경 변수: EVIDENCE_DUPLICATE_SIGNAL_ENABLED (true/false)
	// 기본값: false (Sandbox PoC 비활성)
	EvidenceDuplicateSignalEnabled bool
}

// validEvidenceStrategies 허용된 storage_strategy 열거값 (DDL CHECK 제약과 정합)
var validEvidenceStrategies = map[string]struct{}{
	"filesystem":    {},
	"database_blob": {},
	"minio":         {},
}

// Load 환경 변수에서 Config를 로드하여 반환
// 환경 변수가 설정되지 않은 경우 로컬 개발 기본값을 사용
func Load() *Config {
	return &Config{
		PostgresDSN:              getEnv("POSTGRES_DSN", "postgres://iroum:iroum@localhost:5432/iroum_ax?sslmode=disable"),
		RedisAddr:                getEnv("REDIS_ADDR", "localhost:6379"),
		GRPCAddr:                 getEnv("GRPC_ADDR", ":50051"),
		RESTAddr:                 getEnv("REST_ADDR", ":8080"),
		Env:                      getEnv("ENV", "development"),
		OIDCIssuerURL:            getEnv("OIDC_ISSUER_URL", ""),
		OIDCAudience:             getEnv("OIDC_AUDIENCE", "iroum-ax-control-plane"),
		JWKSCacheTTLSeconds:      getIntEnv("JWKS_CACHE_TTL_SECONDS", 3600),
		JWKSStaleMaxAgeSeconds:   getIntEnv("JWKS_STALE_MAX_AGE_SECONDS", 14400),
		ClockSkewSeconds:         getIntEnv("CLOCK_SKEW_SECONDS", 30),
		CeleryQueue:              getEnv("CELERY_QUEUE", "celery"),
		ShutdownTimeoutSeconds:   getIntEnv("SHUTDOWN_TIMEOUT_SECONDS", 30),
		ReadyProbeTimeoutSeconds: getIntEnv("READY_PROBE_TIMEOUT_SECONDS", 5),
		AuthEnabled:              getBoolEnv("AUTH_ENABLED", false),

		EvidenceStorageStrategy:        getEnv("EVIDENCE_STORAGE_STRATEGY", "database_blob"),
		EvidenceMaxFileBytes:           getInt64Env("EVIDENCE_MAX_FILE_BYTES", 52428800),
		EvidenceDuplicateSignalEnabled: getBoolEnv("EVIDENCE_DUPLICATE_SIGNAL_ENABLED", false),
	}
}

// Validate 설정 무결성을 검증한다 — 잘못된 값은 startup fail-fast (panic 금지)
// SPEC-AX-EVID-001 DC-017-gap/E-09: EVIDENCE_STORAGE_STRATEGY 열거 검증
func (c *Config) Validate() error {
	if _, ok := validEvidenceStrategies[c.EvidenceStorageStrategy]; !ok {
		return fmt.Errorf("EVIDENCE_STORAGE_STRATEGY 잘못된 값 %q (허용: filesystem|database_blob|minio)",
			c.EvidenceStorageStrategy)
	}
	return nil
}

// LoadConfig Load() + Validate()를 결합하여 검증된 Config를 반환
// 검증 실패 시 nil, error를 반환하여 호출자(main.go)가 fail-fast 종료한다.
// 기존 Load()는 backward-compat을 위해 시그니처/동작 불변 유지 (T-001 회귀).
func LoadConfig() (*Config, error) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config 검증 실패: %w", err)
	}
	return cfg, nil
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

// getInt64Env 환경 변수를 int64로 파싱하여 반환
// 파싱 실패 시 기본값을 반환 (EVIDENCE_MAX_FILE_BYTES 등 대용량 임계값용)
func getInt64Env(key string, defaultVal int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return parsed
}
