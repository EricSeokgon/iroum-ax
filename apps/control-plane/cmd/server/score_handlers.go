// score_handlers.go — 점수 조회/집계 HTTP API 핸들러 (SPEC-AX-SCORE-API-001)
//
// 라우트(ServeMux Go1.22+, 최장일치): GET /api/v1/scores/{id} · GET /api/v1/scores
// · GET /api/v1/scores/rollup · GET /api/v1/scores/grade · POST /api/v1/scores
// · PUT /api/v1/scores/{id} · POST /api/v1/scores/{id}/supersede
//
// 본 SPEC은 SPEC-AX-SCORE-001(store/audit)·EVID-001(핸들러 선례)·AUTH-003(ABAC)의
// 순수 consumer다. store/audit/auth/스키마 0-diff, 신규 마이그레이션 0,
// 자체 audit 0(store 계층 RecordScore* 동일 TX 전담). evidence_handlers.go 미러링.
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// 핸들러 사전 검증/페이지네이션 상수.
// evaluation_item_id VARCHAR(64) DDL 정합 (DB ERROR 22001 누출 방지, evidence_handlers.go:32-36 선례).
// maxListLimit/defaultListLimit: §6 OPEN #2 RESOLVED — store가 offset/limit 미지원이라
// 핸들러 메모리 슬라이싱, p99<50ms NFR 보호상 500 (workflow 1000 기각).
const (
	maxScoreEvalItemIDLen = 64
	maxListLimit          = 500
	defaultListLimit      = 50
)

// ScoreHandler 점수 조회/집계 REST 엔드포인트 핸들러.
// recorder 의존 미주입 — 감사는 store 계층(RecordScore*)이 동일 TX로 전담 (UBI-002-2).
// 필드 순서: 인터페이스(16B) → 포인터(8B) (fieldalignment 정렬)
//
// @MX:ANCHOR: [AUTO] 점수 REST 진입점 — 핸들러 단위 테스트 + 서버 마운트 + Routes() 3곳 이상에서 사용
// @MX:REASON: 점수 조회/집계/변경 단일 HTTP 계약 (SPEC-AX-SCORE-API-001, 7 엔드포인트)
type ScoreHandler struct {
	store  store.ScoreStore
	logger *zap.Logger
}

// NewScoreHandler 점수 핸들러를 생성한다 (store만 주입 — recorder 없음, UBI-002-2).
func NewScoreHandler(st store.ScoreStore, logger *zap.Logger) *ScoreHandler {
	return &ScoreHandler{store: st, logger: logger}
}

// Routes 점수 라우트 7개를 등록한 http.Handler 반환.
// ServeMux Go1.22+ 최장일치: /rollup·/grade·/{id}/supersede가 /{id}보다 우선 매칭된다
// (§7 edge #15 — /api/v1/scores/rollup 은 /api/v1/scores/{id}보다 구체적).
//
// @MX:ANCHOR: [AUTO] 7 라우트 등록 단일 지점 — server.go 마운트 + 전 핸들러 테스트에서 사용
// @MX:REASON: ServeMux 최장일치 우선순위 불변 계약 (edge #15 라우트 충돌 방지)
func (h *ScoreHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	// 구체 경로(최장일치 우선) 먼저 등록
	mux.HandleFunc("GET /api/v1/scores/rollup", h.handleRollup)
	mux.HandleFunc("GET /api/v1/scores/grade", h.handleGrade)
	mux.HandleFunc("POST /api/v1/scores/{id}/supersede", h.handleSupersedeScore)
	mux.HandleFunc("GET /api/v1/scores/{id}", h.handleGetScore)
	mux.HandleFunc("PUT /api/v1/scores/{id}", h.handleUpdateScore)
	mux.HandleFunc("GET /api/v1/scores", h.handleListScores)
	mux.HandleFunc("POST /api/v1/scores", h.handleCreateScore)
	return mux
}

// ── 표준 JSON/에러 헬퍼 (evidence_handlers.go:89-124 미러, 한국어, INFO) ────────

// scoreErrorBody 에러 응답 — {"error":{"code","message","field"}}
type scoreErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

// writeScoreJSON Content-Type 설정 후 JSON 직렬화
func writeScoreJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // 헤더 전송 후라 로깅 불가
}

