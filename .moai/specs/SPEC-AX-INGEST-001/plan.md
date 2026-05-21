# Plan: SPEC-AX-INGEST-001

Implementation plan for replacing the `ingestion_worker._execute()` stub with the real
VLM OCR + RAG embedding + Go score trigger pipeline.

**Scope:** Python-only changes within `pipelines/`. Go code untouched ([HARD] 0-diff).
**Methodology:** TDD (project default per quality.yaml).
**Brownfield delta markers:** Required (see Section 5).

---

## 1. 구현 단계 (Implementation Phases)

### Phase A — 사전 검증 및 환경 준비 (Priority: High)

**목표:** 잔여 OPEN 사항 #2, #4, #5, #6 해결 + 의존성 확정 (OPEN #1/#3은 v0.1.1 RESOLVED)

**작업:**
1. ~~**OPEN #3 검증**~~: **RESOLVED v0.1.1** — `pipelines/mapping/embedding_service.py` 존재 확인 완료. `EmbeddingService.encode()` 768-dim 인터페이스 그대로 활용.
2. ~~**OPEN #1 결정**~~: **RESOLVED v0.1.1** — `POST /api/v1/scores` (`apps/control-plane/cmd/server/score_handlers.go:68`, `server.go:282`). Go 0-diff 자연 성립.
3. **OPEN #2 결정**: Python→Go 인증 토큰 전달 방식 후보 3개 중 1개 선택
4. **OPEN #4 결정**: DocumentMetadataClient 데이터 소스 후보 3개 중 1개 선택
5. **OPEN #5/#6 결정**: VLM 타임아웃 위치 및 TextChunker 알고리즘 선택
6. 의존성 추가 검토:
   - `pyproject.toml`에 `transformers`, `torch`, `sentence-transformers`, `pyhwp` 추가 (기존 미존재 시)
   - lock 파일 갱신
7. 모델 파일 준비 확인:
   - `/models/Qwen/Qwen2-VL-7B-Instruct` 디렉터리 존재
   - `/models/hf_cache/jhgan/ko-sroberta-multitask` 디렉터리 존재

**산출물:**
- OPEN 사항 결정 기록 (spec.md §6.2 업데이트)
- 의존성 변경 PR (선행 commit)
- annotation cycle 결과

**우선순위:** High — 모든 후속 Phase의 진입 조건

---

### Phase B — VLMProcessor 클라이언트 구현 (Priority: High)

**목표:** `pipelines/ingestion/vlm_processor.py` 신규 구현, 기존 테스트 컨트랙트 통과

**작업:**
1. **RED**: `tests/unit/test_req_ax_001_vlm_processor.py` 실행하여 현재 실패 상태 확인 (현재 코드는 mock 기반 — 실제 클래스 미존재 가능성)
2. **VLMProcessor 클래스 신규 작성:**
   - `__init__(use_gpu: bool)`: GPU 모드 / CPU 모드 분기
   - `_load_model(use_gpu)`: transformers 또는 vLLM 백엔드 로딩
   - `_run_inference(image_path, model)`: OCR 추론 실행
   - `ocr(image_path: str) -> str`: 공개 API
   - `ocr_with_lock(document_id, image_path)`: 동시성 제어 (RLock)
   - `last_inference_meta` property: dict 반환
3. **HWP 처리 통합:** `pyhwp` 또는 `olefile`로 HWP → 임시 IMAGE 변환 후 `ocr()` 위임
4. **REQ-UBI-001 검증 통합:** `__init__`에서 `validate_llm_endpoint(settings.vlm_endpoint)` 호출
5. **GREEN**: 모든 단위 테스트(8개) 통과

**테스트 전략:**
- `unittest.mock` 사용
- `_load_model` mock으로 transformers/vLLM 의존성 우회
- `_run_inference` mock으로 모델 호출 우회
- 실제 모델 로딩은 통합 테스트(별도 Phase)에서 검증

**산출물:**
- `pipelines/ingestion/vlm_processor.py` (신규, ~150-200 LOC)
- 8개 단위 테스트 GREEN
- @MX:ANCHOR on `VLMProcessor.ocr()` (fan_in >= 3)

**우선순위:** High

---

### Phase C — RAG 보조 모듈 구현 (Priority: High)

**목표:** TextChunker + ScoreTrigger + DocumentMetadataClient 신규 모듈

**작업:**

#### C.1 TextChunker
1. **RED**: `tests/unit/test_ingest_text_chunker.py` 신규 작성
   - 단순 청크 분할 검증
   - 한국어 텍스트 처리 검증
   - 빈 텍스트 / 단일 토큰 엣지 케이스
