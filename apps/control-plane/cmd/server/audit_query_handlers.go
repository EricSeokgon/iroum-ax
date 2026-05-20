// audit_query_handlers.go — 감사 로그 검색 HTTP API 핸들러 (SPEC-AX-AUDIT-QUERY-001)
//
// 라우트(ServeMux Go1.22+): GET /api/v1/audit-logs (목록) · GET /api/v1/audit-logs/{id} (단건)
//
// 본 SPEC은 SPEC-AX-CTRL-001(audit_logs 0001 마이그레이션) + 6 SPEC 누적 Action 상수의
// 순수 read-only consumer다. store/audit/auth/스키마 0-diff, 신규 마이그레이션 0,
// 신규 Action 상수 0, 신규 Recorder 메서드 0, 신규 외부 의존 0.
//
// read-only 명시 [HARD]: 핸들러는 audit_logs INSERT를 0건 수행하며 Recorder 의존을
// 주입받지 않는다 (UBI-002-1). mutation 시점의 audit는 7 누적 SPEC store가 이미
// 동일-TX로 기록했다. 검색 요청 자체를 audit_logs에 기록하는 "audit-of-audit-read"는
// 본 SPEC 범위 밖(§5 #2 — 후속 SPEC SPEC-AX-AUDIT-READ-AUDIT-001 가능).
//
// admin-only narrowing [HARD]: 핸들러-레벨 helper requireAuditQueryReadRole + guardAuditQueryRead
// (score_handlers.go:161-187 write-role 패턴 동형). frozen rbac.go 0-diff — RoleAuditor 신설 0,
// permissionMatrix 무변경. RoleAdmin은 rbac.go:20-21에 기존재 — 핸들러-레벨 매핑으로 충분.
//
// D1 iter2 lesson acknowledgment: read-only이므로 NewRecorder mode 무관, mutation 메서드 0이라
// userID string 파라미터 0. SCORE-001 D1 iter2 lesson은 본 SPEC에 적용 불가하나 명시 acknowledge.
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	apperrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// 핸들러 사전 검증 상수.
// audit_logs.resource_type VARCHAR(32) / user_id VARCHAR(64) DDL 정합 (initial.sql:117-120).
const (
	maxAuditQueryResourceTypeLen = 32
	maxAuditQueryUserIDLen       = 64
)

// AuditQueryHandler 감사 로그 검색 REST 엔드포인트 핸들러 (read-only).
// recorder 의존 미주입 — UBI-002-1 read-only이므로 mutation 0 → audit 0.
// write-role 게이트 미차용 — score_handlers.go:161-187 requireScoreWriteRole 차용 0 (§1.5).
//
// @MX:ANCHOR: [AUTO] 감사 로그 검색 REST 진입점 — server.go 마운트 + 단위 테스트 + Routes() 3곳 이상
// @MX:REASON: 감사 로그 검색 단일 HTTP 계약 (SPEC-AX-AUDIT-QUERY-001, 2 엔드포인트, read-only)
type AuditQueryHandler struct {
	store  store.WorkflowStore
	logger *zap.Logger
}

// NewAuditQueryHandler 감사 로그 검색 핸들러를 생성한다 (store만 주입 — recorder 없음, UBI-002-1).
func NewAuditQueryHandler(st store.WorkflowStore, logger *zap.Logger) *AuditQueryHandler {
	return &AuditQueryHandler{store: st, logger: logger}
}

// Routes 감사 로그 검색 라우트 2개를 등록한 http.Handler 반환.
// ServeMux Go1.22+: 단건 /{id}와 목록 / 정확 매칭 별개 등록.
//
// @MX:ANCHOR: [AUTO] 2 라우트 등록 단일 지점 — server.go 마운트 + 핸들러 테스트
// @MX:REASON: Go1.22 ServeMux path-param 라우팅 구조적 필수 (collection + single)
func (h *AuditQueryHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/audit-logs/{id}", h.handleGetAuditLog)
	mux.HandleFunc("GET /api/v1/audit-logs", h.handleListAuditLogs)
	return mux
}

// ── 표준 JSON/에러 헬퍼 (score_handlers.go:74-101 미러, 한국어, INFO) ────────────

// auditQueryErrorBody 에러 응답 — {"error":{"code","message","field"}}
type auditQueryErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

