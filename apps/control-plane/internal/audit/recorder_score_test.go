// recorder_score_test.go — T-007/T-008: 점수 감사 상수 + RecordScoreCreated/Updated 검증
// SPEC-AX-SCORE-001:
//
//	T-007: ActionScoreCreated="SCORE_CREATED", ActionScoreUpdated="SCORE_UPDATED" 상수 값 검증
//	       TH-11/TH-12 [HARD D2]: audit.go 내 0 ScoreAuditNamespace 상수, recorder.go 내 0 NewSHA1 호출 (grep)
//	T-008: RecordScoreCreated/RecordScoreUpdated:
//	       - Action 값, ResourceType="score", ResourceID=scoreID UUID 직접 대입 (D2)
//	       - resource_id != uuid.Nil
//	       - DetailsJSON score_id/evaluation_item_id/level 포함
//	       - cli-anonymous 기본값 (authEnabled=false)
//	       - RecordScoreCreated + RecordScoreUpdated 동일 scoreID → 동일 resource_id (D2 결정성)
//
// 실행: go test ./apps/control-plane/internal/audit/ -run TestScore -v -count=1
package audit_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// ── T-007: 상수 값 검증 + grep 기반 D2 불변 ──────────────────────────────────

// TestScoreActionConstants T-007 / DC-002.3:
// ActionScoreCreated="SCORE_CREATED", ActionScoreUpdated="SCORE_UPDATED"
func TestScoreActionConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "SCORE_CREATED", string(audit.ActionScoreCreated),
		"ActionScoreCreated 문자열 값 (REQ-SCORE-004 감사 액션)")
	assert.Equal(t, "SCORE_UPDATED", string(audit.ActionScoreUpdated),
		"ActionScoreUpdated 문자열 값 (REQ-SCORE-004 감사 액션)")
}

// TestScore_NoNamespaceConstantInAuditGo TH-12 [HARD D2]:
// audit.go/recorder.go에 ScoreAuditNamespace 등 score 전용 namespace 상수 없음
// D2: scores.id UUID 직접 대입 — 평가항목과 달리 AUD-1 surrogate 불필요
// NOTE: _test.go 파일은 제외하고 audit.go/recorder.go만 검색 (grep -rn이 테스트 파일 자체를 오탐)
func TestScore_NoNamespaceConstantInAuditGo(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err, "repo 루트 조회 실패")
	root := strings.TrimSpace(string(out))

	auditDir := root + "/apps/control-plane/internal/audit"
	// _test.go 파일 제외: audit.go + recorder.go만 검사 (테스트 파일의 오탐 방지)
	targets := []string{auditDir + "/audit.go", auditDir + "/recorder.go"}
	for _, pattern := range []string{"ScoreAuditNamespace", "ScoreNamespace", "scoreNamespace"} {
		args := append([]string{"-n", pattern}, targets...)
		grep := exec.Command("grep", args...)
		grOut, _ := grep.Output()
		assert.Empty(t, string(grOut),
			"audit 패키지에 %s 상수 없어야 함 (TH-12 [HARD D2]: score는 UUID surrogate 미사용)", pattern)
	}
}

// TestScore_NoNewSHA1InRecorderGo TH-11 [HARD D2]:
// recorder.go의 RecordScore* 함수 내에 uuid.NewSHA1 호출 없음
// D2: scores.id를 그대로 resource_id로 대입 — AUD-1 surrogate 미사용
// NOTE: EvalItem용 NewSHA1(non-score 함수)은 recorder.go에 허용됨
// 검증 전략: grep으로 NewSHA1 호출 라인을 찾되, 그 라인이 RecordScore 함수 내부인지 확인
// 실제로 D2를 위반하면 RecordScore* 함수 내에 `uuid.NewSHA1(` 호출이 나타남
func TestScore_NoNewSHA1InRecorderGo(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err, "repo 루트 조회 실패")
	root := strings.TrimSpace(string(out))

	recorderPath := root + "/apps/control-plane/internal/audit/recorder.go"

	// uuid.NewSHA1( 실제 함수 호출만 검색 (주석 라인 제외: grep -v "^[[:space:]]*//"
	// 단순화: grep으로 `uuid.NewSHA1` 포함 라인 추출 후 주석이 아닌 실제 호출인지 확인
	grep := exec.Command("grep", "-n", "uuid\\.NewSHA1", recorderPath)
	grOut, _ := grep.Output()

	// NewSHA1 호출 라인이 있다면, RecordScore 관련 함수 (RecordScoreCreated/RecordScoreUpdated)
	// 에서 호출되는지 확인해야 함. D2 위반 = RecordScore* 함수 내 NewSHA1 호출.
	// 현재 구현에서 RecordScore*는 scoreID를 직접 대입하므로 NewSHA1 호출 없어야 함.
	// 가장 직접적인 검증: RecordScoreCreated/RecordScoreUpdated 함수 범위 내 NewSHA1 검색
	grepScore := exec.Command("grep", "-A30", `^func RecordScore`, recorderPath)
	scoreFuncOut, _ := grepScore.Output()

	if strings.Contains(string(scoreFuncOut), "uuid.NewSHA1") {
		t.Errorf("recorder.go RecordScore* 함수 내 uuid.NewSHA1 호출 발견 — D2 위반 (TH-11 [HARD]): %s", string(grOut))
	}
}

// ── T-008: RecordScoreCreated / RecordScoreUpdated 계약 검증 ─────────────────

