// eval_item.go — 평가항목(EvalItem) taxonomy pgx 기반 EvalItemTx 구현
// SPEC-AX-EVAL-ITEM-001: PgEvidenceTx 패턴을 미러링한 평가항목 트랜잭션 구현
// 자기참조 adjacency list (Option A): 단일 evaluation_items 테이블 + parent_id 자기 FK
// 항목 create/update와 audit_logs 1건은 동일 pgx TX에 atomic하게 처리된다 (REQ-EVALITEM-UBI-002)
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// maxEvalItemIDLen 평가항목 id 최대 길이 (DDL VARCHAR(64) 정합 — REQ-EVALITEM-001-U1)
const maxEvalItemIDLen = 64

// PgEvalItemTx pgx.Tx 래퍼 — EvalItemTx 인터페이스 구현
// 단일 PostgreSQL 트랜잭션 내에서 모든 평가항목 쓰기/조회 연산을 수행
// PgEvidenceTx와 동일한 트랜잭션 원자성 패턴을 미러링한다.
//
// @MX:WARN: [AUTO] InsertEvalItem/InsertAuditLog 사이 panic/early-return 시 orphan 항목 행 누출
// @MX:REASON: BeginEvalItemTx 직후 defer Rollback 등록 필수 — Commit 전 모든 경로가 단일 TX 내 (research.md §8)
type PgEvalItemTx struct {
	// tx 래핑된 pgx 트랜잭션
	tx pgx.Tx
	// logger 구조화 로그
	logger *zap.Logger
}

// validateEvalItemInput id/displayName/hierarchyCode blank·id 길이 초과를 SQL 미실행 후 거부
// REQ-EVALITEM-001-U1 + GAP-03 (hierarchyCode 비-blank — AUD-1 surrogate 일관성)
func validateEvalItemInput(id, displayName, hierarchyCode string) error {
	if id == "" {
		return fmt.Errorf("id가 비어 있음: %w", stderrors.ErrEvalItemInvalidInput)
	}
	if len(id) > maxEvalItemIDLen {
		return fmt.Errorf("id가 %d자를 초과함 (len=%d): %w",
			maxEvalItemIDLen, len(id), stderrors.ErrEvalItemInvalidInput)
	}
	if displayName == "" {
		return fmt.Errorf("display_name이 비어 있음: %w", stderrors.ErrEvalItemInvalidInput)
	}
	if hierarchyCode == "" {
		// GAP-03: hierarchy_code가 NULL/빈 문자열이면 AUD-1 surrogate가 비결정적/충돌 가능
		return fmt.Errorf("hierarchy_code가 비어 있음: %w", stderrors.ErrEvalItemInvalidInput)
	}
	return nil
}

// InsertEvalItem evaluation_items 테이블에 새 행을 삽입하고 삽입된 id를 반환
// parentID가 nil이면 루트(parent_id NULL), non-nil이면 부모 존재를 사전 조회로 검증한다.
// SQL은 $N placeholder만 사용 (SEC-01 SQL injection 방지 — 문자열 보간 금지)
//
// @MX:NOTE: [AUTO] parent 사전 조회 → orphan 방지 (REQ-EVALITEM-001-S1). 검증 실패 시 SQL 미실행
func (t *PgEvalItemTx) InsertEvalItem(
	ctx context.Context,
	id string,
	parentID *string,
	displayName, description string,
	level *int,
	hierarchyCode string,
	weight *float64,
	maxScore *int,
	metadata map[string]any,
) (string, error) {
	if err := validateEvalItemInput(id, displayName, hierarchyCode); err != nil {
		return "", err
	}

	// parent 사전 조회 — non-nil parentID가 존재하지 않으면 SQL INSERT 미실행 후 거부 (orphan 방지)
	if parentID != nil {
		if _, err := t.GetEvalItemByID(ctx, *parentID); err != nil {
			if errors.Is(err, stderrors.ErrEvalItemNotFound) {
				return "", fmt.Errorf("parent_id=%s 미존재: %w",
					*parentID, stderrors.ErrEvalItemParentNotFound)
			}
			return "", fmt.Errorf("InsertEvalItem parent 조회 실패: %w", err)
		}
	}

	const query = `
		INSERT INTO evaluation_items (
			id, parent_id, display_name, description, level,
			hierarchy_code, weight, max_score, status, metadata,
			created_at, created_by, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, 'ACTIVE', $9,
			now(), 'cli-anonymous', now()
		)
		RETURNING id
	`

	metaJSON, err := marshalEvalItemMetadata(metadata)
	if err != nil {
		return "", err
	}

	var returned string
	err = t.tx.QueryRow(ctx, query,
		id, parentID, displayName, nullIfEmpty(description), level,
		hierarchyCode, weight, maxScore, metaJSON,
	).Scan(&returned)
	if err != nil {
		if pgErr, ok := pgEvalItemErrorOf(err); ok {
			t.logger.Error("InsertEvalItem 실패",
				zap.String("id", id),
				zap.String("sqlstate", pgErr.Code),
				zap.Error(err),
			)
			return "", fmt.Errorf("InsertEvalItem 실패 (SQLSTATE %s): %w", pgErr.Code, err)
		}
		t.logger.Error("InsertEvalItem 실패", zap.String("id", id), zap.Error(err))
		return "", fmt.Errorf("InsertEvalItem 실패: %w", err)
	}
	return returned, nil
}

