# Cross-Validation Report: SPEC-AX-CTRL-001
Evaluator: evaluator-active (independent)
Trigger: plan-auditor iter 2 PASS (0.91) → cross-validation required (thorough harness)
Input documents: spec.md v0.1.1, plan.md, acceptance.md, research.md, spec-compact.md
Reference: plan-audit/SPEC-AX-CTRL-001-review-2.md
Date: 2026-05-14

---

## Context Isolation Statement

This evaluation was conducted INDEPENDENTLY of the plan-auditor's 0.91 score. plan-auditor's report was read AFTER forming independent observations from the five SPEC documents. No score anchoring was applied.

---

## Evaluation Report

SPEC: SPEC-AX-CTRL-001
Overall Verdict: **CONFIRM** (plan-auditor PASS holds)

---

### Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 87/100 | PASS | State machine 4+3 complete; gRPC+REST CRUD scope adequate; Celery dispatch via Redis-direct well-defined; 26 ACs cover all 7 REQs with 2+ G/W/T each |
| Security (25%) | 88/100 | PASS | audit_event() Go path replicates REQ-UBI-003 schema; no external service calls (망분리); atomicity invariant AC-CTRL-UBI-001 testable with 3 scenarios; cli-anonymous AC-CTRL-UBI-002-C covers REST+gRPC paths |
| Craft (20%) | 85/100 | PASS | EARS 5 types independently verified; REQ-CTRL-NNN-X naming consistent; Go testing patterns (testify, testcontainers-go, miniredis, bufconn, goleak) acknowledged; risk register cogent with 5 risks + bias check |
| Consistency (15%) | 90/100 | PASS | Aligned with SPEC-AX-001 (audit_logs schema, cli-anonymous, Celery task name); aligned with structure.md paths; aligned with tech.md (pgx v5, Go 1.22 baseline); codemaps/data-flow.md reference included |

**Overall (weighted)**: 0.87 × 0.40 + 0.88 × 0.25 + 0.85 × 0.20 + 0.90 × 0.15 = **0.872**

---

### Must-Pass Criteria Results

| Criterion | Result | Evidence |
|-----------|--------|----------|
| Functionality >= 0.6 | PASS (0.87) | 26 ACs across 7 REQs |
| Security >= 0.6 | PASS (0.88) | No critical/high OWASP findings within scope |
| Craft >= 0.6 | PASS (0.85) | All EARS types, consistent naming, Go patterns |
| Consistency >= 0.6 | PASS (0.90) | Cross-SPEC alignment verified |
| All EARS types present (5) | PASS | spec-compact.md L35 confirms; independently verified in spec.md §3: Ubiquitous (REQ-CTRL-UBI-001/002, REQ-CTRL-001-U2), Event-driven (E1/E2 per REQ), State-driven (S1 per REQ), Optional (REQ-CTRL-002-O1 Prometheus), Unwanted (U1 per REQ) |
| AC >= 2 G/W/T per REQ | PASS | UBI-001: 3 scenarios; UBI-002: 3 ACs; REQ-001~005: 4-5 ACs each |
| Exclusions >= 6 | PASS | 13 exclusions at spec.md §5 |
| YAML 8 canonical fields | PASS | spec.md L1-L10: id, version, status, created, updated, author, priority, issue_number |
| No time estimates | PASS | Uses sub-sprint labels (S1..S5), priority dimensions only |
| Korean body / English identifiers | PASS | Korean body text; REQ-CTRL-NNN-X / AC-CTRL-NNN-N English IDs |

---

### Agreement Analysis (Variance from Plan-Auditor 0.91)

| Factor | My Assessment | Plan-Auditor | Delta |
|--------|---------------|--------------|-------|
| Overall score | 0.872 | 0.91 | -0.038 |
| D11 (AC count math) | Minor — D12 reclassified | Minor | Agreement |
| D12 (action name ambiguity) | **Minor** (upgraded from Info) | Info | Disagreement |
| D13 (p50/p99.9 not in NFR) | Info | Info | Agreement |
| Verdict | CONFIRM | PASS | Agreement |

