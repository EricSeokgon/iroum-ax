# SPEC-AX-EVID-001 — Compact (auto-generated)

> 자동 생성 condensation. spec.md/plan.md/acceptance.md에서 REQ + AC + files-to-modify + Exclusions만 추출. 개요/접근/research 참조 생략. 원본: `spec.md` v0.1.2.

## REQ 목록 (EARS)

### Ubiquitous (REQ-EVID-UBI, 4 sub)
- REQ-EVID-UBI-001 (데이터 주권): 저장/조회/해시 경로 외부 서비스 호출 0건, 내부망 자원 only.
- REQ-EVID-UBI-002 (감사 가능성): 모든 evidence create/version → 동일 TX `audit_logs` 1건.
- REQ-EVID-UBI-003 (cli-anonymous 기본값): AuthN disabled 시 `created_by`/`user_id`='cli-anonymous' literal.
- REQ-EVID-UBI-004 (버전 불변): successor 존재 시 이전 버전 본문 컬럼 UPDATE/DELETE 금지 (status ACTIVE→SUPERSEDED만 허용).

### REQ-EVID-001 — 데이터 모델 & Store
- 엔티티 `evidences`에 `file_content BYTEA` (nullable) 추가 — database_blob 시 바이너리 저장처(NOT NULL 의미), 타 전략 NULL.
- E1 (Event): 신규 증빙 → SHA-256 → 단일 EvidenceTx INSERT(version=1, prev=NULL, file_content) + audit EVIDENCE_CREATED 동일 TX → HTTP 201.
- S1 (State): 버전 결정 TX 중 동일 evaluation_item_id 최신 행에 SELECT FOR UPDATE 직렬화.
- O1 (Optional): 동일 SHA-256 존재 시 non-blocking `duplicate_of` 신호 (생성 거부 안 함).
- U1 (Unwanted): 초과 크기/빈 파일/eval_item_id 누락 → HTTP 400, TX 미진입, row 0건, INFO 로그.

### REQ-EVID-002 — 버전 관리
- E1 (Event): 기존 item 재업로드 → 새 행(version+1, previous_version_id=직전 id), 직전 status SUPERSEDED, audit EVIDENCE_VERSIONED, 단일 TX atomic.
- S1 (State): 다중 버전 시 전체 체인(prev_version_id) 단절 없이 보존, 물리 삭제 0건.
- U1 (Unwanted): successor 보유 행의 file_*/content_type/storage_location/metadata UPDATE → store 계층 거부 (SQL 미실행).

### REQ-EVID-003 — 감사 연계
- E1 (Event): RecordEvidenceCreated/Versioned → audit.Event(Action∈{EVIDENCE_CREATED,EVIDENCE_VERSIONED}, ResourceType=evidence, ResourceID=uuid, UserID=resolveUserID, Details JSONB) 동일 AuditTx INSERT.
- U1 (Unwanted): audit_logs INSERT 실패 → tx.Rollback, evidence row도 rollback (all-or-nothing), goroutine leak 0.

### REQ-EVID-004 — 저장 전략 (RESOLVED: `database_blob`)
- **확정**: storage 전략 = `database_blob` (PostgreSQL BYTEA). Run Phase 1 strategy.md §2 + Human Gate 승인. 가중 8.45 vs filesystem 5.45 vs MinIO 3.85. 동일 TX 원자성 구조적 보장 + 망분리 외부 의존 0. enum/추상화 유지(post-PoC 무중단 전환).
- S1 (State): 모든 row의 storage_strategy ∈ {'filesystem','database_blob','minio'}, NULL/열거외 금지. config `EVIDENCE_STORAGE_STRATEGY` 기본값=`database_blob`, 로드 시 enum 검증 fail-fast.
- O1 (Optional): EvidenceBlobStore 인터페이스(Put/Get) 유지. database_blob 시 blob bytes는 인터페이스 미통과 → `EvidenceTx.InsertEvidence(file_content)` 동일 TX 경유; 인터페이스는 논리 location(`db://evidences/<id>`)만 기록 (strategy.md §2.6.5 6a).
- U1 (Unwanted): 외부 호스트 네트워크 호출 전략 부적격. 확정 database_blob은 내부망 PG만 사용 → by-construction 충족. 외부 관리형 S3 영구 부적격, self-hosted MinIO는 post-PoC 재검토.

## Acceptance Criteria (19)

