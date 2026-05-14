"""pytest 설정 및 픽스처

testcontainers-python을 사용하여 PostgreSQL(pgvector), Redis 컨테이너를
pytest가 자동으로 시작/종료한다.

Sprint 2 (REQ-AX-001): mock_hwp_doc, mock_pdf_doc, mock_qwen2vl 픽스처 추가.
"""
from __future__ import annotations

from typing import Any
from unittest.mock import MagicMock

import pytest


# ============================================================
# pytest 마크 등록 (pyproject.toml에도 등록 필요)
# ============================================================


def pytest_configure(config: pytest.Config) -> None:
    """커스텀 pytest 마크 등록"""
    config.addinivalue_line(
        "markers", "gpu: GPU 환경에서만 실행 (pytest -m gpu 로 opt-in)"
    )
    config.addinivalue_line(
        "markers", "integration: 통합 테스트 — 외부 서비스 필요 (testcontainers 등)"
    )
    config.addinivalue_line(
        "markers", "slow_cpu: CPU 추론 포함 — 실행 시간이 길 수 있음"
    )


# ============================================================
# Sprint 2 REQ-AX-001 단위 테스트 픽스처
# ============================================================


@pytest.fixture()
def mock_hwp_doc() -> dict[str, Any]:
    """합성 HWP 문서 구조 모의 반환값.

    실제 HWP 파싱 대신 모의 데이터를 사용하는 단위 테스트용 픽스처.
    GREEN 페이즈에서 실제 파서 구현과 함께 픽스처도 보완됨.
    """
    return {
        "text": "안전보건 실적보고서\n안전교육 이수율 100%\n안전사고 발생건수 0건\n"
                "안전보건 관리체계 인증 획득\n근로자 안전보건 교육 실시",
        "tables": [
            {
                "rows": [
                    ["평가지표", "실적", "목표", "달성율"],
                    ["안전교육 이수율", "100%", "95%", "105%"],
                    ["안전사고 건수", "0건", "0건", "100%"],
                ],
                "rotation": 0,
            }
        ],
        "metadata": {
            "author": "KEPCO E&C 안전보건팀",
            "created_date": "2026-01-15",
            "sections": ["1. 개요", "2. 안전교육 실적", "3. 안전사고 현황", "4. 향후 계획"],
        },
    }


@pytest.fixture()
def mock_pdf_doc() -> dict[str, Any]:
    """합성 PDF 문서 구조 모의 반환값.

    회전 페이지가 포함된 PDF 테스트용.
    """
    return {
        "text": "PDF 안전보건 보고서\n안전교육 이수율 현황",
        "pages": [
            {
                "text": "일반 페이지 내용",
                "rotation": 0,
            },
            {
                "text": "헤더1\t헤더2\t헤더3\n값1\t값2\t값3",
                "rotation": 90,
            },
        ],
    }


@pytest.fixture()
def mock_qwen2vl() -> MagicMock:
    """Qwen2-VL 7B 모델 모의 객체.

    단위 테스트에서 실제 모델 로딩/추론을 방지.
    - generate() → OCR 텍스트 문자열 반환
    - device → 'cpu' (단위 테스트 기본값)
    """
    mock = MagicMock()
    mock.generate.return_value = "Qwen2-VL OCR 결과: 안전보건 실적보고서 텍스트"
    mock.device = "cpu"
    mock.config.model_type = "qwen2_vl"
    return mock


# ============================================================
# Sprint 1 픽스처 (기존, 유지)
# ============================================================


@pytest.fixture(scope="session", autouse=True)
def _sprint_0_placeholder() -> None:
    """Sprint 0 placeholder — Sprint 1 RED 페이즈에서 제거됨"""
    pass


# ============================================================
# Sprint 2 이후 활성화 예정 픽스처 (주석 처리)
# ============================================================

# @pytest.fixture(scope="session")
# def postgres_container():
#     """testcontainers PostgreSQL + pgvector 컨테이너 (세션 단위)"""
#     from testcontainers.postgres import PostgresContainer
#     with PostgresContainer("pgvector/pgvector:pg16") as pg:
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