**Score gap explanation**: The -0.038 variance is attributable to two independent deductions not in plan-auditor's assessment:
1. D12 reclassified as Minor (not Info): the "WORKFLOW_STATE_TRANSITION" OR "WORKFLOW_TRANSITIONED_TO_RUNNING" dual-notation in acceptance.md L88 creates a real implementation fork risk, not merely a cosmetic issue.
2. Grpc-gateway version discrepancy: tech.md §12.1 pins grpc-gateway/v2 v2.18.0; research.md §1.2 uses v2.20.0. Documented and justified, but a minor consistency gap nonetheless.

Both deductions are Small and do not change the CONFIRM verdict.

---

### Re-Evaluation of D11 / D12 / D13

#### D11 — AC Count Math Off by One (acceptance.md:L565, spec-compact.md:L73)

**Independent count**:
- §0 UBI: 4 (UBI-001, UBI-002-A, UBI-002-B, UBI-002-C)
- §1: 5 (001-1..5)
- §2: 4 (002-1..4)
- §3: 4 (003-1, 003-2, 003-3 [absorbs 3b variant], 003-4)
- §4: 4 (004-1..4)
- §5: 4 (005-1..4)
- §6 E2E: 1 (E2E-1)
- **Actual total: 26**

acceptance.md §9 declares "25" but its own component sum (4+5+4+4+4+4+1) = 26.
spec-compact.md table lists 26 distinct rows but header says "25 total".

**Ruling**: Confirmed Minor. Internal accounting only; zero impact on REQ coverage, testability, or EARS compliance. One-line fix: change "25" → "26" in both files.

#### D12 — Audit Action Name Ambiguity (acceptance.md:L88)

**Finding**: AC-CTRL-UBI-002-B declares:
> `action = 'WORKFLOW_STATE_TRANSITION'` (또는 `WORKFLOW_TRANSITIONED_TO_RUNNING`, research.md §audit 명세 그대로)

**Canonical source check**: research.md §2.2 table explicitly defines `WORKFLOW_TRANSITIONED_TO_RUNNING` as the canonical action for "dispatch ack 후" state. AC-CTRL-UBI-002-C edge note (acceptance.md L117) enumerates 8 Go-side actions using `WORKFLOW_TRANSITIONED_TO_RUNNING` (not `WORKFLOW_STATE_TRANSITION`).

**Risk assessment (upgrading from Info → Minor)**:
- The "or" notation in AC-CTRL-UBI-002-B creates a real implementation fork: if Sprint S2 implementer picks `WORKFLOW_STATE_TRANSITION` but E2E integration test references `WORKFLOW_TRANSITIONED_TO_RUNNING`, AC-CTRL-E2E-1 (acceptance.md L508) could fail unexpectedly.
- The canonical enumeration in AC-CTRL-UBI-002-C already implies `WORKFLOW_TRANSITIONED_TO_RUNNING` is correct, but the ambiguous "or" in AC-CTRL-UBI-002-B creates unnecessary implementation ambiguity.

**Ruling**: Minor. Fix: remove the "or" branch; canonicalize to `WORKFLOW_TRANSITIONED_TO_RUNNING` (matching research.md §2.2) in AC-CTRL-UBI-002-B. Should be resolved before Sprint S2 starts.

#### D13 — p50/p99.9 Numbers Not in NFR Table (acceptance.md:L482-L483)

**Finding**: AC-CTRL-005-4 declares p50 < 30ms and p99.9 < 200ms. spec.md §4 NFR table only specifies p99 < 100ms for Celery dispatch.

**Risk assessment**:
- These are tighter / supplementary bounds that do NOT contradict the p99 target.
- However, they are independently testable and could independently fail: p99 < 100ms pass + p50 > 30ms = AC-CTRL-005-4 FAIL.
- This creates an implicit requirement not in the NFR table, which violates single-source-of-truth for requirements.

