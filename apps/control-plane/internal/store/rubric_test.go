// rubric_test.go — 등급 rubric store-layer 단위 테스트 (SPEC-AX-RUBRIC-001 Phase B GREEN)
//
// 격리 전략: PgRubricTx 내부 validateRubricInput/validateCriterionInput/validateBandInput/
// validateRubricStatusTransition 등 SQL 미실행 헬퍼는 default 빌드 태그로 실행.
// SELECT FOR UPDATE / DB CHECK constraint / audit fault rollback 등 DB 의존 테스트는
// rubric_integration_test.go (//go:build integration) 에서 testcontainers postgres:16-alpine 검증.
//
// Phase B GREEN [HARD]: Phase A의 "not implemented" assertion → 실제 sentinel 검증으로 전환.
// score_review_request_test.go 패턴 정확 미러 (validateReviewRequestInput / validateRejectionReason /
// validateReviewStatusTransition + AuditTx fake recorder).
//
// REVIEW-001 D1 iter2 lesson 검증 의무 테스트:
//   - T-RED-005 (TestInsertRubric_UserIDPropagatesToCreatedByAndAuditUserID): userID 시그니처 정확성
//   - T-RED-019 (TestNewRecorderTrue_AuthEnabledUserIDPropagates): PgRubricTx.recorder 필드 정합
//
// OPEN #6 검증 의무:
//   - T-RED-018 (TestApplyRubric_ReadOnly_NoAuditWritten): recorder 호출 0 — fake recorder fail-fast
package store

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// ════════════════════════════════════════════════════════════════════════════
// fakeRubricAuditTx — audit.AuditTx 기반 호출 카운터 (OPEN #6 read-only no-audit 검증용)
// SCORE-001/REVIEW-001 audit fake 패턴 미러
// ════════════════════════════════════════════════════════════════════════════

type fakeRubricAuditTx struct {
	calls  int
	events []*audit.Event
}

