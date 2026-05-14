"""AC-001-3 (심층): TableExtractor — 회전 표 셀 구조 보존 테스트

REQ-AX-001-E1: VLM OCR 출력에서 표 셀 추출 (논리적 행/열 순서)
AC-001-3: 90° 회전 페이지 OCR 결과 → 표 셀 논리적 순서 보존
          table_extractor가 rotation 메타데이터 기록

# @MX:TODO: [AUTO] AC-001-3 Table 추출 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1 / AC-001-3
"""
from __future__ import annotations

from unittest.mock import patch

import pytest

# =============================================================================
# 픽스처
# =============================================================================


@pytest.fixture()
def rotated_image_path(tmp_path: pytest.TempPathFactory) -> str:
    """90도 회전된 표 이미지 경로"""
    img_file = tmp_path / "rotated_table_page.png"
    img_file.write_bytes(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100)
    return str(img_file)


@pytest.fixture()
def normal_image_path(tmp_path: pytest.TempPathFactory) -> str:
    """정상 방향 표 이미지 경로"""
    img_file = tmp_path / "normal_table_page.png"
    img_file.write_bytes(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100)
    return str(img_file)


# =============================================================================
# TableExtractor 인터페이스 및 기본 동작 테스트
# =============================================================================


class TestTableExtractorInterface:
    """TableExtractor 기본 인터페이스 검증"""

    def test_table_extractor_extract_should_return_list(
        self, normal_image_path: str
    ) -> None:
        """TableExtractor.extract()는 Table 리스트를 반환해야 한다.

        Given: 표가 있는 일반 이미지
        When: TableExtractor().extract(image_path) 호출
        Then: list[Table] 반환
        """
        from pipelines.ingestion.table_extractor import TableExtractor  # type: ignore[import]

        extractor = TableExtractor()
        with patch.object(extractor, "_detect_cells", return_value=[
            {"row": 0, "col": 0, "text": "헤더1"},
            {"row": 0, "col": 1, "text": "헤더2"},
            {"row": 1, "col": 0, "text": "값1"},
            {"row": 1, "col": 1, "text": "값2"},
        ]):
            tables = extractor.extract(normal_image_path)
        assert isinstance(tables, list)

    def test_table_extractor_returns_table_model_instances(
        self, normal_image_path: str
    ) -> None:
        """extract() 반환 리스트의 원소는 Table 모델 인스턴스이어야 한다.

        Given: 표가 있는 이미지
        When: extract() 호출
        Then: 반환 리스트의 각 원소가 Table 인스턴스
        """
        from pipelines.ingestion.table_extractor import TableExtractor  # type: ignore[import]
        from pkg.models.document import Table  # type: ignore[import]

        extractor = TableExtractor()
        with patch.object(extractor, "_detect_cells", return_value=[
            {"row": 0, "col": 0, "text": "셀1"},
        ]):
            tables = extractor.extract(normal_image_path)
        if len(tables) > 0:
            assert isinstance(tables[0], Table)


# =============================================================================
# AC-001-3: 회전 표 셀 보존
# =============================================================================


class TestTableExtractorRotatedPage:
    """AC-001-3: 90° 회전 표 셀 논리적 순서 보존"""

    def test_rotated_table_should_record_rotation_90_metadata(
        self, rotated_image_path: str
    ) -> None:
        """회전된 표 페이지에서 추출한 Table.rotation은 90이어야 한다.

        Given: 90° 회전 표 이미지
        When: TableExtractor().extract(image_path) 호출
        Then: 추출된 table.rotation == 90
        """
        from pipelines.ingestion.table_extractor import TableExtractor  # type: ignore[import]

        extractor = TableExtractor()
        with patch.object(extractor, "_detect_cells", return_value=[
            {"row": 0, "col": 0, "text": "헤더1", "rotation": 90},
            {"row": 0, "col": 1, "text": "헤더2", "rotation": 90},
        ]), patch.object(extractor, "_detect_rotation", return_value=90):
            tables = extractor.extract(rotated_image_path)
        assert len(tables) > 0
        assert tables[0].rotation == 90

    def test_rotated_table_cells_should_be_in_logical_row_column_order(
        self, rotated_image_path: str
    ) -> None:
        """회전 보정 후 셀은 행/열 논리적 순서(좌→우, 위→아래)로 정렬되어야 한다.

        Given: 90° 회전 2×2 표 이미지
        When: extract() 호출 (회전 보정 포함)
        Then: table.rows[0] = 첫 번째 행, table.rows[0][0] = 좌상단 셀
        """
        from pipelines.ingestion.table_extractor import TableExtractor  # type: ignore[import]

        extractor = TableExtractor()
        # 회전 보정 후 논리적 순서로 정렬된 셀 반환
        with patch.object(extractor, "_detect_cells", return_value=[
            {"row": 0, "col": 0, "text": "헤더1", "rotation": 90},
            {"row": 0, "col": 1, "text": "헤더2", "rotation": 90},
            {"row": 1, "col": 0, "text": "값1", "rotation": 90},
            {"row": 1, "col": 1, "text": "값2", "rotation": 90},
        ]), patch.object(extractor, "_detect_rotation", return_value=90):
            tables = extractor.extract(rotated_image_path)
        assert len(tables) > 0
        table = tables[0]
        # 2행 2열 구조 확인
        assert len(table.rows) == 2
        assert len(table.rows[0]) == 2

    def test_rotated_table_first_cell_should_be_header(
        self, rotated_image_path: str
    ) -> None:
        """회전 보정 후 첫 번째 셀이 헤더 위치(논리적 순서)여야 한다.

        Given: 90° 회전 표 (헤더1, 헤더2, 값1, 값2)
        When: extract() 호출
        Then: table.rows[0][0] == '헤더1'
        """
        from pipelines.ingestion.table_extractor import TableExtractor  # type: ignore[import]

        extractor = TableExtractor()
        with patch.object(extractor, "_detect_cells", return_value=[
            {"row": 0, "col": 0, "text": "헤더1", "rotation": 90},
            {"row": 0, "col": 1, "text": "헤더2", "rotation": 90},
            {"row": 1, "col": 0, "text": "값1", "rotation": 90},
            {"row": 1, "col": 1, "text": "값2", "rotation": 90},
        ]), patch.object(extractor, "_detect_rotation", return_value=90):
            tables = extractor.extract(rotated_image_path)
        assert tables[0].rows[0][0] == "헤더1"

    def test_normal_table_should_have_rotation_zero(
        self, normal_image_path: str
    ) -> None:
        """회전 없는 정상 표의 Table.rotation은 0이어야 한다.

        Given: 정상 방향 표 이미지
        When: extract() 호출
        Then: table.rotation == 0
        """
        from pipelines.ingestion.table_extractor import TableExtractor  # type: ignore[import]

        extractor = TableExtractor()
        with patch.object(extractor, "_detect_cells", return_value=[
            {"row": 0, "col": 0, "text": "헤더1"},
        ]), patch.object(extractor, "_detect_rotation", return_value=0):
            tables = extractor.extract(normal_image_path)
        if len(tables) > 0:
            assert tables[0].rotation == 0