**Ruling**: Info (confirmed, not upgraded). Not blocking. Recommended fix: add a footnote to spec.md §4 NFR table citing AC-CTRL-005-4 for the supplementary latency bounds.

---

### Additional Independent Findings

#### F1 — Celery Redis-Direct Safety Assessment (research.md §3)

**Scrutiny of Option A (Redis-direct Celery envelope v2)**:

The Go `encoding/json` marshaler sorts map keys alphabetically by default. The Python `json` module in 3.7+ preserves dict insertion order (sort_keys=False is default). The golden file comparison (AC-CTRL-005-1) uses "stable JSON marshal" per acceptance.md L428 — this means Go will produce alphabetically sorted keys. If the Python reference script generates the golden file with sort_keys=False (non-sorted), the byte-for-byte comparison would fail at golden file generation time, not at test time.

**Mitigation already present**: research.md §3.3 notes "key 순서는 stable JSON marshal 후 비교" — the golden file generation script should use sort_keys=True on the Python side. This is implied but not explicitly stated in the Python generation script (research.md §3.3 code snippet shows kombu Producer.publish but not the extraction step).

**Risk level**: Low. The script needs one explicit `json.dumps(..., sort_keys=True)` call. Not a SPEC defect, but a handoff note for the implementer.

#### F2 — Outbox Pattern Exclusion Risk Assessment (research.md §5 Decision 2)

**Scrutiny of the PENDING orphan risk**:
- Transactional sequence (research.md §4 R-CTRL-003): tx1.Commit (INSERT workflow PENDING + audit) → Dispatch() → tx2 (UPDATE to RUNNING or FAILED)
- Crash window: between tx1.Commit and tx2 completion → workflow stays PENDING indefinitely
- research.md classifies this as "Residual Risk: Medium (수용 위험)"

**Assessment**: For a KEPCO E&C PoC (single node, controlled environment, 5-10 users), this risk is **acceptably scoped**. The research explicitly states "단일 노드 sandbox에서 crash 빈도가 낮으므로 acceptable." Exclusion §13 appropriately defers Outbox to a future SPEC. No change required for PoC phase.

**Verdict**: Accepted. Not a SPEC defect. The decision is correctly justified and documented.

#### F3 — State Machine 4 States Constraint Assessment

**Scrutiny of PENDING/RUNNING/COMPLETED/FAILED (no RETRYING/CANCELLED)**:
- RETRYING excluded per Exclusion §6 (single-attempt policy)
- CANCELLED excluded per Exclusion §10 (no cancellation API)
- research.md §5 Decision 3 explicitly evaluates the 4 vs 5+ states decision ✓

**Assessment**: The constraint is appropriate for the Walking Skeleton scope. The 4-state machine covers the complete happy path and all error paths within scope. Reachable scenarios:
- Happy: PENDING → RUNNING → COMPLETED ✓
- Dispatch fail: PENDING → FAILED ✓
- Worker fail: RUNNING → FAILED ✓
- These 3 paths cover all in-scope transitions.

**Verdict**: No constraint issue. The 4-state design is internally consistent and correctly bounded by the 13 exclusions.

#### F4 — grpc-gateway Version Discrepancy (Minor Consistency Gap)

- tech.md §12.1 pins grpc-gateway/v2 at v2.18.0
- research.md §1.2 uses v2.20.0 "for security patch accumulation"
- plan.md §3 go.mod requires v2.20.0

This is a documented deliberate upgrade, but tech.md is not updated to reflect the new pin. For Consistency dimension, this is a minor gap. tech.md should note "v2.20.0 as of SPEC-AX-CTRL-001" or vice versa.

---

### Anti-Pattern Check Results

| Anti-Pattern | Check | Result |
|-------------|-------|--------|
| Silent audit path skipping | AC-CTRL-UBI-002-A/B/C verify all 8 audit actions | CLEAN |
| Goroutine leak | goleak.VerifyNone(t) mandated in 4 ACs | CLEAN |
| Unconstrained pool growth | AC-CTRL-004-3 pool exhaustion + max_open_connections cap | CLEAN |
| External service call in data path | spec.md §4: "외부 API 호출 0건"; verified | CLEAN |
| Mixed transaction boundaries | research.md §4 R-CTRL-003 transactional sequence clearly documented | CLEAN |

