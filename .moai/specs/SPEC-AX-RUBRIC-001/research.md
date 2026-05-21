# Research: SPEC-AX-RUBRIC-001 — 등급 rubric 확장 시스템

**Phase**: 0.5 Deep Research
**Generated**: 2026-05-20
**Author**: Explore (read-only codebase analysis, prose summary materialized by orchestrator per plan-mode scratch pattern)
**Status**: complete
**Scope**: SPEC-AX-RUBRIC-001 EARS/Plan/Tasks 의 SSOT (모든 주장 file:line 근거)

> RUBRIC-001은 EVAL-ITEM-001/SCORE-001이 명시적으로 이연한 풀 grade rubric 시스템. REVIEW-001(2026-05-20 커밋 536d7dd/89a8828) 수직 슬라이스 패턴을 정확 미러하되 3계층 데이터 모델(rubrics/criteria/bands)과 버전화를 추가한다.

---

## 1. REVIEW-001 수직 슬라이스 선례 (최우선 미러 대상)

REVIEW-001은 2026-05-20 GAN iter2 PASS 92.8/TRUST5 PASS로 완료된 가장 최근 store+API 수직 슬라이스. RUBRIC-001은 이를 정확 미러한다.

### 1.1 핵심 파일 인벤토리 (source-verified)

| 영역 | 파일 | 라인 |
|------|------|------|
| Store struct + methods | `apps/control-plane/internal/store/score_review_request.go` | 전체 (PgScoreReviewRequestTx, validate*, 4 mutation methods) |
| TX entry + Recorder 주입 | `apps/control-plane/internal/store/pg_store.go` | 157-172 (BeginScoreReviewRequestTx + `audit.NewRecorder(true)` L171 — D1 iter2 fix) |
| Audit Action 상수 | `apps/control-plane/internal/audit/audit.go` | 76-84 (4건: CREATED/REVIEWER_ASSIGNED/APPROVED/REJECTED) |
| Audit Recorder 메서드 | `apps/control-plane/internal/audit/recorder.go` | 445-520 (4건 RecordScoreReviewRequest*, local AuditTx, resource_id=UUID 직접) |
| Error sentinel | `apps/control-plane/internal/errors/errors.go` | (확장 6건 ErrScoreReviewRequest*) |
| HTTP handler | `apps/control-plane/cmd/server/review_handlers.go` | 1-86 (ReviewHandler + Routes + handler-local ABAC) |
| ABAC 핸들러-로컬 게이트 | `apps/control-plane/cmd/server/review_handlers.go` | 209-243 (requireReviewAdminRole/requireReviewSubmitRole/guardReviewAdmin/guardReviewSubmit) |
| Server.go 마운트 | `apps/control-plane/cmd/server/server.go` | 55-57(field) / 210-213(constructor) / 266-267/269-270(2-line subtree mount) — ≈7줄 |
| Migration | `.moai/db/schema/migrations/0005_score_review_request_tables.sql` | 멱등 패턴(CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION duplicate_object + CHECK status enum + CHECK rejection_reason) |

### 1.2 D1 iter2 핵심 lesson (RUBRIC가 처음부터 적용해야 함)

REVIEW-001 iter1에서 `pg_store.go BeginScoreReviewRequestTx`가 `audit.NewRecorder(false)`를 호출 → auth-enabled에서도 모든 userID가 'cli-anonymous'로 강제 override → AC-REVIEW-UBI-003 Must-Pass Firewall 위반 → evaluator-active iter1 FAIL 73.5. iter2 fix:
1. `NewRecorder(true)` 전환
2. Store TX mutation methods 시그니처에 `userID string` 파라미터 추가
3. Handler에서 `resolveCreatedBy(r.Context())` 결과를 store TX 메서드로 전파
4. SQL INSERT/UPDATE의 created_by/updated_by가 `$N` placeholder 바인딩으로 정확 영속

**RUBRIC-001은 iter1부터 이 lesson을 사전 적용한다** — `BeginRubricTx`가 처음부터 `audit.NewRecorder(true)`, mutation methods 시그니처에 userID 파라미터, SQL `$N` 바인딩. iter1 FAIL→iter2 fix 사이클 비재발.

---

## 2. SCORE-001 grade_thresholds 병행 관계 (consumer-only [HARD])

