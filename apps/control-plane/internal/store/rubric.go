// rubric.go — 등급 rubric(Rubric) 도메인 pgx 기반 트랜잭션 구현 (SPEC-AX-RUBRIC-001)
//
// score_review_request.go 패턴 정확 미러 — PgScoreReviewRequestTx 동형 구조.
// 상태 머신: draft → active → archived (terminal). archived → 모든 전이 거부 (UBI-004).
// 3계층 데이터 모델: rubrics(부모) + rubric_criteria(가중치) + rubric_bands(등급 구간).
//
// REVIEW-001 D1 iter2 lesson pre-applied [HARD]:
//   - 모든 mutation 메서드(InsertRubric/UpdateRubric/ArchiveRubric/AddCriterion/AddBand)는
//     명시적 userID string 파라미터 보유 — SQL $N에 created_by/updated_by/audit_logs.user_id로 일관 전파.
//   - pg_store.go BeginRubricTx에서 audit.NewRecorder(true) 주입 (false 금지).
//
// OPEN 결정 strategy.md §1 5요소 적용:
//   - #2: active 1-per-(name, scope) — 0006 partial unique idx + handler pre-check (Phase C)
//   - #3: weight sum=1.0 강제 — handler validation pre-store만 (PoC, store는 weight 0-1만)
//   - #4: band overlap — 0006 EXCLUSION USING gist + handler pre-check (Phase C)
//   - #5: active 직접 편집 허용 (admin) — UpdateRubric guard만, immutable 강제 0
//   - #6: ApplyRubric read-only no-audit — recorder 호출 0 [HARD]
//   - #7: archive_reason 필수 — handler validation + 0006 DB CHECK constraint
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// 상태 머신 상수 — 0006 마이그레이션 rubrics_status_chk와 정합
const (
	rubricStatusDraft    = "draft"
	rubricStatusActive   = "active"
	rubricStatusArchived = "archived"
)

// maxRubricNameLen rubric name 최대 길이 (VARCHAR(64) DDL 정합 — REQ-RUBRIC-001-U1)
const maxRubricNameLen = 64

// rubricFallbackUserID userID 빈 문자열이면 'cli-anonymous'로 대체 (UBI-003 fallback).
// score_review_request.go fallbackUserID 동형 — audit.DefaultUserID와 일치하나
// audit 패키지 비의존을 위해 내부 상수로 보유.
const rubricFallbackUserID = "cli-anonymous"

// allowedRubricTransitions state-machine 화이트리스트 (UBI-004 / REQ-RUBRIC-003-S2).
// draft→active (UpdateRubric), active→archived (ArchiveRubric) 만 허용.
// archived는 terminal — 어떤 전이도 거부.
// score_review_request.go allowedReviewTransitions 동형 패턴.
var allowedRubricTransitions = map[string]map[string]struct{}{
	rubricStatusDraft: {
		rubricStatusActive: {},
	},
	rubricStatusActive: {
		rubricStatusArchived: {},
	},
	rubricStatusArchived: {}, // terminal
}

// PgRubricTx pgx.Tx 래퍼 — RubricTx 인터페이스 구현 (score_review_request.go 동형).
// 단일 PostgreSQL 트랜잭션 내에서 모든 rubric 쓰기/조회 연산을 수행한다.
// recorder는 동일 t.tx에 audit_logs INSERT 전담 (mutation 5종 동일-TX 원자성, UBI-002).
// ApplyRubric은 read-only — recorder 미호출 (OPEN #6).
//
// @MX:WARN: [AUTO] mutation 내 entity-write 후 recorder.RecordRubric* 실패 시
//
//	호출자가 Commit하면 안 됨 — deferred Rollback이 entity+audit 양방향 취소
//
// @MX:REASON: REQ-RUBRIC-UBI-002 — mutation 5종이 동일 t.tx에서 원자 실행. 순서
//
//	(entity-write → audit-INSERT → 호출자 Commit) 변경 시 양방향 원자성 붕괴.
type PgRubricTx struct {
	// tx 래핑된 pgx 트랜잭션
	tx pgx.Tx
	// logger 구조화 로그
	logger *zap.Logger
	// recorder 등급 rubric 감사 이벤트 기록기 — 동일 tx에 audit_logs 1건 INSERT.
	// pg_store.go BeginRubricTx에서 audit.NewRecorder(true) 주입 [HARD] (REVIEW-001 D1 iter2 lesson).
	recorder *audit.Recorder
}

