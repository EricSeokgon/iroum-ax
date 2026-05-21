# SPEC-AX-SCORE-API-001 Acceptance Criteria

> Companion: `spec.md` (EARS), `plan.md` (TDD Sprint), `research.md` (Phase 0.5 SSOT)
> 형식: Given/When/Then. AC 명명: `AC-SCORE-API-{REQ}-{N}`. 각 REQ ≥2 AC.
> 검증 도구: `httptest.NewRequest`/`httptest.NewRecorder`, fake `ScoreStore`/`ScoreTx`, testify/assert, t.Parallel, goleak (plan.md §4).
> Total AC = **27** / §7 Edge Case = **16** (count cross-file 일관 — spec.md §9·spec-compact.md와 동일. **plan.md §8은 plan-phase DoD로 AC/edge count를 운반하지 않음** [D3-1] — count single source of truth는 acceptance.md §9 + spec.md §9 + spec-compact.md).

---

## 1. REQ-SCORE-API-UBI-001 — 데이터 주권 (AC ×2)

### AC-SCORE-API-UBI-001-1 — 외부 호출 0건 (정적)

- **Given** `cmd/server/score_handlers.go` 구현 소스
- **When** import 목록과 네트워크 호출(`http.Client`, 외부 SDK, 외부 URL)을 정적 검사
- **Then** 신규 외부 의존이 0건이며 `net/http`/`encoding/json`/`github.com/google/uuid`/`go.uber.org/zap`/`pgtype`/`internal/store`/`internal/auth`만 사용한다 (REQ-SCORE-API-UBI-001, research.md §8.1).

### AC-SCORE-API-UBI-001-2 — 모든 경로 store 위임

- **Given** 7 엔드포인트 핸들러
- **When** 각 핸들러의 데이터 접근 경로를 추적
- **Then** 모든 영속·계산이 `store.ScoreStore`/`ScoreTx` 메서드 호출로만 이루어지고 외부망 egress가 0건이다 (망분리, research.md §8.4).

---

## 2. REQ-SCORE-API-UBI-002 — 감사 가능성 (store 전담) (AC ×2)

### AC-SCORE-API-UBI-002-1 — mutation 1건 store audit

- **Given** 인가된 `POST /api/v1/scores` (또는 PUT/supersede) 요청
- **When** 핸들러가 `BeginScoreTx` → `InsertScore`(또는 `UpdateScore`/`SupersedeAndReplaceScore`) → `Commit` 수행
- **Then** 점수 변경과 동일 TX에 정확히 1건의 `audit_logs`가 store 계층(`RecordScore*`, score.go:130/307/364)에 의해 기록되고, 그 기록은 store 계약에 따라 보장된다 (REQ-SCORE-API-UBI-002, research.md §4.2).

### AC-SCORE-API-UBI-002-2 — API 자체 audit 0건 (이중 감사 금지)

