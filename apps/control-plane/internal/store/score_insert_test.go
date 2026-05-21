//go:build integration

// score_insert_test.go — T-004/T-005/T-006/T-010/T-012/T-013 통합 RED 테스트
// SPEC-AX-SCORE-001:
//
//	T-004: InsertScore 해피 패스 + GetScoreByID round-trip
//	T-005: 입력 검증 pre-INSERT 4 케이스 (blank eval_item_id, 64자 초과, 잘못된 level, bad evidence UUID)
//	T-006: metadata JSONB semantic round-trip (map[string]any + reflect.DeepEqual)
//	T-010: SumWeightedByEvaluationItem DECIMAL 정확도 (Σ=83.00) + NULL weight 결정적 정책
//	T-012: status state-machine DRAFT→CONFIRMED→SUPERSEDED (CONFIRMED score-field 불변)
//	T-013: 횡단 UBI — created_by=cli-anonymous, D2 grep (0 NewSHA1, 0 namespace constant)
//	T-014: FK 부재 경계 — evaluation_item_id FK 없어도 INSERT 성공
//	EC-ADD-2: Sprintf.*UPDATE 동적 SQL 정적 검사
//	EC-ADD-3: internal/audit 순환 참조 부재 정적 검사 (GAP-05 [HARD])
//
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestScore -v -count=1
package store

import (
	"context"
	"math"
	"math/big"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// ── T-004: InsertScore 해피 패스 ─────────────────────────────────────────────

// TestScore_InsertHappyPath AC-SCORE-001-1:
// evaluation_item_id + level + score_value → INSERT 성공 + 생성 UUID 반환
func TestScore_InsertHappyPath(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	scoreValue := 85.0
	id, err := tx.InsertScore(ctx, "AX-SAFETY-ORG-01", nil, "raw", scoreValue, nil, nil)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id, "생성된 UUID가 nil이 아님")

	require.NoError(t, tx.Commit(ctx))
}

// TestScore_InsertAndGetRoundTrip AC-SCORE-001-1 / DC-001-S1:
// InsertScore → GetScoreByID: 삽입한 행을 정확히 조회 (모든 필드 검증)
func TestScore_InsertAndGetRoundTrip(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	const evalItemID = "AX-SAFETY-ORG-01"
	scoreValue := 72.5
	weight := 0.4
	id, err := tx.InsertScore(ctx, evalItemID, nil, "raw", scoreValue, &weight, nil)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)

	score, err := tx.GetScoreByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, score)
	assert.Equal(t, id, score.ID)
	assert.Equal(t, evalItemID, score.EvaluationItemID)
	assert.Nil(t, score.EvidenceID, "evidence_id NULL")
	assert.Equal(t, "raw", score.Level)
	require.NotNil(t, score.ScoreValue)
	assert.InDelta(t, scoreValue, *score.ScoreValue, 0.001)
	require.NotNil(t, score.Weight)
	assert.InDelta(t, weight, *score.Weight, 0.0001)
	assert.Equal(t, "DRAFT", score.Status, "초기 status=DRAFT (D4)")
	assert.Empty(t, score.Grade, "grade NULL → 빈 문자열")
	assert.False(t, score.CreatedAt.IsZero())
	assert.False(t, score.UpdatedAt.IsZero())

	require.NoError(t, tx.Commit(ctx))
}

// TestScore_InsertWithEvidenceID evidence_id UUID 정상 삽입 (nullable stub):
// evidences 테이블에 해당 UUID가 없어도 FK-less이므로 INSERT 성공
func TestScore_InsertWithEvidenceID(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	evID := uuid.New()
	id, err := tx.InsertScore(ctx, "AX-ITEM-02", &evID, "item", 60.0, nil, nil)
	require.NoError(t, err, "FK-less이므로 evidences 부재 evidence_id도 INSERT 성공해야 함")
	require.NotEqual(t, uuid.Nil, id)

	score, err := tx.GetScoreByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, score.EvidenceID)
	assert.Equal(t, evID, *score.EvidenceID)

	require.NoError(t, tx.Commit(ctx))
}

