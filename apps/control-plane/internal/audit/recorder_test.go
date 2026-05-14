// recorder_test.go — Recorder 단위 테스트 (RED phase)
// AC-CTRL-UBI-002-A: WORKFLOW_CREATED 감사 완전성
// AC-CTRL-UBI-002-B: WORKFLOW_TRANSITIONED_TO_RUNNING 감사 완전성
// AC-CTRL-UBI-002-C: cli-anonymous 기본값
// 모든 테스트는 GREEN 단계 구현 전까지 FAIL 상태여야 함
package audit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// captureTx InsertAuditLog 호출을 캡처하는 테스트용 AuditTx 구현
type captureTx struct {
	// Captured 캡처된 이벤트 목록
	Captured []*audit.Event
	// FailInsert true이면 InsertAuditLog 에러 반환
	FailInsert bool
}

func (c *captureTx) InsertAuditLog(_ context.Context, e *audit.Event) error {
	if c.FailInsert {
		return audit.ErrAuditCaptureFail
	}
	c.Captured = append(c.Captured, e)
	return nil
}

// TestRecorder_RecordCreated_InsertsAction — RecordCreated가 WORKFLOW_CREATED 액션을 삽입
// AC-CTRL-UBI-002-A
func TestRecorder_RecordCreated_InsertsAction(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	// RED: errNotImplemented 반환으로 실패 예정
	err := recorder.RecordCreated(ctx, tx, "wf-uuid-ubi-002a", "d-uuid-ubi-002a", "cli-anonymous")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1, "정확히 1개의 감사 이벤트가 삽입되어야 함")
	assert.Equal(t, audit.ActionWorkflowCreated, tx.Captured[0].Action)
}

// TestRecorder_RecordCreated_ResourceType — RecordCreated가 resource_type='workflow'로 삽입
// AC-CTRL-UBI-002-A
func TestRecorder_RecordCreated_ResourceType(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordCreated(ctx, tx, "wf-test", "d-test", "cli-anonymous")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	assert.Equal(t, "workflow", tx.Captured[0].ResourceType)
}

// TestRecorder_RecordCreated_UserID — user_id가 요청의 userID와 일치
// AC-CTRL-UBI-002-A
func TestRecorder_RecordCreated_UserID(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(true) // authEnabled=true
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordCreated(ctx, tx, "wf-test", "d-test", "user-from-request")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	assert.Equal(t, "user-from-request", tx.Captured[0].UserID)
}

// TestRecorder_RecordCreated_DetailsContainsDocumentID — details에 document_id 포함
// AC-CTRL-UBI-002-A: details JSONB에 document_id 포함
func TestRecorder_RecordCreated_DetailsContainsDocumentID(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordCreated(ctx, tx, "wf-test", "d-uuid-ubi-002a", "cli-anonymous")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	require.NotEmpty(t, tx.Captured[0].DetailsJSON, "details JSONB가 비어있지 않아야 함")

	var details map[string]interface{}
	err = json.Unmarshal(tx.Captured[0].DetailsJSON, &details)
	require.NoError(t, err)
	assert.Equal(t, "d-uuid-ubi-002a", details["document_id"])
}

// TestRecorder_RecordTransition_ActionName — WORKFLOW_TRANSITIONED_TO_RUNNING 액션명 검증
// AC-CTRL-UBI-002-B: v0.1.2에서 통일된 단일 액션명 사용
func TestRecorder_RecordTransition_ActionName(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	// from: WORKFLOW_CREATED (실제로는 PENDING 상태를 나타내는 액션 타입 활용)
	// to: WORKFLOW_TRANSITIONED_TO_RUNNING
	err := recorder.RecordTransition(
		ctx, tx,
		"wf-uuid-ubi-002b",
		audit.ActionWorkflowCreated,
		audit.ActionWorkflowTransitionedToRunning,
		"cli-anonymous",
	)
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	assert.Equal(t, audit.ActionWorkflowTransitionedToRunning, tx.Captured[0].Action)
}

