// PostgreSQL 클라이언트 (documents / workflows 상태 저장)
// Sprint 0 스켈레톤 — 실제 쿼리는 Sprint 7에서 구현
package store

import "context"

// Config PostgreSQL 연결 설정
type Config struct {
	DSN         string // postgres://user:pass@host:5432/dbname
	MaxOpenConn int
	MaxIdleConn int
}

// DocumentRecord documents 테이블 레코드 (skeleton)
type DocumentRecord struct {
	ID       string `db:"id"`
	Filename string `db:"filename"`
	FileType string `db:"file_type"`
	Status   string `db:"status"`
}

// WorkflowRecord workflows 테이블 레코드 (skeleton)
type WorkflowRecord struct {
	ID         string `db:"id"`
	UserID     string `db:"user_id"`
	DocumentID string `db:"document_id"`
	Status     string `db:"status"`
}

// Store PostgreSQL 데이터 접근 레이어
// @MX:TODO - Sprint 7에서 pgx/v5 기반 실제 구현
type Store struct {
	cfg Config
}

// New Store 인스턴스 생성 (연결 풀 초기화 포함)
// TODO(Sprint 7): pgxpool.New() 연결 풀 설정
func New(cfg Config) (*Store, error) {
	return &Store{cfg: cfg}, nil
}

// GetDocument ID로 문서 레코드 조회 (stub)
// TODO(Sprint 7): SELECT 쿼리 구현
func (s *Store) GetDocument(_ context.Context, _ string) (*DocumentRecord, error) {
	return nil, nil
}

// CreateWorkflow 워크플로우 레코드 생성 (stub)
// TODO(Sprint 7): INSERT 쿼리 구현
func (s *Store) CreateWorkflow(_ context.Context, _ WorkflowRecord) error {
	return nil
}

// UpdateWorkflowStatus 워크플로우 상태 갱신 (stub)
// TODO(Sprint 7): UPDATE 쿼리 구현
func (s *Store) UpdateWorkflowStatus(_ context.Context, _, _ string) error {
	return nil
}
