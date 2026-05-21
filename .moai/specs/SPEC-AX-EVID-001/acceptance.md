# SPEC-AX-EVID-001 Acceptance Criteria

> Format: Given / When / Then
> Methodology: TDD — 각 AC가 RED phase 단위 테스트로 1:1 매핑 (RED→GREEN 컬럼 명시)
> Tooling: Go 1.22 + testify/assert + testcontainers-go(postgres:16-pgvector) + goleak
> AC 명명: `AC-EVID-{REQ}-{N}` (예: AC-EVID-001-1, AC-EVID-UBI-002-A)

본 문서는 SPEC-AX-EVID-001의 5개 REQ 모듈(1 Ubiquitous 묶음 + 4 modal)에 대한 acceptance criteria를 정의한다. 각 AC는 자동화 테스트로 검증 가능해야 하며 모호한 표현("적절히", "신속하게")을 사용하지 않는다.

---

## §0. REQ-EVID-UBI Transverse Invariants — Acceptance

§1-§4 modal REQ를 가로지르는 4개 Ubiquitous 불변 조건의 dedicated AC. SPEC-AX-CTRL-001 §0 AC-CTRL-UBI-* 패턴을 동일 구조로 적용한다.

### AC-EVID-UBI-001 (Data Sovereignty — No External Storage Call)

REQ 대응: REQ-EVID-UBI-001 (데이터 주권 — 저장/조회/해시 경로 외부 호출 0건).
TDD: RED `recorder_evidence_test.go` 외부 host 차단 환경 → GREEN 표준 라이브러리 `crypto/sha256` + 내부 자원만.

**Given**:
- 테스트 환경이 외부 네트워크 egress를 차단 (loopback/내부망 host만 허용)
- 증빙 생성 경로가 SHA-256 해시 + `EvidenceTx` INSERT를 수행

**When**:
- 클라이언트가 증빙 생성 엔드포인트를 정상 호출한다 (50 MiB 이하 파일)

**Then**:
- 증빙 생성이 성공한다 (HTTP 201)
- 해시/저장/감사 경로에서 외부 host로의 DNS 해석 또는 TCP 연결이 0건 (네트워크 spy 또는 정적 import 검사)
- `crypto/sha256` 표준 라이브러리만 사용 (외부 해시 서비스 호출 없음)

### AC-EVID-UBI-002-A (Audit Completeness — Evidence Creation)

REQ 대응: REQ-EVID-UBI-002 (감사 가능성 — 모든 create에 동일 TX `audit_logs` 1건).
TDD: RED audit row 부재 검증 → GREEN RecordEvidenceCreated 동일 TX 기입.

**Given**:
- PostgreSQL `evidences` + `audit_logs` 테이블이 testcontainers fixture로 clean state
- AuthN disabled (Walking Skeleton 기본값)

**When**:
- 클라이언트가 신규 `evaluation_item_id="eval-ubi-002a"`로 증빙을 생성한다

**Then**:
- `audit_logs`에 정확히 1개 row 존재: `action='EVIDENCE_CREATED'`, `resource_type='evidence'`, `resource_id=<반환된 evidence_id>`, `user_id='cli-anonymous'`
- `details` JSONB가 `{evaluation_item_id:"eval-ubi-002a", version:1, file_hash_sha256:<64hex>}` 포함
- `evidences` row 1개 + `audit_logs` row 1개가 동일 트랜잭션 커밋 (한쪽만 존재하는 상태 없음)

### AC-EVID-UBI-002-B (Audit Completeness — Evidence Versioning)

REQ 대응: REQ-EVID-UBI-002 (버전 이벤트별 audit row 1:1).
TDD: RED 버전 audit 부재 → GREEN RecordEvidenceVersioned.

**Given**:
- `evaluation_item_id="eval-ubi-002b"`에 version=1 증빙이 이미 존재

**When**:
- 동일 `evaluation_item_id`로 증빙을 재업로드한다 (버전 생성)

**Then**:
- `audit_logs`에 `action='EVIDENCE_VERSIONED'` row 정확히 1개: `resource_id=<새 version row id>`, `user_id='cli-anonymous'`
- `details` JSONB가 `{evaluation_item_id:"eval-ubi-002b", version:2, previous_version_id:<v1 id>}` 포함
- 동일 TX 내 새 evidence row(version=2) + audit row 함께 커밋

### AC-EVID-UBI-003 (cli-anonymous Default)

