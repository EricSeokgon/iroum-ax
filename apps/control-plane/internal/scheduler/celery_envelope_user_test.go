// celery_envelope_user_test.go — REQ-AUTH-UBI-001 Celery envelope user_id propagation RED 테스트
// Sprint 4 RED: dispatcher.go의 BuildEnvelope에 userID 파라미터 + Headers.UserID 필드 없음
// → 컴파일 에러 또는 필드 없음으로 RED 상태 확보
//
// 대상 변경사항 (Sprint 4 GREEN에서 구현):
//   - Headers 구조체에 UserID string `json:"user_id"` 필드 추가
//   - BuildEnvelope 시그니처에 userID string 파라미터 추가
//   - 빈 userID → "cli-anonymous" 폴백 (backward compat)
//
// 기존 celery_envelope_v2.json (Sprint 6 CTRL 15개 테스트 골든 파일) 무손상 유지.
package scheduler

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 신규 골든 파일 경로 상수
const (
	// user_id='cli-anonymous' 골든 파일
	goldenAnonPath = "testdata/celery_envelope_v2_anon.json"
	// user_id='kepco-analyst-001' 골든 파일
	goldenAuthUserPath = "testdata/celery_envelope_v2_authuser.json"

	// Cross-SPEC 테스트용 인증 사용자 ID
	fixedAuthUserID = "kepco-analyst-001"
	// cli-anonymous 기본값
	defaultAnonUserID = "cli-anonymous"
)

// TestBuildEnvelope_WithUserID_PopulatesHeader
// userID='kepco-analyst-001' 입력 시 headers.user_id에 정확히 전달되는지 검증.
//
// RED: BuildEnvelope 시그니처에 userID 파라미터가 없음 → 컴파일 에러.
// GREEN: BuildEnvelope(workflowID, documentID, deliveryTag, replyTo, userID) 구현 후 PASS.
func TestBuildEnvelope_WithUserID_PopulatesHeader(t *testing.T) {
	t.Parallel()

	// Arrange
	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act — RED: userID 파라미터 미존재 → 컴파일 에러
	got, err := d.BuildEnvelope(
		fixedWorkflowID,
		fixedDocumentID,
		fixedDeliveryTag,
		fixedReplyTo,
		fixedAuthUserID, // 신규 파라미터 — GREEN에서 추가됨
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, got)

	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &envelope))

	headers, ok := envelope["headers"].(map[string]interface{})
	require.True(t, ok, "headers 필드가 object여야 함")

	// headers.user_id 필드 검증
	userID, exists := headers["user_id"]
	assert.True(t, exists, "headers.user_id 필드가 존재해야 함")
	assert.Equal(t, fixedAuthUserID, userID, "headers.user_id가 입력 userID와 일치해야 함")
}

// TestBuildEnvelope_EmptyUserID_DefaultsToCliAnonymous
// userID='' 입력 시 headers.user_id='cli-anonymous' 폴백 검증.
//
// RED: BuildEnvelope 시그니처에 userID 파라미터가 없음 → 컴파일 에러.
// GREEN: 빈 userID → 'cli-anonymous' 기본값 처리 후 PASS.
func TestBuildEnvelope_EmptyUserID_DefaultsToCliAnonymous(t *testing.T) {
	t.Parallel()

	// Arrange
	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act — RED: 5번째 파라미터(userID) 미존재 → 컴파일 에러
	got, err := d.BuildEnvelope(
		fixedWorkflowID,
		fixedDocumentID,
		fixedDeliveryTag,
		fixedReplyTo,
		"", // 빈 userID → "cli-anonymous" 폴백
	)

	// Assert
	require.NoError(t, err)

	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &envelope))

	headers := envelope["headers"].(map[string]interface{})
	assert.Equal(t, defaultAnonUserID, headers["user_id"],
		"빈 userID는 'cli-anonymous' 기본값으로 채워져야 함")
}

// TestBuildEnvelope_GoldenFileMatch_AuthUser
// userID='kepco-analyst-001' 입력 → celery_envelope_v2_authuser.json과 JSON 구조 일치 검증.
//
// RED: BuildEnvelope 시그니처 변경 없음 → 컴파일 에러.
// GREEN: 구현 완료 후 골든 파일과 일치 여부 PASS.
func TestBuildEnvelope_GoldenFileMatch_AuthUser(t *testing.T) {
	t.Parallel()

	// Arrange: 골든 파일 로드
	goldenBytes, err := os.ReadFile(goldenAuthUserPath)
	require.NoError(t, err, "authuser 골든 파일 읽기 실패: %s", goldenAuthUserPath)

	var goldenEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(goldenBytes, &goldenEnvelope))
	normalizedGolden, err := json.Marshal(goldenEnvelope)
	require.NoError(t, err)

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act — RED: 5번째 파라미터 미존재 → 컴파일 에러
	got, err := d.BuildEnvelope(
		fixedWorkflowID,
		fixedDocumentID,
		fixedDeliveryTag,
		fixedReplyTo,
		fixedAuthUserID,
	)

	// Assert
	require.NoError(t, err)

	var gotEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &gotEnvelope))
	normalizedGot, err := json.Marshal(gotEnvelope)
	require.NoError(t, err)

	assert.Equal(t, string(normalizedGolden), string(normalizedGot),
		"authuser envelope이 골든 파일과 일치해야 함")
}

