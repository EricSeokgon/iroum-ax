# SPEC-AX-RUBRIC-001 (Compact) — 등급 rubric 확장 시스템 (3-tier Rubric+Criteria+Bands Store + HTTP API + Apply Engine)

> v0.1.0 · status draft · TDD · thorough harness · sub-agent · brownfield. 상세는 spec.md / plan.md / acceptance.md / research.md. (AC=25, edge=16, §6 OPEN=7, 본 SPEC=수직 슬라이스 store+audit+HTTP+ABAC+apply 엔진 단일 SPEC)

## 핵심 stub 계약 [HARD]

- **3계층 데이터 모델**: `rubrics`(부모) + `rubric_criteria`(자식, `evaluation_item_id` **FK-less stub** to EvalItem, SCORE-001 §1.4 동형) + `rubric_bands`(자식, `letter`+`min_score`+`max_score`). `rubric_criteria.rubric_id` / `rubric_bands.rubric_id` REFERENCES `rubrics(id)` — **내부 FK 허용**(동일 마이그레이션 0006 내, REVIEW-001 외부 FK-less와 다른 동일-마이그레이션 내부 FK 패턴)
- `rubrics.status` = state-machine enum `draft|active|archived` (DB CHECK + store-level `validateRubricStatusTransition` 이중 방어). `archived` = terminal(모든 mutation 차단, `ErrRubricArchived` 409). 정정 = 신규 `draft` INSERT만, 물리 DELETE 0
- **SCORE-001 `grade_thresholds`(0004) 0-diff [HARD] 병행 존재**: SCORE-001 단일 테이블 모델 + RUBRIC-001 `rubric_bands` rubric별 다중 모델 = 공존, 호출자가 어느 모델 선택. `Score.DetermineGrade`(`store.go:296-298`) 호출 0건
- SCORE-001/SCORE-API-001/EVAL-ITEM-001/EVID-001/REPORT-001/REVIEW-001/AUTH-003 코드·스키마·FK·permissionMatrix·핸들러 = **0-diff [HARD] consumer-only**
- 신규 마이그레이션 = `0006_rubric_tables.sql` 1건 (0001~0005 디스크 확인 후 비충돌, 0005 REVIEW-001 멱등 패턴 정확 미러)
- 동일-TX audit (mutations만, 5종) — SCORE-001/REVIEW-001 D2/D4 패턴 미러: resource_id=entity UUID 직접 (surrogate 미사용). `ApplyRubric`은 read-only이므로 audit 0건 (§6 OPEN #6 Option A 채택 시 확정)
- **REVIEW-001 D1 iter2 lesson pre-applied [HARD] (3가지 동시 적용 — iter1 fix 사이클 비재발)**:
  - `pg_store.go BeginRubricTx`는 `audit.NewRecorder(true)`로 호출 (**`false` 절대 금지**)
  - store TX mutation 메서드 5종은 명시적 `userID string` 파라미터 (처음부터 도입)
  - 핸들러 `resolveCreatedBy(r.Context())` → principal.id or `'cli-anonymous'` → store에 일관 전파
- frozen `rbac.go` 3-role (`admin`/`analyst`/`viewer`) **0-diff** — **`RoleReviewer`/`evaluator` 신설 절대 금지** (SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시, REVIEW-001 §1.5 동형 자연 회피 — admin/모든 인증 매핑 모두 rbac.go에 존재)
- Apply 엔진 **cross-store handler-compose 2-TX** (REPORT-001 §6.3 / REVIEW-001 §6.3 정확 미러): TX-1 read-only `scoreStore.BeginScoreTx`→`SumWeightedByEvaluationItem`(`store.go:292-295` `pgtype.Numeric` SEC-03)→Rollback, TX-2 read-only `rubricStore.BeginRubricTx`→`ApplyRubric`→Rollback. 결과 letter 반환. Race window는 append-only로 결정적 不發生
- 1차 산출물 = store 도메인 신설 + 동일-TX audit + 6+ HTTP 엔드포인트 + 핸들러-로컬 ABAC narrowing + read-only apply 엔진

## REQ 모듈 (UBI 4 + modal 21)

| REQ-ID | EARS | 요지 |
|--------|------|------|
| REQ-RUBRIC-UBI-001 (데이터 주권) | Ubiquitous | 외부 호출 0건 (외부 LLM/SaaS/CDN/secrets manager 모두 0), 단일 내부 PostgreSQL pgx pool만 |
| REQ-RUBRIC-UBI-002 (감사 가능성 — mutations 동일-TX, apply read-only 예외) | Ubiquitous | mutation 5종 → 동일 TX `audit_logs` 1건 (resource_id=UUID 직접, namespace 0), `ApplyRubric` audit 0건 (read-only), 핸들러 self-audit 0 (이중 감사 금지) |
| REQ-RUBRIC-UBI-003 (cli-anonymous + auth-disabled fallback + userID 명시 전파) | Ubiquitous(State) | AuthN off → `created_by`/`updated_by`/`audit_logs.user_id`=`'cli-anonymous'` literal. AuthN on → store mutation `userID string` 파라미터로 principal.id 정확 전파 (**REVIEW-001 D1 iter2 lesson HARD**), `NewRecorder(true)` 검증 |
| REQ-RUBRIC-UBI-004 (권한 narrowing + 3-state 불변식 + archived terminal) | Ubiquitous | (1) ABAC narrowing: admin=mutations / 모든 인증 incl. viewer=read+apply, frozen rbac.go 0-diff (RoleReviewer/evaluator 신설 0); (2) 3-state allowed transitions only (`draft→active→archived`, terminal 불변); (3) archived rubric mutation 거부 (`ErrRubricArchived` 409); 물리 DELETE 0 |
| REQ-RUBRIC-001-E1/E2/E3/S1/U1/O1 | E/E/E/S/U/O | Store + 3계층 데이터 모델 + 마이그레이션: 동일-TX entity+audit + userID 전파(E1), criterion 추가 cross-store EvalItem stub(E2/S1), band 추가(E3), blank/length/weight/min<max 거부(U1), metadata opaque(O1) |
| REQ-RUBRIC-002-E1/E2/E3/E4/E5/U1 | E/E/E/E/E/U | HTTP CRUD + sub-resource: admin create(E1), 단건 조회 with embedded criteria/bands(E2), 목록 + filter + pagination clamp(E3), criterion 추가 + cross-store EvalItem 검증(E4), band 추가 + overlap check(E5), malformed UUID 거부(U1) |
| REQ-RUBRIC-003-E1/E2/S1/S2 | E/E/S/S | HTTP transitions: draft→active update(E1), active→archived terminal with archive_reason(E2), archived mutation 거부(S1), 불법 전이 fail-closed(S2) |
| REQ-RUBRIC-004-E1/E2/U1 | E/E/U | Apply 엔진: store-layer ApplyRubric read-only no-audit(E1), HTTP cross-store handler-compose 2-TX(E2), score out of all bands fail-closed(U1) |
| REQ-RUBRIC-005-E1/U1 | E/U | 에러 매핑: 결정적 store→HTTP 매핑(E1, 7 센티넬 + unknown), raw pgx 누출 금지(U1) |

## AC (25: AC-RUBRIC-{REQ}-{N})

- §0 UBI: AC-RUBRIC-UBI-001(외부호출 0), -002(audit 동일 TX 원자 + apply read-only 0), -003(cli-anonymous + auth-disabled 투과 + **userID 명시 전파 검증** + **`NewRecorder(true)` 검증**), -004(권한 narrowing + 3-state 불변식 + archived terminal + 물리 DELETE 0)
- §1 REQ-RUBRIC-001: AC-RUBRIC-001-1(entity+audit 원자 생성 + userID 전파), -2(criterion 추가 + cross-store stub), -3(band 추가 entity+audit), -4(FK-less stub EvalItem + 내부 FK rubric_id), -5(blank/length/weight/min<max 거부), -6(metadata opaque round-trip)
- §2 REQ-RUBRIC-002: AC-RUBRIC-002-1(admin create 201), -2(단건 조회 + 자식 임베드 + 404), -3(목록 + filter + pagination + 빈 결과 200), -4(criterion 추가 sub-resource + cross-store EvalItem 404), -5(band 추가 sub-resource + overlap), -6(malformed UUID 400)
- §3 REQ-RUBRIC-003: AC-RUBRIC-003-1(draft→active update), -2(archive active→archived terminal), -3(archived mutation 거부), -4(불법 전이 모두 거부 fail-closed)
- §4 REQ-RUBRIC-004: AC-RUBRIC-004-1(store ApplyRubric read-only no-audit), -2(HTTP cross-store 2-TX end-to-end), -3(score out of all bands fail-closed)
- §5 REQ-RUBRIC-005: AC-RUBRIC-005-1(7 센티넬 + unknown 결정적 매핑 + 한국어 메시지), -2(raw pgx.ErrNoRows 누출 0)
- 분해 합 = 4+6+6+4+3+2 = 25 (heading) → 통합 카운트 4+6+6+4+3+2 = **25 heading, 25 unique AC** (UBI 4 + modal REQ 21). §6 edge=16. 각 modal REQ ≥2 AC. modal REQ 21 = REQ-RUBRIC-001(E1/E2/E3/S1/U1/O1=6) + 002(E1/E2/E3/E4/E5/U1=6) + 003(E1/E2/S1/S2=4) + 004(E1/E2/U1=3) + 005(E1/U1=2).

## Files

| 경로 | Delta |
|------|-------|
| `internal/store/rubric.go` | [NEW] PgRubricTx pgx (eval_item.go/score_review_request.go 정확 미러) + 11 메서드(Insert/Get/List/Update/Archive + AddCriterion/AddBand/GetCriteriaByRubric/GetBandsByRubric/ApplyRubric/InsertAuditLog) + validation 4종(rubric/criterion/band/statusTransition) + **모든 mutation 메서드 `userID string` 파라미터** (REVIEW-001 D1 iter2 lesson pre-applied) |
| `internal/store/rubric_test.go` | [NEW] store-layer 단위 테스트 — validation 가드 4종 + ApplyRubric 알고리즘 + fake recorder 격리 |
| `internal/store/rubric_integration_test.go` | [NEW] testcontainers — 0006 마이그레이션 + BeginRubricTx end-to-end + 동일-TX audit 원자성 + SELECT FOR UPDATE 동시성 |
| `cmd/server/rubric_handlers.go` | [NEW] RubricHandler + 3-store 의존(`rubricStore`/`evalItemStore`/`scoreStore`) + Routes() + 8+ 핸들러 + 핸들러-로컬 ABAC + apply 엔진 cross-store 2-TX (review_handlers.go 정확 미러) |
| `cmd/server/rubric_handlers_test.go` | [NEW] httptest 22+ test (CRUD/생명주기/ABAC/cross-store/edge/audit 원자 + **D1 iter2 lesson 검증 명시**) |
| `.moai/db/schema/migrations/0006_rubric_tables.sql` | [NEW] 3개 테이블 멱등 SQL (0005 정확 미러), CHECK 상태 enum + weight bound + min<max + 옵션 partial unique idx(§6 OPEN #2 결정 후) + 옵션 EXCLUSION USING gist(§6 OPEN #4 Option A 결정 후), 인덱스 5개 |
| `internal/store/store.go` | [MODIFY] RubricStore/RubricTx 인터페이스 + Rubric/RubricCriterion/RubricBand struct 3개 (ScoreReviewRequestStore/Tx 패턴 정확 미러) |
| `internal/store/pg_store.go` | [MODIFY] BeginRubricTx — **`audit.NewRecorder(true)` HARD** (REVIEW-001 D1 iter2 lesson, `false` 절대 금지), PgWorkflowStore.pool 재사용 |
| `internal/audit/audit.go` | [MODIFY] ActionRubric{Created,Updated,Archived,CriterionAdded,BandAdded} 상수 **5개만** (namespace 0, D2) |
| `internal/audit/recorder.go` | [MODIFY] RecordRubric{Created,Updated,Archived,CriterionAdded,BandAdded} 메서드 **5개만** (local AuditTx, resource_id=UUID 직접). ApplyRubric은 read-only이므로 RecordRubricApplied 없음 |
| `internal/errors/errors.go` | [MODIFY] 센티넬 **7개만** 추가 (Not/InvalidInput/InvalidStatus/Archived/WeightOutOfBounds/BandOverlap/AuditWriteFailed) — **SCORE-API-001 errors.go drift 교훈 — manifest §2.1+§2.3 양쪽 EXPLICIT 부착 (이중 부착으로 분실 방지)** |
| `cmd/server/server.go` | [MODIFY] **≈7줄** (필드+생성+innerMux.Handle 2줄+ko 주석, REVIEW-001 :55-57/:210-213/:266-267/:269-270 + SCORE-API-001 :55/:210/:266-267 + REPORT-001 :56/:212/:269-270 정확 미러, **REPORT-001 lesson — "1줄" 잘못 기술 0**) |
| `internal/store/score.go`(특히 `SumWeightedByEvaluationItem`/`DetermineGrade`) / `eval_item.go` / `evidence.go` / `score_review_request.go`(REVIEW-001) / `cmd/server/{score,report,review,evidence}_handlers.go` / `internal/auth/**/*.go` / `0001`–`0005`(특히 `0004` `grade_thresholds`) / `go.mod` | [EXISTING] **0-diff [HARD] consumer-only** — 특히 SCORE-001 `grade_thresholds` 병행 존재 0-diff, frozen rbac.go(RoleReviewer/evaluator 신설 0) |

## Exclusions (What NOT to Build)

1. **SCORE-001 `grade_thresholds`(0004) 0건 수정 (병행 존재만)** — RUBRIC-001 `rubric_bands`와 공존, SCORE-001 0-diff [HARD], `Score.DetermineGrade` 호출 0건
2. **SCORE-001 / SCORE-API-001 / EVAL-ITEM-001 / EVID-001 / REPORT-001 / REVIEW-001 / AUTH-003 코드·스키마 수정** — consumer-only [HARD], 마이그레이션 0001~0005 0-diff
3. **Simulation 엔진 (rubric simulation_runs 4계층)** — PoC=3계층(rubric+criteria+bands), 4번째 simulation_runs는 post-PoC
4. **LLM 기반 rubric 자동 생성/추천** — 외부 호출 0 (REQ-RUBRIC-UBI-001 정합)
5. **AUTH-003 모델 초과 풀 org-unit 속성 ABAC** — 조직단위 narrowing 미구현
6. **6번째 한국 공공 제약 (시간 제약 — KST 업무시간 한정 mutations)** — 의도적 제외 (SCORE-API-001/REPORT-001/REVIEW-001 동형)
7. **Multi-org rubric 조율 / cross-organization rubric sharing** — PoC 단일 조직 범위
8. **Evaluator consensus engine (다중 평가자 합의 alg)** — PoC 단일 평가자 모델
9. **물리 삭제 / hard delete** — append-only (UBI-004 정합), archived 이후에도 행 보존
10. **rubric simulation UI / dashboard** — HTTP API만, UI는 post-PoC

## §6 OPEN (strategy phase 결정 — sub-agent + Human Gate, 7건)

1. **Versioning endpoint shape**: A `PUT /rubrics/{id}` 자동 버전 증분 vs B `POST /rubrics/{id}/clone-new-version` 별도 sub-resource. **권장**: B (REVIEW-001 supersede 선례 정합, 명시적 의도, PUT 멱등성 보존).
2. **Active rubric 1-per-(name, scope) constraint**: A PostgreSQL partial unique index `WHERE status='active'` (DB 보장) vs B 핸들러-layer check + state-machine 가드. **권장**: A+B 이중 방어 (REVIEW-001 §6.5 dual defense 선례).
3. **Criteria weight sum validation**: A DB CHECK constraint (sum=1.0 trigger 복잡) vs B 핸들러 validation pre-store (REVIEW-001 §A.5 dual defense). **권장**: B (PoC 단순, 한국어 친화 에러). C(합=1.0 강제 없음)은 PoC 외.
4. **Band overlap prevention**: A PostgreSQL `EXCLUSION USING gist + numrange &&` (DB-level 결정적) vs B 핸들러 validation (`AddBand` 기존 bands 로드 후 numrange 겹침 검사). **권장**: A+B 이중 방어 (REVIEW-001 §6.5 dual defense 선례).
5. **Admin direct edit vs new-version-only policy**: A `active` 직접 수정 허용 (단순) vs B `active` immutable + clone-new-version 강제 (불변, 감사 추적 강화). **권장**: A (PoC 단순성). B는 한국 공공 감사 요구 강도 결정 후.
6. **ApplyRubric audit policy**: A read-only no-audit (REPORT-001 선례 정합) vs B application-level audit_logs 추가 (한국 공공 감사 보수적). **권장**: A (REPORT-001 read-only no-audit 일관성). 본 SPEC §3 EARS는 권장안 A 기준 작성.
7. **Archive reason 필드 작성 조건**: A 필수 (REVIEW-001 `rejection_reason` 패턴 dual defense — handler validation + DB CHECK) vs B optional (단순). **권장**: A (한국 공공 감사 추적 강화).

> 본 OPEN 7건은 **SCORE-API-001 §6 OPEN #4(write 역할 매핑 `evaluator` 부재) 상속하지 않음** — 본 SPEC의 역할 매핑(admin=mutations / 모든 인증 incl. viewer=read+apply)은 frozen rbac.go에 모두 존재(`rbac.go:19-26`)하므로 0-diff 자연 성립.

## Drift-Guard Manifest Highlights (REVIEW-001 / SCORE-API-001 / REPORT-001 lessons 적용)

- **errors.go 부착 정확 (SCORE-API-001 교훈)**: [MODIFY] 7 센티넬 추가를 manifest §2.1/§2.3 **양쪽 명시 부착** (drift detection 분실 방지) — 본 SPEC 의무 검증 항목
- **server.go ≈7줄 정확 (REPORT-001 교훈)**: "1줄"이 아닌 ≈7줄(필드+생성+마운트 2줄+ko 주석, REVIEW-001 :55-57/:210-213/:266-267/:269-270 정확 라인 인용)
- **D1 iter2 lesson pre-applied (REVIEW-001 교훈, 3가지 동시 적용 — iter1 fix 사이클 비재발)**:
  - `pg_store.go BeginRubricTx` → `audit.NewRecorder(true)` HARD (false 절대 금지)
  - store TX mutation 메서드 → `userID string` 파라미터 처음부터 도입
  - 핸들러 `resolveCreatedBy(ctx)` → store로 일관 전파
- **frozen rbac.go 0-diff (SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시)**: 본 SPEC은 RoleReviewer/evaluator 신설 절대 금지, admin/모든 인증 매핑 frozen rbac.go에 모두 존재 → 충돌 자연 회피 (REVIEW-001 §1.5 동형)
- **SCORE-001 grade_thresholds 병행 존재 0-diff [HARD]**: SCORE-001 단일 테이블 모델(`0004` `grade_thresholds`, `Score.DetermineGrade` `store.go:296-298`)과 RUBRIC-001 `rubric_bands` rubric별 다중 모델 공존, SCORE-001 0-diff 유지, `DetermineGrade` 호출 0건
- **phantom API 0 (lesson #9)**: 모든 [NEW] 메서드 spec.md §2.1 명시, 모든 [EXISTING] 호출 source-verified 라인 인용 (`SumWeightedByEvaluationItem` `store.go:292-295`, `BeginScoreTx` `pg_store.go:134`, `RoleAdmin` `rbac.go:19-26`, `ErrCodeABACDenied` `abac.go:24` 등)
- **consumer-only [HARD]**: SCORE-001/SCORE-API-001/EVAL-ITEM-001/EVID-001/REPORT-001/REVIEW-001/AUTH-003 모두 0-diff 검증 (M5 git diff)

> phantom 0 (lesson #9), errors.go drift 부착 (SCORE-API-001 lesson), server.go ≈7줄 정확 (REPORT-001 lesson), **D1 iter2 lesson pre-applied (REVIEW-001 lesson — NewRecorder(true) + userID string 파라미터, iter1 fix 사이클 비재발)**, frozen rbac.go 0-diff (SCORE-API-001 §6 OPEN #4 evaluator-INFEASIBLE 비재발 명시), grade_thresholds 0-diff 병행 존재. SSOT: research.md.
