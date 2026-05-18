# SPEC-AX-EVID-001 — Phase 1 Strategy Plan (Run Workflow)

> Agent: manager-strategy | Mode: UltraThink | Methodology: TDD (quality.yaml `development_mode: tdd`) | Harness: thorough
> SPEC: `.moai/specs/SPEC-AX-EVID-001/spec.md` v0.1.1 | Status: draft → (run)
> [HARD] This is analysis & planning only. No code written in Phase 1.

---

## 0. Verified Reality (phantom-API prevention — source-confirmed)

All planning anchors below were confirmed against real code (not assumed):

| Symbol / Fact | Verified Location | Note |
|---|---|---|
| `WorkflowStore.BeginTx(ctx) (WorkflowTx, error)`, `ListWorkflows` | `apps/control-plane/internal/store/store.go:18-26` | Interface to mirror for `EvidenceStore` |
| `WorkflowTx` (`InsertWorkflow`, `InsertAuditLog`, `GetWorkflow` w/ FOR UPDATE, `Commit`, `Rollback`, ...) | `store.go:34-51` | Interface to mirror for `EvidenceTx` |
| `Recorder` struct `{authEnabled bool}`, `resolveUserID`, `parseResourceID` | `audit/recorder.go:38-67` | Extend with 2 methods |
| `RecordCreated(ctx, tx AuditTx, workflowID, documentID, userID string) error` | `audit/recorder.go:71` | Exact signature pattern to mirror |
| local `AuditTx interface { InsertAuditLog(ctx, *Event) error }` | `audit/recorder.go:28-31` | Reuse — store→audit cycle avoidance (R-EVID-001) |
| `Action string` + constants block | `audit/audit.go:13-54` | Add 2 constants here |
| `Event{Timestamp, Action, ResourceType, UserID, DetailsJSON []byte, ResourceID uuid.UUID}` | `audit/audit.go:59-66` | Reuse as-is |
| **REAL pgx pool**: `PgWorkflowStore{pool *pgxpool.Pool, logger}`; `NewPgWorkflowStore(ctx, dsn, logger)`; `BeginTx` uses `pgx.TxOptions{IsoLevel: pgx.ReadCommitted}` | `apps/control-plane/internal/store/pg_store.go:26-89` | **Single pool source of truth** |
| `PgWorkflowTx{tx pgx.Tx, logger}` with `InsertAuditLog` (audit_logs INSERT, JSONB details, auto uuid.New id) | `pg_store.go:96-275` | Mirror for `PgEvidenceTx` |
| Pool wired in server | `server.go:86` `store.NewPgWorkflowStore(ctx, cfg.PostgresDSN, logger)`; `server.go:172` `audit.NewRecorder(cfg.AuthEnabled)` | Reuse `s.pgStore` pool |
| `config.Config` + `Load()`, `getEnv`/`getBoolEnv`, `AuthEnabled` (`AUTH_ENABLED` default false) | `internal/config/config.go:12-103` | Add `EVIDENCE_*` env keys here |
| `audit_logs` columns (id, action, resource_type, resource_id, user_id, details JSONB, timestamp) | `pg_store.go:243-246` INSERT | Reuse schema unchanged |
| migrations dir = ONLY `0001_initial.sql` (311 bytes, idempotent pointer to `initial.sql`) | `.moai/db/schema/migrations/` | `0002_evidence_tables.sql` confirmed **non-colliding** |
| `internal/storage/` directory does NOT exist | filesystem check | `[NEW]` package, correct |

### [HARD] Phantom-API Correction (must propagate to Run)

spec.md §2.1, plan.md §1.1/§2 Sprint 1, research.md §1/§8 Risk 6 all say evidence TX wiring goes into **`postgres.go`** and call the pool owner the "PgWorkflowStore singleton in postgres.go". This is **inaccurate**:

- `internal/store/postgres.go` is a **Sprint-0 legacy stub** — `New(cfg Config) (*Store, error)` with `// TODO(Sprint 7): pgxpool.New()`, no real pool, no `BeginTx`. It is unrelated to the live path.
- The **real** pgx pool is `PgWorkflowStore` in **`pg_store.go`**, constructed by `NewPgWorkflowStore`, holding `pool *pgxpool.Pool`, wired at `server.go:86`.

