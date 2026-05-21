# strategy.md — SPEC-AX-REPORT-001 §A 결정 (Run Phase)

Version: 0.1.0
Generated: 2026-05-19
Companion: spec.md §6, plan.md §6, research.md(674줄 SSOT)
Status: RESOLVED (Human Gate sign-off 2026-05-19 완료 — 3건 전부 권고안 승인, §A.5 fallback 미발동)

## S0 Hard-Verify 게이트 결과 (source-verified 2026-05-19)

본 strategy의 모든 결정은 다음 ground-truth grep으로 독립 확정된 사실에 근거한다. **S0 게이트는 Run 진입 시에도 유지된다**(plan.md §4 S0):

| 소비 시그니처 | 위치(source-verified) | 인터페이스 |
|--------------|----------------------|-----------|
| `GetEvalItemByID(ctx, id string) (*EvalItem, error)` | `store.go:197-199` | `EvalItemTx` |
| `GetEvalItemsByParentID(ctx, parentID string) ([]*EvalItem, error)` | `store.go:200-202` (빈 자식 → 빈 슬라이스, error 아님) | `EvalItemTx` |
| `BeginEvalItemTx(ctx) (EvalItemTx, error)` | `store.go:115-118` / `pg_store.go:118` | `EvalItemStore` |
| `SumWeightedByEvaluationItem(ctx, evaluationItemID string) (pgtype.Numeric, error)` | `store.go:292-295` (raw-level 자식 행 Σ(score×weight), float64 미경유) | `ScoreTx` |
| `DetermineGrade(ctx, scope string, score float64) (string, error)` | `store.go:296-298` (scope 0건 → `ErrGradeThresholdsUnavailable`) | `ScoreTx` |
| `BeginScoreTx(ctx) (ScoreTx, error)` | `store.go:254-255` / `pg_store.go:134` | `ScoreStore` |
| `PgWorkflowStore` 두 인터페이스 동시 구현 | `pg_store.go:118`(EvalItem) + `pg_store.go:134`(Score), 동일 `s.pool` | — |
| `EvalItem` struct: `ID string`(VARCHAR64 계층코드), `ParentID *string`, `Level *int`("1범주 2항목 3지표 4배점, informational"), `Weight *float64` | `store.go:125-152` | — |
| `pgtype.Numeric{ Int *big.Int; Exp int32; NaN bool; InfinityModifier; Valid bool }` (exported 필드) | `pgx/v5@v5.9.2/pgtype/numeric.go:52-58` (go.mod 핀 = `github.com/jackc/pgx/v5 v5.9.2`, go.sum 검증; GOMODCACHE에 v5.7.2 잔류분 존재하나 빌드 진실은 go.mod 핀 v5.9.2 — Numeric struct는 v5 전 구간 안정, 비물질적) | — |
| `shopspring/decimal` **부재** | `go.mod`(monorepo 단일 모듈 `github.com/ircp/iroum-ax`, go 1.25.0, 16 direct deps) / `go.sum` 0건 grep | — |
| 핸들러 선례: `ScoreHandler`/`NewScoreHandler`/`Routes()`/`writeScoreJSON`/`writeScoreErr`/`scoreErrorBody`/`mapStoreErr`/handleRollup(`sum.Value()`→string, float64 미경유) | `score_handlers.go:37-138, 335-403` | — |

phantom API 0건 — 메모리 lesson #9 게이트 통과. research.md §14.4 `shopspring/decimal` 권장은 go.mod/go.sum 0건 ground-truth로 **superseded**(§A.2가 흡수).

---

## §A.1 — OPEN #1 [CRITICAL] RESOLVED: 범주 롤업 cross-store 2-TX 조합 (Option A)

### 권고 결정 [Human Gate 승인 2026-05-19]

**핸들러가 `EvalItemStore` + `ScoreStore` 두 store 의존을 보유하는 cross-store 2-TX read 조합(Option A). 신규 store 메서드 0.**

**(a) cross-store 조합 형태 [확정]**

