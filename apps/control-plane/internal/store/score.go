// score.go — 점수(Score) 도메인 pgx 기반 ScoreTx 구현
// SPEC-AX-SCORE-001: PgEvalItemTx 패턴을 미러링한 점수 트랜잭션 구현
// D1: scores 테이블 단일 + level discriminator ∈{raw,item,category}
// D4: DRAFT 가변, CONFIRMED score-field 불변, CONFIRMED→SUPERSEDED 후 신규 INSERT 패턴
// D2: audit resource_id = scores.id 직접 대입 (surrogate 해시 및 namespace 상수 금지)
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// maxScoreEvalItemIDLen evaluation_item_id 최대 길이 (DDL VARCHAR(64) 정합 — REQ-SCORE-001-U1)
const maxScoreEvalItemIDLen = 64

// allowedScoreLevel D1 discriminator 허용 값
var allowedScoreLevel = map[string]struct{}{
	"raw":      {},
	"item":     {},
	"category": {},
}

// allowedScoreStatus D4 state-machine 허용 상태
var allowedScoreStatus = map[string]struct{}{
	"DRAFT":      {},
	"CONFIRMED":  {},
	"SUPERSEDED": {},
}

// confirmedImmutableFields CONFIRMED 행 변경 금지 필드 이름 목록 (에러 메시지용)
// D4: score_value/weight/grade는 CONFIRMED 행에서 불변
var confirmedImmutableFields = []string{"score_value", "weight", "grade"}

// PgScoreTx pgx.Tx 래퍼 — ScoreTx 인터페이스 구현
// 단일 PostgreSQL 트랜잭션 내에서 모든 점수 쓰기/조회 연산을 수행
// PgEvalItemTx와 동일한 트랜잭션 원자성 패턴을 미러링한다.
// recorder를 보유하여 InsertScore/UpdateScore가 entity-INSERT 직후 동일 tx에
// audit-INSERT를 수행한다 (PgScoreTx 자신이 audit.AuditTx를 구현 — 동일 pgx.Tx).
//
// @MX:WARN: [AUTO] InsertScore/UpdateScore 내 entity-INSERT 후 recorder.RecordScore* 실패 시
//
//	호출자가 Commit하면 안 됨 — deferred Rollback이 entity+audit 양방향 취소
//
// @MX:REASON: DC-UBI-002 — InsertScore→RecordScoreCreated, UpdateScore→RecordScoreUpdated가
//
//	동일 t.tx에서 원자적으로 실행됨 (recorder 와이어링 실재 — phantom 아님). 순서
//	(entity-INSERT → audit-INSERT → 호출자 Commit) 변경 시 양방향 원자성 붕괴.
type PgScoreTx struct {
	// tx 래핑된 pgx 트랜잭션
	tx pgx.Tx
	// logger 구조화 로그
	logger *zap.Logger
	// recorder 점수 감사 이벤트 기록기 — 동일 tx에 audit_logs 1건 INSERT (DC-UBI-002)
	recorder *audit.Recorder
}

// validateScoreInput InsertScore pre-INSERT 검증 — SQL 미실행 후 거부 (REQ-SCORE-001-U1)
//  1. evaluation_item_id blank/64자 초과 (VARCHAR(64) 정합)
//  2. level 열거 외 값 (D1 raw|item|category)
//  3. score_value 누락/비수치: NaN/±Inf는 DECIMAL(6,2) 표현 불가 → 거부
//     (DC-001-U1 Case C — "missing"은 Go float64에서 NaN sentinel로 매핑됨)
//  4. evidence_id: scores.evidence_id는 UUID NULL FK-less stub. *uuid.UUID 시그니처가
//     "비-UUID evidence_id"를 경계(handler/CLI의 uuid.Parse)에서 차단하므로
//     store 계층 형식 재검증은 불필요 (DC-001-U1 Case D — 타입 설계상 unreachable,
//     phantom 검증을 추가하지 않는 것이 정직한 모델).
func validateScoreInput(evaluationItemID, level string, scoreValue float64) error {
	if strings.TrimSpace(evaluationItemID) == "" {
		return fmt.Errorf("evaluation_item_id가 비어 있음: %w", stderrors.ErrScoreInvalidInput)
	}
	if len(evaluationItemID) > maxScoreEvalItemIDLen {
		return fmt.Errorf("evaluation_item_id가 %d자를 초과함 (len=%d): %w",
			maxScoreEvalItemIDLen, len(evaluationItemID), stderrors.ErrScoreInvalidInput)
	}
	if _, ok := allowedScoreLevel[level]; !ok {
		return fmt.Errorf("level=%q 허용 외 값 (raw|item|category): %w",
			level, stderrors.ErrScoreInvalidInput)
	}
	// DC-001-U1 Case C: NaN/±Inf score_value는 DECIMAL(6,2) 컬럼에 표현 불가.
	// SQL 미실행 후 fail-closed 거부 (경영평가 점수 무결성 — 비수치 점수 금지).
	if math.IsNaN(scoreValue) || math.IsInf(scoreValue, 0) {
		return fmt.Errorf("score_value가 비수치(NaN/Inf)임: %w", stderrors.ErrScoreInvalidInput)
	}
	return nil
}

