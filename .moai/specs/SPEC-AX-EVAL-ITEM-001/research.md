# Research: SPEC-AX-EVAL-ITEM-001 경영평가 평가항목 taxonomy (Evaluation Item Taxonomy)

Phase: 0.5 Deep Research
Generated: 2026-05-18
Agent: Explore (read-only deep codebase analysis)
Status: complete

> 본 문서는 SPEC-AX-EVAL-ITEM-001 EARS 요구사항 설계 근거입니다. 모든 주장은 file:line 근거 포함.

---

## 1. Architecture Analysis — Store/Audit 재사용 맵

- `WorkflowStore`/`WorkflowTx` 2계층 패턴 (`internal/store/store.go:20-28`, `:36-53`) → `EvalItemStore.BeginEvalItemTx` + `EvalItemTx{InsertEvalItem, GetEvalItemByID, GetEvalItemsByParentID, InsertAuditLog, Commit, Rollback}` 미러링.
- 단일 pgx pool: `PgWorkflowStore{pool}` (`pg_store.go:26-31`, `NewPgWorkflowStore` `:38-56`). `BeginTx`(:83-89), `BeginEvidenceTx`(:96-109) 선례. [HARD] `BeginEvalItemTx`는 `s.pool` 재사용 — 신규 풀 금지. `postgres.go`는 Sprint-0 死 스텁(비대상).
- Recorder 패턴: `Recorder{clock, authEnabled}` (`recorder.go:38-46`), `RecordCreated(ctx, tx AuditTx, ...)` 시그니처(:100-117), `resolveUserID`(:81-86), `parseResourceID`(:90-96), 로컬 `AuditTx` 인터페이스(:29-32, store→audit 순환 회피). → `RecordEvalItemCreated/Updated` 동일 패턴 추가.

## 2. EVID-001 Stub 계약 — PK 타입/의미 [핵심]

- `0002_evidence_tables.sql:8`: `evaluation_item_id VARCHAR(64) NOT NULL -- 경량 FK stub, 제약 없음`
- `SPEC-AX-EVID-001/spec.md:50`: 평가항목 taxonomy는 EVID 범위 밖, `evidences.evaluation_item_id`는 FK 제약 없는 VARCHAR(64) stub, `evaluation_items` 테이블 미생성.
- **[HARD] 계약**: `evaluation_items.id`는 **VARCHAR(64)** (UUID 아님) — 미래 FK `evidences.evaluation_item_id → evaluation_items(id)` 타입 호환 필수. 식별자는 계층 코드(예: `AX-SAFETY-ORG-01-1`), auto-increment/random UUID 아님.
- EVID-001 코드 변경 및 `evidences`에 FK 소급 추가는 **본 SPEC 범위 밖** (미래 별도 SPEC). `AC-EVID-001-3`(acceptance.md:136-149)이 FK 부재 stub 동작 검증 중.

## 3. 경영평가 편람 계층 의미 (실세계 도메인)

- `product.md:62`: PoC 평가항목 = "안전 보건" (경영평가 500개 항목 중 1개), `:67` 기획재정부 경영평가 편람, `:160` "계층: 항목 → 지표 → 배점".
- 실세계 4계층: Level0 평가범주 → Level1 평가항목 → Level2 평가지표 → Level3 배점·가중치·등급기준(S/A/B/C/D). PoC는 "안전보건" 범주 전체.
- 식별자 계층 코드 예: `AX-SAFETY-ORG-01`(항목) → `AX-SAFETY-ORG-01-1`(지표). 등급기준은 metadata 또는 별도 테이블(후속 이연).

## 4. Migration & Schema 규약 (0003_eval_item_tables.sql)

- 멱등성(`0002_evidence_tables.sql:6-34`): `CREATE TABLE IF NOT EXISTS`, `DO $$ BEGIN ALTER ... ADD CONSTRAINT ... EXCEPTION WHEN duplicate_object THEN NULL; END $$;`, `CREATE INDEX IF NOT EXISTS`.
- `uuid-ossp`/`pgvector`는 `initial.sql:11-12`에 이미 존재. [HARD] `initial.sql` 미수정.
- 컬럼 규약(`initial.sql:49-82`): VARCHAR(N) 식별자/이름, TEXT 본문, DECIMAL(5,4) 가중치, TIMESTAMP WITH TIME ZONE, JSONB metadata. 상태/타입은 `VARCHAR + CHECK` 권장(0002 storage_strategy 선례 :27-29, ENUM 마이그레이션 회피).
- 자기참조 FK: `parent_id VARCHAR(64) REFERENCES evaluation_items(id) ON DELETE RESTRICT` (root는 parent_id NULL). evidences→evaluation_items 역 FK는 0003에서 생성 금지(미래 별도 작업).

## 5. Taxonomy 테이블 구조 옵션 (트레이드오프 — strategy 단계 결정 이연)

| Option | 구조 | Pros | Cons | 감사 호환 |
|--------|------|------|------|-----------|
| **A 자기참조 adjacency list** (PoC 권고) | 단일 테이블 + parent_id 자기 FK | 최소 스키마, 임의 깊이, 단일 TX 1 entity+audit, CTRL-001 단순성 미러 | 재귀 쿼리(WITH RECURSIVE), 깊이 미강제 | 1행 1 Recorder 1 audit (동일 TX) — 적합 |
| B 다단계 정규화 (categories/items/indicators/scoring) | 4 테이블 FK 체인 | DB 타입 안전, 범주 쿼리 빠름 | **4 audit 루프/saga 필요 — "1 TX 1 entity" 위배** | 부적합 (분산 TX) |
| C closure table | items + closure | 서브트리 쿼리 빠름 | 2테이블 insert, closure 일관성 유지 부담 | 2 Recorder/항목 — 복잡 |
| D JSONB nested | 단일 hierarchy_json | 1행 insert, 스키마리스 | 참조 무결성 0, evidence FK 불가, coarse audit | 계층당 1 audit (항목 단위 불가) |

