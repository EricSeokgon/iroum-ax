# SPEC-AX-001 Plan Audit — Iteration 1

> Reasoning context ignored per M1 Context Isolation. This audit is based solely on the SPEC artifacts and project documentation read independently.

- Auditor: plan-auditor (adversarial mode)
- Harness level: thorough
- Iteration: 1/3
- Date: 2026-05-14

## Verdict: FAIL

## Overall Score: 0.62

Reason: MP-3 YAML frontmatter firewall violation (missing `labels`, field name `created` instead of `created_at`) combined with audit-prompt criterion violation (REQ-UBI module has only 1 AC for 3 sub-requirements — below the minimum 2 G/W/T per module requirement) and multiple Traceability gaps (4 REQs without dedicated ACs). Per M5 Must-Pass Firewall, any single must-pass failure forces overall FAIL regardless of other dimension scores.

---

## Must-Pass Criteria

- [X] **MP-1 REQ number consistency**: PASS
  - Evidence: spec.md L131-198. Modules REQ-AX-001 through REQ-AX-005 are sequential, no gaps, no duplicates. Sub-IDs (E1, E2, S1, O1, U1, U2) are well-formed within each module. REQ-UBI-001/002/003 are sequential within Ubiquitous section.

- [X] **MP-2 EARS format compliance**: PASS
  - Evidence: All 17 EARS clauses (REQ-UBI-001..003 + 14 sub-requirements across REQ-AX-001..005) match one of the five EARS patterns exactly.
  - Examples:
    - Ubiquitous: spec.md:L127 `The system SHALL store all input documents...`
    - Event-driven: spec.md:L134 `WHEN a user uploads a HWP or PDF file... THEN the system SHALL...`
    - State-driven: spec.md:L137 `WHILE the VLM is processing OCR... THE system SHALL not accept...`
    - Optional: spec.md:L140 `WHERE a GPU is available... THE system SHALL use...`
    - Unwanted: spec.md:L143 `IF hwp-converter detects HWP OLE structure corruption... THEN the system SHALL...`
  - All five EARS types present at the SPEC level.

- [ ] **MP-3 YAML frontmatter validity**: FAIL
  - Evidence: spec.md L1-10. Field set is `id, version, status, created, updated, author, priority, issue_number` (8 fields total).
  - **Defect 1**: `labels` field is MISSING (required per MP-3 firewall in plan-auditor spec).
  - **Defect 2**: Field named `created` (spec.md:L5), not `created_at` (required per MP-3 firewall).
  - Even though the count is 8 fields, the firewall requires specific field names (`id`, `version`, `status`, `created_at`, `priority`, `labels`). Two of six required canonical fields are absent or mis-named.

- [X] **MP-4 Section 22 language neutrality**: N/A
  - Justification: SPEC-AX-001 is a single-domain Walking Skeleton for Korean public-sector report automation. It is not a multi-language tooling SPEC. The language-specific tools (hwp-converter, ko-sroberta-multitask, Qwen2-VL, EXAONE 3.5) are bound to the Korean domain by design (see product.md §3.3, tech.md §2-3). Auto-pass per MP-4 definition.

### Audit-Prompt-Specific Must-Pass Criteria

- [X] **EARS compliance (all 5 types present, testable, ≤ 5 REQ modules)**: PASS
  - Evidence: All 5 EARS types present across the SPEC (spec.md §3.1-3.6). 5 REQ-AX modules + 1 REQ-UBI module ≤ 5 functional modules. REQ-IDs follow `REQ-AX-NNN-X` format (variant of `SPEC-AX-NNN-X`, acceptable per project convention `interview.md` L142).
  - Caveat: Not every REQ-AX module contains all 5 EARS types (REQ-AX-002 lacks Optional, REQ-AX-003 lacks Optional, REQ-AX-004 lacks State-driven, REQ-AX-005 lacks Optional). This is allowed under standard EARS practice — modules use only the types relevant to their scope.

