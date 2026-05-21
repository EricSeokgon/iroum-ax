// audit_query_test.go — QueryAuditLogs 단위 테스트 (SPEC-AX-AUDIT-QUERY-001)
//
// store-layer 테스트는 인터페이스 contract만 검증한다 (FakeTx 인메모리 구현).
// 실 PostgreSQL behavior는 후속 //go:build integration 테스트에서 testcontainers로 검증.
// 본 SPEC은 read-only이므로 testcontainers fan-out 비용 회피, 핸들러 단위 우선.
package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
)

// seedAuditLogs FakeStore에 audit 이벤트들을 직접 삽입한다 (BeginTx 없이 SeedAuditLog 헬퍼).
// 본 PoC에서는 FakeTx.QueryAuditLogs가 store.AuditLogs를 필터링/페이지네이션하는 단순 구현 검증.
func seedAuditLogs(t *testing.T, st *FakeStore, events []*audit.Event) {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, e := range events {
		evCopy := *e
		st.AuditLogs = append(st.AuditLogs, &evCopy)
	}
}

// strPtr String literal → *string helper
func strPtr(s string) *string { return &s }

// timePtr Time → *time.Time helper
func timePtr(t time.Time) *time.Time { return &t }

// uuidPtr UUID → *uuid.UUID helper
func uuidPtr(u uuid.UUID) *uuid.UUID { return &u }

// ════════════════════════════════════════════════════════════════════════════
// T-001 [S1] empty filter → 모든 audit_logs row 반환
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_EmptyFilter_ReturnsAll(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()

	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)
	t3 := time.Now()

	seedAuditLogs(t, st, []*audit.Event{
		{Timestamp: t1, Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: uuid.New(), ResourceType: "score"},
		{Timestamp: t2, Action: audit.ActionScoreUpdated, UserID: "bob", ResourceID: uuid.New(), ResourceType: "score"},
		{Timestamp: t3, Action: audit.ActionRubricCreated, UserID: "alice", ResourceID: uuid.New(), ResourceType: "rubric"},
	})

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	events, total, err := tx.QueryAuditLogs(ctx, AuditLogFilter{}, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, events, 3)
}

