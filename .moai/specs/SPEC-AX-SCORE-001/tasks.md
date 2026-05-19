# SPEC-AX-SCORE-001 Tasks (Phase 1.5 Decomposition)

> SPEC: SPEC-AX-SCORE-001 v0.1.2 (경영평가 점수 산출/집계)
> Methodology: TDD (RED-GREEN-REFACTOR), brownfield enhancement · Harness: thorough
> Mode: Agent Teams (backend-dev implementer + tester, 병렬, isolation:worktree)
> Source: spec.md v0.1.2 §3 EARS / plan.md v0.1.2 §2·§3·§4 Sprint S0~S6 / acceptance.md 17 AC·16 edge / strategy.md §A (§6 4건 RESOLVED)
> §6 RESOLVED: D1=Option A (단일 scores + level discriminator) · D2=Option 1 (직접 score.id UUID, surrogate 0) · D3=최소 grade_thresholds 테이블 · D4=status state-machine + append-only

## §0. 결정 사항 (Human Gate sign-off — v0.1.2 확정, 구현 시 변경 금지)

- **D1 (테이블)**: 단일 `scores` 테이블 + `level` discriminator ∈ {raw,item,category}. Option B(scores+score_aggregates) 기각 — 1 TX=1 entity=1 audit 불변식 위반.
- **D2 (audit resource_id)**: `Event.ResourceID = score.id` 직접 대입 (UUID PK). [HARD] `audit.go` 신규 namespace 상수 **0건**, `recorder.go` `uuid.NewSHA1` 호출 **0건** — AUD-1 surrogate 미적용 (EVAL-ITEM-001과 다름).
- **D3 (grade-threshold)**: 별도 `grade_thresholds`(scope, letter, min_value, boundary_rule, PK(scope,letter)) 4컬럼 테이블. metadata JSONB 아님. `boundary_rule` 기본 `gte` (score ≥ min_value, S→D 내림차순 스캔).
- **D4 (불변/정정)**: status state-machine — DRAFT 행 in-place 가변 / CONFIRMED 행 score 필드 불변 / CONFIRMED 정정 = 신규 행 INSERT + 구 행 `CONFIRMED→SUPERSEDED` (동일 TX) / `DeleteScore` 메서드 미존재(물리 삭제 0건). 전이표: `DRAFT→DRAFT`·`DRAFT→CONFIRMED`·`CONFIRMED→SUPERSEDED`, SUPERSEDED terminal.

## §1. 파일 소유권 (Agent Teams — 병렬 worktree 충돌 방지) [HARD]

| Role | 배타 소유 파일 | 비고 |
|------|----------------|------|
| **backend-dev** (implementer, isolation:worktree, mode:acceptEdits) | `apps/control-plane/internal/store/store.go`, `apps/control-plane/internal/store/score.go`, `apps/control-plane/internal/store/pg_store.go`, `apps/control-plane/internal/audit/audit.go`, `apps/control-plane/internal/audit/recorder.go`, `.moai/db/schema/migrations/0004_score_tables.sql` | 프로덕션 코드 + 마이그레이션. `*_test.go` 미수정. 프롬프트 상대경로만 (`cd /abs` 금지) |
| **tester** (tester, isolation:worktree, mode:acceptEdits) | `apps/control-plane/internal/store/score_test.go` [NEW], `apps/control-plane/internal/audit/recorder_score_test.go` [NEW], `apps/control-plane/internal/store/pg_store_test.go` 회귀 확인 [EXISTING] | `*_test.go` **배타 소유**. 프로덕션 코드 미수정 |

[HARD] 두 role 모두 implementer/tester → `isolation: "worktree"` 필수. TDD: tester가 RED 작성 → backend-dev가 GREEN. 동일 모듈 RED/GREEN 동기화는 공유 TaskList + SendMessage 조정. `postgres.go`는 Sprint-0 死 스텁 — **비대상**(BeginScoreTx는 `pg_store.go` PgWorkflowStore.pool에만).

## §2. Drift-Guard Authoritative Planned-Files Manifest

