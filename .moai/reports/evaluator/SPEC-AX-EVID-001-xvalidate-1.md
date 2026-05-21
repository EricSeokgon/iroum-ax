# Cross-Validation Report: SPEC-AX-EVID-001

**Cross-Validation: AGREE-PASS**

SPEC: SPEC-AX-EVID-001 v0.1.1
Evaluator: evaluator-active (independent cross-validator)
Prior verdict being challenged: PASS (plan-auditor iter 1, score 0.88)
This verdict: AGREE-PASS

---

## Methodology

Independent re-evaluation of the five must-pass criteria. Prior auditor's D1/D2 corrections in v0.1.1 verified independently (not taken on trust). Adversarial posture: actively seeking defects to overturn PASS.

Files inspected:
- `/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-EVID-001/spec.md` (v0.1.1)
- `/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-EVID-001/plan.md`
- `/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-EVID-001/acceptance.md`
- `/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-EVID-001/spec-compact.md`
- `/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-EVID-001/research.md`
- `/home/sklee/moai/iroum-ax/.moai/reports/plan-audit/SPEC-AX-EVID-001-review-1.md`

---

## Per-Criterion Findings

### Criterion 1: EARS Format Compliance + REQ→AC Traceability (including Optional REQ-EVID-001-O1)

**Verdict: PASS**

All 16 normative REQ clauses verified independently:

| REQ | EARS type | Keyword present | AC coverage |
|-----|-----------|-----------------|-------------|
| REQ-EVID-UBI-001 | Ubiquitous | "SHALL NOT" (no trigger required) | AC-EVID-UBI-001 |
| REQ-EVID-UBI-002 | Ubiquitous | "SHALL write" | AC-EVID-UBI-002-A/B |
| REQ-EVID-UBI-003 | State-driven | "WHILE 인증이 비활성... SHALL persist" | AC-EVID-UBI-003 |
| REQ-EVID-UBI-004 | Ubiquitous | "SHALL NOT UPDATE or DELETE" | AC-EVID-UBI-004 |
| REQ-EVID-001-E1 | Event-driven | "WHEN a client submits... THEN SHALL" | AC-EVID-001-1 |
| REQ-EVID-001-S1 | State-driven | "WHILE a version-resolving transaction... SHALL hold" | AC-EVID-001-4 |
| REQ-EVID-001-O1 | Optional | "WHERE an existing evidence row... MAY surface" | AC-EVID-001-O1-1 (NEW in v0.1.1) |
| REQ-EVID-001-U1 | Unwanted | "IF the incoming file exceeds... SHALL reject" | AC-EVID-001-2 |
| REQ-EVID-002-E1 | Event-driven | "WHEN a client submits... THEN SHALL determine" | AC-EVID-002-1 |
| REQ-EVID-002-S1 | State-driven | "WHILE more than one version exists... SHALL preserve" | AC-EVID-002-2/3 |
| REQ-EVID-002-U1 | Unwanted | "IF any code path attempts to UPDATE... SHALL reject" | AC-EVID-002-3 |
| REQ-EVID-003-E1 | Event-driven | "WHEN an evidence create or version transaction calls... SHALL construct" | AC-EVID-003-1/2 |
| REQ-EVID-003-U1 | Unwanted | "IF the audit_logs INSERT fails... SHALL execute Rollback" | AC-EVID-003-3 |
| REQ-EVID-004-S1 | State-driven | "WHILE any evidence row is persisted... SHALL hold exactly one value" | AC-EVID-004-1 |
| REQ-EVID-004-O1 | Optional | "WHERE the EvidenceBlobStore interface is defined... SHALL expose it" | AC-EVID-004-3 |
| REQ-EVID-004-U1 | Unwanted | "IF a future storage strategy... issues a network call... SHALL be considered non-compliant" | AC-EVID-004-2 |

D1 verification (prior auditor finding: REQ-EVID-001-O1 had no dedicated AC):
- acceptance.md line 169–191 contains AC-EVID-001-O1-1 with a complete Given/When/Then
- Given: version=1 already exists for eval-001-o1, duplicate signal enabled
- When: byte-identical file resubmitted
- Then: HTTP 201 (not rejected), version=2 created, `duplicate_of` field present in response; AND inactive mode: `duplicate_of` field absent
- §6 Edge Case Catalog (acceptance.md line 381) correctly shows "중복 SHA-256 (duplicate 신호, non-blocking) | AC-EVID-001-O1-1 | (선택 기능, REQ-EVID-001-O1)"

