"""SPEC-AX-INTEG-001 — Celery app 초기화 + 부팅 환경변수 검증

REQ-INTEG-006 / REQ-INTEG-008 / AC-INTEG-001-7 구현:
- GO_CONTROL_PLANE_URL 누락/빈 문자열 → ValueError로 부팅 실패
- VLLM_ENDPOINT 외부 URL → ExternalLLMBlockedError로 부팅 실패 (allowlist 검증)
- CELERY_BROKER_URL 누락 → Celery 기본 동작 (Settings.redis_url로 자동 fallback)

본 모듈은 Celery 패키지 미설치 환경(단위 테스트)에서도 import 가능하도록
검증 헬퍼를 Celery import 없이 구현한다. Celery app 생성은 별도 함수에 격리.
"""
from __future__ import annotations

import os

from pipelines.config.settings import validate_llm_endpoint


def validate_worker_environment(
    *,
    vlm_endpoint: str,
    go_control_plane_url: str,
) -> bool:
    """Celery worker 부팅 시점 환경변수 검증.

    REQ-INTEG-008 (REQ-UBI-001): vlm_endpoint가 non-empty이면 localhost-only 검증.
    빈 문자열은 transformers 직접 로딩 모드 (settings.py VLLM_ENDPOINT=="" 정합) — 통과.

    REQ-INTEG-006: go_control_plane_url empty 거부.

    Args:
        vlm_endpoint: env VLLM_ENDPOINT 값 (빈 문자열 허용 — transformers opt-out)
        go_control_plane_url: Go Control Plane base URL (빈 문자열 거부)

    Returns:
        True — 모든 검증 통과

    Raises:
        ExternalLLMBlockedError: vlm_endpoint가 non-empty이면서 외부 host인 경우
        ValueError: go_control_plane_url이 빈 문자열인 경우
    """
    # 1) GO_CONTROL_PLANE_URL — 빈 문자열 거부 (REQ-INTEG-006)
    if not go_control_plane_url:
        raise ValueError(
            "GO_CONTROL_PLANE_URL 환경변수가 비어 있습니다 — Celery worker 부팅 실패 "
            "(REQ-INTEG-006). docker-compose 또는 .env에 'http://localhost:8080' 등을 주입하세요."
        )

    # 2) VLLM_ENDPOINT — non-empty이면 localhost-only 검증 (REQ-INTEG-008, REQ-UBI-001)
    # 빈 문자열은 transformers 직접 로딩 모드 (settings.py:124-127) — 통과
    if vlm_endpoint:
        validate_llm_endpoint(vlm_endpoint)  # 외부 URL → ExternalLLMBlockedError

    return True


def validate_worker_environment_from_env() -> bool:
    """os.environ 직접 읽어 worker 부팅 검증 (Celery worker 진입점에서 호출).

    Returns:
        True — 모든 검증 통과 (raise되지 않은 경우)
    """
    return validate_worker_environment(
        vlm_endpoint=os.environ.get("VLLM_ENDPOINT", ""),
        go_control_plane_url=os.environ.get("GO_CONTROL_PLANE_URL", ""),
    )


# ─────────────────────────────────────────────────────────────────────────────
# Celery app 생성 — celery 패키지가 설치된 환경에서만 동작.
# 단위 테스트 환경(celery 미설치)에서는 import 자체를 회피하기 위해
# 함수 내부 lazy import 사용.
# ─────────────────────────────────────────────────────────────────────────────


def create_celery_app(broker_url: str | None = None):  # noqa: ANN201 — celery 미설치 환경 대응
    """Celery app 인스턴스를 생성한다 (lazy import).

    REQ-INTEG-001 — task name 'pipelines.workers.ingestion_worker.run'가
    Celery autodiscover에 의해 등록된다 (pipelines.workers 패키지 스캔).

    Args:
        broker_url: Celery broker URL. None이면 CELERY_BROKER_URL 환경변수 또는
                    redis://localhost:6379/0 (Settings.redis_url 정합).

    Returns:
        celery.Celery 인스턴스
    """
    # lazy import — celery 패키지 미설치 환경(단위 테스트)에서 ImportError 회피
    from celery import Celery  # type: ignore[import-not-found]

    if broker_url is None:
        broker_url = os.environ.get("CELERY_BROKER_URL", "redis://localhost:6379/0")

    app = Celery("pipelines", broker=broker_url, backend=None)
    # task autodiscover — pipelines.workers 패키지의 모든 @app.task 등록
    app.autodiscover_tasks(["pipelines.workers"])
    return app
