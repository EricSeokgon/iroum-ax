# SPEC-AX-SCORE-001 Sprint Contract

**Evaluator**: evaluator-active (Phase 2.0, thorough harness)
**Status**: BINDING — implementation scored against this contract in Phase 2.8a
**Negotiation round**: 1 of 2

---

## 1. Done Criteria

Binary-checkable, mapped to AC IDs and T-IDs. "Pass" means the assertion is reproducible via deterministic test or static grep.

### DC-UBI-001 — Zero External Egress (T-013, T-011)

```
PASS conditions:
  1. grep -rn '".*aws\|openai\|anthropic\|external.*llm"' \
       apps/control-plane/internal/store/ \
       apps/control-plane/internal/audit/ = 0 matches
  2. All 3 paths (InsertScore, SumWeightedByEvaluationItem, DetermineGrade)
     succeed in testcontainers test with external network blocked.
```

### DC-UBI-002 — 1 TX = 1 score entity = 1 audit row (T-009, T-013)

```
PASS conditions:
  1. After InsertScore+Commit:
     SELECT COUNT(*) FROM audit_logs WHERE resource_type='score'
       AND action='SCORE_CREATED' = 1
  2. After UpdateScore(DRAFT)+Commit:
     SELECT COUNT(*) FROM audit_logs WHERE resource_type='score'
       AND action='SCORE_UPDATED' = 1
  3. No test allows a score row to exist without a corresponding audit row
     (assert within same test TX or read-committed snapshot).
```

### DC-UBI-003 — cli-anonymous literal exact (T-013)

```
PASS conditions:
  1. SELECT created_by FROM scores WHERE id=$inserted_id
       = 'cli-anonymous' (byte-identical, not NULL)
  2. SELECT user_id FROM audit_logs WHERE resource_id=$inserted_id
       = 'cli-anonymous' (byte-identical, not NULL)
```

### DC-UBI-004 — No DeleteScore, status state-machine enforced (T-012, T-013)

```
PASS conditions:
  1. grep -n "DeleteScore" \
       apps/control-plane/internal/store/store.go = 0 matches
  2. UpdateScore on CONFIRMED row with score_value change
       → structured error returned; SELECT score_value FROM scores unchanged
  3. CONFIRMED correction path:
       SELECT COUNT(*) FROM scores WHERE evaluation_item_id=$eid = 2
       (original row status='SUPERSEDED', new row status='CONFIRMED')
  4. SELECT COUNT(*) FROM audit_logs WHERE resource_type='score' = 2
     (one SCORE_UPDATED for SUPERSEDED, one SCORE_CREATED for new row)
  [Gap-02 resolution: DC-UBI-004-D asserts both audit rows explicitly]
```

### DC-001-E1 — InsertScore happy path, atomicity (T-004, T-009)

```
PASS conditions:
  1. InsertScore returns non-nil UUID (not uuid.Nil)
  2. SELECT COUNT(*) FROM scores WHERE id=$returned_id = 1
  3. SELECT COUNT(*) FROM audit_logs WHERE resource_id=$returned_id = 1
  4. Both rows exist within same logical TX boundary (test asserts after Commit)
  5. p99 latency < 50ms over 10 iterations (testcontainers timing loop)
```

### DC-001-S1 — FK-less type stubs, schema exactness (T-002, T-004)

```
PASS conditions:
  1. SELECT data_type FROM information_schema.columns
       WHERE table_name='scores' AND column_name='evaluation_item_id'
       = 'character varying'
  2. SELECT character_maximum_length FROM information_schema.columns
       WHERE table_name='scores' AND column_name='evaluation_item_id'
       = 64
  3. SELECT data_type FROM information_schema.columns
       WHERE table_name='scores' AND column_name='evidence_id'
       = 'uuid'
  4. SELECT is_nullable FROM information_schema.columns
       WHERE table_name='scores' AND column_name='evidence_id'
       = 'YES'
  5. SELECT data_type FROM information_schema.columns
       WHERE table_name='scores' AND column_name='id'
       = 'uuid'
     [Gap-04 resolution: id type explicitly verified]
  6. SELECT COUNT(*) FROM information_schema.referential_constraints
       WHERE constraint_schema='public'
       AND (unique_constraint_name LIKE '%evaluation_items%'
            OR unique_constraint_name LIKE '%evidences%') = 0
  7. InsertScore with evidence_id='11111111-1111-1111-1111-111111111111'
     (no row in evidences table) → success, no FK violation error
```

