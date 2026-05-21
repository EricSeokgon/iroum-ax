# SPEC-AX-EVID-001 Sprint Contract

> **Produced by**: evaluator-active (Phase 2.0 — Pre-implementation)
> **Harness**: thorough
> **Methodology**: TDD RED-GREEN-REFACTOR
> **Date**: 2026-05-18
> **Status**: BINDING — implementation will be scored against this contract at Phase 2.8a

---

## §1. Done Criteria (19 AC → Binary-Checkable Tests)

Each criterion names the expected test file, test function pattern, and the specific assertion that makes it objectively verifiable (PASS/FAIL, not "works correctly").

### §1.0 REQ-EVID-UBI Ubiquitous Invariants

---

#### DC-001 — AC-EVID-UBI-001 (데이터 주권: 저장/해시/조회 외부 egress 0건)

**T-IDs**: T-001 (DDL), T-005 (interface), T-010 (storage), T-018 (integration test)

**Test file**: `apps/control-plane/internal/store/evidence_sovereignty_test.go`
**Test function**: `TestEvidenceSovereignty_NoExternalEgress`

**Assertion** (binary):
1. `go list -deps ./internal/store/... ./internal/storage/... ./cmd/server/...` output must NOT contain any package matching `s3`, `minio`, `gcs`, `azure`, or any package with a domain name (e.g., `github.com/aws/`, `github.com/minio/`).
2. Test runs with a custom `http.Transport` that records all outbound dial attempts. After calling the evidence create endpoint (loopback testcontainer), `dialLog` length == 0 for any non-loopback host.
3. `crypto/sha256` stdlib confirmed via `grep -r '"crypto/sha256"' internal/store/` returning at least one match AND `grep -r '"golang.org/x/crypto"' internal/store/` returning no matches.

**Concrete FAIL conditions**: Any resolved import containing `aws-sdk`, `minio-go`, external hash library; any TCP dial to non-loopback host during evidence operation.

---

#### DC-002 — AC-EVID-UBI-002-A (감사 완결성: 증빙 생성 TX에 audit_logs 1건)

**T-IDs**: T-007 (RecordEvidenceCreated), T-012 (handler), T-018 (integration)

**Test file**: `apps/control-plane/internal/store/evidence_audit_test.go`
**Test function**: `TestEvidenceAudit_CreateRowAtomicCommit`

**Assertion** (binary):
1. After calling evidence create handler: `SELECT COUNT(*) FROM audit_logs WHERE action='EVIDENCE_CREATED' AND resource_id=$evidenceID` == 1.
2. `SELECT COUNT(*) FROM evidences` == 1 (both rows exist — not a partial commit).
3. `SELECT details->>'evaluation_item_id', details->>'version', details->>'file_hash_sha256' FROM audit_logs WHERE action='EVIDENCE_CREATED'` returns `{"eval-ubi-002a", "1", <64-char hex>}` — all three fields non-null.
4. Verified by issuing SELECT AFTER COMMIT (not inside same TX) using a second pgxpool connection.

---

#### DC-003 — AC-EVID-UBI-002-B (감사 완결성: 버전 이벤트에 EVIDENCE_VERSIONED 1건)

**T-IDs**: T-008 (versioning), T-009 (RecordEvidenceVersioned), T-018

**Test file**: `apps/control-plane/internal/store/evidence_audit_test.go`
**Test function**: `TestEvidenceAudit_VersionRowAtomicCommit`

**Assertion** (binary):
1. After re-upload creating version=2: `SELECT COUNT(*) FROM audit_logs WHERE action='EVIDENCE_VERSIONED' AND resource_id=$newEvidenceID` == 1.
2. `details->>'version'` == `"2"`, `details->>'previous_version_id'` == `$ev1ID`.
3. `user_id = 'cli-anonymous'` (exact literal).
4. Both `evidences` (2 rows) and `audit_logs` (2 rows: 1 CREATED + 1 VERSIONED) exist after commit — verified by separate connection COUNT queries.

---

#### DC-004 — AC-EVID-UBI-003 (cli-anonymous 기본값)

**T-IDs**: T-007 (recorder), T-012 (handler)

**Test file**: `apps/control-plane/internal/audit/recorder_evidence_test.go`
**Test function**: `TestRecorder_EvidenceDefaultUserID`

**Assertion** (binary):
1. `SELECT created_by FROM evidences WHERE id=$evidenceID` == `'cli-anonymous'` (exact, no trailing space, no NULL).
2. `SELECT user_id FROM audit_logs WHERE resource_id=$evidenceID` == `'cli-anonymous'`.
3. `assert.Equal(t, evidenceRow.CreatedBy, auditRow.UserID)` — byte-identical cross-table.
4. Test environment: `AUTH_ENABLED=false`, no Authorization header in request.

---

#### DC-005 — AC-EVID-UBI-004 (버전 불변: 이전 버전 byte-identical 보존)

**T-IDs**: T-008 (versioning store), T-011 (mutation guard)

