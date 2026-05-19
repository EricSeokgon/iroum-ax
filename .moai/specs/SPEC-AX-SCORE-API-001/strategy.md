# SPEC-AX-SCORE-API-001 — Run Phase 1 전략 분석 & 실행 계획

> 작성: manager-strategy · 2026-05-19 · 모드: sub-agent TDD · Harness: thorough
> 대상: SPEC-AX-SCORE-API-001 v0.1.0 (점수 조회/집계 HTTP API 계층) — iroum-ax Go control-plane brownfield
> Human Gate Decision Point 1 사인오프 **완료**(2026-05-19). §A §6 OPEN 4건 = RESOLVED.
> SCORE-001 strategy.md 구조 미러. 본 SPEC = SCORE-001/EVID-001/AUTH-003 순수 consumer — store/audit/auth/스키마 0 diff.

## 0. 소스 검증 요약 (phantom-API 방지 — 코드 실측, 세션 lesson #9)

| 검증 대상 | 결과 | 근거 (file:line) |
|-----------|------|------------------|
| `BeginScoreTx` 실재 (S0 게이트) | `func (s *PgWorkflowStore) BeginScoreTx(ctx) (ScoreTx, error)` ✓ | `pg_store.go:134` + `store.go:255` |
| score 에러 센티넬 7종 | `ErrScoreNotFound`/`ErrScoreInvalidInput`/`ErrScoreImmutable`/`ErrScoreInvalidStatus`/`ErrGradeThresholdsUnavailable`/`ErrScoreAuditWriteFailed`/`ErrScoreNotConfirmed` ✓ | `errors.go:54-78` (Grade=69) |
| `evaluator` 부재 (OPEN #4) | `roleRegex = ^iroum-ax:(admin\|analyst\|viewer)$` ✓ | `rbac.go:33` |
| `permissionMatrix` score Perm 0건 | admin/analyst/viewer = workflow/recommendation/audit만 ✓ | `rbac.go:39-60` |
| ABAC narrowing-only | "ABAC는 절대 allow 부여 안 함" ✓ | `abac.go:11`, `:65-89` |
| `DefaultABACPolicies()` 빈 | `return []ABACPolicy{}` ✓ | `abac.go:258-260` |
| `ErrCodeABACDenied` exported | `const ErrCodeABACDenied = "ABAC_CONDITION_DENIED"` ✓ | `abac.go:24` |
| server.go ABAC 와이어링 | `auth.ABACMiddleware(abacEvaluator, s.cfg.AuthEnabled)(innerMux)` ✓ | `server.go:261` |
| evidence 마운트 선례 | `innerMux.Handle("/api/v1/evidences", s.evidenceH.Routes())` ✓ | `server.go:257` |
| `GetScoresByEvaluationItem` offset/limit 미지원 | `(ctx, evaluationItemID string) ([]*Score, error)` ✓ | `store.go:278` |
| `SupersedeAndReplaceScore` non-idempotent | 새 행 INSERT + 구 행 CONFIRMED→SUPERSEDED, 매 호출 새 UUID ✓ | `store.go:285-291` |
| TX orchestration 선례 | `committed:=false; defer{ if !committed { tx.Rollback } }` ✓ | `evidence_handlers.go:348-353` |
| OBS-001 frozen-RBAC domain-local | `metrics/permission.go` `IsMetricsAuthorized` (RoleAdmin만), rbac.go 0-diff, research Option B | `SPEC-AX-OBS-001/spec.md:110,302,325` |

phantom 0건. 메커니즘 독립 재도출(앵커링 회피).

## A. §6 OPEN 4건 — RESOLVED (Human Gate sign-off 2026-05-19)

### Decision #4 [CRITICAL] — ABAC write-role & 적용 지점: (a) 핸들러-로컬 + write={RoleAdmin,RoleAnalyst} + org_unit 비적용

**RESOLVED**: 적용 지점 = (a) 핸들러-로컬 역할→동작 매핑. write = {RoleAdmin, RoleAnalyst}((c) analyst 재사용을 (a) 메커니즘으로). viewer = read-only. org_unit 행-필터링 = 비적용.

**🔑 `evaluator` INFEASIBLE (사용자 승인)**: 원지정 "evaluator/admin=write" — `rbac.go:33` `^iroum-ax:(admin|analyst|viewer)$`에 evaluator 부재 + permissionMatrix score Perm 0 + rbac.go consumer-only 수정 불가 → 구현 불가. 충실 대체 = RoleAnalyst. write={RoleAdmin,RoleAnalyst}, read-only=RoleViewer.

**근거**: OBS-001 `metrics/permission.go` domain-local registry 직접 이식 — score_handlers.go가 `auth.UserFromContext`+`auth.ParseRolesFromScope`로 roles 추출 → viewer-only mutation→403. abac.go:11 narrowing-only 동형(auth-disabled 투과·admin 우회 보존). `ErrCodeABACDenied`(abac.go:24) 호출만. org_unit 비적용 3중: store.Score 컬럼 부재 + ABAC 정책 활성화=consumer-only 위반 + SPEC §5 #5 제외.
**거부**: (b) ABAC 정책 주입 — internal/auth/* or server.go 범위 초과=consumer-only/Drift-Guard 위반(OBS-001도 동일 기각); (c) 순수 RBAC matrix 경유 — frozen 위반(503 fail-closed), 통찰만 (a) 흡수.
**consumer-only**: rbac.go/abac.go/authz_middleware.go/chain.go 0-diff. write-role 판정=score_handlers.go domain-local 헬퍼(`requireScoreWriteRole`). AC-SCORE-API-BOUNDARY-1 만족.

### Decision #1 — supersede REST shape: Option B

**RESOLVED**: `POST /api/v1/scores/{id}/supersede` body `{score_value,weight?,metadata?}` → 201 `{score_id:"<new>",superseded_id:"<old>"}`.
**근거**: `SupersedeAndReplaceScore`(store.go:285-291)=새 행 INSERT+구 행 전이, 매 호출 새 UUID=non-idempotent+새 자원→201; EVID-001 append-only POST 선례; ServeMux 최장일치로 `/{id}` 무충돌.
**거부**: A(PUT+flag)=idempotent 위반·의도 불명확; C(PATCH status)=복합 연산·새 score_value 표현 불가.
**consumer-only**: `store.SupersedeAndReplaceScore` 호출만, 0-diff.

### Decision #3 — offset vs cursor: offset/limit (#2 선행)

**RESOLVED**: offset/limit. 핸들러가 `GetScoresByEvaluationItem` 전체 결과 메모리 `[offset:offset+limit]` 슬라이싱.
**근거**: store.go:278 offset/limit/cursor/정렬키 미지원 → cursor=over-engineering(R-API-010); workflow 선례 정합.
**거부**: cursor — SPEC §5 post-PoC 이연; store 시그니처 확장=consumer-only 위반.
**consumer-only**: `GetScoresByEvaluationItem` 호출만+핸들러 슬라이싱, 0-diff.

### Decision #2 — pagination max/default: max=500, default=50

**RESOLVED**: `maxListLimit=500`, `defaultListLimit=50`. clamp: limit 누락/0→50, limit>500→500, offset 음수/비수치→0.
**근거**: workflow=store DB 전달 vs score=핸들러 메모리 슬라이싱(store.go:278 미지원) — 메커니즘 상이로 1000 답습 근거 약화; p99<50ms NFR(spec.md §4) 보호상 500. research §9.3.
**거부**: max 1000(workflow 답습) — 메커니즘 상이·NFR 과대(상수 1개로 향후 조정).
**consumer-only**: `clampPagination` 상수 2개(score_handlers.go 내부), store/마이그레이션 0-diff.

### 결정 간 일관성 (인지편향 점검 통과)

- #4=(a) ⟹ rbac.go/abac.go/authz_middleware.go/chain.go 0-diff, server.go 라우트 마운트 1줄만.
- #1=B ⟹ `/{id}/supersede` POST sub-resource (ServeMux 최장일치, `/{id}` 무충돌). #4=(a)와 무충돌.
- #3=offset + #2=500/50 ⟹ `clampPagination` 상수 2개, `GetScoresByEvaluationItem` 후 메모리 슬라이싱.
- 신규 외부 의존 0, phantom 0, 신규 마이그레이션 0, 자체 audit 0. 4건 전부 consumer-only [HARD] 0-diff — AC-SCORE-API-BOUNDARY-1 만족.
- S0 hard-verify 게이트(plan.md §4 S0): BeginScoreTx@pg_store.go:134 + ErrGradeThresholdsUnavailable@errors.go:69 검증 완료, Run 진입 방어 장치 유지.
- 앵커링 점검: #1=store 메서드 분리+non-idempotent, #2=메커니즘 차이, #3=store 미지원, #4=OBS-001+narrowing-only 검증 — 전부 코드 독립 도출.

## B. 실행 계획

### B.1 단계 접근 — sub-agent TDD (team 아님)

신규 파일 2개 + server.go 1줄, 단일 도메인 순차 → **sub-agent manager-tdd** (spec-workflow.md "sub-agent 선호: 단일 도메인 routine"). [DELTA]: [EXISTING] baseline → [NEW/MODIFY] RED-GREEN-REFACTOR.

### B.2 Sprint (plan.md §4 — §6 RESOLVED 반영)

| Sprint | 우선순위 | 내용 | REQ |
|--------|----------|------|-----|
| S0 | High | hard-verify 게이트(BeginScoreTx pg_store.go:134+store.go:255 / ErrGradeThresholdsUnavailable errors.go:69) — 미존재 시 STOP·재계획. 회귀 baseline + git 해시 스냅샷 | (전제) |
| S1 | High | `ScoreHandler`+`NewScoreHandler`+`Routes()` 7라우트 골격 + JSON/에러 헬퍼 (evidence_handlers.go:82-124 미러) + server.go 마운트 1줄 | 001/002 |
| S2 | High | 조회: GetScore(404/400)/ListScores(filter+clamp max500/def50/empty)/Rollup(numeric)/Grade(unavailable→404) | 001 |
| S3 | High | 변경: Create(201)/Update(409 immutable)/Supersede(POST /{id}/supersede→201{new,old}) + BeginScoreTx→Commit defer Rollback + pre-TX 검증(400) | 002 |
| S4 | High | mapStoreErr 표(errors.Is 전 센티넬) + TX rollback 부분커밋 0 + goleak | 004 |
| S5 | Medium | 핸들러-로컬 write-role(`requireScoreWriteRole` {Admin,Analyst}) + viewer write→403/read→200/auth-disabled 투과/admin 우회 + UBI 횡단 | 003, UBI |
| S6 | Medium | REFACTOR 헬퍼 분리(복잡도≥15 회피) + @MX + BOUNDARY-1 0-diff + 커버리지≥85% + evaluator strict≥0.75 | 전체 |

store는 fake `ScoreStore`/`ScoreTx` 격리(evidence_handlers.go 선례). 통합은 SCORE-001 커버, 본 SPEC 핸들러 단위 집중.

## C. 요구사항 → REQ 매핑 (27 AC)

| REQ 모듈 | 핵심 | AC |
|----------|------|----|
| UBI-001~004 | 데이터주권/감사 store전담/cli-anonymous/권한·불변 | UBI-001-1~2/002-1~2/003-1~2/004-1~3 (9) |
| 001 (조회) | 단건/목록 filter+pagination/롤업/등급/empty/clamp | 001-1~7 (7) |
| 002 (변경) | 생성/수정/supersede/입력검증/malformed | 002-1~5 (5) |
| 003 (ABAC) | viewer write 403/auth-disabled 투과/admin 우회 | 003-1~3 (3) |
| 004 (에러·경계) | 센티넬→HTTP/TX rollback/consumer-only 0-diff | 004-1~2 + BOUNDARY-1 (3) |

DoD: 27 AC + 16 edge, coverage≥85%, golangci-lint default+gosec 0, goleak 통과, 회귀 0, @MX, TRUST 5, evaluator strict≥0.75.
