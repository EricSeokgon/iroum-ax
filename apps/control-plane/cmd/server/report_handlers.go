// report_handlers.go — 평가 결과 리포트/집계 HTTP API 핸들러 (SPEC-AX-REPORT-001)
//
// 라우트(ServeMux Go1.22+): GET /api/v1/reports/category/{id} (단건, read-only)
//
// 본 SPEC은 SPEC-AX-SCORE-001(점수 store) · SPEC-AX-EVAL-ITEM-001(평가항목 taxonomy) ·
// SPEC-AX-AUTH-003(ABAC) · SPEC-AX-SCORE-API-001(핸들러 선례)의 순수 read-only consumer다.
// consumer-only [HARD]: store/audit/auth/errors/score_handlers.go 0-diff, 신규 마이그레이션 0,
// 신규 store 메서드 0, 신규 외부 의존 0(math/big stdlib), 자체 audit 0(read-only — mutation 0).
//
// 범주 롤업 = cross-store 2-TX read 조합 (§6.1 OPEN #1 RESOLVED Option A):
//
//	TX#1 EvalItemTx: GetEvalItemByID(범주 검증)+GetEvalItemsByParentID(1-level 직계 자식)
//	TX#2 ScoreTx:    자식별 SumWeightedByEvaluationItem 누적 + DetermineGrade(범주 등급)
//
// 두 TX 모두 Commit 없음 — defer Rollback만 (read-only, score_handlers.go:348/393 미러).
// 정밀도 누적 = pgtype.Numeric{Int,Exp} → math/big.Rat 무손실 (§6.2 OPEN #2, float64 미경유).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// maxReportCategoryIDLen evaluation_items.id VARCHAR(64) DDL 정합
// (DB ERROR 22001 누출 방지 — score_handlers.go:32 maxScoreEvalItemIDLen 동형).
const maxReportCategoryIDLen = 64

// reportNumericScale SCORE-001 집계가 numeric(12,4) 고정 scale (score.go SEC-03).
// 입력이 모두 scale≤4 십진이므로 합도 scale≤4, FloatString(4) 반올림 무발생.
const reportNumericScale = 4

// ReportHandler 범주 집계 리포트 REST 엔드포인트 핸들러 (read-only).
// scoreStore + evalItemStore 두 store 별개 의존 (cross-store 2-TX — §6.1 OPEN #1).
// recorder 미주입 — read-only이므로 mutation 0 → audit 0 (REQ-REPORT-UBI-002).
// write-role 게이트 미차용 — score_handlers.go:161-187 requireScoreWriteRole 비차용 (§1.5).
// 필드 순서: 인터페이스(16B) × 2 → 포인터(8B) (govet fieldalignment 정렬)
//
// @MX:ANCHOR: [AUTO] 리포트 REST 진입점 — 핸들러 단위 테스트 + 서버 마운트 + Routes() 3곳 이상
// @MX:REASON: 범주 집계 리포트 단일 HTTP 계약 (SPEC-AX-REPORT-001 GET /api/v1/reports/category/{id})
type ReportHandler struct {
	scoreStore    store.ScoreStore
	evalItemStore store.EvalItemStore
	logger        *zap.Logger
}

// NewReportHandler 리포트 핸들러를 생성한다 (2 store 주입 — recorder/write-role 없음).
// server.go에서 NewReportHandler(pgStore, pgStore, logger) — PgWorkflowStore가
// ScoreStore(pg_store.go:134)+EvalItemStore(pg_store.go:118) 동시 구현 (source-verified).
func NewReportHandler(ss store.ScoreStore, eis store.EvalItemStore, logger *zap.Logger) *ReportHandler {
	return &ReportHandler{scoreStore: ss, evalItemStore: eis, logger: logger}
}

// Routes 리포트 라우트 1개를 등록한 http.Handler 반환 (단건만 — §6.3 O1 비활성).
//
// @MX:ANCHOR: [AUTO] 단일 라우트 등록 지점 — server.go 마운트 + 핸들러 테스트에서 사용
// @MX:REASON: PoC 단건 endpoint 불변 계약 (목록/페이지네이션 미적용 — §6.3 OPEN #3 RESOLVED)
func (h *ReportHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/reports/category/{id}", h.handleCategoryReport)
	return mux
}

// ── 표준 JSON/에러 헬퍼 (score_handlers.go:74-101 미러, 한국어, INFO) ────────────

// reportErrorBody 에러 응답 — {"error":{"code","message","field"}}
type reportErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

// writeReportJSON Content-Type 설정 후 JSON 직렬화 (score_handlers.go:84 미러).
func writeReportJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // 헤더 전송 후라 로깅 불가
}

// writeReportErr 표준 에러 본문 + INFO 로그 (거부는 INFO — score_handlers.go:91 미러).
func (h *ReportHandler) writeReportErr(w http.ResponseWriter, code int, errCode, msg, field string) {
	var body reportErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	body.Error.Field = field
	h.logger.Info("리포트 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("field", field),
	)
	writeReportJSON(w, code, body)
}

