// report_handlers_test.go — 평가 결과 리포트/집계 HTTP API 핸들러 단위 테스트 (SPEC-AX-REPORT-001)
//
// 격리 전략: httptest + fake ScoreStore/ScoreTx/EvalItemStore/EvalItemTx
// (cross-store 2-TX 조합 — §6 OPEN #1 RESOLVED Option A). 통합은 SCORE-001/EVAL-ITEM-001 커버.
// integration 빌드 태그 미사용 — 기본 `go test ./apps/control-plane/cmd/server/`로 실행.
// SCORE-001 교훈: defer goleak.VerifyNone 테스트는 t.Parallel() 미사용 (형제 러너 오탐).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
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

// reportGoLeakOptions — score_handlers_test.go:42-50 동형 (httptest 인프라 + 형제 러너 제외).
// 핸들러의 cross-store 2-TX 실누출은 이 옵션 후에도 엄격 탐지 (T-011 fault-inject 충족).
var reportGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("testing.tRunner.func1"),
	goleak.IgnoreTopFunction("testing.tRunner"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
}

// withReportTestUser auth.WithUser로 principal을 context에 주입 (viewer/analyst/admin).
func withReportTestUser(ctx context.Context, scope string) context.Context {
	return auth.WithUser(ctx, &auth.User{UID: "test-user", Scopes: []string{scope}})
}

// numFromString text-format 십진 문자열을 pgtype.Numeric로 변환 (테스트 시드 헬퍼).
func numFromString(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	require.NoError(t, n.Scan(s))
	return n
}

// ── fake EvalItemStore / EvalItemTx (TX#1 — 범주 검증 + 직계 자식 열거) ──────────

// fakeEvalItemTx store.EvalItemTx 인메모리 fake — 범주 조회/자식 열거 반환 주입 + 호출 기록.
// 필드 순서: slice → 포인터(error/*EvalItem) → string → bool (govet fieldalignment).
type fakeEvalItemTx struct {
	getByIDErr     error
	childrenErr    error
	getByIDResult  *store.EvalItem
	children       []*store.EvalItem
	rollbackCalled bool
	commitCalled   bool
}

func (f *fakeEvalItemTx) InsertEvalItem(_ context.Context, _ string, _ *string, _, _ string, _ *int, _ string, _ *float64, _ *int, _ map[string]any) (string, error) {
	return "", nil
}

func (f *fakeEvalItemTx) GetEvalItemByID(_ context.Context, _ string) (*store.EvalItem, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	return f.getByIDResult, nil
}

func (f *fakeEvalItemTx) GetEvalItemsByParentID(_ context.Context, _ string) ([]*store.EvalItem, error) {
	if f.childrenErr != nil {
		return nil, f.childrenErr
	}
	return f.children, nil
}

func (f *fakeEvalItemTx) UpdateEvalItem(_ context.Context, _ string, _ store.EvalItemUpdate) error {
	return nil
}

func (f *fakeEvalItemTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }

func (f *fakeEvalItemTx) Commit(_ context.Context) error {
	f.commitCalled = true
	return nil
}

func (f *fakeEvalItemTx) Rollback(_ context.Context) error {
	f.rollbackCalled = true
	return nil
}

// fakeEvalItemStore store.EvalItemStore fake — BeginEvalItemTx로 주입된 tx 반환.
type fakeEvalItemStore struct {
	tx          *fakeEvalItemTx
	beginErr    error
	beginCalled bool
}

