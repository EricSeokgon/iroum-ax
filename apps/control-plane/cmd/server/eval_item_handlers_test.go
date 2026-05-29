// eval_item_handlers_test.go — 평가항목(EvalItem) REST API 핸들러 단위 테스트
//
// 격리 전략: httptest + evalItemHndlTx/Store (report_handlers_test.go 충돌 방지 위해 별도 명명).
// integration 빌드 태그 미사용 — 기본 `go test ./apps/control-plane/cmd/server/`로 실행.
//
// 테스트 카운트: 14건
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// evalItemGoLeakOptions goroutine 누출 검증용
var evalItemGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("testing.tRunner.func1"),
	goleak.IgnoreTopFunction("testing.tRunner"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
}

// ── fake EvalItemTx (핸들러 전용 — report_handlers_test.go의 fakeEvalItemTx와 충돌 방지) ──

// evalItemHndlTx eval_item_handlers_test.go 전용 store.EvalItemTx fake.
// InsertErr/GetErr/UpdateErr 개별 필드로 핸들러 경로 분기 검증.
type evalItemHndlTx struct {
	childrenResult []*store.EvalItem
	getResult      *store.EvalItem
	insertErr      error
	getErr         error
	childrenErr    error
	updateErr      error
	commitErr      error
	insertedID     string
	updateCalled   bool
	commitCalled   bool
	rollbackCalled bool
}

func (f *evalItemHndlTx) InsertEvalItem(_ context.Context, id string, _ *string, _, _ string, _ *int, _ string, _ *float64, _ *int, _ map[string]any) (string, error) {
	if f.insertErr != nil {
		return "", f.insertErr
	}
	if f.insertedID != "" {
		return f.insertedID, nil
	}
	return id, nil
}

func (f *evalItemHndlTx) GetEvalItemByID(_ context.Context, _ string) (*store.EvalItem, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *evalItemHndlTx) GetEvalItemsByParentID(_ context.Context, _ string) ([]*store.EvalItem, error) {
	if f.childrenErr != nil {
		return nil, f.childrenErr
	}
	return f.childrenResult, nil
}

func (f *evalItemHndlTx) UpdateEvalItem(_ context.Context, _ string, _ store.EvalItemUpdate) error {
	f.updateCalled = true
	return f.updateErr
}

func (f *evalItemHndlTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }

func (f *evalItemHndlTx) Commit(_ context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *evalItemHndlTx) Rollback(_ context.Context) error {
	f.rollbackCalled = true
	return nil
}

// evalItemHndlStore eval_item_handlers_test.go 전용 store.EvalItemStore fake.
type evalItemHndlStore struct {
	tx       *evalItemHndlTx
	beginErr error
}

func (f *evalItemHndlStore) BeginEvalItemTx(_ context.Context) (store.EvalItemTx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// ── 헬퍼 ─────────────────────────────────────────────────────────────────────

func newTestEvalItemHandler(t *testing.T, tx *evalItemHndlTx) *EvalItemHandler {
	t.Helper()
	st := &evalItemHndlStore{tx: tx}
	rec := audit.NewRecorder(false)
	return NewEvalItemHandler(st, rec, zaptest.NewLogger(t))
}

func doEvalItemReq(t *testing.T, h *EvalItemHandler, method, target, body, userScope string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userScope != "" {
		ctx := auth.WithUser(req.Context(), &auth.User{UID: "test-user", Scopes: []string{userScope}})
		req = req.WithContext(ctx)
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	var parsed map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed) //nolint:errcheck
	return rec.Code, parsed
}

func buildFakeEvalItem(id string) *store.EvalItem {
	now := time.Now().UTC()
	level := 1
	return &store.EvalItem{
		ID:            id,
		DisplayName:   "항목A",
		Description:   "설명",
		HierarchyCode: "1.1",
		Status:        "ACTIVE",
		Level:         &level,
		CreatedAt:     now,
		UpdatedAt:     now,
		CreatedBy:     "cli-anonymous",
	}
}

// ════════════════════════════════════════════════════════════════════════════
// POST /api/v1/eval-items
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_EvalItems_AdminCreates_201(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{}
	h := newTestEvalItemHandler(t, tx)

	body := `{"id":"ITEM-001","display_name":"평가항목1","hierarchy_code":"1.1","description":"설명"}`
	code, resp := doEvalItemReq(t, h, "POST", "/api/v1/eval-items", body, "iroum-ax:admin")

	require.Equal(t, http.StatusCreated, code, "admin POST → 201")
	assert.True(t, tx.commitCalled, "Commit 호출")
	assert.Equal(t, "ITEM-001", resp["id"], "응답에 id 포함")
}

func TestPOST_EvalItems_NonAdminForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{}
	h := newTestEvalItemHandler(t, tx)

	body := `{"id":"ITEM-001","display_name":"평가항목1","hierarchy_code":"1.1"}`
	code, _ := doEvalItemReq(t, h, "POST", "/api/v1/eval-items", body, "iroum-ax:viewer")

	require.Equal(t, http.StatusForbidden, code, "viewer POST → 403")
	assert.False(t, tx.commitCalled, "Commit 미호출")
}