// InsertScore scores 테이블에 새 행을 삽입하고 생성된 UUID를 반환
// 검증 실패 시 SQL 미실행 후 ErrScoreInvalidInput 래핑 반환 (REQ-SCORE-001-U1)
// SQL은 $N placeholder만 사용 (SEC-01)
// 성공 시 동일 t.tx에 SCORE_CREATED audit 1건을 기록 (DC-UBI-002.1 — recorder 실 호출).
// audit-INSERT 실패 시 ErrScoreAuditWriteFailed를 래핑하여 반환하고, 호출자가
// Rollback하면 scores/audit_logs 양방향 취소된다 (DC-004-U1 — entity+audit 동일 TX).
//
// @MX:ANCHOR: [AUTO] 점수 생성 단일 진입점 — 핸들러/통합 테스트/recorder 등 3곳 이상 호출
// @MX:REASON: InsertScore → recorder.RecordScoreCreated(동일 t.tx) 원자성 계약 실재
//
//	(REQ-SCORE-UBI-002/REQ-SCORE-001-E1) — phantom 아님, DC-UBI-002 통합 단언으로 검증
func (t *PgScoreTx) InsertScore(
	ctx context.Context,
	evaluationItemID string,
	evidenceID *uuid.UUID,
	level string,
	scoreValue float64,
	weight *float64,
	metadata map[string]any,
) (uuid.UUID, error) {
	if err := validateScoreInput(evaluationItemID, level, scoreValue); err != nil {
		return uuid.Nil, err
	}

	id, err := t.insertScoreRow(ctx, evaluationItemID, evidenceID, level, scoreValue, weight, "DRAFT", metadata)
	if err != nil {
		return uuid.Nil, err
	}

	// DC-UBI-002.1: entity-INSERT 직후 동일 t.tx에 SCORE_CREATED audit 1건.
	// userID="" → resolveUserID가 'cli-anonymous' 반환 (authEnabled=false, DC-UBI-003).
	if auditErr := t.recorder.RecordScoreCreated(ctx, t, id, evaluationItemID, level, ""); auditErr != nil {
		t.logger.Error("InsertScore audit 기록 실패",
			zap.String("evaluation_item_id", evaluationItemID),
			zap.String("score_id", id.String()),
			zap.Error(auditErr),
		)
		return uuid.Nil, fmt.Errorf("InsertScore audit 기록 실패: %w: %w",
			stderrors.ErrScoreAuditWriteFailed, auditErr)
	}
	return id, nil
}

