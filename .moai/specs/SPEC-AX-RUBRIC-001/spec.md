---
id: SPEC-AX-RUBRIC-001
version: 0.1.0
status: draft
created: 2026-05-20
updated: 2026-05-20
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-20): 등급 rubric 확장 시스템 — rubric+criteria+bands 3계층 store + HTTP API 수직 슬라이스(Rubric Expansion System — 3-tier Rubric+Criteria+Bands Store + HTTP API Layer) 첫 초안. SPEC-AX-SCORE-001(완료, v0.1.3)의 `grade_thresholds` 단일 테이블(SCORE-001 `0004_score_tables.sql` `grade_thresholds` + `Score.DetermineGrade` `store.go:296-298`)을 **수정하지 않은 채**, `rubrics`(부모) + `rubric_criteria`(가중치별 평가 항목) + `rubric_bands`(등급 구간 letter+min/max)의 3계층 데이터 모델 + store TX + 6+ HTTP 엔드포인트 + 등급 산정 엔진을 신규로 추가한다. SCORE-001 `grade_thresholds`(범 score 등급 임계)와 RUBRIC-001 `rubric_bands`(rubric별 등급 구간)는 병행 존재하며 본 SPEC은 SCORE-001 consumer-only [HARD] 0-diff를 유지한다. 신규 store 도메인(`RubricStore`/`RubricTx`) + 신규 마이그레이션 `0006_rubric_tables.sql` + 동일-TX `RecordRubric*` 감사(SCORE-001/REVIEW-001 D2/D4 패턴 미러) + 6+ HTTP 엔드포인트(rubric 생성/조회/목록/수정/archive + criteria/bands sub-resource) + `ApplyRubric(ctx, rubricID, scoreValue)` store-layer 등급 산정 메서드(read-only, audit 0건) + 핸들러 cross-store handler-compose 2-TX(`SumWeightedByEvaluationItem` → `ApplyRubric`, REPORT-001 §6.3 / REVIEW-001 §6.3 선례 정확 미러). 핸들러-로컬 ABAC narrowing(admin=mutations / 모든 인증 incl. viewer=read+apply) — frozen `rbac.go`(`admin`/`analyst`/`viewer` 3역할, `rbac.go:33` 정규식 `^iroum-ax:(admin|analyst|viewer)$`) **0-diff**(SCORE-API-001 `requireScoreWriteRole`/`guardScoreWrite` `score_handlers.go:161-187` + REVIEW-001 `requireReviewAdminRole`/`guardReviewAdmin` 동형 패턴 재사용). FK-제약-없는 stub 정책 일관 적용(SCORE-001 §1.4 동형). 한국 공공 6제약(데이터 주권/한국어/감사 가능성/망분리/조직 격리/시간 제약) 준수. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-SCORE-001 / SCORE-API-001 / REPORT-001 / REVIEW-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등 canonical 외 필드는 사용하지 않는다. 본 SPEC의 모든 EARS 요구사항·영향파일·HTTP 계약·DB 스키마는 본 디렉토리의 `research.md`(file:line 근거)에 근거하며, 소비 계약 시그니처는 `apps/control-plane/internal/store/store.go`(`ScoreStore`/`ScoreTx` `store.go:253-305`, `EvalItemStore` `store.go:115-119` 패턴 미러 대상), `apps/control-plane/internal/store/score.go`(`SumWeightedByEvaluationItem` `store.go:292-295` `pgtype.Numeric` SEC-03 반환 — research.md cite), `apps/control-plane/internal/store/pg_store.go`(`BeginScoreTx` `pg_store.go:134` Recorder 주입 선례), `apps/control-plane/internal/store/score_review_request.go`(REVIEW-001 D1 lesson pre-applied — `NewRecorder(true)` + `userID string` 파라미터 store-TX 시그니처), `apps/control-plane/internal/audit/recorder.go`(`RecordScoreReviewRequest*` 동형 패턴 미러 대상), `apps/control-plane/cmd/server/review_handlers.go`(REVIEW-001 핸들러+ABAC + `requireReviewAdminRole`/`guardReviewAdmin` 동형 패턴 미러 대상), `apps/control-plane/internal/auth/rbac.go`(frozen 3-role `:19-26/:33/:68-80`), `apps/control-plane/internal/errors/errors.go`(센티넬 패턴 `:52-78`), `apps/control-plane/cmd/server/server.go`(라우트 마운트 ≈7줄 — REVIEW-001 `reviewH` 필드 `:55-57` 영역, 생성 `:210-213` 영역, 마운트 `:266-267/:269-270` 영역 동형), `.moai/db/schema/migrations/0005_score_review_request_tables.sql`(REVIEW-001 멱등 패턴 정확 미러)에서 직접 검증된다(phantom 0 — manager-spec orchestrator ground-truth, 2026-05-20).

---

# SPEC-AX-RUBRIC-001 — 등급 rubric 확장 시스템 (Rubric Expansion System — 3-tier Rubric+Criteria+Bands Store + HTTP API Layer)

## 1. 개요

경영평가팀이 SPEC-AX-SCORE-001이 완성한 `scores` 데이터 + `grade_thresholds` 단일 테이블 임계 모델 위에, **rubric별로 가중치 평가 항목과 등급 구간(letter + min/max)을 명시적으로 운영**할 수 있도록, `apps/control-plane/`(Go 1.23+)에 **신규 store 도메인 + HTTP API 계층 + 등급 산정 엔진**을 한 SPEC으로 추가한다. 본 SPEC은 SPEC-AX-SCORE-001 + SCORE-API-001 + REPORT-001 + REVIEW-001이 형성한 store/audit/handler-compose/ABAC 패턴(`BeginXxxTx` + `RecordXxx*` 동일-TX + cross-store handler-compose 2-TX + 핸들러-로컬 ABAC narrowing)을 정확히 미러링하며, SCORE-001의 단일 테이블 `grade_thresholds` 모델은 일절 변경하지 않는다(consumer-only of SCORE-001 §1.4 동형).

### 1.1 본 SPEC 범위 — 수직 슬라이스 (store + audit + HTTP API + apply 엔진)

본 SPEC은 SPEC-AX-REVIEW-001과 동형의 **단일 수직 슬라이스**이다. 사용자 인터뷰(2026-05-20)로 확정된 범위가 작고 명확하므로 store↔API↔engine을 분리하지 않는다.

