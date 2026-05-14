"""공유 로거 유틸리티 스켈레톤

audit_logs INSERT 헬퍼는 Sprint 1(REQ-UBI)에서 구현됨.
현재는 구조화 JSON 로거 설정만 제공.
"""
from __future__ import annotations

import logging
import sys
from typing import Any

from pipelines.config.settings import settings


def get_logger(name: str) -> logging.Logger:
    """구조화 JSON 로거 반환

    Args:
        name: 로거 이름 (보통 __name__ 사용)

    Returns:
        설정된 Logger 인스턴스
    """
    logger = logging.getLogger(name)

    if not logger.handlers:
        handler = logging.StreamHandler(sys.stdout)
        handler.setFormatter(
            logging.Formatter(
                fmt='{"time": "%(asctime)s", "level": "%(levelname)s", '
                '"logger": "%(name)s", "message": "%(message)s"}',
                datefmt="%Y-%m-%dT%H:%M:%S",
            )
        )
        logger.addHandler(handler)
        logger.setLevel(settings.log_level.upper())

    return logger


# TODO(Sprint 1): audit_log_action 함수 구현 (REQ-UBI)
# async def audit_log_action(
#     user_id: str,
#     action: str,
#     resource_id: str,
#     resource_type: str,
#     details: dict[str, Any] | None = None,
# ) -> None:
#     """audit_logs 테이블에 행동 로그 INSERT (REQ-UBI)"""
#     ...
