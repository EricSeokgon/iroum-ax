# Research: SPEC-AX-REPORT-001 평가 결과 리포트/집계 HTTP API 계층 (Evaluation Result Report/Aggregation HTTP API Layer)

Phase: 0.5 Deep Research
Generated: 2026-05-19
Agent: Explore (read-only deep codebase analysis)
Status: complete

> SPEC-AX-REPORT-001 EARS 요구사항 설계 근거. 모든 주장 file:line 근거 포함.

---

## 1. Architecture Analysis — 평가 체계 전체 흐름

SPEC-AX-REPORT-001은 읽기 전용 HTTP API 계층으로 SPEC-AX-SCORE-001(store/audit) + SPEC-AX-SCORE-API-001(API 계층) + SPEC-AX-EVAL-ITEM-001(taxonomy) 위에 구축되는 최상위 집계·리포트 API다.

### 1.1 평가 계층 구조: 지표(indicator) → 항목(item) → 범주(category)

- **EVAL-ITEM-001 계층 모델** (`0003_eval_item_tables.sql:7-22`): `evaluation_items` 테이블은 자기참조 adjacency list(parent_id FK self-reference) — root는 parent_id NULL, 자식들은 parent_id로 부모 id 보유. id는 VARCHAR(64) 계층코드(예: AX-SAFETY-ORG-01).
- **평가 흐름** (`product.md:160`): 4계층 — 평가범주(category) → 평가항목(item) → 평가지표(indicator) → 배점/가중치. 지표별 raw 점수 입력 → weight 기반 item 롤업 → category 집계 → 등급(S/A/B/C/D) threshold mapping.
- **SCORE-001 level discriminator** (`score.go:30-34` + `0004_score_tables.sql:15,29`): scores 테이블의 `level` 컬럼은 'raw'|'item'|'category' discriminator 보유. raw-level = 지표별 개별 입력, item/category level 추가 저장 가능성 존재하나 PoC는 raw만 사용.

### 1.2 SCORE-API-001 기반 구조

- **SCORE-API-001 7 엔드포인트** (`score_handlers.go:59-70`): GET /scores/{id} / GET /scores?filter / GET /scores/rollup / GET /scores/grade / POST/PUT/supersede. REPORT-001은 읽기만 필요 (POST/PUT/supersede 범위 밖).
- **핸들러 선례**: `score_handlers.go` 미러링 패턴 — `ScoreHandler` struct + `Routes()` + 표준 JSON/에러 헬퍼(`evidence_handlers.go:82-124` 동일 스키마) + ABAC narrowing.
- **에러 매핑**: `score_handlers.go:111-129` `mapStoreErr` — store 센티넬 → HTTP status 결정적 매핑 (`errors.Is` 기반).

---

## 2. Category Rollup 기술 & SCORE-001 소비 계약

REPORT-001의 핵심은 **범주(category) 레벨 집계**다. 이는 item 또는 category level의 점수들을 조회·합산하여 범주별 가중 총합 및 등급을 산출하는 것이다.

### 2.1 SCORE-001 저장 API (read-only 경로)

REPORT-001이 소비 가능한 메서드:

| 메서드 | 시그니처 | 역할 | 파일 위치 |
|--------|---------|------|----------|
| `GetScoresByEvaluationItem` | `(ctx, evaluationItemID string) → []*Score` | 특정 evaluation_item_id의 모든 점수 행 반환 | `store.go:277-278` + `score.go:217-246` |
| `SumWeightedByEvaluationItem` | `(ctx, evaluationItemID string) → pgtype.Numeric` | 동일 item의 raw-level 행들의 가중합 Σ(score×weight) 반환. DB 사이드 집계, pgtype.Numeric (float64 미경유, 정확 십진) | `store.go:292-295` + `score.go:450-478` |
| `DetermineGrade` | `(ctx, scope string, score float64) → string` | grade_thresholds 테이블 lookup → 등급 문자(S/A/B/C/D) 반환. scope 행 0건 → ErrGradeThresholdsUnavailable (fail-closed) | `store.go:296-298` + `score.go:591-648` |
| `GetScoreByID` | `(ctx, id uuid.UUID) → *Score` | 단건 점수 조회 | `store.go:275-276` + `score.go:197-215` |

### 2.2 범주 롤업 전략 — OPEN 항목

