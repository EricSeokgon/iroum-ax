# SPEC-AX-EVAL-ITEM-001 Sprint Contract

> Phase 2.0 — evaluator-active binding implementation contract (thorough harness)
> Status: PROPOSED (Round 1)
> SPEC version: v0.1.3 (Option A RESOLVED, AUD-1 RESOLVED, 22 AC, 18 edge cases)
> Methodology: TDD RED-GREEN-REFACTOR
> Produced before any implementation code is written.
> Implementation is scored against this document at Phase 2.8a.

---

## 1. Done Criteria

Each criterion is binary-checkable (PASS/FAIL deterministic via `go test` output, database row counts, or `information_schema` queries). "Works correctly" and "is implemented" are not acceptable evidence — only the listed artifact must be shown.

### DC-001 — Migration file correctness (T-001)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 1.1 | `0003_eval_item_tables.sql` exists at `.moai/db/schema/migrations/0003_eval_item_tables.sql` | file path exists |
| 1.2 | `initial.sql` is byte-identical to its pre-SPEC state | `git diff .moai/db/schema/initial.sql` = 0 lines |
| 1.3 | After sequential application of `0001_initial.sql` → `0002_evidence_tables.sql` → `0003_eval_item_tables.sql` in testcontainers, `SELECT data_type, character_maximum_length FROM information_schema.columns WHERE table_name='evaluation_items' AND column_name='id'` returns exactly `('character varying', 64)` | query result match |
| 1.4 | `evaluation_items` DDL contains: `parent_id VARCHAR(64) REFERENCES evaluation_items(id) ON DELETE RESTRICT` | `information_schema.referential_constraints` shows constraint `evaluation_items_parent_id_fkey` with `delete_rule='RESTRICT'` |
| 1.5 | `evaluation_items_status_chk` CHECK constraint exists with `status IN ('ACTIVE','DEPRECATED','ARCHIVED')` | `information_schema.check_constraints` row exists |
| 1.6 | Indexes `evaluation_items_parent_id_idx`, `evaluation_items_hierarchy_code_idx`, `evaluation_items_created_at_idx` exist | `pg_indexes` table query |
| 1.7 | `hierarchy_code` column has UNIQUE constraint | `information_schema.table_constraints` shows UNIQUE on `evaluation_items_hierarchy_code_idx` |
| 1.8 | `created_by` column default is `'cli-anonymous'` (NOT NULL) | `information_schema.columns.column_default = 'cli-anonymous'::character varying`, `is_nullable = 'NO'` |
| 1.9 | Existing `pg_store_test.go` WorkflowStore and EvidenceStore characterization tests remain GREEN after migration | test output shows 0 failures |

### DC-002 — Interface and constant skeleton (T-002)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 2.1 | `store.go` exports `EvalItemStore` interface with method `BeginEvalItemTx(ctx context.Context) (EvalItemTx, error)` | `go build ./...` passes + interface symbol visible via `go doc` |
| 2.2 | `store.go` exports `EvalItemTx` interface with exactly: `InsertEvalItem`, `GetEvalItemByID`, `GetEvalItemsByParentID`, `UpdateEvalItem`, `InsertAuditLog`, `Commit`, `Rollback` | `go doc` lists all 7 methods |
| 2.3 | `audit.go` declares `ActionEvalItemCreated Action = "EVAL_ITEM_CREATED"` and `ActionEvalItemUpdated Action = "EVAL_ITEM_UPDATED"` | string value comparison in test: `assert.Equal(t, "EVAL_ITEM_CREATED", string(audit.ActionEvalItemCreated))` |
| 2.4 | `audit.go` declares `EvalItemAuditNamespace` as a package-level constant or `var` of type `uuid.UUID`, assigned as a fixed UUID literal (e.g., `uuid.MustParse("...")`), NOT derived from `uuid.New()` at runtime | static code review: no call to `uuid.New()` in `EvalItemAuditNamespace` initialization; the value is a hardcoded UUID string |
| 2.5 | No existing symbols in `store.go`, `audit.go`, `recorder.go` are modified (additive-only) | `git diff` shows only additions, no deletions to existing lines in these files |

