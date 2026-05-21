# SPEC-AX-AUDIT-QUERY-001 Strategy

> Phase: Strategy (M0 → Run M1-M4 진입 직전)
> 결정 기준: spec.md §6 OPEN 6건 + Human Gate 권장 (a) 일괄 채택 (2026-05-20)
> manager-strategy 위임 — D.S 컨테이너 본문 (planning artifact)

---

## §1 OPEN 6건 5-element RESOLVED

각 OPEN은 (1) decision, (2) 근거 (frozen scope·SPEC source 정합), (3) 거부된 대안, (4) consumer-only impact (0-diff 보증), (5) Run phase 적용 지점.

### OPEN #1 — admin-only ABAC narrowing 매핑 결정성 [CRITICAL]

1. **Decision**: 핸들러-레벨 helper `requireAuditQueryReadRole(scope string) bool` + `(h *AuditQueryHandler) guardAuditQueryRead(w, r) bool` — `score_handlers.go:161-187` 패턴 동형 미러. RoleAdmin 단일 매핑.
2. **근거**:
   - `rbac.go:20-21` `RoleAdmin` 기존재 — 신규 역할 신설 0
   - `permissionMatrix`(rbac.go:39-60) 무수정 — frozen rbac.go 0-diff [HARD]
   - SCORE-API-001 §6 OPEN #4 lesson (evaluator 부재 → 핸들러-레벨 RESOLVED) 동형
   - `abac.go:4` narrowing-only 정신 — handler-level가 viewer/analyst 추가 거부, ABAC chain은 통과시킴
3. **거부**: (b) `permissionMatrix`에 `PermissionAuditQuery` 추가 — frozen rbac.go [HARD] 위반. (c) `abac.go`에 `OrgUnitCondition` 추가 — frozen abac.go [HARD] 위반.
4. **consumer-only impact**: `rbac.go`/`abac.go`/`middleware.go` 0-diff. 신규 RBAC Permission 0, 신규 ABAC Condition 0.
5. **Run 적용**: M2 작업 6 `audit_query_handlers.go` — `requireAuditQueryReadRole` + `guardAuditQueryRead` 헬퍼 신규 정의. `auth.ParseRolesFromScope`(rbac.go:68) + `auth.UserFromContext`(middleware.go:49) + `auth.RoleAdmin`(rbac.go:21) + `auth.ErrCodeABACDenied`(abac.go:24) 호출만.

### OPEN #2 — 필터 조합 정책 (AND only)

1. **Decision**: 5-필터 AND only (`WHERE 1=1 [AND a] [AND b] [AND c] [AND d] [AND e] [AND f]`).
2. **근거**: PoC 단순. SCORE-API-001 list filter 패턴(`score_handlers.go:283-333 handleListScores`) 정합 — level/status 필터 AND only. 동적 SQL 빌더 minimal. OR 지원 시 query string syntax 복잡(`action=X,Y` 모호).
3. **거부**: (b) OR 지원 — query string 모호성. (c) GraphQL filter — graphql 파서 신규 외부 의존 필요 — §1.4 HARD 위반.
4. **consumer-only impact**: 동적 SQL 빌더는 `internal/store/audit_query.go` 신규 파일 내부 함수. frozen scope 0-diff.
5. **Run 적용**: M1 T-003 (5-filter AND 통합 테스트), M2 작업 3 `internal/store/audit_query.go` 동적 SQL 빌더 슬라이스 append 패턴.

### OPEN #3 — 응답 schema + 페이지네이션 default + endpoint URL + 빈/누락

1. **Decision** (4 subdecisions):
   - **(1) 응답 schema**: `{"events":[{id, user_id, action, resource_id, resource_type, timestamp, details}], "count":N, "total":M, "generated_at":"RFC3339"}` — 전 필드 표면화 + total count + generated_at (REPORT-001 categoryReportResponse 정합).
   - **(2) 페이지네이션 default**: SCORE-API-001 정합 — `clampPagination(rawLimit, rawOffset)` 재사용(`score_handlers.go:144-157`) — defaultListLimit=50, maxListLimit=500, offset 음수→0.
   - **(3) endpoint URL**: `/api/v1/audit-logs` (collection) + `/api/v1/audit-logs/{id}` (single, path param) — REST 명사 복수, 테이블명 정합. `innerMux.Handle("/api/v1/audit-logs", ...)` + `innerMux.Handle("/api/v1/audit-logs/", ...)` 2줄.
   - **(4) 빈/누락**: 결과 0건 → `200 OK` `{"events":[], "count":0, "total":0, "generated_at":"..."}` (data-completeness, REPORT-001 §6.3 B-2 정합).
