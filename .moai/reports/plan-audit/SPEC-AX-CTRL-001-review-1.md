# SPEC Review Report: SPEC-AX-CTRL-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.72

## Context Isolation Statement

Reasoning context provided in the orchestrator prompt was IGNORED per M1 Context Isolation. This audit is formed independently from spec.md, plan.md, acceptance.md, research.md, spec-compact.md, and project documentation files referenced as cross-references.

---

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency**:
  - Primary REQ IDs `REQ-CTRL-001` ~ `REQ-CTRL-005` are sequential (spec.md:L108, L136, L158, L172, L188), no gaps, no duplicates, consistent zero-padding (3 digits).
  - Transverse Ubiquitous family `REQ-CTRL-UBI-001` / `REQ-CTRL-UBI-002` (spec.md:L105-L106) is sequential within its own family, mirroring the SPEC-AX-001 `REQ-UBI-001..003` pattern. Acceptable per established project convention (SPEC-AX-001 §3.1 review-2 PASS precedent).
  - Sub-IDs within each REQ (E1/E2/S1/U1/U2/O1) are consistent.

- [PASS] **MP-2 EARS format compliance**:
  - Ubiquitous: REQ-CTRL-UBI-001 ("The control plane SHALL persist...", spec.md:L105), REQ-CTRL-UBI-002 ("...SHALL write...", spec.md:L106), REQ-CTRL-001-U2 (spec.md:L128).
  - Event-driven: REQ-CTRL-001-E1/E2, REQ-CTRL-002-E1/E2, REQ-CTRL-003-E1/E2, REQ-CTRL-004-E1, REQ-CTRL-005-E1 — all use "WHEN ... THEN ... SHALL" pattern.
  - State-driven: REQ-CTRL-001-S1 (spec.md:L124), REQ-CTRL-002-S1 (spec.md:L146), REQ-CTRL-004-S1 (spec.md:L180), REQ-CTRL-005-S1 (spec.md:L198) — all use "WHILE ... SHALL".
  - Optional: REQ-CTRL-002-O1 (spec.md:L150) — "WHERE ... THE control plane SHALL ...".
  - Unwanted: REQ-CTRL-001-U1, REQ-CTRL-002-U1, REQ-CTRL-003-U1, REQ-CTRL-004-U1, REQ-CTRL-005-U1 — all use "IF ... THEN ... SHALL".
  - All 5 EARS types present and correctly applied.

- [PASS] **MP-3 YAML frontmatter validity**:
  - spec.md:L1-L10 contains all 8 fields of the canonical schema declared at `.claude/skills/moai/workflows/plan.md` Phase 2 (referenced via spec.md:L16 schema note):
    - `id: SPEC-AX-CTRL-001` (string, matches SPEC-{DOMAIN}-{NUM} pattern)
    - `version: 0.1.0` (string)
    - `status: draft` (string, valid enum value)
    - `created: 2026-05-14` (ISO date)
    - `updated: 2026-05-14` (ISO date)
    - `author: ircp` (string)
    - `priority: high` (string, valid enum value)
    - `issue_number: 0` (integer)
  - The `labels` / `created_at` rename concern raised in the audit framework is not applicable; project canonical schema (8-field, established in SPEC-AX-001 iteration 2) is correctly followed. No false positive flagged.

- [N/A] **MP-4 Section 22 language neutrality**: Single-language (Go) project scope. The SPEC targets only the Go control plane (`apps/control-plane/`) and explicitly references Go-specific tooling (pgx/v5, go-redis/v9, zap, testcontainers-go, miniredis, bufconn, goleak) which is correct for the implementation language. No multi-language enumeration required. Auto-PASS.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75 band (minor ambiguity in one or two requirements) | Most requirements unambiguous; REQ-CTRL-004-E1 (spec.md:L176) hardcodes pool config (max_open_connections=25, conn_max_lifetime=1h) which is borderline between requirement and implementation detail. REQ-CTRL-001-U2 categorization (Ubiquitous label within REQ-CTRL-001 modal block) is slightly confusing but parseable. |
| Completeness | 0.88 | 0.75-1.0 band | All required sections present (HISTORY spec.md:L12-L16, WHY/Overview spec.md:L22-L44, WHAT/Affected Files spec.md:L48-L97, REQUIREMENTS spec.md:L101-L202, NFR spec.md:L206-L220, EXCLUSIONS spec.md:L224-L240 with 13 entries, DEPENDENCIES spec.md:L244-L251). Frontmatter complete. Single deduction: REQ-CTRL-UBI-001/002 are stated as transverse invariants but their full implications (e.g., what counts as "same transaction") are mostly explicit. |
| Testability | 0.85 | 0.75 band (one or two non-precisely-binary ACs) | All ACs in acceptance.md use quantified G/W/T format with explicit numbers (50ms, 100ms, 5ms, 10ms p99). No weasel words ("적절", "신속" explicitly forbidden per acceptance.md:L7). Deduction: REQ-CTRL-005-E1 declares "100ms p99" for dispatch but no AC explicitly measures dispatch latency (AC-CTRL-005-1 is a golden file byte-compare, not a benchmark — see Defect D5). Performance summary table §7 (acceptance.md:L307-L316) ties §1-§5 ACs to performance targets but the dispatch p99 target is not isolated in any single test. |
| Traceability | 0.55 | 0.50 band (multiple REQs lack ACs) | **Major gap**: REQ-CTRL-UBI-001 and REQ-CTRL-UBI-002 (spec.md:L105-L106) have NO dedicated acceptance criteria in acceptance.md §1-§9. The audit pattern from SPEC-AX-001 iteration 2 mandates dedicated AC-UBI-* entries (see SPEC-AX-001 acceptance.md AC-UBI-001~004). 2 of 7 stated REQs uncovered = ~28% gap. Sub-defects: REQ-CTRL-002-O1 (Optional Prometheus) and REQ-CTRL-003-E1 (gateway startup time), REQ-CTRL-004-U1 (mid-tx error rollback) lack dedicated AC entries (partial implicit coverage only). |