### DC-003 — Root item creation + metadata round-trip (T-003, AC-001-1, AC-001-3, AC-001-O1-1)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 3.1 | `InsertEvalItem` with `id="AX-SAFETY"`, `display_name="안전보건"`, `parent_id=nil`, `level=1` inserts exactly 1 row in `evaluation_items` with `parent_id IS NULL`, `status='ACTIVE'`, `created_by='cli-anonymous'` | `SELECT COUNT(*) FROM evaluation_items WHERE id='AX-SAFETY' AND parent_id IS NULL AND status='ACTIVE' AND created_by='cli-anonymous'` = 1 |
| 3.2 | The InsertEvalItem call returns the inserted `id` string `"AX-SAFETY"` without error | assert.Equal + assert.NoError |
| 3.3 | Latency of 10 consecutive InsertEvalItem calls (single node, no parent lookup) < 50ms p99 in testcontainers environment | `time.Since` measurement; test helper records durations and asserts `max(durations) < 50ms` |
| 3.4 | `metadata` JSONB provided as `{"등급기준": {"S": "...", "A": "..."}, "draft_note": "임의 중첩"}` round-trips with **semantic equivalence** (no field loss/mutation) | test reads back metadata column, unmarshals both original and stored as `map[string]any`, asserts `reflect.DeepEqual`; byte-level string comparison is PROHIBITED |
| 3.5 | No `metadata` field names are enforced or rejected (empty `{}` and arbitrary nesting accepted without error) | test inserts `{}`, `{"x": {"y": {}}}`, `null` metadata — no error returned |
| 3.6 | `evaluation_items.id` column type confirmed as `character varying(64)` via `information_schema.columns` query | query returns `data_type='character varying'`, `character_maximum_length=64` |

### DC-004 — Child creation, parent validation, input validation, BeginEvalItemTx wiring (T-004, AC-001-2, AC-001-S1-1, AC-001-4)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 4.1 | Child item `id="AX-SAFETY-ORG-01"`, `parent_id="AX-SAFETY"` inserts successfully when parent exists | row exists in DB with `parent_id='AX-SAFETY'` |
| 4.2 | After child insertion, `GetEvalItemsByParentID("AX-SAFETY")` returns a slice containing an item with `id="AX-SAFETY-ORG-01"` | assert.Len(result, 1); assert.Equal(result[0].ID, "AX-SAFETY-ORG-01") |
| 4.3 | Inserting with `parent_id="AX-NONEXISTENT-PARENT"` returns a non-nil error | assert.Error |
| 4.4 | After the failed insert in 4.3: `SELECT COUNT(*) FROM evaluation_items WHERE id='AX-ORPHAN-01'` = 0 (no row) | count = 0 |
| 4.5 | After the failed insert in 4.3: `SELECT COUNT(*) FROM audit_logs WHERE details->>'eval_item_id'='AX-ORPHAN-01'` = 0 | count = 0 |
| 4.6 | `id=""` (blank) returns structured error, no DB change | assert.Error; row count = 0 |
| 4.7 | `id` of 65 characters returns structured error, no DB change | assert.Error; row count = 0 |
| 4.8 | `display_name=""` (blank) returns structured error, no DB change | assert.Error; row count = 0 |
| 4.9 | `id="AX-DUP"` when row `"AX-DUP"` already exists returns structured error (PK violation), existing row unchanged | assert.Error; original row still present with original values |
| 4.10 | `BeginEvalItemTx` is implemented on `PgWorkflowStore` by calling `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})` — identical pattern to `BeginEvidenceTx` at `pg_store.go:103` | code review: `pg_store.go` shows `BeginEvalItemTx` using `s.pool`; `postgres.go` shows zero `EvalItemTx`-related changes |

### DC-005 — Hierarchy traversal (T-005, AC-002-1, AC-002-4)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 5.1 | Root item has `parent_id IS NULL` in DB | column null check |
| 5.2 | `GetEvalItemsByParentID("AX-SAFETY")` with children `[AX-SAFETY-ORG-01, AX-SAFETY-ORG-02]` returns exactly 2 items | assert.Len(result, 2); IDs present in result |
| 5.3 | `EXPLAIN` (or `EXPLAIN ANALYZE`) output for `GetEvalItemsByParentID` shows `Index Scan` on `evaluation_items_parent_id_idx` (not Seq Scan) | test queries `EXPLAIN SELECT ... WHERE parent_id=$1` and asserts `strings.Contains(explainOutput, "Index Scan")` |
| 5.4 | `GetEvalItemsByParentID` query p99 < 50ms in testcontainers for a 3-level tree | latency assertion |
| 5.5 | 3-level sequential traversal: `GetEvalItemsByParentID("AX-SAFETY")` → [AX-SAFETY-ORG-01]; then `GetEvalItemsByParentID("AX-SAFETY-ORG-01")` → [AX-SAFETY-ORG-01-1]; no cross-level leakage | exact ID assertions at each level |
| 5.6 | `GetEvalItemsByParentID` for a leaf node (no children) returns an empty slice, NOT an error | assert.NoError; assert.Empty(result) |

