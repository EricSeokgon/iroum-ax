// recorder_eval_item_test.go — T-007: RecordEvalItemCreated/Updated 단위 테스트
// SPEC-AX-EVAL-ITEM-001 §6.6 AUD-1:
//
//	DC-007.1~7.6  RecordEvalItemCreated 감사 row + AUD-1 surrogate resource_id + DetailsJSON
//	DC-007.7      RecordEvalItemUpdated 동일 계약
//	DC-007.8~7.10 결정성(동일 hierarchy_code → byte-identical) / 충돌저항(상이 → 상이) / RFC4122 v5
//	DC-007.3/E-07 resource_id != uuid.Nil
//	DC-010.1~10.2 외부 SaaS SDK 미import (audit/store 데이터 주권 — UBI-001 정적 검사)
//
// 기존 recorder_test.go의 captureTx 패턴 재사용 (동일 패키지 audit_test)
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

// TestRecorder_RecordEvalItemCreated DC-007.1~7.6 / AC-EVALITEM-003-1:
// EVAL_ITEM_CREATED 감사 row + AUD-1 결정적 UUIDv5 resource_id + 실 식별자 DetailsJSON
func TestRecorder_RecordEvalItemCreated(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false) // authEnabled=false → cli-anonymous
	tx := &captureTx{}
	ctx := context.Background()

	const (
		itemID = "AX-SAFETY-ORG-01"
		hc     = "AX.SAFETY.ORG.01"
		parent = "AX-SAFETY"
		level  = 2
	)
	err := recorder.RecordEvalItemCreated(ctx, tx, itemID, hc, parent, level, "")
	require.NoError(t, err)

	require.Len(t, tx.Captured, 1, "정확히 1개의 감사 이벤트")
	ev := tx.Captured[0]
	assert.Equal(t, audit.ActionEvalItemCreated, ev.Action)
	assert.Equal(t, "evaluation_item", ev.ResourceType)
	assert.Equal(t, audit.DefaultUserID, ev.UserID, "authEnabled=false → cli-anonymous")
	assert.False(t, ev.Timestamp.IsZero(), "Timestamp NOT NULL")

	// DC-007.2: resource_id == uuid.NewSHA1(namespace, hierarchyCode) — 테스트에서 직접 계산
	expected := uuid.NewSHA1(audit.EvalItemAuditNamespace, []byte(hc))
	assert.Equal(t, expected, ev.ResourceID, "AUD-1 결정적 UUIDv5 surrogate")
	// DC-007.3 / E-07: != uuid.Nil
	assert.NotEqual(t, uuid.Nil, ev.ResourceID, "resource_id는 uuid.Nil이 아님 (parseResourceID 폴백 경로 미사용)")
	// DC-007.4: 원시 계층코드 문자열이 아님 (VARCHAR(64)는 uuid.UUID에 들어갈 수 없음)
	assert.NotEqual(t, itemID, ev.ResourceID.String())
	// DC-007.10: RFC 4122 version 5
	assert.Equal(t, uuid.Version(5), ev.ResourceID.Version(), "RFC 4122 v5 (SHA-1 name-based)")

	// DC-007.5/7.6: 실 식별자는 DetailsJSON
	var details map[string]any
	require.NoError(t, json.Unmarshal(ev.DetailsJSON, &details))
	assert.Equal(t, itemID, details["eval_item_id"])
	assert.Equal(t, hc, details["hierarchy_code"])
	assert.Equal(t, parent, details["parent_id"])
	assert.Equal(t, "2", details["level"], "level은 문자열 \"2\"")
}

// TestRecorder_RecordEvalItemCreated_RootNoParent 루트(parent 없음) — parent_id 키 부재
func TestRecorder_RecordEvalItemCreated_RootNoParent(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	err := recorder.RecordEvalItemCreated(ctx, tx, "AX-SAFETY", "AX.SAFETY", "", 1, "")
	require.NoError(t, err)
	require.Len(t, tx.Captured, 1)

	var details map[string]any
	require.NoError(t, json.Unmarshal(tx.Captured[0].DetailsJSON, &details))
	assert.Equal(t, "AX-SAFETY", details["eval_item_id"])
	_, hasParent := details["parent_id"]
	assert.False(t, hasParent, "루트는 parent_id 키 부재 (빈 parent)")
}

