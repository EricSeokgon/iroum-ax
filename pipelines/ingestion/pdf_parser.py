"""PDF 문서 파서 (REQ-AX-001)

회전 페이지를 포함하는 PDF 텍스트 추출.
_load_pdf() 메서드를 통해 실제 파싱을 수행하므로
단위 테스트에서 patch.object로 격리 가능.

# @MX:ANCHOR: [AUTO] PDFParser.parse — PDF 파싱 파이프라인 진입점
# @MX:REASON: IngestionPipeline, API 레이어, 통합 테스트에서 호출 (fan_in >= 3)
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1 / AC-001-3
"""
from __future__ import annotations

from typing import Any

from pkg.models.document import DocumentMetadata, ParsedDocument, Table


class PDFParser:
    """PDF 파일 파서.

    회전된 페이지가 포함된 PDF에서 텍스트와 표를 추출하며
    Table.rotation 메타데이터에 회전 각도를 기록한다.
    """

    def __init__(self) -> None:
        pass

    def parse(self, file_path: str) -> ParsedDocument:
        """PDF 파일을 파싱하여 ParsedDocument를 반환한다.

        _load_pdf()가 반환한 pages 목록을 순회하며
        회전 정보를 Table 모델에 기록한다.
        """
        raw = self._load_pdf(file_path)

        tables: list[Table] = []
        for page in raw.get("pages", []):
            rotation: int = page.get("rotation", 0)
            page_text: str = page.get("text", "")
            # 탭/개행 구분으로 셀 파싱 (최소 구현)
            rows = self._text_to_rows(page_text)
            if rows:
                tables.append(Table(rows=rows, rotation=rotation))

        return ParsedDocument(
            text=raw.get("text", ""),
            tables=tables,
            metadata=DocumentMetadata(),
            status="ok",
        )

    # ------------------------------------------------------------------
    # 내부 메서드 (단위 테스트에서 patch.object 대상)
    # ------------------------------------------------------------------

    def _load_pdf(self, file_path: str) -> dict[str, Any]:
        """PDF 파일을 파싱하여 원시 딕셔너리 반환.

        단위 테스트에서는 patch.object로 대체됨.
        """
        # pypdf 지연 임포트 — 설치되지 않은 환경 대응
        try:
            import pypdf  # type: ignore[import]
        except ImportError as exc:
            raise RuntimeError(f"PDF 파싱 라이브러리를 찾을 수 없음: {exc}") from exc

        pages: list[dict[str, Any]] = []
        full_text_parts: list[str] = []

        try:
            reader = pypdf.PdfReader(file_path)
            for page in reader.pages:
                text = page.extract_text() or ""
                rotation = getattr(page, "rotation", 0) or 0
                pages.append({"text": text, "rotation": rotation})
                full_text_parts.append(text)
        except Exception as exc:
            raise RuntimeError(f"PDF 파싱 실패: {exc}") from exc

        return {"text": "\n".join(full_text_parts), "pages": pages}

    # ------------------------------------------------------------------
    # 도우미
    # ------------------------------------------------------------------

    @staticmethod
    def _text_to_rows(text: str) -> list[list[str]]:
        """탭/개행 기반 텍스트를 2차원 셀 배열로 변환."""
        rows: list[list[str]] = []
        for line in text.strip().splitlines():
            cells = [cell.strip() for cell in line.split("\t")]
            if any(cells):
                rows.append(cells)
        return rows
