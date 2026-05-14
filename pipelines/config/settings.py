"""환경 설정 — Pydantic Settings 기반

모든 설정은 환경 변수 또는 .env 파일에서 로드됨.
기본값은 로컬 개발(docker-compose) 환경 기준.
"""
from __future__ import annotations

from urllib.parse import urlparse

from pkg.errors.custom_errors import ExternalLLMBlockedError
from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

# 허용된 내부 호스트 패턴 (allowlist)
# @MX:NOTE: [AUTO] 외부 LLM 차단을 위한 allowlist — REQ-UBI-001 AC-UBI-001
# @MX:SPEC: SPEC-AX-001 REQ-UBI-001
_ALLOWED_HOSTS: frozenset[str] = frozenset(
    [
        "localhost",
        "127.0.0.1",
        "::1",
    ]
)


def validate_llm_endpoint(url: str) -> bool:
    """LLM 엔드포인트 URL이 내부 allowlist에 속하는지 검증한다.

    # @MX:ANCHOR: [AUTO] 외부 LLM 호출 차단 진입점 — fan_in >= 3 예상
    # @MX:REASON: REQ-UBI-001 데이터 주권 — 외부 LLM API 호출 원천 차단

    Args:
        url: 검증할 LLM 엔드포인트 URL

    Returns:
        True — 내부 allowlist 호스트인 경우

    Raises:
        ExternalLLMBlockedError: 외부 호스트이거나 파싱 불가 URL인 경우
    """
    try:
        parsed = urlparse(url)
        host = parsed.hostname or ""
    except Exception as exc:
        raise ExternalLLMBlockedError(f"LLM 엔드포인트 URL 파싱 실패: {url!r}") from exc

    if host not in _ALLOWED_HOSTS:
        raise ExternalLLMBlockedError(
            f"외부 LLM 엔드포인트 차단됨: {host!r} — allowlist={sorted(_ALLOWED_HOSTS)}"
        )

    return True


class Settings(BaseSettings):
    """iroum-ax 파이프라인 전역 설정"""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    # --- 데이터베이스 ---
    postgres_host: str = Field(default="localhost", description="PostgreSQL 호스트")
    postgres_port: int = Field(default=5432, description="PostgreSQL 포트")
    postgres_user: str = Field(default="ax", description="PostgreSQL 사용자")
    postgres_password: str = Field(
        default="",
        description="PostgreSQL 비밀번호 (운영 환경에서는 POSTGRES_PASSWORD 환경변수로 주입 필수)",
    )
    postgres_db: str = Field(default="iroum_ax", description="PostgreSQL 데이터베이스명")

    @property
    def postgres_dsn(self) -> str:
        """PostgreSQL 연결 문자열 (DSN) 생성"""
        return (
            f"postgresql+asyncpg://{self.postgres_user}:{self.postgres_password}"
            f"@{self.postgres_host}:{self.postgres_port}/{self.postgres_db}"
        )

    # --- Redis ---
    redis_host: str = Field(default="localhost", description="Redis 호스트")
    redis_port: int = Field(default=6379, description="Redis 포트")

    @property
    def redis_url(self) -> str:
        """Redis 연결 URL 생성"""
        return f"redis://{self.redis_host}:{self.redis_port}/0"

    # --- LLM 모델 경로 ---
    # GPU 미사용 환경: transformers 직접 로딩 (vLLM 불필요)
    # GPU 환경(pytest -m gpu): VLM_ENDPOINT 등 vLLM 서버 URL 지정
    model_dir: str = Field(
        default="/models",
        description="로컬 모델 저장 디렉토리 (Qwen2-VL, Qwen 2.5, ko-sroberta)",
    )
    hf_home: str = Field(
        default="/models/hf_cache",
        description="Hugging Face 캐시 디렉토리 (HF_HOME)",
    )

    # Qwen 2.5 (주 LLM — EXAONE 3.5 미접근 시 primary, AC-004-3 결정사항)
    qwen25_model_name: str = Field(
        default="Qwen/Qwen2.5-7B-Instruct",
        description="Qwen 2.5 모델 이름 (Hugging Face Hub ID)",
    )

    # Qwen2-VL (VLM OCR 전용)
    qwen2vl_model_name: str = Field(
        default="Qwen/Qwen2-VL-7B-Instruct",
        description="Qwen2-VL 모델 이름",
    )

    # ko-sroberta-multitask (임베딩 전용)
    embedding_model_name: str = Field(
        default="jhgan/ko-sroberta-multitask",
        description="한국어 임베딩 모델 이름",
    )
    embedding_dim: int = Field(default=768, description="임베딩 벡터 차원 (ko-sroberta: 768)")

    # vLLM 엔드포인트 (GPU 환경 opt-in)
    vlm_endpoint: str = Field(
        default="",
        description="vLLM VLM 서버 URL (비어있으면 transformers 직접 로딩)",
    )
    llm_endpoint: str = Field(
        default="",
        description="vLLM LLM 서버 URL (비어있으면 transformers 직접 로딩)",
    )

    # --- 보안 / 인증 ---
    auth_enabled: bool = Field(
        default=False,
        alias="auth_enabled",
        description="인증 활성화 여부 (PoC: False)",
        validation_alias="AUTH_ENABLED",
    )
    default_user_id: str = Field(
        default="cli-anonymous",
        alias="default_user_id",
        description="미인증 사용자 기본 ID (sandbox 환경)",
        validation_alias="DEFAULT_USER_ID",
    )

    # --- 로깅 ---
    log_level: str = Field(default="INFO", description="로그 레벨")


# 싱글톤 설정 인스턴스
settings = Settings()