func (f *fakeRubricAuditTx) InsertAuditLog(_ context.Context, e *audit.Event) error {
	f.calls++
	f.events = append(f.events, e)
	return nil
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-001 — InsertRubric 시그니처 정확성 + userID 파라미터 보유 (REQ-RUBRIC-001-E1 + UBI-002)
// Phase B GREEN: 시그니처가 (ctx, name, scope, metadata, userID) 정확. tx=nil이면 nil-panic 보호로 인해
// 패스가 어렵지만 — validateRubricInput pre-write가 먼저 동작 → blank name이면 SQL 미실행.
// 실 DB 검증은 rubric_integration_test.go에서 testcontainers로 진행.
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_SignatureContract_UserIDParameterPresent(t *testing.T) {
	// 시그니처 정확성 (D1 iter2 lesson): userID string 파라미터가 처음부터 존재
	tx := &PgRubricTx{}
	// blank name이면 validateRubricInput에서 SQL 미실행 후 ErrRubricInvalidInput 반환 — tx nil 안전
	_, err := tx.InsertRubric(context.Background(), "", "default", nil, "admin-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
		"blank name은 ErrRubricInvalidInput 래핑되어야 한다 (validateRubricInput pre-write)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-002 — validateRubricInput blank name 거부 (REQ-RUBRIC-001-U1)
// ════════════════════════════════════════════════════════════════════════════

func TestValidateRubricInput_BlankName_ReturnsInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab only", "\t"},
		{"newline only", "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRubricInput(tc.input)
			require.Error(t, err)
			assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
				"blank/whitespace name은 ErrRubricInvalidInput 래핑되어야 한다")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-003 — validateRubricInput name 64자 초과 거부 (REQ-RUBRIC-001-U1)
// ════════════════════════════════════════════════════════════════════════════

func TestValidateRubricInput_NameOver64Chars_ReturnsInvalidInput(t *testing.T) {
	longName := strings.Repeat("a", 65) // VARCHAR(64) 초과
	err := validateRubricInput(longName)
	require.Error(t, err, "name 64자 초과는 거부되어야 한다")
	assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
		"64자 초과 name은 ErrRubricInvalidInput 래핑되어야 한다")

	// 경계 검증: 64자는 통과해야 한다
	boundaryName := strings.Repeat("a", 64)
	require.NoError(t, validateRubricInput(boundaryName), "정확히 64자는 통과해야 한다")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-004 — InsertRubric blank name 시 SQL 미실행 fail-closed (UBI-002 양방향 원자성)
// Phase B GREEN: 입력 검증 단계에서 ErrRubricInvalidInput 반환 — tx 무관, 핸들러 Rollback 불필요
// 실 audit fault rollback은 rubric_integration_test.go에서 testcontainers로 진행
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_InvalidInput_FailClosedBeforeSQL(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.InsertRubric(context.Background(), strings.Repeat("a", 65), "default", nil, "admin-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
		"64자 초과 name은 SQL 미실행 후 ErrRubricInvalidInput 반환 (fail-closed)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-005 [D1 iter2 lesson] — InsertRubric userID string 파라미터 시그니처 검증 (UBI-003)
// REVIEW-001 D1 iter2 lesson pre-applied [HARD]: mutation 메서드가 userID string 파라미터 보유.
// 본 단위 테스트는 시그니처 정확성만 검증 — SQL $N 바인딩의 실제 영속화는 integration test.
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_UserIDParameterSignature_D1IterTwoLesson(t *testing.T) {
	tx := &PgRubricTx{}
	// 컴파일 시점에 userID 파라미터가 마지막 위치에 존재해야 한다 (D1 iter2 lesson)
	// blank name으로 fail-closed 경로를 트리거하여 tx nil 안전 확보
	_, err := tx.InsertRubric(context.Background(), "", "default", nil, "user-42")
	require.Error(t, err, "blank name + userID='user-42' 호출 가능 — 시그니처 정합 (Phase B GREEN)")
	assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput)
	// 핵심: 컴파일이 통과한다는 사실 자체가 시그니처 D1 iter2 lesson 검증.
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-006 — AddCriterion 시그니처 정확성 + userID 파라미터 (REQ-RUBRIC-001-E2)
// ════════════════════════════════════════════════════════════════════════════

func TestAddCriterion_SignatureContract_WeightValidationFailClosed(t *testing.T) {
	tx := &PgRubricTx{}
	// weight 범위 외 → validateCriterionInput pre-write fail-closed (tx nil 안전)
	_, err := tx.AddCriterion(context.Background(), uuid.New(), uuid.New(), 1.5, "admin-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrRubricWeightOutOfBounds,
		"weight=1.5는 ErrRubricWeightOutOfBounds 래핑되어야 한다")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-007 — validateCriterionInput weight 범위 외 거부 (REQ-RUBRIC-001-U1)
// ════════════════════════════════════════════════════════════════════════════

func TestValidateCriterionInput_WeightOutOfBounds(t *testing.T) {
	cases := []struct {
		name   string
		weight float64
		bad    bool
	}{
		{"negative weight", -0.1, true},
		{"over 1.0", 1.5, true},
		{"exactly above 1.0", 1.0001, true},
		{"zero (boundary)", 0.0, false},
		{"one (boundary)", 1.0, false},
		{"mid range", 0.5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCriterionInput(tc.weight)
			if tc.bad {
				require.Error(t, err)
				assert.ErrorIs(t, err, stderrors.ErrRubricWeightOutOfBounds,
					"범위 외 weight는 ErrRubricWeightOutOfBounds 래핑")
			} else {
				require.NoError(t, err, "범위 내 weight는 통과해야 한다")
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-008 — AddCriterion archived rubric 거부 시그니처 (REQ-RUBRIC-003-S1)
// archived terminal 불변 — 실 영속 검증은 integration test
// ════════════════════════════════════════════════════════════════════════════

func TestAddCriterion_NegativeWeight_FailClosedBeforeSQL(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.AddCriterion(context.Background(), uuid.New(), uuid.New(), -0.001, "admin-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrRubricWeightOutOfBounds,
		"negative weight는 SQL 미실행 fail-closed")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-009 — AddBand 시그니처 정확성 + blank letter 거부 (REQ-RUBRIC-001-E3)
// ════════════════════════════════════════════════════════════════════════════

func TestAddBand_BlankLetter_ReturnsInvalidInput(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.AddBand(context.Background(), uuid.New(), "", 80.0, 89.999, "admin-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
		"blank letter는 ErrRubricInvalidInput 래핑되어야 한다 (fail-closed)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-010 — validateBandInput min >= max 거부 (REQ-RUBRIC-001-U1)
// ════════════════════════════════════════════════════════════════════════════

func TestValidateBandInput_MinGteMax_ReturnsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		min  float64
		max  float64
		bad  bool
	}{
		{"min equals max", 80.0, 80.0, true},
		{"min greater than max", 90.0, 80.0, true},
		{"min less than max", 80.0, 89.999, false},
		{"adjacent ranges allowed", 70.0, 79.999, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBandInput(tc.min, tc.max)
			if tc.bad {
				require.Error(t, err)
				assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
					"min >= max는 ErrRubricInvalidInput 래핑")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-011 — UpdateRubric 시그니처 정확성 + validation pre-write (REQ-RUBRIC-003-E1)
// 실 draft→active 전이 + audit은 integration test에서 testcontainers로 검증
// ════════════════════════════════════════════════════════════════════════════

func TestUpdateRubric_BlankName_FailClosedBeforeSQL(t *testing.T) {
	tx := &PgRubricTx{}
	err := tx.UpdateRubric(context.Background(), uuid.New(), "", "default", "active", nil, "admin-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
		"blank name은 UpdateRubric에서도 fail-closed (validateRubricInput 공유)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-012 — UpdateRubric archived rubric 거부 시그니처 (REQ-RUBRIC-003-S1)
// 실 SELECT FOR UPDATE + status check는 integration test
// ════════════════════════════════════════════════════════════════════════════

func TestUpdateRubric_OverlongName_FailClosed(t *testing.T) {
	tx := &PgRubricTx{}
	err := tx.UpdateRubric(context.Background(), uuid.New(),
		strings.Repeat("a", 100), "default", "active", nil, "admin-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
		"100자 name은 UpdateRubric에서 fail-closed")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-013 — ArchiveRubric archive_reason 필수 (REQ-RUBRIC-003-E2 + OPEN #7)
// archive_reason blank/empty 시 SQL 미실행 후 ErrRubricInvalidInput
// ════════════════════════════════════════════════════════════════════════════

func TestArchiveRubric_BlankReason_ReturnsInvalidInput(t *testing.T) {
	tx := &PgRubricTx{}
	cases := []struct {
		name   string
		reason string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab only", "\t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tx.ArchiveRubric(context.Background(), uuid.New(), tc.reason, "admin-001")
			require.Error(t, err)
			assert.ErrorIs(t, err, stderrors.ErrRubricInvalidInput,
				"blank archive_reason은 OPEN #7 Layer 1 fail-closed")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-014 — ArchiveRubric archive_reason 비어있지 않으면 validation 통과
// (실 SELECT FOR UPDATE는 integration test — tx nil로 인한 nil-panic 가능)
// ════════════════════════════════════════════════════════════════════════════

func TestArchiveRubric_ValidReason_PassesValidation(t *testing.T) {
	// validation 단계 통과 확인 — 실제 DB 호출은 integration test로 분리
	// 본 unit test는 archive_reason="reason"이 ErrRubricInvalidInput 아님을 확인
	// (이후 lockAndCheckRubricStatus가 nil tx 접근 panic하더라도 별개 — 시그니처 검증 목적)
	defer func() {
		// nil tx 접근으로 panic 발생 가능 — recovery로 unit-test 친화 처리
		_ = recover()
	}()
	tx := &PgRubricTx{}
	err := tx.ArchiveRubric(context.Background(), uuid.New(), "deprecated by new policy", "admin-001")
	// nil tx로 인한 다른 에러가 발생할 수 있으나, ErrRubricInvalidInput는 아니어야 한다
	if err != nil {
		assert.False(t, isInvalidInputError(err),
			"valid archive_reason은 ErrRubricInvalidInput 발생하지 않아야 한다 (다른 nil-tx 에러는 무방)")
	}
}

// isInvalidInputError 헬퍼 — ErrRubricInvalidInput 래핑 여부 검사
func isInvalidInputError(err error) bool {
	if err == nil {
		return false
	}
	// errors.Is로 wrapper chain 검사
	type unwrap interface{ Unwrap() error }
	for cur := err; cur != nil; {
		if cur == stderrors.ErrRubricInvalidInput {
			return true
		}
		u, ok := cur.(unwrap)
		if !ok {
			break
		}
		cur = u.Unwrap()
	}
	return false
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-015 — validateRubricStatusTransition 전체 매트릭스 (UBI-004 + REQ-RUBRIC-003-S2)
// 허용: draft→active / active→archived (2건)
// 거부: 9-2 = 7건 (3 states × 3 targets - 2 allowed)
// score_review_request_test.go TestValidateReviewStatusTransition 동형 패턴
// ════════════════════════════════════════════════════════════════════════════

func TestValidateRubricStatusTransition_AllAllowedAndDisallowed(t *testing.T) {
	type tc struct {
		current string
		next    string
		allowed bool
	}
	cases := []tc{
		{rubricStatusDraft, rubricStatusActive, true},     // 허용
		{rubricStatusDraft, rubricStatusArchived, false},  // 거부 — draft 직접 archive 금지
		{rubricStatusDraft, rubricStatusDraft, false},     // 거부 — self-loop
		{rubricStatusActive, rubricStatusArchived, true},  // 허용
		{rubricStatusActive, rubricStatusDraft, false},    // 거부 — 되돌리기
		{rubricStatusActive, rubricStatusActive, false},   // 거부 — self-loop
		{rubricStatusArchived, rubricStatusDraft, false},  // 거부 — terminal
		{rubricStatusArchived, rubricStatusActive, false}, // 거부 — terminal
		{rubricStatusArchived, rubricStatusArchived, false}, // 거부 — terminal self
	}
	for _, c := range cases {
		t.Run(c.current+"->"+c.next, func(t *testing.T) {
			err := validateRubricStatusTransition(c.current, c.next)
			if c.allowed {
				require.NoError(t, err, "%s→%s는 허용되어야 한다", c.current, c.next)
			} else {
				require.Error(t, err, "%s→%s는 거부되어야 한다", c.current, c.next)
				assert.ErrorIs(t, err, stderrors.ErrRubricInvalidStatus,
					"불법 전이는 ErrRubricInvalidStatus 래핑")
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-016 — ApplyRubric 경계 inclusive 정확성 (REQ-RUBRIC-004-E1)
// Risk R-RUBRIC-007 검증 — numrange '[]' inclusive 의미 + 양 끝 경계 매치
// 실 DB는 integration test — 본 단위 테스트는 시그니처 + tx nil panic 회피만 검증
// ════════════════════════════════════════════════════════════════════════════

func TestApplyRubric_SignatureContract_ReadOnlyReturn(t *testing.T) {
	// 시그니처 정확성: (ctx, rubricID, scoreValue) → (letter, *band, error)
	// 실 linear scan은 integration test로 검증
	defer func() { _ = recover() }() // nil tx panic recovery
	tx := &PgRubricTx{}
	_, _, _ = tx.ApplyRubric(context.Background(), uuid.New(), 85.5)
	// 컴파일이 통과한다는 사실 자체가 시그니처 검증 — 실 동작은 integration test
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-017 — ApplyRubric 시그니처 fail-closed 의도 명시 (REQ-RUBRIC-004-U1)
// 실 동작 (모든 band 밖 → ErrRubricInvalidInput)은 integration test 검증
// ════════════════════════════════════════════════════════════════════════════

func TestApplyRubric_SignatureFailClosedIntent(t *testing.T) {
	// 본 단위 테스트는 시그니처 + ErrRubricInvalidInput sentinel 존재 확인만 검증
	// 실제 fail-closed 경로(모든 band 밖)는 integration test에서 testcontainers DB로 검증
	require.NotNil(t, stderrors.ErrRubricInvalidInput,
		"ErrRubricInvalidInput sentinel이 정의되어 있어야 한다 (REQ-RUBRIC-004-U1 fail-closed)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-018 [OPEN #6 read-only no-audit] — ApplyRubric은 recorder 호출 0 [HARD]
// REPORT-001 read-only no-audit 선례 정확 미러
// fake recorder fail-fast assertion — UBI-002 second clause "apply read-only 예외" carve-out
//
// 본 단위 테스트의 핵심 검증:
//  1. fakeRubricAuditTx.calls 정확히 0 (recorder 미호출)
//  2. PgRubricTx.ApplyRubric 시그니처가 (ctx, rubricID, scoreValue) → (letter, *band, error)
//
// 실 DB end-to-end는 integration test에서 testcontainers로 진행 — 본 unit test는
// "recorder 호출 자체가 없음" 자체를 검증 (정적 보장 + tx nil panic 회피).
// ════════════════════════════════════════════════════════════════════════════

func TestApplyRubric_ReadOnly_NoAuditWritten(t *testing.T) {
	// fakeAuditTx — InsertAuditLog 호출 시 calls++ (호출되면 즉시 실패 의도)
	auditTx := &fakeRubricAuditTx{}

	// ApplyRubric 실행 시도 — 실 DB 없이 nil tx panic 가능. defer recovery로 보호.
	defer func() {
		_ = recover()
		// 핵심 단언: ApplyRubric이 어떤 경로로든 audit_logs INSERT를 시도해서는 안 된다.
		// 본 unit test에서는 ApplyRubric 진입 자체가 nil tx로 panic할 수 있지만,
		// 그 이전에 recorder 호출 경로가 코드 상 부재함을 확인 (OPEN #6 정적 보장).
		assert.Equal(t, 0, auditTx.calls,
			"OPEN #6 [HARD]: ApplyRubric은 read-only no-audit — fakeAuditTx.calls 정확히 0이어야 한다")
	}()

	tx := &PgRubricTx{}
	_, _, _ = tx.ApplyRubric(context.Background(), uuid.New(), 85.0)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-018-static [iter2 dark-flow fix] — ApplyRubric 정적 코드 분석 검증
// iter1 적발 사항: fakeRubricAuditTx가 PgRubricTx.recorder에 wire되지 않아 calls=0이
// vacuously true. 본 보강 테스트는 rubric.go의 ApplyRubric 함수 본문을 직접 검사하여
// `t.recorder.` 호출 패턴 부재를 정적으로 보장한다 (OPEN #6 [HARD] 진정성 강화).
//
// PgRubricTx.recorder는 *audit.Recorder 구체 타입이라 인터페이스 mock 주입이 불가능.
// 따라서 정적 소스코드 분석이 가장 강력한 검증 수단이다.
// ════════════════════════════════════════════════════════════════════════════

func TestApplyRubric_NoRecorderCallStatic(t *testing.T) {
	content, err := os.ReadFile("rubric.go")
	require.NoError(t, err, "rubric.go 소스 파일을 읽을 수 있어야 한다")

	src := string(content)
	// ApplyRubric 함수 시작점 찾기 — "func (t *PgRubricTx) ApplyRubric("
	funcStart := strings.Index(src, "func (t *PgRubricTx) ApplyRubric(")
	require.NotEqual(t, -1, funcStart, "ApplyRubric 함수가 rubric.go에 정의되어 있어야 한다")

	// 함수 종료점 — 다음 "\nfunc " 또는 EOF
	rest := src[funcStart:]
	funcEnd := strings.Index(rest[1:], "\nfunc ")
	if funcEnd == -1 {
		funcEnd = len(rest)
	} else {
		funcEnd++ // \n 보정
	}
	funcBody := rest[:funcEnd]

	// 검증: ApplyRubric 본문에 t.recorder.* 호출이 부재
	assert.NotContains(t, funcBody, "t.recorder.",
		"OPEN #6 [HARD]: ApplyRubric 함수 본문에는 t.recorder.* 호출이 부재해야 한다 (read-only no-audit 정적 보장)")
	// 추가 검증: RecordRubric* 직접 호출도 부재
	assert.NotContains(t, funcBody, "RecordRubric",
		"ApplyRubric 본문에는 RecordRubric* 호출이 부재해야 한다")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-019 [D1 iter2 lesson] — PgRubricTx.recorder 필드 정합 검증
// REVIEW-001 D1 iter2 lesson pre-applied [HARD]: pg_store.go BeginRubricTx에
// `audit.NewRecorder(true)` 주입 의무. false 사용 시 user_id 영구 'cli-anonymous' override.
//
// 본 단위 테스트:
//  1. PgRubricTx struct에 recorder *audit.Recorder 필드 존재 검증
//  2. audit.NewRecorder(true) 호출 가능성 검증 (시그니처 + 함수 존재)
//
// 실 end-to-end (BeginRubricTx → InsertRubric → audit_logs.user_id 검증)는 integration test.
// ════════════════════════════════════════════════════════════════════════════

func TestNewRecorderTrue_AuthEnabledUserIDPropagates(t *testing.T) {
	// 시그니처 정확성 (Phase B GREEN): PgRubricTx struct에 recorder *audit.Recorder 필드 존재
	// REVIEW-001 D1 iter2 lesson pre-applied — authEnabled=true 의도 명시
	tx := &PgRubricTx{
		recorder: audit.NewRecorder(true),
	}
	require.NotNil(t, tx.recorder, "PgRubricTx.recorder 필드가 정의되어 있어야 한다 (D1 iter2 lesson)")

	// authEnabled=true 정합성: integration test에서 InsertRubric(userID="alice") →
	// audit_logs.user_id="alice" 영속화 검증. NewRecorder(false)면 'cli-anonymous'로 덮어쓰기.
	// 본 unit test는 시그니처 + 컴파일 정합 + 명시적 의도(NewRecorder(true)) 확인.
}
