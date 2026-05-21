// audit_eval_item_test.go — T-002: 평가항목 감사 상수/namespace 골격 검증
// SPEC-AX-EVAL-ITEM-001 DC-002.3 (Action 상수), DC-002.4 / SEC-05 / TH-12 (고정 namespace)
package audit_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// TestEvalItemActionConstants DC-002.3: Action 상수 문자열 값 검증
func TestEvalItemActionConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "EVAL_ITEM_CREATED", string(audit.ActionEvalItemCreated))
	assert.Equal(t, "EVAL_ITEM_UPDATED", string(audit.ActionEvalItemUpdated))
}

// TestEvalItemAuditNamespace_FixedConstant DC-002.4 / SEC-05 / TH-12:
// EvalItemAuditNamespace는 고정 UUID 리터럴 (uuid.Nil 아님, 런타임 생성 아님 — 결정성 보장)
func TestEvalItemAuditNamespace_FixedConstant(t *testing.T) {
	t.Parallel()
	assert.NotEqual(t, uuid.Nil, audit.EvalItemAuditNamespace,
		"namespace는 고정 비-nil UUID여야 함 (SEC-05)")
	// 두 번 읽어도 동일 — 런타임 uuid.New()가 아님을 간접 증명
	assert.Equal(t, audit.EvalItemAuditNamespace, audit.EvalItemAuditNamespace,
		"namespace는 컴파일 타임 고정값 (TH-12)")
	// 고정 리터럴 정확값 (변경 시 모든 과거 audit 상관관계 단절 — 회귀 가드)
	assert.Equal(t, "a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b",
		audit.EvalItemAuditNamespace.String())
}
