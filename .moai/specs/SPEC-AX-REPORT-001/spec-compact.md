# SPEC-AX-REPORT-001 (Compact) — 평가 결과 리포트/집계 HTTP API 계층 (Evaluation Result Report/Aggregation HTTP API Layer)

> v0.1.0 · status draft · TDD · thorough · brownfield · sub-agent. 상세는 spec.md / plan.md / acceptance.md / research.md. (AC=21, §7 edge case=13, §6 3건 OPEN — Run Phase strategy.md §A + Human Gate sign-off RESOLVED 대상)

## 핵심 consumer-only 계약 [HARD]

- 본 SPEC은 SPEC-AX-SCORE-001(완료 v0.1.3) / SCORE-API-001(완료 v0.1.1) / EVAL-ITEM-001(완료) / AUTH-003(완료)의 **순수 consumer** — store/audit/auth/errors/`score_handlers.go`/`evidence_handlers.go`/`.moai/db/schema/**`/`go.mod` **0 diff**
- 신규 DB 마이그레이션 0 (읽기 전용 API — `scores`/`grade_thresholds`/`evaluation_items`는 SCORE-001/EVAL-ITEM-001이 이미 생성)
- 신규 store 메서드 0 — Option A(핸들러 cross-store 조합)만, `SumWeightedByCategory` 등 추가 금지 (research.md §2.2 Option B = consumer-only 위반)
- 신규 외부 의존 0 — `shopspring/decimal` 부재(`go.mod`·`go.sum` 0건 orchestrator ground-truth 독립 확정 = source-verified 명제; go.mod 16개 직접 의존 보유, 목록 축소 열거 금지), 정밀도 누적은 표준 `math/big`/`pgtype` (§6 OPEN #2)
- API 자체 audit 0 — **read-only이므로 mutation 0 → audit 이벤트 0** (점수 audit는 SCORE-001 store가 생성 시점에 이미 기록, research.md §6.1/§6.3)
- TX 진입점 = `store.ScoreStore.BeginScoreTx`(→`pg_store.go:134`) + `store.EvalItemStore.BeginEvalItemTx`(→`pg_store.go:118`)만. `postgres.go` 死 스텁 비대상
- ABAC: 기존 미들웨어 체인(server.go:261 와이어링)이 innerMux 전체 자동 적용 — server.go ABAC 변경 0. **읽기 전용 → write-role 게이트(`score_handlers.go:161-187`) 미차용** (§1.5 — SCORE-API-001 §6 OPEN #4 본 SPEC 비적용)
- 1차 산출물 = SCORE-001 점수 store + EVAL-ITEM-001 taxonomy를 결합한 범주별 집계 리포트 read-only REST API + AUTH-003 ABAC read-narrowing

## 리포트 엔드포인트 (research.md §14.2 — 잠정, §6 OPEN #3 확정)

GET 범주별 집계 리포트(read-only) — (EvalItemTx) `GetEvalItemByID`+`GetEvalItemsByParentID` 자식 열거 → (ScoreTx) 자식별 `SumWeightedByEvaluationItem` 누적 + `DetermineGrade` 범주 등급 — cross-store 2-TX 조합(Option A). base `/api/v1`. mutation 엔드포인트 0

## REQ 모듈 (4: 4 Ubiquitous 묶음 + 3 modal — read-only, mutation REQ 0)

| REQ-ID | EARS | 요지 |
|--------|------|------|
| REQ-REPORT-UBI-001 (데이터 주권) | Ubiquitous | 외부 호출 0, store(내부 pgx) 위임만, 신규 외부 의존 0(shopspring 미도입) |
| REQ-REPORT-UBI-002 (감사 가능성) | Ubiquitous | read-only이므로 mutation 0 → API 자체 audit 0 (점수 audit는 SCORE-001 store가 생성 시점 기록) |
| REQ-REPORT-UBI-003 (cli-anonymous) | Ubiquitous(State) | authEnabled=false 투과 + 실 식별자 비위조 (store 'cli-anonymous' 계약) |
| REQ-REPORT-UBI-004 (권한·read-narrowing) | Ubiquitous | viewer 포함 모든 인증 사용자 read 허용 + write-role 게이트 0 (SCORE-API-001 §6 OPEN #4 비적용) |
| REQ-REPORT-001-E1~2/S1~2/O1/U1 | E/S/O/U | 범주 리포트: cross-store 조합 200(E1)/빈·0 리포트(E2), viewer read 허용(S1), float64 미경유 누적(S2), 페이지네이션 clamp(O1), 404/400(U1) |
| REQ-REPORT-002-E1/S1/S2/U1 | E/S/U | ABAC: 인증 사용자 read 허용(E1), auth-disabled 투과(S1), admin 우회(S2), write-role 게이트·mutation OUT(U1) |
| REQ-REPORT-003-S1/U1/U2 | S/U | 에러 매핑: 센티넬→HTTP 결정적(S1), cross-store 2-TX read rollback(U1), consumer-only 0-diff·신규 store/의존 0 경계(U2) |

## AC (21: AC-REPORT-{REQ}-{N})

- §1 UBI-001: -1(외부호출 0+신규의존 0 정적), -2(store 위임)
- §2 UBI-002: -1(API 자체 audit 0 read-only), -2(점수 audit SCORE-001 위임)
- §3 UBI-003: -1(authEnabled=false 투과), -2(실 식별자 비위조)
- §4 UBI-004: -1(viewer read 허용), -2(write-role 게이트 부재)
- §5 REQ-001: -1(범주 리포트 200 cross-store), -2(범주 미존재 404), -3(검증 400), -4(빈·0 리포트), -5(정밀도 float64 미경유), -6(viewer read S1)
- §6 REQ-002: -1(인증 사용자 read 허용), -2(auth-disabled 투과), -3(admin 우회)
- §7 REQ-003: -1(센티넬→HTTP 매핑 표), -2(cross-store 2-TX rollback), -3(단일 store 가정 금지), BOUNDARY-1(consumer-only 0-diff)
- 분해 합 = 2+2+2+2+6+3+4 = 21 = 물리 heading 21. §7 edge=13. 각 modal REQ ≥2 AC

## Files to Modify

| 경로 | Delta |
|------|-------|
| `cmd/server/report_handlers.go` | [NEW] ReportHandler(scoreStore+evalItemStore, recorder/write-role 미주입)+Routes+리포트 핸들러+cross-store 롤업 조합+JSON/에러 헬퍼+에러 매핑 (score_handlers.go read-only 미러) |
| `cmd/server/report_handlers_test.go` | [NEW] httptest 핸들러 단위 테스트 |
| `cmd/server/server.go` | [MODIFY] **라우트 마운트만** (reportH 필드:55 + `NewReportHandler(pgStore,pgStore,logger)`:209 + innerMux.Handle 2줄:263-264, ≈7줄 최소 단위) |
| `internal/store/store.go`·`score.go`·`pg_store.go` | [EXISTING] ScoreStore/ScoreTx/EvalItemStore/EvalItemTx/Score/EvalItem 호출만 — **0 diff** (store.go:115-298, pg_store.go:118/134) |
| `internal/errors/errors.go` | [EXISTING] 센티넬 errors.Is 매핑만 — 0 diff |
| `internal/auth/abac.go`·`rbac.go`·`middleware.go` | [EXISTING] **frozen, 0 diff [HARD]** (permissionMatrix 수정 금지) |
| `cmd/server/score_handlers.go`·`evidence_handlers.go` | [EXISTING] 패턴 미러 참조만 (write-role 미차용) — 0 diff |
| `.moai/db/schema/migrations/**`·`go.mod`·`go.sum` | [EXISTING] SCORE-001/EVAL-ITEM-001 생성 — 본 SPEC 마이그레이션 0, 신규 외부 의존 0 |

## Exclusions (What NOT to Build)

1. DB 스키마·FK·신규 마이그레이션 변경 (읽기 전용 — SCORE-001/EVAL-ITEM-001 제공)
2. API 자체 감사(own-audit) — read-only mutation 0, 점수 audit는 SCORE-001 store 전담
3. mutation(write) 엔드포인트 — SCORE-API-001 제공, write-role 게이트 미차용
4. 집계 결과 스냅샷 영속화 (on-the-fly만)
5. 신규 store 메서드(`SumWeightedByCategory` Option B) / 신규 외부 의존(`shopspring/decimal`) — 표준 `math/big`/`pgtype`
6. 풀 등급기준(scoring rubric) 시스템 (`DetermineGrade` 호출만 — SCORE-001/EVAL-ITEM-001 §5 이연)
7. 6번째 시간 제약(KST 업무시간) — AUTH-003/SCORE-API-001 정합, 범위 밖
8. AUTH-003 모델 초과 풀 org-unit 속성 ABAC + RBAC permissionMatrix에 report Permission 추가
9. SPEC-AX-SCORE-001/SCORE-API-001/EVAL-ITEM-001/AUTH-003 코드·스키마·FK·마이그레이션 변경 (consumer-only [HARD], rbac.go frozen)
10. Console UI / 클라이언트 SDK / OpenAPI 생성 (server-side 핸들러만)

## §6 OPEN (plan.md §6 — Run Phase strategy.md §A + Human Gate sign-off 대상)

1. **OPEN #1 [CRITICAL] — 범주 롤업 cross-store 조합 + child-enumeration**: research.md §2.3/§14.1 인용 `GetEvalItemsByParentID` **source-verified 실재**(store.go:200-202 — phantom 아님, 메모리 lesson #9 게이트 통과). 단 `EvalItemTx`(BeginEvalItemTx store.go:115-118) ≠ `ScoreTx`(BeginScoreTx store.go:254-255) → 범주 리포트 1건 = **cross-store 2-TX read 조합**(research 미표면화 load-bearing — Agent Core Behavior #2). strategy: (a) ReportHandler 2 store 의존(NewReportHandler(pgStore,pgStore,logger), pgStore 동시 구현 source-verified) (b) 계층 깊이 1-level vs 재귀 (c) 시그니처 불일치 시 블로커. Option B(SumWeightedByCategory) 불가=consumer-only [HARD]
2. **OPEN #2 — 정밀도 누적 산술 [신규 외부 의존 [HARD] 금지]**: `SumWeightedByEvaluationItem`→`pgtype.Numeric`(float64 미경유 SEC-03). 범주 누적 N개 정확 합산. **research §14.4 권장 `shopspring/decimal`은 `go.mod`·`go.sum` 0건 부재**(orchestrator ground-truth 독립 확정) → 신규 직접 의존 추가 금지. **[D3-2]** `DetermineGrade(scope, score float64)` float64는 SEC-03 무충돌(SEC-03 범위=N-항 누적 경로, 등급 임계 비교 1회 입력 아님 — spec.md §6.2 D3-2). strategy: (a)[권장] `pgtype.Numeric`↔`math/big` 표준 산술 (b) pgtype 자체 산술 API (c) 간접 의존 재확인(현 부재→[HARD] 금지). 신규 외부 의존 baked-in 금지
3. **OPEN #3 — 리포트 응답 형식 + 페이지네이션/필터 + 빈/누락**: (1) 응답 JSON 구조(research §9.3/§14.2) (2) 경로(단건 `/reports/category/{id}` vs 목록 `/reports`) (3) 페이지네이션/필터 필요 여부(`score_handlers.go` clamp 선례) (4) 빈/누락 표면화(자식 0·점수 0·`ErrGradeThresholdsUnavailable` → 빈 리포트 권장 research §14.5, grade `null` vs 404 — REQ-REPORT-003-S1 동기화). strategy + Human Gate RESOLVED

> issue_number 0 (gh unavailable). phantom 0 — 전 시그니처 source-verified (store.go:115-298 / pg_store.go:118,134 / score.go:110-648 / errors.go / score_handlers.go:43-187 / abac.go:4-24 / rbac.go:20-33 / middleware.go:25-49 / server.go:55,209,263-265 / go.mod). 특히 `GetEvalItemsByParentID`(store.go:200-202)·`BeginEvalItemTx`(pg_store.go:118) orchestrator ground-truth grep 확정 — 메모리 lesson #9 phantom-API 게이트 통과. SPEC-AX-SCORE-API-001 §6 OPEN→RESOLVED 흐름 동위상. 읽기 전용 설계로 SCORE-API-001 §6 OPEN #4(write 역할/`evaluator` 부재) 충돌 본 SPEC 비발생(설계 우월점).
