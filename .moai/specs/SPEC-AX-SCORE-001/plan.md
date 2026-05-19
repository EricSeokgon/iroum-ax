# SPEC-AX-SCORE-001 Implementation Plan

> Version: 0.1.2
> Methodology: TDD (RED-GREEN-REFACTOR), brownfield enhancement
> Harness: thorough
> Companion: `spec.md` (EARS), `acceptance.md` (Given/When/Then), `research.md` (Phase 0.5 근거), `strategy.md` (Run Phase 1 분석 — §6 RESOLVED 근거)

본 plan은 SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 plan.md 구조를 미러링한다. **§6 4건은 Run Phase 1 manager-strategy 분석(`strategy.md` §A) + Human Gate 사용자 sign-off로 v0.1.2에서 RESOLVED 완료** (SPEC-AX-EVAL-ITEM-001 §6 OPEN→RESOLVED 흐름 동위상). §3 DDL은 잠정에서 확정으로 승격(2테이블), §6은 OPEN/CONFIRMABLE → RESOLVED.

---

## 1. 목표 & 범위

1차 산출물 = **기초 점수 데이터 모델 + store 계층 + audit 연계 Walking Skeleton** (spec.md §1.1). 가중 롤업 엔진·등급 산출은 **모델링 + 최소 구현**. 풀 집계 엔진·풀 rubric·REST/Console·LLM 시뮬레이션/Recommendation·EVID/EVAL-ITEM FK 하드닝은 범위 밖 (spec.md §5).

핵심 불변식 (research.md §9):
- `scores.evaluation_item_id` = `VARCHAR(64)` FK 없는 stub (EVAL-ITEM-001 §1.4 호환)
- `scores.evidence_id` = `UUID` nullable FK 없는 stub (EVID-001 호환)
- 단일 pgx 풀 재사용 (`PgWorkflowStore.pool`, `pg_store.go` — `postgres.go` 死 스텁 비대상)
- 1 TX = 1 entity = 1 audit row (Option B 2-테이블 집계 saga 기각 — EVAL-ITEM Option B 기각 동일 선례)
- `initial.sql` / `0002` / `0003` 미수정 (additive only)

---

## 2. 영향받는 파일 (Delta)

spec.md §2 표를 따른다. Delta 마커:

| 경로 | Delta | 비고 |
|------|-------|------|
| `internal/store/store.go` | [MODIFY] | `ScoreStore`/`ScoreTx` 인터페이스 추가 (Workflow/Evidence/EvalItem 패턴) |
| `internal/store/score.go` | [NEW] | `PgScoreTx` pgx 구현 (`eval_item.go:31-36` 미러) |
| `internal/store/pg_store.go` | [MODIFY] | `BeginScoreTx` 진입점 (`s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: ReadCommitted})`, `BeginEvidenceTx`/`BeginEvalItemTx` 선례). **`postgres.go` 死 스텁 비대상** |
| `internal/audit/audit.go` | [MODIFY] | `ActionScoreCreated`/`ActionScoreUpdated` 상수 (+ §6 audit-id 전략이 Option 2일 경우에만 namespace 상수) |
| `internal/audit/recorder.go` | [MODIFY] | `RecordScoreCreated`/`RecordScoreUpdated` (로컬 `AuditTx`, store→audit 순환 의존 회피) |
| `.moai/db/schema/migrations/0004_score_tables.sql` | [NEW] | `scores` (+ §6에서 별도 테이블 결정 시 `grade_thresholds`) 멱등 SQL |
| `.moai/db/schema/initial.sql` | [EXISTING] | 참조 only, 미수정 (특성화 회귀 0) |
| `.moai/db/schema/migrations/0002_evidence_tables.sql` | [EXISTING] | 참조 only, 미수정 (FK 하드닝 범위 밖) |
| `.moai/db/schema/migrations/0003_eval_item_tables.sql` | [EXISTING] | 참조 only, 미수정 (FK 하드닝 범위 밖) |
| `internal/store/score_test.go` | [NEW] | testcontainers 통합 테스트 |
| `internal/audit/recorder_score_test.go` | [NEW] | audit 원자성/rollback 테스트 |
| `internal/store/pg_store_test.go` | [EXISTING] | 기존 Workflow/Evidence/EvalItem 특성화 회귀 검증 |

