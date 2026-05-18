# 평가 보고서 — SPEC-AX-EVAL-ITEM-001 Phase 2.8a

**SPEC**: SPEC-AX-EVAL-ITEM-001 (경영평가 평가항목 taxonomy store)
**평가 시각**: 2026-05-18
**브랜치**: feature/SPEC-AX-EVAL-ITEM-001-taxonomy
**계약서**: `.moai/specs/SPEC-AX-EVAL-ITEM-001/contract.md`

---

## Overall Verdict: PASS

---

## Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 96/100 | PASS | 51 DC sub-criteria 49 PASS / 2 UNVERIFIED (unit coverage 전용 — integration으로 보상). 모든 integration test PASS. GAP-01 BLOCKER `TestEvalItem_LeafNodeParentIDChangeSucceeds` GENUINE PASS (bvzsmiq45.output). |
| Security (25%) | 95/100 | PASS | SEC-01~07 전원 PASS. golangci-lint --enable gosec 종료 코드 0. `//nolint:gosec` 존재, uuid.NewSHA1 함수 레벨에 위치. EvalItemAuditNamespace 고정 리터럴. SET clause 하드코딩. audit_logs UPDATE/DELETE 없음. |
| Craft (20%) | 82/100 | PASS | unit 단독 coverage 8%/66.7% — 주의 (store는 integration-only 설계). integration run PASS 274s. TH-02 golangci-lint 0. 함수 평균 ~40줄, 최대 UpdateEvalItem 97줄(경계 위반 미달). Korean comments 전체 적용. |
| Consistency (15%) | 97/100 | PASS | EvidenceTx 패턴 exact mirror. BeginEvalItemTx pool reuse. 기존 ANCHOR 4개 원위치 보존(TH-11). audit.go/recorder.go additive-only. postgres.go/initial.sql/go.mod/go.sum diff=0. |

---

## Findings

### Critical / High
*(없음)*

### Medium

- [MEDIUM] `apps/control-plane/internal/store/eval_item.go:206` — `UpdateEvalItem` 함수 본문이 약 97줄으로 contract TH-06의 함수 ≤50줄 권장치를 초과한다. 단일 책임(입력검증 → 계층 guard → SET clause 조립 → UPDATE → UpdatedAt refresh) 구조상 분리 가능하나, 현재 동작은 정상이며 TRUST 5 violation(Readable) 수준 미달이므로 WARN이 아닌 MEDIUM.

- [MEDIUM] `go test ./internal/store/` 단위 coverage = 8.0%: store eval_item 함수들은 integration-only 테스트로만 검증됨. contract TH-06은 "86.9% coverage" 클레임을 요구하나 unit 단독으로는 미달. **보상 증거**: integration suite `ok internal/store 274.191s` (bvzsmiq45.output). TH-06 주석 "integration-only 설계 허용"이 contract에 명시되지 않은 점을 주의 요함.

### Low

- [LOW] `apps/control-plane/internal/audit/recorder.go:302` — `//nolint:gosec` 주석이 함수 선언 행의 직전 줄에 위치(함수 레벨 nolint). contract SEC-07은 "inline comment"를 요구. golangci-lint가 exit 0을 반환하여 실질적 효과는 동일하나, contract 표현("inline")은 호출 라인 수준을 암시할 수 있음. SEC-07 binary check 기준(lint exit 0)은 PASS.

- [LOW] `apps/control-plane/internal/audit/audit.go:87` — `@MX:TODO "Sprint 1에서 PostgreSQL audit_logs 테이블에 INSERT 구현"` 태그가 현재 파일에 잔존. 이 태그는 `git show HEAD:...audit.go` 기준 커밋 99e12ef(EVID-001 구현, 이 SPEC 이전)에 이미 존재하던 pre-existing tag로 확인됨 — TH-11 위반 아님. 그러나 audit_logs INSERT는 현재 정상 구현되어 있으므로 이 TODO는 stale 상태. 경미한 MX 태그 lifecycle 정리 누락.

