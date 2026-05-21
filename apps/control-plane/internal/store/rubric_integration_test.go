//go:build integration

// rubric_integration_test.go — SPEC-AX-RUBRIC-001 DB 의존 통합 테스트 (Phase A RED skeleton)
//
// 검증 대상 (Phase C GREEN에서 testcontainers postgres:16-alpine + 0006 마이그레이션 적용 후 실행):
//   - T-RUBRIC-I-001 (REQ-RUBRIC-001-E1 + UBI-002): InsertRubric 동일-TX entity+audit
//   - T-RUBRIC-I-002 (UBI-003 / D1 iter2 lesson): userID 영속화 (created_by/updated_by/audit user_id)
//   - T-RUBRIC-I-003 (REQ-RUBRIC-003-E1): UpdateRubric draft→active 전이 + audit
//   - T-RUBRIC-I-004 (UBI-004): archived terminal 어떤 전이도 거부
//   - T-RUBRIC-I-005 (OPEN #2): partial unique idx race — 동시 active 전이 시 1건만 성공
//   - T-RUBRIC-I-006 (OPEN #4): EXCLUSION USING gist — 동시 band 추가 시 overlap 차단
//   - T-RUBRIC-I-007 (OPEN #6): ApplyRubric read-only — audit_logs 0건 단언
//   - T-RUBRIC-I-008 (OPEN #7): archive_reason CHECK constraint — empty 시 SQLSTATE 23514
//   - T-RUBRIC-I-009 (REQ-RUBRIC-004-E1): ApplyRubric 경계 inclusive 정확성 (numrange '[]')
//   - T-RUBRIC-I-010 (UBI-002): audit fault injection 양방향 rollback
//   - T-RUBRIC-I-011 (REQ-RUBRIC-002-E4): cross-store EvalItem 존재 검증 race
//   - T-RUBRIC-I-012 (REQ-RUBRIC-002-E3): CountRubrics full count (pagination total)
//   - T-RUBRIC-I-013 (멱등성): 0006 마이그레이션 재실행 안전
//   - T-RUBRIC-I-014 (R-RUBRIC-006): btree_gist extension fallback 동작
//
// 실행: go test -tags=integration -count=1 -p 1 -timeout=600s -run TestRubric ./apps/control-plane/internal/store/
//
// Phase C GREEN [HARD]: PgRubricTx (rubric.go) + 0006 마이그레이션 (Phase B) 완성.
// 본 통합 테스트는 testcontainers + Docker 환경 의존이므로 build tag `//go:build integration`로 분리.
// Docker 미가용 환경(CI/dev)에서는 setupRubricTestDB가 t.Skip으로 안전 우회.
// 활성화 명령: docker 환경 + `go test -tags=integration -count=1 -p 1 -timeout=600s -run TestRubric ./apps/control-plane/internal/store/`
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// setupRubricTestDB testcontainers postgres:16-alpine + 0001-0006 모든 마이그레이션이 적용된 testDB 반환.
// Phase C 활성화 시 setupReviewTestDB 패턴 정확 미러로 완성 예정 — 본 turn은 Docker 미검증으로 safe-skip 유지.
//
// applyMigration0006: btree_gist extension + rubrics/rubric_criteria/rubric_bands 테이블 +
// partial unique idx (OPEN #2) + EXCLUSION USING gist (OPEN #4) + CHECK archive_reason (OPEN #7)
//
// TODO(Phase D): docker 환경 검증 후 setupReviewTestDB 패턴 정확 미러로 활성화.
func setupRubricTestDB(t *testing.T) {
	t.Skip("Phase C: testcontainers + Docker + 0006 적용 helper 미구현 — Phase D 활성화 대기")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-001 [GREEN] InsertRubric 동일-TX entity+audit (UBI-002)
// ════════════════════════════════════════════════════════════════════════════

func TestRubricInsert_EntityAndAuditAtomic_Integration(t *testing.T) {
	setupRubricTestDB(t) // Phase A: Skip
	// Phase C GREEN:
	//   db := setupRubricTestDB(t); defer db.cleanup(t)
	//   ctx := context.Background()
	//   tx, err := db.store.BeginRubricTx(ctx); require.NoError(t, err)
	//   id, err := tx.InsertRubric(ctx, "rubric-i-001", "default", nil, "admin-001"); require.NoError(t, err)
	//   require.NoError(t, tx.Commit(ctx))
	//   assert.Equal(t, 1, rubricCount(t, db, id))
	//   assert.Equal(t, 1, auditRubricCount(t, db, id, "RUBRIC_CREATED"))
	_ = uuid.New() // 컴파일 통과용 — Phase C에서 실제 검증
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-002 [D1 iter2 lesson] userID 영속화 (UBI-003)
// REVIEW-001 D1 iter2 lesson 정확 미러 — BeginRubricTx audit.NewRecorder(true) 정합
// ════════════════════════════════════════════════════════════════════════════

func TestRubricInsert_UserIDPropagatesToAuditUserID_Integration(t *testing.T) {
	setupRubricTestDB(t)
	// Phase C GREEN:
	//   userID := "user-d1-lesson-42"
	//   id, _ := insertRubricHelper(t, db, ctx, userID)
	//   // audit_logs.user_id가 정확히 "user-d1-lesson-42" 영속 (NOT 'cli-anonymous')
	//   assert.Equal(t, userID, auditRubricUserID(t, db, id, "RUBRIC_CREATED"))
	//   // rubrics.created_by + updated_by도 정확 영속
	//   assert.Equal(t, userID, rubricCreatedBy(t, db, id))
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-003 UpdateRubric draft→active 전이 + RUBRIC_UPDATED audit
// ════════════════════════════════════════════════════════════════════════════

func TestRubricUpdate_DraftToActive_Integration(t *testing.T) {
	setupRubricTestDB(t)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-004 archived terminal 불변 (UBI-004)
// ════════════════════════════════════════════════════════════════════════════

func TestRubricArchive_TerminalImmutable_Integration(t *testing.T) {
	setupRubricTestDB(t)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-005 [OPEN #2 race] partial unique idx (rubrics_active_unique_idx) 차단
// 동시 admin이 동일 (name, scope) draft를 active로 전이 시 1건만 성공
// 나머지는 SQLSTATE 23505 (unique violation) → ErrRubricInvalidStatus → 409
// ════════════════════════════════════════════════════════════════════════════

func TestRubricUpdate_ConcurrentDraftToActive_OnlyOneSucceeds_Integration(t *testing.T) {
	setupRubricTestDB(t)
	// Phase C GREEN: goroutine 2개로 동시 UPDATE → 1 성공 + 1 23505 SQLSTATE
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-006 [OPEN #4 race] EXCLUSION USING gist 차단
// 동시 admin이 동일 rubric에 겹치는 band 추가 시 1건만 성공
// 나머지는 SQLSTATE 23P01 (exclusion violation) → ErrRubricBandOverlap → 400
// ════════════════════════════════════════════════════════════════════════════

func TestRubricBand_ConcurrentOverlap_OnlyOneSucceeds_Integration(t *testing.T) {
	setupRubricTestDB(t)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-007 [OPEN #6 read-only no-audit] ApplyRubric audit_logs 0건 [HARD]
// REPORT-001 선례 정확 미러 — UBI-002 second clause carve-out 검증
// ════════════════════════════════════════════════════════════════════════════

func TestRubricApply_ReadOnlyNoAuditWritten_Integration(t *testing.T) {
	setupRubricTestDB(t)
	// Phase C GREEN:
	//   beforeCount := totalAuditCount(t, db)
	//   _, _, _ = tx.ApplyRubric(ctx, rubricID, 85.5)
	//   afterCount := totalAuditCount(t, db)
	//   assert.Equal(t, beforeCount, afterCount, "OPEN #6: ApplyRubric은 audit_logs 0건 [HARD]")
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-008 [OPEN #7] archive_reason DB CHECK constraint (SQLSTATE 23514)
// handler bypass 시에도 strong invariant 보장
// ════════════════════════════════════════════════════════════════════════════

func TestRubricArchive_DBCheckRequiresReason_Integration(t *testing.T) {
	setupRubricTestDB(t)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-009 [REQ-RUBRIC-004-E1 + R-RUBRIC-007] 경계 inclusive 정확성
// numrange(min, max, '[]') 와 Go-side linear scan (min <= score <= max) 정합
// score=80.0 → B / 89.999 → B / 90.0 → A 명시 케이스
// ════════════════════════════════════════════════════════════════════════════

func TestRubricApply_BoundaryInclusive_Integration(t *testing.T) {
	setupRubricTestDB(t)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-010 [UBI-002] audit fault injection 양방향 rollback
// recorder가 InsertAuditLog 실패 반환 시 → entity-INSERT/audit-INSERT 양방향 취소
// ════════════════════════════════════════════════════════════════════════════

func TestRubricInsert_AuditFault_TwoWayRollback_Integration(t *testing.T) {
	setupRubricTestDB(t)
	// Phase C: whitebox fault recorder 주입 — REVIEW-001 score_review_request_test 패턴 미러
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-011 [REQ-RUBRIC-002-E4] cross-store EvalItem 존재 검증 race
// AddCriterion 시점 EvalItem 미존재 → 404 (cross-store handler-compose 검증)
// ════════════════════════════════════════════════════════════════════════════

func TestRubricCriterion_CrossStoreEvalItemRace_Integration(t *testing.T) {
	setupRubricTestDB(t)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-012 [REQ-RUBRIC-002-E3] CountRubrics full count (pagination total)
// AC-RUBRIC-002-3 — limit/offset 적용 전 전체 카운트
// ════════════════════════════════════════════════════════════════════════════

func TestRubricCount_FullCount_Integration(t *testing.T) {
	setupRubricTestDB(t)
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-013 0006 마이그레이션 멱등성 (재실행 안전)
// strategy.md §2.2 — CREATE EXTENSION IF NOT EXISTS / CREATE TABLE IF NOT EXISTS /
// DO$$ EXCEPTION duplicate_object / CREATE [UNIQUE] INDEX IF NOT EXISTS
// ════════════════════════════════════════════════════════════════════════════

func TestRubricMigration_Idempotent_Integration(t *testing.T) {
	setupRubricTestDB(t)
	// Phase C GREEN: applyMigration0006(t, db) 2회 호출 → error 0
}

// ════════════════════════════════════════════════════════════════════════════
// T-RUBRIC-I-014 [R-RUBRIC-006] btree_gist extension 활성화 확인
// PoC 환경(testcontainers postgres:16-alpine) btree_gist 지원 검증
// ════════════════════════════════════════════════════════════════════════════

func TestRubricBtreeGistExtension_Available_Integration(t *testing.T) {
	setupRubricTestDB(t)
	// Phase C GREEN: SELECT extname FROM pg_extension WHERE extname='btree_gist'
}

// ════════════════════════════════════════════════════════════════════════════
// 컴파일 통과용 — Phase C에서 helper 함수들 (rubricCount/auditRubricCount/etc.) 추가
// ════════════════════════════════════════════════════════════════════════════

var _ = context.Background // 미사용 import 회피 (Phase C에서 사용)
