# SPEC-AX-SCORE-API-001 (Compact) — 경영평가 점수 조회/집계 HTTP API 계층 (Score Query/Aggregation HTTP API Layer)

> v0.1.1 · status completed · TDD · thorough · brownfield · sub-agent. 상세는 spec.md / plan.md / acceptance.md / research.md. (AC=27, §7 edge case=16, §6 4건 RESOLVED — 커밋 8a61193, evaluator 0.9235/TRUST5 PASS)

## 핵심 consumer-only 계약 [HARD]

- 본 SPEC은 SPEC-AX-SCORE-001(완료 v0.1.3) / EVID-001 / AUTH-003의 **순수 consumer** — store/audit/auth/errors/`evidence_handlers.go`/`.moai/db/schema/**` **0 diff**
- 신규 DB 마이그레이션 0 (순수 API 계층 — `scores`/`grade_thresholds`는 SCORE-001이 이미 생성)
- API 자체 audit 0 — mutation 감사는 store `RecordScore*` 동일 TX 전담 (이중 감사 금지, research.md §4.2)
- TX 진입점 = `store.ScoreStore.BeginScoreTx`(→ `PgWorkflowStore.pool`)만. `postgres.go` 死 스텁 비대상
- ABAC: `RESTAuthzMiddleware`(`authz_middleware.go`/`chain.go:17`) 기존 미들웨어 체인(server.go:261 와이어링)이 innerMux 전체 자동 적용 — server.go ABAC 변경 0 (D2-2: 미들웨어 와이어링 파일, 거부 코드·상수=abac.go:24)
- 1차 산출물 = SCORE-001 store 7 메서드를 노출하는 최소 REST API + AUTH-003 ABAC 통합

## 7 엔드포인트 (research.md §11)

GET `/scores/{id}`(GetScoreByID) · GET `/scores`(GetScoresByEvaluationItem + filter/pagination) · GET `/scores/rollup`(SumWeightedByEvaluationItem) · GET `/scores/grade`(DetermineGrade) · POST `/scores`(InsertScore) · PUT `/scores/{id}`(UpdateScore) · POST `/scores/{id}/supersede`(SupersedeAndReplaceScore) — base `/api/v1`

## REQ 모듈 (5: 4 Ubiquitous 묶음 + 4 modal)

| REQ-ID | EARS | 요지 |
|--------|------|------|
| REQ-SCORE-API-UBI-001 (데이터 주권) | Ubiquitous | 7 엔드포인트 외부 호출 0건, store(내부 pgx) 위임만 |
| REQ-SCORE-API-UBI-002 (감사 가능성) | Ubiquitous | mutation은 store 동일 TX audit 1건 전담 — API 자체 audit INSERT 0 (이중 감사 금지) |
| REQ-SCORE-API-UBI-003 (cli-anonymous) | Ubiquitous(State) | authEnabled=false 투과 + created_by/user_id='cli-anonymous' (store 부여) |
| REQ-SCORE-API-UBI-004 (권한·불변) | Ubiquitous | 미인가 write→403 ABAC_CONDITION_DENIED + CONFIRMED 불변→409 (ErrScoreImmutable/NotConfirmed) |
| REQ-SCORE-API-001-E1~4/S1/O1/U1 | E/S/O/U | 조회 API: 단건 200(E1)/목록 filter+pagination(E2)/롤업 numeric(E3)/등급(E4), viewer read 허용(S1), pagination clamp(O1), 404/400(U1) |
| REQ-SCORE-API-002-E1~3/S1/U1 | E/S/U | 변경 API: POST 201(E1)/PUT 200(E2)/supersede 201(E3), write 권한 게이팅(S1), 400 검증 TX 미진입(U1) |
| REQ-SCORE-API-003-E1/S1/S2/U1 | E/S/U | ABAC: 미인가 write 403(E1), auth-disabled 투과(S1), admin 우회(S2), viewer write deny + read 유지(U1) |
| REQ-SCORE-API-004-S1/U1/U2 | S/U | 에러 매핑: 센티넬→HTTP 결정적(S1), TX rollback 부분커밋 0(U1), consumer-only 0-diff 경계(U2) |

## AC (27: AC-SCORE-API-{REQ}-{N})