**Resolution**: The implementation MUST add the evidence TX entry point to **`pg_store.go`** (or a new `pg_evidence.go` in package `store` sharing `PgWorkflowStore.pool`), NOT `postgres.go`. The single-pool-reuse intent (research.md §8 Risk 6 / R-EVID-005) is satisfied by reusing `PgWorkflowStore.pool`, not by touching the dead `postgres.go`. This is a documentation/path correction, not a scope or requirement change — no SPEC re-plan needed, but it is a named decision in the Human Gate below.

---

## 1. Philosopher Framework

### Phase 0 — Assumption Audit

| # | Assumption | Type | Confidence | Risk if wrong |
|---|---|---|---|---|
| A1 | SPEC-AX-CTRL-001 store/audit layer is GREEN and source-stable for the run | Hard constraint | High (verified `pg_store.go`, `recorder.go`, `store.go`, `audit.go` all present + coherent) | Low — confirmed by Read |
| A2 | Evidence TX entry point belongs in `pg_store.go`, not stub `postgres.go` | Hard constraint | High (verified `postgres.go` is no-op stub) | Medium if implementer follows SPEC literally → wires dead stub. Mitigated by §0 correction propagated to Run prompt |
| A3 | The storage-strategy decision (this deliverable) does NOT block the data-model Walking Skeleton | Hard constraint | High (plan.md §6.3 explicit; `EvidenceBlobStore` is interface-only this SPEC) | Low |
| A4 | `EVIDENCE_STORAGE_STRATEGY` is read at config load and persisted into `evidences.storage_strategy` per-row | Preference | Medium (env key named in spec.md §3.5-S1, not yet in config.go) | Low — additive config |
| A5 | p99 < 150ms NFR explicitly excludes blob physical-write latency (spec.md §4) | Hard constraint | High (spec.md §4 row 4 literal "파일 바이너리 물리 저장 latency 제외") | Low — strategy choice does not affect the measured path |
| A6 | testcontainers `postgres:16-pgvector` is available in CI (same as CTRL-001) | Hard constraint | High (existing `postgres_test.go` uses it) | Low |
| A7 | The PoC is single-node, air-gapped on-prem (KEPCO E&C 망분리) | Hard constraint | High (product.md §3.2, plan.md §6.2, memory anchor) | High if multi-node assumed → wrong storage pick. Drives the recommendation |

→ A2 and A7 are the load-bearing assumptions. Both are verified High. Proceeding.

### Phase 0.5 — First Principles (Five Whys: storage strategy)

- **Surface**: Which blob backend stores evidence file bytes?
- **Why 1**: Because evidence documents (편람, 작성지침, 실적보고서, 벤치마크) must be retained and version-chained.
- **Why 2**: Because 경영평가 audit/traceability requires immutable, retrievable proof artifacts.
- **Why 3**: Because KEPCO E&C must defend evaluation scores against 기획재정부 review with original evidence.
- **Why 4 (enabling)**: The PoC must demonstrate end-to-end evidence capture inside a 망분리 network with zero external dependency.
- **Root cause**: The Walking Skeleton's value is proving the *transactional integrity* of (evidence row + audit row) in an air-gapped PoC — the blob backend is an implementation detail that must not compromise that integrity or the data-sovereignty constraint.

**Constraint vs Freedom:**
- Hard constraints (non-negotiable): zero external network egress (REQ-EVID-UBI-001/004-U1); evidence row + audit_logs row atomic in one pgx TX (REQ-EVID-UBI-002, REQ-EVID-003-U1); no orphan blob on TX rollback; single-node PoC; 50 MiB max; no new external deps vs 데이터 주권.
- Soft constraints: minimal operational surface for PoC; mirror CTRL-001 patterns; abstraction shipped this SPEC, exactly one concrete impl recommended.
- Degrees of freedom: which of {filesystem, database_blob, minio} is the recommended concrete impl; write-ordering relative to the DB commit; orphan-cleanup mechanism.

### Phase 0.75 — Alternative Generation (storage strategy)

Three distinct candidates (this is the deferred OPEN DECISION — see §2 for full analysis):
- Conservative: **DB BLOB (BYTEA)** — natively transactional, zero new infra, single artifact to back up.
- Balanced: **Filesystem** (`/srv/evidence/`) — simple, fast, DB stays lean, needs write-ordering + orphan-cleanup story.
- Aggressive: **Self-hosted MinIO** — S3 API, multi-node ready, adds an operational service inside the 망분리 network.

### Cognitive Bias Check (applied to the recommendation in §2)

