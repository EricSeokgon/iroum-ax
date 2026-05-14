# SPEC-AX-001 Plan Audit — Iteration 2

> Reasoning context ignored per M1 Context Isolation, except for the explicit Schema Reconciliation Context (procedural directive to verify canonical schema at `.claude/skills/moai/workflows/plan.md` Phase 2 L377). All other author defenses were ignored. Findings below are formed independently from the revised SPEC artifacts and the verified canonical schema source.

- Auditor: plan-auditor (adversarial mode)
- Harness level: thorough
- Iteration: 2/3
- Date: 2026-05-14
- Previous report: `.moai/reports/plan-audit/SPEC-AX-001-review-1.md` (iteration 1, FAIL, 14 defects)

## Verdict: PASS

## Overall Score: 0.86

Reason: All 14 iteration-1 defects are either resolved (12) or formally withdrawn after schema verification (2). The revised SPEC now satisfies all must-pass criteria: MP-1 (REQ consistency), MP-2 (EARS), MP-3 (frontmatter against the verified canonical 8-field schema), and MP-4 (N/A — single-domain Korean SPEC). The audit-prompt minimum 2 G/W/T per module is now met (REQ-UBI has 3 ACs, REQ-AX-001 has 5, REQ-AX-002 has 6, others have 3 each, totaling 23). Traceability is complete: every REQ sub-requirement maps to at least one AC. No new critical or major defects were identified during the fresh pass. Several minor observations are noted but none block PASS.

---

## Schema Reconciliation Outcome (D4/D5)

I independently read `.claude/skills/moai/workflows/plan.md` Phase 2 L370-378 from the project's canonical source. The file explicitly states at L377:

```
YAML frontmatter with 8 required fields (id, version, status, created, updated, author, priority, issue_number)
```

This confirms the canonical schema is exactly the 8 fields shown — neither `labels` nor `created_at` is part of the project's SPEC frontmatter schema. The iteration-1 defects D4 and D5 were therefore based on an incorrect schema assumption (the auditor template's generic schema versus the project's actual canonical schema).

- **D4 (`labels` missing)**: **WITHDRAWN**. Evidence: plan.md:L377 lists 8 canonical fields; `labels` is not among them. Per audit protocol, MP-3 is evaluated against the project's canonical schema, not a generic template.
- **D5 (`created` vs `created_at`)**: **WITHDRAWN**. Evidence: plan.md:L377 specifies `created` and `updated` (not `created_at`/`updated_at`). spec.md:L5-6 conforms.

The "Schema note" at spec.md:L17 added by manager-spec is informative but no longer needed for the audit verdict. It does not harm the document and can remain as provenance.

---

## Iteration 1 Defect Resolution Status