// GetEvalItemByID 평가항목 id로 단건을 조회
// 존재하지 않으면 stderrors.ErrEvalItemNotFound를 래핑하여 반환 (raw pgx.ErrNoRows 금지)
func (t *PgEvalItemTx) GetEvalItemByID(ctx context.Context, id string) (*EvalItem, error) {
	const query = `
		SELECT id, parent_id, display_name, description, level,
		       hierarchy_code, weight, max_score, status, metadata,
		       created_at, created_by, updated_at
		FROM evaluation_items
		WHERE id = $1
	`
	item, err := scanEvalItemRow(t.tx.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetEvalItemByID id=%s: %w", id, stderrors.ErrEvalItemNotFound)
		}
		return nil, fmt.Errorf("GetEvalItemByID scan 실패: %w", err)
	}
	return item, nil
}

// GetEvalItemsByParentID 동일 parent_id를 가진 직계 자식 목록을 반환
// 자식이 없으면 빈 슬라이스 반환 (error 아님 — GAP-02/DC-005.6).
// evaluation_items_parent_id_idx 인덱스를 사용한 단일 레벨 조회 (재귀 subtree 범위 밖).
//
// @MX:WARN: [AUTO] 호출자가 이 메서드를 루프로 트리 순회 시, 순환 parent 참조(자기/조상을 parent 지정)는
//
//	애플리케이션 무한 루프를 유발한다. WITH RECURSIVE 미사용 — 단일 레벨만 보장.
//
// @MX:REASON: 순환 탐지 자동화는 본 SPEC 범위 밖 (REQ-EVALITEM-002-U1 PoC 수동 규율).
//
//	evaluation_items_parent_id_idx 의존 — 인덱스 제거 시 깊은 계층에서 Seq Scan 성능 붕괴 (research.md §8 R-EVALITEM-001/004)
func (t *PgEvalItemTx) GetEvalItemsByParentID(ctx context.Context, parentID string) ([]*EvalItem, error) {
	const query = `
		SELECT id, parent_id, display_name, description, level,
		       hierarchy_code, weight, max_score, status, metadata,
		       created_at, created_by, updated_at
		FROM evaluation_items
		WHERE parent_id = $1
		ORDER BY id
	`
	rows, err := t.tx.Query(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("GetEvalItemsByParentID 실패: %w", err)
	}
	defer rows.Close()

	result := make([]*EvalItem, 0)
	for rows.Next() {
		item, scanErr := scanEvalItemRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("GetEvalItemsByParentID scan 실패: %w", scanErr)
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("GetEvalItemsByParentID rows 에러: %w", err)
	}
	// 자식 없음 → 빈 슬라이스 (GAP-02/DC-005.6 — pgx.ErrNoRows 아님)
	return result, nil
}

// allowedEvalItemStatus status 전이 화이트리스트 (REQ-EVALITEM-004-U1)
// store 사전 검증 + DB CHECK 이중 방어. 열거 외/NULL은 SQL 미실행 후 거부.
var allowedEvalItemStatus = map[string]struct{}{
	"ACTIVE":     {},
	"DEPRECATED": {},
	"ARCHIVED":   {},
}

