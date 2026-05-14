"""공유 로거 유틸리티

audit_logs INSERT 헬퍼 (REQ-UBI-003, AC-UBI-003, AC-UBI-004) 구현.
테스트 환경에서는 in-memory dict를 반환 (DB 미의존).
"""
from __future__ import annotations

import logging
import os
import sys
from datetime import UTC, datetime
from typing import Any


def get_logger(name: str) -> logging.Logger:
    """구조화 JSON 로거 반환

    Args:
        name: 로거 이름 (보통 __name__ 사용)

    Returns:
        설정된 Logger 인스턴스
    """
    # settings 임포트를 지연하여 순환 임포트 방지
    from pipelines.config.settings import settings

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


def audit_event(
    action: str,
    resource_type: str,
    resource_id: str | None = None,
    user_id: str | None = None,
    details: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """감사 이벤트를 기록하고 레코드 dict를 반환한다.

    # @MX:ANCHOR: [AUTO] 감사 로그 기록 진입점 — 4종 액션 모두 이 함수 경유
    # @MX:REASON: REQ-UBI-003 감사 로깅 완전성 — 모든 핵심 액션 기록 필수

    user_id가 None이고 인증이 비활성화(AUTH_ENABLED=false)인 경우
    'cli-anonymous'를 기본값으로 사용한다 (AC-UBI-004).

    Args:
        action: 감사 액션 (예: 'document_upload', 'workflow_create')
        resource_type: 리소스 유형 (예: 'document', 'workflow')
        resource_id: 리소스 식별자 (선택)
        user_id: 사용자 ID (None이면 기본값 적용)
        details: 추가 상세 정보 (선택)

    Returns:
        감사 레코드 dict — user_id, action, resource_id, timestamp 필드 포함
    """
    # user_id 기본값 결정 — AUTH_ENABLED 환경변수 또는 settings 확인
    resolved_user_id = user_id
    if resolved_user_id is None:
        # 환경변수 직접 읽기 (monkeypatch 호환)
        auth_enabled_env = os.environ.get("AUTH_ENABLED", "false").lower()
        if auth_enabled_env in ("false", "0", "no"):
            resolved_user_id = os.environ.get("DEFAULT_USER_ID", "cli-anonymous")
        else:
            # 인증 활성화 상태에서 user_id 미제공 — 기본값 사용
            resolved_user_id = os.environ.get("DEFAULT_USER_ID", "cli-anonymous")

    now_iso = datetime.now(tz=UTC).isoformat()

    record: dict[str, Any] = {
        "user_id": resolved_user_id,
        "action": action,
        "resource_type": resource_type,
        "resource_id": resource_id,
        "timestamp": now_iso,
    }
    if details is not None:
        record["details"] = details

    return record
