# SPEC Review Report: SPEC-AX-SERVER-001
Iteration: 2/3
Verdict: FAIL
Overall Score: 0.78

> Reasoning context ignored per M1 Context Isolation. The prior-iteration defect summaries supplied in the spawn prompt were used ONLY as a regression checklist; every claim re-verified against the 9 actual source files (read directly) plus spec.md / acceptance.md. Adversarial stance (M2) active.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: `REQ-SERVER-UBI-001` (sub-clauses a/b/c) + `REQ-SERVER-001..004`, sequential, no gap/dup, consistent padding (spec.md:L133-220).
- [PASS] MP-2 EARS format compliance: All §3 requirements match valid EARS patterns. UBI-001-a Ubiquitous (L137), 001-S1 Ubiquitous (L158); UBI-001-b/c, 003-S1, 004-S1 State-driven; *-E* Event-driven; *-U* Unwanted. The corrected 4-pattern claim (L129) is now accurate. No Given/When/Then mislabeled in §3.
- [PASS] MP-3 YAML frontmatter validity: 8 canonical fields present, correct types; `version: 0.1.1` correctly bumped (spec.md:L2-9). Verified vs canonical 8-field schema. No `labels`/`created_at` false positive (lesson #1 honored; spec.md:L16-17 rebuttal correct).
- [N/A] MP-4 Section 22 language neutrality: single-language Go control-plane; Python deferred to SPEC-AX-SERVER-PY-001 (spec.md:L113). Auto-pass.

No must-pass firewall failure. FAIL is driven by one residual MAJOR spec-to-code integrity defect in the central wiring spine (same defect class as iteration 1, now reduced to a single step).

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 | Requirements precise and individually unambiguous (L133-220); one false interface-satisfaction claim (L79) misleads implementation. |
| Completeness | 0.85 | 0.75-1.0 | All sections present; S0 helper ACs added (acceptance.md:L13-37); Exclusions = 10 specific entries (L246-255). |
| Testability | 0.75 | 0.75 | ACs now assert verified APIs (pg_store.Ping S0, JWKSCache.Reachable S0); AC-SERVER-UBI-001-b captured slice + AC-SERVER-002 still implicitly assume raw `*redis.Client` injection into dispatcher (uncompilable as written). |
| Traceability | 0.90 | 0.75-1.0 | Every REQ has ≥1 AC; every AC cites valid REQ; S0 ACs added. Minor: acceptance.md:L390 §8 stale "5-pattern" cross-reference. |

Weighted mean ≈ 0.81; capped at **0.78** because step (h) of the dependency-wiring spine — the SPEC's core deliverable — remains uncompilable as written (D9).

## Defects Found

D9. spec.md:L79, L146 (step h), L266; acceptance.md:L72,L290 — **MAJOR** — spec.md §2.0:L79 asserts "`*redis.Client`는 `scheduler.RedisClient` 인터페이스(`RPush`+`Ping`) 충족". This is FALSE against actual code. `scheduler.RedisClient` (dispatcher.go:24-29) requires `RPush(ctx,key,...) (int64,error)` and `Ping(ctx) error`. go-redis/v9 `*redis.Client.RPush` returns `*redis.IntCmd` and `.Ping` returns `*redis.StatusCmd` — it does NOT satisfy the interface directly. The codebase already PROVES an adapter is mandatory: `internal/server/e2e_test.go:172,198-208` defines `goRedisAdapter{client *redis.Client}` with `RPush→.Result()` / `Ping→.Err()`. spec.md cites `e2e_test.go:42` (the import line) but missed the adapter at L198. Step (h) `scheduler.NewCeleryDispatcher(redisClient, ...)` passing a raw `*redis.Client` would not compile. (Raw-client `Ping`/`Close` for readiness/cleanup ARE valid — that part of D2 is resolved.)

D11. spec.md:L98 — **MINOR** — S0 deliverable `JWKSCache.Reachable(ctx) bool` is specified as "`c.lastSuccessAt`가 zero가 아니고 `c.cacheAge() < c.staleMaxAge`" but omits that `cacheAge()` (jwks_cache.go:212-213) documents "호출자가 mu.RLock을 보유해야 한다". `Reachable` must acquire `c.mu.RLock()` before reading `lastSuccessAt`/calling `cacheAge()`. Feasible but under-specified for the Run phase.

D12. acceptance.md:L390 (§8) — **MINOR** — Still references "EARS 5-pattern 분류" while spec.md:L129 was correctly amended to 4 patterns (Optional N/A). Stale cross-reference; D7 fix not propagated to acceptance.md §8.

## Iteration-1 Defect Resolution Status (Regression Check)

- D1 `pgStore.Ping` — **RESOLVED**: pg_store.go confirmed NO `Ping()` (only Close/BeginTx/PoolStats/ListWorkflows; `pool.Ping` used only in constructor L50). S0 scope-add (spec.md:L97) is structurally feasible and genuinely required by readiness + startup abort. AC-SERVER-S0-PgPing added (acceptance.md:L13-23). Justified.
- D2 redis client — **PARTIALLY RESOLVED**: raw `redis.NewClient`/`Ping`/`Close` are real (go.mod confirms `redis/go-redis/v9 v9.19.0`); readiness/cleanup aspects fixed. BUT false interface-satisfaction claim remains → see D9 (MAJOR).
- D3 auth constructors — **RESOLVED**: validator.go:157 `New(_ ctx, oidcIssuer, audience string, opts...) (*TokenValidator, error)`; refresh.go:406 `NewRedisRefreshStore(addr) *RedisRefreshStore`; refresh.go:96 `NewRefreshService(...)`. spec.md step (f):L144 + acceptance.md:L74 match.
- D4 oidc/jwks — **RESOLVED**: oidc.go confirmed `OIDCClient` has only `NewOIDCClient`+`GetMetadata()` (no JWKSReachable/Close); spec.md:L78 correctly excludes OIDCClient from cleanup (stateless). JWKSCache.Reachable S0 feasible (jwks_cache.go has lastSuccessAt/staleMaxAge/cacheAge). Minor residual D11.
- D5 cleanup — **RESOLVED**: TokenValidator(only Verify), OIDCClient(only GetMetadata), JWKSCache(GetKey/refresh), CeleryDispatcher(SetBuilder/Dispatch/BuildEnvelope) confirmed NO Close. spec.md:L178,L192 + acceptance.md:L170,L220 now only `redisClient.Close()` → `pgStore.Close()`. Phantom Close calls removed.
- D6 TxCoordinator — **RESOLVED**: transaction.go:26 `NewTxCoordinator(store.WorkflowStore, *audit.Recorder)`; state_machine.go:63 `NewStateMachine(*TxCoordinator, *zap.Logger)`; recorder.go:46 `NewRecorder(bool)`. spec.md step (g):L145 recorder→tx_coordinator→state_machine + acceptance.md:L72-73 captured slice match exactly.
- D7 EARS 4-pattern — **RESOLVED** (spec.md:L129); minor residual D12 in acceptance.md §8.
- D8 enum/narrative scope — **RESOLVED**: spec.md:L140-141,L176 mark config.Load()/logger.New() infallible & outside enum (starts at `pg_store`); acceptance.md:L159 aligned.

Resolution: 7 of 8 fully resolved; D2 partially resolved with one residual MAJOR (D9). Progress 8 defects (4C/2M/2m) → 1 Major + 2 minor. Substantial progress, NOT stagnation.

## Chain-of-Verification Pass

Second-look (re-read all 9 source files end-to-end, not spot-check):
- Re-verified REQ numbering L133→L220: no gap/dup confirmed.
- Re-verified intra-doc traceability: every REQ-SERVER-* has ≥1 AC; every AC cites valid REQ; S0 ACs cover both new helpers. No orphan AC, no uncovered REQ.
- Re-verified non-redis wiring interface satisfaction: `*PgWorkflowStore` implements `store.WorkflowStore` (BeginTx L70 + ListWorkflows L288 vs store.go:18 interface) — SOUND for `NewTxCoordinator`/`NewWorkflowService`. `chain.go` `recorder auditRecorder` param vs `*audit.Recorder` is AUTH-002 GREEN territory, explicitly scoped out (spec.md:L101) — not a SERVER-001 defect.
- New defect surfaced on second pass: **D9** (redis adapter gap) — found by cross-checking `scheduler.RedisClient` interface signature against go-redis v9 command return types and the existing `goRedisAdapter` at e2e_test.go:198. Iteration-1 framed D2 as object non-existence; iteration-2 introduced a different, false interface-satisfaction claim. Added above.
- Exclusions §5 re-checked: 10 specific entries each with named follow-up SPEC — not vague.

## Recommendation

FAIL. One MAJOR + two MINOR defects. The wiring spine still has one uncompilable step. Required actions for manager-spec (iteration 3 — targeted, small):

1. **D9 (spec.md:L79, step h L146, §6:L266; acceptance.md:L72,L290):** Correct the false claim. `*redis.Client` does NOT satisfy `scheduler.RedisClient` directly. Specify the adapter explicitly — the pattern already exists at `internal/server/e2e_test.go:198-208` (`goRedisAdapter`: `RPush→.Result()`, `Ping→.Err()`). Either (a) declare a small adapter as an S0/cmd-local deliverable and inject `&goRedisAdapter{client: redisClient}` into `scheduler.NewCeleryDispatcher` at step (h), or (b) reuse/promote the existing test adapter. Update §2.0 row, step (h), and AC-SERVER-UBI-001-b captured-slice rationale accordingly. Fix the `e2e_test.go:42` citation to point at the adapter (`:198`).
2. **D11 (spec.md:L98):** Add to the S0 `Reachable(ctx) bool` deliverable note that the method MUST acquire `c.mu.RLock()` before reading `lastSuccessAt`/calling `cacheAge()` (per jwks_cache.go:212 contract).
3. **D12 (acceptance.md:L390):** Update §8 to "EARS 4-pattern (Optional 미해당)" to match spec.md:L129.

Routing: This is iteration 2 of 3. Defect set reduced 8 → 1 Major + 2 minor — major progress, not a blocking/stagnant defect. Recommend **ONE targeted iteration 3** (manager-spec) to close D9/D11/D12 (D9 is a ~2-line spec correction against an adapter that already exists in the repo). Do NOT escalate to user and do NOT hand to evaluator yet — re-audit after iteration 3; the SPEC is close to PASS once the dispatcher injection is corrected.
