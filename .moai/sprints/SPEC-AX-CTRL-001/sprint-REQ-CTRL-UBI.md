# Sprint Contract — SPEC-AX-CTRL-001 Sprint 1: REQ-CTRL-UBI-001/002

> Harness level: thorough
> Priority dimension: **Security** (트랜잭션 원자성 + 감사 일관성 = constitutional invariant)
> Created: 2026-05-14
> Phase: RED (현재) → GREEN → REFACTOR

---

## 1. Acceptance Checklist

- [ ] [AC-CTRL-UBI-001-A] audit INSERT 실패 시 workflow INSERT도 rollback (Scenario A)
- [ ] [AC-CTRL-UBI-001-B] audit INSERT 성공 후 workflow INSERT 실패 시 둘 다 rollback (Scenario B)
- [ ] [AC-CTRL-UBI-001-C] RUNNING→COMPLETED 전이 중 audit INSERT 실패 → 둘 다 rollback (Scenario C)
- [ ] [AC-CTRL-UBI-002-A] WORKFLOW_CREATED 감사 이벤트 완전성 (action, resource_type, user_id, details)
- [ ] [AC-CTRL-UBI-002-B] WORKFLOW_TRANSITIONED_TO_RUNNING 감사 이벤트 완전성 (details JSONB에 from/to 포함)
- [ ] [AC-CTRL-UBI-002-C] REST/gRPC 요청에서 인증 헤더 없을 때 user_id = 'cli-anonymous'

---

## 2. Test Scenarios

### store/fake_store_test.go (6-8 tests)

| 테스트 함수 | 검증 내용 |
|------------|---------|
| `TestFakeStore_InsertWorkflow_Success` | 정상 INSERT 후 in-memory 반영 |
| `TestFakeStore_InsertAuditLog_Success` | 정상 감사 INSERT 후 in-memory 반영 |
| `TestFakeStore_Tx_Commit_PersistsBoth` | Commit 후 양쪽 row 영속 |
| `TestFakeStore_Tx_Rollback_RemovesBoth` | Rollback 후 양쪽 row 없음 |
| `TestFakeTx_FailOnAuditInsert_RollsBack` | FailOnAuditInsert=true 시 rollback |
| `TestFakeTx_FailOnWorkflowInsert_RollsBack` | FailOnWorkflowInsert=true 시 rollback |
| `TestFakeStore_BeginTx_ReturnsNewTx` | BeginTx 정상 반환 |
| `TestFakeStore_MultiTx_Independent` | 두 Tx가 서로 독립적 |

### audit/recorder_test.go (8-12 tests)

| 테스트 함수 | 검증 내용 |
|------------|---------|
| `TestRecorder_RecordCreated_InsertsAction` | WORKFLOW_CREATED action 삽입 확인 |
| `TestRecorder_RecordCreated_ResourceType` | resource_type = 'workflow' |
| `TestRecorder_RecordCreated_UserID` | user_id = 요청의 user_id |
| `TestRecorder_RecordTransition_ActionName` | WORKFLOW_TRANSITIONED_TO_RUNNING action |
| `TestRecorder_RecordTransition_Details` | details JSONB에 {"from":"PENDING","to":"RUNNING"} |
| `TestRecorder_DefaultUserID_CliAnonymous` | userID 빈 문자열 시 cli-anonymous 적용 |
| `TestRecorder_RecordCreated_DetailsContainsDocumentID` | details에 document_id 포함 |
| `TestRecorder_AllActions_Covered` | 8종 Action enum 모두 string 검증 |
| `TestRecorder_RecordTransition_UserIDPropagated` | userID 전파 |
| `TestRecorder_RecordCompleted_Action` | WORKFLOW_COMPLETED action |

### workflow/transaction_test.go (5-8 tests)

| 테스트 함수 | 검증 내용 |
|------------|---------|
| `TestExecuteWorkflowCreate_Success_BothCommitted` | 정상 경로: workflow + audit 모두 commit |
| `TestExecuteWorkflowCreate_AuditFail_BothRolledBack` | audit 실패 → 양쪽 rollback (UBI-001 Scenario A) |
| `TestExecuteWorkflowCreate_WorkflowFail_BothRolledBack` | workflow 실패 → 양쪽 rollback (UBI-001 Scenario B) |
| `TestExecuteWorkflowTransition_Success_BothCommitted` | 전이 성공 시 양쪽 commit |
| `TestExecuteWorkflowTransition_AuditFail_BothRolledBack` | 전이 중 audit 실패 → 양쪽 rollback (UBI-001 Scenario C) |
| `TestExecuteWorkflowCreate_CliAnonymous_Default` | 인증 없을 때 cli-anonymous (UBI-002-C) |
| `TestExecuteWorkflowCreate_UserID_FromRequest` | user_id 요청에서 전파 |

---

## 3. Pass Conditions

- 모든 RED 테스트 컴파일 성공 (compilation errors 0)
- 모든 RED 테스트 FAIL (assert가 stub 반환값에 의해 실패)
- `go vet ./apps/control-plane/...` 0 error
- `golangci-lint run ./apps/control-plane/...` 0 error
- coverage target: ≥ 85% (GREEN 완료 시 기준)

---

## 4. Evaluator 4-Dimension Scoring (Sprint 1)

| Dimension | Weight | Target Score |
|-----------|--------|-------------|
| Security | 45% | ≥ 0.75 |
| Functionality | 30% | ≥ 0.75 |
| Completeness | 15% | ≥ 0.75 |
| Originality | 10% | ≥ 0.75 |

Overall pass threshold: ≥ 0.75 (strict profile per thorough harness)

---

## 5. Sprint 1 Artifact Paths

| 파일 | 역할 |
|------|------|
| `apps/control-plane/internal/store/store.go` | Store + Tx 인터페이스 |
| `apps/control-plane/internal/store/fake_store.go` | 테스트용 FakeStore + FakeTx |
| `apps/control-plane/internal/store/fake_store_test.go` | RED tests (6-8) |
| `apps/control-plane/internal/audit/recorder.go` | Recorder 타입 stub |
| `apps/control-plane/internal/audit/recorder_test.go` | RED tests (8-12) |
| `apps/control-plane/internal/workflow/transaction.go` | Tx 코디네이터 stub |
| `apps/control-plane/internal/workflow/transaction_test.go` | RED tests (5-8) |

---

Sprint Contract version: 1.0.0
