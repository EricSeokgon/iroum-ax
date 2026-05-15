# SPEC Review Report: SPEC-AX-AUTH-002
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.74

Reasoning context ignored per M1 Context Isolation. This audit forms independent conclusions from spec.md / plan.md / acceptance.md / spec-compact.md and cross-referenced source/SPEC files only.

## Must-Pass Results

- [FAIL] MP-1 REQ number consistency: DUPLICATE identifier `REQ-AUTH2-001-U1` used for TWO different requirements with conflicting semantics.
  - spec.md:L116 `REQ-AUTH2-001-U1` labeled "Ubiquitous", states "The system SHALL define method/path → Permission mapping in code (Go source), not in runtime configuration file."
  - spec.md:L145 `REQ-AUTH2-001-U1` labeled "Default-deny safety net" under `#### Unwanted`, states "IF the incoming method+path ... is NOT defined ... THEN the system SHALL return HTTP 503 ...".
  - Both share identical REQ ID under the same section §3.2 REQ-AUTH2-001. Acceptance.md references "REQ-AUTH2-001-U1" ambiguously (acceptance.md:L103, L141, L228) — traceability is corrupted.
  - Likely intent: first should be `REQ-AUTH2-001-U` or numbered sub-clause (e.g., `REQ-AUTH2-001-U0`), second `REQ-AUTH2-001-X1` (Unwanted in EARS = "X" prefix not "U"). Per EARS conventions "U" prefix typically denotes Ubiquitous; using it for Unwanted compounds the confusion.

- [PASS] MP-2 EARS format compliance: All ACs reviewed against EARS patterns. Modal requirements use proper Event-driven (WHEN/THEN), State-driven (WHILE), Unwanted (IF/THEN), and Ubiquitous (SHALL) forms.
  - Evidence: spec.md:L108 "WHILE AuthEnabled=false ... the system SHALL bypass" (State-driven). spec.md:L120 "WHEN an HTTP request arrives ... THEN the system SHALL resolve" (Event-driven). spec.md:L145 "IF the incoming method+path ... THEN the system SHALL return HTTP 503" (Unwanted). spec.md:L169 "WHILE info.FullMethod == ... THE interceptor SHALL bypass" (State-driven). All 5 EARS patterns present per §3 claim (spec.md:L100).