REQ 대응: REQ-EVID-UBI-003 (AuthN disabled 시 created_by/user_id = 'cli-anonymous').
TDD: RED user_id NULL/실사용자 검증 실패 → GREEN resolveUserID 재사용.

**Given**:
- Walking Skeleton 환경 (`AUTH_ENABLED=false`)
- 클라이언트가 인증 헤더 없이 요청

**When**:
- 증빙을 생성한다 (no auth header)

**Then**:
- `evidences.created_by = 'cli-anonymous'` (정확히 literal, NULL 금지)
- `audit_logs.user_id = 'cli-anonymous'`
- 두 컬럼이 byte-identical (cross-table consistency, SPEC-AX-CTRL-001 AC-CTRL-UBI-002-C 정합)

### AC-EVID-UBI-004 (Version Immutability — Prior Version Preserved)

REQ 대응: REQ-EVID-UBI-004 (이전 버전 byte-identical 보존, 물리 삭제 금지).
TDD: RED 이전 버전 변경/삭제 탐지 → GREEN store mutation guard.

**Given**:
- `evaluation_item_id="eval-ubi-004"`에 version=1 (id=`ev1`, file_hash=`H1`)이 존재

**When**:
- 동일 item으로 version=2를 생성한다 (id=`ev2`, file_hash=`H2`)

**Then**:
- `ev1` row가 여전히 조회 가능 (`GetEvidenceByID(ev1)`): `file_name`, `file_size_bytes`, `file_hash_sha256=H1`, `content_type`, `metadata` 모두 변경 전과 byte-identical
- `ev1.status`는 `'ACTIVE'` → `'SUPERSEDED'`로만 전이 (본문 컬럼은 불변)
- `ev1` row 물리 삭제 0건 (`DELETE` 미발생)
- `ev2.previous_version_id = ev1.id`

---

## §1. REQ-EVID-001 증빙 데이터 모델 & Store — Acceptance

### AC-EVID-001-1 (Happy Path: 증빙 생성 + 감사 원자적 커밋)

TDD: RED InsertEvidence 미구현 → GREEN evidence.go pgx 구현.

**Given**:
- `evidences`/`audit_logs` 테이블 clean state (testcontainers)
- 신규 `evaluation_item_id="eval-001-1"`, 1 MiB 유효 파일

**When**:
- 클라이언트가 증빙 생성 엔드포인트를 호출한다

**Then**:
- HTTP 201, body `{"evidence_id":"<uuid>","version":1}`
- `evidences`에 정확히 1 row: `version=1`, `previous_version_id IS NULL`, `file_hash_sha256`=실제 SHA-256, `storage_strategy` ∈ {filesystem,database_blob,minio}
- `audit_logs`에 `EVIDENCE_CREATED` 1 row (동일 TX)
- 응답 < 150ms p99 (10회 반복, 파일 물리 저장 제외)

### AC-EVID-001-2 (Edge — Oversized / Empty File 거부)

TDD: RED 크기 검증 부재 → GREEN 핸들러 pre-TX 검증.

**Given**:
- `EVIDENCE_MAX_FILE_BYTES=50MiB` 설정

**When**:
- 클라이언트가 (a) 51 MiB 파일, 또는 (b) 0-byte 파일, 또는 (c) `evaluation_item_id` 누락으로 증빙 생성을 시도한다

**Then**:
- HTTP 400, body `{"error":{"code":"INVALID_ARGUMENT","message":...,"field":<해당 필드>}}`
- `evidences` 변화 0건, `audit_logs` 변화 0건 (트랜잭션 미진입)
- 서버 로그 레벨 = INFO (client error)

### AC-EVID-001-3 (Edge — eval_item_id Stub: 참조 테이블 부재)

TDD: RED FK 제약 가정 테스트 실패 → GREEN FK 없는 VARCHAR stub.

**Given**:
- `evaluation_items` 테이블이 데이터베이스에 **존재하지 않는다** (의도된 설계)
- 임의 문자열 `evaluation_item_id="nonexistent-item-xyz-999"`

**When**:
- 해당 `evaluation_item_id`로 증빙을 생성한다

**Then**:
- 증빙 생성이 성공한다 (HTTP 201) — FK 제약 위반 에러 없음 (참조 무결성 미강제는 의도)
- `evidences.evaluation_item_id = 'nonexistent-item-xyz-999'`로 그대로 저장
- 스키마에 `evidences` → `evaluation_items` FK가 존재하지 않음 (정보 스키마 쿼리로 확인)

