# SPEC-AX-SCORE-001 — Run Phase 1 전략 분석 & 실행 계획

> 작성: manager-strategy (UltraThink) · 2026-05-19 · 모드: Agent Teams (team run) · Harness: thorough · TDD
> 대상: SPEC-AX-SCORE-001 v0.1.1 (경영평가 점수 산출/집계) — iroum-ax Go control-plane brownfield 확장
> 본 문서는 Human Gate Decision Point 1 제출용. **§A의 §6 OPEN 4건 해결안이 사용자 사인오프 대상**.

---

## 0. 소스 검증 요약 (phantom-API 방지 — 코드 실측)

계획 수립 전 SPEC/research의 모든 load-bearing 주장을 grep으로 실 시그니처 검증 완료. (세션 lesson #9 phantom API 대응)

| 검증 대상 | 결과 | 근거 (file:line) |
|-----------|------|------------------|
| `audit.Event.ResourceID` 타입 | `uuid.UUID` ✓ | `internal/audit/audit.go:94` |
| `audit_logs.resource_id` 타입 | `UUID NOT NULL` ✓ | `.moai/db/schema/initial.sql:119` |
| 실 pgx pool 위치 | `PgWorkflowStore{pool *pgxpool.Pool}` ✓ | `internal/store/pg_store.go:26-28` |
| TX 진입점 선례 | `BeginEvidenceTx`/`BeginEvalItemTx` = `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})` ✓ | `pg_store.go:103-121` |
| **`postgres.go` 死 스텁 확인** | `func New(cfg)` + `TODO(Sprint 7)`, 실 pool 없음 ✓ | `internal/store/postgres.go:31-55` |
| Store 2계층 패턴 | `EvalItemStore`(Begin만) + `EvalItemTx`(in-TX ops) ✓ | `store.go:114-117,181` |
| Recorder 시그니처 패턴 | `RecordEvidenceCreated(ctx, tx AuditTx, ...)` / `RecordEvalItemCreated(...)` ✓ | `recorder.go:248,332` |
| 로컬 `AuditTx` 인터페이스 (순환의존 회피) | `type AuditTx interface` (recorder.go 내부) ✓ | `recorder.go:29` |
| AUD-1 surrogate 선례 | `EvalItemAuditNamespace = uuid.MustParse("a7f3c2e1-...")` + `uuid.NewSHA1(ns,[]byte(code))` ✓ | `audit.go:82`, `recorder.go:304` |
| `google/uuid` 기존 import | 존재 ✓ (신규 외부 의존 0) | `recorder.go:15` |
| `score.go` 미러 대상 | `eval_item.go` 456줄: `PgEvalItemTx`, `nullIfEmpty`, `validateStatusTransition`, `buildEvalItemUpdateSet` ✓ | `eval_item.go:31,439,254,293` |
| **EVID-001 불변/버전체이닝 선례** | `PreviousVersionID *uuid.UUID`, `Status 'ACTIVE'\|'SUPERSEDED'`, `version=prev.Version+1` (append-only) ✓ | `evidence.go:33,46-47,108` |
| 멱등 DDL 선례 | `CREATE TABLE IF NOT EXISTS` + `DO $$ ... EXCEPTION WHEN duplicate_object THEN NULL; END $$` + status `CHECK` ✓ | `0003_eval_item_tables.sql:7-33` |
| 마이그레이션 파일 | `0001/0002/0003` 존재, **`0004` 부재** (비충돌 확인) ✓ | `ls .moai/db/schema/migrations/` |

phantom 0건. 모든 계획은 실 코드 위에 수립됨.

---

## A. §6 OPEN/CONFIRMABLE 결정 4건 — 해결안 (Human Gate 사인오프 대상)

> EVAL-ITEM-001 §6 Option A + AUD-1 RESOLVED 흐름과 동위상. 잠정 권고가 아닌 **확정 권고 + 기각 사유 + 구현 함의**.

### 🔑 핵심 신규 발견 (NEW LOAD-BEARING FINDING) — 사용자 주목 요망

> **EVAL-ITEM-001 strategy는 `evaluation_items.id`가 `VARCHAR(64)` 계층코드여서 `audit.Event.ResourceID(uuid.UUID NOT NULL)`에 직접 못 넣는 타입 불일치를 *발견*하고 AUD-1 UUIDv5 surrogate를 강제당했다. SCORE-001은 그 클래스의 문제가 *존재하지 않는다*.**
>
> `scores.id`는 `UUID PK DEFAULT uuid_generate_v4()` (실 UUID). 따라서 `Event.ResourceID = score.id`는 `evidences`와 동일한 **타입 클린 직접 대입**이다. `parseResourceID`의 비-UUID→`uuid.Nil` 강등(데이터 손실)이 구조적으로 발생 불가 — AC-SCORE-004-1의 `resource_id != uuid.Nil` 단언이 항상 성립. **이것이 Decision 1과 Decision 2를 결합(coupling)시킨다: Decision 1 = Option A ⟹ UUID PK ⟹ Decision 2 = Option 1이 필연. Option 2(surrogate)는 비-UUID PK일 때만 살아있는데, 우리는 그것을 선택하지 않으므로 dead path다.** → `audit.go`에 신규 `ScoreAuditNamespace` 상수 **불필요**, `recorder.go`에 `uuid.NewSHA1` 호출 **불필요**.

---

### Decision 1 — Score 테이블 구조: **Option A 채택 (단일 `scores` + level discriminator)**

**권고: Option A.** 1개 테이블, `level VARCHAR(16) ∈ {raw, item, category}` discriminator.

**근거 (감사 원자성 — REQ-SCORE-UBI-002 재검증):**
- 검증된 Recorder 패턴: `Record*(ctx, tx AuditTx, ...)`는 단일 `AuditTx`로 1개 `audit.Event`를 동일 TX 삽입 (recorder.go:248-364). **불변식: 1 TX = 1 entity write = 1 Recorder 호출 = 1 audit row.**
- Option A는 raw 행이든 (미래 materialized) item/category 집계 행이든 **모든 행이 단일 `scores` 행**이므로 행 1개 생성/수정 = 1 TX = 1 audit row. level 무관하게 1:1:1 보존. PoC 최소 롤업(`SumWeightedByEvaluationItem`, REQ-SCORE-002-E1)은 raw 행 위 read-time 계산이며 item 행 영속을 요구하지 않음 → Option A는 raw-only PoC 동작(=Option C)을 포함하는 superset.

**기각 사유:**
- **Option B (scores + score_aggregates) — 기각.** 집계 행은 raw 행과 다른 시점에 기록됨. 감사하려면 (a) 집계 전용 별도 TX + 별도 audit row = **saga** (raw-TX 커밋 후 집계-TX 커밋, 파생 엔티티 audit이 원본 raw 행과 디커플 → orphan/staleness 윈도우), 또는 (b) 모든 raw InsertScore TX 안에서 score_aggregates UPDATE + 2번째 audit row = **1 TX에 2 entity write + 2 audit row → 1:1:1 형태 위반 + 무관한 쓰기 증폭**. EVAL-ITEM이 Option B(saga 복잡/orphan)를 기각한 것과 **구조적으로 동일한 실패**. plan.md §7 R-SCORE-007 / R-SCORE-001 정합. **검증된 Recorder 단일-AuditTx 구조에 비추어 독립 재도출한 기각** (앵커링 아님).
- **Option C (raw only + on-the-fly) — named post-PoC 전환 경로로만 보존.** 1:1:1은 만족하나 집계 snapshot 이력 부재 + 깊은 재귀 롤업 O(tree) read-time. 깊은 계층 재귀가 필요해지면 named 전환. PoC(안전보건 단일 범주 + 단일 레벨 최소 롤업)엔 불요. Option A가 C의 PoC 동작을 이미 포함.

**구현 함의:** `0004_score_tables.sql`의 plan.md §3 잠정 DDL을 **확정 DDL로 승격**. `scores.id UUID PK DEFAULT uuid_generate_v4()`. `level` CHECK ∈ {raw,item,category}, `status` CHECK ∈ {DRAFT,CONFIRMED,SUPERSEDED} (Decision 4 연계), `grade` CHECK ∈ {NULL,S,A,B,C,D}. EVAL-ITEM Option A(단일테이블+discriminator) 정합.

---

### Decision 2 — Audit `resource_id`: **Option 1 채택 (직접 UUID, surrogate 없음)** [Decision 1에 의해 필연]

**권고: Option 1 — `Event.ResourceID = score.id` 직접 대입.**

**근거:** Decision 1 = Option A ⟹ `scores.id`는 UUID PK. 검증: `audit.Event.ResourceID uuid.UUID`(audit.go:94), `audit_logs.resource_id UUID NOT NULL`(initial.sql:119). `evidences`(UUID PK)에 대한 `RecordEvidenceCreated`와 **동일한 타입 클린 직접 패턴**. 실 비즈니스 식별자(`evaluation_item_id` VARCHAR(64), `evidence_id` UUID, `level`, `grade`)는 SPEC §3.5 REQ-SCORE-004-E1대로 `DetailsJSON`에 보존.

**기각 사유 — Option 2 (EVAL-ITEM AUD-1 식 deterministic UUIDv5 surrogate):** EVAL-ITEM-001에서 surrogate가 강제된 이유는 `evaluation_items.id`가 비-UUID(VARCHAR(64) 계층코드)였기 때문. SCORE는 `scores.id`가 실 UUID PK라 그 전제가 부재. Option 2는 Decision 1이 비-UUID PK를 채택할 때만 유효한 **dead path**. → **`audit.go`에 `ScoreAuditNamespace` 상수 추가 안 함, `recorder.go`에 `uuid.NewSHA1` 호출 안 함.**

**구현 함의:** `recorder.go` `RecordScoreCreated/Updated`는 `score.ID`(uuid.UUID)를 `Event.ResourceID`에 직접 세팅. `audit.go` 변경 = `ActionScoreCreated`/`ActionScoreUpdated` **상수 2개만** (namespace 상수 0). 신규 외부 의존 0 (`google/uuid`는 이미 recorder.go:15 import, 본 경로엔 불요).

---

### Decision 3 — Grade-threshold 모델 위치: **Option (b) 채택 (최소 `grade_thresholds` 테이블)**

**권고: Option (b) — 최소 `grade_thresholds` 테이블** (`scope, letter, min_value, boundary_rule`, PK(scope,letter)). plan.md §3 주석 잠정 DDL을 본 DDL로 승격.

**근거 (REQ-SCORE-003-S1/U1 결정성 + O1 opacity 모순 해소):**
- **REQ-SCORE-003-S1**("config 미설정 시 결정적 구조화 실패"): 테이블이면 `SELECT ... WHERE scope=$1` → 0행 = 결정적 "unavailable" 에러, fabricate 0건. JSONB 방식은 scope 단위 "설정 존재" 판정이 ill-defined.
- **REQ-SCORE-003-U1**("경계값 결정성, `>=` vs `>`"): `boundary_rule` 명시 타입 컬럼으로 결정적. JSONB 임계값은 비결정.
- **O1 모순 해소**: REQ-SCORE-001-O1은 `metadata`를 **불투명/미해석**으로 계약. 그런데 grade 산출은 임계값을 **반드시 해석**해야 함. 해석 대상 임계값을 불투명 계약 컬럼(`scores.metadata`)에 넣으면 두 계약이 충돌. 별도 테이블은 충돌 제거.

**기각 사유 — Option (a) metadata JSONB:** S1의 scope 단위 결정적 미설정 판정 불가 + O1 opacity 계약과 자기모순. simplest해 보이나 요구사항을 결정적으로 만족 못 함.

**경계 준수 (over-engineering 아님):** 4컬럼 테이블 = S1+U1을 결정적으로 만족하는 **최소 구조(floor)**. 룰 엔진/가점·감점/rubric 계층 없음 — 그것이 "minimal internal grade-threshold model, full rubric out-of-scope"(spec.md §1.5/§5 #2, EVAL-ITEM-001 §5 #2 이연) 경계 안에 머무는 이유. 이것은 이연된 풀 rubric이 **아니다**.

**구현 함의:** `0004_score_tables.sql`이 **2개 테이블 생성** (`scores` + `grade_thresholds`), 둘 다 additive/멱등(동일 `DO $$` 패턴). `boundary_rule ∈ {gte, gt}`. **U1 기본 권고 = `gte`** (score ≥ min_value → letter, S→D 내림차순 스캔; 기획재정부 경영평가 편람 벤치마크의 "임계값=하한" 의미론 정합, research.md §3 `product.md:67/70`). PoC scope = `'default'` (또는 `'안전보건'`).

---

### Decision 4 — 점수 불변/정정 규칙 (REQ-SCORE-UBI-004): **status state-machine + append-only 정정 (하이브리드)**

**권고:** 두 완료 SPEC 선례를 통합한 최소 규칙:
1. **DRAFT 행은 in-place 가변** — `status='DRAFT'`인 동안 `UpdateScore`로 `score_value`/`weight`/`grade` 수정 허용 (확정 전 작업 상태). `eval_item.go:254` `validateStatusTransition` 가드 스타일 재사용.
2. **CONFIRMED 행은 불변** — `status='CONFIRMED'` 행의 실체 필드 변경 `UpdateScore`는 구조화 에러로 거부.
3. **CONFIRMED 점수 정정 = 신규 행 INSERT** (`evidence.go:108` append-only 버전체이닝 선례) + 구 행 `CONFIRMED→SUPERSEDED` status 전이 (비-DRAFT 행에 허용되는 **유일** 변형).
4. **물리 DELETE 어떤 status에서도 금지** — `DeleteScore` 메서드 미존재. AC-SCORE-UBI-004 "물리 삭제 0건" 충족.

**전이표 (최소):** `DRAFT→DRAFT`(값 편집), `DRAFT→CONFIRMED`(확정), `CONFIRMED→SUPERSEDED`(정정 시 값 동결). `SUPERSEDED` terminal.

**근거:** 점수는 evidentiary record(경영평가 등급 근거, 감사, 공공 무결성). `eval_item`(가변 taxonomy)의 in-place는 부적합. `evidence.go` append-only 선례 + `scores.status` enum(`DRAFT/CONFIRMED/SUPERSEDED`)이 **이미 Decision 1 DDL에 존재** → 신규 컬럼 0. 순수 append-only(EVID 식)만 쓰면 DRAFT 작업 중 매 수정마다 신규 행 = audit row 폭증 → DRAFT-가변 + CONFIRMED-동결 분할이 현실적 채점 워크플로우(작성→수정→확정→supersede 정정)를 지원하는 최소 규칙. 모든 전이/신규행 → `RecordScoreUpdated/Created` 감사(REQ-SCORE-UBI-002).

**구현 함의:** `score.go`에 `validateScoreStatusTransition` 헬퍼(eval_item 선례). `UpdateScore`는 DRAFT면 값 SET 허용 / CONFIRMED면 status-only(`→SUPERSEDED`)만 허용. 정정 = 신규 `InsertScore`. acceptance.md AC-SCORE-UBI-004의 "정정 = status 전이 또는 신규 행" 잠정 텍스트가 본 규칙으로 확정됨.

---

### 결정 간 일관성 (cross-decision coherence) — 인지편향 점검 통과

- D1=A (UUID PK, level discriminator) **⟹** D2=Option1 (직접 score.id, namespace 상수 0) — D1이 D2를 강제.
- D3=`grade_thresholds` 테이블 ⟹ `0004` = `scores` + `grade_thresholds` (둘 다 additive/멱등).
- D4=status state-machine ⟹ D1 DDL의 `status` enum 재사용 + `eval_item.validateStatusTransition` + `evidence.go` 신규행 선례 재사용 (신규 컬럼 0).
- **신규 외부 의존 0** (어느 경로에도). phantom 0 (`BeginScoreTx`→`pg_store.go PgWorkflowStore.pool`, `postgres.go` 死 스텁 비대상 — 검증됨).
- 앵커링/확증/단순성 편향 점검: 4건 모두 검증된 선례에 1:1 매핑되며 plan.md 잠정안을 독립 재도출로 재검증 (B는 검증된 Recorder 단일-AuditTx 구조로 독립 기각, D3 2번째 테이블은 over-engineering 아닌 S1/U1 floor, D4 하이브리드는 audit-row 폭증 회피 위한 최소 규칙).

---

## B. 실행 계획 (plan_summary)

### B.1 단계 접근 — [DELTA] 순서 + Agent Teams 파일 소유권

브라운필드 [DELTA] 순서: **[EXISTING] 특성화 baseline → [NEW/MODIFY] TDD RED-GREEN-REFACTOR**. plan.md §4 Sprint(S0~S6) 우선순위 라벨 유지(시간 추정 금지).

**Agent Teams 모드** (backend-dev implementer + tester, 병렬, `isolation: worktree`). 파일 소유권 분리(쓰기 충돌 방지):

| 팀원 | 소유 파일 (배타) | 비고 |
|------|------------------|------|
| **backend-dev** (implementer, isolation:worktree, mode:acceptEdits) | `internal/store/store.go` [MODIFY], `internal/store/score.go` [NEW], `internal/store/pg_store.go` [MODIFY], `internal/audit/audit.go` [MODIFY], `internal/audit/recorder.go` [MODIFY], `.moai/db/schema/migrations/0004_score_tables.sql` [NEW] | 프로덕션 코드 + 마이그레이션 |
| **tester** (tester, isolation:worktree, mode:acceptEdits) | `internal/store/score_test.go` [NEW], `internal/audit/recorder_score_test.go` [NEW], `internal/store/pg_store_test.go` 회귀 확인 [EXISTING] | `*_test.go` **배타 소유** |

> [HARD] 두 팀원 모두 implementer/tester role → `isolation: "worktree"` 필수. backend-dev는 `*_test.go` 미수정, tester는 프로덕션 코드 미수정. TDD: tester가 RED 작성 → backend-dev가 GREEN. 동일 모듈 RED/GREEN 동기화는 공유 TaskList + SendMessage 조정. 프롬프트는 **상대경로**만 (worktree 격리 — `internal/store/score.go`, `cd /abs` 금지).

### B.2 Sprint 계획 (plan.md §4 준수 — §6 RESOLVED 반영)

| Sprint | 우선순위 | 내용 | REQ | 소유 |
|--------|----------|------|-----|------|
| S0 | High | [EXISTING] 특성화 baseline: 기존 Workflow/Evidence/EvalItem `pg_store_test.go` GREEN 확인, `0004` 비충돌 재확인 | (전제) | tester |
| S1 | High | `0004_score_tables.sql` 멱등 작성 (**확정 Option A** + `grade_thresholds` 2테이블) + `ScoreStore`/`ScoreTx` 인터페이스 (store.go) | REQ-SCORE-001 | backend-dev |
| S2 | High | `score.go` `PgScoreTx`(InsertScore/GetScoreByID/GetScoresByEvaluationItem/UpdateScore) + `BeginScoreTx` (**pg_store.go** PgWorkflowStore.pool, postgres.go 死스텁 아님) + 입력검증(U1) + stub 타입(S1) | REQ-SCORE-001 | backend-dev / tester(RED) |
| S3 | High | `audit.go` `ActionScoreCreated`/`Updated` **상수 2개만(namespace 0)** + `recorder.go` `RecordScoreCreated`/`RecordScoreUpdated`(로컬 AuditTx, **직접 score.id UUID**) + 동일 TX 원자성 + 양방향 rollback(U1) | REQ-SCORE-004, UBI-002 | backend-dev / tester(RED) |
| S4 | Medium | `SumWeightedByEvaluationItem` 단일레벨 `Σ(score×weight)` DECIMAL + NULL weight 결정적 정책 (002-U1) | REQ-SCORE-002 | backend-dev / tester(RED) |
| S5 | Medium | `grade_thresholds` 조회 + score→letter 결정적 매핑(`gte` 기본, S→D 스캔) + 경계/미설정 결정적 실패 (003-S1/U1) + status state-machine(D4) | REQ-SCORE-003, UBI-004 | backend-dev / tester(RED) |
| S6 | Medium | REFACTOR(헬퍼 분리: `validateScoreInput`/`computeWeightedSum`/`resolveGrade`/`validateScoreStatusTransition`, 복잡도≥15 회피) + @MX 태그 + 커버리지≥85% + BOUNDARY-1 + evaluator-active strict≥0.75 | 전체 | backend-dev / tester |

각 Sprint: tester RED(실패 테스트) → backend-dev GREEN(최소 구현) → 공동 REFACTOR. brownfield: RED 전 `eval_item.go`/`evidence.go`/`recorder.go` 패턴 정독 (workflow-modes.md Brownfield Enhancement).

### B.3 확정 DDL (`0004_score_tables.sql` — Option A + grade_thresholds, 멱등)

plan.md §3 잠정 DDL을 확정. `scores`(§3 본 DDL 그대로, `id UUID PK DEFAULT uuid_generate_v4()`, `evaluation_item_id VARCHAR(64) NOT NULL` FK없음, `evidence_id UUID NULL` FK없음, `level`/`status`/`grade` CHECK) **+** plan.md §3 주석의 `grade_thresholds`(scope, letter, min_value, boundary_rule DEFAULT 'gte', PK(scope,letter)) 주석 해제하여 본 테이블로. `0003` 멱등 규약(`CREATE TABLE IF NOT EXISTS`/`DO $$ EXCEPTION WHEN duplicate_object`/`CREATE INDEX IF NOT EXISTS`) 그대로. `initial.sql`/`0002`/`0003` **미수정**.

---

## C. 요구사항 → REQ 매핑 & 성공 기준

| REQ 모듈 | 핵심 | 성공 기준 (16 AC) |
|----------|------|-------------------|
| REQ-SCORE-UBI-001~004 | 데이터주권/감사/cli-anonymous/불변 | AC-SCORE-UBI-001~004 |
| REQ-SCORE-001 (모델·Store) | scores + ScoreStore/Tx, stub 타입, 입력검증, metadata opaque | AC-SCORE-001-1/2/3/4/O1-1 (5개) |
| REQ-SCORE-002 (가중 롤업 최소) | 단일레벨 Σ(score×weight), NULL weight 결정적 | AC-SCORE-002-1/2 |
| REQ-SCORE-003 (등급 최소) | 결정적 threshold, 경계/미설정 결정적 실패 | AC-SCORE-003-1/2 |
| REQ-SCORE-004 (감사 연계) | RecordScore*, 동일 TX, 양방향 rollback | AC-SCORE-004-1/2 |
| 경계 | EVID/EVAL-ITEM FK 부재 + 미수정 | AC-SCORE-BOUNDARY-1 |

**전체 성공 기준 (acceptance.md §9 DoD):** 16 AC 자동화 통과, 15 edge case 대응, coverage≥85%, golangci-lint default+gosec 0, `goleak.VerifyNone(t)` 통과, 기존 Workflow/Evidence/EvalItem 특성화 회귀 0, @MX 태그(plan.md §5), manager-quality TRUST 5 통과, evaluator-active per-sprint strict≥0.75. §6 OPEN 의존 AC(UBI-004, 002-2, 003-1/2, 004-1)는 본 §A 확정에 맞춰 구체 단언 정정.

---

## D. 기술 스택 & 의존성

- Go 1.22+, module `github.com/ircp/iroum-ax`. `jackc/pgx/v5`(pool/TX/DECIMAL), `go.uber.org/zap`(구조화 로그), `stretchr/testify`(단위), `testcontainers-go`(postgres:16-pgvector 통합), `goleak`.
- DECIMAL: `score_value DECIMAL(6,2)`, `weight DECIMAL(5,4)`, `min_value DECIMAL(6,2)` — 부동소수 오차 0 (AC-SCORE-002-1 `Σ` 정확성).
- 수동 SQL `0004` 단일 패치 (마이그레이션 러너 미사용, §5 #7).
- **신규 외부 의존 0건** (데이터 주권 REQ-SCORE-UBI-001 정합). `google/uuid` 기존 import(recorder.go:15) — Decision 2 직접 UUID 경로엔 surrogate 불요라 사실상 미사용.

---

## E. 복잡도 & 노력 (우선순위 라벨 — 시간 추정 없음)

| 영역 | 우선순위 | 복잡도 근거 |
|------|----------|-------------|
| `0004` DDL 2테이블 멱등 | High | 0003 선례 직접 미러, 낮음 |
| `ScoreStore/Tx` + `score.go` | High | `eval_item.go`(456줄) 미러, 중간 (InsertScore/Get/Update/SumWeighted) |
| `BeginScoreTx` (pg_store.go) | High | `BeginEvalItemTx`(pg_store.go:118) 3줄 미러, 낮음 — **phantom 주의** |
| `recorder.go` RecordScore* | High | `RecordEvalItemCreated`(recorder.go:332) 미러, 직접 UUID(surrogate 없음)라 EVAL-ITEM보다 단순 |
| 가중 롤업 / 등급 결정성 | Medium | DECIMAL 정밀도·경계 결정성 (@MX:WARN 영역) |
| status state-machine (D4) | Medium | `validateStatusTransition`(eval_item.go:254) + `evidence.go` 신규행 통합 |

---

## F. 참조 구현 (research.md / 코드 file:line — 미러 대상)

- `store.go:114-117,181` `EvalItemStore`/`EvalItemTx` → `ScoreStore`/`ScoreTx` 미러
- `pg_store.go:111-121` `BeginEvalItemTx` (단일 pool 재사용, `stderrors.ErrPgxPoolExhausted` 래핑) → `BeginScoreTx` **(pg_store.go, postgres.go 아님)**
- `eval_item.go:31-456` `PgEvalItemTx`/`nullIfEmpty`/`validateStatusTransition`/`buildEvalItemUpdateSet` → `score.go` 미러 (REFACTOR 헬퍼 분리 선례)
- `recorder.go:332-364` `RecordEvalItemCreated/Updated`(로컬 `AuditTx` recorder.go:29) → `RecordScoreCreated/Updated` 미러 (단, **AUD-1 surrogate 미적용** — Decision 2)
- `evidence.go:33,46-47,108` `PreviousVersionID`/`Status ACTIVE|SUPERSEDED`/append-only `version+1` → Decision 4 정정=신규행 선례
- `0003_eval_item_tables.sql:7-33` 멱등 `CREATE TABLE IF NOT EXISTS`+`DO $$ EXCEPTION`+`CHECK`+`CREATE INDEX IF NOT EXISTS` → `0004` 미러
- AUD-1: `audit.go:82` `EvalItemAuditNamespace` / `recorder.go:304` `uuid.NewSHA1` — **SCORE에선 미사용 근거**(Decision 2 직접 UUID)

---

## G. 리스크 처리 (plan.md §7 R-SCORE-001~008)

| ID | 완화 (본 계획 반영) |
|----|---------------------|
| R-SCORE-001 (§6 미확정 재작업) | **본 §A에서 4건 RESOLVED** — Human Gate 사인오프 후 코드 동결 |
| R-SCORE-002 (FK 잘못 추가) | spec.md §1.4/§5#3 HARD, AC-SCORE-BOUNDARY-1로 FK 부재+EVID/EVAL-ITEM 미수정 검증 (tester) |
| R-SCORE-003 (DECIMAL/누락 가중치) | `@MX:WARN`+`@MX:REASON`, 002-U1 결정적 정책(제외/에러 strategy 확정→S4), 알려진 집합 정확성 AC |
| R-SCORE-004 (등급 경계 비결정) | `@MX:WARN`, `grade_thresholds.boundary_rule='gte'` 결정적, 동일입력→동일등급 AC |
| R-SCORE-005 (audit 부분 커밋) | 동일 TX + `tx.Rollback` 양방향, fault injection AC (EVID/EVAL-ITEM 선례) |
| R-SCORE-006 (phantom postgres.go) | **§0 검증**: `BeginScoreTx`→`pg_store.go PgWorkflowStore.pool`. postgres.go=死스텁(TODO Sprint 7) 비대상 |
| R-SCORE-007 (Option B saga) | **Decision 1에서 B 명시 기각** (1:1:1 불변식 위반, Recorder 단일-AuditTx 구조로 독립 재도출) |
| R-SCORE-008 (rubric/LLM scope 폭증) | spec.md §5#2/#5 HARD. `grade_thresholds`=4컬럼 최소(rubric 엔진 아님), 등급=내부 결정적 threshold만 |

---

## H. @MX 태그 계획 (plan.md §5)

| 대상 | 태그 | 사유 |
|------|------|------|
| `pg_store.go` `BeginScoreTx` | `@MX:ANCHOR` | 단일 풀 TX 진입점, fan_in≥3 invariant 계약 |
| `recorder.go` `RecordScoreCreated/Updated` | `@MX:ANCHOR` | 동일-TX audit 불변식 강제, fan_in≥3 |
| `score.go` `SumWeightedByEvaluationItem` | `@MX:WARN`+`@MX:REASON` | DECIMAL 가중합산 누락/정밀도 왜곡 (002-U1) |
| `score.go` grade 산출(threshold/경계) | `@MX:WARN`+`@MX:REASON` | 경계 결정성·미설정 실패 (003-U1) |
| `score.go` `InsertScore`/`UpdateScore` | RED `@MX:TODO` → GREEN `@MX:NOTE` | TDD 진행 표시 (EVAL-ITEM M2 패턴) |

---

## I. Human Gate — 사인오프 요청 (Decision Point 1)

아래 §6 OPEN 4건 해결안 승인 요청. 승인 시 코드 동결, Agent Teams(backend-dev+tester) 구현 착수:

1. **Decision 1**: Score 테이블 = **Option A** (단일 scores + level discriminator). Option B/C 기각.
2. **Decision 2**: Audit resource_id = **Option 1** (직접 `score.id` UUID, surrogate/namespace 상수 없음) — Decision 1에 의해 필연.
3. **Decision 3**: Grade-threshold = **Option (b)** 최소 `grade_thresholds` 테이블 (boundary_rule 기본 `gte`).
4. **Decision 4**: 불변/정정 = **status state-machine + append-only** (DRAFT 가변 / CONFIRMED 불변 / 정정=신규행+CONFIRMED→SUPERSEDED / 물리삭제 0).

**🔑 신규 load-bearing 발견 (사용자 주목):** SCORE-001은 EVAL-ITEM-001이 겪은 audit resource_id 타입 불일치(VARCHAR PK→uuid.Nil 강등)가 **구조적으로 부재** — `scores.id`가 UUID PK이므로 직접 매핑 타입 클린. Decision 1↔2 결합으로 AUD-1 surrogate 불필요(audit.go namespace 상수 0). EVAL-ITEM-001보다 단순.
