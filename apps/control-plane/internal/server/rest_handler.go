// rest_handler.go — REQ-CTRL-003 REST API 핸들러 (Option A: 직접 net/http)
// Sprint 5 GREEN: stdlib net/http + encoding/json으로 모든 핸들러 구현
// grpc-gateway codegen 없이 WorkflowService(gRPC)를 in-process 델리게이션
package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	proto "github.com/ircp/iroum-ax/apps/control-plane/internal/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
func NewRESTHandler(svc *WorkflowService, logger *zap.Logger) *RESTHandler {
	return &RESTHandler{
		svc:    svc,
		logger: logger,
	}
}

// workflowResponse REST API 응답 DTO
// 테스트가 기대하는 필드명(workflow_id, status)에 맞춘 JSON 태그 사용
type workflowResponse struct {
	// WorkflowID UUID 문자열
	WorkflowID string `json:"workflow_id"`
	// Status 문자열 상태 ("PENDING", "RUNNING" 등)
	Status string `json:"status"`
	// DocumentID 연결된 문서 ID
	DocumentID string `json:"document_id,omitempty"`
}

// listWorkflowsResponse ListWorkflows 응답 DTO
type listWorkflowsResponse struct {
	// Workflows 워크플로우 목록
	Workflows []workflowResponse `json:"workflows"`
	// Total 전체 개수 (현재 반환된 수와 동일 — 향후 count 쿼리로 교체 가능)
	Total int32 `json:"total"`
}

// errorDetail REST 에러 응답 내부 객체
type errorDetail struct {
	// Code 에러 코드 문자열 ("INVALID_ARGUMENT", "NOT_FOUND" 등)
	Code string `json:"code"`
	// Message 에러 메시지
	Message string `json:"message,omitempty"`
}

// errorResponse REST 에러 응답 DTO
type errorResponse struct {
	// Error 에러 상세 정보
	Error errorDetail `json:"error"`
}

// healthResponse GET /health 응답 DTO
type healthResponse struct {
	// Status 상태 문자열 ("ok")
	Status string `json:"status"`
	// Service 서비스 식별자
	Service string `json:"service"`
	// Version 버전 문자열
	Version string `json:"version"`
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

// writeJSON Content-Type 헤더 설정 후 JSON 직렬화하여 응답 전송
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Encode 실패는 헤더가 이미 전송된 후이므로 로깅만 가능
		_ = err
	}
}

// writeError gRPC 상태 코드에서 HTTP 에러 응답 생성
func writeError(w http.ResponseWriter, httpCode int, grpcCodeStr, message string) {
	writeJSON(w, httpCode, errorResponse{
		Error: errorDetail{
			Code:    grpcCodeStr,
			Message: message,
		},
	})
}

// grpcCodeToHTTP gRPC status code를 HTTP status code로 변환
func grpcCodeToHTTP(c codes.Code) int {
	switch c {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.Canceled:
		// 499 Client Closed Request (비표준, nginx 관례)
		return 499
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// grpcCodeToString gRPC status code를 REST 에러 코드 문자열로 변환
func grpcCodeToString(c codes.Code) string {
	switch c {
	case codes.InvalidArgument:
		return "INVALID_ARGUMENT"
	case codes.NotFound:
		return "NOT_FOUND"
	case codes.Canceled:
		return "CANCELED"
	case codes.Internal:
		return "INTERNAL"
	case codes.AlreadyExists:
		return "ALREADY_EXISTS"
	default:
		return "INTERNAL"
	}
}

// protoWorkflowToResponse proto.Workflow를 workflowResponse DTO로 변환
func protoWorkflowToResponse(w *proto.Workflow) workflowResponse {
	return workflowResponse{
		WorkflowID: w.ID,
		Status:     w.Status.String(),
		DocumentID: w.DocumentID,
	}
}

// handleCreateWorkflow POST /api/v1/workflows 처리
// AC-CTRL-003-1: 201 Created + Location 헤더 + JSON body
func (h *RESTHandler) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	// JSON 요청 바디 디코딩
	var req proto.CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "요청 바디 파싱 실패: "+err.Error())
		return
	}

	// document_id 필수 검증 (빈 문자열도 InvalidArgument)
	if req.DocumentID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "document_id는 필수 항목입니다")
		return
	}

	// gRPC 서비스 호출 (in-process)
	resp, err := h.svc.CreateWorkflow(r.Context(), &req)
	if err != nil {
		st, _ := status.FromError(err)
		httpCode := grpcCodeToHTTP(st.Code())
		codeStr := grpcCodeToString(st.Code())
		writeError(w, httpCode, codeStr, st.Message())
		return
	}

	// 성공: 201 Created + Location 헤더
	wf := resp.Workflow
	w.Header().Set("Location", "/api/v1/workflows/"+wf.ID)
	writeJSON(w, http.StatusCreated, protoWorkflowToResponse(wf))
}

// handleGetWorkflow GET /api/v1/workflows/{id} 처리
// AC-CTRL-003-2: 200 OK + JSON body; 404 if not found; 400 if invalid UUID
func (h *RESTHandler) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	// Go 1.22+ path param 추출
	id := r.PathValue("id")

	// UUID 형식 검증
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "유효하지 않은 UUID 형식: "+id)
		return
	}

	// gRPC 서비스 호출
	resp, err := h.svc.GetWorkflow(r.Context(), &proto.GetWorkflowRequest{ID: id})
	if err != nil {
		st, _ := status.FromError(err)
		httpCode := grpcCodeToHTTP(st.Code())
		codeStr := grpcCodeToString(st.Code())
		writeError(w, httpCode, codeStr, st.Message())
		return
	}

	writeJSON(w, http.StatusOK, protoWorkflowToResponse(resp.Workflow))
}

// handleListWorkflows GET /api/v1/workflows 처리
// AC-CTRL-003-3: 200 OK + JSON array; ?limit=&offset= 쿼리 파라미터 지원
// AC-CTRL-003-3b: ?limit=invalid → 400 Bad Request
func (h *RESTHandler) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// limit 파라미터 파싱 (기본값 100)
	limit := int32(100)
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "limit 파라미터가 정수가 아닙니다: "+raw)
			return
		}
		limit = int32(v) //nolint:gosec
	}

	// offset 파라미터 파싱 (기본값 0)
	offset := int32(0)
	if raw := q.Get("offset"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "offset 파라미터가 정수가 아닙니다: "+raw)
			return
		}
		offset = int32(v) //nolint:gosec
	}

	// gRPC 서비스 호출
	resp, err := h.svc.ListWorkflows(r.Context(), &proto.ListWorkflowsRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		st, _ := status.FromError(err)
		httpCode := grpcCodeToHTTP(st.Code())
		codeStr := grpcCodeToString(st.Code())
		writeError(w, httpCode, codeStr, st.Message())
		return
	}

	// proto.Workflow 슬라이스 → DTO 슬라이스 변환
	// 빈 배열도 `[]`로 직렬화되도록 명시적으로 초기화
	dtos := make([]workflowResponse, 0, len(resp.Workflows))
	for _, wf := range resp.Workflows {
		dtos = append(dtos, protoWorkflowToResponse(wf))
	}

	writeJSON(w, http.StatusOK, listWorkflowsResponse{
		Workflows: dtos,
		Total:     resp.Total,
	})
}

// handleHealth GET /health 처리
// AC-CTRL-003-3: 200 OK + {"status":"ok",...}
// AC-CTRL-003-4: 이 핸들러는 동기적으로 즉시 반환하여 startup < 2s 보장
func (h *RESTHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "iroum-ax-control-plane",
		Version: "0.1.0",
	})
}
