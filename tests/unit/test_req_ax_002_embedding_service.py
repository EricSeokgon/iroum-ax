"""REQ-AX-002 EmbeddingService 단위 테스트 — RED phase

ko-sroberta-multitask 임베딩 서비스의 계약을 정의한다.
단위 테스트에서 실제 모델 로딩을 방지하기 위해 mock을 사용한다.

# @MX:TODO: [AUTO] GREEN phase에서 pipelines.mapping.embedding_service.EmbeddingService 구현 필요
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest


class TestEmbeddingServiceContract:
    """EmbeddingService 기본 계약 검증"""

    def test_encode_returns_list_of_floats(self, mock_ko_sroberta: MagicMock) -> None:
        """encode()가 float 리스트를 반환해야 한다."""
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta,
        ):
            service = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")
            result = service.encode("안전교육 이수율 평가기준")

        assert isinstance(result, list)
        assert all(isinstance(v, float) for v in result)

    def test_encode_returns_vector_of_dim_768(
        self, mock_ko_sroberta: MagicMock
    ) -> None:
        """encode()가 768차원 벡터를 반환해야 한다.

        ko-sroberta-multitask는 768차원 임베딩을 생성한다.
        spec.md §6 D12: VECTOR(768) 사용 확정.
        """
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta,
        ):
            service = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")
            result = service.encode("안전보건 관리체계")

        assert len(result) == 768

    def test_encode_vector_has_nonzero_norm(
        self, mock_ko_sroberta: MagicMock
    ) -> None:
        """encode()가 반환한 벡터의 L2 노름이 0보다 커야 한다.

        영벡터는 유효한 임베딩이 아니므로 norm > 0 을 보장해야 함.
        """
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta,
        ):
            service = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")
            result = service.encode("안전교육 이수율")

        norm_sq = sum(v * v for v in result)
        assert norm_sq > 0.0, "임베딩 벡터의 L2 노름이 0이면 안 됨"

    def test_encode_handles_hanja_text_without_exception(
        self, mock_ko_sroberta: MagicMock
    ) -> None:
        """한자가 포함된 텍스트를 인코딩할 때 예외가 발생하지 않아야 한다.

        AC-002-5: 한자 정규화 실패 시에도 임베딩 자체는 fail하지 않아야 함.
        """
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta,
        ):
            service = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")

            try:
                result = service.encode("安全 教育 기준 古文書 보존")
            except Exception as e:
                pytest.fail(f"한자 포함 텍스트 인코딩 시 예외 발생: {e}")

        assert len(result) == 768

    def test_encode_empty_string_returns_zero_vector_or_raises_value_error(
        self, mock_ko_sroberta: MagicMock
    ) -> None:
        """빈 문자열 인코딩 시 영벡터 반환 또는 ValueError를 발생시켜야 한다.

        양쪽 모두 허용됨 — 구현에서 선택.
        """
        from pipelines.mapping.embedding_service import EmbeddingService  # type: ignore[import]

        with patch(
            "pipelines.mapping.embedding_service.SentenceTransformer",
            return_value=mock_ko_sroberta,
        ):
            service = EmbeddingService(model_name="jhgan/ko-sroberta-multitask")

            try:
                result = service.encode("")
                # 영벡터 반환 경우: 768차원이어야 함
                assert len(result) == 768
            except ValueError:
                pass  # ValueError 발생도 허용


@pytest.fixture()
def mock_ko_sroberta() -> MagicMock:
    """ko-sroberta-multitask SentenceTransformer 모의 객체.

    실제 모델 다운로드/로딩 없이 768차원 벡터를 반환한다.
    """
    import random

    mock = MagicMock()

    def fake_encode(
        text: str, convert_to_tensor: bool = False, **kwargs: object
    ) -> list[float]:
        """768차원 임의 벡터 반환 (norm > 0 보장)"""
        if not text:
            return [0.0] * 768
        # 재현 가능한 난수 (텍스트 기반 seed)
        rng = random.Random(hash(text) % (2**32))
        return [rng.gauss(0, 0.1) for _ in range(768)]

    mock.encode.side_effect = fake_encode
    mock.get_sentence_embedding_dimension.return_value = 768
    return mock
