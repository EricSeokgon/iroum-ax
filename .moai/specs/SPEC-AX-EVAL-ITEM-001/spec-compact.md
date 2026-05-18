# SPEC-AX-EVAL-ITEM-001 — Compact (auto-generated)

> 자동 생성 condensation. spec.md/plan.md/acceptance.md에서 REQ + AC + files-to-modify + Exclusions만 추출. 개요/접근/research 참조 생략. 원본: `spec.md` v0.1.3 (plan-auditor iter1 D1/D2 + evaluator AGREE-PASS LOW-1/LOW-2 + Run Phase 1 Human Gate 3결정 반영 — §6 Option A RESOLVED, AUD-1 deterministic UUIDv5 audit-id RESOLVED, AC-003-1/2 정정 + REQ/AC-003-E2 신규).

## REQ 목록 (EARS)

### Ubiquitous (REQ-EVALITEM-UBI, 4 sub)
- REQ-EVALITEM-UBI-001 (데이터 주권): 생성/조회/수정/검증 경로 외부 서비스 호출 0건, 내부망 단일 pgx pool only.
- REQ-EVALITEM-UBI-002 (감사 가능성): 모든 evaluation item create/update → 동일 TX `audit_logs` 1건.
- REQ-EVALITEM-UBI-003 (cli-anonymous 기본값): AuthN disabled 시 `created_by`/`user_id`='cli-anonymous' literal (NULL 금지).
- REQ-EVALITEM-UBI-004 (계층 불변): 자식 보유 항목의 `parent_id`/`level` 변경 금지 (잎 노드 + 비-계층 컬럼 변경은 허용).

### REQ-EVALITEM-001 — 평가항목 데이터 모델 & Store
- 엔티티 `evaluation_items` — **id VARCHAR(64) PK = 계층 코드 (UUID/auto-inc 아님)**, SPEC-AX-EVID-001 `evidences.evaluation_item_id VARCHAR(64)` stub과 타입 호환 (HARD 계약). parent_id 자기 FK, display_name, level(informational), hierarchy_code UNIQUE, weight, max_score, status, metadata JSONB(opaque).
- E1 (Event): 신규 항목 (id + display_name + parent_id?) → 단일 EvalItemTx INSERT + audit EVAL_ITEM_CREATED 동일 TX → Commit.
- S1 (State): non-NULL parent_id 생성 시 parent 존재 검증, 미존재 시 거부 (orphan 방지). 실패 경로 전용 AC-EVALITEM-001-S1-1 (v0.1.2 LOW-2 — INSERT 시 FK 위반 거부, 행 미생성, audit 미기록; ON DELETE RESTRICT와 별개).
- O1 (Optional): caller가 metadata JSONB payload 제공 시 구조 해석/등급기준 스키마 검증 없이 semantic value-equality로 opaque 영속 (byte 비교 금지 — JSONB 정규화 키 순서/공백/중복키 비교 제외). 전용 AC-EVALITEM-001-O1-1 (v0.1.1 D1, v0.1.2 LOW-1 표현 정정).
- U1 (Unwanted): id blank/64자 초과/display_name blank/중복 PK → 거부, TX 미진입, row 0건, INFO 로그.

### REQ-EVALITEM-002 — 계층 구조 & 자기참조 (Option A adjacency list)
- E1a (Event): parent_id NULL → 루트 노드(L1, parent_id 컬럼 NULL). (v0.1.1 D2: 002-E1 atomic 분할)
- E1b (Event): non-NULL parent_id (기존 parent 참조) → 자기참조 링크 영속, GetEvalItemsByParentID로 자식 조회 가능. AC-EVALITEM-002-1이 E1a/E1b 양 절 검증.
- S1 (State): 자식 보유 행은 parent FK `ON DELETE RESTRICT`로 물리 DELETE 거부 (삭제 API는 미제공, DB 제약만).
- U1 (Unwanted): hierarchy_code 중복 → UNIQUE 위반 거부. 순환 parent 참조는 DB CHECK 불가 → store 검증 + PoC 수동 규율 (깊은 순환 자동 탐지는 범위 밖).

