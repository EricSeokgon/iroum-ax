# SPEC-AX-001 Cross-Validation — evaluator-active

- Evaluator: evaluator-active (adversarial, independent)
- Harness: thorough
- SPEC version: 0.1.1
- Date: 2026-05-14
- Plan-auditor reference: `.moai/reports/plan-audit/SPEC-AX-001-review-2.md` (PASS, 0.86)
- Context isolation: active — plan-auditor conclusion was not anchor for scoring

---

## Verdict: CONFIRM (plan-auditor PASS holds)

All must-pass dimensions exceed 0.6 threshold. All structural criteria are satisfied. One upgraded defect (M-D16 → MEDIUM) and two new findings are reported as mandatory pre-Run clarifications but do not individually or collectively block PASS.

---

## 4-Dimension Scores

| Dimension | Score | Verdict | Rubric Reference |
|-----------|-------|---------|------------------|
| Functionality (40%) | 0.78 | PASS | Band 0.75-1.0: "Most acceptance criteria are concrete and executable; integration chain explicit; one REQ logical contradiction (REQ-AX-003-E1 vs U1) and one API surface gap (OCR retry endpoint) degrade from 1.0 band" |
| Security (25%) | 0.82 | PASS | Band 0.75-1.0: "Data sovereignty enforced via REQ-UBI-001 + AC-UBI-001 + endpoint allowlist; 망분리 NetworkPolicy documented; PII masking basic regex scoped and acknowledged; audit_logs REQ-UBI-003 complete; SSO exclusion explicitly justified" |
| Craft (20%) | 0.82 | PASS | Band 0.75-1.0: "All 5 EARS types present; REQ-AX-NNN-X naming consistent; Korean body / English identifiers per language.yaml; AC-002-5 bleeds HOW into WHAT (M-D17); schema note at L17 is now residual metadata (M-D18); HISTORY entry dense (M-D19)" |
| Consistency (15%) | 0.88 | PASS | Band 0.75-1.0: "5 MVP capabilities 1:1 mapped to product.md §5.1-5.5; Walking Skeleton aligns with interview.md §10 (6 conditions all addressed); tech.md stack reused consistently (Qwen2-VL/EXAONE/ko-sroberta/pgvector/vLLM); structure.md VECTOR(1536→768) inconsistency handled via explicit handoff note rather than silent acceptance" |

**Overall (weighted)**: 0.81

Calculation: (0.78 × 0.40) + (0.82 × 0.25) + (0.82 × 0.20) + (0.88 × 0.15) = 0.312 + 0.205 + 0.164 + 0.132 = **0.813**

---

## Must-Pass Criteria

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Functionality >= 0.6 | PASS (0.78) | 23 AC scenarios, E2E chain explicit, integration points mapped |
| Security >= 0.6 | PASS (0.82) | REQ-UBI-001, AC-UBI-001, plan.md §3.3 망분리, audit_logs schema |
| Craft >= 0.6 | PASS (0.82) | 5 EARS types verified, REQ-AX-NNN-X naming, Korean body |
| Consistency >= 0.6 | PASS (0.88) | product.md §5.1-5.5 ↔ REQ-AX-001..005; interview.md §10 ↔ spec.md §1 |
| All 5 EARS types present | PASS | Ubiquitous (REQ-UBI), Event (E1/E2), State (S1), Optional (O1), Unwanted (U1/U2) — verified spec.md L130-201 |
| Min 2 G/W/T per REQ module | PASS | REQ-UBI: 3 / AX-001: 5 / AX-002: 6 / AX-003: 3 / AX-004: 3 / AX-005: 3 = 23 total |
| Exclusions minimum 5 entries | PASS | 14 entries (spec.md L228-241), each with concrete scope boundary |
| YAML frontmatter 8 fields | PASS | id, version, status, created, updated, author, priority, issue_number — spec.md L1-10, verified against plan.md L377 canonical schema |
| No time estimates | PASS | Only runtime performance targets and phase ordering labels found; `ETA < 10초` in AC-002-4 is system runtime ETA, not schedule estimate |
| Korean body | PASS | All body text in Korean; identifiers, REQ IDs, endpoint names in English |

