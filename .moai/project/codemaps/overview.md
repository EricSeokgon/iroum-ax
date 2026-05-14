# iroum-ax 아키텍처 개요

## 프로젝트 목표

iroum-ax는 정책 문서(HWP, PDF)를 자동으로 분석하여 적용 가능 등급 및 개선 방안을 생성하는 AI 추천 시스템이다.

## 3계층 아키텍처

```
┌─────────────────────────────────────────────────────────────┐
│ TypeScript Console (react 18, tailwindcss)                   │ [Deferred]
│ User Interface Layer                                         │
└────────────────────┬────────────────────────────────────────┘
                     │ REST + gRPC
┌────────────────────┴────────────────────────────────────────┐
│ Go Control Plane (v0.1.2, SPEC-AX-CTRL-001)                 │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │ gRPC Service (port 50051) + REST API (port 8080)        │ │
│ │ - CreateWorkflow, GetWorkflow, ListWorkflows            │ │
│ │ - Workflow State Machine (PENDING→RUNNING→COMPLETED)   │ │
│ │ - PostgreSQL Transaction Management                     │ │
│ │ - Celery v2 Dispatch via Redis                          │ │
│ └──────────────────────────────────────────────────────────┘ │
│ Orchestration Layer                                          │
└────────────────────┬────────────────────────────────────────┘
                     │ Redis (Celery Queue)
┌────────────────────┴────────────────────────────────────────┐
│ Python Celery Workers (v0.1.2, SPEC-AX-001)                │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │ Document Ingestion (HWP/PDF parsing + VLM OCR)          │ │
│ │ Criterion Mapping (RAG search + pgvector)               │ │
│ │ Grade Simulation (2-class prediction + scenario gen)    │ │
│ │ Report Draft Generation (EXAONE 3.5 7B via vLLM)        │ │
│ │ Recommendation Synthesis (종합 + 최적화)                 │ │
│ └──────────────────────────────────────────────────────────┘ │
│ Processing Layer                                             │
└─────────────────────────────────────────────────────────────┘
                     │
        ┌────────────┴────────────┐
        ↓                         ↓
    PostgreSQL 16            Redis 7
    (workflows,              (celery queue)
     audit_logs,
     documents,
     pgvector)
```

## 기술 스택

| 계층 | 기술 | 역할 |
|------|------|------|
| **인프라** | Docker Compose, Kubernetes(Helm) | 로컬 개발 + 프로덕션 배포 |
| **Go Control Plane** | gRPC, REST, pgx/v5, go-redis | 워크플로우 오케스트레이션 |
| **Python 파이프라인** | FastAPI, Celery, PostgreSQL 16, pgvector | 문서 처리, RAG, 등급 예측, 보고서 생성 |
| **TypeScript 콘솔** | React 18, TailwindCSS | 사용자 인터페이스 [Deferred] |
| **벡터 DB** | PostgreSQL + pgvector (HNSW) | 의료 기준 임베딩 저장/검색 |
| **LLM** | EXAONE 3.5 7B (vLLM), Qwen 2.5 7B | 보고서 생성 |
| **OCR/VLM** | Qwen2-VL 7B (vLLM) | 이미지 문서 처리 |

## 모노레포 구조

