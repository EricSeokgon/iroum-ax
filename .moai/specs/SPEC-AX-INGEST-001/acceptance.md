# Acceptance Criteria: SPEC-AX-INGEST-001

Detailed Given-When-Then scenarios for the real ingestion pipeline implementation
(VLM OCR + RAG embedding + Go score trigger).

**관련 SPEC:** SPEC-AX-INGEST-001 v0.1.0
**대상 코드:** `pipelines/ingestion/*`, `pipelines/workers/ingestion_worker.py`
**테스트 위치:** `tests/unit/test_ingest_*.py` + `tests/integration/test_ingest_001_*.py`

---

## AC-INGEST-001-1: PDF 정상 처리 (골든 패스)

**REQ 매핑:** REQ-INGEST-001, REQ-INGEST-002, REQ-INGEST-003, REQ-INGEST-004
**우선순위:** Critical

### Given
- 유효한 `document_id` (UUID 형식) 및 `workflow_id` (UUID 형식)
- DocumentMetadataClient가 `{file_type: "PDF", file_path: "/tmp/sample.pdf", user_id: "cli-anonymous"}` 반환
- VLMProcessor가 mock으로 5페이지 분량 한국어 텍스트 반환 (~3,840 문자)
- VLMProcessor.last_inference_meta = `{"inference_backend": "transformers_cpu", "gpu_device": null}`
- EmbeddingService가 mock으로 768-dim 벡터 반환
- VectorStore.upsert()가 정상 (예외 없음)
- ScoreTrigger.fire()가 HTTP 200 반환
- post_callback이 정상 호출됨

### When
- Celery worker가 `_execute("doc-uuid", workflow_id="wf-uuid")` 호출

### Then
- `VectorStore.upsert()`가 1회 이상 호출됨 (chunks > 0)
- `ScoreTrigger.fire(workflow_id="wf-uuid", document_id="doc-uuid", summary={...})`가 1회 호출됨
- 반환 dict가 다음 스키마 준수:
  ```python
  {
    "document_id": "doc-uuid",
    "chunks": 5,  # > 0
    "tokens": 3840,  # > 0
    "score_triggered": True,
    "ocr_backend": "transformers_cpu",
    "pages_processed": 5,
    "spec": "SPEC-AX-INGEST-001"
  }
  ```
- `post_callback`이 `status="completed"`, `result_json=<위 dict>`로 호출됨
- ERROR 레벨 로그가 0건

---

## AC-INGEST-001-2: VLM 타임아웃 → 실패 경로

**REQ 매핑:** REQ-INGEST-001, REQ-INGEST-004
**우선순위:** High

### Given
- 유효한 document_id, workflow_id
- DocumentMetadataClient 정상 동작
- VLMProcessor.ocr()이 120초 후 `VLMTimeoutError` 또는 `asyncio.TimeoutError` 발생 (mock)

### When
- Celery worker가 `_execute("doc-uuid", workflow_id="wf-uuid")` 호출

### Then
- VectorStore.upsert()가 호출되지 않음 (call_count == 0)
- ScoreTrigger.fire()가 호출되지 않음 (call_count == 0)
- 반환 dict의 `status="failed"`, `result_json`에 다음 포함:
  ```python
  {
    "document_id": "doc-uuid",
    "error": "VLM OCR 처리 시간 초과 (120초)",  # 한국어 메시지
    "spec": "SPEC-AX-INGEST-001"
  }
  ```
- `post_callback`이 `status="failed"`로 호출됨
- Celery 태스크가 정상 ACK (raise 없음)
- ERROR 레벨 로그 1건: "VLM OCR timeout for document doc-uuid"

---

## AC-INGEST-001-3: REQ-UBI-001 강제 — 외부 LLM 차단

**REQ 매핑:** REQ-INGEST-005
**우선순위:** Critical (보안)

### Given
- `settings.vlm_endpoint = "https://api.openai.com/v1/chat/completions"`
- 또는 `settings.vlm_endpoint = "http://192.168.1.100:8000"` (allowlist 외 IP)

### When
- Celery worker가 시작됨 (모듈 import 또는 `_app.task` 데코레이터 평가 시점)
- 또는 `validate_llm_endpoint(settings.vlm_endpoint)` 직접 호출

### Then
- `ExternalLLMBlockedError` 예외 발생
- 예외 메시지에 차단된 호스트명 포함 (한국어 우선)
- Worker process가 시작되지 않음 (FastAPI startup이 차단됨)
- audit_logs에 기록 없음 (시작 자체가 차단되므로)

### 추가 검증 케이스
- `vlm_endpoint = ""` → 예외 없음 (CPU fallback 허용)
- `vlm_endpoint = "http://localhost:8000"` → 통과
- `vlm_endpoint = "http://127.0.0.1:8000"` → 통과
- `vlm_endpoint = "http://[::1]:8000"` → 통과

