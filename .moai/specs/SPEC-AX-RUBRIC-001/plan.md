# Plan: SPEC-AX-RUBRIC-001 등급 rubric 확장 시스템 — 3-tier Rubric+Criteria+Bands Store + HTTP API

**SPEC**: SPEC-AX-RUBRIC-001 v0.1.0 (draft)
**Phase**: Plan
**Generated**: 2026-05-20
**Author**: ircp
**Methodology**: TDD (RED-GREEN-REFACTOR)
**Harness Level**: thorough
**Mode**: sub-agent
**Status**: draft

---

## 1. 구현 접근 (Implementation Approach)

본 SPEC은 SPEC-AX-SCORE-001(store-only) + SPEC-AX-SCORE-API-001(API-only) + SPEC-AX-REPORT-001(cross-store handler-compose) + SPEC-AX-REVIEW-001(store+API+ABAC+D1 iter2 store-TX userID 시그니처) 4개 SPEC의 패턴을 **단일 수직 슬라이스**로 결합한다. 사용자 인터뷰(2026-05-20)로 확정된 3-tier rubric+criteria+bands 데이터 모델 + 6+ 엔드포인트 + apply 엔진 범위가 작고 명확하므로 store↔API↔engine을 분리하지 않는다.

### 1.1 핵심 전략

1. **선례 정확 미러**: REVIEW-001(store+API+ABAC+D1 iter2 lesson + ≈7줄 server.go) + SCORE-API-001(errors.go drift 부착) + REPORT-001(cross-store 2-TX) 패턴 정확 재사용 — 신규 발명 0건
2. **Consumer-only [HARD]**: SCORE-001/SCORE-API-001/EVAL-ITEM-001/EVID-001/REPORT-001/REVIEW-001/AUTH-003 코드·스키마·FK·permissionMatrix 0-diff. 특히 SCORE-001 `grade_thresholds`(0004) **병행 존재만**(0-diff [HARD])
3. **단일 신규 마이그레이션 0006**: 0001/0002/0003/0004/0005 디스크 확인 후 비충돌, 멱등 패턴(0005 REVIEW-001 정확 미러)
4. **3계층 데이터 모델**: `rubrics`(부모) + `rubric_criteria`(자식, EvalItem FK-less stub) + `rubric_bands`(자식, 내부 rubric_id FK 허용)
5. **frozen RBAC 0-diff**: `RoleAdmin`/`RoleAnalyst`/`RoleViewer` 3역할만 사용, **`RoleReviewer`/`evaluator` 신설 절대 금지**(SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시, REVIEW-001 §1.5 동형 자연 회피)
6. **핸들러-로컬 ABAC narrowing**: REVIEW-001 `requireReviewAdminRole`/`guardReviewAdmin` 동형 패턴 `requireRubricAdminRole`/`guardRubricAdmin` — admin=mutations / 모든 인증 incl. viewer=read+apply
7. **REVIEW-001 D1 iter2 lesson pre-applied [HARD]**:
   - `pg_store.go BeginRubricTx`는 `audit.NewRecorder(true)`로 호출 (`false` 절대 금지) — auth-enabled principal.id 전파 보장
   - store TX mutation 메서드는 명시적 `userID string` 파라미터 — 처음부터 도입, iter1 fix 사이클 비재발
   - 핸들러는 `resolveCreatedBy(r.Context())`(또는 동형 헬퍼)로 principal.id 또는 `'cli-anonymous'` 해석 후 store에 전파
8. **SCORE-API-001 errors.go drift lesson pre-applied [HARD]**: 신규 센티넬 7개는 spec.md §2.1 [MODIFY] 표와 §2.3 Drift-Guard manifest 양쪽에 EXPLICIT 부착(분실 방지)
9. **REPORT-001 server.go ≈7줄 lesson pre-applied [HARD]**: 마운트는 "1줄"이 아닌 ≈7줄(필드+생성자+innerMux.Handle 2줄+ko 주석) 최소 단위로 명시
10. **Apply 엔진 cross-store 2-TX**: TX-1 read-only(SumWeightedByEvaluationItem from SCORE-001) → TX-2 read-only(ApplyRubric from RUBRIC-001) — REPORT-001 §6.3 / REVIEW-001 §6.3 선례 정확 미러
11. **TDD RED-first**: 모든 신규 store 메서드 + 핸들러 메서드에 대해 failing test 우선 작성

### 1.2 phantom 회피 (lesson #9)

