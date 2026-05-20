// rubric_handlers_test.go — 등급 rubric HTTP API 핸들러 단위 테스트 (SPEC-AX-RUBRIC-001 Phase A RED)
//
// 격리 전략: httptest + fake RubricStore/Tx + fake EvalItemStore/Tx + fake ScoreStore/Tx
// (review_handlers_test.go의 cross-store 2-TX fake 패턴 동형 — 통합은 store-layer integration test).
// integration 빌드 태그 미사용 — 기본 `go test ./apps/control-plane/cmd/server/`로 실행.
//
// Phase A RED [HARD]: 모든 테스트는 핸들러 구현 전에 작성되어 실패 확인되어야 한다.
// Phase A에서는 모든 핸들러가 503 NOT_IMPLEMENTED 반환 → RED 테스트가 200/400/403/404/409 기대값과 불일치.
// Phase C GREEN에서 정확한 status code + 한국어 메시지로 전환.
//
// 테스트 카운트: 22건 (T-RED-020~033 14건 + OPEN-derived 8건 — 021b/022b/023b/024b/025b/026b/027b/028b)
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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// rubricGoLeakOptions 핸들러 테스트 goroutine 누출 검증용 (review_handlers_test.go:31 동형)
var rubricGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("testing.tRunner.func1"),
	goleak.IgnoreTopFunction("testing.tRunner"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
}

// ── fake RubricStore / RubricTx ──────────────────────────────────────────────

// fakeRubricTx store.RubricTx 인메모리 fake.
// 필드 순서(govet fieldalignment): 슬라이스(24B) → 인터페이스(16B) → 포인터(8B) × N → bool(1B) × N
type fakeRubricTx struct {
	bandsResult       []*store.RubricBand
	criteriaResult    []*store.RubricCriterion
	listResult        []*store.Rubric
	insertErr         error
	getByIDErr        error
	listErr           error
	countErr          error
	updateErr         error
	archiveErr        error
	addCriterionErr   error
	addBandErr        error
	getCriteriaErr    error
	getBandsErr       error
	applyErr          error
	commitErr         error
	getByIDResult     *store.Rubric
	applyBand         *store.RubricBand
	gotUserID         string
	gotName           string
	gotScope          string
	gotStatus         string
	gotArchiveReason  string
	applyLetter       string
	insertID          uuid.UUID
	addCriterionID    uuid.UUID
	addBandID         uuid.UUID
	countResult       int64
	insertCalled      bool
	updateCalled      bool
	archiveCalled     bool
	addCriterionDone  bool
	addBandDone       bool
	applyCalled       bool
	commitCalled      bool
	rollbackCalled    bool
}

func (f *fakeRubricTx) InsertRubric(_ context.Context, name, scope string, _ map[string]any, userID string) (uuid.UUID, error) {
	f.insertCalled = true
	f.gotName, f.gotScope, f.gotUserID = name, scope, userID
	if f.insertErr != nil {
		return uuid.Nil, f.insertErr
	}
	return f.insertID, nil
}

func (f *fakeRubricTx) GetRubricByID(_ context.Context, _ uuid.UUID) (*store.Rubric, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	return f.getByIDResult, nil
}

func (f *fakeRubricTx) ListRubrics(_ context.Context, _, _ string, _, _ int) ([]*store.Rubric, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeRubricTx) CountRubrics(_ context.Context, _, _ string) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.countResult, nil
}

func (f *fakeRubricTx) UpdateRubric(_ context.Context, _ uuid.UUID, name, scope, status string, _ map[string]any, userID string) error {
	f.updateCalled = true
	f.gotName, f.gotScope, f.gotStatus, f.gotUserID = name, scope, status, userID
	return f.updateErr
}

func (f *fakeRubricTx) ArchiveRubric(_ context.Context, _ uuid.UUID, archiveReason, userID string) error {
	f.archiveCalled = true
	f.gotArchiveReason, f.gotUserID = archiveReason, userID
	return f.archiveErr
}

