# SPEC-AX-EVID-001 구현 계획 (Plan)

> 대응 SPEC: `.moai/specs/SPEC-AX-EVID-001/spec.md` v0.1.2
> 방법론: TDD (RED-GREEN-REFACTOR), harness: thorough
> 근거: `.moai/specs/SPEC-AX-EVID-001/research.md` (Phase 0.5 deep research, file:line 근거 포함) + `.moai/specs/SPEC-AX-EVID-001/strategy.md` (Run Phase 1 전략, 저장 전략 RESOLVED)

---

## 1. 개요 및 접근 방식

본 SPEC은 SPEC-AX-CTRL-001이 확립한 **`WorkflowStore`/`WorkflowTx` + `Recorder`/`AuditTx` 트랜잭션 원자성 패턴**(research.md §2, §3)을 증빙 도메인으로 brownfield 확장한다. 핵심 설계 결정은 다음과 같다:

1. **데이터 모델 우선 (1st deliverable)**: `evidences` 테이블 + `EvidenceStore`/`EvidenceTx` 인터페이스를 먼저 GREEN으로 만든다. 업로드 UX는 범위 밖.
2. **버전 체이닝**: 재업로드는 UPDATE가 아닌 새 행 INSERT (`previous_version_id` 자기 참조). 이전 버전 불변 (REQ-EVID-UBI-004).
3. **감사 원자성**: 증빙 변경과 `audit_logs` 기입을 단일 `EvidenceTx` 내에서 atomic 처리 (기존 `Recorder` 패턴 확장).
4. **저장 전략 확정 = `database_blob`** (Run Phase 1, §6 RESOLVED): blob 바이너리를 `file_content BYTEA`로 evidence 행+audit 행과 동일 pgx TX에 저장 → 원자성 구조적 보장. `EvidenceBlobStore` 인터페이스 + `storage_strategy` enum은 post-PoC 전환 대비 유지(filesystem/minio 구현체는 미빌드).
5. **평가항목 경량 stub**: `evaluation_item_id`는 FK 없는 VARCHAR. 참조 테이블 미생성.

### 1.1 Brownfield Delta 요약

| 마커 | 대상 | 처리 |
|------|------|------|
| [EXISTING] | `internal/store/pg_store.go` 기존 `PgWorkflowStore`/`PgWorkflowTx`, `internal/audit/recorder.go` 기존 8 메서드, `cmd/server` 기존 핸들러 | 특성화 회귀 테스트로 보존 — 동작 변경 0건 |
| [NEW] | `internal/store/evidence.go`, `internal/storage/storage.go`, `cmd/server/evidence_handlers.go`, `0002_evidence_tables.sql`, `internal/audit/clock.go` | 신규 추가 |
| [MODIFY] | `internal/audit/audit.go` (액션 상수 2개 추가), `internal/store/store.go` (인터페이스 2개 추가), `internal/store/pg_store.go` (실 `PgWorkflowStore.pool` 재사용한 evidence TX 진입점 추가) | 기존 심볼 불변, additive only |

> **Phantom-path 교정 (strategy.md §0)**: 본 계획 초안은 evidence TX 와이어링 대상을 `postgres.go`로 기술했으나, `postgres.go`는 Sprint-0 死 스텁(`New(cfg) (*Store, error)` + `// TODO(Sprint 7)`, 실 pool 없음, `BeginTx` 없음)이다. 실 pgx pool은 `pg_store.go`의 `PgWorkflowStore{pool *pgxpool.Pool}`(`NewPgWorkflowStore` 생성, `server.go:86` 와이어링)이다. 단일 pool 재사용(R-EVID-005)은 `PgWorkflowStore.pool` 재사용으로 달성하며 `postgres.go`는 대상이 아니다. 문서/경로 교정이며 scope/요구사항 변경 아님.

---

## 2. 작업 분해 (Sprint Decomposition)

TDD RED-GREEN-REFACTOR. 각 Sprint는 RED(실패 테스트) → GREEN(최소 구현) → REFACTOR 순.

