# iroum-ax

한국 공공기관 경영평가 AI 플랫폼 — 안전보건 PoC Walking Skeleton

KEPCO E&C anchor 고객 대상 경영평가 자동화 플랫폼. HWP 문서 수집부터 Gap 추천까지 5개 MVP 기능을 단일 워크플로우로 통과시키는 E2E 슬라이스.

> SPEC 참조: [.moai/specs/SPEC-AX-001/spec.md](.moai/specs/SPEC-AX-001/spec.md)

---

## 빠른 시작

```bash
# 1. 의존성 설치
make setup

# 2. 로컬 개발 환경 시작 (PostgreSQL + Redis)
make dev-up

# 3. 테스트 실행 (Sprint 1 이후 실제 테스트 추가됨)
make test
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

## 개발 명령어

```bash
make lint         # ruff + mypy + go vet
make format       # ruff format + gofmt
make test         # lint → pytest → go test
make dev-down     # 로컬 환경 종료
make docker-build # Docker 이미지 빌드
```

---

## 라이선스

Private — KEPCO E&C PoC 전용
