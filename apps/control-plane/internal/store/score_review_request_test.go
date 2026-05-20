// score_review_request_test.go — 평가 검토 요청 store-layer 단위 테스트 (SPEC-AX-REVIEW-001)
//
// 격리 전략: PgScoreReviewRequestTx 내부 validateReviewRequestInput/validateRejectionReason/
// validateReviewStatusTransition/resolveUserID 등 SQL 미실행 헬퍼는 default 빌드 태그로 실행.
// SELECT FOR UPDATE 비관 락 / DB CHECK constraint / audit fault rollback 등 DB 의존 테스트는
// score_review_request_integration_test.go (//go:build integration) 에서 testcontainers postgres:16-alpine으로 검증.
package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// ════════════════════════════════════════════════════════════════════════════
// validateReviewRequestInput — uuid.Nil 거부 (T-102, AC-REVIEW-001-3)
// ════════════════════════════════════════════════════════════════════════════

func TestValidateReviewRequestInput_NilScoreID_ReturnsInvalidInput(t *testing.T) {
	err := validateReviewRequestInput(uuid.Nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreReviewRequestInvalidInput,
		"uuid.Nil score_id는 ErrScoreReviewRequestInvalidInput 래핑되어야 한다")
}

func TestValidateReviewRequestInput_ValidScoreID_Passes(t *testing.T) {
	err := validateReviewRequestInput(uuid.New())
	require.NoError(t, err, "유효한 score_id는 통과해야 한다")
}

// ════════════════════════════════════════════════════════════════════════════
// validateRejectionReason — REJECTED 시 rejection_reason non-empty 강제 (§A.5 Layer 1)
// T-105 / Edge E9 / AC-REVIEW-003-3
// D5: dummy UUID 안티-패턴 제거 → 독립 함수로 분리 후 테스트
// ════════════════════════════════════════════════════════════════════════════

func TestValidateRejectionReason_BlankReasonsRejected(t *testing.T) {
	cases := []struct {
		name   string
		reason string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab only", "\t"},
		{"newline only", "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRejectionReason(tc.reason)
			require.Error(t, err)
			assert.ErrorIs(t, err, stderrors.ErrScoreReviewRequestInvalidInput,
				"blank rejection_reason은 ErrScoreReviewRequestInvalidInput 래핑되어야 한다")
		})
	}
}

func TestValidateRejectionReason_NonEmptyReasonPasses(t *testing.T) {
	err := validateRejectionReason("근거 부족")
	require.NoError(t, err)
}

// ════════════════════════════════════════════════════════════════════════════
// validateReviewStatusTransition — terminal에서 어떤 전이도 거부 (T-107)
// ════════════════════════════════════════════════════════════════════════════

func TestValidateReviewStatusTransition_FromTerminal_AllRejected(t *testing.T) {
	terminalStates := []string{reviewStatusApproved, reviewStatusRejected}
	targetStates := []string{reviewStatusSubmitted, reviewStatusUnderReview, reviewStatusApproved, reviewStatusRejected}

	for _, current := range terminalStates {
		for _, next := range targetStates {
			t.Run(current+"->"+next, func(t *testing.T) {
				err := validateReviewStatusTransition(current, next)
				require.Error(t, err, "%s→%s 전이는 terminal에서 항상 거부되어야 한다", current, next)
				assert.ErrorIs(t, err, stderrors.ErrScoreReviewRequestInvalidStatus)
			})
		}
	}
}

// ════════════════════════════════════════════════════════════════════════════
// validateReviewStatusTransition full matrix (4 × 4 = 16 조합) — T-112
// 허용: SUBMITTED→UNDER_REVIEW, UNDER_REVIEW→APPROVED, UNDER_REVIEW→REJECTED (3건)
// 거부: 13건
// ════════════════════════════════════════════════════════════════════════════