---

## Agreement Analysis with plan-auditor

- Plan-auditor verdict: PASS (0.86)
- Evaluator-active verdict: CONFIRM (0.813)
- Score variance: |0.813 - 0.86| = **0.047**
- Variance threshold (Section 12 Mechanism 4): 0.15
- Variance exceeds threshold: **NO** — calibration review not triggered

The score gap (0.047) reflects:
1. Evaluator-active upgraded M-D16 from minor to MEDIUM (REQ-AX-003-E1/U1 mathematical impossibility), reducing Functionality from ~0.85 to 0.78.
2. Evaluator-active identified NEW-F2 (user_id undefined without auth) and NEW-F3 (query embedding latency risk) not flagged by plan-auditor.
3. Consistency and Security scores are higher because evaluator-active independently confirmed tech stack alignment and data sovereignty enforcement.

---

## Re-evaluation of plan-auditor Minor Defects

### M-D15 — OCR retry endpoint missing in §3.2 API surface

**Severity: Minor — confirmed**

`POST /api/documents/doc-101/ocr/retry` is referenced in AC-001-4 but absent from the REQ-AX-001 endpoint surface (spec.md §3.2 only lists `POST /api/documents/upload`). However:
- AC-001-4 provides two acceptable behaviors (HTTP 202 queued OR HTTP 409 already-in-progress), giving implementers clear options.
- The behavioral requirement (no concurrent inference) IS captured in REQ-AX-001-S1 via Celery queueing.
- The specific endpoint path is an implementation decision.

Action: plan.md T-AX-001 should enumerate the retry/concurrency control surface during Run phase. Not blocking.

### M-D16 — `abstain` class undocumented in REQ-AX-003-E1

**Severity: MEDIUM — UPGRADED from Minor**

This is more significant than plan-auditor assessed. The defect involves a **mathematical impossibility**:

REQ-AX-003-E1 specifies a "2-class predictor (A vs B)" with "probability distribution that sums to 1.0 ± 0.001". With standard softmax classification, P(A) + P(B) = 1.0, making it **mathematically impossible** for both P(A) < 0.5 AND P(B) < 0.5 simultaneously. Therefore, REQ-AX-003-U1's trigger condition ("IF the predicted probability for both A and B grades is below 0.5") is dead code for a pure 2-class model.

AC-003-2 resolves this by introducing an `abstain` class: `{A: 0.42, B: 0.45, abstain: 0.13}`. This is the correct engineering solution, but it requires a 3-class model, contradicting REQ-AX-003-E1's "2-class" constraint.

An implementer strictly following REQ-AX-003-E1 will build a 2-class model where REQ-AX-003-U1 never fires. An implementer reading AC-003-2 will correctly build a 3-class system. This ambiguity MUST be resolved before Run phase.

**Pre-Run action required**: REQ-AX-003-E1 must add: "The predictor MAY emit an `abstain` probability mass; when included, all probabilities (A + B + abstain) sum to 1.0 ± 0.001. The `low_confidence` status in REQ-AX-003-U1 triggers when max(P(A), P(B)) < 0.5 regardless of abstain mass." This is a 1-line REQ amendment, not a structural change.

### M-D17 — AC-002-5 reweight detail bleeds HOW into AC

**Severity: Minor — confirmed**

"Confidence reweighted ×0.8" in AC-002-5 (acceptance.md L111) is an implementation specification (a specific multiplier value) rather than an observable contract. The observable behavior should be: "search results containing unresolved hanja are ranked lower." The 0.8 factor belongs in plan.md §3.5.

However, the overall AC-002-5 structure is sound — the other elements (raw fallback, normalization_warning metadata, no crash/HTTP 500, audit_logs entry) are all observable behaviors. This is a polish issue. Not blocking.

