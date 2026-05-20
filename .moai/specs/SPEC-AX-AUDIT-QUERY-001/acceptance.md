# SPEC-AX-AUDIT-QUERY-001 Acceptance Criteria

> Companion: `spec.md` (EARS), `plan.md` (TDD Sprint), `research.md` (Phase 0.5 SSOT)
> 형식: Given/When/Then. AC 명명: `AC-AUDIT-QUERY-{REQ}-{N}`. 각 REQ ≥2 AC.
> 검증 도구: `httptest.NewRequest`/`httptest.NewRecorder`, fake `WorkflowStore`/`WorkflowTx` (또는 `AuditQueryStore` — OPEN #5 RESOLVED 후), testify/assert, t.Parallel, goleak (plan.md §4).
> Total AC = **25** / §7 Edge Case = **14** (count cross-file 일관 — spec.md §9·spec-compact.md와 동일. **plan.md §8은 plan-phase DoD로 AC/edge count를 운반하지 않음** — SCORE-API-001 D3-1 정합. count single source of truth = acceptance.md §9 + spec.md §9 + spec-compact.md).

---

## 1. REQ-AUDIT-QUERY-UBI-001 — 데이터 주권 (AC ×2)

### AC-AUDIT-QUERY-UBI-001-1 — 외부 호출 0건 + 신규 외부 의존 0 (정적)

- **Given** `cmd/server/audit_query_handlers.go` + `internal/store/audit_query.go` 구현 소스 + `go.mod`/`go.sum`
- **When** import 목록·네트워크 호출(`http.Client`, 외부 SDK, 외부 URL)·신규 의존을 정적 검사
- **Then** 신규 외부 의존이 0건이며 `net/http`/`encoding/json`/`database/sql`/`strings`/`strconv`/`time`/`github.com/google/uuid`/`go.uber.org/zap`/`jackc/pgx`/`internal/store`/`internal/auth`/`internal/errors`/`internal/audit`만 사용한다 (REQ-AUDIT-QUERY-UBI-001, spec.md §1.4 HARD, research.md §7.1).

### AC-AUDIT-QUERY-UBI-001-2 — 모든 경로 store 위임

- **Given** 감사 검색 엔드포인트 핸들러
- **When** 각 핸들러의 데이터 접근 경로를 추적
- **Then** 모든 영속·조회·필터링이 `store.WorkflowStore`/`WorkflowTx.QueryAuditLogs` 메서드 호출(또는 `AuditQueryStore` OPEN #5 RESOLVED)만으로 이루어지고 외부망 egress가 0건이다 (망분리, research.md §7.1).

---

## 2. REQ-AUDIT-QUERY-UBI-002 — 감사 가능성 확장 (read-only, 자체 audit 0) (AC ×2)

### AC-AUDIT-QUERY-UBI-002-1 — API 자체 audit 0건 (read-only)

- **Given** `audit_query_handlers.go` 구현 소스
- **When** 핸들러 코드에서 `audit_logs` INSERT SQL 또는 `Recorder` 직접 호출 존재 여부 검사
- **Then** 모든 엔드포인트가 read-only(GET)이므로 mutation이 0건이고 API 핸들러는 자체 `audit_logs` INSERT를 0건 수행하며 `AuditQueryHandler`에 audit recorder 의존이 주입되지 않는다 (consumer-only §1.4 #4, research.md §6.1/§6.3).

### AC-AUDIT-QUERY-UBI-002-2 — mutation audit는 7 SPEC store가 이미 기록 (위임)

- **Given** 검색 결과로 반환되는 audit_logs row들
- **When** 검색 핸들러가 `QueryAuditLogs`를 호출하여 결과를 반환
- **Then** 핸들러는 audit_logs INSERT를 일으키지 않고(read-only) 모든 audit row는 이미 7 SPEC store(`RecordXxx` 동일 TX, audit.go:17-100의 23+ Action)가 mutation 시점에 기록한 상태이며, 본 SPEC은 audit-of-audit-read도 발생시키지 않는다 (REQ-AUDIT-QUERY-UBI-002, §5 #2, research.md §6.2).

---

## 3. REQ-AUDIT-QUERY-UBI-003 — 권한 (admin-only narrowing) (AC ×3)

### AC-AUDIT-QUERY-UBI-003-1 — admin scope read 허용

- **Given** `authEnabled=true`, principal scope="iroum-ax:admin" (RoleAdmin)
- **When** 감사 로그 검색 요청
- **Then** SPEC-AX-AUTH-003 ABAC narrowing 경로가 admin 우회(`abac.go:94-99 hasAdminRole`)로 통과하고 handler-level `requireAuditQueryReadRole`이 RoleAdmin 매핑 매치하여 true 반환, 검색 결과가 `200 OK`로 반환된다 (REQ-AUDIT-QUERY-UBI-003, §1.5).

### AC-AUDIT-QUERY-UBI-003-2 — viewer scope 403

- **Given** `authEnabled=true`, principal scope="iroum-ax:viewer" (RoleViewer)
- **When** 감사 로그 검색 요청
- **Then** handler-level `requireAuditQueryReadRole`이 RoleAdmin 비매핑으로 false 반환하고 `403 ABAC_CONDITION_DENIED` + `{"error":{"code":"ABAC_CONDITION_DENIED","message":"감사 로그 조회 권한이 없습니다",...}}` (한국어) 본문이 반환된다 (REQ-AUDIT-QUERY-002-U1).

### AC-AUDIT-QUERY-UBI-003-3 — analyst scope 403

- **Given** `authEnabled=true`, principal scope="iroum-ax:analyst" (RoleAnalyst)
- **When** 감사 로그 검색 요청
- **Then** handler-level `requireAuditQueryReadRole`이 false 반환하고 `403 ABAC_CONDITION_DENIED`가 반환된다 (감사 데이터는 admin only — analyst는 write 권한 있어도 read 부재, §1.5 — REPORT-001/SCORE-API-001 viewer 허용과 정반대 정책).

---

## 4. REQ-AUDIT-QUERY-UBI-004 — cli-anonymous 기본값 + auth-disabled fallback (AC ×2)

### AC-AUDIT-QUERY-UBI-004-1 — authEnabled=false 전 엔드포인트 투과

- **Given** `s.cfg.AuthEnabled = false` (Walking Skeleton 기본값)
- **When** 모든 감사 검색 엔드포인트에 인증 없는 요청을 전송
- **Then** ABAC/RBAC 미들웨어가 투과(`abac.go:8` REQ-ABAC-009)하고 handler-level `requireAuditQueryReadRole`/`guardAuditQueryRead`도 투과(`UserFromContext` ok=false → true 반환, `score_handlers.go:181` 패턴 동형)하여 모든 엔드포인트가 정상 응답한다 (REQ-AUDIT-QUERY-UBI-004, REQ-AUDIT-QUERY-002-S1, research.md §7.2).

### AC-AUDIT-QUERY-UBI-004-2 — 실 사용자 식별자 비위조

- **Given** 인증 비활성 환경의 검색 요청
- **When** 핸들러가 store에 전달하는 user context를 검사
- **Then** API 핸들러는 실 사용자 식별자를 위조·주입하지 않고 검색 필터의 user_id는 query string에 명시된 값만 사용한다(누출 0, read-only이므로 created_by 생성도 0).

---

## 5. REQ-AUDIT-QUERY-001 — 감사 로그 검색 API (AC ×9)

### AC-AUDIT-QUERY-001-1 — empty filter 검색 200

- **Given** 존재하는 audit_logs row N건, admin scope
- **When** `GET /api/v1/audit-logs`(필터 0개)
- **Then** 핸들러가 `WorkflowStore.BeginTx`(또는 `AuditQueryStore.BeginTx` — OPEN #5)→`QueryAuditLogs(ctx, AuditQueryFilter{}, defaultListLimit=50, offset=0)` 호출 결과 `200 OK` + `{"events":[...], "count":50, "total":N, "generated_at":"..."}` (정확 형식 §6 OPEN #3 RESOLVED) JSON, `ORDER BY timestamp DESC, user_id`로 정렬된 결과를 반환한다 (REQ-AUDIT-QUERY-001-E1, audit_logs_user_id_timestamp_idx 활용 `initial.sql:138`).

### AC-AUDIT-QUERY-001-2 — 5-filter AND 조합 200

- **Given** audit_logs에 다양한 row, admin scope
- **When** `GET /api/v1/audit-logs?action=SCORE_CREATED&resource_type=score&resource_id={uuid}&user_id=alice&since=2026-01-01T00:00:00Z&until=2026-12-31T23:59:59Z`
- **Then** 핸들러가 5-필터 AND 조합 동적 SQL(`WHERE 1=1 AND action=$1 AND resource_type=$2 AND resource_id=$3 AND user_id=$4 AND timestamp >= $5 AND timestamp <= $6`)로 결과 반환, `200 OK` + 매칭 events JSON. SQL injection 0(파라미터 바인딩만 `$N`) (REQ-AUDIT-QUERY-001-E1, REQ-AUDIT-QUERY-001-S2, research.md §10).

### AC-AUDIT-QUERY-001-3 — 결과 0건 → 200 빈 응답

- **Given** 매칭 row 0건 (예: `?action=UNKNOWN_ACTION_XYZ`)
- **When** 검색 요청
- **Then** `200 OK` + `{"events":[], "count":0, "total":0, "generated_at":"..."}`, 404/500 비반환 (REQ-AUDIT-QUERY-001-E2, REQ-AUDIT-QUERY-001-U2, data-completeness research.md §14.5, REPORT-001 §6.3 B-2 정합).

### AC-AUDIT-QUERY-001-4 — malformed resource_id → 400

- **Given** admin scope
- **When** `GET /api/v1/audit-logs?resource_id=not-a-uuid`
- **Then** `400 Bad Request` + `{"error":{"code":"INVALID_ARGUMENT","message":"resource_id가 유효한 UUID가 아닙니다","field":"resource_id"}}` (한국어), `errors.Is(err, apperrors.ErrAuditQueryInvalidFilter)`, store 미진입 (REQ-AUDIT-QUERY-001-U1, score_handlers.go pre-store 검증 선례).

### AC-AUDIT-QUERY-001-5 — malformed timestamp → 400

- **Given** admin scope
- **When** `GET /api/v1/audit-logs?since=2026-1-1` (non-RFC3339)
- **Then** `400 Bad Request` + 표준 에러 본문(`field="since"`), `errors.Is(err, apperrors.ErrAuditQueryInvalidFilter)`, store 미진입 (REQ-AUDIT-QUERY-001-U1).

### AC-AUDIT-QUERY-001-6 — since > until → 400

- **Given** admin scope
- **When** `GET /api/v1/audit-logs?since=2026-12-31T00:00:00Z&until=2026-01-01T00:00:00Z`
- **Then** `400 Bad Request` + `{"error":{"code":"INVALID_ARGUMENT","message":"since는 until보다 클 수 없습니다","field":"since"}}`, `errors.Is(err, apperrors.ErrAuditQueryInvalidTimeRange)`, store 미진입 (REQ-AUDIT-QUERY-001-U1).

### AC-AUDIT-QUERY-001-7 — future timestamp → 400

- **Given** admin scope, 현재 시각 < `until`
- **When** `GET /api/v1/audit-logs?until=2099-12-31T23:59:59Z`
- **Then** `400 Bad Request` + 표준 에러 본문(`field="until"`), `errors.Is(err, apperrors.ErrAuditQueryInvalidTimeRange)` (REQ-AUDIT-QUERY-001-U1, business logic 거부 — future 시점 검색 무의미).

### AC-AUDIT-QUERY-001-8 — unknown action → 200 빈 결과 (자유 문자열)

- **Given** admin scope
- **When** `GET /api/v1/audit-logs?action=UNKNOWN_ACTION_XYZ` (audit.go:17-100 23+ Action 상수 외 값)
- **Then** 핸들러가 `action` 필터를 enum 검증 없이 string으로 store에 전달, 매칭 row 0건이므로 `200 OK` + `{"events":[], "count":0, "total":0}` 반환, 400 비반환 (REQ-AUDIT-QUERY-001-U2 — `audit_logs.action`은 VARCHAR(64) 자유 문자열, `initial.sql:118`).

### AC-AUDIT-QUERY-001-9 — 페이지네이션 clampPagination 재사용

- **Given** audit_logs에 500+ row, admin scope
- **When** `GET /api/v1/audit-logs?limit=10000&offset=-5` (large limit + negative offset)
- **Then** `clampPagination`(score_handlers.go:144-157)이 limit=500(max), offset=0(음수 클램프) 적용, `200 OK` + 500개 events 반환, total 정확 (REQ-AUDIT-QUERY-001-O1, score_handlers.go:144 재사용).

---

## 6. REQ-AUDIT-QUERY-002 — ABAC admin-only narrowing 통합 (AC ×4)

### AC-AUDIT-QUERY-002-1 — admin scope 200 허용 (E1)

- **Given** `authEnabled=true`, `RoleAdmin` principal
- **When** 감사 검색 요청
- **Then** SPEC-AX-AUTH-003 ABAC narrowing 경로가 admin 우회(`abac.go:9` REQ-ABAC-004, `abac.go:94-99 hasAdminRole`)로 통과하고 handler-level `requireAuditQueryReadRole`도 매칭, `200 OK` 반환 (REQ-AUDIT-QUERY-002-E1, research.md §4.2).

### AC-AUDIT-QUERY-002-2 — auth-disabled 투과 (Walking Skeleton)

- **Given** `authEnabled=false`
- **When** 모든 검색 엔드포인트 각각 요청
- **Then** ABAC/RBAC 미들웨어가 투과하고 handler-level `requireAuditQueryReadRole`/`guardAuditQueryRead`도 투과(`UserFromContext` ok=false → true)하여 모든 엔드포인트가 정상 동작한다 (REQ-AUDIT-QUERY-002-S1, `server.go:287` 미들웨어 체인 비활성 시 투과 정합).

### AC-AUDIT-QUERY-002-3 — viewer scope 403 (U1)

- **Given** `authEnabled=true`, `RoleViewer` principal
- **When** 검색 요청
- **Then** handler-level `requireAuditQueryReadRole`이 false 반환(RoleAdmin 비매핑) + `403 ABAC_CONDITION_DENIED` (REQ-AUDIT-QUERY-002-U1).

### AC-AUDIT-QUERY-002-4 — RoleAuditor 미신설 + permissionMatrix 무변경 (U2 BOUNDARY)

- **Given** 본 SPEC 구현 완료 상태
- **When** `git diff apps/control-plane/internal/auth/rbac.go` 검사
- **Then** rbac.go 0-diff (RoleAuditor 신규 0, permissionMatrix 무변경, 정규식 `^iroum-ax:(admin|analyst|viewer)$` 무변경), admin-only 매핑은 핸들러-레벨 helper(`requireAuditQueryReadRole`)로만 구현 (REQ-AUDIT-QUERY-002-U2, frozen rbac.go [HARD], §5 #9, SCORE-API-001 §6 OPEN #4 lesson 동형).

---

## 7. REQ-AUDIT-QUERY-003 — store 에러→HTTP 매핑 & consumer-only 경계 (AC ×3)

### AC-AUDIT-QUERY-003-1 — 에러 센티넬→HTTP status 매핑 표

- **Given** store가 각 센티넬을 반환하는 상황 (fake `WorkflowTx`)
- **When** 핸들러가 `errors.Is`로 매핑 (`score_handlers.go:111-137 mapStoreErr` 미러)
- **Then** 다음 매핑이 결정적으로 성립한다:

| 센티넬 | HTTP |
|--------|------|
| `ErrAuditQueryInvalidFilter` (malformed UUID/timestamp/length) | 400 |
| `ErrAuditQueryInvalidTimeRange` (since>until/future timestamp) | 400 |
| unwrapped/unknown DB error | 500 |

(REQ-AUDIT-QUERY-003-S1).

### AC-AUDIT-QUERY-003-2 — read-only TX rollback + goroutine 누출 0 (BOUNDARY)

- **Given** 검색 핸들러가 `BeginTx`(또는 `BeginAuditQueryTx` OPEN #5) 후 downstream 호출이 실패
- **When** 요청 처리
- **Then** 열린 read TX가 `defer tx.Rollback(ctx)`로 정리되고(Commit 없음 — read-only), 부분 상태가 0이며 goroutine 누출이 0이다 (goleak) (REQ-AUDIT-QUERY-003-U1, research.md §3.3, report_handlers.go 선례 동형).

### AC-AUDIT-QUERY-BOUNDARY-1 — consumer-only 0-diff (BOUNDARY)

- **Given** 본 SPEC 구현 완료 상태
- **When** `git diff`로 다음 경로 변경 검사:
  - `internal/audit/{audit.go, recorder.go}` (Action 상수·Recorder·Event 무변경)
  - `internal/auth/{abac.go, rbac.go, middleware.go}` (frozen [HARD])
  - `cmd/server/{score, report, review, rubric, evidence}_handlers.go`
  - `internal/store/pg_store.go:346-381` (audit INSERT 부분 무변경)
  - `.moai/db/schema/migrations/0001~0006_*.sql` + `initial.sql`
  - `go.mod`/`go.sum`
- **Then** 위 경로의 변경이 **0 diff**이고, 신규 마이그레이션이 0건, 신규 `audit.Action` 상수가 0건, 신규 `Recorder` 메서드가 0건, 신규 외부 의존이 0건이며, `cmd/server/server.go`의 변경은 라우트 마운트(`auditQueryH` 필드 + `NewAuditQueryHandler` + `innerMux.Handle` 2줄, ≈7줄)로 한정되고, `internal/errors/errors.go`의 변경은 정확히 2 sentinel 신규 추가(`ErrAuditQueryInvalidFilter`/`ErrAuditQueryInvalidTimeRange`) 한정, `internal/store/{store.go, pg_store.go, audit_query.go}`의 변경은 read-only SELECT `QueryAuditLogs` 메서드 추가 한정이다 (REQ-AUDIT-QUERY-003-U2, consumer-only §1.4 [HARD], Drift-Guard manifest §2.3, SCORE-API-001 errors.go drift lesson EXPLICIT 부착).

---

## §7 Edge Case Catalog (14개)

| # | Edge Case | 기대 동작 | AC |
|---|-----------|----------|-----|
| 1 | 존재하지 않는 결과 (필터 매칭 0건) | 200 빈 응답 `{"events":[], "count":0, "total":0}` | AC-AUDIT-QUERY-001-3, 001-8 |
| 2 | malformed UUID resource_id | 400 + ErrAuditQueryInvalidFilter, store 미진입 | AC-AUDIT-QUERY-001-4 |
| 3 | malformed RFC3339 timestamp | 400 + ErrAuditQueryInvalidFilter, store 미진입 | AC-AUDIT-QUERY-001-5 |
| 4 | since > until | 400 + ErrAuditQueryInvalidTimeRange | AC-AUDIT-QUERY-001-6 |
| 5 | future timestamp (until > now) | 400 + ErrAuditQueryInvalidTimeRange | AC-AUDIT-QUERY-001-7 |
| 6 | unknown action 문자열 (audit.go:17-100 외) | 200 빈 결과 (자유 문자열, enum 검증 없음) | AC-AUDIT-QUERY-001-8 |
| 7 | authEnabled=false 전 엔드포인트 | 투과, 정상 응답 (handler-level helper도 투과) | AC-AUDIT-QUERY-UBI-004-1, 002-2 |
| 8 | viewer-only scope | 403 ABAC_CONDITION_DENIED | AC-AUDIT-QUERY-UBI-003-2, 002-3 |
| 9 | analyst scope | 403 ABAC_CONDITION_DENIED (admin only) | AC-AUDIT-QUERY-UBI-003-3 |
| 10 | RoleAdmin scope | 200 허용 (ABAC + handler-level 모두 매칭) | AC-AUDIT-QUERY-UBI-003-1, 002-1 |
| 11 | limit=10000 + offset=-5 | clampPagination max 500 + offset 0 적용 | AC-AUDIT-QUERY-001-9 |
| 12 | 5-필터 AND 조합 | 모든 필터 AND, SQL injection 0 (parameter binding) | AC-AUDIT-QUERY-001-2 |
| 13 | read-only TX downstream 실패 | defer Rollback, goroutine 누출 0, 부분 상태 0 | AC-AUDIT-QUERY-003-2 |
| 14 | consumer-only 경계 (audit/auth/handlers/schema/go.mod/audit INSERT 0-diff + frozen rbac 0-diff + API audit 0 + 신규 Action 0) | git diff 0, audit INSERT 0, RoleAuditor 미신설, 신규 외부 의존 0, errors.go 정확 2 sentinel | AC-AUDIT-QUERY-BOUNDARY-1, UBI-002-1, 002-4 |

---

## §9 Definition of Done (Acceptance 단계)

- [ ] Total AC = **25** (UBI-001 2 + UBI-002 2 + UBI-003 3 + UBI-004 2 + 001 9 + 002 4 + 003 3 = 25), 컴포넌트 분해 일치
- [ ] §7 Edge Case Catalog = **14개** 물리적 행, 각 AC 추적
- [ ] 각 REQ ≥2 G/W/T, AC 명명 `AC-AUDIT-QUERY-{REQ}-{N}`
- [ ] AC 25 / edge 14 count가 spec.md §9 · spec-compact.md와 cross-file 일관 (count single source of truth = acceptance.md §9 + spec.md §9 + spec-compact.md; plan.md §8은 plan-phase DoD로 count 미운반 — SCORE-API-001 D3-1 정합)
- [ ] consumer-only [HARD] 검증 AC (AC-AUDIT-QUERY-BOUNDARY-1: audit/auth/7-SPEC-handlers/migrations/pg_store.go audit INSERT/go.mod 0-diff, 신규 마이그레이션 0, 신규 Action 상수 0, 신규 Recorder 메서드 0, 신규 외부 의존 0, 자체 audit 0)
- [ ] **frozen rbac.go 0-diff [CRITICAL]** 검증 AC (AC-AUDIT-QUERY-002-4: RoleAuditor 미신설, permissionMatrix 무변경, 정규식 무변경, 핸들러-레벨 helper만)
- [ ] **SCORE-API-001 errors.go drift lesson [HARD]** EXPLICIT 부착 검증 — AC-AUDIT-QUERY-BOUNDARY-1에 정확 2 sentinel 신규 한정 명시
- [ ] admin-only narrowing 검증 AC (AC-AUDIT-QUERY-UBI-003-1/2/3 + 002-1/3): admin 200 / viewer 403 / analyst 403, REPORT-001/SCORE-API-001 viewer 허용과 정반대 정책
- [ ] phantom API 0 — 모든 AC가 source-verified 시그니처(audit.go:17-128 / recorder.go:295-305 / pg_store.go:346-381 / abac.go:4-99 / rbac.go:20-33 / middleware.go:25-49 / score_handlers.go:43-190 / report_handlers.go:43-72 / server.go:55-220/277-285 / initial.sql:115-138 / go.mod)에 추적 (메모리 lesson #9)
- [ ] 구현 코드/테스트 미작성 (acceptance 문서만)
