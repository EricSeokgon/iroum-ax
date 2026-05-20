# Strategy: SPEC-AX-RUBRIC-001 등급 rubric 확장 시스템 — Run Phase Strategy

**SPEC**: SPEC-AX-RUBRIC-001 v0.1.0 (draft)
**Phase**: Run — Strategy (post Plan, pre TDD RED)
**Generated**: 2026-05-20
**Author**: ircp
**Methodology**: TDD (RED-GREEN-REFACTOR)
**Harness Level**: thorough
**Mode**: sub-agent
**Status**: Draft pending Human Gate sign-off
**Predecessors**: SPEC-AX-SCORE-001 / SCORE-API-001 / EVAL-ITEM-001 / EVID-001 / REPORT-001 / REVIEW-001 / AUTH-003 / CTRL-001 (all GREEN)

---

## 1. §6 OPEN 결정 매트릭스 — 7건 5-element decision

본 §1은 spec.md §6 OPEN 7건에 대한 Run phase 확정 결정이다. 모든 결정은 plan.md / acceptance.md / research.md SSOT와 일관되며, 5요소(결정 / 근거 / 거부대안 / consumer-only impact / Run phase 적용)를 부착한다. 사용자 Human Gate 승인 권장 옵션을 일괄 채택하였으며, 각 결정은 spec.md / plan.md / acceptance.md / 0006 마이그레이션 / 핸들러 ServeMux 라우팅 / EARS 요구사항과 정합한다.

### 1.1 OPEN #1 — Versioning endpoint shape (RESOLVED)

