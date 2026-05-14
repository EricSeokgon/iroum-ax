# iroum-ax 구조 문서

> **최종 수정**: 2026-05-14  
> **소스**: `.moai/project/interview.md`, `product.md`  
> **상태**: 계획 단계 (greenfield 프로젝트)

---

## 1. 모노레포 설계 원칙

### 1.1 레이어 아키텍처

```
iroum-ax (모노레포)
├── Control Plane (Go) — 에이전트 라이프사이클, 워크플로우 오케스트레이션
├── Pipelines (Python) — VLM/RAG/Document AI — MVP 비중 최대
├── Console (TypeScript/React/Next.js) — 사용자 UI
├── Shared Schemas (Protocol Buffers / OpenAPI) — 계약 정의
└── Deployments (Helm + Kubernetes) — 배포 자동화
```

### 1.2 기술별 책임 분리

| 계층 | 언어 | 책임 | 진입점 |
|------|------|------|--------|
| **Inference** | Python | VLM/LLM 추론, RAG 벡터 DB 관리, Document 파싱 | FastAPI, Celery worker |
| **Orchestration** | Go | 워크플로우 상태 관리, 에이전트 스케줄링, K8s Operator | gRPC, REST API |
| **User Interface** | TypeScript | 콘솔 대시보드, 실적보고서 뷰어, Recommendation 인터페이스 | Next.js SSR |
| **Contract** | Protobuf / OpenAPI | 계층 간 통신 규약 | `schemas/proto/`, `schemas/openapi/` |

---

## 2. 모노레포 디렉토리 트리 (계획)

