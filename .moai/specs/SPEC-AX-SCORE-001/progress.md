# SPEC-AX-SCORE-001 진행 로그

## GAN-loop iteration 2 — CRITICAL 결함 수정 (F1-F6, TDD RED→GREEN→REFACTOR)

Phase 3 evaluator-active FAIL (46.25/100, Must-Pass Firewall ×2) 후 구조적 결함을
RED-first로 수정. 이전 구현은 post-hoc 테스트가 가장 어려운 binding 기준을 단언하지
않아 빌드/통합이 통과했었음 (TDD 순서 위반). 본 iteration에서 TDD 순서 정정.

### 결함 요약
- F1 [CRITICAL] DC-UBI-002/REQ-SCORE-001-E1: InsertScore/UpdateScore audit 원자성 부재
  (RecordScoreCreated/RecordScoreUpdated production 호출자 0건 — phantom @MX:REASON)
- F2 [CRITICAL] DC-UBI-004/REQ-SCORE-001-S2: CONFIRMED 정정 경로(신규행+SUPERSEDED) 부재
- F3 [HIGH] SEC-03/DC-002-E1: SumWeightedByEvaluationItem float64 사용 (DECIMAL 부정확)
- F4 [MEDIUM] DC-001-U1 Case C/D + 미단언 조건 (score_value 검증, metadata 빈/중첩)
- F5 [HARD TH-02] gofmt 4개 파일
- F6 [Advisory] ResolveGrade→DetermineGrade (binding contract 어휘 정합)

---

## RED 증거 (수정 전 — 현재 코드 대비 실패 확인)

명령: `go vet -tags=integration ./internal/store/ ./internal/audit/`

컴파일 실패가 각 결함을 현재 코드 대비 명확히 입증 (signature 변경형 결함의
가장 강력한 RED — 심볼 부재):

```
internal/store/score_grade_test.go:113:21: tx.DetermineGrade undefined
  (type ScoreTx has no field or method DetermineGrade)   [F6 — 코드는 여전히 ResolveGrade]
  ... (×6 동일)
internal/store/score_insert_test.go:436:45: cannot use sum (variable of type
  float64) as pgtype.Numeric value in argument to numericExact
  [F3 — SumWeightedByEvaluationItem 여전히 float64 반환, SEC-03 위반]
  ... (×3 동일: 436/470/488)
internal/store/score_supersede_test.go:51:19: tx.SupersedeAndReplaceScore undefined
  (type ScoreTx has no field or method SupersedeAndReplaceScore)
  [F2 — CONFIRMED 정정 경로 부재]
```

F1 RED: 컴파일 수정 후 `TestScoreAtomicity_InsertCreatesAuditRow`의
`auditScoreCount(SCORE_CREATED)==1` 단언은 현재 InsertScore가 recorder를 호출하지
않으므로 0을 반환하여 실패한다 (이전 contrived captureFaultTx는 이 경로를 우회했음).
`TestScoreAtomicity_AuditFaultBidirectionalRollback`의
`assert.ErrorIs(insErr, ErrScoreAuditWriteFailed)`는 현재 InsertScore가 audit를
호출하지 않아 에러 자체가 없으므로 실패.

→ RED 상태 확정. TDD 순서 정정: 테스트가 binding 기준을 먼저 단언하고
  현재 코드 대비 실패함을 확인한 후 GREEN 구현 진행.

---

## GREEN 증거 (수정 후 — binding 기준 실 단언 통과)

### 구현 (production)
- `score.go`:
  - F1: `PgScoreTx.recorder *audit.Recorder` 필드 + InsertScore→RecordScoreCreated /
    UpdateScore→RecordScoreUpdated 동일 t.tx 호출. audit 실패 시
    `ErrScoreAuditWriteFailed` 래핑. @MX:REASON을 실 동작과 일치하도록 정정
    (phantom annotation 제거). no-op UpdateScore는 audit 미기록.
  - F2: `SupersedeAndReplaceScore` 신설 — 신규행 INSERT(CONFIRMED) + 구행
    CONFIRMED→SUPERSEDED, 동일 t.tx, 각 1 audit row, 물리 DELETE 0.
    비-CONFIRMED → `ErrScoreNotConfirmed`. `insertScoreRow` 헬퍼로
    InsertScore/Supersede INSERT 중복 제거 (REFACTOR).
  - F3: `SumWeightedByEvaluationItem` 반환 `float64`→`pgtype.Numeric`,
    `::numeric(12,4)` 캐스트. float64 집계 경로 완전 제거 (SEC-03).
  - F4: `validateScoreInput`에 NaN/±Inf score_value 거부 추가.
    evidence_id Case D는 *uuid.UUID 타입 경계 차단을 godoc로 명시 (phantom 검증 미추가).
  - F6: `ResolveGrade`→`DetermineGrade` 전면 리네임 (binding contract DC-003-*
    + errors.go:68 godoc 어휘 정합).
