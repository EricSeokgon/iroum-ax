-- 마이그레이션: 0005_score_review_request_tables
-- SPEC-AX-REVIEW-001 평가 제출/승인 워크플로우 데이터 모델 (멱등성 패턴 유지, 수동 SQL)
-- 0004_score_tables.sql 정확 미러 (DO$$ EXCEPTION duplicate_object + CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS)
-- D-MIRROR: SCORE-001 D2 — audit resource_id = score_review_requests.id UUID 직접 대입 (surrogate 미사용)
-- 상태 머신: SUBMITTED → UNDER_REVIEW → {APPROVED|REJECTED} (terminal terminal)
-- §A.5 Layer 2: REJECTED 시 rejection_reason non-empty CHECK (이중 방어 fail-safe)
-- FK 없음: score_id UUID NOT NULL FK-less (SCORE-001 §1.4 정합 — handler-compose 2-TX 검증)
-- 적용: psql "$POSTGRES_DSN" -f 0005_score_review_request_tables.sql (0001 → ... → 0004 → 0005 순차)

CREATE TABLE IF NOT EXISTS score_review_requests (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),         -- D2: audit resource_id 직접 대입
    score_id              UUID NOT NULL,                                       -- SCORE-001 §1.4 FK-less stub
    status                VARCHAR(32) NOT NULL DEFAULT 'SUBMITTED',            -- 상태 머신: SUBMITTED|UNDER_REVIEW|APPROVED|REJECTED
    assigned_reviewer_id  VARCHAR(128),                                        -- nullable: 정보/감사용 (ABAC 무관)
    rejection_reason      TEXT,                                                -- nullable: REJECTED 시 non-empty 필수 (CHECK 강제)
    comment               TEXT,                                                -- nullable: 모든 상태에서 optional
    metadata              JSONB,                                                -- opaque placeholder
    created_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by            VARCHAR(128) NOT NULL DEFAULT 'cli-anonymous',       -- audit.DefaultUserID 정합 (REQ-REVIEW-UBI-003)
    updated_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_by            VARCHAR(128) NOT NULL DEFAULT 'cli-anonymous'
);

-- status state-machine CHECK (UBI-004)
DO $$ BEGIN
    ALTER TABLE score_review_requests ADD CONSTRAINT score_review_requests_status_chk
        CHECK (status IN ('SUBMITTED', 'UNDER_REVIEW', 'APPROVED', 'REJECTED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- §A.5 Layer 2: rejection_reason REJECTED 시 non-empty (fail-safe DB CHECK)
-- handler-local validateReviewRequestInput와 이중 방어 (EVAL-ITEM-001 선례 동형)
DO $$ BEGIN
    ALTER TABLE score_review_requests ADD CONSTRAINT score_review_requests_reject_reason_chk
        CHECK ((status != 'REJECTED') OR (rejection_reason IS NOT NULL AND length(rejection_reason) > 0));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- 조회 인덱스 (목록 조회 + 상태 필터 + 시간 정렬)
CREATE INDEX IF NOT EXISTS score_review_requests_score_id_idx ON score_review_requests (score_id);
CREATE INDEX IF NOT EXISTS score_review_requests_status_idx ON score_review_requests (status);
CREATE INDEX IF NOT EXISTS score_review_requests_created_at_idx ON score_review_requests (created_at DESC);