- [ ] **Acceptance criteria minimum 2 G/W/T per module**: FAIL
  - Evidence: acceptance.md:L5 explicitly states `REQ-UBI 1개` (1 scenario for REQ-UBI). REQ-UBI module contains 3 sub-requirements (REQ-UBI-001, 002, 003) but only AC-UBI-001 (acceptance.md:L15-20) is defined.
  - REQ-AX-001 through REQ-AX-005 each have 3 scenarios (≥ 2). ✓
  - REQ-UBI module FAILS the minimum.

- [X] **Exclusions minimum 5 entries**: PASS
  - Evidence: spec.md:L225-238 lists 14 Exclusions entries, each specific (Console UI, batch processing, 500 evaluation items, ESG/audit/license/공문 domains, finance, multi-tenant, prod K8s, audit-log UI, fine-tuning, advanced PII, C/D grades, SSO/JWT, Helm prod values, CI/CD deploy automation).

- [PARTIAL] **YAML frontmatter 8 fields complete**: PARTIAL
  - Evidence: spec.md:L1-10 has 8 fields by count (id, version, status, created, updated, author, priority, issue_number) → meets count requirement.
  - However, see MP-3 above: missing `labels`, field name `created` vs `created_at`. Marking as PARTIAL but the MP-3 firewall failure dominates.

- [X] **No time estimates**: PASS
  - Evidence: spec.md and plan.md use only performance/latency targets (`<2 seconds per page`, `<100ms p99`, `within 30 seconds`, `within 5 seconds`) which are quality criteria, not schedule estimates. plan.md:L4 explicitly states `모든 우선순위는 High/Medium/Low 레이블만 사용한다 (시간 추정 금지)`. plan.md §2 uses "Phase 1/2/3/4" ordering. No instances of "weeks", "months", "days", or "as soon as possible" found via grep.

- [X] **Greenfield constraints (no Delta markers)**: PASS
  - Evidence: grep for `[EXISTING]`, `[MODIFY]`, `[REMOVE]`, `[REPLACE]` across spec.md, plan.md, acceptance.md returned 0 matches.
  - research.md:L181 explicitly states `Delta marker ([EXISTING], [MODIFY], [REMOVE]) 미사용. 모든 파일은 신규 작성 ([NEW] 가정)`.
  - All affected file paths in spec.md §2 cross-reference structure.md §2 (verified for 30+ files).

- [PARTIAL] **thorough harness compliance**: PARTIAL
  - Evidence: acceptance.md §10 references plan-auditor (item 2) and evaluator-active 4-dimension scoring (item 3), consistent with thorough harness requirements.
  - However, the Traceability gaps below undermine "thorough" quality.

---

## Category Scores (rubric-anchored per M3)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band — "Minor ambiguity in one or two requirements" | REQ-UBI-002 (`Korean text as the primary language`) — "primary" is not measurable. REQ-AX-003-U1 (`predicted probability for both A and B grades is below 0.5`) — testable but conflicts with AC-003-2 threshold 0.55 (see Defect D6). Other REQs are precise. (spec.md:L128, L171) |
| Completeness | 0.75 | 0.75 band — "One non-critical section missing or sparse" | All required sections present (HISTORY L12, WHY/개요 L20, WHAT/§2-3, REQUIREMENTS §3, ACCEPTANCE in acceptance.md, Exclusions §5). HISTORY entry is sparse (single 0.1.0 line). Frontmatter complete by count but missing `labels`. |
| Testability | 0.50 | 0.50 band — "Several ACs contain weasel words or require judgment calls" | acceptance.md:L173 `사용자 만족도 항목 포함` and §8.1 `공문 스타일 주관 평가 ≥ 4/5 (KEPCO E&C 담당자 5명 이상)` — subjective evaluation depends on human raters, not binary-testable; acceptance.md:L106 `output in 합니다체` — testable via style_applier but `subjective rating ≥ 4/5` is judgment-based. AC-001-1 `파싱 정확도가 95% 이상` is testable but requires manual ground-truth labels not specified. |
| Traceability | 0.50 | 0.50 band — "Multiple REQs lack ACs" | 4 REQs lack dedicated ACs (REQ-UBI-002, REQ-UBI-003, REQ-AX-001-S1, REQ-AX-002-S1). AC-001-3 claims to verify REQ-AX-001-O1 (acceptance.md:L46) but only tests rotated PDF, not the GPU-vs-CPU branching logic. AC-003-2 claims to verify REQ-AX-003-U1 (acceptance.md:L88) but introduces a 0.55 threshold absent from the REQ. |

