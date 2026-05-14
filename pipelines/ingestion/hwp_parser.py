"""HWP 문서 파서 (REQ-AX-001)

OLE 손상 시 VLMProcessor OCR 자동 폴백.
모든 실제 HWP 파싱은 _load_hwp() 메서드를 통해 수행되므로
단위 테스트에서 patch.object로 격리 가능.

# @MX:ANCHOR: [AUTO] HWPParser.parse — HWP 파싱 파이프라인 진입점
# @MX:REASON: IngestionPipeline, API 레이어, 통합 테스트에서 호출 (fan_in >= 3)
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1 / REQ-AX-001-U1 / REQ-AX-001-U2
"""
from __future__ import annotations

from typing import Any, Protocol, runtime_checkable

from pkg.models.document import DocumentMetadata, ParsedDocument, Table


@runtime_checkable
class _VLMProcessorProtocol(Protocol):
    """VLMProcessor 덕타입 인터페이스 — 순환 임포트 방지"""

    def ocr(self, image_path: str) -> str: ...


class HWPParser:
    """HWP 파일 파서.

    Args:
        vlm_processor: OLE 손상 시 OCR 폴백으로 사용할 VLMProcessor 인스턴스.
                       None이면 폴백 불가 → status='parse_failed'.
    """

    def __init__(self, vlm_processor: _VLMProcessorProtocol | None = None) -> None:
        # OLE 손상 폴백용 VLM 프로세서
        self._vlm = vlm_processor

    def parse(self, file_path: str) -> ParsedDocument:
        """HWP 파일을 파싱하여 ParsedDocument를 반환한다.

        정상 경우: _load_hwp()로 텍스트/표/메타데이터 추출 → status='ok'
        OLE 손상: VLMProcessor.ocr() 폴백 → status='ocr_fallback'
        양쪽 모두 실패: status='parse_failed' (예외 전파 없음)
        """
        try:
            raw = self._load_hwp(file_path)
            return self._build_document(raw, status="ok")
        except Exception:
            return self._ocr_fallback(file_path)

    # ------------------------------------------------------------------
    # 내부 메서드 (단위 테스트에서 patch.object 대상)
    # ------------------------------------------------------------------

    def _load_hwp(self, file_path: str) -> dict[str, Any]:
        """HWP 파일을 파싱하여 원시 딕셔너리 반환.

        실제 구현은 hwp-converter 라이브러리를 사용.
        단위 테스트에서는 patch.object로 대체됨.
        """
        # hwp-converter 지연 임포트 — 설치되지 않은 환경 대응
        try:
            import hwp  # type: ignore[import]
        except ImportError as exc:
            raise RuntimeError(f"hwp-converter 라이브러리를 찾을 수 없음: {exc}") from exc

        doc = hwp.open(file_path)
        return {
            "text": doc.text,
            "tables": [],
            "metadata": {
                "author": getattr(doc, "author", None),
                "created_date": getattr(doc, "created_date", None),
                "sections": [],
            },
        }

    def _ocr_fallback(self, file_path: str) -> ParsedDocument:
        """VLMProcessor OCR 폴백 경로.

        VLM도 실패하면 status='parse_failed' 반환 (예외 전파 없음).
        """
        if self._vlm is None:
            return ParsedDocument(text="", status="parse_failed")

        try:
            ocr_text: str = self._vlm.ocr(file_path)
            return ParsedDocument(text=ocr_text, status="ocr_fallback")
        except Exception:
            return ParsedDocument(text="", status="parse_failed")

    # ------------------------------------------------------------------
    # 도우미
    # ------------------------------------------------------------------

    @staticmethod
    def _build_document(raw: dict[str, Any], status: str) -> ParsedDocument:
        """원시 딕셔너리를 ParsedDocument로 변환."""
        meta_raw: dict[str, Any] = raw.get("metadata", {})
        metadata = DocumentMetadata(
            author=meta_raw.get("author"),
            created_date=meta_raw.get("created_date"),
            sections=meta_raw.get("sections", []),
        )

        tables: list[Table] = []
        for t in raw.get("tables", []):
            tables.append(Table(rows=t.get("rows", []), rotation=t.get("rotation", 0)))

        return ParsedDocument(
            text=raw.get("text", ""),
            tables=tables,
            metadata=metadata,
            status=status,
        )