**Test file**: `apps/control-plane/internal/store/evidence_immutability_test.go`
**Test function**: `TestEvidence_PriorVersionImmutable`

**Assertion** (binary):
1. After creating v2, call `GetEvidenceByID(ev1ID)`: returns non-error, non-nil.
2. Fields compared byte-identical to pre-v2 snapshot: `FileName`, `FileSizeBytes`, `FileHashSHA256`, `ContentType`, `Metadata` — all `assert.Equal`.
3. `ev1.Status == "SUPERSEDED"` (status transition allowed).
4. `SELECT COUNT(*) FROM evidences WHERE id=$ev1ID` == 1 (DELETE not issued).
5. `ev2.PreviousVersionID == ev1.ID`.

---

### §1.1 REQ-EVID-001 증빙 데이터 모델 & Store

---

#### DC-006 — AC-EVID-001-1 (해피 패스: 증빙 생성 + 감사 원자적 커밋)

**T-IDs**: T-004 (EvidenceStore interface), T-005 (EvidenceTx interface), T-006 (DDL migration), T-012 (handler), T-018

**Test file**: `apps/control-plane/internal/store/evidence_store_test.go`
**Test function**: `TestEvidenceStore_CreateHappyPath`

**Assertion** (binary):
1. HTTP response status == 201.
2. Response body JSON: `evidence_id` is a valid UUID (uuid.Parse returns nil error), `version` == 1.
3. `SELECT version, previous_version_id, file_hash_sha256, storage_strategy FROM evidences WHERE id=$evidenceID`: `version=1`, `previous_version_id IS NULL`, `file_hash_sha256` == sha256(testFileBytes) encoded as 64-char lowercase hex, `storage_strategy IN ('filesystem','database_blob','minio')`.
4. `SELECT COUNT(*) FROM audit_logs WHERE action='EVIDENCE_CREATED' AND resource_id=$evidenceID` == 1.
5. Response latency (measured via `time.Now()` before/after httptest round-trip, excluding physical storage) < 150ms for 10 consecutive iterations — `assert.Less(t, maxLatency, 150*time.Millisecond)`.

**BLOCKER noted (T-013 gap)**: HTTP method and route path are not specified in tasks.md. Contract mandates: `POST /api/v1/evidences` (plural, REST-standard). Implementation must register this exact route. Evaluator will fail DC-006 if the registered path differs.

---

#### DC-007 — AC-EVID-001-2 (Edge: Oversized / Empty / 누락 필드 거부)

**T-IDs**: T-015 (handler validation), T-013 (handler setup)

**Test file**: `apps/control-plane/internal/store/evidence_handler_validation_test.go`
**Test function**: `TestEvidenceHandler_InputValidation`

Sub-cases (all must pass):

| Sub-case | Input | Expected HTTP | Expected body field |
|----------|-------|--------------|---------------------|
| (a) Oversized | 51 MiB body (`EVIDENCE_MAX_FILE_BYTES=52428800`) | 400 | `error.code="INVALID_ARGUMENT"`, `error.field="file"` |
| (b) Empty | 0-byte file body | 400 | `error.code="INVALID_ARGUMENT"`, `error.field="file"` |
| (c) Missing eval_item_id | no `evaluation_item_id` form field | 400 | `error.code="INVALID_ARGUMENT"`, `error.field="evaluation_item_id"` |

**Assertion** (binary):
1. All three sub-cases return HTTP 400.
2. `SELECT COUNT(*) FROM evidences` == 0 after each sub-case (transaction not entered — validated pre-TX).
3. `SELECT COUNT(*) FROM audit_logs` == 0.
4. Server log entry level for each rejection == INFO (not ERROR). Verified by injecting a test logger and asserting `logLevel <= zap.InfoLevel`.

**Additional contract constraint** (tasks.md gap DC-007-G1): Handler must pre-validate `evaluation_item_id` length ≤ 64 chars → HTTP 400 with `field="evaluation_item_id"`. If >64 chars, DB constraint error (ERROR 22001) must never be the first response — handler validation fires first.

**Additional contract constraint** (tasks.md gap DC-007-G2): Handler must pre-validate `file_name` length ≤ 512 chars → HTTP 400 with `field="file_name"`. Same rationale.

---

#### DC-008 — AC-EVID-001-3 (Edge: eval_item_id Stub — FK 미강제)

**T-IDs**: T-006 (DDL), T-018

**Test file**: `apps/control-plane/internal/store/evidence_store_test.go`
**Test function**: `TestEvidenceStore_EvalItemStubNoFKConstraint`

**Assertion** (binary):
1. `evaluation_item_id="nonexistent-item-xyz-999"` → HTTP 201 (no FK violation error).
2. `SELECT evaluation_item_id FROM evidences WHERE id=$evidenceID` == `'nonexistent-item-xyz-999'` (exact).
3. Information schema query: `SELECT COUNT(*) FROM information_schema.referential_constraints WHERE constraint_name LIKE '%evidences%' AND unique_constraint_name LIKE '%evaluation_items%'` == 0.

