# Tasks: SPEC-AX-RUBRIC-001 등급 rubric 확장 시스템 — Decomposed Task List

**SPEC**: SPEC-AX-RUBRIC-001 v0.1.0 (draft)
**Phase**: Run — Tasks (post Strategy, pre TDD RED)
**Generated**: 2026-05-20
**Author**: ircp
**Methodology**: TDD (RED-GREEN-REFACTOR)
**Harness Level**: thorough
**Total Tasks**: 53 (RED 33 + GREEN 12 + REFACTOR 5 + Verification 3)

---

## 1. 마일스톤 매핑 (Plan M0-M5 → Tasks)

| Plan Milestone | Tasks | Tasks 개수 |
|----------------|-------|-----------|
| M0 RED — 테스트 골격 + 인터페이스 정의 | T-RED-001 ~ T-RED-033 + T-IFACE-001 ~ T-IFACE-003 | 36 |
| M1 GREEN — 0006 마이그레이션 + store 구현 | T-GREEN-001 ~ T-GREEN-006 | 6 |
| M2 GREEN — HTTP API 핸들러 + ABAC + apply 엔진 | T-GREEN-007 ~ T-GREEN-011 | 5 |
| M3 GREEN — server.go 마운트 + 통합 | T-GREEN-012 | 1 |
| M4 REFACTOR — 품질 보강 + TRUST 5 | T-REFACTOR-001 ~ T-REFACTOR-005 | 5 |
| M5 Drift-Guard 검증 | T-VERIFY-001 ~ T-VERIFY-003 | 3 |

---

## 2. RED Tasks — 33 failing tests + 3 인터페이스 골격

본 §2는 plan.md §4.1에 열거된 33개 RED 테스트를 task ID로 분해한다. 각 task는 (a) 테스트 파일 + 함수명, (b) 검증 대상 REQ-ID, (c) 예상 fail 시그니처를 명시한다. **모든 테스트는 GREEN 구현 전에 작성되어 실패 확인되어야 한다** (post-hoc rubber-stamp 금지 — SCORE-001/REVIEW-001 dark-flow iter1 적발 패턴 회피).

### 2.1 인터페이스 골격 (선행 task, 컴파일 통과만)

| Task ID | 파일 | 책임 | 의존 |
|---------|------|------|------|
| T-IFACE-001 | `internal/store/store.go` [MODIFY] | `RubricStore`/`RubricTx` 인터페이스 + `Rubric`/`RubricCriterion`/`RubricBand` struct 3개 추가 (REVIEW-001 `ScoreReviewRequestStore`/`Tx`/`ScoreReviewRequest` 패턴 미러, fieldalignment 정렬) | 없음 |
| T-IFACE-002 | `internal/store/rubric.go` [NEW] | `PgRubricTx` struct + 11 메서드 빈 구현 스켈레톤 (모든 mutation 메서드는 처음부터 `userID string` 파라미터 포함 — **REVIEW-001 D1 iter2 lesson pre-applied**). 컴파일만 통과. | T-IFACE-001 |
| T-IFACE-003 | `cmd/server/rubric_handlers.go` [NEW] | `RubricHandler` struct + `NewRubricHandler` + `Routes()` + 9 핸들러 메서드 빈 스켈레톤. 컴파일만 통과. | T-IFACE-001 |

### 2.2 store-layer RED Tests (T-RED-001 ~ T-RED-019)