### REQ-EVALITEM-003 — 감사 연계 (AUD-1 RESOLVED)
- **AUD-1 (RESOLVED, plan.md §6.6)**: `audit_logs.resource_id`=`uuid.UUID NOT NULL`(initial.sql:119)이고 `evaluation_items.id`=VARCHAR(64) 계층코드라 직접 저장 불가 → `ResourceID = uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))` 결정적 UUIDv5 surrogate, 실 식별자는 DetailsJSON. `EvalItemAuditNamespace` 고정 상수 = `internal/audit/audit.go`. `initial.sql` 불변(§2.2 HARD), 신규 외부 dep 0(google/uuid 기존).
- E1 (Event): RecordEvalItemCreated/Updated → audit.Event(Action∈{EVAL_ITEM_CREATED,EVAL_ITEM_UPDATED}, ResourceType=evaluation_item, **ResourceID=uuid.NewSHA1(namespace, hierarchyCode) — 원시 id 아님**, UserID=resolveUserID, Details JSONB{eval_item_id,hierarchy_code,parent_id?,level?}) 동일 AuditTx INSERT.
- E2 (Event, v0.1.3 Decision 3 신규): resource_id 파생은 결정적 — 동일 hierarchy_code + 고정 namespace → byte-identical UUIDv5 (재현 가능·충돌 없음, audit 추적성). 전용 AC-EVALITEM-003-E2-1.
- U1 (Unwanted): audit_logs INSERT 실패 → tx.Rollback, evaluation_items row도 rollback (all-or-nothing), goroutine leak 0.

### REQ-EVALITEM-004 — 계층 불변성 & 라이프사이클
- S1 (State): 자식 보유 항목의 parent_id/level UpdateEvalItem 거부 (successor 확인 후 SQL 미실행 — REQ-EVALITEM-UBI-004).
- U1 (Unwanted): status 열거 외/NULL → CHECK + store 검증 거부 (enum {ACTIVE,DEPRECATED,ARCHIVED}).
- O1 (Optional): 잎 노드 비-계층 속성(display_name/description/weight/max_score/status/metadata) 수정 → 단일 TX + EVAL_ITEM_UPDATED audit 1건. 항목 versioning은 audit_logs 추적으로 충분 (별도 버전 테이블 범위 밖).

## RESOLVED DECISIONS (Run Phase 1 Human Gate, plan.md §6/§6.6)

- **Taxonomy 테이블 구조 = Option A (자기참조 adjacency list) — RESOLVED** (strategy.md §1 + Human Gate Decision 1). 가중 A 9.0 ≫ C 6.55 > D 3.40 > B 3.25. B 기각=4테이블 saga audit 원자성 위반+FK-target 모호성, D 기각=node별 PK 부재로 §1.4 VARCHAR(64) HARD 위배, C=재귀 subtree 요구 시 named post-PoC 전환 경로.
- **audit resource_id = AUD-1 deterministic UUIDv5 — RESOLVED** (strategy.md §2 + Human Gate Decision 2). 계층코드 → `uuid.NewSHA1(EvalItemAuditNamespace, hierarchyCode)` surrogate, 실 식별자 DetailsJSON. initial.sql 불변, 신규 외부 dep 0. AC-003-1/2 정정 + REQ/AC-003-E2 신규 (Decision 3).
- **등급기준(scoring rubric) 저장 = open follow-up, 본 SPEC 미설계** — metadata JSONB opaque placeholder만. 구조(JSONB inline vs 별도 테이블) 미결정, 후속 별도 결정 이연.
- **EVID-001 FK 하드닝 타임라인** = 본 SPEC 범위 밖, 미래 별도 SPEC.

## Acceptance Criteria (22)

