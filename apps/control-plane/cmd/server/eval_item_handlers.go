// eval_item_handlers.go — 평가항목(EvalItem) taxonomy REST API HTTP 핸들러
// (SPEC-AX-EVAL-ITEM-001 라우트 등록 — store/audit 구현 기 완료, 핸들러+라우트 추가)
//
// 라우트(ServeMux Go1.22+, 최장일치):
//
//	POST   /api/v1/eval-items                   (admin only)
//	GET    /api/v1/eval-items/{id}/children      (모든 인증)
//	GET    /api/v1/eval-items/{id}               (모든 인증)
//	PUT    /api/v1/eval-items/{id}               (admin only)
//
// 본 핸들러는 SPEC-AX-EVAL-ITEM-001(store/audit)·SPEC-AX-AUTH-003(ABAC)의
// consumer다. store 0-diff. recorder.RecordEvalItemCreated/Updated 호출로 감사 기록.
//
// 목록(GET /api/v1/eval-items) 미제공 — EvalItemStore에 ListEvalItems 미존재.
// 루트 조회는 GET /{id}/children를 parentID="" 공약(루트 전용 미래 SPEC) 대기.
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// EvalItemHandler 평가항목 taxonomy REST 엔드포인트 핸들러.
// recorder — 감사는 handler TX orchestration에서 recorder.RecordEvalItem* 호출 (UBI-002).
// 필드 순서: 인터페이스(16B) → 포인터(8B) (fieldalignment 정렬).
//
// @MX:ANCHOR: [AUTO] 평가항목 REST 진입점 — server.go 마운트 + 핸들러 테스트 + Routes() 3곳 이상
// @MX:REASON: 평가항목 taxonomy 단일 HTTP 계약 (SPEC-AX-EVAL-ITEM-001, 4 엔드포인트)
type EvalItemHandler struct {
	store    store.EvalItemStore
	recorder *audit.Recorder
	logger   *zap.Logger
}

// NewEvalItemHandler 평가항목 핸들러를 생성한다.
// pgStore가 EvalItemStore 구현 — server.go:NewEvalItemHandler(pgStore, rec, logger) 형태로 주입.
func NewEvalItemHandler(st store.EvalItemStore, rec *audit.Recorder, logger *zap.Logger) *EvalItemHandler {
	return &EvalItemHandler{store: st, recorder: rec, logger: logger}
}

// Routes 평가항목 라우트 4개를 등록한 http.Handler 반환.
// ServeMux Go1.22+ 최장일치: /children 이 /{id} 보다 우선 매칭.
//
// @MX:ANCHOR: [AUTO] 4 라우트 등록 단일 지점 — server.go 마운트 + 핸들러 테스트
// @MX:REASON: Go1.22 ServeMux path-param 라우팅 — /children 구체 경로 우선 등록 필수
func (h *EvalItemHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	// 구체 경로(최장일치 우선) 먼저 등록
	mux.HandleFunc("GET /api/v1/eval-items/{id}/children", h.handleGetChildren)
	mux.HandleFunc("GET /api/v1/eval-items/{id}", h.handleGetByID)
	mux.HandleFunc("PUT /api/v1/eval-items/{id}", h.handleUpdate)
	mux.HandleFunc("POST /api/v1/eval-items", h.handleCreate)
	return mux
}

// ── 표준 JSON/에러 헬퍼 (rubric_handlers.go:111-130 미러) ────────────────────

// evalItemErrorBody 에러 응답
type evalItemErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

// writeEvalItemJSON Content-Type 설정 후 JSON 직렬화
func writeEvalItemJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // 헤더 전송 후라 로깅 불가
}

// writeEvalItemErr 표준 에러 본문 + INFO 로그
func (h *EvalItemHandler) writeEvalItemErr(w http.ResponseWriter, code int, errCode, msg, field string) {
	var body evalItemErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	body.Error.Field = field
	h.logger.Info("평가항목 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("field", field),
	)
	writeEvalItemJSON(w, code, body)
}

