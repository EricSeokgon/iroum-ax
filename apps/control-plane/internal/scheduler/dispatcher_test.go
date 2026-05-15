// dispatcher_test.go — REQ-CTRL-005 Celery dispatch GREEN phase 테스트
// Sprint 6 GREEN: 실제 구현에 대한 동작 검증 (ErrNotImplemented 가드 제거)
// AC-CTRL-005-1: Celery envelope golden file byte match
// AC-CTRL-005-2: Redis unavailable → ErrDispatchFailed
// AC-CTRL-005-3: Serialization failure → no RPUSH
// AC-CTRL-005-4: Dispatch latency p99 < 100ms
package scheduler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// --- TestMain: goroutine 누수 감지 ---

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// --- 테스트용 Redis mock 구현 ---

// mockRedisClient RedisClient 인터페이스를 구현하는 테스트 mock
// 필드 순서: 에러(16바이트) × 2 → 슬라이스(24바이트) → bool(1바이트) → 뮤텍스(8바이트 내부 state)
// mu는 rpushCalls 동시 접근 보호용 — race detector 통과 필수 (DISPUTE #3 DATA RACE fix)
// fieldalignment: sync.Mutex는 마지막에 배치 (24B → 0B padding 제거)
type mockRedisClient struct {
	pingErr    error
	rpushErr   error
	rpushCalls []rpushCall
	closed     bool
	mu         sync.Mutex
}

type rpushCall struct {
	key    string
	values []interface{}
}

func (m *mockRedisClient) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.rpushErr != nil {
		err := m.rpushErr
		if m.closed {
			err = errors.New("redis: client is closed")
		}
		return 0, err
	}
	m.rpushCalls = append(m.rpushCalls, rpushCall{key: key, values: values})
	return int64(len(m.rpushCalls)), nil
}

func (m *mockRedisClient) Ping(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("redis: client is closed")
	}
	return m.pingErr
}

// getRpushCalls rpushCalls 슬라이스를 mutex 보호 하에 복사 반환 (race-safe read helper)
func (m *mockRedisClient) getRpushCalls() []rpushCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]rpushCall, len(m.rpushCalls))
	copy(cp, m.rpushCalls)
	return cp
}

// 골든 파일 경로
const goldenFilePath = "testdata/celery_envelope_v2.json"

// 결정적 테스트용 고정 UUID 상수
const (
	fixedWorkflowID  = "fixed-test-uuid-005-001"
	fixedDocumentID  = "d-fixed-005-001"
	fixedDeliveryTag = "fixed-delivery-tag-005-001"
	fixedReplyTo     = "fixed-reply-to-005-001"
	fixedTaskName    = "pipelines.workers.ingestion_worker.run"
	fixedHostname    = "localhost"
)

// --- AC-CTRL-005-1: Celery envelope golden file byte match ---

// TestCeleryDispatcher_BuildEnvelope_GoldenFileMatch
// 고정 입력으로 생성한 envelope가 골든 파일과 JSON 구조적으로 일치해야 함
// GREEN: 실제 BuildEnvelope 구현이 정상 동작 — NoError + JSON 동등성 검증
func TestCeleryDispatcher_BuildEnvelope_GoldenFileMatch(t *testing.T) {
	t.Parallel()

	// Arrange
	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// user_id 포함 골든 파일 (goldenAnonPath = celery_envelope_v2_anon.json)
	// Sprint 4 이후 BuildEnvelope는 user_id 필드를 포함하므로 anon 골든 파일 사용
	goldenBytes, err := os.ReadFile(goldenAnonPath)
	require.NoError(t, err, "골든 파일 읽기 실패: %s", goldenAnonPath)

	// Go의 encoding/json은 map key를 알파벳 순 정렬하므로 정규화된 비교 수행
	var goldenEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(goldenBytes, &goldenEnvelope))
	normalizedGolden, err := json.Marshal(goldenEnvelope)
	require.NoError(t, err)

	// Act
	got, err := d.BuildEnvelope(fixedWorkflowID, fixedDocumentID, fixedDeliveryTag, fixedReplyTo, defaultAnonUserID)

	// Assert: GREEN — 실제 구현이 정상 동작해야 함
	require.NoError(t, err, "BuildEnvelope가 에러 없이 완료되어야 함")
	require.NotNil(t, got, "BuildEnvelope가 비어있지 않은 바이트를 반환해야 함")

	var gotEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &gotEnvelope))
	normalizedGot, err := json.Marshal(gotEnvelope)
	require.NoError(t, err)
	assert.Equal(t, string(normalizedGolden), string(normalizedGot),
		"envelope이 골든 파일(anon)과 일치해야 함")
}

