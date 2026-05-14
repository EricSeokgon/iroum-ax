// dispatcher.go — Celery 태스크 디스패처 (Redis 직접 RPUSH, Kombu 프로토콜 v2)
// REQ-CTRL-005: Celery dispatch via Redis-direct JSON envelope v2 (Kombu 호환)
package scheduler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrDispatchFailed Redis RPUSH 실패 시 반환되는 에러 (R-CTRL-003 PENDING 고아 mitigation)
var ErrDispatchFailed = errors.New("celery dispatch failed")

// ErrEnvelopeSerializationFailed envelope 직렬화 실패 시 반환 (AC-CTRL-005-3)
var ErrEnvelopeSerializationFailed = errors.New("celery envelope serialization failed")

// RedisClient Redis 클라이언트 인터페이스 (mock-friendly, go-redis 의존 없이 테스트 가능)
// @MX:ANCHOR: [AUTO] Dispatcher + 테스트 fake 2곳에서 구현됨
// @MX:REASON: AC-CTRL-005-2 Redis unavailable 테스트가 이 인터페이스를 mock으로 교체
type RedisClient interface {
	// RPush Redis LIST의 오른쪽에 하나 이상의 값을 추가
	RPush(ctx context.Context, key string, values ...interface{}) (int64, error)
	// Ping Redis 연결 상태 확인
	Ping(ctx context.Context) error
}

// CeleryEnvelopeBuilder Celery envelope 생성 함수 시그니처 (테스트에서 교체 가능)
type CeleryEnvelopeBuilder func(workflowID, documentID string) ([]byte, error)

// CeleryDispatcher Redis를 통해 Celery 태스크를 디스패치하는 구조체
// 필드 순서: 인터페이스(16바이트) → 함수(8바이트) → 문자열들 (padding 최소화)
type CeleryDispatcher struct {
	redis    RedisClient
	builder  CeleryEnvelopeBuilder
	queue    string
	hostname string
}

// NewCeleryDispatcher CeleryDispatcher 인스턴스 생성
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
// @MX:ANCHOR: [AUTO] Celery dispatch의 외부 진입점 — REQ-CTRL-005 핵심 계약
// @MX:REASON: 테스트 + handlers.go + workflow coordinator 3곳에서 호출 예정
// @MX:SPEC: REQ-CTRL-005, AC-CTRL-005-2
func (d *CeleryDispatcher) Dispatch(ctx context.Context, workflowID, documentID string) error {
	// context 취소 여부 먼저 확인
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("dispatch cancelled: %w", err)
	}

	var envelopeBytes []byte
	var err error

	// builder가 주입된 경우 builder 사용 (테스트에서 직렬화 실패 시뮬레이션에 활용)
	if d.builder != nil {
		envelopeBytes, err = d.builder(workflowID, documentID)
		if err != nil {
			return fmt.Errorf("dispatch envelope build failed: %w", err)
		}
	} else {
		// 기본 BuildEnvelope 사용: deliveryTag와 replyTo는 workflowID 기반 생성
		envelopeBytes, err = d.BuildEnvelope(workflowID, documentID, workflowID, workflowID)
		if err != nil {
			return fmt.Errorf("dispatch envelope build failed: %w", err)
		}
	}

	// Redis RPUSH
	if _, rpushErr := d.redis.RPush(ctx, d.queue, envelopeBytes); rpushErr != nil {
		return fmt.Errorf("%w: %w", ErrDispatchFailed, rpushErr)
	}

	return nil
}