| 요소 | 내용 |
|------|------|
| **Decision** | **Option B 채택**: `POST /api/v1/rubrics/{id}/clone-new-version` sub-resource (별도 엔드포인트). spec.md §3.4 REQ-RUBRIC-003-E1 update endpoint shape의 versioning 의미를 sub-resource로 분리. PUT `/api/v1/rubrics/{id}`는 별도 메타 수정(name/scope/metadata/status)을 담당하며 version 자동 증분은 금지한다. |
| **근거** | (1) REVIEW-001 `POST /reviews/{id}/approve` / `POST /reviews/{id}/reject` 별도 sub-resource supersede 선례(`review_handlers.go:64`) 정확 미러. (2) PUT 멱등성 보존(version 증분 부작용 0). (3) 의도가 명시적(clone-new-version은 "기존 rubric을 베이스로 새 version 생성" 의미 표현). (4) ServeMux Go1.22+ 최장일치에서 구체 경로(`/clone-new-version`) 먼저 등록 가능. (5) 본 SPEC PoC 범위에서 versioning 발생 빈도가 낮아 추가 엔드포인트 수 증가 부담이 작다. |
| **거부대안** | Option A (`PUT /api/v1/rubrics/{id}` 자동 버전 증분): PUT 멱등성 약화(server-side 자동 증분은 client 재시도 시 의도하지 않은 다중 version 생성), 의도 모호(수정인지 새 version인지 클라이언트가 헷갈림), REVIEW-001 sub-resource 선례와 불일치. 거부. |
| **Consumer-only impact** | 0. 본 결정은 본 SPEC 내부 신규 핸들러 메서드(`handleCloneNewVersion`) 추가만 영향. `internal/auth/**/*.go` / SCORE-001 store / migration 0001-0005 모두 0-diff. |
| **Run phase 적용** | (1) `cmd/server/rubric_handlers.go` `Routes()`: `mux.HandleFunc("POST /api/v1/rubrics/{id}/clone-new-version", h.handleCloneNewVersion)` 등록 (구체 경로 우선). (2) `handleCloneNewVersion`: ABAC `guardRubricAdmin` 통과 → `rubricStore.BeginRubricTx` → `GetRubricByID(id)` (NOT archived 검증) → `InsertRubric(name, version+1, scope, metadata, userID)` 동일 TX → audit `RUBRIC_CREATED` (clone은 신규 생성으로 본다, `RUBRIC_CLONED` 별도 액션 신설 0) → Commit → 201 + new rubric JSON. (3) `PUT /api/v1/rubrics/{id}` `handleUpdateRubric`는 version 무관 메타 수정만(name/scope/metadata/status 전이 — OPEN #5 결정과 결합). (4) tasks.md T-RED-020 (TestPOST_RubricsCloneNewVersion_AdminSucceeds) 신설. (5) acceptance.md AC-RUBRIC-003-1 update endpoint 의미 명확화: PUT은 메타 수정 only, version 증분은 clone-new-version sub-resource 책임. |

### 1.2 OPEN #2 — Active rubric 1-per-(name, scope) constraint (RESOLVED)

| 요소 | 내용 |
|------|------|
| **Decision** | **Option A + B 혼합 채택 (이중 방어)**: (a) DB-level partial unique index `CREATE UNIQUE INDEX IF NOT EXISTS rubrics_active_unique_idx ON rubrics(name, scope) WHERE status='active'` + (b) handler-layer state-machine guard pre-check in `handleUpdateRubric` (draft→active 전이 시 기존 active 충돌 검사). REVIEW-001 §6.5/§6.6 dual defense 선례 정확 미러. |
| **근거** | (1) DB-level 보장으로 동시성 race 자동 차단(2개 admin이 동시에 같은 (name,scope) draft를 active로 전이 시 1건만 성공, 나머지는 PostgreSQL unique violation → 핸들러가 409로 매핑). (2) Handler-layer pre-check로 한국어 친화 에러 메시지 제공(DB 위반보다 명확한 사전 검증). (3) REVIEW-001 §6.5 dual defense 패턴 정합(DB CHECK + handler validation 양방향). (4) `(name, scope) WHERE status='active'` partial index만 unique → draft/archived는 자유 다중 row 허용(version별 다중 draft 가능). (5) `0005_score_review_request_tables.sql` partial unique index 패턴 직접 미러 가능. |
| **거부대안** | (1) Option A 단독(DB only): 한국어 에러 메시지 일관성 약화, PoC 단순화 측면 미흡. (2) Option B 단독(handler only): race window 가능성(2개 핸들러 동시 진입 시 둘 다 통과 후 DB INSERT 둘 다 성공). (3) 미설정(unique index 0): active rubric 1-per-(name,scope) 의미적 일관성 깨짐 — `evaluator-active`가 의미 불일치 적발 위험. 모두 거부. |
| **Consumer-only impact** | 0. `0006_rubric_tables.sql` 내부 partial index 추가만 영향. SCORE-001 `0004_score_tables.sql` `grade_thresholds`와 무관. |
| **Run phase 적용** | (1) `0006_rubric_tables.sql`에 `CREATE UNIQUE INDEX IF NOT EXISTS rubrics_active_unique_idx ON rubrics(name, scope) WHERE status='active';` 추가(plan.md §3.3 인라인 주석을 active 라인으로 승격). (2) `handleUpdateRubric` (PUT) draft→active 전이 시: `SELECT COUNT(*) FROM rubrics WHERE name=$1 AND scope=$2 AND status='active' AND id != $3` 사전 검사. count > 0 시 `ErrRubricInvalidStatus` 매핑 → 409 한국어 "동일 name/scope의 active rubric이 이미 존재합니다". (3) 만일 race로 DB unique violation 발생 시 (`pgconn.PgError.Code == "23505"` SQLSTATE), 동일 sentinel `ErrRubricInvalidStatus`로 매핑하여 409 응답(클라이언트 관점 결정성). (4) tasks.md T-RED-021/022 (TestUpdateRubric_DraftToActive_DuplicateActiveBlocks_409 + TestUpdateRubric_RaceConcurrent_UniqueViolationMaps409) 신설. (5) acceptance.md AC-RUBRIC-003-1 확장: 중복 active 차단 시나리오 명시. |

### 1.3 OPEN #3 — Criteria weight sum validation (RESOLVED)

| 요소 | 내용 |
|------|------|
| **Decision** | **Option B 채택 (handler validation pre-store, PoC 단순)**: rubric 단위 weight sum=1.0 강제는 핸들러 단계 사전 검증만 적용. DB CHECK constraint trigger 또는 deferred constraint 0. `AddCriterion` 호출 시 handler가 `GetCriteriaByRubric(rubricID)` → 기존 weight 합 + 신규 weight > 1.0 시 `ErrRubricInvalidInput` 매핑 → 400. 합 = 1.0 정확 강제는 PoC 범위 외 — `<=1.0` 만 강제(partial weight 허용). |
| **근거** | (1) PostgreSQL trigger 또는 deferred constraint 복잡도가 PoC 범위 대비 과도(다중 row 동시 INSERT 시 atomic check 어려움, `INITIALLY DEFERRED CONSTRAINT TRIGGER` 사용 시 복잡한 PL/pgSQL 함수 필요). (2) REVIEW-001 §A.5 dual defense에서 handler validation 단독 채택 선례 일관. (3) 한국어 친화 에러 메시지: "가중치 합이 1.0을 초과합니다 (현재 합: 0.85, 신규: 0.20)". (4) 합=1.0 정확 강제는 partial rubric(점진적 구축) 시나리오에서 비현실적 — admin이 점진적으로 criterion 추가하는 워크플로를 봉쇄. PoC는 `<=1.0` 만 강제하고 최종 1.0 정렬은 admin 책임. (5) DB CHECK는 단일 row constraint만 단순 지원, multi-row aggregate constraint는 미지원 → trigger 외 대안 없음, trigger는 부담. |
| **거부대안** | (1) Option A (DB CHECK constraint trigger): trigger 복잡도 + multi-row atomic check 어려움 + PoC 범위 외. (2) Option C (합 강제 0): 의미적 일관성 약화 — 합 > 1.0 허용 시 가중 합산 무의미. 모두 거부. |
| **Consumer-only impact** | 0. 본 결정은 핸들러 단계 검증만 영향. 0006 마이그레이션 / SCORE-001 store / `internal/auth` 모두 0-diff. |
| **Run phase 적용** | (1) `handleAddCriterion`: `rubricStore.BeginRubricTx` 직전에 `GetCriteriaByRubric(rubricID)` 호출(별도 read-only TX 또는 동일 TX 내부 SELECT) → `sum(existing_weights) + new_weight > 1.0` 검사. 위반 시 `ErrRubricInvalidInput` 매핑 → 400 한국어 "가중치 합이 1.0을 초과합니다". (2) `handleCloneNewVersion`은 weight sum 재검증 불필요(기존 rubric 복제만, criteria 동시 복제는 PoC 범위 외 — clone은 빈 rubric으로 생성, criteria/bands 별도 등록). (3) tasks.md T-RED-023 (TestAddCriterion_WeightSumExceedsOne_400) 신설. (4) acceptance.md AC-RUBRIC-001-2 확장: weight sum > 1.0 거부 시나리오 명시. (5) `internal/errors/errors.go`에 별도 `ErrRubricWeightSumExceedsOne` 센티넬 신설 0 — 기존 `ErrRubricInvalidInput` 재사용(메시지로 구분). |

### 1.4 OPEN #4 — Band overlap prevention (RESOLVED)

| 요소 | 내용 |
|------|------|
| **Decision** | **Option A + B 혼합 채택 (이중 방어)**: (a) PostgreSQL `EXCLUSION USING gist (rubric_id WITH =, numrange(min_score, max_score, '[]') WITH &&)` (DB-level 결정적 차단) + (b) handler-layer pre-validation in `handleAddBand` (한국어 친화 에러 메시지 + race window 추가 방어). 0006 마이그레이션은 `CREATE EXTENSION IF NOT EXISTS btree_gist;`를 멱등 추가. `numrange` 경계 inclusive `'[]'` 명시(min_score/max_score 양 끝 inclusive — acceptance.md AC-RUBRIC-004-1 `[B: 80.0..89.999]` 시나리오 정합). |
| **근거** | (1) DB-level EXCLUSION 결정적 차단으로 동시성 race 자동 봉쇄. (2) `btree_gist` 확장은 PostgreSQL 기본 contrib 모듈 — Neon/Supabase/RDS 모두 지원, 외부 의존 추가 0(go.mod 0-diff, server-side extension only). (3) Handler pre-validation으로 한국어 에러 메시지 일관성("등급 구간이 기존 구간과 겹칩니다 (기존 [80.0, 89.999], 신규 [85.0, 95.0])"). (4) REVIEW-001 §6.5 dual defense 선례 정합. (5) `numrange` 경계 inclusive `'[]'`로 acceptance.md AC-RUBRIC-004-1 시나리오(`B: 80.0..89.999` linear scan match)와 정합(min/max 양 끝 포함). (6) Apply 엔진 결정성↑(score=89.999 정확히 max인 경우 inclusive match). |
| **거부대안** | (1) Option B 단독(handler only): race window 가능성. (2) Option A 단독(DB only): 한국어 에러 메시지 일관성 약화. (3) 미설정(overlap 검증 0): apply 엔진이 첫 매치 band 반환 시 의미 모호. 모두 거부. |
| **Consumer-only impact** | 0. `btree_gist`는 PostgreSQL server-side extension이며 Go 의존성이 아니다 — `go.mod`/`go.sum` 0-diff [HARD]. `CREATE EXTENSION IF NOT EXISTS btree_gist;` 멱등이며 0006 마이그레이션 정방향 적용 + 재실행 안전. SCORE-001 `0004_score_tables.sql` `grade_thresholds`는 별도 모델로 본 결정 무관. |
| **Run phase 적용** | (1) `0006_rubric_tables.sql` 시작부에 `CREATE EXTENSION IF NOT EXISTS btree_gist;` 멱등 추가. (2) `rubric_bands` 테이블 DDL 직후: `DO $$ BEGIN ALTER TABLE rubric_bands ADD CONSTRAINT rubric_bands_no_overlap EXCLUDE USING gist (rubric_id WITH =, numrange(min_score, max_score, '[]') WITH &&); EXCEPTION WHEN duplicate_object THEN NULL; END $$;` 멱등 추가. (3) `handleAddBand`: `rubricStore.BeginRubricTx` 직전에 `GetBandsByRubric(rubricID)` → 기존 bands `numrange` 겹침 검사(Go-side `numrange` 구현 또는 단순 `min_score <= existing.max_score AND max_score >= existing.min_score` 검사). 위반 시 `ErrRubricBandOverlap` → 400. (4) Race로 DB EXCLUSION violation 발생 시(`pgconn.PgError.Code == "23P01"` SQLSTATE), 동일 sentinel `ErrRubricBandOverlap`로 매핑하여 400 응답. (5) tasks.md T-RED-024/025 (TestAddBand_Overlap_HandlerPreCheck_400 + TestAddBand_RaceConcurrent_ExclusionViolationMaps400) 신설. (6) acceptance.md AC-RUBRIC-001-3 / E3 시나리오 확장: dual defense 매핑 명시. |

### 1.5 OPEN #5 — Admin direct edit vs new-version-only policy (RESOLVED)

| 요소 | 내용 |
|------|------|
| **Decision** | **Option A 채택 (admin 직접 편집 허용, PoC 단순)**: `active` 상태 rubric에 대해 admin이 metadata/scope/criteria/bands를 직접 편집 가능. immutable 강제 0. version 증분 필요 시 admin이 명시적으로 OPEN #1 결정(`POST /api/v1/rubrics/{id}/clone-new-version`)을 사용한다. status 전이는 spec.md §3.4 REQ-RUBRIC-003-S2 가드(draft→active, active→archived만 허용)로 통제. |
| **근거** | (1) PoC 범위 단순성 — immutable + clone-only 모델은 UX 복잡도↑ + 엔드포인트 수↑. (2) audit_logs `RUBRIC_UPDATED` 액션 + same TX recording이 audit 추적 보장(spec.md §3.1 UBI-002 정합). (3) 한국 공공 감사 요구는 audit trail로 충족 — 매 update가 별도 audit row + user_id 전파(REVIEW-001 D1 iter2 lesson). (4) admin 책임 분담 명확화 — 의도적으로 immutable이 필요한 경우 admin이 clone-new-version 사용(OPEN #1과 결합). (5) REVIEW-001 / SCORE-API-001은 모두 admin 직접 편집 허용 모델 채택 — 본 SPEC도 일관. |
| **거부대안** | Option B (active immutable + clone-only 강제): UX 복잡도↑ + 엔드포인트 수↑ + PoC 범위 대비 과도. 한국 공공 감사 요구는 audit trail만으로 충족 가능(audit_logs 동일-TX entity+audit + user_id 전파). 거부. |
| **Consumer-only impact** | 0. `internal/auth/**/*.go` / SCORE-001 / migration 0001-0005 모두 0-diff. |
| **Run phase 적용** | (1) `handleUpdateRubric`: ABAC `guardRubricAdmin` 통과 → `BeginRubricTx` → `SELECT ... FOR UPDATE` row lock → `status != 'archived'` 검증(archived terminal) → `validateRubricStatusTransition` (status 변경 요청 시) → UPDATE → audit `RUBRIC_UPDATED` 동일 TX → Commit → 200. (2) `active` 상태 직접 편집 시 `status` 컬럼은 그대로(`active` → `active` no-op 허용, but `RUBRIC_UPDATED` audit row는 생성). (3) tasks.md T-RED-026 (TestUpdateRubric_ActiveDirectEdit_AdminSucceeds_200) 신설. (4) acceptance.md AC-RUBRIC-003-1 확장: active 직접 편집 허용 명시. (5) OPEN #2와 결합: active 직접 편집 시 (name, scope) unique constraint 보장(name/scope 변경으로 기존 active와 충돌 시 차단). |

### 1.6 OPEN #6 — ApplyRubric audit policy (RESOLVED)

| 요소 | 내용 |
|------|------|
| **Decision** | **Option A 채택 (read-only no-audit)**: `ApplyRubric`은 query-like 연산이므로 `audit_logs` 0건. REPORT-001 read-only no-audit 선례 정확 미러. spec.md §3.1 UBI-002 second clause "apply read-only 예외" carve-out 확정. |
| **근거** | (1) REPORT-001 `handleReportScore` / `handleReportEvidence` 모두 read-only no-audit 선례 — 본 SPEC apply도 동형. (2) UBI-002 second clause "apply read-only 예외" 명시적 carve-out — 패턴 일관성. (3) Apply는 점수 → 등급 변환 query-like 연산으로 entity 변경 0 → audit 의미 약함. (4) 감사 추적은 score INSERT 시점(SCORE-001 `RecordScoreCreated`)과 rubric/criteria/bands INSERT 시점(본 SPEC `RecordRubric*`)에서 이미 보장됨 → apply는 그 결과의 read-only 조회. (5) `recorder.go`에 별도 `RecordRubricApplied` 메서드 신설 0 — spec.md §2.1 [MODIFY] `RecordRubric*` 5건과 일관. |
| **거부대안** | Option B (apply audit_logs 행 추가): SCORE-001/REVIEW-001/REPORT-001 패턴과 비대칭(read-only operation에서 audit row 생성). 한국 공공 감사 추가 요구는 score+rubric/criteria/bands INSERT 시점 audit trail로 충족. 거부. |
| **Consumer-only impact** | 0. `recorder.go`에 신규 메서드 신설 0, `audit.go` 신규 Action 상수 신설 0. |
| **Run phase 적용** | (1) `internal/store/rubric.go` `PgRubricTx.ApplyRubric(ctx, rubricID, scoreValue)`: SELECT bands → linear scan → return (letter, band, err). recorder 호출 0 — read-only. (2) `cmd/server/rubric_handlers.go` `handleApplyRubric`: cross-store handler-compose 2-TX (OPEN-independent — TX-1 score sum read-only Rollback + TX-2 rubric apply read-only Rollback). audit row 0건. (3) acceptance.md AC-RUBRIC-004-1 / AC-RUBRIC-004-2 / E10 시나리오 일관 확정(audit_logs row count SHALL not increase). (4) tasks.md T-RED-018 (TestApplyRubric_ReadOnly_NoAuditWritten) genuine RED 테스트 작성 — recorder mock fail-fast assertion(`recorder.Calls()` 검증). (5) plan.md §4.1 RED test #18 unchanged. |

### 1.7 OPEN #7 — Archive reason 필드 작성 조건 (RESOLVED)

| 요소 | 내용 |
|------|------|
| **Decision** | **Option A 채택 (필수, dual defense)**: rubric archive 시 `archive_reason TEXT` 필수. REVIEW-001 `rejection_reason` 패턴 정확 미러 — handler validation pre-store(blank/length 0 거부) + DB CHECK constraint(`(status != 'archived') OR (archive_reason IS NOT NULL AND length(archive_reason) > 0)`). |
| **근거** | (1) REVIEW-001 `rejection_reason` 필수 모델 정확 미러(dual defense — handler + DB CHECK). (2) 한국 공공 감사 요구 강도 — archive 의도 추적 명시화는 의무 수준. (3) audit_logs `RUBRIC_ARCHIVED` 액션이 archive_reason를 audit row payload에 함께 기록(추적 강화). (4) 단순성 약간 손해 → 감사 가치 우위(REVIEW-001 동형 선례에서 검증된 trade-off). (5) DB CHECK는 archive_reason NULL/empty 차단 — handler bypass(예: cli 직접 SQL) 시에도 strong invariant 보장. |
| **거부대안** | Option B (archive_reason optional): archive 의도 추적 어려움 — 한국 공공 감사 약화. REVIEW-001 동형 선례와 불일치. 거부. |
| **Consumer-only impact** | 0. `rubrics` 테이블 신규 `archive_reason TEXT` 컬럼은 본 SPEC 신규 도메인이므로 [EXISTING] 0-diff와 무관. |
| **Run phase 적용** | (1) `0006_rubric_tables.sql` `rubrics` 테이블 DDL에 `archive_reason TEXT` 컬럼 추가(spec.md §2.1 이미 명시) + 멱등 DO$$ CHECK constraint `rubrics_archive_reason_chk`: `CHECK ((status != 'archived') OR (archive_reason IS NOT NULL AND length(archive_reason) > 0))`. (2) `cmd/server/rubric_handlers.go` `handleArchiveRubric`: body `{"archive_reason": "..."}` 필수 — blank/missing 시 `ErrRubricInvalidInput` 매핑 → 400 한국어 "archive_reason은 필수입니다". (3) `internal/store/rubric.go` `ArchiveRubric(ctx, rubricID, archiveReason, userID)` 시그니처에 `archiveReason string` 파라미터 추가 + validation pre-SQL. (4) audit row payload(metadata JSONB)에 `archive_reason` 포함(spec.md §2.1 RecordRubricArchived 시그니처에 archiveReason 파라미터 명시됨). (5) tasks.md T-RED-027/028 (TestArchiveRubric_BlankReason_400 + TestArchiveRubric_DBCheckViolation_500MapsCorrectly) 신설. (6) acceptance.md AC-RUBRIC-003-2 확장: archive_reason 필수 시나리오 명시. |

### 1.8 결정 요약 매트릭스

| OPEN | 결정 | 영향 영역 | 신규 테스트 |
|------|------|----------|------------|
| #1 Versioning | Option B (clone-new-version sub-resource) | rubric_handlers.go Routes + handleCloneNewVersion | T-RED-020 |
| #2 Active unique | Option A+B (partial unique idx + handler check) | 0006 migration + handleUpdateRubric | T-RED-021, T-RED-022 |
| #3 Weight sum | Option B (handler validation pre-store) | handleAddCriterion | T-RED-023 |
| #4 Band overlap | Option A+B (EXCLUSION gist + handler check) | 0006 migration (btree_gist + EXCLUSION) + handleAddBand | T-RED-024, T-RED-025 |
| #5 Active edit | Option A (admin 직접 편집 허용) | handleUpdateRubric | T-RED-026 |
| #6 Apply audit | Option A (read-only no-audit) | rubric.go ApplyRubric (recorder 호출 0) | T-RED-018 (기존 강화) |
| #7 archive_reason | Option A (필수, dual defense) | 0006 migration CHECK + handleArchiveRubric + ArchiveRubric 시그니처 | T-RED-027, T-RED-028 |

---

## 2. 데이터 모델 확정 — `0006_rubric_tables.sql` 최종 DDL

본 §2는 OPEN #2/#4/#7 결정을 적용한 0006 마이그레이션 최종 DDL이다. 0005 멱등 패턴 정확 미러(plan.md §3.3 베이스 + OPEN 결정 인라인 승격). 0001-0005 정확 디스크 확인 후 0006 비충돌.

### 2.1 0006_rubric_tables.sql 최종 DDL (멱등)

```sql
-- 0006_rubric_tables.sql
-- SPEC-AX-RUBRIC-001 — 등급 rubric 확장 시스템 3계층 데이터 모델
-- REVIEW-001 0005 멱등 패턴 정확 미러
-- 의존: 0001-0005 (0001 audit_logs / 0004 grade_thresholds 병행 존재)
-- consumer-only [HARD]: SCORE-001 grade_thresholds 0-diff (병행 존재만)

-- OPEN #4 dual defense: btree_gist extension for EXCLUSION USING gist with numrange
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS rubrics (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(64) NOT NULL,
    version         INT NOT NULL DEFAULT 1,
    scope           VARCHAR(64),
    status          VARCHAR(32) NOT NULL DEFAULT 'draft',
    archive_reason  TEXT,
    metadata        JSONB,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by      VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_by      VARCHAR(64)
);

CREATE TABLE IF NOT EXISTS rubric_criteria (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rubric_id           UUID NOT NULL REFERENCES rubrics(id),
    evaluation_item_id  UUID NOT NULL,
    weight              NUMERIC(5,4) NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rubric_bands (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rubric_id   UUID NOT NULL REFERENCES rubrics(id),
    letter      VARCHAR(8) NOT NULL,
    min_score   NUMERIC(8,4) NOT NULL,
    max_score   NUMERIC(8,4) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- 멱등 CHECK constraints
DO $$ BEGIN
    ALTER TABLE rubrics ADD CONSTRAINT rubrics_status_chk
        CHECK (status IN ('draft', 'active', 'archived'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE rubric_criteria ADD CONSTRAINT rubric_criteria_weight_chk
        CHECK (weight >= 0 AND weight <= 1);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE rubric_bands ADD CONSTRAINT rubric_bands_range_chk
        CHECK (min_score < max_score);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- OPEN #7: archive_reason 필수 (status='archived' 시)
DO $$ BEGIN
    ALTER TABLE rubrics ADD CONSTRAINT rubrics_archive_reason_chk
        CHECK ((status != 'archived') OR (archive_reason IS NOT NULL AND length(archive_reason) > 0));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- OPEN #4 dual defense: EXCLUSION USING gist for band overlap (inclusive boundaries '[]')
DO $$ BEGIN
    ALTER TABLE rubric_bands ADD CONSTRAINT rubric_bands_no_overlap
        EXCLUDE USING gist (rubric_id WITH =, numrange(min_score, max_score, '[]') WITH &&);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- OPEN #2 dual defense: partial unique index for active rubric 1-per-(name, scope)
CREATE UNIQUE INDEX IF NOT EXISTS rubrics_active_unique_idx
    ON rubrics (name, scope) WHERE status = 'active';

-- 일반 인덱스
CREATE INDEX IF NOT EXISTS rubrics_status_idx ON rubrics (status);
CREATE INDEX IF NOT EXISTS rubrics_scope_idx ON rubrics (scope);
CREATE INDEX IF NOT EXISTS rubrics_created_at_idx ON rubrics (created_at DESC);
CREATE INDEX IF NOT EXISTS rubric_criteria_rubric_id_idx ON rubric_criteria (rubric_id);
CREATE INDEX IF NOT EXISTS rubric_bands_rubric_id_idx ON rubric_bands (rubric_id);
```

### 2.2 멱등성 검증 패턴

- `CREATE EXTENSION IF NOT EXISTS btree_gist` — 멱등.
- `CREATE TABLE IF NOT EXISTS` — 멱등.
- `DO $$ BEGIN ALTER TABLE ... ADD CONSTRAINT ...; EXCEPTION WHEN duplicate_object THEN NULL; END $$` — 멱등(0005 정확 미러).
- `CREATE UNIQUE INDEX IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS` — 멱등.
- 정방향 적용 후 재실행 안전(테스트: `psql -f 0006_rubric_tables.sql` × 2 → error 0).

### 2.3 FK 정책 일관

- **내부 FK 허용**: `rubric_criteria.rubric_id REFERENCES rubrics(id)`, `rubric_bands.rubric_id REFERENCES rubrics(id)` — 동일 마이그레이션 0006 내부.
- **FK-less stub**: `rubric_criteria.evaluation_item_id` UUID NOT NULL (REFERENCES 0) — SCORE-001 §1.4 / REVIEW-001 §1.4 동형 정합. 존재 검증은 핸들러 단계 cross-store 별도 TX.

---

## 3. API 엔드포인트 확정 — 9 endpoints + ABAC 매핑 + 응답 schema

본 §3은 OPEN #1/#5/#7 결정을 적용한 최종 엔드포인트 목록이다. ServeMux Go1.22+ 최장일치 — 구체 경로 먼저 등록.

### 3.1 엔드포인트 목록 (9건)

| # | Method | Path | ABAC | 책임 | 응답 |
|---|--------|------|------|------|------|
| 1 | POST | `/api/v1/rubrics` | admin only | rubric 생성 (status='draft' 기본) | 201 + rubric JSON |
| 2 | GET | `/api/v1/rubrics/{id}` | 모든 인증 | 단건 조회 + criteria/bands 임베드 | 200 + rubric JSON |
| 3 | GET | `/api/v1/rubrics` | 모든 인증 | 목록 + filter(status/scope) + pagination | 200 + {items, total} |
| 4 | PUT | `/api/v1/rubrics/{id}` | admin only | 메타 수정 (name/scope/status/metadata) + status 전이 | 200 + rubric JSON |
| 5 | POST | `/api/v1/rubrics/{id}/archive` | admin only | active → archived 전이 + archive_reason 필수 | 200 + rubric JSON |
| 6 | POST | `/api/v1/rubrics/{id}/criteria` | admin only | criterion 추가 + cross-store EvalItem 검증 | 201 + criterion JSON |
| 7 | POST | `/api/v1/rubrics/{id}/bands` | admin only | band 추가 + overlap 검증 | 201 + band JSON |
| 8 | POST | `/api/v1/rubrics/{id}/apply` | 모든 인증 | cross-store 2-TX apply 엔진 | 200 + {letter, band, score_value} |
| 9 | POST | `/api/v1/rubrics/{id}/clone-new-version` | admin only | 신규 version sub-resource (OPEN #1) | 201 + new rubric JSON |

### 3.2 ABAC 매핑 (frozen rbac.go 0-diff)

- **admin only (mutations + clone)**: 1, 4, 5, 6, 7, 9 — 핸들러-로컬 `guardRubricAdmin` 게이트 (REVIEW-001 `guardReviewAdmin` 동형).
- **모든 인증 (read + apply)**: 2, 3, 8 — write-role 게이트 미차용. auth-disabled 투과는 자동(`abac.go:8`).
- **frozen rbac.go**: admin=`RoleAdmin`, 모든 인증=`RoleAdmin|RoleAnalyst|RoleViewer` (`rbac.go:19-26`). `RoleReviewer`/`evaluator` 신설 0 [HARD].

### 3.3 ServeMux 라우팅 (Go 1.22+ 최장일치, 구체 경로 먼저)

```go
func (h *RubricHandler) Routes() http.Handler {
    mux := http.NewServeMux()
    // 구체 경로 먼저 (가장 긴 path-param 경로)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/clone-new-version", h.handleCloneNewVersion)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/apply",             h.handleApplyRubric)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/criteria",          h.handleAddCriterion)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/bands",             h.handleAddBand)
    mux.HandleFunc("POST /api/v1/rubrics/{id}/archive",           h.handleArchiveRubric)
    mux.HandleFunc("PUT  /api/v1/rubrics/{id}",                   h.handleUpdateRubric)
    mux.HandleFunc("GET  /api/v1/rubrics/{id}",                   h.handleGetRubric)
    mux.HandleFunc("GET  /api/v1/rubrics",                        h.handleListRubrics)
    mux.HandleFunc("POST /api/v1/rubrics",                        h.handleCreateRubric)
    return mux
}
```

### 3.4 응답 schema (표준)

- **rubric JSON**: `{"id","name","version","scope","status","archive_reason","created_at","created_by","updated_at","updated_by","metadata","criteria":[...],"bands":[...]}`
- **criterion JSON**: `{"id","rubric_id","evaluation_item_id","weight","created_at"}`
- **band JSON**: `{"id","rubric_id","letter","min_score","max_score","created_at"}`
- **apply JSON**: `{"rubric_id","score_value","letter","band":{"letter","min_score","max_score"}}`
- **list JSON**: `{"items":[...],"total":N}`
- **error JSON**: `{"error":{"code","message","field?"}}` (REVIEW-001/SCORE-API-001 동형)

---

## 4. Apply 엔진 알고리즘 확정 — cross-store handler-compose 2-TX

본 §4는 REQ-RUBRIC-004 (apply 엔진)의 cross-store 2-TX 흐름 + SEC-03 float64 1회 변환 경로를 확정한다. REPORT-001 §6.3 / REVIEW-001 §6.3 선례 정확 미러.

### 4.1 cross-store 2-TX 흐름

```
[Handler]
handleApplyRubric(rubricID, scoreID):
  // ABAC: 모든 인증 통과 (write-role 게이트 미차용)

  // TX-1: SCORE-001 cross-store 점수 합산 (read-only, Rollback 종료)
  scoreTx, _ := scoreStore.BeginScoreTx(ctx)
  defer scoreTx.Rollback(ctx)  // 안전망
  numericSum, err := scoreTx.SumWeightedByEvaluationItem(ctx, scoreID)  // pgtype.Numeric SEC-03 반환
  if err != nil → mapStoreErr(err)
  scoreTx.Rollback(ctx)  // 명시적

  // SEC-03 float64 1회 변환 경로 (REPORT-001 D3-2 lesson 정합)
  scoreValue, ok := numericSum.Float64()  // pgtype.Numeric → float64 단일 변환
  if !ok → 500 INTERNAL  // 변환 실패는 데이터 정합성 결함

  // TX-2: RUBRIC-001 등급 적용 (read-only, Rollback 종료)
  rubricTx, _ := rubricStore.BeginRubricTx(ctx)
  defer rubricTx.Rollback(ctx)  // 안전망
  letter, band, err := rubricTx.ApplyRubric(ctx, rubricID, scoreValue)
  if err != nil → mapRubricStoreErr(err)
  rubricTx.Rollback(ctx)  // 명시적, audit row 0건 보장

  // 200 + apply JSON
  writeRubricJSON(w, 200, {rubricID, scoreValue, letter, band})
```

### 4.2 race window trade-off

- SCORE-001/RUBRIC-001 모두 append-only (UBI-004 / 물리 DELETE 0) → race window 자연 봉쇄.
- archived rubric mutation 차단 → TX-2 시점 `rubric_bands` 안정성 확보.
- PoC 수용 trade-off (REPORT-001 §6.3 / REVIEW-001 §6.3 동형 결정).

### 4.3 fail-closed 경로

- `scoreID` 미존재: SCORE-001 `SumWeightedByEvaluationItem`이 `ErrScoreNotFound` 반환 → 404.
- `rubricID` 미존재: `ApplyRubric` 내부 `GetRubricByID` 단계에서 `ErrRubricNotFound` 반환 → 404.
- `scoreValue` 모든 band 밖: `ApplyRubric` 마지막 단계에서 `ErrRubricInvalidInput` 반환 → 400 (acceptance.md AC-RUBRIC-004-3).
- SEC-03 float64 변환 실패: 500 INTERNAL + ERROR log.

### 4.4 SEC-03 float64 1회 변환 (REPORT-001 D3-2 lesson 정합)

- `pgtype.Numeric.Float64()` 단일 변환 경로 사용.
- 다중 변환 (`.Numeric.AssignTo(&f)` + `.Float64()`) 금지 — precision loss 위험.
- 변환 실패 시 명시적 500 + ERROR log (silent fallback 금지).

---

## 5. D1 iter2 lesson pre-application 검증 체크리스트

본 §5는 REVIEW-001 D1 iter2 lesson을 처음부터(iter1) 정확 적용하기 위한 의무 체크리스트이다. iter1 fix → iter2 사이클 비재발이 목표.

### 5.1 3가지 동시 적용 의무 사항

| # | 적용 위치 | 정확 코드 | 위반 시 결과 |
|---|----------|----------|------------|
| 1 | `internal/store/pg_store.go` `BeginRubricTx` | `recorder: audit.NewRecorder(true)` | `false` 시 user_id 영구 'cli-anonymous' override → UBI-003 Must-Pass Firewall 위반 → evaluator iter1 FAIL |
| 2 | `internal/store/rubric.go` 5 mutation 메서드 시그니처 | `func (t *PgRubricTx) InsertRubric(ctx, ..., userID string) (...)` <br> `func (t *PgRubricTx) UpdateRubric(ctx, ..., userID string) (...)` <br> `func (t *PgRubricTx) ArchiveRubric(ctx, ..., archiveReason string, userID string) (...)` <br> `func (t *PgRubricTx) AddCriterion(ctx, ..., userID string) (...)` <br> `func (t *PgRubricTx) AddBand(ctx, ..., userID string) (...)` | 누락 시 SQL INSERT의 `created_by`/`updated_by` 값이 hard-coded 'cli-anonymous' → AC-RUBRIC-UBI-003 FAIL |
| 3 | `cmd/server/rubric_handlers.go` 핸들러 메서드 | `userID := resolveCreatedBy(r.Context())` 호출 후 store TX 메서드에 명시 전파 | 누락 시 store 시그니처는 정확해도 핸들러가 ID 미전달 → 효과 없음 |

### 5.2 SQL `$N` 바인딩 정확성

- `INSERT INTO rubrics (name, ..., created_by, updated_by) VALUES ($1, ..., $N, $N+1) RETURNING id`
- `INSERT INTO audit_logs (resource_id, action, ..., user_id) VALUES ($1, $2, ..., $N)` — user_id에 userID 정확 바인딩
- placeholder 미정렬 / 정수 hardcode (예: `VALUES (..., 'cli-anonymous')`) 금지

### 5.3 handler resolveCreatedBy 헬퍼

```go
// REVIEW-001 D1 iter2 동형 — auth.UserFromContext 미사용 시 'cli-anonymous' fallback
func resolveCreatedBy(ctx context.Context) string {
    if u, ok := auth.UserFromContext(ctx); ok && u.ID != "" {
        return u.ID
    }
    return "cli-anonymous"
}
```

- `auth.UserFromContext`: AUTH-003 미들웨어가 주입.
- auth-disabled 모드(`AUTH_ENABLED=false`): `UserFromContext` returns `ok=false` → 'cli-anonymous' fallback.
- auth-enabled 모드: principal.id 정확 전파.

### 5.4 명시적 검증 테스트 (T-RED-005, T-RED-019 — tasks.md §2 RED 참조)

- T-RED-005 `TestInsertRubric_UserIDPropagatesToCreatedByAndAuditUserID`: store-layer 단위 테스트. fake recorder + fake pgx.Tx. assertion: `created_by='user-42'` + `updated_by='user-42'` + `audit_logs.user_id='user-42'` 모두 정확.
- T-RED-019 `TestNewRecorderTrue_AuthEnabledUserIDPropagates`: `pg_store.BeginRubricTx` 호출 후 `recorder.IsEnabled() == true` 직접 검증 + auth-enabled principal context로 end-to-end 시나리오에서 user_id 정확 전파.

---

## 6. Frozen 0-diff manifest — Drift-Guard 검증 명령 (orchestrator 직접 grep)

본 §6은 consumer-only [HARD] 0-diff 검증 명령을 명시한다. **orchestrator는 매 phase(RED 완료 / GREEN 완료 / REFACTOR 완료 / M5 최종) 후 직접 `git diff --quiet -- <frozen scope>` 검증한다. teammate 자가보고만 신뢰 금지** (consumer-only orchestrator grep 패턴, lesson `feedback_consumer_only_orchestrator_grep` 적용).

### 6.1 frozen scope (절대 수정 금지, 0-diff)

```
# SCORE-001/SCORE-API-001/EVAL-ITEM-001/EVID-001/REPORT-001/REVIEW-001/AUTH-003 코드
apps/control-plane/internal/store/score.go
apps/control-plane/internal/store/eval_item.go
apps/control-plane/internal/store/evidence.go
apps/control-plane/internal/store/score_review_request.go
apps/control-plane/cmd/server/score_handlers.go
apps/control-plane/cmd/server/report_handlers.go
apps/control-plane/cmd/server/review_handlers.go
apps/control-plane/cmd/server/evidence_handlers.go

# frozen RBAC/ABAC (RoleReviewer/evaluator 신설 0)
apps/control-plane/internal/auth/rbac.go
apps/control-plane/internal/auth/abac.go
apps/control-plane/internal/auth/middleware.go
apps/control-plane/internal/auth/authz_middleware.go
apps/control-plane/internal/auth/chain.go

# migration (0006만 신규, 0001-0005 무수정)
.moai/db/schema/migrations/0001_workflows.sql
.moai/db/schema/migrations/0002_evidence.sql
.moai/db/schema/migrations/0003_evaluation_items.sql
.moai/db/schema/migrations/0004_score_tables.sql   # SCORE-001 grade_thresholds 병행 존재 [HARD]
.moai/db/schema/migrations/0005_score_review_request_tables.sql

# 외부 의존 (신규 추가 0)
apps/control-plane/go.mod
apps/control-plane/go.sum
```

### 6.2 매 phase orchestrator 직접 검증 명령

```bash
# Frozen scope 0-diff 검증 (orchestrator가 실행, exit 0 = PASS)
git diff --quiet -- \
  apps/control-plane/internal/store/score.go \
  apps/control-plane/internal/store/eval_item.go \
  apps/control-plane/internal/store/evidence.go \
  apps/control-plane/internal/store/score_review_request.go \
  apps/control-plane/cmd/server/score_handlers.go \
  apps/control-plane/cmd/server/report_handlers.go \
  apps/control-plane/cmd/server/review_handlers.go \
  apps/control-plane/cmd/server/evidence_handlers.go \
  apps/control-plane/internal/auth/ \
  .moai/db/schema/migrations/0001_workflows.sql \
  .moai/db/schema/migrations/0002_evidence.sql \
  .moai/db/schema/migrations/0003_evaluation_items.sql \
  .moai/db/schema/migrations/0004_score_tables.sql \
  .moai/db/schema/migrations/0005_score_review_request_tables.sql \
  apps/control-plane/go.mod \
  apps/control-plane/go.sum
# Exit code: 0=PASS (0 diff), 1=FAIL (drift 발생 → 즉시 중단)
```

### 6.3 [MODIFY] 정확 범위 검증

- `internal/store/store.go`: 신규 인터페이스 `RubricStore`/`RubricTx` + 신규 struct `Rubric`/`RubricCriterion`/`RubricBand` 추가만(기존 인터페이스/struct 무변경) — `git diff` 검사 시 추가만 표시.
- `internal/store/pg_store.go`: 신규 메서드 `BeginRubricTx` 추가만 — 특히 `audit.NewRecorder(true)` 호출 라인 정확 명시.
- `internal/audit/audit.go`: 신규 상수 5건만(`ActionRubricCreated`/`ActionRubricUpdated`/`ActionRubricArchived`/`ActionRubricCriterionAdded`/`ActionRubricBandAdded`).
- `internal/audit/recorder.go`: 신규 메서드 5건만(`RecordRubricCreated`/`RecordRubricUpdated`/`RecordRubricArchived`/`RecordRubricCriterionAdded`/`RecordRubricBandAdded`). `RecordRubricApplied` 신설 0 (OPEN #6 read-only no-audit).
- `internal/errors/errors.go`: **신규 센티넬 7건만**(`ErrRubricNotFound`/`ErrRubricInvalidInput`/`ErrRubricInvalidStatus`/`ErrRubricArchived`/`ErrRubricWeightOutOfBounds`/`ErrRubricBandOverlap`/`ErrRubricAuditWriteFailed`). **SCORE-API-001 errors.go drift lesson — spec.md §2.1 + §2.3 양쪽 부착 EXPLICIT [HARD]**.
- `cmd/server/server.go`: **≈7줄만** (`rubricH *RubricHandler` 필드 1줄 + 생성자 1줄 + ko 주석 1줄 + `innerMux.Handle` 2줄 + 상단 ko 주석 1-2줄). **REPORT-001 lesson — "1줄" 표기 금지 [HARD]**.

### 6.4 위반 발생 시 대응

- drift 발견 → 즉시 중단 (M5 단계 의무).
- spec.md §2.3 R-CONSUMER-001 발동.
- 재계획: 위반 파일 원복 → drift 원인 분석 → 의도된 변경이면 spec.md 갱신, 의도 아니면 코드 fix.

---

## 7. Risk Register — R-RUBRIC-001 ~ R-RUBRIC-010

본 §7은 plan.md §5에 명시된 R-RUBRIC-001 ~ R-RUBRIC-005에 추가하여, OPEN 결정 적용으로 발생한 신규 리스크 5건을 확장한다.

### 7.1 R-RUBRIC-006 (Medium) — `btree_gist` extension 환경 의존

**Risk**: PoC 또는 prod 환경에서 PostgreSQL `btree_gist` extension이 비활성화되었거나 권한 부족으로 `CREATE EXTENSION` 실패 → 0006 마이그레이션 실패 → 본 SPEC 전체 차단.

**Mitigation**:
- `btree_gist`는 PostgreSQL 표준 contrib 모듈 — RDS/Neon/Supabase/일반 PostgreSQL 모두 지원.
- `CREATE EXTENSION IF NOT EXISTS` 멱등 — 이미 활성화 시 무시.
- PoC 환경(testcontainers) postgres:16 이미지에 `postgres-contrib` 포함 확인.
- 권한 부족 시 fallback: handler-layer overlap check만으로 운영(OPEN #4 Option B 단독) — 운영 가이드 문서화.
- M4 통합 테스트에서 EXCLUSION constraint 동작 명시적 검증 → 환경 의존 조기 감지.

### 7.2 R-RUBRIC-007 (Medium) — `numrange` inclusive boundary 의미 정확성

**Risk**: `numrange(min, max, '[]')` inclusive 경계 사용 시 `acceptance.md AC-RUBRIC-004-1` 시나리오(`B: 80.0..89.999`)의 max=89.999 정확 매치가 의도된 행동인지 확실. 만일 `(80.0, 89.999]` half-open으로 의도되었다면 score=80.0이 B 대신 C로 매치되어 등급 결과 불일치.

**Mitigation**:
- acceptance.md AC-RUBRIC-004-1 시나리오에서 `score=85.5 → letter='B'` 검증으로 inclusive 의도 명시.
- `0006_rubric_tables.sql` `numrange(min, max, '[]')` inclusive 명시 + 주석으로 의도 기록.
- `ApplyRubric` Go-side linear scan: `min_score <= scoreValue <= max_score` (inclusive 정확 일치).
- T-RED-016 `TestApplyRubric_ScoreInBand_ReturnsLetter` 경계 정확성 검증 (score=80.0 → B, score=89.999 → B, score=90.0 → A 명시 케이스).

### 7.3 R-RUBRIC-008 (Low) — `clone-new-version` clone 깊이 모호

**Risk**: OPEN #1 결정 `clone-new-version` 엔드포인트가 (a) 빈 rubric 신규 생성(criteria/bands 별도 등록) vs (b) 기존 criteria/bands 모두 복제 중 어느 의미인지 모호.

**Mitigation**:
- 본 SPEC 결정: **(a) 빈 rubric 신규 생성**만 채택. clone은 metadata + name + scope만 복제. criteria/bands는 admin이 별도 등록.
- 이유: PoC 단순성 + clone 시 weight sum 재검증 부담 회피 + REVIEW-001 supersede 동형 단순화.
- tasks.md T-RED-020 명시: clone 후 criteria/bands 모두 0건 가정.
- 만일 deep clone 필요 시 post-PoC 별도 SPEC에서 다룬다.

### 7.4 R-RUBRIC-009 (Low) — PUT 메타 수정 시 status 전이 분리 (OPEN #1+#5 결합)

**Risk**: OPEN #1+#5 결정으로 PUT은 메타 수정 + status 전이 둘 다 담당하나, status 전이만 시도 vs 메타 변경만 시도 vs 둘 다 시도 중 어느 시나리오에서 audit row가 단일/다중 생성되는지 모호.

**Mitigation**:
- 본 SPEC 결정: **단일 PUT → 단일 audit row** (`RUBRIC_UPDATED` 액션 1건). status 전이 + 메타 변경 동시 발생해도 audit row 1건.
- audit row payload(metadata JSONB)에 변경된 모든 필드 기록 → 추적 가능성 유지.
- acceptance.md AC-RUBRIC-003-1 명시.

### 7.5 R-RUBRIC-010 (Low) — partial unique index race 와 EXCLUSION violation SQLSTATE 매핑

**Risk**: OPEN #2/#4 dual defense에서 DB violation 발생 시 (`23505` partial unique / `23P01` EXCLUSION) 정확한 sentinel 매핑이 필요. 잘못 매핑 시 500 (서버 결함) 또는 400/409 (클라이언트 오류) 응답 의미 모호.

**Mitigation**:
- `mapRubricStoreErr`: `pgconn.PgError` 직접 감지 → SQLSTATE 분기.
  - `23505` (unique violation) + index name `rubrics_active_unique_idx` → `ErrRubricInvalidStatus` (409 한국어 "동일 name/scope의 active rubric이 이미 존재합니다").
  - `23P01` (exclusion violation) + constraint name `rubric_bands_no_overlap` → `ErrRubricBandOverlap` (400 한국어 "등급 구간이 기존 구간과 겹칩니다").
  - 기타 SQLSTATE → 500 INTERNAL (ERROR log).
- T-RED-022 / T-RED-025: 명시적 race 시나리오 테스트.

---

## 8. 다음 단계 (Post-Strategy)

1. **Human Gate sign-off**: 본 strategy.md 7 OPEN 결정 매트릭스 사용자 최종 승인.
2. **/clear → /moai run SPEC-AX-RUBRIC-001**: TDD RED-GREEN-REFACTOR 사이클 진입.
3. **tasks.md 참조**: M0 RED → M1 GREEN store → M2 GREEN handler → M3 server.go 마운트 → M4 REFACTOR → M5 Drift-Guard.
4. **매 phase orchestrator 직접 grep**: §6 Drift-Guard 검증 명령 의무 실행.
5. **harness=thorough**: evaluator-active 4-차원 ≥0.85 + manager-quality TRUST 5 PASS.
6. **lesson 부착**: 본 SPEC 완료 후 D1 iter2 + SCORE-API-001 errors.go drift + REPORT-001 server.go ≈7줄 모두 pre-applied된 사례로 lesson DB 업데이트.

---

## 9. 참조

- spec.md §1.4 consumer-only [HARD] / §1.5 ABAC narrowing / §2.3 Drift-Guard manifest / §6 OPEN 7건
- plan.md §1.1 핵심 전략 11건 / §2 마일스톤 M0-M5 / §3.3 0006 마이그레이션 / §4 TDD RED-GREEN-REFACTOR / §5 R-RUBRIC-001-005
- acceptance.md AC 25 + edge 16 (Given-When-Then)
- research.md §1 REVIEW-001 수직 슬라이스 선례 / §2 SCORE-001 grade_thresholds 병행
- SPEC-AX-REVIEW-001 (commit 89a8828, gold standard mirror target)
- SPEC-AX-REPORT-001 (cross-store 2-TX + ≈7줄 server.go 선례)
- SPEC-AX-SCORE-API-001 (errors.go drift lesson + OPEN #4 evaluator-INFEASIBLE lesson)
- `.claude/skills/moai/workflows/plan.md` L378 (canonical 8-field frontmatter schema)
- `.claude/rules/moai/core/moai-constitution.md` (TRUST 5 + Agent Core Behaviors)
- Memory: `feedback_consumer_only_orchestrator_grep` / `feedback_dark_flow_iter2_pattern` / `project_pg_store_recorder_bool`
