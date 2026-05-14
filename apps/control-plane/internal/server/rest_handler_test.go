// rest_handler_test.go — RESTHandler RED 단계 테스트
// Sprint 5 RED: 모든 핸들러가 501 Not Implemented를 반환하므로 테스트가 실패 (RED 정상)
//
// 테스트 전략:
//   - httptest.NewServer: in-process HTTP 서버로 네트워크 오버헤드 없이 검증
//   - stdlib http.Client: 서드파티 없이 요청 전송
//   - FakeStore: Sprint 1 인메모리 스토어로 pgx 없이 단위 격리
//
// RED 실패 이유:
//   - 모든 핸들러가 http.StatusNotImplemented(501)를 반환
//   - 테스트는 201/200/400/404 등 실제 상태 코드를 기대 → 실패 (RED 정상)
//
// AC 커버리지:
//   - AC-CTRL-003-1: POST /api/v1/workflows → 201 + Location + JSON body
//   - AC-CTRL-003-2: GET /api/v1/workflows/{id} → 200/404/400
//   - AC-CTRL-003-3: GET /api/v1/workflows → 200 + JSON array
//   - AC-CTRL-003-3b: GET /api/v1/workflows?limit=invalid → 400
//   - AC-CTRL-003-4: REST 게이트웨이 기동 < 2s
package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/server"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestRESTHandler 테스트용 RESTHandler + FakeStore 조합을 반환
// newTestService(grpc_server_test.go)와 동일한 팩토리 패턴
func newTestRESTHandler(t *testing.T) (*server.RESTHandler, *store.FakeStore) {
	t.Helper()
	svc, fakeStore := newTestService(t)
	handler := server.NewRESTHandler(svc, zap.NewNop())
	return handler, fakeStore
}

// seedRESTWorkflow FakeStore에 테스트용 Workflow를 직접 삽입
func seedRESTWorkflow(t *testing.T, s *store.FakeStore, state types.WorkflowState) *types.Workflow {
	t.Helper()
	wf := &types.Workflow{
		ID:         uuid.New(),
		DocumentID: uuid.New(),
		State:      state,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	s.SeedWorkflow(wf)
	return wf
}

// doRequest 테스트용 HTTP 요청을 전송하고 응답을 반환하는 헬퍼
func doRequest(t *testing.T, client *http.Client, method, url, body string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, bodyReader)
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// -------------------------------------------------------------------
// AC-CTRL-003-1: POST /api/v1/workflows
// -------------------------------------------------------------------

// TestRESTHandler_CreateWorkflow_Success POST 성공 시 201 Created + Location 헤더 + JSON body 검증
// AC-CTRL-003-1: "POST /api/v1/workflows" → 201 + body.workflow_id(uuid) + Location header
func TestRESTHandler_CreateWorkflow_Success(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	validDocID := uuid.New().String()
	body := fmt.Sprintf(`{"document_id":%q}`, validDocID)
	resp := doRequest(t, ts.Client(), http.MethodPost, ts.URL+"/api/v1/workflows", body)
	defer resp.Body.Close()

	// RED 실패 예상: 현재 501이 반환됨
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "POST /api/v1/workflows should return 201 Created")

	// Location 헤더 검증
	location := resp.Header.Get("Location")
	assert.True(t, strings.HasPrefix(location, "/api/v1/workflows/"), "Location header should be set to /api/v1/workflows/<uuid>")

	// JSON 응답 바디 검증
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	workflowID, ok := result["workflow_id"].(string)
	assert.True(t, ok, "response should contain workflow_id string")
	_, err := uuid.Parse(workflowID)
	assert.NoError(t, err, "workflow_id should be a valid UUID")
	assert.Equal(t, "PENDING", result["status"], "initial status should be PENDING")
}

// TestRESTHandler_CreateWorkflow_MissingDocumentID_400 document_id 누락 시 400 반환 검증
// AC-CTRL-003-1 edge: 빈 document_id → 400 Bad Request
func TestRESTHandler_CreateWorkflow_MissingDocumentID_400(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	resp := doRequest(t, ts.Client(), http.MethodPost, ts.URL+"/api/v1/workflows", `{}`)
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "missing document_id should return 400")

	var errBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
	errObj, ok := errBody["error"].(map[string]interface{})
	assert.True(t, ok, "response should contain error object")
	assert.Equal(t, "INVALID_ARGUMENT", errObj["code"], "error code should be INVALID_ARGUMENT")
}

