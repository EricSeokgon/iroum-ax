---
id: SPEC-AX-EVID-001
version: 0.1.2
status: draft
created: 2026-05-18
updated: 2026-05-18
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.2 (2026-05-18): Run Phase 1 전략 승인 반영 (manager-strategy 분석 strategy.md + 사용자 Human Gate 승인). (1) 저장 전략 OPEN DECISION → **RESOLVED: `database_blob`** 확정 (§3.5 REQ-EVID-004, plan.md §6; 가중 8.45 vs filesystem 5.45 vs MinIO 3.85, 동일 TX 원자성 구조적 보장 + orphan-cleanup 제거 + 망분리 외부 의존 0건; 추상화·enum 유지로 post-PoC 무중단 전환 대비). (2) DDL/엔티티에 `file_content BYTEA` (nullable) 컬럼 추가 (§3.2, plan.md §3 — database_blob 시 바이너리 저장처, 타 전략 NULL). (3) Phantom-path 교정: evidence TX 진입점 `postgres.go`(Sprint-0 死 스텁) → `pg_store.go`(실 `PgWorkflowStore.pool`) (§2.1, plan.md §2). (4) EvidenceBlobStore 정합: database_blob 시 blob bytes는 인터페이스 미통과, `EvidenceTx.InsertEvidence(file_content)` 경로로 동일 TX 저장, 인터페이스는 논리 location 기록만 (§3.5 REQ-EVID-004-O1). (5) config 기본값 명시 (§4: `EVIDENCE_STORAGE_STRATEGY=database_blob`, `EVIDENCE_MAX_FILE_BYTES=52428800`, `EVIDENCE_DUPLICATE_SIGNAL_ENABLED=false`). (작성자: ircp)
- 0.1.1 (2026-05-18): plan-auditor iter 1 리뷰 반영 (PASS 0.88, minor 결함 2건 + cosmetic 정정). D1(REQ-EVID-001-O1 coverage gap 해소 — acceptance.md에 전용 AC-EVID-001-O1-1 추가, §6 catalog의 AC-EVID-001-1 오매핑 정정, Total AC 18→19, spec-compact.md 동기화). D2(spec.md §2.2 / plan.md §8 DB schema 경로 표현 불일치 해소 — research.md §2 규약대로 `.moai/db/schema/initial.sql`=참조 스키마, `.moai/db/schema/migrations/NNNN_*.sql`=마이그레이션으로 통일, `0002_evidence_tables.sql` 파일번호 비충돌 무조건 확정 — migrations/에 `0001_initial.sql`만 실재 확인). Cosmetic(§3.1 `REQ-UBI-NNN`→도메인 스코프 `REQ-EVID-UBI-NNN` 표현 정정, acceptance.md §6 immutability edge의 R-EVID-002 risk-ID 재사용 → REQ-EVID-UBI-004 불변식 참조로 교체). (작성자: ircp)
- 0.1.0 (2026-05-18): 경영평가 증빙 자료 수집/관리(Evidence Management) 첫 초안. iroum-ax Go control-plane을 brownfield 확장하여 증빙 문서 데이터 모델 + store 계층 + audit 연계 Walking Skeleton 정의. 재업로드 버전 체이닝(`previous_version_id`), 평가항목 ID 경량 stub(FK 제약 없음), 저장 전략 추상화(filesystem/DB BLOB/self-hosted MinIO 후보 — 선택은 run/strategy 단계로 이연). 평가항목 taxonomy, 전체 업로드 UI, 저장 전략 최종 선택은 의도적 제외. SPEC-AX-CTRL-001의 WorkflowStore/WorkflowTx/AuditRecorder 패턴(GREEN 가정) 위에 EvidenceStore/EvidenceTx/Recorder 확장 메서드를 동일 패턴으로 추가. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001 / SPEC-AX-CTRL-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등은 canonical schema에 존재하지 않으므로 사용하지 않는다.

---

# SPEC-AX-EVID-001 — 경영평가 증빙 자료 수집/관리 (Evidence Management)

## 1. 개요

경영평가팀이 평가항목별 실적을 입증하는 **증빙 문서(evidence)를 수집·버전 관리·감사 추적**할 수 있도록, `apps/control-plane/`(Go 1.22+)에 증빙 데이터 모델·영속 계층·감사 연계를 추가하는 Walking Skeleton을 정의한다. 본 SPEC은 기존 SPEC-AX-CTRL-001의 워크플로우 오케스트레이션 계층 위에, **재업로드 시 새 버전 행을 생성하고 이전 버전을 보존하며 모든 변경을 `audit_logs`에 원자적으로 기록하는** 최소 실행 가능한 증빙 관리 계층을 제공한다.

