// store.go — 워크플로우 영속성 인터페이스
// Sprint 1 RED: 인터페이스 정의만 포함, 실제 pgx 구현은 Sprint 3에서 담당
// 주의: postgres.go의 Store struct와 이름 충돌을 피하기 위해 WorkflowStore로 명명
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

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

// EvalItemStore 평가항목 taxonomy 영속성 최상위 인터페이스 (SPEC-AX-EVAL-ITEM-001 REQ-EVALITEM-001)
// EvidenceStore 패턴을 미러링하여 BeginEvalItemTx 트랜잭션 진입점만 제공한다.
// 실제 DB 접근은 EvalItemTx를 통해서만 수행 (raw SQL 누출 방지).
//
// @MX:ANCHOR: [AUTO] eval_item store 구현, recorder 연계, 향후 계층 조회 caller 등 3개 이상 호출자 예정
// @MX:REASON: 기존 EvidenceStore와 동일 — 평가항목 도메인 단일 DB 접근 계약. 실 PgWorkflowStore.pool 재사용(R-EVALITEM-005)
type EvalItemStore interface {
	// BeginEvalItemTx 새로운 평가항목 트랜잭션을 시작하여 반환
	// 반환된 EvalItemTx는 반드시 Commit 또는 Rollback 중 하나로 종료해야 함
	BeginEvalItemTx(ctx context.Context) (EvalItemTx, error)
}

// EvalItem 평가항목 도메인 엔티티 — evaluation_items 테이블 1행에 대응
// 자기참조 adjacency list (Option A): root는 ParentID=nil, 자식은 부모 id를 ParentID로 보유
// 필드 순서: map(8B) → time.Time(24B) 블록 → 포인터(8B) → 문자열(16B) → int 포인터(8B)
// (golangci-lint fieldalignment govet 분석기 정합 — 큰→작은 순)
type EvalItem struct {
	// Metadata 임의 메타데이터 (JSONB opaque placeholder) — nil 허용, 본 SPEC 미해석
	Metadata map[string]any
	// CreatedAt 생성 시각 (TIMESTAMPTZ)
	CreatedAt time.Time
	// UpdatedAt 갱신 시각 (TIMESTAMPTZ)
	UpdatedAt time.Time
	// ParentID 부모 항목 id (root이면 nil) — 자기참조 FK
	ParentID *string
	// Level 계층 레벨 (1범주 2항목 3지표 4배점, informational) — nil 허용
	Level *int
	// Weight 가중치 0.0-1.0 — nil 허용
	Weight *float64
	// MaxScore 최대 점수 — nil 허용
	MaxScore *int
	// ID 계층 코드 PK (예: AX-SAFETY-ORG-01, VARCHAR(64) — UUID 아님)
	ID string
	// DisplayName 표시명 (NOT NULL)
	DisplayName string
	// Description 설명 — 빈 문자열 허용
	Description string
	// HierarchyCode 경로 인코딩 (NOT NULL UNIQUE) — AUD-1 surrogate 입력
	HierarchyCode string
	// Status 'ACTIVE'|'DEPRECATED'|'ARCHIVED'
	Status string
	// CreatedBy 생성자 (audit.DefaultUserID 정합, 기본 'cli-anonymous')
	CreatedBy string
}

// EvalItemUpdate UpdateEvalItem 부분 갱신 입력 (GAP-05 — option (a) nullable 필드)
// nil 포인터 = "변경 요청 없음", non-nil = "해당 값으로 변경 요청".
// mutation guard는 ParentID/Level가 non-nil일 때만 successor 검증 후 거부 판단한다.
type EvalItemUpdate struct {
	// ParentID non-nil이면 부모 변경 요청 (자식 보유 시 거부 — REQ-EVALITEM-UBI-004)
	ParentID **string
	// Level non-nil이면 레벨 변경 요청 (자식 보유 시 거부)
	Level *int
	// DisplayName non-nil이면 표시명 변경
	DisplayName *string
	// Description non-nil이면 설명 변경
	Description *string
	// Weight non-nil이면 가중치 변경
	Weight *float64
	// MaxScore non-nil이면 최대 점수 변경
	MaxScore *int
	// Status non-nil이면 status 전이 요청 (열거 외/NULL 거부 — REQ-EVALITEM-004-U1)
	Status *string
	// Metadata non-nil이면 metadata verbatim 갱신 (semantic round-trip)
	Metadata *map[string]any
}

