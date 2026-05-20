// rubric.go — 등급 rubric(Rubric) 도메인 pgx 기반 트랜잭션 skeleton (SPEC-AX-RUBRIC-001)
//
// Phase A (RED): T-IFACE-002 skeleton. 모든 메서드는 미구현 — 컴파일만 통과 + RED 테스트가
// genuine FAIL을 표시하도록 nil/panic/fmt.Errorf("not implemented") 반환.
//
// REVIEW-001 D1 iter2 lesson pre-applied [HARD]:
//   - 모든 mutation 메서드(InsertRubric/UpdateRubric/ArchiveRubric/AddCriterion/AddBand)는
//     처음부터 userID string 파라미터 보유. Phase B GREEN에서 SQL $N에 직접 전파.
//   - 본 Phase A에서는 시그니처만 정확히 명시 (실제 SQL/audit/validation 0).
//
// 상태 머신: draft → active → archived (terminal). archived → 모든 전이 거부 (UBI-004).
// OPEN 결정 strategy.md §1 5요소 적용:
//   - #2: active 1-per-(name, scope) — partial unique idx + handler pre-check (Phase B/C)
//   - #4: band overlap — EXCLUSION USING gist + handler pre-check (Phase B/C)
//   - #6: ApplyRubric read-only no-audit — recorder 호출 0 (Phase B GREEN)
//   - #7: archive_reason 필수 — handler validation + DB CHECK (Phase B/C)
package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// 상태 머신 상수 — Phase B 0006 마이그레이션 CHECK 제약과 정합 예정
const (
	rubricStatusDraft    = "draft"
	rubricStatusActive   = "active"
	rubricStatusArchived = "archived"
)

// PgRubricTx pgx.Tx 래퍼 — RubricTx 인터페이스 구현 (skeleton).
// Phase B에서 PgScoreReviewRequestTx 패턴을 정확 미러하여 완성된다.
// recorder는 동일 t.tx에 audit_logs INSERT 전담 — REVIEW-001 D1 iter2 lesson 정합으로
// authEnabled=true 주입(Phase B pg_store.go BeginRubricTx에서).
//
// @MX:WARN: [AUTO] Phase A skeleton — mutation 메서드 모두 미구현, RED 테스트 의도된 실패
// @MX:REASON: REVIEW-001 D1 iter2 lesson pre-applied — userID 파라미터는 처음부터 시그니처 보유
//
//	GREEN 단계(Phase B)에서 SQL $N 직접 전파 + recorder 동일 TX 기록 추가
type PgRubricTx struct {
	// tx 래핑된 pgx 트랜잭션 (Phase B 설정)
	tx pgx.Tx
	// logger 구조화 로그
	logger *zap.Logger
	// recorder 등급 rubric 감사 이벤트 기록기 — 동일 tx에 audit_logs 1건 INSERT.
	// Phase B에서 audit.NewRecorder(true)로 주입 — REVIEW-001 D1 iter2 lesson pre-applied [HARD].
	recorder *audit.Recorder
}

// validateRubricInput InsertRubric pre-write 검증 (Phase B에서 구현 — name blank/64자 초과 거부)
func validateRubricInput(_ string) error {
	// @MX:TODO: Phase B GREEN — name blank + 64자 초과 검증 + ErrRubricInvalidInput 래핑
	return fmt.Errorf("validateRubricInput not implemented in RED phase")
}

// validateCriterionInput AddCriterion pre-write 검증 (Phase B에서 구현 — weight 0.0-1.0 범위 검사)
func validateCriterionInput(_ float64) error {
	// @MX:TODO: Phase B GREEN — weight < 0 또는 > 1.0 거부 + ErrRubricWeightOutOfBounds 래핑
	return fmt.Errorf("validateCriterionInput not implemented in RED phase")
}

// validateBandInput AddBand pre-write 검증 (Phase B에서 구현 — min >= max 거부)
func validateBandInput(_, _ float64) error {
	// @MX:TODO: Phase B GREEN — min_score >= max_score 거부 + ErrRubricInvalidInput 래핑
	return fmt.Errorf("validateBandInput not implemented in RED phase")
}

// validateRubricStatusTransition 현재 상태 → 목표 상태 전이 허용 여부 (Phase B에서 구현).
// 허용: draft→active / active→archived. 그 외 거부 (UBI-004, REQ-RUBRIC-003-S2).
func validateRubricStatusTransition(_, _ string) error {
	// @MX:TODO: Phase B GREEN — state-machine guard + ErrRubricInvalidStatus 래핑
	return fmt.Errorf("validateRubricStatusTransition not implemented in RED phase")
}