### 2.1 SCORE-001 grade_thresholds 구조 (source-verified, frozen)

| 항목 | 위치 | 내용 |
|------|------|------|
| Table DDL | `.moai/db/schema/migrations/0004_score_tables.sql:52-70` | `grade_thresholds(scope VARCHAR(64), letter VARCHAR(2), min_value DECIMAL(6,2), boundary_rule VARCHAR(8) DEFAULT 'gte', PK(scope,letter))` + CHECK(letter ∈ {S,A,B,C,D}, boundary_rule ∈ {gte,gt}) |
| DetermineGrade 메서드 | `apps/control-plane/internal/store/score.go:591-648` | `DetermineGrade(scope, score float64) (string, error)` — S→D 내림차순 스캔, 첫 매치 letter 반환, scope 0행 → `ErrGradeThresholdsUnavailable` fail-closed |
| Error sentinel | `apps/control-plane/internal/errors/errors.go:67` | `ErrGradeThresholdsUnavailable` |
| 인터페이스 정의 | `apps/control-plane/internal/store/store.go:296-298` | `DetermineGrade(ctx, scope string, score float64) (string, error)` on ScoreTx |

### 2.2 RUBRIC vs grade_thresholds 차이

| 특성 | grade_thresholds (SCORE-001) | bands (RUBRIC-001) |
|------|------------------------------|--------------------|
| 데이터 모델 | 2컬럼(scope/letter/min_value/boundary_rule) | 3계층(rubric→criteria→bands), bands는 min_value + **max_value** 둘 다 |
| 매칭 알고리즘 | min_value 내림차순 스캔, 첫 ≥ 매치 (상한 무한) | min_value ≤ score ≤ max_value 범위 매칭(구간) |
| 버전화 | 없음(scope당 단일 letter 집합) | rubric.version 명시(이력 추적) |
| 가중치 | 없음 | criteria.weight (가중 평가) |
| 메타데이터 | boundary_rule만 | rubric.metadata JSONB, criteria.metadata, bands.description |

**병행 존재 의미**: RUBRIC는 SCORE-001 grade_thresholds를 대체하지 않고 **상위 계층으로 공존**한다. SCORE-001 코드/스키마/0004 마이그레이션 **0-diff** 유지(consumer-only [HARD]). `DetermineGrade`는 단순 임계값 매핑, `ApplyRubric`는 풀 rubric 평가 — 둘 다 letter S/A/B/C/D 반환으로 일관.

---

## 3. 3계층 데이터 모델 설계 (신규 0006 마이그레이션)

### 3.1 테이블 구조

```sql
CREATE TABLE IF NOT EXISTS rubrics (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name VARCHAR(128) NOT NULL,
  version INT NOT NULL DEFAULT 1,
  scope VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',  -- active | archived
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by VARCHAR(128) NOT NULL DEFAULT 'cli-anonymous',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by VARCHAR(128) NOT NULL DEFAULT 'cli-anonymous'
);
-- UNIQUE(name, version, scope) 필수 (버전 식별)
-- §6 OPEN #2: active 1-per-(name,scope) — partial unique index vs handler check

CREATE TABLE IF NOT EXISTS criteria (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  rubric_id UUID NOT NULL REFERENCES rubrics(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  weight DECIMAL(5,4) NOT NULL,
  order_idx INT NOT NULL,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by VARCHAR(128) NOT NULL DEFAULT 'cli-anonymous'
);
-- CHECK (weight >= 0 AND weight <= 1)
-- §6 OPEN #3: weight sum == 1.0 강제 여부

CREATE TABLE IF NOT EXISTS bands (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  rubric_id UUID NOT NULL REFERENCES rubrics(id) ON DELETE CASCADE,
  letter VARCHAR(2) NOT NULL,  -- S | A | B | C | D
  min_value DECIMAL(6,2) NOT NULL,
  max_value DECIMAL(6,2) NOT NULL,
  description TEXT,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by VARCHAR(128) NOT NULL DEFAULT 'cli-anonymous'
);
-- CHECK (letter IN ('S','A','B','C','D'))
-- CHECK (min_value <= max_value)
-- §6 OPEN #4: band overlap 방지 — EXCLUSION USING gist vs handler validation
```

### 3.2 멱등 패턴 (0005 정확 미러)

