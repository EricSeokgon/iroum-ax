# Plan: SPEC-AX-REVIEW-001 평가 제출/승인 워크플로우 저장소 + HTTP API 계층

**SPEC**: SPEC-AX-REVIEW-001 v0.1.0 (draft)
**Phase**: Plan
**Generated**: 2026-05-20
**Author**: ircp
**Methodology**: TDD (RED-GREEN-REFACTOR)
**Harness Level**: thorough
**Mode**: sub-agent
**Status**: draft

---

## 1. 구현 접근 (Implementation Approach)

본 SPEC은 SPEC-AX-SCORE-001(store-only) + SPEC-AX-SCORE-API-001(API-only) 두 SPEC의 패턴을 **단일 수직 슬라이스**로 결합한다. 사용자 인터뷰(2026-05-20)로 확정된 4-상태 워크플로우 범위가 작고 명확하므로 store↔API를 분리하지 않는다.

### 1.1 핵심 전략

1. **선례 정확 미러**: SCORE-001(store+audit) + SCORE-API-001(HTTP + ABAC) + REPORT-001(server.go ≈7줄, errors.go drift 부착 정확성) 패턴 정확 재사용 — 신규 발명 0건
2. **Consumer-only [HARD]**: SCORE-001/EVAL-ITEM-001/EVID-001/AUTH-003 코드·스키마·FK·permissionMatrix 0-diff
3. **단일 신규 마이그레이션 0005**: 0001/0002/0003/0004 디스크 확인 후 비충돌, 멱등 패턴(0004 미러)
4. **frozen RBAC 0-diff**: `RoleAdmin`/`RoleAnalyst`/`RoleViewer` 3역할만 사용, `RoleReviewer`/`evaluator` 신설 금지
5. **핸들러-로컬 ABAC narrowing**: SCORE-API-001 `requireScoreWriteRole`/`guardScoreWrite` 동형 패턴 `requireReviewSubmitRole`/`requireReviewAdminRole`
6. **TDD RED-first**: 모든 신규 store 메서드 + 핸들러 메서드에 대해 failing test 우선 작성

### 1.2 phantom 회피 (lesson #9)

`BeginScoreReviewRequestTx`/`PgScoreReviewRequestTx`/`ScoreReviewRequestStore`는 본 SPEC에서 **신설**되는 것이므로 phantom이 아니며, 다음의 source-verified 기존 진입점만 호출한다:
- `ScoreStore.BeginScoreTx` (`store.go:253-255`, `pg_store.go:134-148` — cross-store 점수 검증용)
- `ScoreTx.GetScoreByID` (`store.go:275-276`)
- `audit.NewRecorder(false)` (`pg_store.go:146`)
- `auth.UserFromContext`, `auth.ParseRolesFromScope`, `auth.RoleAdmin`/`auth.RoleAnalyst` (rbac.go/middleware.go)
- `auth.ErrCodeABACDenied` (`abac.go:24`)
- `apperrors.Err*` 센티넬 (`errors.go:52-78`)

신설되는 메서드/구조체는 본 SPEC의 [NEW]/[MODIFY] 선언에 모두 명시되어 있다(spec.md §2.1).

---

## 2. 마일스톤 (Priority-Based, 시간 추정 금지)

### Milestone M0 (Priority High) — RED Phase: 신규 테스트 골격 + 인터페이스 정의

**산출물**:
- `internal/store/store.go` MODIFY: `ScoreReviewRequestStore`/`ScoreReviewRequestTx` 인터페이스 + `ScoreReviewRequest` struct 정의
- `internal/store/score_review_request.go` NEW (skeleton): 컴파일만 통과하는 빈 구현
- `cmd/server/review_handlers.go` NEW (skeleton): 컴파일만 통과하는 빈 핸들러
- `cmd/server/review_handlers_test.go` NEW: 16+ failing test 작성(생명주기 전이/ABAC/edge case/audit 원자성)
- `internal/store/score_review_request_test.go` NEW: store-layer failing test

**완료 기준**: 모든 신규 테스트가 RED(실패)로 확인됨. `go vet ./...` 통과. `go test ./...` 신규 테스트만 실패.

**의존**: 없음

### Milestone M1 (Priority High) — GREEN Phase: 0005 마이그레이션 + store 구현

