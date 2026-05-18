# SPEC-AX-EVAL-ITEM-001 Acceptance Criteria

> Format: Given / When / Then
> Methodology: TDD — 각 AC가 RED phase 단위 테스트로 1:1 매핑 (RED→GREEN 컬럼 명시)
> Tooling: Go 1.22 + testify/assert + testcontainers-go(postgres:16-pgvector) + goleak
> AC 명명: `AC-EVALITEM-{REQ}-{N}` (예: AC-EVALITEM-001-1, AC-EVALITEM-UBI-002)

본 문서는 SPEC-AX-EVAL-ITEM-001의 5개 REQ 모듈(1 Ubiquitous 묶음 + 4 modal)에 대한 acceptance criteria를 정의한다. 각 AC는 자동화 테스트로 검증 가능해야 하며 모호한 표현("적절히", "신속하게")을 사용하지 않는다.

---

## §0. REQ-EVALITEM-UBI Transverse Invariants — Acceptance

§1-§4 modal REQ를 가로지르는 4개 Ubiquitous 불변 조건의 dedicated AC. SPEC-AX-CTRL-001 §0 / SPEC-AX-EVID-001 §0 AC-*-UBI-* 패턴을 동일 구조로 적용한다.

### AC-EVALITEM-UBI-001 (Data Sovereignty — No External Call)

REQ 대응: REQ-EVALITEM-UBI-001 (데이터 주권 — 생성/조회/수정/검증 경로 외부 호출 0건).
TDD: RED `recorder_eval_item_test.go` 외부 host 차단 환경 → GREEN 내부 pgx pool 자원만.

**Given**:
- 테스트 환경이 외부 네트워크 egress를 차단 (loopback/내부망 host만 허용)
- 평가항목 생성 경로가 `EvalItemTx` INSERT를 수행

**When**:
- 호출자가 신규 평가항목 `id="AX-SAFETY-ROOT"`를 정상 생성한다

**Then**:
- 평가항목 생성이 성공한다
- 생성/감사 경로에서 외부 host로의 DNS 해석 또는 TCP 연결이 0건 (네트워크 spy 또는 정적 import 검사)
- `internal/store`, `internal/audit`가 외부 SaaS SDK(aws-sdk, 외부 client 등) 미import

### AC-EVALITEM-UBI-002 (Audit Completeness — Create & Update)

REQ 대응: REQ-EVALITEM-UBI-002 (감사 가능성 — 모든 create/update에 동일 TX `audit_logs` 1건).
TDD: RED audit row 부재 검증 → GREEN RecordEvalItemCreated/Updated 동일 TX 기입.

**Given**:
- PostgreSQL `evaluation_items` + `audit_logs` 테이블이 testcontainers fixture로 clean state
- AuthN disabled (Walking Skeleton 기본값)

**When**:
- 호출자가 신규 항목 `id="AX-SAFETY-ORG-01"`을 생성하고, 이어서 동일 항목의 `display_name`을 수정한다 (각각 별도 `EvalItemTx`)

**Then**:
- 생성 후 `audit_logs`에 `action='EVAL_ITEM_CREATED'`, `resource_type='evaluation_item'` row 정확히 1개; `resource_id`는 AUD-1 결정적 UUIDv5 surrogate(`uuid.NewSHA1(EvalItemAuditNamespace, hierarchy_code)`)이며 실 항목 식별은 `details->>'eval_item_id' = 'AX-SAFETY-ORG-01'`로 검증 (원시 계층코드는 `resource_id`에 저장 불가 — `uuid.UUID NOT NULL`, spec.md §3.4 AUD-1)
- 수정 후 `audit_logs`에 `action='EVAL_ITEM_UPDATED'` row 정확히 1개 (동일 hierarchy_code → 동일 `resource_id` UUIDv5, `details->>'eval_item_id'` 동일)
- 각 이벤트마다 `evaluation_items` 변경 + `audit_logs` row 1건이 동일 트랜잭션 커밋 (한쪽만 존재하는 상태 없음)

### AC-EVALITEM-UBI-003 (cli-anonymous Default)

REQ 대응: REQ-EVALITEM-UBI-003 (AuthN disabled 시 created_by/user_id = 'cli-anonymous').
TDD: RED user_id NULL/실사용자 검증 실패 → GREEN resolveUserID 재사용.

**Given**:
- Walking Skeleton 환경 (`AUTH_ENABLED=false`)
- 호출자가 인증 컨텍스트 없이 평가항목 생성

**When**:
- 평가항목 `id="AX-SAFETY-UBI-003"`을 생성한다 (no auth)

**Then**:
- `evaluation_items.created_by = 'cli-anonymous'` (정확히 literal, NULL 금지)
- `audit_logs.user_id = 'cli-anonymous'`
- 두 컬럼이 byte-identical (cross-table consistency, SPEC-AX-EVID-001 AC-EVID-UBI-003 정합)

### AC-EVALITEM-UBI-004 (Hierarchy Immutability — Child-bearing Node)

REQ 대응: REQ-EVALITEM-UBI-004 (자식 존재 시 parent_id/level 변경 금지).
TDD: RED 자식 보유 항목 parent_id 변경 허용(잘못) → GREEN store mutation guard.

**Given**:
- `id="AX-SAFETY-CAT"` (parent_id NULL, level=1)와 그 자식 `id="AX-SAFETY-CAT-01"` (parent_id='AX-SAFETY-CAT', level=2)이 존재