// validateRubricInput InsertRubric/UpdateRubric pre-write 검증 (REQ-RUBRIC-001-U1).
// blank name / >64자 거부 — SQL 미실행 후 ErrRubricInvalidInput 래핑 (fail-closed).
func validateRubricInput(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name이 비어 있음: %w", stderrors.ErrRubricInvalidInput)
	}
	if len(name) > maxRubricNameLen {
		return fmt.Errorf("name이 %d자를 초과함 (len=%d): %w",
			maxRubricNameLen, len(name), stderrors.ErrRubricInvalidInput)
	}
	return nil
}

// validateCriterionInput AddCriterion pre-write 검증 (REQ-RUBRIC-001-U1).
// weight 범위 0.0-1.0 강제 — SQL 미실행 후 ErrRubricWeightOutOfBounds 래핑.
func validateCriterionInput(weight float64) error {
	if weight < 0.0 || weight > 1.0 {
		return fmt.Errorf("weight=%.4f 범위 0.0-1.0 위반: %w",
			weight, stderrors.ErrRubricWeightOutOfBounds)
	}
	return nil
}

// validateBandInput AddBand pre-write 검증 (REQ-RUBRIC-001-U1).
// min < max 강제 — SQL 미실행 후 ErrRubricInvalidInput 래핑 (DB CHECK와 이중 방어).
func validateBandInput(minScore, maxScore float64) error {
	if minScore >= maxScore {
		return fmt.Errorf("min_score(%.4f) >= max_score(%.4f) 위반: %w",
			minScore, maxScore, stderrors.ErrRubricInvalidInput)
	}
	return nil
}

// validateRubricStatusTransition 현재 → 목표 상태 전이 허용 여부 (UBI-004 / REQ-RUBRIC-003-S2).
// 허용: draft→active / active→archived. 그 외 모두 거부.
// archived는 terminal — 어떤 전이도 허용되지 않는다 (allowedRubricTransitions에 비어 있음).
// score_review_request.go validateReviewStatusTransition 동형 패턴.
func validateRubricStatusTransition(current, next string) error {
	allowed, ok := allowedRubricTransitions[current]
	if !ok {
		return fmt.Errorf("status=%q 허용 외 현재 상태: %w",
			current, stderrors.ErrRubricInvalidStatus)
	}
	if _, ok := allowed[next]; !ok {
		return fmt.Errorf("status 전이 %s→%s 허용되지 않음: %w",
			current, next, stderrors.ErrRubricInvalidStatus)
	}
	return nil
}

// marshalRubricMetadata metadata map을 JSONB 바이트로 직렬화 (nil/빈 맵이면 NULL).
// score.go/score_review_request.go marshalReviewMetadata 동형.
func marshalRubricMetadata(metadata map[string]any) (interface{}, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("rubric metadata 직렬화 실패: %w", err)
	}
	return b, nil
}

// resolveRubricUserID userID 빈 문자열이면 'cli-anonymous' fallback (UBI-003).
// auth-disabled Walking Skeleton 시 핸들러가 ""를 전달 → 본 함수가 fallback.
func resolveRubricUserID(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return rubricFallbackUserID
	}
	return userID
}