// writeScoreErr 표준 에러 본문 + INFO 로그 (거부는 INFO 레벨)
func (h *ScoreHandler) writeScoreErr(w http.ResponseWriter, code int, errCode, msg, field string) {
	var body scoreErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	body.Error.Field = field
	h.logger.Info("점수 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("field", field),
	)
	writeScoreJSON(w, code, body)
}

// ── 에러 매핑 (T-011, AC-SCORE-API-004-1, errors.go:54-78 센티넬) ──────────────

// mapStoreErr store 센티넬을 HTTP status + 에러코드 + 한국어 메시지로 결정적 매핑한다.
// errors.Is로 래핑된 센티넬도 식별. unknown → 500.
//
// @MX:WARN: [AUTO] 센티넬→HTTP 매핑 누락 시 client 에러가 500으로 오분류된다
// @MX:REASON: errors.go:54-78 센티넬 7종 + unknown 전수 매핑 — 신규 센티넬 추가 시 본 표 갱신 필수
func mapStoreErr(err error) (int, string, string) {
	switch {
	case errors.Is(err, apperrors.ErrScoreNotFound):
		return http.StatusNotFound, "NOT_FOUND", "요청한 점수를 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrGradeThresholdsUnavailable):
		// 설계결정[D2-3]: scope 등급 자원 부재 → 404 (fail-closed, 등급 fabricate 금지)
		return http.StatusNotFound, "NOT_FOUND", "요청한 scope의 등급 기준이 설정되지 않았습니다"
	case errors.Is(err, apperrors.ErrScoreInvalidInput):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "점수 입력이 유효하지 않습니다"
	case errors.Is(err, apperrors.ErrScoreImmutable):
		return http.StatusConflict, "CONFLICT", "CONFIRMED 점수는 정정(supersede)으로만 수정 가능합니다"
	case errors.Is(err, apperrors.ErrScoreInvalidStatus):
		return http.StatusConflict, "CONFLICT", "허용되지 않은 점수 상태 전이입니다"
	case errors.Is(err, apperrors.ErrScoreNotConfirmed):
		return http.StatusConflict, "CONFLICT", "정정(supersede)은 CONFIRMED 점수에만 허용됩니다"
	default:
		return http.StatusInternalServerError, "INTERNAL", "점수 처리 중 오류가 발생했습니다"
	}
}

// writeStoreErr store 에러를 매핑하여 표준 본문으로 응답한다 (서버 결함은 ERROR 로그).
func (h *ScoreHandler) writeStoreErr(w http.ResponseWriter, err error) {
	code, errCode, msg := mapStoreErr(err)
	if code == http.StatusInternalServerError {
		h.logger.Error("점수 store 오류", zap.Error(err))
	}
	h.writeScoreErr(w, code, errCode, msg, "")
}

// ── 페이지네이션 clamp (T-006, AC-SCORE-API-001-7, §6 #2) ──────────────────────

// clampPagination limit/offset 쿼리 문자열을 결정적으로 보정한다.
// limit 누락/0 → defaultListLimit(50), limit>maxListLimit → 500, offset 음수/비수치 → 0.
func clampPagination(rawLimit, rawOffset string) (int, int) {
	limit := defaultListLimit
	if v, err := strconv.Atoi(rawLimit); err == nil && v > 0 {
		limit = v
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	offset := 0
	if v, err := strconv.Atoi(rawOffset); err == nil && v > 0 {
		offset = v
	}
	return limit, offset
}

// ── ABAC write-role 게이팅 (T-013, AC-SCORE-API-003, §6 #4) ────────────────────

// requireScoreWriteRole scope 문자열에서 추출한 역할이 write 권한({admin,analyst})인지 판단한다.
// OBS-001 metrics.IsMetricsAuthorized 동형 — auth.ParseRolesFromScope 재사용, frozen rbac.go 0-diff.
// (evaluator는 rbac.go:33 정규식 부재로 INFEASIBLE → RoleAnalyst 대체, strategy.md §A #4)
func requireScoreWriteRole(scope string) bool {
	for _, r := range auth.ParseRolesFromScope(scope) {
		if r == auth.RoleAdmin || r == auth.RoleAnalyst {
			return true
		}
	}
	return false
}

// guardScoreWrite mutation 진입 직전 ABAC narrowing 동형 게이트.
// auth context 부재(auth-disabled Walking Skeleton) → 투과(true). user 존재 시
// write 권한 없으면 403 ABAC_CONDITION_DENIED 응답 후 false 반환 (store TX 미진입).
//
// @MX:NOTE: [AUTO] abac.go:11 narrowing-only 동형 — ABAC는 allow 부여 없이 미인가만 거부.
// auth-disabled(ok=false)는 투과, RoleAdmin은 우회(write-role에 admin 포함).
func (h *ScoreHandler) guardScoreWrite(w http.ResponseWriter, r *http.Request) bool {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return true // auth-disabled 투과 (cli-anonymous store 위임, UBI-003)
	}
	if requireScoreWriteRole(strings.Join(u.Scopes, " ")) {
		return true
	}
	h.writeScoreErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
		"쓰기 권한이 없는 사용자입니다", "")
	return false
}