- **Anchoring**: research.md §9 lists filesystem first → do not let ordering bias the pick. Re-derived from constraints, not list order.
- **Confirmation bias**: actively constructed the failure case for the recommended option (§2.4 "Why the recommendation could be wrong").
- **Sunk cost**: none — greenfield blob layer, no prior impl to protect.
- **Overconfidence**: the recommendation is for the **PoC only**; the abstraction explicitly preserves the right to switch backends post-PoC without a schema change.

---

## 2. KEY DECISION — Storage-Strategy OPEN DECISION Resolution (Human Gate Decision Point 1)

> plan.md §6 deferred the blob backend choice (filesystem / DB BLOB / self-hosted MinIO) to this strategy phase. This section delivers the concrete recommendation. **This is the primary item requiring user sign-off.**

### 2.1 Binding constraints (all must hold)

1. **REQ-EVID-UBI-001 데이터 주권**: zero external service calls. External managed S3/CDN/secrets = permanently disqualified.
2. **REQ-EVID-UBI-002 + REQ-EVID-003-U1**: evidence row + audit_logs row commit/rollback atomically in ONE pgx TX. Blob handling must not create a partial/orphan state on rollback.
3. **PoC anchor**: KEPCO E&C 경영평가, 망분리 on-prem, single node (product.md §3.2). Single pgx pool reuse (`PgWorkflowStore.pool`, R-EVID-005). 50 MiB max file. p99 < 150ms excluding blob latency.
4. **Walking Skeleton priority**: data model first; `EvidenceBlobStore` interface + `storage_strategy` enum ship this SPEC; exactly ONE concrete impl recommended for PoC.

### 2.2 Trade-off Matrix (weighted, criteria confirmed against PoC priorities)

Weights chosen for an **air-gapped single-node PoC whose core value is transactional integrity**:
Transactional Atomicity 30% · Data Sovereignty / Operational Simplicity 25% · Implementation Cost 20% · Risk 15% · Scalability (post-PoC) 10%. (Scalability deliberately low — PoC is single-node; full data-sovereignty is a hard gate, scored within the 25% simplicity axis since all three pass the gate.)

Rated 1-10 (10 = best for this PoC):

| Criterion (weight) | DB BLOB | Filesystem | Self-hosted MinIO |
|---|---|---|---|
| Transactional Atomicity (0.30) | 10 — bytes are part of the same pgx TX; rollback is automatic, zero orphan risk | 4 — blob write is outside the DB TX; needs explicit write-ordering + orphan sweep | 4 — same as filesystem; network call to MinIO outside TX, orphan risk + egress surface |
| Data Sovereignty + Op Simplicity (0.25) | 9 — no new service, no new mount, single backup target; sovereignty trivially satisfied | 7 — no new service but needs a managed PV + backup story + path hygiene | 3 — new stateful service to deploy/operate/secure inside 망분리; largest sovereignty surface |
| Implementation Cost (0.20) | 8 — one BYTEA column + Put/Get over the same tx; smallest code delta | 6 — file IO + fsync + temp-then-rename + cleanup job | 3 — MinIO client wiring, bucket lifecycle, credential mgmt, deploy manifests |
| Risk (0.15) | 6 — PG WAL/table bloat at scale; bounded here by 50 MiB cap + PoC volume | 6 — orphan files, partial writes, multi-node PV later | 4 — extra failure domain, ext-egress misconfig risk (REQ-EVID-004-U1) |
| Scalability post-PoC (0.10) | 4 — heavy at high volume/large files | 6 — fine single-node, weak multi-node | 9 — designed for scale-out |
| **Weighted total** | **8.45** | **5.45** | **3.85** |

DB BLOB: 0.30·10 + 0.25·9 + 0.20·8 + 0.15·6 + 0.10·4 = **8.45**
Filesystem: 0.30·4 + 0.25·7 + 0.20·6 + 0.15·6 + 0.10·6 = **5.45**
MinIO: 0.30·4 + 0.25·3 + 0.20·3 + 0.15·4 + 0.10·9 = **3.85**

### 2.3 RECOMMENDATION → `database_blob` (DB BLOB / PostgreSQL BYTEA) for the PoC