```
iroum-ax/
├── README.md (프로젝트 소개, 빠른 시작 가이드)
├── .gitignore
├── .env.example
├── go.mod, go.sum (Go 의존성)
├── pyproject.toml, poetry.lock (Python 의존성)
├── package.json, pnpm-lock.yaml (TypeScript 의존성)
│
├── apps/
│   ├── control-plane/ (Go)
│   │   ├── main.go
│   │   ├── cmd/
│   │   │   ├── server/ (gRPC + REST API 서버)
│   │   │   └── operator/ (K8s Operator)
│   │   ├── internal/
│   │   │   ├── workflow/ (워크플로우 상태 머신)
│   │   │   ├── scheduler/ (에이전트 스케줄러)
│   │   │   ├── proto/ (Protocol Buffer 생성 코드)
│   │   │   └── store/ (상태 저장소: PostgreSQL 클라이언트)
│   │   ├── pkg/ (외부 공유 라이브러리)
│   │   ├── config/ (설정 템플릿)
│   │   └── tests/
│   │
│   └── console/ (TypeScript/React/Next.js)
│       ├── app/ (Next.js 13+ App Router)
│       │   ├── page.tsx (홈)
│       │   ├── dashboard/ (평가 대시보드)
│       │   ├── documents/ (문서 업로드·관리)
│       │   ├── report/ (초안 보고서 뷰어)
│       │   ├── simulation/ (등급 시뮬레이션 UI)
│       │   ├── recommendations/ (추천 항목 목록)
│       │   └── api/ (API 라우트, 컨트롤플레인 호출)
│       ├── components/ (재사용 가능한 UI 컴포넌트)
│       │   ├── DocumentUploader.tsx
│       │   ├── ReportViewer.tsx
│       │   ├── SimulationChart.tsx
│       │   └── RecommendationCard.tsx
│       ├── hooks/ (React 커스텀 훅)
│       ├── styles/ (글로벌 스타일, Tailwind 설정)
│       ├── lib/ (유틸리티: API 클라이언트, 데이터 포맷팅)
│       ├── public/ (정적 자산)
│       ├── next.config.js
│       ├── tsconfig.json
│       └── tests/
│
├── pipelines/ (Python — VLM/RAG/Document AI)
│   ├── __init__.py
│   │
│   ├── ingestion/ (Document Ingestion)
│   │   ├── __init__.py
│   │   ├── hwp_parser.py (HWP 파일 파싱)
│   │   ├── pdf_parser.py (PDF + OCR)
│   │   ├── vlm_processor.py (Qwen2-VL 기반 VLM 추론)
│   │   ├── table_extractor.py (테이블·그래프 인식)
│   │   └── tests/
│   │
│   ├── mapping/ (기준 매핑 & RAG 인덱싱)
│   │   ├── __init__.py
│   │   ├── criterion_parser.py (경영평가 편람 파싱)
│   │   ├── embedding_service.py (한국어 임베딩 모델)
│   │   ├── vector_store.py (pgvector 관리)
│   │   ├── retriever.py (RAG 검색)
│   │   └── tests/
│   │
│   ├── scoring/ (등급 시뮬레이션)
│   │   ├── __init__.py
│   │   ├── benchmark_learner.py (A/B/C/D 등급 보고서 학습)
│   │   ├── grade_predictor.py (등급 예측)
│   │   ├── scenario_simulator.py (시나리오별 점수 계산)
│   │   └── tests/
│   │
│   ├── generation/ (보고서 초안 생성)
│   │   ├── __init__.py
│   │   ├── llm_client.py (EXAONE 3.5 호출)
│   │   ├── prompt_builder.py (프롬프트 템플릿)
│   │   ├── style_applier.py (한국 공문 스타일 적용)
│   │   └── tests/
│   │
│   ├── recommendation/ (Recommendation 엔진)
│   │   ├── __init__.py
│   │   ├── gap_analyzer.py (현재 vs 목표 Gap 분석)
│   │   ├── content_suggester.py (콘텐츠 제안)
│   │   ├── prioritizer.py (실현 가능성 우선순위)
│   │   └── tests/
│   │
│   ├── workers/ (Celery 비동기 작업자)
│   │   ├── __init__.py
│   │   ├── ingestion_worker.py (Document Ingestion 태스크)
│   │   ├── generation_worker.py (보고서 생성 태스크)
│   │   └── simulation_worker.py (등급 시뮬레이션 태스크)
│   │
│   ├── config/
│   │   ├── settings.py (환경 설정)
│   │   ├── logging.py
│   │   └── models.py (SQLAlchemy/Pydantic 모델)
│   │
│   ├── main.py 또는 app.py (FastAPI 서버 엔트리포인트)
│   ├── pyproject.toml
│   └── Dockerfile
│
├── pkg/ (공유 라이브러리)
│   ├── agents/ (에이전트 정의)
│   │   ├── __init__.py (Python) / init.go (Go)
│   │   ├── document_ingestion_agent.py/go (Document Ingestion 에이전트)
│   │   ├── mapping_agent.py/go (기준 매핑 에이전트)
│   │   ├── scoring_agent.py/go (등급 시뮬레이션 에이전트)
│   │   ├── generation_agent.py/go (보고서 생성 에이전트)
│   │   └── recommendation_agent.py/go (Recommendation 에이전트)
│   │
│   ├── models/ (공유 데이터 모델)
│   │   ├── document.py/go (문서 구조)
│   │   ├── report.py/go (실적보고서 구조)
│   │   ├── criterion.py/go (평가기준 구조)
│   │   └── simulation.py/go (등급 시뮬레이션 결과)
│   │
│   ├── errors/ (공유 에러 정의)
│   │   ├── __init__.py (Python) / errors.go (Go)
│   │   └── custom_errors.py/go
│   │
│   └── logging/ (공유 로깅)
│       ├── __init__.py (Python) / init.go (Go)
│       └── logger.py/go
│
├── schemas/ (공유 계약 정의)
│   ├── proto/ (Protocol Buffers)
│   │   ├── buf.yaml
│   │   ├── document.proto
│   │   ├── report.proto
│   │   ├── criterion.proto
│   │   ├── simulation.proto
│   │   ├── recommendation.proto
│   │   ├── workflow.proto
│   │   └── Makefile (protoc 빌드)
│   │
│   └── openapi/ (OpenAPI/Swagger)
│       ├── openapi.yaml
│       └── schemas/ (재사용 가능한 OpenAPI 스키마)
│
├── deployments/ (K8s + Helm)
│   ├── helm/
│   │   └── iroum-ax/
│   │       ├── Chart.yaml
│   │       ├── values.yaml (기본 설정)
│   │       ├── values-dev.yaml
│   │       ├── values-prod.yaml
│   │       ├── templates/
│   │       │   ├── control-plane-deployment.yaml
│   │       │   ├── console-deployment.yaml
│   │       │   ├── ingestion-worker-deployment.yaml
│   │       │   ├── postgresql-statefulset.yaml
│   │       │   ├── redis-statefulset.yaml
│   │       │   ├── service.yaml
│   │       │   ├── ingress.yaml
│   │       │   ├── configmap.yaml
│   │       │   ├── secret.yaml
│   │       │   └── rbac.yaml (K8s RBAC 설정)
│   │       └── README.md (Helm 배포 가이드)
│   │
│   ├── kustomize/ (선택사항: 다중 환경 매니지먼트)
│   │   ├── base/
│   │   └── overlays/
│   │       ├── dev/
│   │       └── prod/
│   │
│   └── scripts/
│       ├── deploy.sh (배포 자동화)
│       └── setup.sh (초기 설정)
│
├── docs/ (프로젝트 문서)
│   ├── architecture.md (아키텍처 상세)
│   ├── api.md (API 레퍼런스)
│   ├── deployment.md (배포 가이드)
│   ├── contributing.md (기여 가이드)
│   ├── faq.md
│   └── troubleshooting.md
│
├── tests/ (통합 테스트)
│   ├── e2e/ (엔드-투-엔드 테스트)
│   │   └── test_document_to_report.py (Document Ingestion → 초안 생성 통합)
│   ├── integration/ (계층 간 통합 테스트)
│   │   └── test_control_plane_integration.py
│   └── fixtures/ (테스트 데이터)
│
├── .moai/
│   ├── config/
│   │   └── sections/
│   │       ├── user.yaml
│   │       └── language.yaml
│   ├── project/
│   │   ├── product.md (본 문서의 참고 대상)
│   │   ├── structure.md (본 파일)
│   │   ├── tech.md
│   │   └── interview.md
│   ├── specs/ (SPEC 문서)
│   │   ├── SPEC-AX-001/ (안전보건 PoC Walking Skeleton)
│   │   │   ├── spec.md
│   │   │   └── acceptance-criteria.md
│   │   └── SPEC-AX-002/ (첫 확장 평가항목)
│   │       └── spec.md
│   └── db/
│       ├── schema/ (데이터베이스 스키마)
│       │   ├── initial.sql
│       │   └── migrations/
│       └── seeds/ (테스트 데이터)
│
├── Dockerfile (멀티스테이지 빌드)
├── docker-compose.yml (로컬 개발 환경)
├── Makefile (빌드·테스트 자동화)
├── .github/
│   └── workflows/
│       ├── ci.yml (테스트·린트·빌드)
│       └── deploy.yml (배포 파이프라인)
│
└── .claude/
    ├── rules/ (프로젝트 규칙)
    ├── agents/ (MoAI 에이전트 정의)
    ├── skills/ (MoAI 스킬)
    └── commands/ (클로드 코드 커맨드)
```