// writeAuditQueryJSON Content-Type 설정 후 JSON 직렬화 (score_handlers.go:84 미러)
func writeAuditQueryJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // 헤더 전송 후라 로깅 불가
}

// writeAuditQueryErr 표준 에러 본문 + INFO 로그 (거부는 INFO 레벨)
func (h *AuditQueryHandler) writeAuditQueryErr(w http.ResponseWriter, code int, errCode, msg, field string) {
	var body auditQueryErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	body.Error.Field = field
	h.logger.Info("감사 로그 검색 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("field", field),
	)
	writeAuditQueryJSON(w, code, body)
}

// mapAuditQueryStoreErr store 센티넬을 HTTP status + 에러코드 + 한국어 메시지로 결정적 매핑.
// errors.Is로 래핑된 센티넬도 식별. unknown → 500.
//
// @MX:NOTE: [AUTO] 신규 2 sentinel(ErrAuditQueryInvalidFilter/ErrAuditQueryInvalidTimeRange)
// + unknown → 500. SCORE-API-001 errors.go drift lesson 정합.
func mapAuditQueryStoreErr(err error) (int, string, string) {
	switch {
	case errors.Is(err, apperrors.ErrAuditQueryInvalidFilter):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "감사 로그 검색 필터가 유효하지 않습니다"
	case errors.Is(err, apperrors.ErrAuditQueryInvalidTimeRange):
		return http.StatusBadRequest, "INVALID_ARGUMENT", "감사 로그 검색 시간 범위가 유효하지 않습니다"
	default:
		return http.StatusInternalServerError, "INTERNAL", "감사 로그 검색 중 오류가 발생했습니다"
	}
}

// writeAuditQueryStoreErr store 에러를 매핑하여 표준 본문으로 응답 (500은 ERROR 로그).
func (h *AuditQueryHandler) writeAuditQueryStoreErr(w http.ResponseWriter, err error) {
	code, errCode, msg := mapAuditQueryStoreErr(err)
	if code == http.StatusInternalServerError {
		h.logger.Error("감사 로그 검색 store 오류", zap.Error(err))
	}
	h.writeAuditQueryErr(w, code, errCode, msg, "")
}

// ── admin-only narrowing 게이팅 (handler-level helper, frozen rbac.go 0-diff) ──────

// requireAuditQueryReadRole scope 문자열에서 추출한 역할이 RoleAdmin인지 판단한다.
// score_handlers.go:161-187 requireScoreWriteRole 동형 — 단, RoleAnalyst 제외(admin only).
// frozen rbac.go·permissionMatrix 0-diff — RoleAdmin은 rbac.go:20-21에 기존재.
//
// @MX:NOTE: [AUTO] admin-only narrowing 핸들러-레벨 helper — frozen rbac.go 활용
// (SCORE-API-001 §6 OPEN #4 lesson 동형 — RoleAuditor 신설 0, permissionMatrix 무변경)
func requireAuditQueryReadRole(scope string) bool {
	for _, r := range auth.ParseRolesFromScope(scope) {
		if r == auth.RoleAdmin {
			return true
		}
	}
	return false
}

// guardAuditQueryRead read 진입 직전 admin-only 게이트.
// auth context 부재(auth-disabled Walking Skeleton) → 투과(true). user 존재 시
// admin 권한 없으면 403 ABAC_CONDITION_DENIED 응답 후 false 반환 (store TX 미진입).
//
// @MX:NOTE: [AUTO] abac.go:4 narrowing-only 동형 — ABAC는 allow 부여 없이 미인가만 거부.
// auth-disabled(ok=false)는 투과, viewer/analyst는 admin-only narrowing으로 거부.
func (h *AuditQueryHandler) guardAuditQueryRead(w http.ResponseWriter, r *http.Request) bool {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return true // auth-disabled 투과 (Walking Skeleton, REQ-AUDIT-QUERY-UBI-004)
	}
	if requireAuditQueryReadRole(strings.Join(u.Scopes, " ")) {
		return true
	}
	h.writeAuditQueryErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
		"감사 로그 조회 권한이 없습니다", "")
	return false
}

// ── 응답 DTO (§6 OPEN #3 RESOLVED — research §14 채택) ───────────────────────────

