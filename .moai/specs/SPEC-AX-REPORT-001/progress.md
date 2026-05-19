# SPEC-AX-REPORT-001 Progress (TDD RED-GREEN-REFACTOR)

> Mode: sub-agent manager-tdd 순차 · Harness: thorough · 2026-05-19
> Branch: feature/SPEC-AX-SCORE-001-scoring (REPORT work uncommitted)

## T-001 [S0 BLOCKING] hard-verify gate — PASS

source-verified grep 결과 (ground-truth, phantom 0):
- `BeginEvalItemTx` store.go:118 / pg_store.go:118 (EvalItemStore)
- `GetEvalItemByID` store.go:199, `GetEvalItemsByParentID` store.go:202 (EvalItemTx, 빈 자식→빈 슬라이스 error 아님)
- `BeginScoreTx` store.go:255 / pg_store.go:134 (ScoreStore)
- `SumWeightedByEvaluationItem` store.go:295, `DetermineGrade` store.go:298 (ScoreTx)
- `ErrEvalItemNotFound` errors.go:33, `ErrGradeThresholdsUnavailable` errors.go:69, `ErrScoreInvalidInput` errors.go:58
- `shopspring` go.mod/go.sum (repo root) — 0건 (exit=1, 부재 확정)
- `pgtype.Numeric{Int *big.Int; Exp int32; NaN bool; InfinityModifier; Valid bool}` numeric.go:52-58 — exported 확정
- server.go 선례: scoreH 필드 server.go:55, NewScoreHandler(pgStore,logger) server.go:209, innerMux.Handle 263-264

Drift-Guard git snapshot (consumer-only baseline, HEAD=e7817f3):
- store.go b5f29e3 · score.go 94ccf0f · pg_store.go 17a1e8f · errors.go 5c59222
- score_handlers.go 1020db7 · evidence_handlers.go 80082db · server.go 22e781b
- go.mod b6ccc33 · go.sum a971dee

회귀 baseline: `go test ./apps/control-plane/cmd/server/` → ok (cached) GREEN
report_handlers*.go 부재 확인 (clean start, 2 NEW files)

## T-002 [S0 BLOCKING] RED-time 시드 가정 검증

§A.1(c) 잔여 가정: "범주 직계 자식 항목 id → fake ScoreTx SumWeightedByEvaluationItem non-zero" 경로.
핸들러 단위 테스트는 fake로 격리되므로 이 가정은 fake ScoreTx 주입으로 명시 검증되며
구조적으로 가능(SumWeightedByEvaluationItem(itemID) 시그니처 store.go:295 실재).
→ T-003 RED 첫 테스트에서 fakeEvalItemTx 자식 + fakeScoreTx 항목별 sum 경로 단언. STOP 불필요.

---

## T-003/T-008 [S1/S3] ReportHandler struct + Routes + accumulateWeighted

RED: `go test -run TestReportHandler` → build failed
  `undefined: ReportHandler / NewReportHandler / numericToRat` (구현 부재 — expected reason)
GREEN: report_handlers.go 생성 후
  TestReportHandler_SeedAssumption_ItemLevelAggregation PASS (§A.1(c) 시드 가정 충족)
  TestReportHandler_RoutesRegistersOneRoute PASS (단건 라우트, 목록 404)
  TestReportHandler_CrossStoreTwoDistinctStores PASS (EvalItemStore≠ScoreStore)
  TestAccumulateWeighted_PrecisionNoFloat64 PASS (0.1+0.2=0.3000 float64 미경유)

## T-004 [S1] 에러 헬퍼 + server.go 라우트 마운트

RED: TestReportHandler_ErrorBodySchema (헬퍼 부재 → 단언 실패)
GREEN: writeReportJSON/writeReportErr/reportErrorBody + server.go 6-line 마운트
  `go build ./apps/control-plane/...` OK · `go test ./.../cmd/server/` ok (회귀 GREEN 유지)
  server.go diff = 6 insertions (reportH 필드 + ko주석 + NewReportHandler(pgStore,pgStore,logger) + ko주석 + innerMux.Handle 2줄)

## T-005~T-014 [S2~S6] 검증/2-TX/B-2/매핑/rollback/ABAC/UBI/BOUNDARY

RED→GREEN 증거 (전 테스트 PASS, 회귀 GREEN):
  T-005 TestReportHandler_InputValidation400 (blank/64자→400, store 미진입) PASS
  T-006/007 TestReportHandler_CategoryReport200 (cross-store 2-TX, defer Rollback, Commit 없음) PASS
  T-009 EmptyReport_NoChildren / ZeroScoreItem / GradeNull_B2Graceful (200×3 graceful) PASS
  T-010 StoreErrorMapping (404/400/500) + WrappedSentinel (errors.Is) PASS
  T-011 EvalItemTxBeginFailure / ScoreTxBeginFailure / DownstreamFailure_BothTxRollback
        / ChildrenEnumerationError (각 defer Rollback, goleak.VerifyNone, race PASS) PASS
  T-012 ViewerReadAllowed / AuthDisabledPassthrough / AdminBypass (read-narrowing 200) PASS
  T-013 NoExternalDeps_NoAudit_NoWriteRole (정적: shopspring/audit/recorder/write-role 0) PASS
  T-014 ServeMuxPathParam (path-param 매칭, POST 405) + NumericToRat_ExpBranches PASS

## REFACTOR (T-014)