---

#### DC-009 — AC-EVID-001-4 (Concurrent Versioning: SELECT FOR UPDATE 직렬화)

**T-IDs**: T-008 (versioning), T-005 (GetLatestVersionByEvalItem), T-018

**Test file**: `apps/control-plane/internal/store/evidence_concurrent_test.go`
**Test function**: `TestEvidenceStore_ConcurrentReupload_SerializedBySelectForUpdate`

**Assertion** (binary):
1. Setup: v1 exists for `eval-001-4`.
2. Synchronization: both G1 and G2 goroutines are started simultaneously using `sync.WaitGroup` + a `startCh chan struct{}` that is closed only after both goroutines confirm they have begun. This prevents timing-based flakiness.
3. `SELECT MAX(version) FROM evidences WHERE evaluation_item_id='eval-001-4'` == 3.
4. `SELECT COUNT(DISTINCT version) FROM evidences WHERE evaluation_item_id='eval-001-4'` == 3 (versions 1, 2, 3 — no gap, no duplicate).
5. Version chain: `SELECT COUNT(*) FROM evidences WHERE evaluation_item_id='eval-001-4' AND previous_version_id IS NOT NULL` == 2 (v2 and v3 each point to a predecessor).
6. `goleak.VerifyNone(t)` at test end.
7. Neither goroutine returns a deadlock error (`pgconn.PgError` with code `40P01` must not appear).

---

#### DC-010 — AC-EVID-001-O1-1 (Optional: Duplicate SHA-256 → non-blocking `duplicate_of` 신호)

**T-IDs**: T-016 (duplicate signal), T-013 (handler)

**Test file**: `apps/control-plane/internal/store/evidence_duplicate_test.go`
**Test function**: `TestEvidenceHandler_DuplicateSignal_ActiveMode` and `TestEvidenceHandler_DuplicateSignal_InactiveMode`

**Assertion — ACTIVE mode** (`EVIDENCE_DUPLICATE_SIGNAL_ENABLED=true`):
1. HTTP 201 (not rejected).
2. `SELECT COUNT(*) FROM evidences WHERE evaluation_item_id='eval-001-o1'` == 2 (v2 created normally).
3. `response.duplicate_of` == `$ev1ID` (exact UUID string match).

**Assertion — INACTIVE mode** (`EVIDENCE_DUPLICATE_SIGNAL_ENABLED=false` or unset):
1. HTTP 201.
2. `SELECT COUNT(*) FROM evidences` == 2.
3. `jsonBody` does NOT contain key `"duplicate_of"` — `assert.False(t, gjson.Get(body, "duplicate_of").Exists())`.

---

### §1.2 REQ-EVID-002 버전 관리

---

#### DC-011 — AC-EVID-002-1 (Re-upload → version=2 + previous_version_id 체이닝)

**T-IDs**: T-008 (versioning store)

**Test file**: `apps/control-plane/internal/store/evidence_version_test.go`
**Test function**: `TestEvidenceVersion_ReuploadCreatesV2`

**Assertion** (binary):
1. After re-upload: `SELECT version, previous_version_id, status FROM evidences WHERE id=$ev2ID` returns `{2, $ev1ID, 'ACTIVE'}`.
2. `SELECT status FROM evidences WHERE id=$ev1ID` == `'SUPERSEDED'`.
3. `SELECT COUNT(*) FROM evidences WHERE evaluation_item_id='eval-002-1'` == 2.
4. ev1 body columns (file_name, file_size_bytes, file_hash_sha256) unchanged — SELECT and compare against pre-reupload snapshot.

**Contract constraint** (tasks.md gap): The SUPERSEDED status UPDATE must be issued by `EvidenceTx` (store layer), not by the handler. Evaluator will check that no raw SQL UPDATE appears in handler code.

---

#### DC-012 — AC-EVID-002-2 (Prior Version Retrievable)

**T-IDs**: T-005 (GetEvidenceByID, ListEvidenceByEvalItem)

**Test file**: `apps/control-plane/internal/store/evidence_version_test.go`
**Test function**: `TestEvidenceVersion_PriorVersionRetrievable`

**Assertion** (binary):
1. `GetEvidenceByID(ev1ID)` returns non-nil, non-error.
2. `ListEvidenceByEvalItem("eval-002-2")` returns slice of length 2, first element has `version=2`, second has `version=1` (version DESC order).
3. `SELECT COUNT(*) FROM evidences WHERE evaluation_item_id='eval-002-2'` == 2.

**Contract constraint** (tasks.md gap): `GetEvidenceByID` must return a typed sentinel error (e.g., `ErrEvidenceNotFound`) when the ID does not exist — not a raw `pgx.ErrNoRows`. At minimum one test case `TestEvidenceStore_GetByID_NotFound` must assert this sentinel.

---

