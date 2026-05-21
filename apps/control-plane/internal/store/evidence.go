// evidence.go — 증빙(Evidence) 도메인 엔티티 + pgx 기반 EvidenceTx 구현
// SPEC-AX-EVID-001: PgWorkflowTx 패턴을 미러링한 증빙 트랜잭션 구현
// database_blob 전략: file_content BYTEA가 evidence 행 + audit 행과 동일 pgx TX에 저장됨
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// Evidence 증빙 도메인 엔티티 — evidences 테이블 1행에 대응
// 필드 순서: map(8B ptr) → time.Time(24B) 블록 → 포인터/문자열(16B/8B) → int64(8B) → int(8B)
// (golangci-lint fieldalignment govet 분석기 정합 — 큰→작은 순)
type Evidence struct {
	// Metadata 임의 메타데이터 (JSONB) — nil 허용
	Metadata map[string]string
	// CreatedAt 생성 시각 (TIMESTAMPTZ)
	CreatedAt time.Time
	// UpdatedAt 갱신 시각 (TIMESTAMPTZ)
	UpdatedAt time.Time
	// PreviousVersionID 직전 버전 ID (version=1이면 nil)
	PreviousVersionID *uuid.UUID
	// EvaluationItemID 경량 평가항목 stub (FK 없음)
	EvaluationItemID string
	// FileName 원본 파일명
	FileName string
	// FileHashSHA256 서버가 계산한 SHA-256 (64자 소문자 hex)
	FileHashSHA256 string
	// ContentType MIME 타입
	ContentType string
	// StorageStrategy 'filesystem'|'database_blob'|'minio'
	StorageStrategy string
	// StorageLocation database_blob: 'db://evidences/<id>'; 기타: 외부 위치
	StorageLocation string
	// Status 'ACTIVE'|'SUPERSEDED'
	Status string
	// CreatedBy 생성자 (audit.DefaultUserID 정합, 기본 'cli-anonymous')
	CreatedBy string
	// ID 증빙 UUID (PK)
	ID uuid.UUID
	// FileSizeBytes 파일 바이트 수
	FileSizeBytes int64
	// Version 버전 번호 (1부터 시작)
	Version int
}

// PgEvidenceTx pgx.Tx 래퍼 — EvidenceTx 인터페이스 구현
// 단일 PostgreSQL 트랜잭션 내에서 모든 증빙 쓰기/조회 연산을 수행
// PgWorkflowTx와 동일한 트랜잭션 원자성 패턴을 미러링한다.
//
// @MX:WARN: [AUTO] InsertEvidence/InsertAuditLog 사이 panic/early-return 시 orphan 증빙 행 누출
// @MX:REASON: BeginEvidenceTx 직후 defer Rollback 등록 필수 — Commit 전 모든 경로가 단일 TX 내 (research.md §8 Risk 3)
type PgEvidenceTx struct {
	// tx 래핑된 pgx 트랜잭션
	tx pgx.Tx
	// logger 구조화 로그
	logger *zap.Logger
}