| ID | Status | Evidence |
|----|--------|----------|
| D1 (REQ-UBI 1 AC for 3 sub-reqs) | **RESOLVED** | acceptance.md:L13-34 — AC-UBI-001 (external API block), AC-UBI-002 (Korean primary, mixed-language), AC-UBI-003 (4 audit events). 1→3 ACs, one per sub-requirement. |
| D2 (REQ-AX-001-S1 no AC) | **RESOLVED** | acceptance.md:L62-66 — AC-001-4 tests Celery queueing on duplicate OCR for same document, expects either HTTP 202 queued or HTTP 409 in-progress; no concurrent inference allowed. |
| D3 (REQ-AX-002-S1 no AC) | **RESOLVED** | acceptance.md:L100-105 — AC-002-4 tests retrieval during HNSW `reindex_status: rebuilding`; either internal queue or HTTP 503 + `retry_after_seconds`; no stale/partial results returned. |
| D4 (frontmatter missing `labels`) | **WITHDRAWN** | Canonical schema confirmed (plan.md:L377). See Schema Reconciliation Outcome. |
| D5 (`created` vs `created_at`) | **WITHDRAWN** | Canonical schema confirmed (plan.md:L377). See Schema Reconciliation Outcome. |
| D6 (AC-003-2 vs REQ-AX-003-U1 inconsistency) | **RESOLVED** | acceptance.md:L131-138 — AC-003-2 primary scenario rewritten to `{A: 0.42, B: 0.45, abstain: 0.13}` where `max(p_a, p_b) < 0.5`, matching REQ-AX-003-U1. 0.55 threshold is explicitly excluded (Note at L138 defers "near-tie majority" to SPEC-AX-002, Option B accepted). |
| D7 (AC-001-3 weak link to REQ-AX-001-O1) | **RESOLVED** | acceptance.md:L70-73 — AC-001-5 explicitly toggles `CUDA_VISIBLE_DEVICES=0` vs `""`, asserts `inference_backend: "vllm_gpu"` vs `"transformers_cpu"`, with p99 < 2s vs p99 < 20s (5-10× relaxed per tech.md §6.1). AC-001-3 scope narrowed accordingly (acceptance.md:L59). |
| D8 (16 → 18+ scenarios) | **RESOLVED** | acceptance.md:L5 — total now 23 (REQ-UBI 3 + REQ-AX-001 5 + REQ-AX-002 6 + REQ-AX-003 3 + REQ-AX-004 3 + REQ-AX-005 3). Verified by counting AC headings: confirmed 23. |
| D9 (한자/한글 normalization failure missing) | **RESOLVED** | acceptance.md:L107-112 — AC-002-5 tests unresolved hanja graceful path: raw fallback, `normalization_warning`, confidence ×0.8 reweight, no HTTP 500, audit_logs entry. |
| D10 (RAG cold-start missing) | **RESOLVED** | acceptance.md:L114-118 — AC-002-6 tests empty index path (`COUNT(*) = 0`), returns HTTP 503 or HTTP 200 + `{status:"index_not_bootstrapped", indexed_chunks:0, next_step:...}`. Distinguished from AC-002-2 (insufficient_context). |
| D11 (C/D data under-utilization) | **RESOLVED** | spec.md:L238 — Exclusions item 11 explicitly states: "KEPCO E&C가 제공한 C/D 등급 벤치마크 보고서 데이터(`product.md` §3.2 item 4)는 본 SPEC 범위에서 학습 입력으로 사용되지 않는다... 데이터 보관·인덱싱 자체도 제외한다 (under-utilization 명시)." Same notation reproduced in spec-compact.md:L247. |
| D12 (structure.md VECTOR(1536) vs 768) | **RESOLVED (handoff)** | spec.md:L248 — explicit handoff note added in §6: "T-AX-008 DDL 작성 시 `VECTOR(768)`로 정정하며... 본 불일치는 SPEC-AX-001 범위 밖이나 Run phase 시작 전 해결되어야 한다." Acceptable scope handoff. |
| D13 (HISTORY sparse) | **RESOLVED** | spec.md:L15 — 0.1.0 entry now references interview.md §10 anchor explicitly: "Discovery 인터뷰는 `.moai/project/interview.md` §10 anchor 참조." Iteration 2 entry at L14 documents all changes. |
| D14 (Korean-specific quality criteria) | **RESOLVED** | acceptance.md:L212-223 — new §7.1 with 6-row Korean-specific quality table: HWP OLE corruption recovery rate (≥80%), 합니다체 consistency (≥95%), 한자 normalization success (≥90%), 한자 graceful (100% no 500), Korean ratio < 20% warning (100%), RAG cold-start explicit response (100%). |

**Summary**: 12 resolved, 2 withdrawn (D4, D5), 0 unresolved.

---

## Must-Pass Criteria (re-check)

- [X] **MP-1 REQ number consistency**: PASS. spec.md:L130-201 — REQ-UBI-001/002/003 + REQ-AX-001..005 sequential, no gaps, no duplicates. Sub-IDs (E1/E2/S1/O1/U1/U2) well-formed.
- [X] **MP-2 EARS format compliance**: PASS. All 17 EARS clauses (3 ubiquitous + 14 sub-requirements) match the five canonical EARS patterns. spec.md:L130-201, verified clause-by-clause. All 5 EARS types present at SPEC level (Ubiquitous L130, Event-driven L137, State-driven L140, Optional L143, Unwanted L146-147).
- [X] **MP-3 YAML frontmatter validity (per canonical 8-field schema)**: PASS. spec.md:L1-10 has exactly the 8 canonical fields: `id, version, status, created, updated, author, priority, issue_number`. Types correct (string id, string version "0.1.1", string status "draft", ISO date "2026-05-14", string author, string priority "high", integer issue_number 0). After D4/D5 withdrawal, no firewall violation remains.
- [X] **MP-4 Section 22 language neutrality**: N/A (auto-pass). SPEC is single-domain (Korean public-sector report automation, anchored to KEPCO E&C). Language-specific tooling (Qwen2-VL, EXAONE 3.5, ko-sroberta-multitask, hwp-converter) is bound by domain, not by multi-language tooling scope.