// TestRecorder_RecordEvalItemUpdated DC-007.7 / AC-EVALITEM-003-2:
// EVAL_ITEM_UPDATED — 동일 resource_id 산출 로직, 동일 DetailsJSON 계약
func TestRecorder_RecordEvalItemUpdated(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()

	const (
		itemID = "AX-LEAF"
		hc     = "AX.SAFETY.LEAF"
	)
	err := recorder.RecordEvalItemUpdated(ctx, tx, itemID, hc, "AX-SAFETY", 3, "")
	require.NoError(t, err)
	require.Len(t, tx.Captured, 1)
	ev := tx.Captured[0]
	assert.Equal(t, audit.ActionEvalItemUpdated, ev.Action)
	assert.Equal(t, "evaluation_item", ev.ResourceType)
	assert.Equal(t, uuid.NewSHA1(audit.EvalItemAuditNamespace, []byte(hc)), ev.ResourceID)
	assert.Equal(t, uuid.Version(5), ev.ResourceID.Version())

	var details map[string]any
	require.NoError(t, json.Unmarshal(ev.DetailsJSON, &details))
	assert.Equal(t, itemID, details["eval_item_id"])
	assert.Equal(t, hc, details["hierarchy_code"])
}

// TestRecorder_EvalItemAUD1_Determinism DC-007.8/7.9 / E-08/E-09 / AC-EVALITEM-003-E2-1:
// 동일 hierarchy_code → byte-identical resource_id, 상이 hierarchy_code → 상이 resource_id
func TestRecorder_EvalItemAUD1_Determinism(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	ctx := context.Background()

	// 동일 hierarchy_code를 created + updated 두 번 기록 → resource_id byte-identical
	txC := &captureTx{}
	require.NoError(t, recorder.RecordEvalItemCreated(ctx, txC, "AX-X", "AX.SAFETY.ORG.01", "AX-SAFETY", 2, ""))
	txU := &captureTx{}
	require.NoError(t, recorder.RecordEvalItemUpdated(ctx, txU, "AX-X", "AX.SAFETY.ORG.01", "AX-SAFETY", 2, ""))
	require.Len(t, txC.Captured, 1)
	require.Len(t, txU.Captured, 1)
	assert.Equal(t, txC.Captured[0].ResourceID, txU.Captured[0].ResourceID,
		"동일 hierarchy_code → byte-identical resource_id (결정성, EvalItemAuditNamespace 고정 증명)")

	// 상이 hierarchy_code → 상이 resource_id (충돌 저항)
	tx2 := &captureTx{}
	require.NoError(t, recorder.RecordEvalItemCreated(ctx, tx2, "AX-Y", "AX.SAFETY.ORG.02", "AX-SAFETY", 2, ""))
	require.Len(t, tx2.Captured, 1)
	assert.NotEqual(t, txC.Captured[0].ResourceID, tx2.Captured[0].ResourceID,
		"상이 hierarchy_code → 상이 resource_id (충돌 저항 — hierarchy_code별 audit 그룹핑 가능)")
}

// TestRecorder_EvalItemDefaultUserID cli-anonymous 기본값 + authEnabled=true 전파
func TestRecorder_EvalItemDefaultUserID(t *testing.T) {
	t.Parallel()
	recorder := audit.NewRecorder(false)
	tx := &captureTx{}
	ctx := context.Background()
	require.NoError(t, recorder.RecordEvalItemCreated(ctx, tx, "AX-A", "AX.A", "", 1, ""))
	require.Len(t, tx.Captured, 1)
	assert.Equal(t, "cli-anonymous", tx.Captured[0].UserID,
		"authEnabled=false + 빈 userID → cli-anonymous (SEC-06 — resolveUserID 재사용)")

	rec2 := audit.NewRecorder(true)
	tx2 := &captureTx{}
	require.NoError(t, rec2.RecordEvalItemUpdated(ctx, tx2, "AX-A", "AX.A", "", 1, "real-user"))
	require.Len(t, tx2.Captured, 1)
	assert.Equal(t, "real-user", tx2.Captured[0].UserID)
}

// TestEvalItem_NoExternalSDKImports DC-010.1/10.2 / AC-EVALITEM-UBI-001:
// internal/store, internal/audit 의존성에 외부 SaaS SDK 부재 (데이터 주권 정적 검사)
func TestEvalItem_NoExternalSDKImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/ircp/iroum-ax/apps/control-plane/internal/store/...",
		"github.com/ircp/iroum-ax/apps/control-plane/internal/audit/...",
	).CombinedOutput()
	require.NoError(t, err, "go list -deps: %s", string(out))
	deps := string(out)
	for _, f := range []string{
		"github.com/aws/", "aws-sdk", "github.com/minio/", "minio-go",
		"cloud.google.com/", "github.com/Azure/", "azure-storage-blob",
	} {
		assert.NotContains(t, deps, f,
			"외부 SaaS SDK(%s) 의존 금지 — REQ-EVALITEM-UBI-001 데이터 주권", f)
	}
	// 신규 외부 dep 0건 보강: uuid는 기존 import (TH-10)
	assert.True(t, strings.Contains(deps, "github.com/google/uuid"),
		"google/uuid는 기존 의존 (신규 외부 dep 0건 — AUD-1 동일 패키지 uuid.NewSHA1)")
}
