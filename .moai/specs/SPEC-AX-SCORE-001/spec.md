---
id: SPEC-AX-SCORE-001
version: 0.1.1
status: draft
created: 2026-05-19
updated: 2026-05-19
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.1 (2026-05-19): plan-auditor iter1 FAIL 0.88 반영 (must-pass 전부 PASS, category Clarity 0.95/Completeness 1.0/Testability 0.92/Traceability 1.0 — FAIL 사유는 AC-count 원장 내부 모순 2건뿐). **요구사항 본질·EARS·traceability·frontmatter 8-field·타입 계약(`scores.evaluation_item_id`=VARCHAR(64)/`scores.evidence_id`=UUID FK 없음)·§6 OPEN 이연·Exclusion set은 모두 정확하여 무변경 — count 메타데이터만 정정.** D1(major) 정정: `acceptance.md` Total AC count + `spec-compact.md` AC 헤더의 잘못된 "14"를 물리적 AC heading 직접 카운트 결과 **16**으로 정정 (component breakdown `4+5+2+2+2+1=16`은 이미 정확, 명시 total만 오류 → 삼중 내부 모순 해소: `acceptance.md` Total = §9 DoD §1-§4 enumeration = spec-compact.md count = 16 single source of truth). D2(minor) 정정: `acceptance.md` §9 DoD의 "16개 edge case"를 §7 Edge Case Catalog 표 물리적 데이터 행 수(L348-362 = **15개**)와 일치하도록 "15개"로 정정 — §7 표 15행이 의도된 정답(BOUNDARY-1 + 4 UBI AC가 15개 안에 이미 표현, 표 보강 불요), DoD를 표에 맞춤. spec.md §9 DoD에는 하드코딩된 AC/edge count 원장 부재로 무변경(요구사항 텍스트·타입 계약·OPEN 이연 일체 불변). 4개 문서(spec/plan/acceptance/spec-compact) count cross-file 일관성 검증 완료 (AC=16, edge=15). SPEC-AX-EVAL-ITEM-001 v0.1.1 plan-auditor 정정 패턴과 동일. (작성자: ircp)
- 0.1.0 (2026-05-19): 경영평가 점수 산출/집계(Evaluation Scoring & Aggregation) 첫 초안. iroum-ax Go control-plane을 brownfield 확장하여 **점수 기록 데이터 모델 + store 계층 + audit 연계 Walking Skeleton**을 정의하고, EVAL-ITEM weight를 적용한 롤업(평가지표→평가항목→평가범주)과 등급(S/A/B/C/D) 산출은 **모델링 + 최소 구현**으로만 제공한다. 단일 `scores` 테이블 + level discriminator(Option A 잠정), `scores.evaluation_item_id`는 **FK 제약 없는 `VARCHAR(64)` stub** (SPEC-AX-EVAL-ITEM-001 `evaluation_items.id VARCHAR(64)` 계층코드와 타입 호환 — 미래 FK 승격 대비), `scores.evidence_id`는 **FK 제약 없는 `UUID` stub** (SPEC-AX-EVID-001 `evidences.id UUID`와 타입 호환). SPEC-AX-CTRL-001의 `WorkflowStore`/`WorkflowTx`/`Recorder`/`AuditTx` 패턴(GREEN 가정), SPEC-AX-EVID-001(완료, v0.1.2)·SPEC-AX-EVAL-ITEM-001(완료, v0.1.3)의 store/audit 미러링 선례 위에 `ScoreStore`/`ScoreTx`/`RecordScore*`/`BeginScoreTx`를 동일 패턴으로 추가한다. 풀 등급기준(scoring rubric) 시스템, LLM 등급 시뮬레이션 / Recommendation 엔진(후속 Python/AI 파이프라인), `evidences`/`evaluation_items` FK 하드닝(EVID-001/EVAL-ITEM-001 코드·마이그레이션 변경 포함), 풀 집계 엔진 / REST API / Console UI는 의도적 제외(후속 SPEC). (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등은 canonical schema에 존재하지 않으므로 사용하지 않는다. 본 SPEC의 모든 EARS 요구사항·DDL·영향파일은 `.moai/specs/SPEC-AX-SCORE-001/research.md`(Phase 0.5 deep research, file:line 근거)에 근거한다. 두 개의 **load-bearing 타입 계약**: (1) `scores.evaluation_item_id` = `VARCHAR(64)` — SPEC-AX-EVAL-ITEM-001 §1.4 [HARD] `evaluation_items.id VARCHAR(64)` 계층코드 타입 결정과 호환(research.md §2, SPEC-AX-EVAL-ITEM-001/spec.md:55, AC-EVALITEM-001-3 검증 완료); (2) `scores.evidence_id` = `UUID` — SPEC-AX-EVID-001 `evidences.id`(`0002_evidence_tables.sql`, UUID PK)와 호환(research.md §2). 두 컬럼 모두 **FK 제약 없는 stub**이며 SPEC-AX-EVID-001 §1.4 / SPEC-AX-EVAL-ITEM-001 §1.4의 stub-consumer 선례를 동일하게 적용한다. 신규 마이그레이션 파일번호 `0004_score_tables.sql`은 `migrations/`에 실재하는 `0001_initial.sql` + `0002_evidence_tables.sql` + `0003_eval_item_tables.sql`과 비충돌(research.md §4, 디스크 확인)임을 검증했다.

---

# SPEC-AX-SCORE-001 — 경영평가 점수 산출/집계 (Evaluation Scoring & Aggregation)

## 1. 개요

경영평가팀이 평가지표별 raw 점수를 기록하고, SPEC-AX-EVAL-ITEM-001의 평가항목 weight를 적용해 평가지표→평가항목→평가범주로 가중 롤업하며, 최종 등급(S/A/B/C/D)을 산출할 수 있도록, `apps/control-plane/`(Go 1.22+)에 점수 데이터 모델·영속 계층·감사 연계를 추가하는 Walking Skeleton을 정의한다. 본 SPEC은 SPEC-AX-CTRL-001의 워크플로우 오케스트레이션 계층과 SPEC-AX-EVID-001(증빙)·SPEC-AX-EVAL-ITEM-001(평가항목 taxonomy) store/audit 확장 선례 위에, **점수 행을 단일 트랜잭션 내에서 생성/수정하고 모든 변경을 `audit_logs`에 원자적으로 기록하며 가중 롤업·등급 산출을 최소 모델로 제공하는** 최소 실행 가능한 점수 산출/집계 계층을 제공한다.

### 1.1 Walking Skeleton의 의미 (본 SPEC 범위)

본 SPEC의 1차 산출물은 **기초 점수 데이터 모델 + store 계층 + audit 연계 Walking Skeleton**이다. 가중 롤업 엔진과 등급 산출은 **모델링 + 최소 구현(minimally implemented)**으로만 제공한다 — 풀 집계 엔진·풀 등급기준 시스템·REST API·Console UI·LLM 등급 시뮬레이션 / Recommendation 엔진은 본 SPEC 범위가 아니다(§5 Exclusions, §7 Out of Scope).

- 단일 엔티티: `scores` 테이블 (level discriminator로 raw 평가지표 점수 / item 집계 / category 집계 구분 — Option A 잠정, plan.md §6 OPEN/strategy-confirmable)
- 단일 store 추상화: `ScoreStore` / `ScoreTx` 인터페이스 (SPEC-AX-CTRL-001 `WorkflowStore`/`WorkflowTx`, SPEC-AX-EVID-001 `EvidenceStore`/`EvidenceTx`, SPEC-AX-EVAL-ITEM-001 `EvalItemStore`/`EvalItemTx` 패턴 그대로 — 동일 pgx pool 재사용, 신규 풀 생성 없음)
- 단일 감사 연계: 기존 `internal/audit` Recorder에 `RecordScoreCreated` / `RecordScoreUpdated` 확장 (기존 `RecordCreated`/`RecordEvidenceCreated`/`RecordEvalItemCreated`와 동일 시그니처 패턴, 동일 트랜잭션 내 atomic 기록)
- 단일 생성/수정 경로: 점수 생성·수정 store 메서드 (BeginScoreTx → 입력 검증 → InsertScore/UpdateScore → Recorder 기록 → Commit)
- 최소 가중 롤업: `evaluation_item_id`별 `Σ(score_value × weight)` 단일 레벨 가중 합산 함수 (깊은 재귀 집계·집계 snapshot 영속·incremental 집계는 범위 밖)
- 최소 등급 산출: score → letter(S/A/B/C/D) 내부 threshold 모델 (metadata JSONB 또는 최소 `grade_thresholds` — plan.md §6 OPEN/strategy-confirmable). 풀 rubric·LLM 시뮬레이션은 범위 밖
- 인증 없음: 모든 호출은 `created_by="cli-anonymous"` (SPEC-AX-001 REQ-UBI-003, SPEC-AX-CTRL-001 REQ-CTRL-UBI-002, SPEC-AX-EVID-001 REQ-EVID-UBI-003, SPEC-AX-EVAL-ITEM-001 REQ-EVALITEM-UBI-003과 정합)

### 1.2 Anchor 컨텍스트

본 SPEC은 `product.md` §3.2의 기획재정부 경영평가 편람("안전 보건" 평가기준·배점·등급)을 시스템 내부에서 **실측 점수로 채점·집계**하는 기반을 형성한다. `product.md:160`의 4계층(평가범주 → 평가항목 → 평가지표 → 배점/가중치), `product.md:67` 평가편람·`:70` A/B/C/D 등급 벤치마크를 점수 흐름(지표별 raw score → EVAL-ITEM weight 가중 롤업 Σ(score×weight) → 항목 → 범주 → 등급 threshold)으로 모델링한다(research.md §3). PoC 범위는 "안전보건" 범주의 점수 기록 + 최소 롤업/등급이며, SPEC-AX-CTRL-001의 REQ-CTRL/REQ-CTRL-UBI, SPEC-AX-EVID-001·SPEC-AX-EVAL-ITEM-001의 store/audit 확장은 GREEN(완료) 상태로 가정한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `SCORE` (Evaluation Scoring & Aggregation sub-domain)
- 따라서 SPEC ID: `SPEC-AX-SCORE-001` (2 domains — `AX` + `SCORE`, `.claude/skills/moai/workflows/plan.md` Composite domain rules "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내)

### 1.4 의존성 stub 계약 — evaluation_item_id / evidence_id 타입 [핵심, load-bearing]

점수는 평가항목(evaluation item)에 귀속되며 증빙(evidence)으로 뒷받침된다. 그러나 평가항목 taxonomy(SPEC-AX-EVAL-ITEM-001, 완료)와 증빙 관리(SPEC-AX-EVID-001, 완료)는 **본 SPEC의 범위가 아니다**. 본 SPEC은 그 두 도메인의 **stub-consumer**이며, SPEC-AX-EVID-001 §1.4 / SPEC-AX-EVAL-ITEM-001 §1.4가 확립한 FK-제약-없는 stub 선례를 동일하게 적용한다. 따라서 다음이 **HARD 계약**이다:

- **[HARD]** `scores.evaluation_item_id`는 **`VARCHAR(64)`** 이며 **FK 제약을 갖지 않는다**. SPEC-AX-EVAL-ITEM-001 §1.4 [HARD] `evaluation_items.id VARCHAR(64)`(계층 코드, 예: `AX-SAFETY-ORG-01-1`, UUID/auto-increment 아님)와 **타입 호환**(VARCHAR(64) ↔ VARCHAR(64))되도록 보장하기 위한 결정이며, research.md §2의 핵심 계약이다. 미래 FK `scores.evaluation_item_id → evaluation_items(id)` 승격 시 타입 불일치가 0건이 되도록 한다.
- **[HARD]** `scores.evidence_id`는 **`UUID`** (nullable — 모든 점수가 증빙에 직접 묶이지는 않음)이며 **FK 제약을 갖지 않는다**. SPEC-AX-EVID-001 `evidences.id`(`0002_evidence_tables.sql`, `UUID` PK)와 **타입 호환**되도록 보장하기 위한 결정이다(research.md §2).
- **[HARD]** SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 코드 변경, 그리고 `scores.evaluation_item_id` / `scores.evidence_id`에 FK 제약을 추가하거나 `evidences` / `evaluation_items`에 역 제약을 소급(retroactive) 추가하는 작업은 **본 SPEC의 범위가 아니다**. 본 SPEC은 type-compatible한 `scores` 테이블을 제공하기만 하며, 실제 FK 하드닝은 미래 별도 SPEC이 수행한다(§5 Exclusions #3, §7 Out of Scope). 그때까지 두 컬럼은 FK 없는 stub으로 유지된다(AC-SCORE-BOUNDARY-1로 경계 확인).

### 1.5 등급기준(grade rubric) 경계 [핵심]

SPEC-AX-EVAL-ITEM-001 §5 Exclusion #2는 등급기준(scoring rubric) 저장 설계를 명시적으로 이연했다. 본 SPEC은 **풀 rubric 시스템을 설계하지 않는다**. 본 SPEC이 정의하는 것은 점수를 letter 등급(S/A/B/C/D)으로 매핑하는 **최소 내부 grade-threshold 모델**(예: score → letter를 metadata JSONB 임계값 또는 최소 `grade_thresholds` 테이블로 — plan.md §6 OPEN/strategy-confirmable)뿐이다. 풀 등급기준 규칙 시스템, `product.md:171-183`의 **LLM "등급 시뮬레이션"** 및 `product.md:177`의 **"Recommendation 엔진"**(후속 Python/AI 파이프라인)은 본 SPEC 범위 밖이다(§5 Exclusions #2/#5, §7 Out of Scope, research.md §3).

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 `apps/control-plane/` 트리를 따른다. 본 SPEC은 stub이 아닌 **실제 구현이 존재하는 코드를 brownfield 확장**하므로 Delta 마커를 적용한다 ([EXISTING]=특성화 테스트로 보존, [NEW]=신규 추가, [MODIFY]=기존 파일 수정).

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/internal/store/store.go` | `ScoreStore` / `ScoreTx` 인터페이스 추가 (기존 `WorkflowStore`/`WorkflowTx`, `EvidenceStore`/`EvidenceTx`, `EvalItemStore`/`EvalItemTx` 패턴 준수) | [MODIFY] | REQ-SCORE-001 |
| `apps/control-plane/internal/store/score.go` | `ScoreTx` 메서드 (InsertScore, GetScoreByID, GetScoresByEvaluationItem, UpdateScore, SumWeightedByEvaluationItem, InsertAuditLog, Commit, Rollback) pgx 구현 (`eval_item.go` 미러링) | [NEW] | REQ-SCORE-001, REQ-SCORE-002, REQ-SCORE-003 |
| `apps/control-plane/internal/store/pg_store.go` | 실 pgx pool(`PgWorkflowStore{pool *pgxpool.Pool}`, `server.go` `store.NewPgWorkflowStore(...)` 와이어링, `BeginEvidenceTx`/`BeginEvalItemTx` 선례)에 `BeginScoreTx` 진입점 추가 (신규 풀 금지, `PgWorkflowStore.pool` 단일 재사용). **주의: `postgres.go`는 Sprint-0 死 스텁(`New(cfg)` + TODO, 실 pool 없음)이며 본 SPEC 대상 아님** — research.md §1 phantom-path 회피 | [MODIFY] | REQ-SCORE-001 |
| `apps/control-plane/internal/audit/audit.go` | 신규 액션 상수 `ActionScoreCreated`, `ActionScoreUpdated` 추가 (기존 `Action string`, `ActionEvidenceCreated`/`ActionEvalItemCreated` 패턴). audit-id 전략에 따라 namespace 상수 추가 여부는 plan.md §6 OPEN | [MODIFY] | REQ-SCORE-004 |
| `apps/control-plane/internal/audit/recorder.go` | `RecordScoreCreated`, `RecordScoreUpdated` 메서드 추가 (기존 `RecordCreated`/`RecordEvidenceCreated`/`RecordEvalItemCreated` 시그니처 패턴, 로컬 `AuditTx` 인터페이스 유지 — store→audit 순환 의존 회피) | [MODIFY] | REQ-SCORE-004 |

### 2.2 Database (`.moai/db/schema/`)

| 경로 | 책임 | Delta |
|------|------|-------|
| `.moai/db/schema/initial.sql` | **참조 only**: 본 SPEC은 initial.sql을 수정하지 않는다 (기존 documents/audit_logs 테이블 schema drift 방지, SPEC-AX-EVID-001 §2.2 / SPEC-AX-EVAL-ITEM-001 §2.2 동일 정책). | [EXISTING] |
| `.moai/db/schema/migrations/0002_evidence_tables.sql` | **참조 only**: 본 SPEC은 SPEC-AX-EVID-001의 마이그레이션을 수정하지 않는다 (FK 하드닝 범위 밖, §5 Exclusion #3). | [EXISTING] |
| `.moai/db/schema/migrations/0003_eval_item_tables.sql` | **참조 only**: 본 SPEC은 SPEC-AX-EVAL-ITEM-001의 마이그레이션을 수정하지 않는다 (FK 하드닝 범위 밖, §5 Exclusion #3). | [EXISTING] |
| `.moai/db/schema/migrations/0004_score_tables.sql` | **신규**: `scores` 테이블 + 인덱스 + level/status CHECK + (grade-threshold 모델이 별도 테이블일 경우) `grade_thresholds` 추가. 멱등성 패턴(`CREATE TABLE IF NOT EXISTS`, `DO $$ ... EXCEPTION ...`, `CREATE INDEX IF NOT EXISTS`) 유지, 수동 SQL 규약 (마이그레이션 도구 미사용). 파일번호 `0004`는 디스크상 `0001_initial.sql` + `0002_evidence_tables.sql` + `0003_eval_item_tables.sql`과 비충돌 확인(research.md §4). | [NEW] |

### 2.3 Tests (`apps/control-plane/`)

| 경로 | 책임 | Delta |
|------|------|-------|
| `apps/control-plane/internal/store/score_test.go` | testcontainers-go(postgres) 기반 InsertScore + evaluation_item_id VARCHAR(64) / evidence_id UUID 타입 검증 + 가중 롤업 최소 정확성 + grade-threshold 경계 | [NEW] |
| `apps/control-plane/internal/audit/recorder_score_test.go` | RecordScoreCreated/Updated audit row 검증 + audit fail 시 양방향 rollback | [NEW] |
| `apps/control-plane/internal/store/pg_store_test.go` | 기존 WorkflowStore/EvidenceStore/EvalItemStore 특성화 테스트 보존 확인 (score 와이어링 후 회귀 없음) | [EXISTING] |

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건)

Ubiquitous 요구사항은 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001의 canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track 패턴을 도메인 스코프 형태(`REQ-SCORE-UBI-NNN`)로 적용한다 (research.md §8).

- **REQ-SCORE-UBI-001 (데이터 주권)**: The evaluation scoring subsystem SHALL NOT make any external service call (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) for creating, reading, updating, validating, rolling up, or grading score rows. 모든 영속·계산 경로는 고객사 내부망 자원(SPEC-AX-CTRL-001이 확립한 단일 PostgreSQL pgx pool)에만 의존한다 (`tech.md` §9.1 망분리 정합). LLM 등급 시뮬레이션은 본 SPEC 범위 밖이며(§5 #5), 본 SPEC의 등급 산출은 내부 결정적 threshold만 사용한다.
- **REQ-SCORE-UBI-002 (감사 가능성)**: The subsystem SHALL write exactly one `audit_logs` entry within the same database transaction as every score create and every score update event, reusing the `audit_logs` schema defined in SPEC-AX-001 REQ-UBI-003 (SPEC-AX-EVID-001 REQ-EVID-UBI-002 / SPEC-AX-EVAL-ITEM-001 REQ-EVALITEM-UBI-002와 동일 규약). 트랜잭션 외부에서 발생한 점수 변경은 audit 불가능하므로 금지된다.
- **REQ-SCORE-UBI-003 (cli-anonymous 기본값)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the subsystem SHALL persist `scores.created_by = 'cli-anonymous'` and the corresponding `audit_logs.user_id = 'cli-anonymous'` (정확히 literal 문자열, NULL 금지), reusing `audit.DefaultUserID` / `Recorder.resolveUserID` 계약 (실 사용자 식별자 누출 금지).
- **REQ-SCORE-UBI-004 (점수 불변 — strategy 확정)**: The subsystem SHALL NOT physically delete any `scores` row, and SHALL treat a confirmed/finalized score as immutable. 점수 정정은 새 값 기록 또는 상태 전이로 표현되며 물리 삭제는 금지된다. 불변 경계의 정확한 규칙(예: "grade 확정 후 raw 점수 불변", "정정은 신규 행 vs in-place update 중 무엇인가")은 Run Phase 1 strategy + Human Gate에서 확정한다 (plan.md §6 OPEN/strategy-confirmable, research.md §8/§9).

### 3.2 REQ-SCORE-001 — 점수 데이터 모델 & Store 계층

**엔티티**: `scores` (id UUID PK — `DEFAULT uuid_generate_v4()`, evaluation_item_id VARCHAR(64) — **FK 없는 stub, EVAL-ITEM-001 호환**, evidence_id UUID nullable — **FK 없는 stub, EVID-001 호환**, level VARCHAR(16) — `raw`/`item`/`category` discriminator(Option A 잠정, plan.md §6 OPEN), score_value DECIMAL(6,2), weight DECIMAL(5,4) nullable, grade VARCHAR(2) nullable — 산출 letter S/A/B/C/D, status VARCHAR(32) DEFAULT 'DRAFT' — CHECK enum, metadata JSONB, created_at, created_by DEFAULT 'cli-anonymous', updated_at). `evaluation_item_id`의 VARCHAR(64) / `evidence_id`의 UUID 타입은 §1.4 HARD 계약이다. 구체 DDL은 `plan.md` §3 참조 (테이블 구조 최종 형태는 plan.md §6 OPEN/strategy-confirmable).

**Store 추상화**: `ScoreStore.BeginScoreTx(ctx) (ScoreTx, error)` + `ScoreTx{InsertScore, GetScoreByID, GetScoresByEvaluationItem, UpdateScore, SumWeightedByEvaluationItem, InsertAuditLog, Commit, Rollback}` — 기존 `WorkflowStore`/`WorkflowTx`, `EvidenceStore`/`EvidenceTx`, `EvalItemStore`/`EvalItemTx` 인터페이스 설계를 그대로 미러링하며 동일 pgx pool을 재사용한다 (`pg_store.go` `BeginScoreTx`, 신규 풀 금지).

#### Event-driven

- **REQ-SCORE-001-E1**: WHEN a caller submits a new score with a non-blank `evaluation_item_id` (VARCHAR(64)), a numeric `score_value`, an optional `evidence_id` (UUID), and an optional `weight`, THEN the subsystem SHALL open a single `ScoreTx`, INSERT one `scores` row, INSERT one corresponding `audit_logs` row with action `SCORE_CREATED` in the same transaction, Commit atomically, and return the created score `id` (UUID).

#### State-driven

- **REQ-SCORE-001-S1**: WHILE a score create or update transaction is in progress, the subsystem SHALL persist `scores.evaluation_item_id` as an opaque `VARCHAR(64)` value and `scores.evidence_id` as an opaque `UUID` value WITHOUT enforcing any database foreign-key constraint to `evaluation_items` or `evidences` (FK-없는 stub — §1.4 HARD, SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 stub-consumer 선례). 참조 무결성 강제는 미래 FK 하드닝 SPEC 책임이다.

#### Optional

- **REQ-SCORE-001-O1**: WHERE a caller supplies a `metadata` JSONB payload (예: 채점 코멘트, 등급 임계값 초안, raw 산식 부가 정보), the subsystem SHALL persist it verbatim as an opaque JSONB column without interpreting its structure. metadata의 스키마·검증·등급기준 규칙 해석은 본 SPEC 범위가 아니며 향후 별도 결정으로 이연한다 (§5 Exclusions #2). (SPEC-AX-EVAL-ITEM-001 REQ-EVALITEM-001-O1 패턴 동일.)

#### Unwanted

- **REQ-SCORE-001-U1**: IF the incoming `evaluation_item_id` is blank/empty OR exceeds 64 characters OR `score_value` is non-numeric/absent OR `evidence_id` is supplied but is not a valid UUID, THEN the subsystem SHALL reject the request with a structured error, SHALL NOT INSERT any `scores` or `audit_logs` row, and SHALL surface the validation error to the caller (client error, not a server defect — INFO 로그).

### 3.3 REQ-SCORE-002 — 가중 롤업 (Weighted Aggregation — Minimal)

본 SPEC은 가중 롤업을 **모델링 + 최소 구현**으로만 제공한다. 단일 레벨 가중 합산(평가지표 raw 점수 → 평가항목 가중 점수)을 검증 가능한 최소 정확성으로 제공하며, 깊은 재귀 집계·집계 snapshot 영속·incremental 집계·풀 집계 엔진은 본 SPEC 범위 밖이다(§5 Exclusions #4).

#### Event-driven

- **REQ-SCORE-002-E1**: WHEN a caller invokes `SumWeightedByEvaluationItem(evaluationItemID)` for an evaluation item that has one or more `raw`-level child score rows, THEN the subsystem SHALL compute the weighted sum `Σ(score_value × weight)` over those rows within a single read transaction and return the aggregate value (단일 레벨 가중 합산 — 최소 정확성).

#### Unwanted

- **REQ-SCORE-002-U1**: IF a `raw`-level score row participating in a weighted rollup has a NULL `weight`, THEN the subsystem SHALL NOT silently coerce the missing weight to an arbitrary default that would distort the aggregate; it SHALL either exclude the row deterministically or surface a structured error per the rollup contract (가중치 누락이 집계 결과를 조용히 왜곡하지 않음). 정확한 누락-가중치 정책(제외 vs 에러)은 Run Phase 1 strategy에서 확정한다 (plan.md §6 OPEN/strategy-confirmable).

### 3.4 REQ-SCORE-003 — 등급 산출 (Grade Computation — Minimal Threshold Model)

본 SPEC은 점수→등급(S/A/B/C/D) 매핑을 **최소 내부 threshold 모델**로만 제공한다(§1.5). 풀 등급기준 규칙 시스템, LLM 등급 시뮬레이션 / Recommendation 엔진은 본 SPEC 범위 밖이다(§5 Exclusions #2/#5).

#### Event-driven

- **REQ-SCORE-003-E1**: WHEN a caller requests the letter grade for an aggregate score value, THEN the subsystem SHALL map the value to exactly one letter in the ordered set `{S, A, B, C, D}` using a deterministic internal threshold model (metadata JSONB 임계값 또는 최소 `grade_thresholds` — plan.md §6 OPEN/strategy-confirmable), entirely within internal resources (REQ-SCORE-UBI-001 정합 — 외부 LLM 호출 0건).

#### State-driven

- **REQ-SCORE-003-S1**: WHILE no grade-threshold configuration is available for the requested scope, the subsystem SHALL surface a structured "grade thresholds unavailable" error and SHALL NOT fabricate a letter grade (등급 임의 생성 금지 — 미설정 시 결정적 실패). 임계값 모델의 저장 위치(metadata JSONB vs 별도 테이블)는 plan.md §6 OPEN.

#### Unwanted

- **REQ-SCORE-003-U1**: IF an aggregate value falls exactly on a configured grade boundary, THEN the subsystem SHALL resolve it deterministically and consistently (동일 입력 → 동일 등급, 경계값 비결정성 0건). 경계 포함/배제 규칙(`>=` vs `>`)은 Run Phase 1 strategy에서 확정한다 (plan.md §6 OPEN/strategy-confirmable).

### 3.5 REQ-SCORE-004 — 감사 연계 (Audit Recorder 확장)

기존 `internal/audit` Recorder 패턴을 확장한다. 새 액션 상수 `ActionScoreCreated Action = "SCORE_CREATED"`, `ActionScoreUpdated Action = "SCORE_UPDATED"`를 추가하고, `Recorder`에 `RecordScoreCreated`/`RecordScoreUpdated` 메서드를 추가한다 (기존 `RecordCreated(ctx, tx AuditTx, ...)` / `RecordEvidenceCreated` / `RecordEvalItemCreated` 시그니처 패턴, 로컬 `AuditTx` 인터페이스 유지 — store→audit 순환 의존 회피).

> **Audit `resource_id` 전략 — OPEN/strategy-confirmable** (plan.md §6, research.md §6). `audit.Event.ResourceID`는 `uuid.UUID`(`audit.go:72`), `audit_logs.resource_id`는 `UUID NOT NULL`(`initial.sql:119`). **잠정 작업 설계 = Option 1 (직접 UUID)**: `scores.id`가 UUID PK이므로 `Event.ResourceID = score.id`를 직접 사용한다(evidences 동일, surrogate 불필요, 신규 namespace 상수 없음). Option 2(EVAL-ITEM-001 AUD-1 식 deterministic UUIDv5 surrogate)는 테이블 구조 결정(plan.md §6) 결과 `scores`가 비-UUID PK를 채택할 경우에만 적용한다. **본 §3.5는 EVAL-ITEM-001 §3.4가 v0.1.2까지 OPEN이었던 것과 동위상으로 OPEN 상태로 둔다 — Run Phase 1 strategy + Human Gate가 테이블 구조와 함께 확정한다.**

#### Event-driven

- **REQ-SCORE-004-E1**: WHEN a score create or update transaction calls `Recorder.RecordScoreCreated` or `Recorder.RecordScoreUpdated` with the active `AuditTx`, THEN the Recorder SHALL construct an `audit.Event` with `Action` ∈ {`SCORE_CREATED`, `SCORE_UPDATED`}, `ResourceType="score"`, `ResourceID` = the score's audit identifier (Option 1 직접 `scores.id` UUID 잠정 — 최종 전략 plan.md §6), `UserID`=`resolveUserID(userID)`, and `DetailsJSON` containing `{score_id, evaluation_item_id, evidence_id?, level, grade?}` (실제 식별자·맥락은 `DetailsJSON`에 보존), and SHALL insert it via the same `AuditTx` (동일 트랜잭션).

#### Unwanted

- **REQ-SCORE-004-U1**: IF the `audit_logs` INSERT fails for any reason (constraint violation, connection reset) during a score create or update transaction, THEN the subsystem SHALL execute `tx.Rollback(ctx)`, leave the `scores` table with NO trace of this operation (the score INSERT/UPDATE also rolled back), return the wrapped audit-insertion error to the caller, and SHALL NOT leak goroutines beyond the request scope. 점수 행과 감사 행은 함께 커밋되거나 함께 롤백된다 (all-or-nothing, SPEC-AX-EVID-001 REQ-EVID-003-U1 / SPEC-AX-EVAL-ITEM-001 REQ-EVALITEM-003-U1 패턴).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 데이터 주권 (망분리) | 점수 생성·조회·수정·롤업·등급 산출 경로의 외부 API 호출 0건. 단일 내부망 PostgreSQL pgx pool만 사용. LLM 등급 시뮬레이션 범위 밖 | §3.1 REQ-SCORE-UBI-001, `tech.md` §9.1, §5 #5 |
| 감사 가능성 | 모든 score create/update → 동일 TX 내 `audit_logs` 1건. 누락 0건 | §3.1 REQ-SCORE-UBI-002 |
| 점수 불변 / 무삭제 | `scores` 물리 삭제 0건, 확정 점수 불변 (정확 규칙 strategy 확정) | §3.1 REQ-SCORE-UBI-004 |
| EVAL-ITEM-001 FK 타입 호환 | `scores.evaluation_item_id` = `VARCHAR(64)` (FK 없는 stub), `evaluation_items.id VARCHAR(64)`와 타입 호환 | §1.4, research.md §2 |
| EVID-001 FK 타입 호환 | `scores.evidence_id` = `UUID` nullable (FK 없는 stub), `evidences.id UUID`와 타입 호환 | §1.4, research.md §2 |
| 가중 롤업 최소 정확성 | `SumWeightedByEvaluationItem`은 단일 레벨 `Σ(score×weight)` 정확. 깊은 재귀·snapshot 범위 밖 | §3.3, §5 #4 |
| 등급 결정성 | 동일 입력 → 동일 등급, 경계값 비결정성 0건, 외부 호출 0건 | §3.4 REQ-SCORE-003-U1 |
| 성능 — 점수 생성 | p99 < 50ms (단일 TX INSERT + 검증, 단일 행, 한국 공공 시간 제약 research.md §8) | §3.2 REQ-SCORE-001-E1 |
| 성능 — 가중 롤업 | `SumWeightedByEvaluationItem`은 `scores_evaluation_item_id_idx` 인덱스 사용, p99 < 50ms (단일 레벨) | §3.3 REQ-SCORE-002-E1 |
| cli-anonymous 기본값 | AuthN disabled 시 created_by/user_id='cli-anonymous' literal (NULL 금지) | §3.1 REQ-SCORE-UBI-003 |
| pgx pool 재사용 | 기존 SPEC-AX-CTRL-001 단일 pgx pool(`PgWorkflowStore.pool`, `pg_store.go`) 재사용, 신규 풀 생성 금지. `postgres.go` 死 스텁 비대상 | research.md §1, §9 |
| 로깅 | 구조화 JSON 로그(zap), 검증 거부는 INFO, 서버 결함은 ERROR | `tech.md` §8.2 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR), harness: thorough | `quality.yaml` development_mode |
| Go 도구 | go vet, golangci-lint (default + gosec), goimports | `.claude/rules/moai/languages/go.md` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **풀 집계 엔진 / REST API / Console UI** — HTTP 엔드포인트(점수 생성/조회/집계/등급), 점수 대시보드, 평가범주 롤업 뷰어, `apps/console/` 화면 일체 제외. 본 SPEC은 store 계층 메서드 + audit 연계 + 최소 단일 레벨 가중 합산만 다룬다.
2. **풀 등급기준(scoring rubric) 시스템** — S/A/B/C/D 등급별 판정 기준·점수 산식·가점/감점 규칙의 구조화 저장·룰 엔진은 본 SPEC에서 **설계하지 않는다**. SPEC-AX-EVAL-ITEM-001 §5 Exclusion #2가 이미 이연한 영역이며, 본 SPEC은 점수→letter 최소 내부 threshold 모델만 정의한다. `metadata` JSONB / 최소 `grade_thresholds`는 placeholder이며 풀 rubric 스키마·검증·해석은 미래 별도 SPEC이 수행한다 (open follow-up — plan.md §6).
3. **`evidences` / `evaluation_items` FK 하드닝 (EVID-001 / EVAL-ITEM-001 코드·마이그레이션 변경 포함)** — `scores.evaluation_item_id → evaluation_items(id)` 및 `scores.evidence_id → evidences(id)` FK 제약을 추가하는 작업, 그에 수반되는 SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 코드·마이그레이션 변경은 **본 SPEC 범위 밖**이다. 본 SPEC은 타입 호환(VARCHAR(64) / UUID)되는 `scores` 테이블을 제공하기만 하며, `0002_evidence_tables.sql` / `0003_eval_item_tables.sql` / `evidences` / `evaluation_items` 코드는 본 SPEC 구현 중 수정하지 않는다. FK 하드닝은 미래 별도 SPEC이 수행하며, 그때까지 두 컬럼은 FK 없는 stub으로 유지된다(AC-SCORE-BOUNDARY-1로 경계 확인).
4. **풀 집계: 깊은 재귀 롤업 / 집계 snapshot 영속 / incremental 집계** — 평가지표→평가항목→평가범주 전체 트리 재귀 집계, 집계 결과 별도 테이블 영속(`score_aggregates`), 증분 재계산 파이프라인은 제외. 본 SPEC은 단일 레벨 `Σ(score×weight)` 최소 함수만 제공한다. (research.md §5 — Option B 2-테이블은 집계 별도 TX → 감사 원자성 위반으로 기각, EVAL-ITEM Option B saga 기각 동일 선례; Option C on-the-fly 깊은 계산은 post-PoC 이연.)
5. **LLM 등급 시뮬레이션 / Recommendation 엔진** — `product.md:171-183`의 LLM 기반 "등급 시뮬레이션", `product.md:177`의 "Recommendation 엔진"은 **후속 Python/AI 파이프라인 책임**이며 본 SPEC 범위 밖이다. 본 SPEC의 등급 산출은 내부 결정적 threshold만 사용하며 외부 AI 호출 0건이다(REQ-SCORE-UBI-001 정합, research.md §3).
6. **다중 테넌시 / 조직 격리** — 기관별 `org_id` 분리, 테넌트 스코핑은 SPEC-AX-AUTH 계열 또는 미래 platform SPEC 책임. 본 SPEC은 단일 테넌트 가정 (research.md §8 조직 격리는 org_id 미래).
7. **마이그레이션 도구 통합 (alembic / golang-migrate)** — `0004_score_tables.sql` 수동 멱등 SQL 단일 패치. 마이그레이션 러너 도구 제외 (SPEC-AX-CTRL-001 / SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 §5 동일 정책).

---

## 6. 의존성 및 전제

- **SPEC-AX-CTRL-001 GREEN 가정**: `internal/store`의 `WorkflowStore`/`WorkflowTx` 인터페이스, `internal/audit`의 `Recorder`/`AuditTx`/`Action`/`Event`/`DefaultUserID`, 단일 pgx pool 와이어링이 모두 GREEN 상태이고 source-verified (research.md §1에서 `store.go`, `recorder.go`, `audit.go`, `pg_store.go`, `initial.sql` 실 시그니처 확인 — phantom API 없음).
- **SPEC-AX-EVID-001 완료(v0.1.2) 선례 재사용**: `EvidenceStore`/`EvidenceTx` 미러링 패턴, `RecordEvidenceCreated` 추가 패턴, `BeginEvidenceTx`(`pg_store.go` 실 pool 재사용) 진입점, FK-없는 stub-consumer 선례(§1.4 패턴), `0002_evidence_tables.sql` 멱등 SQL 규약을 본 SPEC의 `ScoreStore`/`ScoreTx`/`RecordScore*`/`BeginScoreTx`/`0004_score_tables.sql`이 동일하게 미러링한다.
- **SPEC-AX-EVAL-ITEM-001 완료(v0.1.3) 선례 재사용**: `EvalItemStore`/`EvalItemTx` 미러링, `RecordEvalItemCreated`/`Updated` 시그니처, `BeginEvalItemTx`, `0003_eval_item_tables.sql` 멱등 규약, `evaluation_items.id VARCHAR(64)` 계층코드 타입 결정(§1.4 본 SPEC stub 타입 계약의 근거), §6 OPEN→Run Phase 1 strategy+Human Gate RESOLVED 흐름(본 SPEC §3.5/plan.md §6도 동일 흐름으로 OPEN 유지)을 재사용한다.
- **`audit_logs` 테이블 스키마 재사용**: `initial.sql`의 `audit_logs`(id, user_id VARCHAR(64), action VARCHAR(64), resource_id `UUID NOT NULL`, resource_type VARCHAR(32), timestamp, details JSONB)를 그대로 사용한다. 본 SPEC은 `audit_logs` schema를 변경하지 않는다(§2.2 [EXISTING] HARD). `resource_id`(`uuid.UUID NOT NULL`) 매핑 전략은 §3.5 OPEN — `scores.id`가 UUID PK이면 Option 1(직접) 잠정, 비-UUID PK 결정 시 Option 2(EVAL-ITEM AUD-1 식 surrogate). Run Phase 1 strategy + Human Gate 확정.
- **`scores` 테이블 신규**: `0004_score_tables.sql`로 추가. `initial.sql` / `0002` / `0003` 미수정 (schema drift 방지, FK 하드닝 범위 밖 — §5 #3).
- **stub-consumer 단방향 type 계약**: 본 SPEC은 `scores`의 consumer이며 `evaluation_items`/`evidences`에 대한 역 FK는 생성하지 않는다(§5 Exclusion #3). 순환 의존 없음 — EVID-001/EVAL-ITEM-001은 본 SPEC 없이 이미 GREEN(완료)이다.
- **Go 1.22+**, module `github.com/ircp/iroum-ax`. 주요 의존성: `github.com/jackc/pgx/v5`, `go.uber.org/zap`, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go` (test). `github.com/google/uuid`는 `scores.id` UUID 및 audit `resource_id` 매핑에 사용될 수 있다(기존 import — recorder.go:91, 신규 외부 의존 0).
- **Cross-SPEC artifact 영향 없음**: 본 SPEC은 SPEC-AX-CTRL-001/EVID-001/EVAL-ITEM-001의 골든 파일 또는 기존 generated artifact를 수정하지 않는다 (clean additive 확장).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- **`evidences` / `evaluation_items` FK 하드닝**: `scores.evaluation_item_id`·`scores.evidence_id`에 FK를 추가하는 작업, 그에 수반되는 SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 코드·마이그레이션(`0002`/`0003`) 변경은 본 SPEC 범위 밖이다(§5 Exclusion #3). 본 SPEC은 type-compatible(VARCHAR(64) / UUID) `scores` 테이블 provider 역할만 하며 FK 하드닝은 미래 별도 SPEC이 수행한다. `evidences`/`evaluation_items` 테이블·코드를 본 SPEC 구현 중 수정하지 말 것.
- **풀 등급기준(scoring rubric) 시스템**: `metadata` JSONB / 최소 `grade_thresholds`는 점수→letter 최소 매핑 placeholder이며 풀 등급 산식·판정 룰 엔진 구조 설계는 본 SPEC에서 하지 않는다(SPEC-AX-EVAL-ITEM-001 §5 #2가 이미 이연 — open follow-up, plan.md §6).
- **LLM 등급 시뮬레이션 / Recommendation 엔진**: `product.md:171-183`/`:177`의 AI 기반 등급 시뮬레이션·추천은 후속 Python/AI `ingestion`/`mapping` 파이프라인 책임이며 본 SPEC 범위 아님. 본 SPEC 등급 산출은 내부 결정적 threshold만.
- **풀 집계 엔진 / 깊은 재귀 롤업 / 집계 snapshot**: 전체 트리 재귀 집계·`score_aggregates` 영속·incremental 재계산은 본 SPEC 범위 밖. 단일 레벨 `Σ(score×weight)` 최소 함수만.
- **점수 CRUD/REST/Console**: HTTP 엔드포인트, 점수 대시보드 UI는 본 SPEC 범위 밖. store 계층 + audit + 최소 롤업/등급만.
- **점수 권한/조직 격리**: SPEC-AX-AUTH 계열 책임. 본 SPEC은 cli-anonymous 기본값만.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/internal/{store,audit}/*_test.go` — 테이블 테스트, testify/assert, t.Parallel, goleak
- 통합 테스트: `apps/control-plane/internal/store/score_test.go` — testcontainers-go(postgres:16-pgvector), 점수 생성 + audit 원자성, 가중 롤업 최소 정확성, grade-threshold 경계
- 점수+감사 원자성 테스트: `apps/control-plane/internal/audit/recorder_score_test.go` — fault injection으로 audit INSERT 실패 시 scores INSERT 양방향 rollback 검증
- 데이터 주권 검증: 생성/조회/수정/롤업/등급 경로에서 외부 네트워크 egress 0건 (테스트 환경 외부 host 차단 + 코드 정적 검사)
- EVAL-ITEM-001 FK 타입 호환 검증: `scores.evaluation_item_id`의 정보 스키마상 데이터 타입이 `character varying(64)`인지 확인 (information_schema 쿼리)
- EVID-001 FK 타입 호환 검증: `scores.evidence_id`의 정보 스키마상 데이터 타입이 `uuid`(nullable)인지 확인 (information_schema 쿼리)
- cli-anonymous 기본값 검증: AuthN disabled 시 `scores.created_by` / `audit_logs.user_id` = 'cli-anonymous' literal
- 가중 롤업 최소 정확성: 알려진 score_value/weight 집합에 대해 `SumWeightedByEvaluationItem`이 `Σ(score×weight)`를 DECIMAL 정밀도로 정확 반환 (단일 레벨)
- 등급 threshold 경계 검증: 경계값 입력 시 결정적·일관 등급 산출 (동일 입력 → 동일 등급), 미설정 시 결정적 실패
- metadata opaque 검증: caller 제공 metadata JSONB가 구조 해석 없이 semantic value-equality로 round-trip 영속 (byte-level 비교 금지 — JSONB 정규화)
- 경계 검증: `scores.evaluation_item_id`/`scores.evidence_id`에 FK 제약이 부재하고 `evidences`/`evaluation_items`가 미수정임을 information_schema로 확인 (out-of-scope 경계 — AC-SCORE-BOUNDARY-1)
- 회귀: 기존 WorkflowStore/EvidenceStore/EvalItemStore 특성화 테스트가 score 와이어링 후에도 GREEN 유지

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.

---

## 9. Definition of Done (SPEC 단계)

- [ ] frontmatter 8-field canonical (plan.md L378) 준수, HISTORY + Schema note 포함
- [ ] EARS 5개 REQ 모듈(UBI 묶음 + 4 modal: 001/002/003/004) 모두 E/S/O/U 분류 명시, 모듈 ≤5
- [ ] §5 Exclusions ≥1 (풀 집계 엔진/REST/Console, 풀 rubric, EVID/EVAL-ITEM FK 하드닝, 풀 집계, LLM 시뮬레이션/Recommendation, 멀티테넌시, 마이그레이션 도구 — 7항목)
- [ ] §1.4 stub 계약: `scores.evaluation_item_id`=VARCHAR(64), `scores.evidence_id`=UUID, **FK 없음** — §1/§3.2/§3.2-S1/§7에 명시
- [ ] §3.5 audit-id 전략 / §3.4 grade-threshold 모델 위치 / 테이블 구조는 plan.md §6 OPEN/strategy-confirmable로 유지 (본 SPEC에서 미해결 — EVAL-ITEM-001 §6 pre-Run 동위상)
- [ ] acceptance.md 각 REQ ≥2 G/W/T, AC 명명 `AC-SCORE-{REQ}-{N}`, spec-compact.md 동기화
- [ ] 구현 코드/테스트 미작성 (SPEC 문서만)
