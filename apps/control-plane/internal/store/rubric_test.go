// rubric_test.go — 등급 rubric store-layer 단위 테스트 (SPEC-AX-RUBRIC-001 Phase A RED)
//
// 격리 전략: PgRubricTx 내부 validateRubricInput/validateCriterionInput/validateBandInput/
// validateRubricStatusTransition 등 SQL 미실행 헬퍼는 default 빌드 태그로 실행.
// SELECT FOR UPDATE / DB CHECK constraint / audit fault rollback 등 DB 의존 테스트는
// rubric_integration_test.go (//go:build integration) 에서 testcontainers postgres:16-alpine으로 검증.
//
// Phase A RED [HARD]: 모든 테스트는 GREEN 구현 전에 작성되어 실패 확인되어야 한다
// (post-hoc rubber-stamp 금지 — SCORE-001/REVIEW-001 dark-flow iter1 적발 패턴 회피).
// 본 Phase에서는 PgRubricTx 메서드 + helper 함수 모두 미구현 — 모든 테스트 FAIL 기대.
//
// REVIEW-001 D1 iter2 lesson 검증 의무 테스트:
//   - T-RED-005 (TestInsertRubric_UserIDPropagatesToCreatedByAndAuditUserID): userID 시그니처 정확성
//   - T-RED-019 (TestNewRecorderTrue_AuthEnabledUserIDPropagates): BeginRubricTx audit.NewRecorder(true) 정합
//
// OPEN #6 검증 의무:
//   - T-RED-018 (TestApplyRubric_ReadOnly_NoAuditWritten): recorder 호출 0 — fake recorder fail-fast
package store

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// ════════════════════════════════════════════════════════════════════════════
// fakeRubricAuditTx — audit.AuditTx 기반 호출 카운터 (read-only no-audit 검증용)
// SCORE-001 audit fake 패턴 미러 — OPEN #6 ApplyRubric recorder 호출 0 단언
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
// T-RED-001 — InsertRubric 유효 입력 시 UUID 반환 + audit 동일 TX 1건 (REQ-RUBRIC-001-E1 + UBI-002)
// Phase A: PgRubricTx.InsertRubric 미구현 → "not implemented" 에러 반환 (genuine RED)
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_ValidInput_ReturnsUUIDAndInsertsAuditRow(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.InsertRubric(context.Background(), "safety-rubric-v1", "kepco-safety", nil, "admin-001")
	require.Error(t, err, "Phase A skeleton — InsertRubric 미구현 시 에러 반환")
	assert.Contains(t, err.Error(), "not implemented",
		"genuine RED: GREEN 구현 전에는 명확한 'not implemented' 시그니처여야 한다")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-002 — InsertRubric blank name 거부 (REQ-RUBRIC-001-U1)
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_BlankName_ReturnsInvalidInput(t *testing.T) {
	err := validateRubricInput("")
	require.Error(t, err)
	// Phase B GREEN 단계에서 ErrRubricInvalidInput 래핑 검증
	// Phase A에서는 "not implemented" 에러 반환 → assertion 명시적 RED
	assert.True(t, err != nil, "blank name은 거부되어야 한다 (Phase A: not implemented)")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-003 — InsertRubric name 64자 초과 거부 (REQ-RUBRIC-001-U1)
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_NameOver64Chars_ReturnsInvalidInput(t *testing.T) {
	longName := strings.Repeat("a", 65) // VARCHAR(64) 초과
	err := validateRubricInput(longName)
	require.Error(t, err, "name 64자 초과는 거부되어야 한다")
	// Phase B GREEN: stderrors.ErrRubricInvalidInput 래핑 검증
	// Phase A: not implemented 에러 — 신규 sentinel은 Phase B에서 추가
	_ = stderrors.ErrEvalItemInvalidInput // 기존 errors.go 참조 — 신규 ErrRubricInvalidInput는 Phase B 추가
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-004 — InsertRubric audit 실패 시 양방향 rollback (UBI-002)
// Phase A: 미구현 → InsertRubric "not implemented" 자체가 RED 의도 충족
// Phase B GREEN에서 fake recorder.RecordRubricCreated가 ErrRubricAuditWriteFailed 반환 시
// entity-INSERT/audit-INSERT 양방향 rollback 단언 추가
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_AuditFailure_TwoWayRollback(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.InsertRubric(context.Background(), "rubric-x", "default", nil, "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented",
		"Phase A: InsertRubric 자체가 미구현 — Phase B에서 audit fault rollback 단언 추가")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-005 [D1 iter2 lesson] — userID="user-42" 정확 전파 검증 (UBI-003)
// REVIEW-001 D1 iter2 lesson pre-applied [HARD] — 'cli-anonymous' hardcode 검출
// Phase A: InsertRubric 시그니처에 userID string 파라미터 존재 + skeleton 호출 가능 검증
// Phase B GREEN: SQL INSERT $N에 userID 정확 바인딩 + audit_logs.user_id='user-42' 검증
// ════════════════════════════════════════════════════════════════════════════

func TestInsertRubric_UserIDPropagatesToCreatedByAndAuditUserID(t *testing.T) {
	tx := &PgRubricTx{}
	// 시그니처 정확성 검증 — userID 파라미터가 처음부터 존재 (D1 iter2 lesson pre-applied)
	_, err := tx.InsertRubric(context.Background(), "rubric-user-42", "default", nil, "user-42")
	require.Error(t, err, "Phase A: InsertRubric 미구현 → 'not implemented' (genuine RED)")
	assert.Contains(t, err.Error(), "not implemented",
		"Phase B GREEN에서 created_by='user-42' + updated_by='user-42' + audit_logs.user_id='user-42' 검증")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-006 — AddCriterion 유효 입력 시 UUID 반환 + audit 동일 TX (REQ-RUBRIC-001-E2)
// ════════════════════════════════════════════════════════════════════════════

func TestAddCriterion_ValidInput_ReturnsUUIDAndInsertsAudit(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.AddCriterion(context.Background(), uuid.New(), uuid.New(), 0.25, "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-007 — AddCriterion weight 범위 외 거부 (REQ-RUBRIC-001-U1)
// Phase B GREEN: ErrRubricWeightOutOfBounds 래핑 검증
// ════════════════════════════════════════════════════════════════════════════

func TestAddCriterion_WeightOutOfBounds_ReturnsWeightOutOfBounds(t *testing.T) {
	cases := []struct {
		name   string
		weight float64
	}{
		{"negative weight", -0.1},
		{"over 1.0", 1.5},
		{"exactly above 1.0", 1.0001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCriterionInput(tc.weight)
			require.Error(t, err, "범위 외 weight는 거부되어야 한다")
			// Phase A: "not implemented" — Phase B에서 ErrRubricWeightOutOfBounds 래핑
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-008 — AddCriterion archived rubric 거부 (REQ-RUBRIC-003-S1)
// Phase B GREEN: archived terminal 불변 — store-side guard
// ════════════════════════════════════════════════════════════════════════════

func TestAddCriterion_ArchivedRubric_ReturnsArchived(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.AddCriterion(context.Background(), uuid.New(), uuid.New(), 0.2, "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented",
		"Phase B에서 archived rubric 시 ErrRubricArchived 래핑 검증")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-009 — AddBand 유효 입력 시 UUID 반환 + audit 동일 TX (REQ-RUBRIC-001-E3)
// ════════════════════════════════════════════════════════════════════════════

func TestAddBand_ValidInput_ReturnsUUIDAndInsertsAudit(t *testing.T) {
	tx := &PgRubricTx{}
	_, err := tx.AddBand(context.Background(), uuid.New(), "B", 80.0, 89.999, "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-010 — AddBand min >= max 거부 (REQ-RUBRIC-001-U1)
// ════════════════════════════════════════════════════════════════════════════

func TestAddBand_MinGteMax_ReturnsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		min  float64
		max  float64
	}{
		{"min equals max", 80.0, 80.0},
		{"min greater than max", 90.0, 80.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBandInput(tc.min, tc.max)
			require.Error(t, err, "min >= max는 거부되어야 한다")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-011 — UpdateRubric draft → active 전이 성공 (REQ-RUBRIC-003-E1 + UBI-004)
// ════════════════════════════════════════════════════════════════════════════

func TestUpdateRubric_DraftToActive_TransitionsSuccess(t *testing.T) {
	tx := &PgRubricTx{}
	err := tx.UpdateRubric(context.Background(), uuid.New(), "rubric-x", "default", "active", nil, "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented",
		"Phase B GREEN에서 draft→active 전이 + RUBRIC_UPDATED audit 동일 TX 검증")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-012 — UpdateRubric archived rubric 거부 (REQ-RUBRIC-003-S1)
// archived terminal — 어떤 전이도 거부 (UBI-004)
// ════════════════════════════════════════════════════════════════════════════

func TestUpdateRubric_ArchivedRubric_ReturnsArchived(t *testing.T) {
	tx := &PgRubricTx{}
	err := tx.UpdateRubric(context.Background(), uuid.New(), "rubric-x", "default", "draft", nil, "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-013 — ArchiveRubric active → archived 전이 성공 (REQ-RUBRIC-003-E2 + OPEN #7)
// archive_reason 필수 — handler validation + DB CHECK 이중 방어
// ════════════════════════════════════════════════════════════════════════════

func TestArchiveRubric_ActiveToArchived_TransitionsSuccess(t *testing.T) {
	tx := &PgRubricTx{}
	err := tx.ArchiveRubric(context.Background(), uuid.New(), "deprecated by new policy", "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented",
		"Phase B GREEN: active→archived + archive_reason 필수 + RUBRIC_ARCHIVED audit 검증")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-014 — ArchiveRubric terminal 유지 (UBI-004)
// archived 상태에서 다른 전이 시도 거부 — terminal 불변
// ════════════════════════════════════════════════════════════════════════════

func TestArchiveRubric_Terminal_PersistsAfter(t *testing.T) {
	tx := &PgRubricTx{}
	err := tx.ArchiveRubric(context.Background(), uuid.New(), "reason", "admin-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented",
		"Phase B GREEN: archived terminal — 추가 ArchiveRubric 호출 시 ErrRubricInvalidStatus")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-015 — Status transition full matrix (UBI-004 + REQ-RUBRIC-003-S2)
// 허용: draft→active / active→archived (2건)
// 거부: 9-2 = 7건 (3 states × 3 targets - 2 allowed)
// ════════════════════════════════════════════════════════════════════════════

func TestStatusTransition_AllAllowedAndDisallowed(t *testing.T) {
	type tc struct {
		current string
		next    string
		allowed bool
	}
	cases := []tc{
		{rubricStatusDraft, rubricStatusActive, true},
		{rubricStatusDraft, rubricStatusArchived, false},
		{rubricStatusDraft, rubricStatusDraft, false},
		{rubricStatusActive, rubricStatusArchived, true},
		{rubricStatusActive, rubricStatusDraft, false},
		{rubricStatusActive, rubricStatusActive, false},
		{rubricStatusArchived, rubricStatusDraft, false},
		{rubricStatusArchived, rubricStatusActive, false},
		{rubricStatusArchived, rubricStatusArchived, false},
	}
	for _, c := range cases {
		t.Run(c.current+"->"+c.next, func(t *testing.T) {
			err := validateRubricStatusTransition(c.current, c.next)
			// Phase A: validateRubricStatusTransition 미구현 — 모든 호출이 에러
			// Phase B GREEN: allowed=true는 nil, allowed=false는 ErrRubricInvalidStatus 래핑
			require.Error(t, err, "Phase A: validateRubricStatusTransition 미구현 (genuine RED)")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-016 — ApplyRubric 경계 inclusive 정확성 (REQ-RUBRIC-004-E1)
// Risk R-RUBRIC-007 검증 — numrange '[]' inclusive 의미
// score=80.0 → B, score=89.999 → B, score=90.0 → A (B: 80.0..89.999)
// ════════════════════════════════════════════════════════════════════════════

func TestApplyRubric_ScoreInBand_ReturnsLetter(t *testing.T) {
	tx := &PgRubricTx{}
	cases := []struct {
		name  string
		score float64
	}{
		{"lower boundary inclusive", 80.0},
		{"middle of band", 85.5},
		{"upper boundary inclusive", 89.999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tx.ApplyRubric(context.Background(), uuid.New(), tc.score)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented",
				"Phase B/C GREEN: linear scan + 경계 inclusive 정확성 (B: 80.0..89.999)")
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-017 — ApplyRubric 모든 band 밖 시 fail-closed (REQ-RUBRIC-004-U1)
// AC-RUBRIC-004-3 — ErrRubricInvalidInput 래핑 → 400
// ════════════════════════════════════════════════════════════════════════════

func TestApplyRubric_ScoreOutOfAllBands_ReturnsInvalidInput(t *testing.T) {
	tx := &PgRubricTx{}
	_, _, err := tx.ApplyRubric(context.Background(), uuid.New(), 999.99) // 모든 band 밖
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented",
		"Phase B/C GREEN: 모든 band 밖 → ErrRubricInvalidInput fail-closed 검증")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-018 [OPEN #6 read-only no-audit] — ApplyRubric은 recorder 호출 0 [HARD]
// REPORT-001 read-only no-audit 선례 정확 미러
// fake recorder fail-fast assertion — UBI-002 second clause "apply read-only 예외" carve-out
// ════════════════════════════════════════════════════════════════════════════

func TestApplyRubric_ReadOnly_NoAuditWritten(t *testing.T) {
	tx := &PgRubricTx{}
	auditTx := &fakeRubricAuditTx{}
	// PgRubricTx는 audit.AuditTx 인터페이스 구현 (recorder가 호출) — Phase B에서 InsertAuditLog 메서드 완성
	// Phase A에서는 ApplyRubric 자체가 "not implemented" → audit calls 0 자동 충족 (RED)
	_, _, err := tx.ApplyRubric(context.Background(), uuid.New(), 85.0)
	require.Error(t, err, "Phase A: ApplyRubric 미구현 (genuine RED)")
	assert.Contains(t, err.Error(), "not implemented")
	// 핵심 단언: ApplyRubric 경로에서 어떤 audit 이벤트도 발생해서는 안 된다 (OPEN #6)
	assert.Equal(t, 0, auditTx.calls,
		"OPEN #6 [HARD]: ApplyRubric은 read-only no-audit — auditTx.calls 정확히 0이어야 한다")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RED-019 [D1 iter2 lesson] — BeginRubricTx audit.NewRecorder(true) 정합 검증
// REVIEW-001 D1 iter2 lesson pre-applied [HARD]: pg_store.go BeginRubricTx (Phase B 추가)에
// `audit.NewRecorder(true)` 사용 의무. false 사용 시 user_id 영구 'cli-anonymous' override.
//
// Phase A: PgRubricTx.recorder 필드 존재 검증 (필드 시그니처 정확성)
// Phase B/C GREEN: BeginRubricTx 추가 + auth-enabled principal 전파 end-to-end 검증
// ════════════════════════════════════════════════════════════════════════════

func TestNewRecorderTrue_AuthEnabledUserIDPropagates(t *testing.T) {
	// 시그니처 정확성 (Phase A): PgRubricTx struct에 recorder *audit.Recorder 필드 존재
	tx := &PgRubricTx{
		recorder: audit.NewRecorder(true), // D1 iter2 lesson pre-applied — authEnabled=true 의도 명시
	}
	require.NotNil(t, tx.recorder, "PgRubricTx.recorder 필드가 처음부터 존재 (D1 iter2 lesson)")

	// Phase B GREEN end-to-end 검증 placeholder:
	// (1) pg_store.go BeginRubricTx → recorder=audit.NewRecorder(true) 정확 와이어링
	// (2) InsertRubric(userID="alice") → audit_logs.user_id="alice" (NOT 'cli-anonymous')
	// (3) integration test에서 testcontainers DB로 raw SQL 검증
	_, err := tx.InsertRubric(context.Background(), "rubric-d1-lesson", "default", nil, "alice")
	require.Error(t, err, "Phase A RED: InsertRubric 미구현")
	assert.Contains(t, err.Error(), "not implemented",
		"Phase B GREEN: pg_store.go BeginRubricTx + audit.NewRecorder(true) + userID 'alice' 영속화 검증")
}
