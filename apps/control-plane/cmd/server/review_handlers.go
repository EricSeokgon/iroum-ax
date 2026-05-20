// review_handlers.go — 평가 검토 요청 제출/승인 워크플로우 HTTP API 핸들러 (SPEC-AX-REVIEW-001)
//
// 라우트(ServeMux Go1.22+, 최장일치):
//
//	POST /api/v1/reviews/{id}/assign-reviewer  (admin only)
//	POST /api/v1/reviews/{id}/approve          (admin only)
//	POST /api/v1/reviews/{id}/reject           (admin only, rejection_reason required)
//	GET  /api/v1/reviews/{id}                  (모든 인증 사용자)
//	GET  /api/v1/reviews                       (모든 인증 사용자)
//	POST /api/v1/reviews                       (analyst/admin, cross-store 2-TX score 검증)
//
// 본 SPEC은 SPEC-AX-SCORE-001(store)·SPEC-AX-AUTH-003(ABAC)·SPEC-AX-SCORE-API-001(핸들러 선례)
// ·SPEC-AX-REPORT-001(cross-store 2-TX 선례)의 순수 consumer다. consumer-only [HARD]:
// store/audit/auth/score_handlers.go/report_handlers.go 0-diff, 자체 audit 0 (store 전담).
//
// §A.3 Cross-store 2-TX (handleCreateReview):
//
//	TX-1: scoreStore.BeginScoreTx → GetScoreByID → Rollback (read-only, defer)
//	TX-2: reviewStore.BeginScoreReviewRequestTx → InsertScoreReviewRequest → Commit
//
// Race window는 SCORE-001 물리 삭제 0이라 결정적 不發生 (PoC 수용).
//
// §A.4 Handler-local ABAC (frozen rbac.go 0-diff):
//
//	제출(submit) = RoleAnalyst || RoleAdmin
//	할당/승인/반려 = RoleAdmin only
//	조회 = 모든 인증 사용자 (게이트 없음)
//	auth-disabled = 모든 엔드포인트 투과 (cli-anonymous)
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// 핸들러 검증/페이지네이션 상수 (score_handlers.go:31-35 미러)
const (
	maxReviewListLimit     = 500
	defaultReviewListLimit = 50
)

// ReviewHandler 평가 검토 요청 REST 엔드포인트 핸들러.
// reviewStore + scoreStore 두 store 별개 의존 (cross-store 2-TX — §A.3).
// recorder 미주입 — 감사는 store 계층(RecordScoreReviewRequest*)이 동일 TX로 전담 (UBI-002).
// 필드 순서: 인터페이스(16B) × 2 → 포인터(8B) (govet fieldalignment 정렬).
//
// @MX:ANCHOR: [AUTO] 평가 검토 REST 진입점 — 핸들러 단위 테스트 + 서버 마운트 + Routes() 3곳 이상에서 사용
// @MX:REASON: 평가 검토 단일 HTTP 계약 (SPEC-AX-REVIEW-001, 6 엔드포인트)
type ReviewHandler struct {
	reviewStore store.ScoreReviewRequestStore
	scoreStore  store.ScoreStore
	logger      *zap.Logger
}

// NewReviewHandler 평가 검토 핸들러를 생성 (2 store 주입, recorder 없음 — store 전담).
// server.go에서 NewReviewHandler(pgStore, pgStore, logger) — PgWorkflowStore가
// ScoreReviewRequestStore(pg_store.go BeginScoreReviewRequestTx) + ScoreStore(BeginScoreTx) 동시 구현.
func NewReviewHandler(rs store.ScoreReviewRequestStore, ss store.ScoreStore, logger *zap.Logger) *ReviewHandler {
	return &ReviewHandler{reviewStore: rs, scoreStore: ss, logger: logger}
}

// Routes 평가 검토 라우트 6개 등록한 http.Handler 반환.
// ServeMux Go1.22+ 최장일치: 구체 경로(/assign-reviewer, /approve, /reject)가 /{id}보다 우선.
//
// @MX:ANCHOR: [AUTO] 6 라우트 등록 단일 지점 — server.go 마운트 + 핸들러 테스트에서 사용
// @MX:REASON: ServeMux 최장일치 우선순위 불변 계약 (구체 경로 우선)
func (h *ReviewHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	// 구체 경로 (최장일치 우선) 먼저 등록
	mux.HandleFunc("POST /api/v1/reviews/{id}/assign-reviewer", h.handleAssignReviewer)
	mux.HandleFunc("POST /api/v1/reviews/{id}/approve", h.handleApprove)
	mux.HandleFunc("POST /api/v1/reviews/{id}/reject", h.handleReject)
	mux.HandleFunc("GET /api/v1/reviews/{id}", h.handleGetReview)
	mux.HandleFunc("GET /api/v1/reviews", h.handleListReviews)
	mux.HandleFunc("POST /api/v1/reviews", h.handleCreateReview)
	return mux
}