// InsertRubric rubrics 테이블에 새 draft 행 삽입 + RUBRIC_CREATED audit 동일 TX (REQ-RUBRIC-001-E1).
// 검증 실패 시 SQL 미실행 후 ErrRubricInvalidInput 래핑 반환.
// userID: created_by/updated_by + audit_logs.user_id에 일관 영속 (UBI-003, D1 iter2 lesson).
// version은 기본 1로 시작 (clone-new-version sub-resource는 Phase C 핸들러에서 별도 호출).
//
// @MX:NOTE: [AUTO] 등급 rubric 생성 단일 진입점 — InsertRubric → recorder.RecordRubricCreated 동일 t.tx 원자성 계약
//
//	(iter2 demotion: fan_in=2 — handleCreateRubric + handleCloneNewVersion. ANCHOR fan_in≥3 기준 미달.
//	 ANCHOR는 UpdateRubric/ArchiveRubric/ApplyRubric 3개로 mx.yaml anchor_per_file=3 한도 준수.)
func (t *PgRubricTx) InsertRubric(
	ctx context.Context,
	name, scope string,
	metadata map[string]any,
	userID string,
) (uuid.UUID, error) {
	if vErr := validateRubricInput(name); vErr != nil {
		return uuid.Nil, vErr
	}

	metaJSON, mErr := marshalRubricMetadata(metadata)
	if mErr != nil {
		return uuid.Nil, mErr
	}

	var scopeArg interface{}
	if strings.TrimSpace(scope) != "" {
		scopeArg = scope
	}

	actor := resolveRubricUserID(userID)

	const query = `
		INSERT INTO rubrics (
			name, version, scope, status, metadata,
			created_at, created_by, updated_at, updated_by
		) VALUES (
			$1, 1, $2, 'draft', $3,
			now(), $4, now(), $4
		)
		RETURNING id
	`
	var id uuid.UUID
	if qErr := t.tx.QueryRow(ctx, query, name, scopeArg, metaJSON, actor).Scan(&id); qErr != nil {
		t.logger.Error("InsertRubric 실패",
			zap.String("name", name),
			zap.Error(qErr),
		)
		return uuid.Nil, fmt.Errorf("InsertRubric 실패: %w", qErr)
	}

	// REQ-RUBRIC-UBI-002: entity-INSERT 직후 동일 t.tx에 audit 1건 (actor 일관 영속).
	if auditErr := t.recorder.RecordRubricCreated(ctx, t, id, name, 1, actor); auditErr != nil {
		t.logger.Error("InsertRubric audit 기록 실패",
			zap.String("rubric_id", id.String()),
			zap.Error(auditErr),
		)
		return uuid.Nil, fmt.Errorf("InsertRubric audit 실패: %w: %w",
			stderrors.ErrRubricAuditWriteFailed, auditErr)
	}
	return id, nil
}

// GetRubricByID rubric PK로 단건 조회.
// 미존재 시 ErrRubricNotFound 래핑 반환 (raw pgx.ErrNoRows 누출 금지 — GAP-03 동형).
func (t *PgRubricTx) GetRubricByID(ctx context.Context, id uuid.UUID) (*Rubric, error) {
	const query = `
		SELECT id, name, version, scope, status, archive_reason,
		       metadata, created_at, created_by, updated_at, updated_by
		FROM rubrics
		WHERE id = $1
	`
	r, err := scanRubricRow(t.tx.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetRubricByID id=%s: %w", id, stderrors.ErrRubricNotFound)
		}
		return nil, fmt.Errorf("GetRubricByID scan 실패: %w", err)
	}
	return r, nil
}

// ListRubrics 필터(status/scope optional) + 페이지네이션 조회. created_at DESC 정렬.
// 빈 결과는 빈 슬라이스 반환 (error 아님).
func (t *PgRubricTx) ListRubrics(
	ctx context.Context,
	statusFilter, scopeFilter string,
	limit, offset int,
) ([]*Rubric, error) {
	// 동적 WHERE 구성 — SQL injection 회피 위해 $N placeholder만 사용
	conds := make([]string, 0, 2)
	args := make([]interface{}, 0, 4)
	argIdx := 1
	if strings.TrimSpace(statusFilter) != "" {
		conds = append(conds, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, statusFilter)
		argIdx++
	}
	if strings.TrimSpace(scopeFilter) != "" {
		conds = append(conds, fmt.Sprintf("scope = $%d", argIdx))
		args = append(args, scopeFilter)
		argIdx++
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, name, version, scope, status, archive_reason,
		       metadata, created_at, created_by, updated_at, updated_by
		FROM rubrics
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	rows, qErr := t.tx.Query(ctx, query, args...)
	if qErr != nil {
		return nil, fmt.Errorf("ListRubrics 실패: %w", qErr)
	}
	defer rows.Close()

	result := make([]*Rubric, 0)
	for rows.Next() {
		r, scanErr := scanRubricRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("ListRubrics scan 실패: %w", scanErr)
		}
		result = append(result, r)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("ListRubrics rows 에러: %w", rowsErr)
	}
	return result, nil
}

