# SPEC Review Report: SPEC-AX-AUTH-001

Iteration: 1/3
Verdict: **PASS** (with 1 Major + 5 Minor advisories for refinement before Run phase)
Overall Score: 0.88
Harness: thorough

Reasoning context ignored per M1 Context Isolation. Audit conducted from spec.md / acceptance.md / plan.md / research.md / spec-compact.md only. Cross-reference reads: SPEC-AX-001/spec.md, SPEC-AX-CTRL-001/spec.md, SPEC-AX-CTRL-001/acceptance.md, SPEC-AX-CTRL-001/research.md.

---

## Must-Pass Results

- **[PASS] MP-1 REQ Number Consistency**: 6 modules (REQ-AUTH-UBI-001, REQ-AUTH-001, REQ-AUTH-002, REQ-AUTH-003, REQ-AUTH-004, REQ-AUTH-005). Sequential 001→005 + UBI module. No gaps, no duplicates. Verified spec.md:L148, L150, L168, L178, L192, L210. Sub-IDs (E1/E2/S1/U1/U2) uniformly named.

- **[PASS] MP-2 EARS Format Compliance**: All 5 EARS types present —
  - Ubiquitous: spec.md:L148 ("The system SHALL propagate...")
  - Event-driven: spec.md:L154, L156, L172, L182, L184, L186, L214
  - State-driven: spec.md:L148 (WHILE clause), L160, L196
  - Optional: spec.md:L176 ("WHERE OIDC_DISCOVERY_REFRESH_INTERVAL is set..."), L182 (WHERE health check)
  - Unwanted: spec.md:L164, L166, L190, L208, L218
  All 5 patterns covered; minor advisories tracked as D2/D6 (supplementary non-EARS prose embedded in REQ-AUTH-UBI-001 and REQ-AUTH-005-E1).

- **[PASS] MP-3 YAML Frontmatter Validity**: spec.md:L1–10 canonical 8-field schema per `.claude/skills/moai/workflows/plan.md` Phase 2: id, version, status, created, updated, author, priority, issue_number — all present with correct types. Schema note documented at spec.md:L16. No false positive on `labels`/`created_at` (not in canonical schema).

- **[N/A] MP-4 Section 22 Language Neutrality**: Project is Go+Python composite (apps/control-plane + pipelines), explicitly scoped. Language tooling (golang-jwt/jwt v5, PyJWT 2.8) is project-specific, not template-bound multi-language. Auto-passes.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75–1.0 | spec.md:L148–218 normative REQs use precise quantifiers (30-second skew, 3600s TTL, RS256/EdDSA/ES256 allow-list, p99 thresholds). Minor: D2/D6 supplementary prose. |
| Completeness | 0.95 | 1.0 band | HISTORY (L12), 8-field frontmatter, §1–8 all present, Exclusions 15 entries (L246–260), Open Questions 5 + sensible defaults (research.md:L367–397), §6 dependencies (L266–273). |
| Testability | 0.90 | 0.75–1.0 | acceptance.md:L8 forbids weasel words; 36 ACs all G/W/T; performance ACs quantified (acceptance.md:L501–509); edge case catalog 17 entries (L515–533). Minor: D8 missing dedicated RBAC perf AC. |
| Traceability | 0.95 | 1.0 band | Every REQ→AC mapped (acceptance.md:L18, L35, L52, L70, L88, L130, L156, etc. "REQ 대응" markers); every AC traces to valid REQ-AUTH-XXX. spec-compact.md:L50–61 AC count table matches acceptance.md DoD §9 (4+11+3+6+6+4+2=36). |

---

## Audit Checklist Detail

### Group 1: YAML Frontmatter — ALL PASS
- FC-1 id "SPEC-AX-AUTH-001" matches SPEC-{DOMAIN}-{NUM} (composite AX+AUTH); spec.md:L2
- FC-2 version "0.1.0"; spec.md:L3
- FC-3 status "draft"; spec.md:L4
- FC-4 created "2026-05-14" ISO date; spec.md:L5
- FC-5 priority "high"; spec.md:L8
- FC-6 labels: N/A in canonical schema; SUPERSEDED by canonical 8-field per project lesson.