---

## 3. 모듈 책임 경계

### 3.1 핵심 모듈 정의

| 모듈 | 언어 | 책임 | 진입점 | 의존성 |
|------|------|------|--------|--------|
| **control-plane** | Go | 워크플로우 오케스트레이션, 에이전트 라이프사이클, 상태 관리 | gRPC server (50051), REST (8080) | PostgreSQL, Redis |
| **ingestion** | Python | HWP/PDF 파싱, OCR (VLM), 테이블 추출 | FastAPI 엔드포인트, Celery 워커 | Qwen2-VL (vLLM) |
| **mapping** | Python | 평가편람 파싱, RAG 인덱싱, 벡터 검색 | FastAPI 엔드포인트 | pgvector, 임베딩 모델 |
| **scoring** | Python | 벤치마크 학습, 등급 예측, 시나리오 계산 | FastAPI 엔드포인트, Celery 워커 | 학습 데이터 (A/B/C/D 보고서) |
| **generation** | Python | 초안 생성 (LLM), 스타일 적용 | FastAPI 엔드포인트, Celery 워커 | EXAONE 3.5 (vLLM) |
| **recommendation** | Python | Gap 분석, 콘텐츠 제안, 우선순위 지정 | FastAPI 엔드포인트 | 벤치마크 데이터, 등급 점수 |
| **console** | TypeScript | 대시보드, 문서 뷰어, 시뮬레이션 UI | Next.js (3000) | control-plane REST API |

