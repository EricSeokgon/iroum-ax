# Strategy: SPEC-AX-REVIEW-001 §6 OPEN #1-#6 RESOLVED

**SPEC**: SPEC-AX-REVIEW-001 v0.1.0
**Phase**: Run — Phase 1 strategy
**Status**: **RESOLVED — Human Gate sign-off 2026-05-20 (6/6 권고안 승인)**
**Generated**: 2026-05-20
**Author**: manager-strategy
**Mode**: sub-agent
**Output target**: `.moai/specs/SPEC-AX-REVIEW-001/strategy.md` (this file)
**Precedent**: SCORE-API-001 / REPORT-001 strategy.md format
**Source-verification date**: 2026-05-20 (rbac.go L19-26 + L33, score_handlers.go L161-190 + L163 evaluator-INFEASIBLE 주석, pg_store.go L134-148 + L143-147 Recorder 주입 모두 직접 spot-verify)
**Sign-off**: ircp (user, 2026-05-20) — 6건 전부 권고안 그대로 승인

---

## A. Resolution Format (SCORE-API-001 mirror)

각 RESOLVED 항목에 다음 5요소를 부착한다:
1. **Decision** (확정 옵션)
2. **근거** (선례 file:line + 정합성)
3. **거부 대안** (왜 배제하는가)
4. **Consumer-only [HARD] 영향** (frozen 계약과의 정합)
5. **Run phase 적용** (어느 milestone/파일에서 구현)

---

## §A.1 — 검토자 할당 엔드포인트 형태 [RESOLVED]

### Decision: **Option A** — `POST /api/v1/reviews/{id}/assign-reviewer` (별도 sub-resource)

**Body**: `{"reviewer_id": "<string>"}`
**Response**: HTTP 200 OK + updated entity JSON
**Side-effect**: status `SUBMITTED → UNDER_REVIEW`, `assigned_reviewer_id = reviewer_id`, audit_logs 1건 (`SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED`)

### 근거

1. **SCORE-API-001 supersede 선례 동형**: `POST /api/v1/scores/{id}/supersede`(`score_handlers.go:64` direct-verified) — 비-멱등 상태 전이 + side-effect-with-state-change는 sub-resource로 노출. spec.md §6 #1 권장 + research.md §12.3 #1 정합.
2. **HTTP 의미론 정확**: POST = non-idempotent action. PUT은 멱등 idempotent를 의미하나 reviewer 재할당은 audit row가 누적되어 멱등 아님 → POST sub-resource가 정확.
3. **테스트/감사 분리 단순**: 핸들러 함수 `handleAssignReviewer`를 단일 책임(SRP)으로 격리. 테스트 시나리오 routing 명확.
4. **REST 표준 비위반**: REST는 "리소스의 상태를 표현하는 URI"를 권장. `/reviews/{id}/assign-reviewer`는 검토자 할당 액션 sub-resource로 해석 가능 (controller resource pattern).

### 거부 대안

| 대안 | 거부 사유 |
|------|----------|
| Option B `POST /api/v1/reviews/{id}/transition` with body `{to: "UNDER_REVIEW", reviewer_id}` | 단일 엔드포인트에 3종 전이(assign/approve/reject) 분기 → 핸들러 복잡도↑, body validation 분기↑, 감사 라우팅 복잡. SCORE-API-001 supersede 선례 아님. |
| Option C `PUT /api/v1/reviews/{id}` with body `{status: "UNDER_REVIEW", reviewer_id}` | PUT은 멱등 의미 — reviewer 재할당이 audit row 누적이라 의미 충돌. body parsing 분기 복잡. REST 표준 권장 패턴 아님. |

### Consumer-only [HARD] 영향

- **0-diff**: SCORE-001 / EVAL-ITEM-001 / EVID-001 / AUTH-003 코드 무변경 ✓
- **frozen rbac.go**: 0-diff (admin-only 게이트는 핸들러-로컬, §A.4 정합) ✓
- **신규 마이그레이션 0건**: 본 결정은 URL 형태만 — 0005 마이그레이션 무영향 ✓

### Run phase 적용

- **M2 GREEN**: `review_handlers.go` `Routes()` 메서드에 `mux.HandleFunc("POST /api/v1/reviews/{id}/assign-reviewer", h.handleAssignReviewer)` 등록 (plan.md §3.2 L187)
- **M0 RED**: `review_handlers_test.go`에서 `httptest.NewRequest("POST", "/api/v1/reviews/{id}/assign-reviewer", body)` 사용 → tasks.md T-201

---

## §A.2 — 승인/반려 엔드포인트 형태 [RESOLVED]

### Decision: **Option A** — `POST /api/v1/reviews/{id}/approve` + `POST /api/v1/reviews/{id}/reject` (별도 sub-resource each)

**Approve Body**: `{"comment": "<optional string>"}`
**Reject Body**: `{"rejection_reason": "<required non-empty string>", "comment": "<optional string>"}`
**Response**: HTTP 200 OK + updated entity JSON
**Side-effect**: status `UNDER_REVIEW → APPROVED|REJECTED` (terminal), audit_logs 1건

### 근거

1. **§A.1 결정 정합성**: §A.1에서 sub-resource 패턴 채택 — §A.2도 동일하게 적용해야 일관된 API 계약 유지. mixed 패턴(assign=sub-resource, approve/reject=PUT) 사용 시 클라이언트 혼란.
2. **의도 명확성**: `/approve`와 `/reject`는 두 액션의 의미를 URL에 명시 → 클라이언트가 의도 분명히 표현. body `{status}` 파싱보다 routing 단계 검증이 강력.
3. **테스트 분리**: `handleApprove`와 `handleReject`는 별도 책임 — reject는 `rejection_reason` non-empty 필수(REQ-REVIEW-003-E3 + §A.5), approve는 optional comment만. 핸들러 함수 분리로 validation 로직 단순.
4. **SCORE-API-001 supersede 선례**: 단일 비-멱등 mutation에 대해 sub-resource 채택한 정확 동형 패턴 (score_handlers.go:64 direct-verified).
5. **감사 routing**: `ActionScoreReviewRequestApproved` vs `ActionScoreReviewRequestRejected` 두 액션이 audit_logs에 분리 기록되어야 함 — 핸들러 단계에서 분기하면 명확.