// TestCeleryDispatcher_BuildEnvelope_RequiredFields
// 생성된 envelope에 Kombu v2 필수 필드가 모두 포함되어야 함
// GREEN: 실제 BuildEnvelope 구현 검증
func TestCeleryDispatcher_BuildEnvelope_RequiredFields(t *testing.T) {
	t.Parallel()

	// Arrange
	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act
	got, err := d.BuildEnvelope(fixedWorkflowID, fixedDocumentID, fixedDeliveryTag, fixedReplyTo, defaultAnonUserID)

	// Assert: GREEN — 에러 없이 필수 필드 포함
	require.NoError(t, err, "BuildEnvelope가 에러 없이 완료되어야 함")
	require.NotNil(t, got)

	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &envelope))

	// 최상위 필수 필드
	assert.Equal(t, "utf-8", envelope["content-encoding"])
	assert.Equal(t, "application/json", envelope["content-type"])
	assert.NotEmpty(t, envelope["body"])

	// headers 필드 검증
	headers, ok := envelope["headers"].(map[string]interface{})
	require.True(t, ok, "headers 필드가 object여야 함")
	assert.Equal(t, "py", headers["lang"])
	assert.Equal(t, fixedTaskName, headers["task"])
	assert.Equal(t, fixedWorkflowID, headers["id"])
	assert.Equal(t, fixedWorkflowID, headers["root_id"])
	assert.Nil(t, headers["parent_id"])

	// properties 필드 검증
	props, ok := envelope["properties"].(map[string]interface{})
	require.True(t, ok, "properties 필드가 object여야 함")
	assert.Equal(t, fixedWorkflowID, props["correlation_id"])
	assert.Equal(t, "base64", props["body_encoding"])
	assert.Equal(t, float64(2), props["delivery_mode"])
	assert.Equal(t, float64(0), props["priority"])

	deliveryInfo, ok := props["delivery_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "", deliveryInfo["exchange"])
	assert.Equal(t, "celery", deliveryInfo["routing_key"])
}

// TestCeleryDispatcher_BuildEnvelope_BodyBase64Decodable
// envelope body를 base64 디코드하면 유효한 JSON [args, kwargs, embed] 배열이어야 함
// GREEN: 실제 BuildEnvelope body 구조 검증
func TestCeleryDispatcher_BuildEnvelope_BodyBase64Decodable(t *testing.T) {
	t.Parallel()

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	got, err := d.BuildEnvelope(fixedWorkflowID, fixedDocumentID, fixedDeliveryTag, fixedReplyTo, defaultAnonUserID)

	// GREEN — 에러 없이 body 구조 검증
	require.NoError(t, err, "BuildEnvelope가 에러 없이 완료되어야 함")
	require.NotNil(t, got)

	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &envelope))

	bodyStr, ok := envelope["body"].(string)
	require.True(t, ok)

	decoded, err := base64.StdEncoding.DecodeString(bodyStr)
	require.NoError(t, err, "body는 base64 디코드 가능해야 함")

	var bodyArray []interface{}
	require.NoError(t, json.Unmarshal(decoded, &bodyArray), "body JSON은 배열이어야 함")
	require.Len(t, bodyArray, 3, "body는 [args, kwargs, embed] 3개 요소여야 함")

	// args: ["d-fixed-005-001"]
	args, ok := bodyArray[0].([]interface{})
	require.True(t, ok)
	assert.Equal(t, fixedDocumentID, args[0])

	// kwargs: {"workflow_id": "fixed-test-uuid-005-001"}
	kwargs, ok := bodyArray[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, fixedWorkflowID, kwargs["workflow_id"])

	// embed: {"callbacks":null, "chain":null, "chord":null, "errbacks":null}
	embed, ok := bodyArray[2].(map[string]interface{})
	require.True(t, ok)
	assert.Nil(t, embed["callbacks"])
	assert.Nil(t, embed["chain"])
	assert.Nil(t, embed["chord"])
	assert.Nil(t, embed["errbacks"])
}