[EXISTING] 특성화: score 와이어링 추가 후 기존 `pg_store_test.go`의 Workflow/Evidence/EvalItem 테스트가 GREEN 유지(회귀 0)임을 RED 단계 진입 전 확인.

---

## 3. DDL (research.md §10, §4 멱등 규약 — §6 RESOLVED 확정, 2테이블)

> **확정**: §6 4건 RESOLVED(v0.1.2, strategy.md §A + Human Gate). 아래는 **Decision 1=Option A(단일 scores + level discriminator) + Decision 3=`grade_thresholds` 테이블 확정 DDL**이다 (잠정 → 확정 승격). `0004_score_tables.sql`은 **2개 테이블**을 생성한다.

```sql
-- 0004_score_tables.sql  (멱등: 0002/0003 규약 — research.md §4) — Decision 1+3+4 RESOLVED
-- 테이블 1/2: scores (Decision 1 Option A 단일테이블 + level discriminator)
CREATE TABLE IF NOT EXISTS scores (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),  -- Decision 2: Event.ResourceID 직접 대입
    evaluation_item_id  VARCHAR(64)  NOT NULL,   -- 경량 stub, FK 없음 (EVAL-ITEM-001 §1.4 호환)
    evidence_id         UUID         NULL,        -- 경량 stub, FK 없음 (EVID-001 evidences.id 호환)
    level               VARCHAR(16)  NOT NULL DEFAULT 'raw',  -- raw|item|category discriminator (Decision 1 Option A)
    score_value         DECIMAL(6,2) NOT NULL,
    weight              DECIMAL(5,4) NULL,
    grade               VARCHAR(2)   NULL,        -- 산출 letter S|A|B|C|D
    status              VARCHAR(32)  NOT NULL DEFAULT 'DRAFT',  -- Decision 4 state-machine
    metadata            JSONB        NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by          VARCHAR(64)  NOT NULL DEFAULT 'cli-anonymous',
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS scores_evaluation_item_id_idx ON scores (evaluation_item_id);
CREATE INDEX IF NOT EXISTS scores_evidence_id_idx        ON scores (evidence_id);
CREATE INDEX IF NOT EXISTS scores_level_idx              ON scores (level);

DO $$ BEGIN
    ALTER TABLE scores ADD CONSTRAINT scores_level_chk
        CHECK (level IN ('raw','item','category'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE scores ADD CONSTRAINT scores_status_chk
        CHECK (status IN ('DRAFT','CONFIRMED','SUPERSEDED'));  -- Decision 4 전이표 enum
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE scores ADD CONSTRAINT scores_grade_chk
        CHECK (grade IS NULL OR grade IN ('S','A','B','C','D'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- 테이블 2/2: grade_thresholds (Decision 3 RESOLVED — metadata JSONB 아님, 최소 4컬럼 floor)
CREATE TABLE IF NOT EXISTS grade_thresholds (
    scope         VARCHAR(64)  NOT NULL,
    letter        VARCHAR(2)   NOT NULL,
    min_value     DECIMAL(6,2) NOT NULL,
    boundary_rule VARCHAR(8)   NOT NULL DEFAULT 'gte',  -- gte|gt (REQ-SCORE-003-U1, 기본 gte=하한 포함)
    PRIMARY KEY (scope, letter)
);

DO $$ BEGIN
    ALTER TABLE grade_thresholds ADD CONSTRAINT grade_thresholds_letter_chk
        CHECK (letter IN ('S','A','B','C','D'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE grade_thresholds ADD CONSTRAINT grade_thresholds_boundary_chk
        CHECK (boundary_rule IN ('gte','gt'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
```