### 3.2 계층 간 통신 규약

#### Python ↔ Control Plane (Go)
- **방식**: gRPC (성능) + REST (관리)
- **스키마**: `schemas/proto/workflow.proto`, `schemas/proto/task.proto`
- **예시**: control-plane → "문서 수집 태스크 생성" → Celery 워커 스케줄 → 완료 콜백

#### Console ↔ Control Plane
- **방식**: REST API (JSON)
- **스키마**: `schemas/openapi/api.yaml`
- **인증**: JWT 토큰 (공공기관 SSO 연동 _TBD)

#### Python 모듈 간
- **방식**: 함수 호출 또는 Celery 이벤트
- **스키마**: Pydantic 데이터 클래스

---

## 4. 데이터 플로우

```
사용자 (Console)
    ↓
Control Plane (REST API 수신)
    ↓
Workflow 상태 머신 (PostgreSQL 저장)
    ↓
Celery 워커 스케줄 (비동기)
    ↓
[Python 파이프라인]
  Document Ingestion → Mapping (RAG) → Scoring → Generation → Recommendation
    ↓
결과 저장 (PostgreSQL + Redis 캐시)
    ↓
Console (대시보드에 표시)
```

---

## 5. 데이터베이스 스키마 (개략)

```sql
-- 문서
CREATE TABLE documents (
  id UUID PRIMARY KEY,
  filename VARCHAR NOT NULL,
  file_type ENUM('HWP', 'PDF', 'IMAGE'),
  parsed_text TEXT,
  created_at TIMESTAMP
);

-- 평가 기준 (평가편람 인덱싱)
CREATE TABLE criteria (
  id UUID PRIMARY KEY,
  criterion_name VARCHAR NOT NULL,
  criterion_detail TEXT,
  max_points INT,
  embedding VECTOR(1536),  -- pgvector
  created_at TIMESTAMP
);

-- 실적보고서 (자사 또는 벤치마크)
CREATE TABLE reports (
  id UUID PRIMARY KEY,
  organization_name VARCHAR,
  grade ENUM('A', 'B', 'C', 'D'),
  content TEXT,
  score INT,
  created_at TIMESTAMP
);

-- 워크플로우 실행 (control-plane 관리)
CREATE TABLE workflows (
  id UUID PRIMARY KEY,
  user_id UUID,
  status ENUM('PENDING', 'RUNNING', 'COMPLETED', 'FAILED'),
  document_id UUID REFERENCES documents,
  report_id UUID REFERENCES reports,
  result_json JSONB,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

-- 등급 시뮬레이션 결과
CREATE TABLE simulations (
  id UUID PRIMARY KEY,
  workflow_id UUID REFERENCES workflows,
  current_grade ENUM('A', 'B', 'C', 'D'),
  target_grade ENUM('A', 'B', 'C', 'D'),
  probability DECIMAL(3, 2),
  recommendations JSONB,
  created_at TIMESTAMP
);
```

---

## 6. 향후 추가 예정 모듈 (확장 단계)

### Phase 2 (2027 초)
- **domain-adapter** (Python) — ESG/감사/면허 도메인 적응 레이어
- **multi-domain-console** (TypeScript) — 도메인 선택 UI

### Phase 3 (2027 중반)
- **api-gateway** (Go) — 다중 테넌트 관리 (각 고객사별 격리)
- **audit-logger** (Go) — 감시 로그 수집 (규제 준수)

### Phase 4 (2027 후반)
- **analytics** (Python) — 사용 현황, 성능 분석
- **feedback-loop** (Python) — 모델 개선 피드백 수집

