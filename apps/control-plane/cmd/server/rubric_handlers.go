// rubric_handlers.go — 등급 rubric REST API HTTP 핸들러 (SPEC-AX-RUBRIC-001 Phase C GREEN)
//
// 라우트(ServeMux Go1.22+, 최장일치 — strategy.md §3.3):
//
//	POST /api/v1/rubrics/{id}/clone-new-version  (admin only, OPEN #1)
//	POST /api/v1/rubrics/{id}/apply              (모든 인증, cross-store 2-TX, OPEN #6 read-only)
//	POST /api/v1/rubrics/{id}/criteria           (admin only, cross-store EvalItem 검증 + OPEN #3 weight sum)
//	POST /api/v1/rubrics/{id}/bands              (admin only, OPEN #4 dual defense)
//	POST /api/v1/rubrics/{id}/archive            (admin only, OPEN #7 archive_reason 필수)
//	PUT  /api/v1/rubrics/{id}                    (admin only, OPEN #2 + #5 active 직접 편집 허용)
//	GET  /api/v1/rubrics/{id}                    (모든 인증)
//	GET  /api/v1/rubrics                         (모든 인증)
//	POST /api/v1/rubrics                         (admin only)
//
// 본 SPEC은 SPEC-AX-SCORE-001(store) · SPEC-AX-EVAL-ITEM-001(cross-store) ·
// SPEC-AX-AUTH-003(ABAC) · SPEC-AX-REPORT-001(2-TX 선례) · SPEC-AX-REVIEW-001(dual defense 선례)의
// 순수 consumer다. consumer-only [HARD]: store/audit/auth/score_handlers/report_handlers/
// review_handlers 0-diff, 자체 audit 0 (store 전담 — OPEN #6 read-only는 0건).
//
// §A.4 Handler-local ABAC (frozen rbac.go 0-diff):
//
//	mutations(create/update/archive/criteria/bands/clone) = RoleAdmin only
//	조회/apply = 모든 인증 사용자 (게이트 없음)
//	auth-disabled = 모든 엔드포인트 투과 (cli-anonymous, UBI-003)
//
// §D1 iter2 lesson pre-applied (REVIEW-001 선례 정확 미러):
//   - mutation 6 핸들러는 `resolveCreatedBy(r)`로 actor 추출 후 store TX 메서드의 userID 파라미터로 전파.
//   - 효과: rubrics.created_by/updated_by + audit_logs.user_id 일관 영속화 (UBI-003).
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// 핸들러 검증/페이지네이션 상수 — review_handlers.go:46-49 / score_handlers.go:31-35 미러
const (
	maxRubricListLimit     = 500
	defaultRubricListLimit = 50
)

// SQLSTATE 코드 — OPEN #2 partial unique idx + OPEN #4 EXCLUSION violation race 매핑용
const (
	sqlstateUniqueViolation    = "23505" // OPEN #2 race — rubrics_active_unique_idx
	sqlstateExclusionViolation = "23P01" // OPEN #4 race — rubric_bands_no_overlap
)

// RubricHandler 등급 rubric REST 엔드포인트 핸들러.
// 3 store 의존(rubricStore + evalItemStore + scoreStore) — strategy.md §4 cross-store handler-compose.
// recorder 미주입 — 감사는 store 계층(RecordRubric*)이 동일 TX로 전담 (UBI-002).
// 필드 순서(govet fieldalignment 정렬): 인터페이스(16B) × 3 → 포인터(8B).
//
// @MX:ANCHOR: [AUTO] 등급 rubric REST 진입점 — 핸들러 단위 테스트 + 서버 마운트 + Routes() 3곳 이상에서 사용
// @MX:REASON: 등급 rubric 단일 HTTP 계약 (SPEC-AX-RUBRIC-001, 9 엔드포인트)
type RubricHandler struct {
	rubricStore   store.RubricStore
	evalItemStore store.EvalItemStore
	scoreStore    store.ScoreStore
	logger        *zap.Logger
}

// NewRubricHandler 등급 rubric 핸들러를 생성 (3 store 주입, recorder 없음 — store 전담).
// server.go에서 NewRubricHandler(pgStore, pgStore, pgStore, logger) — PgWorkflowStore가
// RubricStore + EvalItemStore + ScoreStore 동시 구현.
func NewRubricHandler(rs store.RubricStore, es store.EvalItemStore, ss store.ScoreStore, logger *zap.Logger) *RubricHandler {
	return &RubricHandler{rubricStore: rs, evalItemStore: es, scoreStore: ss, logger: logger}
}

