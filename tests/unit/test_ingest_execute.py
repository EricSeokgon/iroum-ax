"""SPEC-AX-INGEST-001 — _execute() 본체 단위 테스트 (RED 단계)

REQ-INGEST-001/002/003/004 — VLM OCR + RAG 임베딩 + Go 채점 트리거 파이프라인.
격리: VLMProcessor, EmbeddingService, VectorStore, ScoreTrigger, post_callback 모두 patch.

테스트 표적: pipelines.workers.ingestion_worker._execute(document_id, workflow_id, **kwargs)
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest
from pipelines.workers.ingestion_worker import _execute


def _make_post_callback_recorder() -> tuple[MagicMock, list[dict]]:
    """post_callback 호출을 기록하는 MagicMock과 캡처 리스트 반환."""
    captured: list[dict] = []

    def _record(*, base_url: str, workflow_id: str, status: str, result_json: dict, **_) -> None:
        captured.append(
            {
                "base_url": base_url,
                "workflow_id": workflow_id,
                "status": status,
                "result_json": result_json,
            }
        )

    return MagicMock(side_effect=_record), captured


@patch("pipelines.workers.ingestion_worker.post_callback")
@patch("pipelines.workers.ingestion_worker.ScoreTrigger")
@patch("pipelines.workers.ingestion_worker.VectorStore")
@patch("pipelines.workers.ingestion_worker.EmbeddingService")
@patch("pipelines.workers.ingestion_worker.VLMProcessor")
class TestExecuteGoldenPath:
    """AC-INGEST-001-1: PDF 골든 패스 시나리오"""

    def test_execute_golden_path_pdf_returns_completed(
        self,
        mock_vlm_cls: MagicMock,
        mock_emb_cls: MagicMock,
        mock_vs_cls: MagicMock,
        mock_st_cls: MagicMock,
        mock_callback: MagicMock,
    ) -> None:
        """VLM→Embed→VectorStore→ScoreTrigger 전체 성공 → status='completed'"""
        # Mock 동작 설정
        mock_vlm = MagicMock()
        mock_vlm.ocr.return_value = "추출된 평가편람 텍스트 " * 200  # ~4000 chars → 청크 분할 트리거
        mock_vlm.last_inference_meta = {"inference_backend": "transformers_cpu"}
        mock_vlm_cls.return_value = mock_vlm

        mock_emb = MagicMock()
        mock_emb.encode.return_value = [0.1] * 768
        mock_emb_cls.return_value = mock_emb

        mock_vs = MagicMock()
        mock_vs.upsert.return_value = None
        mock_vs_cls.return_value = mock_vs

        mock_st = MagicMock()
        mock_st.fire.return_value = True  # 채점 트리거 성공
        mock_st_cls.return_value = mock_st

        result = _execute(
            document_id="doc-pdf-001",
            workflow_id="wf-001",
            file_type="PDF",
            file_path="/uploads/test.pdf",
            user_id="u-001",
        )

        # result_json 스키마 검증 (REQ-INGEST-004)
        assert result["document_id"] == "doc-pdf-001"
        assert result["chunks"] > 0
        assert result["tokens"] > 0
        assert result["score_triggered"] is True
        assert result["ocr_backend"] == "transformers_cpu"
        assert result["pages_processed"] >= 1
        assert result["spec"] == "SPEC-AX-INGEST-001"

        # callback 'completed' status 검증
        callback_calls = mock_callback.call_args_list
        assert len(callback_calls) == 1
        kwargs = callback_calls[0].kwargs
        assert kwargs["status"] == "completed"
        assert kwargs["workflow_id"] == "wf-001"


@patch("pipelines.workers.ingestion_worker.post_callback")
@patch("pipelines.workers.ingestion_worker.ScoreTrigger")
@patch("pipelines.workers.ingestion_worker.VectorStore")
@patch("pipelines.workers.ingestion_worker.EmbeddingService")
@patch("pipelines.workers.ingestion_worker.VLMProcessor")
class TestExecuteVLMTimeout:
    """AC-INGEST-001-2: VLM 타임아웃 시나리오"""

    def test_execute_vlm_timeout_returns_failed(
        self,
        mock_vlm_cls: MagicMock,
        mock_emb_cls: MagicMock,
        mock_vs_cls: MagicMock,
        mock_st_cls: MagicMock,
        mock_callback: MagicMock,
    ) -> None:
        """VLM.ocr이 TimeoutError → status='failed', 한국어 error 메시지"""
        mock_vlm = MagicMock()
        mock_vlm.ocr.side_effect = TimeoutError("VLM 처리 시간 초과")
        mock_vlm_cls.return_value = mock_vlm

        result = _execute(
            document_id="doc-timeout",
            workflow_id="wf-timeout",
            file_path="/uploads/x.pdf",
        )

        # status='failed' + error 필드 + Hangul 포함
        assert "error" in result
        assert any("가" <= ch <= "힯" for ch in result["error"]) or "OCR" in result["error"]

        kwargs = mock_callback.call_args_list[0].kwargs
        assert kwargs["status"] == "failed"


@patch("pipelines.workers.ingestion_worker.post_callback")
@patch("pipelines.workers.ingestion_worker.ScoreTrigger")
@patch("pipelines.workers.ingestion_worker.VectorStore")
@patch("pipelines.workers.ingestion_worker.EmbeddingService")
@patch("pipelines.workers.ingestion_worker.VLMProcessor")
class TestExecuteVLMEmpty:
    """REQ-INGEST-001: 빈 OCR 결과 → IngestionEmptyError → status='failed'"""

    def test_execute_vlm_empty_result_returns_failed(
        self,
        mock_vlm_cls: MagicMock,
        mock_emb_cls: MagicMock,
        mock_vs_cls: MagicMock,
        mock_st_cls: MagicMock,
        mock_callback: MagicMock,
    ) -> None:
        """OCR 결과 빈 문자열 → IngestionEmptyError → status='failed', 한국어 메시지"""
        mock_vlm = MagicMock()
        mock_vlm.ocr.return_value = ""  # 빈 OCR
        mock_vlm.last_inference_meta = {"inference_backend": "transformers_cpu"}
        mock_vlm_cls.return_value = mock_vlm

        result = _execute(
            document_id="doc-empty",
            workflow_id="wf-empty",
            file_path="/uploads/blank.pdf",
        )

        assert "error" in result
        # 한국어 에러 메시지
        assert "비어" in result["error"] or "empty" in result["error"].lower()

        kwargs = mock_callback.call_args_list[0].kwargs
        assert kwargs["status"] == "failed"


@patch("pipelines.workers.ingestion_worker.post_callback")
@patch("pipelines.workers.ingestion_worker.ScoreTrigger")
@patch("pipelines.workers.ingestion_worker.VectorStore")
@patch("pipelines.workers.ingestion_worker.EmbeddingService")
@patch("pipelines.workers.ingestion_worker.VLMProcessor")
class TestExecuteScoreTriggerFailure:
    """AC-INGEST-001-4: 채점 트리거 503 → score_triggered=False, status='completed' 유지"""

    def test_execute_score_trigger_503_still_completes(
        self,
        mock_vlm_cls: MagicMock,
        mock_emb_cls: MagicMock,
        mock_vs_cls: MagicMock,
        mock_st_cls: MagicMock,
        mock_callback: MagicMock,
    ) -> None:
        """ScoreTrigger.fire() False (503) → result_json['score_triggered']=False, status='completed'"""
        mock_vlm = MagicMock()
        mock_vlm.ocr.return_value = "유효한 OCR 텍스트 " * 200
        mock_vlm.last_inference_meta = {"inference_backend": "transformers_cpu"}
        mock_vlm_cls.return_value = mock_vlm

        mock_emb = MagicMock()
        mock_emb.encode.return_value = [0.1] * 768
        mock_emb_cls.return_value = mock_emb

        mock_vs = MagicMock()
        mock_vs.upsert.return_value = None
        mock_vs_cls.return_value = mock_vs

        mock_st = MagicMock()
        mock_st.fire.return_value = False  # 채점 트리거 503/504/네트워크 실패
        mock_st_cls.return_value = mock_st

        result = _execute(
            document_id="doc-st-fail",
            workflow_id="wf-st-fail",
            file_path="/uploads/x.pdf",
        )

        assert result["score_triggered"] is False

        # status='completed' — 채점 트리거 실패는 워크플로우 status에 영향 없음
        kwargs = mock_callback.call_args_list[0].kwargs
        assert kwargs["status"] == "completed"


@patch("pipelines.workers.ingestion_worker.post_callback")
@patch("pipelines.workers.ingestion_worker.ScoreTrigger")
@patch("pipelines.workers.ingestion_worker.VectorStore")
@patch("pipelines.workers.ingestion_worker.EmbeddingService")
@patch("pipelines.workers.ingestion_worker.VLMProcessor")
class TestExecuteCallbackContract:
    """AC-INGEST-001-7: callback 인터페이스 보존 검증"""

    def test_execute_callback_interface_preserved(
        self,
        mock_vlm_cls: MagicMock,
        mock_emb_cls: MagicMock,
        mock_vs_cls: MagicMock,
        mock_st_cls: MagicMock,
        mock_callback: MagicMock,
    ) -> None:
        """post_callback 호출 키워드는 (base_url, workflow_id, status, result_json) 4개 보존"""
        mock_vlm = MagicMock()
        mock_vlm.ocr.return_value = "텍스트 " * 100
        mock_vlm.last_inference_meta = {"inference_backend": "transformers_cpu"}
        mock_vlm_cls.return_value = mock_vlm

        mock_emb_cls.return_value.encode.return_value = [0.1] * 768
        mock_vs_cls.return_value.upsert.return_value = None
        mock_st_cls.return_value.fire.return_value = True

        _execute(
            document_id="doc-iface",
            workflow_id="wf-iface",
            file_path="/uploads/x.pdf",
        )

        kwargs = mock_callback.call_args_list[0].kwargs
        # 정확히 4개 키워드 인자
        assert set(kwargs.keys()) == {"base_url", "workflow_id", "status", "result_json"}


@patch("pipelines.workers.ingestion_worker.post_callback")
@patch("pipelines.workers.ingestion_worker.ScoreTrigger")
@patch("pipelines.workers.ingestion_worker.VectorStore")
@patch("pipelines.workers.ingestion_worker.EmbeddingService")
@patch("pipelines.workers.ingestion_worker.VLMProcessor")
class TestExecuteChunkEmbeddingFailure:
    """EC-10: 청크 단위 임베딩 실패 → skip + WARNING + failed_chunks 필드"""

    def test_execute_chunk_embedding_failure_skip(
        self,
        mock_vlm_cls: MagicMock,
        mock_emb_cls: MagicMock,
        mock_vs_cls: MagicMock,
        mock_st_cls: MagicMock,
        mock_callback: MagicMock,
        caplog: pytest.LogCaptureFixture,
    ) -> None:
        """3 청크 중 1개 embedding 실패 → 2개 upsert + failed_chunks=1, status='completed'"""
        mock_vlm = MagicMock()
        # 청크 분할이 정확히 3개 발생하도록 길이 3500자 (step=1408 → start=0/1408/2816, end=4352≥3500 break)
        mock_vlm.ocr.return_value = "X" * 3500
        mock_vlm.last_inference_meta = {"inference_backend": "transformers_cpu"}
        mock_vlm_cls.return_value = mock_vlm

        # embedding 호출 시 2번째에서 실패
        mock_emb = MagicMock()
        mock_emb.encode.side_effect = [
            [0.1] * 768,  # 청크 1 성공
            RuntimeError("embedding model crashed"),  # 청크 2 실패
            [0.3] * 768,  # 청크 3 성공
        ]
        mock_emb_cls.return_value = mock_emb

        mock_vs = MagicMock()
        mock_vs.upsert.return_value = None
        mock_vs_cls.return_value = mock_vs

        mock_st = MagicMock()
        mock_st.fire.return_value = True
        mock_st_cls.return_value = mock_st

        result = _execute(
            document_id="doc-ec10",
            workflow_id="wf-ec10",
            file_path="/uploads/x.pdf",
        )

        # failed_chunks 필드 존재 + 1
        assert result.get("failed_chunks") == 1
        # chunks = 성공한 청크 수 (3 - 1 = 2)
        assert result["chunks"] == 2
        # status='completed' 유지
        kwargs = mock_callback.call_args_list[0].kwargs
        assert kwargs["status"] == "completed"


@patch("pipelines.workers.ingestion_worker.post_callback")
@patch("pipelines.workers.ingestion_worker.ScoreTrigger")
@patch("pipelines.workers.ingestion_worker.VectorStore")
@patch("pipelines.workers.ingestion_worker.EmbeddingService")
@patch("pipelines.workers.ingestion_worker.VLMProcessor")
class TestExecuteVectorStoreFailure:
    """AC-INGEST-001: VectorStore.upsert 실패 → status='failed'"""

    def test_execute_vector_store_failure_returns_failed(
        self,
        mock_vlm_cls: MagicMock,
        mock_emb_cls: MagicMock,
        mock_vs_cls: MagicMock,
        mock_st_cls: MagicMock,
        mock_callback: MagicMock,
    ) -> None:
        """VectorStore.upsert이 일관되게 raise → status='failed'"""
        mock_vlm = MagicMock()
        mock_vlm.ocr.return_value = "OCR 텍스트 " * 200
        mock_vlm.last_inference_meta = {"inference_backend": "transformers_cpu"}
        mock_vlm_cls.return_value = mock_vlm

        mock_emb_cls.return_value.encode.return_value = [0.1] * 768

        # VectorStore.upsert이 항상 실패
        mock_vs = MagicMock()
        mock_vs.upsert.side_effect = RuntimeError("pgvector connection lost")
        mock_vs_cls.return_value = mock_vs

        result = _execute(
            document_id="doc-vs-fail",
            workflow_id="wf-vs-fail",
            file_path="/uploads/x.pdf",
        )

        assert "error" in result
        kwargs = mock_callback.call_args_list[0].kwargs
        assert kwargs["status"] == "failed"
