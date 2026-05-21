# Research: SPEC-AX-EVID-001 경영평가 증빙 자료 수집/관리 (Evidence Management)

Phase: 0.5 Deep Research
Generated: 2026-05-18
Agent: Explore (read-only deep codebase analysis)
Status: complete

> 본 문서는 SPEC-AX-EVID-001 EARS 요구사항 설계의 근거 자료입니다. 모든 주장은 file:line 근거를 포함합니다.

---

## 1. Architecture Analysis

패키지 맵 (`apps/control-plane/internal/`):

| Package | Purpose | Key Files | Line References |
|---------|---------|-----------|-----------------|
| `store/` | 영속 계층 (PostgreSQL) | `store.go:1-52`, `postgres.go:1-59`, `fake_store.go` | WorkflowStore interface (:18-26), WorkflowTx interface (:28-51) |
| `audit/` | 감사 추적 기록 | `audit.go:1-67`, `recorder.go:1-212` | Event struct (:59-66), Recorder struct (:38-48) |
| `server/` | gRPC/REST 핸들러 | `server.go:1-450+` | Server struct (:39-65), New() (:67-150+) |
| `workflow/` | 상태 머신 & 조정 | `state_machine.go`, `handlers.go`, `callback.go` | WorkflowState enum (types.go:14-25) |
| `config/` | 환경 설정 | `config.go:1-137` | Config struct (:12-79), Load() (:83-100) |
| `scheduler/` | Celery dispatch | `dispatcher.go`, `celery_envelope.go` | CeleryDispatcher |
| `auth/` | 인증/인가 | `rbac.go`, `middleware.go`, `validator.go` | server.go:23에서 참조 |
| `metrics/` | Prometheus 계측 | Various | Optional (REQ-CTRL-002-O1) |
| `observability/` | OpenTelemetry tracing | `tracer.go` | server.go:79에서 InitTracer() 호출 |

서버 초기화 패턴 (`cmd/server/server.go:73-150`):
- REQ-SERVER-UBI-001-b가 10단계 초기화 순서 정의
- (c)단계: PgWorkflowStore 초기화 + Ping 검증 (line 86-98)
- (d~j)단계: Redis, OIDC, auth, metrics, workflow coordination 순서대로 연결
- 패턴: 초기화 실패 시 fail-fast (DB 미가용 시 panic) — REQ-CTRL-004-E1

## 2. Store Layer & Migration Conventions

DB 스키마 위치: `.moai/db/schema/initial.sql:1-139`

주요 테이블:
- `workflows` (lines 74-83): UUID id, user_id VARCHAR(64) DEFAULT 'cli-anonymous', status ENUM, document_id FK, report_id FK, result_json JSONB, timestamps
- `audit_logs` (lines 115-123): id UUID, user_id VARCHAR(64), action VARCHAR(64), resource_id UUID, resource_type VARCHAR(32), timestamp TIMESTAMP, details JSONB

마이그레이션 아키텍처 (`.moai/db/schema/migrations/0001_initial.sql:1-8`):
- 패턴: 수동 SQL 파일 (Alembic/golang-migrate 미사용)
- 위치: `.moai/db/schema/migrations/NNNN_description.sql`
- 멱등성: PostgreSQL `DO $$...EXCEPTION WHEN duplicate_object...$$` (initial.sql lines 17-30)
- 확장: uuid-ossp, pgvector (lines 11-12)
- enum 타입: IF NOT EXISTS 보호 (lines 17-30)

DSN 설정 패턴 (`config.go:85`):
- 환경 변수 `POSTGRES_DSN` (default: `postgres://iroum:iroum@localhost:5432/iroum_ax?sslmode=disable`)
- 제약: 외부 서비스 호출 없음 — REQ-CTRL-004 (내부 postgres만)
- pool 설정 (`config.go:86-97`): 현재 코드 미외부화 (Sprint 0 stub), pgxpool.New()에서 처리 예정 (SPEC-AX-CTRL-001 §2.1)

