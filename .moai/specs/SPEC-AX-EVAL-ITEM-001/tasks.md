# SPEC-AX-EVAL-ITEM-001 — Tasks (SDD Atomic Task Decomposition)

> SPEC: `.moai/specs/SPEC-AX-EVAL-ITEM-001/spec.md` v0.1.3
> Plan: `.moai/specs/SPEC-AX-EVAL-ITEM-001/plan.md` v0.1.3 (§2 Sprint 0-5, §6 Option A RESOLVED, §6.6 AUD-1)
> Methodology: TDD (RED-GREEN-REFACTOR), harness: thorough
> Acceptance: `.moai/specs/SPEC-AX-EVAL-ITEM-001/acceptance.md` (22 AC, 18 edge cases)
> [DELTA] order: [EXISTING] characterization baseline → [MODIFY] characterize-modify-verify → [NEW] full RED-GREEN-REFACTOR
> Phantom-path [HARD]: `BeginEvalItemTx` targets `pg_store.go` `PgWorkflowStore.pool` (BeginEvidenceTx pattern, pg_store.go:103). `postgres.go` is a Sprint-0 dead stub — NOT a target.
> coverage_verified: true (all 22 AC + every REQ-EVALITEM-UBI/001/002/003/004 E·S·U·O sub-clause incl. 003-E2 mapped to ≥1 task)

