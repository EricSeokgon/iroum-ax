# Sprint Contract — SPEC-AX-CTRL-001 Sprint 4 (REQ-CTRL-002 gRPC Server)

**Sprint ID**: Sprint 4 CTRL
**REQ**: REQ-CTRL-002 — gRPC Server on :50051
**Phase**: RED (2026-05-14)
**Priority Dimension**: Functionality (AC 달성률 우선)
**Harness Level**: thorough (per strategy.md)

---

## 1. Acceptance Checklist (Sprint Contract)

이 체크리스트는 evaluator-active가 GREEN 단계 평가 시 기준으로 사용하는 계약 목록이다.
각 항목은 구체적이고 자동화 테스트로 검증 가능해야 한다.

### AC-CTRL-002-1: CreateWorkflow RPC (Functionality — Must Pass)

- [ ] `CreateWorkflow(ctx, req)` 호출 시 에러 없이 `CreateWorkflowResponse` 반환
- [ ] 반환된 `resp.Workflow.Status == WorkflowStatusPending`
- [ ] 반환된 `resp.Workflow.ID`가 비어 있지 않음 (UUID 형식)
- [ ] `FakeStore.AuditLogs`에 `action=WORKFLOW_CREATED` 이벤트 정확히 1건
- [ ] audit INSERT 실패 시 `workflows` row도 롤백 (atomicity: AC-CTRL-UBI-001)
- [ ] 3개 고루틴 동시 CreateWorkflow 호출 시 모두 성공 + workflow_id 모두 고유

### AC-CTRL-002-2: GetWorkflow RPC (Functionality — Must Pass)

- [ ] 존재하는 ID 조회 시 해당 `Workflow` 반환, ID 필드 일치
- [ ] 존재하지 않는 ID 조회 시 `codes.NotFound` 반환
- [ ] `resp.Workflow.Status`가 `FakeStore`의 실제 상태와 일치

### AC-CTRL-002-3: ListWorkflows RPC (Functionality — Must Pass)

- [ ] 빈 스토어에서 ListWorkflows 호출 시 에러 없이 빈 목록 반환
- [ ] 스토어에 3개 존재 시 Limit=10 → 3개 반환
- [ ] 스토어에 5개 존재 시 Limit=2 → 정확히 2개 반환 (limit enforcement)
- [ ] 취소된 컨텍스트로 CreateWorkflow 호출 시 `codes.Canceled` 또는 `codes.DeadlineExceeded`

### AC-CTRL-002-1 startup: gRPC Server 등록/연결 (Functionality — Must Pass)

- [ ] `RegisterWorkflowServiceServer` + `grpc.Serve` 후 bufconn 연결 성공
- [ ] 연결된 `grpc.ClientConn`이 nil이 아님

### AC-CTRL-002-4: Prometheus 메트릭 (Optional — Nice to Have)

- [ ] `WorkflowService.RPCCallCounter()` 메서드 존재 (prometheus.CounterVec 반환)
- [ ] CreateWorkflow 3회 호출 후 카운터 값 = 3.0

---

## 2. Priority Dimension

**Priority**: Functionality

**근거**: Sprint 4는 gRPC 서버 핵심 CRUD 계약 확립이 목표.
REST 게이트웨이(Sprint 5), Celery dispatch(Sprint 6)의 진입점이 되므로
API 계약의 정확성이 가장 중요. Prometheus 메트릭은 Sprint 5 grpc-prometheus 미들웨어
통합 시 자동 충족 예정이므로 Optional로 분류.

---

## 3. Test Scenarios (bufconn 기반)