// UpdateEvalItem 평가항목을 부분 갱신한다 (GAP-05 — option (a) nullable 필드).
// 핵심 불변식: 자식(successor) 보유 항목의 parent_id/level 변경 요청은 SQL 미실행 후 거부
// (REQ-EVALITEM-UBI-004). 잎 노드는 parent_id/level 변경 가능 (REQ-EVALITEM-UBI-004 명시 허용 — GAP-01).
// SET 절은 하드코딩 컬럼명만 사용 (SEC-02 — 동적 컬럼 주입 금지).
//
// @MX:WARN: [AUTO] mutation guard — successor 존재 확인 누락 시 자식 보유 항목의 계층 위치 변경으로
//
//	트리 무결성 붕괴. 검증 순서(successor 확인 → 거부 → SQL) 변경 금지.
//
// @MX:REASON: 계층 불변식(REQ-EVALITEM-UBI-004) 강제 지점 — 잎/비-잎 구분이 GAP-01 핵심
func (t *PgEvalItemTx) UpdateEvalItem(ctx context.Context, id string, upd EvalItemUpdate) error {
	// 대상 존재 확인 (없으면 ErrEvalItemNotFound)
	if _, err := t.GetEvalItemByID(ctx, id); err != nil {
		return err
	}

	// status 사전 검증 (열거 외/NULL 거부 — REQ-EVALITEM-004-U1)
	if err := validateStatusTransition(upd); err != nil {
		return err
	}

	// 계층 불변식 가드 — successor 확인 → 거부 → SQL 순서 (REQ-EVALITEM-UBI-004, GAP-01)
	if err := t.checkHierarchyMutationGuard(ctx, id, upd); err != nil {
		return err
	}

	// SET 절 동적 구성 (컬럼명 하드코딩, 값만 $N 바인딩 — SEC-02)
	setClauses, args, err := buildEvalItemUpdateSet(upd)
	if err != nil {
		return err
	}
	if len(setClauses) == 0 {
		// 변경 요청 없음 — no-op (audit는 호출자 TX orchestration이 결정)
		return nil
	}
	setClauses = append(setClauses, "updated_at = now()")

	// SEC-02: 컬럼명은 buildEvalItemUpdateSet의 하드코딩 분기에서만 유래, WHERE 값은 $N
	argN := len(args) + 1
	query := "UPDATE evaluation_items SET " + joinComma(setClauses) +
		fmt.Sprintf(" WHERE id = $%d", argN)
	args = append(args, id)

	result, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		if pgErr, ok := pgEvalItemErrorOf(err); ok {
			return fmt.Errorf("UpdateEvalItem 실패 (SQLSTATE %s): %w", pgErr.Code, err)
		}
		return fmt.Errorf("UpdateEvalItem 실패: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("UpdateEvalItem id=%s: %w", id, stderrors.ErrEvalItemNotFound)
	}
	return nil
}

// validateStatusTransition status가 지정된 경우 화이트리스트 외/NULL 값을 SQL 미실행 후 거부
// (REQ-EVALITEM-004-U1 — store 사전 검증, DB CHECK과 이중 방어)
func validateStatusTransition(upd EvalItemUpdate) error {
	if upd.Status == nil {
		return nil
	}
	if _, ok := allowedEvalItemStatus[*upd.Status]; !ok {
		return fmt.Errorf("status=%q 허용 외 값: %w",
			*upd.Status, stderrors.ErrEvalItemInvalidStatus)
	}
	return nil
}

