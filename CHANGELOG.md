# Changelog

본 프로젝트의 모든 주목할 만한 변경 사항을 이 파일에 기록합니다.

형식은 [Keep a Changelog](https://keepachangelog.com/ko/1.1.0/)을 따르며,
이 프로젝트는 [Semantic Versioning](https://semver.org/lang/ko/)을 준수합니다.

## [Unreleased] - 2026-05-15

### Added — SPEC-AX-AUTH-001 v0.1.1 (SSO/JWT 인증 + RBAC + OAuth 2.0 BCP)

#### Sprint 0: Auth Foundation (Go 3 의존성 + Python 2 의존성)
- pkg/auth + pipelines/auth + apps/control-plane/internal/auth 신규 12개 파일
- Go 의존성: golang-jwt/v5, coreos/go-oidc/v3, MicahParks/keyfunc/v3
- Python 의존성: PyJWT[cryptography], authlib

#### Sprint 1: REQ-AUTH-001 JWT Validator (SF-1 + SF-2)
- TokenValidator: JWT signature + iss(SF-1) + alg/kty(SF-2) + aud/exp/kid 검증
- 19개 테스트 (signature/issuer/algorithm/key-type/expiration/kid)
- SF-1: RFC 7519 §4.1.1 cross-realm 토큰 재사용 공격 차단
- SF-2: Algorithm Confusion Attack 변형 방어 (OWASP JWT cheat sheet)

#### Sprint 2: REQ-AUTH-002 OIDC Discovery + JWKS Cache
- OIDCClient: well-known/openid-configuration 자동 discovery
- JWKSCache: TTL 3600초 + max-age 4시간 stale-while-revalidate
- 17개 테스트 (discovery/cache-hit/ttl-expire/background-refresh/concurrent)

#### Sprint 3: REQ-AUTH-003 Middleware (gRPC + REST)
- UnaryServerInterceptor: gRPC 인증 미들웨어 (Bearer token)
- RESTMiddleware: HTTP Authorization 헤더 검증
- Health endpoint bypass (/grpc.health.v1.Health/Check)
- AuthDisabled 폴백 (테스트/개발 모드)
- 20개 테스트 (valid/invalid/expired/malformed/health-bypass)

#### Sprint 4: Python + Celery Cross-SPEC
- pipelines/auth/validator.py: FastAPI 동기 검증
- celery_auth.py: envelope.headers.user_id 전파
- 15개 Python + 5개 Go cross-SPEC 테스트
- Golden file 재생성 (envelope 형식 정규화)

#### Sprint 5: REQ-AUTH-004 RBAC (3-Role Matrix)
- ParseRolesFromScope: "admin:*", "analyst:read:*", "viewer:read:document" 파싱
- EffectivePermissions: 3역할 매트릭스 (admin > analyst > viewer)
- Authorize(action): 필수 권한 검증 + LogForbidden audit
- 18개 테스트 (role-matrix/permission-calculation/forbidden-logging)

#### Sprint 6: REQ-AUTH-005 Refresh + Logout (OAuth 2.0 BCP)
- RefreshSession: 토큰 갱신 + 새 access/refresh 발급
- RefreshTokenReuseDetection: family invalidation (재사용 감지 시 전체 계열 무효화)
- Logout: refresh_token_family 블랙리스트 기록
- 13개 테스트 (rotation/reuse-detection/family-invalidation)

#### Sprint 7: E2E Integration
- AC-AUTH-E2E-1 ✓ 전체 JWT 체인 (Keycloak → validator → middleware → RBAC → audit)
- AC-AUTH-E2E-2 ✓ 익명 요청 역호환성 (AuthDisabled=true)
- AC-AUTH-E2E-4 ✓ 유효하지 않은 토큰 401 응답
- AC-E2E-RBAC-1 SKIP → SPEC-AX-AUTH-002 (REST handler 통합)
- 4 PASS + 1 SKIP (E2E 통합 테스트)

#### 품질 (SPEC-AX-AUTH-001 누적)
- Go 90개 + Python 15개 신규 테스트 = 105 신규 tests
- TOTAL: Python 192 + Go 156 unit + 11 integration + 5 E2E = 380+ 테스트
- TRUST 5: Tested 90/15 ✓ | Readable (gofmt+ko-comments) ✓ | Unified (golangci-lint 0 errors) ✓ | Secured (SF-1/SF-2) ✓ | Trackable (55 @MX tags) ✓
- plan-auditor 0.88 PASS + evaluator-active 0.782 CONFIRM

#### 보안 (계속)
- SF-1 iss per-token validation (RFC 7519 §4.1.1)
- SF-2 alg/kty cross-check (Algorithm Confusion Attack 방어)
- OAuth 2.0 BCP: refresh token rotation + family invalidation
- 망분리 정합 유지 (Keycloak self-hosted, 외부 OAuth 0건)
- audit_logs.user_id 실 사용자 propagation (JWT sub 추출)
- ErrTokenInvalidIssuer/ErrAlgorithmKeyMismatch/ErrRefreshTokenReuseDetected sentinel 도입

#### Fixed
- grpc_server.go CreateWorkflow의 cli-anonymous 하드코딩 → auth.UserFromContext JWT sub 추출 (Sprint 7 E2E 발견)

#### Deferred (후속 SPEC 후보)
- SPEC-AX-AUTH-002: RBAC REST handler 통합 (E2E SKIP된 항목)
- SPEC-AX-AUTH-EGOV-001: 전자정부 표준 인증 (KEPCO 요구 시)
- SPEC-AX-AUTH-MFA-001: 다단계 인증

---

### Added — SPEC-AX-CTRL-001 v0.1.2 Go Control Plane Walking Skeleton

#### Sprint 0: CTRL Foundation (Go 1.22 모듈 + 기본 의존성)
- Go 1.22 모듈 구조 (apps/control-plane/)
- 핵심 의존성 9개 (uuid, zap, pgx, redis, testcontainers, etc.)
- golangci-lint 설정 + GitHub Actions CI/CD
- @MX 태그 규칙 정의 (27개 ANCHOR/NOTE/WARN)

#### Sprint 1: REQ-CTRL-UBI-001/002 (감시 로깅 + 트랜잭션 원자성)
- WorkflowStore/WorkflowTx 인터페이스 (8 감시 액션 정의)
- TxCoordinator 스텁 (트랜잭션 조율)
- SELECT FOR UPDATE 검증 기본 계획
- 26개 테스트 (fake_store 8 + recorder 11 + transaction 7)

#### Sprint 2: REQ-CTRL-001 Workflow State Machine
- 4상태 워크플로우 (PENDING → RUNNING → COMPLETED | FAILED)
- 3전이 규칙 (Start, Complete, Fail)
- 동시성 직렬화 (SELECT FOR UPDATE)
- 14개 테스트 (상태 전이 + 불변성 + edge cases)

#### Sprint 3: REQ-CTRL-004 PostgreSQL Store (pgx v5 + testcontainers)
- PgWorkflowStore/PgWorkflowTx 구현
- CRUD + SELECT FOR UPDATE 동시성 테스트
- audit_logs JSONB INSERT
- 11개 통합 테스트 (//go:build integration)

#### Sprint 4: REQ-CTRL-002 gRPC Server (unary RPC × 3)
- CreateWorkflow/GetWorkflow/ListWorkflows RPC
- 구조화 JSON 로깅 미들웨어 (zap)
- bufconn in-memory 클라이언트 테스트
- 12개 테스트 (RPC 동작 + 에러 처리 + 동시성)

#### Sprint 5: REQ-CTRL-003 REST API (net/http + JSON)
- POST /api/v1/workflows (201 Created + Location)
- GET /api/v1/workflows/{id} (200/404/400)
- GET /api/v1/workflows (LIST + pagination)
- /healthz 헬스체크 엔드포인트
- 12개 테스트 (httptest 기반)

#### Sprint 6: REQ-CTRL-005 Celery Dispatcher (Kombu v2)
- CeleryDispatcher 구현 (Redis RPUSH)
- Kombu v2 envelope 직렬화 (body/headers/properties)
- base64 인코딩 + JSON 필드 매핑
- 15개 테스트 + 벤치마크 (7μs/op)

#### Sprint 7: E2E Integration (testcontainers postgres + redis)
- 전체 흐름 검증: 생성 → 상태 전이 → Dispatch → 동시성
- 5개 E2E 테스트 (29.3초 실행)
- 유닛 테스트 회귀 검사 완료 (79개 PASS)

**품질 게이트**:
- 95개 테스트 (79 단위 + 11 통합 + 5 E2E) 모두 PASS
- go vet 0 에러 | golangci-lint 0 이슈 | gofmt 100% 준수
- 27개 @MX 태그 (20 ANCHOR + 4 NOTE + 3 WARN)
- plan-auditor 0.91 PASS + evaluator-active 0.872 CONFIRM

**커버리지**:
- 전체: 55.0% (unitprofile 기준)
- 실제 결합 (통합 포함): ~80% (pg_store testcontainers-only 제외)
- WARNING 3개 (모두 정보성, 차단 불가)

**REQ-CTRL 추적성**:
- REQ-CTRL-UBI-001/002 (감시) ✓
- REQ-CTRL-001 (상태 머신) ✓
- REQ-CTRL-002 (gRPC) ✓
- REQ-CTRL-003 (REST) ✓
- REQ-CTRL-004 (PostgreSQL) ✓
- REQ-CTRL-005 (Celery) ✓
- AC-CTRL-E2E-1 (전체 흐름) ✓

### Added — SPEC-AX-001 v0.1.2 Walking Skeleton

#### Sprint 0: 모노레포 스캐폴딩 (commit: 2a3cdec)

- 모노레포 구조 구성 (Python/Go/TypeScript 계층)
- Helm Chart 스켈레톤 (values-dev.yaml, values-prod.yaml)
- Docker Compose 로컬 개발 환경 (PostgreSQL, Redis, vLLM)
- Makefile 기본 타겟 (setup, test, lint, docker-build)
- GitHub Actions CI/CD 파이프라인 (테스트·린트·빌드)

#### Sprint 1: REQ-UBI 기본 요구사항 (commits: c29f17f, 625b214)

- **데이터 주권** (REQ-UBI-001): 외부 LLM API 차단, 자체 호스팅만 허용
  - `pipelines/config/settings.py`: validate_llm_endpoint() 검증
  - K8s NetworkPolicy 기본 정의
  
- **한국어 언어 Enforcement** (REQ-UBI-002):
  - `pipelines/ingestion/language_detector.py`: 한국어만 처리
  - `pipelines/generation/style_applier.py`: 합니다체 검증
  
- **감시 로깅** (REQ-UBI-003):
  - `pkg/logging/logger.py`: AuditLogger, 모든 주요 이벤트 기록
  - `pipelines/config/models.py`: audit_logs 테이블 스키마
  - CLI/API 모드 지원 (user_id='cli-anonymous')

**테스트**: 25개 통과 (audit 4 + language 3 + sovereignty 3 + ... 15개)

#### Sprint 2: REQ-AX-001 Document Ingestion (commits: 83b6343, 3f40e54)

- **HWP 파싱** (`pipelines/ingestion/hwp_parser.py`):
  - OLE 구조 파싱, 텍스트·표·메타데이터 추출
  
- **PDF 파싱** (`pipelines/ingestion/pdf_parser.py`):
  - 텍스트 추출, 회전 페이지 감지 및 정렬
  
- **VLM OCR** (`pipelines/ingestion/vlm_processor.py`):
  - Qwen2-VL 7B via vLLM (fallback when hwp-converter fails)
  - GPU <2sec/page, CPU fallback 5-10배 증가
  
- **테이블 추출** (`pipelines/ingestion/table_extractor.py`):
  - VLM 출력 후처리, 셀 정렬 검증
  
- **비동기 워커** (`pipelines/workers/ingestion_worker.py`):
  - Celery 기반 문서 처리 태스크

**테스트**: 31개 통과 (파싱 15 + OCR 8 + 테이블 5 + worker 3)

#### Sprint 3: REQ-AX-002 Criterion Mapping & RAG (commits: 8f002ae, 00de17b)

- **평가편람 파싱** (`pipelines/mapping/criterion_parser.py`):
  - 항목→지표→배점 계층 추출
  - 기획재정부 편람 호환성
  
- **한국어 임베딩** (`pipelines/mapping/embedding_service.py`):
  - ko-sroberta-multitask (768 dim)
  - 500-1000 token 청킹
  
- **Vector DB** (`pipelines/mapping/vector_store.py`):
  - PostgreSQL 16 + pgvector (HNSW 인덱스)
  - 배치 upsert, 인덱싱 최적화
  
- **RAG 검색** (`pipelines/mapping/retriever.py`):
  - top-3 검색 (relevance >= 0.8)
  - p99 latency < 100ms
  - insufficient_context 처리 (silent skip 금지)

**테스트**: 35개 통과 (파싱 8 + 임베딩 7 + 저장소 9 + 검색 11)

#### Sprint 4: REQ-AX-003 Grade Simulation (commits: 74d4fed, 3bf2bf4)

- **벤치마크 학습** (`pipelines/scoring/benchmark_learner.py`):
  - A/B 등급 보고서 특징 추출
  - 2-class 분류기 학습
  
- **등급 예측** (`pipelines/scoring/grade_predictor.py`):
  - 2-class softmax + abstain 분기
  - max(P(A), P(B)) < 0.5 시 abstain 활성화
  - < 1초 추론 시간
  
- **시나리오 시뮬레이션** (`pipelines/scoring/scenario_simulator.py`):
  - B→A 상향 시나리오 3-5개 생성
  - score_delta 예측

**테스트**: 29개 통과 (학습 6 + 예측 10 + 시뮬 8 + abstain 5)

#### Sprint 5: REQ-AX-004 Report Draft Generation (commits: d8363cd, e219ba2)

- **LLM 클라이언트** (`pipelines/generation/llm_client.py`):
  - EXAONE 3.5 7B via vLLM (primary)
  - Qwen 2.5 7B (fallback after 3 EXAONE failures)
  - 데이터 주권 검증 재확인
  
- **프롬프트 빌더** (`pipelines/generation/prompt_builder.py`):
  - 평가기준·지침·예시 조합 (2000-3000 tokens)
  
- **스타일 검증** (`pipelines/generation/style_applier.py`):
  - 한국 공문 합니다체 강제
  - 반말/존댓말 혼용 감지 → reject & retry (≤3)
  - @MX:ANCHOR (high fan_in)
  
- **초안 생성** (`pipelines/generation/report_drafter.py`):
  - FastAPI 엔드포인트, Celery 워커 연동

**테스트**: 38개 통과 (LLM 호출 9 + 프롬프트 8 + 스타일 12 + 초안 9)

#### Sprint 6: REQ-AX-005 Gap Recommendation (commits: 3a3adda, 3084331)

- **Gap 분석** (`pipelines/recommendation/gap_analyzer.py`):
  - 현재(B) vs 목표(A) 콘텐츠 비교
  - 3-5개 gap 항목 식별
  
- **콘텐츠 제안** (`pipelines/recommendation/content_suggester.py`):
  - 벤치마크 기반 matching
  - 소스 reference 기록
  
- **우선순위 정렬** (`pipelines/recommendation/prioritizer.py`):
  - 실현 가능성 스코어 (0.0~1.0)
  - Priority 1-5 부여

**테스트**: 21개 통과 (gap 분석 8 + 제안 6 + 우선순위 7)

### Quality

- **TRUST 5 게이트** (commit: f909f18):
  - Tested: 82% 커버리지 (목표 85%, SPEC-AX-COV-001 후속)
  - Readable: ruff zero errors (linting + formatting)
  - Unified: black 일관 포맷팅
  - Secured: 외부 LLM API 차단, PII 마스킹 regex
  - Trackable: 17개 RED-GREEN pair commits, 일관된 메시지

- **Plan Auditor PASS**: 0.86 점수 (SPEC-AX-001 v0.1.1 review iteration)
- **Evaluator-Active CONFIRM**: 0.813 점수 (cross-validation 통과)

### Security

- **데이터 주권** (REQ-UBI-001):
  - 외부 LLM 엔드포인트 검증 (`validate_llm_endpoint()`)
  - K8s NetworkPolicy 기본값 (내부 통신만)
  
- **감시 로깅** (REQ-UBI-003):
  - 모든 document/workflow/generation 이벤트 기록
  - user_id='cli-anonymous' (SSO 미구현, sandbox 전용)
  - audit_logs 별도 채널
  
- **PII 마스킹**:
  - 기본 regex (전화번호, 한글 인명 2-4자)
  - 후속 SPEC에서 확장 예정

### Deferred (후속 SPEC)

#### Sprint 7: Go Control Plane 구현
- **SPEC-AX-CTRL-001** 후보
- gRPC(:50051) + REST(:8080) 서버 구현
- 워크플로우 상태 머신, 에이전트 스케줄러
- 현재: 스텁 (protobuf 정의만 완료)

#### Sprint 8: E2E 통합 테스트
- **SPEC-AX-E2E-001** 후보
- Document Ingestion → Recommendation 전체 파이프라인 검증
- Helm 배포 후 실환경 validation

#### Sprint 9: 커버리지 확대
- **SPEC-AX-COV-001** 후보
- 82% → 85% 목표
- integration 테스트 추가

#### 다중 평가항목 확장
- **SPEC-AX-EXPANDED-001** 후보
- 현재: 안전보건 1개 항목만
- 향후: 500개 전체 항목 지원

#### 인접 도메인 (Phase 3)
- **SPEC-AX-ESG-001**: ESG 보고서 자동화
- **SPEC-AX-AUDIT-001**: 감사 보고서 자동화
- **SPEC-AX-LICENSE-001**: 면허신청서 자동화

#### 금융권 도메인 (Phase 4+)
- **SPEC-AX-FINTECH-001**: 금융권 규제 보고서
- 선행 조건: 공공 anchor 성공 사례 3+ 확보
- 망분리 + K-ISMS 인증 트랙

### Documentation

- **Architecture Codemaps**: `.moai/project/codemaps/`
  - overview.md: 전체 시스템 아키텍처
  - pipelines.md: Python 17개 모듈 맵
  - pkg.md: 5개 데이터 모델 + 에러 정의
  - data-flow.md: E2E 시나리오 데이터 흐름
  - req-traceability.md: AC ↔ 구현 ↔ 테스트 매트릭스
  - README.md: codemaps 디렉토리 인덱스

- **README.md 갱신**:
  - 프로젝트 상태: "Walking Skeleton 완료 (Sprint 0-6)"
  - 빠른 시작: pytest 명령 업데이트
  - 기술 스택 표: 최신 버전 반영

---

## Metadata

**최종 상태**: SPEC-AX-001 v0.1.2 Walking Skeleton 완료  
**총 커밋 수**: 17개 (plan 1 + scaffold 1 + sprint 1-6 pair 12 + quality 1 + cleanup 2)  
**총 테스트**: 177개 passing  
**코드 라인 수**: pipelines/ 약 8,000줄  
**모듈 수**: Python 17개 + Go 스텁 + TS 스텁  
**품질**: TRUST 5 + plan-auditor + evaluator-active 검증 완료  

---

**최신 업데이트**: 2026-05-14  
**Project Version**: 0.1.2  
**SPEC Reference**: `.moai/specs/SPEC-AX-001/spec.md`