- `CREATE TABLE IF NOT EXISTS` (모든 테이블)
- `DO $$ BEGIN ALTER TABLE ... ADD CONSTRAINT ... CHECK (...); EXCEPTION WHEN duplicate_object THEN NULL; END $$;` (모든 CHECK)
- `CREATE INDEX IF NOT EXISTS rubrics_status_idx ON rubrics(status)`
- `CREATE INDEX IF NOT EXISTS criteria_rubric_id_idx ON criteria(rubric_id)`
- `CREATE INDEX IF NOT EXISTS bands_rubric_id_letter_idx ON bands(rubric_id, letter)`
- FK ON DELETE CASCADE (rubric 삭제 시 자식 자동 정리; 단 PoC는 archive only — 물리 DELETE 0 정책)

---

## 4. ApplyRubric 알고리즘

### 4.1 입력/출력

- **입력**: `(ctx, rubricID uuid.UUID, scoreValue float64)`
- **출력**: `(letter string, band *Band, err error)`

### 4.2 알고리즘

```go
// ApplyRubric — rubric의 bands에 scoreValue를 매칭해 letter+band 반환
// SCORE-001 DetermineGrade와 다른 점: 상한+하한 구간 매칭 (DetermineGrade는 상한만)
// 1. rubric 존재 + status='active' 검증 (archived rubric은 ErrRubricInactive)
// 2. SELECT bands WHERE rubric_id=$1 ORDER BY min_value DESC
// 3. for band in bands: if scoreValue >= band.min_value AND scoreValue <= band.max_value: return band
// 4. no match → ErrRubricBandNotFound (fail-closed, fabricate 금지)
```

### 4.3 SEC-03 정밀도 (SCORE-001 lesson)

- 핸들러가 SCORE-001 `SumWeightedByEvaluationItem`(`store.go:292-295`) 결과 `pgtype.Numeric`를 받음
- `pgtype.Numeric` → `float64` **1회 변환** (DetermineGrade store 계약과 동형 — score.go:298 시그니처)
- 핸들러는 `ApplyRubric(ctx, rubricID, scoreValueFloat64)` 호출
- ApplyRubric 내부는 `DECIMAL(6,2)` 비교 (SQL 단계, float64 미경유)
- SEC-03 "float64 미경유" 범위는 N-항 누적 경로 — 단일 입력 변환은 무관 (REPORT-001 D3-2 lesson)

---

## 5. HTTP API 엔드포인트 (§6 OPEN 후보)

### 5.1 설계 후보 (review_handlers.go 패턴 미러)

```
POST   /api/v1/rubrics                          — 생성 (admin)
GET    /api/v1/rubrics                          — 목록 (filter: status, scope)
GET    /api/v1/rubrics/{id}                     — 단건 (criteria+bands embed)
PUT    /api/v1/rubrics/{id}                     — 수정 (active draft) — §6 OPEN #5
POST   /api/v1/rubrics/{id}/archive             — 아카이브 (active→archived terminal)
POST   /api/v1/rubrics/{id}/clone-new-version   — 새 버전 (§6 OPEN #1)
POST   /api/v1/rubrics/{id}/criteria            — criterion 추가 (admin)
POST   /api/v1/rubrics/{id}/bands               — band 추가 (admin)
POST   /api/v1/rubrics/{id}/apply               — 적용 body{score_value}→{letter,band} (read-like, 모든 인증)
```

### 5.2 ABAC 매핑 (handler-local, frozen rbac.go 0-diff)

| 작업 | 역할 게이트 |
|------|------------|
| 생성·수정·아카이브·버전·criterion·band 추가 | `RoleAdmin` only (`requireRubricAdminRole`) |
| 조회·apply | 모든 인증 사용자 + auth-disabled 투과 |

**SCORE-API-001 OPEN#4 evaluator-INFEASIBLE 자연 회피**: admin이 frozen rbac.go에 실재(rbac.go:19-26), `RoleReviewer`/`evaluator` 같은 신규 역할 신설 0. `score_handlers.go:163` evaluator-INFEASIBLE 주석 lesson 정합.

---

## 6. 동일-TX Audit 패턴 (UBI-002)

### 6.1 audit.go Action 상수 5건 (REVIEW-001 audit.go:76-84 미러)

