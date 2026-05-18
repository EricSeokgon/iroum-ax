# 요구사항 추적성 매트릭스 (req-traceability)

REQ-UBI + REQ-AX-001~005 (Python) + REQ-CTRL-001~005 (Go) + REQ-AUTH-001~005 (Go/Python SSO) + REQ-SERVER-001~004 (Go Server Bootstrap) 와 구현 파일 · 테스트 파일의 맵핑

---

## SPEC-AX-SERVER-001: Server Bootstrap + Dual Listener (새로 추가)

### SERVER-001 고유 요구사항

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-SERVER-001 | Dual listener (gRPC :50051 + REST :8080) | `cmd/server/server.go:169~190` | `server_e2e_test.go` (11 E2E) | PASS |
| REQ-SERVER-002 | Dependency injection (11-step sequence) | `cmd/server/server.go:66~165` | `server_test.go` (7 tests) | PASS |
| REQ-SERVER-003 | Graceful shutdown (SIGTERM/SIGINT, 30s timeout) | `cmd/server/server.go:274~330` | `server_e2e_test.go:TestServerShutdown` | PASS |
| REQ-SERVER-004 | Health/readiness probes (/health, /ready) | `cmd/server/probes.go` | `probes_test.go` (5 tests) | PASS |
| REQ-SERVER-UBI-001 | Audit trail (SERVER_STARTUP/SHUTDOWN_INITIATED/COMPLETED) | `cmd/server/server.go:78,120,310` | `server_test.go` + E2E | PASS |

### SERVER-001 추가 산출물

| 요구사항 | 상태 | 설명 |
|----------|------|------|
| redis_adapter.go promotion | PASS | goRedisAdapter (e2e_test.go test-only) → `internal/scheduler/redis_adapter.go` (production) |
| PgWorkflowStore.Ping | PASS | Sprint 0 추가 (readiness probe 필요) |
| JWKSCache.Reachable | PASS | Sprint 0 추가 (readiness probe 필요) |

**테스트 합계**: 19개 단위 + 11개 E2E = 30개 신규 (모두 PASS, cumulative ~445+)

---

## SPEC-AX-AUTH-002: RBAC REST/gRPC Handler 통합

### AUTH-002 고유 요구사항

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-AUTH2-001-U1 | REST/gRPC 메서드-권한 매핑 | `internal/auth/authz_mapping.go` | `authz_mapping_test.go` (5 tests) | PASS |
| REQ-AUTH2-002 | RESTAuthzMiddleware 차단 결정 | `internal/auth/authz_middleware.go` | `authz_middleware_test.go` (8 tests) | PASS |
| REQ-AUTH2-003 | UnaryAuthzInterceptor (gRPC 차단) | `internal/auth/authz_middleware.go` | `authz_middleware_test.go` (8 tests) | PASS |
| REQ-AUTH2-004 | 체인 조합 순서 강제 (auth → authz → handler) | `internal/auth/chain.go` | `chain_test.go` (2 tests) | PASS |
| REQ-AUTH2-UBI-001 | default-deny 안전장치 (매핑 미정의 → 503) | `internal/auth/authz_middleware.go:240` | E2E 6개 + 단위 5개 | PASS |

**테스트 합계**: 28개 단위 + 6개 E2E = 34개 신규 (모두 PASS, AUTH-001 SKIP 차단 해제)

---

## SPEC-AX-CTRL-001: Go Control Plane 요구사항

### Control Plane 고유 요구사항

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-CTRL-001 | Workflow State Machine (4 상태, 3 전이) | `internal/workflow/state_machine.go` | `state_machine_test.go` (14 tests) | PASS |
| REQ-CTRL-002 | gRPC Service (Create/Get/List) | `cmd/server/grpc_handlers.go` | `grpc_handlers_test.go` (12 tests) | PASS |
| REQ-CTRL-003 | REST API (POST/GET/LIST + /healthz) | `cmd/server/rest_handler.go` | `rest_handler_test.go` (12 tests) | PASS |
| REQ-CTRL-004 | PostgreSQL Store (pgx v5, SELECT FOR UPDATE) | `internal/store/pg_store.go` | `pg_store_test.go` (11 integration) | PASS |
| REQ-CTRL-005 | Celery Dispatcher (Kombu v2 envelope, Redis RPUSH) | `internal/scheduler/dispatcher.go` | `dispatcher_test.go` (15 tests) | PASS |