// InsertEvidence evidences 테이블에 새 증빙 행을 삽입하고 생성된 UUID를 반환
// database_blob 전략 시 fileContent는 동일 TX의 BYTEA 컬럼에 저장된다.
// SQL은 $N placeholder만 사용 (SEC-04 SQL injection 방지 — 문자열 보간 금지)
func (t *PgEvidenceTx) InsertEvidence(
	ctx context.Context,
	evalItemID, fileName, contentType string,
	fileSizeBytes int64,
	fileHashSHA256 string,
	storageStrategy, storageLocation string,
	metadata map[string]string,
	fileContent []byte,
	previousVersionID *uuid.UUID,
) (uuid.UUID, error) {
	const query = `
		INSERT INTO evidences (
			id, evaluation_item_id, version, previous_version_id,
			file_name, file_size_bytes, file_hash_sha256, content_type,
			file_content, storage_location, storage_strategy, status,
			metadata, created_at, created_by, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, 'ACTIVE',
			$12, now(), 'cli-anonymous', now()
		)
		RETURNING id
	`

	id := uuid.New()

	// 버전 결정: previousVersionID가 nil이면 version=1, 아니면 직전+1
	version := 1
	if previousVersionID != nil {
		prev, err := t.GetEvidenceByID(ctx, *previousVersionID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("InsertEvidence 직전 버전 조회 실패: %w", err)
		}
		version = prev.Version + 1
	}

	// metadata JSONB 직렬화 (nil이면 NULL)
	var metaJSON interface{}
	if len(metadata) > 0 {
		b, err := json.Marshal(metadata)
		if err != nil {
			return uuid.Nil, fmt.Errorf("InsertEvidence metadata 직렬화 실패: %w", err)
		}
		metaJSON = b
	}

	var returned uuid.UUID
	err := t.tx.QueryRow(ctx, query,
		id, evalItemID, version, previousVersionID,
		fileName, fileSizeBytes, fileHashSHA256, contentType,
		fileContent, storageLocation, storageStrategy,
		metaJSON,
	).Scan(&returned)
	if err != nil {
		if pgErr, ok := pgErrorOf(err); ok {
			t.logger.Error("InsertEvidence 실패",
				zap.String("evaluation_item_id", evalItemID),
				zap.String("sqlstate", pgErr.Code),
				zap.Error(err),
			)
			return uuid.Nil, fmt.Errorf("InsertEvidence 실패 (SQLSTATE %s): %w", pgErr.Code, err)
		}
		t.logger.Error("InsertEvidence 실패", zap.String("evaluation_item_id", evalItemID), zap.Error(err))
		return uuid.Nil, fmt.Errorf("InsertEvidence 실패: %w", err)
	}
	return returned, nil
}

// GetEvidenceByID 증빙 ID로 단건을 조회
// 존재하지 않으면 stderrors.ErrEvidenceNotFound를 래핑하여 반환 (GAP-03 / DC-012)
func (t *PgEvidenceTx) GetEvidenceByID(ctx context.Context, id uuid.UUID) (*Evidence, error) {
	const query = `
		SELECT id, evaluation_item_id, version, previous_version_id,
		       file_name, file_size_bytes, file_hash_sha256, content_type,
		       storage_location, storage_strategy, status, metadata,
		       created_at, created_by, updated_at
		FROM evidences
		WHERE id = $1
	`
	return t.scanOne(ctx, t.tx.QueryRow(ctx, query, id), id.String())
}

