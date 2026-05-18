// store.go — 워크플로우 영속성 인터페이스
// Sprint 1 RED: 인터페이스 정의만 포함, 실제 pgx 구현은 Sprint 3에서 담당
// 주의: postgres.go의 Store struct와 이름 충돌을 피하기 위해 WorkflowStore로 명명
package store

import (
	"context"

	"github.com/google/uuid"

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
	// ListWorkflows 워크플로우 목록을 limit/offset 기반으로 조회
	// 반환 순서: created_at DESC (최신순)
	// limit=0이면 기본값 100, 최대 1000 적용은 호출자(gRPC 핸들러) 책임
	ListWorkflows(ctx context.Context, limit, offset int) ([]*types.Workflow, error)
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

// EvidenceStore 증빙 영속성 최상위 인터페이스 (SPEC-AX-EVID-001 REQ-EVID-001)
// WorkflowStore 패턴을 미러링하여 BeginEvidenceTx 트랜잭션 진입점만 제공한다.
// 실제 DB 접근은 EvidenceTx를 통해서만 수행 (raw SQL 누출 방지).
//
// @MX:ANCHOR: [AUTO] evidence_handlers, recorder 연계, 향후 list 조회 등 3개 이상 호출자 예정
// @MX:REASON: 기존 WorkflowStore와 동일 — 증빙 도메인 단일 DB 접근 계약. 실 PgWorkflowStore.pool 재사용(R-EVID-005)
type EvidenceStore interface {
	// BeginEvidenceTx 새로운 증빙 트랜잭션을 시작하여 반환
	// 반환된 EvidenceTx는 반드시 Commit 또는 Rollback 중 하나로 종료해야 함
	BeginEvidenceTx(ctx context.Context) (EvidenceTx, error)
}

// EvidenceTx 단일 데이터베이스 트랜잭션 내 증빙 쓰기/조회 연산 인터페이스
// InsertEvidence와 InsertAuditLog는 동일 트랜잭션 내에서 atomic하게 처리되어야 함
// (REQ-EVID-UBI-002 / REQ-EVID-003-U1 트랜잭션 원자성 불변 조건)
//
// @MX:ANCHOR: [AUTO] AC-EVID-003-3 / AC-EVID-UBI-002 원자성 계약의 핵심
// @MX:REASON: pgx 구현체(PgEvidenceTx) + 핸들러 TX orchestration 등 3곳 이상에서 사용 — 증빙 원자성 단일 계약
type EvidenceTx interface {
	// InsertEvidence evidences 테이블에 새 증빙 행을 삽입하고 생성된 UUID를 반환
	// database_blob 전략 시 fileContent는 동일 TX의 BYTEA 컬럼에 저장됨 (strategy.md §2.6.5)
	// previousVersionID가 nil이 아니면 버전 체이닝 (version>1), nil이면 신규 증빙 (version=1)
	InsertEvidence(
		ctx context.Context,
		evalItemID, fileName, contentType string,
		fileSizeBytes int64,
		fileHashSHA256 string,
		storageStrategy, storageLocation string,
		metadata map[string]string,
		fileContent []byte,
		previousVersionID *uuid.UUID,
	) (uuid.UUID, error)
	// GetEvidenceByID 증빙 ID로 단건을 조회
	// 존재하지 않으면 errors.ErrEvidenceNotFound를 래핑하여 반환 (raw pgx.ErrNoRows 금지 — GAP-03)
	GetEvidenceByID(ctx context.Context, id uuid.UUID) (*Evidence, error)
	// GetLatestVersionByEvalItem 동일 evaluation_item_id의 최신(최대 version) 행을 SELECT ... FOR UPDATE로 조회
	// 동시 재업로드 직렬화(REQ-EVID-001-S1)를 위해 행 잠금을 획득한다.
	// 기존 증빙이 없으면 (nil, nil) 반환 (신규 증빙 경로)
	GetLatestVersionByEvalItem(ctx context.Context, evalItemID string) (*Evidence, error)
	// ListEvidenceByEvalItem 동일 evaluation_item_id의 모든 버전을 version DESC로 반환
	ListEvidenceByEvalItem(ctx context.Context, evalItemID string) ([]*Evidence, error)
	// MarkSuperseded 직전 버전 행의 status를 ACTIVE → SUPERSEDED로 전이 (store 계층 소유 — GAP-04)
	// 본문 컬럼은 절대 변경하지 않으며 status만 전이 (REQ-EVID-UBI-004)
	MarkSuperseded(ctx context.Context, id uuid.UUID) error
	// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입 (Recorder가 호출)
	InsertAuditLog(ctx context.Context, e *audit.Event) error
	// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
	Commit(ctx context.Context) error
	// Rollback 현재 트랜잭션을 롤백 — defer로 호출하는 것이 안전, Commit 후 무시
	Rollback(ctx context.Context) error
}