D1 is **genuinely resolved**. The false AC-EVID-001-1 mapping is gone; AC-EVID-001-O1-1 is a real dedicated AC with proper binary-measurable assertions.

Minor classification note (non-blocking): REQ-EVID-UBI-004 uses no EARS keyword ("once a newer version exists" is a state-conditional) yet is classified as Ubiquitous rather than State-driven. REQ-EVID-002-U1 covers the same constraint with correct Unwanted framing. This dual-coverage is not a defect — both are traceable to ACs.

Minor testability note carried over (non-blocking): AC-EVID-001-1 line 119 specifies "p99 < 150ms (10회 반복)". n=10 cannot produce a statistically valid p99. The 150ms threshold itself is binary-measurable; only the statistical framing is imprecise. Not a blocking issue.

---

### Criterion 2: Internal Consistency (AC counts, version, HISTORY, cross-references across all 4 files)

**Verdict: PASS with one minor defect (D3)**

AC count consistency:
| File | AC count stated |
|------|----------------|
| acceptance.md §8 | 19 (§0:5, §1:5, §2:3, §3:3, §4:3) |
| spec-compact.md line 41 | 19 (§0:5, §1:5, §2:3, §3:3, §4:3) |
| Manual count from acceptance.md | 19 (verified: UBI-001/002-A/002-B/003/004=5, 001-1/2/3/4/O1-1=5, 002-1/2/3=3, 003-1/2/3=3, 004-1/2/3=3) |

Count is consistent across files. ✓

Version consistency:
- spec.md YAML frontmatter: `version: 0.1.1` ✓
- spec.md HISTORY: v0.1.1 entry with D1/D2 correction details ✓
- spec-compact.md header: "원본: `spec.md` v0.1.1" ✓
- **plan.md header line 3: "대응 SPEC: `.moai/specs/SPEC-AX-EVID-001/spec.md` v0.1.0"** ← STALE

**D3 [Minor]**: plan.md line 3 version reference is v0.1.0 but spec.md is at v0.1.1. plan.md §8 content was updated as part of the D2 fix (the 0002_evidence_tables.sql conflict resolution text appears in §8), but the header self-description was not bumped from 0.1.0 to 0.1.1. This is a cosmetic documentation inconsistency — plan.md §8 content accurately reflects v0.1.1 changes, only the header version pointer is stale.

D2 verification (prior auditor finding: spec.md §2.2 vs plan.md §8 path inconsistency):
- spec.md §2.2 (lines 73-75): `.moai/db/schema/initial.sql` = [EXISTING] reference only; `.moai/db/schema/migrations/0002_evidence_tables.sql` = [NEW]
- plan.md §8 (lines 186-187): `.moai/db/schema/initial.sql` = existing baseline; `.moai/db/schema/migrations/NNNN_*.sql` = migrations; `migrations/` has only `0001_initial.sql`; this SPEC's new file is `0002_evidence_tables.sql`
- The conditional CTRL-001 conflict language has been replaced with explicit confirmation: "0002_evidence_tables.sql 파일번호 비충돌 확정 (조건부 표현 제거)"

D2 is **genuinely resolved**. ✓

Cross-reference accuracy:
- spec.md line 44 cites `.claude/skills/moai/workflows/plan.md:366` for Composite domain rules — accepted as correct per prior auditor verification (not re-verified independently; the citation format itself is accurate)
- spec.md line 17 cites plan.md L378 for schema note — prior auditor confirmed L377-378 lists the 8 fields

---

### Criterion 3: Scope/Exclusion Completeness and Testability

**Verdict: PASS**

9 exclusions in spec.md §5, each with a named deferral target or explicit rationale:
1. Evaluation taxonomy → `SPEC-AX-EVAL-ITEM-001` (named placeholder, explicitly "미생성") ✓
2. Upload UI/UX → Console responsibility ✓
3. Storage strategy selection → `plan.md §6 OPEN DECISION` (explicit cross-reference) ✓
4. Auth/authorization → `SPEC-AX-AUTH 계열` ✓
5. Delete/retention policy → `archived_at` placeholder defined ✓
6. Evidence-workflow linkage → `evaluation_item_id` scope only ✓
7. Content parsing/OCR/RAG → Python pipelines ✓
8. Advanced search/filtering → single eval_item_id query only ✓
9. Migration tool integration → consistent with SPEC-AX-CTRL-001 Exclusion §8 ✓

