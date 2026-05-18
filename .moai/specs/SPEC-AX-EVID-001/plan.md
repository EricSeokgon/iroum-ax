# SPEC-AX-EVID-001 구현 계획 (Plan)

> 대응 SPEC: `.moai/specs/SPEC-AX-EVID-001/spec.md` v0.1.1
> 방법론: TDD (RED-GREEN-REFACTOR), harness: thorough
> 근거: `.moai/specs/SPEC-AX-EVID-001/research.md` (Phase 0.5 deep research, file:line 근거 포함)

---

## 1. 개요 및 접근 방식

본 SPEC은 SPEC-AX-CTRL-001이 확립한 **`WorkflowStore`/`WorkflowTx` + `Recorder`/`AuditTx` 트랜잭션 원자성 패턴**(research.md §2, §3)을 증빙 도메인으로 brownfield 확장한다. 핵심 설계 결정은 다음과 같다:

1. **데이터 모델 우선 (1st deliverable)**: `evidences` 테이블 + `EvidenceStore`/`EvidenceTx` 인터페이스를 먼저 GREEN으로 만든다. 업로드 UX는 범위 밖.
2. **버전 체이닝**: 재업로드는 UPDATE가 아닌 새 행 INSERT (`previous_version_id` 자기 참조). 이전 버전 불변 (REQ-EVID-UBI-004).
3. **감사 원자성**: 증빙 변경과 `audit_logs` 기입을 단일 `EvidenceTx` 내에서 atomic 처리 (기존 `Recorder` 패턴 확장).
4. **저장 전략 추상화 (결정 이연)**: `EvidenceBlobStore` 인터페이스 + `storage_strategy` 컬럼만 정의. 구체 백엔드는 §6 OPEN DECISION.
5. **평가항목 경량 stub**: `evaluation_item_id`는 FK 없는 VARCHAR. 참조 테이블 미생성.

### 1.1 Brownfield Delta 요약

| 마커 | 대상 | 처리 |
|------|------|------|
| [EXISTING] | `internal/store/postgres.go` 기존 WorkflowStore, `internal/audit/recorder.go` 기존 8 메서드, `cmd/server` 기존 핸들러 | 특성화 회귀 테스트로 보존 — 동작 변경 0건 |
| [NEW] | `internal/store/evidence.go`, `internal/storage/storage.go`, `cmd/server/evidence_handlers.go`, `0002_evidence_tables.sql`, `internal/audit/clock.go` | 신규 추가 |
| [MODIFY] | `internal/audit/audit.go` (액션 상수 2개 추가), `internal/store/store.go` (인터페이스 2개 추가), `internal/store/postgres.go` (evidence TX 진입점 와이어링) | 기존 심볼 불변, additive only |

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
- GREEN: [NEW] `evidence.go` pgx 구현, [MODIFY] `postgres.go` 기존 pool에 evidence TX 진입점 와이어링
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
    storage_location    VARCHAR(255),
    storage_strategy    VARCHAR(32) NOT NULL,                          -- 'filesystem'|'database_blob'|'minio'
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

---

## 5. MX 태그 계획

research.md §8 Risk 1/3/6/7 및 SPEC-AX-CTRL-001 기존 태그 패턴(`store.go:16` `@MX:ANCHOR`/`@MX:REASON`)을 따른다.

| 대상 심볼 | 태그 | 근거 (@MX:REASON) |
|-----------|------|-------------------|
| `EvidenceStore` 인터페이스 (`store.go`) | `@MX:ANCHOR` | fan_in ≥ 3 (evidence_handlers, recorder 연계, 향후 list 조회) — 기존 WorkflowStore와 동일 |
| `EvidenceTx` 인터페이스 (`store.go`) | `@MX:ANCHOR` | AC-EVID-003-3 / AC-EVID-UBI-002 원자성 계약의 핵심 (FakeStore + pgx 구현체 2곳 구현) |
| `RecordEvidenceCreated` / `RecordEvidenceVersioned` (`recorder.go`) | `@MX:ANCHOR` | 증빙 감사 기록 단일 진입점 (REQ-EVID-UBI-002 AC 전체가 이 메서드 경유) |
| 증빙 생성/버전 핸들러의 BeginTx→InsertEvidence→InsertAuditLog→Commit 블록 (`evidence_handlers.go`) | `@MX:WARN` | `@MX:REASON`: 트랜잭션 원자성 — InsertAuditLog 이후 Commit 전 panic/early-return 시 orphan 증빙 행 누출 (research.md §8 Risk 3). Rollback defer 순서 변경 금지 |
| `EvidenceBlobStore` 인터페이스 (`storage.go`) | `@MX:NOTE` | 저장 전략 미선택 추상화 — 구현체 0개, 전략 결정 후 확장 (§6 OPEN DECISION 참조) |
| 버전 결정 로직 (max version → +1 → previous_version_id) | `@MX:WARN` | `@MX:REASON`: SELECT FOR UPDATE 락 미보유 시 동일 evaluation_item_id 동시 재업로드가 중복 version 번호 / orphan 체인 생성 (REQ-EVID-001-S1) |