`BeginRubricTx`/`PgRubricTx`/`RubricStore`/`ApplyRubric`은 본 SPEC에서 **신설**되는 것이므로 phantom이 아니며, 다음의 source-verified 기존 진입점만 호출한다:

- `ScoreStore.BeginScoreTx` (`store.go:253-255`, `pg_store.go:134` — apply 엔진 cross-store 점수 합산용)
- `ScoreTx.SumWeightedByEvaluationItem` (`store.go:292-295` `pgtype.Numeric` SEC-03 반환)
- `EvalItemStore.BeginEvalItemTx` → `GetEvalItemByID` (criterion 추가 시 cross-store 존재 검증)
- `audit.NewRecorder(true)` (REVIEW-001 D1 iter2 — **`false` 금지**)
- `auth.UserFromContext`, `auth.ParseRolesFromScope`, `auth.RoleAdmin` (rbac.go/middleware.go)
- `auth.ErrCodeABACDenied` (`abac.go:24`)
- `apperrors.Err*` 센티넬 (`errors.go:52-78` 기존 패턴)

신설되는 메서드/구조체는 본 SPEC의 [NEW]/[MODIFY] 선언에 모두 명시되어 있다(spec.md §2.1). SCORE-001 `Score.DetermineGrade`(`store.go:296-298`)는 **호출 0건**(병행 존재, 본 SPEC ApplyRubric은 `rubric_bands`만 사용).

---

## 2. 마일스톤 (Priority-Based, 시간 추정 금지)

### Milestone M0 (Priority High) — RED Phase: 신규 테스트 골격 + 인터페이스 정의

**산출물**:
- `internal/store/store.go` MODIFY: `RubricStore`/`RubricTx` 인터페이스 + `Rubric`/`RubricCriterion`/`RubricBand` struct 3개 정의 (REVIEW-001 `ScoreReviewRequestStore`/`Tx`/`ScoreReviewRequest` 패턴 미러)
- `internal/store/rubric.go` NEW (skeleton): 컴파일만 통과하는 빈 구현 (모든 mutation 메서드는 처음부터 `userID string` 파라미터 포함 — **REVIEW-001 D1 iter2 lesson pre-applied**)
- `cmd/server/rubric_handlers.go` NEW (skeleton): 컴파일만 통과하는 빈 핸들러
- `internal/store/rubric_test.go` NEW: store-layer failing test (validateRubricInput/validateCriterionInput/validateBandInput/validateRubricStatusTransition + ApplyRubric 알고리즘)
- `cmd/server/rubric_handlers_test.go` NEW: 22+ failing test (CRUD/생명주기/ABAC/edge case/audit 원자성/cross-store apply)

**완료 기준**: 모든 신규 테스트가 RED(실패)로 확인됨. `go vet ./...` 통과. `go test ./...` 신규 테스트만 실패.

**의존**: 없음

### Milestone M1 (Priority High) — GREEN Phase: 0006 마이그레이션 + store 구현

