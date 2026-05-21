-- 마이그레이션: 0004_score_tables
-- SPEC-AX-SCORE-001 경영평가 점수 산출/집계 데이터 모델 (멱등성 패턴 유지, 수동 SQL)
-- initial.sql은 수정하지 않는다 (schema drift 방지 — TH-06)
-- D1: 단일 scores 테이블 + level discriminator ∈{raw,item,category}
-- D3: 별도 grade_thresholds 테이블 (scope,letter,min_value,boundary_rule, PK(scope,letter))
-- D4: status state-machine DRAFT→CONFIRMED→SUPERSEDED (CONFIRMED→SUPERSEDED terminal)
-- FK 없음: evaluation_item_id VARCHAR(64) NOT NULL FK-less (EVAL-ITEM-001 §1.4 compat)
--          evidence_id UUID NULL FK-less (EVID-001 compat) — consumer stub, 하드닝 out-of-scope
-- 적용: psql "$POSTGRES_DSN" -f 0004_score_tables.sql (0001 → 0002 → 0003 → 0004 순차)

CREATE TABLE IF NOT EXISTS scores (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),         -- D2: audit resource_id 직접 대입 (AUD-1 surrogate 미사용)
    evaluation_item_id  VARCHAR(64) NOT NULL,                                -- EVAL-ITEM-001 §1.4 FK-less stub (evaluation_items.id VARCHAR(64) 타입 호환)
    evidence_id         UUID,                                                 -- EVID-001 FK-less stub (evidences.id UUID 타입 호환), nullable
    level               VARCHAR(16) NOT NULL DEFAULT 'raw',                   -- D1 discriminator: raw|item|category
    score_value         DECIMAL(6,2),                                         -- nullable: 미입력 가능
    weight              DECIMAL(5,4),                                         -- nullable: NULL weight 정책 = exclude (GAP-01 결정적 정책)
    grade               VARCHAR(2),                                            -- nullable: 등급 산출 전 NULL, D3 grade_thresholds 기반
    status              VARCHAR(32) NOT NULL DEFAULT 'DRAFT',                 -- D4 state-machine: DRAFT|CONFIRMED|SUPERSEDED
    metadata            JSONB,                                                 -- opaque placeholder (한글 키 포함 임의 구조, 본 SPEC 미해석)
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by          VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',         -- audit.DefaultUserID 정합 (REQ-SCORE-UBI-003)
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- level discriminator CHECK (D1)
DO $$ BEGIN
    ALTER TABLE scores ADD CONSTRAINT scores_level_chk
        CHECK (level IN ('raw', 'item', 'category'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- status state-machine CHECK (D4)
DO $$ BEGIN
    ALTER TABLE scores ADD CONSTRAINT scores_status_chk
        CHECK (status IN ('DRAFT', 'CONFIRMED', 'SUPERSEDED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- grade 열거 CHECK — NULL 허용, 값이 있을 경우 S|A|B|C|D 중 하나
DO $$ BEGIN
    ALTER TABLE scores ADD CONSTRAINT scores_grade_chk
        CHECK (grade IS NULL OR grade IN ('S', 'A', 'B', 'C', 'D'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- 조회 인덱스
CREATE INDEX IF NOT EXISTS scores_evaluation_item_id_idx ON scores (evaluation_item_id);  -- SumWeightedByEvaluationItem EXPLAIN Index Scan 보장
CREATE INDEX IF NOT EXISTS scores_evidence_id_idx ON scores (evidence_id) WHERE evidence_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS scores_level_idx ON scores (level);

-- D3: 등급 기준 테이블 — scope별 letter↔min_value 매핑
-- boundary_rule: gte(score >= min_value, 기본), gt(score > min_value)
-- 결정적 S→D 내림차순 스캔: 첫 번째 매칭(score >= min_value) → grade
CREATE TABLE IF NOT EXISTS grade_thresholds (
    scope           VARCHAR(64)    NOT NULL,                                 -- 등급 기준 범위 식별자 (예: 'default', 'kepco-safety')
    letter          VARCHAR(2)     NOT NULL,                                 -- 등급 문자: S|A|B|C|D
    min_value       DECIMAL(6,2)   NOT NULL,                                 -- 해당 등급의 최소 점수
    boundary_rule   VARCHAR(8)     NOT NULL DEFAULT 'gte',                   -- gte: score >= min_value (기본), gt: score > min_value
    PRIMARY KEY (scope, letter)
);

-- letter 열거 CHECK
DO $$ BEGIN
    ALTER TABLE grade_thresholds ADD CONSTRAINT grade_thresholds_letter_chk
        CHECK (letter IN ('S', 'A', 'B', 'C', 'D'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- boundary_rule 열거 CHECK
DO $$ BEGIN
    ALTER TABLE grade_thresholds ADD CONSTRAINT grade_thresholds_boundary_rule_chk
        CHECK (boundary_rule IN ('gte', 'gt'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