- §0 UBI: AC-EVID-UBI-001 (외부 호출 0건), AC-EVID-UBI-002-A (create audit), AC-EVID-UBI-002-B (version audit), AC-EVID-UBI-003 (cli-anonymous byte-identical), AC-EVID-UBI-004 (이전 버전 보존+immutable).
- §1 REQ-EVID-001: AC-EVID-001-1 (happy path atomic), -2 (oversized/empty/누락 거부), -3 (eval_item_id stub, FK 미강제), -4 (동시 재업로드 SELECT FOR UPDATE), -O1-1 (중복 SHA-256 → 비차단 `duplicate_of` 신호, Optional env-gated).
- §2 REQ-EVID-002: AC-EVID-002-1 (재업로드→v2+prev chain), -2 (이전 버전 조회 가능), -3 (3-deep 체인 + 본문 immutable).
- §3 REQ-EVID-003: AC-EVID-003-1 (RecordEvidenceCreated row), -2 (RecordEvidenceVersioned row), -3 (audit fail→양방향 rollback).
- §4 REQ-EVID-004: AC-EVID-004-1 (storage_strategy 열거값 강제), -2 (외부 서비스 부적격/egress 0), -3 (EvidenceBlobStore 추상화 swap 가능).

Total AC: 19 (§0:5, §1:5, §2:3, §3:3, §4:3). 각 modal REQ ≥ 3 AC. REQ-EVID-001-O1 Optional은 AC-EVID-001-O1-1 전용 AC로 coverage 확보 (v0.1.1).

## Files to Modify

| 경로 | Delta |
|------|-------|
| `internal/store/store.go` | [MODIFY] EvidenceStore/EvidenceTx 인터페이스 추가 |
| `internal/store/evidence.go` | [NEW] EvidenceTx pgx 구현 (InsertEvidence/GetEvidenceByID/GetLatestVersionByEvalItem/ListEvidenceByEvalItem) |
| `internal/store/pg_store.go` | [MODIFY] 실 `PgWorkflowStore.pool` 재사용한 evidence TX 진입점 추가 (신규 풀 금지). **`postgres.go`는 死 스텁 비대상** — strategy.md §0 phantom-path 교정 |
| `internal/audit/audit.go` | [MODIFY] ActionEvidenceCreated/ActionEvidenceVersioned 상수 |
| `internal/audit/recorder.go` | [MODIFY] RecordEvidenceCreated/RecordEvidenceVersioned 메서드 (로컬 AuditTx 유지) |
| `internal/audit/clock.go` | [NEW] clock 주입 (테스트 친화, Risk 7) |
| `internal/storage/storage.go` | [NEW] EvidenceBlobStore 인터페이스 (구현체 0개) |
| `cmd/server/evidence_handlers.go` | [NEW] 증빙 생성/버전 엔드포인트 |
| `.moai/db/schema/migrations/0002_evidence_tables.sql` | [NEW] evidences 테이블(+`file_content BYTEA`, storage_strategy DEFAULT 'database_blob') + 인덱스 (멱등 SQL) |
| `.moai/db/schema/initial.sql` | [EXISTING] 미수정 (schema drift 방지) |
| `internal/config/config.go` | [MODIFY] EVIDENCE_STORAGE_STRATEGY/MAX_FILE_BYTES/DUPLICATE_SIGNAL_ENABLED env 추가 (additive) |
| `internal/store/{evidence,pg_store}_test.go`, `internal/audit/recorder_evidence_test.go`, `cmd/server/evidence_handlers_test.go` | [NEW]/[EXISTING] |

## Exclusions (What NOT to Build)

1. 평가항목 taxonomy / `evaluation_items` 테이블 — eval_item_id는 FK 없는 VARCHAR stub. [Deferred: future SPEC-AX-EVAL-ITEM-001, 미생성]
2. 전체 업로드 UI/UX (Console 멀티파트 폼/진행률/미리보기).
3. ~~저장 전략 최종 선택~~ — **RESOLVED: `database_blob`** (Run Phase 1, plan.md §6). filesystem/minio 구현체는 미빌드(추상화·enum만 유지, post-PoC 전환 대비).
4. 인증/인가/증빙 권한 모델 (SPEC-AX-AUTH 계열).
5. 증빙 삭제/보존 정책 (retention/archival/PIPA 파기).
6. 증빙 ↔ 보고서/워크플로우 연계.
7. 증빙 내용 파싱/OCR/RAG 인덱싱 (Python pipelines 책임).
8. 고급 검색/필터링 (단일 evaluation_item_id 버전 정렬 조회만).
9. 마이그레이션 도구 통합 (수동 멱등 SQL only).

## Resolved / Deferred

- RESOLVED (plan.md §6, Run Phase 1): 파일 바이너리 저장 전략 = `database_blob` (PostgreSQL BYTEA). 가중 8.45; 동일 TX 원자성 구조적 보장 + 망분리 외부 의존 0. EvidenceBlobStore 추상화·`storage_strategy` enum 유지 → post-PoC schema 변경 없는 filesystem/minio 전환 가능.
- Config 기본값: `EVIDENCE_STORAGE_STRATEGY=database_blob`, `EVIDENCE_MAX_FILE_BYTES=52428800` (50 MiB), `EVIDENCE_DUPLICATE_SIGNAL_ENABLED=false`.
- Phantom-path 교정: evidence TX 진입점 = `pg_store.go`(실 `PgWorkflowStore.pool`), NOT `postgres.go`(死 스텁).
- Named placeholder: 평가항목 taxonomy → SPEC-AX-EVAL-ITEM-001 (미생성).