---

## AC-INGEST-001-4: score trigger 실패 → fire-and-forget

**REQ 매핑:** REQ-INGEST-003, REQ-INGEST-004
**우선순위:** High

### Given
- 유효한 document_id, workflow_id
- VLM OCR 정상 완료 (5 chunks, 3840 tokens)
- VectorStore.upsert() 정상
- ScoreTrigger.fire()가 다음 중 하나 발생:
  - HTTP 503 Service Unavailable
  - HTTP 504 Gateway Timeout
  - `httpx.NetworkError`
  - `httpx.TimeoutException`

### When
- Celery worker가 `_execute("doc-uuid", workflow_id="wf-uuid")` 호출

### Then
- VectorStore.upsert()는 호출되어 chunks 저장됨
- ScoreTrigger.fire() 내부에서 예외가 캐치됨 (no raise)
- 반환 dict의 `status="completed"`, `result_json`에 다음 포함:
  ```python
  {
    "document_id": "doc-uuid",
    "chunks": 5,
    "tokens": 3840,
    "score_triggered": False,  # 실패 표시
    "ocr_backend": "transformers_cpu",
    "pages_processed": 5,
    "spec": "SPEC-AX-INGEST-001"
  }
  ```
- `post_callback`이 `status="completed"`로 호출됨 (실패해도 워크플로우는 완료)
- ERROR 레벨 로그 1건: "Score trigger failed for workflow wf-uuid: <상세>"
- Celery 태스크가 정상 ACK (raise 없음)

---

## AC-INGEST-001-5: HWP 파일 처리

**REQ 매핑:** REQ-INGEST-001
**우선순위:** Medium

### Given
- DocumentMetadataClient가 `{file_type: "HWP", file_path: "/tmp/sample.hwp", user_id: "cli-anonymous"}` 반환
- 유효한 HWP 파일 (OLE2 구조)
- VLMProcessor가 HWP → 임시 IMAGE 변환 후 OCR 수행 (mock)
- 변환 결과: 3페이지 분량 텍스트

### When
- Celery worker가 `_execute("doc-uuid", workflow_id="wf-uuid")` 호출

### Then
- VLMProcessor.ocr()이 IMAGE 경로로 호출됨 (HWP 직접 처리 아님)
- VectorStore.upsert()가 chunks > 0으로 호출됨
- 반환 dict의 `status="completed"`, `result_json["chunks"] > 0`
- `post_callback`이 `status="completed"`로 호출됨

### 추가 검증 케이스 (손상된 HWP)
- 유효하지 않은 OLE2 구조의 HWP 파일 → `HWPParseError`
- 반환 dict의 `status="failed"`, `result_json["error"]="HWP 파일 파싱 실패: <상세>"` (한국어)
- Celery ACK 정상

---

## AC-INGEST-001-6: cold-start 허용 (Vector Store 미부트스트랩)

**REQ 매핑:** REQ-INGEST-002
**우선순위:** Medium

### Given
- 유효한 document_id, workflow_id
- VLM OCR 정상 완료
- VectorStore.count_indexed_criteria()가 0 반환 (cold-start 상태)
- VectorStore.upsert()는 정상 동작 (INSERT ON CONFLICT)
- VectorStore.is_rebuilding()이 False

### When
- Celery worker가 `_execute("doc-uuid", workflow_id="wf-uuid")` 호출

### Then
- upsert()는 정상 호출되어 첫 청크들이 인덱싱됨
- query() 호출은 skip되거나 빈 결과 처리 (cold-start)
- 반환 dict의 `status="completed"`, `result_json["chunks"] > 0`
- `post_callback`이 `status="completed"`로 호출됨

### 추가 검증 케이스 (재구성 중)
- VectorStore.is_rebuilding()이 True (HNSW 재구성 중)
- upsert() 시 `IndexRebuildingError` 발생 → 최대 3회 지수 백오프 재시도 (2s/4s/8s)
- 3회 모두 실패 시 callback `status="failed"`, error="벡터 인덱스 재구성 중 — 잠시 후 재시도 필요"

---

## AC-INGEST-001-7: callback 인터페이스 보존 (INTEG-001 contract)

**REQ 매핑:** REQ-INGEST-004 (consumer-only)
**우선순위:** High (regression 방지)

### Given
- INTEG-001의 `post_callback(base_url, workflow_id, status, result_json)` 시그니처
- 본 SPEC이 result_json 스키마만 변경 (호출 인터페이스 무변경)

### When
- `_execute()` 정상 또는 실패 경로 어느 쪽이든 실행

### Then
- `post_callback`이 정확히 4개 keyword argument로 호출됨:
  - `base_url=settings.go_control_plane_url`
  - `workflow_id="<UUID>"`
  - `status="completed" | "failed"`
  - `result_json={...dict...}`