### AC-EVID-001-4 (Concurrent Versioning — SELECT FOR UPDATE 직렬화)

TDD: RED 동시 재업로드 중복 version → GREEN GetLatestVersionByEvalItem FOR UPDATE.

**Given**:
- `evaluation_item_id="eval-001-4"`에 version=1 존재
- 2개 goroutine G1, G2가 거의 동시(<1ms)에 동일 item으로 재업로드 시도

**When**:
- G1, G2가 각각 `EvidenceTx` 내에서 `GetLatestVersionByEvalItem("eval-001-4")` (SELECT FOR UPDATE) 후 새 버전 INSERT

**Then**:
- 정확히 version=2, version=3이 생성됨 (중복 version=2 없음, gap 없음)
- 두 row의 `previous_version_id` 체인이 단절 없이 v1→v2→v3 (한쪽이 다른 쪽 직전 가리킴)
- 두 TX 모두 결국 성공 (deadlock 없음)
- `goleak.VerifyNone(t)` 통과

### AC-EVID-001-O1-1 (Optional — Duplicate SHA-256 → non-blocking `duplicate_of` 신호)

REQ 대응: REQ-EVID-001-O1 (동일 evaluation_item_id에 동일 SHA-256 재제출 시 비차단 `duplicate_of` 신호 노출, 생성 거부 안 함).
TDD: RED `duplicate_of` 응답 필드 부재 → GREEN 핸들러 SHA-256 일치 시 신호 부착.
주의: REQ-EVID-001-O1은 `MAY`/Optional이며 "Sandbox PoC에서는 비활성 가능"(spec.md REQ-EVID-001-O1). 본 AC는 기능이 활성(`EVIDENCE_DUPLICATE_SIGNAL_ENABLED=true`)인 경우에만 검증하며, 비활성 시에는 skip 처리한다.

**Given**:
- `evaluation_item_id="eval-001-o1"`에 version=1 (id=`ev1`, `file_hash_sha256=H_dup`)이 이미 존재
- 중복 신호 기능 활성 (`EVIDENCE_DUPLICATE_SIGNAL_ENABLED=true`)

**When**:
- 동일 `evaluation_item_id="eval-001-o1"`에 **byte-identical 동일 파일**(SHA-256 = `H_dup`)을 재제출한다

**Then**:
- 생성이 **거부되지 않는다** (HTTP 201 — 합법적 버전 생성으로 처리)
- `evidences`에 새 version=2 row가 정상 INSERT (`previous_version_id=ev1`, `file_hash_sha256=H_dup`)
- 응답 본문에 비차단 신호 `"duplicate_of": "<ev1 evidence_id>"`가 노출됨 (정확히 직전 동일 해시 row의 id)
- 신호는 정보 제공용이며 워크플로우/트랜잭션을 차단하지 않음 (status code, audit 기록 모두 정상 create/version 경로와 동일)

**Edge — 기능 비활성 (Sandbox PoC 기본):**
- `EVIDENCE_DUPLICATE_SIGNAL_ENABLED` 미설정 또는 `false`
- 동일 시나리오 호출 시 HTTP 201 + 정상 version=2 생성은 동일하나, 응답 본문에 `duplicate_of` 필드가 **존재하지 않는다** (Optional 미활성 — 정상 동작)
- 이는 REQ-EVID-001-O1의 `MAY` 의미론과 일치 (테스트는 비활성 모드에서 `duplicate_of` 부재를 검증, 활성 모드에서 존재를 검증)

---

## §2. REQ-EVID-002 버전 관리 — Acceptance

### AC-EVID-002-1 (Re-upload → version=2 + previous_version_id 체이닝)

TDD: RED 재업로드가 version=1 덮어씀(잘못) → GREEN 새 행 INSERT.

**Given**:
- `evaluation_item_id="eval-002-1"`에 version=1 (id=`ev1`)

**When**:
- 동일 item으로 다른 파일을 재업로드한다

**Then**:
- 새 row `ev2`: `version=2`, `previous_version_id=ev1`, `status='ACTIVE'`
- `ev1.status='SUPERSEDED'`로 전이 (본문 불변)
- `evidences` 총 2 row (v1 보존)

### AC-EVID-002-2 (Prior Version Retrievable)

TDD: RED GetEvidenceByID(이전버전) 실패 → GREEN 전체 조회.

**Given**:
- `eval-002-2`에 v1(`ev1`), v2(`ev2`) 존재