// CountRubrics 필터(status/scope optional)에 해당하는 전체 행 수 반환.
// pagination total 정확 계산용 (limit/offset 적용 전).
func (t *PgRubricTx) CountRubrics(ctx context.Context, statusFilter, scopeFilter string) (int64, error) {
	conds := make([]string, 0, 2)
	args := make([]interface{}, 0, 2)
	argIdx := 1
	if strings.TrimSpace(statusFilter) != "" {
		conds = append(conds, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, statusFilter)
		argIdx++
	}
	if strings.TrimSpace(scopeFilter) != "" {
		conds = append(conds, fmt.Sprintf("scope = $%d", argIdx))
		args = append(args, scopeFilter)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	query := fmt.Sprintf(`SELECT COUNT(*) FROM rubrics %s`, where)

	var n int64
	if err := t.tx.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("CountRubrics 실패: %w", err)
	}
	return n, nil
}

// lockAndCheckRubricStatus SELECT ... FOR UPDATE row lock + 현재 status 반환.
// score_review_request.go lockAndCheckStatus 동형 패턴.
// 미존재 시 ErrRubricNotFound 반환.
func (t *PgRubricTx) lockAndCheckRubricStatus(ctx context.Context, id uuid.UUID) (string, error) {
	const lockSQL = `SELECT status FROM rubrics WHERE id = $1 FOR UPDATE`
	var currentStatus string
	if err := t.tx.QueryRow(ctx, lockSQL, id).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("rubric id=%s: %w", id, stderrors.ErrRubricNotFound)
		}
		return "", fmt.Errorf("SELECT FOR UPDATE rubric 실패: %w", err)
	}
	return currentStatus, nil
}

// UpdateRubric rubric 메타/상태 부분 수정 + RUBRIC_UPDATED audit 동일 TX (REQ-RUBRIC-003-E1).
// OPEN #5: active 직접 편집 허용 (admin) — UpdateRubric가 status 변경 + 메타 변경 통합 담당.
// archived rubric mutation 거부 — ErrRubricArchived (terminal 불변, UBI-004).
// state 전이 시 validateRubricStatusTransition 의무.
// userID: updated_by + audit_logs.user_id (UBI-003, D1 iter2 lesson).
//
// @MX:ANCHOR: [AUTO] rubric 수정 단일 진입점 — REQ-RUBRIC-003-E1 AC + active 직접 편집 (OPEN #5)
// @MX:REASON: SELECT FOR UPDATE pessimistic lock + state guard + RUBRIC_UPDATED audit 동일 TX
func (t *PgRubricTx) UpdateRubric(
	ctx context.Context,
	id uuid.UUID,
	name, scope, newStatus string,
	metadata map[string]any,
	userID string,
) error {
	if vErr := validateRubricInput(name); vErr != nil {
		return vErr
	}

	currentStatus, lockErr := t.lockAndCheckRubricStatus(ctx, id)
	if lockErr != nil {
		return lockErr
	}
	// archived terminal — 어떤 mutation도 거부 (UBI-004 / REQ-RUBRIC-003-S1)
	if currentStatus == rubricStatusArchived {
		return fmt.Errorf("UpdateRubric id=%s: archived rubric: %w",
			id, stderrors.ErrRubricArchived)
	}

	// status 변경 요청 시 state-machine 가드 (UBI-004 / REQ-RUBRIC-003-S2)
	effectiveStatus := newStatus
	if strings.TrimSpace(newStatus) == "" || newStatus == currentStatus {
		// 메타만 변경 — status 유지 (OPEN #5 active 직접 편집 허용)
		effectiveStatus = currentStatus
	} else {
		if tErr := validateRubricStatusTransition(currentStatus, newStatus); tErr != nil {
			return tErr
		}
	}

	metaJSON, mErr := marshalRubricMetadata(metadata)
	if mErr != nil {
		return mErr
	}

	var scopeArg interface{}
	if strings.TrimSpace(scope) != "" {
		scopeArg = scope
	}

	actor := resolveRubricUserID(userID)

	const updateSQL = `
		UPDATE rubrics
		SET name = $2, scope = $3, status = $4, metadata = $5,
		    updated_at = now(), updated_by = $6
		WHERE id = $1
	`
	if _, execErr := t.tx.Exec(ctx, updateSQL, id, name, scopeArg, effectiveStatus, metaJSON, actor); execErr != nil {
		t.logger.Error("UpdateRubric UPDATE 실패",
			zap.String("id", id.String()),
			zap.Error(execErr),
		)
		return fmt.Errorf("UpdateRubric UPDATE 실패: %w", execErr)
	}

	// REQ-RUBRIC-UBI-002: 단일 PUT → 단일 audit row (R-RUBRIC-009)
	if auditErr := t.recorder.RecordRubricUpdated(ctx, t, id, actor); auditErr != nil {
		t.logger.Error("UpdateRubric audit 기록 실패",
			zap.String("rubric_id", id.String()),
			zap.Error(auditErr),
		)
		return fmt.Errorf("UpdateRubric audit 실패: %w: %w",
			stderrors.ErrRubricAuditWriteFailed, auditErr)
	}
	return nil
}

