// store.go — 워크플로우 영속성 인터페이스
// Sprint 1 RED: 인터페이스 정의만 포함, 실제 pgx 구현은 Sprint 3에서 담당
// 주의: postgres.go의 Store struct와 이름 충돌을 피하기 위해 WorkflowStore로 명명
package store

import (
	"context"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// WorkflowStore 워크플로우 영속성 최상위 인터페이스
// 트랜잭션 시작 진입점을 제공하며, 실제 DB 접근은 WorkflowTx를 통해서만 수행
//
// @MX:ANCHOR: [AUTO] Sprint 3 pgx 구현체, Sprint 1 FakeStore가 이 인터페이스를 구현
// @MX:REASON: gRPC 핸들러, 워크플로우 핸들러, 감사 레코더 등 3개 이상 호출자 예정
type WorkflowStore interface {
	// BeginTx 새로운 데이터베이스 트랜잭션을 시작하여 반환
	// 반환된 WorkflowTx는 반드시 Commit 또는 Rollback 중 하나로 종료해야 함
	BeginTx(ctx context.Context) (WorkflowTx, error)
}

// WorkflowTx 단일 데이터베이스 트랜잭션 내 쓰기 연산 인터페이스
// InsertWorkflow와 InsertAuditLog는 동일 트랜잭션 내에서 atomic하게 처리되어야 함
// (REQ-CTRL-UBI-001 트랜잭션 원자성 불변 조건)
//
// @MX:ANCHOR: [AUTO] AC-CTRL-UBI-001 Scenario A/B/C 원자성 테스트의 핵심 계약
// @MX:REASON: FakeStore, pgx 구현체, 워크플로우 트랜잭션 코디네이터 3곳에서 구현
type WorkflowTx interface {
	// InsertWorkflow workflows 테이블에 새 행을 삽입
	InsertWorkflow(ctx context.Context, w *types.Workflow) error
	// InsertAuditLog audit_logs 테이블에 감사 이벤트 행을 삽입
	InsertAuditLog(ctx context.Context, e *audit.Event) error
	// UpdateWorkflowState 워크플로우 상태를 갱신 (SELECT FOR UPDATE 후 호출)
	UpdateWorkflowState(ctx context.Context, id string, newState types.WorkflowState) error
	// GetWorkflow 트랜잭션 내에서 워크플로우 행을 조회 (Sprint 3에서 SELECT FOR UPDATE로 구현)
	// 워크플로우가 없으면 errors.ErrWorkflowNotFound를 래핑하여 반환
	GetWorkflow(ctx context.Context, id string) (*types.Workflow, error)
	// UpdateWorkflowResult 워크플로우 resultJSON을 갱신 (RUNNING → COMPLETED 전이 시 사용)
	UpdateWorkflowResult(ctx context.Context, id string, resultJSON []byte) error
	// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
	Commit(ctx context.Context) error
	// Rollback 현재 트랜잭션을 롤백하여 모든 변경사항을 취소
	// defer로 호출하는 것이 안전하며, Commit 후 호출 시 무시
	Rollback(ctx context.Context) error
}