// Routes 등급 rubric 라우트 9개 등록한 http.Handler 반환.
// ServeMux Go1.22+ 최장일치: 구체 경로(/clone-new-version, /apply, /criteria, /bands, /archive)
// 가 /{id} PUT/GET보다 우선 매칭. strategy.md §3.3 정확 정합.
//
// @MX:ANCHOR: [AUTO] 9 라우트 등록 단일 지점 — server.go 마운트 + 핸들러 테스트에서 사용
// @MX:REASON: ServeMux 최장일치 우선순위 불변 계약 (구체 경로 우선)
func (h *RubricHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	// 구체 경로 (최장일치 우선) 먼저 등록 — strategy.md §3.3
	mux.HandleFunc("POST /api/v1/rubrics/{id}/clone-new-version", h.handleCloneNewVersion)
	mux.HandleFunc("POST /api/v1/rubrics/{id}/apply", h.handleApplyRubric)
	mux.HandleFunc("POST /api/v1/rubrics/{id}/criteria", h.handleAddCriterion)
	mux.HandleFunc("POST /api/v1/rubrics/{id}/bands", h.handleAddBand)
	mux.HandleFunc("POST /api/v1/rubrics/{id}/archive", h.handleArchiveRubric)
	mux.HandleFunc("PUT /api/v1/rubrics/{id}", h.handleUpdateRubric)
	mux.HandleFunc("GET /api/v1/rubrics/{id}", h.handleGetRubric)
	mux.HandleFunc("GET /api/v1/rubrics", h.handleListRubrics)
	mux.HandleFunc("POST /api/v1/rubrics", h.handleCreateRubric)
	return mux
}

// ── 표준 JSON/에러 헬퍼 (review_handlers.go:100-118 미러) ────────────────────

// rubricErrorBody 에러 응답
type rubricErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

// writeRubricJSON Content-Type 설정 후 JSON 직렬화
func writeRubricJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // 헤더 전송 후라 로깅 불가
}

// writeRubricErr 표준 에러 본문 + INFO 로그 (거부는 INFO 레벨)
func (h *RubricHandler) writeRubricErr(w http.ResponseWriter, code int, errCode, msg, field string) {
	var body rubricErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	body.Error.Field = field
	h.logger.Info("등급 rubric 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("field", field),
	)
	writeRubricJSON(w, code, body)
}

// ── 에러 매핑 (errors.go ErrRubric* 7 sentinels + cross-store ErrEvalItemNotFound + SQLSTATE) ──

// mapRubricStoreErr store 센티넬을 HTTP status + 에러코드 + 한국어 메시지로 결정적 매핑.
// SQLSTATE 23505 (rubrics_active_unique_idx OPEN #2 race) → 409.
// SQLSTATE 23P01 (rubric_bands_no_overlap OPEN #4 race) → 400.
//
// @MX:WARN: [AUTO] 센티넬→HTTP 매핑 누락 시 client 에러가 500으로 오분류된다
// @MX:REASON: errors.go ErrRubric* 7 + cross-store ErrEvalItemNotFound + SQLSTATE 분기 전수 매핑
func mapRubricStoreErr(err error) (int, string, string) {
	// SQLSTATE 분기 — OPEN #2/#4 race race window DB-layer 결정적 매핑
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case sqlstateUniqueViolation:
			// OPEN #2 race — rubrics_active_unique_idx (name, scope) 동시 active 전이
			return http.StatusConflict, "CONFLICT", "동일 name/scope의 active rubric이 이미 존재합니다"
		case sqlstateExclusionViolation:
			// OPEN #4 race — rubric_bands_no_overlap EXCLUSION USING gist 동시 추가
			return http.StatusBadRequest, "INVALID_ARGUMENT", "등급 구간이 기존 구간과 겹칩니다"
		}
	}
	switch {
	case errors.Is(err, apperrors.ErrRubricNotFound):
		return http.StatusNotFound, "NOT_FOUND", "요청한 등급 rubric을 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrEvalItemNotFound):
		// §3.2 cross-store TX-1 결과 — 평가항목 미존재 시 404
		return http.StatusNotFound, "NOT_FOUND", "참조한 평가항목을 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrScoreNotFound):
		// apply cross-store TX-1 — 점수 미존재 시 404
		return http.StatusNotFound, "NOT_FOUND", "적용 대상 점수를 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrRubricInvalidInput),
		errors.Is(err, apperrors.ErrRubricWeightOutOfBounds):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "rubric 입력이 유효하지 않습니다"
	case errors.Is(err, apperrors.ErrRubricBandOverlap):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "등급 구간이 기존 구간과 겹칩니다"
	case errors.Is(err, apperrors.ErrRubricInvalidStatus):
		return http.StatusConflict, "CONFLICT", "허용되지 않은 rubric 상태 전이입니다"
	case errors.Is(err, apperrors.ErrRubricArchived):
		// archived terminal mutation — 409 (REQ-RUBRIC-003-S1 + AC E4)
		return http.StatusConflict, "CONFLICT", "archived rubric은 수정할 수 없습니다"
	case errors.Is(err, apperrors.ErrRubricAuditWriteFailed):
		return http.StatusInternalServerError, "INTERNAL", "rubric 처리 중 오류가 발생했습니다"
	default:
		return http.StatusInternalServerError, "INTERNAL", "rubric 처리 중 오류가 발생했습니다"
	}
}

