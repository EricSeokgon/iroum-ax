# SPEC-AX-SCORE-001 Implementation Plan

> Version: 0.1.1
> Methodology: TDD (RED-GREEN-REFACTOR), brownfield enhancement
> Harness: thorough
> Companion: `spec.md` (EARS), `acceptance.md` (Given/When/Then), `research.md` (Phase 0.5 근거)

본 plan은 SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 plan.md 구조를 미러링한다. **§6 OPEN/CONFIRMABLE 결정 항목은 SPEC-AX-EVAL-ITEM-001 §6가 Run Phase 1 이전 OPEN이었던 것과 동위상으로, 본 plan에서 해결하지 않는다 — Run Phase 1 manager-strategy 분석 + Human Gate가 확정한다.**

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

## 3. DDL (research.md §10, §4 멱등 규약 — Option A 잠정)

> **주의**: 아래 DDL은 **Option A(단일 scores + level discriminator) 잠정 작업 설계**이다. 최종 테이블 구조 / audit-id 전략 / grade-threshold 모델 위치는 §6 OPEN/strategy-confirmable이며 Run Phase 1에서 확정 후 본 DDL을 정정한다 (SPEC-AX-EVAL-ITEM-001 plan.md §6 OPEN→RESOLVED 흐름 동일).

```sql
-- 0004_score_tables.sql  (멱등: 0002/0003 규약 — research.md §4)
CREATE TABLE IF NOT EXISTS scores (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_item_id  VARCHAR(64)  NOT NULL,   -- 경량 stub, FK 없음 (EVAL-ITEM-001 §1.4 호환)
    evidence_id         UUID         NULL,        -- 경량 stub, FK 없음 (EVID-001 evidences.id 호환)
    level               VARCHAR(16)  NOT NULL DEFAULT 'raw',  -- raw|item|category discriminator (Option A 잠정 §6)
    score_value         DECIMAL(6,2) NOT NULL,
    weight              DECIMAL(5,4) NULL,
    grade               VARCHAR(2)   NULL,        -- 산출 letter S|A|B|C|D
    status              VARCHAR(32)  NOT NULL DEFAULT 'DRAFT',
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
        CHECK (status IN ('DRAFT','CONFIRMED','SUPERSEDED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE scores ADD CONSTRAINT scores_grade_chk
        CHECK (grade IS NULL OR grade IN ('S','A','B','C','D'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- grade-threshold 모델 위치 = §6 OPEN. 별도 테이블 결정 시 (잠정 형태, strategy 확정):
-- CREATE TABLE IF NOT EXISTS grade_thresholds (
--     scope        VARCHAR(64) NOT NULL,
--     letter       VARCHAR(2)  NOT NULL,
--     min_value    DECIMAL(6,2) NOT NULL,
--     boundary_rule VARCHAR(8) NOT NULL DEFAULT 'gte',  -- gte|gt (REQ-SCORE-003-U1 경계 규칙)
--     PRIMARY KEY (scope, letter)
-- );
-- metadata JSONB 임계값 방식 채택 시 이 테이블 미생성 (§6 OPEN).
```

`scores.id` UUID PK는 audit `resource_id`(`uuid.UUID NOT NULL`) Option 1 직접 매핑(잠정)을 가능케 한다 — §6.

---

## 4. 구현 접근 (TDD Sprint, no time estimates)

> Sprint 우선순위 라벨: Priority High → Medium. Phase 순서: S0 완료 후 S1, 순차.

| Sprint | 우선순위 | 내용 | REQ |
|--------|----------|------|-----|
| S0 | High | 특성화 회귀 baseline — 기존 Workflow/Evidence/EvalItem `pg_store_test.go` GREEN 확인, `0004` 비충돌 재확인, **§6 OPEN 결정 4건 manager-strategy 입력 준비** | (전제) |
| S1 | High | `0004_score_tables.sql` 멱등 작성 (Option A 잠정) + `ScoreStore`/`ScoreTx` 인터페이스 (store.go) | REQ-SCORE-001 |
| S2 | High | `score.go` `PgScoreTx` (InsertScore/GetScoreByID/GetScoresByEvaluationItem/UpdateScore) + `BeginScoreTx` (pg_store.go) + 입력 검증 (U1) + stub 타입 (S1) | REQ-SCORE-001 |
| S3 | High | `audit.go` 액션 상수 + `recorder.go` `RecordScoreCreated`/`RecordScoreUpdated` (로컬 AuditTx) + 동일 TX 원자성 + rollback (U1) | REQ-SCORE-004, REQ-SCORE-UBI-002 |
| S4 | Medium | `SumWeightedByEvaluationItem` 최소 단일 레벨 `Σ(score×weight)` + 누락 가중치 정책 (002-U1) | REQ-SCORE-002 |
| S5 | Medium | 최소 grade-threshold 모델 (§6 확정 위치) + score→letter 결정적 매핑 + 경계/미설정 처리 (003-S1/U1) | REQ-SCORE-003 |
| S6 | Medium | REFACTOR (헬퍼 분리 — 복잡도 ≥15 회피), @MX 태그, 커버리지 ≥85%, 경계 AC (BOUNDARY-1), evaluator-active strict ≥0.75 | 전체 |

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