// insertScoreRow scores 1행을 INSERT하고 생성 UUID를 반환 (status 파라미터화).
// InsertScore(status='DRAFT')와 SupersedeAndReplaceScore(status='CONFIRMED')가 공유.
// SQL은 $N placeholder만 사용 (SEC-01 — 문자열 보간 금지). status는 호출부 하드코딩
// 리터럴만 전달 (사용자 입력 유래 아님 — D4 허용 상태).
func (t *PgScoreTx) insertScoreRow(
	ctx context.Context,
	evaluationItemID string,
	evidenceID *uuid.UUID,
	level string,
	scoreValue float64,
	weight *float64,
	status string,
	metadata map[string]any,
) (uuid.UUID, error) {
	const query = `
		INSERT INTO scores (
			evaluation_item_id, evidence_id, level,
			score_value, weight, status, metadata,
			created_at, created_by, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			now(), 'cli-anonymous', now()
		)
		RETURNING id
	`

	metaJSON, err := marshalScoreMetadata(metadata)
	if err != nil {
		return uuid.Nil, err
	}

	var id uuid.UUID
	err = t.tx.QueryRow(ctx, query,
		evaluationItemID, evidenceID, level,
		scoreValue, weight, status, metaJSON,
	).Scan(&id)
	if err != nil {
		if pgErr, ok := pgScoreErrorOf(err); ok {
			t.logger.Error("InsertScore 실패",
				zap.String("evaluation_item_id", evaluationItemID),
				zap.String("sqlstate", pgErr.Code),
				zap.Error(err),
			)
			return uuid.Nil, fmt.Errorf("InsertScore 실패 (SQLSTATE %s): %w", pgErr.Code, err)
		}
		t.logger.Error("InsertScore 실패",
			zap.String("evaluation_item_id", evaluationItemID),
			zap.Error(err),
		)
		return uuid.Nil, fmt.Errorf("InsertScore 실패: %w", err)
	}
	return id, nil
}

// GetScoreByID 점수 id로 단건을 조회
// 존재하지 않으면 stderrors.ErrScoreNotFound를 래핑하여 반환 (raw pgx.ErrNoRows 금지)
func (t *PgScoreTx) GetScoreByID(ctx context.Context, id uuid.UUID) (*Score, error) {
	const query = `
		SELECT id, evaluation_item_id, evidence_id, level,
		       score_value, weight, grade, status, metadata,
		       created_at, created_by, updated_at
		FROM scores
		WHERE id = $1
	`
	score, err := scanScoreRow(t.tx.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetScoreByID id=%s: %w", id, stderrors.ErrScoreNotFound)
		}
		return nil, fmt.Errorf("GetScoreByID scan 실패: %w", err)
	}
	return score, nil
}

// GetScoresByEvaluationItem 동일 evaluation_item_id의 점수 목록을 반환
// 결과 없으면 빈 슬라이스 (error 아님 — EvalItem DC-005.6 패턴 미러)
func (t *PgScoreTx) GetScoresByEvaluationItem(ctx context.Context, evaluationItemID string) ([]*Score, error) {
	const query = `
		SELECT id, evaluation_item_id, evidence_id, level,
		       score_value, weight, grade, status, metadata,
		       created_at, created_by, updated_at
		FROM scores
		WHERE evaluation_item_id = $1
		ORDER BY created_at DESC
	`
	rows, err := t.tx.Query(ctx, query, evaluationItemID)
	if err != nil {
		return nil, fmt.Errorf("GetScoresByEvaluationItem 실패: %w", err)
	}
	defer rows.Close()

	result := make([]*Score, 0)
	for rows.Next() {
		score, scanErr := scanScoreRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("GetScoresByEvaluationItem scan 실패: %w", scanErr)
		}
		result = append(result, score)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("GetScoresByEvaluationItem rows 에러: %w", err)
	}
	return result, nil
}

