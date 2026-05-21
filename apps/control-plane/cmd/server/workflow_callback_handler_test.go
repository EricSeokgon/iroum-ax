// workflow_callback_handler_test.go — Python→Go 콜백 핸들러 단위 테스트
// (SPEC-AX-AUDIT-INTEG-001 — REQ-INTEG-004 / REQ-INTEG-005 검증)
//
// 격리 전략: httptest + fake WorkflowStore/WorkflowTx (audit_query_handlers_test.go 동형).
// integration 빌드 태그 미사용 — 기본 `go test ./apps/control-plane/cmd/server/`로 실행.
// E2E (docker-compose) 통합 테스트는 별도 tests/integration/test_integ_001_workflow_e2e.py.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// ── fake WorkflowStore/WorkflowTx (callback 검증용) ───────────────────────────────

// fakeCallbackTx WorkflowTx의 GetWorkflow + UpdateWorkflowState + UpdateWorkflowResult +
// InsertAuditLog만 사용한다. QueryAuditLogs/InsertWorkflow는 panic — 콜백 경로 외 호출 금지.
type fakeCallbackTx struct {
	// 주입: workflow 조회 결과
	workflow    *types.Workflow
	getErr      error
	updateErr   error
	resultErr   error
	insertErr   error
	commitErr   error
	beginCalled bool

	// 호출 기록
	getCalls          int
	getID             string
	updateStateCalls  int
	updateStateID     string
	updateStateNew    types.WorkflowState
	updateResultCalls int
	updateResultID    string
	updateResultJSON  []byte
	auditEvents       []*audit.Event
	commitCalled      bool
	rollbackCalled    bool
}

func (tx *fakeCallbackTx) InsertWorkflow(_ context.Context, _ *types.Workflow) error {
	panic("InsertWorkflow not expected — callback handler MUST NOT INSERT workflow")
}

func (tx *fakeCallbackTx) InsertAuditLog(_ context.Context, e *audit.Event) error {
	if tx.insertErr != nil {
		return tx.insertErr
	}
	evCopy := *e
	tx.auditEvents = append(tx.auditEvents, &evCopy)
	return nil
}

func (tx *fakeCallbackTx) UpdateWorkflowState(_ context.Context, id string, newState types.WorkflowState) error {
	tx.updateStateCalls++
	tx.updateStateID = id
	tx.updateStateNew = newState
	return tx.updateErr
}

func (tx *fakeCallbackTx) GetWorkflow(_ context.Context, id string) (*types.Workflow, error) {
	tx.getCalls++
	tx.getID = id
	if tx.getErr != nil {
		return nil, tx.getErr
	}
	if tx.workflow == nil {
		return nil, apperrors.ErrWorkflowNotFound
	}
	wfCopy := *tx.workflow
	return &wfCopy, nil
}

func (tx *fakeCallbackTx) UpdateWorkflowResult(_ context.Context, id string, resultJSON []byte) error {
	tx.updateResultCalls++
	tx.updateResultID = id
	buf := make([]byte, len(resultJSON))
	copy(buf, resultJSON)
	tx.updateResultJSON = buf
	return tx.resultErr
}

func (tx *fakeCallbackTx) QueryAuditLogs(
	_ context.Context, _ store.AuditLogFilter, _, _ int,
) ([]*audit.Event, int64, error) {
	panic("QueryAuditLogs not expected in callback path")
}

func (tx *fakeCallbackTx) Commit(_ context.Context) error {
	tx.commitCalled = true
	return tx.commitErr
}

func (tx *fakeCallbackTx) Rollback(_ context.Context) error {
	tx.rollbackCalled = true
	return nil
}

// fakeCallbackStore store.WorkflowStore fake — BeginTx로 주입된 tx를 반환한다.
type fakeCallbackStore struct {
	tx          *fakeCallbackTx
	beginErr    error
	beginCalled bool
}

func (s *fakeCallbackStore) BeginTx(_ context.Context) (store.WorkflowTx, error) {
	s.beginCalled = true
	if s.tx != nil {
		s.tx.beginCalled = true
	}
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	return s.tx, nil
}