```go
ActionRubricCreated      Action = "RUBRIC_CREATED"
ActionRubricUpdated      Action = "RUBRIC_UPDATED"
ActionRubricArchived     Action = "RUBRIC_ARCHIVED"
ActionRubricCriterionAdded Action = "RUBRIC_CRITERION_ADDED"
ActionRubricBandAdded    Action = "RUBRIC_BAND_ADDED"
```

### 6.2 recorder.go Record 메서드 5건 (REVIEW-001 recorder.go:445-520 미러)

- `RecordRubricCreated(ctx, tx, rubricID, name, version, userID)`
- `RecordRubricUpdated(ctx, tx, rubricID, userID)`
- `RecordRubricArchived(ctx, tx, rubricID, userID)` (+ archive_reason §6 OPEN #7)
- `RecordRubricCriterionAdded(ctx, tx, criterionID, rubricID, userID)`
- `RecordRubricBandAdded(ctx, tx, bandID, rubricID, letter, userID)`

D2 패턴 미러: `resource_id = rubricID/criterionID/bandID UUID PK 직접 대입` (surrogate 0, namespace 추가 0).

### 6.3 ApplyRubric은 audit 0 (§6 OPEN #6 권장 A)

`ApplyRubric`은 read-only query — SCORE-001 `DetermineGrade`/REPORT-001 read endpoints와 동형, audit 0. UBI-002 "all mutations audit"의 carve-out (read-only는 예외 명시).

---

## 7. 한국 공공 6제약 정합 (REQ-RUBRIC-UBI)

| UBI | 한국 공공 6제약 | RUBRIC-001 적용 |
|-----|--------------|-----------------|
| UBI-001 데이터 주권 | 외부 호출 0, 신규 외부 의존 0 | go.mod/go.sum 0-diff, pgx/uuid/zap만 사용 |
| UBI-002 감사 가능성 | 모든 mutation 동일-TX audit | RecordRubric* 5건 동일 TX, apply는 read-only carve-out |
| UBI-003 cli-anonymous | auth-disabled fallback | NewRecorder(true) + userID 파라미터, 'cli-anonymous' default |
| UBI-004 권한·상태 불변 | ABAC narrowing + state-machine | admin write, all-authenticated read+apply; status archived terminal |

6번째 시간 제약은 의도적 §5 Exclusion (KST 업무시간 제약은 PoC 범위 밖).

---

## 8. SCORE-001 점수 적용 cross-store 조합 (REPORT-001 §6.3 선례)

`POST /api/v1/rubrics/{id}/apply` 핸들러 흐름:

1. **TX-1 (read-only, SCORE)**: `scoreStore.BeginScoreTx` → `SumWeightedByEvaluationItem(evalItemID)` → `pgtype.Numeric` → `float64` 변환 → Rollback (no Commit)
2. **TX-2 (read-only, RUBRIC)**: `rubricStore.BeginRubricTx` → `ApplyRubric(rubricID, scoreValueFloat64)` → `(letter, band)` 반환 → Rollback

SCORE-001 0-diff (SumWeightedByEvaluationItem 호출만), RUBRIC store도 read-only TX. 동일 패턴이 REPORT-001 §6.3 OPEN #1 RESOLVED 선례.

---

## 9. Drift Guard & Manifest 정확성 (SCORE-API-001/REPORT-001/REVIEW-001 세션 누적 lesson)

### 9.1 errors.go [MODIFY] Drift-Guard 명시 부착 (SCORE-API-001 lesson)

7건 신규 sentinel (errors.go:52-78 패턴 미러):
- `ErrRubricNotFound`
- `ErrRubricInvalidInput`
- `ErrRubricInvalidStatus`
- `ErrRubricInactive`
- `ErrCriterionInvalidWeight`
- `ErrBandOverlap`
- `ErrRubricAuditWriteFailed`

§2.1 [MODIFY] 표 + §2.3 Drift-Guard manifest 양쪽에 EXPLICIT 부착 (이중 부착으로 분실 방지).

### 9.2 server.go ≈7줄 (REPORT-001 lesson)

`server.go [MODIFY]`: 필드 1줄 + 생성자 주석 1줄 + 생성자 1줄 + 마운트 주석 1줄 + `innerMux.Handle("/api/v1/rubrics", ...)` 1줄 + `innerMux.Handle("/api/v1/rubrics/", ...)` 1줄 + ko 주석 1줄 = **정확히 ≈7줄**. "1줄" 잘못 기술 금지.

### 9.3 frozen 0-diff 강제 검증 (모든 session lesson)

[EXISTING] consumer-only: SCORE-001/EVAL-ITEM-001/SCORE-API-001/REPORT-001/REVIEW-001/AUTH-003 + score_handlers/report_handlers/review_handlers/evidence_handlers + frozen rbac.go(rbac.go:19-26/:33/:68-80)/abac.go/authz_middleware/chain/middleware + grade_thresholds(0004 미수정) + go.mod/go.sum + 0001-0005 마이그레이션 모두 `git diff` 0 line.

---

## 10. Strategy Phase OPEN 결정 후보 (7건)

1. **Versioning endpoint shape**: `POST /rubrics/{id}/clone-new-version` (sub-resource, REVIEW-001 supersede 선례) vs PUT 자동 버전 증분 — 권장 A(sub-resource)
2. **Active rubric 1-per-(name,scope) 제약**: PostgreSQL partial unique index `WHERE status='active'` vs handler-layer check + state-machine guard — 권장 A+B 이중 방어
3. **Criteria weight sum validation**: DB CHECK (constraint trigger 복잡) vs handler validation pre-store — 권장 B (PoC 단순)
4. **Band overlap prevention**: PostgreSQL `EXCLUSION USING gist` (range type) vs handler validation — 권장 A+B 이중 방어
5. **Admin direct edit policy**: active rubric 직접 수정 허용 vs immutable + clone-new-version 강제 — 권장 A (PoC 단순)
6. **ApplyRubric audit policy**: read-only no-audit (REPORT-001 선례) vs application audit_logs 추가 — 권장 A
7. **Archive reason field**: REVIEW-001 rejection_reason 패턴(필수) vs optional — 권장 A (감사 강화)

---

## 11. Risks & Implicit Contracts

- **HARD: consumer-only of SCORE-001 grade_thresholds**: 병행 존재만, 0004 마이그레이션·DetermineGrade·grade_thresholds 테이블 수정 0
- **HARD: frozen RBAC 0-diff**: admin/analyst/viewer 3-role 고정, RoleReviewer/evaluator 신설 INFEASIBLE
- **HARD: 신규 외부 의존 0**: go.mod/go.sum 0-diff, pgx/uuid/zap만
- **HARD: 신규 마이그레이션 0006만**: 0001-0005 무수정
- **HARD: NewRecorder(true) + userID 파라미터 사전 적용**: REVIEW-001 D1 iter2 lesson, iter1 FAIL→iter2 fix 사이클 비재발
- **암묵**: rubric.letter ↔ grade_thresholds.letter 동일 enum(S/A/B/C/D) 일관성, ApplyRubric/DetermineGrade 둘 다 letter 반환
- **암묵**: ApplyRubric는 read-only — UBI-002 carve-out, SCORE-001 DetermineGrade/REPORT-001 read 패턴 동형
- **strategy 불확실성**: versioning shape(§6 OPEN #1), active-1 제약(§6 OPEN #2), weight sum 정책(§6 OPEN #3), band overlap 정책(§6 OPEN #4), edit-vs-clone 정책(§6 OPEN #5), apply audit(§6 OPEN #6), archive reason(§6 OPEN #7)

---

## 12. Recommended Implementation Approach

**최소 vertical slice**: 3계층 테이블(rubrics/criteria/bands) + RubricStore/Tx 인터페이스 + PgRubricTx 구현(NewRecorder(true) + userID 파라미터 사전 적용) + 9 HTTP 엔드포인트(생성/목록/조회/수정/아카이브/클론/criterion 추가/band 추가/apply) + 동일-TX audit 5건(apply 제외) + handler-local ABAC (admin write / all-authenticated read+apply).

SCORE-001/grade_thresholds 병행 0-diff. REVIEW-001 패턴 정확 미러로 구조적 위험 최소.

복합 도메인 `SPEC-AX-RUBRIC-001`. 의존: SPEC-AX-SCORE-001(grade_thresholds 모델 참조) + SPEC-AX-AUTH-003(ABAC) + REVIEW-001(store+API 수직 슬라이스 lesson). thorough harness.

Strategy 결정 7건 + Run phase Human Gate sign-off 후 M0 RED 진입 가능.