Store 인터페이스 설계 (`store.go:13-51`):
- `WorkflowStore` (:18-26): BeginTx() 진입점 (트랜잭션 전용), ListWorkflows() 페이지네이션
- `WorkflowTx` (:28-51): InsertWorkflow, InsertAuditLog, UpdateWorkflowState, GetWorkflow, UpdateWorkflowResult, Commit, Rollback
- 설계 패턴: 모든 쓰기는 트랜잭션 추상화 경유 (raw SQL 누출 방지)
- `@MX:ANCHOR` (store.go:16, audit/recorder.go:36): fan_in >= 3 식별

## 3. Audit Integration Path

감사 모델 (`audit/audit.go:13-54`):
- 8개 워크플로우 액션: ActionWorkflowCreated(:17), ...TransitionedToRunning(:19), ...Completed(:21), ...FailedDispatch(:23), ...FailedCallback(:25), ActionTransitionRejected(:27), ActionCallbackRejectedTerminal(:29), ActionWorkflowCreateCancelled(:31)
- 추가 Auth/Server 액션: ActionAuthForbidden(:34), ActionAuthLogout(:37), ActionAuthRefreshReuseDetected(:40), ActionABACDenied(:43), ActionServerStartup(:47-48), ...ShutdownInitiated(:50), ...ShutdownCompleted(:53)

Event 엔티티 (`audit/audit.go:56-66`):
- 필드: Timestamp, Action, ResourceType, UserID, DetailsJSON ([]byte), ResourceID (UUID)
- DefaultUserID = "cli-anonymous" (recorder.go:19, SPEC-AX-001 REQ-UBI-003 정합)

Recorder 패턴 (`audit/recorder.go:33-212`):
- `Recorder` struct (:38-48): authEnabled 보유, 액션별 8개 메서드
- 시그니처: `func (r *Recorder) Record*(ctx, tx AuditTx, workflowID, userID string) error`
- `AuditTx` interface (:28-31): WorkflowTx.InsertAuditLog 부분집합 (순환 의존 회피)
- 원자성 보장: Recorder가 비즈니스 로직과 동일 트랜잭션 내 tx.InsertAuditLog 호출 (REQ-CTRL-UBI-001/002, AC-CTRL-UBI-002-A/B/C)
- 버전 히스토리 훅 포인트: EVID-001은 Recorder에 `RecordEvidenceVersioned()` 등을 동일 패턴으로 확장

## 4. REQ-UBI Pattern & 한국 공공 6제약

Canonical REQ-UBI 구조 (SPEC-AX-001 `spec.md:131-133`):

```
- **REQ-UBI-NNN (한글 제목)**: The system SHALL... [영문 정책]
```

EARS 사용 (SPEC-AX-CTRL-001 §3):
- Ubiquitous (U): 시스템 전역 불변식 (REQ-CTRL-UBI-001/UBI-002 §3.1)
- Event-driven (E): 외부 자극 트리거 (REQ-CTRL-001-E1/E2 §3.2)
- State-driven (S): 연속 조건 검사 (REQ-CTRL-001-S1 §3.2)
- Unwanted: 오류/무효 케이스 (REQ-CTRL-001-U1/U2, REQ-CTRL-003-U1 §3.3-3.4)
- Optional (O): 비필수 기능 (REQ-CTRL-002-O1 §3.3)

HISTORY 포맷 (SPEC-AX-CTRL-001 lines 12-16):
```
- VERSION (DATE): [카테고리] [요약]. [D1(...), D2(...) 섹션 참조]. (작성자: author)
```

AC 명명: `AC-[SPEC-ID]-[REQ-ID]-[NUMBER]` (예: AC-CTRL-UBI-002-A/B/C 삼중 시나리오)