// TestRESTHandler_CreateWorkflow_MalformedJSON_400 비정형 JSON 요청 시 400 반환 검증
// AC-CTRL-003-1 edge: 파싱 불가 바디 → 400 Bad Request
func TestRESTHandler_CreateWorkflow_MalformedJSON_400(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	resp := doRequest(t, ts.Client(), http.MethodPost, ts.URL+"/api/v1/workflows", `not-json`)
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "malformed JSON should return 400")
}

// -------------------------------------------------------------------
// AC-CTRL-003-2: GET /api/v1/workflows/{id}
// -------------------------------------------------------------------

// TestRESTHandler_GetWorkflow_Success 존재하는 워크플로우 조회 시 200 OK + JSON body 검증
// AC-CTRL-003-2: GET /api/v1/workflows/{id} → 200 + workflow JSON
func TestRESTHandler_GetWorkflow_Success(t *testing.T) {
	t.Parallel()
	handler, fakeStore := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	// 사전 조건: FakeStore에 워크플로우 삽입
	seeded := seedRESTWorkflow(t, fakeStore, types.WorkflowStatePending)

	resp := doRequest(t, ts.Client(), http.MethodGet,
		ts.URL+"/api/v1/workflows/"+seeded.ID.String(), "")
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusOK, resp.StatusCode, "existing workflow should return 200")

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, seeded.ID.String(), result["workflow_id"], "workflow_id should match")
}

// TestRESTHandler_GetWorkflow_NotFound_404 존재하지 않는 워크플로우 조회 시 404 반환 검증
// AC-CTRL-003-2 edge: 없는 ID → 404 Not Found
func TestRESTHandler_GetWorkflow_NotFound_404(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	nonExistentID := uuid.New().String()
	resp := doRequest(t, ts.Client(), http.MethodGet,
		ts.URL+"/api/v1/workflows/"+nonExistentID, "")
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "nonexistent workflow should return 404")
}

// TestRESTHandler_GetWorkflow_InvalidUUID_400 비UUID 형식의 ID 조회 시 400 반환 검증
// AC-CTRL-003-2 edge: 파싱 불가 UUID → 400 Bad Request
func TestRESTHandler_GetWorkflow_InvalidUUID_400(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	resp := doRequest(t, ts.Client(), http.MethodGet,
		ts.URL+"/api/v1/workflows/not-a-uuid", "")
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "invalid UUID should return 400")
}

// -------------------------------------------------------------------
// AC-CTRL-003-3: GET /api/v1/workflows (목록 조회)
// -------------------------------------------------------------------

// TestRESTHandler_ListWorkflows_Empty 빈 스토어에서 목록 조회 시 200 + 빈 배열 검증
// AC-CTRL-003-3: 비어 있는 경우 → 200 OK + {"workflows":[],"total":0}
func TestRESTHandler_ListWorkflows_Empty(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	resp := doRequest(t, ts.Client(), http.MethodGet, ts.URL+"/api/v1/workflows", "")
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusOK, resp.StatusCode, "empty store should return 200")

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	workflows, ok := result["workflows"].([]interface{})
	assert.True(t, ok, "response should contain workflows array")
	assert.Len(t, workflows, 0, "empty store should return empty array")
}

// TestRESTHandler_ListWorkflows_WithMultiple N개 워크플로우 존재 시 200 + N개 반환 검증
// AC-CTRL-003-3: 3개 삽입 → 200 OK + 3개 배열
func TestRESTHandler_ListWorkflows_WithMultiple(t *testing.T) {
	t.Parallel()
	handler, fakeStore := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	// 사전 조건: 3개 워크플로우 삽입
	for range 3 {
		seedRESTWorkflow(t, fakeStore, types.WorkflowStatePending)
	}

	resp := doRequest(t, ts.Client(), http.MethodGet, ts.URL+"/api/v1/workflows", "")
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 with multiple workflows")

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	workflows, ok := result["workflows"].([]interface{})
	assert.True(t, ok, "response should contain workflows array")
	assert.Len(t, workflows, 3, "should return all 3 seeded workflows")
}

