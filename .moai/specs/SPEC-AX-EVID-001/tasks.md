# SPEC-AX-EVID-001 — Task Decomposition (tasks.md)

> SPEC: `.moai/specs/SPEC-AX-EVID-001/spec.md` v0.1.2 | plan.md v0.1.2 | acceptance.md (19 AC)
> Methodology: TDD (RED-GREEN-REFACTOR) — `quality.yaml development_mode: tdd` | Harness: thorough
> Storage strategy: RESOLVED `database_blob` (Run Phase 1, Human Gate approved)
> [DELTA] processing order: [EXISTING] characterization baseline → [MODIFY] characterize→modify→verify → [NEW] full TDD cycle
> Phantom-path: evidence TX entry point = `internal/store/pg_store.go` (real `PgWorkflowStore.pool`). `internal/store/postgres.go` is a Sprint-0 死 stub — NOT a target.

## Task Table

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | [EXISTING] Capture characterization/regression baseline: run full `store` + `audit` + `cmd/server` suites, record GREEN snapshot before any [MODIFY]. Regression guard for all downstream [MODIFY] tasks (R-EVID-005/008 gate). | (regression baseline; no new REQ) | — | `apps/control-plane/internal/store/postgres_test.go` [EXISTING], `apps/control-plane/internal/store/pg_store_ping_test.go` [EXISTING], `apps/control-plane/internal/store/fake_store_test.go` [EXISTING], `apps/control-plane/internal/audit/recorder_test.go` [EXISTING], `apps/control-plane/cmd/server/server_test.go` [EXISTING] | completed |
| T-002 | [NEW] Author idempotent migration `0002_evidence_tables.sql` per plan.md §3 DDL incl. `file_content BYTEA` (nullable), `storage_strategy` DEFAULT `database_blob` + CHECK enum, self-FK `previous_version_id` ON DELETE RESTRICT, 2 indexes. `CREATE TABLE IF NOT EXISTS` + `DO $$ ... EXCEPTION WHEN duplicate_object ...`. `initial.sql` untouched (R-EVID-008). Verified by integration RED in T-005/T-009. | REQ-EVID-001, REQ-EVID-004-S1 | T-001 | `.moai/db/schema/migrations/0002_evidence_tables.sql` [NEW] | completed |
| T-003 | [MODIFY] Add action constants `ActionEvidenceCreated="EVIDENCE_CREATED"`, `ActionEvidenceVersioned="EVIDENCE_VERSIONED"` to existing const block. Additive only, existing symbols unchanged; verify `audit` package compiles + T-001 audit regression GREEN. | REQ-EVID-003 | T-001 | `apps/control-plane/internal/audit/audit.go` [MODIFY] | completed |
| T-004 | [MODIFY] Declare `EvidenceStore` (BeginEvidenceTx) + `EvidenceTx` (InsertEvidence, GetEvidenceByID, GetLatestVersionByEvalItem, ListEvidenceByEvalItem, InsertAuditLog, Commit, Rollback) interfaces mirroring `WorkflowStore`/`WorkflowTx`. Signatures only. Additive; verify `store` package compiles + T-001 store regression GREEN. | REQ-EVID-001 | T-001 | `apps/control-plane/internal/store/store.go` [MODIFY] | completed |
| T-005 | [NEW] TDD: InsertEvidence + GetEvidenceByID (version=1, previous_version_id=NULL, file_content BYTEA round-trip, storage_strategy persisted). RED testcontainers test → GREEN `PgEvidenceTx` pgx impl mirroring `PgWorkflowTx`. | REQ-EVID-001-E1, REQ-EVID-004-S1 | T-002, T-003, T-004 | `apps/control-plane/internal/store/evidence.go` [NEW], `apps/control-plane/internal/store/evidence_test.go` [NEW] | completed |
| T-006 | [MODIFY] Add evidence TX entry point `BeginEvidenceTx(ctx) (EvidenceTx, error)` to real `PgWorkflowStore` reusing `s.pool` (`pgx.TxOptions{IsoLevel: ReadCommitted}`, mirror `BeginTx`). NO new pool (R-EVID-005). NOT `postgres.go`. RED: store-level entry test → GREEN wiring; verify T-001 `PgWorkflowStore` regression GREEN. | REQ-EVID-001-E1 | T-005 | `apps/control-plane/internal/store/pg_store.go` [MODIFY] | completed |
| T-007 | [NEW] TDD: concurrent same-`evaluation_item_id` re-upload serialization. RED 2-goroutine duplicate-version race (+goleak) → GREEN `GetLatestVersionByEvalItem` with `SELECT ... FOR UPDATE`. | REQ-EVID-001-S1 | T-006 | `apps/control-plane/internal/store/evidence.go` [NEW], `apps/control-plane/internal/store/evidence_test.go` [NEW] | completed |
| T-008 | [NEW] TDD: re-upload → new row version=2, previous_version_id=predecessor.id, predecessor status ACTIVE→SUPERSEDED, all in one TX; prior row preserved (no DELETE). RED → GREEN store/version-resolution logic. | REQ-EVID-002-E1, REQ-EVID-002-S1, REQ-EVID-UBI-004 | T-007 | `apps/control-plane/internal/store/evidence.go` [NEW], `apps/control-plane/internal/store/evidence_test.go` [NEW] | completed |
| T-009 | [NEW] TDD: 3-deep chain navigability (v3→v2→v1→NULL via GetEvidenceByID + ListEvidenceByEvalItem DESC) AND store-layer mutation guard rejecting body-column UPDATE when a successor exists (no SQL executed). RED → GREEN immutability guard. | REQ-EVID-002-S1, REQ-EVID-002-U1, REQ-EVID-UBI-004 | T-008 | `apps/control-plane/internal/store/evidence.go` [NEW], `apps/control-plane/internal/store/evidence_test.go` [NEW] | completed |
| T-010 | [MODIFY] Add `RecordEvidenceCreated` + `RecordEvidenceVersioned` to Recorder — exact `RecordCreated(ctx, tx AuditTx, ...)` signature pattern, reuse local `AuditTx` (no `store` import — R-EVID-001), `resolveUserID` (R-EVID-003), DetailsJSON `{evaluation_item_id, version, file_hash_sha256, previous_version_id?}`. RED recorder test → GREEN; verify T-001 recorder regression GREEN. | REQ-EVID-003-E1, REQ-EVID-UBI-002, REQ-EVID-UBI-003 | T-003 | `apps/control-plane/internal/audit/recorder.go` [MODIFY], `apps/control-plane/internal/audit/recorder_evidence_test.go` [NEW] | completed |
| T-011 | [NEW] TDD: audit INSERT fault injection → evidence + audit bidirectional rollback (no partial commit, wrapped error, +goleak). RED → GREEN handler/store TX orchestration `defer tx.Rollback(ctx)` (pg_store.go:287-296 pattern). DB-BLOB makes blob bytes part of same TX → no orphan-cleanup machinery. | REQ-EVID-003-U1, REQ-EVID-UBI-002 | T-010, T-008 | `apps/control-plane/internal/audit/recorder_evidence_test.go` [NEW], `apps/control-plane/internal/store/evidence.go` [NEW] | completed |
| T-012 | [NEW] TDD: `EvidenceBlobStore` interface (`Put(ctx,key,reader)(string,error)`, `Get(ctx,location)(reader,error)`) + PoC `dbBlobStore` returning logical `db://evidences/<id>` (bytes ride EvidenceTx.InsertEvidence, NOT through interface — strategy.md §2.6.5 option 6a); storage_strategy enum validation (CHECK + store-layer); no external SaaS SDK import. RED contract/static-import test → GREEN. | REQ-EVID-004-O1, REQ-EVID-004-S1, REQ-EVID-UBI-001, REQ-EVID-004-U1 | T-005 | `apps/control-plane/internal/storage/storage.go` [NEW], `apps/control-plane/internal/store/evidence.go` [NEW] | completed |
| T-013 | [NEW] TDD: evidence create/version endpoint — multipart → `crypto/sha256` (stdlib) → BeginEvidenceTx → version-resolve → InsertEvidence(+file_content) → RecordEvidence{Created\|Versioned} → Commit → 201 `{evidence_id,version}`. Happy path + p99<150ms (ex-blob) + no-FK stub success. RED httptest → GREEN handler. | REQ-EVID-001-E1, REQ-EVID-002-E1, REQ-EVID-UBI-002, REQ-EVID-UBI-003 | T-006, T-009, T-011, T-012 | `apps/control-plane/cmd/server/evidence_handlers.go` [NEW], `apps/control-plane/cmd/server/evidence_handlers_test.go` [NEW] | completed |
| T-014 | [MODIFY] Add config keys `EVIDENCE_STORAGE_STRATEGY` (getEnv, default `database_blob`, enum-validate fail-fast at Load), `EVIDENCE_MAX_FILE_BYTES` (default `52428800`), `EVIDENCE_DUPLICATE_SIGNAL_ENABLED` (getBoolEnv, default `false`). RED config-load test → GREEN; verify T-001 config-dependent regression GREEN. | REQ-EVID-004-S1, REQ-EVID-001-U1, REQ-EVID-001-O1 | T-001 | `apps/control-plane/internal/config/config.go` [MODIFY] | completed |
| T-015 | [NEW] TDD: pre-TX input validation — oversized (>EVIDENCE_MAX_FILE_BYTES), empty payload, missing/blank evaluation_item_id → HTTP 400 `{error:{code,message,field}}`, no TX opened, no rows, INFO log. RED → GREEN handler guard before BeginEvidenceTx. | REQ-EVID-001-U1 | T-013, T-014 | `apps/control-plane/cmd/server/evidence_handlers.go` [NEW], `apps/control-plane/cmd/server/evidence_handlers_test.go` [NEW] | completed |
| T-016 | [NEW] TDD: env-gated duplicate SHA-256 non-blocking `duplicate_of` signal — active (`EVIDENCE_DUPLICATE_SIGNAL_ENABLED=true`) surfaces signal + still HTTP 201 version=2; inactive (default) omits field. RED → GREEN handler optional signal. | REQ-EVID-001-O1 | T-013, T-014 | `apps/control-plane/cmd/server/evidence_handlers.go` [NEW], `apps/control-plane/cmd/server/evidence_handlers_test.go` [NEW] | completed |
| T-017 | [NEW] TDD: data-sovereignty enforcement — external host TCP/DNS 0 on store/hash/retrieve path; `internal/storage` imports no external SaaS SDK; 0 concrete external EvidenceBlobStore impls. RED network-spy + static-import test → GREEN (assert-only; behavior already satisfied by stdlib sha256 + DB-BLOB). | REQ-EVID-UBI-001, REQ-EVID-004-U1 | T-013, T-012 | `apps/control-plane/cmd/server/evidence_handlers_test.go` [NEW], `apps/control-plane/internal/store/evidence_test.go` [NEW] | completed |
| T-018 | [REFACTOR] Introduce clock-injection abstraction replacing direct `time.Now().UTC()` in evidence audit path (test-friendliness, R-EVID-007). [NEW] `clock.go`; recorder evidence methods use injected clock. Tests stay GREEN (refactor, no behavior change). | REQ-EVID-003-E1 (test-quality refactor) | T-010, T-011 | `apps/control-plane/internal/audit/clock.go` [NEW], `apps/control-plane/internal/audit/recorder.go` [MODIFY], `apps/control-plane/internal/audit/recorder_evidence_test.go` [NEW] | completed |
| T-019 | [QUALITY GATE] Coverage ≥85%, golangci-lint default+gosec 0 issue, `goleak.VerifyNone(t)` all tests, @MX tags per plan.md §5 (RED `@MX:TODO` all resolved; ANCHOR/WARN/NOTE finalized; Korean tag text per `code_comments: ko`), T-001 characterization 0 regression, TRUST 5, evaluator-active strict ≥0.75. Pre-submission self-review (Drift Guard + simplicity gate). | All REQ-EVID-* (aggregate DoD) | T-013, T-015, T-016, T-017, T-018, T-009 | (no production file — gate task; touches `.moai/specs/SPEC-AX-EVID-001/progress.md`) | completed |

