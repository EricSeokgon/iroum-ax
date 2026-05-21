# SPEC-AX-REPORT-001 Implementation Plan

> Version: 0.1.0
> Methodology: TDD (RED-GREEN-REFACTOR), brownfield enhancement, sub-agent mode
> Harness: thorough
> Companion: `spec.md` (EARS), `acceptance.md` (Given/When/Then), `research.md` (Phase 0.5 근거, 674줄 file:line SSOT), `spec-compact.md` (압축)

본 plan은 SPEC-AX-SCORE-API-001 / SPEC-AX-SCORE-001 plan.md 구조를 미러링한다. **§6 3건 = OPEN (Run Phase 진입 시 strategy.md §A + Human Gate sign-off로 RESOLVED 대상)**. 본 SPEC은 SPEC-AX-SCORE-001/SCORE-API-001/EVAL-ITEM-001/AUTH-003의 **순수 consumer** — store/audit/auth/score-API/스키마 코드 0 diff, 신규 마이그레이션 0, 신규 외부 의존 0, 자체 audit 0.

---

## 1. 목표 & 범위

1차 산출물 = **SPEC-AX-SCORE-001 + SPEC-AX-EVAL-ITEM-001 store 메서드를 결합하여 범주별 집계 리포트를 노출하는 최소 read-only REST HTTP API 계층 + SPEC-AX-AUTH-003 ABAC read-narrowing 통합** (spec.md §1.1). 범주 롤업 조회 엔드포인트(GET, on-the-fly). mutation 엔드포인트 0. DB 스키마·FK·신규 마이그레이션·자체 audit·스냅샷 영속·신규 store 메서드·신규 외부 의존·풀 rubric·Console/SDK·6번째 시간제약은 범위 밖 (spec.md §5).