```
iroum-ax/
├── apps/
│   ├── control-plane/      (SPEC-AX-CTRL-001 v0.1.2) [GO]
│   │   ├── cmd/server/
│   │   ├── internal/
│   │   │   ├── workflow/   (State Machine, Handlers)
│   │   │   ├── store/      (PostgreSQL)
│   │   │   ├── scheduler/  (Celery Dispatcher)
│   │   │   ├── audit/      (Audit Logging)
│   │   │   ├── config/
│   │   │   └── proto/
│   │   ├── go.mod, go.sum
│   │   └── tests/
│   ├── console/            (React UI) [Deferred]
│   └── pipelines/          (SPEC-AX-001 v0.1.2) [PYTHON]
│       ├── ingestion/      (HWP/PDF parsing, VLM OCR)
│       ├── mapping/        (RAG, pgvector)
│       ├── scoring/        (Grade prediction, simulation)
│       ├── generation/     (LLM, report draft)
│       ├── workers/        (Celery tasks)
│       ├── config/
│       ├── tests/
│       └── requirements.txt
├── pkg/                    (Shared utilities)
│   ├── logging/            (AuditLogger)
│   ├── models/             (Database schemas)
│   └── proto/              (Protobuf shared)
├── schemas/
│   ├── proto/              (Workflow.proto, etc.)
│   ├── openapi/            (OpenAPI spec)
│   └── sql/
├── .moai/
│   ├── specs/
│   │   ├── SPEC-AX-001/    (Python pipeline)
│   │   └── SPEC-AX-CTRL-001/ (Go control plane)
│   └── project/
│       └── codemaps/       (Architecture docs)
└── Makefile, docker-compose.yml, Helm chart
```

## 데이터 흐름

```
[REST/gRPC 클라이언트]
    ↓
[Go Control Plane] → CreateWorkflow
    ├─→ PostgreSQL: INSERT workflows (status=PENDING)
    ├─→ PostgreSQL: INSERT audit_logs (action=WORKFLOW_CREATED)
    └─→ Redis: RPUSH celery_queue (Kombu v2 envelope)
        ↓
[Redis Celery Queue]
    ↓
[Python Celery Worker]
    ├─→ Document Ingestion (HWP/PDF parsing, VLM OCR)
    ├─→ Criterion Mapping (RAG + pgvector search)
    ├─→ Grade Simulation (2-class ML model)
    ├─→ Report Draft Generation (EXAONE 3.5 7B)
    └─→ Recommendation Synthesis
        ↓
[Go Control Plane] ← Worker callback
    ├─→ PostgreSQL: UPDATE workflows (status=COMPLETED, result=JSON)
    ├─→ PostgreSQL: INSERT audit_logs (action=WORKFLOW_COMPLETED)
    └─→ [REST/gRPC 클라이언트] ← Result
```

## 주요 특성

- **데이터 주권** (REQ-UBI-001): 모든 처리가 자체 호스팅 LLM/VLM에서 실행
- **한국어 Enforcement** (REQ-UBI-002): 입출력 모두 한국어만 지원
- **완전 감시 로깅** (REQ-UBI-003): 모든 주요 이벤트 audit_logs에 기록
- **원자성 보장** (SPEC-AX-CTRL-001 REQ-CTRL-UBI-001): 워크플로우 생성 시 audit INSERT 실패 → 전체 롤백
- **비동기 처리**: gRPC/REST 요청 수신 후 Redis 큐를 통해 Python 워커가 처리

## SPEC 추적

| SPEC | 버전 | 상태 | 범위 |
|------|------|------|------|
| SPEC-AX-001 | v0.1.2 | PASSED | Python 파이프라인 (5 REQ + 5 REQ-UBI) |
| SPEC-AX-CTRL-001 | v0.1.2 | PASSED | Go Control Plane (5 REQ + 2 REQ-UBI) |
| SPEC-AX-AUTH-001 | TBD | Deferred | 인증 + 멀티테넌트|
| SPEC-AX-UI-001 | TBD | Deferred | TypeScript React 콘솔 |

## 다음 단계

1. GitHub 푸시 (36+ 커밋)
2. PR 리뷰 대기
3. 후속 SPEC 후보:
   - SPEC-AX-COV-001: 통합 커버리지 측정
   - SPEC-AX-AUD-001: 감시 로그 에러 경로 보강
   - SPEC-AX-AUTH-001: gRPC TLS + 인증 + 멀티테넌트
   - SPEC-AX-UI-001: React 콘솔 + 대시보드

## 관련 코드맵

- [go-control-plane.md](go-control-plane.md) — 12개 Go 패키지 구조, fan-in/fan-out 분석
- [req-traceability.md](req-traceability.md) — REQ-CTRL 및 REQ-AX 매핑
- [data-flow.md](data-flow.md) — 문서 인입 → 결과 생성 흐름
