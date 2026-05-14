"""ko-sroberta-multitask 임베딩 서비스 (REQ-AX-002)

# @MX:ANCHOR: [AUTO] EmbeddingService.encode — 텍스트를 768차원 벡터로 변환하는 RAG 핵심 함수
# @MX:REASON: CriterionParser→EmbeddingService→VectorStore 파이프라인에서 fan_in >= 3
# @MX:SPEC: SPEC-AX-001 REQ-AX-002-E1
"""
from __future__ import annotations

# 테스트 패치 대상이 되려면 모듈 네임스페이스에 이름이 있어야 하므로
# 조건부 임포트로 실제 패키지 없이도 로딩 가능하게 한다.
try:
    from sentence_transformers import SentenceTransformer  # type: ignore[import-untyped]
except ImportError:
    SentenceTransformer = None  # type: ignore[assignment,misc]

# 지원 임베딩 차원
_EXPECTED_DIM = 768


class EmbeddingService:
    """ko-sroberta-multitask 기반 텍스트 임베딩 서비스.

    단위 테스트에서는 SentenceTransformer를 patch하여 실제 모델 로딩을 방지한다.
    """

    def __init__(self, model_name: str = "jhgan/ko-sroberta-multitask") -> None:
        self._model_name = model_name
        # 지연 초기화 — 첫 encode() 호출 시 로딩
        self._model: object | None = None

    def _get_model(self) -> object:
        """SentenceTransformer 모델을 지연 로딩한다."""
        if self._model is None:
            # 테스트에서 이 라인을 패치한다:
            # patch("pipelines.mapping.embedding_service.SentenceTransformer", ...)
            self._model = SentenceTransformer(self._model_name)  # type: ignore[misc]
        return self._model

    def encode(self, text: str) -> list[float]:
        """텍스트를 768차원 float 벡터로 인코딩한다.

        Args:
            text: 임베딩할 텍스트 (한자 포함 가능)

        Returns:
            768차원 float 리스트

        Note:
            빈 문자열은 768차원 영벡터를 반환한다.
        """
        if not text:
            return [0.0] * _EXPECTED_DIM

        model = self._get_model()
        # SentenceTransformer.encode()는 numpy array 또는 list를 반환함
        raw: object = model.encode(text)  # type: ignore[attr-defined]

        # numpy array → python list[float] 변환
        if hasattr(raw, "tolist"):
            vector: list[float] = raw.tolist()  # type: ignore[assignment]
        elif isinstance(raw, list):
            vector = [float(v) for v in raw]
        else:
            vector = list(raw)  # type: ignore[arg-type]

        return vector
