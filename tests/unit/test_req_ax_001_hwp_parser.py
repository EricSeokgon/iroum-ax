"""AC-001-1, AC-001-2: HWP 파서 — 정상 파싱 및 OLE 손상 폴백 테스트

REQ-AX-001-E1: HWP 파일 파싱 → ParsedDocument (텍스트 + 표 + 메타데이터) 반환
REQ-AX-001-U1: OLE 손상 시 VLM OCR 자동 폴백 (status=ocr_fallback)
REQ-AX-001-U2: HWP + VLM 모두 실패 시 status=parse_failed, 사용자 개입 없음

# @MX:TODO: [AUTO] AC-001-1, AC-001-2 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-E1 / REQ-AX-001-U1 / REQ-AX-001-U2
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest


# =============================================================================
# 픽스처
# =============================================================================


@pytest.fixture()
def sample_hwp_path(tmp_path: pytest.TempPathFactory) -> str:
    """합성 HWP 파일 경로 (RED phase: 파일 내용은 mock으로 대체)"""
    hwp_file = tmp_path / "sample_report.hwp"
    # RED phase: 최소 바이너리 — GREEN phase에서 실제 HWP 픽스처로 교체
    hwp_file.write_bytes(b"\xd0\xcf\x11\xe0" + b"\x00" * 508)  # OLE 시그니처 + 패딩
    return str(hwp_file)


@pytest.fixture()
def corrupted_hwp_path(tmp_path: pytest.TempPathFactory) -> str:
    """손상된 OLE 헤더를 가진 HWP 파일 경로"""
    hwp_file = tmp_path / "corrupted_ole.hwp"
    # 의도적으로 손상된 헤더 (OLE 시그니처 깨짐)
    hwp_file.write_bytes(b"\xff\xfe\xfd\xfc" + b"\x00" * 508)
    return str(hwp_file)


@pytest.fixture()
def mock_vlm_processor() -> MagicMock:
    """VLMProcessor 모의 객체 — OCR 결과 반환"""
    mock = MagicMock()
    mock.ocr.return_value = "모의 OCR 텍스트: 안전보건 실적보고서"
    return mock


# =============================================================================
# AC-001-1: 정상 HWP 파싱 — ParsedDocument 반환
# =============================================================================


class TestHWPParserHappyPath:
    """AC-001-1: 정상 HWP 파싱 시나리오"""

    def test_hwp_parser_parse_should_return_parsed_document_type(
        self, sample_hwp_path: str
    ) -> None:
        """HWPParser.parse()는 ParsedDocument 인스턴스를 반환해야 한다.

        Given: 유효한 합성 HWP 파일 경로
        When: HWPParser().parse(file_path) 호출
        Then: ParsedDocument 타입 객체 반환
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]
        from pkg.models.document import ParsedDocument  # type: ignore[import]

        parser = HWPParser()
        with patch.object(parser, "_load_hwp", return_value={"text": "안전보건 실적보고서\n안전교육 이수율 100%", "tables": [], "metadata": {"author": "홍길동", "created_date": "2026-01-01", "sections": ["개요", "실적"]}}):
            result = parser.parse(sample_hwp_path)
        assert isinstance(result, ParsedDocument)

    def test_hwp_parser_parse_should_extract_text_field(
        self, sample_hwp_path: str
    ) -> None:
        """파싱 결과 ParsedDocument.text 필드는 비어 있지 않아야 한다.

        Given: 텍스트가 포함된 HWP 파일
        When: HWPParser().parse() 호출
        Then: result.text가 str 타입이고 len > 0
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser()
        with patch.object(parser, "_load_hwp", return_value={"text": "안전보건 실적보고서 내용", "tables": [], "metadata": {"author": "홍길동", "created_date": "2026-01-01", "sections": []}}):
            result = parser.parse(sample_hwp_path)
        assert isinstance(result.text, str)
        assert len(result.text) > 0

    def test_hwp_parser_parse_should_include_metadata_author_and_date(
        self, sample_hwp_path: str
    ) -> None:
        """파싱 결과 metadata에 author, created_date 필드가 포함되어야 한다.

        Given: 메타데이터가 있는 HWP 파일
        When: HWPParser().parse() 호출
        Then: result.metadata.author != None and result.metadata.created_date != None
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser()
        with patch.object(parser, "_load_hwp", return_value={"text": "내용", "tables": [], "metadata": {"author": "홍길동", "created_date": "2026-01-01", "sections": ["제1장"]}}):
            result = parser.parse(sample_hwp_path)
        assert result.metadata is not None
        assert result.metadata.author is not None
        assert result.metadata.created_date is not None

    def test_hwp_parser_parse_should_return_status_ok_on_success(
        self, sample_hwp_path: str
    ) -> None:
        """정상 파싱 시 ParsedDocument.status 는 'ok' 이어야 한다.

        Given: 정상 HWP 파일
        When: HWPParser().parse() 호출
        Then: result.status == 'ok'
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser()
        with patch.object(parser, "_load_hwp", return_value={"text": "내용", "tables": [], "metadata": {"author": "홍길동", "created_date": "2026-01-01", "sections": []}}):
            result = parser.parse(sample_hwp_path)
        assert result.status == "ok"

    def test_hwp_parser_parse_should_return_tables_as_list(
        self, sample_hwp_path: str
    ) -> None:
        """파싱 결과 tables 필드는 리스트 타입이어야 한다.

        Given: 표가 포함된 HWP 파일
        When: HWPParser().parse() 호출
        Then: result.tables는 list 타입
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser()
        with patch.object(parser, "_load_hwp", return_value={"text": "내용", "tables": [{"rows": [["헤더1", "헤더2"], ["값1", "값2"]], "rotation": 0}], "metadata": {"author": "홍길동", "created_date": "2026-01-01", "sections": []}}):
            result = parser.parse(sample_hwp_path)
        assert isinstance(result.tables, list)