// BuildEnvelope Kombu 호환 Celery JSON envelope v2 직렬화
// 결정적 입력(workflowID, documentID, deliveryTag, replyTo)을 받아 byte 배열 반환
// 테스트에서 고정 값을 주입하여 golden file 비교 가능하도록 설계
//
// @MX:ANCHOR: [AUTO] Celery envelope 직렬화 계약 — golden file이 유일한 진실
// @MX:REASON: TestCeleryDispatcher_BuildEnvelope_GoldenFileMatch가 byte-exact match 요구
// @MX:SPEC: REQ-CTRL-005, AC-CTRL-005-1
func (d *CeleryDispatcher) BuildEnvelope(
	workflowID string,
	documentID string,
	deliveryTag string,
	replyTo string,
) ([]byte, error) {
	const taskName = "pipelines.workers.ingestion_worker.run"

	// body 배열: [args, kwargs, embed]
	// args: 위치 인자 — documentID 단일 원소
	// kwargs: 키워드 인자 — workflow_id
	// embed: Celery chain/chord/callback 정보 (모두 null)
	args := []interface{}{documentID}
	kwargs := map[string]interface{}{
		"workflow_id": workflowID,
	}
	embed := map[string]interface{}{
		"callbacks": nil,
		"chain":     nil,
		"chord":     nil,
		"errbacks":  nil,
	}
	bodyArray := []interface{}{args, kwargs, embed}

	bodyJSON, err := json.Marshal(bodyArray)
	if err != nil {
		return nil, fmt.Errorf("%w: body marshal: %w", ErrEnvelopeSerializationFailed, err)
	}

	// body를 base64 인코딩 (StdEncoding — Kombu 호환)
	encodedBody := base64.StdEncoding.EncodeToString(bodyJSON)

	// argsrepr / kwargsrepr: Python repr 형식
	argsRepr := pythonReprList([]string{documentID})
	kwargsRepr := pythonReprDict(map[string]string{"workflow_id": workflowID})

	// origin: "go-control-plane@<hostname>"
	origin := "go-control-plane@" + d.hostname

	// headers: Kombu v2 필수 16개 필드
	headers := map[string]interface{}{
		"argsrepr":      argsRepr,
		"eta":           nil,
		"expires":       nil,
		"group":         nil,
		"group_index":   nil,
		"id":            workflowID,
		"ignore_result": false,
		"kwargsrepr":    kwargsRepr,
		"lang":          "py",
		"origin":        origin,
		"parent_id":     nil,
		"retries":       0,
		"root_id":       workflowID,
		"shadow":        nil,
		"task":          taskName,
		"timelimit":     []interface{}{nil, nil},
	}

	// properties: 메시지 전달 메타데이터
	properties := map[string]interface{}{
		"body_encoding":  "base64",
		"correlation_id": workflowID,
		"delivery_info": map[string]interface{}{
			"exchange":    "",
			"routing_key": d.queue,
		},
		"delivery_mode": 2,
		"delivery_tag":  deliveryTag,
		"priority":      0,
		"reply_to":      replyTo,
	}

	// 최상위 envelope
	envelope := map[string]interface{}{
		"body":             encodedBody,
		"content-encoding": "utf-8",
		"content-type":     "application/json",
		"headers":          headers,
		"properties":       properties,
	}

	result, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("%w: envelope marshal: %w", ErrEnvelopeSerializationFailed, err)
	}

	return result, nil
}

// pythonReprList Go []string을 Python list repr 형식으로 변환
// 예: ["d-uuid"] → "['d-uuid']"
// Celery headers.argsrepr 필드에 사용 (AC-CTRL-005-1 golden file 비교 필수)
func pythonReprList(args []string) string {
	if len(args) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, s := range args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteByte('\'')
		// 단일 따옴표 이스케이프 처리
		sb.WriteString(strings.ReplaceAll(s, "'", "\\'"))
		sb.WriteByte('\'')
	}
	sb.WriteByte(']')
	return sb.String()
}

// pythonReprDict Go map[string]string을 Python dict repr 형식으로 변환
// 예: {"workflow_id": "uuid"} → "{'workflow_id': 'uuid'}"
// Celery headers.kwargsrepr 필드에 사용 (AC-CTRL-005-1 golden file 비교 필수)
// 키는 알파벳 순 정렬 (Go map 순서 비결정적이므로 명시적 정렬 필요)
func pythonReprDict(kwargs map[string]string) string {
	if len(kwargs) == 0 {
		return "{}"
	}

	// 키 알파벳 정렬
	keys := make([]string, 0, len(kwargs))
	for k := range kwargs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteByte('\'')
		sb.WriteString(strings.ReplaceAll(k, "'", "\\'"))
		sb.WriteString("': '")
		sb.WriteString(strings.ReplaceAll(kwargs[k], "'", "\\'"))
		sb.WriteByte('\'')
	}
	sb.WriteByte('}')
	return sb.String()
}

// ErrNotImplemented Sprint 6 GREEN 단계에서 실제 구현으로 대체됨
// 테스트 파일이 이 변수를 참조하므로 선언 유지 필요 (실제 반환하지 않음)
// @MX:NOTE: [AUTO] GREEN 완료 후 테스트 파일에서 참조 제거 시 함께 삭제 가능
var ErrNotImplemented = errors.New("not implemented: Sprint 6 GREEN pending")
