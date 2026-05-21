// score_handlers_test.go — 점수 조회/집계 HTTP API 핸들러 단위 테스트 (SPEC-AX-SCORE-API-001)
//
// 격리 전략: httptest + fake ScoreStore/ScoreTx (evidence_handlers_test.go의
// testcontainers 통합과 달리 본 SPEC은 핸들러 단위에 집중 — 통합은 SCORE-001 커버).
// integration 빌드 태그 미사용 — 기본 `go test ./apps/control-plane/cmd/server/`로 실행.
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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// withTestUser auth.WithUser로 viewer/analyst/admin principal을 context에 주입한다.
// scope 토큰(예: "iroum-ax:viewer")을 User.Scopes에 넣어 ParseRolesFromScope 경로를 탄다.
func withTestUser(ctx context.Context, scope string) context.Context {
	return auth.WithUser(ctx, &auth.User{UID: "test-user", Scopes: []string{scope}})
}

// scoreGoLeakOptions 병렬 형제 테스트 러너 + httptest 인프라 goroutine 제외.
// testing.tRunner: t.Parallel() 사용 시 defer goleak가 아직 살아있는 형제 테스트의
// 러너 goroutine을 포착하는 알려진 패턴 — 핸들러 누출이 아님(핸들러 누출은 다른
// top-function으로 표면화). 본 핸들러의 TX/goroutine 실누출은 이 옵션 후에도
// 엄격 탐지됨 (T-012 fault-inject 의도 충족).
var scoreGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("testing.tRunner.func1"),
	goleak.IgnoreTopFunction("testing.tRunner"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
}

// ── fake ScoreStore / ScoreTx (인터페이스 격리, store 미진입 단언 가능) ──────────

// fakeScoreTx store.ScoreTx 인메모리 fake — 각 메서드 반환을 주입하고 호출을 기록한다.
// 필드 순서: slice(24B) → 포인터-보유(error/*Score/Numeric 16B) → 값-집계(uuid 16B) →
// string(16B) → bool(1B) — govet fieldalignment 최적화 (포인터 바이트 최소화)
type fakeScoreTx struct {
	gradeErr        error
	listErr         error
	getByIDErr      error
	commitErr       error
	insertErr       error
	updateErr       error
	supersedeErr    error
	rollupErr       error
	getByIDResult   *store.Score
	weightedSum     pgtype.Numeric
	grade           string
	scoresByItem    []*store.Score
	insertResult    uuid.UUID
	supersedeID     uuid.UUID
	insertCalled    bool
	updateCalled    bool
	supersedeCalled bool
	commitCalled    bool
	rollbackCalled  bool
}

func (f *fakeScoreTx) InsertScore(_ context.Context, _ string, _ *uuid.UUID, _ string, _ float64, _ *float64, _ map[string]any) (uuid.UUID, error) {
	f.insertCalled = true
	if f.insertErr != nil {
		return uuid.Nil, f.insertErr
	}
	return f.insertResult, nil
}

func (f *fakeScoreTx) GetScoreByID(_ context.Context, _ uuid.UUID) (*store.Score, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	return f.getByIDResult, nil
}

func (f *fakeScoreTx) GetScoresByEvaluationItem(_ context.Context, _ string) ([]*store.Score, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.scoresByItem, nil
}

func (f *fakeScoreTx) UpdateScore(_ context.Context, _ uuid.UUID, _ store.ScoreUpdate) error {
	f.updateCalled = true
	return f.updateErr
}

func (f *fakeScoreTx) SupersedeAndReplaceScore(_ context.Context, _ uuid.UUID, _ float64, _ *float64, _ map[string]any) (uuid.UUID, error) {
	f.supersedeCalled = true
	if f.supersedeErr != nil {
		return uuid.Nil, f.supersedeErr
	}
	return f.supersedeID, nil
}

func (f *fakeScoreTx) SumWeightedByEvaluationItem(_ context.Context, _ string) (pgtype.Numeric, error) {
	if f.rollupErr != nil {
		return pgtype.Numeric{}, f.rollupErr
	}
	return f.weightedSum, nil
}

func (f *fakeScoreTx) DetermineGrade(_ context.Context, _ string, _ float64) (string, error) {
	if f.gradeErr != nil {
		return "", f.gradeErr
	}
	return f.grade, nil
}

func (f *fakeScoreTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }

func (f *fakeScoreTx) Commit(_ context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakeScoreTx) Rollback(_ context.Context) error {
	f.rollbackCalled = true
	return nil
}

// fakeScoreStore store.ScoreStore fake — BeginScoreTx로 주입된 tx를 반환한다.
type fakeScoreStore struct {
	tx          *fakeScoreTx
	beginErr    error
	beginCalled bool
}