**When**:
- `GetEvidenceByID(ev1)` 및 `ListEvidenceByEvalItem("eval-002-2")` 호출

**Then**:
- `GetEvidenceByID(ev1)`이 v1 row를 정상 반환 (삭제/소실 없음)
- `ListEvidenceByEvalItem`이 [v2, v1] (version DESC) 2개 반환
- 어떤 버전도 물리 삭제되지 않음

### AC-EVID-002-3 (Edge — 3-Deep Version Chain 보존 + 이전 버전 불변)

TDD: RED 3-deep 체인 단절 / 이전 버전 본문 UPDATE 허용(잘못) → GREEN mutation guard.

**Given**:
- `eval-002-3`에 v1(`ev1`) → v2(`ev2`) → v3(`ev3`) 3-deep 체인

**When**:
- (a) 체인 순회: `ev3.previous_version_id`→`ev2`→`ev1`→NULL
- (b) `ev1`의 `file_hash_sha256` 컬럼 UPDATE 시도 (successor 존재)

**Then** (a):
- 단절 없는 단일 연결 리스트 (v3→v2→v1, v1.previous_version_id IS NULL)
- 3 row 모두 조회 가능

**Then** (b):
- store 계층이 mutation을 거부 (에러 반환, SQL 미실행 — REQ-EVID-002-U1)
- `ev1.file_hash_sha256` 변경 0건 (불변 보장)

---

## §3. REQ-EVID-003 감사 연계 — Acceptance

### AC-EVID-003-1 (RecordEvidenceCreated Audit Row)

TDD: RED RecordEvidenceCreated 미구현 → GREEN recorder.go 메서드 추가.

**Given**:
- `audit_logs` clean state, Recorder(`authEnabled=false`)

**When**:
- 증빙 생성 TX가 `Recorder.RecordEvidenceCreated(ctx, tx, evidenceID, evalItemID, hash, version, userID="")` 호출

**Then**:
- `audit.Event`: `Action="EVIDENCE_CREATED"`, `ResourceType="evidence"`, `ResourceID=evidenceID`(uuid), `UserID="cli-anonymous"`(resolveUserID), `Timestamp` NOT NULL
- `DetailsJSON`에 `{evaluation_item_id, version:1, file_hash_sha256}` 포함
- 동일 `AuditTx`로 INSERT (store→audit 순환 의존 없음 — `audit` 패키지가 `store` 미import)

### AC-EVID-003-2 (RecordEvidenceVersioned Audit Row)

TDD: RED RecordEvidenceVersioned 미구현 → GREEN 메서드 추가.

**Given**:
- v1 존재, 버전 생성 TX 진행 중

**When**:
- `Recorder.RecordEvidenceVersioned(ctx, tx, newEvidenceID, evalItemID, version=2, previousVersionID, userID="")` 호출

**Then**:
- `Action="EVIDENCE_VERSIONED"`, `ResourceID=newEvidenceID`
- `DetailsJSON`에 `{evaluation_item_id, version:2, previous_version_id}` 포함
- `user_id="cli-anonymous"`

### AC-EVID-003-3 (Edge — Audit Fail → 증빙+감사 양방향 Rollback 원자성)

TDD: RED audit 실패 시 evidence row 잔존(잘못) → GREEN tx.Rollback 양방향.

**Given**:
- `evidences`/`audit_logs` clean state
- Test harness가 `audit_logs` INSERT에 fault injection (CHECK constraint violation 강제)

**When**:
- 증빙 생성 TX가 (a) `evidences` INSERT 성공 후 (b) `audit_logs` INSERT가 실패한다
- 핸들러가 `tx.Rollback(ctx)` 호출

**Then**:
- `evidences` 테이블에 row **존재하지 않음** (evidence INSERT도 rollback)
- `audit_logs` 테이블에 row 없음 (애초에 실패)
- 핸들러 반환 에러 = wrapped audit insertion failure
- 부분 커밋 0건 (all-or-nothing, REQ-EVID-003-U1)
- `goleak.VerifyNone(t)` 통과
- 이는 SPEC-AX-CTRL-001 AC-CTRL-UBI-001 패턴의 증빙 도메인 대응

---

## §4. REQ-EVID-004 저장 전략 추상화 — Acceptance

### AC-EVID-004-1 (storage_strategy 컬럼 — 열거값 강제)

TDD: RED NULL/임의값 허용(잘못) → GREEN CHECK 제약 + store 검증.

**Given**:
- `evidences` 테이블 (CHECK `storage_strategy IN ('filesystem','database_blob','minio')`)

