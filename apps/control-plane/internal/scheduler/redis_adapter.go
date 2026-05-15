// redis_adapter.go — go-redis v9 클라이언트를 scheduler.RedisClient 인터페이스로 래핑하는 어댑터
// SPEC-AX-SERVER-001 S0 deliverable: internal/server/e2e_test.go:199~209 goRedisAdapter를 production code로 promote
//
// 배경: go-redis v9 raw *redis.Client의 RPush는 *redis.IntCmd, Ping은 *redis.StatusCmd를 반환하여
// scheduler.RedisClient 인터페이스(RPush(ctx,key,...) (int64,error) + Ping(ctx) error)를 직접 충족 못 함.
// 어댑터가 .Result() / .Err() 변환을 수행한다.
package scheduler

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisClientAdapter go-redis v9 *redis.Client를 scheduler.RedisClient 인터페이스로 래핑하는 어댑터.
// wiring 단계 (h)에서 NewRedisClientAdapter(redisClient)를 호출하여 NewCeleryDispatcher에 주입한다.
//
// @MX:NOTE: [AUTO] go-redis v9 command 타입 ↔ scheduler.RedisClient 인터페이스 변환 어댑터
// (e2e_test.go test-only goRedisAdapter에서 production code로 promote — D9 해소)
type RedisClientAdapter struct {
	client *redis.Client
}

// NewRedisClientAdapter RedisClientAdapter 인스턴스를 생성한다.
// 단순 struct 래핑이므로 error를 반환하지 않는다 (infallible).
func NewRedisClientAdapter(client *redis.Client) *RedisClientAdapter {
	return &RedisClientAdapter{client: client}
}

// RPush Redis LIST의 오른쪽에 하나 이상의 값을 추가한다.
// go-redis v9 *redis.IntCmd.Result()를 통해 (int64, error)로 변환한다.
func (a *RedisClientAdapter) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	return a.client.RPush(ctx, key, values...).Result()
}

// Ping Redis 연결 상태를 확인한다.
// go-redis v9 *redis.StatusCmd.Err()를 통해 error로 변환한다.
func (a *RedisClientAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

// 컴파일 타임 인터페이스 충족 검증
var _ RedisClient = (*RedisClientAdapter)(nil)
