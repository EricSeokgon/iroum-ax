# Acceptance Criteria: SPEC-AX-RUBRIC-001 등급 rubric 확장 시스템 — 3-tier Rubric+Criteria+Bands Store + HTTP API

**SPEC**: SPEC-AX-RUBRIC-001 v0.1.0 (draft)
**Phase**: Plan
**Generated**: 2026-05-20
**Author**: ircp
**Format**: Given-When-Then per AC

> AC ID 형식: `AC-RUBRIC-{REQ}-{N}` (SPEC-AX-SCORE-001 / SCORE-API-001 / REPORT-001 / REVIEW-001 canonical 패턴 정합).
> 모든 AC는 `go test` 자동화 가능. 모든 한국어 에러 메시지는 `score_handlers.go:114-127` / REVIEW-001 정합.

---

## §0 Ubiquitous AC (REQ-RUBRIC-UBI-*)

### AC-RUBRIC-UBI-001 — 데이터 주권 (외부 호출 0)

**Given** the rubric expansion store + HTTP API + apply engine layer is initialized AND any client invokes any endpoint (POST `/api/v1/rubrics` create, GET `/api/v1/rubrics/{id}` get, GET `/api/v1/rubrics` list, PUT `/api/v1/rubrics/{id}` update, POST `/api/v1/rubrics/{id}/archive`, POST `/api/v1/rubrics/{id}/criteria` add criterion, POST `/api/v1/rubrics/{id}/bands` add band, POST `/api/v1/rubrics/{id}/apply` apply engine),
**When** the request is processed end-to-end,
**Then** the entire request path SHALL make 0 outbound calls to any external service (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) — verified by network sandbox + import audit (`go list -deps` showing no `net/http.Client.Do` to external hosts within this package). 모든 영속·검증·등급 산정은 단일 내부 PostgreSQL pgx pool(`PgWorkflowStore.pool` 재사용, `pg_store.go:134` `BeginScoreTx` 동형)에만 위임된다.

### AC-RUBRIC-UBI-002 — 감사 가능성 (mutations 동일-TX entity+audit 원자성, apply read-only 예외)

