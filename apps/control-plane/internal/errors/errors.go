// 컨트롤 플레인 도메인 에러 정의
// Python pkg/errors/custom_errors.py 에 대응하는 Go 센티널 에러
// errors.Is / errors.As 호환 패턴 사용
package errors

import "errors"

// ErrInvalidTransition 허용되지 않은 워크플로우 상태 전이 시도
var ErrInvalidTransition = errors.New("invalid workflow state transition")

// ErrWorkflowNotFound 요청한 워크플로우 ID가 존재하지 않음
var ErrWorkflowNotFound = errors.New("workflow not found")

// ErrAuditLogFailed 감사 이벤트 기록 실패 (DB 쓰기 오류)
var ErrAuditLogFailed = errors.New("audit log write failed")

// ErrCeleryDispatchFailed Celery 브로커(Redis)로 태스크 전송 실패
var ErrCeleryDispatchFailed = errors.New("celery task dispatch failed")

// ErrPgxPoolExhausted pgx 연결 풀 고갈 — 재시도 또는 회로 차단기 필요
var ErrPgxPoolExhausted = errors.New("pgx connection pool exhausted")