- [PASS] MP-3 YAML frontmatter validity: 8-field canonical schema per `.claude/skills/moai/workflows/plan.md` Phase 2 line ~378. All required fields present at spec.md:L1-L10: `id` (SPEC-AX-AUTH-002), `version` (0.1.0), `status` (draft), `created` (2026-05-15), `updated` (2026-05-15), `author` (ircp), `priority` (high), `issue_number` (0). Schema note at spec.md:L16 correctly documents canonical schema lineage. `labels` and `created_at` are NOT part of canonical schema and are CORRECTLY absent — no false-positive raised (per audit prompt directive + lessons_session_2026_05_14 #1).

- [N/A] MP-4 Section 22 language neutrality: SPEC is scoped to Go control-plane (`apps/control-plane/internal/server/`) — explicitly single-language. Python FastAPI RBAC wiring deferred to follow-up `SPEC-AX-AUTH-PY-001` (spec.md:L75, L240). Auto-passes.

## Category Scores

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.70 | 0.50–0.75 band | REQ-AUTH2-001-U1 duplicate ID (spec.md:L116 vs L145) introduces ambiguous reference — reasonable engineer cannot determine which REQ a downstream AC maps to. Otherwise well-bounded language; no weasel words in normative text. |
| Completeness | 0.90 | 0.75–1.0 band | All required sections present: HISTORY (L12), §1 개요 (WHY/Context), §2 영향받는 파일 (WHAT/scope), §3 EARS 요구사항, §4 NFR, §5 Exclusions (9 entries — exceeds floor of 8), §6 의존성/전제, §7 Out of Scope, §8 검증 방법. 8-field frontmatter complete. |
| Testability | 0.85 | 0.75–1.0 band | 23 ACs total (acceptance.md:L427). Binary-testable predicates: HTTP status codes, exact audit row counts (`정확히 1건`, `0건`), `details.required=delete:workflow`, `WWW-Authenticate` header text. Edge cases enumerated (acceptance.md §6 L389). Minor: AC-AUTH2-004-3 admits 501/404/200 ambiguity but explicitly limits assertion to "not 403" (acceptance.md:L345) — defensible. |
| Traceability | 0.75 | 0.75–1.0 band | acceptance.md headers explicitly state "REQ 대응: REQ-AUTH2-XXX" per AC. Every modal REQ has ≥1 AC. REQ-AUTH2-UBI-001 sub-clauses a/b/c each map to dedicated AC (acceptance.md:L15, L45, L62). However, AC-AUTH2-001-2 (acceptance.md:L103) and AC-AUTH2-001-5 (L141) both reference "REQ-AUTH2-001-U1" — and due to MP-1 duplicate, traceability target is ambiguous. AC-AUTH2-002-6 (L228) references "REQ-AUTH2-001-U1 + AC-AUTH-004-5" — same ambiguity. Effectively 3 ACs have broken traceability. |

## Defects Found

D1. spec.md:L116 + spec.md:L145 — **DUPLICATE REQ ID `REQ-AUTH2-001-U1`** under §3.2 REQ-AUTH2-001. First instance is Ubiquitous (code-as-config invariant); second is Unwanted (default-deny safety net). Three ACs (acceptance.md:L103/L141/L228) cite "REQ-AUTH2-001-U1" with ambiguous resolution. — Severity: **critical** (MP-1 firewall failure).

D2. plan.md:L81, L111 + spec.md:L69 — **Wiring target file `apps/control-plane/cmd/server/server.go` is a 40-line stub** containing only `Server.Run() error { logger.Info("server stub — not yet implemented"); return nil }` (verified via Read). Plan claims chain composition `auth.RESTMiddleware(validator)(server.RESTAuthzMiddleware(recorder)(handler.Mux()))` and `grpc.ChainUnaryInterceptor(...)` will be added "한 줄" / "소규모 수정". But there is **no `grpc.NewServer()`, no `http.ListenAndServe`, no Mux mounting** in `cmd/server/server.go`. The SPEC's dependency assumption "SPEC-AX-CTRL-001 GREEN gRPC + REST 모두 GREEN" (spec.md:L226) is **contradicted by codebase state** — CTRL-001 left server bootstrap as a stub (verified by file content + comment "Sprint 0 스켈레톤 — 실제 구현은 Sprint 7(T-AX-006) 예정" at server.go:L2). This means SPEC-AX-AUTH-002 Sprint S1/S2 has hidden prerequisite work (implement actual gRPC + REST bootstrap) not enumerated in plan.md. Token budget estimate (plan.md:L196-L203 750-1000K) is therefore unreliable. — Severity: **critical** (Scope/Risk hidden; will trigger Re-planning Gate during Run phase per spec-workflow.md).

D3. plan.md:L141 + spec.md:L183 — **AC-AUTH2-004-3 / REQ-AUTH2-004-E3 contradiction**. REQ text (spec.md:L183) says "the system SHALL return HTTP 200 (or 204), delete the workflow, AND insert WORKFLOW_DELETED audit row" — but the same line then states "DELETE 핸들러 자체는 본 SPEC 범위 외; ... 501/404 가능, 단 403은 아니어야 함". The REQ contains a **SHALL** assertion (delete the workflow) that the SPEC itself disclaims as out-of-scope, then weakens to "not 403". Per EARS, a SHALL must be testable as written; mixing SHALL semantics with "DELETE 핸들러 부재로 501/404 가능" violates Manage Confusion HARD rule (moai-constitution.md §2). AC at acceptance.md:L344-L347 correctly narrows to "403이 아님" but the REQ wording remains contradictory. — Severity: **major**.

D4. spec.md:L131 + spec-compact.md:L37 — **`/metrics` bypass is security-sensitive but un-justified**. Mapping table states `GET /metrics → bypass` with parenthetical "Prometheus scrape — 내부망 한정, 운영 정책 후속". This embeds an unverified network-level assumption (intranet-only) into a code-level immutable matrix (REQ-AUTH2-001 declares "operational hot-reload 금지"). If deployment exposes `/metrics` externally (likely in cloud staging), this is an unauthenticated information disclosure path (memory layout, request paths, internal metric labels). No AC verifies `/metrics` cannot be reached from outside trust boundary. Should either (a) require authentication, or (b) Exclusion entry making external exposure explicit boundary, or (c) AC asserting Prometheus token check. — Severity: **major** (security).

D5. spec.md:L70 + plan.md:L137 — **Cross-SPEC artifact mutation under-specified**. S3 modifies AUTH-001's `auth_e2e_test.go` and `acceptance.md` §6 AC-AUTH-E2E-3 status. Plan.md:L186 acknowledges this. Cross-reference confirmed: AUTH-001 acceptance.md does contain `t.Skip("SPEC-AX-AUTH-002: RBAC REST handler wiring deferred ...")` at line 370 of `auth_e2e_test.go` (verified). However, NO AC in SPEC-AX-AUTH-002 asserts the exact mechanical edit (e.g., "after S3, `grep -c 'SPEC-AX-AUTH-002: RBAC REST handler wiring deferred' apps/control-plane/internal/server/auth_e2e_test.go == 0"). DoD bullet (acceptance.md:L420-L421) is informal checkbox, not a Given/When/Then AC. — Severity: **minor** (improvement opportunity).

D6. plan.md:L171 + spec.md:L107-L110 — **REQ-AUTH2-UBI-001 transactional atomicity claim is unverifiable**. spec.md:L108 says deny path "동일 transaction 내에서 commit하여 atomicity 보장". AC-AUTH2-UBI-001-a (acceptance.md:L19-L31) asserts the audit row exists after 403, but does NOT prove same-transaction commit (e.g., simulating tx rollback mid-LogForbidden and asserting no orphan audit row). For a security-critical thorough harness SPEC, atomicity should have a dedicated negative-path AC. — Severity: **minor**.

D7. spec.md:L165 + acceptance.md:L292 — **Interceptor chain order not enforced at compile time**. REQ-AUTH2-003-E1 prescribes chain order `auth.UnaryServerInterceptor → server.UnaryAuthzInterceptor`. AC-AUTH2-003-6 (acceptance.md:L292-L301) verifies behavior with bad token reaching `Unauthenticated` first — but this is an indirect test (could pass even if order swapped, depending on `UserFromContext` returning ok=false → REQ-AUTH2-002-U2 500 path conflicts with REQ-AUTH2-002-U2 spec wording at spec.md:L159). The race condition described in plan.md R-AUTH2-002 (L169) deserves a dedicated AC asserting "if interceptors registered in wrong order, server bootstrap FAILS to start" (e.g., via runtime validation). — Severity: **minor**.

## Chain-of-Verification Pass

Second-pass re-read sections §3.1-§3.5 of spec.md, §0-§4 of acceptance.md, and plan.md Sprint S0-S3.

New findings on second pass:
- D1 (duplicate REQ ID) was confirmed by complete enumeration via grep: `REQ-AUTH2-001-U1` returns exactly 2 hits in semantic REQ-defining positions (spec.md:L116 and L145).
- D2 (stub server) was confirmed by direct Read of `cmd/server/server.go` (40 lines, no `grpc.NewServer`, no `ListenAndServe`).
- Additional check: §1.4 한국 공공 도메인 영향 평가 (spec.md:L48-L54) is present and addresses 6 constraints — no defect.
- Additional check: HISTORY section (spec.md:L12-L14) is single-version entry but acceptable for v0.1.0 first SPEC iteration.
- Additional check: §5 Exclusions enumerates 9 specific items (spec.md:L211-L219) — exceeds floor of 8, each entry has concrete description + deferral target. PASS.
- Additional check: NFR table (spec.md:L194-L204) includes performance + security + backward-compat dimensions, properly cross-referenced to AUTH-001 §4 — PASS.
- Confirmed `Authorize` signature in `apps/control-plane/internal/auth/rbac.go:100` matches `auth.Authorize(ctx, requiredPerm Permission) error` — SPEC's call site references are signature-compatible.
- Confirmed `LogForbidden` at rbac.go:118 takes `(ctx, tx audit.AuditTx, method, path, userID, grantedRoles)` — matches spec.md:L157 usage.
- Confirmed `ActionAuthForbidden` exists at `audit/audit.go:34` — spec.md:L71 reuse claim valid.
- Confirmed `RESTMiddleware` at `auth/middleware.go:149` and `UnaryServerInterceptor` at `auth/middleware.go:96` — both exist; chain composition documented in plan.md L81/L111.
- Confirmed `UserFromContext` at `auth/middleware.go:49` — SPEC's REQ-AUTH2-002-E1 call (spec.md:L151) is valid.

No additional critical defects discovered. D2 (stub server) is the most operationally dangerous — D1 is most procedurally critical (MP-1 firewall).

## Regression Check

Iteration 1 — not applicable.

## Cross-SPEC Integration Completeness

- AUTH-001 AC-AUTH-E2E-3 SKIP unblock: **EXPLICITLY ADDRESSED**. spec.md:L70 + plan.md:L138-L143 + acceptance.md AC-AUTH2-004-1 (L307-L326). Cross-reference verified: `auth_e2e_test.go:370` contains `t.Skip("SPEC-AX-AUTH-002: RBAC REST handler wiring deferred — see TODO above")`.
- AUTH-001 `rbac.go` `Authorize` signature compatibility: **VERIFIED** (rbac.go:100 matches spec usage).
- AUTH-001 `LogForbidden` signature compatibility: **VERIFIED** (rbac.go:118 matches spec usage).
- AUTH-001 `RESTMiddleware` chain composition: **DOCUMENTED** in plan.md:L81 — but actual mount site is stub (D2).
- AUTH-001 `UnaryServerInterceptor` chain composition: **DOCUMENTED** in plan.md:L111 — but actual mount site is stub (D2).
- CTRL-001 `grpc_server.go` + `rest_handler.go` entry points listed: **PARTIALLY**. spec.md:L67-L69 + §2.1 list `rest_handler.go` (no change) and `grpc_server.go` (미수정, interceptor 등록은 `cmd/server/server.go`). But CTRL-001's expected mounting in `cmd/server/server.go` is itself unimplemented (D2).
- Default-deny safety net: explicit (REQ-AUTH2-001-U1 second instance) + tested (AC-AUTH2-001-2, AC-AUTH2-001-5) — except D1 ID collision.
- AuthEnabled=false backward compat: explicit (REQ-AUTH2-UBI-001-b) + 3 dedicated ACs (UBI-001-b, AC-AUTH2-003-5, regression invariant in §5 acceptance.md:L381).

## Schema Decisions

Canonical 8-field schema per `.claude/skills/moai/workflows/plan.md` Phase 2 ~L378 applied. No false positives raised on `labels` or `created_at` — both correctly absent. Schema note at spec.md:L16 references the canonical lineage and lessons_session_2026_05_14 #1, which I respected.

## Recommendation

FAIL — return to manager-spec for the following fixes, in priority order:

1. **(D1 — critical, blocks MP-1)** Rename one of the two `REQ-AUTH2-001-U1` occurrences. Suggested: keep spec.md:L116 as `REQ-AUTH2-001-U1` (Ubiquitous, code-as-config); rename spec.md:L145 to `REQ-AUTH2-001-X1` (Unwanted/eXception). Update all AC references in acceptance.md (L103, L141, L228) to the new ID. Update plan.md and spec-compact.md cross-references. Re-verify traceability after rename.

2. **(D2 — critical, hidden scope risk)** Either (a) add a Sprint S-1 (prerequisite) that implements the real `cmd/server/server.go` bootstrap (grpc.NewServer registration + http.Server + handler.Mux mount + interceptor chain wiring), OR (b) explicitly upgrade SPEC-AX-AUTH-002's "dependency" entry (spec.md:L225-L227) to state "Requires SPEC-AX-CTRL-001 Sprint 7 bootstrap implementation as PRE-prerequisite; if Sprint 7 not GREEN, SPEC-AX-AUTH-002 is blocked". The current claim "CTRL-001 GREEN" is factually wrong against the codebase (verified `cmd/server/server.go` is 40-line stub).

3. **(D3 — major)** Rewrite REQ-AUTH2-004-E3 (spec.md:L183) to remove conflicting SHALL clauses. Suggested: "WHEN a request `DELETE /api/v1/workflows/{id}` arrives with a valid `iroum-ax:admin` token AND AuthEnabled=true, THEN the system SHALL pass the request through the RBAC authorization layer without returning HTTP 403 Forbidden. (Downstream handler behavior is out of scope and tested in SPEC-AX-WF-DELETE-001.)"

4. **(D4 — major, security)** Either add an Exclusion entry (#10) explicitly noting "external `/metrics` exposure assumed prevented by network ACL — verification deferred to operations runbook, not in-process check", OR add a REQ requiring authenticated Prometheus scrape, OR add an AC asserting `/metrics` exposure boundary.

5. **(D5/D6/D7 — minor)** Add explicit ACs for (a) post-S3 SKIP-marker removal grep assertion, (b) audit-row transactional atomicity (force tx rollback mid-LogForbidden), (c) compile-time/bootstrap-time interceptor chain order validation.

Re-audit on iteration 2 will verify all five recommendations are addressed and that the D1 duplicate is removed (MP-1 must pass before overall PASS is achievable).

## Report file

/home/sklee/moai/iroum-ax/.moai/reports/plan-audit/SPEC-AX-AUTH-002-review-1.md