// UpdateScore 점수를 부분 갱신 (D4 state-machine 가드 포함)
// CONFIRMED 행의 score_value/weight/grade 변경 요청은 SQL 미실행 후 ErrScoreImmutable 반환 (D4)
// status 열거 외/허용되지 않은 전이는 ErrScoreInvalidStatus 반환 (D4)
// SET 절은 하드코딩 컬럼명만 사용 (SEC-02 — 동적 컬럼 주입 금지)
//
// @MX:WARN: [AUTO] CONFIRMED 불변 가드 — score_value/weight/grade 변경 시도 체크 순서 변경 금지
// @MX:REASON: D4 CONFIRMED 불변 계약 (REQ-SCORE-UBI-004) — 가드 누락 시 감사 추적성 붕괴
//
//	실 상태 변경 시 동일 t.tx에 SCORE_UPDATED audit 1건 (DC-UBI-002.2). no-op은 미기록.
func (t *PgScoreTx) UpdateScore(ctx context.Context, id uuid.UUID, upd ScoreUpdate) error {
	// 대상 존재 확인
	current, err := t.GetScoreByID(ctx, id)
	if err != nil {
		return err
	}

	// D4: CONFIRMED 행의 score-field 불변 가드
	if current.Status == "CONFIRMED" {
		if upd.ScoreValue != nil || upd.Weight != nil || upd.Grade != nil {
			return fmt.Errorf("UpdateScore id=%s: CONFIRMED 행 불변 필드(%s) 변경 거부: %w",
				id, strings.Join(confirmedImmutableFields, "/"), stderrors.ErrScoreImmutable)
		}
	}

	// status 전이 검증
	if upd.Status != nil {
		if txErr := validateScoreStatusTransition(current.Status, *upd.Status); txErr != nil {
			return txErr
		}
	}

	// SET 절 동적 구성 (컬럼명 하드코딩, 값만 $N 바인딩 — SEC-02)
	setClauses, args, err := buildScoreUpdateSet(upd)
	if err != nil {
		return err
	}
	if len(setClauses) == 0 {
		// 변경 요청 없음 — no-op (상태 무변경 → audit 미기록, DC-UBI-002)
		return nil
	}
	setClauses = append(setClauses, "updated_at = now()")

	argN := len(args) + 1
	query := "UPDATE scores SET " + joinComma(setClauses) +
		fmt.Sprintf(" WHERE id = $%d", argN)
	args = append(args, id)

	result, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		if pgErr, ok := pgScoreErrorOf(err); ok {
			return fmt.Errorf("UpdateScore 실패 (SQLSTATE %s): %w", pgErr.Code, err)
		}
		return fmt.Errorf("UpdateScore 실패: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("UpdateScore id=%s: %w", id, stderrors.ErrScoreNotFound)
	}

	// DC-UBI-002.2: 실 상태 변경 후 동일 t.tx에 SCORE_UPDATED audit 1건.
	if auditErr := t.recorder.RecordScoreUpdated(ctx, t, id, current.EvaluationItemID, current.Level, ""); auditErr != nil {
		t.logger.Error("UpdateScore audit 기록 실패",
			zap.String("score_id", id.String()),
			zap.Error(auditErr),
		)
		return fmt.Errorf("UpdateScore audit 기록 실패: %w: %w",
			stderrors.ErrScoreAuditWriteFailed, auditErr)
	}
	return nil
}