// ArchiveRubric active → archived terminal 전이 + RUBRIC_ARCHIVED audit + archive_reason 필수 (REQ-RUBRIC-003-E2).
// OPEN #7 dual defense Layer 1: handler-local validation pre-store + 0006 DB CHECK constraint.
// archive_reason blank/empty 시 SQL 미실행 후 ErrRubricInvalidInput 래핑.
// userID: updated_by + audit_logs.user_id (UBI-003).
//
// @MX:ANCHOR: [AUTO] rubric archive 단일 진입점 — REQ-RUBRIC-003-E2 AC + OPEN #7 dual defense
// @MX:REASON: active→archived terminal 전이 + archive_reason 추적 (한국 공공 감사 요구 강도)
func (t *PgRubricTx) ArchiveRubric(ctx context.Context, id uuid.UUID, archiveReason, userID string) error {
	// OPEN #7 Layer 1: handler/store-side pre-validation
	if strings.TrimSpace(archiveReason) == "" {
		return fmt.Errorf("archive_reason이 비어 있음: %w", stderrors.ErrRubricInvalidInput)
	}

	currentStatus, lockErr := t.lockAndCheckRubricStatus(ctx, id)
	if lockErr != nil {
		return lockErr
	}
	// active만 archived로 전이 가능 (UBI-004 / REQ-RUBRIC-003-S2)
	if currentStatus == rubricStatusArchived {
		return fmt.Errorf("ArchiveRubric id=%s: 이미 archived: %w",
			id, stderrors.ErrRubricInvalidStatus)
	}
	if tErr := validateRubricStatusTransition(currentStatus, rubricStatusArchived); tErr != nil {
		return tErr
	}

	actor := resolveRubricUserID(userID)

	const updateSQL = `
		UPDATE rubrics
		SET status = 'archived', archive_reason = $2,
		    updated_at = now(), updated_by = $3
		WHERE id = $1
	`
	if _, execErr := t.tx.Exec(ctx, updateSQL, id, archiveReason, actor); execErr != nil {
		return fmt.Errorf("ArchiveRubric UPDATE 실패: %w", execErr)
	}

	if auditErr := t.recorder.RecordRubricArchived(ctx, t, id, archiveReason, actor); auditErr != nil {
		return fmt.Errorf("ArchiveRubric audit 실패: %w: %w",
			stderrors.ErrRubricAuditWriteFailed, auditErr)
	}
	return nil
}