// writeRubricStoreErr store 에러를 매핑하여 표준 본문으로 응답 (500은 ERROR 로그)
func (h *RubricHandler) writeRubricStoreErr(w http.ResponseWriter, err error) {
	code, errCode, msg := mapRubricStoreErr(err)
	if code == http.StatusInternalServerError {
		h.logger.Error("등급 rubric store 오류", zap.Error(err))
	}
	h.writeRubricErr(w, code, errCode, msg, "")
}

// ── 페이지네이션 clamp (score_handlers.go:144-157 미러) ───────────────────────

// clampRubricPagination limit 50 기본, 500 max, offset 0 min.
// review_handlers.go의 parseIntDefault 재사용 (동일 main 패키지).
func clampRubricPagination(rawLimit, rawOffset string) (int, int) {
	limit := defaultRubricListLimit
	if v := parseIntDefault(rawLimit, 0); v > 0 {
		limit = v
	}
	if limit > maxRubricListLimit {
		limit = maxRubricListLimit
	}
	offset := 0
	if v := parseIntDefault(rawOffset, 0); v > 0 {
		offset = v
	}
	return limit, offset
}

// ── ABAC 핸들러-로컬 게이트 (§A.4, score_handlers.go:161-190 / review_handlers.go:210-249 미러) ──

// requireRubricAdminRole 모든 mutation 엔드포인트는 admin only 허용 (frozen rbac.go 0-diff)
func requireRubricAdminRole(scope string) bool {
	for _, r := range auth.ParseRolesFromScope(scope) {
		if r == auth.RoleAdmin {
			return true
		}
	}
	return false
}

// guardRubricAdmin mutation 진입 직전 admin-only 게이트.
// auth context 부재(auth-disabled Walking Skeleton) → 투과(true, UBI-003).
// auth 활성 시 RoleAdmin 아니면 403 ABAC_CONDITION_DENIED 응답 후 false.
//
// @MX:NOTE: [AUTO] abac.go narrowing-only 동형 — ABAC는 미인가만 거부, allow 부여 없음
func (h *RubricHandler) guardRubricAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return true // auth-disabled 투과 (cli-anonymous, UBI-003)
	}
	if requireRubricAdminRole(strings.Join(u.Scopes, " ")) {
		return true
	}
	h.writeRubricErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
		"관리자 권한이 없는 사용자입니다", "")
	return false
}

// ── 요청/응답 DTO ─────────────────────────────────────────────────────────────

// rubricCreateBody POST /api/v1/rubrics 요청 본문
type rubricCreateBody struct {
	Metadata map[string]any `json:"metadata"`
	Name     string         `json:"name"`
	Scope    string         `json:"scope"`
}

// rubricUpdateBody PUT /api/v1/rubrics/{id} 요청 본문
type rubricUpdateBody struct {
	Metadata map[string]any `json:"metadata"`
	Name     string         `json:"name"`
	Scope    string         `json:"scope"`
	Status   string         `json:"status"`
}

// rubricArchiveBody POST /api/v1/rubrics/{id}/archive 요청 본문 (OPEN #7)
type rubricArchiveBody struct {
	ArchiveReason string `json:"archive_reason"`
}

// rubricCriterionBody POST /api/v1/rubrics/{id}/criteria 요청 본문
type rubricCriterionBody struct {
	EvaluationItemID string  `json:"evaluation_item_id"`
	Weight           float64 `json:"weight"`
}