### DC-006 — FK RESTRICT and hierarchy_code uniqueness (T-006, AC-002-2, AC-002-3)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 6.1 | Direct SQL `DELETE FROM evaluation_items WHERE id='AX-SAFETY-CAT'` when child `AX-SAFETY-CAT-01` exists returns a pgx error containing FK constraint violation | assert.Error containing "evaluation_items_parent_id_fkey" or "foreign key" |
| 6.2 | After the failed DELETE in 6.1: both `AX-SAFETY-CAT` and `AX-SAFETY-CAT-01` rows still exist | count = 2 |
| 6.3 | Inserting `id="AX-B"` with `hierarchy_code="AX.SAFETY.ORG.01"` when `AX-A` already has the same hierarchy_code returns a non-nil error | assert.Error |
| 6.4 | After the failed insert in 6.3: `AX-B` row does not exist; `AX-A` row is unchanged | AX-B count = 0; AX-A values unchanged |

### DC-007 — AUD-1 deterministic UUIDv5 audit recorder (T-007, AC-003-1, AC-003-2, AC-003-E2-1)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 7.1 | `RecordEvalItemCreated(ctx, tx, itemID="AX-SAFETY-ORG-01", hierarchyCode="AX.SAFETY.ORG.01", ...)` inserts an `audit_logs` row with `action='EVAL_ITEM_CREATED'`, `resource_type='evaluation_item'`, `user_id='cli-anonymous'` | DB row query; column value assertions |
| 7.2 | The inserted `resource_id` equals `uuid.NewSHA1(audit.EvalItemAuditNamespace, []byte("AX.SAFETY.ORG.01"))` — computed in the test and compared byte-by-byte | `assert.Equal(t, expected, actual)` where `expected` is computed in test code |
| 7.3 | `resource_id != uuid.Nil` | assert.NotEqual(t, uuid.Nil, resource_id) |
| 7.4 | `resource_id` is NOT the raw string `"AX-SAFETY-ORG-01"` (a VARCHAR(64) cannot be stored as uuid.UUID — this assertion proves the surrogate path was taken) | The test explicitly asserts `resource_id != uuid.MustParse("00000000-0000-0000-0000-000000000000")` AND `resource_id.String() != "AX-SAFETY-ORG-01"` (the latter is type-impossible but the surrogate UUID != any UUID derived from "AX-SAFETY-ORG-01" as if it were a UUID-format string) |
| 7.5 | `details->>'eval_item_id' = 'AX-SAFETY-ORG-01'` in the audit row | DB query `SELECT details->>'eval_item_id' FROM audit_logs WHERE id=...` = 'AX-SAFETY-ORG-01' |
| 7.6 | `details->>'hierarchy_code' = 'AX.SAFETY.ORG.01'` in the audit row | DB query result = 'AX.SAFETY.ORG.01' |
| 7.7 | `RecordEvalItemUpdated` inserts row with `action='EVAL_ITEM_UPDATED'`, same `resource_id` derivation logic, same DetailsJSON contract | same assertions as 7.1–7.6 for update path |
| 7.8 | **Determinism — same input**: Calling `RecordEvalItemCreated` then `RecordEvalItemUpdated` with the same `hierarchyCode="AX.SAFETY.ORG.01"` produces two audit rows with byte-identical `resource_id` | `assert.Equal(t, row1.ResourceID, row2.ResourceID)` |
| 7.9 | **Determinism — different input**: `hierarchyCode="AX.SAFETY.ORG.02"` produces a different `resource_id` than `"AX.SAFETY.ORG.01"` | `assert.NotEqual(t, uuidForCode01, uuidForCode02)` |
| 7.10 | The `resource_id` UUID is RFC 4122 version 5 (SHA-1 name-based): `resource_id.Version() == 5` | assert.Equal(t, uuid.Version(5), resource_id.Version()) |
| 7.11 | `store.go` (EvalItemTx), `eval_item.go` do NOT import `internal/store` from `internal/audit` package (no circular import) | `go build ./...` passes; `go list -f '{{.Imports}}' ./internal/audit/...` does NOT contain `internal/store` |

### DC-008 — Bidirectional rollback atomicity (T-008, AC-003-3)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 8.1 | With fault injection that causes `audit_logs` INSERT to fail (e.g., via a CHECK constraint violation or a test-controlled error injection on the AuditTx), after the TX rollback: `SELECT COUNT(*) FROM evaluation_items WHERE id='AX-FAULT-ITEM'` = 0 | count = 0 |
| 8.2 | `SELECT COUNT(*) FROM audit_logs WHERE details->>'eval_item_id'='AX-FAULT-ITEM'` = 0 | count = 0 |
| 8.3 | The returned error wraps the audit insertion failure (not nil, not a generic error) | assert.ErrorContains or assert.Error; error message contains audit context |
| 8.4 | `goleak.VerifyNone(t)` passes for the fault-injection test case | goleak output 0 leaked goroutines |
| 8.5 | Same rollback behavior applies to `UpdateEvalItem` → `RecordEvalItemUpdated` → audit fail path (update rolled back, audit row absent, no leak) | same assertions for update path |

