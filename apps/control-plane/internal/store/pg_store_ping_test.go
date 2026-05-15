//go:build integration

// pg_store_ping_test.go — PgWorkflowStore.Ping() 통합 테스트
// SPEC-AX-SERVER-001 S0 deliverable: readiness probe 경량 liveness 메서드 검증
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestPgWorkflowStore_Ping -v -count=1
package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestPgWorkflowStore_Ping_ReturnsNilOnSuccess 연결 성공 시 Ping()은 nil을 반환해야 한다.
func TestPgWorkflowStore_Ping_ReturnsNilOnSuccess(t *testing.T) {
	// 통합 테스트: testcontainers postgres 필요
	if testing.Short() {
		t.Skip("통합 테스트 — -short 플래그로 건너뜀")
	}

	ctx := context.Background()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	// TestMain의 testDSN 전역 변수 사용 (postgres_test.go에서 관리)
	if testDSN == "" {
		t.Skip("testDSN이 설정되지 않았습니다 — TestMain이 실행되어야 합니다")
	}

	store, err := NewPgWorkflowStore(ctx, testDSN, logger)
	require.NoError(t, err, "NewPgWorkflowStore는 유효한 DSN으로 에러 없이 생성되어야 한다")
	defer store.Close()

	err = store.Ping(ctx)
	assert.NoError(t, err, "활성 PostgreSQL 연결에서 Ping()은 nil을 반환해야 한다")
}

// TestPgWorkflowStore_Ping_PropagatesError 컨텍스트 취소 시 에러를 전파해야 한다.
func TestPgWorkflowStore_Ping_PropagatesError(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트 — -short 플래그로 건너뜀")
	}

	if testDSN == "" {
		t.Skip("testDSN이 설정되지 않았습니다 — TestMain이 실행되어야 합니다")
	}

	ctx := context.Background()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	store, err := NewPgWorkflowStore(ctx, testDSN, logger)
	require.NoError(t, err)
	defer store.Close()

	// 이미 취소된 컨텍스트로 Ping 호출
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // 즉시 취소

	err = store.Ping(cancelCtx)
	assert.Error(t, err, "취소된 컨텍스트에서 Ping()은 에러를 반환해야 한다")
}
