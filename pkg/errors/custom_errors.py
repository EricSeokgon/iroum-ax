"""공유 커스텀 예외 정의 스켈레톤

각 예외 클래스의 실제 raise 로직은 Sprint 2-6에서 구현됨.
"""


class IroumAxBaseError(Exception):
    """iroum-ax 기본 예외 클래스"""


class ExternalLLMBlockedError(IroumAxBaseError):
    """LLM 엔드포인트 차단/불허 시 발생 (REQ-AX-004 allowlist 검증)"""


class OCRConcurrencyError(IroumAxBaseError):
    """VLM OCR 동시 요청 한도 초과 시 발생 (REQ-AX-001)"""


class StyleViolationError(IroumAxBaseError):
    """한국어 공문 합니다체 검증 실패 시 발생 (REQ-AX-004)"""


class BenchmarkNotAvailableError(IroumAxBaseError):
    """비교 대상 벤치마크 보고서 부재 시 발생 (RAG 콜드스타트, REQ-AX-002)"""


class IndexRebuildingError(IroumAxBaseError):
    """pgvector HNSW 인덱스 재구성 중 검색 요청 시 발생 (REQ-AX-002)"""