// rubricBandBody POST /api/v1/rubrics/{id}/bands 요청 본문
type rubricBandBody struct {
	Letter   string  `json:"letter"`
	MinScore float64 `json:"min_score"`
	MaxScore float64 `json:"max_score"`
}

// rubricApplyBody POST /api/v1/rubrics/{id}/apply 요청 본문
type rubricApplyBody struct {
	ScoreID string `json:"score_id"`
}

// rubricResponse 단건 + 목록 응답에 사용하는 직렬화 DTO.
// criteria/bands는 GET /{id}에서만 임베드 (목록은 메타데이터만 반환).
type rubricResponse struct {
	Metadata      map[string]any  `json:"metadata,omitempty"`
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Scope         string          `json:"scope,omitempty"`
	Status        string          `json:"status"`
	ArchiveReason string          `json:"archive_reason,omitempty"`
	CreatedAt     string          `json:"created_at"`
	CreatedBy     string          `json:"created_by"`
	UpdatedAt     string          `json:"updated_at"`
	UpdatedBy     string          `json:"updated_by,omitempty"`
	Criteria      []criterionJSON `json:"criteria,omitempty"`
	Bands         []bandJSON      `json:"bands,omitempty"`
	Version       int             `json:"version"`
}

// criterionJSON RubricCriterion 직렬화
type criterionJSON struct {
	ID               string  `json:"id"`
	RubricID         string  `json:"rubric_id"`
	EvaluationItemID string  `json:"evaluation_item_id"`
	CreatedAt        string  `json:"created_at"`
	Weight           float64 `json:"weight"`
}

// bandJSON RubricBand 직렬화
type bandJSON struct {
	ID        string  `json:"id"`
	RubricID  string  `json:"rubric_id"`
	Letter    string  `json:"letter"`
	CreatedAt string  `json:"created_at"`
	MinScore  float64 `json:"min_score"`
	MaxScore  float64 `json:"max_score"`
}

const rubricTimeFormat = "2006-01-02T15:04:05Z07:00"

// toRubricResponse store.Rubric → rubricResponse 변환 (criteria/bands 미포함)
func toRubricResponse(r *store.Rubric) rubricResponse {
	return rubricResponse{
		ID:            r.ID.String(),
		Name:          r.Name,
		Version:       r.Version,
		Scope:         r.Scope,
		Status:        r.Status,
		ArchiveReason: r.ArchiveReason,
		Metadata:      r.Metadata,
		CreatedAt:     r.CreatedAt.UTC().Format(rubricTimeFormat),
		CreatedBy:     r.CreatedBy,
		UpdatedAt:     r.UpdatedAt.UTC().Format(rubricTimeFormat),
		UpdatedBy:     r.UpdatedBy,
	}
}

// toCriterionJSON store.RubricCriterion → criterionJSON
func toCriterionJSON(c *store.RubricCriterion) criterionJSON {
	return criterionJSON{
		ID:               c.ID.String(),
		RubricID:         c.RubricID.String(),
		EvaluationItemID: c.EvaluationItemID.String(),
		Weight:           c.Weight,
		CreatedAt:        c.CreatedAt.UTC().Format(rubricTimeFormat),
	}
}

// toBandJSON store.RubricBand → bandJSON
func toBandJSON(b *store.RubricBand) bandJSON {
	return bandJSON{
		ID:        b.ID.String(),
		RubricID:  b.RubricID.String(),
		Letter:    b.Letter,
		MinScore:  b.MinScore,
		MaxScore:  b.MaxScore,
		CreatedAt: b.CreatedAt.UTC().Format(rubricTimeFormat),
	}
}

// parseRubricID path {id} → UUID 파싱 (malformed → 400, store 미진입)
func (h *RubricHandler) parseRubricID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "유효하지 않은 rubric ID 형식입니다", "id")
		return uuid.Nil, false
	}
	return id, true
}

// ── 9 핸들러 메서드 ───────────────────────────────────────────────────────────

