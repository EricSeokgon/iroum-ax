---
id: SPEC-AX-INGEST-001
title: Ingestion Worker 실제 구현 — VLM OCR + RAG 임베딩 + Go 채점 트리거
version: "0.1.0"
status: completed
created: "2026-05-21"
updated: "2026-06-01"
author: ircp
priority: high
issue_number: 0
parent_specs: [SPEC-AX-INTEG-001, SPEC-AX-PIPE-001, SPEC-AX-SCORE-API-001]
---

## HISTORY

- 2026-05-21: 초안 작성 (v0.1.0). research.md(2026-05-21) 기반 EARS 5 REQ + 8 AC 정의. SPEC-AX-INTEG-001이 남긴 `_execute()` 스텁(ingestion_worker.py:52-82)을 실제 VLM OCR + RAG 임베딩 + 채점 트리거 파이프라인으로 교체. consumer-only [HARD] 0-diff (rbac/auth/schema/go.mod 무변경) 자연 성립 — Python-only 변경 범위.
- 2026-05-21: v0.1.1 — plan-auditor iter1 FAIL 해소: EARS 혼합 분리(REQ-INGEST-003→003+003b, 004→004+004b, 005→005+005b+005c), OPEN #1/#3 RESOLVED, SLO 모순 수정(30s→200s/문서 CPU), summary 파라미터 스키마 정의, EC-10 청크 실패 처리 명세.
- 2026-06-01: **v0.1.0 구현 완료(completed)** — TDD GREEN 확인. 8개 AC 전체 통과(31 단위 테스트). 신규 모듈: TextChunker, ScoreTrigger, DocumentMetadataClient. `_execute()` 7-Step 파이프라인 본체 교체. coverage 88%. consumer-only 0-diff [HARD] 준수.

---

## 1. 개요 (Overview)

### 1.1 목적 (Purpose)

본 SPEC은 SPEC-AX-INTEG-001이 통합 골격만 보장하고 남겨둔 `ingestion_worker._execute()` 스텁(`pipelines/workers/ingestion_worker.py:52-82`)을 **실제 문서 수집 파이프라인**으로 교체한다. 구현 범위:

- **VLM OCR**: HWP/PDF/IMAGE 문서를 로컬 Qwen2-VL 모델로 텍스트 추출
- **RAG 임베딩**: ko-sroberta-multitask로 청크 임베딩 생성 및 pgvector 업서트
- **Go 채점 트리거**: 임베딩 완료 후 Go 채점 API(`POST /api/v1/scores`)에 fire-and-forget POST
- **결과 callback 강화**: 현재 `{"stub": True}` 구조를 `{"chunks": N, "tokens": N, "score_triggered": bool}` 구조로 교체

### 1.2 범위 (Scope)

**IN scope:**
- `_execute()` 본체 구현 — 문서 메타데이터 획득 → VLM OCR → 청크 분할 → 임베딩 → VectorStore.upsert() → 채점 트리거 POST → callback
- VLM 클라이언트 신규 모듈 (`pipelines/ingestion/vlm_processor.py` 또는 동등 위치) — 기존 테스트 컨트랙트(`tests/unit/test_req_ax_001_vlm_processor.py`) 준수
- VectorStore 래퍼 호출 — 기존 `pipelines/mapping/vector_store.py` 인터페이스 활용 (스키마 변경 없음)
- ScoreTrigger HTTP 클라이언트 신규 모듈 — `httpx`로 `POST /api/v1/scores` 또는 동등 trigger 엔드포인트 호출 (fire-and-forget)
- worker 부팅 시 REQ-UBI-001 검증 강제 (`validate_llm_endpoint(settings.vlm_endpoint)`)
- 타임아웃 / 오류 복원력 / cold-start 대응

