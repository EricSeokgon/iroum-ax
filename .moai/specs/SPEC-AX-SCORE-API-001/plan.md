# SPEC-AX-SCORE-API-001 Implementation Plan

> Version: 0.1.0
> Methodology: TDD (RED-GREEN-REFACTOR), brownfield enhancement, sub-agent mode
> Harness: thorough
> Companion: `spec.md` (EARS), `acceptance.md` (Given/When/Then), `research.md` (Phase 0.5 근거, 611줄 file:line SSOT), `spec-compact.md` (압축)

본 plan은 SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 plan.md 구조를 미러링한다. **§6 4건 = Human Gate sign-off(2026-05-19)로 RESOLVED 완료** (strategy.md §A SSOT). 본 SPEC은 SPEC-AX-SCORE-001/EVID-001/AUTH-003의 **순수 consumer** — store/audit/auth/스키마 코드 0 diff.

---

## 1. 목표 & 범위

1차 산출물 = **SPEC-AX-SCORE-001 store 메서드를 노출하는 최소 REST HTTP API 계층 + SPEC-AX-AUTH-003 ABAC 통합** (spec.md §1.1). 7 엔드포인트(GET 단건/목록/롤업/등급, POST 생성, PUT 수정, POST 정정). DB 스키마·FK·신규 마이그레이션·자체 audit·풀 rubric·Console/SDK·6번째 시간제약은 범위 밖 (spec.md §5).

핵심 불변식 (research.md §1/§4.2/§9, source-verified):
- consumer-only [HARD]: `internal/store|audit|auth|errors`, `cmd/server/evidence_handlers.go`, `.moai/db/schema/**` **0 diff** (spec.md §1.4/§2.3 Drift-Guard)
- 신규 마이그레이션 0 (순수 API 계층 — `scores`/`grade_thresholds`는 SCORE-001이 이미 생성)
- API 자체 audit 0 — mutation 감사는 store 계층 `RecordScore*` 동일 TX 전담 (research.md §4.2)
- TX 진입점 = `store.ScoreStore.BeginScoreTx`(→ `PgWorkflowStore.pool`)만. `postgres.go` 死 스텁 비대상 (research.md §1)
- ABAC narrowing-only: `auth.ABACMiddleware(...)(innerMux)`가 이미 innerMux 전체 적용 (server.go:261) — score.go 라우트는 자동 ABAC 적용, server.go ABAC 와이어링 변경 0

---

## 2. 영향받는 파일 (Delta)

spec.md §2 표를 따른다. Delta 마커:

| 경로 | Delta | 비고 |
|------|-------|------|
| `cmd/server/score_handlers.go` | [NEW] | `ScoreHandler`+`NewScoreHandler`+`Routes()`+7 핸들러 메서드+JSON/에러 헬퍼+에러 매핑 (`evidence_handlers.go:82-377` 미러) |
| `cmd/server/score_handlers_test.go` | [NEW] | `httptest` 핸들러 단위 테스트 (7 엔드포인트 × 정상/에러, ABAC/404/409/400/clamp/empty/경계) |
| `cmd/server/server.go` | [MODIFY] | **라우트 마운트만**(≈7줄 최소 단위): `scoreH` 필드 + `NewScoreHandler(...)` 생성자 + `innerMux.Handle` **2줄**(`/api/v1/scores` + `/api/v1/scores/` 서브트리 — Go1.22 ServeMux path-param 구조적 필수) + ko 주석 (`server.go:53/207/261` evidence 선례 정확 미러). ABAC 와이어링 변경 **0-diff** |
| `internal/store/store.go` | [EXISTING] | `ScoreStore`/`ScoreTx`/`Score`/`ScoreUpdate` 호출만 (store.go:217-292) — 0 diff |
| `internal/store/score.go` | [EXISTING] | `PgScoreTx` 메서드 호출만 (score.go:110-648) — 0 diff |
| `internal/errors/errors.go` | [EXISTING] | 에러 센티넬 `errors.Is` 매핑만 (errors.go:54-78) — 0 diff |
| `cmd/server/evidence_handlers.go` | [EXISTING] | 패턴 미러 참조만 — 0 diff |
| `internal/auth/abac.go` | [EXISTING] | 거부 코드·상수·평가자 `ErrCodeABACDenied`(abac.go:24)/`ABACEvaluator` — 호출만, 0 diff (D2-2: 인가 2-파일 중 코드 파일) |
| `internal/auth/authz_middleware.go`/`chain.go` | [EXISTING] | 미들웨어 와이어링 `RESTAuthzMiddleware`/`chain.go:17` — 호출만, 0 diff (D2-2: research.md §4.1 경로 정합, 인가 2-파일 중 미들웨어 파일) |
| `internal/auth/rbac.go`/`middleware.go` | [EXISTING] | `UserFromContext`/`ParseRolesFromScope`/`User`/`RoleAdmin` 호출만 (rbac.go:17-111, middleware.go:25-52) — **permissionMatrix/Authorize frozen, 0 diff [HARD]** |
| `.moai/db/schema/migrations/0004_score_tables.sql` | [EXISTING] | SCORE-001 생성 테이블 — 본 SPEC 마이그레이션 추가/수정 0 |