| 테스트명 | AC | 검증 포인트 |
|---------|-----|------------|
| TestWorkflowService_CreateWorkflow_HappyPath | AC-CTRL-002-1 | PENDING 상태 + audit 1건 |
| TestWorkflowService_CreateWorkflow_AuditFail_RollsBack | AC-CTRL-UBI-001 | rollback atomicity |
| TestWorkflowService_GetWorkflow_Found | AC-CTRL-002-2 | 정상 조회 + ID 일치 |
| TestWorkflowService_GetWorkflow_NotFound_ReturnsNotFoundStatus | AC-CTRL-002-2 | codes.NotFound |
| TestWorkflowService_ListWorkflows_EmptyStore | AC-CTRL-002-3 | 빈 목록 반환 |
| TestWorkflowService_ListWorkflows_WithMultiple | AC-CTRL-002-3 | N개 반환 |
| TestWorkflowService_ListWorkflows_LimitEnforcement | AC-CTRL-002-3 | limit=2 강제 |
| TestWorkflowService_CreateWorkflow_ContextCancelled_ReturnsCanceledStatus | AC-CTRL-002-3 | cancellation propagation |
| TestWorkflowService_CreateWorkflow_ConcurrentCalls_AllSucceed | AC-CTRL-002-2 | 3 goroutine 동시성 |
| TestWorkflowService_GRPCServer_RegisterAndConnect | AC-CTRL-002-1 startup | bufconn 서버 등록 |
| TestWorkflowService_GRPCServer_Serve_AcceptsConnection | AC-CTRL-002-1 startup | 연결 수락 |
| TestWorkflowService_PrometheusMetrics_RPCCounter | AC-CTRL-002-4 Optional | 메트릭 카운터 |

---

## 4. Pass Conditions (Sprint 4 GREEN 완료 기준)

- All Must Pass AC (AC-CTRL-002-1/2/3 + startup) 테스트 12개 중 9개 PASS
  (PrometheusMetrics 3개 Optional 제외 시 9개 Must Pass 기준)
- `go build ./apps/control-plane/...` PASS
- `go vet ./apps/control-plane/...` 0 에러
- `golangci-lint run ./apps/control-plane/...` 0 에러 (proto/server 패키지)
- Sprint 1+2+3 단위 테스트 40개 회귀 없음

---

## 5. Proto Codegen 결정

**방식**: Hand-written (protoc 부재 대안)

**이유**:
- `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc`가 PATH에 없음
- `buf` 미설치
- RED 단계 목적은 테스트 계약 확립이며, wire format 직렬화는 GREEN 단계 요건
- `workflow.pb.go`에는 Go 순수 struct 타입 선언만 포함 (proto.Message 인터페이스 미구현)
- `workflow_grpc.pb.go`에는 `WorkflowServiceServer` 인터페이스 + `grpc.ServiceDesc` + handler 어댑터 포함
- 테스트는 서비스 인터페이스 직접 호출 방식으로 wire format 이슈를 완전히 우회

**GREEN 단계 해결 계획**:
1. `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` 시도
2. `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` 시도
3. `schemas/proto/workflow.proto`에 WorkflowService 서비스 정의 추가
4. `buf generate` 실행 → `internal/proto/ax/v1/` 생성
5. 현재 hand-written 파일을 generated 파일로 교체

---

## 6. 파일 변경 이력

| 파일 | 유형 | 설명 |
|------|------|------|
| `apps/control-plane/internal/proto/workflow.pb.go` | 신규 | Hand-written proto 메시지 타입 |
| `apps/control-plane/internal/proto/workflow_grpc.pb.go` | 신규 | Hand-written gRPC 서비스 스텁 |
| `apps/control-plane/internal/server/grpc_server.go` | 신규 | WorkflowService 구현체 (Unimplemented 스텁) |
| `apps/control-plane/internal/server/grpc_server_test.go` | 신규 | Sprint 4 RED 테스트 (12개) |
| `.moai/sprints/SPEC-AX-CTRL-001/sprint-REQ-CTRL-002.md` | 신규 | 본 Sprint Contract |
| `.moai/specs/SPEC-AX-CTRL-001/progress.md` | 갱신 | Sprint 4 RED 진입 기록 |
