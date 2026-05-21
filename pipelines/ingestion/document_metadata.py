"""SPEC-AX-INGEST-001 — Celery task kwargs로부터 문서 메타데이터 추출

REQ-INGEST-001 / OPEN #4 결정: Celery envelope kwargs를 single-source-of-truth로
사용하며, 누락된 필드는 기본값으로 폴백 (Go REST 호출 없음 — Python-only 격리).
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass
class DocumentMeta:
    """문서 처리에 필요한 최소 메타데이터.

    Attributes:
        file_type: 'PDF' | 'HWP' | 'IMAGE' | 'unknown'
        file_path: 원본 파일 절대 경로 (빈 문자열 가능 — 폴백)
        user_id: 처리 요청자 ID (sandbox 환경에서는 default_user_id)
    """

    file_type: str
    file_path: str
    user_id: str


class DocumentMetadataClient:
    """Celery kwargs 기반 문서 메타데이터 추출기.

    Args:
        default_user_id: user_id 누락 시 폴백 (settings.default_user_id 권장)

    Example:
        client = DocumentMetadataClient(default_user_id=settings.default_user_id)
        meta = client.fetch("doc-001", kwargs={"file_type": "PDF", ...})
    """

    def __init__(self, default_user_id: str = "cli-anonymous") -> None:
        self._default_user_id = default_user_id

    def fetch(self, document_id: str, kwargs: dict[str, Any]) -> DocumentMeta:
        """kwargs로부터 메타데이터를 추출하고 누락 필드는 폴백한다.

        Args:
            document_id: 문서 ID (현재 미사용 — 향후 Go REST fallback hook 준비)
            kwargs: Celery envelope kwargs (file_type/file_path/user_id 등)

        Returns:
            DocumentMeta 인스턴스. 누락 필드는 다음 폴백:
              - file_type: 'unknown'
              - file_path: ''
              - user_id: self._default_user_id
        """
        _ = document_id  # 미래 확장: Go REST 호출 시 사용
        return DocumentMeta(
            file_type=str(kwargs.get("file_type") or "unknown"),
            file_path=str(kwargs.get("file_path") or ""),
            user_id=str(kwargs.get("user_id") or self._default_user_id),
        )