### Group 2: Document Structure — ALL PASS
- SC-1 HISTORY: spec.md:L12 + L14 single 0.1.0 entry (initial)
- SC-2 WHY: §1.2 운영 컨텍스트 (Why now) L32–40
- SC-3 WHAT: §1 개요 + §1.4 Walking Skeleton L48–54
- SC-4 REQUIREMENTS: §3 EARS L142–218
- SC-5 ACCEPTANCE CRITERIA: acceptance.md 36 ACs across §0–§6
- SC-6 Exclusions: §5 L246–260, 15 specific entries (target ≥7, exceeded by 8)

### Group 3: Requirements Quality
- RQ-1 PASS: Sequential 001–005 + UBI; no gaps
- RQ-2 PASS: No duplicate REQ-IDs across all 6 modules
- RQ-3 PASS: Requirements express behavior/outcome (e.g., "reject the request with HTTP 401", "propagate the verified `sub` claim"). Note: spec.md:L182 mentions concrete API names (`UnaryServerInterceptor`, `auth.WithUser(ctx, user)`) which lean toward HOW — borderline; acceptable in TDD plan context but flagged as D5 advisory.
- RQ-4 PARTIAL: Library names (golang-jwt/jwt v5, coreos/go-oidc, python-jose, PyJWT) appear in §6 dependencies (L271–272) — that section is appropriate; library names also appear in REQ-AUTH-001-E1/E2 text only as protocol indicators, not function signatures. PASS.
- RQ-5 PASS: No "should"/"may"/"reasonable" in normative text (other than REQ-AUTH-002-O1 which correctly uses MAY for Optional EARS pattern).

### Group 4: Acceptance Criteria Quality — ALL PASS
- AC-1 PASS: Each REQ maps to G/W/T ACs in EARS-traceable form (acceptance.md "REQ 대응:" markers)
- AC-2 PASS: All ACs binary-testable (e.g., "HTTP 401", "byte-identical", "p99 < 5ms")
- AC-3 PASS: No weasel words (verified search for "적절히", "신속하게", "reasonable", "appropriate")
- AC-4 PASS: Every AC references a valid REQ-AUTH-XXX
- AC-5 PASS: Every REQ-AUTH-XXX has at least one dedicated AC (UBI: 4, 001: 11, 002: 3, 003: 6, 004: 6, 005: 4)

### Group 5: Language Neutrality — N/A
- LN-3: SPEC scoped to Go (apps/control-plane) + Python (pipelines). Not multi-language template. N/A auto-pass.

### Group 6: Consistency
- CN-1 PASS: No requirement contradictions found
- CN-2 PASS: Exclusions (§5) do not conflict with included REQs — e.g., Excl #4 (SAML) consistent with OIDC-only scope; Excl #11 (ABAC) consistent with role-only RBAC in REQ-AUTH-004
- CN-3 PARTIAL: Priority/labels consistent with stated scope. Minor: D3 file-table inconsistency between spec.md §2.1 and plan.md S5 (`refresh_family.go`).

---

## Defects Found

### D1 (Major) — File-targeting mismatch for Celery envelope user_id propagation
- **Location**: spec.md:L79 says `apps/control-plane/internal/scheduler/dispatcher.go` is modified for `headers.user_id` propagation.
- **Conflict**: plan.md:L205 (S6) and spec-compact.md:L33 correctly target `apps/control-plane/internal/scheduler/celery_envelope.go`.
- **Cross-SPEC fact**: SPEC-AX-CTRL-001/spec.md:L66–67 and L94–95 establish that the envelope JSON builder is `celery_envelope.go`; `dispatcher.go` performs RPUSH only. The envelope `headers.user_id` field belongs in `celery_envelope.go`.
- **Severity**: Major — implementer reading spec.md §2.1 affected files table will edit the wrong file. Golden file `celery_envelope_v2.json` regeneration is anchored to the envelope builder, not dispatcher.
- **Recommendation**: Edit spec.md:L79 to target `celery_envelope.go` (envelope builder modification) and optionally add a separate row noting that `dispatcher.go` passes context-derived `user_id` to the envelope builder. Cross-check against SPEC-AX-CTRL-001 affected-files table.