### Sprint 0 — Foundation (Migration + 인터페이스 골격)
- [NEW] `0002_evidence_tables.sql` 작성 (멱등 SQL, §3 DDL)
- [MODIFY] `store.go`에 `EvidenceStore`/`EvidenceTx` 인터페이스 선언 (메서드 시그니처만, 구현은 Sprint 1)
- [MODIFY] `audit.go`에 `ActionEvidenceCreated`/`ActionEvidenceVersioned` 상수 추가
- 기존 WorkflowStore 특성화 테스트 회귀 확인 (GREEN 유지)

### Sprint 1 — Evidence Store (REQ-EVID-001)
- RED: `evidence_test.go` — InsertEvidence + GetEvidenceByID 단위 테스트 (testcontainers)
- GREEN: [NEW] `evidence.go` pgx 구현(`PgEvidenceTx`, `pg_store.go`의 `PgWorkflowTx` 미러링), [MODIFY] `pg_store.go`에 `PgWorkflowStore.pool` 재사용한 evidence TX 진입점 추가 (NOT `postgres.go` 死 스텁 — strategy.md §0)
- RED: SELECT FOR UPDATE 동시성 테스트 (REQ-EVID-001-S1)
- GREEN: `GetLatestVersionByEvalItem`에 `FOR UPDATE` 절 구현
- REFACTOR: TX 진입점 중복 제거 (WorkflowTx와 공유 가능 헬퍼 추출 검토)

### Sprint 2 — Versioning (REQ-EVID-002)
- RED: 재업로드 → version=2 + previous_version_id 체이닝 테스트
- GREEN: 핸들러/store 버전 결정 로직 (max version 조회 → +1 → 직전 행 status SUPERSEDED)
- RED: 3-deep 체인 보존 + 이전 버전 immutability(REQ-EVID-002-U1) 테스트
- GREEN: store 계층 mutation guard (successor 존재 시 본문 컬럼 UPDATE 거부)

### Sprint 3 — Audit Integration (REQ-EVID-003)
- RED: `recorder_evidence_test.go` — RecordEvidenceCreated/Versioned audit row 검증
- GREEN: [MODIFY] `recorder.go`에 2개 메서드 추가 (기존 `RecordCreated` 시그니처 패턴, 로컬 `AuditTx`)
- RED: audit INSERT 실패 → evidence + audit 양방향 rollback (REQ-EVID-003-U1)
- GREEN: 핸들러 TX orchestration (audit 실패 시 `tx.Rollback`)
- REFACTOR: [NEW] `clock.go` clock 주입 (research.md §8 Risk 7 — `time.Now().UTC()` 직접 호출 테스트 비친화 해소)

### Sprint 4 — Storage Abstraction + Endpoint (REQ-EVID-004 + REQ-EVID-001-E1 통합)
- RED: [NEW] `storage.go` `EvidenceBlobStore` 인터페이스 계약 테스트 (구현체 없음 — 인터페이스 존재 + nil-impl 거부)
- GREEN: 인터페이스 정의, `storage_strategy` 컬럼 검증 (열거값 강제)
- RED: `evidence_handlers_test.go` — 생성/버전 엔드포인트 + oversized/empty/duplicate (REQ-EVID-001-U1, O1)
- GREEN: [NEW] `evidence_handlers.go` (multipart → SHA256 → BeginTx → 버전 결정 → InsertEvidence → Recorder → Commit)
- RED: 외부 네트워크 egress 0건 검증 (REQ-EVID-UBI-001, REQ-EVID-004-U1)
- GREEN: 데이터 주권 정적 보장 (저장/해시 경로 외부 host 호출 없음)

### Sprint 5 — Quality Gate
- 커버리지 ≥ 85%, golangci-lint default+gosec 0 issue, goleak 전체 통과
- @MX 태그 §5 매핑 완료, TRUST 5, evaluator-active strict ≥ 0.75

---

## 3. `evidences` 테이블 DDL (`0002_evidence_tables.sql`)

