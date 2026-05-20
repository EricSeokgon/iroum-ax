# Tasks: SPEC-AX-REVIEW-001 평가 제출/승인 워크플로우 저장소 + HTTP API 계층

**SPEC**: SPEC-AX-REVIEW-001 v0.1.0
**Phase**: Run — Phase 1.5 task decomposition
**Generated**: 2026-05-20
**Author**: manager-strategy
**Mode**: sub-agent TDD (manager-tdd, RED-first)
**Harness Level**: thorough
**Status**: ready (Human Gate sign-off 2026-05-20 완료)
**Precedent**: SCORE-API-001 / REPORT-001 tasks.md 구조 동형

---

## §0 RESOLVED 결정 명문 (strategy.md §A SSOT 참조)

본 SPEC의 6 OPEN은 2026-05-20 Human Gate에서 권고안 그대로 6/6 승인 완료. SSOT: `strategy.md §A.1~§A.6`. 구현 위치는 다음 표 참조.

| OPEN | RESOLVED 결정 | 구현 위치 (Milestone / 파일 / Task) |
|------|--------------|--------------------------------------|
| **#1** | `POST /api/v1/reviews/{id}/assign-reviewer` (별도 sub-resource), body `{reviewer_id}`, status `SUBMITTED→UNDER_REVIEW` + audit 1건 | M2 — `review_handlers.go` `Routes()` + `handleAssignReviewer` / T-201, T-606 |
| **#2** | `POST /{id}/approve` (body `{comment?}`) + `POST /{id}/reject` (body `{rejection_reason* required, comment?}`) (별도 sub-resource each), terminal | M2 — `review_handlers.go` `handleApprove` + `handleReject` / T-202, T-203, T-607, T-608 |
| **#3** | handler-compose 2-TX cross-store. TX-1 `BeginScoreTx`→`GetScoreByID`→Rollback (read-only). TX-2 `BeginScoreReviewRequestTx`→`Insert+audit`→Commit. Race window는 SCORE-001 물리 삭제 0이라 결정적 不發生 — PoC 수용 | M2 — `review_handlers.go::handleCreateReview` / T-204, T-605 |
| **#4** | 핸들러-로컬 매핑. 제출={RoleAnalyst, RoleAdmin}, 승인/반려/할당=RoleAdmin only, 조회=모든 인증 사용자(viewer 포함). `assigned_reviewer_id` 컬럼은 정보/감사용. frozen rbac.go / abac.go / permissionMatrix 0-diff | M2 — `review_handlers.go` `requireReviewSubmitRole` / `requireReviewAdminRole` / `guardReviewSubmit` / `guardReviewAdmin` / T-301~T-308, T-604 |
| **#5** | 이중 방어 — Layer 1 handler validation `validateReviewRequestInput` (REJECTED 시 `rejection_reason` non-empty 검증, 400 INVALID_ARGUMENT 한국어, `score.go:79-97` 동형) + Layer 2 DB CHECK constraint `score_review_requests_reject_reason_chk` (멱등 DO$$ duplicate_object, 0004 미러) | M1 — `score_review_request.go` `validateReviewRequestInput` + `0005_*.sql` CHECK / T-105, T-203, T-501, T-507 |
| **#6** | `SELECT ... FOR UPDATE` pessimistic row lock + state-machine 가드 이중 방어. TX 내 `SELECT status FROM score_review_requests WHERE id=$1 FOR UPDATE` → `validateReviewStatusTransition` → UPDATE + `RecordScoreReviewRequest*` (동일-TX). Commit/Rollback 자동 해제. `version` 컬럼 추가 0 | M1 — `score_review_request.go` `AssignReviewer` / `ApproveRequest` / `RejectRequest` / T-103, T-104, T-106, T-107, T-109, T-507 |

**Sign-off**: ircp (2026-05-20), 6/6 권고안 그대로 승인. **Status**: RESOLVED.

---

## §1 File Ownership (sub-agent TDD 단일 manager-tdd)

본 SPEC은 **sub-agent TDD** 모드로 진행 (manager-tdd 단일 agent, team mode 비활성). file ownership은 다음 [NEW]/[MODIFY]/[EXISTING] 마커를 따른다.

### §1.1 [NEW] 4 파일 (신규 생성)

| 파일 | 책임 | Owning agent |
|------|------|--------------|
| `apps/control-plane/internal/store/score_review_request.go` | `PgScoreReviewRequestTx` pgx 구현 (Insert/Get/List/UpdateStatus/AssignReviewer/Approve/Reject + validation 2개 + InsertAuditLog/Commit/Rollback). SELECT FOR UPDATE pessimistic lock 패턴. | manager-tdd |
| `apps/control-plane/cmd/server/review_handlers.go` | `ReviewHandler` + `NewReviewHandler(rs, ss, logger)` + `Routes()` (6 sub-resource 라우트, 구체 경로 먼저) + 6 핸들러 메서드 + writeReviewJSON/Err/mapReviewStoreErr + 핸들러-로컬 ABAC 게이트 4개 (`requireReviewSubmitRole`/`requireReviewAdminRole`/`guardReviewSubmit`/`guardReviewAdmin`) + cross-store 2-TX | manager-tdd |
| `apps/control-plane/cmd/server/review_handlers_test.go` | `httptest` 핸들러 단위 테스트 (20+ test): 6 엔드포인트 정상/에러, 4-상태 생명주기, 불법 전이 거부, ABAC narrowing, cross-store 점수 미존재 404, reject reason 누락 400, auth-disabled 투과, 동시성, consumer-only 경계 | manager-tdd |
| `.moai/db/schema/migrations/0005_score_review_request_tables.sql` | `score_review_requests` 테이블 신규 (멱등 패턴: CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION duplicate_object CHECK status enum + CHECK rejection_reason + CREATE INDEX IF NOT EXISTS). 0004 정확 미러 | manager-tdd |