- 신규 파일 6개: `internal/store/rubric.go`(`PgRubricTx` 구현 + `ApplyRubric` 엔진 메서드), `internal/store/rubric_test.go`(store-layer 단위 테스트), `internal/store/rubric_integration_test.go`(testcontainers 통합), `cmd/server/rubric_handlers.go`(`RubricHandler`+`Routes()`+ABAC+6+ 핸들러), `cmd/server/rubric_handlers_test.go`(httptest 단위 테스트), `.moai/db/schema/migrations/0006_rubric_tables.sql`(멱등 신규 마이그레이션)
- 기존 파일 7개 수정: `internal/store/store.go`(`RubricStore`/`RubricTx` 인터페이스 + `Rubric`/`RubricCriterion`/`RubricBand` struct 추가), `internal/store/pg_store.go`(`BeginRubricTx` + `NewRecorder(true)` Recorder 주입 — **REVIEW-001 D1 iter2 lesson pre-applied**), `internal/audit/audit.go`(액션 상수 5개), `internal/audit/recorder.go`(`RecordRubric*` 메서드 5개), `internal/errors/errors.go`(센티넬 7개 — **SCORE-API-001 errors.go drift 교훈에 따라 §2.3 Drift-Guard manifest에 EXPLICIT 부착**), `cmd/server/server.go`(라우트 마운트 **≈7줄** — REVIEW-001/REPORT-001 정확 미러)
- 3계층 데이터 모델: `rubrics`(부모: name/version/scope/status/created_by/...) + `rubric_criteria`(자식: rubric_id/evaluation_item_id/weight) + `rubric_bands`(자식: rubric_id/letter/min_score/max_score)
- 6+ HTTP 엔드포인트(정확한 엔드포인트 shape는 §6 OPEN #1 후속 결정 — versioning shape이 단건 조회 JSON embed 형태와 active 편집 정책에 함께 영향): rubric CRUD + criteria sub-resource + bands sub-resource + apply 엔진 read-only
- 동일-TX audit: mutation 5종(rubric 생성/수정/archive + criterion 추가 + band 추가)이 각각 동일 `pgx.Tx`에 `audit_logs` 1건을 기록. `ApplyRubric`은 read-only이며 audit 0건(§6 OPEN #6 — read-only no-audit 선례 vs 한국 공공 추가 요구).
- ABAC narrowing(핸들러-로컬, frozen `rbac.go` 0-diff): admin=mutations(rubric create/update/archive + criteria add + bands add), 모든 인증 incl. viewer=read + apply(query-like, audit 0).
- cli-anonymous 기본값 + auth-disabled Walking Skeleton fallback (SCORE-001 §1.1, AUTH-003 정합, REVIEW-001 UBI-003 동형).
- 표준 에러: `score_handlers.go`/`review_handlers.go` `{"error":{"code","message","field"}}` 동일 스키마.

### 1.2 Anchor 컨텍스트

본 SPEC은 SPEC-AX-SCORE-001이 형성한 점수 흐름(raw → item → category → grade)과 SCORE-001 단일 `grade_thresholds` 등급 매핑의 **상위 운영 계층**(평가편람별 rubric, 가중치 평가 항목, 등급 구간 명시화)을 HTTP로 외부에 노출하여 `product.md` 경영평가 편람의 "rubric 운영" 절차를 클라이언트가 구성·적용할 수 있게 한다(research.md §1.1). PoC 범위는 "안전보건" 범주의 rubric 운영이며, SPEC-AX-SCORE-001 / SCORE-API-001 / REPORT-001 / REVIEW-001 / AUTH-003 / CTRL-001은 GREEN(완료) 상태로 가정한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `RUBRIC` (Rubric expansion sub-domain — rubric 부모 + criteria + bands 3계층 데이터 모델 + 등급 산정 엔진)
- 따라서 SPEC ID: `SPEC-AX-RUBRIC-001` (`.claude/skills/moai/workflows/plan.md` Composite domain rules "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내 — `AX` + `RUBRIC`).

### 1.4 의존성 stub 계약 — consumer-only of SCORE-001 [핵심, load-bearing]

본 SPEC은 SPEC-AX-SCORE-001 / SCORE-API-001 / REPORT-001 / REVIEW-001 / AUTH-003의 **순수 consumer**이다. 다음이 **HARD 계약**이다(메모리 lesson #9 phantom 회피, REVIEW-001 §1.4 동형):

- **[HARD]** 본 SPEC은 SCORE-001 `grade_thresholds` 테이블·`Score.DetermineGrade`(`store.go:296-298`)·`SumWeightedByEvaluationItem`(`store.go:292-295` `pgtype.Numeric` SEC-03 반환) **시그니처 무수정**. SCORE-001 등급 임계 단일 테이블 모델은 그대로 두고, RUBRIC-001 `rubric_bands`는 **병행 존재**한다. 호출자(handler)가 어느 모델을 사용할지 결정한다(§6 OPEN 외 사안 아님 — 단순 공존).
- **[HARD]** `internal/store/score.go`(SCORE-001 `PgScoreTx`), `internal/store/score_review_request.go`(REVIEW-001 `PgScoreReviewRequestTx`), `internal/store/eval_item.go`(EVAL-ITEM-001), `internal/store/evidence.go`(EVID-001), `cmd/server/score_handlers.go` / `report_handlers.go` / `review_handlers.go` / `evidence_handlers.go`, `internal/auth/rbac.go`/`abac.go`(frozen RBAC/ABAC), `.moai/db/schema/migrations/0001`–`0005`를 **일절 수정하지 않는다**. SCORE-001/EVAL-ITEM-001/EVID-001/AUTH-003 비즈니스 로직·audit·핸들러·마이그레이션·RBAC 매트릭스는 호출만 한다.
- **[HARD]** FK-제약-없는 stub 정책 일관(SCORE-001 §1.4 동형):
  - `rubric_criteria.evaluation_item_id` = `UUID NOT NULL` — EVAL-ITEM-001 `evaluation_items.id`(UUID PK) 참조의 FK-less stub. 존재 검증은 핸들러 단계 cross-store(별도 TX `EvalItemStore`).
  - `rubric_criteria.rubric_id`, `rubric_bands.rubric_id` = 본 SPEC 내부 `rubrics.id` 참조 — **내부 FK는 허용**(같은 마이그레이션 0006 내, REVIEW-001 §1.4 외부 FK-less와 다른 동일-마이그레이션 내부 FK 패턴).
- **[HARD]** 신규 마이그레이션 1건: `0006_rubric_tables.sql`. REVIEW-001 `0005` 멱등 패턴(0001~0005 일관, 0006 비충돌 디스크 확인) 정확 미러 — `CREATE TABLE IF NOT EXISTS` + `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object` + `CREATE INDEX IF NOT EXISTS`. 0001/0002/0003/0004/0005 / 기타 기존 마이그레이션은 무수정.
- **[HARD]** 동일-TX audit (mutation만): mutation 5종(rubric_created/updated/archived + criterion_added + band_added)이 각각 동일 `pgx.Tx`에 `audit_logs` 1건을 기록한다. REVIEW-001 `BeginScoreReviewRequestTx`(D1 iter2 fix — `NewRecorder(true)` + `userID string` 파라미터) 패턴 **정확 미러**. `ApplyRubric`은 read-only이며 audit 0건(§6 OPEN #6).
- **[HARD] REVIEW-001 D1 lesson pre-applied**: `pg_store.go BeginRubricTx`는 `audit.NewRecorder(true)`로 호출한다(`false` 금지). 이유: auth-enabled 모드에서 `auth.UserFromContext(ctx)`가 반환하는 principal.id를 audit 행 `user_id`에 올바로 전파하기 위해서이다(REVIEW-001 v0.1.1 iter2 root-cause fix). `NewRecorder(false)`는 `user_id`를 항상 `'cli-anonymous'`로 고정하여 UBI-003 Must-Pass 위반.
- **[HARD] REVIEW-001 D1 lesson pre-applied**: store TX mutation 메서드는 명시적 `userID string` 파라미터를 가져야 한다(REVIEW-001 v0.1.1 iter2 store-TX 시그니처 fix). 핸들러는 `resolveCreatedBy(r.Context())`(또는 동형 헬퍼)로 principal.id 또는 `'cli-anonymous'`를 해석하여 store SQL `$N` placeholder에 `created_by`/`updated_by`/`audit_logs.user_id`로 일관 전파한다. 이 시그니처는 처음부터 도입한다(iter1 fix 사이클 비재발).
- **[HARD]** `postgres.go`는 Sprint-0 死 스텁(SPEC-AX-SCORE-001 plan.md §2)이며 본 SPEC 대상 아님. TX 진입점은 `store.RubricStore.BeginRubricTx`(→ `pg_store.go PgWorkflowStore.pool` 단일 풀 재사용, `pg_store.go:134` `BeginScoreTx` 패턴 미러)만 사용한다. 신규 `pgxpool.Pool` 생성 금지.
- **[HARD]** 신규 외부 의존 0건 — 본 SPEC은 어떤 신규 라이브러리/SDK도 `go.mod`에 추가하지 않는다. 기존 `pgx/v5`, `google/uuid`, `zap`, `pgtype`(SCORE-001 SEC-03 `SumWeightedByEvaluationItem` 반환 타입)만 사용한다.

### 1.5 ABAC 권한 경계 [핵심] — 핸들러-로컬 narrowing, frozen RBAC 0-diff

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 SPEC-AX-REVIEW-001 `requireReviewAdminRole`/`guardReviewAdmin`(`review_handlers.go` source-verified) **동형 패턴으로 재사용**한다. 핸들러-로컬 매핑이며 frozen `rbac.go`(`RoleAdmin`/`RoleAnalyst`/`RoleViewer` 3역할, scope 정규식 `^iroum-ax:(admin|analyst|viewer)$` `rbac.go:33` source-verified)는 0-diff이다.

- **mutation (rubric create/update/archive + criteria/bands 추가)**: `RoleAdmin`만 허용 — 핸들러-로컬 `requireRubricAdminRole`(REVIEW-001 `requireReviewAdminRole` 동형). analyst/viewer/anonymous는 403.
- **read (단건/목록 GET) + apply (등급 산정 query-like)**: 모든 인증 사용자 허용(`RoleViewer` 포함, read-only/query-like). write-role 게이트 미차용. `ApplyRubric`은 read-only이므로 ABAC narrowing 기준에서 read 권한으로 분류한다(audit 0건과 일관).
- **auth-disabled 투과**: `authEnabled=false`(Walking Skeleton 기본값) → ABAC/RBAC 미들웨어 자동 투과(`abac.go:8` REQ-ABAC-009 정합), store 계층이 `cli-anonymous` 기록(SCORE-001 §1.1 동형, REVIEW-001 UBI-003 동형). 본 SPEC은 인증 비활성에서도 동작한다.
- **admin 우회**: `RoleAdmin`은 모든 ABAC narrowing을 우회한다(`abac.go:71` REQ-ABAC-004 정합).
- **narrowing-only**: ABAC는 권한 부여 없이 미인가만 거부한다(`abac.go:11` 정합).

> **[중요 — SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 자연 회피]** SCORE-API-001 §6 OPEN #4는 "write 권한 보유자 매핑" + `evaluator` 역할 부재 충돌이 strategy phase 사안이었다. 본 SPEC은 **역할 매핑이 명확** — admin=mutations(rbac.go에 존재), 모든 인증 incl. viewer=read+apply(rbac.go에 존재). `evaluator`/`reviewer` 같은 신규 역할이 필요하지 않으므로 frozen `rbac.go` 0-diff가 자연스럽게 성립한다(REVIEW-001 §1.5 동형 회피). 따라서 본 SPEC §6 OPEN은 SCORE-API-001 OPEN #4를 상속하지 않는다.

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` `apps/control-plane/` 트리를 따른다. 본 SPEC은 SCORE-001 / SCORE-API-001 / EVAL-ITEM-001 / EVID-001 / REPORT-001 / REVIEW-001 / AUTH-003 코드는 일절 수정하지 않고(consumer-only §1.4 HARD), rubric 운영 도메인 신규 파일과 통합 지점(store interface 추가/마이그레이션/audit constants/server 라우트)만 추가/수정한다. Delta 마커: [EXISTING]=consumer로 호출만(무변경), [NEW]=신규 추가, [MODIFY]=정확한 추가 범위만.

### 2.1 Go Control Plane — store + audit + HTTP API + apply 엔진

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/internal/store/rubric.go` | `PgRubricTx` pgx 구현 — `InsertRubric`/`GetRubricByID`/`ListRubrics`/`UpdateRubric`/`ArchiveRubric`/`AddCriterion`/`AddBand`/`GetCriteriaByRubric`/`GetBandsByRubric` + `ApplyRubric(ctx, rubricID, scoreValue float64) (letter string, band RubricBand, error)` (read-only, audit 0) + `validateRubricInput`/`validateCriterionInput`/`validateBandInput`/`validateRubricStatusTransition`(state-machine: draft→active→archived terminal). 모든 mutation 메서드는 명시적 `userID string` 파라미터(**REVIEW-001 D1 iter2 lesson pre-applied**). 동일 TX에 `InsertAuditLog` 호출. REVIEW-001 `score_review_request.go` 정확 미러. | [NEW] | REQ-RUBRIC-001/002/004 |
| `apps/control-plane/internal/store/rubric_test.go` | store-layer 단위 테스트 — `validateRubricInput`/`validateCriterionInput`/`validateBandInput`/`validateRubricStatusTransition` 가드 함수 + `ApplyRubric` 등급 산정 알고리즘 단위 테스트 + fake `recorder` 격리. | [NEW] | 전체 |
| `apps/control-plane/internal/store/rubric_integration_test.go` | testcontainers 통합 — 0006 마이그레이션 적용 + `BeginRubricTx` end-to-end + 동일-TX audit 원자성 + `SELECT ... FOR UPDATE` 동시성(§6 OPEN #2 active 1-per-(name,scope) 결정 후) + cross-store EvalItem 존재 검증 race. | [NEW] | 전체 |
| `apps/control-plane/cmd/server/rubric_handlers.go` | `RubricHandler` struct + `NewRubricHandler(rubricStore, evalItemStore, scoreStore, logger)`(cross-store 의존: rubric 자체 + criterion EvalItem 존재 검증 + apply 시 점수 합산) + `Routes() http.Handler`(ServeMux Go1.22+ 최장일치, 구체 경로 먼저) + 6+ 핸들러 메서드(handleCreateRubric/handleGetRubric/handleListRubrics/handleUpdateRubric/handleArchiveRubric/handleAddCriterion/handleAddBand/handleApplyRubric) + 표준 JSON/에러 헬퍼(`writeRubricJSON`/`writeRubricErr`/`mapRubricStoreErr` — REVIEW-001 동형) + 핸들러-로컬 ABAC 게이트(`requireRubricAdminRole`/`guardRubricAdmin` — `review_handlers.go` 동형). `handleApplyRubric`은 cross-store handler-compose 2-TX(TX-1 `scoreStore.BeginScoreTx`→`SumWeightedByEvaluationItem`→Rollback, TX-2 `rubricStore.BeginRubricTx`→`ApplyRubric`→Rollback, REPORT-001 §6.3 / REVIEW-001 §6.3 선례 정확 미러). | [NEW] | REQ-RUBRIC-002/003/005 |
| `apps/control-plane/cmd/server/rubric_handlers_test.go` | `httptest` 기반 핸들러 단위 테스트 — 6+ 엔드포인트 정상/에러 경로, 3-state 생명주기 전이(`draft→active→archived`), 불법 전이 거부(409), ABAC narrowing(viewer create 403 / analyst archive 403 / admin all mutations 200 / 모든 인증 read+apply 200), rubric 미존재(404), blank/>64 name(400), weight CHECK 위반(400), band overlap(400 또는 CHECK 500 — §6 OPEN #4 결정 후), archived mutation(409), auth-disabled 투과, apply score out of all bands fail-closed, concurrent version increment race. store는 fake 격리. | [NEW] | 전체 |
| `.moai/db/schema/migrations/0006_rubric_tables.sql` | 3개 테이블 신규 — `rubrics`(id UUID PK / name VARCHAR(64) NOT NULL / version INT NOT NULL DEFAULT 1 / scope VARCHAR(64) / status VARCHAR(32) NOT NULL DEFAULT 'draft' / archive_reason TEXT NULL / created_at/updated_at TIMESTAMPTZ / created_by/updated_by VARCHAR(64) DEFAULT 'cli-anonymous' / metadata JSONB), `rubric_criteria`(id UUID PK / rubric_id UUID NOT NULL REFERENCES rubrics(id) **내부 FK 허용** / evaluation_item_id UUID NOT NULL **FK-less stub to EvalItem** / weight NUMERIC(5,4) NOT NULL CHECK weight≥0 AND weight≤1 / created_at TIMESTAMPTZ), `rubric_bands`(id UUID PK / rubric_id UUID NOT NULL REFERENCES rubrics(id) **내부 FK 허용** / letter VARCHAR(8) NOT NULL / min_score NUMERIC(8,4) NOT NULL / max_score NUMERIC(8,4) NOT NULL / CHECK min<max / created_at TIMESTAMPTZ). 멱등 패턴(CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION duplicate_object CHECK 상태 enum/weight bound/min<max + CREATE INDEX IF NOT EXISTS). 0001/0002/0003/0004/0005 디스크 확인 후 0006 비충돌. | [NEW] | REQ-RUBRIC-001 |
| `apps/control-plane/internal/store/store.go` | `RubricStore` 인터페이스 추가(`BeginRubricTx` 진입점만, `ScoreReviewRequestStore` `store.go:115-119` 패턴 미러) + `RubricTx` 인터페이스 추가(Insert/Get/List/Update/Archive/AddCriterion/AddBand/GetCriteriaByRubric/GetBandsByRubric/ApplyRubric/InsertAuditLog/Commit/Rollback — REVIEW-001 `ScoreReviewRequestTx` 패턴 미러) + `Rubric`/`RubricCriterion`/`RubricBand` 도메인 struct 3개(fieldalignment 정렬: map → time.Time × 2 → 포인터 → 문자열 → UUID). | [MODIFY] | REQ-RUBRIC-001 |
| `apps/control-plane/internal/store/pg_store.go` | `BeginRubricTx(ctx) (RubricTx, error)` 메서드 추가 — `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})`(`pg_store.go:84/103/118/134` 동형) + `PgRubricTx{tx, logger, recorder: audit.NewRecorder(true)}` 반환(**REVIEW-001 D1 iter2 lesson pre-applied** — `NewRecorder(true)`로 auth-enabled 시 principal.id 전파, `false` 금지). 신규 `pgxpool.Pool` 생성 금지(단일 풀 재사용, postgres.go 死 스텁 비대상). | [MODIFY] | REQ-RUBRIC-001/004 |
| `apps/control-plane/internal/audit/audit.go` | Action 상수 5개 추가: `ActionRubricCreated = "RUBRIC_CREATED"` / `ActionRubricUpdated = "RUBRIC_UPDATED"` / `ActionRubricArchived = "RUBRIC_ARCHIVED"` / `ActionRubricCriterionAdded = "RUBRIC_CRITERION_ADDED"` / `ActionRubricBandAdded = "RUBRIC_BAND_ADDED"`. SCORE-001/REVIEW-001 `ActionScoreReviewRequest*` 동형 추가. namespace 상수 0(D2 — UUID PK 직접 대입). | [MODIFY] | REQ-RUBRIC-UBI-002 |
| `apps/control-plane/internal/audit/recorder.go` | 메서드 5개 추가: `RecordRubricCreated(ctx, tx, rubricID, creatorID, userID)` / `RecordRubricUpdated(ctx, tx, rubricID, updaterID, userID)` / `RecordRubricArchived(ctx, tx, rubricID, archiverID, archiveReason, userID)` / `RecordRubricCriterionAdded(ctx, tx, criterionID, rubricID, evaluationItemID, userID)` / `RecordRubricBandAdded(ctx, tx, bandID, rubricID, letter, userID)`. SCORE-001/REVIEW-001 `RecordScoreReviewRequest*` 동형 — local AuditTx, resource_id = `rubrics.id`/`rubric_criteria.id`/`rubric_bands.id` UUID 직접 대입(D2 미러). `ApplyRubric`은 read-only이므로 별도 `RecordRubricApplied` 메서드 없음(§6 OPEN #6). | [MODIFY] | REQ-RUBRIC-UBI-002 |
| `apps/control-plane/internal/errors/errors.go` | 센티넬 7개 추가(SCORE-001/REVIEW-001 센티넬 `errors.go:52-78` 동형 패턴): `ErrRubricNotFound`(404), `ErrRubricInvalidInput`(400, blank/>64 name 포함), `ErrRubricInvalidStatus`(409, 불법 전이 archived terminal 포함), `ErrRubricArchived`(409, archived rubric mutation 시도 거부), `ErrRubricWeightOutOfBounds`(400, weight CHECK violation 사전 검증), `ErrRubricBandOverlap`(400, band min/max 겹침 사전 검증 — §6 OPEN #4 Option B 채택 시), `ErrRubricAuditWriteFailed`(REVIEW-001 `ErrScoreReviewRequestAuditWriteFailed` 동형 — 양방향 rollback 트리거). 모든 SCORE-001/REVIEW-001 센티넬은 무수정. **SCORE-API-001 errors.go drift 교훈에 따라 §2.3 Drift-Guard manifest에 EXPLICIT 부착**(분실 방지). | [MODIFY] | REQ-RUBRIC-005-U1 |
| `apps/control-plane/cmd/server/server.go` | **라우트 마운트 ≈7줄**(REVIEW-001 / SCORE-API-001 / REPORT-001 정확 미러): `rubricH *RubricHandler` 필드 추가(`server.go:55-57` `scoreH`/`reportH`/`reviewH` 선례 라인) + `s.rubricH = NewRubricHandler(pgStore, pgStore, pgStore, logger)` 생성자(`server.go:210-213` `NewScoreHandler`/`NewReportHandler`/`NewReviewHandler` 선례 — pgStore가 RubricStore+EvalItemStore+ScoreStore 동시 구현) + `innerMux.Handle("/api/v1/rubrics", s.rubricH.Routes())` + `innerMux.Handle("/api/v1/rubrics/", s.rubricH.Routes())` 마운트 2줄(`server.go:266-267/269-270` 선례 정확 미러 — Go1.22 ServeMux path-param 서브트리 라우팅 구조적 필수) + ko 주석. ABAC 와이어링은 기존 미들웨어 체인이 innerMux 전체 자동 적용 — **ABAC 와이어링 변경 0-diff**. **REPORT-001 lesson pre-applied: "1줄"이 아닌 ≈7줄 최소 단위**(필드+생성+마운트 2줄+ko 주석). | [MODIFY] | REQ-RUBRIC-002/003 |

### 2.2 소비 계약 — 호출만, 무변경 (consumer-only [HARD] §1.4)

| 경로 | 소비 계약 | Delta |
|------|----------|-------|
| `apps/control-plane/internal/store/store.go` (SCORE-001 정의 부분) | `ScoreStore.BeginScoreTx`(`store.go:253-255`), `ScoreTx.SumWeightedByEvaluationItem`(`store.go:292-295` `pgtype.Numeric` SEC-03 반환) — apply 핸들러의 cross-store handler-compose 1-st TX에서 호출만 | [EXISTING] |
| `apps/control-plane/internal/store/store.go` (SCORE-001 `DetermineGrade`) | `Score.DetermineGrade`(`store.go:296-298`) — **호출 0건**(본 SPEC `ApplyRubric`은 SCORE-001 `grade_thresholds` 무관, RUBRIC-001 `rubric_bands`만 사용). SCORE-001 모델 병행 존재 — 0-diff [HARD] | [EXISTING] |
| `apps/control-plane/internal/store/store.go` (EVAL-ITEM-001 정의 부분) | `EvalItemStore`/`EvalItemTx`/`GetEvalItemByID` — criterion 추가 시 evaluation_item_id 존재 검증 cross-store(별도 TX) 호출만 | [EXISTING] |
| `apps/control-plane/internal/store/score.go` | `PgScoreTx.SumWeightedByEvaluationItem` 구현 — 호출만, 점수 비즈니스 로직 무변경 | [EXISTING] |
| `apps/control-plane/internal/store/eval_item.go` | `PgEvalItemTx.GetEvalItemByID` 구현 — 호출만, EvalItem 비즈니스 로직 무변경 | [EXISTING] |
| `apps/control-plane/internal/store/score_review_request.go` | REVIEW-001 `PgScoreReviewRequestTx` — 호출 0건, 패턴 미러 참조만 | [EXISTING] |
| `apps/control-plane/internal/store/pg_store.go` (기존 BeginXxxTx 메서드들) | `BeginScoreTx`(`pg_store.go:134`), `BeginEvalItemTx`, `BeginEvidenceTx`, `BeginScoreReviewRequestTx`(REVIEW-001 v0.1.1 — Recorder 주입 `NewRecorder(true)` 패턴 미러 대상) — 호출만 | [EXISTING] |
| `apps/control-plane/internal/audit/audit.go` (SCORE-001/EVID-001/EVAL-ITEM-001/REVIEW-001 부분) | 기존 Action 상수 — 무변경, 패턴 참조만 | [EXISTING] |
| `apps/control-plane/internal/audit/recorder.go` (SCORE-001/REVIEW-001 부분) | 기존 `RecordScore*`/`RecordScoreReviewRequest*` 메서드 — 무변경, 동형 패턴 참조만 | [EXISTING] |
| `apps/control-plane/cmd/server/score_handlers.go`/`report_handlers.go`/`review_handlers.go`/`evidence_handlers.go` | SCORE-API-001/REPORT-001/REVIEW-001/EVID-001 핸들러 + ABAC 게이트(`requireScoreWriteRole`/`requireReviewAdminRole` 등) — 패턴 미러만(코드 무변경) | [EXISTING] |
| `apps/control-plane/internal/auth/abac.go`, `authz_middleware.go`, `chain.go` | `ErrCodeABACDenied`(abac.go:24), `ABACEvaluator`/narrowing-only/admin 우회/auth-disabled 투과, `RESTAuthzMiddleware` — 호출만/미들웨어 체인 자동 적용. **0-diff [HARD]** | [EXISTING] |
| `apps/control-plane/internal/auth/rbac.go`, `middleware.go` | `RoleAdmin`/`RoleAnalyst`/`RoleViewer`(`rbac.go:19-26`), scope 정규식(`rbac.go:33`), `ParseRolesFromScope`(`rbac.go:68-80`), `UserFromContext`, `User` struct — 호출만. **permissionMatrix/Authorize/정규식 frozen — 무변경 [HARD]**. 신규 역할 추가 0(특히 `RoleReviewer`/`evaluator` 신설 금지). **SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시** — 본 SPEC 매핑(admin / 모든 인증)은 rbac.go에 모두 존재하여 충돌 자연 회피 | [EXISTING] |
| `.moai/db/schema/migrations/0001`/`0002`/`0003`/`0004`/`0005` | SCORE-001/EVID-001/EVAL-ITEM-001/CTRL-001/REVIEW-001 기존 마이그레이션 — 본 SPEC은 마이그레이션 추가/수정 0건(0006만 신규). **SCORE-001 `grade_thresholds`(0004)는 무수정 — RUBRIC-001 `rubric_bands`와 병행 존재** | [EXISTING] |
| `apps/control-plane/go.mod`/`go.sum` | 기존 직접 의존 무변경 — 신규 외부 의존 추가 0건(`pgx/v5`/`google/uuid`/`zap`/`pgtype` 등 기존만 사용) | [EXISTING] |

### 2.3 Drift-Guard Manifest

[NEW]만 신규 생성, [MODIFY]는 정확히 명시된 추가 범위만, [EXISTING]은 0 diff. 구현 중 [EXISTING] 파일에 1줄이라도 수정 발생 시 consumer-only [HARD] 위반 → 즉시 중단·재계획.

- **신규 생성 허용**: `internal/store/rubric.go`, `internal/store/rubric_test.go`, `internal/store/rubric_integration_test.go`, `cmd/server/rubric_handlers.go`, `cmd/server/rubric_handlers_test.go`, `.moai/db/schema/migrations/0006_rubric_tables.sql`
- **수정 허용(범위 한정)**:
  - `internal/store/store.go`: `RubricStore`/`RubricTx` 인터페이스 + `Rubric`/`RubricCriterion`/`RubricBand` struct 3개 추가만(기존 인터페이스/struct 무변경)
  - `internal/store/pg_store.go`: `BeginRubricTx` 메서드 추가만(기존 메서드 무변경) — **`NewRecorder(true)` HARD**(REVIEW-001 D1 iter2 lesson)
  - `internal/audit/audit.go`: 액션 상수 5개 추가만(기존 상수 무변경)
  - `internal/audit/recorder.go`: `RecordRubric*` 메서드 5개 추가만(기존 메서드 무변경)
  - **`internal/errors/errors.go`: 센티넬 7개 추가만**(기존 센티넬 무변경) — **SCORE-API-001 errors.go drift 교훈 적용**: 본 항목은 §2.1 [MODIFY] 표와 §2.3 Drift-Guard 양쪽에 EXPLICIT 부착되어 manifest 분실 방지
  - `cmd/server/server.go`: `rubricH` 필드 1줄 + 생성자 1줄 + `innerMux.Handle` 2줄 + ko 주석 ≈7줄(REVIEW-001/SCORE-API-001/REPORT-001 정확 미러 — `server.go:55-57/210-213/266-267/269-270` 선례; **REPORT-001 lesson — "1줄"이 아닌 ≈7줄 최소 단위 명시 정확성**)
- **수정 절대 금지(0 diff 검증)**: `internal/store/score.go`(SCORE-001 PgScoreTx — 특히 `SumWeightedByEvaluationItem` 시그니처 + `DetermineGrade`), `internal/store/eval_item.go`, `internal/store/evidence.go`, `internal/store/score_review_request.go`(REVIEW-001), `cmd/server/score_handlers.go`(SCORE-API-001), `cmd/server/report_handlers.go`(REPORT-001), `cmd/server/review_handlers.go`(REVIEW-001), `cmd/server/evidence_handlers.go`, **`internal/auth/**/*.go`(frozen RBAC/ABAC — 특히 `rbac.go:19-26/:33/:68-80` `RoleAdmin`/`RoleAnalyst`/`RoleViewer` 3-role + 정규식 frozen, RoleReviewer/evaluator 신설 절대 금지)**, `.moai/db/schema/migrations/0001`–`0005`(**특히 `0004_score_tables.sql` `grade_thresholds` — RUBRIC-001 `rubric_bands`와 병행 존재 0-diff**), `apps/control-plane/go.mod`/`go.sum`

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건) — REQ-RUBRIC-UBI-NNN dual-track

Ubiquitous 요구사항은 SPEC-AX-SCORE-001 / SCORE-API-001 / REPORT-001 / REVIEW-001의 canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track 패턴을 도메인 스코프 형태(`REQ-RUBRIC-UBI-NNN`)로 적용한다.

- **REQ-RUBRIC-UBI-001 (데이터 주권)**: The rubric expansion layer (store + HTTP API + apply engine) SHALL NOT make any external service call (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) on any request path (rubric 생성/조회/목록/수정/archive/criteria 추가/bands 추가/apply). 모든 영속·검증·등급 산정은 단일 내부 PostgreSQL pgx pool(`PgWorkflowStore.pool` 재사용, `pg_store.go:134` `BeginScoreTx` 동형)에만 위임한다(`tech.md` 망분리 정합). 신규 외부 의존을 도입하지 않는다.
- **REQ-RUBRIC-UBI-002 (감사 가능성 — mutations 동일-TX entity+audit 원자성, apply read-only 예외)**: For every **mutation operation** (`InsertRubric` 생성 / `UpdateRubric` 수정 / `ArchiveRubric` archive / `AddCriterion` criteria 추가 / `AddBand` bands 추가), the store layer SHALL write exactly one `audit_logs` row (`ActionRubric{Created|Updated|Archived|CriterionAdded|BandAdded}`) within the same `pgx.Tx` as the entity change, AND the `audit_logs.resource_id` SHALL equal the corresponding entity UUID directly (rubrics.id / rubric_criteria.id / rubric_bands.id, SCORE-001/REVIEW-001 D2 미러 — surrogate 미사용, namespace 상수 0). audit 실패 시 호출자가 Rollback하면 entity 변경과 audit-INSERT 양쪽이 취소되어야 한다(양방향 원자성). For the **read-only `ApplyRubric` operation**, the store SHALL NOT write any `audit_logs` row (apply는 query-like, audit 0건 — §6 OPEN #6 결정 후 확정). HTTP API 핸들러는 별도 audit row를 INSERT하지 않는다(이중 감사 금지).
- **REQ-RUBRIC-UBI-003 (cli-anonymous 기본값 + auth-disabled fallback)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the API layer SHALL serve all endpoints with ABAC/RBAC middleware transparently passing through (`abac.go:8` REQ-ABAC-009 정합), AND the resulting `rubrics`/`rubric_criteria`/`rubric_bands` rows SHALL carry `created_by`/`updated_by` = `'cli-anonymous'` literal AND the corresponding `audit_logs.user_id` SHALL also equal `'cli-anonymous'` literal (NULL 금지, SCORE-001 §1.1 정합). When 인증이 활성(`AUTH_ENABLED=true`)이면 store TX mutation 메서드는 `userID string` 파라미터(**REVIEW-001 D1 iter2 lesson pre-applied** — `resolveCreatedBy(r.Context())` 결과)로 받은 principal.id를 `created_by`/`updated_by`/`audit_logs.user_id` 모두에 일관 전파한다. `pg_store.go BeginRubricTx`는 `audit.NewRecorder(true)`(`false` 금지)로 호출되어 auth-enabled principal.id 전파를 보장한다.
- **REQ-RUBRIC-UBI-004 (권한 narrowing + 3-state 불변식 + archived terminal)**: The system SHALL enforce three unbreakable invariants simultaneously: (1) **ABAC narrowing-only**: mutation 권한 미보유 principal의 mutation 시도는 HTTP 403 `ErrCodeABACDenied`(`abac.go:24`)로 거부하며 핸들러-로컬 게이트(`requireRubricAdminRole`/`guardRubricAdmin`)가 frozen `rbac.go` 무수정으로 매핑한다(admin=mutations / 모든 인증 incl. viewer=read+apply), (2) **3-state allowed transitions only**: store-level `validateRubricStatusTransition` 가드가 `draft → active` / `active → archived`만 허용하고 그 외 모든 전이(예: `archived → active`, `active → draft`, `archived → draft` 등)는 SQL 미실행 후 `ErrRubricInvalidStatus`(HTTP 409)로 거부한다, (3) **archived terminal — archived rubric mutation 거부**: status `archived`인 rubric에 대한 mutation(update/criteria 추가/bands 추가) 시도는 SQL 미실행 후 `ErrRubricArchived`(HTTP 409)로 거부한다. `archived`는 terminal 상태이며 되돌리기 불가(SCORE-001 D4 / REVIEW-001 UBI-004 state-machine 동형). 정정 경로는 신규 rubric 생성(`draft` 신규 row INSERT)만 — 물리 DELETE 0건.

### 3.2 REQ-RUBRIC-001 — Store + 3계층 데이터 모델 + 마이그레이션

본 모듈은 `rubrics`/`rubric_criteria`/`rubric_bands` 3개 테이블 신설과 store TX 진입점을 정의한다.

- **REQ-RUBRIC-001-E1 (Event-Driven, rubric entity+audit 원자 생성)**: WHEN a caller invokes `RubricTx.InsertRubric(ctx, name, version, scope, metadata, userID)` with valid input within an open `pgx.Tx`, THEN the store SHALL INSERT one row into `rubrics` with `status = 'draft'` AND SHALL write exactly one `audit_logs` row with `action = 'RUBRIC_CREATED'` AND `resource_id = rubrics.id` UUID directly (D2 미러) AND `user_id = userID` within the same transaction, returning the generated UUID. The store SHALL bind `userID` to SQL `$N` placeholder for `created_by`/`updated_by`/`audit_logs.user_id` columns consistently (**REVIEW-001 D1 iter2 lesson pre-applied — `userID string` 파라미터 명시 전파**).
- **REQ-RUBRIC-001-E2 (Event-Driven, criterion 추가 — cross-store EvalItem 검증 외부, store는 entity+audit만)**: WHEN a caller invokes `RubricTx.AddCriterion(ctx, rubricID, evaluationItemID, weight, userID)` with valid input within an open `pgx.Tx`, THEN the store SHALL INSERT one row into `rubric_criteria` with the provided `evaluation_item_id` (FK-less stub to `evaluation_items.id`) AND `weight` AND SHALL write one `audit_logs` row with `action = 'RUBRIC_CRITERION_ADDED'` AND `resource_id = rubric_criteria.id` UUID directly within the same TX, returning the generated criterion UUID. Existence validation of `evaluationItemID` SHALL be performed by the HTTP handler via `EvalItemStore.BeginEvalItemTx` → `GetEvalItemByID` in a separate transaction (cross-store 2-TX, REVIEW-001 §6.3 / REPORT-001 §6.3 선례 동형).
- **REQ-RUBRIC-001-E3 (Event-Driven, band 추가 — entity+audit 원자)**: WHEN a caller invokes `RubricTx.AddBand(ctx, rubricID, letter, minScore, maxScore, userID)` with valid input within an open `pgx.Tx`, THEN the store SHALL INSERT one row into `rubric_bands` with the provided `letter`/`min_score`/`max_score` AND SHALL write one `audit_logs` row with `action = 'RUBRIC_BAND_ADDED'` AND `resource_id = rubric_bands.id` UUID directly within the same TX, returning the generated band UUID. Band overlap detection SHALL follow §6 OPEN #4 결정 후(handler-layer validation vs PostgreSQL EXCLUSION USING gist).
- **REQ-RUBRIC-001-S1 (State-Driven, FK-less stub to EvalItem)**: IF `evaluation_item_id` is provided in any `AddCriterion` payload, THEN the store SHALL accept it as `UUID NOT NULL` without enforcing a foreign-key constraint to `evaluation_items.id` (FK-less stub — SCORE-001 §1.4 / REVIEW-001 §1.4 동형 계약), AND the existence of the referenced eval item SHALL be validated by the HTTP handler via `EvalItemStore.BeginEvalItemTx` → `GetEvalItemByID` in a separate transaction (cross-store 2-TX 조합).
- **REQ-RUBRIC-001-U1 (Unwanted, blank/invalid 입력)**: IF `name` is blank/empty OR `name` length > 64 chars OR `weight < 0` OR `weight > 1` OR `min_score >= max_score` OR `letter` is blank/empty OR `metadata` is not a JSONB-serializable map, THEN the store SHALL reject the input before SQL execution by returning the appropriate sentinel (`ErrRubricInvalidInput` for blank/length, `ErrRubricWeightOutOfBounds` for weight, `ErrRubricInvalidInput` for min/max/letter) AND SHALL NOT INSERT any row into `rubrics`/`rubric_criteria`/`rubric_bands` or `audit_logs` (fail-closed).
- **REQ-RUBRIC-001-O1 (Optional, metadata opaque placeholder)**: WHERE the client provides a `metadata` JSONB object in any rubric request body, the store SHALL persist it verbatim into `rubrics.metadata` AND SHALL NOT interpret, validate, or transform its contents (opaque placeholder — SCORE-001 `Score.Metadata` / REVIEW-001 동형). 본 SPEC은 metadata 스키마를 정의하지 않는다.

### 3.3 REQ-RUBRIC-002 — HTTP API rubric CRUD + criteria/bands sub-resource (read+create endpoints)

본 모듈은 rubric CRUD 엔드포인트 군을 정의한다. 정확한 엔드포인트 shape는 §6 OPEN #1·#2·#3 후속 결정.

- **REQ-RUBRIC-002-E1 (Event-Driven, rubric 생성 엔드포인트)**: WHEN the HTTP API receives `POST /api/v1/rubrics` with a JSON body containing `{name, version?, scope?, metadata?}` AND the principal passes the ABAC `requireRubricAdminRole` gate (admin OR auth-disabled), THEN the handler SHALL (a) validate input via `validateRubricInput`, (b) invoke `RubricTx.InsertRubric` within a new TX, (c) commit the TX, AND (d) respond with HTTP 201 Created + `{id, name, version, scope, status, created_at, created_by}` JSON body. 핸들러는 자체 audit row를 INSERT하지 않는다(store-only audit, UBI-002 정합).
- **REQ-RUBRIC-002-E2 (Event-Driven, rubric 단건 조회)**: WHEN the HTTP API receives `GET /api/v1/rubrics/{id}` with a valid UUID path param AND the principal passes basic authentication narrowing (any authenticated user including viewer OR auth-disabled), THEN the handler SHALL respond with HTTP 200 OK + the full rubric entity JSON (including embedded `criteria[]` and `bands[]` arrays — §6 OPEN #1 versioning shape 후속 결정에 따라 embed payload schema 동시 확정) if found, OR HTTP 404 NOT_FOUND with `{"error":{"code":"NOT_FOUND","message":"요청한 rubric을 찾을 수 없습니다"}}` if absent.
- **REQ-RUBRIC-002-E3 (Event-Driven, rubric 목록)**: WHEN the HTTP API receives `GET /api/v1/rubrics?status=&scope=&limit=&offset=` with optional filter and pagination query params, THEN the handler SHALL respond with HTTP 200 OK + `{items: [...], total: N}` JSON body, applying `clampPagination`(REVIEW-001 `score_handlers.go:144-157` 동형 — `limit` 기본 50, 최대 500, `offset` 음수 → 0) AND ordering rows by `created_at DESC`. 결과 0건은 빈 배열(error 아님).
- **REQ-RUBRIC-002-E4 (Event-Driven, criterion 추가 sub-resource)**: WHEN the HTTP API receives `POST /api/v1/rubrics/{id}/criteria` with body `{evaluation_item_id, weight}` AND the principal passes the `requireRubricAdminRole` gate, THEN the handler SHALL (a) verify `rubrics.id` exists and is NOT archived (returning 404/409 respectively), (b) validate the referenced `evaluation_item_id` exists via `EvalItemStore.BeginEvalItemTx` → `GetEvalItemByID` (cross-store TX-1, returning 404 if absent), (c) invoke `RubricTx.AddCriterion` (TX-2, write), (d) commit, AND (e) respond with HTTP 201 + criterion JSON.
- **REQ-RUBRIC-002-E5 (Event-Driven, band 추가 sub-resource)**: WHEN the HTTP API receives `POST /api/v1/rubrics/{id}/bands` with body `{letter, min_score, max_score}` AND the principal passes the `requireRubricAdminRole` gate, THEN the handler SHALL (a) verify `rubrics.id` exists and is NOT archived, (b) validate `min_score < max_score`, (c) check overlap with existing bands (§6 OPEN #4 결정 후), (d) invoke `RubricTx.AddBand`, (e) commit, AND (f) respond with HTTP 201 + band JSON.
- **REQ-RUBRIC-002-U1 (Unwanted, malformed UUID)**: IF the path param `{id}` is not a valid UUID v4 string, THEN the handler SHALL respond with HTTP 400 BAD_REQUEST + `{"error":{"code":"INVALID_ARGUMENT","message":"유효하지 않은 rubric ID 형식입니다","field":"id"}}` without invoking the store layer.

### 3.4 REQ-RUBRIC-003 — HTTP API 상태 전이 + archived terminal (update/archive endpoints)

본 모듈은 rubric 상태 전이 엔드포인트를 정의한다. 정확한 엔드포인트 shape(`PUT /rubrics/{id}` 자동 버전 증분 vs `POST /rubrics/{id}/clone-new-version` sub-resource)는 §6 OPEN #1 후속 결정.

- **REQ-RUBRIC-003-E1 (Event-Driven, rubric 수정 — draft→active 또는 active 직접 수정)**: WHEN the HTTP API receives the update endpoint (shape §6 OPEN #1) for rubric `{id}` with body containing `{name?, scope?, status?, metadata?}` AND the principal passes the `requireRubricAdminRole` gate, THEN the store SHALL (a) acquire a row lock via `SELECT ... FOR UPDATE` for concurrent transition safety (REVIEW-001 §6.6 동형), (b) verify current status (NOT `archived`, returning `ErrRubricArchived` HTTP 409 otherwise), (c) verify state transition (if `status` change requested, must follow `draft→active`), (d) update fields AND `updated_at = now()` AND `updated_by = userID`, (e) write one `audit_logs` row with `action = 'RUBRIC_UPDATED'` within the same TX (with `user_id = userID`), AND (f) respond with HTTP 200 OK + updated entity. **§6 OPEN #5 active 직접 수정 정책 결정**(immutable + clone-new-version 강제 vs admin 직접 편집 허용).
- **REQ-RUBRIC-003-E2 (Event-Driven, archive — active → archived terminal)**: WHEN the HTTP API receives the archive endpoint for rubric `{id}` with body `{archive_reason}` (§6 OPEN #7 — 필수 vs optional 결정 후) AND the principal passes the `requireRubricAdminRole` gate, THEN the store SHALL (a) acquire row lock, (b) verify current status is `active` (or `draft` — 정책 결정 후), (c) verify `archive_reason` non-empty if §6 OPEN #7 Option A 채택, (d) update `status = 'archived'` AND `archive_reason` AND `updated_at/updated_by`, (e) write `audit_logs` row with `action = 'RUBRIC_ARCHIVED'` within the same TX, AND (f) respond with HTTP 200 OK + updated entity. `archived`는 terminal(되돌리기 금지, UBI-004).
- **REQ-RUBRIC-003-S1 (State-Driven, archived terminal — mutation 거부)**: IF any mutation request (update/criteria add/bands add) targets a rubric with `status = 'archived'`, THEN the store-level guard SHALL reject before SQL execution by returning `ErrRubricArchived` (HTTP 409) AND SHALL NOT INSERT or UPDATE any row in `rubrics`/`rubric_criteria`/`rubric_bands` or `audit_logs`. archived rubric은 read 및 apply만 허용된다(append-only, UBI-004 정합).
- **REQ-RUBRIC-003-S2 (State-Driven, 불법 전이 거부)**: IF any state transition request targets a non-allowed transition (e.g., `archived → active`, `active → draft`, `archived → draft` 등) OR targets a row already in terminal state (`archived`), THEN the store-level `validateRubricStatusTransition` SHALL reject before SQL execution by returning `ErrRubricInvalidStatus` (HTTP 409) AND SHALL NOT INSERT or UPDATE any row in any table.

### 3.5 REQ-RUBRIC-004 — Apply 엔진 (read-only 등급 산정)

본 모듈은 `ApplyRubric` 엔진의 store-layer 메서드 + HTTP 엔드포인트를 정의한다.

- **REQ-RUBRIC-004-E1 (Event-Driven, store-layer ApplyRubric — read-only no-audit)**: WHEN a caller invokes `RubricTx.ApplyRubric(ctx, rubricID, scoreValue float64)` within an open `pgx.Tx`, THEN the store SHALL (a) load `rubric_bands` for the given `rubricID` ordered by `min_score ASC`, (b) find the band where `min_score <= scoreValue <= max_score`, (c) return `(letter string, band RubricBand, nil)` if found, OR `(\"\", RubricBand{}, ErrRubricInvalidInput)` if `scoreValue` falls outside all bands (fail-closed — §6 OPEN #6 결정 후 확정), AND the store SHALL NOT write any `audit_logs` row (read-only, audit 0건 — UBI-002 second clause 정합).
- **REQ-RUBRIC-004-E2 (Event-Driven, HTTP apply 엔드포인트 — cross-store handler-compose 2-TX)**: WHEN the HTTP API receives the apply endpoint (e.g., `POST /api/v1/rubrics/{id}/apply` with body `{score_id?, score_value?}`) AND the principal passes basic authentication narrowing (read+apply scope, any authenticated user OR auth-disabled), THEN the handler SHALL execute **cross-store handler-compose 2-TX** (REPORT-001 §6 RESOLVED #1 / REVIEW-001 §6.3 선례 정확 미러): (TX-1 read-only) `scoreStore.BeginScoreTx` → `SumWeightedByEvaluationItem(scoreID)` returning `pgtype.Numeric` SEC-03 → convert to float64 → Rollback, (TX-2 read-only) `rubricStore.BeginRubricTx` → `ApplyRubric(rubricID, scoreValue)` → Rollback, AND the handler SHALL respond with HTTP 200 OK + `{rubric_id, score_value, letter, band: {min_score, max_score}}` JSON body. Race window는 SCORE-001/RUBRIC-001 물리 삭제 0(append-only)이라 결정적 不發生 — PoC 수용 trade-off (REVIEW-001 §6.3 동형).
- **REQ-RUBRIC-004-U1 (Unwanted, score out of all bands)**: IF the provided `scoreValue` falls outside all `rubric_bands` (e.g., scoreValue=200 but max(max_score)=100), THEN the store SHALL return `ErrRubricInvalidInput` (mapped to HTTP 400) with message `{"error":{"code":"INVALID_ARGUMENT","message":"점수가 rubric 등급 구간을 벗어났습니다","field":"score_value"}}` — fail-closed, no default fallback (한국 공공 결정성 정합).

### 3.6 REQ-RUBRIC-005 — 에러 매핑 + 표준 응답

- **REQ-RUBRIC-005-E1 (Event-Driven, 결정적 store→HTTP 매핑)**: WHEN the handler invokes any store method AND the store returns a sentinel error, THEN the handler's `mapRubricStoreErr`(SCORE-API-001 `mapStoreErr` `score_handlers.go:111-129` / REVIEW-001 `mapReviewStoreErr` 동형) SHALL map deterministically: `ErrRubricNotFound` → 404 `NOT_FOUND` / `ErrRubricInvalidInput` → 400 `INVALID_ARGUMENT` / `ErrRubricInvalidStatus` → 409 `CONFLICT` / `ErrRubricArchived` → 409 `CONFLICT` / `ErrRubricWeightOutOfBounds` → 400 `INVALID_ARGUMENT` / `ErrRubricBandOverlap` → 400 `INVALID_ARGUMENT` / `ErrRubricAuditWriteFailed` → 500 `INTERNAL` (with ERROR log) / unknown → 500 `INTERNAL` (fail-safe). 모든 에러 본문은 한국어 메시지(`score_handlers.go:114-127` / `review_handlers.go` 동형). 클라이언트 오류(400/403/404/409)는 INFO 로그, 서버 결함(500)은 ERROR 로그.
- **REQ-RUBRIC-005-U1 (Unwanted, raw pgx 에러 누출 금지)**: IF the store layer encounters `pgx.ErrNoRows` or other raw pgx-level errors, THEN it SHALL wrap and return the appropriate sentinel (`ErrRubricNotFound` for ErrNoRows) — never returning raw `pgx.ErrNoRows` (GAP-03 패턴 정합, EVID-001/SCORE-001/REVIEW-001 동형).

---

## 4. 상수·계약 인용 검증

| 항목 | 정의 | 검증 |
|------|------|------|
| Allowed rubric 상태 enum | `draft` / `active` / `archived` | research.md / 본 SPEC §3 |
| Allowed transitions | `draft→active`, `active→archived`만 (archived terminal) | 본 SPEC REQ-RUBRIC-UBI-004 / REQ-RUBRIC-003-S2 |
| Terminal state | `archived` (되돌리기 금지, 모든 mutation 차단) | 본 SPEC REQ-RUBRIC-UBI-004 / REQ-RUBRIC-003-S1 |
| RBAC 역할 (frozen, **0-diff [HARD]**) | `RoleAdmin`/`RoleAnalyst`/`RoleViewer` (`rbac.go:19-26`) — `RoleReviewer`/`evaluator` 신설 절대 금지 (**SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시**) | source-verified |
| Scope 정규식 (frozen) | `^iroum-ax:(admin|analyst|viewer)$` (`rbac.go:33`) | source-verified |
| ABAC 매핑 | admin=mutations / 모든 인증 incl. viewer=read+apply (frozen rbac.go에 모두 존재 → 충돌 자연 회피) | 본 SPEC §1.5 |
| Cross-store 점수 합산 TX 진입점 (apply 엔진) | `ScoreStore.BeginScoreTx`(`store.go:253-255`, `pg_store.go:134`) → `SumWeightedByEvaluationItem`(`store.go:292-295` `pgtype.Numeric` SEC-03 반환) | source-verified |
| Cross-store EvalItem 검증 TX 진입점 (criterion 추가) | `EvalItemStore.BeginEvalItemTx` → `GetEvalItemByID` | source-verified |
| 동일 TX 진입점 (신규) | `RubricStore.BeginRubricTx` (Recorder 주입 **`NewRecorder(true)` HARD — REVIEW-001 D1 iter2 lesson pre-applied**) | 본 SPEC §2.1 |
| Audit resource_id 규칙 | `rubrics.id` / `rubric_criteria.id` / `rubric_bands.id` UUID 직접 (D2 미러, namespace 상수 0) | 본 SPEC REQ-RUBRIC-UBI-002 |
| Apply read-only no-audit | `ApplyRubric`은 read-only이므로 `audit_logs` 0건 (§6 OPEN #6 결정 후 확정) | 본 SPEC REQ-RUBRIC-004-E1 |
| FK-less stub (criteria → eval_items) | `rubric_criteria.evaluation_item_id` UUID NOT NULL, FK 제약 없음 (SCORE-001 §1.4 / REVIEW-001 §1.4 동형) | 본 SPEC §1.4 |
| 내부 FK 허용 (criteria/bands → rubrics) | `rubric_criteria.rubric_id` / `rubric_bands.rubric_id` REFERENCES `rubrics(id)` (동일 마이그레이션 0006 내부 FK) | 본 SPEC §2.1 |
| 신규 마이그레이션 | 0006 (0001~0005 디스크 확인 후 비충돌) | 본 SPEC §1.4 |
| 멱등 패턴 | `CREATE TABLE IF NOT EXISTS` + `DO$$ EXCEPTION duplicate_object` + `CREATE INDEX IF NOT EXISTS` (0005 정확 미러) | 본 SPEC §1.4 |
| Store TX userID 시그니처 (**HARD**) | mutation 메서드는 명시적 `userID string` 파라미터 (**REVIEW-001 D1 iter2 lesson pre-applied** — iter1 fix 사이클 비재발) | 본 SPEC §1.4 / REQ-RUBRIC-001-E1/E2/E3 |
| `NewRecorder(true)` (**HARD**) | `pg_store.go BeginRubricTx`는 `audit.NewRecorder(true)`로 호출 (**REVIEW-001 D1 iter2 lesson pre-applied** — `false` 금지) | 본 SPEC §2.1 |
| server.go 마운트 줄 수 | **≈7줄** (필드+생성+innerMux.Handle 2줄+ko 주석 — REVIEW-001/SCORE-API-001/REPORT-001 정확 미러; **REPORT-001 lesson — "1줄"이 아닌 ≈7줄 정확 표기**) | 본 SPEC §2.1 |
| SCORE-001 `grade_thresholds` 병행 존재 | RUBRIC-001 `rubric_bands`와 병행 존재 (SCORE-001 0-diff [HARD], 호출자가 어느 모델 선택) | 본 SPEC §1.4 |

---

## 5. Exclusions (What NOT to Build)

본 SPEC이 의도적으로 제외하는 범위는 다음과 같다. 제외 항목은 본 SPEC 산출물(코드/SQL/감사/엔드포인트)에 포함되지 않으며 향후 별도 SPEC에서 다룬다.

1. **SCORE-001 `grade_thresholds` 테이블 수정 0건 (병행 존재만)**: SCORE-001 단일 테이블 등급 임계 모델(`grade_thresholds`, `Score.DetermineGrade` `store.go:296-298`)은 수정하지 않는다. RUBRIC-001 `rubric_bands`(rubric별 다중)와 병행 존재하며 호출자가 어느 모델을 사용할지 결정한다. SCORE-001 `0004_score_tables.sql` 0-diff [HARD].
2. **SCORE-001 / SCORE-API-001 / EVAL-ITEM-001 / EVID-001 / REPORT-001 / REVIEW-001 / AUTH-003 코드·스키마 수정**: 모든 기존 SPEC의 store, 핸들러, RBAC permissionMatrix, ABAC 정책, 마이그레이션 0001~0005 모두 0-diff. 본 SPEC은 consumer-only [HARD].
3. **Simulation 엔진 (rubric simulation_runs 4계층)**: PoC 범위는 3계층(rubric+criteria+bands)만. 4번째 계층(simulation_runs — 다양한 score 값에 대한 일괄 등급 산정 결과 저장)은 post-PoC 별도 SPEC.
4. **LLM 기반 rubric 자동 생성/추천**: 외부 호출 0(REQ-RUBRIC-UBI-001 정합). LLM 자동화는 post-PoC.
5. **AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC**: 검토자가 특정 조직단위에 속해야 한다는 추가 narrowing 미구현(AUTH-003 `User`/`Scopes` 모델만 사용). 풀 org-unit ABAC은 post-PoC.
6. **6번째 한국 공공 제약(시간 제약 — KST 업무시간 한정 mutations)**: 의도적 제외(본 SPEC은 데이터 주권·감사·언어·망분리·조직 격리만 적용, SCORE-API-001 / REPORT-001 / REVIEW-001 동형). 시간 제약은 post-PoC.
7. **Multi-org rubric 조율 / cross-organization rubric sharing**: PoC는 단일 조직 범위. multi-org 조율/공유는 post-PoC.
8. **Evaluator consensus engine (다중 평가자 합의 alg)**: PoC는 단일 평가자 모델. 다중 평가자 합의/투표 엔진은 post-PoC.
9. **물리 삭제 / hard delete**: archived 이후에도 행을 삭제하지 않는다(append-only, UBI-004 정합). 정정 경로는 신규 rubric 생성(draft INSERT)만.
10. **rubric simulation UI / dashboard**: HTTP API만 제공. UI/dashboard는 post-PoC 별도 SPEC.

---

## 6. OPEN Decisions — strategy phase 결정 사안 (7건, sub-agent + Human Gate)

다음 7건은 Run phase strategy phase에서 결정되어야 한다. Plan phase에서는 권장 옵션만 표시하며 spec.md/plan.md/acceptance.md는 권장안 기준으로 작성된다. 최종 결정은 strategy.md에서 5요소(결정/근거/거부대안/consumer-only/Run phase 적용) 부착 후 Human Gate sign-off로 RESOLVED.

### 6.1 Versioning endpoint shape [OPEN #1]

- **Option A** (권장 가능성): `PUT /api/v1/rubrics/{id}` 자동 버전 증분 — 단순, REST 의미 정합. 단점: PUT 멱등성 약화(server-side 자동 증분).
- **Option B**: `POST /api/v1/rubrics/{id}/clone-new-version` 별도 sub-resource — 명시적 의도, REVIEW-001 supersede 선례(`score_handlers.go:64`) 정합. 단점: 엔드포인트 수 증가.
- **Open**: 단순성(A) vs 명시성(B). REVIEW-001 `POST /reviews/{id}/approve` 별도 sub-resource 선례 정합도 고려.

### 6.2 Active rubric 1-per-(name, scope) constraint [OPEN #2]

- **Option A** (권장 가능성): PostgreSQL partial unique index `CREATE UNIQUE INDEX ... ON rubrics(name, scope) WHERE status='active'` — DB-level 보장, 결정성↑, 동시성 race 자동 차단.
- **Option B**: 핸들러-layer check + state-machine 가드 — 코드 복잡도↑, race window 가능성.
- **Open**: DB 보장(A) vs 코드 보장(B). REVIEW-001 §6.5/§6.6 이중 방어 패턴은 (A+B) 혼합도 가능.

### 6.3 Criteria weight sum validation [OPEN #3]

- **Option A**: DB CHECK constraint (sum 1.0 강제, 트리거 또는 deferred constraint) — DB 보장 강도↑, 단점: PostgreSQL trigger 복잡도, multi-row atomic check 어려움.
- **Option B** (권장 가능성): 핸들러 validation pre-store (REVIEW-001 §A.5 dual defense 패턴 정합) — 한국어 친화 에러, 단순, fail-closed.
- **Option C**: 합=1.0 강제하지 않음 — partial weight 허용. 단점: 의미적 일관성 약화.
- **Open**: A+B 이중 방어(REVIEW-001 §6.5 선례) 권장. C는 PoC 범위 외.

### 6.4 Band overlap prevention [OPEN #4]

- **Option A**: PostgreSQL `EXCLUSION USING gist (rubric_id WITH =, numrange(min_score, max_score) WITH &&)` — DB-level 결정적 범위 겹침 차단, 강도↑. 단점: gist 인덱스 비용, 마이그레이션 복잡도.
- **Option B** (권장 가능성): 핸들러 validation pre-store (`AddBand` 시 기존 bands 로드 → numrange 겹침 검사 → fail-closed `ErrRubricBandOverlap`) — REVIEW-001 §A.5 dual defense 정합, 한국어 친화 에러.
- **Open**: A+B 이중 방어 vs B 단독. REVIEW-001 §6.5 선례 정합.

### 6.5 Admin direct edit vs new-version-only policy [OPEN #5]

- **Option A** (권장 가능성): `active` rubric 직접 수정 허용 — admin이 active rubric의 metadata/criteria/bands 직접 편집 가능. 단점: audit 변경 추적이 단일 row update만으로 비-immutable.
- **Option B**: `active` immutable + clone-new-version 강제 — `active` 상태 진입 후에는 `archived` 외 변경 불가, 수정 시 새 version으로 clone. 장점: 불변성, 감사 추적 명확. 단점: 엔드포인트 수↑, UX 복잡도↑.
- **Open**: PoC 단순성(A) vs 감사 엄격성(B). 한국 공공 감사 요구 강도에 따라 결정.

### 6.6 ApplyRubric audit policy [OPEN #6]

- **Option A** (권장 가능성): read-only no-audit — `ApplyRubric`은 query-like 연산이므로 `audit_logs` 0건. REPORT-001 read-only no-audit 선례 정합.
- **Option B**: application-level 추적용 `audit_logs` 행 추가 — 보수적, 한국 공공 감사 추가 요구 가능성. 단점: read-only operation의 audit이 SCORE-001/REVIEW-001 패턴과 비대칭.
- **Open**: REPORT-001 선례(A) vs 한국 공공 감사 추가 요구(B). 본 SPEC §3 EARS는 권장안 A 기준 작성.

### 6.7 Archive reason 필드 작성 조건 [OPEN #7]

- **Option A** (권장 가능성): rubric archive 시 `archive_reason` TEXT 필수 (REVIEW-001 `rejection_reason` 패턴 동형, dual defense — handler validation + DB CHECK) — 감사 추적 강화.
- **Option B**: `archive_reason` optional — 단순성, 단점: archive 의도 추적 어려움.
- **Open**: REVIEW-001 §6.5 dual defense(A) vs 단순성(B). 한국 공공 감사 요구 강도.

> 본 7건 OPEN은 **SCORE-API-001 §6 OPEN #4(write 역할 매핑 `evaluator` 부재) 상속하지 않는다** — 본 SPEC의 역할 매핑(admin=mutations / 모든 인증 incl. viewer=read+apply)은 frozen `rbac.go`에 모두 존재(`rbac.go:19-26`)하므로 0-diff 자연 성립. SCORE-API-001 OPEN #4 같은 frozen RBAC 충돌은 발생하지 않는다. (SCORE-API-001 OPEN #4 evaluator-INFEASIBLE 비재발 명시 — REVIEW-001 §1.5/§6.4 동형 회피)

---

## 7. Edge Cases (acceptance.md §§별도 번호에서 세부)

acceptance.md §6에서 16 edge case(E1-E16)를 정의한다(SSOT). 본 §7은 핵심 15건 발췌 요약 — 전체 16건은 acceptance.md SSOT 우선:

1. rubric not-found (GET/PUT/POST sub-resource) → 404
2. weight CHECK violation (weight < 0 또는 > 1) → 400 fail-closed
3. band overlap (§6 OPEN #4 결정 후) → 400 `ErrRubricBandOverlap`
4. archived rubric mutation 시도 (update/criteria add/bands add) → 409 `ErrRubricArchived`
5. blank name 또는 name > 64 chars → 400 `ErrRubricInvalidInput`
6. viewer가 mutation 시도 (POST/PUT/DELETE) → 403 `ABAC_CONDITION_DENIED`
7. analyst가 mutation 시도 (특히 archive) → 403 `ABAC_CONDITION_DENIED`
8. auth-disabled 모드 모든 엔드포인트 투과 + `cli-anonymous` 기록 (UBI-003)
9. apply score out of all bands → 400 `ErrRubricInvalidInput` fail-closed
10. unknown rubric apply (rubric_id 미존재) → 404 cross-store
11. unknown evaluation_item_id 추가 시도 → 404 cross-store
12. min_score >= max_score 검증 → 400 fail-closed
13. `RecordRubric*` audit INSERT 실패 → `ErrRubricAuditWriteFailed` + 호출자 Rollback → entity+audit 양방향 취소
14. 불법 전이 (archived → active 등) → 409 `ErrRubricInvalidStatus`
15. malformed UUID path param → 400 fail-closed

---

## 8. 참조

- `apps/control-plane/internal/store/store.go` (ScoreStore/ScoreTx `:253-305`, EvalItemStore/Tx 패턴 `:115-119` + REVIEW-001 ScoreReviewRequestStore/Tx 동형 미러 대상)
- `apps/control-plane/internal/store/score.go` (`SumWeightedByEvaluationItem` `store.go:292-295` `pgtype.Numeric` SEC-03 반환 — apply 엔진 cross-store 의존)
- `apps/control-plane/internal/store/pg_store.go` (`BeginScoreTx` `pg_store.go:134` Recorder 주입 — **REVIEW-001 D1 iter2 `NewRecorder(true)` 동형 미러** HARD)
- `apps/control-plane/internal/store/score_review_request.go` (REVIEW-001 `PgScoreReviewRequestTx` — 패턴 미러 대상, `userID string` 파라미터 시그니처 정확 미러)
- `apps/control-plane/internal/store/eval_item.go` (`GetEvalItemByID` — cross-store criterion 검증 의존)
- `apps/control-plane/cmd/server/review_handlers.go` (REVIEW-001 핸들러+`requireReviewAdminRole`/`guardReviewAdmin` 동형 미러 대상)
- `apps/control-plane/cmd/server/score_handlers.go` (`mapStoreErr` `:111-129`, `clampPagination` `:144-157`, `writeScoreJSON`/`writeScoreErr` `:74-101`, 한국어 에러 메시지 `:114-127` — 헬퍼 패턴 미러)
- `apps/control-plane/internal/auth/rbac.go` (frozen 3-role `:19-26/:33/:68-80` — 0-diff [HARD], **RoleReviewer/evaluator 신설 절대 금지** SCORE-API-001 OPEN #4 비재발 명시)
- `apps/control-plane/internal/auth/abac.go` (narrowing-only/admin 우회/auth-disabled 투과)
- `apps/control-plane/internal/errors/errors.go` (센티넬 패턴 `:52-78` 동형 추가 대상 — **SCORE-API-001 errors.go drift 교훈 명시 부착**)
- `apps/control-plane/cmd/server/server.go` (`scoreH`/`reportH`/`reviewH` 필드 `:55-57`, 생성 `:210-213`, 마운트 `:266-267/:269-270` — **REPORT-001 lesson ≈7줄 정확 미러** HARD)
- `.moai/db/schema/migrations/0004_score_tables.sql` (SCORE-001 `grade_thresholds` — **0-diff [HARD] 병행 존재**)
- `.moai/db/schema/migrations/0005_score_review_request_tables.sql` (REVIEW-001 — 멱등 패턴 정확 미러 대상)
- `.moai/specs/SPEC-AX-RUBRIC-001/research.md` (SSOT — file:line 근거)
- `.moai/specs/SPEC-AX-SCORE-001/spec.md` (store+audit + grade_thresholds 단일 테이블 모델 선례)
- `.moai/specs/SPEC-AX-SCORE-API-001/spec.md` (HTTP API + ABAC + errors.go drift lesson + OPEN #4 evaluator-INFEASIBLE lesson)
- `.moai/specs/SPEC-AX-REPORT-001/spec.md` (consumer-only + ≈7줄 server.go + cross-store 2-TX 선례)
- `.moai/specs/SPEC-AX-REVIEW-001/spec.md` (가장 최근 store+API 수직 슬라이스 + frontmatter v0.1.1 + D1 iter2 lesson 정확 미러 대상, commit 89a8828)

---

> SSOT: research.md(file:line 근거). 본 spec.md의 모든 기술적 주장은 research.md 또는 source 파일 직접 확인을 통해 검증됨(phantom 0건). 세션 lessons #1-#12 + REVIEW-001 D1 iter2 + SCORE-API-001 errors.go drift + REPORT-001 server.go ≈7줄 모든 lesson은 작성 시점부터 pre-applied되어 iter1 fix 사이클 비재발.