func (f *fakeScoreStore) BeginScoreTx(_ context.Context) (store.ScoreTx, error) {
	f.beginCalled = true
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// newTestScoreHandler fake store로 ScoreHandler 구성 (auth 비활성 기본 — 핸들러 단위)
func newTestScoreHandler(t *testing.T, tx *fakeScoreTx) (*ScoreHandler, *fakeScoreStore) {
	t.Helper()
	st := &fakeScoreStore{tx: tx}
	h := NewScoreHandler(st, zaptest.NewLogger(t))
	return h, st
}

// doScoreReq Routes() 핸들러에 요청을 보내고 status + raw body 반환
func doScoreReq(t *testing.T, h *ScoreHandler, method, target string, body string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	var parsed map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed) //nolint:errcheck
	return rec.Code, parsed
}

// ════════════════════════════════════════════════════════════════════════════
// T-002 [S1] ScoreHandler struct + Routes() 7 패턴 + ServeMux 최장일치 우선순위
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_RoutesRegistersSevenPatterns Routes()가 7개 라우트를 등록하고
// ServeMux 최장일치로 /rollup·/grade·/{id}/supersede가 /{id}보다 우선함을 검증한다.
// (AC-SCORE-API-001-1, 002-3, §7 edge #15 ServeMux 충돌)
func TestScoreHandler_RoutesRegistersSevenPatterns(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	validID := uuid.New()
	tx := &fakeScoreTx{
		getByIDResult: &store.Score{ID: validID, EvaluationItemID: "EI-1", Level: "raw", Status: "DRAFT"},
		scoresByItem:  []*store.Score{},
		grade:         "A",
	}
	num := pgtype.Numeric{}
	require.NoError(t, num.Scan("42.5000"))
	tx.weightedSum = num
	h, _ := newTestScoreHandler(t, tx)

	cases := []struct {
		name       string
		method     string
		target     string
		wantStatus int
	}{
		{"GET single /{id}", http.MethodGet, "/api/v1/scores/" + validID.String(), http.StatusOK},
		{"GET list", http.MethodGet, "/api/v1/scores?evaluation_item_id=EI-1", http.StatusOK},
		{"GET rollup (longest-match over /{id})", http.MethodGet, "/api/v1/scores/rollup?evaluation_item_id=EI-1", http.StatusOK},
		{"GET grade (longest-match over /{id})", http.MethodGet, "/api/v1/scores/grade?scope=default&score=85.50", http.StatusOK},
		{"POST create", http.MethodPost, "/api/v1/scores", http.StatusCreated},
		{"PUT update", http.MethodPut, "/api/v1/scores/" + validID.String(), http.StatusOK},
		{"POST supersede (longest-match over /{id})", http.MethodPost, "/api/v1/scores/" + validID.String() + "/supersede", http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body string
			if tc.method == http.MethodPost || tc.method == http.MethodPut {
				body = `{"evaluation_item_id":"EI-1","level":"raw","score_value":80.0}`
			}
			// supersede는 CONFIRMED 행 전제 → fake가 성공 반환하도록 supersedeID 주입
			tx.supersedeID = uuid.New()
			tx.insertResult = uuid.New()
			status, _ := doScoreReq(t, h, tc.method, tc.target, body)
			assert.Equal(t, tc.wantStatus, status, "라우트 %s %s 가 최장일치로 정확히 디스패치되어야 함", tc.method, tc.target)
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-003 [S1] server.go 마운트 합성: /api/v1/scores 정확 + /api/v1/scores/ 서브트리
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_ServerMountComposition server.go innerMux 합성과 동형으로
// 외부 mux에 `/api/v1/scores`(목록/생성) + `/api/v1/scores/`(단건/rollup/grade/supersede)
// 두 패턴을 마운트했을 때 양쪽이 정확히 ScoreHandler로 디스패치됨을 검증한다.
// (AC-SCORE-API-BOUNDARY-1 마운트 한정 — server.go 라우트 2줄, ABAC 와이어링 무변경)
func TestScoreHandler_ServerMountComposition(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	id := uuid.New()
	tx := &fakeScoreTx{
		getByIDResult: &store.Score{ID: id, EvaluationItemID: "EI-1", Level: "raw", Status: "DRAFT"},
		scoresByItem:  []*store.Score{},
	}
	h, _ := newTestScoreHandler(t, tx)

	// server.go:257-259 합성 동형 (innerMux.Handle 2 패턴)
	innerMux := http.NewServeMux()
	innerMux.Handle("/api/v1/scores", h.Routes())
	innerMux.Handle("/api/v1/scores/", h.Routes())

	for _, tc := range []struct {
		name, target string
		want         int
	}{
		{"목록(정확 경로)", "/api/v1/scores?evaluation_item_id=EI-1", http.StatusOK},
		{"단건(서브트리)", "/api/v1/scores/" + id.String(), http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			rec := httptest.NewRecorder()
			innerMux.ServeHTTP(rec, req)
			assert.Equal(t, tc.want, rec.Code, "server.go 마운트 합성에서 %s 디스패치", tc.target)
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-004 [S2] handleGetScore: 200 / 404 / 400
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_GetScore_200 존재하는 점수 → GetScoreByID → 200 + 전 필드 JSON
// (AC-SCORE-API-001-1)
func TestScoreHandler_GetScore_200(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	id := uuid.New()
	sv := 87.5
	tx := &fakeScoreTx{getByIDResult: &store.Score{
		ID: id, EvaluationItemID: "EI-9", Level: "raw", Status: "DRAFT",
		ScoreValue: &sv, Grade: "B", CreatedBy: "cli-anonymous",
	}}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/"+id.String(), "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, id.String(), body["id"])
	assert.Equal(t, "EI-9", body["evaluation_item_id"])
	assert.Equal(t, "DRAFT", body["status"])
}

// TestScoreHandler_GetScore_404 ErrScoreNotFound → 404 표준 에러 본문 (200/500 금지)
// (AC-SCORE-API-001-2, §7 edge #2)
func TestScoreHandler_GetScore_404(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{getByIDErr: apperrors.ErrScoreNotFound}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/"+uuid.New().String(), "")
	assert.Equal(t, http.StatusNotFound, status)
	assert.NotEqual(t, http.StatusOK, status)
	assert.NotEqual(t, http.StatusInternalServerError, status)
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok, "표준 에러 본문 {\"error\":{...}} 필요")
	assert.NotEmpty(t, errObj["code"])
	assert.NotEmpty(t, errObj["message"])
}

// TestScoreHandler_GetScore_400_MalformedUUID 비-UUID path → 400, store 미진입
// (AC-SCORE-API-002-5, §7 edge #6)
func TestScoreHandler_GetScore_400_MalformedUUID(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	h, st := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/not-a-uuid", "")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.False(t, st.beginCalled, "malformed UUID는 store TX 미진입")
	errObj, _ := body["error"].(map[string]any)
	assert.NotEmpty(t, errObj["code"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-005 [S2] handleListScores: filter + slicing + empty + blank evalitem
// ════════════════════════════════════════════════════════════════════════════

func mkScore(item, level, status string) *store.Score {
	return &store.Score{ID: uuid.New(), EvaluationItemID: item, Level: level, Status: status}
}

// TestScoreHandler_ListScores_FilterAndSlice level/status 필터 + offset/limit 슬라이싱
// (AC-SCORE-API-001-3, §6 #3)
func TestScoreHandler_ListScores_FilterAndSlice(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{scoresByItem: []*store.Score{
		mkScore("EI-1", "raw", "DRAFT"),
		mkScore("EI-1", "raw", "DRAFT"),
		mkScore("EI-1", "item", "CONFIRMED"), // level 필터로 제외
		mkScore("EI-1", "raw", "CONFIRMED"),  // status 필터로 제외
	}}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet,
		"/api/v1/scores?evaluation_item_id=EI-1&level=raw&status=DRAFT&offset=0&limit=10", "")
	assert.Equal(t, http.StatusOK, status)
	arr, ok := body["scores"].([]any)
	require.True(t, ok)
	assert.Len(t, arr, 2, "level=raw&status=DRAFT 필터 적용 후 2건")
	assert.Equal(t, float64(2), body["count"])
}

// TestScoreHandler_ListScores_Empty 결과 0건 → {"scores":[],"count":0} (NULL/누락 금지)
// (AC-SCORE-API-001-6, §7 edge #12)
func TestScoreHandler_ListScores_Empty(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{scoresByItem: []*store.Score{}}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores?evaluation_item_id=none", "")
	assert.Equal(t, http.StatusOK, status)
	arr, ok := body["scores"].([]any)
	require.True(t, ok, "scores는 빈 배열이어야 함 (NULL/필드 누락 금지)")
	assert.Empty(t, arr)
	assert.Equal(t, float64(0), body["count"])
}

// TestScoreHandler_ListScores_BlankEvalItem evaluation_item_id 공백 → 400
// (AC-SCORE-API-001-3 / REQ-001-E2)
func TestScoreHandler_ListScores_BlankEvalItem(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores?evaluation_item_id=", "")
	assert.Equal(t, http.StatusBadRequest, status)
	errObj, _ := body["error"].(map[string]any)
	assert.Equal(t, "evaluation_item_id", errObj["field"])
}

// ════════════════════════════════════════════════════════════════════════════
// T-006 [S2] clampPagination: 누락/0→50, >500→500, offset 음수→0
// ════════════════════════════════════════════════════════════════════════════

// TestClampPagination 3분기 결정적 clamp (§6 #2: max=500 def=50)
func TestClampPagination(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		rawLimit   string
		rawOffset  string
		wantLimit  int
		wantOffset int
	}{
		{"누락 → default 50", "", "", 50, 0},
		{"limit 0 → default 50", "0", "0", 50, 0},
		{"limit > max 500 → clamp 500", "99999", "10", 500, 10},
		{"offset 음수 → 0", "20", "-5", 20, 0},
		{"offset 비수치 → 0", "20", "abc", 20, 0},
		{"정상 통과", "100", "30", 100, 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotLimit, gotOffset := clampPagination(tc.rawLimit, tc.rawOffset)
			assert.Equal(t, tc.wantLimit, gotLimit)
			assert.Equal(t, tc.wantOffset, gotOffset)
		})
	}
}

// TestScoreHandler_ListScores_PaginationClamp limit>max + offset 음수 → 결정적 결과
// (AC-SCORE-API-001-7, §7 edge #8/#9/#10)
func TestScoreHandler_ListScores_PaginationClamp(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	items := make([]*store.Score, 3)
	for i := range items {
		items[i] = mkScore("EI-1", "raw", "DRAFT")
	}
	tx := &fakeScoreTx{scoresByItem: items}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet,
		"/api/v1/scores?evaluation_item_id=EI-1&limit=99999&offset=-5", "")
	assert.Equal(t, http.StatusOK, status)
	arr, _ := body["scores"].([]any)
	assert.Len(t, arr, 3, "offset=0으로 보정, limit=500 clamp → 전체 3건 결정적 반환")
}

// ════════════════════════════════════════════════════════════════════════════
// T-007 [S2] handleRollup (numeric round-trip) + handleGrade (200 / 404 / 400)
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_Rollup_NumericPrecision pgtype.Numeric → float64 미경유 정확 십진 문자열
// (AC-SCORE-API-001-4, SEC-03)
func TestScoreHandler_Rollup_NumericPrecision(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	var num pgtype.Numeric
	require.NoError(t, num.Scan("1234.5678"))
	tx := &fakeScoreTx{weightedSum: num}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/rollup?evaluation_item_id=EI-1", "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "EI-1", body["evaluation_item_id"])
	// float64 직렬화면 1234.5678 정밀도 손실 가능 — 문자열로 정확 십진 보존되어야 함
	assert.Equal(t, "1234.5678", body["weighted_sum"])
}

// TestScoreHandler_Grade_200 DetermineGrade → 200
// (AC-SCORE-API-001-5)
func TestScoreHandler_Grade_200(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{grade: "A"}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/grade?scope=default&score=85.50", "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "default", body["scope"])
	assert.Equal(t, "A", body["grade"])
	assert.Equal(t, 85.50, body["score"])
}

// TestScoreHandler_Grade_404_ThresholdsUnavailable ErrGradeThresholdsUnavailable → 404
// (AC-SCORE-API-004-1 설계결정, §7 edge #13)
func TestScoreHandler_Grade_404_ThresholdsUnavailable(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{gradeErr: apperrors.ErrGradeThresholdsUnavailable}
	h, _ := newTestScoreHandler(t, tx)

	status, _ := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/grade?scope=unknown&score=50", "")
	assert.Equal(t, http.StatusNotFound, status)
}

// TestScoreHandler_Grade_400 scope 공백/score 비수치 → 400
// (AC-SCORE-API-001-U1)
func TestScoreHandler_Grade_400(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	h, _ := newTestScoreHandler(t, tx)

	st1, _ := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/grade?scope=&score=50", "")
	assert.Equal(t, http.StatusBadRequest, st1, "scope 공백 → 400")

	st2, _ := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/grade?scope=default&score=abc", "")
	assert.Equal(t, http.StatusBadRequest, st2, "score 비수치 → 400")
}

// ════════════════════════════════════════════════════════════════════════════
// T-008 [S3] handleCreateScore: 201 + pre-TX 검증 400 4종
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_CreateScore_201 유효 body → BeginScoreTx→InsertScore→Commit→201
// (AC-SCORE-API-002-1)
func TestScoreHandler_CreateScore_201(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	newID := uuid.New()
	tx := &fakeScoreTx{insertResult: newID}
	h, st := newTestScoreHandler(t, tx)

	body := `{"evaluation_item_id":"EI-1","level":"raw","score_value":88.0}`
	status, resp := doScoreReq(t, h, http.MethodPost, "/api/v1/scores", body)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, newID.String(), resp["score_id"])
	assert.Equal(t, "DRAFT", resp["status"])
	assert.True(t, st.beginCalled)
	assert.True(t, tx.insertCalled)
	assert.True(t, tx.commitCalled)
}