- `ReportHandler struct { scoreStore store.ScoreStore; evalItemStore store.EvalItemStore; logger *zap.Logger }` — recorder 미주입(read-only, audit 0), write-role 필드/게이트 미차용(spec.md §1.5).
- `NewReportHandler(ss store.ScoreStore, eis store.EvalItemStore, logger *zap.Logger) *ReportHandler` — `server.go`에서 `s.reportH = NewReportHandler(pgStore, pgStore, logger)` (`PgWorkflowStore`가 두 인터페이스 동시 구현, `pg_store.go:118/134` source-verified, `server.go:209` `NewScoreHandler(pgStore,...)` 선례 정합).
- 범주 1건 산출 시퀀스 (2개 독립 read TX):
  1. **TX#1 (EvalItemTx)**: `eit, _ := h.evalItemStore.BeginEvalItemTx(ctx); defer eit.Rollback(ctx)` → `eit.GetEvalItemByID(ctx, categoryID)` (범주 검증; not-found 센티넬 → 404) → `eit.GetEvalItemsByParentID(ctx, categoryID)` (직계 자식 항목 열거; 빈 슬라이스 허용).
  2. **TX#2 (ScoreTx)**: `st, _ := h.scoreStore.BeginScoreTx(ctx); defer st.Rollback(ctx)` → 자식 항목별 `st.SumWeightedByEvaluationItem(ctx, child.ID)` 누적(§A.2 big.Rat) → `st.DetermineGrade(ctx, scope, total)` (범주 등급).
  - **Commit 없음** (read-only) — 각 TX `defer Rollback`만(`score_handlers.go:348/393` `defer func(){ _ = tx.Rollback(...) }()` 선례 정확 미러). 2개 TX 모두 함수 스코프 defer로 누수 0(REQ-REPORT-003-U1, goleak AC).

**(b) 계층 깊이 [확정: 1-level 직계 자식만]**

- `EvalItem.Level` 주석은 "1범주 2항목 3지표 4배점, **informational**"(store.go:134). PoC scope는 "안전보건" 범주 단건(spec.md §1.2), raw-level 점수만 사용(research §1.1).
- `SumWeightedByEvaluationItem`은 `evaluation_item_id`로 그 항목의 **raw-level 자식 행을 DB-side 집계**(store.go:292). 따라서 범주의 직계 자식(항목) id로 호출하면 그 항목의 raw 점수 가중합이 산출된다. 4-level 재귀 traversal(범주→항목→지표→배점)은 불필요하며 over-engineering이다(plan §7 R-RPT-010).
- **결정: `GetEvalItemsByParentID(categoryID)` 1-level 직계 자식만 열거. 재귀 미적용.** (research Appendix 알고리즘 정합.)

**(c) 블로커 조건 [해소 + 잔여 RED-time 검증]**

- 시그니처/식별자 블로커: S0 hard-verify로 실재 확정(grep 결과 위 표). 범주 식별은 path param `category id`를 그대로 `GetEvalItemByID`에 전달 — `Level`/`ParentID` 별도 discriminator 불요(not-found 센티넬이 미존재를 결정적 표면화). **블로커 해소.**
- **잔여 가정(RED-time 검증 대상)**: PoC 시드에서 raw 점수가 어느 레벨 `evaluation_item_id`에 적재되는지는 SCORE-001/EVAL-ITEM-001 시드 데이터 의존이다. "범주의 직계 자식 항목 id로 `SumWeightedByEvaluationItem`이 non-zero를 반환"한다는 가정이 시드와 어긋나면(예: raw 점수가 지표 레벨 id에만 존재) 1-level 가정이 깨진다. → **RED 첫 테스트에서 fake `ScoreTx`로 항목 레벨 집계 경로를 명시 검증**하고, 실데이터 계층 깊이 가정 위배 시 즉시 STOP·재계획(plan §4 S0 게이트 동위상). 핸들러 단위 테스트는 fake로 격리되므로 이 가정은 통합 책임(SCORE-001/EVAL-ITEM-001)으로 이연되며 본 SPEC 범위 밖 — 단 블로커 조건으로 명문화.

### 거부 대안