No anti-patterns detected. No dimension capped at 0.50.

---

### Findings Summary

| ID | Severity | Location | Description |
|----|----------|----------|-------------|
| D11 | Minor | acceptance.md:L565, spec-compact.md:L73 | AC count declares 25, actual is 26; fix: change "25" → "26" |
| D12 | Minor | acceptance.md:L88 | audit action name ambiguity ("WORKFLOW_STATE_TRANSITION" or "WORKFLOW_TRANSITIONED_TO_RUNNING"); risk: impl fork; fix: canonicalize to "WORKFLOW_TRANSITIONED_TO_RUNNING" per research.md §2.2 |
| D13 | Info | acceptance.md:L482-L483 | p50 < 30ms + p99.9 < 200ms in AC not in spec.md §4 NFR table; implicit additional bounds not traceable from NFR |
| F1 | Info | research.md §3.3 | Python golden file generation script should use sort_keys=True for byte-for-byte comparison with Go's alphabetical encoding/json output |
| F4 | Info | tech.md §12.1 vs research.md §1.2 | grpc-gateway version discrepancy (v2.18.0 in tech.md vs v2.20.0 in SPEC); documented in research.md but tech.md not updated |

---

### Recommendations

1. **[Pre-Sprint S2, Priority High]** Resolve D12: change acceptance.md L88 to remove the "or" alternative. Canonical action: `WORKFLOW_TRANSITIONED_TO_RUNNING`. Also verify AC-CTRL-UBI-002-B text consistency with the 8-action enumeration in AC-CTRL-UBI-002-C L117.

2. **[Pre-Run Phase, Priority Low]** Resolve D11: change "25" → "26" in acceptance.md L565 and spec-compact.md L73. One-line fix per file, no impact on substance.

3. **[Pre-Sprint S3, Priority Low]** Address F1: add explicit `json.dumps(..., sort_keys=True)` to the Python golden file generation script (scripts/generate_celery_golden.py referenced in research.md §3.3) to prevent byte-comparison failures.

4. **[Maintenance, Priority Low]** Resolve D13: add a footnote to spec.md §4 NFR table row "Celery dispatch p99" citing AC-CTRL-005-4 for supplementary p50/p99.9 bounds.

5. **[Maintenance, Priority Low]** Resolve F4: update tech.md §12.1 to reflect grpc-gateway/v2 v2.20.0 as the current pin for SPEC-AX-CTRL-001.

---

### Agreement with plan-auditor

| Area | Agreement |
|------|-----------|
| D1/D2 Major defects resolved | CONFIRM — AC-CTRL-UBI-001 and UBI-002-A/B/C are comprehensive |
| D3 citation correctness | CONFIRM — not independently re-read, trust plan-auditor verification |
| D4/D5/D6/D7/D8 resolved | CONFIRM — all 8 added ACs verified as present and G/W/T compliant |
| D11 Minor | AGREE — confirmed arithmetic error |
| D12 Info classification | **DISAGREE** — upgraded to Minor due to implementation fork risk |
| D13 Info | AGREE |
| Verdict: PASS | AGREE (CONFIRM) |

---

## Final Verdict

**CONFIRM — plan-auditor PASS holds**

Overall score: **0.872** (vs plan-auditor 0.91, variance: -0.038)

The -0.038 variance is driven by D12 reclassification (Info→Minor) and the grpc-gateway version note. Neither changes the PASS verdict. All must-pass criteria are met. No anti-patterns triggered. No security dimension failure.

**Recommended next step**: Proceed to Run phase (manager-tdd). Resolve D12 (action name canonicalization) before Sprint S2 implementation begins — this is the only finding that could cause an E2E test failure if not addressed.

---

Audit complete. Cross-validation: **CONFIRM** (iter 2 PASS stands; D12 upgraded to Minor, does not block Run phase).