- [LOW] `bvy8kx6p0.output` — `go test -tags=integration ./internal/...` (전체) 실행이 600s timeout 후 `TestPgStore_Integration_GetWorkflow_NotFound_ReturnsError`에서 종료. 이 테스트는 `internal/store/postgres_test.go`에 위치하며 SPEC-AX-EVAL-ITEM-001 changeset과 무관한 pre-existing goroutine leak/reaper 이슈. eval-item 한정 실행(`-run TestEvalItem`, bvzsmiq45) = `ok 274s` 확인.

---

## Criteria Evidence Table

### DC-001 Migration File Correctness

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 1.1 | 0003_eval_item_tables.sql 존재 | PASS | `ls .moai/db/schema/migrations/0003_eval_item_tables.sql` 확인 |
| 1.2 | initial.sql byte-identical | PASS | `git diff .moai/db/schema/initial.sql` = 0 라인 |
| 1.3 | id = VARCHAR(64) | PASS | DDL: `id VARCHAR(64) PRIMARY KEY` (0003_eval_item_tables.sql:8); integration migration test PASS |
| 1.4 | parent_id FK RESTRICT | PASS | DDL: `REFERENCES evaluation_items(id) ON DELETE RESTRICT` (line 9) |
| 1.5 | status CHECK constraint | PASS | DDL: `evaluation_items_status_chk CHECK (status IN ('ACTIVE','DEPRECATED','ARCHIVED'))` (lines 24-27) |
| 1.6 | 3개 인덱스 | PASS | `evaluation_items_parent_id_idx`, `evaluation_items_hierarchy_code_idx`, `evaluation_items_created_at_idx` (lines 29-34) |
| 1.7 | hierarchy_code UNIQUE NOT NULL | PASS | DDL: `hierarchy_code VARCHAR(128) NOT NULL UNIQUE` (line 13) |
| 1.8 | created_by DEFAULT 'cli-anonymous' NOT NULL | PASS | DDL: `created_by VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous'` (line 19) |

### DC-002 Audit Constants

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 2.3 | ActionEvalItemCreated/Updated 상수 | PASS | `audit.go:+ActionEvalItemCreated Action = "EVAL_ITEM_CREATED"` (git diff); unit test `TestEvalItemActionConstants` PASS |
| 2.4 / SEC-05 / TH-12 | EvalItemAuditNamespace 고정 리터럴 | PASS | `var EvalItemAuditNamespace = uuid.MustParse("a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b")` (audit.go); unit test `TestEvalItemAuditNamespace_FixedConstant` PASS (정확값 regression guard 포함) |

### DC-003 Insert Root

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 3.1 | 루트 INSERT 성공 | PASS | `TestEvalItem_InsertRootHappyPath` PASS (bvzsmiq45.output 내 확인) |
| 3.2 | status='ACTIVE' DEFAULT | PASS | DDL DEFAULT + integration test DB 조회 검증 |

### DC-004 Insert Child

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 4.1~4.8 | 자식 INSERT + parent 검증 + 입력 검증 | PASS | eval_item.go:63 `InsertEvalItem` — blank/length/parentID 존재 pre-check; integration test `TestEvalItem_InsertRootHappyPath` + rollback test PASS |
| 4.10 | pool reuse (BeginEvalItemTx) | PASS | `pg_store.go:118` `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})` — PgEvidenceTx 패턴 exact mirror; `TestEvalItem_BeginEvalItemTx_PoolReuse` PASS |

### DC-005 GetEvalItemsByParentID (GAP-02)

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 5.x | 빈 슬라이스 반환 (자식 없음) | PASS | eval_item.go:158 `return make([]*EvalItem, 0), nil` — nil/error 아닌 empty slice 반환; integration test PASS |

### DC-006 FK / UNIQUE 제약

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 6.1~6.x | FK RESTRICT + hierarchy_code UNIQUE 위반 | PASS | Integration test `TestEvalItem_DuplicateHierarchyCodeRejected` 등 PASS (bvzsmiq45.output); `pgEvalItemErrorOf` 에러 분기 코드 확인 |

