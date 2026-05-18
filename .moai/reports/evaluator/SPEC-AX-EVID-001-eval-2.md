# SPEC-AX-EVID-001 Evaluation Report — Iteration 2

**Evaluator**: evaluator-active (skeptical)
**Harness**: thorough
**Phase**: 2.8a post-iteration-1 fix re-evaluation
**Date**: 2026-05-18
**Verdict**: PASS

---

## Evaluation Report

**SPEC**: SPEC-AX-EVID-001 (경영평가 증빙 자료 수집/관리)
**Overall Verdict**: PASS

---

### Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 95/100 | PASS | 19/19 DC GREEN; DC-002/DC-003 integration PASS; DC-001 sovereignty PASS |
| Security (25%) | 92/100 | PASS | SEC-02 streaming confirmed by code+test+unit run; gosec exit 0; no external egress |
| Craft (20%) | 90/100 | PASS | TH-01 resolved: evidence-core union 91.4% ≥ 85%; lint exit 0; handleCreateEvidence orchestration-only |
| Consistency (15%) | 93/100 | PASS | Pattern adherence intact; brownfield MX anchors untouched; scope discipline observed |

**Weighted score**: 0.95×0.40 + 0.92×0.25 + 0.90×0.20 + 0.93×0.15 = 0.380 + 0.230 + 0.180 + 0.1395 = **0.930**

---

## TH-01 Ruling (Binding Decision by Evaluator)

**Contract text (contract.md §3)**: "overall coverage >= 85.0%" with scope "all new packages (...evidence additions...)".

**Ruling**: The **evidence-core file-union interpretation is the correct binding reading** of TH-01 for this brownfield SPEC.

**Justification**:

1. **Scope discipline (HARD rule)**: The `internal/store` package contains pre-existing SPEC-AX-CTRL-001 code (`postgres.go` stub, `fake_store.go` workflow stubs, `recorder.go` workflow methods — verified `RecordTransitionRejected`, `RecordTransitionedToRunning`, `RecordFailedCallback`, `RecordCreateCancelled` all at 0.0% — none of which were in SPEC-AX-EVID-001's Planned Files). Retroactively testing that code to satisfy TH-01 violates [HARD] scope discipline.

2. **Contract phrase "evidence additions"**: The SPEC explicitly scopes TH-01 to the evidence additions, not the entire `internal/store` package total. The `go tool cover -func` package total (~62%) is dominated by out-of-scope code; it is not a faithful measurement of this SPEC's deliverables.

3. **Independent measurement confirmed**: I re-measured the 4 evidence-core files (`evidence_handlers.go`, `evidence.go`, `storage.go`, `clock.go`) from a freshly generated `coverage_evid_int.out` (integration run, `-coverpkg` scoped to cmd/server + internal/store + internal/storage + internal/audit packages). Per-file results:
   - `cmd/server/evidence_handlers.go`: 114/128 stmts = **89.1%**
   - `internal/store/evidence.go`: 104/111 stmts = **93.7%**
   - `internal/storage/storage.go`: 5/5 stmts = **100.0%**
   - `internal/audit/clock.go`: 1/1 stmts = **100.0%**
   - **Union (deduplicated blocks): 224/245 = 91.4% ≥ 85%**

**TH-01 verdict: PASS** (91.4% evidence-core union, independently verified).

---

## Per-Criterion Verdicts

### F1 — SEC-02 Single-Pass Streaming Hash

**Verdict: PASS**

Evidence:
- `cmd/server/evidence_handlers.go:134` `parseAndHashMultipart`: confirmed `io.Copy(io.MultiWriter(&buf, hasher), io.LimitReader(r, maxBytes+1))` — single-pass, no ReadAll-then-hash pattern.
- Oversized input: `LimitReader(r, maxBytes+1)` — when `n > maxBytes`, returns `errEvidenceOversized` without materializing the full body.
- Bounded metadata fields: `io.ReadAll(io.LimitReader(part, maxEvalItemIDLen+1))` (65 bytes max) and `io.ReadAll(io.LimitReader(part, maxFileNameLen+1))` (513 bytes max) — both are provably bounded per-contract. Not a SEC-02 violation.
- Unit test `TestParseAndHashMultipart_StreamingSinglePass` (4 sub-tests) — **all PASS** (output: `ok cmd/server 0.007s`). The `hashing_interleaves_with_read_single_pass` sub-test uses a 1-byte reader to confirm MultiWriter interleaving. The `oversized_rejected_without_full_buffering` sub-test asserts `cr.bytesRead <= maxBytes+1`.

```
=== RUN   TestParseAndHashMultipart_StreamingSinglePass
    --- PASS: .../valid_input_streaming_hash_correct (0.00s)
    --- PASS: .../oversized_rejected_without_full_buffering (0.00s)
    --- PASS: .../hashing_interleaves_with_read_single_pass (0.00s)
    --- PASS: .../empty_input_zero_bytes (0.00s)
--- PASS: TestParseAndHashMultipart_StreamingSinglePass (0.00s)
ok  github.com/ircp/iroum-ax/apps/control-plane/cmd/server 0.007s
```

---

### F2 — Blob Location TX-Scope

**Verdict: PASS**

Evidence:
- `cmd/server/evidence_handlers.go:254` `persistEvidenceTx`: `blobStore.Put(ctx, newID.String(), bytes.NewReader(nil))` is called AFTER `BeginEvidenceTx` and BEFORE `InsertEvidence`. The `database_blob` strategy's `dbBlobStore.Put` is side-effect-free (returns a logical `db://evidences/<id>` location); actual bytes flow through `InsertEvidence(file_content)` in the same TX. No evidence TX-external blob persistence path exists.
- Integration tests `TestEvidenceStore_CreateHappyPath_HTTP`, `TestEvidenceAudit_VersionRowAtomicCommit_HTTP` PASS (both commit evidence+blob+audit atomically).

---

### F3+F4 — DC-003 Store-Level Atomicity Test

**Verdict: PASS**

Evidence:
- `internal/store/evidence_audit_test.go:70` `TestEvidenceAudit_VersionRowAtomicCommit` exists, is store-level (`package store`, `//go:build integration`), not an HTTP test.
- Test asserts all DC-003 binary conditions:
  1. Line 118: pre-commit atomicity guard — `countAuditByAction(..., "EVIDENCE_VERSIONED") == 0` before `tx.Commit`.
  2. Line 125: post-commit `EVIDENCE_VERSIONED` count == 1.
  3. Lines 131-139: `details.version=="2"`, `details.previous_version_id==ev1ID`, `user_id=="cli-anonymous"`.
  4. Lines 142-148: `evidences` 2 rows, `audit_logs` 2 rows (1 CREATED + 1 VERSIONED) via separate pool connection.
  5. Lines 152-154 (F4): predecessor `status == "SUPERSEDED"` confirmed via separate pool query.
- Integration run result: **PASS** (5.16s, testcontainers postgres:16-alpine).

```
=== RUN   TestEvidenceAudit_VersionRowAtomicCommit
--- PASS: TestEvidenceAudit_VersionRowAtomicCommit (5.16s)
PASS
```

---

### F5 — handleCreateEvidence Complexity/Length

**Verdict: PASS**

Evidence:
- `handleCreateEvidence` at lines 305–377: 72 total lines (including blank/comment lines), orchestration-only. Logic is delegated to helpers: `parseMultipartForm`, `validateEvidenceParts`, `resolveVersion`, `persistEvidenceTx`.
- No helper function is longer than ~50 logical lines. Cyclomatic complexity of `handleCreateEvidence` is ≤5 (sequential if-return branches, no nested loops).
- All pre-existing and new tests remain GREEN (confirmed by `go test -count=1 ./...` and integration suite).

---

### F6 / TH-01 — Coverage

**Verdict: PASS** (see TH-01 Ruling above)

Independently measured: 91.4% evidence-core union (224/245 statements, integration coverage run).

---

### DC-001 — Data Sovereignty

**Verdict: PASS**

- `go list -deps ./internal/store/... ./internal/storage/... ./cmd/server/...` — zero output matching `s3|minio|gcs|azure|aws`.
- `"crypto/sha256"` present in `cmd/server/evidence_handlers.go`; no `golang.org/x/crypto` imports in evidence files.
- `TestEvidenceSovereignty_NoExternalEgress` (integration): **PASS** (23.47s).

---

### DC-002 — Audit Create Atomicity

**Verdict: PASS**

- `TestEvidenceAudit_CreateRowAtomicCommit` (store integration): **PASS** (6.38s).
- Verifies: evidence 1 row + `EVIDENCE_CREATED` audit 1 row post-commit via separate pool; `details.evaluation_item_id`, `details.version=="1"`, `details.file_hash_sha256` all non-null; `user_id=="cli-anonymous"`.

---

### DC-003 — Audit Version Atomicity (store-level)

**Verdict: PASS** (see F3+F4 above)

---

### Out-of-Scope Pre-Existing Failure: TestE2E_GRPC_Authz_ViewerForbidden_Create

**Verdict: EXCLUDED (confirmed pre-existing, out of scope)**

Evidence:
- `git diff HEAD -- apps/control-plane/internal/server/authz_e2e_test.go` returns 0 lines. Zero changes to this file from the SPEC-AX-EVID-001 changeset.
- Failure is gRPC `CallbackSerializer` goroutine leak + Redis dial timeout (`127.0.0.1:16399`) — SPEC-AX-AUTH-002/SERVER-001 territory.
- All other `internal/server` integration tests pass (29 PASS, 1 FAIL confined to this pre-existing test).
- This failure does NOT affect the SPEC-AX-EVID-001 verdict.

---

### Golangci-lint + gosec

**Exit code: 0, 0 issues**

```
$ golangci-lint run --enable=gosec --timeout=120s ./...
Exit code: 0
```

---

### Build + Vet

```
$ go build ./...   → exit 0
$ go vet ./...     → exit 0
$ go test -count=1 ./...  → all 11 packages PASS, 0 FAIL
```

---

## Findings

- [Info] `cmd/server/evidence_handlers.go:177,180` — `io.ReadAll(io.LimitReader(...))` for `evaluation_item_id` and `file_name` metadata fields use `//nolint:errcheck`. These are bounded (≤65 and ≤513 bytes respectively) and acceptable per the evaluator's SEC-02 ruling stated in the spawn prompt. Not a defect.
- [Info] `internal/audit/recorder.go:168,191,212,231` — `RecordTransitionRejected`, `RecordTransitionedToRunning`, `RecordFailedCallback`, `RecordCreateCancelled` all at 0.0% coverage. These are SPEC-AX-CTRL-001 pre-existing methods, outside TH-01 scope. Confirmed excluded by TH-01 ruling.
- [Info] `internal/server/authz_e2e_test.go:540` `TestE2E_GRPC_Authz_ViewerForbidden_Create` — pre-existing, zero git diff, Redis dial timeout. Out of scope (SPEC-AX-AUTH-002/SERVER-001). Excluded from verdict per instruction and verification.

---

## Recommendations

- The TH-01 ruling (evidence-core union ≥85%) should be formalized in contract.md as an amendment so future evaluators have an unambiguous measurement target. The current text "all new packages" is ambiguous in brownfield contexts where new files coexist with pre-existing untested code in the same package.
- `TestE2E_GRPC_Authz_ViewerForbidden_Create` Redis dial timeout and goroutine leak should be addressed in a dedicated SPEC (SPEC-AX-AUTH-002 or SERVER-001 follow-up) per lessons #11/#12.

---

## Test Run Summary

| Test Suite | Command | Result |
|-----------|---------|--------|
| Unit (all packages) | `go test -count=1 ./...` | 11 packages PASS, 0 FAIL |
| SEC-02 unit | `-run TestParseAndHashMultipart_StreamingSinglePass ./cmd/server/` | 4/4 sub-tests PASS |
| Store integration | `go test -count=1 -tags=integration ./internal/store/...` | PASS (216s) |
| cmd/server integration | `go test -count=1 -tags=integration ./cmd/server/...` | PASS (123s) |
| DC-003 store-level | `-run TestEvidenceAudit_VersionRowAtomicCommit ./internal/store/...` | PASS (5.16s) |
| DC-002 store-level | `-run TestEvidenceAudit_CreateRowAtomicCommit ./internal/store/...` | PASS (6.38s) |
| DC-001 sovereignty | `-run TestEvidenceSovereignty_NoExternalEgress ./internal/store/...` | PASS (23.47s) |
| golangci-lint + gosec | `golangci-lint run --enable=gosec ./...` | exit 0, 0 issues |
| Build + Vet | `go build ./... && go vet ./...` | exit 0 |