func (f *fakeEvalItemStore) BeginEvalItemTx(_ context.Context) (store.EvalItemTx, error) {
	f.beginCalled = true
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// ── fake ScoreStore / ScoreTx (TX#2 — 자식별 가중합 + 범주 등급) ─────────────────

// fakeReportScoreTx store.ScoreTx 인메모리 fake (리포트 경로 한정 — sum/grade/rollback).
// sumByItem: evaluation_item_id → pgtype.Numeric 매핑으로 항목레벨 집계 경로를 명시 검증.
type fakeReportScoreTx struct {
	gradeErr       error
	sumErr         error
	sumByItem      map[string]pgtype.Numeric
	grade          string
	sumCalls       []string
	rollbackCalled bool
	commitCalled   bool
}

func (f *fakeReportScoreTx) InsertScore(_ context.Context, _ string, _ *uuid.UUID, _ string, _ float64, _ *float64, _ map[string]any) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (f *fakeReportScoreTx) GetScoreByID(_ context.Context, _ uuid.UUID) (*store.Score, error) {
	return nil, nil
}

func (f *fakeReportScoreTx) GetScoresByEvaluationItem(_ context.Context, _ string) ([]*store.Score, error) {
	return nil, nil
}

func (f *fakeReportScoreTx) UpdateScore(_ context.Context, _ uuid.UUID, _ store.ScoreUpdate) error {
	return nil
}

func (f *fakeReportScoreTx) SupersedeAndReplaceScore(_ context.Context, _ uuid.UUID, _ float64, _ *float64, _ map[string]any) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (f *fakeReportScoreTx) SumWeightedByEvaluationItem(_ context.Context, itemID string) (pgtype.Numeric, error) {
	f.sumCalls = append(f.sumCalls, itemID)
	if f.sumErr != nil {
		return pgtype.Numeric{}, f.sumErr
	}
	if n, ok := f.sumByItem[itemID]; ok {
		return n, nil
	}
	return pgtype.Numeric{}, nil // 미시드 항목 → Valid=false (점수 0건 표면화)
}

func (f *fakeReportScoreTx) DetermineGrade(_ context.Context, _ string, _ float64) (string, error) {
	if f.gradeErr != nil {
		return "", f.gradeErr
	}
	return f.grade, nil
}

func (f *fakeReportScoreTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }

func (f *fakeReportScoreTx) Commit(_ context.Context) error {
	f.commitCalled = true
	return nil
}

func (f *fakeReportScoreTx) Rollback(_ context.Context) error {
	f.rollbackCalled = true
	return nil
}

// fakeReportScoreStore store.ScoreStore fake — BeginScoreTx로 주입된 tx 반환.
type fakeReportScoreStore struct {
	tx          *fakeReportScoreTx
	beginErr    error
	beginCalled bool
}

func (f *fakeReportScoreStore) BeginScoreTx(_ context.Context) (store.ScoreTx, error) {
	f.beginCalled = true
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// ── 테스트 핸들러 구성 + 요청 헬퍼 ───────────────────────────────────────────────

// newTestReportHandler fake 2 store로 ReportHandler 구성 (auth 비활성 기본).
func newTestReportHandler(t *testing.T, eit *fakeEvalItemTx, st *fakeReportScoreTx) (*ReportHandler, *fakeReportScoreStore, *fakeEvalItemStore) {
	t.Helper()
	ss := &fakeReportScoreStore{tx: st}
	eis := &fakeEvalItemStore{tx: eit}
	h := NewReportHandler(ss, eis, zaptest.NewLogger(t))
	return h, ss, eis
}

// doReportReq Routes() 핸들러에 요청을 보내고 status + 파싱 본문 반환.
func doReportReq(t *testing.T, h *ReportHandler, ctx context.Context, target string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	var parsed map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed) //nolint:errcheck
	return rec.Code, parsed
}

// rawReportReq body를 raw string으로 반환 (JSON 외 응답/엄밀 직렬화 검사용).
func rawReportReq(t *testing.T, h *ReportHandler, target string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	b, _ := io.ReadAll(rec.Body) //nolint:errcheck
	return rec.Code, string(b)
}

// readSource 핸들러 소스의 "코드 라인만" 반환한다 (// 주석 라인/주석부 제거).
// UBI 정적 검사는 실제 코드 구성을 검증해야 하며, 부재를 설명하는 문서 주석
// ("recorder 미주입", "Commit 없음" 등)은 검사 대상이 아니다.
func readSource(t *testing.T, name string) (string, error) {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	var code strings.Builder
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue // 주석 전용 라인 제외
		}
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i] // 행말 주석 제거 (문자열 리터럴 내 // 미발생 — 본 핸들러 한정)
		}
		code.WriteString(line)
		code.WriteByte('\n')
	}
	return code.String(), nil
}

// ════════════════════════════════════════════════════════════════════════════
// T-002/T-003 [S0/S1] RED-time 시드 가정 + ReportHandler struct + Routes() 1 라우트
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_SeedAssumption_ItemLevelAggregation §A.1(c) 잔여 가정 검증:
// "범주 직계 자식 항목 id로 SumWeightedByEvaluationItem이 non-zero를 반환"하는
// 항목레벨 집계 경로가 fake ScoreTx로 구조적으로 표현 가능함을 명시한다.
// (T-002 BLOCKING — 가정이 fake로 표현 불가하면 STOP. 표현 가능 → 진행.)
func TestReportHandler_SeedAssumption_ItemLevelAggregation(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	child := &store.EvalItem{ID: "AX-SAFETY-ORG-01", DisplayName: "안전조직"}
	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "AX-SAFETY-ORG", DisplayName: "안전보건"},
		children:      []*store.EvalItem{child},
	}
	st := &fakeReportScoreTx{
		sumByItem: map[string]pgtype.Numeric{"AX-SAFETY-ORG-01": numFromString(t, "85.5000")},
		grade:     "A",
	}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, body := doReportReq(t, h, nil, "/api/v1/reports/category/AX-SAFETY-ORG")

	require.Equal(t, http.StatusOK, code)
	// 항목레벨 집계 경로 검증: 직계 자식 id로 SumWeightedByEvaluationItem 호출됨
	assert.Equal(t, []string{"AX-SAFETY-ORG-01"}, st.sumCalls,
		"§A.1(c): 직계 자식 항목 id로 항목레벨 가중합 호출 (시드 가정 충족)")
	assert.Equal(t, "85.5000", body["category_total"])
}

