# SPEC Review Report: SPEC-AX-OBS-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.78

> Reasoning context ignored per M1 Context Isolation. Audit based solely on spec.md / acceptance.md / plan.md and cross-referenced source code.

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: Domain-prefixed scheme (consistent with sibling SPEC-AX-AUTH-002 `REQ-AUTH2-*`). Functional REQs sequential with no gaps/duplicates: `REQ-OBS-001` (spec.md:L156), `REQ-OBS-002` (L180), `REQ-OBS-003` (L198), `REQ-OBS-004` (L214) + `REQ-OBS-UBI-001` (L147). 4 functional + 1 UBI = 5, within "max 5 + UBI". Sub-IDs (`-S1/-E1/-U1/-a..d`) unique throughout. No duplicate IDs.
- **[PASS] MP-2 EARS format compliance**: All 5 patterns present and well-formed. Ubiquitous: L151 "The system SHALL expose metrics exclusively…"; L160 "The system SHALL define a single…". Event-driven: L174 "WHEN the metrics package is initialized… THEN…". State-driven: L190 "WHILE `cfg.AuthEnabled=true`, THE /metrics RBAC wrapper SHALL…". Optional: L218 "WHERE `cfg.OTelEnabled=true` AND `cfg.OTLPEndpoint != ''`…". Unwanted: L178 / L194 "IF … THEN the system SHALL return HTTP 401…". Each AC is binary-testable.
- **[PASS] MP-3 YAML frontmatter validity**: 8-field project canonical schema verified against `.claude/skills/moai/workflows/plan.md` L378 (`id, version, status, created, updated, author, priority, issue_number`). All 8 present with correct types (spec.md:L1-10). Per lesson #1 + Schema note L16 + plan.md §7, `labels`/`created_at` are NOT canonical for this project — no false-positive raised.
- **[N/A] MP-4 Section 22 language neutrality**: Single-language scoped SPEC. Explicitly Go control-plane only; Python FastAPI deferred to `SPEC-AX-OBS-PY-001` (spec.md:L118, L295). `prometheus/client_golang`, `go.opentelemetry.io/otel` are Go project dependencies, not multi-language tooling. Auto-pass.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band | One/two requirements need interpretation: §3.4 REQ-OBS-003-E2 gRPC wiring phrasing imprecise vs §2.1 (L106); /metrics auth path underspecified (L190-194). |
| Completeness | 0.75 | 0.75 band | All sections present (HISTORY L12, WHY §1.2, WHAT §1, REQUIREMENTS §3, AC, Exclusions §5 with 10 entries). One substantive gap: authentication wiring for the chain-bypassed /metrics path absent. |
| Testability | 1.0 | 1.0 band | Every AC binary-testable; no weasel words. Concrete assertions (HTTP codes, exposition format, `git diff rbac.go` empty L25, goleak L152, bench p99 L168). |
| Traceability | 0.75 | 0.75 band | Every REQ has ≥2 AC; no orphan ACs. But AC-OBS-002-1/4 (admin→200 / viewer→403) are NOT implementable as specified due to D1 (no token→User population on bypass path), so the mapping is broken in practice for REQ-OBS-002-S1. |

## Defects Found

**D1. spec.md:L184-194 / acceptance.md:L59-81 — `/metrics` RBAC wrapper authentication wiring gap — Severity: major**
The SPEC mounts `GET /metrics` on `outerMux` *outside* `auth.BuildRESTChain` (mirroring `/health`/`/ready` — verified `cmd/server/server.go:L184-195`). `UserFromContext` is populated *only* by `auth.RESTMiddleware` (verified `internal/auth/middleware.go:L195` `next.ServeHTTP(w, r.WithContext(WithUser(...)))`), which lives *inside* `BuildRESTChain`. Because `/metrics` bypasses that chain, no component parses the Bearer token into `*auth.User` on the `/metrics` path. REQ-OBS-002-S1 (L190) says "extract `*auth.User` via `auth.UserFromContext`" and AC-OBS-002-1 (L61) expects "valid admin token → 200", but `UserFromContext` will always return `(nil, false)` on this path → every request (even valid admin) yields 401. The SPEC only specifies *authorization* (`IsMetricsAuthorized`) and never specifies the *authentication* step (token validation → context) for the bypass path. REQ-OBS-002-U1 note (L194) acknowledges the bypass ("`auth.RESTMiddleware` 우회 경로 방어") but does not prescribe how the wrapper authenticates. The /metrics RBAC wrapper must explicitly compose `auth.RESTMiddleware` (or an equivalent `TokenValidator` step) before `IsMetricsAuthorized`; this is unspecified in §3.3, §2.1 (L105-106), and plan.md S1 (L41-42).

**D2. spec.md:L204 vs L106 — gRPC outermost-interceptor wiring inconsistent/imprecise — Severity: minor**
REQ-OBS-003-E2 (L204) prescribes `grpc.ChainUnaryInterceptor(metricsInterceptor, authChain...)`, implying `authChain` is a spread of interceptors. But `auth.BuildGRPCInterceptorChain` returns an opaque `grpc.ServerOption` (verified `internal/auth/chain.go:L86-101`), not interceptors — the literal §3.4 expression is a type mismatch. §2.1 (L106) gives the correct feasible approach ("`auth.BuildGRPCInterceptorChain` 결과 앞", i.e. pass the metrics `ChainUnaryInterceptor` ServerOption before the auth ServerOption; gRPC v1.81.0 accumulates `chainUnaryInts` across multiple options, preserving `[metrics, authn, authz]`). Intent ("metrics outermost") is unambiguous and feasible, but §3.4 and §2.1 disagree on mechanism — implementer following §3.4 literally hits a compile error.

