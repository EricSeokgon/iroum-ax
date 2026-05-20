# Acceptance Criteria: SPEC-AX-REVIEW-001 평가 제출/승인 워크플로우 저장소 + HTTP API 계층

**SPEC**: SPEC-AX-REVIEW-001 v0.1.0 (draft)
**Phase**: Plan
**Generated**: 2026-05-20
**Author**: ircp
**Format**: Given-When-Then per AC

> AC ID 형식: `AC-REVIEW-{REQ}-{N}` (SPEC-AX-SCORE-001 / SCORE-API-001 / REPORT-001 canonical 패턴 정합).
> 모든 AC는 `go test` 자동화 가능. 모든 한국어 에러 메시지는 `score_handlers.go:114-127` 정합.

---

## §0 Ubiquitous AC (REQ-REVIEW-UBI-*)

### AC-REVIEW-UBI-001 — 데이터 주권 (외부 호출 0)

**Given** the score review request store + HTTP API layer is initialized AND any client invokes any endpoint (POST `/api/v1/reviews` create, GET `/api/v1/reviews/{id}` get, GET `/api/v1/reviews` list, POST `/api/v1/reviews/{id}/assign-reviewer`, POST `/api/v1/reviews/{id}/approve`, POST `/api/v1/reviews/{id}/reject`),
**When** the request is processed end-to-end,
**Then** the entire request path SHALL make 0 outbound calls to any external service (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) — verified by network sandbox + import audit (`go list -deps` showing no `net/http.Client.Do` to external hosts within this package). 모든 영속·검증은 단일 내부 PostgreSQL pgx pool(`PgWorkflowStore.pool` 재사용, `pg_store.go:134-148` `BeginScoreTx` 동형)에만 위임된다.

### AC-REVIEW-UBI-002 — 감사 가능성 (동일-TX entity+audit 원자성)

**Given** an open `pgx.Tx` via `ScoreReviewRequestStore.BeginScoreReviewRequestTx` AND any mutation operation (Insert/AssignReviewer/Approve/Reject) is invoked,
**When** the operation succeeds AND the TX is committed,
**Then** exactly one new row SHALL exist in `score_review_requests` AND exactly one new row SHALL exist in `audit_logs` with matching `resource_id = score_review_requests.id` UUID directly (D2 미러 — surrogate 미사용, namespace 상수 0) AND `action ∈ {SCORE_REVIEW_REQUEST_CREATED, SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED, SCORE_REVIEW_REQUEST_APPROVED, SCORE_REVIEW_REQUEST_REJECTED}`.
**And** WHEN the TX is rolled back instead, THEN both `score_review_requests` and `audit_logs` SHALL have 0 new rows from this attempt (양방향 원자성).
**And** the HTTP handler SHALL NOT INSERT any additional `audit_logs` row (이중 감사 금지 — store-only audit).

### AC-REVIEW-UBI-003 — cli-anonymous + auth-disabled fallback

**Given** `AUTH_ENABLED=false` (Walking Skeleton 기본값, SCORE-001 §1.1 정합),
**When** any client invokes any endpoint without an Authorization header,
**Then** the ABAC/RBAC middleware SHALL pass through transparently (`abac.go:8` REQ-ABAC-009 정합) AND the resulting `score_review_requests` row SHALL carry `created_by = 'cli-anonymous'` literal AND `updated_by = 'cli-anonymous'` literal (NULL 금지) AND the corresponding `audit_logs.user_id = 'cli-anonymous'` literal (`pg_store.go:146` `audit.NewRecorder(false)` 패턴 동형).
**And** the handler SHALL respond with HTTP 200/201 (not 401/403).

### AC-REVIEW-UBI-004 — 권한 narrowing + 4-상태 불변식 (동시 보장)