#### DC-013 — AC-EVID-002-3 (3-Deep Version Chain 보존 + 이전 버전 불변)

**T-IDs**: T-008 (versioning), T-011 (mutation guard)

**Test file**: `apps/control-plane/internal/store/evidence_version_test.go`
**Test function**: `TestEvidenceVersion_ThreeDeepChainIntact`

**Assertion** (binary):
1. `ev3.PreviousVersionID == ev2.ID`, `ev2.PreviousVersionID == ev1.ID`, `ev1.PreviousVersionID == nil` — full chain traversal via three `GetEvidenceByID` calls.
2. All three rows returned by `GetEvidenceByID` are non-nil.
3. Mutation guard: Calling `UpdateEvidenceBodyColumn(ev1ID, "file_hash_sha256", "newvalue")` (or equivalent store method) returns a non-nil error — the store layer refuses the operation.
4. `SELECT file_hash_sha256 FROM evidences WHERE id=$ev1ID` == original `H1` (unchanged).

---

### §1.3 REQ-EVID-003 감사 연계

---

#### DC-014 — AC-EVID-003-1 (RecordEvidenceCreated 감사 Row)

**T-IDs**: T-007 (RecordEvidenceCreated)

**Test file**: `apps/control-plane/internal/audit/recorder_evidence_test.go`
**Test function**: `TestRecorder_RecordEvidenceCreated`

**Assertion** (binary):
1. After `Recorder.RecordEvidenceCreated(ctx, auditTx, evidenceID, evalItemID, hash, version=1, userID="")`:
   - `SELECT action, resource_type, resource_id, user_id FROM audit_logs WHERE resource_id=$evidenceID` == `{'EVIDENCE_CREATED', 'evidence', $evidenceID, 'cli-anonymous'}`.
2. `details` JSONB: `details->>'evaluation_item_id'` == evalItemID, `details->>'version'` == `"1"`, `details->>'file_hash_sha256'` == 64-char hex.
3. `Timestamp NOT NULL` — `SELECT created_at FROM audit_logs WHERE resource_id=$evidenceID` IS NOT NULL.
4. Circular dependency check: `go list -f '{{.Imports}}' ./internal/audit/` output does NOT contain `internal/store` — verified by `assert.NotContains`.

---

#### DC-015 — AC-EVID-003-2 (RecordEvidenceVersioned 감사 Row)

**T-IDs**: T-009 (RecordEvidenceVersioned)

**Test file**: `apps/control-plane/internal/audit/recorder_evidence_test.go`
**Test function**: `TestRecorder_RecordEvidenceVersioned`

**Assertion** (binary):
1. `SELECT action, resource_id, user_id FROM audit_logs WHERE resource_id=$newEvidenceID` == `{'EVIDENCE_VERSIONED', $newEvidenceID, 'cli-anonymous'}`.
2. `details->>'version'` == `"2"`, `details->>'previous_version_id'` == `$ev1ID`.
3. `details->>'evaluation_item_id'` non-null and matches test input.

---

#### DC-016 — AC-EVID-003-3 (Audit Fail → 양방향 Rollback, goroutine leak 없음)

**T-IDs**: T-012 (handler TX flow), T-018

**Test file**: `apps/control-plane/internal/store/evidence_rollback_test.go`
**Test function**: `TestEvidenceStore_AuditFailRollbackBidirectional`

**Assertion** (binary):
1. Fault injection: before test, add a CHECK constraint to `audit_logs` that will reject the insert: `ALTER TABLE audit_logs ADD CONSTRAINT ci_fail CHECK (false)`. Call evidence create. Remove constraint after.
2. `SELECT COUNT(*) FROM evidences` == 0 (evidence row rolled back).
3. `SELECT COUNT(*) FROM audit_logs` == 0 (audit row never inserted).
4. Handler returns non-nil error (wrapped audit error).
5. `goleak.VerifyNone(t)` — called at the start of the test with `goleak.VerifyTestMain` or `defer goleak.VerifyNone(t)` pattern. No goroutine leak from deferred `tx.Rollback`.

**Contract constraint on implementation**: `defer evidenceTx.Rollback(ctx)` MUST be registered immediately after `BeginEvidenceTx` returns successfully — before any `InsertEvidence` or `RecordEvidenceCreated` call. Any implementation that sets up rollback after the first operation is a contract violation.

---

### §1.4 REQ-EVID-004 저장 전략 추상화

---

#### DC-017 — AC-EVID-004-1 (storage_strategy 열거값 강제)

**T-IDs**: T-006 (DDL migration), T-014 (config), T-018

**Test file**: `apps/control-plane/internal/store/evidence_storage_strategy_test.go`
**Test function**: `TestEvidenceStorage_StrategyEnumEnforced`

