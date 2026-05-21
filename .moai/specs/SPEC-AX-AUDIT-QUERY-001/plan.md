# SPEC-AX-AUDIT-QUERY-001 Implementation Plan

> Companion: `spec.md` (EARS), `acceptance.md` (G/W/T), `research.md` (Phase 0.5 SSOT)
> 방법론: TDD (RED-GREEN-REFACTOR). harness: thorough. mode: sub-agent. brownfield.
> Plan-phase DoD는 §8. AC/edge count는 plan.md가 운반하지 않음(single source of truth = acceptance.md §9 + spec.md §9 + spec-compact.md — SCORE-API-001 D3-1 정합).

---

## 1. 구현 전제 (orchestrator ground-truth 2026-05-20)

- **7 SPEC 누적 GREEN**: SCORE-001 v0.1.3 + SCORE-API-001 v0.1.1 + REPORT-001 v0.1.1 + REVIEW-001 + RUBRIC-001 + EVID-001 + EVAL-ITEM-001 + AUTH-003 + CTRL-001 + OBS-001 + SERVER-001. 본 SPEC 7번째 (iroum-ax 누적).
- **소비 시그니처 source-verified (phantom 0)**:
  - audit_logs schema: `.moai/db/schema/initial.sql:115-123` (id UUID PK / user_id VARCHAR(64) / action VARCHAR(64) / resource_id UUID NOT NULL / resource_type VARCHAR(32) / timestamp TIMESTAMP WITH TIME ZONE / details JSONB)
  - audit_logs index: `audit_logs_user_id_timestamp_idx ON audit_logs(user_id, timestamp DESC)` (`initial.sql:138`)
  - Action 23+ 상수: `apps/control-plane/internal/audit/audit.go:17-100` (7 SPEC 누적)
  - Event struct: `apps/control-plane/internal/audit/audit.go:121-128` (Timestamp/Action/ResourceType/ResourceID uuid/UserID/DetailsJSON)
  - InsertAuditLog: `apps/control-plane/internal/store/pg_store.go:346-381` (id 자동생성/JSONB 직렬화/timestamp 보존)
  - AUD-1 namespace: `apps/control-plane/internal/audit/audit.go:115` + `recorder.go:295-305 evalItemResourceID`
  - ABAC/RBAC: `apps/control-plane/internal/auth/{abac.go:4-99, rbac.go:20-33, middleware.go:25-49}`
  - read-only 핸들러 선례: `apps/control-plane/cmd/server/{score_handlers.go:43-190, report_handlers.go:43-72}`
  - server.go 라우트 패턴: `apps/control-plane/cmd/server/server.go:55-60, 216-220, 277-285`
  - 에러 센티넬: `apps/control-plane/internal/errors/errors.go:1-145`
  - go.mod: 신규 외부 의존 0 (`database/sql`/`pgx`/`encoding/json`/`net/http`/`zap`/`google/uuid`만)
- **frozen scope 6 SPEC 누적**:
  - `internal/store/{score, eval_item, evidence, score_review_request, rubric}.go` — 무변경
  - `cmd/server/{score, report, review, rubric, evidence}_handlers.go` — 무변경
  - `internal/auth/{abac.go, rbac.go, middleware.go}` — 무변경 [HARD]
  - `.moai/db/schema/migrations/0001~0006_*.sql` + `initial.sql` — 무변경
  - `internal/audit/{audit.go, recorder.go}` — 무변경
  - `pg_store.go` audit INSERT 부분 (`pg_store.go:346-381`) — 무변경
  - `go.mod`/`go.sum` — 무변경
