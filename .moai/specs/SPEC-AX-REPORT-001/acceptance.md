# SPEC-AX-REPORT-001 Acceptance Criteria

> Companion: `spec.md` (EARS), `plan.md` (TDD Sprint), `research.md` (Phase 0.5 SSOT)
> 형식: Given/When/Then. AC 명명: `AC-REPORT-{REQ}-{N}`. 각 REQ ≥2 AC.
> 검증 도구: `httptest.NewRequest`/`httptest.NewRecorder`, fake `ScoreStore`/`ScoreTx`/`EvalItemStore`/`EvalItemTx`, testify/assert, t.Parallel, goleak (plan.md §4).
> Total AC = **21** / §7 Edge Case = **13** (count cross-file 일관 — spec.md §9·spec-compact.md와 동일. **plan.md §8은 plan-phase DoD로 AC/edge count를 운반하지 않음** — SCORE-API-001 D3-1 정합. count single source of truth = acceptance.md §9 + spec.md §9 + spec-compact.md).

---

## 1. REQ-REPORT-UBI-001 — 데이터 주권 (AC ×2)

### AC-REPORT-UBI-001-1 — 외부 호출 0건 + 신규 외부 의존 0 (정적)

- **Given** `cmd/server/report_handlers.go` 구현 소스 + `go.mod`/`go.sum`
- **When** import 목록·네트워크 호출(`http.Client`, 외부 SDK, 외부 URL)·신규 의존을 정적 검사
- **Then** 신규 외부 의존이 0건이며 `net/http`/`encoding/json`/`math/big`/`github.com/google/uuid`/`go.uber.org/zap`/`jackc/pgx`(`pgtype`)/`internal/store`/`internal/auth`만 사용하고, `shopspring/decimal`이 go.mod에 추가되지 않는다 (REQ-REPORT-UBI-001, spec.md §1.4 HARD, research.md §7.1).

### AC-REPORT-UBI-001-2 — 모든 경로 store 위임

- **Given** 리포트 엔드포인트 핸들러
- **When** 각 핸들러의 데이터 접근·집계 경로를 추적
- **Then** 모든 영속·계산·범주 롤업이 `store.ScoreStore`/`ScoreTx` + `store.EvalItemStore`/`EvalItemTx` 메서드 호출 조합으로만 이루어지고 외부망 egress가 0건이다 (망분리, research.md §7.1).

---

## 2. REQ-REPORT-UBI-002 — 감사 가능성 (read-only, 자체 audit 0) (AC ×2)

### AC-REPORT-UBI-002-1 — API 자체 audit 0건 (read-only)