func TestPOST_EvalItems_MissingID_400(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{}
	h := newTestEvalItemHandler(t, tx)

	body := `{"display_name":"항목A","hierarchy_code":"1.1"}`
	code, resp := doEvalItemReq(t, h, "POST", "/api/v1/eval-items", body, "iroum-ax:admin")

	require.Equal(t, http.StatusBadRequest, code, "id 미제공 → 400")
	assert.NotNil(t, resp["error"])
}

func TestPOST_EvalItems_MissingDisplayName_400(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{}
	h := newTestEvalItemHandler(t, tx)

	body := `{"id":"ITEM-001","hierarchy_code":"1.1"}`
	code, _ := doEvalItemReq(t, h, "POST", "/api/v1/eval-items", body, "iroum-ax:admin")

	require.Equal(t, http.StatusBadRequest, code, "display_name 미제공 → 400")
}

func TestPOST_EvalItems_StoreParentNotFound_404(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{insertErr: apperrors.ErrEvalItemParentNotFound}
	h := newTestEvalItemHandler(t, tx)

	body := `{"id":"ITEM-002","display_name":"항목B","hierarchy_code":"1.2","parent_id":"ITEM-NONE"}`
	code, _ := doEvalItemReq(t, h, "POST", "/api/v1/eval-items", body, "iroum-ax:admin")

	require.Equal(t, http.StatusNotFound, code, "부모 없음 → 404")
}