// ── 요청 파싱 헬퍼 ───────────────────────────────────────────────────────────

// scoreCreateBody POST/PUT/supersede 요청 본문
type scoreCreateBody struct {
	Metadata         map[string]any `json:"metadata"`
	ScoreValue       *float64       `json:"score_value"`
	Weight           *float64       `json:"weight"`
	EvaluationItemID string         `json:"evaluation_item_id"`
	Level            string         `json:"level"`
	EvidenceID       string         `json:"evidence_id"`
}

// scoreResponse Score 직렬화 응답 (전 필드)
type scoreResponse struct {
	Metadata         map[string]any `json:"metadata,omitempty"`
	ScoreValue       *float64       `json:"score_value,omitempty"`
	Weight           *float64       `json:"weight,omitempty"`
	EvidenceID       *string        `json:"evidence_id,omitempty"`
	ID               string         `json:"id"`
	EvaluationItemID string         `json:"evaluation_item_id"`
	Level            string         `json:"level"`
	Grade            string         `json:"grade,omitempty"`
	Status           string         `json:"status"`
	CreatedBy        string         `json:"created_by,omitempty"`
}

// toScoreResponse store.Score를 응답 DTO로 변환
func toScoreResponse(s *store.Score) scoreResponse {
	resp := scoreResponse{
		ID:               s.ID.String(),
		EvaluationItemID: s.EvaluationItemID,
		Level:            s.Level,
		Grade:            s.Grade,
		Status:           s.Status,
		CreatedBy:        s.CreatedBy,
		ScoreValue:       s.ScoreValue,
		Weight:           s.Weight,
		Metadata:         s.Metadata,
	}
	if s.EvidenceID != nil {
		eid := s.EvidenceID.String()
		resp.EvidenceID = &eid
	}
	return resp
}

// parseScoreID path {id} 세그먼트를 UUID로 파싱한다 (malformed → 400, TX 미진입).
func (h *ScoreHandler) parseScoreID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "점수 id가 유효한 UUID가 아닙니다", "id")
		return uuid.Nil, false
	}
	return id, true
}

// decodeScoreBody JSON 본문을 디코드한다 (malformed JSON → 400, TX 미진입).
func (h *ScoreHandler) decodeScoreBody(w http.ResponseWriter, r *http.Request) (*scoreCreateBody, bool) {
	var b scoreCreateBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return nil, false
	}
	return &b, true
}

// ── 조회 핸들러 (T-004/T-005/T-007) ───────────────────────────────────────────

// handleGetScore GET /api/v1/scores/{id} — 단건 조회 (200 / 404 / 400)
func (h *ScoreHandler) handleGetScore(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseScoreID(w, r)
	if !ok {
		return
	}
	tx, err := h.store.BeginScoreTx(r.Context())
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }() //nolint:errcheck // 읽기 전용 — 항상 rollback
	s, err := tx.GetScoreByID(r.Context(), id)
	if err != nil {
		h.writeStoreErr(w, err)
		return
	}
	writeScoreJSON(w, http.StatusOK, toScoreResponse(s))
}