**When**:
- `UpdateEvalItem("AX-SAFETY-CAT", parent_id=<다른 값> 또는 level=<다른 값>)`을 시도한다 (자식 보유 노드)

**Then**:
- store 계층이 변경을 거부 (에러 반환, SQL UPDATE 미실행 — REQ-EVALITEM-UBI-004, REQ-EVALITEM-004-S1)
- `AX-SAFETY-CAT.parent_id`/`level` 변경 0건 (불변 보장)
- 동일 항목의 `display_name` 등 비-계층 컬럼 수정은 허용됨 (대조 검증)

---

## §1. REQ-EVALITEM-001 평가항목 데이터 모델 & Store — Acceptance

### AC-EVALITEM-001-1 (Happy Path: 루트 항목 생성 + 감사 원자적 커밋)

TDD: RED InsertEvalItem 미구현 → GREEN eval_item.go pgx 구현.

**Given**:
- `evaluation_items`/`audit_logs` 테이블 clean state (testcontainers)
- 신규 루트 항목 `id="AX-SAFETY"`, `display_name="안전보건"`, `parent_id=NULL`, `level=1`

**When**:
- 호출자가 `BeginEvalItemTx` → `InsertEvalItem` → `RecordEvalItemCreated` → `Commit`을 수행한다

**Then**:
- `evaluation_items`에 정확히 1 row: `id='AX-SAFETY'`, `parent_id IS NULL`, `status='ACTIVE'` (DEFAULT), `created_by='cli-anonymous'`
- `audit_logs`에 `EVAL_ITEM_CREATED` 1 row (동일 TX)
- 응답 < 50ms p99 (10회 반복, research.md §7 한국 공공 시간 제약)

### AC-EVALITEM-001-2 (자식 항목 생성 — parent_id 참조)

TDD: RED 자식 항목 parent 미검증 → GREEN parent 사전 조회.

**Given**:
- 루트 `id="AX-SAFETY"` (level=1)가 이미 존재

**When**:
- 자식 항목 `id="AX-SAFETY-ORG-01"`, `parent_id="AX-SAFETY"`, `level=2`를 생성한다

**Then**:
- `evaluation_items`에 자식 row 생성: `parent_id='AX-SAFETY'`
- `GetEvalItemsByParentID("AX-SAFETY")`가 `AX-SAFETY-ORG-01`을 포함 반환
- 동일 TX 내 `EVAL_ITEM_CREATED` audit row 1건

### AC-EVALITEM-001-S1-1 (Edge — 존재하지 않는 parent_id → INSERT 시 FK 위반 거부, orphan 방지)

REQ 대응: REQ-EVALITEM-001-S1 (non-NULL parent_id 생성 시 parent 존재 검증, 미존재 시 트랜잭션 커밋 없이 거부 — orphan 노드 방지). 실패 경로 전용 AC (LOW-2 정정, v0.1.2 — AC-EVALITEM-001-2는 success 경로만 검증했음).
TDD: RED 존재하지 않는 parent_id가 성공(잘못, orphan 생성) → GREEN parent 사전 조회 + FK 제약(`evaluation_items_parent_id_fkey`) 위반 거부.
주의: 본 AC는 **INSERT 시점의 FK 위반 거부**(존재하지 않는 parent 참조)를 검증하며, AC-EVALITEM-002-2의 **ON DELETE RESTRICT**(자식 보유 parent 삭제 거부)와는 별개 경로다.

**Given**:
- `evaluation_items`/`audit_logs` 테이블 clean state (testcontainers)
- `parent_id="AX-NONEXISTENT-PARENT"`를 가리키는 항목이 데이터베이스에 **존재하지 않음**

**When**:
- 호출자가 `id="AX-ORPHAN-01"`, `parent_id="AX-NONEXISTENT-PARENT"` (존재하지 않는 parent)로 평가항목 생성을 시도한다

**Then**:
- 생성이 거부됨 (store 계층 parent 사전 조회 미존재 에러 또는 `evaluation_items_parent_id_fkey` FK 제약 위반 — orphan 노드 방지, REQ-EVALITEM-001-S1)
- `evaluation_items`에 `id='AX-ORPHAN-01'` row **미생성** (트랜잭션 커밋 0건)
- `audit_logs`에 해당 생성 관련 row **미기록** (TX 미커밋 → audit 미기입, REQ-EVALITEM-UBI-002 정합 — 부분 커밋 0건)
- 거부 에러가 호출자에게 surface됨 (client error, INFO 로그)

### AC-EVALITEM-001-3 (id 타입 — VARCHAR(64), EVID-001 FK 호환)

TDD: RED id가 UUID/auto-increment 가정 테스트 실패 → GREEN VARCHAR(64) 계층 코드 PK.

**Given**:
- `evaluation_items` 테이블이 `0003_eval_item_tables.sql`로 생성됨
- 계층 코드 형태 id `"AX-SAFETY-ORG-01-1"` (UUID 아님)

**When**:
- 해당 계층 코드 id로 평가항목을 생성하고, information_schema로 `evaluation_items.id` 컬럼 타입을 조회한다

