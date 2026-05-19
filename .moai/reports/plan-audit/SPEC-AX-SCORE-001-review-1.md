# SPEC Review Report: SPEC-AX-SCORE-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.88

Reasoning context ignored per M1 Context Isolation. The orchestrator-supplied
"factual project context" (canonical 8-field schema, issue_number=0 expected,
§6 OPEN intentional, deferral out-of-scope) was treated strictly as audit scoping
(legitimate orchestrator instruction, corroborated by `.claude/skills/moai/workflows/plan.md:377-378`
and auto-memory `feedback_plan_auditor_schema.md`), NOT as author justification.
Every "not a defect" claim was independently verified against the four source files
before acceptance.

Inputs audited: spec.md, plan.md, acceptance.md, spec-compact.md
(research.md read only for cross-reference of provenance claims; not graded).

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: Dual-track pattern (1 Ubiquitous bundle + 4 modal modules), identical to the PASS-audited SPEC-AX-EVAL-ITEM-001 precedent. UBI sequential `REQ-SCORE-UBI-001..004` (spec.md:101-104), no gap/dup. Modal modules `REQ-SCORE-001/002/003/004` sequential (spec.md:106,128,140,156), no gap, no duplicate, consistent 3-digit zero-padding. Sub-clause suffixes E1/S1/O1/U1 consistently applied (spec.md:114-168).
- **[PASS] MP-2 EARS format compliance**: All 14 requirement clauses match exactly one of the five EARS patterns. Ubiquitous: REQ-SCORE-UBI-001 (spec.md:101), -002 (L102), -004 (L104). State-driven: REQ-SCORE-UBI-003 (`WHILE ... 동안, the subsystem SHALL persist` L103), REQ-SCORE-001-S1 (L118), REQ-SCORE-003-S1 (L150). Event-driven: REQ-SCORE-001-E1 (`WHEN ... THEN ... SHALL` L114), -002-E1 (L134), -003-E1 (L146), -004-E1 (L164). Optional: REQ-SCORE-001-O1 (`WHERE ... the subsystem SHALL` L122). Unwanted: REQ-SCORE-001-U1 (`IF ... THEN ... SHALL` L126), -002-U1 (L138), -003-U1 (L154), -004-U1 (L168). REQ-SCORE-001-E1 is a single-trigger atomic operation (not the dual-trigger conflation that EVAL-ITEM-001 D2 had to split) — checked and cleared.
- **[PASS] MP-3 YAML frontmatter validity**: Project canonical 8-field schema applied per `.claude/skills/moai/workflows/plan.md:377-378` and auto-memory `feedback_plan_auditor_schema.md`. spec.md:1-10 — `id: SPEC-AX-SCORE-001`, `version: 0.1.0`, `status: draft`, `created: 2026-05-19`, `updated: 2026-05-19`, `author: ircp`, `priority: high`, `issue_number: 0`. All 8 present, correct types. `issue_number: 0` intentional (gh unavailable, canonical "0 if Issue creation skipped"). Absence of `labels`/`created_at` is correct per canonical schema and explicitly documented at spec.md:16 (Schema note) — NOT flagged per the schema-firewall.
- **[N/A] MP-4 Section 22 language neutrality**: N/A — single-language Go SPEC (`apps/control-plane/`, Go 1.22+). Only Go-ecosystem tools referenced (pgx, zap, testify, testcontainers-go, golangci-lint, gosec). No multi-language tooling enumeration required. Auto-pass.