`scores.id` UUID PK ⟹ audit `resource_id`(`uuid.UUID NOT NULL`) **Decision 2 Option 1 직접 매핑** (`Event.ResourceID = score.id`, surrogate/namespace 상수 0 — §6.2). `initial.sql`/`0002`/`0003` 미수정 (FK 하드닝 범위 밖). `grade_thresholds`는 spec.md §5 #2 경계상 **최소 floor**(풀 rubric 아님).

---

## 4. 구현 접근 (TDD Sprint, no time estimates)

> Sprint 우선순위 라벨: Priority High → Medium. Phase 순서: S0 완료 후 S1, 순차.

| Sprint | 우선순위 | 내용 | REQ |
|--------|----------|------|-----|
| S0 | High | 특성화 회귀 baseline — 기존 Workflow/Evidence/EvalItem `pg_store_test.go` GREEN 확인, `0004` 비충돌 재확인 (§6 4건 RESOLVED 완료 — 코드 동결) | (전제) |
| S1 | High | `0004_score_tables.sql` 멱등 작성 (**2테이블 확정: `scores`(Option A) + `grade_thresholds`(Decision 3)**) + `ScoreStore`/`ScoreTx` 인터페이스 (store.go) | REQ-SCORE-001 |
| S2 | High | `score.go` `PgScoreTx` (InsertScore/GetScoreByID/GetScoresByEvaluationItem/UpdateScore) + `BeginScoreTx` (pg_store.go) + 입력 검증 (U1) + stub 타입 (S1) | REQ-SCORE-001 |
| S3 | High | `audit.go` 액션 상수 **2개만(namespace 0, Decision 2)** + `recorder.go` `RecordScoreCreated`/`RecordScoreUpdated` (로컬 AuditTx, **resource_id=score.id 직접**) + 동일 TX 원자성 + rollback (U1) | REQ-SCORE-004, REQ-SCORE-UBI-002 |
| S4 | Medium | `SumWeightedByEvaluationItem` 최소 단일 레벨 `Σ(score×weight)` + 누락 가중치 단일 결정적 정책 고정 (002-U1) | REQ-SCORE-002 |
| S5 | Medium | `grade_thresholds` 테이블 조회(Decision 3) + score→letter 결정적 매핑(`gte`, S→D 스캔) + 0행/경계 결정적 처리 (003-S1/U1) + status state-machine `validateScoreStatusTransition` (Decision 4, 001-S2/UBI-004) | REQ-SCORE-003, REQ-SCORE-001-S2, REQ-SCORE-UBI-004 |
| S6 | Medium | REFACTOR (헬퍼 분리: `validateScoreInput`/`computeWeightedSum`/`resolveGrade`/`validateScoreStatusTransition` — 복잡도 ≥15 회피), @MX 태그, 커버리지 ≥85%, 경계 AC (BOUNDARY-1), evaluator-active strict ≥0.75 | 전체 |

각 Sprint: RED(실패 테스트) → GREEN(최소 구현) → REFACTOR. brownfield enhancement — RED 작성 전 기존 `eval_item.go`/`evidence.go`/`recorder.go` 패턴 정독(workflow-modes.md Brownfield Enhancement).

---

## 5. @MX 태그 계획

| 대상 | 태그 | 사유 |
|------|------|------|
| `pg_store.go` `BeginScoreTx` | `@MX:ANCHOR` | 단일 풀 TX 진입점, fan_in≥3 (Score store 전 경로 진입) — invariant 계약 |
| `recorder.go` `RecordScoreCreated`/`RecordScoreUpdated` | `@MX:ANCHOR` | 동일-TX audit 불변식 강제 지점, fan_in≥3 (생성/수정 경로 공유) |
| `score.go` `SumWeightedByEvaluationItem` (가중 롤업) | `@MX:WARN` + `@MX:REASON` | DECIMAL 가중 합산 — 누락 가중치/정밀도 왜곡 위험 영역 (REQ-SCORE-002-U1) |
| `score.go` grade 산출 (threshold 매핑/경계) | `@MX:WARN` + `@MX:REASON` | 경계값 결정성·미설정 실패 — 비결정성 시 audit 추적성 붕괴 (REQ-SCORE-003-U1) |
| `score.go` `InsertScore`/`UpdateScore` | RED `@MX:TODO` → GREEN `@MX:NOTE` | TDD 진행 표시 (EVAL-ITEM-001 M2 패턴) |

