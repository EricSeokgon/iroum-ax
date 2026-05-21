# SPEC-AX-EVAL-ITEM-001 — Phase 1 전략 분석 및 실행 계획

> Run Workflow Phase 1 (Analysis & Planning). UltraThink. 코드 구현 없음 — 분석/계획만.
> 대응 SPEC: `.moai/specs/SPEC-AX-EVAL-ITEM-001/` (spec.md v0.1.2, plan.md v0.1.2, acceptance.md 21 AC, research.md Phase 0.5)
> 방법론: TDD (RED-GREEN-REFACTOR), harness: thorough

---

## 0. Phantom-Path 검증 결과 (실 시그니처 source-verified)

Grep로 실 코드 시그니처를 확인했다 (lesson #9 phantom-API 재발 방지). 계획은 검증된 API에만 의존한다.

| 심볼 | 실 위치 (verified) | 사실 |
|------|-------------------|------|
| `PgWorkflowStore{pool *pgxpool.Pool}` | `pg_store.go:26-28` | 실 pgx pool **존재** |
| `NewPgWorkflowStore` | `pg_store.go:38-55` | `server.go` 와이어링 진입점 |
| `BeginEvidenceTx` (미러 대상) | `pg_store.go:103` `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})` | **BeginEvalItemTx가 복제할 정확한 패턴** |
| `postgres.go` `New(cfg)` | `postgres.go:38` `// @MX:TODO - Sprint 7`, TODO-only body | **死 스텁 — 본 SPEC 대상 아님** (phantom-path 회피 확정) |
| `EvidenceStore`/`EvidenceTx` 인터페이스 | `store.go:61-104` | 미러링 대상 |
| `Recorder.RecordEvidenceCreated` | `recorder.go:248` `(ctx, tx AuditTx, evidenceID, evalItemID, fileHashSHA256 string, version int, userID string) error` | RecordEvalItem* 시그니처 템플릿 |
| `AuditTx` 로컬 인터페이스 | `recorder.go:29` | store→audit 순환 회피 (재사용) |
| `audit.Action` 상수 | `audit.go:13`, `ActionEvidenceCreated`=`"EVIDENCE_CREATED"` `:57` | 신규 상수 추가 패턴 |
| 마이그레이션 디렉터리 | `.moai/db/schema/migrations/` — `0001_initial.sql`, `0002_evidence_tables.sql` 실재 | 신규 = `0003` 비충돌 확정 |
| `evidences.evaluation_item_id` | `0002_evidence_tables.sql:8` `VARCHAR(64) NOT NULL -- 경량 FK stub, 제약 없음` | §1.4 HARD type-compat 계약 확인 |

**[HARD] 확정**: `BeginEvalItemTx`는 `pg_store.go` `PgWorkflowStore.pool`를 재사용한다 (`pg_store.go:103` `BeginEvidenceTx` 패턴 복제). `postgres.go`는 절대 대상이 아니다.

---

## 1. 핵심 결정 #1 [Human Gate 필수] — §6 Taxonomy 테이블 구조 = **Option A 확정 권고**

> SPEC-AX-EVID-001 §6의 storage-strategy resolution과 동등한 위상의 strategy-confirmable 결정. plan.md §6이 OPEN 상태로 남긴 항목. 사용자 sign-off 필요.

### 1.1 First-Principles 분석

축소 불가능한 요구사항 두 가지가 구조를 결정한다:
- **REQ-EVALITEM-UBI-002 (load-bearing)**: 모든 항목 create/update = 동일 TX 내 정확히 1개 `audit_logs` row. → "1 논리 엔티티 변경 = 1 TX = 1 Recorder 호출 = 1 audit row" 가 가능한 구조여야 한다.
- **§1.4 HARD 계약**: `evaluation_items.id` = VARCHAR(64) 계층 코드, `evidences.evaluation_item_id VARCHAR(64)`(`0002_evidence_tables.sql:8` verified) stub과 type-compat. → per-node 단일 VARCHAR(64) PK 필요.

### 1.2 가중 트레이드오프 매트릭스 (가중치 = SPEC 명시 핵심가치 기반, 1-10)

| 기준 (가중치) | A 자기참조 adjacency | B 4-table 정규화 | C closure table | D JSONB nested |
|---------------|---------------------|------------------|-----------------|----------------|
| 감사 원자성 호환 (UBI-002) **30%** | 10 | 2 | 7 | 2 |
| EVID §1.4 HARD type-compat **25%** | 10 | 3 | 8 | 1 |
| 구현 비용/패턴 재사용 **20%** | 9 | 3 | 4 | 6 |
| 유지보수/스키마 최소성 **15%** | 8 | 4 | 5 | 5 |
| 확장성 (깊은 subtree — 본 SPEC 이연) **10%** | 5 | 7 | 9 | 6 |
| **가중 합계** | **9.0** | **3.25** | **6.55** | **3.40** |

순위: **A 9.0** >> C 6.55 > D 3.40 > B 3.25.

### 1.3 권고: **Option A (자기참조 adjacency list)** — 확정

- ONE 테이블, ONE row/논리 항목, PK = VARCHAR(64) 계층 코드 → §1.4 HARD 직접 충족.
- 항목 1개 생성 = 1 INSERT + 1 Recorder + 1 audit row, 단일 `EvalItemTx` → UBI-002 완벽 충족.
- 검증된 GREEN 선례(`store.go:73` `EvidenceTx`, `pg_store.go:103` `BeginEvidenceTx`)를 그대로 미러 → 구현 비용 최소.

### 1.4 기각/이연 사유 (독립 도출 — research §5 단순 echo 아님)

- **B 기각 (구조적 부적격)**: 단일 논리 "평가항목"이 category/item/indicator 4 테이블에 분산 → "1 TX 1 entity 1 audit" **구조적 위반** (saga/loop 필요). 추가로 `evidences.evaluation_item_id` FK가 4 PK 중 어느 테이블을 가리킬지 **FK-target 모호성** 발생(독립 발견 — research §5 미언급). → §1.4 HARD 계약 훼손.
- **D 기각 (HARD 계약 파괴)**: JSON-nested는 per-node 행/PK 부재 → `evidences.evaluation_item_id` type-compat-FK 대상 불가 → 본 SPEC 존재 이유(EVID stub provider) 자체 붕괴. 감사도 whole-tree 단위(per-item 불가) → UBI-002 위반.
- **C 이연 (post-PoC 정당)**: C는 B와 달리 감사 원자성을 위반하지 **않음**(논리 엔티티 1개 = 1 audit 방어 가능). 유일 결격 = 매 TX closure 일관성 비용 vs 그 이득(빠른 임의 깊이 subtree)이 본 SPEC에서 **명시적으로 이연된 capability**(spec.md §3.3 단일 레벨 `GetEvalItemsByParentID`만, 재귀 subtree 범위 밖, R-EVALITEM-001). 이연 비용을 선지불하는 셈. → **named post-PoC 마이그레이션 경로로 유지** (research §5 정합).

### 1.5 인지 편향 점검

- 앵커링: research §5 + plan §6이 A 선권고 → 단순 확인 우려. 반증 테스트: `EvidenceTx` GREEN 패턴(store.go:73, pg_store.go:103)을 독립 검증, B의 FK-target 모호성·D의 no-PK 문제를 research §5에 없던 근거로 독립 도출 → 검증(validation)이지 고무도장 아님.
- A 실패 시나리오: (1) PoC 범위가 post-PoC 전 깊은 재귀 subtree로 확대 → WITH RECURSIVE 성능(완화: `@MX:WARN` + `evaluation_items_parent_id_idx` + `hierarchy_code` 경로 인코딩, R-EVALITEM-001). (2) 순환 parent 참조 — DB CHECK 불가, 앱 검증만(R-EVALITEM-004, REQ-EVALITEM-002-U1이 PoC 수동 규율로 명시 수용). 둘 다 SPEC-인지 잔여 리스크, A-결정적 결함 아님.

### 1.6 본 SPEC이 Option A로 보장하는 것

- `evaluation_items.id` = VARCHAR(64) 계층 코드 (UUID/auto-inc 아님) — EVID stub type-compat (HARD)
- 자기참조 `parent_id` FK + `ON DELETE RESTRICT` + `hierarchy_code` UNIQUE
- create/update가 단일 `EvalItemTx` 내 `audit_logs` 1건과 atomic
- `evidences` 테이블·코드 불변 (FK 하드닝 미수행 — AC-EVALITEM-BOUNDARY-1)

---

## 2. 핵심 결정 #2 [Human Gate 필수, 신규 — plan.md §6 미열거] — Audit `resource_id` 타입 불일치 해소

> **strategy 단계에서 새로 surface된 load-bearing 발견.** spec.md §6 라인 209가 "`resource_id` 컬럼이 UUID 타입일 경우 ... run 단계 store 구현 시 ... 확인 후 결정"으로 명시 이연한 항목 — 즉 SPEC 작성자가 미해결을 인지하고 run으로 미룬 결정. EVID-001 storage-strategy 해소와 동일 위상의 Human Gate 항목으로 격상한다 (run 중 묵시 결정 금지).

### 2.1 모순 (verified)

- `audit.Event.ResourceID` 타입 = **`uuid.UUID`** (`audit.go:72` verified)
- `audit_logs.resource_id` 컬럼 = **`UUID NOT NULL`** (`initial.sql:119` verified)
- `Recorder.parseResourceID(s string) uuid.UUID`는 파싱 실패 시 **`uuid.Nil`** 반환 (`recorder.go:90-96` verified)
- `evaluation_items.id` = VARCHAR(64) 계층 코드 `"AX-SAFETY-ORG-01"` (UUID 아님) → `parseResourceID("AX-SAFETY-ORG-01")` = `uuid.Nil`
- **EVID가 안 부딪힌 이유**: `evidences.id`는 UUID PK이므로 `RecordEvidenceCreated`의 `ResourceID=parseResourceID(evidenceID)`가 정상 UUID. eval-item-id는 `DetailsJSON.evaluation_item_id` 문자열로만 들어감(`recorder.go:250`). EVAL-ITEM은 주 리소스 PK 자체가 VARCHAR(64)라 정면 충돌.
- **SPEC 텍스트 결함**: acceptance.md AC-EVALITEM-003-1 (`ResourceID="AX-SAFETY-ORG-01"`)은 `uuid.UUID` 컬럼이 보유 불가한 값을 단언 → 올바른 구현에서도 false RED. AC 문구 재조정 필요.

### 2.2 옵션

| 옵션 | 방식 | Pros | Cons | 본 SPEC |
|------|------|------|------|---------|
| **AUD-1 (권고)** | `ResourceID` = 계층 코드 기반 **deterministic UUIDv5** (고정 namespace + `uuid.NewSHA1`); 실 식별자는 `DetailsJSON.{hierarchy_code, parent_id?, level?, eval_item_id}` (REQ-EVALITEM-003-E1이 이미 hierarchy_code 의무화) | `audit_logs` 스키마 불변(initial.sql 미수정 — spec §2.2 HARD), `resource_id` NOT NULL 충족, 결정적·충돌없음·재현가능, 신규 외부 dep 0 (`google/uuid` 이미 import `recorder.go:91`) | `resource_id` 비가독(식별은 DetailsJSON 경유), 고정 namespace 상수 1개 추가, AC-003-1 문구 재조정 필요 | **SELECTED 권고** |
| AUD-2 | `ResourceID = uuid.Nil` + DetailsJSON 의존 | 0 신규 코드, EVID parseResourceID-fallback과 동일 거동 | 모든 eval-item audit row가 `uuid.Nil` 공유 → resource_id 기반 audit 쿼리 붕괴, 다른 Nil-id row와 충돌, 감사 추적성(UBI-002 품질) 저하 | 약함 — 비권고 |
| AUD-3 | `audit_logs` 스키마 변경(resource_id text화 또는 컬럼 추가) | 의미적 최정합 | **spec §2.2 [EXISTING] "initial.sql 미수정" HARD 위반** + schema drift + CTRL/EVID 골든 거동 영향 (spec.md "Cross-SPEC artifact 영향 없음" 위배) | **기각** |

### 2.3 권고: **AUD-1** (deterministic UUIDv5 surrogate + DetailsJSON 식별)

- `audit_logs` 스키마 불변 (HARD: initial.sql 미수정 — spec §2.2 / Cross-SPEC 무영향 보장).
- `resource_id` NOT NULL을 결정적·충돌없는·재현가능 값으로 충족; 실 계층 식별은 REQ-EVALITEM-003-E1이 이미 의무화한 `DetailsJSON.hierarchy_code`에 위치.
- 신규 외부 의존 0 (`google/uuid` recorder.go:91 기존 import; `uuid.NewSHA1`은 동일 패키지 — 데이터 주권 REQ-EVALITEM-UBI-001 정합). plan.md §4 "id에 UUID 미사용"은 그대로 유효 (id는 VARCHAR(64) 유지; UUIDv5는 audit `resource_id` surrogate에만).
- **종속 산출물**: AC-EVALITEM-003-1 / AC-EVALITEM-003-2 "ResourceID='AX-SAFETY-ORG-01'" 문구를 "DetailsJSON 식별 + 결정적 resource_id 파생" 검증으로 재조정 (SPEC 텍스트 정정 — Human Gate 승인 시 acceptance.md 갱신, run 진입 전).

---

## 3. 요구사항 → REQ 매핑 & 성공 기준 (21 AC)

| REQ 모듈 | 요구사항 핵심 | 대응 AC | 성공 기준 |
|----------|--------------|---------|-----------|
| REQ-EVALITEM-UBI-001 데이터 주권 | 생성/조회/수정/검증 외부 호출 0 | AC-EVALITEM-UBI-001 | 네트워크 spy + 정적 import 검사 0건 |
| REQ-EVALITEM-UBI-002 감사 가능성 | 모든 create/update = 동일 TX audit 1건 | AC-EVALITEM-UBI-002, AC-003-1/2 | testcontainers row count 정확 1 |
| REQ-EVALITEM-UBI-003 cli-anonymous | created_by/user_id = literal 'cli-anonymous' | AC-EVALITEM-UBI-003 | 컬럼 byte 비교, NULL 금지 |
| REQ-EVALITEM-UBI-004 계층 불변 | 자식 보유 항목 parent_id/level 불변 | AC-EVALITEM-UBI-004, AC-004-1 | mutation guard, SQL 미실행 |
| REQ-EVALITEM-001 모델/Store | 루트/자식 생성, parent 검증, id 검증, metadata opaque | AC-001-1,2,3,4, AC-001-S1-1, AC-001-O1-1 | InsertEvalItem + BeginEvalItemTx GREEN, p99<50ms |
| REQ-EVALITEM-002 계층/자기참조 | parent_id NULL root, 자식 조회, FK RESTRICT, hierarchy_code UNIQUE | AC-002-1,2,3,4 | GetEvalItemsByParentID + 제약 위반 검증 |
| REQ-EVALITEM-003 감사 연계 | RecordEvalItemCreated/Updated, audit 실패 양방향 rollback | AC-003-1,2,3 | recorder 2 메서드 + tx.Rollback 원자성 |
| REQ-EVALITEM-004 계층 불변/lifecycle | parent_id/level 변경 거부, status CHECK, 잎 노드 수정 | AC-004-1,2,3 | mutation guard + CHECK + 단일 TX update |
| 경계 (EVID out-of-scope) | evidences FK 부재 유지, evidences 미수정 | AC-EVALITEM-BOUNDARY-1 | information_schema FK 부재 확인 |

전체 21 AC: §0 UBI 4, §1 6, §2 4, §3 3, §4 3, §5 1. DoD = 21 AC 자동화 통과 + coverage≥85% + golangci-lint(default+gosec) 0 + goleak 통과 + 기존 Workflow/Evidence 특성화 회귀 0 + evaluator-active strict per-sprint ≥0.75.

---

## 4. 기술 스택 & 의존성

- Go 1.22+, module `github.com/ircp/iroum-ax`
- `github.com/jackc/pgx/v5` (기존 단일 pool 재사용 — `pg_store.go:26` `PgWorkflowStore.pool`)
- `go.uber.org/zap` (기존), `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go`(postgres:16-pgvector), `go.uber.org/goleak`
- `github.com/google/uuid` — **기존 import** (`recorder.go:91` `uuid.Parse`), AUD-1 권고 시 `uuid.NewSHA1` 동일 패키지 사용 → **신규 외부 dep 0건** (데이터 주권 REQ-EVALITEM-UBI-001 정합)
- 마이그레이션: 수동 멱등 SQL (`CREATE TABLE IF NOT EXISTS`, `DO $$ ... EXCEPTION WHEN duplicate_object ...`, `CREATE INDEX IF NOT EXISTS`) — `0002_evidence_tables.sql:6-34` 패턴 미러. 도구 미사용
- `id`에 UUID 미사용 (VARCHAR(64) 계층 코드 유지) — plan.md §4 정합

---

## 5. 인터페이스 형태 (EvidenceStore/EvidenceTx 미러 — store.go:61-104 verified)

```
EvalItemStore interface {
    BeginEvalItemTx(ctx) (EvalItemTx, error)        // pg_store.go BeginEvidenceTx:103 미러 (s.pool 재사용)
}
EvalItemTx interface {
    InsertEvalItem(ctx, id string, parentID *string, displayName, description string,
        level *int, hierarchyCode string, weight *float64, maxScore *int,
        status string, metadata map[string]any) (string, error)   // ← string id 반환 (EvidenceTx의 uuid.UUID와 다름: PK가 VARCHAR(64))
    GetEvalItemByID(ctx, id string) (*EvalItem, error)             // ErrEvalItemNotFound 래핑 (raw pgx.ErrNoRows 금지 — GAP-03 선례)
    GetEvalItemsByParentID(ctx, parentID string) ([]*EvalItem, error)  // evaluation_items_parent_id_idx 사용
    UpdateEvalItem(ctx, id string, ...) error                      // successor 존재 시 parent_id/level 변경 거부
    InsertAuditLog(ctx, *audit.Event) error                        // store.go:40/100 패턴 동일
    Commit(ctx) error
    Rollback(ctx) error
}
```

Recorder 확장 (recorder.go:248 RecordEvidenceCreated 시그니처 미러, 로컬 AuditTx):
```
RecordEvalItemCreated(ctx, tx AuditTx, itemID, hierarchyCode string, parentID *string, level *int, userID string) error
RecordEvalItemUpdated(ctx, tx AuditTx, itemID, hierarchyCode string, parentID *string, level *int, userID string) error
// 내부: ResourceID = (AUD-1) deterministic UUIDv5(itemID) [Human Gate 확정 대상],
//       DetailsJSON = {hierarchy_code, parent_id?, level?, eval_item_id} (REQ-EVALITEM-003-E1),
//       ResourceType="evaluation_item", UserID=resolveUserID(userID)
```

audit.go 신규 상수 (audit.go:57 ActionEvidenceCreated 패턴):
```
ActionEvalItemCreated Action = "EVAL_ITEM_CREATED"
ActionEvalItemUpdated Action = "EVAL_ITEM_UPDATED"
```

---

## 6. 단계별 실행 계획 ([DELTA] 순서 준수)

### Phase A — [EXISTING] 특성화 baseline (행동 변경 0)
- `pg_store_test.go` 기존 WorkflowStore/EvidenceStore 테스트 GREEN 확인 (eval-item 와이어링 전 baseline 캡처)
- `recorder` 기존 RecordCreated/RecordEvidence* 회귀 baseline

### Phase B — Sprint 0 Foundation ([NEW] migration + [MODIFY] 골격)
- [NEW] `0003_eval_item_tables.sql` (멱등, §7 DDL — `id VARCHAR(64) PK`, parent 자기 FK ON DELETE RESTRICT, status CHECK, parent_id/hierarchy_code/created_at 인덱스). `initial.sql` 미수정
- [MODIFY] `store.go` `EvalItemStore`/`EvalItemTx` 인터페이스 선언 (시그니처만)
- [MODIFY] `audit.go` `ActionEvalItemCreated`/`ActionEvalItemUpdated` 상수
- 기존 특성화 회귀 GREEN 유지 확인

### Phase C — Sprint 1-4 TDD 사이클 (RED→GREEN→REFACTOR)
- **Sprint 1 (REQ-EVALITEM-001)**: RED InsertEvalItem(루트)+GetEvalItemByID → GREEN [NEW] `eval_item.go` `PgEvalItemTx` (pg_store.go `PgEvidenceTx` 미러), [MODIFY] `pg_store.go` `BeginEvalItemTx` (**`PgWorkflowStore.pool` 재사용, NOT postgres.go**). RED 자식 생성+parent 검증(S1)+id/dup 거부(U1) → GREEN parent 사전조회+PK/길이/blank 검증. AC-001-1,2,3,4, AC-001-S1-1, AC-001-O1-1
- **Sprint 2 (REQ-EVALITEM-002)**: RED GetEvalItemsByParentID + root parent_id NULL → GREEN 자기참조 SELECT(parent_id_idx). RED FK ON DELETE RESTRICT + hierarchy_code UNIQUE → GREEN DDL 제약 + store 사전검증. AC-002-1,2,3,4
- **Sprint 3 (REQ-EVALITEM-003)**: RED RecordEvalItemCreated/Updated audit row → GREEN [MODIFY] `recorder.go` 2 메서드 (RecordEvidenceCreated 시그니처 미러, **AUD-1 deterministic UUIDv5 resource_id — Human Gate 확정 후 구현**). RED audit INSERT 실패→양방향 rollback(U1) → GREEN store TX orchestration. AC-003-1,2,3
- **Sprint 4 (REQ-EVALITEM-004 + UBI 통합)**: RED 자식 보유 parent_id/level 변경 거부(S1/UBI-004) → GREEN UpdateEvalItem mutation guard (successor 확인 후 거부, SQL 미실행). RED status 전이+열거외/NULL 거부(U1)+잎 노드 속성 수정(O1) → GREEN status CHECK+잎 update. RED 외부 egress 0(UBI-001)+cli-anonymous(UBI-003)+evidences FK 부재 경계(BOUNDARY-1) → GREEN resolveUserID 재사용+정적 보장+evidences 미수정 확인. AC-004-1,2,3, AC-UBI-001/003/004, AC-BOUNDARY-1

### Phase D — Sprint 5 Quality Gate
- coverage≥85%, golangci-lint default+gosec 0, goleak 전체 통과
- @MX 태그 plan.md §5 매핑 완료 (RED `@MX:TODO` 전부 해소 → `@MX:ANCHOR`/`@MX:WARN` 확정)
- TRUST 5, evaluator-active strict per-sprint ≥0.75 (thorough harness)

복잡도: **PRIORITY High** (load-bearing §1.4 계약 + 신규 audit 타입 결정 + 5 REQ 모듈). 시간 추정 없음.

---

## 7. `0003_eval_item_tables.sql` DDL 최종형 (Option A 확정 전제)

```sql
-- 마이그레이션: 0003_eval_item_tables (SPEC-AX-EVAL-ITEM-001, 멱등, 수동 SQL)
-- id = 계층 코드 VARCHAR(64) (UUID 아님) — evidences.evaluation_item_id VARCHAR(64) stub type-compat
CREATE TABLE IF NOT EXISTS evaluation_items (
    id              VARCHAR(64) PRIMARY KEY,                                       -- 계층 코드 (예: AX-SAFETY-ORG-01)
    parent_id       VARCHAR(64) REFERENCES evaluation_items(id) ON DELETE RESTRICT, -- 자기참조, root는 NULL
    display_name    VARCHAR(256) NOT NULL,
    description     TEXT,
    level           INT,                                                           -- 1범주 2항목 3지표 4배점 (informational)
    hierarchy_code  VARCHAR(128) UNIQUE,                                           -- 경로 인코딩 (중복 방지)
    weight          DECIMAL(5,4),                                                  -- 0.0-1.0 nullable
    max_score       INT,
    status          VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    metadata        JSONB,                                                         -- opaque placeholder (본 SPEC 미해석)
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by      VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at     TIMESTAMP WITH TIME ZONE
);
DO $$ BEGIN
    ALTER TABLE evaluation_items ADD CONSTRAINT evaluation_items_status_chk
        CHECK (status IN ('ACTIVE','DEPRECATED','ARCHIVED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
CREATE INDEX IF NOT EXISTS evaluation_items_parent_id_idx ON evaluation_items (parent_id);
CREATE INDEX IF NOT EXISTS evaluation_items_hierarchy_code_idx ON evaluation_items (hierarchy_code);
CREATE INDEX IF NOT EXISTS evaluation_items_created_at_idx ON evaluation_items (created_at DESC);
```

DDL 근거: research §9. `initial.sql` 미수정 (schema drift 방지, spec §2.2). `0003` = `0001`+`0002` 비충돌 (Phase 0 verified).

---

## 8. 리스크 처리 (plan.md §7 R-EVALITEM-001..006 매핑)

| Risk | 완화 |
|------|------|
| R-EVALITEM-001 재귀 쿼리 성능 | 단일 레벨 `GetEvalItemsByParentID`만 (재귀 subtree 범위 밖), `parent_id_idx`+`hierarchy_code`, 계층 순회 로직 `@MX:WARN` |
| R-EVALITEM-002 evidences FK 지연 | type-compat 테이블 provider만, FK 하드닝 미래 SPEC, AC-BOUNDARY-1 경계 확인 |
| R-EVALITEM-003 편람 import | import 이연 (spec §5 #1) |
| R-EVALITEM-004 순환 parent | DB CHECK 불가 — 앱 검증+PoC 수동 규율 (REQ-EVALITEM-002-U1), `@MX:WARN` |
| R-EVALITEM-005 audit 폭증 | write-once, PoC 안전보건 단일 범주, 단일 pgx pool 재사용 (`PgWorkflowStore.pool`) |
| R-EVALITEM-006 cli-anonymous 마스킹 | `resolveUserID` 재사용, created_by DEFAULT 'cli-anonymous' |
| **신규 R: audit resource_id 타입 불일치** | **§2 핵심 결정 #2 (AUD-1 권고) — Human Gate 확정. 미해소 시 false RED / 약한 감사 추적** |

---

## 9. Surface Assumptions (Human Gate 사인오프 항목)

1. **§6 = Option A** (검증된 9.0 vs C 6.55 결정적 우위) — EVID-001 storage-strategy 해소와 동위상, 명시 사인오프 필요.
2. **신규 핵심 결정 #2: audit resource_id 타입 불일치** — AUD-1 (deterministic UUIDv5 surrogate + DetailsJSON 식별) 권고. **SPEC 텍스트 결함**(AC-003-1/2 "ResourceID='AX-SAFETY-ORG-01'"는 uuid.UUID 컬럼 보유 불가)이므로 run 중 묵시 결정 금지 — Human Gate 승인 시 acceptance.md AC-003-1/2 문구 정정(run 진입 전).
3. AUD-1은 신규 외부 dep 0 (`google/uuid` 기존 import); `id`는 VARCHAR(64) 유지 (UUIDv5는 audit surrogate 한정).

---

## 10. Definition of Done (Plan 관점)

- [ ] §1 §6 결정 = Option A 사용자 사인오프
- [ ] §2 핵심 결정 #2 = AUD-1(또는 사용자 선택) 사인오프 + acceptance.md AC-003-1/2 문구 정정 합의
- [ ] Phase A-D 전부 GREEN, 21 AC 자동화 통과
- [ ] coverage≥85%, golangci-lint default+gosec 0, goleak 통과
- [ ] BeginEvalItemTx = `PgWorkflowStore.pool` 재사용 (postgres.go 死 스텁 비대상) 확인
- [ ] `evaluation_items.id`=VARCHAR(64) (information_schema), `evidences.evaluation_item_id` FK 부재 (AC-BOUNDARY-1), evidences 미수정
- [ ] 기존 Workflow/Evidence 특성화 회귀 0, TRUST 5, evaluator-active strict ≥0.75
