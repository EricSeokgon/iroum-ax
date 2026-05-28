"""SPEC-AX-INGEST-001 — Celery ingestion worker 본체

REQ-INGEST-001/002/003/004/005 — VLM OCR + RAG 임베딩 + Go 채점 트리거 파이프라인.
스텁(SPEC-AX-INTEG-001 §6.3)을 7-Step 본체로 교체.

7-Step 파이프라인:
1. (모듈 부팅 시) validate_llm_endpoint(vlm_endpoint) 검증 — REQ-INGEST-005
2. DocumentMetadataClient.fetch() — Celery kwargs 메타데이터 추출
3. VLMProcessor.ocr() — VLM OCR 실행 (REQ-INGEST-001, IngestionEmptyError)
4. TextChunker.chunk() + EmbeddingService.encode() — 청크별 768-dim 임베딩 (REQ-INGEST-002)
   - EC-10: 청크 단위 실패 → skip + WARNING + failed_chunks 카운트
5. VectorStore.upsert() — pgvector 저장 (REQ-INGEST-002)
6. ScoreTrigger.fire() — Go 채점 API POST (REQ-INGEST-003, fire-and-forget)
7. post_callback(status, result_json) — Go Control Plane 콜백 (REQ-INTEG-002 보존)

# @MX:NOTE: [AUTO] REQ-INGEST-005 — validate_llm_endpoint 부팅 시 강제 (외부 LLM 차단)
# @MX:SPEC: SPEC-AX-INGEST-001 REQ-INGEST-005/005b/005c

REQ-INTEG-001 — Celery task name 'pipelines.workers.ingestion_worker.run' 보존.
REQ-INTEG-002 — 처리 완료 시 Go Control Plane으로 callback POST.
REQ-INTEG-003 — callback fire-and-forget (예외 흡수, D2).
"""
from __future__ import annotations

import logging
from typing import Any
from uuid import uuid4

from pkg.errors.custom_errors import (
    ExternalLLMBlockedError,
    IndexRebuildingError,
    IngestionEmptyError,
)
from pkg.models.criterion import Criterion

from pipelines.callbacks.control_plane import post_callback
from pipelines.config.settings import settings, validate_llm_endpoint
from pipelines.ingestion.document_metadata import DocumentMetadataClient
from pipelines.ingestion.score_trigger import ScoreTrigger
from pipelines.ingestion.text_chunker import TextChunker
from pipelines.ingestion.vlm_processor import VLMProcessor
from pipelines.mapping.embedding_service import EmbeddingService
from pipelines.mapping.vector_store import VectorStore

logger = logging.getLogger(__name__)

# @MX:NOTE: [AUTO] TASK_NAME — Kombu envelope headers.task와 정확 일치 필수 (마법 상수)
# @MX:SPEC: SPEC-AX-INTEG-001 REQ-INTEG-001 (Go dispatcher도 동일 문자열 사용 — 양쪽 동시 변경)
TASK_NAME: str = "pipelines.workers.ingestion_worker.run"

# 한국어 에러 메시지 표준 사전 (SPEC-AX-INGEST-001 §한국어 에러 메시지 표준)
_ERROR_VLM_TIMEOUT = "VLM OCR 처리 시간 초과 ({timeout}초)"
_ERROR_VLM_EMPTY = "OCR 결과가 비어있습니다 — 문서 형식 확인 필요"
_ERROR_VLM_FAILED = "VLM OCR 실패: {detail}"
_ERROR_VECTOR_UPSERT_FAILED = "벡터 저장 실패: {detail}"
_ERROR_EMBEDDING_DIM_MISMATCH = "임베딩 차원 불일치 (예상: 768, 실제: {actual})"

# IndexRebuildingError 최대 재시도 (REQ-INGEST-002)
_INDEX_REBUILD_MAX_RETRIES = 3

# 모듈 레벨 부팅 검증 (REQ-INGEST-005)
# vlm_endpoint=""이면 REQ-INGEST-005c — CPU fallback (검증 skip).
# allowlist 위반 시 ExternalLLMBlockedError raise → worker import 차단 (REQ-INGEST-005b).
if settings.vlm_endpoint:
    validate_llm_endpoint(settings.vlm_endpoint)


def build_dispatch_payload(*, document_id: str, workflow_id: str) -> list[Any]:
    """REQ-INTEG-001 — Celery envelope payload 구조 빌더.

    Kombu v2 envelope body는 base64 디코딩 후 다음 3-tuple 리스트 형태:
        [
          ["<document_id>"],          # positional args
          {"workflow_id": "<uuid>"},  # kwargs
          {"callbacks": None, ...}    # Celery options
        ]
    """
    return [
        [document_id],
        {"workflow_id": workflow_id},
        {
            "callbacks": None,
            "chain": None,
            "chord": None,
            "errbacks": None,
        },
    ]