**Assertion** (binary):
1. `EVIDENCE_STORAGE_STRATEGY='filesystem'` → `SELECT storage_strategy FROM evidences WHERE id=$evidenceID` == `'filesystem'`.
2. Direct SQL: `INSERT INTO evidences (..., storage_strategy) VALUES (..., 'external_s3')` returns pgx error containing `check_constraint_violation` (pq error code 23514).
3. `INSERT INTO evidences (..., storage_strategy) VALUES (..., NULL)` returns pgx error (NOT NULL violation or check constraint).
4. `storage_location` format for `database_blob` strategy: `SELECT storage_location FROM evidences WHERE storage_strategy='database_blob' AND id=$evidenceID` matches regex `^db://evidences/[0-9a-f-]{36}$` — verified by `regexp.MustCompile`.

**Contract constraint** (tasks.md gap): `EVIDENCE_STORAGE_STRATEGY` env var set to an invalid value (e.g., `"s3_external"`) must cause `LoadConfig()` to return a non-nil error at startup — NOT a runtime panic. Test: `TestConfig_InvalidStorageStrategyRejectsAtLoad`.

---

#### DC-018 — AC-EVID-004-2 (외부 서비스 호출 부적격)

**T-IDs**: T-010 (storage interface), T-019 (coverage check)

**Test file**: `apps/control-plane/internal/storage/evidence_sovereignty_static_test.go`
**Test function**: `TestEvidenceStorage_NoExternalSDKImported`

**Assertion** (binary):
1. `go list -deps ./internal/storage/...` output does NOT contain any of: `github.com/aws/`, `github.com/minio/`, `cloud.google.com/`, `github.com/Azure/`. Asserted via `strings.Contains` loop.
2. `find ./internal/storage/ -name '*.go' | xargs grep -l 'EvidenceBlobStore' | wc -l` == 1 (`storage.go` only — interface definition file) — no concrete implementations.
3. Functional test with loopback-only network: evidence create returns HTTP 201 with no non-loopback TCP connection recorded.

---

#### DC-019 — AC-EVID-004-3 (EvidenceBlobStore 추상화 — 전략 교체 가능)

**T-IDs**: T-010 (storage interface)

**Test file**: `apps/control-plane/internal/storage/storage_interface_test.go`
**Test function**: `TestEvidenceBlobStore_InterfaceCompiles`

**Assertion** (binary):
1. Compile-time: `internal/storage/storage.go` defines `type EvidenceBlobStore interface { Put(ctx context.Context, key string, r io.Reader) (string, error); Get(ctx context.Context, location string) (io.ReadCloser, error) }` — exact signatures verified by `reflect`-based test or compilation of a mock implementation.
2. Handler struct contains a field of type `EvidenceBlobStore` (interface), not any concrete struct. Verified by reading handler source via `go/ast` parse or grep: `grep 'EvidenceBlobStore' cmd/server/evidence_handlers.go` returns at least one match AND `grep 'dbBlobStore\|fileBlobStore' cmd/server/evidence_handlers.go` returns no matches.
3. `internal/storage/storage.go` compiles cleanly with `go build ./internal/storage/...` exit code 0.

---

## §2. Edge Cases — Mandatory Coverage Beyond Happy Path

The following 10 edge cases must be covered by dedicated test functions (not just implicitly exercised). The contract names the exact test that must exist.

| # | Edge Case | Mandatory Test Function | DC Ref |
|---|-----------|------------------------|--------|
| E-01 | 재업로드 버전 체이닝 1→2→3 (previous_version_id 단절 없음) | `TestEvidenceVersion_ThreeDeepChainIntact` | DC-013 |
| E-02 | 동시 재업로드 — SELECT FOR UPDATE 직렬화 (barrier-synchronized, not timing-based) | `TestEvidenceStore_ConcurrentReupload_SerializedBySelectForUpdate` | DC-009 |
| E-03 | audit INSERT 실패 → evidence+audit 양방향 롤백, goroutine leak 0 | `TestEvidenceStore_AuditFailRollbackBidirectional` | DC-016 |
| E-04 | eval_item_id FK 미강제 (임의 문자열 허용) | `TestEvidenceStore_EvalItemStubNoFKConstraint` | DC-008 |
| E-05 | oversized / empty / 누락 evaluation_item_id 핸들러 pre-TX 거부 | `TestEvidenceHandler_InputValidation` | DC-007 |
| E-06 | 중복 SHA-256 — env-gated `duplicate_of` 신호 (활성/비활성 양방향 검증) | `TestEvidenceHandler_DuplicateSignal_ActiveMode` + `TestEvidenceHandler_DuplicateSignal_InactiveMode` | DC-010 |
| E-07 | 이전 버전 본문 UPDATE 시도 → store mutation guard 거부 | `TestEvidence_PriorVersionImmutable` (mutation guard branch) | DC-013 |
| E-08 | storage_strategy 열거 외 값 / NULL → CHECK 제약 위반 거부 | `TestEvidenceStorage_StrategyEnumEnforced` | DC-017 |
| E-09 | EVIDENCE_STORAGE_STRATEGY 잘못된 값 → LoadConfig 반환 에러 (startup fail-fast) | `TestConfig_InvalidStorageStrategyRejectsAtLoad` | DC-017 (gap) |
| E-10 | GetEvidenceByID 존재하지 않는 ID → typed sentinel ErrEvidenceNotFound 반환 | `TestEvidenceStore_GetByID_NotFound` | DC-012 (gap) |

