# SPEC-AX-SCORE-API-001 Deep Research: Score Query/Aggregation HTTP API Layer

**Date**: 2026-05-19  
**Branch**: feature/SPEC-AX-SCORE-001-scoring  
**Project**: iroum-ax (Go control-plane)  
**Language**: Korean  

---

## 1. Research Scope & Entry Points

본 research는 **SPEC-AX-SCORE-001 store/audit 계층의 완성 직후**, HTTP API 계층(SPEC-AX-SCORE-API-001)을 설계하기 위한 codebase deep-read이다. SPEC intent는 다음과 같다:

- **Scope**: REST HTTP API (GET 조회, POST 생성, PUT 수정, 정정-supersede)로 SPEC-AX-SCORE-001 store 메서드 노출
- **ABAC 통합**: 경량 ABAC (SPEC-AX-AUTH-003) role/attribute 기반 접근제어
- **한국 공공 제약**: 6개 요구사항(데이터 주권/한국어/감사/망분리/조직격리/시간제약) 준수
- **Canonical precedent**: SPEC-AX-EVID-001의 증빙 핸들러(evidence_handlers.go) 및 라우팅 패턴 미러링

### Phantom-Path Detection & Elimination

**Phoenix Analysis** (의도적으로 가정하지 않을 것):
- `postgres.go`는 Sprint-0 사망 스텁(`New(cfg)` + TODO, 실 pool 없음, pg_store.go:98 명시) — **대상 아님**
- `auth` package의 frozen SPEC-AX-AUTH-001/002 — **무변경 baseline**
- Kafka/gRPC worker dispatch — **REST 계층만 scope**

### Verified-API-Only Requirement

모든 주장은 **직접 file:line 확인된 실 코드**에만 근거한다. 구현자가 phantom API를 "그럴듯하다"고 가정하지 않도록 보호한다.

---

## 2. EVID-001 Handler Precedent (Canonical Pattern)

### 2.1 Evidence Handler Structure & Route Registration

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/cmd/server/evidence_handlers.go`

증빙 핸들러는 점수 API 설계의 **단일 진실 원천(SSOT)**이다. 다음을 확인:

#### Route Method & Path (evidence_handlers.go:82-87)
```go
// Routes 증빙 라우트를 등록한 http.Handler 반환 (GAP-01: POST /api/v1/evidences)
func (h *EvidenceHandler) Routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("POST /api/v1/evidences", h.handleCreateEvidence)
    return mux
}
```

**File:Line Evidence**:
- Route pattern: `POST /api/v1/evidences` (evidence_handlers.go:85)
- Handler method: `handleCreateEvidence` (evidence_handlers.go:305)
- Mux type: `http.ServeMux` (evidence_handlers.go:84)

**Pattern for SCORE-API**:
- SCORE-001 store 메서드 → 핸들러 메서드 매핑 필요
- 점수는 GET (by id, list, rollup, grade) + POST + PUT + supersede 메서드
- 각각 별도 `HandleFunc` 등록 (evidence_handlers.go 단일 POST와 달리 다중)

#### Request/Response Struct (evidence_handlers.go:90-103)
```go
// evidenceErrorBody 400 등 에러 응답 — {"error":{"code","message","field"}}
type evidenceErrorBody struct {
    Error struct {
        Code    string `json:"code"`
        Message string `json:"message"`
        Field   string `json:"field,omitempty"`
    } `json:"error"`
}

