-- 마이그레이션: 0002_evidence_tables
-- SPEC-AX-EVID-001 증빙 데이터 모델 (멱등성 패턴 유지, 수동 SQL)
-- initial.sql 은 수정하지 않는다 (schema drift 방지 — R-EVID-008, spec.md §2.2)
-- 적용: psql "$POSTGRES_DSN" -f 0002_evidence_tables.sql

CREATE TABLE IF NOT EXISTS evidences (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_item_id  VARCHAR(64) NOT NULL,                          -- 경량 FK stub, 제약 없음 (REQ-EVID 의도)
    version             INT NOT NULL DEFAULT 1,
    previous_version_id UUID REFERENCES evidences(id) ON DELETE RESTRICT,
    file_name           VARCHAR(512) NOT NULL,
    file_size_bytes     BIGINT,
    file_hash_sha256    VARCHAR(64),
    content_type        VARCHAR(128),
    file_content        BYTEA,                                         -- DB BLOB 바이너리 저장처. storage_strategy='database_blob'일 때 NOT NULL 의미(앱 계층 강제), 타 전략(filesystem/minio)에서는 NULL
    storage_location    VARCHAR(255),                                  -- database_blob: 논리 식별자 'db://evidences/<id>'; filesystem/minio: 외부 위치
    storage_strategy    VARCHAR(32) NOT NULL DEFAULT 'database_blob',  -- 'filesystem'|'database_blob'|'minio' (Run Phase 1 확정 기본값=database_blob)
    status              VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',         -- ACTIVE|SUPERSEDED
    metadata            JSONB,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by          VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',  -- audit.DefaultUserID 정합
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at         TIMESTAMP WITH TIME ZONE                       -- 미래 retention placeholder (본 SPEC 미사용)
);

DO $$ BEGIN
    ALTER TABLE evidences ADD CONSTRAINT evidences_storage_strategy_chk
        CHECK (storage_strategy IN ('filesystem','database_blob','minio'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS evidences_eval_item_version_idx
    ON evidences (evaluation_item_id, version DESC);
CREATE INDEX IF NOT EXISTS evidences_created_at_idx
    ON evidences (created_at DESC);