### Audit-Prompt-Specific Must-Pass Criteria

- [X] **EARS compliance (all 5 types present, ≤ 5 functional REQ-AX modules)**: PASS. All 5 EARS types present; 5 REQ-AX modules.
- [X] **Acceptance criteria minimum 2 G/W/T per module**: PASS. REQ-UBI 3 / REQ-AX-001 5 / REQ-AX-002 6 / REQ-AX-003 3 / REQ-AX-004 3 / REQ-AX-005 3. Total 23. Verified by counting AC-* headings.
- [X] **Exclusions minimum 5 entries**: PASS. spec.md:L228-241 lists 14 specific Exclusions; all retain concrete scope boundaries; Exclusion 11 enriched with under-utilization consequence.
- [X] **YAML frontmatter complete per canonical schema (post D4/D5 withdrawal)**: PASS. Schema verified against plan.md:L377. All 8 fields present and well-typed.
- [X] **No time estimates**: PASS. spec.md, plan.md, acceptance.md use only latency/performance targets and Phase-ordered priorities. plan.md:L4 reaffirms "시간 추정 금지". The only `ETA < 10초` reference in AC-002-4:L104 is a runtime ETA (system behavior), not a schedule estimate — allowed.
- [X] **Greenfield (no Delta markers)**: PASS. No `[EXISTING]/[MODIFY]/[REMOVE]/[REPLACE]` markers detected in any of the 5 files. research.md:L181 reiterates `[NEW]` assumption.
- [X] **thorough harness compliance**: PASS. acceptance.md:§10 invokes plan-auditor + evaluator-active 4-dim scoring + KEPCO E&C subjective review. acceptance.md §8.2 enumerates TDD + 85% coverage + LSP zero-error + ast-grep + TRUST 5 gates.

---

## Category Scores (rubric-anchored per M3)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | between 0.75-1.0 band — "Every requirement has a single, unambiguous interpretation" | REQ-UBI-002 is now operationally clarified by AC-UBI-002 (Korean ratio thresholds, parse_quality_flag). REQ-AX-003-U1 inconsistency removed. Minor residual ambiguity in REQ-UBI-002's word "primary" remains but is operationally bounded by AC-UBI-002. |
| Completeness | 0.95 | 1.0 band — "All required sections present, frontmatter complete, exclusions present" | HISTORY (now 2 entries with interview anchor), WHY (§1), WHAT (§2-3), REQUIREMENTS (§3), NFR (§4), ACCEPTANCE (acceptance.md), Exclusions (§5, 14 items), dependencies (§6), MX plan (plan.md §5). Quality criteria broken out (§7.1 Korean-specific). Frontmatter 8/8 fields per canonical schema. |
| Testability | 0.80 | between 0.75-1.0 band | 23/23 ACs are concrete with measurable criteria. AC-001-5 binary-testable via env var injection. AC-003-2 binary-testable via probability distribution check. AC-002-5/006 binary-testable via response status. Subjective "주관 평가 4/5 이상" (acceptance.md:L155, L236) remains but is bounded to a single dimension and documented as "주관 평가 (KEPCO E&C 담당자 5명 이상)" — operationally testable via human rater protocol; not blocking. |
| Traceability | 1.00 | 1.0 band — "Every REQ has ≥ 1 AC; every AC references a valid REQ" | Verified 22/22 REQ sub-requirements have at least one AC. Mapping verified: REQ-UBI-001→AC-UBI-001; REQ-UBI-002→AC-UBI-002; REQ-UBI-003→AC-UBI-003; REQ-AX-001-{E1,S1,O1,U1,U2}→{AC-001-1/3, AC-001-4, AC-001-5, AC-001-2, AC-001-2}; REQ-AX-002-{E1,E2,S1,U1}→{AC-002-1/3/5, AC-002-1, AC-002-4, AC-002-2/6}; REQ-AX-003-{E1,S1,U1}→{AC-003-1, AC-003-3, AC-003-2}; REQ-AX-004-{E1,O1,U1,U2}→{AC-004-1, AC-004-3, AC-004-2, AC-004-3}; REQ-AX-005-{E1,S1,U1}→{AC-005-1, AC-005-2, AC-005-3}. No orphan ACs. |

Aggregate (weighted average, default profile): ≈ 0.86.

---

## New Defects (fresh pass)

### Critical
None.

### Major
None.

### Minor