**산출물**:
- `.moai/db/schema/migrations/0005_score_review_request_tables.sql` NEW: 멱등 SQL(CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION + CREATE INDEX IF NOT EXISTS) — 0004 정확 미러
- `internal/store/pg_store.go` MODIFY: `BeginScoreReviewRequestTx` 메서드(Recorder 주입 `pg_store.go:143-147` 동형)
- `internal/store/score_review_request.go` 완성: `PgScoreReviewRequestTx` 7 메서드(Insert/Get/List/UpdateStatus/AssignReviewer/Approve/Reject) + validation 함수 2개(validateReviewRequestInput / validateReviewStatusTransition) + InsertAuditLog/Commit/Rollback
- `internal/audit/audit.go` MODIFY: Action 상수 4개 추가
- `internal/audit/recorder.go` MODIFY: `RecordScoreReviewRequest*` 메서드 4개
- `internal/errors/errors.go` MODIFY: 센티넬 6개 추가

**완료 기준**: M0 store-layer 테스트 GREEN. 마이그레이션 정방향 적용 + 재실행 멱등성 확인.

**의존**: M0

### Milestone M2 (Priority High) — GREEN Phase: HTTP API 핸들러 + ABAC

**산출물**:
- `cmd/server/review_handlers.go` 완성: `ReviewHandler` + `NewReviewHandler` + `Routes()`(ServeMux Go1.22+ 최장일치, 구체 경로 먼저) + 6 핸들러 메서드 + `writeReviewJSON`/`writeReviewErr`/`mapReviewStoreErr` + `requireReviewSubmitRole`/`requireReviewAdminRole`/`guardReviewSubmit`/`guardReviewAdmin` + `clampPagination` 재사용 또는 동형 구현
- 핸들러는 cross-store 점수 검증을 위해 `ScoreStore`도 의존(생성자에서 주입)

**완료 기준**: M0 핸들러 단위 테스트 GREEN(16+ test). ABAC narrowing 모든 경계 통과(viewer create 403 / analyst approve 403 / admin approve 200 / auth-disabled 투과).

**의존**: M1

### Milestone M3 (Priority Medium) — GREEN Phase: server.go 마운트 + 통합

**산출물**:
- `cmd/server/server.go` MODIFY: `reviewH *ReviewHandler` 필드 + `NewReviewHandler(pgStore, pgStore, logger)` 생성 + `innerMux.Handle("/api/v1/reviews", s.reviewH.Routes())` + `innerMux.Handle("/api/v1/reviews/", s.reviewH.Routes())` 2줄 + ko 주석. **정확히 ≈7줄**(SCORE-API-001 `:55/:210/:266-267`, REPORT-001 `:56/:212/:269-270` 미러).

**완료 기준**: `go build ./...` 통과. 헬스체크 정상. `/api/v1/reviews` 라우트 reachable.

**의존**: M2

### Milestone M4 (Priority Medium) — REFACTOR Phase: 품질 보강 + TRUST 5

**산출물**:
- 한국어 에러 메시지 정합
- 공통 헬퍼 추출(중복 제거)
- @MX 태그 추가(fan_in≥3 ANCHOR / 복잡도≥15 WARN / NEW 코드 NOTE)
- godoc 추가(exported 모든 함수)
- 테스트 커버리지 ≥85% 검증

**완료 기준**: TRUST 5 PASS. 커버리지 ≥85%. evaluator-active 0.85+. golangci-lint zero issues.

**의존**: M3

### Milestone M5 (Priority Medium) — Drift-Guard 검증

**산출물**:
- 모든 [EXISTING] 파일 `git diff` 0 확인:
  - `internal/store/score.go`, `eval_item.go`, `evidence.go`
  - `cmd/server/score_handlers.go`, `report_handlers.go`, `evidence_handlers.go`
  - `internal/auth/**/*.go` (rbac.go, abac.go, middleware.go, authz_middleware.go, chain.go)
  - `.moai/db/schema/migrations/0001`~`0004`
  - `go.mod`, `go.sum`