No must-pass failure. The FAIL verdict derives from internal-consistency defects (M5 firewall not triggered; verdict driven by the audited "AC counts aligned across all 4 files" dimension).

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.95 | 0.75–1.0 (single, unambiguous interpretation; only controlled, explicitly-scoped deferrals) | §6 OPEN items flagged as OPEN/strategy-confirmable at every occurrence (spec.md:30,35,104,138,146,150,154,160); disjunctive policies (REQ-SCORE-002-U1 spec.md:138, -003-U1 L154) keep the invariant fixed/testable and only defer precise policy. No pronoun ambiguity. |
| Completeness | 1.00 | 1.0 (all sections + frontmatter + ≥1 specific exclusion) | HISTORY (spec.md:12-16), WHY (§1/§1.2 L22-40), WHAT (§1.1/§2 L26-93), HOW (plan.md §3 DDL/§4 Sprint), REQUIREMENTS (§3 L95-168), ACCEPTANCE (acceptance.md full), Exclusions (§5 — 7 specific entries L198-204 + §7). Frontmatter 8/8. |
| Testability | 0.92 | 0.75–1.0 (binary G/W/T, concrete values, weasel words explicitly forbidden) | Concrete assertions: AC-SCORE-002-1 `90×0.5+80×0.3+70×0.2=83.00` (acceptance.md:186); AC-SCORE-001-3 `data_type='character varying', character_maximum_length=64` (L135); weasel-word ban stated (acceptance.md:8). §6-dependent ACs split fixed-testable-core vs strategy-deferred-precision. |
| Traceability | 1.00 | 1.0 (every REQ ≥1 AC; every AC → valid existing REQ; no orphans/uncovered) | All 15 REQ→AC links verified with citations: UBI-001..004 → AC-SCORE-UBI-001..004 (acceptance.md:16,33,50,67); 001-E1/S1/O1/U1 → AC-SCORE-001-1/(2+3)/O1-1/4 (L89,105,122,154,139); 002/003/004 → AC-SCORE-002-{1,2}/003-{1,2}/004-{1,2} (L179,196,218,235,262,283). AC-SCORE-BOUNDARY-1 → §5#3 scope boundary (valid, explicitly labeled). |

---

## Defects Found

**D1. acceptance.md:404 + spec-compact.md:25 — "Total AC count: 14" is wrong and self-contradictory; actual enumerable AC count is 16 — Severity: major**

Evidence:
- acceptance.md:404 states verbatim: `Total AC count: 14 — (§0 UBI: 4 [...], §1: 5 [...], §2: 2 [...], §3: 2 [...], §4: 2 [...], §5: 1 [...])`. The stated component breakdown sums to `4+5+2+2+2+1 = 16`, contradicting the stated total of `14` within the same sentence.
- Physical enumeration of every AC heading in acceptance.md confirms 16: AC-SCORE-UBI-001 (L16), -UBI-002 (L33), -UBI-003 (L50), -UBI-004 (L67), AC-SCORE-001-1 (L89), -001-2 (L105), -001-3 (L122), -001-4 (L139), -001-O1-1 (L154), AC-SCORE-002-1 (L179), -002-2 (L196), AC-SCORE-003-1 (L218), -003-2 (L235), AC-SCORE-004-1 (L262), -004-2 (L283), AC-SCORE-BOUNDARY-1 (L308) = **16 ACs**.
- acceptance.md:391 (DoD bullet) independently enumerates the same modal set (`001-{1..4,O1-1}`=5, `002-{1,2}`=2, `003-{1,2}`=2, `004-{1,2}`=2 = 11 modal) + 4 UBI + 1 BOUNDARY = 16, directly contradicting L404's "14".
- spec-compact.md:25 propagates the same error: `## AC (14: AC-SCORE-{REQ}-{N})`, while spec-compact.md:27-32 lists a breakdown that sums to 16.

This is a triple-internal contradiction (L404 total vs L404 breakdown vs L391 enumeration vs 16 actual headings) that has propagated into the spec-compact.md quick-reference companion. It directly fails the commissioned audit dimension "internal consistency (AC counts ... aligned across all 4 files)". The SPEC-AX-EVAL-ITEM-001 precedent HISTORY (spec.md:14-17) shows this project family version-tracks and grades AC-count accuracy as a load-bearing consistency invariant (19→20→21→22), so an inaccurate, self-contradictory AC ledger total is not approval-ready in this family's standard. Mechanically fixable.

**D2. acceptance.md:394 — §7 Edge Case Catalog DoD claims "16개 edge case" but the §7 table contains 15 rows — Severity: minor**

Evidence:
- acceptance.md:394 DoD bullet: `§7: 16개 edge case 모두 대응 AC로 검증`.
- The §7 Edge Case Catalog table rows (acceptance.md:348-362) enumerate exactly 15 distinct edge cases (점수 생성+audit / evidence_id stub / evaluation_item_id stub / blank·64초과·비수치·비-UUID 거부 / metadata opaque / 단일 레벨 Σ / NULL weight / score→letter / 등급 경계+미설정 / audit fail rollback / store→audit 순환 회피 / scores 물리 삭제 / FK 부재 / cli-anonymous / 외부 LLM 호출 = 15).
- 15 ≠ 16. Lower impact than D1 (the edge-case catalog is a supporting cross-reference, not the primary AC ledger), but the EVAL-ITEM-001 HISTORY explicitly tracked "edge case 15→16" as a graded correction item, so the count mismatch is audit-relevant. Mechanically fixable.