**범주(category)의 정의**: EVAL-ITEM hierarchy에서 category-level의 항목은 parent가 root인 1차 항목 또는 명시적 level=1. category ID를 받으면 그 자식(2차 item들 또는 직속 자식)의 점수를 집계.

**집계 구성 옵션**:

| 옵션 | 전략 | API 핸들러 책임 | SCORE-001 메서드 조합 |
|-----|------|-----------------|----------------------|
| **Option A [권장]** | 1. category ID → EVAL-ITEM 자식 열거(GetEvalItemsByParentID) → 2. 각 자식별 SumWeightedByEvaluationItem 호출 → 3. 합산 | handler가 loop로 자식별 SumWeighted 조합 | GetEvalItemsByParentID(EVAL-ITEM) + SumWeightedByEvaluationItem(SCORE) ×N + DetermineGrade(SCORE) |
| **Option B [DB 최적화]** | category→item rollup을 DB 사이드에서 JOIN/GROUP BY 구성. 신규 메서드 추가 | 신규 `SumWeightedByCategory(categoryID) → pgtype.Numeric` 추가 필요 | [HARD] consumer-only 위반 — SCORE-001 신규 메서드 추가 불가 |

**현재 상태**: SCORE-001 `SumWeightedByEvaluationItem`은 **item-level**만 지원 (`score.go:460-478`; WHERE evaluation_item_id = $1). category-level 조회 메서드는 부재. **REPORT-001이 Option A(Option B 불가)를 선택하려면 EVAL-ITEM-001의 `GetEvalItemsByParentID` 메서드를 소비해야 함** (`store.go:200-202` 계약 존재).

### 2.3 EVAL-ITEM-001 자식 열거 메서드

| 메서드 | 시그니처 | 역할 | 파일 위치 |
|--------|---------|------|----------|
| `GetEvalItemsByParentID` | `(ctx, parentID string) → []*EvalItem` | 동일 parent_id를 가진 직계 자식 목록 반환 (empty → 빈 슬라이스, not error) | `store.go:200-202` + `eval_item.go` 구현 |
| `GetEvalItemByID` | `(ctx, id string) → *EvalItem` | 단건 항목 조회 (not found → ErrEvalItemNotFound) | `store.go:197-199` + `eval_item.go` 구현 |

**hierarchy 모델**: `EvalItem` struct (store.go:125-152) — `id VARCHAR(64)`, `parent_id *string`, `level *int`, `hierarchy_code`, `weight *float64`. category-level 항목의 ID를 알면 `GetEvalItemsByParentID(categoryID)` → 자식 item ID 목록 → 각 item마다 SCORE 집계.

---

## 3. SCORE-API-001 핸들러 선례 & REPORT-001 미러링 기술

REPORT-001은 SCORE-API-001의 핸들러 구조를 정확히 미러링한다.

### 3.1 ScoreHandler 패턴 검증 (`score_handlers.go:37-52`)

```go
type ScoreHandler struct {
    store  store.ScoreStore
    logger *zap.Logger
}

func NewScoreHandler(st store.ScoreStore, logger *zap.Logger) *ScoreHandler {
    return &ScoreHandler{store: st, logger: logger}
}

func (h *ScoreHandler) Routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /api/v1/scores/rollup", h.handleRollup)
    // ... 7 endpoints
    return mux
}
```

**REPORT-001 적용**: 동일 패턴 — `ReportHandler` struct + `NewReportHandler(...)` + `Routes()` 메서드 + 단일 진입점.

### 3.2 표준 에러 스키마 (evidence_handlers.go:89-124 선례)

```go
type scoreErrorBody struct {
    Error struct {
        Code    string `json:"code"`
        Message string `json:"message"`
        Field   string `json:"field,omitempty"`
    } `json:"error"`
}

func writeScoreJSON(w http.ResponseWriter, code int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(v) // 헤더 전송 후 로깅 불가
}

func (h *ScoreHandler) writeScoreErr(w http.ResponseWriter, code int, errCode, msg, field string) {
    var body scoreErrorBody
    body.Error.Code = errCode
    body.Error.Message = msg
    body.Error.Field = field
    h.logger.Info("점수 요청 거부", zap.Int("http_status", code), zap.String("error_code", errCode))
    writeScoreJSON(w, code, body)
}
```

**REPORT-001 적용**: 동일 `reportErrorBody` + `writeReportJSON` + `writeReportErr` — score_handlers.go 코드 100% 재사용 가능 (generic 상수만 변경).

