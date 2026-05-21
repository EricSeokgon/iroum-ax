# SPEC-AX-AUDIT-QUERY-001 Research (Phase 0.5 SSOT)

> Companion: `spec.md` (EARS) / `plan.md` (Sprint) / `acceptance.md` (G/W/T) / `spec-compact.md` (요약)
> 본 문서는 SPEC의 Single Source of Truth. 모든 EARS 요구사항·영향 파일·HTTP 계약은 본 문서가 인용한 file:line 근거에 추적된다. Phantom API 0 — orchestrator ground-truth grep 2026-05-20.

## §1. 도메인 위치 (Anchor)

### 1.1 7 SPEC 누적 — 본 SPEC의 consumer 입력

iroum-ax는 KEPCO E&C 한국 공공 기관 경영평가 PoC. 본 SPEC은 7번째 SPEC(누적 11 SPEC 완료 후)로 다음 누적 audit 적재 데이터를 검색 표면화한다:
- SPEC-AX-CTRL-001 (완료): `audit_logs` 테이블 + `audit_logs_user_id_timestamp_idx` 생성 (`.moai/db/schema/initial.sql:115-138`)
- SPEC-AX-SCORE-001 (완료 v0.1.3): `RecordScoreCreated`/`RecordScoreUpdated` 동일-TX (`audit/recorder.go:371/393`)
- SPEC-AX-REVIEW-001 (완료): `RecordScoreReviewRequest{Created,Assigned,Approved,Rejected}` (`audit/recorder.go:446`)
- SPEC-AX-RUBRIC-001 (완료): `RecordRubric{Created,Updated,Archived,CriterionAdded,BandAdded}` (`audit/recorder.go:559/631/656`)
- SPEC-AX-EVID-001 (완료): `RecordEvidence{Created,Versioned}` (`audit/recorder.go:243/269`)
- SPEC-AX-EVAL-ITEM-001 (완료): `RecordEvalItem{Created,Updated}` + AUD-1 UUIDv5 surrogate (`audit/recorder.go:327/349`, `audit.go:115 EvalItemAuditNamespace`)
- SPEC-AX-AUTH-003 (완료): ABAC narrowing-only + admin 우회 (`auth/abac.go:4/9/94-99`)
- SPEC-AX-SCORE-API-001 (완료 v0.1.1): write-role 게이트 핸들러-레벨 helper 패턴 — 본 SPEC의 admin-only narrowing 패턴 동형 미러 (`score_handlers.go:161-190`)
- SPEC-AX-REPORT-001 (완료 v0.1.1): read-only handler 선례 (`report_handlers.go:43-72`)

### 1.2 분리된 책임 — searchability vs adding