Aggregate Overall Score (weighted equally): 0.78. Adjusted down to **0.72** because the Traceability gap is structural and matches a pattern that caused SPEC-AX-001 iteration 1 to fail and require iteration 2 (which added explicit AC-UBI entries). Under thorough harness, this regression of an already-corrected pattern warrants FAIL.

---

## Defects Found

### Major (Must Resolve Before Run Phase)

- **D1 (Major) — Traceability gap: REQ-CTRL-UBI-001 lacks dedicated AC**
  - Location: spec.md:L105 declares `REQ-CTRL-UBI-001 (상태 불변)`: "The control plane SHALL persist every workflow state transition into the `workflows` table within the same transaction as the corresponding business-side change."
  - acceptance.md has no `AC-CTRL-UBI-001` (or equivalent) entry verifying that workflow state transitions and business-side audit writes are in the same transaction.
  - While AC-CTRL-001-1 (acceptance.md:L13) implicitly checks audit_logs INSERT after workflow INSERT, it does not explicitly verify the *transactional atomicity* invariant (e.g., audit + workflow row visible/invisible together under tx rollback).
  - This is the identical pattern that caused SPEC-AX-001 iteration 1 review (D4/D5 reconcile + REQ-UBI dedicated ACs added in iteration 2). Inheriting this defect after the precedent is regression of project quality discipline.
  - Severity: Major (Traceability rule AC-5 violated).

