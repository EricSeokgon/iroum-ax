---
id: SPEC-AX-REPORT-001
version: 0.1.0
status: draft
created: 2026-05-19
updated: 2026-05-19
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-19): 평가 결과 리포트/집계 HTTP API 계층(Evaluation Result Report/Aggregation HTTP API Layer) 첫 초안. SPEC-AX-SCORE-001(완료, v0.1.3)의 `ScoreStore`/`ScoreTx` 점수 store 계층 + SPEC-AX-EVAL-ITEM-001(완료)의 `EvalItemStore`/`EvalItemTx` taxonomy 계층 위에 **읽기 전용 리포트 HTTP API 계층만** 추가한다(SPEC-AX-SCORE-API-001 `score_handlers.go` 핸들러·라우팅 선례를 **read-only 부분집합으로** 미러링 — research.md §3). 핵심 기능: **범주(category) 롤업** — 평가범주 id를 받아 그 자식 평가항목(item)들을 EVAL-ITEM 계층에서 열거 → 각 item의 가중합(SCORE-001 `SumWeightedByEvaluationItem`)을 누적 → 범주 가중 총합 + 범주 등급(`DetermineGrade` 재사용)을 on-the-fly 산출(스냅샷 영속 없음). SPEC-AX-AUTH-003 경량 ABAC narrowing 통합 — **읽기 전용이므로 `viewer` 포함 모든 인증 사용자 허용, write 권한 게이팅 불필요(mutation 엔드포인트 0)**; cli-anonymous 기본값 + auth-disabled Walking Skeleton 투과. 한국 공공 6제약(데이터 주권/한국어/감사 가능성/망분리/조직 격리/시간 제약) 준수. **본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-EVAL-ITEM-001 / SPEC-AX-AUTH-003의 순수 consumer이며 그 코드·스키마·FK·마이그레이션·audit을 일절 변경하지 않는다 — DB 변경 0, 신규 마이그레이션 0(읽기 전용 API 계층), 자체 audit 0(읽기 전용이므로 mutation 0 → audit 이벤트 0), 신규 외부 의존 0**. 풀 rubric 시스템, 6번째 시간 제약(KST 업무시간), 스냅샷 영속화, mutation(write) 엔드포인트, AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC은 의도적 제외(§5 Exclusions). research.md(Phase 0.5 deep research, 674줄, file:line 근거)가 SSOT. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-EVAL-ITEM-001 / SPEC-AX-AUTH-003과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2 (L378)의 8-field canonical 정의(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 등 canonical 외 필드는 사용하지 않는다. 본 SPEC의 모든 EARS 요구사항·영향파일·HTTP 계약은 `.moai/specs/SPEC-AX-REPORT-001/research.md`(file:line 근거)에 근거하며, 소비 계약 시그니처는 `apps/control-plane/internal/store/store.go`(`ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx`/`Score`/`EvalItem`), `apps/control-plane/internal/store/score.go`(`PgScoreTx` 메서드), `apps/control-plane/internal/store/pg_store.go`(`BeginScoreTx`/`BeginEvalItemTx` 진입점), `apps/control-plane/internal/errors/errors.go`(에러 센티넬), `apps/control-plane/cmd/server/score_handlers.go`(read-only 핸들러 선례), `apps/control-plane/internal/auth/abac.go`·`rbac.go`·`middleware.go`(ABAC/RBAC), `apps/control-plane/cmd/server/server.go`(라우트 마운트), `apps/control-plane/go.mod`(외부 의존 인벤토리)에서 직접 검증되었다(phantom API 0건 — manager-spec orchestrator ground-truth grep, 2026-05-19).

---

# SPEC-AX-REPORT-001 — 평가 결과 리포트/집계 HTTP API 계층 (Evaluation Result Report/Aggregation HTTP API Layer)

## 1. 개요

경영평가팀(및 후속 Console/외부 클라이언트)이 SPEC-AX-SCORE-001이 완성한 점수 store + SPEC-AX-EVAL-ITEM-001이 완성한 평가항목 taxonomy를 결합하여 **평가범주(category) 단위의 집계 리포트**를 HTTP로 조회할 수 있도록, `apps/control-plane/cmd/server/`(Go 1.25.0, `go.mod:3` 명시)에 **읽기 전용 리포트 HTTP API 계층**을 추가한다. 본 SPEC은 SPEC-AX-SCORE-API-001의 점수 핸들러(`score_handlers.go` — `ScoreHandler` struct·`Routes()`·표준 JSON/에러 헬퍼·store 에러→HTTP 매핑·ABAC 게이트)를 정확히 미러링하되 **read-only 부분집합**으로 한정한다(research.md §3, `score_handlers.go:43-137` source-verified). 본 SPEC은 **mutation 엔드포인트를 0개** 노출하므로 `score_handlers.go:161-187`의 write-role 게이트(`requireScoreWriteRole`/`guardScoreWrite`)는 **차용하지 않는다**(읽기 전용 — §1.5). 리포트는 매 요청마다 store를 조회하여 on-the-fly로 산출하며 어떤 집계 결과도 영속화하지 않는다(스냅샷 0 — §5 #4).

### 1.1 리포트 API 계층의 의미 (본 SPEC 범위)

본 SPEC의 1차 산출물은 **SPEC-AX-SCORE-001 + SPEC-AX-EVAL-ITEM-001 store 메서드를 결합하여 범주별 집계 리포트를 HTTP로 노출하는 최소 read-only REST API 계층 + ABAC read-narrowing 통합**이다. 점수 데이터 모델·가중합 계산·등급 threshold·평가항목 계층(adjacency list)은 SPEC-AX-SCORE-001 / SPEC-AX-EVAL-ITEM-001이 이미 GREEN(완료)으로 제공한다 — 본 SPEC은 그 **consumer**이며 신규 비즈니스 로직·DB 스키마·마이그레이션·audit을 추가하지 않는다. 집계 자체는 **핸들러가 store 호출을 조합(compose)**하여 수행한다(Option A, 신규 store 메서드 0 — §6 OPEN #1).

- 신규 파일 2개: `cmd/server/report_handlers.go`(`ReportHandler` + `Routes()` + 리포트 핸들러 메서드 + 표준 JSON/에러 헬퍼 + store 에러→HTTP 매핑), `cmd/server/report_handlers_test.go`(테스트)
- 기존 1개 수정: `cmd/server/server.go` — **라우트 마운트 최소 단위(필드+생성자+innerMux.Handle 2줄, ≈7줄) + 핸들러 인스턴스화만** (`score_handlers.go` → `server.go:55/209/263-264` 선례 정확 미러, research.md §5)
- 리포트 엔드포인트(최소 surface — 구체 경로/형식은 §6 OPEN #3): 범주별 집계 리포트 조회(GET, read-only). mutation 엔드포인트 0
- 범주 롤업: 핸들러가 `EvalItemTx.GetEvalItemsByParentID`(자식 열거) + `ScoreTx.SumWeightedByEvaluationItem`(자식별 가중합) ×N + `ScoreTx.DetermineGrade`(범주 등급) 조합 — Option A consumer-only(§6 OPEN #1, research.md §2.2/§14.1/Appendix)
- ABAC 통합: SPEC-AX-AUTH-003 경량 ABAC narrowing-only — **읽기 전용이므로 `viewer` 포함 모든 인증 사용자 허용**(read-only GET, write 게이팅 불필요 — mutation 0); auth-disabled 투과; admin 우회
- cli-anonymous 기본값 + auth-disabled Walking Skeleton fallback (SPEC-AX-SCORE-001 §1.1, AUTH-003 정합 — store 계층 처리)
- 표준 에러: `score_handlers.go` `{"error":{"code","message","field"}}` 동일 스키마(`score_handlers.go:74-101` 선례, research.md §3.2)

### 1.2 Anchor 컨텍스트

본 SPEC은 SPEC-AX-SCORE-001 + SPEC-AX-EVAL-ITEM-001이 형성한 평가 흐름(지표별 raw score → EVAL-ITEM weight 가중 롤업 → 항목 → **범주** → 등급 threshold)의 **최상위 집계 단계(범주 롤업·등급 요약)를 HTTP로 외부에 노출**하여 `product.md` §3.2 기획재정부 경영평가 편람의 범주별 채점 결과를 클라이언트가 조회할 수 있게 한다(research.md §1.1/§1.2, `product.md:160` 4계층 모델). PoC 범위는 "안전보건" 범주의 집계 리포트 조회 API이며, SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-EVAL-ITEM-001 / SPEC-AX-AUTH-003 / SPEC-AX-CTRL-001은 GREEN(완료) 상태로 가정한다.

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `REPORT` (Evaluation Result Report sub-domain — SCORE-001/EVAL-ITEM-001 집계 결과의 리포트 API 계층)
- 따라서 SPEC ID: `SPEC-AX-REPORT-001` (`.claude/skills/moai/workflows/plan.md` Composite domain rules "Maximum 2 domains recommended, maximum 3 allowed" 권장 범위 내 — `AX` + `REPORT`)

### 1.4 의존성 stub 계약 — consumer-only [핵심, load-bearing]

본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-EVAL-ITEM-001 / SPEC-AX-AUTH-003의 **순수 consumer**이다. 다음이 **HARD 계약**이다(research.md §12, §7, 메모리 lesson #9 phantom 회피):

- **[HARD]** 본 SPEC은 `internal/store/score.go`·`store.go`·`pg_store.go`(`ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx`/`Score`/`EvalItem`), 평가항목 store 구현(`eval_item.go` 등 `EvalItemTx` 구현), `internal/audit/*`(`RecordScore*`), `internal/auth/*`(ABAC/RBAC), `internal/errors/*`(센티넬), `cmd/server/score_handlers.go`·`evidence_handlers.go`(핸들러 선례), `.moai/db/schema/migrations/*.sql`을 **일절 수정하지 않는다**. 점수/평가항목 비즈니스 로직·감사·접근제어·DB 스키마는 호출만 한다.
- **[HARD]** `scores.evaluation_item_id`(VARCHAR(64))·`evaluation_items.id`(VARCHAR(64) 계층코드, `store.go:147` source-verified)는 SPEC-AX-SCORE-001 §1.4 / SPEC-AX-EVAL-ITEM-001이 확립한 stub/모델로 유지된다. 본 SPEC은 리포트 요청에서 category id를 받아 store에 그대로 전달하며, 참조 무결성 검증(EVAL-ITEM/EVID 존재 여부의 FK 강제)은 **수행하지 않는다**(상위 로직 책임 — research.md §2.3/§12).
- **[HARD]** 신규 DB 마이그레이션 0건 — 본 SPEC은 읽기 전용 API 계층이므로 `scores`/`grade_thresholds`/`evaluation_items` 등 어떤 마이그레이션도 추가/수정하지 않는다. 모든 테이블은 SPEC-AX-SCORE-001 / SPEC-AX-EVAL-ITEM-001이 이미 생성했다(research.md §12).
- **[HARD]** 자체 audit 0건 — 본 SPEC의 모든 엔드포인트는 **read-only(GET)**이므로 mutation(INSERT/UPDATE/DELETE)이 발생하지 않으며 따라서 `audit_logs` 신규 행이 0건이다(research.md §6.1/§6.3). 점수 생성·수정 시점의 감사는 이미 SPEC-AX-SCORE-001 store(`RecordScore*` 동일 TX)가 기록했다. 리포트 API 핸들러가 별도 audit row를 INSERT하면 부당한 감사가 되므로 금지된다(REQ-REPORT-UBI-002).
- **[HARD]** `postgres.go`는 Sprint-0 死 스텁(SPEC-AX-SCORE-001 plan.md §2)이며 본 SPEC 대상 아님. TX 진입점은 `store.ScoreStore.BeginScoreTx`(→ `pg_store.go:134`) 및 `store.EvalItemStore.BeginEvalItemTx`(→ `pg_store.go:118`)만 사용한다(research.md §1, source-verified: `PgWorkflowStore`가 두 인터페이스 모두 구현).
- **[HARD]** 신규 외부 의존 0건 — 본 SPEC은 어떤 신규 라이브러리/SDK도 `go.mod`에 추가하지 않는다. 정밀도 누적용 `shopspring/decimal`은 **`go.mod`·`go.sum` 양쪽 0건 부재**임이 orchestrator ground-truth grep으로 독립 확정되었다(source-verified 명제 = `shopspring/decimal` 부재). 따라서 도입 금지 대상이다(§6 OPEN #2 — 범주 누적은 신규 외부 의존 없이 표준 라이브러리 `math/big` 또는 기존 `pgtype` 산술로 해소). 핵심 불변식은 "신규 직접 의존 0 + `shopspring/decimal` 부재"이지 "go.mod에 특정 N개만 존재"가 아니다(go.mod는 16개 직접 의존을 보유 — 그 목록을 3개로 축소 열거하지 않는다).

### 1.5 ABAC 권한 경계 [핵심] — 읽기 전용 → write 게이팅 불필요

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 통합한다. 확립된 사실(research.md §4, `abac.go`/`rbac.go`/`middleware.go` source-verified):

- **읽기 전용 정책**: 본 SPEC의 모든 엔드포인트가 read-only(GET 집계 리포트 조회)이므로 `viewer` 역할 포함 **모든 인증 사용자**가 리포트를 조회할 수 있다(SCORE-API-001의 GET 엔드포인트와 동일 read-only 정책 — research.md §4.2). **mutation 엔드포인트가 0개이므로 SPEC-AX-SCORE-API-001 §1.5의 "write 권한 역할 매핑 / `evaluator` 역할 부재 충돌(SCORE-API-001 §6 OPEN #4)"은 본 SPEC에 적용되지 않는다** — write-role 게이트(`score_handlers.go:161-187` `requireScoreWriteRole`/`guardScoreWrite`)를 차용하지 않는다.
- **auth-disabled 투과**: `authEnabled=false`(Walking Skeleton 기본값, SPEC-AX-SCORE-001 §1.1 정합)일 때 ABAC/RBAC 미들웨어가 자동 투과(`abac.go:8` REQ-ABAC-009 source-verified), `cli-anonymous` 기록은 store 계층이 처리한다. 본 SPEC은 인증 비활성에서도 동작한다.
- **admin 우회**: `RoleAdmin` 보유 principal은 모든 ABAC 조건을 우회한다(`abac.go:9` REQ-ABAC-004 source-verified).
- **narrowing-only**: ABAC는 RBAC가 통과시킨 요청만 추가 거부할 수 있고 권한을 부여하지 않는다(`abac.go:4` source-verified). 읽기 전용 본 SPEC은 ABAC가 거부할 write 동작이 없으므로 narrowing 경로상 추가 거부 없이 모든 인증 사용자에게 read를 허용한다.

> **[경계 노트]** `internal/auth/rbac.go`의 RBAC 역할은 `RoleAdmin`/`RoleAnalyst`/`RoleViewer`만 존재하며(`rbac.go:20-25`, scope 정규식 `^iroum-ax:(admin|analyst|viewer)$` — `rbac.go:33` source-verified), `permissionMatrix`(`rbac.go:39-`)에 report/score Permission이 부재하다. consumer-only [HARD] 제약상 본 SPEC은 frozen RBAC을 수정할 수 없다. 그러나 본 SPEC은 **읽기 전용**이므로 write 역할 매핑이 필요 없고 `viewer` 포함 모든 인증 사용자에게 read를 허용하면 충분하다 — 따라서 SCORE-API-001을 괴롭힌 "write 역할 매핑 OPEN"이 본 SPEC에는 발생하지 않는다(설계상 우월점). 본 SPEC은 불변인 ABAC read 거동(auth-disabled → 투과, viewer 인증 read → 허용, admin → 우회)만 EARS로 고정한다.

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` `apps/control-plane/` 트리를 따른다. 본 SPEC은 store/audit/auth/score-API 코드는 일절 수정하지 않고(consumer-only §1.4 HARD), 리포트 API 계층 파일만 추가한다. Delta 마커: [EXISTING]=consumer로 호출만(무변경), [NEW]=신규 추가, [MODIFY]=라우트 마운트만.

### 2.1 Go Control Plane 리포트 API 계층 (`apps/control-plane/cmd/server/`)

| 경로 | 책임 | Delta | 모듈 |
|------|------|-------|------|
| `apps/control-plane/cmd/server/report_handlers.go` | `ReportHandler` struct + `NewReportHandler(...)` + `Routes() http.Handler` + 리포트 핸들러 메서드(범주별 집계 리포트 조회 — GET) + 범주 롤업 조합 로직(EvalItem 자식 열거 → 자식별 SumWeighted 누적 → 범주 등급) + 표준 JSON/에러 헬퍼(`score_handlers.go:74-101` 선례 미러). store 에러 센티넬→HTTP status 매핑(`score_handlers.go:111-137` 선례). **write-role 게이트 미차용(읽기 전용 §1.5)**. | [NEW] | REQ-REPORT-001~003 |
| `apps/control-plane/cmd/server/report_handlers_test.go` | `httptest` 기반 핸들러 단위 테스트 — 리포트 정상/에러 경로, 범주 미존재 404, 빈 범주(자식 0) empty report, 점수 0 item, grade 미설정 처리, numeric 정밀도(float64 미경유), auth-disabled 투과, viewer read 허용, admin 우회, cross-store 2-TX rollback, consumer-only 경계(API audit 0·git 0-diff). store는 fake `ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx`로 격리. | [NEW] | 전체 |
| `apps/control-plane/cmd/server/server.go` | **라우트 마운트만**(≈7줄 최소 단위): `reportH` 필드(`server.go:55` `scoreH` 선례 위치) + `s.reportH = NewReportHandler(pgStore, pgStore, logger)` 생성자(`server.go:209` `NewScoreHandler` 선례 위치 — `pgStore`가 `ScoreStore`+`EvalItemStore` 동시 구현, `pg_store.go:118/134` source-verified) + `innerMux.Handle` **2줄**(`server.go:263-264` `/api/v1/scores` 선례 정확 미러: `/api/v1/reports` + `/api/v1/reports/` 서브트리 — Go1.22 ServeMux path-param 라우팅 구조적 필수) + ko 주석. ABAC은 기존 미들웨어 체인(`server.go:261` 와이어링)이 innerMux 전체를 감싸 자동 적용 — ABAC 와이어링 변경 **0-diff**. | [MODIFY] | REQ-REPORT-002 |

### 2.2 소비 계약 — 호출만, 무변경 (consumer-only [HARD] §1.4)

| 경로 | 소비 계약 | Delta |
|------|----------|-------|
| `apps/control-plane/internal/store/store.go` | `ScoreStore.BeginScoreTx`(`store.go:254-255`), `ScoreTx`(`GetScoreByID` store.go:275-276 / `GetScoresByEvaluationItem` store.go:277-278 / `SumWeightedByEvaluationItem` store.go:292-295 / `DetermineGrade` store.go:296-298 / `Commit`/`Rollback`), `EvalItemStore.BeginEvalItemTx`(`store.go:115-118`), `EvalItemTx`(`GetEvalItemByID` store.go:197-199 / `GetEvalItemsByParentID` store.go:200-202 / `Commit`/`Rollback`), `Score`/`EvalItem` struct(`store.go:125-152` EvalItem 필드 `ID string`/`ParentID *string`/`Level *int`/`Weight *float64`) — 호출만 (source-verified) | [EXISTING] |
| `apps/control-plane/internal/store/score.go` | `PgScoreTx` 구현 — 호출만 (`score.go:110-648` 검증). 점수 비즈니스 로직 무변경 | [EXISTING] |
| `apps/control-plane/internal/store/pg_store.go` | `PgWorkflowStore.BeginScoreTx`(`pg_store.go:134`) + `PgWorkflowStore.BeginEvalItemTx`(`pg_store.go:118`) — 단일 pgStore가 두 store 인터페이스 동시 구현(source-verified), 호출만 | [EXISTING] |
| `apps/control-plane/internal/store/{eval_item 구현}` | `EvalItemTx` 구현(`GetEvalItemByID`/`GetEvalItemsByParentID` 등) — 호출만, 평가항목 taxonomy 로직 무변경 | [EXISTING] |
| `apps/control-plane/internal/errors/errors.go` | 에러 센티넬 `ErrScoreNotFound`/`ErrGradeThresholdsUnavailable` + 평가항목 not-found 센티넬 — `errors.Is`로 매핑만 (research.md §8.1) | [EXISTING] |
| `apps/control-plane/cmd/server/score_handlers.go` | `ScoreHandler` 구조·`Routes()`·`writeScoreJSON`/`writeScoreErr`·`mapStoreErr` 선례(`score_handlers.go:43-137` 검증) — read-only 부분집합 패턴 미러만(코드 무변경). write-role 게이트(`score_handlers.go:161-187`) **미차용** | [EXISTING] |
| `apps/control-plane/cmd/server/evidence_handlers.go` | 핸들러 구조·표준 에러 패턴 2차 선례 — 패턴 미러만(코드 무변경) | [EXISTING] |
| `apps/control-plane/internal/auth/abac.go`, `rbac.go`, `middleware.go` | `ErrCodeABACDenied`(abac.go:24), `ABACEvaluator`(narrowing-only/admin 우회/auth-disabled 투과 abac.go:4/8/9), `RoleViewer`/`RoleAdmin`(rbac.go:20-25), `UserFromContext`(middleware.go:44-49), `User` struct(middleware.go:25) — 호출만/미들웨어 체인 자동 적용. **permissionMatrix/Authorize frozen — 무변경 [HARD]** | [EXISTING] |
| `.moai/db/schema/migrations/*.sql` | SPEC-AX-SCORE-001/EVAL-ITEM-001이 생성한 `scores`/`grade_thresholds`/`evaluation_items` 테이블 — 본 SPEC은 마이그레이션 추가/수정 0건 (읽기 전용 API §1.4 HARD) | [EXISTING] |
| `apps/control-plane/go.mod` | 기존 16개 직접 의존 무변경 — 신규 의존 추가 0건, `shopspring/decimal` 부재(`go.mod`·`go.sum` 0건 독립 확정, §1.4 HARD) | [EXISTING] |

### 2.3 Drift-Guard Manifest

[NEW]만 신규 생성, [MODIFY]는 정확히 1파일·라우트 마운트 범위, [EXISTING]은 0 diff. 구현 중 [EXISTING] 파일에 1줄이라도 수정 발생 시 consumer-only [HARD] 위반 → 즉시 중단·재계획(plan.md §7 R-RPT-002).

- 신규 생성 허용: `cmd/server/report_handlers.go`, `cmd/server/report_handlers_test.go`
- 수정 허용(라우트 마운트 한정, ≈7줄): `cmd/server/server.go` (`reportH` 필드 + `NewReportHandler(...)` 호출 + `innerMux.Handle("/api/v1/reports", ...)` + `innerMux.Handle("/api/v1/reports/", ...)` 2줄 + ko 주석)
- 수정 절대 금지(0 diff 검증): `internal/store/*.go`, `internal/audit/*.go`, `internal/auth/*.go`, `internal/errors/*.go`, `cmd/server/score_handlers.go`, `cmd/server/evidence_handlers.go`, `.moai/db/schema/**`, `go.mod`/`go.sum`(신규 외부 의존 0)

---

## 3. EARS 요구사항

### 3.1 Ubiquitous (시스템 전반 불변 조건)

Ubiquitous 요구사항은 SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 / SPEC-AX-EVAL-ITEM-001의 canonical `REQ-{DOMAIN}-UBI-NNN (한글 제목)` dual-track 패턴을 도메인 스코프 형태(`REQ-REPORT-UBI-NNN`)로 적용한다(research.md §13.2, §6, §7).

- **REQ-REPORT-UBI-001 (데이터 주권)**: The report HTTP API layer SHALL NOT make any external service call (외부 LLM API, 외부 SaaS, 외부 CDN, 외부 secrets manager) on any request path. 모든 영속·계산·집계는 SPEC-AX-SCORE-001 / SPEC-AX-EVAL-ITEM-001 store(단일 내부 PostgreSQL pgx pool)에만 위임하며, 범주 롤업 누적 산술은 신규 외부 의존 없이 표준 라이브러리/`pgtype`로 수행한다(`tech.md` §9.1 망분리 정합, research.md §7.1; `shopspring/decimal` 부재 = `go.mod`·`go.sum` 0건 orchestrator ground-truth 독립 확정 — source-verified 명제).
- **REQ-REPORT-UBI-002 (감사 가능성 — read-only이므로 자체 audit 0, store 위임)**: For every request, the report API layer SHALL NOT itself INSERT any `audit_logs` row, AND SHALL rely on the SPEC-AX-SCORE-001 store layer which already recorded each score change in the same database transaction at score-creation/update time (`RecordScore*`, research.md §6.1/§6.2/§6.3). 본 SPEC의 모든 엔드포인트는 read-only이므로 mutation이 0건이고 따라서 audit 이벤트가 0건이다(읽기 감시는 기본 정책상 부재 — research.md §6.3).
- **REQ-REPORT-UBI-003 (cli-anonymous 기본값 + auth-disabled fallback)**: WHILE 인증이 비활성(`AUTH_ENABLED=false`, Walking Skeleton 기본값)인 동안, the report API layer SHALL serve all report endpoints with ABAC/RBAC middleware transparently passing through (`abac.go:8` REQ-ABAC-009 정합), AND SHALL NOT fabricate or leak any real user identifier — store가 부여한 `cli-anonymous` 기본값 계약을 그대로 따른다(research.md §7.2).
- **REQ-REPORT-UBI-004 (권한 — read-narrowing, viewer 허용)**: The report API layer SHALL permit any authenticated principal including `viewer` to read aggregation reports via the SPEC-AX-AUTH-003 ABAC narrowing path (read-only — write 권한 게이팅 불필요, mutation 엔드포인트 0), AND SHALL NOT introduce any write-role gate (`score_handlers.go:161-187` `requireScoreWriteRole`/`guardScoreWrite` 미차용 — §1.5). admin은 ABAC 우회(`abac.go:9` REQ-ABAC-004)로 모든 리포트를 조회할 수 있다.

### 3.2 REQ-REPORT-001 — 범주 집계 리포트 조회 API (Read Endpoints)

조회 엔드포인트 그룹: 범주별 집계 리포트(GET, read-only). 모두 read-only, `viewer` 포함 모든 인증 사용자 허용(§1.5). 핸들러가 `EvalItemTx`(자식 열거) + `ScoreTx`(자식별 가중합·범주 등급)를 조합하여 on-the-fly 산출(스냅샷 영속 0 — §5 #4). 구체 경로/응답 JSON 형식/페이지네이션은 §6 OPEN #3 미확정 — strategy phase 확정.

#### Event-driven

- **REQ-REPORT-001-E1**: WHEN a caller issues a category report request for an existing category id, THEN the API SHALL (1) open an `EvalItemTx` via `EvalItemStore.BeginEvalItemTx`, resolve the category via `GetEvalItemByID` (store.go:199) and enumerate its direct child items via `GetEvalItemsByParentID` (store.go:202), (2) open a `ScoreTx` via `ScoreStore.BeginScoreTx`, invoke `SumWeightedByEvaluationItem` (store.go:295) per child item accumulating an exact decimal total without float64 (research.md §10, score.go:453-457 SEC-03 정합), (3) invoke `DetermineGrade(scope, total)` (store.go:298) for the category grade, and return `200 OK` with a category report JSON (category id/name + per-item weighted sums + category total + category grade — 정확 형식 §6 OPEN #3).
- **REQ-REPORT-001-E2**: WHEN a caller requests a report for a category whose child enumeration (`GetEvalItemsByParentID`) returns an empty slice, OR a child item whose `SumWeightedByEvaluationItem` yields no contributing rows, THEN the API SHALL return `200 OK` with an empty/zero report (`items:[]` 또는 해당 item `weighted_sum:"0"`, category total `"0"`, grade null) rather than 404 or 500 (data-completeness 원칙 — research.md §14.5 권장 빈 리포트; 빈 리포트 vs 제외 최종 결정 §6 OPEN #3).

#### State-driven

- **REQ-REPORT-001-S1**: WHILE the SPEC-AX-AUTH-003 ABAC/RBAC chain admits a `viewer`-only authenticated principal (or auth is disabled), the API SHALL serve all report read endpoints without denial (read-only 허용 — §1.5, research.md §4.2).
- **REQ-REPORT-001-S2**: WHILE composing a category report, the API SHALL accumulate per-item `pgtype.Numeric` weighted sums using exact decimal arithmetic that never converts through Go `float64` (precision 보존 — SCORE-001 SEC-03, score.go:453-457; 누적 메커니즘 `pgtype`/`math/big` 표준 라이브러리, 신규 외부 의존 0 — §6 OPEN #2).

#### Optional

- **REQ-REPORT-001-O1**: WHERE a report list request supplies pagination/filter query parameters (예: 범주 목록 조회 시 `offset`/`limit` 또는 `min_grade`), the API SHALL clamp them deterministically at the handler layer (default/max 상수화 — `score_handlers.go` clamp 선례; pagination/filter 필요 여부 및 구체값은 §6 OPEN #3 strategy phase 확정).
  - **[D3-1 — O1 전용 AC 부재 정당화]** 본 Optional 요구사항은 §6 OPEN #3에서 "페이지네이션/필터 필요 여부 자체가 미확정"으로 게이트되어 있다. 따라서 acceptance.md에 O1 전용 AC를 두지 않는다 — 페이지네이션 적용 여부·default/max·필터 키가 OPEN #3 RESOLVED 후에야 단언 가능하기 때문이다(미확정 요구에 추측 AC 작성 금지 원칙). OPEN #3가 "페이지네이션 적용"으로 RESOLVED되면 그 시점에 O1 전용 AC(clamp 결정성)를 추가하고 AC/edge count를 3문서 재정합한다. OPEN #3가 "PoC 미적용(단건 `/reports/category/{id}`만)"으로 RESOLVED되면 O1은 비활성 Optional로 유지되며 별도 AC 불요.

#### Unwanted

- **REQ-REPORT-001-U1**: IF a report request references a non-existent category id (`GetEvalItemByID` → store returns not-found 센티넬) OR supplies a blank/over-64-char category id OR a malformed request, THEN the API SHALL return `404 Not Found` (not-found) or `400 Bad Request` (validation) with the standard `{"error":{"code","message","field"}}` body (한국어 메시지, `score_handlers.go:74-101` 선례), and SHALL NOT return `200` or `500` for these client errors (INFO 로그, research.md §8.2).

### 3.3 REQ-REPORT-002 — ABAC read-narrowing 통합 (SPEC-AX-AUTH-003, read-only)

본 SPEC은 SPEC-AX-AUTH-003 경량 ABAC narrowing-only 모델을 통합하며 그 코드를 수정하지 않는다(§1.4/§1.5 HARD). 본 SPEC은 mutation 엔드포인트가 0개이므로 write 거부 경로가 없다.

#### Event-driven

- **REQ-REPORT-002-E1**: WHEN any authenticated principal (`viewer`/`analyst`/`admin`) with authentication enabled issues a report read request, THEN the SPEC-AX-AUTH-003 ABAC narrowing path SHALL admit it (read-only — narrowing이 추가 거부할 write 동작 없음, `abac.go:4` source-verified), and the API SHALL serve the report.

#### State-driven

- **REQ-REPORT-002-S1**: WHILE `authEnabled=false` (Walking Skeleton 기본값), the ABAC/RBAC middleware SHALL pass through transparently and all report endpoints SHALL be served without authorization denial (`abac.go:8` REQ-ABAC-009, `server.go:261` 미들웨어 체인이 비활성 시 투과, research.md §4.1/§7).
- **REQ-REPORT-002-S2**: WHILE the requesting principal holds `RoleAdmin`, the ABAC evaluator SHALL bypass all ABAC conditions and admit the request (`abac.go:9` REQ-ABAC-004), so admin may invoke any report endpoint.

#### Unwanted

- **REQ-REPORT-002-U1**: IF the implementation would introduce any write-role gate, mutation endpoint, or principal-based denial of read access for an authenticated user, THEN that is OUT OF SCOPE — the report API is read-only and SHALL permit all authenticated principals to read (no write-role mapping, no `evaluator`-role dependency — §1.5; SCORE-API-001 §6 OPEN #4 충돌 본 SPEC 비적용).

### 3.4 REQ-REPORT-003 — store 에러→HTTP 매핑 & consumer-only 경계

본 SPEC은 SPEC-AX-SCORE-001 / SPEC-AX-EVAL-ITEM-001 에러 센티넬을 HTTP status로 결정적으로 매핑하며, 비즈니스 로직·감사·스키마·신규 store 메서드를 추가하지 않는다(§1.4 HARD).

#### State-driven

- **REQ-REPORT-003-S1**: WHILE handling any store error during report composition, the API SHALL map SPEC-AX-SCORE-001 / SPEC-AX-EVAL-ITEM-001 sentinels deterministically via `errors.Is` (`score_handlers.go:111-137` `mapStoreErr` 선례 미러): category not-found(EVAL-ITEM not-found 센티넬)→`404`, `ErrGradeThresholdsUnavailable`→리포트 등급 필드 `null`(빈 리포트 원칙 — fail-closed 정합, store가 등급을 fabricate하지 않음) 또는 `404`(자원 부재 — 최종 표면화 정책 §6 OPEN #3), invalid input→`400`, unwrapped/unknown DB error→`500` (research.md §8.1/§8.2). 등급 미설정 처리(`null` vs `404`)는 §6 OPEN #3에서 strategy phase 확정한다.

#### Unwanted

- **REQ-REPORT-003-U1**: IF an `EvalItemTx` and/or `ScoreTx` is opened for read but a downstream call fails, THEN the API SHALL ensure each opened read transaction is released via a `defer tx.Rollback(ctx)` (read-only — Commit 불필요; cross-store 2-TX 모두 정리), SHALL return the mapped HTTP status, and SHALL NOT leak goroutines beyond the request scope (research.md §3.3 read-only TX 정합 — `score_handlers.go` defer-Rollback 선례).
- **REQ-REPORT-003-U2 (consumer-only 경계)**: IF implementation would require modifying any file under `internal/store/`, `internal/audit/`, `internal/auth/`, `internal/errors/`, `cmd/server/score_handlers.go`, `cmd/server/evidence_handlers.go`, `.moai/db/schema/**`, or adding a new external dependency to `go.mod` (incl. `shopspring/decimal`) or a new store method (e.g. `SumWeightedByCategory`), THEN that change is OUT OF SCOPE and the API SHALL instead be redesigned to compose the existing contract unchanged (consumer-only §1.4 HARD; Option A 핸들러 조합 — research.md §2.2/§14.1; AC-REPORT-BOUNDARY-1로 검증).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 데이터 주권 (망분리) | 리포트 경로의 외부 API 호출 0건. 단일 내부망 PostgreSQL pgx pool(store 위임)만 사용. 신규 외부 의존 0 (`shopspring/decimal` 부재 — `go.mod`·`go.sum` 0건) | §3.1 REQ-REPORT-UBI-001, research.md §7.1, orchestrator ground-truth grep (shopspring/decimal 0건 독립 확정) |
| 감사 가능성 (read-only) | read-only이므로 mutation 0 → API 자체 audit INSERT 0건. 점수 audit는 SCORE-001 store가 생성 시점에 이미 기록 | §3.1 REQ-REPORT-UBI-002, research.md §6.1/§6.3 |
| consumer-only 무변경 | `internal/store|audit|auth|errors`, `score_handlers.go`, `evidence_handlers.go`, `.moai/db/schema/**`, `go.mod` 0 diff. 신규 마이그레이션 0, 신규 store 메서드 0 | §1.4 HARD, §2.3 Drift-Guard |
| 한국어 | 모든 에러 메시지 한국어 (`score_handlers.go` 선례) | research.md §8.2 |
| ABAC read-narrowing | viewer 포함 모든 인증 사용자 read 허용, auth-disabled→투과, admin→우회, write 게이트 0 | §3.3, research.md §4.2 |
| 정밀도 보존 | 범주 롤업 누적이 `pgtype.Numeric`/`math/big` 표준 산술로 float64 미경유, 정확 십진 직렬화 | §3.2 REQ-REPORT-001-S2, score.go:453-457, §6 OPEN #2 |
| 빈/누락 데이터 | 자식 0 범주·점수 0 item → 빈/0 리포트(NULL/누락 금지, data-completeness) | §3.2 REQ-REPORT-001-E2, research.md §14.5 |
| cross-store TX | 읽기용 `EvalItemTx`+`ScoreTx` 각각 defer Rollback(Commit 불필요), goroutine 누출 0 | §3.4 REQ-REPORT-003-U1, research.md §3.3 |
| pgx pool 재사용 | `ScoreStore.BeginScoreTx`+`EvalItemStore.BeginEvalItemTx`(→`PgWorkflowStore.pool`)만. `postgres.go` 死 스텁 비대상 | §1.4 HARD, pg_store.go:118/134 source-verified |
| 성능 — 범주 리포트 | p99 < 100ms 목표(자식 수 N의 SumWeighted N+2 호출 — 한국 공공 시간 제약, research.md §8.6 정합) | §3.2, research.md §8 |
| 로깅 | 구조화 JSON 로그(zap), 검증/not-found 거부는 INFO, 서버 결함은 ERROR | research.md §8.2, `tech.md` §8.2 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (RED-GREEN-REFACTOR), harness: thorough, sub-agent mode | `quality.yaml` development_mode |
| Go 도구 | go vet, golangci-lint (default + gosec), goimports | `.claude/rules/moai/languages/go.md` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC 또는 후속 Phase에서 다룬다.

1. **DB 스키마·FK·마이그레이션 변경** — 본 SPEC은 읽기 전용 API 계층이므로 `scores`/`grade_thresholds`/`evaluation_items` 스키마 변경, FK 하드닝, 신규 마이그레이션 추가/수정을 **하지 않는다**. 데이터 모델은 SPEC-AX-SCORE-001 / SPEC-AX-EVAL-ITEM-001이 이미 제공했다(SPEC-AX-SCORE-API-001 §5 #1 정합).
2. **API 자체 감사(own-audit)** — 본 SPEC은 read-only이므로 mutation 0 → audit 이벤트 0. 점수 변경 시점의 `audit_logs` 기록은 SPEC-AX-SCORE-001 store(`RecordScore*`)가 이미 동일 TX로 기록했다. 리포트 API는 별도 audit row를 INSERT하지 않으며 audit 스키마·Recorder를 변경하지 않는다(REQ-REPORT-UBI-002, research.md §6).
3. **mutation(write) 엔드포인트** — 점수 생성/수정/정정(supersede) 등 변경 API는 본 SPEC 범위 밖이며 SPEC-AX-SCORE-API-001이 이미 제공한다. 본 SPEC은 집계 리포트 조회(GET)만 노출하고 write-role 게이트(`score_handlers.go:161-187`)를 차용하지 않는다(§1.5).
4. **집계 결과 스냅샷 영속화** — 범주 롤업 결과를 테이블/캐시/파일에 영속하는 스냅샷·머티리얼라이즈드 뷰·리포트 이력 저장은 본 SPEC 범위 밖이다. 본 SPEC은 매 요청 store 조회로 on-the-fly 산출만 한다(research.md §6.1 read-only 정합).
5. **신규 store 메서드 / 신규 외부 의존** — SPEC-AX-SCORE-001에 `SumWeightedByCategory` 등 category-level 집계 메서드를 추가하는 것(research.md §2.2 Option B)은 consumer-only [HARD] 위반이므로 제외한다. 정밀도 누적용 `shopspring/decimal` 등 신규 라이브러리 도입도 제외(망분리·§6 OPEN #2 — 표준 `math/big`/`pgtype`로 해소). 본 SPEC은 핸들러 조합(Option A)만 사용한다.
6. **풀 등급기준(scoring rubric) 시스템** — 점수→letter 매핑은 SPEC-AX-SCORE-001 `DetermineGrade`(최소 `grade_thresholds` 테이블)를 호출만 한다. 풀 rubric 규칙 엔진·가점/감점·계층·rubric CRUD API는 본 SPEC 범위 밖이다(SPEC-AX-SCORE-001 §5 #2, SPEC-AX-SCORE-API-001 §5 #3 이연 영역 유지).
7. **6번째 시간 제약(KST 업무시간 09:00–18:00 검증)** — 한국 공공 6제약 중 시간 제약은 SPEC-AX-AUTH-003 / SPEC-AX-SCORE-API-001 §5 #4와 동일하게 본 SPEC 범위 밖이다(research.md §13.1). 본 SPEC은 데이터 주권/한국어/감사 가능성/망분리/조직 격리(5제약)만 API 계층에서 보장한다.
8. **AUTH-003 모델을 넘는 풀 org-unit 속성 ABAC** — 본 SPEC은 SPEC-AX-AUTH-003이 제공하는 ABAC narrowing(admin 우회, narrowing-only, auth-disabled 투과)을 **호출만** 한다. 정교한 org-unit 행 레벨 필터링·다중 속성 정책 엔진·RBAC `permissionMatrix`에 report Permission 추가는 본 SPEC 범위 밖이며 AUTH-003 모델 확장은 미래 별도 SPEC 책임이다(§1.5 경계 노트, research.md §13.1).
9. **SPEC-AX-SCORE-001 / SCORE-API-001 / EVAL-ITEM-001 / AUTH-003 코드 변경** — 위 SPEC들의 store/audit/auth/score-API 코드, DB 스키마, FK, 마이그레이션을 본 SPEC 구현 중 일절 수정하지 않는다(consumer-only §1.4 HARD, REQ-REPORT-003-U2). `permissionMatrix`/`Authorize`/`rbac.go` frozen.
10. **Console UI / 클라이언트 SDK / OpenAPI 생성** — `apps/console/` 화면, 클라이언트 라이브러리, OpenAPI/Swagger 스펙 자동 생성은 본 SPEC 범위 밖이다. 본 SPEC은 server-side HTTP 핸들러 + 라우트 마운트만 다룬다.

---

## 6. 의존성 및 전제 (RESOLVED — Run Phase strategy.md §A + Human Gate sign-off 2026-05-19)

> §6.1~§6.3 = **RESOLVED** (Human Gate sign-off 2026-05-19, 권고안 전부 승인 — OPEN#3=B-2, §A.5 fallback 미발동). SPEC-AX-SCORE-API-001 §6 OPEN→RESOLVED 흐름과 동위상으로 strategy.md §A에서 결정·Human Gate sign-off로 확정했다. 결정 SSOT = `.moai/specs/SPEC-AX-REPORT-001/strategy.md` §A.1~§A.5. consumer-only [HARD] 0-diff·신규 마이그레이션 0·신규 store 메서드 0·신규 외부 의존 0·자체 audit 0은 결정과 무관하게 불변이다. §6.0 GREEN 전제·방어 게이트(S0 hard-verify)는 Run 진입 시에도 그대로 유지된다.

### 6.0 GREEN 전제 (검증 완료 — orchestrator ground-truth grep 2026-05-19)

- **SPEC-AX-SCORE-001 완료(v0.1.3) GREEN**: `ScoreStore.BeginScoreTx`(`store.go:254-255`), `ScoreTx.{GetScoreByID(store.go:275-276), GetScoresByEvaluationItem(store.go:277-278), SumWeightedByEvaluationItem(store.go:292-295), DetermineGrade(store.go:296-298)}`, `PgWorkflowStore.BeginScoreTx`(`pg_store.go:134`), `RecordScore*` 동일-TX audit(score.go:130/307/364) — 모두 source-verified, phantom 0건.
- **SPEC-AX-EVAL-ITEM-001 완료 GREEN [phantom-risk 해소]**: research.md §2.3/§14.1이 인용한 자식 열거 메서드 `GetEvalItemsByParentID`가 **실재**함을 orchestrator ground-truth grep으로 확정 — `store.go:200-202` `GetEvalItemsByParentID(ctx, parentID string) ([]*EvalItem, error)`, `store.go:197-199` `GetEvalItemByID(ctx, id string) (*EvalItem, error)`, 인터페이스 `EvalItemTx`(`store.go:182`), 진입점 `EvalItemStore.BeginEvalItemTx`(`store.go:115-118`) + `PgWorkflowStore.BeginEvalItemTx`(`pg_store.go:118`). `EvalItem` struct(`store.go:125-152`): `ID string`(VARCHAR(64) 계층코드), `ParentID *string`, `Level *int`, `Weight *float64`. **메모리 lesson #9 phantom-API 게이트 통과 — 가정 아닌 source-verified**.
- **SPEC-AX-SCORE-API-001 완료(v0.1.1) 선례**: `ScoreHandler` 구조·`Routes()`·`writeScoreJSON`/`writeScoreErr`/`scoreErrorBody`·`mapStoreErr`·`server.go:55/209/263-264` 라우트 마운트 패턴(`score_handlers.go:43-137`, `server.go` source-verified). write-role 게이트 `requireScoreWriteRole`/`guardScoreWrite`(`score_handlers.go:161-187`)는 본 SPEC 미차용(읽기 전용).
- **SPEC-AX-AUTH-003 완료 GREEN**: `ErrCodeABACDenied="ABAC_CONDITION_DENIED"`(abac.go:24), narrowing-only/admin 우회/auth-disabled 투과(abac.go:4/8/9), `RoleViewer`/`RoleAdmin`(rbac.go:20-25), 정규식 `^iroum-ax:(admin|analyst|viewer)$`(rbac.go:33), `UserFromContext`(middleware.go:44-49) — source-verified.
- **단일 pgStore가 두 store 동시 구현 [load-bearing]**: `PgWorkflowStore`가 `BeginScoreTx`(`pg_store.go:134`)와 `BeginEvalItemTx`(`pg_store.go:118`)를 모두 구현 → `NewReportHandler(pgStore, pgStore, logger)`로 양 store 주입 가능(server.go:209 `NewScoreHandler(pgStore, ...)` 패턴 정합). **현재 HTTP 계층에 EvalItem 핸들러 부재(`cmd/server/`에 evidence/score 핸들러만 존재) — 본 SPEC이 첫 `EvalItemStore` HTTP consumer**.
- **Cross-SPEC artifact 영향 없음**: 본 SPEC은 위 SPEC들의 골든 파일·generated artifact·코드를 수정하지 않는다(clean additive — API 파일 2개 신규 + server.go 라우트 ≈7줄).
- **[방어 게이트] Run 진입 hard-verify**: consumer-only 전제(소비 시그니처 실재)를 보호하기 위해, Run 진입 시 `grep -n 'GetEvalItemsByParentID\|GetEvalItemByID\|BeginEvalItemTx' apps/control-plane/internal/store/store.go apps/control-plane/internal/store/pg_store.go` 및 `grep -n 'SumWeightedByEvaluationItem\|DetermineGrade\|BeginScoreTx' apps/control-plane/internal/store/store.go apps/control-plane/internal/store/pg_store.go`를 hard-verify한다. **미존재/시그니처 불일치 시 consumer-only 불가 → 즉시 STOP·재계획**(plan.md §4 S0 명문 게이트, 메모리 lesson #9 흡수).

### 6.1 OPEN #1 → RESOLVED [CRITICAL — 범주 롤업 cross-store 2-TX 조합 (Option A)]

> **RESOLVED (Human Gate sign-off 2026-05-19)** — 결정 SSOT: strategy.md §A.1.

**결정**: 핸들러가 `EvalItemStore` + `ScoreStore` 두 store 의존을 보유하는 **cross-store 2-TX read 조합(Option A). 신규 store 메서드 0.**
- (a) `ReportHandler{ scoreStore store.ScoreStore; evalItemStore store.EvalItemStore; logger }`, `NewReportHandler(ss store.ScoreStore, eis store.EvalItemStore, logger)` → `server.go`에서 `NewReportHandler(pgStore, pgStore, logger)` (`PgWorkflowStore` 두 인터페이스 동시 구현 — `pg_store.go:118/134` source-verified, `server.go:209` `NewScoreHandler` 선례). 범주 1건당 2개 독립 read TX: (i) `BeginEvalItemTx`→`GetEvalItemByID`(범주 검증, not-found→404)+`GetEvalItemsByParentID`(직계 자식 열거)→`defer Rollback`, (ii) `BeginScoreTx`→자식별 `SumWeightedByEvaluationItem` 누적+`DetermineGrade`→`defer Rollback`. **Commit 없음**(read-only, `score_handlers.go:348/393` 선례 미러).
- (b) **계층 깊이: 1-level 직계 자식만**. `SumWeightedByEvaluationItem`이 `evaluation_item_id`로 raw-level 자식 행을 DB-side 집계(`store.go:292`)하므로 직계 자식(항목) id 호출이면 충분. 4-level 재귀(범주→항목→지표→배점) 미적용 — PoC scope·over-engineering 회피.
- (c) **블로커 해소**: S0 hard-verify로 시그니처 실재 확정(`store.go:197-202/115-118`, `pg_store.go:118,134`). 범주 식별은 path param category id를 `GetEvalItemByID`에 그대로 전달(not-found 센티넬이 미존재 결정적 표면화) — 별도 discriminator 불요. **잔여 가정**: "raw 점수가 직계 자식 항목 id에 적재"는 PoC 시드 데이터 의존 → RED 첫 테스트에서 fake `ScoreTx`로 항목레벨 경로 검증, 실데이터 계층 깊이 가정 위배 시 즉시 STOP·재계획(strategy.md §A.1(c), S0 게이트 동위상).

> **[제외 — Option B 불가, 확정]** SCORE-001에 `SumWeightedByCategory` 신규 메서드 추가는 consumer-only [HARD] 위반(§5 #5, REQ-REPORT-003-U2) → **REJECTED**. 단일 store/단일 TX/4-level 재귀도 구조적 불가 또는 over-engineering으로 거부(strategy.md §A.1).

### 6.2 OPEN #2 → RESOLVED [정밀도 누적 = `math/big.Rat` 표준 라이브러리, 신규 외부 의존 0]

> **RESOLVED (Human Gate sign-off 2026-05-19)** — 결정 SSOT: strategy.md §A.2.

**결정**: `pgtype.Numeric`의 exported 필드 `{Int *big.Int, Exp int32}`(numeric.go:52-57 source-verified)를 `math/big.Rat`로 무손실 변환하여 누적. **신규 외부 의존 0 (`math/big` = stdlib).**
- 각 자식 `SumWeightedByEvaluationItem` → `pgtype.Numeric` → `*big.Rat`(값 = `Int × 10^Exp`; `Exp≥0`→`SetInt(Int×10^Exp)`, `Exp<0`→`SetFrac(Int, 10^|Exp|)` — 분모가 10거듭제곱인 정확 유리수, `Valid==false`→0). accumulator `*big.Rat` `Add` 누적(유리수 누적 오차 0). 최종 `acc.FloatString(4)`(SCORE-001 `numeric(12,4)` 고정 scale) → 정확 십진 문자열 직렬화(`score_handlers.go:370` `"weighted_sum":decStr` 선례 정합, float64 미경유).
- **[D3-2 정합]** 누적은 `big.Rat` 보존, `DetermineGrade(scope, total float64)`(store.go:298) 입력만 `rat→Float64()` **1회** 변환. SEC-03 "float64 미경유" 범위는 N-항 누적 경로(big.Rat 보존)이지 누적 완료 후 등급 임계 비교 1회 입력이 아니다 — `score_handlers.go:382` handleGrade `strconv.ParseFloat` 동일 store 계약.

> **[거부 — 확정]** `shopspring/decimal`(go.mod/go.sum 0건 부재, research §14.4 권장은 ground-truth로 **superseded** — 신규 직접 의존 = §1.4 HARD/망분리 위반), `Float64Value()` 누적(SEC-03 위반), `pgtype.Numeric` 자체 산술(API 부재 — 코덱 타입), `GetScoresByEvaluationItem` 전 행 Go-side Σ(로직 중복·열등) 모두 REJECTED(strategy.md §A.2).

### 6.3 OPEN #3 → RESOLVED [단건 endpoint + 빈 리포트 + grade null graceful (B-2)]

> **RESOLVED (Human Gate sign-off 2026-05-19, B-2 채택, §A.5 fallback 미발동)** — 결정 SSOT: strategy.md §A.3.

**결정**:
- **(1) 응답 JSON** (research §14.2 채택): `{category_id, category_name(=EvalItem.DisplayName), items:[{item_id, item_name, weighted_sum}], category_total, category_grade, generated_at}`. `weighted_sum`/`category_total` = 십진 문자열(SEC-03, float64 미경유). `category_grade` = string | `null`.
- **(2) 엔드포인트: 단건 `GET /api/v1/reports/category/{id}` 1개만**. 목록 `GET /api/v1/reports` PoC 미적용(over-engineering 회피 — plan §7 R-RPT-010, §5 #10). `server.go` `innerMux.Handle` 2줄(`/api/v1/reports` + `/api/v1/reports/`) — `server.go:263-264` 선례 미러(Go1.22 ServeMux 최장일치, path-param 구조적 필수). `Routes()`는 1 라우트 등록.
- **(3) 페이지네이션/필터: PoC 미적용**. `REQ-REPORT-001-O1`은 D3-1대로 **비활성 Optional 유지, O1 전용 AC 미추가**(AC/edge count 21/13 불변). 향후 목록 endpoint 도입 시 `score_handlers.go:144` `clampPagination` 선례 재사용 — 미래 별도 SPEC.
- **(4) 빈/누락 표면화**: 범주 not-found(`GetEvalItemByID` 센티넬)→**404**; blank/>64자→**400**(pre-store, TX 미진입); 자식 0(`GetEvalItemsByParentID` 빈 슬라이스)/자식 점수 0→**200** `{items:[], category_total:"0", category_grade:null}`(data-completeness); **grade_thresholds scope 0건(`DetermineGrade`→`ErrGradeThresholdsUnavailable`)→200 + `category_grade:null` (B-2 graceful)**. REQ-REPORT-003-S1을 B-2로 확정.

> **[B-2 정당화]** REPORT는 집계 리포트로 등급은 부가 필드(`handleGrade` 단일 등급 조회와 본질 차이) — 정상 산출 점수를 등급 1개로 404 처리 시 정보 손실·범주부재(404)와 혼동. `null`은 fabricate 아님(거짓 등급 미생성 → `score_handlers.go:115` D2-3 "fabricate 금지"와 비충돌, research §14.5 data-completeness 정합). **구현(consumer-only 정합)**: REPORT 핸들러가 `DetermineGrade` 에러를 `mapStoreErr`에 넘기지 않고 핸들러 레벨 `errors.Is(err, apperrors.ErrGradeThresholdsUnavailable)` 분기로 `grade=null` 흡수, 그 외 store 에러만 `mapReportStoreErr` 미러 → `score_handlers.go` `mapStoreErr` 코드 0-diff. **[거부]** B-1(404 엄격 선례 미러): 집계 리포트에 부적합 → §A.5 fallback으로 보존하나 Human Gate 미발동(strategy.md §A.3/§A.5).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- **SPEC-AX-SCORE-001 store/audit 로직**: 점수 데이터 모델·가중 롤업 계산(item-level `SumWeightedByEvaluationItem`)·등급 threshold·동일-TX 감사는 SPEC-AX-SCORE-001이 이미 GREEN으로 제공한다. 본 SPEC은 범주 단위로 **조합·노출만** 하며 그 로직·스키마를 재구현/수정하지 않는다.
- **SPEC-AX-SCORE-API-001 mutation/write-role**: 점수 생성/수정/정정 및 write-role 게이트(`score_handlers.go:161-187`)는 SCORE-API-001이 제공한다. 본 SPEC은 read-only이므로 차용하지 않으며 SCORE-API-001 §6 OPEN #4(write 역할/`evaluator` 부재 충돌)는 본 SPEC에 발생하지 않는다(§1.5).
- **SPEC-AX-EVAL-ITEM-001 taxonomy CRUD**: 평가항목 생성/수정/계층 재배치는 EVAL-ITEM-001 책임. 본 SPEC은 `GetEvalItemByID`/`GetEvalItemsByParentID` 읽기만 호출한다.
- **DB 스키마/FK/마이그레이션**: `scores`/`grade_thresholds`/`evaluation_items` 스키마, FK 하드닝, 신규 `.sql` 추가는 본 SPEC 범위 밖(§5 #1).
- **풀 rubric / LLM 등급 시뮬레이션 / 스냅샷 이력**: SPEC-AX-SCORE-001 §5 이연 영역 유지. 본 SPEC 등급은 `DetermineGrade` 호출만, 결과 영속 0(§5 #4/#6).
- **Console UI / SDK / OpenAPI 생성**: server-side 핸들러만(§5 #10).
- **6번째 시간 제약(KST 업무시간)**: AUTH-003/SCORE-API-001 정합 — 본 SPEC 범위 밖(§5 #7).

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/cmd/server/report_handlers_test.go` — `httptest.NewRequest`/`httptest.NewRecorder`, 리포트 엔드포인트 정상/에러 경로, 테이블 테스트, testify/assert, t.Parallel, goleak
- store 모킹: `ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx` 인터페이스 fake(테스트 격리, consumer-only 정합) — `score_handlers.go` fake 격리 선례 미러
- 데이터 주권 검증: 리포트 경로 외부 네트워크 egress 0건 (코드 정적 검사 — 신규 외부 import 0, `shopspring/decimal` 부재 확인)
- 감사 비-발생 검증: read-only 핸들러가 `audit_logs` INSERT·`Recorder` 호출 0건 (API 코드에 audit SQL 0건, recorder 의존 미주입)
- ABAC read-narrowing 검증: viewer 인증 read→200 / authEnabled=false→전 엔드포인트 투과 / admin→우회 / write 게이트 부재(코드에 `requireScoreWriteRole` 미존재)
- 범주 롤업 검증: `GetEvalItemByID`+`GetEvalItemsByParentID`(EvalItemTx) → 자식별 `SumWeightedByEvaluationItem`+`DetermineGrade`(ScoreTx) cross-store 2-TX 조합 결과 정확성
- store 에러→HTTP 매핑 검증: category not-found→404 / `ErrGradeThresholdsUnavailable`→grade null 또는 404(§6 OPEN #3) / invalid→400 / unknown→500
- cross-store TX 검증: 읽기용 EvalItemTx+ScoreTx 각각 defer Rollback, 실패 시 부분 상태 0, goroutine 누출 0 (goleak)
- 정밀도 검증: 범주 누적이 `pgtype.Numeric`/`math/big` 표준 산술로 float64 미경유, 정확 십진 직렬화
- 빈/누락 검증: 자식 0 범주·점수 0 item → 빈/0 리포트(`items:[]`/`"0"`, NULL/누락 금지)
- consumer-only 경계 검증: `internal/store|audit|auth|errors`, `score_handlers.go`, `evidence_handlers.go`, `.moai/db/schema/**`, `go.mod`/`go.sum` 0 diff (AC-REPORT-BOUNDARY-1 — git diff/Drift-Guard manifest)
- 회귀: 기존 `score_handlers.go`·`evidence_handlers.go`·workflow REST 핸들러 테스트가 report 라우트 마운트 후에도 GREEN 유지

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.

---

## 9. Definition of Done (SPEC 단계)

- [ ] frontmatter 8-field canonical (plan.md L378) 준수, HISTORY + Schema note 포함
- [ ] EARS 4개 REQ 모듈(UBI 묶음 4 + 3 modal: 001 범주리포트 조회 / 002 ABAC read-narrowing / 003 에러매핑·경계) 모두 E/S/O/U 분류 명시, 모듈 ≤5 (read-only — mutation REQ 0)
- [ ] §5 Exclusions ≥1 (DB/스키마/FK·신규마이그레이션, own-audit, mutation 엔드포인트, 스냅샷 영속, 신규 store 메서드·신규 외부의존, 풀 rubric, 6번째 시간제약, AUTH-003 모델 초과 ABAC, SCORE-001/SCORE-API-001/EVAL-ITEM-001/AUTH-003 코드변경, Console/SDK — 10항목)
- [ ] §1.4 consumer-only 계약: store/audit/auth/errors/score_handlers.go/evidence_handlers.go/migrations/go.mod **0 diff**, 신규 마이그레이션 0, 신규 store 메서드 0, 신규 외부 의존 0, 자체 audit 0 — §1/§2.3/§3.4-U2/§7에 명시
- [ ] §1.5 ABAC read-narrowing 경계 — 읽기 전용이므로 write-role 매핑 불필요(SCORE-API-001 §6 OPEN #4 본 SPEC 비적용) 명시
- [ ] §6 OPEN 3건 (#1 [CRITICAL] 범주 롤업 cross-store 조합·child-enumeration 검증 / #2 정밀도 누적 산술·신규 외부의존 금지 / #3 응답형식·페이지네이션·빈데이터) — strategy phase + Human Gate RESOLVED 대상으로 명시. #1 phantom-risk 해소(GetEvalItemsByParentID source-verified) 명기
- [ ] §2 [DELTA]: [NEW] report_handlers.go/_test.go, [MODIFY] server.go(라우트 마운트만 ≈7줄), [EXISTING] consumer 무변경 + Drift-Guard manifest
- [ ] acceptance.md 각 REQ ≥2 G/W/T, AC 명명 `AC-REPORT-{REQ}-{N}`, AC 21/§7 edge 13 count가 acceptance.md §9 · spec.md §9 · spec-compact.md 3문서 cross-file 일관 (plan.md §8은 plan-phase DoD로 count 미운반 — SCORE-API-001 D3-1 정합)
- [ ] spec.md에 함수명/클래스 구조/API 스키마 상세 구현 미기재 (WHAT/WHY only — store 시그니처는 소비 계약 검증 근거로만 인용)
- [ ] phantom API 0 — 전 소비 시그니처 source-verified (store.go:115-298 / pg_store.go:118/134 / score_handlers.go:43-187 / abac.go:4-24 / rbac.go:20-33 / server.go:55/209/263-264 / go.mod) 명시 (메모리 lesson #9)
- [ ] 구현 코드/테스트 미작성 (SPEC 문서만)
