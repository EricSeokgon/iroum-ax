# SPEC-AX-REPORT-001 Tasks (Phase 1.5 Decomposition)

> SPEC: SPEC-AX-REPORT-001 v0.1.0 (평가 결과 리포트/집계 HTTP API 계층)
> Methodology: TDD (RED-GREEN-REFACTOR), brownfield · Harness: thorough
> Mode: **sub-agent (manager-tdd 순차, team 아님)** — 신규 파일 2개 + server.go 라우트 마운트(≈7줄), 단일 도메인 read-only
> Source: spec.md §3 EARS / plan.md §2·§3·§4 S0~S6 / acceptance.md 21 AC·13 edge / strategy.md §A (§6 3건 RESOLVED)
> §6 RESOLVED (Human Gate 2026-05-19): #1=cross-store 2-TX(EvalItemStore+ScoreStore, 1-level 직계 자식, Option A, 신규 store 메서드 0) · #2=`pgtype.Numeric{Int,Exp}`→`math/big.Rat` 누적·`FloatString(4)`(신규 외부 의존 0) · #3=단건 `GET /api/v1/reports/category/{id}`+빈 리포트+grade null(B-2 graceful, 페이지네이션 미적용, O1 비활성)

## §0. 결정 사항 (Human Gate sign-off 2026-05-19 — 구현 시 변경 금지)

- **OPEN#1 [CRITICAL]**: 범주 롤업 = cross-store 2-TX read 조합(Option A). `ReportHandler{scoreStore store.ScoreStore; evalItemStore store.EvalItemStore; logger *zap.Logger}`, `NewReportHandler(ss store.ScoreStore, eis store.EvalItemStore, logger)` → server.go `NewReportHandler(pgStore, pgStore, logger)`(`PgWorkflowStore` 두 인터페이스 동시 구현, pg_store.go:118/134 source-verified). 범주 1건: (i) `BeginEvalItemTx`→`GetEvalItemByID`(범주검증·not-found→404)+`GetEvalItemsByParentID`(**1-level 직계 자식만**, 재귀 미적용)→`defer Rollback`; (ii) `BeginScoreTx`→자식별 `SumWeightedByEvaluationItem` 누적+`DetermineGrade`→`defer Rollback`. **Commit 없음**(read-only). Option B(SumWeightedByCategory 신규 메서드)=consumer-only [HARD] 위반 기각. 잔여 가정(raw 점수 항목레벨 적재)=RED-time fake 검증, 시드 위배 시 STOP·재계획.
- **OPEN#2**: 정밀도 누적 = `pgtype.Numeric` exported `{Int *big.Int, Exp int32}`(pgx/v5@v5.9.2/pgtype/numeric.go:52-58 source-verified; go.mod 핀 v5.9.2·go.sum 검증, GOMODCACHE v5.7.2 잔류분 무관 — Numeric struct v5 전 구간 안정) → `*big.Rat` 무손실 변환(`Exp≥0`→`SetInt(Int×10^Exp)`, `Exp<0`→`SetFrac(Int,10^|Exp|)`, `Valid==false`→0) → `Add` 누적 → `acc.FloatString(4)` 십진 문자열(SCORE-001 numeric(12,4) scale). float64 미경유. **신규 외부 의존 0 (`math/big` stdlib)**. [D3-2] `DetermineGrade(scope, total float64)` 입력만 `rat.Float64()` 1회(store 계약, SEC-03 무충돌). shopspring/decimal·Float64Value·자체산술·전행Σ 기각.
- **OPEN#3 (B-2)**: 단건 `GET /api/v1/reports/category/{id}`만(목록 `/api/v1/reports` PoC 미적용). 응답 `{category_id, category_name, items:[{item_id, item_name, weighted_sum}], category_total, category_grade, generated_at}`(weighted_sum/total=십진 문자열, grade=string|null). 페이지네이션/필터 PoC 미적용, REQ-REPORT-001-O1 **비활성 Optional 유지·전용 AC 미추가**(AC 21/edge 13 불변). 빈/누락: 범주부재→404, blank/>64자→400 pre-store, 자식0/점수0→200 빈리포트(`items:[]`,total`"0"`,grade`null`), **grade_thresholds 미설정(`ErrGradeThresholdsUnavailable`)→200 + `category_grade:null`(B-2 graceful)**. 구현: 핸들러-로컬 `errors.Is(err, apperrors.ErrGradeThresholdsUnavailable)` 분기로 grade=null 흡수, 그 외 store 에러만 `mapReportStoreErr`(score_handlers.go:111-137 미러) → `score_handlers.go` mapStoreErr 코드 **0-diff**. B-1(404 엄격) 기각(§A.5 미발동).