- **D2 (Major) — Traceability gap: REQ-CTRL-UBI-002 lacks dedicated AC**
  - Location: spec.md:L106 declares `REQ-CTRL-UBI-002 (감사 일관성)`: "The control plane SHALL write an `audit_logs` entry for every workflow creation, state transition, and dispatch event ... user_id 기본값은 `cli-anonymous`."
  - No dedicated AC in acceptance.md asserts the *completeness* of audit event emission across all 8 Go-side action types enumerated in research.md:L83-L94 (WORKFLOW_CREATED, WORKFLOW_TRANSITIONED_TO_RUNNING, WORKFLOW_COMPLETED, WORKFLOW_FAILED_DISPATCH, WORKFLOW_FAILED_CALLBACK, TRANSITION_REJECTED, CALLBACK_REJECTED_TERMINAL, WORKFLOW_CREATE_CANCELLED).
  - Also missing: no AC explicitly verifies `user_id='cli-anonymous'` default for the Go-side INSERT path (parallel to SPEC-AX-001's AC-UBI-004 which exists for the Python path).
  - Severity: Major (Traceability rule AC-5 violated, parallel to D1).

### Minor

- **D3 (Minor) — Broken citation in spec.md:L44**
  - spec.md:L44 states: "따라서 SPEC ID: `SPEC-AX-CTRL-001` (2 domains, plan.md L367 3-domain 한계 내)".
  - plan.md:L367 actually contains: "- [ ] plan-auditor 1차 audit 통과 (본 plan + spec.md + acceptance.md + research.md 정합성)" — a Definition of Done item with no 3-domain limit.
  - No 3-domain composite-SPEC limit exists in plan.md or the project skills directory (`.claude/skills/moai/workflows/plan.md` — grep confirmed no such rule).
  - The 2-domain composite ID `SPEC-AX-CTRL-001` itself is fine in practice (the SPEC is already approved for use), but the cited rationale points to a non-existent rule.
  - Fix: remove the bogus citation or replace with an actual reference to the canonical naming guide.

- **D4 (Minor) — AC heading inconsistency: AC-CTRL-001-5**
  - acceptance.md:L72 uses heading `### Edge Case AC-CTRL-001-5` whereas siblings (acceptance.md:L13, L30, L43, L57) use `### AC-CTRL-001-{N}` with no "Edge Case" prefix.
  - Other "edge case" ACs (AC-CTRL-002-3, AC-CTRL-005-3 etc.) do not carry the prefix. The inconsistency makes mechanical AC enumeration error-prone but the AC ID itself is parseable.

- **D5 (Minor) — Performance target lacks dedicated AC: Celery dispatch p99 < 100ms**
  - spec.md:L213 (NFR §4) declares "Celery dispatch p99 < 100ms (PENDING→RUNNING 전이 + RPUSH ack)".
  - acceptance.md:L312 (§7) maps this target to "AC-CTRL-005-1", but AC-CTRL-005-1 (acceptance.md:L235) is a golden-file byte-comparison test that does not measure latency.
  - No standalone AC validates the 100ms p99 dispatch latency on a miniredis-backed benchmark. The benchmark mention in §7 is asserted only at the summary level.
  - Fix: add `AC-CTRL-005-4 (Dispatch Latency p99)` or expand AC-CTRL-005-1 to include a timing assertion.

- **D6 (Minor) — REQ-CTRL-002-O1 (Prometheus Optional) lacks AC**
  - spec.md:L150 declares Optional behavior gated on `PROMETHEUS_ENABLED=true`. While Optional requirements can be deferred, this one is observable when enabled and ought to have a "Where enabled" AC (or be explicitly marked as deferred to a future SPEC).
  - acceptance.md has no AC for the Prometheus metrics path.

- **D7 (Minor) — REQ-CTRL-003-E1 (REST gateway startup <2s) lacks AC**
  - spec.md:L162 declares the REST gateway "SHALL become ready within 2 seconds of process start".
  - AC-CTRL-002-1 (acceptance.md:L91-L102) tests the gRPC server startup with a 2s budget, but no parallel AC verifies the REST gateway startup time. This is a parallel testable obligation that should be explicit.

- **D8 (Minor) — REQ-CTRL-004-U1 (mid-tx PostgreSQL failure rollback) lacks AC**
  - spec.md:L184 declares behavior on mid-tx query failure (rollback + log full SQL state + return gRPC INTERNAL + no inconsistent state).
  - No dedicated AC simulates a PostgreSQL mid-transaction failure (e.g., via fault injection in testcontainers). AC-CTRL-001-5 covers ctx cancellation rollback but not server-side query failure.

### Informational

- **D9 (Info) — REQ-CTRL-004-E1 hardcodes operational config in requirement text**
  - spec.md:L176 specifies `max_open_connections=25, max_idle_connections=5, conn_max_lifetime=1h` inside the SHALL clause. These are tuning parameters; consider moving to a config table and keeping the requirement abstract ("SHALL initialize a pgx/v5 connection pool from configuration").
  - Not a blocking defect — Walking Skeleton scope justifies concrete values, and AC-CTRL-004-3 (pool exhaustion at `max_open_connections=2`) demonstrates the parameter is testable.

- **D10 (Info) — Total AC count discrepancy**
  - acceptance.md:L352 declares "Total AC count: 18 (§1: 5, §2: 3, §3: 3+1variant, §4: 3, §5: 3, §6: 1)".
  - Counting the AC-CTRL-003-3b variant as a separate scenario gives 19. Counting strictly by heading gives 18. Internal accounting is consistent but the "3+1variant" notation is ambiguous.

---

## Chain-of-Verification Pass

Second-look findings:

Re-read sections to confirm I did not skim:
- spec.md §3 (Requirements): re-read each REQ-CTRL block. Confirmed every requirement uses an EARS pattern. Found that REQ-CTRL-001-U2 is marked as "Ubiquitous (Ubiquitous invariant)" inside the REQ-CTRL-001 modal section — categorization unusual but the EARS form is correct.
- acceptance.md §1-§6: re-read each AC entry. Confirmed only 18 explicit ACs are present, none labelled AC-CTRL-UBI-*.
- research.md §2.2: confirms the 8 Go-side audit actions are explicitly enumerated — yet acceptance.md never validates this enumeration end-to-end. Reinforces D2.
- research.md §3.4: confirms the Python settings.py prerequisite (Handoff Note) but the prerequisite is also visible in spec.md §6 L249 — properly cross-linked.
- plan.md §1 sub-sprint definitions: all 5 sub-sprints reference correct REQs and the implementation DAG (plan.md §2) is acyclic.
- plan.md §3 Go module versioning: pinned versions are explicit, "Go 1.22 baseline" trade-off vs 1.23 recommendation in `.claude/rules/moai/languages/go.md` is acknowledged at plan.md:L220.
- plan.md §5 Risk register: 5 risks each with Likelihood/Impact/Mitigation/Residual. Coverage adequate.

New defect discovered in CoV pass:
- **D6, D7, D8 added** during CoV: previously I only counted REQ-CTRL-UBI gaps. On second pass I verified each EARS sub-id and found REQ-CTRL-002-O1, REQ-CTRL-003-E1 (gateway startup), REQ-CTRL-004-U1 (mid-tx error) also lack dedicated ACs. These are minor but the cumulative AC coverage gap matters.

No contradictions found between requirements within the SPEC; no contradictions between Exclusions and stated requirements.

---

## Regression Check

Not applicable (iteration 1).

Note: the orchestrator prompt indicates SPEC-AX-001 review-1 surfaced a similar REQ-UBI coverage defect that was resolved in iteration 2 by adding AC-UBI-001~004. The same pattern is being repeated here — D1/D2 SHOULD HAVE BEEN PRE-EMPTED based on the SPEC-AX-001 precedent. This is not a regression of *this* SPEC's iteration, but it is a process-level signal that the precedent was not fully internalized.

---

## Recommendation

To pass iteration 2, manager-spec must address the following in priority order:

1. **(Resolve D1, D2 — Major)** Add dedicated AC entries in acceptance.md for the two Ubiquitous transverse invariants:
   - `AC-CTRL-UBI-001 (Transactional Atomicity)`: Given a Go workflow handler within an active pgx transaction; When the state transition UPDATE succeeds but the audit_logs INSERT fails (or vice-versa, via injected fault); Then `tx.Rollback(ctx)` runs and neither row is visible after rollback. Reference SPEC-AX-001 AC-UBI-003 structure.
   - `AC-CTRL-UBI-002 (Audit Event Completeness for Go path)`: Given a full workflow lifecycle (CREATE → DISPATCH → CALLBACK COMPLETED); When the audit_logs table is queried by resource_id; Then exactly the 3 expected actions (WORKFLOW_CREATED, WORKFLOW_TRANSITIONED_TO_RUNNING, WORKFLOW_COMPLETED) are present, with `user_id='cli-anonymous'` (no NULL), and resource_type='workflow'.
   - Optionally add `AC-CTRL-UBI-003 (cli-anonymous default for Go path)`: parallel to SPEC-AX-001 AC-UBI-004 but verified against Go `store.InsertAuditLog`.

2. **(Resolve D3 — Minor)** Fix the broken citation at spec.md:L44. Either:
   - Remove "plan.md L367 3-domain 한계 내" entirely and just state "2-domain composite SPEC-ID per project naming convention"; or
   - Replace with a valid citation to wherever the project's composite-SPEC-ID convention is defined (and document that convention if it does not yet exist).

3. **(Resolve D4 — Minor)** Rename acceptance.md:L72 heading to `### AC-CTRL-001-5` (drop "Edge Case " prefix) for consistency with siblings.

4. **(Resolve D5 — Minor)** Either add a dedicated `AC-CTRL-005-4 (Dispatch Latency p99)` benchmark, or extend AC-CTRL-005-1's "Then" block with an `assert.Less(t, p99, 100*time.Millisecond)` over miniredis-backed benchmark runs (10 concurrent dispatches × 1000 iterations).

5. **(Resolve D6 — Minor)** Either add `AC-CTRL-002-4 (Prometheus Optional)` that runs only when `PROMETHEUS_ENABLED=true`, or explicitly mark REQ-CTRL-002-O1 as deferred to a future observability SPEC (and reference that SPEC).

6. **(Resolve D7 — Minor)** Add `AC-CTRL-003-4 (REST Gateway Startup <2s)` parallel to AC-CTRL-002-1.

7. **(Resolve D8 — Minor)** Add `AC-CTRL-004-4 (PostgreSQL Mid-Tx Failure Rollback)`: use a testcontainers-backed integration test that kills the PostgreSQL connection mid-tx (or injects a deadlock) and verifies tx.Rollback succeeds + no partial row visible + audit log captures the SQL state code.

After these fixes, the Traceability score should rise to >= 0.85 and the audit should pass iteration 2.

---

## Schema Decision Verification (D4/D5 Equivalent)

Per the canonical 8-field schema established in SPEC-AX-001 iteration 2 (spec.md:L16-L18 schema note), the SPEC-AX-CTRL-001 frontmatter uses exactly:
- `id, version, status, created, updated, author, priority, issue_number`

This matches the project canonical schema. The previously-flagged `labels` / `created_at` schema variants (rejected in SPEC-AX-001 iteration 2 as D4/D5 REJECT) are NOT applicable to this SPEC. **No false positive raised on frontmatter schema.**

---

Audit complete. Verdict: FAIL — primarily due to D1/D2 Traceability gaps. Defect resolution path is well-defined and matches the SPEC-AX-001 precedent; iteration 2 should be straightforward.