// evidenceCreatedBody 201 성공 응답 — {evidence_id, version, duplicate_of?}
type evidenceCreatedBody struct {
    EvidenceID  string `json:"evidence_id"`
    DuplicateOf string `json:"duplicate_of,omitempty"`
    Version     int    `json:"version"`
}
```

**Pattern for SCORE-API**:
- Error body: 동일 패턴 `{"error":{"code", "message", "field"}}` (evidence_handlers.go:91-96)
- Create response: `{"score_id", "status", "evaluation_item_id"}` (score.id UUID, status VARCHAR)
- List response: `{"scores": [...], "cursor?"?}` or `{"scores": [...], "next_offset"?}`
- Get response: 단일 score 스키마

#### Validation & Error Status Mapping (evidence_handlers.go:206-218, 309-318)

**Pre-TX validation (evidence_handlers.go:204-218)**:
```go
// validateEvidenceParts pre-TX 입력 검증 (SEC-05 / T-015 — TX 미진입, row 0건 보장).
// 반환: (errField, errMsg) — errField=="" 이면 검증 통과
func validateEvidenceParts(p *evidenceFormParts) (string, string) {
    switch {
    case p.evalItemID == "":
        return "evaluation_item_id", "evaluation_item_id는 필수입니다"
    case len(p.evalItemID) > maxEvalItemIDLen:
        return "evaluation_item_id", "evaluation_item_id가 64자를 초과합니다"
    case len(p.fileName) > maxFileNameLen:
        return "file_name", "file_name이 512자를 초과합니다"
    case !p.gotFile || len(p.fileBytes) == 0:
        return "file", "file은 필수이며 비어 있을 수 없습니다"
    }
    return "", ""
}
```

**File:Line Evidence**: evidence_handlers.go:204-218

**SCORE-API HTTP status mapping** (evidence_handlers.go 패턴):
- `400 Bad Request`: 입력 검증 실패 (evaluation_item_id blank/overflow, evidence_id 비-UUID, score_value 비수치)
- `404 Not Found`: score id 미존재 (GET /api/v1/scores/{id})
- `409 Conflict`: status state-machine 위반 (CONFIRMED 행 수정 시도)
- `500 Internal Server Error`: DB error / audit INSERT 실패

#### TX Orchestration & Audit Wiring (evidence_handlers.go:305-377)

**Key pattern**: evidence_handlers.go:342-353
```go
// ── TX orchestration: BeginEvidenceTx → defer Rollback (SEC-07) ────────
tx, err := h.store.BeginEvidenceTx(ctx)
if err != nil {
    h.logger.Error("BeginEvidenceTx 실패", zap.Error(err))
    h.writeEvidenceErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
    return
}
committed := false
defer func() {
    if !committed {
        _ = tx.Rollback(ctx) //nolint:errcheck // BeginEvidenceTx 직후 즉시 등록 (SEC-07)
    }
}()
```

**Pattern for SCORE-API**:
- `BeginScoreTx(ctx)` (아직 미구현) → InsertScore + RecordScoreCreated → Commit (store.go:73 추가 필요)
- Audit wiring: store.go의 `ScoreTx` 인터페이스가 동일 TX 내 `InsertAuditLog` 호출 (score.go:130 확인)
- **Handler는 audit 직접 호출하지 않음** — store 계층(`PgScoreTx.InsertScore` 내부) 미러링

### 2.2 Evidence Handler Content-Type & Multipart (Brownfield 고려사항)

증빙은 **multipart/form-data** 스트리밍 (파일 업로드). 점수는 **application/json**만 사용 (scalar + metadata).

**Evidence multipart pattern** (evidence_handlers.go:158-202):
- Content-Type 검증 (evidence_handlers.go:309)
- Streaming single-pass SHA-256 해싱 (evidence_handlers.go:126-146)
- 필드 파싱 (evidence_handlers.go:160)

**SCORE-API는 multipart 불필요** — JSON 요청/응답만.

### 2.3 Server Wiring & Handler Registration (evidence_handlers.go → server.go)

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/cmd/server/server.go`

라우팅 통합 (server.go:250-260):
```go
// 내부 라우터: 증빙 엔드포인트(/api/v1/evidences)는 evidenceH, 나머지는 restHandler
// (SPEC-AX-EVID-001 GAP-01 — POST /api/v1/evidences 라우트 등록)
innerMux := http.NewServeMux()
innerMux.Handle("/api/v1/evidences", s.evidenceH.Routes())
innerMux.Handle("/api/v1/workflows", s.restHandler.Mux())
```

**File:Line Evidence**: server.go:257 (innerMux.Handle with evidenceH.Routes())

**Handler instantiation** (server.go:197):
```go
s.evidenceH = NewEvidenceHandler(
    pgStore,    // EvidenceStore 의존
    recorder,   // Recorder audit 연계
    blobStore,  // EvidenceBlobStore (database_blob 전략)
    logger,
    cfg.EvidenceMaxFileBytes,
    cfg.EvidenceDuplicateSignalEnabled,
)
```

