// pg_store.go — pgx v5 기반 실제 PostgreSQL WorkflowStore 구현
// Sprint 3 GREEN에서 실제 쿼리로 교체될 stub 단계
// Sprint 0 postgres.go의 레거시 Store와 충돌 없이 공존
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// ErrNotImplemented Sprint 3 RED phase — pgx 구현 미완성 sentinel 에러
// GREEN phase에서 실제 쿼리 구현 후 제거됨
//
// @MX:TODO: [AUTO] Sprint 3 GREEN에서 실제 pgx 쿼리로 교체 필요
var ErrNotImplemented = errors.New("pg store: not implemented yet")

// PgWorkflowStore pgx v5 연결 풀 기반 WorkflowStore 구현체
// WorkflowStore 인터페이스를 구현하며, BeginTx로 PgWorkflowTx를 반환
//
// @MX:ANCHOR: [AUTO] gRPC 핸들러, 워크플로우 핸들러, 통합 테스트가 이 구조체를 사용
// @MX:REASON: control-plane의 유일한 실제 DB 접근 계층 — 3개 이상 호출자 예정
type PgWorkflowStore struct {
	// pool pgx 연결 풀 (pgxpool.New로 초기화)
	pool *pgxpool.Pool
	// logger 구조화 로그 출력용 zap 로거
	logger *zap.Logger
}

// NewPgWorkflowStore pgx 연결 풀을 초기화하여 PgWorkflowStore를 반환
// dsn 연결 실패 시 에러를 반환하며 *PgWorkflowStore는 nil
//
// @MX:ANCHOR: [AUTO] main.go + 통합 테스트 진입점 — pool 초기화 실패 시 fail-fast
// @MX:REASON: AC-CTRL-004-1 fail-fast 요구사항 — 잘못된 DSN 시 5초 이내 에러 반환
func NewPgWorkflowStore(ctx context.Context, dsn string, logger *zap.Logger) (*PgWorkflowStore, error) {
	// Sprint 3 RED: 연결 풀 초기화 stub
	// GREEN에서: pgxpool.ParseConfig + pgxpool.NewWithConfig 구현
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	// 연결 검증 (fail-fast — AC-CTRL-004-1)
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PgWorkflowStore{pool: pool, logger: logger}, nil
}

// Close pgx 연결 풀을 닫음 — 테스트 정리(t.Cleanup)에서 호출
func (s *PgWorkflowStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// BeginTx 새로운 PostgreSQL 트랜잭션을 시작하여 PgWorkflowTx를 반환
// 반환된 PgWorkflowTx는 반드시 Commit 또는 Rollback 중 하나로 종료해야 함
//
// @MX:WARN: [AUTO] pool 고갈 시 블록킹 가능 — context timeout 필수
// @MX:REASON: AC-CTRL-004-3 pool exhaustion 테스트 — MaxConns=1 설정 시 두 번째 BeginTx가 블록
func (s *PgWorkflowStore) BeginTx(ctx context.Context) (WorkflowTx, error) {
	// Sprint 3 RED: stub — GREEN에서 s.pool.Begin(ctx) 구현
	return nil, ErrNotImplemented
}

// PoolStats pgx 연결 풀 상태를 반환 (풀 고갈 테스트용)
func (s *PgWorkflowStore) PoolStats() *pgxpool.Stat {
	return s.pool.Stat()
}

// PgWorkflowTx pgx.Tx 래퍼 — WorkflowTx 인터페이스 구현
// 단일 PostgreSQL 트랜잭션 내에서 모든 쓰기 연산을 수행
//
// @MX:ANCHOR: [AUTO] AC-CTRL-004-3 SELECT FOR UPDATE 직렬화 핵심 구조체
// @MX:REASON: 동시 트랜잭션 테스트(goroutine 2개)가 이 구조체의 락 동작을 검증
type PgWorkflowTx struct {
	// tx 래핑된 pgx 트랜잭션 (GREEN에서 실제 pgx.Tx로 교체)
	tx pgx.Tx //nolint:unused // Sprint 3 GREEN에서 활성화
	// logger 구조화 로그
	logger *zap.Logger //nolint:unused // Sprint 3 GREEN에서 활성화
}

// InsertWorkflow workflows 테이블에 새 행을 삽입
// Sprint 3 RED: ErrNotImplemented 반환 (GREEN에서 INSERT INTO workflows 구현)
func (t *PgWorkflowTx) InsertWorkflow(_ context.Context, _ *types.Workflow) error {
	return ErrNotImplemented
}

// GetWorkflow workflows 테이블에서 행을 조회 (SELECT ... FOR UPDATE)
// Sprint 3 RED: ErrNotImplemented 반환
// GREEN에서: SELECT id, status, document_id, report_id, result_json, created_at, updated_at
//
//	FROM workflows WHERE id=$1 FOR UPDATE 구현
func (t *PgWorkflowTx) GetWorkflow(_ context.Context, _ string) (*types.Workflow, error) {
	return nil, ErrNotImplemented
}

// UpdateWorkflowState 워크플로우 상태를 갱신
// Sprint 3 RED: ErrNotImplemented 반환
// GREEN에서: UPDATE workflows SET status=$2, updated_at=NOW() WHERE id=$1 구현
func (t *PgWorkflowTx) UpdateWorkflowState(_ context.Context, _ string, _ types.WorkflowState) error {
	return ErrNotImplemented
}

// UpdateWorkflowResult 워크플로우 result_json을 갱신
// Sprint 3 RED: ErrNotImplemented 반환
// GREEN에서: UPDATE workflows SET result_json=$2, updated_at=NOW() WHERE id=$1 구현
func (t *PgWorkflowTx) UpdateWorkflowResult(_ context.Context, _ string, _ []byte) error {
	return ErrNotImplemented
}

// InsertAuditLog audit_logs 테이블에 감사 이벤트 행을 삽입
// Sprint 3 RED: ErrNotImplemented 반환
// GREEN에서: INSERT INTO audit_logs(action, resource_type, resource_id, user_id, details, timestamp) 구현
func (t *PgWorkflowTx) InsertAuditLog(_ context.Context, _ *audit.Event) error {
	return ErrNotImplemented
}

// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
// Sprint 3 RED: ErrNotImplemented 반환
func (t *PgWorkflowTx) Commit(ctx context.Context) error {
	return ErrNotImplemented
}

// Rollback 현재 트랜잭션을 롤백하여 모든 변경사항을 취소
// Commit 후 호출 시 pgx는 무시(no-op) — Sprint 3 RED는 ErrNotImplemented 반환
func (t *PgWorkflowTx) Rollback(ctx context.Context) error {
	return ErrNotImplemented
}

// newPgWorkflowStoreWithPool 기존 풀로 PgWorkflowStore 생성 (테스트용 풀 주입)
// testcontainers 기반 통합 테스트에서 커스텀 MaxConns 설정 후 풀을 주입할 때 사용
func newPgWorkflowStoreWithPool(pool *pgxpool.Pool, logger *zap.Logger) *PgWorkflowStore {
	return &PgWorkflowStore{pool: pool, logger: logger}
}
