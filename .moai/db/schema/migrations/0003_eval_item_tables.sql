-- 마이그레이션: 0003_eval_item_tables
-- SPEC-AX-EVAL-ITEM-001 평가항목 taxonomy 데이터 모델 (멱등성 패턴 유지, 수동 SQL)
-- initial.sql 은 수정하지 않는다 (schema drift 방지 — TH-07, spec.md §2.2)
-- id는 계층 코드 VARCHAR(64) (UUID 아님) — SPEC-AX-EVID-001 evidences.evaluation_item_id VARCHAR(64) stub과 타입 호환 (§1.4 HARD)
-- 적용: psql "$POSTGRES_DSN" -f 0003_eval_item_tables.sql (0001 → 0002 → 0003 순차)

CREATE TABLE IF NOT EXISTS evaluation_items (
    id              VARCHAR(64) PRIMARY KEY,                                  -- 계층 코드 (예: AX-SAFETY-ORG-01) — EVID-001 FK 타입 호환
    parent_id       VARCHAR(64) REFERENCES evaluation_items(id) ON DELETE RESTRICT,  -- 자기참조, root는 NULL
    display_name    VARCHAR(256) NOT NULL,
    description     TEXT,
    level           INT,                                                      -- 1범주 2항목 3지표 4배점 (informational)
    hierarchy_code  VARCHAR(128) NOT NULL UNIQUE,                              -- 경로 인코딩 (중복 방지, GAP-03 NOT NULL — AUD-1 surrogate 일관성)
    weight          DECIMAL(5,4),                                             -- 0.0-1.0 nullable
    max_score       INT,                                                      -- nullable
    status          VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',                    -- ACTIVE|DEPRECATED|ARCHIVED (CHECK)
    metadata        JSONB,                                                    -- opaque placeholder (등급기준 등 — 본 SPEC 미해석)
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by      VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',             -- audit.DefaultUserID 정합
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at     TIMESTAMP WITH TIME ZONE                                  -- 미래 lifecycle placeholder (본 SPEC 미사용)
);

DO $$ BEGIN
    ALTER TABLE evaluation_items ADD CONSTRAINT evaluation_items_status_chk
        CHECK (status IN ('ACTIVE','DEPRECATED','ARCHIVED'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS evaluation_items_parent_id_idx
    ON evaluation_items (parent_id);
CREATE INDEX IF NOT EXISTS evaluation_items_hierarchy_code_idx
    ON evaluation_items (hierarchy_code);
CREATE INDEX IF NOT EXISTS evaluation_items_created_at_idx
    ON evaluation_items (created_at DESC);
