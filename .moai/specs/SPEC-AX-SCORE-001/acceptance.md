# SPEC-AX-SCORE-001 Acceptance Criteria

> Format: Given / When / Then
> Methodology: TDD — 각 AC가 RED phase 단위 테스트로 1:1 매핑 (RED→GREEN 컬럼 명시)
> Tooling: Go 1.22 + testify/assert + testcontainers-go(postgres:16-pgvector) + goleak
> AC 명명: `AC-SCORE-{REQ}-{N}` (예: AC-SCORE-001-1, AC-SCORE-UBI-002)

본 문서는 SPEC-AX-SCORE-001의 5개 REQ 모듈(1 Ubiquitous 묶음 + 4 modal: 001/002/003/004)에 대한 acceptance criteria를 정의한다. 각 AC는 자동화 테스트로 검증 가능해야 하며 모호한 표현("적절히", "신속하게")을 사용하지 않는다. **§6 OPEN 결정(테이블 구조/audit-id/grade-threshold 위치/불변 규칙, plan.md §6)에 의존하는 AC는 그 의존성을 명시하며, Run Phase 1 strategy 확정 후 구체 단언을 정정한다 (SPEC-AX-EVAL-ITEM-001 §6 OPEN AC 흐름 동일).**

---

## §0. REQ-SCORE-UBI Transverse Invariants — Acceptance

§1-§4 modal REQ를 가로지르는 4개 Ubiquitous 불변 조건의 dedicated AC. SPEC-AX-EVID-001 §0 / SPEC-AX-EVAL-ITEM-001 §0 AC-*-UBI-* 패턴을 동일 구조로 적용한다.

### AC-SCORE-UBI-001 (Data Sovereignty — No External Call)

REQ 대응: REQ-SCORE-UBI-001 (데이터 주권 — 생성/조회/수정/롤업/등급 경로 외부 호출 0건, LLM 시뮬레이션 범위 밖).
TDD: RED 외부 host 차단 환경 → GREEN 내부 pgx pool + 내부 결정적 threshold만.

**Given**:
- 테스트 환경이 외부 네트워크 egress를 차단 (loopback/내부망 host만 허용)
- 점수 생성·롤업·등급 산출 경로가 `ScoreTx`/내부 threshold만 사용

**When**:
- 호출자가 신규 점수를 생성하고, `SumWeightedByEvaluationItem`을 호출하고, 등급을 산출한다