(No EARS, traceability, frontmatter, scope-boundary, or requirement-contradiction defects found. Type contracts `scores.evaluation_item_id=VARCHAR(64)` and `scores.evidence_id=UUID` verified internally consistent across all 4 files: spec.md:52-53,108,118 / plan.md:17-18,56-57 / acceptance.md:118,135 / spec-compact.md:7-8.)

---

## Chain-of-Verification Pass

Second-look findings: ONE additional defect discovered and D1 strengthened.

- Re-read every REQ clause (not skimmed): all 14 individually verified against the 5 EARS patterns with line citations — no new EARS defect.
- Re-checked REQ sequencing end-to-end: UBI-001..004 and modal 001..004 both gap-free/dup-free, consistent zero-pad — confirmed.
- Re-verified traceability for every REQ (not sampled): all 15 links re-walked; REQ-SCORE-001-S1 correctly maps to two ACs (AC-SCORE-001-2 acceptance.md:117 + AC-SCORE-001-3 acceptance.md:136); no orphan AC, no uncovered REQ — confirmed.
- Re-checked Exclusions for specificity (not just presence): all 7 entries (spec.md:198-204) carry concrete scope + cross-references (e.g., #3 names files `0002`/`0003`; #5 names `product.md:171-183`/`:177`; #2 cites SPEC-AX-EVAL-ITEM-001 §5 #2) — specific, not vague.
- Looked for contradictions between requirements: UBI-004 vs §4 NFR (spec.md:178), §1.1 minimal-rollup vs §5#4 vs REQ-SCORE-002-E1, UBI-001 vs §3.4 internal-threshold, evidence_id nullability across all 4 files — no cross-requirement contradiction.
- NEW: acceptance.md:391 DoD enumeration independently sums to 16, adding a third internal contradiction to D1 (strengthens D1 to major).
- NEW: acceptance.md:394 §7 edge-case DoD count (16) vs actual §7 table rows (15) — surfaced as D2.
- Re-confirmed §6 OPEN state and out-of-scope deferrals are explicitly stated in the documents themselves (spec.md:259, plan.md:8/135-137, acceptance.md:8, spec-compact.md:59) — independently verified, NOT accepted on author faith; correctly NOT flagged per audit scope.

---

## Recommendation

FAIL on iteration 1 due to a major internal-consistency defect (D1) in the exact dimension this audit was commissioned to check ("AC counts aligned across all 4 files"), plus a minor count mismatch (D2). The SPEC is otherwise of high quality (EARS 1.0, Traceability 1.0, Completeness 1.0, canonical-clean frontmatter, internally consistent type contracts, explicitly-stated OPEN/exclusion boundaries). Both defects are mechanical and low-cost. Fix instructions for manager-spec:

1. **Fix D1 — correct the AC total to 16 in two locations:**
   - acceptance.md:404 — change `Total AC count: 14` to `Total AC count: 16` (the component breakdown `4+5+2+2+2+1` already correctly equals 16; only the stated total is wrong). Verify the corrected total against the 16 physical AC headings listed in this report's D1 evidence.
   - spec-compact.md:25 — change `## AC (14: AC-SCORE-{REQ}-{N})` to `## AC (16: AC-SCORE-{REQ}-{N})`.
   - After correction, confirm acceptance.md:404 total == acceptance.md:391 enumeration == spec-compact.md:25 count == 16 (single source of truth).

2. **Fix D2 — reconcile the §7 edge-case count:**
   - acceptance.md:394 — either change `§7: 16개 edge case` to `§7: 15개 edge case` to match the 15 rows in the §7 table (acceptance.md:348-362), OR add the missing 16th edge-case row to the §7 table if one was intended (decide based on whether all 16 ACs each warrant a distinct edge-case entry — note BOUNDARY-1 and the 4 UBI ACs are already represented, so 15 may be the correct intended count; verify against the actual table before choosing). State the chosen count consistently.

3. **Regression guard for iteration 2:** when re-submitting, re-derive the AC total by physically enumerating `### AC-SCORE-` headings in acceptance.md and assert it equals the number stated at acceptance.md:404, spec-compact.md:25, and the §9 DoD breakdown — these four must be identical. Apply the same physical-count check to the §7 edge-case table vs acceptance.md:394.

No requirement-substance, EARS, traceability, frontmatter, or scope-boundary changes are required — do NOT alter the requirement text, the type contracts, the §6 OPEN deferrals, or the exclusion set; those are correct as written. Scope the iteration-2 fix strictly to the count metadata in acceptance.md and spec-compact.md.