### DC-001-S2 — Status state-machine transitions (T-012)

```
PASS conditions:
  1. UpdateScore(id=$draft_id, score_value=75.00) on DRAFT row
       → row updated in-place, score_value=75.00, status='DRAFT'
       + 1 SCORE_UPDATED audit row
  2. UpdateScore(id=$confirmed_id, score_value=95.00) on CONFIRMED row
       → structured error returned; score_value unchanged; 0 new audit rows
  3. UpdateScore(id=$draft_id, status='CONFIRMED') on DRAFT row
       → status='CONFIRMED'; 1 SCORE_UPDATED audit row
  4. No code path produces status value outside {DRAFT,CONFIRMED,SUPERSEDED}
     (verify by CHECK constraint in DDL: SELECT constraint_definition
      FROM information_schema.check_constraints
      WHERE constraint_name LIKE '%scores_status%' CONTAINS 'SUPERSEDED')
```

### DC-001-U1 — Pre-INSERT input rejection, 4 cases (T-005)

```
PASS conditions (each case independent):
  Case A: InsertScore(evaluation_item_id="")
            → structured error; SELECT COUNT(*) FROM scores = 0
  Case B: InsertScore(evaluation_item_id=<65-char string>)
            → structured error; SELECT COUNT(*) FROM scores = 0
  Case C: InsertScore(score_value=NaN or missing)
            → structured error; SELECT COUNT(*) FROM scores = 0
  Case D: InsertScore(evidence_id="not-a-uuid")
            → structured error; SELECT COUNT(*) FROM scores = 0
  All 4: SELECT COUNT(*) FROM audit_logs = 0 (no partial audit)
```

### DC-001-O1 — metadata JSONB opaque semantic round-trip (T-006)

```
PASS conditions:
  1. InsertScore with metadata={"채점_코멘트":"현장 점검 반영","draft_threshold":{"S":"90"}}
       → SELECT metadata FROM scores WHERE id=$id → unmarshal to map[string]any
       → reflect.DeepEqual(original_map, unmarshalled_map) = true
  2. Test uses semantic equality (reflect.DeepEqual or jsonEqual())
     NOT byte-level string comparison (JSONB normalizes key order)
  3. metadata={} accepted without error
  4. Deeply nested metadata accepted without rejection
  5. Korean Unicode keys preserved through round-trip
```

### DC-002-E1 — SumWeightedByEvaluationItem DECIMAL exactness (T-010)

```
PASS conditions:
  1. 3 score rows: (90.00, weight=0.5000), (80.00, weight=0.3000),
                   (70.00, weight=0.2000) for evaluation_item_id='AX-SAFETY-ORG-01'
  2. SumWeightedByEvaluationItem('AX-SAFETY-ORG-01') = exactly 83.00
     (DECIMAL type, not float64; assert via string comparison "83.00"
      or pgtype.Numeric.Float64() == 83.0 with epsilon=0)
  3. EXPLAIN (FORMAT TEXT) output for underlying query
     contains 'Index Scan' on scores_evaluation_item_id_idx
  4. p99 < 50ms over 10 iterations (testcontainers timing loop)
```

### DC-002-U1 — NULL weight deterministic policy (T-010)

```
PASS conditions:
  1. 2 score rows: (90.00, weight=0.6000), (80.00, weight=NULL)
     for evaluation_item_id='AX-SAFETY-ORG-02'
  2. SumWeightedByEvaluationItem('AX-SAFETY-ORG-02') called twice
       → same result both calls (deterministic)
  3. Result is NOT the value produced by treating NULL weight as 0
     (i.e., result != 54.00 if policy is exclude;
      OR result = structured error if policy is error)
  [Gap-01 resolution: chosen policy MUST be documented as
   // NULL-weight policy: [exclude|error] comment in helper
   BEFORE sprint concludes; this DC asserts the chosen policy is consistent]
```

### DC-003-E1 — DetermineGrade happy path (T-011)