func (s *fakeCallbackStore) ListWorkflows(_ context.Context, _, _ int) ([]*types.Workflow, error) {
	panic("ListWorkflows not expected in callback path")
}

// newTestCallbackHandler 테스트용 WorkflowCallbackHandler를 생성한다.
func newTestCallbackHandler(t *testing.T, tx *fakeCallbackTx) (*WorkflowCallbackHandler, *fakeCallbackStore) {
	t.Helper()
	st := &fakeCallbackStore{tx: tx}
	rec := audit.NewRecorder(false) // auth-disabled — user_id force 'cli-anonymous'
	logger := zaptest.NewLogger(t)
	h := NewWorkflowCallbackHandler(st, rec, logger)
	return h, st
}

// makeRunningWorkflow 테스트 fixture — RUNNING 상태 워크플로우 생성
func makeRunningWorkflow(t *testing.T) *types.Workflow {
	t.Helper()
	wfID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	docID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return &types.Workflow{
		ID:         wfID,
		DocumentID: docID,
		State:      types.WorkflowStateRunning,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

// doCallbackRequest POST /api/v1/workflows/{id}/callback 호출 헬퍼
func doCallbackRequest(t *testing.T, h *WorkflowCallbackHandler, wfID string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+wfID+"/callback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	return rec
}

// ── 테스트 ────────────────────────────────────────────────────────────────────────

// T-001: RUNNING → COMPLETED 정상 콜백 — HTTP 204 + UpdateWorkflowState(COMPLETED) +
// UpdateWorkflowResult + WORKFLOW_COMPLETED audit row (AC-INTEG-001-2)
func TestWorkflowCallback_RunningToCompleted_Returns204(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"completed","result_json":{"pages":5}}`)

	assert.Equal(t, http.StatusNoContent, rec.Code, "RUNNING→COMPLETED expects 204")
	assert.Equal(t, 1, tx.updateStateCalls, "UpdateWorkflowState should be called once")
	assert.Equal(t, types.WorkflowStateCompleted, tx.updateStateNew)
	assert.Equal(t, 1, tx.updateResultCalls, "UpdateWorkflowResult should be called once")
	assert.Contains(t, string(tx.updateResultJSON), "pages")
	require.Len(t, tx.auditEvents, 1, "exactly 1 audit row (WORKFLOW_COMPLETED)")
	assert.Equal(t, audit.ActionWorkflowCompleted, tx.auditEvents[0].Action)
	assert.True(t, tx.commitCalled, "TX must be committed")
}

// T-002: RUNNING → FAILED 정상 콜백 — HTTP 204 + UpdateWorkflowState(FAILED) +
// WORKFLOW_FAILED_CALLBACK audit row (AC-INTEG-001-3)
func TestWorkflowCallback_RunningToFailed_Returns204(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"failed","result_json":{"error":"parse error"}}`)

	assert.Equal(t, http.StatusNoContent, rec.Code, "RUNNING→FAILED expects 204")
	assert.Equal(t, types.WorkflowStateFailed, tx.updateStateNew)
	require.Len(t, tx.auditEvents, 1)
	assert.Equal(t, audit.ActionWorkflowFailedCallback, tx.auditEvents[0].Action)
	assert.True(t, tx.commitCalled)
}