- **Given** `score_handlers.go` 구현 소스
- **When** 핸들러 코드에서 `audit_logs` INSERT SQL 또는 `Recorder` 직접 호출 존재 여부 검사
- **Then** API 핸들러는 자체 `audit_logs` INSERT를 0건 수행하며 `ScoreHandler`에 audit recorder 의존이 주입되지 않는다 (consumer-only §1.4 #4, 이중 감사 금지).

---

## 3. REQ-SCORE-API-UBI-003 — cli-anonymous 기본값 + auth-disabled fallback (AC ×2)

### AC-SCORE-API-UBI-003-1 — authEnabled=false 전 엔드포인트 투과

- **Given** `s.cfg.AuthEnabled = false` (Walking Skeleton 기본값)
- **When** 7 엔드포인트 각각에 인증 없는 요청을 전송
- **Then** ABAC/RBAC 미들웨어가 투과(`abac.go:8` REQ-ABAC-009)하여 모든 엔드포인트가 정상 응답하고, 생성된 점수/감사 행의 `created_by`/`user_id`는 store 계층이 부여한 `cli-anonymous`이다 (REQ-SCORE-API-UBI-003, research.md §7).

### AC-SCORE-API-UBI-003-2 — 실 사용자 식별자 비위조

- **Given** 인증 비활성 환경의 mutation 요청
- **When** 핸들러가 store에 전달하는 user context를 검사
- **Then** API 핸들러는 실 사용자 식별자를 위조·주입하지 않고 store/`cli-anonymous` 기본값 계약을 그대로 따른다 (누출 0).

---

## 4. REQ-SCORE-API-UBI-004 — 권한·불변 (AC ×3)

### AC-SCORE-API-UBI-004-1 — 미인가 write → 403

- **Given** `authEnabled=true`, write 권한 없는 principal
- **When** `POST /api/v1/scores`(또는 PUT/supersede) 요청
- **Then** SPEC-AX-AUTH-003 ABAC narrowing 경로가 HTTP `403` + 에러코드 `ABAC_CONDITION_DENIED`(`abac.go:24`)로 거부하고 `ScoreTx`가 열리지 않는다 (REQ-SCORE-API-UBI-004).

### AC-SCORE-API-UBI-004-2 — CONFIRMED 행 PUT → 409

- **Given** `status='CONFIRMED'`인 점수 id, 인가된 write principal
- **When** `PUT /api/v1/scores/{id}` body에 `score_value` 변경 요청
- **Then** store가 `ErrScoreImmutable`(errors.go:62)을 반환하고 핸들러는 HTTP `409 Conflict` + 표준 에러 본문(한국어 "CONFIRMED 점수는 정정(supersede)으로만 수정 가능")을 반환하며 200/500이 아니다 (research.md §9.1).

### AC-SCORE-API-UBI-004-3 — non-CONFIRMED supersede → 409 (두 진입 상태 차별)

- **Given** non-CONFIRMED 점수 id — **두 분기**: (분기 A, §7 edge #14) `status='DRAFT'`인 행(아직 미확정, in-place UpdateScore 경로 대상), (분기 B, §7 edge #4) `status='SUPERSEDED'`인 terminal 행(이미 정정 완료된 종단)
- **When** 각 분기에 `POST /api/v1/scores/{id}/supersede` 요청
- **Then** 두 분기 모두 store가 `ErrScoreNotConfirmed`(errors.go:78)을 반환하고 핸들러는 HTTP `409 Conflict` + 표준 에러 본문을 반환한다 — 정정(supersede)은 `CONFIRMED` 행에만 허용(SCORE-001 D4 state-machine: DRAFT는 UpdateScore in-place 경로, SUPERSEDED는 terminal). 두 진입 상태는 §7 edge #14(DRAFT)·#4(SUPERSEDED)로 별개 차별화되어 16-count 실질 보장 (research.md §9.1).

---

## 5. REQ-SCORE-API-001 — 점수 조회 API (AC ×7)

### AC-SCORE-API-001-1 — GET 단건 200

- **Given** 존재하는 점수 id, well-formed UUID path
- **When** `GET /api/v1/scores/{id}`
- **Then** `GetScoreByID` 호출 결과 `store.Score`(store.go:217-230 전 필드)를 `Content-Type: application/json`으로 직렬화하여 `200 OK` 반환 (REQ-SCORE-API-001-E1).

### AC-SCORE-API-001-2 — not-found → 404

- **Given** 존재하지 않는 점수 id
- **When** `GET /api/v1/scores/{id}`
- **Then** store가 `ErrScoreNotFound`(errors.go:54)을 반환하고 핸들러는 `404 Not Found` + `{"error":{"code","message","field"}}` 본문을 반환하며 200/500이 아니다 (REQ-SCORE-API-001-U1).

### AC-SCORE-API-001-3 — GET 목록 filter + pagination

- **Given** 동일 `evaluation_item_id`의 점수 다수, `?evaluation_item_id=X&level=raw&status=DRAFT&offset=0&limit=10`
- **When** `GET /api/v1/scores?...`
- **Then** `GetScoresByEvaluationItem` 호출 후 핸들러 계층에서 `level`/`status` 필터 + `offset`/`limit` 슬라이싱 적용하여 `200 OK` `{"scores":[...],"count":N}` 반환 (REQ-SCORE-API-001-E2).

### AC-SCORE-API-001-4 — GET 가중 롤업 정밀도

- **Given** raw-level 자식 점수 보유 `evaluation_item_id`
- **When** `GET /api/v1/scores/rollup?evaluation_item_id=X`
- **Then** `SumWeightedByEvaluationItem`의 `pgtype.Numeric` 결과를 float64 미경유 정확 십진 문자열로 직렬화하여 `200 OK` `{"evaluation_item_id":"X","weighted_sum":"<decimal>"}` 반환 (REQ-SCORE-API-001-E3, score.go:453-457).

### AC-SCORE-API-001-5 — GET 등급 200

- **Given** `grade_thresholds` 설정된 scope, `?scope=default&score=85.50`
- **When** `GET /api/v1/scores/grade?scope=default&score=85.50`
- **Then** `DetermineGrade(scope, score)` 호출 결과를 `200 OK` `{"scope":"default","score":85.50,"grade":"<S|A|B|C|D>"}`로 반환 (REQ-SCORE-API-001-E4).

### AC-SCORE-API-001-6 — empty list

- **Given** 결과 0건인 `evaluation_item_id`
- **When** `GET /api/v1/scores?evaluation_item_id=none`
- **Then** `200 OK` `{"scores":[],"count":0}` (빈 배열, NULL/필드 누락 금지) (REQ-SCORE-API-001-E2).

### AC-SCORE-API-001-7 — pagination clamp

- **Given** `?evaluation_item_id=X&limit=99999&offset=-5` (max 초과 + 음수 offset)
- **When** `GET /api/v1/scores?...`
- **Then** `limit`은 설정 최대값으로 clamp, `offset`은 0으로 보정되어 결정적 결과를 반환한다 (REQ-SCORE-API-001-O1; 구체 max/default 값은 §6 OPEN #2/#3 RESOLVED 후 단언 확정 — workflow `grpc_server.go` clamp 선례).

---

## 6. REQ-SCORE-API-002 — 점수 변경 API (AC ×5)

### AC-SCORE-API-002-1 — POST 생성 201 (store TX)

- **Given** 인가된 write principal, 유효 body `{evaluation_item_id,level,score_value,...}`
- **When** `POST /api/v1/scores`
- **Then** `BeginScoreTx`→`InsertScore`→`Commit` 후 `201 Created` `{"score_id":"<uuid>","status":"DRAFT"}` 반환, 동일 TX audit 1건은 store 전담 (REQ-SCORE-API-002-E1, research.md §2.1).

### AC-SCORE-API-002-2 — PUT 수정 200

- **Given** 인가된 write principal, DRAFT 점수 id, body `{score_value:90.0}`
- **When** `PUT /api/v1/scores/{id}`
- **Then** `ScoreUpdate`(store.go:235-246)로 매핑 후 `UpdateScore`→`Commit`, `200 OK` `{"score_id":"<uuid>"}` 반환 (REQ-SCORE-API-002-E2).

### AC-SCORE-API-002-3 — POST supersede 201

- **Given** 인가된 write principal, CONFIRMED 점수 id, body `{score_value:88.0}`
- **When** `POST /api/v1/scores/{id}/supersede`
- **Then** `SupersedeAndReplaceScore` 후 `201 Created` `{"score_id":"<new-uuid>","superseded_id":"<old-uuid>"}` 반환 (REQ-SCORE-API-002-E3; supersede REST shape는 §6 OPEN #1 RESOLVED 후 확정 — research §9.4 권장 B).

### AC-SCORE-API-002-4 — 입력 검증 → 400 (TX 미진입)

- **Given** body `evaluation_item_id` 공백(또는 >64자 / `score_value` 비수치 / `evidence_id` 비-UUID)
- **When** `POST /api/v1/scores`
- **Then** `400 Bad Request` + 표준 에러 본문(`field` 지정), `ScoreTx` 미진입, `scores`/`audit_logs` write 0건 (REQ-SCORE-API-002-U1, evidence_handlers.go:204-218 선례).

### AC-SCORE-API-002-5 — malformed UUID/JSON → 400

- **Given** path가 비-UUID(`/api/v1/scores/not-a-uuid`) 또는 body가 malformed JSON
- **When** PUT/POST 요청
- **Then** `400 Bad Request` + 표준 에러 본문, `ScoreTx` 미진입 (REQ-SCORE-API-002-U1).

---

## 7. REQ-SCORE-API-003 — ABAC 통합 & 권한 narrowing (AC ×3)

### AC-SCORE-API-003-1 — viewer write deny → 403

- **Given** `authEnabled=true`, `viewer`-only principal
- **When** `POST`/`PUT`/`supersede` 요청
- **Then** SPEC-AX-AUTH-003 narrowing 경로가 store 호출 전 `403` + `ABAC_CONDITION_DENIED`로 거부하고, 동일 principal의 GET 요청은 허용된다 (REQ-SCORE-API-003-E1/U1, research.md §4.1).

### AC-SCORE-API-003-2 — auth-disabled 투과 (Walking Skeleton)

- **Given** `authEnabled=false`
- **When** 7 엔드포인트 각각 요청
- **Then** ABAC/RBAC 미들웨어가 투과하여 모든 엔드포인트가 정상 동작한다 (REQ-SCORE-API-003-S1, `server.go:261` `ABACMiddleware(...,s.cfg.AuthEnabled)` 정합).

### AC-SCORE-API-003-3 — admin 우회

- **Given** `authEnabled=true`, `RoleAdmin` 보유 principal
- **When** 7 엔드포인트(쓰기 포함) 요청
- **Then** ABAC evaluator가 모든 조건을 우회(`abac.go:71` REQ-ABAC-004)하여 admin이 전 엔드포인트를 호출할 수 있다 (REQ-SCORE-API-003-S2).

---

## 8. REQ-SCORE-API-004 — store 에러→HTTP 매핑 & consumer-only 경계 (AC ×3)

### AC-SCORE-API-004-1 — 에러 센티넬→HTTP status 매핑 표

- **Given** store가 각 센티넬을 반환하는 상황 (fake `ScoreTx`)
- **When** 핸들러가 `errors.Is`로 매핑
- **Then** 다음 매핑이 결정적으로 성립한다 (errors.go:54-78):

| 센티넬 | HTTP |
|--------|------|
| `ErrScoreNotFound` | 404 |
| `ErrScoreInvalidInput` | 400 |
| `ErrScoreImmutable` | 409 |
| `ErrScoreInvalidStatus` | 409 |
| `ErrScoreNotConfirmed` | 409 |
| `ErrGradeThresholdsUnavailable` | 404 |
| unwrapped/unknown DB error | 500 |

### AC-SCORE-API-004-2 — TX rollback 부분 커밋 0 (BOUNDARY)

- **Given** mutation 핸들러가 `BeginScoreTx` 후 store 호출이 commit 전 실패
- **When** 요청 처리
- **Then** `committed`-flag defer가 `tx.Rollback(ctx)`를 실행하여 부분 커밋이 0이고 goroutine 누출이 0이다 (goleak) (REQ-SCORE-API-004-U1, evidence_handlers.go:342-353 선례).

### AC-SCORE-API-BOUNDARY-1 — consumer-only 0-diff (BOUNDARY)

- **Given** 본 SPEC 구현 완료 상태
- **When** `git diff`로 `internal/store/`, `internal/audit/`, `internal/auth/`, `internal/errors/`, `cmd/server/evidence_handlers.go`, `.moai/db/schema/**` 변경 검사
- **Then** 위 경로의 변경이 **0 diff**이고, 신규 마이그레이션이 0건이며, `cmd/server/server.go`의 변경은 라우트 마운트(`scoreH` 필드 + `NewScoreHandler` + `innerMux.Handle("/api/v1/scores", ...)` 1줄)로 한정된다 (REQ-SCORE-API-004-U2, consumer-only §1.4 [HARD], Drift-Guard manifest).

---

## §7 Edge Case Catalog (16개)

| # | Edge Case | 기대 동작 | AC |
|---|-----------|----------|-----|
| 1 | ABAC 미인가 write | 403 `ABAC_CONDITION_DENIED`, store 미호출 | AC-SCORE-API-UBI-004-1, 003-1 |
| 2 | 존재하지 않는 score id GET | 404 표준 에러 | AC-SCORE-API-001-2 |
| 3 | CONFIRMED 행 PUT | 409 `ErrScoreImmutable` | AC-SCORE-API-UBI-004-2 |
| 4 | supersede on SUPERSEDED (terminal 행) | 409 `ErrScoreNotConfirmed` (terminal 정정 거부) | AC-SCORE-API-UBI-004-3 |
| 5 | 입력 검증 실패(blank/over-64/비수치) | 400, TX 미진입 | AC-SCORE-API-002-4 |
| 6 | malformed UUID path | 400, TX 미진입 | AC-SCORE-API-002-5 |
| 7 | malformed JSON body | 400, TX 미진입 | AC-SCORE-API-002-5 |
| 8 | limit > max | max로 clamp | AC-SCORE-API-001-7 |
| 9 | limit 누락/0 | default 적용 | AC-SCORE-API-001-7 |
| 10 | offset 음수/비수치 | 0으로 보정 | AC-SCORE-API-001-7 |
| 11 | authEnabled=false | 전 엔드포인트 투과, cli-anonymous | AC-SCORE-API-UBI-003-1, 003-2 |
| 12 | empty list | `{"scores":[],"count":0}` | AC-SCORE-API-001-6 |
| 13 | grade thresholds 미설정 scope | `ErrGradeThresholdsUnavailable`→404 | AC-SCORE-API-004-1 |
| 14 | supersede on DRAFT (미확정 행) | 409 `ErrScoreNotConfirmed` (DRAFT는 정정 대상 아님 — in-place UpdateScore 경로) | AC-SCORE-API-UBI-004-3 |
| 15 | ServeMux 라우트 충돌(`/{id}` vs `/rollup`·`/grade`·`/supersede`) | 최장 일치로 정확 라우팅 | AC-SCORE-API-001-1, 002-3 |
| 16 | consumer-only 경계 (store/audit/auth/schema 0-diff + API audit 0) | git diff 0, audit INSERT 0 | AC-SCORE-API-BOUNDARY-1, UBI-002-2 |

---

## §9 Definition of Done (Acceptance 단계)

- [ ] Total AC = **27** (UBI-001 2 + UBI-002 2 + UBI-003 2 + UBI-004 3 + 001 7 + 002 5 + 003 3 + 004 3 = 27), 컴포넌트 분해 일치
- [ ] §7 Edge Case Catalog = **16개** 물리적 행, 각 AC 추적
- [ ] 각 REQ ≥2 G/W/T, AC 명명 `AC-SCORE-API-{REQ}-{N}`
- [ ] AC 27 / edge 16 count가 spec.md §9 · spec-compact.md와 cross-file 일관 (count single source of truth = acceptance.md §9 + spec.md §9 + spec-compact.md; plan.md §8은 plan-phase DoD로 count 미운반 — D3-1)
- [ ] consumer-only [HARD] 검증 AC (AC-SCORE-API-BOUNDARY-1: store/audit/auth/errors/evidence_handlers.go/migrations 0-diff, 신규 마이그레이션 0, 자체 audit 0)
- [ ] phantom API 0 — 모든 AC가 source-verified 시그니처(store.go/score.go/errors.go/abac.go/evidence_handlers.go file:line)에 추적
- [ ] 구현 코드/테스트 미작성 (acceptance 문서만)