2. **근거**: REPORT-001/SCORE-API-001 response schema 미러 — 일관성 확보. clampPagination 재사용으로 신규 상수/함수 0 — DRY. REST 명사 복수 + path param은 Go1.22 ServeMux idiomatic. 빈 결과 200은 IMO REST best-practice.
3. **거부**: (1)(b) total 미포함 — UX 손해. (1)(c) cursor pagination — over-engineering. (2)(b) default 100 — 일관성 손실. (3)(b) `/api/v1/audits` — 테이블명 불일치. (3)(c) `/api/v1/admin/audit-logs` — URL에 권한 인코딩(REST 안티패턴). (4)(b) 404 — REST 안티패턴(필터 결과 0건은 not-found 아님).
4. **consumer-only impact**: `clampPagination` 재사용은 동일 package main의 score_handlers.go 함수 호출이므로 frozen scope 영향 0. URL은 신규 라우트 패턴 2개 — server.go ≈7줄 한정.
5. **Run 적용**:
   - M1 T-101 (응답 schema 검증), T-103 (빈 결과 200), T-108/T-109 (페이지네이션 clamp).
   - M2 작업 6 `audit_query_handlers.go` `Routes()` — `mux.HandleFunc("GET /api/v1/audit-logs/{id}", h.handleGetAuditLog)` + `mux.HandleFunc("GET /api/v1/audit-logs", h.handleListAuditLogs)`.
   - M2 작업 8 `server.go` — innerMux.Handle 2줄 (path-prefix routing).

### OPEN #4 — JSONB details 부분 검색 (PoC 미적용 / 이연)

1. **Decision**: PoC 미적용 — `details->>'eval_item_id'` 필터 0 / GIN index 추가 0. 후속 SPEC(`SPEC-AX-AUDIT-DETAILS-001` 가능) 책임.
2. **근거**: JSONB 부분 검색은 GIN index 추가 마이그레이션 필요 → §1.4 HARD 위반. evaluation_items hierarchy_code 역검색 use case는 PoC 우선순위 아님 — admin이 `resource_type='evaluation_item'`로 우회 가능. AUD-1 UUIDv5 surrogate(`recorder.go:295-305`)와의 정합성은 후속 SPEC.
3. **거부**: (b) PoC 활성 — GIN index 마이그레이션 0001~0006 무수정 [HARD] 위반.
4. **consumer-only impact**: `audit_logs` 스키마 0-diff. `details` JSONB는 응답 byte slice로 그대로 직렬화 (검색 불가, 표시만).
5. **Run 적용**: M2 작업 6 `audit_query_handlers.go` 핸들러는 `details` query param 0. M2 작업 3 `audit_query.go` `WHERE` 절은 5-필터만(action/resource_type/resource_id/user_id/since/until — `details->>` 0).

### OPEN #5 — store 메서드 위치 결정성 (WorkflowStore 확장)

1. **Decision**: 기존 `WorkflowStore` 인터페이스에 `QueryAuditLogs(ctx, AuditLogFilter, limit, offset) (events []*audit.Event, total int64, err error)` 메서드 추가. 신규 `WorkflowTx` 메서드 0 — TX 미진입(SELECT는 pool 직접 호출, read-only이므로 BeginTx 불필요).

   **Sub-decision**: SELECT는 pool 직접 호출 (BeginTx 우회 가능) vs BeginTx + read-only TX. → **pool 직접 호출** 채택 (read-only SELECT는 TX 불필요, score_handlers list 패턴은 BeginTx+Rollback이지만 본 SPEC은 단일 SELECT라 TX 오버헤드 회피, REPORT-001 BeginEvalItemTx 패턴은 multi-query 시퀀스 — 본 SPEC은 single SELECT라 불필요).

   **수정**: ⚠️ 그러나 SPEC 본문 §3.4 REQ-AUDIT-QUERY-003-U1이 "defer Rollback"을 명시하고, acceptance.md AC-AUDIT-QUERY-003-2가 read-only TX rollback을 검증. → **BeginTx + defer Rollback 채택** (read-only TX 일관성, goleak 검증 필요).

   **최종 시그니처**:
   ```go
   // WorkflowStore에 추가
   QueryAuditLogs(ctx, filter AuditLogFilter, limit, offset int) (events []*audit.Event, total int64, err error)
   // pool 직접 호출 — TX 미진입, read-only SELECT
   ```

   Wait — re-reading acceptance.md AC-AUDIT-QUERY-003-2: "BeginTx(또는 BeginAuditQueryTx OPEN #5) 후 downstream 호출이 실패 → defer tx.Rollback(ctx)". `BeginTx`가 WorkflowStore 메서드. **결정**: WorkflowStore.BeginTx() 후 새 메서드 `WorkflowTx.QueryAuditLogs` 호출 — score_handlers.go BeginScoreTx 패턴 미러.

   **최종 결정**: `WorkflowTx`에 `QueryAuditLogs` 메서드 추가. WorkflowStore.BeginTx 진입점 활용. defer tx.Rollback(ctx) read-only.