- §1 UBI-001: -1(외부호출 0 정적), -2(store 위임)
- §2 UBI-002: -1(mutation store audit 1건), -2(API 자체 audit 0)
- §3 UBI-003: -1(authEnabled=false 투과+cli-anonymous), -2(실 식별자 비위조)
- §4 UBI-004: -1(미인가 write 403), -2(CONFIRMED PUT 409), -3(non-CONFIRMED supersede 409)
- §5 REQ-001: -1(GET 단건 200), -2(404), -3(목록 filter+pagination), -4(롤업 numeric 정밀도), -5(등급 200), -6(empty list), -7(pagination clamp)
- §6 REQ-002: -1(POST 201 store TX), -2(PUT 200), -3(supersede 201), -4(검증 400 TX미진입), -5(malformed UUID/JSON 400)
- §7 REQ-003: -1(viewer write deny 403 + read 허용), -2(auth-disabled 투과), -3(admin 우회)
- §8 REQ-004: -1(센티넬→HTTP 매핑 표), -2(TX rollback 부분커밋 0), BOUNDARY-1(consumer-only 0-diff)
- 분해 합 = 2+2+2+3+7+5+3+3 = 27 = 물리 heading 27. §7 edge=16. 각 modal REQ ≥2 AC

## Files to Modify

| 경로 | Delta |
|------|-------|
| `cmd/server/score_handlers.go` | [NEW] ScoreHandler+Routes+7 핸들러+JSON/에러 헬퍼+에러 매핑 (evidence_handlers.go 미러) |
| `cmd/server/score_handlers_test.go` | [NEW] httptest 핸들러 단위 테스트 |
| `cmd/server/server.go` | [MODIFY] **라우트 마운트만** (scoreH 필드:55 + NewScoreHandler:209 + innerMux.Handle 2줄:263-264, ≈7줄 최소 단위) |
| `internal/store/store.go`·`score.go` | [EXISTING] ScoreStore/ScoreTx/Score/ScoreUpdate 호출만 — **0 diff** |
| `internal/errors/errors.go` | [EXISTING] 센티넬 errors.Is 매핑만 — 0 diff |
| `internal/auth/abac.go`·`rbac.go`·`middleware.go` | [EXISTING] **frozen, 0 diff [HARD]** (permissionMatrix 수정 금지) |
| `cmd/server/evidence_handlers.go` | [EXISTING] 패턴 미러 참조만 — 0 diff |
| `.moai/db/schema/migrations/**` | [EXISTING] SCORE-001 생성 — 본 SPEC 마이그레이션 0 |

## Exclusions (What NOT to Build)

1. DB 스키마·FK·신규 마이그레이션 변경 (순수 API — SCORE-001 제공)
2. API 자체 감사(own-audit) — store `RecordScore*` 동일 TX 전담
3. 풀 등급기준(scoring rubric) 시스템 (`DetermineGrade` 호출만 — SCORE-001/EVAL-ITEM-001 §5 #2 이연)
4. 6번째 시간 제약(KST 업무시간 09:00–18:00) — AUTH-003 정합, 범위 밖
5. AUTH-003 모델 초과 풀 org-unit 속성 ABAC + RBAC permissionMatrix에 score Permission 추가
6. SPEC-AX-SCORE-001/EVID-001/AUTH-003 코드·스키마·FK·마이그레이션 변경 (consumer-only [HARD], rbac.go frozen)
7. Console UI / 클라이언트 SDK / OpenAPI 생성 (server-side 핸들러만)
8. 마이그레이션 도구·신규 외부 의존 0

## §6 OPEN (plan.md §6 — Run Phase 1 strategy.md §A + Human Gate sign-off 대상)

1. **OPEN #1 — supersede REST shape**: POST `/scores/{id}/supersede`(research §9.4/§10.2 권장 B) vs PUT+flag(A) vs PATCH(C)
2. **OPEN #2 — pagination max/default limit**: max 500(research §5/§9.3 권장) vs 1000(workflow 선례); default 50 vs 100
3. **OPEN #3 — offset vs cursor**: offset/limit(research §5/§10.1 권장 — GetScoresByEvaluationItem+핸들러 슬라이싱, workflow 선례) vs cursor
4. **OPEN #4 — ABAC 적용지점 & write 역할 매핑** [중요 — spec.md §1.5 surface]: rbac.go에 `evaluator` 역할·score Permission **부재**(rbac.go:33 정규식 `^iroum-ax:(admin|analyst|viewer)$`), permissionMatrix/Authorize **frozen** (consumer-only [HARD] 수정 불가) → (a) 핸들러-로컬 역할 매핑(OBS-001 domain-local 선례, 유력) vs (c) `analyst` 재사용 vs (b) ABAC 정책 주입; ABAC org_unit 적용지점 middleware vs handler (research §10.6)

> issue_number 0 (gh unavailable). phantom 0 — 전 시그니처 source-verified (store.go:217-292 / score.go:110-648 / errors.go:54-78 / abac.go:65-206 / rbac.go:17-111 / evidence_handlers.go:82-377 / server.go:257-261). SPEC-AX-SCORE-001 §6 OPEN→RESOLVED 흐름 동위상.
