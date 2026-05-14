// dispatcher.go — Celery 태스크 디스패처 (Redis 직접 RPUSH, Kombu 프로토콜 v2)
// Sprint 6 RED: 인터페이스 + ErrNotImplemented 스텁 — GREEN 단계에서 구현 예정
// REQ-CTRL-005: Celery dispatch via Redis-direct JSON envelope v2 (Kombu 호환)
package scheduler

import (
	"context"
	"errors"
)

// ErrNotImplemented Sprint 6 GREEN 이전 스텁 함수 호출 시 반환되는 에러
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 실제 구현으로 교체 예정
// @MX:SPEC: REQ-CTRL-005
var ErrNotImplemented = errors.New("not implemented: Sprint 6 GREEN pending")

// ErrDispatchFailed Redis RPUSH 실패 시 반환되는 에러 (R-CTRL-003 PENDING 고아 mitigation)
var ErrDispatchFailed = errors.New("celery dispatch failed")

// ErrEnvelopeSerializationFailed envelope 직렬화 실패 시 반환 (AC-CTRL-005-3)
var ErrEnvelopeSerializationFailed = errors.New("celery envelope serialization failed")

// RedisClient Redis 클라이언트 인터페이스 (mock-friendly, go-redis 의존 없이 테스트 가능)
// @MX:ANCHOR: [AUTO] Dispatcher + 테스트 fake 2곳에서 구현 예정
// @MX:REASON: AC-CTRL-005-2 Redis unavailable 테스트가 이 인터페이스를 mock으로 교체
type RedisClient interface {
	// RPush Redis LIST의 오른쪽에 하나 이상의 값을 추가
	RPush(ctx context.Context, key string, values ...interface{}) (int64, error)
	// Ping Redis 연결 상태 확인
	Ping(ctx context.Context) error
}

// CeleryEnvelopeBuilder Celery envelope 생성 함수 시그니처 (테스트에서 교체 가능)
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 BuildIngestionTaskEnvelope로 구현
type CeleryEnvelopeBuilder func(workflowID, documentID string) ([]byte, error)

// CeleryDispatcher Redis를 통해 Celery 태스크를 디스패치하는 구조체
// 필드 순서: 인터페이스(16바이트) → 함수(8바이트) → 문자열들 (padding 최소화)
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 구현체 필드 채움 (redis *redis.Client → RedisClient 인터페이스)
type CeleryDispatcher struct {
	redis    RedisClient
	builder  CeleryEnvelopeBuilder
	queue    string
	hostname string
}

// NewCeleryDispatcher CeleryDispatcher 인스턴스 생성
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 redisAddr을 go-redis 클라이언트로 연결
func NewCeleryDispatcher(redis RedisClient, queue string, hostname string) *CeleryDispatcher {
	if queue == "" {
		queue = "celery"
	}
	if hostname == "" {
		hostname = "localhost"
	}
	return &CeleryDispatcher{
		redis:    redis,
		queue:    queue,
		hostname: hostname,
	}
}

// SetBuilder envelope 빌더 함수 주입 (테스트에서 결정적 UUID 생성을 위해 사용)
func (d *CeleryDispatcher) SetBuilder(b CeleryEnvelopeBuilder) {
	d.builder = b
}

// Dispatch 워크플로우 태스크를 Celery 브로커(Redis)에 전송
// 성공: Redis LIST에 RPUSH 후 nil 반환
// 실패: ErrDispatchFailed 래핑 에러 반환 → 호출자(handlers.go)가 workflow FAILED로 전이
//
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 구현 — 3회 exponential backoff retry (50/200/800ms)
// @MX:SPEC: REQ-CTRL-005, AC-CTRL-005-2
func (d *CeleryDispatcher) Dispatch(ctx context.Context, workflowID, documentID string) error {
	return ErrNotImplemented
}

// BuildEnvelope Kombu 호환 Celery JSON envelope v2 직렬화
// 결정적 입력(workflowID, documentID, deliveryTag, replyTo)을 받아 byte 배열 반환
// 테스트에서 고정 UUID를 주입하여 golden file 비교 가능하도록 설계
//
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 구현 — encoding/json 사용, map key 알파벳 정렬 보장
// @MX:SPEC: REQ-CTRL-005, AC-CTRL-005-1
func (d *CeleryDispatcher) BuildEnvelope(
	workflowID string,
	documentID string,
	deliveryTag string,
	replyTo string,
) ([]byte, error) {
	return nil, ErrNotImplemented
}

// pythonReprList Go []string을 Python list repr 형식으로 변환
// 예: ["d-uuid"] → "['d-uuid']"
// Celery headers.argsrepr 필드에 사용 (AC-CTRL-005-1 golden file 비교 필수)
//
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 구현
func pythonReprList(args []string) string {
	return ""
}

// pythonReprDict Go map[string]string을 Python dict repr 형식으로 변환
// 예: {"workflow_id": "uuid"} → "{'workflow_id': 'uuid'}"
// Celery headers.kwargsrepr 필드에 사용 (AC-CTRL-005-1 golden file 비교 필수)
//
// @MX:TODO: [AUTO] Sprint 6 GREEN에서 구현
func pythonReprDict(kwargs map[string]string) string {
	return ""
}
