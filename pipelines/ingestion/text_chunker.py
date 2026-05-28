"""SPEC-AX-INGEST-001 — Character-based sliding window 텍스트 청커

REQ-INGEST-002 — VLM OCR 결과를 청크 단위로 분할하여 임베딩 입력으로 사용.
D2 결정: chunk_size=1536 chars, overlap=128 chars (768-dim 임베딩 차원 정렬).

외부 의존성 없음 — pure-Python 구현.
"""
from __future__ import annotations


class TextChunker:
    """문자 기반 슬라이딩 윈도우 청커.

    Args:
        chunk_size: 청크당 최대 문자 수 (기본 1536)
        overlap: 인접 청크 간 중복 문자 수 (기본 128)

    Raises:
        ValueError: chunk_size <= 0 또는 overlap >= chunk_size

    Example:
        >>> chunker = TextChunker(chunk_size=100, overlap=10)
        >>> chunks = chunker.chunk("긴 한국어 문서...")
    """

    def __init__(self, chunk_size: int = 1536, overlap: int = 128) -> None:
        if chunk_size <= 0:
            raise ValueError(f"chunk_size는 양수여야 합니다 (실제: {chunk_size})")
        if overlap >= chunk_size:
            raise ValueError(
                f"overlap({overlap})은 chunk_size({chunk_size})보다 작아야 합니다"
            )
        self.chunk_size = chunk_size
        self.overlap = overlap

    def chunk(self, text: str) -> list[str]:
        """텍스트를 sliding window 방식으로 청크 리스트로 분할한다.

        Args:
            text: 분할할 원본 텍스트 (한국어 포함 가능)

        Returns:
            청크 문자열 리스트. 빈 문자열은 빈 리스트 반환.
        """
        if not text:
            return []

        # 텍스트 길이가 청크 크기 이하면 단일 청크
        if len(text) <= self.chunk_size:
            return [text]

        # sliding window: step = chunk_size - overlap
        step = self.chunk_size - self.overlap
        chunks: list[str] = []
        start = 0
        while start < len(text):
            end = start + self.chunk_size
            chunks.append(text[start:end])
            # 마지막 청크가 끝까지 포함했으면 종료
            if end >= len(text):
                break
            start += step
        return chunks