본 SPEC이 추가하는 것: **searchability**(검색 가능성) — 7 SPEC 누적 데이터의 admin 검색 API.
본 SPEC이 추가하지 않는 것: audit logging(적재) — 이미 7 SPEC store가 동일-TX로 제공.
두 책임은 분리. 본 SPEC은 read-only — mutation 0, audit-of-audit-read 0 (§5 #2).

## §2. audit_logs 스키마 (CTRL-001, frozen)

### 2.1 테이블 정의

source: `.moai/db/schema/initial.sql:115-123`

```sql
CREATE TABLE IF NOT EXISTS audit_logs (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        VARCHAR(64)   NOT NULL DEFAULT 'cli-anonymous',
    action         VARCHAR(64)   NOT NULL,
    resource_id    UUID          NOT NULL,
    resource_type  VARCHAR(32),
    timestamp      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    details        JSONB
);
```

핵심:
- `user_id`: VARCHAR(64) default 'cli-anonymous' — Walking Skeleton 정합 (REQ-AUDIT-QUERY-UBI-004)
- `action`: VARCHAR(64) — **자유 문자열, DB enum 검증 없음**. 7 SPEC 누적 23+ 상수가 자유 문자열로 저장됨. unknown action 필터는 200 빈결과 (AC-AUDIT-QUERY-001-8)
- `resource_id`: UUID NOT NULL — EVAL-ITEM-001은 VARCHAR(64) hierarchy_code를 UUIDv5 surrogate로 변환하여 저장 (`recorder.go:295-305`)
- `resource_type`: VARCHAR(32) — 도메인 식별자(evidence/evaluation_item/score/score_review_request/rubric 등). 부재 가능 (NULL allowed)
- `timestamp`: TIMESTAMP WITH TIME ZONE — 한국 공공 감사 시간 추적성, RFC3339 정합
- `details`: JSONB — 도메인별 메타데이터. EVAL-ITEM은 hierarchy_code/eval_item_id 보존 (`recorder.go:307-323 evalItemDetails`)

### 2.2 인덱스

source: `.moai/db/schema/initial.sql:138`

```sql
CREATE INDEX IF NOT EXISTS audit_logs_user_id_timestamp_idx
    ON audit_logs (user_id, timestamp DESC);
```

핵심: B-tree composite index `(user_id, timestamp DESC)`. user_id filter 시 효율적 + timestamp 역순 정렬 효율적. 본 SPEC의 `ORDER BY timestamp DESC, user_id`는 정확히 이 인덱스를 활용. 추가 인덱스 신설 0 (§1.4 HARD).

## §3. Action 상수 23+개 (audit.go, 7 SPEC 누적)

source: `apps/control-plane/internal/audit/audit.go:17-100`

```go
const (
    // WORKFLOW (CTRL-001)
    ActionWorkflowCreated                Action = "WORKFLOW_CREATED"
    ActionWorkflowTransitionedToRunning  Action = "WORKFLOW_TRANSITIONED_TO_RUNNING"
    ActionWorkflowCompleted              Action = "WORKFLOW_COMPLETED"
    ActionWorkflowFailedDispatch         Action = "WORKFLOW_FAILED_DISPATCH"
    ActionWorkflowFailedCallback         Action = "WORKFLOW_FAILED_CALLBACK"
    ActionTransitionRejected             Action = "TRANSITION_REJECTED"
    ActionCallbackRejectedTerminal       Action = "CALLBACK_REJECTED_TERMINAL"
    ActionWorkflowCreateCancelled        Action = "WORKFLOW_CREATE_CANCELLED"
    // AUTH (AUTH-002, AUTH-003)
    ActionAuthForbidden                  Action = "AUTH_FORBIDDEN"
    ActionAuthLogout                     Action = "AUTH_LOGOUT"
    ActionAuthRefreshReuseDetected       Action = "AUTH_REFRESH_REUSE_DETECTED"
    ActionABACDenied                     Action = "ABAC_CONDITION_DENIED"
    // SERVER (SERVER-001)
    ActionServerStartup                  Action = "SERVER_STARTUP"
    ActionServerShutdownInitiated        Action = "SERVER_SHUTDOWN_INITIATED"
    ActionServerShutdownCompleted        Action = "SERVER_SHUTDOWN_COMPLETED"
    // EVID (EVID-001)
    ActionEvidenceCreated                Action = "EVIDENCE_CREATED"
    ActionEvidenceVersioned              Action = "EVIDENCE_VERSIONED"
    // EVAL-ITEM (EVAL-ITEM-001) — AUD-1 UUIDv5 surrogate (recorder.go:303-305)
    ActionEvalItemCreated                Action = "EVAL_ITEM_CREATED"
    ActionEvalItemUpdated                Action = "EVAL_ITEM_UPDATED"
    // SCORE (SCORE-001)
    ActionScoreCreated                   Action = "SCORE_CREATED"
    ActionScoreUpdated                   Action = "SCORE_UPDATED"
    // REVIEW (REVIEW-001) — 4-state machine
    ActionScoreReviewRequestCreated           Action = "SCORE_REVIEW_REQUEST_CREATED"
    ActionScoreReviewRequestReviewerAssigned  Action = "SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED"
    ActionScoreReviewRequestApproved          Action = "SCORE_REVIEW_REQUEST_APPROVED"
    ActionScoreReviewRequestRejected          Action = "SCORE_REVIEW_REQUEST_REJECTED"
    // RUBRIC (RUBRIC-001) — 5 actions
    ActionRubricCreated                  Action = "RUBRIC_CREATED"
    ActionRubricUpdated                  Action = "RUBRIC_UPDATED"
    ActionRubricArchived                 Action = "RUBRIC_ARCHIVED"
    ActionRubricCriterionAdded           Action = "RUBRIC_CRITERION_ADDED"
    ActionRubricBandAdded                Action = "RUBRIC_BAND_ADDED"
)
```

총 29개 상수 확인. 본 SPEC은 **신규 Action 상수 0건** 추가(§1.4 HARD, §5 #10).

## §4. Event struct + InsertAuditLog (PgWorkflowTx)

### 4.1 Event struct

source: `apps/control-plane/internal/audit/audit.go:121-128`

```go
type Event struct {
    Timestamp    time.Time
    Action       Action
    ResourceType string
    UserID       string
    DetailsJSON  []byte
    ResourceID   uuid.UUID
}
```

### 4.2 InsertAuditLog 구현

source: `apps/control-plane/internal/store/pg_store.go:346-381`

```go
func (t *PgWorkflowTx) InsertAuditLog(ctx context.Context, e *audit.Event) error {
    const query = `
        INSERT INTO audit_logs (id, action, resource_type, resource_id, user_id, details, timestamp)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
    id := uuid.New() // ID 자동 생성
    var details interface{}
    if len(e.DetailsJSON) > 0 {
        details = e.DetailsJSON
    }
    _, err := t.tx.Exec(ctx, query, id, string(e.Action), e.ResourceType, e.ResourceID, e.UserID, details, e.Timestamp)
    ...
}
```

핵심:
- id 자동 생성 (uuid.New())
- DetailsJSON []byte → pgx JSONB 직렬화
- timestamp 보존 (Event.Timestamp 직접 전달)
- **본 SPEC은 이 INSERT 코드를 수정하지 않는다 (`pg_store.go:346-381` 0-diff [HARD])**

### 4.3 AUD-1 UUIDv5 surrogate (EVAL-ITEM-001)

source: `apps/control-plane/internal/audit/audit.go:115` + `apps/control-plane/internal/audit/recorder.go:295-305`

```go
var EvalItemAuditNamespace = uuid.MustParse("a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b")

func evalItemResourceID(hierarchyCode string) uuid.UUID {
    return uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))
}
```

핵심: VARCHAR(64) hierarchy_code → 고정 namespace 기반 UUIDv5 결정적 변환. evaluation_items domain은 audit_logs.resource_id에 UUIDv5 surrogate를 저장하고 실 식별자(`eval_item_id`, `hierarchy_code`)는 details JSONB에 보존(`recorder.go:307-323 evalItemDetails`).

**본 SPEC 영향**: 검색 사용자가 hierarchy_code(예: "2.1.3.4")로 audit를 검색하려면 `details->>'eval_item_id'=$1` JSONB 부분 검색 필요. **§6 OPEN #4 RESOLVED 권장 = PoC 미적용**(이연) — 5-필터만 + `resource_type='evaluation_item'` filter로 우회 가능.

## §5. read-only 핸들러 선례

### 5.1 SCORE-API-001 (score_handlers.go) — write-role helper 패턴

source: `apps/control-plane/cmd/server/score_handlers.go:161-190`

```go
// requireScoreWriteRole scope 문자열에서 추출한 역할이 write 권한({admin,analyst})인지 판단한다.
func requireScoreWriteRole(scope string) bool {
    for _, r := range auth.ParseRolesFromScope(scope) {
        if r == auth.RoleAdmin || r == auth.RoleAnalyst {
            return true
        }
    }
    return false
}