## AC → Task Mapping (19 AC)

| AC | Task(s) | REQ |
|----|---------|-----|
| AC-EVID-UBI-001 | T-017 | REQ-EVID-UBI-001 |
| AC-EVID-UBI-002-A | T-010, T-013 | REQ-EVID-UBI-002 |
| AC-EVID-UBI-002-B | T-010, T-013 | REQ-EVID-UBI-002 |
| AC-EVID-UBI-003 | T-010, T-013, T-014 | REQ-EVID-UBI-003 |
| AC-EVID-UBI-004 | T-008, T-009 | REQ-EVID-UBI-004 |
| AC-EVID-001-1 | T-005, T-013 | REQ-EVID-001-E1 |
| AC-EVID-001-2 | T-015 | REQ-EVID-001-U1 |
| AC-EVID-001-3 | T-002, T-013 | REQ-EVID-001 (no-FK stub) |
| AC-EVID-001-4 | T-007 | REQ-EVID-001-S1 |
| AC-EVID-001-O1-1 | T-016 | REQ-EVID-001-O1 |
| AC-EVID-002-1 | T-008 | REQ-EVID-002-E1 |
| AC-EVID-002-2 | T-008, T-009 | REQ-EVID-002-S1 |
| AC-EVID-002-3 | T-009 | REQ-EVID-002-U1, REQ-EVID-UBI-004 |
| AC-EVID-003-1 | T-010 | REQ-EVID-003-E1 |
| AC-EVID-003-2 | T-010 | REQ-EVID-003-E1 |
| AC-EVID-003-3 | T-011 | REQ-EVID-003-U1 |
| AC-EVID-004-1 | T-012 | REQ-EVID-004-S1 |
| AC-EVID-004-2 | T-017 | REQ-EVID-004-U1, REQ-EVID-UBI-001 |
| AC-EVID-004-3 | T-012 | REQ-EVID-004-O1 |

