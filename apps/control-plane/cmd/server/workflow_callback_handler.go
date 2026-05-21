// workflow_callback_handler.go — Python→Go 콜백 핸들러 (SPEC-AX-INTEG-001)
//
// 라우트(ServeMux Go1.22+): POST /api/v1/workflows/{id}/callback
//
// 본 SPEC은 Python Celery worker가 처리 결과를 Go Control Plane으로 보고하는
// 단방향 콜백 채널이다. RUNNING → COMPLETED/FAILED 전이만 처리하며,
// 단말 상태(COMPLETED/FAILED) 또는 비-RUNNING(PENDING) 상태에서 도착한 콜백은 409 Conflict.
//
// 트랜잭션 원자성 (REQ-INTEG-004):
//   1. BeginTx → 2. GetWorkflow → 3. UpdateWorkflowState → 4. UpdateWorkflowResult →
//   5. InsertAuditLog(WORKFLOW_COMPLETED|WORKFLOW_FAILED_CALLBACK) → 6. Commit
//   모든 단계는 단일 pgx TX 위에서 진행 — 부분 실패 시 전체 rollback (양방향 원자성).
//
// 신규 Action 상수 0, 신규 store 메서드 0 (모두 audit.go·store.go 기존재):
//   - audit.ActionWorkflowCompleted (audit.go:21)
//   - audit.ActionWorkflowFailedCallback (audit.go:25)
//   - audit.ActionCallbackRejectedTerminal (audit.go:29)
//   - WorkflowTx.GetWorkflow / UpdateWorkflowState / UpdateWorkflowResult / InsertAuditLog (store.go:38-62)
//
// consumer-only 0-diff 대상:
//   - apps/control-plane/internal/audit/ (Action 상수 신설 0)
//   - apps/control-plane/internal/store/ (인터페이스 메서드 신설 0)
//   - apps/control-plane/internal/auth/ + rbac.go (역할 신설 0 — 콜백은 내부 신뢰 경계)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// 콜백 상태 열거 (HTTP 계약, §5.2)
const (
	callbackStatusCompleted = "completed"
	callbackStatusFailed    = "failed"
)

// WorkflowCallbackHandler Python→Go 콜백 REST 엔드포인트 핸들러.
//
// @MX:ANCHOR: [AUTO] 콜백 REST 진입점 — server.go 마운트 + 단위 테스트 + Routes() 3곳 이상
// @MX:REASON: 콜백 단일 HTTP 계약 (SPEC-AX-INTEG-001, 1 엔드포인트, RUNNING→terminal 전이 전담)
type WorkflowCallbackHandler struct {
	store    store.WorkflowStore
	recorder *audit.Recorder
	logger   *zap.Logger
}

// NewWorkflowCallbackHandler 콜백 핸들러를 생성한다.
// recorder: audit_logs 동일-TX 기록용 (audit.NewRecorder(authEnabled)로 생성된 인스턴스 전달).
func NewWorkflowCallbackHandler(st store.WorkflowStore, rec *audit.Recorder, logger *zap.Logger) *WorkflowCallbackHandler {
	return &WorkflowCallbackHandler{store: st, recorder: rec, logger: logger}
}

// Routes 콜백 라우트 1개를 등록한 http.Handler 반환.
// ServeMux Go1.22+ path-param 활용 — {id}는 r.PathValue("id")로 추출.
//
// @MX:ANCHOR: [AUTO] 콜백 라우트 등록 단일 지점 — server.go 마운트 + 핸들러 테스트
// @MX:REASON: Go1.22 ServeMux path-param 라우팅 구조적 필수
func (h *WorkflowCallbackHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/workflows/{id}/callback", h.handleCallback)
	return mux
}

// ── 요청/응답 DTO ────────────────────────────────────────────────────────────────

// callbackRequest §5.2 Request Body 스키마
// status: "completed" | "failed" (정확 일치, 그 외 400)
// result_json: free-form JSONB — 생략 가능 (생략 시 빈 객체 {})
type callbackRequest struct {
	Status     string          `json:"status"`
	ResultJSON json.RawMessage `json:"result_json,omitempty"`
}