// TestReportHandler_RoutesRegistersOneRoute Routes()가 단건 라우트 1개를 등록하고
// GET /api/v1/reports/category/{id}가 매칭됨을 검증한다 (§6.3 단건 endpoint, AC-REPORT-003-3).
func TestReportHandler_RoutesRegistersOneRoute(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"}, children: []*store.EvalItem{}}
	st := &fakeReportScoreTx{grade: "B"}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, _ := doReportReq(t, h, nil, "/api/v1/reports/category/C1")
	require.Equal(t, http.StatusOK, code, "단건 범주 리포트 라우트가 등록되어야 한다")

	// 미등록 경로(목록 /api/v1/reports)는 404 ServeMux (PoC 단건만 — O1 비활성)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "목록 endpoint는 PoC 미적용 (O1 비활성)")
}

// TestReportHandler_CrossStoreTwoDistinctStores AC-REPORT-003-3 — ReportHandler가
// ScoreStore와 EvalItemStore를 별개 의존으로 보유 (단일 store 가정 금지).
func TestReportHandler_CrossStoreTwoDistinctStores(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"}, children: []*store.EvalItem{}}
	st := &fakeReportScoreTx{grade: "B"}
	h, ss, eis := newTestReportHandler(t, eit, st)

	_, _ = doReportReq(t, h, nil, "/api/v1/reports/category/C1")

	assert.True(t, eis.beginCalled, "EvalItemStore.BeginEvalItemTx 호출 (TX#1 — 자식 열거)")
	assert.True(t, ss.beginCalled, "ScoreStore.BeginScoreTx 호출 (TX#2 — 가중합)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-008 [S3] accumulateWeighted — pgtype.Numeric → big.Rat float64 미경유 정밀도
// ════════════════════════════════════════════════════════════════════════════

// TestAccumulateWeighted_PrecisionNoFloat64 0.1+0.2 누적 오차 0 + 십진 round-trip.
// big.Rat 누적은 유리수이므로 float64 0.1+0.2=0.30000000000000004 오차 미발생.
// (AC-REPORT-001-5, §6.2 OPEN #2 RESOLVED, edge #6)
func TestAccumulateWeighted_PrecisionNoFloat64(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	acc := new(big.Rat)
	addWeightedToRat(t, acc, numFromString(t, "0.1000"))
	addWeightedToRat(t, acc, numFromString(t, "0.2000"))
	assert.Equal(t, "0.3000", acc.FloatString(4), "0.1+0.2는 정확히 0.3000 (float64 미경유)")

	// Valid=false (점수 0건) → 0 기여
	acc2 := new(big.Rat)
	addWeightedToRat(t, acc2, pgtype.Numeric{})
	assert.Equal(t, "0.0000", acc2.FloatString(4), "Valid=false numeric은 0 기여")

	// 음수 Exp (소수) + 양수 Exp(정수 scaled) round-trip 무손실
	acc3 := new(big.Rat)
	addWeightedToRat(t, acc3, numFromString(t, "12.3456"))
	addWeightedToRat(t, acc3, numFromString(t, "0.0044"))
	assert.Equal(t, "12.3500", acc3.FloatString(4), "음수 Exp 십진 무손실 누적")
}

// TestNumericToRat_ExpBranches Exp≥0(정수 scaled, SetInt) / Exp<0(SetFrac) /
// NaN·Infinity 가드(0) 분기를 직접 검증한다 (§6.2 무손실 변환 전 분기).
func TestNumericToRat_ExpBranches(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	// Exp>=0: Int=12, Exp=2 → 1200 (정수 scaled, SetInt 경로)
	posExp := pgtype.Numeric{Int: big.NewInt(12), Exp: 2, Valid: true}
	assert.Equal(t, "1200.0000", numericToRat(posExp).FloatString(4), "Exp>=0 SetInt(Int×10^Exp)")

	// Exp=0: Int=7 → 7 (10^0=1)
	zeroExp := pgtype.Numeric{Int: big.NewInt(7), Exp: 0, Valid: true}
	assert.Equal(t, "7.0000", numericToRat(zeroExp).FloatString(4), "Exp=0 → Int 그대로")

	// NaN → 0 기여 (집계 부적합 안전 표면화)
	nan := pgtype.Numeric{NaN: true, Valid: true}
	assert.Equal(t, "0.0000", numericToRat(nan).FloatString(4), "NaN → 0")

	// Infinity → 0 기여
	inf := pgtype.Numeric{InfinityModifier: pgtype.Infinity, Valid: true}
	assert.Equal(t, "0.0000", numericToRat(inf).FloatString(4), "Infinity → 0")

	// Int=nil (Valid=true이나 Int 미설정) → 0
	nilInt := pgtype.Numeric{Valid: true}
	assert.Equal(t, "0.0000", numericToRat(nilInt).FloatString(4), "Int=nil → 0")
}

// addWeightedToRat 테스트 헬퍼 — accumulateWeighted 내부 변환 단위 검증 위임.
func addWeightedToRat(t *testing.T, acc *big.Rat, n pgtype.Numeric) {
	t.Helper()
	acc.Add(acc, numericToRat(n))
}

// ════════════════════════════════════════════════════════════════════════════
// T-004 [S1] 표준 에러 스키마 {"error":{"code","message","field"}} (한국어)
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_ErrorBodySchema 에러 본문이 score_handlers.go:74-101 동일 스키마
// {"error":{"code","message","field"}}로 직렬화되고 한국어 메시지임을 검증한다.
func TestReportHandler_ErrorBodySchema(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDErr: apperrors.ErrEvalItemNotFound}
	st := &fakeReportScoreTx{}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, raw := rawReportReq(t, h, "/api/v1/reports/category/NOPE")
	require.Equal(t, http.StatusNotFound, code)

	var parsed reportErrorBody
	require.NoError(t, json.Unmarshal([]byte(raw), &parsed))
	assert.Equal(t, "NOT_FOUND", parsed.Error.Code)
	assert.Contains(t, parsed.Error.Message, "범주", "한국어 메시지")
	assert.NotEmpty(t, parsed.Error.Message)
}

// ════════════════════════════════════════════════════════════════════════════
// T-005 [S2] parseCategoryID pre-store 검증 → 400, store 미진입
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_InputValidation400 공백/64자 초과 범주 id → 400 INVALID_ARGUMENT,
// EvalItemStore/ScoreStore 미진입 (TX 0건). (AC-REPORT-001-3, edge #2)
func TestReportHandler_InputValidation400(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	tests := []struct {
		name   string
		target string
	}{
		{"blank(공백) id", "/api/v1/reports/category/%20%20"},
		{"64자 초과 id", "/api/v1/reports/category/" + strings.Repeat("X", 65)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eit := &fakeEvalItemTx{}
			st := &fakeReportScoreTx{}
			h, ss, eis := newTestReportHandler(t, eit, st)

			code, body := doReportReq(t, h, nil, tc.target)

			require.Equal(t, http.StatusBadRequest, code)
			errObj, _ := body["error"].(map[string]any)
			require.NotNil(t, errObj)
			assert.Equal(t, "INVALID_ARGUMENT", errObj["code"])
			assert.Equal(t, "id", errObj["field"], "field=id 지정")
			assert.False(t, eis.beginCalled, "EvalItemStore 미진입 (pre-store)")
			assert.False(t, ss.beginCalled, "ScoreStore 미진입 (pre-store)")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-006/T-007 [S2] cross-store 2-TX 조합 정상 200 + 응답 직렬화
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_CategoryReport200 존재 범주 + 자식 + 점수 → 200 cross-store 조합.
// (i) EvalItemTx GetEvalItemByID+GetEvalItemsByParentID (ii) ScoreTx 자식별 SumWeighted+DetermineGrade.
// (AC-REPORT-001-1, AC-REPORT-001-2(범주 검증 경로))
func TestReportHandler_CategoryReport200(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "AX-SAFETY-ORG", DisplayName: "안전보건"},
		children: []*store.EvalItem{
			{ID: "AX-SAFETY-ORG-01", DisplayName: "안전조직"},
			{ID: "AX-SAFETY-ORG-02", DisplayName: "안전계획"},
		},
	}
	st := &fakeReportScoreTx{
		sumByItem: map[string]pgtype.Numeric{
			"AX-SAFETY-ORG-01": numFromString(t, "40.0000"),
			"AX-SAFETY-ORG-02": numFromString(t, "45.5000"),
		},
		grade: "A",
	}
	h, ss, eis := newTestReportHandler(t, eit, st)

	code, body := doReportReq(t, h, nil, "/api/v1/reports/category/AX-SAFETY-ORG")

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "AX-SAFETY-ORG", body["category_id"])
	assert.Equal(t, "안전보건", body["category_name"])
	assert.Equal(t, "85.5000", body["category_total"], "40+45.5=85.5 정확 누적")
	assert.Equal(t, "A", body["category_grade"])
	assert.NotEmpty(t, body["generated_at"])

	items, _ := body["items"].([]any)
	require.Len(t, items, 2)
	first, _ := items[0].(map[string]any)
	assert.Equal(t, "AX-SAFETY-ORG-01", first["item_id"])
	assert.Equal(t, "안전조직", first["item_name"])
	assert.Equal(t, "40.0000", first["weighted_sum"])

	// cross-store 2-TX 분리 + 자식별 호출 검증
	assert.True(t, eis.beginCalled, "TX#1 EvalItemTx")
	assert.True(t, ss.beginCalled, "TX#2 ScoreTx")
	assert.True(t, eit.rollbackCalled, "EvalItemTx defer Rollback (Commit 없음)")
	assert.True(t, st.rollbackCalled, "ScoreTx defer Rollback (Commit 없음)")
	assert.False(t, eit.commitCalled, "EvalItemTx Commit 없음 (read-only)")
	assert.False(t, st.commitCalled, "ScoreTx Commit 없음 (read-only)")
	assert.Equal(t, []string{"AX-SAFETY-ORG-01", "AX-SAFETY-ORG-02"}, st.sumCalls)
}

// ════════════════════════════════════════════════════════════════════════════
// T-009 [S4] 빈/누락 표면화 (B-2) — 자식0/점수0/grade미설정 → 200 graceful
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_EmptyReport_NoChildren 자식 0 범주 → 200 빈 리포트
// items:[], category_total:"0", category_grade:null (404/500 아님). (AC-REPORT-001-4, edge #3/#10)
func TestReportHandler_EmptyReport_NoChildren(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "빈범주"},
		children:      []*store.EvalItem{}, // GetEvalItemsByParentID 빈 슬라이스 (error 아님)
	}
	st := &fakeReportScoreTx{grade: "C"}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, raw := rawReportReq(t, h, "/api/v1/reports/category/C1")
	require.Equal(t, http.StatusOK, code, "자식 0은 정상 상태 — 404/500 아님")

	var resp categoryReportResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	assert.Equal(t, []reportItem{}, resp.Items, "items:[] (제외 아님)")
	assert.Equal(t, "0.0000", resp.CategoryTotal, "범주 총합 0")
	assert.Contains(t, raw, `"items":[]`, "빈 배열 직렬화")
	assert.Empty(t, st.sumCalls, "자식 0이므로 SumWeighted 미호출")
}