- **Given** `report_handlers.go` 구현 소스
- **When** 핸들러 코드에서 `audit_logs` INSERT SQL 또는 `Recorder` 직접 호출 존재 여부 검사
- **Then** 모든 엔드포인트가 read-only(GET)이므로 mutation이 0건이고 API 핸들러는 자체 `audit_logs` INSERT를 0건 수행하며 `ReportHandler`에 audit recorder 의존이 주입되지 않는다 (consumer-only §1.4 #4, research.md §6.1/§6.3).

### AC-REPORT-UBI-002-2 — 점수 audit는 SCORE-001 store가 이미 기록 (위임)

- **Given** 범주 롤업이 참조하는 점수 행들
- **When** 리포트 핸들러가 `SumWeightedByEvaluationItem`/`DetermineGrade`를 호출
- **Then** 핸들러는 점수 변경을 일으키지 않고(read-only) 점수 생성/수정 시점의 `audit_logs`는 이미 SPEC-AX-SCORE-001 store(`RecordScore*` 동일 TX)가 기록한 상태이며, 본 SPEC은 추가 audit를 발생시키지 않는다 (REQ-REPORT-UBI-002, research.md §6.2).

---

## 3. REQ-REPORT-UBI-003 — cli-anonymous 기본값 + auth-disabled fallback (AC ×2)

### AC-REPORT-UBI-003-1 — authEnabled=false 전 엔드포인트 투과

- **Given** `s.cfg.AuthEnabled = false` (Walking Skeleton 기본값)
- **When** 모든 리포트 엔드포인트에 인증 없는 요청을 전송
- **Then** ABAC/RBAC 미들웨어가 투과(`abac.go:8` REQ-ABAC-009)하여 모든 엔드포인트가 정상 응답한다 (REQ-REPORT-UBI-003, research.md §7.2).

### AC-REPORT-UBI-003-2 — 실 사용자 식별자 비위조

- **Given** 인증 비활성 환경의 리포트 요청
- **When** 핸들러가 store에 전달하는 user context를 검사
- **Then** API 핸들러는 실 사용자 식별자를 위조·주입하지 않고 store/`cli-anonymous` 기본값 계약을 그대로 따른다 (누출 0, read-only이므로 created_by 생성도 0).

---

## 4. REQ-REPORT-UBI-004 — 권한 (read-narrowing, viewer 허용) (AC ×2)

### AC-REPORT-UBI-004-1 — viewer 포함 모든 인증 사용자 read 허용

- **Given** `authEnabled=true`, `viewer`-only principal (write 권한 없음)
- **When** 범주 리포트 조회 요청
- **Then** SPEC-AX-AUTH-003 ABAC narrowing 경로가 요청을 허용하고(read-only — narrowing이 거부할 write 동작 없음, `abac.go:4`) 리포트가 `200 OK`로 반환된다 (REQ-REPORT-UBI-004, §1.5).

### AC-REPORT-UBI-004-2 — write-role 게이트 부재 (SCORE-API-001 §6 OPEN #4 비적용)

- **Given** `report_handlers.go` 구현 소스
- **When** write-role 게이트(`requireScoreWriteRole`/`guardScoreWrite` 류) 존재 여부 정적 검사
- **Then** 본 SPEC은 mutation 엔드포인트가 0개이므로 write-role 게이트 코드가 0건이고 `score_handlers.go:161-187` 패턴을 차용하지 않으며, SCORE-API-001 §6 OPEN #4(`evaluator` 역할 부재 충돌)가 본 SPEC에 발생하지 않는다 (REQ-REPORT-002-U1, §1.5).

---

## 5. REQ-REPORT-001 — 범주 집계 리포트 조회 API (AC ×6)

### AC-REPORT-001-1 — 범주 리포트 200 (cross-store 조합)

- **Given** 존재하는 범주 id, 자식 평가항목 및 점수 보유
- **When** 범주 리포트 조회 요청
- **Then** 핸들러가 (i) `EvalItemStore.BeginEvalItemTx`→`GetEvalItemByID`(store.go:199)+`GetEvalItemsByParentID`(store.go:202) (ii) `ScoreStore.BeginScoreTx`→자식별 `SumWeightedByEvaluationItem`(store.go:295) 누적+`DetermineGrade`(store.go:298)을 조합하여 `200 OK` 범주 리포트 JSON(category id/name + per-item weighted sums + category total + category grade)을 반환한다 (REQ-REPORT-001-E1, research.md §Appendix; 정확 형식 §6 OPEN #3).

### AC-REPORT-001-2 — 범주 미존재 → 404

- **Given** 존재하지 않는 범주 id
- **When** 범주 리포트 조회 요청
- **Then** `GetEvalItemByID`가 not-found 센티넬을 반환하고 핸들러는 `404 Not Found` + `{"error":{"code","message","field"}}`(한국어) 본문을 반환하며 200/500이 아니다 (REQ-REPORT-001-U1, research.md §8.2).

### AC-REPORT-001-3 — 입력 검증 → 400

- **Given** 공백 또는 64자 초과 범주 id (또는 malformed 요청)
- **When** 범주 리포트 조회 요청
- **Then** `400 Bad Request` + 표준 에러 본문(`field` 지정), store 미진입 (REQ-REPORT-001-U1, `score_handlers.go` pre-store 검증 선례).

### AC-REPORT-001-4 — 빈 범주/점수 0 → 빈·0 리포트

- **Given** 자식이 0개인 범주(`GetEvalItemsByParentID` 빈 슬라이스) 또는 자식 item에 기여 점수 0건
- **When** 범주 리포트 조회 요청
- **Then** `200 OK`로 빈/0 리포트(`items:[]` 또는 해당 item `weighted_sum:"0"`, category total `"0"`, grade `null`)를 반환하며 404/500이 아니다 (REQ-REPORT-001-E2, data-completeness — research.md §14.5; 빈 리포트 vs 제외 최종 §6 OPEN #3).

### AC-REPORT-001-5 — 범주 롤업 정밀도 (float64 미경유)

- **Given** raw-level 점수 보유 자식 다수 (예: 0.1+0.2 누적 케이스)
- **When** 핸들러가 자식별 `SumWeightedByEvaluationItem`의 `pgtype.Numeric`를 범주 합계로 누적
- **Then** 누적이 `pgtype.Numeric`/표준 `math/big` 산술로 수행되어 Go `float64`를 미경유하고 정확 십진 문자열로 직렬화된다(0.1+0.2 오차 미발생) (REQ-REPORT-001-S2, score.go:453-457 SEC-03; 누적 메커니즘 §6 OPEN #2 RESOLVED안 적용).

### AC-REPORT-001-6 — viewer-only read 허용 (state-driven)

- **Given** `viewer`-only 인증 principal (또는 auth 비활성)
- **When** 범주 리포트 조회 요청
- **Then** ABAC/RBAC 체인이 거부 없이 통과하여 리포트가 `200 OK`로 반환된다 (REQ-REPORT-001-S1, read-only — §1.5).

---

## 6. REQ-REPORT-002 — ABAC read-narrowing 통합 (AC ×3)

### AC-REPORT-002-1 — 인증 사용자(viewer/analyst) read 허용

- **Given** `authEnabled=true`, `viewer` 또는 `analyst` principal
- **When** 범주 리포트 조회 요청
- **Then** SPEC-AX-AUTH-003 ABAC narrowing 경로가 요청을 허용하고(read-only — 추가 거부 없음, `abac.go:4`) 리포트가 `200 OK`로 반환된다 (REQ-REPORT-002-E1, research.md §4.2).

### AC-REPORT-002-2 — auth-disabled 투과 (Walking Skeleton)

- **Given** `authEnabled=false`
- **When** 모든 리포트 엔드포인트 각각 요청
- **Then** ABAC/RBAC 미들웨어가 투과하여 모든 엔드포인트가 정상 동작한다 (REQ-REPORT-002-S1, `server.go:261` 미들웨어 체인 비활성 시 투과 정합).

### AC-REPORT-002-3 — admin 우회

- **Given** `authEnabled=true`, `RoleAdmin` 보유 principal
- **When** 모든 리포트 엔드포인트 요청
- **Then** ABAC evaluator가 모든 조건을 우회(`abac.go:9` REQ-ABAC-004)하여 admin이 전 리포트 엔드포인트를 호출할 수 있다 (REQ-REPORT-002-S2).

---

## 7. REQ-REPORT-003 — store 에러→HTTP 매핑 & consumer-only 경계 (AC ×4)

### AC-REPORT-003-1 — 에러 센티넬→HTTP status 매핑 표

- **Given** store가 각 센티넬을 반환하는 상황 (fake `EvalItemTx`/`ScoreTx`)
- **When** 핸들러가 `errors.Is`로 매핑 (`score_handlers.go:111-137` `mapStoreErr` 미러)
- **Then** 다음 매핑이 결정적으로 성립한다:

| 센티넬 | HTTP |
|--------|------|
| category not-found (EVAL-ITEM not-found 센티넬) | 404 |
| `ErrGradeThresholdsUnavailable` | grade 필드 `null` + HTTP 200 (B-2 graceful — strategy.md §A.3 RESOLVED 2026-05-19; 핸들러-로컬 `errors.Is` 분기, `mapStoreErr` 0-diff) |
| invalid input | 400 |
| unwrapped/unknown DB error | 500 |

### AC-REPORT-003-2 — cross-store 2-TX read rollback (BOUNDARY)

- **Given** 리포트 핸들러가 `BeginEvalItemTx` 및/또는 `BeginScoreTx` 후 downstream 호출이 실패
- **When** 요청 처리
- **Then** 열린 각 read TX(`EvalItemTx`/`ScoreTx`)가 `defer tx.Rollback(ctx)`로 정리되고(Commit 없음 — read-only), 부분 상태가 0이며 goroutine 누출이 0이다 (goleak) (REQ-REPORT-003-U1, research.md §3.3).

### AC-REPORT-003-3 — 단일 store 가정 금지 (cross-store 분리)

- **Given** `ReportHandler` 구현
- **When** store 의존 구조를 검사
- **Then** `ReportHandler`는 `ScoreStore`와 `EvalItemStore`를 **별개 의존**으로 보유하고(`NewReportHandler(pgStore, pgStore, logger)`, pgStore가 두 인터페이스 동시 구현 — pg_store.go:118/134 source-verified), 자식 열거(`GetEvalItemsByParentID`, EvalItemTx)와 가중합(`SumWeightedByEvaluationItem`, ScoreTx)을 서로 다른 TX로 수행한다 (§6 OPEN #1, spec.md §6.1 [CRITICAL]).

### AC-REPORT-BOUNDARY-1 — consumer-only 0-diff (BOUNDARY)

- **Given** 본 SPEC 구현 완료 상태
- **When** `git diff`로 `internal/store/`, `internal/audit/`, `internal/auth/`, `internal/errors/`, `cmd/server/score_handlers.go`, `cmd/server/evidence_handlers.go`, `.moai/db/schema/**`, `go.mod`, `go.sum` 변경 검사
- **Then** 위 경로의 변경이 **0 diff**이고, 신규 마이그레이션이 0건, 신규 store 메서드가 0건, 신규 외부 의존이 0건이며, `cmd/server/server.go`의 변경은 라우트 마운트(`reportH` 필드 + `NewReportHandler` + `innerMux.Handle` 2줄, ≈7줄)로 한정된다 (REQ-REPORT-003-U2, consumer-only §1.4 [HARD], Drift-Guard manifest).

---

## §7 Edge Case Catalog (13개)

| # | Edge Case | 기대 동작 | AC |
|---|-----------|----------|-----|
| 1 | 존재하지 않는 범주 id | 404 표준 에러, store 미진입(검증 후) | AC-REPORT-001-2 |
| 2 | 공백/64자 초과 범주 id | 400, store 미진입 | AC-REPORT-001-3 |
| 3 | 자식 0 범주 (`GetEvalItemsByParentID` 빈) | 200 빈 리포트(`items:[]`, total `"0"`, grade `null`) | AC-REPORT-001-4 |
| 4 | 자식 item에 점수 0건 | 200, 해당 item `weighted_sum:"0"` (제외 아님 — 빈 리포트 원칙) | AC-REPORT-001-4 |
| 5 | grade thresholds 미설정 scope | grade 필드 `null` + 200 (B-2 graceful — strategy.md §A.3) | AC-REPORT-003-1 |
| 6 | 0.1+0.2 누적 정밀도 | float64 미경유, 정확 십진 직렬화(오차 0) | AC-REPORT-001-5 |
| 7 | authEnabled=false 전 엔드포인트 | 투과, 정상 응답 | AC-REPORT-UBI-003-1, 002-2 |
| 8 | viewer-only 인증 read | 200 허용 (read-narrowing) | AC-REPORT-UBI-004-1, 001-6, 002-1 |
| 9 | RoleAdmin | 전 엔드포인트 우회 허용 | AC-REPORT-002-3 |
| 10 | 루트만 존재(범주 하위 계층 없음) | 200 빈/0 리포트 (data-completeness) | AC-REPORT-001-4 |
| 11 | cross-store 2-TX read 실패 (EvalItemTx 또는 ScoreTx) | 각 TX defer Rollback, 부분 상태 0, goroutine 누출 0 | AC-REPORT-003-2 |
| 12 | write-role 게이트 차용 시도 | 코드에 write-role 게이트 0건 (read-only — score_handlers.go:161-187 미차용) | AC-REPORT-UBI-004-2 |
| 13 | consumer-only 경계 (store/auth/schema/go.mod 0-diff + API audit 0 + 신규 store 메서드 0) | git diff 0, audit INSERT 0, 신규 외부 의존 0 | AC-REPORT-BOUNDARY-1, UBI-002-1 |

---

## §9 Definition of Done (Acceptance 단계)

- [ ] Total AC = **21** (UBI-001 2 + UBI-002 2 + UBI-003 2 + UBI-004 2 + 001 6 + 002 3 + 003 4 = 21), 컴포넌트 분해 일치
- [ ] §7 Edge Case Catalog = **13개** 물리적 행, 각 AC 추적
- [ ] 각 REQ ≥2 G/W/T, AC 명명 `AC-REPORT-{REQ}-{N}`
- [ ] AC 21 / edge 13 count가 spec.md §9 · spec-compact.md와 cross-file 일관 (count single source of truth = acceptance.md §9 + spec.md §9 + spec-compact.md; plan.md §8은 plan-phase DoD로 count 미운반 — SCORE-API-001 D3-1 정합)
- [ ] consumer-only [HARD] 검증 AC (AC-REPORT-BOUNDARY-1: store/audit/auth/errors/score_handlers.go/evidence_handlers.go/migrations/go.mod 0-diff, 신규 마이그레이션 0, 신규 store 메서드 0, 신규 외부 의존 0, 자체 audit 0)
- [ ] phantom API 0 — 모든 AC가 source-verified 시그니처(store.go:115-298 / pg_store.go:118,134 / score.go:453-457 / score_handlers.go:43-137 / abac.go:4-24 / rbac.go:20-33 / server.go:55,209,263-265 / go.mod file:line)에 추적 (메모리 lesson #9 — `GetEvalItemsByParentID` 실재 확정)
- [ ] cross-store 2-TX 조합 AC (AC-REPORT-003-3 — EvalItemStore≠ScoreStore 분리 의존, §6 OPEN #1 surface)
- [ ] 구현 코드/테스트 미작성 (acceptance 문서만)
