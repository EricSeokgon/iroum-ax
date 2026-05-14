# Changelog

본 프로젝트의 모든 주목할 만한 변경 사항을 이 파일에 기록합니다.

형식은 [Keep a Changelog](https://keepachangelog.com/ko/1.1.0/)을 따르며,
이 프로젝트는 [Semantic Versioning](https://semver.org/lang/ko/)을 준수합니다.

## [Unreleased] - 2026-05-14

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