// ── ABAC 핸들러-로컬 게이트 (rubric_handlers.go:209-235 미러) ─────────────────

// requireEvalItemAdminRole admin 권한 보유 여부 확인
func requireEvalItemAdminRole(scope string) bool {
	for _, r := range auth.ParseRolesFromScope(scope) {
		if r == auth.RoleAdmin {
			return true
		}
	}
	return false
}

// guardEvalItemAdmin mutation 진입 직전 admin-only 게이트.
// auth-disabled(Walking Skeleton) → 투과(true, UBI-003).
// auth 활성 시 RoleAdmin 아니면 403 ABAC_CONDITION_DENIED 응답 후 false.
func (h *EvalItemHandler) guardEvalItemAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return true // auth-disabled 투과 (cli-anonymous, UBI-003)
	}
	if requireEvalItemAdminRole(strings.Join(u.Scopes, " ")) {
		return true
	}
	h.writeEvalItemErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
		"관리자 권한이 없는 사용자입니다", "")
	return false
}

// ── 에러 매핑 (apperrors.ErrEvalItem* 5 sentinels) ──────────────────────────

// mapEvalItemStoreErr store 센티넬을 HTTP status + 에러코드 + 한국어 메시지로 결정적 매핑.
func mapEvalItemStoreErr(err error) (int, string, string) {
	switch {
	case errors.Is(err, apperrors.ErrEvalItemNotFound):
		return http.StatusNotFound, "NOT_FOUND", "요청한 평가항목을 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrEvalItemParentNotFound):
		return http.StatusNotFound, "NOT_FOUND", "지정한 부모 항목을 찾을 수 없습니다"
	case errors.Is(err, apperrors.ErrEvalItemInvalidInput):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "평가항목 입력값이 유효하지 않습니다"
	case errors.Is(err, apperrors.ErrEvalItemHierarchyImmutable):
		return http.StatusConflict, "HIERARCHY_IMMUTABLE", "자식 항목이 있어 계층 속성을 변경할 수 없습니다"
	case errors.Is(err, apperrors.ErrEvalItemInvalidStatus):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "유효하지 않은 status 값입니다"
	default:
		return http.StatusInternalServerError, "INTERNAL", "서버 내부 오류가 발생했습니다"
	}
}

// writeEvalItemStoreErr store 에러를 HTTP 응답으로 변환
func (h *EvalItemHandler) writeEvalItemStoreErr(w http.ResponseWriter, err error) {
	code, errCode, msg := mapEvalItemStoreErr(err)
	h.writeEvalItemErr(w, code, errCode, msg, "")
}

// ── 요청/응답 DTO ─────────────────────────────────────────────────────────────

// evalItemCreateBody POST /api/v1/eval-items 요청 본문
type evalItemCreateBody struct {
	Metadata      map[string]any `json:"metadata"`
	ID            string         `json:"id"`
	ParentID      *string        `json:"parent_id"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	HierarchyCode string         `json:"hierarchy_code"`
	Level         *int           `json:"level"`
	Weight        *float64       `json:"weight"`
	MaxScore      *int           `json:"max_score"`
}

// evalItemUpdateBody PUT /api/v1/eval-items/{id} 요청 본문.
// parent_id: *json.RawMessage — absent(nil)=변경 없음, "null"=NULL로 초기화, string=부모 변경.
type evalItemUpdateBody struct {
	ParentIDRaw *json.RawMessage `json:"parent_id"`
	Metadata    *map[string]any  `json:"metadata"`
	DisplayName *string          `json:"display_name"`
	Description *string          `json:"description"`
	Status      *string          `json:"status"`
	Level       *int             `json:"level"`
	Weight      *float64         `json:"weight"`
	MaxScore    *int             `json:"max_score"`
}

// toStoreUpdate evalItemUpdateBody를 store.EvalItemUpdate로 변환
func (b *evalItemUpdateBody) toStoreUpdate() (store.EvalItemUpdate, error) {
	upd := store.EvalItemUpdate{
		DisplayName: b.DisplayName,
		Description: b.Description,
		Level:       b.Level,
		Weight:      b.Weight,
		MaxScore:    b.MaxScore,
		Status:      b.Status,
		Metadata:    b.Metadata,
	}
	if b.ParentIDRaw != nil {
		raw := strings.TrimSpace(string(*b.ParentIDRaw))
		if raw == "null" {
			// parent_id: null → 루트로 초기화 (**string → &nil)
			var nilStr *string
			upd.ParentID = &nilStr
		} else {
			// parent_id: "someID" → 부모 변경
			var parentID string
			if err := json.Unmarshal(*b.ParentIDRaw, &parentID); err != nil {
				return upd, err
			}
			parentIDStr := parentID
			parentIDPtr := &parentIDStr
			upd.ParentID = &parentIDPtr
		}
	}
	return upd, nil
}

// evalItemResponse 단건/목록 응답 DTO
type evalItemResponse struct {
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	ParentID      *string        `json:"parent_id"`
	Level         *int           `json:"level"`
	Weight        *float64       `json:"weight"`
	MaxScore      *int           `json:"max_score"`
	ID            string         `json:"id"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	HierarchyCode string         `json:"hierarchy_code"`
	Status        string         `json:"status"`
	CreatedBy     string         `json:"created_by"`
}

