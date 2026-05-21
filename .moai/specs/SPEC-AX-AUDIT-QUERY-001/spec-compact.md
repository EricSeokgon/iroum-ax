# SPEC-AX-AUDIT-QUERY-001 (Compact) — 감사 로그 검색 HTTP API 계층 (Audit Log Query/Search HTTP API Layer)

> v0.1.0 · status draft · TDD · thorough · brownfield · sub-agent. 상세는 spec.md / plan.md / acceptance.md / research.md. (AC=25, §7 edge case=14, §6 6건 OPEN — Run Phase strategy.md §A + Human Gate sign-off RESOLVED 대상)

## 핵심 consumer-only 계약 [HARD]

- 본 SPEC은 SPEC-AX-CTRL-001(완료, audit_logs 0001) / SCORE-001(완료 v0.1.3) / SCORE-API-001(완료 v0.1.1) / REPORT-001(완료 v0.1.1) / REVIEW-001(완료) / RUBRIC-001(완료) / EVID-001(완료) / EVAL-ITEM-001(완료, AUD-1 UUIDv5 surrogate) / AUTH-003(완료, ABAC) / OBS-001(완료) / SERVER-001(완료)의 **순수 consumer** — `internal/audit/`, `internal/auth/`, `cmd/server/{score, report, review, rubric, evidence}_handlers.go`, `internal/store/pg_store.go:346-381`(audit INSERT 부분), `.moai/db/schema/**`(0001~0006 + initial.sql), `go.mod` **0 diff**
- 신규 DB 마이그레이션 0 (read-only API — `audit_logs`는 CTRL-001/0001가 이미 생성, `audit_logs_user_id_timestamp_idx` `initial.sql:138`)
- 신규 `audit.Action` 상수 0, 신규 `Recorder` 메서드 0 (read-only — `RecordXxx` 패턴 비차용)
- 신규 외부 의존 0 — CSV/XLSX export 라이브러리(§5 #6) 도입 0, 검색은 표준 `database/sql`/`pgx`만
- API 자체 audit 0 — **read-only이므로 mutation 0 → audit 이벤트 0** (mutation audit는 7 SPEC store가 동일 TX로 이미 기록, research.md §6.1/§6.3). audit-of-audit-read는 §5 #2(범위 밖)
- **frozen rbac.go 0-diff [CRITICAL]** — `rbac.go:20-25` 3-role 고정(`RoleAdmin`/`RoleAnalyst`/`RoleViewer`), `rbac.go:33` 정규식 `^iroum-ax:(admin|analyst|viewer)$` 무변경, `permissionMatrix` 무변경, **RoleAuditor 신설 절대 금지** (§5 #9). admin-only 매핑은 핸들러-레벨 helper(SCORE-API-001 §6 OPEN #4 lesson 동형)
- TX 진입점 = `store.WorkflowStore.BeginTx`(→`PgWorkflowStore.BeginTx`)만 (OPEN #5 RESOLVED 후 정확). `postgres.go` 死 스텁 비대상
- ABAC: 기존 미들웨어 체인(`server.go:287` 와이어링)이 innerMux 전체 자동 적용 — server.go ABAC 변경 0. **admin only narrowing** (REPORT-001/SCORE-API-001 viewer 허용과 정반대 정책 — 감사 데이터 민감성)
- 1차 산출물 = 7 SPEC 누적 audit_logs 적재 데이터 + AUTH-003 ABAC admin-only narrowing 통합한 검색·필터·페이지네이션 가능 read-only REST API

## 감사 검색 엔드포인트 (research.md §14.2 — 잠정, §6 OPEN #3 확정)

GET `/api/v1/audit-logs`(read-only) — 5-필터 AND 조합(`action`/`resource_type`/`resource_id`/`user_id`/time range `since`+`until` — 모두 optional) + offset/limit 페이지네이션(`clampPagination` 재사용). base `/api/v1`. mutation 엔드포인트 0

## REQ 모듈 (4: 4 Ubiquitous 묶음 + 3 modal — read-only, mutation REQ 0)

| REQ-ID | EARS | 요지 |
|--------|------|------|
| REQ-AUDIT-QUERY-UBI-001 (데이터 주권) | Ubiquitous | 외부 호출 0, store(내부 pgx) 위임만, 신규 외부 의존 0 |
| REQ-AUDIT-QUERY-UBI-002 (감사 가능성 확장) | Ubiquitous | read-only이므로 mutation 0 → API 자체 audit 0 (mutation audit는 7 SPEC store가 이미 동일-TX 기록). audit-of-audit-read OUT |
| REQ-AUDIT-QUERY-UBI-003 (권한·admin-only narrowing) | Ubiquitous | admin only read 허용, viewer/analyst 403 — REPORT-001 viewer 허용과 정반대(감사 민감성), frozen rbac.go 0-diff 핸들러-레벨 helper로 |
| REQ-AUDIT-QUERY-UBI-004 (cli-anonymous) | Ubiquitous(State) | authEnabled=false 전 엔드포인트 + handler-level helper 모두 투과 + 실 식별자 비위조 |
| REQ-AUDIT-QUERY-001-E1~2/S1~2/O1/U1~2 | E/S/O/U | 검색: cross-store empty filter 200(E1)/빈 결과 200(E2), admin narrowing 통과(S1), 5-filter AND 동적 SQL parameter binding(S2), clampPagination 재사용(O1), malformed UUID/time/range 400(U1), unknown action 200 빈결과(U2) |
| REQ-AUDIT-QUERY-002-E1/S1/U1~2 | E/S/U | ABAC: admin 200 허용(E1), auth-disabled 투과(S1), viewer/analyst 403(U1), RoleAuditor 신설·permissionMatrix 추가 OUT(U2) |
| REQ-AUDIT-QUERY-003-S1/U1~2 | S/U | 에러 매핑: 센티넬→HTTP 결정적(S1, 정확 2 신규: ErrAuditQueryInvalidFilter/ErrAuditQueryInvalidTimeRange), read-only TX rollback(U1), consumer-only 0-diff·frozen rbac 0-diff 경계(U2) |

## AC (25: AC-AUDIT-QUERY-{REQ}-{N})

- §1 UBI-001: -1(외부호출 0+신규의존 0 정적), -2(store 위임)
- §2 UBI-002: -1(API 자체 audit 0 read-only), -2(mutation audit 7-SPEC 위임)
- §3 UBI-003: -1(admin scope 200), -2(viewer 403), -3(analyst 403)
- §4 UBI-004: -1(authEnabled=false 투과), -2(실 식별자 비위조)
- §5 REQ-001: -1(empty filter 200), -2(5-filter AND 200), -3(빈 결과 200), -4(malformed UUID 400), -5(malformed timestamp 400), -6(since>until 400), -7(future timestamp 400), -8(unknown action 200 빈결과), -9(clampPagination 재사용)
- §6 REQ-002: -1(admin 200 E1), -2(auth-disabled 투과 S1), -3(viewer 403 U1), -4(RoleAuditor 미신설·permissionMatrix 무변경 BOUNDARY U2)
- §7 REQ-003: -1(센티넬→HTTP 매핑 표), -2(read-only TX rollback BOUNDARY), BOUNDARY-1(consumer-only 0-diff)
- 분해 합 = 2+2+3+2+9+4+3 = 25 = 물리 heading 25. §7 edge=14. 각 modal REQ ≥2 AC

## Files to Modify

| 경로 | Delta |
|------|-------|
| `cmd/server/audit_query_handlers.go` | [NEW] AuditQueryHandler(WorkflowStore 의존, recorder 미주입)+Routes+검색 핸들러+5-filter 파싱+`requireAuditQueryReadRole`/`guardAuditQueryRead`(admin-only handler-level, score_handlers.go:161-187 동형)+JSON/에러 헬퍼+에러 매핑+`clampPagination` 재사용 |
| `cmd/server/audit_query_handlers_test.go` | [NEW] httptest 핸들러 단위 테스트 31+ |
| `internal/store/audit_query.go` | [NEW] `QueryAuditLogs(ctx, filter, limit, offset) (events, total, err)` 구현 — 5-filter AND 동적 SQL(parameter binding), ORDER BY timestamp DESC user_id, COUNT(*) OVER() total |
| `internal/store/audit_query_test.go` | [NEW] store SELECT 단위 테스트 8+ |
| `internal/store/store.go` | [MODIFY] AuditQueryFilter struct + WorkflowStore.QueryAuditLogs 인터페이스 추가 (OPEN #5 §A.5) — 기존 메서드 무수정 |
| `internal/store/pg_store.go` | [MODIFY] PgWorkflowStore.QueryAuditLogs 위임 호출 — audit INSERT 부분(`pg_store.go:346-381`) **0-diff** |
| `internal/errors/errors.go` | [MODIFY] 정확 2 신규 sentinel: `ErrAuditQueryInvalidFilter`, `ErrAuditQueryInvalidTimeRange` — SCORE-API-001 drift lesson [HARD] EXPLICIT 부착(§2.1+§2.3 양쪽 manifest) |
| `cmd/server/server.go` | [MODIFY] **라우트 마운트만** (`auditQueryH` 필드:55 + `NewAuditQueryHandler(pgStore, logger)`:216 + innerMux.Handle 2줄:277-278, ≈7줄 최소 단위) |
| `internal/audit/{audit.go, recorder.go}` | [EXISTING] **0 diff** — Action 23+ 상수·Recorder·EvalItemAuditNamespace UUIDv5 무변경 |
| `internal/auth/{abac.go, rbac.go, middleware.go}` | [EXISTING] **frozen, 0 diff [CRITICAL]** (RoleAuditor 신설 절대 금지, permissionMatrix 무변경) |
| `cmd/server/{score, report, review, rubric, evidence}_handlers.go` | [EXISTING] 패턴 미러 참조만(write-role 게이트 미차용) — 0 diff |
| `.moai/db/schema/migrations/0001~0006_*.sql` + `initial.sql` | [EXISTING] CTRL-001/SCORE-001/REVIEW-001/RUBRIC-001/EVID-001/EVAL-ITEM-001 생성 — 본 SPEC 마이그레이션 0 |
| `go.mod`/`go.sum` | [EXISTING] 7 SPEC 누적 의존만 사용 — 신규 외부 의존 0 |

## Exclusions (What NOT to Build)

1. DB 스키마·FK·신규 마이그레이션 변경 (read-only — CTRL-001 audit_logs + audit_logs_user_id_timestamp_idx 제공)
2. audit-of-audit-read (검색 요청 자체를 audit_logs에 기록) — 후속 SPEC SPEC-AX-AUDIT-READ-AUDIT-001 책임
3. mutation(write) 엔드포인트 — 7 SPEC store가 동일-TX 제공, write-role 게이트 미차용
4. 검색 결과 스냅샷 영속화 (on-the-fly만)
5. JSONB details 부분 검색 (`details->>'eval_item_id'` 등 hierarchy_code 역검색) — §6 OPEN #4에서 PoC 미적용 결정, 후속 SPEC
6. CSV/XLSX export endpoint — JSON only, `encoding/csv`/`excelize` 신규 외부 의존 0, 후속 SPEC SPEC-AX-AUDIT-EXPORT-001
7. 6번째 시간 제약(KST 업무시간) — AUTH-003/SCORE-API-001/REPORT-001 정합, 범위 밖
8. AUTH-003 모델 초과 풀 org-unit 속성 ABAC + RBAC permissionMatrix에 audit Permission 추가
9. **신규 RBAC 역할 신설 (RoleAuditor 등)** — frozen rbac.go [HARD] 위반, 핸들러-레벨 helper(`requireAuditQueryReadRole`)로 해소
10. 7 SPEC(CTRL/SCORE/SCORE-API/REVIEW/RUBRIC/EVID/EVAL-ITEM/AUTH/REPORT) 코드·스키마·FK·마이그레이션·Action 상수·Recorder 변경 (consumer-only [HARD])
11. Console UI / 클라이언트 SDK / OpenAPI 생성 (server-side 핸들러 + store SELECT만)

## §6 OPEN (plan.md §6 — Run Phase strategy.md §A + Human Gate sign-off 대상)

1. **OPEN #1 [CRITICAL] — admin-only narrowing 매핑 결정성**: `rbac.go:20-25` 3-role frozen + `permissionMatrix`에 audit/query Permission 부재. SCORE-API-001 §6 OPEN #4 패턴(write 역할 게이트가 frozen RBAC 우회 위해 핸들러-레벨 helper 사용) 답습할지(권장 (a)) vs `permissionMatrix` 확장 별도 SPEC 선행할지(REJECTED — frozen rbac.go [HARD]). strategy: (a)[권장] 핸들러-레벨 `requireAuditQueryReadRole` + `guardAuditQueryRead` (score_handlers.go:161-187 동형) — frozen rbac.go 0-diff 자연 성립 (b)(REJECTED) rbac.go 수정 (c)(REJECTED) abac.go 수정. T-201/T-205 RED-first 검증
2. **OPEN #2 — 필터 조합 정책 AND vs OR**: 5-필터 조합. strategy: (a)[권장] AND only (PoC 단순, score_handlers.go:283-333 list filter 정합) (b)(REJECTED) OR 지원 — query string 모호 (c)(REJECTED) GraphQL — 신규 외부 의존
3. **OPEN #3 — 응답 schema + 페이지네이션 default + endpoint URL + 빈/누락**: (1)[권장] `{"events":[...], "count":N, "total":M, "generated_at":"..."}` 전 필드 + total + 메타 (2)[권장] default=50/max=500 `clampPagination` 재사용 (3)[권장] `/api/v1/audit-logs` (audit_logs 테이블명 정합) (4)[권장] 빈 결과→200 (REPORT-001 §6.3 B-2 data-completeness)
4. **OPEN #4 — JSONB details 부분 검색 PoC 활성 vs 이연**: AUD-1 UUIDv5 surrogate(`recorder.go:295-305`)와 evaluation_items hierarchy_code 역검색 가능성. strategy: (a)[권장] PoC 미적용(이연) — 5-필터만, JSONB indexing(GIN) 추가는 §1.4 HARD 위반, 후속 SPEC (b)(REJECTED) PoC 활성 — GIN index 마이그레이션 필요
5. **OPEN #5 — store 메서드 위치 (WorkflowStore vs 신규 AuditQueryStore)**: strategy: (a)[권장] 기존 `WorkflowStore.QueryAuditLogs` 인터페이스 추가 — store.go [MODIFY] 1메서드, 단순 (b)(REJECTED) 신규 `AuditQueryStore` 인터페이스 — over-engineering (c)(REJECTED) stateless function — interface 회피, tx-aware 검증 어려움
6. **OPEN #6 — total count 산출 방식 (COUNT(*) OVER() vs 별도 쿼리)**: strategy: (a)[권장] `COUNT(*) OVER()` window function — 1 round-trip, 결과·total 동시 (b)(REJECTED) 별도 SELECT COUNT(*) — 2 round-trip, isolation 일치 어려움 (c)(REJECTED) total 미산출 — UX 손해

> issue_number 0 (gh unavailable). phantom 0 — 전 시그니처 source-verified (audit.go:17-128 / recorder.go:295-305 / pg_store.go:346-381 / abac.go:4-99 / rbac.go:20-33 / middleware.go:25-49 / score_handlers.go:43-190 / report_handlers.go:43-72 / server.go:55-220/277-285 / initial.sql:115-138 / go.mod). 특히 `audit_logs` 스키마·인덱스(initial.sql:115-138) + Action 23+ 상수(audit.go:17-100) + InsertAuditLog(pg_store.go:346-381) + ABAC/RBAC(abac.go:4-99/rbac.go:20-33) orchestrator ground-truth grep 확정 — 메모리 lesson #9 phantom-API 게이트 통과. SCORE-API-001 §6 OPEN #4 패턴 동위상(핸들러-레벨 helper로 frozen rbac 0-diff 자연 성립). REPORT-001 viewer 허용 정책과 정반대(감사 데이터 민감성으로 admin only — KEPCO E&C 한국 공공 감사 책임자 권한 정합). D1 iter2 lesson 적용 0(read-only — NewRecorder mode 무관, mutation 메서드 0이지만 §1.4에 명시 의무로 acknowledge). SCORE-API-001 errors.go drift lesson EXPLICIT 부착(§2.1+§2.3 양쪽 정확 2 sentinel 명시).
