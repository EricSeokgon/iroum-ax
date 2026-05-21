---
id: SPEC-AX-SCORE-API-001
version: 0.1.1
status: completed
created: 2026-05-19
updated: 2026-05-19
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.1 (2026-05-19): Sync — TDD sub-agent 진성 RED-first 구현, 커밋 8a61193. 이중 게이트 PASS: evaluator-active 0.9235 / manager-quality TRUST 5 PASS, 커버리지 95.79%. consumer-only 0-diff 확인(store/audit/auth/migrations 무변경). plan-audit D3-NEW-1 정리(ABACMiddleware↔RESTAuthzMiddleware 출처 정합), 서버 마운트 기술 정밀화(1줄→≈7줄 최소 단위), §6 4건 RESOLVED. 상세: §2.1/§1.5 D2-2 reconciliation 반영.
- 0.1.0 (2026-05-19): 경영평가 점수 조회/집계 HTTP API 계층(Score Query/Aggregation HTTP API Layer) 첫 초안. SPEC-AX-SCORE-001(완료, v0.1.3)의 `ScoreStore`/`ScoreTx` store+audit 계층 위에 **REST HTTP API 계층만** 추가한다(SPEC-AX-EVID-001 `evidence_handlers.go` 핸들러·라우팅 선례 미러링, research.md §2). 노출 엔드포인트: GET 단건(`/api/v1/scores/{id}`)·GET 목록(filter: evaluation_item_id/level/status + offset/limit)·GET 가중 롤업(`/api/v1/scores/rollup`)·GET 등급(`/api/v1/scores/grade`)·POST 생성·PUT 수정·POST CONFIRMED 정정(supersede). SPEC-AX-AUTH-003 경량 ABAC 통합(viewer=read-only / write 권한 역할은 §6 OPEN #4 미확정 — frozen `rbac.go`에 `evaluator` 역할 부재; cli-anonymous 기본값 + auth-disabled Walking Skeleton fallback). 한국 공공 6제약(데이터 주권/한국어/감사 가능성/망분리/조직 격리/시간 제약) 준수. **본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 / SPEC-AX-AUTH-003의 consumer이며 그 코드·스키마·FK·마이그레이션을 일절 변경하지 않는다 — DB 변경 0, 신규 마이그레이션 0(순수 API 계층), 자체 audit 0(store 계층 전담)**. 풀 rubric 시스템, 6번째 시간 제약(KST 업무시간), AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC은 의도적 제외(§5 Exclusions). research.md(Phase 0.5 deep research, 611줄, file:line 근거)가 SSOT. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 / SPEC-AX-AUTH-003과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등 canonical 외 필드는 사용하지 않는다. 본 SPEC의 모든 EARS 요구사항·영향파일·HTTP 계약은 `.moai/specs/SPEC-AX-SCORE-API-001/research.md`(file:line 근거)에 근거하며, 소비 계약 시그니처는 `internal/store/store.go`(`ScoreStore`/`ScoreTx`/`Score`/`ScoreUpdate`), `internal/store/score.go`(`PgScoreTx` 메서드), `internal/errors/errors.go`(에러 센티넬), `cmd/server/evidence_handlers.go`(핸들러 선례), `internal/auth/abac.go`·`rbac.go`·`middleware.go`(ABAC/RBAC), `cmd/server/server.go`(라우트 마운트)에서 직접 검증되었다(phantom API 0건).

---

# SPEC-AX-SCORE-API-001 — 경영평가 점수 조회/집계 HTTP API 계층 (Score Query/Aggregation HTTP API Layer)

## 1. 개요

경영평가팀(및 후속 Console/외부 클라이언트)이 SPEC-AX-SCORE-001이 완성한 점수 store 계층을 HTTP로 조회·생성·수정·정정·집계·등급조회할 수 있도록, `apps/control-plane/cmd/server/`(Go 1.23+)에 **REST HTTP API 계층**을 추가한다. 본 SPEC은 SPEC-AX-EVID-001의 증빙 핸들러(`evidence_handlers.go` — 라우트 등록·검증·TX orchestration·표준 에러)를 정확히 미러링하며(research.md §2/§6), SPEC-AX-AUTH-003 경량 ABAC을 통합하고, 모든 변경의 감사는 **이미 store 계층(SPEC-AX-SCORE-001 `InsertScore`/`UpdateScore`/`SupersedeAndReplaceScore` 내부 `RecordScore*`)이 동일 TX로 전담**하므로 API 계층은 audit를 추가하지 않는다(research.md §4.2).

### 1.1 API 계층의 의미 (본 SPEC 범위)

본 SPEC의 1차 산출물은 **SPEC-AX-SCORE-001 store 메서드를 HTTP 엔드포인트로 노출하는 최소 REST API 계층 + ABAC 통합**이다. 데이터 모델·store 메서드·audit 연계·가중 롤업·등급 산출 로직 자체는 SPEC-AX-SCORE-001이 이미 GREEN(완료, v0.1.3)으로 제공한다 — 본 SPEC은 그 **consumer**이며 신규 비즈니스 로직·DB 스키마·마이그레이션을 추가하지 않는다.

- 신규 파일 2개: `cmd/server/score_handlers.go`(`ScoreHandler` + `Routes()` + 핸들러 메서드), `cmd/server/score_handlers_test.go`(테스트)
- 기존 1개 수정: `cmd/server/server.go` — **라우트 마운트 최소 단위(필드+생성자+innerMux.Handle 2줄, ≈7줄) + 핸들러 인스턴스화만** (`evidence_handlers.go` → `server.go:257` 선례, research.md §2.3)
- 7개 엔드포인트: GET 단건 / GET 목록(filter+pagination) / GET 롤업 / GET 등급 / POST 생성 / PUT 수정 / POST 정정(supersede)
- ABAC 통합: SPEC-AX-AUTH-003 경량 ABAC narrowing-only(viewer=read-only GET, write 권한 역할=§6 OPEN #4 미확정 — POST/PUT/supersede 게이팅)
- cli-anonymous 기본값 + auth-disabled Walking Skeleton fallback (SPEC-AX-SCORE-001 §1.1, AUTH-003 정합)
- 표준 에러: `evidence_handlers.go` `{"error":{"code","message","field"}}` 동일 스키마(research.md §6)

### 1.2 Anchor 컨텍스트

본 SPEC은 SPEC-AX-SCORE-001이 형성한 점수 흐름(지표별 raw score → EVAL-ITEM weight 가중 롤업 → 항목 → 범주 → 등급 threshold)을 **HTTP로 외부에 노출**하여 `product.md` §3.2 기획재정부 경영평가 편람의 채점·집계·등급을 클라이언트가 조회/입력할 수 있게 한다. PoC 범위는 "안전보건" 범주의 점수 CRUD + 최소 롤업/등급 조회 API이며, SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 / SPEC-AX-AUTH-003 / SPEC-AX-CTRL-001은 GREEN(완료) 상태로 가정한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `SCORE-API` (Evaluation Scoring HTTP API sub-domain — SPEC-AX-SCORE-001의 API 계층 분리)
- 따라서 SPEC ID: `SPEC-AX-SCORE-API-001` (`.claude/skills/moai/workflows/plan.md` Composite domain rules "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내 — `AX` + `SCORE-API`)

### 1.4 의존성 stub 계약 — consumer-only [핵심, load-bearing]

본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 / SPEC-AX-AUTH-003의 **순수 consumer**이다. 다음이 **HARD 계약**이다(research.md §1 phantom-path 회피, §10.7):

- **[HARD]** 본 SPEC은 `internal/store/score.go`·`store.go`(`ScoreStore`/`ScoreTx`/`Score`/`ScoreUpdate`), `internal/audit/*`(`RecordScore*`), `internal/auth/*`(ABAC/RBAC), `.moai/db/schema/migrations/*.sql`을 **일절 수정하지 않는다**. 점수 비즈니스 로직·감사·접근제어·DB 스키마는 호출만 한다.
- **[HARD]** `scores.evaluation_item_id`(VARCHAR(64))·`scores.evidence_id`(UUID)는 SPEC-AX-SCORE-001 §1.4가 확립한 **FK-제약-없는 stub**으로 유지된다. 본 SPEC은 API 요청 본문에서 이 두 값을 받아 store에 그대로 전달하며, 참조 무결성 검증(EVAL-ITEM/EVID 존재 여부)은 **수행하지 않는다**(FK 부재 — 상위 로직 책임, research.md §10.7).
- **[HARD]** 신규 DB 마이그레이션 0건 — 본 SPEC은 순수 API 계층이므로 `0004_score_tables.sql` 등 어떤 마이그레이션도 추가/수정하지 않는다. `scores` / `grade_thresholds` 테이블은 SPEC-AX-SCORE-001이 이미 생성했다.
- **[HARD]** 자체 audit 0건 — 모든 mutation(POST/PUT/supersede)의 `audit_logs` 기록은 SPEC-AX-SCORE-001 store 메서드 내부(`InsertScore`/`UpdateScore`/`SupersedeAndReplaceScore` → `RecordScore*`)가 동일 TX로 전담한다(research.md §4.2, §3.1). API 핸들러가 별도 audit row를 INSERT하면 **이중 감사**가 되므로 금지된다(REQ-SCORE-API-UBI-002).
- **[HARD]** `postgres.go`는 Sprint-0 死 스텁(SPEC-AX-SCORE-001 plan.md §2)이며 본 SPEC 대상 아님. TX 진입점은 `store.ScoreStore.BeginScoreTx`(→ `pg_store.go PgWorkflowStore.pool`)만 사용한다(research.md §1, §2.3).

### 1.5 ABAC 권한 경계 [핵심]

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 통합한다. 확립된 사실(research.md §4, `abac.go`/`authz_middleware.go`/`rbac.go` 검증):

> **[ABAC 출처 reconciliation 노트 — D2-2]** SPEC-AX-AUTH-003 인가 서브시스템은 **두 개의 실재 파일**로 구성된다 — (1) **권한 거부 코드·상수·평가자**: `internal/auth/abac.go`(`const ErrCodeABACDenied = "ABAC_CONDITION_DENIED"` abac.go:24, `ABACEvaluator`, narrowing-only 평가 로직), (2) **미들웨어 와이어링**: `internal/auth/authz_middleware.go`의 `RESTAuthzMiddleware`(REST 인가 미들웨어) + `chain.go:17`("기본 RESTAuthzMiddleware" 빌드). research.md §4.1은 `authz_middleware.go`/`RESTAuthzMiddleware` 경로를 기술하고 본 spec.md는 거부 코드/상수를 `abac.go`로 인용 — 이는 **phantom이 아니라 동일 인가 서브시스템의 두 실재 파일 간 출처 발산**일 뿐이며 명칭은 정합한다(코드/상수=abac.go, 미들웨어 체인=authz_middleware.go/chain.go). 본 SPEC은 두 파일 모두 [EXISTING] consumer로 호출만 하며 0 diff이다.

- 읽기(GET 단건/목록/롤업/등급): `viewer` 역할 포함 모든 인증 사용자 허용(read-only).
- 쓰기(POST/PUT/supersede): **write 권한 보유 principal**만 허용 — 정확한 write 역할 매핑은 §6 OPEN #4 미확정(frozen `rbac.go`에 `evaluator` 역할 부재이므로 핸들러-로컬 매핑 vs `analyst` 재사용 vs ABAC 정책 주입 중 strategy phase 결정).
- `authEnabled=false`(Walking Skeleton 기본값, SPEC-AX-SCORE-001 §1.1 정합): ABAC/RBAC 미들웨어가 자동 투과(`abac.go:8` REQ-ABAC-009, 미들웨어 체인 `authz_middleware.go`/`chain.go:17`, `cli-anonymous` 기록 — store 계층이 처리). 본 SPEC은 인증 비활성에서도 동작한다.
- `RoleAdmin` 보유: 모든 ABAC 조건 우회(`abac.go:71` REQ-ABAC-004).
- ABAC는 narrowing-only — RBAC가 통과시킨 요청만 추가 거부할 수 있고 권한을 부여하지 않는다(`abac.go:11`).

> **[중요 경계 — strategy phase 해소 대상, §6 OPEN #4]** `internal/auth/rbac.go`의 RBAC 역할은 `RoleAdmin`/`RoleAnalyst`/`RoleViewer`만 존재하며 **`evaluator` 역할은 부재**하고(`rbac.go:19-26`, scope 정규식 `^iroum-ax:(admin|analyst|viewer)$` — `rbac.go:33`), `permissionMatrix`(`rbac.go:39-60`)에 **score 관련 Permission이 0건**이다. consumer-only [HARD] 제약상 본 SPEC은 `permissionMatrix`/`Authorize` 등 frozen RBAC을 수정할 수 없다. 따라서 "viewer=read / write 권한 역할=write"의 정확한 write 역할 매핑 및 적용 지점(핸들러-로컬 역할→동작 매핑 vs ABAC 정책 주입 vs `analyst` 역할 재사용 — `evaluator`는 frozen rbac.go에 부재)은 §6 OPEN #4에서 strategy phase에 결정한다. 본 SPEC은 그 결정과 무관하게 불변인 ABAC narrowing 동작(미인가 쓰기 → 403, viewer 읽기 → 허용, auth-disabled → 투과)만 EARS로 고정한다.

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` `apps/control-plane/` 트리를 따른다. 본 SPEC은 store/audit/auth 코드는 일절 수정하지 않고(consumer-only §1.4 HARD), API 계층 파일만 추가한다. Delta 마커: [EXISTING]=consumer로 호출만(무변경), [NEW]=신규 추가, [MODIFY]=라우트 마운트만.

### 2.1 Go Control Plane API 계층 (`apps/control-plane/cmd/server/`)

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/cmd/server/score_handlers.go` | `ScoreHandler` struct + `NewScoreHandler(...)` + `Routes() http.Handler` + 7개 핸들러 메서드(handleGetScore/handleListScores/handleRollup/handleGrade/handleCreateScore/handleUpdateScore/handleSupersedeScore) + 표준 JSON/에러 헬퍼(`evidence_handlers.go:82-124` 선례 미러). store 에러 센티넬→HTTP status 매핑. | [NEW] | REQ-SCORE-API-001~004 |
| `apps/control-plane/cmd/server/score_handlers_test.go` | `httptest` 기반 핸들러 단위 테스트 — 7 엔드포인트 정상/에러 경로, ABAC deny 403, 404, 409, 400, pagination clamp, auth-disabled 투과, empty list, consumer-only 경계(API audit 0). | [NEW] | 전체 |
| `apps/control-plane/cmd/server/server.go` | **라우트 마운트만**(≈7줄 최소 단위): `scoreH` 필드(server.go:55) + `s.scoreH = NewScoreHandler(...)` 생성자(server.go:209) + `innerMux.Handle` **2줄**(server.go:263-264: `/api/v1/scores` + `/api/v1/scores/` 서브트리 — Go1.22 ServeMux path-param 라우팅 구조적 필수, evidence 단일 라우트와 달리 7라우트는 서브트리 필요) + ko 주석 (`server.go:53/207/261` 선례 미러). ABAC은 기존 `RESTAuthzMiddleware`(`authz_middleware.go`/`chain.go:17`) 미들웨어 체인이 innerMux 전체를 감싸 자동 적용(`server.go:261` 기존 와이어링, D2-2 reconciliation: 미들웨어 와이어링 파일) — ABAC 와이어링 변경 **0-diff**. | [MODIFY] | REQ-SCORE-API-003 |

### 2.2 소비 계약 — 호출만, 무변경 (consumer-only [HARD] §1.4)

| 경로 | 소비 계약 | Delta |
|------|----------|-------|
| `apps/control-plane/internal/store/store.go` | `ScoreStore.BeginScoreTx`, `ScoreTx`(InsertScore/GetScoreByID/GetScoresByEvaluationItem/UpdateScore/SupersedeAndReplaceScore/SumWeightedByEvaluationItem/DetermineGrade/Commit/Rollback), `Score`/`ScoreUpdate` struct — 호출만 (store.go:217-292 검증) | [EXISTING] |
| `apps/control-plane/internal/store/score.go` | `PgScoreTx` 구현 — 호출만 (score.go:110-648 검증). `InsertScore`/`UpdateScore`/`SupersedeAndReplaceScore` 내부가 `RecordScore*` 동일-TX audit 전담 | [EXISTING] |
| `apps/control-plane/internal/errors/errors.go` | 에러 센티넬 `ErrScoreNotFound`/`ErrScoreInvalidInput`/`ErrScoreImmutable`/`ErrScoreInvalidStatus`/`ErrScoreNotConfirmed`/`ErrScoreAuditWriteFailed`/`ErrGradeThresholdsUnavailable` — `errors.Is`로 매핑만 (errors.go:54-78 검증) | [EXISTING] |
| `apps/control-plane/cmd/server/evidence_handlers.go` | 핸들러 구조·`Routes()`·`writeEvidenceJSON`/`writeEvidenceErr`·TX orchestration 선례 — 패턴 미러만(코드 무변경, evidence_handlers.go:82-124/305-377 검증) | [EXISTING] |
| `apps/control-plane/internal/auth/abac.go` | **거부 코드·상수·평가자** `ErrCodeABACDenied`(abac.go:24)/`ABACEvaluator`/`OrgUnitFromUser`/narrowing-only 로직 — 호출만, 본 SPEC abac.go 무변경 (D2-2 reconciliation: 인가 서브시스템 2-파일 중 코드·상수 파일) | [EXISTING] |
| `apps/control-plane/internal/auth/authz_middleware.go`, `chain.go` | **미들웨어 와이어링** `RESTAuthzMiddleware`(REST 인가 미들웨어) + `chain.go:17`(기본 RESTAuthzMiddleware 빌드) — server.go 기존 와이어링이 innerMux 전체 적용(server.go:261 검증). research.md §4.1 경로와 정합. 본 SPEC 무변경 (D2-2 reconciliation: 2-파일 중 미들웨어 파일) | [EXISTING] |
| `apps/control-plane/internal/auth/rbac.go`, `middleware.go` | `RoleAdmin`/`RoleViewer`/`RoleAnalyst`, `ParseRolesFromScope`, `UserFromContext`, `User` struct — 호출만 (rbac.go:17-111, middleware.go:25-52 검증). **permissionMatrix/Authorize frozen — 무변경 [HARD]** | [EXISTING] |
| `.moai/db/schema/migrations/0004_score_tables.sql` | SPEC-AX-SCORE-001이 생성한 `scores`/`grade_thresholds` 테이블 — 본 SPEC은 마이그레이션 추가/수정 0건 (순수 API §1.4 HARD) | [EXISTING] |

### 2.3 Drift-Guard Manifest

[NEW]만 신규 생성, [MODIFY]는 정확히 1파일·라우트 마운트 범위, [EXISTING]은 0 diff. 구현 중 [EXISTING] 파일에 1줄이라도 수정 발생 시 consumer-only [HARD] 위반 → 즉시 중단·재계획(plan.md §7 R-API-002).

- 신규 생성 허용: `cmd/server/score_handlers.go`, `cmd/server/score_handlers_test.go`
- 수정 허용(라우트 마운트 한정): `cmd/server/server.go` (`scoreH` 필드 + `NewScoreHandler` 호출 + `innerMux.Handle("/api/v1/scores", ...)` 1줄)
- 수정 절대 금지(0 diff 검증): `internal/store/*.go`, `internal/audit/*.go`, `internal/auth/*.go`, `internal/errors/*.go`, `cmd/server/evidence_handlers.go`, `.moai/db/schema/**`

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건)

Ubiquitous 요구사항은 SPEC-AX-SCORE-001 / SPEC-AX-EVID-001 / SPEC-AX-AUTH-003의 canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track 패턴을 도메인 스코프 형태(`REQ-SCORE-API-UBI-NNN`)로 적용한다(research.md §8, §4.2).

- **REQ-SCORE-API-UBI-001 (데이터 주권)**: The score HTTP API layer SHALL NOT make any external service call (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) on any request path (GET 단건/목록/롤업/등급, POST 생성, PUT 수정, POST 정정). 모든 영속·계산은 SPEC-AX-SCORE-001 store(단일 내부 PostgreSQL pgx pool)에만 위임한다(`tech.md` §9.1 망분리 정합, research.md §8.1/§8.4). API 계층은 신규 외부 의존을 도입하지 않는다(외부 import 0).
- **REQ-SCORE-API-UBI-002 (감사 가능성 — store 전담, API 중복 audit 금지)**: For every mutation request (POST create / PUT update / POST supersede), the API layer SHALL rely exclusively on the SPEC-AX-SCORE-001 store layer to write exactly one `audit_logs` entry within the same database transaction as the score change (`InsertScore`/`UpdateScore`/`SupersedeAndReplaceScore` 내부 `RecordScore*`, research.md §3.1/§4.2), AND SHALL NOT itself INSERT any `audit_logs` row (이중 감사 금지 — consumer-only §1.4 HARD). GET 요청은 mutation이 아니므로 audit를 발생시키지 않는다.
- **REQ-SCORE-API-UBI-003 (cli-anonymous 기본값 + auth-disabled fallback)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the API layer SHALL serve all endpoints with ABAC/RBAC middleware transparently passing through (`abac.go:8` REQ-ABAC-009 정합), and the resulting score/audit rows SHALL carry `created_by`/`user_id = 'cli-anonymous'` as provided by the SPEC-AX-SCORE-001 store layer (API 계층은 실 사용자 식별자를 위조·누출하지 않음, research.md §7).
- **REQ-SCORE-API-UBI-004 (권한·불변)**: The API layer SHALL deny any write request (POST/PUT/supersede) from a principal lacking write authority via the SPEC-AX-AUTH-003 ABAC narrowing path with HTTP 403 (`ErrCodeABACDenied`, `abac.go:24`), AND SHALL surface a CONFIRMED-immutability violation reported by the store (`ErrScoreImmutable` on PUT to a CONFIRMED row, `ErrScoreNotConfirmed` on supersede of a non-CONFIRMED row) as HTTP 409 Conflict — never as 200/500 (research.md §3.1/§9.1). 미인가·불변 위반은 클라이언트 오류이며 INFO 로그로 기록된다.

### 3.2 REQ-SCORE-API-001 — 점수 조회 API (Read Endpoints)

조회 엔드포인트 그룹: GET 단건 / GET 목록(filter+pagination) / GET 가중 롤업 / GET 등급. 모두 read-only, `viewer` 포함 모든 인증 사용자 허용(§1.5).

#### Event-driven

- **REQ-SCORE-API-001-E1**: WHEN a caller issues `GET /api/v1/scores/{id}` with a well-formed UUID path segment for an existing score, THEN the API SHALL invoke `ScoreTx.GetScoreByID`, serialize the `store.Score`(id, evaluation_item_id, evidence_id?, level, score_value, weight?, grade?, status, metadata, created_at/by, updated_at — store.go:217-230) as `Content-Type: application/json`, and return `200 OK`.
- **REQ-SCORE-API-001-E2**: WHEN a caller issues `GET /api/v1/scores?evaluation_item_id={id}` (optionally `&level=&status=&offset=&limit=`), THEN the API SHALL invoke `ScoreTx.GetScoresByEvaluationItem`, apply the `level`/`status` filters and `offset`/`limit` pagination at the handler layer, and return `200 OK` with `{"scores":[...],"count":N}` (빈 결과는 `{"scores":[],"count":0}` — 빈 배열, NULL 금지).
- **REQ-SCORE-API-001-E3**: WHEN a caller issues `GET /api/v1/scores/rollup?evaluation_item_id={id}`, THEN the API SHALL invoke `ScoreTx.SumWeightedByEvaluationItem`, render the returned `pgtype.Numeric` as an exact decimal string (float64 미경유 — SEC-03 정밀도 보존, score.go:453-457), and return `200 OK` with `{"evaluation_item_id":"...","weighted_sum":"<decimal>"}`.
- **REQ-SCORE-API-001-E4**: WHEN a caller issues `GET /api/v1/scores/grade?scope={scope}&score={value}`, THEN the API SHALL invoke `ScoreTx.DetermineGrade(scope, score)` and return `200 OK` with `{"scope":"...","score":<value>,"grade":"<S|A|B|C|D>"}`.

#### State-driven

- **REQ-SCORE-API-001-S1**: WHILE the SPEC-AX-AUTH-003 ABAC/RBAC chain admits a `viewer`-only authenticated principal (or auth is disabled), the API SHALL serve all four read endpoints without denial (read-only 허용 — §1.5, research.md §4.1).

#### Optional

- **REQ-SCORE-API-001-O1**: WHERE the list request supplies `offset`/`limit` query parameters, the API SHALL clamp them deterministically — a missing/zero `limit` defaults to a fixed default, a `limit` above the configured maximum is clamped to the maximum, a negative/invalid `offset` is treated as 0 (workflow `grpc_server.go` `defaultListLimit`/`maxListLimit` clamping 선례, research.md §5/§9.3; 구체 default/max 값은 §6 OPEN #2/#3에서 strategy phase 확정).

#### Unwanted

- **REQ-SCORE-API-001-U1**: IF a read request references a non-existent score id (`GET /api/v1/scores/{id}` → store returns `ErrScoreNotFound`) OR supplies a malformed UUID path segment OR a non-numeric `score`/blank `scope` for grade OR a blank `evaluation_item_id` for list/rollup, THEN the API SHALL return `404 Not Found` (not-found) or `400 Bad Request` (validation) with the standard `{"error":{"code","message","field"}}` body, and SHALL NOT return `200` or `500` for these client errors (INFO 로그, research.md §6/§9.1).

### 3.3 REQ-SCORE-API-002 — 점수 변경 API (Mutation Endpoints)

변경 엔드포인트 그룹: POST 생성 / PUT 수정 / POST CONFIRMED 정정(supersede). 모두 write 권한 보유 principal 필요 — 정확한 write 역할 매핑은 §6 OPEN #4 미확정(§1.5). 감사는 store 계층 전담(REQ-SCORE-API-UBI-002).

#### Event-driven

- **REQ-SCORE-API-002-E1**: WHEN an authorized caller issues `POST /api/v1/scores` with a JSON body `{evaluation_item_id, evidence_id?, level, score_value, weight?, metadata?}`, THEN the API SHALL open `ScoreStore.BeginScoreTx`, call `ScoreTx.InsertScore(...)` (which records `SCORE_CREATED` audit in the same TX — score.go:130), `Commit`, and return `201 Created` with `{"score_id":"<uuid>","status":"DRAFT"}` (TX orchestration: `evidence_handlers.go:342-353` defer-Rollback 선례 미러, research.md §2.1).
- **REQ-SCORE-API-002-E2**: WHEN an authorized caller issues `PUT /api/v1/scores/{id}` with a JSON body of partial fields (`score_value?`, `weight?`, `grade?`, `status?`, `metadata?` — mapped to `store.ScoreUpdate`, store.go:235-246), THEN the API SHALL open a `ScoreTx`, call `UpdateScore(id, upd)` (records `SCORE_UPDATED` audit same TX — score.go:307), `Commit`, and return `200 OK` with `{"score_id":"<uuid>"}`.
- **REQ-SCORE-API-002-E3**: WHEN an authorized caller issues `POST /api/v1/scores/{id}/supersede` with a JSON body `{score_value, weight?, metadata?}` for a `CONFIRMED` score, THEN the API SHALL call `SupersedeAndReplaceScore(oldID, ...)` (new row INSERT + old row `CONFIRMED→SUPERSEDED`, each change one audit, same TX — score.go:332-391), `Commit`, and return `201 Created` with `{"score_id":"<new-uuid>","superseded_id":"<old-uuid>"}` (supersede REST shape는 §6 OPEN #1에서 strategy phase 확정 — research §9.4 권장 Option B).

#### State-driven

- **REQ-SCORE-API-002-S1**: WHILE the ABAC narrowing path admits only write-authorized principals (정확한 write 역할 매핑은 §6 OPEN #4 미확정), the API SHALL gate all three mutation endpoints behind that authority, denying others before opening any `ScoreTx` (ABAC decision before store TX — 불필요 DB 부하 0, research.md §9.2).

#### Unwanted

- **REQ-SCORE-API-002-U1**: IF a mutation request body fails validation (blank/over-64-char `evaluation_item_id`, non-numeric/absent `score_value`, `evidence_id` present but not a valid UUID, malformed JSON body, malformed UUID path segment), THEN the API SHALL return `400 Bad Request` with the standard error body, SHALL NOT open a `ScoreTx`, and SHALL NOT cause any `scores`/`audit_logs` write (pre-TX validation — `evidence_handlers.go:204-218` 선례, research.md §6/§9.1).

### 3.4 REQ-SCORE-API-003 — ABAC 통합 & 권한 narrowing (SPEC-AX-AUTH-003)

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 통합하며 그 코드를 수정하지 않는다(§1.4/§1.5 HARD).

#### Event-driven

- **REQ-SCORE-API-003-E1**: WHEN a write request (POST/PUT/supersede) is issued by a principal lacking write authority and authentication is enabled, THEN the SPEC-AX-AUTH-003 ABAC narrowing path (미들웨어 = `authz_middleware.go`/`RESTAuthzMiddleware`·`chain.go:17`, 거부 코드 상수 = `abac.go:24` `ErrCodeABACDenied`) SHALL deny it with HTTP `403` and error code `ABAC_CONDITION_DENIED` (distinct from RBAC 403), and the API SHALL NOT open a `ScoreTx` (research.md §4.1 미들웨어 경로 정합, §9.2).

#### State-driven

- **REQ-SCORE-API-003-S1**: WHILE `authEnabled=false` (Walking Skeleton 기본값), the ABAC/RBAC middleware (`authz_middleware.go`/`RESTAuthzMiddleware`, `chain.go:17`) SHALL pass through transparently and all 7 endpoints SHALL be served without authorization denial (`abac.go:8` REQ-ABAC-009, `server.go:261` 인가 미들웨어가 `s.cfg.AuthEnabled` 인자로 비활성 시 투과, research.md §4.1 미들웨어 경로 정합, §7).
- **REQ-SCORE-API-003-S2**: WHILE the requesting principal holds `RoleAdmin`, the ABAC evaluator SHALL bypass all ABAC conditions and admit the request (`abac.go:71` REQ-ABAC-004), so admin may invoke any of the 7 endpoints.

#### Unwanted

- **REQ-SCORE-API-003-U1**: IF a `viewer`-only principal attempts a write endpoint (POST/PUT/supersede) with authentication enabled, THEN the request SHALL be denied (403) by the SPEC-AX-AUTH-003 narrowing path before any store interaction, AND read endpoints SHALL remain permitted for the same principal (read-only 보장 — narrowing-only, §1.5).

### 3.5 REQ-SCORE-API-004 — store 에러→HTTP 매핑 & consumer-only 경계

본 SPEC은 SPEC-AX-SCORE-001 에러 센티넬을 HTTP status로 결정적으로 매핑하며, 비즈니스 로직·감사·스키마를 추가하지 않는다(§1.4 HARD).

#### State-driven

- **REQ-SCORE-API-004-S1**: WHILE handling any store error, the API SHALL map SPEC-AX-SCORE-001 sentinels deterministically via `errors.Is` (errors.go:54-78 검증, `ErrGradeThresholdsUnavailable`는 `errors.go:69` source-verified): `ErrScoreNotFound`→`404`, `ErrScoreInvalidInput`→`400`, `ErrScoreImmutable`→`409`, `ErrScoreInvalidStatus`→`409`, `ErrScoreNotConfirmed`→`409`, `ErrGradeThresholdsUnavailable`→`404`, unwrapped/unknown DB error→`500` (`evidence_handlers.go` status 매핑 패턴, research.md §6). **`ErrGradeThresholdsUnavailable`→`404` provenance [D2-3]**: 이는 **핸들러 설계 결정**이다 — 요청된 scope에 grade thresholds가 미설정이면 해당 등급 자원이 부재하므로 `404 Not Found`로 표면화한다(fail-closed 정합 — store가 등급을 fabricate하지 않고 에러를 반환하는 SCORE-001 §3.4 REQ-SCORE-003-S1 거동을 클라이언트에 자원-부재로 노출). 근거 출처는 `errors.go:69`(센티넬 실재) + 본 설계 결정이며, **research.md §11(grade 행 200/400/500만 기재, 404 없음)을 인용 근거로 삼지 않는다**.

#### Unwanted

- **REQ-SCORE-API-004-U1**: IF a `ScoreTx` is opened but a downstream call fails before commit, THEN the API SHALL ensure `tx.Rollback(ctx)` runs via a `committed`-flag defer (no partial commit, `evidence_handlers.go:342-353` 선례), SHALL return the mapped HTTP status, and SHALL NOT leak goroutines beyond the request scope (research.md §9.2).
- **REQ-SCORE-API-004-U2 (consumer-only 경계)**: IF implementation would require modifying any file under `internal/store/`, `internal/audit/`, `internal/auth/`, `internal/errors/`, `cmd/server/evidence_handlers.go`, or `.moai/db/schema/**`, THEN that change is OUT OF SCOPE and the API SHALL instead be redesigned to consume the existing contract unchanged (consumer-only §1.4 HARD; SPEC-AX-SCORE-001/EVID-001/AUTH-003 코드·스키마·FK·마이그레이션 0 diff — AC-SCORE-API-BOUNDARY-1로 검증).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 데이터 주권 (망분리) | 7 엔드포인트 경로의 외부 API 호출 0건. 단일 내부망 PostgreSQL pgx pool(store 위임)만 사용 | §3.1 REQ-SCORE-API-UBI-001, research.md §8.1/§8.4 |
| 감사 가능성 (store 전담) | 모든 mutation → store 동일 TX `audit_logs` 1건. API 자체 audit INSERT 0건 (이중 감사 금지) | §3.1 REQ-SCORE-API-UBI-002, research.md §4.2 |
| consumer-only 무변경 | `internal/store|audit|auth|errors`, `evidence_handlers.go`, `.moai/db/schema/**` 0 diff. 신규 마이그레이션 0 | §1.4 HARD, §2.3 Drift-Guard |
| 한국어 | 모든 에러 메시지 한국어 (`evidence_handlers.go:209-215` 선례) | research.md §8.2 |
| ABAC narrowing | viewer=read-only, 미인가 write→403, auth-disabled→투과, admin→투과 | §3.4, research.md §4.1 |
| CONFIRMED 불변 | PUT(CONFIRMED)→409, supersede(non-CONFIRMED)→409 (store 센티넬 매핑) | §3.5 REQ-SCORE-API-004-S1, research.md §9.1 |
| 페이지네이션 경계 | limit 누락→default, limit>max→clamp, offset 음수→0 (구체값 §6 OPEN #2/#3) | §3.2 REQ-SCORE-API-001-O1, research.md §9.3 |
| 정밀도 보존 | rollup `pgtype.Numeric`를 float64 미경유 정확 십진 문자열로 직렬화 | §3.2 REQ-SCORE-API-001-E3, score.go:453-457 |
| 성능 — 단건/생성 | p99 < 50ms (단일 store 호출 + 직렬화, 한국 공공 시간 제약) | §3.2/§3.3, research.md §8.6 |
| TX orchestration | mutation은 BeginScoreTx→ops→Commit, defer Rollback(committed flag) | §3.3, research.md §2.1 |
| pgx pool 재사용 | `ScoreStore.BeginScoreTx`(→`PgWorkflowStore.pool`)만. `postgres.go` 死 스텁 비대상 | §1.4 HARD, research.md §1 |
| 로깅 | 구조화 JSON 로그(zap), 검증/미인가 거부는 INFO, 서버 결함은 ERROR | research.md §6, `tech.md` §8.2 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR), harness: thorough, sub-agent mode | `quality.yaml` development_mode |
| Go 도구 | go vet, golangci-lint (default + gosec), goimports | `.claude/rules/moai/languages/go.md` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **DB 스키마·FK·마이그레이션 변경** — 본 SPEC은 순수 API 계층이므로 `scores`/`grade_thresholds` 테이블 스키마 변경, `scores.evaluation_item_id`/`scores.evidence_id` FK 하드닝, 신규 마이그레이션(`0004` 등) 추가/수정을 **하지 않는다**. 데이터 모델은 SPEC-AX-SCORE-001이 이미 제공했고 FK 하드닝은 미래 별도 SPEC 책임이다(SPEC-AX-SCORE-001 §5 #3 정합).
2. **API 자체 감사(own-audit)** — POST/PUT/supersede의 `audit_logs` 기록은 SPEC-AX-SCORE-001 store 메서드(`RecordScore*`)가 동일 TX로 전담한다. 본 SPEC은 API 핸들러에서 별도 audit row를 INSERT하지 않으며 audit 스키마·Recorder를 변경하지 않는다(REQ-SCORE-API-UBI-002, research.md §4.2).
3. **풀 등급기준(scoring rubric) 시스템** — 점수→letter 매핑은 SPEC-AX-SCORE-001 `DetermineGrade`(최소 `grade_thresholds` 테이블)를 호출만 한다. 풀 rubric 규칙 엔진·가점/감점·계층·rubric CRUD API는 본 SPEC 범위 밖이다(SPEC-AX-SCORE-001 §5 #2, SPEC-AX-EVAL-ITEM-001 §5 #2 이연 영역 유지).
4. **6번째 시간 제약(KST 업무시간 09:00–18:00 검증)** — 한국 공공 6제약 중 시간 제약은 SPEC-AX-AUTH-003 §Out-of-scope와 동일하게 본 SPEC 범위 밖이다(research.md §8.6). 본 SPEC은 데이터 주권/한국어/감사 가능성/망분리/조직 격리(5제약)만 API 계층에서 보장한다.
5. **AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC** — 본 SPEC은 SPEC-AX-AUTH-003이 제공하는 ABAC narrowing(`OrgUnitFromUser` scope 인코딩, admin 우회, narrowing-only)을 **호출만** 한다. 정교한 org-unit 행 레벨 필터링·다중 속성 정책 엔진·RBAC `permissionMatrix`에 score Permission 추가는 본 SPEC 범위 밖이며 AUTH-003 모델 확장은 미래 별도 SPEC 책임이다(§1.5 [중요 경계], research.md §10.6).
6. **SPEC-AX-SCORE-001 / EVID-001 / AUTH-003 코드 변경** — 위 세 SPEC의 store/audit/auth 코드, DB 스키마, FK, 마이그레이션을 본 SPEC 구현 중 일절 수정하지 않는다(consumer-only §1.4 HARD, REQ-SCORE-API-004-U2). `permissionMatrix`/`Authorize`/`rbac.go` frozen.
7. **Console UI / 클라이언트 SDK / OpenAPI 생성** — `apps/console/` 화면, 클라이언트 라이브러리, OpenAPI/Swagger 스펙 자동 생성은 본 SPEC 범위 밖이다. 본 SPEC은 server-side HTTP 핸들러 + 라우트 마운트만 다룬다.
8. **마이그레이션 도구·신규 외부 의존** — 신규 외부 라이브러리/SDK/마이그레이션 러너 도입 0건. 기존 `net/http`·`encoding/json`·`github.com/google/uuid`·`go.uber.org/zap`·`pgtype`만 사용(망분리, REQ-SCORE-API-UBI-001).

---

## 6. 의존성 및 전제 (RESOLVED — Human Gate sign-off 2026-05-19)

> §6.1~§6.4 = Human Gate sign-off(2026-05-19)로 RESOLVED 완료. 확정 = strategy.md §A SSOT. consumer-only [HARD] 0-diff·신규 마이그레이션 0 불변.

### 6.0 GREEN 전제 (검증 완료 — research.md file:line)

- **SPEC-AX-SCORE-001 완료(v0.1.3) GREEN**: `ScoreStore`/`ScoreTx`(store.go:253-292), `PgScoreTx` 7 메서드(score.go:110-648), 에러 센티넬(errors.go:54-78), `RecordScore*` 동일-TX audit(score.go:130/307/364), `0004_score_tables.sql` 2테이블 — 모두 source-verified, phantom API 0건.
- **[조정 노트 — D2-1] `BeginScoreTx`는 SCORE-001 v0.1.3에서 실구현·커밋 완료**: orchestrator ground-truth grep(SCORE-001 커밋 79595d4 기준)으로 `func (s *PgWorkflowStore) BeginScoreTx(ctx) (ScoreTx, error)`가 `pg_store.go:134`에 실재하고 인터페이스가 `store.go:255`에 실재함을 확정했다(SCORE-001 v0.1.3 evaluator PASS 85.25에서 구현·커밋). 따라서 본 SPEC의 "BeginScoreTx GREEN" 전제는 **사실상 참**이다. research.md §2.1 L134/§12의 "BeginScoreTx 미구현/(VERIFY)" 표현은 SCORE-001 v0.1.3 완료(`pg_store.go:134`, `store.go:255`)로 **superseded(stale)**이며, 본 SPEC은 그 stale 표현이 아닌 ground-truth(실구현)를 권위로 삼는다. consumer-only 전제가 깨지지 않도록 Run 진입 hard-verify 게이트를 plan.md §4 S0에 명문화했다(아래 §6.0 마지막 항목 참조).
- **SPEC-AX-EVID-001 완료(v0.1.2) 선례**: `evidence_handlers.go` 핸들러 구조·`Routes()`·표준 에러·TX orchestration(evidence_handlers.go:82-377), `server.go:257` 라우트 마운트 패턴.
- **SPEC-AX-AUTH-003 완료 GREEN [D2-2 2-파일]**: 거부 코드·상수·평가자 = `abac.go`(`ErrCodeABACDenied` abac.go:24, `ABACEvaluator`, narrowing-only), 미들웨어 와이어링 = `authz_middleware.go`의 `RESTAuthzMiddleware` + `chain.go:17`; admin 우회·auth-disabled 투과, `server.go:261` innerMux 전체 인가 와이어링. research.md §4.1(`authz_middleware.go`/`RESTAuthzMiddleware`)과 본 spec.md(`abac.go` 상수)는 동일 서브시스템의 두 실재 파일 — phantom 아님(§1.5 reconciliation 노트).
- **Cross-SPEC artifact 영향 없음**: 본 SPEC은 위 SPEC들의 골든 파일·generated artifact·코드를 수정하지 않는다(clean additive — API 파일 2개 신규 + server.go 라우트 1줄).
- **[방어 게이트 — D2-1] Run 진입 hard-verify**: consumer-only 전제(BeginScoreTx 등 소비 시그니처 실재)를 보호하기 위해, Run 진입 시 `grep BeginScoreTx apps/control-plane/internal/store/pg_store.go apps/control-plane/internal/store/store.go` 및 핵심 센티넬(`grep ErrGradeThresholdsUnavailable apps/control-plane/internal/errors/errors.go`)을 hard-verify한다. **미존재 시 consumer-only 불가 → 즉시 재계획**(plan.md §4 S0 명문 게이트). 본 게이트는 ground-truth stale risk(research.md §2.1/§12 표현이 SCORE-001 진척에 따라 낙후)를 흡수하는 방어 장치이다.
- **[D2-1 검증 완료 2026-05-19]** S0 hard-verify 게이트 사전 검증 완료: `BeginScoreTx`@`pg_store.go:134`+`store.go:255`, `ErrGradeThresholdsUnavailable`@`errors.go:69` 실재 확정(SCORE-001 커밋 79595d4). Run 진입 시 T-001이 재확인 — 방어 장치 유지.

### 6.1 OPEN #1 [RESOLVED: Option B]

`POST /api/v1/scores/{id}/supersede` body `{score_value, weight?, metadata?}` → `201` `{score_id:"<new>", superseded_id:"<old>"}`.
**근거**: `SupersedeAndReplaceScore`(store.go:285-291)는 새 행 INSERT + 구 행 `CONFIRMED→SUPERSEDED`의 non-idempotent 복합 연산이며 매 호출 새 UUID 자원을 생성 → 201; EVID-001 append-only POST 선례; ServeMux 최장일치로 `/{id}`와 무충돌.
**거부**: A(PUT+flag)=idempotent 의미론 위반·의도 불명확; C(PATCH status)=복합 연산·새 score_value 표현 불가.
**consumer-only**: `store.SupersedeAndReplaceScore` 호출만, 0-diff. 상세: strategy.md §A Decision #1.

### 6.2 OPEN #2 [RESOLVED: max=500, default=50]

`maxListLimit=500`, `defaultListLimit=50`. clamp: limit 누락/0→50, limit>500→500, offset 음수/비수치→0.
**근거**: workflow는 store가 limit/offset을 DB 전달하나 score는 `GetScoresByEvaluationItem`(store.go:278) 미지원 → 핸들러 메모리 슬라이싱(메커니즘 상이로 1000 답습 근거 약화); p99<50ms NFR(§4) 보호상 500. research §9.3.
**거부**: max 1000(workflow 답습) — 메커니즘 상이·NFR 과대.
**consumer-only**: `clampPagination` 상수 2개(score_handlers.go 내부), store/마이그레이션 0-diff. 상세: strategy.md §A Decision #2.

### 6.3 OPEN #3 [RESOLVED: offset/limit]

offset/limit. 핸들러가 `GetScoresByEvaluationItem` 전체 결과를 메모리 `[offset:offset+limit]` 슬라이싱.
**근거**: store.go:278은 offset/limit/cursor/정렬키 미지원 → cursor는 over-engineering(R-API-010); workflow 선례 정합.
**거부**: cursor — SPEC §5 post-PoC 이연; store 시그니처 확장=consumer-only 위반.
**consumer-only**: `GetScoresByEvaluationItem` 호출만 + 핸들러 슬라이싱, 0-diff. 상세: strategy.md §A Decision #3.

### 6.4 OPEN #4 [RESOLVED: (a) 핸들러-로컬 + write={RoleAdmin, RoleAnalyst}]

적용 지점 = (a) 핸들러-로컬 역할→동작 매핑. write = {RoleAdmin, RoleAnalyst}, viewer = read-only. org_unit 행-필터링 = 비적용.
**🔑 `evaluator` INFEASIBLE (사용자 승인 2026-05-19)**: 원지정 "evaluator/admin=write" — `rbac.go:33` `^iroum-ax:(admin|analyst|viewer)$`에 `evaluator` 부재 + `permissionMatrix`(rbac.go:39-60) score Perm 0 + rbac.go consumer-only 수정 불가 → 구현 불가. 충실 대체 = `RoleAnalyst`(rbac.go가 인식하는 유일한 비-admin·비-viewer 역할). write={RoleAdmin,RoleAnalyst}, read-only=RoleViewer.
**근거**: OBS-001 `metrics/permission.go` domain-local registry 직접 이식 — `score_handlers.go`가 `auth.UserFromContext`+`auth.ParseRolesFromScope`로 roles 추출 → viewer-only mutation→403 `ErrCodeABACDenied`(abac.go:24 호출만). abac.go:11 narrowing-only 동형(auth-disabled 투과·admin 우회 보존). org_unit 비적용 3중: store.Score 컬럼 부재 + ABAC 정책 활성화=consumer-only 위반 + §5 #5 제외.
**거부**: (b) ABAC 정책 주입 — internal/auth/* or server.go 범위 초과=consumer-only/Drift-Guard 위반; (c) 순수 RBAC matrix — frozen 위반(503 fail-closed), 통찰만 (a) 흡수.
**consumer-only**: rbac.go/abac.go/authz_middleware.go/chain.go **0-diff**. write-role 판정 = score_handlers.go domain-local 헬퍼 `requireScoreWriteRole`. AC-SCORE-API-BOUNDARY-1 만족. 상세: strategy.md §A Decision #4.

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- **SPEC-AX-SCORE-001 store/audit 로직**: 점수 데이터 모델·가중 롤업 계산·등급 threshold·동일-TX 감사는 SPEC-AX-SCORE-001이 이미 GREEN으로 제공한다. 본 SPEC은 HTTP로 노출만 하며 그 로직·스키마를 재구현/수정하지 않는다.
- **SPEC-AX-AUTH-003 RBAC `permissionMatrix`**: score Permission을 `rbac.go`에 추가하거나 `Authorize`를 수정하는 작업은 frozen RBAC 변경으로 consumer-only [HARD] 위반이다. write 역할 게이팅은 §6 OPEN #4에서 ABAC narrowing 또는 핸들러-로컬 매핑으로 해소한다.
- **DB 스키마/FK/마이그레이션**: `scores`/`grade_thresholds` 스키마, FK 하드닝, 신규 `.sql` 추가는 본 SPEC 범위 밖(§5 #1).
- **풀 rubric / LLM 등급 시뮬레이션 / Recommendation 엔진**: SPEC-AX-SCORE-001 §5 #2/#5 이연 영역 유지. 본 SPEC 등급 조회는 `DetermineGrade` 호출만.
- **Console UI / SDK / OpenAPI 생성**: server-side 핸들러만(§5 #7).
- **6번째 시간 제약(KST 업무시간)**: AUTH-003 정합 — 본 SPEC 범위 밖(§5 #4).

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/cmd/server/score_handlers_test.go` — `httptest.NewRequest`/`ResponseRecorder`, 7 엔드포인트 정상/에러 경로, 테이블 테스트, testify/assert, t.Parallel, goleak
- store 모킹: `ScoreStore`/`ScoreTx` 인터페이스 fake(테스트 격리, R-API store 미수정 정합) — `evidence_handlers.go` `evidenceRecorder` 인터페이스 격리 선례 미러
- 데이터 주권 검증: 7 엔드포인트 경로에서 외부 네트워크 egress 0건 (코드 정적 검사 — 신규 외부 import 0)
- 감사 비-중복 검증: mutation 핸들러가 자체 `audit_logs` INSERT를 하지 않고 store 위임만 함을 검증 (API 코드에 audit INSERT SQL 0건)
- ABAC narrowing 검증: viewer write→403 ABAC_CONDITION_DENIED / viewer read→200 / authEnabled=false→전 엔드포인트 투과 / admin→투과
- store 에러→HTTP 매핑 검증: `ErrScoreNotFound`→404 / `ErrScoreInvalidInput`→400 / `ErrScoreImmutable`·`ErrScoreInvalidStatus`·`ErrScoreNotConfirmed`→409 / `ErrGradeThresholdsUnavailable`→404(핸들러 설계 결정 — scope 미설정 = 자원 부재, errors.go:69 센티넬 실재 + 설계 근거; research §11 비인용) / unknown→500 (errors.go:54-78 정합)
- TX orchestration 검증: mutation 실패 시 defer Rollback(committed flag)로 부분 커밋 0, goroutine 누출 0 (goleak)
- pagination clamp 검증: limit 누락→default, limit>max→max clamp, offset 음수→0 (결정적)
- 정밀도 검증: rollup 응답이 `pgtype.Numeric`를 float64 미경유 정확 십진 문자열로 직렬화
- empty list 검증: 결과 0건 시 `{"scores":[],"count":0}` (NULL/누락 금지)
- consumer-only 경계 검증: `internal/store|audit|auth|errors`, `evidence_handlers.go`, `.moai/db/schema/**` 0 diff (AC-SCORE-API-BOUNDARY-1 — git diff/Drift-Guard manifest)
- 회귀: 기존 `evidence_handlers.go`·workflow REST 핸들러 테스트가 score 라우트 마운트 후에도 GREEN 유지

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.

---

## 9. Definition of Done (SPEC 단계)

- [ ] frontmatter 8-field canonical (plan.md L378) 준수, HISTORY + Schema note 포함
- [ ] EARS 5개 REQ 모듈(UBI 묶음 4 + 4 modal: 001 조회/002 변경/003 ABAC/004 에러매핑·경계) 모두 E/S/O/U 분류 명시, 모듈 ≤5
- [ ] §5 Exclusions ≥1 (DB/스키마/FK·신규마이그레이션, own-audit, 풀 rubric, 6번째 시간제약, AUTH-003 모델 초과 ABAC, SCORE-001/EVID-001/AUTH-003 코드변경, Console/SDK, 외부의존 — 8항목)
- [ ] §1.4 consumer-only 계약: store/audit/auth/errors/evidence_handlers.go/migrations **0 diff**, 신규 마이그레이션 0, 자체 audit 0 — §1/§2.3/§3.5-U2/§7에 명시
- [ ] §1.5 ABAC 권한 경계 + RBAC evaluator 부재 충돌 surface (§6 OPEN #4 연결)
- [ ] §6 OPEN 4건 (supersede shape / pagination max / offset-cursor / ABAC 적용지점·write 역할 매핑) — strategy phase RESOLVED 대상으로 명시
- [ ] §2 [DELTA]: [NEW] score_handlers.go/_test.go, [MODIFY] server.go(라우트 마운트만), [EXISTING] consumer 무변경 + Drift-Guard manifest
- [ ] acceptance.md 각 REQ ≥2 G/W/T, AC 명명 `AC-SCORE-API-{REQ}-{N}`, AC 27/§7 edge 16 count가 acceptance.md §9 · spec.md §9 · spec-compact.md 3문서 cross-file 일관 (plan.md §8은 plan-phase DoD로 count 미운반 — D3-1)
- [ ] spec.md에 함수명/클래스 구조/API 스키마 상세 구현 미기재 (WHAT/WHY only — store 시그니처는 소비 계약 검증 근거로만 인용)
- [ ] 구현 코드/테스트 미작성 (SPEC 문서만)