### DC-009 — Hierarchy immutability and lifecycle (T-009, AC-004-1, AC-004-2, AC-004-3, AC-UBI-004)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 9.1 | `UpdateEvalItem("AX-SAFETY-CAT", parent_id="AX-OTHER")` with child `AX-SAFETY-CAT-01` returns a non-nil error BEFORE any SQL is executed (the mutation guard fires at the app layer, not from a DB constraint) | assert.Error; verified by: the error message references "children" or "hierarchy invariant" (not a pgx FK error) |
| 9.2 | After the rejection in 9.1: `AX-SAFETY-CAT.parent_id` is unchanged | DB read confirms original parent_id |
| 9.3 | `UpdateEvalItem("AX-SAFETY-CAT", level=3)` similarly rejected when children exist | same as 9.1 |
| 9.4 | **Leaf node positive case**: `UpdateEvalItem` on a node with no children successfully changes `parent_id` or `level` (leaf nodes MAY have hierarchical columns changed per REQ-EVALITEM-UBI-004) | assert.NoError; DB shows updated parent_id/level on the leaf node |
| 9.5 | `UpdateEvalItem("AX-LEAF", status='DEPRECATED')` succeeds; DB shows `status='DEPRECATED'` | row query |
| 9.6 | `UpdateEvalItem("AX-LEAF", status='ARCHIVED')` succeeds; DB shows `status='ARCHIVED'` | row query |
| 9.7 | Each status transition produces exactly 1 `EVAL_ITEM_UPDATED` audit row in the same TX | `SELECT COUNT(*) FROM audit_logs WHERE action='EVAL_ITEM_UPDATED' AND details->>'eval_item_id'='AX-LEAF'` = 1 per transition |
| 9.8 | `UpdateEvalItem("AX-LEAF", status='INVALID_X')` returns error; no DB change | assert.Error; status unchanged |
| 9.9 | `UpdateEvalItem` with `status=NULL` returns error; no DB change | assert.Error; status unchanged |
| 9.10 | Leaf node update of `display_name`, `weight`, `max_score`, `metadata` succeeds atomically with 1 `EVAL_ITEM_UPDATED` audit row | all column values updated; audit count = 1 |
| 9.11 | `metadata` update in `UpdateEvalItem` also uses semantic JSONB equality (no byte comparison) | same constraint as DC-003.4 |

### DC-010 — UBI invariants + boundary + quality gate (T-010, AC-UBI-001..004, AC-BOUNDARY-1)

| # | Criterion | Binary check |
|---|-----------|-------------|
| 10.1 | Static import check: `internal/store` package imports do NOT include any external SaaS SDK (`aws-sdk-go`, `google/cloud`, etc.) | `go list -f '{{.Imports}}' ./internal/store/...` output does not contain external cloud SDKs |
| 10.2 | Static import check: `internal/audit` package imports do NOT include any external SaaS SDK | same for `./internal/audit/...` |
| 10.3 | `created_by='cli-anonymous'` (exact bytes, NOT NULL, NOT empty string) after any `InsertEvalItem` call with no explicit user | assert.Equal(t, "cli-anonymous", item.CreatedBy) |
| 10.4 | `audit_logs.user_id='cli-anonymous'` (exact bytes) for the corresponding audit row | assert.Equal(t, "cli-anonymous", auditRow.UserID) |
| 10.5 | Both `evaluation_items.created_by` and `audit_logs.user_id` are `'cli-anonymous'` — cross-table consistency | assert.Equal(t, evalItem.CreatedBy, auditRow.UserID) |
| 10.6 | After `InsertEvalItem` and `UpdateEvalItem`, `audit_logs` contains exactly 1 row per operation with AUD-1 `resource_id` (UUIDv5) and `details->>'eval_item_id'` matching the item id | COUNT queries per operation = 1 |
| 10.7 | After `0003_eval_item_tables.sql` is applied, `information_schema.referential_constraints` returns 0 rows for any FK from `evidences` table referencing `evaluation_items` | `SELECT COUNT(*) FROM information_schema.referential_constraints rc JOIN information_schema.table_constraints tc ON rc.constraint_name=tc.constraint_name WHERE tc.table_name='evidences' AND rc.unique_constraint_name LIKE '%evaluation_items%'` = 0 |
| 10.8 | `evidences` table schema (columns, constraints) is byte-identical to its pre-SPEC state | `git diff` of any `evidences`-related migration or schema file = 0 |
| 10.9 | Both `evaluation_items.id` and `evidences.evaluation_item_id` have `character_maximum_length=64` in `information_schema.columns` | separate queries for each column, both return 64 |
| 10.10 | `go test -coverprofile=coverage.out ./apps/control-plane/internal/...` reports `≥ 85.0%` | coverage output |
| 10.11 | `golangci-lint run --enable gosec ./apps/control-plane/...` exits 0 with 0 issues | lint exit code = 0 |
| 10.12 | `goleak.VerifyNone(t)` passes for ALL test functions in `eval_item_test.go` and `recorder_eval_item_test.go` | goleak output 0 leaked goroutines per test |
| 10.13 | Existing `WorkflowStore`/`EvidenceStore`/`Recorder` characterization tests pass unchanged (regression = 0) | test output shows 0 failures for pre-existing tests |