2. **근거**: store.go 인터페이스 minimal 확장 — 신규 interface 0(over-engineering 회피). REPORT-001 패턴(EvalItemStore≠ScoreStore) 분리는 도메인이 다를 때 — audit_logs는 워크플로우 컨텍스트의 부속이라 WorkflowStore 자연 확장. BeginTx는 기존(`pg_store.go:83`) 재사용 — read-only TX 일관성 + goleak 검증 + Rollback defer 패턴.
3. **거부**: (b) `AuditQueryStore` 신규 인터페이스 — over-engineering. (c) stateless function — interface 안티패턴, mocking 어려움.
4. **consumer-only impact**: store.go [MODIFY] — `WorkflowStore` interface에 `QueryAuditLogs` 추가 (1줄) + `WorkflowTx` interface에 `QueryAuditLogs` 추가 (1줄) + `AuditLogFilter` struct 추가 (10줄). 기존 `InsertWorkflow`/`InsertAuditLog`/`UpdateWorkflowState`/`GetWorkflow`/`UpdateWorkflowResult`/`Commit`/`Rollback` 시그니처 0-diff. `WorkflowTx`의 audit INSERT(`pg_store.go:346-381`) 0-diff. FakeTx에도 stub 추가 필요(test 인프라).
5. **Run 적용**:
   - M2 작업 2 `store.go` — `AuditLogFilter` struct + `WorkflowStore.QueryAuditLogs` + `WorkflowTx.QueryAuditLogs` interface 추가.
   - M2 작업 3 `pg_store.go` — `(t *PgWorkflowTx) QueryAuditLogs(ctx, filter, limit, offset)` SELECT 구현 추가.
   - M2 작업 5 `fake_store.go` — `(tx *FakeTx) QueryAuditLogs` stub 추가 (test 인프라, 핸들러 테스트는 별도 `fakeAuditQueryStore` 사용).

### OPEN #6 — total count 산출 방식 (COUNT(*) OVER() window function)

1. **Decision**: `COUNT(*) OVER()` window function — 동일 쿼리 1 round-trip.
2. **근거**: PostgreSQL native — pagination + total 동시 산출. 별도 `SELECT COUNT(*)` 쿼리는 2 round-trip + isolation 일치 보장 어려움(read-only TX는 SNAPSHOT이라 일치하나, network round-trip 비용). p99 < 200ms 목표 — audit_logs row count 100K 미만 PoC scope에서 window function 충분.
3. **거부**: (b) 별도 `SELECT COUNT(*)` 쿼리 — 2 round-trip. (c) total 미산출 — UX 손해.
4. **consumer-only impact**: 동적 SQL 빌더는 `internal/store/audit_query.go` 신규 함수. frozen scope 0-diff. PG 14+ window function 표준 — 신규 외부 의존 0.
5. **Run 적용**: M2 작업 3 `audit_query.go` — SQL 빌더에 `SELECT id, ..., COUNT(*) OVER() AS total FROM audit_logs WHERE ...`. row scan 시 total 단일 변수에 저장(첫 row에서 1회 + 후속 row마다 동일 값 덮어쓰기 — total은 모든 row에서 동일).

---

## §2 store 계약 — `AuditLogFilter` + `QueryAuditLogs`