**산출물**:
- `.moai/db/schema/migrations/0006_rubric_tables.sql` NEW: 3개 테이블 멱등 SQL(CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION + CREATE INDEX IF NOT EXISTS) — 0005 정확 미러
- `internal/store/pg_store.go` MODIFY: `BeginRubricTx` 메서드 — **`audit.NewRecorder(true)` HARD** (REVIEW-001 D1 iter2 lesson pre-applied — `false` 절대 금지, iter1 fix 사이클 비재발)
- `internal/store/rubric.go` 완성: `PgRubricTx` 11 메서드(InsertRubric/GetRubricByID/ListRubrics/UpdateRubric/ArchiveRubric/AddCriterion/AddBand/GetCriteriaByRubric/GetBandsByRubric/ApplyRubric/InsertAuditLog) + validation 함수 4개(validateRubricInput / validateCriterionInput / validateBandInput / validateRubricStatusTransition) + Commit/Rollback. 모든 mutation 메서드는 명시적 `userID string` 파라미터 받아 SQL `$N` placeholder에 `created_by`/`updated_by` 일관 전파.
- `internal/audit/audit.go` MODIFY: Action 상수 5개 추가
- `internal/audit/recorder.go` MODIFY: `RecordRubric*` 메서드 5개 (`ApplyRubric`은 read-only, 별도 Record* 메서드 없음 — §6 OPEN #6 결정 후 확정)
- `internal/errors/errors.go` MODIFY: 센티넬 7개 추가 (**SCORE-API-001 errors.go drift 교훈 — manifest 부착 명시**)

**완료 기준**: M0 store-layer 테스트 GREEN. 마이그레이션 정방향 적용 + 재실행 멱등성 확인. `BeginRubricTx`가 `NewRecorder(true)` 호출하는지 명시적 단위 테스트 PASS.

**의존**: M0

### Milestone M2 (Priority High) — GREEN Phase: HTTP API 핸들러 + ABAC + apply 엔진

**산출물**:
- `cmd/server/rubric_handlers.go` 완성: `RubricHandler` + `NewRubricHandler(rubricStore, evalItemStore, scoreStore, logger)` + `Routes()`(ServeMux Go1.22+ 최장일치, 구체 경로 먼저) + 8+ 핸들러 메서드 + `writeRubricJSON`/`writeRubricErr`/`mapRubricStoreErr` + `requireRubricAdminRole`/`guardRubricAdmin` + `clampPagination` 재사용 또는 동형 구현 + `resolveCreatedBy` 헬퍼(REVIEW-001 D1 iter2 동형 — `auth.UserFromContext` → principal.id 또는 `'cli-anonymous'`)
- 핸들러는 cross-store apply 엔진을 위해 `ScoreStore`도 의존, criterion 추가 cross-store 검증을 위해 `EvalItemStore`도 의존(생성자 3-store 주입)
- `handleApplyRubric`은 REPORT-001 §6.3 / REVIEW-001 §6.3 cross-store handler-compose 2-TX 정확 미러

**완료 기준**: M0 핸들러 단위 테스트 GREEN(22+ test). ABAC narrowing 모든 경계 통과(viewer create 403 / analyst archive 403 / admin all mutations 200 / 모든 인증 read+apply 200 / auth-disabled 투과). Apply 엔진 cross-store 2-TX 검증 (TX-1 score 합산 → TX-2 rubric 적용 → letter 반환).

**의존**: M1

### Milestone M3 (Priority Medium) — GREEN Phase: server.go 마운트 + 통합

**산출물**:
- `cmd/server/server.go` MODIFY: `rubricH *RubricHandler` 필드 + `NewRubricHandler(pgStore, pgStore, pgStore, logger)` 생성 + `innerMux.Handle("/api/v1/rubrics", s.rubricH.Routes())` + `innerMux.Handle("/api/v1/rubrics/", s.rubricH.Routes())` 2줄 + ko 주석. **정확히 ≈7줄**(REVIEW-001 `:55-57/:210-213/:266-267/:269-270`, SCORE-API-001 `:55/:210/:266-267`, REPORT-001 `:56/:212/:269-270` 정확 미러 — **REPORT-001 lesson pre-applied: "1줄" 잘못 기술 금지**).

**완료 기준**: `go build ./...` 통과. 헬스체크 정상. `/api/v1/rubrics` 라우트 reachable. 통합 빌드 후 `0006` 마이그레이션 자동 적용 verify.

**의존**: M2

### Milestone M4 (Priority Medium) — REFACTOR Phase: 품질 보강 + TRUST 5

**산출물**:
- 한국어 에러 메시지 정합 (REVIEW-001 `score_handlers.go:114-127` 동형)
- 공통 헬퍼 추출(중복 제거)
- @MX 태그 추가(fan_in≥3 ANCHOR / 복잡도≥15 WARN / NEW 코드 NOTE)
- godoc 추가(exported 모든 함수)
- 테스트 커버리지 ≥85% 검증
- 통합 테스트 testcontainers 14+/14+ PASS (rubric_integration_test.go)

**완료 기준**: TRUST 5 PASS. 커버리지 ≥85%. evaluator-active 4-차원 ≥0.85. golangci-lint zero issues.

**의존**: M3

### Milestone M5 (Priority Medium) — Drift-Guard 검증

**산출물**:
- 모든 [EXISTING] 파일 `git diff` 0 확인:
  - `internal/store/score.go`(특히 `SumWeightedByEvaluationItem`/`DetermineGrade` 시그니처), `eval_item.go`, `evidence.go`, `score_review_request.go`(REVIEW-001)
  - `cmd/server/score_handlers.go`, `report_handlers.go`, `review_handlers.go`, `evidence_handlers.go`
  - **`internal/auth/**/*.go`**(rbac.go, abac.go, middleware.go, authz_middleware.go, chain.go — **RoleReviewer/evaluator 신설 0** SCORE-API-001 §6 OPEN #4 비재발 명시)
  - **`.moai/db/schema/migrations/0001`~`0005`**(특히 0004 `grade_thresholds` — SCORE-001 0-diff [HARD] 병행 존재)
  - `go.mod`, `go.sum`
- [MODIFY] 파일 추가 범위 검증:
  - `store.go`: 신규 인터페이스/struct 3개만, 기존 인터페이스/struct 무변경
  - `pg_store.go`: `BeginRubricTx`만(특히 `NewRecorder(true)` 확인), 기존 메서드 무변경
  - `audit.go`: 신규 상수 5개만
  - `recorder.go`: 신규 메서드 5개만
  - **`errors.go`: 신규 센티넬 7개만 (SCORE-API-001 errors.go drift 교훈 — manifest 부착 정확성 검증)**
  - `server.go`: ≈7줄만(필드+생성+마운트 2줄+주석) — REPORT-001 lesson "1줄" 잘못 기술 0 확인

**완료 기준**: drift = 0%. 위반 발생 시 즉시 중단·재계획(spec.md §2.3 R-CONSUMER-001).

**의존**: M4

---

## 3. 기술 접근 (Technical Approach)

### 3.1 store layer 패턴 (score_review_request.go 정확 미러 + apply 엔진 추가)

```
PgRubricTx struct {
    tx       pgx.Tx
    logger   *zap.Logger
    recorder *audit.Recorder  // pg_store.go: audit.NewRecorder(true) HARD — REVIEW-001 D1 iter2 lesson
}

// REVIEW-001 D1 iter2 lesson pre-applied — 모든 mutation 메서드는 userID string 파라미터
func (t *PgRubricTx) InsertRubric(ctx, name string, version int, scope string, metadata map[string]any, userID string) (UUID, error) {
    // 1. validateRubricInput (SQL 미실행 후 거부 — fail-closed)
    // 2. INSERT INTO rubrics (..., created_by, updated_by) VALUES (..., $N, $N) RETURNING id  -- userID 일관 전파
    // 3. t.recorder.RecordRubricCreated(ctx, t, id, creatorID, userID) — 동일 TX, audit_logs.user_id=userID
    // 4. return id
}

func (t *PgRubricTx) AddCriterion(ctx, rubricID UUID, evaluationItemID UUID, weight float64, userID string) (UUID, error) {
    // 1. validateCriterionInput (weight bound 0..1)
    // 2. SELECT status FROM rubrics WHERE id=$1 FOR UPDATE — concurrent + archived terminal check
    // 3. if status='archived' → return ErrRubricArchived
    // 4. INSERT INTO rubric_criteria RETURNING id  -- internal FK to rubrics(id)
    // 5. RecordRubricCriterionAdded — 동일 TX
}

func (t *PgRubricTx) AddBand(ctx, rubricID UUID, letter string, minScore, maxScore float64, userID string) (UUID, error) {
    // 1. validateBandInput (min<max, letter non-blank)
    // 2. SELECT FOR UPDATE rubrics row + archived check
    // 3. (§6 OPEN #4 Option B if approved) check band overlap: load existing bands → numrange overlap test
    // 4. INSERT INTO rubric_bands RETURNING id
    // 5. RecordRubricBandAdded — 동일 TX
}

// read-only, audit 0건 — §6 OPEN #6 Option A 채택 시
func (t *PgRubricTx) ApplyRubric(ctx, rubricID UUID, scoreValue float64) (letter string, band RubricBand, err error) {
    // 1. SELECT id, letter, min_score, max_score FROM rubric_bands WHERE rubric_id=$1 ORDER BY min_score ASC
    // 2. linear scan: find band where min_score <= scoreValue <= max_score
    // 3. if found → return (letter, band, nil)
    // 4. else → return ("", RubricBand{}, ErrRubricInvalidInput) — fail-closed, no default fallback
    // 5. NO recorder call — read-only operation
}
```

state-machine 가드 함수 (3-state, REVIEW-001 4-state 동형 단순화):

```
var allowedRubricTransitions = map[string][]string{
    "draft":    {"active"},
    "active":   {"archived"},
    "archived": {},  // terminal
}

func validateRubricStatusTransition(current, target string) error {
    allowed, ok := allowedRubricTransitions[current]
    if !ok { return ErrRubricInvalidStatus }
    for _, a := range allowed {
        if a == target { return nil }
    }
    return ErrRubricInvalidStatus
}
```

### 3.2 HTTP handler 패턴 (review_handlers.go 정확 미러)

```
type RubricHandler struct {
    rubricStore   store.RubricStore
    evalItemStore store.EvalItemStore  // cross-store criterion 존재 검증용
    scoreStore    store.ScoreStore     // cross-store apply 엔진용
    logger        *zap.Logger
}

func NewRubricHandler(rs store.RubricStore, es store.EvalItemStore, ss store.ScoreStore, logger *zap.Logger) *RubricHandler {
    return &RubricHandler{rubricStore: rs, evalItemStore: es, scoreStore: ss, logger: logger}
}

func (h *RubricHandler) Routes() http.Handler {
    mux := http.NewServeMux()
    // 구체 경로 먼저 (ServeMux Go1.22+ 최장일치)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/apply", h.handleApplyRubric)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/criteria", h.handleAddCriterion)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/bands", h.handleAddBand)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/archive", h.handleArchiveRubric)
    // §6 OPEN #1 결정 후: PUT /rubrics/{id} 또는 POST /rubrics/{id}/clone-new-version
    mux.HandleFunc("PUT /api/v1/rubrics/{id}", h.handleUpdateRubric)
    mux.HandleFunc("GET /api/v1/rubrics/{id}", h.handleGetRubric)
    mux.HandleFunc("GET /api/v1/rubrics", h.handleListRubrics)
    mux.HandleFunc("POST /api/v1/rubrics", h.handleCreateRubric)
    return mux
}
```

ABAC 게이트(review_handlers.go 동형):

```
func requireRubricAdminRole(scope string) bool {
    for _, r := range auth.ParseRolesFromScope(scope) {
        if r == auth.RoleAdmin { return true }
    }
    return false
}

func (h *RubricHandler) guardRubricAdmin(w, r) bool {
    u, ok := auth.UserFromContext(r.Context())
    if !ok { return true }  // auth-disabled 투과
    if requireRubricAdminRole(strings.Join(u.Scopes, " ")) { return true }
    h.writeRubricErr(w, 403, auth.ErrCodeABACDenied, "rubric 관리 권한이 없는 사용자입니다", "")
    return false
}

// REVIEW-001 D1 iter2 lesson pre-applied — resolveCreatedBy 헬퍼
func resolveCreatedBy(ctx context.Context) string {
    if u, ok := auth.UserFromContext(ctx); ok && u.ID != "" {
        return u.ID
    }
    return "cli-anonymous"
}
```

Apply 엔진 cross-store handler-compose 2-TX (REPORT-001 §6.3 / REVIEW-001 §6.3 정확 미러):

```
func (h *RubricHandler) handleApplyRubric(w, r) {
    rubricID := parseUUID(r.PathValue("id"))
    scoreID := parseUUID(r.URL.Query().Get("score_id"))  // 또는 body
    
    // TX-1: read-only, SCORE-001 cross-store 합산
    scoreTx, _ := h.scoreStore.BeginScoreTx(ctx)
    defer scoreTx.Rollback(ctx)
    numericSum, err := scoreTx.SumWeightedByEvaluationItem(ctx, scoreID)  // pgtype.Numeric SEC-03
    scoreTx.Rollback(ctx)
    scoreValue, _ := numericSum.Float64Value()  // pgtype 변환
    
    // TX-2: read-only, RUBRIC-001 등급 적용
    rubricTx, _ := h.rubricStore.BeginRubricTx(ctx)
    defer rubricTx.Rollback(ctx)
    letter, band, err := rubricTx.ApplyRubric(ctx, rubricID, scoreValue)
    rubricTx.Rollback(ctx)
    
    h.writeRubricJSON(w, 200, map[string]any{
        "rubric_id": rubricID, "score_value": scoreValue,
        "letter": letter, "band": band,
    })
}
```

### 3.3 0006 마이그레이션 SQL (0005 패턴 정확 미러)

```sql
-- 0006_rubric_tables.sql

CREATE TABLE IF NOT EXISTS rubrics (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(64) NOT NULL,
    version         INT NOT NULL DEFAULT 1,
    scope           VARCHAR(64),
    status          VARCHAR(32) NOT NULL DEFAULT 'draft',
    archive_reason  TEXT,
    metadata        JSONB,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by      VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_by      VARCHAR(64)
);

CREATE TABLE IF NOT EXISTS rubric_criteria (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rubric_id           UUID NOT NULL REFERENCES rubrics(id),  -- 내부 FK 허용 (동일 마이그레이션)
    evaluation_item_id  UUID NOT NULL,                          -- FK-less stub to EvalItem
    weight              NUMERIC(5,4) NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rubric_bands (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rubric_id   UUID NOT NULL REFERENCES rubrics(id),  -- 내부 FK 허용
    letter      VARCHAR(8) NOT NULL,
    min_score   NUMERIC(8,4) NOT NULL,
    max_score   NUMERIC(8,4) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

DO $$ BEGIN
    ALTER TABLE rubrics
        ADD CONSTRAINT rubrics_status_chk
        CHECK (status IN ('draft', 'active', 'archived'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE rubric_criteria
        ADD CONSTRAINT rubric_criteria_weight_chk
        CHECK (weight >= 0 AND weight <= 1);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE rubric_bands
        ADD CONSTRAINT rubric_bands_range_chk
        CHECK (min_score < max_score);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- §6 OPEN #7 Option A 채택 시:
-- DO $$ BEGIN
--     ALTER TABLE rubrics
--         ADD CONSTRAINT rubrics_archive_reason_chk
--         CHECK ((status != 'archived') OR (archive_reason IS NOT NULL AND length(archive_reason) > 0));
-- EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- §6 OPEN #2 Option A 채택 시 (active 1-per-(name, scope)):
-- CREATE UNIQUE INDEX IF NOT EXISTS rubrics_active_unique_idx
--     ON rubrics (name, scope) WHERE status = 'active';

-- §6 OPEN #4 Option A 채택 시 (band overlap EXCLUSION):
-- CREATE EXTENSION IF NOT EXISTS btree_gist;
-- ALTER TABLE rubric_bands ADD CONSTRAINT rubric_bands_no_overlap
--     EXCLUDE USING gist (rubric_id WITH =, numrange(min_score, max_score) WITH &&);

CREATE INDEX IF NOT EXISTS rubrics_status_idx ON rubrics (status);
CREATE INDEX IF NOT EXISTS rubrics_scope_idx ON rubrics (scope);
CREATE INDEX IF NOT EXISTS rubrics_created_at_idx ON rubrics (created_at DESC);
CREATE INDEX IF NOT EXISTS rubric_criteria_rubric_id_idx ON rubric_criteria (rubric_id);
CREATE INDEX IF NOT EXISTS rubric_bands_rubric_id_idx ON rubric_bands (rubric_id);
```

### 3.4 server.go 마운트 (≈7줄, REVIEW-001/SCORE-API-001/REPORT-001 정확 미러)

```go
// server.go:55-57 영역 (필드 추가, scoreH/reportH/reviewH 선례 라인)
rubricH *RubricHandler

// server.go:210-213 영역 (생성자, NewScoreHandler/NewReportHandler/NewReviewHandler 선례)
// 단계 (i-5): 등급 rubric 핸들러 (SPEC-AX-RUBRIC-001) — pgStore가 RubricStore+EvalItemStore+ScoreStore 동시 구현
s.rubricH = NewRubricHandler(pgStore, pgStore, pgStore, logger)

// server.go:266-270 영역 (innerMux 마운트 2줄)
innerMux.Handle("/api/v1/rubrics", s.rubricH.Routes())
innerMux.Handle("/api/v1/rubrics/", s.rubricH.Routes())
```

**총 ≈7줄**: 필드 1줄 + 주석 1줄 + 생성자 1줄 + 마운트 2줄 + 주석 2줄(상단/구분). **"1줄"이 아닌 ≈7줄 최소 단위** — REPORT-001 spec.md:32 manifest accuracy 교훈 정확 적용 (REVIEW-001에서 검증됨).

---

## 4. TDD RED-GREEN-REFACTOR 사이클

### 4.1 RED: 22+ failing test 작성

**store-layer** (rubric_test.go):

1. TestInsertRubric_ValidInput_ReturnsUUIDAndInsertsAuditRow
2. TestInsertRubric_BlankName_ReturnsInvalidInput
3. TestInsertRubric_NameOver64Chars_ReturnsInvalidInput
4. TestInsertRubric_AuditFailure_TwoWayRollback
5. TestInsertRubric_UserIDPropagatesToCreatedByAndAuditUserID (**REVIEW-001 D1 iter2 lesson 검증 — `userID string` 파라미터가 SQL `$N`로 `created_by`/`updated_by`/`audit_logs.user_id` 일관 전파**)
6. TestAddCriterion_ValidInput_ReturnsUUIDAndInsertsAudit
7. TestAddCriterion_WeightOutOfBounds_ReturnsWeightOutOfBounds
8. TestAddCriterion_ArchivedRubric_ReturnsArchived
9. TestAddBand_ValidInput_ReturnsUUIDAndInsertsAudit
10. TestAddBand_MinGteMax_ReturnsInvalidInput
11. TestUpdateRubric_DraftToActive_TransitionsSuccess
12. TestUpdateRubric_ArchivedRubric_ReturnsArchived
13. TestArchiveRubric_ActiveToArchived_TransitionsSuccess
14. TestArchiveRubric_Terminal_PersistsAfter (archived → 다른 전이 시도 ErrRubricInvalidStatus)
15. TestStatusTransition_AllAllowedAndDisallowed (state-machine 가드 완전성)
16. TestApplyRubric_ScoreInBand_ReturnsLetter
17. TestApplyRubric_ScoreOutOfAllBands_ReturnsInvalidInput
18. TestApplyRubric_ReadOnly_NoAuditWritten (read-only no-audit § 6 OPEN #6 Option A 검증)
19. TestNewRecorderTrue_AuthEnabledUserIDPropagates (**REVIEW-001 D1 iter2 lesson 검증 — `NewRecorder(true)` 호출 + auth-enabled principal.id가 `audit_logs.user_id`로 정확 전파**)

**HTTP handler** (rubric_handlers_test.go):

20. TestPOST_Rubrics_AdminCreates_201
21. TestPOST_Rubrics_ViewerForbidden_403
22. TestPOST_Rubrics_AnalystForbidden_403 (admin-only mutations)
23. TestGET_Rubrics_ViewerCanRead_200
24. TestPOST_RubricsArchive_AnalystForbidden_403
25. TestPOST_RubricsApply_ViewerCanApply_200 (apply는 모든 인증 read-like)
26. TestPOST_RubricsArchive_AdminSucceeds_200_TerminalImmutable
27. TestPOST_RubricsAddCriterion_EvalItemNotExist_404 (cross-store TX-1 검증)
28. TestPOST_RubricsApply_CrossStoreTwoTX (SumWeightedByEvaluationItem TX-1 → ApplyRubric TX-2 시나리오)
29. TestPOST_RubricsApply_ScoreOutOfAllBands_400 (fail-closed)
30. TestGET_Rubrics_AuthDisabled_PassthroughCliAnonymous (UBI-003)
31. TestGET_Rubrics_MalformedUUID_400
32. TestPOST_RubricsArchivedRubricMutation_409 (archived terminal)
33. TestPOST_Rubrics_AuditWriteFailed_RollsBack

### 4.2 GREEN: 최소 구현으로 통과

각 RED 테스트를 통과시키는 최소 코드. score_review_request.go / review_handlers.go 패턴 정확 미러로 발명 비용 최소화. **`NewRecorder(true)` + `userID string` 파라미터는 처음부터 정확히 구현**(REVIEW-001 iter1 fix 사이클 비재발).

### 4.3 REFACTOR: TRUST 5 보강

- 공통 헬퍼 추출 (writeRubricJSON/Err, mapRubricStoreErr — SCORE-API-001/REVIEW-001 패턴)
- godoc 추가 (exported 모든 함수)
- @MX 태그 (ANCHOR/WARN/NOTE)
- 한국어 주석/에러 메시지 정합
- 통합 테스트 testcontainers 14+/14+ PASS

---

## 5. 리스크 (Risks)

### 5.1 R-RUBRIC-001 (High): SCORE-001 `grade_thresholds` 병행 존재 일관성

**Risk**: SCORE-001 단일 테이블 `grade_thresholds` 모델과 RUBRIC-001 `rubric_bands` 모델이 병행 존재. 호출자가 두 모델을 혼동하면 등급 산정 결과 불일치.

**Mitigation**:
- 본 SPEC §1.4 [HARD] consumer-only 명시 (SCORE-001 `Score.DetermineGrade` 호출 0건)
- spec.md §2.2 [EXISTING] manifest에 `DetermineGrade` 호출 0건 명시
- ApplyRubric은 rubric_bands만 사용, SCORE-001 grade_thresholds는 무관
- 통합 테스트: 두 모델 모두 적용된 시나리오에서 letter 결과 차이 검증(의도적 차이 = 다른 모델)

### 5.2 R-RUBRIC-002 (Medium): cross-store apply 엔진 race window

**Risk**: TX-1 `SumWeightedByEvaluationItem` → TX-2 `ApplyRubric` 사이에 점수 또는 rubric_bands 변경 가능성.

**Mitigation**:
- SCORE-001/RUBRIC-001 모두 append-only (UBI-004 / REVIEW-001 §6.3 동형 — 물리 DELETE 0)
- archived rubric mutation 차단으로 TX-2 시점 rubric_bands 안정성 확보
- PoC 수용 trade-off (REPORT-001 §6.3 / REVIEW-001 §6.3 동형 결정)

### 5.3 R-RUBRIC-003 (Medium): frozen RBAC 위반 시도

**Risk**: 개발자가 admin-only 정책이 너무 strict하다고 판단하여 `RoleReviewer` 신설 또는 `permissionMatrix`에 `write:rubric` 추가 시도.

**Mitigation**:
- 본 SPEC §1.5 [HARD] frozen rbac.go 0-diff 명시
- SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시
- M5 Drift-Guard에서 `internal/auth/**/*.go` 0-diff 검증
- 본 SPEC 매핑(admin/모든 인증)이 frozen rbac.go에 모두 존재 → 충돌 자연 회피

### 5.4 R-RUBRIC-004 (Low): REVIEW-001 D1 iter2 lesson 재현 (`NewRecorder(false)` 또는 store-TX userID 파라미터 누락)

**Risk**: spec.md §1.4 [HARD] 강조에도 불구하고 구현 단계에서 `NewRecorder(false)` 사용 또는 store mutation 메서드에 `userID` 파라미터 누락 가능성.

**Mitigation**:
- 본 SPEC §2.1 [MODIFY] `pg_store.go BeginRubricTx`에 `NewRecorder(true)` HARD 명시
- 본 SPEC §3 EARS REQ-RUBRIC-001-E1에 `userID` 파라미터 의무 명시
- 본 plan.md §4.1 RED test #5/#19에서 명시적 검증 테스트 작성 (UserID 전파 + NewRecorder(true) 호출)
- M2 store-layer 테스트에서 auth-enabled 시나리오 PASS 필수
- iter1 fix 사이클 비재발 = 처음부터 정확히 구현

### 5.5 R-RUBRIC-005 (Low): SCORE-API-001 errors.go drift 재현

**Risk**: 신규 센티넬 7개가 §2.1 [MODIFY] 표에만 있고 §2.3 Drift-Guard manifest에서 누락되어 drift 검증 실패 시 분실 가능성.

**Mitigation**:
- 본 SPEC §2.1과 §2.3 양쪽에 errors.go [MODIFY] 7 센티넬 추가 EXPLICIT 명시 (이중 부착)
- M5 Drift-Guard에서 errors.go 추가 범위 검증 + manifest 정확성 동시 확인

---

## 6. 다음 단계 (Run Phase Strategy)

1. **§6 OPEN 7건 결정**: strategy.md (Run phase) — 5요소(결정/근거/거부대안/consumer-only/Run phase 적용) 부착 → Human Gate sign-off
2. **/clear → /moai run SPEC-AX-RUBRIC-001**: TDD RED-GREEN-REFACTOR 사이클 진입
3. **harness=thorough**: evaluator-active 4-차원 ≥0.85 + manager-quality TRUST 5 PASS
4. **drift-guard 0%**: M5 검증으로 consumer-only [HARD] 무위반 확인
5. **lesson 부착**: 본 SPEC 완료 후 D1 iter2 패턴이 처음부터 적용된 사례로 lesson DB 업데이트

---

## 7. 참조

- spec.md §2.1 [NEW]/[MODIFY]/[EXISTING] Delta 매트릭스
- spec.md §3 EARS 요구사항 (UBI 4 + modal 21)
- spec.md §6 OPEN 7건 (권장 옵션 부착)
- acceptance.md AC 25 (Given-When-Then, UBI 4 + modal REQ 21)
- research.md (file:line 근거, SSOT)
- SPEC-AX-SCORE-001 plan.md (store+audit 패턴 + grade_thresholds 병행 존재 선례)
- SPEC-AX-SCORE-API-001 plan.md (errors.go drift lesson + OPEN #4 evaluator-INFEASIBLE lesson)
- SPEC-AX-REPORT-001 plan.md (cross-store 2-TX + ≈7줄 server.go 선례)
- SPEC-AX-REVIEW-001 plan.md (store+API+ABAC + D1 iter2 lesson 가장 최근 정확 미러 대상, commit 89a8828)
- `.claude/skills/moai/workflows/plan.md` L378 (canonical 8-field frontmatter schema)
