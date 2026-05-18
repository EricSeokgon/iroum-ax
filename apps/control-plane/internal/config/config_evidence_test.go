// config_evidence_test.go — T-014: 증빙 config 키 + fail-fast 검증
// DC-017 (storage_strategy enum), DC-017-gap/E-09 (invalid → LoadConfig 에러),
// REQ-EVID-001-U1 (MAX_FILE_BYTES), REQ-EVID-001-O1 (DUPLICATE_SIGNAL_ENABLED)
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_EvidenceDefaults 환경변수 미설정 시 기본값 검증
func TestConfig_EvidenceDefaults(t *testing.T) {
	t.Setenv("EVIDENCE_STORAGE_STRATEGY", "")
	t.Setenv("EVIDENCE_MAX_FILE_BYTES", "")
	t.Setenv("EVIDENCE_DUPLICATE_SIGNAL_ENABLED", "")

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "database_blob", cfg.EvidenceStorageStrategy, "Run Phase 1 확정 기본값")
	assert.Equal(t, int64(52428800), cfg.EvidenceMaxFileBytes, "50 MiB 기본값")
	assert.False(t, cfg.EvidenceDuplicateSignalEnabled, "Optional 비활성 기본값")
}

// TestConfig_EvidenceOverrides 환경변수 오버라이드 반영
func TestConfig_EvidenceOverrides(t *testing.T) {
	t.Setenv("EVIDENCE_STORAGE_STRATEGY", "filesystem")
	t.Setenv("EVIDENCE_MAX_FILE_BYTES", "1024")
	t.Setenv("EVIDENCE_DUPLICATE_SIGNAL_ENABLED", "true")

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "filesystem", cfg.EvidenceStorageStrategy)
	assert.Equal(t, int64(1024), cfg.EvidenceMaxFileBytes)
	assert.True(t, cfg.EvidenceDuplicateSignalEnabled)
}

// TestConfig_InvalidStorageStrategyRejectsAtLoad
// DC-017-gap, E-09: 잘못된 EVIDENCE_STORAGE_STRATEGY → LoadConfig 에러 (panic 아님, fail-fast)
func TestConfig_InvalidStorageStrategyRejectsAtLoad(t *testing.T) {
	t.Setenv("EVIDENCE_STORAGE_STRATEGY", "s3_external")

	cfg, err := LoadConfig()
	require.Error(t, err, "열거 외 전략은 LoadConfig가 에러를 반환해야 함 (startup fail-fast)")
	assert.Nil(t, cfg, "검증 실패 시 *Config는 nil")
	assert.Contains(t, err.Error(), "EVIDENCE_STORAGE_STRATEGY")
}

// TestConfig_AllValidStrategiesAccepted 3개 열거값 모두 허용
func TestConfig_AllValidStrategiesAccepted(t *testing.T) {
	for _, s := range []string{"filesystem", "database_blob", "minio"} {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Setenv("EVIDENCE_STORAGE_STRATEGY", s)
			cfg, err := LoadConfig()
			require.NoError(t, err)
			assert.Equal(t, s, cfg.EvidenceStorageStrategy)
		})
	}
}

// TestConfig_LoadBackwardCompat 기존 Load()는 시그니처/동작 불변 (T-001 회귀)
func TestConfig_LoadBackwardCompat(t *testing.T) {
	cfg := Load() // 기존 시그니처 *Config (에러 없음) 보존
	require.NotNil(t, cfg)
	assert.Equal(t, "database_blob", cfg.EvidenceStorageStrategy,
		"Load()도 신규 필드를 기본값으로 채움 (additive)")
}