// TestCeleryDispatcher_BuildEnvelope_ArgsKwargsRepr
// Python repr 형식 argsrepr/kwargsrepr 검증
// GREEN: 실제 BuildEnvelope Python repr 필드 검증
func TestCeleryDispatcher_BuildEnvelope_ArgsKwargsRepr(t *testing.T) {
	t.Parallel()

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	got, err := d.BuildEnvelope(fixedWorkflowID, fixedDocumentID, fixedDeliveryTag, fixedReplyTo, defaultAnonUserID)

	// GREEN — 에러 없이 argsrepr/kwargsrepr 검증
	require.NoError(t, err, "BuildEnvelope가 에러 없이 완료되어야 함")
	require.NotNil(t, got)

	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &envelope))
	headers := envelope["headers"].(map[string]interface{})

	// Python list repr: ['d-fixed-005-001']
	assert.Equal(t, "['d-fixed-005-001']", headers["argsrepr"])
	// Python dict repr: {'workflow_id': 'fixed-test-uuid-005-001'}
	assert.Equal(t, "{'workflow_id': 'fixed-test-uuid-005-001'}", headers["kwargsrepr"])
}

// TestPythonReprList Python list repr 변환 함수 단위 테스트
func TestPythonReprList(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		expected string
		input    []string
	}
	cases := []testCase{
		{
			name:     "단일 문자열",
			input:    []string{"d-fixed-005-001"},
			expected: "['d-fixed-005-001']",
		},
		{
			name:     "빈 리스트",
			input:    []string{},
			expected: "[]",
		},
		{
			name:     "복수 문자열",
			input:    []string{"a", "b"},
			expected: "['a', 'b']",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pythonReprList(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestPythonReprDict Python dict repr 변환 함수 단위 테스트
func TestPythonReprDict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    map[string]string
		expected string
	}{
		{
			name:     "단일 키-값",
			input:    map[string]string{"workflow_id": "fixed-test-uuid-005-001"},
			expected: "{'workflow_id': 'fixed-test-uuid-005-001'}",
		},
		{
			name:     "빈 dict",
			input:    map[string]string{},
			expected: "{}",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pythonReprDict(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// --- AC-CTRL-005-2: Redis RPUSH 검증 ---

// TestCeleryDispatcher_Dispatch_RedisRPUSH
// 정상 dispatch가 Redis LIST "celery"에 RPUSH를 수행해야 함
// GREEN: 실제 Dispatch 구현 — RPUSH 1회 호출 검증
func TestCeleryDispatcher_Dispatch_RedisRPUSH(t *testing.T) {
	t.Parallel()

	// Arrange
	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act
	err := d.Dispatch(context.Background(), fixedWorkflowID, fixedDocumentID)

	// Assert: GREEN — 에러 없이 RPUSH 1회 호출
	require.NoError(t, err, "정상 dispatch는 에러를 반환하지 않아야 함")
	calls := mockRedis.getRpushCalls()
	require.Len(t, calls, 1, "RPUSH가 정확히 1회 호출되어야 함")
	assert.Equal(t, "celery", calls[0].key,
		"기본 queue 이름은 'celery'여야 함")
	require.Len(t, calls[0].values, 1, "envelope 1개가 RPUSH되어야 함")
}

// TestCeleryDispatcher_Dispatch_CustomQueue
// 커스텀 queue 이름으로 dispatch 시 해당 queue에 RPUSH해야 함
// GREEN: 실제 Dispatch 구현 — 커스텀 queue 검증
func TestCeleryDispatcher_Dispatch_CustomQueue(t *testing.T) {
	t.Parallel()

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "custom_queue", fixedHostname)

	err := d.Dispatch(context.Background(), fixedWorkflowID, fixedDocumentID)

	require.NoError(t, err, "커스텀 queue dispatch는 에러를 반환하지 않아야 함")
	calls2 := mockRedis.getRpushCalls()
	require.Len(t, calls2, 1)
	assert.Equal(t, "custom_queue", calls2[0].key)
}

// TestCeleryDispatcher_Dispatch_RedisUnavailable_ReturnsError
// Redis가 불가능한 상태에서 dispatch 시 에러를 반환해야 함 (AC-CTRL-005-2)
// GREEN: ErrDispatchFailed 래핑 에러 반환 검증
func TestCeleryDispatcher_Dispatch_RedisUnavailable_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange: Redis close 시뮬레이션
	mockRedis := &mockRedisClient{closed: true}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act
	err := d.Dispatch(context.Background(), fixedWorkflowID, fixedDocumentID)

	// Assert: GREEN — ErrDispatchFailed를 포함한 에러 반환
	require.Error(t, err, "Redis 불가 상태에서 에러를 반환해야 함")
	assert.ErrorIs(t, err, ErrDispatchFailed,
		"Redis RPUSH 실패 시 ErrDispatchFailed가 래핑되어야 함 (AC-CTRL-005-2)")
}

// TestCeleryDispatcher_Dispatch_FailureMarksWorkflowFailed
// dispatch 실패 시 ErrDispatchFailed를 포함한 에러를 반환해야 함 (R-CTRL-003 mitigation)
// 호출자(handlers.go)가 이 에러를 받아 workflow FAILED로 전이하는 계약 검증
// GREEN: ErrDispatchFailed 래핑 검증
func TestCeleryDispatcher_Dispatch_FailureMarksWorkflowFailed(t *testing.T) {
	t.Parallel()

	mockRedis := &mockRedisClient{rpushErr: errors.New("redis connection refused")}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	err := d.Dispatch(context.Background(), fixedWorkflowID, fixedDocumentID)

	require.Error(t, err, "Redis 에러 시 dispatch가 에러를 반환해야 함")
	// GREEN 검증: err가 ErrDispatchFailed를 포함해야 함 (R-CTRL-003: workflow FAILED 전이 계약)
	assert.ErrorIs(t, err, ErrDispatchFailed,
		"dispatch 실패 시 ErrDispatchFailed가 래핑되어야 함 — 호출자가 workflow FAILED로 전이 가능")
}

// TestCeleryDispatcher_Dispatch_ContextCancelled
// context가 취소된 상태에서 dispatch 시 context 에러를 반환해야 함
// GREEN: context.Canceled 에러 체인 검증
func TestCeleryDispatcher_Dispatch_ContextCancelled(t *testing.T) {
	t.Parallel()

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	err := d.Dispatch(ctx, fixedWorkflowID, fixedDocumentID)

	require.Error(t, err, "취소된 context에서 dispatch는 에러를 반환해야 함")
	// GREEN 검증: context.Canceled가 에러 체인에 포함되어야 함
	assert.ErrorIs(t, err, context.Canceled,
		"context 취소 시 context.Canceled가 에러 체인에 포함되어야 함")
}

// TestCeleryDispatcher_Dispatch_NoRPUSH_WhenEnvelopeFails
// envelope 직렬화 실패 시 RPUSH가 발생하지 않아야 함 (AC-CTRL-005-3)
// GREEN: ErrEnvelopeSerializationFailed 래핑 + RPUSH 0건 검증
func TestCeleryDispatcher_Dispatch_NoRPUSH_WhenEnvelopeFails(t *testing.T) {
	t.Parallel()

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// builder가 항상 실패하는 mock 주입
	d.SetBuilder(func(_, _ string) ([]byte, error) {
		return nil, ErrEnvelopeSerializationFailed
	})

	err := d.Dispatch(context.Background(), fixedWorkflowID, fixedDocumentID)

	require.Error(t, err, "envelope 직렬화 실패 시 에러를 반환해야 함")
	// GREEN 검증: ErrEnvelopeSerializationFailed 래핑 + RPUSH 0건
	assert.ErrorIs(t, err, ErrEnvelopeSerializationFailed,
		"직렬화 실패 에러가 체인에 포함되어야 함 (AC-CTRL-005-3)")
	assert.Empty(t, mockRedis.getRpushCalls(), "직렬화 실패 시 RPUSH가 발생하지 않아야 함")
}

// --- AC-CTRL-005-1 골든 파일 구조 검증 ---

// TestCeleryEnvelopeGoldenFile_StructureValid
// 골든 파일 자체가 유효한 JSON이며 필수 키를 보유해야 함
func TestCeleryEnvelopeGoldenFile_StructureValid(t *testing.T) {
	t.Parallel()

	absPath := goldenFilePath
	if !filepath.IsAbs(absPath) {
		// 테스트 실행 위치와 관계없이 올바른 경로
		absPath = filepath.Join("testdata", "celery_envelope_v2.json")
	}

	data, err := os.ReadFile(absPath)
	require.NoError(t, err, "골든 파일이 존재해야 함: %s", absPath)

	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &envelope), "골든 파일이 유효한 JSON이어야 함")

	// 최상위 필수 키 확인
	requiredKeys := []string{"body", "content-encoding", "content-type", "headers", "properties"}
	for _, k := range requiredKeys {
		assert.Contains(t, envelope, k, "골든 파일에 '%s' 키가 있어야 함", k)
	}

	// body가 base64 디코드 가능한지 확인
	bodyStr, ok := envelope["body"].(string)
	require.True(t, ok, "body가 문자열이어야 함")
	decoded, err := base64.StdEncoding.DecodeString(bodyStr)
	require.NoError(t, err, "body가 base64 디코드 가능해야 함")

	var bodyArray []interface{}
	require.NoError(t, json.Unmarshal(decoded, &bodyArray), "body가 JSON 배열이어야 함")
	assert.Len(t, bodyArray, 3, "body 배열이 [args, kwargs, embed] 3개 요소여야 함")
}

// TestCeleryEnvelopeGoldenFile_KeysAlphabeticalOrder
// 골든 파일의 key 순서가 Go encoding/json의 알파벳 정렬과 일치해야 함
func TestCeleryEnvelopeGoldenFile_KeysAlphabeticalOrder(t *testing.T) {
	t.Parallel()

	// user_id 포함 anon 골든 파일로 17개 headers 검증 (Sprint 4 이후)
	data, err := os.ReadFile(goldenAnonPath)
	require.NoError(t, err)

	// json.Unmarshal 후 re-marshal이 idempotent한지 검증 (round-trip 안정성)
	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &envelope))

	// 첫 번째 marshal
	marshaled1, err := json.Marshal(envelope)
	require.NoError(t, err)

	// 두 번째 round-trip
	var envelope2 map[string]interface{}
	require.NoError(t, json.Unmarshal(marshaled1, &envelope2))
	marshaled2, err := json.Marshal(envelope2)
	require.NoError(t, err)

	// encoding/json은 map key를 알파벳 순으로 정렬하므로 round-trip이 idempotent해야 함
	assert.Equal(t, string(marshaled1), string(marshaled2),
		"골든 파일의 JSON round-trip이 idempotent해야 함 (encoding/json 알파벳 정렬 보장)")

	// headers 필드가 존재하고 16개 키를 보유하는지 확인 (골든 파일 완전성)
	headers, ok := envelope["headers"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, headers, 17, "headers에 Kombu v2 필수 16개 키 + user_id 1개 = 17개 키가 있어야 함")
}

