# Research: SPEC-AX-REVIEW-001 평가 제출/승인 워크플로우 저장소 + HTTP API (Score Review Request Store + HTTP API)

**Phase**: 0.5 Deep Research  
**Generated**: 2026-05-20  
**Agent**: Explore (read-only deep codebase analysis)  
**Status**: complete  

> SPEC-AX-REVIEW-001 EARS 요구사항 설계 근거. 모든 주장 file:line 근거 포함.

---

## 1. 아키텍처 분석 — Store/Audit 재사용 맵

### 1.1 2계층 패턴 (WorkflowStore/EvidenceStore/EvalItemStore 선례 미러)

- 공개 `XxxStore`(TX 진입점만) + 내부 `XxxTx`(단일 pgx TX 내 read/write) 패턴
  - `store.go:17-30` WorkflowStore 인터페이스 (BeginTx만 노출)
  - `store.go:32-55` WorkflowTx 인터페이스 (InsertWorkflow/InsertAuditLog/UpdateWorkflowState/Commit/Rollback 정의)
  - `store.go:56-67` EvidenceStore 인터페이스 (BeginEvidenceTx 패턴 미러)
  - `store.go:69-107` EvidenceTx 인터페이스 (InsertEvidence/GetEvidenceByID/MarkSuperseded/InsertAuditLog/Commit/Rollback)
  - `store.go:109-119` EvalItemStore 인터페이스 (BeginEvalItemTx 패턴 동일 미러)

**REVIEW-001 통합**: `store.go`에 `ScoreReviewRequestStore` 인터페이스 추가 (EvalItemStore 패턴), `pg_store.go`에 `BeginScoreReviewRequestTx` 진입점, 신규 `score_review_request.go`에 `PgScoreReviewRequestTx` 구현(eval_item.go 미러), `recorder.go`에 `RecordScoreReviewRequest*` 메서드(local AuditTx, recorder.go RecordEvalItem* 미러).

### 1.2 PgWorkflowStore 단일 pgx 풀 재사용 (신규 풀 금지)

- `PgWorkflowStore{pool *pgxpool.Pool, logger *zap.Logger}` 단일 풀 전 도메인 공유
  - `pg_store.go:26-31` PgWorkflowStore 구조체 정의
  - `pg_store.go:84` BeginTx: `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})`
  - `pg_store.go:103-109` BeginEvidenceTx: 동일 풀 재사용, `s.pool.BeginTx(...)`
  - `pg_store.go:118-124` BeginEvalItemTx: 동일 풀 재사용, `s.pool.BeginTx(...)`
  - `pg_store.go:134-148` **BeginScoreTx** (SPEC-AX-SCORE-001 완성): 동일 풀 재사용, `s.pool.BeginTx(...)`
  - `pg_store.go:143-147` Recorder 주입: `recorder: audit.NewRecorder(false)` (authEnabled=false)

**REVIEW-001**: `pg_store.go`에 `BeginScoreReviewRequestTx` 추가 = `s.pool.BeginTx()` + `PgScoreReviewRequestTx{tx, logger, recorder}` 반환. 신규 pgxpool 금지, PgWorkflowStore.pool만 사용.

### 1.3 postgres.go 회피 (Sprint-0 死 스텁, 비대상)

- `postgres.go` 비대상 확인 (Sprint-0 Death stub, 실 pool 없음)
  - SCORE-001 research.md §1, plan.md §2 명시: postgres.go는 비대상
  - score_handlers.go:9 주석: "postgres.go의 레거시 Store와 충돌 없이 공존"
  - pg_store.go:98, :113, :129 주석: "신규 pool을 생성하지 않으며, postgres.go(死 스텁)는 대상이 아니다"

**REVIEW-001**: postgres.go 회피 동일 적용.

---

## 2. 의존성 Stub 계약 — score_id FK-less 참조 (SCORE-001 소비 계약)

### 2.1 scores 테이블 구조 (SCORE-001 0004_score_tables.sql)

**점수 테이블 schema** — SPEC-AX-SCORE-001 완료:
- `0004_score_tables.sql:11-24` scores 테이블
  - `id UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
  - `evaluation_item_id VARCHAR(64) NOT NULL` (FK-less stub, EVAL-ITEM-001 호환)
  - `evidence_id UUID` (FK-less stub, EVID-001 호환, nullable)
  - `score_value DECIMAL(6,2)` (nullable)
  - `status VARCHAR(32) NOT NULL DEFAULT 'DRAFT'` (state-machine: DRAFT|CONFIRMED|SUPERSEDED)
  - `created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()`
  - `created_by VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous'`