---

## 2. Edge Cases for Coverage

The following edge cases MUST each have a dedicated test assertion. Cases already covered by the AC list above are noted with their DC reference; new cases not in the original acceptance.md are marked **[CONTRACT ADDITION]**.

| # | Edge Case | DC Reference / AC | Why Required |
|---|-----------|-------------------|-------------|
| E-01 | Non-existent parent_id INSERT → FK violation, row NOT inserted, audit NOT inserted (orphan prevention) | DC-004.3–4.5, AC-001-S1-1 | Both app-layer and DB-layer rejection paths must leave the DB clean |
| E-02 | `hierarchy_code` UNIQUE violation → INSERT rejected, TX rolled back, existing row unchanged | DC-006.3–6.4, AC-002-3 | Path: DB UNIQUE constraint vs store pre-check — either is acceptable, but clean rollback is mandatory |
| E-03 | Parent ON DELETE RESTRICT with children → DELETE rejected by DB | DC-006.1–6.2, AC-002-2 | Tested via raw SQL, not store API (no delete API exists per spec) |
| E-04 | Child-bearing node `parent_id` mutation REJECTED by store (app-layer guard, not DB constraint) | DC-009.1–9.3, AC-004-1 | Guard must fire BEFORE SQL execution; error message must be domain-level |
| E-05 | **[CONTRACT ADDITION]** Leaf node (no children) `parent_id` CHANGE SUCCEEDS | DC-009.4 | REQ-EVALITEM-UBI-004 explicitly permits leaf parent_id changes; the mutation guard must NOT over-generalize by blocking all parent_id changes |
| E-06 | status CHECK: `'INVALID_X'` and `NULL` both rejected | DC-009.8–9.9, AC-004-2 | Both store pre-validation AND DB CHECK are allowed, but the combined effect must reject both cases |
| E-07 | AUD-1 `resource_id` != `uuid.Nil` (audit row always gets a real UUID, never the nil UUID) | DC-007.3, AC-003-1/003-2 | The parseResourceID fallback path (which returns uuid.Nil for non-UUID strings) must NOT be called — RecordEvalItem* must use uuid.NewSHA1 directly |
| E-08 | AUD-1 determinism: same `hierarchyCode` on two calls → byte-identical `resource_id` | DC-007.8, AC-003-E2-1 | Proves EvalItemAuditNamespace is a fixed constant, not runtime-generated |
| E-09 | AUD-1 collision resistance: different `hierarchyCodes` → different `resource_id` values | DC-007.9, AC-003-E2-1 | Verifies the surrogate can group audit rows by hierarchy_code correctly |
| E-10 | Audit INSERT failure → BOTH `evaluation_items` row AND `audit_logs` row absent (all-or-nothing) | DC-008.1–8.2, AC-003-3 | Full bidirectional rollback — partial commit (item exists, audit absent) is a FAIL condition |
| E-11 | metadata JSONB semantic equality (NOT byte comparison) — input with nested objects round-trips correctly | DC-003.4, AC-001-O1-1 | PostgreSQL JSONB normalizes key order; byte comparison fails on correct implementations |
| E-12 | metadata with `{}` (empty object), arbitrary nesting, and `null` all accepted without validation error | DC-003.5, AC-001-O1-1 | Opaque placeholder — any valid JSONB must be accepted |
| E-13 | `cli-anonymous` literal in BOTH `evaluation_items.created_by` AND `audit_logs.user_id` — cross-table | DC-010.3–10.5, AC-UBI-003 | Byte-identical comparison of both columns |
| E-14 | `evidences.evaluation_item_id` FK ABSENT after `0003_eval_item_tables.sql` applied | DC-010.7–10.8, AC-BOUNDARY-1 | Confirms this SPEC does NOT add the FK (FK hardening is a future SPEC) |
| E-15 | `evaluation_items.id` type = `character varying(64)` (not UUID, not integer) | DC-003.6, AC-001-3 | §1.4 HARD contract — EVID-001 FK type compatibility |
| E-16 | `goleak.VerifyNone` passes for fault-injection (audit-fail) test | DC-008.4, AC-003-3 | No goroutine leak on error paths |
| E-17 | `GetEvalItemsByParentID` for a leaf node (zero children) returns empty slice, NOT error | DC-005.6, **[CONTRACT ADDITION]** | The query `WHERE parent_id=$1` on a node that is nobody's parent must return `[]` — not pgx.ErrNoRows |
| E-18 | `id` blank / > 64 chars / `display_name` blank / duplicate PK — all four produce structured errors with no DB change | DC-004.6–4.9, AC-001-4 | Input validation before transaction; PK collision must also be tested |