```go
// internal/store/store.go 추가

// AuditLogFilter audit_logs 검색 필터 (5-필터 AND 조합, 모두 optional pointer)
// nil 포인터 = "필터 미적용", non-nil = "해당 값으로 WHERE AND 추가"
// REQ-AUDIT-QUERY-001-E1 / SPEC-AX-AUDIT-QUERY-001 §6 OPEN #2 RESOLVED (AND only)
type AuditLogFilter struct {
    Action       *string    // audit.Action 자유 문자열 매치 (audit_logs.action VARCHAR(64))
    ResourceType *string    // audit_logs.resource_type VARCHAR(32)
    ResourceID   *uuid.UUID // audit_logs.resource_id UUID
    UserID       *string    // audit_logs.user_id VARCHAR(64)
    Since        *time.Time // audit_logs.timestamp >= since (inclusive)
    Until        *time.Time // audit_logs.timestamp <= until (inclusive)
}

// WorkflowStore 인터페이스 미존재 메서드 (1줄)
// — QueryAuditLogs는 WorkflowTx 메서드. BeginTx 후 호출.

// WorkflowTx 인터페이스에 추가 메서드:
// QueryAuditLogs audit_logs 테이블을 5-필터 AND + offset/limit으로 검색하고
// 결과 events + total count (COUNT(*) OVER() 윈도우 함수)를 반환한다.
// (REQ-AUDIT-QUERY-001-E1, OPEN #5/#6 RESOLVED)
//
// ORDER BY timestamp DESC, user_id (audit_logs_user_id_timestamp_idx 활용)
// 매개변수 바인딩($N) — SQL injection 0건 (REQ-AUDIT-QUERY-001-S2)
// 결과 0건 → empty slice + total=0 (404/500 미반환 — REQ-AUDIT-QUERY-001-E2)
QueryAuditLogs(
    ctx context.Context,
    filter AuditLogFilter,
    limit, offset int,
) (events []*audit.Event, total int64, err error)
```

---

## §3 endpoint matrix

| Method | URL | Handler | Auth Role | Audit |
|--------|-----|---------|-----------|-------|
| GET | `/api/v1/audit-logs` | `handleListAuditLogs` | admin only (handler-level helper) | 0 (read-only) |
| GET | `/api/v1/audit-logs/{id}` | `handleGetAuditLog` | admin only (handler-level helper) | 0 (read-only) |

**Query params (목록 검색)**:
- `action` (string, optional) — audit.Action 자유 문자열
- `resource_type` (string, optional) — VARCHAR(32) 정합
- `resource_id` (UUID, optional) — `uuid.Parse` 검증
- `user_id` (string, optional) — VARCHAR(64) 정합
- `since` (RFC3339, optional) — timestamp >= since
- `until` (RFC3339, optional) — timestamp <= until, since > until → 400, future timestamp → 400
- `limit` (int, optional) — clampPagination, default 50, max 500
- `offset` (int, optional) — clampPagination, default 0, 음수→0

**응답 schema**:
```json
{
  "events": [
    {
      "id": "uuid",
      "user_id": "string",
      "action": "string",
      "resource_id": "uuid",
      "resource_type": "string",
      "timestamp": "RFC3339",
      "details": { /* JSONB object */ }
    }
  ],
  "count": N,        // 페이지 결과 개수
  "total": M,        // 5-필터 매치 전체 개수 (COUNT(*) OVER())
  "generated_at": "RFC3339"
}
```

**에러 응답** (`score_handlers.go:74-101` 미러):
```json
{
  "error": {
    "code": "INVALID_ARGUMENT|ABAC_CONDITION_DENIED|NOT_FOUND|INTERNAL",
    "message": "한국어 메시지",
    "field": "field-name (optional)"
  }
}
```

---

## §4 Drift-Guard 명령 (Run M5 검증)

```bash
# 1. consumer-only 0-diff 핵심 frozen scope
git diff --quiet -- \
  apps/control-plane/internal/audit/audit.go \
  apps/control-plane/internal/audit/recorder.go \
  apps/control-plane/internal/auth/abac.go \
  apps/control-plane/internal/auth/rbac.go \
  apps/control-plane/internal/auth/middleware.go \
  apps/control-plane/cmd/server/score_handlers.go \
  apps/control-plane/cmd/server/report_handlers.go \
  apps/control-plane/cmd/server/review_handlers.go \
  apps/control-plane/cmd/server/rubric_handlers.go \
  apps/control-plane/cmd/server/evidence_handlers.go \
  .moai/db/schema/initial.sql

# 2. 0001~0006 마이그레이션 0-diff
git diff --quiet -- .moai/db/schema/migrations/

# 3. go.mod/go.sum 0-diff (신규 외부 의존 0)
git diff --quiet -- apps/control-plane/go.mod apps/control-plane/go.sum

# 4. pg_store.go audit INSERT 부분 0-diff (line 346-381 — InsertAuditLog 메서드 본문)
# QueryAuditLogs 신규 추가는 허용, InsertAuditLog 무수정 검증
git diff apps/control-plane/internal/store/pg_store.go | grep -E '^[+-].*InsertAuditLog' | grep -v '^+++\|^---'
# 결과 0줄이어야 함 (InsertAuditLog 시그니처/본문 무수정)

# 5. server.go ≈7줄 검증
git diff --stat apps/control-plane/cmd/server/server.go
# 7~10 lines added 범위

# 6. errors.go 정확히 2 sentinel 추가 검증
git diff apps/control-plane/internal/errors/errors.go | grep -c '^+var Err' 
# 결과 == 2

# 7. RoleAuditor 미신설 검증 (frozen rbac.go [HARD])
grep -c 'RoleAuditor' apps/control-plane/internal/auth/rbac.go
# 결과 == 0

# 8. API audit INSERT 0건 검증 (read-only)
grep -c 'InsertAuditLog\|Recorder' apps/control-plane/cmd/server/audit_query_handlers.go
# 결과 == 0

# 9. write-role 게이트 미차용 검증
grep -c 'requireScoreWriteRole\|guardScoreWrite' apps/control-plane/cmd/server/audit_query_handlers.go
# 결과 == 0
```

