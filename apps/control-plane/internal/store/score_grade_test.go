//go:build integration

// score_grade_test.go — T-011: grade_thresholds 등급 산출 DetermineGrade 검증
// SPEC-AX-SCORE-001 D3:
//
//	scope 기반 letter↔min_value 매핑
//	boundary_rule gte(>=) / gt(>) 정책
//	S→D 내림차순 스캔: 첫 번째 일치 등급 반환
//	scope 0행 → ErrGradeThresholdsUnavailable (fail-closed, D3)
//	경계값 gte: score=min_value → 해당 등급
//	경계값 gt:  score=min_value → 미달, score=min_value+ε → 달성
//
// 실행: go test -tags=integration ./apps/control-plane/internal/store/ -run TestScoreGrade -v -count=1
package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	stderrors "github.com/ircp/iroum-ax/apps/control-plane/internal/errors"
)

// insertDefaultGradeThresholds 'default' scope 기준 등급 임계값 삽입:
//
//	S≥90, A≥80, B≥70, C≥60, D≥0 (gte 기본)
func insertDefaultGradeThresholds(t *testing.T, db *testDB) {
	t.Helper()
	ctx := context.Background()
	thresholds := []struct {
		letter   string
		minValue float64
	}{
		{"S", 90.0},
		{"A", 80.0},
		{"B", 70.0},
		{"C", 60.0},
		{"D", 0.0},
	}
	for _, th := range thresholds {
		_, err := db.pool.Exec(ctx,
			`INSERT INTO grade_thresholds (scope, letter, min_value, boundary_rule)
			 VALUES ('default', $1, $2, 'gte')
			 ON CONFLICT (scope, letter) DO UPDATE SET min_value=EXCLUDED.min_value`,
			th.letter, th.minValue,
		)
		require.NoError(t, err, "grade_thresholds(%s) 삽입 실패", th.letter)
	}
}

// insertGtGradeThresholds 'strict' scope — boundary_rule='gt' 테스트용
// S>90, A>80, B>70, C>60, D>0
func insertGtGradeThresholds(t *testing.T, db *testDB) {
	t.Helper()
	ctx := context.Background()
	thresholds := []struct {
		letter   string
		minValue float64
	}{
		{"S", 90.0},
		{"A", 80.0},
		{"B", 70.0},
		{"C", 60.0},
		{"D", 0.0},
	}
	for _, th := range thresholds {
		_, err := db.pool.Exec(ctx,
			`INSERT INTO grade_thresholds (scope, letter, min_value, boundary_rule)
			 VALUES ('strict', $1, $2, 'gt')
			 ON CONFLICT (scope, letter) DO UPDATE SET min_value=EXCLUDED.min_value`,
			th.letter, th.minValue,
		)
		require.NoError(t, err, "grade_thresholds(strict, %s) 삽입 실패", th.letter)
	}
}

// TestScoreGrade_DetermineGrade_HappyPath D3:
// 표준 'default' scope로 각 경계값 정확히 등급 반환
func TestScoreGrade_DetermineGrade_HappyPath(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	insertDefaultGradeThresholds(t, db)
	ctx := context.Background()

	cases := []struct {
		score    float64
		expected string
	}{
		{100.0, "S"},
		{90.0, "S"}, // gte: 90.0 >= 90 → S
		{89.9, "A"}, // 89.9 < 90 → A
		{80.0, "A"}, // gte: 80.0 >= 80 → A
		{79.9, "B"},
		{70.0, "B"}, // gte: 70.0 >= 70 → B
		{69.9, "C"},
		{60.0, "C"}, // gte: 60.0 >= 60 → C
		{59.9, "D"},
		{0.0, "D"}, // gte: 0.0 >= 0 → D
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("score=%.1f→%s", tc.score, tc.expected), func(t *testing.T) {
			tx, err := db.store.BeginScoreTx(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(ctx) }()

			grade, err := tx.DetermineGrade(ctx, "default", tc.score)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, grade,
				"score=%.1f → 등급=%s (D3 gte 경계값)", tc.score, tc.expected)
		})
	}
}

// TestScoreGrade_DetermineGrade_BoundaryRuleGt D3 boundary_rule='gt':
// 'strict' scope: score=min_value → 미달 (gt), score=min_value+0.01 → 달성
func TestScoreGrade_DetermineGrade_BoundaryRuleGt(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	insertGtGradeThresholds(t, db)
	ctx := context.Background()

	// S>90: score=90.0 → 미달 → A
	tx1, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	grade, err := tx1.DetermineGrade(ctx, "strict", 90.0)
	require.NoError(t, err)
	assert.Equal(t, "A", grade, "gt 정책: 90.0 > 90 false → A (경계값 미달)")
	_ = tx1.Rollback(ctx)

	// S>90: score=90.01 → 달성 → S
	tx2, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	grade, err = tx2.DetermineGrade(ctx, "strict", 90.01)
	require.NoError(t, err)
	assert.Equal(t, "S", grade, "gt 정책: 90.01 > 90 true → S")
	_ = tx2.Rollback(ctx)
}

// TestScoreGrade_DetermineGrade_ScopeNotFound D3 fail-closed:
// scope 행 0건 → ErrGradeThresholdsUnavailable (등급 fabricate 금지)
func TestScoreGrade_DetermineGrade_ScopeNotFound(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	ctx := context.Background()

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.DetermineGrade(ctx, "nonexistent-scope", 85.0)
	require.Error(t, err)
	assert.ErrorIs(t, err, stderrors.ErrGradeThresholdsUnavailable,
		"scope 0행 → ErrGradeThresholdsUnavailable (D3 fail-closed — 임의 등급 fabricate 금지)")
}

// TestScoreGrade_DetermineGrade_MultiScope 다중 scope 독립성:
// 'default'와 'kepco-safety' scope가 서로 다른 임계값을 가져도 독립 동작
func TestScoreGrade_DetermineGrade_MultiScope(t *testing.T) {
	defer goleak.VerifyNone(t, infraGoleakOptions()...)
	db := setupTestDB(t)
	applyMigration0004(t, db)
	insertDefaultGradeThresholds(t, db)
	ctx := context.Background()

	// 'kepco-safety' scope: S≥95 (더 엄격)
	_, err := db.pool.Exec(ctx,
		`INSERT INTO grade_thresholds (scope, letter, min_value, boundary_rule)
		 VALUES ('kepco-safety', 'S', 95, 'gte'),
		        ('kepco-safety', 'A', 85, 'gte'),
		        ('kepco-safety', 'B', 75, 'gte'),
		        ('kepco-safety', 'C', 65, 'gte'),
		        ('kepco-safety', 'D', 0, 'gte')
		 ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	tx, err := db.store.BeginScoreTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	// default scope: 90.0 → S
	gradeDefault, err := tx.DetermineGrade(ctx, "default", 90.0)
	require.NoError(t, err)
	assert.Equal(t, "S", gradeDefault)

	// kepco-safety scope: 90.0 → A (S≥95 조건 미달)
	gradeKepco, err := tx.DetermineGrade(ctx, "kepco-safety", 90.0)
	require.NoError(t, err)
	assert.Equal(t, "A", gradeKepco,
		"'kepco-safety' scope S 임계값 95 → 90.0은 A (scope 독립성)")
}