[EXISTING] 특성화: score 라우트 마운트 추가 후 기존 `evidence_handlers.go`·workflow REST 핸들러 테스트가 GREEN 유지(회귀 0)임을 RED 진입 전 확인.

---

## 3. HTTP 계약 & 핸들러 구조 (research.md §2/§11, evidence_handlers.go 선례)

> §6 4건 RESOLVED 완료(Human Gate 2026-05-19, strategy.md §A). 아래 표는 §6 확정 반영: supersede=POST `/{id}/supersede`(#1), pagination max500/def50 offset(#2/#3), ABAC write={Admin,Analyst} 핸들러-로컬(#4).

### 3.1 엔드포인트 표 (research.md §11 — 잠정)

| Method | Path | store 메서드 | 권한 | HTTP status | 요청 / 응답 |
|--------|------|-------------|------|-------------|-------------|
| GET | /api/v1/scores/{id} | GetScoreByID | read | 200/404/400/500 | — → `Score` JSON |
| GET | /api/v1/scores | GetScoresByEvaluationItem | read | 200/400/500 | `?evaluation_item_id&level&status&offset&limit` → `{scores[],count}` |
| GET | /api/v1/scores/rollup | SumWeightedByEvaluationItem | read | 200/400/500 | `?evaluation_item_id` → `{evaluation_item_id,weighted_sum:"<decimal>"}` |
| GET | /api/v1/scores/grade | DetermineGrade | read | 200/400/404/500 | `?scope&score` → `{scope,score,grade}` |
| POST | /api/v1/scores | InsertScore | write | 201/400/403/500 | `{evaluation_item_id,evidence_id?,level,score_value,weight?,metadata?}` → `{score_id,status}` |
| PUT | /api/v1/scores/{id} | UpdateScore | write | 200/400/403/409/500 | `{score_value?,weight?,grade?,status?,metadata?}` → `{score_id}` |
| POST | /api/v1/scores/{id}/supersede | SupersedeAndReplaceScore | write | 201/400/403/409/500 | `{score_value,weight?,metadata?}` → `{score_id,superseded_id}` (§6 OPEN #1) |

### 3.2 핸들러 구조 (evidence_handlers.go:82-124 미러)

- `ScoreHandler struct { store store.ScoreStore; logger *zap.Logger }` (recorder 불필요 — audit는 store 내부 전담, spec.md §1.4 #4)
- `NewScoreHandler(st store.ScoreStore, logger *zap.Logger) *ScoreHandler`
- `Routes() http.Handler`: `mux := http.NewServeMux()`; Go 1.22+ method-prefixed 패턴 7개 (`GET /api/v1/scores/{id}`, `GET /api/v1/scores`, `GET /api/v1/scores/rollup`, `GET /api/v1/scores/grade`, `POST /api/v1/scores`, `PUT /api/v1/scores/{id}`, `POST /api/v1/scores/{id}/supersede`) — 라우트 우선순위는 ServeMux 최장 일치 규칙으로 `/rollup`·`/grade`·`/supersede`가 `/{id}`보다 우선. `evidence_handlers.go:83-87` 단일 `HandleFunc`와 달리 다중 등록.
- 표준 헬퍼: `writeScoreJSON(w, code, v)` / `writeScoreErr(w, code, errCode, msg, field)` + `scoreErrorBody{Error{Code,Message,Field}}` (`evidence_handlers.go:90-124` 동일 스키마, 한국어 메시지)
- mutation TX orchestration: `tx, err := h.store.BeginScoreTx(ctx)` → `committed := false; defer func(){ if !committed { _ = tx.Rollback(ctx) } }()` → store 메서드 → `tx.Commit(ctx)`; `committed = true` (`evidence_handlers.go:342-353` 정확 미러)
- 에러 매핑(`errors.Is`): `ErrScoreNotFound`→404, `ErrScoreInvalidInput`→400, `ErrScoreImmutable`/`ErrScoreInvalidStatus`/`ErrScoreNotConfirmed`→409, `ErrGradeThresholdsUnavailable`→404, default→500 (errors.go:54-78)
- 입력 검증 pre-TX: `evaluation_item_id` blank/>64자, `score_value` 비수치/누락, `evidence_id` 비-UUID, JSON 파싱 실패, path UUID 파싱 실패 → 400, store TX 미진입 (`evidence_handlers.go:204-218` 선례)
- pagination: `offset`/`limit` query 파싱 + clamp (default/max는 §6 OPEN #2/#3 확정 후 상수화 — workflow `grpc_server.go` clamp 선례)
- ABAC: server.go 기존 `ABACMiddleware(...)(innerMux)`가 자동 적용. write 역할 게이팅 적용 지점은 §6 OPEN #4 확정 (consumer-only [HARD] — rbac.go permissionMatrix 수정 금지)

---

## 4. 구현 접근 (TDD Sprint, no time estimates)

> Sprint 우선순위 라벨: Priority High → Medium. Phase 순서: S0 완료 후 S1, 순차. brownfield — RED 작성 전 `evidence_handlers.go`(핸들러/TX/에러 선례), `score.go`(store 시그니처), `abac.go`(ABAC 동작) 정독(workflow-modes.md Brownfield Enhancement).

| Sprint | 우선순위 | 내용 | REQ |
|--------|----------|------|-----|
| S0 | High | **[D2-1 hard-verify 게이트, 차단]** Run 진입 시 `grep -n 'func.*BeginScoreTx' apps/control-plane/internal/store/pg_store.go apps/control-plane/internal/store/store.go` (실재 기대: `pg_store.go:134`+`store.go:255`) 및 `grep -n 'ErrGradeThresholdsUnavailable' apps/control-plane/internal/errors/errors.go` (실재 기대: `errors.go:69`) hard-verify. **미존재/시그니처 불일치 시 consumer-only 전제 붕괴 → 즉시 STOP·재계획**(research.md §2.1/§12 stale 표현 무시, ground-truth 우선). 통과 후: 회귀 baseline(기존 `evidence_handlers.go`·workflow REST 핸들러 테스트 GREEN 확인) + §6 OPEN 4건 strategy RESOLVED 입력 준비 + consumer-only 대상 파일 git 해시 스냅샷(Drift-Guard 기준선) | (전제) |
| S1 | High | `ScoreHandler` struct + `NewScoreHandler` + `Routes()` 7 라우트 골격 + `writeScoreJSON`/`writeScoreErr`/`scoreErrorBody` 헬퍼 (evidence_handlers.go 미러). server.go 라우트 마운트 1줄 | REQ-SCORE-API-001/002 |
| S2 | High | 조회 핸들러: handleGetScore(404/400)/handleListScores(filter+pagination clamp/empty list)/handleRollup(numeric 직렬화)/handleGrade(ErrGradeThresholdsUnavailable→404) | REQ-SCORE-API-001 |
| S3 | High | 변경 핸들러: handleCreateScore(201)/handleUpdateScore(409 immutable)/handleSupersedeScore(409 not-confirmed) + BeginScoreTx→Commit, defer Rollback(committed flag), pre-TX 검증(400) | REQ-SCORE-API-002 |
| S4 | High | store 에러→HTTP 매핑 표 (`errors.Is` 전 센티넬) + TX rollback 부분커밋 0 + goleak | REQ-SCORE-API-004 |
| S5 | Medium | ABAC 통합 검증 (viewer write→403 / viewer read→200 / authEnabled=false 투과 / admin 투과) + write 역할 게이팅(§6 OPEN #4 확정안 적용) | REQ-SCORE-API-003, REQ-SCORE-API-UBI-004 |
| S6 | Medium | REFACTOR (핸들러 공통 추출: `decodeScoreBody`/`parseScoreID`/`mapStoreErr`/`clampPagination`/`writeScoreErr` — 복잡도 ≥15 회피), @MX 태그, consumer-only 0-diff 검증(BOUNDARY-1), 커버리지 ≥85%, evaluator-active strict ≥0.75 | 전체 |

각 Sprint: RED(실패 테스트) → GREEN(최소 구현) → REFACTOR. store는 fake `ScoreStore`/`ScoreTx`로 격리(`evidence_handlers.go` `evidenceRecorder` 인터페이스 격리 선례) — 통합 테스트는 SCORE-001이 이미 커버하므로 본 SPEC은 핸들러 단위에 집중(over-engineering 회피).

---

## 5. @MX 태그 계획 (code_comments: ko)

| 대상 | 태그 | 사유 |
|------|------|------|
| `score_handlers.go` `ScoreHandler` | `@MX:ANCHOR` + `@MX:REASON` | 점수 REST 진입점 — 핸들러 테스트 + 서버 마운트 + (미래) 통합 테스트 fan_in≥3 (evidence_handlers.go:51 선례) |
| `score_handlers.go` `Routes()` | `@MX:ANCHOR` + `@MX:REASON` | 7 라우트 단일 등록 계약 — server.go 마운트 invariant |
| `score_handlers.go` mutation TX orchestration (Begin/Commit/defer Rollback) | `@MX:WARN` + `@MX:REASON` | committed-flag defer 누락 시 부분 커밋·goroutine 누출 위험 영역 (REQ-SCORE-API-004-U1) |
| `score_handlers.go` `mapStoreErr` (에러→HTTP) | `@MX:WARN` + `@MX:REASON` | 센티넬 매핑 누락 시 409→500 오분류로 클라이언트 혼동 (REQ-SCORE-API-004-S1) |
| `score_handlers.go` 각 핸들러 메서드 | RED `@MX:TODO` → GREEN `@MX:NOTE` | TDD 진행 표시 (EVID-001 패턴) |

REFACTOR 시 단일 함수 복잡도 ≥15 회피 — `decodeScoreBody`/`parseScoreID`/`mapStoreErr`/`clampPagination` 헬퍼 분리.

---

## 6. RESOLVED 결정 (Human Gate sign-off 2026-05-19 완료)

> Human Gate sign-off 완료(2026-05-19). 확정 = strategy.md §A SSOT. consumer-only [HARD] 0-diff 불변.

### 6.1 OPEN #1 [RESOLVED: Option B]

`POST /api/v1/scores/{id}/supersede` body `{score_value, weight?, metadata?}` → 201 `{score_id:"<new>", superseded_id:"<old>"}`. 근거: non-idempotent 복합 연산(store.go:285-291, 매 호출 새 UUID)·EVID-001 append-only POST 선례·ServeMux 최장일치 `/{id}` 무충돌. 거부: A(PUT idempotent 위반)·C(PATCH 복합연산 표현 불가). consumer-only: `SupersedeAndReplaceScore` 호출만 0-diff.

### 6.2 OPEN #2 [RESOLVED: max=500, default=50]

`maxListLimit=500`, `defaultListLimit=50`. clamp: 누락/0→50, >500→500, offset 음수/비수치→0. 근거: store.go:278 미지원→핸들러 메모리 슬라이싱(메커니즘 상이로 1000 답습 약화), p99<50ms NFR 보호. 거부: 1000(workflow 답습). consumer-only: clampPagination 상수 2개, 0-diff.

### 6.3 OPEN #3 [RESOLVED: offset/limit]

offset/limit, `GetScoresByEvaluationItem` 후 핸들러 메모리 `[offset:offset+limit]` 슬라이싱. 근거: store.go:278 cursor/정렬키 미지원→cursor over-engineering(R-API-010). 거부: cursor(post-PoC 이연·store 시그니처 확장=consumer-only 위반). consumer-only: 0-diff.

### 6.4 OPEN #4 [RESOLVED: (a) 핸들러-로컬 + write={RoleAdmin, RoleAnalyst}]

적용 지점 (a) 핸들러-로컬 역할→동작 매핑. write={RoleAdmin, RoleAnalyst}, viewer=read-only, org_unit 행-필터링 비적용. 🔑 `evaluator` INFEASIBLE(rbac.go:33 부재, 사용자 승인 2026-05-19)→`RoleAnalyst` 충실 대체. 근거: OBS-001 `metrics/permission.go` domain-local 이식, `auth.UserFromContext`+`auth.ParseRolesFromScope`, viewer-only mutation→403 `ErrCodeABACDenied`(abac.go:24 호출만), narrowing-only 동형(auth-disabled 투과·admin 우회). 거부: (b)consumer-only/Drift-Guard 위반·(c)frozen 위반→(a) 흡수. consumer-only: rbac.go/abac.go/authz_middleware.go/chain.go **0-diff**, write 판정=score_handlers.go `requireScoreWriteRole`.

### 6.5 결정 간 일관성 (확정)

- #4=(a) 핸들러-로컬 ⟹ rbac.go/abac.go 0 diff (consumer-only [HARD] 보존) — server.go 라우트 마운트만.
- #1=B ⟹ `Routes()` `/supersede` sub-resource 라우트 (ServeMux 최장 일치 — `/{id}` 충돌 없음).
- #2/#3=offset+500/50 ⟹ `clampPagination` 상수 2개, store 슬라이싱은 `GetScoresByEvaluationItem` 후 핸들러 메모리 슬라이싱(SCORE-001 store는 offset/limit 미지원 — research §5).
- 신규 외부 의존 0, phantom 0. **자기인증 한정 [D2-2]**: 실제 source-verified 인용 범위는 `store.go:217-292`/`score.go:110-648`/`errors.go:54-78`(+`errors.go:69`)/`abac.go`(거부 코드·상수)/`authz_middleware.go`·`chain.go:17`(미들웨어)/`rbac.go:17-111`/`evidence_handlers.go:82-377`/`server.go:257-261` + orchestrator ground-truth grep(`pg_store.go:134` `BeginScoreTx`, SCORE-001 커밋 79595d4)이다. research.md §1~§11 전체를 무차별 인용 권위로 삼지 않으며, research.md §2.1/§12의 "BeginScoreTx 미구현/(VERIFY)"·§11 grade 행은 stale/불완전이므로 ground-truth가 우선한다(S0 hard-verify 게이트로 흡수).

---

## 7. 리스크 레지스터

| ID | 리스크 | 영향 | 완화 |
|----|--------|------|------|
| R-API-001 | §6 OPEN 4건 미확정 상태로 Run 진입 시 재작업 | ~~High~~ 해소 | **해소**: Human Gate sign-off 2026-05-19 RESOLVED 완료 (strategy.md §A 확정, #1=POST /supersede·#2=500/50·#3=offset·#4=핸들러-로컬 {Admin,Analyst}) |
| R-API-002 | consumer-only 위반 — store/audit/auth/스키마 1줄이라도 수정 | High | spec.md §1.4/§2.3 Drift-Guard manifest, S0 git 해시 스냅샷, BOUNDARY-1 0-diff 검증, 위반 시 즉시 중단·재설계 |
| R-API-003 | API 자체 audit INSERT로 이중 감사 | High | REQ-SCORE-API-UBI-002 [HARD], `ScoreHandler`에 recorder 의존 미주입, store 위임만, audit INSERT SQL 0건 검증 |
| R-API-004 | phantom API — `evaluator` 역할/score Permission 가정 | High | spec.md §1.5 surface(write 권한 역할 §6 OPEN #4 미확정으로 약화), rbac.go:33 정규식 source-verified, §6 OPEN #4 strategy 해소, S0 hard-verify 게이트, frontmatter Schema note phantom 0 명시 (메모리 lesson #9) |
| R-API-005 | rollup `pgtype.Numeric`를 float64 변환해 정밀도 손실 | Medium | `@MX:WARN`, score.go:453-457 SEC-03 정합, 정확 십진 문자열 직렬화 AC |
| R-API-006 | store 에러 센티넬 매핑 누락 → 409가 500으로 오분류 | Medium | `@MX:WARN`, `errors.Is` 전 센티넬 매핑 표(errors.go:54-78), 매핑 AC 전수 |
| R-API-007 | mutation TX 부분 커밋/goroutine 누출 | High | committed-flag defer Rollback(evidence_handlers.go:342-353 미러), fault injection + goleak AC |
| R-API-008 | `postgres.go` 死 스텁에 작성 | High | `ScoreStore.BeginScoreTx`만 (research.md §1, SCORE-001 R-SCORE-006 선례) |
| R-API-009 | ServeMux 라우트 충돌 (`/{id}` vs `/rollup`·`/grade`·`/supersede`) | Medium | Go 1.22+ 최장 일치 규칙 활용, 라우트 우선순위 테스트 AC |
| R-API-010 | over-engineering — 통합 테스트/SDK/OpenAPI 추가 | Medium | spec.md §5 #7, 핸들러 단위 테스트만(store 통합은 SCORE-001 커버), 최소 API surface |

---

## 8. 완료 정의 (Plan 단계)

- [ ] §2 Delta 표 spec.md §2와 일치, [NEW]/[MODIFY]/[EXISTING] 명시 + Drift-Guard manifest
- [ ] §3 HTTP 계약 evidence_handlers.go 선례 미러 (Routes/표준 에러/TX orchestration/에러 매핑), store 시그니처 source-verified
- [ ] §4 TDD Sprint S0~S6, 우선순위 라벨(High/Medium), 시간 추정 0
- [ ] §5 @MX 계획 (ScoreHandler/Routes ANCHOR, TX/매핑 WARN+REASON, code_comments: ko)
- [x] §6 4건 RESOLVED 완료 (Human Gate sign-off 2026-05-19, strategy.md §A): #1 POST /scores/{id}/supersede · #2 max500/def50 · #3 offset/limit · #4 핸들러-로컬 write={RoleAdmin,RoleAnalyst} (rbac.go/abac.go 0-diff)
- [ ] §7 리스크 레지스터 R-API-001~010 (consumer-only·phantom·이중감사 포함)
- [ ] consumer-only [HARD]: SCORE-001/EVID-001/AUTH-003 코드·스키마·FK·마이그레이션 0 diff 보장 명시
- [ ] 구현 코드/테스트 미작성 (plan 문서만)
