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

// ErrEvidenceNotFound 요청한 증빙 ID가 존재하지 않음 (SPEC-AX-EVID-001 GAP-03/DC-012)
// GetEvidenceByID는 pgx.ErrNoRows 대신 이 타입 센티널을 래핑하여 반환한다.
var ErrEvidenceNotFound = errors.New("evidence not found")

// ErrEvidenceImmutable successor가 존재하는 이전 버전 증빙 본문 컬럼 변경 시도
// (REQ-EVID-UBI-004 / REQ-EVID-002-U1 — store 계층 mutation guard가 SQL 미실행 후 반환)
var ErrEvidenceImmutable = errors.New("evidence is immutable: a successor version exists")

// ErrEvalItemNotFound 요청한 평가항목 id가 존재하지 않음 (SPEC-AX-EVAL-ITEM-001)
// GetEvalItemByID는 pgx.ErrNoRows 대신 이 센티널을 래핑하여 반환한다.
var ErrEvalItemNotFound = errors.New("evaluation item not found")

// ErrEvalItemInvalidInput 평가항목 입력 검증 실패 (id/display_name/hierarchy_code blank,
// id 64자 초과 등 — REQ-EVALITEM-001-U1 / GAP-03). store 계층이 SQL 미실행 후 반환.
var ErrEvalItemInvalidInput = errors.New("evaluation item invalid input")

// ErrEvalItemParentNotFound 지정한 parent_id 항목이 존재하지 않음 (orphan 방지)
// REQ-EVALITEM-001-S1 — store 계층 사전 조회가 SQL INSERT 미실행 후 반환.
var ErrEvalItemParentNotFound = errors.New("evaluation item parent not found")

// ErrEvalItemHierarchyImmutable 자식(successor)이 존재하는 항목의 parent_id/level 변경 시도
// (REQ-EVALITEM-UBI-004 / REQ-EVALITEM-004-S1 — store 계층 mutation guard가 SQL 미실행 후 반환)
// 도메인 수준 거부 — pgx FK 에러가 아님 (DC-009.1 검증 대상).
var ErrEvalItemHierarchyImmutable = errors.New("evaluation item hierarchy is immutable: children exist")

// ErrEvalItemInvalidStatus status 열거 외 값 또는 NULL 전이 시도 (REQ-EVALITEM-004-U1)
// store 계층 사전 검증이 SQL 미실행 후 반환 (DB CHECK와 이중 방어).
var ErrEvalItemInvalidStatus = errors.New("evaluation item invalid status value")

// ErrScoreNotFound 요청한 점수 ID가 존재하지 않음 (SPEC-AX-SCORE-001)
// GetScoreByID는 pgx.ErrNoRows 대신 이 센티널을 래핑하여 반환한다.
var ErrScoreNotFound = errors.New("score not found")

// ErrScoreInvalidInput 점수 입력 검증 실패 (evaluation_item_id blank/64자 초과,
// score_value 누락, evidence_id 비-UUID — REQ-SCORE-001-U1). store 계층이 SQL 미실행 후 반환.
var ErrScoreInvalidInput = errors.New("score invalid input")

// ErrScoreImmutable CONFIRMED 행의 score_value/weight/grade 변경 시도 (D4)
// store 계층 mutation guard가 SQL 미실행 후 반환.
var ErrScoreImmutable = errors.New("score is immutable: confirmed score fields cannot be changed")

// ErrScoreInvalidStatus status 열거 외 값 또는 허용되지 않은 전이 시도 (D4)
var ErrScoreInvalidStatus = errors.New("score invalid status transition")

// ErrGradeThresholdsUnavailable 요청한 scope의 grade_thresholds 행이 0개 (D3)
// DetermineGrade는 등급을 fabricate하지 않고 이 에러를 반환 (SEC-04 fail-closed).
var ErrGradeThresholdsUnavailable = errors.New("grade thresholds unavailable for scope")

// ErrScoreAuditWriteFailed InsertScore/UpdateScore가 동일 TX 내 audit_logs INSERT에
// 실패했을 때 반환 (DC-UBI-002 / REQ-SCORE-001-E1). 호출자가 errors.Is로 식별하여
// 트랜잭션을 Rollback하면 scores/audit_logs 양쪽이 취소된다 (양방향 원자성, DC-004-U1).
var ErrScoreAuditWriteFailed = errors.New("score audit write failed")

// ErrScoreNotConfirmed CONFIRMED 정정(SupersedeAndReplaceScore)을 CONFIRMED 아닌 행에
// 시도했을 때 반환 (D4 — 정정 대상은 반드시 CONFIRMED여야 함).
var ErrScoreNotConfirmed = errors.New("score is not in CONFIRMED status; supersede requires CONFIRMED")
