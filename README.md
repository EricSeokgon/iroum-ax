# iroum-ax

[![CI](https://github.com/EricSeokgon/iroum-ax/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/EricSeokgon/iroum-ax/actions/workflows/ci.yml)
[![CodeQL](https://github.com/EricSeokgon/iroum-ax/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/EricSeokgon/iroum-ax/actions/workflows/codeql.yml)
[![License: Private](https://img.shields.io/badge/License-Private-red.svg)](LICENSE)
[![Python](https://img.shields.io/badge/python-3.11-blue.svg)](pyproject.toml)
[![Go](https://img.shields.io/badge/go-1.22-00ADD8.svg)](go.mod)
[![Tests](https://img.shields.io/badge/tests-380+_passing-brightgreen.svg)](#)
[![SPEC](https://img.shields.io/badge/SPECs-3_GREEN-purple.svg)](#)
[![Security](https://img.shields.io/badge/Algorithm_Confusion_Attack-Defended-blue.svg)](#)

> 한국 공공기관 경영평가 보고서 자동화 AI 플랫폼 — KEPCO E&C anchor

한국 공공기관 경영평가 AI 플랫폼 — 안전보건 PoC Walking Skeleton

KEPCO E&C anchor 고객 대상 경영평가 자동화 플랫폼. HWP 문서 수집부터 Gap 추천까지 5개 MVP 기능을 단일 워크플로우로 통과시키는 E2E 슬라이스.

> SPEC 참조: [.moai/specs/SPEC-AX-001/spec.md](.moai/specs/SPEC-AX-001/spec.md)

---

## 프로젝트 상태

**Walking Skeleton + Auth 완료** (Sprint 0-7, 2026-05-15)

**Python 파이프라인** (SPEC-AX-001 v0.1.2)
- 192개 단위 테스트 통과 (83% 커버리지)
- 17개 모듈 (ingestion, mapping, scoring, generation, workers, auth)
- 5개 AC 그룹 (Document Ingestion, Mapping, Simulation, Generation, Synthesis)

**Go Control Plane** (SPEC-AX-CTRL-001 v0.1.2)
- 95개 테스트 (79개 단위 + 11개 통합 + 5개 E2E)
- 12개 내부 패키지 (workflow, store, audit, scheduler, server, proto, auth)
- 5개 REQ-CTRL (State Machine, gRPC, REST, PostgreSQL, Celery)

**Go 인증 모듈** (SPEC-AX-AUTH-001 v0.1.1)
- 90개 신규 Go 테스트 + 15개 Python 테스트 = 105 신규 테스트
- SF-1 (발행자 검증) + SF-2 (알고리즘 혼동 공격 방어) 통합
- OAuth 2.0 BCP (refresh token rotation + family invalidation)
- 4 E2E PASS + 1 SKIP (REST handler SPEC-AX-AUTH-002 연기)

**품질**
- TRUST 5 PASS (모든 5가지 차원): Tested ✓ | Readable ✓ | Unified ✓ | Secured ✓ | Trackable ✓
- plan-auditor PASS (0.88점), evaluator-active CONFIRM (0.782점)
- 55개 @MX 태그 (40 ANCHOR + 10 NOTE + 5 WARN)
- **총 380+ 테스트 (Python 192 + Go 156 + 11 integration + 21 E2E), 50+ 커밋**

## 빠른 시작

```bash
# 1. 의존성 설치
make setup

# 2. 로컬 개발 환경 시작 (PostgreSQL + Redis + vLLM)
make dev-up

# 3. Python 파이프라인 테스트 (177개)
python -m pytest tests/unit/ -v

# 4. Go Control Plane 빌드 및 테스트 (95개)
cd apps/control-plane
go build ./cmd/server
go test ./... -v

# 5. 통합 테스트 (E2E: HWP 업로드 → Recommendation)
python -m pytest tests/integration/ -v
```

### 빠른 설정

```bash
# PostgreSQL DSN (필수 환경변수, 없으면 기본값 사용)
export POSTGRES_DSN="postgres://user:pass@localhost:5432/iroum_ax"

# Redis URL (필수 환경변수)
export REDIS_URL="redis://localhost:6379"

# Go Control Plane 시작
cd apps/control-plane
go run cmd/server/main.go

# gRPC 서버: localhost:50051
# REST API: http://localhost:8080
# 헬스체크: curl http://localhost:8080/healthz
```

---

## 구조 요약

```
iroum-ax/
├── apps/control-plane/   # Go — 워크플로우 오케스트레이터 (gRPC:50051, REST:8080)
├── pipelines/            # Python — VLM/RAG/Document AI (FastAPI:8000, Celery)
├── schemas/              # Protobuf + OpenAPI 계약 정의
├── deployments/helm/     # Helm Chart 스켈레톤 (K8s 배포)
├── tests/                # pytest 통합 테스트 (testcontainers)
└── .moai/specs/          # SPEC 문서 (SPEC-AX-001~)
```

---

## 기술 스택

| 계층 | 기술 |
|------|------|
| VLM (OCR) | Qwen2-VL 7B |
| 텍스트 LLM | Qwen 2.5 7B (CPU 직접 로딩) |
| 임베딩 | ko-sroberta-multitask (768 dim) |
| Vector DB | PostgreSQL 16 + pgvector (HNSW) |
| API | FastAPI + gRPC-Gateway v2 |
| 비동기 큐 | Celery + Redis |
| 오케스트레이터 | Go 1.22 + K8s (Helm) |

---

## 아키텍처 & 설계

모든 주요 설계 결정과 모듈 맵핑은 아래 문서에서 확인하세요:

- **[Architecture Overview](.moai/project/codemaps/overview.md)**: 3계층 아키텍처 (Go Control Plane + Python 파이프라인 + TypeScript UI) · 모노레포 구조 · E2E 데이터 흐름
- **[Go Control Plane](.moai/project/codemaps/go-control-plane.md)**: 12개 내부 패키지 · State Machine · gRPC/REST · PostgreSQL Store · Celery Dispatcher · @MX:ANCHOR 분석
- **[Python Pipelines](.moai/project/codemaps/pipelines.md)**: Python 17개 모듈 · REQ-AX 매핑 · @MX:ANCHOR
- **[Data Models](.moai/project/codemaps/pkg.md)**: 공유 모델 · Pydantic 스키마 · 에러 정의 · 로깅 구조
- **[Data Flow](.moai/project/codemaps/data-flow.md)**: HWP 입력 → Go orchestration → Python 처리 → Recommendation 출력
- **[Requirements Traceability](.moai/project/codemaps/req-traceability.md)**: REQ-CTRL + REQ-AX + REQ-UBI 매트릭스 · AC ↔ 구현 ↔ 테스트 (272개 테스트 총합)

## 개발 명령어

```bash
make lint         # ruff + mypy + go vet
make format       # ruff format + gofmt
make test         # lint → pytest → go test (모든 177개 테스트)
make dev-down     # 로컬 환경 종료
make docker-build # Docker 이미지 빌드
```

## 다음 단계 (후속 SPEC)

| Sprint | SPEC 후보 | 범위 |
|--------|-----------|------|
| 7 | SPEC-AX-CTRL-001 | Go Control Plane 구현 (gRPC/REST 서버) |
| 8 | SPEC-AX-E2E-001 | 통합 테스트 (Helm 배포 후 validation) |
| 9 | SPEC-AX-COV-001 | 커버리지 82% → 85% |
| - | SPEC-AX-EXPANDED-001 | 다중 평가항목 (안전보건 → 500개 전체) |
| Phase 3 | SPEC-AX-{ESG,AUDIT,LICENSE}-001 | 인접 도메인 확장 |
| Phase 4+ | SPEC-AX-FINTECH-001 | 금융권 규제 보고서 (조건: 공공 anchor 성공 3+ 확보) |

---

## 아키텍처 결정 기록 (ADR)

주요 설계 결정은 [docs/adr/](docs/adr/)에 영구 보존됩니다.

핵심 ADR:
- [0002 자체 호스팅 LLM + 망분리 정합](docs/adr/0002-self-hosted-llm-data-sovereignty.md)
- [0003 abstain 3-way softmax 분류](docs/adr/0003-abstain-3way-softmax-classifier.md)
- [0004 Celery-from-Go Redis-direct envelope](docs/adr/0004-celery-from-go-redis-direct.md)
- [0006 Keycloak OIDC provider 선정](docs/adr/0006-keycloak-oidc-provider.md)

모든 ADR 목록은 [docs/adr/README.md](docs/adr/README.md)를 참조하세요.

---

## 라이선스

Private — KEPCO E&C PoC 전용