---

## Defects Found

### Critical (blocks PASS)

- **D1** (acceptance.md:L13-20) — REQ-UBI module has only 1 AC for 3 sub-requirements. REQ-UBI-002 (Korean primary language) and REQ-UBI-003 (audit log entry events) have NO acceptance scenarios. Severity: critical. Suggested fix: Add `AC-UBI-002` covering Korean text processing verification (e.g., a non-Korean input is rejected or processed with degraded confidence flag), and `AC-UBI-003` covering audit_logs table population for each of the 4 events listed in REQ-UBI-003 (document upload, workflow creation, draft generation, prediction).

- **D2** (spec.md:L137-138) — REQ-AX-001-S1 (OCR concurrency queueing via Celery) has NO corresponding AC in acceptance.md §2. Severity: critical. Suggested fix: Add `AC-001-4` testing concurrent OCR requests for the same document — second request should be queued, not accepted.

- **D3** (spec.md:L155-156) — REQ-AX-002-S1 (index rebuild blocks retrieval) has NO corresponding AC in acceptance.md §3. Severity: critical. Suggested fix: Add `AC-002-4` testing retrieval request during HNSW index rebuild — request should be queued or rejected, not served stale results.

- **D4** (spec.md:L1-10) — YAML frontmatter missing `labels` field. Severity: critical (MP-3 firewall). Suggested fix: Add `labels: [poc, anchor, kepco-enc, walking-skeleton]` or similar at line 9.

- **D5** (spec.md:L5) — YAML frontmatter uses field name `created` instead of `created_at` (MP-3 firewall requires `created_at` as ISO date string). Severity: critical (MP-3 firewall). Suggested fix: Rename `created: 2026-05-14` to `created_at: 2026-05-14`. Apply the same to `updated_at`.

### Major (should fix before run)

- **D6** (spec.md:L171 vs acceptance.md:L86-88) — REQ-AX-003-U1 states `IF the predicted probability for both A and B grades is below 0.5`, but AC-003-2 tests the scenario `{A: 0.48, B: 0.52}` where only A is below 0.5 (B is above). AC-003-2 introduces an additional `threshold 0.55 권장` not stated in the REQ. This is a Traceability inconsistency: the AC tests a different rule than the REQ specifies. Severity: major. Suggested fix: Either (a) update REQ-AX-003-U1 to clarify the threshold semantics (e.g., `if max(p_a, p_b) < 0.55`), or (b) update AC-003-2 to test both probabilities below 0.5 as the primary scenario.

- **D7** (acceptance.md:L42-46 claims to verify REQ-AX-001-O1) — AC-001-3 (rotated PDF) does not actually test the WHERE-clause logic of REQ-AX-001-O1 (GPU-available branch vs GPU-unavailable branch). It only tests OCR on a rotated table. Severity: major. Suggested fix: Add `AC-001-5` explicitly toggling GPU availability and asserting the inference path (`vllm` vs CPU) selected.

- **D8** (acceptance.md:L5) — Claim "Total: 16 시나리오" is mathematically correct (1+3×5=16) but the REQ-UBI count of "1" hides the gap that 2 of 3 REQ-UBI sub-requirements are uncovered. Severity: major. Suggested fix: Increase to at least 18 scenarios with 3 REQ-UBI ACs (one per sub-requirement).

### Minor (nice to have)

- **D9** (research.md:L19, acceptance.md:L70) — 한자/한글 혼용 edge case is mentioned (AC-002-3 says "한자/한글 혼용 텍스트는 정규화 후 임베딩된다") but no AC tests the failure mode when normalization itself fails or degrades embedding quality. Audit prompt requires this Korean-specific edge case. Severity: minor.