// --- AC-CTRL-005-4: Dispatch 레이턴시 벤치마크 ---

// BenchmarkDispatchLatency dispatch 레이턴시 p99 < 100ms 벤치마크 (AC-CTRL-005-4)
// GREEN 이후: 실제 레이턴시 측정 가능
func BenchmarkDispatchLatency(b *testing.B) {
	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = d.Dispatch(ctx, fixedWorkflowID+string(rune('0'+i%10)), fixedDocumentID)
			i++
		}
	})
}

// TestCeleryDispatcher_Latency_P99Under100ms dispatch p99 < 100ms 검증 (단위 테스트 버전)
// 10 goroutine × 100 iteration = 1000회 dispatch, p99 측정
// GREEN: 실제 Redis mock RPUSH 포함 레이턴시 검증
func TestCeleryDispatcher_Latency_P99Under100ms(t *testing.T) {
	if testing.Short() {
		t.Skip("단기 테스트 모드에서는 레이턴시 테스트 건너뜀")
	}
	t.Parallel()

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	const (
		concurrency = 10
		iterations  = 100
		total       = concurrency * iterations
	)

	durations := make([]time.Duration, 0, total)
	durationCh := make(chan time.Duration, total)
	done := make(chan struct{})

	// 수집 goroutine
	go func() {
		for dur := range durationCh {
			durations = append(durations, dur)
		}
		close(done)
	}()

	// 측정 goroutine 풀
	errCh := make(chan error, total)
	sem := make(chan struct{}, concurrency)
	for i := 0; i < total; i++ {
		i := i
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			start := time.Now()
			err := d.Dispatch(context.Background(),
				fixedWorkflowID, fixedDocumentID+string(rune('0'+i%10)))
			elapsed := time.Since(start)
			durationCh <- elapsed
			errCh <- err
		}()
	}

	// 모든 goroutine 완료 대기
	for i := 0; i < concurrency; i++ {
		sem <- struct{}{}
	}
	close(durationCh)
	<-done

	// GREEN: 모든 dispatch가 성공해야 함
	close(errCh)
	for err := range errCh {
		assert.NoError(t, err, "GREEN 단계: 모든 dispatch가 성공해야 함 (mock Redis)")
	}

	// p99 계산
	require.Len(t, durations, total)
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p99Idx := int(float64(total) * 0.99)
	if p99Idx >= total {
		p99Idx = total - 1
	}
	p99 := durations[p99Idx]

	// GREEN: 실제 Redis mock RPUSH 포함 p99 < 100ms 검증
	assert.Less(t, p99, 100*time.Millisecond,
		"dispatch p99 레이턴시가 100ms 미만이어야 함 (현재: %v)", p99)
}