권고: PoC는 **Option A**. B/C/D는 post-PoC 이연. strategy 단계가 최종 확정.

## 6. SPEC 문서 규약 (EVID-001 v0.1.2 템플릿 재사용)

- frontmatter 8필드(`EVID-001/spec.md:1-10`): id, version(0.1.0), status(draft), created/updated(2026-05-18), author(ircp), priority(high), issue_number(0).
- HISTORY + Schema note 블록(`:12-16`). 섹션: §1 개요(Walking Skeleton/Anchor/Composite domain) §2 영향파일([DELTA]) §3 EARS(Ubiquitous+E/S/U/O) §4 NFR §5 Exclusions §6 의존성 §7 Out of Scope §8 검증요약.
- AC 명명 `AC-EVALITEM-{REQ}-{N}` (acceptance.md Given/When/Then). spec-compact.md 자동 생성.

## 7. REQ-UBI 패턴 & 한국 공공 6제약 (verbatim)

- canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track (`EVID-001/spec.md:91-99`). EVAL-ITEM 적용:
  - REQ-EVALITEM-UBI-001 (데이터 주권): 외부 서비스 호출 0, 내부 PostgreSQL만.
  - REQ-EVALITEM-UBI-002 (감사 가능성): 모든 항목 create/update → 동일 TX `audit_logs` 1건.
  - REQ-EVALITEM-UBI-003 (cli-anonymous 기본값): auth 비활성 시 `created_by`/`user_id` = literal 'cli-anonymous'(NULL 금지).
  - REQ-EVALITEM-UBI-004 (계층 불변): 자식 존재 시 parent_id/level 변경 금지.
- 6제약: 데이터 주권 / 언어(한글 display_name) / 감사 가능성 / 망분리 / 조직 격리(org_id 미래) / 시간 제약(p99<50ms).

## 8. Risks, Constraints, Implicit Contracts

- R-EVALITEM-001 재귀 쿼리 성능(깊은 계층) — parent_id 인덱스 + hierarchy_code 경로 인코딩, closure post-PoC.
- R-EVALITEM-002 evidences→evaluation_items FK 지연 — orphan evidence 가능(EVID-001 설계 수용, 미래 SPEC 하드닝).
- R-EVALITEM-003 편람 import upsert 복잡 — import 이연(§5 제외).
- R-EVALITEM-004 순환 parent 참조 — DB CHECK 불가, 앱 검증(PoC 수동 규율).
- R-EVALITEM-005 audit 행 폭증(500항목 일괄) — write-once, 파티셔닝 이연.
- R-EVALITEM-006 cli-anonymous 마스킹 — 미래 auth가 override.
- 암묵 계약: 단일 pgx pool 재사용, 1 TX=1 entity+audit, 로컬 AuditTx(순환 회피), ON DELETE RESTRICT, 멱등 마이그레이션, cli-anonymous literal.

## 9. Recommended Implementation Approach

제안 DDL (Option A):
```sql
CREATE TABLE IF NOT EXISTS evaluation_items (
    id              VARCHAR(64) PRIMARY KEY,                                  -- 계층 코드 (EVID-001 FK 호환)
    parent_id       VARCHAR(64) REFERENCES evaluation_items(id) ON DELETE RESTRICT,
    display_name    VARCHAR(256) NOT NULL,
    description     TEXT,
    level           INT,                                                      -- 1범주 2항목 3지표 4배점 (informational)
    hierarchy_code  VARCHAR(128) UNIQUE,                                      -- 경로 인코딩
    weight          DECIMAL(5,4),                                             -- 0.0-1.0 nullable
    max_score       INT,                                                      -- nullable
    status          VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',                    -- CHECK
    metadata        JSONB,                                                    -- 등급기준 등
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by      VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at     TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS evaluation_items_parent_id_idx ON evaluation_items(parent_id);
CREATE INDEX IF NOT EXISTS evaluation_items_hierarchy_code_idx ON evaluation_items(hierarchy_code);
CREATE INDEX IF NOT EXISTS evaluation_items_created_at_idx ON evaluation_items(created_at DESC);
DO $$ BEGIN
  ALTER TABLE evaluation_items ADD CONSTRAINT evaluation_items_status_chk
    CHECK (status IN ('ACTIVE','DEPRECATED','ARCHIVED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
```

Store: `internal/store/eval_item.go` [NEW] `EvalItem` struct + `EvalItemTx`; `store.go` [MODIFY] 인터페이스 선언; `pg_store.go` [MODIFY] `BeginEvalItemTx`(s.pool 재사용) + `PgEvalItemTx`. Audit: `audit.go` [MODIFY] `ActionEvalItemCreated` 등 상수; `recorder.go` [MODIFY] `RecordEvalItemCreated` (RecordCreated 패턴, 로컬 AuditTx). Migration `0003_eval_item_tables.sql` [NEW].

EVID-001 경계: 본 SPEC은 `evaluation_items` 제공자. evidences FK 하드닝은 §Out of Scope 명시(미래 SPEC).

strategy 단계 결정 항목: Option A 확정 vs 재고, 등급기준 저장(metadata JSONB vs 별도 테이블 — 후속 이연), FK 하드닝 타임라인.

복합 도메인: `SPEC-AX-EVAL-ITEM-001` (AX + EVAL-ITEM). 의존성: SPEC-AX-CTRL-001 store/audit GREEN, SPEC-AX-EVID-001 0002 마이그레이션 존재(evaluation_item_id VARCHAR(64) stub, FK 없음). TDD, thorough harness.