### 3.3 TX Orchestration & Rollback 안전성 (score_handlers.go:428-474)

```go
func (h *ScoreHandler) handleCreateScore(w http.ResponseWriter, r *http.Request) {
    // ... validation
    tx, err := h.store.BeginScoreTx(r.Context())
    if err != nil {
        h.logger.Error("BeginScoreTx 실패", zap.Error(err))
        h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
        return
    }
    committed := false
    defer func() {
        if !committed {
            _ = tx.Rollback(r.Context())
        }
    }()
    
    // ... business logic
    if err = tx.Commit(r.Context()); err != nil {
        h.logger.Error("Commit 실패 — 롤백", zap.Error(err))
        h.writeScoreErr(w, http.StatusInternalServerError, "INTERNAL", "점수 저장 실패", "")
        return
    }
    committed = true
}
```

**REPORT-001 적용**: read-only 엔드포인트이므로 TX 불필요 (BeginScoreTx/Commit 불필요, Rollback only in defer for idempotency). SELECT-only path: `tx, _ := h.store.BeginScoreTx(...); defer tx.Rollback(...)`.

---

## 4. ABAC 권한 경계 & 읽기 전용 설계

### 4.1 SCORE-API-001 ABAC 통합 (score_handlers.go:159-190)

```go
// requireScoreWriteRole scope 문자열에서 추출한 역할이 write 권한인지 판단
func requireScoreWriteRole(scope string) bool {
    for _, r := range auth.ParseRolesFromScope(scope) {
        if r == auth.RoleAdmin || r == auth.RoleAnalyst {
            return true
        }
    }
    return false
}

// guardScoreWrite mutation 진입 직전 ABAC narrowing 게이트
func (h *ScoreHandler) guardScoreWrite(w http.ResponseWriter, r *http.Request) bool {
    u, ok := auth.UserFromContext(r.Context())
    if !ok {
        return true // auth-disabled 투과
    }
    if requireScoreWriteRole(strings.Join(u.Scopes, " ")) {
        return true
    }
    h.writeScoreErr(w, http.StatusForbidden, auth.ErrCodeABACDenied, "쓰기 권한이 없는 사용자입니다", "")
    return false
}
```

**파일 위치**:
- `abac.go:24` — `const ErrCodeABACDenied = "ABAC_CONDITION_DENIED"`
- `rbac.go:19-26` — `RoleAdmin/RoleAnalyst/RoleViewer` 정의 + `ParseRolesFromScope` (`roleRegex` 패턴: `^iroum-ax:(admin|analyst|viewer)$`, rbac.go:33)
- `middleware.go:44-52` — `UserFromContext(ctx) (*User, bool)`

### 4.2 REPORT-001 권한 정책: 읽기 전용이므로 viewer 포함 모든 authenticated user 허용

REPORT-001은 읽기 전용이므로 `viewer` 역할 포함 **모든 인증된 사용자**가 리포트를 조회할 수 있어야 한다 (SCORE-API-001의 GET 엔드포인트와 동일 정책). write 엔드포인트가 없으므로 `guardScoreWrite` 동일 메커니즘 불필요.

- **auth-disabled**: ABAC/RBAC 미들웨어 투과 (`authz_middleware.go` + `chain.go:17`), `cli-anonymous` 자동 기록 (SCORE-001 store 계층 처리)
- **RoleAdmin**: narrowing-only 우회 (`abac.go:71` REQ-ABAC-004)
- **RoleViewer/RoleAnalyst**: 읽기 허용 (write 엔드포인트 없음)

---

## 5. Server.go 마운트 패턴 (SCORE-API-001 선례)

### 5.1 현재 마운트 구조 (`server.go:53-55, 207-209, 261-264`)

```go
type Server struct {
    // ... 기타 필드
    evidenceH      *EvidenceHandler   // line 54
    scoreH         *ScoreHandler      // line 55 (신규)
    // ...
}

func New(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Server, error) {
    // ...
    s.evidenceH = NewEvidenceHandler(...)   // line 198-205
    s.scoreH = NewScoreHandler(pgStore, logger)  // line 209 (신규)
    return s, nil
}

func (s *Server) Run(ctx context.Context) error {
    // ...
    innerMux := http.NewServeMux()
    innerMux.Handle("/api/v1/evidences", s.evidenceH.Routes())  // line 262
    innerMux.Handle("/api/v1/scores", s.scoreH.Routes())        // line 263 (신규)
    innerMux.Handle("/api/v1/scores/", s.scoreH.Routes())       // line 264 (신규, Go1.22 path-param)
    innerMux.Handle("/", s.restHandler.Mux())
    // ...
}
```