- [MODIFY] 파일 추가 범위 검증:
  - `store.go`: 신규 인터페이스/struct만, 기존 인터페이스/struct 무변경
  - `pg_store.go`: `BeginScoreReviewRequestTx`만, 기존 메서드 무변경
  - `audit.go`: 신규 상수 4개만
  - `recorder.go`: 신규 메서드 4개만
  - `errors.go`: 신규 센티넬 6개만 (SCORE-API-001 errors.go drift 교훈 — manifest에 부착하여 분실 방지)
  - `server.go`: ≈7줄만(필드+생성+마운트 2줄+주석)

**완료 기준**: drift = 0%. 위반 발생 시 즉시 중단·재계획(spec.md §2.3 R-CONSUMER-001).

**의존**: M4

---

## 3. 기술 접근 (Technical Approach)

### 3.1 store layer 패턴 (eval_item.go / score.go 미러)

```
PgScoreReviewRequestTx struct {
    tx       pgx.Tx
    logger   *zap.Logger
    recorder *audit.Recorder  // pg_store.go:143-147 동형
}

func (t *PgScoreReviewRequestTx) InsertScoreReviewRequest(ctx, scoreID UUID, comment string, metadata map[string]any) (UUID, error) {
    // 1. validateReviewRequestInput (SQL 미실행 후 거부 — fail-closed)
    // 2. INSERT INTO score_review_requests RETURNING id
    // 3. t.recorder.RecordScoreReviewRequestCreated(ctx, t, id, scoreID, ...) — 동일 TX
    // 4. return id
}

func (t *PgScoreReviewRequestTx) AssignReviewer(ctx, id UUID, reviewerID string, principalID string) error {
    // 1. SELECT ... FOR UPDATE — concurrent transition lock (§6 OPEN #6 Option A)
    // 2. validateReviewStatusTransition(current, 'UNDER_REVIEW') — SUBMITTED 아니면 ErrScoreReviewRequestNotSubmitted
    // 3. UPDATE status='UNDER_REVIEW', assigned_reviewer_id=reviewerID, updated_at=now(), updated_by=principalID
    // 4. t.recorder.RecordScoreReviewRequestReviewerAssigned(...)
}
```

state-machine 가드 함수:

```
var allowedReviewTransitions = map[string][]string{
    "SUBMITTED":    {"UNDER_REVIEW"},
    "UNDER_REVIEW": {"APPROVED", "REJECTED"},
    "APPROVED":     {},  // terminal
    "REJECTED":     {},  // terminal
}

func validateReviewStatusTransition(current, target string) error {
    allowed, ok := allowedReviewTransitions[current]
    if !ok { return ErrScoreReviewRequestInvalidStatus }
    for _, a := range allowed {
        if a == target { return nil }
    }
    return ErrScoreReviewRequestInvalidStatus
}
```

### 3.2 HTTP handler 패턴 (score_handlers.go:43-190 미러)

```
type ReviewHandler struct {
    reviewStore store.ScoreReviewRequestStore
    scoreStore  store.ScoreStore  // cross-store 점수 검증용 (§6 OPEN #3 Option A)
    logger      *zap.Logger
}

func NewReviewHandler(rs store.ScoreReviewRequestStore, ss store.ScoreStore, logger *zap.Logger) *ReviewHandler {
    return &ReviewHandler{reviewStore: rs, scoreStore: ss, logger: logger}
}

func (h *ReviewHandler) Routes() http.Handler {
    mux := http.NewServeMux()
    // 구체 경로 먼저 (ServeMux Go1.22+ 최장일치)
    mux.HandleFunc("POST /api/v1/reviews/{id}/assign-reviewer", h.handleAssignReviewer)
    mux.HandleFunc("POST /api/v1/reviews/{id}/approve", h.handleApprove)
    mux.HandleFunc("POST /api/v1/reviews/{id}/reject", h.handleReject)
    mux.HandleFunc("GET /api/v1/reviews/{id}", h.handleGetReview)
    mux.HandleFunc("GET /api/v1/reviews", h.handleListReviews)
    mux.HandleFunc("POST /api/v1/reviews", h.handleCreateReview)
    return mux
}
```

ABAC 게이트(score_handlers.go:161-190 동형):