## Task Table

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | [EXISTING] baseline + [NEW] migration: 기존 WorkflowStore/EvidenceStore/Recorder 특성화 테스트가 GREEN임을 확인(회귀 baseline 캡처). Option A `0003_eval_item_tables.sql` 멱등 SQL 작성 (`id VARCHAR(64) PK`, 자기참조 `parent_id` self-FK `ON DELETE RESTRICT`, `hierarchy_code VARCHAR(128) UNIQUE`, `status` CHECK `{ACTIVE,DEPRECATED,ARCHIVED}`, parent_id/hierarchy_code/created_at 인덱스, `created_by DEFAULT 'cli-anonymous'`). `initial.sql` 미수정 [HARD]. testcontainers에서 `0001→0002→0003` 적용 + information_schema로 `evaluation_items.id` = `character varying(64)` 확인. | REQ-EVALITEM-001 (엔티티/DDL), REQ-EVALITEM-002-S1 (parent FK ON DELETE RESTRICT DDL), REQ-EVALITEM-004-U1 (status CHECK DDL) | (none) | [NEW] `.moai/db/schema/migrations/0003_eval_item_tables.sql`; [NEW] `apps/control-plane/internal/store/eval_item_migration_test.go`; [EXISTING] `apps/control-plane/internal/store/pg_store_test.go`, `apps/control-plane/internal/audit/recorder_test.go` (baseline GREEN 확인, 미수정) | completed |
| T-002 | [MODIFY] 인터페이스/상수 골격: `store.go`에 `EvalItemStore`(BeginEvalItemTx) / `EvalItemTx`(InsertEvalItem, GetEvalItemByID, GetEvalItemsByParentID, UpdateEvalItem, InsertAuditLog, Commit, Rollback) 인터페이스 선언 (시그니처만, `EvidenceStore`/`EvidenceTx` 미러, `@MX:ANCHOR`). `audit.go`에 `ActionEvalItemCreated`/`ActionEvalItemUpdated Action` 상수 + **AUD-1 `EvalItemAuditNamespace` 고정 UUID 상수 1개** 추가. 컴파일 GREEN + 기존 store/audit 특성화 회귀 0건 확인. | REQ-EVALITEM-001 (Store 추상화), REQ-EVALITEM-003 (Action 상수), REQ-EVALITEM-003-E2 (AUD-1 namespace 상수 사전 정의) | T-001 | [MODIFY] `apps/control-plane/internal/store/store.go` (EvalItemStore/EvalItemTx 인터페이스); [MODIFY] `apps/control-plane/internal/audit/audit.go` (ActionEvalItem* 상수 + EvalItemAuditNamespace 고정 UUID 상수); [EXISTING] `apps/control-plane/internal/store/pg_store_test.go` (회귀 확인) | completed |
| T-003 | [NEW] RED-GREEN-REFACTOR: 루트 항목 생성 happy path. `eval_item_test.go` RED(InsertEvalItem 미구현) → GREEN `eval_item.go` `PgEvalItemTx`(pg_store.go `PgEvidenceTx` 미러) `InsertEvalItem`(루트 parent_id NULL)+`GetEvalItemByID`. 루트 1 row(parent_id IS NULL, status='ACTIVE' DEFAULT, created_by='cli-anonymous'), p99<50ms(10회). metadata JSONB opaque verbatim semantic round-trip(byte 비교 금지, JSONB 정규화 제외). | REQ-EVALITEM-001-E1 (단일 TX INSERT), REQ-EVALITEM-002-E1a (root parent_id NULL), REQ-EVALITEM-001-O1 (metadata opaque verbatim) | T-002 | [NEW] `apps/control-plane/internal/store/eval_item.go` (PgEvalItemTx, InsertEvalItem, GetEvalItemByID); [NEW] `apps/control-plane/internal/store/eval_item_test.go` (AC-EVALITEM-001-1, AC-EVALITEM-001-3, AC-EVALITEM-001-O1-1, AC-EVALITEM-002-1 root 부분) | completed |
| T-004 | [NEW]+[MODIFY] RED-GREEN-REFACTOR: 자식 생성 + parent 검증 + 입력 검증 + TX 진입점. RED 자식 항목(non-NULL parent_id)·존재하지 않는 parent_id·id blank/64자초과/display_name blank/중복 PK → GREEN parent 사전 조회 + FK 위반 거부(orphan 방지, TX 미커밋) + pre-INSERT 검증. [MODIFY] `pg_store.go`에 `BeginEvalItemTx` 진입점 추가 — **실 `PgWorkflowStore.pool` 재사용 (pg_store.go:103 `BeginEvidenceTx` 패턴 복제), `postgres.go` 死 스텁 비대상 [HARD]**, `@MX:ANCHOR`. | REQ-EVALITEM-001-E1 (BeginEvalItemTx 진입점), REQ-EVALITEM-001-S1 (parent 존재 검증/orphan 거부), REQ-EVALITEM-001-U1 (id/display_name/중복 거부), REQ-EVALITEM-002-E1b (child non-NULL parent_id self-link) | T-003 | [NEW] `apps/control-plane/internal/store/eval_item.go` (parent 사전 조회, 입력 검증 추가); [MODIFY] `apps/control-plane/internal/store/pg_store.go` (BeginEvalItemTx — PgWorkflowStore.pool 재사용); [NEW] `apps/control-plane/internal/store/eval_item_test.go` (AC-EVALITEM-001-2, AC-EVALITEM-001-S1-1, AC-EVALITEM-001-4) | completed |
| T-005 | [NEW] RED-GREEN-REFACTOR: 계층 자기참조 조회. RED `GetEvalItemsByParentID` 미구현/계층 단절 → GREEN 자기참조 SELECT(`evaluation_items_parent_id_idx` 사용, EXPLAIN 검증, p99<50ms). root parent_id NULL + 단일 부모 자식 목록 + 다단계(L1→L2→L3) 단계별 하향 순회(재귀 subtree는 범위 밖 — `@MX:WARN`). | REQ-EVALITEM-002-E1a (root 조회), REQ-EVALITEM-002-E1b (자식 조회 self-link) | T-004 | [NEW] `apps/control-plane/internal/store/eval_item.go` (GetEvalItemsByParentID, `@MX:WARN` 계층 순회); [NEW] `apps/control-plane/internal/store/eval_item_test.go` (AC-EVALITEM-002-1, AC-EVALITEM-002-4) | completed |
| T-006 | [NEW] RED-GREEN-REFACTOR: 계층 제약 강제(FK RESTRICT + hierarchy_code UNIQUE). RED 자식 보유 parent DELETE 성공(잘못)·hierarchy_code 중복 INSERT 허용(잘못) → GREEN DDL FK `ON DELETE RESTRICT` 강제(직접 SQL DELETE 거부, 두 row 보존) + `evaluation_items_hierarchy_code_idx` UNIQUE 위반 거부(TX 미커밋, 기존 row 불변) + store 사전 검증. | REQ-EVALITEM-002-S1 (ON DELETE RESTRICT), REQ-EVALITEM-002-U1 (hierarchy_code UNIQUE) | T-005 | [NEW] `apps/control-plane/internal/store/eval_item.go` (hierarchy_code 사전 검증); [NEW] `apps/control-plane/internal/store/eval_item_test.go` (AC-EVALITEM-002-2, AC-EVALITEM-002-3) | completed |
| T-007 | [MODIFY] RED-GREEN-REFACTOR: 감사 연계 — AUD-1 deterministic UUIDv5. RED `RecordEvalItemCreated`/`RecordEvalItemUpdated` 미구현 + (잘못) 원시 계층코드 `resource_id` 단언(false RED) → GREEN `recorder.go`에 2개 메서드 추가 (`RecordEvidenceCreated` recorder.go:248 시그니처 미러, 로컬 `AuditTx`, `@MX:ANCHOR`). **`Event.ResourceID = uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))`** (결정적 UUIDv5 surrogate, 원시 계층코드 아님, `!= uuid.Nil`, RFC4122 v5). 실 식별자(`eval_item_id`,`hierarchy_code`,`parent_id`,`level`)는 `DetailsJSON`. Action ∈ {EVAL_ITEM_CREATED,EVAL_ITEM_UPDATED}, ResourceType="evaluation_item", UserID=resolveUserID. RED 결정성(동일 hierarchy_code 재기록→byte-identical UUID, 상이 hierarchy_code→상이 UUID) → GREEN `uuid.NewSHA1`(고정 namespace) 결정적 산출. 신규 외부 dep 0 (`github.com/google/uuid` recorder.go:91 기존 import). | REQ-EVALITEM-003-E1 (UUIDv5 surrogate + DetailsJSON 식별), REQ-EVALITEM-003-E2 (결정성/재현성), REQ-EVALITEM-UBI-002 (동일 TX audit 1건) | T-002, T-004 | [MODIFY] `apps/control-plane/internal/audit/recorder.go` (RecordEvalItemCreated/RecordEvalItemUpdated — uuid.NewSHA1 surrogate, DetailsJSON); [NEW] `apps/control-plane/internal/audit/recorder_eval_item_test.go` (AC-EVALITEM-003-1, AC-EVALITEM-003-2, AC-EVALITEM-003-E2-1) | completed |
| T-008 | [NEW] RED-GREEN-REFACTOR: 감사 원자성(양방향 rollback). RED audit_logs INSERT fault injection 시 evaluation_items row 잔존(잘못) → GREEN store TX orchestration: audit INSERT 실패 시 `tx.Rollback(ctx)`로 evaluation_items + audit 모두 미존재(부분 커밋 0건), wrapped audit 에러 반환, goroutine leak 0. create + update 경로 모두. | REQ-EVALITEM-003-U1 (audit fail → all-or-nothing rollback), REQ-EVALITEM-UBI-002 (TX 미커밋 시 audit 미기입) | T-007 | [NEW] `apps/control-plane/internal/store/eval_item.go` (create/update TX orchestration, audit 실패 rollback); [NEW] `apps/control-plane/internal/audit/recorder_eval_item_test.go` (AC-EVALITEM-003-3); [NEW] `apps/control-plane/internal/store/eval_item_test.go` (양방향 rollback 통합) | completed |
| T-009 | [NEW] RED-GREEN-REFACTOR: 계층 불변성 & 라이프사이클. RED 자식 보유 항목 parent_id/level 변경 성공(잘못)·status 열거외/NULL 허용(잘못)·잎 노드 update 미구현 → GREEN `UpdateEvalItem` mutation guard(successor 존재 확인 후 parent_id/level 변경 거부, SQL 미실행, `@MX:WARN`) + status CHECK 사전 검증(ACTIVE→DEPRECATED→ARCHIVED 전이, 열거외/NULL 거부) + 잎 노드 비-계층 속성(display_name/description/weight/max_score/status/metadata) 단일 TX update + 동일 TX `EVAL_ITEM_UPDATED` audit 1건. metadata update 경로 verbatim 갱신. | REQ-EVALITEM-004-S1 (자식 보유 parent_id/level 변경 거부), REQ-EVALITEM-004-U1 (status 열거외/NULL 거부), REQ-EVALITEM-004-O1 (잎 노드 비-계층 속성 수정 + audit), REQ-EVALITEM-UBI-004 (계층 불변) | T-007, T-008 | [NEW] `apps/control-plane/internal/store/eval_item.go` (UpdateEvalItem mutation guard + status 검증 + 잎 update, `@MX:WARN`); [NEW] `apps/control-plane/internal/store/eval_item_test.go` (AC-EVALITEM-004-1, AC-EVALITEM-004-2, AC-EVALITEM-004-3, AC-EVALITEM-UBI-004) | completed |
| T-010 | [NEW] UBI 통합 + 경계 + Quality Gate. RED+GREEN: 외부 네트워크 egress 0건(네트워크 spy + 정적 import 검사 — internal/store·internal/audit 외부 SaaS SDK 미import) + cli-anonymous 기본값(created_by/user_id literal, NULL 금지, byte-identical) + audit completeness(create→EVAL_ITEM_CREATED 1·update→EVAL_ITEM_UPDATED 1, resource_id=AUD-1 UUIDv5·details->>'eval_item_id' 검증) + evidences FK 부재 경계(0003 적용 전후 information_schema, evidences 미수정·schema 불변). Quality: coverage≥85%, golangci-lint default+gosec 0, `goleak.VerifyNone` 전체 통과, @MX 태그 plan.md §5 매핑 완료(RED `@MX:TODO` 전부 해소), 기존 Workflow/Evidence/Recorder 특성화 회귀 0, evaluator-active strict per-sprint ≥0.75. | REQ-EVALITEM-UBI-001 (데이터 주권), REQ-EVALITEM-UBI-002 (audit completeness create+update), REQ-EVALITEM-UBI-003 (cli-anonymous 기본값), spec.md §5#6/§7 (evidences FK 부재 경계) | T-009 | [NEW] `apps/control-plane/internal/store/eval_item_test.go` (AC-EVALITEM-UBI-001, AC-EVALITEM-UBI-002, AC-EVALITEM-UBI-003, AC-EVALITEM-BOUNDARY-1); [NEW] `apps/control-plane/internal/audit/recorder_eval_item_test.go` (UBI-001 import 정적 검사); [EXISTING] `apps/control-plane/internal/store/pg_store_test.go`, `apps/control-plane/internal/audit/recorder_test.go` (회귀 0 확인, 미수정) | completed |

