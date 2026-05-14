// fake_store.go — 단위 테스트용 인메모리 WorkflowStore 구현
// Sprint 1 RED: FakeStore + FakeTx를 통해 트랜잭션 원자성 동작을 시뮬레이션
// 실제 pgx 구현체(Sprint 3)가 동일 인터페이스를 구현하기 전 테스트 격리 제공
package store

import (
	"context"
	"errors"
	"sync"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// errFakeNotImplemented 미구현 메서드 호출 시 반환하는 sentinel 에러
// Sprint 1 stub에서 GREEN 단계 진행까지 모든 미구현 경로에 사용
var errFakeNotImplemented = errors.New("not implemented")

// FakeStore 단위 테스트용 인메모리 WorkflowStore 구현체
// BeginTx로 FakeTx를 생성하며, commit/rollback 에 따라 영속 저장소에 반영
//
// @MX:TODO - Sprint 1 GREEN: FakeStore.BeginTx 실제 동작 구현
type FakeStore struct {
	// Workflows commit된 워크플로우 행 (key: workflow ID 문자열)
	Workflows map[string]*types.Workflow
	// AuditLogs commit된 감사 이벤트 행
	AuditLogs []*audit.Event

	// mu 동시 접근 보호 (GREEN에서 BeginTx 구현 시 사용)
	mu sync.RWMutex //nolint:unused

	// NextTxFailOnAudit 다음 BeginTx에서 반환하는 FakeTx의 FailOnAuditInsert=true 설정
	// 트랜잭션 원자성 Scenario A/C 테스트용 (AC-CTRL-UBI-001)
	NextTxFailOnAudit bool
	// NextTxFailOnWorkflow 다음 BeginTx에서 반환하는 FakeTx의 FailOnWorkflowInsert=true 설정
	// 트랜잭션 원자성 Scenario B 테스트용 (AC-CTRL-UBI-001)
	NextTxFailOnWorkflow bool
}

// NewFakeStore FakeStore 인스턴스를 초기화하여 반환
func NewFakeStore() *FakeStore {
	return &FakeStore{
		Workflows: make(map[string]*types.Workflow),
		AuditLogs: make([]*audit.Event, 0),
	}
}

// BeginTx 새로운 FakeTx를 생성하여 반환
// FakeTx는 Commit 전까지 모든 변경사항을 내부 버퍼에 보관
// NextTxFailOnAudit/NextTxFailOnWorkflow 플래그를 FakeTx에 전파
//
// @MX:TODO - Sprint 1 GREEN: 반환 stub 제거 후 실제 FakeTx 반환
func (s *FakeStore) BeginTx(_ context.Context) (WorkflowTx, error) {
	return nil, errFakeNotImplemented
}

// FakeTx 단위 테스트용 인메모리 WorkflowTx 구현체
// 장애 주입(FailOnAuditInsert, FailOnWorkflowInsert)을 통해 rollback 경로 테스트 지원
//
// @MX:TODO - Sprint 1 GREEN: 모든 메서드에 실제 동작 구현
type FakeTx struct {
	// store 상위 FakeStore 참조 (commit 시 반영 대상, GREEN에서 사용)
	store *FakeStore //nolint:unused
	// pendingUpdates 아직 commit되지 않은 상태 갱신 버퍼 (id → newState, GREEN에서 사용)
	pendingUpdates map[string]types.WorkflowState //nolint:unused

	// pendingWorkflows 아직 commit되지 않은 워크플로우 행 버퍼 (GREEN에서 사용)
	pendingWorkflows []*types.Workflow //nolint:unused
	// pendingAuditLogs 아직 commit되지 않은 감사 이벤트 버퍼 (GREEN에서 사용)
	pendingAuditLogs []*audit.Event //nolint:unused

	// FailOnAuditInsert true이면 InsertAuditLog 호출 시 에러 반환 (장애 주입)
	FailOnAuditInsert bool
	// FailOnWorkflowInsert true이면 InsertWorkflow 호출 시 에러 반환 (장애 주입)
	FailOnWorkflowInsert bool
	// FailOnUpdateState true이면 UpdateWorkflowState 호출 시 에러 반환 (장애 주입)
	FailOnUpdateState bool

	// committed commit 완료 플래그 (rollback 중복 호출 방지, GREEN에서 사용)
	committed bool //nolint:unused
}

// InsertWorkflow 트랜잭션 버퍼에 워크플로우 행을 추가
// FailOnWorkflowInsert=true이면 장애를 시뮬레이션
//
// @MX:TODO - Sprint 1 GREEN: stub 제거 후 버퍼에 추가하는 동작 구현
func (tx *FakeTx) InsertWorkflow(_ context.Context, _ *types.Workflow) error {
	return errFakeNotImplemented
}

// InsertAuditLog 트랜잭션 버퍼에 감사 이벤트 행을 추가
// FailOnAuditInsert=true이면 장애를 시뮬레이션
//
// @MX:TODO - Sprint 1 GREEN: stub 제거 후 버퍼에 추가하는 동작 구현
func (tx *FakeTx) InsertAuditLog(_ context.Context, _ *audit.Event) error {
	return errFakeNotImplemented
}

// UpdateWorkflowState 트랜잭션 버퍼에 상태 갱신을 추가
//
// @MX:TODO - Sprint 1 GREEN: stub 제거 후 pendingUpdates에 기록
func (tx *FakeTx) UpdateWorkflowState(_ context.Context, _ string, _ types.WorkflowState) error {
	return errFakeNotImplemented
}

// Commit 버퍼의 모든 변경사항을 FakeStore 영속 저장소에 반영
//
// @MX:TODO - Sprint 1 GREEN: pendingWorkflows + pendingAuditLogs + pendingUpdates를 store에 반영
func (tx *FakeTx) Commit(_ context.Context) error {
	return errFakeNotImplemented
}

// Rollback 버퍼의 모든 변경사항을 폐기 (영속 저장소 무수정)
// Commit 후 호출 시 무시 (幂等)
//
// @MX:TODO - Sprint 1 GREEN: committed 플래그 확인 후 버퍼 클리어
func (tx *FakeTx) Rollback(_ context.Context) error {
	return errFakeNotImplemented
}
