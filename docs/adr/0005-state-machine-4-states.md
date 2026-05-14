# ADR 0005: Workflow State Machine: 4 states (RETRYING/CANCELLED 제외)

## Status

Accepted (2026-05-14)

## Context

**Walking Skeleton 단계의 워크플로우 생명주기 설계**:

Workflow 상태 선택지:
- **Path A (채택)**: 4 states (PENDING, RUNNING, COMPLETED, FAILED)
- **Path B**: 6+ states (+ RETRYING, CANCELLED, PAUSED)
- **Path C**: Event-sourced (상태 대신 이벤트 스트림 저장)

**제약 조건**:
- Walking Skeleton 범위: 단일 시도, 취소 API 없음
- SPEC-AX-001 완료: 초안 생성, 추천까지만 (재시도 정책 불필요)
- 운영 단계: 후속 SPEC에서 재시도, 취소 처리 예정

## Decision

**4 states + 3 transitions** 구현.

상태 정의:

| 상태 | 의미 | Terminal? |
|------|------|----------|
| PENDING | 워크플로우 생성, dispatch 전 | No |
| RUNNING | dispatch 완료, worker 처리 중 | No |
| COMPLETED | 성공 종료 | **Yes** |
| FAILED | 실패 종료 (dispatch or worker) | **Yes** |

전이 경로:

```
PENDING → RUNNING (dispatch 성공)
       ↘ FAILED  (dispatch 실패)

RUNNING → COMPLETED (worker 완료, 상태=success)
       ↘ FAILED    (worker 실패, 상태=error)
```

## Consequences

### 긍정적 영향

- **명확성**: 상태 정의 간결, worker는 성공/실패만 보냄
- **구현 단순**: 3개 전이만 구현, race condition 최소화
- **SELECT FOR UPDATE**: PostgreSQL row-level locking으로 atomic transition 보증
- **감시 용이**: terminal state (COMPLETED/FAILED) 도달 시 추가 전이 불가

### 부정적 영향

- **재시도 불가**: RETRYING state 없으므로 자동 retry 불가
  - Mitigation: callback 실패 시 즉시 FAILED로 전이, 클라이언트가 재호출
- **취소 불가**: CANCELLED state 없으므로 in-flight 작업 취소 불가
  - Mitigation: 운영 단계 SPEC-AX-OUTBOX-001에서 별도 처리

## Implementation

### State Machine (apps/control-plane/internal/workflow/state_machine.go)

```go
type WorkflowStatus string

const (
    StatusPending   WorkflowStatus = "PENDING"
    StatusRunning   WorkflowStatus = "RUNNING"
    StatusCompleted WorkflowStatus = "COMPLETED"
    StatusFailed    WorkflowStatus = "FAILED"
)

type StateMachine struct {
    CurrentStatus WorkflowStatus
}

func (s *StateMachine) CanTransition(from, to WorkflowStatus) bool {
    switch {
    case from == StatusPending && to == StatusRunning:
        return true
    case from == StatusPending && to == StatusFailed:
        return true
    case from == StatusRunning && to == StatusCompleted:
        return true
    case from == StatusRunning && to == StatusFailed:
        return true
    case IsTerminal(from):
        return false  // No transition from terminal state
    default:
        return false
    }
}

func IsTerminal(status WorkflowStatus) bool {
    return status == StatusCompleted || status == StatusFailed
}

// PostgreSQL transaction context
func (s *WorkflowStore) Transition(ctx context.Context, tx pgx.Tx, 
    workflowID string, newStatus WorkflowStatus) error {
    
    var currentStatus WorkflowStatus
    // SELECT FOR UPDATE: lock row until tx commit
    row := tx.QueryRow(ctx, 
        "SELECT status FROM workflows WHERE id = $1 FOR UPDATE", 
        workflowID)
    if err := row.Scan(&currentStatus); err != nil {
        return err
    }
    
    // Validate transition
    if !s.stateMachine.CanTransition(currentStatus, newStatus) {
        return fmt.Errorf("invalid transition: %s → %s", currentStatus, newStatus)
    }
    
    // Update + audit
    _, err := tx.Exec(ctx, 
        "UPDATE workflows SET status = $1, updated_at = NOW() WHERE id = $2",
        newStatus, workflowID)
    if err != nil {
        return err
    }
    
    // Insert audit log
    _, err = tx.Exec(ctx,
        "INSERT INTO audit_logs (user_id, action, resource_type, resource_id) "+
        "VALUES ($1, $2, $3, $4)",
        "cli-anonymous", 
        "WORKFLOW_TRANSITIONED_TO_" + string(newStatus),
        "workflow",
        workflowID)
    
    return err
}
```

### Acceptance Criteria

| ID | 기준 |
|----|------|
| AC-CTRL-001-1 | 4 states + 3 transitions 구현 |
| AC-CTRL-001-2 | SELECT FOR UPDATE로 race condition 방지 |
| AC-CTRL-001-3 | Terminal state 도달 후 전이 거부 (409) |
| AC-CTRL-001-4 | 동시성 단위 테스트 (2 goroutine 동시 callback) |

## References

- SPEC-AX-CTRL-001 spec.md REQ-CTRL-001 (State Machine)
- SPEC-AX-CTRL-001 research.md §2.2 (Audit log enumeration)
- `.moai/project/structure.md` §5 (workflows 테이블 스키마)
- PostgreSQL SELECT FOR UPDATE: https://www.postgresql.org/docs/current/sql-select.html

---

**작성자**: ircp  
**날짜**: 2026-05-14