정확히 아래 파일만 변경된다. 이 목록을 벗어나는 수정은 drift로 간주 (workflow-modes.md Drift Guard, 누적 30% 초과 시 Re-planning Gate).

| Delta | 파일 | 변경 내용 |
|-------|------|-----------|
| [NEW] | `.moai/db/schema/migrations/0004_score_tables.sql` | `scores` + `grade_thresholds` 2테이블 멱등 (0003 규약) |
| [NEW] | `apps/control-plane/internal/store/score.go` | `PgScoreTx` pgx 구현 (`eval_item.go` 미러) |
| [NEW] | `apps/control-plane/internal/store/score_test.go` | testcontainers 통합 테스트 |
| [NEW] | `apps/control-plane/internal/audit/recorder_score_test.go` | audit 원자성/rollback 테스트 |
| [MODIFY] | `apps/control-plane/internal/store/store.go` | `ScoreStore`/`ScoreTx` 인터페이스 추가 |
| [MODIFY] | `apps/control-plane/internal/store/pg_store.go` | `BeginScoreTx` 진입점 (실 `PgWorkflowStore.pool` 재사용; `postgres.go` 비대상 [HARD]) |
| [MODIFY] | `apps/control-plane/internal/audit/audit.go` | `ActionScoreCreated`/`ActionScoreUpdated` 상수 **2개만** (신규 namespace 상수 0 — D2) |
| [MODIFY] | `apps/control-plane/internal/audit/recorder.go` | `RecordScoreCreated`/`RecordScoreUpdated` (로컬 `AuditTx`, `resource_id=score.id` 직접, `uuid.NewSHA1` 0 — D2) |
| [MODIFY] | `apps/control-plane/internal/errors/errors.go` | `ErrScore*` 센티널 가산 (`ErrEvalItem*`/`ErrEvidence*` 선례 미러, additive-only). GAN iter2 확정: `ErrScoreNotFound`/`ErrScoreInvalidInput`/`ErrScoreImmutable`/`ErrScoreInvalidStatus`/`ErrGradeThresholdsUnavailable`/`ErrScoreAuditWriteFailed`/`ErrScoreNotConfirmed`. 계약 `errors.Is` 단언(DC-001-U1, DC-UBI-004, DC-003-S1)에 필수 |
| [EXISTING] | `apps/control-plane/internal/store/pg_store_test.go` | Workflow/Evidence/EvalItem 특성화 회귀 확인 (수정 없음) |
| [EXISTING] | `.moai/db/schema/initial.sql` | 참조 only, 미수정 (schema drift 방지) |
| [EXISTING] | `.moai/db/schema/migrations/0002_evidence_tables.sql` | 참조 only, 미수정 (FK 하드닝 범위 밖) |
| [EXISTING] | `.moai/db/schema/migrations/0003_eval_item_tables.sql` | 참조 only, 미수정 (FK 하드닝 범위 밖) |

## §3. Atomic Task Table