// TestReportHandler_ZeroScoreItem 자식 item에 기여 점수 0건 → 200, weighted_sum:"0".
// (AC-REPORT-001-4, edge #4)
func TestReportHandler_ZeroScoreItem(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"},
		children:      []*store.EvalItem{{ID: "C1-01", DisplayName: "점수없음항목"}},
	}
	// sumByItem 미시드 → Valid=false (점수 0건 표면화)
	st := &fakeReportScoreTx{sumByItem: map[string]pgtype.Numeric{}, grade: "D"}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, body := doReportReq(t, h, nil, "/api/v1/reports/category/C1")
	require.Equal(t, http.StatusOK, code)
	items, _ := body["items"].([]any)
	require.Len(t, items, 1)
	first, _ := items[0].(map[string]any)
	assert.Equal(t, "0.0000", first["weighted_sum"], "점수 0건 → weighted_sum 0 (제외 아님)")
	assert.Equal(t, "0.0000", body["category_total"])
}

// TestReportHandler_GradeNull_B2Graceful grade_thresholds 미설정
// (DetermineGrade→ErrGradeThresholdsUnavailable) → 200 + category_grade:null (B-2 graceful).
// 핸들러-로컬 errors.Is 분기 흡수 — 404 아님. (AC-REPORT-003-1, edge #5, §6.3 B-2)
func TestReportHandler_GradeNull_B2Graceful(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"},
		children:      []*store.EvalItem{{ID: "C1-01", DisplayName: "항목"}},
	}
	st := &fakeReportScoreTx{
		sumByItem: map[string]pgtype.Numeric{"C1-01": numFromString(t, "70.0000")},
		gradeErr:  apperrors.ErrGradeThresholdsUnavailable,
	}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, raw := rawReportReq(t, h, "/api/v1/reports/category/C1")
	require.Equal(t, http.StatusOK, code, "B-2 graceful — 404 아님 (집계 리포트는 등급이 부가 필드)")

	var resp categoryReportResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	assert.Nil(t, resp.CategoryGrade, "category_grade=null (등급 fabricate 금지)")
	assert.Contains(t, raw, `"category_grade":null`, "null 직렬화 (빈 문자열 아님)")
	assert.Equal(t, "70.0000", resp.CategoryTotal, "점수는 정상 산출 (정보 손실 0)")
}

