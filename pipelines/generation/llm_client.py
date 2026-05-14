"""LLM 클라이언트 — 외부 엔드포인트 차단 + generate() 포함 (REQ-UBI-001, REQ-AX-004)

실제 HTTP 호출 전에 allowlist 검증을 수행한다.

# @MX:ANCHOR: [AUTO] LLMClient.generate — LLM 호출 및 fallback 진입점
# @MX:REASON: ReportDrafter, 통합 테스트, 단위 테스트 모두 이 메서드를 호출함
# @MX:SPEC: SPEC-AX-001 REQ-AX-004 / AC-004-3
"""
from __future__ import annotations

from pkg.models.report import GenerationResult

from pipelines.config.settings import validate_llm_endpoint

# Qwen 2.5 7B 기본 모델명 (AC-004-3)
_DEFAULT_MODEL = "qwen2.5-7b"
# EXAONE 최대 재시도 횟수 (3회 실패 → fallback)
_EXAONE_MAX_RETRIES = 3


class LLMClient:
    """LLM 엔드포인트 래퍼 — 외부 URL 차단 + 텍스트 생성 포함

    # @MX:NOTE: [AUTO] 외부 LLM 차단 — REQ-UBI-001 데이터 주권
    # @MX:SPEC: SPEC-AX-001 REQ-UBI-001 / AC-UBI-001
    """

    def __init__(self, endpoint: str) -> None:
        """LLMClient 초기화.

        Args:
            endpoint: LLM 서버 엔드포인트 URL (내부 서버여야 함)
        """
        self._endpoint = endpoint

    def validate_endpoint(self) -> None:
        """엔드포인트가 내부 allowlist에 있는지 검증한다.

        Raises:
            ExternalLLMBlockedError: 외부 엔드포인트 사용 시도 시
        """
        validate_llm_endpoint(self._endpoint)

    def _call_model(
        self,
        prompt: str,
        model: str = _DEFAULT_MODEL,
        *,
        use_gpu: bool = False,
    ) -> GenerationResult:
        """실제 모델 호출 내부 메서드 — 테스트에서 patch 대상.

        Args:
            prompt: 입력 프롬프트 텍스트
            model: 사용할 모델명
            use_gpu: GPU 디바이스 사용 여부

        Returns:
            GenerationResult

        Raises:
            NotImplementedError: 실제 구현은 추후 Sprint에서 진행
        """
        raise NotImplementedError("실제 LLM 호출은 추후 Sprint에서 구현됩니다.")  # noqa: EM101

    def generate(self, prompt: str, *, use_gpu: bool = False) -> GenerationResult:
        """프롬프트로 텍스트를 생성한다.

        내부 allowlist에 없는 외부 엔드포인트이면 실제 HTTP 연결 전에 차단한다.
        EXAONE 3회 실패 시 Qwen 2.5 7B로 자동 fallback한다 (AC-004-3).

        Args:
            prompt: 입력 프롬프트 텍스트
            use_gpu: GPU 디바이스 사용 여부 (기본값: False → CPU)

        Returns:
            GenerationResult (text, model_used, tokens, latency_ms)

        Raises:
            ExternalLLMBlockedError: 외부 엔드포인트 사용 시도 시
        """
        # 외부 엔드포인트 검증
        validate_llm_endpoint(self._endpoint)

        # EXAONE 시도 (최대 _EXAONE_MAX_RETRIES 회) → 실패 시 Qwen fallback
        for _ in range(_EXAONE_MAX_RETRIES):
            try:
                result = self._call_model(prompt, model="exaone", use_gpu=use_gpu)
                return result
            except Exception:  # noqa: BLE001
                pass

        # EXAONE 실패 → Qwen 2.5 7B fallback
        return self._call_model(prompt, model=_DEFAULT_MODEL, use_gpu=use_gpu)