// TestRESTHandler_ListWorkflows_LimitOffsetQuery limit/offset 쿼리 파라미터 동작 검증
// AC-CTRL-003-3: ?limit=2&offset=1 → 5개 중 2개 반환
func TestRESTHandler_ListWorkflows_LimitOffsetQuery(t *testing.T) {
	t.Parallel()
	handler, fakeStore := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	// 사전 조건: 5개 워크플로우 삽입
	for range 5 {
		seedRESTWorkflow(t, fakeStore, types.WorkflowStatePending)
	}

	resp := doRequest(t, ts.Client(), http.MethodGet,
		ts.URL+"/api/v1/workflows?limit=2&offset=1", "")
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusOK, resp.StatusCode, "limit/offset query should return 200")

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	workflows, ok := result["workflows"].([]interface{})
	assert.True(t, ok, "response should contain workflows array")
	assert.Len(t, workflows, 2, "limit=2 should return exactly 2 workflows")
}

// -------------------------------------------------------------------
// AC-CTRL-003-3b: 잘못된 limit 파라미터
// -------------------------------------------------------------------

// TestRESTHandler_ListWorkflows_InvalidLimit_400 비정수 limit 파라미터 시 400 반환 검증
// AC-CTRL-003-3b: ?limit=invalid → 400 Bad Request
func TestRESTHandler_ListWorkflows_InvalidLimit_400(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	resp := doRequest(t, ts.Client(), http.MethodGet,
		ts.URL+"/api/v1/workflows?limit=invalid", "")
	defer resp.Body.Close()

	// RED 실패 예상 — AC-CTRL-003-3b
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "invalid limit param should return 400")
}

// -------------------------------------------------------------------
// AC-CTRL-003-3: GET /health
// -------------------------------------------------------------------

// TestRESTHandler_Health_OK 헬스체크 엔드포인트 200 OK + JSON body 검증
// AC-CTRL-003-3: GET /health → 200 OK + {"status":"ok"}
func TestRESTHandler_Health_OK(t *testing.T) {
	t.Parallel()
	handler, _ := newTestRESTHandler(t)
	ts := httptest.NewServer(handler.Mux())
	defer ts.Close()

	resp := doRequest(t, ts.Client(), http.MethodGet, ts.URL+"/health", "")
	defer resp.Body.Close()

	// RED 실패 예상
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /health should return 200")

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"], "health response should have status=ok")
}

// -------------------------------------------------------------------
// AC-CTRL-003-4: REST 게이트웨이 기동 시간 < 2s
// -------------------------------------------------------------------

// TestRESTHandler_StartupTime_Under2s REST 서버가 2초 이내에 /health에 응답하는지 검증
// AC-CTRL-003-4: 서버 기동 후 GET /health 최초 200 응답 시각 - 기동 시각 < 2s
//
// 주의: httptest.NewServer는 동기적으로 기동하므로 실제 비동기 기동 테스트를 시뮬레이션하기 위해
// net.Listen으로 소켓을 확보한 후 goroutine에서 http.Serve를 호출하는 방식을 사용한다.
func TestRESTHandler_StartupTime_Under2s(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService(t)
	handler := server.NewRESTHandler(svc, zap.NewNop())

	// 소켓 먼저 확보 (사용 가능한 포트 자동 할당)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	// t0: 기동 시작 시각
	t0 := time.Now()

	// 백그라운드에서 HTTP 서버 기동
	srv := &http.Server{Handler: handler.Mux()} //nolint:gosec
	go func() {
		// Serve는 ln이 Close()될 때 반환
		_ = srv.Serve(ln)
	}()
	defer func() { _ = srv.Close() }()

	// /health 폴링 (100ms 간격, 최대 2s)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	healthURL := "http://" + addr + "/health"

	var t1 time.Time
	var lastStatus int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, healthURL, nil)
		if reqErr != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		resp, doErr := client.Do(req)
		if doErr == nil {
			lastStatus = resp.StatusCode
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t1 = time.Now()
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// RED 실패 예상: 현재 /health가 501을 반환하므로 t1이 zero value
	// GREEN 단계에서 handleHealth가 200을 반환하면 이 어설션이 통과됨
	require.False(t, t1.IsZero(), "GET /health should return 200 within 2s (last status: %d)", lastStatus)
	elapsed := t1.Sub(t0)
	assert.Less(t, elapsed, 2*time.Second,
		"REST gateway should be ready within 2s, actual: %v", elapsed)
}