각 태스크는 단일 TDD RED-GREEN-REFACTOR 사이클로 완결. [DELTA] 순서: [EXISTING] 특성화 baseline → [MODIFY] 특성화→수정→검증 → [NEW] full RED-GREEN-REFACTOR. Status 초기값 = pending.

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | [EXISTING][S0] 특성화 baseline 확립: 기존 `pg_store_test.go`의 Workflow/Evidence/EvalItem 테스트 + `internal/audit` Recorder 테스트가 score 와이어링 전 GREEN임을 실행 확인. `0004_score_tables.sql` 디스크 비충돌(0001/0002/0003만 존재) 재확인. §6 4건 RESOLVED 코드 동결 사실 기록. (tester) | 전제 (회귀 baseline) | — | `apps/control-plane/internal/store/pg_store_test.go` [EXISTING], `.moai/db/schema/migrations/` [EXISTING] | pending |
| T-002 | [NEW][S1] `0004_score_tables.sql` 멱등 작성 — **2테이블**: (1) `scores`(id UUID PK DEFAULT uuid_generate_v4(), evaluation_item_id VARCHAR(64) NOT NULL FK없음, evidence_id UUID NULL FK없음, level VARCHAR(16) DEFAULT 'raw', score_value DECIMAL(6,2), weight DECIMAL(5,4) NULL, grade VARCHAR(2) NULL, status VARCHAR(32) DEFAULT 'DRAFT', metadata JSONB, created_at/created_by 'cli-anonymous'/updated_at) + 인덱스(evaluation_item_id/evidence_id/level) + CHECK(level∈{raw,item,category}, status∈{DRAFT,CONFIRMED,SUPERSEDED}, grade∈{NULL,S,A,B,C,D}); (2) `grade_thresholds`(scope VARCHAR(64), letter VARCHAR(2), min_value DECIMAL(6,2), boundary_rule VARCHAR(8) DEFAULT 'gte', PK(scope,letter)) + CHECK(letter∈{S,A,B,C,D}, boundary_rule∈{gte,gt}). 멱등 패턴 `CREATE TABLE IF NOT EXISTS`/`DO $$ EXCEPTION WHEN duplicate_object`/`CREATE INDEX IF NOT EXISTS` (0003 규약 미러). tester가 RED(테이블/컬럼타입/제약 information_schema 단언) 작성 → backend-dev GREEN. (backend-dev / tester RED) | REQ-SCORE-001, REQ-SCORE-001-S1, D1, D3, D4 | T-001 | `.moai/db/schema/migrations/0004_score_tables.sql` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-003 | [MODIFY][S1] `store.go`에 `ScoreStore` 인터페이스(`BeginScoreTx(ctx) (ScoreTx, error)`) + `ScoreTx` 인터페이스(`InsertScore, GetScoreByID, GetScoresByEvaluationItem, UpdateScore, SumWeightedByEvaluationItem, InsertAuditLog, Commit, Rollback`) 추가. 기존 `EvalItemStore`/`EvalItemTx`(store.go:114-181) 2계층 패턴 그대로 미러. 특성화 회귀: 기존 인터페이스 변경 0. (backend-dev / tester RED) | REQ-SCORE-001 | T-002 | `apps/control-plane/internal/store/store.go` [MODIFY]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-004 | [NEW]+[MODIFY][S2] `score.go` `PgScoreTx` 구현(`eval_item.go:31` 미러): `InsertScore`(검증 후 INSERT, UUID 반환), `GetScoreByID`, `GetScoresByEvaluationItem`, `UpdateScore` 골격, `InsertAuditLog`, `Commit`, `Rollback` + `nullIfEmpty` 식 nullable 처리. `pg_store.go`에 `BeginScoreTx` 추가 — [HARD] 실 `PgWorkflowStore.pool`(`s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})`, `BeginEvalItemTx` pg_store.go:118 미러), `stderrors.ErrPgxPoolExhausted` 래핑, **`postgres.go` 死 스텁 비대상**. tester RED(InsertScore happy path + evidence_id UUID stub FK없음 + evaluation_item_id VARCHAR(64) stub FK없음) → backend-dev GREEN. | REQ-SCORE-001, REQ-SCORE-001-E1, REQ-SCORE-001-S1 | T-003 | `apps/control-plane/internal/store/score.go` [NEW], `apps/control-plane/internal/store/pg_store.go` [MODIFY]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-005 | [NEW][S2] `score.go` 입력 검증 pre-INSERT: `evaluation_item_id` blank/64자 초과, `score_value` 비수치/누락, `evidence_id` 비-UUID → 구조화 검증 에러(client error, INFO 로그), `scores`/`audit_logs` 변화 0. tester RED(4 거부 케이스 a/b/c/d) → backend-dev GREEN(`validateScoreInput` 골격, REFACTOR는 T-015). | REQ-SCORE-001-U1 | T-004 | `apps/control-plane/internal/store/score.go` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-006 | [NEW][S2] `score.go` metadata JSONB opaque verbatim 영속 — caller 제공 임의 구조(한글 키 포함) 구조 해석/검증 없이 JSONB 컬럼 저장, semantic value-equality round-trip. tester RED([HARD] byte-level 비교 금지 — `map[string]any` unmarshal + `reflect.DeepEqual`/`jsonEqual()`) → backend-dev GREEN. | REQ-SCORE-001-O1 | T-004 | `apps/control-plane/internal/store/score.go` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-007 | [MODIFY][S3] `audit.go`에 `ActionScoreCreated Action = "SCORE_CREATED"`, `ActionScoreUpdated Action = "SCORE_UPDATED"` **상수 2개만** 추가. [HARD] 신규 `ScoreAuditNamespace` 등 namespace 상수 추가 **금지**(D2 — `scores.id` UUID PK 직접 대입, AUD-1 surrogate 미적용). 기존 `Action`/`ActionEvalItemCreated` 패턴(audit.go:64-67). tester RED(상수 존재 + 값) → backend-dev GREEN. | REQ-SCORE-004, D2 | T-001 | `apps/control-plane/internal/audit/audit.go` [MODIFY]; `apps/control-plane/internal/audit/recorder_score_test.go` [NEW] (RED) | pending |
| T-008 | [MODIFY][S3] `recorder.go`에 `RecordScoreCreated`/`RecordScoreUpdated` 추가(`RecordEvalItemCreated` recorder.go:332 시그니처 패턴, 로컬 `AuditTx` recorder.go:29 — store→audit 순환 의존 회피). [HARD] `Event.ResourceID = score.id` **직접 대입**(D2 Option 1, surrogate 아님; `resource_id != uuid.Nil` 항상 성립), `uuid.NewSHA1` 호출 **0건**. `Action`∈{SCORE_CREATED,SCORE_UPDATED}, `ResourceType="score"`, `UserID=resolveUserID(userID)`, `DetailsJSON={score_id, evaluation_item_id, evidence_id?, level, grade?, status?}`, 동일 `AuditTx` INSERT. tester RED(audit.Event 필드 + resource_id byte-identical score.id + DetailsJSON + namespace/NewSHA1 0건 정적 검사) → backend-dev GREEN. | REQ-SCORE-004, REQ-SCORE-004-E1, REQ-SCORE-UBI-002, D2 | T-007, T-004 | `apps/control-plane/internal/audit/recorder.go` [MODIFY]; `apps/control-plane/internal/audit/recorder_score_test.go` [NEW] (RED) | pending |
| T-009 | [NEW][S3] `score.go` 생성/수정 경로 동일-TX 원자성 + 양방향 rollback: InsertScore/UpdateScore → RecordScore* → Commit 단일 TX; `audit_logs` INSERT 실패 시 `tx.Rollback(ctx)`로 `scores` 변경도 롤백(부분 커밋 0), wrapped audit-insertion 에러 반환, goroutine 누출 0. tester RED(fault injection: audit INSERT CHECK 위반 강제 → scores row 부재 + `goleak.VerifyNone(t)`) → backend-dev GREEN. | REQ-SCORE-004-U1, REQ-SCORE-UBI-002 | T-008 | `apps/control-plane/internal/store/score.go` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW], `apps/control-plane/internal/audit/recorder_score_test.go` [NEW] (RED) | pending |
| T-010 | [NEW][S4] `score.go` `SumWeightedByEvaluationItem(evaluationItemID)` — 단일 read TX, raw-level 자식 행에 대해 `Σ(score_value × weight)` DECIMAL 정확 합산, `scores_evaluation_item_id_idx` 인덱스 사용(EXPLAIN). NULL weight: **단일 고정 결정적 정책**(제외 또는 구조화 에러 중 하나로 S4에서 확정 — 조용한 0/1 강제 금지, 동일 입력→동일 결과). 깊은 재귀 미수행. tester RED(알려진 집합 83.00 정확성 + NULL weight 2회 호출 동일 결과 + EXPLAIN 인덱스) → backend-dev GREEN(`computeWeightedSum` 헬퍼). | REQ-SCORE-002, REQ-SCORE-002-E1, REQ-SCORE-002-U1 | T-004 | `apps/control-plane/internal/store/score.go` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-011 | [NEW][S5] `score.go` 등급 산출 — `grade_thresholds` 테이블 `SELECT ... WHERE scope=$1` 후 S→D 내림차순 `min_value` 스캔, `boundary_rule`(기본 `gte`: score≥min_value) 기준 결정적 letter 매핑. [HARD] metadata JSONB 미사용(D3). scope 0행 → 구조화 "grade thresholds unavailable" 에러(등급 fabricate 0). 경계값 동일 입력→동일 등급. 외부 호출 0(내부 테이블 lookup만). tester RED(83.00→A / 80.00 경계 gte→A 2회 동일 / unknown-scope 0행→구조화 실패 + fabricate 0) → backend-dev GREEN(`resolveGrade` 헬퍼). | REQ-SCORE-003, REQ-SCORE-003-E1, REQ-SCORE-003-S1, REQ-SCORE-003-U1, REQ-SCORE-UBI-001, D3 | T-002, T-004 | `apps/control-plane/internal/store/score.go` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-012 | [NEW][S5] `score.go` status state-machine — `validateScoreStatusTransition` 가드(`eval_item.go:254` `validateStatusTransition` 스타일): DRAFT 행 `UpdateScore` score 필드(score_value/weight/grade) in-place 가변; CONFIRMED 행 score 필드 변경은 구조화 에러 거부; 허용 전이 `DRAFT→DRAFT`/`DRAFT→CONFIRMED`/`CONFIRMED→SUPERSEDED`만, SUPERSEDED terminal; CONFIRMED 정정 = 신규 행 INSERT + 구 행 `CONFIRMED→SUPERSEDED`(동일 `ScoreTx`, 각 변경 1 audit row); `DeleteScore` 메서드 **미존재**(물리 삭제 0). tester RED(DRAFT in-place 가변 / CONFIRMED in-place 거부 / DRAFT→CONFIRMED 전이 / CONFIRMED 정정=신규행+SUPERSEDED 동일 TX / 물리 DELETE 0) → backend-dev GREEN. | REQ-SCORE-001-S2, REQ-SCORE-UBI-004, D4 | T-008, T-004 | `apps/control-plane/internal/store/score.go` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW] (RED) | pending |
| T-013 | [NEW][S5/S6] 횡단 UBI 불변 검증: (UBI-001) 생성/롤업/등급/감사 경로 외부 host DNS/TCP 0건 + `internal/store`·`internal/audit` 외부 SaaS/LLM SDK 미import 정적 검사; (UBI-002) 모든 create/update 동일 TX audit_logs 1건; (UBI-003) AuthN disabled 시 `scores.created_by`/`audit_logs.user_id`='cli-anonymous' literal byte-identical(NULL 금지, `resolveUserID` 재사용); (UBI-004) 물리 삭제 0 + status state-machine(T-012 보강). tester RED → backend-dev GREEN(기존 `resolveUserID`/내부 pool 재사용으로 대부분 충족, 누락분 보강). | REQ-SCORE-UBI-001, REQ-SCORE-UBI-002, REQ-SCORE-UBI-003, REQ-SCORE-UBI-004 | T-009, T-011, T-012 | `apps/control-plane/internal/store/score.go` [NEW]; `apps/control-plane/internal/store/score_test.go` [NEW], `apps/control-plane/internal/audit/recorder_score_test.go` [NEW] (RED) | pending |
| T-014 | [EXISTING][S6] 경계 검증 AC-SCORE-BOUNDARY-1: `0004` 적용 전후 information_schema(`referential_constraints`/`table_constraints`)로 `scores.evaluation_item_id`→`evaluation_items` / `scores.evidence_id`→`evidences` FK **부재** 확인, `evidences`/`evaluation_items`/`0002`/`0003` 스키마 불변(본 SPEC 미수정), `scores.evaluation_item_id`↔`evaluation_items.id`=character varying(64) / `scores.evidence_id`↔`evidences.id`=uuid 타입 호환. tester RED(FK 존재 가정 → schema 불일치) → backend-dev: FK 미추가 확인(코드 변경 없음, 0004가 FK 미생성임을 보증). | spec.md §5 #3 / §7 (FK 하드닝 out-of-scope) | T-002 | `apps/control-plane/internal/store/score_test.go` [NEW] (RED); `.moai/db/schema/migrations/0004_score_tables.sql` [NEW] (FK 미생성 보증) | pending |
| T-015 | [NEW][S6] REFACTOR + 품질 게이트: 헬퍼 분리(`validateScoreInput`/`computeWeightedSum`/`resolveGrade`/`validateScoreStatusTransition`, 단일 함수 복잡도 ≥15 회피 — `eval_item.go` `validateStatusTransition`/`buildEvalItemUpdateSet` 선례), @MX 태그(BeginScoreTx/RecordScore* `@MX:ANCHOR` fan_in≥3, SumWeightedByEvaluationItem/grade 산출 `@MX:WARN`+`@MX:REASON`, InsertScore/UpdateScore RED `@MX:TODO`→GREEN `@MX:NOTE`), 성능(점수 생성·롤업 p99<50ms 10회 반복), 기존 Workflow/Evidence/EvalItem 특성화 회귀 0, coverage≥85%, golangci-lint default+gosec 0 issue, `goleak.VerifyNone(t)` 전 테스트, manager-quality TRUST 5, evaluator-active per-sprint strict≥0.75. (backend-dev REFACTOR + tester 회귀/커버리지) | 전체 (REQ-SCORE-UBI-001~004 + 001~004 전 sub-clause) | T-005, T-006, T-009, T-010, T-011, T-012, T-013, T-014 | `apps/control-plane/internal/store/score.go` [NEW], `apps/control-plane/internal/store/pg_store.go` [MODIFY], `apps/control-plane/internal/audit/recorder.go` [MODIFY]; `apps/control-plane/internal/store/score_test.go` [NEW], `apps/control-plane/internal/audit/recorder_score_test.go` [NEW], `apps/control-plane/internal/store/pg_store_test.go` [EXISTING] | pending |