- §0 UBI: AC-EVALITEM-UBI-001 (외부 호출 0건), -UBI-002 (create/update audit 동일 TX; resource_id=AUD-1 UUIDv5·실 식별자 DetailsJSON — v0.1.3 정정), -UBI-003 (cli-anonymous byte-identical), -UBI-004 (자식 보유 노드 parent_id/level 불변).
- §1 REQ-EVALITEM-001: AC-EVALITEM-001-1 (루트 항목 happy path atomic), -2 (자식 항목 parent 참조 success), -S1-1 (존재하지 않는 parent_id → INSERT FK 위반 거부, 행 미생성, audit 미기록 — v0.1.2 LOW-2 S1 실패경로 전용 AC), -3 (id VARCHAR(64) EVID-001 FK 타입 호환), -4 (id/display_name 검증 + 중복 PK 거부), -O1-1 (metadata JSONB opaque semantic-equality 영속, byte 비교 금지·등급기준 미해석 — v0.1.1 D1 전용 AC + v0.1.2 LOW-1 표현 정정).
- §2 REQ-EVALITEM-002: AC-EVALITEM-002-1 (root parent_id NULL + 자식 조회, E1a/E1b 분할 절 모두 검증), -2 (parent FK ON DELETE RESTRICT, S1 실패경로와 별개), -3 (hierarchy_code UNIQUE), -4 (다단계 계층 순회).
- §3 REQ-EVALITEM-003: AC-EVALITEM-003-1 (RecordEvalItemCreated row — resource_id=AUD-1 결정적 UUIDv5, 실 식별자 details->>'eval_item_id'/'hierarchy_code'; v0.1.3 원시 계층코드 false RED 정정), -2 (RecordEvalItemUpdated row — AUD-1 동일), -E2-1 (resource_id 결정성: 동일 hierarchy_code → 동일 UUID, 재현·충돌없음 — v0.1.3 Decision 3 신규 AC), -3 (audit fail → 양방향 rollback).
- §4 REQ-EVALITEM-004: AC-EVALITEM-004-1 (자식 보유 parent_id/level 변경 거부), -2 (status 전이 + 열거외/NULL 거부), -3 (잎 노드 비-계층 속성 수정 + audit; metadata O1 1:1 coverage는 §1 AC-EVALITEM-001-O1-1로 이관).
- §5 Boundary: AC-EVALITEM-BOUNDARY-1 (evidences.evaluation_item_id FK 부재 유지, evidences 미수정 — out-of-scope 경계 확인).

Total AC: 22 (§0:4, §1:6, §2:4, §3:4, §4:3, §5 boundary:1). 각 modal REQ ≥ 3 AC (§1은 6개, §2/§3은 4개). 버전 이력: v0.1.1 D1 AC-EVALITEM-001-O1-1 추가 (19→20). v0.1.2 LOW-2 AC-EVALITEM-001-S1-1 추가 (20→21) + LOW-1 O1-1 표현 semantic JSONB 정정. v0.1.3 Run Phase 1 Decision 3 AC-EVALITEM-003-E2-1 추가 (REQ-EVALITEM-003-E2 AUD-1 resource_id 결정성, 21→22) + AC-003-1/2·UBI-002 텍스트를 AUD-1 deterministic UUIDv5 기준 정정(원시 계층코드 resource_id false RED 제거).

## Files to Modify

| 경로 | Delta |
|------|-------|
| `internal/store/store.go` | [MODIFY] EvalItemStore/EvalItemTx 인터페이스 추가 (WorkflowStore/EvidenceStore 패턴) |
| `internal/store/eval_item.go` | [NEW] EvalItemTx pgx 구현 (InsertEvalItem/GetEvalItemByID/GetEvalItemsByParentID/UpdateEvalItem) |
| `internal/store/pg_store.go` | [MODIFY] 실 `PgWorkflowStore.pool` 재사용한 `BeginEvalItemTx` 진입점 추가 (신규 풀 금지, BeginEvidenceTx 선례). **`postgres.go`는 死 스텁 비대상** — research.md §1 phantom-path 회피 |
| `internal/audit/audit.go` | [MODIFY] ActionEvalItemCreated/ActionEvalItemUpdated 상수 + `EvalItemAuditNamespace` 고정 UUID 상수 (AUD-1 UUIDv5 namespace) |
| `internal/audit/recorder.go` | [MODIFY] RecordEvalItemCreated/RecordEvalItemUpdated 메서드 (로컬 AuditTx 유지, RecordEvidenceCreated 선례; ResourceID=`uuid.NewSHA1(EvalItemAuditNamespace, hierarchyCode)` AUD-1, 실 식별자 DetailsJSON) |
| `.moai/db/schema/migrations/0003_eval_item_tables.sql` | [NEW] evaluation_items 테이블(id VARCHAR(64) PK, parent 자기 FK ON DELETE RESTRICT, status CHECK, hierarchy_code UNIQUE, 인덱스) 멱등 SQL. 0001/0002 비충돌 확인 |
| `.moai/db/schema/initial.sql` | [EXISTING] 미수정 (schema drift 방지) |
| `internal/store/eval_item_test.go`, `internal/audit/recorder_eval_item_test.go` | [NEW] |
| `internal/store/pg_store_test.go` | [EXISTING] WorkflowStore/EvidenceStore 특성화 회귀 확인 |