// toEvalItemResponse store.EvalItem → HTTP 응답 DTO 변환
func toEvalItemResponse(item *store.EvalItem) evalItemResponse {
	return evalItemResponse{
		ID:            item.ID,
		ParentID:      item.ParentID,
		DisplayName:   item.DisplayName,
		Description:   item.Description,
		Level:         item.Level,
		HierarchyCode: item.HierarchyCode,
		Weight:        item.Weight,
		MaxScore:      item.MaxScore,
		Status:        item.Status,
		Metadata:      item.Metadata,
		CreatedAt:     item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy:     item.CreatedBy,
	}
}

// ── 핸들러 ────────────────────────────────────────────────────────────────────

// handleCreate POST /api/v1/eval-items (admin only) — 평가항목 생성
func (h *EvalItemHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !h.guardEvalItemAdmin(w, r) {
		return
	}
	var body evalItemCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}
	if strings.TrimSpace(body.ID) == "" {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "id는 필수입니다", "id")
		return
	}
	if strings.TrimSpace(body.DisplayName) == "" {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "display_name은 필수입니다", "display_name")
		return
	}
	if strings.TrimSpace(body.HierarchyCode) == "" {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "hierarchy_code는 필수입니다", "hierarchy_code")
		return
	}

	ctx := r.Context()
	tx, err := h.store.BeginEvalItemTx(ctx)
	if err != nil {
		h.logger.Error("BeginEvalItemTx 실패", zap.Error(err))
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck // BeginEvalItemTx 직후 즉시 등록
		}
	}()

	actor := resolveCreatedBy(r)
	newID, err := tx.InsertEvalItem(ctx,
		body.ID,
		body.ParentID,
		body.DisplayName,
		body.Description,
		body.Level,
		body.HierarchyCode,
		body.Weight,
		body.MaxScore,
		body.Metadata,
	)
	if err != nil {
		h.writeEvalItemStoreErr(w, err)
		return
	}

	// 감사 기록 — 동일 TX 원자성 보장
	parentIDStr := ""
	if body.ParentID != nil {
		parentIDStr = *body.ParentID
	}
	levelInt := 0
	if body.Level != nil {
		levelInt = *body.Level
	}
	if recErr := h.recorder.RecordEvalItemCreated(ctx, tx, newID, body.HierarchyCode, parentIDStr, levelInt, actor); recErr != nil {
		h.logger.Error("RecordEvalItemCreated 실패", zap.Error(recErr))
		// 감사 실패는 internal error로 처리 (원자성 — rollback)
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "감사 기록 실패", "")
		return
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "평가항목 저장 실패", "")
		return
	}
	committed = true

	writeEvalItemJSON(w, http.StatusCreated, map[string]any{
		"id":             newID,
		"hierarchy_code": body.HierarchyCode,
		"display_name":   body.DisplayName,
		"status":         "ACTIVE",
		"created_by":     actor,
	})
}