총 15 atomic 태스크 (≤최대 권장, SDD atomic 표준). 각 단일 TDD 사이클 완결.

## §4. Sprint 매핑 (plan.md §4 S0~S6)

| Sprint | 우선순위 | Tasks |
|--------|----------|-------|
| S0 (특성화 baseline) | High | T-001 |
| S1 (DDL 2테이블 + 인터페이스) | High | T-002, T-003 |
| S2 (PgScoreTx + BeginScoreTx + 검증 + metadata) | High | T-004, T-005, T-006 |
| S3 (audit 상수 + Recorder + 원자성/rollback) | High | T-007, T-008, T-009 |
| S4 (가중 롤업) | Medium | T-010 |
| S5 (등급 + status state-machine + UBI 횡단) | Medium | T-011, T-012, T-013 |
| S6 (경계 + REFACTOR + 품질 게이트) | Medium | T-014, T-015 |

## §5. AC ↔ Task Coverage Matrix (17 AC 전부 ≥1 태스크)

| AC | Task(s) | | AC | Task(s) |
|----|---------|--|----|---------|
| AC-SCORE-UBI-001 (데이터 주권) | T-013, T-011 | | AC-SCORE-001-O1-1 (metadata opaque) | T-006 |
| AC-SCORE-UBI-002 (감사 완전성) | T-009, T-013 | | AC-SCORE-002-1 (가중합 정확성) | T-010 |
| AC-SCORE-UBI-003 (cli-anonymous) | T-013 | | AC-SCORE-002-2 (NULL weight 결정적) | T-010 |
| AC-SCORE-UBI-004 (불변/무삭제 state-machine) | T-012, T-013 | | AC-SCORE-003-1 (등급 결정 매핑) | T-011 |
| AC-SCORE-001-1 (생성+감사 원자 커밋) | T-004, T-009 | | AC-SCORE-003-2 (경계/미설정 결정적) | T-011 |
| AC-SCORE-001-2 (evidence_id UUID stub) | T-002, T-004 | | AC-SCORE-004-1 (RecordScore* audit row) | T-008 |
| AC-SCORE-001-3 (eval_item_id VARCHAR(64) stub) | T-002, T-004 | | AC-SCORE-004-2 (audit fail 양방향 rollback) | T-009 |
| AC-SCORE-001-4 (입력 검증 거부) | T-005 | | AC-SCORE-BOUNDARY-1 (FK 부재+미수정) | T-014 |
| AC-SCORE-001-S2 (status state-machine) | T-012 | | | |