### Control Plane + AX 공유 요구사항

| REQ ID | 설명 | Go 구현 | Python 구현 | 상태 |
|--------|------|--------|-----------|------|
| REQ-UBI-001 | 트랜잭션 원자성 (audit 실패 → 롤백) | `internal/workflow/transaction.go` + `internal/store/pg_store.go` | `pipelines/workers/` | PASS (CTRL) |
| REQ-UBI-002 | 8개 감시 액션 (audit_logs 기록) | `internal/audit/audit.go` (8 Action enum) | `pkg/logging/logger.py` | PASS (CTRL) |
| REQ-UBI-003 | cli-anonymous 기본값 | `internal/config/config.go` | `pipelines/config/settings.py` | PASS (CTRL) |

### Control Plane E2E

| AC ID | 설명 | 테스트 위치 | 상태 |
|-------|------|-----------|------|
| AC-CTRL-E2E-1 | 전체 흐름 (create → transition → dispatch) | `cmd/server/e2e_test.go` (5 tests) | PASS (5/5) |

**테스트 합계**: 79개 단위 + 11개 통합 + 5개 E2E = 95개 (모두 PASS)

---

## SPEC-AX-001: Python 파이프라인 요구사항

## REQ-UBI (Ubiquitous Requirements)

### REQ-UBI-001: 데이터 주권

| AC | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----|----|----|---------|----------|
| AC-UBI-001-1 | 외부 LLM API (OpenAI, Claude) 호출 시도 | 파이프라인 초기화 | validate_llm_endpoint() 체크로 403 반환 및 audit 기록 | `pipelines/config/settings.py` | `pipelines/config/tests/test_llm_validation.py` |
| AC-UBI-001-2 | 모든 output 아티팩트 | 워크플로우 완료 시 | 고객사 내부 PostgreSQL/Redis에만 저장 (외부 전송 금지) | `pipelines/main.py` | `tests/integration/test_data_sovereignty.py` |
| AC-UBI-001-3 | 모델 파라미터 (Qwen2-VL, EXAONE) | 자체 호스팅 vLLM 배포 | 외부 API 엔드포인트 미사용 (K8s 내부 통신만) | `pipelines/generation/llm_client.py` | `pipelines/generation/tests/test_llm_endpoint.py` |

### REQ-UBI-002: 언어 (한국어 Enforcement)

| AC | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----|----|----|---------|----------|
| AC-UBI-002-1 | 영문/혼용 입력 문서 | 파싱 후 언어 감지 | detect_language()가 한국어 아닌 경우 "non_korean" 플래그 반환 | `pipelines/ingestion/language_detector.py` | `pipelines/ingestion/tests/test_language_detector.py` |
| AC-UBI-002-2 | 한국어 보고서 초안 | LLM 생성 후 스타일 검증 | 반말, 존댓말 혼용, 비-합니다체 감지 → reject (재시도) | `pipelines/generation/style_applier.py` | `pipelines/generation/tests/test_korean_style.py` |
| AC-UBI-002-3 | 임베딩 모델 선택 | RAG 인덱싱 | ko-sroberta-multitask 한국어 모델만 사용 (768 dim) | `pipelines/mapping/embedding_service.py` | `pipelines/mapping/tests/test_embedding_ko.py` |

### REQ-UBI-003: 감시 로깅 (Audit)