```
PASS conditions:
  grade_thresholds rows: (default,S,90.00,gte),(default,A,80.00,gte),
    (default,B,70.00,gte),(default,C,60.00,gte),(default,D,0.00,gte)
  1. DetermineGrade(83.00, 'default') = 'A'
  2. No external service call (verified by static egress check + no HTTP client)
  3. Scan order: S→D descending by min_value;
     first match (score >= min_value) determines grade
```

### DC-003-S1 — Unknown scope → structured error, no fabricated grade (T-011)

```
PASS conditions:
  1. DetermineGrade(83.00, 'unknown-scope') with 0 grade_thresholds rows
       for scope='unknown-scope'
       → non-nil error returned; error message contains "grade thresholds"
         or "unavailable"
  2. No grade letter returned (return value = "" or zero value)
  3. audit_logs unchanged (no audit row for failed grade lookup)
```

### DC-003-U1 — Boundary determinism, gte rule (T-011)

```
PASS conditions:
  grade_thresholds: (default,A,80.00,gte) among others
  1. DetermineGrade(80.00, 'default') = 'A'
     (80.00 >= 80.00 = true, gte boundary inclusive)
  2. Called twice → 'A' both times (deterministic, no randomness)
  3. DetermineGrade(79.99, 'default') != 'A'
     (79.99 < 80.00 = false, falls to next letter)
```

### DC-004-E1 — Audit resource_id = score.id direct, no surrogate (T-007, T-008)

```
PASS conditions:
  1. grep -n "NewSHA1" \
       apps/control-plane/internal/audit/recorder.go
       → 0 matches for lines added by SPEC-AX-SCORE-001
       (existing AUD-1 lines for EvalItem are pre-existing; count of NEW lines = 0)
  2. grep -n "ScoreAuditNamespace\|ScoreNamespace\|NamespaceScore" \
       apps/control-plane/internal/audit/audit.go = 0 matches
  3. SELECT resource_id FROM audit_logs WHERE resource_id=$score_uuid
       = $score_uuid (byte-identical UUID, no transformation)
  4. SELECT resource_type FROM audit_logs WHERE resource_id=$score_uuid
       = 'score'
  5. SELECT details->>'evaluation_item_id' FROM audit_logs
       WHERE resource_id=$score_uuid = 'AX-SAFETY-ORG-01-1'
       (DetailsJSON contains business context)
  6. resource_id != uuid.Nil (non-zero UUID)
```

### DC-004-U1 — Audit fault → bidirectional rollback, no goroutine leak (T-009)

```
PASS conditions:
  Precondition: fault-injected audit INSERT failure
  1. SELECT COUNT(*) FROM scores = 0 (score row rolled back)
  2. SELECT COUNT(*) FROM audit_logs = 0 (no partial audit)
  3. Non-nil error returned from the composite operation
  4. Error wraps the audit-insertion failure (errors.Is or error message check)
  5. goleak.VerifyNone(t) passes (0 goroutine leaks)
```

### DC-BOUNDARY — Upstream migration files unmodified, FK-absence (T-014)

```
PASS conditions:
  1. git diff HEAD -- .moai/db/schema/migrations/0002_evidence_tables.sql
       = empty output (0 bytes diff)
  2. git diff HEAD -- .moai/db/schema/migrations/0003_eval_item_tables.sql
       = empty output (0 bytes diff)
  3. SELECT COUNT(*) FROM information_schema.table_constraints tc
       JOIN information_schema.constraint_column_usage ccu USING (constraint_name)
       WHERE tc.table_name='scores' AND tc.constraint_type='FOREIGN KEY' = 0
  4. SELECT data_type FROM information_schema.columns
       WHERE table_name='evaluation_items' AND column_name='id'
       = 'character varying'
     AND data_type matches scores.evaluation_item_id = 'character varying'
  5. SELECT data_type FROM information_schema.columns
       WHERE table_name='evidences' AND column_name='id'
       = 'uuid'
     AND data_type matches scores.evidence_id = 'uuid'
```

---

## 2. Edge Cases

The 16 from acceptance.md §7, plus 3 additional from evaluator scrutiny.

### From acceptance.md §7

**EC-01** Create+audit atomic commit: InsertScore TX commits only when both score row and audit row are written successfully; partial commit of either alone is not observable.