## AC → Task Coverage Matrix (22 AC, coverage_verified=true)

| AC | Task | | AC | Task |
|----|------|-|----|------|
| AC-EVALITEM-UBI-001 | T-010 | | AC-EVALITEM-002-1 | T-003(root)/T-005 |
| AC-EVALITEM-UBI-002 | T-007/T-008/T-010 | | AC-EVALITEM-002-2 | T-006 |
| AC-EVALITEM-UBI-003 | T-010 | | AC-EVALITEM-002-3 | T-006 |
| AC-EVALITEM-UBI-004 | T-009 | | AC-EVALITEM-002-4 | T-005 |
| AC-EVALITEM-001-1 | T-003 | | AC-EVALITEM-003-1 | T-007 |
| AC-EVALITEM-001-2 | T-004 | | AC-EVALITEM-003-2 | T-007 |
| AC-EVALITEM-001-S1-1 | T-004 | | AC-EVALITEM-003-E2-1 | T-007 |
| AC-EVALITEM-001-3 | T-001/T-003 | | AC-EVALITEM-003-3 | T-008 |
| AC-EVALITEM-001-4 | T-004 | | AC-EVALITEM-004-1 | T-009 |
| AC-EVALITEM-001-O1-1 | T-003 | | AC-EVALITEM-004-2 | T-009 |
| | | | AC-EVALITEM-004-3 | T-009 |
| | | | AC-EVALITEM-BOUNDARY-1 | T-010 |

