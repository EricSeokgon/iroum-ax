// 워크플로우 핵심 타입 정의 — 상태 열거형, 엔티티 구조체, 전이 유효성 검사
// Sprint 0: 타입 선언만 포함, 비즈니스 로직은 Sprint 2에서 구현
package types

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowState 워크플로우 실행 상태 (문자열 기반 열거형)
// @MX:ANCHOR: [AUTO] 전체 스프린트에서 상태 전이 로직이 이 타입에 의존함
// @MX:REASON: Sprint 2 상태 머신, Sprint 3 감사 로그, Sprint 5 gRPC 핸들러가 모두 참조
type WorkflowState string

const (
	// WorkflowStatePending 워크플로우 생성 직후 초기 상태
	WorkflowStatePending WorkflowState = "PENDING"
	// WorkflowStateRunning Celery 태스크 디스패치 후 실행 중 상태
	WorkflowStateRunning WorkflowState = "RUNNING"
	// WorkflowStateCompleted 모든 파이프라인 단계 성공 완료
	WorkflowStateCompleted WorkflowState = "COMPLETED"
	// WorkflowStateFailed 처리 중 오류 발생 또는 디스패치 실패
	WorkflowStateFailed WorkflowState = "FAILED"
)

// ValidTransitions 허용된 상태 전이 맵
// PENDING → RUNNING, FAILED
// RUNNING → COMPLETED, FAILED
// COMPLETED, FAILED 는 종료 상태 (전이 없음)
var ValidTransitions = map[WorkflowState][]WorkflowState{
	WorkflowStatePending: {WorkflowStateRunning, WorkflowStateFailed},
	WorkflowStateRunning: {WorkflowStateCompleted, WorkflowStateFailed},
}

// Workflow 워크플로우 상태 엔티티 — Python Pydantic WorkflowRecord에 대응
// 필드 순서: 포인터(8바이트) → 슬라이스(24바이트) → 시간(24바이트) → UUID(16바이트) × 3 → 문자열
type Workflow struct {
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
	ReportID   *uuid.UUID    `json:"report_id,omitempty"`
	State      WorkflowState `json:"state"`
	ResultJSON []byte        `json:"result_json,omitempty"`
	ID         uuid.UUID     `json:"id"`
	DocumentID uuid.UUID     `json:"document_id"`
}

// IsValidTransition 주어진 상태 전이가 허용된 전이인지 검사
// Sprint 2에서 상태 머신이 이 함수를 호출하여 전이 유효성을 확인함
//
// @MX:ANCHOR: [AUTO] Sprint 2 상태 머신의 핵심 게이트 함수
// @MX:REASON: 상태 머신, 감사 로그, gRPC 핸들러 3곳 이상에서 호출 예정
func IsValidTransition(from, to WorkflowState) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