```
func requireReviewSubmitRole(scope string) bool {
    for _, r := range auth.ParseRolesFromScope(scope) {
        if r == auth.RoleAdmin || r == auth.RoleAnalyst { return true }
    }
    return false
}

func requireReviewAdminRole(scope string) bool {
    for _, r := range auth.ParseRolesFromScope(scope) {
        if r == auth.RoleAdmin { return true }
    }
    return false
}

func (h *ReviewHandler) guardReviewSubmit(w, r) bool {
    u, ok := auth.UserFromContext(r.Context())
    if !ok { return true }  // auth-disabled 투과
    if requireReviewSubmitRole(strings.Join(u.Scopes, " ")) { return true }
    h.writeReviewErr(w, 403, auth.ErrCodeABACDenied, "제출 권한이 없는 사용자입니다", "")
    return false
}

func (h *ReviewHandler) guardReviewAdmin(w, r) bool {
    u, ok := auth.UserFromContext(r.Context())
    if !ok { return true }
    if requireReviewAdminRole(strings.Join(u.Scopes, " ")) { return true }
    h.writeReviewErr(w, 403, auth.ErrCodeABACDenied, "행정 권한이 없는 사용자입니다", "")
    return false
}
```

### 3.3 0005 마이그레이션 SQL (0004 패턴 정확 미러)

```sql
-- 0005_score_review_request_tables.sql

CREATE TABLE IF NOT EXISTS score_review_requests (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    score_id             UUID NOT NULL,  -- FK-less stub → SCORE-001 scores.id (UUID)
    status               VARCHAR(32) NOT NULL DEFAULT 'SUBMITTED',
    assigned_reviewer_id VARCHAR(256),
    rejection_reason     TEXT,
    comment              TEXT,
    metadata             JSONB,
    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by           VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
    updated_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_by           VARCHAR(64)
);

DO $$ BEGIN
    ALTER TABLE score_review_requests
        ADD CONSTRAINT score_review_requests_status_chk
        CHECK (status IN ('SUBMITTED', 'UNDER_REVIEW', 'APPROVED', 'REJECTED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- §6 OPEN #5 Option A 채택 시 추가 (이중 방어):
DO $$ BEGIN
    ALTER TABLE score_review_requests
        ADD CONSTRAINT score_review_requests_reject_reason_chk
        CHECK ((status != 'REJECTED') OR (rejection_reason IS NOT NULL AND length(rejection_reason) > 0));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS score_review_requests_score_id_idx ON score_review_requests (score_id);
CREATE INDEX IF NOT EXISTS score_review_requests_status_idx ON score_review_requests (status);
CREATE INDEX IF NOT EXISTS score_review_requests_created_at_idx ON score_review_requests (created_at DESC);
```

### 3.4 server.go 마운트 (≈7줄, SCORE-API-001/REPORT-001 정확 미러)

```go
// server.go:55-57 영역 (필드 추가, scoreH/reportH 선례 라인)
reviewH *ReviewHandler

// server.go:210-213 영역 (생성자, NewScoreHandler/NewReportHandler 선례)
// 단계 (i-4): 평가 검토 핸들러 (SPEC-AX-REVIEW-001) — pgStore가 ScoreReviewRequestStore+ScoreStore 동시 구현
s.reviewH = NewReviewHandler(pgStore, pgStore, logger)

// server.go:266-270 영역 (innerMux 마운트 2줄)
innerMux.Handle("/api/v1/reviews", s.reviewH.Routes())
innerMux.Handle("/api/v1/reviews/", s.reviewH.Routes())
```

**총 ≈7줄**: 필드 1줄 + 주석 1줄 + 생성자 1줄 + 마운트 2줄 + 주석 2줄(상단/구분). **"1줄"이 아닌 ≈7줄 최소 단위** — REPORT-001 spec.md:32 manifest accuracy 교훈 정확 적용.

---

## 4. TDD RED-GREEN-REFACTOR 사이클

### 4.1 RED: 16+ failing test 작성