- **신규 도메인 (본 SPEC만 확장)**:
  - `internal/store/audit_query.go` (NEW): `QueryAuditLogs` SELECT 구현
  - `internal/store/audit_query_test.go` (NEW)
  - `cmd/server/audit_query_handlers.go` (NEW): `AuditQueryHandler` + `requireAuditQueryReadRole`/`guardAuditQueryRead` 핸들러-레벨 admin-only 게이트
  - `cmd/server/audit_query_handlers_test.go` (NEW)
  - `internal/store/store.go` (MODIFY): `QueryAuditLogs` 인터페이스 추가 (OPEN #5 → §A.5)
  - `internal/store/pg_store.go` (MODIFY): `QueryAuditLogs` SELECT 구현 호출
  - `cmd/server/server.go` (MODIFY): ≈7줄 (auditQueryH 필드 + 생성자 + 2 routes)
  - `internal/errors/errors.go` (MODIFY): 2 sentinel (`ErrAuditQueryInvalidFilter`/`ErrAuditQueryInvalidTimeRange`) — SCORE-API-001 drift lesson [HARD] EXPLICIT 부착

## 2. 작업 단위 분해 (Milestones, no time estimates)

마일스톤은 priority 라벨로 분류, 시간 추정 금지(@MX 워크플로우 정책).

### M0 — Strategy + Plan-audit (Priority High)

manager-strategy + plan-auditor sub-agent로 §6 OPEN 6건 RESOLVED:
- OPEN #1 admin-only narrowing 매핑(핸들러-레벨 helper) → 권장 (a) 채택
- OPEN #2 필터 조합 AND only → 권장 (a) 채택
- OPEN #3 응답 schema + 페이지네이션 default + endpoint URL + 빈/누락 → 권장 (a) 4건 채택
- OPEN #4 JSONB details 부분 검색 PoC 미적용(이연) → 권장 (a) 채택
- OPEN #5 store 메서드 위치 (WorkflowStore에 추가) → 권장 (a) 채택
- OPEN #6 total count 산출 (COUNT(*) OVER()) → 권장 (a) 채택

Human Gate sign-off 후 strategy.md §A 작성. plan-auditor iter1/iter2 검증.

### M1 — RED 첫 단계 (TDD RED) (Priority High)

failing 테스트 작성, GREEN 코드는 절대 작성 0.

테스트 enumeration (acceptance.md AC ↔ 테스트 매핑):
- **store TDD**: `internal/store/audit_query_test.go` (fake store / 또는 testdata-driven pgx in-memory)
  - T-001 [S1]: `QueryAuditLogs` empty filter → 모든 audit_logs row 반환(default limit=50, ORDER BY timestamp DESC)
  - T-002 [S1]: `QueryAuditLogs` filter by `action='SCORE_CREATED'` → 매칭 row만, AND 조합
  - T-003 [S1]: `QueryAuditLogs` 5-filter AND 조합 (action + user_id + resource_type + resource_id + time range)
  - T-004 [S1]: `QueryAuditLogs` 페이지네이션 (limit=10, offset=5) — 슬라이싱 정확성
  - T-005 [S1]: `QueryAuditLogs` total count 정확성 (`COUNT(*) OVER()` — OPEN #6 RESOLVED)
  - T-006 [S1]: `QueryAuditLogs` 결과 0건 → empty slice + total=0 (404/500 미반환)
  - T-007 [S1]: `QueryAuditLogs` ORDER BY 결정성 (timestamp DESC, user_id ASC tiebreak)
  - T-008 [SEC]: SQL injection 방어 — `action="'; DROP TABLE audit_logs; --"` → 파라미터 바인딩으로 안전
- **핸들러 TDD**: `cmd/server/audit_query_handlers_test.go`
  - T-101 [E1]: GET `/api/v1/audit-logs` (admin) → 200 + JSON schema 검증 (`{events, count, total, generated_at}`)
  - T-102 [E1]: GET `/api/v1/audit-logs?action=SCORE_CREATED&user_id=...` → 200 5-filter AND
  - T-103 [E2]: GET 결과 0건 → 200 `{"events":[], "count":0, "total":0}` (404 비반환)
  - T-104 [U1]: GET `?resource_id=not-a-uuid` → 400 + `ErrAuditQueryInvalidFilter` + 한국어 메시지
  - T-105 [U1]: GET `?since=2026-12-31T00:00:00Z&until=2026-01-01T00:00:00Z` → 400 + `ErrAuditQueryInvalidTimeRange` (since > until)
  - T-106 [U1]: GET `?since=2099-01-01T00:00:00Z` → 400 + `ErrAuditQueryInvalidTimeRange` (future timestamp)
  - T-107 [U2]: GET `?action=UNKNOWN_ACTION_XYZ` → 200 `{"events":[], ...}` (free string, not 400)
  - T-108 [E1/O1]: GET `?limit=10000` → clampPagination max 500 적용 후 처리
  - T-109 [E1/O1]: GET `?limit=-5&offset=-10` → 기본값 50/0 적용 (clamp 결정성)
  - T-110 [E1/E2]: GET `?action=SCORE_CREATED` 1건만 매치 → `{events:[..], count:1, total:1}`
  - T-201 [E1/S1]: `authEnabled=true` + admin scope → 200 허용 (admin narrowing 매칭)
  - T-202 [U1]: `authEnabled=true` + viewer scope → 403 ABAC_CONDITION_DENIED + 한국어 메시지
  - T-203 [U1]: `authEnabled=true` + analyst scope → 403 ABAC_CONDITION_DENIED + 한국어 메시지
  - T-204 [S1]: `authEnabled=false` (Walking Skeleton) → 200 투과 (모든 미인증 요청 허용)
  - T-205 [E1]: `authEnabled=true` + RoleAdmin scope → 200 (admin 우회 `abac.go:94-99 hasAdminRole`)
  - T-301 [S1]: store 에러 → `errors.Is(err, apperrors.ErrAuditQueryInvalidFilter)` → 400
  - T-302 [S1]: store 에러 → `errors.Is(err, apperrors.ErrAuditQueryInvalidTimeRange)` → 400
  - T-303 [S1]: 미매핑 DB 에러 → 500 + INTERNAL
  - T-304 [U1]: `BeginTx` 후 downstream 실패 → `defer Rollback` 호출 검증 + goleak 0
  - T-305 [U2 — BOUNDARY]: `audit_query_handlers.go` 코드 정적 검사 — `Recorder` 의존 미주입 + `audit_logs` INSERT SQL 0건 + write-role 게이트(`requireScoreWriteRole`) 미차용
  - T-306 [BOUNDARY]: `git diff` consumer-only 0-diff 검증 (Drift-Guard manifest §2.3) — internal/audit, internal/auth, score|report|review|rubric|evidence_handlers.go, .moai/db/schema, pg_store.go audit INSERT 부분, go.mod 무수정
  - T-307 [BOUNDARY]: rbac.go RoleAuditor 미신설 검증 + permissionMatrix 무수정 검증 (`git diff internal/auth/rbac.go` 0건)
  - T-308 [SEC]: SQL injection 방어 — handler URL query value를 통한 injection 시도가 파라미터 바인딩으로 차단됨
- **negative-control mutation test (D-1)**: T-101 GREEN 후 store 메서드를 의도적으로 손상시켜(예: AND→OR 치환) 테스트가 RED로 전환됨을 확인 (REPORT-001 D-1 mutation-tested negative-control 패턴 정합)

**[HARD] RED-first 검증**: 모든 테스트가 첫 실행에서 fail(또는 compile-fail)함을 명시적으로 확인. self-report fake GREEN 방지 (dark-flow iter2 lesson — feedback_dark_flow_iter2_pattern.md).

### M2 — GREEN minimal 코드 (Priority High)

failing 테스트 통과를 위한 minimal Go 코드 작성. premature optimization 0. self-report fake GREEN 절대 금지(skeptical evaluator-active iter2 게이트 필수).

작업 단위:
1. `internal/errors/errors.go` (MODIFY): 신규 센티넬 정확히 2개 추가 — `ErrAuditQueryInvalidFilter`, `ErrAuditQueryInvalidTimeRange`. 기존 145줄 무수정.
2. `internal/store/store.go` (MODIFY): `AuditQueryFilter` struct + `WorkflowStore.QueryAuditLogs(...)` 인터페이스 추가 (OPEN #5 §A.5 RESOLVED 후). 기존 인터페이스 무수정.
3. `internal/store/audit_query.go` (NEW): `PgWorkflowStore.QueryAuditLogs(ctx, filter, limit, offset) (events, total, err)` 구현 — `audit_logs WHERE` 동적 빌더(파라미터 바인딩), `ORDER BY timestamp DESC, user_id`, `LIMIT $1 OFFSET $2`, `COUNT(*) OVER() AS total` window function.
4. `internal/store/audit_query_test.go` (NEW): T-001~T-008 테스트 작성.
5. `internal/store/pg_store.go` (MODIFY): `BeginTx` 후 `WorkflowTx.QueryAuditLogs` 위임 메서드 추가 (OPEN #5 §A.5 RESOLVED). audit INSERT 부분(`pg_store.go:346-381`) 무수정.
6. `cmd/server/audit_query_handlers.go` (NEW): `AuditQueryHandler` struct + `NewAuditQueryHandler(store, logger)` + `Routes()` + `requireAuditQueryReadRole(scope) bool` + `guardAuditQueryRead(w, r) bool` (`score_handlers.go:161-187` 동형) + `handleQueryAuditLogs(w, r)` + 5-필터 파싱(`action`/`resource_type`/`resource_id`/`user_id`/`since`/`until`) + `clampPagination` 재사용 + JSON/에러 헬퍼(`score_handlers.go:74-101` 미러) + `mapAuditQueryStoreErr` (sentinel→HTTP).
7. `cmd/server/audit_query_handlers_test.go` (NEW): T-101~T-308 테스트 작성.
8. `cmd/server/server.go` (MODIFY): ≈7줄 — `auditQueryH *AuditQueryHandler` 필드 + `s.auditQueryH = NewAuditQueryHandler(pgStore, logger)` + `innerMux.Handle("/api/v1/audit-logs", s.auditQueryH.Routes())` + `innerMux.Handle("/api/v1/audit-logs/", s.auditQueryH.Routes())` + ko 주석. ABAC 와이어링 무변경(기존 미들웨어 체인 `server.go:287`이 innerMux 전체 감쌈).

**evaluator-active 무조건 실행 [HARD]** — Phase 2 후 dark-flow iter2 lesson per memory (feedback_dark_flow_iter2_pattern.md). self-report fake GREEN 적발 위해 독립 skeptical 게이트 실행. 1차 iter PASS 시까지 반복.

### M3 — REFACTOR + 회귀 (Priority Medium)

테스트 GREEN 유지하며 코드 품질 향상:
- `audit_query.go`/`audit_query_handlers.go` 가독성 개선 (지역변수 명명, 한국어 주석 일관성)
- 5-필터 동적 SQL 빌더 함수 추출 (단일 책임)
- `mapAuditQueryStoreErr` 패턴 `score_handlers.go:111-137` 미러 정합 검증
- consumer-only 0-diff 회귀 검증 (`git diff` 실행)
- frozen rbac.go 0-diff 회귀 검증
- goleak 검증 (전 핸들러)

@MX 태그 작성 정책:
- `audit_query.go` `QueryAuditLogs`: @MX:ANCHOR (fan_in ≥3 예상 — handler/test/future-consumer) + @MX:REASON (audit_logs SELECT 단일 진입점, frozen schema 정합)
- `audit_query_handlers.go` `AuditQueryHandler`: @MX:ANCHOR (server.go 마운트 + handler 단위 테스트 + Routes() 3곳)
- `audit_query_handlers.go` `requireAuditQueryReadRole`: @MX:NOTE (admin-only narrowing 핸들러-레벨 helper, score_handlers.go:161 동형 미러)
- `audit_query_handlers.go` `handleQueryAuditLogs`: @MX:NOTE (audit-of-audit-read OUT — read-only mutation 0 → audit 이벤트 0)

### M4 — Sync (Priority Low — Run phase Phase 3 위임)

`/moai sync SPEC-AX-AUDIT-QUERY-001`:
- manager-docs sub-agent로 API documentation + CHANGELOG entry 생성
- spec.md HISTORY 0.1.0 → 0.1.1 SYNC entry 추가 (TDD sub-agent genuine RED-first / D-1 negative-control mutation-tested 완료, evaluator-active 이중 게이트 PASS, consumer-only 0-diff 불변, §6 6건 OPEN RESOLVED 명기)
- Git commit + PR (이전 작업처럼 manager-git 위임)

## 3. 기술 접근 (Technical Approach)

### 3.1 store SELECT 메서드 시그니처 (OPEN #5 §A.5 RESOLVED 후 정확)

```go
// AuditQueryFilter 감사 로그 검색 필터 (5-필터 AND 조합, 모두 optional pointer)
type AuditQueryFilter struct {
    Action       *string    // audit.Action 자유 문자열 매치
    ResourceType *string
    ResourceID   *uuid.UUID
    UserID       *string
    Since        *time.Time // timestamp >= since
    Until        *time.Time // timestamp <= until
}

// WorkflowStore (or AuditQueryStore — OPEN #5)
QueryAuditLogs(ctx context.Context, filter AuditQueryFilter, limit, offset int) (events []*audit.Event, total int, err error)
```

### 3.2 동적 SQL 빌더 (5-필터 AND, 파라미터 바인딩)

```
SELECT id, action, resource_type, resource_id, user_id, timestamp, details,
       COUNT(*) OVER() AS total
  FROM audit_logs
 WHERE 1=1
   [AND action = $N]
   [AND resource_type = $N]
   [AND resource_id = $N]
   [AND user_id = $N]
   [AND timestamp >= $N]
   [AND timestamp <= $N]
 ORDER BY timestamp DESC, user_id
 LIMIT $N OFFSET $N
```

- 동적 placeholder index 관리(슬라이스 append): SQL injection 0건
- ORDER BY는 `audit_logs_user_id_timestamp_idx`(`initial.sql:138`) 활용 위해 `timestamp DESC, user_id`. (인덱스는 `(user_id, timestamp DESC)` — query planner가 partial 활용 가능, EXPLAIN 검증 권장 (M3))

### 3.3 admin-only narrowing (핸들러-레벨 helper)

```go
// requireAuditQueryReadRole — score_handlers.go:161-187 패턴 동형
func requireAuditQueryReadRole(scope string) bool {
    for _, r := range auth.ParseRolesFromScope(scope) {
        if r == auth.RoleAdmin {
            return true
        }
    }
    return false
}

// guardAuditQueryRead — score_handlers.go:179-190 패턴 동형
func (h *AuditQueryHandler) guardAuditQueryRead(w http.ResponseWriter, r *http.Request) bool {
    u, ok := auth.UserFromContext(r.Context())
    if !ok {
        return true // auth-disabled 투과
    }
    if requireAuditQueryReadRole(strings.Join(u.Scopes, " ")) {
        return true
    }
    h.writeAuditQueryErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
        "감사 로그 조회 권한이 없습니다", "")
    return false
}
```

### 3.4 5-필터 파싱 (handler level)

- `action` (string): 무검증 자유 문자열(`U2` 미알려진 액션 200 빈결과)
- `resource_type` (string): `audit_logs.resource_type VARCHAR(32)` 정합, 길이 0~32 검증
- `resource_id` (uuid.UUID): `uuid.Parse` 실패 시 `ErrAuditQueryInvalidFilter` 400
- `user_id` (string): `audit_logs.user_id VARCHAR(64)` 정합, 길이 0~64 검증
- `since`/`until` (time.Time): `time.RFC3339` 파싱 실패 시 `ErrAuditQueryInvalidFilter` 400. since > until → `ErrAuditQueryInvalidTimeRange` 400. future timestamp(현재 > until) → `ErrAuditQueryInvalidTimeRange` 400(business logic 거부)

## 4. RED 테스트 enumeration (Phase 1 RED-first)

§2 M1에 전체 enumeration 게재. 핵심 분류:
- store TDD: T-001~T-008 (8개)
- 핸들러 TDD: T-101~T-110 (10개) + T-201~T-205 (5개) + T-301~T-308 (8개)
- 누계: 31개 RED 테스트 (acceptance.md AC 25개 + edge case 14개 = 39 기준점 → 일부 통합)
- D-1 negative-control mutation test: T-101 GREEN 후 store 의도적 손상

**[HARD] S0 게이트 (Run 진입 전 hard-verify, dark-flow iter2 lesson)**:
- `grep -n 'InsertAuditLog\|PgWorkflowTx' apps/control-plane/internal/store/pg_store.go` (라인 346-381 실재 확인)
- `grep -n 'audit_logs\|audit_logs_user_id_timestamp_idx' .moai/db/schema/initial.sql` (라인 115-138 실재)
- `grep -n 'RoleAdmin\|hasAdminRole\|ParseRolesFromScope' apps/control-plane/internal/auth/*.go` (rbac.go:20-33 + abac.go:94-99 + middleware.go 실재)
- `grep -n 'Action' apps/control-plane/internal/audit/audit.go` (23+ 상수 실재)
- 미존재/시그니처 불일치 시 즉시 STOP → 재계획 (메모리 lesson #9 흡수)

## 5. 디스크 산출물 (Plan-phase output)

`.moai/specs/SPEC-AX-AUDIT-QUERY-001/`:
- `spec.md` (이 SPEC, manager-spec 작성)
- `plan.md` (이 문서, manager-spec 작성)
- `acceptance.md` (G/W/T, manager-spec 작성)
- `research.md` (Phase 0.5 SSOT, manager-spec 작성)
- `spec-compact.md` (요약, manager-spec 작성)
- `strategy.md` (M0 OPEN 6건 RESOLVED, manager-strategy 후속)
- `plan-audit.md` (plan-auditor iter1/iter2 검증, plan-auditor 후속)
- `tasks.md` (Run-phase 작업 분해, manager-tdd 후속)
- `progress.md` (Run-phase 진행 기록, manager-tdd 후속)

## 6. 의존성 (External)

- 본 SPEC 의존: SPEC-AX-CTRL-001(GREEN, audit_logs 0001 마이그레이션) / SCORE-001(GREEN, RecordScore*) / SCORE-API-001(GREEN, handler 선례) / REVIEW-001(GREEN, RecordScoreReviewRequest*) / RUBRIC-001(GREEN, RecordRubric*) / EVID-001(GREEN, RecordEvidence*) / EVAL-ITEM-001(GREEN, RecordEvalItem* + AUD-1 surrogate) / AUTH-003(GREEN, ABAC narrowing + admin 우회) / REPORT-001(GREEN, read-only handler 선례). 모두 7-누적 SPEC.
- 본 SPEC 미의존: 향후 SPEC-AX-AUDIT-READ-AUDIT-001(audit-of-audit-read 후속), SPEC-AX-AUDIT-EXPORT-001(CSV/XLSX export 후속).

## 7. 위험 (Risks)

- **R-AUDIT-QUERY-001 [HIGH]** — admin-only narrowing 매핑 불일치: 핸들러-레벨 `requireAuditQueryReadRole`이 RoleAdmin scope 인식 실패 시 admin이 viewer/analyst와 동일 403. 완화: T-201/T-205로 admin scope("iroum-ax:admin") 매치 RED-first 검증, ABAC chain의 `hasAdminRole`(abac.go:94-99) 우회 경로와 핸들러 게이트 동시 검증.

- **R-AUDIT-QUERY-002 [HIGH]** — consumer-only 0-diff 위반: 신규 store SELECT 메서드 추가 시 `WorkflowStore` 인터페이스 확장(OPEN #5 §A.5)이 의도치 않게 기존 `WorkflowTx.InsertAuditLog` 시그니처 영향 가능성. 완화: T-306 `git diff pg_store.go:346-381` 0-diff 검증, M3 회귀 시 `WorkflowTx` 메서드 셋 변경 0 확인.

- **R-AUDIT-QUERY-003 [HIGH]** — frozen rbac.go 0-diff 위반: 구현자가 admin-only narrowing을 위해 `permissionMatrix`에 audit Permission 추가 시도 가능성(SCORE-API-001 §6 OPEN #4 동위상 위반). 완화: T-307 `git diff internal/auth/rbac.go` 0-diff 검증, 핸들러-레벨 helper만 사용 (`requireAuditQueryReadRole`).

- **R-AUDIT-QUERY-004 [MED]** — SQL injection 방어 누락: 5-필터 동적 빌더에서 string interpolation 사용 시 SQL injection 발생. 완화: T-008/T-308 injection 시도 테스트, pgx placeholder($N) 일관 사용, lint(gosec) 통과.

- **R-AUDIT-QUERY-005 [MED]** — audit-of-audit-read 부당 발생: 검색 핸들러가 부주의하게 `audit_logs` INSERT 시 audit-of-audit-read 재귀(§5 #2 범위 밖). 완화: T-305 정적 검사로 `Recorder` 의존 미주입 + audit INSERT SQL 0건 검증.

- **R-AUDIT-QUERY-006 [MED]** — D-1 mutation negative-control 누락: RED-first가 self-report fake GREEN인 경우 dark-flow iter2 lesson 위반. 완화: T-101 GREEN 후 store AND → OR 의도적 손상 → 테스트가 RED 전환 확인 (REPORT-001 D-1 negative-control 패턴 정합).

- **R-AUDIT-QUERY-007 [LOW]** — 인덱스 활용 미흡: query planner가 `audit_logs_user_id_timestamp_idx`(user_id 우선)를 활용하지 않을 가능성(ORDER BY timestamp 우선 → seq scan). 완화: M3 EXPLAIN 검증, 필요 시 ORDER BY 명시 또는 partial index 추가는 §1.4 HARD 위반이므로 ORDER BY 정렬만 조정.

- **R-AUDIT-QUERY-008 [LOW]** — total count window function 성능: `COUNT(*) OVER()`이 large audit_logs에 대해 full scan 발생 가능성. 완화: PoC scope 100K row 미만 가정, p99 < 200ms 목표 M3에서 검증, 미달 시 OPEN #6 (b) 별도 쿼리로 fallback 가능(SPEC 변경 없이 strategy.md §A.6 추가).

## 8. Definition of Done (Plan 단계)

- [ ] plan.md 8 섹션 모두 작성 (§1~§8)
- [ ] §2 M0~M4 5 마일스톤 priority 라벨로 분류 (시간 추정 0)
- [ ] §4 RED 테스트 enumeration ≥30 (acceptance.md AC 25 + edge 14 추적성)
- [ ] §3 기술 접근에 store SELECT 시그니처 + 동적 SQL + admin-only narrowing 명시
- [ ] §1 소비 시그니처 source-verified (audit.go:17-128 / recorder.go:295-305 / pg_store.go:346-381 / abac.go:4-99 / rbac.go:20-33 / middleware.go:25-49 / score_handlers.go:43-190 / report_handlers.go:43-72 / server.go:55-220/277-285 / initial.sql:115-138)
- [ ] §1 frozen scope 7 SPEC 누적 명시 (RoleAuditor 신설 절대 금지, permissionMatrix 무변경)
- [ ] §7 위험 ≥8 (admin-only/consumer-only/frozen rbac/SQL injection/audit-of-audit-read/D-1 mutation/인덱스/window function)
- [ ] dark-flow iter2 lesson 흡수 명시 (§2 M2 evaluator-active 무조건 실행, R-AUDIT-QUERY-006 D-1 mutation)
- [ ] SCORE-API-001 errors.go drift lesson EXPLICIT 부착 (§1 신규 도메인 + §2 M2 작업 1)
- [ ] REPORT-001 server.go ≈7줄 정확 기술 (§2 M2 작업 8)
- [ ] SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 (§1 frozen rbac 명시, §3.3 핸들러-레벨 helper만)
- [ ] D1 iter2 lesson 명시적 acknowledge (§1 신규 도메인 — read-only NewRecorder mode 무관 명시, mutation 메서드 0)
- [ ] §6 OPEN 6건 → M0 strategy 위임 명시
- [ ] AC/edge count 운반 0 (single source = acceptance.md §9 + spec.md §9 + spec-compact.md, SCORE-API-001 D3-1 정합)
- [ ] 구현 코드/테스트 미작성 (plan 문서만)