// handleCreateRubric POST /api/v1/rubrics (admin only) — REQ-RUBRIC-002-E1
func (h *RubricHandler) handleCreateRubric(w http.ResponseWriter, r *http.Request) {
	if !h.guardRubricAdmin(w, r) {
		return
	}
	var body rubricCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "rubric name은 필수입니다", "name")
		return
	}

	ctx := r.Context()
	tx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck // BeginTx 직후 즉시 등록
		}
	}()

	actor := resolveCreatedBy(r) // D1 iter2 lesson Point 3 — review_handlers.go:396 재사용
	newID, err := tx.InsertRubric(ctx, body.Name, body.Scope, body.Metadata, actor)
	if err != nil {
		h.writeRubricStoreErr(w, err)
		return
	}
	if cmErr := tx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "rubric 저장 실패", "")
		return
	}
	committed = true

	writeRubricJSON(w, http.StatusCreated, map[string]any{
		"id":         newID.String(),
		"name":       body.Name,
		"scope":      body.Scope,
		"status":     "draft",
		"version":    1,
		"created_by": actor,
		"updated_by": actor,
	})
}

// handleGetRubric GET /api/v1/rubrics/{id} (모든 인증) — REQ-RUBRIC-002-E2
// rubric 본문에 criteria/bands 임베드 (단일 read-only TX).
func (h *RubricHandler) handleGetRubric(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseRubricID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // 읽기 전용

	r1, gerr := tx.GetRubricByID(ctx, id)
	if gerr != nil {
		h.writeRubricStoreErr(w, gerr)
		return
	}
	criteria, cErr := tx.GetCriteriaByRubric(ctx, id)
	if cErr != nil {
		h.writeRubricStoreErr(w, cErr)
		return
	}
	bands, bErr := tx.GetBandsByRubric(ctx, id)
	if bErr != nil {
		h.writeRubricStoreErr(w, bErr)
		return
	}
	resp := toRubricResponse(r1)
	resp.Criteria = make([]criterionJSON, 0, len(criteria))
	for _, c := range criteria {
		resp.Criteria = append(resp.Criteria, toCriterionJSON(c))
	}
	resp.Bands = make([]bandJSON, 0, len(bands))
	for _, b := range bands {
		resp.Bands = append(resp.Bands, toBandJSON(b))
	}
	writeRubricJSON(w, http.StatusOK, resp)
}

// handleListRubrics GET /api/v1/rubrics?status=&scope=&limit=&offset= (모든 인증) — REQ-RUBRIC-002-E3
func (h *RubricHandler) handleListRubrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statusFilter := strings.TrimSpace(q.Get("status"))
	scopeFilter := strings.TrimSpace(q.Get("scope"))
	limit, offset := clampRubricPagination(q.Get("limit"), q.Get("offset"))

	ctx := r.Context()
	tx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // 읽기 전용

	items, lerr := tx.ListRubrics(ctx, statusFilter, scopeFilter, limit, offset)
	if lerr != nil {
		h.writeRubricStoreErr(w, lerr)
		return
	}
	// AC-RUBRIC-002-3: total은 limit/offset 적용 전 전체 카운트
	total, cErr := tx.CountRubrics(ctx, statusFilter, scopeFilter)
	if cErr != nil {
		h.writeRubricStoreErr(w, cErr)
		return
	}
	resp := make([]rubricResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, toRubricResponse(it))
	}
	writeRubricJSON(w, http.StatusOK, map[string]any{
		"items": resp,
		"total": total,
	})
}

// handleUpdateRubric PUT /api/v1/rubrics/{id} (admin only, OPEN #2 + #5) — REQ-RUBRIC-003-E1
// OPEN #2 dual defense Layer 1: draft→active 전이 시 (scope, status='active') 중복 사전 검사.
// OPEN #5: active 직접 편집 허용 — UpdateRubric가 status no-op + 메타 변경 통합 담당.
func (h *RubricHandler) handleUpdateRubric(w http.ResponseWriter, r *http.Request) {
	if !h.guardRubricAdmin(w, r) {
		return
	}
	id, ok := h.parseRubricID(w, r)
	if !ok {
		return
	}
	var body rubricUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}

	ctx := r.Context()
	tx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck // BeginTx 직후 즉시 등록
		}
	}()

	// 현재 상태 조회 — OPEN #2 사전 검사 분기 결정
	cur, gerr := tx.GetRubricByID(ctx, id)
	if gerr != nil {
		h.writeRubricStoreErr(w, gerr)
		return
	}

	// OPEN #2 Layer 1: draft → active 전이 시 동일 (scope, status='active') 중복 차단.
	// scope 일치 기준 단순 사전 검사 (PoC 단순화). 정확한 (name, scope) match는
	// DB partial unique idx (Layer 2)가 race window까지 결정적 차단.
	if cur.Status == "draft" && body.Status == "active" {
		dupCount, cErr := tx.CountRubrics(ctx, "active", body.Scope)
		if cErr != nil {
			h.writeRubricStoreErr(w, cErr)
			return
		}
		if dupCount > 0 {
			h.writeRubricErr(w, http.StatusConflict, "CONFLICT",
				"동일 name/scope의 active rubric이 이미 존재합니다", "status")
			return
		}
	}

	actor := resolveCreatedBy(r) // D1 iter2 lesson Point 3
	if uerr := tx.UpdateRubric(ctx, id, body.Name, body.Scope, body.Status, body.Metadata, actor); uerr != nil {
		h.writeRubricStoreErr(w, uerr)
		return
	}

	// 갱신된 entity 재조회 (응답용)
	updated, ugErr := tx.GetRubricByID(ctx, id)
	if ugErr != nil {
		h.writeRubricStoreErr(w, ugErr)
		return
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "rubric 갱신 실패", "")
		return
	}
	committed = true

	writeRubricJSON(w, http.StatusOK, toRubricResponse(updated))
}