## 6. OPEN / CONFIRMABLE 결정 (Run Phase 1 strategy + Human Gate 확정 — 본 plan 미해결)

> **[HARD] 본 §6은 해결하지 않는다.** SPEC-AX-EVAL-ITEM-001 plan.md §6가 Run Phase 1 이전 "OPEN/CONFIRMABLE"였다가 strategy.md §1/§2 분석 + Human Gate Decision Point 1/2 승인으로 RESOLVED된 것과 **동일 흐름**을 따른다. 아래 잠정 권고는 §3/§4 작업 설계의 placeholder일 뿐 확정이 아니다.

### 6.1 결정 1 — Score 테이블 구조 (research.md §5)

| Option | 구조 | audit 원자성 | Pros | Cons |
|--------|------|-------------|------|------|
| **A 단일 scores + level discriminator** (잠정 권고) | 1 테이블 (level: raw/item/category) | 1 entity 1 Recorder 1 audit — atomic ✓ | 최단순, cascade 없음, EVAL-ITEM Option A 정합 | 런타임 집계(깊은 계층 느림), 집계 audit 이력 부재 |
| B scores + score_aggregates | 2 테이블 | 집계가 별도 TX → **audit 위반 (기각)** | 집계 빠름, 집계 audit | saga 복잡, orphan — **EVAL-ITEM Option B saga 기각 동일 선례** |
| C raw scores + on-the-fly 계산 | 1 테이블 + 읽기시 계산 | 1 audit ✓ | 최단순 audit | 읽기 집계 느림, 집계 snapshot audit 부재 — 깊은 재귀는 post-PoC 이연 |

strategy 결정 의존 입력: grade threshold가 집계를 참조하는지, 집계 snapshot audit 필요 여부, PoC가 raw 점수만인지 (research.md §5/§9). **B는 1 TX=1 entity=1 audit 핵심 불변식 위반으로 사실상 기각**, C는 깊은 재귀 subtree 요구 시 named post-PoC 전환 경로. A가 잠정 작업 설계(§3 DDL).

### 6.2 결정 2 — Audit `resource_id` 매핑 (research.md §6, spec.md §3.5)

- **Option 1 (잠정 권고) — 직접 UUID**: `scores.id`가 UUID PK이므로 `Event.ResourceID = score.id` 직접 (evidences 동일, surrogate 불필요, 신규 namespace 상수 0). 결정 1이 Option A/C(UUID PK 유지) 확정 시 적용.
- **Option 2 — surrogate (fallback)**: 결정 1이 비-UUID PK를 채택할 경우에만, SPEC-AX-EVAL-ITEM-001 AUD-1 선례(`uuid.NewSHA1(ScoreAuditNamespace, []byte(key))` 결정적 UUIDv5, 실 식별자 `DetailsJSON`, `audit.go` 고정 namespace 상수)를 적용. `recorder.go:91` 기존 `google/uuid` import 재사용 — 신규 외부 의존 0.

결정 1과 함께 strategy + Human Gate 확정. `initial.sql` `audit_logs.resource_id UUID NOT NULL` 불변.

### 6.3 결정 3 — Grade-threshold 모델 위치 (spec.md §1.5/§3.4)

- Option (a) — `scores.metadata` JSONB 임계값: 별도 테이블 없음, 최단순, scope별 임계값 미지원
- Option (b) — 최소 `grade_thresholds` 테이블(§3 주석 잠정 DDL): scope별 임계값·경계 규칙(`gte`/`gt`, REQ-SCORE-003-U1) 명시적

풀 rubric 시스템은 양쪽 모두 **범위 밖**(spec.md §5 #2). strategy가 PoC 등급 산출이 scope별 임계값을 요구하는지로 (a)/(b) 확정.

### 6.4 결정 4 — 점수 불변/정정 규칙 (spec.md §3.1 REQ-SCORE-UBI-004)

확정 점수 불변의 정확 규칙: "grade 확정 후 raw 불변" / "정정 = 신규 행 INSERT(EVID-001 버전 체이닝 식) vs in-place UPDATE + status 전이" / status enum 전이표. strategy + Human Gate 확정 (research.md §9 strategy 불확실성 항목).

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
- [ ] §3 DDL 멱등 규약(research.md §4), `scores.evaluation_item_id`=VARCHAR(64) / `scores.evidence_id`=UUID FK 없음
- [ ] §5 @MX 계획 (BeginScoreTx/RecordScore* ANCHOR, 롤업/등급 WARN+REASON)
- [ ] §6 OPEN/CONFIRMABLE 4건 (테이블 구조 A/B/C, audit-id Option 1/2, grade-threshold 위치, 불변 규칙) **미해결 유지** — EVAL-ITEM-001 §6 pre-Run 동위상
- [ ] §7 리스크 레지스터 R-SCORE-001~008
- [ ] 구현 코드/테스트 미작성 (plan 문서만)