### 5.2 REPORT-001 추가 패턴

동일 구조:

```go
// server.go 필드 추가
reportH        *ReportHandler     // 신규 필드

// server.go New() 함수
s.reportH = NewReportHandler(pgStore, logger)

// server.go Run() 함수, innerMux 마운트
innerMux.Handle("/api/v1/reports", s.reportH.Routes())
innerMux.Handle("/api/v1/reports/", s.reportH.Routes())
```

---

## 6. 읽기 전용 특성 & Audit 0 정책

REPORT-001은 **읽기 전용 HTTP API 계층**으로 다음이 HARD 계약이다:

### 6.1 자체 Audit 0건 (store 위임)

`score_handlers.go:8-9` 주석:
> 본 SPEC은 SPEC-AX-SCORE-001(store/audit)·EVID-001(핸들러 선례)·AUTH-003(ABAC)의 순수 consumer다. store/audit/auth/스키마 0-diff, 신규 마이그레이션 0, **자체 audit 0(store 계층 RecordScore* 동일 TX 전담)**.

- REPORT-001은 read-only이므로 mutation 이벤트(INSERT/UPDATE/DELETE) 발생 0
- audit은 SCORE 생성/수정 시 SCORE-001 `RecordScore*` 내부에서 동일 TX로 기록됨
- GET 엔드포인트는 read-only 조회이므로 audit_logs row 생성 불필요 (read 감시는 선택적, 기본 정책 absent)

### 6.2 SCORE-001 audit 메커니즘 (recorder.go 참조)

- `InsertScore` → `recorder.RecordScoreCreated(ctx, t, id, ...)`  (`score.go:130`)
- `UpdateScore` → `recorder.RecordScoreUpdated(ctx, t, id, ...)` (`score.go:307`)
- `SupersedeAndReplaceScore` → `recorder.RecordScoreCreated` + `recorder.RecordScoreUpdated` (`score.go:362, 386`)

모두 동일 TX(`t pgx.Tx`) 내 `InsertAuditLog` 호출 → 원자성 보장.

### 6.3 "읽기만" = 감사 생성 0

GET 엔드포인트는 mutation이 아니므로 audit 이벤트 필요 없음. REPORT-001 모든 엔드포인트가 read-only이면 audit_logs 신규 행 0.

---

## 7. 데이터 주권 & 외부 의존성 0

### 7.1 REQ-SCORE-API-UBI-001 (데이터 주권)

`score_handlers.go:7` + `spec.md §1.5`:
> 본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 / SPEC-AX-AUTH-003의 **순수 consumer**. store/audit/auth/스키마 0-diff, 신규 마이그레이션 0, **자체 audit 0**.

REPORT-001도 동일 원칙:

- **데이터 호출처**: PostgreSQL pgx pool만 (store.BeginScoreTx/GetScoresByEvaluationItem/SumWeightedByEvaluationItem/DetermineGrade)
- **외부 API 호출 0**: LLM API, 외부 SaaS, CDN, secrets manager 등 없음
- **신규 import 0**: 외부 의존성 도입 금지

### 7.2 CLI 익명성 (auth-disabled Walking Skeleton)

`score_handlers.go:38, 48-51`:
> recorder 의존 미주입 — 감사는 store 계층(RecordScore*)이 동일 TX로 전담 (UBI-002-2).

SCORE-001의 `created_by DEFAULT 'cli-anonymous'` (`0004_score_tables.sql:22`) 정책 → auth-disabled 시 모든 행이 'cli-anonymous' 자동 기록. REPORT-001도 동일 store 계층 정책 준수.

---

## 8. 에러 매핑 & HTTP 상태 (SCORE-API-001 선례)

### 8.1 Store 센티넬 매핑 (`score_handlers.go:111-129`)

