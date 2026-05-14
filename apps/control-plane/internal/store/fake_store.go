// fake_store.go — 단위 테스트용 인메모리 WorkflowStore 구현
// Sprint 1 GREEN: FakeStore + FakeTx 트랜잭션 원자성 시뮬레이션 구현
// 실제 pgx 구현체(Sprint 3)가 동일 인터페이스를 구현하기 전 테스트 격리 제공
package store

import (
	"context"
	"errors"
	"sync"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/types"
)

// errFakeAuditInsertFail audit INSERT 장애 주입 시 반환하는 sentinel 에러
var errFakeAuditInsertFail = errors.New("fake: injected audit insert failure")

// errFakeWorkflowInsertFail workflow INSERT 장애 주입 시 반환하는 sentinel 에러
var errFakeWorkflowInsertFail = errors.New("fake: injected workflow insert failure")

// errFakeUpdateStateFail UpdateWorkflowState 장애 주입 시 반환하는 sentinel 에러
var errFakeUpdateStateFail = errors.New("fake: injected update state failure")

// FakeStore 단위 테스트용 인메모리 WorkflowStore 구현체
// BeginTx로 FakeTx를 생성하며, commit/rollback에 따라 영속 저장소에 반영
//
// @MX:ANCHOR: [AUTO] 스프린트 전반의 단위 테스트 격리 제공 — pgx 없이 원자성 검증
// @MX:REASON: FakeStore는 store, workflow, audit 패키지 테스트 3곳에서 사용
type FakeStore struct {
	// Workflows commit된 워크플로우 행 (key: workflow ID 문자열)
	Workflows map[string]*types.Workflow
	// AuditLogs commit된 감사 이벤트 행
	AuditLogs []*audit.Event

	// mu 동시 접근 보호 (맵/슬라이스 뒤에 배치하여 fieldalignment 최적화)
	mu sync.Mutex //nolint:structcheck

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
// NextTxFailOnAudit/NextTxFailOnWorkflow 플래그를 FakeTx에 전파
//
// @MX:ANCHOR: [AUTO] 트랜잭션 격리의 진입점 — 모든 쓰기 연산이 이 경로를 통해 시작
// @MX:REASON: workflow, store, audit 테스트 모두 BeginTx를 통해 격리된 Tx를 획득
func (s *FakeStore) BeginTx(_ context.Context) (WorkflowTx, error) {
	// NextTx 플래그를 읽고 초기화 (한 번만 적용)
	s.mu.Lock()
	failAudit := s.NextTxFailOnAudit
	failWorkflow := s.NextTxFailOnWorkflow
	s.NextTxFailOnAudit = false
	s.NextTxFailOnWorkflow = false
	s.mu.Unlock()

	tx := &FakeTx{
		store:                s,
		pendingWorkflows:     make([]*types.Workflow, 0),
		pendingAuditLogs:     make([]*audit.Event, 0),
		pendingUpdates:       make(map[string]types.WorkflowState),
		FailOnAuditInsert:    failAudit,
		FailOnWorkflowInsert: failWorkflow,
	}
	return tx, nil
}

// FakeTx 단위 테스트용 인메모리 WorkflowTx 구현체
// 장애 주입(FailOnAuditInsert, FailOnWorkflowInsert)을 통해 rollback 경로 테스트 지원
type FakeTx struct {
	// store 상위 FakeStore 참조 (commit 시 반영 대상)
	store *FakeStore
	// pendingUpdates 아직 commit되지 않은 상태 갱신 버퍼 (id → newState)
	pendingUpdates map[string]types.WorkflowState
	// pendingWorkflows 아직 commit되지 않은 워크플로우 행 버퍼
	pendingWorkflows []*types.Workflow
	// pendingAuditLogs 아직 commit되지 않은 감사 이벤트 버퍼
	pendingAuditLogs []*audit.Event

	// FailOnAuditInsert true이면 InsertAuditLog 호출 시 에러 반환 (장애 주입)
	FailOnAuditInsert bool
	// FailOnWorkflowInsert true이면 InsertWorkflow 호출 시 에러 반환 (장애 주입)
	FailOnWorkflowInsert bool
	// FailOnUpdateState true이면 UpdateWorkflowState 호출 시 에러 반환 (장애 주입)
	FailOnUpdateState bool

	// committed commit 완료 플래그 (rollback 중복 호출 방지)
	committed bool
}

// InsertWorkflow 트랜잭션 버퍼에 워크플로우 행을 추가
// FailOnWorkflowInsert=true이면 장애를 시뮬레이션
func (tx *FakeTx) InsertWorkflow(_ context.Context, w *types.Workflow) error {
	if tx.FailOnWorkflowInsert {
		return errFakeWorkflowInsertFail
	}
	// 얕은 복사로 Tx 내부 버퍼에 격리 보관
	wfCopy := *w
	tx.pendingWorkflows = append(tx.pendingWorkflows, &wfCopy)
	return nil
}

// InsertAuditLog 트랜잭션 버퍼에 감사 이벤트 행을 추가
// FailOnAuditInsert=true이면 장애를 시뮬레이션
func (tx *FakeTx) InsertAuditLog(_ context.Context, e *audit.Event) error {
	if tx.FailOnAuditInsert {
		return errFakeAuditInsertFail
	}
	evCopy := *e
	tx.pendingAuditLogs = append(tx.pendingAuditLogs, &evCopy)
	return nil
}

// UpdateWorkflowState 트랜잭션 버퍼에 상태 갱신을 추가
// FailOnUpdateState=true이면 장애를 시뮬레이션
func (tx *FakeTx) UpdateWorkflowState(_ context.Context, id string, newState types.WorkflowState) error {
	if tx.FailOnUpdateState {
		return errFakeUpdateStateFail
	}
	tx.pendingUpdates[id] = newState
	return nil
}

// Commit 버퍼의 모든 변경사항을 FakeStore 영속 저장소에 반영
// committed=true 이후에는 멱등하게 nil 반환
func (tx *FakeTx) Commit(_ context.Context) error {
	if tx.committed {
		return nil
	}
	tx.committed = true

	tx.store.mu.Lock()
	defer tx.store.mu.Unlock()

	// pendingWorkflows를 store에 반영
	for _, wf := range tx.pendingWorkflows {
		tx.store.Workflows[wf.ID.String()] = wf
	}
	// pendingAuditLogs를 store에 반영
	tx.store.AuditLogs = append(tx.store.AuditLogs, tx.pendingAuditLogs...)
	// pendingUpdates를 store의 Workflows에 반영
	for id, newState := range tx.pendingUpdates {
		if wf, ok := tx.store.Workflows[id]; ok {
			wf.State = newState
		}
	}
	return nil
}

// Rollback 버퍼의 모든 변경사항을 폐기 (영속 저장소 무수정)
// Commit 후 호출 시 무시 (멱등)
func (tx *FakeTx) Rollback(_ context.Context) error {
	if tx.committed {
		// commit 이후 rollback은 no-op
		return nil
	}
	// 버퍼를 비워 pending 데이터 폐기
	tx.pendingWorkflows = nil
	tx.pendingAuditLogs = nil
	tx.pendingUpdates = make(map[string]types.WorkflowState)
	return nil
}