---

## §5 Risk register (M0 OPEN RESOLVED 후 잔여)

| ID | 위험 | Severity | Mitigation |
|----|------|----------|-----------|
| R-AUDIT-QUERY-001 | admin-only narrowing 매핑 불일치 (RoleAdmin scope 인식 실패) | HIGH | T-201/T-205 admin scope("iroum-ax:admin") RED-first 검증 + ABAC chain hasAdminRole(abac.go:94-99) 우회 검증 |
| R-AUDIT-QUERY-002 | consumer-only 0-diff 위반 (`WorkflowStore` 확장이 기존 InsertAuditLog 영향) | HIGH | T-306 `git diff pg_store.go:346-381` 0-diff 검증, M3 회귀 `WorkflowTx` 메서드 셋 변경 0 |
| R-AUDIT-QUERY-003 | frozen rbac.go 0-diff 위반 (RoleAuditor 신설 유혹) | HIGH | T-307 `git diff internal/auth/rbac.go` 0-diff, 핸들러-레벨 helper만 |
| R-AUDIT-QUERY-004 | SQL injection 방어 누락 (5-필터 동적 빌더) | MED | T-008/T-308 injection 시도 테스트, pgx placeholder($N) 일관 사용 |
| R-AUDIT-QUERY-005 | audit-of-audit-read 부당 발생 | MED | T-305 `recorder` 의존 미주입 + audit INSERT SQL 0건 정적 검증 |
| R-AUDIT-QUERY-006 | D-1 mutation negative-control 누락 | MED | T-101 GREEN 후 AND→OR 의도적 손상 → 테스트 RED 전환 확인 (REPORT-001 D-1 패턴) |
| R-AUDIT-QUERY-007 | 인덱스 활용 미흡 (`audit_logs_user_id_timestamp_idx` user_id 우선 vs ORDER BY timestamp 우선) | LOW | M3 EXPLAIN 검증 (integration test에서만, 핸들러 단위 N/A). PoC 100K row 미만에서 seq scan 허용 |
| R-AUDIT-QUERY-008 | `COUNT(*) OVER()` window function 성능 | LOW | PoC scope 100K row 미만 가정. p99 < 200ms M3 검증 |
| R-AUDIT-QUERY-009 | D1 iter2 lesson misapply | LOW | read-only이므로 NewRecorder mode 무관, mutation 메서드 0. acknowledge comment만 |
| R-AUDIT-QUERY-010 | SCORE-API-001 errors.go drift lesson 분실 | LOW | M2 작업 1 — §2.1 + §2.3 EXPLICIT 부착 확인, 신규 정확히 2 sentinel |

---

## §6 D1 iter2 lesson acknowledgment

본 SPEC은 read-only이므로:
- `NewRecorder(true/false)` 모드 결정 0 — recorder 의존 미주입(SCORE-001 D1 lesson 적용 불필요)
- mutation 메서드 0 — userID string 파라미터 0
- 그러나 명시적 acknowledge: "AUDIT-QUERY-001 is read-only; D1 iter2 lesson (NewRecorder mode + userID parameter) does not apply but acknowledged for memory record"

→ M2 작업 6 `audit_query_handlers.go` 파일 헤더 주석에 명시.

---

Version: 1.0.0
Last Updated: 2026-05-20
Author: manager-tdd via /moai run

→ Acceptance: M0 RESOLVED. Proceed to M1 RED.