```go
func mapStoreErr(err error) (int, string, string) {
    switch {
    case errors.Is(err, apperrors.ErrScoreNotFound):
        return http.StatusNotFound, "NOT_FOUND", "요청한 점수를 찾을 수 없습니다"
    case errors.Is(err, apperrors.ErrGradeThresholdsUnavailable):
        return http.StatusNotFound, "NOT_FOUND", "요청한 scope의 등급 기준이 설정되지 않았습니다"
    case errors.Is(err, apperrors.ErrScoreInvalidInput):
        return http.StatusBadRequest, "INVALID_ARGUMENT", "점수 입력이 유효하지 않습니다"
    case errors.Is(err, apperrors.ErrScoreImmutable):
        return http.StatusConflict, "CONFLICT", "CONFIRMED 점수는 정정(supersede)으로만 수정 가능합니다"
    case errors.Is(err, apperrors.ErrScoreInvalidStatus):
        return http.StatusConflict, "CONFLICT", "허용되지 않은 점수 상태 전이입니다"
    case errors.Is(err, apperrors.ErrScoreNotConfirmed):
        return http.StatusConflict, "CONFLICT", "정정(supersede)은 CONFIRMED 점수에만 허용됩니다"
    default:
        return http.StatusInternalServerError, "INTERNAL", "점수 처리 중 오류가 발생했습니다"
    }
}
```

**적용 대상**: GetScoresByEvaluationItem, SumWeightedByEvaluationItem, DetermineGrade 결과 처리 시 동일 매핑.

### 8.2 입력 검증 (pre-TX)

- 빈/초과 evaluation_item_id → 400 INVALID_ARGUMENT
- 비수치 점수값 → 400 INVALID_ARGUMENT
- malformed JSON → 400 INVALID_ARGUMENT
- non-existent category/item → 404 NOT_FOUND (GetEvalItemByID 또는 SumWeightedByEvaluationItem에서 결과 empty 처리)

---

## 9. 응답 형식 & JSON 스키마

### 9.1 단건 점수 조회 (score_handlers.go:204-216)

```go
type scoreResponse struct {
    Metadata         map[string]any `json:"metadata,omitempty"`
    ScoreValue       *float64       `json:"score_value,omitempty"`
    Weight           *float64       `json:"weight,omitempty"`
    EvidenceID       *string        `json:"evidence_id,omitempty"`
    ID               string         `json:"id"`
    EvaluationItemID string         `json:"evaluation_item_id"`
    Level            string         `json:"level"`
    Grade            string         `json:"grade,omitempty"`
    Status           string         `json:"status"`
    CreatedBy        string         `json:"created_by,omitempty"`
}
```

### 9.2 LIST 응답 (score_handlers.go:332)

```go
writeScoreJSON(w, http.StatusOK, map[string]any{
    "scores": page,  // []scoreResponse
    "count": len(page)
})
```

### 9.3 ROLLUP 응답 (score_handlers.go:368-371)

```go
writeScoreJSON(w, http.StatusOK, map[string]any{
    "evaluation_item_id": evalItem,
    "weighted_sum": decStr  // pgtype.Numeric → text-format 십진 문자열 (float64 미경유)
})
```

**REPORT-001 추가 응답**:

- **Category Rollup Summary**: `{"category_id": "...", "category_name": "...", "total_weighted_sum": "...", "grade": "...", "items": [...]}`
- **Report Data**: 범주별 요약 테이블 (JSON array of categories)

---

## 10. Numeric 정밀도 & SEC-03 (SCORE-001 설계 결정)

### 10.1 Float64 회피 (score_handlers.go:335-372)

```go
// SEC-03: 집계 경로에 float64 사용 금지 — pgtype.Numeric로 정확 십진 반환
// DECIMAL(6,2)/DECIMAL(5,4)를 Go float64로 읽으면 0.1+0.2≠0.3 식 부동소수점 오차 축적
// DB SUM을 numeric(12,4) 캐스트 후 pgtype.Numeric로 스캔하여 float64 변환 없이 정확 십진 텍스트로 비교 가능

sum, err := tx.SumWeightedByEvaluationItem(r.Context(), evalItem)
// pgtype.Numeric.Value()는 text-format 십진 문자열 반환
decimal, valErr := sum.Value()
if valErr != nil { ... }
decStr := ""
if s, isStr := decimal.(string); isStr {
    decStr = s
}
writeScoreJSON(w, http.StatusOK, map[string]any{
    "evaluation_item_id": evalItem,
    "weighted_sum": decStr  // float64 미경유, 정확 문자열
})
```

**REPORT-001 적용**: category rollup 합산도 `SumWeightedByEvaluationItem` 결과 `pgtype.Numeric` 누적 후 text-format 직렬화 (각 item마다 Numeric string 파싱 후 decimal 누적은 미지원 → Option B 불가 근거).