// AddCriterion rubric_criteria 행 추가 + RUBRIC_CRITERION_ADDED audit 동일 TX (REQ-RUBRIC-001-E2).
// archived rubric에 추가 시 ErrRubricArchived (terminal 불변).
// weight 0.0-1.0 검사 store 단계 (validateCriterionInput).
// weight sum > 1.0 검사는 handler 단계 (OPEN #3, PoC store는 단일 row CHECK만).
// userID: audit_logs.user_id (UBI-003).
//
// @MX:NOTE: [AUTO] criterion 추가 단일 진입점 — archived guard + weight 검증 + RUBRIC_CRITERION_ADDED audit 동일 TX
//
//	(iter2 demotion: fan_in=1 — handleAddCriterion만 호출. ANCHOR fan_in≥3 기준 미달.
//	 mx.yaml anchor_per_file=3 한도 준수 — UpdateRubric/ArchiveRubric/ApplyRubric 유지.)
func (t *PgRubricTx) AddCriterion(
	ctx context.Context,
	rubricID, evaluationItemID uuid.UUID,
	weight float64,
	userID string,
) (uuid.UUID, error) {
	if vErr := validateCriterionInput(weight); vErr != nil {
		return uuid.Nil, vErr
	}

	// archived guard — SELECT FOR UPDATE로 lock 후 status 확인
	currentStatus, lockErr := t.lockAndCheckRubricStatus(ctx, rubricID)
	if lockErr != nil {
		return uuid.Nil, lockErr
	}
	if currentStatus == rubricStatusArchived {
		return uuid.Nil, fmt.Errorf("AddCriterion rubric_id=%s: archived rubric: %w",
			rubricID, stderrors.ErrRubricArchived)
	}

	actor := resolveRubricUserID(userID)

	const insertSQL = `
		INSERT INTO rubric_criteria (rubric_id, evaluation_item_id, weight, created_at)
		VALUES ($1, $2, $3, now())
		RETURNING id
	`
	var criterionID uuid.UUID
	if qErr := t.tx.QueryRow(ctx, insertSQL, rubricID, evaluationItemID, weight).Scan(&criterionID); qErr != nil {
		t.logger.Error("AddCriterion INSERT 실패",
			zap.String("rubric_id", rubricID.String()),
			zap.Error(qErr),
		)
		return uuid.Nil, fmt.Errorf("AddCriterion INSERT 실패: %w", qErr)
	}

	if auditErr := t.recorder.RecordRubricCriterionAdded(ctx, t, criterionID, rubricID, evaluationItemID, actor); auditErr != nil {
		return uuid.Nil, fmt.Errorf("AddCriterion audit 실패: %w: %w",
			stderrors.ErrRubricAuditWriteFailed, auditErr)
	}
	return criterionID, nil
}

// AddBand rubric_bands 행 추가 + RUBRIC_BAND_ADDED audit 동일 TX (REQ-RUBRIC-001-E3).
// OPEN #4 dual defense Layer 1: store-side min < max 검증 + 0006 EXCLUSION USING gist (race window 봉쇄).
// archived rubric에 추가 시 ErrRubricArchived.
// userID: audit_logs.user_id (UBI-003).
//
// @MX:NOTE: [AUTO] band 추가 단일 진입점 — archived guard + min<max validation + OPEN #4 dual defense
//
//	(iter2 demotion: fan_in=1 — handleAddBand만 호출. ANCHOR fan_in≥3 기준 미달.
//	 mx.yaml anchor_per_file=3 한도 준수 — UpdateRubric/ArchiveRubric/ApplyRubric 유지.)
func (t *PgRubricTx) AddBand(
	ctx context.Context,
	rubricID uuid.UUID,
	letter string,
	minScore, maxScore float64,
	userID string,
) (uuid.UUID, error) {
	if strings.TrimSpace(letter) == "" {
		return uuid.Nil, fmt.Errorf("letter가 비어 있음: %w", stderrors.ErrRubricInvalidInput)
	}
	if vErr := validateBandInput(minScore, maxScore); vErr != nil {
		return uuid.Nil, vErr
	}

	currentStatus, lockErr := t.lockAndCheckRubricStatus(ctx, rubricID)
	if lockErr != nil {
		return uuid.Nil, lockErr
	}
	if currentStatus == rubricStatusArchived {
		return uuid.Nil, fmt.Errorf("AddBand rubric_id=%s: archived rubric: %w",
			rubricID, stderrors.ErrRubricArchived)
	}

	actor := resolveRubricUserID(userID)

	const insertSQL = `
		INSERT INTO rubric_bands (rubric_id, letter, min_score, max_score, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id
	`
	var bandID uuid.UUID
	if qErr := t.tx.QueryRow(ctx, insertSQL, rubricID, letter, minScore, maxScore).Scan(&bandID); qErr != nil {
		t.logger.Error("AddBand INSERT 실패",
			zap.String("rubric_id", rubricID.String()),
			zap.String("letter", letter),
			zap.Error(qErr),
		)
		return uuid.Nil, fmt.Errorf("AddBand INSERT 실패: %w", qErr)
	}

	if auditErr := t.recorder.RecordRubricBandAdded(ctx, t, bandID, rubricID, letter, actor); auditErr != nil {
		return uuid.Nil, fmt.Errorf("AddBand audit 실패: %w: %w",
			stderrors.ErrRubricAuditWriteFailed, auditErr)
	}
	return bandID, nil
}