**store-layer** (score_review_request_test.go):
1. TestInsertScoreReviewRequest_ValidInput_ReturnsUUIDAndInsertsAuditRow
2. TestInsertScoreReviewRequest_BlankScoreID_ReturnsInvalidInput
3. TestInsertScoreReviewRequest_AuditFailure_TwoWayRollback
4. TestAssignReviewer_FromSubmitted_TransitionsToUnderReview
5. TestAssignReviewer_FromUnderReview_ReturnsNotSubmitted
6. TestApproveRequest_FromUnderReview_TransitionsToApproved
7. TestApproveRequest_FromSubmitted_ReturnsNotUnderReview
8. TestRejectRequest_FromUnderReview_WithReason_TransitionsToRejected
9. TestRejectRequest_FromUnderReview_NoReason_ReturnsInvalidInput
10. TestStatusTransition_FromTerminal_AlwaysReturnsInvalidStatus
11. TestConcurrentAdminApprove_SelectForUpdate_OnlyFirstSucceeds
12. TestValidateReviewStatusTransition_AllAllowedAndDisallowed

**HTTP handler** (review_handlers_test.go):
13. TestPOST_Reviews_AnalystCreates_201
14. TestPOST_Reviews_ViewerForbidden_403
15. TestPOST_Reviews_ScoreNotExist_404 (cross-store 검증)
16. TestPOST_ReviewsApprove_AnalystForbidden_403
17. TestPOST_ReviewsApprove_AdminSucceeds_200
18. TestPOST_ReviewsReject_RejectionReasonMissing_400
19. TestPOST_ReviewsAssignReviewer_AnalystForbidden_403
20. TestGET_Reviews_AuthDisabled_PassthroughCliAnonymous
21. TestGET_Reviews_EmptyList_Returns200WithEmptyArray
22. TestGET_Reviews_MalformedUUID_400
23. TestGET_Reviews_PaginationClamp
24. TestPOST_Reviews_AuditWriteFailed_RollsBack

### 4.2 GREEN: 최소 구현으로 통과

각 test에 대해 최소 코드만 작성. 추상화 금지(YAGNI). 모든 한국어 에러 메시지 적용.

### 4.3 REFACTOR: 품질 보강

- 공통 헬퍼 추출(writeReviewJSON/Err 등)
- @MX 태그 추가
- godoc 완성
- 한국어 에러 메시지 정합 검토

---

## 5. 위험 요소 및 완화 (Risks & Mitigations)

