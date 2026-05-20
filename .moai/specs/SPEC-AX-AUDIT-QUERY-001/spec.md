---
id: SPEC-AX-AUDIT-QUERY-001
version: 0.1.0
status: completed
created: 2026-05-20
updated: 2026-05-20
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 2026-05-20 v0.1.0 SYNC: TRUST 5 PASS 0.887 / evaluator-active iter1 PASS 0.887 (7 SPEC 최고 iter1, dark-flow saturation 효과 — RUBRIC iter1 FAIL 0.838 → iter2 PASS와 달리 본 SPEC iter1 직접 PASS). 4-차원: Functionality 88 / Security 92 / Craft 85 / Consistency 90. 30 tests GREEN (store 8 + handler 22). 신규 4 파일 + 수정 4 파일 (+700 LOC). consumer-only [HARD] 0-diff 진정 유지 (7 SPEC 코드/audit/auth/handlers/schema/go.mod 모두 무수정 — read-only API). frozen rbac.go 0-diff (RoleAuditor 신설 0, admin은 rbac.go:20-21 기존재). errors.go 정확 2 sentinel (ErrAuditQueryInvalidFilter/ErrAuditQueryInvalidTimeRange, Drift-Guard manifest EXPLICIT). server.go +7줄 정확 (REPORT-001 lesson 정합). audit_logs 0001 SELECT만 (INSERT 0). plan-auditor iter1 PASS CONDITIONAL 0.91 + D1 typo fix (audit.go:121-128 정정). Human Gate 6 OPEN 권장 일괄 채택 (admin-only handler narrowing / AND-only / events·count·total·generated_at / JSONB GIN 이연 / WorkflowStore.QueryAuditLogs 확장 / COUNT(*) OVER() window). Minor 3건 비차단 (parseAuditQueryFilters 68.8% / handleGetAuditLog 73.9% / toAuditEventResponse 75.0% — Sprint 2 권장). real pg integration test //go:build integration 분리 (Docker 환경 별도 검증).
- 2026-05-20 v0.1.0 RUN: TDD 사이클 단일 turn 완성 (가장 작은 vertical slice ~700 LOC). M0 RED 30 tests → M1-M2 GREEN store+handler → M3 server.go ≈7줄 → M4 REFACTOR @MX. WorkflowStore.QueryAuditLogs(AuditLogFilter) ([]Event, int64, error) + COUNT(*) OVER() 1 round-trip + 5-필터 AND + clampPagination 50/500 + admin-only handler-level requireAuditQueryReadRole/guardAuditQueryRead.
- 2026-05-20 v0.1.0 PLAN: plan-auditor iter1 PASS CONDITIONAL 0.91 (7 SPEC 최고 iter1 — saturation), D1 minor typo 정정 후 Human Gate 진행. 25 AC + 14 edge + 6 OPEN 모두 RESOLVED.
- 0.1.0 (2026-05-20): 감사 로그 검색 HTTP API 계층(Audit Log Query/Search HTTP API Layer) 첫 초안. SPEC-AX-CTRL-001(완료, audit_logs 0001 마이그레이션)·SPEC-AX-SCORE-001(완료, RecordScoreCreated/Updated)·SPEC-AX-REVIEW-001(완료, RecordScoreReviewRequest{Created,Assigned,Approved,Rejected})·SPEC-AX-RUBRIC-001(완료, RecordRubric{Created,Updated,Archived,CriterionAdded,BandAdded})·SPEC-AX-EVID-001(완료, RecordEvidence{Created,Versioned})·SPEC-AX-EVAL-ITEM-001(완료, RecordEvalItem{Created,Updated} + AUD-1 UUIDv5 surrogate)·SPEC-AX-AUTH-003(완료, ABAC)의 누적 23+ Action 상수를 통해 PgWorkflowTx.InsertAuditLog(`pg_store.go:346-381`)로 적재된 `audit_logs` 테이블(0001_initial.sql의 `initial.sql:115-123`) 위에 **검색·필터·페이지네이션이 가능한 read-only HTTP API 계층만** 추가한다. 핵심 기능: 7 SPEC 누적 감사 데이터를 한국 공공 기관 감사 책임자(audit/compliance officer)가 조회할 수 있도록 5개 필터(`action`, `resource_type`, `resource_id`, `user_id`, time range `since`/`until`)와 offset/limit 페이지네이션을 노출. SPEC-AX-AUTH-003 경량 ABAC narrowing 통합 — **감사 데이터 민감성으로 인해 admin only**(viewer/analyst 403, REQ-AUDIT-QUERY-UBI-003); cli-anonymous 기본값 + auth-disabled Walking Skeleton 투과. 한국 공공 6제약(데이터 주권/한국어/감사 가능성 확장/망분리/조직 격리) 준수. **본 SPEC은 7 SPEC 누적의 순수 consumer이며 그 코드·스키마·FK·마이그레이션·audit Action 상수·Recorder 메서드를 일절 변경하지 않는다 — DB 변경 0, 신규 마이그레이션 0(0001 audit_logs 활용), 자체 audit 0(read-only이므로 mutation 0 → audit 이벤트 0), 신규 외부 의존 0, frozen rbac.go 0-diff(admin은 `rbac.go:20-21`에 기존재 — 신규 역할 신설 절대 금지)**. CSV/XLSX export, audit 자체 read 추적, JSONB details 부분 검색, 시간 제약(KST 업무시간), 별도 audit 자체 표시 스키마 변경, AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC은 의도적 제외(§5 Exclusions). research.md(Phase 0.5 deep research, file:line 근거)가 SSOT. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-REPORT-001 / SPEC-AX-REVIEW-001 / SPEC-AX-RUBRIC-001 등 6 SPEC 누적과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등 canonical 외 필드는 사용하지 않는다. 본 SPEC의 모든 EARS 요구사항·영향파일·HTTP 계약은 `.moai/specs/SPEC-AX-AUDIT-QUERY-001/research.md`(file:line 근거)에 근거하며, 소비 계약 시그니처는 `apps/control-plane/internal/store/pg_store.go`(`PgWorkflowStore`/`BeginTx`/`PgWorkflowTx.InsertAuditLog`), `apps/control-plane/internal/audit/audit.go`(`Action`/`Event` 23+ 상수), `apps/control-plane/internal/audit/recorder.go`(`Recorder`/`evalItemResourceID` UUIDv5 surrogate AUD-1), `apps/control-plane/internal/auth/abac.go`·`rbac.go`·`middleware.go`(ABAC/RBAC `RoleAdmin`/`hasAdminRole`), `apps/control-plane/cmd/server/score_handlers.go`·`report_handlers.go`(read-only 핸들러 선례), `apps/control-plane/cmd/server/server.go`(라우트 마운트), `apps/control-plane/internal/errors/errors.go`(에러 센티넬), `apps/control-plane/go.mod`(외부 의존 인벤토리), `.moai/db/schema/initial.sql`(audit_logs 스키마 + `audit_logs_user_id_timestamp_idx`)에서 직접 검증되었다(phantom API 0건 — manager-spec orchestrator ground-truth grep, 2026-05-20).

---

# SPEC-AX-AUDIT-QUERY-001 — 감사 로그 검색 HTTP API 계층 (Audit Log Query/Search HTTP API Layer)

## 1. 개요

