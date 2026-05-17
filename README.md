# iroum-ax

[![CI](https://github.com/EricSeokgon/iroum-ax/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/EricSeokgon/iroum-ax/actions/workflows/ci.yml)
[![CodeQL](https://github.com/EricSeokgon/iroum-ax/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/EricSeokgon/iroum-ax/actions/workflows/codeql.yml)
[![License: Private](https://img.shields.io/badge/License-Private-red.svg)](LICENSE)
[![Python](https://img.shields.io/badge/python-3.11-blue.svg)](pyproject.toml)
[![Go](https://img.shields.io/badge/go-1.22-00ADD8.svg)](go.mod)
[![Tests](https://img.shields.io/badge/tests-490+_passing-brightgreen.svg)](#)
[![SPEC](https://img.shields.io/badge/SPECs-7_GREEN-purple.svg)](#)
[![Security](https://img.shields.io/badge/Algorithm_Confusion_Attack-Defended-blue.svg)](#)

> 한국 공공기관 경영평가 보고서 자동화 AI 플랫폼 — KEPCO E&C anchor

한국 공공기관 경영평가 AI 플랫폼 — 안전보건 PoC Walking Skeleton

KEPCO E&C anchor 고객 대상 경영평가 자동화 플랫폼. HWP 문서 수집부터 Gap 추천까지 5개 MVP 기능을 단일 워크플로우로 통과시키는 E2E 슬라이스.

> SPEC 참조: [.moai/specs/SPEC-AX-001/spec.md](.moai/specs/SPEC-AX-001/spec.md)

---

## 프로젝트 상태

**Walking Skeleton + Auth + Observability + ABAC 완료** (Sprint 0-7 + OBS + AUTH-003, 2026-05-18)

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

**Go RBAC REST/gRPC Handler** (SPEC-AX-AUTH-002 v0.1.2)
- 34개 신규 테스트 (28 unit + 6 E2E)
- default-deny 안전장치 (매핑 미정의 → 503 AUTHZ_MAPPING_MISSING)
- 체인 순서 강제 (auth → authz → handler)
- AUTH-001 SKIP unblock (grep count=0)
- plan-auditor PASS 0.92 + evaluator-active CONFIRM 0.8415

**Go Server Bootstrap + Dual Listener** (SPEC-AX-SERVER-001 v0.1.2)
- 30개 신규 테스트 (19 unit + 11 E2E/integration)
- cmd/server/{main,server,probes}.go — package main 전환 + 11-step 의존성 주입
- errgroup dual listener (gRPC :50051 + REST :8080) + graceful shutdown (SIGTERM/SIGINT, 30s timeout)
- Health/readiness probes (DB+Redis+JWKS) + audit trail (SERVER_STARTUP/SHUTDOWN)
- plan-auditor PASS 0.92 + evaluator-active CONFIRM 0.83

**Go 관측성 — Prometheus + OTel** (SPEC-AX-OBS-001 v0.1.2)
- 7개 core collector (HTTP 지연/gRPC 지연/workflow 전이/auth 거부/celery 작업/pg pool/authz 거부)
- `/metrics` endpoint + RBAC (`read:metrics` 권한, `MetricsAuthMiddleware` — authn 401 + authz 403 분리)
- gRPC `UnaryMetricsInterceptor` (chain 최외곽, 인증 실패도 계측)
- OpenTelemetry tracing skeleton (noop exporter, AlwaysSample — 망분리 정합)
- Dependency Inversion (`RejectionObserver` interface) via `internal/auth/observer.go` — circular import 영구 해소
- 24/24 AC GREEN, evaluator-active CONFIRM 89.0 (3 rounds), metrics 87.2% / observability 100%

**Go 경량 ABAC** (SPEC-AX-AUTH-003 v0.1.0)
- RBAC 위에 속성 기반 접근 제어 레이어 추가 (authn → authz → ABAC → handler)
- OwnershipCondition (X-Resource-Owner), OrgUnitCondition (iroum-ax-org:<unit>), TimeWindowCondition (KST 09:00–18:00)
- Admin(RoleAdmin) 전체 우회, 안전 무작동 (fail-safe no-op, REQ-ABAC-009)
- 외부 의존성 0 (no OPA/Casbin), 망분리 정합, time.LoadLocation 금지
- 30 AC 검증, abac.go 커버리지 98.5%, evaluator-active PASS 0.905

**품질**
- TRUST 5 PASS (모든 5가지 차원): Tested ✓ | Readable ✓ | Unified ✓ | Secured ✓ | Trackable ✓
- 7개 SPEC 통합 완료 (AX-001 + CTRL-001 + AUTH-001 + AUTH-002 + SERVER-001 + OBS-001 + AUTH-003)

---

## 빠른 시작

### 서버 부팅

```bash
# Control Plane 서버 시작 (Go main 진입점)
go run ./apps/control-plane/cmd/server

# 또는 테스트 실행
go test ./apps/control-plane/cmd/server/... -cover
```

**서버 리스너:**
- gRPC: `:50051` (protocol buffers)
- REST: `:8080` (HTTP/JSON)
- Readiness probe: `GET /ready` (DB + Redis + JWKS 검증)
- Liveness probe: `GET /health` (항상 200)
- plan-auditor PASS 0.92 (iter 2), evaluator-active CONFIRM 0.8415 (iter 3)
- 66개 @MX 태그 (44 ANCHOR + 13 NOTE + 9 WARN)
- **총 410+ 테스트 (Python 192 + Go 190 + 11 integration + 27 E2E), 55+ 커밋**

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
