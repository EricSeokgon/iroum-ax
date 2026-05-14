# Sprint Contract — REQ-CTRL-004: PostgreSQL Store

> SPEC: SPEC-AX-CTRL-001 v0.1.2
> Sprint: 3 (REQ-CTRL-004 PostgreSQL Store)
> Phase: RED (완료)
> 작성일: 2026-05-14
> Harness Level: thorough (Sprint Contract 필수)
> Priority Dimension: **Functionality + Security** (데이터 저장 계층 — 원자성, 직렬화, 롤백)

---

## 1. Sprint 목표

pgx v5 기반 PgWorkflowStore 구현 — testcontainers-go postgres:16-alpine 통합 테스트로 검증.

## 2. Acceptance Checklist (Sprint 3 범위)

| AC | 설명 | 테스트 함수 | 상태 |
|----|------|-------------|------|
| AC-CTRL-004-1 | 잘못된 DSN → 5초 이내 에러 반환 | TestPgStore_Integration_NewPgWorkflowStore_InvalidDSN | RED |
| AC-CTRL-004-1 | workflows 행 INSERT + SELECT 왕복 | TestPgStore_Integration_InsertAndGetWorkflow | RED |
| AC-CTRL-004-2 | audit_logs JSONB details INSERT | TestPgStore_Integration_InsertAuditLog_WithJSONDetails | RED |
| AC-CTRL-004-2 | 8종 Action 타입 audit_logs 삽입 | TestPgStore_Integration_InsertAuditLog_AllActionTypes | RED |
| AC-CTRL-004-3 | SELECT FOR UPDATE 동시 전이 직렬화 | TestPgStore_Integration_SelectForUpdate_ConcurrentTransition | RED |
| AC-CTRL-004-3 | 풀 고갈 시 BeginTx 에러 반환 | TestPgStore_Integration_BeginTx_PoolExhausted_ReturnsError | RED |
| AC-CTRL-004-4 | mid-tx 실패(23505) → 전체 롤백 | TestPgStore_Integration_MidTxFailure_Rollback | RED |
| AC-CTRL-UBI-001 | audit 실패 시 workflow INSERT도 롤백 | TestPgStore_Integration_AtomicWorkflowAndAudit_AuditFailRollsBackWorkflow | RED |
| 추가 | UpdateWorkflowState 단일 행 영향 | TestPgStore_Integration_UpdateWorkflowState_AffectsExactlyOneRow | RED |
| 추가 | UpdateWorkflowResult JSONB 저장 | TestPgStore_Integration_UpdateWorkflowResult_StoresJSONB | RED |
| 추가 | GetWorkflow NotFound 에러 반환 | TestPgStore_Integration_GetWorkflow_NotFound_ReturnsError | RED |

**총 통합 테스트**: 11개 (TestPgStore_Integration_* 11개 / TestPgStore_Integration_InsertAuditLog_AllActionTypes는 서브테스트 8개 포함)

## 3. Priority Dimension

**Functionality** (주): InsertWorkflow/GetWorkflow/UpdateWorkflow 왕복 정확성, SELECT FOR UPDATE 직렬화
**Security** (부): 트랜잭션 원자성 — 부분 커밋 금지, 롤백 일관성

## 4. Test Scenarios (GREEN phase 검증 기준)

### 4.1 INSERT + SELECT 왕복 (AC-CTRL-004-1)
- 입력: Workflow{ID: uuid.New(), State: PENDING, DocumentID: uuid.New()}
- 기대: SELECT 결과의 ID/State/DocumentID가 입력과 일치
- 허용 오차: CreatedAt ±1초

### 4.2 audit_logs JSONB (AC-CTRL-004-2)
- 입력: Event{Action: WORKFLOW_CREATED, details: {"test":"data"}}
- 기대: audit_logs 테이블 SELECT → action="WORKFLOW_CREATED", details JSONB 역직렬화 성공

### 4.3 SELECT FOR UPDATE 직렬화 (AC-CTRL-004-3)
- 2 goroutine 동시 BeginTx + GetWorkflow(FOR UPDATE)
- G1 커밋 후 G2가 RUNNING 상태 관찰
- 데드락 없음, 두 goroutine 모두 완료

### 4.4 mid-tx 실패 롤백 (AC-CTRL-004-4)
- 동일 UUID 두 번 INSERT → SQLSTATE 23505 unique violation
- 트랜잭션 롤백 후 workflows 행 0건

## 5. Pass Conditions

- 11개 통합 테스트 모두 PASS
- go test -race 통과 (race condition 없음)
- goleak.VerifyNone(t) 통과 (goroutine 누수 없음)
- go vet 0 오류, golangci-lint 0 오류

## 6. Sprint 3 RED 상태 확인

```
go build -tags=integration ./apps/control-plane/internal/store/... → PASS
go vet   -tags=integration ./apps/control-plane/internal/store/... → PASS
golangci-lint run --build-tags=integration ./apps/control-plane/... → PASS
go test  -tags=integration ./apps/control-plane/internal/store/ -v -count=1:
  FAIL: 10개 (ErrNotImplemented — 정상 RED 상태)
  PASS: 1개 (TestPgStore_Integration_NewPgWorkflowStore_InvalidDSN — DSN 에러는 stub에도 동작)
  Sprint 1+2 단위 테스트: 3 pkg / 0 FAIL (회귀 없음)
```

## 7. Files Created (Sprint 3 RED)

| 파일 | 역할 |
|------|------|
| `apps/control-plane/internal/store/pg_store.go` | PgWorkflowStore + PgWorkflowTx stub (ErrNotImplemented) |
| `apps/control-plane/internal/store/postgres_test.go` | 11개 통합 테스트 (//go:build integration) |
| `apps/control-plane/internal/store/testdata/schema.sql` | 통합 테스트용 PostgreSQL 스키마 |

## 8. GREEN phase 작업 계획

GREEN phase에서 stub 메서드를 실제 pgx 쿼리로 교체:

1. `BeginTx`: `s.pool.Begin(ctx)` + `&PgWorkflowTx{tx: pgxTx}` 반환
2. `InsertWorkflow`: `INSERT INTO workflows(id, status, document_id, result_json, created_at, updated_at) VALUES($1,$2,$3,$4,$5,$6)`
3. `GetWorkflow`: `SELECT ... FROM workflows WHERE id=$1 FOR UPDATE` + pgx.ErrNoRows → ErrWorkflowNotFound
4. `UpdateWorkflowState`: `UPDATE workflows SET status=$2, updated_at=NOW() WHERE id=$1`
5. `UpdateWorkflowResult`: `UPDATE workflows SET result_json=$2, updated_at=NOW() WHERE id=$1`
6. `InsertAuditLog`: `INSERT INTO audit_logs(action, resource_type, resource_id, user_id, details, timestamp) VALUES($1,$2,$3,$4,$5,$6)`
7. `Commit`: `t.tx.Commit(ctx)`
8. `Rollback`: `t.tx.Rollback(ctx)`

---

Version: 1.0.0 (Sprint 3 RED)
작성자: manager-tdd