**OUT of scope:**
- Go 서버 코드 변경 (consumer-only [HARD])
- 채점 결과 처리 로직 (SPEC-AX-SCORE-001 / SPEC-AX-SCORE-API-001 범위)
- 외부 LLM API 연동 (REQ-UBI-001 위반)
- Celery 태스크 이름 변경 (`pipelines.workers.ingestion_worker.run` 고정 — REQ-INTEG-001 위반)
- 데이터베이스 직접 접근 (Python→Go REST만 허용)
- HWP 파서 자체 구현 (기존 `VLMProcessor.ocr()` 인터페이스를 통한 HWP 처리 — research §6.2)
- 임베딩 서비스 본체 구현 (pipelines/mapping/embedding_service.py 존재 확인 — OPEN #3 RESOLVED; EmbeddingService.encode() 인터페이스 그대로 활용)
- 채점 결과 polling / 재시도 (별도 워크플로우)
- audit_logs 직접 기록 (Go callback handler가 처리 — REQ-INTEG-007)
- 동시성 워커 풀 튜닝 (Celery 기본값 사용)
- Dead-letter queue, callback 재시도 (SPEC-AX-INTEG-001 D2 — out of scope 명시)

### 1.3 선행 SPEC 결정 사항 (Prior Decisions that constrain this SPEC)

| 선행 SPEC | 결정 사항 | 본 SPEC에 미치는 제약 |
|----------|----------|---------------------|
| SPEC-AX-INTEG-001 REQ-INTEG-001 | Celery task name `pipelines.workers.ingestion_worker.run` 고정 | 태스크 이름 변경 금지, 시그니처(`document_id`, `workflow_id`) 유지 |
| SPEC-AX-INTEG-001 REQ-INTEG-003 | callback fire-and-forget (no-raise) | 채점 트리거 실패 시에도 raise 금지, ERROR 로그만 |
| SPEC-AX-INTEG-001 REQ-INTEG-008 | REQ-UBI-001 부팅 검증 | worker 시작 시 `validate_llm_endpoint(vlm_endpoint)` 호출 필수 |
| SPEC-AX-INTEG-001 callback contract | `post_callback(base_url, workflow_id, status, result_json)` 유지 | callback 호출 인터페이스 무변경 (research §1.2) |
| SPEC-AX-PIPE-001 D1/D2 | VLM/LLM toggle gating | `vlm_endpoint=""` 시 transformers CPU fallback 허용 (research §6.1) |
| SPEC-AX-SCORE-API-001 | 채점 API 엔드포인트 `POST /api/v1/scores` (write-role guarded) | Python worker는 trigger만, 결과 처리 불가 |
| 프로젝트 횡단 REQ-UBI-001 | 로컬호스트-온리 LLM (allowlist: `localhost`, `127.0.0.1`, `::1`) | 모든 VLM/LLM 호출 사전 검증 |
| 프로젝트 횡단 REQ-UBI-002 | 한국어 우선 출력 | callback `result_json` 메시지는 한국어로 작성 |
| 프로젝트 횡단 REQ-UBI-003 | audit `cli-anonymous` 기본 user_id | Python은 user_id를 envelope에서 보존, audit 기록은 Go가 처리 |
| consumer-only [HARD] | `rbac.go` / `auth/` / `migrations/schema/` / `go.mod` 0-diff | Python-only scope 자연 성립 |

---

## 2. 배경 및 동기 (Background)

### 2.1 현재 Gap (research.md §1.1, §7)

SPEC-AX-INTEG-001이 완료시킨 `_execute()` 함수의 현재 상태(`pipelines/workers/ingestion_worker.py:52-82`):

```python
def _execute(document_id: str, *, workflow_id: str) -> dict[str, Any]:
    # 현재: 스텁 반환
    return {"document_id": document_id, "stub": True, "spec": "SPEC-AX-INTEG-001"}
```

이 스텁은 통합 골격(Celery dispatch → callback POST → state transition)을 검증할 뿐, 실제 비즈니스 로직(OCR/RAG/채점)은 비어 있다. SPEC-AX-INTEG-001 §1.2 "OUT of scope"에 명시된 4개 항목 중 #1 "비즈니스 로직 본체 — ingestion 파싱, VLM OCR, RAG 매핑"이 본 SPEC의 핵심 범위다.

### 2.2 구현 목표

본 SPEC 완료 시 다음 시나리오가 통과해야 한다:

```
Python worker ← Celery dequeue → _execute(document_id, workflow_id)
  → Step 1: 문서 메타데이터 획득 (file_type, file_path 등)
  → Step 2: VLMProcessor.ocr(file_path) → 텍스트 추출
  → Step 3: 텍스트 청크 분할 (chunk_size = 768 토큰)
  → Step 4: EmbeddingService.embed(chunk) → 768-dim 벡터 (반복)
  → Step 5: VectorStore.upsert(criteria) → pgvector 저장
  → Step 6: ScoreTrigger.fire(workflow_id, document_id) → POST /api/v1/scores (fire-and-forget)
  → Step 7: post_callback(status="completed", result_json={"chunks": N, "tokens": N, "score_triggered": bool, "spec": "SPEC-AX-INGEST-001"})
Go callback ← {status: completed, result_json} → workflow.Complete(result_json) → COMPLETED
사용자 → GET /api/v1/workflows/{id} → {status: COMPLETED, result_json: {chunks: 5, tokens: 3840, score_triggered: true}}
```

### 2.3 비기능 동기

- **REQ-UBI-001 강제**: 외부 LLM API 호출 완전 차단 (KEPCO E&C 보안 요구사항)
- **fire-and-forget 일관성**: 채점 트리거 실패가 ingestion 워크플로우를 차단하지 않음 (REQ-INTEG-003 동형)
- **cold-start 허용**: pgvector 인덱스 미부트스트랩 상태에서도 첫 문서 처리 가능

---

## 3. 아키텍처 설계 (Architecture)

### 3.1 데이터 흐름

```
ingestion_worker.run (Celery task entry, INTEG-001 무변경)
        │
        ▼
ingestion_worker._execute (INGEST-001 신규 구현)
        │
        ├── (1) DocumentMetadataClient.fetch(document_id)
        │       ─→ Go REST GET /api/v1/documents/{id} (또는 파일시스템 직접 read)
        │       ─→ 반환: {file_type, file_path, user_id}
        │
        ├── (2) VLMProcessor.ocr(file_path)
        │       ─→ Qwen2-VL 로컬 모델 (transformers CPU / vLLM GPU 자동 선택)
        │       ─→ REQ-UBI-001 검증 통과 후에만 호출
        │       ─→ 반환: str (OCR 텍스트), processor.last_inference_meta
        │
        ├── (3) TextChunker.split(ocr_text)
        │       ─→ chunk_size 기본 768 토큰
        │       ─→ 반환: list[ChunkDict]
        │
        ├── (4) for chunk in chunks: EmbeddingService.embed(chunk.text)
        │       ─→ ko-sroberta-multitask 로컬 모델
        │       ─→ 반환: list[float] (768-dim)
        │
        ├── (5) VectorStore.upsert([Criterion(...)])
        │       ─→ pgvector INSERT ON CONFLICT (기존 인터페이스, research §5.1)
        │       ─→ FK-less stub 패턴 유지 (SPEC-AX-EVAL-ITEM-001)
        │
        ├── (6) ScoreTrigger.fire(workflow_id, document_id, summary)
        │       ─→ POST /api/v1/scores 또는 동등 trigger 엔드포인트
        │       ─→ httpx.post(timeout=5s)
        │       ─→ 실패 시 ERROR 로그만, raise 금지
        │
        └── (7) return {"document_id": ..., "chunks": N, "tokens": N, "score_triggered": bool, "spec": "SPEC-AX-INGEST-001"}
                ─→ run() 함수가 post_callback()으로 Go에 전송 (INTEG-001 무변경)
```

### 3.2 모듈 구성

**신규 파일:**
- `pipelines/ingestion/vlm_processor.py` (또는 적합 경로) — VLMProcessor 클래스 구현체 (테스트 컨트랙트 준수)
- `pipelines/ingestion/text_chunker.py` — 청크 분할 유틸
- `pipelines/ingestion/score_trigger.py` — Go 채점 API HTTP 클라이언트 (fire-and-forget)
- `pipelines/ingestion/document_metadata.py` — 문서 메타데이터 클라이언트

**수정 파일:**
- `pipelines/workers/ingestion_worker.py` — `_execute()` 본체 교체 (현재 line 52-82)

**무변경 (consumer-only [HARD]):**
- `pipelines/callbacks/control_plane.py` — `post_callback()` 호출 인터페이스 그대로
- `pipelines/config/settings.py` — `validate_llm_endpoint` 호출 추가만 (스키마 무변경)
- `pipelines/mapping/vector_store.py` — `.upsert()` / `.query()` 호출만, 인터페이스 무변경
- `apps/control-plane/**` — Go 전체 무변경 ([HARD] 0-diff)

### 3.3 트랜잭션 / 일관성 모델

- **VectorStore.upsert()**: 자체 트랜잭션 (INSERT ON CONFLICT, 원자적)
- **ScoreTrigger.fire()**: fire-and-forget, ACK 보장 없음 (REQ-INTEG-003 동형)
- **post_callback()**: fire-and-forget, ERROR 로그만 (INTEG-001 무변경)
- **부분 실패 처리**: VLM 성공 + VectorStore 실패 → callback `status="failed"`, `result_json={"error": ...}`

---

## 4. 요구사항 (Requirements — EARS Format)

### REQ-INGEST-001: VLM OCR 문서 파싱 (Event-driven)

**WHEN** Celery worker가 `_execute(document_id, workflow_id)`를 호출하면,
**THE SYSTEM SHALL** 다음을 순서대로 수행한다:

1. `DocumentMetadataClient.fetch(document_id)`로 문서 메타데이터 획득 (file_type, file_path, user_id)
2. `VLMProcessor.ocr(file_path)` 호출하여 OCR 텍스트 추출
3. `processor.last_inference_meta` 로깅 (`inference_backend`, `gpu_device`)
4. OCR 텍스트가 빈 문자열이면 `IngestionEmptyError` 발생

**제약:**
- VLMProcessor 생성자: `VLMProcessor(use_gpu: bool)` — `settings.vlm_endpoint != ""` 시 GPU 모드, 아니면 CPU 모드
- `image_path`: PDF/IMAGE는 직접 전달, HWP는 OLE2 → 임시 IMAGE 변환 후 전달 (VLMProcessor 내부 처리 또는 사전 변환)
- 타임아웃: 기본 120초 (settings로 노출, `vlm_timeout_seconds` 신규 키 또는 환경변수)

**참조:** research.md §6.1 (VLMProcessor 컨트랙트), §6.2 (HWP/PDF/IMAGE 지원)

---

### REQ-INGEST-002: RAG 임베딩 매핑 (Event-driven)

**WHEN** REQ-INGEST-001이 OCR 텍스트를 성공 반환하면,
**THE SYSTEM SHALL** 다음을 수행한다:

1. `TextChunker.split(ocr_text, chunk_size=768)`로 청크 분할
2. 각 청크에 대해 `EmbeddingService.embed(chunk.text)` 호출 → 768-dim 벡터
3. `VectorStore.upsert(criteria)` 호출하여 pgvector에 일괄 저장
4. 청크 수 및 토큰 수 카운트 누적

**제약:**
- 임베딩 차원: 768 (ko-sroberta-multitask 표준, research §5.2)
- `EmbeddingService.embed()`가 768-dim 외 차원 반환 시 `ValueError`
- VectorStore가 `IndexRebuildingError` 발생 시 최대 3회 재시도 (지수 백오프 2s/4s/8s), 그래도 실패 시 callback `status="failed"`
- VectorStore가 `IndexNotBootstrappedError` 발생 시 cold-start 허용 — upsert는 진행, query는 skip

**참조:** research.md §5.1 (VectorStore 인터페이스), §5.2 (Criterion 모델), §10.1 #2 (cold-start 위험)

---

### REQ-INGEST-003: Go 채점 트리거 (Event-driven)

**WHEN** REQ-INGEST-002가 `VectorStore.upsert()`를 성공 반환하면,
**THE SYSTEM SHALL** `ScoreTrigger.fire(workflow_id, document_id, summary)`를 호출하여 Go 채점 API에 trigger POST를 전송한다.

**제약:**
- 엔드포인트: `POST {go_control_plane_url}/api/v1/scores` (RESOLVED — `apps/control-plane/cmd/server/score_handlers.go:68`, `server.go:282`)
- 응답 성공 형식: 201 + `{"score_id": "...", "status": "DRAFT"}`
- 타임아웃: 5초 (callback과 동일)
- 인증: SPEC-AX-SCORE-API-001 write-role guard 통과 필요 — Python worker의 인증 토큰 전달 방식은 §6 OPEN #2
- `score_triggered=true` 조건: HTTP 2xx 응답만 (3xx/4xx/5xx 모두 false)
- `summary` 파라미터 스키마: `{"pages_processed": int, "chunk_count": int, "tokens": int, "ocr_backend": str}`

**참조:** research.md §4.2 (Score handler ABAC gate), §10.2 #1 (HTTP-only)

---

### REQ-INGEST-003b: Go 채점 트리거 실패 처리 (Unwanted Behavior)

**IF** Go 채점 API가 503/504/네트워크 오류 또는 timeout을 반환하면,
**THEN THE SYSTEM SHALL** ERROR 로그를 기록하고 예외를 raise하지 않으며, `result_json["score_triggered"]=false`로 설정한다.

**제약:**
- fire-and-forget 패턴 (INTEG-001 REQ-INTEG-003 동형) — 채점 트리거 실패가 ingestion 워크플로우를 차단하지 않음
- ERROR 로그에 `document_id`, `workflow_id`, HTTP status code (또는 exception type) 포함
- callback `status="completed"` 유지 (score_triggered=false라도 ingestion 자체는 성공)

**참조:** research.md §4.3 (트리거 엔드포인트 fire-and-forget 패턴), SPEC-AX-INTEG-001 §4 (callback no-raise 원칙)

---

### REQ-INGEST-004: callback 결과 구조 정상 경로 (State-driven)

**WHILE** `_execute()`가 정상 실행 경로를 따르는 동안,
**THE SYSTEM SHALL** 다음 스키마의 `result_json`을 반환한다:

```json
{
  "document_id": "<UUID>",
  "chunks": <int, OCR 청크 수>,
  "tokens": <int, 총 임베딩 토큰 수>,
  "score_triggered": <bool, REQ-INGEST-003 성공 여부>,
  "ocr_backend": "<str: 'transformers_cpu' | 'vllm_gpu'>",
  "pages_processed": <int, PDF/HWP 페이지 수>,
  "spec": "SPEC-AX-INGEST-001"
}
```

**제약:**
- 기존 `{"stub": True, "spec": "SPEC-AX-INTEG-001"}` 구조는 완전 폐기
- callback 호출 인터페이스(`post_callback(base_url, workflow_id, status, result_json)`)는 INTEG-001 그대로 (consumer-only)
- 정상 경로에서는 callback `status="completed"`

**참조:** research.md §1.1 (현재 스텁 구조)

---

### REQ-INGEST-004b: 부분 실패 처리 (Unwanted Behavior)

**IF** `_execute()` 실행 중 VLM OCR 또는 VectorStore.upsert() 단계에서 복구 불가능한 오류가 발생하면,
**THEN THE SYSTEM SHALL** `result_json`에 `"error": "<한국어 오류 메시지>"` 필드를 추가하고 callback을 `status="failed"`로 호출한다.

**제약:**
- 모든 오류 메시지는 한국어 우선 (REQ-UBI-002)
- Celery ACK는 항상 정상 반환 (예외 흡수 — fire-and-forget 원칙)

**참조:** research.md §9.3 (error handling strategy)

---

### REQ-INGEST-004c: 청크 단위 임베딩 실패 처리 (Unwanted Behavior)

**IF** 임베딩 단계에서 특정 청크의 임베딩이 실패하면 (EC-10 시나리오),
**THEN THE SYSTEM SHALL** 해당 청크를 skip하고, WARNING 로그를 청크 인덱스와 함께 기록하며, `result_json`에 `"failed_chunks": <int>` 필드를 포함한다.

**제약:**
- 최소 1개 이상 청크 임베딩 성공 시 callback `status="completed"`, 모든 청크 실패 시에만 `status="failed"`
- `failed_chunks` 필드는 0 이상의 정수, 0이면 result_json에서 생략 가능

**참조:** acceptance.md EC-10 (청크 임베딩 부분 실패)

---

### REQ-INGEST-005: 부팅 시 LLM 엔드포인트 검증 강제 (Event-driven)

**WHEN** Celery worker가 시작될 때, **THE SYSTEM SHALL** `validate_llm_endpoint(settings.vlm_endpoint)`를 호출하여 REQ-UBI-001 준수를 강제한다.

**제약:**
- 검증은 Celery worker 부팅 시점에만 수행 (`_execute()` 호출 시점이 아님)
- INTEG-001 REQ-INTEG-008과 동형 패턴 — 동일 함수 `validate_llm_endpoint()` 재사용
- 검증 통과 시 정상 boot, 실패 시 worker 시작 차단 (REQ-INGEST-005b 참조)

**참조:** research.md §2.3 (validate_llm_endpoint), §8.1 (REQ-UBI-001 적용)

---

### REQ-INGEST-005b: 외부 LLM 호스트 차단 (Unwanted Behavior)

**IF** `settings.vlm_endpoint`이 비어 있지 않고 allowlist(`localhost`, `127.0.0.1`, `::1`) 밖의 호스트를 지정하면,
**THEN THE SYSTEM SHALL** `ExternalLLMBlockedError`를 발생시키고 worker를 시작하지 않는다.

**제약:**
- allowlist는 REQ-UBI-001 정의를 준수 (research §2.3, `pipelines/config/settings.py:17-23`)
- 오류 메시지는 한국어로 명확히 표시 (REQ-UBI-002) — 예: `"외부 LLM 엔드포인트 차단됨: {host} — allowlist 외 호스트"`
- 검증 실패 시 Celery worker는 `exit code != 0`으로 종료

**참조:** research.md §2.3 (validate_llm_endpoint), §8.1 (REQ-UBI-001 적용)

---

### REQ-INGEST-005c: CPU Fallback 모드 (State-driven)

**WHILE** `settings.vlm_endpoint`이 빈 문자열인 동안,
**THE SYSTEM SHALL** transformers CPU fallback 모드로 동작한다 (외부 호출 없음).

**제약:**
- `VLMProcessor(use_gpu=False)` 생성자 호출 (research §6.1)
- 모든 OCR 추론은 로컬 CPU에서 수행, 네트워크 호출 0회
- `ocr_backend="transformers_cpu"` 필드를 `result_json`에 기록 (REQ-INGEST-004 스키마)
- INTEG-001 REQ-INTEG-008과 동형 — `vlm_endpoint=""`는 정상 운영 모드 (오류 아님)

**참조:** research.md §6.1 (VLMProcessor CPU/GPU 자동 선택), SPEC-AX-PIPE-001 D1 (VLM toggle gating)

---

## 5. 인수 기준 (Acceptance Criteria)

상세 시나리오는 [`acceptance.md`](./acceptance.md) 참조. 요약:

- **AC-INGEST-001-1** (골든 패스): PDF 정상 처리, chunks > 0, VectorStore.upsert 호출, score trigger POST 성공, callback status="completed"
- **AC-INGEST-001-2** (VLM 타임아웃): VLM 응답 없음 → callback status="failed", error 메시지 포함, Celery ACK 정상
- **AC-INGEST-001-3** (REQ-UBI-001 강제): `vlm_endpoint="https://api.openai.com/v1"` → worker 부팅 실패, `ExternalLLMBlockedError`
- **AC-INGEST-001-4** (score trigger 실패): Go API 503 → ERROR 로그, callback `score_triggered=false`, status="completed" (fire-and-forget)
- **AC-INGEST-001-5** (HWP 처리): HWP 파일 → VLMProcessor가 IMAGE 변환 후 OCR, callback chunks > 0
- **AC-INGEST-001-6** (cold-start): VectorStore.count_indexed_criteria()=0 → upsert 진행, callback 정상 완료
- **AC-INGEST-001-7** (callback 인터페이스 보존): `post_callback()` 시그니처 무변경 (INTEG-001 contract test 통과)
- **AC-INGEST-001-8** (한국어 메시지): 모든 error 메시지가 한국어 (REQ-UBI-002)

---

## 6. 결정 사항 및 OPEN 사항 (Decisions and OPEN Items)

### 6.1 D — 본 SPEC에서 확정된 결정

- **D1**: VLM 모드 자동 선택 — `settings.vlm_endpoint != ""` 시 vLLM GPU, 아니면 transformers CPU (research §6.1 컨트랙트 준수)
- **D2**: 청크 크기 기본 768 토큰 — 임베딩 모델 차원과 일치 (정렬용)
- **D3**: 채점 트리거 fire-and-forget — INTEG-001 REQ-INTEG-003과 동형 패턴
- **D4**: HWP 처리 — VLMProcessor 내부에서 OLE2 → IMAGE 변환 (자체 HWP 파서 구현 금지, research §6.2)
- **D5**: cold-start 허용 — VectorStore.count_indexed_criteria()=0이어도 upsert 진행
- **D6**: 결과 스키마 — `{document_id, chunks, tokens, score_triggered, ocr_backend, pages_processed, spec}` (REQ-INGEST-004 명시)

### 6.2 OPEN — Plan/Run phase에서 해결 필요

- **OPEN #1**: ~~채점 트리거 엔드포인트 정확한 경로~~ → **RESOLVED**: `POST /api/v1/scores` (`apps/control-plane/cmd/server/score_handlers.go:68` `mux.HandleFunc("POST /api/v1/scores", h.handleCreateScore)`, `server.go:282` mount). Go 측 0-diff — 기존 핸들러 활용. 응답: 201 + `{"score_id": "...", "status": "DRAFT"}`. REQ-INGEST-003 본문에 반영 완료.

- **OPEN #2**: **Python worker의 Go API 인증 토큰 전달 방식**
  - SPEC-AX-SCORE-API-001 write-role guard 통과 방법
  - 후보 A: 서비스 계정 토큰 환경변수 (`SCORE_API_TOKEN`)
  - 후보 B: 워크플로우 envelope 헤더에 토큰 포함 (Go dispatcher가 주입)
  - 후보 C: 내부 RPC 채널 (gRPC 인증 우회) — [HARD] 위반 위험
  - **해결 필요 시점**: Plan 후반
  - **결정 기준**: REQ-UBI-003 (audit user_id 보존)과 [HARD] 0-diff 양립

- **OPEN #3**: ~~EmbeddingService 구현 상태~~ → **RESOLVED**: `pipelines/mapping/embedding_service.py` 존재 확인. `EmbeddingService` 클래스, `encode(text)` 메서드, 768-dim (ko-sroberta-multitask). 본 SPEC 추가 구현 불필요 — REQ-INGEST-002에서 기존 인터페이스 그대로 호출.

- **OPEN #4**: **DocumentMetadataClient 데이터 소스**
  - 후보 A: Go REST `GET /api/v1/documents/{id}` (미존재 가능성 — research에 미언급)
  - 후보 B: 파일시스템 직접 read (envelope에서 `file_path` 전달받기)
  - 후보 C: Celery envelope에 메타데이터 전체 포함 (Go dispatcher 수정 필요 — [HARD] 위반 위험)
  - **해결 필요 시점**: Plan 후반
  - **결정 기준**: [HARD] 0-diff 유지

- **OPEN #5**: **VLM 타임아웃 설정 위치**
  - 후보 A: `settings.vlm_timeout_seconds` 신규 키 (settings.py 수정 — REQ-INGEST-005 검증 추가와 함께)
  - 후보 B: 환경변수 직접 read (`VLM_TIMEOUT_SECONDS`)
  - 후보 C: 하드코딩 (120s, ANCHOR 태그)
  - **해결 필요 시점**: Plan 중반

- **OPEN #6**: **TextChunker 알고리즘 선택**
  - 후보 A: 단순 토큰 카운트 슬라이딩 윈도우 (간단)
  - 후보 B: 문장 경계 보존 분할 (한국어 처리 복잡)
  - 후보 C: 외부 라이브러리 (langchain 등 — 의존성 추가 [HARD] 검토)
  - **해결 필요 시점**: Plan 중반

---

## 7. 비기능 요구사항 (Non-Functional Requirements)

### 7.1 성능 (Performance)

- **VLM OCR**: p99 < 2s/페이지 (GPU) 또는 p99 < 20s/페이지 (CPU) — research §10.4
- **임베딩**: p99 < 100ms/청크 (CPU 기준, ko-sroberta-multitask)
- **VectorStore.upsert**: p99 < 500ms/일괄 (배치 100개)
- **채점 트리거 POST**: 5s 타임아웃 (callback과 동일)
- **전체 _execute() 처리**: p99 < 200s/문서 (CPU 환경, 10페이지 PDF 순차 처리 기준; GPU 환경 p99 < 20s/문서)

### 7.2 보안 (Security)

- **REQ-UBI-001**: 외부 LLM API 호출 완전 차단 (부팅 시 검증 강제 — REQ-INGEST-005)
- **REQ-UBI-002**: 모든 오류 메시지 한국어 우선
- **REQ-UBI-003**: audit user_id는 envelope에서 보존, Go가 audit 기록 (Python은 audit 직접 기록 안 함)
- **인증 토큰**: SCORE_API_TOKEN 환경변수는 로그에 절대 노출 금지

### 7.3 가용성 (Availability)

- **VLM 타임아웃 복원력**: 120초 초과 시 callback `status="failed"`, Celery ACK 정상 (워커 무한 hang 방지)
- **VectorStore 재시도**: `IndexRebuildingError` 최대 3회 지수 백오프
- **채점 트리거 fire-and-forget**: Go API 장애 시 ingestion 워크플로우 차단 안 함

### 7.4 관측성 (Observability)

- `_execute()` 진입/종료 시점 INFO 로그 (`document_id`, `workflow_id`, 소요 시간)
- VLM `last_inference_meta` 로깅 (백엔드 모드, GPU device)
- 청크 수 / 토큰 수 / score_triggered 결과 로깅
- 모든 ERROR 로그에 `document_id`, `workflow_id` 포함

---

## 8. 의존성 (Dependencies)

### 8.1 외부 라이브러리 (Python)

**기존 (변경 없음):**
- `celery[redis]>=5.3.6` (INTEG-001)
- `redis>=5.0.4` (INTEG-001)
- `httpx>=0.24.0` (callback에서 이미 사용)
- `psycopg[binary]>=3.1` (pgvector 연결, 기존)

**신규 추가 가능성 (OPEN #6에 따라):**
- `transformers>=4.30.0` (Qwen2-VL 로딩, research §10.3)
- `torch>=2.0.0` (Qwen2-VL 백본)
- `sentence-transformers>=2.2` (ko-sroberta-multitask)
- `pyhwp` (HWP 파서) — VLMProcessor 내부 의존성

**금지:**
- `openai` (REQ-UBI-001 위반)
- `langchain` 외부 LLM 어댑터 (REQ-UBI-001 위반 가능성)

### 8.2 환경변수

**기존 (INTEG-001 유지):**
- `GO_CONTROL_PLANE_URL` (callback POST 대상)
- `REDIS_URL` (Celery broker)
- `POSTGRES_DSN` (pgvector 직접 연결)

**신규:**
- `VLM_ENDPOINT` (vLLM 서버 URL, 빈 문자열 = CPU 모드)
- `SCORE_API_TOKEN` (Go 채점 API 인증, OPEN #2 결정 후 확정)
- `VLM_TIMEOUT_SECONDS` (선택, OPEN #5 결정 후)

### 8.3 외부 시스템

- **Qwen2-VL 모델 파일**: `/models/Qwen/Qwen2-VL-7B-Instruct` (research §2.2)
- **ko-sroberta 모델 파일**: `/models/hf_cache/jhgan/ko-sroberta-multitask`
- **PostgreSQL + pgvector**: 기존 인스턴스 (INTEG-001과 동일)
- **Go Control Plane**: localhost:8080 (callback + 채점 API)

---

## 9. 위험 및 완화 (Risks & Mitigations)

| 위험 | 영향 | 완화 |
|------|------|------|
| VLM 모델 메모리 부족 (Qwen2-VL 7B = ~14GB GPU) | OOM, 워커 크래시 | 동시 처리 1개 제한 (Celery `concurrency=1`), CPU fallback 권장 |
| ko-sroberta 콜드 스타트 지연 | 첫 요청 ~30s 대기 | worker 부팅 시 모델 사전 로딩 (process-scope cache) |
| 채점 트리거 인증 실패 (OPEN #2 미해결) | 모든 ingestion이 score_triggered=false | Plan 후반 OPEN #2 강제 해결 |
| HWP 파싱 실패 (OLE2 손상 파일) | callback status="failed" | error 메시지 명확화, audit_logs에 file_type=HWP 기록 |
| Celery worker 재시작 시 멱등성 | 동일 문서 2회 처리 | VectorStore.upsert ON CONFLICT로 멱등 보장 |
| 임베딩 차원 불일치 | ValueError, 워커 크래시 | EmbeddingService 단위 테스트로 사전 검증 |
| Go API 일시 장애 | 모든 score_triggered=false | fire-and-forget 패턴이 워크플로우 차단 방지 (D3) |

---

## 10. 변경 이력 (Change Log)

- **v0.1.0** (2026-05-21): 초안 작성. 5 REQ + 8 AC. INTEG-001 스텁 교체 범위 명시.
- **v0.1.1** (2026-05-21): plan-auditor iter1 FAIL 해소. EARS 혼합 분리 (REQ 003→003+003b, 004→004+004b, 005→005+005b+005c — 총 5→8 REQ). OPEN #1/#3 RESOLVED. SLO p99 < 30s/문서를 200s/문서(CPU)/20s/문서(GPU)로 수정. summary 파라미터 스키마 정의. EC-10 청크 실패 처리 명세 (failed_chunks 필드).

---

## 11. 참조 (References)

- **Research SSOT**: [`research.md`](./research.md) — 모든 file:line 참조의 원본
- **Plan**: [`plan.md`](./plan.md) — 구현 계획 및 Phase 분할
- **Acceptance**: [`acceptance.md`](./acceptance.md) — Given-When-Then 시나리오
- **부모 SPEC**: SPEC-AX-INTEG-001 (스텁 골격), SPEC-AX-PIPE-001 (Python 스택 결정), SPEC-AX-SCORE-API-001 (채점 API)
- **횡단 제약**: SPEC-AX-001 REQ-UBI-001/002/003