// callbackErrorBody 4xx/5xx 표준 envelope — score_handlers 패턴 동형
type callbackErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeCallbackErr 4xx/5xx 응답 헬퍼 — INFO 로그
func (h *WorkflowCallbackHandler) writeCallbackErr(w http.ResponseWriter, code int, errCode, msg string) {
	var body callbackErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck // 헤더 전송 후라 로깅 불가
	h.logger.Info("워크플로우 콜백 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("message", msg),
	)
}

// ── 메인 핸들러 ───────────────────────────────────────────────────────────────────

// handleCallback POST /api/v1/workflows/{id}/callback — Python worker 결과 보고
//
// HTTP 응답 매트릭스 (§5.2):
//   204 No Content   — RUNNING → COMPLETED/FAILED 전이 성공
//   400 Bad Request  — JSON 파싱 실패, status 값 위반, UUID 위반
//   404 Not Found    — 워크플로우 미존재
//   409 Conflict     — 비-RUNNING 상태 (PENDING/COMPLETED/FAILED)
//   500              — DB 오류 (TX rollback)
//
// @MX:WARN: [AUTO] 신뢰 경계 — 본 핸들러는 내부 서비스 간 호출 가정 (localhost docker network).
// 외부망 노출 시 발신자 인증(mTLS/pre-shared key) 추가 필수 — §11 #11 후속 SPEC.
// @MX:REASON: callback body의 status 필드 자체는 검증되나, 발신자 신원은 본 SPEC 범위 외
func (h *WorkflowCallbackHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	// 1. path UUID 파싱
	raw := r.PathValue("id")
	wfID, err := uuid.Parse(raw)
	if err != nil {
		h.writeCallbackErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"워크플로우 ID가 유효한 UUID가 아닙니다")
		return
	}

	// 2. body 파싱 + 검증
	var req callbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeCallbackErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"요청 본문 JSON 파싱 실패")
		return
	}
	if req.Status != callbackStatusCompleted && req.Status != callbackStatusFailed {
		h.writeCallbackErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"status 필드는 'completed' 또는 'failed'만 허용됩니다")
		return
	}

	// 3. result_json 정규화 — 생략 시 빈 객체 "{}" 로 영속
	resultJSON := []byte(req.ResultJSON)
	if len(resultJSON) == 0 {
		resultJSON = []byte("{}")
	}

	// 4. TX 시작 — GetWorkflow + 전이 + 결과 + audit 단일 TX
	ctx := r.Context()
	tx, err := h.store.BeginTx(ctx)
	if err != nil {
		h.logger.Error("BeginTx 실패", zap.Error(err))
		h.writeCallbackErr(w, http.StatusInternalServerError, "INTERNAL",
			"트랜잭션 시작 실패")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // 멱등 rollback

	// 5. GetWorkflow — 미존재 → 404, 그 외 → 500
	wf, err := tx.GetWorkflow(ctx, wfID.String())
	if err != nil {
		if errors.Is(err, apperrors.ErrWorkflowNotFound) {
			h.writeCallbackErr(w, http.StatusNotFound, "NOT_FOUND",
				"워크플로우를 찾을 수 없습니다")
			return
		}
		h.logger.Error("GetWorkflow 실패", zap.Error(err), zap.String("workflow_id", wfID.String()))
		h.writeCallbackErr(w, http.StatusInternalServerError, "INTERNAL",
			"워크플로우 조회 실패")
		return
	}

	// 6. 상태 검증 — RUNNING만 허용, 비-RUNNING은 409 + CALLBACK_REJECTED_TERMINAL audit
	if wf.State != types.WorkflowStateRunning {
		h.recordRejectedTerminal(ctx, tx, wfID, wf.State, req.Status)
		if commitErr := tx.Commit(ctx); commitErr != nil {
			h.logger.Warn("CALLBACK_REJECTED_TERMINAL audit commit 실패",
				zap.Error(commitErr), zap.String("workflow_id", wfID.String()))
		}
		h.writeCallbackErr(w, http.StatusConflict, "INVALID_TRANSITION",
			"워크플로우가 RUNNING 상태가 아니어서 콜백을 수용할 수 없습니다")
		return
	}

	// 7. 상태 전이 결정
	var newState types.WorkflowState
	var action audit.Action
	if req.Status == callbackStatusCompleted {
		newState = types.WorkflowStateCompleted
		action = audit.ActionWorkflowCompleted
	} else {
		newState = types.WorkflowStateFailed
		action = audit.ActionWorkflowFailedCallback
	}

	// 8. UpdateWorkflowState
	if err := tx.UpdateWorkflowState(ctx, wfID.String(), newState); err != nil {
		h.logger.Error("UpdateWorkflowState 실패", zap.Error(err),
			zap.String("workflow_id", wfID.String()),
			zap.String("new_state", string(newState)))
		h.writeCallbackErr(w, http.StatusInternalServerError, "INTERNAL",
			"워크플로우 상태 갱신 실패")
		return
	}

	// 9. UpdateWorkflowResult
	if err := tx.UpdateWorkflowResult(ctx, wfID.String(), resultJSON); err != nil {
		h.logger.Error("UpdateWorkflowResult 실패", zap.Error(err))
		h.writeCallbackErr(w, http.StatusInternalServerError, "INTERNAL",
			"워크플로우 결과 영속화 실패")
		return
	}

	// 10. audit_logs INSERT 동일 TX (REQ-INTEG-007 / REQ-UBI-003)
	auditEvent := &audit.Event{
		Timestamp:    time.Now().UTC(),
		Action:       action,
		ResourceType: "workflow",
		ResourceID:   wfID,
		UserID:       audit.DefaultUserID, // auth-disabled 기본 (REQ-UBI-003)
	}
	if err := tx.InsertAuditLog(ctx, auditEvent); err != nil {
		h.logger.Error("audit_logs INSERT 실패", zap.Error(err),
			zap.String("action", string(action)))
		h.writeCallbackErr(w, http.StatusInternalServerError, "INTERNAL",
			"감사 로그 기록 실패")
		return
	}

	// 11. Commit
	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("TX Commit 실패", zap.Error(err))
		h.writeCallbackErr(w, http.StatusInternalServerError, "INTERNAL",
			"트랜잭션 커밋 실패")
		return
	}

	// 12. 204 No Content — body 없음 (§5.2)
	w.WriteHeader(http.StatusNoContent)
}