// TestScoreHandler_CreateScore_400_Validations 4종 입력검증 → 400, TX 미진입
// (AC-SCORE-API-002-4, §7 edge #5/#7)
func TestScoreHandler_CreateScore_400_Validations(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	long := strings.Repeat("x", 65)
	cases := []struct {
		name  string
		body  string
		field string
	}{
		{"evaluation_item_id 공백", `{"evaluation_item_id":"","level":"raw","score_value":80}`, "evaluation_item_id"},
		{"evaluation_item_id >64", `{"evaluation_item_id":"` + long + `","level":"raw","score_value":80}`, "evaluation_item_id"},
		{"score_value 누락", `{"evaluation_item_id":"EI-1","level":"raw"}`, "score_value"},
		{"evidence_id 비-UUID", `{"evaluation_item_id":"EI-1","level":"raw","score_value":80,"evidence_id":"nope"}`, "evidence_id"},
		{"malformed JSON", `{"evaluation_item_id":`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &fakeScoreTx{}
			h, st := newTestScoreHandler(t, tx)
			status, body := doScoreReq(t, h, http.MethodPost, "/api/v1/scores", tc.body)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.False(t, st.beginCalled, "pre-TX 검증 실패는 ScoreTx 미진입")
			assert.False(t, tx.insertCalled)
			if tc.field != "" {
				errObj, _ := body["error"].(map[string]any)
				assert.Equal(t, tc.field, errObj["field"])
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-009 [S3] handleUpdateScore: 200 / 409 immutable / 400 malformed
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_UpdateScore_200 DRAFT 행 → UpdateScore→Commit→200
// (AC-SCORE-API-002-2)
func TestScoreHandler_UpdateScore_200(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	id := uuid.New()
	tx := &fakeScoreTx{}
	h, _ := newTestScoreHandler(t, tx)

	status, resp := doScoreReq(t, h, http.MethodPut, "/api/v1/scores/"+id.String(), `{"score_value":90.0}`)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, id.String(), resp["score_id"])
	assert.True(t, tx.updateCalled)
	assert.True(t, tx.commitCalled)
}

// TestScoreHandler_UpdateScore_409_Immutable CONFIRMED 행 → ErrScoreImmutable → 409 (한국어)
// (AC-SCORE-API-UBI-004-2, §7 edge #3)
func TestScoreHandler_UpdateScore_409_Immutable(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{updateErr: apperrors.ErrScoreImmutable}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodPut, "/api/v1/scores/"+uuid.New().String(), `{"score_value":90.0}`)
	assert.Equal(t, http.StatusConflict, status)
	assert.NotEqual(t, http.StatusOK, status)
	assert.NotEqual(t, http.StatusInternalServerError, status)
	errObj, _ := body["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	assert.Contains(t, msg, "CONFIRMED", "한국어 표준 에러 본문")
}

// TestScoreHandler_UpdateScore_400_Malformed malformed UUID/JSON → 400
// (AC-SCORE-API-002-5)
func TestScoreHandler_UpdateScore_400_Malformed(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	h, st := newTestScoreHandler(t, tx)

	st1, _ := doScoreReq(t, h, http.MethodPut, "/api/v1/scores/not-a-uuid", `{"score_value":90}`)
	assert.Equal(t, http.StatusBadRequest, st1)
	assert.False(t, st.beginCalled)

	st2, _ := doScoreReq(t, h, http.MethodPut, "/api/v1/scores/"+uuid.New().String(), `{bad json`)
	assert.Equal(t, http.StatusBadRequest, st2)
}

// ════════════════════════════════════════════════════════════════════════════
// T-010 [S3] handleSupersedeScore: 201 / 409×2 (DRAFT edge#14, SUPERSEDED edge#4)
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_Supersede_201 CONFIRMED → SupersedeAndReplaceScore → 201{new,old}
// (AC-SCORE-API-002-3, §6 #1)
func TestScoreHandler_Supersede_201(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	oldID := uuid.New()
	newID := uuid.New()
	tx := &fakeScoreTx{supersedeID: newID}
	h, _ := newTestScoreHandler(t, tx)

	status, resp := doScoreReq(t, h, http.MethodPost, "/api/v1/scores/"+oldID.String()+"/supersede", `{"score_value":88.0}`)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, newID.String(), resp["score_id"])
	assert.Equal(t, oldID.String(), resp["superseded_id"])
	assert.True(t, tx.supersedeCalled)
	assert.True(t, tx.commitCalled)
}

// TestScoreHandler_Supersede_409 non-CONFIRMED (DRAFT/SUPERSEDED 두 진입) → ErrScoreNotConfirmed → 409
// (AC-SCORE-API-UBI-004-3, §7 edge #14 DRAFT / #4 SUPERSEDED)
func TestScoreHandler_Supersede_409(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	// 분기 A(edge#14 DRAFT) + 분기 B(edge#4 SUPERSEDED) 모두 store가 동일 센티넬 반환
	for _, branch := range []string{"DRAFT-entry", "SUPERSEDED-entry"} {
		t.Run(branch, func(t *testing.T) {
			tx := &fakeScoreTx{supersedeErr: apperrors.ErrScoreNotConfirmed}
			h, _ := newTestScoreHandler(t, tx)
			status, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores/"+uuid.New().String()+"/supersede", `{"score_value":88.0}`)
			assert.Equal(t, http.StatusConflict, status)
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-011 [S4] mapStoreErr: 7 센티넬 + unknown 표
// ════════════════════════════════════════════════════════════════════════════

// TestMapStoreErr 센티넬 → HTTP status 결정적 매핑 (AC-SCORE-API-004-1)
func TestMapStoreErr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		name string
		want int
	}{
		{apperrors.ErrScoreNotFound, "ErrScoreNotFound → 404", http.StatusNotFound},
		{apperrors.ErrScoreInvalidInput, "ErrScoreInvalidInput → 400", http.StatusBadRequest},
		{apperrors.ErrScoreImmutable, "ErrScoreImmutable → 409", http.StatusConflict},
		{apperrors.ErrScoreInvalidStatus, "ErrScoreInvalidStatus → 409", http.StatusConflict},
		{apperrors.ErrScoreNotConfirmed, "ErrScoreNotConfirmed → 409", http.StatusConflict},
		{apperrors.ErrGradeThresholdsUnavailable, "ErrGradeThresholdsUnavailable → 404", http.StatusNotFound},
		{errors.New("connection refused"), "unknown DB error → 500", http.StatusInternalServerError},
		{errfmt(apperrors.ErrScoreImmutable), "wrapped ErrScoreImmutable → 409", http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := mapStoreErr(tc.err)
			assert.Equal(t, tc.want, code)
		})
	}
}

func errfmt(e error) error { return errWrap{e} }

type errWrap struct{ inner error }

func (w errWrap) Error() string { return "wrapped: " + w.inner.Error() }
func (w errWrap) Unwrap() error { return w.inner }

// ════════════════════════════════════════════════════════════════════════════
// T-012 [S4] TX rollback 부분커밋 0 + goroutine 0
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_TXRollback_OnDownstreamFailure InsertScore 후 Commit 실패 →
// committed-defer Rollback, 부분커밋 0, goroutine 0 (AC-SCORE-API-004-2)
func TestScoreHandler_TXRollback_OnDownstreamFailure(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{insertResult: uuid.New(), commitErr: errors.New("commit failed")}
	h, _ := newTestScoreHandler(t, tx)

	status, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores",
		`{"evaluation_item_id":"EI-1","level":"raw","score_value":80}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.True(t, tx.rollbackCalled, "Commit 실패 시 committed-defer가 Rollback 실행")
	assert.True(t, tx.insertCalled)
}

// TestScoreHandler_TXRollback_BeginFailure BeginScoreTx 실패 → 500, TX 미진입
func TestScoreHandler_TXRollback_BeginFailure(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	st := &fakeScoreStore{tx: tx, beginErr: errors.New("pool exhausted")}
	h := NewScoreHandler(st, zaptest.NewLogger(t))

	status, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores",
		`{"evaluation_item_id":"EI-1","level":"raw","score_value":80}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.False(t, tx.insertCalled)
	assert.False(t, tx.rollbackCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-013 [S5] requireScoreWriteRole + ABAC write-role 게이팅
// ════════════════════════════════════════════════════════════════════════════

// TestRequireScoreWriteRole {RoleAdmin,RoleAnalyst}만 true (OBS-001 IsMetricsAuthorized 동형)
// (AC-SCORE-API-003-1, §6 #4)
func TestRequireScoreWriteRole(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"admin → write 허용", []string{"iroum-ax:admin"}, true},
		{"analyst → write 허용", []string{"iroum-ax:analyst"}, true},
		{"viewer → write 거부", []string{"iroum-ax:viewer"}, false},
		{"역할 없음 → 거부", []string{}, false},
		{"admin+viewer 혼합 → 허용", []string{"iroum-ax:viewer", "iroum-ax:admin"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, requireScoreWriteRole(strings.Join(tc.roles, " ")))
		})
	}
}

// TestScoreHandler_ViewerWriteDeny_403 viewer-only mutation → 403 ABAC_CONDITION_DENIED, store 미진입
// (AC-SCORE-API-003-1, UBI-004-1, §7 edge #1)
func TestScoreHandler_ViewerWriteDeny_403(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	st := &fakeScoreStore{tx: tx}
	h := NewScoreHandler(st, zaptest.NewLogger(t))

	for _, route := range []struct {
		method, target, body string
	}{
		{http.MethodPost, "/api/v1/scores", `{"evaluation_item_id":"EI-1","level":"raw","score_value":80}`},
		{http.MethodPut, "/api/v1/scores/" + uuid.New().String(), `{"score_value":90}`},
		{http.MethodPost, "/api/v1/scores/" + uuid.New().String() + "/supersede", `{"score_value":88}`},
	} {
		req := httptest.NewRequest(route.method, route.target, strings.NewReader(route.body))
		req.Header.Set("Content-Type", "application/json")
		// authEnabled=true 환경에서 viewer-only principal 주입
		req = req.WithContext(withTestUser(req.Context(), "iroum-ax:viewer"))
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s viewer write → 403", route.method, route.target)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body) //nolint:errcheck
		errObj, _ := body["error"].(map[string]any)
		assert.Equal(t, "ABAC_CONDITION_DENIED", errObj["code"])
	}
	assert.False(t, st.beginCalled, "viewer write deny는 store TX 미진입")
}

// TestScoreHandler_ViewerRead_200 동일 viewer principal의 GET은 허용
// (AC-SCORE-API-003-1 read 무게이트)
func TestScoreHandler_ViewerRead_200(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	id := uuid.New()
	tx := &fakeScoreTx{getByIDResult: &store.Score{ID: id, EvaluationItemID: "EI-1", Level: "raw", Status: "DRAFT"}}
	h, _ := newTestScoreHandler(t, tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scores/"+id.String(), nil)
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:viewer"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "viewer GET은 무게이트 허용")
}

// TestScoreHandler_AuthDisabled_Passthrough auth context 부재(ok=false) → 투과
// (AC-SCORE-API-003-2/UBI-003-1, §7 edge #11)
func TestScoreHandler_AuthDisabled_Passthrough(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{insertResult: uuid.New()}
	h, st := newTestScoreHandler(t, tx)

	// context에 user 없음 → ABAC narrowing 동형 투과 (auth-disabled Walking Skeleton)
	status, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores",
		`{"evaluation_item_id":"EI-1","level":"raw","score_value":80}`)
	assert.Equal(t, http.StatusCreated, status, "auth context 부재 → 투과(cli-anonymous store 위임)")
	assert.True(t, st.beginCalled)
}

// TestScoreHandler_AdminBypass_200 admin → 전 엔드포인트(쓰기) 통과
// (AC-SCORE-API-003-3)
func TestScoreHandler_AdminBypass_200(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{insertResult: uuid.New()}
	h, _ := newTestScoreHandler(t, tx)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scores",
		strings.NewReader(`{"evaluation_item_id":"EI-1","level":"raw","score_value":80}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withTestUser(req.Context(), "iroum-ax:admin"))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code, "admin은 전 쓰기 엔드포인트 통과")
}

// ════════════════════════════════════════════════════════════════════════════
// T-014 [S5] 횡단 UBI: 외부의존 0 / 자체 audit 0 / cli-anonymous 비위조
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_UBI_NoSelfAudit_NoForge GET은 audit 미발생, 핸들러는 user 비위조
// (AC-SCORE-API-UBI-002-2, UBI-003-2) — 정적 import 검증은 T-014 progress.md grep으로 보강
func TestScoreHandler_UBI_NoSelfAudit_NoForge(t *testing.T) {
	// t.Parallel() 미사용: defer goleak.VerifyNone와 t.Parallel() 병행은
	// 형제 러너 goroutine 포착으로 false-positive 발생 (evidence_handlers_test.go 선례 동형).
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	id := uuid.New()
	tx := &fakeScoreTx{getByIDResult: &store.Score{ID: id, EvaluationItemID: "EI-1", Level: "raw", Status: "DRAFT"}}
	h, st := newTestScoreHandler(t, tx)

	status, _ := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/"+id.String(), "")
	assert.Equal(t, http.StatusOK, status)
	// GET 경로: store 인터페이스상 읽기도 ScoreTx 경유(GetScoreByID는 ScoreTx 메서드, store.go:276)
	// — 핵심 불변(UBI-002-2)은 "API 자체 audit/mutation 0건": 읽기 TX는 commit 없이 rollback,
	// InsertScore/UpdateScore/Supersede 등 audit-유발 메서드 미호출.
	assert.True(t, st.beginCalled, "읽기도 ScoreTx 경유 (store.go:276 GetScoreByID는 ScoreTx 메서드)")
	assert.False(t, tx.insertCalled, "GET은 mutation/audit 유발 메서드 미호출")
	assert.False(t, tx.updateCalled)
	assert.False(t, tx.supersedeCalled)
	assert.False(t, tx.commitCalled, "읽기 TX는 commit 없이 rollback (audit 0건)")
	assert.True(t, tx.rollbackCalled, "읽기 TX는 항상 rollback")
}

// ════════════════════════════════════════════════════════════════════════════
// T-012 확장 [S4] 결함 주입: 읽기 핸들러 BeginScoreTx 실패 + downstream 실패
//   (방어 분기 결정적 매핑 — AC-SCORE-API-004-1/004-2, ≥85% 커버리지)
// ════════════════════════════════════════════════════════════════════════════

// TestScoreHandler_ReadHandlers_BeginTxFailure 4개 읽기 핸들러 BeginScoreTx 실패 → 500
// (AC-SCORE-API-004-1 unknown→500, 방어 분기)
func TestScoreHandler_ReadHandlers_BeginTxFailure(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	for _, tc := range []struct{ name, target string }{
		{"GET single", "/api/v1/scores/" + uuid.New().String()},
		{"GET list", "/api/v1/scores?evaluation_item_id=EI-1"},
		{"GET rollup", "/api/v1/scores/rollup?evaluation_item_id=EI-1"},
		{"GET grade", "/api/v1/scores/grade?scope=default&score=50"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &fakeScoreStore{tx: &fakeScoreTx{}, beginErr: errors.New("pool exhausted")}
			h := NewScoreHandler(st, zaptest.NewLogger(t))
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
		})
	}
}

// TestScoreHandler_ReadHandlers_StoreError 읽기 store 메서드 에러 → 매핑된 status
// (AC-SCORE-API-004-1: list/rollup unknown→500)
func TestScoreHandler_ReadHandlers_StoreError(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	t.Run("list store error → 500", func(t *testing.T) {
		tx := &fakeScoreTx{listErr: errors.New("db down")}
		h, _ := newTestScoreHandler(t, tx)
		status, _ := doScoreReq(t, h, http.MethodGet, "/api/v1/scores?evaluation_item_id=EI-1", "")
		assert.Equal(t, http.StatusInternalServerError, status)
	})
	t.Run("rollup store error → 500", func(t *testing.T) {
		tx := &fakeScoreTx{rollupErr: errors.New("agg failed")}
		h, _ := newTestScoreHandler(t, tx)
		status, _ := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/rollup?evaluation_item_id=EI-1", "")
		assert.Equal(t, http.StatusInternalServerError, status)
	})
	t.Run("rollup blank evalitem → 400", func(t *testing.T) {
		tx := &fakeScoreTx{}
		h, _ := newTestScoreHandler(t, tx)
		status, _ := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/rollup?evaluation_item_id=", "")
		assert.Equal(t, http.StatusBadRequest, status)
	})
}

// TestScoreHandler_GetScore_EvidenceIDSerialized evidence_id non-nil 행 → 응답에 직렬화
// (AC-SCORE-API-001-1 전 필드 — toScoreResponse evidence 분기)
func TestScoreHandler_GetScore_EvidenceIDSerialized(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	id := uuid.New()
	evID := uuid.New()
	tx := &fakeScoreTx{getByIDResult: &store.Score{
		ID: id, EvaluationItemID: "EI-1", Level: "raw", Status: "DRAFT", EvidenceID: &evID,
	}}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/"+id.String(), "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, evID.String(), body["evidence_id"], "evidence_id non-nil 행은 직렬화되어야 함")
}

// TestScoreHandler_Update_CommitFailure_Rollback UpdateScore 후 Commit 실패 → 500 + Rollback
// (AC-SCORE-API-004-2 부분커밋 0)
func TestScoreHandler_Update_CommitFailure_Rollback(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{commitErr: errors.New("commit failed")}
	h, _ := newTestScoreHandler(t, tx)

	status, _ := doScoreReq(t, h, http.MethodPut, "/api/v1/scores/"+uuid.New().String(), `{"score_value":90,"metadata":{"k":"v"}}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.True(t, tx.updateCalled)
	assert.True(t, tx.rollbackCalled, "Commit 실패 시 committed-defer Rollback")
}

// TestScoreHandler_Supersede_CommitFailure_Rollback Supersede 후 Commit 실패 → 500 + Rollback
// (AC-SCORE-API-004-2)
func TestScoreHandler_Supersede_CommitFailure_Rollback(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{supersedeID: uuid.New(), commitErr: errors.New("commit failed")}
	h, _ := newTestScoreHandler(t, tx)

	status, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores/"+uuid.New().String()+"/supersede", `{"score_value":88}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.True(t, tx.supersedeCalled)
	assert.True(t, tx.rollbackCalled)
}

// TestScoreHandler_Supersede_400_MissingScoreValue supersede body score_value 누락 → 400, TX 미진입
// (AC-SCORE-API-002-4 supersede pre-TX 검증)
func TestScoreHandler_Supersede_400_MissingScoreValue(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	h, st := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodPost, "/api/v1/scores/"+uuid.New().String()+"/supersede", `{"weight":1.0}`)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.False(t, st.beginCalled, "score_value 누락 supersede는 TX 미진입")
	errObj, _ := body["error"].(map[string]any)
	assert.Equal(t, "score_value", errObj["field"])
}

// TestScoreHandler_Supersede_400_Malformed supersede malformed UUID/JSON → 400
// (AC-SCORE-API-002-5)
func TestScoreHandler_Supersede_400_Malformed(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{}
	h, st := newTestScoreHandler(t, tx)

	st1, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores/not-a-uuid/supersede", `{"score_value":88}`)
	assert.Equal(t, http.StatusBadRequest, st1)
	assert.False(t, st.beginCalled)

	st2, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores/"+uuid.New().String()+"/supersede", `{bad`)
	assert.Equal(t, http.StatusBadRequest, st2)
}

// TestScoreHandler_CreateScore_StoreError InsertScore store 에러 → 매핑 status + Rollback
// (AC-SCORE-API-004-1/004-2 mutation downstream 실패)
func TestScoreHandler_CreateScore_StoreError(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	tx := &fakeScoreTx{insertErr: apperrors.ErrScoreInvalidInput}
	h, _ := newTestScoreHandler(t, tx)

	status, _ := doScoreReq(t, h, http.MethodPost, "/api/v1/scores",
		`{"evaluation_item_id":"EI-1","level":"raw","score_value":80,"evidence_id":"`+uuid.New().String()+`"}`)
	assert.Equal(t, http.StatusBadRequest, status, "ErrScoreInvalidInput → 400")
	assert.True(t, tx.insertCalled)
	assert.True(t, tx.rollbackCalled, "store 에러 시 committed-defer Rollback")
}

// TestScoreHandler_Rollup_NumericInvalid Numeric.Value() 실패(미설정 Numeric) → 빈 문자열 안전 직렬화
// (AC-SCORE-API-001-4 — handleRollup numeric 분기, valErr 경로 방어)
func TestScoreHandler_Rollup_ZeroNumeric(t *testing.T) {
	defer goleak.VerifyNone(t, scoreGoLeakOptions...)

	// Valid=false인 zero Numeric → Value()는 (nil,nil) 반환 → 빈 문자열, 200 유지
	tx := &fakeScoreTx{weightedSum: pgtype.Numeric{}}
	h, _ := newTestScoreHandler(t, tx)

	status, body := doScoreReq(t, h, http.MethodGet, "/api/v1/scores/rollup?evaluation_item_id=EI-1", "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "EI-1", body["evaluation_item_id"])
	assert.Equal(t, "", body["weighted_sum"], "Valid=false Numeric은 빈 십진 문자열")
}