---

## 3. Hard Thresholds

All thresholds are binary gates. Implementation MUST satisfy ALL of the following before Phase 2.8a evaluation. No negotiation or partial credit.

| Threshold | Requirement | Failure condition |
|-----------|-------------|-------------------|
| Test coverage | `go test -coverprofile` reports **≥ 85.0%** for `./apps/control-plane/internal/...` | Coverage < 85% → FAIL |
| Linter | `golangci-lint run --enable gosec` exits **0** with **0 issues** | Any issue → FAIL. Exception: gosec G401/G501 false positive for `uuid.NewSHA1` MUST be suppressed with `//nolint:gosec // RFC 4122 UUID v5 name-based, not cryptographic` inline comment; this nolint comment is REQUIRED (not optional) |
| Goroutine leak | `goleak.VerifyNone(t)` passes for **every** test function in `eval_item_test.go` and `recorder_eval_item_test.go` | Any leaked goroutine → FAIL |
| External network egress | Zero external TCP/DNS connections in any test execution path | Any external connection → FAIL |
| initial.sql immutability | `git diff .moai/db/schema/initial.sql` = zero lines changed | Any diff → FAIL |
| postgres.go immutability | `git diff apps/control-plane/internal/store/postgres.go` = zero lines changed | Any diff → FAIL |
| evidences table immutability | Zero changes to `evidences` table definition or data migrations | Any change → FAIL |
| No new external dependency | `go.mod` diff shows no new `require` lines (beyond `google/uuid` already present) | New dependency → FAIL |
| Brownfield @MX:ANCHOR intact | `store.go:18/34/59/71`, `audit/recorder.go:28`, `recorder.go:37/246/272`, `pg_store.go:24` — all still present and unchanged | Any modification to these lines → FAIL |
| @MX additive-only | No existing `@MX` tags removed or modified | Any removal/modification → FAIL |
| MX:TODO resolved | All `@MX:TODO` tags introduced during RED phase are removed or replaced with `@MX:ANCHOR`/`@MX:WARN` in REFACTOR phase | Any `@MX:TODO` remaining at Sprint 5 → FAIL |
| Brownfield characterization regression | `pg_store_test.go` WorkflowStore + EvidenceStore tests: 0 failures | Any failure → FAIL |
| recorder_test.go regression | Existing `recorder_test.go` tests: 0 failures | Any failure → FAIL |
| LSP errors | `go build ./...` and `go vet ./...` exit 0 | Any error → FAIL |
| Phantom-path guard | `BeginEvalItemTx` implementation is in `pg_store.go` using `PgWorkflowStore.pool`; `postgres.go` is unchanged | Any EvalItem code in `postgres.go` → FAIL |

---

## 4. Security Review

OWASP-relevant findings for a hierarchical taxonomy store. These are BINDING: each identified concern must be addressed or explicitly documented as not applicable.

### SEC-01 — SQL Injection via parameterized queries (OWASP A03:2021)

**Requirement**: ALL SQL queries in `eval_item.go` MUST use pgx parameterized placeholders (`$1`, `$2`, ...). No user-provided values (id, hierarchy_code, display_name, parent_id, status, metadata, etc.) may be concatenated into SQL strings.

**Binary check**: Code review of every `pgx.Tx.Exec`/`Query`/`QueryRow` call in `eval_item.go` — all must show `$N` placeholders for user-supplied values.

**Risk**: pgx parameterized queries are the primary defense. The VARCHAR(64) id field accepts arbitrary strings that could include SQL fragments if interpolated directly.

### SEC-02 — Dynamic column injection in UpdateEvalItem (OWASP A03:2021)

**Requirement**: The `UpdateEvalItem` implementation MUST use a hardcoded SET clause or a static list of column names derived from code constants — NOT from user-provided keys. If the implementation uses a dynamic map to build the SET clause (e.g., `for k, v := range updates { query += k + "=" ... }`), this is a HIGH-severity injection vector.

**Binary check**: Code review of `UpdateEvalItem`'s SQL construction. Acceptable: `UPDATE evaluation_items SET display_name=$1, description=$2, weight=$3, ... WHERE id=$N`. Not acceptable: any `fmt.Sprintf` or string concatenation involving column names from user input.

**Risk**: If column names are dynamically injected, an attacker could pass `"id; DROP TABLE evaluation_items; --"` as a column name key.

### SEC-03 — Recursive/self-ref query depth (OWASP — DoS)

**Requirement**: `GetEvalItemsByParentID` is a single-level query (no `WITH RECURSIVE`). This SPEC does NOT use `WITH RECURSIVE`, so unlimited recursion is not a risk at the SQL level. However, if the caller loops `GetEvalItemsByParentID` to walk the tree, cycles introduced by incorrect `parent_id` data could cause application-level infinite loops.