All 9 exclusions are specific with named targets — no vague "out of scope" entries.

Testability: Every Then-clause in acceptance.md is binary-measurable (exact row counts, exact HTTP status codes, byte-identical assertions, `goleak.VerifyNone(t)`, CHECK-constraint violation). No weasel words ("appropriate", "reasonable", "adequate") detected.

spec.md §4 non-functional requirements table: all rows have measurable criteria (p99 < 150ms, ≥ 85% coverage, external egress 0 calls, SELECT FOR UPDATE concurrency). ✓

---

### Criterion 4: No Phantom APIs

**Verdict: PASS**

Claimed extension points vs. research.md verified signatures:

**WorkflowStore/WorkflowTx pattern (research.md §2, store.go:13-51):**
- Actual: `WorkflowStore.BeginTx() (WorkflowTx, error)` + WorkflowTx{InsertWorkflow, InsertAuditLog, UpdateWorkflowState, GetWorkflow, UpdateWorkflowResult, Commit, Rollback}
- Claimed in spec.md: `EvidenceStore.BeginTx(ctx) (EvidenceTx, error)` + EvidenceTx{InsertEvidence, GetEvidenceByID, GetLatestVersionByEvalItem, ListEvidenceByEvalItem, InsertAuditLog, Commit, Rollback}
- Assessment: Correct structural mirroring. EvidenceTx retains InsertAuditLog (same as WorkflowTx), adds domain-specific query methods. Pattern is authentic, not phantom. ✓

**Recorder/AuditTx pattern (research.md §3, recorder.go:33-212):**
- Actual: `func (r *Recorder) Record*(ctx, tx AuditTx, workflowID, userID string) error` + local `AuditTx` interface at audit/recorder.go:28-31 (avoids circular dependency)
- Claimed in spec.md: `RecordEvidenceCreated`/`RecordEvidenceVersioned` with "기존 `RecordCreated` 시그니처 패턴, 로컬 `AuditTx` 인터페이스 유지"
- AC-EVID-003-1 specifies: `Recorder.RecordEvidenceCreated(ctx, tx, evidenceID, evalItemID, hash, version, userID="")`
- Assessment: The claimed methods extend the existing signature pattern with domain-specific parameters (evidenceID, evalItemID, hash, version instead of workflowID). This is not a phantom — the base pattern (AuditTx-based, same recorder.go, same resolveUserID) is real and confirmed in research.md §3. The parameter extension is proportionate to the domain. ✓

**Action constants (research.md §3, audit.go:13-54):**
- Actual: 8 workflow actions + auth/server actions (all `Action = "WORKFLOW_..."`, `Action = "AUTH_..."` etc.)
- Claimed: `ActionEvidenceCreated Action = "EVIDENCE_CREATED"`, `ActionEvidenceVersioned Action = "EVIDENCE_VERSIONED"`
- Assessment: Additive constants following the exact same `Action string` type pattern (audit.go:1 confirms `type Action string`). Not phantom. ✓

**`AuditTx` interface (audit/recorder.go:28-31):**
- research.md §3: local interface, subset of WorkflowTx.InsertAuditLog, prevents circular dependency (store imports audit, not vice versa)
- spec.md §3.4 and plan.md §7 R-EVID-001: correctly identifies the circular dependency risk and prescribes the same solution
- No phantom here — the local AuditTx pattern is validated by research.md ✓

**pgx pool singleton (research.md §8 Risk 6, server.go:86-98):**
- Actual: single `pgStore` initialized at startup (PgWorkflowStore)
- Claimed: evidence store reuses the same pool via `postgres.go` wiring, no new pool creation
- Consistent with actual pattern. ✓

No phantom APIs found. All extension points are faithful to the existing patterns documented in research.md with file:line references.

---

### Criterion 5: Deferred Storage-Strategy Decision Is an Explicit Open Decision, Not an Unspecified Gap