# =============================================================================
# AC-001-2: OLE 손상 → VLM OCR 폴백
# =============================================================================


class TestHWPParserOLECorruptionFallback:
    """AC-001-2: HWP OLE 구조 손상 시 자동 VLM OCR 폴백"""

    def test_corrupted_hwp_should_trigger_vlm_ocr_fallback(
        self, corrupted_hwp_path: str, mock_vlm_processor: MagicMock
    ) -> None:
        """OLE 손상 HWP 파싱 시 VLMProcessor.ocr()가 자동 호출되어야 한다.

        Given: OLE 헤더가 손상된 HWP 파일
        When: HWPParser().parse() 호출 (OLE 파싱 실패 시뮬레이션)
        Then: VLMProcessor.ocr() 자동 호출 (폴백 경로 활성화)
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser(vlm_processor=mock_vlm_processor)
        # _load_hwp가 OLE 오류를 발생시키도록 패치
        with patch.object(parser, "_load_hwp", side_effect=Exception("OLECompoundError: invalid OLE header")):
            result = parser.parse(corrupted_hwp_path)
        mock_vlm_processor.ocr.assert_called()
        assert result is not None

    def test_corrupted_hwp_fallback_should_set_status_ocr_fallback(
        self, corrupted_hwp_path: str, mock_vlm_processor: MagicMock
    ) -> None:
        """OLE 손상 폴백 시 ParsedDocument.status는 'ocr_fallback' 이어야 한다.

        Given: OLE 손상 HWP + VLM OCR 성공
        When: HWPParser().parse() 호출 → OCR 폴백 활성화
        Then: result.status == 'ocr_fallback'
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser(vlm_processor=mock_vlm_processor)
        with patch.object(parser, "_load_hwp", side_effect=Exception("OLECompoundError")):
            result = parser.parse(corrupted_hwp_path)
        assert result.status == "ocr_fallback"

    def test_corrupted_hwp_fallback_requires_no_user_intervention(
        self, corrupted_hwp_path: str, mock_vlm_processor: MagicMock
    ) -> None:
        """OLE 손상 폴백은 사용자 개입 없이 자동으로 처리되어야 한다.

        Given: OLE 손상 HWP
        When: HWPParser().parse() 호출 (예외 발생 후 자동 폴백)
        Then: 예외가 호출자에게 전파되지 않음 (자동 처리)
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        parser = HWPParser(vlm_processor=mock_vlm_processor)
        with patch.object(parser, "_load_hwp", side_effect=Exception("OLECompoundError")):
            # 예외가 상위로 전파되면 안 됨
            try:
                result = parser.parse(corrupted_hwp_path)
                assert result is not None
            except Exception as exc:
                pytest.fail(f"사용자 개입이 필요한 예외가 전파됨: {exc}")

    def test_both_hwp_and_vlm_fail_should_return_parse_failed_status(
        self, corrupted_hwp_path: str
    ) -> None:
        """HWP + VLM 모두 실패 시 status='parse_failed' 를 반환해야 한다.

        Given: OLE 손상 HWP + VLM OCR도 실패
        When: HWPParser().parse() 호출
        Then: result.status == 'parse_failed' (HTTP 422에 해당)
        """
        from pipelines.ingestion.hwp_parser import HWPParser  # type: ignore[import]

        failing_vlm = MagicMock()
        failing_vlm.ocr.side_effect = Exception("VLM OCR 실패: GPU 메모리 부족")

        parser = HWPParser(vlm_processor=failing_vlm)
        with patch.object(parser, "_load_hwp", side_effect=Exception("OLECompoundError")):
            result = parser.parse(corrupted_hwp_path)
        assert result.status == "parse_failed"