**Rationale (decisive factors):**
1. **Atomicity is the Walking Skeleton's core value** (Five Whys root cause). DB BLOB makes the blob write *part of the same pgx transaction* as the `evidences` row and the `audit_logs` row. REQ-EVID-UBI-002 / REQ-EVID-003-U1 (all-or-nothing) become **structurally guaranteed** with zero extra orphan-cleanup machinery. Filesystem and MinIO both put the blob write *outside* the DB TX, forcing a write-ordering protocol + an orphan-sweep job — net new complexity that directly fights the SPEC's central invariant.
2. **데이터 주권 by construction**: bytes live in the already-internal PostgreSQL the system depends on. Zero new network endpoints, zero new DNS, zero REQ-EVID-004-U1 exposure surface. The single pgx pool (R-EVID-005) is the only resource touched.
3. **Lowest implementation cost & risk for a single-node PoC**: one `BYTEA` storage column, `EvidenceBlobStore.Put/Get` implemented over the active `EvidenceTx` — no PV provisioning, no MinIO deploy, no credential management, no cleanup cron.
4. **Bounded blast radius**: the known DB-BLOB weakness (table/WAL bloat, slow large-object scans) is capped by the 50 MiB file limit (REQ-EVID-001-U1) and PoC-scale volume, and is explicitly *outside* the p99<150ms measured path (spec.md §4 excludes blob latency).

### 2.4 Rejected-options rationale

- **Self-hosted MinIO — rejected for PoC.** Highest operational and sovereignty surface: a new stateful service to deploy, secure, back up, and monitor *inside the 망분리 network* — directly contradicts "minimal operational surface for an air-gapped PoC". External managed S3 is *permanently* disqualified by REQ-EVID-UBI-001; only self-hosted is even a candidate, and it still adds a service dependency with no PoC-scale payoff. Reconsider only post-PoC if multi-node horizontal scale becomes a real requirement (the abstraction makes this a non-breaking switch).
- **Filesystem — rejected for PoC (viable but inferior here).** Simple and fast, but the blob write lives outside the DB transaction. To honor REQ-EVID-003-U1 it requires either (a) write-blob-then-commit with a compensating delete on commit failure, or (b) commit-then-write with a forward-cleanup of rows whose blob never landed — plus an orphan-file sweeper for crash windows. That machinery is exactly the complexity the DB-BLOB choice eliminates. Keep as the documented fallback if DB bloat becomes a measured problem during the PoC (switch is a `storage_strategy` value + a new `EvidenceBlobStore` impl, no schema migration).

### 2.5 Why the recommendation could be wrong (bias check, stated explicitly)

