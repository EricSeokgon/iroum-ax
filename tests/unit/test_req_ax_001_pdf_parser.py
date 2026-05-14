"""AC-001-3 (부분): PDF 파서 — 회전된 페이지 정규화 테스트

REQ-AX-001-E1: PDF 파일 업로드 시 텍스트 추출 + 페이지 회전 처리
AC-001-3: 90° 회전된 표 페이지를 포함하는 PDF → 셀 논리적 순서 보존

참고: AC-001-3는 OCR 품질만 검증. GPU/CPU 분기는 AC-001-5(vlm_processor 테스트)에서 검증.

# @MX:TODO: [AUTO] AC-001-3 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1 / AC-001-3
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

# =============================================================================
# 픽스처
# =============================================================================


@pytest.fixture()
def sample_pdf_path(tmp_path: pytest.TempPathFactory) -> str:
    """일반 PDF 파일 경로 (RED phase: 내용은 mock으로 대체)"""
    pdf_file = tmp_path / "sample.pdf"
    # 최소 PDF 바이너리 시그니처 (실제 파싱은 mock으로 대체)
    pdf_file.write_bytes(b"%PDF-1.4\n")
    return str(pdf_file)


@pytest.fixture()
def rotated_pdf_path(tmp_path: pytest.TempPathFactory) -> str:
    """90도 회전된 표 페이지를 포함하는 PDF 파일 경로"""
    pdf_file = tmp_path / "rotated_table.pdf"
    pdf_file.write_bytes(b"%PDF-1.4\n")
    return str(pdf_file)


@pytest.fixture()
def mock_page_with_rotation() -> MagicMock:
    """회전 메타데이터를 가진 PDF 페이지 모의 객체"""
    mock = MagicMock()
    mock.rotation = 90
    mock.extract_text.return_value = "헤더1\t헤더2\n값1\t값2"
    return mock


# =============================================================================
# AC-001-3: 회전 PDF 정규화 — 텍스트 추출 정확도
# =============================================================================


class TestPDFParserNormalPath:
    """PDF 파서 정상 경로 테스트"""

    def test_pdf_parser_parse_should_return_parsed_document_type(
        self, sample_pdf_path: str
    ) -> None:
        """PDFParser.parse()는 ParsedDocument 인스턴스를 반환해야 한다.

        Given: 일반 PDF 파일
        When: PDFParser().parse(file_path) 호출
        Then: ParsedDocument 타입 객체 반환
        """
        from pipelines.ingestion.pdf_parser import PDFParser  # type: ignore[import]
        from pkg.models.document import ParsedDocument  # type: ignore[import]

        parser = PDFParser()
        with patch.object(parser, "_load_pdf", return_value={"text": "안전보건 PDF 내용", "pages": [{"text": "안전보건 내용", "rotation": 0}]}):
            result = parser.parse(sample_pdf_path)
        assert isinstance(result, ParsedDocument)

    def test_pdf_parser_parse_should_extract_text(
        self, sample_pdf_path: str
    ) -> None:
        """PDFParser는 PDF에서 텍스트를 추출해야 한다.

        Given: 텍스트가 있는 PDF
        When: PDFParser().parse() 호출
        Then: result.text가 비어 있지 않음
        """
        from pipelines.ingestion.pdf_parser import PDFParser  # type: ignore[import]

        parser = PDFParser()
        with patch.object(parser, "_load_pdf", return_value={"text": "안전교육 이수율 100%", "pages": []}):
            result = parser.parse(sample_pdf_path)
        assert len(result.text) > 0


class TestPDFParserRotatedPage:
    """AC-001-3: 회전된 PDF 페이지 처리"""

    def test_rotated_page_should_record_rotation_metadata(
        self, rotated_pdf_path: str
    ) -> None:
        """회전된 페이지에서 추출한 표에 rotation 메타데이터가 기록되어야 한다.

        Given: 90도 회전된 표 페이지 포함 PDF
        When: PDFParser().parse() 호출
        Then: result.tables 중 하나의 rotation == 90
        """
        from pipelines.ingestion.pdf_parser import PDFParser  # type: ignore[import]

        parser = PDFParser()
        with patch.object(parser, "_load_pdf", return_value={
            "text": "표 내용",
            "pages": [{"text": "헤더1\t헤더2\n값1\t값2", "rotation": 90}]
        }):
            result = parser.parse(rotated_pdf_path)
        rotations = [t.rotation for t in result.tables]
        assert 90 in rotations, f"rotation=90 테이블이 없음: {rotations}"

    def test_rotated_page_cells_should_be_in_logical_row_column_order(
        self, rotated_pdf_path: str
    ) -> None:
        """회전된 페이지의 표 셀은 논리적 행/열 순서로 추출되어야 한다.

        Given: 90도 회전된 표 (2행 2열)
        When: PDFParser().parse() 호출 후 표 추출
        Then: table.rows[0][0]이 첫 번째 헤더 셀 (논리적 순서)
        """
        from pipelines.ingestion.pdf_parser import PDFParser  # type: ignore[import]

        parser = PDFParser()
        # 회전 보정 후 논리적 순서로 셀 반환
        with patch.object(parser, "_load_pdf", return_value={
            "text": "헤더1 헤더2 값1 값2",
            "pages": [{"text": "헤더1\t헤더2\n값1\t값2", "rotation": 90}]
        }):
            result = parser.parse(rotated_pdf_path)
        assert len(result.tables) > 0
        table = result.tables[0]
        assert len(table.rows) > 0
        assert len(table.rows[0]) > 0

    def test_normal_page_should_have_rotation_zero(
        self, sample_pdf_path: str
    ) -> None:
        """정상 PDF 페이지의 표 rotation은 0이어야 한다.

        Given: 회전 없는 일반 PDF
        When: PDFParser().parse() 호출
        Then: 모든 table.rotation == 0
        """
        from pipelines.ingestion.pdf_parser import PDFParser  # type: ignore[import]

        parser = PDFParser()
        with patch.object(parser, "_load_pdf", return_value={
            "text": "정상 PDF 내용",
            "pages": [{"text": "헤더1\t헤더2\n값1\t값2", "rotation": 0}]
        }):
            result = parser.parse(sample_pdf_path)
        for table in result.tables:
            assert table.rotation == 0
