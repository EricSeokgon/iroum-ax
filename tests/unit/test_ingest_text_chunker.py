"""SPEC-AX-INGEST-001 — TextChunker 단위 테스트 (RED 단계)

REQ-INGEST-002 — OCR 텍스트를 청크 단위로 분할하여 임베딩 입력으로 사용.
character-based sliding window: chunk_size=1536 chars, overlap=128 chars (D2 결정).

격리: 외부 의존성 없음 — pure function 테스트.
"""
from __future__ import annotations

import pytest
from pipelines.ingestion.text_chunker import TextChunker


class TestTextChunker:
    """TextChunker — character-based sliding window 단위 테스트"""

    def test_chunk_short_text_returns_single_chunk(self) -> None:
        """chunk_size 미만 텍스트 → 단일 청크 반환"""
        chunker = TextChunker(chunk_size=1536, overlap=128)
        text = "짧은 문장입니다."

        chunks = chunker.chunk(text)

        assert chunks == [text]

    def test_chunk_empty_text_returns_empty_list(self) -> None:
        """빈 문자열 → 빈 리스트"""
        chunker = TextChunker(chunk_size=1536, overlap=128)

        chunks = chunker.chunk("")

        assert chunks == []

    def test_chunk_exact_size_text_returns_single_chunk(self) -> None:
        """정확히 chunk_size 길이 텍스트 → 단일 청크"""
        chunker = TextChunker(chunk_size=100, overlap=10)
        text = "a" * 100

        chunks = chunker.chunk(text)

        assert len(chunks) == 1
        assert chunks[0] == text

    def test_chunk_larger_than_size_returns_multiple_chunks(self) -> None:
        """chunk_size 초과 텍스트 → 다중 청크, overlap 적용"""
        chunker = TextChunker(chunk_size=100, overlap=10)
        text = "a" * 250  # 100 + (100-10) + (60) = 3 청크 예상

        chunks = chunker.chunk(text)

        assert len(chunks) >= 2
        # 각 청크는 chunk_size를 초과하지 않아야 함
        for chunk in chunks:
            assert len(chunk) <= 100
        # 모든 문자가 어떤 청크에든 포함되어야 함 (overlap 허용)
        assert chunks[0] == "a" * 100

    def test_chunk_korean_text_handled_correctly(self) -> None:
        """한국어 텍스트도 정상 청킹"""
        chunker = TextChunker(chunk_size=50, overlap=5)
        text = "한국어 평가편람 문서 분석 결과입니다. " * 10  # 약 200자

        chunks = chunker.chunk(text)

        assert len(chunks) >= 2
        # 텍스트 데이터 손실 없음 (overlap 고려)
        for chunk in chunks:
            assert len(chunk) <= 50

    def test_chunk_overlap_applied(self) -> None:
        """overlap이 실제로 적용되는지 검증 — 첫 청크 끝과 둘째 청크 시작이 겹침"""
        chunker = TextChunker(chunk_size=20, overlap=5)
        text = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "12345"  # 31자

        chunks = chunker.chunk(text)

        assert len(chunks) >= 2
        # 첫 청크 마지막 5자 == 둘째 청크 처음 5자
        first = chunks[0]
        second = chunks[1]
        assert first[-5:] == second[:5]

    def test_chunk_default_size_is_1536(self) -> None:
        """기본 chunk_size는 1536 (D2 결정)"""
        chunker = TextChunker()

        assert chunker.chunk_size == 1536
        assert chunker.overlap == 128

    def test_chunk_invalid_overlap_raises_valueerror(self) -> None:
        """overlap >= chunk_size 시 ValueError"""
        with pytest.raises(ValueError):
            TextChunker(chunk_size=100, overlap=100)

    def test_chunk_negative_size_raises_valueerror(self) -> None:
        """chunk_size <= 0 시 ValueError"""
        with pytest.raises(ValueError):
            TextChunker(chunk_size=0, overlap=0)