```sql
-- 마이그레이션: 0002_evidence_tables
-- SPEC-AX-EVID-001 증빙 데이터 모델 (멱등성 패턴 유지, 수동 SQL)
CREATE TABLE IF NOT EXISTS evidences (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_item_id  VARCHAR(64) NOT NULL,                          -- 경량 FK stub, 제약 없음 (REQ-EVID 의도)
    version             INT NOT NULL DEFAULT 1,
    previous_version_id UUID REFERENCES evidences(id) ON DELETE RESTRICT,
    file_name           VARCHAR(512) NOT NULL,
    file_size_bytes     BIGINT,
    file_hash_sha256    VARCHAR(64),
    content_type        VARCHAR(128),
    file_content        BYTEA,                                         -- DB BLOB 바이너리 저장처. storage_strategy='database_blob'일 때 NOT NULL 의미(앱 계층 강제), 타 전략(filesystem/minio)에서는 NULL(외부 storage_location 사용)
    storage_location    VARCHAR(255),                                  -- database_blob: 논리 식별자 'db://evidences/<id>'; filesystem/minio: 외부 위치
    storage_strategy    VARCHAR(32) NOT NULL DEFAULT 'database_blob',  -- 'filesystem'|'database_blob'|'minio' (Run Phase 1 확정 기본값=database_blob)
    status              VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',         -- ACTIVE|SUPERSEDED
    metadata            JSONB,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by          VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',  -- audit.DefaultUserID 정합
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at         TIMESTAMP WITH TIME ZONE                       -- 미래 retention placeholder (본 SPEC 미사용)
);

DO $$ BEGIN
    ALTER TABLE evidences ADD CONSTRAINT evidences_storage_strategy_chk
        CHECK (storage_strategy IN ('filesystem','database_blob','minio'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS evidences_eval_item_version_idx
    ON evidences (evaluation_item_id, version DESC);
CREATE INDEX IF NOT EXISTS evidences_created_at_idx
    ON evidences (created_at DESC);
```

DDL 근거: research.md §9 제안 데이터 모델. `initial.sql`은 수정하지 않는다 (schema drift 방지, spec.md §2.2).

---

## 4. 기술 스택

- 언어/런타임: Go 1.22+, module `github.com/ircp/iroum-ax`
- DB 드라이버: `github.com/jackc/pgx/v5` (기존 단일 pool 재사용 — research.md §8 Risk 6)
- 해시: 표준 라이브러리 `crypto/sha256` (외부 의존 없음 — 데이터 주권)
- UUID: `github.com/google/uuid`
- 로깅: `go.uber.org/zap` (기존 패턴)
- 테스트: `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go`(postgres:16-pgvector), `go.uber.org/goleak`
- 마이그레이션: 수동 멱등 SQL (`DO $$ ... EXCEPTION ...`), 도구 미사용
- 저장 백엔드: `database_blob` 확정 — 신규 외부 의존(MinIO SDK / aws-sdk) **추가 안 함** (데이터 주권 REQ-EVID-UBI-001 정합)

### 4.1 Config 환경변수 기본값 (Run Phase 1 확정 — strategy.md §2.6.2)

`internal/config/config.go`에 additive로 추가 (기존 `getEnv`/`getBoolEnv`/`Load` 패턴, config 로드 시 enum 검증 fail-fast — SPEC-AX-CTRL-001 startup 패턴 정합):

| 환경변수 | 기본값 | 타입/근거 |
|----------|--------|-----------|
| `EVIDENCE_STORAGE_STRATEGY` | `database_blob` | `getEnv`, enum {filesystem,database_blob,minio} 검증; Run Phase 1 확정 |
| `EVIDENCE_MAX_FILE_BYTES` | `52428800` (50 MiB) | `getEnv` 정수 파싱; REQ-EVID-001-U1 pre-TX 거부 임계 |
| `EVIDENCE_DUPLICATE_SIGNAL_ENABLED` | `false` | `getBoolEnv`; REQ-EVID-001-O1 Optional, Sandbox PoC 비활성 |

---

## 5. MX 태그 계획

research.md §8 Risk 1/3/6/7 및 SPEC-AX-CTRL-001 기존 태그 패턴(`store.go:16` `@MX:ANCHOR`/`@MX:REASON`)을 따른다.

