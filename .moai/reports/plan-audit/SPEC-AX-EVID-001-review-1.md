# SPEC Review Report: SPEC-AX-EVID-001
Iteration: 1/3
Verdict: PASS
Overall Score: 0.88

> Reasoning context ignored per M1 Context Isolation. The supplied project context (Go control-plane, thorough harness, TDD, SPEC-AX-CTRL-001 GREEN, issue_number=0 intentional, canonical 8-field schema at `.claude/skills/moai/workflows/plan.md:377`, reference SPEC-AX-CTRL-001) is treated as factual configuration, not author justification. No prose inside the SPEC documents was credited as author reasoning during scoring.

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: Two parallel ID tracks, both contiguous with consistent 3-digit zero-padding and no duplicates. Ubiquitous: `REQ-EVID-UBI-001/002/003/004` (spec.md:L93–96). Modal: `REQ-EVID-001` (spec.md:L98), `REQ-EVID-002` (L120), `REQ-EVID-003` (L134), `REQ-EVID-004` (L146). Sub-requirement IDs within each modal module are well-formed (E1/S1/O1/U1) with no gaps within an EARS type. This dual-track scheme mirrors the GREEN reference SPEC-AX-CTRL-001 (`REQ-CTRL-UBI-*` + `REQ-CTRL-*`). No gap, no duplicate.

- **[PASS] MP-2 EARS format compliance**: All 19 normative `SHALL`/`MAY` clauses in spec.md §3 match an EARS pattern. Ubiquitous: REQ-EVID-UBI-001 (L93, "The evidence management subsystem SHALL NOT make any external service call"), UBI-002 (L94), UBI-004 (L96). State-driven: UBI-003 (L95, "WHILE 인증이 비활성... the subsystem SHALL persist"), REQ-EVID-001-S1 (L110, "WHILE a version-resolving transaction is in progress... SHALL hold a SELECT ... FOR UPDATE lock"), 002-S1 (L128), 004-S1 (L152). Event-driven: 001-E1 (L106, "WHEN a client submits... THEN the subsystem SHALL compute"), 002-E1 (L124), 003-E1 (L140). Optional: 001-O1 (L114, "WHERE an existing evidence row... the subsystem MAY surface"), 004-O1 (L156). Unwanted: 001-U1 (L118, "IF the incoming file exceeds... THEN the subsystem SHALL reject"), 002-U1 (L132), 003-U1 (L144), 004-U1 (L160). The bilingual Korean elaboration appended to each clause is non-normative annotation, identical to the GREEN SPEC-AX-CTRL-001 convention, and does not break the EARS structure. `acceptance.md` correctly uses Given/When/Then for test scenarios (acceptance.md:L2) per `.claude/skills/moai/workflows/plan.md:389` — GWT in acceptance.md is the prescribed format and is NOT counted as informal EARS.

- **[PASS] MP-3 YAML frontmatter validity**: spec.md:L1–10 contains all 8 canonical fields with correct scalar types: `id: SPEC-AX-EVID-001` (L2), `version: 0.1.0` (L3), `status: draft` (L4), `created: 2026-05-18` (L5), `updated: 2026-05-18` (L6), `author: ircp` (L7), `priority: high` (L8), `issue_number: 0` (L9). Validated against the canonical schema enumerated at `.claude/skills/moai/workflows/plan.md:377` (`id, version, status, created, updated, author, priority, issue_number`). `issue_number: 0` is correct per documented project context (gh CLI unavailable). Absence of `labels`/`created_at` is NOT a defect — they are non-canonical (confirmed against plan.md:377 and the spec.md:L16 schema note).