## Requirement Coverage Verification (coverage_verified: true)

| REQ-ID | Covered by | Status |
|--------|-----------|--------|
| REQ-EVID-UBI-001 | T-012, T-017 | covered |
| REQ-EVID-UBI-002 | T-010, T-011, T-013 | covered |
| REQ-EVID-UBI-003 | T-010, T-013, T-014 | covered |
| REQ-EVID-UBI-004 | T-008, T-009 | covered |
| REQ-EVID-001 (E1/S1/U1/O1) | T-002, T-005, T-006, T-007, T-013, T-014, T-015, T-016 | covered |
| REQ-EVID-002 (E1/S1/U1) | T-008, T-009 | covered |
| REQ-EVID-003 (E1/U1) | T-003, T-010, T-011, T-018 | covered |
| REQ-EVID-004 (S1/O1/U1) | T-002, T-012, T-014, T-017 | covered |

**coverage_verified: true** — all 4 REQ-EVID-UBI + all 4 modal REQ-EVID-001..004 (incl. every E/S/U/O sub-clause) have ≥1 task; all 19 AC mapped to ≥1 task.

## Sprint → Task Mapping (plan.md §2)

| Sprint | Tasks |
|--------|-------|
| Sprint 0 — Foundation | T-001, T-002, T-003, T-004 |
| Sprint 1 — Evidence Store | T-005, T-006, T-007 |
| Sprint 2 — Versioning | T-008, T-009 |
| Sprint 3 — Audit Integration | T-010, T-011, T-018 |
| Sprint 4 — Storage + Endpoint + Config | T-012, T-013, T-014, T-015, T-016, T-017 |
| Sprint 5 — Quality Gate | T-019 |