| 위험 | 영향 | 완화 |
|------|------|------|
| R-CONSUMER-001: 구현 중 [EXISTING] 파일 수정 발생 (drift) | HARD 위반, consumer-only 깨짐 | M5 Drift-Guard에서 `git diff` 0 검증. 위반 시 즉시 중단·재계획 |
| R-PHANTOM-001: 신규 store 메서드 시그니처 오류 (phantom) | 컴파일 실패 또는 잘못된 동작 | 모든 시그니처 spec.md §2.1 검증 후 작성. 신규 [NEW]만 가능. 기존 [EXISTING] 호출은 source-verified 라인 인용 |
| R-MIGRATION-001: 0005 마이그레이션 비멱등 | 재실행 시 duplicate_object 에러 | 0004 정확 미러(CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION). 정방향 + 재실행 모두 테스트 |
| R-RBAC-001: frozen rbac.go 0-diff 위반 (RoleReviewer 신설 등) | AUTH-003 frozen 계약 위반 | M2에서 ABAC 게이트는 핸들러-로컬만 사용. `RoleAdmin`/`RoleAnalyst` 기존 상수만 호출 |
| R-CROSS-STORE-001: cross-store 2-TX race (점수 삭제 race) | 점수 존재 검증 후 INSERT 사이 race | PoC 수용 — SCORE-001은 물리 삭제 0(append-only) → race 不發生. 명시적 trade-off 문서화(§6 OPEN #3) |
| R-CONCURRENT-001: 동시 admin 승인 race | 둘 다 APPROVED 처리 | `SELECT ... FOR UPDATE` row lock (§6 OPEN #6 Option A) + `validateReviewStatusTransition` 이중 방어 |
| R-DRIFT-MANIFEST-001: errors.go 변경 manifest 누락 (SCORE-API-001 교훈) | drift-guard 통과 실패 | spec.md §2.1/§2.3 manifest에 errors.go [MODIFY] 명시 부착 완료. M5에서 명시 검증 |
| R-SERVER-MOUNT-001: server.go 마운트 "1줄"로 잘못 기술 (REPORT-001 교훈) | manifest accuracy 위반 | spec.md §2.1 server.go 행에 ≈7줄(필드+생성+마운트 2줄+주석) 명시. SCORE-API-001 :55/:210/:266-267 + REPORT-001 :56/:212/:269-270 라인 인용 |
| R-OPEN-RESOLUTION-001: 6 OPEN 결정 미루어진 채 구현 시작 | 재작업 발생 | M0 RED 작성 전 strategy.md 작성 + Human Gate 통과 필수. 권장 옵션은 spec.md §6에 명시 |

---

## 6. 검증 (Verification)

### 6.1 TRUST 5 기준

- **Tested**: 테스트 커버리지 ≥85%, 16+ test (store-layer 12 + handler 12+), 4-상태 전이/ABAC narrowing/cross-store/edge case 모두 포함
- **Readable**: 한국어 godoc/에러 메시지, 한국어 @MX 주석(language.yaml `code_comments: ko` 정합), 명확한 함수명
- **Unified**: `gofmt` / `goimports` / `golangci-lint` zero issues
- **Secured**: OWASP 준수(입력 검증 fail-closed, raw pgx 에러 누출 금지, ABAC narrowing 강제), `RecordScoreReviewRequest*` 동일-TX
- **Trackable**: SCORE-001/SCORE-API-001/REPORT-001 패턴 인용 commit, drift-guard manifest 검증 commit

### 6.2 Acceptance Criteria 분포 (acceptance.md 상세)

- §0 UBI: 4 AC (UBI-001~UBI-004)
- §1 REQ-REVIEW-001 (store): 4 AC (001-E1, 001-S1, 001-U1, 001-O1)
- §2 REQ-REVIEW-002 (HTTP create/get/list): 4 AC (002-E1, 002-E2, 002-E3, 002-U1)
- §3 REQ-REVIEW-003 (HTTP transitions): 4 AC (003-E1, 003-E2, 003-E3, 003-S1)
- §4 REQ-REVIEW-004 (audit): 2 AC (004-E1, 004-U1)
- §5 REQ-REVIEW-005 (error mapping): 2 AC (005-E1, 005-U1)
- **총 AC = 20** (8 UBI/store + 8 HTTP + 4 audit/error)
- Edge case: 16 (acceptance.md §3)

### 6.3 OPEN 결정 검증 [RESOLVED]

`strategy.md` (Human Gate sign-off 2026-05-20)에서 6 OPEN 모두 RESOLVED 완료 — §A.1~§A.6 참조. spec.md §6.1~§6.6에 결정 요약 부착. tasks.md §0에 RESOLVED 결정 명문 + Task 매핑. M0 RED 진입 준비 완료.

---

## 7. Pre-flight Checklist

- [x] research.md SSOT 검증 (665줄, file:line 근거)
- [x] SCORE-001 / SCORE-API-001 / REPORT-001 패턴 정확 미러 확인
- [x] frozen RBAC 0-diff 매핑 명확 (RoleAdmin/RoleAnalyst 기존 사용)
- [x] phantom API 0건 (source-verified line 인용)
- [x] errors.go drift 부착 정확 (SCORE-API-001 교훈)
- [x] server.go 마운트 ≈7줄 정확 (REPORT-001 교훈, "1줄" 잘못 기술 방지)
- [x] 0005 마이그레이션 비충돌 확인 (0001/0002/0003/0004 디스크)
- [x] Drift-Guard manifest 명시(§2.3)
- [x] §6 OPEN 6건 모두 권장 옵션 부착 (Run phase strategy.md 결정)
- [ ] Human Gate sign-off (Run phase 진입 시)

---

## 8. 참조

- spec.md §1.4 consumer-only HARD
- spec.md §2.1 영향파일 + Delta 마커
- spec.md §2.3 Drift-Guard Manifest (errors.go 부착 정확)
- spec.md §3 EARS 20 AC
- spec.md §6 OPEN 6건 (권장 옵션 부착)
- research.md §13 추천 구현 접근
- SPEC-AX-SCORE-001 plan.md (store+audit 패턴 선례)
- SPEC-AX-SCORE-API-001 plan.md (HTTP + ABAC 패턴 선례)
- SPEC-AX-REPORT-001 plan.md (consumer-only + server.go ≈7줄 + errors.go drift 정확성 선례)
