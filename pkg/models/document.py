"""공유 문서 데이터 모델 스켈레톤 (REQ-AX-001)"""
from __future__ import annotations

from enum import Enum
from uuid import UUID

from pydantic import BaseModel, Field


class FileType(str, Enum):
    """문서 파일 형식"""

    HWP = "HWP"
    PDF = "PDF"
    IMAGE = "IMAGE"


class DocumentModel(BaseModel):
    """파싱된 문서 공유 모델 — Sprint 2에서 필드 확정"""

    id: UUID = Field(description="문서 고유 ID")
    filename: str = Field(description="원본 파일명")
    file_type: FileType = Field(description="파일 형식")
    language: str = Field(default="ko", description="언어 코드")
    # TODO(Sprint 2): parsed_text, parse_quality_flag, status 필드 추가 (REQ-AX-001)