// TestRecorder_RecordTransition_Details — details JSONB에 from/to 포함
// AC-CTRL-UBI-002-B: details JSONB = {"from":"PENDING", "to":"RUNNING"}
func TestRecorder_RecordTransition_Details(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordTransition(
		ctx, tx,
		"wf-uuid-ubi-002b",
		audit.ActionWorkflowCreated,
		audit.ActionWorkflowTransitionedToRunning,
		"cli-anonymous",
	)
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	require.NotEmpty(t, tx.Captured[0].DetailsJSON)

	var details map[string]string
	err = json.Unmarshal(tx.Captured[0].DetailsJSON, &details)
	require.NoError(t, err)
	// from/to 필드가 details에 존재해야 함
	assert.Contains(t, details, "from")
	assert.Contains(t, details, "to")
}

// TestRecorder_DefaultUserID_CliAnonymous — 빈 userID + authEnabled=false → cli-anonymous
// AC-CTRL-UBI-002-C: Walking Skeleton 환경에서 user_id 기본값 검증
func TestRecorder_DefaultUserID_CliAnonymous(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false) // authEnabled=false
	tx := &captureTx{}
	ctx := context.Background()

	// userID를 빈 문자열로 전달 — cli-anonymous로 대체되어야 함
	err := recorder.RecordCreated(ctx, tx, "wf-test", "d-test", "")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	// RED: 현재 resolveUserID stub이 "" 반환하므로 실패 예정
	assert.Equal(t, audit.DefaultUserID, tx.Captured[0].UserID,
		"authEnabled=false이고 userID 빈 문자열이면 cli-anonymous이어야 함")
}

// TestRecorder_RecordTransition_UserIDPropagated — userID가 transition 이벤트에 전파
func TestRecorder_RecordTransition_UserIDPropagated(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordTransition(
		ctx, tx,
		"wf-uuid-ubi-002b",
		audit.ActionWorkflowCreated,
		audit.ActionWorkflowTransitionedToRunning,
		"cli-anonymous",
	)
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	assert.Equal(t, "cli-anonymous", tx.Captured[0].UserID)
}

// TestRecorder_RecordCompleted_Action — WORKFLOW_COMPLETED 액션 검증
func TestRecorder_RecordCompleted_Action(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordCompleted(ctx, tx, "wf-test", "cli-anonymous")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	assert.Equal(t, audit.ActionWorkflowCompleted, tx.Captured[0].Action)
}

// TestRecorder_AllActions_Covered — 8종 Action 상수의 문자열 값 검증
// research.md §2.2 감사 액션 열거형 정합성 보장
func TestRecorder_AllActions_Covered(t *testing.T) {
	t.Parallel()
	// 8종 액션을 테이블로 검증 (acceptance.md §0 AC-CTRL-UBI-002-C edge 인용)
	tests := []struct {
		action   audit.Action
		expected string
	}{
		{audit.ActionWorkflowCreated, "WORKFLOW_CREATED"},
		{audit.ActionWorkflowTransitionedToRunning, "WORKFLOW_TRANSITIONED_TO_RUNNING"},
		{audit.ActionWorkflowCompleted, "WORKFLOW_COMPLETED"},
		{audit.ActionWorkflowFailedDispatch, "WORKFLOW_FAILED_DISPATCH"},
		{audit.ActionWorkflowFailedCallback, "WORKFLOW_FAILED_CALLBACK"},
		{audit.ActionTransitionRejected, "TRANSITION_REJECTED"},
		{audit.ActionCallbackRejectedTerminal, "CALLBACK_REJECTED_TERMINAL"},
		{audit.ActionWorkflowCreateCancelled, "WORKFLOW_CREATE_CANCELLED"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, string(tt.action))
		})
	}
}

// TestRecorder_RecordFailedDispatch_Action — WORKFLOW_FAILED_DISPATCH 액션 검증
func TestRecorder_RecordFailedDispatch_Action(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordFailedDispatch(ctx, tx, "wf-test", "cli-anonymous")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	assert.Equal(t, audit.ActionWorkflowFailedDispatch, tx.Captured[0].Action)
}