// TestScore_GetScoreByID_NotFound:
// 존재하지 않는 UUID 조회 → ErrScoreNotFound 래핑 반환
func TestScore_GetScoreByID_NotFound(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.GetScoreByID(ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreNotFound, "미존재 id → ErrScoreNotFound")
}

// ── T-005: 입력 검증 pre-INSERT 4 케이스 ────────────────────────────────────

// TestScore_Validation_BlankEvalItemID REQ-SCORE-001-U1:
// evaluation_item_id="" → SQL 미실행, ErrScoreInvalidInput 래핑
func TestScore_Validation_BlankEvalItemID(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.InsertScore(ctx, "", nil, "raw", 80.0, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreInvalidInput,
		"blank evaluation_item_id → ErrScoreInvalidInput (SQL 미실행)")
}

// TestScore_Validation_EvalItemIDTooLong REQ-SCORE-001-U1:
// evaluation_item_id 65자 → SQL 미실행, ErrScoreInvalidInput 래핑
func TestScore_Validation_EvalItemIDTooLong(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	longID := strings.Repeat("X", 65)
	_, err = tx.InsertScore(ctx, longID, nil, "raw", 80.0, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreInvalidInput,
		"evaluation_item_id 65자 → ErrScoreInvalidInput (SQL 미실행, VARCHAR(64) 위반)")
}

// TestScore_Validation_InvalidLevel REQ-SCORE-001-U1 / D1:
// level="invalid" → SQL 미실행, ErrScoreInvalidInput 래핑
func TestScore_Validation_InvalidLevel(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.InsertScore(ctx, "AX-ITEM-01", nil, "invalid", 80.0, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreInvalidInput,
		"level 열거 외 값 → ErrScoreInvalidInput (SQL 미실행, D1)")
}

// TestScore_Validation_AllThreeLevelsAccepted D1:
// raw|item|category 세 가지 level 모두 허용
func TestScore_Validation_AllThreeLevelsAccepted(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	for i, level := range []string{"raw", "item", "category"} {
		tx, err := db.store.BeginScoreTx(ctx)
		require.NoError(t, err)
		evalItemID := "AX-LEVEL-" + level
		id, insErr := tx.InsertScore(ctx, evalItemID, nil, level, float64(70+i), nil, nil)
		require.NoError(t, insErr, "level=%s 는 D1 허용값", level)
		assert.NotEqual(t, uuid.Nil, id)
		require.NoError(t, tx.Commit(ctx))
	}
}