| AC | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----|----|----|---------|----------|
| AC-UBI-003-1 | 문서 업로드 | POST /api/documents/upload 완료 | audit_logs INSERT: user_id='cli-anonymous' (SSO 미구현), action='UPLOAD', resource_id=document_id | `pipelines/ingestion/hwp_parser.py` + AuditLogger | `tests/unit/test_audit_logging.py` |
| AC-UBI-003-2 | 초안 생성 | Report 레코드 INSERT | audit_logs: user_id='cli-anonymous', action='DRAFT_GENERATE', resource_id=report_id, details={'model': 'exaone'} | `pipelines/generation/report_drafter.py` | `tests/unit/test_generation_audit.py` |
| AC-UBI-003-3 | 등급 예측 | Simulation INSERT | audit_logs: action='PREDICTION', resource_id=simulation_id, details={'confidence': 'high/medium/low'} | `pipelines/scoring/grade_predictor.py` | `tests/unit/test_scoring_audit.py` |
| AC-UBI-003-4 | 모든 감시 로그 | 지속적 | 모든 audit_logs 레코드: timestamp=NOW(), details JSON 저장 | `pkg/logging/logger.py` | `tests/unit/test_audit_logger.py` |

---

## REQ-AX-001: Document Ingestion

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-001-1 | HWP 정상 파싱 | KEPCO E&C 실적보고서.hwp (50페이지) | POST /api/documents/upload | document.status='parsed', parsed_text ≥ 1000자, 테이블 3개 추출 | `pipelines/ingestion/hwp_parser.py` | `pipelines/ingestion/tests/test_hwp_parser_basic.py` |
| AC-001-2 | PDF 정상 파싱 | 평가편람.pdf | 파싱 | parsed_text ≥ 500자, 회전 페이지 감지 후 정렬 | `pipelines/ingestion/pdf_parser.py` | `pipelines/ingestion/tests/test_pdf_parser.py` |
| AC-001-3 | HWP 파싱 실패 → OCR fallback | hwp_parser 예외 발생 | VLM 호출 | document.status='ocr_fallback', Qwen2-VL 7B <2초/페이지 처리 | `pipelines/ingestion/vlm_processor.py` | `pipelines/ingestion/tests/test_vlm_ocr_fallback.py` |
| AC-001-4 | 테이블 추출 | VLM OCR 결과 (셀 정렬 필요) | table_extractor 호출 | 셀 경계 검증, schema 일치 확인, tables[] 저장 | `pipelines/ingestion/table_extractor.py` | `pipelines/ingestion/tests/test_table_extraction.py` |
| AC-001-5 | GPU 없음 환경 | CPU 모드 배포 | vLLM inference | CPU fallback 자동 실행, 지연시간 5-10배 증가 (문서 처리 여전히 가능) | `pipelines/config/settings.py` | `tests/integration/test_cpu_mode.py` |

**AC 총 5개** (실제 테스트 31개 = Sprint 2 RED-GREEN)

---

## REQ-AX-002: Criterion Mapping & RAG Indexing

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-002-1 | 평가편람 계층 파싱 | 기획재정부 경영평가 편람 PDF | criterion_parser.parse_criterion_handbook() | Item→Indicator→Detail 계층 추출, max_points 구조 검증 | `pipelines/mapping/criterion_parser.py` | `pipelines/mapping/tests/test_criterion_parser.py` |
| AC-002-2 | 임베딩 생성 | 평가기준 텍스트 (500-1000 tokens) | embedding_service.embed_text(text, "ko-sroberta-multitask") | embedding: List[float] (768 dim), norm 검증 | `pipelines/mapping/embedding_service.py` | `pipelines/mapping/tests/test_embedding_generation.py` |
| AC-002-3 | pgvector 인덱싱 | 50개 평가기준 + 임베딩 | vector_store.upsert_vectors() | PostgreSQL criteria table: HNSW 인덱스 생성, INSERT 성공 | `pipelines/mapping/vector_store.py` | `pipelines/mapping/tests/test_vector_store_indexing.py` |
| AC-002-4 | RAG 검색 정확도 | 쿼리: "안전교육 이수율" | retriever.retrieve(query, top_k=3) | top-3 결과 반환, 평균 relevance ≥ 0.8, p99 latency < 100ms | `pipelines/mapping/retriever.py` | `pipelines/mapping/tests/test_rag_accuracy.py` |
| AC-002-5 | 불충분한 context | RAG 결과 relevance < 0.7 × 3개 | retriever 반환 | status='insufficient_context' + candidates 반환 (silent skip 금지) | `pipelines/mapping/retriever.py` | `pipelines/mapping/tests/test_rag_insufficient_context.py` |
| AC-002-6 | 한글/한자 정규화 실패 | 입력: "敎育" (한자) | parsing 중 에러 | 테스트는 skip (후속 SPEC-AX-002에서 처리) | N/A | N/A |