**Then**:
- 세 경로 모두 성공한다
- 생성/롤업/등급/감사 경로에서 외부 host로의 DNS 해석 또는 TCP 연결이 0건 (네트워크 spy 또는 정적 import 검사)
- `internal/store`, `internal/audit`가 외부 SaaS/LLM SDK(aws-sdk, 외부 LLM client 등) 미import — 등급 산출은 내부 결정적 threshold만 (REQ-SCORE-UBI-001, spec.md §5 #5)

### AC-SCORE-UBI-002 (Audit Completeness — Create & Update)

REQ 대응: REQ-SCORE-UBI-002 (감사 가능성 — 모든 create/update에 동일 TX `audit_logs` 1건).
TDD: RED audit row 부재 검증 → GREEN RecordScoreCreated/Updated 동일 TX 기입.

**Given**:
- PostgreSQL `scores` + `audit_logs` 테이블이 testcontainers fixture로 clean state
- AuthN disabled (Walking Skeleton 기본값)

**When**:
- 호출자가 신규 점수(`evaluation_item_id="AX-SAFETY-ORG-01-1"`, `score_value=85.00`)를 생성하고, 이어서 동일 점수의 `score_value`를 수정한다 (각각 별도 `ScoreTx`)

**Then**:
- 생성 후 `audit_logs`에 `action='SCORE_CREATED'`, `resource_type='score'` row 정확히 1개; `resource_id`는 점수 audit 식별자(Option 1 잠정 = `scores.id` UUID, 최종 plan.md §6), 실 맥락은 `details->>'evaluation_item_id'='AX-SAFETY-ORG-01-1'`로 검증
- 수정 후 `audit_logs`에 `action='SCORE_UPDATED'` row 정확히 1개
- 각 이벤트마다 `scores` 변경 + `audit_logs` row 1건이 동일 트랜잭션 커밋 (한쪽만 존재하는 상태 없음)

### AC-SCORE-UBI-003 (cli-anonymous Default)

REQ 대응: REQ-SCORE-UBI-003 (AuthN disabled 시 created_by/user_id = 'cli-anonymous').
TDD: RED user_id NULL/실사용자 검증 실패 → GREEN resolveUserID 재사용.

**Given**:
- Walking Skeleton 환경 (`AUTH_ENABLED=false`)
- 호출자가 인증 컨텍스트 없이 점수 생성

**When**:
- 점수(`evaluation_item_id="AX-SAFETY-UBI-003"`, `score_value=70.00`)를 생성한다 (no auth)

**Then**:
- `scores.created_by = 'cli-anonymous'` (정확히 literal, NULL 금지)
- `audit_logs.user_id = 'cli-anonymous'`
- 두 컬럼이 byte-identical (cross-table consistency, SPEC-AX-EVID-001 AC-EVID-UBI-003 / SPEC-AX-EVAL-ITEM-001 AC-EVALITEM-UBI-003 정합)

### AC-SCORE-UBI-004 (Score Immutability — status state-machine + append-only, Decision 4 RESOLVED)

REQ 대응: REQ-SCORE-UBI-004 (물리 삭제 0건 + status state-machine, Decision 4 RESOLVED).
TDD: RED 점수 물리 DELETE 허용 또는 CONFIRMED in-place 수정 허용(잘못) → GREEN 무삭제 + DRAFT/CONFIRMED/SUPERSEDED state-machine.

**Given**:
- 점수 `id=<UUID-A>` (`status='CONFIRMED'`, `score_value=85.00`)이 존재

**When**:
- (a) `UpdateScore(<UUID-A>, score_value=90.00)` (CONFIRMED in-place 수정 시도)
- (b) CONFIRMED 점수 정정을 수행한다 (Decision 4 RESOLVED 규칙: 신규 행 INSERT + 구 행 `CONFIRMED→SUPERSEDED`)
- (c) 어떤 status의 점수든 물리 DELETE를 시도한다

**Then** (a):
- CONFIRMED 행 score 필드 in-place 수정이 구조화 에러로 거부됨 (`<UUID-A>.score_value` 불변 — Decision 4)

**Then** (b):
- 신규 행 `id=<UUID-B>` INSERT (정정값) + `<UUID-A>.status` = `CONFIRMED → SUPERSEDED` 전이 (동일 `ScoreTx`)
- `<UUID-A>` 행은 보존(SUPERSEDED), `score_value` 동결; `SUPERSEDED`는 terminal
- 신규 행 INSERT + 구 행 전이 각각 `audit_logs` row 1건 (동일 TX, REQ-SCORE-UBI-002 정합)

**Then** (c):
- `scores` 테이블에서 어떤 행의 물리 DELETE 0건 (`DeleteScore` 메서드 미존재 — 무삭제 보장, REQ-SCORE-UBI-004)
- 모든 변경이 `audit_logs`로 추적 가능

> 비고: Decision 4 RESOLVED (plan.md §6.4, strategy.md §A) 확정 규칙으로 본 AC 단언 확정 — 전이표 `DRAFT→DRAFT`/`DRAFT→CONFIRMED`/`CONFIRMED→SUPERSEDED`. DRAFT 행 in-place 가변 검증은 AC-SCORE-001-S2가 담당. SPEC-AX-EVAL-ITEM-001 v0.1.3 Decision 정정 패턴 동일.

---

## §1. REQ-SCORE-001 점수 데이터 모델 & Store — Acceptance

### AC-SCORE-001-1 (Happy Path: 점수 생성 + 감사 원자적 커밋)

TDD: RED InsertScore 미구현 → GREEN score.go pgx 구현.

**Given**:
- `scores`/`audit_logs` 테이블 clean state (testcontainers)
- 신규 점수 `evaluation_item_id="AX-SAFETY-ORG-01-1"`, `score_value=85.00`, `evidence_id=NULL`, `weight=0.1500`

**When**:
- 호출자가 `BeginScoreTx` → `InsertScore` → `RecordScoreCreated` → `Commit`을 수행한다

**Then**:
- `scores`에 정확히 1 row: `evaluation_item_id='AX-SAFETY-ORG-01-1'`, `score_value=85.00`, `status='DRAFT'` (DEFAULT), `created_by='cli-anonymous'`, `id`는 생성된 UUID
- `audit_logs`에 `SCORE_CREATED` 1 row (동일 TX)
- 응답 < 50ms p99 (10회 반복, research.md §8 한국 공공 시간 제약)

### AC-SCORE-001-2 (evidence_id UUID stub — EVID-001 호환, FK 없음)

TDD: RED evidence_id가 FK 가정 테스트 실패 → GREEN UUID nullable stub (FK 없음).

**Given**:
- `scores` 테이블이 `0004_score_tables.sql`로 생성됨
- 유효한 UUID `evidence_id="11111111-1111-1111-1111-111111111111"` (대응 `evidences` 행은 존재하지 않아도 됨 — FK 없는 stub)

**When**:
- 해당 `evidence_id`로 점수를 생성하고, information_schema로 `scores.evidence_id` 컬럼 타입 + FK 제약을 조회한다

**Then**:
- 생성이 성공한다 (`evidences`에 해당 id가 없어도 거부되지 않음 — FK 없는 stub, REQ-SCORE-001-S1)
- `information_schema.columns`에서 `scores.evidence_id`의 `data_type='uuid'`, `is_nullable='YES'`
- `information_schema.referential_constraints`에 `scores.evidence_id → evidences` FK 제약 **부재** (§1.4 HARD — 본 SPEC은 FK 추가 안 함)
- `evidences.id`도 `uuid` — 두 컬럼 타입 호환 (미래 FK 하드닝 SPEC 가능 상태, 단 추가는 범위 밖 — AC-SCORE-BOUNDARY-1)

### AC-SCORE-001-3 (evaluation_item_id 타입 — VARCHAR(64), EVAL-ITEM-001 FK 호환)

TDD: RED evaluation_item_id가 UUID/FK 가정 테스트 실패 → GREEN VARCHAR(64) stub (FK 없음).

**Given**:
- `scores` 테이블이 `0004_score_tables.sql`로 생성됨
- 계층 코드 형태 `evaluation_item_id="AX-SAFETY-ORG-01-1"` (UUID 아님, `evaluation_items` 행 존재 불요 — FK 없는 stub)

**When**:
- 해당 계층 코드로 점수를 생성하고, information_schema로 `scores.evaluation_item_id` 컬럼 타입 + FK 제약을 조회한다

**Then**:
- 생성이 성공한다 (임의 계층 코드 문자열, UUID 형식 강제 없음, 대응 `evaluation_items` 행 불요)
- `information_schema.columns`에서 `scores.evaluation_item_id`의 `data_type='character varying'`, `character_maximum_length=64`
- `scores.evaluation_item_id → evaluation_items` FK 제약 **부재** (§1.4 HARD)
- `evaluation_items.id`도 `character varying(64)` (SPEC-AX-EVAL-ITEM-001 §1.4) — 두 컬럼 타입 호환 (미래 FK 추가 시 타입 불일치 0건, 단 추가는 범위 밖)

### AC-SCORE-001-4 (Edge — evaluation_item_id blank / 64자 초과 / score_value 비수치 / evidence_id 비-UUID 거부)

TDD: RED 검증 부재 → GREEN store pre-INSERT 검증.

**Given**:
- `scores` clean state

**When**:
- 호출자가 (a) `evaluation_item_id=""` (blank), 또는 (b) `evaluation_item_id` 65자, 또는 (c) `score_value` 비수치/누락, 또는 (d) `evidence_id="not-a-uuid"` (비-UUID 문자열)으로 생성을 시도한다

**Then**:
- 각 경우 구조화 검증 에러 반환 (client error)
- `scores` 변화 0건, `audit_logs` 변화 0건 (트랜잭션 미커밋)
- 서버 로그 레벨 = INFO (client error, not server defect — REQ-SCORE-001-U1)

### AC-SCORE-001-S2 (status state-machine — DRAFT in-place 가변 / CONFIRMED 불변, Decision 4 RESOLVED)

REQ 대응: REQ-SCORE-001-S2 (Decision 4 RESOLVED status state-machine — DRAFT 가변, CONFIRMED 불변+정정=신규행). AC-SCORE-UBI-004와 상보: UBI-004는 CONFIRMED 정정 경로+무삭제, 본 AC는 DRAFT 가변 경로 + CONFIRMED 거부 단언.
TDD: RED DRAFT 수정 거부 또는 CONFIRMED in-place 허용(잘못) → GREEN `validateScoreStatusTransition` 가드.

**Given**:
- 점수 `id=<UUID-D>` (`status='DRAFT'`, `score_value=70.00`)와 점수 `id=<UUID-C>` (`status='CONFIRMED'`, `score_value=88.00`)이 존재

**When**:
- (a) `UpdateScore(<UUID-D>, score_value=75.00, weight=0.2000)` (DRAFT in-place 수정)
- (b) `UpdateScore(<UUID-C>, score_value=95.00)` (CONFIRMED score 필드 수정 시도)
- (c) `UpdateScore(<UUID-D>, status='CONFIRMED')` (DRAFT→CONFIRMED 전이)

**Then** (a):
- `<UUID-D>` 행이 in-place 갱신됨 (`score_value=75.00`, `weight=0.2000` — DRAFT 가변, Decision 4 rule 1)
- 동일 `ScoreTx` 내 `SCORE_UPDATED` audit row 1건 (REQ-SCORE-UBI-002 정합)

**Then** (b):
- 구조화 에러로 거부됨 (CONFIRMED 행 score 필드 불변 — Decision 4 rule 2), `<UUID-C>.score_value` 변경 0건
- `scores`/`audit_logs` 변화 0건 (트랜잭션 미커밋)

**Then** (c):
- `<UUID-D>.status` = `DRAFT → CONFIRMED` 전이 성공 (허용 전이, Decision 4 전이표)
- 동일 TX 내 `SCORE_UPDATED` audit row 1건

> 비고: 본 AC는 plan.md §6.4 Decision 4 RESOLVED 규칙으로 확정. CONFIRMED 정정=신규행+SUPERSEDED 경로 및 물리 삭제 0건은 AC-SCORE-UBI-004가 담당 (상보 coverage, 중복 아님).

### AC-SCORE-001-O1-1 (Optional — metadata JSONB opaque verbatim 영속)

REQ 대응: REQ-SCORE-001-O1 (caller가 metadata JSONB payload 제공 시 구조 해석 없이 verbatim opaque 영속).
TDD: RED metadata 미저장/구조 강제(잘못) → GREEN JSONB 컬럼 semantic round-trip.
주의: SPEC-AX-EVAL-ITEM-001 AC-EVALITEM-001-O1-1 패턴 동일 — §1 전용 AC로 O1 1:1 coverage.

**Given**:
- `scores`/`audit_logs` clean state (testcontainers)
- caller가 신규 점수 생성 시 임의 구조 metadata JSONB 제공 (예: `{"채점_코멘트": "현장 점검 반영", "draft_threshold": {"S": "...", "A": "..."}}`)

**When**:
- 호출자가 해당 metadata payload를 포함하여 `BeginScoreTx` → `InsertScore` → `Commit`을 수행한다

**Then**:
- 점수가 정상 생성되고 `scores.metadata`가 입력 payload와 **semantically equivalent JSONB**로 저장됨 (필드 누락/값 변형 0건 — PostgreSQL JSONB value-equality 기준. 키 순서·공백·중복 키 제거는 JSONB 정규화 범위이므로 비교 대상 아님)
- subsystem이 metadata 구조를 **해석/검증하지 않음**: 등급기준 스키마 검증 없음, 필수 키 강제 없음, 임의 중첩/빈 객체(`{}`)/NULL 모두 거부 없이 수용 (REQ-SCORE-001-O1 — opaque placeholder)
- 동일 TX 내 `SCORE_CREATED` audit row 1건 (REQ-SCORE-UBI-002 정합)
- 본 AC는 등급기준(scoring rubric) 저장 구조를 검증하지 **않는다** (풀 rubric 미설계 — spec.md §5 #2)

> 비고: 테스트 구현은 metadata round-trip 검증 시 **byte-level string 비교를 사용하지 말 것** (PostgreSQL JSONB 정규화 — false RED 유발). semantic JSON equality(`map[string]any` unmarshal + `reflect.DeepEqual` 또는 `jsonEqual()`)로 수행. SPEC-AX-EVAL-ITEM-001 AC-EVALITEM-001-O1-1 LOW-1 정정 동일.

---

## §2. REQ-SCORE-002 가중 롤업 (Minimal) — Acceptance

### AC-SCORE-002-1 (Happy Path: 단일 레벨 가중 합산 정확성)

REQ 대응: REQ-SCORE-002-E1.
TDD: RED SumWeightedByEvaluationItem 미구현 → GREEN Σ(score×weight) 단일 레벨.

**Given**:
- 평가항목 `evaluation_item_id="AX-SAFETY-ORG-01"`에 raw-level 자식 점수 3건: (90.00, 0.5000), (80.00, 0.3000), (70.00, 0.2000)
- 기대 가중합 = 90×0.5 + 80×0.3 + 70×0.2 = 45.00 + 24.00 + 14.00 = 83.00

**When**:
- `SumWeightedByEvaluationItem("AX-SAFETY-ORG-01")`을 단일 read 트랜잭션으로 호출한다

**Then**:
- 반환값이 `83.00` (DECIMAL 정밀도 — 부동소수 오차 0, 단일 레벨)
- 조회가 `scores_evaluation_item_id_idx` 인덱스 사용 (EXPLAIN 검증, p99 < 50ms)
- 깊은 재귀 서브트리 집계는 수행하지 않음 (단일 레벨만 — spec.md §5 #4 범위 밖)

### AC-SCORE-002-2 (Edge — NULL weight 누락 가중치 결정적 처리)

REQ 대응: REQ-SCORE-002-U1 (NULL weight가 집계를 조용히 왜곡하지 않음, 결정적 단일 고정 정책).
TDD: RED NULL weight가 0 또는 1로 조용히 강제됨(잘못) → GREEN 결정적 단일 정책 (제외 또는 구조화 에러).

**Given**:
- 평가항목 `evaluation_item_id="AX-SAFETY-ORG-02"`에 raw-level 점수 2건: (90.00, 0.6000), (80.00, NULL weight)

**When**:
- `SumWeightedByEvaluationItem("AX-SAFETY-ORG-02")`을 2회 호출한다

**Then**:
- NULL weight 행이 조용히 임의 기본값(0 또는 1)으로 강제되어 결과를 왜곡하지 **않음** (REQ-SCORE-002-U1)
- subsystem이 단일 고정 결정적 정책을 적용: 해당 행을 결정적으로 제외하거나 구조화 "missing weight" 에러를 surface
- 2회 호출이 동일 입력에 동일 결과 (결정성 — 비결정적 분기 0건)

> 비고: §6 Human Gate 4건은 테이블/audit-id/grade-threshold/불변이며, NULL-weight 정책(제외 vs 에러)은 그 4건에 포함되지 않는 **구현 세부**로 Sprint S4 구현 시 단일 결정적 규칙으로 고정한다(spec.md REQ-SCORE-002-U1, plan.md §4 S4). 본 AC의 "조용한 왜곡 0건 + 결정성" 핵심 단언은 구현 형태와 무관하게 고정 — 두 형태(제외/에러) 모두 본 Then을 만족한다.

---

## §3. REQ-SCORE-003 등급 산출 (Minimal Threshold) — Acceptance

### AC-SCORE-003-1 (Happy Path: score → letter 결정적 매핑)

REQ 대응: REQ-SCORE-003-E1 (deterministic `grade_thresholds` table lookup, 외부 호출 0 — Decision 3 RESOLVED).
TDD: RED 등급 매핑 미구현 또는 metadata JSONB 의존(잘못) → GREEN `grade_thresholds` 테이블 결정적 lookup.

**Given**:
- `grade_thresholds` 테이블(Decision 3 RESOLVED)에 `scope='default'` 행 5건: (S,90.00,gte) (A,80.00,gte) (B,70.00,gte) (C,60.00,gte) (D,0.00,gte)
- 집계 점수 값 `83.00`, scope=`'default'`

**When**:
- 호출자가 scope=`'default'`, value=`83.00`에 대한 letter 등급을 요청한다

**Then**:
- `grade_thresholds` 테이블을 `SELECT ... WHERE scope='default'` 후 S→D 내림차순 `min_value` 스캔하여 `boundary_rule='gte'` 기준 `83.00 >= 80.00` (A 행) 매칭 → 등급 `A` 반환
- metadata JSONB를 등급 산출에 사용하지 **않음** (Decision 3 — REQ-SCORE-001-O1 opacity 계약 정합)
- 등급 산출 경로에서 외부 LLM/SaaS 호출 0건 (내부 결정적 테이블 lookup만 — REQ-SCORE-UBI-001 / spec.md §5 #5)
- 동일 (scope, value) 재요청 시 동일 등급 (결정성)

### AC-SCORE-003-2 (Edge — 경계값 결정성 + threshold 미설정 결정적 실패)

REQ 대응: REQ-SCORE-003-S1 (`grade_thresholds` 0행 → 결정적 실패) + REQ-SCORE-003-U1 (`boundary_rule` 경계 결정성). Decision 3 RESOLVED.
TDD: RED 경계값 비결정/미설정 시 임의 등급 fabricate(잘못) → GREEN `boundary_rule` 결정적 경계 + 0행 구조화 실패.

**Given**:
- (a) `grade_thresholds`에 `scope='default'` A 행 `(A, min_value=80.00, boundary_rule='gte')` 존재, 집계 값이 경계값과 정확히 일치(`80.00`)
- (b) 요청 scope=`'unknown-scope'`에 `grade_thresholds` 행이 **0건**

**When**:
- (a) scope=`'default'`, `80.00`에 대한 등급을 2회 요청한다
- (b) scope=`'unknown-scope'`에 대해 등급을 요청한다

**Then** (a):
- `boundary_rule='gte'`이므로 `80.00 >= 80.00` → 등급 `A` 반환 (경계 포함, 기획재정부 편람 "임계값=하한" 의미론 정합)
- 두 요청 모두 동일 등급 `A` (경계값 비결정성 0건 — `boundary_rule` 명시 컬럼 기반 결정적, Decision 3)

**Then** (b):
- `SELECT ... WHERE scope='unknown-scope'` → 0행 → 구조화 "grade thresholds unavailable" 에러 surface (REQ-SCORE-003-S1)
- letter 등급을 임의 생성하지 **않음** (등급 fabricate 0건)
- 서버 로그 레벨 = INFO 또는 명시적 설정 누락 에러 (client/config error, not silent default)

> 비고: Decision 3 RESOLVED (`grade_thresholds` 테이블 + `boundary_rule` 컬럼, 기본 `gte`) 확정으로 본 AC 단언 확정. plan.md §6.3 / strategy.md §A Decision 3.

---

## §4. REQ-SCORE-004 감사 연계 — Acceptance

### AC-SCORE-004-1 (RecordScoreCreated / RecordScoreUpdated Audit Row)

REQ 대응: REQ-SCORE-004-E1.
TDD: RED RecordScore* 미구현 → GREEN recorder.go 메서드 + audit.Event 구성.

**Given**:
- `audit_logs` clean state, Recorder(`authEnabled=false`)
- 점수 `id=<UUID>`, `evaluation_item_id="AX-SAFETY-ORG-01-1"`, `level="raw"`

**When**:
- 점수 생성 TX가 `Recorder.RecordScoreCreated(ctx, tx, score, userID="")` 호출, 이어서 수정 TX가 `Recorder.RecordScoreUpdated(...)` 호출

**Then**:
- 생성: `audit.Event`의 `Action="SCORE_CREATED"`, `ResourceType="score"`, `UserID="cli-anonymous"`(resolveUserID), `Timestamp` NOT NULL
- `Event.ResourceID` (= `audit_logs.resource_id`, `uuid.UUID NOT NULL`)가 **`score.id`와 byte-identical** (Decision 2 RESOLVED Option 1 직접 대입, surrogate 아님; `resource_id != uuid.Nil` 항상 성립 — `scores.id`가 UUID PK이므로 비-UUID→`uuid.Nil` 강등 구조적 부재)
- 실제 비즈니스 맥락은 `DetailsJSON`: `details->>'score_id'`, `details->>'evaluation_item_id'='AX-SAFETY-ORG-01-1'`, `details->>'level'='raw'` (`evidence_id`/`grade`/`status`도 해당 시 포함)
- `uuid.NewSHA1` 호출 0건, 신규 `ScoreAuditNamespace` 상수 0건 (Decision 2 — AUD-1 surrogate 미적용)
- 수정: `Action="SCORE_UPDATED"`, 동일 식별 규칙(`ResourceID = score.id`)
- 동일 `AuditTx`로 INSERT (store→audit 순환 의존 없음 — `audit` 패키지가 `store` 미import)

> 비고: Decision 2 RESOLVED (plan.md §6.2, strategy.md §A) — `scores.id` UUID PK ⟹ Option 1 직접 대입 확정. EVAL-ITEM-001이 겪은 audit resource_id 타입 불일치(VARCHAR PK→`uuid.Nil`)가 구조적으로 부재하므로 AC-EVALITEM-003-1 AUD-1 surrogate 패턴 미적용 (본 SPEC이 EVAL-ITEM보다 단순).

### AC-SCORE-004-2 (Edge — Audit Fail → 점수+감사 양방향 Rollback 원자성)

REQ 대응: REQ-SCORE-004-U1.
TDD: RED audit 실패 시 scores row 잔존(잘못) → GREEN tx.Rollback 양방향.

**Given**:
- `scores`/`audit_logs` clean state
- Test harness가 `audit_logs` INSERT에 fault injection (CHECK constraint violation 강제)

**When**:
- 점수 생성 TX가 (a) `scores` INSERT 성공 후 (b) `audit_logs` INSERT가 실패한다
- store가 `tx.Rollback(ctx)` 호출

**Then**:
- `scores` 테이블에 row **존재하지 않음** (score INSERT도 rollback)
- `audit_logs` 테이블에 row 없음 (애초에 실패)
- 반환 에러 = wrapped audit insertion failure
- 부분 커밋 0건 (all-or-nothing, REQ-SCORE-004-U1)
- `goleak.VerifyNone(t)` 통과
- SPEC-AX-EVID-001 AC-EVID-003-3 / SPEC-AX-EVAL-ITEM-001 AC-EVALITEM-003-3 패턴의 점수 도메인 대응

---

## §5. Boundary — EVID-001 / EVAL-ITEM-001 Out-of-Scope Confirmation

### AC-SCORE-BOUNDARY-1 (evidences / evaluation_items FK 부재 + 미수정 유지)

REQ 대응: spec.md §5 Exclusion #3 / §7 Out of Scope (EVID/EVAL-ITEM FK 하드닝 = 본 SPEC 범위 밖).
TDD: RED `scores`→`evidences`/`evaluation_items` FK 존재 가정 테스트 → GREEN FK 부재 확인 (본 SPEC이 FK를 추가하지 않음).

**Given**:
- `0004_score_tables.sql` 적용으로 `scores` 테이블이 생성됨
- SPEC-AX-EVID-001 `evidences`(UUID PK) + SPEC-AX-EVAL-ITEM-001 `evaluation_items`(VARCHAR(64) PK)가 이미 존재

**When**:
- `0004_score_tables.sql` 적용 전후로 information_schema(`table_constraints`, `referential_constraints`)를 쿼리하여 `scores.evaluation_item_id`→`evaluation_items(id)` 및 `scores.evidence_id`→`evidences(id)` FK 제약 존재 여부와 `evidences`/`evaluation_items` 스키마를 확인한다

**Then**:
- `0004` 적용 후에도 `scores.evaluation_item_id`/`scores.evidence_id`에 FK 제약이 **존재하지 않음** (본 SPEC은 FK를 추가하지 않음 — spec.md §5 #3)
- `evidences`/`evaluation_items` 테이블 스키마 + `0002`/`0003` 마이그레이션이 SPEC-AX-EVID-001/SPEC-AX-EVAL-ITEM-001 정의 그대로 **불변** (본 SPEC 구현이 EVID/EVAL-ITEM 코드·마이그레이션을 수정하지 않음)
- `scores.evaluation_item_id`↔`evaluation_items.id` 둘 다 `character varying(64)`, `scores.evidence_id`↔`evidences.id` 둘 다 `uuid`로 **타입 호환** (미래 FK 하드닝 SPEC 추가 가능 상태) — 단, FK 추가 자체는 본 SPEC 범위 밖이며 미래 별도 SPEC이 수행

---

## §6. Korean Public-Sector Constraint Acceptance (표 포맷)

SPEC-AX-EVID-001 §5 / SPEC-AX-EVAL-ITEM-001 §6 표 패턴. 한국 공공 6제약 중 본 SPEC 적용 항목.

| 제약 | 검증 기준 | 대응 AC | 측정 방법 |
|------|----------|---------|-----------|
| 데이터 주권 | 생성/조회/수정/롤업/등급 외부 호출 0건, 외부 LLM/SaaS SDK 미import (LLM 시뮬레이션 범위 밖) | AC-SCORE-UBI-001 | 네트워크 spy + 정적 import 검사 |
| 언어 (한글 metadata) | metadata JSONB가 한글 채점 코멘트 수용 (UTF-8) | AC-SCORE-001-O1-1 | 한글 문자열 round-trip |
| 감사 가능성 | 모든 create/update → 동일 TX audit_logs 1건, 누락 0; resource_id=`score.id` 직접 UUID(Decision 2 RESOLVED Option 1, surrogate 아님), 실 맥락 DetailsJSON | AC-SCORE-UBI-002, AC-SCORE-004-1 | testcontainers row count |
| cli-anonymous 기본값 | AuthN disabled 시 created_by/user_id='cli-anonymous' literal | AC-SCORE-UBI-003 | 컬럼 byte 비교 |
| 무결성 (불변/무삭제) | `scores` 물리 삭제 0건, status state-machine (DRAFT 가변/CONFIRMED 불변/정정=신규행+SUPERSEDED, Decision 4 RESOLVED) | AC-SCORE-UBI-004, AC-SCORE-001-S2 | DELETE 부재 + status 전이 검증 |
| 시간 제약 | 점수 생성/가중 롤업 p99 < 50ms (단일 레벨) | AC-SCORE-001-1, AC-SCORE-002-1 | 10회 반복 latency 측정 |

---

## §7. Edge Case Catalog

plan.md §7 R-SCORE-001~008 risk register 매핑.

| Edge Case | 대응 AC | Risk ID |
|-----------|--------|---------|
| 점수 생성 + audit 동일 TX 원자 커밋 | AC-SCORE-001-1, AC-SCORE-UBI-002 | (감사 원자성) |
| evidence_id UUID stub (EVID-001 호환, FK 없음) | AC-SCORE-001-2 | R-SCORE-002 |
| evaluation_item_id VARCHAR(64) stub (EVAL-ITEM-001 호환, FK 없음) | AC-SCORE-001-3 | R-SCORE-002 |
| evaluation_item_id blank/64자 초과 / score_value 비수치 / evidence_id 비-UUID 거부 | AC-SCORE-001-4 | (입력 검증) |
| metadata JSONB opaque verbatim 영속 (rubric 미해석) | AC-SCORE-001-O1-1 | R-SCORE-008 |
| 단일 레벨 가중 합산 정확성 Σ(score×weight) | AC-SCORE-002-1 | (정상 롤업, 최소) |
| NULL weight 누락 가중치 결정적 처리 (조용한 왜곡 0건) | AC-SCORE-002-2 | R-SCORE-003 |
| score → letter 결정적 매핑 (외부 호출 0) | AC-SCORE-003-1 | R-SCORE-008 |
| 등급 경계값 결정성 + threshold 미설정 결정적 실패 (fabricate 0건) | AC-SCORE-003-2 | R-SCORE-004 |
| audit INSERT 실패 → 점수+audit 양방향 rollback | AC-SCORE-004-2, AC-SCORE-UBI-002 | R-SCORE-005 |
| store→audit 순환 의존 회피 (로컬 AuditTx) | AC-SCORE-004-1 | (아키텍처 불변식) |
| `scores` 물리 삭제 0건 + CONFIRMED 정정=신규행+SUPERSEDED (Decision 4) | AC-SCORE-UBI-004 | (무결성, §6.4 RESOLVED) |
| status state-machine: DRAFT in-place 가변 / CONFIRMED 불변 전이 (Decision 4) | AC-SCORE-001-S2 | (무결성 state-machine, §6.4 RESOLVED) |
| evidences/evaluation_items FK 부재 + 미수정 유지 (out-of-scope 경계) | AC-SCORE-BOUNDARY-1 | R-SCORE-002, R-SCORE-007 |
| cli-anonymous 기본값 (NULL 금지) | AC-SCORE-UBI-003 | (입력 검증) |
| 외부 LLM/저장 서비스 호출 부적격 (LLM 시뮬레이션 범위 밖) | AC-SCORE-UBI-001 | R-SCORE-008 |

---

## §8. TDD RED/GREEN 매핑 요약

| AC | RED (실패 테스트 작성) | GREEN (최소 구현) |
|----|------------------------|-------------------|
| AC-SCORE-001-1 | InsertScore 미구현 → undefined 메서드 | score.go pgx InsertScore + BeginScoreTx (pg_store.go) |
| AC-SCORE-001-2 | evidence_id FK 가정 → schema 불일치 | UUID nullable stub 컬럼 (FK 없음) DDL |
| AC-SCORE-001-3 | evaluation_item_id UUID/FK 가정 → schema 불일치 | VARCHAR(64) stub 컬럼 (FK 없음) DDL |
| AC-SCORE-001-4 | 입력 검증 부재 → blank/비수치가 성공 | store pre-INSERT 검증 |
| AC-SCORE-001-S2 | DRAFT 수정 거부 또는 CONFIRMED in-place 허용(잘못) | `validateScoreStatusTransition` 가드 (DRAFT 가변/CONFIRMED 불변/전이표, Decision 4) |
| AC-SCORE-001-O1-1 | metadata 미저장/구조 강제(잘못) | JSONB 컬럼 semantic round-trip (opaque, byte 비교 금지) |
| AC-SCORE-002-1 | SumWeightedByEvaluationItem 미구현 | 단일 레벨 Σ(score×weight) DECIMAL + evaluation_item_id 인덱스 |
| AC-SCORE-002-2 | NULL weight 0/1 조용히 강제(잘못) | 단일 고정 결정적 정책 (제외 또는 에러, S4 구현 고정) |
| AC-SCORE-003-1 | 등급 매핑 미구현 또는 metadata JSONB 의존(잘못) | `grade_thresholds` 테이블 결정적 lookup (Decision 3) |
| AC-SCORE-003-2 | 경계값 비결정/미설정 시 fabricate(잘못) | `boundary_rule` 결정적 경계 + 0행 구조화 "unavailable" 실패 (Decision 3) |
| AC-SCORE-004-1 | RecordScore* 미구현 또는 surrogate 가정(잘못) | recorder.go 메서드 2개 + audit.Event (resource_id = score.id 직접 UUID, namespace 상수 0, Decision 2) |
| AC-SCORE-004-2 | audit 실패 시 점수 잔존 | store tx.Rollback 양방향 |
| AC-SCORE-BOUNDARY-1 | scores→evidences/evaluation_items FK 존재 가정 → schema 불일치 | 본 SPEC FK 미추가 확인 (EVID/EVAL-ITEM 불변) |
| AC-SCORE-UBI-001~004 | sovereignty/audit/cli-anonymous/무삭제 위반 탐지 | 내부 pgx pool + 내부 threshold + Recorder 재사용 + resolveUserID + 무삭제 |

---

## §9. Definition of Done (Acceptance Phase)

모두 PASS 필요:

- [ ] §0: REQ-SCORE-UBI 전용 AC 4개 (UBI-001~004) 자동화 통과
- [ ] §1-§4: 4개 modal REQ AC 자동화 통과 (AC-SCORE-001-{1..4, S2, O1-1}, AC-SCORE-002-{1,2}, AC-SCORE-003-{1,2}, AC-SCORE-004-{1,2})
- [ ] §5: 경계 AC (AC-SCORE-BOUNDARY-1) — evidences/evaluation_items FK 부재 + 미수정 확인
- [ ] §6: 한국 공공 6제약 검증 통과
- [ ] §7: 16개 edge case 모두 대응 AC로 검증 (§7 Edge Case Catalog 표 물리적 데이터 행 수 = 16 — v0.1.2 status state-machine row 추가로 15→16)
- [ ] §6 4건 RESOLVED (v0.1.2): AC-SCORE-UBI-004/001-S2(Decision 4 status state-machine), 003-1/003-2(Decision 3 grade_thresholds), 004-1(Decision 2 직접 UUID) 구체 단언 확정. 002-2(NULL weight)는 §6 4건 외 S4 구현 고정 (핵심 불변 단언은 구현 무관 고정)
- [ ] coverage ≥ 85% (go test -cover)
- [ ] golangci-lint default + gosec 0 issue
- [ ] `goleak.VerifyNone(t)` 모든 테스트 통과
- [ ] 기존 WorkflowStore/EvidenceStore/EvalItemStore/Recorder 특성화 회귀 0건
- [ ] @MX 태그 plan.md §5 매핑 완료
- [ ] manager-quality TRUST 5 통과
- [ ] evaluator-active per-sprint scoring 모두 ≥ 0.75 (strict profile, thorough harness)

**Total AC count**: 17 — (§0 UBI: 4 [UBI-001~004], §1: 6 [AC-SCORE-001-1..4, AC-SCORE-001-S2, AC-SCORE-001-O1-1], §2: 2 [AC-SCORE-002-1,2], §3: 2 [AC-SCORE-003-1,2], §4: 2 [AC-SCORE-004-1,2], §5: 1 [AC-SCORE-BOUNDARY-1]). 분해 합 = 4+6+2+2+2+1 = 17 = 물리적 AC heading 17개(acceptance.md `### AC-SCORE-` heading 직접 카운트 — UBI-001/002/003/004, 001-1/2/3/4/S2/O1-1, 002-1/2, 003-1/2, 004-1/2, BOUNDARY-1). v0.1.2: AC-SCORE-001-S2 신규 추가(Decision 4 status state-machine DRAFT/CONFIRMED — AC 16→17), §7 edge case 15→16(status state-machine row 추가). 각 modal REQ 모듈은 최소 2개 AC (≥2 요건 충족, §1은 6개). 5개 REQ 모듈 = 1 Ubiquitous 묶음(UBI-001~004) + 4 modal(001/002/003/004) — 모듈 ≤5. §9 DoD §1-§4 enumeration(modal 12 + UBI 4 + BOUNDARY 1 = 17)·spec-compact.md count·본 Total·물리 heading이 모두 17로 일치(single source of truth); §7 edge=16. §6 4건 RESOLVED 반영 완료 (v0.1.2, strategy.md §A + Human Gate). SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 v0.1.x 점진 보강 패턴과 동일.