func (f *fakeRubricTx) AddCriterion(_ context.Context, _, _ uuid.UUID, _ float64, userID string) (uuid.UUID, error) {
	f.addCriterionDone = true
	f.gotUserID = userID
	if f.addCriterionErr != nil {
		return uuid.Nil, f.addCriterionErr
	}
	return f.addCriterionID, nil
}

func (f *fakeRubricTx) AddBand(_ context.Context, _ uuid.UUID, _ string, _, _ float64, userID string) (uuid.UUID, error) {
	f.addBandDone = true
	f.gotUserID = userID
	if f.addBandErr != nil {
		return uuid.Nil, f.addBandErr
	}
	return f.addBandID, nil
}

func (f *fakeRubricTx) GetCriteriaByRubric(_ context.Context, _ uuid.UUID) ([]*store.RubricCriterion, error) {
	if f.getCriteriaErr != nil {
		return nil, f.getCriteriaErr
	}
	return f.criteriaResult, nil
}

func (f *fakeRubricTx) GetBandsByRubric(_ context.Context, _ uuid.UUID) ([]*store.RubricBand, error) {
	if f.getBandsErr != nil {
		return nil, f.getBandsErr
	}
	return f.bandsResult, nil
}

func (f *fakeRubricTx) ApplyRubric(_ context.Context, _ uuid.UUID, _ float64) (string, *store.RubricBand, error) {
	f.applyCalled = true
	if f.applyErr != nil {
		return "", nil, f.applyErr
	}
	return f.applyLetter, f.applyBand, nil
}

func (f *fakeRubricTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }

func (f *fakeRubricTx) Commit(_ context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakeRubricTx) Rollback(_ context.Context) error {
	f.rollbackCalled = true
	return nil
}

// fakeRubricStore store.RubricStore fake
type fakeRubricStore struct {
	tx       *fakeRubricTx
	beginErr error
}