- 추가 keyword argument 없음
- 호출 횟수 1회 (per `_execute()` 호출)

### 회귀 테스트
- INTEG-001 기존 단위 테스트(`tests/unit/test_integ_001_*.py`) 모두 GREEN 유지
- callback contract 변경 없음

---

## AC-INGEST-001-8: 한국어 메시지 (REQ-UBI-002)

**REQ 매핑:** REQ-UBI-002 (횡단 제약)
**우선순위:** Medium

### Given
- 다양한 실패 경로 시나리오 (VLM 타임아웃, VectorStore 오류, HWP 파싱 실패 등)

### When
- 각 실패 경로에서 `result_json["error"]` 필드 생성

### Then
- 모든 error 메시지가 한국어 문자 (Hangul Unicode U+AC00 ~ U+D7A3) 포함
- 영어 전용 메시지 금지 (단, 기술적 식별자 — `document_id`, `workflow_id`, HTTP 상태 코드 등은 영어 허용)

### 표준 메시지 사전
```python
ERROR_VLM_TIMEOUT = "VLM OCR 처리 시간 초과 ({timeout}초)"
ERROR_VLM_EMPTY = "OCR 결과가 비어있습니다 — 문서 형식 확인 필요"
ERROR_VLM_FAILED = "VLM OCR 실패: {detail}"
ERROR_VECTOR_REBUILDING = "벡터 인덱스 재구성 중 — 잠시 후 재시도 필요"
ERROR_VECTOR_UPSERT_FAILED = "벡터 저장 실패: {detail}"
ERROR_DOCUMENT_NOT_FOUND = "문서를 찾을 수 없습니다: {document_id}"
ERROR_HWP_PARSE_FAILED = "HWP 파일 파싱 실패: {detail}"
ERROR_EMBEDDING_DIM_MISMATCH = "임베딩 차원 불일치 (예상: 768, 실제: {actual})"
```

### 검증 방법
- 각 실패 시나리오 단위 테스트에서 `assert any('가' <= c <= '힣' for c in error_msg)` 확인

---

## Edge Cases (엣지 케이스)

### EC-1: 빈 OCR 결과
**Given**: VLM이 빈 문자열 반환
**Then**: `IngestionEmptyError` → callback `status="failed"`, error="OCR 결과가 비어있습니다"

### EC-2: 매우 큰 PDF (100+ 페이지)
**Given**: 100페이지 PDF
**Then**: 정상 처리, chunks 다수, p99 < 200s/문서 (CPU, 10페이지 PDF 순차 처리) 초과 가능 — timeout 적용

### EC-3: 동시 처리 시 동일 document_id 충돌
**Given**: 동일 document_id에 대한 동시 `_execute()` 호출 2건
**Then**: VLMProcessor.ocr_with_lock()이 `OCRConcurrencyError` 발생 → callback `status="failed"`

### EC-4: 임베딩 차원 mismatch
**Given**: EmbeddingService가 512-dim (768 대신) 반환
**Then**: `ValueError` (VectorStore.upsert 단계) → callback `status="failed"`, error="임베딩 차원 불일치"

### EC-5: workflow_id가 비어있음
**Given**: `_execute("doc-uuid", workflow_id="")`
**Then**: `ValueError("workflow_id is required")` 즉시 발생, callback 호출 안 됨, Celery 태스크 retry (max_retries=3)

### EC-6: settings.go_control_plane_url이 빈 문자열
**Given**: `GO_CONTROL_PLANE_URL=""`
**Then**: worker 시작 시점에 검증 실패 (INTEG-001 REQ-INTEG-006) — 본 SPEC 범위 외

### EC-7: VectorStore 데이터베이스 연결 끊김
**Given**: pgvector connection이 dropped
**Then**: `psycopg.OperationalError` → callback `status="failed"`, error="벡터 저장 실패: 데이터베이스 연결 끊김"

### EC-8: ScoreTrigger HTTP 200이지만 응답 body invalid
**Given**: Go API가 HTTP 200 + 유효하지 않은 JSON 반환
**Then**: `score_triggered=True` 유지 (HTTP 상태만 기준), 응답 파싱 오류는 ERROR 로그만

### EC-9: VLM 모드 자동 선택 — vlm_endpoint 변경 중
**Given**: worker 가동 중 `vlm_endpoint`를 빈 문자열 → localhost URL로 변경
**Then**: 변경 적용 안 됨 (worker 부팅 시점 검증) — 재시작 필요

### EC-10: 부분 chunks 실패 (5개 중 3개만 embedding 성공)
**Given**: EmbeddingService가 일부 청크에서 실패
**Then**: 실패 청크 skip + WARNING 로그 (청크 인덱스 포함) + `failed_chunks=2` (result_json); 성공한 3개 청크만 upsert; callback `status="completed"` (REQ-INGEST-004c)