// ── 표준 JSON/에러 헬퍼 (score_handlers.go:74-101 미러) ────────────────────────

// reviewErrorBody 에러 응답
type reviewErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

// writeReviewJSON Content-Type 설정 후 JSON 직렬화
func writeReviewJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // 헤더 전송 후라 로깅 불가
}

// writeReviewErr 표준 에러 본문 + INFO 로그 (거부는 INFO 레벨, score_handlers.go:91 미러)
func (h *ReviewHandler) writeReviewErr(w http.ResponseWriter, code int, errCode, msg, field string) {
	var body reviewErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	body.Error.Field = field
	h.logger.Info("평가 검토 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("field", field),
	)
	writeReviewJSON(w, code, body)
}

// ── 에러 매핑 (AC-REVIEW-005-1, 7 sentinels) ───────────────────────────────────

// mapReviewStoreErr store 센티넬을 HTTP status + 에러코드 + 한국어 메시지로 결정적 매핑
//
// @MX:WARN: [AUTO] 센티넬→HTTP 매핑 누락 시 client 에러가 500으로 오분류된다
// @MX:REASON: errors.go ErrScoreReviewRequest* 6 센티넬 전수 매핑 + ScoreNotFound (cross-store) — 신규 시 갱신 필수
func mapReviewStoreErr(err error) (int, string, string) {
	switch {
	case errors.Is(err, apperrors.ErrScoreReviewRequestNotFound):
		return http.StatusNotFound, "NOT_FOUND", "요청한 평가 검토를 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrScoreNotFound):
		// §A.3 cross-store 2-TX TX-1 결과 — 점수 미존재 시 404
		return http.StatusNotFound, "NOT_FOUND", "검토 대상 점수를 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrScoreReviewRequestInvalidInput):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "평가 검토 입력이 유효하지 않습니다"
	case errors.Is(err, apperrors.ErrScoreReviewRequestInvalidStatus):
		return http.StatusConflict, "CONFLICT", "허용되지 않은 평가 검토 상태 전이입니다"
	case errors.Is(err, apperrors.ErrScoreReviewRequestNotSubmitted):
		return http.StatusConflict, "CONFLICT", "검토자 할당은 SUBMITTED 상태의 평가 검토에만 허용됩니다"
	case errors.Is(err, apperrors.ErrScoreReviewRequestNotUnderReview):
		return http.StatusConflict, "CONFLICT", "승인/반려는 UNDER_REVIEW 상태의 평가 검토에만 허용됩니다"
	case errors.Is(err, apperrors.ErrScoreReviewRequestAuditWriteFailed):
		return http.StatusInternalServerError, "INTERNAL", "평가 검토 처리 중 오류가 발생했습니다"
	default:
		return http.StatusInternalServerError, "INTERNAL", "평가 검토 처리 중 오류가 발생했습니다"
	}
}

// writeReviewStoreErr store 에러를 매핑하여 표준 본문으로 응답 (500은 ERROR 로그)
func (h *ReviewHandler) writeReviewStoreErr(w http.ResponseWriter, err error) {
	code, errCode, msg := mapReviewStoreErr(err)
	if code == http.StatusInternalServerError {
		h.logger.Error("평가 검토 store 오류", zap.Error(err))
	}
	h.writeReviewErr(w, code, errCode, msg, "")
}

// ── 페이지네이션 clamp (score_handlers.go:144-157 미러) ────────────────────────

// clampReviewPagination limit 50 기본, 500 max, offset 0 min
func clampReviewPagination(rawLimit, rawOffset string) (int, int) {
	limit := defaultReviewListLimit
	if v := parseIntDefault(rawLimit, 0); v > 0 {
		limit = v
	}
	if limit > maxReviewListLimit {
		limit = maxReviewListLimit
	}
	offset := 0
	if v := parseIntDefault(rawOffset, 0); v > 0 {
		offset = v
	}
	return limit, offset
}

// parseIntDefault strconv.Atoi의 헬퍼 (defaultValue로 graceful)
func parseIntDefault(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	var n int
	for i, c := range s {
		if i == 0 && c == '-' {
			continue
		}
		if c < '0' || c > '9' {
			return defaultValue
		}
		n = n*10 + int(c-'0')
	}
	if strings.HasPrefix(s, "-") {
		return -n
	}
	return n
}