// recordRejectedTerminal 비-RUNNING 상태 콜백 거부를 audit_logs에 기록.
// 본 audit row는 거부 사실 자체의 추적성을 위해 commit 되며, 워크플로우 자체는 무변경.
func (h *WorkflowCallbackHandler) recordRejectedTerminal(
	ctx context.Context,
	tx store.WorkflowTx,
	wfID uuid.UUID,
	currentState types.WorkflowState,
	requestedStatus string,
) {
	details, err := json.Marshal(map[string]string{
		"current_state":    string(currentState),
		"requested_status": requestedStatus,
	})
	if err != nil {
		// JSON marshal 실패는 사실상 불가 — 안전을 위해 details 없이 진행
		details = nil
	}
	event := &audit.Event{
		Timestamp:    time.Now().UTC(),
		Action:       audit.ActionCallbackRejectedTerminal,
		ResourceType: "workflow",
		ResourceID:   wfID,
		UserID:       audit.DefaultUserID,
		DetailsJSON:  details,
	}
	if insErr := tx.InsertAuditLog(ctx, event); insErr != nil {
		// audit 실패는 콜백 결과(409)에 영향 주지 않음 — 로그만 기록
		h.logger.Warn("CALLBACK_REJECTED_TERMINAL audit_logs INSERT 실패",
			zap.Error(insErr), zap.String("workflow_id", wfID.String()))
	}
}