**coverage_verified = true**: 17/17 AC 매핑. REQ sub-clause: REQ-SCORE-UBI-001~004(T-013 등) · 001-E1(T-004) · 001-S1(T-002,T-004) · 001-S2(T-012) · 001-O1(T-006) · 001-U1(T-005) · 002-E1(T-010) · 002-U1(T-010) · 003-E1(T-011) · 003-S1(T-011) · 003-U1(T-011) · 004-E1(T-008) · 004-U1(T-009) — 모든 E/S/O/U sub-clause(001-S2 포함) ≥1 태스크.

## §6. 16 Edge Case 매핑 (acceptance.md §7, 16행)

생성+audit 원자 커밋(T-009) · evidence_id UUID stub(T-002/4) · eval_item_id VARCHAR(64) stub(T-002/4) · 입력 4종 거부(T-005) · metadata opaque(T-006) · 단일레벨 가중합(T-010) · NULL weight 결정적(T-010) · score→letter 결정(T-011) · 경계/미설정 결정적 실패(T-011) · audit fail 양방향 rollback(T-009) · store→audit 순환의존 회피(T-008) · 물리삭제0+CONFIRMED 정정=신규행+SUPERSEDED(T-012) · status state-machine DRAFT가변/CONFIRMED불변(T-012) · FK부재+미수정(T-014) · cli-anonymous(T-013) · 외부 LLM/저장 호출 부적격(T-013) = 16/16.

