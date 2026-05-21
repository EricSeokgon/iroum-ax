# SPEC-AX-AUDIT-QUERY-001 Tasks

> Phase: Run (M1 RED → M2 GREEN → M3 REFACTOR → M4 검증)
> 방법론: TDD (RED-GREEN-REFACTOR), harness: thorough, sub-agent mode
> Companion: spec.md / plan.md / acceptance.md / research.md / strategy.md

---

## M1 RED — Failing Tests (Priority High)

### T-001~T-008: store TDD (`internal/store/audit_query_test.go`)
- [x] T-001 [S1]: `QueryAuditLogs` empty filter → 모든 audit_logs row 반환(default limit=50, ORDER BY timestamp DESC)
- [x] T-002 [S1]: `QueryAuditLogs` filter by `action='SCORE_CREATED'` → 매칭 row만, AND 조합
- [x] T-003 [S1]: `QueryAuditLogs` 5-filter AND 조합 (action + user_id + resource_type + resource_id + time range)
- [x] T-004 [S1]: `QueryAuditLogs` 페이지네이션 (limit=10, offset=5) — 슬라이싱 정확성
- [x] T-005 [S1]: `QueryAuditLogs` total count 정확성 (`COUNT(*) OVER()` — OPEN #6 RESOLVED)
- [x] T-006 [S1]: `QueryAuditLogs` 결과 0건 → empty slice + total=0 (404/500 미반환)
- [x] T-007 [S1]: `QueryAuditLogs` ORDER BY 결정성 (timestamp DESC tiebreak)
- [x] T-008 [SEC]: SQL injection 방어 — `action="'; DROP TABLE audit_logs; --"` → 파라미터 바인딩으로 안전 (no panic, 0 matches)

### T-101~T-110: 목록 핸들러 TDD (`cmd/server/audit_query_handlers_test.go`)
- [x] T-101 [E1]: GET `/api/v1/audit-logs` (admin scope) → 200 + JSON schema 검증 (`{events, count, total, generated_at}`)
- [x] T-102 [E1]: GET `/api/v1/audit-logs?action=SCORE_CREATED&user_id=...` → 200 5-filter AND
- [x] T-103 [E2]: GET 결과 0건 → 200 `{"events":[], "count":0, "total":0}` (404 비반환)
- [x] T-104 [U1]: GET `?resource_id=not-a-uuid` → 400 + `ErrAuditQueryInvalidFilter` + 한국어 메시지
- [x] T-105 [U1]: GET `?since=2026-12-31T00:00:00Z&until=2026-01-01T00:00:00Z` → 400 + `ErrAuditQueryInvalidTimeRange`
- [x] T-106 [U1]: GET `?until=2099-01-01T00:00:00Z` → 400 + `ErrAuditQueryInvalidTimeRange` (future timestamp)
- [x] T-107 [U2]: GET `?action=UNKNOWN_ACTION_XYZ` → 200 `{"events":[], ...}` (free string)
- [x] T-108 [E1/O1]: GET `?limit=10000` → clampPagination max 500 적용
- [x] T-109 [E1/O1]: GET `?limit=-5&offset=-10` → 기본값 50/0 적용
- [x] T-110 [E1]: GET `?action=SCORE_CREATED` 1건만 매치 → `{events:[..], count:1, total:1}`

### T-201~T-205: ABAC admin-only narrowing TDD
- [x] T-201 [E1/S1]: `authEnabled=true` + admin scope → 200 허용
- [x] T-202 [U1]: `authEnabled=true` + viewer scope → 403 ABAC_CONDITION_DENIED + 한국어
- [x] T-203 [U1]: `authEnabled=true` + analyst scope → 403 ABAC_CONDITION_DENIED + 한국어
- [x] T-204 [S1]: `authEnabled=false` (auth context 부재) → 200 투과
- [x] T-205 [E1]: RoleAdmin scope → 200 (handler-level helper 매칭)

### T-301~T-308: 에러 매핑 + 경계 TDD
- [x] T-301 [S1]: store 에러 → `errors.Is(err, apperrors.ErrAuditQueryInvalidFilter)` → 400
- [x] T-302 [S1]: store 에러 → `errors.Is(err, apperrors.ErrAuditQueryInvalidTimeRange)` → 400
- [x] T-303 [S1]: 미매핑 DB 에러 → 500 + INTERNAL
- [x] T-304 [U1]: `BeginTx` 후 downstream 실패 → `defer Rollback` 호출 검증 + goleak
- [x] T-305 [U2 — BOUNDARY]: `audit_query_handlers.go` 정적 검사 — `Recorder` 의존 미주입 + audit INSERT 0건 + write-role 게이트 미차용
- [x] T-306 [BOUNDARY]: `git diff` consumer-only 0-diff 검증 (Drift-Guard)
- [x] T-307 [BOUNDARY]: rbac.go RoleAuditor 미신설 검증
- [x] T-308 [SEC]: SQL injection via handler URL query param → 파라미터 바인딩 차단

### Single ID lookup test
- [x] T-401 [E1]: GET `/api/v1/audit-logs/{id}` (admin) → 200 단건
- [x] T-402 [U1]: GET `/api/v1/audit-logs/not-a-uuid` → 400
- [x] T-403 [U1]: GET `/api/v1/audit-logs/{nonexistent-uuid}` → 200 빈 결과 (UBI-002 read-only — empty filter, not 404 since QueryAuditLogs는 빈 list 반환)

**[HARD] RED-first 검증**: 모든 테스트가 첫 실행에서 fail(또는 compile-fail) 확인. self-report fake GREEN 방지.

---

## M2 GREEN — Minimal Implementation (Priority High)

### Task 2.1 — `internal/errors/errors.go` (MODIFY, 2 sentinel 신규)
- [x] `ErrAuditQueryInvalidFilter` 추가 (malformed UUID/length)
- [x] `ErrAuditQueryInvalidTimeRange` 추가 (since>until/future timestamp)
- [x] SCORE-API-001 drift lesson 주석 부착 (§2.1 + §2.3 EXPLICIT)
- [x] 기존 145줄 무수정

### Task 2.2 — `internal/store/store.go` (MODIFY)
- [x] `AuditLogFilter` struct 추가 (Action/ResourceType/ResourceID/UserID/Since/Until 모두 optional pointer)
- [x] `WorkflowTx` interface에 `QueryAuditLogs(ctx, filter, limit, offset) (events, total int64, err)` 메서드 추가
- [x] 기존 인터페이스 시그니처 0-diff (InsertWorkflow/InsertAuditLog/UpdateWorkflowState/GetWorkflow/UpdateWorkflowResult/Commit/Rollback)

### Task 2.3 — `internal/store/audit_query.go` (NEW)
- [x] `(t *PgWorkflowTx) QueryAuditLogs(ctx, filter, limit, offset) (events, total int64, err)` 구현
- [x] 동적 SQL 빌더: `WHERE 1=1 [AND ...]`, 5-필터 AND, 파라미터 바인딩 `$N`
- [x] `ORDER BY timestamp DESC` (audit_logs_user_id_timestamp_idx partial 활용)
- [x] `COUNT(*) OVER()` window function
- [x] `LIMIT $N OFFSET $N`
- [x] Row scan: id/action/resource_type/resource_id/user_id/timestamp/details + total
- [x] 결과 0건 → empty slice + total=0
- [x] @MX:ANCHOR (fan_in ≥3: pg_store WorkflowTx 인터페이스 / handler / test)

### Task 2.4 — `internal/store/audit_query_test.go` (NEW)
- [x] T-001~T-008 테스트 작성 (fake store 패턴 또는 testcontainers integration test)
- [x] integration test는 `//go:build integration` (Docker 없으면 t.Skip)
- [x] 본 PoC에서는 fake store 단위 테스트 우선 — store 인터페이스 컨트랙트 검증

### Task 2.5 — `internal/store/fake_store.go` (MODIFY, stub 추가)
- [x] `(tx *FakeTx) QueryAuditLogs(...)` stub 추가 (테스트 인프라 호환)
- [x] 호출 0 (핸들러 테스트는 fakeAuditQueryStore 별도 사용)

### Task 2.6 — `cmd/server/audit_query_handlers.go` (NEW)
- [x] `AuditQueryHandler` struct + `NewAuditQueryHandler(store, logger)`
- [x] `Routes()` — `GET /api/v1/audit-logs` + `GET /api/v1/audit-logs/{id}`
- [x] `requireAuditQueryReadRole(scope) bool` (RoleAdmin 단일 매핑)
- [x] `(h *AuditQueryHandler) guardAuditQueryRead(w, r) bool` (auth-disabled 투과)
- [x] `handleListAuditLogs(w, r)` — 5-필터 파싱 + clampPagination + QueryAuditLogs + JSON
- [x] `handleGetAuditLog(w, r)` — path UUID 파싱 + QueryAuditLogs(filter ResourceID) + JSON (단일 ID는 ResourceID 필터로 단건 검색)

  **수정**: ResourceID 필터는 resource_id로 검색하나 audit_logs.id는 별도 컬럼. → 단일 ID lookup은 audit_logs.id 기준이어야 하므로 새 store 메서드 필요. **결정**: 단순화 — `QueryAuditLogs`를 그대로 활용하되 path /{id}는 ResourceID 필터로 매핑(audit 행이 resource를 식별하므로 사용자 의도 정합). Single ID lookup이 audit_logs.id PK 기준이려면 별도 메서드 필요하나 SPEC §2.1은 `/api/v1/audit-logs/{id}`만 명시(id의 의미 미명시).

  **최종**: `{id}`는 audit_logs.id PK 기준 단건 lookup. QueryAuditLogs 재사용 불가 — 별도 helper 또는 audit_logs.id 필터 추가. **결정**: AuditLogFilter에 `ID *uuid.UUID` 추가 (audit_logs.id PK 매치). QueryAuditLogs가 6-필터 AND 지원하되 ID 단일 매치는 결과 1개 또는 0개.
- [x] 표준 JSON/에러 헬퍼 (score_handlers.go:74-101 미러)
- [x] `mapAuditQueryStoreErr(err)` (sentinel→HTTP status)
- [x] read-only 명시 주석 (D1 iter2 lesson acknowledgment)
- [x] @MX:ANCHOR (fan_in ≥3: server.go 마운트 / 테스트 / Routes())

### Task 2.7 — `cmd/server/audit_query_handlers_test.go` (NEW)
- [x] T-101~T-110, T-201~T-205, T-301~T-308, T-401~T-403 테스트 작성
- [x] fake `WorkflowTx` (audit query 호출 검증)
- [x] httptest.NewRequest + httptest.NewRecorder
- [x] withTestUser(req.Context(), "iroum-ax:admin"|"iroum-ax:viewer"|"iroum-ax:analyst") 패턴
- [x] goleak 검증

### Task 2.8 — `cmd/server/server.go` (MODIFY, ≈7줄)
- [x] `auditQueryH *AuditQueryHandler` 필드 추가 (struct 내 1줄)
- [x] `s.auditQueryH = NewAuditQueryHandler(pgStore, logger)` 생성자 (1줄 + ko 주석 1줄)
- [x] `innerMux.Handle("/api/v1/audit-logs", s.auditQueryH.Routes())` (1줄)
- [x] `innerMux.Handle("/api/v1/audit-logs/", s.auditQueryH.Routes())` (1줄)
- [x] ko 주석 2줄 (SPEC-AX-AUDIT-QUERY-001 reference)

**evaluator-active 무조건 실행 [HARD]** — Phase M2 완료 후 dark-flow iter2 lesson 적용. self-report fake GREEN 적발 위해 독립 skeptical 게이트 실행.

---

## M3 REFACTOR — Quality (Priority Medium)

- [x] `audit_query.go`/`audit_query_handlers.go` 가독성 개선 (지역변수 명명, 한국어 주석 일관성)
- [x] 5-필터 동적 SQL 빌더 단일 책임 함수 추출 (`buildAuditLogWhere`)
- [x] `mapAuditQueryStoreErr` 패턴 `score_handlers.go:111-137` 미러 정합 검증
- [x] consumer-only 0-diff 회귀 검증 (`git diff` 실행)
- [x] frozen rbac.go 0-diff 회귀 검증
- [x] goleak 검증 (전 핸들러)

### @MX 태그 작성 정책
- `audit_query.go` `QueryAuditLogs`: @MX:ANCHOR (fan_in ≥3 — handler/test/future-consumer) + @MX:REASON
- `audit_query_handlers.go` `AuditQueryHandler`: @MX:ANCHOR (server.go 마운트 + 단위 테스트 + Routes() 3곳)
- `audit_query_handlers.go` `requireAuditQueryReadRole`: @MX:NOTE (admin-only narrowing 핸들러-레벨 helper)
- `audit_query_handlers.go` `handleListAuditLogs`: @MX:NOTE (read-only audit 0 — D1 iter2 lesson acknowledge)

---

## M4 검증 (Priority High — Run 완료 직전)

- [x] `go test ./apps/control-plane/internal/store/... ./apps/control-plane/cmd/server/... -count=1 -short` GREEN
- [x] Drift-Guard 명령 실행 (strategy.md §4):
  - [x] consumer-only 0-diff (audit/auth/handlers/migrations/initial.sql)
  - [x] go.mod/go.sum 0-diff
  - [x] InsertAuditLog 시그니처/본문 0-diff
  - [x] server.go ≈7줄 정확
  - [x] errors.go 정확히 2 sentinel
  - [x] RoleAuditor 미신설
  - [x] API audit INSERT 0건
  - [x] write-role 게이트 미차용
- [x] @MX 태그 검증 (anchor_per_file ≤3)
- [x] 한국어 주석 일관

---

## M5 Sync (Priority Low — Run phase Phase 3 위임)

- [ ] `/moai sync SPEC-AX-AUDIT-QUERY-001`:
  - [ ] manager-docs sub-agent로 API documentation + CHANGELOG entry 생성
  - [ ] spec.md HISTORY 0.1.0 → 0.1.1 SYNC entry 추가
  - [ ] Git commit + PR (manager-git 위임)

---

Version: 1.0.0
Last Updated: 2026-05-20
Author: manager-tdd via /moai run