// handleArchiveRubric POST /api/v1/rubrics/{id}/archive (admin only, OPEN #7) — REQ-RUBRIC-003-E2
// OPEN #7 dual defense Layer 1: archive_reason blank/missing 시 400 한국어 메시지.
func (h *RubricHandler) handleArchiveRubric(w http.ResponseWriter, r *http.Request) {
	if !h.guardRubricAdmin(w, r) {
		return
	}
	id, ok := h.parseRubricID(w, r)
	if !ok {
		return
	}
	var body rubricArchiveBody
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
			return
		}
	}
	// OPEN #7 Layer 1: handler-side archive_reason 검증
	if strings.TrimSpace(body.ArchiveReason) == "" {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"archive_reason은 필수입니다", "archive_reason")
		return
	}

	ctx := r.Context()
	tx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck
		}
	}()

	actor := resolveCreatedBy(r) // D1 iter2 lesson Point 3
	if aerr := tx.ArchiveRubric(ctx, id, body.ArchiveReason, actor); aerr != nil {
		h.writeRubricStoreErr(w, aerr)
		return
	}

	updated, gerr := tx.GetRubricByID(ctx, id)
	if gerr != nil {
		h.writeRubricStoreErr(w, gerr)
		return
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "rubric archive 실패", "")
		return
	}
	committed = true

	writeRubricJSON(w, http.StatusOK, toRubricResponse(updated))
}

// handleAddCriterion POST /api/v1/rubrics/{id}/criteria (admin only, OPEN #3) — REQ-RUBRIC-001-E2
// cross-store 2-TX handler-compose:
//
//	TX-1 (read-only): evalItemStore.BeginEvalItemTx → GetEvalItemByID → Rollback (존재 검증)
//	TX-2 (write): rubricStore.BeginRubricTx → GetCriteriaByRubric (sum check) → AddCriterion → Commit
//
// @MX:WARN: [AUTO] cross-store 2-TX — defer Rollback 누락 시 커넥션 누수
// @MX:REASON: handler-compose — TX-1 EvalItem 존재, TX-2 weight sum + criterion INSERT
func (h *RubricHandler) handleAddCriterion(w http.ResponseWriter, r *http.Request) {
	if !h.guardRubricAdmin(w, r) {
		return
	}
	id, ok := h.parseRubricID(w, r)
	if !ok {
		return
	}
	var body rubricCriterionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	evalItemUUID, perr := uuid.Parse(strings.TrimSpace(body.EvaluationItemID))
	if perr != nil || evalItemUUID == uuid.Nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "유효하지 않은 평가항목 ID 형식입니다", "evaluation_item_id")
		return
	}

	ctx := r.Context()

	// ── TX-1 (read-only): cross-store EvalItem 존재 검증 ──
	evalTx, err := h.evalItemStore.BeginEvalItemTx(ctx)
	if err != nil {
		h.logger.Error("BeginEvalItemTx 실패 (cross-store TX-1)", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "평가항목 검증 트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = evalTx.Rollback(ctx) }() //nolint:errcheck // 읽기 전용

	if _, geErr := evalTx.GetEvalItemByID(ctx, evalItemUUID.String()); geErr != nil {
		h.writeRubricStoreErr(w, geErr)
		return
	}

	// ── TX-2 (write): rubric criterion INSERT ──
	rTx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패 (cross-store TX-2)", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "rubric 트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = rTx.Rollback(ctx) //nolint:errcheck
		}
	}()

	// OPEN #3 Layer 1: handler-side weight sum 사전 검사 (sum + new > 1.0 거부)
	existing, ceErr := rTx.GetCriteriaByRubric(ctx, id)
	if ceErr != nil {
		h.writeRubricStoreErr(w, ceErr)
		return
	}
	var sumExisting float64
	for _, c := range existing {
		sumExisting += c.Weight
	}
	if sumExisting+body.Weight > 1.0 {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"가중치 합이 1.0을 초과합니다", "weight")
		return
	}

	actor := resolveCreatedBy(r) // D1 iter2 lesson Point 3
	newID, aerr := rTx.AddCriterion(ctx, id, evalItemUUID, body.Weight, actor)
	if aerr != nil {
		h.writeRubricStoreErr(w, aerr)
		return
	}

	if cmErr := rTx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "criterion 저장 실패", "")
		return
	}
	committed = true

	writeRubricJSON(w, http.StatusCreated, map[string]any{
		"id":                 newID.String(),
		"rubric_id":          id.String(),
		"evaluation_item_id": evalItemUUID.String(),
		"weight":             body.Weight,
	})
}