한국 공공 6제약:
1. 데이터 주권 (Data Sovereignty) — 외부 LLM API 호출 금지 (REQ-UBI-001, SPEC-AX-001 §3.1 "외부 LLM API 호출은 금지된다")
2. 언어 (Language) — 한국어 1차 언어 (REQ-UBI-002)
3. 감사 가능성 (Auditability) — 모든 이벤트 audit_logs 기록 (REQ-UBI-003)
4. 망분리 정합 (Air-gapped compliance) — 외부 서비스 의존 없음 (SPEC-AX-AUTH-003 §1.3, product.md §3.3)
5. 조직 격리 (Organization Isolation) — 멀티 조직 데이터 분리 (SPEC-AX-AUTH-003 §2.2 갭 #2)
6. 시간 제약 (Time Constraints) — 업무 시간 강제 (SPEC-AX-AUTH-003 §2.2 갭 #3, 향후 SPEC)

선례 강제 (SPEC-AX-CTRL-001 §1.2): "SPEC-AX-001의 5개 REQ-AX와 REQ-UBI-001~003은 Python 측 GREEN 상태로 가정하며, 본 SPEC은 그 위에 Go orchestration 계층을 얹는다."

## 5. SPEC Document Conventions

YAML frontmatter 8 필드 (SPEC-AX-CTRL-001 lines 2-9):

| Field | Example | Rules |
|-------|---------|-------|
| `id` | SPEC-AX-CTRL-001 | 복합 도메인 `SPEC-[D1]-[D2]-NNN` (최대 3) |
| `version` | 0.1.2 | 0.minor.patch |
| `status` | draft/completed | PASS 평가 후에만 completed |
| `created` | 2026-05-14 | YYYY-MM-DD (최초) |
| `updated` | 2026-05-14 | YYYY-MM-DD (최신) |
| `author` | ircp | git author |
| `priority` | high | high/medium/low |
| `issue_number` | 0 | 없으면 0 |

섹션 구조: ① Frontmatter ② HISTORY ③ §1 개요(Walking Skeleton) ④ §2 영향받는 파일 ⑤ §3 EARS 요구사항(U/E/S/Unwanted/Optional) ⑥ §4 비기능 요구사항 ⑦ §5 Exclusions ⑧ §6 의존성/전제 ⑨ §7 Out of Scope ⑩ §8 검증 방법 요약

acceptance.md: Given/When/Then BDD, AC당 1 시나리오 (AC-REQ-ID-N 번호), 한국 특화 기준은 표 포맷

## 6. Data Sovereignty Precedent

SPEC-AX-CTRL-001 §4 비기능 요구사항 인용:
> | 망분리 | 외부 API 호출 0건 (Redis + PostgreSQL 내부망만), 외부 패키지 fetch는 go.sum 핀 | `tech.md` §9.1 |

config.go:85 DSN 선례:
```go
PostgresDSN: getEnv("POSTGRES_DSN", "postgres://iroum:iroum@localhost:5432/iroum_ax?sslmode=disable"),
```
- HTTPS/TLS override 없음, 외부 secrets manager 없음 (env var 12-factor만)

서버 startup fail-fast (server.go:86-98): PostgreSQL ping 실패 시 `pgStore.Close(); tracerShutdown(); return nil, fmt.Errorf(...)` — 코어 인프라 graceful degradation 없음 (데이터 주권 비협상)

## 7. Reference Implementations

"바이너리/문서 저장 + 버전 엔티티 + 감사 연계"에 가장 근접한 기존 패턴:

documents 테이블 (initial.sql:37-47):
```sql
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    filename VARCHAR(512) NOT NULL,
    file_type file_type_enum NOT NULL,
    parsed_text TEXT,
    language VARCHAR(8) NOT NULL DEFAULT 'ko',
    parse_quality_flag VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
```
- version 컬럼 없음 (현재 불변 레코드), parent_id FK 없음 (버전 체이닝 없음)
- 감사 추적은 audit_logs로 분리

EVID-001 갭: documents는 불변 → 재업로드 버전 관리 부재. 추가 필요: evidence 테이블 `version` + `previous_version_id` FK 체이닝, Recorder에 `RecordEvidenceCreated`/`RecordEvidenceVersioned` 확장.

## 8. Risks, Constraints, Implicit Contracts

- Risk 1 — 순환 의존 방지: `audit/recorder.go:28-31`이 `AuditTx`를 로컬 정의 (store import 금지, store.go:9가 audit import). EVID-001 evidence audit recorder도 동일 패턴 필수.
- Risk 2 — User ID 전파: `recorder.go:52-57` resolveUserID, 기본 `cli-anonymous`. authEnabled=false면 모든 audit_logs.user_id가 'cli-anonymous' (실 사용자 누출 금지). 증빙 업로드 user_id도 준수.
- Risk 3 — 트랜잭션 원자성: `store.go:32` WorkflowTx atomic. BeginTx writer lock (pgx SELECT FOR UPDATE). 증빙 버전 관리도 동일 TX 패턴 아니면 orphan 버전 누출.
- Risk 4 — 임베디드 자격증명 금지: `config.go:103-109` getEnv() 하드코딩 시크릿 없음. DB BLOB 선택 시 자격증명 소스 노출 금지.
- Risk 5 — Auth 미들웨어 통합: server.go REST 체인 `auth.BuildRESTChain(...)` (SPEC-AX-AUTH-003 §2.0 line 70). 증빙 업로드 엔드포인트는 authz_mapping.go에 권한(`write:evidence`) 등록 필수.
- Risk 6 — PgWorkflowStore 싱글톤: server.go:86-98 단일 pgStore. 증빙 store는 동일 pgx pool 재사용 (신규 풀 생성 금지).
- Risk 7 — Recorder 테스트 비친화: recorder.go time.Now().UTC() 직접 호출. 증빙 audit recorder는 clock 주입 설계 권장.

## 9. Recommended Implementation Approach

제안 데이터 모델:

```sql
CREATE TABLE IF NOT EXISTS evidences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_item_id VARCHAR(64) NOT NULL,  -- 경량 FK stub (제약 없음)
    version INT NOT NULL DEFAULT 1,
    previous_version_id UUID REFERENCES evidences(id) ON DELETE RESTRICT,
    file_name VARCHAR(512) NOT NULL,
    file_size_bytes BIGINT,
    file_hash_sha256 VARCHAR(64),
    content_type VARCHAR(128),
    storage_location VARCHAR(255),
    storage_strategy VARCHAR(32) NOT NULL,  -- 'filesystem'|'database_blob'|'minio' (strategy 단계 결정)
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    created_by VARCHAR(64) NOT NULL DEFAULT 'cli-anonymous',
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    archived_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS evidences_eval_item_version_idx ON evidences (evaluation_item_id, version DESC);
CREATE INDEX IF NOT EXISTS evidences_created_at_idx ON evidences (created_at DESC);
```

Store 통합: `EvidenceStore`/`EvidenceTx` 인터페이스를 store.go 패턴대로 추가 (BeginTx → InsertEvidence/GetEvidenceByID/ListEvidenceByEvalItem/MarkEvidenceDeprecated/InsertAuditLog/Commit/Rollback). 동일 pgx pool 재사용.

Audit 통합: Recorder에 `RecordEvidenceCreated`, `RecordEvidenceVersioned` 추가. audit.go에 신규 액션 상수 (ActionEvidenceCreated/Versioned). AuditTx 로컬 인터페이스 패턴 유지.

서버 엔드포인트: multipart 업로드 → SHA256 해시 → BeginTx → eval_item_id 기존 max version 조회 → InsertEvidence (previous_version_id FK) → Recorder 기록 → 파일 저장 (전략 미정) → Commit → 201.

저장 전략 결정 프레임워크 (strategy 단계로 이연):

| Strategy | Pros | Cons | 망분리 호환 |
|----------|------|------|-----------|
| Filesystem (`/srv/evidence/`) | 단순, 빠름, DB 비대화 없음 | 백업 복잡, 멀티노드 제약 | YES |
| DB BLOB (BYTEA) | evidence 레코드와 원자적, 트랜잭션 일관성 | PG 저장 한계, dedup 없음, 조회 느림 | YES |
| MinIO (self-hosted S3) | 확장성, S3 API | 추가 서비스 의존(외부면 망분리 위반) | MAYBE (self-hosted MinIO만) |

복합 도메인: `SPEC-AX-EVID-001` (AX + EVID, 최대 3 규칙 준수)

의존성: SPEC-AX-CTRL-001 (WorkflowStore/AuditRecorder 패턴) GREEN 가정 + 평가 항목 ID 경량 stub (순환 의존 없음) + PostgreSQL 신규 테이블/인덱스

TDD 전략: RED(evidence_created audit 이벤트 테스트) → GREEN(RecordEvidenceCreated) → RED(InsertEvidence atomic+audit) → GREEN → REFACTOR(TxCoordinator 추출), 버전/deprecated/listing 반복

마이그레이션: `.moai/db/schema/migrations/0002_evidence_tables.sql` 신규, 멱등성 패턴(DO $$...EXCEPTION...) 유지, 수동 SQL 규약