## Execution Notes
- Sub-agent mode, sequential per dependency chain (tightly coupled same-package edits; not team mode).
- TDD: within each [NEW]/[MODIFY] task the test is RED-authored before production code; [EXISTING] T-001 is characterization-only.
- Drift Guard reads the Planned Files column; deviations >30% cumulative → Re-planning Gate.
- `internal/store/postgres.go` is intentionally absent from all Planned Files (Sprint-0 死 stub, phantom-path corrected).

## Targeted Fix Cycle — Iteration 1 (2026-05-18)

Post-implementation 품질 게이트(manager-quality Phase 2.5 CRITICAL + evaluator-active Phase 2.8a Security must-pass FAIL) 해소. T-013/T-015(F1,F5), T-008(F3/F4 store-level), T-013(F2) 보강:

| 결함 | Task 연관 | 산출물 | Status |
|------|-----------|--------|--------|
| F1 SEC-02 단일 패스 스트리밍 SHA-256 | T-013, T-015 | `cmd/server/evidence_handlers.go` `parseAndHashMultipart`, `cmd/server/evidence_streaming_test.go` [NEW unit] | completed |
| F2 blob location TX-스코프 정합 | T-013 | `cmd/server/evidence_handlers.go` `persistEvidenceTx` | completed |
| F3+F4 DC-003 store-level 원자성 테스트 | T-008, T-010 | `internal/store/evidence_audit_test.go` `TestEvidenceAudit_VersionRowAtomicCommit` [NEW] | completed |
| F5 handleCreateEvidence 복잡도/길이 (TRUST 5 Readable) | T-013 | `cmd/server/evidence_handlers.go` 헬퍼 분해 (행위 불변) | completed |
| F6 evidence-core 커버리지 91.4% (≥85%) + 계약 명료화 권고 | T-019 | (측정 — 계약 미수정, progress.md 권고 기록) | completed |

- 신규 단위 테스트 파일 `cmd/server/evidence_streaming_test.go` (DB불요, SEC-02 단일 패스 binary spec) — Planned Files 추가 (T-013/T-015 보강, drift 의도적·승인됨).
- F6: out-of-scope 사전 미테스트(postgres.go/fake_store.go/recorder.go workflow 메서드, SPEC-AX-CTRL-001)에 테스트 미작성 — scope discipline [HARD] 준수.