**Given** the system enforces ABAC narrowing + state-machine guard simultaneously,
**When** (a) a viewer-only principal attempts POST `/api/v1/reviews` (create), OR (b) an analyst-only principal attempts POST `/api/v1/reviews/{id}/approve`, OR (c) any client attempts a non-allowed status transition (e.g., `APPROVED → SUBMITTED`, `REJECTED → UNDER_REVIEW`, `SUBMITTED → APPROVED` 직접),
**Then** for (a)(b) the API SHALL respond HTTP 403 with body `{"error":{"code":"ABAC_CONDITION_DENIED","message":"...권한이 없는 사용자입니다"}}` (`abac.go:24` 정합) AND no row SHALL be modified,
**And** for (c) the store SHALL reject before SQL execution by returning `ErrScoreReviewRequestInvalidStatus` mapped to HTTP 409 with body `{"error":{"code":"CONFLICT","message":"허용되지 않은 평가 검토 상태 전이입니다"}}` AND no row SHALL be modified in `score_review_requests` or `audit_logs`.
**And** `APPROVED`/`REJECTED` SHALL be terminal — no transition out is allowed.
**And** no physical DELETE SHALL occur on `score_review_requests` (append-only — 정정은 신규 SUBMITTED row INSERT만).

---

## §1 REQ-REVIEW-001 — Store + 데이터 모델 + 마이그레이션

### AC-REVIEW-001-1 (REQ-REVIEW-001-E1, entity+audit 원자 생성)

**Given** a `ScoreReviewRequestTx` obtained via `BeginScoreReviewRequestTx` AND a valid score UUID `scoreID` that exists in `scores` table,
**When** `tx.InsertScoreReviewRequest(ctx, scoreID, "검토 요청합니다", nil)` is invoked AND `tx.Commit(ctx)` succeeds,
**Then** one new row SHALL exist in `score_review_requests` with `score_id = scoreID`, `status = 'SUBMITTED'`, `created_by = 'cli-anonymous'` (auth-disabled) or principal ID (auth-enabled), `assigned_reviewer_id IS NULL`, `rejection_reason IS NULL`,
**And** one new row SHALL exist in `audit_logs` with `action = 'SCORE_REVIEW_REQUEST_CREATED'` AND `resource_id` equal to the returned UUID (UUID 직접 — D2 미러, namespace 상수 0).

### AC-REVIEW-001-2 (REQ-REVIEW-001-S1, score_id FK-less stub)

**Given** the database schema produced by `0005_score_review_request_tables.sql`,
**When** the schema is inspected via `\d+ score_review_requests`,
**Then** the column `score_id` SHALL be `UUID NOT NULL` AND SHALL have NO foreign key constraint to `scores.id` (FK-less stub — SCORE-001 §1.4 동형 계약),
**And** when `InsertScoreReviewRequest` is called with a `scoreID` that does NOT exist in `scores`, the store SHALL succeed (no FK violation) — score existence validation is the HTTP handler's responsibility (REQ-REVIEW-001-S1 / §6.3 RESOLVED — handler-compose 2-TX, strategy.md §A.3).

### AC-REVIEW-001-3 (REQ-REVIEW-001-U1, blank/invalid 입력 거부)

**Given** a `ScoreReviewRequestTx`,
**When** `tx.InsertScoreReviewRequest(ctx, uuid.Nil, "", nil)` is invoked (zero UUID),
**Then** the store SHALL return `ErrScoreReviewRequestInvalidInput` wrapped error AND SHALL NOT execute any SQL INSERT (fail-closed, SQL 미실행 후 거부 — SCORE-001 `validateScoreInput` `score.go:79-97` 동형) AND no row SHALL appear in `score_review_requests` or `audit_logs`.

### AC-REVIEW-001-4 (REQ-REVIEW-001-O1, metadata opaque)

**Given** a `ScoreReviewRequestTx` AND a metadata map `{"custom_field":"value", "nested":{"a":1}}`,
**When** `tx.InsertScoreReviewRequest(ctx, scoreID, "", metadata)` is invoked AND committed,
**Then** the stored row's `metadata` JSONB SHALL be semantically identical to the input map (round-trip 검증) AND no schema validation SHALL be applied on the metadata contents (opaque placeholder).
**And** SELECT의 결과를 다시 Go map으로 unmarshal했을 때 입력과 동일.

---

## §2 REQ-REVIEW-002 — HTTP API 생성/조회/목록

### AC-REVIEW-002-1 (REQ-REVIEW-002-E1, 생성 endpoint analyst 성공)

