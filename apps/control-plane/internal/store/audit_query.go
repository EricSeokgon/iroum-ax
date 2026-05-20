// audit_query.go — audit_logs read-only SELECT 구현 (SPEC-AX-AUDIT-QUERY-001)
//
// 본 SPEC은 read-only이므로 audit_logs INSERT 0건 — pg_store.go:346-381 InsertAuditLog
// 시그니처/본문 0-diff (consumer-only §1.4 [HARD]). 본 파일은 SELECT 메서드만 추가하며
// audit_logs 스키마 / 인덱스 / Action 상수 / Recorder를 일절 수정하지 않는다.
//
// FakeTx 구현은 fake_store.go에 위임 — 인메모리 store.AuditLogs 슬라이스를 필터링/페이지네이션.
// PgWorkflowTx 구현은 동적 SQL 빌더 + 파라미터 바인딩($N) + COUNT(*) OVER() window function.
//
// SQL injection 방어: 5-필터 모두 placeholder ($1, $2, ...) — string interpolation 0건.
// ORDER BY는 audit_logs_user_id_timestamp_idx(user_id, timestamp DESC) partial 활용을 위해
// timestamp DESC 단독 — user_id 필터 시 인덱스 prefix 부분 활용 가능 (EXPLAIN 검증은 M3).
package store

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// QueryAuditLogs audit_logs 테이블을 5-필터 AND + offset/limit으로 검색하고 결과 + total을 반환
// (SPEC-AX-AUDIT-QUERY-001 REQ-AUDIT-QUERY-001-E1, OPEN #5/#6 RESOLVED).
//
// 동적 SQL: WHERE 1=1 [AND action=$N] [AND resource_type=$N] [AND resource_id=$N]
//
//	[AND user_id=$N] [AND timestamp >= $N] [AND timestamp <= $N]
//	ORDER BY timestamp DESC LIMIT $N OFFSET $N
//
// COUNT(*) OVER() window function — 1 round-trip으로 events + total 동시 산출.
// 결과 0건 → empty slice + total=0 (404/500 미반환, REQ-AUDIT-QUERY-001-E2).
//
// @MX:ANCHOR: [AUTO] 감사 로그 검색 단일 진입점 — 핸들러 / 통합 테스트 / 후속 consumer 3곳 이상에서 호출
// @MX:REASON: audit_logs SELECT 단일 계약 (read-only, consumer-only §1.4) — 신규 mutation 0
func (t *PgWorkflowTx) QueryAuditLogs(
	ctx context.Context, filter AuditLogFilter, limit, offset int,
) ([]*audit.Event, int64, error) {
	whereClauses := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if filter.Action != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("action = $%d", idx))
		args = append(args, *filter.Action)
		idx++
	}
	if filter.ResourceType != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("resource_type = $%d", idx))
		args = append(args, *filter.ResourceType)
		idx++
	}
	if filter.ResourceID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("resource_id = $%d", idx))
		args = append(args, *filter.ResourceID)
		idx++
	}
	if filter.UserID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *filter.UserID)
		idx++
	}
	if filter.Since != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp >= $%d", idx))
		args = append(args, *filter.Since)
		idx++
	}
	if filter.Until != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp <= $%d", idx))
		args = append(args, *filter.Until)
		idx++
	}

	// LIMIT / OFFSET placeholder
	limitIdx := idx
	args = append(args, limit)
	idx++
	offsetIdx := idx
	args = append(args, offset)

	query := fmt.Sprintf(
		`SELECT id, action, resource_type, resource_id, user_id, timestamp, details,
		        COUNT(*) OVER() AS total
		   FROM audit_logs
		  WHERE %s
		  ORDER BY timestamp DESC
		  LIMIT $%d OFFSET $%d`,
		strings.Join(whereClauses, " AND "),
		limitIdx, offsetIdx,
	)

	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		t.logger.Error("QueryAuditLogs Query 실패", zap.Error(err))
		return nil, 0, fmt.Errorf("QueryAuditLogs 실패: %w", err)
	}
	defer rows.Close()

	events := []*audit.Event{}
	var total int64
	for rows.Next() {
		var (
			id           string // unused (audit_logs.id는 audit.Event struct에 없음)
			e            audit.Event
			resourceType *string
			details      []byte
		)
		if err := rows.Scan(
			&id, &e.Action, &resourceType, &e.ResourceID, &e.UserID, &e.Timestamp, &details, &total,
		); err != nil {
			t.logger.Error("QueryAuditLogs Scan 실패", zap.Error(err))
			return nil, 0, fmt.Errorf("QueryAuditLogs Scan 실패: %w", err)
		}
		if resourceType != nil {
			e.ResourceType = *resourceType
		}
		if len(details) > 0 {
			e.DetailsJSON = details
		}
		events = append(events, &e)
	}
	if err := rows.Err(); err != nil {
		t.logger.Error("QueryAuditLogs rows.Err", zap.Error(err))
		return nil, 0, fmt.Errorf("QueryAuditLogs rows iteration 실패: %w", err)
	}

	return events, total, nil
}
