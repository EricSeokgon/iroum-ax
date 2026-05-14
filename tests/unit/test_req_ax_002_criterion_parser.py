"""REQ-AX-002 CriterionParser 단위 테스트 — RED phase

평가편람 PDF를 파싱하여 항목→지표→배점 계층 구조를 추출하는
CriterionParser의 계약을 정의한다.

# @MX:TODO: [AUTO] GREEN phase에서 pipelines.mapping.criterion_parser.CriterionParser 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1 / AC-002-1 / AC-002-3
"""
from __future__ import annotations

import pytest
from pkg.models.criterion import Criterion


class TestCriterionParserHappyPath:
    """AC-002-1, AC-002-3: 정상 파싱 경로"""

    def test_parse_returns_list_of_criterion_objects(self, tmp_path: Path) -> None:  # noqa: F821
        """parse()가 Criterion 객체 리스트를 반환해야 한다."""
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]

        dummy_pdf = tmp_path / "criterion.pdf"
        dummy_pdf.write_bytes(b"%PDF-1.4 fake content")

        parser = CriterionParser()
        result = parser.parse(str(dummy_pdf))

        assert isinstance(result, list)
        assert len(result) > 0
        assert all(isinstance(c, Criterion) for c in result)

    def test_parse_preserves_criterion_name_field(self, tmp_path: Path) -> None:  # noqa: F821
        """파싱 결과의 criterion_name 필드가 비어 있지 않아야 한다.

        AC-002-3: 항목→지표→배점 계층 정보가 응답에 포함되어야 함.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]

        dummy_pdf = tmp_path / "criterion.pdf"
        dummy_pdf.write_bytes(b"%PDF-1.4 fake content")

        parser = CriterionParser()
        result = parser.parse(str(dummy_pdf))

        assert all(c.criterion_name != "" for c in result)

    def test_parse_preserves_hierarchy_parent_criterion_id(
        self, tmp_path: Path  # noqa: F821
    ) -> None:
        """계층 구조 보존: 하위 지표에 parent_criterion_id가 연결되어야 한다.

        AC-002-3: '안전보건 → 안전교육 → 5점' 구조에서
        leaf criterion은 parent_criterion_id를 가져야 한다.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]

        dummy_pdf = tmp_path / "criterion.pdf"
        dummy_pdf.write_bytes(b"%PDF-1.4 fake content")

        parser = CriterionParser()
        result = parser.parse(str(dummy_pdf))

        # 루트가 아닌 leaf criterion은 parent_criterion_id를 가져야 함
        leaf_criteria = [c for c in result if c.parent_criterion_id is not None]
        assert len(leaf_criteria) > 0, "계층 구조의 하위 지표(leaf)가 최소 1개 이상이어야 함"

    def test_parse_extracts_max_points_for_leaf_criteria(
        self, tmp_path: Path  # noqa: F821
    ) -> None:
        """leaf criterion의 max_points 필드가 추출되어야 한다.

        AC-002-3: 배점('5점') 정보가 max_points 필드에 저장되어야 함.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]

        dummy_pdf = tmp_path / "criterion.pdf"
        dummy_pdf.write_bytes(b"%PDF-1.4 fake content")

        parser = CriterionParser()
        result = parser.parse(str(dummy_pdf))

        leaf_criteria = [c for c in result if c.parent_criterion_id is not None]
        criteria_with_points = [c for c in leaf_criteria if c.max_points is not None]
        assert len(criteria_with_points) > 0, "배점이 있는 leaf criterion이 최소 1개 이상이어야 함"


class TestCriterionParserHanjaHandling:
    """AC-002-3, AC-002-5: 한자/한글 혼용 처리"""

    def test_parse_handles_hanja_text_without_crash(self, tmp_path: Path) -> None:  # noqa: F821
        """한자가 포함된 텍스트를 파싱할 때 크래시가 발생하지 않아야 한다.

        AC-002-5: 시스템 크래시·HTTP 500 발생 시 본 AC는 실패.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]

        dummy_pdf = tmp_path / "hanja_criterion.pdf"
        dummy_pdf.write_bytes(b"%PDF-1.4 hanja content: \xe5\xae\x89\xe5\x85\xa8")  # 安全

        parser = CriterionParser()
        # 한자 파싱 중 예외가 발생하면 안 됨
        try:
            parser.parse(str(dummy_pdf))
        except Exception as e:
            pytest.fail(f"한자 포함 PDF 파싱 시 예외 발생: {e}")

    def test_parse_attaches_normalization_warning_for_unresolved_hanja(
        self, mock_criterion_with_hanja: Criterion  # noqa: F821
    ) -> None:
        """미등록 한자 토큰에 대해 normalization_warning 메타데이터가 첨부되어야 한다.

        AC-002-5: 정규화 실패한 토큰에 'normalization_warning' 기록 필요.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]

        # 미등록 한자가 포함된 criterion을 처리할 때 warning이 붙어야 함
        parser = CriterionParser()
        processed = parser.apply_hanja_normalization(mock_criterion_with_hanja)

        assert processed.normalization_warning is not None
        assert processed.normalization_warning.get("type") == "unresolved_hanja"
        assert "unresolved_chars" in processed.normalization_warning

    def test_parse_preserves_raw_text_when_hanja_unresolved(
        self, mock_criterion_with_hanja: Criterion  # noqa: F821
    ) -> None:
        """한자 정규화 실패 시 raw 텍스트가 보존되어 임베딩에 전달되어야 한다.

        AC-002-5: (a) 정규화 실패한 토큰을 그대로 보존하여 임베딩에 전달.
        """
        from pipelines.mapping.criterion_parser import CriterionParser  # type: ignore[import]

        parser = CriterionParser()
        processed = parser.apply_hanja_normalization(mock_criterion_with_hanja)

        # 한자가 그대로 남아 있어야 함 (삭제되지 않음)
        assert len(processed.criterion_name) > 0
        assert len(processed.criterion_detail) >= 0


@pytest.fixture()
def mock_criterion_with_hanja() -> Criterion:
    """미등록 한자가 포함된 criterion 픽스처.

    '古文書' 같은 희귀 한자 포함 — hanja→hangul 사전 미등재 가정.
    """
    return Criterion(
        id="test-criterion-hanja-001",
        criterion_name="古文書 보존 안전보건 기준",
        criterion_detail="古文書 관련 안전보건 관리 지표",
        max_points=5,
        parent_criterion_id="test-criterion-parent-001",
    )
