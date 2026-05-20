// audit_query_handlers_test.go — 감사 로그 검색 HTTP API 핸들러 단위 테스트
// (SPEC-AX-AUDIT-QUERY-001)
//
// 격리 전략: httptest + fake WorkflowStore/WorkflowTx (score_handlers_test.go 동형).
// integration 빌드 태그 미사용 — 기본 `go test ./apps/control-plane/cmd/server/`로 실행.
// 본 SPEC은 read-only이므로 testcontainers 통합 테스트 fan-out 0 (PoC scope).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// auditQueryGoLeakOptions score_handlers_test.go scoreGoLeakOptions 동형.
// 병렬 형제 테스트 러너 + httptest 인프라 goroutine 제외 (실 핸들러 누출은 다른 top-function으로 표면화).
var auditQueryGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("testing.tRunner.func1"),
	goleak.IgnoreTopFunction("testing.tRunner"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
}

// ── fake WorkflowStore/WorkflowTx (audit_query 검증용) ──────────────────────────

// fakeAuditQueryTx WorkflowTx의 QueryAuditLogs만 사용 — 다른 mutation 메서드는 panic.
type fakeAuditQueryTx struct {
	queryErr       error
	queryEvents    []*audit.Event
	queryTotal     int64
	rollbackCalled bool
	queryCalled    bool
	queryFilter    store.AuditLogFilter
	queryLimit     int
	queryOffset    int
}

func (tx *fakeAuditQueryTx) InsertWorkflow(_ context.Context, _ *types.Workflow) error {
	panic("InsertWorkflow not expected in read-only audit query")
}
func (tx *fakeAuditQueryTx) InsertAuditLog(_ context.Context, _ *audit.Event) error {
	panic("InsertAuditLog not expected — read-only audit query MUST NOT INSERT")
}
func (tx *fakeAuditQueryTx) UpdateWorkflowState(_ context.Context, _ string, _ types.WorkflowState) error {
	panic("UpdateWorkflowState not expected")
}
func (tx *fakeAuditQueryTx) GetWorkflow(_ context.Context, _ string) (*types.Workflow, error) {
	panic("GetWorkflow not expected")
}
func (tx *fakeAuditQueryTx) UpdateWorkflowResult(_ context.Context, _ string, _ []byte) error {
	panic("UpdateWorkflowResult not expected")
}
func (tx *fakeAuditQueryTx) QueryAuditLogs(
	_ context.Context, filter store.AuditLogFilter, limit, offset int,
) ([]*audit.Event, int64, error) {
	tx.queryCalled = true
	tx.queryFilter = filter
	tx.queryLimit = limit
	tx.queryOffset = offset
	if tx.queryErr != nil {
		return nil, 0, tx.queryErr
	}
	return tx.queryEvents, tx.queryTotal, nil
}
func (tx *fakeAuditQueryTx) Commit(_ context.Context) error { return nil }
func (tx *fakeAuditQueryTx) Rollback(_ context.Context) error {
	tx.rollbackCalled = true
	return nil
}

// fakeAuditQueryStore WorkflowStore — BeginTx로 fakeAuditQueryTx 반환
type fakeAuditQueryStore struct {
	tx          *fakeAuditQueryTx
	beginErr    error
	beginCalled bool
}

func (s *fakeAuditQueryStore) BeginTx(_ context.Context) (store.WorkflowTx, error) {
	s.beginCalled = true
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	return s.tx, nil
}
func (s *fakeAuditQueryStore) ListWorkflows(_ context.Context, _, _ int) ([]*types.Workflow, error) {
	return nil, nil
}

// newTestAuditQueryHandler fake store로 AuditQueryHandler 구성
func newTestAuditQueryHandler(t *testing.T, tx *fakeAuditQueryTx) (*AuditQueryHandler, *fakeAuditQueryStore) {
	t.Helper()
	st := &fakeAuditQueryStore{tx: tx}
	h := NewAuditQueryHandler(st, zaptest.NewLogger(t))
	return h, st
}

// doAuditQueryReq Routes() 핸들러에 요청을 보내고 status + raw body 반환
func doAuditQueryReq(t *testing.T, h *AuditQueryHandler, method, target string, body string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rdr)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	var parsed map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed) //nolint:errcheck
	return rec.Code, parsed
}

