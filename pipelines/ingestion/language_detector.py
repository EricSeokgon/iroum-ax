"""언어 감지 — 한국어 우선 처리 (REQ-UBI-002, AC-UBI-002)

Unicode 범위 기반 한국어 비율 계산.
한글 음절 블록: U+AC00–U+D7AF (가-힣)
"""
from __future__ import annotations

from typing import Any

# 한국어 비율 임계값 — 이 값 미만이면 'low_korean_ratio' 플래그 설정
# @MX:NOTE: [AUTO] 임계값 0.2 — REQ-UBI-002 한국어 우선 처리 기준
_KOREAN_RATIO_THRESHOLD: float = 0.2


def _count_korean_chars(text: str) -> int:
    """텍스트에서 한글 음절(U+AC00–U+D7AF) 수를 반환한다."""
    return sum(1 for ch in text if "가" <= ch <= "힯")


def detect_language(text: str) -> dict[str, Any]:
    """텍스트의 주요 언어를 감지하고 한국어 비율을 반환한다.

    # @MX:ANCHOR: [AUTO] 언어 감지 진입점 — REQ-UBI-002 한국어 우선 처리
    # @MX:REASON: 모든 문서 수집 파이프라인이 이 함수를 통해 언어를 판별

    Args:
        text: 분석할 텍스트

    Returns:
        dict with:
            - lang_code: ISO 639-1 언어 코드 ('ko', 'en' 등)
            - korean_ratio: 전체 알파벳/문자 중 한글 비율 (0.0–1.0)
            - parse_quality_flag: 품질 플래그 (korean_ratio < 임계값이면 'low_korean_ratio')
    """
    if not text:
        return {
            "lang_code": "unknown",
            "korean_ratio": 0.0,
            "parse_quality_flag": "low_korean_ratio",
        }

    # 한글 음절 수 계산
    korean_count = _count_korean_chars(text)
    # 총 문자 수(공백 포함) 대비 비율
    total_chars = len(text)
    korean_ratio = korean_count / total_chars if total_chars > 0 else 0.0

    # 언어 코드 결정
    lang_code = "ko" if korean_ratio >= _KOREAN_RATIO_THRESHOLD else "en"

    # 품질 플래그 결정
    parse_quality_flag: str | None = None
    if korean_ratio < _KOREAN_RATIO_THRESHOLD:
        parse_quality_flag = "low_korean_ratio"

    result: dict[str, Any] = {
        "lang_code": lang_code,
        "korean_ratio": korean_ratio,
    }
    if parse_quality_flag is not None:
        result["parse_quality_flag"] = parse_quality_flag

    return result