// EvalItemTx 단일 데이터베이스 트랜잭션 내 평가항목 쓰기/조회 연산 인터페이스
// InsertEvalItem/UpdateEvalItem과 InsertAuditLog는 동일 트랜잭션 내에서 atomic하게 처리되어야 함
// (REQ-EVALITEM-UBI-002 / REQ-EVALITEM-003-U1 트랜잭션 원자성 불변 조건)
//
// @MX:ANCHOR: [AUTO] AC-EVALITEM-003-3 / AC-EVALITEM-UBI-002 원자성 계약의 핵심
// @MX:REASON: pgx 구현체(PgEvalItemTx) + 핸들러 TX orchestration 등 3곳 이상에서 사용 — 평가항목 원자성 단일 계약
type EvalItemTx interface {
	// InsertEvalItem evaluation_items 테이블에 새 행을 삽입하고 삽입된 id를 반환
	// parentID가 nil이면 루트(parent_id NULL), 아니면 사전 조회로 부모 존재를 검증
	// id/displayName/hierarchyCode blank·id 64자 초과·중복 PK는 SQL 미실행 후 거부
	InsertEvalItem(
		ctx context.Context,
		id string,
		parentID *string,
		displayName, description string,
		level *int,
		hierarchyCode string,
		weight *float64,
		maxScore *int,
		metadata map[string]any,
	) (string, error)
	// GetEvalItemByID 평가항목 id로 단건을 조회
	// 존재하지 않으면 errors.ErrEvalItemNotFound를 래핑하여 반환 (raw pgx.ErrNoRows 금지)
	GetEvalItemByID(ctx context.Context, id string) (*EvalItem, error)
	// GetEvalItemsByParentID 동일 parent_id를 가진 직계 자식 목록을 반환
	// 자식이 없으면 빈 슬라이스 반환 (error 아님 — GAP-02/DC-005.6)
	GetEvalItemsByParentID(ctx context.Context, parentID string) ([]*EvalItem, error)
	// UpdateEvalItem 평가항목을 부분 갱신
	// 자식 보유 항목의 parent_id/level 변경 요청은 SQL 미실행 후 거부 (REQ-EVALITEM-UBI-004)
	// status 열거 외/NULL 요청은 거부 (REQ-EVALITEM-004-U1)
	UpdateEvalItem(ctx context.Context, id string, upd EvalItemUpdate) error
	// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입 (Recorder가 호출)
	InsertAuditLog(ctx context.Context, e *audit.Event) error
	// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
	Commit(ctx context.Context) error
	// Rollback 현재 트랜잭션을 롤백 — defer로 호출하는 것이 안전, Commit 후 무시
	Rollback(ctx context.Context) error
}

// Score 점수 도메인 엔티티 — scores 테이블 1행에 대응 (SPEC-AX-SCORE-001)
// D1: level discriminator ∈{raw,item,category}, D4: status state-machine
type Score struct {
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Metadata         map[string]any
	EvidenceID       *uuid.UUID
	ScoreValue       *float64
	Weight           *float64
	EvaluationItemID string
	Level            string
	Grade            string
	Status           string
	CreatedBy        string
	ID               uuid.UUID
}

// ScoreUpdate UpdateScore 부분 갱신 입력
// nil 포인터 = "변경 요청 없음", non-nil = "해당 값으로 변경 요청".
// CONFIRMED 행의 score_value/weight/grade 변경은 store 계층에서 거부 (D4).
type ScoreUpdate struct {
	// ScoreValue non-nil이면 점수 변경 (CONFIRMED 행 거부)
	ScoreValue *float64
	// Weight non-nil이면 가중치 변경 (CONFIRMED 행 거부)
	Weight *float64
	// Grade non-nil이면 등급 변경 (CONFIRMED 행 거부)
	Grade *string
	// Status non-nil이면 status 전이 요청
	Status *string
	// Metadata non-nil이면 metadata verbatim 갱신
	Metadata *map[string]any
}

