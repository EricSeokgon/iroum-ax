"""SPEC-AX-INGEST-001 — DocumentMetadataClient 단위 테스트 (RED 단계)

REQ-INGEST-001 — Celery task kwargs에서 file_type/file_path/user_id 추출.
OPEN #4 결정: kwargs 우선, 누락 시 settings.default_user_id 및 'unknown' 폴백.
"""
from __future__ import annotations

from pipelines.ingestion.document_metadata import DocumentMeta, DocumentMetadataClient


class TestDocumentMetadataClient:
    """DocumentMetadataClient.fetch() — kwargs 우선 추출 + 폴백 검증"""

    def test_fetch_returns_kwargs_values_when_complete(self) -> None:
        """kwargs에 모든 필드 존재 → 그대로 반환"""
        client = DocumentMetadataClient(default_user_id="cli-anonymous")
        kwargs = {
            "file_type": "PDF",
            "file_path": "/uploads/doc.pdf",
            "user_id": "u-001",
            "workflow_id": "wf-xxx",  # 무관한 키는 무시
        }

        meta = client.fetch(document_id="doc-001", kwargs=kwargs)

        assert isinstance(meta, DocumentMeta)
        assert meta.file_type == "PDF"
        assert meta.file_path == "/uploads/doc.pdf"
        assert meta.user_id == "u-001"

    def test_fetch_uses_unknown_when_file_type_missing(self) -> None:
        """file_type 누락 → 'unknown' 폴백"""
        client = DocumentMetadataClient(default_user_id="cli-anonymous")
        kwargs = {
            "file_path": "/uploads/x.dat",
            "user_id": "u-002",
        }

        meta = client.fetch(document_id="doc-002", kwargs=kwargs)

        assert meta.file_type == "unknown"
        assert meta.file_path == "/uploads/x.dat"
        assert meta.user_id == "u-002"

    def test_fetch_uses_default_user_id_when_missing(self) -> None:
        """user_id 누락 → default_user_id 폴백"""
        client = DocumentMetadataClient(default_user_id="cli-anonymous")
        kwargs = {
            "file_type": "HWP",
            "file_path": "/uploads/x.hwp",
        }

        meta = client.fetch(document_id="doc-003", kwargs=kwargs)

        assert meta.user_id == "cli-anonymous"
        assert meta.file_type == "HWP"

    def test_fetch_empty_kwargs_uses_all_defaults(self) -> None:
        """kwargs 빈 dict → 모두 폴백"""
        client = DocumentMetadataClient(default_user_id="cli-anonymous")

        meta = client.fetch(document_id="doc-004", kwargs={})

        assert meta.file_type == "unknown"
        assert meta.file_path == ""
        assert meta.user_id == "cli-anonymous"

    def test_fetch_with_pdf_uppercase_preserved(self) -> None:
        """file_type 대소문자 보존 — 정규화하지 않음"""
        client = DocumentMetadataClient(default_user_id="cli-anonymous")

        meta = client.fetch(
            document_id="doc-005",
            kwargs={"file_type": "PDF", "file_path": "/x.pdf"},
        )

        assert meta.file_type == "PDF"  # 대문자 그대로

    def test_documentmeta_is_dataclass_with_fields(self) -> None:
        """DocumentMeta 데이터클래스 구조 검증"""
        meta = DocumentMeta(
            file_type="PDF",
            file_path="/x.pdf",
            user_id="u-1",
        )

        assert meta.file_type == "PDF"
        assert meta.file_path == "/x.pdf"
        assert meta.user_id == "u-1"