### EC-11: HWP 파일이지만 OLE2 구조가 새 버전 (HWPX)
**Given**: file_type=HWP인데 실제로 HWPX (XML 기반) 형식
**Then**: VLMProcessor가 형식 감지 실패 → callback `status="failed"`, error="지원하지 않는 HWP 버전"

### EC-12: ko-sroberta 모델 메모리 부족
**Given**: 시스템 메모리 < 500MB
**Then**: `MemoryError` 또는 `OSError` → worker process restart (Celery 자동 처리) — 본 SPEC 범위 외

### EC-13: 동시 ingestion 워커 다수 + GPU 단일
**Given**: Celery `concurrency=4`, GPU 1개
**Then**: 첫 워커만 GPU 점유, 나머지는 OOM 또는 CUDA fallback 실패 → @MX:WARN으로 명시

### EC-14: score trigger 성공 후 Go가 즉시 401 Unauthorized (토큰 만료)
**Given**: SCORE_API_TOKEN 만료
**Then**: `score_triggered=False`, ERROR 로그 — REQ-INGEST-003 fire-and-forget 보장

### EC-15: callback POST 자체가 실패 (INTEG-001 영역)
**Given**: post_callback 실패 (Go callback handler 다운)
**Then**: ERROR 로그만, Celery ACK 정상 — 본 SPEC 영역 외, INTEG-001 기존 동작

### EC-16: 잘못된 file_type (FileType enum 외 값)
**Given**: DocumentMetadataClient가 `{file_type: "XLSX", ...}` 반환 (지원 안 함)
**Then**: `ValueError("Unsupported file type: XLSX")` → callback `status="failed"`, error="지원하지 않는 파일 형식"

---

## Quality Gate Criteria

### Tested (T)
- [ ] 8개 AC 모두 단위 테스트 GREEN
- [ ] 16개 EC(엣지 케이스) 중 최소 10개 단위 테스트 GREEN
- [ ] 1개 통합 테스트(e2e) GREEN
- [ ] 커버리지: 신규 모듈 90%+, 수정 모듈 85%+
- [ ] 회귀 테스트: INTEG-001 기존 테스트 모두 GREEN 유지

### Readable (R)
- [ ] ruff check 통과 (warnings 0)
- [ ] black --check 통과
- [ ] 모든 함수에 docstring (한국어 OK, code_comments=ko 설정)
- [ ] 변수명 영문, 주석 한국어

### Unified (U)
- [ ] isort 통과 (import 순서 표준)
- [ ] 신규 파일이 기존 `pipelines/` 구조 패턴 준수
- [ ] 에러 메시지 한국어 표준 사전 사용

### Secured (S)
- [ ] REQ-UBI-001 부팅 검증 강제 (AC-INGEST-001-3 통과)
- [ ] SCORE_API_TOKEN 로그 마스킹 (`***` 또는 마지막 4자리만)
- [ ] 모든 외부 호출에 timeout 적용 (httpx 5s, VLM 120s)
- [ ] file_path 입력 검증 (path traversal 방지)

### Trackable (T)
- [ ] 모든 commit message에 `SPEC-AX-INGEST-001` ID 포함
- [ ] @MX:ANCHOR 2개 (`_execute`, `VLMProcessor.ocr`)
- [ ] @MX:NOTE 4개 이상 (REQ-UBI-001, VLM timeout, fire-and-forget, schema 변경)
- [ ] @MX:WARN 2개 (Qwen2-VL 메모리, HWP 손상 가능성)

---

## Definition of Done

본 SPEC은 다음 조건 모두 만족 시 완료:

1. **기능 완성**: 5개 REQ 모두 EARS 형식으로 구현 + 8개 AC 모두 단위 테스트 GREEN
2. **품질 통과**: TRUST 5 5개 영역 모두 PASS + 커버리지 85%+
3. **회귀 없음**: INTEG-001 기존 28개 테스트(Go 14 + Python 14) 모두 GREEN 유지
4. **0-diff 검증**: `git diff --quiet -- apps/control-plane/ go.mod go.sum` PASS
5. **OPEN 해결**: spec.md §6.2의 OPEN #1-#6 모두 결정 기록 완료
6. **MX 태그**: ANCHOR 2개 + NOTE 4개 + WARN 2개 추가
7. **evaluator-active**: 최종 평가 ≥0.85 PASS
8. **한국어 표준**: 모든 user-facing error 메시지가 한국어 표준 사전 사용
9. **통합 검증**: docker-compose 기반 e2e 테스트 1건 이상 GREEN
10. **문서 동기화**: README/CHANGELOG/architecture 문서 업데이트 완료 (Sync phase)