## §1. 파일 소유권 (sub-agent TDD — 단일 manager-tdd 순차) [HARD]

| 역할 | 대상 파일 | 비고 |
|------|-----------|------|
| manager-tdd (sub-agent, RED-GREEN-REFACTOR 순차) | `cmd/server/report_handlers.go` [NEW], `cmd/server/report_handlers_test.go` [NEW], `cmd/server/server.go` [MODIFY ≈7줄: 라우트 마운트 최소 단위] | team 미사용(단일 도메인·read-only·신규 파일·강결합). store fake `ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx` 인터페이스 격리 |

[HARD] consumer-only: `internal/store|audit|auth|errors`·`cmd/server/score_handlers.go`·`cmd/server/evidence_handlers.go`·`.moai/db/schema/**`·`go.mod`/`go.sum` **0-diff**. TX 진입 = `store.ScoreStore.BeginScoreTx`(pg_store.go:134) + `store.EvalItemStore.BeginEvalItemTx`(pg_store.go:118)만(postgres.go 死스텁 비대상). 신규 마이그레이션 0·신규 store 메서드 0·신규 외부 의존 0·자체 audit 0(read-only, recorder 미주입). 프롬프트 상대경로만(`cmd/server/report_handlers.go`).

## §2. Drift-Guard Manifest

| Delta | 파일 | 변경 내용 |
|-------|------|-----------|
| [NEW] | `cmd/server/report_handlers.go` | `ReportHandler`(scoreStore+evalItemStore+logger, **recorder/write-role 미주입**)+`NewReportHandler(ss,eis,logger)`+`Routes()`(1 라우트 `GET /api/v1/reports/category/{id}`)+`handleCategoryReport`+범주 롤업 cross-store 2-TX 조합+`accumulateWeighted`(pgtype.Numeric→big.Rat)+`writeReportJSON`/`writeReportErr`/`reportErrorBody`+`mapReportStoreErr`(score_handlers.go:111-137 미러, `ErrGradeThresholdsUnavailable`는 핸들러-로컬 분기로 제외)+`parseCategoryID`(pre-store blank/>64자 검증) |
| [NEW] | `cmd/server/report_handlers_test.go` | httptest 단위(정상/404/400/empty/grade-null/numeric 정밀도/auth-disabled/viewer read/admin/cross-store 2-TX rollback/경계), fake `ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx`, t.Parallel, goleak |
| [MODIFY] | `cmd/server/server.go` | `reportH *ReportHandler` 필드(server.go:55 `scoreH` 선례 위치) + `s.reportH = NewReportHandler(pgStore, pgStore, logger)`(server.go:209 `NewScoreHandler(pgStore,...)` 선례 — pgStore가 ScoreStore+EvalItemStore 동시 구현 pg_store.go:118/134) + `innerMux.Handle` **2줄**(`/api/v1/reports` + `/api/v1/reports/` 서브트리 — Go1.22 ServeMux path-param 라우팅 구조적 필수, server.go:263-264 선례 정확 미러) + ko 주석 = **정확히 ≈7줄**. ABAC 와이어링(server.go:261 미들웨어 체인) **0-diff** — innerMux 전체 자동 적용. [SCORE-API-001 정밀도 교훈: "1줄" 과소기술 금지 — 라우트 마운트 최소 단위는 필드+생성자+Handle 2줄+주석 ≈7줄] |
| [EXISTING] | `internal/store/store.go`·`score.go`·`pg_store.go`(ScoreStore/ScoreTx/EvalItemStore/EvalItemTx/Score/EvalItem 호출만), `internal/errors`(센티넬 errors.Is), `cmd/server/score_handlers.go`(read-only 부분집합 패턴 미러 — 코드 무변경, write-role 게이트 미차용), `cmd/server/evidence_handlers.go`(패턴 2차 선례), `internal/auth/*`(abac/rbac/middleware 호출만 — 미들웨어 체인 자동), `.moai/db/schema/**`, `go.mod`/`go.sum` | 호출만 **0-diff**. permissionMatrix/Authorize/rbac.go frozen [HARD]. 신규 외부 의존 0(`shopspring/decimal` 부재 — go.mod/go.sum 0건) |