// ════════════════════════════════════════════════════════════════════════════
// T-002 [S1] filter by Action — 매칭 row만
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_ActionFilter_OnlyMatches(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()
	now := time.Now()
	seedAuditLogs(t, st, []*audit.Event{
		{Timestamp: now, Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: uuid.New()},
		{Timestamp: now, Action: audit.ActionScoreUpdated, UserID: "bob", ResourceID: uuid.New()},
		{Timestamp: now, Action: audit.ActionScoreCreated, UserID: "carol", ResourceID: uuid.New()},
	})

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	filter := AuditLogFilter{Action: strPtr(string(audit.ActionScoreCreated))}
	events, total, err := tx.QueryAuditLogs(ctx, filter, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "2개의 SCORE_CREATED 매치")
	assert.Len(t, events, 2)
	for _, e := range events {
		assert.Equal(t, audit.ActionScoreCreated, e.Action)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// T-003 [S1] 5-필터 AND 조합 (action + user_id + resource_type + resource_id + time)
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_FiveFilterAND_OnlyMatches(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()

	targetResource := uuid.New()
	now := time.Now()

	seedAuditLogs(t, st, []*audit.Event{
		// 매치 — 5필터 모두 일치
		{Timestamp: now, Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: targetResource, ResourceType: "score"},
		// action 불일치
		{Timestamp: now, Action: audit.ActionScoreUpdated, UserID: "alice", ResourceID: targetResource, ResourceType: "score"},
		// user_id 불일치
		{Timestamp: now, Action: audit.ActionScoreCreated, UserID: "bob", ResourceID: targetResource, ResourceType: "score"},
		// resource_id 불일치
		{Timestamp: now, Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: uuid.New(), ResourceType: "score"},
		// resource_type 불일치
		{Timestamp: now, Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: targetResource, ResourceType: "rubric"},
	})

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	filter := AuditLogFilter{
		Action:       strPtr(string(audit.ActionScoreCreated)),
		ResourceType: strPtr("score"),
		ResourceID:   uuidPtr(targetResource),
		UserID:       strPtr("alice"),
		Since:        timePtr(now.Add(-1 * time.Hour)),
		Until:        timePtr(now.Add(1 * time.Hour)),
	}
	events, total, err := tx.QueryAuditLogs(ctx, filter, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "5-필터 AND 정확 1개 매치")
	require.Len(t, events, 1)
	assert.Equal(t, audit.ActionScoreCreated, events[0].Action)
	assert.Equal(t, "alice", events[0].UserID)
	assert.Equal(t, targetResource, events[0].ResourceID)
}

// ════════════════════════════════════════════════════════════════════════════
// T-004 [S1] 페이지네이션 limit/offset 슬라이싱
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_Pagination_LimitOffset(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()

	now := time.Now()
	events := make([]*audit.Event, 0, 20)
	for i := 0; i < 20; i++ {
		events = append(events, &audit.Event{
			Timestamp:  now.Add(time.Duration(-i) * time.Minute),
			Action:     audit.ActionScoreCreated,
			UserID:     "alice",
			ResourceID: uuid.New(),
		})
	}
	seedAuditLogs(t, st, events)

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	result, total, err := tx.QueryAuditLogs(ctx, AuditLogFilter{}, 10, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(20), total, "total = 전체 5-필터 매치 (페이지네이션 전)")
	assert.Len(t, result, 10, "page size limit=10")
}

// ════════════════════════════════════════════════════════════════════════════
// T-005 [S1] total count 정확성
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_TotalCount_CorrectAfterFilter(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()

	now := time.Now()
	all := make([]*audit.Event, 0, 30)
	for i := 0; i < 30; i++ {
		action := audit.ActionScoreCreated
		if i%3 == 0 {
			action = audit.ActionRubricCreated
		}
		all = append(all, &audit.Event{
			Timestamp:  now.Add(time.Duration(-i) * time.Minute),
			Action:     action,
			UserID:     "alice",
			ResourceID: uuid.New(),
		})
	}
	seedAuditLogs(t, st, all)

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	// filter: action=SCORE_CREATED (20개), limit=5
	filter := AuditLogFilter{Action: strPtr(string(audit.ActionScoreCreated))}
	result, total, err := tx.QueryAuditLogs(ctx, filter, 5, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(20), total, "total = 필터 매치 전체 (페이지네이션 전)")
	assert.Len(t, result, 5)
}

// ════════════════════════════════════════════════════════════════════════════
// T-006 [S1] 결과 0건 → empty slice + total=0 (404/500 미반환)
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_NoMatches_EmptySliceTotalZero(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()

	seedAuditLogs(t, st, []*audit.Event{
		{Timestamp: time.Now(), Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: uuid.New()},
	})

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	filter := AuditLogFilter{Action: strPtr("UNKNOWN_ACTION_XYZ")}
	result, total, err := tx.QueryAuditLogs(ctx, filter, 50, 0)
	require.NoError(t, err, "결과 0건은 에러 아님 (404/500 미반환)")
	assert.Equal(t, int64(0), total)
	assert.Empty(t, result)
}

// ════════════════════════════════════════════════════════════════════════════
// T-007 [S1] ORDER BY 결정성 (timestamp DESC)
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_OrderBy_TimestampDesc(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()

	t1 := time.Now().Add(-3 * time.Hour)
	t2 := time.Now().Add(-2 * time.Hour)
	t3 := time.Now().Add(-1 * time.Hour)

	seedAuditLogs(t, st, []*audit.Event{
		{Timestamp: t1, Action: audit.ActionScoreCreated, UserID: "a", ResourceID: uuid.New()},
		{Timestamp: t3, Action: audit.ActionScoreCreated, UserID: "c", ResourceID: uuid.New()},
		{Timestamp: t2, Action: audit.ActionScoreCreated, UserID: "b", ResourceID: uuid.New()},
	})

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	result, _, err := tx.QueryAuditLogs(ctx, AuditLogFilter{}, 50, 0)
	require.NoError(t, err)
	require.Len(t, result, 3)
	// timestamp DESC: t3 > t2 > t1
	assert.True(t, result[0].Timestamp.After(result[1].Timestamp), "ORDER BY timestamp DESC")
	assert.True(t, result[1].Timestamp.After(result[2].Timestamp), "ORDER BY timestamp DESC")
}

// ════════════════════════════════════════════════════════════════════════════
// T-008 [SEC] SQL injection 방어 — fake는 string match, 실 pgx는 parameter binding
// ════════════════════════════════════════════════════════════════════════════

func TestQueryAuditLogs_SQLInjectionDefense_NoMatchNoPanic(t *testing.T) {
	ctx := context.Background()
	st := NewFakeStore()

	seedAuditLogs(t, st, []*audit.Event{
		{Timestamp: time.Now(), Action: audit.ActionScoreCreated, UserID: "alice", ResourceID: uuid.New()},
	})

	tx, err := st.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	// injection 시도: literal string match만 (no SQL execution)
	maliciousAction := "'; DROP TABLE audit_logs; --"
	filter := AuditLogFilter{Action: &maliciousAction}
	result, total, err := tx.QueryAuditLogs(ctx, filter, 50, 0)
	require.NoError(t, err, "injection 시도는 literal string로 처리 — panic 없음")
	assert.Equal(t, int64(0), total, "DROP TABLE 실행 0 — audit_logs 무손상")
	assert.Empty(t, result)

	// 손상 검증: 정상 검색이 여전히 작동
	allResult, allTotal, allErr := tx.QueryAuditLogs(ctx, AuditLogFilter{}, 50, 0)
	require.NoError(t, allErr)
	assert.Equal(t, int64(1), allTotal, "audit_logs 무손상 — 원본 1개 row 유지")
	assert.Len(t, allResult, 1)
	assert.False(t, strings.Contains(allResult[0].UserID, "DROP"), "injection 미실행")
}