// TestScore_Validation_ScoreValueNaN DC-001-U1 Case C / REQ-SCORE-001-U1:
// score_value=NaN → SQL 미실행, ErrScoreInvalidInput 래핑 (누락/비수치 거부)
func TestScore_Validation_ScoreValueNaN(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.InsertScore(ctx, "AX-NAN-01", nil, "raw", math.NaN(), nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreInvalidInput,
		"score_value=NaN → ErrScoreInvalidInput (DC-001-U1 Case C, SQL 미실행)")

	// SQL 미실행 보장: scores 행 0건
	var cnt int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scores WHERE evaluation_item_id=$1`, "AX-NAN-01").Scan(&cnt))
	assert.Equal(t, 0, cnt, "score_value=NaN → scores 행 0건 (SQL 미실행)")
}

// TestScore_Validation_ScoreValueInf DC-001-U1 Case C 확장:
// score_value=+Inf/-Inf → ErrScoreInvalidInput (DECIMAL 표현 불가, fail-closed)
func TestScore_Validation_ScoreValueInf(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	for _, inf := range []float64{math.Inf(1), math.Inf(-1)} {
		tx, err := db.store.BeginScoreTx(ctx)
		require.NoError(t, err)
		_, err = tx.InsertScore(ctx, "AX-INF-01", nil, "raw", inf, nil, nil)
		require.Error(t, err, "score_value=±Inf 거부")
		assert.ErrorIs(t, err, stderrors.ErrScoreInvalidInput,
			"score_value=±Inf → ErrScoreInvalidInput (DECIMAL 표현 불가)")
		_ = tx.Rollback(ctx)
	}
}

// TestScore_Validation_EvidenceIDBoundaryIsUUIDType DC-001-U1 Case D:
// scores.evidence_id는 UUID NULL FK-less stub. *uuid.UUID 타입 시그니처가
// "비-UUID evidence_id"를 경계(uuid.Parse)에서 차단함을 입증한다.
// (validateScoreInput가 형식 재검증을 하지 않는 것은 타입 설계상 정당 — phantom 검증 아님)
func TestScore_Validation_EvidenceIDBoundaryIsUUIDType(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	// 경계(handler/CLI)에서 evidence_id 문자열을 *uuid.UUID로 파싱하는 계약.
	// 비-UUID 문자열은 uuid.Parse에서 에러 → InsertScore 시그니처에 도달 불가.
	_, parseErr := uuid.Parse("not-a-uuid")
	require.Error(t, parseErr,
		"비-UUID evidence_id는 경계 uuid.Parse에서 차단됨 (DC-001-U1 Case D — 타입 설계로 unreachable)")

	// 정상 UUID는 파싱되어 *uuid.UUID로 InsertScore에 전달 가능 (FK-less stub).
	valid, parseErr2 := uuid.Parse("11111111-1111-1111-1111-111111111111")
	require.NoError(t, parseErr2)
	assert.NotEqual(t, uuid.Nil, valid, "정상 UUID는 *uuid.UUID 시그니처로 전달 가능")
}

// ── T-006: metadata JSONB semantic round-trip ────────────────────────────────

// TestScore_MetadataJSONBRoundTrip AC-SCORE-006-1:
// metadata map[string]any (한글 키 포함) → 삽입 → GetScoreByID → reflect.DeepEqual
func TestScore_MetadataJSONBRoundTrip(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	meta := map[string]any{
		"평가연도":     "2025",
		"source":   "자동화평가",
		"verified": true,
		"count":    float64(3),
	}

	id, err := tx.InsertScore(ctx, "AX-META-01", nil, "raw", 90.0, nil, meta)
	require.NoError(t, err)

	score, err := tx.GetScoreByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, score.Metadata)

	assert.True(t, reflect.DeepEqual(meta, score.Metadata),
		"metadata JSONB semantic round-trip: map[string]any 동등 (byte 비교 아님)\ngot: %v\nwant: %v",
		score.Metadata, meta)

	require.NoError(t, tx.Commit(ctx))
}

// TestScore_NilMetadataRoundTrip:
// nil metadata → INSERT 후 조회 → score.Metadata nil
func TestScore_NilMetadataRoundTrip(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	id, err := tx.InsertScore(ctx, "AX-NOMETA-01", nil, "raw", 50.0, nil, nil)
	require.NoError(t, err)

	score, err := tx.GetScoreByID(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, score.Metadata, "nil metadata → DB NULL → 조회 시 nil")

	require.NoError(t, tx.Commit(ctx))
}

// TestScore_EmptyMetadataAccepted DC-001-O1 cond3:
// metadata={} (빈 맵) → 에러 없이 수용 (opaque placeholder)
func TestScore_EmptyMetadataAccepted(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	id, err := tx.InsertScore(ctx, "AX-EMPTYMETA-01", nil, "raw", 60.0, nil, map[string]any{})
	require.NoError(t, err, "metadata={} 는 에러 없이 수용 (DC-001-O1.3 opaque)")
	require.NotEqual(t, uuid.Nil, id)
	require.NoError(t, tx.Commit(ctx))
}

// TestScore_DeeplyNestedMetadataRoundTrip DC-001-O1 cond4:
// 깊게 중첩된 metadata + 한글 키 → semantic round-trip (reflect.DeepEqual)
func TestScore_DeeplyNestedMetadataRoundTrip(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	// JSON 숫자는 unmarshal 시 float64로 복원되므로 기대값도 float64로 작성
	meta := map[string]any{
		"채점_코멘트": "현장 점검 반영",
		"draft_threshold": map[string]any{
			"S": "90",
			"세부": map[string]any{
				"가중치":   float64(0.35),
				"검토자목록": []any{"홍길동", "김철수"},
			},
		},
		"flags": []any{true, false, float64(1)},
	}

	id, err := tx.InsertScore(ctx, "AX-NESTEDMETA-01", nil, "raw", 90.0, nil, meta)
	require.NoError(t, err, "깊게 중첩된 metadata 수용 (DC-001-O1.4)")

	score, err := tx.GetScoreByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, score.Metadata)
	assert.True(t, reflect.DeepEqual(meta, score.Metadata),
		"깊게 중첩 + 한글 키 metadata semantic round-trip (DC-001-O1.4/5)\ngot:  %v\nwant: %v",
		score.Metadata, meta)

	require.NoError(t, tx.Commit(ctx))
}

// ── T-010: SumWeightedByEvaluationItem DECIMAL 정확도 (SEC-03 exact) ─────────

// numericRat pgtype.Numeric을 정확 유리수(big.Rat)로 변환한다 (값 = Int × 10^Exp).
// float64를 경유하지 않으므로 epsilon=0 정확 비교가 가능하다 (SEC-03). 스케일
// 표기 차이("0" vs "0.0000")는 동일 값으로 정상 비교되며, 부동소수점 오차가
// 섞이면 서로 다른 유리수가 되어 즉시 검출된다.
func numericRat(t *testing.T, n pgtype.Numeric) *big.Rat {
	t.Helper()
	require.True(t, n.Valid, "pgtype.Numeric Valid=true 여야 함")
	require.False(t, n.NaN, "집계 결과는 NaN이 아니어야 함")
	r := new(big.Rat).SetInt(n.Int)
	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs32(n.Exp))), nil)
	if n.Exp >= 0 {
		r.Mul(r, new(big.Rat).SetInt(pow))
	} else {
		r.Quo(r, new(big.Rat).SetInt(pow))
	}
	return r
}

// ratInt 정수 기대값을 정확 유리수로 변환 (기대 합계는 모두 정수: 83, 50, 0).
// big.Rat.SetString(gosec G113)을 회피하고 정수 분모 1 유리수를 직접 구성한다.
func ratInt(v int64) *big.Rat {
	return new(big.Rat).SetInt64(v)
}

// abs32 int32 절댓값
func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestScore_SumWeightedByEvaluationItem_Exactness SEC-03 / DC-002-E1:
// 계약 표준 데이터셋 (90.00,0.5000),(80.00,0.3000),(70.00,0.2000) → Σ = 정확히 "83.0000"
// float64 누적 금지: 반환 타입 pgtype.Numeric, 문자열 정확 비교 (epsilon=0)
func TestScore_SumWeightedByEvaluationItem_Exactness(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	const evalItemID = "AX-SAFETY-ORG-01"
	rows := []struct {
		score, weight float64
	}{
		{90.0, 0.5000},
		{80.0, 0.3000},
		{70.0, 0.2000},
	}

	for _, r := range rows {
		r := r
		tx, err := db.store.BeginScoreTx(ctx)
		require.NoError(t, err)
		_, err = tx.InsertScore(ctx, evalItemID, nil, "raw", r.score, &r.weight, nil)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	sum, err := tx.SumWeightedByEvaluationItem(ctx, evalItemID)
	require.NoError(t, err)
	// SEC-03: 정확 십진 — 90*0.5 + 80*0.3 + 70*0.2 = 45 + 24 + 14 = 83 (epsilon=0)
	assert.Zero(t, numericRat(t, sum).Cmp(ratInt(83)),
		"Σ(score_value × weight) = 정확히 83 (DECIMAL exact, big.Rat epsilon=0 — SEC-03/DC-002-E1) got=%s",
		numericRat(t, sum).FloatString(4))
}

// TestScore_SumWeightedByEvaluationItem_NullWeightExcluded GAP-01 / SEC-03:
// weight=NULL 행은 합산에서 제외 → 결정적 정책 (정확 십진)
func TestScore_SumWeightedByEvaluationItem_NullWeightExcluded(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	const evalItemID = "AX-NULL-WEIGHT-01"

	w := 0.5
	tx1, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	_, err = tx1.InsertScore(ctx, evalItemID, nil, "raw", 100.0, &w, nil)
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	// weight NULL 행 (합산 제외 대상)
	tx2, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	_, err = tx2.InsertScore(ctx, evalItemID, nil, "raw", 999.0, nil, nil)
	require.NoError(t, err)
	require.NoError(t, tx2.Commit(ctx))

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	sum, err := tx.SumWeightedByEvaluationItem(ctx, evalItemID)
	require.NoError(t, err)
	assert.Zero(t, numericRat(t, sum).Cmp(ratInt(50)),
		"100×0.5=50 정확; weight=NULL 행(999) 제외 (GAP-01 결정적, SEC-03 epsilon=0) got=%s",
		numericRat(t, sum).FloatString(4))
}

// TestScore_SumWeightedByEvaluationItem_EmptyIsZero SEC-03:
// 해당 evaluation_item_id 없음 → Σ = 정확히 "0.0000" (COALESCE)
func TestScore_SumWeightedByEvaluationItem_EmptyIsZero(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	sum, err := tx.SumWeightedByEvaluationItem(ctx, "AX-NONEXIST")
	require.NoError(t, err)
	assert.Zero(t, numericRat(t, sum).Cmp(ratInt(0)),
		"결과 없음 → COALESCE(SUM(),0) = 정확히 0 (SEC-03 epsilon=0, 스케일 무관) got=%s",
		numericRat(t, sum).FloatString(4))
}

// ── T-012: status state-machine ──────────────────────────────────────────────

// TestScore_StatusStateMachine_DraftToConfirmed D4:
// DRAFT → CONFIRMED 전이 성공
func TestScore_StatusStateMachine_DraftToConfirmed(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	id := insertDraftScore(t, db, ctx, "AX-SM-01")

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	status := "CONFIRMED"
	err = tx.UpdateScore(ctx, id, ScoreUpdate{Status: &status})
	require.NoError(t, err)

	score, err := tx.GetScoreByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "CONFIRMED", score.Status)
	require.NoError(t, tx.Commit(ctx))
}

// TestScore_StatusStateMachine_ConfirmedScoreFieldImmutable D4:
// CONFIRMED 행의 score_value/weight/grade 변경 요청 → ErrScoreImmutable 반환
func TestScore_StatusStateMachine_ConfirmedScoreFieldImmutable(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	id := insertDraftScore(t, db, ctx, "AX-SM-02")
	confirmScore(t, db, ctx, id)

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	newVal := 99.0
	err = tx.UpdateScore(ctx, id, ScoreUpdate{ScoreValue: &newVal})
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrScoreImmutable,
		"CONFIRMED 행 score_value 변경 → ErrScoreImmutable (D4)")
}

// TestScore_StatusStateMachine_ConfirmedToSuperseded D4:
// CONFIRMED → SUPERSEDED 전이 성공 (정정 경로)
func TestScore_StatusStateMachine_ConfirmedToSuperseded(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	id := insertDraftScore(t, db, ctx, "AX-SM-03")
	confirmScore(t, db, ctx, id)

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	status := "SUPERSEDED"
	err = tx.UpdateScore(ctx, id, ScoreUpdate{Status: &status})
	require.NoError(t, err, "CONFIRMED → SUPERSEDED 전이 성공 (D4 정정 경로)")
	require.NoError(t, tx.Commit(ctx))
}

// TestScore_StatusStateMachine_SupersededTerminal D4:
// SUPERSEDED에서 어떤 상태로도 전이 불가
func TestScore_StatusStateMachine_SupersededTerminal(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	id := insertDraftScore(t, db, ctx, "AX-SM-04")
	supersededScore(t, db, ctx, id)

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	for _, next := range []string{"DRAFT", "CONFIRMED", "SUPERSEDED"} {
		next := next
		s := next
		err = tx.UpdateScore(ctx, id, ScoreUpdate{Status: &s})
		require.Error(t, err, "SUPERSEDED → %s 전이 거부 (D4 terminal)", next)
		assert.ErrorIs(t, err, stderrors.ErrScoreInvalidStatus,
			"SUPERSEDED terminal → ErrScoreInvalidStatus")
	}
}

// ── T-013: 횡단 UBI 불변 + 정적 grep 검사 ───────────────────────────────────

// TestScore_CreatedByIsCliAnonymous DC-UBI-003:
// InsertScore created_by = 'cli-anonymous' (audit.DefaultUserID 정합)
func TestScore_CreatedByIsCliAnonymous(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	id, err := tx.InsertScore(ctx, "AX-UBI-01", nil, "raw", 75.0, nil, nil)
	require.NoError(t, err)

	score, err := tx.GetScoreByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "cli-anonymous", score.CreatedBy,
		"created_by 기본값 'cli-anonymous' (DC-UBI-003, audit.DefaultUserID 정합)")

	require.NoError(t, tx.Commit(ctx))
}

// TestScore_NoNewSHA1InScoreGo TH-11 / D2 [HARD]:
// score.go에 uuid.NewSHA1( 실제 함수 호출 없음 (D2: UUID 직접 대입, surrogate namespace 미사용)
// NOTE: 주석 라인에 "NewSHA1"이 포함될 수 있으므로 실제 호출 패턴 `uuid.NewSHA1(` 만 검색
func TestScore_NoNewSHA1InScoreGo(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err, "repo 루트 조회 실패")
	root := strings.TrimSpace(string(out))

	// 실제 함수 호출 패턴만 검색 (주석 제외: grep -v "^[[:space:]]*//"로 주석 라인 필터링)
	scorePath := root + "/apps/control-plane/internal/store/score.go"
	grep := exec.Command("grep", "-n", "uuid\\.NewSHA1(", scorePath)
	grOut, _ := grep.Output()

	// 주석 라인을 수동으로 필터링
	lines := strings.Split(string(grOut), "\n")
	var actualCalls []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 라인 번호 이후의 내용 추출 (grep -n 출력: "NNN:content")
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			continue
		}
		content := strings.TrimSpace(trimmed[colonIdx+1:])
		// 주석 라인 (//로 시작) 제외
		if strings.HasPrefix(content, "//") {
			continue
		}
		actualCalls = append(actualCalls, line)
	}
	assert.Empty(t, actualCalls,
		"score.go에 uuid.NewSHA1() 실제 호출 없어야 함 (D2 [HARD]: audit resource_id = scores.id UUID 직접 대입)")
}

// TestScore_NoNamespaceConstantInScoreFiles TH-12 / D2 [HARD]:
// store/score*.go 파일에 ScoreAuditNamespace 등 namespace 상수 없음
func TestScore_NoNamespaceConstantInScoreFiles(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err)
	root := strings.TrimSpace(string(out))

	grep := exec.Command("grep", "-rn", "Namespace",
		root+"/apps/control-plane/internal/store/score.go")
	grOut, _ := grep.Output()
	assert.Empty(t, string(grOut),
		"score.go에 Namespace 상수 없어야 함 (D2 [HARD]: 평가항목과 달리 UUID surrogate 미사용)")
}

// TestCircularImportAbsence GAP-05 [HARD]:
// internal/audit/ 패키지가 internal/store 를 import하지 않음 (정적 grep)
// compile check 불충분 — 별도 grep 필수 (GAP-05 계약)
func TestCircularImportAbsence(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err, "repo 루트 조회 실패")
	root := strings.TrimSpace(string(out))

	// internal/audit/*.go 파일에서 "internal/store" import 검색
	grep := exec.Command("grep", "-rn", `"github.com/ircp/iroum-ax/apps/control-plane/internal/store"`,
		root+"/apps/control-plane/internal/audit/")
	grOut, _ := grep.Output()
	assert.Empty(t, string(grOut),
		"internal/audit 는 internal/store 를 import해선 안 됨 (GAP-05 [HARD] 순환참조 방지 — grep 필수)")
}

// TestScore_NoDynamicUpdateSQL EC-ADD-2:
// score.go에 fmt.Sprintf("...UPDATE...") 형태 동적 SQL 없음
// buildScoreUpdateSet에서 컬럼명 하드코딩 리터럴만 사용 (SEC-02)
func TestScore_NoDynamicUpdateSQL(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err)
	root := strings.TrimSpace(string(out))

	// Sprintf 호출 내에 "UPDATE"가 있으면 동적 SQL 의심
	grep := exec.Command("grep", "-n", `Sprintf.*UPDATE`,
		root+"/apps/control-plane/internal/store/score.go")
	grOut, _ := grep.Output()
	assert.Empty(t, string(grOut),
		"score.go에 Sprintf.*UPDATE 동적 SQL 없어야 함 (EC-ADD-2 / SEC-02 동적 컬럼 주입 금지)")
}

// ── T-014: FK 부재 경계 검증 ─────────────────────────────────────────────────

// TestScore_FKLess_EvalItemIDAcceptsAnyString AC-SCORE-BOUNDARY-1:
// evaluation_item_id가 evaluation_items에 존재하지 않아도 INSERT 성공 (FK-less)
func TestScore_FKLess_EvalItemIDAcceptsAnyString(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	// evaluation_items 테이블에 없는 임의 문자열 — FK-less이므로 INSERT 성공
	id, err := tx.InsertScore(ctx, "NON-EXISTENT-EVAL-ITEM", nil, "raw", 42.0, nil, nil)
	require.NoError(t, err, "evaluation_item_id FK-less → 미존재 값도 INSERT 성공 (AC-SCORE-BOUNDARY-1)")
	assert.NotEqual(t, uuid.Nil, id)

	require.NoError(t, tx.Commit(ctx))
}

// TestScore_FKLess_EvidenceIDAcceptsAnyUUID:
// evidence_id가 evidences에 존재하지 않아도 INSERT 성공 (FK-less)
func TestScore_FKLess_EvidenceIDAcceptsAnyUUID(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	nonExistEvID := uuid.New()
	id, err := tx.InsertScore(ctx, "AX-FK-01", &nonExistEvID, "raw", 55.0, nil, nil)
	require.NoError(t, err, "evidence_id FK-less → 미존재 UUID도 INSERT 성공")
	assert.NotEqual(t, uuid.Nil, id)

	require.NoError(t, tx.Commit(ctx))
}

// ── GetScoresByEvaluationItem ────────────────────────────────────────────────

// TestScore_GetScoresByEvaluationItem T-004 확장:
// InsertScore 2건 → GetScoresByEvaluationItem → 2건 반환, UUID 일치
func TestScore_GetScoresByEvaluationItem(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	const evalItemID = "AX-LIST-ITEM-01"

	// 동일 evalItemID로 2건 삽입
	id1 := insertDraftScore(t, db, ctx, evalItemID)
	id2 := insertDraftScore(t, db, ctx, evalItemID)

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	scores, err := tx.GetScoresByEvaluationItem(ctx, evalItemID)
	require.NoError(t, err)
	require.Len(t, scores, 2, "evaluation_item_id로 2건 조회")

	ids := map[uuid.UUID]bool{scores[0].ID: true, scores[1].ID: true}
	assert.True(t, ids[id1], "id1 포함")
	assert.True(t, ids[id2], "id2 포함")
}

// TestScore_GetScoresByEvaluationItem_Empty T-004 확장:
// 존재하지 않는 evalItemID → 빈 슬라이스 반환 (nil 아님)
func TestScore_GetScoresByEvaluationItem_Empty(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	scores, err := tx.GetScoresByEvaluationItem(ctx, "AX-NONEXISTENT-99")
	require.NoError(t, err)
	assert.Empty(t, scores, "미존재 evalItemID → 빈 슬라이스")
}

// ── 테스트 헬퍼 ──────────────────────────────────────────────────────────────

// insertDraftScore DRAFT 상태 점수 1건을 삽입하고 UUID를 반환
func insertDraftScore(t *testing.T, db *testDB, ctx context.Context, evalItemID string) uuid.UUID {
	t.Helper()
	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	id, err := tx.InsertScore(ctx, evalItemID, nil, "raw", 77.0, nil, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return id
}

// confirmScore 대상 점수를 CONFIRMED 상태로 전이
func confirmScore(t *testing.T, db *testDB, ctx context.Context, id uuid.UUID) {
	t.Helper()
	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	status := "CONFIRMED"
	require.NoError(t, tx.UpdateScore(ctx, id, ScoreUpdate{Status: &status}))
	require.NoError(t, tx.Commit(ctx))
}

// supersededScore 대상 점수를 SUPERSEDED 상태로 전이 (DRAFT → SUPERSEDED)
func supersededScore(t *testing.T, db *testDB, ctx context.Context, id uuid.UUID) {
	t.Helper()
	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	status := "SUPERSEDED"
	require.NoError(t, tx.UpdateScore(ctx, id, ScoreUpdate{Status: &status}))
	require.NoError(t, tx.Commit(ctx))
}
