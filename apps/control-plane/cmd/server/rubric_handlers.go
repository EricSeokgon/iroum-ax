// rubric_handlers.go — 등급 rubric REST API HTTP 핸들러 skeleton (SPEC-AX-RUBRIC-001)
//
// Phase A (RED): T-IFACE-003 skeleton. 모든 핸들러 메서드 빈 구현 — 컴파일만 통과 +
// RED 테스트가 genuine FAIL을 표시하도록 http.Error(NOT_IMPLEMENTED) 또는 503 반환.
//
// 라우트(ServeMux Go1.22+, 최장일치 — strategy.md §3.3):
//
//	POST /api/v1/rubrics/{id}/clone-new-version  (admin only, OPEN #1)
//	POST /api/v1/rubrics/{id}/apply              (모든 인증, cross-store 2-TX, OPEN #6 read-only)
//	POST /api/v1/rubrics/{id}/criteria           (admin only, cross-store EvalItem 검증)
//	POST /api/v1/rubrics/{id}/bands              (admin only, OPEN #4 dual defense)
//	POST /api/v1/rubrics/{id}/archive            (admin only, OPEN #7 archive_reason 필수)
//	PUT  /api/v1/rubrics/{id}                    (admin only, OPEN #5 active 직접 편집 허용)
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
package main

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// 핸들러 검증/페이지네이션 상수 — review_handlers.go:46-49 / score_handlers.go:31-35 미러
const (
	maxRubricListLimit     = 500
	defaultRubricListLimit = 50
)

// RubricHandler 등급 rubric REST 엔드포인트 핸들러 (Phase A skeleton).
// 3 store 의존(rubricStore + evalItemStore + scoreStore) — strategy.md §A handler-compose 패턴.
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
// RubricStore + EvalItemStore + ScoreStore 동시 구현 (Phase B/C에서 BeginRubricTx 추가 예정).
func NewRubricHandler(rs store.RubricStore, es store.EvalItemStore, ss store.ScoreStore, logger *zap.Logger) *RubricHandler {
	return &RubricHandler{rubricStore: rs, evalItemStore: es, scoreStore: ss, logger: logger}
}

// Routes 등급 rubric 라우트 9개 등록한 http.Handler 반환 (Phase A skeleton).
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

// ── 9 핸들러 메서드 (Phase A skeleton — 모두 503 NOT_IMPLEMENTED) ─────────────

// handleCreateRubric POST /api/v1/rubrics (admin only, OPEN #1)
// @MX:TODO: Phase B/C GREEN — admin guard + InsertRubric + 201 응답
func (h *RubricHandler) handleCreateRubric(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleGetRubric GET /api/v1/rubrics/{id} (모든 인증)
// @MX:TODO: Phase C GREEN — UUID 파싱 + GetRubricByID + criteria/bands 임베드
func (h *RubricHandler) handleGetRubric(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleListRubrics GET /api/v1/rubrics (모든 인증)
// @MX:TODO: Phase C GREEN — filter(status/scope) + pagination + List/Count
func (h *RubricHandler) handleListRubrics(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleUpdateRubric PUT /api/v1/rubrics/{id} (admin only, OPEN #5)
// @MX:TODO: Phase C GREEN — OPEN #2 dual defense (active 1-per-(name,scope))
func (h *RubricHandler) handleUpdateRubric(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleArchiveRubric POST /api/v1/rubrics/{id}/archive (admin only, OPEN #7)
// @MX:TODO: Phase C GREEN — archive_reason 필수 검증 + ArchiveRubric
func (h *RubricHandler) handleArchiveRubric(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleAddCriterion POST /api/v1/rubrics/{id}/criteria (admin only, OPEN #3)
// @MX:TODO: Phase C GREEN — cross-store EvalItem 검증 + weight sum <=1.0 + AddCriterion
func (h *RubricHandler) handleAddCriterion(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleAddBand POST /api/v1/rubrics/{id}/bands (admin only, OPEN #4)
// @MX:TODO: Phase C GREEN — dual defense (handler pre-check overlap + DB EXCLUSION SQLSTATE 23P01)
func (h *RubricHandler) handleAddBand(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleApplyRubric POST /api/v1/rubrics/{id}/apply (모든 인증, cross-store 2-TX, OPEN #6)
// @MX:TODO: Phase C GREEN — TX-1 score sum (REPORT-001 미러) + TX-2 ApplyRubric read-only
//
// @MX:WARN: cross-store 2-TX — 2개 read-only TX defer Rollback 누락 시 커넥션 누수
// @MX:REASON: §A handler-compose — TX-1 score 합산, TX-2 rubric apply (race window=不發生 PoC)
func (h *RubricHandler) handleApplyRubric(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}

// handleCloneNewVersion POST /api/v1/rubrics/{id}/clone-new-version (admin only, OPEN #1)
// @MX:TODO: Phase C GREEN — REVIEW-001 supersede 패턴 미러 — version+1 + 빈 rubric 신규 생성
func (h *RubricHandler) handleCloneNewVersion(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusServiceUnavailable)
}