// ── ABAC 핸들러-로컬 게이트 (§A.4, score_handlers.go:161-190 정확 미러) ─────────

// requireReviewSubmitRole 제출(POST /reviews)은 analyst 또는 admin 허용
// (score_handlers.go:164 requireScoreWriteRole 동형)
func requireReviewSubmitRole(scope string) bool {
	for _, r := range auth.ParseRolesFromScope(scope) {
		if r == auth.RoleAdmin || r == auth.RoleAnalyst {
			return true
		}
	}
	return false
}

// requireReviewAdminRole 검토자 할당/승인/반려는 admin only 허용
func requireReviewAdminRole(scope string) bool {
	for _, r := range auth.ParseRolesFromScope(scope) {
		if r == auth.RoleAdmin {
			return true
		}
	}
	return false
}

// guardReviewSubmit POST /api/v1/reviews 진입 직전 ABAC narrowing 게이트
// auth context 부재(auth-disabled Walking Skeleton) → 투과(true)
// auth 활성 시 RoleAdmin/RoleAnalyst 아니면 403 ABAC_CONDITION_DENIED 응답 후 false
//
// @MX:NOTE: [AUTO] abac.go narrowing-only 동형 — ABAC는 미인가만 거부, allow 부여 없음.
func (h *ReviewHandler) guardReviewSubmit(w http.ResponseWriter, r *http.Request) bool {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return true // auth-disabled 투과 (cli-anonymous, UBI-003)
	}
	if requireReviewSubmitRole(strings.Join(u.Scopes, " ")) {
		return true
	}
	h.writeReviewErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
		"제출 권한이 없는 사용자입니다", "")
	return false
}

// guardReviewAdmin POST /api/v1/reviews/{id}/{assign-reviewer|approve|reject} 진입 직전 admin-only 게이트
func (h *ReviewHandler) guardReviewAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return true // auth-disabled 투과
	}
	if requireReviewAdminRole(strings.Join(u.Scopes, " ")) {
		return true
	}
	h.writeReviewErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
		"관리자 권한이 없는 사용자입니다", "")
	return false
}

// ── 요청/응답 DTO ─────────────────────────────────────────────────────────────

// reviewCreateBody POST /api/v1/reviews 요청 본문
type reviewCreateBody struct {
	Metadata map[string]any `json:"metadata"`
	ScoreID  string         `json:"score_id"`
	Comment  string         `json:"comment"`
}

// reviewAssignBody POST /api/v1/reviews/{id}/assign-reviewer 요청 본문
type reviewAssignBody struct {
	ReviewerID string `json:"reviewer_id"`
}

// reviewApproveBody POST /api/v1/reviews/{id}/approve 요청 본문
type reviewApproveBody struct {
	Comment string `json:"comment"`
}

// reviewRejectBody POST /api/v1/reviews/{id}/reject 요청 본문
type reviewRejectBody struct {
	RejectionReason string `json:"rejection_reason"`
	Comment         string `json:"comment"`
}

// reviewResponse ScoreReviewRequest 직렬화 응답
type reviewResponse struct {
	Metadata           map[string]any `json:"metadata,omitempty"`
	AssignedReviewerID *string        `json:"assigned_reviewer_id,omitempty"`
	RejectionReason    *string        `json:"rejection_reason,omitempty"`
	Comment            *string        `json:"comment,omitempty"`
	ID                 string         `json:"id"`
	ScoreID            string         `json:"score_id"`
	Status             string         `json:"status"`
	CreatedAt          string         `json:"created_at"`
	CreatedBy          string         `json:"created_by"`
	UpdatedAt          string         `json:"updated_at"`
	UpdatedBy          string         `json:"updated_by"`
}

// toReviewResponse store.ScoreReviewRequest를 응답 DTO로 변환
func toReviewResponse(r *store.ScoreReviewRequest) reviewResponse {
	return reviewResponse{
		ID:                 r.ID.String(),
		ScoreID:            r.ScoreID.String(),
		Status:             r.Status,
		AssignedReviewerID: r.AssignedReviewerID,
		RejectionReason:    r.RejectionReason,
		Comment:            r.Comment,
		Metadata:           r.Metadata,
		CreatedAt:          r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy:          r.CreatedBy,
		UpdatedAt:          r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedBy:          r.UpdatedBy,
	}
}

// parseReviewID path {id} → UUID 파싱 (malformed → 400, store 미진입)
func (h *ReviewHandler) parseReviewID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "유효하지 않은 평가 검토 ID 형식입니다", "id")
		return uuid.Nil, false
	}
	return id, true
}