**Then**:
- 생성이 성공한다 (id가 임의 계층 코드 문자열, UUID 형식 강제 없음)
- `information_schema.columns`에서 `evaluation_items.id`의 `data_type='character varying'`, `character_maximum_length=64`
- 동일하게 `evidences.evaluation_item_id`도 `character varying(64)` (SPEC-AX-EVID-001 stub) — 두 컬럼 타입이 호환되어 미래 FK 추가 시 타입 불일치 0건 (단, 본 SPEC은 FK를 추가하지 않음 — AC-EVALITEM-BOUNDARY-1 참조)

### AC-EVALITEM-001-4 (Edge — id blank / 64자 초과 / display_name blank / 중복 id 거부)

TDD: RED 검증 부재 → GREEN store pre-INSERT 검증 + PK 위반 처리.

**Given**:
- `evaluation_items` clean state, 기존 항목 `id="AX-DUP"` 존재

**When**:
- 호출자가 (a) `id=""` (blank), 또는 (b) `id` 65자, 또는 (c) `display_name=""` (blank), 또는 (d) `id="AX-DUP"` (중복 PK)으로 생성을 시도한다

**Then**:
- 각 경우 구조화 검증 에러 반환 (client error)
- `evaluation_items` 변화 0건, `audit_logs` 변화 0건 (트랜잭션 미진입 또는 PK 위반 rollback)
- (d) 중복 id는 PRIMARY KEY 제약 위반으로 거부 (기존 row 불변)
- 서버 로그 레벨 = INFO (client error, not server defect)

### AC-EVALITEM-001-O1-1 (Optional — metadata JSONB opaque verbatim 영속)

REQ 대응: REQ-EVALITEM-001-O1 (caller가 metadata JSONB payload를 제공하면 subsystem이 구조 해석 없이 verbatim opaque 영속, 등급기준 스키마 검증 없음).
TDD: RED metadata 미저장/구조 강제(잘못) → GREEN JSONB 컬럼 verbatim round-trip.
주의: REQ-EVALITEM-001-O1은 `WHERE ... SHALL persist` Optional 절이며, 본 AC는 SPEC-AX-EVID-001 AC-EVID-001-O1-1 패턴과 동일하게 §1 전용 AC로 O1 1:1 coverage를 확보한다 (D1 정정, v0.1.1 — 기존 AC-EVALITEM-004-3 내부 간접 검증의 coverage illusion 해소).

**Given**:
- `evaluation_items`/`audit_logs` 테이블 clean state (testcontainers)
- caller가 신규 항목 `id="AX-META-O1"`, `display_name="안전교육"` 생성 시 임의 구조의 metadata JSONB payload 제공 (예: `{"등급기준": {"S": "...", "A": "..."}, "draft_note": "임의 중첩 구조"}`)

**When**:
- 호출자가 해당 metadata payload를 포함하여 `BeginEvalItemTx` → `InsertEvalItem` → `Commit`을 수행한다

