# SPEC-AX-SCORE-API-001 Tasks (Phase 1.5 Decomposition)

> SPEC: SPEC-AX-SCORE-API-001 v0.1.0 (점수 조회/집계 HTTP API 계층)
> Methodology: TDD (RED-GREEN-REFACTOR), brownfield · Harness: thorough
> Mode: **sub-agent (manager-tdd 순차, team 아님)** — 신규 파일 2개 + server.go 1줄, 단일 도메인
> Source: spec.md §3 EARS / plan.md §2·§3·§4 S0~S6 / acceptance.md 27 AC·16 edge / strategy.md §A (§6 4건 RESOLVED)
> §6 RESOLVED: #1=POST /scores/{id}/supersede→201{score_id,superseded_id} · #2=max500/def50 · #3=offset/limit · #4=핸들러-로컬 write={RoleAdmin,RoleAnalyst} rbac.go/abac.go 0-diff

## §0. 결정 사항 (Human Gate sign-off 2026-05-19 — 구현 시 변경 금지)

- **OPEN#1**: `POST /api/v1/scores/{id}/supersede` body `{score_value,weight?,metadata?}`→201 `{score_id:"<new>",superseded_id:"<old>"}`. A(PUT+flag)/C(PATCH) 기각. ServeMux 최장일치 `/{id}` 무충돌.
- **OPEN#2**: `maxListLimit=500`, `defaultListLimit=50`. clamp: 누락/0→50, >500→500, offset 음수/비수치→0. workflow 1000 기각(store 미지원·메모리 슬라이싱·p99 NFR).
- **OPEN#3**: offset/limit. `GetScoresByEvaluationItem`(store.go:278) 전체→메모리 `[offset:offset+limit]` 슬라이싱. cursor 기각(post-PoC).
- **OPEN#4**: [HARD] (a) 핸들러-로컬. write={RoleAdmin,RoleAnalyst}, viewer=read-only. `auth.UserFromContext`+`auth.ParseRolesFromScope`, viewer-only mutation→403 `ErrCodeABACDenied`(abac.go:24 호출만). `evaluator` INFEASIBLE(rbac.go:33)→analyst 대체. **rbac.go/abac.go/authz_middleware.go/chain.go 0-diff** (OBS-001 metrics/permission.go 선례). org_unit 행-필터링 비적용(store.Score 컬럼 부재+SPEC §5 #5).

## §1. 파일 소유권 (sub-agent TDD — 단일 manager-tdd 순차) [HARD]

| 역할 | 대상 파일 | 비고 |
|------|-----------|------|
| manager-tdd (sub-agent, RED-GREEN-REFACTOR 순차) | `cmd/server/score_handlers.go` [NEW], `cmd/server/score_handlers_test.go` [NEW], `cmd/server/server.go` [MODIFY ≈7줄: 라우트 마운트 최소 단위] | team 미사용(단일 도메인·강결합·신규 파일). store fake 인터페이스 격리 |

[HARD] consumer-only: `internal/store|audit|auth|errors`·`cmd/server/evidence_handlers.go`·`.moai/db/schema/**` **0-diff**. TX 진입=`store.ScoreStore.BeginScoreTx`(pg_store.go:134)만(postgres.go 死스텁 비대상). 프롬프트 상대경로만.

## §2. Drift-Guard Manifest