// T-003: COMPLETED 상태에서 콜백 도착 — HTTP 409 Conflict +
// CALLBACK_REJECTED_TERMINAL audit + state 무변경 (AC-INTEG-001-4)
func TestWorkflowCallback_CompletedTerminal_Returns409(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	wf.State = types.WorkflowStateCompleted
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"completed","result_json":{}}`)

	assert.Equal(t, http.StatusConflict, rec.Code, "COMPLETED terminal expects 409")
	assert.Equal(t, 0, tx.updateStateCalls, "state must NOT be updated on terminal")
	assert.Equal(t, 0, tx.updateResultCalls, "result must NOT be updated on terminal")
	// CALLBACK_REJECTED_TERMINAL audit row should be present
	require.Len(t, tx.auditEvents, 1)
	assert.Equal(t, audit.ActionCallbackRejectedTerminal, tx.auditEvents[0].Action)
	assert.True(t, tx.commitCalled, "TX must commit audit row even on 409")
}

// T-004: FAILED terminal 상태에서 콜백 도착 — HTTP 409 Conflict
func TestWorkflowCallback_FailedTerminal_Returns409(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	wf.State = types.WorkflowStateFailed
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"failed","result_json":{}}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, 0, tx.updateStateCalls)
}

// T-005: PENDING 상태에서 콜백 도착 — HTTP 409 Conflict (RUNNING 외 거부)
func TestWorkflowCallback_PendingState_Returns409(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	wf.State = types.WorkflowStatePending
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"completed","result_json":{}}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, 0, tx.updateStateCalls)
}

// T-006: 존재하지 않는 workflow ID — HTTP 404 Not Found (AC-INTEG-001-5)
func TestWorkflowCallback_WorkflowNotFound_Returns404(t *testing.T) {
	t.Parallel()
	// workflow=nil → GetWorkflow가 ErrWorkflowNotFound를 반환
	tx := &fakeCallbackTx{workflow: nil}
	h, _ := newTestCallbackHandler(t, tx)

	wfID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa").String()
	rec := doCallbackRequest(t, h, wfID,
		`{"status":"completed","result_json":{}}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, 0, tx.updateStateCalls)
}

// T-007: 잘못된 status 값 — HTTP 400 Bad Request (AC-INTEG-001-6)
func TestWorkflowCallback_InvalidStatus_Returns400(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"unknown","result_json":{}}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid status expects 400")
	assert.Equal(t, 0, tx.getCalls, "store must not be touched on invalid input")
}

// T-008: 빈 body — HTTP 400 Bad Request
func TestWorkflowCallback_EmptyBody_Returns400(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(), ``)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// T-009: 잘못된 JSON — HTTP 400 Bad Request
func TestWorkflowCallback_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"completed", malformed`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// T-010: 잘못된 UUID path parameter — HTTP 400
func TestWorkflowCallback_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()
	tx := &fakeCallbackTx{}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, "not-a-uuid",
		`{"status":"completed","result_json":{}}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, tx.getCalls)
}

// T-011: result_json 생략 — 빈 객체 {} 로 영속화 (§5.2)
func TestWorkflowCallback_MissingResultJSON_PersistsEmptyObject(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"completed"}`)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, tx.updateResultCalls)
	// 빈 객체 "{}" 으로 영속
	assert.Equal(t, "{}", string(tx.updateResultJSON))
}

// T-012: store 에러 격리 — UpdateWorkflowState 실패 시 500 (TX rollback)
func TestWorkflowCallback_UpdateStateError_Returns500(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{
		workflow:  wf,
		updateErr: errors.New("db error: connection lost"),
	}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"completed","result_json":{}}`)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.False(t, tx.commitCalled, "must NOT commit when state update fails")
}

// T-013: 응답 body 확인 — 204는 body 없음
func TestWorkflowCallback_SuccessResponseBody_Empty(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"completed","result_json":{"ok":true}}`)

	require.Equal(t, http.StatusNoContent, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	assert.Empty(t, body, "204 No Content body must be empty")
}

// T-014: 4xx 응답은 JSON envelope 포함 — score_handlers 패턴 동형
func TestWorkflowCallback_ErrorResponseBody_HasJSONEnvelope(t *testing.T) {
	t.Parallel()
	wf := makeRunningWorkflow(t)
	tx := &fakeCallbackTx{workflow: wf}
	h, _ := newTestCallbackHandler(t, tx)

	rec := doCallbackRequest(t, h, wf.ID.String(),
		`{"status":"bogus","result_json":{}}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err, "error response must be valid JSON")
	assert.NotNil(t, body["error"], "JSON envelope must contain 'error' key")
}