**EC-02** Audit INSERT fault → bidirectional rollback + no goroutine leak: See DC-004-U1. Both tables revert; `goleak.VerifyNone(t)` passes.

**EC-03** evaluation_item_id VARCHAR(64) FK-less stub: InsertScore with evaluation_item_id referencing non-existent evaluation_items row → no FK error; success.

**EC-04** evidence_id UUID FK-less stub: InsertScore with evidence_id referencing non-existent evidences row → no FK error; success.

**EC-05** Pre-INSERT input rejection: All 4 cases (blank eval_item_id, 65-char eval_item_id, missing score_value, non-UUID evidence_id) → structured error, 0 rows in scores, 0 rows in audit_logs.

**EC-06** metadata JSONB opaque semantic round-trip: See DC-001-O1. Korean keys, nested objects, empty `{}` all preserved via semantic equality, NOT byte comparison.

**EC-07** Single-level weighted sum DECIMAL exactness: See DC-002-E1. 83.00 exact.

**EC-08** NULL weight deterministic policy: See DC-002-U1. Consistent behavior, NOT silent coerce to 0.

**EC-09** grade_thresholds boundary gte determinism: See DC-003-U1. value=80.00, gte rule → 'A' twice.

**EC-10** Unknown scope → structured error, no fabricated grade: See DC-003-S1.

**EC-11** Status DRAFT mutable: UpdateScore on DRAFT row → updated in-place.

**EC-12** Status CONFIRMED immutable: UpdateScore on CONFIRMED row (score fields) → structured error, row unchanged.

**EC-13** CONFIRMED correction = new row + SUPERSEDED same TX: 2 rows after correction, original=SUPERSEDED, new=CONFIRMED, both in same TX.

**EC-14** No physical DELETE: grep -n "DELETE FROM scores" in all .go files = 0 matches.

**EC-15** Store→audit circular-import avoidance: recorder.go uses local AuditTx interface (not store package import).

**EC-16** cli-anonymous literal: See DC-UBI-003. Exact 'cli-anonymous' in both scores.created_by and audit_logs.user_id.

### Additional (Evaluator Scrutiny)

**EC-ADD-1** CONFIRMED correction atomicity failure: If SUPERSEDED update succeeds but new row INSERT fails (or vice versa), the entire TX rolls back. After failure: `SELECT COUNT(*) FROM scores WHERE evaluation_item_id=$eid = 1` (original row, still CONFIRMED). `SELECT COUNT(*) FROM audit_logs WHERE resource_type='score'` unchanged.

**EC-ADD-2** UpdateScore dynamic SQL identifier injection prevention: UpdateScore implementation must NOT construct column names from user-supplied strings. All SET clauses must use static field mapping. Verify: no `fmt.Sprintf("UPDATE scores SET %s", userInput)` pattern anywhere in score-related code. Static grep check: `grep -n "Sprintf.*UPDATE" apps/control-plane/internal/store/` = 0 suspicious matches.

**EC-ADD-3** Circular import prevention (static grep): `grep -rn '"github.com/ircp/iroum-ax/apps/control-plane/internal/store"' apps/control-plane/internal/audit/` = 0 matches. This is a static build-time invariant, not just a runtime check.

---

## 3. Hard Thresholds

All are binary (pass/fail) and grep-checkable or test-output verifiable.

**TH-01** Test coverage ≥ 85% for new score code:
```
go test -coverprofile=coverage.out ./apps/control-plane/internal/store/...
  ./apps/control-plane/internal/audit/...
go tool cover -func=coverage.out | grep -E "score\.go|scorer|grade"
→ each new function ≥ 85% line coverage; total new-file coverage ≥ 85%
```

**TH-02** golangci-lint + gosec 0 issues:
```
golangci-lint run --enable=gosec ./apps/control-plane/internal/store/...
  ./apps/control-plane/internal/audit/...
→ exit code 0, 0 issues reported on changed files
```

**TH-03** goleak zero in all test functions:
```
Every test function in *_test.go added by SPEC-AX-SCORE-001 calls
  defer goleak.VerifyNone(t) or goleak.VerifyNone(t) at end
→ 0 goroutine leaks reported in any test run
```

