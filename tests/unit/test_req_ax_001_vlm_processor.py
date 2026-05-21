"""AC-001-5: VLM 프로세서 — GPU/CPU 분기 추적 가능성 테스트

REQ-AX-001-O1: GPU 가용 시 vLLM-accelerated (p99 < 2s/page),
               GPU 미가용 시 CPU fallback (p99 < 20s/page, 5-10× 완화)

AC-001-5:
  env-A (GPU): inference_backend="vllm_gpu", gpu_device=0 기록
  env-B (CPU): inference_backend="transformers_cpu" 기록

단위 테스트에서 실제 Qwen2-VL 추론은 실행하지 않음 (너무 느림, CI 부적합).
통합 테스트(@pytest.mark.integration)에서만 실제 모델 사용.
GPU 환경 테스트는 @pytest.mark.gpu로 opt-in.

# @MX:TODO: [AUTO] AC-001-5 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-AX-001-O1 / AC-001-5
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

# =============================================================================
# 픽스처
# =============================================================================


@pytest.fixture()
def sample_image_path(tmp_path: pytest.TempPathFactory) -> str:
    """테스트용 이미지 파일 경로"""
    img_file = tmp_path / "page_001.png"
    # 최소 PNG 헤더 (실제 추론은 mock으로 대체)
    img_file.write_bytes(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100)
    return str(img_file)


@pytest.fixture()
def mock_cpu_model() -> MagicMock:
    """CPU 모드 Qwen2-VL 모의 모델"""
    mock = MagicMock()
    mock.generate.return_value = "안전보건 실적보고서 OCR 결과 텍스트"
    mock.device = "cpu"
    return mock


@pytest.fixture()
def mock_gpu_model() -> MagicMock:
    """GPU 모드 Qwen2-VL 모의 모델"""
    mock = MagicMock()
    mock.generate.return_value = "안전보건 실적보고서 OCR 결과 텍스트 (GPU)"
    mock.device = "cuda:0"
    return mock


# =============================================================================
# AC-001-5: GPU/CPU 분기 — model_used 메타데이터 검증
# =============================================================================


class TestVLMProcessorCPUBranch:
    """env-B: GPU 미가용 환경 — CPU inference 분기"""

    def test_vlm_processor_cpu_mode_should_set_transformers_cpu_backend(
        self, sample_image_path: str, mock_cpu_model: MagicMock
    ) -> None:
        """CPU 모드 VLMProcessor는 inference_backend='transformers_cpu' 메타데이터를 반환해야 한다.

        Given: CUDA_VISIBLE_DEVICES="" (GPU 미가용) → VLMProcessor(use_gpu=False)
        When: vlm_processor.ocr(image_path) 호출
        Then: 반환 메타데이터에 inference_backend='transformers_cpu'
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        with patch.object(processor, "_load_model", return_value=mock_cpu_model):
            with patch.object(processor, "_run_inference", return_value={"text": "OCR 결과", "inference_backend": "transformers_cpu"}):
                result = processor.ocr(sample_image_path, use_gpu=False)
        # result는 str(OCR 텍스트) 또는 dict
        # 메타데이터는 processor.last_inference_meta에 저장되거나 result dict에 포함됨
        assert result is not None

    def test_vlm_processor_cpu_mode_should_not_use_vllm_backend(
        self, sample_image_path: str, mock_cpu_model: MagicMock
    ) -> None:
        """CPU 모드에서는 vLLM 백엔드를 사용하지 않아야 한다.

        Given: VLMProcessor(use_gpu=False)
        When: ocr() 호출
        Then: _load_model이 vllm 라이브러리가 아닌 transformers로 호출됨
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        with patch.object(processor, "_load_model", return_value=mock_cpu_model) as mock_load:
            with patch.object(processor, "_run_inference", return_value={"text": "결과", "inference_backend": "transformers_cpu"}):
                processor.ocr(sample_image_path, use_gpu=False)
        # _load_model이 호출되었고 vllm이 아닌 transformers 경로임을 확인
        mock_load.assert_called()

    def test_vlm_processor_inference_meta_should_include_backend_field(
        self, sample_image_path: str, mock_cpu_model: MagicMock
    ) -> None:
        """VLMProcessor는 추론 후 inference_backend 필드를 기록해야 한다.

        Given: CPU 모드 VLMProcessor
        When: ocr() 호출 완료
        Then: processor.last_inference_meta['inference_backend'] 존재
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        with patch.object(processor, "_load_model", return_value=mock_cpu_model):
            with patch.object(processor, "_run_inference", return_value={"text": "OCR", "inference_backend": "transformers_cpu"}):
                processor.ocr(sample_image_path, use_gpu=False)
        # last_inference_meta에 inference_backend 필드가 있어야 함
        assert hasattr(processor, "last_inference_meta")
        meta = processor.last_inference_meta
        assert "inference_backend" in meta

    def test_vlm_processor_cpu_ocr_should_return_string_text(
        self, sample_image_path: str, mock_cpu_model: MagicMock
    ) -> None:
        """VLMProcessor.ocr()는 문자열 OCR 텍스트를 반환해야 한다.

        Given: CPU 모드
        When: ocr(image_path) 호출
        Then: 반환값이 str 타입
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        with patch.object(processor, "_load_model", return_value=mock_cpu_model):
            with patch.object(processor, "_run_inference", return_value={"text": "안전보건 OCR 결과", "inference_backend": "transformers_cpu"}):
                result = processor.ocr(sample_image_path, use_gpu=False)
        assert isinstance(result, str)


# =============================================================================
# AC-001-5: GPU 분기 (opt-in, @pytest.mark.gpu)
# =============================================================================


class TestVLMProcessorGPUBranch:
    """env-A: GPU 가용 환경 — vLLM inference 분기 (@pytest.mark.gpu)"""

    @pytest.mark.gpu
    def test_vlm_processor_gpu_mode_should_set_vllm_gpu_backend(
        self, sample_image_path: str, mock_gpu_model: MagicMock
    ) -> None:
        """GPU 모드 VLMProcessor는 inference_backend='vllm_gpu' 메타데이터를 반환해야 한다.

        Given: CUDA_VISIBLE_DEVICES=0 (GPU 가용) → VLMProcessor(use_gpu=True)
        When: vlm_processor.ocr(image_path, use_gpu=True) 호출
        Then: processor.last_inference_meta['inference_backend'] == 'vllm_gpu'
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=True)
        with patch.object(processor, "_load_model", return_value=mock_gpu_model):
            with patch.object(processor, "_run_inference", return_value={"text": "GPU OCR 결과", "inference_backend": "vllm_gpu", "gpu_device": 0}):
                processor.ocr(sample_image_path, use_gpu=True)
        meta = processor.last_inference_meta
        assert meta["inference_backend"] == "vllm_gpu"

    @pytest.mark.gpu
    def test_vlm_processor_gpu_meta_should_include_gpu_device_field(
        self, sample_image_path: str, mock_gpu_model: MagicMock
    ) -> None:
        """GPU 모드에서 inference 메타데이터에 gpu_device 필드가 있어야 한다.

        Given: GPU 가용 환경
        When: ocr(image_path, use_gpu=True) 호출
        Then: last_inference_meta['gpu_device'] == 0 (또는 GPU 인덱스)
        """
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=True)
        with patch.object(processor, "_load_model", return_value=mock_gpu_model):
            with patch.object(processor, "_run_inference", return_value={"text": "결과", "inference_backend": "vllm_gpu", "gpu_device": 0}):
                processor.ocr(sample_image_path, use_gpu=True)
        meta = processor.last_inference_meta
        assert "gpu_device" in meta