## Exclusions (What NOT to Build)

1. 평가편람(평가지침) HWP/PDF import/파싱 연계 — 본 SPEC은 항목 행 수용 데이터 모델만. [Deferred]
2. 등급기준(scoring rubric) 저장 설계 — metadata JSONB opaque placeholder만, 구조 미설계 (open follow-up).
3. 다중 테넌시 / 조직 격리 (org_id) — SPEC-AX-AUTH 계열/미래 platform SPEC.
4. 항목 버전 관리(versioning) — audit_logs 추적으로 충분, 스냅샷 테이블/PITR 제외.
5. 항목 CRUD REST API / 삭제 API / Console UI — store 계층 + audit만 (삭제는 DB RESTRICT 제약만, 경로 미구현).
6. **`evidences.evaluation_item_id` FK 하드닝 (SPEC-AX-EVID-001 코드 변경 포함)** — 본 SPEC은 type-compatible(VARCHAR(64)) `evaluation_items` provider만. FK 소급 추가·EVID-001 코드/마이그레이션 변경은 미래 별도 SPEC (AC-EVALITEM-BOUNDARY-1 경계 확인).
7. 계층 재배치 / 트리 이동 (re-parenting) — 자식 보유 항목 parent_id/level 변경 금지 (REQ-EVALITEM-UBI-004).
8. closure table / JSONB nested / 다단계 정규화 (research.md §5 Option B/C/D) — Option A 확정(plan.md §6 RESOLVED). B·D 기각, C는 named post-PoC 전환 경로(본 SPEC 미구현).
9. 마이그레이션 도구 통합 (alembic/golang-migrate) — 수동 멱등 SQL only.

## Resolved / Deferred

- HARD 계약: `evaluation_items.id` = `VARCHAR(64)` 계층 코드 (UUID/auto-increment 아님) — SPEC-AX-EVID-001 `evidences.evaluation_item_id VARCHAR(64)` FK-제약-없는 stub과 타입 호환 (미래 FK 승격 대비). research.md §2, spec.md §1.4.
- RESOLVED (plan.md §6, Run Phase 1 strategy + Human Gate Decision 1): taxonomy 구조 = Option A (자기참조 adjacency list). 가중 A 9.0 ≫ C 6.55 > D 3.40 > B 3.25. B·D 기각, C = named post-PoC 전환 경로.
- RESOLVED (plan.md §6.6, Run Phase 1 strategy + Human Gate Decision 2): audit resource_id = AUD-1 deterministic UUIDv5 surrogate(`uuid.NewSHA1(EvalItemAuditNamespace, hierarchyCode)`), 실 식별자 DetailsJSON. initial.sql 불변(§2.2 HARD), 신규 외부 dep 0(google/uuid 기존). REQ/AC-003-E2 결정성 검증 신규(Decision 3).
- 미설계 (open follow-up): 등급기준 저장 구조 (metadata JSONB vs 별도 테이블) — 후속 별도 결정.
- 범위 밖: evidences FK 하드닝 + EVID-001 코드 변경 = 미래 별도 SPEC. 본 SPEC은 `evidences`/EVID-001 코드 미수정.
- Phantom-path 회피: eval-item TX 진입점 = `pg_store.go`(실 `PgWorkflowStore.pool`, BeginEvidenceTx 선례), NOT `postgres.go`(死 스텁).
- 마이그레이션: `0003_eval_item_tables.sql` (0001_initial + 0002_evidence_tables 비충돌, research.md §4). initial.sql 미수정.
