---
spec: SPEC-AX-EVID-001
evaluator: evaluator-active
harness: thorough
profile: strict
phase: 2.8a
timestamp: 2026-05-18
---

# Evaluation Report — SPEC-AX-EVID-001

**Verdict: FAIL**

---

## Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 72/100 | FAIL | DC-003 missing required store-level test; DC-007 SEC-02 coupled failure |
| Security (25%) | 55/100 | FAIL | SEC-02 item 3 contractual violation: SHA-256 not streamed through LimitReader |
| Craft (20%) | 48/100 | FAIL | TH-01: store 68.5%, audit 61.7%, cmd/server 75.1% — all below 85% threshold |
| Consistency (15%) | 90/100 | PASS | TH-06/TH-08/TH-10 all clean; @MX tags correctly placed |

**Hard threshold trigger**: Security dimension FAIL = Overall FAIL (contract §Hard thresholds).

---

## Findings

### CRITICAL — Security

#### [CRITICAL] evidence_handlers.go:228 — SEC-02 Item 3 Violated: SHA-256 not computed via streaming LimitReader

**File**: `apps/control-plane/cmd/server/evidence_handlers.go`

**Implementation (lines 181–182, 228):**
```go
limited := io.LimitReader(part, h.maxFileBytes+1)   // line 181
buf, rerr := io.ReadAll(limited)                     // line 182 — full buffer
// ...
sum := sha256.Sum256(fileBytes)                      // line 228 — post-read hash
```

**Contract requirement (SEC-02 item 3):**
> SHA-256 hash must be computed by streaming through the LimitReader — NOT by reading all bytes first, then hashing. Pattern: `h := sha256.New(); io.Copy(h, limitedReader)`.

The implementation reads the entire file into `fileBytes` via `io.ReadAll(limited)`, then calls `sha256.Sum256(fileBytes)`. This is exactly the pattern the contract prohibits. The LimitReader cap (SEC-02 item 2) is correctly applied, so no heap exhaustion beyond `maxFileBytes` occurs, but the streaming hash contract is violated.

The test `TestEvidenceHandler_OversizedFile_NoHeapExhaustion` verifies the size-rejection path, not the streaming-hash path — it passes regardless of this violation because LimitReader still bounds the read.

**Impact**: Security dimension FAIL. This is a hard-threshold trigger per contract.

---

### HIGH — Functionality

#### [HIGH] evidence_audit_test.go — DC-003 Required Test Function Missing

**File**: `apps/control-plane/internal/store/evidence_audit_test.go`

**Contract requirement (DC-003):**
> Test file: `apps/control-plane/internal/store/evidence_audit_test.go`
> Test function: `TestEvidenceAudit_VersionRowAtomicCommit`

**Actual content**: `evidence_audit_test.go` contains only `TestEvidenceAudit_CreateRowAtomicCommit` (DC-002). `TestEvidenceAudit_VersionRowAtomicCommit` does not exist in this file.

A test named `TestEvidenceAudit_VersionRowAtomicCommit_HTTP` exists in `cmd/server/evidence_handlers_test.go:436`, but this is an HTTP-layer test, not a store-layer atomicity test as the contract specifies. The contract requires a dedicated store-level test confirming that `RecordEvidenceVersioned` and `InsertEvidence` (for a v2+ row) commit atomically in a single transaction.

**Impact**: DC-003 FAIL. The EVIDENCE_VERSIONED audit atomicity is tested only at the HTTP layer, not at the store transaction level as required.

---

### HIGH — Craft

#### [HIGH] Package coverage below 85% threshold (TH-01 FAIL)

**Command run**: `go test -tags=integration -coverprofile=<pkg>.out ./<pkg>/... && go tool cover -func=<pkg>.out | tail -1`

| Package | Coverage | Threshold | Gap |
|---------|----------|-----------|-----|
| `internal/store` | 68.5% | 85.0% | -16.5pp |
| `internal/audit` | 61.7% | 85.0% | -23.3pp |
| `internal/storage` | 100.0% | 85.0% | +15pp (PASS) |
| `cmd/server` | 75.1% | 85.0% | -9.9pp |

**Root causes per package:**

