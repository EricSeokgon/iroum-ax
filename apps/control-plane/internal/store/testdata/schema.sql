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
-- 문서 테이블 (workflows 외래키 참조 해소용 최소 정의)
-- ============================================================
CREATE TABLE IF NOT EXISTS documents (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    filename    VARCHAR(512)     NOT NULL,
    file_type   VARCHAR(16)      NOT NULL DEFAULT 'PDF',
    status      VARCHAR(32)      NOT NULL DEFAULT 'PENDING',
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- ============================================================
-- 워크플로우 테이블
-- Sprint 3 핵심 대상 테이블
-- ============================================================
CREATE TABLE IF NOT EXISTS workflows (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     VARCHAR(64)          NOT NULL DEFAULT 'cli-anonymous',
    status      workflow_status_enum NOT NULL DEFAULT 'PENDING',
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
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
-- 인덱스 정의
-- ============================================================
CREATE INDEX IF NOT EXISTS workflows_status_idx ON workflows (status);
CREATE INDEX IF NOT EXISTS workflows_document_id_idx ON workflows (document_id);
CREATE INDEX IF NOT EXISTS audit_logs_resource_idx ON audit_logs (resource_type, resource_id);
CREATE INDEX IF NOT EXISTS audit_logs_user_id_timestamp_idx ON audit_logs (user_id, timestamp DESC);