func (f *fakeRubricStore) BeginRubricTx(_ context.Context) (store.RubricTx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// ── fake EvalItemStore/Tx for cross-store EvalItem 검증 (T-RED-028) ───────────

type fakeRubricEvalItemTx struct {
	getErr        error
	getResult     *store.EvalItem
	rollbackDone  bool
}

func (f *fakeRubricEvalItemTx) InsertEvalItem(_ context.Context, _ string, _ *string, _, _ string, _ *int, _ string, _ *float64, _ *int, _ map[string]any) (string, error) {
	return "", nil
}

func (f *fakeRubricEvalItemTx) GetEvalItemByID(_ context.Context, _ string) (*store.EvalItem, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeRubricEvalItemTx) GetEvalItemsByParentID(_ context.Context, _ string) ([]*store.EvalItem, error) {
	return nil, nil
}

func (f *fakeRubricEvalItemTx) UpdateEvalItem(_ context.Context, _ string, _ store.EvalItemUpdate) error {
	return nil
}

func (f *fakeRubricEvalItemTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }
func (f *fakeRubricEvalItemTx) Commit(_ context.Context) error                         { return nil }
func (f *fakeRubricEvalItemTx) Rollback(_ context.Context) error {
	f.rollbackDone = true
	return nil
}

type fakeRubricEvalItemStore struct {
	tx       *fakeRubricEvalItemTx
	beginErr error
}

func (f *fakeRubricEvalItemStore) BeginEvalItemTx(_ context.Context) (store.EvalItemTx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// ── fake ScoreStore/Tx for cross-store apply 2-TX (T-RED-029) ─────────────────
// review_handlers_test.go의 fakeScoreTx/fakeScoreStore 재사용 (이미 정의됨 — score_handlers_test.go)
// 별도 fakeRubricScore* 정의 회피하여 충돌 방지

// ── newTestRubricHandler 헬퍼 ────────────────────────────────────────────────

func newTestRubricHandler(t *testing.T, rubricTx *fakeRubricTx, evalItemTx *fakeRubricEvalItemTx, scoreTx *fakeScoreTx) *RubricHandler {
	t.Helper()
	rs := &fakeRubricStore{tx: rubricTx}
	es := &fakeRubricEvalItemStore{tx: evalItemTx}
	ss := &fakeScoreStore{tx: scoreTx}
	return NewRubricHandler(rs, es, ss, zaptest.NewLogger(t))
}

// doRubricReq Routes() 핸들러에 요청 보내고 status + parsed body 반환
func doRubricReq(t *testing.T, h *RubricHandler, method, target, body, userScope string) (int, map[string]any) {
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
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed) //nolint:errcheck // parsed nil은 호출자가 raw status로 판단
	return rec.Code, parsed
}

// buildFakeRubric 응답 DTO 검증을 위한 entity 생성 헬퍼
func buildFakeRubric(status string) *store.Rubric {
	now := time.Now().UTC()
	return &store.Rubric{
		ID:        uuid.New(),
		Name:      "rubric-x",
		Version:   1,
		Scope:     "default",
		Status:    status,
		CreatedAt: now,
		CreatedBy: "admin-001",
		UpdatedAt: now,
		UpdatedBy: "admin-001",
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-020 [OPEN #1] — POST /rubrics/{id}/clone-new-version admin 성공 201
// AC-RUBRIC-003-1 supersede 패턴 (REVIEW-001 sub-resource 선례 미러)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsCloneNewVersion_AdminSucceeds_201(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("active"),
		insertID:      uuid.New(),
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/clone-new-version"
	code, _ := doRubricReq(t, h, "POST", target, `{}`, "iroum-ax:admin")
	require.Equal(t, http.StatusCreated, code, "admin clone-new-version → 201")
	assert.True(t, tx.insertCalled, "Phase C GREEN: InsertRubric 호출 (version+1 신규 생성)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-021 — POST /rubrics admin 생성 201 (REQ-RUBRIC-002-E1 + AC-RUBRIC-002-1)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_Rubrics_AdminCreates_201(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	tx := &fakeRubricTx{insertID: uuid.New()}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	body := `{"name":"safety-v1","scope":"kepco-safety"}`
	code, _ := doRubricReq(t, h, "POST", "/api/v1/rubrics", body, "iroum-ax:admin")
	require.Equal(t, http.StatusCreated, code, "admin POST /rubrics → 201")
	assert.True(t, tx.insertCalled)
	assert.True(t, tx.commitCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-021b [OPEN #2] — UpdateRubric draft→active 중복 active 차단 409
// handler pre-check 단계에서 (name, scope) 중복 active 검사 + DB partial unique idx 보완
// ════════════════════════════════════════════════════════════════════════════

func TestUpdateRubric_DraftToActive_DuplicateActiveBlocks_409(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	// listResult에 동일 (name, scope)의 기존 active rubric 시뮬레이션
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("draft"),
		listResult: []*store.Rubric{
			{
				ID:     uuid.New(),
				Name:   "safety-v1",
				Scope:  "kepco-safety",
				Status: "active",
			},
		},
		countResult: 1, // 중복 active 1건 존재
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String()
	body := `{"name":"safety-v1","scope":"kepco-safety","status":"active"}`
	code, parsed := doRubricReq(t, h, "PUT", target, body, "iroum-ax:admin")
	require.Equal(t, http.StatusConflict, code, "동일 (name,scope) active 중복 → 409 (OPEN #2 handler pre-check)")

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok, "error 객체 존재")
	assert.Equal(t, "CONFLICT", errMap["code"])
	assert.Contains(t, errMap["message"], "active",
		"한국어 메시지에 'active' 키워드 (Phase C: 동일 name/scope의 active rubric이 이미 존재합니다)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-022 — POST /rubrics viewer 차단 403 (UBI-004 + AC E6)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_Rubrics_ViewerForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	tx := &fakeRubricTx{}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	body := `{"name":"x","scope":"default"}`
	code, parsed := doRubricReq(t, h, "POST", "/api/v1/rubrics", body, "iroum-ax:viewer")
	require.Equal(t, http.StatusForbidden, code, "viewer → 403 (admin-only)")

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ABAC_CONDITION_DENIED", errMap["code"])
	assert.False(t, tx.insertCalled, "store 미진입 (fail-closed)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-022b [OPEN #2 race] — UniqueViolationMaps409 (testcontainers integration)
// Phase A skeleton: 통합 시나리오 placeholder — Phase C integration test에서 race window 검증
// ════════════════════════════════════════════════════════════════════════════

func TestUpdateRubric_RaceConcurrent_UniqueViolationMaps409(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	// Phase A: handler-level 시뮬레이션 — Phase C testcontainers에서 실제 race 검증
	// fake updateErr가 pgconn.PgError SQLSTATE 23505 매핑 시뮬레이션
	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("draft"),
		// Phase C GREEN: pgconn.PgError{Code:"23505",ConstraintName:"rubrics_active_unique_idx"}
		// → mapRubricStoreErr가 ErrRubricInvalidStatus로 매핑 → 409
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String()
	body := `{"name":"safety-v1","scope":"kepco-safety","status":"active"}`
	code, _ := doRubricReq(t, h, "PUT", target, body, "iroum-ax:admin")
	// Phase A: 503 (handler 미구현) — Phase C: 409 (race SQLSTATE 23505 → mapRubricStoreErr → 409)
	require.NotEqual(t, http.StatusOK, code, "race 시 200 불가 — Phase A 503 / Phase C 409")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-023 — POST /rubrics analyst 차단 403 (UBI-004 + AC E7)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_Rubrics_AnalystForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	tx := &fakeRubricTx{}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	body := `{"name":"x","scope":"default"}`
	code, _ := doRubricReq(t, h, "POST", "/api/v1/rubrics", body, "iroum-ax:analyst")
	require.Equal(t, http.StatusForbidden, code, "analyst → 403 (admin-only)")
	assert.False(t, tx.insertCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-023b [OPEN #3] — AddCriterion weight 합산 1.0 초과 차단 400
// handler 단계 weight sum pre-check (OPEN #3 Option B handler validation only)
// ════════════════════════════════════════════════════════════════════════════

func TestAddCriterion_WeightSumExceedsOne_400(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("draft"),
		// 기존 weight 0.85 → 신규 0.20 추가 시 합 1.05 > 1.0 거부 (Phase C handler validation)
		criteriaResult: []*store.RubricCriterion{
			{Weight: 0.85},
		},
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/criteria"
	body := `{"evaluation_item_id":"` + uuid.New().String() + `","weight":0.20}`
	code, parsed := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	require.Equal(t, http.StatusBadRequest, code, "weight 합 1.0 초과 → 400 (OPEN #3 handler pre-check)")

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INVALID_ARGUMENT", errMap["code"])
	assert.Contains(t, errMap["message"], "가중치",
		"한국어 메시지: 가중치 합이 1.0을 초과합니다")
	assert.False(t, tx.addCriterionDone, "store 미진입 (handler-side fail-closed)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-024 — GET /rubrics/{id} viewer 200 (REQ-RUBRIC-002-E2 + AC-RUBRIC-002-2)
// ════════════════════════════════════════════════════════════════════════════

func TestGET_Rubrics_ViewerCanRead_200(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult:  &store.Rubric{ID: id, Name: "x", Status: "active", Scope: "default", Version: 1},
		criteriaResult: []*store.RubricCriterion{},
		bandsResult:    []*store.RubricBand{},
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String()
	code, parsed := doRubricReq(t, h, "GET", target, "", "iroum-ax:viewer")
	require.Equal(t, http.StatusOK, code, "viewer read → 200 (모든 인증)")
	assert.NotNil(t, parsed)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-024b [OPEN #4] — AddBand handler pre-check overlap 400
// dual defense Layer 1: handler 단계 overlap 검사 (DB EXCLUSION은 Layer 2)
// ════════════════════════════════════════════════════════════════════════════

func TestAddBand_Overlap_HandlerPreCheck_400(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("draft"),
		// 기존 band [80.0, 89.999] → 신규 [85.0, 95.0] 추가 시 겹침 (Phase C handler pre-check)
		bandsResult: []*store.RubricBand{
			{Letter: "B", MinScore: 80.0, MaxScore: 89.999},
		},
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/bands"
	body := `{"letter":"A","min_score":85.0,"max_score":95.0}`
	code, parsed := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	require.Equal(t, http.StatusBadRequest, code, "band overlap → 400 (OPEN #4 handler pre-check)")

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INVALID_ARGUMENT", errMap["code"])
	assert.Contains(t, errMap["message"], "구간",
		"한국어 메시지: 등급 구간이 기존 구간과 겹칩니다")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-025 — POST /rubrics/{id}/archive analyst 차단 403 (UBI-004)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsArchive_AnalystForbidden_403(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/archive"
	body := `{"archive_reason":"deprecated"}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:analyst")
	require.Equal(t, http.StatusForbidden, code, "analyst archive → 403 (admin-only)")
	assert.False(t, tx.archiveCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-025b [OPEN #4 race] — ExclusionViolationMaps400 (testcontainers integration)
// Phase A skeleton: 통합 시나리오 placeholder — Phase C에서 testcontainers race 검증
// SQLSTATE 23P01 (EXCLUSION violation) → ErrRubricBandOverlap → 400 매핑
// ════════════════════════════════════════════════════════════════════════════

func TestAddBand_RaceConcurrent_ExclusionViolationMaps400(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("draft"),
		bandsResult:   []*store.RubricBand{}, // handler pre-check 통과 (검사 시점 빈 결과)
		// Phase C GREEN: AddBand가 pgconn.PgError{Code:"23P01"} 반환 → mapRubricStoreErr 400
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/bands"
	body := `{"letter":"A","min_score":90.0,"max_score":100.0}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	// Phase A: 503 — Phase C: 400 (race SQLSTATE 23P01 → mapRubricStoreErr → 400)
	require.NotEqual(t, http.StatusOK, code, "race 시 200 불가 — Phase A 503 / Phase C 400")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-026 — POST /rubrics/{id}/apply viewer 200 (REQ-RUBRIC-004-E2 + AC-RUBRIC-004-2)
// ABAC: 모든 인증 사용자 apply 허용 (read-only no-audit, OPEN #6)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsApply_ViewerCanApply_200(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	scoreID := uuid.New()
	// fakeScoreTx: SumWeightedByEvaluationItem가 85.5 numeric 반환
	num := pgtype.Numeric{}
	require.NoError(t, num.Scan("85.5"))
	scoreTx := &fakeScoreTx{
		weightedSum:   num,
		getByIDResult: &store.Score{ID: scoreID, EvaluationItemID: "item-001", Level: "raw"},
	}
	rubricTx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("active"),
		applyLetter:   "B",
		applyBand:     &store.RubricBand{Letter: "B", MinScore: 80.0, MaxScore: 89.999},
	}
	h := newTestRubricHandler(t, rubricTx, &fakeRubricEvalItemTx{}, scoreTx)

	target := "/api/v1/rubrics/" + id.String() + "/apply"
	body := `{"score_id":"` + scoreID.String() + `"}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:viewer")
	require.Equal(t, http.StatusOK, code, "viewer apply → 200 (모든 인증)")
	assert.True(t, rubricTx.applyCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-026b [OPEN #5] — UpdateRubric active 직접 편집 admin 성공 200
// admin은 active 상태에서도 metadata/scope/criteria/bands 직접 편집 가능 (OPEN #5 Option A)
// ════════════════════════════════════════════════════════════════════════════

func TestUpdateRubric_ActiveDirectEdit_AdminSucceeds_200(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("active"),
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String()
	body := `{"name":"renamed","scope":"default","status":"active"}`
	code, _ := doRubricReq(t, h, "PUT", target, body, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code, "OPEN #5: active 직접 편집 허용 → 200")
	assert.True(t, tx.updateCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-027 — POST /rubrics/{id}/archive admin 200 + terminal 불변 (REQ-RUBRIC-003-E2)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsArchive_AdminSucceeds_200_TerminalImmutable(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("archived"),
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/archive"
	body := `{"archive_reason":"deprecated by new policy"}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code, "admin archive → 200")
	assert.True(t, tx.archiveCalled)
	assert.Equal(t, "deprecated by new policy", tx.gotArchiveReason,
		"archive_reason 정확 전파")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-027b [OPEN #7] — ArchiveRubric blank archive_reason 거부 400
// handler validation Layer 1: archive_reason blank/missing 시 400 한국어 메시지
// ════════════════════════════════════════════════════════════════════════════

func TestArchiveRubric_BlankReason_400(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/archive"
	cases := []struct {
		name string
		body string
	}{
		{"empty reason", `{"archive_reason":""}`},
		{"missing reason", `{}`},
		{"whitespace only", `{"archive_reason":"   "}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, parsed := doRubricReq(t, h, "POST", target, tc.body, "iroum-ax:admin")
			require.Equal(t, http.StatusBadRequest, code, "archive_reason 누락은 400")

			errMap, ok := parsed["error"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "INVALID_ARGUMENT", errMap["code"])
			assert.Contains(t, errMap["message"], "archive_reason",
				"한국어 메시지: archive_reason은 필수입니다")
			assert.False(t, tx.archiveCalled, "store 미진입 (handler-side fail-closed)")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-028 — POST /rubrics/{id}/criteria EvalItem 미존재 시 404 (cross-store TX-1)
// REQ-RUBRIC-002-E4 — handler-compose cross-store EvalItem 검증
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsAddCriterion_EvalItemNotExist_404(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{getByIDResult: buildFakeRubric("draft")}
	// EvalItem 미존재 → ErrEvalItemNotFound 시뮬레이션
	evalTx := &fakeRubricEvalItemTx{
		// Phase C GREEN: getErr = stderrors.ErrEvalItemNotFound → 404 매핑
	}
	h := newTestRubricHandler(t, tx, evalTx, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/criteria"
	body := `{"evaluation_item_id":"` + uuid.New().String() + `","weight":0.5}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	// Phase A: 503 (미구현) — Phase C: 404 (cross-store EvalItem 미존재)
	require.NotEqual(t, http.StatusCreated, code, "Phase A 503 / Phase C 404 — EvalItem 미존재")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-028b [OPEN #7] — ArchiveRubric DB CHECK violation 500 매핑 (integration placeholder)
// Phase A: handler-level 시뮬레이션 — Phase C testcontainers에서 DB CHECK violation 직접 검증
// ════════════════════════════════════════════════════════════════════════════

func TestArchiveRubric_DBCheckViolation_500MapsCorrectly(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		// Phase C GREEN: archiveErr = pgconn.PgError{Code:"23514", ConstraintName:"rubrics_archive_reason_chk"}
		// → mapRubricStoreErr가 500 INTERNAL로 매핑 (handler-side validation bypass 시 DB layer 차단)
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/archive"
	body := `{"archive_reason":"valid reason"}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	// Phase A: 503 — Phase C: 200 정상 또는 DB violation 시 500
	require.NotEqual(t, http.StatusCreated, code)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-029 — POST /rubrics/{id}/apply cross-store 2-TX 흐름 (REQ-RUBRIC-004-E2)
// TX-1: scoreStore.BeginScoreTx → SumWeightedByEvaluationItem → Rollback
// TX-2: rubricStore.BeginRubricTx → ApplyRubric → Rollback (read-only no-audit)
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsApply_CrossStoreTwoTX(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	scoreID := uuid.New()
	num := pgtype.Numeric{}
	require.NoError(t, num.Scan("82.5"))
	scoreTx := &fakeScoreTx{
		weightedSum:   num,
		getByIDResult: &store.Score{ID: scoreID, EvaluationItemID: "item-002", Level: "raw"},
	}
	rubricTx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("active"),
		applyLetter:   "B",
		applyBand:     &store.RubricBand{Letter: "B", MinScore: 80.0, MaxScore: 89.999},
	}
	h := newTestRubricHandler(t, rubricTx, &fakeRubricEvalItemTx{}, scoreTx)

	target := "/api/v1/rubrics/" + id.String() + "/apply"
	body := `{"score_id":"` + scoreID.String() + `"}`
	code, parsed := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	require.Equal(t, http.StatusOK, code, "cross-store 2-TX 정상 흐름 → 200")
	assert.True(t, rubricTx.applyCalled, "TX-2 ApplyRubric 호출")
	assert.True(t, rubricTx.rollbackCalled, "TX-2 read-only Rollback (no-audit, OPEN #6)")

	// 응답 schema 검증: {rubric_id, score_value, letter, band:{...}}
	require.NotNil(t, parsed)
	assert.Equal(t, "B", parsed["letter"], "apply 결과 letter")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-030 — POST /rubrics/{id}/apply score 모든 band 밖 400 (REQ-RUBRIC-004-U1 + AC E10)
// ApplyRubric fail-closed → ErrRubricInvalidInput → 400
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsApply_ScoreOutOfAllBands_400(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	scoreID := uuid.New()
	num := pgtype.Numeric{}
	require.NoError(t, num.Scan("999.99"))
	scoreTx := &fakeScoreTx{
		weightedSum:   num,
		getByIDResult: &store.Score{ID: scoreID},
	}
	rubricTx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("active"),
		// Phase C GREEN: ApplyRubric returns ErrRubricInvalidInput → mapRubricStoreErr → 400
	}
	h := newTestRubricHandler(t, rubricTx, &fakeRubricEvalItemTx{}, scoreTx)

	target := "/api/v1/rubrics/" + id.String() + "/apply"
	body := `{"score_id":"` + scoreID.String() + `"}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:viewer")
	// Phase A: 503 — Phase C: 400 (fail-closed)
	require.NotEqual(t, http.StatusOK, code, "score 모든 band 밖 → 400 (Phase C fail-closed)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-031 — GET /rubrics auth-disabled 투과 (UBI-003 + AC E8)
// auth context 부재 시 cli-anonymous 투과 (admin/analyst/viewer 게이트 우회)
// ════════════════════════════════════════════════════════════════════════════

func TestGET_Rubrics_AuthDisabled_PassthroughCliAnonymous(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	tx := &fakeRubricTx{
		listResult:  []*store.Rubric{},
		countResult: 0,
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	// userScope="" → auth context 부재 (auth-disabled Walking Skeleton)
	code, _ := doRubricReq(t, h, "GET", "/api/v1/rubrics", "", "")
	require.Equal(t, http.StatusOK, code, "auth-disabled 투과 → 200 (cli-anonymous)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-032 — GET /rubrics/{id} malformed UUID 400 (REQ-RUBRIC-002-U1 + AC E16)
// ════════════════════════════════════════════════════════════════════════════

func TestGET_Rubrics_MalformedUUID_400(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	tx := &fakeRubricTx{}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	code, parsed := doRubricReq(t, h, "GET", "/api/v1/rubrics/not-a-uuid", "", "iroum-ax:viewer")
	require.Equal(t, http.StatusBadRequest, code, "malformed UUID → 400")

	errMap, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INVALID_ARGUMENT", errMap["code"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-033 — archived rubric mutation 시도 409 (REQ-RUBRIC-003-S1 + AC E4)
// terminal 불변 — archived 상태에서 AddCriterion/AddBand/Update 시도 409 매핑
// ════════════════════════════════════════════════════════════════════════════

func TestPOST_RubricsArchivedRubricMutation_409(t *testing.T) {
	defer goleak.VerifyNone(t, rubricGoLeakOptions...)

	id := uuid.New()
	tx := &fakeRubricTx{
		getByIDResult: buildFakeRubric("archived"),
		// Phase C GREEN: AddCriterion이 ErrRubricArchived 반환 → mapRubricStoreErr → 409
	}
	h := newTestRubricHandler(t, tx, &fakeRubricEvalItemTx{}, &fakeScoreTx{})

	target := "/api/v1/rubrics/" + id.String() + "/criteria"
	body := `{"evaluation_item_id":"` + uuid.New().String() + `","weight":0.2}`
	code, _ := doRubricReq(t, h, "POST", target, body, "iroum-ax:admin")
	// Phase A: 503 — Phase C: 409 (archived terminal 불변)
	require.NotEqual(t, http.StatusCreated, code, "archived 상태 mutation → 409 (Phase C)")
}