// TestReportHandler_GradeError_NonUnavailable_NotAbsorbed (D-1 negative-control)
// B-2 graceful 분기가 ErrGradeThresholdsUnavailable에만 **좁게** 적용됨을 pin한다
// (strategy.md §A.3 B-2 불변식 회귀 방어). DetermineGrade가 Unavailable이 **아닌**
// 일반 store 에러를 반환하면 → handleCategoryReport default arm → writeReportStoreErr →
// (1) HTTP 500, (2) category_grade가 null로 흡수되지 **않고** 표준 에러 본문 반환,
// (3) errors.Is 분기가 Unavailable 센티넬에만 좁게 매칭됨을 검증한다.
// (AC-REPORT-003-1 negative-control, §6.3 B-2 narrow-scope 회귀 방어)
func TestReportHandler_GradeError_NonUnavailable_NotAbsorbed(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"},
		children:      []*store.EvalItem{{ID: "C1-01", DisplayName: "항목"}},
	}
	st := &fakeReportScoreTx{
		sumByItem: map[string]pgtype.Numeric{"C1-01": numFromString(t, "70.0000")},
		// non-Unavailable 일반 에러 — B-2 graceful 흡수 대상이 아님 (좁은 분기 pin)
		gradeErr: errors.New("pgx: grade query failed"),
	}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, raw := rawReportReq(t, h, "/api/v1/reports/category/C1")

	// (1) HTTP 500 — default arm으로 떨어져 writeReportStoreErr 처리 (200 흡수 아님)
	require.Equal(t, http.StatusInternalServerError, code,
		"non-Unavailable grade 에러는 B-2 graceful 흡수 대상이 아니다 (좁은 분기)")

	// (2) category_grade null 흡수가 아니라 표준 에러 본문 반환
	var errBody reportErrorBody
	require.NoError(t, json.Unmarshal([]byte(raw), &errBody))
	assert.Equal(t, "INTERNAL", errBody.Error.Code, "default arm → INTERNAL 에러 코드")
	assert.NotEmpty(t, errBody.Error.Message, "한국어 에러 메시지 반환")
	assert.NotContains(t, raw, `"category_grade":null`,
		"category_grade가 null로 흡수되지 않음 (리포트 본문 아닌 에러 본문)")
	assert.NotContains(t, raw, `"category_total"`,
		"정상 리포트 본문이 아닌 에러 본문 — category_total 부재")

	// (3) errors.Is 분기가 Unavailable 센티넬에만 좁게 적용됨을 명시 (역검증)
	assert.False(t, errors.Is(st.gradeErr, apperrors.ErrGradeThresholdsUnavailable),
		"주입 에러는 Unavailable 센티넬이 아님 — B-2 분기 미발동이 정확한 거동")
}