// ── 에러 매핑 (score_handlers.go:111-137 mapStoreErr 미러 — ErrGradeThresholds는 제외) ──

// mapReportStoreErr store 센티넬을 HTTP status + 에러코드 + 한국어 메시지로 결정적 매핑.
// ErrGradeThresholdsUnavailable은 핸들러-로컬 errors.Is 분기로 grade=null+200 흡수되므로
// 본 표에 미포함 (§6.3 B-2 graceful — mapStoreErr 0-diff 보존). unknown → 500.
//
// @MX:WARN: [AUTO] 센티넬→HTTP 매핑 누락 시 client 에러가 500으로 오분류된다
// @MX:REASON: EVAL-ITEM not-found→404 / invalid→400 / unknown→500 전수 매핑 — 신규 센티넬 시 갱신 필수
func mapReportStoreErr(err error) (int, string, string) {
	switch {
	case errors.Is(err, apperrors.ErrEvalItemNotFound):
		return http.StatusNotFound, "NOT_FOUND", "요청한 범주를 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrScoreNotFound):
		return http.StatusNotFound, "NOT_FOUND", "요청한 점수를 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrScoreInvalidInput):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "리포트 입력이 유효하지 않습니다"
	default:
		return http.StatusInternalServerError, "INTERNAL", "리포트 처리 중 오류가 발생했습니다"
	}
}

// writeReportStoreErr store 에러를 매핑하여 표준 본문으로 응답 (500은 ERROR 로그).
func (h *ReportHandler) writeReportStoreErr(w http.ResponseWriter, err error) {
	code, errCode, msg := mapReportStoreErr(err)
	if code == http.StatusInternalServerError {
		h.logger.Error("리포트 store 오류", zap.Error(err))
	}
	h.writeReportErr(w, code, errCode, msg, "")
}

// ── 입력 검증 (pre-store — score_handlers.go validateCreateBody:409-424 선례) ────

// parseCategoryID path {id}를 검증한다 (blank/>64자 → 400, store 미진입).
func (h *ReportHandler) parseCategoryID(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.PathValue("id"))
	if raw == "" {
		h.writeReportErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "범주 id는 필수입니다", "id")
		return "", false
	}
	if len(raw) > maxReportCategoryIDLen {
		h.writeReportErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "범주 id가 64자를 초과합니다", "id")
		return "", false
	}
	return raw, true
}

// ── 정밀도 누적 (§6.2 OPEN #2 — pgtype.Numeric → big.Rat 무손실, float64 미경유) ──

// numericToRat pgtype.Numeric{Int,Exp,Valid}를 *big.Rat로 무손실 변환한다.
// 값 = Int × 10^Exp (numeric.go:52-57). Exp≥0 → SetInt(Int×10^Exp),
// Exp<0 → SetFrac(Int, 10^|Exp|) (분모 10거듭제곱인 정확 유리수), Valid==false → 0.
// NaN/Infinity는 집계 부적합 — 0으로 안전 표면화 (score_handlers.go:362-367 선례 정합).
//
// @MX:WARN: [AUTO] float64 경유 시 0.1+0.2≠0.3 오차 축적·신규 외부 의존 도입 위험
// @MX:REASON: SEC-03 float64 미경유 불변식 — Exp 부호별 무손실 변환 보존 필수 (REQ-REPORT-001-S2)
func numericToRat(n pgtype.Numeric) *big.Rat {
	if !n.Valid || n.Int == nil || n.NaN || n.InfinityModifier != 0 {
		return new(big.Rat) // Valid=false/nil/비유한 → 0 기여 (빈 십진 안전)
	}
	r := new(big.Rat)
	if n.Exp >= 0 {
		// 10^Exp 정수배: Int × 10^Exp
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Exp)), nil)
		r.SetInt(new(big.Int).Mul(n.Int, scale))
		return r
	}
	// Exp<0: Int / 10^|Exp| — 분모가 10거듭제곱인 정확 유리수 (십진 손실 0)
	denom := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil)
	r.SetFrac(n.Int, denom)
	return r
}

// accumulateWeighted 자식 항목별 가중합(pgtype.Numeric)을 big.Rat accumulator에 누적한다.
// 유리수 누적이므로 오차 0 (0.1+0.2=0.3 정확). float64 미경유.
func accumulateWeighted(acc *big.Rat, n pgtype.Numeric) {
	acc.Add(acc, numericToRat(n))
}

// ── 응답 DTO (§6.3 OPEN #3 RESOLVED — research §14.2 채택) ──────────────────────

// reportItem 자식 평가항목별 가중합 (weighted_sum=십진 문자열, float64 미경유).
type reportItem struct {
	ItemID      string `json:"item_id"`
	ItemName    string `json:"item_name"`
	WeightedSum string `json:"weighted_sum"`
}

// categoryReportResponse 범주 집계 리포트 응답.
// category_grade는 string|null (grade_thresholds 미설정 시 null — B-2 graceful).
type categoryReportResponse struct {
	CategoryGrade *string      `json:"category_grade"`
	CategoryID    string       `json:"category_id"`
	CategoryName  string       `json:"category_name"`
	CategoryTotal string       `json:"category_total"`
	GeneratedAt   string       `json:"generated_at"`
	Items         []reportItem `json:"items"`
}