### D2 (Minor) — REQ-AUTH-UBI-001 trailing sentence is not in EARS form
- **Location**: spec.md:L148 final sentence: "토큰 만료 / 서명 실패 / 재사용 시 즉시 거부하고, 거부 이벤트는 `audit_logs`에 `AUTH_REJECTED` 액션으로 기록되어야 한다(거부도 추적 가능)."
- **Issue**: This Korean assertive prose is appended to the EARS Ubiquitous+State-driven clauses but is itself not in any EARS pattern. The intent maps to an Unwanted REQ ("IF token expired/signature-failed/replayed, THEN the system SHALL reject and record AUTH_REJECTED in audit_logs").
- **Severity**: Minor — semantics are unambiguous, the EARS-conformant clauses earlier in the same REQ cover most of the intent; AC-AUTH-UBI-001-B operationalizes the audit on rejection.
- **Recommendation**: Split into REQ-AUTH-UBI-001 (Ubiquitous + State-driven only) and an explicit Unwanted clause, OR remove the trailing sentence since rejection behavior is fully specified by REQ-AUTH-001-* and AC-AUTH-UBI-001-B already covers the audit recording.

### D3 (Minor) — `refresh_family.go` missing from spec.md §2.1 affected files
- **Location**: plan.md:L198 (S5) lists `apps/control-plane/internal/auth/refresh_family.go (신규)`. spec-compact.md:L29 also lists `refresh_family.go`.
- **Conflict**: spec.md:L66–80 §2.1 Go Control Plane table lists 8 modules in `internal/auth/` but omits `refresh_family.go`.
- **Severity**: Minor — Run phase will catch this since plan.md is the implementation contract, but spec.md should be authoritative for affected-files inventory.
- **Recommendation**: Add row to spec.md §2.1: `apps/control-plane/internal/auth/refresh_family.go` | Refresh token family tracking + Lua script atomic invalidation | REQ-AUTH-005 | 신규.

### D4 (Minor) — REQ-AUTH-005-E1 embeds a non-EARS conditional in trailing sentence
- **Location**: spec.md:L214 final sentence: "If either token is malformed, return HTTP 400 (do not partial-blacklist)."
- **Issue**: This is an embedded conditional in English, not extracted as a separate Unwanted REQ. AC-AUTH-005-4 (acceptance.md:L454–460) operationalizes the behavior but the source REQ is mid-sentence.
- **Severity**: Minor — testability preserved via AC-AUTH-005-4.
- **Recommendation**: Promote to a separate REQ-AUTH-005-U2 "IF either logout token is malformed, THEN the system SHALL return HTTP 400 and SHALL NOT partial-blacklist either token (transaction atomicity)."

### D5 (Minor) — REQ-AUTH-003-E1 leaks implementation detail (function names) into EARS body
- **Location**: spec.md:L182 mentions `auth.WithUser(ctx, user)` API by name within the EARS Event-driven clause.
- **Issue**: EARS clauses should be WHAT/WHY, not HOW. Specific helper-function names belong in plan.md §3 (implementation mapping), not in the normative REQ body.
- **Severity**: Minor — does not affect testability; AC-AUTH-003-1 (acceptance.md:L260) references `auth.UserFromContext(ctx)` which is the appropriate test-level abstraction.
- **Recommendation**: Replace "via `auth.WithUser(ctx, user)`" with "via a context value carrying the validated user" in spec.md:L182.