### 1.1 Walking Skeleton의 의미 (본 SPEC 범위)

본 SPEC의 Walking Skeleton은 **기초 데이터 모델 + store 계층 + audit 연계**에 집중한다. 전체 업로드 UX(멀티파트 폼, 진행률 표시, 미리보기)는 본 SPEC 범위가 아니다.

- 단일 엔티티: `evidences` 테이블 (`evaluation_item_id` 경량 FK stub + `version` + `previous_version_id` 자기 참조 체이닝)
- 단일 store 추상화: `EvidenceStore` / `EvidenceTx` 인터페이스 (SPEC-AX-CTRL-001 `WorkflowStore`/`WorkflowTx` 패턴 그대로 — 동일 pgx pool 재사용, 신규 풀 생성 없음)
- 단일 감사 연계: 기존 `internal/audit` Recorder에 `RecordEvidenceCreated` / `RecordEvidenceVersioned` 확장 (기존 `RecordCreated`/`RecordTransition`과 동일 시그니처 패턴, 동일 트랜잭션 내 atomic 기록)
- 단일 생성 경로: 증빙 생성/버전 1개 엔드포인트 (multipart 수신 → SHA256 해시 → BeginTx → 기존 max version 조회 → InsertEvidence → Recorder 기록 → Commit). 파일 바이너리의 물리적 저장은 추상화만 정의하고 **전략 선택은 이연**.
- 인증 없음: 모든 호출은 `created_by="cli-anonymous"` (SPEC-AX-001 REQ-UBI-003 및 SPEC-AX-CTRL-001 REQ-CTRL-UBI-002와 정합)

### 1.2 Anchor 컨텍스트