// checkHierarchyMutationGuard parent_id/level 변경 요청 시에만 successor(자식) 보유 여부를
// 확인하고, 자식이 있으면 SQL 미실행 후 도메인 거부한다. 잎 노드는 변경 허용 (GAP-01:
// mutation guard가 모든 parent_id/level 변경을 거부하도록 과일반화하면 안 됨).
//
// [HARD] 검증 순서(successor 확인 → 거부 → SQL)는 PgEvalItemTx 타입 @MX:WARN 계약상 변경 금지.
func (t *PgEvalItemTx) checkHierarchyMutationGuard(ctx context.Context, id string, upd EvalItemUpdate) error {
	if upd.ParentID == nil && upd.Level == nil {
		return nil
	}
	var childCount int
	if err := t.tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM evaluation_items WHERE parent_id = $1`, id,
	).Scan(&childCount); err != nil {
		return fmt.Errorf("UpdateEvalItem successor 확인 실패: %w", err)
	}
	if childCount > 0 {
		// 자식 보유 — parent_id/level 변경은 SQL 미실행 후 도메인 거부
		return fmt.Errorf(
			"UpdateEvalItem id=%s: 자식(children) %d개 보유 — 계층 속성 변경 거부: %w",
			id, childCount, stderrors.ErrEvalItemHierarchyImmutable)
	}
	return nil
}

// buildEvalItemUpdateSet upd의 non-nil 필드에 대해 SET 절 fragment와 $N 바인딩 인자를 생성한다.
// [HARD] SEC-02: 컬럼명은 이 함수 내 하드코딩 리터럴만 사용 (사용자 입력 유래 동적 컬럼 주입 금지),
// 값만 $N 파라미터로 바인딩한다. 반환된 setClauses는 마지막에 updated_at 추가 전 상태이며,
// 호출자가 WHERE 절 argN을 len(args)+1로 산출한다.
func buildEvalItemUpdateSet(upd EvalItemUpdate) (setClauses []string, args []any, err error) {
	setClauses = make([]string, 0, 8)
	args = make([]any, 0, 9)
	add := func(clause string, val any) {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", clause, len(args)+1))
		args = append(args, val)
	}

	if upd.ParentID != nil {
		add("parent_id", *upd.ParentID) // **string deref → *string (NULL 가능)
	}
	if upd.Level != nil {
		add("level", *upd.Level)
	}
	if upd.DisplayName != nil {
		add("display_name", *upd.DisplayName)
	}
	if upd.Description != nil {
		add("description", *upd.Description)
	}
	if upd.Weight != nil {
		add("weight", *upd.Weight)
	}
	if upd.MaxScore != nil {
		add("max_score", *upd.MaxScore)
	}
	if upd.Status != nil {
		add("status", *upd.Status)
	}
	if upd.Metadata != nil {
		metaJSON, mErr := marshalEvalItemMetadata(*upd.Metadata)
		if mErr != nil {
			return nil, nil, mErr
		}
		add("metadata", metaJSON)
	}
	return setClauses, args, nil
}

// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입
// PgEvidenceTx.InsertAuditLog와 동일 패턴 — Recorder가 동일 TX 원자성을 위해 호출
func (t *PgEvalItemTx) InsertAuditLog(ctx context.Context, e *audit.Event) error {
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
		id,
		string(e.Action),
		e.ResourceType,
		e.ResourceID,
		e.UserID,
		details,
		e.Timestamp,
	)
	if err != nil {
		t.logger.Error("InsertAuditLog(eval_item) 실패",
			zap.String("action", string(e.Action)),
			zap.String("resource_id", e.ResourceID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("InsertAuditLog(eval_item) 실패: %w", err)
	}
	return nil
}

// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
func (t *PgEvalItemTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("Commit(eval_item) 실패: %w", err)
	}
	return nil
}

// Rollback 현재 트랜잭션을 롤백 — Commit 후 호출 시 pgx가 무시(no-op)
func (t *PgEvalItemTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return nil
		}
		return fmt.Errorf("Rollback(eval_item) 실패: %w", err)
	}
	return nil
}

// marshalEvalItemMetadata metadata map을 JSONB 바이트로 직렬화 (nil/빈 맵이면 NULL)
// opaque placeholder — 필드명 검증/거부 없음 (REQ-EVALITEM-001-O1)
func marshalEvalItemMetadata(metadata map[string]any) (interface{}, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata 직렬화 실패: %w", err)
	}
	return b, nil
}

// scanEvalItemRow pgx.Row/pgx.Rows 공통 스캔 헬퍼 (DAMP — 컬럼 순서 단일 정의)
func scanEvalItemRow(row pgx.Row) (*EvalItem, error) {
	var (
		item    EvalItem
		parent  *string
		desc    *string
		level   *int
		weight  *float64
		maxS    *int
		metaRaw []byte
	)
	if err := row.Scan(
		&item.ID, &parent, &item.DisplayName, &desc, &level,
		&item.HierarchyCode, &weight, &maxS, &item.Status, &metaRaw,
		&item.CreatedAt, &item.CreatedBy, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.ParentID = parent
	item.Level = level
	item.Weight = weight
	item.MaxScore = maxS
	if desc != nil {
		item.Description = *desc
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &item.Metadata) //nolint:errcheck // 손상된 메타데이터는 nil로 graceful
	}
	return &item, nil
}

// pgEvalItemErrorOf err 체인에서 *pgconn.PgError를 추출 (errors.As 래핑 호환)
func pgEvalItemErrorOf(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr, true
	}
	return nil, false
}

// nullIfEmpty 빈 문자열을 SQL NULL(nil)로, 아니면 포인터로 변환 (description nullable)
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// joinComma []string을 ", "로 결합 (strings.Join 회피 — import 최소화 불요하나 명시적)
func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