**Then**:
- 항목이 정상 생성되고 `evaluation_items.metadata`가 입력 payload와 **semantically equivalent JSONB**로 저장됨 (필드 누락/값 변형 0건 — PostgreSQL JSONB value-equality 기준. 오브젝트 키 순서·공백·중복 키 제거는 PostgreSQL JSONB 정규화 범위이므로 비교 대상이 아니다)
- subsystem이 metadata 구조를 **해석/검증하지 않음**: 등급기준 스키마 검증 없음, 필수 키 강제 없음, 임의 중첩/빈 객체(`{}`)/NULL 모두 거부 없이 그대로 수용 (REQ-EVALITEM-001-O1 — opaque placeholder)
- 동일 TX 내 `EVAL_ITEM_CREATED` audit row 1건 (REQ-EVALITEM-UBI-002 정합)
- 본 AC는 등급기준(scoring rubric) 저장 구조를 검증하지 **않는다** (미설계 open follow-up — spec.md §5 Exclusion #2, plan.md §6 #2). metadata는 opaque로만 검증

> 비고 (LOW-1 정정, v0.1.2): 테스트 구현은 metadata round-trip 검증 시 **byte-level string 비교를 사용하지 말 것**. PostgreSQL JSONB는 입력을 정규화(키 순서 재배열, 공백 제거, 중복 키 last-wins)하므로 올바른 구현에서도 byte-identical은 달성 불가하며 false RED를 유발한다. 검증은 semantic JSON equality(예: 양측을 `map[string]any`로 unmarshal 후 `reflect.DeepEqual`, 또는 `jsonEqual()` 헬퍼)로 수행한다. JSONB가 정규화하는 키 순서/공백/중복키는 동치 판정에서 제외한다.

---

## §2. REQ-EVALITEM-002 계층 구조 & 자기참조 — Acceptance

### AC-EVALITEM-002-1 (Root parent_id NULL + 자식 조회)

REQ 대응: REQ-EVALITEM-002-E1a (root, parent_id NULL) + REQ-EVALITEM-002-E1b (child, non-NULL parent_id). 본 AC가 분할된 두 절을 함께 검증한다 (D2 정정, v0.1.1 — 002-E1 atomic 분할).
TDD: RED 자기참조 SELECT 미구현 → GREEN GetEvalItemsByParentID.

**Given**:
- 루트 `id="AX-SAFETY"` (parent_id NULL — E1a), 자식 `AX-SAFETY-ORG-01`, `AX-SAFETY-ORG-02` (parent_id='AX-SAFETY' — E1b)

**When**:
- `GetEvalItemsByParentID("AX-SAFETY")` 및 루트 조회 호출

**Then**:
- 루트 row의 `parent_id IS NULL` (root 노드는 parent 없음)
- `GetEvalItemsByParentID("AX-SAFETY")`가 [AX-SAFETY-ORG-01, AX-SAFETY-ORG-02] 2개 반환
- 조회가 `evaluation_items_parent_id_idx` 인덱스 사용 (EXPLAIN 검증, p99 < 50ms)

### AC-EVALITEM-002-2 (Edge — parent FK ON DELETE RESTRICT, orphan 방지)

TDD: RED 자식 있는 parent DELETE 허용(잘못) → GREEN FK RESTRICT 강제.

**Given**:
- 부모 `id="AX-SAFETY-CAT"`와 자식 `id="AX-SAFETY-CAT-01"` (parent_id='AX-SAFETY-CAT')이 존재

**When**:
- 직접 SQL로 `DELETE FROM evaluation_items WHERE id='AX-SAFETY-CAT'`을 시도한다 (자식 보유 부모)

**Then**:
- FK 제약 `evaluation_items_parent_id_fkey ON DELETE RESTRICT` 위반으로 DELETE 거부 (orphan 하위 계층 방지)
- `AX-SAFETY-CAT`, `AX-SAFETY-CAT-01` 두 row 모두 보존 (삭제 0건)
- 본 SPEC은 삭제 API를 제공하지 않으며, 이 AC는 DB 제약의 구조적 보호를 검증 (REQ-EVALITEM-002-S1)

### AC-EVALITEM-002-3 (Edge — hierarchy_code Uniqueness)

TDD: RED hierarchy_code 중복 허용(잘못) → GREEN UNIQUE 제약 + store 검증.

**Given**:
- 항목 `id="AX-A"`, `hierarchy_code="AX.SAFETY.ORG.01"`이 이미 존재

**When**:
- 다른 항목 `id="AX-B"`를 동일 `hierarchy_code="AX.SAFETY.ORG.01"`로 생성을 시도한다

**Then**:
- `evaluation_items_hierarchy_code_idx` UNIQUE 제약 위반으로 INSERT 거부 (계층 경로 중복 방지 — REQ-EVALITEM-002-U1)
- 트랜잭션 커밋 0건, `AX-B` row 미생성
- 기존 `AX-A` row 불변

### AC-EVALITEM-002-4 (계층 순회 — GetEvalItemsByParentID 다단계)

TDD: RED 다단계 계층 조회 단절 → GREEN level별 자식 조회.

**Given**:
- 3계층: `AX-SAFETY`(L1) → `AX-SAFETY-ORG-01`(L2, parent='AX-SAFETY') → `AX-SAFETY-ORG-01-1`(L3, parent='AX-SAFETY-ORG-01')

**When**:
- `GetEvalItemsByParentID("AX-SAFETY")` 후 `GetEvalItemsByParentID("AX-SAFETY-ORG-01")` 순차 호출 (단계별 하향 순회)

**Then**:
- 1단계: [AX-SAFETY-ORG-01] 반환
- 2단계: [AX-SAFETY-ORG-01-1] 반환
- 각 노드의 `level` 값이 1/2/3으로 informational하게 일관 (단, level은 강제 제약 아님 — research.md §9)
- 어떤 노드도 누락/중복 없음 (단일 레벨 조회만 — 재귀 서브트리 쿼리는 본 SPEC 범위 밖, plan.md §7 R-EVALITEM-001)

---

## §3. REQ-EVALITEM-003 감사 연계 — Acceptance

### AC-EVALITEM-003-1 (RecordEvalItemCreated Audit Row — AUD-1 deterministic UUIDv5 resource_id)

REQ 대응: REQ-EVALITEM-003-E1. AUD-1 전략(plan.md §6.6, spec.md §3.4): `resource_id`는 원시 계층코드가 아닌 결정적 UUIDv5, 실 식별자는 `DetailsJSON`.
TDD: RED RecordEvalItemCreated 미구현 + (잘못) `resource_id`에 원시 계층코드 단언 → GREEN recorder.go 메서드 + `uuid.NewSHA1` surrogate.
주의 (Decision 3 정정, v0.1.3): 이전 v0.1.2까지 `ResourceID="AX-SAFETY-ORG-01"`(원시 계층코드) 단언은 `audit_logs.resource_id`가 `uuid.UUID NOT NULL`(initial.sql:119)이라 올바른 구현에서도 false RED → AUD-1 기준 재작성.

**Given**:
- `audit_logs` clean state, Recorder(`authEnabled=false`)
- 고정 namespace 상수 `EvalItemAuditNamespace`가 `internal/audit/audit.go`에 정의됨
- `hierarchyCode="AX.SAFETY.ORG.01"`, `itemID="AX-SAFETY-ORG-01"`

**When**:
- 평가항목 생성 TX가 `Recorder.RecordEvalItemCreated(ctx, tx, itemID="AX-SAFETY-ORG-01", hierarchyCode="AX.SAFETY.ORG.01", parentID, level, userID="")` 호출

**Then**:
- `audit.Event`: `Action="EVAL_ITEM_CREATED"`, `ResourceType="evaluation_item"`, `UserID="cli-anonymous"`(resolveUserID), `Timestamp` NOT NULL
- `Event.ResourceID` (= `audit_logs.resource_id`, 타입 `uuid.UUID`)가 **원시 계층코드가 아니라** `uuid.NewSHA1(EvalItemAuditNamespace, []byte("AX.SAFETY.ORG.01"))`와 byte-identical (결정적 UUIDv5 surrogate — `resource_id != uuid.Nil`, `resource_id`가 문자열 "AX-SAFETY-ORG-01"이 아님을 검증)
- 실제 계층 식별자는 `DetailsJSON`으로 검증: `details->>'eval_item_id' = 'AX-SAFETY-ORG-01'` AND `details->>'hierarchy_code' = 'AX.SAFETY.ORG.01'` (`parent_id`/`level`도 포함)
- 동일 `AuditTx`로 INSERT (store→audit 순환 의존 없음 — `audit` 패키지가 `store` 미import)

### AC-EVALITEM-003-2 (RecordEvalItemUpdated Audit Row — AUD-1)

REQ 대응: REQ-EVALITEM-003-E1 (update 이벤트). AUD-1 동일 적용.
TDD: RED RecordEvalItemUpdated 미구현 → GREEN 메서드 + UUIDv5 surrogate.

**Given**:
- 항목 `id="AX-SAFETY-ORG-01"`, `hierarchyCode="AX.SAFETY.ORG.01"`이 존재, 수정 TX 진행 중

**When**:
- `Recorder.RecordEvalItemUpdated(ctx, tx, itemID="AX-SAFETY-ORG-01", hierarchyCode="AX.SAFETY.ORG.01", parentID, level, userID="")` 호출 (display_name 수정 이벤트)

**Then**:
- `Action="EVAL_ITEM_UPDATED"`
- `Event.ResourceID` = `uuid.NewSHA1(EvalItemAuditNamespace, []byte("AX.SAFETY.ORG.01"))` (결정적 UUIDv5, 원시 계층코드 아님), `resource_id != uuid.Nil`
- 실 식별자는 `DetailsJSON`: `details->>'eval_item_id' = 'AX-SAFETY-ORG-01'` AND `details->>'hierarchy_code' = 'AX.SAFETY.ORG.01'`
- `user_id="cli-anonymous"`

### AC-EVALITEM-003-E2-1 (resource_id 결정성 — 동일 hierarchy_code → 동일 UUID)

REQ 대응: REQ-EVALITEM-003-E2 (deterministic UUIDv5 surrogate 재현성 — v0.1.3 Decision 3 신규 AC).
TDD: RED 비결정적/랜덤 resource_id (잘못, audit 추적성 붕괴) → GREEN `uuid.NewSHA1(고정 namespace, hierarchyCode)` 결정적 산출.

**Given**:
- 고정 상수 `EvalItemAuditNamespace` (`internal/audit/audit.go`)
- 동일 `hierarchyCode="AX.SAFETY.ORG.01"`로 2회 audit 기록 (예: 항목 생성 후 동일 항목 update — 같은 hierarchy_code)

**When**:
- `RecordEvalItemCreated`(1회차)와 `RecordEvalItemUpdated`(2회차)가 동일 `hierarchyCode`로 `Event.ResourceID`를 파생한다

**Then**:
- 두 호출의 `Event.ResourceID`가 **byte-identical** (`uuid.NewSHA1`은 고정 namespace + 동일 입력 → 동일 UUID — 결정적·재현 가능)
- 산출 UUID는 `uuid.Nil`이 아니며 RFC 4122 version 5 (SHA-1 name-based)
- 서로 다른 `hierarchyCode`(예: `"AX.SAFETY.ORG.02"`)는 다른 `ResourceID` 산출 (충돌 없음 — surrogate가 hierarchy_code별로 안정적으로 audit row를 그룹화 가능, R-EVALITEM-007 완화 검증)

### AC-EVALITEM-003-3 (Edge — Audit Fail → 항목+감사 양방향 Rollback 원자성)

TDD: RED audit 실패 시 evaluation_items row 잔존(잘못) → GREEN tx.Rollback 양방향.

**Given**:
- `evaluation_items`/`audit_logs` clean state
- Test harness가 `audit_logs` INSERT에 fault injection (CHECK constraint violation 강제)

**When**:
- 평가항목 생성 TX가 (a) `evaluation_items` INSERT 성공 후 (b) `audit_logs` INSERT가 실패한다
- store가 `tx.Rollback(ctx)` 호출

**Then**:
- `evaluation_items` 테이블에 row **존재하지 않음** (item INSERT도 rollback)
- `audit_logs` 테이블에 row 없음 (애초에 실패)
- 반환 에러 = wrapped audit insertion failure
- 부분 커밋 0건 (all-or-nothing, REQ-EVALITEM-003-U1)
- `goleak.VerifyNone(t)` 통과
- 이는 SPEC-AX-EVID-001 AC-EVID-003-3 / SPEC-AX-CTRL-001 AC-CTRL-UBI-001 패턴의 평가항목 도메인 대응

---

## §4. REQ-EVALITEM-004 계층 불변성 & 라이프사이클 — Acceptance

### AC-EVALITEM-004-1 (자식 존재 항목의 parent_id/level 변경 거부)

TDD: RED successor 미확인 변경 허용(잘못) → GREEN mutation guard.

**Given**:
- `id="AX-SAFETY-CAT"`와 자식 `id="AX-SAFETY-CAT-01"` (parent_id='AX-SAFETY-CAT') 존재

**When**:
- `UpdateEvalItem("AX-SAFETY-CAT", parent_id="AX-OTHER")` 및 `UpdateEvalItem("AX-SAFETY-CAT", level=3)`을 시도한다

**Then**:
- store 계층이 successor 존재를 확인 후 두 변경 모두 거부 (에러 반환, SQL UPDATE 미실행 — REQ-EVALITEM-004-S1)
- `AX-SAFETY-CAT.parent_id`/`level` 변경 0건
- 거부 에러 메시지에 계층 불변식 위반 사유 포함

### AC-EVALITEM-004-2 (status 전이 + 열거 외/NULL 거부)

TDD: RED status 임의값 허용(잘못) → GREEN CHECK 제약 + store 검증.

**Given**:
- 잎 노드 항목 `id="AX-LEAF"` (자식 없음, status='ACTIVE')

**When**:
- (a) `UpdateEvalItem("AX-LEAF", status='DEPRECATED')` 후 `status='ARCHIVED'`로 전이
- (b) `UpdateEvalItem("AX-LEAF", status='INVALID_X')` (열거 외) 또는 `status=NULL` 시도

**Then** (a):
- `AX-LEAF.status`가 ACTIVE → DEPRECATED → ARCHIVED로 정상 전이, 각 전이마다 `EVAL_ITEM_UPDATED` audit row 1건 (동일 TX)

**Then** (b):
- `evaluation_items_status_chk` CHECK 제약 위반(또는 store 사전 검증)으로 거부
- NULL status도 거부 (NOT NULL)
- `AX-LEAF.status` 변경 0건

### AC-EVALITEM-004-3 (잎 노드 비-계층 속성 수정 허용 + audit)

TDD: RED 잎 노드 update 미구현 → GREEN 단일 TX update + audit.

**Given**:
- 잎 노드 `id="AX-LEAF-2"` (자식 없음, `display_name="구안전교육"`, `weight=NULL`)

**When**:
- `UpdateEvalItem("AX-LEAF-2", display_name="안전교육 이수율", weight=0.1500, metadata={...})`을 단일 `EvalItemTx`로 수행한다

**Then**:
- `AX-LEAF-2`의 `display_name`, `weight`, `metadata`가 갱신됨 (잎 노드 비-계층 속성 변경 허용 — REQ-EVALITEM-004-O1)
- `parent_id`/`level`은 잎 노드 단순 수정 시 불변(이 시나리오에서 변경 안 함) — leaf는 계층 변경도 허용되나 본 AC는 비-계층 속성만 검증
- 동일 TX 내 `EVAL_ITEM_UPDATED` audit row 정확히 1건 (REQ-EVALITEM-UBI-002 정합)
- `metadata` JSONB가 update 경로에서도 verbatim 갱신됨 (구조 해석/검증 없음)

> 주의 (D1 정정, v0.1.1): REQ-EVALITEM-001-O1 (metadata opaque verbatim 영속)의 1:1 검증은 §1 전용 AC **AC-EVALITEM-001-O1-1**이 담당한다. 본 AC-EVALITEM-004-3은 update 경로에서의 비-계층 속성 수정 + audit를 검증하며, metadata는 부수적으로 다룰 뿐 O1 전용 coverage 보유처가 아니다 (이전 v0.1.0의 간접 검증 coverage illusion 해소).

---

## §5. Boundary — EVID-001 Out-of-Scope Confirmation

### AC-EVALITEM-BOUNDARY-1 (evidences.evaluation_item_id FK 부재 유지)

REQ 대응: spec.md §5 Exclusion #6 / §7 Out of Scope (evidences FK 하드닝 = 본 SPEC 범위 밖).
TDD: RED `evidences`→`evaluation_items` FK 존재 가정 테스트 → GREEN FK 부재 확인 (본 SPEC이 FK를 추가하지 않음).

**Given**:
- `0003_eval_item_tables.sql` 적용으로 `evaluation_items` 테이블이 생성됨
- SPEC-AX-EVID-001의 `evidences` 테이블(`evaluation_item_id VARCHAR(64)` FK 없는 stub)이 이미 존재

**When**:
- `0003_eval_item_tables.sql` 적용 전후로 information_schema(`table_constraints`, `referential_constraints`)를 쿼리하여 `evidences.evaluation_item_id` → `evaluation_items(id)` FK 제약 존재 여부를 확인한다

**Then**:
- `0003_eval_item_tables.sql` 적용 후에도 `evidences.evaluation_item_id`에 `evaluation_items`를 참조하는 FK 제약이 **존재하지 않음** (본 SPEC은 FK를 추가하지 않음 — spec.md §5 #6)
- `evidences` 테이블 schema가 SPEC-AX-EVID-001 정의 그대로 불변 (본 SPEC 구현이 `evidences`/EVID-001 코드를 수정하지 않음)
- `evaluation_items.id`와 `evidences.evaluation_item_id`가 둘 다 `character varying(64)`로 **타입 호환** (미래 FK 하드닝 SPEC이 추가 가능한 상태) — 단, 그 FK 추가 자체는 본 SPEC 범위 밖이며 미래 별도 SPEC이 수행

---

## §6. Korean Public-Sector Constraint Acceptance (표 포맷)

SPEC-AX-CTRL-001 §4 / SPEC-AX-EVID-001 §5 표 패턴. 한국 공공 6제약 중 본 SPEC 적용 항목.

| 제약 | 검증 기준 | 대응 AC | 측정 방법 |
|------|----------|---------|-----------|
| 데이터 주권 | 생성/조회/수정/검증 외부 호출 0건, 외부 SDK 미import | AC-EVALITEM-UBI-001 | 네트워크 spy + 정적 import 검사 |
| 언어 (한글 display_name) | `display_name`이 한글 평가항목명 수용 (VARCHAR(256) UTF-8) | AC-EVALITEM-001-1, AC-EVALITEM-004-3 | 한글 문자열 round-trip |
| 감사 가능성 | 모든 create/update → 동일 TX audit_logs 1건, 누락 0; resource_id=AUD-1 결정적 UUIDv5(원시 계층코드 아님), 실 식별자 DetailsJSON | AC-EVALITEM-UBI-002, AC-EVALITEM-003-1/2, AC-EVALITEM-003-E2-1 | testcontainers row count + UUIDv5 결정성 |
| cli-anonymous 기본값 | AuthN disabled 시 created_by/user_id='cli-anonymous' literal | AC-EVALITEM-UBI-003 | 컬럼 byte 비교 |
| 계층 무결성 | 자식 보유 항목 parent_id/level 불변, FK RESTRICT, hierarchy_code UNIQUE | AC-EVALITEM-UBI-004, AC-EVALITEM-002-2/3, AC-EVALITEM-004-1 | mutation guard + 제약 위반 검증 |
| 시간 제약 | 항목 생성/계층 조회 p99 < 50ms (단일 노드) | AC-EVALITEM-001-1, AC-EVALITEM-002-1 | 10회 반복 latency 측정 |

---

## §7. Edge Case Catalog

plan.md §7 R-EVALITEM-001~006 risk register 매핑.

| Edge Case | 대응 AC | Risk ID |
|-----------|--------|---------|
| 루트 항목 parent_id NULL | AC-EVALITEM-002-1 | (의도된 설계) |
| 존재하지 않는 parent_id로 생성 → INSERT 시 FK 위반 거부, 행 미생성, audit 미기록 (orphan 방지, S1 실패 경로) | AC-EVALITEM-001-S1-1 | R-EVALITEM-001 |
| 자식 보유 parent DELETE 거부 (ON DELETE RESTRICT, S1 실패 경로와 별개) | AC-EVALITEM-002-2 | R-EVALITEM-001 |
| hierarchy_code 중복 거부 | AC-EVALITEM-002-3 | R-EVALITEM-004 |
| 다단계 계층 순회 (재귀 서브트리는 범위 밖) | AC-EVALITEM-002-4 | R-EVALITEM-001 |
| cli-anonymous 기본값 (NULL 금지) | AC-EVALITEM-UBI-003 | R-EVALITEM-006 |
| audit INSERT 실패 → 항목+audit 양방향 rollback | AC-EVALITEM-003-3, AC-EVALITEM-UBI-002 | (감사 원자성) |
| id VARCHAR(64) 타입 (EVID-001 FK 호환) | AC-EVALITEM-001-3 | R-EVALITEM-002 |
| id blank / 64자 초과 / display_name blank / 중복 PK 거부 | AC-EVALITEM-001-4 | (입력 검증) |
| metadata JSONB opaque verbatim 영속 (등급기준 미해석) | AC-EVALITEM-001-O1-1 | (선택 기능, REQ-EVALITEM-001-O1) |
| 자식 존재 항목 parent_id/level 변경 거부 (계층 불변) | AC-EVALITEM-UBI-004, AC-EVALITEM-004-1 | R-EVALITEM-004 |
| status 열거 외/NULL 거부 | AC-EVALITEM-004-2 | (입력 검증) |
| 잎 노드 비-계층 속성 수정 + audit | AC-EVALITEM-004-3 | (정상 lifecycle) |
| evidences.evaluation_item_id FK 부재 유지 (out-of-scope 경계) | AC-EVALITEM-BOUNDARY-1 | R-EVALITEM-002 |
| store→audit 순환 의존 회피 (로컬 AuditTx) | AC-EVALITEM-003-1 | (아키텍처 불변식) |
| audit resource_id 타입 불일치 → AUD-1 결정적 UUIDv5 surrogate (원시 계층코드 아님, 실 식별자 DetailsJSON) | AC-EVALITEM-003-1, AC-EVALITEM-003-2, AC-EVALITEM-UBI-002 | R-EVALITEM-007 (RESOLVED) |
| audit resource_id 결정성 (동일 hierarchy_code → 동일 UUID, 재현 가능·충돌 없음) | AC-EVALITEM-003-E2-1 | R-EVALITEM-007 (RESOLVED) |
| 외부 저장 서비스 호출 부적격 | AC-EVALITEM-UBI-001 | R-EVALITEM-005 |

---

## §8. TDD RED/GREEN 매핑 요약

| AC | RED (실패 테스트 작성) | GREEN (최소 구현) |
|----|------------------------|-------------------|
| AC-EVALITEM-001-1 | InsertEvalItem 미구현 → undefined 메서드 | eval_item.go pgx InsertEvalItem + BeginEvalItemTx |
| AC-EVALITEM-001-2 | 자식 parent 미검증 (success 경로) | parent 사전 조회 + 자기참조 INSERT |
| AC-EVALITEM-001-S1-1 | 존재하지 않는 parent_id가 성공(잘못, orphan) | parent 사전 조회 + FK 제약 위반 거부, TX 미커밋 |
| AC-EVALITEM-001-3 | id UUID/auto-inc 가정 → schema 불일치 | VARCHAR(64) 계층 코드 PK DDL |
| AC-EVALITEM-001-4 | id/display_name 검증 부재 → blank가 성공 | store pre-INSERT 검증 + PK 위반 처리 |
| AC-EVALITEM-001-O1-1 | metadata 미저장/구조 강제(잘못) | JSONB 컬럼 semantic round-trip (opaque, byte 비교 금지 — JSONB 정규화) |
| AC-EVALITEM-002-1/4 | 자기참조 SELECT 미구현 / 계층 단절 (002-E1a/E1b 분할) | GetEvalItemsByParentID + parent_id 인덱스 |
| AC-EVALITEM-002-2 | 자식 있는 parent DELETE 성공(잘못) | FK ON DELETE RESTRICT |
| AC-EVALITEM-002-3 | hierarchy_code 중복 허용 | UNIQUE 인덱스 + store 검증 |
| AC-EVALITEM-003-1/2 | RecordEvalItem* 미구현 + (잘못) 원시 계층코드 resource_id 단언 → false RED | recorder.go 메서드 2개 + `uuid.NewSHA1(EvalItemAuditNamespace, hierarchyCode)` UUIDv5 surrogate, 실 식별자 DetailsJSON |
| AC-EVALITEM-003-E2-1 | 비결정적/랜덤 resource_id (audit 추적성 붕괴) | `uuid.NewSHA1`(고정 namespace + hierarchy_code) 결정적·재현 가능 산출 |
| AC-EVALITEM-003-3 | audit 실패 시 항목 잔존 | store tx.Rollback 양방향 |
| AC-EVALITEM-004-1 | successor 미확인 변경 허용 | UpdateEvalItem mutation guard |
| AC-EVALITEM-004-2 | status 임의값 허용 | status CHECK 제약 + store 검증 |
| AC-EVALITEM-004-3 | 잎 노드 update 미구현 | 단일 TX update + EVAL_ITEM_UPDATED audit |
| AC-EVALITEM-BOUNDARY-1 | evidences FK 존재 가정 → schema 불일치 | 본 SPEC FK 미추가 확인 (evidences 불변) |
| AC-EVALITEM-UBI-001~004 | sovereignty/audit/cli-anonymous/계층불변 위반 탐지 | 내부 pgx pool only + Recorder 재사용 + resolveUserID + mutation guard |

---

## §9. Definition of Done (Acceptance Phase)

모두 PASS 필요:

- [ ] §0: REQ-EVALITEM-UBI 전용 AC 4개 (UBI-001, UBI-002, UBI-003, UBI-004) 자동화 통과
- [ ] §1-§4: 4개 modal REQ AC 자동화 통과 (AC-EVALITEM-001-{1..4, S1-1, O1-1}, AC-EVALITEM-002-{1..4}, AC-EVALITEM-003-{1..3, E2-1}, AC-EVALITEM-004-{1..3})
- [ ] §5: 경계 AC (AC-EVALITEM-BOUNDARY-1) — evidences FK 부재 + evidences 미수정 확인
- [ ] §6: 한국 공공 6제약 검증 통과
- [ ] §7: 18개 edge case 모두 대응 AC로 검증 (REQ-EVALITEM-001-O1→AC-EVALITEM-001-O1-1, REQ-EVALITEM-001-S1 실패경로→AC-EVALITEM-001-S1-1, AUD-1 resource_id 전략/결정성→AC-EVALITEM-003-1/2/E2-1 전용 매핑)
- [ ] coverage ≥ 85% (go test -cover)
- [ ] golangci-lint default + gosec 0 issue
- [ ] `goleak.VerifyNone(t)` 모든 테스트 통과
- [ ] 기존 WorkflowStore/EvidenceStore/Recorder 특성화 회귀 0건
- [ ] @MX 태그 plan.md §5 매핑 완료
- [ ] manager-quality TRUST 5 통과
- [ ] evaluator-active per-sprint scoring 모두 ≥ 0.75 (strict profile, thorough harness)

**Total AC count**: 22 — (§0 UBI: 4 [UBI-001, UBI-002, UBI-003, UBI-004], §1: 6 [AC-EVALITEM-001-1..4, AC-EVALITEM-001-S1-1, AC-EVALITEM-001-O1-1], §2: 4 [AC-EVALITEM-002-1..4], §3: 4 [AC-EVALITEM-003-1..3, AC-EVALITEM-003-E2-1], §4: 3 [AC-EVALITEM-004-1..3], §5: 1 [AC-EVALITEM-BOUNDARY-1]). 각 modal REQ 모듈은 최소 3개 AC (≥2 요건 충족, §1은 6개·§2/§3은 4개). 버전 이력: v0.1.1 AC-EVALITEM-001-O1-1 추가 (D1, REQ-EVALITEM-001-O1 1:1 coverage, 19→20). v0.1.2 AC-EVALITEM-001-S1-1 추가 (LOW-2, REQ-EVALITEM-001-S1 실패 경로, 20→21). v0.1.3 AC-EVALITEM-003-E2-1 추가 (Run Phase 1 Decision 3, REQ-EVALITEM-003-E2 AUD-1 resource_id 결정성 — 신규 EARS sub-clause, 21→22); 동시에 AC-EVALITEM-003-1/2 + AC-EVALITEM-UBI-002 텍스트를 AUD-1 deterministic UUIDv5 기준으로 정정(원시 계층코드 resource_id false RED 제거). SPEC-AX-EVID-001 v0.1.x 점진 보강 패턴과 동일.