---

## 7. 개발 환경 설정

### 7.1 로컬 환경 (docker-compose)

```bash
docker-compose up  # PostgreSQL, Redis, vLLM, Control Plane, Console
```

### 7.2 필수 도구
- Go 1.22+
- Python 3.11+ (with Poetry)
- Node.js 20+ (with pnpm)
- Docker + Docker Compose
- kubectl 1.24+
- Helm 3.10+

---

## 8. 의존성 관리

### 8.1 Go 모듈 (go.mod)
```
module github.com/ircp/iroum-ax/control-plane

require (
  github.com/grpc-ecosystem/grpc-gateway/v2 v2.x.x
  github.com/google/protobuf v3.x.x
  ...
)
```

### 8.2 Python (pyproject.toml)
```
[tool.poetry.dependencies]
python = "^3.11"
fastapi = "^0.100.0"
sqlalchemy = "^2.0.0"
pgvector = "^0.2.0"
celery = "^5.3.0"
torch = "^2.0.0"
transformers = "^4.30.0"  # Qwen2-VL, EXAONE
vllm = "^0.2.0"
```

### 8.3 TypeScript (package.json)
```json
{
  "dependencies": {
    "next": "^14.0.0",
    "react": "^18.2.0",
    "tailwindcss": "^3.3.0"
  }
}
```

---

## 9. 빌드 및 배포 자동화

### 9.1 Makefile 주요 타겟
```makefile
make build          # 모든 컴포넌트 빌드
make test           # 전체 테스트 실행
make lint           # 린트 체크
make docker-build   # 도커 이미지 빌드
make helm-deploy    # K8s 배포
```

### 9.2 CI/CD 파이프라인 (.github/workflows)
1. **PR 체크**: 테스트 → 린트 → 빌드
2. **병합 후**: 도커 이미지 푸시 → 개발 환경 자동 배포
3. **릴리스**: 릴리스 태그 생성 → 운영 환경 배포 (수동 승인)

---

## 10. 의존성 및 위험 분석

### 10.1 레이어 간 의존성 (DAG)

```
Console
  ↓
Control Plane ← Schemas (Proto + OpenAPI)
  ↓
Pipelines (Python 모듈들)
  ↓
[외부 의존성]
  ├── vLLM (Qwen2-VL, EXAONE 3.5)
  ├── pgvector (PostgreSQL)
  ├── Redis (캐시)
  └── 고객 데이터 (평가편람, 보고서)
```

### 10.2 위험 완화 전략

| 위험 | 영향 | 완화 |
|------|------|------|
| **vLLM 정지** | Inference 불가 | 재시작 자동화, 헬스 체크 |
| **PostgreSQL 장애** | 데이터 손실 | 일일 백업, 복제본 |
| **고객 네트워크 단절** | 서비스 중단 | 오프라인 모드, 로컬 캐시 |

---

## 11. 모니터링 및 로깅

### 11.1 메트릭 수집 (Prometheus)
```
iroum_ax_document_processing_duration_seconds (히스토그램)
iroum_ax_grade_prediction_accuracy (게이지)
iroum_ax_api_request_latency_seconds (히스토그램)
```

### 11.2 로그 수집 (Loki)
- Control Plane: 구조화된 JSON 로그 (Zap)
- Pipelines: 파이썬 로깅 (구조화 JSON)
- Console: 브라우저 콘솔 + 서버 로그

---

## 12. 문서 및 배포 명령어

### 12.1 프로젝트 초기화
```bash
git clone https://github.com/ircp/iroum-ax.git
cd iroum-ax
make setup  # 의존성 설치
make docker-build
docker-compose up -d  # 로컬 개발 환경 시작
```

### 12.2 K8s 배포
```bash
helm install iroum-ax ./deployments/helm/iroum-ax \
  -f values-prod.yaml \
  --namespace iroum-ax
```

---

**참고**: 본 문서는 계획 단계 구조입니다. 구현 과정에서 필요에 따라 조정될 수 있습니다. SPEC 진행 시 `product.md`, `tech.md`와 함께 참조하세요.