**상태 전이** (SCORE-001 spec.md:33, research.md §5/§6/§7, HISTORY v0.1.3):
- Decision 4 (D4) RESOLVED: status state-machine + append-only 하이브리드
- 전이표: `DRAFT→DRAFT`(가변) · `DRAFT→CONFIRMED`(확정) · `CONFIRMED→SUPERSEDED`(정정 시) → terminal
- DELETE 연산 0건 (물리 삭제 금지)

### 2.2 FK-less Stub 계약 (consumer-only [HARD])

**[HARD] SCORE-001 계약**:
- `scores.id` = UUID PK (audit resource_id 직접 대입, surrogate 미사용)
- `scores.evaluation_item_id` = VARCHAR(64) FK-less (EVAL-ITEM-001 §1.4 호환)
- `scores.evidence_id` = UUID FK-less (EVID-001 호환)
- EVID-001/EVAL-ITEM-001 코드 변경 범위 밖 (SCORE-001 spec.md:54-57 명시)

**REVIEW-001 consumer 계약**:
- `score_review_requests.score_id` = UUID NOT NULL FK-less stub (SCORE-001 scores.id 참조, FK 제약 없음)
- 점수 데이터 검증 = SCORE-001 store layer (점수 존재 여부/상태 검증 → SCORE-001 GetScoreByID)
- SCORE-001 코드 무수정 (mutation 0)

---

## 3. 평가 제출/승인 워크플로우 (Review Request 생명주기)

### 3.1 4-상태 생명주기 및 전이

**요구사항** (spec intent, 2026-05-20 interview):
- **4 상태**: SUBMITTED → UNDER_REVIEW → {APPROVED|REJECTED} (terminal)
- **SUBMITTED**: 분석가(analyst) 제출 (score 평가 대상 선정 후 검토 요청)
- **UNDER_REVIEW**: 행정가(admin)가 검토자 할당하고 검토 수행 중
- **APPROVED/REJECTED**: 행정가의 최종 승인/반려, 되돌리기 불가

**상태 다이어그램**:
```
SUBMITTED 
    ↓ (assign-reviewer: admin action)
UNDER_REVIEW 
    ↓ (approve or reject: admin action)
APPROVED or REJECTED (terminal)
```

**SCORE-001 D4 전이표와 동형**:
- SCORE-001: DRAFT→DRAFT · DRAFT→CONFIRMED · CONFIRMED→SUPERSEDED (§5 decision 4, plan.md §6)
  - 차용점: 상태 불변식 + terminal 선언 + 되돌리기 불가
  - 차용점: append-only (신규 행 INSERT, 기존 행 UPDATE status만)

### 3.2 검토 요청 엔티티 필드

**구조** (SCORE-001 scores 테이블 구조 기반):
```
score_review_requests 테이블 (0005_score_review_request_tables.sql 신규)
  id                    UUID PK DEFAULT uuid_generate_v4()
  score_id              UUID NOT NULL FK-less (SCORE-001 stub)
  status                VARCHAR(32) NOT NULL DEFAULT 'SUBMITTED' (enum: SUBMITTED|UNDER_REVIEW|APPROVED|REJECTED)
  assigned_reviewer_id  VARCHAR(256) NULL (사용자 ID, 권한 맵핑 스코프 정보)
  rejection_reason      TEXT NULL (REJECTED 상태일 때만 의미)
  comment               TEXT NULL (검토자 코멘트, 모든 상태)
  created_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
  created_by            VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous'
  updated_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
  updated_by            VARCHAR(64) NULL (마지막 수정자)
  metadata              JSONB (opaque placeholder)
```

**Key decisions**:
- `score_id` = UUID FK-less stub (SCORE-001 점수 참조, 존재 검증은 핸들러/store layer)
- `status` = state-machine enum (CHECK constraint + 전이 validation 양방향 방어)
- `created_by` = 'cli-anonymous' 기본값 (authEnabled=false, SCORE-001 §1.1 정합)
- `assigned_reviewer_id` = nullable VARCHAR(256) (reviewer 할당 전까지 NULL)
- `rejection_reason` = nullable TEXT (REJECTED 상태일 때만 기록)
- `comment` = nullable TEXT (검토자 추가 기록, 모든 상태)

---

## 4. 마이그레이션 구조 (0005_score_review_request_tables.sql)

### 4.1 파일 충돌 검증

**기존 마이그레이션 목록**:
- `0001_initial.sql` (SPEC-AX-CTRL-001 + audit/workflow 기본)
- `0002_evidence_tables.sql` (SPEC-AX-EVID-001)
- `0003_eval_item_tables.sql` (SPEC-AX-EVAL-ITEM-001)
- `0004_score_tables.sql` (SPEC-AX-SCORE-001, disk 확인됨)

**REVIEW-001 신규**:
- `0005_score_review_request_tables.sql` (비충돌 확인, 파일번호 순차)

### 4.2 멱등성 패턴 (0002/0003/0004 선례)