2. **GREEN**: `pipelines/ingestion/text_chunker.py` 구현
   - OPEN #6 결정 알고리즘 적용
   - 기본 chunk_size=768

#### C.2 ScoreTrigger
1. **RED**: `tests/unit/test_ingest_score_trigger.py` 신규 작성
   - HTTP 2xx → `score_triggered=True`
   - HTTP 503/504 → `score_triggered=False` (no-raise)
   - 네트워크 오류 → `score_triggered=False` (no-raise)
   - 타임아웃 → `score_triggered=False` (no-raise)
2. **GREEN**: `pipelines/ingestion/score_trigger.py` 구현
   - `httpx.post(timeout=5.0)`
   - 엔드포인트: `POST {GO_CONTROL_PLANE_URL}/api/v1/scores` (OPEN #1 RESOLVED v0.1.1)
   - summary 페이로드: `{"pages_processed", "chunk_count", "tokens", "ocr_backend"}` (REQ-INGEST-003)
   - OPEN #2 결정 인증 방식 적용
   - 모든 예외 catch + ERROR 로그 + `score_triggered=False` (REQ-INGEST-003b)

#### C.3 DocumentMetadataClient
1. **RED**: `tests/unit/test_ingest_document_metadata.py` 신규 작성
2. **GREEN**: `pipelines/ingestion/document_metadata.py` 구현
   - OPEN #4 결정 데이터 소스 사용
   - 반환: `{file_type, file_path, user_id}` dict

**산출물:**
- 3개 신규 모듈 (~50-80 LOC each)
- 3개 신규 단위 테스트 파일 (각 4-6 테스트)
- @MX:NOTE on `ScoreTrigger.fire()` (fire-and-forget 의도 표시)

**우선순위:** High

---

### Phase D — _execute() 본체 교체 (Priority: High)

**목표:** `pipelines/workers/ingestion_worker.py:52-82` 스텁을 실제 파이프라인으로 교체

**작업:**
1. **RED**: `tests/unit/test_ingest_execute.py` 신규 작성
   - AC-INGEST-001-1 (골든 패스): PDF 정상 처리
   - AC-INGEST-001-2 (VLM 타임아웃)
   - AC-INGEST-001-4 (score trigger 실패)
   - AC-INGEST-001-5 (HWP 처리)
   - AC-INGEST-001-6 (cold-start)
   - AC-INGEST-001-7 (callback 인터페이스 보존)
   - AC-INGEST-001-8 (한국어 메시지)
2. **GREEN**: `_execute()` 본체 작성
   - Step 1-7 순서대로 구현 (spec.md §3.1)
   - 각 Step 예외 처리 + 한국어 ERROR 메시지
   - result_json 스키마 (REQ-INGEST-004)
3. **REFACTOR**: 함수 분할 (각 Step을 private helper로)

**테스트 전략:**
- 모든 의존 모듈 mock (VLMProcessor, EmbeddingService, VectorStore, ScoreTrigger, DocumentMetadataClient)
- 실제 통합 테스트는 별도 Phase F에서

**산출물:**
- `pipelines/workers/ingestion_worker.py` 수정 (line 52-82 교체)
- 8+ 단위 테스트 GREEN
- @MX:ANCHOR on `_execute()` (fan_in >= 3: run task + 4+ 테스트)
- @MX:NOTE on REQ-UBI-001 검증 호출 지점

**우선순위:** High

---

### Phase E — 부팅 검증 + 타임아웃 통합 (Priority: Medium)

**목표:** REQ-INGEST-005 구현 — worker 시작 시 REQ-UBI-001 검증

**작업:**
1. **RED**: `tests/unit/test_ingest_startup_validation.py` 신규 작성
   - AC-INGEST-001-3 (REQ-UBI-001 강제)
   - 외부 호스트(예: `https://api.openai.com/v1`) 차단 검증
   - localhost 호스트 허용 검증
   - 빈 문자열 → CPU fallback 모드 검증
2. **GREEN**: ingestion_worker 모듈 import 시점 또는 Celery worker init 시점에 `validate_llm_endpoint()` 호출
   - 위치: `pipelines/workers/ingestion_worker.py` 모듈 최상단 또는 `_app.task` 데코레이터 직전
   - INTEG-001 REQ-INTEG-008 패턴과 동형 (settings.py:14-52 재사용)
3. VLM 타임아웃 통합 (OPEN #5 결정 적용):
   - settings.py 또는 환경변수 → VLMProcessor에 주입
   - 단위 테스트: 타임아웃 적용 검증

**산출물:**
- 부팅 검증 로직 (settings.py 또는 worker init 추가)
- 4+ 단위 테스트 GREEN
- @MX:NOTE on REQ-UBI-001 검증 진입점 (settings.py:28-30 기존 ANCHOR 참조)

**우선순위:** Medium (Phase D 완료 후 진행 가능)

---

### Phase F — 통합 테스트 (Priority: Low)

**목표:** docker-compose 기반 end-to-end 통합 테스트

**작업:**
1. `tests/integration/test_ingest_001_pipeline_e2e.py` 신규 작성
2. 실제 PostgreSQL+pgvector 인스턴스 사용
3. mock 최소화 (VLM/Embedding은 가벼운 fake 모델 또는 실제 모델 사용)
4. 시나리오:
   - 작은 PDF 파일(~1페이지) 정상 처리
   - HWP 파일 처리 (테스트 fixture 필요)
   - 채점 트리거 검증 (Go API mock 서버 사용)

**산출물:**
- 1개 통합 테스트 파일 (~200-300 LOC)
- docker-compose.test.yml 업데이트 가능성

**우선순위:** Low (단위 테스트 GREEN 후 진행, CI/CD 통합 시점에 강화)

---

## 2. 의존성 그래프 (Phase Dependencies)

```
Phase A (사전 검증)
   │
   ├── Phase B (VLMProcessor)  ──┐
   │                              │
   ├── Phase C (RAG 보조 모듈) ──┼──► Phase D (_execute 본체)
   │                              │
   └── Phase E (부팅 검증) ─────┘
                                  │
                                  ▼
                              Phase F (통합 테스트)
```

**병렬 실행 가능:** Phase B / C / E는 Phase A 완료 후 독립적으로 진행 가능
**순차 강제:** Phase D는 B/C/E 모두 GREEN 후 진입

---

## 3. 기술적 접근 (Technical Approach)

### 3.1 Mock 전략

**Unit tests:**
- 모든 외부 의존성(transformers, torch, sentence-transformers, pgvector connection, httpx) mock
- `unittest.mock.MagicMock` + `patch` 데코레이터 패턴

**Integration tests:**
- 실제 pgvector 인스턴스 (docker-compose)
- Go callback handler는 mock HTTP 서버 (`pytest-httpserver`)
- VLM 모델은 fixture 텍스트 반환 fake processor

### 3.2 오류 처리 패턴

**Fatal (callback status="failed"):**
- VLMProcessor.ocr() raise → callback status="failed", error="VLM OCR 실패: <상세>"
- VectorStore.upsert() raise → callback status="failed", error="벡터 저장 실패: <상세>"
- DocumentMetadataClient.fetch() raise → callback status="failed"

**Recoverable (log + continue):**
- ScoreTrigger.fire() raise → ERROR 로그, callback `score_triggered=false`, status="completed"
- post_callback() raise → ERROR 로그, Celery ACK 정상 (기존 INTEG-001 패턴)

### 3.3 한국어 메시지 표준

모든 user-facing 오류 메시지는 한국어로 작성 (REQ-UBI-002):

```python
# 예시
ERROR_VLM_TIMEOUT = "VLM OCR 처리 시간 초과 (120초)"
ERROR_VECTOR_REBUILDING = "벡터 인덱스 재구성 중 — 잠시 후 재시도 필요"
ERROR_VLM_EMPTY = "OCR 결과가 비어있습니다 — 문서 형식 확인 필요"
ERROR_DOCUMENT_NOT_FOUND = "문서를 찾을 수 없습니다: {document_id}"
```

---

## 4. MX 태그 계획 (MX Tag Plan)

### 4.1 @MX:ANCHOR (fan_in >= 3)

- **`_execute()`** in `pipelines/workers/ingestion_worker.py`
  - Reason: run() task에서 호출 + 8+ 단위 테스트 + 1+ 통합 테스트
  - 위치: 함수 docstring 위
  - 형식: `# @MX:ANCHOR INGEST-001 _execute is the ingestion pipeline entry — DO NOT inline mock`

- **`VLMProcessor.ocr()`** in `pipelines/ingestion/vlm_processor.py`
  - Reason: _execute() 호출 + 8+ 테스트 + HWP/PDF/IMAGE 3개 경로
  - 형식: `# @MX:ANCHOR INGEST-001 ocr() is the VLM entry point — backend selection branch`

### 4.2 @MX:NOTE (context delivery)

- **REQ-UBI-001 검증 호출 지점** (settings.py:28-30 기존 ANCHOR 참조)
  - 형식: `# @MX:NOTE INGEST-001 REQ-UBI-001 — validate_llm_endpoint must be called before any VLM instantiation`

- **VLM 타임아웃 상수 (`VLM_TIMEOUT_SECONDS = 120`)**
  - 형식: `# @MX:NOTE INGEST-001 VLM timeout default — overridable via env (OPEN #5)`

- **`ScoreTrigger.fire()` fire-and-forget 의도**
  - 형식: `# @MX:NOTE INGEST-001 REQ-INGEST-003 fire-and-forget — never raise; INTEG-001 §4 동형`

- **callback result_json 스키마 변경 지점**
  - 형식: `# @MX:NOTE INGEST-001 REQ-INGEST-004 — replaces INTEG-001 stub schema {"stub": True}`

### 4.3 @MX:WARN (danger zone)

- **VLMProcessor `_load_model()`** (메모리 14GB GPU 사용 가능성)
  - 형식: `# @MX:WARN INGEST-001 Qwen2-VL 7B requires ~14GB GPU or ~16GB CPU RAM`
  - `# @MX:REASON Concurrent processing must be limited to 1 — Celery concurrency=1`

- **HWP 파싱 (OLE2 손상 가능성)**
  - 형식: `# @MX:WARN INGEST-001 HWP files may be malformed OLE2 — wrap in try/except`

### 4.4 @MX:TODO (Phase 별 미완)

- Phase A 진입 전: 잔여 OPEN #2/#4/#5/#6 → `# @MX:TODO INGEST-001 OPEN #<N> — decision pending` (OPEN #1/#3은 v0.1.1 RESOLVED, TODO 불필요)

---

## 5. Delta Markers (Brownfield)

### [DELTA] ingestion_worker

- **[MODIFY]** `pipelines/workers/ingestion_worker.py:52-82` — `_execute()` 본체 스텁 교체
  - 현재: `{"document_id": ..., "stub": True, "spec": "SPEC-AX-INTEG-001"}` 반환
  - 신규: 7-Step 실제 파이프라인 실행 → `{chunks, tokens, score_triggered, ocr_backend, pages_processed, spec}` 반환

- **[EXISTING]** `pipelines/workers/ingestion_worker.py:20-49` (`build_dispatch_payload`) — 무변경
- **[EXISTING]** `pipelines/workers/ingestion_worker.py:91-109` (Celery task decoration + fallback) — 무변경
- **[EXISTING]** `pipelines/callbacks/control_plane.py:30-77` (`post_callback`) — 호출 인터페이스 무변경

- **[NEW]** `pipelines/ingestion/vlm_processor.py` — VLMProcessor 신규 모듈
- **[NEW]** `pipelines/ingestion/text_chunker.py` — TextChunker 신규 모듈
- **[NEW]** `pipelines/ingestion/score_trigger.py` — ScoreTrigger 신규 모듈
- **[NEW]** `pipelines/ingestion/document_metadata.py` — DocumentMetadataClient 신규 모듈

- **[POSSIBLE MODIFY]** `pipelines/config/settings.py` — OPEN #5 결정 시 `vlm_timeout_seconds` 키 추가
  - 다른 필드(`vlm_endpoint`, `validate_llm_endpoint` 등) 무변경

- **[NEW]** `tests/unit/test_ingest_*.py` — 신규 단위 테스트 (5+ 파일)
- **[NEW]** `tests/integration/test_ingest_001_pipeline_e2e.py` — 통합 테스트

### [FROZEN — 0-diff 강제]

- `apps/control-plane/internal/rbac/rbac.go` — Go RBAC 무변경
- `apps/control-plane/internal/auth/**` — 인증 무변경
- `apps/control-plane/migrations/schema/**` — 스키마 무변경
- `go.mod` / `go.sum` — Go 의존성 무변경
- `apps/control-plane/cmd/server/server.go` — 라우트 등록 무변경

---

## 6. 테스트 전략 (Test Strategy)

### 6.1 단위 테스트 커버리지 목표

- **신규 모듈**: 90%+ 라인 커버리지 (TDD 강제)
- **수정 모듈** (`ingestion_worker.py`): 85%+ (기존 + 신규 통합)
- **전체 프로젝트**: 85%+ (TRUST 5 Tested 기준)

### 6.2 테스트 카테고리

| 카테고리 | 파일 | 테스트 수 (예상) | 우선순위 |
|---------|------|-----------------|---------|
| VLMProcessor | test_req_ax_001_vlm_processor.py (기존) | 8 (기존) + 4 (신규) | High |
| VLMProcessor 실제 클래스 | test_ingest_vlm_processor.py | 6 | High |
| TextChunker | test_ingest_text_chunker.py | 5 | High |
| ScoreTrigger | test_ingest_score_trigger.py | 6 | High |
| DocumentMetadataClient | test_ingest_document_metadata.py | 4 | High |
| `_execute()` 본체 | test_ingest_execute.py | 8 | High |
| 부팅 검증 | test_ingest_startup_validation.py | 4 | Medium |
| 통합 (e2e) | test_ingest_001_pipeline_e2e.py | 3 | Low |
| **합계** | — | **48** | — |

### 6.3 CI/CD 통합

- 모든 단위 테스트는 `pytest tests/unit/test_ingest_*.py` 명령으로 실행 가능
- 통합 테스트는 `pytest tests/integration/test_ingest_001_*.py -m integration` (별도 마커)
- 커버리지 리포트는 `pytest --cov=pipelines.ingestion --cov=pipelines.workers.ingestion_worker --cov-report=term-missing`

---

## 7. 완료 정의 (Definition of Done)

- [ ] 9개 REQ 모두 EARS 형식으로 구현 (v0.1.2: 004b/004c 분리 포함; 001/002/003/003b/004/004b/004c/005/005b/005c 중 핵심 9개)
- [ ] 8개 AC 모두 GREEN (단위 + 통합 테스트)
- [ ] 85%+ 라인 커버리지 (Pipelines/ingestion + ingestion_worker)
- [ ] 잔여 OPEN #2/#4/#5/#6 모두 결정 완료 및 spec.md §6.2 업데이트 (OPEN #1/#3은 v0.1.1 RESOLVED)
- [ ] consumer-only [HARD] 0-diff 검증: `git diff --quiet -- apps/control-plane/ go.mod go.sum` PASS
- [ ] TRUST 5 5개 영역 모두 PASS:
  - Tested: ≥85% 커버리지
  - Readable: ruff/black 통과
  - Unified: import 순서 정렬
  - Secured: REQ-UBI-001 부팅 검증 강제 + SCORE_API_TOKEN 로그 마스킹
  - Trackable: 모든 commit이 SPEC-AX-INGEST-001 ID 포함
- [ ] MX 태그 추가: @MX:ANCHOR 2개, @MX:NOTE 4개, @MX:WARN 2개
- [ ] 한국어 오류 메시지 표준 적용 (REQ-UBI-002)
- [ ] 통합 테스트(docker-compose 기반) 1개 이상 GREEN
- [ ] evaluator-active 평가 PASS (≥0.85)

---

## 8. 위험 완화 작업 (Risk Mitigation Tasks)

### ~~Risk-1: OPEN #1 (채점 트리거 엔드포인트) 결정 지연~~ — **RESOLVED v0.1.1**

- 채점 트리거 엔드포인트 `POST /api/v1/scores` 확정 (`apps/control-plane/cmd/server/score_handlers.go:68`, `server.go:282`)
- Go 측 0-diff 자연 성립 — 기존 핸들러 활용

### Risk-2: Qwen2-VL 모델 메모리 부족 (CPU 환경)

- **완화 작업**: Phase B 진입 전 메모리 측정, 부족 시 quantization (4-bit) 옵션 검토
- **fallback**: 더 작은 VLM 모델(Qwen2-VL-2B) 임시 대체

### Risk-3: pyhwp 라이브러리 의존성 추가 거부

- **완화 작업**: Phase A에서 `pyhwp` 대신 `olefile` + 자체 OLE2 파싱 시도
- **fallback**: HWP 지원 일시 보류 → PDF/IMAGE만 지원, FileType.HWP는 callback status="failed"

### ~~Risk-4: EmbeddingService 미구현 (OPEN #3)~~ — **RESOLVED v0.1.1**

- `pipelines/mapping/embedding_service.py` 존재 확인
- `EmbeddingService.encode(text)` 메서드 768-dim (ko-sroberta-multitask)
- 본 SPEC 범위에 추가 구현 불필요

---

## 9. 참고 자료 (References)

- **research.md** — 모든 file:line 참조의 SSOT
- **SPEC-AX-INTEG-001 spec.md §3** — Celery + callback 아키텍처
- **SPEC-AX-PIPE-001** — Python 스택 결정 (FastAPI/Celery 버전)
- **SPEC-AX-SCORE-API-001** — Go 채점 API write-role guard
- **tests/unit/test_req_ax_001_vlm_processor.py** — VLMProcessor 컨트랙트
- **tests/unit/test_req_ax_002_vector_store.py** — VectorStore 사용 패턴