// InsertRubric rubrics 테이블에 새 draft 행 삽입 (Phase A skeleton — 미구현).
// Phase B에서 동일 t.tx에 RUBRIC_CREATED audit 1건 (UBI-002) + userID 전파 (UBI-003 / D1 iter2 lesson).
//
// @MX:TODO: Phase B GREEN — SQL INSERT + recorder.RecordRubricCreated 동일 TX 추가
func (t *PgRubricTx) InsertRubric(
	_ context.Context,
	_, _ string,
	_ map[string]any,
	_ string,
) (uuid.UUID, error) {
	return uuid.Nil, fmt.Errorf("InsertRubric not implemented in RED phase")
}

// GetRubricByID rubric PK로 단건 조회 (Phase A skeleton — 미구현).
// 미존재 시 ErrRubricNotFound 래핑 — raw pgx.ErrNoRows 누출 금지.
func (t *PgRubricTx) GetRubricByID(_ context.Context, _ uuid.UUID) (*Rubric, error) {
	return nil, fmt.Errorf("GetRubricByID not implemented in RED phase")
}

// ListRubrics 필터(status/scope) + 페이지네이션 조회 (Phase A skeleton — 미구현).
func (t *PgRubricTx) ListRubrics(
	_ context.Context,
	_, _ string,
	_, _ int,
) ([]*Rubric, error) {
	return nil, fmt.Errorf("ListRubrics not implemented in RED phase")
}

// CountRubrics 필터에 해당하는 전체 행 수 (Phase A skeleton — 미구현).
func (t *PgRubricTx) CountRubrics(_ context.Context, _, _ string) (int64, error) {
	return 0, fmt.Errorf("CountRubrics not implemented in RED phase")
}

// UpdateRubric rubric 메타/상태 부분 수정 (Phase A skeleton — 미구현).
// Phase B에서 동일 t.tx에 RUBRIC_UPDATED audit 1건 + userID 전파.
func (t *PgRubricTx) UpdateRubric(
	_ context.Context,
	_ uuid.UUID,
	_, _, _ string,
	_ map[string]any,
	_ string,
) error {
	return fmt.Errorf("UpdateRubric not implemented in RED phase")
}

// ArchiveRubric active → archived terminal 전이 (Phase A skeleton — 미구현).
// archive_reason 필수 — Phase B에서 OPEN #7 dual defense (validation pre-store + DB CHECK).
func (t *PgRubricTx) ArchiveRubric(_ context.Context, _ uuid.UUID, _, _ string) error {
	return fmt.Errorf("ArchiveRubric not implemented in RED phase")
}

// AddCriterion rubric_criteria 행 추가 (Phase A skeleton — 미구현).
func (t *PgRubricTx) AddCriterion(
	_ context.Context,
	_, _ uuid.UUID,
	_ float64,
	_ string,
) (uuid.UUID, error) {
	return uuid.Nil, fmt.Errorf("AddCriterion not implemented in RED phase")
}

// AddBand rubric_bands 행 추가 (Phase A skeleton — 미구현).
func (t *PgRubricTx) AddBand(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_, _ float64,
	_ string,
) (uuid.UUID, error) {
	return uuid.Nil, fmt.Errorf("AddBand not implemented in RED phase")
}

// GetCriteriaByRubric 동일 rubric의 모든 criteria 반환 (Phase A skeleton — 미구현).
func (t *PgRubricTx) GetCriteriaByRubric(_ context.Context, _ uuid.UUID) ([]*RubricCriterion, error) {
	return nil, fmt.Errorf("GetCriteriaByRubric not implemented in RED phase")
}

// GetBandsByRubric 동일 rubric의 모든 bands 반환 (Phase A skeleton — 미구현).
func (t *PgRubricTx) GetBandsByRubric(_ context.Context, _ uuid.UUID) ([]*RubricBand, error) {
	return nil, fmt.Errorf("GetBandsByRubric not implemented in RED phase")
}

// ApplyRubric scoreValue → 등급 결정 (Phase A skeleton — 미구현).
// OPEN #6: read-only no-audit — recorder 호출 0 [HARD] (UBI-002 second clause).
func (t *PgRubricTx) ApplyRubric(_ context.Context, _ uuid.UUID, _ float64) (string, *RubricBand, error) {
	return "", nil, fmt.Errorf("ApplyRubric not implemented in RED phase")
}

// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입 (Phase A skeleton — 미구현).
// Phase B에서 PgScoreReviewRequestTx.InsertAuditLog 패턴 정확 미러 — D2 resource_id 직접 대입.
func (t *PgRubricTx) InsertAuditLog(_ context.Context, _ *audit.Event) error {
	return fmt.Errorf("InsertAuditLog not implemented in RED phase")
}

// Commit 현재 트랜잭션을 커밋 (Phase A skeleton — 미구현).
func (t *PgRubricTx) Commit(_ context.Context) error {
	return fmt.Errorf("Commit not implemented in RED phase")
}

// Rollback 현재 트랜잭션을 롤백 (Phase A skeleton — 미구현).
func (t *PgRubricTx) Rollback(_ context.Context) error {
	return fmt.Errorf("Rollback not implemented in RED phase")
}
