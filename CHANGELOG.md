# Changelog

본 프로젝트의 모든 주목할 만한 변경 사항을 이 파일에 기록합니다.

형식은 [Keep a Changelog](https://keepachangelog.com/ko/1.1.0/)을 따르며,
이 프로젝트는 [Semantic Versioning](https://semver.org/lang/ko/)을 준수합니다.

## [Unreleased] - 2026-05-21

### Added — SPEC-AX-INGEST-001 v0.1.0 (Ingestion Worker 실제 구현 — VLM OCR + RAG 임베딩 + Go 채점 트리거)

- **`_execute()` 실제 파이프라인** (`pipelines/workers/ingestion_worker.py`): SPEC-AX-INTEG-001이 남긴 `stub=True` 스텁을 7-Step VLM OCR + RAG + 채점 트리거 파이프라인으로 교체. REQ-INGEST-001~005c 전체 구현.
- **TextChunker** (`pipelines/ingestion/text_chunker.py`, 54 LOC): 문자 기반 슬라이딩 윈도우 청커. `chunk_size=1536, overlap=128` — 외부 토크나이저 의존 없음(REQ-UBI-001 준수).
- **ScoreTrigger** (`pipelines/ingestion/score_trigger.py`, 115 LOC): Go 채점 API fire-and-forget 클라이언트. `POST /api/v1/scores` HTTP 201 확인. Bearer 토큰 헤더(`SCORE_API_TOKEN` 환경변수), 빈값 시 헤더 생략(PoC 샌드박스 모드). 예외 완전 흡수 — Celery ACK 보장(REQ-INGEST-003/003b).
- **DocumentMetadataClient** (`pipelines/ingestion/document_metadata.py`, 60 LOC): Celery 엔벨로프 kwargs에서 `file_path`, `file_type`, `user_id` 추출. fallback: `default_user_id`(REQ-UBI-003, REQ-INGEST-004b).
- **settings.py 확장**: `vlm_timeout_seconds: int = Field(default=120)` + `score_api_token: str = Field(default="")` 추가(OPEN #2/#5 해소).
- **IngestionEmptyError** (`pkg/errors/custom_errors.py`): VLM OCR 결과 빈 텍스트 시 사용자 친화적 오류.
- **모듈 레벨 부팅 검증**: `validate_llm_endpoint(settings.vlm_endpoint)` — Celery worker 시작 시 외부 LLM 차단 강제(REQ-INGEST-005).
- **result_json 강화**: `{document_id, chunks, tokens, score_triggered, ocr_backend, pages_processed, spec}` — stub `{"stub": True}` 교체(REQ-INGEST-004).
- **EC-10 처리**: 청크별 임베딩 실패 → 건너뜀 + WARNING + `failed_chunks: int` 집계(REQ-INGEST-004b 준수).
- **신규 환경변수**: `VLM_TIMEOUT_SECONDS`(VLM OCR 타임아웃, 기본값 120초) · `SCORE_API_TOKEN`(Go 채점 API Bearer 토큰, 빈값 시 PoC 샌드박스 모드).
- **보안 제약 준수**: REQ-UBI-001(외부 LLM 차단) · REQ-UBI-002(한국어 오류 메시지) · REQ-UBI-003(audit user_id='cli-anonymous').
- **신규 단위 테스트 31건**: `test_ingest_text_chunker.py`(9건) · `test_ingest_score_trigger.py`(9건) · `test_ingest_document_metadata.py`(6건) · `test_ingest_execute.py`(7건) — 골든패스/VLM 타임아웃/503/EC-10/한국어 메시지 커버.
- **커버리지**: 88%(신규·수정 모듈 기준, 목표 85% 초과).
- **consumer-only 0-diff [HARD]**: `apps/control-plane/` · `go.mod` · `go.sum` 무변경. Python 전용 변경.
- **INTEG-001 회귀 방지**: `test_integ_001_celery_envelope.py` 목킹 업데이트(stub→pipeline 계약 변경 반영), 26건 GREEN 유지.

---

### Added — SPEC-AX-INTEG-001 v0.1.0 (Python↔Go 통합 — Celery 워크플로우 트리거 및 REST 콜백)

- **Go callback handler** (`apps/control-plane/cmd/server/workflow_callback_handler.go`, 275 LOC): `POST /api/v1/workflows/{id}/callback` — RUNNING→COMPLETED|FAILED 상태 전이(단일 TX: GetWorkflow FOR UPDATE + UpdateWorkflowState + UpdateWorkflowResult + InsertAuditLog). 204 성공 / 400 잘못된 본문 또는 상태 / 404 워크플로우 없음 / 409 비-RUNNING 상태(terminal state 거부). audit user_id='cli-anonymous'(REQ-UBI-003). 한국어 에러 메시지(REQ-UBI-002).
- **server.go 마운트**: `callbackH *WorkflowCallbackHandler` 필드 추가 + `/api/v1/workflows/` 마운트. Go 14 단위 테스트 PASS.
- **Python Celery worker** (`pipelines/workers/ingestion_worker.py`, 107 LOC): Celery task `pipelines.workers.ingestion_worker.run` — Kombu v2 엔벨로프 수신, 스텁 처리, callback POST 전송. 부팅 시 `validate_worker_environment_from_env()` 환경변수 검증.
- **callback 클라이언트** (`pipelines/callbacks/control_plane.py`, 119 LOC): `post_callback()` fire-and-forget HTTP POST via httpx. 2xx 성공 / 3xx 차단 / 4xx/5xx 경고 로깅. 콜백 실패 시 Celery 태스크 상태에 영향 없음(fire-and-forget).
- **Celery 앱 초기화** (`pipelines/config/celery_client.py`, 98 LOC): `validate_worker_environment()` + `create_celery_app()`. `GO_CONTROL_PLANE_URL` 빈값 시 Celery worker 시작 거부(REQ-INTEG-006).
- **settings.py 확장**: `go_control_plane_url` 필드(기본값="", fail-fast 검증). `vlm_endpoint` validation_alias VLLM_ENDPOINT 수정.
- **신규 환경변수**: `GO_CONTROL_PLANE_URL`(Go 콜백 URL, 미설정 시 worker 시작 거부) · `VLLM_ENDPOINT`(vLLM 엔드포인트 localhost/127.0.0.1/::1 허용 목록).
- **보안 제약 준수**: REQ-UBI-001(외부 LLM 차단 — VLLM_ENDPOINT localhost-only 허용 목록) · REQ-UBI-002(한국어 에러 메시지) · REQ-UBI-003(audit_log user_id='cli-anonymous').
- **Python 테스트**: 26 단위 테스트(startup/validation 14 + callback 복원력 6 + Kombu 엔벨로프 6) PASS.
- **consumer-only 0-diff [HARD]**: `internal/auth`·`internal/store`·`internal/errors`·`rbac.go`·schema·go.mod 무변경.
- **evaluator-active**: TRUST 5 PASS 0.912 (iter-2). 4-차원: Functionality / Security / Craft / Consistency.

---

### Added — SPEC-AX-PIPE-001 v0.1.0 (Python AI 파이프라인 REST API 계층)

- **FastAPI 7개 엔드포인트** (`pipelines/main.py`): `POST /api/documents/upload` (문서 업로드·VLM 처리) · `POST /api/criteria/index` (평가기준 인덱싱) · `GET /api/criteria/search` (유사도 검색) · `POST /api/simulations/predict` (등급 시뮬레이션) · `POST /api/reports/generate` (보고서 초안 생성) · `POST /api/recommendations/generate` (Gap 추천 생성) · `PATCH /api/recommendations/{id}/feedback` (피드백 반영). BackgroundTasks D7 패턴으로 Celery 의존 없는 비동기 처리.
- **Pydantic 모델 17종** (`pipelines/config/models.py`): Sprint 2–6 입출력 스키마. `DocumentUpload` · `CriterionIndex` · `CriterionSearch` · `SimulationPredict` · `ReportGenerate` · `RecommendationGenerate` 계열 요청/응답 모델 + 기반 설정 모델.
- **신규 테스트 31건**: `tests/unit/test_config_models.py` 22건(Pydantic 모델 직렬화·유효성 검사) · `tests/unit/test_main_endpoints.py` 9건(FastAPI TestClient 통합). 기존 `FakeVectorStore` · `VLMProcessor` · `ScenarioSimulator` 커버리지 파일 3개 확장.
- **커버리지**: 91.49% (목표 85% 초과)
- **TRUST 5 게이트**: evaluator-active PASS 0.933 (Tested 0.95 / Security 0.90 / Craft 0.92 / Consistency 0.94)
- **consumer-only 0-diff [HARD]**: `apps/control-plane/` · `apps/web/` · `internal/` · `go.mod` · `.moai/db/` 무변경. Python 파이프라인 계층 신규 구현 전용.

---

### Added — SPEC-AX-E2E-001 v0.4.0 (Playwright E2E 테스트 슈트)

- **21개 Playwright E2E 테스트 케이스** (`apps/web/e2e/`, 5개 spec 파일): `auth.spec.ts`(인증 흐름 5건) · `rbac-viewer.spec.ts`(Viewer 권한 제한 5건) · `flow-analyst.spec.ts`(Analyst 골든패스 4건) · `flow-admin.spec.ts`(Admin 골든패스 4건) · `logout.spec.ts`(로그아웃 3건). `npx playwright test --list` 21 tests discovered PASS.
- **역할별 Mock JWT storageState 픽스처** (`e2e/global-setup.ts`, `e2e/fixtures/auth.ts`): Keycloak 의존 없이 viewer/analyst/admin 역할별 `ax_access_token` HttpOnly 쿠키 직접 주입. `decodeJwt(jose)` 서명 미검증 특성 활용. `e2e/.auth/{viewer,analyst,admin}.json` storageState 사전 생성.
- **BFF API 이중 모드 목** (`e2e/fixtures/api-mocks.ts`): `E2E_LIVE_BACKEND=1` 시 실제 BFF 통과, 미설정 시 `page.route` 7개 도메인 JSON fixture 자동 인터셉트. 6개 fixture 파일(`e2e/fixtures/api/*.json`): evidences · evaluation-items · scores · reviews · audit-logs · rubric-thresholds.
- **한국어 텍스트 셀렉터** (`e2e/selectors.ts`): SUT 실제 DOM 텍스트 기반 셀렉터 상수. SUT 텍스트 변경 시 단일 파일만 수정.
- **기지 차단(pre-existing SUT 결함)**: `apps/web/next.config.ts` + Next.js 14.2.18 비호환(`loadConfig`가 `.ts` 설정 거부) → `npm run dev` 미기동 → `npm run test:e2e` 실행 불가. 구조 검증(`--list` PASS) 및 0-diff는 완료. 실행 차단 해소는 **SPEC-AX-WEB-003** 별도 처리 예정.
- **consumer-only 0-diff [HARD]**: `apps/control-plane/` · `apps/web/src/` · `pipelines/` 무변경.

---

### Added — SPEC-AX-WEB-001 v0.1.0 (PoC 데모 웹 대시보드 프런트엔드)

- **Next.js 14+ App Router 워크스페이스** (`apps/web/`): TypeScript 5.4+ strict 모드, shadcn/ui + Tailwind CSS 3.4+, TanStack Query v5. 루트 `package.json` `workspaces: ["apps/web"]` 갱신.
- **BFF HttpOnly 쿠키 인증 레이어** (Phase A, `app/api/auth/[...]/route.ts` 4개 Route Handler): Keycloak 24.x PKCE/S256 OIDC 콜백 처리, 토큰 교환(`POST /api/v1/auth/token`), 자동 갱신(`POST /api/v1/auth/refresh`), 로그아웃(`POST /api/v1/auth/logout`), me 엔드포인트. `ax_access_token`·`ax_refresh_token` HttpOnly·Secure·SameSite=Lax 쿠키에 저장 — 클라이언트 JS 토큰 직접 접근 불가(XSS 방어).
- **Edge 미들웨어 라우트 가드** (`middleware.ts`): `/dashboard/**` 미인증 접근 시 `/login` 리다이렉트. `RoleGate` 컴포넌트로 viewer/analyst/admin RBAC 조건부 렌더링. `Sidebar` 7개 네비게이션 항목 역할별 가시성 제어.
- **Keycloak realm-export.json 갱신** (`deployments/keycloak/realm-export.json`): OIDC Public Client(`iroum-ax-web`) 추가 — PKCE, redirect URI `http://localhost:3000/api/auth/callback` (백엔드 0-diff 예외, OPEN #2 RESOLVED).
- **증빙 관리 UI** (Phase B, 7파일): `GET /api/v1/evidences` 목록(pagination), `POST /api/v1/evidences` multipart 업로드 프록시(100MB 클라이언트 가드), `GET /api/v1/evidences/{id}` 상세. 드래그-드롭 업로드 드롭존(analyst/admin 한정), 페이지네이션 목록, 상세 모달.
- **평가항목 트리 + 점수 입력 UI** (Phase C, 13파일): `parent_id` 기반 플랫→트리 클라이언트 재구성(고아 항목 fail-soft 승격), code 자연 정렬. 2-패널 레이아웃(접이식 트리 좌 + 상세/점수입력 폼 우). 점수 입력/수정 폼 RoleGate analyst/admin 한정. BFF: 평가항목 목록/상세, 점수 목록/생성/상세/수정.
- **범주 리포트 뷰** (Phase D, 6파일): `GET /api/v1/reports/category/{id}` 프록시. 범주 선택 드롭다운, 등급 배지(A→E, 초록→빨강). SSR 초기 범주 목록 로드.
- **리뷰 워크플로 Kanban 보드** (Phase E, 11파일): CSS-only 4-컬럼 Kanban(SUBMITTED/UNDER_REVIEW/APPROVED/REJECTED). 리뷰 제출(analyst/admin), 리뷰어 배정·승인·반려(admin) 인라인 폼. terminal 상태 카드 액션 버튼 비표시.
- **감사 로그 뷰어 + 루브릭 설정** (Phase F, 10파일): 감사 로그 7개 쿼리 파라미터 허용 목록 + 페이지네이션. 루브릭 임계값 인라인 편집 + 신규 생성. `[scope]` 경로 순회 가드(`^[A-Za-z0-9:_-]{1,64}$` 정규식). Admin 전용 RSC 가드(역할 검사 렌더링 전 차단).
- **공통 횡단 구현**: 한국어 정적 메시지 사전(`lib/i18n/ko.ts`), ApiError 표준화 fetch wrapper, 401 자동 refresh 1회 + 실패 시 `/login` redirect, 403 한국어 toast, 로딩 skeleton.
- **consumer-only 0-diff [HARD]**: `apps/control-plane/**`·`go.mod`·`pyproject.toml`·`pipelines/**` 무변경. 31 endpoints 사용(phantom endpoint 0건).

---

## [Unreleased] - 2026-05-20

### Added — SPEC-AX-REVIEW-001 v0.1.1 (평가 제출/승인 워크플로우 store + HTTP API 수직 슬라이스)

- **6개 REST 엔드포인트** (`apps/control-plane/cmd/server/review_handlers.go`): `POST /api/v1/reviews` (검토 요청 생성, 201) · `GET /api/v1/reviews` (목록 조회) · `GET /api/v1/reviews/{id}` (단건 조회) · `POST /api/v1/reviews/{id}/assign-reviewer` (검토자 배정) · `POST /api/v1/reviews/{id}/approve` (승인) · `POST /api/v1/reviews/{id}/reject` (반려). Go1.22 ServeMux 최장일치 라우팅(`score_handlers.go` 선례 미러).
- **ReviewHandler** (`cmd/server/review_handlers.go`): `ReviewHandler` struct + `NewReviewHandler(reviewStore ScoreReviewRequestStore, scoreStore ScoreStore, logger)` + `Routes() http.Handler`. cross-store 2-TX 패턴(TX-1: scoreStore read-only score 존재 검증, TX-2: reviewStore write 상태 전이), `SELECT FOR UPDATE` 비관적 락(동시 전이 중복 방지), 4-state machine(`SUBMITTED→UNDER_REVIEW→APPROVED/REJECTED` 단방향 비가역), `resolveCreatedBy(r)` → `userID` 파라미터 영속화(`BeginScoreReviewRequestTx` 서명), 표준 에러 본문 `{"error":{"code","message","field"}}`.
- **server.go 마운트** (`cmd/server/server.go`, ≈7줄 최소 단위): `reviewH` 필드 + `NewReviewHandler(pgStore, pgStore, logger)` + `innerMux.Handle("/api/v1/reviews", ...)` + `innerMux.Handle("/api/v1/reviews/", ...)` 2줄. 기존 `RESTAuthzMiddleware` 와이어링 자동 적용, ABAC 와이어링 0-diff(analyst=submit, admin=assign/approve/reject, 전 role=read).
- **score_review_requests 테이블** (`.moai/db/schema/migrations/0005_score_review_request_tables.sql`): 11컬럼(`id UUID PK`, `score_id UUID`, `status VARCHAR(32) DEFAULT 'SUBMITTED'`, `assigned_reviewer_id VARCHAR(128)`, `rejection_reason TEXT`, `comment TEXT`, `metadata JSONB`, `created_at/updated_at TIMESTAMPTZ`, `created_by/updated_by VARCHAR(128) DEFAULT 'cli-anonymous'`). CHECK 제약 2종(`status` 열거형 4값, `rejection_reason` REJECTED 시 필수). 인덱스 3개(`score_id`, `status`, `created_at DESC`).
- **consumer-only 0-diff [HARD]**: `internal/store|audit|auth|errors`·`score_handlers.go`·`evidence_handlers.go`·`report_handlers.go`·기존 마이그레이션 무변경. 신규 DB 마이그레이션 1건(0005)·신규 외부 의존 0건.
- **TDD GAN iter2 PASS**: iter1 FAIL 73.5 (Must-Pass UBI-003 위반 — D1 `NewRecorder(false)` 반환으로 auth-enabled 시 `userID`가 항상 `'cli-anonymous'`로 고정) → iter2 PASS 92.8 (D1 root cause fix: `NewRecorder(true)` 전환 + `userID` 파라미터 전파). T-111 DB-level CHECK(false) audit fault rollback(EVAL-ITEM-001 동형 패턴). 이중 게이트 PASS: evaluator-active 92.8 / manager-quality TRUST 5 PASS. integration 14/14(testcontainers).

---

## [Unreleased] - 2026-05-19

### Added — SPEC-AX-REPORT-001 v0.1.1 (경영평가 결과 리포트/집계 HTTP API 계층)

- **단건 REST 엔드포인트 1개** (`apps/control-plane/cmd/server/report_handlers.go`): `GET /api/v1/reports/category/{id}` — 범주별 집계 리포트(범주 id·name, 자식 item별 `weighted_sum`, `category_total`, `category_grade` string|null, `generated_at`). 목록/페이지네이션 미적용(PoC 단건만 — §6.3 OPEN #3 RESOLVED B-2).
- **ReportHandler** (`cmd/server/report_handlers.go`): `ReportHandler` struct + `NewReportHandler(ss ScoreStore, eis EvalItemStore, logger)` + `Routes() http.Handler`. cross-store 2-TX read 조합(EvalItemTx: `GetEvalItemByID`+`GetEvalItemsByParentID` / ScoreTx: `SumWeightedByEvaluationItem`×N+`DetermineGrade`), `math/big.Rat` 무손실 누적(float64 미경유 SEC-03), `ErrGradeThresholdsUnavailable`→`category_grade:null` B-2 graceful. 표준 에러 본문 `{"error":{"code","message","field"}}` 한국어(`score_handlers.go` 선례 미러). recorder 미주입 — read-only·mutation 0·자체 audit 0(REQ-REPORT-UBI-002).
- **server.go 마운트** (`cmd/server/server.go`, ≈7줄 최소 단위): `reportH` 필드 + `NewReportHandler(pgStore, pgStore, logger)` + `innerMux.Handle("/api/v1/reports", ...)` + `innerMux.Handle("/api/v1/reports/", ...)` 2줄. Go1.22 ServeMux path-param 라우팅 구조적 필수(`score_handlers.go:263-264` 선례 정확 미러). ABAC 와이어링 0-diff.
- **consumer-only 0-diff**: `internal/store|audit|auth|errors`·`score_handlers.go`·`evidence_handlers.go`·`.moai/db/schema/**` 무변경. 신규 DB 마이그레이션 0건·신규 store 메서드 0건·신규 외부 의존 0건(go.mod 핀 `github.com/jackc/pgx/v5 v5.9.2` 유지, `math/big` stdlib). read-only·API 자체 audit 0건.
- **TDD RED-GREEN-REFACTOR**: genuine RED-first(D-1 negative-control mutation-tested 포함). 이중 게이트 PASS: evaluator-active 90.8 / manager-quality TRUST 5 PASS. `report_handlers.go` 커버리지 100%.

### Added — SPEC-AX-SCORE-API-001 v0.1.1 (경영평가 점수 조회/집계 HTTP API 계층)

- **7개 REST 엔드포인트** (`apps/control-plane/cmd/server/score_handlers.go`): `GET /api/v1/scores/{id}` (단건 조회) · `GET /api/v1/scores` (목록, filter+pagination) · `GET /api/v1/scores/rollup` (가중 롤업, pgtype.Numeric 정밀도) · `GET /api/v1/scores/grade` (등급 조회) · `POST /api/v1/scores` (생성, 201) · `PUT /api/v1/scores/{id}` (수정, CONFIRMED 불변 409) · `POST /api/v1/scores/{id}/supersede` (CONFIRMED 정정, 201). Go1.22 ServeMux 최장일치 라우팅.
- **ScoreHandler** (`cmd/server/score_handlers.go`): `ScoreHandler` struct + `NewScoreHandler(store, logger)` + `Routes() http.Handler`. 핸들러-로컬 ABAC write-role 게이팅(`requireScoreWriteRole`, write={RoleAdmin,RoleAnalyst}), store 에러 센티넬→HTTP 결정적 매핑(`mapStoreErr`), TX orchestration(BeginScoreTx→Commit, defer Rollback committed-flag), pagination clamp(default=50, max=500), 표준 에러 본문 `{"error":{"code","message","field"}}`.
- **server.go 마운트** (`cmd/server/server.go`, ≈7줄 최소 단위): `scoreH` 필드(L55) + `NewScoreHandler` 생성자(L209) + `innerMux.Handle` 2줄(L263-264: `/api/v1/scores` + `/api/v1/scores/` 서브트리). 기존 `RESTAuthzMiddleware` 와이어링 자동 적용, ABAC 와이어링 0-diff.
- **consumer-only 0-diff**: `internal/store|audit|auth|errors`·`evidence_handlers.go`·`.moai/db/schema/**` 무변경. 신규 DB 마이그레이션 0건 (순수 API 계층 — `scores`/`grade_thresholds`는 SPEC-AX-SCORE-001이 제공). API 자체 audit 0건 (store `RecordScore*` 동일 TX 전담).
- **TDD RED-GREEN-REFACTOR**: 커밋 8a61193. evaluator-active PASS 0.9235 / manager-quality TRUST 5 PASS / 커버리지 95.79%.

### Added — SPEC-AX-SCORE-001 v0.1.3 (경영평가 점수 산출/집계 Walking Skeleton)

- **점수 데이터 모델** (`scores` + `grade_thresholds` 2테이블, `.moai/db/schema/migrations/0004_score_tables.sql`): Decision 1 Option A — 단일 `scores` 테이블 + `level` discriminator(`raw`/`item`/`category`). `id UUID PK DEFAULT uuid_generate_v4()`, `evaluation_item_id VARCHAR(64)` (FK 없는 stub, EVAL-ITEM-001 호환), `evidence_id UUID nullable` (FK 없는 stub, EVID-001 호환), `score_value DECIMAL(6,2)`, `weight DECIMAL(5,4) NULL` (NULL-weight policy: exclude, GAP-01), `grade VARCHAR(2) NULL`, `status VARCHAR(32) DEFAULT 'DRAFT'` CHECK(`DRAFT`/`CONFIRMED`/`SUPERSEDED`, D4 state-machine), `metadata JSONB`, `created_by DEFAULT 'cli-anonymous'`. CHECK 제약 3종(`level`, `status`, `grade`), 인덱스 3개. `grade_thresholds`(scope, letter, min_value, boundary_rule, PK(scope,letter)) — Decision 3, 최소 등급 임계값 테이블(풀 rubric 아님).
- **ScoreStore / ScoreTx 계층** (`internal/store/store.go`, `internal/store/score.go`): `ScoreStore` 인터페이스(`BeginScoreTx`) + `ScoreTx` 인터페이스 + `PgScoreTx` 구현체. 7 메서드: `InsertScore`(DRAFT 생성+audit), `GetScoreByID`, `GetScoresByEvaluationItem`, `UpdateScore`(D4 CONFIRMED 불변 가드+status 전이 검증+audit), `SupersedeAndReplaceScore`(CONFIRMED 정정: 신규 INSERT + SUPERSEDED UPDATE + 2 audit, append-only), `SumWeightedByEvaluationItem`(pgtype.Numeric 정밀도, SEC-03), `DetermineGrade`(grade_thresholds 결정적 스캔, fail-closed). `InsertAuditLog`, `Commit`, `Rollback` 포함.
- **BeginScoreTx pool 재사용** (`internal/store/pg_store.go`): `PgWorkflowStore.pool` 단일 pgx 풀 재사용 — 신규 풀 연결 0건 (SPEC-AX-CTRL-001 / SPEC-AX-EVID-001 / SPEC-AX-EVAL-ITEM-001 동일 패턴).
- **감사 Recorder 확장** (`internal/audit/recorder.go`): `RecordScoreCreated(ctx, tx AuditTx, scoreID uuid.UUID, evaluationItemID, level, userID string)` / `RecordScoreUpdated(...)` 추가. Decision 2 — resource_id = `scores.id` UUID 직접 대입(surrogate 불필요, EVAL-ITEM-001과 달리 UUID PK).
- **액션 상수 2종** (`internal/audit/audit.go`): `ActionScoreCreated = "SCORE_CREATED"`, `ActionScoreUpdated = "SCORE_UPDATED"` 추가. 신규 namespace 상수 0건.
- **에러 센티널 6종** (`internal/errors/errors.go`): `ErrScoreNotFound`, `ErrScoreInvalidInput`, `ErrScoreImmutable`, `ErrScoreInvalidStatus`, `ErrGradeThresholdsUnavailable`, `ErrScoreAuditWriteFailed`, `ErrScoreNotConfirmed` (실질 7종, 연산 단위 별 명확한 구분).
- **Walking Skeleton 범위**: 데이터 모델 + store 계층 + audit 연계 — **HTTP 엔드포인트 없음, REST/gRPC 핸들러 없음, cmd/server 변경 없음.**
- **GAN 평가**: iter1 FAIL 46.25 → iter2 PASS 85.25 (DC-UBI-002 audit 원자성·DC-UBI-004 CONFIRMED 불변·SEC-03 pgtype.Numeric 3건 해소); 통합 테스트 `ok store 278.986s`, 커버리지 87.2%; evaluator-active PASS 85.25/100

### Deferred — SPEC-AX-SCORE-001

- HTTP CRUD 엔드포인트 / REST API (점수 생성·조회·집계·등급) — 후속 SPEC
- 풀 집계 엔진 (깊은 재귀 롤업, score_aggregates 영속, incremental 집계) — 후속 SPEC
- 풀 등급기준(scoring rubric) 시스템 (룰 엔진, 가점/감점, 계층) — 후속 SPEC
- `scores.evaluation_item_id → evaluation_items(id)` / `scores.evidence_id → evidences(id)` FK 하드닝 — 후속 SPEC
- LLM 등급 시뮬레이션 / Recommendation 엔진 — 후속 Python/AI 파이프라인 SPEC

---

## [Unreleased] - 2026-05-18

### Added — SPEC-AX-EVAL-ITEM-001 v0.1.3 (경영평가 평가항목 taxonomy Walking Skeleton)

- **평가항목 데이터 모델** (`evaluation_items` 테이블, `.moai/db/schema/migrations/0003_eval_item_tables.sql`): Option A 자기참조 adjacency list 단일 테이블. `id VARCHAR(64) PK` (계층 코드 형태, e.g. `AX-SAFETY-ORG-01`), `parent_id VARCHAR(64) REFERENCES evaluation_items(id) ON DELETE RESTRICT` (root = NULL), `hierarchy_code VARCHAR(128) NOT NULL UNIQUE`, `display_name VARCHAR(256) NOT NULL`, `level INT NOT NULL`, `status VARCHAR(32) DEFAULT 'ACTIVE'` CHECK(`ACTIVE`,`DEPRECATED`,`ARCHIVED`), `metadata JSONB`. 인덱스 3개 (`evaluation_items_parent_id_idx`, `evaluation_items_hierarchy_code_idx`, `evaluation_items_created_at_idx`). **단일 테이블 — 추가 테이블 없음.**
- **EvalItemStore / EvalItemTx 계층** (`internal/store/store.go`, `internal/store/eval_item.go`): `EvalItemStore` 인터페이스 (`BeginEvalItemTx`) + `EvalItemTx` 인터페이스 (`InsertEvalItem`, `GetEvalItemByID`, `GetEvalItemsByParentID`, `UpdateEvalItem`, `InsertAuditLog`, `Commit`, `Rollback`). `PgEvalItemTx` 구현체 — `validateStatusTransition` / `checkHierarchyMutationGuard` / `buildEvalItemUpdateSet` 3-헬퍼 분리(M1 리팩터). `EvalItemUpdate`는 포인터 필드(`Status *string`, `Metadata *map[string]any`)로 부분 업데이트 지원.
- **BeginEvalItemTx pool 재사용** (`internal/store/pg_store.go`): `PgWorkflowStore.pool` 단일 pgx 풀 재사용 — 신규 풀 연결 0건 (SPEC-AX-CTRL-001 / SPEC-AX-EVID-001 `BeginWorkflowTx` / `BeginEvidenceTx` 동일 패턴).
- **감사 Recorder 확장** (`internal/audit/recorder.go`): `RecordEvalItemCreated` / `RecordEvalItemUpdated` 추가 — AUD-1 결정적 UUIDv5 surrogate: `resource_id = uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))`. 실 식별자(`eval_item_id`, `hierarchy_code`, `parent_id`, `level`)는 `DetailsJSON`에 저장. `resource_id`는 원시 계층 코드가 아닌 UUIDv5.
- **EvalItemAuditNamespace** (`internal/audit/audit.go`): `var EvalItemAuditNamespace = uuid.MustParse("a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b")` — AUD-1 불변식 컴파일 타임 상수 (@MX:ANCHOR). `ActionEvalItemCreated = "EVAL_ITEM_CREATED"`, `ActionEvalItemUpdated = "EVAL_ITEM_UPDATED"` 액션 상수 추가.
- **에러 센티널 5종** (`internal/errors/errors.go`): `ErrEvalItemNotFound`, `ErrEvalItemInvalidInput`, `ErrEvalItemParentNotFound`, `ErrEvalItemHierarchyImmutable`, `ErrEvalItemInvalidStatus` 추가적 합산 (기존 sentinel 비변경).
- **Walking Skeleton 범위**: 데이터 모델 + store 계층 + audit 연계 — **HTTP 엔드포인트 없음, REST/gRPC 핸들러 없음, cmd/server 변경 없음.**
- **커버리지**: `eval_item.go` 86.2% (목표 85%+ 충족); TDD RED-GREEN-REFACTOR 방법론
- evaluator-active PASS — Functionality 96 / Security 95 / Craft 82 / Consistency 97; plan-auditor PASS 0.955

### Deferred — SPEC-AX-EVAL-ITEM-001

- HTTP CRUD 엔드포인트 / REST API — 후속 SPEC
- Console UI / 평가편람 HWP·PDF import — 후속 SPEC
- `evidences.evaluation_item_id` FK 하드닝 (EVID-001 코드 변경 포함) — 후속 SPEC

---

### Added — SPEC-AX-EVID-001 v0.1.0 (경영평가 증빙 자료 수집/관리)

- **증빙 데이터 모델** (`evidences` 테이블, `.moai/db/schema/migrations/0002_evidence_tables.sql`): `id UUID PK`, `evaluation_item_id VARCHAR(64)` (FK 제약 없음), `version INT`, `previous_version_id UUID` 자기 참조, `file_content BYTEA` (database_blob 전략 시 바이너리 저장 컬럼), `storage_location VARCHAR(255)`, `storage_strategy VARCHAR(32)`, `file_hash_sha256`, `created_by DEFAULT 'cli-anonymous'` 등. 인덱스 2개 (`evidences_eval_item_version_idx`, `evidences_created_at_idx`).
- **단일 증빙 엔드포인트** (`POST /api/v1/evidences`, `cmd/server/evidence_handlers.go`): 증빙 생성(version=1)과 버전 업(version+1)을 단일 핸들러 `handleCreateEvidence`로 통합. multipart 수신 → Content-Type/Content-Length 사전 검증 → SHA-256 단일 패스 스트리밍 → pre-TX 입력 검증 → `BeginEvidenceTx` → `SELECT FOR UPDATE` 버전 결정 → `InsertEvidence(file_content)` → 감사 기록 → Commit → 201 `{evidence_id, version}` 반환.
- **저장 전략 추상화** (`internal/storage/storage.go`): `EvidenceBlobStore` 인터페이스 + `dbBlobStore` 구현체. database_blob 전략에서 blob bytes는 이 인터페이스를 통과하지 않으며 `EvidenceTx.InsertEvidence(file_content)`로 동일 pgx TX에 저장; `dbBlobStore.Put`은 논리 위치 문자열 `db://evidences/<uuid>` 만 반환 (외부 SaaS SDK 의존 0건 — REQ-EVID-UBI-001 망분리 정합).
- **감사 Recorder 확장** (`internal/audit/recorder.go`): `RecordEvidenceCreated` / `RecordEvidenceVersioned` 메서드 추가 — 각각 `EVIDENCE_CREATED`, `EVIDENCE_VERSIONED` 액션으로 동일 AuditTx 내 audit_logs 원자 기록 (REQ-EVID-UBI-002).
- **Clock 주입 추상화** (`internal/audit/clock.go`): `Clock` 인터페이스 + `systemClock` 기본 구현 — 증빙 감사 시각 검증을 위한 테스트 친화 구조.
- **환경 변수 3종** (`internal/config/config.go`): `EVIDENCE_STORAGE_STRATEGY` (기본 `database_blob`), `EVIDENCE_MAX_FILE_BYTES` (기본 50 MiB), `EVIDENCE_DUPLICATE_SIGNAL_ENABLED` (기본 `false`). `Validate()` / `LoadConfig()`로 fail-fast 열거 검증.
- **에러 센티널** (`internal/errors/errors.go`): `ErrEvidenceNotFound`, `ErrEvidenceImmutable` 추가 (GAP-03/04 해소).
- **TDD 기반 구현**: evidence-core 커버리지 91.4%, 신규 테스트 다수 (store/audit/handler 각 파일 분리).
- evaluator-active Phase 2.8a 재평가 PASS 0.930

### Fixed — SPEC-AX-EVID-001

- GAP-01 (`POST /api/v1/evidences` 단일 라우트로 생성+버전 통합): 해소
- GAP-03/04 (`ErrEvidenceNotFound`, `ErrEvidenceImmutable` 센티널): 해소
- database_blob 전략 RESOLVED (plan.md §6): 외부 저장소 의존 없는 pgx TX 내 BYTEA 직접 저장으로 확정

### Known — SPEC-AX-EVID-001 범위 외 기지 항목

- `TestE2E_GRPC_Authz_ViewerForbidden_Create`: SPEC-AX-AUTH-002/SERVER-001 범위의 pre-existing 실패, 본 SPEC 범위 밖

---

### Added — SPEC-AX-AUTH-003 v0.1.0 (경량 ABAC — 속성 기반 접근 제어)

- **ABACEvaluator** (`internal/auth/abac.go`): RBAC 위에 속성 기반 접근 제어 레이어; `authn → authz(RBAC) → ABAC → handler` 체인 (chain.go 무변경)
- **OwnershipCondition**: X-Resource-Owner 헤더 기반 문서 소유권 검사; 비소유자 접근 시 `ABAC_CONDITION_DENIED` 403 반환
- **OrgUnitCondition**: scope 토큰 `iroum-ax-org:<unit>` 기반 조직 단위 격리; 교차 조직 접근 차단 (비-Admin)
- **TimeWindowCondition**: KST 09:00–18:00 업무 시간 제한; `time.FixedZone("KST",9*3600)` 강제 (time.LoadLocation 금지, 망분리 정합)
- **Admin bypass**: `RoleAdmin` 감지 시 모든 ABAC 조건 우회 (REQ-ABAC-004)
- **Fail-safe no-op**: 정책 미정의·조건 오류 시 ALLOW + 로그 (REQ-ABAC-009); 기본 정책 = 빈 집합
- **ActionABACDenied**: `internal/audit/audit.go`에 Sprint 0 D5 상수 추가
- **ABACMiddleware**: `cmd/server/server.go` REST mux 래핑 (BuildRESTChain 내부 무변경)
- 30 AC 검증, abac.go 98.5% 커버리지, evaluator-active PASS 0.905 (Func 0.92/Sec 0.90/Craft 0.92/Cons 0.88)
- plan-auditor PASS 0.93 (iter 2) — EARS 30 AC, 9 REQ (망분리/frozen/fail-safe)

### Fixed — SPEC-AX-AUTH-003

- AUTH-002 §6 Excl #4 (ABAC 속성 조건): 해소 (OwnershipCondition + OrgUnitCondition + TimeWindowCondition 구현)

### Deferred — SPEC-AX-AUTH-003

- `audit.Recorder.LogForbiddenEvent` 운영 구현 (AC-007-3 정상 경로 활성화) — Sprint 2
- 자원별 ABAC 정책 추가 (`DefaultABACPolicies` 현재 빈 집합) — Sprint 2
- OwnershipCondition X-Resource-Owner 헤더 기본 파서 배선 + 입력 bound — Sprint 2
- gRPC endpoint ABAC 적용 — 별도 SPEC 검토

---

### Added — SPEC-AX-OBS-001 v0.1.2 (Prometheus Metrics + OpenTelemetry Tracing Skeleton)
- **Metrics Registry**: `prometheus/client_golang` 기반 레지스트리 싱글톤 (`internal/metrics/registry.go`)
- **7개 core collector**: `iroum_ax_http_{requests,latency}`, `iroum_ax_grpc_{requests,latency}`, `iroum_ax_workflow_state_transitions_total`, `iroum_ax_auth_rejections_total{reason}`, `iroum_ax_authz_forbidden_total{permission}`, `iroum_ax_celery_tasks_{total,latency}`, `iroum_ax_pg_pool_connections` (GaugeFunc)
- **`/metrics` Endpoint + RBAC**: `read:metrics` 권한(OBS 자체 permission registry) + `MetricsAuthMiddleware` (authn: `auth.TokenValidator.Verify`, authz: `metrics.IsMetricsAuthorized`, 401/403 분리)
- **HTTP Instrumentation**: `MetricsMiddleware` (chain 최외곽 — probe/metrics 경로 제외, REQ-OBS-003-S1)
- **gRPC Instrumentation**: `UnaryMetricsInterceptor` (chain 최외곽 — 인증 실패도 계측)
- **Dependency Inversion (circular import 영구 해소)**: `internal/auth/observer.go` — `RejectionObserver interface` 선언; `auth.TokenValidator.Verify` → observer hook; `cmd/server/server.go` DI wire point
- **OTel Tracing Skeleton**: `internal/observability/tracer.go` — `InitTracerProvider(cfg)` + AlwaysSample + noop exporter (망분리 정합; OTLP exporter Sprint 3 deferred)
- **Authz mapping**: `authz_mapping.go`에 `GET /metrics → read:metrics` 추가
- **Scheduler/Workflow 계측**: `dispatcher.go` celery task counter + `state_machine.go` workflow transition counter (직접 import — no cycle)
- 24/24 AC GREEN, evaluator-active CONFIRM 89.0 (3 rounds), metrics 87.2% / observability 100%

### Quality (OBS-001)
- **plan-auditor**: iter 1 PASS 0.97 (CONFIRM — spec-to-code 0 contradictions)
- **evaluator-active**: R1 0.67.8 → DISPUTE 8건 → R2 79.0 → 3건 → R3 CONFIRM 89.0 (24/24 AC GREEN)
- **TRUST 5**: [PASS] Tested 0.87 | Readable 0.90 | Unified 0.88 | Secured 0.87 | Trackable 0.85

### Fixed (OBS-001)
- SPEC-AX-AUTH-002 v0.1.2 §5 Exclusion #13 공식 해소 (`read:metrics` 권한 매핑)
- SPEC-AX-SERVER-001 v0.1.2 §5 Exclusion #4·#5 공식 해소 (OTel + `/metrics` endpoint)
- `auth → metrics → auth` circular import 영구 제거 (Dependency Inversion Pattern)

### Deferred (後続 SPEC — OBS-001 범위 외)
- OTLP exporter wire (Sprint 3 — 망분리 환경 설계 후 진행)
- e2e_test.go goRedisAdapter cleanup (redis_adapter.go 통합 후 중복 제거)
- dispatcher_test.go:549 pre-existing race 수정

---

### Added — SPEC-AX-SERVER-001 v0.1.2 (Server Bootstrap + Dual Listener)
- **Sprint 0**: PgWorkflowStore.Ping + JWKSCache.Reachable + redis_adapter.go (goRedisAdapter production promotion) + 3 server lifecycle audit actions
- **Sprint 1**: cmd/server/{server,probes,main}.go (package main 전환) — 11-step dependency wiring + errgroup dual listener + signal.NotifyContext
- **Sprint 2**: graceful shutdown (sync.Once + double-signal force-kill + reverse cleanup) + E2E full-stack (testcontainers)
- **30 신규 tests** (19 unit + 11 E2E/integration), 누적 ~445+
- REQ-SERVER-001 dual listener | REQ-SERVER-002 dependency wiring | REQ-SERVER-003 graceful shutdown | REQ-SERVER-004 health/readiness probes

### Quality (continued — SERVER-001)
- **plan-auditor**: iter 1 FAIL 0.62 (8 phantom API defects) → iter 2 FAIL 0.78 (3 refined) → iter 3 PASS 0.92 (spec-to-code 0 contradictions)
- **evaluator-active**: CONFIRM 0.83 (integrated wiring validation)
- **TRUST 5**: [PASS] Tested 0.95 | Readable 0.92 | Unified 0.90 | Secured 0.89 | Trackable 0.88

### Fixed (SERVER-001)
- 5개 SPEC이 전제했던 cmd/server/server.go stub → production wiring 완성 (통합 결함 정식 해소)
- CTRL-001 Sprint 7 T-AX-006 gap + AUTH-002 Exclusion #12 unblock

### Deferred (後続 SPEC)
- SPEC-AX-OBS-001 (Prometheus /metrics + OTel)
- e2e_test.go goRedisAdapter cleanup (redis_adapter.go로 통합 후 중복 제거)
- dispatcher_test.go:549 pre-existing race 수정

---

### Added — SPEC-AX-AUTH-002 v0.1.2 (RBAC REST/gRPC Handler 통합)
- **Sprint 0+1+2 (통합)**: 메서드-권한 매핑 테이블 + RESTAuthzMiddleware + UnaryAuthzInterceptor + 체인 조합 헬퍼
  - `LookupRESTPermission` / `LookupGRPCPermission` (경로 매개변수 매칭)
  - `BuildRESTChain` (auth → authz → handler 순서 강제)
  - `BuildGRPCInterceptorChain` (ChainUnaryInterceptor)
  - 28+ 단위 테스트, 신규 함수 90-100% 커버리지
- **Sprint 3 E2E**: testcontainers Postgres+Redis 5 신규 + 1 차단 해제 = 6 통과
  - TestE2E_Authz_AdminFullAccess / ViewerForbidden_POST / AnalystWriteAllowed / AuthDisabled_BypassesAuthz / GRPC_ViewerForbidden_Create
  - AUTH-001 TestE2E_Auth_RBACForbidden SKIP 제거 (grep count=0)

### Quality (계속)
- **테스트**: AUTH-002 28 unit + 6 E2E = 34 신규, 누적 ~410+
- **TRUST 5**: PASS (Tested ≥ 0.85 / Readable / Unified / Secured ≥ 0.85 / Trackable)
  - plan-auditor iter 1 FAIL 0.74 → iter 2 PASS 0.92 (7개 결함 해소)
  - evaluator-active iter 1 DISPUTE 0.7505 → iter 3 CONFIRM 0.8415 (보안 필수 0.85 ≥ 0.75)

### Security (계속)
- **기본값-거부 안전장치**: 매핑 미정의 메서드 → 503 AUTHZ_MAPPING_MISSING (200 절대 금지)
- **AUTH-001 통합**: RBAC 라이브러리 + REST/gRPC 핸들러 엔트리 포인트 통합 완료
- **체인 순서 강제**: auth → authz → handler 순서 강제 (BuildRESTChain / BuildGRPCInterceptorChain)
- **viewer 차단**: DELETE/POST 시도 사전 차단 + AUTH_FORBIDDEN audit row

### Fixed
- AUTH-001 SKIP'd TestE2E_Auth_RBACForbidden을 SPEC-AX-AUTH-002 Sprint 3에서 차단 해제

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
