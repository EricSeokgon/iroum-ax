# iroum-ax

한국 공공기관 경영평가 AI 플랫폼 — 안전보건 PoC Walking Skeleton

KEPCO E&C anchor 고객 대상 경영평가 자동화 플랫폼. HWP 문서 수집부터 Gap 추천까지 5개 MVP 기능을 단일 워크플로우로 통과시키는 E2E 슬라이스.

> SPEC 참조: [.moai/specs/SPEC-AX-001/spec.md](.moai/specs/SPEC-AX-001/spec.md)

---

## 프로젝트 상태

**Walking Skeleton 완료** (Sprint 0-6, 2026-05-14)

- 177개 단위 테스트 통과 (82% 커버리지)
- TRUST 5 품질 게이트 통과
- plan-auditor PASS (0.86점), evaluator-active CONFIRM (0.813점)
- 17개 Python 파이프라인 모듈 + 5개 공유 모델
- 24개 AC (Acceptance Criteria) 100% 구현

## 빠른 시작

```bash
# 1. 의존성 설치
make setup

# 2. 로컬 개발 환경 시작 (PostgreSQL + Redis + vLLM)
make dev-up

# 3. 단위 테스트 실행 (177개 passing)
python -m pytest tests/unit/ -v

# 4. E2E 테스트 (HWP 업로드 → Recommendation)
python -m pytest tests/e2e/ -v
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

- **[Architecture Overview](.moai/project/codemaps/overview.md)**: 전체 시스템 레이어 · E2E 데이터 흐름
- **[Pipeline Modules](.moai/project/codemaps/pipelines.md)**: Python 17개 모듈 · REQ-AX 매핑 · @MX:ANCHOR
- **[Data Models](.moai/project/codemaps/pkg.md)**: 5개 Pydantic 모델 · 에러 정의 · 로깅 구조
- **[Data Flow](.moai/project/codemaps/data-flow.md)**: HWP 입력 → Recommendation 출력 단계별 흐름
- **[Requirements Traceability](.moai/project/codemaps/req-traceability.md)**: AC ↔ 구현 파일 ↔ 테스트 매트릭스 (24개 AC, 177개 테스트)

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

## 라이선스

Private — KEPCO E&C PoC 전용