REFACTOR 시 단일 함수 복잡도 ≥15 회피 — `validateScoreInput` / `computeWeightedSum` / `resolveGrade` 헬퍼 분리 (EVAL-ITEM-001 M1 `validateStatusTransition`/`checkHierarchyMutationGuard`/`buildEvalItemUpdateSet` 선례).

---

## 6. RESOLVED 결정 (Run Phase 1 strategy.md §A + Human Gate sign-off — v0.1.2 확정)

> **[RESOLVED]** SPEC-AX-EVAL-ITEM-001 plan.md §6 OPEN→RESOLVED 흐름과 동위상으로, 본 §6 4건은 `strategy.md` §A 분석 + 사용자 Human Gate sign-off로 **확정 완료**되었다. §3 DDL은 확정(2테이블)으로 승격됨.

### 6.1 결정 1 — Score 테이블 구조: **RESOLVED — Option A (단일 `scores` + level discriminator)**

| Option | 구조 | audit 원자성 | 판정 |
|--------|------|-------------|------|
| **A 단일 scores + level discriminator** ✅ **채택** | 1 테이블 (level: raw/item/category) | 1 entity 1 Recorder 1 audit — atomic ✓ | 최단순, EVAL-ITEM Option A 정합, PoC 단일레벨 롤업이 C를 superset 포함 |
| B scores + score_aggregates | 2 테이블 | 집계 별도 시점 기록 → saga 또는 1 TX 2 entity write+2 audit → **1 TX=1 entity=1 audit 불변식 위반** | **기각** — EVAL-ITEM Option B saga 기각과 구조적 동일, 검증된 Recorder 단일-AuditTx 구조 기준 독립 재도출 (앵커링 아님) |
| C raw scores + on-the-fly 계산 | 1 테이블 + 읽기시 계산 | 1 audit ✓ | **named post-PoC 전환 경로로만 보존** — 깊은 재귀 롤업 read-time + snapshot 이력 부재; PoC(안전보건 단일 범주 단일레벨)엔 불요, Option A가 C 동작 포함 |

**구현 함의**: §3 DDL `scores`(`id UUID PK DEFAULT uuid_generate_v4()`, `level` CHECK ∈ {raw,item,category}) 확정. strategy.md §A Decision 1.

### 6.2 결정 2 — Audit `resource_id`: **RESOLVED — Option 1 (직접 `score.id` UUID, surrogate 없음)** [Decision 1에 의해 필연]

🔑 **신규 load-bearing 발견**: EVAL-ITEM-001은 `evaluation_items.id`가 VARCHAR(64) 계층코드라 `audit.Event.ResourceID(uuid.UUID NOT NULL, audit.go:94)`에 직접 못 넣어 AUD-1 deterministic UUIDv5 surrogate를 강제당했다. SCORE-001은 `scores.id`가 `UUID PK`(Decision 1=Option A)라 그 타입 불일치(비-UUID → `parseResourceID` `uuid.Nil` 강등, recorder.go:90-94)가 **구조적으로 부재**. 따라서:

- **Option 1 ✅ 채택**: `Event.ResourceID = score.id` 타입 클린 직접 대입 (`evidences`의 `RecordEvidenceCreated`와 동일 패턴). 실 비즈니스 식별자(`evaluation_item_id`/`evidence_id`/`level`/`grade`/`status`)는 `DetailsJSON`. `audit.go` 변경 = `ActionScoreCreated`/`ActionScoreUpdated` **상수 2개만** (신규 `ScoreAuditNamespace` 0건, `recorder.go` `uuid.NewSHA1` 호출 0건).
- **Option 2 (AUD-1 surrogate) — dead path**: Decision 1이 비-UUID PK를 채택할 때만 유효. Decision 1=Option A(UUID PK)이므로 적용 안 함. 기록만 보존.

Decision 1=Option A ⟹ UUID PK ⟹ Decision 2=Option 1 **필연(coupling)**. EVAL-ITEM-001보다 단순. `initial.sql audit_logs.resource_id UUID NOT NULL` 불변. strategy.md §A Decision 2.

### 6.3 결정 3 — Grade-threshold 모델 위치: **RESOLVED — 최소 `grade_thresholds` 테이블 (metadata JSONB 아님)**

| Option | 판정 |
|--------|------|
| (a) `scores.metadata` JSONB 임계값 | **기각** — REQ-SCORE-003-S1 scope 단위 "설정 존재" 판정 ill-defined + 등급 산출은 임계값을 **반드시 해석**하므로 REQ-SCORE-001-O1 metadata 불투명/미해석 계약과 자기모순 |
| (b) 최소 `grade_thresholds` 테이블 ✅ **채택** | `SELECT WHERE scope=$1`→0행 = 결정적 미설정 실패(S1), `boundary_rule` 명시 컬럼 = 경계 결정성(U1), `metadata` opacity 충돌 제거 |