### D6 (Minor) — Missing dedicated AC for RBAC scope-parse p99 < 1ms target
- **Location**: spec.md:L229 §4 NFR table specifies "성능 — RBAC 검사: scope 파싱 + 매트릭스 lookup p99 < 1ms". acceptance.md:§7 L506 row references the target but no dedicated AC-AUTH-004-Performance line exists; only AC-AUTH-001-Performance covers token verify.
- **Severity**: Minor — measurement method (benchmark) is implied; CI can be added in Run phase.
- **Recommendation**: Add `AC-AUTH-004-Performance` paralleling AC-AUTH-001-Performance, with Given/When/Then and `go test -bench=BenchmarkAuthorizeRBAC` evidence path.

---

## Chain-of-Verification Pass

Second-look findings (re-read sections marked as quick-skim during first pass):

1. **Re-read REQ numbering end-to-end** (spec.md:L148–218): Confirmed 6 modules, no gaps, all sub-IDs (E1, E2, S1, U1, U2, O1) consistent. ✓
2. **Re-read all 36 ACs**: Confirmed REQ→AC traceability for every REQ. Specifically verified:
   - UBI-001 → 4 ACs (UBI-001-A/B/C/D) ✓
   - 001 → 10 + 1 perf ✓
   - 002 → 3 ✓
   - 003 → 6 ✓
   - 004 → 6 ✓
   - 005 → 4 ✓
   - E2E → 2 ✓
3. **Re-read Exclusions section** (spec.md:L246–260): 15 entries, each specific (not vague). External OAuth, MFA, password policy, SAML, 전자정부, impersonation, console UI, audit report, JWE, perm caching, ABAC, RLS, mTLS, end-session, RFC 7009. All concrete with rationale. ✓
4. **Cross-doc consistency check**: Discovered D1 (envelope file targeting), D3 (refresh_family.go missing from spec.md table). Both surfaced from second pass — first pass would have missed D1 if I had only read plan.md.
5. **Cross-SPEC integration verification**:
   - SPEC-AX-001 REQ-UBI-003 audit_logs.user_id (SPEC-AX-001/spec.md:L133): correctly referenced in spec.md:L24 and REQ-AUTH-UBI-001 ✓
   - SPEC-AX-CTRL-001 REQ-CTRL-UBI-002 (SPEC-AX-CTRL-001/spec.md:L108): correctly referenced in spec.md:L24, L29 ✓
   - SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C (SPEC-AX-CTRL-001/acceptance.md:L98, L137): correctly anchored as backward-compat baseline in AC-AUTH-UBI-001-C (acceptance.md:L63) and AC-AUTH-E2E-2 (acceptance.md:L496) ✓
   - SPEC-AX-CTRL-001 REQ-CTRL-005 Celery envelope (SPEC-AX-CTRL-001/spec.md:L196): cross-SPEC artifact regeneration explicitly flagged in plan.md:L205, research.md:L200 + L203, Open Question Q4 (research.md:L387–391) ✓
6. **Contradiction scan**: No requirements contradict each other. Exclusion §11 (ABAC) consistent with REQ-AUTH-004 role-only. Exclusion §13 (mTLS) consistent with JWT bearer-only auth. Exclusion §14 (RP-initiated logout) consistent with client-side blacklist-only logout in REQ-AUTH-005-E1.

First pass would have missed D1 and D3 (cross-document file-table inconsistencies). All other findings surfaced in first pass.

---

## Regression Check

Iteration 1 — no previous iteration to compare. N/A.

---

## Cross-SPEC Integration Completeness