[HARD] 구현 중 [EXISTING] 1줄이라도 수정 발생 시 consumer-only 위반 → 즉시 중단·재계획(plan.md §7 R-RPT-002, AC-REPORT-BOUNDARY-1).

## §3. Atomic Task Table (14 tasks, RED-first, 각 단일 TDD 사이클)

| Task | Description | Req | Deps | Files | Status |
|------|-------------|-----|------|-------|--------|
| T-001 | [S0][차단·선행] hard-verify grep: `GetEvalItemsByParentID`(store.go:200-202)·`GetEvalItemByID`(store.go:197-199)·`BeginEvalItemTx`(store.go:115-118/pg_store.go:118) **AND** `SumWeightedByEvaluationItem`(store.go:292-295)·`DetermineGrade`(store.go:296-298)·`BeginScoreTx`(store.go:254-255/pg_store.go:134) **AND** `grep -rn shopspring go.mod go.sum`→**0건 부재 확인** **AND** `pgtype.Numeric` exported `Int *big.Int`/`Exp int32`(pgx/v5@v5.9.2/pgtype/numeric.go:52-58, go.mod 핀 v5.9.2·go.sum 검증 — GOMODCACHE v5.7.2 잔류분 무관). 미존재/시그니처 불일치/shopspring 존재 시 **STOP·재계획**(메모리 lesson #9 phantom 게이트, ground-truth 우선). 통과 후: 회귀 baseline(`score_handlers.go`·`evidence_handlers.go`·workflow REST 테스트 GREEN) + consumer-only 대상 파일 git 해시 스냅샷(Drift-Guard 기준선) | 전제 | — | internal/store, go.mod/go.sum [EXISTING] | done |
| T-002 | [S0][차단·RED-time 시드 가정] §A.1(c) 잔여 가정 검증: fake `ScoreTx`로 "범주 직계 자식 항목 id → `SumWeightedByEvaluationItem` non-zero" 경로를 RED 첫 테스트로 명시. 항목레벨 집계 가정이 PoC 시드와 어긋나면(예: raw 점수가 지표레벨에만) **STOP·재계획**(S0 게이트 동위상, strategy.md §A.1(c)) | 전제, 001-E1 | T-001 | report_handlers_test.go [NEW] | done |
| T-003 | [S1] `ReportHandler` struct(scoreStore+evalItemStore+logger, **recorder/write-role 미주입**)+`NewReportHandler(ss,eis,logger)`+`Routes()`(Go1.22 `GET /api/v1/reports/category/{id}` 1 라우트, ServeMux). RED(struct 2 store 의존+1 라우트 패턴)→GREEN | 001, §6.1 | T-002 | report_handlers.go [NEW], _test | done |
| T-004 | [S1] `writeReportJSON`/`writeReportErr`/`reportErrorBody{Error{Code,Message,Field}}`(score_handlers.go:74-101 미러, 한국어, INFO) + server.go 라우트 마운트 ≈7줄(reportH 필드 server.go:55 선례 + `NewReportHandler(pgStore,pgStore,logger)` server.go:209 선례 + innerMux.Handle 2줄 server.go:263-264 선례 + ko 주석). RED(에러 스키마+server 회귀 GREEN 유지)→GREEN | 003 마운트, §2 MODIFY | T-003 | report_handlers.go, server.go [MODIFY], _test | done |
| T-005 | [S2] `parseCategoryID` pre-store 검증: blank/>64자/malformed → 400 INVALID_ARGUMENT, store 미진입(`score_handlers.go` validateCreateBody:409-424 선례). RED(400 3분기, TX 미진입)→GREEN | 001-U1 | T-004 | report_handlers.go, _test | done |
| T-006 | [S2] EvalItemTx 단계: `BeginEvalItemTx`→`GetEvalItemByID`(범주 검증; not-found 센티넬→404)+`GetEvalItemsByParentID`(**1-level 직계 자식** 열거; 빈 슬라이스 허용)→`defer eit.Rollback(ctx)`(Commit 없음). RED(범주 200 경로/404/빈 자식)→GREEN | 001-E1,E2,U1, §6.1 | T-005 | report_handlers.go, _test | done |
| T-007 | [S2] ScoreTx 단계: `BeginScoreTx`→자식별 `SumWeightedByEvaluationItem`+`DetermineGrade`(범주 등급)→`defer st.Rollback(ctx)`(Commit 없음). 응답 직렬화 `{category_id,category_name,items:[{item_id,item_name,weighted_sum}],category_total,category_grade,generated_at}`. RED(2-TX 조합 정상 200)→GREEN | 001-E1, §6.3 | T-006 | report_handlers.go, _test | done |
| T-008 | [S3] `accumulateWeighted`: `pgtype.Numeric{Int,Exp,Valid}`→`*big.Rat`(`Exp≥0`→SetInt(Int×10^Exp), `Exp<0`→SetFrac(Int,10^|Exp|), `Valid==false`→0)→`Add` 누적→`acc.FloatString(4)` 십진 문자열. float64 미경유. `DetermineGrade` 입력만 `rat.Float64()` 1회. RED(0.1+0.2 정밀도 오차 0/십진 round-trip)→GREEN | 001-S2, §6.2 | T-007 | report_handlers.go, _test | done |
| T-009 | [S4] 빈/누락 표면화(B-2): 자식 0(`GetEvalItemsByParentID` 빈)→200 `{items:[],category_total:"0",category_grade:null}`; 자식 점수 0 item→해당 `weighted_sum:"0"`; `DetermineGrade`→`ErrGradeThresholdsUnavailable` 시 **핸들러-로컬 `errors.Is` 분기로 `category_grade:null`+200**(mapReportStoreErr에 미위임). RED(빈리포트/점수0/grade-null 200×3)→GREEN | 001-E2, 003-S1, §6.3 B-2 | T-008 | report_handlers.go, _test | done |
| T-010 | [S4] `mapReportStoreErr`(errors.Is, score_handlers.go:111-137 미러 — `ErrGradeThresholdsUnavailable`는 T-009 핸들러-로컬 분기로 제외): category not-found(EVAL-ITEM 센티넬)→404, invalid→400, unwrapped/unknown→500. `writeReportStoreErr`(500은 ERROR 로그). RED(센티넬→status 표+unknown 500)→GREEN | 003-S1 | T-006,T-009 | report_handlers.go, _test | done |
| T-011 | [S4] cross-store 2-TX rollback: `BeginEvalItemTx` 및/또는 `BeginScoreTx` 후 downstream 실패→각 read TX `defer Rollback`(Commit 없음), 부분 상태 0, goroutine 누출 0. RED(fault inject EvalItemTx/ScoreTx 각 경로 + goleak.VerifyNone)→GREEN | 003-U1 | T-006,T-007 | report_handlers.go, _test | done |
| T-012 | [S5] ABAC read-narrowing: viewer-only 인증 read→200(`abac.go:4` narrowing 추가 거부 없음) / authEnabled=false 전 엔드포인트 투과(`abac.go:8`, server.go:261 체인) / admin 우회(`abac.go:9`) / **write-role 게이트 코드 0건 정적 검증**(`requireScoreWriteRole`/`guardScoreWrite` 미차용 — score_handlers.go:161-187 비차용). RED(viewer read 200/auth-disabled 투과/admin/write-role 부재 정적)→GREEN. [HARD] auth/* 0-diff | 002-E1,S1,S2,U1, UBI-004 | T-007 | report_handlers.go, _test | done |
| T-013 | [S5] 횡단 UBI: (001) 외부 host/TCP 0 + SaaS/LLM SDK 미import + `shopspring/decimal` 미import 정적; (002) read-only이므로 `audit_logs` INSERT 0 + Recorder 의존 미주입 정적; (003) auth-disabled cli-anonymous 비위조(실 식별자 주입 0, created_by 생성 0). RED→GREEN | UBI-001,002,003 | T-008,T-012 | report_handlers.go, _test | done |
| T-014 | [S6] BOUNDARY-1 + REFACTOR + 품질: (1) consumer-only 0-diff `git diff`(T-001 스냅샷 대비 — internal/store|audit|auth|errors, score_handlers.go, evidence_handlers.go, .moai/db/schema/**, go.mod/go.sum 0, server.go 라우트 ≈7줄만); (2) 헬퍼 분리 확정(`enumerateCategoryChildren`/`accumulateWeighted`/`mapReportStoreErr`/`parseCategoryID` — 단일 함수 복잡도≥15 회피); (3) @MX(ReportHandler/Routes ANCHOR+REASON, cross-store 2-TX/accumulateWeighted/mapReportStoreErr WARN+REASON, RED `@MX:TODO`→GREEN `@MX:NOTE`, code_comments:ko); (4) ServeMux 충돌 테스트(`/api/v1/reports/category/{id}` 최장일치); (5) 회귀 GREEN + coverage≥85% + golangci-lint(default+gosec) 0 + goleak + TRUST 5 + evaluator-active strict ≥0.75 | 전체+BOUNDARY | T-005,T-008,T-009,T-010,T-011,T-012,T-013 | report_handlers.go, _test, server.go [MODIFY] | done |

총 **14 atomic 태스크** (SCORE-API-001 15 / SCORE-001 동일 SDD atomic 상한 내). 각 단일 RED-GREEN-REFACTOR 완결. mutation/write-role/supersede 태스크 0(read-only — SCORE-API-001 T-008~T-010/T-013 대응 의도적 부재).

## §4. Sprint 매핑

| Sprint | 우선순위 | Tasks |
|--------|----------|-------|
| S0 게이트+시드가정+baseline | High | T-001, T-002 |
| S1 골격+Routes+헬퍼+마운트 | High | T-003, T-004 |
| S2 입력검증+cross-store 2-TX 조합 | High | T-005, T-006, T-007 |
| S3 정밀도 누적 | High | T-008 |
| S4 빈/누락(B-2)+에러매핑+rollback | High | T-009, T-010, T-011 |
| S5 ABAC read-narrowing+UBI | Medium | T-012, T-013 |
| S6 경계+REFACTOR+품질 | Medium | T-014 |

## §5. AC ↔ Task Coverage (21 AC 전부 ≥1, coverage_verified=true)

UBI-001-1→T-013 · UBI-001-2→T-013 · UBI-002-1→T-013 · UBI-002-2→T-013 · UBI-003-1→T-012/T-013 · UBI-003-2→T-013 · UBI-004-1→T-012 · UBI-004-2→T-012(write-role 부재 정적) · 001-1→T-007 · 001-2→T-006 · 001-3→T-005 · 001-4→T-009 · 001-5→T-008 · 001-6→T-012 · 002-1→T-012 · 002-2→T-012/T-013 · 002-3→T-012 · 003-1→T-009/T-010 · 003-2→T-011 · 003-3→T-003 · BOUNDARY-1→T-014. **coverage_verified=true: 21/21**. 전 E/S/O/U sub-clause ≥1 태스크. REQ-REPORT-001-O1 = 비활성 Optional(D3-1, OPEN#3 RESOLVED B-2 페이지네이션 미적용) — 전용 AC 부재 정당, 태스크 미할당.

## §6. 13 Edge 매핑 (acceptance.md §7)

1 미존재 범주 id→T-006 · 2 공백/64자 초과 id→T-005 · 3 자식 0 범주→T-009 · 4 자식 점수 0→T-009 · 5 grade thresholds 미설정(grade null 200 B-2)→T-009 · 6 0.1+0.2 정밀도→T-008 · 7 authEnabled=false 투과→T-012/T-013 · 8 viewer-only read→T-012 · 9 RoleAdmin 우회→T-012 · 10 루트만 존재(하위 0)→T-009 · 11 cross-store 2-TX 실패→T-011 · 12 write-role 게이트 차용 시도(0건)→T-012 · 13 consumer-only 0-diff→T-014 = **13/13**.

## §7. 리스크 (plan.md §7 R-RPT-001~010)

R-RPT-001(§6 미확정)—해소(Human Gate 2026-05-19 RESOLVED, strategy.md §A). R-RPT-002(consumer-only 위반)—T-001 git 스냅샷+T-014 BOUNDARY-1. R-RPT-003(phantom GetEvalItemsByParentID)—해소(T-001 hard-verify, store.go:200-202 source-verified). R-RPT-004(cross-store 2-TX를 단일 store로 오구현)—T-001/T-003 2 store 의존 명시+T-006/T-007 EvalItemTx≠ScoreTx 분리 AC. R-RPT-005(신규 외부 의존 shopspring)—T-001 grep 부재 기준선+T-013 미import 정적+T-014 go.mod 0-diff. R-RPT-006(float64 정밀도 손실)—T-008 big.Rat·`@MX:WARN`. R-RPT-007(API 자체 audit INSERT)—T-013 recorder 미주입·audit SQL 0. R-RPT-008(write-role 게이트 차용→SCORE-API-001 §6#4 충돌)—T-012 write-role 코드 0건 정적. R-RPT-009(cross-store TX 누수/goroutine)—T-011 각 defer Rollback+goleak. R-RPT-010(over-eng: 목록/페이지네이션/스냅샷/SDK)—§0 OPEN#3 단건만, 핸들러 단위 테스트만(통합은 SCORE-001/EVAL-ITEM-001 커버). 추가: RED-time 시드 가정(R-RPT-004 파생)—T-002 fake 검증, 위배 시 STOP.

## §8. DoD (Phase 1.5)

- [ ] 14 atomic 태스크 각 단일 TDD, Status 전부 pending
- [ ] §0 §6 3건 RESOLVED 반영 (#1 cross-store 2-TX EvalItemStore+ScoreStore·1-level·신규 store 메서드 0 / #2 math/big.Rat·신규 외부 의존 0 / #3 단건 endpoint+빈리포트+grade null B-2·O1 비활성)
- [ ] Drift-Guard §2 = spec.md §2/plan.md §2 일치, server.go "≈7줄" 정확 기술(필드+생성자+Handle 2줄+주석, "1줄" 과소기술 금지 — SCORE-API-001 정밀도 교훈)
- [ ] 21 AC ≥1 태스크 (coverage_verified=true: 21/21), 13 edge 매핑, O1 비활성 전용 AC 부재 정당
- [ ] file-ownership = sub-agent 단일 manager-tdd (team 아님, read-only)
- [ ] [DELTA] 순서 (T-001/T-002 게이트·baseline → NEW/MODIFY)
- [ ] consumer-only [HARD] 0-diff (store/audit/auth/errors/score_handlers.go/evidence_handlers.go/migrations/go.mod/go.sum), 신규 마이그레이션 0, 신규 store 메서드 0, 신규 외부 의존 0, 자체 audit 0(read-only)
- [ ] phantom 0: GetEvalItemsByParentID→store.go:200-202, BeginEvalItemTx→pg_store.go:118, BeginScoreTx→pg_store.go:134, pgtype.Numeric{Int,Exp}→numeric.go:52-57, shopspring 부재(go.mod/go.sum 0건). 자기인증 한정(strategy.md §6.4 인용 범위)
- [ ] S0 hard-verify(T-001) + RED-time 시드 가정(T-002) 선행 — 미통과 STOP·재계획
- [ ] mutation/write-role/supersede 태스크 0 (read-only — SCORE-API-001 대비 의도적 부재)
- [ ] tasks.md git 추적