**AC 총 5개** (실제 테스트 35개 = Sprint 3 RED-GREEN)

---

## REQ-AX-003: Grade Simulation

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-003-1 | 2-class softmax 예측 | 자사 보고서 (parsed_text) | grade_predictor.predict_probabilities() | P(A) + P(B) ≤ 1.0, 두 값 모두 ≥ 0.1 | `pipelines/scoring/grade_predictor.py` | `pipelines/scoring/tests/test_grade_prediction_basic.py` |
| AC-003-2 | Abstain 분기 활성화 | P(A)=0.42, P(B)=0.45 | predict_probabilities() 결과 | abstain_flag=true, abstain=0.13, P(A)+P(B)+abstain=1.0 | `pipelines/scoring/grade_predictor.py` | `pipelines/scoring/tests/test_abstain_activation.py` |
| AC-003-3 | 시나리오 생성 | 현재 등급 B, 목표 A | scenario_simulator.simulate_scenarios() | 3-5개 시나리오 생성, score_delta 양수, new_grade ≤ A | `pipelines/scoring/scenario_simulator.py` | `pipelines/scoring/tests/test_scenario_generation.py` |
| AC-003-4 | 벤치마크 학습 | A/B 등급 보고서 각 3개 | benchmark_learner.fit() | feature 추출 완료, decision boundary 설정, 모델 저장 | `pipelines/scoring/benchmark_learner.py` | `pipelines/scoring/tests/test_benchmark_training.py` |
| AC-003-5 | 저신뢰도 처리 | abstain_flag=true 예측 | downstream consumption (generation/recommendation) | low_confidence 플래그 전파, 인간 검증 요청 | `pipelines/scoring/grade_predictor.py` | `pipelines/scoring/tests/test_low_confidence_handling.py` |

**AC 총 5개** (실제 테스트 29개 = Sprint 4 RED-GREEN)

---

## REQ-AX-004: Report Draft Generation

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-004-1 | EXAONE 초안 생성 | 평가기준 + 고객 콘텐츠 | llm_client.generate_text(prompt, model="exaone-3.5-7b") | 초안 텍스트 (200-500자), <5초, 합니다체 준수 | `pipelines/generation/llm_client.py` | `pipelines/generation/tests/test_draft_generation_exaone.py` |
| AC-004-2 | Qwen fallback | EXAONE 호출 3회 연속 실패 | auto-fallback to Qwen 2.5 7B | 같은 프롬프트로 Qwen 호출, 초안 반환 | `pipelines/generation/llm_client.py` | `pipelines/generation/tests/test_fallback_qwen.py` |
| AC-004-3 | 공문 스타일 검증 | LLM 생성 텍스트 | style_applier.validate_korean_style() | 반말/존댓말 혼용 감지 → reject, re-prompt (retry ≤ 3) | `pipelines/generation/style_applier.py` | `pipelines/generation/tests/test_style_validation.py` |
| AC-004-4 | 스타일 위반 → 실패 | LLM 재시도 3회 모두 실패 | generate_text() 반환 | status='style_violation', HTTP 503, 외부 LLM 호출 금지 (REQ-UBI-001) | `pipelines/generation/llm_client.py` | `pipelines/generation/tests/test_style_fallback_exhausted.py` |
| AC-004-5 | 프롬프트 구성 | 평가기준 + 지침 + 예시 | prompt_builder.build_prompt() | 프롬프트 생성 (2000-3000 tokens), 맥락 주입 완료 | `pipelines/generation/prompt_builder.py` | `pipelines/generation/tests/test_prompt_building.py` |

**AC 총 5개** (실제 테스트 38개 = Sprint 5 RED-GREEN)

---