func (h *ScoreHandler) guardScoreWrite(w http.ResponseWriter, r *http.Request) bool {
    u, ok := auth.UserFromContext(r.Context())
    if !ok {
        return true // auth-disabled 투과
    }
    if requireScoreWriteRole(strings.Join(u.Scopes, " ")) {
        return true
    }
    h.writeScoreErr(w, http.StatusForbidden, auth.ErrCodeABACDenied,
        "쓰기 권한이 없는 사용자입니다", "")
    return false
}
```

핵심: **frozen rbac.go·permissionMatrix를 수정하지 않고** 핸들러-레벨에서 write/read 역할 게이트 처리. `auth.ParseRolesFromScope`로 RoleAdmin/RoleAnalyst 매핑 식별. SCORE-API-001 §6 OPEN #4 RESOLVED 패턴.

**본 SPEC 미러 적용**: `requireAuditQueryReadRole`(RoleAdmin 단일 매핑) + `guardAuditQueryRead`(viewer/analyst → 403 ABAC_CONDITION_DENIED + "감사 로그 조회 권한이 없습니다" 한국어 메시지). admin-only narrowing.

### 5.2 clampPagination (score_handlers.go)

source: `apps/control-plane/cmd/server/score_handlers.go:140-157`

```go
const (
    defaultListLimit = 50
    maxListLimit     = 500
)