**When**:
- (a) `EVIDENCE_STORAGE_STRATEGY='filesystem'`로 증빙 생성
- (b) 직접 SQL로 `storage_strategy='external_s3'` (열거 외) INSERT 시도

**Then** (a):
- `evidences.storage_strategy = 'filesystem'`로 저장

**Then** (b):
- CHECK 제약 위반으로 INSERT 거부 (또는 store 계층 사전 검증 에러)
- NULL 값도 거부 (NOT NULL)

### AC-EVID-004-2 (Edge — External Service Call 부적격)

REQ 대응: REQ-EVID-004-U1 + REQ-EVID-UBI-001.
TDD: RED 외부 host egress 탐지 → GREEN 저장/해시 경로 내부망 only.

**Given**:
- 테스트 환경 외부 host 차단, 코드 정적 import 검사 도구

**When**:
- 증빙 생성/조회 전체 경로 실행 + `internal/storage`, `internal/store`, `cmd/server/evidence_handlers.go` 정적 분석

**Then**:
- 저장/해시/조회 경로에서 외부 host TCP/DNS 0건
- `internal/storage` 패키지가 외부 SaaS SDK(aws-sdk, 외부 s3 client 등) 미import
- 본 SPEC 산출물에 concrete `EvidenceBlobStore` 구현체 0개 (인터페이스만 — REQ-EVID-004-O1)

### AC-EVID-004-3 (EvidenceBlobStore 추상화 — 전략 교체 가능)

TDD: RED 인터페이스 부재 → GREEN storage.go 인터페이스 정의.

**Given**:
- `internal/storage/storage.go`에 `EvidenceBlobStore` 인터페이스 정의

**When**:
- 컴파일 타임에 인터페이스 메서드 시그니처 (`Put(ctx,key,reader)(string,error)`, `Get(ctx,location)(reader,error)`) 검사
- 핸들러가 `EvidenceBlobStore` 타입에 의존 (concrete 타입 아님)

**Then**:
- 인터페이스가 정의되어 있고, 핸들러가 인터페이스 의존 (전략 swap 가능 구조)
- 본 SPEC은 nil/no-op placeholder만 허용, 실 구현체는 §6 OPEN DECISION 이후
- 인터페이스 메서드가 전략 선택과 무관하게 안정 (filesystem/blob/minio 어느 것도 시그니처 변경 불요)

---

## §5. Korean Public-Sector Constraint Acceptance (표 포맷)

SPEC-AX-CTRL-001 §4 표 패턴. 한국 공공 6제약 중 본 SPEC 적용 항목.

| 제약 | 검증 기준 | 대응 AC | 측정 방법 |
|------|----------|---------|-----------|
| 데이터 주권 | 저장/조회/해시 외부 호출 0건, 외부 SaaS SDK 미import | AC-EVID-UBI-001, AC-EVID-004-2 | 네트워크 spy + 정적 import 검사 |
| 감사 가능성 | 모든 evidence create/version → 동일 TX audit_logs 1건, 누락 0 | AC-EVID-UBI-002-A/B, AC-EVID-003-1/2 | testcontainers row count |
| cli-anonymous 기본값 | AuthN disabled 시 created_by/user_id='cli-anonymous' literal | AC-EVID-UBI-003 | 컬럼 byte 비교 |
| 버전 무결성 | 이전 버전 byte-identical 보존, 물리 삭제 0건 | AC-EVID-UBI-004, AC-EVID-002-2/3 | GetEvidenceByID 회귀 |
| 망분리 정합 | 채택 가능 저장 전략이 내부망 only (외부 S3 영구 부적격) | AC-EVID-004-2 | REQ-EVID-004-U1 게이트 |

---

## §6. Edge Case Catalog

plan.md §7 R-EVID-001~008 risk register 매핑.

