// review_handlers_test.go — 평가 검토 요청 HTTP API 핸들러 단위 테스트 (SPEC-AX-REVIEW-001)
//
// 격리 전략: httptest + fake ScoreReviewRequestStore/Tx + fake ScoreStore/Tx
// (score_handlers_test.go의 fake 패턴 동형 — 통합은 store-layer integration test가 커버).
// integration 빌드 태그 미사용 — 기본 `go test ./apps/control-plane/cmd/server/`로 실행.
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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// reviewGoLeakOptions 핸들러 테스트 goroutine 누출 검증용 (score_handlers_test.go:42 동형)
var reviewGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("testing.tRunner.func1"),
	goleak.IgnoreTopFunction("testing.tRunner"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
}

// ── fake ScoreReviewRequestStore/Tx ──────────────────────────────────────────

// fakeReviewTx store.ScoreReviewRequestTx 인메모리 fake.
// 필드 순서(govet fieldalignment 최적화): 문자열(16B) × 2 → 슬라이스(24B) → 포인터(8B) × 9 →
// 값집계(uuid 16B) → int64(8B) → bool(1B) × 6
type fakeReviewTx struct {
	commitErr      error
	listErr        error
	insertErr      error
	assignErr      error
	approveErr     error
	rejectErr      error
	getByIDErr     error
	countErr       error
	getByIDResult  *store.ScoreReviewRequest
	gotReviewerID  string
	gotUserID      string
	listResult     []*store.ScoreReviewRequest
	countResult    int64
	insertID       uuid.UUID
	insertCalled   bool
	assignCalled   bool
	approveCalled  bool
	rejectCalled   bool
	commitCalled   bool
	rollbackCalled bool
}

func (f *fakeReviewTx) InsertScoreReviewRequest(_ context.Context, _ uuid.UUID, _ string, _ map[string]any, userID string) (uuid.UUID, error) {
	f.insertCalled = true
	f.gotUserID = userID
	if f.insertErr != nil {
		return uuid.Nil, f.insertErr
	}
	return f.insertID, nil
}

func (f *fakeReviewTx) GetScoreReviewRequestByID(_ context.Context, _ uuid.UUID) (*store.ScoreReviewRequest, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	return f.getByIDResult, nil
}

func (f *fakeReviewTx) ListScoreReviewRequests(_ context.Context, _ string, _ int, _ int) ([]*store.ScoreReviewRequest, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeReviewTx) CountScoreReviewRequests(_ context.Context, _ string) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.countResult, nil
}

func (f *fakeReviewTx) AssignReviewer(_ context.Context, _ uuid.UUID, reviewerID, userID string) error {
	f.assignCalled = true
	f.gotReviewerID = reviewerID
	f.gotUserID = userID
	return f.assignErr
}

func (f *fakeReviewTx) ApproveRequest(_ context.Context, _ uuid.UUID, _ string, userID string) error {
	f.approveCalled = true
	f.gotUserID = userID
	return f.approveErr
}

func (f *fakeReviewTx) RejectRequest(_ context.Context, _ uuid.UUID, _, _, userID string) error {
	f.rejectCalled = true
	f.gotUserID = userID
	return f.rejectErr
}

func (f *fakeReviewTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }

func (f *fakeReviewTx) Commit(_ context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakeReviewTx) Rollback(_ context.Context) error {
	f.rollbackCalled = true
	return nil
}

// fakeReviewStore store.ScoreReviewRequestStore fake
type fakeReviewStore struct {
	tx       *fakeReviewTx
	beginErr error
}