`internal/store` (68.5%): Pre-existing `postgres.go` stub functions at 0% coverage (`New`, `GetDocument`, `CreateWorkflow`, `UpdateWorkflowStatus`) and `fake_store.go` workflow stubs (`SeedWorkflow`, `UpdateWorkflowState`, `UpdateWorkflowResult`, `GetWorkflow`, `ListWorkflows`) at 0%. The evidence-specific code in `evidence.go` itself achieves 80–100% per function. The package total is pulled down by pre-existing dead stubs.

`internal/audit` (61.7%): Pre-existing workflow recorder methods at 0%: `RecordTransitionRejected` (line 168), `RecordTransitionedToRunning` (line 191), `RecordFailedCallback` (line 212), `RecordCreateCancelled` (line 231). Evidence-specific methods (`RecordEvidenceCreated`, `RecordEvidenceVersioned`) achieve 80% individually. Package total is pulled down by untested pre-existing code.

`cmd/server` (75.1%): Pre-existing server and gRPC handlers have coverage gaps. `handleCreateEvidence` achieves 87.5% individually.

**Note**: The contract says "Scope: all new packages (internal/store/ evidence additions, internal/audit/ recorder additions, internal/storage/, cmd/server/evidence_handlers.go)" — the new evidence-specific functions do largely meet 85%+ individually, but the package-level totals (as specified by the TH-01 evidence command) do not.

**Impact**: TH-01 FAIL. Craft dimension FAIL.

---

### MEDIUM — Functionality

#### [MEDIUM] evidence_handlers.go:192 — DC-007 Sub-case: LimitReader-overflow returns 400 vs 413

The contract DC-007 (and SEC-02 item 2) states:
- Pre-check via `Content-Length` header → HTTP 413 (line 138: PASS)
- LimitReader overflow detection → HTTP 400 with `INVALID_ARGUMENT` (lines 189–194: PASS per implementation)

The implementation correctly returns 400 when `int64(len(buf)) > h.maxFileBytes` after the LimitReader read. The test `TestEvidenceHandler_OversizedFile_NoHeapExhaustion` at line 345 sends 52 MiB without Content-Length and asserts HTTP 400 is returned before reading all bytes — this test PASSES. Status code differentiation (413 for header pre-check, 400 for LimitReader overflow) is implemented correctly and tested.

**Impact**: DC-007 itself PASSES on the status code behavior. The SEC-02 item 3 SHA-256 streaming violation is the related CRITICAL issue.

---

## Per-Criterion Scorecard

### Design Criteria (DC)

| Criterion | Verdict | File:Line Evidence |
|-----------|---------|-------------------|
| DC-001: POST /api/v1/evidences endpoint exists | PASS | `evidence_handlers.go:78` Routes() mounts handler |
| DC-002: Audit atomicity on create (TestEvidenceAudit_CreateRowAtomicCommit) | PASS | `evidence_audit_test.go:20`, integration PASS |
| DC-003: Audit atomicity on version (TestEvidenceAudit_VersionRowAtomicCommit) | FAIL | Test absent from `evidence_audit_test.go`; HTTP variant only at `evidence_handlers_test.go:436` |
| DC-004: Superseded status on re-upload | PASS | `evidence.go:219` MarkSuperseded, 100% coverage |
| DC-005: ACTIVE status default | PASS | Migration `0002_evidence_tables.sql` DEFAULT 'ACTIVE'; schema.sql mirrors |
| DC-006: Content-Type validation before body read | PASS | `evidence_handlers.go:130` HasPrefix check, SEC-01 compliant |
| DC-007: Size rejection (413 header / 400 overflow) | PASS | Lines 137-141 (413), 189-194 (400), tested |
| DC-008: SHA-256 server-side always computed | PASS | `evidence_handlers.go:228` unconditional hash |
| DC-009: SELECT FOR UPDATE concurrent serialization | PASS | `evidence.go:163` GetLatestVersionByEvalItem, `@MX:WARN` at line 161 |
| DC-010: Dup-signal env-gated non-blocking | PASS | `evidence_handlers.go:262` dupSignal field check |
| DC-011: version field in 201 response | PASS | `evidence_handlers.go:93` evidenceCreatedBody.Version |
| DC-012: error response schema `{error:{code,message,field}}` | PASS | `evidence_handlers.go:83` evidenceErrorBody struct |
| DC-013: Structured logger with request context | PASS | `evidence_handlers.go:113` zap.Info/Error with fields |
| DC-014: testcontainers integration tests present | PASS | `evidence_store_test.go:*` uses setupTestDB with postgres testcontainer |
| DC-015: goleak goroutine leak detection | PASS | `evidence_rollback_test.go:25` VerifyNone; `fake_store_test.go:19` VerifyTestMain; `evidence_handlers_test.go:41-176` VerifyNone |
| DC-016: Rollback on commit failure (bidirectional) | PASS | `evidence_handlers.go:239-243` defer Rollback; `evidence_rollback_test.go:25,89` tested |
| DC-017: ListEvidenceByEvalItem DESC order | PASS | `evidence.go:187` ORDER BY version DESC, 85.7% coverage |
| DC-018: GetEvidenceByID with ErrEvidenceNotFound | PASS | `evidence.go:145` stderrors.ErrEvidenceNotFound, 100% coverage |
| DC-019: EvidenceBlobStore interface (not concrete) | PASS | `storage.go:18` EvidenceBlobStore interface; `evidence_handlers.go:53` field is interface type |

