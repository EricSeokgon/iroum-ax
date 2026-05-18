-- Sprint 3 통합 테스트용 PostgreSQL 스키마
-- initial.sql에서 control-plane 관련 테이블만 추출
-- testcontainers-go postgres:16-alpine 인스턴스에서 실행

-- ============================================================
-- 확장 기능 활성화
-- ============================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================
-- 열거형 타입 정의
-- ============================================================
DO $$ BEGIN
    CREATE TYPE workflow_status_enum AS ENUM ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- 워크플로우 테이블
-- Sprint 3 핵심 대상 테이블
-- 통합 테스트에서 document_id FK 없이 UUID를 직접 삽입할 수 있도록 참조 제약 없음
-- ============================================================
CREATE TABLE IF NOT EXISTS workflows (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     VARCHAR(64)          NOT NULL DEFAULT 'cli-anonymous',
    status      workflow_status_enum NOT NULL DEFAULT 'PENDING',
    document_id UUID,
    report_id   UUID,
    result_json JSONB,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- ============================================================
-- 감사 로그 테이블 (AC-CTRL-004-2 검증 대상)
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       VARCHAR(64)   NOT NULL DEFAULT 'cli-anonymous',
    action        VARCHAR(64)   NOT NULL,
    resource_id   UUID          NOT NULL,
    resource_type VARCHAR(32),
    timestamp     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    details       JSONB
);

-- ============================================================
-- 증빙 테이블 (SPEC-AX-EVID-001 — migrations/0002_evidence_tables.sql 미러)
-- 통합 테스트에서 evaluation_item_id FK 없이 임의 문자열 삽입 허용 (AC-EVID-001-3)
-- ============================================================
CREATE TABLE IF NOT EXISTS evidences (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_item_id  VARCHAR(64) NOT NULL,
    version             INT NOT NULL DEFAULT 1,
    previous_version_id UUID REFERENCES evidences(id) ON DELETE RESTRICT,
    file_name           VARCHAR(512) NOT NULL,
    file_size_bytes     BIGINT,
    file_hash_sha256    VARCHAR(64),
    content_type        VARCHAR(128),
    file_content        BYTEA,
    storage_location    VARCHAR(255),
    storage_strategy    VARCHAR(32) NOT NULL DEFAULT 'database_blob',
    status              VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    metadata            JSONB,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by          VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at         TIMESTAMP WITH TIME ZONE
);

DO $$ BEGIN
    ALTER TABLE evidences ADD CONSTRAINT evidences_storage_strategy_chk
        CHECK (storage_strategy IN ('filesystem','database_blob','minio'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ============================================================
-- 인덱스 정의
-- ============================================================
CREATE INDEX IF NOT EXISTS workflows_status_idx ON workflows (status);
CREATE INDEX IF NOT EXISTS workflows_document_id_idx ON workflows (document_id);
CREATE INDEX IF NOT EXISTS audit_logs_resource_idx ON audit_logs (resource_type, resource_id);
CREATE INDEX IF NOT EXISTS audit_logs_user_id_timestamp_idx ON audit_logs (user_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS evidences_eval_item_version_idx ON evidences (evaluation_item_id, version DESC);
CREATE INDEX IF NOT EXISTS evidences_created_at_idx ON evidences (created_at DESC);
