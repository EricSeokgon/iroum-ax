// rest_handler.go — REQ-CTRL-003 REST API 핸들러 (Option A: 직접 net/http)
// Sprint 5 RED: 모든 메서드가 http.StatusNotImplemented 반환 (GREEN에서 구현)
// grpc-gateway codegen 없이 stdlib net/http + encoding/json 사용
package server

import (
	"net/http"

	"go.uber.org/zap"
)

// RESTHandler WorkflowService를 감싸 REST API를 제공하는 핸들러
// WorkflowService(gRPC)를 in-process로 델리게이션하여 JSON/HTTP 변환을 처리한다.
//
// 필드 정렬: 포인터(8B) 순으로 배치 (fieldalignment 최적화)
//
// @MX:ANCHOR: [AUTO] REST API 진입점 — Mux(), Create, Get, List, Health 5개 핸들러가 이 struct를 공유
// @MX:REASON: grpc_server_test.go(Sprint 4), rest_handler_test.go(Sprint 5), cmd/server/server.go(Sprint 5 이후) 3곳에서 참조
type RESTHandler struct {
	// svc gRPC WorkflowService (in-process 델리게이션 대상)
	svc *WorkflowService
	// logger 구조화 zap 로거
	logger *zap.Logger
}

// NewRESTHandler RESTHandler 인스턴스 생성
//
// @MX:TODO: [AUTO] Sprint 5 GREEN에서 실제 핸들러 구현 후 이 태그 제거
func NewRESTHandler(svc *WorkflowService, logger *zap.Logger) *RESTHandler {
	return &RESTHandler{
		svc:    svc,
		logger: logger,
	}
}

// Mux RESTHandler의 모든 라우트를 등록한 http.Handler 반환
// Go 1.22+ http.ServeMux의 method+path 패턴 및 {id} 와일드카드를 사용한다.
//
// 등록 라우트:
//   - POST /api/v1/workflows     → handleCreateWorkflow
//   - GET  /api/v1/workflows     → handleListWorkflows
//   - GET  /api/v1/workflows/{id} → handleGetWorkflow
//   - GET  /health               → handleHealth
//
// @MX:ANCHOR: [AUTO] 모든 REST 라우트의 단일 등록 지점
// @MX:REASON: rest_handler_test.go(12개 테스트), cmd/server/server.go(서버 마운트), AC-CTRL-003-4(startup 검증) 3곳에서 호출
func (h *RESTHandler) Mux() http.Handler {
	mux := http.NewServeMux()

	// POST /api/v1/workflows — 워크플로우 생성 (AC-CTRL-003-1)
	mux.HandleFunc("POST /api/v1/workflows", h.handleCreateWorkflow)

	// GET /api/v1/workflows — 워크플로우 목록 조회 (AC-CTRL-003-3, 3b)
	mux.HandleFunc("GET /api/v1/workflows", h.handleListWorkflows)

	// GET /api/v1/workflows/{id} — 단건 조회 (AC-CTRL-003-2)
	mux.HandleFunc("GET /api/v1/workflows/{id}", h.handleGetWorkflow)

	// GET /health — 헬스체크 (AC-CTRL-003-3, AC-CTRL-003-4)
	mux.HandleFunc("GET /health", h.handleHealth)

	return mux
}

// handleCreateWorkflow POST /api/v1/workflows 처리
// AC-CTRL-003-1: 201 Created + Location 헤더 + JSON body
//
// @MX:TODO: [AUTO] Sprint 5 GREEN에서 구현
func (h *RESTHandler) handleCreateWorkflow(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// handleGetWorkflow GET /api/v1/workflows/{id} 처리
// AC-CTRL-003-2: 200 OK + JSON body; 404 if not found; 400 if invalid UUID
//
// @MX:TODO: [AUTO] Sprint 5 GREEN에서 구현
func (h *RESTHandler) handleGetWorkflow(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// handleListWorkflows GET /api/v1/workflows 처리
// AC-CTRL-003-3: 200 OK + JSON array; ?limit=&offset= 쿼리 파라미터 지원
// AC-CTRL-003-3b: ?limit=invalid → 400 Bad Request
//
// @MX:TODO: [AUTO] Sprint 5 GREEN에서 구현
func (h *RESTHandler) handleListWorkflows(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// handleHealth GET /health 처리
// AC-CTRL-003-3: 200 OK + {"status":"ok"} (최소 형식; 실제 DB/Redis 체크는 GREEN에서)
//
// @MX:TODO: [AUTO] Sprint 5 GREEN에서 구현
func (h *RESTHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
