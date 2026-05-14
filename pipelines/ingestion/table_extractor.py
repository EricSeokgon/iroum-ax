"""표 추출기 (REQ-AX-001)

이미지에서 표 셀 구조를 감지하고 Table 모델로 변환.
_detect_cells(), _detect_rotation() 메서드를 통해
단위 테스트에서 patch.object로 격리 가능.

# @MX:ANCHOR: [AUTO] TableExtractor.extract — 표 추출 파이프라인 진입점
# @MX:REASON: PDFParser, VLMProcessor 폴백, API 레이어에서 호출 (fan_in >= 3)
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1 / AC-001-3
"""
from __future__ import annotations

from typing import Any

from pkg.models.document import Table


class TableExtractor:
    """이미지에서 표 구조를 감지하고 셀을 추출하는 추출기."""

    def __init__(self) -> None:
        pass

    def extract(self, image_path: str) -> list[Table]:
        """이미지에서 Table 목록을 추출한다.

        Args:
            image_path: 분석할 이미지 파일 경로.

        Returns:
            추출된 Table 인스턴스 목록. 표가 없으면 빈 리스트.
        """
        cells = self._detect_cells(image_path)
        if not cells:
            return []

        rotation = self._detect_rotation(image_path)
        rows = self._cells_to_rows(cells)

        return [Table(rows=rows, rotation=rotation)]

    # ------------------------------------------------------------------
    # 내부 메서드 (단위 테스트에서 patch.object 대상)
    # ------------------------------------------------------------------

    def _detect_cells(self, image_path: str) -> list[dict[str, Any]]:
        """이미지에서 셀 목록을 감지한다.

        단위 테스트에서는 patch.object로 대체됨.
        각 셀 딕셔너리: {'row': int, 'col': int, 'text': str}

        Returns:
            셀 딕셔너리 목록. 표가 없으면 빈 리스트.
        """
        # 실제 구현은 REFACTOR 페이즈에서 CV 라이브러리 연동
        return []

    def _detect_rotation(self, image_path: str) -> int:
        """이미지의 회전 각도를 감지한다.

        단위 테스트에서는 patch.object로 대체됨.

        Returns:
            회전 각도 (0, 90, 180, 270 중 하나).
        """
        # 실제 구현은 REFACTOR 페이즈에서 exif/CV 분석 연동
        return 0

    # ------------------------------------------------------------------
    # 도우미
    # ------------------------------------------------------------------

    @staticmethod
    def _cells_to_rows(cells: list[dict[str, Any]]) -> list[list[str]]:
        """셀 목록을 행/열 2차원 배열로 변환.

        row, col 인덱스를 기준으로 정렬하여 논리적 순서 보장.
        """
        if not cells:
            return []

        # 행/열 최대 인덱스 계산
        max_row = max(c.get("row", 0) for c in cells)
        max_col = max(c.get("col", 0) for c in cells)

        # 2차원 배열 초기화
        grid: list[list[str]] = [
            ["" for _ in range(max_col + 1)] for _ in range(max_row + 1)
        ]

        for cell in cells:
            r = cell.get("row", 0)
            c = cell.get("col", 0)
            grid[r][c] = cell.get("text", "")

        return grid