- **[N/A] MP-4 Section 22 language neutrality**: N/A — single-language SPEC. Scope is exclusively the Go control-plane (`apps/control-plane/`, spec.md:L24, L56). Go tooling references (`go vet`, `golangci-lint`, `gosec`, spec.md:L178) describe the project's own single language, not multi-language tooling. Auto-passes per MP-4 single-language clause.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 1.0 band (low) | Every REQ has a single unambiguous EARS interpretation (spec.md:L93–160); no pronoun ambiguity; measurable thresholds. Minor: spec.md:L91 states the pattern is `REQ-UBI-NNN` while the document actually uses domain-scoped `REQ-EVID-UBI-NNN` — cosmetic wording imprecision, not interpretive ambiguity. |
| Completeness | 0.95 | 1.0 band | All required sections present: HISTORY (spec.md:L12), WHY/Context (§1 L22, §1.2 Anchor L36), WHAT (§2 Affected Files L52), REQUIREMENTS (§3 L87), ACCEPTANCE (acceptance.md, referenced spec.md:L221), Exclusions (§5 L182, 9 specific entries). Frontmatter 8/8. Exclusions/Dependencies/Out-of-Scope cleanly separated (§5/§6/§7). |
| Testability | 0.93 | 1.0 band | No weasel words ("appropriate"/"adequate"/"reasonable"). acceptance.md:L8 self-imposes the no-vague-language rule and complies. Every AC Then-clause is binary (exact row counts, exact HTTP codes, byte-identical assertions, `goleak.VerifyNone(t)`, CHECK-constraint violation). Minor methodology note: AC-EVID-001-1 (acceptance.md:L119) "p99 < 150ms (10회 반복)" — n=10 cannot produce a true p99, but the 150ms threshold is binary-measurable. |
| Traceability | 0.80 | 0.75 band | All 18 ACs reference valid existing REQs; no orphaned ACs; no AC points to a non-existent REQ. Every UBI and modal REQ module is covered EXCEPT REQ-EVID-001-O1 (Optional duplicate-of signal), which has no dedicated AC and whose only cross-reference (acceptance.md:L357 §6 catalog) falsely claims AC-EVID-001-1 covers it. One Optional sub-req indirectly/inaccurately mapped → 0.75 band. |

## Defects Found

**D1.** acceptance.md:L357 (§6 Edge Case Catalog) — `중복 SHA-256 (duplicate 신호, non-blocking) | AC-EVID-001-1 + REQ-EVID-001-O1` falsely asserts that AC-EVID-001-1 verifies the REQ-EVID-001-O1 duplicate-detection behavior. AC-EVID-001-1 (acceptance.md:L104–119) is a single fresh-create happy path with a new `evaluation_item_id` and no pre-existing identical-hash row; its Given/When/Then contains no `duplicate_of` assertion. Consequently REQ-EVID-001-O1 (spec.md:L114, "the subsystem MAY surface a non-blocking `duplicate_of` signal") has **no verifying acceptance criterion**, and the §6 catalog presents a coverage illusion. Note: REQ-EVID-001-O1 is `MAY`/Optional and explicitly deactivatable ("Sandbox PoC에서는 비활성 가능", spec.md:L114), so functional impact is low — but the false coverage claim is a documentation-integrity defect. — Severity: minor

**D2.** spec.md:L73 vs plan.md:L186 — Inconsistent description of the existing baseline DB schema. spec.md §2.2 (L73) lists the existing schema path as `.moai/db/schema/initial.sql` (`[EXISTING]`, "참조 only"), while plan.md §8 (L186) states "현재 migrations/ 에는 `0001_initial.sql`만 존재". Two different stated locations/filenames for the same existing baseline (`initial.sql` at schema root vs `migrations/0001_initial.sql`). The same plan.md §8 sentence also leaves the `0002_evidence_tables.sql` filename-collision with SPEC-AX-CTRL-001 conditionally unresolved ("CTRL-001의 `0002_workflow_indexes.sql`이 아직 미커밋 상태이면 run 단계 진입 시 번호 조정 협의"). Run-phase resolvable and explicitly flagged as a risk, but the path/filename discrepancy is a factual internal inconsistency between spec.md and plan.md that should be reconciled before Run. — Severity: minor