**Given** an `analyst` role principal (scope contains `iroum-ax:analyst`) AND a score `scoreID` that exists in `scores`,
**When** POST `/api/v1/reviews` is sent with body `{"score_id":"<scoreID>","comment":"검토 요청합니다"}`,
**Then** the API SHALL respond HTTP 201 Created AND the response body SHALL be `{"id":"<uuid>","score_id":"<scoreID>","status":"SUBMITTED","created_at":"<rfc3339>","created_by":"<principal_id>"}`,
**And** one new row SHALL appear in `score_review_requests` AND one new row SHALL appear in `audit_logs` (UBI-002 동일-TX),
**And** the handler SHALL NOT invoke `RecordScoreReviewRequest*` directly (store-only audit, UBI-002).

### AC-REVIEW-002-2 (REQ-REVIEW-002-E2, 단건 조회)

**Given** a valid `score_review_requests` row with id `<uuid>` AND any authenticated user (viewer/analyst/admin) OR auth-disabled,
**When** GET `/api/v1/reviews/<uuid>` is sent,
**Then** the API SHALL respond HTTP 200 OK with the full entity JSON `{"id","score_id","status","assigned_reviewer_id","rejection_reason","comment","created_at","created_by","updated_at","updated_by","metadata"}`.
**And** WHEN the `<uuid>` does NOT exist, THEN the API SHALL respond HTTP 404 with body `{"error":{"code":"NOT_FOUND","message":"요청한 평가 검토를 찾을 수 없습니다"}}`.

### AC-REVIEW-002-3 (REQ-REVIEW-002-E3, 목록 + pagination + filter)

**Given** 100 `score_review_requests` rows with mixed statuses,
**When** GET `/api/v1/reviews?status=SUBMITTED&limit=20&offset=10` is sent,
**Then** the API SHALL respond HTTP 200 with body `{"items":[...20 items max...], "total":<count of SUBMITTED rows>}`,
**And** items SHALL all have `status = 'SUBMITTED'` AND SHALL be ordered by `created_at DESC`,
**And** `limit=0` or missing SHALL clamp to 50, `limit > 500` SHALL clamp to 500, negative `offset` SHALL clamp to 0 (`score_handlers.go:144-157` `clampPagination` 동형).
**And** WHEN the filter returns 0 rows, THEN body SHALL be `{"items":[],"total":0}` (HTTP 200, not 404).

### AC-REVIEW-002-4 (REQ-REVIEW-002-U1, malformed UUID)

**Given** any authenticated user,
**When** GET `/api/v1/reviews/not-a-uuid` is sent,
**Then** the API SHALL respond HTTP 400 with body `{"error":{"code":"INVALID_ARGUMENT","message":"유효하지 않은 평가 검토 ID 형식입니다","field":"id"}}` AND SHALL NOT invoke the store layer (fail-closed).

---

## §3 REQ-REVIEW-003 — HTTP API 상태 전이 (assign/approve/reject)

### AC-REVIEW-003-1 (REQ-REVIEW-003-E1, 검토자 할당 SUBMITTED → UNDER_REVIEW)

**Given** a `score_review_requests` row with `status = 'SUBMITTED'` AND id `<uuid>` AND an `admin` role principal,
**When** POST `/api/v1/reviews/<uuid>/assign-reviewer` is sent with body `{"reviewer_id":"user-123"}`,
**Then** the store SHALL acquire `SELECT ... FOR UPDATE` row lock (§6.6 RESOLVED — pessimistic + state-machine 가드, strategy.md §A.6) AND verify `status = 'SUBMITTED'` AND UPDATE `status = 'UNDER_REVIEW'`, `assigned_reviewer_id = 'user-123'`, `updated_at = now()`, `updated_by = principal.id` AND write one `audit_logs` row with `action = 'SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED'` within the same TX,
**And** the API SHALL respond HTTP 200 OK with the updated entity.

### AC-REVIEW-003-2 (REQ-REVIEW-003-E2, 승인 UNDER_REVIEW → APPROVED terminal)

**Given** a `score_review_requests` row with `status = 'UNDER_REVIEW'` AND id `<uuid>` AND an `admin` role principal,
**When** POST `/api/v1/reviews/<uuid>/approve` is sent with optional body `{"comment":"승인합니다"}`,
**Then** the store SHALL acquire row lock AND verify `status = 'UNDER_REVIEW'` AND UPDATE `status = 'APPROVED'`, `comment = '승인합니다'` (if provided), `updated_at/updated_by` AND write `audit_logs` row with `action = 'SCORE_REVIEW_REQUEST_APPROVED'` within same TX,
**And** the API SHALL respond HTTP 200 OK with updated entity,
**And** any subsequent transition attempt from this row SHALL return `ErrScoreReviewRequestInvalidStatus` → HTTP 409 (terminal — UBI-004).