// auditEventResponse 단일 audit_logs 행의 응답 직렬화
type auditEventResponse struct {
	Details      json.RawMessage `json:"details,omitempty"`
	Timestamp    string          `json:"timestamp"`
	Action       string          `json:"action"`
	UserID       string          `json:"user_id"`
	ResourceType string          `json:"resource_type,omitempty"`
	ResourceID   string          `json:"resource_id"`
}

// toAuditEventResponse audit.Event → response DTO 변환
func toAuditEventResponse(e *audit.Event) auditEventResponse {
	resp := auditEventResponse{
		Timestamp:    e.Timestamp.UTC().Format(time.RFC3339),
		Action:       string(e.Action),
		UserID:       e.UserID,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID.String(),
	}
	if len(e.DetailsJSON) > 0 {
		resp.Details = e.DetailsJSON
	}
	return resp
}

// auditQueryListResponse 검색 목록 응답
type auditQueryListResponse struct {
	GeneratedAt string               `json:"generated_at"`
	Events      []auditEventResponse `json:"events"`
	Count       int                  `json:"count"`
	Total       int64                `json:"total"`
}

// ── 5-필터 파싱 (pre-store validation, REQ-AUDIT-QUERY-001-U1) ───────────────────

// parsedAuditFilter 검증 통과 후의 store filter
type parsedAuditFilter struct {
	filter   store.AuditLogFilter
	errField string
	errMsg   string
	errCode  string // 한국어 메시지 매핑용 (InvalidFilter vs InvalidTimeRange)
}

// parseAuditQueryFilters URL query → AuditLogFilter 변환 + 검증.
// 검증 실패 시 errField/errMsg가 non-empty, 호출자는 400 응답.
//
// @MX:NOTE: [AUTO] 5-필터 + ID 파싱은 store 미진입 사전 검증 — malformed UUID/timestamp/length
// 초과는 ErrAuditQueryInvalidFilter, since>until 또는 future timestamp는 ErrAuditQueryInvalidTimeRange.
func parseAuditQueryFilters(q map[string][]string) parsedAuditFilter {
	var pf parsedAuditFilter

	if v := firstQueryValue(q, "action"); v != "" {
		s := v
		pf.filter.Action = &s
	}
	if v := firstQueryValue(q, "resource_type"); v != "" {
		if len(v) > maxAuditQueryResourceTypeLen {
			pf.errField = "resource_type"
			pf.errMsg = "resource_type이 32자를 초과합니다"
			pf.errCode = "filter"
			return pf
		}
		s := v
		pf.filter.ResourceType = &s
	}
	if v := firstQueryValue(q, "resource_id"); v != "" {
		parsed, err := uuid.Parse(v)
		if err != nil {
			pf.errField = "resource_id"
			pf.errMsg = "resource_id가 유효한 UUID가 아닙니다"
			pf.errCode = "filter"
			return pf
		}
		pf.filter.ResourceID = &parsed
	}
	if v := firstQueryValue(q, "user_id"); v != "" {
		if len(v) > maxAuditQueryUserIDLen {
			pf.errField = "user_id"
			pf.errMsg = "user_id가 64자를 초과합니다"
			pf.errCode = "filter"
			return pf
		}
		s := v
		pf.filter.UserID = &s
	}

	var since, until *time.Time
	if v := firstQueryValue(q, "since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			pf.errField = "since"
			pf.errMsg = "since가 유효한 RFC3339 시간이 아닙니다"
			pf.errCode = "filter"
			return pf
		}
		since = &t
		pf.filter.Since = since
	}
	if v := firstQueryValue(q, "until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			pf.errField = "until"
			pf.errMsg = "until이 유효한 RFC3339 시간이 아닙니다"
			pf.errCode = "filter"
			return pf
		}
		until = &t
		pf.filter.Until = until
	}

	// 시간 범위 business logic 검증
	now := time.Now()
	if since != nil && until != nil && since.After(*until) {
		pf.errField = "since"
		pf.errMsg = "since는 until보다 클 수 없습니다"
		pf.errCode = "timerange"
		return pf
	}
	if until != nil && until.After(now) {
		pf.errField = "until"
		pf.errMsg = "until은 현재 시각을 초과할 수 없습니다"
		pf.errCode = "timerange"
		return pf
	}
	// since 단독 future도 거부 (REQ-AUDIT-QUERY-001-U1)
	if since != nil && since.After(now) {
		pf.errField = "since"
		pf.errMsg = "since는 현재 시각을 초과할 수 없습니다"
		pf.errCode = "timerange"
		return pf
	}

	return pf
}

