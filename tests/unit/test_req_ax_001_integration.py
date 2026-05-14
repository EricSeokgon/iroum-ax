"""AC-001-1~005 통합 시나리오: HWP → ParsedDocument 전체 파이프라인

전체 Document Ingestion 파이프라인 오케스트레이션:
  HWP 파일 → HWPParser → (선택적) VLMProcessor → TableExtractor → ParsedDocument

AC-001-1 연동: 정상 파이프라인 통합 검증
AC-001-2 연동: OLE 손상 → VLM 폴백 통합 검증
AC-001-4: OCR 동시 요청 큐잉 동작 검증 (Celery 큐 or HTTP 409)

# @MX:TODO: [AUTO] REQ-AX-001 통합 파이프라인 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-AX-001 / AC-001-1 / AC-001-2 / AC-001-4
"""
from __future__ import annotations

from unittest.mock import MagicMock, call, patch

import pytest


# =============================================================================
# 픽스처
# =============================================================================


@pytest.fixture()
def normal_hwp_path(tmp_path: pytest.TempPathFactory) -> str:
    """정상 HWP 파일 경로"""
    hwp_file = tmp_path / "normal.hwp"
    hwp_file.write_bytes(b"\xd0\xcf\x11\xe0" + b"\x00" * 508)
    return str(hwp_file)


@pytest.fixture()
def corrupted_hwp_path(tmp_path: pytest.TempPathFactory) -> str:
    """손상된 HWP 파일 경로"""
    hwp_file = tmp_path / "corrupted.hwp"
    hwp_file.write_bytes(b"\xff\xfe\xfd\xfc" + b"\x00" * 508)
    return str(hwp_file)


@pytest.fixture()
def mock_vlm() -> MagicMock:
    """통합 테스트용 VLM 모의 객체"""
    mock = MagicMock()
    mock.ocr.return_value = "통합 OCR 텍스트: 안전보건 실적보고서"
    return mock


# =============================================================================
# 통합 시나리오 1: 정상 파이프라인
# =============================================================================


class TestDocumentIngestionNormalPipeline:
    """AC-001-1 연동: 정상 HWP → ParsedDocument 전체 파이프라인"""

    def test_normal_hwp_pipeline_should_produce_parsed_document(
        self, normal_hwp_path: str, mock_vlm: MagicMock
    ) -> None:
        """정상 HWP 파일이 ParsedDocument로 변환되는 전체 파이프라인이 동작해야 한다.

        Given: 정상 HWP 파일 + 정상 HWPParser
        When: HWPParser.parse() 전체 파이프라인 실행
        Then: ParsedDocument 반환 (text, tables, metadata, status='ok')
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]
        from pkg.models.document import ParsedDocument  # type: ignore[import]

        parser = HWPParser(vlm_processor=mock_vlm)
        with patch.object(parser, "_load_hwp", return_value={
            "text": "안전보건 실적보고서\n안전교육 이수율 100%\n안전사고 건수 0건",
            "tables": [
                {"rows": [["지표", "실적", "목표"], ["안전교육 이수율", "100%", "95%"]], "rotation": 0}
            ],
            "metadata": {"author": "KEPCO E&C", "created_date": "2026-01-15", "sections": ["개요", "실적", "향후계획"]},
        }):
            result = parser.parse(normal_hwp_path)

        assert isinstance(result, ParsedDocument)
        assert result.status == "ok"
        assert len(result.text) > 0
        assert isinstance(result.tables, list)

    def test_normal_hwp_pipeline_should_extract_metadata(
        self, normal_hwp_path: str, mock_vlm: MagicMock
    ) -> None:
        """정상 파이프라인에서 문서 메타데이터가 정확히 추출되어야 한다.

        Given: 메타데이터가 있는 HWP 파일
        When: parse() 호출
        Then: metadata.author, metadata.created_date, metadata.sections 모두 존재
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser(vlm_processor=mock_vlm)
        with patch.object(parser, "_load_hwp", return_value={
            "text": "내용",
            "tables": [],
            "metadata": {"author": "KEPCO E&C", "created_date": "2026-01-15", "sections": ["개요"]},
        }):
            result = parser.parse(normal_hwp_path)

        assert result.metadata.author == "KEPCO E&C"
        assert result.metadata.created_date is not None
        assert len(result.metadata.sections) > 0


# =============================================================================
# 통합 시나리오 2: OLE 손상 폴백 파이프라인
# =============================================================================


class TestDocumentIngestionFallbackPipeline:
    """AC-001-2 연동: 손상 HWP → VLM OCR 폴백 전체 흐름"""

    def test_fallback_pipeline_produces_ocr_fallback_status(
        self, corrupted_hwp_path: str, mock_vlm: MagicMock
    ) -> None:
        """손상 HWP → VLM 폴백 파이프라인이 status='ocr_fallback' ParsedDocument를 생성해야 한다.

        Given: OLE 손상 HWP + 정상 VLM
        When: HWPParser.parse() 전체 파이프라인 (OLE 실패 → VLM 폴백)
        Then: result.status == 'ocr_fallback' AND result.text is not None
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser(vlm_processor=mock_vlm)
        with patch.object(parser, "_load_hwp", side_effect=Exception("OLECompoundError")):
            result = parser.parse(corrupted_hwp_path)

        assert result.status == "ocr_fallback"
        assert result.text is not None


# =============================================================================
# AC-001-4: OCR 동시 요청 큐잉
# =============================================================================


class TestOCRConcurrencyQueuing:
    """AC-001-4: 동일 문서 OCR 동시 요청 시 GPU memory race 방지"""

    def test_concurrent_ocr_request_should_raise_concurrency_error(
        self,
    ) -> None:
        """진행 중인 OCR 작업과 동일한 문서에 대한 재요청 시 에러 또는 큐잉이 발생해야 한다.

        Given: document_id='doc-101'에 대한 OCR 작업이 진행 중
        When: 동일 document_id로 OCR 재요청
        Then: OCRConcurrencyError 발생 (HTTP 409에 해당) 또는 큐잉 응답
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]
        from pkg.errors.custom_errors import OCRConcurrencyError  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        # 진행 중인 OCR 상태 시뮬레이션
        processor._active_ocr_doc_ids = {"doc-101"}  # type: ignore[attr-defined]

        with pytest.raises(OCRConcurrencyError):
            processor.ocr_with_lock("doc-101", "page_001.png")

    def test_concurrent_ocr_request_should_not_run_simultaneous_inference(
        self,
    ) -> None:
        """동일 페이지에 대해 동시 추론이 실행되지 않아야 한다.

        Given: doc-101 OCR 진행 중
        When: 동일 doc-101 재요청
        Then: _run_inference가 두 번 동시 호출되지 않음 (동시 GPU 사용 방지)
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]
        from pkg.errors.custom_errors import OCRConcurrencyError  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        processor._active_ocr_doc_ids = {"doc-101"}  # type: ignore[attr-defined]

        inference_call_count = 0

        def count_inference(*args: object, **kwargs: object) -> str:
            nonlocal inference_call_count
            inference_call_count += 1
            return "OCR 결과"

        with patch.object(processor, "_run_inference", side_effect=count_inference):
            try:
                processor.ocr_with_lock("doc-101", "page_001.png")
            except OCRConcurrencyError:
                pass  # 예상된 동작

        # 동시 요청 시 추론이 실행되지 않아야 함
        assert inference_call_count == 0, "동시 OCR 요청 시 추론이 실행되면 안 됨"