### §1.2 [MODIFY] 6 파일 (정확한 추가 범위만)

| 파일 | 추가 범위 | 무변경 범위 |
|------|----------|-------------|
| `apps/control-plane/internal/store/store.go` | `ScoreReviewRequestStore` 인터페이스 + `ScoreReviewRequestTx` 인터페이스 + `ScoreReviewRequest` struct (fieldalignment 정렬) | 기존 인터페이스/struct 무변경 (ScoreStore/ScoreTx/Score 포함) |
| `apps/control-plane/internal/store/pg_store.go` | `BeginScoreReviewRequestTx(ctx) (ScoreReviewRequestTx, error)` 메서드 (s.pool.BeginTx + Recorder 주입, `pg_store.go:134-148` BeginScoreTx 동형) | 기존 메서드 무변경 (BeginScoreTx 포함) |
| `apps/control-plane/internal/audit/audit.go` | Action 상수 4개 (`ActionScoreReviewRequestCreated`/`...ReviewerAssigned`/`...Approved`/`...Rejected`) | 기존 상수 무변경 (ActionScore* 포함) |
| `apps/control-plane/internal/audit/recorder.go` | `RecordScoreReviewRequestCreated`/`...ReviewerAssigned`/`...Approved`/`...Rejected` 4 메서드 (local AuditTx, resource_id = `score_review_requests.id` UUID 직접 — D2 미러) | 기존 메서드 무변경 (RecordScore* 포함) |
| `apps/control-plane/internal/errors/errors.go` | 센티넬 6개 (`ErrScoreReviewRequestNotFound`/`InvalidInput`/`InvalidStatus`/`NotSubmitted`/`NotUnderReview`/`AuditWriteFailed`) | 기존 센티넬 무변경 (ErrScore* 포함). **SCORE-API-001 errors.go drift 교훈 — manifest에 명시 부착 필수** |
| `apps/control-plane/cmd/server/server.go` | **≈7줄**: `reviewH *ReviewHandler` 필드 1줄 + 주석 1줄 + `s.reviewH = NewReviewHandler(pgStore, pgStore, logger)` 생성자 1줄 + `innerMux.Handle("/api/v1/reviews", s.reviewH.Routes())` + `innerMux.Handle("/api/v1/reviews/", s.reviewH.Routes())` 2줄 + ko 주석 2줄 | 기존 필드/생성자/라우트 무변경 (scoreH/reportH 포함). **REPORT-001 server.go ≈7줄 교훈 — "1줄"이 아닌 ≈7줄 최소 단위 정확 기술** |

### §1.3 [EXISTING] consumer-only [HARD] 0-diff (절대 수정 금지)

| 파일 | consumer 호출 | Delta |
|------|--------------|-------|
| `apps/control-plane/internal/store/score.go` (SCORE-001) | `PgScoreTx.GetScoreByID` (cross-store 점수 검증용, 호출만) | 0-diff [HARD] |
| `apps/control-plane/internal/store/eval_item.go` (EVAL-ITEM-001) | — | 0-diff [HARD] |
| `apps/control-plane/internal/store/evidence.go` (EVID-001) | SELECT FOR UPDATE 패턴 미러 (코드 호출 없음) | 0-diff [HARD] |
| `apps/control-plane/cmd/server/score_handlers.go` (SCORE-API-001) | 핸들러+ABAC 패턴 미러 (코드 호출 없음) | 0-diff [HARD] |
| `apps/control-plane/cmd/server/report_handlers.go` (REPORT-001) | cross-store 2-TX 패턴 미러 (코드 호출 없음) | 0-diff [HARD] |
| `apps/control-plane/cmd/server/evidence_handlers.go` | — | 0-diff [HARD] |
| `apps/control-plane/internal/auth/rbac.go` (frozen) | `RoleAdmin`/`RoleAnalyst`/`RoleViewer`/`ParseRolesFromScope`/`UserFromContext` 호출만 | 0-diff [HARD]. `RoleReviewer`/`evaluator` 신설 0건 |
| `apps/control-plane/internal/auth/abac.go` (frozen) | `ErrCodeABACDenied` 호출만 | 0-diff [HARD] |
| `apps/control-plane/internal/auth/authz_middleware.go` (frozen) | 미들웨어 체인 자동 적용 | 0-diff [HARD] |
| `apps/control-plane/internal/auth/chain.go`, `middleware.go` (frozen) | 호출만 | 0-diff [HARD] |
| `.moai/db/schema/migrations/0001`/`0002`/`0003`/`0004` | — | 0-diff [HARD]. 본 SPEC은 0005만 신규 |
| `apps/control-plane/go.mod`, `go.sum` | 기존 직접 의존(`pgx/v5`, `google/uuid`, `zap`)만 사용 | 0-diff [HARD]. 신규 외부 의존 0건 (REQ-REVIEW-UBI-001 데이터 주권 정합, **go.mod 거짓 인벤토리 lesson 준수**) |

---

## §2 Drift-Guard Manifest

[NEW]만 신규 생성, [MODIFY]는 §1.2 정확한 추가 범위만, [EXISTING]은 0-diff. 구현 중 [EXISTING] 파일에 1줄이라도 수정 발생 시 consumer-only [HARD] 위반 → 즉시 중단·재계획 (spec.md §2.3 R-CONSUMER-001).

### §2.1 [NEW] 4 (정확 일치 검증 대상)

- `internal/store/score_review_request.go`
- `cmd/server/review_handlers.go`
- `cmd/server/review_handlers_test.go`
- `.moai/db/schema/migrations/0005_score_review_request_tables.sql`