// ── 범주 롤업 핸들러 (cross-store 2-TX read 조합 — §6.1 OPEN #1 Option A) ────────

// handleCategoryReport GET /api/v1/reports/category/{id} — 범주 집계 리포트 (200/404/400/500).
//
// 시퀀스 (2개 독립 read TX, Commit 없음):
//
//	TX#1 EvalItemTx: GetEvalItemByID(범주 검증, not-found→404)
//	                 + GetEvalItemsByParentID(1-level 직계 자식, 빈 슬라이스 허용)
//	TX#2 ScoreTx:    자식별 SumWeightedByEvaluationItem big.Rat 누적
//	                 + DetermineGrade(범주 등급; ErrGradeThresholdsUnavailable→grade null+200)
//
// @MX:WARN: [AUTO] 2개 read TX defer Rollback 누락 시 커넥션 누수·goroutine 누출; 단일 store 가정 시 오작동
// @MX:REASON: cross-store 2-TX(EvalItemTx≠ScoreTx) 각 defer Rollback 불변식 — Commit 없음 read-only (REQ-REPORT-003-U1)
func (h *ReportHandler) handleCategoryReport(w http.ResponseWriter, r *http.Request) {
	categoryID, ok := h.parseCategoryID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// ── TX#1 EvalItemTx: 범주 검증 + 직계 자식 열거 ──
	category, children, ok := h.enumerateCategoryChildren(w, ctx, categoryID)
	if !ok {
		return
	}

	// ── TX#2 ScoreTx: 자식별 가중합 누적 + 범주 등급 ──
	st, err := h.scoreStore.BeginScoreTx(ctx)
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeReportErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = st.Rollback(ctx) }() //nolint:errcheck // 읽기 전용 — 항상 rollback

	acc := new(big.Rat)
	items := make([]reportItem, 0, len(children))
	for _, child := range children {
		sum, sumErr := st.SumWeightedByEvaluationItem(ctx, child.ID)
		if sumErr != nil {
			h.writeReportStoreErr(w, sumErr)
			return
		}
		accumulateWeighted(acc, sum)
		items = append(items, reportItem{
			ItemID:      child.ID,
			ItemName:    child.DisplayName,
			WeightedSum: numericToRat(sum).FloatString(reportNumericScale),
		})
	}

	categoryTotal := acc.FloatString(reportNumericScale)

	// 범주 등급: DetermineGrade(scope, total) — total은 store 계약상 float64 1회 변환
	// (D3-2: SEC-03 float64 미경유 범위는 N-항 누적 경로, 등급 임계 비교 입력 1회는 store 계약).
	var gradePtr *string
	totalF, _ := acc.Float64() // big.Rat → float64 1회 (DetermineGrade store 계약, score_handlers.go:382 동형)
	grade, gradeErr := st.DetermineGrade(ctx, categoryID, totalF)
	switch {
	case gradeErr == nil:
		g := grade
		gradePtr = &g
	case errors.Is(gradeErr, apperrors.ErrGradeThresholdsUnavailable):
		// B-2 graceful: 등급 기준 미설정 → grade null + 200 (핸들러-로컬 분기,
		// mapReportStoreErr에 미위임 — score_handlers.go mapStoreErr 0-diff §6.3)
		gradePtr = nil
	default:
		h.writeReportStoreErr(w, gradeErr)
		return
	}

	writeReportJSON(w, http.StatusOK, categoryReportResponse{
		CategoryID:    category.ID,
		CategoryName:  category.DisplayName,
		Items:         items,
		CategoryTotal: categoryTotal,
		CategoryGrade: gradePtr,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	})
}

// enumerateCategoryChildren TX#1 EvalItemTx — 범주 검증 + 1-level 직계 자식 열거.
// not-found 센티넬 → 404. 자식 0 → 빈 슬라이스(error 아님, store.go:201). Commit 없음.
func (h *ReportHandler) enumerateCategoryChildren(
	w http.ResponseWriter, ctx context.Context, categoryID string,
) (*store.EvalItem, []*store.EvalItem, bool) {
	eit, err := h.evalItemStore.BeginEvalItemTx(ctx)
	if err != nil {
		h.logger.Error("BeginEvalItemTx 실패", zap.Error(err))
		h.writeReportErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return nil, nil, false
	}
	defer func() { _ = eit.Rollback(ctx) }() //nolint:errcheck // 읽기 전용 — 항상 rollback

	category, err := eit.GetEvalItemByID(ctx, categoryID)
	if err != nil {
		h.writeReportStoreErr(w, err) // ErrEvalItemNotFound → 404
		return nil, nil, false
	}
	children, err := eit.GetEvalItemsByParentID(ctx, categoryID)
	if err != nil {
		h.writeReportStoreErr(w, err)
		return nil, nil, false
	}
	return category, children, true
}
