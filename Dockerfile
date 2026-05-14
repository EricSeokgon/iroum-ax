# iroum-ax 멀티스테이지 Dockerfile
# Sprint 0 스켈레톤 — 프로덕션 빌드는 Sprint 7+ 이후 완성
# GPU 런타임은 미사용 (CPU primary path, pytest -m gpu opt-in 별도)

# =============================================================
# Stage 1: Go 빌드 (Control Plane)
# =============================================================
FROM golang:1.22-alpine AS go-builder

WORKDIR /build

# 의존성 캐시 최적화: go.mod/go.sum 먼저 복사
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사 및 빌드
COPY apps/control-plane/ ./apps/control-plane/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" \
    -o /bin/control-plane \
    ./apps/control-plane/

# =============================================================
# Stage 2: Python 런타임 (Pipelines)
# =============================================================
FROM python:3.11-slim AS python-base

# 보안: 비루트 사용자 생성
RUN groupadd --gid 1000 axuser && \
    useradd --uid 1000 --gid axuser --shell /bin/bash axuser

WORKDIR /app

# 시스템 의존성 (HWP 파싱 + PDF 처리)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# Python 의존성 설치 (poetry 없이 pip 사용)
# TODO(Sprint 7): poetry export --without-hashes -f requirements.txt 로 고정
COPY pyproject.toml ./
# CPU-only torch: GPU 미사용 환경
RUN pip install --no-cache-dir \
    torch --index-url https://download.pytorch.org/whl/cpu && \
    pip install --no-cache-dir poetry==1.8.2 && \
    poetry config virtualenvs.create false && \
    poetry install --only main --no-root

# 애플리케이션 소스 복사
COPY pipelines/ ./pipelines/
COPY pkg/ ./pkg/

# =============================================================
# Stage 3: 최종 런타임 이미지
# =============================================================
FROM python-base AS runtime

# Go 바이너리 복사
COPY --from=go-builder /bin/control-plane /usr/local/bin/control-plane

# 비루트 사용자로 실행
USER axuser

# 헬스 체크 (Pipelines FastAPI)
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD python -c "import httpx; httpx.get('http://localhost:8000/health').raise_for_status()"

# 기본 진입점: FastAPI Pipelines
# Control Plane은 별도 서비스로 실행 (docker-compose 참조)
CMD ["uvicorn", "pipelines.main:app", "--host", "0.0.0.0", "--port", "8000"]