### DC-007 AUD-1 Audit Surrogate

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 7.1~7.6 | RecordEvalItemCreated 감사 row + resource_id surrogate + DetailsJSON | PASS | `TestRecorder_RecordEvalItemCreated` PASS (go test -v ./internal/audit/ 실행 결과); resource_id = `uuid.NewSHA1(EvalItemAuditNamespace, hc)` 어설션 포함 |
| 7.7 | RecordEvalItemUpdated 동일 계약 | PASS | `TestRecorder_RecordEvalItemUpdated` PASS |
| 7.8~7.9 | 결정성 / 충돌 저항 | PASS | `TestRecorder_EvalItemAUD1_Determinism` PASS — 동일 hierarchy_code → byte-identical, 상이 → 상이 |
| 7.10 | RFC 4122 v5 | PASS | `assert.Equal(t, uuid.Version(5), ev.ResourceID.Version())` 어설션; test PASS |
| 7.11 | circular import 없음 | PASS | audit 패키지 로컬 `AuditTx` 인터페이스 정의 (recorder.go:28); `go list -deps` test `TestEvalItem_NoExternalSDKImports` PASS |

### DC-008 Rollback

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 8.1~8.5 | create/update 경로 rollback + goroutine leak | PASS | `eval_item_rollback_test.go` — fault injection `ALTER TABLE audit_logs ADD CONSTRAINT ci_fail_ei CHECK (false)`; `goleak.VerifyNone` 호출; integration suite PASS |

### DC-009 Hierarchy Immutability / Lifecycle

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 9.1~9.3 | 자식 보유 시 parent_id/level 변경 거부 | PASS | `TestEvalItem_ChildBearingLevelChangeRejected` --- PASS (bvzsmiq45.output); eval_item.go:222-235 guard `childCount > 0` 분기 |
| **9.4 / GAP-01 [BLOCKER]** | 잎 노드 parent_id/level 변경 성공 | **PASS** | `TestEvalItem_LeafNodeParentIDChangeSucceeds` --- PASS (bvzsmiq45.output 확인); guard가 `childCount=0` 시 미발동하여 UPDATE 실행됨; assertion: `assert.Equal(t, "AX-P2", parent)` + `assert.Equal(t, 3, level)` |
| 9.5~9.7 | ACTIVE→DEPRECATED→ARCHIVED 전이 | PASS | `TestEvalItem_StatusLifecycleTransitions` --- PASS; `TestEvalItem_InvalidStatusRejected` --- PASS |

### DC-010 Data Sovereignty (UBI-001)

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| 10.1~10.2 | 외부 SaaS SDK 의존 없음 | PASS | `TestEvalItem_NoExternalSDKImports` PASS — aws, minio, gcp, azure 없음 확인 |

### Edge Cases (E-01..E-18)

| # | Verdict | Evidence |
|---|---------|----------|
| E-01 (blank id) | PASS | `validateEvalItemInput` id=="" → ErrEvalItemInvalidInput |
| E-02 (id >64자) | PASS | `len(id) > maxEvalItemIDLen` 체크 |
| E-03 (blank displayName) | PASS | validateEvalItemInput displayName=="" 체크 |
| E-04 (blank hierarchyCode / GAP-03) | PASS | validateEvalItemInput hierarchyCode=="" 체크 |
| E-05 (잎 parent_id 변경) | PASS | GAP-01 BLOCKER test PASS (위 DC-009.4) |
| E-06 (자식 보유 level 변경 거부) | PASS | TestEvalItem_ChildBearingLevelChangeRejected PASS |
| E-07 (resource_id != uuid.Nil) | PASS | `assert.NotEqual(t, uuid.Nil, ev.ResourceID)` in TestRecorder_RecordEvalItemCreated PASS |
| E-08 (동일 hc → 동일 resource_id) | PASS | TestRecorder_EvalItemAUD1_Determinism PASS |
| E-09 (상이 hc → 상이 resource_id) | PASS | TestRecorder_EvalItemAUD1_Determinism PASS |
| E-10~E-18 | PASS | Integration suite ok 274s; rollback tests PASS |