func clampPagination(rawLimit, rawOffset string) (int, int) {
    limit := defaultListLimit
    if v, err := strconv.Atoi(rawLimit); err == nil && v > 0 {
        limit = v
    }
    if limit > maxListLimit {
        limit = maxListLimit
    }
    offset := 0
    if v, err := strconv.Atoi(rawOffset); err == nil && v > 0 {
        offset = v
    }
    return limit, offset
}
```

본 SPEC 재사용: 동일 함수 호출(코드 무수정), default=50/max=500/offset 음수→0 클램프.

### 5.3 REPORT-001 (report_handlers.go) — read-only TX 패턴

source: `apps/control-plane/cmd/server/report_handlers.go:43-72`

```go
type ReportHandler struct {
    scoreStore    store.ScoreStore
    evalItemStore store.EvalItemStore
    logger        *zap.Logger
}

func NewReportHandler(ss store.ScoreStore, eis store.EvalItemStore, logger *zap.Logger) *ReportHandler {
    return &ReportHandler{scoreStore: ss, evalItemStore: eis, logger: logger}
}
```

핵심: **recorder 미주입** (read-only — mutation 0 → audit 0). 본 SPEC AuditQueryHandler도 동일하게 recorder 미주입.

### 5.4 server.go 라우트 마운트 (REPORT-001 선례)

source: `apps/control-plane/cmd/server/server.go:55-60, 216-220, 277-285`

```go
// Server struct field (server.go:55-60)
reportH *ReportHandler
reviewH *ReviewHandler
rubricH *RubricHandler

// 생성자 호출 (server.go:216-220)
s.reportH = NewReportHandler(pgStore, pgStore, logger)

// 라우트 마운트 (server.go:277-278)
innerMux.Handle("/api/v1/reports", s.reportH.Routes())
innerMux.Handle("/api/v1/reports/", s.reportH.Routes())
```

본 SPEC 미러 정확 ≈7줄:
```go
// auditQueryH 필드 추가 (server.go:55-60 위치)
auditQueryH *AuditQueryHandler

// 생성자 호출 (server.go:216-220 위치)
s.auditQueryH = NewAuditQueryHandler(pgStore, logger)

// 라우트 마운트 (server.go:277-278 패턴 미러)
innerMux.Handle("/api/v1/audit-logs", s.auditQueryH.Routes())
innerMux.Handle("/api/v1/audit-logs/", s.auditQueryH.Routes())
```

REPORT-001 lesson: server.go 정확 ≈7줄(필드+생성자+2 routes+ko 주석). 더 많은 변경 시 consumer-only [HARD] 위반.

## §6. ABAC/RBAC — admin-only narrowing 통합

### 6.1 RBAC 3-role frozen

source: `apps/control-plane/internal/auth/rbac.go:20-33`

```go
const (
    RoleAdmin   Role = "admin"
    RoleAnalyst Role = "analyst"
    RoleViewer  Role = "viewer"
)