func TestPOST_EvalItems_BeginTxError_500(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	st := &evalItemHndlStore{beginErr: apperrors.ErrEvalItemNotFound}
	rec := audit.NewRecorder(false)
	h := NewEvalItemHandler(st, rec, zaptest.NewLogger(t))

	body := `{"id":"ITEM-001","display_name":"항목A","hierarchy_code":"1.1"}`
	req := httptest.NewRequest("POST", "/api/v1/eval-items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ════════════════════════════════════════════════════════════════════════════
// GET /api/v1/eval-items/{id}
// ════════════════════════════════════════════════════════════════════════════

func TestGET_EvalItems_ByID_200(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{getResult: buildFakeEvalItem("ITEM-001")}
	h := newTestEvalItemHandler(t, tx)

	code, resp := doEvalItemReq(t, h, "GET", "/api/v1/eval-items/ITEM-001", "", "iroum-ax:viewer")

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ITEM-001", resp["id"])
}

func TestGET_EvalItems_ByID_NotFound_404(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{getErr: apperrors.ErrEvalItemNotFound}
	h := newTestEvalItemHandler(t, tx)

	code, _ := doEvalItemReq(t, h, "GET", "/api/v1/eval-items/ITEM-NONE", "", "iroum-ax:viewer")

	require.Equal(t, http.StatusNotFound, code)
}

// ════════════════════════════════════════════════════════════════════════════
// GET /api/v1/eval-items/{id}/children
// ════════════════════════════════════════════════════════════════════════════

func TestGET_EvalItems_Children_200(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	children := []*store.EvalItem{buildFakeEvalItem("ITEM-002"), buildFakeEvalItem("ITEM-003")}
	tx := &evalItemHndlTx{
		getResult:      buildFakeEvalItem("ITEM-001"),
		childrenResult: children,
	}
	h := newTestEvalItemHandler(t, tx)

	code, resp := doEvalItemReq(t, h, "GET", "/api/v1/eval-items/ITEM-001/children", "", "iroum-ax:viewer")

	require.Equal(t, http.StatusOK, code)
	items, ok := resp["items"].([]any)
	require.True(t, ok, "items 배열 존재")
	assert.Len(t, items, 2)
	assert.Equal(t, float64(2), resp["total"])
}

func TestGET_EvalItems_Children_ParentNotFound_404(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{getErr: apperrors.ErrEvalItemNotFound}
	h := newTestEvalItemHandler(t, tx)

	code, _ := doEvalItemReq(t, h, "GET", "/api/v1/eval-items/ITEM-NONE/children", "", "iroum-ax:viewer")

	require.Equal(t, http.StatusNotFound, code)
}

// ════════════════════════════════════════════════════════════════════════════
// PUT /api/v1/eval-items/{id}
// ════════════════════════════════════════════════════════════════════════════

func TestPUT_EvalItems_AdminUpdates_200(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{getResult: buildFakeEvalItem("ITEM-001")}
	h := newTestEvalItemHandler(t, tx)

	body := `{"display_name":"수정된항목"}`
	code, resp := doEvalItemReq(t, h, "PUT", "/api/v1/eval-items/ITEM-001", body, "iroum-ax:admin")

	require.Equal(t, http.StatusOK, code, "admin PUT → 200")
	assert.True(t, tx.updateCalled, "UpdateEvalItem 호출")
	assert.True(t, tx.commitCalled, "Commit 호출")
	assert.Equal(t, "updated", resp["status"])
}

func TestPUT_EvalItems_NonAdminForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{getResult: buildFakeEvalItem("ITEM-001")}
	h := newTestEvalItemHandler(t, tx)

	body := `{"display_name":"수정시도"}`
	code, _ := doEvalItemReq(t, h, "PUT", "/api/v1/eval-items/ITEM-001", body, "iroum-ax:viewer")

	require.Equal(t, http.StatusForbidden, code, "viewer PUT → 403")
	assert.False(t, tx.updateCalled, "UpdateEvalItem 미호출")
}

func TestPUT_EvalItems_NotFound_404(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{getErr: apperrors.ErrEvalItemNotFound}
	h := newTestEvalItemHandler(t, tx)

	body := `{"display_name":"수정"}`
	code, _ := doEvalItemReq(t, h, "PUT", "/api/v1/eval-items/ITEM-NONE", body, "iroum-ax:admin")

	require.Equal(t, http.StatusNotFound, code)
}

func TestPUT_EvalItems_HierarchyImmutable_409(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{
		getResult: buildFakeEvalItem("ITEM-001"),
		updateErr: apperrors.ErrEvalItemHierarchyImmutable,
	}
	h := newTestEvalItemHandler(t, tx)

	body := `{"parent_id":"ITEM-OTHER"}`
	code, _ := doEvalItemReq(t, h, "PUT", "/api/v1/eval-items/ITEM-001", body, "iroum-ax:admin")

	require.Equal(t, http.StatusConflict, code, "자식 보유 항목 부모 변경 → 409")
}

func TestPUT_EvalItems_ParentIDNull_200(t *testing.T) {
	defer goleak.VerifyNone(t, evalItemGoLeakOptions...)

	tx := &evalItemHndlTx{getResult: buildFakeEvalItem("ITEM-001")}
	h := newTestEvalItemHandler(t, tx)

	// parent_id: null → 루트로 초기화
	body := `{"parent_id":null}`
	code, _ := doEvalItemReq(t, h, "PUT", "/api/v1/eval-items/ITEM-001", body, "iroum-ax:admin")

	require.Equal(t, http.StatusOK, code, "parent_id:null → 루트 초기화 200")
	assert.True(t, tx.updateCalled)
}