## REQ-AX-005: Gap Recommendation

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-005-1 | Gap 분석 | 현재 보고서(B) + 목표(A) + 벤치마크 | gap_analyzer.analyze_gap() | 3-5개 gap 항목 식별, score_delta 계산 | `pipelines/recommendation/gap_analyzer.py` | `pipelines/recommendation/tests/test_gap_analysis.py` |
| AC-005-2 | 콘텐츠 제안 | 벤치마크 인덱스 (A 등급 RAG) | content_suggester.suggest_content() | 각 gap당 matching content 제안, 소스 reference 기록 | `pipelines/recommendation/content_suggester.py` | `pipelines/recommendation/tests/test_content_suggestion.py` |
| AC-005-3 | 우선순위 정렬 | 5개 제안 항목 + feasibility score | prioritizer.rank_by_feasibility() | priority 1-5 부여, 합계 score_delta >= 5 | `pipelines/recommendation/prioritizer.py` | `pipelines/recommendation/tests/test_prioritization.py` |
| AC-005-4 | 벤치마크 부재 | A등급 벤치마크 데이터 미로드 | gap_analyzer 호출 | status='benchmark_not_available', empty recommendation list (정책: 제안 fabrication 금지) | `pipelines/recommendation/gap_analyzer.py` | `pipelines/recommendation/tests/test_no_benchmark.py` |
| AC-005-5 | 사용자 피드백 | 추천 항목 "not_feasible" 마크 | POST /api/recommendations/{id}/feedback | 피드백 저장, 우선순위 downgrade, 대체 제안 반환 | `pipelines/recommendation/gap_analyzer.py` | `pipelines/recommendation/tests/test_user_feedback.py` |

**AC 총 5개** (실제 테스트 21개 = Sprint 6 RED-GREEN)

---

## 종합 요구사항 매트릭스

---

## SPEC-AX-AUTH-001: JWT/OIDC 인증 + RBAC (Go + Python)

### REQ-AUTH-001: JWT Validator (SF-1 + SF-2)

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-AUTH-001-1 | JWT signature 검증 | RS256 서명 토큰 | validator.Validate(token) | signature 유효 → user_id 추출 | `apps/control-plane/internal/auth/validator.go` | `validator_test.go` (5개) |
| AC-AUTH-001-2 | SF-1 발행자 검증 | iss="https://other-realm/" 토큰 | validator 검증 | ErrTokenInvalidIssuer 반환 | `apps/control-plane/internal/auth/validator.go` | `validator_test.go` (3개) |
| AC-AUTH-001-3 | SF-2 알고리즘 혼동 공격 | alg=RS256, kty=EC 불일치 | validator.validateJWTSignature() | ErrAlgorithmKeyMismatch 반환 | `apps/control-plane/internal/auth/validator.go` | `validator_test.go` (5개) |
| AC-AUTH-001-4 | 추가 클레임 검증 | aud/exp/kid 포함 토큰 | validator | exp > now, aud 일치, kid in JWKS | `apps/control-plane/internal/auth/validator.go` | `validator_test.go` (6개) |

**AC 총 4개** (실제 테스트 19개)

### REQ-AUTH-002: OIDC Discovery + JWKS Cache

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-AUTH-002-1 | OIDC Discovery | Keycloak 호스트 URL | OIDCClient.Discovery() | jwks_uri 자동 발견 | `apps/control-plane/internal/auth/oidc.go` | `oidc_test.go` (3개) |
| AC-AUTH-002-2 | JWKS 캐시 히트 | 동일 kid 반복 요청 | JWKSCache.GetKey(kid) | TTL 내: 캐시 반환 (네트워크 요청 0) | `apps/control-plane/internal/auth/jwks_cache.go` | `jwks_cache_test.go` (3개) |
| AC-AUTH-002-3 | TTL 만료 후 갱신 | TTL=3600초 경과 | background refresh 트리거 | stale 캐시 반환 + 백그라운드 갱신 | `apps/control-plane/internal/auth/jwks_cache.go` | `jwks_cache_test.go` (4개) |
| AC-AUTH-002-4 | JWKS 불가능 상황 | 네트워크 실패 | max-age 4시간 내 | stale 캐시 반환 (에러 아님) | `apps/control-plane/internal/auth/jwks_cache.go` | `jwks_cache_test.go` (4개) |
| AC-AUTH-002-5 | 동시 요청 | 5개 goroutine 동시 kid 요청 | 캐시 경합 | 1회만 fetch, 나머지 대기 | `apps/control-plane/internal/auth/jwks_cache.go` | `jwks_cache_test.go` (3개) |