## REQ Sub-clause Coverage (coverage_verified=true)

| REQ sub-clause | Task | REQ sub-clause | Task |
|----------------|------|----------------|------|
| REQ-EVALITEM-UBI-001 | T-010 | REQ-EVALITEM-002-E1a | T-003/T-005 |
| REQ-EVALITEM-UBI-002 | T-007/T-008/T-010 | REQ-EVALITEM-002-E1b | T-004/T-005 |
| REQ-EVALITEM-UBI-003 | T-010 | REQ-EVALITEM-002-S1 | T-001/T-006 |
| REQ-EVALITEM-UBI-004 | T-009 | REQ-EVALITEM-002-U1 | T-006 |
| REQ-EVALITEM-001-E1 | T-003/T-004 | REQ-EVALITEM-003-E1 | T-007 |
| REQ-EVALITEM-001-S1 | T-004 | REQ-EVALITEM-003-E2 | T-002/T-007 |
| REQ-EVALITEM-001-O1 | T-003 | REQ-EVALITEM-003-U1 | T-008 |
| REQ-EVALITEM-001-U1 | T-004 | REQ-EVALITEM-004-S1 | T-009 |
| REQ-EVALITEM-001 (모델/Store) | T-001/T-002 | REQ-EVALITEM-004-U1 | T-001/T-009 |
| REQ-EVALITEM-003 (Action/namespace) | T-002 | REQ-EVALITEM-004-O1 | T-009 |