golangci-lint default+gosec 발견 2건 → 수정:
  1. report_handlers.go:257 errcheck → numericToRat의 always-nil error 반환 제거 (단순화)
  2. report_handlers_test.go:553 fieldalignment → 익명 struct 필드 재정렬
  gofmt -w (doc 주석 블록 tab-indent 정규화)
재검증: build OK / 전 테스트 GREEN / vet clean / gofmt -l empty / golangci-lint 0 issue

## 최종 품질 게이트 (T-014 BOUNDARY-1)

- consumer-only 0-diff: store.go/score.go/pg_store.go/errors.go/score_handlers.go/
  evidence_handlers.go/go.mod/go.sum git-hash = T-001 baseline 동일 (8/8 ZERO)
- .moai/db/schema/** 0-diff · internal/* 0-diff
- git diff --stat: server.go 6 insertions + report_handlers.go [NEW] + report_handlers_test.go [NEW] 만
- report_handlers.go statement coverage = 97.8% (89/91), 전 함수 ≥85% (≥85 target 충족)
- golangci-lint(default+gosec) 0 issue · go vet clean · gofmt -l empty
- goleak.VerifyNone 전 테스트 통과 · -race PASS
- @MX: ANCHOR×2(ReportHandler/Routes) WARN×3(mapReportStoreErr/numericToRat/handleCategoryReport)
  REASON×5 (모든 ANCHOR+WARN 커버) · code_comments:ko · [AUTO] prefix · 한도 내(3/5)
- 21 AC + 13 edge 전부 genuine RED-first 단언으로 커버
- pre-existing authz_e2e_test.go (AUTH-002/SERVER-001) 미접촉 — 범위 밖

STATUS: 14/14 tasks done. RED-GREEN-REFACTOR 완결.

---

## Phase 3 이중 게이트 pre-SYNC 표적 보강 (evaluator 90.8 / TRUST 5 PASS)

### D-1 [Minor] B-2 분기 negative-control 추가 (RED-first, 테스트만)

신규 테스트: `TestReportHandler_GradeError_NonUnavailable_NotAbsorbed`
  - B-2 graceful 분기가 `ErrGradeThresholdsUnavailable`에만 **좁게** 적용됨을 pin
  - non-Unavailable 일반 grade 에러 → handleCategoryReport default arm → 500 + 에러 본문
    (category_grade null 흡수 아님, category_total 부재 — 리포트 본문 아닌 에러 본문)
  - errors.Is 역검증: 주입 에러 ≠ Unavailable 센티넬

RED 입증 (genuine RED-first, post-hoc rubber-stamp 아님):
  - 임시 mutation(B-2 분기를 `case gradeErr != nil`로 확대) 적용 → negative-control **FAIL**
    (`Not equal`: 500 기대했으나 200으로 흡수) → 테스트가 좁은 분기를 실제 검증함 입증
  - mutation 즉시 복원 → GREEN 재확인 (B-2 graceful + negative-control 동반 PASS)
  - [HARD] 프로덕션 거동 이미 정확 — report_handlers.go 수정 0 (테스트만 추가, 거동 변경 0)

handleCategoryReport 커버리지: **94.1% → 100.0%** (default arm writeReportStoreErr 경로 커버)
report_handlers.go statement: **97.8% (89/91) → 100.0% (91/91)** — 전 함수 100%

검증: build OK · go vet clean · go test -count=1 -cover full cmd/server GREEN
  · gofmt -l report_handlers_test.go empty · race PASS · goleak PASS

### Info [pgx 버전 — ground-truth 정정·정밀화]

사용자 지시("실제 모듈 캐시 v5.7.2로 정정")는 ground-truth와 충돌하여 미수행:
  - go.mod 핀 = `github.com/jackc/pgx/v5 v5.9.2` (go.sum `v5.9.2/go.mod h1:...` 검증)
  - GOMODCACHE에 v5.7.2·v5.9.2 모두 잔류하나 **빌드 진실은 go.mod 핀 v5.9.2**
  - T-001 source-verified가 `pgx/v5@v5.9.2/pgtype/numeric.go`였음 (메모리 lesson #9 ground-truth 우선)
  - v5.7.2로 바꾸면 SPEC이 빌드 진실과 어긋남 → Push Back (Agent Core Behavior #3)
대신 혼동 해소 정밀화 (strategy.md §A.0 + tasks.md, cross-file 정합):
  - `numeric.go:52-57` → `:52-58` (실제 struct 범위 — Int/Exp/NaN/InfinityModifier/Valid)
  - "go.mod 핀 v5.9.2·go.sum 검증, GOMODCACHE v5.7.2 잔류분 무관, struct v5 전 구간 안정" 명시
  - 정정 위치: strategy.md L22(§A.0 표)·L79·L167, tasks.md L12(OPEN#2)·L38(T-001)
  - spec.md/plan.md: v5.9.2 표기 부재 — 대상 아님 (cross-file 정합 영향 없음)

### consumer-only 0-diff 재확인 (D-1 보강 후)

- 프로덕션 변경 0: report_handlers.go = untracked 신규 (mutation 복원 완료, 거동 0-diff)
- server.go = T-001 이후 6-line 라우트 마운트만 (D-1로 추가 변경 0)
- store/score/pg_store/errors/score_handlers/evidence_handlers/go.mod/go.sum git-hash = T-001 baseline 동일
- D-1 변경 = report_handlers_test.go [TEST만]·strategy.md·tasks.md·progress.md (SPEC 문서)
- pre-existing authz_e2e_test.go (AUTH-002/SERVER-001) 미접촉

STATUS: Phase 3 이중 게이트 표적 보강 완료. handleCategoryReport 100%, B-2 회귀 방어 pin.