| Delta | 파일 | 변경 내용 |
|-------|------|-----------|
| [NEW] | `cmd/server/score_handlers.go` | ScoreHandler+NewScoreHandler+Routes() 7라우트+7 핸들러+writeScoreJSON/writeScoreErr/scoreErrorBody+mapStoreErr/clampPagination/requireScoreWriteRole/decodeScoreBody/parseScoreID |
| [NEW] | `cmd/server/score_handlers_test.go` | httptest 단위(7×정상/에러, ABAC/404/409/400/clamp/empty/경계), fake ScoreStore/ScoreTx |
| [MODIFY] | `cmd/server/server.go` | `scoreH` 필드 + `NewScoreHandler` 생성자 + `innerMux.Handle` **2줄**(`/api/v1/scores` + `/api/v1/scores/` 서브트리 — Go1.22 ServeMux path-param 라우팅 구조적 필수) + ko 주석 (≈7줄, evidence_handlers server.go:53/207/261 선례 정확 미러). ABAC 와이어링(:261) **0-diff**. [GAN iter — Drift-Guard manifest 정밀화: 당초 "1줄" 과소기술, 실제 라우트 마운트 최소 단위. consumer-only 0-diff 불변 검증됨] |
| [EXISTING] | internal/store|errors, evidence_handlers.go, internal/auth/* (abac/authz_middleware/chain/rbac/middleware), 0004_score_tables.sql | 호출만 0-diff. permissionMatrix/Authorize frozen [HARD] |

## §3. Atomic Task Table (15 tasks, RED-first, 각 단일 TDD 사이클)

| Task | Description | Req | Deps | Files | Status |
|------|-------------|-----|------|-------|--------|
| T-001 | [S0][차단] hard-verify: grep BeginScoreTx pg_store.go(:134)+store.go(:255) / grep ErrGradeThresholdsUnavailable errors.go(:69). 미존재/불일치→STOP·재계획. 통과 후 회귀 baseline(evidence_handlers.go/workflow REST GREEN) + consumer-only git 해시 스냅샷 | 전제 | — | internal/store, internal/errors [EXISTING] | done |
| T-002 | [S1] ScoreHandler struct(store+logger, recorder 미주입)+NewScoreHandler+Routes()(Go1.22+ 7 패턴, ServeMux 최장일치 /rollup·/grade·/supersede>/{id}). RED(7 패턴+우선순위)→GREEN | 001/002, #1 | T-001 | score_handlers.go [NEW], _test [NEW] | done |
| T-003 | [S1] writeScoreJSON/writeScoreErr/scoreErrorBody(evidence_handlers.go:105-124, 한국어, INFO) + server.go 마운트 1줄(server.go:257 선례, :261 ABAC 자동). RED(스키마+server 회귀)→GREEN | 003 마운트, §2 MODIFY | T-002 | score_handlers.go, server.go [MODIFY], _test | done |
| T-004 | [S2] handleGetScore: UUID→GetScoreByID→200; ErrScoreNotFound→404; malformed UUID→400. RED(200/404/400)→GREEN | 001-E1,U1 | T-003 | score_handlers.go, _test | done |
| T-005 | [S2] handleListScores: evalitem blank→400; GetScoresByEvaluationItem→level/status 필터+offset/limit 메모리 슬라이싱; empty→{"scores":[],"count":0}→200. RED(filter+slice/empty/blank)→GREEN | 001-E2, #3 | T-004 | score_handlers.go, _test | done |
| T-006 | [S2] clampPagination + 상수 maxListLimit=500/defaultListLimit=50: 누락/0→50, >500→500, offset 음수→0. RED(3분기)→GREEN | 001-O1, #2 | T-005 | score_handlers.go, _test | done |
| T-007 | [S2] handleRollup(SumWeightedByEvaluationItem→pgtype.Numeric float64 미경유 십진→200) + handleGrade(DetermineGrade→200; ErrGradeThresholdsUnavailable→404 설계결정; scope blank/score 비수치→400). RED(정밀도 round-trip/404/400)→GREEN | 001-E3,E4,U1 | T-004 | score_handlers.go, _test | done |
| T-008 | [S3] handleCreateScore: pre-TX 검증(evalitem blank/>64, score_value 비수치/누락, evidence_id 비-UUID, JSON 실패→400 TX미진입); BeginScoreTx→InsertScore→Commit committed-defer Rollback→201{score_id,status:DRAFT}. decodeScoreBody/parseScoreID. RED(201/400 4종)→GREEN | 002-E1,U1 | T-003 | score_handlers.go, _test | done |
| T-009 | [S3] handleUpdateScore: ScoreUpdate; UpdateScore→200{score_id}; CONFIRMED→ErrScoreImmutable→409(한국어); malformed→400. RED(200/409/400)→GREEN | 002-E2, UBI-004 | T-008 | score_handlers.go, _test | done |
| T-010 | [S3] handleSupersedeScore(POST /{id}/supersede): CONFIRMED→SupersedeAndReplaceScore(동일 TX)→201{score_id<new>,superseded_id<old>}; non-CONFIRMED(DRAFT edge#14/SUPERSEDED edge#4)→ErrScoreNotConfirmed→409. RED(201/409×2)→GREEN | 002-E3, UBI-004, #1 | T-008 | score_handlers.go, _test | done |
| T-011 | [S4] mapStoreErr(errors.Is 전 센티넬 errors.go:54-78): NotFound→404, InvalidInput→400, Immutable/InvalidStatus/NotConfirmed→409, GradeThresholdsUnavailable→404, unknown→500. RED(7종 표+unknown)→GREEN | 004-S1 | T-007,T-009,T-010 | score_handlers.go, _test | done |
| T-012 | [S4] TX rollback 부분커밋 0: mutation downstream 실패→committed-defer Rollback, 매핑 status, goroutine 0. RED(fault inject+goleak.VerifyNone)→GREEN | 004-U1 | T-008,T-009,T-010 | score_handlers.go, _test | done |
| T-013 | [S5] requireScoreWriteRole({RoleAdmin,RoleAnalyst}만 true, OBS-001 IsMetricsAuthorized 동형)+mutation 진입 auth.UserFromContext+auth.ParseRolesFromScope→viewer-only→403 ErrCodeABACDenied(store 미호출); read 무게이트; auth-disabled(ok=false) 투과; admin 통과. RED(viewer write→403/viewer read→200/auth-disabled 투과/admin 우회)→GREEN. [HARD] auth/* 0-diff | 003-E1,S1,S2,U1, UBI-004, #4 | T-009,T-010 | score_handlers.go, _test | done |
| T-014 | [S5] 횡단 UBI: (001) 외부 host/TCP 0+SaaS/LLM SDK 미import 정적; (002) mutation 자체 audit_logs INSERT 0(store 위임)+GET audit 미발생; (003) auth-disabled cli-anonymous 비위조. RED→GREEN | UBI-001,002,003 | T-008,T-013 | score_handlers.go, _test | done |
| T-015 | [S6] BOUNDARY-1 + REFACTOR + 품질: (1) consumer-only 0-diff git diff(T-001 스냅샷 대비, server.go 라우트만); (2) 헬퍼 분리 확정(복잡도≥15 회피); (3) @MX(ScoreHandler/Routes ANCHOR+REASON, TX/mapStoreErr WARN+REASON, RED TODO→GREEN NOTE, ko); (4) ServeMux 충돌 테스트(edge#15); (5) 회귀 GREEN+coverage≥85%+golangci-lint default+gosec 0+goleak+TRUST 5+evaluator strict≥0.75 | 전체+BOUNDARY | T-005,T-006,T-007,T-011,T-012,T-013,T-014 | score_handlers.go, _test, server.go [MODIFY] | done |

총 **15 atomic 태스크** (SCORE-001 동일 SDD atomic 상한 내). 각 단일 RED-GREEN-REFACTOR 완결.

## §4. Sprint 매핑

| Sprint | 우선순위 | Tasks |
|--------|----------|-------|
| S0 게이트+baseline | High | T-001 |
| S1 골격+Routes+헬퍼+마운트 | High | T-002, T-003 |
| S2 조회+clamp | High | T-004, T-005, T-006, T-007 |
| S3 변경+TX | High | T-008, T-009, T-010 |
| S4 에러매핑+rollback | High | T-011, T-012 |
| S5 ABAC write-role+UBI | Medium | T-013, T-014 |
| S6 경계+REFACTOR+품질 | Medium | T-015 |

## §5. AC ↔ Task Coverage (27 AC 전부 ≥1)

UBI-001-1/2→T-014 · UBI-002-1/2→T-014 · UBI-003-1→T-013/T-014 · UBI-003-2→T-014 · UBI-004-1→T-013 · UBI-004-2→T-009 · UBI-004-3→T-010 · 001-1→T-004 · 001-2→T-004 · 001-3→T-005/T-006 · 001-4→T-007 · 001-5→T-007 · 001-6→T-005 · 001-7→T-006 · 002-1→T-008 · 002-2→T-009 · 002-3→T-010 · 002-4→T-008 · 002-5→T-008/T-009 · 003-1→T-013 · 003-2→T-013 · 003-3→T-013 · 004-1→T-011 · 004-2→T-012 · BOUNDARY-1→T-015. **coverage_verified=true: 27/27**. 전 E/S/O/U sub-clause ≥1 태스크.

## §6. 16 Edge 매핑

1 ABAC 미인가 write→T-013 · 2 미존재 GET→T-004 · 3 CONFIRMED PUT→T-009 · 4 supersede SUPERSEDED→T-010 · 5 입력검증→T-008 · 6 malformed UUID→T-008 · 7 malformed JSON→T-008 · 8 limit>max→T-006 · 9 limit 누락/0→T-006 · 10 offset 음수→T-006 · 11 authEnabled=false→T-013/T-014 · 12 empty list→T-005 · 13 grade unavailable→T-007 · 14 supersede DRAFT→T-010 · 15 ServeMux 충돌→T-002/T-015 · 16 consumer-only 0-diff→T-015 = **16/16**.

## §7. 리스크 (plan.md §7 R-API-001~010)

R-001(§6 미확정) — 해소(Human Gate 2026-05-19 RESOLVED). R-002(consumer-only 위반)—T-001 스냅샷+T-015 BOUNDARY-1. R-003(이중감사)—T-014. R-004(phantom evaluator)—OPEN#4 RESOLVED(evaluator 미사용)+T-001. R-005(numeric 정밀도)—T-007. R-006(센티넬 누락)—T-011. R-007(TX 부분커밋)—T-012. R-008(postgres.go 死스텁)—T-001/T-008 pg_store.go:134만. R-009(ServeMux 충돌)—T-002/T-015. R-010(over-eng)—sub-agent 핸들러 단위만, cursor/SDK/OpenAPI 미추가.

## §8. DoD (Phase 1.5)

- [ ] 15 atomic 태스크 각 단일 TDD, Status 전부 pending
- [ ] §0 §6 4건 RESOLVED 반영 (#1 POST /supersede / #2 500·50 / #3 offset / #4 핸들러-로컬 {Admin,Analyst} 0-diff)
- [ ] Drift-Guard §2 = spec.md §2/plan.md §2 일치
- [ ] 27 AC ≥1 태스크 (coverage_verified=true), 16 edge 매핑
- [ ] file-ownership = sub-agent 단일 manager-tdd (team 아님)
- [ ] [DELTA] 순서 (T-001 baseline → NEW/MODIFY)
- [ ] consumer-only [HARD] 0-diff, 신규 마이그레이션 0, 자체 audit 0
- [ ] phantom 0: BeginScoreTx→pg_store.go:134, evaluator 미사용(rbac.go:33 부재)
- [ ] S0 hard-verify(T-001) 선행 — 미통과 STOP
- [ ] tasks.md git 추적