- **D10** (research.md R-AX-003, plan.md §4) — RAG cold-start (no chunks indexed yet, vs the partial-result scenario AC-002-2 covers) is not explicitly tested. AC-002-2 covers `insufficient_context` for a missing query; a cold-start scenario (entire index empty) is not exercised. Severity: minor.

- **D11** (Exclusions item 11) — Excludes C/D 4-class but does not document the consequence: customer-provided C/D benchmark reports (product.md §3.2 item 4: "A/B/C/D 등급 타 기관 안전보건 실적보고서") will NOT be utilized in this SPEC. The data asset under-utilization should be explicit in Exclusions or research.md risk register. Severity: minor.

- **D12** (structure.md:L329 vs plan.md §3.5) — structure.md defines `criteria.embedding VECTOR(1536)` but plan.md §3.5 and tech.md confirm ko-sroberta-multitask is 768-dim. This is a structure.md issue out of strict SPEC scope but will cause a runtime DDL conflict when T-AX-008 creates the schema. Severity: minor (technically a structure.md defect, not spec.md). Recommend manager-spec note this in the SPEC handoff or correct structure.md.

- **D13** (spec.md HISTORY L12-14) — HISTORY has only one entry (0.1.0). For a thorough-harness SPEC, including the original Discovery interview date and a pointer to interview.md §10 anchor would strengthen provenance. Severity: minor.

- **D14** (acceptance.md:L153 NFR table) — Performance criteria reference `tech.md §11.1` but the audit prompt's Korean-specific edge cases (HWP corruption, 공문 mismatches) are not separately broken out as performance/quality criteria. Severity: minor.

---

## Chain-of-Verification Pass

Second-look findings:

1. **Re-read each REQ-AX module top to bottom**: Confirmed REQ-AX-001 has 5 sub-requirements (E1, S1, O1, U1, U2). Initial scan had identified all defects already. No new defects found in REQ-AX-001..005 sub-requirements.

2. **REQ number sequencing end-to-end**: Verified REQ-AX-001 → 002 → 003 → 004 → 005 are sequential. Sub-IDs E1, E2, S1, O1, U1, U2 are consistent across modules where used. No gaps.

3. **Traceability for every REQ (not just sample)**: Confirmed 4 REQs lack dedicated ACs (D1, D2, D3 above plus REQ-UBI-002, REQ-UBI-003 within D1). Found D6 (AC-003-2 / REQ-AX-003-U1 inconsistency) and D7 (AC-001-3 weak link to REQ-AX-001-O1) during the second pass.

4. **Exclusions specificity (not just presence)**: Re-read all 14 items. All are specific with concrete scope boundaries. No vague entries like "advanced features" or "future work".

5. **Contradictions between requirements**: Re-checked for cross-REQ contradictions. REQ-UBI-001 prohibits external LLM calls; REQ-AX-004-U2 reinforces this. No contradictions found. REQ-AX-004-O1 (EXAONE primary, Qwen fallback) aligns with tech.md §3.3.

6. **YAML frontmatter recount**: Confirmed 8 fields present, `labels` missing, `created` instead of `created_at`. Defects D4 and D5 stand.

7. **Performance table consistency**: spec.md §4 NFR table values match acceptance.md §7 measurement table values. Consistent. ✓

8. **Korean-specific edge cases comprehensive check**: HWP corruption (AC-001-2 ✓), 공문 honorifics (AC-004-2 ✓), 한자/한글 mixed text (AC-002-3 — covers normalization, not failure mode — partial), RAG cold-start (no AC — D10). Two of four Korean-specific edge cases are weakly covered.

Conclusion of second pass: Original findings stand. D6, D7 strengthened during second-look. No new critical defects discovered, but D6/D7 (acceptance-to-requirement mismatch) moved from minor to major during second-pass scrutiny.

---

## Regression Check

N/A — this is iteration 1.

---

## Recommendations for manager-spec

Address the following in priority order:

1. **Fix YAML frontmatter (D4, D5)** — Add `labels: [poc, anchor, kepco-enc, walking-skeleton, ax]` and rename `created`/`updated` to `created_at`/`updated_at` in spec.md:L1-10. This resolves the MP-3 firewall failure.

2. **Add missing ACs (D1, D2, D3)** — Author the following acceptance scenarios in acceptance.md:
   - `AC-UBI-002` covering REQ-UBI-002 (Korean text primary processing verified by mixed-language input handling).
   - `AC-UBI-003` covering REQ-UBI-003 (audit_logs table populated for upload + workflow_create + draft_generate + prediction events).
   - `AC-001-4` covering REQ-AX-001-S1 (concurrent OCR for same document queued via Celery).
   - `AC-002-4` covering REQ-AX-002-S1 (retrieval blocked during HNSW rebuild, queued not stale-served).
   
   Update acceptance.md:L5 total count from 16 to 20.

3. **Reconcile AC-003-2 with REQ-AX-003-U1 (D6)** — Choose one of:
   - Option A (recommended): Update REQ-AX-003-U1 to use `IF max(p_a, p_b) < 0.55` and reconcile AC-003-2 to test exactly that boundary case.
   - Option B: Keep REQ-AX-003-U1 as-is (`both below 0.5`) and rewrite AC-003-2's primary scenario to `{A: 0.42, B: 0.45}` so both are below 0.5. Move the `{A: 0.48, B: 0.52}` scenario to a separate AC (e.g., `AC-003-4` for "near-tie majority") with its own REQ if the 0.55 rule is desired.

4. **Strengthen AC-001-3 → REQ-AX-001-O1 link (D7)** — Add an explicit GPU-availability toggle AC. Suggested `AC-001-5`: Given GPU unavailable in container, When document uploaded, Then system uses CPU inference path and response latency budget relaxes by 5-10x (per `tech.md` §6.1).

5. **Add Korean-specific edge case coverage (D9, D10)** — Add `AC-002-5` for 한자/한글 normalization failure mode (graceful degradation when normalization can't resolve a 한자), and `AC-002-6` for RAG cold-start (zero chunks indexed; the search endpoint returns clear bootstrapping status, not a 500).

6. **Document C/D data under-utilization consequence (D11)** — In Exclusions item 11 or research.md §4 risk register, add: "결과: KEPCO E&C가 제공한 C/D 등급 벤치마크 보고서 데이터(product.md §3.2 item 4)는 본 SPEC 범위에서 학습 입력으로 사용되지 않는다. SPEC-AX-002 이후 4-class 확장 시 활용 예정."

7. **Correct structure.md pgvector dimension (D12)** — Outside SPEC scope strictly, but flag to user: structure.md:L329 `VECTOR(1536)` conflicts with ko-sroberta-multitask 768 dim. Either update structure.md to `VECTOR(768)` or note that the dimension will be determined by the chosen embedding model.

8. **Expand HISTORY (D13)** — Optional: Reference interview.md §10 anchor and 2026-05-14 Discovery date for full provenance.

After applying these fixes, re-submit for iteration 2. The expected verdict change requires all critical defects (D1-D5) and major defects (D6-D8) resolved; minor defects (D9-D14) may remain for iteration 2 review at user discretion.

---

## Audit Summary

The SPEC-AX-001 Walking Skeleton demonstrates strong domain framing, clear Walking Skeleton boundaries, and disciplined cross-referencing to project documentation. However, it FAILS the iteration 1 audit on two must-pass dimensions: (a) MP-3 YAML frontmatter is incomplete (missing `labels`, field `created` vs `created_at`), and (b) the audit-required minimum of 2 G/W/T scenarios per REQ module is not met for the REQ-UBI module. Additionally, 4 sub-requirements (REQ-UBI-002, REQ-UBI-003, REQ-AX-001-S1, REQ-AX-002-S1) lack dedicated acceptance scenarios, and AC-003-2 introduces a 0.55 threshold not present in REQ-AX-003-U1. With these gaps addressed, the SPEC has a credible path to PASS in iteration 2 — the underlying EARS structure, exclusions discipline, and risk coverage are sound.
