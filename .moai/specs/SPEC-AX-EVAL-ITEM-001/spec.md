---
id: SPEC-AX-EVAL-ITEM-001
version: 0.1.3
status: draft
created: 2026-05-18
updated: 2026-05-18
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.3 (2026-05-18): Run Phase 1 전략 Human Gate 3개 결정 승인 반영 (manager-strategy strategy.md §1/§2 분석 + 사용자 sign-off). 결정 1(§6 taxonomy 구조 = **Option A RESOLVED** — plan.md §6 "OPEN/CONFIRMABLE" → "RESOLVED: Option A 자기참조 adjacency list", SPEC-AX-EVID-001 §6 RESOLVED 동위상. 가중 A 9.0 ≫ C 6.55 > D 3.40 > B 3.25; B 기각=4테이블 saga audit 원자성 위반+`evidences.evaluation_item_id` FK-target 모호성, D 기각=node별 PK 부재로 §1.4 VARCHAR(64) HARD 계약 직접 위배, C=재귀 subtree 요구 시 named post-PoC 전환 경로. spec.md §1.4/§3.3/§5#8/§6 OPEN 표현을 확정 표현으로 정정). 결정 2(audit `resource_id` = **AUD-1 deterministic UUIDv5 RESOLVED** — spec.md §6 line ~209 "run 단계 이연" 항목 확정. 검증된 모순: `audit.Event.ResourceID`=`uuid.UUID`(audit.go:72), `audit_logs.resource_id`=`UUID NOT NULL`(initial.sql:119), `parseResourceID` 비-UUID 시 `uuid.Nil`(recorder.go:90-94), `evaluation_items.id`=VARCHAR(64) 계층코드 직접 저장 불가. 해결: `RecordEvalItem*`가 계층코드를 `uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))` 결정적 UUIDv5로 변환해 `Event.ResourceID`에 저장, 실 식별자(`eval_item_id`/`hierarchy_code`/`parent_id`/`level`)는 `DetailsJSON`. `EvalItemAuditNamespace` 고정 상수 UUID는 `internal/audit/audit.go`에 정의. `initial.sql` 불변(§2.2 HARD), 신규 외부 dep 0(`google/uuid` recorder.go:91 기존 import), Cross-SPEC 무영향. REQ-EVALITEM-003-E1 재작성 + REQ-EVALITEM-003-E2(결정성) 신규. spec.md §3.4/§6.6(plan)). 결정 3(AC-EVALITEM-003-1/003-2 텍스트 정정 — `resource_id='AX-SAFETY-ORG-01'` 원시 계층코드 단언은 `uuid.UUID` 컬럼 보유 불가 false RED → AUD-1 기준 재작성: `resource_id`=결정적 UUIDv5, 실 계층코드는 `details->>'hierarchy_code'`/`details->>'eval_item_id'`로 검증, resource_id 결정성 검증 절 추가. REQ-EVALITEM-003-E2 신규로 §3 13개→14개 modal sub-clause, AC 카운트 21→22(AC-EVALITEM-003-E2-1 추가). §7/§8/§9·spec-compact.md 일관 갱신). (작성자: ircp)
- 0.1.2 (2026-05-18): evaluator-active 교차검증 AGREE-PASS 반영 (plan-auditor PASS 0.955 확인, Run 진입 전 LOW 2건 정정). LOW-1(acceptance.md AC-EVALITEM-001-O1-1 Then 표현 정정 — "byte-identical(verbatim) JSONB / 키 재정렬 0건"은 PostgreSQL JSONB 키 순서·공백·중복키 정규화로 올바른 구현에서도 달성 불가 → false RED 유발. REQ-EVALITEM-001-O1이 저장 타입을 "opaque JSONB column"으로 명시했으므로 "semantically equivalent JSONB (필드 누락/값 변형 0건, PostgreSQL JSONB value-equality 기준 — 키 순서/공백/중복키는 JSONB 정규화 범위로 비교 제외)"로 정정. 테스트 구현이 byte-level string 비교가 아닌 semantic JSON equality(map unmarshal + DeepEqual / jsonEqual())를 사용해야 함을 AC 비고에 명시. §8 RED/GREEN 매핑 동기화). LOW-2(REQ-EVALITEM-001-S1 실패 경로 coverage 보강 — AC-EVALITEM-001-2는 success 경로만 검증했으므로, 존재하지 않는 parent_id로 항목 생성 시 INSERT 시점 FK 위반 거부·행 미생성·audit 미기록(orphan 방지) 전용 AC-EVALITEM-001-S1-1을 §1에 신규 추가. ON DELETE RESTRICT(AC-EVALITEM-002-2)와 별개 경로임을 명시. §7 Edge Case Catalog/§8 RED·GREEN/§9 DoD AC 카운트 일관 갱신 20→21, edge case 15→16, spec-compact.md 동기화). evaluator 보고서 파일 경로(`.moai/reports/evaluator/SPEC-AX-EVAL-ITEM-001-xvalidate-1.md`)는 디스크 미존재였으나 LOW-1/LOW-2 결함이 현행 SPEC 텍스트에 대해 독립 검증되어 정정 수행. (작성자: ircp)
- 0.1.1 (2026-05-18): plan-auditor iter1 리뷰 반영 (PASS 0.955, minor 결함 2건 정정 + 1건 수용). D1(REQ-EVALITEM-001-O1 coverage illusion 해소 — acceptance.md §1에 전용 AC-EVALITEM-001-O1-1 신규 추가, 기존 AC-EVALITEM-004-3 내부 간접 검증을 update-경로 보조 검증으로 강등하고 O1 1:1 coverage를 §1 전용 AC로 이관, §7 Edge Case Catalog/§8 RED·GREEN/§9 DoD AC 카운트 일관 갱신 19→20, spec-compact.md 동기화 — SPEC-AX-EVID-001 v0.1.2 D1 정정 AC-EVID-001-O1-1 추가 패턴과 동일). D2(spec.md §3.3 REQ-EVALITEM-002-E1 복합 event-driven 문장을 1트리거-1응답 atomic 절 REQ-EVALITEM-002-E1a(root, parent_id NULL) / REQ-EVALITEM-002-E1b(child, non-NULL parent_id)로 분할 — EARS 의미·검증 범위 불변, AC-EVALITEM-002-1이 양 절 검증, REQ modal 모듈 수 ≤5 유지(002 모듈 내 sub-clause 분할이며 신규 modal 모듈 미추가)). D3(research.md provenance 부정확 — §1 EvalItemTx 열거 UpdateEvalItem 누락, §6 EVID-001 v0.1.0 인용)은 Phase 0.5 frozen 시점 지원 산출물로 비채점 수용 — 4종 SPEC 문서가 EVID-001 v0.1.2 기준 + UpdateEvalItem 포함으로 내부 정합하므로 research.md 미수정. (작성자: ircp)
- 0.1.0 (2026-05-18): 경영평가 평가항목 taxonomy(Evaluation Item Taxonomy) 첫 초안. iroum-ax Go control-plane을 brownfield 확장하여 평가범주 → 평가항목 → 평가지표 → 배점·가중치·등급기준 4계층 taxonomy의 **기초 데이터 모델 + store 계층 + audit 연계 Walking Skeleton**을 정의. 자기참조 adjacency-list 단일 테이블(`evaluation_items`, `parent_id` 자기 FK), `id`는 **계층 코드 형태 VARCHAR(64)** (auto-increment/UUID 아님) — SPEC-AX-EVID-001의 `evidences.evaluation_item_id VARCHAR(64)` FK-제약-없는 stub과 **타입 호환** 필수(미래 FK 승격 대비). SPEC-AX-CTRL-001의 `WorkflowStore`/`WorkflowTx`/`Recorder`/`AuditTx` 패턴(GREEN 가정), SPEC-AX-EVID-001의 `EvidenceStore`/`EvidenceTx` 미러링 선례(완료, v0.1.2) 위에 `EvalItemStore`/`EvalItemTx`/`RecordEvalItem*`를 동일 패턴으로 추가. 평가편람(HWP/PDF) import·파싱, 등급기준(scoring rubric) 저장 설계, 항목 CRUD/REST API, Console UI, `evidences` FK 하드닝(EVID-001 코드 변경 포함)은 의도적 제외(후속 SPEC). (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-EVID-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등은 canonical schema에 존재하지 않으므로 사용하지 않는다. 본 SPEC의 모든 EARS 요구사항·DDL·영향파일은 `.moai/specs/SPEC-AX-EVAL-ITEM-001/research.md`(Phase 0.5 deep research, file:line 근거)에 근거하며, `evaluation_items.id`의 **VARCHAR(64) 계층 코드** 타입 결정은 `0002_evidence_tables.sql:8`의 `evaluation_item_id VARCHAR(64)` stub 계약(research.md §2, SPEC-AX-EVID-001/spec.md:50, SPEC-AX-EVID-001/acceptance.md AC-EVID-001-3 검증 중)에서 도출된 load-bearing 제약이다. 신규 마이그레이션 파일번호 `0003_eval_item_tables.sql`은 `migrations/`에 실재하는 `0001_initial.sql` + SPEC-AX-EVID-001의 `0002_evidence_tables.sql`과 비충돌(research.md §4)임을 확인했다.

---

# SPEC-AX-EVAL-ITEM-001 — 경영평가 평가항목 taxonomy (Evaluation Item Taxonomy)

## 1. 개요

경영평가팀이 평가범주·평가항목·평가지표·배점/가중치/등급기준의 **4계층 평가항목 taxonomy를 정의·계층 구성·감사 추적**할 수 있도록, `apps/control-plane/`(Go 1.22+)에 평가항목 데이터 모델·영속 계층·감사 연계를 추가하는 Walking Skeleton을 정의한다. 본 SPEC은 SPEC-AX-CTRL-001의 워크플로우 오케스트레이션 계층과 SPEC-AX-EVID-001의 증빙 store/audit 확장 선례 위에, **자기참조 계층 구조(adjacency list)를 단일 트랜잭션 내에서 생성하고 모든 변경을 `audit_logs`에 원자적으로 기록하며 계층 불변식을 store 계층에서 강제하는** 최소 실행 가능한 평가항목 taxonomy 계층을 제공한다.

### 1.1 Walking Skeleton의 의미 (본 SPEC 범위)

본 SPEC의 Walking Skeleton은 **기초 데이터 모델 + store 계층 + audit 연계**에 집중한다. 평가편람(HWP/PDF) 일괄 import·파싱, 등급기준 규칙 저장 설계, 항목 CRUD REST API, Console 화면은 본 SPEC 범위가 아니다.

- 단일 엔티티: `evaluation_items` 테이블 (자기참조 `parent_id` adjacency list + 계층 코드 `id VARCHAR(64)` + `hierarchy_code` 경로 인코딩 + `level` informational)
- 단일 store 추상화: `EvalItemStore` / `EvalItemTx` 인터페이스 (SPEC-AX-CTRL-001 `WorkflowStore`/`WorkflowTx`, SPEC-AX-EVID-001 `EvidenceStore`/`EvidenceTx` 패턴 그대로 — 동일 pgx pool 재사용, 신규 풀 생성 없음)
- 단일 감사 연계: 기존 `internal/audit` Recorder에 `RecordEvalItemCreated` / `RecordEvalItemUpdated` 확장 (기존 `RecordCreated`/`RecordEvidenceCreated`와 동일 시그니처 패턴, 동일 트랜잭션 내 atomic 기록)
- 단일 생성/수정 경로: 평가항목 생성·수정 store 메서드 (BeginEvalItemTx → 계층 검증 → InsertEvalItem/UpdateEvalItem → Recorder 기록 → Commit)
- 인증 없음: 모든 호출은 `created_by="cli-anonymous"` (SPEC-AX-001 REQ-UBI-003, SPEC-AX-CTRL-001 REQ-CTRL-UBI-002, SPEC-AX-EVID-001 REQ-EVID-UBI-003과 정합)

### 1.2 Anchor 컨텍스트

본 SPEC은 `product.md` §3.2의 기획재정부 경영평가 편람("안전 보건" 평가기준·배점)을 시스템 내부에 **구조화된 평가 기준 계층**으로 표현하는 기반을 형성한다. `product.md:160` "계층: 항목 → 지표 → 배점"을 실세계 4계층(평가범주 Level1 → 평가항목 Level2 → 평가지표 Level3 → 배점·가중치·등급기준 Level4)으로 모델링한다. PoC 범위는 "안전보건" 범주 전체이며, SPEC-AX-CTRL-001의 5개 REQ-CTRL과 2개 REQ-CTRL-UBI, SPEC-AX-EVID-001의 store/audit 확장은 GREEN 상태로 가정한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `EVAL-ITEM` (Evaluation Item Taxonomy sub-domain)
- 따라서 SPEC ID: `SPEC-AX-EVAL-ITEM-001` (2 domains — `AX` + `EVAL-ITEM`, `.claude/skills/moai/workflows/plan.md` Composite domain rules "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내)

### 1.4 EVID-001 stub 계약 — PK 타입 [핵심, load-bearing]

SPEC-AX-EVID-001은 `evidences.evaluation_item_id`를 **FK 제약 없는 plain `VARCHAR(64)` stub**으로 정의하고 `evaluation_items` 테이블을 생성하지 않은 채(`0002_evidence_tables.sql:8` `-- 경량 FK stub, 제약 없음`, SPEC-AX-EVID-001/spec.md:50, AC-EVID-001-3로 FK 부재 동작 검증 중) 평가항목 taxonomy를 본 SPEC으로 명시적으로 이연했다(SPEC-AX-EVID-001 §5 Exclusion #1, §7 Out of Scope, plan.md §8 downstream 추적).

본 SPEC은 그 `evaluation_items` 테이블의 **provider**다. 따라서 다음이 **HARD 계약**이다:

- **[HARD]** `evaluation_items.id`는 **`VARCHAR(64)`** 이며 **UUID도 auto-increment도 아니다**. 식별자는 계층 의미를 담는 코드(예: `AX-SAFETY-ORG-01`(항목), `AX-SAFETY-ORG-01-1`(지표))이다. 이는 미래 FK `evidences.evaluation_item_id → evaluation_items(id)`가 타입 호환(VARCHAR(64) ↔ VARCHAR(64))되도록 보장하기 위한 결정이며, research.md §2의 핵심 계약이다.
- **[HARD]** SPEC-AX-EVID-001 코드 변경, 그리고 `evidences.evaluation_item_id`에 FK 제약을 소급(retroactive) 추가하는 작업은 **본 SPEC의 범위가 아니다**. 본 SPEC은 type-compatible한 `evaluation_items` 테이블을 제공하기만 하며, 실제 FK 하드닝은 미래 별도 SPEC이 수행한다(§5 Exclusions #6, §7 Out of Scope 참조).

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 `apps/control-plane/` 트리를 따른다. 본 SPEC은 stub이 아닌 **실제 구현이 존재하는 코드를 brownfield 확장**하므로 Delta 마커를 적용한다 ([EXISTING]=특성화 테스트로 보존, [NEW]=신규 추가, [MODIFY]=기존 파일 수정).

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/internal/store/store.go` | `EvalItemStore` / `EvalItemTx` 인터페이스 추가 (기존 `WorkflowStore`/`WorkflowTx`, `EvidenceStore`/`EvidenceTx` 패턴 준수) | [MODIFY] | REQ-EVALITEM-001 |
| `apps/control-plane/internal/store/eval_item.go` | `EvalItemTx` 메서드 (InsertEvalItem, GetEvalItemByID, GetEvalItemsByParentID, UpdateEvalItem, InsertAuditLog, Commit, Rollback) pgx 구현 | [NEW] | REQ-EVALITEM-001, REQ-EVALITEM-002 |
| `apps/control-plane/internal/store/pg_store.go` | 실 pgx pool(`PgWorkflowStore{pool *pgxpool.Pool}`, `server.go` `store.NewPgWorkflowStore(...)` 와이어링, `BeginEvidenceTx` 선례)에 `BeginEvalItemTx` 진입점 추가 (신규 풀 금지, `PgWorkflowStore.pool` 단일 재사용). **주의: `postgres.go`는 Sprint-0 死 스텁(`New(cfg)` + TODO, 실 pool 없음)이며 본 SPEC 대상 아님** — research.md §1 phantom-path 회피 | [MODIFY] | REQ-EVALITEM-001 |
| `apps/control-plane/internal/audit/audit.go` | 신규 액션 상수 `ActionEvalItemCreated`, `ActionEvalItemUpdated` 추가 (기존 `Action string`, `ActionEvidenceCreated` 패턴) | [MODIFY] | REQ-EVALITEM-003 |
| `apps/control-plane/internal/audit/recorder.go` | `RecordEvalItemCreated`, `RecordEvalItemUpdated` 메서드 추가 (기존 `RecordCreated`/`RecordEvidenceCreated` 시그니처 패턴, 로컬 `AuditTx` 인터페이스 유지 — store→audit 순환 의존 회피) | [MODIFY] | REQ-EVALITEM-003 |

### 2.2 Database (`.moai/db/schema/`)

| 경로 | 책임 | Delta |
|------|------|-------|
| `.moai/db/schema/initial.sql` | **참조 only**: 본 SPEC은 initial.sql을 수정하지 않는다 (기존 documents/audit_logs 테이블 schema drift 방지, SPEC-AX-EVID-001 §2.2 동일 정책). | [EXISTING] |
| `.moai/db/schema/migrations/0003_eval_item_tables.sql` | **신규**: `evaluation_items` 테이블 + 인덱스 + status CHECK 추가. 멱등성 패턴(`CREATE TABLE IF NOT EXISTS`, `DO $$ ... EXCEPTION ...`, `CREATE INDEX IF NOT EXISTS`) 유지, 수동 SQL 규약 (마이그레이션 도구 미사용). 파일번호 `0003`은 `0001_initial.sql` + SPEC-AX-EVID-001 `0002_evidence_tables.sql`과 비충돌 확인(research.md §4). | [NEW] |

### 2.3 Tests (`apps/control-plane/`)

| 경로 | 책임 | Delta |
|------|------|-------|
| `apps/control-plane/internal/store/eval_item_test.go` | testcontainers-go(postgres) 기반 InsertEvalItem + 자기참조 계층 + parent FK RESTRICT + hierarchy_code uniqueness | [NEW] |
| `apps/control-plane/internal/audit/recorder_eval_item_test.go` | RecordEvalItemCreated/Updated audit row 검증 + audit fail 시 양방향 rollback | [NEW] |
| `apps/control-plane/internal/store/pg_store_test.go` | 기존 WorkflowStore/EvidenceStore 특성화 테스트 보존 확인 (eval-item 와이어링 후 회귀 없음) | [EXISTING] |

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건)

Ubiquitous 요구사항은 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-EVID-001의 canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track 패턴을 도메인 스코프 형태(`REQ-EVALITEM-UBI-NNN`)로 적용한다 (SPEC-AX-EVID-001 `REQ-EVID-UBI-*`와 동일한 규약, research.md §7).

- **REQ-EVALITEM-UBI-001 (데이터 주권)**: The evaluation item taxonomy subsystem SHALL NOT make any external service call (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) for creating, reading, updating, or validating evaluation item rows or their hierarchy. 모든 영속 경로는 고객사 내부망 자원(SPEC-AX-CTRL-001이 확립한 단일 PostgreSQL pgx pool)에만 의존한다 (`tech.md` §9.1 망분리 정합).
- **REQ-EVALITEM-UBI-002 (감사 가능성)**: The subsystem SHALL write exactly one `audit_logs` entry within the same database transaction as every evaluation item create and every evaluation item update event, reusing the `audit_logs` schema defined in SPEC-AX-001 REQ-UBI-003 (SPEC-AX-EVID-001 REQ-EVID-UBI-002와 동일 규약). 트랜잭션 외부에서 발생한 평가항목 변경은 audit 불가능하므로 금지된다.
- **REQ-EVALITEM-UBI-003 (cli-anonymous 기본값)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the subsystem SHALL persist `evaluation_items.created_by = 'cli-anonymous'` and the corresponding `audit_logs.user_id = 'cli-anonymous'` (정확히 literal 문자열, NULL 금지), reusing `audit.DefaultUserID` / `Recorder.resolveUserID` 계약 (실 사용자 식별자 누출 금지).
- **REQ-EVALITEM-UBI-004 (계층 불변)**: The subsystem SHALL NOT change the `parent_id` or `level` column of any evaluation item row that already has at least one child (a row whose `parent_id` references it). 자식이 존재하는 항목의 계층 위치는 불변이며, 계층 재배치는 본 SPEC 범위 밖이다. 잎(leaf) 노드의 `parent_id`/`level` 변경 및 모든 노드의 비-계층 컬럼(`display_name`, `description`, `weight`, `max_score`, `status`, `metadata`) 변경은 허용된다.

### 3.2 REQ-EVALITEM-001 — 평가항목 데이터 모델 & Store 계층

**엔티티**: `evaluation_items` (id VARCHAR(64) PK — **계층 코드, UUID 아님**, parent_id VARCHAR(64) 자기 참조 — FK ON DELETE RESTRICT, display_name VARCHAR(256), description TEXT, level INT — 1범주/2항목/3지표/4배점 informational, hierarchy_code VARCHAR(128) UNIQUE — 경로 인코딩, weight DECIMAL(5,4) nullable, max_score INT nullable, status VARCHAR(32) DEFAULT 'ACTIVE' — CHECK, metadata JSONB, created_at, created_by DEFAULT 'cli-anonymous', updated_at, archived_at). `id`의 VARCHAR(64) 타입은 SPEC-AX-EVID-001 `evidences.evaluation_item_id VARCHAR(64)` stub과 타입 호환을 위한 HARD 계약이다(§1.4). 구체 DDL은 `plan.md` §3 참조.

**Store 추상화**: `EvalItemStore.BeginEvalItemTx(ctx) (EvalItemTx, error)` + `EvalItemTx{InsertEvalItem, GetEvalItemByID, GetEvalItemsByParentID, UpdateEvalItem, InsertAuditLog, Commit, Rollback}` — 기존 `WorkflowStore`/`WorkflowTx`, `EvidenceStore`/`EvidenceTx` 인터페이스 설계를 그대로 미러링하며 동일 pgx pool을 재사용한다.

#### Event-driven

- **REQ-EVALITEM-001-E1**: WHEN a caller submits a new evaluation item with a non-blank hierarchical `id`, a `display_name`, and (optionally) a `parent_id`, THEN the subsystem SHALL open a single `EvalItemTx`, INSERT one `evaluation_items` row, INSERT one corresponding `audit_logs` row with action `EVAL_ITEM_CREATED` in the same transaction, Commit atomically, and return the created item `id`.

#### State-driven

- **REQ-EVALITEM-001-S1**: WHILE an evaluation item create transaction with a non-NULL `parent_id` is in progress, the subsystem SHALL verify the referenced parent row exists (parent FK 제약 + store 계층 사전 조회). 존재하지 않는 `parent_id`를 참조하는 생성은 트랜잭션 커밋 없이 거부된다 (orphan 노드 방지).

#### Optional

- **REQ-EVALITEM-001-O1**: WHERE a caller supplies a `metadata` JSONB payload (예: 등급기준 초안 등의 부가 정보), the subsystem SHALL persist it verbatim as an opaque JSONB column without interpreting its structure. metadata의 스키마·검증·등급기준 규칙 해석은 본 SPEC 범위가 아니며 향후 별도 결정으로 이연한다 (§5 Exclusions #2).

#### Unwanted

- **REQ-EVALITEM-001-U1**: IF the incoming `id` is blank/empty OR exceeds 64 characters OR `display_name` is blank OR a row with the same `id` already exists, THEN the subsystem SHALL reject the request with a structured error, SHALL NOT open a transaction (중복 id 충돌은 INSERT 시 PK 위반으로 거부), SHALL NOT INSERT any `evaluation_items` or `audit_logs` row, and SHALL surface the validation error to the caller (client error, not a server defect).

### 3.3 REQ-EVALITEM-002 — 계층 구조 & 자기참조 (Hierarchy & Adjacency List)

본 SPEC은 **Option A(단일 자기참조 adjacency-list 테이블)를 확정 채택**한다 (Run Phase 1 strategy.md §1 + Human Gate Decision Point 1 승인 — 가중 A 9.0 ≫ C 6.55 > D 3.40 > B 3.25, plan.md §6 RESOLVED). C(closure table)는 재귀 subtree 요구 발생 시의 named post-PoC 전환 경로이다.

#### Event-driven

- **REQ-EVALITEM-002-E1a**: WHEN an evaluation item is created with `parent_id = NULL`, THEN the subsystem SHALL persist it as a root node (최상위 평가범주, Level 1) whose `parent_id` column is NULL.
- **REQ-EVALITEM-002-E1b**: WHEN an evaluation item is created with a non-NULL `parent_id` referencing an existing parent, THEN the subsystem SHALL persist the self-referential link such that `GetEvalItemsByParentID(parent_id)` returns the new row among that parent's children.

> (D2 정정, v0.1.1: 기존 단일 복합 REQ-EVALITEM-002-E1을 1문장 1행동 추적성 향상을 위해 atomic하게 E1a(root, parent_id NULL) / E1b(child, non-NULL parent_id)로 분할. EARS 의미·검증 범위 불변, REQ 모듈 수 ≤5 유지(002 모듈 내 sub-clause 분할이며 신규 modal 모듈 추가 아님). AC-EVALITEM-002-1이 양 절을 모두 검증.)

#### State-driven

- **REQ-EVALITEM-002-S1**: WHILE any evaluation item row has at least one child, the subsystem SHALL enforce the `evaluation_items_parent_id_fkey` foreign key with `ON DELETE RESTRICT` so that a physical DELETE of a row with existing children is rejected by the database (단절된 orphan 하위 계층 방지). 본 SPEC은 삭제 API를 제공하지 않으나(§5 Exclusions #5), DB 제약은 미래 삭제 경로에 대한 구조적 보호로서 정의된다.

#### Unwanted

- **REQ-EVALITEM-002-U1**: IF a caller attempts to create an evaluation item with a `hierarchy_code` that already exists for another row, THEN the subsystem SHALL reject the INSERT (`evaluation_items_hierarchy_code_idx` UNIQUE 제약 위반), SHALL NOT commit the transaction, and SHALL surface the uniqueness conflict error (계층 경로 중복 방지). 순환 parent 참조(자기 자신 또는 조상을 parent로 지정)는 DB CHECK로 강제 불가능하므로 본 SPEC에서는 store 계층 애플리케이션 검증 + PoC 수동 규율로 처리하며, 깊은 순환 탐지 자동화는 본 SPEC 범위 밖이다 (research.md §8 R-EVALITEM-004).

### 3.4 REQ-EVALITEM-003 — 감사 연계 (Audit Recorder 확장)

기존 `internal/audit` Recorder 패턴을 확장한다. 새 액션 상수 `ActionEvalItemCreated Action = "EVAL_ITEM_CREATED"`, `ActionEvalItemUpdated Action = "EVAL_ITEM_UPDATED"`를 추가하고, `Recorder`에 `RecordEvalItemCreated`/`RecordEvalItemUpdated` 메서드를 추가한다 (기존 `RecordCreated(ctx, tx AuditTx, ...)` / `RecordEvidenceCreated` 시그니처 패턴, 로컬 `AuditTx` 인터페이스 유지 — store→audit 순환 의존 회피).

> **Audit `resource_id` 전략 — RESOLVED: AUD-1 (deterministic UUIDv5 surrogate)** (Run Phase 1 strategy.md §2 + Human Gate Decision Point 2 승인, plan.md §6.6). 모순: `audit.Event.ResourceID`는 `uuid.UUID`(`audit.go:72`), `audit_logs.resource_id`는 `UUID NOT NULL`(`initial.sql:119`), `parseResourceID`는 비-UUID 입력 시 `uuid.Nil` 반환(`recorder.go:90-94`)이나 `evaluation_items.id`는 VARCHAR(64) 계층 코드라 직접 저장 불가. 해결: `RecordEvalItem*`는 계층 코드를 **결정적 UUIDv5**(`uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))` — `github.com/google/uuid` 기존 import `recorder.go:91`, 신규 외부 의존 0건, 데이터 주권 REQ-EVALITEM-UBI-001 정합)로 변환해 `Event.ResourceID`에 저장하고, 실제 식별자는 `DetailsJSON`에 기록한다. `EvalItemAuditNamespace`는 고정 상수 UUID 1개를 `internal/audit/audit.go`에 정의한다. `initial.sql`은 수정하지 않는다(§2.2 [EXISTING] HARD — schema drift 방지, Cross-SPEC 무영향). `evaluation_items.id`는 VARCHAR(64) 계층 코드를 유지하며 UUIDv5는 audit `resource_id` surrogate에만 한정된다(§1.4 HARD 계약 불변).

#### Event-driven

- **REQ-EVALITEM-003-E1**: WHEN an evaluation item create or update transaction calls `Recorder.RecordEvalItemCreated` or `Recorder.RecordEvalItemUpdated` with the active `AuditTx`, THEN the Recorder SHALL construct an `audit.Event` with `Action` ∈ {`EVAL_ITEM_CREATED`, `EVAL_ITEM_UPDATED`}, `ResourceType="evaluation_item"`, `ResourceID` = the **deterministic UUIDv5** derived as `uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))` (NOT the raw VARCHAR(64) `id` — `resource_id` 컬럼이 `uuid.UUID NOT NULL`이므로), `UserID`=`resolveUserID(userID)`, and `DetailsJSON` containing `{eval_item_id, hierarchy_code, parent_id?, level?}` (실제 계층 식별자는 `DetailsJSON`에 보존), and SHALL insert it via the same `AuditTx` (동일 트랜잭션).
- **REQ-EVALITEM-003-E2**: WHEN `RecordEvalItem*` derives the audit `ResourceID` for a given `hierarchy_code`, THEN the derivation SHALL be deterministic — invoking the derivation again with the same `hierarchy_code` and the same fixed `EvalItemAuditNamespace` SHALL produce a byte-identical `uuid.UUID` (재현 가능·충돌 없는 surrogate, audit 추적성 보장).

#### Unwanted

- **REQ-EVALITEM-003-U1**: IF the `audit_logs` INSERT fails for any reason (constraint violation, connection reset) during an evaluation item create or update transaction, THEN the subsystem SHALL execute `tx.Rollback(ctx)`, leave the `evaluation_items` table with NO trace of this operation (the item INSERT/UPDATE also rolled back), return the wrapped audit-insertion error to the caller, and SHALL NOT leak goroutines beyond the request scope. 평가항목 행과 감사 행은 함께 커밋되거나 함께 롤백된다 (all-or-nothing, SPEC-AX-EVID-001 REQ-EVID-003-U1 패턴).

### 3.5 REQ-EVALITEM-004 — 계층 불변성 & 라이프사이클 (Hierarchy Immutability & Lifecycle)

#### State-driven

- **REQ-EVALITEM-004-S1**: WHILE an evaluation item row has at least one child, the subsystem SHALL reject any `UpdateEvalItem` call that would change its `parent_id` or `level` (REQ-EVALITEM-UBI-004 강제 — store 계층에서 successor 존재 확인 후 에러 반환, SQL 미실행). 자식이 없는 잎 노드의 `parent_id`/`level` 변경은 허용된다.

#### Unwanted

- **REQ-EVALITEM-004-U1**: IF an `UpdateEvalItem` call attempts to set `status` to a value not in the enumerated set `{'ACTIVE', 'DEPRECATED', 'ARCHIVED'}`, THEN the subsystem SHALL reject the update (`evaluation_items_status_chk` CHECK 제약 + store 계층 사전 검증), SHALL NOT commit, and SHALL surface the constraint error. NULL `status` 또한 거부된다 (NOT NULL).

#### Optional

- **REQ-EVALITEM-004-O1**: WHERE a caller updates a leaf evaluation item's non-hierarchical attributes (`display_name`, `description`, `weight`, `max_score`, `status`, `metadata`), the subsystem MAY persist the change within a single `EvalItemTx` and SHALL record exactly one `EVAL_ITEM_UPDATED` audit row in the same transaction (REQ-EVALITEM-UBI-002 정합). 항목 버전 이력(versioning)은 `audit_logs` 추적으로 충분하며 별도 버전 테이블·스냅샷은 본 SPEC 범위 밖이다 (§5 Exclusions #4).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 데이터 주권 (망분리) | 평가항목 생성·조회·수정·검증 경로의 외부 API 호출 0건. 단일 내부망 PostgreSQL pgx pool만 사용 | §3.1 REQ-EVALITEM-UBI-001, `tech.md` §9.1 |
| 감사 가능성 | 모든 evaluation item create/update → 동일 TX 내 `audit_logs` 1건. 누락 0건 | §3.1 REQ-EVALITEM-UBI-002 |
| 계층 무결성 | 자식 보유 항목의 parent_id/level 불변, parent FK ON DELETE RESTRICT, hierarchy_code UNIQUE | §3.1 REQ-EVALITEM-UBI-004, §3.3, §3.5 |
| EVID-001 FK 타입 호환 | `evaluation_items.id` = `VARCHAR(64)` (UUID/auto-increment 아님), `evidences.evaluation_item_id VARCHAR(64)` stub과 타입 호환 | §1.4, research.md §2 |
| 성능 — 항목 생성 | p99 < 50ms (단일 TX INSERT + parent 검증, 단일 노드, 한국 공공 시간 제약 research.md §7) | §3.2 REQ-EVALITEM-001-E1 |
| 성능 — 계층 조회 | `GetEvalItemsByParentID`는 `evaluation_items_parent_id_idx` 인덱스 사용, p99 < 50ms (단일 레벨 조회) | §3.3 REQ-EVALITEM-002-E1 |
| cli-anonymous 기본값 | AuthN disabled 시 created_by/user_id='cli-anonymous' literal (NULL 금지) | §3.1 REQ-EVALITEM-UBI-003 |
| pgx pool 재사용 | 기존 SPEC-AX-CTRL-001 단일 pgx pool(`PgWorkflowStore.pool`, `pg_store.go`) 재사용, 신규 풀 생성 금지. `postgres.go` 死 스텁 비대상 | research.md §1, §8 R-EVALITEM-005(audit 폭증)·암묵 계약 |
| 로깅 | 구조화 JSON 로그(zap), 검증 거부는 INFO, 서버 결함은 ERROR | `tech.md` §8.2 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR), harness: thorough | `quality.yaml` development_mode |
| Go 도구 | go vet, golangci-lint (default + gosec), goimports | `.claude/rules/moai/languages/go.md` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **평가편람(평가지침) import / 파싱 연계** — 기획재정부 경영평가 편람(HWP/PDF) 일괄 파싱, 항목 자동 추출·upsert, Python `mapping` 파이프라인 연계 일체 제외. 본 SPEC은 평가항목 행을 수용하는 데이터 모델·store만 제공한다 (research.md §8 R-EVALITEM-003 import 이연).
2. **등급기준(scoring rubric) 저장 설계** — S/A/B/C/D 등급별 판정 기준·점수 산식의 구조화 저장(별도 테이블 vs `metadata` JSONB)은 본 SPEC에서 **설계하지 않는다**. `metadata` JSONB 컬럼은 opaque placeholder로만 정의되며 스키마·검증·해석은 미래 별도 결정으로 이연한다 (open follow-up — plan.md §6 참조).
3. **다중 테넌시 / 조직 격리** — 기관별 `org_id` 분리, 테넌트 스코핑, 조직 간 평가항목 격리는 SPEC-AX-AUTH 계열 또는 미래 platform SPEC 책임. 본 SPEC은 단일 테넌트 가정 (research.md §7 조직 격리는 org_id 미래).
4. **항목 버전 관리(versioning) — audit_logs 추적 초과 범위** — 평가항목 변경 스냅샷 테이블, `previous_version_id` 체이닝, 시점 복원(point-in-time restore)은 제외. 변경 추적은 `audit_logs` 행으로 충분(REQ-EVALITEM-UBI-002).
5. **항목 CRUD REST API / 삭제 API / Console UI** — HTTP 엔드포인트(생성/조회/수정/삭제), 평가항목 트리 뷰어, `apps/console/` 화면 일체 제외. 본 SPEC은 store 계층 메서드 + audit 연계만 다룬다 (삭제는 DB ON DELETE RESTRICT 제약만 정의, 삭제 경로 미구현).
6. **`evidences.evaluation_item_id` FK 하드닝 (EVID-001 코드 변경 포함)** — `evidences` 테이블에 `evaluation_item_id → evaluation_items(id)` FK 제약을 소급 추가하는 작업, 그리고 그에 수반되는 SPEC-AX-EVID-001 코드/마이그레이션 변경은 **본 SPEC 범위 밖**이다. 본 SPEC은 타입 호환(VARCHAR(64))되는 `evaluation_items` 테이블을 제공하기만 한다. FK 하드닝은 미래 별도 SPEC이 수행하며(SPEC-AX-EVID-001 §5 Exclusion #1 / plan.md §8 downstream 추적 역참조 예정), 그때까지 `evidences.evaluation_item_id`는 FK 없는 stub으로 유지된다(AC-EVALITEM-BOUNDARY-1로 경계 확인).
7. **계층 재배치 / 트리 이동 (re-parenting)** — 자식이 있는 항목의 `parent_id`/`level` 변경(서브트리 이동), 대량 트리 재구성은 REQ-EVALITEM-UBI-004로 금지되며 본 SPEC 범위 밖. 잎 노드 단순 속성 수정만 지원.
8. **closure table / JSONB nested / 다단계 정규화 구조** — Option A 자기참조 adjacency list 확정(plan.md §6 RESOLVED — Run Phase 1 strategy + Human Gate, 가중 A 9.0). B(4-table)·D(JSONB nested)는 기각, C(closure)는 재귀 subtree 요구 발생 시 named post-PoC 전환 경로(본 SPEC 미구현).
9. **마이그레이션 도구 통합 (alembic / golang-migrate)** — `0003_eval_item_tables.sql` 수동 멱등 SQL 단일 패치. 마이그레이션 러너 도구 제외 (SPEC-AX-CTRL-001 / SPEC-AX-EVID-001 §5 동일 정책).

---

## 6. 의존성 및 전제

- **SPEC-AX-CTRL-001 GREEN 가정**: `internal/store`의 `WorkflowStore`/`WorkflowTx` 인터페이스, `internal/audit`의 `Recorder`/`AuditTx`/`Action`/`Event`/`DefaultUserID`, 단일 pgx pool 와이어링이 모두 GREEN 상태이고 source-verified (research.md §1에서 `store.go`, `recorder.go`, `audit.go`, `pg_store.go`, `initial.sql` 실 시그니처 확인 — phantom API 없음).
- **SPEC-AX-EVID-001 완료(v0.1.2) 선례 재사용**: `EvidenceStore`/`EvidenceTx` 미러링 패턴, `RecordEvidenceCreated`/`RecordEvidenceVersioned` 추가 패턴, `BeginEvidenceTx`(`pg_store.go` 실 pool 재사용) 진입점 패턴, `0002_evidence_tables.sql` 멱등 SQL 규약을 본 SPEC의 `EvalItemStore`/`EvalItemTx`/`RecordEvalItem*`/`BeginEvalItemTx`/`0003_eval_item_tables.sql`이 동일하게 미러링한다.
- **`audit_logs` 테이블 스키마 재사용**: `initial.sql`의 `audit_logs`(id, user_id VARCHAR(64), action VARCHAR(64), resource_id `UUID NOT NULL`, resource_type VARCHAR(32), timestamp, details JSONB)를 그대로 사용한다. 본 SPEC은 `audit_logs` schema를 변경하지 않는다(§2.2 [EXISTING] HARD). **RESOLVED: `resource_id`(`uuid.UUID NOT NULL`, `audit.go:72`/`initial.sql:119`)와 VARCHAR(64) 계층 코드 id의 타입 불일치는 AUD-1(deterministic UUIDv5 surrogate `uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))`, 실 식별자는 `DetailsJSON`)로 확정**(Run Phase 1 strategy.md §2 + Human Gate Decision Point 2, §3.4 REQ-EVALITEM-003 / plan.md §6.6 — 이전 v0.1.2까지 run 단계 이연 항목이었으나 본 v0.1.3에서 확정).
- **`evaluation_items` 테이블 신규**: `0003_eval_item_tables.sql`로 추가. `initial.sql` 미수정 (schema drift 방지).
- **EVID-001 stub과의 단방향 type 계약**: 본 SPEC은 `evaluation_items` 테이블의 provider이며, `evidences.evaluation_item_id`(VARCHAR(64) stub)에 대한 역 FK는 생성하지 않는다(§5 Exclusion #6). 순환 의존 없음 — EVID-001은 본 SPEC 없이 이미 GREEN(완료)이다.
- **Go 1.22+**, module `github.com/ircp/iroum-ax`. 주요 의존성: `github.com/jackc/pgx/v5`, `go.uber.org/zap`, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go` (test). UUID 라이브러리는 본 SPEC에서 사용하지 않는다(`id`가 계층 코드 VARCHAR(64)이므로).
- **Cross-SPEC artifact 영향 없음**: 본 SPEC은 SPEC-AX-CTRL-001/EVID-001의 골든 파일 또는 기존 generated artifact를 수정하지 않는다 (clean additive 확장).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- **`evidences` FK 하드닝**: `evidences.evaluation_item_id`에 FK를 소급 추가하는 작업과 그에 수반되는 SPEC-AX-EVID-001 코드/마이그레이션 변경은 본 SPEC 범위 밖이다(§5 Exclusion #6). 본 SPEC은 type-compatible(VARCHAR(64)) `evaluation_items` 테이블 provider 역할만 하며, FK 하드닝은 미래 별도 SPEC이 수행한다. `evidences` 테이블·코드를 본 SPEC 구현 중 수정하지 말 것.
- **등급기준(scoring rubric) 저장 설계**: `metadata` JSONB는 opaque placeholder이며 등급 산식·판정 규칙 구조 설계는 본 SPEC에서 하지 않는다(open follow-up — plan.md §6).
- **평가편람 import/파싱**: HWP/PDF 파싱, 항목 자동 추출은 Python `ingestion`/`mapping` 파이프라인 책임이며 본 SPEC 범위 아님.
- **항목 CRUD/REST/Console**: HTTP 엔드포인트, 평가항목 트리 UI는 본 SPEC 범위 밖. store 계층 + audit만.
- **평가항목 권한/조직 격리**: SPEC-AX-AUTH 계열 책임. 본 SPEC은 cli-anonymous 기본값만.
- **closure table / JSONB nested 구조**: Option A 확정(plan.md §6 RESOLVED — Run Phase 1 strategy + Human Gate). B(4-table)/D(JSONB nested)는 기각, C(closure)는 재귀 subtree 요구 시의 named post-PoC 전환 경로.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/internal/{store,audit}/*_test.go` — 테이블 테스트, testify/assert, t.Parallel, goleak
- 통합 테스트: `apps/control-plane/internal/store/eval_item_test.go` — testcontainers-go(postgres:16-pgvector), 자기참조 계층, parent FK RESTRICT, hierarchy_code UNIQUE
- 계층 무결성 실패 경로 검증: 존재하지 않는 parent_id로 항목 생성 시 INSERT 시점 FK 위반 거부 + 행 미생성 + audit 미기록 (orphan 방지, REQ-EVALITEM-001-S1 실패 경로 — 전용 AC-EVALITEM-001-S1-1, ON DELETE RESTRICT 경로와 별개)
- 감사 원자성 테스트: `apps/control-plane/internal/audit/recorder_eval_item_test.go` — fault injection으로 audit INSERT 실패 시 evaluation_items INSERT 양방향 rollback 검증
- 데이터 주권 검증: 생성/조회/수정 경로에서 외부 네트워크 egress 0건 (테스트 환경 외부 host 차단 + 코드 정적 검사)
- EVID-001 FK 타입 호환 검증: `evaluation_items.id`의 정보 스키마상 데이터 타입이 `character varying(64)`인지 확인 (information_schema 쿼리)
- metadata opaque 검증: caller 제공 metadata JSONB가 구조 해석 없이 semantic value-equality로 round-trip 영속됨을 확인 (REQ-EVALITEM-001-O1 — 전용 AC-EVALITEM-001-O1-1; byte-level 비교 금지, JSONB 정규화 키 순서/공백/중복키는 비교 제외, 등급기준 스키마 미검증)
- 경계 검증: `evidences.evaluation_item_id`에 FK 제약이 여전히 부재함을 information_schema로 확인 (out-of-scope 경계 — AC-EVALITEM-BOUNDARY-1)
- 회귀: 기존 WorkflowStore/EvidenceStore 특성화 테스트가 eval-item 와이어링 후에도 GREEN 유지

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