### M-D18 — Schema note now redundant after D4/D5 withdrawal

**Severity: Minor — confirmed**

spec.md L17 retains a block explaining why `labels`/`created_at` are not used. Since D4/D5 are formally withdrawn and the canonical schema is confirmed, this block now reads as audit-process metadata rather than SPEC content. Moving this context to HISTORY 0.1.1 or plan.md would clean the spec surface.

Not blocking — the presence of this note does not create implementation ambiguity.

### M-D19 — HISTORY 0.1.1 paragraph dense

**Severity: Minor — confirmed**

The HISTORY 0.1.1 entry (spec.md L13-14) compresses 7+ defect references into a single paragraph. A per-defect bullet format would improve readability. Cosmetic only.

---

## New Findings

### NEW-F1 — REQ-AX-003-E1 vs REQ-AX-003-U1 Mathematical Contradiction (MEDIUM)

See M-D16 UPGRADED above. This finding is the same root cause. REQ-AX-003-E1's "2-class predictor" constraint makes REQ-AX-003-U1's "both below 0.5" condition permanently unreachable with standard classification. Implementers must be told explicitly whether to use a 3-class model (with abstain) or a 2-class model with a different low-confidence metric (e.g., max(logit) < threshold).

**Mandatory pre-Run action**: Add one sentence to REQ-AX-003-E1 clarifying the `abstain` mechanism.

### NEW-F2 — user_id semantics undefined in unauthenticated sandbox (Minor)

REQ-UBI-003 and AC-UBI-003 require each audit_logs record to have a `user_id` field. However, Exclusion 12 explicitly states SSO/JWT authentication is excluded from this SPEC, meaning the sandbox has no authenticated user identity.

The SPEC does not specify what `user_id` value is written when no authentication context exists (e.g., literal `"anonymous"`, `"system"`, `null`, or a session-derived UUID). This is a gap between the audit completeness requirement (REQ-UBI-003) and the authentication exclusion.

AC-UBI-003 verifies that "user_id" field is present but says nothing about its valid values in the unauthenticated case. An implementer could insert NULL (violating the "4 fields present" criterion) or a placeholder — neither is specified.

**Recommended action**: spec.md §3.1 REQ-UBI-003 should add: "In deployments where authentication is not enabled, user_id SHALL default to `'anonymous'`." This can be addressed in Run phase.

### NEW-F3 — Query Embedding Latency Risk for 100ms p99 RAG Target (Informational)

REQ-AX-002-E2 specifies top-3 results with p99 latency < 100ms for `GET /api/criteria/search`. This 100ms budget covers the full API round-trip: query reception → query embedding via ko-sroberta-multitask (170M parameter model) → HNSW vector search → result serialization.

Embedding a query text via ko-sroberta-multitask on CPU takes approximately 20-50ms depending on sequence length. This leaves only 50-80ms for the HNSW search + serialization on the hot path. Without GPU acceleration for embedding or query caching, the 100ms p99 target may be unachievable.

plan.md §3.5 correctly notes `ef_search` tuning for HNSW, but does not address query embedding latency. R-AX-007 covers "pgvector HNSW p99 > 100ms" risk at "낮음" probability — this underestimates the risk when query embedding latency is included in the budget.

**Recommended action**: plan.md R-AX-007 should be revised to include query embedding latency as a contributing factor. AC-002-1 measurement instrumentation should separately measure embedding time vs HNSW search time. Not a SPEC defect per se, but a risk calibration issue.

---

## Anti-Pattern Cross-check

Checked against known anti-patterns per Section 12 Mechanism 5:

- **Scope creep**: Not observed. SPEC is tightly bounded by the Walking Skeleton definition.
- **Vague performance criteria**: Not observed. All 4 performance targets have specific numeric thresholds with measurement instruments identified (Prometheus metrics).
- **Missing edge cases**: M-D16/NEW-F1 represents a missing implementation boundary for the grade predictor. Other edge cases are thoroughly covered (23 scenarios).
- **Security debt inheritance**: Not applicable — Greenfield project. No legacy security debt.
- **Untestable requirements**: REQ-UBI-002's "primary language" is operationally bounded by AC-UBI-002 (Korean ratio < 20% threshold). Subjective quality rating ("4/5") is bounded by "KEPCO E&C 담당자 5명 이상" protocol. No caps applied.

**Anti-pattern threshold**: No dimension capped at 0.50. No anti-pattern pattern found that matches the criteria for score capping under Section 12 Mechanism 5.

---

## Findings Summary

| Severity | ID | Location | Description |
|----------|----|----------|-------------|
| MEDIUM | NEW-F1 / M-D16 | spec.md:L168, acceptance.md:L135 | REQ-AX-003-E1 "2-class predictor" makes REQ-AX-003-U1 "both < 0.5" mathematically unreachable with standard softmax. Requires `abstain` class clarification in REQ. |
| Minor | M-D15 | acceptance.md:L64 | OCR retry endpoint not in §3.2 API surface; behavioral intent covered by REQ-AX-001-S1. |
| Minor | NEW-F2 | spec.md:L132, acceptance.md:L33 | `user_id` value undefined for unauthenticated sandbox; Exclusion 12 creates gap. |
| Minor | M-D17 | acceptance.md:L111 | "×0.8 confidence reweight" is HOW, not WHAT — belongs in plan.md §3.5. |
| Minor | M-D18 | spec.md:L17 | Schema note is residual audit-process metadata; clean for Run phase. |
| Minor | M-D19 | spec.md:L13-14 | HISTORY 0.1.1 paragraph dense; cosmetic. |
| Informational | NEW-F3 | plan.md §3.5, R-AX-007 | Query embedding latency risk underestimated in 100ms p99 target. |

---

## Recommendations

1. **MANDATORY before Run phase** — Amend REQ-AX-003-E1 (spec.md L168) to clarify the `abstain` mass mechanism: add one sentence stating "When both P(A) and P(B) are below 0.5, the system SHALL emit an optional `abstain` probability mass such that P(A) + P(B) + P(abstain) = 1.0 ± 0.001, triggering the `low_confidence` status." This resolves NEW-F1/M-D16 and unblocks the implementer.

2. **Recommended before Run phase** — Add `user_id` default semantics to REQ-UBI-003 for the unauthenticated sandbox case (NEW-F2). Suggested addition: "In authentication-excluded deployments, user_id SHALL default to literal value `'anonymous'`."

3. **Optional, addressable during Run phase** — Add OCR concurrency/retry endpoint to plan.md T-AX-001 output list (M-D15). Move "×0.8 reweight" from AC-002-5 to plan.md §3.5 (M-D17). Move schema note from spec.md L17 to HISTORY or plan.md (M-D18). Reformat HISTORY 0.1.1 as per-defect bullets (M-D19).

4. **Risk calibration** — Revise plan.md R-AX-007 to include query embedding latency as a component of the 100ms p99 budget, and add query embedding instrumentation to AC-002-1 measurement setup (NEW-F3).

---

## Recommendation

**CONFIRM — proceed to Run phase** with mandatory pre-Run amendment to REQ-AX-003-E1 (one sentence clarifying `abstain` class mechanism). The remaining minor findings are addressable during Run phase without architectural consequence.

The SPEC is implementable, internally coherent (with the noted 2-class exception), and aligned with all project context documents. The Walking Skeleton is correctly scoped, the security posture is appropriate for a sandbox PoC, and the 23-scenario acceptance framework provides sufficient coverage for TDD execution.

**Pre-Run gate**: manager-spec must amend REQ-AX-003-E1 before manager-tdd begins T-AX-003 implementation. This is a minor textual amendment, not a structural revision.
