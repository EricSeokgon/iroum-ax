// pg_store.go — pgx v5 기반 실제 PostgreSQL WorkflowStore 구현
// Sprint 3 GREEN: 실제 쿼리로 ErrNotImplemented stub 교체
// Sprint 0 postgres.go의 레거시 Store와 충돌 없이 공존
package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

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
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DSN 파싱 실패: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool 생성 실패: %w", err)
	}

	// 연결 검증 (fail-fast — AC-CTRL-004-1)
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping 실패: %w", err)
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
// @MX:WARN: [AUTO] pool 고갈 시 context 타임아웃까지 블록킹 가능
// @MX:REASON: AC-CTRL-004-3 pool exhaustion 테스트 — MaxConns=1 설정 시 두 번째 BeginTx가 블록
func (s *PgWorkflowStore) BeginTx(ctx context.Context) (WorkflowTx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("BeginTx 실패: %w", stderrors.ErrPgxPoolExhausted)
	}
	return &PgWorkflowTx{tx: tx, logger: s.logger}, nil
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
	// tx 래핑된 pgx 트랜잭션
	tx pgx.Tx
	// logger 구조화 로그
	logger *zap.Logger
}

// InsertWorkflow workflows 테이블에 새 행을 삽입
// SQLSTATE 23505 (unique_violation) 발생 시 에러를 래핑하여 반환
func (t *PgWorkflowTx) InsertWorkflow(ctx context.Context, w *types.Workflow) error {
	const query = `
		INSERT INTO workflows (id, status, document_id, report_id, result_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	// document_id: uuid.Nil이면 NULL로 저장
	var docID *uuid.UUID
	if w.DocumentID != uuid.Nil {
		docID = &w.DocumentID
	}

	_, err := t.tx.Exec(ctx, query,
		w.ID,
		string(w.State),
		docID,
		w.ReportID,
		w.ResultJSON,
		w.CreatedAt,
		w.UpdatedAt,
	)
	if err != nil {
		// SQLSTATE 23505: unique_violation (중복 PK)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return fmt.Errorf("워크플로우 ID 중복 (SQLSTATE 23505): %w", err)
		}
		t.logger.Error("InsertWorkflow 실패", zap.String("id", w.ID.String()), zap.Error(err))
		return fmt.Errorf("InsertWorkflow 실패: %w", err)
	}
	return nil
}

// GetWorkflow workflows 테이블에서 행을 조회 (SELECT ... FOR UPDATE)
// 워크플로우가 없으면 stderrors.ErrWorkflowNotFound를 래핑하여 반환
func (t *PgWorkflowTx) GetWorkflow(ctx context.Context, id string) (*types.Workflow, error) {
	const query = `
		SELECT id, status, document_id, report_id, result_json, created_at, updated_at
		FROM workflows
		WHERE id = $1
		FOR UPDATE
	`

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("워크플로우 ID 파싱 실패: %w", err)
	}

	var w types.Workflow
	var docID *uuid.UUID
	var reportID *uuid.UUID

	err = t.tx.QueryRow(ctx, query, parsedID).Scan(
		&w.ID,
		&w.State,
		&docID,
		&reportID,
		&w.ResultJSON,
		&w.CreatedAt,
		&w.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("GetWorkflow id=%s: %w", id, stderrors.ErrWorkflowNotFound)
		}
		t.logger.Error("GetWorkflow 실패", zap.String("id", id), zap.Error(err))
		return nil, fmt.Errorf("GetWorkflow 실패: %w", err)
	}

	if docID != nil {
		w.DocumentID = *docID
	}
	if reportID != nil {
		w.ReportID = reportID
	}

	return &w, nil
}

// UpdateWorkflowState 워크플로우 상태를 갱신
// 대상 행이 없으면 stderrors.ErrWorkflowNotFound를 반환
func (t *PgWorkflowTx) UpdateWorkflowState(ctx context.Context, id string, state types.WorkflowState) error {
	const query = `
		UPDATE workflows
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("워크플로우 ID 파싱 실패: %w", err)
	}

	result, err := t.tx.Exec(ctx, query, parsedID, string(state))
	if err != nil {
		t.logger.Error("UpdateWorkflowState 실패", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("UpdateWorkflowState 실패: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("UpdateWorkflowState id=%s: %w", id, stderrors.ErrWorkflowNotFound)
	}
	return nil
}

// UpdateWorkflowResult 워크플로우 result_json을 갱신
// 대상 행이 없으면 stderrors.ErrWorkflowNotFound를 반환
func (t *PgWorkflowTx) UpdateWorkflowResult(ctx context.Context, id string, resultJSON []byte) error {
	const query = `
		UPDATE workflows
		SET result_json = $2, updated_at = NOW()
		WHERE id = $1
	`

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("워크플로우 ID 파싱 실패: %w", err)
	}

	result, err := t.tx.Exec(ctx, query, parsedID, resultJSON)
	if err != nil {
		t.logger.Error("UpdateWorkflowResult 실패", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("UpdateWorkflowResult 실패: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("UpdateWorkflowResult id=%s: %w", id, stderrors.ErrWorkflowNotFound)
	}
	return nil
}

// InsertAuditLog audit_logs 테이블에 감사 이벤트 행을 삽입
// e.ID가 uuid.Nil이면 uuid.New()로 자동 생성
func (t *PgWorkflowTx) InsertAuditLog(ctx context.Context, e *audit.Event) error {
	const query = `
		INSERT INTO audit_logs (id, action, resource_type, resource_id, user_id, details, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	// ID 자동 생성 — Event 구조체에 ID 필드가 없으므로 항상 새로 생성
	id := uuid.New()

	// DetailsJSON을 JSONB로 직접 전달 ([]byte → pgx가 JSONB로 직렬화)
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
		t.logger.Error("InsertAuditLog 실패",
			zap.String("action", string(e.Action)),
			zap.String("resource_id", e.ResourceID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("InsertAuditLog 실패: %w", err)
	}
	return nil
}

// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
func (t *PgWorkflowTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("Commit 실패: %w", err)
	}
	return nil
}

// Rollback 현재 트랜잭션을 롤백하여 모든 변경사항을 취소
// Commit 후 호출 시 pgx는 무시(no-op)
func (t *PgWorkflowTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		// 이미 커밋된 트랜잭션의 롤백은 pgx가 ErrTxClosed를 반환 — 무시
		if err == pgx.ErrTxClosed {
			return nil
		}
		return fmt.Errorf("Rollback 실패: %w", err)
	}
	return nil
}

// newPgWorkflowStoreWithPool 기존 풀로 PgWorkflowStore 생성 (테스트용 풀 주입)
// testcontainers 기반 통합 테스트에서 커스텀 MaxConns 설정 후 풀을 주입할 때 사용
func newPgWorkflowStoreWithPool(pool *pgxpool.Pool, logger *zap.Logger) *PgWorkflowStore {
	return &PgWorkflowStore{pool: pool, logger: logger}
}