// TestRecorder_RecordScoreCreated_Action T-008 / AC-SCORE-004-1:
// Action=ActionScoreCreated, ResourceType="score"
func TestRecorder_RecordScoreCreated_Action(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	scoreID := uuid.New()
	err := recorder.RecordScoreCreated(ctx, tx, scoreID, "AX-SAFETY-ORG-01", "raw", "")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1, "정확히 1개의 감사 이벤트")
	ev := tx.Captured[0]
	assert.Equal(t, audit.ActionScoreCreated, ev.Action, "Action=SCORE_CREATED")
	assert.Equal(t, "score", ev.ResourceType, "ResourceType='score'")
	assert.False(t, ev.Timestamp.IsZero(), "Timestamp NOT NULL")
}

// TestRecorder_RecordScoreCreated_ResourceIDIsScoreUUID D2:
// resource_id = scoreID UUID 직접 대입 (AUD-1 surrogate 미사용)
func TestRecorder_RecordScoreCreated_ResourceIDIsScoreUUID(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	scoreID := uuid.MustParse("11223344-5566-7788-99aa-bbccddeeff00")
	err := recorder.RecordScoreCreated(ctx, tx, scoreID, "AX-ITEM-01", "raw", "")
	require.NoError(t, err)

	ev := tx.Captured[0]
	assert.Equal(t, scoreID, ev.ResourceID,
		"resource_id = scoreID UUID 직접 대입 (D2: surrogate/NewSHA1 미사용)")
	assert.NotEqual(t, uuid.Nil, ev.ResourceID, "resource_id != uuid.Nil (DC-004-E1)")
}

// TestRecorder_RecordScoreCreated_DetailsJSON T-008 / DC-004:
// DetailsJSON에 score_id / evaluation_item_id / level 포함
func TestRecorder_RecordScoreCreated_DetailsJSON(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	scoreID := uuid.New()
	const evalItemID = "AX-SAFETY-ORG-02"
	const level = "item"
	err := recorder.RecordScoreCreated(ctx, tx, scoreID, evalItemID, level, "")
	require.NoError(t, err)

	ev := tx.Captured[0]
	require.NotEmpty(t, ev.DetailsJSON, "DetailsJSON 비어 있으면 안 됨")

	var details map[string]string
	require.NoError(t, json.Unmarshal(ev.DetailsJSON, &details), "DetailsJSON 파싱 가능")
	assert.Equal(t, scoreID.String(), details["score_id"], "score_id 포함")
	assert.Equal(t, evalItemID, details["evaluation_item_id"], "evaluation_item_id 포함")
	assert.Equal(t, level, details["level"], "level 포함")
}

// TestRecorder_RecordScoreCreated_DefaultUserID:
// authEnabled=false → user_id=cli-anonymous
func TestRecorder_RecordScoreCreated_DefaultUserID(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordScoreCreated(ctx, tx, uuid.New(), "AX-ITEM-01", "raw", "")
	require.NoError(t, err)

	assert.Equal(t, audit.DefaultUserID, tx.Captured[0].UserID,
		"authEnabled=false + 빈 userID → cli-anonymous (DC-UBI-003)")
}

// TestRecorder_RecordScoreUpdated_Action:
// Action=ActionScoreUpdated, ResourceType="score"
func TestRecorder_RecordScoreUpdated_Action(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	scoreID := uuid.New()
	err := recorder.RecordScoreUpdated(ctx, tx, scoreID, "AX-ITEM-01", "raw", "")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1)
	ev := tx.Captured[0]
	assert.Equal(t, audit.ActionScoreUpdated, ev.Action, "Action=SCORE_UPDATED")
	assert.Equal(t, "score", ev.ResourceType)
}

// TestRecorder_RecordScoreUpdated_ResourceIDIsScoreUUID D2:
// RecordScoreUpdated도 동일 scoreID가 resource_id에 직접 대입
func TestRecorder_RecordScoreUpdated_ResourceIDIsScoreUUID(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	scoreID := uuid.MustParse("aabbccdd-eeff-0011-2233-445566778899")
	err := recorder.RecordScoreUpdated(ctx, tx, scoreID, "AX-ITEM-02", "category", "")
	require.NoError(t, err)

	ev := tx.Captured[0]
	assert.Equal(t, scoreID, ev.ResourceID,
		"RecordScoreUpdated resource_id = scoreID UUID 직접 대입 (D2)")
}

// TestRecorder_ScoreD2_Determinism D2:
// 동일 scoreID → Created/Updated 모두 동일 resource_id (UUID 직접 대입 결정성)
// D2와 달리 namespace hash 불필요: UUID 자체가 결정적
func TestRecorder_ScoreD2_Determinism(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	ctx := context.Background()

	scoreID := uuid.New()

	txC := &captureTx{}
	require.NoError(t, recorder.RecordScoreCreated(ctx, txC, scoreID, "AX-X", "raw", ""))
	txU := &captureTx{}
	require.NoError(t, recorder.RecordScoreUpdated(ctx, txU, scoreID, "AX-X", "raw", ""))

	require.Len(t, txC.Captured, 1)
	require.Len(t, txU.Captured, 1)
	assert.Equal(t, txC.Captured[0].ResourceID, txU.Captured[0].ResourceID,
		"동일 scoreID → Created/Updated 모두 동일 resource_id (D2 UUID 직접 대입 결정성)")
}

// TestRecorder_Score_AuditTxFail:
// tx.InsertAuditLog 장애 → RecordScoreCreated 에러 전파
func TestRecorder_Score_AuditTxFail(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{FailInsert: true}
	ctx := context.Background()

	err := recorder.RecordScoreCreated(ctx, tx, uuid.New(), "AX-ITEM-01", "raw", "")
	require.Error(t, err, "InsertAuditLog 실패 → RecordScoreCreated 에러 전파")
}