// ── 6 핸들러 메서드 ───────────────────────────────────────────────────────────

// handleCreateReview POST /api/v1/reviews — §A.3 cross-store 2-TX
// TX-1: scoreStore.BeginScoreTx → GetScoreByID → Rollback (read-only, defer)
// TX-2: reviewStore.BeginScoreReviewRequestTx → InsertScoreReviewRequest → Commit
//
// @MX:WARN: [AUTO] cross-store 2-TX — 2개 read+write TX defer Rollback 누락 시 커넥션 누수
// @MX:REASON: §A.3 RESOLVED handler-compose 패턴 — TX-1 score 검증, TX-2 review insert (race window=不發生 PoC 수용)
func (h *ReviewHandler) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	if !h.guardReviewSubmit(w, r) {
		return
	}
	var body reviewCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	scoreID, err := uuid.Parse(strings.TrimSpace(body.ScoreID))
	if err != nil || scoreID == uuid.Nil {
		h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "유효하지 않은 점수 ID 형식입니다", "score_id")
		return
	}

	ctx := r.Context()

	// ── TX-1 (read-only): score 존재 검증 (§A.3) ─────────────────────────────
	scoreTx, err := h.scoreStore.BeginScoreTx(ctx)
	if err != nil {
		h.logger.Error("BeginScoreTx 실패 (cross-store TX-1)", zap.Error(err))
		h.writeReviewErr(w, http.StatusInternalServerError, "INTERNAL", "점수 검증 트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = scoreTx.Rollback(ctx) }() //nolint:errcheck // 읽기 전용 — 항상 rollback

	if _, gerr := scoreTx.GetScoreByID(ctx, scoreID); gerr != nil {
		// score not-found → 404, score handler 미위임 (자체 매핑으로 일관성)
		h.writeReviewStoreErr(w, gerr)
		return
	}

	// ── TX-2 (write): 평가 검토 INSERT (§A.3) ────────────────────────────────
	reviewTx, err := h.reviewStore.BeginScoreReviewRequestTx(ctx)
	if err != nil {
		h.logger.Error("BeginScoreReviewRequestTx 실패 (cross-store TX-2)", zap.Error(err))
		h.writeReviewErr(w, http.StatusInternalServerError, "INTERNAL", "평가 검토 트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = reviewTx.Rollback(ctx) //nolint:errcheck // BeginTx 직후 즉시 등록
		}
	}()

	actor := resolveCreatedBy(r)
	newID, err := reviewTx.InsertScoreReviewRequest(ctx, scoreID, body.Comment, body.Metadata, actor)
	if err != nil {
		h.writeReviewStoreErr(w, err)
		return
	}
	if cmErr := reviewTx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패 — 롤백", zap.Error(cmErr))
		h.writeReviewErr(w, http.StatusInternalServerError, "INTERNAL", "평가 검토 저장 실패", "")
		return
	}
	committed = true

	writeReviewJSON(w, http.StatusCreated, map[string]any{
		"id":         newID.String(),
		"score_id":   scoreID.String(),
		"status":     "SUBMITTED",
		"created_by": actor,
		"updated_by": actor,
	})
}

// resolveCreatedBy auth context로부터 created_by 추출 (auth-disabled 시 'cli-anonymous')
func resolveCreatedBy(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u.UID != "" {
		return u.UID
	}
	return "cli-anonymous"
}