// ScoreStore 점수 영속성 최상위 인터페이스 (SPEC-AX-SCORE-001)
// EvalItemStore 패턴을 미러링하여 BeginScoreTx 트랜잭션 진입점만 제공한다.
//
// @MX:ANCHOR: [AUTO] 점수 도메인 단일 DB 접근 계약 — 핸들러/통합 테스트/recorder 등 3곳 이상 호출 예정
// @MX:REASON: 기존 EvalItemStore와 동일 — 단일 pool 싱글톤 재사용 계약. 신규 pgxpool 생성 금지.
type ScoreStore interface {
	// BeginScoreTx 새로운 점수 트랜잭션을 시작하여 반환
	BeginScoreTx(ctx context.Context) (ScoreTx, error)
}

// ScoreTx 단일 데이터베이스 트랜잭션 내 점수 쓰기/조회 연산 인터페이스
// InsertScore/UpdateScore와 InsertAuditLog는 동일 트랜잭션 내에서 atomic하게 처리되어야 함
// (REQ-SCORE-UBI-002 / REQ-SCORE-004-U1 트랜잭션 원자성 불변 조건)
//
// @MX:ANCHOR: [AUTO] AC-SCORE-001-1 / AC-SCORE-UBI-002 원자성 계약의 핵심
// @MX:REASON: pgx 구현체(PgScoreTx) + 핸들러 TX orchestration + recorder 등 3곳 이상에서 사용
type ScoreTx interface {
	// InsertScore scores 테이블에 새 행을 삽입하고 생성된 UUID를 반환
	InsertScore(
		ctx context.Context,
		evaluationItemID string,
		evidenceID *uuid.UUID,
		level string,
		scoreValue float64,
		weight *float64,
		metadata map[string]any,
	) (uuid.UUID, error)
	// GetScoreByID 점수 id로 단건을 조회
	GetScoreByID(ctx context.Context, id uuid.UUID) (*Score, error)
	// GetScoresByEvaluationItem 동일 evaluation_item_id의 점수 목록을 반환
	GetScoresByEvaluationItem(ctx context.Context, evaluationItemID string) ([]*Score, error)
	// UpdateScore 점수를 부분 갱신 (D4 state-machine 가드 포함)
	// 성공 시 동일 TX에 SCORE_UPDATED audit 1건을 기록한다 (DC-UBI-002.2).
	UpdateScore(ctx context.Context, id uuid.UUID, upd ScoreUpdate) error
	// SupersedeAndReplaceScore CONFIRMED 행 정정 (D4): 신규 행 INSERT(status=CONFIRMED) +
	// 구 행 CONFIRMED→SUPERSEDED, 동일 TX. 각 변경 1 audit row (신: SCORE_CREATED,
	// 구: SCORE_UPDATED). 물리 DELETE 0건. 구 행이 CONFIRMED가 아니면 ErrScoreNotConfirmed.
	SupersedeAndReplaceScore(
		ctx context.Context,
		oldID uuid.UUID,
		newScoreValue float64,
		newWeight *float64,
		newMetadata map[string]any,
	) (uuid.UUID, error)
	// SumWeightedByEvaluationItem raw-level 자식 행의 가중합 Σ(score_value × weight) 반환
	// SEC-03: float64 누적 금지 — pgtype.Numeric로 정확 십진 반환 (DB 사이드 집계).
	// NULL-weight policy: exclude (GAP-01 결정적 정책)
	SumWeightedByEvaluationItem(ctx context.Context, evaluationItemID string) (pgtype.Numeric, error)
	// DetermineGrade grade_thresholds 테이블로부터 score에 대한 등급 문자를 결정적으로 반환
	// scope 행 0건 → ErrGradeThresholdsUnavailable (등급 fabricate 금지 — D3 fail-closed)
	DetermineGrade(ctx context.Context, scope string, score float64) (string, error)
	// InsertAuditLog 현재 트랜잭션 내에 감사 이벤트를 삽입
	InsertAuditLog(ctx context.Context, e *audit.Event) error
	// Commit 현재 트랜잭션을 커밋하여 모든 변경사항을 영속화
	Commit(ctx context.Context) error
	// Rollback 현재 트랜잭션을 롤백 — defer로 호출하는 것이 안전, Commit 후 무시
	Rollback(ctx context.Context) error
}