**AC 총 5개** (실제 테스트 17개)

### REQ-AUTH-003: Middleware (gRPC + REST)

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-AUTH-003-1 | gRPC 유효 토큰 | Authorization metadata + valid JWT | UnaryInterceptor | user ctx 주입 후 handler 호출 | `apps/control-plane/internal/auth/middleware.go` | `middleware_test.go` (6개) |
| AC-AUTH-003-2 | REST 유효 토큰 | Authorization header + Bearer token | RESTMiddleware | context에 user 저장 후 next() | `apps/control-plane/internal/auth/middleware.go` | `middleware_test.go` (5개) |
| AC-AUTH-003-3 | Health endpoint 우회 | /grpc.health.v1.Health/Check | middleware | 인증 검사 스킵 | `apps/control-plane/internal/auth/middleware.go` | `middleware_test.go` (2개) |
| AC-AUTH-003-4 | AuthDisabled 폴백 | config.auth_enabled=false | middleware | 익명(cli-anonymous) 사용자 주입 | `apps/control-plane/internal/auth/middleware.go` | `middleware_test.go` (4개) |
| AC-AUTH-003-5 | 유효하지 않은 토큰 | 만료/변조/헤더 누락 | middleware | Unauthenticated 에러 반환 (401) | `apps/control-plane/internal/auth/middleware.go` | `middleware_test.go` (3개) |

**AC 총 5개** (실제 테스트 20개)

### REQ-AUTH-004: RBAC (3-Role Matrix)

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-AUTH-004-1 | admin 역할 | scope="admin:*:*" | ParseRolesFromScope() | roles=[admin] | `apps/control-plane/internal/auth/rbac.go` | `rbac_test.go` (4개) |
| AC-AUTH-004-2 | analyst 역할 | scope="analyst:read:document" | EffectivePermissions() | permissions={read:document} | `apps/control-plane/internal/auth/rbac.go` | `rbac_test.go` (5개) |
| AC-AUTH-004-3 | viewer 역할 제한 | scope="viewer:*", action=write | Authorize(action) | PermissionDenied 반환 | `apps/control-plane/internal/auth/rbac.go` | `rbac_test.go` (4개) |
| AC-AUTH-004-4 | Forbidden audit | 권한 거부 이벤트 | LogForbidden() | audit_logs: action=FORBIDDEN_ACCESS | `apps/control-plane/internal/auth/rbac.go` | `rbac_test.go` (3개) |
| AC-AUTH-004-5 | 역할 계층 | admin > analyst > viewer | matrix 검증 | admin은 모든 권한 포함 | `apps/control-plane/internal/auth/rbac.go` | `rbac_test.go` (2개) |

**AC 총 5개** (실제 테스트 18개)

### REQ-AUTH-005: Refresh + Logout (OAuth 2.0 BCP)

| AC | Scenario | Given | When | Then | 구현 파일 | 테스트 파일 |
|----|----------|-------|------|------|---------|----------|
| AC-AUTH-005-1 | Token 갱신 | 유효한 refresh_token | RefreshSession() | 새 access/refresh_token 발급 | `apps/control-plane/internal/auth/refresh.go` | `refresh_test.go` (3개) |
| AC-AUTH-005-2 | Refresh rotation | 각 갱신마다 새 토큰 | 갱신 3회 | refresh_token_1, 2, 3 모두 다름 | `apps/control-plane/internal/auth/refresh.go` | `refresh_test.go` (3개) |
| AC-AUTH-005-3 | Family invalidation | refresh_token 재사용 감지 | RefreshSession() + 재사용 토큰 재시도 | refresh_token_family 전체 무효화, 에러 반환 | `apps/control-plane/internal/auth/refresh.go` | `refresh_test.go` (4개) |
| AC-AUTH-005-4 | Logout | Logout() 호출 | refresh_token_family 블랙리스트 | 모든 refresh_token 무효화 | `apps/control-plane/internal/auth/refresh.go` | `refresh_test.go` (2개) |
| AC-AUTH-005-5 | Python cross-SPEC | FastAPI envelope.headers.user_id | TokenValidator 동기 호출 | JWT sub 추출, 컨텍스트 주입 | `pipelines/auth/celery_auth.py` | `test_celery_auth.py` (1개) |