**Verdict: PASS**

plan.md §6 OPEN DECISION:
- Status field: "UNRESOLVED — run/strategy 단계에서 결정"
- Self-declaration: "이는 미명세 갭이 아니라 문서화된 명시적 미결정"
- Tradeoff table: Filesystem vs DB BLOB vs self-hosted MinIO with pros, cons, and REQ-EVID-UBI-001 망분리 compatibility column
- Decision gates: two mandatory constraints any chosen strategy must satisfy (external call 0, rollback consistency)
- plan.md §9 DoD item: "§6 OPEN DECISION이 run/strategy 단계로 명시 전달 (미명세 갭 아님)"

spec.md §5 Exclusion #3: "[Deferred to run/strategy phase — plan.md §6 OPEN DECISION]"
spec-compact.md §Deferred: complete restatement with decision gates

The deferred decision is documented at multiple levels with decision gate criteria, tradeoff analysis, and explicit status. A run-phase implementer encountering this will find a structured decision framework, not a gap. ✓

---

## D1/D2 Resolution Verification Summary

| Defect | Prior description | v0.1.1 status | Verified independent? |
|--------|-------------------|---------------|----------------------|
| D1 | REQ-EVID-001-O1 had no dedicated AC; §6 catalog falsely claimed AC-EVID-001-1 | AC-EVID-001-O1-1 added (acceptance.md:169-191); §6 catalog corrected | YES — AC text read and verified |
| D2 | spec.md §2.2 vs plan.md §8 DB path inconsistency; 0002 filename collision unresolved | Both files now use same path convention; 0002 conflict explicitly resolved | YES — both sections read and compared |

Both defects genuinely resolved. No relabeling or rationalization — the corrections are substantive.

---

## New Defects Found

**D3 [Minor]** — plan.md line 3: header version reference stale

`plan.md` line 3: `> 대응 SPEC: `.moai/specs/SPEC-AX-EVID-001/spec.md` v0.1.0`

plan.md §8 content was updated as part of the D2 fix (the `0002_evidence_tables.sql` conflict resolution paragraph is present). However, the header version pointer was not bumped from `v0.1.0` to `v0.1.1`. This is a cosmetic inconsistency — the plan content reflects v0.1.1 changes, only the self-description is stale.

Severity: Minor (cosmetic). Does not affect implementation correctness or any must-pass criterion. Run-phase implementers reading plan.md will find correct content in the body; the header line is the only stale element.

---

## Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| EARS Compliance + REQ→AC Traceability | 97/100 | PASS | All 16 REQs correctly EARS-formatted; all including Optional REQ-EVID-001-O1 have dedicated ACs; D1 resolved |
| Internal Consistency | 90/100 | PASS | AC counts (19) consistent across all 4 files; D2 path inconsistency resolved; plan.md header version stale (D3, minor) |
| Scope/Exclusion Completeness + Testability | 95/100 | PASS | 9 exclusions with named targets; no weasel words; p99/n=10 minor imprecision (inherited from v0.1.0, noted by prior auditor) |
| No Phantom APIs | 98/100 | PASS | All 5 claimed extension points mirror research.md real signatures; Recorder/AuditTx pattern confirmed |
| Deferred Decision Documented | 100/100 | PASS | plan.md §6 explicit OPEN DECISION with tradeoff table, decision gates, DoD item |

---

## Verdict

**Cross-Validation: AGREE-PASS**

The prior plan-auditor's PASS verdict at 0.88 is upheld. D1 and D2 are genuinely corrected in v0.1.1:
- D1: AC-EVID-001-O1-1 is a real, substantive AC with proper Given/When/Then covering the Optional duplicate-signal scenario including both the active-mode and inactive-mode assertions
- D2: The path inconsistency is resolved with consistent `initial.sql` / `migrations/NNNN_*.sql` convention in both spec.md and plan.md, and the 0002 filename conflict is explicitly closed

One new minor defect found (D3: plan.md header version reference stale at v0.1.0). This does not affect must-pass criteria and does not warrant overturning PASS.

No critical or blocking defects. No phantom APIs. No unspecified gaps. The SPEC is ready for Run phase.

Recommended action before Run: fix plan.md line 3 from `v0.1.0` to `v0.1.1` (1-line cosmetic correction, non-blocking).