// firstQueryValue url.Values map의 첫 비-공백 값 반환 (반복 키는 첫 값만)
func firstQueryValue(q map[string][]string, key string) string {
	vs, ok := q[key]
	if !ok || len(vs) == 0 {
		return ""
	}
	return strings.TrimSpace(vs[0])
}

// ── 목록 검색 핸들러 ──────────────────────────────────────────────────────────────

// handleListAuditLogs GET /api/v1/audit-logs — 5-필터 AND + offset/limit 페이지네이션.
// admin-only narrowing. read-only — audit_logs INSERT 0건.
//
// @MX:NOTE: [AUTO] D1 iter2 lesson acknowledgment: AUDIT-QUERY-001 is read-only;
// NewRecorder mode 무관, mutation 메서드 0이라 userID string 파라미터 0.
// SCORE-001 D1 iter2 lesson은 본 SPEC에 적용 불가하나 명시 acknowledge.
func (h *AuditQueryHandler) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	if !h.guardAuditQueryRead(w, r) {
		return
	}

	q := r.URL.Query()
	pf := parseAuditQueryFilters(q)
	if pf.errField != "" {
		h.writeAuditQueryErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", pf.errMsg, pf.errField)
		return
	}

	limit, offset := clampPagination(firstQueryValue(q, "limit"), firstQueryValue(q, "offset"))

	tx, err := h.store.BeginTx(r.Context())
	if err != nil {
		h.logger.Error("BeginTx 실패", zap.Error(err))
		h.writeAuditQueryErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }() //nolint:errcheck // 읽기 전용 — 항상 rollback

	events, total, err := tx.QueryAuditLogs(r.Context(), pf.filter, limit, offset)
	if err != nil {
		h.writeAuditQueryStoreErr(w, err)
		return
	}

	respEvents := make([]auditEventResponse, 0, len(events))
	for _, e := range events {
		respEvents = append(respEvents, toAuditEventResponse(e))
	}
	writeAuditQueryJSON(w, http.StatusOK, auditQueryListResponse{
		Events:      respEvents,
		Count:       len(respEvents),
		Total:       total,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// ── 단건 lookup 핸들러 ─────────────────────────────────────────────────────────────

// handleGetAuditLog GET /api/v1/audit-logs/{id} — 단건 lookup.
// path /{id}는 ResourceID 필터로 매핑 (audit_logs.id PK가 아닌 audit_logs.resource_id 기준).
// 빈 결과는 200 + events:[] (data-completeness, 404 비반환).
//
// 주의: SPEC §2.1은 /{id}를 명시하나 의미는 모호. 본 PoC에서는 ResourceID 필터로 매핑하여
// "이 자원에 대한 모든 audit 행"을 반환 — 사용자 의도 정합 + QueryAuditLogs 재사용.
// 단건 audit_logs.id PK lookup은 후속 SPEC에서 정밀 endpoint 추가 가능.
func (h *AuditQueryHandler) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	if !h.guardAuditQueryRead(w, r) {
		return
	}

	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		h.writeAuditQueryErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"id가 유효한 UUID가 아닙니다", "id")
		return
	}

	tx, err := h.store.BeginTx(r.Context())
	if err != nil {
		h.logger.Error("BeginTx 실패", zap.Error(err))
		h.writeAuditQueryErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }() //nolint:errcheck // 읽기 전용 — 항상 rollback

	filter := store.AuditLogFilter{ResourceID: &id}
	events, total, err := tx.QueryAuditLogs(r.Context(), filter, defaultListLimit, 0)
	if err != nil {
		h.writeAuditQueryStoreErr(w, err)
		return
	}

	respEvents := make([]auditEventResponse, 0, len(events))
	for _, e := range events {
		respEvents = append(respEvents, toAuditEventResponse(e))
	}
	writeAuditQueryJSON(w, http.StatusOK, auditQueryListResponse{
		Events:      respEvents,
		Count:       len(respEvents),
		Total:       total,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}