// handleListScores GET /api/v1/scores — 목록 (filter + offset/limit 슬라이싱 + empty)
func (h *ScoreHandler) handleListScores(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	evalItem := strings.TrimSpace(q.Get("evaluation_item_id"))
	if evalItem == "" {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "evaluation_item_id는 필수입니다", "evaluation_item_id")
		return
	}
	tx, err := h.store.BeginScoreTx(r.Context())
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }() //nolint:errcheck // 읽기 전용

	all, err := tx.GetScoresByEvaluationItem(r.Context(), evalItem)
	if err != nil {
		h.writeStoreErr(w, err)
		return
	}

	// level/status 핸들러 필터 (#3 — store offset/limit 미지원)
	levelF := q.Get("level")
	statusF := q.Get("status")
	filtered := make([]scoreResponse, 0, len(all))
	for _, s := range all {
		if levelF != "" && s.Level != levelF {
			continue
		}
		if statusF != "" && s.Status != statusF {
			continue
		}
		filtered = append(filtered, toScoreResponse(s))
	}

	// offset/limit 메모리 슬라이싱 (clamp 후)
	limit, offset := clampPagination(q.Get("limit"), q.Get("offset"))
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := filtered[offset:end]
	if page == nil {
		page = []scoreResponse{}
	}

	writeScoreJSON(w, http.StatusOK, map[string]any{"scores": page, "count": len(page)})
}

// handleRollup GET /api/v1/scores/rollup — 가중 롤업 (numeric float64 미경유 정확 십진)
func (h *ScoreHandler) handleRollup(w http.ResponseWriter, r *http.Request) {
	evalItem := strings.TrimSpace(r.URL.Query().Get("evaluation_item_id"))
	if evalItem == "" {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "evaluation_item_id는 필수입니다", "evaluation_item_id")
		return
	}
	tx, err := h.store.BeginScoreTx(r.Context())
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }() //nolint:errcheck // 읽기 전용

	sum, err := tx.SumWeightedByEvaluationItem(r.Context(), evalItem)
	if err != nil {
		h.writeStoreErr(w, err)
		return
	}
	// pgtype.Numeric.Value()는 text-format 십진 문자열 반환 — float64 round-trip 없음 (SEC-03)
	decimal, valErr := sum.Value()
	if valErr != nil {
		h.logger.Error("numeric 직렬화 실패", zap.Error(valErr))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "집계 결과 직렬화 실패", "")
		return
	}
	// Value()는 Valid=true면 십진 문자열, Valid=false면 nil 반환 (float64 미경유).
	// 비-문자열(nil 등)은 빈 십진으로 안전 표면화 — comma-ok bool은 의도적 무시.
	decStr := ""
	if s, isStr := decimal.(string); isStr {
		decStr = s
	}
	writeScoreJSON(w, http.StatusOK, map[string]any{
		"evaluation_item_id": evalItem,
		"weighted_sum":       decStr,
	})
}

// handleGrade GET /api/v1/scores/grade — 등급 결정 (200 / 404 unavailable / 400)
func (h *ScoreHandler) handleGrade(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	scope := strings.TrimSpace(q.Get("scope"))
	if scope == "" {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "scope는 필수입니다", "scope")
		return
	}
	score, perr := strconv.ParseFloat(q.Get("score"), 64)
	if perr != nil {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "score는 숫자여야 합니다", "score")
		return
	}
	tx, err := h.store.BeginScoreTx(r.Context())
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }() //nolint:errcheck // 읽기 전용

	grade, err := tx.DetermineGrade(r.Context(), scope, score)
	if err != nil {
		h.writeStoreErr(w, err)
		return
	}
	writeScoreJSON(w, http.StatusOK, map[string]any{
		"scope": scope, "score": score, "grade": grade,
	})
}

// ── 변경 핸들러 (T-008/T-009/T-010) ───────────────────────────────────────────

// validateCreateBody pre-TX 입력 검증 (TX 미진입, row 0건 보장).
// 반환: (errField, errMsg) — errField=="" 이면 통과
func validateCreateBody(b *scoreCreateBody) (string, string) {
	switch {
	case strings.TrimSpace(b.EvaluationItemID) == "":
		return "evaluation_item_id", "evaluation_item_id는 필수입니다"
	case len(b.EvaluationItemID) > maxScoreEvalItemIDLen:
		return "evaluation_item_id", "evaluation_item_id가 64자를 초과합니다"
	case b.ScoreValue == nil:
		return "score_value", "score_value는 필수이며 숫자여야 합니다"
	}
	if b.EvidenceID != "" {
		if _, err := uuid.Parse(b.EvidenceID); err != nil {
			return "evidence_id", "evidence_id가 유효한 UUID가 아닙니다"
		}
	}
	return "", ""
}

