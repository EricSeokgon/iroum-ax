# SPEC-AX-INGEST-001 (Compact)

**Title:** Ingestion Worker 실제 구현 — VLM OCR + RAG 임베딩 + Go 채점 트리거
**Status:** draft | **Version:** 0.1.1 | **Priority:** high
**Parent SPECs:** SPEC-AX-INTEG-001, SPEC-AX-PIPE-001, SPEC-AX-SCORE-API-001

---

## Scope

**IN:** `pipelines/workers/ingestion_worker.py:52-82` 스텁 교체 + 4개 신규 모듈 (vlm_processor, text_chunker, score_trigger, document_metadata)

**OUT:** Go 코드 변경 (consumer-only [HARD] 0-diff), 채점 결과 처리, 외부 LLM API, audit_logs 직접 기록, DLQ/재시도, embedding 본체

---

## REQ Summary (8개 — v0.1.1 EARS 분리)

| ID | Type | 내용 |
|----|------|------|
| REQ-INGEST-001 | Event-driven | Celery `_execute()` 호출 시 VLMProcessor.ocr() 실행, 빈 결과 시 `IngestionEmptyError` |
| REQ-INGEST-002 | Event-driven | OCR 후 청크 분할 → 768-dim 임베딩 → VectorStore.upsert(), `IndexRebuildingError` 최대 3회 재시도 |
| REQ-INGEST-003 | Event-driven | VectorStore 성공 후 `POST /api/v1/scores` 호출 (RESOLVED 엔드포인트), summary 스키마 `{pages_processed, chunk_count, tokens, ocr_backend}` |
| REQ-INGEST-003b | Unwanted | 채점 API 503/504/network 실패 시 no-raise + ERROR 로그 + `score_triggered=false`, `status="completed"` 유지 (fire-and-forget) |
| REQ-INGEST-004 | State-driven | 정상 경로 result_json 스키마: `{document_id, chunks, tokens, score_triggered, ocr_backend, pages_processed, spec}` |
| REQ-INGEST-004b | Unwanted | 부분 실패 시 `error` 필드 + status="failed"; 청크 단위 실패(EC-10)는 skip + WARNING + `failed_chunks` 필드 |
| REQ-INGEST-005 | Ubiquitous | worker 부팅 시 `validate_llm_endpoint(vlm_endpoint)` 호출 강제 (REQ-UBI-001) |
| REQ-INGEST-005b | Unwanted | allowlist 밖 호스트 → `ExternalLLMBlockedError` raise + worker 시작 차단 |
| REQ-INGEST-005c | State-driven | `vlm_endpoint=""` 시 transformers CPU fallback 모드 (외부 호출 0회) |

---

## AC Summary (8개) + Edge Cases (16개)

| AC | 시나리오 | 우선순위 |
|----|---------|---------|
| AC-1 | PDF 골든 패스 (chunks>0, score_triggered=true, status="completed") | Critical |
| AC-2 | VLM 타임아웃 (120s) → status="failed", 한국어 error, Celery ACK | High |
| AC-3 | REQ-UBI-001 강제 — 외부 LLM 호스트 → `ExternalLLMBlockedError` | Critical |
| AC-4 | score trigger 503/504/network → score_triggered=false, status="completed" | High |
| AC-5 | HWP 파일 → VLMProcessor 내부 변환, chunks > 0 | Medium |
| AC-6 | cold-start (count=0) → upsert 진행, query skip | Medium |
| AC-7 | callback contract 보존 — `post_callback(base_url, workflow_id, status, result_json)` 인터페이스 무변경 | High |
| AC-8 | 모든 error 메시지 한국어 (Hangul 포함) | Medium |

---

## 결정 사항 (D)

- **D1**: VLM 모드 자동 — `vlm_endpoint != ""` 시 GPU, 아니면 CPU
- **D2**: 청크 768 토큰 (임베딩 차원 정렬)
- **D3**: 채점 트리거 fire-and-forget (INTEG-001 §4 동형)
- **D4**: HWP는 VLMProcessor 내부에서 OLE2→IMAGE 변환
- **D5**: cold-start 허용 (upsert 진행)
- **D6**: result_json 스키마 7-필드 고정

---

## OPEN 사항 (4개 — v0.1.1에서 #1/#3 RESOLVED)

| # | 항목 | 후보 / 결과 | 해결 시점 |
|---|------|------------|----------|
| ~~1~~ | ~~채점 트리거 엔드포인트~~ | **RESOLVED**: `POST /api/v1/scores` (score_handlers.go:68, server.go:282) — Go 0-diff | v0.1.1 |
| 2 | Python→Go 인증 토큰 | env 토큰 vs envelope 헤더 vs 내부 RPC | Plan 후반 |
| ~~3~~ | ~~EmbeddingService 구현 상태~~ | **RESOLVED**: `pipelines/mapping/embedding_service.py` 존재 (`EmbeddingService.encode()` 768-dim) | v0.1.1 |
| 4 | DocumentMetadataClient 소스 | Go REST vs FS read vs envelope payload | Plan 후반 |
| 5 | VLM 타임아웃 위치 | settings 키 vs env vs 하드코딩 | Plan 중반 |
| 6 | TextChunker 알고리즘 | 토큰 윈도우 vs 문장 경계 vs langchain | Plan 중반 |