## §7. 리스크 (plan.md §7 R-SCORE-001~008)

R-001(§6 미확정) — 해소(v0.1.2 RESOLVED, 본 tasks가 확정 반영). R-002(FK 잘못 추가) — T-014. R-003(DECIMAL/NULL weight) — T-010 @MX:WARN. R-004(등급 경계 비결정) — T-011 boundary_rule. R-005(audit 부분 커밋) — T-009. R-006(phantom postgres.go) — T-004 [HARD] pg_store.go만. R-007(Option B saga) — D1 기각, 단일 scores 유지. R-008(rubric/LLM scope 폭증) — T-011 grade_thresholds 4컬럼 floor, 내부 threshold만.

## §8. Definition of Done (Phase 1.5)

- [ ] tasks.md 15 atomic 태스크, 각 단일 TDD 사이클 완결
- [ ] Status 컬럼 전부 pending
- [ ] Drift-Guard manifest §2 = spec.md §2 / plan.md §2 일치 ([NEW]/[MODIFY]/[EXISTING])
- [ ] 17 AC 전부 ≥1 태스크 (coverage_verified=true), 16 edge 매핑
- [ ] §6 4건 RESOLVED 반영 (D1 단일 scores / D2 직접 UUID·namespace 0 / D3 grade_thresholds / D4 status state-machine)
- [ ] file-ownership 분리 (backend-dev 프로덕션+0004 / tester *_test.go 배타), 각 태스크 role 표기
- [ ] [DELTA] 순서 ([EXISTING]→[MODIFY]→[NEW])
- [ ] phantom-path [HARD]: BeginScoreTx → pg_store.go PgWorkflowStore.pool (postgres.go 비대상)
- [ ] D2 [HARD]: audit.go namespace 상수 0 / recorder.go uuid.NewSHA1 0
- [ ] tasks.md git 추적 대상