func TestValidateReviewStatusTransition_FullMatrix(t *testing.T) {
	type tc struct {
		current string
		next    string
		allowed bool
	}
	cases := []tc{
		{reviewStatusSubmitted, reviewStatusUnderReview, true},
		{reviewStatusSubmitted, reviewStatusApproved, false},
		{reviewStatusSubmitted, reviewStatusRejected, false},
		{reviewStatusSubmitted, reviewStatusSubmitted, false},
		{reviewStatusUnderReview, reviewStatusApproved, true},
		{reviewStatusUnderReview, reviewStatusRejected, true},
		{reviewStatusUnderReview, reviewStatusSubmitted, false},
		{reviewStatusUnderReview, reviewStatusUnderReview, false},
		{reviewStatusApproved, reviewStatusSubmitted, false},
		{reviewStatusApproved, reviewStatusUnderReview, false},
		{reviewStatusApproved, reviewStatusRejected, false},
		{reviewStatusApproved, reviewStatusApproved, false},
		{reviewStatusRejected, reviewStatusSubmitted, false},
		{reviewStatusRejected, reviewStatusUnderReview, false},
		{reviewStatusRejected, reviewStatusApproved, false},
		{reviewStatusRejected, reviewStatusRejected, false},
	}

	for _, c := range cases {
		t.Run(c.current+"->"+c.next, func(t *testing.T) {
			err := validateReviewStatusTransition(c.current, c.next)
			if c.allowed {
				assert.NoError(t, err, "%s→%s는 허용되어야 한다", c.current, c.next)
			} else {
				require.Error(t, err, "%s→%s는 거부되어야 한다", c.current, c.next)
				assert.ErrorIs(t, err, stderrors.ErrScoreReviewRequestInvalidStatus)
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// allowedReviewTransitions 카운트/구조 검증
// ════════════════════════════════════════════════════════════════════════════

func TestAllowedReviewTransitions_OnlyThreePathsAllowed(t *testing.T) {
	allowedCount := 0
	for _, m := range allowedReviewTransitions {
		allowedCount += len(m)
	}
	assert.Equal(t, 3, allowedCount,
		"허용 전이는 정확히 3건 (SUBMITTED→UNDER_REVIEW, UNDER_REVIEW→APPROVED, UNDER_REVIEW→REJECTED)")
}

func TestAllowedReviewTransitions_AllFourStatesPresent(t *testing.T) {
	expected := []string{reviewStatusSubmitted, reviewStatusUnderReview, reviewStatusApproved, reviewStatusRejected}
	for _, s := range expected {
		_, ok := allowedReviewTransitions[s]
		assert.True(t, ok, "%s 상태가 allowedReviewTransitions에 정의되어야 한다", s)
	}
	assert.Len(t, allowedReviewTransitions[reviewStatusApproved], 0, "APPROVED는 terminal")
	assert.Len(t, allowedReviewTransitions[reviewStatusRejected], 0, "REJECTED는 terminal")
}

// ════════════════════════════════════════════════════════════════════════════
// marshalReviewMetadata — nil/empty는 NULL, non-empty는 JSON 바이트
// ════════════════════════════════════════════════════════════════════════════

func TestMarshalReviewMetadata_NilAndEmpty_ReturnNil(t *testing.T) {
	v, err := marshalReviewMetadata(nil)
	require.NoError(t, err)
	assert.Nil(t, v, "nil metadata는 NULL이어야 한다")

	v, err = marshalReviewMetadata(map[string]any{})
	require.NoError(t, err)
	assert.Nil(t, v, "empty metadata는 NULL이어야 한다")
}

func TestMarshalReviewMetadata_NonEmpty_ReturnsBytes(t *testing.T) {
	v, err := marshalReviewMetadata(map[string]any{"key": "value", "nested": map[string]int{"a": 1}})
	require.NoError(t, err)
	assert.NotNil(t, v)
	bytes, ok := v.([]byte)
	require.True(t, ok, "non-empty metadata는 []byte로 반환되어야 한다")
	assert.Contains(t, string(bytes), "value")
}

// ════════════════════════════════════════════════════════════════════════════
// resolveUserID — userID 빈 문자열이면 'cli-anonymous' fallback (UBI-003)
// D1 fix: userID 영속 경로의 fallback 헬퍼 정직성 검증
// ════════════════════════════════════════════════════════════════════════════

func TestResolveUserID_EmptyFallsBackToCliAnonymous(t *testing.T) {
	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{"empty string fallback", "", "cli-anonymous"},
		{"whitespace fallback", "  ", "cli-anonymous"},
		{"tab fallback", "\t\n", "cli-anonymous"},
		{"non-empty passthrough", "user-alice", "user-alice"},
		{"admin passthrough", "admin-bob", "admin-bob"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.exp, resolveUserID(tc.in))
		})
	}
}