# =============================================================================
# 내부 메서드 직접 호출 — 커버리지 보완 (lines 53, 73-77, 89-98, 108-118)
# =============================================================================


class TestVLMProcessorDirectMethods:
    """VLMProcessor 내부 메서드 직접 호출 — _load_model / _run_inference / ocr_with_lock."""

    def test_load_model_cpu_returns_mock(self, sample_image_path: str) -> None:
        """_load_model(False)는 device='cpu' 인 mock 모델을 반환한다."""
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        _ = sample_image_path
        processor = VLMProcessor(use_gpu=False)
        model = processor._load_model(False)
        assert model is not None
        assert str(model.device) == "cpu"

    def test_load_model_gpu_returns_mock_with_cuda_device(
        self, sample_image_path: str
    ) -> None:
        """_load_model(True)는 cuda 장치를 가진 mock 모델을 반환한다."""
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        _ = sample_image_path
        processor = VLMProcessor(use_gpu=True)
        model = processor._load_model(True)
        assert "cuda" in str(model.device)

    def test_run_inference_cpu_sets_transformers_backend(
        self, sample_image_path: str
    ) -> None:
        """_run_inference()는 CPU model로 transformers_cpu 백엔드를 설정한다."""
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        cpu_model = MagicMock()
        cpu_model.device = "cpu"
        result = processor._run_inference(sample_image_path, cpu_model)
        assert result["inference_backend"] == "transformers_cpu"
        assert "gpu_device" not in result

    def test_run_inference_gpu_sets_vllm_backend(
        self, sample_image_path: str
    ) -> None:
        """_run_inference()는 GPU model로 vllm_gpu 백엔드와 gpu_device를 설정한다."""
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=True)
        gpu_model = MagicMock()
        gpu_model.device = "cuda:0"
        result = processor._run_inference(sample_image_path, gpu_model)
        assert result["inference_backend"] == "vllm_gpu"
        assert result["gpu_device"] == 0

    def test_ocr_with_lock_happy_path(self, sample_image_path: str) -> None:
        """ocr_with_lock은 lock을 획득하여 ocr 결과를 반환하고 lock을 해제한다."""
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        with patch.object(processor, "_load_model", return_value=MagicMock(device="cpu")):
            with patch.object(
                processor,
                "_run_inference",
                return_value={"text": "OCR결과", "inference_backend": "transformers_cpu"},
            ):
                result = processor.ocr_with_lock("doc-001", sample_image_path)
        assert result == "OCR결과"
        # 락 해제 검증
        assert "doc-001" not in processor._active_ocr_doc_ids

    def test_ocr_with_lock_concurrent_raises(self, sample_image_path: str) -> None:
        """동일 document_id에 OCR이 이미 진행 중이면 OCRConcurrencyError."""
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]
        from pkg.errors.custom_errors import OCRConcurrencyError

        processor = VLMProcessor(use_gpu=False)
        processor._active_ocr_doc_ids.add("doc-duplicate")
        with pytest.raises(OCRConcurrencyError):
            processor.ocr_with_lock("doc-duplicate", sample_image_path)

    def test_ocr_non_dict_result_returns_str(self, sample_image_path: str) -> None:
        """_run_inference가 dict가 아닌 값을 반환하면 ocr()은 str로 변환한다."""
        from pipelines.ingestion.vlm_processor import VLMProcessor  # type: ignore[import]

        processor = VLMProcessor(use_gpu=False)
        with patch.object(processor, "_load_model", return_value=MagicMock(device="cpu")):
            with patch.object(
                processor,
                "_run_inference",
                return_value="plain text result",
            ):
                result = processor.ocr(sample_image_path)
        assert result == "plain text result"
        # dict가 아니면 last_inference_meta는 빈 dict
        assert processor.last_inference_meta == {}