본 SPEC은 `product.md` §3.2의 고객 제공 자료(기획재정부 경영평가 편람, 작성지침, 자사 실적보고서, A/B/C/D 등급 벤치마크 보고서)를 시스템에 입력하는 진입 경로를 형성한다. SPEC-AX-CTRL-001의 5개 REQ-CTRL과 2개 REQ-CTRL-UBI(트랜잭션 원자성·감사 일관성)는 GREEN 상태로 가정하며, 본 SPEC은 그 store/audit 패턴을 증빙 도메인으로 확장한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `EVID` (Evidence Management sub-domain)
- 따라서 SPEC ID: `SPEC-AX-EVID-001` (2 domains, `.claude/skills/moai/workflows/plan.md:366` Composite domain rules — "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내)

### 1.4 평가항목 의존성 — 경량 참조 stub

증빙은 평가항목(evaluation item)에 귀속된다. 그러나 평가항목 taxonomy(항목 → 지표 → 배점 계층 구조)는 **본 SPEC의 범위가 아니다**. `evidences.evaluation_item_id`는 **FK 제약 없는 plain `VARCHAR(64)` stub**으로 정의하며, 참조 대상 `evaluation_items` 테이블은 생성하지 않는다. 평가항목 taxonomy는 명시적으로 미래 SPEC(아직 미생성: `SPEC-AX-EVAL-ITEM-001` 후보)으로 이연한다. 이는 §5 Exclusions와 §7 Out of Scope에 명시한다.

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 `apps/control-plane/` 트리를 따른다. 본 SPEC은 stub이 아닌 **실제 구현이 존재하는 코드를 brownfield 확장**하므로 Delta 마커를 적용한다 ([EXISTING]=특성화 테스트로 보존, [NEW]=신규 추가, [MODIFY]=기존 파일 수정).

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/internal/store/store.go` | `EvidenceStore` / `EvidenceTx` 인터페이스 추가 (기존 `WorkflowStore`/`WorkflowTx` 패턴 준수) | [MODIFY] | REQ-EVID-001 |
| `apps/control-plane/internal/store/evidence.go` | `EvidenceTx` 메서드 (InsertEvidence, GetEvidenceByID, GetLatestVersionByEvalItem, ListEvidenceByEvalItem) pgx 구현 | [NEW] | REQ-EVID-001, REQ-EVID-002 |
| `apps/control-plane/internal/store/pg_store.go` | 실 pgx pool(`PgWorkflowStore{pool *pgxpool.Pool}`, `server.go:86` `store.NewPgWorkflowStore(...)` 와이어링)에 evidence 트랜잭션 진입점 추가 (신규 풀 금지, `PgWorkflowStore.pool` 단일 재사용). **주의: `postgres.go`는 Sprint-0 死 스텁(`New(cfg)` + TODO Sprint 7, 실 pool 없음)이며 본 SPEC 대상 아님** — strategy.md §0 phantom-path 교정 | [MODIFY] | REQ-EVID-001 |
| `apps/control-plane/internal/audit/audit.go` | 신규 액션 상수 `ActionEvidenceCreated`, `ActionEvidenceVersioned` 추가 (기존 `Action string` 패턴) | [MODIFY] | REQ-EVID-003 |
| `apps/control-plane/internal/audit/recorder.go` | `RecordEvidenceCreated`, `RecordEvidenceVersioned` 메서드 추가 (기존 `RecordCreated` 시그니처 패턴, 로컬 `AuditTx` 인터페이스 유지) | [MODIFY] | REQ-EVID-003 |
| `apps/control-plane/internal/storage/storage.go` | `EvidenceBlobStore` 저장 전략 인터페이스 정의 (구현체 없음 — 전략 미선택) | [NEW] | REQ-EVID-004 |
| `apps/control-plane/cmd/server/evidence_handlers.go` | 증빙 생성/버전 엔드포인트 (multipart 수신 → SHA256 → BeginTx → 버전 결정 → InsertEvidence → Recorder → Commit) | [NEW] | REQ-EVID-001, REQ-EVID-002 |
| `apps/control-plane/internal/audit/clock.go` | 테스트 친화성을 위한 clock 주입 인터페이스 (research.md §8 Risk 7 대응, 선택적 REFACTOR 산출물) | [NEW] | REQ-EVID-003 |

### 2.2 Database (`.moai/db/schema/`)

| 경로 | 책임 | Delta |
|------|------|-------|
| `.moai/db/schema/initial.sql` | **참조 only**: 본 SPEC은 initial.sql을 수정하지 않는다 (기존 documents/audit_logs 테이블 schema drift 방지). | [EXISTING] |
| `.moai/db/schema/migrations/0002_evidence_tables.sql` | **신규**: `evidences` 테이블 + 인덱스 추가. 멱등성 패턴(`CREATE TABLE IF NOT EXISTS`, `DO $$ ... EXCEPTION ...`) 유지, 수동 SQL 규약 (마이그레이션 도구 미사용). | [NEW] |

### 2.3 Tests (`apps/control-plane/`)

| 경로 | 책임 | Delta |
|------|------|-------|
| `apps/control-plane/internal/store/evidence_test.go` | testcontainers-go(postgres) 기반 InsertEvidence + 버전 체이닝 + SELECT FOR UPDATE 동시성 | [NEW] |
| `apps/control-plane/internal/audit/recorder_evidence_test.go` | RecordEvidenceCreated/Versioned audit row 검증 + audit fail 시 양방향 rollback | [NEW] |
| `apps/control-plane/cmd/server/evidence_handlers_test.go` | `httptest.NewRequest` 기반 증빙 생성/버전 엔드포인트 + oversized/duplicate 검증 | [NEW] |
| `apps/control-plane/internal/store/postgres_test.go` | 기존 WorkflowStore 특성화 테스트 보존 확인 (evidence 와이어링 후 회귀 없음) | [EXISTING] |

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건)

Ubiquitous 요구사항은 SPEC-AX-001 / SPEC-AX-CTRL-001의 canonical `REQ-UBI-NNN (한글 제목)` 패턴을 도메인 스코프 형태(`REQ-EVID-UBI-NNN`)로 적용한다 (SPEC-AX-CTRL-001의 `REQ-CTRL-UBI-*`와 동일한 dual-track 규약).

- **REQ-EVID-UBI-001 (데이터 주권)**: The evidence management subsystem SHALL NOT make any external service call (외부 LLM API, 외부 object storage SaaS, 외부 CDN, 외부 secrets manager) for storing, retrieving, or hashing evidence binaries. 모든 저장 경로는 고객사 내부망 자원(PostgreSQL, 내부 파일시스템, self-hosted 서비스)에만 의존한다. 외부 호출이 구조적으로 가능한 저장 전략은 채택될 수 없다 (`tech.md` §9.1 망분리 정합).
- **REQ-EVID-UBI-002 (감사 가능성)**: The evidence management subsystem SHALL write exactly one `audit_logs` entry within the same database transaction as every evidence create and every evidence version event, reusing the `audit_logs` schema defined in SPEC-AX-001 REQ-UBI-003. 트랜잭션 외부에서 발생한 증빙 변경은 audit 불가능하므로 금지된다.
- **REQ-EVID-UBI-003 (cli-anonymous 기본값)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the subsystem SHALL persist `evidences.created_by = 'cli-anonymous'` and the corresponding `audit_logs.user_id = 'cli-anonymous'` (정확히 literal 문자열, NULL 금지), reusing `audit.DefaultUserID` / `Recorder.resolveUserID` 계약 (실 사용자 식별자 누출 금지).
- **REQ-EVID-UBI-004 (버전 불변)**: The subsystem SHALL NOT UPDATE or DELETE any column of an evidence row once a newer version exists for the same `evaluation_item_id`. 동일 논리 증빙의 재업로드는 항상 새 행(new `id`, `version+1`, `previous_version_id`=직전 행 `id`)을 생성하며, 이전 버전 행은 byte-identical하게 보존된다 (상태 컬럼 `status`만 `ACTIVE`→`SUPERSEDED` 전이 허용, 본문/파일 메타데이터는 불변).

### 3.2 REQ-EVID-001 — 증빙 데이터 모델 & Store 계층

**엔티티**: `evidences` (id UUID PK, evaluation_item_id VARCHAR(64) — FK 제약 없음, version INT, previous_version_id UUID 자기 참조, file_name, file_size_bytes, file_hash_sha256, content_type, `file_content BYTEA` (nullable — DB BLOB 바이너리 저장처), storage_location, storage_strategy, status, metadata JSONB, created_at, created_by DEFAULT 'cli-anonymous', updated_at, archived_at). `file_content`는 `storage_strategy='database_blob'`일 때 바이너리가 저장되는 컬럼(해당 전략에서는 NOT NULL 의미)이며, 다른 전략(`filesystem`/`minio`)에서는 NULL이고 바이너리는 `storage_location`이 가리키는 외부 위치에 저장된다. 구체 DDL은 `plan.md` §3 참조.

**Store 추상화**: `EvidenceStore.BeginTx(ctx) (EvidenceTx, error)` + `EvidenceTx{InsertEvidence, GetEvidenceByID, GetLatestVersionByEvalItem, ListEvidenceByEvalItem, InsertAuditLog, Commit, Rollback}` — 기존 `WorkflowStore`/`WorkflowTx` 인터페이스 설계를 그대로 미러링하며 동일 pgx pool을 재사용한다.

#### Event-driven

- **REQ-EVID-001-E1**: WHEN a client submits a new evidence document via the evidence create endpoint with a valid `evaluation_item_id` and file payload, THEN the subsystem SHALL compute the SHA-256 hash of the file content, open a single `EvidenceTx`, INSERT one `evidences` row with `version=1` and `previous_version_id=NULL`, INSERT one corresponding `audit_logs` row with action `EVIDENCE_CREATED` in the same transaction, Commit atomically, and return the `evidence_id` with HTTP 201.

#### State-driven

- **REQ-EVID-001-S1**: WHILE a version-resolving transaction is in progress for a given `evaluation_item_id`, the subsystem SHALL hold a `SELECT ... FOR UPDATE` lock on the latest existing evidence row for that `evaluation_item_id` (via `GetLatestVersionByEvalItem`) for the duration of the transaction. 동일 `evaluation_item_id`에 대한 동시 버전 생성 시도는 락이 해제될 때까지 직렬화된다 (orphan 버전 / 중복 version 번호 방지).

#### Optional

- **REQ-EVID-001-O1**: WHERE an existing evidence row for the same `evaluation_item_id` already has a `file_hash_sha256` identical to the incoming file's hash, the subsystem MAY surface a non-blocking `duplicate_of: <existing_evidence_id>` signal in the response body. 이는 정보 제공용 선택 기능이며 생성을 거부하지 않는다 (재업로드가 합법적 버전 생성일 수 있으므로). Sandbox PoC에서는 비활성 가능.

#### Unwanted

- **REQ-EVID-001-U1**: IF the incoming file exceeds the configured maximum size (`EVIDENCE_MAX_FILE_BYTES`, default 50 MiB) OR the file payload is empty OR `evaluation_item_id` is missing/blank, THEN the subsystem SHALL reject the request with HTTP 400 and a structured error body `{error:{code, message, field}}`, SHALL NOT open a transaction, SHALL NOT INSERT any `evidences` or `audit_logs` row, and SHALL log the rejection at INFO level (client error, not a server defect).

### 3.3 REQ-EVID-002 — 증빙 버전 관리 (Versioning & Chaining)

#### Event-driven

- **REQ-EVID-002-E1**: WHEN a client submits an evidence document for an `evaluation_item_id` that already has at least one existing evidence row, THEN the subsystem SHALL determine the current maximum `version` for that `evaluation_item_id`, INSERT a NEW row with `id`=new UUID, `version`=current_max+1, `previous_version_id`=the id of the immediately preceding (current latest) row, set the preceding row's `status` to `SUPERSEDED`, write an `audit_logs` row with action `EVIDENCE_VERSIONED` in the same transaction, and Commit atomically. 새 버전과 직전 버전의 상태 전이는 단일 트랜잭션 내에서 atomic하다.

#### State-driven

- **REQ-EVID-002-S1**: WHILE more than one version exists for an `evaluation_item_id`, the subsystem SHALL preserve every prior version row retrievable by `GetEvidenceByID` and the full chain navigable via `previous_version_id` (가장 최신 → 가장 오래된 방향으로 단절 없는 단일 연결 리스트). 어떤 버전 행도 물리적으로 삭제되지 않는다.

#### Unwanted

- **REQ-EVID-002-U1**: IF any code path attempts to UPDATE the `file_name`, `file_size_bytes`, `file_hash_sha256`, `content_type`, `storage_location`, or `metadata` column of an evidence row that already has a successor (a row whose `previous_version_id` points to it), THEN the subsystem SHALL reject the mutation at the store layer (return an error, no SQL executed) — prior versions are immutable except for the `status` ACTIVE→SUPERSEDED transition (REQ-EVID-UBI-004 강제).

### 3.4 REQ-EVID-003 — 감사 연계 (Audit Recorder 확장)

기존 `internal/audit` Recorder 패턴을 확장한다. 새 액션 상수 `ActionEvidenceCreated Action = "EVIDENCE_CREATED"`, `ActionEvidenceVersioned Action = "EVIDENCE_VERSIONED"`를 추가하고, `Recorder`에 `RecordEvidenceCreated`/`RecordEvidenceVersioned` 메서드를 추가한다 (기존 `RecordCreated(ctx, tx AuditTx, ...)` 시그니처 패턴, 로컬 `AuditTx` 인터페이스 유지 — store→audit 순환 의존 회피).

#### Event-driven

- **REQ-EVID-003-E1**: WHEN an evidence create or version transaction calls `Recorder.RecordEvidenceCreated` or `Recorder.RecordEvidenceVersioned` with the active `AuditTx`, THEN the Recorder SHALL construct an `audit.Event` with `Action` ∈ {`EVIDENCE_CREATED`, `EVIDENCE_VERSIONED`}, `ResourceType="evidence"`, `ResourceID`=the evidence UUID, `UserID`=`resolveUserID(userID)`, and `DetailsJSON` containing `{evaluation_item_id, version, file_hash_sha256, previous_version_id?}`, and SHALL insert it via the same `AuditTx` (동일 트랜잭션).

#### Unwanted

- **REQ-EVID-003-U1**: IF the `audit_logs` INSERT fails for any reason (constraint violation, connection reset) during an evidence create or version transaction, THEN the subsystem SHALL execute `tx.Rollback(ctx)`, leave the `evidences` table with NO row for this operation (evidence INSERT also rolled back), return the wrapped audit-insertion error to the caller, and SHALL NOT leak goroutines beyond the request scope. 증빙 행과 감사 행은 함께 커밋되거나 함께 롤백된다 (all-or-nothing).

### 3.5 REQ-EVID-004 — 저장 전략 추상화 (Storage Strategy Abstraction)

> **RESOLVED: `database_blob` (PostgreSQL BYTEA)** — Run Phase 1 manager-strategy 분석 + 사용자 승인 (strategy.md §2, Human Gate Decision Point 1). 근거 요약: 증빙 행 + `audit_logs` 행 + 파일 바이너리(`file_content BYTEA`)를 **동일 pgx 트랜잭션**에 포함시켜 REQ-EVID-UBI-002 / REQ-EVID-003-U1 원자성(all-or-nothing)을 구조적으로 보장하고, 별도 orphan-cleanup 메커니즘을 제거하며, 망분리 외부 의존 0건(이미 내부망인 PostgreSQL만 사용)을 by-construction으로 충족. 가중 트레이드오프 점수 DB BLOB **8.45** vs filesystem 5.45 vs self-hosted MinIO 3.85 (strategy.md §2.2). PoC 단일 노드 + 50 MiB 상한 + p99<150ms 측정 경로에서 blob latency 제외(§4)로 알려진 DB BLOB 약점(WAL/테이블 비대화)의 영향 범위가 한정됨.
>
> **추상화 유지(post-PoC 무중단 전환 대비)**: `database_blob` 확정 후에도 `storage_strategy` enum의 `filesystem`/`minio` 값과 `EvidenceBlobStore` 인터페이스 추상화는 그대로 유지한다. PoC 중 DB 비대화가 실측 문제로 드러나면 `storage_strategy` 값 + 새 `EvidenceBlobStore` 구현체 추가만으로 schema 변경 없이 filesystem으로 전환 가능 (strategy.md §2.4/§2.5).

#### State-driven

- **REQ-EVID-004-S1**: WHILE any evidence row is persisted, its `storage_strategy` column SHALL hold exactly one value from the enumerated set `{'filesystem', 'database_blob', 'minio'}`. 어떤 row도 NULL 또는 열거되지 않은 전략 값을 가질 수 없다 (CHECK 제약 + store 계층 검증). 실제 선택된 전략은 환경설정(`EVIDENCE_STORAGE_STRATEGY`, **기본값 `database_blob`** — Run Phase 1 확정)에서 주입되며 config 로드 시 enum 검증(fail-fast, SPEC-AX-CTRL-001 startup 패턴 정합)한다.

#### Optional

- **REQ-EVID-004-O1**: WHERE the `EvidenceBlobStore` interface is defined, the subsystem SHALL expose it as a strategy-swappable abstraction (`Put(ctx, key, reader) (location string, error)`, `Get(ctx, location) (reader, error)`). `database_blob` 전략의 PoC 구현체는 논리 location 문자열(예: `db://evidences/<id>`)을 반환·기록하는 역할만 수행하며, **실제 blob 바이너리는 `EvidenceBlobStore` 인터페이스를 통과하지 않고 `EvidenceTx.InsertEvidence(..., file_content)` 경로로 동일 pgx 트랜잭션에 저장된다** (단일 TX 원자성 보존 — strategy.md §2.6.5 option 6a, Human Gate 승인). 이로써 추상화 계약(전략 교체 가능, 외부 SDK 0건)과 동일 TX 원자성을 동시에 충족한다. 다른 전략(filesystem/minio) 구현체는 전략 전환 시 별도 작업에서 추가된다.

#### Unwanted

- **REQ-EVID-004-U1**: IF a storage strategy implementation issues a network call to a host outside the customer internal network (외부 S3 endpoint, 외부 CDN, 공용 인터넷 DNS 해석), THEN that strategy SHALL be considered non-compliant and SHALL NOT be adopted (REQ-EVID-UBI-001 강제). 확정된 `database_blob` 전략은 이미 내부망인 PostgreSQL pgx pool만 사용하므로 by-construction으로 본 제약을 충족한다. 외부 관리형 S3는 영구 부적격이며, self-hosted MinIO는 내부망 배포 시에만 후보(post-PoC 재검토 대상).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 데이터 주권 (망분리) | 증빙 저장·조회·해시 경로의 외부 API 호출 0건. 채택 저장 전략은 내부망 자원만 사용 | §3.1 REQ-EVID-UBI-001, `tech.md` §9.1 |
| 감사 가능성 | 모든 evidence create/version → 동일 TX 내 `audit_logs` 1건. 누락 0건 | §3.1 REQ-EVID-UBI-002 |
| 버전 무결성 | 이전 버전 행 byte-identical 보존, 물리 삭제 0건, version 번호 gap/중복 0건 | §3.1 REQ-EVID-UBI-004, §3.3 |
| 성능 — 증빙 생성 | p99 < 150ms (SHA-256 해시 + 단일 TX INSERT, 50 MiB 이하 파일, 파일 바이너리 물리 저장 latency 제외) | §3.2 REQ-EVID-001-E1 |
| 동시성 | 동일 `evaluation_item_id` 동시 버전 생성 시 `SELECT FOR UPDATE`로 직렬화 | §3.2 REQ-EVID-001-S1 |
| 자원 한계 | 단일 파일 ≤ `EVIDENCE_MAX_FILE_BYTES` (default `52428800` = 50 MiB), 초과 시 TX 진입 전 거부 | §3.2 REQ-EVID-001-U1 |
| 저장 전략 (확정) | `EVIDENCE_STORAGE_STRATEGY` default `database_blob` (Run Phase 1 확정, strategy.md §2.3), config 로드 시 enum 검증 fail-fast | §3.5 REQ-EVID-004-S1 |
| Config 기본값 | `EVIDENCE_STORAGE_STRATEGY=database_blob`, `EVIDENCE_MAX_FILE_BYTES=52428800` (50 MiB), `EVIDENCE_DUPLICATE_SIGNAL_ENABLED=false` (Sandbox PoC 비활성) — `internal/config/config.go` `getEnv`/`getBoolEnv` 추가 | strategy.md §2.6.2 |
| pgx pool 재사용 | 기존 SPEC-AX-CTRL-001 단일 pgx pool(`PgWorkflowStore.pool`, `pg_store.go`) 재사용, 신규 풀 생성 금지. `postgres.go` 死 스텁 비대상 | research.md §8 Risk 6, strategy.md §0 |
| 로깅 | 구조화 JSON 로그(zap), 거부는 INFO, 서버 결함은 ERROR | `tech.md` §8.2 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR) | `quality.yaml` development_mode |
| Go 도구 | go vet, golangci-lint (default + gosec), goimports | `.claude/rules/moai/languages/go.md` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **평가항목 taxonomy / `evaluation_items` 테이블** — `evidences.evaluation_item_id`는 FK 제약 없는 `VARCHAR(64)` stub. 항목→지표→배점 계층 구조, 평가편람 파싱 연계, evaluation-item CRUD 일체 제외. [Deferred to future SPEC-AX-EVAL-ITEM-001 (아직 미생성, named placeholder)]
2. **전체 업로드 UI / UX** — Console(`apps/console/`)의 멀티파트 업로드 폼, 드래그앤드롭, 진행률 표시기, 썸네일/미리보기, 파일 브라우저 일체 제외. 본 SPEC은 데이터 모델 + store + 단일 생성 엔드포인트만 다룬다.
3. **대체 저장 백엔드 구현 (filesystem / self-hosted MinIO)** — 저장 전략은 Run Phase 1에서 `database_blob`로 확정(§3.5 RESOLVED, `plan.md` §6)되었으며 본 SPEC은 `database_blob` 구현체(`file_content BYTEA` + `dbBlobStore`)만 빌드한다. filesystem 디렉토리 레이아웃, self-hosted MinIO 클라이언트 통합 등 **대체 전략 구현체는 본 SPEC 범위 밖**(추상화·`storage_strategy` enum은 post-PoC 전환 대비 유지하되 구현은 전환 결정 시 별도 작업).
4. **인증 / 인가 / 증빙 권한 모델** — 모든 호출은 `created_by='cli-anonymous'`. JWT, RBAC `write:evidence` 권한 등록, 조직 격리는 SPEC-AX-AUTH 계열(기존/미래)에서 처리. 본 SPEC은 audit user_id 기본값 계약만 준수.
5. **증빙 삭제 / 보존 정책 (retention)** — 증빙 hard delete API, 법정 보존 기간 관리, archival/cold storage 이전, GDPR/PIPA 파기 요청 처리 일체 제외. `archived_at` 컬럼은 미래 확장용 placeholder로만 정의(본 SPEC에서 set하지 않음).
6. **증빙 ↔ 보고서/워크플로우 연계** — 증빙을 특정 `workflow_id` 또는 생성된 보고서 섹션에 바인딩하는 관계 모델 제외. 본 SPEC은 `evaluation_item_id` 귀속만 다룬다.
7. **증빙 내용 파싱 / OCR / RAG 인덱싱** — 증빙 파일의 텍스트 추출, VLM OCR, 임베딩, 벡터 검색은 Python pipelines(`ingestion`/`mapping`) 책임이며 본 SPEC 범위 아님.
8. **고급 검색 / 필터링** — `ListEvidenceByEvalItem`은 단일 `evaluation_item_id` 기준 버전 정렬 조회만 지원. 자유 텍스트 검색, 다중 필드 조합, 날짜 범위 쿼리, full-text 검색 제외.
9. **마이그레이션 도구 통합 (alembic / golang-migrate)** — `0002_evidence_tables.sql` 수동 멱등 SQL 단일 패치. 마이그레이션 러너 도구 제외 (SPEC-AX-CTRL-001 Exclusion §8 동일 정책).

