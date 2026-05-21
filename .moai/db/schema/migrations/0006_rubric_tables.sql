-- 마이그레이션: 0006_rubric_tables
-- SPEC-AX-RUBRIC-001 등급 rubric 확장 시스템 3계층 데이터 모델 (rubrics + rubric_criteria + rubric_bands)
-- 0005_score_review_request_tables.sql 멱등 패턴 정확 미러
-- D-MIRROR: SCORE-001 D2 — audit resource_id = entity.id UUID 직접 대입 (surrogate 미사용)
-- 의존: 0001-0005 (0001 audit_logs / 0004 grade_thresholds 병행 존재 [HARD] 0-diff)
-- consumer-only [HARD]: SCORE-001 grade_thresholds 0-diff (병행 존재만, RUBRIC-001 rubric_bands와 독립)
-- 상태 머신: draft → active → archived (terminal). archived → 모든 전이 거부 (UBI-004).
-- 적용: psql "$POSTGRES_DSN" -f 0006_rubric_tables.sql (0001 → ... → 0005 → 0006 순차)

-- OPEN #4 dual defense: btree_gist extension for EXCLUSION USING gist with numrange
-- PostgreSQL 표준 contrib 모듈 — RDS/Neon/Supabase/일반 PostgreSQL 모두 지원, 외부 의존 추가 0
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS rubrics (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),         -- D2: audit resource_id 직접 대입
    name            VARCHAR(64) NOT NULL,                                -- 이름 (최대 64자, blank 거부는 핸들러 단계)
    version         INT NOT NULL DEFAULT 1,                              -- 버전 (clone-new-version 시 +1, OPEN #1)
    scope           VARCHAR(64),                                         -- 적용 범위 식별자 (nullable, 예: 'default'/'kepco-safety')
    status          VARCHAR(32) NOT NULL DEFAULT 'draft',                -- 상태 머신: draft|active|archived
    archive_reason  TEXT,                                                -- archived 시 필수 (OPEN #7 dual defense, CHECK 강제)
    metadata        JSONB,                                                -- opaque placeholder (스키마 미정의)
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by      VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',        -- audit.DefaultUserID 정합 (UBI-003)
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_by      VARCHAR(64)                                          -- 최종 갱신자
);

CREATE TABLE IF NOT EXISTS rubric_criteria (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rubric_id           UUID NOT NULL REFERENCES rubrics(id),             -- 동일 0006 마이그레이션 내부 FK 허용 (§2.3)
    evaluation_item_id  UUID NOT NULL,                                    -- FK-less stub (SCORE-001 §1.4 / REVIEW-001 §1.4 동형)
    weight              NUMERIC(5,4) NOT NULL,                            -- 가중치 0.0-1.0 (CHECK 강제)
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rubric_bands (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rubric_id   UUID NOT NULL REFERENCES rubrics(id),                     -- 동일 0006 마이그레이션 내부 FK 허용 (§2.3)
    letter      VARCHAR(8) NOT NULL,                                       -- 등급 문자 (예: 'S'|'A'|'B'|'C'|'D')
    min_score   NUMERIC(8,4) NOT NULL,                                     -- 구간 최소값 (inclusive)
    max_score   NUMERIC(8,4) NOT NULL,                                     -- 구간 최대값 (inclusive)
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- 멱등 CHECK constraints (0005 정확 미러)
-- status state-machine 열거 (UBI-004 / REQ-RUBRIC-003-S2)
DO $$ BEGIN
    ALTER TABLE rubrics ADD CONSTRAINT rubrics_status_chk
        CHECK (status IN ('draft', 'active', 'archived'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- weight 0.0-1.0 범위 (REQ-RUBRIC-001-U1)
DO $$ BEGIN
    ALTER TABLE rubric_criteria ADD CONSTRAINT rubric_criteria_weight_chk
        CHECK (weight >= 0 AND weight <= 1);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- min < max 강제 (REQ-RUBRIC-001-U1, AC-RUBRIC-001-5)
DO $$ BEGIN
    ALTER TABLE rubric_bands ADD CONSTRAINT rubric_bands_range_chk
        CHECK (min_score < max_score);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- OPEN #7 dual defense Layer 2: archive_reason archived 시 non-empty 필수
-- handler-local validation pre-store와 이중 방어 (REVIEW-001 rejection_reason 동형)
DO $$ BEGIN
    ALTER TABLE rubrics ADD CONSTRAINT rubrics_archive_reason_chk
        CHECK ((status != 'archived') OR (archive_reason IS NOT NULL AND length(archive_reason) > 0));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- OPEN #4 dual defense Layer 2: EXCLUSION USING gist for band overlap (inclusive boundaries '[]')
-- 동일 rubric_id 내에서 numrange [min_score, max_score] 양 끝 inclusive 겹침 자동 차단
-- handler-local validation pre-check (Phase C)와 이중 방어 — race window 봉쇄
DO $$ BEGIN
    ALTER TABLE rubric_bands ADD CONSTRAINT rubric_bands_no_overlap
        EXCLUDE USING gist (rubric_id WITH =, numrange(min_score, max_score, '[]') WITH &&);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- OPEN #2 dual defense Layer 2: active rubric 1-per-(name, scope) partial unique index
-- draft/archived는 자유 다중 row 허용 (version별 다중 draft 가능), active만 unique
CREATE UNIQUE INDEX IF NOT EXISTS rubrics_active_unique_idx
    ON rubrics (name, scope) WHERE status = 'active';

-- 일반 조회 인덱스 (목록 조회 + 상태 필터 + 시간 정렬)
CREATE INDEX IF NOT EXISTS rubrics_status_idx ON rubrics (status);
CREATE INDEX IF NOT EXISTS rubrics_scope_idx ON rubrics (scope);
CREATE INDEX IF NOT EXISTS rubrics_created_at_idx ON rubrics (created_at DESC);
CREATE INDEX IF NOT EXISTS rubric_criteria_rubric_id_idx ON rubric_criteria (rubric_id);
CREATE INDEX IF NOT EXISTS rubric_bands_rubric_id_letter_idx ON rubric_bands (rubric_id, letter);