핵심 불변식 (research.md / source-verified):
- consumer-only [HARD]: `internal/store|audit|auth|errors`, `cmd/server/score_handlers.go`, `cmd/server/evidence_handlers.go`, `.moai/db/schema/**`, `go.mod`/`go.sum` **0 diff** (spec.md §1.4/§2.3 Drift-Guard)
- 신규 마이그레이션 0 (읽기 전용 API — 모든 테이블 SCORE-001/EVAL-ITEM-001이 생성)
- API 자체 audit 0 — read-only이므로 mutation 0 → audit 이벤트 0 (점수 audit는 SCORE-001 store가 생성 시점에 이미 기록, research.md §6.1/§6.3)
- 신규 store 메서드 0 — Option A(핸들러 cross-store 조합)만, `SumWeightedByCategory` 등 신규 메서드 추가 금지 (research.md §2.2 Option B = consumer-only 위반)
- 신규 외부 의존 0 — `shopspring/decimal` 부재(`go.mod`·`go.sum` 0건 orchestrator ground-truth 독립 확정 — source-verified 명제; go.mod 16개 직접 의존 보유, 목록 축소 열거 금지), 정밀도 누적은 표준 `math/big`/`pgtype`(§6 OPEN #2)
- TX 진입점 = `store.ScoreStore.BeginScoreTx`(→`pg_store.go:134`) + `store.EvalItemStore.BeginEvalItemTx`(→`pg_store.go:118`)만. `postgres.go` 死 스텁 비대상
- ABAC read-narrowing: 기존 미들웨어 체인(server.go:261)이 innerMux 전체 자동 적용 — report 라우트 자동 ABAC, server.go ABAC 와이어링 변경 0. **읽기 전용 → write-role 게이트 미차용**(score_handlers.go:161-187 비차용, §1.5)

---

## 2. 영향받는 파일 (Delta)

spec.md §2 표를 따른다. Delta 마커:

| 경로 | Delta | 비고 |
|------|-------|------|
| `cmd/server/report_handlers.go` | [NEW] | `ReportHandler`+`NewReportHandler`+`Routes()`+리포트 핸들러+범주 롤업 cross-store 조합+JSON/에러 헬퍼+에러 매핑 (`score_handlers.go:43-137` read-only 부분집합 미러, write-role 게이트 미차용) |
| `cmd/server/report_handlers_test.go` | [NEW] | `httptest` 핸들러 단위 테스트 (리포트 정상/에러, 404/400/empty/numeric/auth-disabled/viewer read/admin/cross-store rollback/경계) |
| `cmd/server/server.go` | [MODIFY] | **라우트 마운트만**(≈7줄 최소 단위): `reportH` 필드(`server.go:55` scoreH 선례 위치) + `s.reportH = NewReportHandler(pgStore, pgStore, logger)`(`server.go:209` 선례 — pgStore가 ScoreStore+EvalItemStore 동시 구현 pg_store.go:118/134) + `innerMux.Handle` **2줄**(`server.go:263-264` 선례: `/api/v1/reports` + `/api/v1/reports/` 서브트리 — Go1.22 ServeMux path-param 구조적 필수) + ko 주석. ABAC 와이어링 변경 **0-diff** |
| `internal/store/store.go` | [EXISTING] | `ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx`/`Score`/`EvalItem` 호출만 (store.go:115-298) — 0 diff |
| `internal/store/score.go` | [EXISTING] | `PgScoreTx` 메서드 호출만 (score.go:110-648) — 0 diff |
| `internal/store/pg_store.go` | [EXISTING] | `BeginScoreTx`(pg_store.go:134)+`BeginEvalItemTx`(pg_store.go:118) 호출만 — 0 diff |
| `internal/store/{eval_item 구현}` | [EXISTING] | `EvalItemTx` 구현 호출만 — 0 diff |
| `internal/errors/errors.go` | [EXISTING] | 에러 센티넬 `errors.Is` 매핑만 — 0 diff |
| `cmd/server/score_handlers.go` | [EXISTING] | read-only 부분집합 패턴 미러 참조만, write-role 게이트 미차용 — 0 diff |
| `cmd/server/evidence_handlers.go` | [EXISTING] | 핸들러 패턴 2차 선례 참조만 — 0 diff |
| `internal/auth/abac.go`/`rbac.go`/`middleware.go` | [EXISTING] | `ErrCodeABACDenied`/`ABACEvaluator`/`RoleViewer`/`RoleAdmin`/`UserFromContext` 호출만, 미들웨어 체인 자동 적용 — **permissionMatrix/Authorize frozen, 0 diff [HARD]** |
| `.moai/db/schema/migrations/*.sql` | [EXISTING] | SCORE-001/EVAL-ITEM-001 생성 테이블 — 본 SPEC 마이그레이션 추가/수정 0 |
| `go.mod`/`go.sum` | [EXISTING] | 신규 외부 의존 0 — `shopspring/decimal` 부재(`go.mod`·`go.sum` 0건 독립 확정), 기존 16개 직접 의존 무변경 |

[EXISTING] 특성화: report 라우트 마운트 추가 후 기존 `score_handlers.go`·`evidence_handlers.go`·workflow REST 핸들러 테스트가 GREEN 유지(회귀 0)임을 RED 진입 전 확인.

---

## 3. HTTP 계약 & 핸들러 구조 (research.md §3/§9/§Appendix, score_handlers.go read-only 선례)

> §6 3건 OPEN 상태. 아래는 research 권장 + source-verified 기본 골격이며, 정확 경로/응답형식/페이지네이션은 §6 OPEN #3, 조합 형태는 §6 OPEN #1, 누적 산술은 §6 OPEN #2 RESOLVED 후 확정.

### 3.1 엔드포인트 표 (research.md §14.2 — 잠정, §6 OPEN #3 확정 대상)

| Method | Path(잠정) | store 조합 | 권한 | HTTP status | 응답(잠정) |
|--------|-----------|-----------|------|-------------|-----------|
| GET | /api/v1/reports/category/{id} | (EvalItemTx) GetEvalItemByID+GetEvalItemsByParentID → (ScoreTx) ×N SumWeightedByEvaluationItem + DetermineGrade | read (viewer 포함 인증 사용자) | 200/404/400/500 | `{category_id,category_name,items:[{item_id,weighted_sum}],category_total,category_grade,generated_at}` |
| GET | /api/v1/reports | (반복) 전체 범주 목록 리포트 (페이지네이션 §6 OPEN #3) | read | 200/400/500 | `{reports:[...],generated_at}` |

> 경로/단건 vs 목록/페이지네이션 = §6 OPEN #3. mutation 엔드포인트 0 (read-only).

### 3.2 핸들러 구조 (score_handlers.go:43-137 read-only 부분집합 미러)

- `ReportHandler struct { scoreStore store.ScoreStore; evalItemStore store.EvalItemStore; logger *zap.Logger }` (recorder 불필요 — read-only, audit 0; **write-role 필드/게이트 미차용**, spec.md §1.5)
- `NewReportHandler(ss store.ScoreStore, eis store.EvalItemStore, logger *zap.Logger) *ReportHandler` — server.go에서 `NewReportHandler(pgStore, pgStore, logger)` (pgStore가 두 인터페이스 동시 구현, pg_store.go:118/134 source-verified)
- `Routes() http.Handler`: `mux := http.NewServeMux()`; Go 1.22+ method-prefixed GET 패턴 (정확 라우트 §6 OPEN #3) — `score_handlers.go:59-68` 미러, ServeMux 최장 일치
- 표준 헬퍼: `writeReportJSON(w, code, v)` / `writeReportErr(w, code, errCode, msg, field)` + `reportErrorBody{Error{Code,Message,Field}}` (`score_handlers.go:74-101` 동일 스키마, 한국어 메시지)
- 범주 롤업 조합(cross-store 2-TX, read-only — §6 OPEN #1): (i) `eit, _ := h.evalItemStore.BeginEvalItemTx(ctx); defer eit.Rollback(ctx)` → `GetEvalItemByID`(범주 검증)+`GetEvalItemsByParentID`(자식 열거); (ii) `st, _ := h.scoreStore.BeginScoreTx(ctx); defer st.Rollback(ctx)` → 자식별 `SumWeightedByEvaluationItem` 누적 + `DetermineGrade`(범주 등급). **Commit 없음(read-only) — defer Rollback만**(research.md §3.3 정합)
- 정밀도 누적(§6 OPEN #2): `pgtype.Numeric` → 표준 `math/big`/`pgtype` 산술 누적, float64 미경유 (신규 외부 의존 0)
- 에러 매핑(`errors.Is`, `score_handlers.go:111-137` `mapStoreErr` 미러): category not-found(EVAL-ITEM 센티넬)→404, `ErrGradeThresholdsUnavailable`→grade null/404(§6 OPEN #3), invalid→400, default→500
- 입력 검증 pre-store: category id blank/>64자, malformed → 400, store 미진입 (`score_handlers.go` 선례)
- ABAC: server.go 기존 미들웨어 체인이 자동 적용(read-narrowing). **write-role 게이트 미구현**(읽기 전용 — score_handlers.go:161-187 비차용, spec.md §1.5/§3.3-U1)

---

## 4. 구현 접근 (TDD Sprint, no time estimates)

> Sprint 우선순위 라벨: Priority High → Medium. Phase 순서: S0 완료 후 S1, 순차. brownfield — RED 작성 전 `score_handlers.go`(read-only 핸들러/에러/매핑 선례), `store.go`(ScoreTx+EvalItemTx 시그니처), `abac.go`(ABAC 동작) 정독(workflow-modes.md Brownfield Enhancement).

| Sprint | 우선순위 | 내용 | REQ |
|--------|----------|------|-----|
| S0 | High | **[hard-verify 게이트, 차단]** Run 진입 시 `grep -n 'GetEvalItemsByParentID\|GetEvalItemByID\|BeginEvalItemTx' apps/control-plane/internal/store/store.go apps/control-plane/internal/store/pg_store.go` (실재 기대: store.go:197-202·115-118, pg_store.go:118) 및 `grep -n 'SumWeightedByEvaluationItem\|DetermineGrade\|BeginScoreTx' .../store.go .../pg_store.go` (실재 기대: store.go:292-298·254-255, pg_store.go:134) hard-verify. **미존재/시그니처 불일치 시 consumer-only 전제 붕괴 → 즉시 STOP·재계획**(메모리 lesson #9 phantom 게이트, ground-truth 우선). `grep shopspring/decimal go.mod go.sum` → 부재 확인(신규 의존 금지 기준선). 통과 후: 회귀 baseline(`score_handlers.go`·`evidence_handlers.go`·workflow REST 테스트 GREEN) + §6 OPEN 3건 strategy RESOLVED 입력 준비 + consumer-only 대상 파일 git 해시 스냅샷(Drift-Guard 기준선) | (전제) |
| S1 | High | `ReportHandler` struct(scoreStore+evalItemStore+logger, **recorder/write-role 미주입**) + `NewReportHandler` + `Routes()` 라우트 골격 + `writeReportJSON`/`writeReportErr`/`reportErrorBody` 헬퍼 (score_handlers.go 미러). server.go 라우트 마운트 최소 단위(reportH 필드+생성자 `NewReportHandler(pgStore,pgStore,logger)`+innerMux.Handle 2줄, ≈7줄) | REQ-REPORT-001 |
| S2 | High | 범주 롤업 조합 핸들러: (i) EvalItemTx `GetEvalItemByID`(404)+`GetEvalItemsByParentID`(빈 자식→empty report) (ii) ScoreTx 자식별 `SumWeightedByEvaluationItem` 누적(정밀도 §6 OPEN #2 RESOLVED안 적용)+`DetermineGrade`(범주 등급, 미설정 처리 §6 OPEN #3) → 응답 직렬화 (형식 §6 OPEN #3 RESOLVED안 적용) | REQ-REPORT-001 |
| S3 | High | cross-store 2-TX read 오케스트레이션: EvalItemTx+ScoreTx 각각 `defer Rollback`(Commit 없음 — read-only), 실패 시 부분 상태 0 + goleak. 입력 검증 pre-store(blank/>64자→400). store 에러→HTTP 매핑 표(`errors.Is` 전 센티넬) | REQ-REPORT-003 |
| S4 | High | empty/누락 데이터: 자식 0 범주·점수 0 item → 빈/0 리포트(`items:[]`/`"0"`, NULL 금지), grade 미설정 처리(§6 OPEN #3 RESOLVED안). 정밀도(float64 미경유 누적, 정확 십진 직렬화 AC) | REQ-REPORT-001 |
| S5 | Medium | ABAC read-narrowing 검증 (viewer 인증 read→200 / authEnabled=false 투과 / admin 우회 / write-role 게이트 부재 정적 검증). 데이터 주권(외부 import 0, shopspring 부재) + 자체 audit 0(recorder 미주입·audit SQL 0) | REQ-REPORT-002, REQ-REPORT-UBI-001/002/004 |
| S6 | Medium | REFACTOR (조합 공통 추출: `enumerateCategoryChildren`/`accumulateWeighted`/`mapStoreErr`/`parseCategoryID` — 복잡도 ≥15 회피), @MX 태그, consumer-only 0-diff 검증(BOUNDARY-1: store/auth/schema/go.mod git diff 0), 커버리지 ≥85%, evaluator-active strict ≥0.75 | 전체 |

각 Sprint: RED(실패 테스트) → GREEN(최소 구현) → REFACTOR. store는 fake `ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx`로 격리(`score_handlers.go` fake 격리 선례) — 통합 테스트는 SCORE-001/EVAL-ITEM-001이 이미 커버하므로 본 SPEC은 핸들러 단위(cross-store 조합)에 집중(over-engineering 회피).

---

## 5. @MX 태그 계획 (code_comments: ko)

| 대상 | 태그 | 사유 |
|------|------|------|
| `report_handlers.go` `ReportHandler` | `@MX:ANCHOR` + `@MX:REASON` | 리포트 REST 진입점 — 핸들러 테스트 + 서버 마운트 + (미래) 통합 fan_in≥3 (score_handlers.go:43 선례) |
| `report_handlers.go` `Routes()` | `@MX:ANCHOR` + `@MX:REASON` | 리포트 라우트 단일 등록 계약 — server.go 마운트 invariant |
| `report_handlers.go` 범주 롤업 cross-store 조합 (EvalItemTx+ScoreTx 2-TX) | `@MX:WARN` + `@MX:REASON` | 2개 read TX defer Rollback 누락 시 커넥션 누수·goroutine 누출 위험; 단일 store 가정 시 오작동 (§6 OPEN #1, REQ-REPORT-003-U1) |
| `report_handlers.go` 정밀도 누적 (pgtype.Numeric→big 산술) | `@MX:WARN` + `@MX:REASON` | float64 경유 시 0.1+0.2≠0.3 오차 축적·신규 외부 의존 도입 위험 (REQ-REPORT-001-S2, §6 OPEN #2) |
| `report_handlers.go` `mapStoreErr` (에러→HTTP) | `@MX:WARN` + `@MX:REASON` | 센티넬 매핑 누락 시 404→500 오분류로 클라이언트 혼동 (REQ-REPORT-003-S1) |
| `report_handlers.go` 각 핸들러 메서드 | RED `@MX:TODO` → GREEN `@MX:NOTE` | TDD 진행 표시 (score_handlers.go 패턴) |

REFACTOR 시 단일 함수 복잡도 ≥15 회피 — `enumerateCategoryChildren`/`accumulateWeighted`/`mapStoreErr`/`parseCategoryID` 헬퍼 분리.

---

## 6. RESOLVED 결정 (Run Phase strategy.md §A + Human Gate sign-off 2026-05-19)

> §6 3건 **RESOLVED** (Human Gate sign-off 2026-05-19, 권고안 전부 승인 — OPEN#3=B-2, §A.5 fallback 미발동). 결정 SSOT = `strategy.md` §A.1~§A.5. consumer-only [HARD] 0-diff·신규 마이그레이션 0·신규 store 메서드 0·신규 외부 의존 0·자체 audit 0은 결정과 무관하게 불변. tasks.md §0이 본 결정을 atomic task로 분해한다.

### 6.1 OPEN #1 → RESOLVED [CRITICAL — 범주 롤업 cross-store 2-TX 조합 (Option A)]

spec.md §6.1 RESOLVED + strategy.md §A.1 참조. `ReportHandler{scoreStore,evalItemStore,logger}` 2 store 의존, `NewReportHandler(pgStore,pgStore,logger)`(pgStore 동시 구현 source-verified pg_store.go:118/134). 범주 1건 = 2개 독립 read TX: (i) `BeginEvalItemTx`→`GetEvalItemByID`(404)+`GetEvalItemsByParentID`(직계 자식 1-level)→`defer Rollback` (ii) `BeginScoreTx`→자식별 `SumWeightedByEvaluationItem` 누적+`DetermineGrade`→`defer Rollback`. Commit 없음(read-only). 계층 깊이 = **1-level 직계 자식만**(재귀 미적용). 잔여 가정(raw 점수 항목레벨 적재)은 RED-time fake 검증·시드 위배 시 STOP·재계획. **Option B(SumWeightedByCategory) REJECTED — consumer-only [HARD]**.

### 6.2 OPEN #2 → RESOLVED [정밀도 = `math/big.Rat` stdlib, 신규 외부 의존 0]

spec.md §6.2 RESOLVED + strategy.md §A.2 참조. `pgtype.Numeric{Int *big.Int, Exp int32}`(numeric.go:52-57 source-verified) → `*big.Rat` 무손실 변환·`Add` 누적·`FloatString(4)` 십진 직렬화(float64 미경유, score_handlers.go:370 선례). **신규 외부 의존 0 (math/big stdlib)**. [D3-2] 누적 big.Rat 보존, `DetermineGrade` 입력만 `Float64()` 1회(store 계약 정합, SEC-03 무충돌). `shopspring/decimal`(부재·research §14.4 superseded)/Float64Value/자체산술/전행 Σ 모두 REJECTED.

### 6.3 OPEN #3 → RESOLVED [단건 endpoint + 빈 리포트 + grade null (B-2)]

spec.md §6.3 RESOLVED + strategy.md §A.3 참조. (1) JSON: `{category_id,category_name,items:[{item_id,item_name,weighted_sum}],category_total,category_grade,generated_at}`, 십진 문자열·grade string|null. (2) **단건 `GET /api/v1/reports/category/{id}`만**(목록 미적용), server.go innerMux 2줄. (3) 페이지네이션 PoC 미적용·O1 비활성 유지(AC/edge 21/13 불변). (4) 범주부재→404, blank/>64자→400 pre-store, 자식0/점수0→200 빈리포트, **grade 미설정→200 grade:null (B-2 graceful, 핸들러-로컬 errors.Is 분기, mapStoreErr 0-diff)**. B-1(404 엄격) REJECTED(§A.5 미발동).

### 6.4 결정 간 일관성 (RESOLVED 충족 기준 — 유지)

- #1 = cross-store 2-TX ⟹ `ReportHandler` 2 store 의존, EvalItemTx+ScoreTx 각 read TX defer Rollback(Commit 없음), 신규 store 메서드 0 (consumer-only [HARD] 보존) — server.go 라우트 마운트만(≈7줄).
- #2 = `math/big.Rat` stdlib ⟹ go.mod/go.sum **0-diff** (신규 외부 의존 0, REQ-REPORT-UBI-001 데이터 주권).
- #3 = 단건+빈리포트+grade null(B-2) ⟹ 자식0/점수0/grade미설정 모두 200 graceful, 범주부재만 404, 페이지네이션 미적용·O1 비활성(count 21/13 불변).
- phantom 0. **자기인증 한정**: source-verified 인용 범위 = `store.go:115-298`/`score.go:110-648`/`pg_store.go:118,134`/`errors.go`/`score_handlers.go:43-137`/`abac.go:4-24`/`rbac.go:20-33`/`middleware.go:25-49`/`server.go:55,209,263-265`/`go.mod`/`pgx/v5@v5.9.2/pgtype/numeric.go:52-57`. research.md §14.4 `shopspring/decimal` 권장은 go.mod/go.sum 0건 ground-truth로 superseded(§6.2가 흡수).

---

## 7. 리스크 레지스터

| ID | 리스크 | 영향 | 완화 |
|----|--------|------|------|
| R-RPT-001 | §6 OPEN 3건 미확정 상태로 Run 진입 시 재작업 | Medium | spec.md/plan.md §6 명문화, Run 진입 strategy.md §A + Human Gate sign-off RESOLVED (SCORE-API-001 §6 흐름 동위상) |
| R-RPT-002 | consumer-only 위반 — store/auth/score-API/스키마/go.mod 1줄이라도 수정 | High | spec.md §1.4/§2.3 Drift-Guard manifest, S0 git 해시 스냅샷, BOUNDARY-1 0-diff 검증, 위반 시 즉시 중단·재설계 |
| R-RPT-003 | **phantom API — `GetEvalItemsByParentID` 등 EVAL-ITEM 메서드 가정** | High | **해소**: S0 hard-verify + orchestrator ground-truth grep으로 store.go:197-202/115-118·pg_store.go:118 실재 확정(메모리 lesson #9 게이트 통과). frontmatter Schema note phantom 0 명시 |
| R-RPT-004 | cross-store 2-TX 가정 오류 — 단일 store Option A로 잘못 구현 | High | spec.md §6.1 [CRITICAL] surface(Agent Core Behavior #2), `ReportHandler` 2 store 의존 명시, S0 게이트, EvalItemTx≠ScoreTx 분리 검증 AC |
| R-RPT-005 | 신규 외부 의존(`shopspring/decimal`) 도입으로 망분리·minimalism 위반 | High | §1.4 HARD, §6 OPEN #2 표준 `math/big`/`pgtype` 한정, S0 `grep shopspring` 부재 기준선, go.mod 0-diff AC (BOUNDARY-1) |
| R-RPT-006 | rollup 누적을 float64 변환해 정밀도 손실 | Medium | `@MX:WARN`, score.go:453-457 SEC-03 정합, 정확 십진 문자열 직렬화 AC, float64 미경유 검증 |
| R-RPT-007 | API 자체 audit INSERT로 부당 감사 (read-only인데 audit 발생) | High | REQ-REPORT-UBI-002 [HARD], `ReportHandler`에 recorder 의존 미주입, read-only mutation 0, audit SQL 0건 검증 AC |
| R-RPT-008 | write-role 게이트 차용으로 SCORE-API-001 §6 OPEN #4(`evaluator` 부재) 충돌 전파 | Medium | spec.md §1.5 명문 — 읽기 전용이므로 `score_handlers.go:161-187` 미차용, write-role 코드 부재 정적 검증 AC |
| R-RPT-009 | cross-store TX 누수/goroutine 누출 (EvalItemTx 또는 ScoreTx Rollback 누락) | High | 각 read TX `defer Rollback`(Commit 없음), goleak AC, `@MX:WARN` |
| R-RPT-010 | over-engineering — 스냅샷/SDK/페이지네이션 과대 추가 | Medium | spec.md §5 #4/#10, 핸들러 단위 테스트만(통합은 SCORE-001/EVAL-ITEM-001 커버), 최소 read-only surface, 페이지네이션은 §6 OPEN #3 PoC 필요 시에만 |

---

## 8. 완료 정의 (Plan 단계)

- [ ] §2 Delta 표 spec.md §2와 일치, [NEW]/[MODIFY]/[EXISTING] 명시 + Drift-Guard manifest
- [ ] §3 HTTP 계약 score_handlers.go read-only 선례 미러 (Routes/표준 에러/에러 매핑/cross-store 2-TX), store 시그니처 source-verified
- [ ] §4 TDD Sprint S0~S6, 우선순위 라벨(High/Medium), 시간 추정 0
- [ ] §5 @MX 계획 (ReportHandler/Routes ANCHOR, cross-store TX/정밀도/매핑 WARN+REASON, code_comments: ko)
- [ ] §6 3건 OPEN 명문화 (#1 [CRITICAL] cross-store 조합·child-enumeration phantom-risk 해소 / #2 정밀도 산술·신규 의존 금지 / #3 응답형식·페이지네이션·빈데이터) — Run Phase strategy.md §A + Human Gate sign-off RESOLVED 대상
- [ ] §7 리스크 레지스터 R-RPT-001~010 (consumer-only·phantom·cross-store·신규의존·부당감사 포함)
- [ ] consumer-only [HARD]: SCORE-001/SCORE-API-001/EVAL-ITEM-001/AUTH-003 코드·스키마·FK·마이그레이션·go.mod 0 diff, 신규 store 메서드 0, 자체 audit 0 보장 명시
- [ ] 구현 코드/테스트 미작성 (plan 문서만)