// SupersedeAndReplaceScore CONFIRMED 정정 (D4): 구 행이 CONFIRMED일 때만,
// 동일 ScoreTx 내에서 (1) 신규 행 INSERT(status=CONFIRMED, corrected 값) +
// (2) 구 행 status CONFIRMED→SUPERSEDED UPDATE 를 수행하고, 각 변경에 audit 1건
// (신: SCORE_CREATED, 구: SCORE_UPDATED)을 기록한다. 물리 DELETE 0건.
// 구 행이 CONFIRMED가 아니면 SQL 부작용 없이 ErrScoreNotConfirmed 반환 (D4).
// 모든 연산이 t.tx에서 일어나므로 어느 단계든 실패 시 호출자 Rollback이 전체 취소 (EC-ADD-1).
//
// @MX:WARN: [AUTO] 정정 4단계(신규 INSERT → 신 audit → 구 SUPERSEDED UPDATE → 구 audit)
//
//	순서 변경 금지 — 중간 실패 시 부분 상태가 남으면 안 됨 (전체 t.tx 원자성)
//
// @MX:REASON: DC-UBI-004 — 2 score 행(구 SUPERSEDED, 신 CONFIRMED) + 2 audit 행, 물리삭제 0.
//
//	CONFIRMED 불변 계약(D4)을 우회하지 않고 append-only 정정으로 감사 추적성 보존.
func (t *PgScoreTx) SupersedeAndReplaceScore(
	ctx context.Context,
	oldID uuid.UUID,
	newScoreValue float64,
	newWeight *float64,
	newMetadata map[string]any,
) (uuid.UUID, error) {
	if math.IsNaN(newScoreValue) || math.IsInf(newScoreValue, 0) {
		return uuid.Nil, fmt.Errorf("정정 score_value가 비수치(NaN/Inf)임: %w",
			stderrors.ErrScoreInvalidInput)
	}

	// 구 행 조회 + CONFIRMED 검증 (정정 대상은 반드시 CONFIRMED — D4)
	old, err := t.GetScoreByID(ctx, oldID)
	if err != nil {
		return uuid.Nil, err
	}
	if old.Status != "CONFIRMED" {
		return uuid.Nil, fmt.Errorf(
			"SupersedeAndReplaceScore id=%s: 현재 status=%s, 정정은 CONFIRMED 행만 허용: %w",
			oldID, old.Status, stderrors.ErrScoreNotConfirmed)
	}

	// (1) 신규 행 INSERT — 구 행의 evaluation_item_id/evidence_id/level 승계, 상태 CONFIRMED.
	newID, err := t.insertScoreRow(ctx, old.EvaluationItemID, old.EvidenceID, old.Level,
		newScoreValue, newWeight, "CONFIRMED", newMetadata)
	if err != nil {
		return uuid.Nil, err
	}
	// (2) 신규 행 audit (SCORE_CREATED) — 동일 t.tx
	if auditErr := t.recorder.RecordScoreCreated(ctx, t, newID, old.EvaluationItemID, old.Level, ""); auditErr != nil {
		return uuid.Nil, fmt.Errorf("정정 신규 행 audit 실패: %w: %w",
			stderrors.ErrScoreAuditWriteFailed, auditErr)
	}

	// (3) 구 행 CONFIRMED→SUPERSEDED UPDATE — 물리 DELETE 아님 (append-only)
	const supersedeQuery = `
		UPDATE scores SET status='SUPERSEDED', updated_at=now()
		WHERE id=$1 AND status='CONFIRMED'
	`
	res, err := t.tx.Exec(ctx, supersedeQuery, oldID)
	if err != nil {
		if pgErr, ok := pgScoreErrorOf(err); ok {
			return uuid.Nil, fmt.Errorf("구 행 SUPERSEDED 전이 실패 (SQLSTATE %s): %w", pgErr.Code, err)
		}
		return uuid.Nil, fmt.Errorf("구 행 SUPERSEDED 전이 실패: %w", err)
	}
	if res.RowsAffected() == 0 {
		// 동시 정정 등으로 구 행이 더 이상 CONFIRMED가 아님 — 전체 TX 무효화 유도
		return uuid.Nil, fmt.Errorf(
			"SupersedeAndReplaceScore id=%s: 구 행 CONFIRMED 아님(동시 변경?): %w",
			oldID, stderrors.ErrScoreNotConfirmed)
	}
	// (4) 구 행 audit (SCORE_UPDATED) — 동일 t.tx
	if auditErr := t.recorder.RecordScoreUpdated(ctx, t, oldID, old.EvaluationItemID, old.Level, ""); auditErr != nil {
		return uuid.Nil, fmt.Errorf("정정 구 행 audit 실패: %w: %w",
			stderrors.ErrScoreAuditWriteFailed, auditErr)
	}
	return newID, nil
}

// validateScoreStatusTransition 현재 status에서 요청 status로의 전이가 허용되는지 확인
// 허용 전이: DRAFT→CONFIRMED, DRAFT→SUPERSEDED, CONFIRMED→SUPERSEDED
// 금지: SUPERSEDED에서 어떤 상태로도 전이 불가, 열거 외 값 거부
func validateScoreStatusTransition(current, next string) error {
	if _, ok := allowedScoreStatus[next]; !ok {
		return fmt.Errorf("status=%q 허용 외 값: %w", next, stderrors.ErrScoreInvalidStatus)
	}
	// 허용 전이 화이트리스트
	allowed := map[string]map[string]struct{}{
		"DRAFT": {
			"CONFIRMED":  {},
			"SUPERSEDED": {},
		},
		"CONFIRMED": {
			"SUPERSEDED": {},
		},
		"SUPERSEDED": {},
	}
	if _, ok := allowed[current][next]; !ok {
		return fmt.Errorf("status 전이 %s→%s 허용되지 않음: %w",
			current, next, stderrors.ErrScoreInvalidStatus)
	}
	return nil
}

// buildScoreUpdateSet upd의 non-nil 필드에 대해 SET 절 fragment와 $N 바인딩 인자를 생성
// [HARD] SEC-02: 컬럼명은 이 함수 내 하드코딩 리터럴만 사용 (사용자 입력 유래 동적 컬럼 주입 금지)
func buildScoreUpdateSet(upd ScoreUpdate) (setClauses []string, args []any, err error) {
	setClauses = make([]string, 0, 6)
	args = make([]any, 0, 7)
	add := func(clause string, val any) {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", clause, len(args)+1))
		args = append(args, val)
	}

	if upd.ScoreValue != nil {
		add("score_value", *upd.ScoreValue)
	}
	if upd.Weight != nil {
		add("weight", *upd.Weight)
	}
	if upd.Grade != nil {
		add("grade", *upd.Grade)
	}
	if upd.Status != nil {
		add("status", *upd.Status)
	}
	if upd.Metadata != nil {
		metaJSON, mErr := marshalScoreMetadata(*upd.Metadata)
		if mErr != nil {
			return nil, nil, mErr
		}
		add("metadata", metaJSON)
	}
	return setClauses, args, nil
}