**AC 총 5개** (실제 테스트 13개)

### REQ-AUTH-E2E: End-to-End Integration

| AC | Scenario | Given | When | Then | 테스트 파일 |
|----|----------|-------|------|------|----------|
| AC-AUTH-E2E-1 | 전체 JWT 체인 | Keycloak 토큰 | CreateWorkflow RPC | validator → middleware → RBAC → audit | `apps/control-plane/cmd/server/e2e_test.go` |
| AC-AUTH-E2E-2 | 익명 요청 | auth_enabled=false | REST endpoint | cli-anonymous 사용자 | `apps/control-plane/cmd/server/e2e_test.go` |
| AC-AUTH-E2E-4 | Invalid token 401 | 만료/변조 토큰 | gRPC 요청 | Unauthenticated 반환 | `apps/control-plane/cmd/server/e2e_test.go` |
| AC-E2E-RBAC-1 | RBAC REST handler | viewer 역할 | PUT /api/v1/workflows/{id} | PermissionDenied 반환 | SKIP → SPEC-AX-AUTH-002 |

**E2E 상태**: 4 PASS + 1 SKIP

---

## SPEC-AX-EVID-001: 증빙 자료 수집/관리 (Walking Skeleton)

### REQ-EVID-001: 증빙 데이터 모델 & Store 계층

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-EVID-001 | 증빙 데이터 모델 & Store 계층 | `internal/store/evidence.go`, `internal/store/pg_store.go` | `evidence_store_test.go`, `evidence_helpers_test.go` | PASS |

**구현 세부**: `evidences` 테이블 (단일 테이블, `file_content BYTEA` 컬럼), `EvidenceTx.InsertEvidence`, `BeginEvidenceTx(pool)`, `GetEvidenceByID`, `ListEvidencesByEvalItem`

### REQ-EVID-002: 버전 관리/체이닝

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-EVID-002 | 버전 관리/체이닝 | `internal/store/evidence.go` (`GetLatestVersionByEvalItem`, `MarkSuperseded`) | `evidence_version_test.go`, `evidence_concurrent_test.go`, `evidence_rollback_test.go` | PASS |

**구현 세부**: `previous_version_id UUID REFERENCES evidences(id)` 자기 참조, `SELECT FOR UPDATE` 직렬화, `status=SUPERSEDED` 불변성, `ErrEvidenceNotFound` / `ErrEvidenceImmutable` sentinel (`internal/errors/errors.go`)

### REQ-EVID-003: 감사 연계 Recorder 확장

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-EVID-003 | 감사 연계 Recorder 확장 | `internal/audit/recorder.go`, `internal/audit/clock.go` | `internal/audit/recorder_evidence_test.go` | PASS |

**구현 세부**: `RecordEvidenceCreated(ctx, tx, evidenceID, evalItemID, fileHashSHA256, version, userID)`, `RecordEvidenceVersioned(ctx, tx, evidenceID, evalItemID, fileHashSHA256, version, previousVersionID, userID)`, `Clock` 인터페이스 + `systemClock` (시간 주입), `DefaultUserID = "cli-anonymous"`

### REQ-EVID-004: 저장 전략 추상화

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-EVID-004 | 저장 전략 추상화 | `internal/storage/storage.go` | `evidence_storage_strategy_test.go` | PASS |

**구현 세부**: `EvidenceBlobStore` 인터페이스 (`Put(ctx, key, io.Reader) (string, error)`, `Get(ctx, location) (io.ReadCloser, error)`), `dbBlobStore` 구현체 (`Put` → `"db://evidences/" + key` 반환, bytes는 `InsertEvidence` TX에서 처리), `NewDBBlobStore()`

### REQ-EVID-UBI-001~004: 교차 요구사항