**Requirement**: The `UpdateEvalItem` mutation guard (preventing parent_id changes for child-bearing nodes) and the PoC manual discipline for circular parent detection (per REQ-EVALITEM-002-U1) are the mitigations. `@MX:WARN` on `GetEvalItemsByParentID` is REQUIRED (per plan.md §5).

**Binary check**: `@MX:WARN` annotation present on `GetEvalItemsByParentID` with `@MX:REASON` citing circular parent reference risk.

### SEC-04 — Audit immutability (OWASP A09:2021 — Security Logging)

**Requirement**: `eval_item.go` and `recorder.go` additions MUST NOT contain any `UPDATE` or `DELETE` statements targeting `audit_logs`. The `InsertAuditLog` method MUST be insert-only. Audit records are forensic evidence and must be immutable.

**Binary check**: Code review — no `UPDATE audit_logs` or `DELETE FROM audit_logs` in any new code.

### SEC-05 — EvalItemAuditNamespace fixed-constant (non-mutable)

**Requirement**: `EvalItemAuditNamespace` MUST be a fixed UUID literal defined at compile time (e.g., `var EvalItemAuditNamespace = uuid.MustParse("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx")`). It MUST NOT be:
- Generated via `uuid.New()` at program start (would make UUIDs non-deterministic across restarts)
- Read from environment variables or configuration files (would make it mutable)
- Derived from other runtime state

**Rationale**: The namespace is not secret (it can be known by anyone), but it MUST be stable. A mutable namespace breaks audit trail continuity — existing audit rows can no longer be correlated with new ones for the same hierarchy_code.

**Binary check**: Static code review of `audit.go` — `EvalItemAuditNamespace` initialization shows a hardcoded UUID string literal, no runtime calls.

### SEC-06 — cli-anonymous no real-user leakage (OWASP A07:2021)

**Requirement**: When `AUTH_ENABLED=false`, the `resolveUserID` function MUST return the literal `"cli-anonymous"` without inspecting any request-context user identity (there is none in the Walking Skeleton). The implementation MUST reuse the existing `audit.DefaultUserID` constant and `Recorder.resolveUserID` method — no new user-resolution logic is permitted.

**Binary check**: `RecordEvalItemCreated`/`RecordEvalItemUpdated` call `r.resolveUserID(userID)` (the existing method) for the `UserID` field. No new context.Value() calls for user identity.

### SEC-07 — gosec G401 false positive for uuid.NewSHA1

**Requirement**: If `golangci-lint` with gosec reports G401 ("Use of weak cryptographic primitive") or G501 for `uuid.NewSHA1`, the implementation MUST suppress the warning with an inline nolint comment: `//nolint:gosec // RFC 4122 UUID v5 uses SHA-1 by specification; this is not a cryptographic hash`.

**Rationale**: UUIDv5 is standardized as SHA-1-based (RFC 4122). gosec's G401 applies to cryptographic hashing; UUIDv5 is an identifier derivation, not a security primitive. Suppression without comment is NOT acceptable.

---

## 5. Tasks.md Gap Analysis

The following issues were identified during contract analysis. Items marked [BLOCKER] must be resolved before implementation begins. Items marked [ADVISORY] are noted concerns that should be addressed in the implementation.

### GAP-01 [BLOCKER] — Leaf node parent_id change not tested in any AC

**Finding**: REQ-EVALITEM-UBI-004 and spec.md explicitly state that leaf nodes (no children) MAY have `parent_id`/`level` changed. However, no AC in acceptance.md verifies the POSITIVE case — that a leaf node update of `parent_id` SUCCEEDS. Only the negative case (child-bearing node rejection) is tested (AC-UBI-004, AC-004-1).

**Risk**: The mutation guard implementation could over-generalize and reject ALL `parent_id`/`level` changes (making leaf changes always fail). This would satisfy AC-004-1 (which only tests the child-bearing case) while being incorrect per the spec. This is a classic guard over-specification bug.

**Required action**: DC-009.4 (added in this contract) requires an explicit PASS test for leaf node `parent_id` change. Implementation MUST include this test in `eval_item_test.go` under T-009.

### GAP-02 [ADVISORY] — GetEvalItemsByParentID empty-result case not tested

**Finding**: None of the 22 ACs explicitly test `GetEvalItemsByParentID` when called on a node with no children. The behavior should be: return empty slice (not error). If the implementation returns `pgx.ErrNoRows` for the empty case, callers will see spurious errors.

**Required action**: DC-005.6 (added in this contract) requires this test. Note: EVID-001 has a similar pattern (AC-EVID-001 GetEvidencesByWorkflowID empty case).

### GAP-03 [ADVISORY] — hierarchy_code NULL nullability unspecified