// SumWeightedByEvaluationItem raw-level 자식 행의 가중합 Σ(score_value × weight) 반환
// NULL-weight policy: exclude — weight가 NULL인 행은 합산에서 제외 (GAP-01 결정적 정책)
//
// @MX:WARN: [AUTO] SEC-03: 집계 경로에 float64 사용 금지 — pgtype.Numeric로 정확 십진 반환
// @MX:REASON: DECIMAL(6,2)/DECIMAL(5,4)를 Go float64로 읽으면 0.1+0.2≠0.3 식 부동소수점
//
//	오차 축적 (SEC-03). DB SUM을 numeric(12,4) 캐스트 후 pgtype.Numeric로 스캔하여
//	float64 변환 없이 정확 십진 텍스트(Value())로 비교 가능 — 경영평가 점수 무결성.
//
// NULL-weight policy: exclude — weight IS NOT NULL 필터링으로 결정적 결과 보장 (GAP-01)
func (t *PgScoreTx) SumWeightedByEvaluationItem(ctx context.Context, evaluationItemID string) (pgtype.Numeric, error) {
	// SEC-03: 집계를 DB에 위임 + numeric(12,4) 캐스트로 고정 스케일 — Go float64 미경유.
	// NULL-weight policy: weight IS NOT NULL AND score_value IS NOT NULL 행만 집계 (GAP-01)
	// SUM은 DB가 DECIMAL 정밀도로 계산. numeric(12,4) 캐스트로 고정 스케일.
	// 빈 결과 시 COALESCE가 0 반환 — 정확 값(0)은 스케일 표기와 무관 (호출자 정확 비교).
	const query = `
		SELECT COALESCE(SUM(score_value * weight), 0)::numeric(12,4)
		FROM scores
		WHERE evaluation_item_id = $1
		  AND level = 'raw'
		  AND weight IS NOT NULL
		  AND score_value IS NOT NULL
	`
	var sum pgtype.Numeric
	if err := t.tx.QueryRow(ctx, query, evaluationItemID).Scan(&sum); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("SumWeightedByEvaluationItem 실패: %w", err)
	}
	return sum, nil
}

// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입
// D2: resource_id = scores.id UUID 직접 대입 — surrogate 해시 미사용
// PgEvalItemTx.InsertAuditLog와 동일 패턴 — Recorder가 동일 TX 원자성을 위해 호출
func (t *PgScoreTx) InsertAuditLog(ctx context.Context, e *audit.Event) error {
	const query = `
		INSERT INTO audit_logs (id, action, resource_type, resource_id, user_id, details, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	id := uuid.New()

	var details interface{}
	if len(e.DetailsJSON) > 0 {
		details = e.DetailsJSON
	}

	_, err := t.tx.Exec(ctx, query,
		id,
		string(e.Action),
		e.ResourceType,
		e.ResourceID,
		e.UserID,
		details,
		e.Timestamp,
	)
	if err != nil {
		t.logger.Error("InsertAuditLog(score) 실패",
			zap.String("action", string(e.Action)),
			zap.String("resource_id", e.ResourceID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("InsertAuditLog(score) 실패: %w", err)
	}
	return nil
}

// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
func (t *PgScoreTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("Commit(score) 실패: %w", err)
	}
	return nil
}

// Rollback 현재 트랜잭션을 롤백 — Commit 후 호출 시 pgx가 무시(no-op)
func (t *PgScoreTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return nil
		}
		return fmt.Errorf("Rollback(score) 실패: %w", err)
	}
	return nil
}

// marshalScoreMetadata metadata map을 JSONB 바이트로 직렬화 (nil/빈 맵이면 NULL)
func marshalScoreMetadata(metadata map[string]any) (interface{}, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("score metadata 직렬화 실패: %w", err)
	}
	return b, nil
}

// scanScoreRow pgx.Row/pgx.Rows 공통 스캔 헬퍼 (DAMP — 컬럼 순서 단일 정의)
func scanScoreRow(row pgx.Row) (*Score, error) {
	var (
		score      Score
		scoreValue *float64
		weight     *float64
		grade      *string
		evidenceID *uuid.UUID
		metaRaw    []byte
	)
	if err := row.Scan(
		&score.ID, &score.EvaluationItemID, &evidenceID, &score.Level,
		&scoreValue, &weight, &grade, &score.Status, &metaRaw,
		&score.CreatedAt, &score.CreatedBy, &score.UpdatedAt,
	); err != nil {
		return nil, err
	}
	score.EvidenceID = evidenceID
	score.ScoreValue = scoreValue
	score.Weight = weight
	if grade != nil {
		score.Grade = *grade
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &score.Metadata) //nolint:errcheck // 손상된 메타데이터는 nil로 graceful
	}
	return &score, nil
}

// pgScoreErrorOf err 체인에서 *pgconn.PgError를 추출 (errors.As 래핑 호환)
func pgScoreErrorOf(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr, true
	}
	return nil, false
}

// gradeThreshold 등급 임계값 행 (grade_thresholds 테이블 1행 대응)
type gradeThreshold struct {
	Letter       string
	BoundaryRule string
	MinValue     float64
}

// DetermineGrade score에 대한 등급 문자를 grade_thresholds 테이블로부터 결정적으로 산출한다.
// D3: scope 파라미터로 해당 scope 행 전체 조회 → S→D 내림차순 min_value 스캔,
// boundary_rule gte(≥)/gt(>) 기준 최초 일치 등급 반환.
// scope 행 0건 → ErrGradeThresholdsUnavailable (등급 fabricate 금지, fail-closed).
//
// @MX:WARN: [AUTO] 등급 fabricate 금지 — scope 0행 시 반드시 에러 반환 (fail-closed, SEC-04)
// @MX:REASON: D3 fail-closed 정책 — 임계값 미설정 시 임의 등급 반환은 경영평가 신뢰성 붕괴 (REQ-SCORE-003-U1)
func (t *PgScoreTx) DetermineGrade(ctx context.Context, scope string, score float64) (string, error) {
	// S→D 내림차순 min_value 정렬 (높은 임계값부터 매칭 — 첫 일치가 등급)
	const query = `
		SELECT letter, min_value, boundary_rule
		FROM grade_thresholds
		WHERE scope = $1
		ORDER BY min_value DESC
	`
	rows, err := t.tx.Query(ctx, query, scope)
	if err != nil {
		return "", fmt.Errorf("DetermineGrade 조회 실패: %w", err)
	}
	defer rows.Close()

	thresholds := make([]gradeThreshold, 0, 5)
	for rows.Next() {
		var gt gradeThreshold
		if scanErr := rows.Scan(&gt.Letter, &gt.MinValue, &gt.BoundaryRule); scanErr != nil {
			return "", fmt.Errorf("DetermineGrade scan 실패: %w", scanErr)
		}
		thresholds = append(thresholds, gt)
	}
	if err = rows.Err(); err != nil {
		return "", fmt.Errorf("DetermineGrade rows 에러: %w", err)
	}

	// scope 행 0건 → 구조화 에러 (등급 fabricate 0 — D3 fail-closed)
	if len(thresholds) == 0 {
		return "", fmt.Errorf("DetermineGrade scope=%q: %w", scope, stderrors.ErrGradeThresholdsUnavailable)
	}

	// S→D 내림차순 스캔: 첫 일치 등급 반환
	for _, gt := range thresholds {
		var matches bool
		switch gt.BoundaryRule {
		case "gte":
			matches = score >= gt.MinValue
		case "gt":
			matches = score > gt.MinValue
		}
		if matches {
			return gt.Letter, nil
		}
	}

	// 최저 임계값 미달 — D(가장 낮은 등급) 반환이 아니라 마지막 threshold가 catch-all
	// 설계상 가장 낮은 min_value(D)가 0이어야 하므로 정상적으로 항상 일치해야 함
	// 미일치 시 구조화 에러 (fabricate 금지)
	return "", fmt.Errorf("DetermineGrade scope=%q score=%.2f: 모든 임계값 미달 — %w",
		scope, score, stderrors.ErrGradeThresholdsUnavailable)
}