// ════════════════════════════════════════════════════════════════════════════
// T-101 [E1] GET /api/v1/audit-logs (admin) → 200 + JSON schema
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_AdminScope_200(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	now := time.Now()
	rid := uuid.New()
	tx := &fakeAuditQueryTx{
		queryEvents: []*audit.Event{
			{Timestamp: now, Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: rid, ResourceType: "score"},
		},
		queryTotal: 1,
	}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
	assert.Contains(t, parsed, "events")
	assert.Contains(t, parsed, "count")
	assert.Contains(t, parsed, "total")
	assert.Contains(t, parsed, "generated_at")
	events := parsed["events"].([]any)
	assert.Len(t, events, 1)
	assert.Equal(t, float64(1), parsed["count"])
	assert.Equal(t, float64(1), parsed["total"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-102 [E1] 5-filter AND
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_FiveFilters_AND(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	rid := uuid.New()
	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	target := "/api/v1/audit-logs?action=SCORE_CREATED&resource_type=score&resource_id=" + rid.String() +
		"&user_id=alice&since=2026-01-01T00:00:00Z&until=2026-05-20T00:00:00Z"
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 핸들러가 5-필터를 모두 store에 전달했는지 검증
	require.NotNil(t, tx.queryFilter.Action)
	assert.Equal(t, "SCORE_CREATED", *tx.queryFilter.Action)
	require.NotNil(t, tx.queryFilter.ResourceType)
	assert.Equal(t, "score", *tx.queryFilter.ResourceType)
	require.NotNil(t, tx.queryFilter.ResourceID)
	assert.Equal(t, rid, *tx.queryFilter.ResourceID)
	require.NotNil(t, tx.queryFilter.UserID)
	assert.Equal(t, "alice", *tx.queryFilter.UserID)
	require.NotNil(t, tx.queryFilter.Since)
	require.NotNil(t, tx.queryFilter.Until)
}

// ════════════════════════════════════════════════════════════════════════════
// T-103 [E2] 결과 0건 → 200 빈 응답 (404 비반환)
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_NoMatches_200Empty(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	status, body := doAuditQueryReq(t, h, http.MethodGet, "/api/v1/audit-logs?action=UNKNOWN", "")
	assert.Equal(t, http.StatusOK, status, "결과 0건은 200 (404 비반환)")
	events, ok := body["events"].([]any)
	require.True(t, ok)
	assert.Empty(t, events)
	assert.Equal(t, float64(0), body["count"])
	assert.Equal(t, float64(0), body["total"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-104 [U1] malformed resource_id → 400
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_MalformedResourceID_400(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{}
	h, st := newTestAuditQueryHandler(t, tx)

	status, body := doAuditQueryReq(t, h, http.MethodGet, "/api/v1/audit-logs?resource_id=not-a-uuid", "")
	assert.Equal(t, http.StatusBadRequest, status)
	errObj, _ := body["error"].(map[string]any)
	require.NotNil(t, errObj)
	assert.Equal(t, "INVALID_ARGUMENT", errObj["code"])
	assert.Contains(t, errObj["message"], "resource_id")
	assert.False(t, st.beginCalled, "malformed UUID는 store 미진입 (pre-store 검증)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-105 [U1] since > until → 400
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_SinceAfterUntil_400(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{}
	h, st := newTestAuditQueryHandler(t, tx)

	target := "/api/v1/audit-logs?since=2026-12-31T00:00:00Z&until=2026-01-01T00:00:00Z"
	status, body := doAuditQueryReq(t, h, http.MethodGet, target, "")
	assert.Equal(t, http.StatusBadRequest, status)
	errObj, _ := body["error"].(map[string]any)
	require.NotNil(t, errObj)
	assert.Equal(t, "INVALID_ARGUMENT", errObj["code"])
	assert.False(t, st.beginCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-106 [U1] future timestamp → 400
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_FutureTimestamp_400(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{}
	h, st := newTestAuditQueryHandler(t, tx)

	target := "/api/v1/audit-logs?until=2099-12-31T23:59:59Z"
	status, body := doAuditQueryReq(t, h, http.MethodGet, target, "")
	assert.Equal(t, http.StatusBadRequest, status)
	errObj, _ := body["error"].(map[string]any)
	require.NotNil(t, errObj)
	assert.Equal(t, "INVALID_ARGUMENT", errObj["code"])
	assert.False(t, st.beginCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-107 [U2] unknown action → 200 빈 결과 (free string)
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_UnknownAction_200Empty(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	status, body := doAuditQueryReq(t, h, http.MethodGet, "/api/v1/audit-logs?action=UNKNOWN_ACTION_XYZ", "")
	assert.Equal(t, http.StatusOK, status, "unknown action은 자유 문자열 — 200 빈 결과")
	events, _ := body["events"].([]any)
	assert.Empty(t, events)
}

// ════════════════════════════════════════════════════════════════════════════
// T-108 [E1/O1] limit clamp
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_LimitClampedToMax(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	status, _ := doAuditQueryReq(t, h, http.MethodGet, "/api/v1/audit-logs?limit=10000", "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 500, tx.queryLimit, "limit=10000 → clampPagination max 500")
}

// ════════════════════════════════════════════════════════════════════════════
// T-109 [E1/O1] negative limit/offset → default/0
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_NegativeLimitOffset_DefaultsApplied(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	status, _ := doAuditQueryReq(t, h, http.MethodGet, "/api/v1/audit-logs?limit=-5&offset=-10", "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 50, tx.queryLimit, "limit=-5 → default 50")
	assert.Equal(t, 0, tx.queryOffset, "offset=-10 → 0")
}

// ════════════════════════════════════════════════════════════════════════════
// T-110 [E1] 1건만 매치
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_List_OneMatch(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	rid := uuid.New()
	tx := &fakeAuditQueryTx{
		queryEvents: []*audit.Event{
			{Timestamp: time.Now(), Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: rid},
		},
		queryTotal: 1,
	}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?action=SCORE_CREATED", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["count"])
	assert.Equal(t, float64(1), body["total"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-201~T-205: ABAC admin-only narrowing
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_AdminScope_200(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "admin scope → 200")
}

func TestAuditQueryHandler_ViewerScope_403(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{}
	h, st := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:viewer"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code, "viewer → 403 ABAC_CONDITION_DENIED")

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body) //nolint:errcheck
	errObj, _ := body["error"].(map[string]any)
	require.NotNil(t, errObj)
	assert.Equal(t, "ABAC_CONDITION_DENIED", errObj["code"])
	assert.Contains(t, errObj["message"], "권한")
	assert.False(t, st.beginCalled, "viewer deny는 store TX 미진입")
}

func TestAuditQueryHandler_AnalystScope_403(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{}
	h, st := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:analyst"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"analyst → 403 (감사 데이터 admin only — SCORE-API-001/REPORT-001 viewer 허용과 정반대 정책)")
	assert.False(t, st.beginCalled)
}

func TestAuditQueryHandler_AuthDisabled_Passthrough(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, st := newTestAuditQueryHandler(t, tx)

	// context에 user 없음 → auth-disabled 투과
	status, _ := doAuditQueryReq(t, h, http.MethodGet, "/api/v1/audit-logs", "")
	assert.Equal(t, http.StatusOK, status, "auth-disabled (user 없음) → 투과")
	assert.True(t, st.beginCalled, "auth-disabled에서도 store 진입")
}

// T-205 RoleAdmin scope 매핑 정확성 (T-201과 중복이지만 의도적 별도 검증)
func TestAuditQueryHandler_AdminRoleHelper_ReturnsTrue(t *testing.T) {
	assert.True(t, requireAuditQueryReadRole("iroum-ax:admin"))
	assert.False(t, requireAuditQueryReadRole("iroum-ax:viewer"))
	assert.False(t, requireAuditQueryReadRole("iroum-ax:analyst"))
	assert.False(t, requireAuditQueryReadRole(""))
}

// ════════════════════════════════════════════════════════════════════════════
// T-301~T-303: 에러 매핑
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_StoreErr_InvalidFilter_400(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryErr: apperrors.ErrAuditQueryInvalidFilter}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuditQueryHandler_StoreErr_InvalidTimeRange_400(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryErr: apperrors.ErrAuditQueryInvalidTimeRange}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuditQueryHandler_StoreErr_UnknownDBError_500(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryErr: errors.New("postgres connection refused")}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ════════════════════════════════════════════════════════════════════════════
// T-304: defer Rollback 검증
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_DeferRollback_Called(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.True(t, tx.rollbackCalled, "read-only이므로 defer Rollback 호출 (Commit 없음)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-401: GET /api/v1/audit-logs/{id} 단건 lookup
// ════════════════════════════════════════════════════════════════════════════

func TestAuditQueryHandler_GetByID_AdminScope_200(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	targetID := uuid.New()
	tx := &fakeAuditQueryTx{
		queryEvents: []*audit.Event{
			{Timestamp: time.Now(), Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: uuid.New()},
		},
		queryTotal: 1,
	}
	h, _ := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/"+targetID.String(), nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 핸들러가 ID 필터를 store에 전달 — store가 ID 필터를 적용하여 검색
	// (resource_id 검색이지만 path /{id}는 audit_logs.id로 의도; ResourceID 필터로 매핑 정합)
	require.NotNil(t, tx.queryFilter.ResourceID, "path /{id}는 ResourceID 필터로 매핑")
	assert.Equal(t, targetID, *tx.queryFilter.ResourceID)
}

func TestAuditQueryHandler_GetByID_MalformedUUID_400(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{}
	h, st := newTestAuditQueryHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/not-a-uuid", nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, st.beginCalled, "malformed UUID는 store 미진입")
}

func TestAuditQueryHandler_GetByID_NotFound_200Empty(t *testing.T) {
	defer goleak.VerifyNone(t, auditQueryGoLeakOptions...)

	tx := &fakeAuditQueryTx{queryEvents: []*audit.Event{}, queryTotal: 0}
	h, _ := newTestAuditQueryHandler(t, tx)

	missingID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/"+missingID.String(), nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	// data-completeness: 단건도 빈 결과 200 (UBI-002 read-only)
	assert.Equal(t, http.StatusOK, rec.Code, "단건 lookup도 결과 0건 → 200 빈 응답")
}