**Finding**: The `0003_eval_item_tables.sql` DDL defines `hierarchy_code VARCHAR(128) UNIQUE` without `NOT NULL`. PostgreSQL treats NULL as distinct for UNIQUE constraints (multiple NULLs are allowed). The recorder signature requires a `hierarchyCode string` parameter — if `hierarchy_code` is NULL in the DB (e.g., inserted as empty string `""` which maps to SQL NULL via the store), `uuid.NewSHA1(namespace, []byte(""))` would produce a valid but potentially unexpected UUID.

**Required action**: The implementation MUST ensure that `hierarchy_code` is NEVER stored as SQL NULL or empty string when the recorder is to be called. The `InsertEvalItem` method should validate that `hierarchyCode` is non-blank (same validation pattern as `id` and `display_name` in REQ-EVALITEM-001-U1). This validation is not explicitly specified in REQ-EVALITEM-001-U1 but is implied by the audit surrogate requirement.

**Recommended implementation addition**: Add `hierarchyCode == ""` to the pre-INSERT validation in `InsertEvalItem`. Flag in the task T-003/T-004 implementation — this is not a spec change, it is an implementation invariant required to keep AUD-1 derivation coherent.

### GAP-04 [ADVISORY] — T-001 creates eval_item_migration_test.go (not in spec.md §2.3)

**Finding**: tasks.md T-001 specifies a new test file `eval_item_migration_test.go` which is not in spec.md §2.3's planned test file list. The spec §2.3 lists only `eval_item_test.go` and `recorder_eval_item_test.go`.

**Assessment**: This is acceptable elaboration. The migration test isolates the DDL verification logic from the store tests. No spec amendment needed. However, the migration test file must be counted toward coverage.

### GAP-05 [ADVISORY] — UpdateEvalItem partial-update semantics unspecified

**Finding**: The `UpdateEvalItem` method signature in strategy.md §5 shows `(ctx, id string, ...) error` with `...` for the updatable fields. The spec does not specify whether `UpdateEvalItem` takes a struct with all fields (some nil/zero = no change) or explicit field parameters.

**Required action**: The implementation must choose and document one of: (a) struct with nullable fields (zero/nil = no update), or (b) explicit field list. Either approach is acceptable, but the mutation guard logic must correctly distinguish "no parent_id change requested" (nil pointer) from "parent_id change requested to nil/new value". The test at DC-009.1 must verify the guard fires only when a non-nil `parent_id` change is provided to a child-bearing node.

---

## 6. Negotiation Status

Round 1 — Evaluator proposal. Implementation has not started.
If the implementing agent identifies any criterion in §1 that is not achievable with the planned architecture, the agent MUST surface a structured blocker report to the orchestrator before beginning implementation. No silent deviations from this contract are permitted.

Maximum 2 negotiation rounds per harness rules. After Round 2 (or if no disputes are raised), this contract is final.

---

## Appendix A — AC → DC Traceability

| AC | DC criteria |
|----|-------------|
| AC-EVALITEM-UBI-001 | DC-010.1–10.2 |
| AC-EVALITEM-UBI-002 | DC-010.6, DC-007.1, DC-007.7, DC-008.1–8.5 |
| AC-EVALITEM-UBI-003 | DC-010.3–10.5 |
| AC-EVALITEM-UBI-004 | DC-009.1–9.3 |
| AC-EVALITEM-001-1 | DC-003.1–3.3 |
| AC-EVALITEM-001-2 | DC-004.1–4.2 |
| AC-EVALITEM-001-S1-1 | DC-004.3–4.5 |
| AC-EVALITEM-001-3 | DC-001.3, DC-003.6, DC-010.9 |
| AC-EVALITEM-001-4 | DC-004.6–4.9 |
| AC-EVALITEM-001-O1-1 | DC-003.4–3.5 |
| AC-EVALITEM-002-1 | DC-005.1–5.4 |
| AC-EVALITEM-002-2 | DC-006.1–6.2 |
| AC-EVALITEM-002-3 | DC-006.3–6.4 |
| AC-EVALITEM-002-4 | DC-005.5 |
| AC-EVALITEM-003-1 | DC-007.1–7.6, DC-007.11 |
| AC-EVALITEM-003-2 | DC-007.7 |
| AC-EVALITEM-003-E2-1 | DC-007.8–7.10 |
| AC-EVALITEM-003-3 | DC-008.1–8.5 |
| AC-EVALITEM-004-1 | DC-009.1–9.3 |
| AC-EVALITEM-004-2 | DC-009.5–9.9 |
| AC-EVALITEM-004-3 | DC-009.10–9.11 |
| AC-EVALITEM-BOUNDARY-1 | DC-010.7–10.9 |
| [Contract Addition] Leaf parent_id change | DC-009.4 |
| [Contract Addition] Empty GetEvalItemsByParentID | DC-005.6 |