| 대상 심볼 | 태그 | 근거 (@MX:REASON) |
|-----------|------|-------------------|
| `EvidenceStore` 인터페이스 (`store.go`) | `@MX:ANCHOR` | fan_in ≥ 3 (evidence_handlers, recorder 연계, 향후 list 조회) — 기존 WorkflowStore와 동일 |
| `EvidenceTx` 인터페이스 (`store.go`) | `@MX:ANCHOR` | AC-EVID-003-3 / AC-EVID-UBI-002 원자성 계약의 핵심 (FakeStore + pgx 구현체 2곳 구현) |
| `RecordEvidenceCreated` / `RecordEvidenceVersioned` (`recorder.go`) | `@MX:ANCHOR` | 증빙 감사 기록 단일 진입점 (REQ-EVID-UBI-002 AC 전체가 이 메서드 경유) |
| 증빙 생성/버전 핸들러의 BeginTx→InsertEvidence→InsertAuditLog→Commit 블록 (`evidence_handlers.go`) | `@MX:WARN` | `@MX:REASON`: 트랜잭션 원자성 — InsertAuditLog 이후 Commit 전 panic/early-return 시 orphan 증빙 행 누출 (research.md §8 Risk 3). Rollback defer 순서 변경 금지 |
| `EvidenceBlobStore` 인터페이스 (`storage.go`) | `@MX:NOTE` | 저장 전략 `database_blob` 확정(§6 RESOLVED) — PoC `dbBlobStore`는 논리 location만 기록(bytes는 EvidenceTx 경유), 추상화는 post-PoC 전환 대비 유지 |
| 버전 결정 로직 (max version → +1 → previous_version_id) | `@MX:WARN` | `@MX:REASON`: SELECT FOR UPDATE 락 미보유 시 동일 evaluation_item_id 동시 재업로드가 중복 version 번호 / orphan 체인 생성 (REQ-EVID-001-S1) |

TDD MX 흐름: RED에서 `@MX:TODO`(미구현 마커) → GREEN에서 제거 → REFACTOR에서 `@MX:NOTE`/`@MX:ANCHOR` 확정.

---

## 6. RESOLVED DECISION — 파일 바이너리 저장 전략 = `database_blob`

> **상태**: **RESOLVED — `database_blob` (PostgreSQL BYTEA)** (Run Phase 1 manager-strategy 분석 strategy.md §2 + 사용자 Human Gate Decision Point 1 승인). 이전 v0.1.1까지 run/strategy 단계 이연 OPEN DECISION이었으며, 본 v0.1.2에서 확정됨.

### 6.1 트레이드오프 표 (strategy.md §2.2 가중 평가 — 망분리 단일노드 PoC, 핵심 가치=트랜잭션 무결성)

가중치: Transactional Atomicity 0.30 · Data Sovereignty/Op Simplicity 0.25 · Implementation Cost 0.20 · Risk 0.15 · Scalability(post-PoC) 0.10

| Strategy | 가중 점수 | Pros | Cons | 채택 |
|----------|-----------|------|------|------|
| **DB BLOB** (PostgreSQL BYTEA) | **8.45** | blob bytes가 evidence 행+audit 행과 동일 pgx TX → rollback 자동, orphan 0; 외부 의존 0; 최소 코드 델타 | PG WAL/테이블 비대화(50 MiB 상한+PoC 볼륨으로 한정) | **SELECTED** |
| **Filesystem** (`/srv/evidence/`) | 5.45 | 단순/빠름, DB lean | blob write가 TX 외부 → write-ordering + orphan-sweep 머신러리 필요 (SPEC 핵심 불변식과 상충) | 기각 (post-PoC fallback) |
| **Self-hosted MinIO** | 3.85 | 확장성, S3 API | 망분리 내부 신규 stateful 서비스 운영 부담 최대; 외부 관리형 S3 영구 부적격 | 기각 (post-PoC 멀티노드 시 재검토) |

DB BLOB = 0.30·10 + 0.25·9 + 0.20·8 + 0.15·6 + 0.10·4 = **8.45** (strategy.md §2.2)

### 6.2 확정 근거 (decisive factors — strategy.md §2.3)