---

## §3. Hard Thresholds

All thresholds are PASS/FAIL gates. Partial credit is not awarded.

### TH-01: Test Coverage
- `go test -tags=integration -coverprofile=coverage.out ./...` followed by `go tool cover -func=coverage.out`
- **Threshold**: NEW evidence-core file-union coverage >= 85.0%
- Evidence: statement-weighted union of `internal/store/evidence.go`, `cmd/server/evidence_handlers.go`, `internal/storage/storage.go`, `internal/audit/clock.go` (+ evidence additions in store.go/pg_store.go/audit.go/recorder.go/config.go) must show >= 85.0%
- Scope: NEW evidence-core files only

> **AMENDMENT (Phase 2.8a iteration 2, evaluator-active binding ruling — owns this contract):**
> The original "overall package total >= 85%" reading is NOT a faithful measurement for this brownfield SPEC. `internal/store` package total (~62%) is dominated by PRE-EXISTING SPEC-AX-CTRL-001 untested code (`postgres.go` Sprint-0 死 stub at 0%, `fake_store.go` workflow stubs, `recorder.go` workflow methods RecordTransition*/RecordFailedCallback/RecordCreateCancelled at 0%) explicitly excluded from SPEC-AX-EVID-001 Planned Files. Retroactively testing that out-of-scope code violates [HARD] scope discipline. TH-01 is therefore bound to the statement-weighted UNION of the new evidence-core file set. Measured: **91.4% (224/245 stmts) — PASS**. Rationale: contract phrase "evidence additions" + brownfield scope-discipline. Reports: `.moai/reports/evaluator/SPEC-AX-EVID-001-eval-2.md`.

### TH-02: Linting
- `golangci-lint run --enable=gosec ./...`
- **Threshold**: exit code 0, zero linting issues reported
- Includes gosec rules: G401 (weak crypto), G501 (MD5/SHA1 use), G304 (file path taint) — must all PASS
- Note: G304 is inapplicable if `database_blob` strategy is used (no file path construction from user input), but must be clean if `filesystem` strategy path exists in codebase

### TH-03: Goroutine Leak Detection
- `goleak.VerifyNone(t)` or `goleak.VerifyTestMain(m)` applied to all test functions that exercise TX paths
- **Threshold**: zero leaked goroutines in any test
- Required in: DC-009 (concurrent test), DC-016 (rollback test), DC-006 (handler integration)

### TH-04: Zero External Network Egress
- During all store/hash/retrieve operations: zero TCP/DNS connections to non-loopback hosts
- Verified by network dial spy (see DC-001) and static import check
- **Threshold**: 0 external connections, 0 external SDK imports

### TH-05: LSP Errors
- `gopls check ./...` must report 0 errors
- **Threshold**: exit code 0, zero LSP errors
- Applies at all sprint checkpoints (not only at Phase 2.8a)

### TH-06: `initial.sql` Immutability
- `git diff HEAD apps/control-plane/migrations/initial.sql` must return empty diff
- **Threshold**: zero modifications to `initial.sql`
- New evidence schema lives exclusively in `0002_evidence_tables.sql`
- Evidence: `git status --porcelain apps/control-plane/migrations/initial.sql` shows no changes

### TH-07: No New External SaaS SDK
- `go.mod` and `go.sum` must not introduce any new import for object storage SaaS: `aws-sdk-go*`, `minio-go`, `google-cloud-storage`, `azure-storage-blob`
- **Threshold**: `git diff HEAD go.mod | grep '^+' | grep -E 'aws-sdk|minio|google-cloud|azure'` returns empty
- Crypto: only `crypto/sha256` stdlib; no `golang.org/x/crypto` or external hash library

### TH-08: `postgres.go` Untouched (Dead Stub)
- `git diff HEAD apps/control-plane/internal/store/postgres.go` must return empty diff
- **Threshold**: zero modifications to `postgres.go`
- All evidence TX logic is in `pg_store.go` (extending `PgWorkflowStore`)
- Evidence: `git status --porcelain apps/control-plane/internal/store/postgres.go` shows no changes

### TH-09: MX Tags
- `EvidenceStore` interface → `@MX:ANCHOR` (fan_in will be ≥ 3)
- `EvidenceTx` interface → `@MX:ANCHOR`
- `RecordEvidenceCreated` / `RecordEvidenceVersioned` → `@MX:ANCHOR`
- Handler TX block (BeginEvidenceTx → InsertEvidence → RecordEvidence* → Commit) → `@MX:WARN` with `@MX:REASON: [AUTO] panic/early-return after InsertEvidence but before Commit → orphaned evidence row; defer Rollback immediately after BeginEvidenceTx`
- Version-resolve block (`GetLatestVersionByEvalItem` SELECT FOR UPDATE path) → `@MX:WARN` with `@MX:REASON: [AUTO] lock not held → concurrent version=2 race; SELECT FOR UPDATE serializes access`
- `EvidenceBlobStore` interface → `@MX:NOTE`
- `@MX:REASON` required on all WARN and ANCHOR tags; `[AUTO]` prefix required on all agent-generated tags

