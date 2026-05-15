//go:build integration

// pg_store_ping_test.go — PgWorkflowStore.Ping() 통합 테스트
// SPEC-AX-SERVER-001 S0 deliverable: readiness probe 경량 liveness 메서드 검증
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestPgWorkflowStore_Ping -v -count=1
package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"
)

// setupPingTestContainer Ping 테스트용 postgres 컨테이너를 기동하고 DSN을 반환한다.
func setupPingTestContainer(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("iroum_ax"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err, "Postgres 컨테이너 기동 실패")
	t.Cleanup(func() {
		if terr := pgContainer.Terminate(context.Background()); terr != nil {
			t.Logf("Postgres 컨테이너 종료 실패: %v", terr)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Postgres DSN 조회 실패")
	return dsn
}

// TestPgWorkflowStore_Ping_ReturnsNilOnSuccess 연결 성공 시 Ping()은 nil을 반환해야 한다.
func TestPgWorkflowStore_Ping_ReturnsNilOnSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트 — -short 플래그로 건너뜀")
	}

	dsn := setupPingTestContainer(t)

	ctx := context.Background()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	s, err := NewPgWorkflowStore(ctx, dsn, logger)
	require.NoError(t, err, "NewPgWorkflowStore는 유효한 DSN으로 에러 없이 생성되어야 한다")
	defer s.Close()

	err = s.Ping(ctx)
	assert.NoError(t, err, "활성 PostgreSQL 연결에서 Ping()은 nil을 반환해야 한다")
}

// TestPgWorkflowStore_Ping_PropagatesError 컨텍스트 취소 시 에러를 전파해야 한다.
func TestPgWorkflowStore_Ping_PropagatesError(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트 — -short 플래그로 건너뜀")
	}

	dsn := setupPingTestContainer(t)

	ctx := context.Background()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	s, err := NewPgWorkflowStore(ctx, dsn, logger)
	require.NoError(t, err)
	defer s.Close()

	// 이미 취소된 컨텍스트로 Ping 호출
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // 즉시 취소

	err = s.Ping(cancelCtx)
	assert.Error(t, err, "취소된 컨텍스트에서 Ping()은 에러를 반환해야 한다")
}