`grade_thresholds`(scope, letter, min_value, boundary_rule, PK(scope,letter)) 4컬럼 = S1/U1을 결정적으로 만족하는 **최소 floor**. **이연된 풀 rubric 시스템이 아니다** — 룰 엔진/가점·감점/계층은 여전히 범위 밖(spec.md §5 #2 IN/OUT SCOPE 명확 구분, EVAL-ITEM-001 §5 #2 이연 유지). `boundary_rule` 기본 = `gte`(score ≥ min_value → letter, S→D 내림차순 스캔; 기획재정부 편람 "임계값=하한" 의미론, research.md §3 `product.md:67/70`). PoC scope = `'default'`(또는 `'안전보건'`). §3 DDL 2테이블 확정. strategy.md §A Decision 3.

### 6.4 결정 4 — 점수 불변/정정 규칙: **RESOLVED — status state-machine + append-only 하이브리드** (REQ-SCORE-UBI-004 구체화)

1. **DRAFT 행 in-place 가변** — `status='DRAFT'`인 동안 `UpdateScore`로 `score_value`/`weight`/`grade` 수정 허용 (`eval_item.go:254` `validateStatusTransition` 가드 스타일 재사용).
2. **CONFIRMED 행 불변** — `status='CONFIRMED'` 행의 score 필드 변경 `UpdateScore`는 구조화 에러 거부.
3. **CONFIRMED 정정 = 신규 행 INSERT** (`evidence.go:108` append-only 버전체이닝 선례) + 구 행 `CONFIRMED→SUPERSEDED` (비-DRAFT 행에 허용되는 유일 변형).
4. **물리 DELETE 어떤 status에서도 금지** — `DeleteScore` 메서드 미존재 (AC-SCORE-UBI-004 "물리 삭제 0건").

전이표: `DRAFT→DRAFT`(값 편집) / `DRAFT→CONFIRMED`(확정) / `CONFIRMED→SUPERSEDED`(정정 시 값 동결); `SUPERSEDED` terminal. 근거: 점수는 evidentiary record(공공 무결성) — `eval_item` in-place 부적합, `evidence.go` append-only + `scores.status` enum(이미 Decision 1 DDL 존재, 신규 컬럼 0). 순수 append-only는 DRAFT 작업 중 매 수정마다 신규 행 → audit 폭증 → DRAFT-가변+CONFIRMED-동결 분할이 최소 규칙. 모든 전이/신규행 → `RecordScore*` 감사. `score.go`에 `validateScoreStatusTransition` 헬퍼. strategy.md §A Decision 4.

### 6.5 결정 간 일관성

- D1=A (UUID PK) **⟹** D2=Option1 (직접 score.id, namespace 상수 0) — D1이 D2 강제.
- D3=`grade_thresholds` 테이블 ⟹ `0004` = `scores` + `grade_thresholds` (둘 다 멱등).
- D4=status state-machine ⟹ D1 DDL의 `status` enum 재사용 + `eval_item.validateStatusTransition` + `evidence.go` 신규행 선례 (신규 컬럼 0).
- 신규 외부 의존 0 (전 경로). phantom 0 (`BeginScoreTx`→`pg_store.go PgWorkflowStore.pool`, `postgres.go` 死 스텁 비대상 — strategy.md §0 grep 검증).

---

## 7. 리스크 레지스터

| ID | 리스크 | 영향 | 완화 |
|----|--------|------|------|
| R-SCORE-001 | §6 결정 4건 미확정 상태로 Run 진입 시 재작업 | High | S0에서 strategy 입력 준비, Run Phase 1 Human Gate 전 코드 동결 (EVAL-ITEM-001 §6 흐름 동일) |
| R-SCORE-002 | `evaluation_item_id`를 FK로 잘못 추가 (범위 밖) | High | spec.md §1.4/§5 #3 HARD, AC-SCORE-BOUNDARY-1로 FK 부재 + EVID/EVAL-ITEM 미수정 검증 |
| R-SCORE-003 | DECIMAL 가중 합산 정밀도/누락 가중치 왜곡 | Medium | `@MX:WARN`, REQ-SCORE-002-U1 결정적 정책, 알려진 집합 정확성 AC |
| R-SCORE-004 | 등급 경계값 비결정성 | Medium | `@MX:WARN`, REQ-SCORE-003-U1 결정적 경계, 동일입력→동일등급 AC |
| R-SCORE-005 | audit INSERT 실패 시 부분 커밋 | High | 동일 TX + tx.Rollback 양방향, fault injection AC (EVID/EVAL-ITEM rollback 선례) |
| R-SCORE-006 | phantom-path: `postgres.go` 死 스텁에 작성 | High | `pg_store.go` `PgWorkflowStore.pool`만 (research.md §1, EVID/EVAL-ITEM strategy.md §0 선례) |
| R-SCORE-007 | Option B(2-테이블) 집계 saga로 audit 원자성 위반 | High | §6.1 B 사실상 기각, 1 TX=1 entity=1 audit 불변식 (EVAL-ITEM Option B 기각 동일) |
| R-SCORE-008 | 풀 rubric/LLM 시뮬레이션으로 scope 폭증 | Medium | spec.md §5 #2/#5 HARD, 등급은 내부 결정적 threshold만 |

---

## 8. 완료 정의 (Plan 단계)

- [ ] §2 Delta 표 spec.md §2와 일치, [EXISTING]/[NEW]/[MODIFY] 명시
- [ ] §3 DDL 멱등 규약(research.md §4) **2테이블 확정**(`scores` + `grade_thresholds`), `scores.evaluation_item_id`=VARCHAR(64) / `scores.evidence_id`=UUID FK 없음
- [ ] §5 @MX 계획 (BeginScoreTx/RecordScore* ANCHOR, 롤업/등급 WARN+REASON)
- [ ] §6 4건 **RESOLVED** (v0.1.2 — 테이블=Option A / audit-id=Option 1 직접 UUID / grade-threshold=최소 `grade_thresholds` 테이블 / 불변=status state-machine+append-only) — strategy.md §A + Human Gate, EVAL-ITEM-001 §6 OPEN→RESOLVED 흐름 동위상
- [ ] §7 리스크 레지스터 R-SCORE-001~008
- [ ] 구현 코드/테스트 미작성 (plan 문서만)