---

## Phase 분할

| Phase | 목표 | 의존성 | 우선순위 |
|-------|------|--------|---------|
| A | OPEN 해결 + 의존성 추가 | — | High |
| B | VLMProcessor 신규 구현 (test_req_ax_001 컨트랙트 준수) | A | High |
| C | TextChunker + ScoreTrigger + DocumentMetadataClient 신규 | A | High |
| D | `_execute()` 본체 교체 (7-Step 파이프라인) | B, C | High |
| E | 부팅 검증 + VLM 타임아웃 통합 (REQ-INGEST-005) | A | Medium |
| F | docker-compose e2e 통합 테스트 | D, E | Low |

병렬: B/C/E는 A 완료 후 독립 진행. 순차: D는 B/C/E 모두 GREEN 후 진입.

---

## MX 태그 계획

- **@MX:ANCHOR**: `_execute()`, `VLMProcessor.ocr()`
- **@MX:NOTE**: REQ-UBI-001 검증 지점, VLM 타임아웃 상수, ScoreTrigger fire-and-forget, result_json 스키마 변경
- **@MX:WARN**: Qwen2-VL 14GB 메모리, HWP OLE2 손상 가능성

---

## Delta Markers (Brownfield)

**[MODIFY]:** `pipelines/workers/ingestion_worker.py:52-82` (스텁 교체)
**[POSSIBLE MODIFY]:** `pipelines/config/settings.py` (OPEN #5 결정 시 vlm_timeout_seconds 추가)
**[NEW]:** `pipelines/ingestion/{vlm_processor,text_chunker,score_trigger,document_metadata}.py`
**[NEW]:** `tests/unit/test_ingest_*.py` (5+ 파일, ~48 테스트)
**[NEW]:** `tests/integration/test_ingest_001_pipeline_e2e.py`
**[FROZEN 0-diff]:** `apps/control-plane/**`, `go.mod`, `go.sum`

---

## 핵심 파일 참조 (research.md SSOT)

| 목적 | 파일:라인 |
|------|----------|
| 스텁 교체 대상 | `pipelines/workers/ingestion_worker.py:52-82` |
| callback 인터페이스 | `pipelines/callbacks/control_plane.py:30-77` |
| REQ-UBI-001 검증 | `pipelines/config/settings.py:14-52` (allowlist L17-23) |
| VLMProcessor 컨트랙트 | `tests/unit/test_req_ax_001_vlm_processor.py:60-281` |
| VectorStore 인터페이스 | `pipelines/mapping/vector_store.py:31-141` |
| 채점 API 핸들러 | `apps/control-plane/cmd/server/score_handlers.go:1-200` |
| Celery 태스크 등록 | `pipelines/workers/ingestion_worker.py:20-49` (build_dispatch_payload) |

---

## 의존성

**기존 (변경 없음):** celery[redis]>=5.3.6, httpx>=0.24.0, psycopg[binary]>=3.1
**신규 추가 (Phase A 결정 후):** transformers>=4.30.0, torch>=2.0.0, sentence-transformers>=2.2, pyhwp
**금지:** openai, langchain 외부 LLM 어댑터 (REQ-UBI-001 위반)

**환경변수 신규:** `VLM_ENDPOINT`, `SCORE_API_TOKEN`, `VLM_TIMEOUT_SECONDS`(선택)

---

## DoD (Definition of Done)

1. 8 REQ + 8 AC + 10+ EC GREEN (v0.1.1: 5→8 REQ EARS 분리)
2. 85%+ 커버리지 (ingestion 모듈)
3. consumer-only 0-diff 검증 PASS
4. TRUST 5 5영역 PASS
5. OPEN #2/#4/#5/#6 모두 결정 기록 (#1/#3은 v0.1.1 RESOLVED)
6. MX 태그: ANCHOR 2, NOTE 4+, WARN 2
7. 한국어 메시지 표준 사전 사용
8. evaluator-active ≥0.85
9. e2e 통합 테스트 1+ GREEN
10. 회귀 없음 (INTEG-001 28 tests GREEN 유지)
11. 성능 SLO: p99 < 200s/문서 (CPU 10페이지 PDF) / p99 < 20s/문서 (GPU)

---

## 한국어 에러 메시지 표준 사전

```python
ERROR_VLM_TIMEOUT = "VLM OCR 처리 시간 초과 ({timeout}초)"
ERROR_VLM_EMPTY = "OCR 결과가 비어있습니다 — 문서 형식 확인 필요"
ERROR_VLM_FAILED = "VLM OCR 실패: {detail}"
ERROR_VECTOR_REBUILDING = "벡터 인덱스 재구성 중 — 잠시 후 재시도 필요"
ERROR_VECTOR_UPSERT_FAILED = "벡터 저장 실패: {detail}"
ERROR_DOCUMENT_NOT_FOUND = "문서를 찾을 수 없습니다: {document_id}"
ERROR_HWP_PARSE_FAILED = "HWP 파일 파싱 실패: {detail}"
ERROR_EMBEDDING_DIM_MISMATCH = "임베딩 차원 불일치 (예상: 768, 실제: {actual})"
ERROR_UNSUPPORTED_FILE_TYPE = "지원하지 않는 파일 형식: {file_type}"
```