// handleGetReview GET /api/v1/reviews/{id} (모든 인증 사용자, auth-disabled 투과)
func (h *ReviewHandler) handleGetReview(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseReviewID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := h.reviewStore.BeginScoreReviewRequestTx(ctx)
	if err != nil {
		h.logger.Error("BeginScoreReviewRequestTx 실패", zap.Error(err))
		h.writeReviewErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // 읽기 전용

	r1, gerr := tx.GetScoreReviewRequestByID(ctx, id)
	if gerr != nil {
		h.writeReviewStoreErr(w, gerr)
		return
	}
	writeReviewJSON(w, http.StatusOK, toReviewResponse(r1))
}

// handleListReviews GET /api/v1/reviews?status=&limit=&offset=
func (h *ReviewHandler) handleListReviews(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statusFilter := strings.TrimSpace(q.Get("status"))
	limit, offset := clampReviewPagination(q.Get("limit"), q.Get("offset"))

	ctx := r.Context()
	tx, err := h.reviewStore.BeginScoreReviewRequestTx(ctx)
	if err != nil {
		h.logger.Error("BeginScoreReviewRequestTx 실패", zap.Error(err))
		h.writeReviewErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // 읽기 전용

	items, lerr := tx.ListScoreReviewRequests(ctx, statusFilter, limit, offset)
	if lerr != nil {
		h.writeReviewStoreErr(w, lerr)
		return
	}
	// AC-REVIEW-002-3: total은 limit/offset 적용 전 전체 카운트 (full COUNT(*))
	total, cErr := tx.CountScoreReviewRequests(ctx, statusFilter)
	if cErr != nil {
		h.writeReviewStoreErr(w, cErr)
		return
	}
	resp := make([]reviewResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, toReviewResponse(it))
	}
	writeReviewJSON(w, http.StatusOK, map[string]any{
		"items": resp,
		"total": total,
	})
}

// handleAssignReviewer POST /api/v1/reviews/{id}/assign-reviewer (admin only)
//
// @MX:WARN: [AUTO] mutation TX — Commit 전 early-return 시 부분커밋 위험
// @MX:REASON: committed-defer Rollback BeginTx 직후 즉시 등록 (score_handlers.go:454-460 선례 미러)
func (h *ReviewHandler) handleAssignReviewer(w http.ResponseWriter, r *http.Request) {
	if !h.guardReviewAdmin(w, r) {
		return
	}
	id, ok := h.parseReviewID(w, r)
	if !ok {
		return
	}
	var body reviewAssignBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	if strings.TrimSpace(body.ReviewerID) == "" {
		h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "검토자 ID는 필수입니다", "reviewer_id")
		return
	}

	actor := resolveCreatedBy(r)
	h.executeMutation(w, r, func(tx store.ScoreReviewRequestTx) error {
		return tx.AssignReviewer(r.Context(), id, body.ReviewerID, actor)
	}, id)
}

// handleApprove POST /api/v1/reviews/{id}/approve (admin only)
func (h *ReviewHandler) handleApprove(w http.ResponseWriter, r *http.Request) {
	if !h.guardReviewAdmin(w, r) {
		return
	}
	id, ok := h.parseReviewID(w, r)
	if !ok {
		return
	}
	var body reviewApproveBody
	// approve body는 optional comment만 — JSON 파싱 실패는 빈 본문 허용
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
			return
		}
	}

	actor := resolveCreatedBy(r)
	h.executeMutation(w, r, func(tx store.ScoreReviewRequestTx) error {
		return tx.ApproveRequest(r.Context(), id, body.Comment, actor)
	}, id)
}

// handleReject POST /api/v1/reviews/{id}/reject (admin only, rejection_reason required §A.5 Layer 1)
func (h *ReviewHandler) handleReject(w http.ResponseWriter, r *http.Request) {
	if !h.guardReviewAdmin(w, r) {
		return
	}
	id, ok := h.parseReviewID(w, r)
	if !ok {
		return
	}
	var body reviewRejectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	// §A.5 Layer 1: handler-side rejection_reason non-empty validation
	if strings.TrimSpace(body.RejectionReason) == "" {
		h.writeReviewErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "반려 사유는 필수입니다", "rejection_reason")
		return
	}

	actor := resolveCreatedBy(r)
	h.executeMutation(w, r, func(tx store.ScoreReviewRequestTx) error {
		return tx.RejectRequest(r.Context(), id, body.RejectionReason, body.Comment, actor)
	}, id)
}

// executeMutation Begin → fn(tx) → Commit (defer Rollback 안전) 공통 패턴
// 성공 시 갱신된 entity를 200 OK로 응답
func (h *ReviewHandler) executeMutation(w http.ResponseWriter, r *http.Request, fn func(store.ScoreReviewRequestTx) error, id uuid.UUID) {
	ctx := r.Context()
	tx, err := h.reviewStore.BeginScoreReviewRequestTx(ctx)
	if err != nil {
		h.logger.Error("BeginScoreReviewRequestTx 실패", zap.Error(err))
		h.writeReviewErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck // BeginTx 직후 즉시 등록
		}
	}()

	if err := fn(tx); err != nil {
		h.writeReviewStoreErr(w, err)
		return
	}

	// 갱신된 entity 재조회 (응답용)
	updated, gerr := tx.GetScoreReviewRequestByID(ctx, id)
	if gerr != nil {
		h.writeReviewStoreErr(w, gerr)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("Commit 실패 — 롤백", zap.Error(err))
		h.writeReviewErr(w, http.StatusInternalServerError, "INTERNAL", "평가 검토 갱신 실패", "")
		return
	}
	committed = true

	writeReviewJSON(w, http.StatusOK, toReviewResponse(updated))
}