**Given** an open `pgx.Tx` via `RubricStore.BeginRubricTx` AND any **mutation operation** (InsertRubric / UpdateRubric / ArchiveRubric / AddCriterion / AddBand) is invoked,
**When** the operation succeeds AND the TX is committed,
**Then** exactly one new row SHALL exist in the corresponding entity table (`rubrics` / `rubric_criteria` / `rubric_bands`) AND exactly one new row SHALL exist in `audit_logs` with matching `resource_id` = entity UUID directly (D2 미러 — surrogate 미사용, namespace 상수 0) AND `action ∈ {RUBRIC_CREATED, RUBRIC_UPDATED, RUBRIC_ARCHIVED, RUBRIC_CRITERION_ADDED, RUBRIC_BAND_ADDED}`.
**And** WHEN the TX is rolled back instead, THEN both entity and `audit_logs` SHALL have 0 new rows from this attempt (양방향 원자성).
**And** WHEN the **read-only `ApplyRubric` operation** is invoked, THEN it SHALL NOT write any `audit_logs` row (apply는 query-like, audit 0건 — §6 OPEN #6 Option A 채택 시 확정).
**And** the HTTP handler SHALL NOT INSERT any additional `audit_logs` row (이중 감사 금지 — store-only audit).

### AC-RUBRIC-UBI-003 — cli-anonymous + auth-disabled fallback + userID 명시 전파

**Given** `AUTH_ENABLED=false` (Walking Skeleton 기본값, SCORE-001 §1.1 정합),
**When** any client invokes any endpoint without an Authorization header,
**Then** the ABAC/RBAC middleware SHALL pass through transparently (`abac.go:8` REQ-ABAC-009 정합) AND the resulting `rubrics`/`rubric_criteria`/`rubric_bands` rows SHALL carry `created_by = 'cli-anonymous'` literal AND `updated_by = 'cli-anonymous'` literal (NULL 금지) AND the corresponding `audit_logs.user_id = 'cli-anonymous'` literal.
**And** the handler SHALL respond with HTTP 200/201 (not 401/403).
**And Given** `AUTH_ENABLED=true` AND a valid principal context `auth.UserFromContext` returning user with id `"user-42"`,
**When** any mutation endpoint is invoked,
**Then** the store TX mutation method SHALL receive `userID = "user-42"` parameter (**REVIEW-001 D1 iter2 lesson pre-applied — store-TX 명시적 `userID string` 시그니처**) AND bind it via SQL `$N` placeholder to `created_by` AND `updated_by` AND `audit_logs.user_id` consistently — verified by `SELECT created_by, updated_by FROM rubrics WHERE id=$1` AND `SELECT user_id FROM audit_logs WHERE resource_id=$1` all returning `"user-42"`.
**And** `pg_store.go BeginRubricTx` SHALL call `audit.NewRecorder(true)` (NOT `false`) — verified by unit test (**REVIEW-001 D1 iter2 lesson** — `NewRecorder(false)` would fix user_id to `'cli-anonymous'` ignoring context).

### AC-RUBRIC-UBI-004 — 권한 narrowing + 3-state 불변식 + archived terminal (동시 보장)

**Given** the system enforces ABAC narrowing + state-machine guard simultaneously,
**When** (a) a viewer-only or analyst-only principal attempts any mutation (POST `/api/v1/rubrics` create, PUT `/api/v1/rubrics/{id}` update, POST `/api/v1/rubrics/{id}/archive`, POST `/api/v1/rubrics/{id}/criteria`, POST `/api/v1/rubrics/{id}/bands`), OR (b) any client attempts a non-allowed status transition (e.g., `archived → active`, `active → draft`, `archived → draft` 등), OR (c) any mutation targets a rubric with `status = 'archived'`,
**Then** for (a) the API SHALL respond HTTP 403 with body `{"error":{"code":"ABAC_CONDITION_DENIED","message":"rubric 관리 권한이 없는 사용자입니다"}}` (`abac.go:24` 정합) AND no row SHALL be modified,
**And** for (b) the store SHALL reject before SQL execution by returning `ErrRubricInvalidStatus` mapped to HTTP 409 with body `{"error":{"code":"CONFLICT","message":"허용되지 않은 rubric 상태 전이입니다"}}` AND no row SHALL be modified,
**And** for (c) the store SHALL reject before SQL execution by returning `ErrRubricArchived` mapped to HTTP 409 with body `{"error":{"code":"CONFLICT","message":"보관 처리된 rubric은 수정할 수 없습니다"}}` AND no row SHALL be modified in `rubrics`/`rubric_criteria`/`rubric_bands` or `audit_logs`.
**And** `archived` SHALL be terminal — no transition out is allowed.
**And** no physical DELETE SHALL occur on any rubric table (append-only — 정정은 신규 draft INSERT만).
**And Given** admin role,
**When** admin attempts read or apply,
**Then** SHALL succeed (admin 우회, narrowing-only). Viewer attempts read or apply SHALL also succeed (read+apply는 모든 인증 허용).

---

## §1 REQ-RUBRIC-001 — Store + 3계층 데이터 모델 + 마이그레이션

### AC-RUBRIC-001-1 (REQ-RUBRIC-001-E1, rubric entity+audit 원자 생성 + userID 전파)

**Given** a `RubricTx` obtained via `BeginRubricTx`,
**When** `tx.InsertRubric(ctx, "안전보건 rubric v1", 1, "safety", nil, "user-42")` is invoked AND `tx.Commit(ctx)` succeeds,
**Then** one new row SHALL exist in `rubrics` with `name = '안전보건 rubric v1'`, `version = 1`, `scope = 'safety'`, `status = 'draft'`, `created_by = 'user-42'`, `updated_by = 'user-42'`, `archive_reason IS NULL`,
**And** one new row SHALL exist in `audit_logs` with `action = 'RUBRIC_CREATED'` AND `resource_id` equal to the returned UUID (UUID 직접 — D2 미러, namespace 상수 0) AND `user_id = 'user-42'` (**REVIEW-001 D1 iter2 lesson 검증 — `userID string` 파라미터가 `audit_logs.user_id`로 정확 전파**).

### AC-RUBRIC-001-2 (REQ-RUBRIC-001-E2, criterion 추가 + cross-store EvalItem stub)

**Given** an existing `rubrics` row with id `<rubricID>` AND status `draft` or `active` (NOT archived) AND a valid `evaluation_item_id` `<eiID>` (existence validated by handler via separate `EvalItemStore` TX),
**When** `tx.AddCriterion(ctx, <rubricID>, <eiID>, 0.25, "user-42")` is invoked AND committed,
**Then** one new row SHALL exist in `rubric_criteria` with `rubric_id = <rubricID>` (내부 FK), `evaluation_item_id = <eiID>` (FK-less stub — no DB constraint to `evaluation_items.id`), `weight = 0.25`,
**And** one new row SHALL exist in `audit_logs` with `action = 'RUBRIC_CRITERION_ADDED'` AND `resource_id = rubric_criteria.id` UUID directly AND `user_id = 'user-42'`.
**And Given** an `evaluation_item_id` that does NOT exist in `evaluation_items`, the store SHALL succeed at the store layer (no FK violation) — eval item existence validation is the HTTP handler's responsibility via cross-store TX (REQ-RUBRIC-001-S1).

### AC-RUBRIC-001-3 (REQ-RUBRIC-001-E3, band 추가 entity+audit 원자)

**Given** an existing `rubrics` row with id `<rubricID>` AND status NOT archived,
**When** `tx.AddBand(ctx, <rubricID>, "A", 90.0, 100.0, "user-42")` is invoked AND committed,
**Then** one new row SHALL exist in `rubric_bands` with `rubric_id = <rubricID>`, `letter = 'A'`, `min_score = 90.0`, `max_score = 100.0`,
**And** one new row SHALL exist in `audit_logs` with `action = 'RUBRIC_BAND_ADDED'` AND `resource_id = rubric_bands.id` UUID directly AND `user_id = 'user-42'`.

### AC-RUBRIC-001-4 (REQ-RUBRIC-001-S1, FK-less stub to EvalItem)

**Given** the database schema produced by `0006_rubric_tables.sql`,
**When** the schema is inspected via `\d+ rubric_criteria`,
**Then** the column `evaluation_item_id` SHALL be `UUID NOT NULL` AND SHALL have NO foreign key constraint to `evaluation_items.id` (FK-less stub — SCORE-001 §1.4 / REVIEW-001 §1.4 동형 계약),
**And** the column `rubric_id` SHALL be `UUID NOT NULL REFERENCES rubrics(id)` — 내부 FK 허용(동일 마이그레이션 0006 내).
**And** when `AddCriterion` is called with an `evaluation_item_id` that does NOT exist in `evaluation_items`, the store SHALL succeed (no FK violation) — eval item existence validation is the HTTP handler's responsibility (cross-store 2-TX, REVIEW-001 §6.3 / REPORT-001 §6.3 동형).

### AC-RUBRIC-001-5 (REQ-RUBRIC-001-U1, blank/invalid 입력 거부)

**Given** a `RubricTx`,
**When** any of these invalid inputs is provided:
- `tx.InsertRubric(ctx, "", 1, "", nil, "user-42")` (blank name),
- `tx.InsertRubric(ctx, strings.Repeat("a", 65), 1, "", nil, "user-42")` (name > 64 chars),
- `tx.AddCriterion(ctx, rid, eid, -0.1, "user-42")` (weight < 0),
- `tx.AddCriterion(ctx, rid, eid, 1.5, "user-42")` (weight > 1),
- `tx.AddBand(ctx, rid, "", 0, 100, "user-42")` (blank letter),
- `tx.AddBand(ctx, rid, "A", 100, 100, "user-42")` (min == max),
- `tx.AddBand(ctx, rid, "A", 100, 50, "user-42")` (min > max),

**Then** the store SHALL return the appropriate sentinel (`ErrRubricInvalidInput` for blank/length/letter/range, `ErrRubricWeightOutOfBounds` for weight) wrapped error AND SHALL NOT execute any SQL INSERT (fail-closed, SQL 미실행 후 거부 — SCORE-001 `validateScoreInput` `score.go:79-97` 동형) AND no row SHALL appear in `rubrics`/`rubric_criteria`/`rubric_bands` or `audit_logs`.

### AC-RUBRIC-001-6 (REQ-RUBRIC-001-O1, metadata opaque)

**Given** a `RubricTx` AND a metadata map `{"custom_field":"value", "nested":{"a":1}}`,
**When** `tx.InsertRubric(ctx, "test", 1, "", metadata, "user-42")` is invoked AND committed,
**Then** the stored row's `metadata` JSONB SHALL be semantically identical to the input map (round-trip 검증) AND no schema validation SHALL be applied on the metadata contents (opaque placeholder).
**And** SELECT의 결과를 다시 Go map으로 unmarshal했을 때 입력과 동일.

---

## §2 REQ-RUBRIC-002 — HTTP API rubric CRUD + criteria/bands sub-resource

### AC-RUBRIC-002-1 (REQ-RUBRIC-002-E1, rubric 생성 endpoint admin 성공)

**Given** an `admin` role principal (scope contains `iroum-ax:admin`),
**When** POST `/api/v1/rubrics` is sent with body `{"name":"안전보건 rubric v1","version":1,"scope":"safety"}`,
**Then** the API SHALL respond HTTP 201 Created AND the response body SHALL be `{"id":"<uuid>","name":"안전보건 rubric v1","version":1,"scope":"safety","status":"draft","created_at":"<rfc3339>","created_by":"<principal_id>"}`,
**And** one new row SHALL appear in `rubrics` AND one new row SHALL appear in `audit_logs` (UBI-002 동일-TX),
**And** the handler SHALL NOT invoke `RecordRubric*` directly (store-only audit, UBI-002).

### AC-RUBRIC-002-2 (REQ-RUBRIC-002-E2, rubric 단건 조회 + 자식 임베드)

**Given** a valid `rubrics` row with id `<uuid>` AND any authenticated user (viewer/analyst/admin) OR auth-disabled,
**When** GET `/api/v1/rubrics/<uuid>` is sent,
**Then** the API SHALL respond HTTP 200 OK with the full entity JSON `{"id","name","version","scope","status","archive_reason","created_at","created_by","updated_at","updated_by","metadata","criteria":[...],"bands":[...]}` (criteria/bands 임베드 — §6 OPEN #3 결정 후 확정).
**And** WHEN the `<uuid>` does NOT exist, THEN the API SHALL respond HTTP 404 with body `{"error":{"code":"NOT_FOUND","message":"요청한 rubric을 찾을 수 없습니다"}}`.

### AC-RUBRIC-002-3 (REQ-RUBRIC-002-E3, rubric 목록 + pagination + filter)

**Given** 100 `rubrics` rows with mixed statuses,
**When** GET `/api/v1/rubrics?status=active&scope=safety&limit=20&offset=10` is sent,
**Then** the API SHALL respond HTTP 200 with body `{"items":[...20 items max...], "total":<count>}`,
**And** items SHALL all have `status = 'active'` AND `scope = 'safety'` AND SHALL be ordered by `created_at DESC`,
**And** `limit=0` or missing SHALL clamp to 50, `limit > 500` SHALL clamp to 500, negative `offset` SHALL clamp to 0 (`score_handlers.go:144-157` `clampPagination` / REVIEW-001 동형).
**And** WHEN the filter returns 0 rows, THEN body SHALL be `{"items":[],"total":0}` (HTTP 200, not 404).

### AC-RUBRIC-002-4 (REQ-RUBRIC-002-E4, criterion 추가 sub-resource + cross-store EvalItem 검증)

**Given** an existing `rubrics` row id `<rid>` with status `draft` AND an `admin` role principal AND a valid `evaluation_item_id` `<eiID>` that exists in `evaluation_items`,
**When** POST `/api/v1/rubrics/<rid>/criteria` is sent with body `{"evaluation_item_id":"<eiID>","weight":0.25}`,
**Then** the handler SHALL execute **cross-store handler-compose 2-TX** (REVIEW-001 §6.3 / REPORT-001 §6.3 정확 미러): (TX-1 read-only) `evalItemStore.BeginEvalItemTx` → `GetEvalItemByID(<eiID>)` returning eval item → Rollback (TX-1 returns 404 if absent), (TX-2 write) `rubricStore.BeginRubricTx` → `AddCriterion` → Commit, AND the API SHALL respond HTTP 201 + criterion JSON.
**And Given** `evaluation_item_id` that does NOT exist in `evaluation_items`, the API SHALL respond HTTP 404 with body `{"error":{"code":"NOT_FOUND","message":"참조하는 평가 항목을 찾을 수 없습니다"}}` AND no row SHALL be inserted in `rubric_criteria` or `audit_logs`.

### AC-RUBRIC-002-5 (REQ-RUBRIC-002-E5, band 추가 sub-resource)

**Given** an existing `rubrics` row id `<rid>` with status NOT archived AND an `admin` role principal,
**When** POST `/api/v1/rubrics/<rid>/bands` is sent with body `{"letter":"A","min_score":90.0,"max_score":100.0}`,
**Then** the handler SHALL (a) verify `rubrics.id` exists and is NOT archived, (b) validate `min_score < max_score`, (c) check overlap with existing bands (§6 OPEN #4 결정 후 — Option B handler-layer load+check or Option A EXCLUSION USING gist DB-level), (d) invoke `RubricTx.AddBand` with `userID = principal.id`, (e) commit, AND respond HTTP 201 + band JSON `{"id","rubric_id","letter","min_score","max_score","created_at"}`.

### AC-RUBRIC-002-6 (REQ-RUBRIC-002-U1, malformed UUID)

**Given** any authenticated user,
**When** GET `/api/v1/rubrics/not-a-uuid` is sent,
**Then** the API SHALL respond HTTP 400 with body `{"error":{"code":"INVALID_ARGUMENT","message":"유효하지 않은 rubric ID 형식입니다","field":"id"}}` AND SHALL NOT invoke the store layer (fail-closed).

---

## §3 REQ-RUBRIC-003 — HTTP API 상태 전이 + archived terminal

### AC-RUBRIC-003-1 (REQ-RUBRIC-003-E1, rubric 수정 draft → active 전이)

**Given** an existing `rubrics` row with `status = 'draft'` AND an `admin` role principal,
**When** PUT `/api/v1/rubrics/<id>` is sent with body `{"status":"active"}` (§6 OPEN #1 결정 후 endpoint shape 확정),
**Then** the store SHALL acquire `SELECT ... FOR UPDATE` row lock AND verify current status NOT archived AND verify state transition (draft → active allowed) AND UPDATE `status = 'active'`, `updated_at = now()`, `updated_by = principal.id` AND write one `audit_logs` row with `action = 'RUBRIC_UPDATED'` within the same TX,
**And** the API SHALL respond HTTP 200 OK with the updated entity.

### AC-RUBRIC-003-2 (REQ-RUBRIC-003-E2, archive — active → archived terminal)

**Given** an existing `rubrics` row with `status = 'active'` AND an `admin` role principal,
**When** POST `/api/v1/rubrics/<id>/archive` is sent with body `{"archive_reason":"신규 평가편람 채택"}` (§6 OPEN #7 결정 후 — Option A 필수 / Option B optional),
**Then** the store SHALL acquire row lock AND verify current status = `active` AND verify `archive_reason` non-empty (if §6 OPEN #7 Option A) AND UPDATE `status = 'archived'`, `archive_reason = '신규 평가편람 채택'`, `updated_at/updated_by` AND write `audit_logs` row with `action = 'RUBRIC_ARCHIVED'` within the same TX,
**And** the API SHALL respond HTTP 200 OK with updated entity,
**And** any subsequent mutation attempt on this row (update/criteria add/bands add) SHALL return `ErrRubricArchived` → HTTP 409 (terminal — UBI-004).

### AC-RUBRIC-003-3 (REQ-RUBRIC-003-S1, archived rubric mutation 거부)

**Given** an existing `rubrics` row with `status = 'archived'`,
**When** any mutation is attempted: (a) PUT `/api/v1/rubrics/<id>` update, (b) POST `/api/v1/rubrics/<id>/criteria` add criterion, (c) POST `/api/v1/rubrics/<id>/bands` add band,
**Then** the store SHALL reject before SQL execution by returning `ErrRubricArchived` (HTTP 409) AND no row SHALL be modified in `rubrics`/`rubric_criteria`/`rubric_bands` or `audit_logs` (fail-closed),
**And** the response body SHALL contain `{"error":{"code":"CONFLICT","message":"보관 처리된 rubric은 수정할 수 없습니다"}}`.
**And** read (GET) AND apply (POST .../apply) SHALL still succeed on archived rubric (read-only operations are allowed).

### AC-RUBRIC-003-4 (REQ-RUBRIC-003-S2, 불법 전이 모두 거부)

**Given** rows in various states (`draft`, `active`, `archived`),
**When** any of these illegal transitions is attempted: (a) `archived → active`, (b) `archived → draft`, (c) `active → draft` 되돌리기, (d) `draft → archived` 직접 (skipping active),
**Then** the store-level `validateRubricStatusTransition` SHALL reject before SQL execution by returning `ErrRubricInvalidStatus` (HTTP 409) AND no row SHALL be modified in `rubrics` or `audit_logs` (fail-closed),
**And** the response body SHALL contain `{"error":{"code":"CONFLICT","message":"허용되지 않은 rubric 상태 전이입니다"}}`.

---

## §4 REQ-RUBRIC-004 — Apply 엔진 (read-only 등급 산정)

### AC-RUBRIC-004-1 (REQ-RUBRIC-004-E1, store-layer ApplyRubric read-only no-audit)

**Given** an existing `rubrics` row id `<rid>` with bands `[{A: 90.0..100.0}, {B: 80.0..89.999}, {C: 70.0..79.999}]`,
**When** `tx.ApplyRubric(ctx, <rid>, 85.5)` is invoked,
**Then** the store SHALL load `rubric_bands` for `<rid>` ordered by `min_score ASC` AND find band where `min_score <= 85.5 <= max_score` AND return `(letter = "B", band = {letter:"B", min_score:80.0, max_score:89.999}, nil)`,
**And** the store SHALL NOT write any `audit_logs` row (read-only — UBI-002 second clause, §6 OPEN #6 Option A 채택 시 확정).

### AC-RUBRIC-004-2 (REQ-RUBRIC-004-E2, HTTP apply cross-store handler-compose 2-TX)

**Given** an existing `scores` row id `<sid>` AND an existing `rubrics` row id `<rid>` with bands AND any authenticated user (viewer/analyst/admin) OR auth-disabled,
**When** POST `/api/v1/rubrics/<rid>/apply?score_id=<sid>` is sent,
**Then** the handler SHALL execute **cross-store handler-compose 2-TX** (REPORT-001 §6.3 / REVIEW-001 §6.3 정확 미러): (TX-1 read-only) `scoreStore.BeginScoreTx(ctx)` → `SumWeightedByEvaluationItem(<sid>)` returning `pgtype.Numeric` SEC-03 → convert to float64 → `tx.Rollback(ctx)`, (TX-2 read-only) `rubricStore.BeginRubricTx(ctx)` → `ApplyRubric(<rid>, scoreValue)` → `tx.Rollback(ctx)`,
**And** the API SHALL respond HTTP 200 OK with body `{"rubric_id":"<rid>","score_value":85.5,"letter":"B","band":{"min_score":80.0,"max_score":89.999}}`.
**And** no row SHALL be modified in any table (read-only end-to-end).
**And** `audit_logs` row count SHALL not increase (no audit for apply — §6 OPEN #6 Option A).

### AC-RUBRIC-004-3 (REQ-RUBRIC-004-U1, score out of all bands fail-closed)

**Given** an existing `rubrics` row with bands all within range [70.0, 100.0],
**When** `tx.ApplyRubric(ctx, <rid>, 200.0)` is invoked (or POST .../apply with `score_value=200.0`),
**Then** the store SHALL return `("", RubricBand{}, ErrRubricInvalidInput)` — fail-closed, no default fallback,
**And** the API SHALL respond HTTP 400 with body `{"error":{"code":"INVALID_ARGUMENT","message":"점수가 rubric 등급 구간을 벗어났습니다","field":"score_value"}}` (한국 공공 결정성 정합).

---

## §5 REQ-RUBRIC-005 — 에러 매핑 + 표준 응답

### AC-RUBRIC-005-1 (REQ-RUBRIC-005-E1, 결정적 store→HTTP 매핑)

**Given** the handler invokes any store method,
**When** the store returns each sentinel error,
**Then** `mapRubricStoreErr` SHALL produce the following deterministic mapping:

| Store Sentinel | HTTP Status | Error Code | Korean Message |
|----------------|-------------|------------|----------------|
| `ErrRubricNotFound` | 404 | `NOT_FOUND` | 요청한 rubric을 찾을 수 없습니다 |
| `ErrRubricInvalidInput` | 400 | `INVALID_ARGUMENT` | rubric 입력이 유효하지 않습니다 |
| `ErrRubricInvalidStatus` | 409 | `CONFLICT` | 허용되지 않은 rubric 상태 전이입니다 |
| `ErrRubricArchived` | 409 | `CONFLICT` | 보관 처리된 rubric은 수정할 수 없습니다 |
| `ErrRubricWeightOutOfBounds` | 400 | `INVALID_ARGUMENT` | 가중치는 0과 1 사이여야 합니다 |
| `ErrRubricBandOverlap` | 400 | `INVALID_ARGUMENT` | 등급 구간이 기존 구간과 겹칩니다 |
| `ErrRubricAuditWriteFailed` | 500 | `INTERNAL` | rubric 처리 중 오류가 발생했습니다 (+ ERROR log) |
| unknown error | 500 | `INTERNAL` | rubric 처리 중 오류가 발생했습니다 (+ ERROR log) |

**And** client errors (400/403/404/409) SHALL be logged at INFO level (`score_handlers.go:96-100` `writeScoreErr` / REVIEW-001 동형) AND server faults (500) SHALL be logged at ERROR level.

### AC-RUBRIC-005-2 (REQ-RUBRIC-005-U1, raw pgx 에러 누출 금지)

**Given** the store layer attempts `GetRubricByID` with a non-existent UUID,
**When** the underlying pgx query returns `pgx.ErrNoRows`,
**Then** the store SHALL wrap and return `ErrRubricNotFound` (the proper sentinel) — never returning raw `pgx.ErrNoRows` (GAP-03 패턴 정합, EVID-001/SCORE-001/REVIEW-001 동형),
**And** the test SHALL verify `errors.Is(err, apperrors.ErrRubricNotFound) == true` AND `errors.Is(err, pgx.ErrNoRows) == false` (raw 누출 0).

---

## §6 (별도 번호 — Edge Cases) 16 Edge Cases

| # | 시나리오 | 기대 결과 |
|---|---------|----------|
| E1 | rubric `<id>` 미존재로 GET `/rubrics/<id>` | 404 `ErrRubricNotFound`, 한국어 메시지 |
| E2 | `weight = -0.1` 또는 `weight = 1.5`로 AddCriterion | 400 `ErrRubricWeightOutOfBounds`, store SQL 미실행 |
| E3 | band overlap (기존 [80..89] 존재 시 신규 [85..95] 추가 시도) | 400 `ErrRubricBandOverlap` (§6 OPEN #4 Option B 채택 시 handler-layer, Option A 채택 시 DB EXCLUSION 위반 → 500 매핑은 §6 OPEN #4 결정 후 확정) |
| E4 | archived rubric에 POST `/criteria` 또는 `/bands` 또는 PUT 시도 | 409 `ErrRubricArchived`, 0 row 변경 |
| E5 | blank name(`""`) 또는 name > 64 chars로 POST `/rubrics` | 400 `ErrRubricInvalidInput`, store SQL 미실행 |
| E6 | viewer가 POST `/rubrics` (create) 시도 | 403 `ABAC_CONDITION_DENIED`, 0 row 변경 |
| E7 | analyst가 POST `/rubrics/{id}/archive` 시도 | 403 `ABAC_CONDITION_DENIED`, 0 row 변경 (admin-only) |
| E8 | `AUTH_ENABLED=false`로 모든 엔드포인트 호출 | 200/201 응답, `created_by='cli-anonymous'`, `audit_logs.user_id='cli-anonymous'`, **`NewRecorder(true)` 호출 검증** |
| E9 | `AUTH_ENABLED=true`로 admin principal id=`user-42`로 POST `/rubrics` | 201 응답, `created_by='user-42'`, `audit_logs.user_id='user-42'` — **REVIEW-001 D1 iter2 lesson 검증 (userID string 파라미터 + NewRecorder(true) 전파)** |
| E10 | apply score=200 (모든 band [70..100] 밖) | 400 `ErrRubricInvalidInput` fail-closed, 한국어 "점수가 rubric 등급 구간을 벗어났습니다" |
| E11 | apply 시 rubric_id 미존재 | 404 cross-store 단계 |
| E12 | AddCriterion 시 evaluation_item_id 미존재 (cross-store TX-1) | 404 `NOT_FOUND` "참조하는 평가 항목을 찾을 수 없습니다" |
| E13 | min_score >= max_score로 AddBand (`90, 90` 또는 `100, 50`) | 400 `ErrRubricInvalidInput` fail-closed |
| E14 | `RecordRubric*` audit INSERT 실패 (fault inject) | `ErrRubricAuditWriteFailed` + 호출자 Rollback → entity+audit 0 row (양방향 취소) |
| E15 | 불법 전이 (`archived → active`, `active → draft`, `draft → archived` 직접) | 409 `ErrRubricInvalidStatus`, 0 row 변경 |
| E16 | GET `/rubrics/not-a-uuid` malformed UUID | 400 `INVALID_ARGUMENT`, store 미호출 |

---

## §7 품질 게이트 기준 (Quality Gate Criteria)

### 7.1 TRUST 5 PASS

- **Tested**: 테스트 커버리지 ≥85% (store-layer + handler 합산), 25 AC + 16 edge case 모두 자동화. 특히 D1 iter2 lesson 검증 테스트(AC-RUBRIC-UBI-003 / AC-RUBRIC-001-1 userID 전파 / E9 NewRecorder(true)) 필수 포함.
- **Readable**: 한국어 godoc/에러 메시지, 한국어 @MX 주석, exported 함수 모두 godoc
- **Unified**: `gofmt`/`goimports`/`golangci-lint` zero issues
- **Secured**: OWASP 준수, raw pgx 에러 누출 0, ABAC narrowing fail-closed, 동일-TX audit, archived terminal 불변
- **Trackable**: SCORE-001/SCORE-API-001/REPORT-001/REVIEW-001 패턴 인용 commit, drift-guard 검증 commit, D1 iter2 lesson pre-applied 명시 commit

### 7.2 evaluator-active 4-차원 점수

- Functionality: ≥0.85 (25 AC + 16 edge 통과, apply 엔진 end-to-end)
- Security: ≥0.85 (ABAC narrowing, fail-closed, 양방향 rollback, archived terminal)
- Craft: ≥0.85 (SCORE-001/SCORE-API-001/REPORT-001/REVIEW-001 패턴 정합 + D1 iter2 lesson pre-applied)
- Consistency: ≥0.85 (한국어 일관성, frozen RBAC 0-diff, drift 0)
- **종합**: ≥0.85 (thorough harness 기준)

### 7.3 Drift-Guard 0%

- 모든 [EXISTING] 파일 `git diff` 0 (R-CONSUMER-001):
  - SCORE-001 `internal/store/score.go` (특히 `SumWeightedByEvaluationItem`/`DetermineGrade` 시그니처 무수정)
  - REVIEW-001 `internal/store/score_review_request.go` (호출 0건, 패턴 미러 참조만)
  - **`.moai/db/schema/migrations/0004_score_tables.sql` `grade_thresholds`** (SCORE-001 0-diff 병행 존재)
  - **`internal/auth/**/*.go`** (frozen RBAC/ABAC, RoleReviewer/evaluator 신설 0)
- [MODIFY] 파일 추가 범위만(기존 라인 무변경)
- 위반 시 즉시 중단·재계획

---

## §8 Definition of Done (DoD)

- [ ] research.md SSOT (file:line 근거) 검증 완료
- [ ] spec.md / plan.md / acceptance.md / spec-compact.md 4 파일 작성 완료
- [ ] §6 OPEN 7건 모두 strategy.md에서 RESOLVED + Human Gate sign-off
- [ ] M0 RED: 22+ failing test 작성 (특히 D1 iter2 lesson 검증 테스트 포함), `go vet ./...` PASS
- [ ] M1 GREEN: 0006 마이그레이션 + store-layer 구현 (`BeginRubricTx` `NewRecorder(true)` 검증, mutation 메서드 `userID string` 파라미터 검증), store-layer 테스트 GREEN
- [ ] M2 GREEN: HTTP handler + ABAC + apply 엔진 (cross-store 2-TX) 구현, handler 테스트 GREEN (모든 인증 read+apply, admin-only mutations)
- [ ] M3 GREEN: server.go 마운트 ≈7줄(REPORT-001 lesson 정확), 통합 빌드 PASS
- [ ] M4 REFACTOR: TRUST 5 PASS, @MX 태그 추가, 커버리지 ≥85%
- [ ] M5 Drift-Guard: 모든 [EXISTING] 파일 0 diff 검증 (특히 `grade_thresholds` 0-diff 병행 존재)
- [ ] evaluator-active 4-차원 점수 ≥0.85
- [ ] manager-quality TRUST 5 PASS
- [ ] consumer-only [HARD] 무위반: SCORE-001/SCORE-API-001/EVAL-ITEM-001/EVID-001/REPORT-001/REVIEW-001/AUTH-003 무수정
- [ ] frozen RBAC 0-diff: `RoleAdmin`/`RoleAnalyst`/`RoleViewer` 3역할만, 신규 역할 추가 0 (**SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시 확인**)
- [ ] phantom 0: 모든 신규 메서드/구조체 spec.md §2.1에 명시, 모든 호출은 source-verified 라인 인용
- [ ] **errors.go drift manifest 정확(SCORE-API-001 교훈)** — §2.1/§2.3 양쪽 명시 부착 검증
- [ ] **server.go 마운트 ≈7줄 정확(REPORT-001 교훈)** — "1줄" 잘못 기술 0 검증
- [ ] **D1 iter2 lesson pre-applied 검증(REVIEW-001 교훈)** — `BeginRubricTx`가 `NewRecorder(true)` 호출, store mutation 메서드가 `userID string` 파라미터를 처음부터 가짐 (iter1 fix 사이클 비재발)
- [ ] **grade_thresholds 병행 존재 검증** — SCORE-001 0004 0-diff 유지, `Score.DetermineGrade` 호출 0건

---

## 참조

- spec.md §3 EARS 요구사항 (UBI 4 + modal 21)
- spec.md §6 OPEN 7건 (권장 옵션 부착)
- plan.md §2 마일스톤 M0-M5
- plan.md §4 TDD 사이클 (22+ test 목록 — D1 iter2 lesson 검증 테스트 명시)
- plan.md §5 R-RUBRIC-001..005 (D1 iter2 재현 방지 등)
- research.md (file:line 근거, SSOT)
- SPEC-AX-SCORE-001 acceptance.md (canonical AC 패턴 선례 + grade_thresholds 단일 테이블 모델)
- SPEC-AX-SCORE-API-001 acceptance.md (HTTP + ABAC AC 선례, errors.go drift lesson, OPEN #4 lesson)
- SPEC-AX-REPORT-001 acceptance.md (cross-store 2-TX + drift-guard 0 선례)
- SPEC-AX-REVIEW-001 acceptance.md (가장 최근 store+API+ABAC + D1 iter2 lesson 정확 미러 대상)