// ════════════════════════════════════════════════════════════════════════════
// T-010 [S4] mapReportStoreErr — 센티넬→HTTP status 결정적 매핑
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_StoreErrorMapping category not-found→404 / invalid→400 / unknown→500.
// ErrGradeThresholdsUnavailable은 본 표 제외 (T-009 핸들러-로컬 흡수). (AC-REPORT-003-1)
func TestReportHandler_StoreErrorMapping(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	tests := []struct {
		getByID  error
		name     string
		wantErr  string
		wantCode int
	}{
		{apperrors.ErrEvalItemNotFound, "category not-found → 404", "NOT_FOUND", http.StatusNotFound},
		{apperrors.ErrScoreNotFound, "score not-found → 404", "NOT_FOUND", http.StatusNotFound},
		{apperrors.ErrScoreInvalidInput, "invalid input → 400", "INVALID_ARGUMENT", http.StatusBadRequest},
		{errors.New("pgx: connection reset"), "unwrapped/unknown → 500", "INTERNAL", http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eit := &fakeEvalItemTx{getByIDErr: tc.getByID}
			st := &fakeReportScoreTx{}
			h, _, _ := newTestReportHandler(t, eit, st)

			code, body := doReportReq(t, h, nil, "/api/v1/reports/category/C1")
			require.Equal(t, tc.wantCode, code)
			errObj, _ := body["error"].(map[string]any)
			require.NotNil(t, errObj)
			assert.Equal(t, tc.wantErr, errObj["code"])
		})
	}
}

// TestReportHandler_StoreErrorMapping_WrappedSentinel errors.Is로 래핑 센티넬도 식별.
func TestReportHandler_StoreErrorMapping_WrappedSentinel(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDErr: errors.Join(errors.New("ctx"), apperrors.ErrEvalItemNotFound)}
	st := &fakeReportScoreTx{}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, _ := doReportReq(t, h, nil, "/api/v1/reports/category/C1")
	assert.Equal(t, http.StatusNotFound, code, "errors.Is 래핑 센티넬 식별")
}