- **M-D15** (acceptance.md:L64) — AC-001-4 references endpoint `POST /api/documents/doc-101/ocr/retry` which is not enumerated in spec.md §3.2 REQ-AX-001 (only `POST /api/documents/upload` is). The AC also offers an alternative wording "동일 결과를 유발하는 OCR 재요청 엔드포인트", so it's not a hard contradiction, but a future implementation could add a separate endpoint not yet captured in the API contract section. Recommend manager-spec add a brief endpoint catalog in spec.md §3 or note in plan.md that the retry endpoint is part of T-AX-001 entry-point surface. Severity: minor — non-blocking.

- **M-D16** (acceptance.md:L135) — AC-003-2 references an `abstain` probability of 0.13 alongside A/B probabilities. REQ-AX-003-E1 (spec.md:L168) defines a "2-class predictor (A vs B)" with a "probability distribution that sums to 1.0 ± 0.001". Including an `abstain` output is a 3rd class that is not strictly forbidden but is undocumented in REQ-AX-003-E1. AC text says "또는 동등하게 sum=1.0 ± 0.001을 만족하는 분포" which permits the alternative interpretation. Recommend a one-line clarification in REQ-AX-003-E1 noting that an `abstain` mass is optional. Severity: minor — non-blocking.

- **M-D17** (acceptance.md:L111) — AC-002-5 introduces a "confidence reweighted ×0.8" rule that is an implementation specification not present in REQ-AX-002. This bleeds HOW into the AC. The AC could state the externally-observable contract (e.g., "search results affected by unresolved hanja are tagged with a normalization_warning and ranked lower in returned order") and leave the 0.8 factor to plan.md. Severity: minor — non-blocking.

- **M-D18** (spec.md:L17) — The "Schema note (D4/D5 결정)" block embedded after HISTORY now reads as audit-process metadata rather than SPEC content. Since D4/D5 are formally withdrawn in this iteration, the schema note has served its purpose and could be moved to plan.md or HISTORY 0.1.1 entry alone. Severity: minor — non-blocking.

- **M-D19** (spec.md:L13-14) — HISTORY 0.1.1 entry is a single dense paragraph mixing 7+ defect references. Slightly harder to read than a per-defect bullet list. Severity: minor — non-blocking.

None of M-D15..M-D19 block PASS; they are improvement notes for further polish.

---

## Chain-of-Verification Pass

Second-look findings:

1. **Re-read each REQ-AX module end-to-end**: REQ-AX-001 has 5 sub-requirements (E1, S1, O1, U1, U2); REQ-AX-002 has 4 (E1, E2, S1, U1); REQ-AX-003 has 3 (E1, S1, U1); REQ-AX-004 has 4 (E1, O1, U1, U2); REQ-AX-005 has 3 (E1, S1, U1). Plus REQ-UBI 3. Total 22 sub-REQs. All have at least one AC. No new module-level defects.

2. **REQ sequencing end-to-end**: Verified L130-201 of spec.md. No gaps, no duplicates.

3. **Traceability for EVERY REQ (not sample)**: Performed exhaustive 22-row mapping — see Traceability score evidence above. All REQ → AC mappings present. AC-UBI-003 mentions 4 actions `{document_upload, workflow_create, draft_generate, prediction}` matching the 4 events in REQ-UBI-003 — 1:1 mapping complete.

4. **Exclusions specificity (not just presence)**: All 14 items re-read. Each names a concrete scope boundary (UI, batch, item count, domain, finance, tenancy, K8s prod, audit UI, fine-tuning, PII tier, C/D class, SSO, Helm prod, CI/CD deploy). Item 11 strengthened with under-utilization clause.

5. **Cross-REQ contradictions**: 
   - REQ-UBI-001 (no external API) vs REQ-AX-004-O1/U2 (fallback to Qwen 2.5, no escalation) — consistent.
   - AC-005-1 (workflow with B prediction triggers recommendation) vs AC-003-2 (low_confidence blocks downstream) — non-contradictory: AC-005-1's setup specifies a concrete B prediction, AC-003-2's setup specifies the low_confidence path. The two paths are mutually exclusive in their preconditions.
   - AC-001-3 OCR scope narrowed to "OCR output quality only" while GPU/CPU branching moved to AC-001-5 — clean separation.

6. **YAML frontmatter recount against verified schema**: 8 fields, all present, all correctly typed. Verified.