---

## 11. 범주별 등급 산출

### 11.1 DetermineGrade 호출 (score_handlers.go:374-403)

```go
func (h *ScoreHandler) handleGrade(w http.ResponseWriter, r *http.Request) {
    scope := strings.TrimSpace(q.Get("scope"))
    score, perr := strconv.ParseFloat(q.Get("score"), 64)
    if perr != nil { ... }
    
    grade, err := tx.DetermineGrade(r.Context(), scope, score)
    if err != nil {
        h.writeStoreErr(w, err)
        return
    }
    writeScoreJSON(w, http.StatusOK, map[string]any{
        "scope": scope,
        "score": score,
        "grade": grade
    })
}
```

### 11.2 Grade Threshold 모델 (score.go:591-648)

```go
// DetermineGrade score에 대한 등급 문자를 grade_thresholds 테이블로부터 결정적으로 산출한다.
// D3: scope 파라미터로 해당 scope 행 전체 조회 → S→D 내림차순 min_value 스캔,
// boundary_rule gte(≥)/gt(>) 기준 최초 일치 등급 반환.
// scope 행 0건 → ErrGradeThresholdsUnavailable (등급 fabricate 금지, fail-closed).

const query = `
    SELECT letter, min_value, boundary_rule
    FROM grade_thresholds
    WHERE scope = $1
    ORDER BY min_value DESC
`
rows, err := t.tx.Query(ctx, query, scope)
// ... scan thresholds
if len(thresholds) == 0 {
    return "", fmt.Errorf("DetermineGrade scope=%q: %w", scope, stderrors.ErrGradeThresholdsUnavailable)
}

// S→D 내림차순 스캔: 첫 일치 등급 반환
for _, gt := range thresholds {
    var matches bool
    switch gt.BoundaryRule {
    case "gte":
        matches = score >= gt.MinValue
    case "gt":
        matches = score > gt.MinValue
    }
    if matches {
        return gt.Letter, nil
    }
}

return "", fmt.Errorf("DetermineGrade scope=%q score=%.2f: 모든 임계값 미달 — %w",
    scope, score, stderrors.ErrGradeThresholdsUnavailable)
```

**REPORT-001 적용**: category rollup 합계를 점수로 `DetermineGrade(scope, totalScore)` 호출 → 범주별 등급 자동 산출.

---

## 12. Consumer-Only 무변경 계약 (HARD)

SCORE-API-001 spec.md §1.4 정의:

```
본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 / SPEC-AX-AUTH-003의 **순수 consumer**이다.
다음이 **HARD 계약**이다:

- [HARD] 본 SPEC은 `internal/store/score.go`·`store.go`(`ScoreStore`/`ScoreTx`/`Score`/`ScoreUpdate`),
  `internal/audit/*`(`RecordScore*`), `internal/auth/*`(ABAC/RBAC), `.moai/db/schema/migrations/*.sql`을
  **일절 수정하지 않는다**. 점수 비즈니스 로직·감사·접근제어·DB 스키마는 호출만 한다.
```

**REPORT-001 적용 범위 동일**:

- `internal/store/store.go` — ScoreTx 인터페이스 메서드만 호출
- `internal/store/score.go` — PgScoreTx 구현 호출 (수정 0)
- `internal/store/eval_item.go` — EvalItemTx 인터페이스 메서드만 호출 (GetEvalItemsByParentID)
- `internal/audit/*.go` — audit 기록 위임만 (REPORT 자체 audit 0)
- `internal/auth/*.go` — ABAC/RBAC narrowing 호출만 (정책 수정 0)
- `.moai/db/schema/**` — 신규 마이그레이션 0 (읽기 전용 API이므로 스키마 변경 불필요)
- `cmd/server/evidence_handlers.go` — 핸들러 패턴 선례만 (파일 수정 0)

---

## 13. 한국 공공 6제약 & REQ-UBI 패턴

### 13.1 Six Constraints 준수

| 제약 | REPORT-001 적용 | 근거 |
|-----|-----------------|------|
| **데이터 주권** | PostgreSQL pgx pool 단일 호출, 외부 API 0 | REQ-SCORE-API-UBI-001 |
| **한국어** | 모든 에러 메시지 한국어 (evidence_handlers.go:209-215 선례) | spec 설계 결정 |
| **감사 가능성** | read-only이므로 자체 audit 0, store 위임 | REQ-SCORE-API-UBI-002 |
| **망분리** | 내부 PostgreSQL만, 외부 호출 0 | tech.md §9.1 |
| **조직 격리** | ABAC org_unit narrowing (AUTH-003 정합) | abac.go:29 `orgScopePrefix` |
| **시간 제약** | TIMESTAMPTZ UTC (DB 계층), 미래 KST 업무시간 필터는 후속 PoC | product.md 검토 필요 |

### 13.2 REQ-UBI Dual-Track (EARS 패턴)

**REQ-REPORT-UBI-NNN** 후보:

- REQ-REPORT-UBI-001 (데이터 주권): 집계 내부 PostgreSQL만, 외부 호출 0
- REQ-REPORT-UBI-002 (감사 가능성): read-only이므로 자체 audit 0, SCORE-001 store 위임
- REQ-REPORT-UBI-003 (cli-anonymous 기본값): AUTH 비활성 시 'cli-anonymous' 자동 기록 (store 계층)
- REQ-REPORT-UBI-004 (권한·불변): read-only이므로 write 권한 게이팅 불필요, viewer 포함 모든 authenticated user 허용

---

## 14. 전략 단계 OPEN 항목

### 14.1 범주 롤업 구현 전략

**OPEN [A]**: Option A(핸들러 loop) vs Option B(DB 사이드 JOIN) 중 어느 것 선택?

- Option A [권장]: handler가 `GetEvalItemsByParentID(categoryID)` → 자식 item 목록 조회 → 각 자식마다 `SumWeightedByEvaluationItem` 호출 → 합산. 간단하고 consumer-only 위반 없음.
- Option B: SCORE-001에 신규 메서드 `SumWeightedByCategory(categoryID) → pgtype.Numeric` 추가 — **HARD 위반** (consumer-only).

**권장**: Option A 선택 (consumer-only 제약 준수).

### 14.2 Report 응답 JSON 형식

**OPEN [B]**: Category-level 리포트의 응답 JSON 구조?

예상 엔드포인트 후보:
- `GET /api/v1/reports/category/{categoryID}` — 범주별 집계 리포트 반환
- `GET /api/v1/reports/all` — 모든 범주별 리포트 테이블

예상 응답 형식:
```json
{
  "reports": [
    {
      "category_id": "AX-SAFETY-ORG",
      "category_name": "안전보건",
      "items": [
        {
          "item_id": "AX-SAFETY-ORG-01",
          "item_name": "...",
          "weighted_sum": "85.50",
          "indicators_count": 5
        }
      ],
      "category_total": "85.50",
      "category_grade": "A",
      "evaluation_date": "2026-05-19T12:34:56Z"
    }
  ],
  "generated_at": "2026-05-19T12:34:56Z"
}
```

**결정**: strategy phase에서 PoC 요구사항 및 UI 필요 정보 기반으로 확정.

### 14.3 페이지네이션 & 필터링

**OPEN [C]**: Category list 조회 시 pagination/filtering 필요?

- `GET /api/v1/reports?limit=50&offset=0` — 범주별 리포트 목록 페이징
- `GET /api/v1/reports?scope=default&min_grade=B` — 등급 필터

**결정**: strategy phase에서 PoC 사용 사례 기반으로 확정.

### 14.4 Numeric 정밀도 누적

**OPEN [D]**: Category-level 합산 시 pgtype.Numeric 누적 방식?

- Option 1 [권장]: 각 item의 SumWeightedByEvaluationItem 결과(pgtype.Numeric)를 decimal string으로 파싱 → Go decimal 라이브러리(`shopspring/decimal` 등) 누적 → 정확 십진 보장
- Option 2: 모든 raw score 행을 메모리로 fetch → Go 사이드 Σ 계산 (성능 저하, 메모리 부하)
- Option 3: DB JOIN/GROUP BY로 category-level SUM 계산 (Option B, consumer-only 위반)

**권장**: Option 1 선택 + `decimal` 라이브러리 import (SCORE-001 원칙 준수).

### 14.5 Empty Data 처리

**OPEN [E]**: 범주에 자식 item이 없거나 item에 점수가 없을 경우?

- 빈 리포트 반환 (`{"category_id": "...", "items": [], "category_total": "0", "category_grade": null}`)
- 또는 해당 범주 제외 (null 필터링)

**권장**: 빈 리포트 반환 (data completeness 원칙).

---

## 15. 핵심 결론 & 구현 어드바이스

### 15.1 REPORT-001 = Read-Only HTTP API Layer over SCORE-001/EVAL-ITEM-001

- **패턴 선례**: SCORE-API-001 완전 미러링 가능 (`ScoreHandler` → `ReportHandler`)
- **store 소비**: `ScoreTx.{GetScoresByEvaluationItem, SumWeightedByEvaluationItem, DetermineGrade}` + `EvalItemTx.GetEvalItemsByParentID`
- **범주 롤업**: Option A(핸들러 loop) — consumer-only 준수
- **Audit**: read-only이므로 자체 audit 0 (store 위임, 이미 SCORE 생성 시 기록됨)
- **ABAC**: reader 포함 모든 authenticated user (write 게이팅 불필요, auth-disabled 투과)
- **Numeric**: float64 회피, `pgtype.Numeric` + decimal 라이브러리 누적

### 15.2 신규 파일 & 기존 수정

| 작업 | 범위 | 근거 |
|------|------|------|
| 신규 생성 | `cmd/server/report_handlers.go` + `report_handlers_test.go` | SCORE-API-001 선례 |
| 기존 수정 | `cmd/server/server.go` (필드 + 생성자 + innerMux.Handle 2줄 추가만) | SCORE-API-001 마운트 pattern |
| 0-diff | `internal/store/**`, `internal/auth/**`, `internal/audit/**`, `.moai/db/schema/**`, `evidence_handlers.go` | consumer-only HARD |

### 15.3 TDD Red-Green-Refactor Cycle

1. **RED**: 범주별 리포트 조회 테스트 (예: `TestHandleGetCategoryReport_Success`, `TestHandleGetCategoryReport_NotFound`)
2. **GREEN**: 최소 구현 (GET /api/v1/reports/category/{id} → handler → GetEvalItemsByParentID + SumWeighted loop + DetermineGrade → JSON)
3. **REFACTOR**: 페이지네이션/필터링 추가, 에러 처리 정밀화

---

## 16. 파일 출처 요약

| 주제 | 파일 | 라인 범위 |
|------|------|---------|
| ScoreHandler 패턴 | `score_handlers.go` | 37-70, 82-102, 111-129 |
| 에러 매핑 | `score_handlers.go` | 111-129 |
| TX 오케스트레이션 | `score_handlers.go` | 428-474 |
| ABAC narrowing | `score_handlers.go`, `abac.go`, `rbac.go` | 159-190, 24, 33 |
| EVAL-ITEM 계층 | `store.go`, `0003_eval_item_tables.sql` | 125-152, 7-22 |
| SCORE 계약 | `store.go` | 215-305 |
| Numeric 정밀도 | `score.go` | 450-478, 335-372 |
| Grade Threshold | `score.go` | 591-648 |
| Server 마운트 | `server.go` | 53-55, 207-209, 261-264 |
| SCORE-API-001 spec | `.moai/specs/SPEC-AX-SCORE-API-001/spec.md` | §1.4, §3.1-3.5, research.md §1-14 |

---

## Appendix: Category-Level Aggregation Algorithm

```
Algorithm: CategoryReport(categoryID, scope)
Input: categoryID (평가범주 id), scope (등급 기준 scope)
Output: ReportData{categoryName, items, totalScore, grade}

1. categoryItem := GetEvalItemByID(categoryID)
   if error: return 404 ErrCategoryNotFound

2. childItems := GetEvalItemsByParentID(categoryID)
   if empty: return {categoryName, items: [], totalScore: "0", grade: null}

3. totalScore := 0 (decimal)
   for each childItem in childItems:
       itemScore := SumWeightedByEvaluationItem(childItem.ID)  // pgtype.Numeric
       itemScoreStr := itemScore.Value().(string)  // text-format decimal
       itemScoreDec := decimal.Parse(itemScoreStr)
       totalScore += itemScoreDec
       report.items += {itemID, itemName, itemScore, indicators}

4. totalScoreStr := totalScore.String()  // "85.50"
   grade := DetermineGrade(scope, totalScore.Float64())
   if error == ErrGradeThresholdsUnavailable:
       grade = nil  // null grade when thresholds missing

5. return {
     categoryID, categoryName,
     items: [...],
     totalScore: totalScoreStr,
     grade: grade,
     evaluatedAt: now()
   }
```

---