(Sub-observations, folded into D1's consistency theme — not separate defects: acceptance.md:L358 §6 maps "이전 버전 본문 UPDATE 거부" (immutability) to risk `R-EVID-002`, which plan.md:L173 defines as "트랜잭션 원자성 — orphan 증빙 행" — a conceptually different risk than immutability; this is cosmetic risk-ID reuse in the edge catalog.)

## Chain-of-Verification Pass

Second-look findings (re-read sections that warranted closer inspection):

- **REQ sequencing re-verified end-to-end** (not spot-checked): UBI-001..004 contiguous; modal 001..004 contiguous; sub-req IDs within each modal module well-formed. Confirmed REQ-EVID-004 intentionally has no Event-driven sub-req (S1/O1/U1 only) — acceptable; not all REQ modules require all 5 EARS types, and REQ-EVID-004 still has 3 ACs (acceptance.md:L397 "각 modal REQ 모듈은 최소 3개 AC" holds). MP-1 confirmed.
- **Contradiction sweep (UBI-004 vs 002-E1)**: Apparent tension — REQ-EVID-UBI-004 (spec.md:L96) forbids UPDATE of any column once a newer version exists, while REQ-EVID-002-E1 (L124) sets the preceding row's `status` to `SUPERSEDED`. Verified NO contradiction: UBI-004 explicitly carves out the exception ("상태 컬럼 `status`만 `ACTIVE`→`SUPERSEDED` 전이 허용, 본문/파일 메타데이터는 불변"), and REQ-EVID-002-U1 (L132) restricts the immutable column set to `file_name/file_size_bytes/file_hash_sha256/content_type/storage_location/metadata` (excluding `status`). Consistently stated across UBI-004, 002-E1, 002-U1, and AC-EVID-UBI-004 (acceptance.md:L96). Internally consistent.
- **Traceability re-verified per-REQ (not sampled)**: every one of the 18 ACs mapped to its REQ; the only gap is the Optional REQ-EVID-001-O1 captured in D1.
- **Exclusions specificity re-checked (not presence-only)**: §5 (spec.md:L186–194) has 9 numbered entries, each naming a concrete excluded artifact with a named deferral target (e.g., `SPEC-AX-EVAL-ITEM-001` placeholder, `plan.md §6 OPEN DECISION`, SPEC-AX-AUTH series). No vague entries.
- **Citation accuracy spot-audit**: spec.md:L44 cites `.claude/skills/moai/workflows/plan.md:366` for Composite domain rules — verified L366 reads exactly "Composite domain rules: Maximum 2 domains recommended, maximum 3 allowed." spec.md:L16 cites plan.md L378 for the schema note — verified L377–378 enumerates the 8 fields. Both citations are accurate (the author corrected the broken-citation class of defect seen in SPEC-AX-CTRL-001 iteration 1).

No new defects beyond D1, D2 surfaced in the second pass. The first pass was thorough on must-pass and EARS; the second pass strengthened the contradiction and traceability checks and confirmed no critical/major defects exist.

## Regression Check (Iteration 2+ only)

N/A — iteration 1. No prior report exists for SPEC-AX-EVID-001.

## Recommendation

**Verdict: PASS.** Rationale with evidence per must-pass criterion:

- MP-1 PASS: REQ IDs contiguous and unique across both tracks (spec.md:L93–160).
- MP-2 PASS: all 19 normative clauses match an EARS pattern (spec.md:L93–160); acceptance.md correctly uses GWT (acceptance.md:L2) per workflow spec.
- MP-3 PASS: 8/8 canonical frontmatter fields, correct types (spec.md:L1–10) vs `.claude/skills/moai/workflows/plan.md:377`.
- MP-4 N/A: single-language Go SPEC.

All four scored dimensions are at or above their target bands (Clarity 0.90, Completeness 0.95, Testability 0.93, Traceability 0.80). The two defects found are both **minor**, run-phase-resolvable, and neither is a must-pass failure nor a correctness blocker. The SPEC demonstrably applied corrective lessons from prior SPEC-AX-CTRL-001 audits (accurate line citations, AC-count consistency across spec-compact.md/acceptance.md, dedicated transverse UBI ACs, Cross-SPEC artifact section, named-placeholder deferral).

Required corrections for the next plan/run handoff (track in iteration-2 regression check if a re-audit is triggered; not blocking PASS):

1. **D1**: Resolve the REQ-EVID-001-O1 coverage gap. Either (a) add a dedicated AC that exercises the duplicate-hash → `duplicate_of` signal path, or (b) correct acceptance.md:L357 §6 to state REQ-EVID-001-O1 is an intentionally-unverified Optional feature (and remove the false "AC-EVID-001-1" mapping). Option (b) is acceptable given O1 is `MAY` and PoC-deactivatable, but the false mapping must not remain.
2. **D2**: Reconcile spec.md:L73 and plan.md:L186 — state the existing baseline schema's canonical path/filename once, consistently, and resolve (or explicitly defer with a decision owner) the `0002_evidence_tables.sql` filename-collision risk with SPEC-AX-CTRL-001 before Run begins.
3. **Cosmetic (optional)**: spec.md:L91 wording ("REQ-UBI-NNN") and the acceptance.md:L358 `R-EVID-002` risk-ID reuse for the immutability edge case — tidy for precision; no functional impact.