- If real evidence volume / file sizes in the KEPCO E&C corpus skew large and numerous, PG table+WAL bloat could degrade backup/restore time and overall DB health sooner than "PoC-scale" assumes. Mitigation: the abstraction + `storage_strategy` enum let us switch to filesystem with no model change; recommend a post-PoC volume review checkpoint.
- DB BLOB couples evidence binary lifecycle to the primary OLTP database; a future retention/purge feature (explicitly excluded §5 #5) would operate on table rows rather than cheap file deletes. Acceptable for the PoC, flagged for the retention SPEC.

### 2.6 Concrete implications for the Run implementation plan

Adopting `database_blob` makes the following **binding** for Run:

1. **DDL addition** (`0002_evidence_tables.sql`): add a nullable `file_content BYTEA` column to the `evidences` table (plan.md §3 DDL currently omits it — must be added so the BYTEA strategy has a home; remains NULL-tolerant so a future filesystem strategy that stores only `storage_location` still validates). `storage_strategy` default in config = `'database_blob'`.
2. **Config** (`internal/config/config.go`, additive): `EVIDENCE_STORAGE_STRATEGY` (getEnv, default `"database_blob"`), `EVIDENCE_MAX_FILE_BYTES` (default `52428800` = 50 MiB), `EVIDENCE_DUPLICATE_SIGNAL_ENABLED` (getBoolEnv, default `false` — Sandbox PoC). Validate strategy ∈ enum at load (fail-fast, consistent with CTRL-001 startup pattern).
3. **Transaction ordering (now trivial — the key win)**: `BeginTx → GetLatestVersionByEvalItem (FOR UPDATE) → InsertEvidence (row incl. file_content BYTEA via EvidenceBlobStore.Put bound to the same tx) → RecordEvidence{Created|Versioned} (audit_logs, same tx) → Commit`. No blob write outside the TX → **no separate orphan-cleanup story required**; `tx.Rollback` undoes the bytes automatically. This directly satisfies AC-EVID-003-3 and AC-EVID-UBI-002-A/B with the existing CTRL-001 rollback pattern.
4. **Failure/rollback handling**: identical to CTRL-001 — `defer tx.Rollback(ctx)` (no-op after Commit, per `pg_store.go:287-296`); audit INSERT failure → return wrapped error, deferred Rollback reverts evidence row + blob together. No goroutine leak (goleak gate).
5. **`EvidenceBlobStore` interface still ships interface-only-plus-one-impl**: `Put(ctx, key, reader) (location string, error)` / `Get(ctx, location) (reader, error)`. The PoC concrete impl is a `dbBlobStore` that, for DB BLOB, returns a logical `storage_location` (e.g., `db://evidences/<id>`) and the actual bytes travel as the `file_content` INSERT parameter inside `EvidenceTx`. This keeps REQ-EVID-004-O1 ("strategy-swappable, no external impl") satisfied while still giving the PoC a working path. (Trade-off acknowledged: for DB BLOB the blob bytes are carried by `EvidenceTx.InsertEvidence`, and `EvidenceBlobStore` records location/metadata — this is the minimal honest reconciliation of "interface abstraction" with "must be atomic in one TX". Surface this nuance at the Human Gate.)
6. **Data-sovereignty test (AC-EVID-004-2 / AC-EVID-UBI-001)**: trivially passes — `internal/storage` imports no external SDK; the only egress is the existing internal PostgreSQL pool.

> Open sub-question for the Human Gate: Option 6 has two readings — (6a) `EvidenceBlobStore` is a thin metadata/location recorder and bytes ride `EvidenceTx.InsertEvidence(file_content)` (recommended — preserves single-TX atomicity); vs (6b) `EvidenceBlobStore.Put` itself takes a tx handle. 6a is cleaner and matches the CTRL-001 layering; flagged for explicit user confirmation.

---

## 3. Plan Summary (phased, honoring [DELTA] processing order)

DELTA processing order enforced: **[EXISTING] characterization first → [MODIFY] characterize→modify→verify → [NEW] full TDD RED-GREEN-REFACTOR**. TDD per quality.yaml.

### Sprint 0 — Foundation & Regression Baseline (Priority: High)
- [EXISTING] Run existing `store` + `audit` + `cmd/server` test suites → capture GREEN baseline (characterization: `postgres_test.go`, `pg_store_ping_test.go`, `fake_store_test.go`, `recorder_test.go`, `server_test.go`). This is the regression guard for all [MODIFY] work.
- [NEW] `0002_evidence_tables.sql` — idempotent (`CREATE TABLE IF NOT EXISTS`, `DO $$ ... EXCEPTION WHEN duplicate_object ...`), per plan.md §3 DDL **plus** `file_content BYTEA` (per §2.6.1). `initial.sql` untouched (R-EVID-008).
- [MODIFY] `audit/audit.go` — add `ActionEvidenceCreated = "EVIDENCE_CREATED"`, `ActionEvidenceVersioned = "EVIDENCE_VERSIONED"` in the existing const block (additive, existing symbols unchanged).
- [MODIFY] `store/store.go` — declare `EvidenceStore` / `EvidenceTx` interfaces mirroring `WorkflowStore`/`WorkflowTx` (signatures only).
- Verify [MODIFY] regression: existing suites still GREEN.

### Sprint 1 — Evidence Store + pool reuse (Priority: High) — REQ-EVID-001
- RED: `store/evidence_test.go` — `InsertEvidence` + `GetEvidenceByID` (testcontainers postgres:16-pgvector).
- GREEN: [NEW] `store/evidence.go` (`PgEvidenceTx` pgx impl, mirrors `PgWorkflowTx` in `pg_store.go`); [MODIFY] **`pg_store.go`** (NOT `postgres.go` — §0 correction) — add `(*PgWorkflowStore) BeginEvidenceTx(ctx) (EvidenceTx, error)` reusing `s.pool`, OR a new `pg_evidence.go` in package `store` sharing `PgWorkflowStore.pool`. Reuse `pgx.TxOptions{IsoLevel: pgx.ReadCommitted}`.
- RED: SELECT FOR UPDATE concurrency (REQ-EVID-001-S1, AC-EVID-001-4) — 2 goroutines, goleak.
- GREEN: `GetLatestVersionByEvalItem` with `FOR UPDATE`.
- REFACTOR: extract shared TX helper only if duplication is real (resist premature abstraction — Constitution Behavior 4).
- Verify [MODIFY] regression on `pg_store.go`.

### Sprint 2 — Versioning & Immutability (Priority: High) — REQ-EVID-002
- RED: re-upload → `version=2`, `previous_version_id` chain, prior `status` ACTIVE→SUPERSEDED (AC-EVID-002-1/2).
- GREEN: store/handler version-resolution (max version → +1 → set predecessor SUPERSEDED, same TX).
- RED: 3-deep chain + prior-version body-column UPDATE rejected (REQ-EVID-002-U1, AC-EVID-002-3, AC-EVID-UBI-004).
- GREEN: store-layer mutation guard (successor exists → reject body-column UPDATE, no SQL executed).

### Sprint 3 — Audit Integration (Priority: High) — REQ-EVID-003
- RED: `audit/recorder_evidence_test.go` — `RecordEvidenceCreated` / `RecordEvidenceVersioned` audit-row shape (AC-EVID-003-1/2).
- GREEN: [MODIFY] `audit/recorder.go` — add 2 methods, **exact `RecordCreated(ctx, tx AuditTx, ...)` signature pattern**, reuse local `AuditTx` (R-EVID-001, no `store` import), `resolveUserID` (R-EVID-003), `DetailsJSON` = `{evaluation_item_id, version, file_hash_sha256, previous_version_id?}`.
- RED: audit INSERT fault injection → evidence + audit bidirectional rollback (REQ-EVID-003-U1, AC-EVID-003-3), goleak.
- GREEN: handler TX orchestration — `defer tx.Rollback(ctx)` pattern from `pg_store.go:287-296`.
- REFACTOR: [NEW] `audit/clock.go` clock injection (R-EVID-007 — replace direct `time.Now().UTC()`); narrow scope, optional per spec.md §2.1.

### Sprint 4 — Storage Abstraction + Endpoint (Priority: High) — REQ-EVID-004 + REQ-EVID-001-E1
- RED: [NEW] `internal/storage/storage.go` `EvidenceBlobStore` contract test (interface exists, no external impl, no external SDK import — AC-EVID-004-2/3).
- GREEN: interface + PoC `dbBlobStore` (per §2.6.5, decision-gated), `storage_strategy` enum validation (CHECK + store-layer), config keys (§2.6.2).
- RED: [NEW] `cmd/server/evidence_handlers_test.go` — `httptest.NewRequest` create/version + oversized/empty/missing-field (AC-EVID-001-2), eval_item stub no-FK (AC-EVID-001-3), duplicate SHA-256 env-gated optional (AC-EVID-001-O1-1).
- GREEN: [NEW] `cmd/server/evidence_handlers.go` — multipart → `crypto/sha256` (stdlib, sovereignty) → BeginTx → version-resolve → InsertEvidence (+BYTEA) → Recorder → Commit → 201; pre-TX size/field validation (no TX on reject, INFO log).
- RED: external egress 0 on store/hash/retrieve path (REQ-EVID-UBI-001, REQ-EVID-004-U1, AC-EVID-UBI-001).
- GREEN: static import audit + network-spy assertion.

### Sprint 5 — Quality Gate (Priority: High)
- Coverage ≥ 85% (`quality.yaml` target); golangci-lint default + gosec 0 issues; `goleak.VerifyNone(t)` all tests.
- @MX tags per plan.md §5 (RED `@MX:TODO` all resolved; ANCHOR/WARN/NOTE finalized; `code_comments: ko` → Korean tag text per mx-tag-protocol).
- manager-quality TRUST 5; evaluator-active strict per-sprint ≥ 0.75 (thorough harness).
- Pre-submission self-review (workflow-modes Drift Guard + simplicity gate).

---

## 4. Requirements → REQ-ID Mapping & Success Criteria

| REQ-ID | Sprint | Success Criteria (from 19 AC) |
|---|---|---|
| REQ-EVID-UBI-001 (데이터 주권) | S4 | AC-EVID-UBI-001 (0 external egress, stdlib sha256), AC-EVID-004-2 |
| REQ-EVID-UBI-002 (감사 가능성) | S3 | AC-EVID-UBI-002-A, AC-EVID-UBI-002-B (exactly 1 audit row / event, same TX) |
| REQ-EVID-UBI-003 (cli-anonymous) | S3 | AC-EVID-UBI-003 (`created_by`=`user_id`=`cli-anonymous` literal, byte-identical) |
| REQ-EVID-UBI-004 (버전 불변) | S2 | AC-EVID-UBI-004 (prior version byte-identical, no DELETE, status-only transition) |
| REQ-EVID-001 (모델 & store) | S1, S4 | AC-EVID-001-1 (201 + atomic, p99<150ms ex-blob), -2 (400 oversized/empty/missing), -3 (no-FK stub), -4 (FOR UPDATE serialize), -O1-1 (env-gated duplicate signal) |
| REQ-EVID-002 (버전 체이닝) | S2 | AC-EVID-002-1 (v2+previous_version_id), -2 (prior retrievable), -3 (3-deep + immutability) |
| REQ-EVID-003 (감사 연계) | S3 | AC-EVID-003-1 (RecordEvidenceCreated), -2 (RecordEvidenceVersioned), -3 (bidirectional rollback) |
| REQ-EVID-004 (저장 추상화) | S4 | AC-EVID-004-1 (enum CHECK), -2 (no external SDK/egress), -3 (swappable interface, no concrete external impl) |

Aggregate success: all 19 AC automated GREEN; coverage ≥85%; lint+gosec 0; goleak pass; CTRL-001 characterization 0 regression; TRUST 5 pass; evaluator-active ≥0.75.

---

## 5. Tech Stack & Dependencies

- Go 1.22+ (project go.md says 1.23+; build tag/CI must match existing) · module `github.com/ircp/iroum-ax`.
- `github.com/jackc/pgx/v5` (+`pgxpool`,`pgconn`) — **reuse `PgWorkflowStore.pool`**, no new pool (R-EVID-005).
- `crypto/sha256` stdlib — no external hash dep (데이터 주권).
- `github.com/google/uuid`, `go.uber.org/zap` — existing patterns.
- Test: `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go` (postgres:16-pgvector), `go.uber.org/goleak`.
- Migration: manual idempotent SQL `0002_evidence_tables.sql` — no migration runner (Exclusion §9; CTRL-001 §8 parity).
- **No new external dependency.** DB-BLOB recommendation deliberately avoids MinIO SDK / aws-sdk — consistent with 데이터 주권 (REQ-EVID-UBI-001) and "no new external deps unless justified".

---

## 6. Complexity & Effort (Priority labels — [HARD] no time estimates)

| Work item | Priority | Complexity driver |
|---|---|---|
| Storage-strategy decision sign-off | High | Blocks Sprint 4 shape; key Human Gate item |
| `0002_evidence_tables.sql` (+BYTEA) | High | Idempotency + self-FK + CHECK; one-shot |
| `EvidenceStore`/`EvidenceTx` + pg impl on `pg_store.go` | High | Mirrors verified CTRL-001 pattern (low novelty, high care — phantom-path risk A2) |
| SELECT FOR UPDATE concurrency + version chaining | High | Concurrency correctness; goleak; race tests |
| Audit recorder extension + bidirectional rollback | High | Atomicity invariant; fault injection |
| Endpoint (multipart→sha256→TX) + storage abstraction | High | Most surface; ties all REQs together |
| Quality gate | High | thorough harness, ≥0.75 strict, coverage 85% |

Effort posture: medium-high overall; novelty is low (proven CTRL-001 patterns) but invariant-criticality is high (atomicity, immutability, sovereignty). Manager-tdd, sub-agent mode (sequential dependency chain S0→S1→S2→S3→S4→S5; not team mode — tightly coupled same-package edits, token discipline).

---

## 7. Reference Implementations (mirror these — file:line)

| New artifact | Mirror | Source |
|---|---|---|
| `EvidenceStore`/`EvidenceTx` interfaces | `WorkflowStore`/`WorkflowTx` | `store.go:18-51` |
| `PgEvidenceTx` pgx impl (InsertEvidence, Get*, Commit/Rollback, FOR UPDATE) | `PgWorkflowTx` (`InsertWorkflow`, `GetWorkflow` FOR UPDATE, `InsertAuditLog`, `Commit`, `Rollback`) | `pg_store.go:96-296` |
| Evidence TX entry point (pool reuse) | `PgWorkflowStore.BeginTx` reusing `s.pool` | `pg_store.go:83-89` (**NOT `postgres.go`**) |
| `RecordEvidenceCreated/Versioned` | `RecordCreated` exact signature + `Event` build + `resolveUserID` | `recorder.go:71-88`, `:52-57` |
| local `AuditTx` reuse (no store import) | `AuditTx` interface | `recorder.go:28-31` |
| Action constants | existing const block | `audit.go:15-54` |
| audit_logs INSERT (JSONB details) | `PgWorkflowTx.InsertAuditLog` | `pg_store.go:242-275` |
| Idempotent migration pattern | `initial.sql` DO/EXCEPTION + `0001_initial.sql` | `.moai/db/schema/initial.sql`, `migrations/0001_initial.sql` |
| Config env keys | `getEnv`/`getBoolEnv`/`Load` | `config.go:83-103` |
| Handler test harness | existing `httptest` usage | `cmd/server/server_test.go` |

---

## 8. Risk Treatment (plan.md §7 R-EVID-001..008)

| Risk | Treatment in this plan | Residual |
|---|---|---|
| R-EVID-001 store→audit cycle | Reuse local `AuditTx` (`recorder.go:28-31`); `audit` must not import `store`; verify with import check | Low |
| R-EVID-002 orphan evidence row | Single TX BeginTx→...→Commit; `defer tx.Rollback`; **DB-BLOB makes blob part of same TX → orphan-cleanup story eliminated** (key §2.6.3 benefit); AC-EVID-003-3 | Low (reduced by storage pick) |
| R-EVID-003 user_id leak | `resolveUserID` reuse, `created_by` DEFAULT `cli-anonymous`; AC-EVID-UBI-003 | Low |
| R-EVID-004 concurrent dup version | `GetLatestVersionByEvalItem` FOR UPDATE; AC-EVID-001-4; `@MX:WARN` on version-resolve block | Low |
| R-EVID-005 new pgx pool | **Reuse `PgWorkflowStore.pool` via `pg_store.go`** (§0 phantom-path correction); code-review gate | Low (correction propagated) |
| R-EVID-006 data-sovereignty violation | DB-BLOB = zero external endpoint by construction; no MinIO/S3 SDK; REQ-EVID-004-U1 gate; AC-EVID-004-2 | Very Low (reduced by storage pick) |
| R-EVID-007 time.Now testability | Sprint 3 REFACTOR `clock.go` injection (scoped) | Low |
| R-EVID-008 initial.sql drift | `0002_evidence_tables.sql` separate; `initial.sql` untouched; `[EXISTING]` marker | Low |
| **R-EVID-NEW-A (phantom path)** | spec/plan/research point evidence wiring at dead `postgres.go`; **corrected to `pg_store.go`** in §0 — must be in Run handover prompt | Low if propagated |
| **R-EVID-NEW-B (DDL gap)** | plan.md §3 DDL has no blob column; DB-BLOB pick requires `file_content BYTEA` (§2.6.1) — surface at Human Gate | Resolved if approved |

---

## 9. Human Gate — Decision Point 1 (items requiring user sign-off)

1. **[KEY] Storage strategy = `database_blob` (DB BLOB / PostgreSQL BYTEA) for the PoC.** Rationale §2.3; rejected MinIO/filesystem §2.4; risk-if-wrong §2.5. Approving this unblocks Sprint 4 shape.
2. **DDL amendment**: add `file_content BYTEA` (nullable) to `0002_evidence_tables.sql` (plan.md §3 currently omits it) so the BYTEA strategy has a home while keeping a future filesystem strategy valid (§2.6.1).
3. **Phantom-path correction**: evidence TX entry point goes into `pg_store.go` (real pool), not the dead `postgres.go` stub. Documentation/path correction, not scope change (§0). Confirm acknowledgement.
4. **`EvidenceBlobStore` reconciliation (§2.6.5/6)**: for DB BLOB, blob bytes ride `EvidenceTx.InsertEvidence(file_content)` and `EvidenceBlobStore` records logical location (option 6a, recommended) — confirm vs alternative 6b.
5. **Config keys**: `EVIDENCE_STORAGE_STRATEGY` (default `database_blob`), `EVIDENCE_MAX_FILE_BYTES` (default 52428800), `EVIDENCE_DUPLICATE_SIGNAL_ENABLED` (default false). Confirm defaults.

On approval: `/clear` then handover to manager-tdd with: this plan, the verified signature table (§0), the storage decision (§2), the phantom-path correction (§0/§9.3), TDD sprint sequence (§3), 19-AC success criteria (§4), risk treatments (§8). Sub-agent mode, sequential S0→S5, thorough harness, evaluator-active strict ≥0.75 per sprint.