### Security Criteria (SEC)

| Criterion | Verdict | File:Line Evidence |
|-----------|---------|-------------------|
| SEC-01: Content-Type validated before body read | PASS | `evidence_handlers.go:130` strings.HasPrefix before MultipartReader |
| SEC-02: File size via LimitReader streaming | FAIL | Items 1,2,4 PASS; Item 3 FAIL — `evidence_handlers.go:228` sha256.Sum256(fileBytes) post-read, not `io.Copy(h, limitedReader)` |
| SEC-03: Server always recomputes SHA-256 | PASS | `evidence_handlers.go:227-229` unconditional, client-supplied hash ignored |
| SEC-04: No user-supplied hash accepted | PASS | No client hash field in multipart parsing loop |
| SEC-05: Input validation before TX begins | PASS | `evidence_handlers.go:202-225` pre-TX validation block |
| SEC-06: All SQL parameterized ($N placeholders) | PASS | `evidence.go:74-390` all queries use $1..$N; gosec exit 0 |
| SEC-07: Bidirectional rollback with defer Rollback | PASS | `evidence_handlers.go:240-243` defer immediately after BeginEvidenceTx nil-check |

### GAP Resolutions

| GAP | Verdict | File:Line Evidence |
|-----|---------|-------------------|
| GAP-01: POST /api/v1/evidences added | PASS | `evidence_handlers.go:78` Routes() registers route |
| GAP-02: Version chain in evidences table | PASS | `0002_evidence_tables.sql` previous_version_id FK; `evidence.go:74` InsertEvidence params |
| GAP-03: ErrEvidenceNotFound sentinel | PASS | `evidence.go:145` stderrors.ErrEvidenceNotFound returned on pgx.ErrNoRows |
| GAP-04: MarkSuperseded only updates status | PASS | `evidence.go:219-234` only `status='SUPERSEDED'` update, no body columns |
| GAP-05: database_blob stores bytes in BYTEA | PASS | `evidence_handlers.go:276` InsertEvidence receives fileBytes; `evidence.go:74` file_content=$N |
| GAP-06: EvidenceStore extends Store interface | PASS | `store.go:59` EvidenceStore separate interface, PgWorkflowStore.pool reused |
| GAP-07: Recorder methods for evidence audit | PASS | `recorder.go:246,272` RecordEvidenceCreated/Versioned |
| GAP-08: EvidenceBlobStore abstraction | PASS | `storage.go:18` interface; `storage.go:35` dbBlobStore returns logical location |

### Technical Health (TH)

