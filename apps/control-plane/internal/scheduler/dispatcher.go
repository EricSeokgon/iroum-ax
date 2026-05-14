// Celery 태스크 디스패처
// Sprint 0 스켈레톤 — 실제 구현은 Sprint 7 예정
package scheduler

// TaskName Celery 태스크 이름
type TaskName string

const (
	TaskIngestion  TaskName = "pipelines.workers.ingestion_worker.run"
	TaskGeneration TaskName = "pipelines.workers.generation_worker.run"
	TaskSimulation TaskName = "pipelines.workers.simulation_worker.run"
)

// DispatchRequest Celery 태스크 디스패치 요청
// 필드 순서: 슬라이스(24바이트) → map(8바이트) → 문자열들 (패딩 최소화)
type DispatchRequest struct {
	Kwargs  map[string]interface{} `json:"kwargs"`
	Task    TaskName               `json:"task"`
	QueueID string                 `json:"queue_id,omitempty"`
	Args    []interface{}          `json:"args"`
}

// Dispatcher Celery 브로커(Redis)에 태스크를 전송하는 디스패처
// @MX:TODO - Sprint 7에서 Redis RPUSH 기반 Celery 프로토콜 구현
type Dispatcher struct {
	brokerURL string
}

// NewDispatcher Dispatcher 인스턴스 생성
func NewDispatcher(brokerURL string) *Dispatcher {
	return &Dispatcher{brokerURL: brokerURL}
}

// Dispatch 태스크를 Celery 브로커에 전송 (stub)
// TODO(Sprint 7): Redis 연결 및 Celery 메시지 포맷 직렬화 구현
func (d *Dispatcher) Dispatch(_ DispatchRequest) (string, error) {
	return "", nil
}