**패턴** (0002_evidence_tables.sql:26-29, 0004_score_tables.sql:27-30/33-36):
```sql
CREATE TABLE IF NOT EXISTS score_review_requests (
    -- 컬럼 정의
);

DO $$ BEGIN
    ALTER TABLE score_review_requests ADD CONSTRAINT score_review_requests_status_chk
        CHECK (status IN ('SUBMITTED', 'UNDER_REVIEW', 'APPROVED', 'REJECTED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS score_review_requests_score_id_idx ON score_review_requests (score_id);
```

**REVIEW-001 준수**:
- CREATE TABLE IF NOT EXISTS (idempotent)
- DO $$ BEGIN ... EXCEPTION WHEN duplicate_object $$  (constraint add retry)
- CREATE INDEX IF NOT EXISTS (idempotent)
- 모든 변경 cumulative (rollback 없음)

---

## 5. HTTP API 핸들러 구조 (SCORE-API-001 선례 미러)

### 5.1 SCORE-API-001 핸들러 기본 구조

**참조 핸들러**: `score_handlers.go` (SPEC-AX-SCORE-API-001, 완료)

**구조 요소** (score_handlers.go source line):
- `score_handlers.go:37-51` ScoreHandler 구조체 + NewScoreHandler 생성자
- `score_handlers.go:59-70` Routes() → http.Handler (ServeMux Go1.22+ 최장일치)
- `score_handlers.go:74-102` 표준 JSON 헬퍼 (writeScoreJSON/writeScoreErr)
- `score_handlers.go:111-129` mapStoreErr (store sentinel → HTTP status/code/message)
- `score_handlers.go:161-187` requireScoreWriteRole / guardScoreWrite (ABAC narrowing)

**특징**:
- 라우트 구체 경로(longest-match priority): /rollup · /grade · /{id}/supersede 먼저 등록 (§7 edge)
- 표준 에러 본문: `{"error":{"code", "message", "field"}}`
- store 에러 → HTTP 결정적 매핑 (error sentinel 중복 coverage 방지)
- ABAC write-role 게이팅 (requireScoreWriteRole: admin|analyst만 write 허용)

### 5.2 REVIEW-001 API 설계 (SCORE-API-001 consumer)

**REVIEW-001 핸들러 패턴** (`review_handlers.go` 신규):
- `ReviewHandler` 구조체: `store.ScoreReviewRequestStore` + logger
- `Routes()`: HTTP 메서드별 핸들러 등록 (구체 경로 먼저)
- 표준 JSON/에러 헬퍼: 동일 형식 (writeReviewJSON/writeReviewErr)
- mapStoreErr: REVIEW-001 sentinel 매핑 (NOT_FOUND/CONFLICT/INVALID_ARGUMENT 등)

**엔드포인트** (spec intent):
```
POST   /api/v1/score-review-requests          - 제출 (create-review-request)
GET    /api/v1/score-review-requests/{id}    - 조회 (get-review-request)
GET    /api/v1/score-review-requests          - 목록 (list-review-requests)
POST   /api/v1/score-review-requests/{id}/assign-reviewer  - 검토자 할당 (admin only)
POST   /api/v1/score-review-requests/{id}/approve         - 승인 (admin only)
POST   /api/v1/score-review-requests/{id}/reject          - 반려 (admin only)
```

**ABAC narrowing** (score_handlers.go:161-187 선례):
- analyst = write (create review-request)
- admin = approve/reject/assign-reviewer
- viewer/all-authenticated = read (list/get)
- auth-disabled = cli-anonymous 투과

---

## 6. ABAC/RBAC 통합 (SCORE-API-001 OPEN#4 기각 — REVIEW는 역할 명확)

### 6.1 rbac.go 역할 정의 (frozen RBAC, 0-diff)

**고정 역할** (rbac.go:19-26):
```go
const (
    RoleAdmin    Role = "admin"     // 모든 WorkflowService + 미래 AdminService
    RoleAnalyst  Role = "analyst"   // CreateWorkflow/GetWorkflow/upload
    RoleViewer   Role = "viewer"    // GetWorkflow/ListWorkflows (읽기 전용)
)
```

**역할 정규식** (rbac.go:33):
- `^iroum-ax:(admin|analyst|viewer)$` (scope token 인식)

**역할 추출** (rbac.go:68-80):
- `ParseRolesFromScope(scope string) []Role` (공백 구분 scope → role 배열)

### 6.2 SCORE-API-001 OPEN#4 문제 (REVIEW-001은 미해당)

**SCORE-API-001 OPEN#4** (score_handlers.go comment, spec.md §6):
- write 엔드포인트: analyst가 create · admin이 update/supersede
- rbac.go에 "evaluator" 역할 부재 (정규식 미포함)
- permissionMatrix에 score/report Permission 부재
- frozen RBAC 0-diff constraint → mapping 불가능