- **SPEC-AX-001 REQ-UBI-003 user_id propagation**: ✓ Explicit reference in spec.md:L24 ("`audit_logs.user_id`까지 propagation"), reinforced by AC-AUTH-UBI-001-C byte-identical guarantee with REQ-UBI-003 + SPEC-AX-001 AC-UBI-004.
- **SPEC-AX-CTRL-001 REQ-CTRL-UBI-002-A/B**: ✓ Backward-compat regression guard via dedicated AC-AUTH-UBI-001-C (acceptance.md:L52–66) and tests/integration/test_auth_backward_compat.py (spec.md:L138, plan.md:L27, plan.md:L335). R-AUTH-007 explicit regression risk register entry.
- **SPEC-AX-CTRL-001 REQ-CTRL-005 envelope**: ✓ Cross-SPEC artifact regeneration (celery_envelope_v2.json) explicitly scheduled in plan.md S6 (L205), research.md Decision 3 (L200), Open Question Q4 (research.md:L387–391). **However see D1**: spec.md §2.1 file-table targets the wrong file (`dispatcher.go` vs `celery_envelope.go`).
- **SPEC-AX-001 §5 Exclusion #12 (SSO/JWT)**: ✓ Spec.md:L28 explicitly states "SPEC-AX-001 §5 Exclusion #12을 명시적으로 해소한다".
- **SPEC-AX-CTRL-001 §5 Exclusion #2 (Auth/SSO/JWT)**: ✓ Spec.md:L29 explicitly states "SPEC-AX-CTRL-001 §5 Exclusion #2를 명시적으로 해소한다".

Cross-SPEC integration substantively complete with one Major file-targeting issue (D1) to correct.

---

## Schema Decisions (No False Positives)

Canonical 8-field schema per `.claude/skills/moai/workflows/plan.md` Phase 2 verified at spec.md:L1–10. Per audit lesson from SPEC-AX-001 + SPEC-AX-CTRL-001 prior iterations, `labels` and `created_at` are NOT in canonical schema and have NOT been flagged as defects. spec.md:L16 schema note correctly documents schema lineage with SPEC-AX-001 / SPEC-AX-CTRL-001.

---

## Recommendation

This SPEC is approved for Run phase entry with one Major fix and five Minor refinements applied as a 0.1.1 amendment. The Run phase can proceed in parallel with the amendment since none of the defects block S0/S1/S2 starts — D1 affects S6 only.

### Required (Major — must fix before S6 begins):

1. **D1**: Edit spec.md:L79 to retarget `celery_envelope.go` for `headers.user_id` propagation. Add optional row for `dispatcher.go` as context-passing layer. Cross-check golden file regeneration against SPEC-AX-CTRL-001 `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json` baseline.

### Recommended (Minor — fold into 0.1.1 amendment):

2. **D2**: Refactor REQ-AUTH-UBI-001 trailing prose into an explicit Unwanted clause, or remove it (already covered by REQ-AUTH-001-* + AC-AUTH-UBI-001-B).
3. **D3**: Add `refresh_family.go` row to spec.md §2.1 Go Control Plane table (REQ-AUTH-005, 신규).
4. **D4**: Promote REQ-AUTH-005-E1 trailing malformed-token conditional to REQ-AUTH-005-U2.
5. **D5**: Remove function name `auth.WithUser(ctx, user)` from spec.md:L182 (move to plan.md implementation notes).
6. **D6**: Add `AC-AUTH-004-Performance` for RBAC scope-parse p99 < 1ms benchmark.

### Strengths Worth Preserving

- 4-way OIDC provider matrix (research.md:L86–98) is rigorous; 전자정부 deferral is well-justified with a named follow-up SPEC ID (SPEC-AX-AUTH-EGOV-001) — exemplary scope discipline.
- Backward-compat regression is doubly-guarded: dedicated AC-AUTH-UBI-001-C + AC-AUTH-E2E-2 + S0 baseline test + R-AUTH-007 risk register entry.
- 15-entry Exclusions list exceeds target by 8 and is specific (not vague), with named follow-up SPECs for major deferred items.
- Cognitive bias check section in research.md:L327–337 is exemplary — explicit anchoring/confirmation/sunk-cost/overconfidence checks rare in audited SPECs.
- 7-risk register with detection + mitigation + residual is comprehensive and ties each risk to a specific AC.
- Algorithm allow-list (RS256/EdDSA/ES256) + JWKS alg cross-check addresses OWASP JWT cheat sheet Algorithm Confusion Attack — flagged as Critical by R-AUTH-006.