1. **원자성이 Walking Skeleton 핵심 가치**: DB BLOB은 blob write를 `evidences` 행 + `audit_logs` 행과 **동일 pgx 트랜잭션**에 포함시켜 REQ-EVID-UBI-002 / REQ-EVID-003-U1 (all-or-nothing)을 **구조적으로 보장**. filesystem/MinIO는 blob write가 TX 외부 → write-ordering 프로토콜 + orphan-sweep job이라는 신규 복잡성이 SPEC 중심 불변식과 정면 충돌.
2. **데이터 주권 by construction**: 바이너리가 이미 내부망인 PostgreSQL에 저장 → 신규 네트워크 endpoint/DNS 0, REQ-EVID-004-U1 노출면 0. 단일 pgx pool(R-EVID-005)이 유일 자원.
3. **단일 노드 PoC 최저 구현 비용/리스크**: `BYTEA` 컬럼 1개 + 활성 `EvidenceTx` 위 `EvidenceBlobStore.Put/Get` — PV 프로비저닝/MinIO 배포/credential 관리/cleanup cron 불요.
4. **영향 범위 한정**: DB BLOB 약점(테이블/WAL 비대화)은 50 MiB 상한(REQ-EVID-001-U1) + PoC 볼륨으로 제한되고 p99<150ms 측정 경로 밖(spec.md §4 blob latency 제외).

### 6.3 추상화 유지 — post-PoC 무중단 전환 대비

`database_blob` 확정 후에도 `storage_strategy` enum의 `filesystem`/`minio` 값과 `EvidenceBlobStore` 인터페이스는 그대로 유지한다. PoC 중 DB 비대화가 실측 문제로 드러나면 `storage_strategy` 값 + 새 `EvidenceBlobStore` 구현체 추가만으로 **schema 변경 없이** filesystem으로 전환 (strategy.md §2.4/§2.5 — post-PoC 볼륨 리뷰 체크포인트 권장).

### 6.4 EvidenceBlobStore 정합 (strategy.md §2.6.5, Human Gate option 6a 승인)

`database_blob` 전략에서 **blob 바이너리는 `EvidenceBlobStore` 인터페이스를 통과하지 않는다**. 바이너리는 `EvidenceTx.InsertEvidence(..., file_content)` 파라미터로 동일 pgx TX에 흐르고, `EvidenceBlobStore` 구현체(`dbBlobStore`)는 논리 location 문자열(`db://evidences/<id>`)을 반환·기록하는 메타데이터 역할만 수행한다. 이로써 (a) 단일 TX 원자성 보존, (b) REQ-EVID-004-O1 추상화 계약(전략 교체 가능, 외부 SDK 0) 동시 충족. (대안 6b — `Put`이 tx 핸들을 받는 방식 — 은 CTRL-001 레이어링과 어긋나 기각.)

### 6.5 본 SPEC이 보장하는 것

- `storage_strategy` 컬럼 CHECK 제약(enum) + config enum 검증 fail-fast, 기본값 `database_blob`
- `EvidenceBlobStore` 인터페이스 계약이 전략 교체를 schema 변경 없이 허용
- 데이터 모델/store/audit 계층은 확정 전략(`database_blob`) 위에서 GREEN; 추상화 유지로 전환 옵션 보존

---

## 7. 리스크 분석 (research.md §8 매핑)

| Risk ID | 설명 | 출처 | 완화 |
|---------|------|------|------|
| R-EVID-001 | store→audit 순환 의존 | research.md §8 Risk 1 | `recorder.go`의 로컬 `AuditTx` 인터페이스 패턴 그대로 재사용 (store import 금지). 검증: `audit` 패키지가 `store` 미import |
| R-EVID-002 | 트랜잭션 원자성 — orphan 증빙 행 | research.md §8 Risk 3 | BeginTx→...→Commit 단일 TX, audit 실패 시 `tx.Rollback`. **`database_blob` 확정으로 blob bytes도 동일 TX → orphan-cleanup 머신러리 불요(잔여 위험 추가 감소, strategy.md §2.6.3)**. AC-EVID-003-3 / AC-EVID-UBI-002 검증. 핸들러 블록 `@MX:WARN` |
| R-EVID-003 | user_id 누출 (cli-anonymous 미준수) | research.md §8 Risk 2 | `Recorder.resolveUserID` 재사용, `created_by` DEFAULT 'cli-anonymous'. AC-EVID-UBI-003로 검증 |
| R-EVID-004 | 동일 evaluation_item_id 동시 재업로드 → 중복 version | research.md §8 Risk 3 | `GetLatestVersionByEvalItem`에 SELECT FOR UPDATE. AC-EVID-001-4로 검증. 버전 결정 로직 `@MX:WARN` |
| R-EVID-005 | 신규 pgx pool 생성 (싱글톤 위반) / phantom-path | research.md §8 Risk 6, strategy.md §0 | **실 pool `PgWorkflowStore.pool`(`pg_store.go`) 재사용** — `postgres.go`는 Sprint-0 死 스텁이므로 비대상. evidence TX 진입점만 추가. 코드 리뷰 게이트 |
| R-EVID-006 | 데이터 주권 위반 (외부 저장 호출) | research.md §6, §8 Risk 4 | **`database_blob` 확정 = 외부 endpoint 0건 by construction** (내부망 PG만, MinIO/S3 SDK 미사용). REQ-EVID-004-U1 게이트. AC-EVID-004-2로 외부 egress 0건 검증 (잔여 위험 매우 낮음) |
| R-EVID-007 | `time.Now().UTC()` 직접 호출 테스트 비친화 | research.md §8 Risk 7 | Sprint 3 REFACTOR에서 `clock.go` clock 주입 |
| R-EVID-008 | initial.sql 수정으로 cross-table schema drift | research.md §7 | `0002_evidence_tables.sql` 분리, initial.sql 불변. spec.md §2.2 [EXISTING] 마커 |