### TH-10: Brownfield ANCHOR Preservation
- Existing `@MX:ANCHOR` tags in `store.go:32` (WorkflowTx atomicity), `recorder.go:36` (Recorder single entry), `recorder.go:27` (local AuditTx), `pg_store.go:24` (PgWorkflowStore struct), `pg_store.go:99` (SELECT FOR UPDATE) must remain unchanged
- **Threshold**: zero modifications to existing MX tags in these files
- Verified by: `git diff HEAD apps/control-plane/internal/store/store.go apps/control-plane/internal/audit/recorder.go apps/control-plane/internal/store/pg_store.go | grep '@MX'` returning only additions (lines starting with `+`), no deletions

---

## §4. Security Review (OWASP — Multipart BYTEA Upload Endpoint)

### SEC-01: Content-Type Enforcement (OWASP A03 Injection)

**Concern**: Handler must reject non-`multipart/form-data` requests before attempting `r.ParseMultipartForm()`. Missing validation allows arbitrary bodies to exhaust the multipart parser.

**Contract requirement**: If `Content-Type` header is not `multipart/form-data` (or does not begin with `multipart/form-data;`), handler returns HTTP 400 with `{"error":{"code":"INVALID_ARGUMENT","message":"Content-Type must be multipart/form-data"}}` — before any body read attempt.

**Test**: `TestEvidenceHandler_WrongContentType` → asserts HTTP 400, `evidences` row count == 0.

---

### SEC-02: File Size Enforcement via Streaming (OWASP A05 Security Misconfiguration)

**Concern**: Buffering the entire file before size-check allows a 1 GiB body to exhaust heap memory. The correct pattern is streaming with `io.LimitReader`.

**Contract requirement**:
1. If `Content-Length` header is present and > `EVIDENCE_MAX_FILE_BYTES`, return HTTP 413 before reading body.
2. For all other cases, wrap the multipart file part with `io.LimitReader(part, maxBytes+1)` before reading into memory or hashing. If read length == maxBytes+1, return HTTP 400 (oversized) and discard remaining bytes.
3. SHA-256 hash must be computed by streaming through the `LimitReader` — NOT by reading all bytes first, then hashing. Pattern: `h := sha256.New(); io.Copy(h, limitedReader)`.
4. The entire file content (`[]byte`) passed to `InsertEvidence` must be the bytes read through this `LimitReader`, not a re-read of the original source.

**Test**: `TestEvidenceHandler_OversizedFile_NoHeapExhaustion` — sends a 52 MiB body without Content-Length; asserts HTTP 400 returned before reading all 52 MiB (verify via `bytesRead` counter on a custom Reader).

---

### SEC-03: SHA-256 Server-Side Integrity (OWASP A08 Data Integrity)

**Concern**: If the client supplies a pre-computed hash (not required by SPEC), accepting it without server-side recomputation allows hash spoofing.

**Contract requirement**: Server always computes SHA-256 from the raw bytes it receives. Client-supplied hash headers are ignored. The stored `file_hash_sha256` is exclusively the server-computed value.

**Verification**: `TestEvidenceSovereignty_NoExternalEgress` DC-001 item 3 covers stdlib-only hash. No test should accept a client-provided hash parameter.

---

### SEC-04: SQL Injection Prevention (OWASP A03 Injection)

**Concern**: pgx parameterized queries prevent injection by construction, but only if no string interpolation is used.

**Contract requirement**: All SQL statements in evidence-related code (`pg_evidence.go` or equivalent) must use `$N` placeholders exclusively. No `fmt.Sprintf`, `strings.Join` with user values, or raw string concatenation of user inputs into SQL strings.

**Verification**: `golangci-lint run --enable=gosec` with G201 (SQL string formatting) and G202 (SQL string concatenation) must report 0 issues. Additionally: `grep -rn 'fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT\|fmt.Sprintf.*UPDATE' apps/control-plane/internal/store/` must return empty.

---

### SEC-05: `evaluation_item_id` and `file_name` Length Pre-validation (OWASP A03 Injection / DoS)

**Concern**: User-supplied strings of unbounded length passed to the DB trigger ERROR 22001 (string too long), which returns a 500 to the client and leaks schema information.

**Contract requirement**: Handler validates `len(evaluationItemID) <= 64` and `len(fileName) <= 512` before entering any transaction. Returns HTTP 400 with appropriate `field` value if exceeded.

**Test**: `TestEvidenceHandler_InputValidation` sub-cases for oversized field values (see DC-007 gaps DC-007-G1, DC-007-G2).

