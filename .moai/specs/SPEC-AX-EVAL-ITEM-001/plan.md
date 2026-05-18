# SPEC-AX-EVAL-ITEM-001 구현 계획 (Plan)

> 대응 SPEC: `.moai/specs/SPEC-AX-EVAL-ITEM-001/spec.md` v0.1.2
> 방법론: TDD (RED-GREEN-REFACTOR), harness: thorough
> 근거: `.moai/specs/SPEC-AX-EVAL-ITEM-001/research.md` (Phase 0.5 deep research, file:line 근거 포함). 선례: SPEC-AX-EVID-001 v0.1.2(완료) `EvidenceStore`/`EvidenceTx`/`RecordEvidence*` 패턴.

---

## 1. 개요 및 접근 방식

본 SPEC은 SPEC-AX-CTRL-001이 확립하고 SPEC-AX-EVID-001이 증빙 도메인으로 검증한 **`WorkflowStore`/`WorkflowTx` + `Recorder`/`AuditTx` 트랜잭션 원자성 패턴**(research.md §1)을 평가항목 taxonomy 도메인으로 brownfield 확장한다. 핵심 설계 결정은 다음과 같다:

1. **데이터 모델 우선 (1st deliverable)**: `evaluation_items` 테이블 + `EvalItemStore`/`EvalItemTx` 인터페이스를 먼저 GREEN으로 만든다. 항목 CRUD/REST/Console·편람 import는 범위 밖.
2. **자기참조 adjacency list (Option A)**: 단일 테이블 + `parent_id` 자기 FK. 루트는 `parent_id NULL`, `ON DELETE RESTRICT`로 orphan 하위 계층 방지. 본 결정은 §6에서 A/B/C/D 트레이드오프로 strategy 단계 확정 (현 단계 OPEN/CONFIRMABLE).
3. **계층 코드 PK (VARCHAR(64), UUID 아님)**: `id`는 `AX-SAFETY-ORG-01` 형태 계층 코드. SPEC-AX-EVID-001 `evidences.evaluation_item_id VARCHAR(64)` stub과 타입 호환을 위한 HARD 계약 (spec.md §1.4, research.md §2).
4. **감사 원자성**: 항목 생성/수정과 `audit_logs` 기입을 단일 `EvalItemTx` 내에서 atomic 처리 (기존 `Recorder` 패턴 확장, SPEC-AX-EVID-001 `RecordEvidence*` 선례).
5. **계층 불변식 store 강제**: 자식 존재 항목의 `parent_id`/`level` 변경 거부 (REQ-EVALITEM-UBI-004), status enum CHECK.
6. **EVID-001 경계**: 본 SPEC은 `evaluation_items` provider. `evidences` FK 하드닝·EVID-001 코드 변경은 명시적 범위 밖 (spec.md §5 #6, §7).

### 1.1 Brownfield Delta 요약

| 마커 | 대상 | 처리 |
|------|------|------|
| [EXISTING] | `internal/store/pg_store.go` 기존 `PgWorkflowStore`/`PgWorkflowTx`/`BeginEvidenceTx`, `internal/audit/recorder.go` 기존 메서드(`RecordCreated`/`RecordEvidence*` 등), `internal/store/store.go` 기존 인터페이스 | 특성화 회귀 테스트로 보존 — 동작 변경 0건 |
| [NEW] | `internal/store/eval_item.go`, `.moai/db/schema/migrations/0003_eval_item_tables.sql`, `internal/store/eval_item_test.go`, `internal/audit/recorder_eval_item_test.go` | 신규 추가 |
| [MODIFY] | `internal/store/store.go` (`EvalItemStore`/`EvalItemTx` 인터페이스 2개 추가), `internal/store/pg_store.go` (실 `PgWorkflowStore.pool` 재사용한 `BeginEvalItemTx` 진입점 추가), `internal/audit/audit.go` (액션 상수 2개 추가), `internal/audit/recorder.go` (`RecordEvalItemCreated`/`RecordEvalItemUpdated` 메서드 추가) | 기존 심볼 불변, additive only |

> **Phantom-path 회피 (research.md §1, SPEC-AX-EVID-001 strategy.md §0 선례)**: eval-item TX 와이어링 대상은 `pg_store.go`의 `PgWorkflowStore{pool *pgxpool.Pool}`(`NewPgWorkflowStore` 생성, `server.go` 와이어링, `BeginEvidenceTx` 선례)이다. `postgres.go`는 Sprint-0 死 스텁(`New(cfg) (*Store, error)` + `// TODO`, 실 pool 없음, `BeginTx` 없음)이며 본 SPEC 대상이 아니다. 단일 pool 재사용(R-EVALITEM-005)은 `PgWorkflowStore.pool` 재사용으로 달성하며 run 단계에서 별도 협의 불요.

---

## 2. 작업 분해 (Sprint Decomposition)

TDD RED-GREEN-REFACTOR. 각 Sprint는 RED(실패 테스트) → GREEN(최소 구현) → REFACTOR 순.

### Sprint 0 — Foundation (Migration + 인터페이스 골격)
- [NEW] `0003_eval_item_tables.sql` 작성 (멱등 SQL, §3 DDL — `id VARCHAR(64) PK`, parent 자기 FK, status CHECK, 인덱스)
- [MODIFY] `store.go`에 `EvalItemStore`/`EvalItemTx` 인터페이스 선언 (메서드 시그니처만, 구현은 Sprint 1)
- [MODIFY] `audit.go`에 `ActionEvalItemCreated`/`ActionEvalItemUpdated` 상수 추가
- 기존 WorkflowStore/EvidenceStore 특성화 테스트 회귀 확인 (GREEN 유지)

### Sprint 1 — EvalItem Store: 생성 (REQ-EVALITEM-001)
- RED: `eval_item_test.go` — InsertEvalItem(루트, parent_id NULL) + GetEvalItemByID 단위 테스트 (testcontainers)
- GREEN: [NEW] `eval_item.go` pgx 구현(`PgEvalItemTx`, `pg_store.go`의 `PgEvidenceTx` 미러링), [MODIFY] `pg_store.go`에 `PgWorkflowStore.pool` 재사용한 `BeginEvalItemTx` 진입점 추가 (NOT `postgres.go` 死 스텁)
- RED: 자식 항목 생성 + parent 존재 검증 (REQ-EVALITEM-001-S1), id 형식/중복 거부 (REQ-EVALITEM-001-U1) 테스트
- GREEN: parent 사전 조회 + PK/길이/blank 검증
- REFACTOR: TX 진입점 중복 제거 (WorkflowTx/EvidenceTx와 공유 가능 헬퍼 추출 검토)

### Sprint 2 — 계층 구조 & 자기참조 (REQ-EVALITEM-002)
- RED: `GetEvalItemsByParentID` 자식 조회 + root parent_id NULL 테스트
- GREEN: 자기참조 SELECT 구현 (`evaluation_items_parent_id_idx` 사용)
- RED: parent FK ON DELETE RESTRICT (자식 있는 행 DELETE 거부) + hierarchy_code UNIQUE 충돌 테스트
- GREEN: DDL FK/UNIQUE 제약 검증 + store 계층 사전 검증

### Sprint 3 — 감사 연계 (REQ-EVALITEM-003)
- RED: `recorder_eval_item_test.go` — RecordEvalItemCreated/Updated audit row 검증
- GREEN: [MODIFY] `recorder.go`에 2개 메서드 추가 (기존 `RecordEvidenceCreated` 시그니처 패턴, 로컬 `AuditTx`)
- RED: audit INSERT 실패 → evaluation_items + audit 양방향 rollback (REQ-EVALITEM-003-U1)
- GREEN: store TX orchestration (audit 실패 시 `tx.Rollback`)

### Sprint 4 — 계층 불변성 & 라이프사이클 + 데이터 주권 (REQ-EVALITEM-004 + UBI 통합)
- RED: 자식 존재 항목의 parent_id/level 변경 거부 (REQ-EVALITEM-004-S1, REQ-EVALITEM-UBI-004)
- GREEN: `UpdateEvalItem` mutation guard (successor 존재 확인 후 거부)
- RED: status 전이 (ACTIVE→DEPRECATED→ARCHIVED) + 열거 외/NULL 거부 (REQ-EVALITEM-004-U1), 잎 노드 속성 수정 + audit (REQ-EVALITEM-004-O1)
- GREEN: status CHECK + 잎 노드 update 경로
- RED: 외부 네트워크 egress 0건 (REQ-EVALITEM-UBI-001) + cli-anonymous 기본값 (REQ-EVALITEM-UBI-003) + EVID-001 FK 부재 경계 검증 (AC-EVALITEM-BOUNDARY-1)
- GREEN: 데이터 주권 정적 보장, `resolveUserID` 재사용, `evidences` 미수정 확인

### Sprint 5 — Quality Gate
- 커버리지 ≥ 85%, golangci-lint default+gosec 0 issue, goleak 전체 통과
- @MX 태그 §5 매핑 완료, TRUST 5, evaluator-active strict ≥ 0.75

---

## 3. `evaluation_items` 테이블 DDL (`0003_eval_item_tables.sql`)

```sql
-- 마이그레이션: 0003_eval_item_tables
-- SPEC-AX-EVAL-ITEM-001 평가항목 taxonomy 데이터 모델 (멱등성 패턴 유지, 수동 SQL)
-- id는 계층 코드 VARCHAR(64) (UUID 아님) — SPEC-AX-EVID-001 evidences.evaluation_item_id VARCHAR(64) stub과 타입 호환
CREATE TABLE IF NOT EXISTS evaluation_items (
    id              VARCHAR(64) PRIMARY KEY,                                   -- 계층 코드 (예: AX-SAFETY-ORG-01) — EVID-001 FK 타입 호환
    parent_id       VARCHAR(64) REFERENCES evaluation_items(id) ON DELETE RESTRICT,  -- 자기참조, root는 NULL
    display_name    VARCHAR(256) NOT NULL,
    description     TEXT,
    level           INT,                                                       -- 1범주 2항목 3지표 4배점 (informational)
    hierarchy_code  VARCHAR(128) UNIQUE,                                       -- 경로 인코딩 (중복 방지)
    weight          DECIMAL(5,4),                                              -- 0.0-1.0 nullable
    max_score       INT,                                                       -- nullable
    status          VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',                     -- ACTIVE|DEPRECATED|ARCHIVED (CHECK)
    metadata        JSONB,                                                     -- opaque placeholder (등급기준 등 — 본 SPEC 미해석)
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by      VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',              -- audit.DefaultUserID 정합
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at     TIMESTAMP WITH TIME ZONE                                   -- 미래 lifecycle placeholder (본 SPEC 미사용)
);

DO $$ BEGIN
    ALTER TABLE evaluation_items ADD CONSTRAINT evaluation_items_status_chk
        CHECK (status IN ('ACTIVE','DEPRECATED','ARCHIVED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS evaluation_items_parent_id_idx
    ON evaluation_items (parent_id);
CREATE INDEX IF NOT EXISTS evaluation_items_hierarchy_code_idx
    ON evaluation_items (hierarchy_code);
CREATE INDEX IF NOT EXISTS evaluation_items_created_at_idx
    ON evaluation_items (created_at DESC);
```

DDL 근거: research.md §9 제안 데이터 모델. `initial.sql`은 수정하지 않는다 (schema drift 방지, spec.md §2.2). 파일번호 `0003`은 `migrations/`에 실재하는 `0001_initial.sql` + SPEC-AX-EVID-001 `0002_evidence_tables.sql`과 비충돌 확인(research.md §4) — initial.sql은 미수정.

---

## 4. 기술 스택

- 언어/런타임: Go 1.22+, module `github.com/ircp/iroum-ax`
- DB 드라이버: `github.com/jackc/pgx/v5` (기존 단일 pool 재사용 — research.md §1, §8 R-EVALITEM-005)
- UUID: **미사용** (`id`가 계층 코드 VARCHAR(64)이므로 — `google/uuid` 의존성 불필요)
- 로깅: `go.uber.org/zap` (기존 패턴)
- 테스트: `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go`(postgres:16-pgvector), `go.uber.org/goleak`
- 마이그레이션: 수동 멱등 SQL (`CREATE TABLE IF NOT EXISTS`, `DO $$ ... EXCEPTION ...`, `CREATE INDEX IF NOT EXISTS`), 도구 미사용
- 외부 의존: 신규 외부 SDK 추가 0건 (데이터 주권 REQ-EVALITEM-UBI-001 — 내부망 PostgreSQL pgx pool만)

---

## 5. MX 태그 계획

research.md §1, §8 및 SPEC-AX-CTRL-001/EVID-001 기존 태그 패턴(`store.go` `@MX:ANCHOR`/`@MX:REASON`)을 따른다.

| 대상 심볼 | 태그 | 근거 (@MX:REASON) |
|-----------|------|-------------------|
| `EvalItemStore` 인터페이스 (`store.go`) | `@MX:ANCHOR` | fan_in ≥ 3 (eval_item store 구현, recorder 연계, 향후 계층 조회 caller) — 기존 WorkflowStore/EvidenceStore와 동일 |
| `EvalItemTx` 인터페이스 (`store.go`) | `@MX:ANCHOR` | AC-EVALITEM-003-3 / AC-EVALITEM-UBI-002 원자성 계약의 핵심 (Fake + pgx 구현체 2곳 구현) |
| `BeginEvalItemTx` (`pg_store.go`) | `@MX:ANCHOR` | 평가항목 영속 단일 진입점 (REQ-EVALITEM-UBI-002 AC 전체가 이 경로 경유, `PgWorkflowStore.pool` 단일 재사용 불변식) |
| `RecordEvalItemCreated` / `RecordEvalItemUpdated` (`recorder.go`) | `@MX:ANCHOR` | 평가항목 감사 기록 단일 진입점 (REQ-EVALITEM-UBI-002 AC 전체가 이 메서드 경유) |
| 계층 조회/순회 로직 (`GetEvalItemsByParentID` 및 향후 재귀 서브트리 쿼리) (`eval_item.go`) | `@MX:WARN` | `@MX:REASON`: 깊은 계층에서 재귀 순회(WITH RECURSIVE) 시 인덱스 미사용 / 순환 parent 참조 시 무한 루프 위험 (research.md §8 R-EVALITEM-001, R-EVALITEM-004). `evaluation_items_parent_id_idx` 의존, 깊이 강제 없음 — 순환 탐지 자동화는 본 SPEC 범위 밖 |
| `UpdateEvalItem`의 successor-존재 검증 → parent_id/level 변경 거부 블록 (`eval_item.go`) | `@MX:WARN` | `@MX:REASON`: 계층 불변식(REQ-EVALITEM-UBI-004) 강제 지점 — successor 확인 누락 시 자식 보유 항목의 계층 위치가 변경되어 트리 무결성 붕괴. 검증 순서 변경 금지 |

TDD MX 흐름: RED에서 `@MX:TODO`(미구현 마커) → GREEN에서 제거 → REFACTOR에서 `@MX:NOTE`/`@MX:ANCHOR` 확정.

---

## 6. OPEN / CONFIRMABLE DECISION — taxonomy 테이블 구조 = Option A (자기참조 adjacency list)

> **상태**: **OPEN / strategy-confirmable**. 본 SPEC은 research.md §5 권고에 따라 **Option A(단일 자기참조 adjacency-list 테이블)**를 작업 설계로 채택했다. 본 결정은 Run Phase 1 strategy 단계(manager-strategy 분석 + Human Gate)에서 아래 트레이드오프 표를 통해 최종 확정(또는 재고)된다. SPEC-AX-EVID-001 §6의 RESOLVED 상태와 달리 본 SPEC §6는 아직 OPEN이다.

### 6.1 트레이드오프 표 (research.md §5 — PoC, 핵심 가치=감사 원자성 + 최소 스키마)

| Option | 구조 | Pros | Cons | 감사 호환 | 본 SPEC |
|--------|------|------|------|-----------|---------|
| **A 자기참조 adjacency list** (PoC 권고) | 단일 테이블 + `parent_id` 자기 FK | 최소 스키마, 임의 깊이, 단일 TX 1 entity+audit, CTRL-001/EVID-001 단순성 미러 | 재귀 쿼리(WITH RECURSIVE), 깊이 미강제 | 1행 1 Recorder 1 audit (동일 TX) — 적합 | **SELECTED (작업 설계, strategy 확정 대상)** |
| B 다단계 정규화 (categories/items/indicators/scoring 4 테이블) | 4 테이블 FK 체인 | DB 타입 안전, 범주 쿼리 빠름 | **4 audit 루프/saga 필요 — "1 TX 1 entity" 위배** | 부적합 (분산 TX) | 기각 (감사 원자성 위반) |
| C closure table | items + closure | 서브트리 쿼리 빠름 | 2테이블 insert, closure 일관성 유지 부담 | 2 Recorder/항목 — 복잡 | post-PoC 이연 |
| D JSONB nested | 단일 hierarchy_json | 1행 insert, 스키마리스 | 참조 무결성 0, evidence FK 불가, coarse audit | 계층당 1 audit (항목 단위 불가) | post-PoC 이연 |

권고: PoC는 **Option A**. B는 감사 원자성(REQ-EVALITEM-UBI-002 "1 TX 1 entity 1 audit") 구조적 위반으로 기각. C(closure)/D(JSONB)는 post-PoC strategy 재검토 (research.md §5).

### 6.2 strategy 단계 확정 항목

1. **Option A 확정 vs 재고**: 본 SPEC은 A를 작업 설계로 진행하되, strategy가 PoC 계층 깊이/조회 패턴을 근거로 A 확정 또는 C로의 조기 전환을 판정.
2. **등급기준(scoring rubric) 저장 — open follow-up (본 SPEC 미설계)**: S/A/B/C/D 등급 판정 기준·점수 산식의 저장 위치(`metadata` JSONB inline vs 별도 `scoring_criteria` 테이블)는 본 SPEC에서 **설계하지 않는다**. `metadata` JSONB는 opaque placeholder로만 정의된다. 이는 후속 별도 결정으로 이연하며 본 plan에서 구조를 정하지 않는다 (spec.md §5 Exclusion #2).
3. **EVID-001 FK 하드닝 타임라인**: `evidences.evaluation_item_id → evaluation_items(id)` FK 소급 추가 시점은 본 SPEC 범위 밖이며 미래 별도 SPEC이 결정 (spec.md §5 #6, §7).

### 6.3 본 SPEC이 보장하는 것

- `evaluation_items.id` = `VARCHAR(64)` 계층 코드 (UUID/auto-increment 아님) — EVID-001 stub과 타입 호환 (HARD)
- 자기참조 `parent_id` FK + `ON DELETE RESTRICT` + `hierarchy_code` UNIQUE
- 항목 create/update가 단일 `EvalItemTx` 내 audit_logs 1건과 atomic
- `evidences` 테이블·코드 불변 (FK 하드닝 미수행 — 경계 AC-EVALITEM-BOUNDARY-1로 확인)

---

## 7. 리스크 분석 (research.md §8 매핑)

| Risk ID | 설명 | 출처 | 완화 |
|---------|------|------|------|
| R-EVALITEM-001 | 재귀 쿼리 성능 (깊은 계층 서브트리 조회) | research.md §8 | `evaluation_items_parent_id_idx` + `hierarchy_code` 경로 인코딩. 본 SPEC은 단일 레벨 `GetEvalItemsByParentID`만 제공(재귀 서브트리 쿼리는 범위 밖), closure table은 post-PoC(§6 Option C). 계층 순회 로직 `@MX:WARN` |
| R-EVALITEM-002 | `evidences`→`evaluation_items` FK 지연 — orphan evidence 가능 | research.md §8 | SPEC-AX-EVID-001 설계가 수용(FK 없는 stub). 본 SPEC은 type-compatible 테이블 provider만, FK 하드닝은 미래 SPEC (spec.md §5 #6). AC-EVALITEM-BOUNDARY-1로 경계 확인 |
| R-EVALITEM-003 | 평가편람 import upsert 복잡 | research.md §8 | import 이연 (spec.md §5 Exclusion #1). 본 SPEC은 항목 행 수용 store만 |
| R-EVALITEM-004 | 순환 parent 참조 (자기/조상을 parent 지정) | research.md §8 | DB CHECK 불가 — store 계층 애플리케이션 검증 + PoC 수동 규율. 깊은 순환 자동 탐지는 본 SPEC 범위 밖(REQ-EVALITEM-002-U1 명시). 버전 결정/순회 로직 `@MX:WARN` |
| R-EVALITEM-005 | audit 행 폭증 (500 항목 일괄 등록) | research.md §8 | write-once 패턴, audit_logs 파티셔닝은 post-PoC 이연. PoC는 안전보건 단일 범주로 볼륨 한정. 단일 pgx pool 재사용으로 connection 폭증 방지 |
| R-EVALITEM-006 | cli-anonymous 마스킹 (미래 auth override) | research.md §8 | `Recorder.resolveUserID` 재사용, `created_by` DEFAULT 'cli-anonymous'. 미래 SPEC-AX-AUTH가 user_id override (본 SPEC은 기본값 계약만 — AC-EVALITEM-UBI-003) |

---

## 8. Cross-SPEC 영향

- **영향 받는 기존 generated artifact**: 없음. 본 SPEC은 SPEC-AX-CTRL-001/EVID-001의 골든 파일, 기존 store/audit 동작을 수정하지 않는다 (additive only).
- **DB 스키마 경로 규약 (research.md §4 정합, spec.md §2.2와 통일)**: `.moai/db/schema/initial.sql` = 기존 baseline 참조 스키마([EXISTING], 본 SPEC 미수정). `.moai/db/schema/migrations/NNNN_*.sql` = 순차 적용 마이그레이션. `migrations/`에 `0001_initial.sql`(실재) + SPEC-AX-EVID-001의 `0002_evidence_tables.sql`이 존재하므로, 본 SPEC의 신규 마이그레이션은 다음 가용 번호 `.moai/db/schema/migrations/0003_eval_item_tables.sql`이며 비충돌 확정(research.md §4).
- **EVID-001 stub 경계 (out-of-scope, downstream 역참조)**: SPEC-AX-EVID-001 §5 Exclusion #1 / plan.md §8 downstream 추적이 평가항목 taxonomy를 `SPEC-AX-EVAL-ITEM-001`(본 SPEC)으로 named placeholder 이연했다. 본 SPEC은 그 provider로서 type-compatible `evaluation_items(id VARCHAR(64))` 테이블을 제공하지만, `evidences.evaluation_item_id`에 역 FK를 **추가하지 않는다**(spec.md §5 Exclusion #6). FK 하드닝 = 미래 별도 SPEC. 본 SPEC 구현 중 `evidences` 테이블/코드를 수정하지 말 것 (AC-EVALITEM-BOUNDARY-1 게이트).
- **upstream Exclusion 역참조**: SPEC-AX-EVID-001 §5 #1 (평가항목 taxonomy 이연)을 본 SPEC이 부분 해소(테이블 provider 역할). FK 하드닝 부분은 미해소(미래 SPEC).
- **downstream 추적**: 미래 FK 하드닝 SPEC이 `evidences.evaluation_item_id → evaluation_items(id)` FK를 추가할 때 본 SPEC §5 Exclusion #6 및 SPEC-AX-EVID-001 §5 #1을 역참조해야 한다.

---

## 9. Definition of Done (Plan 관점)

- [ ] §2 Sprint 0-5 전부 GREEN
- [ ] `acceptance.md` 전체 AC(21개; v0.1.1 D1 정정 AC-EVALITEM-001-O1-1 + v0.1.2 LOW-2 정정 AC-EVALITEM-001-S1-1 추가) 자동화 테스트 통과
- [ ] 커버리지 ≥ 85%, golangci-lint default+gosec 0, goleak 통과
- [ ] §5 MX 태그 매핑 완료 (RED `@MX:TODO` 전부 해소)
- [ ] §6 OPEN DECISION = Option A (strategy 단계 확정 또는 재고 반영). 등급기준 저장은 미설계 유지(open follow-up)
- [ ] `evaluation_items.id` = VARCHAR(64) 계층 코드 확인 (information_schema), EVID-001 stub 타입 호환
- [ ] `evidences.evaluation_item_id` FK 부재 경계 확인 (AC-EVALITEM-BOUNDARY-1) — `evidences` 미수정
- [ ] 기존 WorkflowStore/EvidenceStore 특성화 회귀 0건
- [ ] manager-quality TRUST 5 통과, evaluator-active strict ≥ 0.75