7. **Performance table consistency**: spec.md §4 NFR (4 performance targets) ↔ acceptance.md §7 (same 4 in measurement table) ↔ spec-compact.md "Performance Targets" — three-way consistent.

8. **spec-compact.md regeneration**: All 23 ACs reproduced (compact form). REQ list complete. Affected files match spec.md §2. Exclusions list 14 items. Iteration 2 history block at L285-299. Internal consistency verified.

9. **Korean-specific edge cases**: HWP corruption (AC-001-2 + §7.1 row 1 ✓), 공문 honorifics (AC-004-2 + §7.1 row 2 ✓), 한자/한글 normalization happy (AC-002-3 + §7.1 row 3 ✓), 한자 graceful (AC-002-5 + §7.1 row 4 ✓), Korean ratio (AC-UBI-002 + §7.1 row 5 ✓), RAG cold-start (AC-002-6 + §7.1 row 6 ✓). All four iter-1 Korean-specific gaps now covered with both AC and quality metric.

10. **Schema reconciliation directive followed**: Independently read plan.md:L370-378. Confirmed 8 canonical fields. D4 and D5 withdrawn on evidence.

11. **Time estimate sweep**: searched for "week", "day", "month", "ASAP", "as soon as", "추정". Found none in spec.md/plan.md/acceptance.md/spec-compact.md. Only runtime ETAs and performance budgets. PASS.

12. **Greenfield sweep**: searched for `[EXISTING]`, `[MODIFY]`, `[REMOVE]`, `[REPLACE]`. None found. PASS.

Conclusion of second pass: All iteration-1 defects either resolved or formally withdrawn. No new critical/major defects discovered. Five minor observations (M-D15..M-D19) recorded as improvement notes, not blockers.

---

## Regression Check (Iteration 2)

All 14 iteration-1 defects (D1-D14) re-verified above. Status:

- 12 resolved with concrete evidence
- 2 withdrawn after independent schema verification
- 0 unresolved
- 0 stagnating defects (no defect appears unchanged across iterations 1 and 2)

No regression introduced by the revision. The new ACs are additive and do not invalidate any iteration-1-passing criterion (MP-1, MP-2, audit-prompt EARS compliance, Exclusions ≥ 5, no time estimates, Greenfield, thorough harness — all still PASS).

---

## Audit Summary

The iteration-2 revision adequately addresses all 14 iteration-1 defects. Twelve are resolved with concrete cited evidence (new ACs, exclusion clarifications, structure.md handoff note, Korean-specific quality table, HISTORY enrichment), and two (D4, D5) are formally withdrawn after independent verification of the project's canonical 8-field SPEC frontmatter schema at `.claude/skills/moai/workflows/plan.md` L377 — these defects were artifacts of an incorrect generic schema assumption in iteration 1. The revised SPEC passes all must-pass criteria: REQ consistency, EARS compliance, frontmatter validity (per canonical schema), language neutrality (N/A), minimum 2 G/W/T per module, ≥ 5 exclusions, no time estimates, Greenfield discipline, and thorough harness compliance. Traceability is now complete (22/22 REQ sub-requirements → ≥1 AC). Five minor observations are noted for future polish but none block PASS.

---

## Recommendation

**Proceed to evaluator-active cross-validation.**

Rationale by must-pass criterion:
- MP-1 PASS — spec.md:L130-201 REQ sequence verified.
- MP-2 PASS — 17 EARS clauses verified against 5 canonical patterns.
- MP-3 PASS — 8 canonical frontmatter fields present (plan.md:L377 verified).
- MP-4 N/A — single-domain SPEC.
- Audit-prompt must-pass items all PASS — 23 ACs across 6 modules, 14 exclusions, no time estimates, no Delta markers, thorough harness gates documented.

Optional polish (non-blocking, can be addressed during Run phase or in a follow-up):
1. (M-D15) Add the OCR retry endpoint to spec.md §3.2 endpoint surface or document it in plan.md T-AX-001 outputs.
2. (M-D16) Clarify in REQ-AX-003-E1 whether an `abstain` probability mass is permitted alongside A/B.
3. (M-D17) Lift the "×0.8 confidence reweight" detail from AC-002-5 into plan.md §3.5 (RAG strategy section).
4. (M-D18, M-D19) Tidy HISTORY 0.1.1 paragraph and consider relocating the inline Schema note now that D4/D5 are formally withdrawn.

None of these are required for PASS. The SPEC is ready for the next gate.