// ════════════════════════════════════════════════════════════════════════════
// T-011 [S4] cross-store 2-TX rollback fault-inject + goroutine 누출 0
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_EvalItemTxBeginFailure EvalItemStore.BeginEvalItemTx 실패 → 500,
// ScoreStore 미진입, goroutine 누출 0. (AC-REPORT-003-2, edge #11)
func TestReportHandler_EvalItemTxBeginFailure(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{}
	st := &fakeReportScoreTx{}
	ss := &fakeReportScoreStore{tx: st}
	eis := &fakeEvalItemStore{tx: eit, beginErr: errors.New("pool exhausted")}
	h := NewReportHandler(ss, eis, zaptest.NewLogger(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/category/C1", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.False(t, ss.beginCalled, "EvalItemTx 시작 실패 시 ScoreTx 미진입 (부분 상태 0)")
}

// TestReportHandler_ScoreTxBeginFailure ScoreStore.BeginScoreTx 실패 → 500,
// EvalItemTx는 이미 defer Rollback 정리됨, goroutine 누출 0. (AC-REPORT-003-2, edge #11)
func TestReportHandler_ScoreTxBeginFailure(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"},
		children:      []*store.EvalItem{{ID: "C1-01", DisplayName: "항목"}},
	}
	st := &fakeReportScoreTx{}
	ss := &fakeReportScoreStore{tx: st, beginErr: errors.New("pool exhausted")}
	eis := &fakeEvalItemStore{tx: eit}
	h := NewReportHandler(ss, eis, zaptest.NewLogger(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/category/C1", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.True(t, eit.rollbackCalled, "EvalItemTx는 ScoreTx 실패와 무관하게 defer Rollback 정리")
	assert.False(t, eit.commitCalled, "Commit 없음 (read-only)")
}

// TestReportHandler_ChildrenEnumerationError GetEvalItemsByParentID 실패 →
// EvalItemTx defer Rollback, ScoreStore 미진입, 매핑 status. (AC-REPORT-003-2, edge #11)
func TestReportHandler_ChildrenEnumerationError(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"},
		childrenErr:   errors.New("pgx: query failed"),
	}
	st := &fakeReportScoreTx{}
	h, ss, _ := newTestReportHandler(t, eit, st)

	code, _ := doReportReq(t, h, nil, "/api/v1/reports/category/C1")
	assert.Equal(t, http.StatusInternalServerError, code, "자식 열거 실패 → 매핑 status")
	assert.True(t, eit.rollbackCalled, "EvalItemTx defer Rollback (열거 실패 후)")
	assert.False(t, ss.beginCalled, "자식 열거 실패 시 ScoreTx 미진입")
}

// TestReportHandler_DownstreamFailure_BothTxRollback EvalItemTx 후 SumWeighted 실패 →
// 두 read TX 모두 defer Rollback, 부분 상태 0. (AC-REPORT-003-2, edge #11)
func TestReportHandler_DownstreamFailure_BothTxRollback(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{
		getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"},
		children:      []*store.EvalItem{{ID: "C1-01", DisplayName: "항목"}},
	}
	st := &fakeReportScoreTx{sumErr: errors.New("pgx: query timeout")}
	h, _, _ := newTestReportHandler(t, eit, st)

	code, _ := doReportReq(t, h, nil, "/api/v1/reports/category/C1")
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.True(t, eit.rollbackCalled, "EvalItemTx defer Rollback")
	assert.True(t, st.rollbackCalled, "ScoreTx defer Rollback (downstream 실패 후에도)")
	assert.False(t, eit.commitCalled)
	assert.False(t, st.commitCalled)
}

// ════════════════════════════════════════════════════════════════════════════
// T-012 [S5] ABAC read-narrowing — viewer read 200 / auth-disabled 투과 / admin /
//            write-role 게이트 부재 정적
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_ViewerReadAllowed viewer-only 인증 principal read → 200
// (read-only — narrowing이 거부할 write 없음, abac.go:4). (AC-REPORT-UBI-004-1, 001-6, 002-1, edge #8)
func TestReportHandler_ViewerReadAllowed(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"}, children: []*store.EvalItem{}}
	st := &fakeReportScoreTx{grade: "B"}
	h, _, _ := newTestReportHandler(t, eit, st)

	ctx := withReportTestUser(context.Background(), "iroum-ax:viewer")
	code, _ := doReportReq(t, h, ctx, "/api/v1/reports/category/C1")
	assert.Equal(t, http.StatusOK, code, "viewer-only read 허용 (read-narrowing 거부 없음)")
}

// TestReportHandler_AuthDisabledPassthrough auth context 부재(Walking Skeleton) → 투과 200.
// (AC-REPORT-UBI-003-1, 002-2, edge #7) — 핸들러는 user context 위조/주입 0 (UBI-003-2).
func TestReportHandler_AuthDisabledPassthrough(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"}, children: []*store.EvalItem{}}
	st := &fakeReportScoreTx{grade: "B"}
	h, _, _ := newTestReportHandler(t, eit, st)

	// ctx=nil → auth.User context 미주입 (auth-disabled). 핸들러는 식별자 위조 안 함.
	code, _ := doReportReq(t, h, nil, "/api/v1/reports/category/C1")
	assert.Equal(t, http.StatusOK, code, "auth-disabled 투과 — 정상 응답")
}

// TestReportHandler_AdminBypass RoleAdmin principal → 전 엔드포인트 우회 허용.
// (AC-REPORT-002-3, edge #9)
func TestReportHandler_AdminBypass(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDResult: &store.EvalItem{ID: "C1", DisplayName: "범주"}, children: []*store.EvalItem{}}
	st := &fakeReportScoreTx{grade: "A"}
	h, _, _ := newTestReportHandler(t, eit, st)

	ctx := withReportTestUser(context.Background(), "iroum-ax:admin")
	code, _ := doReportReq(t, h, ctx, "/api/v1/reports/category/C1")
	assert.Equal(t, http.StatusOK, code, "admin 우회 — 리포트 조회 허용")
}

// ════════════════════════════════════════════════════════════════════════════
// T-013 [S5] 횡단 UBI — 데이터 주권/감사 0/cli-anonymous 정적 검증
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_NoExternalDeps_NoAudit_NoWriteRole report_handlers.go 정적 검사:
// (UBI-001) 외부 host/SDK/shopspring 미import · (UBI-002) audit_logs INSERT/Recorder 0 ·
// (UBI-004) write-role 게이트(requireScoreWriteRole/guardScoreWrite) 0건.
// (AC-REPORT-UBI-001-1/-2, UBI-002-1, UBI-004-2, edge #12/#13)
func TestReportHandler_NoExternalDeps_NoAudit_NoWriteRole(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	src, err := readSource(t, "report_handlers.go")
	require.NoError(t, err)

	// UBI-001: 외부 의존/네트워크 0 (망분리)
	assert.NotContains(t, src, "shopspring", "shopspring/decimal 미import (신규 외부 의존 0)")
	assert.NotContains(t, src, "net/http.Client", "외부 HTTP 클라이언트 0")
	assert.NotContains(t, src, "http.Get(", "외부 HTTP egress 0")
	assert.NotContains(t, src, "http.Post(", "외부 HTTP egress 0")

	// UBI-002: read-only — audit_logs INSERT / Recorder 의존 0
	assert.NotContains(t, src, "audit_logs", "audit_logs SQL 0 (read-only)")
	assert.NotContains(t, src, "InsertAuditLog", "audit InsertAuditLog 호출 0")
	assert.NotContains(t, src, "recorder", "ReportHandler에 recorder 의존 미주입")
	assert.NotContains(t, src, "Recorder", "Recorder 타입 미사용")

	// UBI-004: write-role 게이트 0 (read-only — score_handlers.go:161-187 미차용)
	assert.NotContains(t, src, "requireScoreWriteRole", "write-role 게이트 미차용")
	assert.NotContains(t, src, "guardScoreWrite", "write 가드 미차용")
	assert.NotContains(t, src, "requireReportWriteRole", "자체 write-role 게이트 0")

	// read-only: mutation store 메서드 호출 0 (InsertScore/UpdateScore/Supersede/InsertEvalItem)
	assert.NotContains(t, src, ".InsertScore(", "mutation 0 (read-only)")
	assert.NotContains(t, src, ".UpdateScore(", "mutation 0")
	assert.NotContains(t, src, ".SupersedeAndReplaceScore(", "mutation 0")
	assert.NotContains(t, src, ".Commit(", "read-only — Commit 0 (defer Rollback만)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-014 [S6] ServeMux 최장일치 / 라우트 충돌 (단건 path-param)
// ════════════════════════════════════════════════════════════════════════════

// TestReportHandler_ServeMuxPathParam {id} path-param이 다양한 범주 id로 매칭됨을 검증.
func TestReportHandler_ServeMuxPathParam(t *testing.T) {
	defer goleak.VerifyNone(t, reportGoLeakOptions...)

	eit := &fakeEvalItemTx{getByIDResult: &store.EvalItem{ID: "x", DisplayName: "범주"}, children: []*store.EvalItem{}}
	st := &fakeReportScoreTx{grade: "A"}
	h, _, _ := newTestReportHandler(t, eit, st)

	for _, id := range []string{"AX-SAFETY-ORG", "C1", "a.b-c_d"} {
		code, _ := doReportReq(t, h, nil, "/api/v1/reports/category/"+id)
		assert.Equal(t, http.StatusOK, code, "path-param 매칭: "+id)
	}
	// POST는 미등록 메서드 (GET만 등록 — read-only)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/category/C1", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "POST 미등록 (read-only — GET만)")
}

// 미사용 import 가드 (errors/strings/assert는 후속 task에서 사용 — RED 단계 빌드 통과용)
var _ = errors.Is
var _ = strings.TrimSpace
var _ = apperrors.ErrEvalItemNotFound