### §2.2 [MODIFY] 6 (추가 범위 한정 검증 대상)

| 파일 | 검증 명령 (M5 Drift-Guard) | 추가 범위 정확 라인 수 (사후 검증) |
|------|---------------------------|------------------------------------|
| `internal/store/store.go` | `git diff store.go` | 신규 인터페이스 2 + struct 1 = ~40-60 line 추가만 (기존 라인 무변경) |
| `internal/store/pg_store.go` | `git diff pg_store.go` | `BeginScoreReviewRequestTx` 메서드 1개 = ~15 line 추가만 (`pg_store.go:134-148` BeginScoreTx 정확 미러) |
| `internal/audit/audit.go` | `git diff audit.go` | Action 상수 4개 = 4 line 추가만 |
| `internal/audit/recorder.go` | `git diff recorder.go` | RecordScoreReviewRequest* 4 메서드 = ~80-120 line 추가만 |
| `internal/errors/errors.go` | `git diff errors.go` | 센티넬 6개 = 6 line 추가만. **SCORE-API-001 errors.go drift 교훈 — manifest에 명시 부착 필수** |
| `cmd/server/server.go` | `git diff server.go` | **≈7줄**: 필드 1줄 + 주석 1줄 + 생성자 1줄 + 마운트 2줄 + ko 주석 2줄. **REPORT-001 server.go ≈7줄 교훈 — "1줄" 잘못 기술 금지** |

### §2.3 [EXISTING] 0-diff 검증 (M5에서 `git diff` 0 line 강제)

- `internal/store/score.go`, `eval_item.go`, `evidence.go`
- `cmd/server/score_handlers.go`, `report_handlers.go`, `evidence_handlers.go`
- `internal/auth/rbac.go`, `abac.go`, `authz_middleware.go`, `chain.go`, `middleware.go`
- `.moai/db/schema/migrations/0001`/`0002`/`0003`/`0004` (`0005`는 [NEW])
- `apps/control-plane/go.mod`, `go.sum` — **신규 외부 의존 0건. go.mod 거짓 인벤토리 금지 (lesson)**

위반 발생 시: 즉시 중단 → spec.md §2.3 R-CONSUMER-001 적용 → manager-strategy 재호출 → 재계획.

---

## §3 Atomic Task Table (RED-first, manager-tdd 단일 agent)

각 Task는 RED → GREEN 단위로 atomic. TDD RED-first 원칙: 모든 Task는 failing test 작성 후 minimal implementation으로 통과.

### §3.0 — S0 Hard-Verify Gate (선행 필수, RED 이전)

**T-001** [S0 hard-verify] consumer 진입점 source-verify gate

