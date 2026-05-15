# SPEC Review Report: SPEC-AX-SERVER-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.62

> Reasoning context ignored per M1 Context Isolation. Audited only spec.md / acceptance.md / plan.md plus actual codebase cross-reference. Adversarial stance (M2) active: assumed defects present, disproved via evidence.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ modules are `REQ-SERVER-UBI-001` (sub-clauses a/b/c) + `REQ-SERVER-001..004` (spec.md:L112-186). No gaps, no duplicates, consistent zero-padding (`001`-`004`). Sub-IDs (`-S1/-E1/-E2/-U1/-U2`) consistent. Exactly 5 modules (4 numbered + 1 UBI) — within "max 5 + UBI".
- [PASS] MP-2 EARS format compliance: Every requirement matches a valid EARS pattern. Ubiquitous: UBI-001-a "The system SHALL record..." (L116), REQ-SERVER-001-S1 (L124). State-driven: UBI-001-b "WHILE server is in startup phase..." (L117), UBI-001-c (L118), 003-S1 (L164), 004-S1 (L182). Event-driven: 001-E1/E2, 002-E1/E2/E3, 003-E1/E2, 004-E1/E2/E3. Unwanted: 001-U1/U2, 002-U1/U2, 003-U1, 004-U1. No informal/Given-When-Then mislabeling in §3.
- [PASS] MP-3 YAML frontmatter validity: All 8 canonical fields present with correct types — `id: SPEC-AX-SERVER-001`, `version: 0.1.0`, `status: draft`, `created: 2026-05-15`, `updated: 2026-05-15`, `author: ircp`, `priority: high`, `issue_number: 0` (spec.md:L2-9). Verified against canonical 8-field schema at `.claude/skills/moai/workflows/plan.md:L378` (`id, version, status, created, updated, author, priority, issue_number`). No `labels`/`created_at` defect raised — those fields are NOT in the canonical schema (lesson #1 honored; spec.md:L16 Schema note correct).
- [N/A] MP-4 Section 22 language neutrality: Single-language project (Go control-plane only; Python pipelines explicitly deferred to SPEC-AX-SERVER-PY-001 at spec.md:L92). No multi-language tooling enumeration required. Auto-pass.

No must-pass firewall failure. The FAIL verdict is driven by a Critical cluster of spec-to-code integrity defects (M5 firewall is not the only fail path; an unimplementable requirements spine fails the audit).

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 | Requirements individually unambiguous and precisely worded (spec.md:L116-186). Minor: wiring contract internally precise but externally false (phantom APIs). |
| Completeness | 0.75 | 0.75 | All sections present (HISTORY L12, WHY §1.2, WHAT §1, REQUIREMENTS §3, AC acceptance.md, Exclusions §5 = 10 entries). Frontmatter complete. Minor: EARS-pattern completeness claim inaccurate (D7). |
| Testability | 0.50 | 0.50 | Multiple ACs assert behavior of methods that do not exist in the codebase (AC-SERVER-002-E1/E2/U1/U2, AC-SERVER-UBI-001-b/c, AC-SERVER-004-E2*). A tester cannot reach binary PASS without inventing APIs — violates the SPEC's own "calling code only / 비즈니스 로직 0줄" scope (spec.md:L69, plan.md:L12). |
| Traceability | 0.75 | 0.75 | Intra-document traceability sound: every REQ has ≥1 AC, every AC references a valid REQ ID. Spec-to-code traceability broken (init-order steps map to non-existent constructors/methods). |

Weighted mean ≈ 0.69; capped at **0.62** because the SPEC's central mechanism (dependency wiring — the sole stated purpose) cannot be implemented as written.

## Defects Found

D1. spec.md:L117,L150 / acceptance.md:L116,L122,L156,L257 — **CRITICAL** — REQ-SERVER-UBI-001-b step (c) and REQ-SERVER-002-U1 require `pgStore.Ping()`. Actual `apps/control-plane/internal/store/pg_store.go` `PgWorkflowStore` exposes only `NewPgWorkflowStore(ctx, dsn, logger)`, `Close()`, `BeginTx()`, `PoolStats()`, `ListWorkflows()`. **There is no `Ping()` method.** AC-SERVER-002-E1/U1 and AC-SERVER-004-E2-DBDown are not implementable against the real store API.

D2. spec.md:L117 step (d), L144, L158, L176 — **CRITICAL** — SPEC assumes `redis.NewClient()` + `redisClient.Ping()` + `redisClient.Close()`. No standalone Redis client constructor/wrapper exists in scope. `scheduler/dispatcher.go` only consumes a `RedisClient` interface (`RPush`, `Ping`); `auth/refresh.go` `RedisRefreshStore` (`NewRedisRefreshStore(addr)`) has neither `Ping()` nor `Close()`. The readiness check `redisClient.Ping(ctx).Err()` (spec.md:L176) and reverse-cleanup `redisClient.Close()` reference an object that does not exist as specified.

D3. spec.md:L117 step (f), L142, acceptance.md:L40 — **CRITICAL** — REQ-SERVER-UBI-001-b names steps `token_validator` / `refresh_token_store` via `auth.NewTokenValidator()` / `auth.NewRefreshTokenStore()`. Actual `internal/auth/validator.go` has **no `NewTokenValidator`** — the constructor is `auth.New(ctx, oidcIssuer, audience, opts...)`. `RefreshTokenStore` is an interface; concrete types are `NewRedisRefreshStore(addr)` / `NewRefreshService(...)` — **no `NewRefreshTokenStore`**. AC-SERVER-UBI-001-b's exact captured-slice token names (`"token_validator","refresh_token_store"`) and REQ-SERVER-002-E1's `stepName` enum are wrong.

D4. spec.md:L152,L176,L178,L144 / acceptance.md:L160-170,L254 — **CRITICAL** — REQ-SERVER-004-E2 (iii) / E3 require `oidcClient.JWKSReachable(ctx)`; REQ-SERVER-002-E2 cleanup calls `oidcClient.Close()`. Actual `internal/auth/oidc.go` `OIDCClient` exposes only `NewOIDCClient(ctx, issuerURL, opts...)` and `GetMetadata()`. **No `JWKSReachable` and no `Close()` method.** The `/ready` "oidc" check and JWKS-warmup-failure cleanup are unimplementable as written.

D5. spec.md:L144 — **MAJOR** — REQ-SERVER-002-E2 reverse-cleanup explicitly calls `celeryDispatcher.Close()` and `tokenValidator.Close()`. `scheduler/dispatcher.go` `CeleryDispatcher` has only `SetBuilder/Dispatch/BuildEnvelope` (no `Close`); `validator.go` `TokenValidator` has only `Verify` (no `Close`). AC-SERVER-002-E2/U2 cleanup-spy assertions cannot pass against real types.

D6. spec.md:L117 step (g) — **MAJOR** — Init order specifies `workflow.NewStateMachine()` with no preceding `TxCoordinator` step. Actual `internal/workflow/state_machine.go` requires `NewStateMachine(coordinator *TxCoordinator, logger)`. The dependency-injection order (the SPEC's core deliverable) omits `TxCoordinator` entirely and would not compile as ordered; AC-SERVER-UBI-001-b captured slice is missing this dependency.

D7. spec.md:L108 — **MINOR** — Claims "EARS 5개 패턴(Ubiquitous / Event-driven / State-driven / Optional / Unwanted) 모두 본 SPEC 내 포함." No standalone Optional ("Where the [feature] ..., the system SHALL ...") requirement exists; AuthEnabled-conditional behavior is embedded inside Unwanted (U2) / State (UBI-001-c) clauses. The "all 5 patterns" claim is inaccurate (4 patterns present). Documentation accuracy defect, not an MP-2 failure.

D8. spec.md:L142 — **MINOR** — REQ-SERVER-002-E1 states "each initialization step that returns an error SHALL be wrapped" but the `stepName` enum begins at `pg_store` (config/logger excluded), and actual `config.Load()` returns `*Config` with no error. Narrative vs. enum scope mismatch; harmless but imprecise.

## Chain-of-Verification Pass

Second-look findings (re-read every REQ end-to-end + every constructor via grep, not spot-check):
- Re-verified REQ numbering sequentially L112→L186: confirmed no gap/dup (first pass held).
- Re-verified traceability: every REQ-SERVER-* has ≥1 AC in acceptance.md §1-6; every AC cites a valid REQ ID. No orphan AC, no uncovered REQ — intra-doc PASS confirmed.
- Re-checked Exclusions §5: exactly 10 specific entries, each with named follow-up SPEC (≥7 target met) — confirmed, not vague.
- New defect surfaced on second pass: **D6** (missing `TxCoordinator` in init order) — not caught in first pass which focused on method-existence; found by cross-checking `NewStateMachine` signature. Added above.
- Cross-SPEC re-check: SERVER-001 §2.1:L81 correctly cites the REAL chain.go path `internal/auth/chain.go` and calls `auth.BuildRESTChain` — note this DIVERGES from AUTH-002 spec.md's stale claim of `internal/server/chain.go` with `auth.Validator` interface. SERVER-001 matches actual code (package `auth`, `validator *TokenValidator`, 4-arg signature) — this is CORRECT, not a defect. `auth.BuildRESTChain(mux, validator, recorder, authEnabled)` (spec.md:L130,L146) and `BuildGRPCInterceptorChain(validator, recorder, authEnabled)` signatures MATCH `chain.go:L43-49,L86-90`.
- AUTH-002 Exclusion #12 (AUTH-002 spec.md:L230) explicitly delegates to SPEC-AX-SERVER-001; SERVER-001 HISTORY L14 + §6:L230-231 + plan.md §4 correctly assert formal resolution as "unblock fact only, AUTH-002 file unmodified" — cross-SPEC narrative integrity PASS.

## Cross-SPEC + Spec-to-Code Completeness

- **Cross-SPEC (document level): SOUND.** chain.go signature compatible (verified). CTRL-001 `NewWorkflowService(store.WorkflowStore, *workflow.StateMachine, logger)` / `NewRESTHandler(*WorkflowService, logger)` / `RESTHandler.Mux()` all exist and SPEC usage matches. AUTH-002 Exclusion #12 unblock correctly specified and cited.
- **Spec-to-Code (calling APIs): BROKEN.** 4 Critical + 2 Major defects: `pgStore.Ping`, redis client+Ping/Close, `NewTokenValidator`/`NewRefreshTokenStore`, `oidcClient.JWKSReachable`/`Close`, `celeryDispatcher.Close`/`tokenValidator.Close`, missing `TxCoordinator`. The dependency-wiring spine — the SPEC's entire raison d'être — is built on phantom APIs.
- Bootstrap order logical sequence (config→store→redis→oidc→validator→sm→dispatcher→service→chain→listeners) is conceptually correct but uncompilable as written (D3/D6).
- Graceful-shutdown race analysis (REQ-SERVER-003-E2/S1/U1 + plan.md Risk Register `Stop()` vs `GracefulStop()`, `sync.Once`, `time.AfterFunc`, double-signal) is adequately analyzed.

## Schema Decisions

8-field canonical schema CONFIRMED against `.claude/skills/moai/workflows/plan.md:L378`. Frontmatter compliant. No false positive on `labels`/`created_at` (lesson #1 applied; spec.md:L16 + plan.md:L238 + acceptance.md:L350 Schema notes are correct rebuttals and were honored, not disputed). MP-3 PASS.

## Recommendation

FAIL. Fix the spec-to-code integrity cluster before re-submission. Required actions for manager-spec:

1. **D1 (spec.md:L117,L150,L176; acceptance.md:L116,L156,L257,L264):** Replace `pgStore.Ping()` with a real readiness mechanism. Either (a) specify a new `Ping(ctx)` method as an explicit S0 pre-req deliverable in `pg_store.go` (like the existing config-field chore at spec.md:L82), or (b) use existing `PoolStats()`/`BeginTx()`+`Rollback()` as the liveness probe and rewrite the affected REQ/AC accordingly.
2. **D2 (spec.md:L117 step d, L144, L158, L176):** Define exactly how the Redis client is constructed (concrete type, constructor, `Ping`/`Close` methods) as an S0 deliverable, or reuse an existing wrapper. Currently no such object exists; the readiness/cleanup/init steps are unanchored.
3. **D3 (spec.md:L117 step f, L142; acceptance.md:L40):** Correct constructor names — `auth.New(ctx, oidcIssuer, audience, opts...)` (not `NewTokenValidator`) and `NewRedisRefreshStore(addr)` / `NewRefreshService(...)` (not `NewRefreshTokenStore`). Update REQ-SERVER-002-E1 `stepName` enum and AC-SERVER-UBI-001-b captured-slice strings to the real symbols.
4. **D4 (spec.md:L152,L176,L178,L144):** `OIDCClient` has no `JWKSReachable`/`Close`. Either add them as explicit S0 pre-req deliverables on `oidc.go`, or redefine the `/ready` "oidc" check (e.g., `GetMetadata()` non-nil) and remove `oidcClient.Close()` from cleanup.
5. **D5 (spec.md:L144):** Remove `celeryDispatcher.Close()` / `tokenValidator.Close()` from REQ-SERVER-002-E2 reverse cleanup, or specify them as new methods to be added (S0). Align AC-SERVER-002-E2/U2 spies accordingly.
6. **D6 (spec.md:L117 step g):** Add `TxCoordinator` construction to the init order before `NewStateMachine`, and reflect it in AC-SERVER-UBI-001-b's captured slice and REQ-SERVER-002-E1 `stepName` enum.
7. **D7 (spec.md:L108):** Either add a genuine Optional ("Where ...") EARS requirement (e.g., "Where `cfg.AuthEnabled` is true, the system SHALL perform JWKS warm-up...") or correct the claim to "4 EARS 패턴 포함 (Optional 미해당)".
8. **D8 (spec.md:L142):** Align the REQ-SERVER-002-E1 narrative ("each initialization step") with the `stepName` enum scope (which starts at `pg_store`), or note config/logger are infallible.

Adopt the existing S0 pre-req-chore pattern (spec.md:L82,L227,L232; plan.md S0:L47-64) for any genuinely new methods (Ping/Close/JWKSReachable) so the wiring spine is anchored to real, deliverable APIs before S1.