---

## 8. Cross-SPEC 영향 (Lesson #5 의무 섹션)

- **영향 받는 기존 generated artifact**: 없음. 본 SPEC은 SPEC-AX-CTRL-001의 `celery_envelope_v2.json` 골든 파일, 기존 store/audit 동작을 수정하지 않는다 (additive only).
- **DB 스키마 경로 규약 (research.md §2 정합, spec.md §2.2와 통일)**: `.moai/db/schema/initial.sql` = 기존 baseline 참조 스키마 (documents/audit_logs 등, [EXISTING], 본 SPEC 미수정). `.moai/db/schema/migrations/NNNN_*.sql` = 순차 적용 마이그레이션 파일. 현재 `migrations/` 디렉토리에는 `0001_initial.sql` 단일 파일만 실재한다 (`initial.sql`의 멱등 복사 포인터). 본 SPEC의 신규 마이그레이션은 `.moai/db/schema/migrations/0002_evidence_tables.sql`이다.
- **`0002_evidence_tables.sql` 파일번호 비충돌 확정 (조건부 표현 제거)**: 검증 결과 `.moai/db/schema/migrations/`에 실재하는 파일은 `0001_initial.sql` 뿐이며 `0002_*`는 존재하지 않으므로, `0002_evidence_tables.sql` 번호는 **충돌 없이 확정 사용 가능**하다. SPEC-AX-CTRL-001 §2.3이 언급한 `0002_workflow_indexes.sql`은 현재 migrations/에 미커밋(파일 부재) 상태이므로 본 SPEC의 `0002_evidence_tables.sql`과 파일시스템 충돌이 발생하지 않는다. (결정 owner: 본 SPEC. run 단계에서 별도 협의 불요 — CTRL-001 인덱스 마이그레이션이 추후 커밋되면 그 SPEC이 다음 가용 번호 `0003`을 취한다.)
- **upstream Exclusion 역참조**: 없음 (본 SPEC이 다른 SPEC의 deferred 항목을 해소하지 않음).
- **downstream 추적**: 평가항목 taxonomy는 `evidences.evaluation_item_id` stub으로 남으며, 미래 `SPEC-AX-EVAL-ITEM-001`(미생성)이 본 stub을 정식 FK로 승격할 때 본 SPEC §5 Exclusion #1을 역참조해야 한다.

---

## 9. Definition of Done (Plan 관점)

- [ ] §2 Sprint 0-5 전부 GREEN
- [ ] `acceptance.md` 전체 AC 자동화 테스트 통과
- [ ] 커버리지 ≥ 85%, golangci-lint default+gosec 0, goleak 통과
- [ ] §5 MX 태그 매핑 완료 (RED `@MX:TODO` 전부 해소)
- [ ] §6 저장 전략 RESOLVED=`database_blob` 반영 (DDL `file_content BYTEA`, config 기본값, EvidenceBlobStore 정합 구현)
- [ ] 기존 WorkflowStore/Recorder 특성화 회귀 0건
- [ ] manager-quality TRUST 5 통과, evaluator-active strict ≥ 0.75
