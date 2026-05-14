"""공유 문서 데이터 모델 (REQ-AX-001)

# @MX:ANCHOR: [AUTO] ParsedDocument — 문서 파싱 파이프라인 전반에서 사용되는 핵심 모델
# @MX:REASON: HWPParser, PDFParser, VLMProcessor, API 레이어 모두 이 모델을 반환/소비함
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1
"""
from __future__ import annotations

from enum import StrEnum
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class FileType(StrEnum):
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


class DocumentMetadata(BaseModel):
    """문서 메타데이터 — 작성자, 생성일, 섹션 구조"""

    model_config = ConfigDict(extra="allow")

    author: str | None = Field(default=None, description="문서 작성자")
    created_date: str | None = Field(default=None, description="문서 생성일 (YYYY-MM-DD)")
    sections: list[str] = Field(default_factory=list, description="섹션 제목 목록")


class Table(BaseModel):
    """표 구조 모델 — 행/열 셀 데이터 및 회전 메타데이터"""

    model_config = ConfigDict(extra="allow")

    rows: list[list[str]] = Field(default_factory=list, description="행/열 셀 2차원 배열")
    rotation: int = Field(default=0, description="페이지 회전 각도 (0, 90, 180, 270)")


class ParsedDocument(BaseModel):
    """파싱된 문서 결과 — HWP/PDF 파서 반환 타입

    # @MX:NOTE: [AUTO] status 값: 'ok' | 'ocr_fallback' | 'parse_failed'
    # @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1 / REQ-AX-001-U1 / REQ-AX-001-U2
    """

    model_config = ConfigDict(extra="allow")

    text: str = Field(default="", description="추출된 전체 텍스트")
    tables: list[Table] = Field(default_factory=list, description="추출된 표 목록")
    metadata: DocumentMetadata = Field(
        default_factory=DocumentMetadata, description="문서 메타데이터"
    )
    status: str = Field(default="ok", description="파싱 상태: ok | ocr_fallback | parse_failed")