- `pg_store.go`: F1 — `BeginScoreTx`가 `audit.NewRecorder(false)` 주입.
- `store.go`: ScoreTx 인터페이스 — F3 시그니처/F6 리네임/F2 메서드 추가.
- `errors.go`: 추가 센티널 `ErrScoreAuditWriteFailed`/`ErrScoreNotConfirmed` (additive).

### 검증 명령 출력
```
go build ./internal/... ./cmd/...        → RC=0 (전 패키지 컴파일)
go vet  ./internal/store/ ./internal/audit/ ./internal/errors/ → RC=0
gofmt -l <9 changed files>               → (빈 출력 = 전부 포맷됨, F5 PASS)
golangci-lint (score.go/store.go/pg_store.go/errors.go) → 0 issues (TH-02 production PASS)
go test -tags=integration -count=1 -timeout=600s -run 'TestScore'
  ./internal/store/ ./internal/audit/:
    ok  github.com/ircp/iroum-ax/apps/control-plane/internal/store   262.661s
    ok  github.com/ircp/iroum-ax/apps/control-plane/internal/audit   0.142s
```

### binding 기준 — 실 단언으로 성립 (개별 PASS)
- **DC-UBI-002 (audit COUNT=1)**:
  - `TestScoreAtomicity_InsertCreatesAuditRow` PASS — InsertScore+Commit 후
    `audit_logs SCORE_CREATED COUNT=1` (실 recorder 경로, contrived mock 제거)
  - `TestScoreAtomicity_UpdateCreatesAuditRow` PASS — `SCORE_UPDATED COUNT=1`
  - `TestScoreAtomicity_NoOpUpdateNoAuditRow` PASS — no-op은 audit 0건
- **DC-004-U1 (양방향 rollback)**:
  - `TestScoreAtomicity_AuditFaultBidirectionalRollback` PASS — audit CHECK(false)
    주입 시 InsertScore가 `errors.Is(ErrScoreAuditWriteFailed)` 래핑 + scores 0 +
    audit_logs 0 + goleak 0
- **DC-UBI-004 (2 행 + 2 audit, 0 delete)**:
  - `TestScoreSupersede_ConfirmedCorrectionCreatesNewRowAndSupersedesOld` PASS —
    scores 2행(구=SUPERSEDED, 신=CONFIRMED) + 정정 TX delta audit 2건
    (구 SCORE_UPDATED + 신 SCORE_CREATED)
  - `TestScoreSupersede_NoPhysicalDelete` PASS — 구 행 물리 보존(EC-14)
  - `TestScoreSupersede_RejectsNonConfirmed` PASS — DRAFT 정정 거부, DB 무변경
  - `TestScoreSupersede_AtomicityFailureRollsBackEntireTX` PASS — EC-ADD-1
- **SEC-03 (정확 십진, epsilon=0)**:
  - `TestScore_SumWeightedByEvaluationItem_Exactness` PASS — 표준 데이터셋
    (90,0.5)(80,0.3)(70,0.2) → big.Rat 정확 비교 = 정확히 83 (float64 미경유)
  - `_NullWeightExcluded` PASS = 정확히 50 / `_EmptyIsZero` PASS = 정확히 0
- **F4**: `TestScore_Validation_ScoreValueNaN`/`_ScoreValueInf`/
  `_EvidenceIDBoundaryIsUUIDType`/`TestScore_EmptyMetadataAccepted`/
  `TestScore_DeeplyNestedMetadataRoundTrip` 전부 PASS
- **F6**: `TestScoreGrade_DetermineGrade_*` 4건 전부 PASS
- **D2 불변 (TH-11/12)**: score.go `uuid.NewSHA1(`=0, Namespace 상수=0;
  recorder.go RecordScore* 함수 내 NewSHA1=0; audit.go score namespace 상수=0
- **경계 (TH-06~10)**: initial.sql/0002/0003/0004/postgres.go/go.mod/go.sum
  전부 0-byte diff (recorder.go/audit.go diff는 본 세션 미변경 — branch 선행 작업)

### TDD 순서 정정 기록
본 fix 이전 구현은 post-hoc 테스트(contrived captureFaultTx, InDelta 0.001,
in-place 단언)가 가장 어려운 binding 기준을 우회하여 RED 없이 통과했었음.
본 iteration에서 (1) binding 기준을 먼저 실패하는 테스트로 강화/작성,
(2) 현재 코드 대비 RED(컴파일 실패 + 단언 실패) 확인,
(3) GREEN 최소 구현, (4) REFACTOR(insertScoreRow 중복 제거, big.Rat 정확비교,
@MX phantom 제거) 순으로 TDD 순서를 정정하여 수행함.