| Task ID | 테스트 함수 | 파일 | REQ-ID 검증 | 예상 fail 시그니처 |
|---------|------------|------|-------------|------------------|
| T-RED-001 | `TestInsertRubric_ValidInput_ReturnsUUIDAndInsertsAuditRow` | `internal/store/rubric_test.go` | REQ-RUBRIC-001-E1 + UBI-002 | InsertRubric 미구현 → nil UUID 또는 panic |
| T-RED-002 | `TestInsertRubric_BlankName_ReturnsInvalidInput` | `rubric_test.go` | REQ-RUBRIC-001-U1 | validateRubricInput 미구현 → nil 또는 panic |
| T-RED-003 | `TestInsertRubric_NameOver64Chars_ReturnsInvalidInput` | `rubric_test.go` | REQ-RUBRIC-001-U1 | 동일 |
| T-RED-004 | `TestInsertRubric_AuditFailure_TwoWayRollback` | `rubric_test.go` | UBI-002 | fake recorder가 ErrRubricAuditWriteFailed 반환 시 양방향 rollback 검증 미구현 |
| T-RED-005 | `TestInsertRubric_UserIDPropagatesToCreatedByAndAuditUserID` | `rubric_test.go` | UBI-003 + **REVIEW-001 D1 iter2 lesson** | `userID="user-42"` 파라미터 미구현 시 'cli-anonymous' hardcode 검출 |
| T-RED-006 | `TestAddCriterion_ValidInput_ReturnsUUIDAndInsertsAudit` | `rubric_test.go` | REQ-RUBRIC-001-E2 | AddCriterion 미구현 |
| T-RED-007 | `TestAddCriterion_WeightOutOfBounds_ReturnsWeightOutOfBounds` | `rubric_test.go` | REQ-RUBRIC-001-U1 | validateCriterionInput 미구현 |
| T-RED-008 | `TestAddCriterion_ArchivedRubric_ReturnsArchived` | `rubric_test.go` | REQ-RUBRIC-003-S1 | archived rubric guard 미구현 |
| T-RED-009 | `TestAddBand_ValidInput_ReturnsUUIDAndInsertsAudit` | `rubric_test.go` | REQ-RUBRIC-001-E3 | AddBand 미구현 |
| T-RED-010 | `TestAddBand_MinGteMax_ReturnsInvalidInput` | `rubric_test.go` | REQ-RUBRIC-001-U1 | validateBandInput 미구현 |
| T-RED-011 | `TestUpdateRubric_DraftToActive_TransitionsSuccess` | `rubric_test.go` | REQ-RUBRIC-003-E1 + UBI-004 | UpdateRubric + transition guard 미구현 |
| T-RED-012 | `TestUpdateRubric_ArchivedRubric_ReturnsArchived` | `rubric_test.go` | REQ-RUBRIC-003-S1 | archived terminal guard 미구현 |
| T-RED-013 | `TestArchiveRubric_ActiveToArchived_TransitionsSuccess` | `rubric_test.go` | REQ-RUBRIC-003-E2 + OPEN #7 | ArchiveRubric + archive_reason 필수 검증 미구현 |
| T-RED-014 | `TestArchiveRubric_Terminal_PersistsAfter` | `rubric_test.go` | UBI-004 | archived → 다른 전이 시도 ErrRubricInvalidStatus 미구현 |
| T-RED-015 | `TestStatusTransition_AllAllowedAndDisallowed` | `rubric_test.go` | UBI-004 + REQ-RUBRIC-003-S2 | validateRubricStatusTransition 완전성 미구현 |
| T-RED-016 | `TestApplyRubric_ScoreInBand_ReturnsLetter` | `rubric_test.go` | REQ-RUBRIC-004-E1 | ApplyRubric 알고리즘 미구현. 경계 inclusive 정확성(80.0/89.999/90.0) 검증. |
| T-RED-017 | `TestApplyRubric_ScoreOutOfAllBands_ReturnsInvalidInput` | `rubric_test.go` | REQ-RUBRIC-004-U1 | fail-closed 미구현 |
| T-RED-018 | `TestApplyRubric_ReadOnly_NoAuditWritten` | `rubric_test.go` | UBI-002 second clause + **OPEN #6** | recorder mock fail-fast: `recorder.Calls() == 0` assertion 미구현 |
| T-RED-019 | `TestNewRecorderTrue_AuthEnabledUserIDPropagates` | `rubric_test.go` | UBI-003 + **REVIEW-001 D1 iter2 lesson** | `BeginRubricTx` 후 `recorder.IsEnabled() == true` + principal context user_id 전파 검증 |

### 2.3 HTTP handler RED Tests (T-RED-020 ~ T-RED-033)