// GetCriteriaByRubric 동일 rubric의 모든 criteria 반환 (weight sum 검증용, OPEN #3 핸들러 단계).
// created_at ASC 정렬 — 추가 순서 보존.
func (t *PgRubricTx) GetCriteriaByRubric(ctx context.Context, rubricID uuid.UUID) ([]*RubricCriterion, error) {
	const query = `
		SELECT id, rubric_id, evaluation_item_id, weight, created_at
		FROM rubric_criteria
		WHERE rubric_id = $1
		ORDER BY created_at ASC
	`
	rows, qErr := t.tx.Query(ctx, query, rubricID)
	if qErr != nil {
		return nil, fmt.Errorf("GetCriteriaByRubric 실패: %w", qErr)
	}
	defer rows.Close()

	result := make([]*RubricCriterion, 0)
	for rows.Next() {
		var c RubricCriterion
		if err := rows.Scan(&c.ID, &c.RubricID, &c.EvaluationItemID, &c.Weight, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetCriteriaByRubric scan 실패: %w", err)
		}
		result = append(result, &c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("GetCriteriaByRubric rows 에러: %w", rowsErr)
	}
	return result, nil
}

// GetBandsByRubric 동일 rubric의 모든 bands 반환 (overlap 검사 + apply linear scan용).
// min_score ASC 정렬 — 등급 구간 순서 보존.
func (t *PgRubricTx) GetBandsByRubric(ctx context.Context, rubricID uuid.UUID) ([]*RubricBand, error) {
	const query = `
		SELECT id, rubric_id, letter, min_score, max_score, created_at
		FROM rubric_bands
		WHERE rubric_id = $1
		ORDER BY min_score ASC
	`
	rows, qErr := t.tx.Query(ctx, query, rubricID)
	if qErr != nil {
		return nil, fmt.Errorf("GetBandsByRubric 실패: %w", qErr)
	}
	defer rows.Close()

	result := make([]*RubricBand, 0)
	for rows.Next() {
		var b RubricBand
		if err := rows.Scan(&b.ID, &b.RubricID, &b.Letter, &b.MinScore, &b.MaxScore, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetBandsByRubric scan 실패: %w", err)
		}
		result = append(result, &b)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("GetBandsByRubric rows 에러: %w", rowsErr)
	}
	return result, nil
}

// ApplyRubric scoreValue → 등급 결정 (read-only no-audit, OPEN #6) (REQ-RUBRIC-004-E1).
// 알고리즘:
//  1. GetRubricByID로 rubric 존재 검증 (archived도 read+apply는 허용, UBI-004)
//  2. GetBandsByRubric로 bands 로드 (min_score ASC 정렬)
//  3. Linear scan: min_score <= scoreValue <= max_score (경계 inclusive '[]', R-RUBRIC-007)
//  4. 매치 band 발견 시 (letter, *band, nil) 반환
//  5. 모든 band 밖 → ErrRubricInvalidInput fail-closed (REQ-RUBRIC-004-U1)
//
// [HARD] recorder 호출 0 — audit_logs 0건 (UBI-002 second clause carve-out, OPEN #6).
// REPORT-001 read-only no-audit 선례 정확 미러.
//
// @MX:ANCHOR: [AUTO] apply 엔진 단일 진입점 — REQ-RUBRIC-004-E1 AC + OPEN #6 read-only
// @MX:REASON: cross-store handler-compose 2-TX의 TX-2 책임 — read-only no-audit 계약
func (t *PgRubricTx) ApplyRubric(ctx context.Context, rubricID uuid.UUID, scoreValue float64) (string, *RubricBand, error) {
	// rubric 존재 검증 — 미존재 시 ErrRubricNotFound 전파 (cross-store 404 매핑 대상)
	if _, err := t.GetRubricByID(ctx, rubricID); err != nil {
		return "", nil, err
	}

	bands, bErr := t.GetBandsByRubric(ctx, rubricID)
	if bErr != nil {
		return "", nil, bErr
	}

	// linear scan, 경계 inclusive (numrange '[]')
	for _, b := range bands {
		if scoreValue >= b.MinScore && scoreValue <= b.MaxScore {
			return b.Letter, b, nil
		}
	}

	// fail-closed — 모든 band 밖이면 default 등급 fabricate 금지 (REQ-RUBRIC-004-U1)
	return "", nil, fmt.Errorf("ApplyRubric scoreValue=%.4f: %w",
		scoreValue, stderrors.ErrRubricInvalidInput)
}

// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입 (Recorder가 호출).
// D2: resource_id = entity UUID 직접 대입 — surrogate 미사용 (rubrics/rubric_criteria/rubric_bands 모두 동형).
// score_review_request.go InsertAuditLog 정확 미러.
func (t *PgRubricTx) InsertAuditLog(ctx context.Context, e *audit.Event) error {
	const query = `
		INSERT INTO audit_logs (id, action, resource_type, resource_id, user_id, details, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	id := uuid.New()
	var details interface{}
	if len(e.DetailsJSON) > 0 {
		details = e.DetailsJSON
	}
	_, err := t.tx.Exec(ctx, query,
		id, string(e.Action), e.ResourceType, e.ResourceID, e.UserID, details, e.Timestamp,
	)
	if err != nil {
		t.logger.Error("InsertAuditLog(rubric) 실패",
			zap.String("action", string(e.Action)),
			zap.String("resource_id", e.ResourceID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("InsertAuditLog(rubric) 실패: %w", err)
	}
	return nil
}

// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화.
func (t *PgRubricTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("Commit(rubric) 실패: %w", err)
	}
	return nil
}

// Rollback 현재 트랜잭션을 롤백 (Commit 후 호출 시 pgx가 무시).
func (t *PgRubricTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return nil
		}
		return fmt.Errorf("Rollback(rubric) 실패: %w", err)
	}
	return nil
}

// scanRubricRow pgx.Row/pgx.Rows 공통 스캔 헬퍼 (DAMP — 컬럼 순서 단일 정의).
// archive_reason은 nullable이므로 *string 임시 변수 후 Rubric struct로 복사.
func scanRubricRow(row pgx.Row) (*Rubric, error) {
	var (
		r             Rubric
		scope         *string
		archiveReason *string
		updatedBy     *string
		metaRaw       []byte
	)
	if err := row.Scan(
		&r.ID, &r.Name, &r.Version, &scope, &r.Status, &archiveReason,
		&metaRaw, &r.CreatedAt, &r.CreatedBy, &r.UpdatedAt, &updatedBy,
	); err != nil {
		return nil, err
	}
	if scope != nil {
		r.Scope = *scope
	}
	if archiveReason != nil {
		r.ArchiveReason = *archiveReason
	}
	if updatedBy != nil {
		r.UpdatedBy = *updatedBy
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &r.Metadata) //nolint:errcheck // 손상된 메타데이터는 nil로 graceful
	}
	return &r, nil
}