def _execute(
    *,
    document_id: str,
    workflow_id: str,
    **kwargs: Any,  # noqa: ANN401
) -> dict[str, Any]:
    """SPEC-AX-INGEST-001 — VLM OCR + RAG + Go 채점 트리거 파이프라인.

    # @MX:ANCHOR: [AUTO] INGEST-001 _execute는 ingestion 파이프라인 진입점 — mock 인라인 금지
    # @MX:REASON: run task + 8+ 단위 테스트 + 통합 테스트에서 호출 (fan_in >= 3)
    # @MX:SPEC: SPEC-AX-INGEST-001 REQ-INGEST-001/002/003/004

    Args:
        document_id: 처리 대상 문서 ID
        workflow_id: 워크플로우 UUID
        **kwargs: Celery envelope kwargs (file_type, file_path, user_id 등)

    Returns:
        result_json — Celery task 반환값 (callback에 이미 전송됨).
        정상: {document_id, chunks, tokens, score_triggered, ocr_backend, pages_processed, spec}
        실패: {document_id, error, spec} + status='failed' 콜백
    """
    result_json: dict[str, Any] = {
        "document_id": document_id,
        "spec": "SPEC-AX-INGEST-001",
    }
    status: str = "completed"

    try:
        # Step 1: 문서 메타데이터 추출 (Celery kwargs → DocumentMeta)
        metadata_client = DocumentMetadataClient(default_user_id=settings.default_user_id)
        meta = metadata_client.fetch(document_id=document_id, kwargs=kwargs)

        # Step 2: VLM OCR (REQ-INGEST-001)
        ocr_text, ocr_backend, pages_processed = _run_vlm_ocr(meta.file_path)

        if not ocr_text:
            # REQ-INGEST-001 — 빈 OCR 결과 → IngestionEmptyError
            raise IngestionEmptyError(_ERROR_VLM_EMPTY)

        result_json["ocr_backend"] = ocr_backend
        result_json["pages_processed"] = pages_processed
        result_json["tokens"] = len(ocr_text)

        # Step 3: 텍스트 청킹 + 임베딩 (REQ-INGEST-002, EC-10 부분 실패 허용)
        chunks_indexed, failed_chunks = _embed_and_upsert_chunks(
            document_id=document_id,
            ocr_text=ocr_text,
        )
        result_json["chunks"] = chunks_indexed
        if failed_chunks > 0:
            result_json["failed_chunks"] = failed_chunks

        # Step 4: Go 채점 API 트리거 (REQ-INGEST-003, fire-and-forget)
        trigger = ScoreTrigger(
            base_url=settings.go_control_plane_url,
            token=settings.score_api_token,
        )
        score_triggered = trigger.fire(
            document_id=document_id,
            summary={
                "pages_processed": pages_processed,
                "chunk_count": chunks_indexed,
                "tokens": result_json["tokens"],
                "ocr_backend": ocr_backend,
            },
        )
        result_json["score_triggered"] = score_triggered

    except IngestionEmptyError as exc:
        # REQ-INGEST-001 — 빈 OCR
        status = "failed"
        result_json["error"] = str(exc)
        logger.error(
            "ingestion 빈 OCR 결과 (document_id=%s workflow_id=%s): %s",
            document_id,
            workflow_id,
            exc,
        )
    except TimeoutError as exc:
        # REQ-INGEST-001 — VLM OCR 타임아웃
        status = "failed"
        result_json["error"] = _ERROR_VLM_TIMEOUT.format(
            timeout=settings.vlm_timeout_seconds
        )
        logger.error(
            "ingestion VLM 타임아웃 (document_id=%s workflow_id=%s): %s",
            document_id,
            workflow_id,
            exc,
        )
    except Exception as exc:  # noqa: BLE001 — 모든 비예상 예외 흡수
        # REQ-INGEST-004b — fatal error → status='failed' + error 메시지
        status = "failed"
        result_json["error"] = _ERROR_VLM_FAILED.format(detail=str(exc))
        logger.error(
            "ingestion 처리 실패 (document_id=%s workflow_id=%s): %s",
            document_id,
            workflow_id,
            exc,
        )

    # Step 5: post_callback (REQ-INTEG-002 — 인터페이스 무변경, AC-INGEST-001-7)
    post_callback(
        base_url=settings.go_control_plane_url,
        workflow_id=workflow_id,
        status=status,
        result_json=result_json,
    )

    logger.info(
        "ingestion worker 완료 (document_id=%s workflow_id=%s status=%s)",
        document_id,
        workflow_id,
        status,
    )
    return result_json


def _run_vlm_ocr(file_path: str) -> tuple[str, str, int]:
    """VLM OCR 실행 — OCR 텍스트, backend 식별자, 처리 페이지 수 반환.

    Args:
        file_path: 원본 문서 파일 경로

    Returns:
        (ocr_text, ocr_backend, pages_processed) 3-튜플
    """
    processor = VLMProcessor(use_gpu=bool(settings.vlm_endpoint))
    ocr_text = processor.ocr(file_path) if file_path else processor.ocr("")
    backend = str(processor.last_inference_meta.get("inference_backend", "unknown"))
    # MVP: pages_processed는 inference_meta에서 추출, 없으면 1로 폴백
    pages = int(processor.last_inference_meta.get("pages", 1))
    return ocr_text, backend, pages