| Task ID | 테스트 함수 | 파일 | REQ-ID / OPEN | 예상 fail 시그니처 |
|---------|------------|------|---------------|------------------|
| T-RED-020 | `TestPOST_RubricsCloneNewVersion_AdminSucceeds_201` | `cmd/server/rubric_handlers_test.go` | **OPEN #1** | handleCloneNewVersion 미구현 → 404 또는 405 |
| T-RED-021 | `TestPOST_Rubrics_AdminCreates_201` | `rubric_handlers_test.go` | REQ-RUBRIC-002-E1 + AC-RUBRIC-002-1 | handleCreateRubric 미구현 |
| T-RED-022 | `TestPOST_Rubrics_ViewerForbidden_403` | `rubric_handlers_test.go` | UBI-004 + AC E6 | guardRubricAdmin 미구현 |
| T-RED-023 | `TestPOST_Rubrics_AnalystForbidden_403` | `rubric_handlers_test.go` | UBI-004 + AC E7 | analyst rejection 미구현 |
| T-RED-024 | `TestGET_Rubrics_ViewerCanRead_200` | `rubric_handlers_test.go` | REQ-RUBRIC-002-E2 + AC-RUBRIC-002-2 | handleGetRubric 미구현 |
| T-RED-025 | `TestPOST_RubricsArchive_AnalystForbidden_403` | `rubric_handlers_test.go` | UBI-004 | admin-only archive guard 미구현 |
| T-RED-026 | `TestPOST_RubricsApply_ViewerCanApply_200` | `rubric_handlers_test.go` | REQ-RUBRIC-004-E2 + AC-RUBRIC-004-2 | handleApplyRubric 미구현 / ABAC apply 분류 미구현 |
| T-RED-027 | `TestPOST_RubricsArchive_AdminSucceeds_200_TerminalImmutable` | `rubric_handlers_test.go` | REQ-RUBRIC-003-E2 + UBI-004 | handleArchiveRubric 미구현 |
| T-RED-028 | `TestPOST_RubricsAddCriterion_EvalItemNotExist_404` | `rubric_handlers_test.go` | REQ-RUBRIC-002-E4 + cross-store TX-1 | cross-store EvalItem 검증 미구현 |
| T-RED-029 | `TestPOST_RubricsApply_CrossStoreTwoTX` | `rubric_handlers_test.go` | REQ-RUBRIC-004-E2 + AC-RUBRIC-004-2 | handler-compose 2-TX 흐름 미구현 |
| T-RED-030 | `TestPOST_RubricsApply_ScoreOutOfAllBands_400` | `rubric_handlers_test.go` | REQ-RUBRIC-004-U1 + AC E10 | fail-closed 매핑 미구현 |
| T-RED-031 | `TestGET_Rubrics_AuthDisabled_PassthroughCliAnonymous` | `rubric_handlers_test.go` | UBI-003 + AC E8 | auth-disabled 투과 미구현 |
| T-RED-032 | `TestGET_Rubrics_MalformedUUID_400` | `rubric_handlers_test.go` | REQ-RUBRIC-002-U1 + AC E16 | UUID 검증 미구현 |
| T-RED-033 | `TestPOST_RubricsArchivedRubricMutation_409` | `rubric_handlers_test.go` | REQ-RUBRIC-003-S1 + AC E4 | archived terminal 핸들러 매핑 미구현 |

### 2.4 OPEN-derived 추가 RED Tests (strategy.md §1.8 표 참조)

| Task ID | 테스트 함수 | 파일 | OPEN | 책임 |
|---------|------------|------|------|------|
| T-RED-021b | `TestUpdateRubric_DraftToActive_DuplicateActiveBlocks_409` | `rubric_handlers_test.go` | OPEN #2 | handler pre-check 단계에서 중복 active 차단 |
| T-RED-022b | `TestUpdateRubric_RaceConcurrent_UniqueViolationMaps409` | `internal/store/rubric_integration_test.go` | OPEN #2 | testcontainers 동시성 — DB unique violation → 409 매핑 |
| T-RED-023b | `TestAddCriterion_WeightSumExceedsOne_400` | `rubric_handlers_test.go` | OPEN #3 | handler weight sum 검증 |
| T-RED-024b | `TestAddBand_Overlap_HandlerPreCheck_400` | `rubric_handlers_test.go` | OPEN #4 | handler pre-check 단계에서 band overlap 차단 |
| T-RED-025b | `TestAddBand_RaceConcurrent_ExclusionViolationMaps400` | `rubric_integration_test.go` | OPEN #4 | testcontainers 동시성 — DB EXCLUSION violation → 400 매핑 |
| T-RED-026b | `TestUpdateRubric_ActiveDirectEdit_AdminSucceeds_200` | `rubric_handlers_test.go` | OPEN #5 | active 상태 직접 편집 허용 검증 |
| T-RED-027b | `TestArchiveRubric_BlankReason_400` | `rubric_handlers_test.go` | OPEN #7 | archive_reason 필수 handler 검증 |
| T-RED-028b | `TestArchiveRubric_DBCheckViolation_500MapsCorrectly` | `rubric_integration_test.go` | OPEN #7 | DB CHECK violation 매핑 |