| 대안 | 거부 근거 |
|------|----------|
| **Option B — `SumWeightedByCategory` 신규 store 메서드** | consumer-only [HARD] 위반(spec.md §1.4/§5 #5/§3.4-U2). `internal/store/*` 0-diff 불변식 파괴. **REJECTED 확정.** |
| 단일 store(ScoreStore만)로 Option A | source 반증: `GetEvalItemsByParentID`/`GetEvalItemByID`는 `EvalItemTx`(store.go:197-202)에만 존재, `ScoreTx`에 부재. 단일 store로는 자식 열거 불가 → 구조적 불가. |
| 단일 TX에 두 store 작업 통합 | `EvalItemTx`와 `ScoreTx`는 별도 `BeginXxxTx` 진입점·별도 pgx.Tx. 단일 TX 통합은 store 인터페이스 변경 필요 → consumer-only 위반. 2개 독립 read TX가 유일 consumer-only 경로. |
| 4-level 재귀 traversal | PoC scope(안전보건 단건, raw-only) 초과. `SumWeightedByEvaluationItem`이 이미 DB-side 집계 제공하므로 핸들러 재귀는 중복·over-engineering. |

### consumer-only 영향

`internal/store|audit|auth|errors` 0-diff. 신규 store 메서드 0. `report_handlers.go` 신규 + `server.go` 라우트 마운트(≈7줄)만. AC-REPORT-BOUNDARY-1 만족.

---

## §A.2 — OPEN #2 RESOLVED: 정밀도 누적 = `math/big.Rat` 표준 라이브러리 (신규 외부 의존 0)

### 권고 결정 [Human Gate 승인 2026-05-19]

**`pgtype.Numeric`의 exported 필드 `{Int *big.Int, Exp int32}`를 `math/big.Rat`로 변환하여 무손실 누적. 신규 외부 의존 0 (math/big = stdlib).**

**누적 메커니즘 [확정]**

- 각 자식 항목의 `SumWeightedByEvaluationItem` → `pgtype.Numeric{Int, Exp, Valid}`. 십진값 = `Int × 10^Exp` (numeric.go:52-58 source-verified).
- `pgtype.Numeric` → `*big.Rat` 무손실 변환:
  - `Exp >= 0`: `rat = new(big.Rat).SetInt(new(big.Int).Mul(n.Int, 10^Exp))`
  - `Exp < 0`: `rat = new(big.Rat).SetFrac(n.Int, 10^|Exp|)` — 분모가 10의 거듭제곱인 정확 유리수, 십진 손실 0.
  - `Valid==false` → 0(빈 십진, score_handlers.go:362-367 "Valid=false면 nil → 빈 십진 안전 표면화" 선례 정합).
- accumulator `*big.Rat`에 `Add` 누적 — 유리수 누적은 오차 0(0.1+0.2=0.3 정확).
- 최종 직렬화: SCORE-001 집계가 `numeric(12,4)` 고정 scale(score.go SEC-03 주석)이므로 `acc.FloatString(4)` → 정확 십진 문자열. 입력이 모두 scale≤4 십진이므로 합도 scale≤4 내, 반올림 무발생. `weighted_sum`/`category_total` 응답 필드는 이 십진 문자열(float64 미경유 — REQ-REPORT-001-S2, score_handlers.go:370 `"weighted_sum":decStr` 선례).

**D3-2 정합 (DetermineGrade float64 입력)**

- 누적은 `big.Rat` 보존. 누적 완료 후 범주 총합을 `DetermineGrade(scope, total float64)`에 전달할 때만 `rat → Float64()` **1회** 변환. SEC-03 "float64 미경유" 범위는 N-항 누적 경로(big.Rat로 보존됨)이지 누적 완료 후 등급 임계 비교 1회 입력이 아니다(spec.md §6.2 D3-2 노트 정합). `DetermineGrade` 시그니처가 float64(store.go:298)이므로 store 계약을 따르는 것이 consumer-only 정합 — `score_handlers.go:382` handleGrade의 `strconv.ParseFloat` 동일 계약.

### 거부 대안

| 대안 | 거부 근거 |
|------|----------|
| `shopspring/decimal` 라이브러리 누적 (research §14.4 권장) | `go.mod`/`go.sum` 0건 부재(ground-truth grep). 신규 직접 의존 추가 = consumer-only minimalism·망분리(REQ-REPORT-UBI-001)·spec.md §1.4 HARD 위반. research §14.4 권장은 ground-truth로 **superseded**. **REJECTED 확정.** |
| `pgtype.Numeric` 자체 산술 API | numeric.go에 Add/Mul 부재(Scan/Value/Int64Value/Float64Value/ScanScientific만 — 코덱 타입, 산술 타입 아님). 자체 산술 불가 → Int/Exp 추출 후 math/big가 유일 stdlib 경로. |
| `Float64Value()` 후 float64 누적 | SEC-03 직접 위반(0.1+0.2≠0.3 오차 축적, score.go:453-457). **REJECTED.** |
| `Value()`(string) 누적 | 문자열은 산술 불가. 누적엔 부적합(handleRollup은 단건이라 누적 불요했음 — REPORT는 N개 누적). |
| `GetScoresByEvaluationItem` 전 행 fetch 후 Go-side Σ | store가 이미 DB-side `SumWeightedByEvaluationItem`(NULL-weight exclude 정책 store.go:294 포함) 제공. 재구현 = 비즈니스 로직 중복·정책 drift 위험·성능 저하. **REJECTED (열등).** |

### consumer-only 영향

`go.mod`/`go.sum` 0-diff(신규 외부 의존 0, REQ-REPORT-UBI-001 데이터 주권). `math/big`은 stdlib import만. AC-REPORT-BOUNDARY-1 만족.

---

## §A.3 — OPEN #3 RESOLVED: 단건 endpoint + 빈 리포트 + grade null (graceful B-2)

### 권고 결정 [Human Gate 승인 2026-05-19 — B-2 graceful 확정, B-1 fallback 미발동]

**(1) 응답 JSON 구조 [확정 — research §14.2 채택]**

```
{
  "category_id":    "AX-SAFETY-ORG",
  "category_name":  "안전보건",            // EvalItem.DisplayName
  "items": [
    { "item_id": "AX-SAFETY-ORG-01", "item_name": "...", "weighted_sum": "85.5000" }
  ],
  "category_total": "85.5000",            // big.Rat.FloatString(4), float64 미경유
  "category_grade": "A",                  // string | null
  "generated_at":   "2026-05-19T12:34:56Z"
}
```
- `weighted_sum`/`category_total` = 십진 문자열(SEC-03, score_handlers.go:370 선례). `category_grade` = string 또는 `null`.

**(2) 엔드포인트 [확정: 단건만]**

- `GET /api/v1/reports/category/{id}` 1개만. 목록 `GET /api/v1/reports`는 **PoC 미적용**(over-engineering 회피, plan §7 R-RPT-010, spec.md §5 #10).
- `server.go` `innerMux.Handle` **2줄**(`/api/v1/reports` + `/api/v1/reports/`) — `score_handlers.go` `server.go:263-264` 선례 정확 미러(Go1.22 ServeMux 최장일치, path-param 라우팅 구조적 필수). `Routes()`는 `GET /api/v1/reports/category/{id}` 1개 등록.

**(3) 페이지네이션/필터 [확정: PoC 미적용]**

- 단건 범주 리포트에 페이지네이션 불요(범주 1건당 자식 N개는 단일 응답 배열, 한국 공공 범주당 자식 소규모 — research §8.6 p99<100ms 목표 충족). `REQ-REPORT-001-O1`은 spec.md D3-1대로 **비활성 Optional 유지, O1 전용 AC 불요**. 향후 목록 endpoint 도입 시 `score_handlers.go:144` `clampPagination` 선례 재사용 — 미래 별도 SPEC.

**(4) 빈/누락 표면화 [확정 — 케이스 분리]**

| 상황 | source 근거 | 응답 |
|------|------------|------|
| 범주 not-found(`GetEvalItemByID` 센티넬) | store.go:198 `ErrEvalItemNotFound` 래핑 | **404** `{"error":{...}}` 한국어(score_handlers.go:74-101 선례) |
| blank/>64자/malformed category id | pre-store 검증(score_handlers.go validateCreateBody:409-424 선례) | **400** pre-store, TX 미진입 |
| 케이스 A: 자식 0(`GetEvalItemsByParentID` 빈 슬라이스) 또는 자식 점수 0 | store.go:201 빈 슬라이스(error 아님), SumWeighted 0 | **200** `{items:[], category_total:"0", category_grade:null}` (data-completeness, research §14.5) |
| 케이스 B: `grade_thresholds` scope 0건(`DetermineGrade`→`ErrGradeThresholdsUnavailable`) | score.go:444 fail-closed 센티넬 | **200** + `category_grade:null` (graceful degradation — **B-2 확정**) |

**케이스 B 정당화 (선례 404와의 긴장 명시적 해소 — Agent Core Behavior #2)**: `score_handlers.go:115-117` `mapStoreErr`는 `ErrGradeThresholdsUnavailable` → **404**(handleGrade D2-3: 단일 등급 조회는 등급이 자원 전부이므로 404가 정당). 그러나 REPORT는 **집계 리포트**로 등급은 부가 필드이지 자원 자체가 아니다 — 자식 점수는 정상 산출됐는데 등급 기준 1개 부재로 리포트 전체를 404로 죽이면 정보 손실이고 "범주 자체 부재(404)"와 클라이언트가 혼동한다. `null`은 등급을 fabricate하지 않으므로(거짓 등급 안 만듦) D2-3 "fabricate 금지"와 비충돌, research §14.5 data-completeness와 정합. **구현 방법(consumer-only 정합)**: REPORT 핸들러는 `DetermineGrade` 에러를 `mapStoreErr`에 넘기지 않고 핸들러 레벨에서 `errors.Is(err, apperrors.ErrGradeThresholdsUnavailable)`를 분기해 `grade=null`로 흡수, 그 외 store 에러만 `mapStoreErr` 미러로 4xx/5xx. → `score_handlers.go` `mapStoreErr` 코드 0-diff(미러는 그대로, 호출 분기만 REPORT 핸들러 자체 로직). REQ-REPORT-003-S1을 B-2로 확정. Human Gate가 B-2(graceful)를 승인했으므로 §A.5 B-1 fallback은 미발동.

### 거부 대안

| 대안 | 거부 근거 |
|------|----------|
| **B-1: `ErrGradeThresholdsUnavailable`→404 (선례 엄격 미러)** | REPORT는 집계 리포트로 등급이 부가 필드. 정상 산출된 점수를 등급 1개로 404 처리 = 정보 손실 + 범주 부재 404와 혼동. Human Gate가 B-2(graceful)를 선택하여 **미발동**. |
| 빈 범주 → 해당 범주 제외/404 | data-completeness(research §14.5) 위배. 자식 0은 정상 데이터 상태이지 에러 아님(store.go:201). |
| 목록 endpoint + 페이지네이션 PoC 포함 | PoC scope(안전보건 단건) 초과, over-engineering(plan §7 R-RPT-010, spec.md §5 #10). |
| `category_grade` 빈 문자열 `""` | `null`이 "미설정"을 명시적 표현(JSON 의미론). `""`는 등급 ""로 오인 가능. |

### consumer-only 영향

`score_handlers.go`(mapStoreErr 포함) 0-diff. `report_handlers.go` 신규 + `server.go` 라우트 마운트만. 자체 audit 0(read-only). AC-REPORT-BOUNDARY-1 만족.

---

## §A.4 결정 간 일관성 (RESOLVED 후 충족 기준)

- **#1 cross-store** ⟹ `ReportHandler` 2 store 의존, `EvalItemTx`+`ScoreTx` 각 read TX `defer Rollback`(Commit 없음), 신규 store 메서드 0 → consumer-only [HARD] 보존, `server.go` 라우트 마운트만.
- **#2 math/big.Rat** ⟹ `go.mod`/`go.sum` 0-diff(신규 외부 의존 0, REQ-REPORT-UBI-001).
- **#3 단건+빈리포트+grade null** ⟹ 자식0/점수0/grade미설정 모두 200 graceful, 범주부재만 404, 페이지네이션 미적용.
- 셋 다 불변식 보존: consumer-only 0-diff·신규 마이그레이션 0·신규 store 메서드 0·신규 외부 의존 0·자체 audit 0. phantom 0(자기인증 한정: `store.go:115-298`/`pg_store.go:118,134`/`score_handlers.go:37-403`/`numeric.go:52-58`/`go.mod`).

---

## §A.5 Human Gate 미서명 시 fallback (미발동 — B-2 graceful 승인됨)

- OPEN #3 케이스 B: Human Gate가 **B-1(404 엄격 선례 미러)** 선호 시 → `mapStoreErr` 미러를 `DetermineGrade` 에러에도 적용(404), `category_grade` 분기 제거. REQ-REPORT-003-S1을 404로 재확정, acceptance.md 해당 AC 1건 갱신. 나머지 #1/#2/#3(1)(2)(3) 결정은 불변. **→ 2026-05-19 Human Gate가 B-2 graceful을 승인하여 본 fallback은 미발동.**