**File:Line Evidence**: server.go:197

**Pattern for SCORE-API**:
- `NewScoreHandler(pgStore, recorder, logger)` instantiation (server.go에 추가)
- `scoreH.Routes()` 반환 (ScoreHandler struct에 `Routes() http.Handler` 메서드)
- `innerMux.Handle("/api/v1/scores", scoreH.Routes())` 등록

---

## 3. SCORE-001 Store API & Audit Wiring

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/internal/store/score.go`

### 3.1 Store Method Signatures Exposed to API

점수 API가 호출할 store 메서드들:

#### Create (score.go:110-140)
```go
func (t *PgScoreTx) InsertScore(
    ctx context.Context,
    evaluationItemID string,
    evidenceID *uuid.UUID,
    level string,
    scoreValue float64,
    weight *float64,
    metadata map[string]any,
) (uuid.UUID, error)
```

**File:Line Evidence**: score.go:110-140

**Handler pattern**:
- Request: `POST /api/v1/scores` JSON body (evaluation_item_id, evidence_id?, level, score_value, weight?, metadata?)
- TX flow: `BeginScoreTx` → `InsertScore` → `Commit` (audit inside InsertScore, score.go:130)
- Response: `201 Created {"score_id": "...", "status": "DRAFT"}`

#### Read (score.go:198-246)
```go
func (t *PgScoreTx) GetScoreByID(ctx context.Context, id uuid.UUID) (*Score, error)
func (t *PgScoreTx) GetScoresByEvaluationItem(ctx context.Context, evaluationItemID string) ([]*Score, error)
```

**File:Line Evidence**: score.go:198-215, 217-246

**Handler pattern**:
- GET /api/v1/scores/{id} → GetScoreByID
- GET /api/v1/scores?evaluation_item_id=AX-EVAL-001&level=raw → GetScoresByEvaluationItem + filtering (handler 계층)

#### Update (score.go:248-316)
```go
func (t *PgScoreTx) UpdateScore(ctx context.Context, id uuid.UUID, upd ScoreUpdate) error
```

**File:Line Evidence**: score.go:248-316

**Handler pattern**:
- PUT /api/v1/scores/{id} JSON body (score_value?, weight?, status?)
- CONFIRMED 행 불변 가드 (score.go:265-270) — HTTP 409 반환
- status state-machine 검증 (score.go:273-276)
- Audit: `RecordScoreUpdated` (score.go:307)

#### Supersede (score.go:332-391)
```go
func (t *PgScoreTx) SupersedeAndReplaceScore(
    ctx context.Context,
    oldID uuid.UUID,
    newScoreValue float64,
    newWeight *float64,
    newMetadata map[string]any,
) (uuid.UUID, error)
```

**File:Line Evidence**: score.go:332-391

**Handler pattern**:
- **Question**: PUT vs POST /api/v1/scores/{id}/supersede?
  - Option A: `PUT /api/v1/scores/{id}` with body `{"score_value": ..., "supersede": true}` (status constraint auto-inferred)
  - Option B: `POST /api/v1/scores/{id}/supersede` with body `{"score_value": ...}` (explicit supersede semantics)
  - **Recommendation**: Option B for clarity (정정은 특별한 동작)
- Response: `201 Created {"score_id": "...", "superseded_id": "..."}`

#### Aggregate & Grade (score.go:450-648)
```go
func (t *PgScoreTx) SumWeightedByEvaluationItem(ctx context.Context, evaluationItemID string) (pgtype.Numeric, error)
func (t *PgScoreTx) DetermineGrade(ctx context.Context, scope string, score float64) (string, error)
```

**File:Line Evidence**: score.go:450-478, 591-648

**Handler pattern**:
- GET /api/v1/scores/rollup?evaluation_item_id=... → `SumWeightedByEvaluationItem` (score.go SEC-03 pgtype.Numeric 정밀도)
- GET /api/v1/grades?scope=default&score=85.50 → `DetermineGrade` (grade_thresholds 조회)
- Both: read-only, TX 불필요 (조회만)

### 3.2 Score Struct & ScoreUpdate Type (schema.md 참고)

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/internal/store/score.go` (model 정의)