**SCORE-API-001 resolution** (strategy.md §A #4):
- evaluator는 RoleAnalyst로 fallback (permissionMatrix analyst 권한 사용)
- 또는 write-role 게이트(`requireScoreWriteRole`)에서 RoleAdmin|RoleAnalyst 직접 판정

### 6.3 REVIEW-001 역할 매핑 (명확, OPEN 없음)

**설계**:
- **제출** (POST create-review-request): analyst만 (write-role 게이팅)
  - `guardScoreWrite` → `requireScoreWriteRole` → RoleAnalyst 체크 통과
- **승인/반려/검토자할당** (POST approve/reject/assign-reviewer): admin only
  - `guardScoreWrite` → `requireScoreWriteRole` → RoleAdmin 체크 통과
- **조회** (GET list/get): 모든 인증 사용자 (viewer 포함)
  - read-only → read-role 게이팅 불필요 (ABAC narrowing-only)

**frozen RBAC 준수**:
- permissionMatrix 수정 0건 (frozen)
- 역할은 rbac.go의 3개만 사용
- requireScoreWriteRole 동형 패턴 재사용

---

## 7. Audit 연계 (SCORE-001 RecordScore* 선례, 동일 TX)

### 7.1 감사 액션 정의 (audit.go)

**SCORE-001 구현** (audit.go:69-74):
```go
ActionScoreCreated  Action = "SCORE_CREATED"   // Decision 2: resource_id = scores.id 직접
ActionScoreUpdated  Action = "SCORE_UPDATED"   // Decision 2: surrogate 미사용
```

**REVIEW-001 신규 액션** (audit.go 확장):
```go
ActionScoreReviewRequestCreated Action = "SCORE_REVIEW_REQUEST_CREATED"
ActionScoreReviewRequestApproved Action = "SCORE_REVIEW_REQUEST_APPROVED"
ActionScoreReviewRequestRejected Action = "SCORE_REVIEW_REQUEST_REJECTED"
ActionScoreReviewRequestReviewerAssigned Action = "SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED"
```

**design decision**:
- resource_id = score_review_requests.id (UUID PK 직접 대입, surrogate 미사용)
- SCORE-001 Decision 2 (D2) 패턴 미러 (UUID PK → audit.Event.ResourceID 직접)
- AUD-1 namespace 상수 추가 0개 (scores.id UUID면 surrogate 불필요 — SCORE-001 research.md §6)

### 7.2 Recorder 확장 (recorder.go)

**SCORE-001 메서드** (recorder.go:130-140, :398+):
```go
RecordScoreCreated(ctx, tx, scoreID, evaluationItemID, level, userID)
RecordScoreUpdated(ctx, tx, scoreID, ...)
```

**REVIEW-001 신규 메서드** (recorder.go 확장):
```go
RecordScoreReviewRequestCreated(ctx, tx, reviewRequestID, scoreID, submitterID, userID)
RecordScoreReviewRequestApproved(ctx, tx, reviewRequestID, approverID, userID)
RecordScoreReviewRequestRejected(ctx, tx, reviewRequestID, approverID, rejectionReason, userID)
RecordScoreReviewRequestReviewerAssigned(ctx, tx, reviewRequestID, reviewerID, assignedByID, userID)
```

**동일 TX 원자성** (SCORE-001 @MX:WARN:DC-UBI-002 선례):
- entity-INSERT (score_review_requests 행) → audit-INSERT (동일 t.tx)
- recorder 실패 시 호출자가 Rollback → 양방향 취소 (DC-004-U1, entity+audit atomic)

---

## 8. State-Machine Validation (SCORE-001 validateStatusTransition 선례)

### 8.1 전이 규칙 (SCORE-001 plan.md §3.4, spec.md:33)

**SCORE-001 모델** (0004_score_tables.sql:32-36, score.go:36-41):
```
DRAFT    → DRAFT (가변)
DRAFT    → CONFIRMED (확정)
CONFIRMED → SUPERSEDED (정정)
SUPERSEDED → terminal (되돌리기 금지)

allowedScoreStatus = {DRAFT, CONFIRMED, SUPERSEDED}
confirmedImmutableFields = [score_value, weight, grade]
```

**REVIEW-001 모델** (신규 validateReviewRequestStatusTransition):
```
SUBMITTED    → UNDER_REVIEW (assign-reviewer)
UNDER_REVIEW → APPROVED (approve)
UNDER_REVIEW → REJECTED (reject)
APPROVED/REJECTED → terminal (되돌리기 금지)

allowedReviewStatus = {SUBMITTED, UNDER_REVIEW, APPROVED, REJECTED}
```

### 8.2 검증 구현 (score.go:79-97 선례)

**SCORE-001 validateScoreInput** (score.go:79-97):
- SQL 미실행 후 거부 (fail-closed)
- evaluation_item_id blank/길이 검증
- level enum 검증
- score_value NaN/Inf 검증 (DECIMAL(6,2) 표현 불가 → 거부)

**REVIEW-001 validateReviewRequestInput** (score_review_request.go 신규):
- SQL 미실행 후 거부
- score_id = UUID (타입 체크, 시그니처로 보장)
- assigned_reviewer_id blank 금지 (assign-reviewer 시)
- status transition 합법성 검증 (현재상태 + 목표상태 → allowedTransitions 확인)
- rejection_reason: REJECTED 상태일 때만 기록 (NULL 아님)

---

## 9. Server.go 라우트 마운트 (SCORE-API-001 선례 정확성)

### 9.1 마운트 구조 (server.go:55/209/263-264)

**SCORE-API-001 구현 검증**:
- `server.go:210` ScoreHandler 생성: `s.scoreH = NewScoreHandler(pgStore, logger)`
- `server.go:266-267` 라우트 마운트:
  ```go
  innerMux.Handle("/api/v1/scores", s.scoreH.Routes())
  innerMux.Handle("/api/v1/scores/", s.scoreH.Routes())
  ```
- 필드: `scoreH *ScoreHandler` (server.go:55, Server struct)

**pattern** (≈7줄, manifest accuracy lesson):
1. (필드 선언) server.go:55 `scoreH *ScoreHandler`
2. (생성자) server.go:210 `s.scoreH = NewScoreHandler(...)`
3. (마운트) server.go:266-267 `innerMux.Handle("/api/v1/scores", ...)`

**REVIEW-001 마운트** (동일 패턴):
1. server.go에 필드 추가: `reportH *ReportHandler` (이미 존재 — SPEC-AX-REPORT-001)
2. New() 단계(i-3)에서 생성: `s.reportH = NewReportHandler(pgStore, pgStore, logger)`
3. Run()에서 마운트:
   ```go
   innerMux.Handle("/api/v1/reviews", s.reviewH.Routes())
   innerMux.Handle("/api/v1/reviews/", s.reviewH.Routes())
   ```

**Attention**: server.go 라우트 마운트는 7줄(필드+생성+2줄마운트 = 최소 필드+초기화+마운트). REPORT-001 spec.md:32 "필드+생성자+innerMux.Handle 2줄, ≈7줄" 정확성 확인됨.

---

## 10. 한국 공공 6제약 정합 (SCORE-001 REQ-UBI pattern)

### 10.1 REQ-UBI 패턴 (audit.go/recorder.go/store layer에 구현됨)

**SCORE-001 canonical 패턴** (research.md §8):
- REQ-SCORE-UBI-001 (데이터 주권): 점수 계산 내부 PostgreSQL만, 외부 호출 0
- REQ-SCORE-UBI-002 (감사 가능성): 모든 score create/update → 동일 TX audit_logs 1건
- REQ-SCORE-UBI-003 (cli-anonymous): AUTH 비활성 시 literal 'cli-anonymous' (NULL 금지)
- REQ-SCORE-UBI-004 (불변): CONFIRMED status → score-field 불변, 정정 = 신규행

### 10.2 REVIEW-001 REQ-UBI (동일 pattern)

**REQ-REVIEW-UBI-001** (데이터 주권):
- 평가 제출/승인 판정 logic 내부 PostgreSQL만
- 외부 API 호출 0 (scoring engine/ML 호출 금지 — post-PoC)

**REQ-REVIEW-UBI-002** (감사 가능성):
- 모든 review-request create/approve/reject/assign-reviewer → 동일 TX audit_logs 1건
- recorder 주입 (pg_store.go:BeginScoreReviewRequestTx에서 동일 TX)

**REQ-REVIEW-UBI-003** (cli-anonymous):
- AUTH 비활성 시 created_by='cli-anonymous' (score_review_requests.created_by)
- authEnabled=false → resolver가 DefaultUserID 반환 (recorder.go:81-86 동형)

**REQ-REVIEW-UBI-004** (불변 — state-machine):
- APPROVED/REJECTED = terminal (상태 되돌리기 금지)
- 정정 경로: REJECTED → 신규 제출(새 SUBMITTED row) — DELETE 연산 0

---

## 11. Error Sentinel 매핑 (SCORE-API-001 mapStoreErr 선례)

### 11.1 SCORE-001 sentinel 정의 (errors.go:52-78)

**점수 관련 sentinel**:
- `ErrScoreNotFound` (line 54): 점수 ID 미존재 → 404 NOT_FOUND
- `ErrScoreInvalidInput` (line 58): 입력 검증 실패 → 400 INVALID_ARGUMENT
- `ErrScoreImmutable` (line 62): CONFIRMED 점수 변경 시도 → 409 CONFLICT
- `ErrScoreInvalidStatus` (line 65): 허용되지 않은 상태 전이 → 409 CONFLICT
- `ErrGradeThresholdsUnavailable` (line 69): scope 등급 기준 부재 → 404 NOT_FOUND
- `ErrScoreAuditWriteFailed` (line 74): audit INSERT 실패 → 내부 TX rollback
- `ErrScoreNotConfirmed` (line 78): CONFIRMED 아닌 행에 정정 시도 → 409 CONFLICT

### 11.2 mapStoreErr 구현 (score_handlers.go:111-129)

**패턴**:
```go
func mapStoreErr(err error) (int, string, string) {
    switch {
    case errors.Is(err, apperrors.ErrScoreNotFound):
        return http.StatusNotFound, "NOT_FOUND", "..."
    // ... 각 sentinel 매핑
    default:
        return http.StatusInternalServerError, "INTERNAL", "..."
    }
}
```

**특징**:
- errors.Is로 래핑된 sentinel 식별 가능
- unknown → 500 INTERNAL (fail-safe)
- 한국어 메시지 (score_handlers.go:114/117/119 등)

### 11.3 REVIEW-001 sentinel (errors.go 확장)

**신규 sentinel**:
```go
var ErrScoreReviewRequestNotFound = errors.New("score review request not found")
var ErrScoreReviewRequestInvalidInput = errors.New("score review request invalid input")
var ErrScoreReviewRequestInvalidStatus = errors.New("score review request invalid status transition")
var ErrScoreReviewRequestNotSubmitted = errors.New("score review request is not in SUBMITTED status")
var ErrScoreReviewRequestNotUnderReview = errors.New("score review request is not in UNDER_REVIEW status")
var ErrReviewerAssignmentFailed = errors.New("reviewer assignment failed")
```

**mapReviewErr** (review_handlers.go):
```go
case errors.Is(err, apperrors.ErrScoreReviewRequestNotFound):
    return http.StatusNotFound, "NOT_FOUND", "요청한 평가 검토를 찾을 수 없습니다"
case errors.Is(err, apperrors.ErrScoreReviewRequestInvalidStatus):
    return http.StatusConflict, "CONFLICT", "허용되지 않은 평가 검토 상태 전이입니다"
// ...
```

---

## 12. 위험 요소, 암묵 계약, 전략 단계 OPEN

### 12.1 [HARD] 불변식

- **score_id FK-less stub**: score_review_requests.score_id는 FK 제약 없음 (SCORE-001 scores.id와 타입 호환 UUID)
  - 존재 검증 = SCORE-001 GetScoreByID 호출 (별도 TX)
  - SCORE-001 코드 무수정 (consumer-only)
- **단일 pgx 풀**: BeginScoreReviewRequestTx = PgWorkflowStore.pool.BeginTx() (신규 풀 금지)
- **1 TX = 1 entity = 1 audit**: entity-INSERT → audit-INSERT (동일 pgx.Tx, 양방향 원자성)
- **audit resource_id = review-request.id** (UUID PK 직접, surrogate 미사용 — SCORE-001 Decision 2 미러)
- **postgres.go 회피**: TX 진입점은 pg_store.go BeginScoreReviewRequestTx만

### 12.2 암묵 계약

- **Recorder.clock 주입**: recorder.go:42-76 (test-friendly time provider)
- **에러 wrapping**: errors.Is로 식별 가능한 sentinel (raw pgx.ErrNoRows 금지 — GAP-03/DC-012)
- **nullIfEmpty 구분**: NULL vs empty string (eval_item.go:439-443 선례)
- **멱등 DO$$**: CREATE TABLE/INDEX IF NOT EXISTS + EXCEPTION duplicate_object (0002/0003/0004 선례)
- **@MX 태그**: fan_in≥3 ANCHOR / 복잡도≥15 WARN / NEW code NOTE (mx-tag-protocol.md)

### 12.3 전략 단계 OPEN 결정 사항

**미결정 사항 (spec intent → strategy phase에서 확정)**:

1. **검토자 할당 형태**
   - Option A: 별도 엔드포인트 `POST /api/v1/score-review-requests/{id}/assign-reviewer` (현재 계획)
   - Option B: UNDER_REVIEW 상태 전이 시 embedded (복잡도 증가)
   - **권장**: Option A (mutation 단일화, state machine 단순)

2. **승인/반려 반응 형태**
   - Option A: 별도 엔드포인트 `POST /api/v1/score-review-requests/{id}/approve` + `POST /{id}/reject`
   - Option B: `PUT /api/v1/score-review-requests/{id}` (상태만 업데이트)
   - **권장**: Option A (SCORE-API-001 선례 — REST 메서드 명확)

3. **Cross-store 검증 (점수 존재 확인)**
   - Option A: 핸들러 단계에서 SCORE-001 GetScoreByID 호출 (2 TX)
   - Option B: store layer에서 단일 TX로 통합 (복잡도 증가, SCORE-001 의존)
   - **권장**: Option A (REPORT-001 spec.md:34 "handler compose" 선례, 신규 store 메서드 0)

4. **Reviewer 역할 정의**
   - Option A: `RoleAdmin`만 검토자 (3역할 유지)
   - Option B: 별도 `RoleReviewer` 역할 추가 (rbac.go 수정 — frozen constraint 위반)
   - **권장**: Option A (frozen RBAC 0-diff, admin이 검토자 역할 수행)

5. **Comment/Reason 필드 작성 조건**
   - `rejection_reason`: REJECTED 상태일 때만 기록 (필수 non-NULL when status=REJECTED)
   - `comment`: 모든 상태에서 optional (NULL 허용)
   - **권장**: 양쪽 nullable (flexibility) — 하지만 rejection_reason은 REJECTED 시 NOT NULL constraint

6. **Concurrent Transition Handling**
   - SELECT FOR UPDATE row lock (EVID-001 GetLatestVersionByEvalItem 패턴)
   - DB CHECK constraint + 애플리케이션 validation (이중 방어)
   - **권장**: SELECT FOR UPDATE + DB CHECK (SCORE-001 eval_item.go 미러)

---

## 13. Recommended Implementation Approach

### 13.1 핵심 모듈 구성

**파일 계획**:
1. `apps/control-plane/internal/store/store.go` [MODIFY]
   - ScoreReviewRequestStore 인터페이스 추가
   - ScoreReviewRequestTx 인터페이스 추가

2. `apps/control-plane/internal/store/score_review_request.go` [NEW]
   - PgScoreReviewRequestTx 구현
   - validateReviewRequestInput / validateStatusTransition
   - InsertScoreReviewRequest / GetScoreReviewRequestByID / ListScoreReviewRequests
   - UpdateScoreReviewRequestStatus / AssignReviewer / ApproveRequest / RejectRequest

3. `apps/control-plane/internal/store/pg_store.go` [MODIFY]
   - BeginScoreReviewRequestTx 메서드 추가 (s.pool.BeginTx + Recorder 주입)

4. `apps/control-plane/internal/audit/audit.go` [MODIFY]
   - ActionScoreReviewRequestCreated 상수 추가
   - ActionScoreReviewRequestApproved 상수 추가
   - ActionScoreReviewRequestRejected 상수 추가
   - ActionScoreReviewRequestReviewerAssigned 상수 추가

5. `apps/control-plane/internal/audit/recorder.go` [MODIFY]
   - RecordScoreReviewRequestCreated 메서드
   - RecordScoreReviewRequestApproved 메서드
   - RecordScoreReviewRequestRejected 메서드
   - RecordScoreReviewRequestReviewerAssigned 메서드

6. `apps/control-plane/internal/errors/errors.go` [MODIFY]
   - ErrScoreReviewRequestNotFound 추가
   - ErrScoreReviewRequestInvalidInput 추가
   - ErrScoreReviewRequestInvalidStatus 추가
   - ErrScoreReviewRequestNotSubmitted 추가
   - ErrScoreReviewRequestNotUnderReview 추가
   - ErrReviewerAssignmentFailed 추가

7. `apps/control-plane/cmd/server/review_handlers.go` [NEW]
   - ReviewHandler 구조체 + NewReviewHandler
   - Routes() 메서드 (구체 경로 먼저)
   - handleCreateReviewRequest / handleGetReviewRequest / handleListReviewRequests
   - handleAssignReviewer / handleApproveRequest / handleRejectRequest
   - 표준 JSON/에러 헬퍼 (writeReviewJSON/writeReviewErr/mapReviewErr)
   - ABAC write-role 게이팅 (requireReviewWriteRole/guardReviewWrite)

8. `apps/control-plane/cmd/server/review_handlers_test.go` [NEW]
   - TDD RED-GREEN-REFACTOR
   - 상태 전이 mutation test
   - ABAC narrowing 테스트
   - concurrent 게이팅 (SELECT FOR UPDATE)

9. `apps/control-plane/cmd/server/server.go` [MODIFY]
   - Server struct 필드 추가: `reviewH *ReviewHandler`
   - New() 단계 추가: `s.reviewH = NewReviewHandler(pgStore, pgStore, logger)`
   - Run() innerMux 마운트: `/api/v1/reviews` 경로

10. `.moai/db/schema/migrations/0005_score_review_request_tables.sql` [NEW]
    - score_review_requests 테이블 (멱등성 패턴)
    - CHECK constraint (status enum)
    - 인덱스 (score_id, created_at)

### 13.2 테스트 전략 (TDD RED-GREEN-REFACTOR)

**RED phase**:
- [ ] TestInsertScoreReviewRequest_ValidInput_Success
- [ ] TestInsertScoreReviewRequest_InvalidScoreID_Fails
- [ ] TestUpdateStatus_SUBMITTED_to_UNDER_REVIEW_Success
- [ ] TestUpdateStatus_CONFIRMED_to_terminal_Rejected_Success
- [ ] TestUpdateStatus_IllegalTransition_APPROVED_to_SUBMITTED_Fails
- [ ] TestAssignReviewer_AuthorizedAdmin_Success
- [ ] TestAssignReviewer_UnauthorizedAnalyst_Forbidden
- [ ] TestApproveRequest_AdminOnly_Success
- [ ] TestRejectRequest_WithReason_Success
- [ ] TestConcurrentTransition_SelectForUpdate_Blocks
- [ ] TestAuditRecordedSameTX_EntityAndAuditAtomic

**GREEN phase**: 최소 구현으로 각 테스트 통과

**REFACTOR**: 에러 메시지 한국어화, 공통 헬퍼 추출

---

## 14. 참고: SPEC-AX-REPORT-001 소비 계약 경험

**교훈** (REPORT-001 spec.md:26 § 1.1 / research.md):
- consumer-only [HARD]: store/audit/auth/schema 무수정
- 신규 마이그레이션 0건 (읽기 전용)
- 자체 audit 0건 (mutation 0)
- 신규 외부 의존 0건 (go.mod 무수정)
- handler compose 패턴 (별도 store 메서드 0, 기존 호출 조합)
- ABAC narrowing-only (write 게이팅 불필요 — mutation 0)
- server.go 라우트 마운트 ≈7줄 (필드+생성+마운트)

**REVIEW-001 적용**:
- consumer-only [HARD]: SCORE-001/EVAL-ITEM-001/AUTH-003 무수정 (mutation 0)
- 신규 마이그레이션 1건: 0005 (DB 변경 존재하므로 REPORT-001과 다름)
- 자체 audit 있음 (RecordScoreReviewRequest*) — mutation 있으므로 REPORT-001과 다름
- 신규 외부 의존 0건 (go.mod 무수정)
- handler compose 패턴 + ABAC 통합
- 신규 store layer 필요 (entity-INSERT/query/status mutation)

---

## Appendix: SCORE-001 Decision Point 참고

**Decision 1 (Option A)**: 단일 scores 테이블 + level discriminator
- research.md §5: Option A ✓ (audit 원자성 1 entity=1 audit), B ✗ (saga orphan), C (post-PoC)

**Decision 2 (Option 1)**: UUID PK → audit resource_id 직접 대입
- research.md §6: 신규 namespace 상수 0개 (UUID 직접)

**Decision 3**: 별도 grade_thresholds 테이블
- research.md §5: metadata JSONB 아님

**Decision 4**: status state-machine + append-only (CONFIRMED 불변)
- research.md §5: DRAFT→CONFIRMED→SUPERSEDED (terminal)

**REVIEW-001 미러**:
- D4 state-machine 동형: SUBMITTED→UNDER_REVIEW→{APPROVED|REJECTED} (terminal)
- D2 audit pattern: resource_id 직접 (UUID PK)

---

## Summary (한국어)

SPEC-AX-REVIEW-001은 SPEC-AX-SCORE-001의 점수 데이터 위에 **평가 제출/검토/승인 워크플로우 저장소 + HTTP API 계층**을 추가한다.

**핵심 결정**:
1. **4-state lifecycle**: SUBMITTED(analyst 제출) → UNDER_REVIEW(admin 검토자 할당) → APPROVED/REJECTED(terminal)
2. **score_id FK-less stub**: SCORE-001 scores.id 참조, 존재 검증은 핸들러 단계
3. **0005 migration**: score_review_requests 테이블 + CHECK constraint (상태)
4. **Recorder 통합**: entity-INSERT → audit-INSERT (동일 TX, 양방향 원자성)
5. **HTTP API 6 엔드포인트**: create/get/list/assign-reviewer/approve/reject
6. **ABAC narrowing**: analyst=submit, admin=approve/reject/assign, viewer+all=read
7. **frozen RBAC 준수**: RoleAdmin/RoleAnalyst/RoleViewer 기존 3역할, rbac.go 무수정

**파일 변경**:
- 신규 4개: `score_review_request.go` / `review_handlers.go` / `review_handlers_test.go` / `0005_*.sql`
- 수정 6개: `store.go` / `pg_store.go` / `audit.go` / `recorder.go` / `errors.go` / `server.go`

**토큰 효율**: store/audit 패턴 기존 선례 미러 → brownfield 신규 확장 최소화, TDD RED-GREEN-REFACTOR 기존 성숙 사이클 준수.
