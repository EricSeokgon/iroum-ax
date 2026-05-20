---
id: SPEC-AX-REVIEW-001
version: 0.1.0
status: draft
created: 2026-05-20
updated: 2026-05-20
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-20): 평가 제출/승인 워크플로우 저장소 + HTTP API 계층(Score Review Request Store + HTTP API Layer) 첫 초안. SPEC-AX-SCORE-001(완료, v0.1.3)이 확립한 `scores` 테이블 위에 **평가 제출/검토/승인 4-상태 생명주기 저장소 + REST HTTP API 계층**을 추가한다(SPEC-AX-SCORE-001 + SPEC-AX-SCORE-API-001 수직 슬라이스 선례를 단일 SPEC으로 결합 — research.md §13). 4 상태: `SUBMITTED`(analyst 제출) → `UNDER_REVIEW`(admin 검토자 할당) → `APPROVED`/`REJECTED`(terminal, 되돌리기 불가, research.md §3.1). 신규 store 도메인(`ScoreReviewRequestStore`/`ScoreReviewRequestTx`) + 신규 마이그레이션 `0005_score_review_request_tables.sql` + 동일-TX `RecordScoreReviewRequest*` 감사(SCORE-001 D2/D4 패턴 미러) + 6 HTTP 엔드포인트(생성/조회/목록/검토자할당/승인/반려). 핸들러-로컬 ABAC narrowing(analyst=제출 / admin=승인·반려·검토자할당 / viewer+모든 인증=조회) — frozen `rbac.go`(`admin`/`analyst`/`viewer` 3역할, `rbac.go:33` 정규식 `^iroum-ax:(admin|analyst|viewer)$`) **0-diff**(SCORE-API-001 `requireScoreWriteRole`/`guardScoreWrite` `score_handlers.go:161-187` 동형 패턴 재사용). `score_review_requests.score_id`는 SCORE-001 `scores.id`(UUID)를 참조하는 **FK-제약-없는 stub**(consumer-only [HARD]) — SCORE-001 코드·스키마·`0004` 마이그레이션 무수정(0 diff). 한국 공공 6제약(데이터 주권/한국어/감사 가능성/망분리/조직 격리/시간 제약) 준수. research.md(Phase 0.5 deep research, 665줄, file:line 근거)가 SSOT. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-REPORT-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등 canonical 외 필드는 사용하지 않는다. 본 SPEC의 모든 EARS 요구사항·영향파일·HTTP 계약·DB 스키마는 `.moai/specs/SPEC-AX-REVIEW-001/research.md`(file:line 근거)에 근거하며, 소비 계약 시그니처는 `apps/control-plane/internal/store/store.go`(`ScoreStore`/`ScoreTx` `store.go:253-305`, `EvalItemStore` `store.go:115-119` 패턴 미러 대상), `apps/control-plane/internal/store/pg_store.go`(`BeginScoreTx` `pg_store.go:134-148` Recorder 주입 선례), `apps/control-plane/cmd/server/score_handlers.go`(핸들러+ABAC 선례 `:43-51/:59-70/:111-129/:161-187/:179-190`), `apps/control-plane/internal/auth/rbac.go`(frozen 3-role `:19-26/:33/:68-80`), `apps/control-plane/internal/errors/errors.go`(센티넬 패턴 `:52-78`), `apps/control-plane/cmd/server/server.go`(라우트 마운트 ≈7줄 — `scoreH` 필드 `:55`, 생성 `:210`, 마운트 `:266-267`), `.moai/db/schema/migrations/0004_score_tables.sql`(멱등 패턴)에서 직접 검증되었다(phantom API 0건 — manager-spec orchestrator ground-truth 검증, 2026-05-20).

---

# SPEC-AX-REVIEW-001 — 평가 제출/승인 워크플로우 저장소 + HTTP API 계층 (Score Review Request Store + HTTP API Layer)

## 1. 개요

경영평가팀이 SPEC-AX-SCORE-001이 완성한 `scores` 데이터 위에 **평가 점수 제출/검토자 할당/최종 승인 또는 반려의 4-상태 워크플로우**를 운영할 수 있도록, `apps/control-plane/`(Go 1.23+)에 **신규 store 도메인 + HTTP API 계층**을 한 SPEC으로 추가한다. 본 SPEC은 SPEC-AX-SCORE-001 + SPEC-AX-SCORE-API-001이 형성한 store/audit 패턴(`BeginXxxTx` + `RecordXxx*` 동일-TX + 핸들러-로컬 ABAC narrowing)을 정확히 미러링하며(research.md §1.1/§1.2/§5/§7), score 데이터 자체는 일절 변경하지 않는다(consumer-only of SCORE-001 §1.4).

### 1.1 본 SPEC 범위 — 수직 슬라이스 (store + audit + HTTP API)

본 SPEC은 SPEC-AX-SCORE-001(store-only) + SPEC-AX-SCORE-API-001(API-only) 두 SPEC을 결합한 **단일 수직 슬라이스**이다. 사용자 인터뷰(2026-05-20)로 확정된 범위가 작고 명확하므로 store↔API를 분리하지 않는다.