| Edge Case | 대응 AC | Risk ID |
|-----------|--------|---------|
| 재업로드 버전 체이닝 (1→2→3) | AC-EVID-002-1, AC-EVID-002-3 | R-EVID-004 |
| 동일 evaluation_item_id 동시 재업로드 (중복 version race) | AC-EVID-001-4 | R-EVID-004 |
| audit INSERT 실패 → evidence+audit 양방향 rollback | AC-EVID-003-3, AC-EVID-UBI-002-A/B | R-EVID-002 |
| eval_item_id stub, 참조 테이블 부재 (FK 미강제) | AC-EVID-001-3 | (의도된 설계) |
| oversized / empty / 누락 파일 거부 | AC-EVID-001-2 | (입력 검증) |
| 중복 SHA-256 (duplicate 신호, non-blocking) | AC-EVID-001-O1-1 | (선택 기능, REQ-EVID-001-O1) |
| 이전 버전 본문 UPDATE 시도 거부 | AC-EVID-002-3, AC-EVID-UBI-004 | (불변식 — REQ-EVID-UBI-004) |
| storage_strategy 열거 외 값 / NULL 거부 | AC-EVID-004-1 | R-EVID-006 |
| 외부 저장 서비스 호출 부적격 | AC-EVID-004-2 | R-EVID-006 |
| store→audit 순환 의존 회피 (로컬 AuditTx) | AC-EVID-003-1 | R-EVID-001 |

---

## §7. TDD RED/GREEN 매핑 요약

| AC | RED (실패 테스트 작성) | GREEN (최소 구현) |
|----|------------------------|-------------------|
| AC-EVID-001-1 | InsertEvidence 미구현 → undefined 메서드 | evidence.go pgx InsertEvidence + 핸들러 |
| AC-EVID-001-2 | 크기/필드 검증 부재 → oversized가 201 | 핸들러 pre-TX 검증 |
| AC-EVID-001-3 | FK 제약 가정 테스트 → schema 불일치 | FK 없는 VARCHAR stub DDL |
| AC-EVID-001-4 | 동시 재업로드 중복 version | GetLatestVersionByEvalItem FOR UPDATE |
| AC-EVID-001-O1-1 | `duplicate_of` 응답 필드 부재 (활성 모드) | 핸들러 SHA-256 일치 시 비차단 신호 부착 (Optional, env-gated) |
| AC-EVID-002-1/2/3 | 재업로드가 v1 덮어씀 / 체인 단절 | 새 행 INSERT + previous_version_id + mutation guard |
| AC-EVID-003-1/2 | RecordEvidence* 미구현 | recorder.go 메서드 2개 (기존 시그니처 패턴) |
| AC-EVID-003-3 | audit 실패 시 evidence 잔존 | 핸들러 tx.Rollback 양방향 |
| AC-EVID-004-1/2/3 | storage_strategy NULL 허용 / 인터페이스 부재 | CHECK 제약 + EvidenceBlobStore 인터페이스 |
| AC-EVID-UBI-001~004 | sovereignty/audit/cli-anonymous/immutability 위반 탐지 | 표준 sha256 + Recorder 재사용 + resolveUserID + mutation guard |

---

## §8. Definition of Done (Acceptance Phase)

모두 PASS 필요:

- [ ] §0: REQ-EVID-UBI 전용 AC 5개 (UBI-001, UBI-002-A, UBI-002-B, UBI-003, UBI-004) 자동화 통과
- [ ] §1-§4: 4개 modal REQ AC 자동화 통과 (AC-EVID-001-{1..4, O1-1}, AC-EVID-002-{1..3}, AC-EVID-003-{1..3}, AC-EVID-004-{1..3})
- [ ] §5: 한국 공공 5제약 검증 통과
- [ ] §6: 10개 edge case 모두 대응 AC로 검증 (REQ-EVID-001-O1은 AC-EVID-001-O1-1 전용 AC로 매핑, Optional env-gated)
- [ ] coverage ≥ 85% (go test -cover)
- [ ] golangci-lint default + gosec 0 issue
- [ ] `goleak.VerifyNone(t)` 모든 테스트 통과
- [ ] 기존 WorkflowStore/Recorder 특성화 회귀 0건
- [ ] @MX 태그 plan.md §5 매핑 완료
- [ ] manager-quality TRUST 5 통과
- [ ] evaluator-active per-sprint scoring 모두 ≥ 0.75 (strict profile)

**Total AC count**: 19 — (§0 UBI: 5 [UBI-001, UBI-002-A, UBI-002-B, UBI-003, UBI-004], §1: 5 [AC-EVID-001-1..4, AC-EVID-001-O1-1], §2: 3 [AC-EVID-002-1..3], §3: 3 [AC-EVID-003-1..3], §4: 3 [AC-EVID-004-1..3]). 각 modal REQ 모듈은 최소 3개 AC (≥2 요건 충족). v0.1.1에서 AC-EVID-001-O1-1 추가로 REQ-EVID-001-O1 Optional coverage 확보 (이전 §6 catalog의 AC-EVID-001-1 오매핑 정정, 18→19).