**D3. acceptance.md:L3 / spec.md:L255 / acceptance.md:L165 — AC count discrepancy — Severity: minor**
Stated "Total AC: 18" (acceptance.md:L3, spec.md:L255, DoD L165 "AC-OBS-* 18건"). Actual REQ-mapped ACs = 19 (UBI a/b/c/d=4 + OBS-001=3 + OBS-002=5 + OBS-003=4 + OBS-004=3), plus 3 EDGE = 22. The stated total is wrong. Does not affect traceability (every REQ still ≥2 AC) but the count claim is inaccurate and propagated into DoD.

## Chain-of-Verification Pass

Second-look findings: D1 was discovered ONLY in the second pass — the first pass accepted Option B's frozen claim and the chain-external mount at face value; re-reading `middleware.go` to trace *who populates UserFromContext* exposed that the authentication step is absent on the bypass path. This is a genuine spec-to-code defect of the same class the audit task flagged (AUTH-002 evaluator pattern: spec describes a behavior the verified code structure cannot deliver). D2 severity was revised down from major to minor after verifying gRPC v1.81.0 `chainUnaryInts` accumulation semantics (`internal/auth/chain.go` + go.mod `grpc v1.81.0`). Re-read sections to confirm thoroughness: §3 all REQ entries (end-to-end, not skimmed), §5 Exclusions 1-10 (all named/specific, not vague), cross-SPEC texts (AUTH-002 L231, SERVER-001 L251/L252) read verbatim — exact match to SPEC §1.1/§6 claims. No intra-document requirement contradictions found beyond D2.

## Option B Spec-to-Code Verification (core check)

- **rbac.go frozen preservation: VERIFIED PASS.** `internal/auth/rbac.go:L39-60` `permissionMatrix` contains only workflow/recommendation/audit perms for admin/analyst/viewer — NO `read:metrics`. Option B (`internal/metrics/permission.go` + `IsMetricsAuthorized(roles)` admin-only) is fully decoupled from `permissionMatrix`. AC-OBS-UBI-001-c (`git diff rbac.go` empty, L25) is achievable. The AUTH-002 evaluator-caught pattern (rbac.go matrix + handler simultaneous change → spec-code contradiction) is genuinely avoided.
- **authz_mapping.go compatibility: COMPATIBLE (with caveat).** Appending one `restEntry{GET,/metrics,read:metrics,bypass=false}` to `restPermissionTable` is safe against the linear-scan `LookupRESTPermission` (`authz_mapping.go:L81-113`) — exact-match, unique prefix, no shadowing. Caveat: since `/metrics` is chain-external, this entry is consulted only by `authz_mapping_test.go` (AC-OBS-002-2), not production routing — the SPEC's "default-deny 503 회피" justification (L186) is over-stated but harmless (not a defect, noted for clarity).

## Cross-SPEC Unblock Completeness

COMPLETE and text-accurate. AUTH-002 §5 Exclusion #13 (verified `.moai/specs/SPEC-AX-AUTH-002/spec.md:L231`: "Prometheus `/metrics` endpoint… 후속 SPEC `SPEC-AX-OBS-001` 또는 `SPEC-AX-METRICS-001`") matches spec.md §1.1/§6 verbatim. SERVER-001 §5 Exclusion #4 (L251 Distributed tracing/OTel → SPEC-AX-OBS-001) and #5 (L252 Prometheus /metrics → SPEC-AX-OBS-001) match exactly. Upstream-file modification correctly scoped out (separate chore).

## Schema Decisions

8-field canonical schema correctly applied; no `labels`/`created_at` false positive raised (lesson #1 honored). Source authority chain verified (workflows/plan.md L378).

## Recommendation (actionable for manager-spec)

1. **(D1, blocking)** Specify the authentication step for the chain-bypassed `/metrics` path. Either: (a) explicitly state the OBS RBAC wrapper composes `auth.RESTMiddleware(s.tokenValidator)` (or a token→`WithUser` step) *before* `IsMetricsAuthorized` so `UserFromContext` is populated; or (b) mount `/metrics` inside a minimal authn-only chain. Update §3.3 REQ-OBS-002-S1/U1 (spec.md:L190-194), §2.1 (L105-106), and plan.md S1 (L41-42) so AC-OBS-002-1 (admin→200) and AC-OBS-002-4 (viewer→403) are implementable against the verified code structure.
2. **(D2)** Reconcile §3.4 REQ-OBS-003-E2 (spec.md:L204) with §2.1 (L106): replace the imprecise `grpc.ChainUnaryInterceptor(metricsInterceptor, authChain...)` with the feasible "pass `grpc.ChainUnaryInterceptor(metricsInterceptor)` as a ServerOption *before* `auth.BuildGRPCInterceptorChain(...)`'s ServerOption" (gRPC accumulates `chainUnaryInts` in option order).
3. **(D3)** Correct the AC total to 19 (or 22 incl. edge) in acceptance.md:L3, spec.md:L255, DoD acceptance.md:L165.

Re-submit for iteration 2 after D1 is resolved (D1 alone blocks PASS — a must-pass-class spec-to-code gap that makes REQ-OBS-002 acceptance unachievable as written).