### AC-REVIEW-003-3 (REQ-REVIEW-003-E3, 반려 UNDER_REVIEW → REJECTED with reason)

**Given** a `score_review_requests` row with `status = 'UNDER_REVIEW'` AND id `<uuid>` AND an `admin` role principal,
**When** POST `/api/v1/reviews/<uuid>/reject` is sent with body `{"rejection_reason":"근거 부족"}`,
**Then** the store SHALL acquire row lock AND verify `status = 'UNDER_REVIEW'` AND verify `rejection_reason` non-empty AND UPDATE `status = 'REJECTED'`, `rejection_reason = '근거 부족'`, `updated_at/updated_by` AND write `audit_logs` row with `action = 'SCORE_REVIEW_REQUEST_REJECTED'` within same TX,
**And** the API SHALL respond HTTP 200 OK with updated entity,
**And** any subsequent transition attempt SHALL return HTTP 409 (terminal — UBI-004).

### AC-REVIEW-003-4 (REQ-REVIEW-003-S1, 불법 전이 모두 거부)

**Given** rows in various states (`APPROVED`, `REJECTED`, `SUBMITTED`),
**When** any of these illegal transitions is attempted: (a) `APPROVED → SUBMITTED`, (b) `REJECTED → UNDER_REVIEW`, (c) `SUBMITTED → APPROVED` 직접 (skipping UNDER_REVIEW), (d) `UNDER_REVIEW → SUBMITTED` 되돌리기,
**Then** the store-level `validateReviewStatusTransition` SHALL reject before SQL execution by returning `ErrScoreReviewRequestInvalidStatus` (HTTP 409) AND no row SHALL be modified in `score_review_requests` or `audit_logs` (fail-closed),
**And** the response body SHALL contain `{"error":{"code":"CONFLICT","message":"허용되지 않은 평가 검토 상태 전이입니다"}}`.

---

## §4 REQ-REVIEW-004 — Store-Level 감사 연계

### AC-REVIEW-004-1 (REQ-REVIEW-004-E1, RecordScoreReviewRequest* 동일 TX)

**Given** any mutation method (`InsertScoreReviewRequest` / `AssignReviewer` / `ApproveRequest` / `RejectRequest`) is invoked within an open `pgx.Tx`,
**When** the method body executes,
**Then** it SHALL invoke the corresponding `recorder.RecordScoreReviewRequest{Created|ReviewerAssigned|Approved|Rejected}` exactly once AND the recorder SHALL construct an `audit.Event` with the appropriate `Action` constant AND `ResourceID = score_review_requests.id` UUID directly (D2 — namespace 상수 0) AND write to `audit_logs` via the SAME `tx` (not a new one) — local AuditTx 패턴, SCORE-001 `RecordScore*` 동형.

### AC-REVIEW-004-2 (REQ-REVIEW-004-U1, audit write 실패 양방향 rollback)

**Given** an `InsertScoreReviewRequest` call within a TX where the `audit_logs` INSERT will fail (simulated via fault injection),
**When** the audit INSERT fails,
**Then** the store method SHALL return `ErrScoreReviewRequestAuditWriteFailed` wrapping the underlying error AND the test SHALL invoke `tx.Rollback(ctx)`,
**Then** AFTER rollback: 0 new rows in `score_review_requests` AND 0 new rows in `audit_logs` (양방향 원자성, SCORE-001 `ErrScoreAuditWriteFailed` `errors.go:71-74` 동형, DC-004-U1 미러).

---

## §5 REQ-REVIEW-005 — 에러 매핑 + 표준 응답

### AC-REVIEW-005-1 (REQ-REVIEW-005-E1, 결정적 store→HTTP 매핑)

**Given** the handler invokes any store method,
**When** the store returns each sentinel error,
**Then** `mapReviewStoreErr` SHALL produce the following deterministic mapping:

| Store Sentinel | HTTP Status | Error Code | Korean Message |
|----------------|-------------|------------|----------------|
| `ErrScoreReviewRequestNotFound` | 404 | `NOT_FOUND` | 요청한 평가 검토를 찾을 수 없습니다 |
| `ErrScoreReviewRequestInvalidInput` | 400 | `INVALID_ARGUMENT` | 평가 검토 입력이 유효하지 않습니다 |
| `ErrScoreReviewRequestInvalidStatus` | 409 | `CONFLICT` | 허용되지 않은 평가 검토 상태 전이입니다 |
| `ErrScoreReviewRequestNotSubmitted` | 409 | `CONFLICT` | 검토자 할당은 SUBMITTED 상태의 평가 검토에만 허용됩니다 |
| `ErrScoreReviewRequestNotUnderReview` | 409 | `CONFLICT` | 승인/반려는 UNDER_REVIEW 상태의 평가 검토에만 허용됩니다 |
| `ErrScoreReviewRequestAuditWriteFailed` | 500 | `INTERNAL` | 평가 검토 처리 중 오류가 발생했습니다 (+ ERROR log) |
| unknown error | 500 | `INTERNAL` | 평가 검토 처리 중 오류가 발생했습니다 (+ ERROR log) |

**And** client errors (400/403/404/409) SHALL be logged at INFO level (`score_handlers.go:96-100` `writeScoreErr` 동형) AND server faults (500) SHALL be logged at ERROR level.

### AC-REVIEW-005-2 (REQ-REVIEW-005-U1, raw pgx 에러 누출 금지)

**Given** the store layer attempts `GetScoreReviewRequestByID` with a non-existent UUID,
**When** the underlying pgx query returns `pgx.ErrNoRows`,
**Then** the store SHALL wrap and return `ErrScoreReviewRequestNotFound` (the proper sentinel) — never returning raw `pgx.ErrNoRows` (GAP-03 패턴 정합, EVID-001/SCORE-001 동형),
**And** the test SHALL verify `errors.Is(err, apperrors.ErrScoreReviewRequestNotFound) == true` AND `errors.Is(err, pgx.ErrNoRows) == false` (라우 누출 0).

---

## §3 (별도 번호 — Edge Cases) 16 Edge Cases

| # | 시나리오 | 기대 결과 |
|---|---------|----------|
| E1 | `APPROVED → SUBMITTED` 시도 | 409 `ErrScoreReviewRequestInvalidStatus`, 0 row 변경 |
| E2 | 존재하지 않는 `score_id`로 POST `/reviews` | 404 `NOT_FOUND` (cross-store handler-compose 2-TX 단계, §6.3 RESOLVED, strategy.md §A.3) |
| E3 | `score_id = uuid.Nil` 또는 empty string POST | 400 `INVALID_ARGUMENT`, store SQL 미실행 |
| E4 | viewer가 POST `/reviews` (create) 시도 | 403 `ABAC_CONDITION_DENIED`, 0 row 변경 |
| E5 | analyst가 POST `/reviews/{id}/approve` 시도 | 403 `ABAC_CONDITION_DENIED`, 0 row 변경 |
| E6 | analyst가 POST `/reviews/{id}/assign-reviewer` 시도 | 403 `ABAC_CONDITION_DENIED`, 0 row 변경 |
| E7 | `AUTH_ENABLED=false`로 모든 엔드포인트 호출 | 200/201 응답, `created_by='cli-anonymous'`, `audit_logs.user_id='cli-anonymous'` |
| E8 | GET `/reviews?status=SUBMITTED` 결과 0건 | 200 + `{"items":[],"total":0}` (404 아님) |
| E9 | POST `/reviews/{id}/reject` body에 `rejection_reason` 누락/empty | 400 `INVALID_ARGUMENT`, status 전이 0 |
| E10 | 두 admin이 동시에 POST `/reviews/{id}/approve` 요청 | 첫 번째 200 `APPROVED`, 두 번째 409 `ErrScoreReviewRequestNotUnderReview` (SELECT FOR UPDATE row lock + state-machine 가드, §6.6 RESOLVED, strategy.md §A.6) |
| E11 | GET `/reviews/not-a-uuid` 단건 조회 | 400 `INVALID_ARGUMENT`, store 미호출 |
| E12 | GET `/reviews?limit=10000` | 500으로 clamp 적용 |
| E13 | GET `/reviews?offset=-5` | 0으로 clamp 적용 |
| E14 | `UNDER_REVIEW` 상태에 POST `/assign-reviewer` 재시도 | 409 `ErrScoreReviewRequestNotSubmitted` |
| E15 | `SUBMITTED` 상태에 POST `/approve` 직접 시도 (skip UNDER_REVIEW) | 409 `ErrScoreReviewRequestNotUnderReview` (불법 전이) |
| E16 | `RecordScoreReviewRequest*` audit INSERT 실패 (fault inject) | `ErrScoreReviewRequestAuditWriteFailed` + 호출자 Rollback → entity+audit 0 row (양방향 취소) |

