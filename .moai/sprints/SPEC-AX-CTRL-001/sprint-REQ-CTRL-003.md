# Sprint Contract — REQ-CTRL-003 REST API

> Harness 레벨: thorough
> Sprint: Sprint 5 (SPEC-AX-CTRL-001)
> 작성일: 2026-05-14
> 작성자: manager-tdd

---

## 1. Priority Dimension

**Functionality + UX (REST는 사람이 직접 호출하는 표면)**

REST API는 CLI/gRPC와 달리 브라우저·curl·외부 연동 클라이언트가 직접 소비하는 인터페이스다.
HTTP 상태 코드, Location 헤더, 오류 응답 바디 형식이 곧 UX이므로 두 차원을 동시에 1순위로 설정한다.

---

## 2. Acceptance Checklist (Sprint 5)

| # | AC ID | 테스트 함수 | 판정 기준 | Pass 조건 |
|---|-------|-----------|---------|---------|
| 1 | AC-CTRL-003-1 | TestRESTHandler_CreateWorkflow_Success | POST → 201 + Location + JSON body | `status == 201 && body.workflow_id valid UUID && Location header set` |
| 2 | AC-CTRL-003-1 | TestRESTHandler_CreateWorkflow_MissingDocumentID_400 | 빈 document_id → 400 | `status == 400 && body.error.code == "INVALID_ARGUMENT"` |
| 3 | AC-CTRL-003-1 | TestRESTHandler_CreateWorkflow_MalformedJSON_400 | 비정형 JSON → 400 | `status == 400` |
| 4 | AC-CTRL-003-2 | TestRESTHandler_GetWorkflow_Success | GET existing → 200 + JSON | `status == 200 && body.workflow_id matches` |
| 5 | AC-CTRL-003-2 | TestRESTHandler_GetWorkflow_NotFound_404 | 없는 ID → 404 | `status == 404` |
| 6 | AC-CTRL-003-2 | TestRESTHandler_GetWorkflow_InvalidUUID_400 | 비UUID ID → 400 | `status == 400` |
| 7 | AC-CTRL-003-3 | TestRESTHandler_ListWorkflows_Empty | 빈 스토어 → 200 + [] | `status == 200 && len(workflows) == 0` |
| 8 | AC-CTRL-003-3 | TestRESTHandler_ListWorkflows_WithMultiple | N개 스토어 → 200 + N | `status == 200 && len(workflows) == N` |
| 9 | AC-CTRL-003-3 | TestRESTHandler_ListWorkflows_LimitOffsetQuery | limit/offset 쿼리 → 올바른 slice | `status == 200 && correct subset returned` |
| 10 | AC-CTRL-003-3b | TestRESTHandler_ListWorkflows_InvalidLimit_400 | limit=invalid → 400 | `status == 400` |
| 11 | AC-CTRL-003-3 | TestRESTHandler_Health_OK | GET /health → 200 + JSON | `status == 200 && body.status == "ok"` |
| 12 | AC-CTRL-003-4 | TestRESTHandler_StartupTime_Under2s | 서버 기동 → /health 200 < 2s | `elapsed < 2s` |

---

## 3. Implementation Approach

**Option A: 직접 net/http 핸들러 (codegen 미의존)**

- `RESTHandler` struct가 `*WorkflowService` (gRPC 서비스)를 내부적으로 호출
- Go 1.22+ `http.ServeMux` 패턴 매칭 (`{id}` wildcard 지원)
- JSON 직렬화: `encoding/json`
- 테스트: `httptest.NewServer` in-process HTTP

grpc-gateway codegen 대신 Option A를 선택한 이유:
1. `protoc-gen-grpc-gateway` 바이너리가 환경에 미설치 → codegen 단계 없이 즉시 테스트 작성 가능
2. Walking Skeleton 범위에서 in-process 델리게이션이 더 단순하고 테스트 격리가 용이
3. 실제 grpc-gateway는 buf generate 환경이 갖춰진 후 별도 Sprint에서 도입 가능

---

## 4. Test Scenarios (httptest 기반)

```
서버: httptest.NewServer(handler.Mux())
클라이언트: http.Client{Timeout: 5s}

시나리오 1: POST /api/v1/workflows {"document_id":"<valid-uuid>"} → 201
시나리오 2: POST /api/v1/workflows {} → 400 (missing document_id)
시나리오 3: POST /api/v1/workflows "not-json" → 400
시나리오 4: GET /api/v1/workflows/<existing-id> → 200
시나리오 5: GET /api/v1/workflows/<nonexistent-id> → 404
시나리오 6: GET /api/v1/workflows/not-a-uuid → 400
시나리오 7: GET /api/v1/workflows (empty store) → 200 + []
시나리오 8: GET /api/v1/workflows (3 workflows seeded) → 200 + 3 items
시나리오 9: GET /api/v1/workflows?limit=2&offset=1 → 200 + correct slice
시나리오 10: GET /api/v1/workflows?limit=invalid → 400
시나리오 11: GET /health → 200 + {"status":"ok"}
시나리오 12: goroutine에서 서버 기동 → /health 폴링 → 2s 이내 200
```

---

## 5. Pass Conditions (per criterion)

- status 코드 정확히 일치 (201/200/400/404 — 범위 아님)
- Location 헤더: `/api/v1/workflows/<uuid>` 형식
- 오류 응답 body: `{"error":{"code":"<CODE>","message":"..."}}`
- 목록 응답 body: `{"workflows":[...],"total":<N>}`
- 스타트업 시간: `time.Since(t0) < 2*time.Second`

---

## 6. Sprint Contract 메타

- **사전 조건**: Sprint 4 GREEN 완료 (WorkflowService 비즈니스 로직 구현, grpc_server.go)
- **금지 사항**: grpc-gateway codegen, gorilla/mux, chi 등 서드파티 라우터
- **의존 인터페이스**: `store.WorkflowStore`, `store.FakeStore` (Sprint 1~3 산출물)
- **@MX 태그**: `RESTHandler.Mux()` fan_in >= 3 예상 → `@MX:ANCHOR` 추가