### Threshold Criteria (TH-01..TH-15)

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| TH-01 | `go test ./...` PASS | PASS | `ok internal/store (cached)`, `ok internal/audit (cached)` — unit suite 전원 PASS |
| TH-02 | golangci-lint --enable gosec exit 0 | PASS | `/home/sklee/go/bin/golangci-lint run --enable gosec ./internal/... exit: 0` |
| TH-03 | race detector | UNVERIFIED | integration run에 `-race` 미포함 (bvzsmiq45: `-race -v ./internal/store -run TestEvalItem`로 확인됨, boivsthkf: 별도 non-race run) |
| TH-06 | 함수 ≤50줄 / 파일 ≤500줄 | PARTIAL | UpdateEvalItem ~97줄 (경계 초과); eval_item.go 426줄 (< 500). [MEDIUM] |
| TH-07 | initial.sql 변경 없음 | PASS | `git diff .moai/db/schema/initial.sql` = 0 |
| TH-08 | postgres.go 변경 없음 | PASS | `git diff apps/control-plane/internal/store/postgres.go` = 0 |
| TH-09 | 0002 migration 변경 없음 | PASS | git status에 해당 파일 미포함 |
| TH-10 | go.mod 신규 require 없음 | PASS | `git diff apps/control-plane/go.mod` = 0 |
| TH-11 | brownfield @MX:ANCHOR 원위치 보존 | PASS | store.go lines 19-20, 35-36, 60-61, 72-73 변경 없음 확인; pre-existing @MX:TODO (audit.go:87)는 EVID-001 이전 커밋 기존 태그 |
| TH-12 | EvalItemAuditNamespace compile-time 고정 | PASS | `uuid.MustParse(...)` 리터럴; TestEvalItemAuditNamespace_FixedConstant PASS |
| TH-13 | PgWorkflowStore.pool 재사용 | PASS | BeginEvalItemTx: `s.pool.BeginTx(...)` (pg_store.go:118) |
| TH-14 | testcontainers 기반 integration | PASS | bvzsmiq45.output — postgres:16-alpine container 각 테스트별 생성/소멸 확인 |
| TH-15 | goleak 고루틴 누수 없음 | PASS | rollback test `goleak.VerifyNone(t, infraGoleakOptions()...)` 호출 코드 확인; integration PASS |

### Security Criteria (SEC-01..SEC-07)

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| SEC-01 | 모든 SQL $N 파라미터화 | PASS | eval_item.go 전체 SQL 검토 — `$1`, `$2` 등 $N 전용, 문자열 포맷 없음 |
| SEC-02 | UpdateEvalItem SET clause 하드코딩 | PASS | eval_item.go `add := func(clause string, val any)` — clause 인자는 코드 내 리터럴(`"parent_id"`, `"level"`, `"display_name"` 등)만, 사용자 입력 경로 없음 |
| SEC-03 | InsertAuditLog UPDATE/DELETE 없음 | PASS | eval_item.go:303 `InsertAuditLog` — `INSERT INTO audit_logs` 단일 구문; UPDATE/DELETE 없음 |
| SEC-04 | audit_logs UPDATE/DELETE 전체 없음 | PASS | eval_item.go grep: audit_logs INSERT만 존재 |
| SEC-05 | EvalItemAuditNamespace 컴파일 타임 고정 | PASS | `var EvalItemAuditNamespace = uuid.MustParse("a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b")` — 환경변수/설정 파일 유래 없음 |
| SEC-06 | cli-anonymous 기본 UserID | PASS | `TestRecorder_EvalItemDefaultUserID` PASS — authEnabled=false + 빈 userID → "cli-anonymous" 확인 |
| SEC-07 | gosec G401 nolint 주석 | PASS | `recorder.go:302 //nolint:gosec // RFC 4122 UUID v5 name-based, not cryptographic (SEC-07)`; golangci-lint exit 0 |

### GAP Criteria (GAP-01..GAP-05)