**목적**: phantom 회피 (lesson #9). RED 이전에 모든 consumer 진입점이 실재함을 source-verify.

**검증 항목**:

| # | 검증 | 명령 | 기대 결과 |
|---|------|------|----------|
| a | `BeginScoreTx` 진입점 실재 (cross-store 검증용) | `Grep -n "func \(s \*PgWorkflowStore\) BeginScoreTx" apps/control-plane/internal/store/pg_store.go` | `pg_store.go:134` match |
| b | `GetScoreByID` 메서드 실재 (cross-store 점수 검증용) | `Grep -n "GetScoreByID" apps/control-plane/internal/store/store.go` | `store.go:275-276` match (인터페이스 시그니처) |
| c | `rbac.go` 정규식 frozen 확인 (`RoleReviewer`/`evaluator` 미포함) | `Grep -n "roleRegex" apps/control-plane/internal/auth/rbac.go` | `rbac.go:33` `^iroum-ax:(admin\|analyst\|viewer)$` (evaluator/reviewer 매칭 0) |
| d | `0004_score_tables.sql` path 존재 (디스크 확인) | `ls -la .moai/db/schema/migrations/0004_score_tables.sql` | 파일 존재 |
| e | `0005_score_review_request_tables.sql` 미존재 (신규 생성 대상) | `ls .moai/db/schema/migrations/0005*` | "No such file" (정확) |
| f | `score_handlers.go:161-190` ABAC 패턴 실재 | `Grep -n "requireScoreWriteRole\|guardScoreWrite" apps/control-plane/cmd/server/score_handlers.go` | 4 match minimum |
| g | `audit.NewRecorder` 시그니처 실재 | `Grep -n "func NewRecorder" apps/control-plane/internal/audit/recorder.go` | match (Recorder 주입 패턴 검증) |

**Acceptance**: 7/7 모두 정확 일치. 1건이라도 불일치 시 즉시 중단 → manager-strategy 재호출 → research.md 재검증.

**Status**: PENDING (M0 진입 전 필수)

---

### §3.1 — M0 RED Phase (failing test 작성, 24 test)

#### Store-layer tests (`score_review_request_test.go`, 12 test)

**T-101** [RED] `TestInsertScoreReviewRequest_ValidInput_ReturnsUUIDAndInsertsAuditRow`
- **AC**: AC-REVIEW-001-1, AC-REVIEW-UBI-002
- **시나리오**: 유효 scoreID + comment로 Insert → UUID 반환 + score_review_requests 1 row (status='SUBMITTED') + audit_logs 1 row (action='SCORE_REVIEW_REQUEST_CREATED', resource_id = 반환 UUID)
- **fake store**: `ScoreReviewRequestStore`/`audit_logs` 격리

**T-102** [RED] `TestInsertScoreReviewRequest_BlankScoreID_ReturnsInvalidInput`
- **AC**: AC-REVIEW-001-3
- **시나리오**: `uuid.Nil` 입력 → `ErrScoreReviewRequestInvalidInput` + SQL 미실행 (fail-closed) + 0 row in 두 테이블

**T-103** [RED] `TestAssignReviewer_FromSubmitted_TransitionsToUnderReview`
- **AC**: AC-REVIEW-003-1
- **시나리오**: SUBMITTED row + AssignReviewer(reviewerID) → status='UNDER_REVIEW', assigned_reviewer_id=reviewerID, audit_logs 1건 (ActionScoreReviewRequestReviewerAssigned)
- **lock 검증**: SELECT FOR UPDATE 호출 확인 (mock pgx.Tx)

**T-104** [RED] `TestAssignReviewer_FromUnderReview_ReturnsNotSubmitted`
- **AC**: Edge E14
- **시나리오**: UNDER_REVIEW row + AssignReviewer 재시도 → `ErrScoreReviewRequestNotSubmitted` (HTTP 409), row 무변경

**T-105** [RED] `TestRejectRequest_FromUnderReview_NoReason_ReturnsInvalidInput`
- **AC**: Edge E9 (§A.5 Layer 1 handler validation + store validation)
- **시나리오**: UNDER_REVIEW row + RejectRequest(rejection_reason="") → `ErrScoreReviewRequestInvalidInput` + SQL 미실행

**T-106** [RED] `TestConcurrentAdminApprove_SelectForUpdate_OnlyFirstSucceeds`
- **AC**: AC-REVIEW-003-2, Edge E10 (§A.6 pessimistic lock)
- **시나리오**: UNDER_REVIEW row + goroutine 2개 동시 ApproveRequest → 첫 번째 200/APPROVED, 두 번째 409/ErrScoreReviewRequestNotUnderReview
- **검증**: pgx.Tx SELECT FOR UPDATE lock 직렬화

**T-107** [RED] `TestStatusTransition_FromTerminal_AlwaysReturnsInvalidStatus`
- **AC**: AC-REVIEW-003-4
- **시나리오**: APPROVED row + AssignReviewer/Approve/Reject 시도 → 모두 `ErrScoreReviewRequestInvalidStatus`. REJECTED row 동일

**T-108** [RED] `TestApproveRequest_FromUnderReview_TransitionsToApproved`
- **AC**: AC-REVIEW-003-2
- **시나리오**: UNDER_REVIEW row + ApproveRequest(comment) → status='APPROVED', audit_logs 1건 (ActionScoreReviewRequestApproved)

**T-109** [RED] `TestApproveRequest_FromSubmitted_ReturnsNotUnderReview`
- **AC**: Edge E15
- **시나리오**: SUBMITTED row + ApproveRequest 직접 시도 (skip UNDER_REVIEW) → `ErrScoreReviewRequestNotUnderReview`

**T-110** [RED] `TestRejectRequest_FromUnderReview_WithReason_TransitionsToRejected`
- **AC**: AC-REVIEW-003-3
- **시나리오**: UNDER_REVIEW row + RejectRequest("근거 부족") → status='REJECTED', rejection_reason='근거 부족', audit 1건

**T-111** [RED] `TestInsertScoreReviewRequest_AuditFailure_TwoWayRollback`
- **AC**: AC-REVIEW-004-2, Edge E16
- **시나리오**: fault inject로 audit INSERT 실패 → `ErrScoreReviewRequestAuditWriteFailed` + 호출자 Rollback → entity+audit 0 row 양방향 취소

**T-112** [RED] `TestValidateReviewStatusTransition_AllAllowedAndDisallowed`
- **AC**: AC-REVIEW-003-4
- **시나리오**: `validateReviewStatusTransition(current, target)` 매트릭스 검증 — 12개 조합 (4 current × 3 target) 중 SUBMITTED→UNDER_REVIEW, UNDER_REVIEW→APPROVED, UNDER_REVIEW→REJECTED만 허용, 나머지 9건 거부

#### HTTP handler tests (`review_handlers_test.go`, 12 test)

**T-201** [RED] `TestPOST_ReviewsAssignReviewer_AdminSucceeds_200`
- **AC**: AC-REVIEW-003-1 (§A.1)
- **시나리오**: admin scope + SUBMITTED review id + POST `/api/v1/reviews/{id}/assign-reviewer` body `{"reviewer_id":"user-123"}` → 200 + 업데이트된 entity

**T-202** [RED] `TestPOST_ReviewsApprove_AdminSucceeds_200`
- **AC**: AC-REVIEW-003-2 (§A.2)
- **시나리오**: admin + UNDER_REVIEW row + POST `/{id}/approve` body `{"comment":"승인합니다"}` → 200

**T-203** [RED] `TestPOST_ReviewsReject_RejectionReasonMissing_400`
- **AC**: Edge E9 (§A.5 Layer 1)
- **시나리오**: admin + UNDER_REVIEW row + POST `/{id}/reject` body `{"rejection_reason":""}` → 400 INVALID_ARGUMENT + 한국어 메시지 "반려 사유는 필수입니다"

**T-204** [RED] `TestPOST_Reviews_ScoreNotExist_404`
- **AC**: Edge E2 (§A.3 cross-store handler-compose)
- **시나리오**: analyst + POST `/api/v1/reviews` body `{"score_id":"<non-existent UUID>"}` → 404 NOT_FOUND (cross-store TX-1 `GetScoreByID` 단계에서 검출)
- **fake scoreStore**: `ErrScoreNotFound` 반환

**T-301** [RED] `TestPOST_Reviews_AnalystCreates_201`
- **AC**: AC-REVIEW-002-1 (§A.4 analyst=submit)
- **시나리오**: analyst scope + 유효 score + POST `/reviews` → 201 + 새 entity (status='SUBMITTED')

**T-302** [RED] `TestPOST_Reviews_ViewerForbidden_403`
- **AC**: Edge E4 (§A.4)
- **시나리오**: viewer scope + POST `/reviews` → 403 ABAC_CONDITION_DENIED + "제출 권한이 없는 사용자입니다"

**T-303** [RED] `TestPOST_ReviewsApprove_AnalystForbidden_403`
- **AC**: Edge E5 (§A.4 admin-only)
- **시나리오**: analyst scope + POST `/{id}/approve` → 403

**T-304** [RED] `TestPOST_ReviewsAssignReviewer_AnalystForbidden_403`
- **AC**: Edge E6 (§A.4 admin-only)
- **시나리오**: analyst + POST `/{id}/assign-reviewer` → 403

**T-305** [RED] `TestGET_Reviews_AuthDisabled_PassthroughCliAnonymous`
- **AC**: AC-REVIEW-UBI-003, Edge E7
- **시나리오**: `AUTH_ENABLED=false` + 모든 6 엔드포인트 호출 → 200/201, `created_by='cli-anonymous'`, `audit_logs.user_id='cli-anonymous'`

**T-306** [RED] `TestGET_Reviews_EmptyList_Returns200WithEmptyArray`
- **AC**: AC-REVIEW-002-3, Edge E8
- **시나리오**: 필터 결과 0건 → 200 + `{"items":[],"total":0}` (404 아님)

**T-307** [RED] `TestGET_Reviews_MalformedUUID_400`
- **AC**: AC-REVIEW-002-4, Edge E11
- **시나리오**: GET `/reviews/not-a-uuid` → 400 INVALID_ARGUMENT + "유효하지 않은 평가 검토 ID 형식입니다" + store 미호출

**T-308** [RED] `TestGET_Reviews_PaginationClamp`
- **AC**: AC-REVIEW-002-3, Edge E12+E13
- **시나리오**: `limit=10000` → 500으로 clamp, `offset=-5` → 0으로 clamp (`score_handlers.go:144-157` `clampPagination` 동형)

**RED Phase 완료 기준**: 24 failing test 작성. `go vet ./...` PASS. `go test ./...` 신규 테스트만 실패. 다른 SPEC 회귀 0건.

---

### §3.2 — M1 GREEN Phase (Store + Audit + Migration 구현)

**T-501** [GREEN] `0005_score_review_request_tables.sql` 작성
- **산출물**: 멱등 CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION duplicate_object CHECK status enum + **DO$$ EXCEPTION duplicate_object CHECK rejection_reason (§A.5 Layer 2)** + CREATE INDEX IF NOT EXISTS (score_id, status, created_at DESC)
- **0004 정확 미러**: 동일 패턴 4 요소
- **검증**: 정방향 적용 + 재실행 멱등성 확인 (`psql` apply twice)

**T-502** [GREEN] `internal/store/store.go` `ScoreReviewRequestStore`/`Tx` 인터페이스 + `ScoreReviewRequest` struct
- **정렬**: fieldalignment (map → time.Time × 2 → 포인터 → 문자열 → UUID)
- **EvalItemStore 패턴 미러** (`store.go:115-119/:182-213`)

**T-503** [GREEN] `internal/store/pg_store.go` `BeginScoreReviewRequestTx` 메서드
- **`pg_store.go:134-148` BeginScoreTx 정확 미러**: `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})` + `PgScoreReviewRequestTx{tx, logger, recorder: audit.NewRecorder(false)}` (L143-147 Recorder 주입)
- **신규 pgxpool 생성 금지** (단일 풀 재사용)

**T-504** [GREEN] `internal/audit/audit.go` Action 상수 4개 추가
- `ActionScoreReviewRequestCreated`/`ActionScoreReviewRequestReviewerAssigned`/`ActionScoreReviewRequestApproved`/`ActionScoreReviewRequestRejected`
- **D2 패턴**: namespace 상수 추가 0건 (UUID PK 직접 대입)

**T-505** [GREEN] `internal/audit/recorder.go` `RecordScoreReviewRequest*` 4 메서드
- **SCORE-001 `RecordScore*` 동형**: local AuditTx, `resource_id = score_review_requests.id` UUID 직접
- **시그니처**: `(ctx, tx, reviewRequestID, ..., userID)` 형태

**T-506** [GREEN] `internal/errors/errors.go` 센티넬 6개 추가
- `ErrScoreReviewRequestNotFound`/`InvalidInput`/`InvalidStatus`/`NotSubmitted`/`NotUnderReview`/`AuditWriteFailed`
- **SCORE-001 errors.go:52-78 동형 패턴**
- **drift-guard manifest 부착**: 본 tasks.md §2.2에 명시 (SCORE-API-001 lesson)

**T-507** [GREEN] `internal/store/score_review_request.go` `PgScoreReviewRequestTx` 구현 (7 메서드 + 2 validation + Insert/Commit/Rollback)
- **`validateReviewRequestInput`** (§A.5 Layer 1): SQL 미실행 후 거부, `score.go:79-97` 동형
- **`validateReviewStatusTransition`** (§A.6 state-machine): allowedReviewTransitions 매트릭스
- **`InsertScoreReviewRequest`**: INSERT + Recorder 호출 (동일 TX)
- **`AssignReviewer`/`ApproveRequest`/`RejectRequest`** (§A.6): SELECT FOR UPDATE → validateReviewStatusTransition → UPDATE + Recorder (동일 TX)

**GREEN Phase M1 완료 기준**: T-101 ~ T-112 (12 store-layer test) 모두 GREEN. M0의 store-layer test 전부 통과.

---

### §3.3 — M2 GREEN Phase (HTTP API + ABAC + Cross-store 2-TX)

**T-601** [GREEN] `cmd/server/review_handlers.go` `ReviewHandler` 구조체 + `NewReviewHandler(rs, ss, logger)`
- **scoreStore 의존성 주입**: cross-store 2-TX (§A.3)용
- **`score_handlers.go:43-51` 동형**

**T-602** [GREEN] `Routes()` 메서드 (6 sub-resource 라우트)
- **구체 경로 먼저** (Go1.22+ ServeMux 최장일치):
  - `POST /api/v1/reviews/{id}/assign-reviewer` (§A.1)
  - `POST /api/v1/reviews/{id}/approve` (§A.2)
  - `POST /api/v1/reviews/{id}/reject` (§A.2)
  - `GET /api/v1/reviews/{id}`
  - `GET /api/v1/reviews`
  - `POST /api/v1/reviews`

**T-603** [GREEN] 표준 JSON/에러 헬퍼 (`writeReviewJSON`/`writeReviewErr`/`mapReviewStoreErr`)
- **`score_handlers.go:74-101/:111-129` 동형**
- **한국어 메시지** (acceptance.md AC-REVIEW-005-1 매핑 표 7 sentinels)

**T-604** [GREEN] 핸들러-로컬 ABAC 게이트 4개 (§A.4)
- `requireReviewSubmitRole(scope) bool` — RoleAdmin || RoleAnalyst
- `requireReviewAdminRole(scope) bool` — RoleAdmin only
- `(h *ReviewHandler) guardReviewSubmit(w, r) bool` — auth-disabled 투과(`ok=false → true`), 미인가 403
- `(h *ReviewHandler) guardReviewAdmin(w, r) bool` — 동일 패턴
- **`score_handlers.go:161-190` 정확 미러**

**T-605** [GREEN] `handleCreateReview` (§A.3 cross-store 2-TX)
- TX-1: `scoreStore.BeginScoreTx` → `GetScoreByID` → Rollback (read-only, defer)
- TX-1 결과: `ErrScoreNotFound` → HTTP 404
- TX-2: `reviewStore.BeginScoreReviewRequestTx` → `InsertScoreReviewRequest` → Commit
- guard: `guardReviewSubmit` 선행

**T-606** [GREEN] `handleAssignReviewer` (§A.1)
- guard: `guardReviewAdmin`
- body parse: `{reviewer_id}` non-empty 검증
- TX: `reviewStore.BeginScoreReviewRequestTx` → `AssignReviewer(id, reviewer_id)` → Commit

**T-607** [GREEN] `handleApprove` (§A.2)
- guard: `guardReviewAdmin`
- body parse: `{comment?}` optional
- TX: `BeginScoreReviewRequestTx` → `ApproveRequest(id, comment)` → Commit

**T-608** [GREEN] `handleReject` (§A.2 + §A.5 Layer 1)
- guard: `guardReviewAdmin`
- body parse: `{rejection_reason* required, comment?}` 검증 (non-empty rejection_reason)
- pre-store validation 실패 시 400 INVALID_ARGUMENT + "반려 사유는 필수입니다"
- TX: `BeginScoreReviewRequestTx` → `RejectRequest(id, rejection_reason, comment)` → Commit

**T-609** [GREEN] `handleGetReview` + `handleListReviews`
- guard 없음 (모든 인증 사용자 + auth-disabled, §A.4 read=all)
- `handleListReviews`: `clampPagination` (limit 기본 50, max 500, offset≥0)

**GREEN Phase M2 완료 기준**: T-201 ~ T-308 (12 handler test) 모두 GREEN. ABAC narrowing 모든 경계 통과 (viewer create 403 / analyst approve 403 / admin approve 200 / auth-disabled 투과).

---

### §3.4 — M3 GREEN Phase (server.go 마운트 ≈7줄)

**T-701** [GREEN] `cmd/server/server.go` 마운트 (정확히 ≈7줄, REPORT-001 lesson)
- 필드 1줄: `reviewH *ReviewHandler` (server.go:55-56 영역)
- 주석 1줄: `// 평가 검토 핸들러 (SPEC-AX-REVIEW-001)`
- 생성자 1줄: `s.reviewH = NewReviewHandler(pgStore, pgStore, logger)` (server.go:210/212 영역)
- 주석 1줄: `// 단계 (i-4): 평가 검토 핸들러`
- 마운트 2줄: `innerMux.Handle("/api/v1/reviews", s.reviewH.Routes())` + `innerMux.Handle("/api/v1/reviews/", s.reviewH.Routes())` (server.go:266-267/269-270 영역)
- ko 주석 1줄

**완료 기준**: `go build ./...` 통과. 헬스체크 정상. `/api/v1/reviews` 라우트 reachable. 통합 빌드 PASS.

---

### §3.5 — M4 REFACTOR Phase (TRUST 5)

**T-801** [REFACTOR] 공통 헬퍼 추출 (writeReviewJSON/Err 중복 제거)

**T-802** [REFACTOR] @MX 태그 추가
- fan_in≥3 `BeginScoreReviewRequestTx` / `validateReviewStatusTransition` → @MX:ANCHOR
- 복잡도≥15 `handleCreateReview` (cross-store 2-TX) → @MX:WARN
- 모든 NEW exported 함수 → @MX:NOTE

**T-803** [REFACTOR] godoc 한국어 (language.yaml `code_comments: ko` 정합)

**T-804** [REFACTOR] 한국어 에러 메시지 정합 검토 (acceptance.md §5 매핑 표 7 sentinels)

**T-805** [REFACTOR] 커버리지 ≥85% 검증 (`go test -cover ./...`)

---

### §3.6 — M5 Drift-Guard 검증

**T-901** [Drift-Guard] [EXISTING] 0-diff 검증
- `git diff` 검증: §1.3 12 파일 + 0001~0004 마이그레이션 + `go.mod`/`go.sum` → 모두 0 line diff

**T-902** [Drift-Guard] [MODIFY] 추가 범위 검증
- §2.2 표 각 파일의 추가 범위가 정확 일치 (기존 라인 무변경 확인)

**T-903** [Drift-Guard] `server.go` ≈7줄 정확 검증
- 마운트 추가 라인 수 카운팅 (≈7줄 ± 2, "1줄" 잘못 기술 0)

**T-904** [Drift-Guard] `go.mod` 신규 외부 의존 0건 검증
- `git diff go.mod go.sum` → 0 line (REQ-REVIEW-UBI-001 정합, **go.mod 거짓 인벤토리 금지 lesson**)

**완료 기준**: drift = 0%. 위반 시 즉시 중단·재계획 (R-CONSUMER-001).

---

## §4 Sprint (Priority-Based, 시간 추정 금지)

| Sprint | Tasks | Priority | Dependency |
|--------|-------|----------|-----------|
| **S0** Hard-Verify Gate | T-001 | High | (선행) |
| **M0** RED (Test 작성) | T-101 ~ T-112 (12) + T-201 ~ T-204 (4) + T-301 ~ T-308 (8) = **24 test** | High | S0 |
| **M1** GREEN Store+Audit+Migration | T-501 ~ T-507 (7) | High | M0 |
| **M2** GREEN HTTP+ABAC+Cross-store | T-601 ~ T-609 (9) | High | M1 |
| **M3** GREEN Server.go 마운트 | T-701 | Medium | M2 |
| **M4** REFACTOR TRUST 5 | T-801 ~ T-805 (5) | Medium | M3 |
| **M5** Drift-Guard | T-901 ~ T-904 (4) | Medium | M4 |

**총 50 Tasks (T-001 ~ T-904), 24 failing test (M0 RED).**

---

## §5 AC 20 ↔ Task Coverage (verified)

| AC ID | REQ | 주 Task | 보조 Task |
|-------|-----|---------|----------|
| AC-REVIEW-UBI-001 | UBI-001 | T-904 (go.mod 0-diff) | — |
| AC-REVIEW-UBI-002 | UBI-002 | T-101, T-111 | T-501 (CHECK status) |
| AC-REVIEW-UBI-003 | UBI-003 | T-305 | T-503 (cli-anonymous) |
| AC-REVIEW-UBI-004 | UBI-004 | T-302, T-303, T-304, T-107, T-112 | T-501 (CHECK status enum), T-507 (state-machine 가드) |
| AC-REVIEW-001-1 | 001-E1 | T-101 | T-502, T-505, T-507 |
| AC-REVIEW-001-2 | 001-S1 | T-204 | T-501 (FK-less stub) |
| AC-REVIEW-001-3 | 001-U1 | T-102 | T-507 (validateReviewRequestInput) |
| AC-REVIEW-001-4 | 001-O1 | (T-101 metadata 검증) | T-501 (JSONB) |
| AC-REVIEW-002-1 | 002-E1 | T-301 | T-605 (handleCreateReview) |
| AC-REVIEW-002-2 | 002-E2 | T-307 | T-609 (handleGetReview) |
| AC-REVIEW-002-3 | 002-E3 | T-306, T-308 | T-609 (handleListReviews) |
| AC-REVIEW-002-4 | 002-U1 | T-307 | T-609 (UUID parsing) |
| AC-REVIEW-003-1 | 003-E1 | T-103, T-201 | T-507 (AssignReviewer), T-606 |
| AC-REVIEW-003-2 | 003-E2 | T-108, T-106, T-202 | T-507 (ApproveRequest), T-607 |
| AC-REVIEW-003-3 | 003-E3 | T-110, T-203 | T-507 (RejectRequest), T-608 |
| AC-REVIEW-003-4 | 003-S1 | T-107, T-112 | T-507 (validateReviewStatusTransition) |
| AC-REVIEW-004-1 | 004-E1 | T-101 (audit assert) | T-505 (Recorder methods) |
| AC-REVIEW-004-2 | 004-U1 | T-111 | T-506 (ErrAuditWriteFailed) |
| AC-REVIEW-005-1 | 005-E1 | T-203, T-307 | T-603 (mapReviewStoreErr) |
| AC-REVIEW-005-2 | 005-U1 | (T-204 wrapping 검증) | T-507 (ErrNoRows wrap) |

**Coverage**: 20/20 AC ↔ Task 매핑 ✓ (orphan AC 0, uncovered REQ 0). plan-audit.md §3.2 traceability 정합.

---

## §6 16 Edge Cases ↔ Task Mapping (verified)

| # | Edge | Task |
|---|------|------|
| E1 | APPROVED → SUBMITTED 시도 | T-107 |
| E2 | non-existent score_id POST | T-204 |
| E3 | blank/zero score_id | T-102 |
| E4 | viewer POST create | T-302 |
| E5 | analyst POST approve | T-303 |
| E6 | analyst POST assign-reviewer | T-304 |
| E7 | auth-disabled 모든 endpoint | T-305 |
| E8 | empty list 0 rows | T-306 |
| E9 | reject reason 누락/empty | T-105, T-203 |
| E10 | 동시 admin approve race | T-106 |
| E11 | GET malformed UUID | T-307 |
| E12 | limit > 500 clamp | T-308 |
| E13 | offset < 0 clamp | T-308 |
| E14 | assign-reviewer on UNDER_REVIEW row | T-104 |
| E15 | approve on SUBMITTED row | T-109 |
| E16 | audit INSERT fail rollback | T-111 |

**Coverage**: 16/16 edge ↔ Task ✓

---

## §7 Risks (R-RV-001 ~ R-RV-010, plan.md §5 보강)

| ID | 위험 | 영향 | 완화 (Task) |
|----|------|------|-------------|
| **R-RV-001** | consumer-only drift (기존 [EXISTING] 수정) | HARD 위반, SCORE-001 무수정 깨짐 | T-901 (M5 git diff 0 강제) |
| **R-RV-002** | phantom API (신규 시그니처 오류) | 컴파일 실패 / 잘못된 동작 | T-001 (S0 hard-verify gate) |
| **R-RV-003** | 0005 마이그레이션 비멱등 | 재실행 시 duplicate_object 에러 | T-501 (DO$$ EXCEPTION 패턴, 정방향+재실행 테스트) |
| **R-RV-004** | frozen rbac.go 0-diff 위반 (RoleReviewer 신설) | AUTH-003 frozen 계약 위반 | T-604 (핸들러-로컬 게이트만), T-901 (rbac.go diff 0) |
| **R-RV-005** | cross-store 2-TX race (점수 삭제 race) | 점수 검증 후 INSERT 사이 race | spec.md §6.3 RESOLVED 명시 trade-off — SCORE-001 물리 삭제 0이라 결정적 不發生 (PoC 수용) |
| **R-RV-006** | 동시 admin 승인 race | 둘 다 APPROVED 처리 | T-507 (SELECT FOR UPDATE + validateReviewStatusTransition 이중 방어), T-106 |
| **R-RV-007** | errors.go drift manifest 누락 (SCORE-API-001 lesson) | M5 drift-guard 통과 실패 | tasks.md §1.2 + §2.2에 errors.go [MODIFY] 명시 부착 완료 |
| **R-RV-008** | server.go 마운트 "1줄"로 잘못 기술 (REPORT-001 lesson) | manifest accuracy 위반 | tasks.md §1.2 + §2.2 + T-701에 ≈7줄 명시 |
| **R-RV-009** | 신규 외부 의존 도입 (go.mod 거짓 인벤토리 lesson) | REQ-REVIEW-UBI-001 데이터 주권 위반 | T-904 (go.mod/go.sum diff 0 강제) |
| **R-RV-010** | rejection_reason validation 우회 (handler bypass) | invariant 약화 | §A.5 Layer 2 DB CHECK constraint (T-501)이 fail-safe 최후 방어 |

---

## §8 Definition of Done (DoD)

- [ ] T-001 S0 hard-verify gate 7/7 PASS (phantom 0건 확인)
- [ ] M0 RED: 24 failing test 작성, `go vet ./...` PASS
- [ ] M1 GREEN: 0005 마이그레이션 + store-layer 7 task 완성, T-101~T-112 GREEN
- [ ] M2 GREEN: HTTP handler 9 task 완성, T-201~T-308 GREEN
- [ ] M3 GREEN: server.go ≈7줄 마운트, 통합 빌드 PASS
- [ ] M4 REFACTOR: TRUST 5 PASS (Tested ≥85%, Readable godoc 한국어, Unified gofmt/golangci-lint zero, Secured OWASP, Trackable 패턴 인용 commit), @MX 태그 추가
- [ ] M5 Drift-Guard: T-901~T-904 모두 0-diff 검증 PASS
- [ ] evaluator-active 4-차원 점수 ≥0.85 (Functionality / Security / Craft / Consistency)
- [ ] manager-quality TRUST 5 PASS
- [ ] consumer-only [HARD] 무위반: SCORE-001/EVAL-ITEM-001/EVID-001/AUTH-003/SCORE-API-001/REPORT-001 0-diff
- [ ] frozen RBAC 0-diff: rbac.go/abac.go/permissionMatrix 무변경
- [ ] 6 RESOLVED 결정 정확 반영 (strategy.md §A.1~§A.6 SSOT 정합)
- [ ] phantom 0: 모든 신규 함수/메서드 spec.md §2.1에 명시, 모든 [EXISTING] 호출 source-verified
- [ ] errors.go drift manifest 정확 (SCORE-API-001 lesson)
- [ ] server.go 마운트 ≈7줄 정확 (REPORT-001 lesson, "1줄" 0건)
- [ ] go.mod/go.sum 0 diff (REQ-REVIEW-UBI-001 데이터 주권, 거짓 인벤토리 lesson)
- [ ] 한국어 에러 메시지 정합 (acceptance.md §5 매핑 표 7 sentinels)
- [ ] AC 20/20 + Edge 16/16 ↔ Task 매핑 검증 (§5/§6)

---

## §9 참조

- `strategy.md` §A.1~§A.6 (RESOLVED 결정 SSOT)
- `spec.md` §6.1~§6.6 (RESOLVED 결정 요약 + cross-reference)
- `spec.md` §2.1 (영향파일 + Delta 마커)
- `spec.md` §2.3 (Drift-Guard Manifest)
- `spec.md` §3 (EARS 20 AC)
- `plan.md` §2 (M0~M5 마일스톤)
- `plan.md` §4.1 (RED phase 24 test 목록)
- `plan.md` §6.3 (RESOLVED 검증)
- `acceptance.md` §0~§5 (20 AC + 16 edge + DoD)
- `plan-audit.md` (iteration 1 PASS, §3.5 OPEN SOUND)
- `research.md` §12.3 (strategy 단계 OPEN 권고 근거)
- SPEC-AX-SCORE-001 tasks.md (store+audit 패턴 선례)
- SPEC-AX-SCORE-API-001 tasks.md (HTTP + ABAC 패턴 선례)
- SPEC-AX-REPORT-001 tasks.md (consumer-only + ≈7줄 + cross-store 2-TX 선례)

---

**Status**: ready. M0 RED 진입 가능. Human Gate sign-off 2026-05-20 완료.
