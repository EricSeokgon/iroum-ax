# iroum-ax Makefile
# Sprint 0 스켈레톤 — 각 타겟은 placeholder + 실제 명령 주석
# make setup → make dev-up → make test 순서로 진행

.PHONY: setup build test lint format dev-up dev-down docker-build clean proto-gen db-migrate

# ============================================================
# 기본 타겟
# ============================================================

## 도움말 출력
help:
	@echo "iroum-ax Makefile 타겟:"
	@echo "  setup        - 의존성 설치 (Go + Python + Node)"
	@echo "  build        - 모든 컴포넌트 빌드"
	@echo "  test         - 전체 테스트 실행 (lint → unit → integration)"
	@echo "  lint         - 코드 린트 (ruff + mypy + go vet)"
	@echo "  format       - 코드 포맷팅 (ruff format + gofmt)"
	@echo "  dev-up       - 로컬 개발 환경 시작 (docker compose)"
	@echo "  dev-down     - 로컬 개발 환경 종료"
	@echo "  docker-build - Docker 이미지 빌드"
	@echo "  clean        - 빌드 아티팩트 정리"
	@echo "  proto-gen    - Protobuf 코드 생성 (buf generate)"
	@echo "  db-migrate   - 데이터베이스 마이그레이션 실행"

# ============================================================
# 환경 설정
# ============================================================

## 의존성 설치 (Go + Python + Node)
setup:
	@echo "[setup] Go 의존성 설치..."
	go mod download
	@echo "[setup] Python 의존성 설치 (Poetry)..."
	poetry install --with dev
	@echo "[setup] Node.js 의존성 설치..."
	npm install
	@echo "[setup] .env 파일 초기화..."
	@[ -f .env ] || cp .env.example .env
	@echo "[setup] 완료. 다음 단계: make dev-up"

# ============================================================
# 빌드
# ============================================================

## Control Plane 바이너리 빌드
build:
	@echo "[build] Go Control Plane 빌드..."
	CGO_ENABLED=0 go build -ldflags="-w -s" \
		-o bin/control-plane \
		./apps/control-plane/
	@echo "[build] 완료: bin/control-plane"

# ============================================================
# 테스트 (lint가 선행 조건)
# ============================================================

## 전체 테스트 실행 (lint → unit → integration)
test: lint
	@echo "[test] Python 단위 테스트 실행..."
	poetry run pytest tests/ pipelines/ -m "not gpu" --cov=pipelines --cov=pkg \
		--cov-report=term-missing --cov-fail-under=0
	@echo "[test] Go 단위 테스트 실행..."
	go test ./... -count=1
	@echo "[test] 완료"

## GPU 테스트 (GPU 환경에서만 실행)
test-gpu:
	@echo "[test-gpu] GPU 필요 테스트 실행..."
	poetry run pytest tests/ -m gpu -v

# ============================================================
# 린트
# ============================================================

## 코드 린트 (Python + Go)
lint:
	@echo "[lint] Python ruff 검사..."
	poetry run ruff check pipelines/ pkg/ tests/
	@echo "[lint] Python mypy 타입 검사..."
	poetry run mypy pipelines/ pkg/ --ignore-missing-imports
	@echo "[lint] Go vet 검사..."
	go vet ./...
	@echo "[lint] 완료"

## 코드 포맷팅
format:
	@echo "[format] Python ruff format..."
	poetry run ruff format pipelines/ pkg/ tests/
	@echo "[format] Go gofmt..."
	gofmt -w ./apps/
	@echo "[format] 완료"

# ============================================================
# 로컬 개발 환경
# ============================================================

## 로컬 개발 환경 시작 (PostgreSQL + Redis)
dev-up:
	@echo "[dev-up] PostgreSQL + Redis 시작..."
	docker compose up -d postgres redis
	@echo "[dev-up] 서비스 준비 대기..."
	@docker compose exec postgres pg_isready -U ax -d iroum_ax || sleep 5
	@echo "[dev-up] 완료. 다음 단계: make test"

## 전체 앱 포함 개발 환경 시작
dev-up-full:
	@echo "[dev-up-full] 전체 서비스 시작 (app 프로파일)..."
	docker compose --profile app up -d
	@echo "[dev-up-full] 완료"

## GPU 포함 개발 환경 시작 (GPU 환경 전용)
dev-up-gpu:
	@echo "[dev-up-gpu] GPU 서비스 포함 시작..."
	docker compose --profile gpu --profile app up -d
	@echo "[dev-up-gpu] 완료"

## 로컬 개발 환경 종료
dev-down:
	@echo "[dev-down] 서비스 종료..."
	docker compose down
	@echo "[dev-down] 완료"

# ============================================================
# Docker 빌드
# ============================================================

## Docker 이미지 빌드
docker-build:
	@echo "[docker-build] 멀티스테이지 이미지 빌드..."
	docker build -t iroum-ax:dev .
	@echo "[docker-build] 완료: iroum-ax:dev"

# ============================================================
# 스키마 / DB
# ============================================================

## Protobuf 코드 생성 (buf 필요)
proto-gen:
	@echo "[proto-gen] buf generate..."
	cd schemas/proto && buf generate
	@echo "[proto-gen] 완료"

## 데이터베이스 마이그레이션 실행
db-migrate:
	@echo "[db-migrate] 마이그레이션 실행..."
	psql "postgresql://ax:devpass@localhost:5432/iroum_ax" \
		-f .moai/db/schema/initial.sql
	@echo "[db-migrate] 완료"

# ============================================================
# 정리
# ============================================================

## 빌드 아티팩트 정리
clean:
	@echo "[clean] Go 빌드 아티팩트 정리..."
	rm -rf bin/
	@echo "[clean] Python 캐시 정리..."
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	find . -type d -name .pytest_cache -exec rm -rf {} + 2>/dev/null || true
	find . -type d -name .mypy_cache -exec rm -rf {} + 2>/dev/null || true
	find . -type d -name .ruff_cache -exec rm -rf {} + 2>/dev/null || true
	@echo "[clean] 완료"