// Scope 정규식
// ^iroum-ax:(admin|analyst|viewer)$
```

**[HARD]** 3-role 고정. `RoleAuditor` 신설 0 (§5 #9). 본 SPEC의 admin-only 매핑은 핸들러-레벨 helper로 처리(§5.1 SCORE-API-001 패턴 동형).

### 6.2 ABAC narrowing-only + admin 우회

source: `apps/control-plane/internal/auth/abac.go:4/9/94-99`

```go
// abac.go:4 — narrowing-only
//  1. ABAC는 RBAC가 통과시킨 요청만 추가 거부할 수 있다 (allow 부여 불가)
//  2. RoleAdmin 보유 → 투과 (REQ-ABAC-004)

func hasAdminRole(user *User) bool {
    roles := ParseRolesFromScope(strings.Join(user.Scopes, " "))
    for _, role := range roles {
        if role == RoleAdmin {
            return true
        }
    }
    return false
}
```

핵심: admin은 ABAC chain 우회. 본 SPEC의 admin은 ABAC narrowing 경로상 자연 허용.

### 6.3 UserFromContext (middleware.go)

source: `apps/control-plane/internal/auth/middleware.go:44-49`

```go
func UserFromContext(ctx context.Context) (*User, bool) {
    u, ok := ctx.Value(userContextKey).(*User)
    ...
}
```

핵심: auth-disabled(`AuthEnabled=false`) 시 `ok=false` → handler-level helper 투과 (`score_handlers.go:181` 동형).

## §7. 비기능 요구사항 근거

### 7.1 데이터 주권 (망분리)

source: `apps/control-plane/go.mod:1-30` + `.claude/rules/moai/languages/go.md`

본 SPEC은 어떤 신규 외부 의존도 `go.mod`에 추가하지 않는다. 검색 쿼리는 표준 `database/sql`/`pgx` + 기존 7 SPEC 누적 의존만 사용. CSV/XLSX export 라이브러리(예: `encoding/csv`/`xuri/excelize`) 도입 0건 (§5 #6).

### 7.2 cli-anonymous 기본값

source: `apps/control-plane/internal/store/pg_store.go:192` + `initial.sql:117`

audit_logs.user_id DEFAULT 'cli-anonymous'. 본 SPEC은 user_id 필터를 query string에서만 받음 — 실 사용자 식별자를 위조·주입하지 않는다(AC-AUDIT-QUERY-UBI-004-2).

## §8. 에러 처리

### 8.1 에러 센티넬 (errors.go) — 정확 2건 신규

source: `apps/control-plane/internal/errors/errors.go:1-145`

기존 145줄 무변경. 본 SPEC 신규 추가 정확 2건:
- `ErrAuditQueryInvalidFilter`: malformed UUID, malformed RFC3339 timestamp, length out of bounds (resource_type>32, user_id>64)
- `ErrAuditQueryInvalidTimeRange`: since > until, future timestamp(until > now)

**[HARD] SCORE-API-001 errors.go drift lesson**: spec.md §2.1 + §2.3 Drift-Guard manifest 양쪽 EXPLICIT 부착 — manifest 분실 방지. 기존 sentinel 무수정.

### 8.2 에러→HTTP status 매핑 (mapStoreErr 선례)

source: `apps/control-plane/cmd/server/score_handlers.go:111-137`

본 SPEC mapAuditQueryStoreErr (신규):
- `errors.Is(err, apperrors.ErrAuditQueryInvalidFilter)` → 400 "INVALID_ARGUMENT"
- `errors.Is(err, apperrors.ErrAuditQueryInvalidTimeRange)` → 400 "INVALID_ARGUMENT"
- unwrapped/unknown → 500 "INTERNAL"

INFO 로그(거부) vs ERROR 로그(서버 결함) 분리 — score_handlers.go:98-101 미러.

## §9. 응답 schema (§6 OPEN #3 RESOLVED 권장)

```json
{
  "events": [
    {
      "id": "{uuid}",
      "user_id": "alice",
      "action": "SCORE_CREATED",
      "resource_id": "{uuid}",
      "resource_type": "score",
      "timestamp": "2026-05-20T10:30:00Z",
      "details": { ... }
    }
  ],
  "count": 50,
  "total": 1247,
  "generated_at": "2026-05-20T10:30:05Z"
}
```

핵심:
- `events`: paged 결과 (limit 적용)
- `count`: events 슬라이스 길이 (clamp 후 limit과 같거나 작음)
- `total`: `COUNT(*) OVER()` window function 결과 — 전체 매칭 row 수 (페이지네이션 UX)
- `generated_at`: 응답 생성 시각 (RFC3339)

## §10. 정밀도·SQL injection 방어

### 10.1 SQL injection 방어 (pgx placeholder)

source: `apps/control-plane/internal/store/pg_store.go:346-381` 패턴 정합

본 SPEC 5-필터 동적 SQL은 string interpolation 0, pgx placeholder `$1`/`$2`/... 일관 사용. 동적 placeholder index 슬라이스 append 패턴:

```
sqlBuilder := strings.Builder{}
sqlBuilder.WriteString("WHERE 1=1")
args := []any{}
argIdx := 1
if filter.Action != nil {
    sqlBuilder.WriteString(fmt.Sprintf(" AND action = $%d", argIdx))
    args = append(args, *filter.Action)
    argIdx++
}
// ... 5 필터 반복
sqlBuilder.WriteString(fmt.Sprintf(" ORDER BY timestamp DESC, user_id LIMIT $%d OFFSET $%d", argIdx, argIdx+1))
args = append(args, limit, offset)
```

T-008/T-308 RED 테스트: injection 시도 (`action="'; DROP TABLE audit_logs; --"`) → 파라미터로 바인딩되어 안전.

### 10.2 timestamp 정밀도

source: `audit.go:121-128 Event.Timestamp time.Time` + `initial.sql:121 TIMESTAMP WITH TIME ZONE`

RFC3339 정합. `pgx`가 `time.Time` ↔ `TIMESTAMP WITH TIME ZONE` 자동 변환. 본 SPEC은 timestamp 부동소수점 산술 0 — 정확 비교만(>= since, <= until).

## §11. 테스트 검증 도구

### 11.1 단위 테스트

- `httptest.NewRequest`/`httptest.NewRecorder` (cmd/server)
- testify/assert (`stretchr/testify` 기존 의존)
- t.Parallel (독립 테스트 병렬)
- goleak (goroutine 누출 0 검증, T-304)
- table-driven (T-001~T-308, 5-필터 조합 다수)

### 11.2 store 모킹

OPEN #5 RESOLVED 후 정확 위치:
- (a) [권장] `fake_store.go` 또는 신규 `fake_audit_query_store.go`에 `QueryAuditLogs` mock 구현
- audit_logs row 시드 데이터: testdata-driven (`internal/store/testdata/audit_logs_seed.json`)

### 11.3 D-1 negative-control mutation test

source: REPORT-001 D-1 lesson — `score_handlers.go` 테스트가 `score.go` 변경에 대한 negative-control

본 SPEC T-101 GREEN 후:
1. `internal/store/audit_query.go`의 동적 SQL을 의도적으로 손상 (예: `AND` → `OR` 치환)
2. T-101 재실행 → RED 전환 확인
3. 손상 복구 → T-101 GREEN 복귀

목적: self-report fake GREEN 적발 (dark-flow iter2 lesson — feedback_dark_flow_iter2_pattern.md).

## §12. consumer-only 경계 (load-bearing)

### 12.1 [HARD] 절대 수정 금지

- `internal/audit/{audit.go, recorder.go}` — Action 상수·Recorder·Event·EvalItemAuditNamespace UUIDv5 무변경
- `internal/auth/{abac.go, rbac.go, middleware.go}` — **frozen [CRITICAL]**, RoleAuditor 신설 0, permissionMatrix 무변경
- `cmd/server/{score, report, review, rubric, evidence}_handlers.go` — 패턴 미러 참조만
- `internal/store/pg_store.go:346-381` — audit INSERT 부분 무수정 (mutation 7-SPEC 위임)
- `.moai/db/schema/migrations/0001~0006_*.sql` + `initial.sql` — 7 SPEC 누적 무수정
- `go.mod`/`go.sum` — 신규 외부 의존 0

### 12.2 [MODIFY] 허용 (정확 범위)

- `cmd/server/server.go` — 정확 ≈7줄 (필드 + 생성자 + 2 routes + ko 주석)
- `internal/store/store.go` — `AuditQueryFilter` struct + `WorkflowStore.QueryAuditLogs` 인터페이스 (OPEN #5 §A.5)
- `internal/store/pg_store.go` — `QueryAuditLogs` 위임 메서드 (audit INSERT 부분 `pg_store.go:346-381` 무변경)
- `internal/errors/errors.go` — 정확 2 sentinel 신규 (`ErrAuditQueryInvalidFilter`/`ErrAuditQueryInvalidTimeRange`) — SCORE-API-001 drift lesson EXPLICIT 부착

### 12.3 [NEW] 허용

- `cmd/server/audit_query_handlers.go` (구현)
- `cmd/server/audit_query_handlers_test.go` (테스트)
- `internal/store/audit_query.go` (SELECT 구현)
- `internal/store/audit_query_test.go` (테스트)

## §13. 한국 공공 6제약

### 13.1 적용 5제약 (본 SPEC 범위)

1. **데이터 주권 (망분리)** — 외부 API 호출 0, store(내부 pgx) 위임만 (`tech.md` §9.1, AC-AUDIT-QUERY-UBI-001-1/2)
2. **한국어** — 모든 에러 메시지 한국어 (`score_handlers.go:74-101` 선례, `report_handlers.go` 미러)
3. **감사 가능성 확장** — searchability 추가 (검색 API), audit logging은 7 SPEC store가 이미 제공 (AC-AUDIT-QUERY-UBI-002-1/2)
4. **조직 격리** — admin 권한자만 검색 허용 (감사 책임자 권한, AC-AUDIT-QUERY-UBI-003-1/2/3)
5. **(망분리 재확인)** — 신규 외부 의존 0, single pgx pool

### 13.2 미적용 1제약 (§5 #7)

6. **6번째 시간 제약(KST 업무시간 09:00-18:00)** — AUTH-003 / SCORE-API-001 / REPORT-001 / REVIEW-001 / RUBRIC-001 동일하게 본 SPEC 범위 밖. 후속 SPEC 책임.

## §14. 권장안 (§6 OPEN RESOLVED proposal)

### 14.1 OPEN #1 권장 = (a) 핸들러-레벨 helper

`requireAuditQueryReadRole`(RoleAdmin 단일 매핑) + `guardAuditQueryRead` (`score_handlers.go:161-187` 동형 미러). frozen rbac.go 0-diff 자연 성립. SCORE-API-001 §6 OPEN #4 lesson 동형 적용.

### 14.2 OPEN #2 권장 = (a) AND only

5-필터 모두 optional pointer, non-nil filter만 `AND` 추가. PoC 단순.

### 14.3 OPEN #3 권장 = (a) 4건 모두

- 응답 schema: `{events, count, total, generated_at}`
- 페이지네이션: default 50, max 500 (`clampPagination` 재사용)
- endpoint URL: `/api/v1/audit-logs`
- 빈 결과: 200 `{events:[], count:0, total:0}` (REPORT-001 §6.3 B-2)

### 14.4 OPEN #4 권장 = (a) PoC 미적용(이연)

JSONB details 부분 검색은 후속 SPEC. PoC scope에서 5-필터(action/resource_type/resource_id/user_id/since/until) AND only로 충분.

### 14.5 OPEN #5 권장 = (a) WorkflowStore 확장

`WorkflowStore` 인터페이스에 `QueryAuditLogs` 메서드 추가. `WorkflowTx.InsertAuditLog`(기존, mutation)와 `WorkflowTx.QueryAuditLogs`(신규, read-only) 공존. store.go [MODIFY] 1메서드.

### 14.6 OPEN #6 권장 = (a) COUNT(*) OVER()

window function로 1 round-trip에 결과·total 동시 산출. PoC scope 100K row 미만 성능 충분.

## §15. Lesson 흡수 (메모리 lesson #9, dark-flow iter2, D1 iter2)

- **메모리 lesson #9 (phantom API)**: 본 SPEC 모든 소비 시그니처 source-verified — file:line 인용. Phantom 0. (audit.go:17-128 / recorder.go:295-305 / pg_store.go:346-381 / abac.go:4-99 / rbac.go:20-33 / middleware.go:25-49 / score_handlers.go:43-190 / report_handlers.go:43-72 / server.go:55-220/277-285 / initial.sql:115-138 / go.mod)
- **dark-flow iter2 lesson (feedback_dark_flow_iter2_pattern.md)**: Phase 2 후 evaluator-active 무조건 실행 (plan.md §2 M2). self-report fake GREEN 방지. D-1 negative-control mutation test 필수 (T-101 GREEN 후 store 의도적 손상 → RED 전환 확인).
- **D1 iter2 lesson (project_pg_store_recorder_bool.md)**: read-only API라 `NewRecorder(true/false)` 결정 무관 — mutation 메서드 0이므로 Recorder 의존 미주입. spec.md §1.4에 명시적 acknowledge.
- **SCORE-API-001 errors.go drift lesson [HARD]**: 신규 sentinel(정확 2건: `ErrAuditQueryInvalidFilter`/`ErrAuditQueryInvalidTimeRange`)는 spec.md §2.1 + §2.3 Drift-Guard manifest 양쪽 EXPLICIT 부착. manifest 분실 방지.
- **REPORT-001 server.go ≈7줄 lesson**: server.go 정확 ≈7줄 (필드 + 생성자 + 2 routes + ko 주석). 더 많은 변경 시 consumer-only [HARD] 위반.
- **SCORE-API-001 §6 OPEN #4 (evaluator-INFEASIBLE) 비재발**: admin 매핑이 `rbac.go:20-21 RoleAdmin`에 기존재. 신규 역할 신설(RoleAuditor) 절대 금지. 핸들러-레벨 helper로 해소(§5.1 패턴 동형).
- **consumer-only [HARD] 0-diff orchestrator grep verification (feedback_consumer_only_orchestrator_grep.md)**: M2 GREEN 종료 시 `git diff --quiet -- internal/audit internal/auth cmd/server/{score,report,review,rubric,evidence}_handlers.go .moai/db/schema go.mod` 명시적 검증. teammate 자가보고만 신뢰 금지.

## §16. 본 SPEC 후속 (out of scope, 후속 SPEC 후보)

- SPEC-AX-AUDIT-READ-AUDIT-001 (audit-of-audit-read 추적, §5 #2)
- SPEC-AX-AUDIT-EXPORT-001 (CSV/XLSX export, §5 #6)
- SPEC-AX-AUDIT-JSONB-001 (JSONB details 부분 검색 + GIN index, §5 #5 / §6 OPEN #4)
- SPEC-AX-AUDIT-STATS-001 (audit 데이터 집계·통계·시각화, §7 out-of-scope)

## §17. 작성자 의도 (annotation)

본 SPEC은 KEPCO E&C 한국 공공 기관 경영평가 PoC의 감사 책임자(audit/compliance officer) 권한을 자연 확장한다. 7 SPEC 누적 audit_logs 적재 데이터의 검색 가능성을 admin only로 노출하여 한국 공공 감사 추적성 정책을 만족한다. consumer-only [HARD] 0-diff·frozen rbac.go 0-diff·신규 외부 의존 0·자체 audit 0이 핵심 불변식. read-only 설계로 mutation 도메인 lesson(D1 iter2 NewRecorder bool 결정)을 acknowledge하되 적용하지 않는다(mutation 0). 후속 SPEC들(audit-of-audit-read / CSV export / JSONB search / stats)이 본 SPEC 위에 incremental 확장 가능한 구조를 유지한다.
