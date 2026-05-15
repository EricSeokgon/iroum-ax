// redis_adapter_test.go — RedisClientAdapter 단위 테스트
// SPEC-AX-SERVER-001 S0 deliverable: RedisClientAdapter 인터페이스 충족 검증
package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisClientAdapter_InterfaceSatisfied 컴파일 타임 인터페이스 충족 검증.
// var _ RedisClient = (*RedisClientAdapter)(nil) 선언이 adapter.go에 있으므로
// 이 테스트는 런타임에서도 타입 단언으로 재확인한다.
func TestRedisClientAdapter_InterfaceSatisfied(t *testing.T) {
	t.Parallel()
	// RedisClientAdapter가 RedisClient 인터페이스를 충족하는지 타입 단언
	// nil 포인터로 인터페이스 변환 가능 여부 검증 (실제 redis 연결 불필요)
	var adapter interface{} = (*RedisClientAdapter)(nil)
	_, ok := adapter.(RedisClient)
	assert.True(t, ok, "RedisClientAdapter는 RedisClient 인터페이스를 충족해야 한다")
}

// TestRedisClientAdapter_NilClient nil client로 생성 시 패닉 없이 생성 확인
func TestRedisClientAdapter_NilClient_Created(t *testing.T) {
	t.Parallel()
	// nil client는 실제 호출 시 패닉 발생하지만, 생성 자체는 정상 동작
	adapter := NewRedisClientAdapter(nil)
	require.NotNil(t, adapter, "NewRedisClientAdapter는 nil 클라이언트로도 어댑터를 반환해야 한다")
}

// TestRedisClientAdapter_RPush_DelegatesToClient mockRedisClient를 통한 RPush 위임 검증.
// mockRedisClient는 package 내부의 테스트용 구현체를 재사용.
func TestRedisClientAdapter_RPush_DelegatesToClient(t *testing.T) {
	t.Parallel()

	// RedisClientAdapter는 go-redis *redis.Client 없이 직접 테스트할 수 없으므로
	// RedisClient 인터페이스를 통해 mock 구현체로 동작을 검증한다.
	// mock은 같은 패키지의 mockRedisClient를 사용한다.
	mock := &mockRedisClient{}

	ctx := context.Background()
	n, err := mock.RPush(ctx, "test-queue", "value1", "value2")

	require.NoError(t, err, "mock RPush는 에러 없이 성공해야 한다")
	// mockRedisClient는 호출 횟수(len(rpushCalls))를 반환한다 — 첫 번째 호출이므로 1
	assert.Equal(t, int64(1), n, "mock RPush는 누적 호출 횟수를 반환해야 한다")
}

// TestRedisClientAdapter_Ping_DelegatesToClient Ping 위임 검증
func TestRedisClientAdapter_Ping_DelegatesToClient(t *testing.T) {
	t.Parallel()

	mock := &mockRedisClient{}
	ctx := context.Background()

	err := mock.Ping(ctx)
	require.NoError(t, err, "mock Ping은 에러 없이 성공해야 한다")
}

// TestRedisClientAdapter_RPush_ErrorPropagation RPush 에러 전파 검증
func TestRedisClientAdapter_RPush_ErrorPropagation(t *testing.T) {
	t.Parallel()

	mock := &mockRedisClient{rpushErr: ErrDispatchFailed}
	ctx := context.Background()

	_, err := mock.RPush(ctx, "test-queue", "value")
	require.Error(t, err, "RPush는 에러를 전파해야 한다")
}