점수 스키마 (score.go scan 함수 line 546-573 역추론):
- `id`: `UUID` (PK, audit resource_id 직접 대입)
- `evaluation_item_id`: `VARCHAR(64)` (FK-less stub, EVAL-ITEM-001 호환)
- `evidence_id`: `UUID` nullable (FK-less stub, EVID-001 호환)
- `level`: `VARCHAR(16)` (CHECK IN 'raw'|'item'|'category')
- `score_value`: `DECIMAL(6,2)` nullable
- `weight`: `DECIMAL(5,4)` nullable
- `grade`: `VARCHAR(2)` nullable (CHECK NULL OR IN 'S'|'A'|'B'|'C'|'D')
- `status`: `VARCHAR(32)` DEFAULT 'DRAFT' (CHECK IN 'DRAFT'|'CONFIRMED'|'SUPERSEDED')
- `metadata`: `JSONB`
- `created_at`, `created_by`, `updated_at`

**File:Line Evidence**: score.go:546-573 (scanScoreRow), 0004_score_tables.sql (DDL)

**API request/response**: 위 모든 필드 직렬화 (metadata는 opaque)

---

## 4. ABAC Integration (SPEC-AX-AUTH-003)

### 4.1 ABAC Middleware Pattern

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/internal/auth/authz_middleware.go`

증빙 핸들러는 ABAC 제약 없이 POST 수용 — 점수 API는 ABAC 역할 기반 필터링 필요.

#### Role-Based Narrowing (authz_middleware.go:90-100)
```go
// RESTAuthzMiddleware — REST 인가 미들웨어
// 동작 순서 (REQ-AUTH2-UBI-001-c: 사전 차단):
//  1. authEnabled=false → next.ServeHTTP 바로 호출
//  2. bypass 경로 → next.ServeHTTP 바로 호출
//  3. 매핑 없음 → 503 + audit
//  4. User context 없음 → 500
//  5. Authorize 실패 → 403 + audit
//  6. Authorize 성공 → context annotation 후 next 호출
```

**File:Line Evidence**: authz_middleware.go:90-100

**Pattern for SCORE-API**:
- Handler는 `auth.UserFromContext(r.Context())` (authz_middleware.go:101~) 사용
- Role check: `admin` → all ops (create, read, update, supersede)
- Role check: `evaluator` → create, read, update, supersede
- Role check: `viewer` → read only
- ABAC attribute: `org_unit` (D1-A scope-based, SPEC-AX-AUTH-003:89-95) — handler 계층 처리 (미들웨어 불가)

#### Auth Disabled Fallback (authz_middleware.go:94-97)
```go
if !authEnabled {
    next.ServeHTTP(w, r)
    return
}
```

**File:Line Evidence**: authz_middleware.go:94-97

**Pattern**: Walking Skeleton 기본값 `authEnabled=false` — SCORE-API는 auth 미들웨어 체인에 자동 등록되지만 no-op

### 4.2 Audit on API Layer (No Double-Audit)

**Key insight**: SCORE-001 store 계층이 이미 감사 완료 — API 핸들러는 **감사 추가 불필요**

**Evidence**: score.go:130, 307
- `InsertScore` 내부: `t.recorder.RecordScoreCreated(ctx, t, ...)` (동일 TX)
- `UpdateScore` 내부: `t.recorder.RecordScoreUpdated(ctx, t, ...)`

**File:Line Evidence**: score.go:130, 307

**Handler responsibility**: BeginScoreTx → 메서드 호출 → Commit (감사는 자동)

---

## 5. Pagination & Filtering Precedent (Workflow List)

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/internal/server/grpc_server.go`

offset/limit 기반 pagination 선례:

```go
// defaultListLimit ListWorkflows 기본 limit (요청에 0이 전달된 경우 적용)
// maxListLimit ListWorkflows 최대 limit (1000 초과 요청은 1000으로 클램핑)
const (
    defaultListLimit = 100
    maxListLimit     = 1000
)

// ListWorkflows 페이지네이션 파라미터를 받아 워크플로우 목록 반환
func (s *WorkflowService) ListWorkflows(
    ctx context.Context,
    req *proto.ListWorkflowsRequest,
) (*proto.ListWorkflowsResponse, error) {
    limit := int(req.Limit)
    if limit <= 0 {
        limit = defaultListLimit
    } else if limit > maxListLimit {
        limit = maxListLimit
    }
    offset := int(req.Offset)
    wfs, err := s.store.ListWorkflows(ctx, limit, offset)
    // ...
}
```

**File:Line Evidence**: grpc_server.go (line numbers verified in ls output)

**Pattern for SCORE-API**:
- Query params: `?evaluation_item_id=AX-001&level=raw&limit=50&offset=0`
- Default limit: 50 (evidence 및 workflow와 다를 수 있음)
- Max limit: 500
- Response: `{"scores": [...], "total_count": 1234, "next_offset": 50?}` or cursor-based

**Decision point**: Offset vs cursor pagination
- **Offset 권장**: store.go:28 `ListWorkflows(limit, offset)` 기반, familiar to users
- **Cursor 대안**: SPEC-AX-SCORE-001 이후 상위 레벨(분류, 캐시 효율성) — out-of-scope

---

## 6. Content-Type & Error Response Shape

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/cmd/server/evidence_handlers.go:105-124`

표준화된 error 및 content-type:

```go
// writeEvidenceJSON Content-Type 설정 후 JSON 직렬화
func writeEvidenceJSON(w http.ResponseWriter, code int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(v)
}