### 거부 대안

| 대안 | 거부 사유 |
|------|----------|
| Option B `PUT /api/v1/reviews/{id}/status` with body `{status, rejection_reason?}` | (a) PUT 멱등 의미 충돌 (terminal 전이는 1회만 가능), (b) body 분기 복잡(status enum → audit 액션 매핑), (c) §A.1과 inconsistent. |
| Option C 단일 `POST /api/v1/reviews/{id}/decision` with body `{decision: "approve"\|"reject", rejection_reason?}` | enum 분기 단일 핸들러로 묶어도 audit/validation 분기 동일 — 분리의 이점 손실 (handler-local 분기 vs URL 분기 차이만). |

### Consumer-only [HARD] 영향

- **0-diff**: 동일 (URL 형태만) ✓
- **§A.1 결정과 일관성**: sub-resource 패턴 통일 → 클라이언트 SDK 생성 시 패턴 학습 비용 최소 ✓

### Run phase 적용

- **M2 GREEN**: `Routes()`에 `mux.HandleFunc("POST /api/v1/reviews/{id}/approve", h.handleApprove)` + `mux.HandleFunc("POST /api/v1/reviews/{id}/reject", h.handleReject)` 등록 (plan.md §3.2 L188-189)
- **M0 RED**: `TestPOST_ReviewsApprove_AdminSucceeds_200` / `TestPOST_ReviewsReject_RejectionReasonMissing_400` (plan.md §4.1 #17/#18) → tasks.md T-202, T-203

---

## §A.3 — Cross-store 점수 존재 검증 [RESOLVED]

### Decision: **Option A** — handler-compose 2-TX

**구현**:
1. **TX-1 (read-only)**: `scoreStore.BeginScoreTx(ctx)` → `scoreTx.GetScoreByID(scoreID)` → `scoreTx.Rollback(ctx)` (read-only이므로 Rollback). score 미존재 시 핸들러는 HTTP 404 `NOT_FOUND` 응답.
2. **TX-2 (write)**: `reviewStore.BeginScoreReviewRequestTx(ctx)` → `reviewTx.InsertScoreReviewRequest(ctx, scoreID, ...)` → audit_logs 1건 (동일 TX) → `reviewTx.Commit(ctx)`.

**Race window 처리**: TX-1과 TX-2 사이에 score가 삭제될 가능성은 SCORE-001 §1.4 + spec.md §1.4 [HARD]에 의해 결정적으로 **0건** — SCORE-001은 물리 DELETE 0(append-only, SUPERSEDED만, research.md §3.1/§10.2). race 不發生 → PoC 수용.

### 근거

1. **REPORT-001 §6 OPEN #1 Option A 선례 정확 미러**: SPEC-AX-REPORT-001의 cross-store 2-TX 패턴이 ground-truth 검증된 precedent. spec.md §6 #3 권장 + research.md §12.3 #3 정합.
2. **Consumer-only [HARD] 자연 충족**: Option B (store-layer 단일 TX 통합)는 `ScoreReviewRequestTx`가 `ScoreTx`의 `GetScoreByID`를 호출 가능해야 하나, 이는 (a) `store.go` 인터페이스 cross-coupling 도입 → store domain isolation 위반, 또는 (b) `pg_store.go`에서 두 store 직접 결합 → consumer-only 위반. Option A는 핸들러 layer에서 깨끗하게 분리.
3. **신규 store 메서드 0**: SCORE-001 측은 `BeginScoreTx` / `GetScoreByID`(이미 존재, `store.go:253-255/275-276` source-verified) 호출만 → 0-diff. spec.md §2.2 [EXISTING] 명시.
4. **TX 분리의 합리성**: 점수 조회는 read-only, 검토 요청 INSERT는 write — isolation level이 다를 수 있고 lock 정책이 다름. 단일 TX 통합 시 read-only 락이 write TX 전체 lifetime 동안 지속될 수 있어 비효율.
5. **race 결정성 명시**: spec.md §6 #3에 "SCORE-001 물리 삭제 0 → race 不發生" 명시 trade-off + plan.md §5 R-CROSS-STORE-001 위험 항목에 "PoC 수용 — append-only" 정합.

### 거부 대안

| 대안 | 거부 사유 |
|------|----------|
| Option B store-layer 단일 TX 통합 (cross-store interface coupling) | (a) `ScoreReviewRequestTx` 인터페이스에 `GetScoreByID` 메서드 추가 → store domain isolation 위반, (b) 또는 `pg_store.go`의 `BeginScoreReviewRequestTx`가 `scores` 테이블 직접 SELECT → SCORE-001 store layer bypass = consumer-only [HARD] 위반. **infeasible**. |
| Option C 핸들러에서 단일 TX로 양쪽 동작 | pgx.Tx 공유 시 두 store 구조체가 동일 TX를 공유해야 함 — Go interface로 표현 시 cross-store coupling 발생. 결과적으로 B와 동일 문제. |
| Option D race 완벽 방지를 위한 distributed lock / advisory lock | over-engineering. SCORE-001이 물리 삭제 0이므로 race 불가 — PoC 수용 가능. |

### Consumer-only [HARD] 영향

- **0-diff**: SCORE-001 `BeginScoreTx` / `GetScoreByID`는 호출만, 코드 무변경 ✓
- **store.go**: 신규 `ScoreReviewRequestStore` 추가, 기존 `ScoreStore` 무변경 ✓
- **pg_store.go**: 신규 `BeginScoreReviewRequestTx` 메서드 추가, 기존 `BeginScoreTx` 무변경 ✓
- **Race trade-off 명시**: spec.md §6 #3 + plan.md §5 R-CROSS-STORE-001에 PoC 수용 trade-off 문서화 완료 ✓

### Run phase 적용

- **M2 GREEN**: `review_handlers.go::handleCreateReview`에서 다음 시퀀스 구현
  1. score 존재 검증: `scoreTx, _ := h.scoreStore.BeginScoreTx(ctx)` → `_, err := scoreTx.GetScoreByID(scoreID)` → `scoreTx.Rollback(ctx)` (read-only, defer)
  2. err == ErrScoreNotFound → HTTP 404
  3. 검토 요청 생성: `reviewTx, _ := h.reviewStore.BeginScoreReviewRequestTx(ctx)` → `id, err := reviewTx.InsertScoreReviewRequest(...)` → `reviewTx.Commit(ctx)`
- **NewReviewHandler 시그니처**: `NewReviewHandler(rs store.ScoreReviewRequestStore, ss store.ScoreStore, logger *zap.Logger) *ReviewHandler` (plan.md §3.2 L180-182 정합)
- **M0 RED**: `TestPOST_Reviews_ScoreNotExist_404` (plan.md §4.1 #15) → tasks.md T-204

---

## §A.4 — 검토자 역할 매핑 [RESOLVED]

### Decision: **Option A** — admin-only for approve/reject/assign-reviewer (handler-local mapping, frozen rbac.go 0-diff)

**매핑 (핸들러-로컬)**:
| 작업 | 역할 게이트 | 함수 |
|------|------------|------|
| 제출 (`POST /api/v1/reviews`) | `RoleAnalyst` 또는 `RoleAdmin` | `requireReviewSubmitRole` / `guardReviewSubmit` |
| 검토자 할당 (`POST /{id}/assign-reviewer`) | `RoleAdmin` only | `requireReviewAdminRole` / `guardReviewAdmin` |
| 승인 (`POST /{id}/approve`) | `RoleAdmin` only | `requireReviewAdminRole` / `guardReviewAdmin` |
| 반려 (`POST /{id}/reject`) | `RoleAdmin` only | `requireReviewAdminRole` / `guardReviewAdmin` |
| 단건 조회 (`GET /{id}`) | 모든 인증 사용자 (viewer 포함) | (게이트 없음, ABAC narrowing-only) |
| 목록 (`GET /api/v1/reviews`) | 모든 인증 사용자 | (게이트 없음) |
| auth-disabled 투과 | 모든 엔드포인트 | guard 함수에서 `ok=false` 시 true 반환 |

**`assigned_reviewer_id` 컬럼**: 실제 검토자의 사용자 식별자(string) 기록. ABAC 결정과 무관 (정보/감사 추적용). 본 PoC에서는 admin이 자기 자신 또는 다른 admin을 할당할 수 있음 — 역할 분리 강제는 post-PoC.

### 근거

1. **frozen rbac.go 0-diff [HARD] (source-verified)**:
   - rbac.go L19-26: `RoleAdmin = "admin"`, `RoleAnalyst = "analyst"`, `RoleViewer = "viewer"` 3-role exhaustive (direct-verified 2026-05-20)
   - rbac.go L33: `^iroum-ax:(admin|analyst|viewer)$` 정규식 (direct-verified) — `evaluator` / `reviewer` 토큰 매칭 0
   - 신규 `RoleReviewer` 도입 시도 시 (a) 상수 추가 → rbac.go 수정 위반, (b) 정규식 확장 → frozen 위반, (c) `permissionMatrix` 추가 → frozen 위반. **infeasible**.
2. **SCORE-API-001 OPEN #4 evaluator-INFEASIBLE 회피 (source-verified)**:
   - score_handlers.go L161-190 direct-verified: `requireScoreWriteRole`이 `RoleAdmin || RoleAnalyst` 직접 판정, frozen rbac.go 0-diff
   - score_handlers.go L163 주석 direct-verified: "(evaluator는 rbac.go:33 정규식 부재로 INFEASIBLE → RoleAnalyst 대체, strategy.md §A #4)"
   - 본 SPEC은 역할 매핑이 (analyst=제출 / admin=승인·반려·할당 / viewer=조회) — 모두 frozen rbac.go에 존재 → **자연 0-diff 성립, SCORE-API-001 충돌 NOT recur** ✓
3. **핸들러-로컬 ABAC narrowing 패턴 (source-verified)**:
   - score_handlers.go L161-190 direct-verified: `requireScoreWriteRole` / `guardScoreWrite` 동형 패턴 본 SPEC에서 재사용
   - `requireReviewSubmitRole` = `RoleAdmin || RoleAnalyst` 체크 (제출 게이트)
   - `requireReviewAdminRole` = `RoleAdmin` only 체크 (관리 게이트)
   - `guardReviewSubmit` / `guardReviewAdmin` — auth-disabled 시 투과(true), 권한 없음 시 403 `ErrCodeABACDenied`(`abac.go:24`)
4. **`assigned_reviewer_id` 의미 분리**: ABAC 결정 ≠ 정보 컬럼. `assigned_reviewer_id`는 감사 추적용 정보로 어떤 admin이 누구를 할당했는지 기록. 권한 게이트는 role-based(admin)만.
5. **spec.md §1.5 + §6 #4 + research.md §6.3 권장 정합**.
6. **SCORE-API-001 / REPORT-001 패턴 동형**: 두 SPEC 모두 핸들러-로컬 게이트로 frozen rbac.go 0-diff 달성 — 본 SPEC도 정확 미러.

### 거부 대안

| 대안 | 거부 사유 |
|------|----------|
| Option B `assigned_reviewer_id == principal.id` 일치 시에만 승인/반려 허용 (assigned-reviewer ABAC narrowing) | (a) `abac.go` narrowing 정책 추가 필요 → frozen ABAC 위반 가능성, (b) cli-anonymous 모드에서 principal.id가 'cli-anonymous'이므로 `assigned_reviewer_id`도 'cli-anonymous'일 때만 가능 → auth-disabled Walking Skeleton에서 의미 손실, (c) admin 위임/대리 승인 시나리오 차단 — PoC 운영 유연성 ↓. post-PoC에서 풀 org-unit ABAC 도입 시 재검토 가능 (spec.md §5 Exclusion #6). |
| Option C `RoleReviewer` 신규 도입 | **INFEASIBLE** — rbac.go L33 정규식 부재, frozen [HARD] 위반. SCORE-API-001 OPEN #4와 동일 충돌 발생. |
| Option D `permissionMatrix`에 `write:review` permission 추가 | frozen permissionMatrix 수정 → AUTH-003 frozen 계약 위반. 핸들러-로컬 매핑이 0-diff로 동일 효과 달성 가능. |

### Consumer-only [HARD] 영향

- **frozen rbac.go**: 0-diff (RoleAdmin/RoleAnalyst 기존 상수만 호출, 상수/정규식/permissionMatrix 무변경) ✓
- **frozen abac.go**: 0-diff (narrowing-only/admin 우회/auth-disabled 투과 기존 정책만 사용) ✓
- **frozen middleware**: 0-diff (RESTAuthzMiddleware 체인 자동 적용, 변경 없음) ✓
- **plan-auditor OPEN #4 비-재발 검증**: plan-audit.md §3.5 OPEN#4 SOUND 평가 + source-verified ✓

### Run phase 적용

- **M2 GREEN**: `review_handlers.go`에 다음 함수 신설 (plan.md §3.2 L199-228 정합)
  - `requireReviewSubmitRole(scope string) bool` — `RoleAdmin || RoleAnalyst`
  - `requireReviewAdminRole(scope string) bool` — `RoleAdmin` only
  - `(h *ReviewHandler) guardReviewSubmit(w, r) bool` — auth-disabled 투과, 미인가 403
  - `(h *ReviewHandler) guardReviewAdmin(w, r) bool` — 동일 패턴
- **M0 RED**: 다음 테스트 (plan.md §4.1, tasks.md T-301 ~ T-306)
  - `TestPOST_Reviews_AnalystCreates_201` (#13)
  - `TestPOST_Reviews_ViewerForbidden_403` (#14)
  - `TestPOST_ReviewsApprove_AnalystForbidden_403` (#16)
  - `TestPOST_ReviewsApprove_AdminSucceeds_200` (#17)
  - `TestPOST_ReviewsAssignReviewer_AnalystForbidden_403` (#19)
  - `TestGET_Reviews_AuthDisabled_PassthroughCliAnonymous` (#20)
- **PoC trade-off 명시**: "모든 admin이 모든 검토 승인 가능 (역할 분리 X) — PoC 수용, post-PoC 풀 org-unit ABAC 도입 시 재검토" (spec.md §6 #4 + §5 Exclusion #6 정합)

---

## §A.5 — Reason/Comment 필드 작성 조건 (이중 방어) [RESOLVED]

### Decision: **Option B + Option A** — pre-store handler validation + DB CHECK constraint (defense-in-depth)

**Layer 1 — Pre-store handler validation** (Option B, fail-friendly):
- `handleReject`에서 body parsing 직후 `rejection_reason` non-empty 검증
- empty/missing 시 `ErrScoreReviewRequestInvalidInput` 반환 → HTTP 400 `INVALID_ARGUMENT` + 한국어 메시지 "반려 사유는 필수입니다"
- `validateReviewRequestInput`에서도 동일 검증 (SQL 미실행 후 거부, SCORE-001 `validateScoreInput` `score.go:79-97` 동형 패턴)

**Layer 2 — DB CHECK constraint** (Option A, fail-safe):
- `0005_score_review_request_tables.sql`에 CHECK 제약 추가 (plan.md §3.3 L257-261 SQL 정확 미러):

  ```sql
  DO $$ BEGIN
      ALTER TABLE score_review_requests
          ADD CONSTRAINT score_review_requests_reject_reason_chk
          CHECK ((status != 'REJECTED') OR (rejection_reason IS NOT NULL AND length(rejection_reason) > 0));
  EXCEPTION WHEN duplicate_object THEN NULL; END $$;
  ```

- 멱등 패턴 (DO$$ EXCEPTION duplicate_object, 0004 미러)

**`comment` 컬럼**: 모든 상태에서 NULL 허용 (opaque optional, 모든 상태 적용 — spec.md research.md §3.2 + §12.3 #5 정합)

### 근거

1. **친화적 한국어 에러 메시지 (UX)**: 클라이언트가 빈 reason으로 POST 시 400 + "반려 사유는 필수입니다" 한국어 응답이 즉시 반환되어 API 사용자가 디버깅 용이. DB CHECK만 사용 시 generic `CONSTRAINT VIOLATION` 또는 500 INTERNAL로 노출되어 한국어 친화성 손실.
2. **Fail-safe 최후 방어 (DB CHECK)**: 핸들러 우회 경로 (예: 직접 SQL, 향후 다른 store 메서드 추가) 시에도 invariant 보호. EVAL-ITEM-001 `validateStatusTransition` 이중 방어 선례 동형.
3. **SCORE-001 `validateScoreInput` 동형 (`score.go:79-97`)**: SQL 미실행 후 거부 — fail-closed 패턴. 본 SPEC `validateReviewRequestInput`도 정확 미러.
4. **spec.md §6 #5 + research.md §12.3 #5 권장 정합**.
5. **plan-audit.md OPEN #5 SOUND 평가**: "Option A+B 이중 방어 — pre-store validation으로 친화적 한국어 에러 + DB CHECK fail-safe. EVAL-ITEM-001 선례 동형. plan.md §3.3 SQL에 CHECK 명시 ✓ (L257-261)"

### 거부 대안

| 대안 | 거부 사유 |
|------|----------|
| Option A 단독 (DB CHECK만) | (a) 한국어 친화 에러 손실, (b) 클라이언트가 400 vs 500 구분 불가 (CONSTRAINT VIOLATION이 어떤 HTTP status로 매핑되는지 결정적이지 않음), (c) handler 단계 fail-fast 손실 — 잘못된 reject body로 TX 진입 후 SQL 단계에서 실패 → TX overhead 발생. |
| Option B 단독 (handler validation만) | DB constraint 부재 → 향후 다른 store method 추가 / 직접 SQL 우회 가능성 → invariant 보호 약화. EVAL-ITEM-001 이중 방어 선례 약화. |
| Option C `comment`도 reject 시 필수 | over-spec. `comment`는 모든 상태에서 optional이 사용 편의성 ↑ (research.md §3.2 정합). REJECTED 시 `rejection_reason`만 필수면 충분. |

### Consumer-only [HARD] 영향

- **0-diff**: SCORE-001 / EVAL-ITEM-001 / EVID-001 / AUTH-003 코드 무변경 ✓
- **0005 마이그레이션**: 본 SPEC에서 신규 (NEW), CHECK 제약 추가는 본 SPEC 범위 내 ✓
- **`validateReviewRequestInput`**: 본 SPEC 신규 함수, SCORE-001 `validateScoreInput` 동형 패턴 사용 ✓

### Run phase 적용

- **M1 GREEN**:
  1. `score_review_request.go`에 `validateReviewRequestInput(scoreID, rejection_reason, status)` 신설 (REJECTED 상태일 때 reason non-empty 검증)
  2. `0005_score_review_request_tables.sql`에 `score_review_requests_reject_reason_chk` CHECK constraint 추가 (멱등 DO$$ 패턴, plan.md §3.3 L257-261 정확 미러)
- **M2 GREEN**: `handleReject`에서 body parsing 후 `validateReviewRequestInput` 호출 → 400 INVALID_ARGUMENT 매핑
- **M0 RED**:
  - `TestRejectRequest_FromUnderReview_NoReason_ReturnsInvalidInput` (store-layer, plan.md §4.1 #9) → tasks.md T-105
  - `TestPOST_ReviewsReject_RejectionReasonMissing_400` (handler-layer, plan.md §4.1 #18) → tasks.md T-203
- **에러 매핑** (`mapReviewStoreErr`): `ErrScoreReviewRequestInvalidInput` → 400 `INVALID_ARGUMENT` + "반려 사유는 필수입니다" 또는 "평가 검토 입력이 유효하지 않습니다"

---

## §A.6 — Concurrent Transition Handling [RESOLVED]

### Decision: **Option A** — `SELECT ... FOR UPDATE` row lock (pessimistic) + state-machine 가드 (이중 방어)

**구현 (assign/approve/reject 3 메서드 모두 동일 패턴, plan.md §3.1 L143-148 정합)**:

  ```
  // 1. Row lock (transaction-scoped)
  const lockSQL = `SELECT status FROM score_review_requests WHERE id = $1 FOR UPDATE`
  var currentStatus string
  err := t.tx.QueryRow(ctx, lockSQL, id).Scan(&currentStatus)
  if err != nil { ... ErrScoreReviewRequestNotFound 매핑 ... }

  // 2. State-machine guard (validateReviewStatusTransition)
  if err := validateReviewStatusTransition(currentStatus, targetStatus); err != nil {
      return err // ErrScoreReviewRequestInvalidStatus / NotSubmitted / NotUnderReview
  }

  // 3. UPDATE + audit (동일 TX)
  _, err = t.tx.Exec(ctx, updateSQL, ...)
  ... t.recorder.RecordScoreReviewRequest<Action>(...) ...
  ```

**Lock lifetime**: pgx.Tx 범위 — `Commit` 또는 `Rollback` 시 자동 해제. 두 admin 동시 승인 시: 첫 번째 TX가 row 점유 → 두 번째 TX는 BLOCK → 첫 번째 Commit 후 두 번째가 lock 획득 → `currentStatus = 'APPROVED'`(terminal) → `validateReviewStatusTransition` 거부 → `ErrScoreReviewRequestNotUnderReview` → HTTP 409 응답.

**`version` 컬럼 추가 0**: 0005 마이그레이션에 version 컬럼 미포함 (optimistic 거부, 스키마 단순 유지).

### 근거

1. **EVID-001 `GetLatestVersionByEvalItem` `store.go:92-95` 패턴 (research.md §12.3 #6 + plan.md §3.1 L143 인용)**: SELECT FOR UPDATE row lock의 SCORE 도메인 선례. EVID-001이 동일 패턴으로 동시성 검증 통과.
2. **state-machine 이중 방어 (SCORE-001 `validateScoreStatusTransition` `score.go` 동형)**: lock 후 `validateReviewStatusTransition` 가드 실행 — terminal 상태(APPROVED/REJECTED)나 잘못된 전이 시도는 SQL UPDATE 전 거부.
3. **추론 단순성 (pessimistic 우위)**: optimistic concurrency(version 컬럼 + CAS UPDATE)는 (a) `version BIGINT NOT NULL` 컬럼 추가 → 0005 마이그레이션 스키마 ↑, (b) UPDATE에 `WHERE id=$1 AND version=$2` 추가 → SQL 복잡도 ↑, (c) 호출자 retry loop 필요 — PoC scope 비대화. pessimistic 락은 추론 명확 + 코드 단순.
4. **결정성 (race 0)**: SELECT FOR UPDATE는 PostgreSQL row-level lock으로 결정적 직렬화. 두 admin 시도 시 정확히 한 명만 성공, 다른 한 명은 409 (테스트 결정적, `TestConcurrentAdminApprove_SelectForUpdate_OnlyFirstSucceeds` plan.md §4.1 #11).
5. **TX scoped 락 안전성**: TX commit/rollback 시 자동 해제 — deadlock 방지. Lock leak 0 (defer Rollback 패턴 사용 시).
6. **spec.md §6 #6 + research.md §12.3 #6 권장 정합**.
7. **plan-audit.md OPEN #6 SOUND 평가**: "Option A `SELECT ... FOR UPDATE` 명시적 row lock + D4-style mutation guard(`current_status` 비교) 이중 방어(EVID-001 + SCORE-001 패턴 동형, research.md §12.3 #6). version 컬럼 추가 없이도 결정적."

### 거부 대안

| 대안 | 거부 사유 |
|------|----------|
| Option B optimistic concurrency (version 컬럼 + CAS) | (a) 0005에 `version BIGINT NOT NULL DEFAULT 0` 컬럼 추가 → 스키마 복잡↑, (b) 모든 UPDATE에 `WHERE version=current_version` + RETURNING 추가 → SQL 복잡↑, (c) 0 rows affected 시 retry loop 또는 conflict error 매핑 → 핸들러 복잡↑. PoC over-engineering. |
| Option C 단순 state-machine 가드 (lock 없음) | TOCTOU race — TX-A가 status='UNDER_REVIEW' 확인 후 UPDATE 직전 TX-B가 APPROVED 처리하면 둘 다 성공 (또는 PostgreSQL transaction isolation에 따라 last-write-wins 발생). 결정성 0. |
| Option D distributed lock (Redis 등) | (a) 신규 외부 의존 도입 → REQ-REVIEW-UBI-001 (데이터 주권) 위반, (b) over-engineering. |

### Consumer-only [HARD] 영향

- **0-diff**: SCORE-001 / EVAL-ITEM-001 / EVID-001 / AUTH-003 코드 무변경 ✓
- **EVID-001 패턴 호출 0건**: EVID-001 코드 직접 호출 없이 동형 SQL 패턴만 본 SPEC `score_review_request.go`에 신설 ✓
- **0005 마이그레이션**: `version` 컬럼 0 (optimistic 거부) → 스키마 단순 유지 ✓
- **신규 외부 의존 0건**: 신규 lock 라이브러리 / Redis 등 0 — `REQ-REVIEW-UBI-001` 데이터 주권 정합 ✓

### Run phase 적용

- **M1 GREEN**: `score_review_request.go`의 `AssignReviewer` / `ApproveRequest` / `RejectRequest` 3 메서드 모두 SELECT FOR UPDATE → validateReviewStatusTransition → UPDATE → RecordScoreReviewRequest* 시퀀스 (plan.md §3.1 L143-148 정확 미러)
- **`validateReviewStatusTransition`** (plan.md §3.1 L154-168 정합):

  ```
  var allowedReviewTransitions = map[string][]string{
      "SUBMITTED":    {"UNDER_REVIEW"},
      "UNDER_REVIEW": {"APPROVED", "REJECTED"},
      "APPROVED":     {},  // terminal
      "REJECTED":     {},  // terminal
  }
  ```

- **M0 RED**:
  - `TestConcurrentAdminApprove_SelectForUpdate_OnlyFirstSucceeds` (plan.md §4.1 #11) → tasks.md T-106 — goroutine 2개 동시 승인, 한 명 200 / 한 명 409 결정적 검증 (acceptance.md AC-REVIEW-003-2 / E10)
  - `TestAssignReviewer_FromUnderReview_ReturnsNotSubmitted` (plan.md §4.1 #5) → tasks.md T-103
  - `TestApproveRequest_FromSubmitted_ReturnsNotUnderReview` (plan.md §4.1 #7) → tasks.md T-104
  - `TestStatusTransition_FromTerminal_AlwaysReturnsInvalidStatus` (plan.md §4.1 #10) → tasks.md T-107
- **에러 매핑** (`mapReviewStoreErr`):
  - `ErrScoreReviewRequestInvalidStatus` → 409 `CONFLICT` + "허용되지 않은 평가 검토 상태 전이입니다"
  - `ErrScoreReviewRequestNotSubmitted` → 409 `CONFLICT` + "검토자 할당은 SUBMITTED 상태의 평가 검토에만 허용됩니다"
  - `ErrScoreReviewRequestNotUnderReview` → 409 `CONFLICT` + "승인/반려는 UNDER_REVIEW 상태의 평가 검토에만 허용됩니다"

---

## B. Resolution Summary Table (Human Gate sign-off 2026-05-20)

| OPEN | Decision (Option) | 근거 (선례 file:line) | 거부 대안 | Consumer-only 영향 |
|------|--------------------|----------------------|-----------|--------------------|
| **#1** | **A** — `POST /reviews/{id}/assign-reviewer` (별도 sub-resource) | SCORE-API-001 supersede `score_handlers.go:64` 동형, REST 비-멱등 sub-resource 패턴 | B (PUT body status), C (`/transition` 통합) — REST 의미 충돌 / 핸들러 복잡↑ | 0-diff [HARD] (URL 형태만, 마이그레이션 무영향) |
| **#2** | **A** — `POST /approve` + `POST /reject` (별도 sub-resource each) | #1 결정 일관성, supersede 선례, audit routing 분리 | B (`PUT /status`), C (`/decision` 통합) — PUT 멱등 충돌 / body 분기 복잡 | 0-diff [HARD] (URL 형태만) |
| **#3** | **A** — handler-compose 2-TX (`BeginScoreTx`+`GetScoreByID` → `BeginScoreReviewRequestTx`+`Insert`) | REPORT-001 §6 OPEN#1 cross-store 2-TX 선례 정확 미러, `store.go:253-255/275-276` 호출만 | B (store-layer 단일 TX 통합) **INFEASIBLE** — cross-store coupling = consumer-only 위반 | 0-diff [HARD]. Race window는 SCORE-001 물리 삭제 0(append-only)이라 결정적 不發生 — PoC 수용 trade-off 명시 |
| **#4** | **A** — admin-only for approve/reject/assign (핸들러-로컬 매핑) | rbac.go L19-26 + L33 frozen 3-role + score_handlers.go L161-190 (`requireScoreWriteRole` 동형, L163 evaluator-INFEASIBLE 주석) source-verified | B (`assigned_reviewer_id == principal.id` ABAC), C (`RoleReviewer` 신설) — **C INFEASIBLE** (rbac.go:33 정규식 부재), B (abac.go 수정 + 위임 시나리오 차단) | frozen rbac.go / abac.go / permissionMatrix 0-diff [HARD]. SCORE-API-001 OPEN#4 evaluator 충돌 자연 회피 (admin/analyst 모두 frozen rbac.go에 존재) |
| **#5** | **B + A 이중 방어** — pre-store handler validation + DB CHECK constraint | EVAL-ITEM-001 이중 방어 + SCORE-001 `validateScoreInput` `score.go:79-97` 동형 + 한국어 친화 에러 + fail-safe | A 단독 (CHECK만, 친화 에러 손실), B 단독 (DB 보호 약화) | 0-diff [HARD]. `0005` 신규 마이그레이션에 CHECK 추가 (본 SPEC 범위 내), `validateReviewRequestInput` 신규 함수 |
| **#6** | **A** — `SELECT ... FOR UPDATE` row lock + state-machine 가드 (pessimistic 이중 방어) | EVID-001 `GetLatestVersionByEvalItem` `store.go:92-95` 패턴, SCORE-001 `validateScoreStatusTransition` 동형 | B (optimistic version + CAS, 스키마+코드 복잡↑), C (lock 없음, TOCTOU race), D (distributed Redis lock, REQ-REVIEW-UBI-001 데이터 주권 위반) | 0-diff [HARD]. `version` 컬럼 0(스키마 단순). 신규 외부 의존 0건. TX scoped lock(commit/rollback 자동 해제) |

---

## C. Cross-cutting Impact Verification

### C.1 Consumer-only [HARD] 무위반 검증

본 6 RESOLVED 결정은 모두 다음 [HARD] 계약을 위반하지 않는다:

- ✅ SCORE-001 `score.go`, `pg_store.go(BeginScoreTx 부분)`, `0004_score_tables.sql` 0-diff
- ✅ EVAL-ITEM-001 `eval_item.go` 0-diff
- ✅ EVID-001 `evidence.go` 0-diff
- ✅ AUTH-003 `rbac.go`, `abac.go`, `authz_middleware.go`, `chain.go`, `middleware.go` 0-diff
- ✅ SCORE-API-001 `score_handlers.go` 0-diff (패턴 미러만, 코드 호출 없음)
- ✅ REPORT-001 `report_handlers.go` 0-diff
- ✅ 기존 마이그레이션 `0001`~`0004` 0-diff
- ✅ `go.mod` / `go.sum` 0-diff (신규 외부 의존 0)

### C.2 frozen RBAC 0-diff 검증 (§A.4 핵심)

| 항목 | 상태 | source-verify |
|------|------|--------------|
| `rbac.go:19-26` 3-role 상수 (admin/analyst/viewer) | 무변경 | 2026-05-20 direct-verified |
| `rbac.go:33` 정규식 `^iroum-ax:(admin\|analyst\|viewer)$` | 무변경 | 2026-05-20 direct-verified |
| `rbac.go` `permissionMatrix` | 무변경 | spec.md §2.2 / plan-audit.md §3.3 |
| `rbac.go` `ParseRolesFromScope` `:68-80` | 무변경 | spec.md §2.2 |
| 신규 `RoleReviewer` / `evaluator` 도입 | **0건 (금지)** | SCORE-API-001 OPEN#4 lesson 정합 |

### C.3 신규 외부 의존 0건 (REQ-REVIEW-UBI-001 정합)

- 6 RESOLVED 결정 모두 기존 의존(`pgx/v5`, `google/uuid`, `zap`)만 사용
- distributed lock 라이브러리 / Redis / 외부 SDK 0건 (§A.6 거부 D)
- LLM / 외부 알림 / 외부 검증 0건 (spec.md §5 Exclusion 정합)

### C.4 한국 공공 6제약 정합

| 제약 | 정합성 | 근거 |
|------|--------|------|
| 데이터 주권 | ✅ | 신규 외부 호출 0 (UBI-001) — 모든 RESOLVED 결정이 내부 PostgreSQL 단일 풀만 사용 |
| 감사 가능성 | ✅ | §A.1/§A.2 sub-resource 모두 audit_logs 1건 동일-TX 기록 (UBI-002) |
| 한국어 친화 | ✅ | §A.5 이중 방어가 한국어 에러 메시지 보장 (REQ-REVIEW-005-E1 정합) |
| 망분리 | ✅ | §A.6 pessimistic lock이 내부 PostgreSQL row lock만 사용 — 외부 lock 서비스 0 |
| 조직 격리 | ✅ | §A.4 admin-only 매핑이 운영 권한 경계 명확 (post-PoC org-unit ABAC 확장 가능, Exclusion #6) |
| 시간 제약 | N/A (의도적 제외) | spec.md §5 Exclusion #9 정합 — 본 SPEC 범위 외 |

### C.5 plan-audit.md 정합 검증

| plan-audit.md §3.5 평가 | 본 strategy.md 결정 | 정합 |
|------------------------|---------------------|------|
| OPEN #1 SOUND | Option A 채택 | ✓ |
| OPEN #2 SOUND | Option A 채택 | ✓ |
| OPEN #3 SOUND (race 결정성 명시) | Option A 채택 + race trade-off 문서화 | ✓ |
| OPEN #4 SOUND (비-재발 source-verified) | Option A 채택 + source-verified 추가 | ✓ |
| OPEN #5 SOUND (이중 방어) | Option B+A 채택 | ✓ |
| OPEN #6 SOUND (version 컬럼 추가 0) | Option A 채택 + version 거부 명시 | ✓ |

---

## D. Risk Updates (plan.md §5 보강)

본 strategy.md RESOLVED로 다음 위험 항목이 명확화된다:

| 위험 ID | 상태 | 본 strategy.md 결정의 영향 |
|--------|------|---------------------------|
| R-CONSUMER-001 (drift) | 명확 | 6 RESOLVED 모두 0-diff 달성, M5 Drift-Guard 검증 단계 그대로 적용 |
| R-PHANTOM-001 (시그니처) | 명확 | 모든 신규 함수/메서드 시그니처 spec.md §2.1 + plan.md §3 정합 |
| R-MIGRATION-001 (멱등) | 명확 | 0005에 CHECK 제약 추가 시 멱등 DO$$ 패턴 유지 (§A.5) |
| R-RBAC-001 (frozen 위반) | **추가 보강** | §A.4 admin-only 매핑으로 RoleReviewer 신설 위험 자연 차단 (rbac.go L33 정규식 source-verified) |
| R-CROSS-STORE-001 (race) | **추가 보강** | §A.3 race window는 SCORE-001 물리 삭제 0이라 결정적 不發生 — PoC 수용 명시 |
| R-CONCURRENT-001 (동시 승인) | **추가 보강** | §A.6 SELECT FOR UPDATE + state-machine 가드 이중 방어로 결정성 보장 |
| R-DRIFT-MANIFEST-001 (errors.go) | 명확 | §A.5 새로운 센티넬 0건 (기존 6 센티넬로 충분, errors.go [MODIFY] 명세 변경 없음) |
| R-SERVER-MOUNT-001 (≈7줄) | 명확 | §A.1/§A.2 sub-resource 결정이 server.go 마운트 (`/api/v1/reviews` + `/api/v1/reviews/` 2줄)만 영향, ≈7줄 유지 |
| R-OPEN-RESOLUTION-001 (미결정) | **해소** | 본 strategy.md로 6 OPEN 모두 RESOLVED → Human Gate sign-off 2026-05-20 완료 → M0 RED 진입 가능 |

---

## F. Sign-off Record

**Sign-off**: ircp (user)
**Sign-off Date**: 2026-05-20
**Sign-off Channel**: AskUserQuestion (orchestrator)
**Outcome**: 6/6 권고안 그대로 승인 (alternative 선택 0)
**Result**: 6 OPEN → 6 RESOLVED, spec.md §6 / plan.md §6.3 / acceptance.md OPEN 언급 / tasks.md §0 모두 정합 유지

---

## G. Source Citation (audit-grade traceability)

본 strategy.md의 모든 결정 근거는 다음 source-verified citation을 기반으로 한다 (자기인증 범위: 본 SPEC 작성 시점에 직접 spot-verify된 라인만 기재; phantom 0):

| 인용 | file:line | 검증 |
|------|-----------|------|
| frozen RBAC 3-role | `apps/control-plane/internal/auth/rbac.go:19-26` | 2026-05-20 direct-verified |
| frozen scope 정규식 | `apps/control-plane/internal/auth/rbac.go:33` | 2026-05-20 direct-verified |
| handler-local ABAC 패턴 | `apps/control-plane/cmd/server/score_handlers.go:161-190` | 2026-05-20 direct-verified |
| evaluator-INFEASIBLE 주석 (lesson) | `apps/control-plane/cmd/server/score_handlers.go:163` | 2026-05-20 direct-verified |
| BeginScoreTx Recorder 주입 | `apps/control-plane/internal/store/pg_store.go:134-148` (특히 L143-147) | 2026-05-20 direct-verified |
| supersede sub-resource 선례 | `apps/control-plane/cmd/server/score_handlers.go:64` | spec.md §6 #1 + research.md §12.3 #1 |
| EVID-001 SELECT FOR UPDATE | `apps/control-plane/internal/store/store.go:92-95` (참조) | research.md §12.3 #6 |
| SCORE-001 score.go validateScoreInput | `apps/control-plane/internal/store/score.go:79-97` | research.md §11 |
| SCORE-001 errors.go 센티넬 패턴 | `apps/control-plane/internal/errors/errors.go:52-78` | spec.md §2.1 + plan-audit.md §3.4 |
| REPORT-001 cross-store 2-TX 선례 | `.moai/specs/SPEC-AX-REPORT-001/spec.md` §6 OPEN#1 | research.md §12.3 #3 |
| plan-audit.md §3.5 OPEN SOUND 평가 | `.moai/specs/SPEC-AX-REVIEW-001/plan-audit.md:155-166` | iteration 1 PASS |

---

**End of strategy (RESOLVED, Human Gate sign-off 2026-05-20).**