---

### SEC-06: `storage_location` Non-traversal Guarantee (OWASP A01 Broken Access Control)

**Concern**: For the `database_blob` strategy, `storage_location` is server-generated (`db://evidences/<UUID>`). No user-controlled path component exists. However, any future `filesystem` strategy implementation must sanitize `file_name` before constructing paths.

**Contract requirement for this SPEC**: `storage_location` for `database_blob` must be `"db://evidences/" + evidenceID.String()`. `evidenceID` is a server-generated UUID — not derived from user input. Verified by DC-017 regex assertion.

**Note for future filesystem strategy**: If `filesystem` is implemented in a later SPEC, it must not use `file_name` (user-supplied) as any component of the storage path. Contract for this SPEC does not cover that implementation, but the evaluator will reject any code that constructs file paths from user input even if disabled by config.

---

### SEC-07: Goroutine Leak on Error Path (OWASP A05 — Availability)

**Concern**: If `defer tx.Rollback(ctx)` is not registered immediately after `BeginEvidenceTx`, a panic or early return anywhere in the TX block will leak the DB connection and associated goroutine.

**Contract requirement**: `defer evidenceTx.Rollback(ctx)` must appear as the first statement after the nil-check of `BeginEvidenceTx(ctx)`. Verified by goleak (TH-03) and code review assertion in DC-016.

---

## §5. Tasks.md Gap Register

Gaps identified during contract analysis that require SPEC amendment or implementation decision before the relevant sprint begins.

| Gap ID | Affected Task(s) | Description | Resolution Required Before |
|--------|-----------------|-------------|---------------------------|
| GAP-01 | T-013 | HTTP method and route path not specified. Contract mandates `POST /api/v1/evidences` (plural). | Sprint 3 start (T-013) |
| GAP-02 | T-004, T-005 | `EvidenceTx.InsertEvidence` method signature not shown in tasks.md. Must include `fileContent []byte` as parameter. Full signature: `InsertEvidence(ctx context.Context, evalItemID, fileName, contentType string, fileSizeBytes int64, fileHashSHA256 string, storageStrategy, storageLocation string, metadata map[string]string, fileContent []byte, previousVersionID *uuid.UUID) (uuid.UUID, error)` | Sprint 1 start (T-005) |
| GAP-03 | T-005 | `GetEvidenceByID` not-found case → sentinel error not in AC. Contract adds `ErrEvidenceNotFound` requirement (E-10, DC-012). | Sprint 1 start (T-005) |
| GAP-04 | T-008 | SUPERSEDED status UPDATE must be in store layer (EvidenceTx), not handler. Not explicitly stated in tasks.md. | Sprint 2 start (T-008) |
| GAP-05 | T-012, T-013 | `storage_location` format for `database_blob` (`db://evidences/<UUID>`) not asserted in any task. Contract adds DC-017 regex assertion. | Sprint 3 start (T-013) |
| GAP-06 | T-014 | `EVIDENCE_STORAGE_STRATEGY` invalid-value fail-fast test not defined. Contract adds `TestConfig_InvalidStorageStrategyRejectsAtLoad`. | Sprint 4 start (T-014) |
| GAP-07 | T-015 | `evaluation_item_id` > 64 chars pre-validation not in T-015 scope. Contract adds DC-007-G1. | Sprint 4 start (T-015) |
| GAP-08 | T-015 | `file_name` > 512 chars pre-validation not in T-015 scope. Contract adds DC-007-G2. | Sprint 4 start (T-015) |

---

## §6. Negotiation Round Status

**Round 1**: Produced by evaluator-active (2026-05-18). No prior implementation exists — this is a pre-implementation contract. Maximum 1 additional round available.

**Binding**: This contract is BINDING unless the implementation agent disputes a specific criterion with evidence that it contradicts the SPEC documents (spec.md v0.1.2, plan.md v0.1.2, acceptance.md, tasks.md, strategy.md). Disputes must reference the exact SPEC line number.

---

## §7. Scoring Profile at Phase 2.8a

Evaluator will apply `strict` profile (per harness.yaml `thorough` → `evaluator_profile: strict`).

| Dimension | Weight | Must-pass Criteria |
|-----------|--------|-------------------|
| Functionality (40%) | 40% | DC-001 through DC-019 all GREEN |
| Security (25%) | 25% | SEC-01 through SEC-07 all clean |
| Craft (20%) | 20% | TH-01 (≥85%), TH-02 (lint 0), TH-03 (goleak 0), TH-09 (MX tags) |
| Consistency (15%) | 15% | TH-08 (postgres.go untouched), TH-10 (ANCHOR preserved), TH-06 (initial.sql untouched) |

**Hard thresholds**: Security FAIL = Overall FAIL regardless of other scores. Coverage < 85% = Craft FAIL.

---

*Contract produced by evaluator-active Phase 2.0. Implementation scoring at Phase 2.8a will use this document as the authoritative criteria source.*