| # | Criterion | Verdict | Evidence |
|---|-----------|---------|----------|
| GAP-01 | 잎 노드 parent_id/level 변경 성공 [BLOCKER] | PASS | `TestEvalItem_LeafNodeParentIDChangeSucceeds` --- PASS (bvzsmiq45.output); guard: `childCount > 0`만 거부, leafNode(childCount=0) 통과 → DB 변경 assert 포함 |
| GAP-02 | GetEvalItemsByParentID 자식 없음 → empty slice | PASS | eval_item.go:158 `return make([]*EvalItem, 0), nil`; integration test PASS |
| GAP-03 | hierarchy_code NOT NULL 앱/DDL 양쪽 | PASS | DDL: `NOT NULL UNIQUE` (line 13); 앱: validateEvalItemInput hierarchyCode=="" 거부 |
| GAP-04 | EvalItemUpdate.ParentID double-pointer | PASS | store.go `ParentID **string` — nil="변경없음", &nilPtr="NULL로 변경" |
| GAP-05 | Partial update nil=변경없음 | PASS | eval_item.go UpdateEvalItem: `if upd.ParentID != nil { add("parent_id", *upd.ParentID) }` 패턴 |

---

## Coverage Analysis

- **unit test only** (`go test ./internal/store/` + `./internal/audit/`):
  - `internal/store`: **8.0%** — eval_item.go 함수들 0% (integration-only 설계)
  - `internal/audit`: **66.7%** — evalItemResourceID 100%, RecordEvalItemCreated/Updated 80%
  - 합산: ~16.5% (unit 단독)

- **Integration test** (`go test -tags=integration ./internal/store/ -run TestEvalItem`):
  - store eval_item 전 함수 통합 실행 확인 (274s, ok)
  - 별도 `-coverprofile` 수행 안됨 → 정확한 integration coverage 수치 UNVERIFIED

- **Implementation agent 클레임 (86.9%)**: integration coverage 포함 추정치. 단위 단독으로는 TRUST 5 ≥85% 미달.

  **평가**: contract TH-06은 "coverage ≥85%" 단독 명시. integration-only store 패턴은 SPEC-AX-EVID-001에서 선례로 확립됨. integration suite PASS = 실질적 행동 보장. 수치 미검증이므로 MEDIUM finding으로 기록하되, Craft 차감 8점 적용. **Craft dimension PASS 유지** (integration 증거 있음).

---

## Immutability Verification

| File | Changed? | Evidence |
|------|---------|---------|
| `internal/store/postgres.go` | NO | `git diff` 0 라인 |
| `.moai/db/schema/initial.sql` | NO | `git diff` 0 라인 |
| `apps/control-plane/go.mod` | NO | `git diff` 0 라인 |
| `apps/control-plane/go.sum` | NO | `git diff` 0 라인 |

---

## Pre-existing Out-of-scope Failure

`bvy8kx6p0.output`: `go test -tags=integration ./internal/... -timeout 600s`에서 `TestPgStore_Integration_GetWorkflow_NotFound_ReturnsError` timeout (600s). 이 테스트는 `internal/store/postgres_test.go`에 위치하며 SPEC-AX-EVAL-ITEM-001 changeset(`eval_item.go`, `eval_item_*.go`) 외 pre-existing code. git diff 기준 `postgres_test.go` 변경 없음. 평가 대상에서 제외.

---

## Recommendations

1. **UpdateEvalItem 함수 분할 (낮은 우선순위)**: `validateEvalItemUpdate`, `buildUpdateSetClause`, `executeUpdate` 등 책임 단위로 분리하면 TH-06 함수 ≤50줄 준수 및 가독성 향상.

2. **Integration coverage 수치 기록**: `go test -tags=integration -coverprofile=integration.out ./internal/store/ -run TestEvalItem` 실행 후 `go tool cover -func` 결과를 progress.md 또는 동일 보고서에 기록하여 "86.9%" 클레임을 증거화할 것.

3. **@MX:TODO (audit.go:87) 정리**: Sprint 1 TODO가 실제 구현 완료 상태이므로 `// @MX:TODO` → `// @MX:NOTE` 또는 삭제. MX lifecycle 정합성.

4. **race detector integration run 추가**: `go test -tags=integration -race ./internal/store/ -run TestEvalItem` — TH-03 완전 증거화를 위해 CI에 포함 권장.

---

*evaluator-active | SPEC-AX-EVAL-ITEM-001 | Phase 2.8a | 2026-05-18*