**TH-04** Zero external network egress (static + runtime):
```
Static: grep -rn '"net/http"\|"google.golang.org\|"github.com/aws"' \
    apps/control-plane/internal/store/ \
    apps/control-plane/internal/audit/ = 0 new imports
Runtime: all score tests pass with external network blocked (testcontainers default)
```

**TH-05** LSP (gopls) 0 errors post-implementation:
```
gopls check ./apps/control-plane/... → 0 errors, 0 type errors
```

**TH-06** initial.sql unmodified:
```
git diff HEAD -- .moai/db/schema/initial.sql → empty output
```

**TH-07** Migration 0002 unmodified:
```
git diff HEAD -- .moai/db/schema/migrations/0002_evidence_tables.sql → empty output
```

**TH-08** Migration 0003 unmodified:
```
git diff HEAD -- .moai/db/schema/migrations/0003_eval_item_tables.sql → empty output
```

**TH-09** postgres.go untouched:
```
git diff HEAD -- apps/control-plane/internal/store/postgres.go → empty output
```

**TH-10** No new external module dependencies:
```
go list -m all (before) vs go list -m all (after) → no new lines added
go.mod and go.sum: only 0004 migration content allowed in new files;
  no new `require` entries in go.mod
```

**TH-11** [HARD D2] Zero new namespace constants in audit.go:
```
grep -n "ScoreAuditNamespace\|ScoreNamespace\|NamespaceScore\|scoreNamespace" \
    apps/control-plane/internal/audit/audit.go = 0 matches
```

**TH-12** [HARD D2] Zero new uuid.NewSHA1 calls in recorder.go for score code:
```
git diff HEAD -- apps/control-plane/internal/audit/recorder.go \
  | grep "^+" | grep "NewSHA1" = 0 lines
(Pre-existing AUD-1 lines are unchanged; only new additions counted)
```

---

## 4. Security Review

OWASP-relevant for a scoring store in a public-sector evaluation context.

**SEC-01** SQL injection — parameterized queries only:
All pgx queries (`InsertScore`, `UpdateScore`, `GetScoreByID`, `GetScoresByEvaluationItem`, `SumWeightedByEvaluationItem`, grade_thresholds lookup) MUST use `$N` placeholder syntax exclusively. Verify: `grep -n "Sprintf\|fmt\.Sprintf" apps/control-plane/internal/store/score*.go` = 0 matches involving SQL strings.

**SEC-02** Dynamic SQL identifier injection in UpdateScore SET:
`UpdateScore` must use a static column-to-parameter mapping. No dynamic construction of column names from user input. Verify: grep for `Sprintf.*SET\|string.*column` pattern in score update code = 0 suspicious matches. The SET clause must enumerate only statically known fields (score_value, weight, grade, status, metadata, updated_at).

**SEC-03** DECIMAL precision — no float64 intermediate:
`SumWeightedByEvaluationItem` must not use `float64` for score or weight values. pgx must scan `DECIMAL` columns as `pgtype.Numeric` or `string` and compute in database or via exact arithmetic. Verify: no `float64` variable holds score_value or weight in the aggregation path. `0.1 + 0.2 != 0.3` in float64; public-sector scoring requires DECIMAL exactness.

**SEC-04** Grade fail-closed — no fabricated grade on missing thresholds:
When `grade_thresholds` has 0 rows for the requested scope, `DetermineGrade` returns a structured error and no grade letter. A default fallback grade (e.g., 'D') is PROHIBITED. Fabricated grades in 경영평가 = audit finding. Verify via DC-003-S1.

**SEC-05** audit_logs INSERT-only enforcement:
Score domain code must contain 0 UPDATE or DELETE statements targeting `audit_logs`. Verify: `grep -n "UPDATE audit_logs\|DELETE.*audit_logs" apps/control-plane/internal/store/ apps/control-plane/internal/audit/` = 0 new matches added by this SPEC.

**SEC-06** cli-anonymous no real-user leak:
When `AUTH_ENABLED=false`, `created_by` in scores and `user_id` in audit_logs MUST be exactly 'cli-anonymous'. No code path may produce a NULL, empty string, or caller-supplied arbitrary value in these fields. Verify: `resolveUserID("")` = "cli-anonymous"; no parameter in `InsertScore` signature accepts a user ID override that could bypass this.