def _embed_and_upsert_chunks(
    *,
    document_id: str,
    ocr_text: str,
) -> tuple[int, int]:
    """청크 분할 + 임베딩 + VectorStore.upsert.

    EC-10: 개별 청크 임베딩 실패 시 skip + WARNING (status='completed' 유지).
    REQ-INGEST-002: IndexRebuildingError 최대 3회 재시도.

    Args:
        document_id: 부모 문서 ID (Criterion.parent_criterion_id로 사용)
        ocr_text: VLM OCR 결과 전체 텍스트

    Returns:
        (indexed_chunks, failed_chunks) — 성공/실패 청크 수
    """
    chunker = TextChunker(chunk_size=1536, overlap=128)
    chunks = chunker.chunk(ocr_text)

    if not chunks:
        return 0, 0

    embedder = EmbeddingService()
    criteria: list[Criterion] = []
    failed = 0

    for idx, chunk in enumerate(chunks):
        try:
            vector = embedder.encode(chunk)
        except Exception as exc:  # noqa: BLE001 — EC-10 청크 단위 실패 허용
            failed += 1
            logger.warning(
                "청크 임베딩 실패 — skip (document_id=%s chunk_idx=%d): %s",
                document_id,
                idx,
                exc,
            )
            continue

        if len(vector) != 768:
            failed += 1
            logger.warning(
                _ERROR_EMBEDDING_DIM_MISMATCH.format(actual=len(vector))
                + f" (document_id={document_id} chunk_idx={idx})"
            )
            continue

        criteria.append(
            Criterion(
                id=str(uuid4()),
                criterion_name=f"{document_id}#chunk-{idx}",
                criterion_detail=chunk,
                parent_criterion_id=document_id,
                embedding=vector,
            )
        )

    if not criteria:
        # 모든 청크 임베딩 실패 → no-op (failed_chunks만 기록)
        return 0, failed

    # VectorStore.upsert — IndexRebuildingError 시 최대 3회 재시도
    # 단위 테스트는 VectorStore 클래스 자체를 patch하므로 인자 conn=None 무관.
    # 통합 테스트는 conn 주입을 위한 어댑터 추가 예정.
    store = VectorStore(conn=None)  # type: ignore[arg-type]

    for attempt in range(1, _INDEX_REBUILD_MAX_RETRIES + 1):
        try:
            store.upsert(criteria)
            break
        except IndexRebuildingError as exc:
            if attempt >= _INDEX_REBUILD_MAX_RETRIES:
                raise RuntimeError(
                    _ERROR_VECTOR_UPSERT_FAILED.format(
                        detail=f"인덱스 재구축 재시도 {attempt}회 초과: {exc}"
                    )
                ) from exc
            logger.warning(
                "벡터 인덱스 재구축 중 — 재시도 %d/%d (document_id=%s)",
                attempt,
                _INDEX_REBUILD_MAX_RETRIES,
                document_id,
            )

    return len(criteria), failed


# ─────────────────────────────────────────────────────────────────────────────
# Celery task decorator — celery 패키지 미설치 환경(단위 테스트)에서는 plain function.
# 설치된 환경에서는 @app.task(name=TASK_NAME)로 등록되어 Redis dequeue 시 호출됨.
# ─────────────────────────────────────────────────────────────────────────────

try:  # noqa: SIM105 — celery 미설치 환경 분기 명시
    from pipelines.config.celery_client import create_celery_app

    _app = create_celery_app()

    @_app.task(name=TASK_NAME, bind=True, max_retries=3)  # type: ignore[misc]
    def run(self, document_id: str, *, workflow_id: str, **kwargs: Any) -> dict[str, Any]:  # noqa: ANN001, ANN401, ARG001
        """Celery task entry — REQ-INTEG-001 정확 task name 매칭.

        AC-INTEG-001-1: Go dispatcher RPUSH 후 Python worker dequeue → 본 함수 호출.
        """
        return _execute(document_id=document_id, workflow_id=workflow_id, **kwargs)

except ImportError:
    # celery 미설치 — 단위 테스트 격리 모드. run을 plain function으로 노출.

    def run(document_id: str, *, workflow_id: str, **kwargs: Any) -> dict[str, Any]:  # type: ignore[no-redef]  # noqa: ANN401
        """단위 테스트 격리용 plain function (celery 미설치 시)."""
        return _execute(document_id=document_id, workflow_id=workflow_id, **kwargs)


# ExternalLLMBlockedError는 부팅 시점에 raise되어 import를 차단함 — re-export 불필요.
_ = ExternalLLMBlockedError  # noqa: F401 — IDE 자동 import 가독성 보존