// handleAddBand POST /api/v1/rubrics/{id}/bands (admin only, OPEN #4) — REQ-RUBRIC-001-E3
// OPEN #4 dual defense Layer 1: handler-side overlap 사전 검사 (Layer 2는 DB EXCLUSION).
func (h *RubricHandler) handleAddBand(w http.ResponseWriter, r *http.Request) {
	if !h.guardRubricAdmin(w, r) {
		return
	}
	id, ok := h.parseRubricID(w, r)
	if !ok {
		return
	}
	var body rubricBandBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	if strings.TrimSpace(body.Letter) == "" {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "letter는 필수입니다", "letter")
		return
	}

	ctx := r.Context()
	tx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck
		}
	}()

	// OPEN #4 Layer 1: handler-side overlap pre-check (numrange '[]' inclusive)
	existing, beErr := tx.GetBandsByRubric(ctx, id)
	if beErr != nil {
		h.writeRubricStoreErr(w, beErr)
		return
	}
	for _, b := range existing {
		// inclusive 양 끝 — min <= existing.max AND max >= existing.min 시 겹침
		if body.MinScore <= b.MaxScore && body.MaxScore >= b.MinScore {
			h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
				"등급 구간이 기존 구간과 겹칩니다", "min_score")
			return
		}
	}

	actor := resolveCreatedBy(r) // D1 iter2 lesson Point 3
	newID, aerr := tx.AddBand(ctx, id, body.Letter, body.MinScore, body.MaxScore, actor)
	if aerr != nil {
		// DB EXCLUSION violation (race) → mapRubricStoreErr가 SQLSTATE 23P01 → 400
		h.writeRubricStoreErr(w, aerr)
		return
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "band 저장 실패", "")
		return
	}
	committed = true

	writeRubricJSON(w, http.StatusCreated, map[string]any{
		"id":        newID.String(),
		"rubric_id": id.String(),
		"letter":    body.Letter,
		"min_score": body.MinScore,
		"max_score": body.MaxScore,
	})
}