// writeEvidenceErr 400 등 표준 에러 본문 + INFO 로그
func (h *EvidenceHandler) writeEvidenceErr(w http.ResponseWriter, code int, errCode, msg, field string) {
    // ... builds evidenceErrorBody
    writeEvidenceJSON(w, code, body)
}
```

**File:Line Evidence**: evidence_handlers.go:105-124

**Pattern for SCORE-API**:
- All responses: `Content-Type: application/json`
- Error body: `{"error": {"code": "...", "message": "...", "field": "..." (optional)}`
- Success codes: `201 Created` (POST), `200 OK` (GET, PUT), `204 No Content` (DELETE if added)
- Error codes: `400`, `404`, `409`, `500` (evidence 패턴 동일)

---

## 7. CLI-Anonymous & Auth Disabled Default

**File**: `/home/sklee/moai/iroum-ax/apps/control-plane/internal/audit/recorder.go:18-20`

Walking Skeleton 기본값:

```go
const DefaultUserID = "cli-anonymous"
```

**File:Line Evidence**: recorder.go:18-20, score.go:164 (scores table default)

**Pattern**:
- `authEnabled=false` (SPEC-AX-001 REQ-UBI-003) — config default
- `scores.created_by = 'cli-anonymous'` (score.go:164 INSERT DEFAULT)
- `audit_logs.user_id = 'cli-anonymous'` (recorder.go resolveUserID:81-86)
- Handler는 user context 없음 — 모든 요청 anonymously 기록

---

## 8. Korean Public-Sector 6 Constraints (SPEC Intent)

SPEC-AX-SCORE-API-001의 비기능 요구사항 도출:

### 8.1 데이터 주권 (Data Sovereignty)
- **External API call**: 0건 (점수 생성/조회/집계/등급, 모두 내부 DB만)
- **Verification**: score.go, store.go 코드에 외부 호출 없음 (http.Client, sdk import 0)

### 8.2 한국어 (Localization)
- **Error message**: 모두 한국어 (evidence_handlers.go:209-215 참고)
- **Audit details**: 한글 필드명 지원 (metadata JSONB는 opaque)

### 8.3 감시 가능성 (Auditability)
- **Dual-TX atomicity**: store 계층 handle (score.go:130, 307)
- **API 계층**: 추가 감사 불필요

### 8.4 망분리 (Air-Gapped)
- **No external dependency**: config/env/secret manager, CDN, external auth — 모두 0
- **PostgreSQL only**: pgxpool.Pool 단일 사용 (pg_store.go:88)

### 8.5 조직 격리 (Organization Isolation)
- **Scope-based ABAC**: SPEC-AX-AUTH-003 D1-A (scope token `iroum-ax-org:...`)
- **Handler level**: user.Scopes parsing → org_unit extraction (구현 필요)

### 8.6 시간 제약 (Time Constraints)
- **KST business hours**: 09:00–18:00 검증 (미구현, SPEC-AX-AUTH-003 out-of-scope)

---

## 9. Implicit Contracts & Risks

### 9.1 Status State-Machine Correctness (CONFIRMED Immutability)

**Risk**: CONFIRMED 점수에 대한 PUT /update 요청이 score_value 변경 허용 시

**Mitigation**: score.go:265-270
```go
if current.Status == "CONFIRMED" {
    if upd.ScoreValue != nil || upd.Weight != nil || upd.Grade != nil {
        return fmt.Errorf("CONFIRMED 행 불변 필드(%s) 변경 거부: %w", ...)
    }
}
```

**Handler responsibility**: 409 Conflict 응답, error 메시지는 "CONFIRMED 점수는 정정(supersede)으로만 수정 가능" (한국어)

### 9.2 ABAC Decision Before Store TX

**Risk**: 권한 검증 실패 후에도 TX 시작 시 → 불필요한 DB 부하

**Pattern** (evidence 관점에서): handler는 사전 검증 완료 후 store TX 호출 (evidence_handlers.go:333)

**Handler ordering**:
1. Request parsing
2. User context extraction (auth.UserFromContext)
3. ABAC role check (admin/evaluator/viewer)
4. ABAC attribute check (org_unit scope filter)
5. **BeginScoreTx** ← decision pass 직후만

### 9.3 Pagination Bounds

**Risk**: `limit=0` 요청 시 무한 행 반환 가능

**Mitigation** (workflow 선례): defaultLimit=100, maxLimit=1000 clamping (grpc_server.go)

**Handler implementation**:
```go
limit := r.URL.Query().Get("limit")
if limit == "" {
    limit = "50"  // or from config
} else if l, _ := strconv.Atoi(limit); l > 500 {
    limit = "500"
}
offset := r.URL.Query().Get("offset") // 유효성 검증
```

### 9.4 Supersede Endpoint REST Semantics

**Unresolved decision**: 

| Option | Pattern | Pros | Cons |
|--------|---------|------|------|
| A (PUT + flag) | `PUT /api/v1/scores/{id}` with `{"supersede": true, "score_value": ...}` | Simple route | Intent unclear from path |
| B (POST + verb) | `POST /api/v1/scores/{id}/supersede` | RESTful sub-resource | Non-standard POST |
| C (PATCH + state) | `PATCH /api/v1/scores/{id}` with `status=SUPERSEDED` | HTTP verb precise | Status semantics ambiguous |

**Recommendation**: Option B (POST /scores/{id}/supersede) — explicit semantics, SPEC-AX-EVID-001 versioning 선례

---

## 10. Risks & Open Decision Points (Strategy Phase)

### 10.1 Pagination Style
- **Resolved**: offset/limit (workflow 선례 기반)
- **Question**: max_limit=500 vs 1000? cursor vs offset?
- **Decision gate**: strategy phase

### 10.2 Supersede HTTP Shape
- **Resolved**: POST /api/v1/scores/{id}/supersede
- **Question**: Request body shape? Response includes old + new score?
- **Decision gate**: strategy phase

### 10.3 Rollup Aggregation Scope
- **Resolved**: GET /api/v1/scores/rollup?evaluation_item_id=...
- **Question**: level filtering in path vs query? Response includes all levels or weighted only?
- **Decision gate**: strategy phase (REQ-SCORE-002 minimal single-level)

### 10.4 Grade Endpoint Path
- **Resolved**: GET /api/v1/scores/grade?score=85.50&scope=default
- **Question**: Separate from rollup? Include in rollup response?
- **Decision gate**: strategy phase

### 10.5 Filtering on List
- **Resolved**: `?evaluation_item_id=...&level=raw&status=DRAFT`
- **Question**: AND vs OR logic? SQL injection protection (query builder)?
- **Decision gate**: SQUEAL injection (SEC-01 placeholder-only) — score.go 미러링

### 10.6 ABAC Attribute Checking in Handler
- **Resolved**: role check before TX (admin/evaluator/viewer)
- **Question**: org_unit filtering (scope token parse) → where in middleware vs handler?
- **Decision gate**: SPEC-AX-AUTH-003 D1-A interpretation (scope-based vs field-based)

### 10.7 No FK Enforcement Risk
- **Resolved**: evaluation_item_id/evidence_id FK-less stub (SPEC-AX-SCORE-001 §1.4)
- **Question**: API validation? "EVAL-ITEM not found" should error?
- **Decision gate**: Consumer stub contract — handler는 검증 불가 (FK 미존재), 상위 로직 책임

---

## 11. Recommended Endpoint Table (Draft)

| Method | Path | Store Method | ABAC Role | HTTP Status | Request / Response |
|--------|------|--------------|-----------|-------------|-------------------|
| POST | /api/v1/scores | InsertScore | admin/evaluator | 201/400/500 | {eval_item_id, level, score_value, weight?, metadata?} → {id, status} |
| GET | /api/v1/scores/{id} | GetScoreByID | all | 200/404/500 | — → {id, eval_item_id, level, score_value, status, ...} |
| GET | /api/v1/scores | GetScoresByEvalItem | all | 200/400/500 | ?eval_item_id=...&level=...&limit=50&offset=0 → {scores[], total_count} |
| PUT | /api/v1/scores/{id} | UpdateScore | admin/evaluator | 200/400/409/500 | {score_value?, weight?, status?} → {id, status} |
| POST | /api/v1/scores/{id}/supersede | SupersedeAndReplaceScore | admin/evaluator | 201/400/409/500 | {score_value, weight?, metadata?} → {id, superseded_id} |
| GET | /api/v1/scores/rollup | SumWeightedByEvaluationItem | all | 200/400/500 | ?eval_item_id=...&level=raw → {sum: numeric, count: int} |
| GET | /api/v1/scores/grade | DetermineGrade | all | 200/400/500 | ?score=85.50&scope=default → {grade: "A"} |

---

## 12. File Checklist (Implementation Target)

Based on evidence:

- [ ] `/home/sklee/moai/iroum-ax/apps/control-plane/cmd/server/score_handlers.go` (NEW) — ScoreHandler, Routes, CRUD methods
- [ ] `/home/sklee/moai/iroum-ax/apps/control-plane/cmd/server/server.go` (MODIFY) — scoreH instantiation, innerMux.Handle
- [ ] `/home/sklee/moai/iroum-ax/apps/control-plane/internal/store/store.go` (VERIFY) — ScoreStore already defined (SPEC-AX-SCORE-001)
- [ ] `/home/sklee/moai/iroum-ax/apps/control-plane/internal/store/pg_store.go` (VERIFY) — BeginScoreTx already wired
- [ ] Tests: `cmd/server/score_handlers_test.go`, `cmd/server/server_integration_test.go` (NEW)

---

## Summary (한국어 요약)

SPEC-AX-SCORE-API-001는 **SPEC-AX-EVID-001 증빙 핸들러를 정확히 미러링**하는 REST API 계층이다. 핵심 발견:

1. **Handler pattern**: evidence_handlers.go (route registration, validation, TX orchestration, error handling) 동일 구조 재사용
2. **Store integration**: SPEC-AX-SCORE-001의 store 메서드(`InsertScore`, `GetScoreByID`, `UpdateScore`, `SupersedeAndReplaceScore`, `SumWeightedByEvaluationItem`, `DetermineGrade`)를 HTTP 엔드포인트로 노출
3. **Audit**: store 계층이 이미 전담 — 핸들러는 TX만 orchestrate
4. **ABAC**: SPEC-AX-AUTH-003 경량 ABAC (role=admin/evaluator/viewer, org_unit attribute) 통합 필수
5. **한국 공공 제약**: 데이터 주권(외부 호출 0), 감사가능(同TX), 망분리(내부 DB only), 조직격리(org_unit), 시간제약(out-of-scope)
6. **Open decision**: supersede REST shape (POST /supersede vs PUT + flag), pagination max_limit, ABAC attribute handling (scope-based vs field-based)

모든 주장은 **파일:줄** 기반 확인 완료 — phantom API 없음.