| REQ ID | 설명 | 구현 위치 | 테스트 | 상태 |
|--------|------|---------|--------|------|
| REQ-EVID-UBI-001 | 데이터 주권 (외부 SaaS SDK 미사용) | `internal/storage/storage.go`, `go.mod` (외부 스토리지 SDK 없음) | `evidence_sovereignty_test.go` | PASS |
| REQ-EVID-UBI-002 | 감사 가능성 (모든 생성/버전 이벤트 audit 기록) | `cmd/server/evidence_handlers.go` (`persistEvidenceTx`), `internal/audit/recorder.go` | `evidence_audit_test.go` | PASS |
| REQ-EVID-UBI-003 | cli-anonymous 기본값 (인증 미구성 시) | `internal/audit/recorder.go` (`DefaultUserID`), `internal/config/config.go` | `recorder_evidence_test.go` | PASS |
| REQ-EVID-UBI-004 | 버전 불변 (successor 존재 시 SUPERSEDED만 허용) | `internal/store/evidence.go` (`MarkSuperseded`), `internal/errors/errors.go` (`ErrEvidenceImmutable`) | `evidence_version_test.go`, `evidence_errorpaths_test.go` | PASS |

### GAP 해소 현황 (GAP-01, GAP-03, GAP-04)

| GAP ID | 설명 | 해소 방법 | 상태 |
|--------|------|---------|------|
| GAP-01 | 증빙 생성 API 부재 | `POST /api/v1/evidences` 단일 라우트 (`cmd/server/evidence_handlers.go`) — 생성과 버전 모두 `handleCreateEvidence` 단일 핸들러 처리 | RESOLVED |
| GAP-03 | `ErrEvidenceNotFound` sentinel 부재 | `internal/errors/errors.go` 추가 | RESOLVED |
| GAP-04 | `ErrEvidenceImmutable` sentinel 부재 | `internal/errors/errors.go` 추가 | RESOLVED |

**테스트 합계**: evidence 관련 Go 테스트 신규 (store 10개 파일 + handler 5개 파일 + audit 1개 파일), coverage 91.4%

---

## 통합 요구사항 추적성 요약

| REQ | AC 수 | 테스트 수 | 구현 모듈 수 | 상태 |
|-----|-----|---------|-----------| ------|
| **REQ-UBI** | 4 | 15 | 5 (설정, 감지, 검증, 로거) | 완료 (Sprint 1 CTRL) |
| **REQ-AX-001** | 5 | 31 | 5 (파서 3 + 추출 2) | 완료 (Sprint 2 AX) |
| **REQ-AX-002** | 5 | 35 | 4 (매핑, 임베딩, 저장, 검색) | 완료 (Sprint 3 AX) |
| **REQ-AX-003** | 5 | 29 | 3 (학습, 예측, 시뮬) | 완료 (Sprint 4 AX) |
| **REQ-AX-004** | 5 | 38 | 4 (클라이언트, 드래프터, 빌더, 스타일) | 완료 (Sprint 5 AX) |
| **REQ-AX-005** | 5 | 21 | 3 (갭, 제안, 우선순위) | 완료 (Sprint 6 AX) |
| **REQ-CTRL-001~005** | - | 95 | 12 (server, workflow, store, audit, scheduler) | 완료 (Sprint 7 CTRL) |
| **REQ-AUTH-001~005 + E2E** | 24 | 105 | 12 (validator, oidc, cache, middleware, rbac, refresh) | 완료 (Sprint 7 AUTH) |
| **REQ-EVID-001~004 + UBI-001~004** | 8 | 91.4% cov | 6 (store, pg_store, storage, recorder, clock, evidence_handlers) | 완료 (SPEC-AX-EVID-001) |
| **합계** | **61+** | **480+** | **49+** | 100% 완료 |

---

**최종 업데이트**: 2026-05-18 (SPEC-AX-EVID-001 v0.1.0 Sync 완료)  
**전체 AC**: 61+ 구현 · 테스트됨  
**전체 테스트**: 480+ passing (Python 192 + Go 247+ + 11 integration + 21 E2E)  
**커버리지**: Python 83% | Go evidence/ 91.4% | Go auth/ 70% | TRUST 5: 모두 PASS