**SEC-07** Zero external egress — grade computation is internal:
No HTTP client, no LLM SDK, no external secrets manager, no gRPC stub targeting external endpoints imported in `internal/store` or `internal/audit`. Grade computation uses only the internal `grade_thresholds` table. Verify via TH-04 + DC-UBI-001.

---

## 5. Gaps Flagged

Issues in tasks.md or acceptance.md requiring resolution before or during implementation.

**GAP-01** [HIGH, T-010]: NULL weight policy not pre-committed. `tasks.md` T-010 defers the NULL weight policy to "S4 구현 고정" without specifying whether NULL weight means exclude the row from the sum or return a structured error. `AC-SCORE-002-2` does not commit to a specific outcome. **Required resolution**: the implementation MUST document the chosen policy as `// NULL-weight policy: [exclude|error]` comment in the `computeWeightedSum` helper before the sprint concludes. `DC-002-U1` asserts consistency of the chosen policy; the tester must assert the specific behavior, not just "consistent".

**GAP-02** [MEDIUM, AC-SCORE-UBI-004(b)]: Two-audit-row assertion for CONFIRMED correction implicit. `acceptance.md` AC-SCORE-UBI-004 states "각 변경 1 audit row" but no AC explicitly verifies that a CONFIRMED correction produces exactly 2 audit rows (one SCORE_UPDATED for old-CONFIRMED→SUPERSEDED, one SCORE_CREATED for new row). This contract adds `DC-UBI-004` condition 4 to close this gap. The tester MUST include this 2-row assertion.

**GAP-03** [LOW, T-004/T-010]: Transaction isolation level for read-only aggregation unspecified. T-004 specifies `ReadCommitted` for write TX (mirroring `BeginEvalItemTx`), but `SumWeightedByEvaluationItem` read path isolation is not stated. **Required resolution**: `SumWeightedByEvaluationItem` must use a transaction (not a raw pool query via `pool.QueryRow`) for consistent snapshot read. `ReadCommitted` or `ReadOnly` are both acceptable; choose and document.

**GAP-04** [MEDIUM, AC-SCORE-001-S1]: scores.id column type not verified in acceptance tests. `acceptance.md` AC-SCORE-001-S1 checks `evaluation_item_id` and `evidence_id` types but omits `id`. This contract adds the `information_schema` check for `scores.id = uuid` to DC-001-S1. The tester must include this assertion.

**GAP-05** [HIGH, AC-SCORE-004-1]: Circular import avoidance has no concrete static test. `acceptance.md` AC-SCORE-004-1 states "store→audit 순환 의존 없음" but specifies no grep-based test. Runtime compilation checks are insufficient for CI enforcement. **Required resolution**: the tester's integration test suite must include a `TestCircularImportAbsence` or equivalent that runs `grep -rn "internal/store" apps/control-plane/internal/audit/` and asserts 0 matches. This is EC-ADD-3 in the edge cases section.

**GAP-06** [MEDIUM, T-015]: @MX:ANCHOR additive-only not verified by any task. The brownfield constraint states that existing `@MX:ANCHOR` tags in `store.go` (EvalItemStore, EvidenceStore), `recorder.go:28/37`, and `pg_store.go:24` must not be removed or modified. No task or test verifies this. **Required resolution**: T-015 (or a dedicated test) must include a grep-based check that all pre-existing `@MX:ANCHOR` tags remain in their original form after score code is added. Example: `grep -c "@MX:ANCHOR" apps/control-plane/internal/store/store.go ≥ 2`.

---

## Summary Counts

| Section | Count |
|---|---|
| Done criteria | 17 (DC-UBI-001 through DC-BOUNDARY) |
| Edge cases | 19 (16 from acceptance.md §7 + EC-ADD-1, EC-ADD-2, EC-ADD-3) |
| Hard thresholds | 12 (TH-01 through TH-12) |
| Security concerns | 7 (SEC-01 through SEC-07) |
| Gaps flagged | 6 (GAP-01 through GAP-06) |

---

**Binding as of Phase 2.0. Implementation starts after this contract is acknowledged.**
**Maximum negotiation rounds remaining: 1.**