func (f *fakeReviewStore) BeginScoreReviewRequestTx(_ context.Context) (store.ScoreReviewRequestTx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// ── newTestReviewHandler 헬퍼 ────────────────────────────────────────────────

func newTestReviewHandler(t *testing.T, reviewTx *fakeReviewTx, scoreTx *fakeScoreTx) (*ReviewHandler, *fakeReviewStore, *fakeScoreStore) {
	t.Helper()
	rs := &fakeReviewStore{tx: reviewTx}
	ss := &fakeScoreStore{tx: scoreTx}
	h := NewReviewHandler(rs, ss, zaptest.NewLogger(t))
	return h, rs, ss
}

// doReviewReq Routes() 핸들러에 요청 보내고 status + parsed body 반환
func doReviewReq(t *testing.T, h *ReviewHandler, method, target string, body string, userScope string) (int, map[string]any) {
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

// buildFakeReview 응답 DTO 검증을 위한 entity 생성 헬퍼
func buildFakeReview(status string) *store.ScoreReviewRequest {
	now := time.Now().UTC()
	return &store.ScoreReviewRequest{
		ID:        uuid.New(),
		ScoreID:   uuid.New(),
		Status:    status,
		CreatedAt: now,
		CreatedBy: "cli-anonymous",
		UpdatedAt: now,
		UpdatedBy: "cli-anonymous",
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-201 [RED→GREEN] POST /reviews/{id}/assign-reviewer admin 성공 200
// AC-REVIEW-003-1 (§A.1)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_ReviewsAssignReviewer_AdminSucceeds_200(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	tx := &fakeReviewTx{
		getByIDResult: &store.ScoreReviewRequest{
			ID:        id,
			ScoreID:   uuid.New(),
			Status:    "UNDER_REVIEW",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	h, _, _ := newTestReviewHandler(t, tx, &fakeScoreTx{})

	target := "/api/v1/reviews/" + id.String() + "/assign-reviewer"
	code, body := doReviewReq(t, h, "POST", target, `{"reviewer_id":"user-123"}`, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code, "admin AssignReviewer 200 (body=%v)", body)
	assert.True(t, tx.assignCalled, "AssignReviewer 호출됨")
	assert.True(t, tx.commitCalled, "Commit 호출됨")
}

// ════════════════════════════════════════════════════════════════════════════
// T-202 [RED→GREEN] POST /reviews/{id}/approve admin 성공 200
// AC-REVIEW-003-2 (§A.2)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_ReviewsApprove_AdminSucceeds_200(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	tx := &fakeReviewTx{
		getByIDResult: buildFakeReview("APPROVED"),
	}
	h, _, _ := newTestReviewHandler(t, tx, &fakeScoreTx{})

	target := "/api/v1/reviews/" + id.String() + "/approve"
	code, _ := doReviewReq(t, h, "POST", target, `{"comment":"승인합니다"}`, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code)
	assert.True(t, tx.approveCalled)
	assert.True(t, tx.commitCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-203 [RED→GREEN] POST /reviews/{id}/reject rejection_reason 누락 400
// Edge E9 (§A.5 Layer 1) — handler-side validation
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_ReviewsReject_RejectionReasonMissing_400(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	tx := &fakeReviewTx{}
	h, _, _ := newTestReviewHandler(t, tx, &fakeScoreTx{})

	target := "/api/v1/reviews/" + id.String() + "/reject"
	cases := []struct {
		name string
		body string
	}{
		{"empty reason", `{"rejection_reason":""}`},
		{"missing reason", `{"comment":"some comment"}`},
		{"whitespace only", `{"rejection_reason":"   "}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := doReviewReq(t, h, "POST", target, tc.body, "iroum-ax:admin")
			require.Equal(t, http.StatusBadRequest, code, "rejection_reason 누락은 400 (body=%v)", body)

			errMap, ok := body["error"].(map[string]any)
			require.True(t, ok, "error 객체 존재")
			assert.Equal(t, "INVALID_ARGUMENT", errMap["code"])
			assert.Equal(t, "반려 사유는 필수입니다", errMap["message"], "한국어 메시지 정합")
			assert.False(t, tx.rejectCalled, "store 미진입 (handler-side fail-closed)")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-204 [RED→GREEN] POST /reviews — score 미존재 시 404 (cross-store TX-1)
// Edge E2 (§A.3) — handler-compose 2-TX score 검증 단계에서 검출
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_Reviews_ScoreNotExist_404(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	reviewTx := &fakeReviewTx{}
	// scoreTx: GetScoreByID가 ErrScoreNotFound 반환 → TX-1 단계에서 404
	scoreTx := &fakeScoreTx{getByIDErr: apperrors.ErrScoreNotFound}
	h, _, _ := newTestReviewHandler(t, reviewTx, scoreTx)

	body := `{"score_id":"` + uuid.New().String() + `"}`
	code, parsed := doReviewReq(t, h, "POST", "/api/v1/reviews", body, "iroum-ax:analyst")
	require.Equal(t, http.StatusNotFound, code, "score 미존재 → 404 (cross-store TX-1)")

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "NOT_FOUND", errMap["code"])
	assert.Equal(t, "검토 대상 점수를 찾을 수 없습니다", errMap["message"],
		"score not-found 메시지는 generic '평가 검토' 메시지와 구분되어야 한다 (cross-store 단계 식별)")
	assert.False(t, reviewTx.insertCalled, "review store TX-2 미진입 (fail-closed)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-301 [RED→GREEN] POST /reviews analyst 생성 201
// AC-REVIEW-002-1 (§A.4 analyst=submit)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_Reviews_AnalystCreates_201(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	newID := uuid.New()
	reviewTx := &fakeReviewTx{insertID: newID}
	scoreTx := &fakeScoreTx{
		getByIDResult: &store.Score{ID: uuid.New(), EvaluationItemID: "AX-001", Level: "raw", Status: "DRAFT"},
	}
	h, _, _ := newTestReviewHandler(t, reviewTx, scoreTx)

	scoreID := uuid.New().String()
	body := `{"score_id":"` + scoreID + `","comment":"검토 요청"}`
	code, parsed := doReviewReq(t, h, "POST", "/api/v1/reviews", body, "iroum-ax:analyst")
	require.Equal(t, http.StatusCreated, code, "analyst 생성은 201")
	assert.True(t, reviewTx.insertCalled)
	assert.True(t, reviewTx.commitCalled)
	assert.Equal(t, "SUBMITTED", parsed["status"])
	assert.Equal(t, newID.String(), parsed["id"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-302 [RED→GREEN] POST /reviews viewer 403
// Edge E4 (§A.4)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_Reviews_ViewerForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	reviewTx := &fakeReviewTx{}
	h, _, _ := newTestReviewHandler(t, reviewTx, &fakeScoreTx{})

	code, parsed := doReviewReq(t, h, "POST", "/api/v1/reviews", `{"score_id":"`+uuid.New().String()+`"}`, "iroum-ax:viewer")
	require.Equal(t, http.StatusForbidden, code)

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, auth.ErrCodeABACDenied, errMap["code"])
	assert.Equal(t, "제출 권한이 없는 사용자입니다", errMap["message"])
	assert.False(t, reviewTx.insertCalled, "viewer는 store 미진입")
}

// ════════════════════════════════════════════════════════════════════════════
// T-303 [RED→GREEN] POST /reviews/{id}/approve analyst 403
// Edge E5 (§A.4 admin-only)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_ReviewsApprove_AnalystForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	reviewTx := &fakeReviewTx{}
	h, _, _ := newTestReviewHandler(t, reviewTx, &fakeScoreTx{})

	target := "/api/v1/reviews/" + id.String() + "/approve"
	code, parsed := doReviewReq(t, h, "POST", target, `{}`, "iroum-ax:analyst")
	require.Equal(t, http.StatusForbidden, code)

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, auth.ErrCodeABACDenied, errMap["code"])
	assert.Equal(t, "관리자 권한이 없는 사용자입니다", errMap["message"])
	assert.False(t, reviewTx.approveCalled, "analyst는 approve store 미진입")
}

// ════════════════════════════════════════════════════════════════════════════
// T-304 [RED→GREEN] POST /reviews/{id}/assign-reviewer analyst 403
// Edge E6 (§A.4 admin-only)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_ReviewsAssignReviewer_AnalystForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	reviewTx := &fakeReviewTx{}
	h, _, _ := newTestReviewHandler(t, reviewTx, &fakeScoreTx{})

	target := "/api/v1/reviews/" + id.String() + "/assign-reviewer"
	code, _ := doReviewReq(t, h, "POST", target, `{"reviewer_id":"r-1"}`, "iroum-ax:analyst")
	require.Equal(t, http.StatusForbidden, code)
	assert.False(t, reviewTx.assignCalled, "analyst는 assign store 미진입")
}

// ════════════════════════════════════════════════════════════════════════════
// T-305 [RED→GREEN] auth-disabled 모든 엔드포인트 투과
// AC-REVIEW-UBI-003, Edge E7
// ════════════════════════════════════════════════════════════════════════════

func TestReviews_AuthDisabled_PassthroughCliAnonymous(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	newID := uuid.New()
	// fake getByIDResult는 cli-anonymous로 영속된 entity를 흉내냄 (D2 응답 body 단언 대상)
	cliAnonReview := buildFakeReviewWithUser("UNDER_REVIEW", "cli-anonymous", "cli-anonymous")
	reviewTx := &fakeReviewTx{
		insertID:      newID,
		getByIDResult: cliAnonReview,
		listResult:    []*store.ScoreReviewRequest{cliAnonReview},
	}
	scoreTx := &fakeScoreTx{
		getByIDResult: &store.Score{ID: uuid.New(), Level: "raw", Status: "DRAFT"},
	}
	h, _, _ := newTestReviewHandler(t, reviewTx, scoreTx)

	// userScope="" → auth context 없음 (auth-disabled), 모든 6 엔드포인트 투과 검증
	// D2: status code 외에 응답 body created_by/updated_by가 'cli-anonymous'인지 단언
	cases := []struct {
		name           string
		method         string
		target         string
		body           string
		expCode        int
		checkCreatedBy bool // 응답 body created_by/updated_by 단언 여부
	}{
		{"create", "POST", "/api/v1/reviews", `{"score_id":"` + uuid.New().String() + `"}`, http.StatusCreated, true},
		{"get", "GET", "/api/v1/reviews/" + id.String(), "", http.StatusOK, true},
		{"assign", "POST", "/api/v1/reviews/" + id.String() + "/assign-reviewer", `{"reviewer_id":"r-1"}`, http.StatusOK, true},
		{"approve", "POST", "/api/v1/reviews/" + id.String() + "/approve", `{}`, http.StatusOK, true},
		{"reject", "POST", "/api/v1/reviews/" + id.String() + "/reject", `{"rejection_reason":"근거 부족"}`, http.StatusOK, true},
		{"list", "GET", "/api/v1/reviews", "", http.StatusOK, false}, // list는 entity 직접 조회 — 별도 검증
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := doReviewReq(t, h, tc.method, tc.target, tc.body, "")
			require.Equal(t, tc.expCode, code, "auth-disabled에서 %s는 %d (투과)", tc.name, tc.expCode)
			if tc.checkCreatedBy {
				// D2: 응답 body의 created_by/updated_by가 'cli-anonymous' 리터럴 (UBI-003 + Edge E7)
				assert.Equal(t, "cli-anonymous", body["created_by"], "%s 응답 body created_by='cli-anonymous'", tc.name)
				assert.Equal(t, "cli-anonymous", body["updated_by"], "%s 응답 body updated_by='cli-anonymous'", tc.name)
			}
		})
	}

	// store 호출 시 userID가 'cli-anonymous'로 전달되었는지 검증 (D1)
	assert.Equal(t, "cli-anonymous", reviewTx.gotUserID,
		"store는 'cli-anonymous' userID로 호출되어야 함 (D1 fix, UBI-003)")
}

// buildFakeReviewWithUser created_by/updated_by 명시적 설정 — D2 응답 body 단언용
func buildFakeReviewWithUser(status, createdBy, updatedBy string) *store.ScoreReviewRequest {
	now := time.Now().UTC()
	return &store.ScoreReviewRequest{
		ID:        uuid.New(),
		ScoreID:   uuid.New(),
		Status:    status,
		CreatedAt: now,
		CreatedBy: createdBy,
		UpdatedAt: now,
		UpdatedBy: updatedBy,
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-305-D1 [NEW iteration 2] auth-enabled principal.id가 store에 전달됨
// AC-REVIEW-UBI-003: auth-enabled 시 store는 principal.UID로 호출되어야 함 (cli-anonymous 아님)
// ════════════════════════════════════════════════════════════════════════════

func TestReviews_AuthEnabled_PrincipalIDPropagatedToStore(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	authedReview := buildFakeReviewWithUser("UNDER_REVIEW", "user-alice", "admin-bob")
	reviewTx := &fakeReviewTx{
		insertID:      uuid.New(),
		getByIDResult: authedReview,
	}
	scoreTx := &fakeScoreTx{
		getByIDResult: &store.Score{ID: uuid.New(), Level: "raw", Status: "DRAFT"},
	}
	h, _, _ := newTestReviewHandler(t, reviewTx, scoreTx)

	// auth-enabled 시뮬레이션: doReviewReq의 userScope 파라미터로 WithUser 주입
	// (test-user UID + iroum-ax:admin scope)
	code, _ := doReviewReq(t, h, "POST", "/api/v1/reviews",
		`{"score_id":"`+uuid.New().String()+`"}`, "iroum-ax:admin")
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "test-user", reviewTx.gotUserID,
		"auth-enabled 시 store는 principal.UID 'test-user'로 호출되어야 함 (D1 fix)")

	// approve 경로
	reviewTx.gotUserID = "" // reset
	code, _ = doReviewReq(t, h, "POST", "/api/v1/reviews/"+id.String()+"/approve", `{}`, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "test-user", reviewTx.gotUserID,
		"approve: auth-enabled principal.UID 전달")

	// reject 경로
	reviewTx.gotUserID = "" // reset
	code, _ = doReviewReq(t, h, "POST", "/api/v1/reviews/"+id.String()+"/reject", `{"rejection_reason":"X"}`, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "test-user", reviewTx.gotUserID,
		"reject: auth-enabled principal.UID 전달")

	// assign-reviewer 경로
	reviewTx.gotUserID = "" // reset
	code, _ = doReviewReq(t, h, "POST", "/api/v1/reviews/"+id.String()+"/assign-reviewer",
		`{"reviewer_id":"r-1"}`, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "test-user", reviewTx.gotUserID,
		"assign-reviewer: auth-enabled principal.UID 전달")
}

// ════════════════════════════════════════════════════════════════════════════
// T-306 [RED→GREEN] GET /reviews 빈 결과 200 + items:[]
// AC-REVIEW-002-3, Edge E8
// ════════════════════════════════════════════════════════════════════════════

func TestGET_Reviews_EmptyList_Returns200WithEmptyArray(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	reviewTx := &fakeReviewTx{
		listResult:  []*store.ScoreReviewRequest{},
		countResult: 0,
	}
	h, _, _ := newTestReviewHandler(t, reviewTx, &fakeScoreTx{})

	code, parsed := doReviewReq(t, h, "GET", "/api/v1/reviews?status=SUBMITTED", "", "iroum-ax:viewer")
	require.Equal(t, http.StatusOK, code, "빈 결과는 200 (404 아님)")
	items, ok := parsed["items"].([]any)
	require.True(t, ok, "items 배열 존재")
	assert.Len(t, items, 0)
	assert.Equal(t, float64(0), parsed["total"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-308-D4 [NEW iteration 2] GET /reviews total = full COUNT(*) (pagination 미적용)
// AC-REVIEW-002-3: total은 limit/offset 적용 전 전체 행 수
// ════════════════════════════════════════════════════════════════════════════

func TestGET_Reviews_TotalIsFullCountIgnoringPagination(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	// fake: store list는 20건 반환(pagination 후), count는 100건 반환 (전체)
	reviewTx := &fakeReviewTx{
		listResult:  make([]*store.ScoreReviewRequest, 20),
		countResult: 100,
	}
	for i := range reviewTx.listResult {
		reviewTx.listResult[i] = buildFakeReviewWithUser("SUBMITTED", "user-alice", "user-alice")
	}
	h, _, _ := newTestReviewHandler(t, reviewTx, &fakeScoreTx{})

	code, parsed := doReviewReq(t, h, "GET", "/api/v1/reviews?status=SUBMITTED&limit=20&offset=10", "", "iroum-ax:viewer")
	require.Equal(t, http.StatusOK, code)
	items, ok := parsed["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 20, "items는 pagination 적용된 20건")
	assert.Equal(t, float64(100), parsed["total"],
		"total은 limit/offset 무시한 full COUNT (AC-REVIEW-002-3 fix, D4)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-307 [RED→GREEN] GET /reviews/{id} malformed UUID 400
// AC-REVIEW-002-4, Edge E11
// ════════════════════════════════════════════════════════════════════════════

func TestGET_Reviews_MalformedUUID_400(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	reviewTx := &fakeReviewTx{}
	h, _, _ := newTestReviewHandler(t, reviewTx, &fakeScoreTx{})

	code, parsed := doReviewReq(t, h, "GET", "/api/v1/reviews/not-a-uuid", "", "")
	require.Equal(t, http.StatusBadRequest, code)
	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INVALID_ARGUMENT", errMap["code"])
	assert.Equal(t, "유효하지 않은 평가 검토 ID 형식입니다", errMap["message"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-308 [RED→GREEN] GET /reviews pagination clamp (limit > 500 → 500, offset < 0 → 0)
// AC-REVIEW-002-3, Edge E12+E13
// ════════════════════════════════════════════════════════════════════════════

func TestClampReviewPagination_LimitAndOffset(t *testing.T) {
	cases := []struct {
		name      string
		rawLimit  string
		rawOffset string
		expLimit  int
		expOffset int
	}{
		{"limit empty → 50", "", "", 50, 0},
		{"limit 100 → 100", "100", "0", 100, 0},
		{"limit 10000 → 500 clamp", "10000", "0", 500, 0},
		{"offset -5 → 0 clamp", "20", "-5", 20, 0},
		{"offset valid", "10", "30", 10, 30},
		{"limit 0 → default 50", "0", "0", 50, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limit, offset := clampReviewPagination(tc.rawLimit, tc.rawOffset)
			assert.Equal(t, tc.expLimit, limit)
			assert.Equal(t, tc.expOffset, offset)
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 보조 검증: mapReviewStoreErr 결정적 매핑 (AC-REVIEW-005-1, 7 sentinels)
// ════════════════════════════════════════════════════════════════════════════

func TestMapReviewStoreErr_DeterministicMapping(t *testing.T) {
	cases := []struct {
		err     error
		expEC   string
		expCode int
	}{
		{apperrors.ErrScoreReviewRequestNotFound, "NOT_FOUND", http.StatusNotFound},
		{apperrors.ErrScoreNotFound, "NOT_FOUND", http.StatusNotFound},
		{apperrors.ErrScoreReviewRequestInvalidInput, "INVALID_ARGUMENT", http.StatusBadRequest},
		{apperrors.ErrScoreReviewRequestInvalidStatus, "CONFLICT", http.StatusConflict},
		{apperrors.ErrScoreReviewRequestNotSubmitted, "CONFLICT", http.StatusConflict},
		{apperrors.ErrScoreReviewRequestNotUnderReview, "CONFLICT", http.StatusConflict},
		{apperrors.ErrScoreReviewRequestAuditWriteFailed, "INTERNAL", http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.err.Error(), func(t *testing.T) {
			code, errCode, msg := mapReviewStoreErr(c.err)
			assert.Equal(t, c.expCode, code)
			assert.Equal(t, c.expEC, errCode)
			assert.NotEmpty(t, msg, "한국어 메시지 비어있으면 안 됨")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 보조 검증: Routes() 6 패턴 등록 + ServeMux 최장일치 우선순위
// ════════════════════════════════════════════════════════════════════════════

func TestReviewHandler_RoutesRegistersSixPatterns(t *testing.T) {
	defer goleak.VerifyNone(t, reviewGoLeakOptions...)

	id := uuid.New()
	tx := &fakeReviewTx{
		getByIDResult: buildFakeReview("UNDER_REVIEW"),
		listResult:    []*store.ScoreReviewRequest{},
		insertID:      uuid.New(),
	}
	h, _, _ := newTestReviewHandler(t, tx, &fakeScoreTx{
		getByIDResult: &store.Score{ID: uuid.New(), Level: "raw", Status: "DRAFT"},
	})

	// 구체 경로 (assign/approve/reject)가 /{id}보다 우선 매칭되는지 검증
	cases := []struct {
		method  string
		target  string
		body    string
		expCode int
	}{
		{"POST", "/api/v1/reviews/" + id.String() + "/assign-reviewer", `{"reviewer_id":"r-1"}`, http.StatusOK},
		{"POST", "/api/v1/reviews/" + id.String() + "/approve", `{}`, http.StatusOK},
		{"POST", "/api/v1/reviews/" + id.String() + "/reject", `{"rejection_reason":"근거 부족"}`, http.StatusOK},
		{"GET", "/api/v1/reviews/" + id.String(), "", http.StatusOK},
		{"GET", "/api/v1/reviews", "", http.StatusOK},
		{"POST", "/api/v1/reviews", `{"score_id":"` + uuid.New().String() + `"}`, http.StatusCreated},
	}
	for _, tc := range cases {
		code, _ := doReviewReq(t, h, tc.method, tc.target, tc.body, "iroum-ax:admin")
		assert.Equal(t, tc.expCode, code, "%s %s 라우트 등록 확인", tc.method, tc.target)
	}
}