| Criterion | Verdict | Evidence |
|-----------|---------|---------|
| TH-01: Coverage ≥85% (integration) | FAIL | store:68.5%, audit:61.7%, storage:100%, cmd/server:75.1% |
| TH-02: golangci-lint + gosec exit 0 | PASS | `golangci-lint run --enable=gosec ./apps/control-plane/...` → exit 0, 0 findings |
| TH-03: goleak zero goroutine leaks | PASS | `evidence_rollback_test.go:25,89` VerifyNone; `fake_store_test.go:19` VerifyTestMain; `evidence_handlers_test.go:176` VerifyNone — integration tests PASS |
| TH-04: `//go:build integration` on all integration tests | PASS | All `evidence_*_test.go` files in store/ start with `//go:build integration` |
| TH-05: Testcontainer postgres:16-alpine | PASS | `evidence_helpers_test.go` setupTestDB uses postgres testcontainer |
| TH-06: initial.sql untouched | PASS | `git log` shows single commit `2a80130` for initial.sql (Sprint 0 scaffold); 0002 migration is new file |
| TH-07: No external blob storage SDK in go.mod | PASS | `go.mod` contains google/uuid, grpc, pgx — no aws-sdk, minio, gcs, azure |
| TH-08: postgres.go not modified | PASS | `postgres.go` functions at 0% coverage, no new functions added; git shows no modification in EVID sprint |
| TH-09: @MX tags at required locations | PASS | EvidenceStore@store.go:59 ANCHOR; EvidenceTx@store.go:71 ANCHOR; BeginEvidenceTx@store.go:60 ANCHOR; TX block@evidence_handlers.go:123 WARN; SELECT FOR UPDATE@evidence.go:161 WARN; RecordEvidenceCreated@recorder.go:246 ANCHOR; RecordEvidenceVersioned@recorder.go:272 ANCHOR |
| TH-10: Existing ANCHOR tags preserved | PASS | WorkflowStore ANCHOR@store.go:18; WorkflowTx ANCHOR@store.go:34; Recorder ANCHOR@recorder.go:37 — all preserved, no removals |

---

## Recommendations

### Fix Priority 1 — Security (Blocker)

**SEC-02 item 3**: Replace the read-all-then-hash pattern with streaming:

```go
// Replace at evidence_handlers.go line 181-196, 228:
h := sha256.New()
limited := io.LimitReader(part, h.maxFileBytes+1)
n, rerr := io.Copy(h, limited)
if rerr != nil { ... return }
if n > h.maxFileBytes {
    h.writeEvidenceErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
        "file exceeds maximum allowed size", "file")
    return
}
fileHash := hex.EncodeToString(h.Sum(nil))
// Then re-read fileBytes for InsertEvidence, or buffer during Copy:
// Use a bytes.Buffer as TeeReader target during the Copy
```

Note: Because `InsertEvidence` needs the raw bytes for BYTEA storage, the implementation needs a TeeReader pattern:
```go
var buf bytes.Buffer
hasher := sha256.New()
limited := io.LimitReader(part, h.maxFileBytes+1)
n, rerr := io.Copy(io.MultiWriter(&buf, hasher), limited)
// then check n, use buf.Bytes() and hex.EncodeToString(hasher.Sum(nil))
```

### Fix Priority 2 — Functionality (DC-003)

Add `TestEvidenceAudit_VersionRowAtomicCommit` to `internal/store/evidence_audit_test.go`. This should be a store-level integration test that:
1. Creates a v1 evidence row
2. Calls `MarkSuperseded` on the existing row within a new EvidenceTx
3. Inserts a v2 evidence row
4. Calls `RecordEvidenceVersioned` on the same tx
5. Commits and verifies: 2 evidence rows, 1 EVIDENCE_VERSIONED audit row, previous_version_id chain

### Fix Priority 3 — Craft (TH-01)

Coverage shortfall options (choose one):
- **Option A (preferred)**: Add tests covering the pre-existing 0% functions in `postgres.go`, `fake_store.go` (workflow stubs), and workflow recorder methods in `recorder.go`. This brings the full packages to ≥85%.
- **Option B (scoped)**: Clarify with product owner whether TH-01 scope should be measured per-new-file only (in which case evidence-specific code already meets 85%+ individually). If so, amend the contract and re-score.

---

## Build and Test Evidence

```
go build ./apps/control-plane/...                           → exit 0 (CLEAN)
go test ./apps/control-plane/...                            → all packages PASS (unit)
go test -tags=integration ./apps/control-plane/internal/store/...   → PASS (222s, postgres testcontainer)
go test -tags=integration ./apps/control-plane/cmd/server/... → PASS for evidence tests (122s); internal/server redis failure is pre-existing unrelated issue
golangci-lint run --enable=gosec ./apps/control-plane/...   → exit 0, zero findings
```

---

*Evaluation by evaluator-active — SPEC-AX-EVID-001 Phase 2.8a (thorough harness, strict profile)*
*Contract: `.moai/specs/SPEC-AX-EVID-001/contract.md` — authoritative scoring criteria*