// handleGetByID GET /api/v1/eval-items/{id} (모든 인증) — 단건 조회
func (h *EvalItemHandler) handleGetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "id는 필수입니다", "id")
		return
	}

	ctx := r.Context()
	tx, err := h.store.BeginEvalItemTx(ctx)
	if err != nil {
		h.logger.Error("BeginEvalItemTx 실패", zap.Error(err))
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // read-only TX

	item, err := tx.GetEvalItemByID(ctx, id)
	if err != nil {
		h.writeEvalItemStoreErr(w, err)
		return
	}
	// read-only — Commit 불필요, Rollback으로 종료
	writeEvalItemJSON(w, http.StatusOK, toEvalItemResponse(item))
}

// handleGetChildren GET /api/v1/eval-items/{id}/children (모든 인증) — 직계 자식 목록
func (h *EvalItemHandler) handleGetChildren(w http.ResponseWriter, r *http.Request) {
	parentID := r.PathValue("id")
	if parentID == "" {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "id는 필수입니다", "id")
		return
	}

	ctx := r.Context()
	tx, err := h.store.BeginEvalItemTx(ctx)
	if err != nil {
		h.logger.Error("BeginEvalItemTx 실패", zap.Error(err))
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // read-only TX

	// 부모 존재 확인 (404 early return)
	if _, err := tx.GetEvalItemByID(ctx, parentID); err != nil {
		h.writeEvalItemStoreErr(w, err)
		return
	}

	children, err := tx.GetEvalItemsByParentID(ctx, parentID)
	if err != nil {
		h.writeEvalItemStoreErr(w, err)
		return
	}

	items := make([]evalItemResponse, 0, len(children))
	for _, c := range children {
		items = append(items, toEvalItemResponse(c))
	}
	writeEvalItemJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

// handleUpdate PUT /api/v1/eval-items/{id} (admin only) — 부분 갱신
func (h *EvalItemHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.guardEvalItemAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "id는 필수입니다", "id")
		return
	}

	var body evalItemUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 본문 JSON 파싱에 실패했습니다", "")
		return
	}

	upd, err := body.toStoreUpdate()
	if err != nil {
		h.writeEvalItemErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "parent_id 파싱 실패", "parent_id")
		return
	}

	ctx := r.Context()
	tx, err := h.store.BeginEvalItemTx(ctx)
	if err != nil {
		h.logger.Error("BeginEvalItemTx 실패", zap.Error(err))
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck
		}
	}()

	// 갱신 전 현재 항목 조회 (감사 기록에 hierarchyCode/parentID/level 필요)
	existing, err := tx.GetEvalItemByID(ctx, id)
	if err != nil {
		h.writeEvalItemStoreErr(w, err)
		return
	}

	if err := tx.UpdateEvalItem(ctx, id, upd); err != nil {
		h.writeEvalItemStoreErr(w, err)
		return
	}

	// 감사 기록 — 동일 TX 원자성 보장
	actor := resolveCreatedBy(r)
	parentIDStr := ""
	if existing.ParentID != nil {
		parentIDStr = *existing.ParentID
	}
	levelInt := 0
	if existing.Level != nil {
		levelInt = *existing.Level
	}
	if recErr := h.recorder.RecordEvalItemUpdated(ctx, tx, id, existing.HierarchyCode, parentIDStr, levelInt, actor); recErr != nil {
		h.logger.Error("RecordEvalItemUpdated 실패", zap.Error(recErr))
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "감사 기록 실패", "")
		return
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		h.logger.Error("Commit 실패", zap.Error(cmErr))
		h.writeEvalItemErr(w, http.StatusInternalServerError, "INTERNAL", "평가항목 갱신 실패", "")
		return
	}
	committed = true

	writeEvalItemJSON(w, http.StatusOK, map[string]any{"id": id, "status": "updated"})
}
