"""pytest 설정 및 픽스처 스켈레톤

testcontainers-python을 사용하여 PostgreSQL(pgvector), Redis 컨테이너를
pytest가 자동으로 시작/종료한다.

실제 픽스처 구현은 Sprint 1 RED 페이즈에서 추가됨.
"""
from __future__ import annotations

import pytest


# ============================================================
# Sprint 1에서 구현할 픽스처 목록 (TODO)
# ============================================================

# @pytest.fixture(scope="session")
# def postgres_container():
#     """testcontainers PostgreSQL + pgvector 컨테이너 (세션 단위)"""
#     from testcontainers.postgres import PostgresContainer
#     with PostgresContainer("pgvector/pgvector:pg16") as pg:
#         # pgvector 확장 활성화
#         # 스키마 초기화 (.moai/db/schema/initial.sql)
#         yield pg

# @pytest.fixture(scope="session")
# def redis_container():
#     """testcontainers Redis 컨테이너 (세션 단위)"""
#     from testcontainers.redis import RedisContainer
#     with RedisContainer("redis:7-alpine") as redis:
#         yield redis

# @pytest.fixture(scope="session")
# def db_session(postgres_container):
#     """SQLAlchemy 비동기 세션 픽스처"""
#     ...

# @pytest.fixture(scope="session")
# def celery_app(redis_container):
#     """테스트용 Celery 앱 픽스처 (ALWAYS_EAGER 모드)"""
#     ...

# @pytest.fixture(scope="function")
# def test_client():
#     """FastAPI TestClient 픽스처"""
#     from httpx import AsyncClient
#     from pipelines.main import app
#     ...


@pytest.fixture(scope="session", autouse=True)
def _sprint_0_placeholder() -> None:
    """Sprint 0 placeholder — Sprint 1 RED 페이즈에서 제거됨"""
    pass