// GetLatestVersionByEvalItem 동일 evaluation_item_id의 최신 행을 SELECT ... FOR UPDATE로 조회
// 동시 재업로드 직렬화(REQ-EVID-001-S1) — 행 잠금으로 중복 version 방지
// 기존 증빙이 없으면 (nil, nil) 반환 (신규 증빙 경로)
//
// @MX:WARN: [AUTO] SELECT FOR UPDATE 락 미보유 시 동일 evaluation_item_id 동시 재업로드가 중복 version/orphan 체인 생성
// @MX:REASON: FOR UPDATE 절이 동시 트랜잭션을 직렬화 — 절 제거/완화 금지 (REQ-EVID-001-S1, pg_store.go:99 패턴)
func (t *PgEvidenceTx) GetLatestVersionByEvalItem(ctx context.Context, evalItemID string) (*Evidence, error) {
	const query = `
		SELECT id, evaluation_item_id, version, previous_version_id,
		       file_name, file_size_bytes, file_hash_sha256, content_type,
		       storage_location, storage_strategy, status, metadata,
		       created_at, created_by, updated_at
		FROM evidences
		WHERE evaluation_item_id = $1
		ORDER BY version DESC
		LIMIT 1
		FOR UPDATE
	`
	ev, err := t.scanOne(ctx, t.tx.QueryRow(ctx, query, evalItemID), evalItemID)
	if err != nil {
		if errors.Is(err, stderrors.ErrEvidenceNotFound) {
			// 신규 증빙 — 기존 버전 없음 (정상 경로)
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	return ev, nil
}

// ListEvidenceByEvalItem 동일 evaluation_item_id의 모든 버전을 version DESC로 반환
func (t *PgEvidenceTx) ListEvidenceByEvalItem(ctx context.Context, evalItemID string) ([]*Evidence, error) {
	const query = `
		SELECT id, evaluation_item_id, version, previous_version_id,
		       file_name, file_size_bytes, file_hash_sha256, content_type,
		       storage_location, storage_strategy, status, metadata,
		       created_at, created_by, updated_at
		FROM evidences
		WHERE evaluation_item_id = $1
		ORDER BY version DESC
	`
	rows, err := t.tx.Query(ctx, query, evalItemID)
	if err != nil {
		return nil, fmt.Errorf("ListEvidenceByEvalItem 실패: %w", err)
	}
	defer rows.Close()

	result := make([]*Evidence, 0)
	for rows.Next() {
		ev, scanErr := scanEvidenceRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("ListEvidenceByEvalItem scan 실패: %w", scanErr)
		}
		result = append(result, ev)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ListEvidenceByEvalItem rows 에러: %w", err)
	}
	return result, nil
}

// MarkSuperseded 직전 버전 행의 status를 ACTIVE → SUPERSEDED로 전이
// 본문 컬럼은 절대 변경하지 않으며 status만 전이 (REQ-EVID-UBI-004, store 계층 소유 — GAP-04)
func (t *PgEvidenceTx) MarkSuperseded(ctx context.Context, id uuid.UUID) error {
	const query = `
		UPDATE evidences
		SET status = 'SUPERSEDED', updated_at = now()
		WHERE id = $1 AND status = 'ACTIVE'
	`
	result, err := t.tx.Exec(ctx, query, id)
	if err != nil {
		t.logger.Error("MarkSuperseded 실패", zap.String("id", id.String()), zap.Error(err))
		return fmt.Errorf("MarkSuperseded 실패: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("MarkSuperseded id=%s: %w", id, stderrors.ErrEvidenceNotFound)
	}
	return nil
}

// allowedBodyColumns mutation guard가 보호하는 본문 컬럼 화이트리스트
// SQL injection 방지: 컬럼명은 사용자 입력이 아닌 이 상수 집합에서만 선택 (SEC-04)
var allowedBodyColumns = map[string]struct{}{
	"file_name":        {},
	"file_size_bytes":  {},
	"file_hash_sha256": {},
	"content_type":     {},
	"storage_location": {},
	"file_content":     {},
	"metadata":         {},
}

// UpdateEvidenceBodyColumn 증빙 본문 컬럼 변경을 시도하되,
// successor(previous_version_id=id 행)가 존재하면 SQL을 실행하지 않고 거부한다.
// (REQ-EVID-UBI-004 / REQ-EVID-002-U1 store 계층 mutation guard — DC-013 item 3)
//
// @MX:NOTE: [AUTO] 컬럼명은 allowedBodyColumns 화이트리스트에서만 선택 — 동적 SQL 식별자 주입 방지
func (t *PgEvidenceTx) UpdateEvidenceBodyColumn(ctx context.Context, id uuid.UUID, column, value string) error {
	if _, ok := allowedBodyColumns[column]; !ok {
		return fmt.Errorf("UpdateEvidenceBodyColumn: 허용되지 않은 컬럼 %q", column)
	}

	// successor 존재 여부 확인 — 존재하면 SQL 미실행 후 거부 (불변식)
	var successorCount int
	if err := t.tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM evidences WHERE previous_version_id = $1`, id,
	).Scan(&successorCount); err != nil {
		return fmt.Errorf("UpdateEvidenceBodyColumn successor 확인 실패: %w", err)
	}
	if successorCount > 0 {
		// SQL UPDATE를 실행하지 않고 거부 (REQ-EVID-UBI-004)
		return fmt.Errorf("UpdateEvidenceBodyColumn id=%s column=%s: %w",
			id, column, stderrors.ErrEvidenceImmutable)
	}

	// successor 없음 — 화이트리스트 컬럼만 $N 파라미터로 갱신 (식별자만 화이트리스트 보간)
	query := "UPDATE evidences SET " + column + " = $2, updated_at = now() WHERE id = $1"
	result, err := t.tx.Exec(ctx, query, id, value)
	if err != nil {
		return fmt.Errorf("UpdateEvidenceBodyColumn 실패: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("UpdateEvidenceBodyColumn id=%s: %w", id, stderrors.ErrEvidenceNotFound)
	}
	return nil
}

// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입
// PgWorkflowTx.InsertAuditLog와 동일 패턴 — Recorder가 동일 TX 원자성을 위해 호출
func (t *PgEvidenceTx) InsertAuditLog(ctx context.Context, e *audit.Event) error {
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
		t.logger.Error("InsertAuditLog(evidence) 실패",
			zap.String("action", string(e.Action)),
			zap.String("resource_id", e.ResourceID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("InsertAuditLog(evidence) 실패: %w", err)
	}
	return nil
}

// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
func (t *PgEvidenceTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("Commit(evidence) 실패: %w", err)
	}
	return nil
}

// Rollback 현재 트랜잭션을 롤백 — Commit 후 호출 시 pgx가 무시(no-op)
func (t *PgEvidenceTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return nil
		}
		return fmt.Errorf("Rollback(evidence) 실패: %w", err)
	}
	return nil
}

// scanOne 단일 행 QueryRow 결과를 Evidence로 스캔
// pgx.ErrNoRows는 stderrors.ErrEvidenceNotFound로 래핑 (GAP-03)
func (t *PgEvidenceTx) scanOne(_ context.Context, row pgx.Row, key string) (*Evidence, error) {
	ev, err := scanEvidenceRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("GetEvidence key=%s: %w", key, stderrors.ErrEvidenceNotFound)
		}
		return nil, fmt.Errorf("GetEvidence scan 실패: %w", err)
	}
	return ev, nil
}

// scanEvidenceRow pgx.Row/pgx.Rows 공통 스캔 헬퍼 (DAMP — 컬럼 순서 단일 정의)
func scanEvidenceRow(row pgx.Row) (*Evidence, error) {
	var (
		ev       Evidence
		prevID   *uuid.UUID
		metaRaw  []byte
		sizeNull *int64
		hashNull *string
		ctypeNul *string
		locNull  *string
	)
	if err := row.Scan(
		&ev.ID, &ev.EvaluationItemID, &ev.Version, &prevID,
		&ev.FileName, &sizeNull, &hashNull, &ctypeNul,
		&locNull, &ev.StorageStrategy, &ev.Status, &metaRaw,
		&ev.CreatedAt, &ev.CreatedBy, &ev.UpdatedAt,
	); err != nil {
		return nil, err
	}
	ev.PreviousVersionID = prevID
	if sizeNull != nil {
		ev.FileSizeBytes = *sizeNull
	}
	if hashNull != nil {
		ev.FileHashSHA256 = *hashNull
	}
	if ctypeNul != nil {
		ev.ContentType = *ctypeNul
	}
	if locNull != nil {
		ev.StorageLocation = *locNull
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &ev.Metadata) //nolint:errcheck // 손상된 메타데이터는 nil로 graceful
	}
	return &ev, nil
}

// pgErrorOf err 체인에서 *pgconn.PgError를 추출 (errors.As 래핑 호환)
func pgErrorOf(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr, true
	}
	return nil, false
}