// handleCreateScore POST /api/v1/scores — 생성 (201 + pre-TX 검증 400)
//
// @MX:WARN: [AUTO] BeginScoreTx 이후 Commit 전 early-return 시 orphan 점수 행 누출
// @MX:REASON: committed-defer Rollback이 BeginScoreTx 직후 즉시 등록되어야 함 — 순서 변경 금지 (evidence_handlers.go:348-353 선례, T-012)
func (h *ScoreHandler) handleCreateScore(w http.ResponseWriter, r *http.Request) {
	if !h.guardScoreWrite(w, r) {
		return
	}
	b, ok := h.decodeScoreBody(w, r)
	if !ok {
		return
	}
	if errField, errMsg := validateCreateBody(b); errField != "" {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", errMsg, errField)
		return
	}

	var evidenceID *uuid.UUID
	if b.EvidenceID != "" {
		eid := uuid.MustParse(b.EvidenceID) // validateCreateBody에서 파싱 성공 확인됨
		evidenceID = &eid
	}

	tx, err := h.store.BeginScoreTx(r.Context())
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(r.Context()) //nolint:errcheck // BeginScoreTx 직후 즉시 등록
		}
	}()

	newID, err := tx.InsertScore(r.Context(), b.EvaluationItemID, evidenceID, b.Level, *b.ScoreValue, b.Weight, b.Metadata)
	if err != nil {
		h.writeStoreErr(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		h.logger.Error("Commit 실패 — 롤백", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "점수 저장 실패", "")
		return
	}
	committed = true
	writeScoreJSON(w, http.StatusCreated, map[string]any{"score_id": newID.String(), "status": "DRAFT"})
}

// handleUpdateScore PUT /api/v1/scores/{id} — 수정 (200 / 409 immutable / 400)
//
// @MX:WARN: [AUTO] BeginScoreTx 이후 Commit 전 early-return 시 부분커밋 위험
// @MX:REASON: committed-defer Rollback BeginScoreTx 직후 즉시 등록 — 순서 변경 금지 (T-012)
func (h *ScoreHandler) handleUpdateScore(w http.ResponseWriter, r *http.Request) {
	if !h.guardScoreWrite(w, r) {
		return
	}
	id, ok := h.parseScoreID(w, r)
	if !ok {
		return
	}
	b, ok := h.decodeScoreBody(w, r)
	if !ok {
		return
	}

	tx, err := h.store.BeginScoreTx(r.Context())
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(r.Context()) //nolint:errcheck
		}
	}()

	upd := store.ScoreUpdate{ScoreValue: b.ScoreValue, Weight: b.Weight}
	if b.Metadata != nil {
		m := b.Metadata
		upd.Metadata = &m
	}
	if err = tx.UpdateScore(r.Context(), id, upd); err != nil {
		h.writeStoreErr(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		h.logger.Error("Commit 실패 — 롤백", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "점수 수정 실패", "")
		return
	}
	committed = true
	writeScoreJSON(w, http.StatusOK, map[string]any{"score_id": id.String()})
}

// handleSupersedeScore POST /api/v1/scores/{id}/supersede — CONFIRMED 정정 (201 / 409×2)
//
// @MX:WARN: [AUTO] BeginScoreTx 이후 Commit 전 early-return 시 부분커밋 위험
// @MX:REASON: committed-defer Rollback BeginScoreTx 직후 즉시 등록 — 순서 변경 금지 (T-012)
func (h *ScoreHandler) handleSupersedeScore(w http.ResponseWriter, r *http.Request) {
	if !h.guardScoreWrite(w, r) {
		return
	}
	oldID, ok := h.parseScoreID(w, r)
	if !ok {
		return
	}
	b, ok := h.decodeScoreBody(w, r)
	if !ok {
		return
	}
	if b.ScoreValue == nil {
		h.writeScoreErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "score_value는 필수이며 숫자여야 합니다", "score_value")
		return
	}

	tx, err := h.store.BeginScoreTx(r.Context())
	if err != nil {
		h.logger.Error("BeginScoreTx 실패", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(r.Context()) //nolint:errcheck
		}
	}()

	newID, err := tx.SupersedeAndReplaceScore(r.Context(), oldID, *b.ScoreValue, b.Weight, b.Metadata)
	if err != nil {
		h.writeStoreErr(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		h.logger.Error("Commit 실패 — 롤백", zap.Error(err))
		h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "점수 정정 실패", "")
		return
	}
	committed = true
	writeScoreJSON(w, http.StatusCreated, map[string]any{
		"score_id": newID.String(), "superseded_id": oldID.String(),
	})
}