TDD MX 흐름: RED에서 `@MX:TODO`(미구현 마커) → GREEN에서 제거 → REFACTOR에서 `@MX:NOTE`/`@MX:ANCHOR` 확정.

---

## 6. OPEN DECISION — 파일 바이너리 저장 전략 (결정 이연)

> **상태**: UNRESOLVED — run/strategy 단계에서 결정. 본 SPEC은 추상화(`EvidenceBlobStore` + `storage_strategy` 컬럼)만 정의하며 어떤 전략도 선택하지 않는다. 이는 미명세 갭이 아니라 **문서화된 명시적 미결정**이다.

### 6.1 트레이드오프 표 (research.md §9 인용)

| Strategy | Pros | Cons | 망분리 호환 (REQ-EVID-UBI-001) |
|----------|------|------|-------------------------------|
| **Filesystem** (`/srv/evidence/`) | 단순, 빠름, DB 비대화 없음, 외부 의존 0 | 백업 복잡, 멀티노드 PV 공유 제약, 정합성 별도 보장 필요 | YES (내부 PV) |
| **DB BLOB** (PostgreSQL BYTEA) | evidence 레코드와 원자적 (단일 TX 일관성), 백업 일원화 | PG 저장 한계, dedup 없음, 대용량 조회 느림, WAL 비대화 | YES (내부 PG) |
| **Self-hosted MinIO** (내부망 S3 호환) | 확장성, S3 API 표준, 멀티노드 친화 | 추가 서비스 운영 부담, **외부 관리형 S3는 영구 부적격** | MAYBE — self-hosted 내부망 배포 시에만. 외부 endpoint는 REQ-EVID-004-U1 위반 |

### 6.2 결정 게이트 (run/strategy 단계 입력)

선택된 전략은 다음을 **모두** 만족해야 한다:
- (필수) REQ-EVID-UBI-001: 저장/조회/해시 경로의 외부 네트워크 호출 0건
- (필수) REQ-EVID-003-U1: 증빙 행 rollback 시 저장된 바이너리도 일관 처리(고아 파일 방지) 가능
- (권장) 단일 PoC 노드 배포 단순성 (KEPCO E&C PoC는 단일 노드 전제 — product.md §3.2)

### 6.3 본 SPEC이 보장하는 것

- `storage_strategy` 컬럼이 어떤 전략을 선택하든 기록 (열거값 CHECK 제약)
- `EvidenceBlobStore` 인터페이스 계약이 전략 교체를 허용 (구현체는 추후)
- 데이터 모델/store/audit 계층은 전략 선택과 **독립적으로** GREEN 가능 (Walking Skeleton이 전략 결정에 블록되지 않음)

---

## 7. 리스크 분석 (research.md §8 매핑)

| Risk ID | 설명 | 출처 | 완화 |
|---------|------|------|------|
| R-EVID-001 | store→audit 순환 의존 | research.md §8 Risk 1 | `recorder.go`의 로컬 `AuditTx` 인터페이스 패턴 그대로 재사용 (store import 금지). 검증: `audit` 패키지가 `store` 미import |
| R-EVID-002 | 트랜잭션 원자성 — orphan 증빙 행 | research.md §8 Risk 3 | BeginTx→...→Commit 단일 TX, audit 실패 시 `tx.Rollback`. AC-EVID-003-3 / AC-EVID-UBI-002로 검증. 핸들러 블록 `@MX:WARN` |
| R-EVID-003 | user_id 누출 (cli-anonymous 미준수) | research.md §8 Risk 2 | `Recorder.resolveUserID` 재사용, `created_by` DEFAULT 'cli-anonymous'. AC-EVID-UBI-003로 검증 |
| R-EVID-004 | 동일 evaluation_item_id 동시 재업로드 → 중복 version | research.md §8 Risk 3 | `GetLatestVersionByEvalItem`에 SELECT FOR UPDATE. AC-EVID-001-4로 검증. 버전 결정 로직 `@MX:WARN` |
| R-EVID-005 | 신규 pgx pool 생성 (싱글톤 위반) | research.md §8 Risk 6 | 기존 `postgres.go` pool 재사용, evidence TX 진입점만 추가. 코드 리뷰 게이트 |
| R-EVID-006 | 데이터 주권 위반 (외부 저장 호출) | research.md §6, §8 Risk 4 | 저장 전략 미선택 + REQ-EVID-004-U1 게이트. AC-EVID-004-2로 외부 egress 0건 검증 |
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
- [ ] §6 OPEN DECISION이 run/strategy 단계로 명시 전달 (미명세 갭 아님)
- [ ] 기존 WorkflowStore/Recorder 특성화 회귀 0건
- [ ] manager-quality TRUST 5 통과, evaluator-active strict ≥ 0.75
