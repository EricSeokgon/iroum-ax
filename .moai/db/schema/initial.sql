-- iroum-ax 초기 데이터베이스 스키마
-- Sprint 0 스켈레톤 (T-AX-008)
-- 실제 데이터 삽입은 Sprint 1 이후
--
-- 실행: psql "postgresql://ax:devpass@localhost:5432/iroum_ax" -f initial.sql
-- docker-compose: 자동 실행 (docker-entrypoint-initdb.d)

-- ============================================================
-- 확장 기능 활성화
-- ============================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";   -- UUID 생성
CREATE EXTENSION IF NOT EXISTS vector;        -- pgvector (HNSW 인덱스)

-- ============================================================
-- 열거형 타입 정의
-- ============================================================
DO $$ BEGIN
    CREATE TYPE file_type_enum AS ENUM ('HWP', 'PDF', 'IMAGE');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE workflow_status_enum AS ENUM ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE grade_enum AS ENUM ('A', 'B', 'C', 'D');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- 테이블 정의
-- ============================================================

-- 파싱된 문서 (HWP / PDF / 이미지)
CREATE TABLE IF NOT EXISTS documents (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    filename           VARCHAR(512)       NOT NULL,
    file_type          file_type_enum     NOT NULL,
    parsed_text        TEXT,
    language           VARCHAR(8)         NOT NULL DEFAULT 'ko',
    parse_quality_flag VARCHAR(64),               -- 예: 'LOW_OCR_CONFIDENCE'
    status             VARCHAR(32)        NOT NULL DEFAULT 'PENDING',
    metadata           JSONB,
    created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- 경영평가 기준 (평가편람 계층 구조)
-- embedding: ko-sroberta-multitask 768 차원 (D12: 1536 → 768 수정)
CREATE TABLE IF NOT EXISTS criteria (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    criterion_name       VARCHAR(256)  NOT NULL,
    criterion_detail     TEXT,
    max_points           INT,
    parent_criterion_id  UUID REFERENCES criteria(id) ON DELETE SET NULL,
    embedding            VECTOR(768),              -- ko-sroberta-multitask 차원
    normalization_warning JSONB,                   -- 한자/한글 정규화 경고
    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- 실적보고서 (자사 또는 벤치마크 A/B/C/D)
CREATE TABLE IF NOT EXISTS reports (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_name   VARCHAR(256),
    grade               grade_enum,
    content             TEXT,
    score               INT,
    source_benchmark_id UUID,                     -- 참조 벤치마크 ID (자사 = NULL)
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- 워크플로우 실행 상태 (Control Plane 관리)
CREATE TABLE IF NOT EXISTS workflows (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     VARCHAR(64)          NOT NULL DEFAULT 'cli-anonymous',
    status      workflow_status_enum NOT NULL DEFAULT 'PENDING',
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    report_id   UUID REFERENCES reports(id)   ON DELETE SET NULL,
    result_json JSONB,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- 등급 시뮬레이션 결과
-- abstain_flag: probability_a < 0.5 AND probability_b < 0.5 (REQ-AX-003-E1)
CREATE TABLE IF NOT EXISTS simulations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID REFERENCES workflows(id) ON DELETE CASCADE,
    current_grade   grade_enum,
    target_grade    grade_enum,
    probability_a   DECIMAL(4, 3),             -- A 등급 확률 [0.000, 1.000]
    probability_b   DECIMAL(4, 3),             -- B 등급 확률 [0.000, 1.000]
    abstain_flag    BOOLEAN         NOT NULL DEFAULT FALSE,
    status          VARCHAR(32)     NOT NULL DEFAULT 'PENDING',
    prediction      VARCHAR(16),               -- 기권 시 NULL
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- Gap 개선 추천 항목
CREATE TABLE IF NOT EXISTS recommendations (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id         UUID REFERENCES workflows(id) ON DELETE CASCADE,
    content             TEXT          NOT NULL,
    expected_score_delta INT,
    feasibility_score   DECIMAL(3, 2),          -- [0.00, 1.00]
    source_benchmark_id UUID,
    status              VARCHAR(32)   NOT NULL DEFAULT 'ACTIVE',
    feedback            JSONB,
    priority            INT           NOT NULL DEFAULT 0,  -- 낮을수록 우선
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- 감사 로그 (REQ-UBI: 모든 사용자 행동 추적)
CREATE TABLE IF NOT EXISTS audit_logs (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        VARCHAR(64)   NOT NULL DEFAULT 'cli-anonymous',
    action         VARCHAR(64)   NOT NULL,
    resource_id    UUID          NOT NULL,
    resource_type  VARCHAR(32),
    timestamp      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    details        JSONB
);

-- ============================================================
-- 인덱스 정의
-- ============================================================

-- pgvector HNSW 인덱스 (코사인 유사도 검색, REQ-AX-002)
CREATE INDEX IF NOT EXISTS criteria_embedding_hnsw_idx
    ON criteria USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- B-tree 인덱스
CREATE INDEX IF NOT EXISTS documents_created_at_idx ON documents (created_at DESC);
CREATE INDEX IF NOT EXISTS workflows_document_id_idx ON workflows (document_id);
CREATE INDEX IF NOT EXISTS workflows_status_idx ON workflows (status);
CREATE INDEX IF NOT EXISTS audit_logs_user_id_timestamp_idx ON audit_logs (user_id, timestamp DESC);