한국 공공 기관(KEPCO E&C) 경영평가 PoC의 감사 책임자(audit/compliance officer)가 SPEC-AX-CTRL-001이 생성한 `audit_logs` 테이블 + 6 SPEC(SCORE-001/REVIEW-001/RUBRIC-001/EVID-001/EVAL-ITEM-001/AUTH-003)이 동일-TX로 적재한 누적 감사 데이터를 검색·조회할 수 있도록, `apps/control-plane/cmd/server/`(Go 1.25.0, `go.mod:3` 명시)에 **읽기 전용 감사 로그 검색 HTTP API 계층**을 추가한다. 본 SPEC은 SPEC-AX-SCORE-API-001의 점수 핸들러(`score_handlers.go` — `ScoreHandler` struct·`Routes()`·표준 JSON/에러 헬퍼·`clampPagination`·store 에러→HTTP 매핑·ABAC 게이트) + SPEC-AX-REPORT-001의 read-only 핸들러(`report_handlers.go` — `defer Rollback`·read-only `BeginTx` 패턴)를 정확히 미러링하되 **read-only 부분집합**으로 한정한다(research.md §3). 본 SPEC은 **mutation 엔드포인트를 0개** 노출하므로 `score_handlers.go:161-187`의 write-role 게이트(`requireScoreWriteRole`/`guardScoreWrite`)는 **차용하지 않는다** — 대신 admin-only narrowing을 위한 **read-role 게이트**(`requireAuditQueryReadRole` — admin 전용)를 신규 추가한다(§1.5). 검색은 매 요청마다 `audit_logs` 테이블을 직접 조회하여 on-the-fly로 산출하며 어떤 결과도 영속화하지 않는다(스냅샷 0 — §5 #4).

### 1.1 감사 로그 검색 API 계층의 의미 (본 SPEC 범위)

본 SPEC의 1차 산출물은 **7 SPEC 누적 감사 데이터를 admin 사용자가 검색·페이지네이션 가능한 최소 read-only REST API 계층 + ABAC admin-only narrowing 통합**이다. `audit_logs` 테이블 스키마(id/user_id/action/resource_id/resource_type/timestamp/details JSONB — `initial.sql:115-123`), 인덱스(`audit_logs_user_id_timestamp_idx ON (user_id, timestamp DESC)` — `initial.sql:138`), Action 상수(`audit/audit.go:17-100` 23+개), 동일-TX INSERT(`pg_store.go:346-381 InsertAuditLog`)는 7 SPEC이 이미 GREEN(완료)으로 제공한다 — 본 SPEC은 그 **consumer**이며 신규 비즈니스 로직·DB 스키마·마이그레이션·audit·Action 상수·Recorder 메서드를 추가하지 않는다. 검색 자체는 **핸들러가 신규 store SELECT 메서드 호출**(consumer-only 위반 아님 — read-only SELECT는 신규 store 메서드 1개를 정당하게 요구; OPEN #5에서 위치/접근 패턴 RESOLVED)을 수행한다.

- 신규 파일 2개: `cmd/server/audit_query_handlers.go`(`AuditQueryHandler` + `Routes()` + 감사 검색 핸들러 메서드 + 표준 JSON/에러 헬퍼 + admin-only read-role 게이트 + store 에러→HTTP 매핑), `cmd/server/audit_query_handlers_test.go`(테스트)
- 기존 1개 수정(라우트 마운트): `cmd/server/server.go` — **라우트 마운트 최소 단위(필드+생성자+innerMux.Handle 2줄, ≈7줄) + 핸들러 인스턴스화만** (`server.go:55/216/277-278` `reportH` 선례 정확 미러, research.md §5)
- 기존 1개 수정(신규 store SELECT 메서드): `internal/store/store.go` + 신규 파일 `internal/store/audit_query.go` — **read-only SELECT 메서드만**(`QueryAuditLogs(ctx, filter, limit, offset) ([]*audit.Event, int, error)` — 결과 + 총 카운트). `audit_logs` 행 INSERT는 7 SPEC store가 이미 동일-TX로 수행 중이며 본 SPEC은 그를 수정·확장하지 않는다(OPEN #5 — 신규 store 메서드 1개 추가는 SELECT 한정으로 consumer-only 정신 유지)
- 검색 엔드포인트 (최소 surface — 구체 경로/형식은 §6 OPEN #3): 감사 로그 목록 검색(GET, read-only). mutation 엔드포인트 0
- 필터 조합: AND only (5 필터 — action, resource_type, resource_id, user_id, time range since/until — 모두 optional, 조합 AND, OPEN #2 RESOLVED). PoC OR 미지원(over-engineering 회피)
- 페이지네이션: offset/limit (`score_handlers.go:144 clampPagination` 재사용 — defaultListLimit=50, maxListLimit=500, OPEN #3 RESOLVED)
- ABAC 통합: SPEC-AX-AUTH-003 경량 ABAC narrowing-only — **감사 데이터 민감성으로 admin only**(viewer/analyst 403 — REQ-AUDIT-QUERY-UBI-003); auth-disabled 투과; admin 우회
- cli-anonymous 기본값 + auth-disabled Walking Skeleton fallback (SPEC-AX-SCORE-001 §1.1, AUTH-003 정합 — store 계층 처리)
- 표준 에러: `score_handlers.go`/`report_handlers.go` `{"error":{"code","message","field"}}` 동일 스키마(한국어, INFO 로그)

### 1.2 Anchor 컨텍스트

본 SPEC은 7 SPEC 누적이 형성한 한국 공공 기관 감사 추적성(REQ-UBI-002 family — 모든 mutation 시점에 audit_logs 행 동일-TX 적재)의 **검색 표면**을 외부에 노출한다. PoC 범위는 SPEC-AX-CTRL-001 audit_logs 테이블의 7 SPEC 누적 데이터에 대한 admin 검색 API이며, SPEC-AX-CTRL-001 / SCORE-001 / REVIEW-001 / RUBRIC-001 / EVID-001 / EVAL-ITEM-001 / AUTH-003은 GREEN(완료) 상태로 가정한다. 본 SPEC이 추가하는 것은 검색 가능성(searchability) — 7 SPEC이 추가한 것은 적재(audit logging). 두 책임은 분리된다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `AUDIT-QUERY` (Audit Log Search sub-domain — 7 SPEC 누적 audit_logs 적재 데이터의 검색 API 계층)
- 따라서 SPEC ID: `SPEC-AX-AUDIT-QUERY-001` (`.claude/skills/moai/workflows/plan.md` Composite domain rules — `AX` + `AUDIT-QUERY` 2-domain, 권장 범위 내)

### 1.4 의존성 stub 계약 — consumer-only [핵심, load-bearing]

본 SPEC은 SPEC-AX-CTRL-001 / SCORE-001 / REVIEW-001 / RUBRIC-001 / EVID-001 / EVAL-ITEM-001 / AUTH-003의 **순수 consumer**이다. 다음이 **HARD 계약**이다(research.md §12, §7, 메모리 lesson #9 phantom 회피):

- **[HARD]** 본 SPEC은 `internal/audit/*`(`Action`/`Event` 23+ 상수, `Recorder`/`RecordXxx` 메서드, `EvalItemAuditNamespace` UUIDv5 namespace), `internal/auth/*`(ABAC/RBAC `permissionMatrix`/`rbac.go`/`abac.go`/`middleware.go`), `internal/errors/*`(센티넬), `cmd/server/score_handlers.go`/`report_handlers.go`/`review_handlers.go`/`rubric_handlers.go`/`evidence_handlers.go`(핸들러 선례), `.moai/db/schema/migrations/*.sql`(0001~0006)을 **일절 수정하지 않는다**. 7 SPEC audit Action 상수·Recorder 메서드·동일-TX INSERT 로직은 그대로 유지된다.
- **[HARD]** `audit_logs` 스키마(`initial.sql:115-123` 즉 0001 마이그레이션의 audit_logs 부분)·인덱스(`audit_logs_user_id_timestamp_idx` `initial.sql:138`)는 본 SPEC이 수정하지 않는다. read-only SELECT만 — DB CHECK·UNIQUE·FK·인덱스 추가/변경 0건. 신규 마이그레이션 0건.
- **[HARD]** 자체 audit 0건 — 본 SPEC의 모든 엔드포인트는 **read-only(GET)**이므로 mutation(INSERT/UPDATE/DELETE)이 발생하지 않으며 따라서 `audit_logs` 신규 행이 0건이다(research.md §6.1/§6.3). 검색 요청 자체를 audit_logs에 기록하는 "audit-of-audit-read"는 **본 SPEC 범위 밖**(§5 #2 — 후속 SPEC). 7 SPEC mutation 시점의 audit는 이미 각 store(`RecordXxx` 동일 TX)가 기록했다. 검색 API 핸들러가 별도 audit row를 INSERT하면 부당한 audit-of-audit 재귀가 되므로 금지된다(REQ-AUDIT-QUERY-UBI-002).
- **[HARD]** `postgres.go`는 Sprint-0 死 스텁(SPEC-AX-SCORE-001 plan.md §2)이며 본 SPEC 대상 아님. TX 진입점은 `store.WorkflowStore.BeginTx`(→ `pg_store.go` `PgWorkflowStore.BeginTx`)만 사용한다. read-only SELECT는 `BeginTx` 후 `defer tx.Rollback(ctx)`(read-only, Commit 불필요 — `score_handlers.go:273 / report_handlers.go:348 / 393` 선례 미러).
- **[HARD]** 신규 외부 의존 0건 — 본 SPEC은 어떤 신규 라이브러리/SDK도 `go.mod`에 추가하지 않는다. 검색 쿼리는 표준 `database/sql`/`pgx`만 사용. CSV/XLSX export 라이브러리(`encoding/csv` 도입 등)는 §5 #6에서 제외.
- **[HARD]** **frozen rbac.go 0-diff [CRITICAL — 6 SPEC 누적 lesson]**: `rbac.go:20-25`의 3-역할(`RoleAdmin`/`RoleAnalyst`/`RoleViewer`)·`rbac.go:33` 정규식 `^iroum-ax:(admin|analyst|viewer)$`·`permissionMatrix`는 7 SPEC 누적 frozen이며 본 SPEC이 수정·확장하지 않는다. 본 SPEC의 admin-only 매핑은 **신규 `RoleAuditor` 신설 0** — 기존 `RoleAdmin`을 그대로 활용한다(admin은 audit/compliance officer 책임을 자연 포괄). SCORE-API-001 §6 OPEN #4(`evaluator` 역할 부재 충돌)와 동위상 회피 — 신규 역할 신설은 frozen rbac.go [HARD] 위반.
- **[HARD]** AUD-1 UUIDv5 surrogate 보존 — `evalItemResourceID(hierarchy_code) → uuid.NewSHA1(EvalItemAuditNamespace, ...)` (`recorder.go:295-305`)는 evaluation_items 도메인의 `audit_logs.resource_id` 직렬화 전략이다. 본 SPEC은 검색에서 `resource_id` UUID 필터를 수용하되, **VARCHAR(64) hierarchy_code 역검색(`resource_type='evaluation_item' AND details->>'eval_item_id'=...`)은 OPEN #4에서 결정**한다(JSONB details 부분 검색 — 본 PoC 활성 vs 이연).

### 1.5 ABAC 권한 경계 [핵심] — admin only narrowing (frozen rbac.go 활용)

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 통합한다. 확립된 사실(research.md §4, `abac.go`/`rbac.go`/`middleware.go` source-verified):

- **admin-only 정책**: 감사 데이터는 한국 공공 기관 감사 책임자 권한에 한정되므로(KEPCO E&C 감사 가능성 정책 정합 — research.md §13.1) 본 SPEC의 모든 엔드포인트가 `RoleAdmin` only로 narrowing된다. `viewer`/`analyst` principal은 ABAC narrowing 경로에서 **403 ABAC_CONDITION_DENIED** 반환. `viewer` 포함 read를 허용한 REPORT-001과 정반대 정책 — 감사 데이터 민감성 차이(point-in-time mutation 기록 vs 집계 도메인 결과)에 기인.
- **신규 read-role 게이트**: `score_handlers.go:161-187 requireScoreWriteRole`/`guardScoreWrite`를 **차용하지 않고**(write 게이트는 mutation 0이므로 비활성), 대신 **신규 helper `requireAuditQueryReadRole`/`guardAuditQueryRead`**를 `audit_query_handlers.go`에 정의 — `auth.ParseRolesFromScope(scope)`에서 `RoleAdmin` 단일 매핑 확인. **frozen rbac.go·permissionMatrix 0-diff** 자연 성립(기존 RoleAdmin 활용, 신규 역할 신설 0).
- **auth-disabled 투과**: `authEnabled=false`(Walking Skeleton 기본값, SPEC-AX-SCORE-001 §1.1 정합)일 때 ABAC/RBAC 미들웨어가 자동 투과(`abac.go:8` REQ-ABAC-009 source-verified), `cli-anonymous` 기록은 store 계층이 처리한다. 본 SPEC은 인증 비활성에서도 동작한다. 단 `requireAuditQueryReadRole`은 auth-context 부재 시 투과(`UserFromContext` ok=false → true 반환 — `score_handlers.go:181 guardScoreWrite` 동형 패턴).
- **admin 우회**: `RoleAdmin` 보유 principal은 모든 ABAC 조건을 우회한다(`abac.go:9` REQ-ABAC-004 source-verified). 본 SPEC에서 admin은 narrowing 경로상 자연 허용.
- **narrowing-only**: ABAC는 RBAC가 통과시킨 요청만 추가 거부할 수 있고 권한을 부여하지 않는다(`abac.go:4` source-verified). 본 SPEC의 admin-only는 read-role helper(`requireAuditQueryReadRole`)가 viewer/analyst를 추가 거부하는 narrowing — 권한 부여 아님.

> **[경계 노트]** `internal/auth/rbac.go`의 RBAC 역할은 `RoleAdmin`/`RoleAnalyst`/`RoleViewer`만 존재(`rbac.go:20-25`, scope 정규식 `^iroum-ax:(admin|analyst|viewer)$` — `rbac.go:33` source-verified), `permissionMatrix`(`rbac.go:39-`)에 audit/query Permission이 부재. consumer-only [HARD] 제약상 본 SPEC은 frozen RBAC을 수정할 수 없다. 그러나 본 SPEC은 **read-role 게이트를 핸들러 레벨에서 처리**(SCORE-API-001 `requireScoreWriteRole`/`guardScoreWrite` 패턴 동형)하므로 `permissionMatrix` 확장이 불필요하다 — `RoleAdmin` 단일 매핑이면 충분. SCORE-API-001 §6 OPEN #4(write 역할 매핑 결정성)와 본 SPEC §6 OPEN #1(admin-only 결정성)이 동위상 핸들러-레벨 RESOLVED 패턴. RoleAuditor 신설은 frozen rbac.go [HARD] 위반 — 절대 금지.

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` `apps/control-plane/` 트리를 따른다. 본 SPEC은 audit/auth/errors/7-SPEC-handler 코드 + 0001~0006 마이그레이션 + frozen rbac.go는 일절 수정하지 않고(consumer-only §1.4 HARD), 감사 검색 API 계층 파일만 추가하고 store에 read-only SELECT 메서드 1개만 추가한다. Delta 마커: [EXISTING]=consumer로 호출만(무변경), [NEW]=신규 추가, [MODIFY]=라우트 마운트 또는 SELECT 메서드 추가.

### 2.1 Go Control Plane 감사 검색 API 계층 (`apps/control-plane/cmd/server/` + `apps/control-plane/internal/store/`)

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/cmd/server/audit_query_handlers.go` | `AuditQueryHandler` struct + `NewAuditQueryHandler(...)` + `Routes() http.Handler` + 감사 검색 핸들러 메서드(목록 검색 GET) + admin-only read-role 게이트(`requireAuditQueryReadRole`/`guardAuditQueryRead` — `score_handlers.go:161-187` write-role 패턴 동형 미러) + 표준 JSON/에러 헬퍼(`score_handlers.go:74-101` 선례 미러) + store 에러 센티넬→HTTP status 매핑(`score_handlers.go:111-137` 선례) + 5-필터 파싱(`action`/`resource_type`/`resource_id`/`user_id`/time range `since`+`until`) + `clampPagination` 재사용(`score_handlers.go:144`) + UUID/time/length 사전 검증(TX 미진입). | [NEW] | REQ-AUDIT-QUERY-001~003 |
| `apps/control-plane/cmd/server/audit_query_handlers_test.go` | `httptest` 기반 핸들러 단위 테스트 — 검색 정상/에러 경로, 5-필터 AND 조합, 빈 결과 200, malformed UUID/timestamp 400, admin only 200 / viewer 403 / analyst 403, auth-disabled 투과, large result truncation(maxListLimit=500), JSONB details 직렬화 정확성, consumer-only 경계(API audit 0·git 0-diff·frozen rbac 0-diff). store는 fake `WorkflowStore`/`WorkflowTx` 또는 fake `AuditQueryStore`(OPEN #5 RESOLVED 후 결정)로 격리. | [NEW] | 전체 |
| `apps/control-plane/internal/store/audit_query.go` | **read-only SELECT만** — `QueryAuditLogs(ctx, filter AuditQueryFilter, limit, offset) (events []*audit.Event, total int, err error)` 구현. `audit_logs WHERE` 절을 5-필터 AND 조합으로 동적 빌드(파라미터 바인딩 — SQL injection 0), `audit_logs_user_id_timestamp_idx`(`initial.sql:138`) 활용 위해 `ORDER BY timestamp DESC, user_id` 정렬. `COUNT(*) OVER()` 또는 별도 `SELECT count(*)`로 total 산출(OPEN #6에서 결정 — window function vs 별도 쿼리). audit_logs INSERT·UPDATE·DELETE 메서드 0건. fake/pg 양쪽 구현. | [NEW] | REQ-AUDIT-QUERY-001 |
| `apps/control-plane/internal/store/audit_query_test.go` | `internal/store` 단위 테스트 — `QueryAuditLogs` 결과 정확성, 5-필터 AND 조합, 페이지네이션 슬라이싱, 결과 0건 처리, total count 정확성, ORDER BY 결정성. testdata-driven. | [NEW] | 전체 |
| `apps/control-plane/internal/store/store.go` | `AuditQueryStore` 인터페이스 추가(또는 `WorkflowStore`에 `QueryAuditLogs` 메서드 추가 — OPEN #5 RESOLVED 후 결정). `AuditQueryFilter` struct(action/resource_type/resource_id/user_id/since/until — 모두 optional pointer). **현재 audit_logs INSERT 인터페이스(`WorkflowTx.InsertAuditLog`)는 0-diff.** | [MODIFY] | REQ-AUDIT-QUERY-001 |
| `apps/control-plane/internal/store/pg_store.go` | `PgWorkflowStore.QueryAuditLogs(...)` 구현 (OPEN #5: 별도 메서드 vs 별도 store 인터페이스). 기존 `BeginTx`(`pg_store.go` 진입점) · `PgWorkflowTx.InsertAuditLog`(`pg_store.go:346-381`) **0-diff**. read-only SELECT 한정. | [MODIFY] | REQ-AUDIT-QUERY-001 |
| `apps/control-plane/cmd/server/server.go` | **라우트 마운트만**(≈7줄 최소 단위): `auditQueryH` 필드(`server.go:55-60` `reportH`/`reviewH`/`rubricH` 선례 위치) + `s.auditQueryH = NewAuditQueryHandler(pgStore, logger)` 생성자(`server.go:216 NewReportHandler` 선례 위치 — `pgStore`가 `AuditQueryStore` 구현 또는 `WorkflowStore`+`QueryAuditLogs` 호출 가능, OPEN #5 RESOLVED 후 정확 시그니처) + `innerMux.Handle` **2줄**(`server.go:277-278 /api/v1/reports` 선례 정확 미러: `/api/v1/audit-logs` + `/api/v1/audit-logs/` 서브트리 — Go1.22 ServeMux path-param 라우팅 구조적 필수) + ko 주석. ABAC은 기존 미들웨어 체인(`server.go:287` 와이어링)이 innerMux 전체를 감싸 자동 적용 — ABAC 와이어링 변경 **0-diff**. | [MODIFY] | REQ-AUDIT-QUERY-002 |
| `apps/control-plane/internal/errors/errors.go` | 신규 센티넬 정확히 2개 추가: `ErrAuditQueryInvalidFilter`(malformed UUID/timestamp/length 사전 검증 실패) + `ErrAuditQueryInvalidTimeRange`(since > until 또는 future timestamp 거부) — **SCORE-API-001 errors.go drift lesson [HARD]**: §2.1 + §2.3 Drift-Guard manifest 양쪽 EXPLICIT 부착 — manifest 분실 방지. 기존 센티넬 무수정. | [MODIFY] | REQ-AUDIT-QUERY-003 |

### 2.2 소비 계약 — 호출만, 무변경 (consumer-only [HARD] §1.4)

| 경로 | 소비 계약 | Delta |
|------|----------|-------|
| `apps/control-plane/internal/audit/audit.go` | `Action` 23+ 상수(`audit.go:17-100` 7 SPEC 누적: WORKFLOW_*/AUTH_*/ABAC_*/SERVER_*/EVIDENCE_*/EVAL_ITEM_*/SCORE_*/SCORE_REVIEW_REQUEST_*/RUBRIC_*), `Event` struct(`audit.go:121-128`, 필드 Timestamp/Action/ResourceType/ResourceID(uuid)/UserID/DetailsJSON), `EvalItemAuditNamespace` UUIDv5 namespace(`audit.go:115`) — 호출만 (source-verified) | [EXISTING] |
| `apps/control-plane/internal/audit/recorder.go` | `Recorder`/`RecordXxx` 메서드(EVID/EVAL-ITEM/SCORE/REVIEW/RUBRIC), `evalItemResourceID` UUIDv5 surrogate(`recorder.go:295-305`) — **mutation 시점 동일-TX INSERT는 기존 동작 유지, 본 SPEC 0-diff** | [EXISTING] |
| `apps/control-plane/internal/store/pg_store.go` | `PgWorkflowStore` 초기화(`store.NewPgWorkflowStore`), `BeginTx`(진입점), `PgWorkflowTx.InsertAuditLog`(`pg_store.go:346-381` audit INSERT) — 호출만, mutation 인터페이스 무변경. QueryAuditLogs SELECT는 §2.1 [MODIFY] | [EXISTING] |
| `apps/control-plane/internal/errors/errors.go` | 기존 센티넬(`ErrAuditLogFailed` 등) — `errors.Is`로 매핑만 (research.md §8.1). 신규 2건 추가는 §2.1 | [EXISTING] (기존 부분) |
| `apps/control-plane/cmd/server/score_handlers.go` | `ScoreHandler` 구조·`Routes()`·`writeScoreJSON`/`writeScoreErr`·`mapStoreErr`·`clampPagination`(`score_handlers.go:144`)·write-role 게이트 helper 패턴(`score_handlers.go:161-187`) 선례 — **read-only 부분집합 + admin-only narrowing 미러만(코드 무변경)** | [EXISTING] |
| `apps/control-plane/cmd/server/report_handlers.go` | read-only `defer Rollback`·`BeginTx` 패턴 선례 — 미러만(코드 무변경) | [EXISTING] |
| `apps/control-plane/internal/auth/abac.go`, `rbac.go`, `middleware.go` | `ErrCodeABACDenied`(abac.go:24), `ABACEvaluator`(narrowing-only/admin 우회/auth-disabled 투과 abac.go:4/8/9), `RoleAdmin`/`RoleAnalyst`/`RoleViewer`(rbac.go:20-25 — 3-role frozen), 정규식 `^iroum-ax:(admin|analyst|viewer)$`(rbac.go:33), `ParseRolesFromScope`(rbac.go), `UserFromContext`(middleware.go:44-49), `hasAdminRole`(abac.go:94-99) — 호출만/미들웨어 체인 자동 적용. **permissionMatrix/Authorize/RoleXxx frozen — 무변경 [HARD], RoleAuditor 신설 절대 금지** | [EXISTING] |
| `.moai/db/schema/initial.sql` (0001 마이그레이션) | `audit_logs` 테이블(`initial.sql:115-123`: id UUID PK / user_id VARCHAR(64) / action VARCHAR(64) / resource_id UUID NOT NULL / resource_type VARCHAR(32) / timestamp TIMESTAMP WITH TIME ZONE / details JSONB), 인덱스 `audit_logs_user_id_timestamp_idx ON (user_id, timestamp DESC)` (`initial.sql:138`) — 본 SPEC은 read-only SELECT만, 스키마/인덱스 변경 0건 | [EXISTING] |
| `.moai/db/schema/migrations/0001~0006_*.sql` | 7 SPEC 누적 마이그레이션 — 본 SPEC은 마이그레이션 추가/수정 0건 (read-only API §1.4 HARD) | [EXISTING] |
| `apps/control-plane/go.mod` | 기존 직접 의존 무변경 — 신규 외부 의존 추가 0건(`database/sql`·`pgx`·`encoding/json`·`net/http`·`go.uber.org/zap`·`github.com/google/uuid` 등 7 SPEC 누적 의존만 사용) | [EXISTING] |

### 2.3 Drift-Guard Manifest

[NEW]만 신규 생성, [MODIFY]는 정확히 4파일·라우트 마운트(server.go ≈7줄)·SELECT 메서드 추가(store.go/pg_store.go/audit_query.go)·errors.go 신규 2 센티넬 한정. [EXISTING]은 0 diff. 구현 중 [EXISTING] 파일에 1줄이라도 수정 발생 시 consumer-only [HARD] 위반 → 즉시 중단·재계획(plan.md §7 R-AUDIT-QUERY-002).

- 신규 생성 허용: `cmd/server/audit_query_handlers.go`, `cmd/server/audit_query_handlers_test.go`, `internal/store/audit_query.go`, `internal/store/audit_query_test.go`
- 수정 허용(라우트 마운트 한정, ≈7줄): `cmd/server/server.go` (`auditQueryH` 필드 + `NewAuditQueryHandler(...)` 호출 + `innerMux.Handle("/api/v1/audit-logs", ...)` + `innerMux.Handle("/api/v1/audit-logs/", ...)` 2줄 + ko 주석)
- 수정 허용(SELECT 인터페이스/구현 한정): `internal/store/store.go` (AuditQueryStore 또는 WorkflowStore.QueryAuditLogs 추가 — OPEN #5), `internal/store/pg_store.go` (`QueryAuditLogs` 구현)
- 수정 허용(센티넬 신규 정확히 2건): `internal/errors/errors.go` (`ErrAuditQueryInvalidFilter`/`ErrAuditQueryInvalidTimeRange` 추가, **기존 무수정** — SCORE-API-001 drift lesson [HARD] EXPLICIT 부착)
- 수정 절대 금지(0 diff 검증): `internal/audit/*.go`(Action 상수·Recorder·Event 무변경), `internal/auth/*.go`(**frozen rbac.go [CRITICAL]** — RoleAuditor 신설 절대 금지, permissionMatrix 무변경), `cmd/server/score_handlers.go`/`report_handlers.go`/`review_handlers.go`/`rubric_handlers.go`/`evidence_handlers.go`, `internal/store/pg_store.go`의 audit INSERT 부분(`pg_store.go:346-381`), `.moai/db/schema/**`(0001~0006 마이그레이션 + initial.sql), `go.mod`/`go.sum`(신규 외부 의존 0)

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건)

Ubiquitous 요구사항은 7 SPEC 누적 canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track 패턴을 도메인 스코프 형태(`REQ-AUDIT-QUERY-UBI-NNN`)로 적용한다.

- **REQ-AUDIT-QUERY-UBI-001 (데이터 주권)**: The audit log query HTTP API layer SHALL NOT make any external service call (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) on any request path. 모든 영속·조회·필터링은 단일 내부 PostgreSQL pgx pool(store 위임)에만 위임하며, 검색 쿼리 빌더·페이지네이션·결과 직렬화는 신규 외부 의존 없이 표준 라이브러리(`database/sql`/`pgx`/`encoding/json`)로 수행한다(`tech.md` §9.1 망분리 정합, research.md §7.1).
- **REQ-AUDIT-QUERY-UBI-002 (감사 가능성 확장 — read-only이므로 자체 audit 0)**: For every request, the audit log query API layer SHALL NOT itself INSERT any `audit_logs` row, AND SHALL rely on the 7 누적 SPEC stores which already recorded each mutation event in the same database transaction at mutation time (`RecordXxx`, research.md §6.1/§6.2/§6.3). 본 SPEC은 검색 가능성(searchability)을 추가하며 적재(audit logging)는 추가하지 않는다. 검색 요청 자체를 audit_logs에 기록하는 "audit-of-audit-read"는 §5 #2(범위 밖 — 후속 SPEC).
- **REQ-AUDIT-QUERY-UBI-003 (권한 — admin-only narrowing, viewer/analyst 거부)**: The audit log query API layer SHALL permit only `RoleAdmin` authenticated principals to read audit logs via the SPEC-AX-AUTH-003 ABAC narrowing path AND a new handler-level `requireAuditQueryReadRole` helper (`score_handlers.go:161-187 requireScoreWriteRole` 패턴 동형 — `auth.ParseRolesFromScope` 활용, `RoleAdmin` 단일 매핑), AND SHALL return `403 ABAC_CONDITION_DENIED` for authenticated `viewer`/`analyst` principals. **frozen rbac.go·permissionMatrix 0-diff 자연 성립** — `RoleAdmin`은 `rbac.go:20-21`에 기존재, 신규 `RoleAuditor` 신설 0(REQ-AUDIT-QUERY-UBI-003-S1 — SCORE-API-001 §6 OPEN #4 본 SPEC 비발생).
- **REQ-AUDIT-QUERY-UBI-004 (cli-anonymous 기본값 + auth-disabled fallback)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the audit log query API layer SHALL serve all audit query endpoints with ABAC/RBAC middleware transparently passing through (`abac.go:8` REQ-ABAC-009 정합) AND `requireAuditQueryReadRole` helper transparently passing(`auth.UserFromContext` ok=false → true 반환, `score_handlers.go:181` 패턴 동형), AND SHALL NOT fabricate or leak any real user identifier — store가 부여한 `cli-anonymous` 기본값 계약을 그대로 따른다(research.md §7.2).

### 3.2 REQ-AUDIT-QUERY-001 — 감사 로그 검색 API (Read Endpoints)

조회 엔드포인트 그룹: 감사 로그 검색(GET, read-only). 모두 read-only, **admin only** narrowing(§1.5). 핸들러가 `WorkflowStore.QueryAuditLogs`(또는 `AuditQueryStore.QueryAuditLogs` — OPEN #5) 호출 → 5-필터 AND 조합 동적 쿼리 → offset/limit 페이지네이션 결과 + total count. 구체 경로/응답 JSON 형식/페이지네이션 기본값은 §6 OPEN #3 미확정 — strategy phase 확정.

#### Event-driven

- **REQ-AUDIT-QUERY-001-E1**: WHEN an `admin` caller issues a search request with zero or more of the 5 filters (`action`, `resource_type`, `resource_id`, `user_id`, time range via `since`/`until`), THEN the API SHALL (1) open a `WorkflowTx` via `WorkflowStore.BeginTx` (또는 dedicated `AuditQueryStore.BeginTx` — OPEN #5), (2) invoke `QueryAuditLogs(ctx, filter, limit, offset)` which dynamically builds a `WHERE` clause with parameterized bindings (SQL injection 0건), `ORDER BY timestamp DESC, user_id` (audit_logs_user_id_timestamp_idx 활용 — `initial.sql:138`), `LIMIT ? OFFSET ?`, (3) return `200 OK` with `{"events":[...], "count":N, "total":M}` (정확 schema §6 OPEN #3) where each event includes id/user_id/action/resource_id/resource_type/timestamp/details (정확 JSON 형식 §6 OPEN #3).
- **REQ-AUDIT-QUERY-001-E2**: WHEN a search request returns an empty result set (필터가 매칭하는 row 0건), THEN the API SHALL return `200 OK` with `{"events":[], "count":0, "total":0}` rather than 404 or 500 (data-completeness 원칙 — research.md §14.5; REPORT-001 빈 리포트 패턴 정합).

#### State-driven

- **REQ-AUDIT-QUERY-001-S1**: WHILE the SPEC-AX-AUTH-003 ABAC/RBAC chain admits an `admin`-authorized principal (or auth is disabled), AND the handler-level `requireAuditQueryReadRole` permits the role, the API SHALL serve all audit query endpoints without denial (admin-only narrowing — §1.5, research.md §4.2).
- **REQ-AUDIT-QUERY-001-S2**: WHILE processing 5-filter AND combinations, the API SHALL apply each non-nil filter as an additional `AND` predicate in the dynamic SQL WHERE clause with parameterized binding (no string interpolation — SQL injection 0건), AND SHALL skip nil filters (default-anything-goes — REPORT-001 빈 필터 정합).

#### Optional

- **REQ-AUDIT-QUERY-001-O1**: WHERE a search request supplies pagination query parameters (`limit`/`offset`), the API SHALL clamp them deterministically via `score_handlers.go:144 clampPagination` (defaultListLimit=50, maxListLimit=500, offset 음수→0 — `score_handlers.go:144-157` 재사용).

#### Unwanted

- **REQ-AUDIT-QUERY-001-U1**: IF a search request supplies a malformed `resource_id` (non-UUID), malformed `since`/`until` (non-RFC3339), `since > until`, or future timestamp (현재 시각 초과 — `audit.go:121-128 Event.Timestamp` 정합), THEN the API SHALL return `400 Bad Request` with the standard `{"error":{"code","message","field"}}` body (한국어 메시지, `score_handlers.go:74-101` 선례), SHALL NOT return `200` or `500` for these client errors, AND SHALL NOT proceed to BeginTx (pre-store validation — `ErrAuditQueryInvalidFilter`/`ErrAuditQueryInvalidTimeRange` 센티넬).
- **REQ-AUDIT-QUERY-001-U2**: IF a search request supplies an unknown `action` string (not in `audit.go:17-100` Action constants), THEN the API SHALL accept it as a literal string filter (no enum validation rejection — DB는 VARCHAR(64) AC `action`을 검증하지 않음, `initial.sql:118`) AND return `200 OK` with `{"events":[], ...}` if 0 rows match (not 400 — `audit_logs.action`은 자유 문자열).

### 3.3 REQ-AUDIT-QUERY-002 — ABAC admin-only narrowing 통합 (SPEC-AX-AUTH-003)

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 통합하며 그 코드를 수정하지 않는다(§1.4/§1.5 HARD). 본 SPEC은 mutation 엔드포인트가 0개이므로 write 거부 경로가 없으며, 대신 admin-only read narrowing이 핵심.

#### Event-driven

- **REQ-AUDIT-QUERY-002-E1**: WHEN an `admin`-role authenticated principal (with authentication enabled) issues an audit query request, THEN the SPEC-AX-AUTH-003 ABAC narrowing path SHALL admit it (admin은 `abac.go:94-99 hasAdminRole`에 의해 ABAC 우회), AND the handler-level `requireAuditQueryReadRole` SHALL return true (RoleAdmin 매핑 매치), AND the API SHALL serve the audit query result.

#### State-driven

- **REQ-AUDIT-QUERY-002-S1**: WHILE `authEnabled=false` (Walking Skeleton 기본값), the ABAC/RBAC middleware SHALL pass through transparently AND the handler-level `requireAuditQueryReadRole`/`guardAuditQueryRead` SHALL transparently pass(`UserFromContext` ok=false → 투과 — `score_handlers.go:181` 패턴 동형), so all audit query endpoints SHALL be served without authorization denial (`abac.go:8` REQ-ABAC-009, `server.go:287` 미들웨어 체인이 비활성 시 투과, research.md §4.1/§7).

#### Unwanted

- **REQ-AUDIT-QUERY-002-U1**: IF an authenticated `viewer` or `analyst` principal (with auth enabled) issues an audit query request, THEN the handler-level `requireAuditQueryReadRole` SHALL return false (RoleAdmin 비매핑) AND the API SHALL return `403 ABAC_CONDITION_DENIED` with the standard `{"error":{"code","message","field"}}` body (한국어, REPORT-001 SCORE-API-001 viewer 허용 정책과 정반대 — 감사 데이터 민감성).
- **REQ-AUDIT-QUERY-002-U2**: IF the implementation would introduce a new RBAC role (예: `RoleAuditor`) or modify `permissionMatrix`/`rbac.go` to grant audit query Permission, THEN that is OUT OF SCOPE — the implementation SHALL use the existing `RoleAdmin` mapping via handler-level helper `requireAuditQueryReadRole` (`score_handlers.go:161-187` 패턴 동형), preserving frozen `rbac.go` 0-diff (SCORE-API-001 §6 OPEN #4 lesson — 핸들러-레벨 RESOLVED 패턴).

### 3.4 REQ-AUDIT-QUERY-003 — store 에러→HTTP 매핑 & consumer-only 경계

본 SPEC은 신규 2 센티넬(`ErrAuditQueryInvalidFilter`/`ErrAuditQueryInvalidTimeRange`) + 기존 store 에러 센티넬을 HTTP status로 결정적으로 매핑하며, 비즈니스 로직·감사·스키마·신규 Action 상수·신규 외부 의존을 추가하지 않는다(§1.4 HARD).

#### State-driven

- **REQ-AUDIT-QUERY-003-S1**: WHILE handling any store error during audit query, the API SHALL map sentinels deterministically via `errors.Is` (`score_handlers.go:111-137 mapStoreErr` 선례 미러): `ErrAuditQueryInvalidFilter` (malformed UUID/length) → `400`, `ErrAuditQueryInvalidTimeRange` (since > until / future timestamp) → `400`, unwrapped/unknown DB error → `500` (research.md §8.1/§8.2). 검증/거부는 INFO 로그, 서버 결함은 ERROR 로그(`tech.md` §8.2 정합).

#### Unwanted

- **REQ-AUDIT-QUERY-003-U1**: IF a `WorkflowTx`(또는 `AuditQueryTx` — OPEN #5) is opened for read but a downstream `QueryAuditLogs` call fails, THEN the API SHALL ensure the opened read transaction is released via a `defer tx.Rollback(ctx)` (read-only — Commit 불필요; REPORT-001 D2-3 정합), SHALL return the mapped HTTP status, AND SHALL NOT leak goroutines beyond the request scope (research.md §3.3 read-only TX 정합 — `report_handlers.go:348/393` 선례).
- **REQ-AUDIT-QUERY-003-U2 (consumer-only 경계)**: IF implementation would require modifying any file under `internal/audit/`, `internal/auth/`, `cmd/server/score_handlers.go`/`report_handlers.go`/`review_handlers.go`/`rubric_handlers.go`/`evidence_handlers.go`, `.moai/db/schema/**`, the audit INSERT path in `pg_store.go:346-381`, or adding a new external dependency to `go.mod` (예: CSV/XLSX export 라이브러리) or a new `audit.Action` constant or `Recorder` method, THEN that change is OUT OF SCOPE and the API SHALL instead be redesigned to compose the existing contract unchanged (consumer-only §1.4 HARD; Drift-Guard manifest §2.3; AC-AUDIT-QUERY-BOUNDARY-1로 검증).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 데이터 주권 (망분리) | 검색 경로의 외부 API 호출 0건. 단일 내부망 PostgreSQL pgx pool(store 위임)만 사용. 신규 외부 의존 0 | §3.1 REQ-AUDIT-QUERY-UBI-001, research.md §7.1, orchestrator ground-truth grep (go.mod 직접 의존 0건 추가) |
| 감사 가능성 (read-only) | read-only이므로 mutation 0 → API 자체 audit INSERT 0건. mutation audit는 7 SPEC store가 동일-TX로 이미 기록 | §3.1 REQ-AUDIT-QUERY-UBI-002, research.md §6.1/§6.3 |
| consumer-only 무변경 | `internal/audit|auth`, `score|report|review|rubric|evidence_handlers.go`, `.moai/db/schema/**`, `pg_store.go:346-381`(audit INSERT 부분), `go.mod` 0 diff. 신규 마이그레이션 0, 신규 Action 상수 0, 신규 Recorder 메서드 0, 신규 외부 의존 0 | §1.4 HARD, §2.3 Drift-Guard |
| **frozen rbac.go 0-diff [HARD]** | `rbac.go:20-25` 3-role 고정, `rbac.go:33` 정규식 무변경, `permissionMatrix` 무변경, **RoleAuditor 신설 절대 금지**. admin 매핑은 핸들러-레벨 helper로 처리(SCORE-API-001 OPEN #4 lesson 동형) | §1.4/§1.5 HARD, REQ-AUDIT-QUERY-002-U2 |
| 한국어 | 모든 에러 메시지 한국어 (`score_handlers.go`/`report_handlers.go` 선례) | research.md §8.2 |
| ABAC admin-only narrowing | viewer/analyst 403, admin 200, auth-disabled→투과, write 게이트 0 | §3.3, research.md §4.2 |
| SQL injection 방어 | 5-필터 AND 동적 SQL은 파라미터 바인딩만(string interpolation 0), `pgx` placeholder($1, $2 ...) | §3.2 REQ-AUDIT-QUERY-001-S2 |
| 빈/누락 데이터 | 결과 0건 → `{"events":[], "count":0, "total":0}` 200 (data-completeness) | §3.2 REQ-AUDIT-QUERY-001-E2 |
| 페이지네이션 | `clampPagination` 재사용 — default 50, max 500, offset 음수→0 | §3.2 REQ-AUDIT-QUERY-001-O1, score_handlers.go:144 |
| read-only TX | 읽기용 `BeginTx` defer Rollback(Commit 불필요), goroutine 누출 0 | §3.4 REQ-AUDIT-QUERY-003-U1, research.md §3.3 |
| 인덱스 활용 | `audit_logs_user_id_timestamp_idx ON (user_id, timestamp DESC)` (initial.sql:138) 활용 위해 ORDER BY `timestamp DESC, user_id` | §3.2 REQ-AUDIT-QUERY-001-E1, initial.sql:138 |
| pgx pool 재사용 | `WorkflowStore.BeginTx`(→`PgWorkflowStore.pool`)만. `postgres.go` 死 스텁 비대상 | §1.4 HARD, pg_store.go source-verified |
| 성능 — 검색 응답 | p99 < 200ms 목표(B-tree index 활용 시 — 한국 공공 시간 제약, research.md §8.6 정합) | §3.2, research.md §8 |
| 로깅 | 구조화 JSON 로그(zap), 검증/거부는 INFO, 서버 결함은 ERROR | research.md §8.2, `tech.md` §8.2 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR), harness: thorough, sub-agent mode | `quality.yaml` development_mode |
| Go 도구 | go vet, golangci-lint (default + gosec), goimports | `.claude/rules/moai/languages/go.md` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **DB 스키마·FK·마이그레이션 변경** — 본 SPEC은 읽기 전용 API 계층이므로 `audit_logs` 스키마(`initial.sql:115-123`)·인덱스(`audit_logs_user_id_timestamp_idx` `initial.sql:138`) 변경, 신규 인덱스 추가, 신규 마이그레이션 추가/수정을 **하지 않는다**. 데이터 모델은 SPEC-AX-CTRL-001이 이미 제공했다(SPEC-AX-REPORT-001 §5 #1 정합).
2. **audit-of-audit-read (자체 read 추적)** — 본 SPEC은 read-only이므로 mutation 0 → audit 이벤트 0. **검색 요청 자체를 audit_logs에 기록하는 "감사 로그 read 행위에 대한 별도 audit row"는 본 SPEC 범위 밖** — 한국 공공 기관에서 감사 데이터 read 자체를 추적해야 한다는 요구가 있을 수 있으나 PoC에서 over-engineering 회피, 후속 SPEC(SPEC-AX-AUDIT-READ-AUDIT-001 가능) 책임. 검색 API는 별도 audit row를 INSERT하지 않으며 audit 스키마·Recorder를 변경하지 않는다(REQ-AUDIT-QUERY-UBI-002, research.md §6).
3. **mutation(write) 엔드포인트** — `audit_logs` 행 생성/수정/삭제 API는 본 SPEC 범위 밖이며 7 SPEC store(`InsertAuditLog` 동일 TX)가 이미 제공한다. 본 SPEC은 검색(GET)만 노출하고 write-role 게이트(`score_handlers.go:161-187`)를 차용하지 않는다(§1.5).
4. **검색 결과 스냅샷 영속화** — 검색 결과를 테이블/캐시/파일에 영속하는 스냅샷·머티리얼라이즈드 뷰·검색 이력 저장은 본 SPEC 범위 밖. 본 SPEC은 매 요청 store 조회로 on-the-fly 산출만 한다(research.md §6.1 read-only 정합, REPORT-001 §5 #4 정합).
5. **JSONB details 부분 검색** — `audit_logs.details JSONB`(`initial.sql:122`)의 부분 필드 검색(예: `details->>'eval_item_id'=...`로 evaluation_items hierarchy_code 역검색)은 §6 OPEN #4에서 결정. PoC 활성 vs 후속 SPEC 이연. AUD-1 UUIDv5 surrogate(`recorder.go:295-305`)와의 정합성은 OPEN #4 RESOLVED 시 명시.
6. **CSV/XLSX export** — 검색 결과의 CSV/XLSX 파일 다운로드 endpoint(`GET /api/v1/audit-logs/export.csv`)는 본 SPEC 범위 밖. JSON only — over-engineering 회피, 후속 SPEC(SPEC-AX-AUDIT-EXPORT-001 가능) 책임. `encoding/csv` 또는 `xuri/excelize` 신규 외부 의존 도입 0건(§1.4 HARD).
7. **6번째 시간 제약(KST 업무시간 09:00–18:00 검증)** — 한국 공공 6제약 중 시간 제약은 SPEC-AX-AUTH-003 / SCORE-API-001 / REPORT-001 §5 #7과 동일하게 본 SPEC 범위 밖(research.md §13.1). 본 SPEC은 데이터 주권/한국어/감사 가능성 확장/망분리/조직 격리(5제약)만 API 계층에서 보장한다.
8. **AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC** — 본 SPEC은 SPEC-AX-AUTH-003이 제공하는 ABAC narrowing(admin 우회, narrowing-only, auth-disabled 투과)을 **호출만** 한다. 정교한 org-unit 행 레벨 필터링(예: 본인 org의 audit_logs만 조회)·다중 속성 정책 엔진·RBAC `permissionMatrix`에 audit Permission 추가는 본 SPEC 범위 밖이며 AUTH-003 모델 확장은 미래 별도 SPEC 책임이다(§1.5 경계 노트, research.md §13.1).
9. **신규 RBAC 역할 신설** — `RoleAuditor` 또는 그 변종 신설은 **frozen rbac.go [HARD] 위반**으로 본 SPEC 범위 밖. admin-only 매핑은 핸들러-레벨 `requireAuditQueryReadRole` helper(`RoleAdmin` 단일 매핑 — SCORE-API-001 OPEN #4 lesson 동형)로 처리한다. `permissionMatrix` 확장 0건.
10. **SPEC-AX-CTRL-001 / SCORE-001 / REVIEW-001 / RUBRIC-001 / EVID-001 / EVAL-ITEM-001 / AUTH-003 코드 변경** — 위 SPEC들의 audit/auth/store/score-API/report/review/rubric/evidence 코드, DB 스키마, FK, 마이그레이션을 본 SPEC 구현 중 일절 수정하지 않는다(consumer-only §1.4 HARD, REQ-AUDIT-QUERY-003-U2). `permissionMatrix`/`Authorize`/`rbac.go` frozen, audit Action 상수 신설 0, Recorder 메서드 신설 0.
11. **Console UI / 클라이언트 SDK / OpenAPI 생성** — `apps/console/` 화면, 클라이언트 라이브러리, OpenAPI/Swagger 스펙 자동 생성은 본 SPEC 범위 밖. 본 SPEC은 server-side HTTP 핸들러 + 라우트 마운트 + store SELECT 메서드만 다룬다.

---

## 6. 의존성 및 전제 (OPEN — strategy phase + Human Gate sign-off 대상)

consumer-only [HARD] 0-diff·신규 마이그레이션 0·신규 Action 상수 0·신규 외부 의존 0·자체 audit 0·frozen rbac.go 0-diff는 결정과 무관하게 불변이다. §6.0 GREEN 전제·방어 게이트(S0 hard-verify)는 Run 진입 시에도 그대로 유지된다.

### 6.0 GREEN 전제 (검증 완료 — orchestrator ground-truth grep 2026-05-20)

- **SPEC-AX-CTRL-001 완료 GREEN**: `audit_logs` 테이블(`initial.sql:115-123`: id UUID PK / user_id VARCHAR(64) default 'cli-anonymous' / action VARCHAR(64) / resource_id UUID NOT NULL / resource_type VARCHAR(32) / timestamp TIMESTAMP WITH TIME ZONE default now() / details JSONB), 인덱스 `audit_logs_user_id_timestamp_idx ON audit_logs(user_id, timestamp DESC)` (`initial.sql:138`), `PgWorkflowTx.InsertAuditLog`(`pg_store.go:346-381` audit INSERT — id 자동생성/details JSONB/timestamp 보존) — source-verified.
- **7 SPEC 누적 Action 상수 23+개 GREEN**: `audit.go:17-100`에 WORKFLOW_CREATED/WORKFLOW_TRANSITIONED_TO_RUNNING/WORKFLOW_COMPLETED/WORKFLOW_FAILED_DISPATCH/WORKFLOW_FAILED_CALLBACK/TRANSITION_REJECTED/CALLBACK_REJECTED_TERMINAL/WORKFLOW_CREATE_CANCELLED/AUTH_FORBIDDEN/AUTH_LOGOUT/AUTH_REFRESH_REUSE_DETECTED/ABAC_CONDITION_DENIED/SERVER_STARTUP/SERVER_SHUTDOWN_INITIATED/SERVER_SHUTDOWN_COMPLETED/EVIDENCE_CREATED/EVIDENCE_VERSIONED/EVAL_ITEM_CREATED/EVAL_ITEM_UPDATED/SCORE_CREATED/SCORE_UPDATED/SCORE_REVIEW_REQUEST_CREATED/SCORE_REVIEW_REQUEST_REVIEWER_ASSIGNED/SCORE_REVIEW_REQUEST_APPROVED/SCORE_REVIEW_REQUEST_REJECTED/RUBRIC_CREATED/RUBRIC_UPDATED/RUBRIC_ARCHIVED/RUBRIC_CRITERION_ADDED/RUBRIC_BAND_ADDED 등 23+개 — 모두 source-verified, phantom 0건.
- **AUD-1 UUIDv5 surrogate GREEN**: `EvalItemAuditNamespace`(`audit.go:115` — `a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b`), `evalItemResourceID(hierarchy_code) → uuid.NewSHA1(...)` (`recorder.go:295-305`) — evaluation_items domain의 audit_logs.resource_id 직렬화 전략. resource_id 단독으로 hierarchy_code 역검색 불가 — details JSONB의 `eval_item_id` 필드 활용 필요(OPEN #4).
- **SPEC-AX-AUTH-003 완료 GREEN**: `ErrCodeABACDenied="ABAC_CONDITION_DENIED"`(abac.go:24), narrowing-only/admin 우회/auth-disabled 투과(abac.go:4/8/9), `RoleAdmin`/`RoleAnalyst`/`RoleViewer`(rbac.go:20-25 — 3-role frozen [HARD]), 정규식 `^iroum-ax:(admin|analyst|viewer)$`(rbac.go:33), `ParseRolesFromScope`(rbac.go), `UserFromContext`(middleware.go:44-49), `hasAdminRole`(abac.go:94-99) — source-verified. **frozen 0-diff**.
- **SPEC-AX-SCORE-API-001 / REPORT-001 완료 선례 GREEN**: `ScoreHandler`/`ReportHandler` 구조·`Routes()`·`writeScoreJSON`/`writeScoreErr`/`scoreErrorBody`·`mapStoreErr`·`clampPagination`(`score_handlers.go:144-157`)·`requireScoreWriteRole`/`guardScoreWrite`(`score_handlers.go:161-190`)·`server.go:55/216/277-278` 라우트 마운트 패턴(`score_handlers.go:43-137`·`report_handlers.go`·`server.go` source-verified). write-role 게이트는 mutation 0이므로 미차용, **read-role 게이트(`requireAuditQueryReadRole`/`guardAuditQueryRead`)는 동형 패턴 신규 추가**.
- **단일 pgStore + 신규 SELECT 메서드 1개 추가**: `PgWorkflowStore`가 audit_logs INSERT를 동일-TX로 이미 제공(`pg_store.go:346-381 InsertAuditLog`). 본 SPEC은 read-only SELECT `QueryAuditLogs` 1개 추가(§2.1 [MODIFY]) — store 신규 메서드 1개 추가는 consumer-only 정신 유지(SELECT 한정, INSERT/UPDATE/DELETE 0). **현재 HTTP 계층에 audit-query 핸들러 부재 — 본 SPEC이 첫 audit query consumer**.
- **Cross-SPEC artifact 영향 없음**: 본 SPEC은 위 7 SPEC들의 골든 파일·generated artifact·코드를 수정하지 않는다(clean additive — API 파일 2개 신규 + store 파일 2개 신규 + server.go 라우트 ≈7줄 + store.go/pg_store.go SELECT 추가 + errors.go 센티넬 2개 추가).
- **[방어 게이트] Run 진입 hard-verify**: consumer-only 전제(소비 시그니처 실재)를 보호하기 위해, Run 진입 시 `grep -n 'InsertAuditLog\|PgWorkflowTx\|Event' apps/control-plane/internal/store/pg_store.go apps/control-plane/internal/audit/audit.go` 및 `grep -n 'RoleAdmin\|hasAdminRole\|UserFromContext\|ParseRolesFromScope' apps/control-plane/internal/auth/*.go` 및 `grep -n 'audit_logs\|audit_logs_user_id_timestamp_idx' .moai/db/schema/initial.sql`를 hard-verify한다. **미존재/시그니처 불일치 시 consumer-only 불가 → 즉시 STOP·재계획**(plan.md §4 S0 명문 게이트, 메모리 lesson #9 흡수).

### 6.1 OPEN #1 → admin-only ABAC narrowing 매핑 결정성 [CRITICAL]

**문제**: 본 SPEC의 ABAC narrowing은 admin only이다(§1.5). `rbac.go:20-25` 3-role frozen에서 `RoleAdmin`은 기존재하나, **frozen `permissionMatrix`(rbac.go:39-)에는 audit/query Permission 항목이 부재**. SCORE-API-001 §6 OPEN #4 패턴(write-role 게이트가 frozen RBAC을 우회하기 위해 핸들러-레벨 helper 사용)을 본 SPEC도 답습할지(권장) vs `permissionMatrix`에 audit Permission을 추가하는 별도 SPEC을 선행할지의 결정성.

**옵션**:
- (a) **[권장]** 핸들러-레벨 `requireAuditQueryReadRole(scope) bool`(`auth.ParseRolesFromScope(scope)`에서 RoleAdmin 단일 매핑 매치) + `guardAuditQueryRead(w, r) bool`(`score_handlers.go:161-187 requireScoreWriteRole`/`guardScoreWrite` 동형 미러)을 `audit_query_handlers.go`에 정의. ABAC chain은 통과시키되 handler-level에서 viewer/analyst 추가 거부 — narrowing-only 정신 정합(`abac.go:4`). **frozen rbac.go/permissionMatrix 0-diff 자연 성립** — SCORE-API-001 §6 OPEN #4 lesson 동형, 본 SPEC도 충돌 없음.
- (b) (REJECTED) `rbac.go:permissionMatrix`에 `PermissionAuditQuery` Permission 추가 + `RoleAdmin` 매핑. → frozen rbac.go [HARD] 위반.
- (c) (REJECTED) ABAC `OrgUnitCondition` 등 신규 ABAC 조건을 `abac.go`에 추가. → frozen abac.go [HARD] 위반.

**잔여 가정**: `auth.ParseRolesFromScope`가 정규식 `^iroum-ax:(admin|analyst|viewer)$`(`rbac.go:33`)로 RoleAdmin을 정확히 식별. → RED 첫 테스트에서 RoleAdmin scope("iroum-ax:admin")와 RoleViewer scope("iroum-ax:viewer") 두 케이스로 차별 검증.

### 6.2 OPEN #2 → 필터 조합 정책 (AND only vs OR 지원)

**문제**: 5-필터(action/resource_type/resource_id/user_id/time range) 조합은 AND only로 한정할 것인지 OR 지원할 것인지.

**옵션**:
- (a) **[권장]** AND only (`WHERE a AND b AND c ...`) — PoC 단순, 동적 SQL 빌더 minimal, SCORE-API-001 list filter 패턴(`score_handlers.go:283-333 handleListScores`) 정합.
- (b) (REJECTED) OR 지원 (예: `action IN (...) OR user_id IN (...)`) — query string syntax 복잡(`action=X,Y&user_id=Z` → 모호), PoC over-engineering.
- (c) (REJECTED) GraphQL-style filter expression — 신규 외부 의존(graphql 파서) 필요, §1.4 HARD 위반.

**잔여 가정**: 사용자 실제 요구가 OR 조합(예: "admin 또는 analyst의 모든 SCORE_*")이라면 후속 SPEC 또는 클라이언트 측 다중 쿼리로 해결.

### 6.3 OPEN #3 → 응답 schema + 페이지네이션 default + endpoint URL

**문제**: (1) 응답 JSON 구조 (event 필드 표면화 정도, total count 포함 여부, generated_at 메타), (2) 페이지네이션 기본값(default 50 vs 100 vs 30), (3) endpoint URL(`/api/v1/audit-logs` vs `/api/v1/audits` vs `/api/v1/admin/audit-logs`), (4) 빈/누락 표면화.

**옵션 (1) 응답 schema**:
- (a) **[권장]** `{"events":[{id, user_id, action, resource_id, resource_type, timestamp, details}], "count":N, "total":M, "generated_at": "..."}` — 전 필드 표면화, total count 포함(페이지네이션 UX), generated_at 메타.
- (b) `{"events":[...], "count":N}` (total 미포함 — 별도 쿼리 회피) — UX 손해.
- (c) Pagination cursor (next_cursor) 추가 — over-engineering.

**옵션 (2) 페이지네이션 default**:
- (a) **[권장]** SCORE-API-001 정합 default 50, max 500 — `clampPagination` 재사용 즉시 활용.
- (b) audit 도메인 특성(시간 역순 다량 조회)으로 default 100 — `clampPagination` 별도 설정 필요.

**옵션 (3) endpoint URL**:
- (a) **[권장]** `/api/v1/audit-logs` (audit_logs 테이블명 정합, REST 명사 복수) — `innerMux.Handle("/api/v1/audit-logs", ...)` + `innerMux.Handle("/api/v1/audit-logs/", ...)` 2줄.
- (b) `/api/v1/audits` (도메인 명사) — 표 이름과 불일치.
- (c) `/api/v1/admin/audit-logs` (admin prefix) — URL에 권한 인코딩, REST 안티패턴.

**옵션 (4) 빈/누락**:
- (a) **[권장]** 빈 결과 → `200 OK` `{"events":[], "count":0, "total":0}` (data-completeness — REPORT-001 §6.3 B-2 정합).
- (b) 빈 결과 → `404 Not Found` — REST 안티패턴(필터 결과 0건은 not-found 아님).

### 6.4 OPEN #4 → JSONB details 부분 검색 (PoC 활성 vs 이연)

**문제**: `audit_logs.details JSONB`(`initial.sql:122`)의 부분 필드 검색 활성화 여부. evaluation_items domain은 `resource_id`에 UUIDv5 surrogate(`recorder.go:303-305`)를 저장하고 실 식별자(`eval_item_id`, `hierarchy_code` VARCHAR(64))는 `details->>'eval_item_id'`에 보존. 따라서 hierarchy_code로 audit 행을 역검색하려면 JSONB 검색 필요(`WHERE resource_type='evaluation_item' AND details->>'eval_item_id' = $1`).

**옵션**:
- (a) **[권장]** PoC 미적용(이연) — 5-필터(action/resource_type/resource_id/user_id/time range) AND only. JSONB details 부분 검색은 후속 SPEC. 이유: evaluation_items 검색 use case가 PoC 우선순위 아님, JSONB indexing(GIN) 추가 필요 시 마이그레이션 발생 — §1.4 HARD 위반.
- (b) PoC 활성 — `details_eval_item_id` 등 별도 query param 추가. → JSONB indexing 없이 full table scan 위험, 신규 GIN index 마이그레이션 필요(§5 #1 위반).

**잔여 가정**: admin 사용자가 hierarchy_code로 audit 검색하려면 `resource_type='evaluation_item' AND user_id=...`로 우회 가능. 사용자 만족도 검증은 PoC 후속.

### 6.5 OPEN #5 → store 메서드 위치 결정성 (WorkflowStore vs 신규 AuditQueryStore)

**문제**: `QueryAuditLogs` SELECT 메서드를 (a) 기존 `WorkflowStore` 인터페이스에 추가 vs (b) 신규 `AuditQueryStore` 인터페이스 + 별도 `AuditQueryTx` 도입.

**옵션**:
- (a) **[권장]** 기존 `WorkflowStore` 인터페이스에 `QueryAuditLogs(ctx, filter, limit, offset) (events, total, err)` 추가 + `WorkflowTx.QueryAuditLogs(...)` 동등 메서드. → store.go [MODIFY] 1파일·1메서드, pg_store.go [MODIFY] 1메서드 구현. 단순.
- (b) 신규 `AuditQueryStore` 인터페이스 + `AuditQueryTx` + `PgWorkflowStore.BeginAuditQueryTx` 도입. → REPORT-001 패턴(EvalItemStore≠ScoreStore 분리) 정합이나 PoC over-engineering(audit 검색만이라 별도 인터페이스 가치 낮음).
- (c) `WorkflowTx`는 무변경, `audit_query.go`에 stateless function `QueryAuditLogs(ctx, pool, filter, limit, offset)` 정의 — interface 회피. → tx-aware 검증 어려움, 미러 패턴 비정합.

**잔여 가정**: (a) 채택 시 `WorkflowTx`가 INSERT(`InsertAuditLog`) + SELECT(`QueryAuditLogs`)를 모두 보유 → 기존 mutation TX 코드 영향 0 확인 필요(grep 검증).

### 6.6 OPEN #6 → total count 산출 방식 (COUNT(*) OVER() vs 별도 쿼리)

**문제**: 총 매칭 행 수(`total`) 산출을 (a) 동일 쿼리에 `COUNT(*) OVER()` window function 추가 vs (b) 별도 `SELECT COUNT(*) FROM audit_logs WHERE ...` 쿼리.

**옵션**:
- (a) **[권장]** `COUNT(*) OVER()` window function — 1 round-trip, 결과·total 동시 산출, PostgreSQL native 기능.
- (b) 별도 `SELECT COUNT(*)` 쿼리 — 2 round-trip, 정확한 isolation(SERIALIZABLE 가정 시 일치 보장 어려움).
- (c) total 미산출(OPEN #3 (1)(b) 채택 시) — UX 손해 + `clampPagination`만으로 부족.

**잔여 가정**: window function `COUNT(*) OVER()`의 성능은 audit_logs row count 100K 미만 PoC scope에서 충분. p99 < 200ms 목표.

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- **SPEC-AX-CTRL-001 audit_logs 적재 로직**: audit_logs INSERT 동작·`audit_logs_user_id_timestamp_idx` 인덱스 정의·`InsertAuditLog`(`pg_store.go:346-381`)는 CTRL-001이 이미 GREEN으로 제공한다. 본 SPEC은 그 데이터를 **검색·노출만** 하며 INSERT 로직·스키마·인덱스를 재구현/수정하지 않는다.
- **7 SPEC 누적 Recorder 메서드**: EVID/EVAL-ITEM/SCORE/REVIEW/RUBRIC `RecordXxx` 메서드는 각 SPEC이 제공. 본 SPEC은 mutation을 발생시키지 않으므로 `Recorder` 의존을 주입하지 않는다.
- **AUD-1 UUIDv5 surrogate (EVAL-ITEM-001)**: `evalItemResourceID(hierarchy_code) → uuid.NewSHA1(...)` (`recorder.go:295-305`)는 EVAL-ITEM-001 책임. 본 SPEC은 검색에서 resource_id UUID 필터를 수용만 하고 surrogate 변환은 OPEN #4(PoC 미적용)에서 결정.
- **DB 스키마/FK/마이그레이션**: `audit_logs` 스키마(`initial.sql:115-123`)·인덱스(`initial.sql:138`)·0001~0006 마이그레이션, FK 하드닝, 신규 `.sql` 추가는 본 SPEC 범위 밖(§5 #1).
- **frozen RBAC 확장**: `rbac.go:20-25` 3-role, `permissionMatrix`에 audit Permission 추가, `RoleAuditor` 신설은 본 SPEC 범위 밖이며 frozen rbac.go [HARD] 위반(§5 #9).
- **풀 audit 분석 / 통계 / 시각화**: audit 데이터 집계(예: action별 빈도, user_id별 활동)는 본 SPEC 범위 밖 — 검색만 제공, 분석/시각화는 후속 SPEC 또는 Console UI 책임.
- **Console UI / SDK / OpenAPI 생성**: server-side 핸들러만(§5 #11).
- **6번째 시간 제약(KST 업무시간)**: AUTH-003/SCORE-API-001/REPORT-001 정합 — 본 SPEC 범위 밖(§5 #7).

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/cmd/server/audit_query_handlers_test.go` — `httptest.NewRequest`/`httptest.NewRecorder`, 검색 엔드포인트 정상/에러 경로, 테이블 테스트, testify/assert, t.Parallel, goleak
- store 단위 테스트: `apps/control-plane/internal/store/audit_query_test.go` — `QueryAuditLogs` 결과 정확성, 5-필터 AND 조합, 페이지네이션 슬라이싱, total count 정확성, ORDER BY 결정성
- store 모킹: fake `WorkflowStore`/`WorkflowTx`(또는 `AuditQueryStore` — OPEN #5)로 핸들러 격리
- 데이터 주권 검증: 검색 경로 외부 네트워크 egress 0건 (코드 정적 검사 — 신규 외부 import 0)
- 감사 비-발생 검증: read-only 핸들러가 `audit_logs` INSERT·`Recorder` 호출 0건 (API 코드에 audit SQL 0건, recorder 의존 미주입)
- **frozen rbac.go 0-diff 검증**: `git diff rbac.go permissionMatrix` 0건, RoleAuditor 신설 0, 정규식 `^iroum-ax:(admin|analyst|viewer)$` 무변경
- ABAC admin-only narrowing 검증: admin 200 / viewer 403 / analyst 403 / auth-disabled→투과 / write 게이트 부재(코드에 `requireScoreWriteRole` 차용 없음)
- 검색 정확성 검증: 5-필터 AND 조합 결과 정확, action 자유 문자열 수용, malformed UUID/timestamp 400, since>until 400, future timestamp 400
- 빈/누락 검증: 결과 0건 → `{"events":[], "count":0, "total":0}` 200, 404/500 비반환
- SQL injection 방어 검증: pgx placeholder($1...) 사용, string interpolation 0건
- 인덱스 활용 검증: `EXPLAIN` 또는 SQL 정적 검사로 `audit_logs_user_id_timestamp_idx` 활용 (`ORDER BY timestamp DESC, user_id`)
- 페이지네이션 검증: `clampPagination` 재사용, default 50 / max 500 / offset 음수→0
- read-only TX 검증: `BeginTx` defer Rollback, 실패 시 부분 상태 0, goroutine 누출 0 (goleak)
- consumer-only 경계 검증: `internal/audit|auth`, `score|report|review|rubric|evidence_handlers.go`, `.moai/db/schema/**`, `pg_store.go:346-381`(audit INSERT), `go.mod` 0 diff (AC-AUDIT-QUERY-BOUNDARY-1 — git diff/Drift-Guard manifest)
- 회귀: 기존 `score_handlers.go`·`report_handlers.go`·`review_handlers.go`·`rubric_handlers.go`·`evidence_handlers.go`·workflow REST 핸들러 테스트가 audit-logs 라우트 마운트 + QueryAuditLogs 추가 후에도 GREEN 유지

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.

---

## 9. Definition of Done (SPEC 단계)

- [ ] frontmatter 8-field canonical (plan.md L378) 준수, HISTORY + Schema note 포함
- [ ] EARS 4개 REQ 모듈(UBI 묶음 4 + 3 modal: 001 검색 API / 002 ABAC admin-only narrowing / 003 에러매핑·경계) 모두 E/S/O/U 분류 명시, 모듈 ≤5 (read-only — mutation REQ 0)
- [ ] §5 Exclusions ≥1 (DB/스키마/FK·신규마이그레이션, audit-of-audit-read, mutation 엔드포인트, 스냅샷 영속, JSONB부분검색, CSV/XLSX export, 6번째 시간제약, AUTH-003 모델 초과 ABAC, 신규 RBAC 역할 신설, 7-SPEC 코드변경, Console/SDK — 11항목)
- [ ] §1.4 consumer-only 계약: audit/auth/7-SPEC-handlers/0001-0006-migrations/pg_store.go audit INSERT 부분/go.mod **0 diff**, 신규 마이그레이션 0, 신규 Action 상수 0, 신규 Recorder 메서드 0, 신규 외부 의존 0, 자체 audit 0 — §1/§2.3/§3.4-U2/§7에 명시
- [ ] §1.4/§1.5 **frozen rbac.go 0-diff [CRITICAL]** — RoleAuditor 신설 절대 금지, permissionMatrix 무변경, admin-only는 핸들러-레벨 helper(SCORE-API-001 §6 OPEN #4 lesson 동형) — §1.4/§1.5/§3.3/§5 #9/§6.1에 명시
- [ ] §1.5 ABAC admin-only narrowing 경계 — viewer/analyst 403, admin 200, auth-disabled→투과, write-role 미차용 명시
- [ ] §6 OPEN 6건 (#1 [CRITICAL] admin-only narrowing 매핑 결정성 / #2 필터 조합 AND vs OR / #3 응답 schema + 페이지네이션 default + endpoint URL + 빈/누락 / #4 JSONB details 부분 검색 PoC 활성 vs 이연 / #5 store 메서드 위치 (WorkflowStore vs 신규 AuditQueryStore) / #6 total count 산출 (COUNT(*) OVER() vs 별도 쿼리)) — strategy phase + Human Gate RESOLVED 대상으로 명시. 권장 옵션 부착
- [ ] §2 [DELTA]: [NEW] audit_query_handlers.go/_test.go + audit_query.go/_test.go, [MODIFY] server.go(라우트 마운트 ≈7줄) + store.go(인터페이스 추가 OPEN #5) + pg_store.go(SELECT 구현) + errors.go(센티넬 2건 — SCORE-API-001 drift lesson EXPLICIT 부착), [EXISTING] consumer 무변경 + Drift-Guard manifest
- [ ] acceptance.md 각 REQ ≥2 G/W/T, AC 명명 `AC-AUDIT-QUERY-{REQ}-{N}`, AC 25 / §7 edge 14 count가 acceptance.md §9 · spec.md §9 · spec-compact.md 3문서 cross-file 일관 (plan.md §8은 plan-phase DoD로 count 미운반 — SCORE-API-001 D3-1 정합)
- [ ] spec.md에 함수명/클래스 구조/API 스키마 상세 구현 미기재 (WHAT/WHY only — store 시그니처는 소비 계약 검증 근거로만 인용)
- [ ] phantom API 0 — 전 소비 시그니처 source-verified (audit.go:17-128 / recorder.go:295-305 / pg_store.go:346-381 / abac.go:4-99 / rbac.go:20-33 / middleware.go:25-49 / score_handlers.go:43-190 / report_handlers.go:43-72 / server.go:55-220/277-285 / initial.sql:115-138 / go.mod) 명시 (메모리 lesson #9)
- [ ] D1 iter2 lesson 명시적 acknowledge (read-only — NewRecorder mode 무관, mutation 메서드 0 — 적용 0이지만 명시 의무) — §1.4 lesson 부착
- [ ] SCORE-API-001 errors.go drift lesson EXPLICIT 부착 — §2.1 + §2.3 양쪽 manifest 양쪽 신규 2 센티넬 명시
- [ ] REPORT-001 server.go ≈7줄 lesson 정확 기술 — §1.1/§2.1/§2.3 일관
- [ ] SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 — admin 매핑이 rbac.go:20-21에 기존재함을 §1.4/§1.5/§3.3/§6.1에 명시
- [ ] 구현 코드/테스트 미작성 (SPEC 문서만)