## Dependency Graph

```
T-001 (baseline + 0003 migration, no dep)
  └─> T-002 (interface/const skeleton + EvalItemAuditNamespace)
        ├─> T-003 (root insert happy path)
        │     └─> T-004 (child + parent valid + BeginEvalItemTx MODIFY)
        │           ├─> T-005 (GetEvalItemsByParentID)
        │           │     └─> T-006 (FK RESTRICT + hierarchy_code UNIQUE)
        │           └─> T-007 (AUD-1 recorder; dep: T-002 namespace + T-004 hierarchy_code)
        │                 └─> T-008 (audit atomicity rollback)
        │                       └─> T-009 (immutability + lifecycle)
        │                             └─> T-010 (UBI integration + boundary + quality gate)
```

## Definition of Done (tasks 관점)

- [ ] T-001~T-010 전부 Status=completed (각 TDD RED-GREEN-REFACTOR 완결)
- [ ] 22 AC 자동화 통과 (위 매트릭스)
- [ ] coverage ≥ 85%, golangci-lint default+gosec 0, goleak 전체 통과 (T-010)
- [ ] §6 Option A DDL(자기참조 adjacency list) — T-001, AUD-1 UUIDv5(`uuid.NewSHA1(EvalItemAuditNamespace,...)`) — T-007
- [ ] `BeginEvalItemTx` = `PgWorkflowStore.pool` 재사용(postgres.go 死 스텁 비대상) — T-004
- [ ] `evaluation_items.id`=VARCHAR(64) (information_schema) + `evidences.evaluation_item_id` FK 부재 + evidences 미수정 — T-001/T-010
- [ ] 기존 Workflow/Evidence/Recorder 특성화 회귀 0 — T-001/T-010
- [ ] @MX 태그 plan.md §5 매핑 완료, manager-quality TRUST 5, evaluator-active strict ≥0.75 — T-010