// TestBuildEnvelope_GoldenFileMatch_Anonymous
// userID='cli-anonymous' 입력 → celery_envelope_v2_anon.json과 JSON 구조 일치 검증.
//
// RED: BuildEnvelope 시그니처 변경 없음 → 컴파일 에러.
// GREEN: 구현 완료 후 골든 파일과 일치 여부 PASS.
func TestBuildEnvelope_GoldenFileMatch_Anonymous(t *testing.T) {
	t.Parallel()

	// Arrange: 골든 파일 로드
	goldenBytes, err := os.ReadFile(goldenAnonPath)
	require.NoError(t, err, "anon 골든 파일 읽기 실패: %s", goldenAnonPath)

	var goldenEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(goldenBytes, &goldenEnvelope))
	normalizedGolden, err := json.Marshal(goldenEnvelope)
	require.NoError(t, err)

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act — RED: 5번째 파라미터 미존재 → 컴파일 에러
	got, err := d.BuildEnvelope(
		fixedWorkflowID,
		fixedDocumentID,
		fixedDeliveryTag,
		fixedReplyTo,
		defaultAnonUserID,
	)

	// Assert
	require.NoError(t, err)

	var gotEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &gotEnvelope))
	normalizedGot, err := json.Marshal(gotEnvelope)
	require.NoError(t, err)

	assert.Equal(t, string(normalizedGolden), string(normalizedGot),
		"anon envelope이 골든 파일과 일치해야 함")
}

// TestBuildEnvelope_BackwardCompat_OriginalGoldenStillValid
// 기존 celery_envelope_v2.json (Sprint 6 CTRL 15개 테스트 골든 파일) 회귀 가드.
// user_id 필드 추가 후에도 기존 골든 파일의 나머지 필드 구조가 동일해야 함.
//
// RED: 현재 BuildEnvelope 4-파라미터 시그니처로 호출 → user_id 필드 검사에서 실패.
// GREEN: user_id 추가된 새 구현 → 기존 필드 구조는 동일하게 유지.
func TestBuildEnvelope_BackwardCompat_OriginalGoldenStillValid(t *testing.T) {
	t.Parallel()

	// Arrange: 기존 골든 파일 로드 (Sprint 6 CTRL 회귀 가드)
	goldenBytes, err := os.ReadFile(goldenFilePath) // Sprint 6 CTRL 원본 골든 파일
	require.NoError(t, err, "원본 골든 파일 읽기 실패: %s", goldenFilePath)

	var goldenEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(goldenBytes, &goldenEnvelope))

	mockRedis := &mockRedisClient{}
	d := NewCeleryDispatcher(mockRedis, "celery", fixedHostname)

	// Act — RED: 5번째 파라미터 미존재 → 컴파일 에러
	got, err := d.BuildEnvelope(
		fixedWorkflowID,
		fixedDocumentID,
		fixedDeliveryTag,
		fixedReplyTo,
		defaultAnonUserID,
	)
	require.NoError(t, err)

	var gotEnvelope map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &gotEnvelope))

	// user_id 필드를 제외한 headers 구조 비교 (기존 골든 파일은 user_id 없음)
	goldenHeaders, ok := goldenEnvelope["headers"].(map[string]interface{})
	require.True(t, ok)
	gotHeaders, ok := gotEnvelope["headers"].(map[string]interface{})
	require.True(t, ok)

	// 기존 골든 파일의 모든 필드가 새 envelope에도 동일하게 존재해야 함
	for key, goldenVal := range goldenHeaders {
		gotVal, exists := gotHeaders[key]
		assert.True(t, exists, "기존 headers.%s 필드가 새 envelope에 존재해야 함", key)
		assert.Equal(t, goldenVal, gotVal,
			"기존 headers.%s 값이 동일해야 함 (Sprint 6 CTRL 회귀 가드)", key)
	}

	// body, content-encoding, content-type, properties 최상위 필드 회귀 검증
	for _, topKey := range []string{"content-encoding", "content-type"} {
		assert.Equal(t, goldenEnvelope[topKey], gotEnvelope[topKey],
			"최상위 %s 필드가 동일해야 함", topKey)
	}
}