### 2.5 RED 완료 기준

- 모든 33 + 8 = **41 테스트가 RED(실패)로 확인됨**.
- `go vet ./...` PASS (컴파일 통과).
- `go test ./...` 신규 테스트만 실패, 기존 SCORE-001/REVIEW-001 테스트 100% PASS (consumer-only 검증).
- **orchestrator 직접 검증**: `git diff --quiet -- <frozen scope>` (strategy.md §6.2 명령) → exit 0 (drift 0).

---

## 3. GREEN Tasks — store + handler + migration + audit + errors 구현

### 3.1 GREEN — store + audit + errors + migration (T-GREEN-001 ~ T-GREEN-006)

| Task ID | 파일 | 책임 | 의존 |
|---------|------|------|------|
| T-GREEN-001 | `.moai/db/schema/migrations/0006_rubric_tables.sql` [NEW] | strategy.md §2.1 최종 DDL 구현 (멱등 패턴 + OPEN #2/#4/#7 인라인). 정방향 적용 + 재실행 멱등성 testcontainers 검증. | T-IFACE-001 |
| T-GREEN-002 | `internal/audit/audit.go` [MODIFY] | Action 상수 5개 추가: `ActionRubricCreated`/`ActionRubricUpdated`/`ActionRubricArchived`/`ActionRubricCriterionAdded`/`ActionRubricBandAdded`. **`ActionRubricApplied` 신설 0** (OPEN #6 read-only no-audit). | T-IFACE-001 |
| T-GREEN-003 | `internal/audit/recorder.go` [MODIFY] | 메서드 5개 추가: `RecordRubricCreated`/`RecordRubricUpdated`/`RecordRubricArchived(ctx, tx, rubricID, archiverID, archiveReason, userID)`/`RecordRubricCriterionAdded`/`RecordRubricBandAdded`. 모든 메서드는 `userID string` 파라미터 — local AuditTx, resource_id=UUID 직접(D2 미러). **`RecordRubricApplied` 신설 0**. | T-GREEN-002 |
| T-GREEN-004 | `internal/errors/errors.go` [MODIFY] | 센티넬 7개 추가: `ErrRubricNotFound`/`ErrRubricInvalidInput`/`ErrRubricInvalidStatus`/`ErrRubricArchived`/`ErrRubricWeightOutOfBounds`/`ErrRubricBandOverlap`/`ErrRubricAuditWriteFailed`. **SCORE-API-001 errors.go drift lesson — spec.md §2.1 + §2.3 양쪽 부착 EXPLICIT [HARD]**. | T-IFACE-001 |
| T-GREEN-005 | `internal/store/pg_store.go` [MODIFY] | `BeginRubricTx(ctx) (RubricTx, error)` 메서드 추가 — **`audit.NewRecorder(true)` HARD** (REVIEW-001 D1 iter2 lesson — `false` 절대 금지). 단일 pgxpool 재사용(`s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})`). | T-GREEN-003 |
| T-GREEN-006 | `internal/store/rubric.go` [완성] | `PgRubricTx` 11 메서드 + 4 validation 함수 + Commit/Rollback. 모든 mutation 메서드는 `userID string` 파라미터 받아 SQL `$N` placeholder에 `created_by`/`updated_by` 일관 전파. ApplyRubric은 read-only no-audit (recorder 호출 0 — OPEN #6). state-machine guard: `draft→active`/`active→archived`만 허용. | T-GREEN-001, T-GREEN-005, T-GREEN-004 |

### 3.2 GREEN — HTTP API handler (T-GREEN-007 ~ T-GREEN-011)

| Task ID | 파일 | 책임 | 의존 |
|---------|------|------|------|
| T-GREEN-007 | `cmd/server/rubric_handlers.go` [완성 핸들러 1/3] | `RubricHandler` struct + `NewRubricHandler(rubricStore, evalItemStore, scoreStore, logger)` (cross-store 3-store 주입) + `Routes()` (strategy.md §3.3 ServeMux 9 엔드포인트 + 최장일치 정렬) + `writeRubricJSON`/`writeRubricErr`/`mapRubricStoreErr` 헬퍼 + `resolveCreatedBy` 헬퍼 (REVIEW-001 D1 iter2 동형) + `requireRubricAdminRole`/`guardRubricAdmin` 게이트. | T-GREEN-006 |
| T-GREEN-008 | `cmd/server/rubric_handlers.go` [완성 핸들러 2/3] | CRUD 핸들러 5개: `handleCreateRubric`/`handleGetRubric`/`handleListRubrics`/`handleUpdateRubric`/`handleArchiveRubric`. OPEN #2 dual defense (handleUpdateRubric pre-check + DB unique violation → 409 매핑). OPEN #5 active 직접 편집 허용. OPEN #7 archive_reason 필수 검증. | T-GREEN-007 |
| T-GREEN-009 | `cmd/server/rubric_handlers.go` [완성 핸들러 3/3] | sub-resource 핸들러 3개: `handleAddCriterion` (OPEN #3 weight sum 검증 + cross-store EvalItem TX-1)/`handleAddBand` (OPEN #4 dual defense — handler pre-check + DB EXCLUSION violation → 400 매핑)/`handleCloneNewVersion` (OPEN #1 sub-resource). | T-GREEN-008 |
| T-GREEN-010 | `cmd/server/rubric_handlers.go` [apply 엔진] | `handleApplyRubric` cross-store handler-compose 2-TX 정확 구현 (strategy.md §4.1): TX-1 score sum read-only Rollback + SEC-03 float64 1회 변환 + TX-2 rubric apply read-only Rollback. audit row 0건. | T-GREEN-009 |
| T-GREEN-011 | `internal/store/rubric_integration_test.go` [NEW] | testcontainers 통합 테스트 14+ 케이스 — 0006 마이그레이션 정방향 + 재실행 멱등 / `BeginRubricTx` end-to-end / 동일-TX audit 원자성 / state transition / cross-store EvalItem 존재 검증 race / OPEN #2 unique violation race / OPEN #4 EXCLUSION violation race / OPEN #7 DB CHECK violation. | T-GREEN-010 |

### 3.3 GREEN — server.go 마운트 (T-GREEN-012)

| Task ID | 파일 | 책임 | 의존 |
|---------|------|------|------|
| T-GREEN-012 | `cmd/server/server.go` [MODIFY ≈7줄] | 정확히 ≈7줄 — REPORT-001 lesson pre-applied [HARD]. (1) `rubricH *RubricHandler` 필드 1줄 + 상단 ko 주석 1줄, (2) `s.rubricH = NewRubricHandler(pgStore, pgStore, pgStore, logger)` 생성자 1줄 + ko 주석 1줄, (3) `innerMux.Handle("/api/v1/rubrics", s.rubricH.Routes())` + `innerMux.Handle("/api/v1/rubrics/", s.rubricH.Routes())` 마운트 2줄. **`1줄` 잘못 기술 절대 금지** — drift 검증 시 라인 정확성 확인. | T-GREEN-011 |

### 3.4 GREEN 완료 기준

- 모든 41 RED 테스트 GREEN으로 전환 (post-hoc 수정 0 — 처음부터 정확).
- `go build ./...` PASS.
- 헬스체크 + `/api/v1/rubrics` 라우트 reachable.
- 0006 마이그레이션 testcontainers 자동 적용 verify.
- **orchestrator 직접 검증**: frozen scope 0-diff (strategy.md §6.2 명령).

---

## 4. REFACTOR Tasks — TRUST 5 + MX 태그 + godoc

| Task ID | 파일 | 책임 | 의존 |
|---------|------|------|------|
| T-REFACTOR-001 | `internal/store/rubric.go` + `cmd/server/rubric_handlers.go` | @MX:ANCHOR 태그 — fan_in≥3 함수에 부착 (예: `BeginRubricTx`, `mapRubricStoreErr`, `resolveCreatedBy`, `ApplyRubric`). 한국어 코멘트 명시. | T-GREEN-012 |
| T-REFACTOR-002 | 동일 파일 | @MX:WARN 태그 — 복잡도≥15 함수 또는 cross-store race window 함수에 부착 (예: `handleApplyRubric`, `handleAddBand`). `@MX:REASON` 의무. | T-REFACTOR-001 |
| T-REFACTOR-003 | 동일 파일 | @MX:NOTE 태그 — 신규 mutation 메서드 + 신규 핸들러 메서드. 한국어 의도 명시(예: "REVIEW-001 D1 iter2 lesson — userID string 파라미터 일관 전파"). @MX:TODO는 GREEN 단계에서 모두 제거 — 잔존 0 검증. | T-REFACTOR-002 |
| T-REFACTOR-004 | 모든 exported 함수 + struct | godoc 추가 — 한국어 일관(SCORE-001/REVIEW-001 동형). 시그니처 + 의도 + 호출 시점 + side-effects 명시. | T-REFACTOR-003 |
| T-REFACTOR-005 | 전체 changeset | 공통 헬퍼 추출 + 중복 제거 + `gofmt`/`goimports`/`golangci-lint` zero issues. 테스트 커버리지 ≥85% 검증 (store-layer + handler 합산). 한국어 에러 메시지 정합 (REVIEW-001 `score_handlers.go:114-127` 동형). | T-REFACTOR-004 |

### 4.1 REFACTOR 완료 기준

- TRUST 5 PASS (Tested + Readable + Unified + Secured + Trackable).
- 커버리지 ≥85%.
- evaluator-active 4-차원 ≥0.85 (Functionality + Security + Craft + Consistency).
- golangci-lint zero issues.
- @MX 잔존 TODO 0건.

---

## 5. 검증 Tasks — Drift-Guard + TRUST 5 + evaluator-active + manager-quality

### 5.1 T-VERIFY-001 — Drift-Guard 0% 검증

| 단계 | 명령 | 통과 기준 |
|------|------|----------|
| 1 | `git diff --quiet -- <frozen scope>` (strategy.md §6.2 명령) | exit 0 |
| 2 | 각 [MODIFY] 파일 추가 범위만 검증 (strategy.md §6.3) | manual review PASS |
| 3 | `server.go` ≈7줄 정확성 | `git diff cmd/server/server.go` 가 정확히 ≈7줄 (필드 1 + 생성 1 + 마운트 2 + ko 주석 1-3) |
| 4 | `errors.go` 신규 센티넬 정확 7개 | manifest 부착 정확성 |
| 5 | `go.mod`/`go.sum` 0-diff | exit 0 (외부 의존 추가 0) |

위반 발생 시 즉시 중단·재계획 (spec.md §2.3 R-CONSUMER-001).

### 5.2 T-VERIFY-002 — TRUST 5 + harness=thorough

| 차원 | 통과 기준 |
|------|----------|
| Tested | 커버리지 ≥85% + 25 AC + 16 edge + OPEN 8 추가 시나리오 모두 자동화. T-RED-005/T-RED-019 (D1 iter2 lesson 검증) 필수 PASS. |
| Readable | 한국어 godoc + 한국어 에러 메시지 + @MX 한국어 주석. exported 모두 godoc. |
| Unified | gofmt/goimports/golangci-lint zero issues. |
| Secured | OWASP 준수. raw pgx 에러 누출 0. ABAC narrowing fail-closed. archived terminal 불변. SQL injection 0(`$N` placeholder만). |
| Trackable | conventional commit + SPEC-AX-RUBRIC-001 reference. lesson pre-applied 명시 commit. |

### 5.3 T-VERIFY-003 — evaluator-active 4-차원 + manager-quality 게이트

| 단계 | 책임 | 통과 기준 |
|------|------|----------|
| 1 | evaluator-active 4-차원 평가 | Functionality ≥0.85 / Security ≥0.85 / Craft ≥0.85 / Consistency ≥0.85. 종합 ≥0.85 (thorough). |
| 2 | manager-quality TRUST 5 PASS | 5 차원 모두 PASS. 위반 0. |
| 3 | dark-flow iter2 패턴 회피 검증 | evaluator-active가 self-report fake GREEN 미적발(genuine RED-first cycle 확인). |
| 4 | D1 iter2 lesson pre-applied 검증 | `BeginRubricTx` `NewRecorder(true)` 직접 grep 확인 + mutation 시그니처 `userID string` 검증 + handler `resolveCreatedBy` 호출 검증. |

---

## 6. 의존성 그래프 (병렬 가능 set 명시)

### 6.1 Sequential 의존 chain

```
T-IFACE-001 (store.go interface)
    ├─→ T-IFACE-002 (rubric.go skeleton)
    │      └─→ T-RED-001..019 (store-layer RED)
    │             └─→ T-GREEN-001..006 (store GREEN)
    ├─→ T-IFACE-003 (rubric_handlers.go skeleton)
    │      └─→ T-RED-020..033 + 021b/022b/023b/024b/025b/026b/027b/028b (handler RED)
    │             └─→ T-GREEN-007..010 (handler GREEN)
    │                    └─→ T-GREEN-011 (integration test)
    │                           └─→ T-GREEN-012 (server.go mount)
    │                                  └─→ T-REFACTOR-001..005
    │                                         └─→ T-VERIFY-001..003
    └─→ T-GREEN-001 (0006 migration, 병행 가능)
```

### 6.2 병렬 가능 set

| Set | Tasks | 조건 |
|-----|-------|------|
| RED store-layer | T-RED-001..019 (19개) | T-IFACE-002 완료 후, 19개 동시 작성 가능(독립 테스트) |
| RED handler | T-RED-020..033 + 021b/022b/023b/024b/025b/026b/027b/028b (22개) | T-IFACE-003 완료 후, 22개 동시 작성 가능 |
| GREEN audit + errors | T-GREEN-002 + T-GREEN-004 | 독립 파일 |
| REFACTOR 단계 | T-REFACTOR-001..005 | sequential (앞 단계 의존) |

### 6.3 Critical Path

```
T-IFACE-001 → T-IFACE-002 → T-RED-001..019 → T-GREEN-001 → T-GREEN-005 → T-GREEN-006
            → T-IFACE-003 → T-RED-020..033 → T-GREEN-007..010 → T-GREEN-011 → T-GREEN-012
                                                                            → T-REFACTOR-001..005
                                                                            → T-VERIFY-001..003
```

Critical Path 우선순위 High → Medium → Low 순으로 진행. Sub-agent mode 단일 세션 권장 (병렬 set은 한 메시지 내 다중 Edit 호출).

---

## 7. 참조

- strategy.md §1 OPEN 결정 매트릭스 / §2 0006 마이그레이션 / §3 9 엔드포인트 / §4 cross-store 2-TX / §5 D1 iter2 lesson / §6 Drift-Guard / §7 Risk Register
- spec.md §2.1 [NEW]/[MODIFY]/[EXISTING] / §2.3 Drift-Guard manifest / §3 EARS / §6 OPEN
- plan.md §2 마일스톤 M0-M5 / §4 TDD RED-GREEN-REFACTOR / §5 R-RUBRIC-001-005
- acceptance.md AC 25 + edge 16
- research.md §1 REVIEW-001 수직 슬라이스 선례 / §2 SCORE-001 grade_thresholds 병행
- SPEC-AX-REVIEW-001 (gold standard mirror target, commit 89a8828)
- SPEC-AX-REPORT-001 (cross-store 2-TX + ≈7줄 server.go 선례)
- SPEC-AX-SCORE-API-001 (errors.go drift lesson + OPEN #4 evaluator-INFEASIBLE lesson)
- `.claude/skills/moai/workflows/plan.md` L378 (canonical 8-field frontmatter schema)
- Memory: `feedback_consumer_only_orchestrator_grep` / `feedback_dark_flow_iter2_pattern` / `project_pg_store_recorder_bool` / `lessons_session_2026_05_14`