---

## §4 품질 게이트 기준 (Quality Gate Criteria)

### 4.1 TRUST 5 PASS

- **Tested**: 테스트 커버리지 ≥85% (store-layer + handler 합산), 20 AC + 16 edge case 모두 자동화
- **Readable**: 한국어 godoc/에러 메시지, 한국어 @MX 주석, exported 함수 모두 godoc
- **Unified**: `gofmt`/`goimports`/`golangci-lint` zero issues
- **Secured**: OWASP 준수, raw pgx 에러 누출 0, ABAC narrowing fail-closed, 동일-TX audit
- **Trackable**: SCORE-001/SCORE-API-001/REPORT-001 패턴 인용 commit, drift-guard 검증 commit

### 4.2 evaluator-active 4-차원 점수

- Functionality: ≥0.85 (20 AC + 16 edge 통과)
- Security: ≥0.85 (ABAC narrowing, fail-closed, 양방향 rollback)
- Craft: ≥0.85 (SCORE-001/SCORE-API-001/REPORT-001 패턴 정합)
- Consistency: ≥0.85 (한국어 일관성, frozen RBAC 0-diff, drift 0)
- **종합**: ≥0.85 (thorough harness 기준)

### 4.3 Drift-Guard 0%

- 모든 [EXISTING] 파일 `git diff` 0 (R-CONSUMER-001)
- [MODIFY] 파일 추가 범위만(기존 라인 무변경)
- 위반 시 즉시 중단·재계획

---

## §5 Definition of Done (DoD)

- [ ] research.md SSOT (665줄, file:line 근거) 검증 완료
- [ ] spec.md / plan.md / acceptance.md / spec-compact.md 4 파일 작성 완료
- [ ] §6 OPEN 6건 모두 strategy.md에서 RESOLVED + Human Gate sign-off
- [ ] M0 RED: 20+ failing test 작성, `go vet ./...` PASS
- [ ] M1 GREEN: 0005 마이그레이션 + store-layer 구현, store-layer 테스트 GREEN
- [ ] M2 GREEN: HTTP handler + ABAC 구현, handler 테스트 GREEN
- [ ] M3 GREEN: server.go 마운트 ≈7줄, 통합 빌드 PASS
- [ ] M4 REFACTOR: TRUST 5 PASS, @MX 태그 추가, 커버리지 ≥85%
- [ ] M5 Drift-Guard: 모든 [EXISTING] 파일 0 diff 검증
- [ ] evaluator-active 4-차원 점수 ≥0.85
- [ ] manager-quality TRUST 5 PASS
- [ ] consumer-only [HARD] 무위반: SCORE-001/EVAL-ITEM-001/EVID-001/AUTH-003 무수정
- [ ] frozen RBAC 0-diff: `RoleAdmin`/`RoleAnalyst`/`RoleViewer` 3역할만, 신규 역할 추가 0
- [ ] phantom 0: 모든 신규 메서드/구조체 spec.md §2.1에 명시, 모든 호출은 source-verified 라인 인용
- [ ] errors.go drift manifest 정확(SCORE-API-001 교훈) — §2.1/§2.3 명시
- [ ] server.go 마운트 ≈7줄 정확(REPORT-001 교훈) — "1줄" 잘못 기술 0

---

## 참조

- spec.md §3 EARS 요구사항
- spec.md §6 OPEN 6건 (권장 옵션 부착)
- plan.md §2 마일스톤 M0-M5
- plan.md §4 TDD 사이클 (20+ test 목록)
- research.md §12.3 strategy 단계 OPEN
- SPEC-AX-SCORE-001 acceptance.md (canonical AC 패턴 선례)
- SPEC-AX-SCORE-API-001 acceptance.md (HTTP + ABAC AC 선례)
- SPEC-AX-REPORT-001 acceptance.md (cross-store 2-TX + drift-guard 0 선례)