// handleApplyRubric POST /api/v1/rubrics/{id}/apply (모든 인증, cross-store 2-TX, OPEN #6 read-only)
// — REQ-RUBRIC-004-E1/E2
//
// 시퀀스 (strategy.md §4.1):
//
//	TX-1 (read-only): scoreStore.BeginScoreTx → GetScoreByID → SumWeightedByEvaluationItem → Rollback
//	SEC-03 float64 1회 변환: pgtype.Numeric.Float64() (REPORT-001 D3-2 lesson 정합)
//	TX-2 (read-only): rubricStore.BeginRubricTx → ApplyRubric → Rollback (audit row 0건, OPEN #6)
//
// @MX:WARN: [AUTO] cross-store 2-TX — defer Rollback 누락 시 커넥션 누수
// @MX:REASON: §4.1 handler-compose — TX-1 score 합산 read-only, TX-2 rubric apply read-only (audit 0건)
func (h *RubricHandler) handleApplyRubric(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseRubricID(w, r)
	if !ok {
		return
	}
	var body rubricApplyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	scoreID, perr := uuid.Parse(strings.TrimSpace(body.ScoreID))
	if perr != nil || scoreID == uuid.Nil {
		h.writeRubricErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "유효하지 않은 점수 ID 형식입니다", "score_id")
		return
	}

	ctx := r.Context()

	// ── TX-1 (read-only): SCORE-001 cross-store 점수 + 가중합 ──
	sTx, err := h.scoreStore.BeginScoreTx(ctx)
	if err != nil {
		h.logger.Error("BeginScoreTx 실패 (cross-store TX-1)", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "점수 트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = sTx.Rollback(ctx) }() //nolint:errcheck // 읽기 전용 — 항상 rollback

	score, gErr := sTx.GetScoreByID(ctx, scoreID)
	if gErr != nil {
		h.writeRubricStoreErr(w, gErr)
		return
	}
	numericSum, sumErr := sTx.SumWeightedByEvaluationItem(ctx, score.EvaluationItemID)
	if sumErr != nil {
		h.writeRubricStoreErr(w, sumErr)
		return
	}
	_ = sTx.Rollback(ctx) //nolint:errcheck // 명시적 — TX-1 종료

	// SEC-03 float64 1회 변환 (REPORT-001 D3-2 lesson 정합 — silent fallback 금지)
	scoreF, fErr := numericSum.Float64Value()
	if fErr != nil || !scoreF.Valid {
		h.logger.Error("pgtype.Numeric Float64 변환 실패",
			zap.String("score_id", scoreID.String()),
			zap.Any("err", fErr),
		)
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "점수 값 변환 실패", "")
		return
	}
	scoreValue := scoreF.Float64

	// ── TX-2 (read-only): RUBRIC apply (OPEN #6 audit row 0건) ──
	rTx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패 (cross-store TX-2)", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "rubric 트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = rTx.Rollback(ctx) }() //nolint:errcheck // OPEN #6 read-only — Commit 없음

	letter, band, aErr := rTx.ApplyRubric(ctx, id, scoreValue)
	if aErr != nil {
		h.writeRubricStoreErr(w, aErr)
		return
	}

	resp := map[string]any{
		"rubric_id":   id.String(),
		"score_id":    scoreID.String(),
		"score_value": scoreValue,
		"letter":      letter,
	}
	if band != nil {
		resp["band"] = toBandJSON(band)
	}
	writeRubricJSON(w, http.StatusOK, resp)
}

// handleCloneNewVersion POST /api/v1/rubrics/{id}/clone-new-version (admin only, OPEN #1)
// — REQ-RUBRIC-003-E1 supersede 패턴 (REVIEW-001 sub-resource 선례 미러)
//
// 동작 (strategy.md §1.1 — 빈 rubric 신규 생성):
//
//	기존 rubric의 name/scope/metadata를 복사하여 새 draft rubric을 생성.
//	criteria/bands는 admin이 별도로 추가 (R-RUBRIC-008 clone 깊이 단순화).
//	version 자동 증분은 PoC 범위 외 — store.InsertRubric은 version=1 고정.
func (h *RubricHandler) handleCloneNewVersion(w http.ResponseWriter, r *http.Request) {
	if !h.guardRubricAdmin(w, r) {
		return
	}
	id, ok := h.parseRubricID(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	tx, err := h.rubricStore.BeginRubricTx(ctx)
	if err != nil {
		h.logger.Error("BeginRubricTx 실패", zap.Error(err))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck
		}
	}()

	src, gErr := tx.GetRubricByID(ctx, id)
	if gErr != nil {
		h.writeRubricStoreErr(w, gErr)
		return
	}
	// archived 원본은 clone 거부 (UBI-004 일관성)
	if src.Status == "archived" {
		h.writeRubricErr(w, http.StatusConflict, "CONFLICT",
			"archived rubric은 clone할 수 없습니다", "id")
		return
	}

	actor := resolveCreatedBy(r) // D1 iter2 lesson Point 3
	newID, iErr := tx.InsertRubric(ctx, src.Name, src.Scope, src.Metadata, actor)
	if iErr != nil {
		h.writeRubricStoreErr(w, iErr)
		return
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeRubricErr(w, http.StatusInternalServerError, "INTERNAL", "rubric clone 실패", "")
		return
	}
	committed = true

	writeRubricJSON(w, http.StatusCreated, map[string]any{
		"id":         newID.String(),
		"name":       src.Name,
		"scope":      src.Scope,
		"status":     "draft",
		"version":    1, // R-RUBRIC-008: PoC clone은 version=1 (store.InsertRubric 고정), 증분 deferred
		"source_id":  id.String(),
		"created_by": actor,
		"updated_by": actor,
	})
}