- 신규 파일 4개: `internal/store/score_review_request.go`(PgScoreReviewRequestTx 구현), `cmd/server/review_handlers.go`(ReviewHandler+Routes()+6 핸들러+ABAC), `cmd/server/review_handlers_test.go`(httptest 단위 테스트), `.moai/db/schema/migrations/0005_score_review_request_tables.sql`(멱등 신규 마이그레이션)
- 기존 파일 6개 수정: `internal/store/store.go`(ScoreReviewRequestStore/Tx 인터페이스 추가), `internal/store/pg_store.go`(BeginScoreReviewRequestTx + Recorder 주입), `internal/audit/audit.go`(액션 상수 4개), `internal/audit/recorder.go`(RecordScoreReviewRequest* 메서드 4개), `internal/errors/errors.go`(센티넬 6개), `cmd/server/server.go`(라우트 마운트 ≈7줄)
- 4-상태 생명주기: `SUBMITTED` → `UNDER_REVIEW` → `APPROVED`/`REJECTED`(terminal, 되돌리기 금지)
- 6 HTTP 엔드포인트(엔드포인트 형태는 §6 OPEN #1·#2): 생성·조회·목록·검토자할당·승인·반려
- 동일-TX audit: entity-INSERT → `audit_logs` INSERT(SCORE-001 D2 패턴 — resource_id = `score_review_requests.id` UUID 직접 대입)
- ABAC narrowing(핸들러-로컬, frozen `rbac.go` 0-diff): analyst=제출, admin=승인·반려·검토자할당, viewer+모든 인증=조회
- cli-anonymous 기본값 + auth-disabled Walking Skeleton fallback (SCORE-001 §1.1, AUTH-003 정합)
- 표준 에러: `score_handlers.go` `{"error":{"code","message","field"}}` 동일 스키마(research.md §11.2)

### 1.2 Anchor 컨텍스트

본 SPEC은 SPEC-AX-SCORE-001이 형성한 점수 흐름(raw → item → category → grade)의 **상위 운영 워크플로우**(분석가 제출 → 행정가 검토 → 승인/반려)를 HTTP로 외부에 노출하여 `product.md` 경영평가 편람의 채점 결과 검토·승인 절차를 클라이언트가 운영할 수 있게 한다(research.md §1.1, §3). PoC 범위는 "안전보건" 범주 점수의 검토 워크플로우이며, SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-AUTH-003 / SPEC-AX-CTRL-001은 GREEN(완료) 상태로 가정한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `REVIEW` (Score Review Request workflow sub-domain — SCORE-001 점수의 검토/승인 운영 계층)
- 따라서 SPEC ID: `SPEC-AX-REVIEW-001` (`.claude/skills/moai/workflows/plan.md` Composite domain rules "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내 — `AX` + `REVIEW`)

### 1.4 의존성 stub 계약 — consumer-only of SCORE-001 [핵심, load-bearing]

본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-AUTH-003의 **순수 consumer**이다. 다음이 **HARD 계약**이다(research.md §2/§12, 메모리 lesson #9 phantom 회피):

- **[HARD]** 본 SPEC은 `internal/store/score.go`(SCORE-001 `PgScoreTx`), `internal/store/store.go`의 `ScoreStore`/`ScoreTx`/`Score`/`ScoreUpdate` 정의, `internal/audit/audit.go`의 기존 `ActionScore*` 상수, `internal/audit/recorder.go`의 기존 `RecordScore*` 메서드, `cmd/server/score_handlers.go`, `.moai/db/schema/migrations/0004_score_tables.sql`, `internal/auth/rbac.go`/`abac.go`(frozen RBAC)를 **일절 수정하지 않는다**. 점수 비즈니스 로직·점수 audit·점수 핸들러·점수 마이그레이션·RBAC 매트릭스는 호출만 한다.
- **[HARD]** `score_review_requests.score_id` = `UUID NOT NULL` — SCORE-001 `scores.id`(UUID PK, `store.go:229` `Score.ID uuid.UUID`, `0004_score_tables.sql:13` 검증) 참조의 **FK-제약-없는 stub**(SCORE-001 §1.4 stub 계약 동형). 점수 존재 검증은 핸들러 단계에서 `ScoreStore.BeginScoreTx` → `ScoreTx.GetScoreByID`(`store.go:276`) 호출 — 별도 TX의 cross-store 조합(§6 OPEN #3, REPORT-001 §1.4 2-TX 선례 미러).
- **[HARD]** 신규 마이그레이션 1건: `0005_score_review_request_tables.sql`. SCORE-001 `0004` 멱등 패턴(0002/0003/0004 선례 일관, research.md §4.2 검증) 미러 — `CREATE TABLE IF NOT EXISTS` + `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object` + `CREATE INDEX IF NOT EXISTS`. `0004` / 기타 기존 마이그레이션은 무수정.
- **[HARD]** 동일-TX audit 있음(REPORT-001과 다른 점): mutation 4종(생성/검토자할당/승인/반려)이 각각 동일 `pgx.Tx`에 `audit_logs` 1건을 기록한다. SCORE-001 `BeginScoreTx`(`pg_store.go:134-148`)의 Recorder 주입 패턴을 정확 미러(`pg_store.go:143-147` `recorder: audit.NewRecorder(false)` 동형). audit 행 0건은 SCORE-001 위반(DC-UBI-002)이며 본 SPEC도 동일 불변식이다.
- **[HARD]** `postgres.go`는 Sprint-0 死 스텁(SPEC-AX-SCORE-001 plan.md §2)이며 본 SPEC 대상 아님. TX 진입점은 `store.ScoreReviewRequestStore.BeginScoreReviewRequestTx`(→ `pg_store.go PgWorkflowStore.pool` 단일 풀 재사용, `pg_store.go:134` `BeginScoreTx` 패턴 미러)만 사용한다. 신규 `pgxpool.Pool` 생성 금지.
- **[HARD]** 신규 외부 의존 0건 — 본 SPEC은 어떤 신규 라이브러리/SDK도 `go.mod`에 추가하지 않는다. 기존 `pgx/v5`, `google/uuid`, `zap`만 사용한다.

### 1.5 ABAC 권한 경계 [핵심] — 핸들러-로컬 narrowing, frozen RBAC 0-diff

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 SPEC-AX-SCORE-API-001 `requireScoreWriteRole`/`guardScoreWrite`(`score_handlers.go:161-187`/`:179-190` source-verified) **동형 패턴으로 재사용**한다. 핸들러-로컬 매핑이며 frozen `rbac.go`(`RoleAdmin`/`RoleAnalyst`/`RoleViewer` 3역할, scope 정규식 `^iroum-ax:(admin|analyst|viewer)$` `rbac.go:33` source-verified)는 0-diff이다.

- **제출** (`POST /api/v1/reviews` 생성): `RoleAnalyst` 또는 `RoleAdmin` 보유 principal 허용 — 핸들러-로컬 `requireReviewSubmitRole`(SCORE-API-001 `requireScoreWriteRole` 동형). viewer/anonymous는 403.
- **검토자 할당 / 승인 / 반려**: `RoleAdmin`만 허용 — 핸들러-로컬 `requireReviewAdminRole`(admin-only 명시 게이트). analyst/viewer는 403.
- **조회 (GET 단건/목록)**: 모든 인증 사용자 허용(`RoleViewer` 포함, read-only). write-role 게이트 미차용.
- **auth-disabled 투과**: `authEnabled=false`(Walking Skeleton 기본값) → ABAC/RBAC 미들웨어 자동 투과(`abac.go:8` REQ-ABAC-009 정합), store 계층이 `cli-anonymous` 기록(SCORE-001 §1.1 동형). 본 SPEC은 인증 비활성에서도 동작한다.
- **admin 우회**: `RoleAdmin`은 모든 ABAC narrowing을 우회한다(`abac.go:71` REQ-ABAC-004 정합).
- **narrowing-only**: ABAC는 권한 부여 없이 미인가만 거부한다(`abac.go:11` 정합).

> **[중요 — SCORE-API-001 OPEN #4 충돌 不發生]** SCORE-API-001 §1.5 / §6 OPEN #4는 "write 권한 보유자가 누구인가(특히 `evaluator` 역할 부재 충돌)"가 strategy phase 미해결 사안이었다. 본 SPEC은 **역할 매핑이 명확**하다 — analyst=제출(rbac.go에 존재), admin=승인/반려/할당(rbac.go에 존재), viewer=조회(rbac.go에 존재). `evaluator` 같은 신규 역할이 필요하지 않으므로 frozen `rbac.go` 0-diff가 자연스럽게 성립한다(research.md §6.3). 따라서 본 SPEC §6 OPEN은 SCORE-API-001 OPEN #4를 상속하지 않는다.

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` `apps/control-plane/` 트리를 따른다. 본 SPEC은 SCORE-001 / SCORE-API-001 / AUTH-003 코드는 일절 수정하지 않고(consumer-only §1.4 HARD), 평가 검토 도메인 신규 파일과 통합 지점(store interface 추가/마이그레이션/audit constants/server 라우트)만 추가/수정한다. Delta 마커: [EXISTING]=consumer로 호출만(무변경), [NEW]=신규 추가, [MODIFY]=정확한 추가 범위만.

### 2.1 Go Control Plane — store + audit + HTTP API 계층

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/internal/store/score_review_request.go` | `PgScoreReviewRequestTx` pgx 구현 — `InsertScoreReviewRequest`/`GetScoreReviewRequestByID`/`ListScoreReviewRequests`/`UpdateScoreReviewRequestStatus`/`AssignReviewer`/`ApproveRequest`/`RejectRequest` + `validateReviewRequestInput`/`validateReviewStatusTransition`(SCORE-001 `validateScoreStatusTransition` 동형). 동일 TX에 `InsertAuditLog` 호출. EvalItem/Score store 패턴(`eval_item.go`/`score.go`) 미러. | [NEW] | REQ-REVIEW-001/004 |
| `apps/control-plane/cmd/server/review_handlers.go` | `ReviewHandler` struct + `NewReviewHandler(store, logger)` + `Routes() http.Handler`(ServeMux Go1.22+ 최장일치, 구체 경로 먼저) + 6 핸들러 메서드(handleCreateReview/handleGetReview/handleListReviews/handleAssignReviewer/handleApprove/handleReject) + 표준 JSON/에러 헬퍼(`writeReviewJSON`/`writeReviewErr`/`mapReviewStoreErr` — `score_handlers.go:74-101/:111-129` 미러) + 핸들러-로컬 ABAC 게이트(`requireReviewSubmitRole`/`requireReviewAdminRole`/`guardReviewSubmit`/`guardReviewAdmin` — `score_handlers.go:161-190` 동형). | [NEW] | REQ-REVIEW-002/003/005 |
| `apps/control-plane/cmd/server/review_handlers_test.go` | `httptest` 기반 핸들러 단위 테스트 — 6 엔드포인트 정상/에러 경로, 4-상태 생명주기 전이(`SUBMITTED→UNDER_REVIEW→APPROVED`/`REJECTED`), 불법 전이 거부(409), ABAC narrowing(analyst create 200 / viewer create 403 / analyst approve 403 / admin approve 200), 점수 미존재(404), reject reason 누락(400), auth-disabled 투과, 동시성(SELECT FOR UPDATE), consumer-only 경계(SCORE-001 store 무변경 검증). store는 fake `ScoreStore`/`ScoreReviewRequestStore` 격리. | [NEW] | 전체 |
| `.moai/db/schema/migrations/0005_score_review_request_tables.sql` | `score_review_requests` 테이블 신규 — id UUID PK / score_id UUID NOT NULL FK-less stub / status VARCHAR(32) NOT NULL DEFAULT 'SUBMITTED' / assigned_reviewer_id VARCHAR(256) NULL / rejection_reason TEXT NULL / comment TEXT NULL / created_at/updated_at TIMESTAMPTZ / created_by/updated_by VARCHAR(64) DEFAULT 'cli-anonymous' / metadata JSONB. 멱등 패턴(CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION duplicate_object CHECK 상태 enum + CREATE INDEX IF NOT EXISTS score_id_idx/status_idx/created_at_idx). 0001/0002/0003/0004 디스크 확인 후 0005 비충돌(research.md §4.1). | [NEW] | REQ-REVIEW-001 |
| `apps/control-plane/internal/store/store.go` | `ScoreReviewRequestStore` 인터페이스 추가(BeginScoreReviewRequestTx 진입점만, `EvalItemStore` `store.go:115-119` 패턴 미러) + `ScoreReviewRequestTx` 인터페이스 추가(Insert/Get/List/UpdateStatus/AssignReviewer/Approve/Reject/InsertAuditLog/Commit/Rollback — `EvalItemTx` `store.go:182-213` 패턴 미러) + `ScoreReviewRequest` 도메인 struct(fieldalignment 정렬: map → time.Time × 2 → 포인터 → 문자열 → UUID). | [MODIFY] | REQ-REVIEW-001 |
| `apps/control-plane/internal/store/pg_store.go` | `BeginScoreReviewRequestTx(ctx) (ScoreReviewRequestTx, error)` 메서드 추가 — `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})`(`pg_store.go:84/103/118/134` 동형) + `PgScoreReviewRequestTx{tx, logger, recorder: audit.NewRecorder(false)}` 반환(`pg_store.go:143-147` `BeginScoreTx`의 Recorder 주입 정확 미러 — authEnabled=false → user_id='cli-anonymous'). 신규 `pgxpool.Pool` 생성 금지(단일 풀 재사용, postgres.go 死 스텁 비대상). | [MODIFY] | REQ-REVIEW-001/004 |
| `apps/control-plane/internal/audit/audit.go` | Action 상수 4개 추가: `ActionScoreReviewRequestCreated = "SCORE_REVIEW_REQUEST_CREATED"` / `ActionScoreReviewRequestReviewerAssigned = "SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED"` / `ActionScoreReviewRequestApproved = "SCORE_REVIEW_REQUEST_APPROVED"` / `ActionScoreReviewRequestRejected = "SCORE_REVIEW_REQUEST_REJECTED"`. SCORE-001 `ActionScoreCreated`/`ActionScoreUpdated`(`audit.go` `:69-74` 영역 — research.md §7.1) 동형 추가. namespace 상수 0(D2 — UUID PK 직접 대입). | [MODIFY] | REQ-REVIEW-UBI-002 |
| `apps/control-plane/internal/audit/recorder.go` | 메서드 4개 추가: `RecordScoreReviewRequestCreated(ctx, tx, reviewRequestID, scoreID, submitterID, userID)` / `RecordScoreReviewRequestReviewerAssigned(ctx, tx, reviewRequestID, reviewerID, assignedByID, userID)` / `RecordScoreReviewRequestApproved(ctx, tx, reviewRequestID, approverID, userID)` / `RecordScoreReviewRequestRejected(ctx, tx, reviewRequestID, approverID, rejectionReason, userID)`. SCORE-001 `RecordScoreCreated`/`RecordScoreUpdated`(`recorder.go` SCORE-001 `RecordScore*` 영역 — research.md §7.2) 동형 — local AuditTx, resource_id = `score_review_requests.id` UUID 직접 대입(D2 미러). | [MODIFY] | REQ-REVIEW-UBI-002 |
| `apps/control-plane/internal/errors/errors.go` | 센티넬 6개 추가(SCORE-001 센티넬 `errors.go:52-78` 동형 패턴): `ErrScoreReviewRequestNotFound`(404), `ErrScoreReviewRequestInvalidInput`(400), `ErrScoreReviewRequestInvalidStatus`(409, 불법 전이), `ErrScoreReviewRequestNotSubmitted`(409, 검토자 할당 대상이 SUBMITTED 아님), `ErrScoreReviewRequestNotUnderReview`(409, 승인/반려 대상이 UNDER_REVIEW 아님), `ErrScoreReviewRequestAuditWriteFailed`(SCORE-001 `ErrScoreAuditWriteFailed` 동형 — 양방향 rollback 트리거). 모든 SCORE-001 센티넬은 무수정. | [MODIFY] | REQ-REVIEW-005-U1 |
| `apps/control-plane/cmd/server/server.go` | **라우트 마운트 ≈7줄**(SCORE-API-001 / REPORT-001 정확 미러): `reviewH *ReviewHandler` 필드 추가(`server.go:55-56` `scoreH`/`reportH` 선례 라인) + `s.reviewH = NewReviewHandler(pgStore, logger)` 생성자(`server.go:210/212` `NewScoreHandler`/`NewReportHandler` 선례) + `innerMux.Handle("/api/v1/reviews", s.reviewH.Routes())` + `innerMux.Handle("/api/v1/reviews/", s.reviewH.Routes())` 마운트 2줄(`server.go:266-267/269-270` 선례 정확 미러 — Go1.22 ServeMux path-param 서브트리 라우팅 구조적 필수) + ko 주석. ABAC 와이어링은 기존 미들웨어 체인이 innerMux 전체 자동 적용 — **ABAC 와이어링 변경 0-diff**. | [MODIFY] | REQ-REVIEW-002/003 |

### 2.2 소비 계약 — 호출만, 무변경 (consumer-only [HARD] §1.4)

| 경로 | 소비 계약 | Delta |
|------|----------|-------|
| `apps/control-plane/internal/store/store.go` (SCORE-001 정의 부분) | `ScoreStore.BeginScoreTx`(`store.go:253-255`), `ScoreTx.GetScoreByID`(`store.go:275-276`) — 점수 존재 검증용 호출만 | [EXISTING] |
| `apps/control-plane/internal/store/score.go` | `PgScoreTx.GetScoreByID` 구현 — 호출만, 점수 비즈니스 로직 무변경 | [EXISTING] |
| `apps/control-plane/internal/store/pg_store.go` (SCORE-001 BeginScoreTx) | `PgWorkflowStore.BeginScoreTx`(`pg_store.go:134-148`) — 점수 검증 TX 진입점, 호출만 | [EXISTING] |
| `apps/control-plane/internal/audit/audit.go`/`recorder.go` (SCORE-001 부분) | `ActionScoreCreated`/`ActionScoreUpdated` 상수 및 `RecordScoreCreated`/`RecordScoreUpdated`/`RecordScoreReviewRequestSupersede*` 메서드 — 무변경, 동형 패턴 참조만 | [EXISTING] |
| `apps/control-plane/cmd/server/score_handlers.go` | `ScoreHandler` 구조·`Routes()`·`writeScoreJSON`/`writeScoreErr`/`mapStoreErr`/`requireScoreWriteRole`/`guardScoreWrite`(`score_handlers.go:43-51/:59-70/:74-101/:111-129/:161-190` 검증) — 패턴 미러만(코드 무변경) | [EXISTING] |
| `apps/control-plane/internal/auth/abac.go`, `authz_middleware.go`, `chain.go` | `ErrCodeABACDenied`(abac.go:24), `ABACEvaluator`/narrowing-only/admin 우회/auth-disabled 투과, `RESTAuthzMiddleware` — 호출만/미들웨어 체인 자동 적용. **0-diff [HARD]** | [EXISTING] |
| `apps/control-plane/internal/auth/rbac.go`, `middleware.go` | `RoleAdmin`/`RoleAnalyst`/`RoleViewer`(`rbac.go:19-26`), scope 정규식(`rbac.go:33`), `ParseRolesFromScope`(`rbac.go:68-80`), `UserFromContext`, `User` struct — 호출만. **permissionMatrix/Authorize/정규식 frozen — 무변경 [HARD]**. 신규 역할 추가 0(특히 `evaluator`/`reviewer` 신설 금지) | [EXISTING] |
| `.moai/db/schema/migrations/0001`/`0002`/`0003`/`0004` | SCORE-001/EVID-001/EVAL-ITEM-001/CTRL-001 기존 마이그레이션 — 본 SPEC은 마이그레이션 추가/수정 0건(0005만 신규) | [EXISTING] |
| `apps/control-plane/go.mod` | 기존 직접 의존 무변경 — 신규 외부 의존 추가 0건(`pgx/v5`/`google/uuid`/`zap` 등 기존만 사용) | [EXISTING] |

### 2.3 Drift-Guard Manifest

[NEW]만 신규 생성, [MODIFY]는 정확히 명시된 추가 범위만, [EXISTING]은 0 diff. 구현 중 [EXISTING] 파일에 1줄이라도 수정 발생 시 consumer-only [HARD] 위반 → 즉시 중단·재계획.

- **신규 생성 허용**: `internal/store/score_review_request.go`, `cmd/server/review_handlers.go`, `cmd/server/review_handlers_test.go`, `.moai/db/schema/migrations/0005_score_review_request_tables.sql`
- **수정 허용(범위 한정)**:
  - `internal/store/store.go`: `ScoreReviewRequestStore`/`ScoreReviewRequestTx` 인터페이스 + `ScoreReviewRequest` struct 추가만(기존 인터페이스 무변경)
  - `internal/store/pg_store.go`: `BeginScoreReviewRequestTx` 메서드 추가만(기존 메서드 무변경)
  - `internal/audit/audit.go`: 액션 상수 4개 추가만(기존 상수 무변경)
  - `internal/audit/recorder.go`: `RecordScoreReviewRequest*` 메서드 4개 추가만(기존 메서드 무변경)
  - `internal/errors/errors.go`: 센티넬 6개 추가만(기존 센티넬 무변경 — SCORE-API-001 errors.go drift 교훈 적용, manifest에 명시 부착)
  - `cmd/server/server.go`: `reviewH` 필드 1줄 + 생성자 1줄 + `innerMux.Handle` 2줄 + ko 주석 ≈7줄(SCORE-API-001/REPORT-001 정확 미러 — `server.go:55/210/266-267` 선례; "1줄"이 아닌 ≈7줄 최소 단위)
- **수정 절대 금지(0 diff 검증)**: `internal/store/score.go`(SCORE-001 PgScoreTx), `internal/store/eval_item.go`, `internal/store/evidence.go`, `cmd/server/score_handlers.go`(SCORE-API-001), `cmd/server/report_handlers.go`(REPORT-001), `cmd/server/evidence_handlers.go`, `internal/auth/**/*.go`(frozen RBAC/ABAC), `.moai/db/schema/migrations/0001`–`0004`, `apps/control-plane/go.mod`/`go.sum`

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건) — REQ-REVIEW-UBI-NNN dual-track

Ubiquitous 요구사항은 SPEC-AX-SCORE-001 / SCORE-API-001 / REPORT-001의 canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track 패턴을 도메인 스코프 형태(`REQ-REVIEW-UBI-NNN`)로 적용한다(research.md §10).

- **REQ-REVIEW-UBI-001 (데이터 주권)**: The score review request layer (store + HTTP API) SHALL NOT make any external service call (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) on any request path (생성/조회/목록/검토자할당/승인/반려). 모든 영속·검증·상태 전이는 단일 내부 PostgreSQL pgx pool(`PgWorkflowStore.pool` 재사용, `pg_store.go:134-148` `BeginScoreTx` 동형)에만 위임한다(`tech.md` 망분리 정합). 신규 외부 의존을 도입하지 않는다.
- **REQ-REVIEW-UBI-002 (감사 가능성 — 동일-TX entity+audit 원자성)**: For every mutation operation (`InsertScoreReviewRequest` 생성 / `AssignReviewer` 할당 / `ApproveRequest` 승인 / `RejectRequest` 반려), the store layer SHALL write exactly one `audit_logs` row (`ActionScoreReviewRequest*`) within the same `pgx.Tx` as the entity change, AND the `audit_logs.resource_id` SHALL equal the `score_review_requests.id` UUID directly (SCORE-001 D2 미러 — surrogate 미사용, namespace 상수 0). audit 실패 시 호출자가 Rollback하면 entity-INSERT와 audit-INSERT 양쪽이 취소되어야 한다(양방향 원자성). HTTP API 핸들러는 별도 audit row를 INSERT하지 않는다(이중 감사 금지).
- **REQ-REVIEW-UBI-003 (cli-anonymous 기본값 + auth-disabled fallback)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the API layer SHALL serve all endpoints with ABAC/RBAC middleware transparently passing through (`abac.go:8` REQ-ABAC-009 정합), AND the resulting `score_review_requests` rows SHALL carry `created_by`/`updated_by` = `'cli-anonymous'` literal AND the corresponding `audit_logs.user_id` SHALL also equal `'cli-anonymous'` literal (NULL 금지, SCORE-001 §1.1 정합, `pg_store.go:146` `audit.NewRecorder(false)` 패턴 동형).
- **REQ-REVIEW-UBI-004 (권한 narrowing + 4-상태 불변식)**: The system SHALL enforce two unbreakable invariants simultaneously: (1) **ABAC narrowing-only**: write 권한 미보유 principal의 mutation 시도는 HTTP 403 `ErrCodeABACDenied`(`abac.go:24`)로 거부하며 핸들러-로컬 게이트(`requireReviewSubmitRole`/`requireReviewAdminRole`)가 frozen `rbac.go` 무수정으로 매핑한다(analyst=제출 / admin=검토자할당·승인·반려 / viewer+모든 인증=조회), (2) **4-상태 allowed transitions only**: store-level `validateReviewStatusTransition` 가드가 `SUBMITTED → UNDER_REVIEW` / `UNDER_REVIEW → APPROVED` / `UNDER_REVIEW → REJECTED`만 허용하고 그 외 모든 전이(예: `APPROVED → SUBMITTED`, `REJECTED → UNDER_REVIEW`, `SUBMITTED → APPROVED` 등)는 SQL 미실행 후 `ErrScoreReviewRequestInvalidStatus`(HTTP 409)로 거부한다. `APPROVED`/`REJECTED`는 terminal 상태이며 되돌리기 불가(SCORE-001 D4 state-machine 동형). 정정 경로는 신규 검토 요청 생성(`SUBMITTED` 신규 row INSERT)만 — 물리 DELETE 0건.

### 3.2 REQ-REVIEW-001 — Store + 데이터 모델 + 마이그레이션

본 모듈은 `score_review_requests` 테이블 신설과 store TX 진입점을 정의한다.

- **REQ-REVIEW-001-E1 (Event-Driven, entity+audit 원자 생성)**: WHEN a caller invokes `ScoreReviewRequestTx.InsertScoreReviewRequest(ctx, scoreID, comment, metadata)` with valid input within an open `pgx.Tx`, THEN the store SHALL INSERT one row into `score_review_requests` with `status = 'SUBMITTED'` AND SHALL write exactly one `audit_logs` row with `action = 'SCORE_REVIEW_REQUEST_CREATED'` AND `resource_id = score_review_requests.id` UUID directly (D2 미러) within the same transaction, returning the generated UUID.
- **REQ-REVIEW-001-S1 (State-Driven, score_id FK-less stub)**: IF `score_id` is provided in any request payload, THEN the store SHALL accept it as `UUID NOT NULL` without enforcing a foreign-key constraint to `scores.id` (FK-less stub — SCORE-001 §1.4 동형 계약), AND the existence of the referenced score SHALL be validated by the HTTP handler via `ScoreStore.BeginScoreTx` → `ScoreTx.GetScoreByID` in a separate transaction (cross-store 2-TX 조합, REPORT-001 §6 OPEN #1 Option A 선례 미러; 형태 세부는 본 SPEC §6 OPEN #3).
- **REQ-REVIEW-001-U1 (Unwanted, blank/invalid 입력)**: IF `score_id` is the zero UUID OR `comment` exceeds the configured maximum length (TBD §6 OPEN #5) OR `metadata` is not a JSONB-serializable map, THEN the store SHALL reject the input before SQL execution by returning `ErrScoreReviewRequestInvalidInput` AND SHALL NOT INSERT any row into `score_review_requests` or `audit_logs`.
- **REQ-REVIEW-001-O1 (Optional, metadata opaque placeholder)**: WHERE the client provides a `metadata` JSONB object in any request body, the store SHALL persist it verbatim into `score_review_requests.metadata` AND SHALL NOT interpret, validate, or transform its contents (opaque placeholder — SCORE-001 `Score.Metadata` 동형). 본 SPEC은 metadata 스키마를 정의하지 않는다.

### 3.3 REQ-REVIEW-002 — HTTP API 생성/조회/목록 (read+create endpoints)

본 모듈은 3개의 기본 엔드포인트(`POST /api/v1/reviews` 생성 / `GET /api/v1/reviews/{id}` 단건 조회 / `GET /api/v1/reviews` 목록)를 정의한다.

- **REQ-REVIEW-002-E1 (Event-Driven, 생성 엔드포인트)**: WHEN the HTTP API receives `POST /api/v1/reviews` with a JSON body containing `{score_id, comment?, metadata?}` AND the principal passes the ABAC `requireReviewSubmitRole` gate (analyst OR admin OR auth-disabled), THEN the handler SHALL (a) validate the referenced score exists via `ScoreTx.GetScoreByID` returning 404 if absent, (b) invoke `ScoreReviewRequestTx.InsertScoreReviewRequest` within a new TX, (c) commit the TX, AND (d) respond with HTTP 201 Created + `{id, score_id, status, created_at, created_by}` JSON body. 핸들러는 자체 audit row를 INSERT하지 않는다(store-only audit, UBI-002 정합).
- **REQ-REVIEW-002-E2 (Event-Driven, 단건 조회)**: WHEN the HTTP API receives `GET /api/v1/reviews/{id}` with a valid UUID path param AND the principal passes basic authentication narrowing (any authenticated user including viewer OR auth-disabled), THEN the handler SHALL respond with HTTP 200 OK + the full review request entity JSON if found, OR HTTP 404 NOT_FOUND with `{"error":{"code":"NOT_FOUND","message":"요청한 평가 검토를 찾을 수 없습니다"}}` if absent.
- **REQ-REVIEW-002-E3 (Event-Driven, 목록)**: WHEN the HTTP API receives `GET /api/v1/reviews?status=&score_id=&limit=&offset=` with optional filter and pagination query params, THEN the handler SHALL respond with HTTP 200 OK + `{items: [...], total: N}` JSON body, applying `clampPagination`(SCORE-API-001 `score_handlers.go:144-157` 동형 — `limit` 기본 50, 최대 500, `offset` 음수 → 0) AND ordering rows by `created_at DESC`. 결과 0건은 빈 배열(error 아님).
- **REQ-REVIEW-002-U1 (Unwanted, malformed UUID)**: IF the path param `{id}` is not a valid UUID v4 string, THEN the handler SHALL respond with HTTP 400 BAD_REQUEST + `{"error":{"code":"INVALID_ARGUMENT","message":"유효하지 않은 평가 검토 ID 형식입니다","field":"id"}}` without invoking the store layer.

### 3.4 REQ-REVIEW-003 — HTTP API 상태 전이 (assign/approve/reject endpoints)

본 모듈은 3개의 상태 전이 엔드포인트를 정의한다. 정확한 엔드포인트 shape(별도 sub-resource vs PUT /status)는 §6 OPEN #1·#2 strategy phase 결정.

- **REQ-REVIEW-003-E1 (Event-Driven, 검토자 할당 — SUBMITTED → UNDER_REVIEW)**: WHEN the HTTP API receives the assign-reviewer endpoint (shape §6 OPEN #1) for review request `{id}` with body containing `{reviewer_id}` AND the principal passes the `requireReviewAdminRole` gate (admin only OR auth-disabled), THEN the store SHALL (a) acquire a row lock via `SELECT ... FOR UPDATE` for concurrent transition safety (SCORE-001 EVID-001 패턴, §6 OPEN #6), (b) verify current `status = 'SUBMITTED'` returning `ErrScoreReviewRequestNotSubmitted` (HTTP 409) otherwise, (c) update `status = 'UNDER_REVIEW'` AND `assigned_reviewer_id = reviewer_id` AND `updated_at = now()` AND `updated_by = principal.id (or 'cli-anonymous')`, (d) write one `audit_logs` row with `action = 'SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED'` within the same TX, AND (e) respond with HTTP 200 OK + updated entity.
- **REQ-REVIEW-003-E2 (Event-Driven, 승인 — UNDER_REVIEW → APPROVED terminal)**: WHEN the HTTP API receives the approve endpoint (shape §6 OPEN #2) for review request `{id}` with optional body `{comment?}` AND the principal passes the `requireReviewAdminRole` gate (admin only OR auth-disabled), THEN the store SHALL (a) acquire `SELECT ... FOR UPDATE` row lock, (b) verify current `status = 'UNDER_REVIEW'` returning `ErrScoreReviewRequestNotUnderReview` (HTTP 409) otherwise, (c) update `status = 'APPROVED'` AND `comment` (if provided) AND `updated_at/updated_by`, (d) write `audit_logs` row with `action = 'SCORE_REVIEW_REQUEST_APPROVED'` within the same TX, AND (e) respond with HTTP 200 OK + updated entity. `APPROVED`는 terminal(되돌리기 금지, UBI-004).
- **REQ-REVIEW-003-E3 (Event-Driven, 반려 — UNDER_REVIEW → REJECTED terminal)**: WHEN the HTTP API receives the reject endpoint (shape §6 OPEN #2) for review request `{id}` with body `{rejection_reason}` AND the principal passes the `requireReviewAdminRole` gate, THEN the store SHALL (a) acquire row lock, (b) verify current `status = 'UNDER_REVIEW'` returning `ErrScoreReviewRequestNotUnderReview` (HTTP 409) otherwise, (c) verify `rejection_reason` is non-empty returning `ErrScoreReviewRequestInvalidInput` (HTTP 400) otherwise (§6 OPEN #5 - CHECK constraint vs pre-store validation 결정), (d) update `status = 'REJECTED'` AND `rejection_reason` AND `updated_at/updated_by`, (e) write `audit_logs` row with `action = 'SCORE_REVIEW_REQUEST_REJECTED'` within the same TX, AND (f) respond with HTTP 200 OK + updated entity. `REJECTED`는 terminal(UBI-004).
- **REQ-REVIEW-003-S1 (State-Driven, 불법 전이 거부)**: IF any state transition request targets a non-allowed transition (e.g., `APPROVED → SUBMITTED`, `REJECTED → UNDER_REVIEW`, `SUBMITTED → APPROVED` 직접 등) OR targets a row already in terminal state (`APPROVED`/`REJECTED`), THEN the store-level `validateReviewStatusTransition` SHALL reject before SQL execution by returning `ErrScoreReviewRequestInvalidStatus` (HTTP 409) AND SHALL NOT INSERT or UPDATE any row in `score_review_requests` or `audit_logs`.

### 3.5 REQ-REVIEW-004 — Store-Level 감사 연계 (Recorder + audit constants)

- **REQ-REVIEW-004-E1 (Event-Driven, RecordScoreReviewRequest*)**: WHEN any of `InsertScoreReviewRequest` / `AssignReviewer` / `ApproveRequest` / `RejectRequest` is invoked within an open `pgx.Tx`, THEN the store SHALL invoke the corresponding `recorder.RecordScoreReviewRequestCreated`/`...ReviewerAssigned`/`...Approved`/`...Rejected` (SCORE-001 `RecordScoreCreated`/`Updated` 동형) which constructs an `audit.Event` with `Action = ActionScoreReviewRequest{Created|ReviewerAssigned|Approved|Rejected}`, `ResourceID = score_review_requests.id` UUID directly (D2 — namespace 상수 0), `UserID = principal.id` or `'cli-anonymous'`, AND writes it to `audit_logs` via the same `tx` (local AuditTx 패턴, `recorder.go` SCORE-001 RecordScore* 영역 동형).
- **REQ-REVIEW-004-U1 (Unwanted, audit write 실패 → 양방향 rollback)**: IF the `audit_logs` INSERT fails within `RecordScoreReviewRequest*`, THEN the store method SHALL return `ErrScoreReviewRequestAuditWriteFailed` wrapping the underlying error AND the caller SHALL invoke `tx.Rollback(ctx)` causing both the `score_review_requests` row mutation and any partial `audit_logs` row to be canceled (양방향 원자성, SCORE-001 `ErrScoreAuditWriteFailed` `errors.go:71-74` 동형, DC-004-U1 미러).

### 3.6 REQ-REVIEW-005 — 에러 매핑 + 표준 응답

- **REQ-REVIEW-005-E1 (Event-Driven, 결정적 store→HTTP 매핑)**: WHEN the handler invokes any store method AND the store returns a sentinel error, THEN the handler's `mapReviewStoreErr`(SCORE-API-001 `mapStoreErr` `score_handlers.go:111-129` 동형) SHALL map deterministically: `ErrScoreReviewRequestNotFound` → 404 `NOT_FOUND` / `ErrScoreReviewRequestInvalidInput` → 400 `INVALID_ARGUMENT` / `ErrScoreReviewRequestInvalidStatus` → 409 `CONFLICT` / `ErrScoreReviewRequestNotSubmitted` → 409 `CONFLICT` / `ErrScoreReviewRequestNotUnderReview` → 409 `CONFLICT` / `ErrScoreReviewRequestAuditWriteFailed` → 500 `INTERNAL` (with ERROR log) / unknown → 500 `INTERNAL` (fail-safe). 모든 에러 본문은 한국어 메시지(`score_handlers.go:114-127` 동형). 클라이언트 오류(400/403/404/409)는 INFO 로그, 서버 결함(500)은 ERROR 로그.
- **REQ-REVIEW-005-U1 (Unwanted, raw pgx 에러 누출 금지)**: IF the store layer encounters `pgx.ErrNoRows` or other raw pgx-level errors, THEN it SHALL wrap and return the appropriate sentinel(`ErrScoreReviewRequestNotFound` for ErrNoRows) — never returning raw `pgx.ErrNoRows` (GAP-03 패턴 정합, EVID-001/SCORE-001 동형).

---

## 4. 상수·계약 인용 검증

| 항목 | 정의 | 검증 |
|------|------|------|
| Allowed 상태 enum | `SUBMITTED` / `UNDER_REVIEW` / `APPROVED` / `REJECTED` | research.md §3.1, 본 SPEC §3 |
| Allowed transitions | `SUBMITTED→UNDER_REVIEW`, `UNDER_REVIEW→APPROVED`, `UNDER_REVIEW→REJECTED`만 | research.md §3.1/§8.1, 본 SPEC REQ-REVIEW-UBI-004 |
| Terminal states | `APPROVED`, `REJECTED` (되돌리기 금지) | research.md §3.1/§10.2 |
| RBAC 역할 (frozen) | `RoleAdmin`/`RoleAnalyst`/`RoleViewer` (`rbac.go:19-26`) — `evaluator`/`reviewer` 신설 금지 | source-verified |
| Scope 정규식 (frozen) | `^iroum-ax:(admin|analyst|viewer)$` (`rbac.go:33`) | source-verified |
| Cross-store 점수 검증 TX 진입점 | `ScoreStore.BeginScoreTx` (`store.go:253-255`, `pg_store.go:134-148`) | source-verified |
| 동일 TX 진입점 (신규) | `ScoreReviewRequestStore.BeginScoreReviewRequestTx` (Recorder 주입 `pg_store.go:143-147` 동형) | research.md §1.2 / 본 SPEC §2.1 |
| Audit resource_id 규칙 | `score_review_requests.id` UUID 직접 (D2 미러, namespace 상수 0) | research.md §7.1 |
| FK-less stub | `score_review_requests.score_id` UUID NOT NULL, FK 제약 없음 (SCORE-001 §1.4 동형) | research.md §2.2 |
| 신규 마이그레이션 | 0005 (0001/0002/0003/0004 디스크 확인 후 비충돌) | research.md §4.1 |
| 멱등 패턴 | `CREATE TABLE IF NOT EXISTS` + `DO$$ EXCEPTION duplicate_object` + `CREATE INDEX IF NOT EXISTS` (0004 선례) | research.md §4.2 |
| server.go 마운트 줄 수 | ≈7줄 (필드+생성자+innerMux.Handle 2줄 — SCORE-API-001 `:55/:210/:266-267`, REPORT-001 `:212/:269-270` 정확 미러) | source-verified |

---

## 5. Exclusions (What NOT to Build)

본 SPEC이 의도적으로 제외하는 범위는 다음과 같다. 제외 항목은 본 SPEC 산출물(코드/SQL/감사/엔드포인트)에 포함되지 않으며 향후 별도 SPEC에서 다룬다.

1. **SCORE-001 / EVAL-ITEM-001 / EVID-001 / AUTH-003 코드·스키마 수정**: 점수 store, 점수 핸들러, 평가항목 store, 증빙 store, RBAC permissionMatrix, ABAC 정책 모두 0-diff. 본 SPEC은 consumer-only [HARD].
2. **`score_id` → `scores.id` 외래키 강제**: FK-less stub 유지(SCORE-001 §1.4 동형). 참조 무결성은 핸들러 단계 cross-store 조회만(REQ-REVIEW-001-S1).
3. **요청당 다중 검토자 (multi-reviewer per request)**: PoC 범위는 검토자 1명. `assigned_reviewer_id`는 단일 VARCHAR. 다중 검토자/투표/합의 워크플로우는 post-PoC.
4. **에스컬레이션 / 타임아웃 / SLA 워크플로우**: SUBMITTED 후 N일 자동 에스컬레이션, 검토 시한 알람, SLA 위반 표시 등은 post-PoC.
5. **알림 / 이메일 / 메신저 통합**: 검토자 할당/승인/반려 시 외부 알림 발송 0건(외부 호출 0 — REQ-REVIEW-UBI-001 정합). 알림은 post-PoC 별도 SPEC.
6. **AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC**: 검토자가 특정 조직단위에 속해야 한다는 추가 narrowing 미구현(AUTH-003 `User`/`Scopes` 모델만 사용). 풀 org-unit ABAC은 post-PoC.
7. **검토 이력의 시계열 조회 / 변경 감사 UI**: 본 SPEC은 `audit_logs`에 SCORE_REVIEW_REQUEST_* 액션을 기록하지만 그 조회 API(GET /api/v1/audit/reviews 등)는 제공하지 않는다. 감사 조회는 별도 audit-query SPEC.
8. **물리 삭제 / hard delete**: REJECTED 후에도 행을 삭제하지 않는다(append-only, UBI-004 정합). 정정 경로는 신규 검토 요청 SUBMITTED INSERT만.
9. **6번째 한국 공공 제약(시간 제약 — KST 업무시간 한정 승인)**: 의도적 제외(본 SPEC은 데이터 주권·감사·언어·망분리·조직 격리만 적용, SCORE-API-001 / REPORT-001 동형). 시간 제약은 post-PoC.
10. **LLM 기반 자동 검토 / 추천 엔진**: 외부 호출 0 + 결정적 처리만(UBI-001 정합). LLM 자동화는 post-PoC.

---

## 6. RESOLVED Decisions — Human Gate sign-off 2026-05-20

다음 6건은 Run phase strategy phase에서 결정되었으며, **2026-05-20 Human Gate(ircp) 권고안 그대로 6/6 승인 완료**. SSOT는 `.moai/specs/SPEC-AX-REVIEW-001/strategy.md` §A.1~§A.6 (5요소: 결정/근거/거부대안/consumer-only/Run phase 적용). 본 §6은 결정 요약 + strategy.md cross-reference만 유지한다.

### 6.1 검토자 할당 엔드포인트 형태 [RESOLVED]

- **Decision**: Option A — `POST /api/v1/reviews/{id}/assign-reviewer` (별도 sub-resource), body `{"reviewer_id": "<string>"}`, status `SUBMITTED→UNDER_REVIEW` + audit 1건
- **근거**: SCORE-API-001 supersede `score_handlers.go:64` 동형, REST 비-멱등 sub-resource 패턴, research.md §12.3 #1
- **거부 대안**: B (`PUT /reviews/{id}` body status) — PUT 멱등 의미 충돌 / C (`/transition` 통합) — 핸들러 분기 복잡
- **SSOT**: strategy.md §A.1

### 6.2 승인/반려 엔드포인트 형태 [RESOLVED]

- **Decision**: Option A — `POST /api/v1/reviews/{id}/approve` (body `{comment?}`) + `POST /api/v1/reviews/{id}/reject` (body `{rejection_reason* required, comment?}`), 각각 별도 sub-resource, terminal 전이
- **근거**: §6.1 결정 일관성, SCORE-API-001 supersede 선례, audit routing 분리 (`ActionScoreReviewRequest{Approved|Rejected}`)
- **거부 대안**: B (`PUT /status` 단일) — PUT 멱등 충돌, body 분기 복잡 / C (`/decision` 통합) — 분리 이점 손실
- **SSOT**: strategy.md §A.2

### 6.3 Cross-store 점수 존재 검증 [RESOLVED]

- **Decision**: Option A — handler-compose 2-TX. TX-1(read-only) `scoreStore.BeginScoreTx`→`GetScoreByID`→Rollback, score 미존재 시 HTTP 404. TX-2(write) `reviewStore.BeginScoreReviewRequestTx`→`Insert+audit`→Commit
- **근거**: REPORT-001 §6 OPEN#1 선례 정확 미러, `store.go:253-255/275-276` 호출만, SCORE-001 0-diff. Race window는 SCORE-001 물리 삭제 0(append-only)이라 결정적 不發生 — PoC 수용 trade-off
- **거부 대안**: B (store-layer 단일 TX 통합) **INFEASIBLE** — cross-store coupling = consumer-only [HARD] 위반 / D (distributed lock) — REQ-REVIEW-UBI-001 데이터 주권 위반
- **SSOT**: strategy.md §A.3

### 6.4 검토자 역할 매핑 [RESOLVED]

- **Decision**: Option A — admin-only for approve/reject/assign-reviewer. 매핑: 제출={RoleAnalyst, RoleAdmin}, 승인/반려/할당=RoleAdmin only, 조회=모든 인증 사용자(viewer 포함). `assigned_reviewer_id` 컬럼은 정보/감사 추적용(ABAC 결정과 무관)
- **근거**: rbac.go L19-26 + L33 frozen 3-role + score_handlers.go L161-190/L163 evaluator-INFEASIBLE 주석 source-verified (2026-05-20). 본 SPEC 매핑(analyst/admin/viewer)이 frozen rbac.go에 모두 존재 → SCORE-API-001 OPEN#4 evaluator 충돌 자연 회피
- **거부 대안**: B (`assigned_reviewer_id == principal.id` ABAC narrowing) — abac.go 수정 + 위임 시나리오 차단 / C (`RoleReviewer` 신설) **INFEASIBLE** — rbac.go:33 정규식 부재, frozen [HARD] 위반 / D (`permissionMatrix`에 `write:review` 추가) — AUTH-003 frozen 위반
- **PoC trade-off**: "모든 admin이 모든 검토 승인 가능 — 역할 분리 X" (post-PoC 풀 org-unit ABAC 도입 시 재검토, §5 Exclusion #6)
- **SSOT**: strategy.md §A.4

### 6.5 Reason/Comment 필드 작성 조건 [RESOLVED]

- **Decision**: Option B + A 이중 방어 — (Layer 1) pre-store handler validation `validateReviewRequestInput` (REJECTED 시 `rejection_reason` non-empty 검증, 400 INVALID_ARGUMENT + 한국어 "반려 사유는 필수입니다", SCORE-001 `validateScoreInput` `score.go:79-97` 동형 fail-closed) + (Layer 2) DB CHECK constraint `score_review_requests_reject_reason_chk` (멱등 DO$$ EXCEPTION duplicate_object, 0004 미러). `comment`는 모든 상태에서 NULL 허용 (opaque optional)
- **근거**: EVAL-ITEM-001 이중 방어 선례 동형 + 한국어 친화 에러(UX) + fail-safe 최후 방어, plan-audit.md §3.5 OPEN#5 SOUND
- **거부 대안**: A 단독 (CHECK만) — 친화 에러 손실, 400 vs 500 결정성 약화 / B 단독 (handler validation만) — DB 보호 약화 / C (`comment`도 reject 시 필수) — over-spec
- **SSOT**: strategy.md §A.5

### 6.6 Concurrent Transition Handling [RESOLVED]

- **Decision**: Option A — `SELECT ... FOR UPDATE` pessimistic row lock + state-machine 가드 이중 방어. assign/approve/reject 3 메서드 모두 동일 패턴: (1) `SELECT status FROM score_review_requests WHERE id=$1 FOR UPDATE` (TX scoped) → (2) `validateReviewStatusTransition(current, target)` → (3) UPDATE + `RecordScoreReviewRequest*` (동일-TX). Lock은 Commit/Rollback 시 자동 해제. `version` 컬럼 추가 0
- **근거**: EVID-001 `GetLatestVersionByEvalItem` `store.go:92-95` 패턴 + SCORE-001 `validateScoreStatusTransition` 동형 + PostgreSQL row-level lock 결정적 직렬화, plan-audit.md §3.5 OPEN#6 SOUND
- **거부 대안**: B (optimistic version + CAS) — 스키마+코드 복잡↑, PoC over-engineering / C (lock 없음, state-machine 가드만) — TOCTOU race, 결정성 0 / D (distributed Redis lock) — REQ-REVIEW-UBI-001 데이터 주권 위반
- **SSOT**: strategy.md §A.6

> 본 6건 RESOLVED 결정은 SCORE-API-001 §6 OPEN #4(write 역할 매핑 충돌, `evaluator` 부재)를 **상속하지 않는다** — 본 SPEC의 역할(analyst/admin)은 frozen `rbac.go`에 모두 존재(`rbac.go:19-26`)하므로 0-diff 매핑이 자연 성립. SCORE-API-001 OPEN #4 같은 frozen RBAC 충돌은 발생하지 않는다. **Sign-off**: ircp (2026-05-20), 6/6 권고안 그대로 승인.

---

## 7. Edge Cases (acceptance.md §3에서 세부)

acceptance.md §3에서 16개 edge case를 정의한다. 요약:

1. 불법 상태 전이 시도(예: `APPROVED → SUBMITTED`) → 409 `ErrScoreReviewRequestInvalidStatus`
2. 존재하지 않는 `score_id` → 404 `NOT_FOUND` (cross-store handler-compose 단계)
3. blank/zero `score_id` → 400 `INVALID_ARGUMENT`
4. ABAC viewer가 제출 시도 → 403 `ABAC_CONDITION_DENIED`
5. ABAC analyst가 승인 시도 → 403 `ABAC_CONDITION_DENIED`
6. ABAC analyst가 검토자 할당 시도 → 403
7. auth-disabled 모드 모든 엔드포인트 투과 + `cli-anonymous` 기록
8. 빈 목록(필터 결과 0건) → 200 + `{"items":[],"total":0}`
9. `REJECTED` 전이 시 `rejection_reason` NULL/empty → 400 `INVALID_ARGUMENT`
10. 동시 admin 승인 race(`SELECT FOR UPDATE`) → 첫 번째 200, 두 번째 409 `ErrScoreReviewRequestNotUnderReview`
11. 단건 조회 malformed UUID → 400
12. 목록 pagination `limit > maxListLimit` → 500으로 clamp
13. 목록 pagination `offset` 음수 → 0으로 clamp
14. 검토자 할당 대상이 `UNDER_REVIEW`/`APPROVED`/`REJECTED`(이미 전이) → 409 `ErrScoreReviewRequestNotSubmitted`
15. 승인/반려 대상이 `SUBMITTED`(아직 할당 안됨) → 409 `ErrScoreReviewRequestNotUnderReview`
16. `RecordScoreReviewRequest*` audit INSERT 실패 → `ErrScoreReviewRequestAuditWriteFailed` + 호출자 Rollback → entity+audit 양방향 취소

---

## 8. 참조

- `apps/control-plane/internal/store/store.go` (ScoreStore/ScoreTx `:253-305`, EvalItemStore 패턴 `:115-119` 미러 대상)
- `apps/control-plane/internal/store/pg_store.go` (`BeginScoreTx` `:134-148` Recorder 주입 동형 선례)
- `apps/control-plane/internal/store/score.go` (validateScoreStatusTransition / 멱등 패턴 미러 대상)
- `apps/control-plane/cmd/server/score_handlers.go` (핸들러 구조·Routes·ABAC 동형 선례 `:43-51/:59-70/:74-101/:111-129/:161-190`)
- `apps/control-plane/internal/auth/rbac.go` (frozen 3-role `:19-26/:33/:68-80` — 0-diff)
- `apps/control-plane/internal/auth/abac.go` (narrowing-only/admin 우회/auth-disabled 투과)
- `apps/control-plane/internal/errors/errors.go` (센티넬 패턴 `:52-78` 동형 추가 대상)
- `apps/control-plane/cmd/server/server.go` (`scoreH` 필드 `:55-56`, 생성 `:210/:212`, 마운트 `:266-267/:269-270` 정확 미러 대상)
- `.moai/db/schema/migrations/0004_score_tables.sql` (멱등 패턴 미러 대상)
- `.moai/specs/SPEC-AX-REVIEW-001/research.md` (665줄 SSOT)
- `.moai/specs/SPEC-AX-SCORE-001/spec.md` (store+audit 패턴 선례)
- `.moai/specs/SPEC-AX-SCORE-API-001/spec.md` (HTTP API + ABAC 패턴 선례, OPEN #4 lesson)
- `.moai/specs/SPEC-AX-REPORT-001/spec.md` (consumer-only + ≈7줄 server.go + cross-store 2-TX 정확성 선례)

---

> SSOT: research.md(665줄, file:line 근거). 본 spec.md의 모든 기술적 주장은 research.md 또는 source 파일 직접 확인을 통해 검증됨(phantom 0건).