---

## 6. 의존성 및 전제

- **SPEC-AX-CTRL-001 GREEN 가정**: `internal/store`의 `WorkflowStore`/`WorkflowTx` 인터페이스, `internal/audit`의 `Recorder`/`AuditTx`/`Action`/`Event`/`DefaultUserID`, 단일 pgx pool 와이어링이 모두 GREEN 상태이고 source-verified (본 SPEC 작성 시 `store.go`, `recorder.go`, `audit.go`, `initial.sql` 실 시그니처 확인 완료 — phantom API 없음).
- **`audit_logs` 테이블 스키마 재사용**: `initial.sql`의 `audit_logs`(id, user_id VARCHAR(64), action VARCHAR(64), resource_id UUID, resource_type VARCHAR(32), timestamp, details JSONB)를 그대로 사용한다. 본 SPEC은 `audit_logs` schema를 변경하지 않는다.
- **`evidences` 테이블 신규**: `0002_evidence_tables.sql`로 추가. `initial.sql` 미수정 (schema drift 방지).
- **평가항목 ID 경량 stub**: `evaluation_item_id`는 외부에서 임의 문자열로 주입됨 (참조 무결성 미보장은 의도된 설계). 평가항목 시스템 부재가 본 SPEC 구현을 차단하지 않는다 (순환 의존 없음).
- **Go 1.22+**, module `github.com/ircp/iroum-ax`. 주요 의존성: `github.com/jackc/pgx/v5`, `github.com/google/uuid`, `go.uber.org/zap`, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go` (test).
- **Cross-SPEC artifact 영향 없음**: 본 SPEC은 SPEC-AX-CTRL-001의 골든 파일(`celery_envelope_v2.json`) 또는 기존 generated artifact를 수정하지 않는다 (clean greenfield 확장).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- **평가항목 taxonomy 시스템**: `evaluation_item_id`가 가리키는 평가항목 정의·계층·배점은 본 SPEC 범위 밖이며 참조 테이블도 만들지 않는다. 미래 `SPEC-AX-EVAL-ITEM-001`(미생성) 후보.
- **대체 저장 백엔드 구현**: 저장 전략은 `database_blob`로 확정(§3.5 RESOLVED). 본 SPEC은 `database_blob`(`file_content BYTEA`)만 구현하며, filesystem 디렉토리 레이아웃·MinIO 클라이언트 통합 등 대체 전략 구현체는 범위 밖(추상화·enum은 전환 대비 유지).
- **Console 업로드 화면**: `apps/console/app/documents/`는 본 SPEC과 무관.
- **증빙 파일 내용 분석**: OCR/파싱/RAG는 Python pipelines 책임.
- **증빙 권한/조직 격리**: SPEC-AX-AUTH 계열 책임. 본 SPEC은 cli-anonymous 기본값만.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/internal/{store,audit}/*_test.go` — 테이블 테스트, testify/assert, t.Parallel, goleak
- 통합 테스트: `apps/control-plane/internal/store/evidence_test.go` — testcontainers-go(postgres:16-pgvector), SELECT FOR UPDATE 동시성, 버전 체이닝
- 감사 원자성 테스트: `apps/control-plane/internal/audit/recorder_evidence_test.go` — fault injection으로 audit INSERT 실패 시 evidence INSERT 양방향 rollback 검증
- 핸들러 테스트: `apps/control-plane/cmd/server/evidence_handlers_test.go` — `httptest.NewRequest`, oversized/empty/duplicate 케이스
- 데이터 주권 검증: 저장/해시 경로에서 외부 네트워크 egress 0건 (테스트 환경 외부 host 차단 + 코드 정적 검사)
- 회귀: 기존 WorkflowStore 특성화 테스트가 evidence 와이어링 후에도 GREEN 유지

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
