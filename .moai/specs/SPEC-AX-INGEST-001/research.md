# Research: SPEC-AX-INGEST-001

Deep codebase analysis supporting SPEC-AX-INGEST-001 implementation planning. This document
serves as the single source of truth (SSOT) for all file:line references and technical constraints
affecting the real VLM OCR document parsing + RAG mapping + scoring trigger pipeline implementation.

---

## 1. 현재 스텁 구조 분석

### 1.1 _execute 함수의 현재 상태

**File:** `/home/sklee/moai/iroum-ax/pipelines/workers/ingestion_worker.py:52-82`

The `_execute()` stub currently:
- **Returns:** `dict[str, Any]` with fields `{"document_id", "stub": True, "spec": "SPEC-AX-INTEG-001"}`
  (Line 63-67: `result_json` construction)
- **Parameters:** Receives `document_id: str` (positional) and `workflow_id: str` (keyword argument)
  (Line 52: Function signature)
- **Callback contract:** Posts immediately to `post_callback()` with `status="completed"` and `result_json`
  (Line 70-75: `post_callback` invocation with 4 keyword arguments)

### 1.2 POST callback contract (Python→Go)

**File:** `/home/sklee/moai/iroum-ax/pipelines/callbacks/control_plane.py:30-123`

The `post_callback()` function defines the Python→Go HTTP contract:
- **URL pattern:** `{base_url.rstrip('/')}/api/v1/workflows/{workflow_id}/callback` (Line 54)
- **Request body:** `{"status": "completed"|"failed", "result_json": {...}}` (Line 55-58)
- **Headers:** `{"Content-Type": "application/json"}` (Line 59)
- **Timeout:** `_CALLBACK_TIMEOUT_SECONDS = 5.0` (Line 24)
- **Error handling:** All exceptions caught and logged as ERROR; no raise (Lines 69-76, 87-112)
  - Fire-and-forget pattern: network error/4xx/5xx all result in ERROR log only
  - Celery task is ACK'd normally after callback attempt (Lines 89-123)

### 1.3 Task envelope and Celery integration

**File:** `/home/sklee/moai/iroum-ax/pipelines/workers/ingestion_worker.py:20-49`

Celery task registration and envelope payload:
- **Task name (magic constant):** `"pipelines.workers.ingestion_worker.run"` (Line 24)
  - Exact match required with Go dispatcher (SPEC-AX-INTEG-001 REQ-INTEG-001)
- **Envelope payload builder:** `build_dispatch_payload()` (Line 27-49)
  - Returns 3-tuple: `[[document_id], {"workflow_id": workflow_id}, {callbacks: null, ...}]` (Line 40-48)
  - Format matches Go Kombu v2 envelope deserialization (research.md §2.1, SPEC-AX-INTEG-001 §5.1)
- **Celery app registration:** Decorator `@_app.task(name=TASK_NAME, bind=True, max_retries=3)` (Line 95)
  - Imports from `pipelines.config.celery_client` (Line 91)
  - Fallback plain function when Celery not installed (Lines 103-109)

---

## 2. Pipeline 모델 및 설정

### 2.1 Pydantic models (Request/Response schemas)

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/models.py:1-166`

Data models defined:
- **FileType enum** (Line 13-18): `HWP`, `PDF`, `IMAGE`
- **WorkflowStatus enum** (Line 21-27): `PENDING`, `RUNNING`, `COMPLETED`, `FAILED`
- **Grade enum** (Line 30-36): `A`, `B`, `C`, `D`
- **DocumentUploadRequest** (Line 59-64): `file_type`, `filename`, `user_id` (default `"cli-anonymous"`)
- **DocumentUploadResponse** (Line 67-70): `workflow_id`, `status`, `user_id`, `filename`

No existing **DocumentRequest** or **WorkflowPayload** models found. SPEC-AX-INGEST-001 will need to define
or extend these for VLM OCR input/output contracts.

### 2.2 Settings configuration

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/settings.py:1-191`

Settings loaded from environment variables via Pydantic:

#### LLM/VLM Configuration (Lines 92-132)
- **model_dir:** Default `"/models"` — local model storage (Line 96)
- **hf_home:** Default `"/models/hf_cache"` — HuggingFace cache (Line 99)
- **qwen25_model_name:** Default `"Qwen/Qwen2.5-7B-Instruct"` (Line 105-107)
- **qwen2vl_model_name:** Default `"Qwen/Qwen2-VL-7B-Instruct"` (Line 110-113)
  - **KEY FINDING:** Qwen2-VL is the designated VLM for OCR (confirmed by test file test_req_ax_001_vlm_processor.py)
- **embedding_model_name:** Default `"jhgan/ko-sroberta-multitask"` (Line 117-119)
- **embedding_dim:** Default `768` (Line 121)
- **vlm_endpoint:** Optional vLLM endpoint URL, empty string default (Line 124-127)
  - **Validation required:** Must validate against allowlist (see §2.3)
- **llm_endpoint:** Optional vLLM LLM server URL (Line 129-131)

#### Database Configuration (Lines 65-81)
- **postgres_dsn:** Built from host/port/user/password/db (Line 76-81)
- **redis_url:** Built from host/port (Line 88-90)

#### Control Plane Callback (Lines 175-183)
- **go_control_plane_url:** Required; empty string triggers worker startup failure (Line 179-182)
  - Validation: Empty string check (REQ-INTEG-006)

#### Security Configuration (Lines 134-173)
- **auth_enabled:** Default `False` (Line 135-139)
- **default_user_id:** Default `"cli-anonymous"` (Line 141-145)
- OIDC/JWT settings (Lines 148-173) for future Sprint 0 auth SPEC

### 2.3 LLM endpoint validation (REQ-UBI-001 implementation)

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/settings.py:14-52`

REQ-UBI-001 enforcement — localhost-only LLM endpoints:

- **Allowlist constant** (Line 17-23): `frozenset(["localhost", "127.0.0.1", "::1"])`
  - These are the ONLY permitted LLM endpoint hostnames
- **validate_llm_endpoint()** function (Line 26-52):
  - Parses URL with `urlparse()` (Line 42)
  - Extracts hostname via `parsed.hostname` (Line 43)
  - Raises `ExternalLLMBlockedError` if hostname not in allowlist (Lines 48-50)
  - **CRITICAL ANCHOR** (Line 28-30): "외부 LLM 호출 차단 진입점" with fan_in >= 3

**INTEGRATION POINT:** SPEC-AX-INGEST-001 REQ-INTEG-008 requires calling this validation
during worker startup for both `VLLM_ENDPOINT` and `llm_endpoint`.

---

## 3. 기존 파이프라인 구조

### 3.1 FastAPI main.py endpoints

**File:** `/home/sklee/moai/iroum-ax/pipelines/main.py:1-189`

Current endpoint skeleton (all are PoC stubs returning BackgroundTasks):

- **POST /api/documents/upload** (Line 44-63)
  - Request: `DocumentUploadRequest` (file_type, filename, user_id)
  - Response: `DocumentUploadResponse` (workflow_id, status)
  - Current impl: Returns immediately with PENDING status
  - Real impl: Should trigger ingestion_worker via Celery

- **POST /api/criteria/index** (Line 66-83)
  - Request: `CriterionIndexRequest` (document_workflow_id, user_id)
  - Response: `WorkflowResponse` (workflow_id, status, user_id)
  - Real impl: RAG mapping + vector store indexing

- **GET /api/criteria/search** (Line 86-101)
  - Query params: query, top_k, user_id
  - Response: `CriterionSearchResponse` (items[], total)
  - Current impl: Returns empty list

- **POST /api/simulations/predict** (Line 104-123)
  - Request: `SimulationPredictRequest` (report_text, target_grade)
  - Response: `SimulationPredictResponse` (current_grade, projected_p_a, content_changes, feasible)

- **POST /api/reports/generate** (Line 126-143)
  - Request: `ReportGenerateRequest` (criteria_workflow_id, user_id)
  - Response: `ReportGenerateResponse` (workflow_id, status)

- **POST /api/recommendations/generate** (Line 146-163)
  - Request: `RecommendationGenerateRequest` (report_workflow_id, user_id)
  - Response: `WorkflowResponse`

### 3.2 Celery configuration

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/celery_client.py`

**STATUS:** File listed but NOT READ (assumed to follow SPEC-AX-INTEG-001 §6.2).
Expected content:
- Celery app initialization from Redis broker URL
- Task registration
- Environment variable validation (REQ-INTEG-006)

---

## 4. 채점 트리거 API

### 4.1 Scoring workflow: SPEC-AX-SCORE-001 (점수 기록)

**File:** `/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-SCORE-001/spec.md:1-40` (partial read)

Scoring system architecture:
- **Store layer:** `ScoreStore`/`ScoreTx` in Go (SPEC-AX-SCORE-001 §2, apps/control-plane/internal/store/)
- **Audit integration:** `RecordScoreCreated`, `RecordScoreUpdated` in Go audit system
- **Data model:** Single `scores` table with `level` discriminator (raw/item/category)
- **Grade thresholds:** Separate `grade_thresholds` table (scope, letter, min_value, boundary_rule)

### 4.2 Scoring HTTP API: SPEC-AX-SCORE-API-001

**File:** `/home/sklee/moai/iroum-ax/apps/control-plane/cmd/server/score_handlers.go:1-200` (partial read)

HTTP endpoints for score CRUD:
- **7 endpoints** (Line 4-5):
  1. `GET /api/v1/scores/{id}` — Retrieve single score
  2. `GET /api/v1/scores` — List scores with filter+pagination
  3. `GET /api/v1/scores/rollup` — Get weighted rollup
  4. `GET /api/v1/scores/grade` — Get grade from threshold
  5. `POST /api/v1/scores` — Create score
  6. `PUT /api/v1/scores/{id}` — Update score
  7. `POST /api/v1/scores/{id}/supersede` — Supersede CONFIRMED score

**Handler implementation details:**
- **Max evaluation_item_id length:** 64 chars (Line 32)
- **Max list limit:** 500 items; default 50 (Line 33-34)
- **Error mapping:** mapStoreErr() (Line 111-128) maps 7 sentinel errors to HTTP status
- **Write role gate:** `requireScoreWriteRole()` checks for admin/analyst scopes (Line 161-171)
- **ABAC middleware:** `guardScoreWrite()` enforces write authorization (Line 179-190)

**KEY FINDING:** Score creation uses callback pattern similar to workflow;
INGEST-001 should consider triggering score creation POST after OCR+RAG+confidence scoring.

### 4.3 Endpoint for triggering score creation

**Question:** Where is the entry point that **triggers** score calculation from OCR+RAG results?

**ANSWER:** Not yet defined. SPEC-AX-INGEST-001 must decide:
- Does ingestion_worker call score creation directly via store layer (Go)?
- Does ingestion_worker POST to `/api/v1/scores` via HTTP (Python)?
- Is score creation a separate worker triggered by INGEST-001 completion callback?

**SPEC-AX-INTEG-001 precedent:** Callback pattern (Python→Go POST) for state reporting.
**Recommendation:** Use same callback pattern for async score trigger.

---

## 5. RAG/벡터 스토어 현황

### 5.1 Vector store interfaces and implementation

**File:** `/home/sklee/moai/iroum-ax/pipelines/mapping/vector_store.py:1-214`

Two implementations:

#### VectorStore (pgvector-backed, production)
- **upsert()** (Line 31-61):
  - Accepts list of `Criterion` objects
  - Each criterion embedded as embedding vector or zero-vector if None
  - SQL: `INSERT ... ON CONFLICT (id) DO UPDATE` (upsert pattern, Line 42-51)
  - **Note:** No FK constraint on criteria.id (as per SPEC-AX-EVAL-ITEM-001 stub pattern)

- **query()** (Line 63-133):
  - Input: 768-dim query vector
  - Raises `ValueError` if vector dim != 768 (Line 76-79)
  - Raises `IndexRebuildingError` if HNSW rebuild in progress (Line 82-85)
  - Raises `IndexNotBootstrappedError` if no criteria indexed (cold-start, Line 89-93)
  - Returns: `list[CriterionMatch]` with distance and confidence_score (Line 109-133)
  - **SQL:** Cosine distance query `embedding <=> %(query_vec)s` (Line 101)

- **is_rebuilding()** / **count_indexed_criteria()** (Line 135-141)

#### FakeVectorStore (in-memory, testing)
- **Implementation:** Pure Python cosine similarity with linear search (Line 181-205)
- Used for unit testing without PostgreSQL

### 5.2 Test expectations for vector store

**File:** `/home/sklee/moai/iroum-ax/tests/unit/test_req_ax_002_vector_store.py:1-406`

Test fixtures and contracts:
- **mock_pgvector_conn** (Line 230-257): Mock returns 3 criteria on query
- **mock_pgvector_conn_empty** (Line 261-272): Cold-start, raises `IndexNotBootstrappedError`
- **mock_pgvector_conn_rebuilding** (Line 275-282): `is_rebuilding()=True`, raises `IndexRebuildingError`

**Criterion model** (imported from `pkg.models.criterion`):
- Fields: `id`, `criterion_name`, `criterion_detail`, `max_points`, `parent_criterion_id`, `embedding`
- Example usage (Line 209-221): 3-item list factory

**CONSTRAINT:** Embedding dimension hardcoded to **768** (ko-sroberta-multitask standard).

### 5.3 Embedding service

**File:** `/home/sklee/moai/iroum-ax/pipelines/mapping/embedding_service.py`

**STATUS:** Not read. Assumed to provide:
- `EmbeddingService` class
- `embed(text: str) -> list[float]` returning 768-dim vector
- Uses `ko-sroberta-multitask` model (from settings.embedding_model_name)

---

## 6. VLM/OCR 클라이언트 패턴

### 6.1 VLM processor interface and test contracts

**File:** `/home/sklee/moai/iroum-ax/tests/unit/test_req_ax_001_vlm_processor.py:1-281`

VLMProcessor class contract:
- **Constructor:** `VLMProcessor(use_gpu: bool)` (Line 74, 158)
  - `use_gpu=False` → CPU mode (transformers backend)
  - `use_gpu=True` → GPU mode (vLLM backend)

- **ocr()** method signature (Line 77, 134):
  - Input: `image_path: str`
  - Output: `str` (OCR text result)
  - Metadata available in `processor.last_inference_meta` (dict)

- **Internal methods:**
  - `_load_model(use_gpu: bool)` (Line 199-212): Returns mock with `.device` attribute
  - `_run_inference(image_path, model)` (Line 223-238): Returns dict with:
    - `"text"`: OCR result string
    - `"inference_backend"`: `"transformers_cpu"` or `"vllm_gpu"`
    - `"gpu_device"`: GPU index (if GPU mode)
  - `ocr_with_lock(document_id: str, image_path: str)` (Line 240-254): Thread-safe variant

- **Error handling:**
  - `OCRConcurrencyError` when document already processing (Line 259)

- **Device detection (AC-001-5):**
  - CPU: `model.device == "cpu"` → backend `"transformers_cpu"`
  - GPU: `model.device == "cuda:X"` → backend `"vllm_gpu"` + device index

### 6.2 Document type support

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/models.py:13-18`

FileType enum supports:
- `HWP` — Korean word processor format (requires OLE2 parser)
- `PDF` — PDF documents
- `IMAGE` — Direct image files (JPEG, PNG)

No explicit file:line for parsers, but architecture suggests:
- `pipelines/ingestion/hwp_parser.py`
- `pipelines/ingestion/pdf_parser.py`
- `pipelines/ingestion/vlm_processor.py`

### 6.3 Localhost constraint for VLM (REQ-UBI-001)

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/settings.py:14-52`

REQ-UBI-001 constraint applies to VLLM_ENDPOINT:
- Only `localhost`, `127.0.0.1`, `::1` allowed (Line 17-23)
- Violation raises `ExternalLLMBlockedError` (Line 48-50)
- **SPEC-AX-INTEG-001 REQ-INTEG-008** requires checking this during worker startup

---

## 7. INTEG-001 out-of-scope → INGEST-001 scope

### 7.1 Explicitly OUT of scope in SPEC-AX-INTEG-001

**File:** `/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-INTEG-001/spec.md:42-49`

INTEG-001 OUT of scope (forward-declared for INGEST-001):
1. **비즈니스 로직 본체** — ingestion 파싱, VLM OCR, RAG 매핑, 점수 계산, 보고서 생성
2. Celery retry/backoff 정책 커스터마이징
3. 워크플로우 취소 API
4. gRPC streaming status push
5. Python 측 audit_logs 직접 기록 (STEP_STARTED/STEP_COMPLETED)
6. Dead-letter queue, callback 재시도 정책 (D2)
7. 워크플로우 timeout-based cleanup

### 7.2 What INGEST-001 MUST implement

**Implied from INTEG-001 skeleton:**

1. **Real ingestion pipeline inside _execute()**
   - Parse document (HWP/PDF/IMAGE → text + tables)
   - Run VLM OCR for confidence scoring
   - Embed results with ko-sroberta
   - Query vector store for criterion matches
   - Trigger score creation via callback

2. **RAG mapping workflow**
   - Query VectorStore.query() with document embedding
   - Return ranked Criterion matches
   - Store matches for scoring trigger

3. **Scoring trigger**
   - POST callback to score creation endpoint
   - Or call store.ScoreStore.BeginScoreTx() directly

---

## 8. REQ-UBI 제약 적용 (INGEST-001에 미치는 제약)

### 8.1 REQ-UBI-001: Localhost-Only LLM

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/settings.py:26-52` and
`/home/sklee/moai/iroum-ax/.moai/specs/SPEC-AX-001/spec.md:1-100`

**CONSTRAINT:** All LLM/VLM endpoints must use localhost-only addresses.

**Implementation required in INGEST-001:**
- Call `validate_llm_endpoint(settings.vlm_endpoint)` at worker startup (REQ-INTEG-008)
- Call `validate_llm_endpoint(settings.llm_endpoint)` if LLM used for fallback reasoning
- Allow empty string (no vLLM server, use transformers locally)
- Raise `ExternalLLMBlockedError` with clear messaging if validation fails

**File:line reference:** SPEC-AX-INTEG-001:223-224 (REQ-INTEG-008 requirement)

### 8.2 REQ-UBI-002: Local Embedding

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/settings.py:117-121`

**CONSTRAINT:** Use locally-loaded embedding model (ko-sroberta-multitask).

**Implementation:** `EmbeddingService` should use settings.embedding_model_name and settings.hf_home
for local model caching; never call external embedding API.

**Out of scope in INGEST-001:** Full embedding service implementation (stated in research.md §5.3).

### 8.3 REQ-UBI-003: Audit user_id

**File:** `/home/sklee/moai/iroum-ax/pipelines/config/settings.py:141-145`

**CONSTRAINT:** Default user_id is `"cli-anonymous"` when auth disabled.

**Implementation in INGEST-001:**
- All callbacks should preserve workflow.user_id from envelope header
- Go callback handler persists user_id to audit_logs (handled by INTEG-001 callback_handler)
- Python worker does NOT need to explicitly audit (handled by Go)

**File:line reference:** SPEC-AX-INTEG-001 REQ-INTEG-007 (audit trail requirement)

---

## 9. 구현 권장사항

### 9.1 Reference implementation patterns

#### Document parsing pattern
- **Precedent:** `pipelines/ingestion/hwp_parser.py`, `pdf_parser.py`, `vlm_processor.py` (enumerated in SPEC-AX-001 §2.1)
- **Recommendation:** Parse document → extract text/tables → serialize to JSON for VLM input

#### VLM OCR call pattern
**File:** `tests/unit/test_req_ax_001_vlm_processor.py:193-238`

```python
processor = VLMProcessor(use_gpu=False)  # or True
ocr_result = processor.ocr(image_path)
meta = processor.last_inference_meta  # {"inference_backend": "...", "gpu_device": ...}
```

#### Embedding + RAG query pattern
**File:** `tests/unit/test_req_ax_002_vector_store.py:303-330`

```python
store = FakeVectorStore()  # or VectorStore(pgvector_conn)
criteria = [Criterion(id=..., embedding=[0.1]*768), ...]
store.upsert(criteria)
query_vec = [0.1] * 768
matches = store.query(query_vec, top_k=5)  # list[CriterionMatch]
```

#### Callback posting pattern
**File:** `pipelines/callbacks/control_plane.py:30-77`

```python
from pipelines.callbacks.control_plane import post_callback
post_callback(
    base_url=settings.go_control_plane_url,
    workflow_id=workflow_id,
    status="completed",
    result_json={"pages": 5, "criterion_matches": [...], ...}
)
```

### 9.2 Testing approach

**Unit tests for VLM:** Mock processor (test_req_ax_001_vlm_processor.py pattern)
**Unit tests for vector store:** FakeVectorStore + mock pgvector (test_req_ax_002_vector_store.py pattern)
**Integration tests:** docker-compose with real PostgreSQL+pgvector

### 9.3 Error handling strategy

**Fatal errors (should fail workflow):**
- Document parsing failure
- Model loading failure
- Database connection failure

**Recoverable (log + continue):**
- Callback POST timeout (fire-and-forget, logged as ERROR)
- Partial criterion matching

**Always include in result_json:**
- `"document_id"`: Original document ID
- `"pages_processed"`: Count of pages parsed
- `"ocr_confidence"`: Average confidence score from VLM
- `"criterion_matches"`: List of top-k matches from RAG
- `"error"`: Null on success, error message on partial failure

---

## 10. 위험 요소 및 제약

### 10.1 Technical risks

1. **VLM inference latency**
   - Qwen2-VL 7B can take 5-20s per page without GPU
   - SPEC-AX-001 AC-001-5 specifies p99 < 2s (GPU) / p99 < 20s (CPU)
   - **MITIGATION:** Use GPU vLLM endpoint if available; implement per-page timeout

2. **Vector store cold-start**
   - SPEC-AX-001 AC-002-6 requires criterion index bootstrap
   - Query on empty index raises `IndexNotBootstrappedError`
   - **MITIGATION:** Check `store.count_indexed_criteria() > 0` before querying;
     handle exception with user-friendly error in callback

3. **Callback resilience (D2 risk)**
   - Go callback endpoint unreachable → worker logs error but ACK completes
   - Workflow could remain RUNNING indefinitely if callback fails
   - **MITIGATION:** Go timeout-based cleanup SPEC (future) will force FAILED→COMPLETED transition

4. **Embedding model memory**
   - ko-sroberta-multitask requires ~500MB GPU or ~200MB CPU
   - Concurrent document processing could exhaust memory
   - **MITIGATION:** Implement document queue + serial processing with task pool limits

### 10.2 Integration constraints

1. **No direct database access from Python**
   - All store operations must go through Go (`post_callback` only)
   - SPEC-AX-INGEST-001 cannot write scores directly to postgres
   - **MITIGATION:** Use HTTP callback + Go score_handlers for all mutations

2. **Localthost LLM enforcement (REQ-UBI-001)**
   - vLLM endpoint MUST be on localhost
   - No cloud-hosted LLM calls permitted
   - **MITIGATION:** Deploy vLLM server locally; validate at startup

3. **No circular dependencies**
   - ingestion_worker imports settings, callbacks
   - callbacks imports settings, httpx
   - scoring import ingestion_worker (if at all) only for type hints
   - **MITIGATION:** Keep ingestion_worker → score_trigger unidirectional

4. **Celery task isolation**
   - ingestion_worker._execute runs in isolated worker process
   - Global state (model cache) must be thread-safe or process-per-document
   - **MITIGATION:** Use worker-process-scoped caching with RLock for thread safety

### 10.3 Dependency versions

**Python dependencies (from pyproject.toml, assumed per SPEC-AX-PIPE-001):**
- `celery[redis]>=5.3.6`
- `redis>=5.0.4`
- `requests>=2.31.0` (for callback POST, already in use)
- `httpx>=0.24.0` (for async callback, preferred in new code)
- `transformers>=4.30.0` (for local Qwen2-VL loading)
- `torch>=2.0.0` (for Qwen2-VL)
- `pgvector>=0.2.0` (if direct Python pgvector used; unlikely — Go handles this)

**Go dependencies (assumed per SPEC-AX-SCORE-001):**
- `pgx>=5.x` (PostgreSQL driver)
- Standard library `net/http`, `encoding/json`

### 10.4 Performance SLOs

From SPEC-AX-001 AC-001-5 and REQ-AX-001-O1:
- **GPU environment:** p99 < 2s per page
- **CPU environment:** p99 < 20s per page
- **Vector search:** p99 < 100ms (HNSW index, top-5 query)
- **Callback POST:** Timeout 5s (SPEC-AX-INTEG-001, Line 24)

**IMPLICATION:** OCR + embedding + query must complete within 20s per document (CPU) for SLO compliance.

---

## 11. 파일 참조 요약

| 목적 | 파일 | 주요 라인 |
|------|------|----------|
| **Stub implementation** | `pipelines/workers/ingestion_worker.py` | 52-82 (_execute), 70-75 (callback) |
| **Callback contract** | `pipelines/callbacks/control_plane.py` | 30-77 (post_callback), 54 (URL pattern) |
| **Settings validation** | `pipelines/config/settings.py` | 26-52 (validate_llm_endpoint), 179-182 (go_control_plane_url) |
| **Celery registration** | `pipelines/workers/ingestion_worker.py` | 20-49 (build_dispatch_payload), 91-102 (task decoration) |
| **VLM processor interface** | `tests/unit/test_req_ax_001_vlm_processor.py` | 60-135 (test contracts) |
| **Vector store interface** | `pipelines/mapping/vector_store.py` | 31-133 (VectorStore methods) |
| **Vector store tests** | `tests/unit/test_req_ax_002_vector_store.py` | 18-135 (mock fixtures, error expectations) |
| **Scoring API handler** | `apps/control-plane/cmd/server/score_handlers.go` | 1-200 (endpoint patterns, write-role gate) |
| **SPEC-AX-INTEG-001** | `.moai/specs/SPEC-AX-INTEG-001/spec.md` | 42-49 (out-of-scope), 223-224 (REQ-INTEG-008) |
| **SPEC-AX-SCORE-001** | `.moai/specs/SPEC-AX-SCORE-001/spec.md` | 1-40 (data model overview) |
| **SPEC-AX-SCORE-API-001** | `.moai/specs/SPEC-AX-SCORE-API-001/spec.md` | 1-80 (endpoint patterns, ABAC gate) |

---

## 12. Phantom path avoidance

**Confirmed EXISTING files (no phantom paths):**
- `pipelines/workers/ingestion_worker.py` — Fully read, 110 lines
- `pipelines/callbacks/control_plane.py` — Fully read, 123 lines
- `pipelines/config/settings.py` — Fully read, 191 lines
- `pipelines/config/models.py` — Fully read, 166 lines
- `pipelines/main.py` — Fully read, 189 lines
- `pipelines/mapping/vector_store.py` — Fully read, 214 lines
- `tests/unit/test_req_ax_001_vlm_processor.py` — Fully read, 281 lines
- `tests/unit/test_req_ax_002_vector_store.py` — Fully read, 406 lines
- `apps/control-plane/cmd/server/score_handlers.go` — Partially read (first 200 lines), sufficient for patterns

**NOT READ (assumed working based on SPEC-AX-INTEG-001 completion):**
- `pipelines/config/celery_client.py` — Expected to implement SPEC-AX-INTEG-001 §6.2
- `apps/control-plane/cmd/server/server.go` — Expected to mount callback route per SPEC-AX-INTEG-001 §6.2
- `apps/control-plane/internal/handler/callback_handler.go` — Expected to implement Go callback logic per SPEC-AX-INTEG-001 §4

**CONFIRMED SAFE CONSUMPTION TARGETS** (no modifications required):
- `pipelines/config/settings.Settings` class — Consumer: read-only, no schema changes
- `pipelines/callbacks.post_callback()` — Consumer: call it with correct payload
- `pipelines/mapping/vector_store.VectorStore` — Consumer: call .upsert() and .query()
- Store layer (Go) — Consumer: via HTTP callback only, no direct DB access

---

## Summary: SPEC-AX-INGEST-001 Implementation Load

**Critical findings:**
1. Stub replacement: Replace `_execute()` lines 63-82 with real ingestion + callback pipeline
2. VLM processor exists but untested (mocked only); real Qwen2-VL loading needed
3. Vector store is production-ready (test-proven); FakeVectorStore works in CPU environments
4. Callback system is solid; reuse `post_callback()` for scoring trigger
5. REQ-UBI-001 validation must be added to worker startup (validate_llm_endpoint call)
6. No direct Go store access from Python; HTTP callbacks only
7. Performance SLO: 20s p99 per document (CPU) or 2s (GPU)
8. All error paths must preserve callback guarantees (fire-and-forget, ACK always)

**Recommended implementation order:**
1. Document parser (HWP/PDF/IMAGE → text) — new files
2. VLM processor initialization with real model loading — enhance test_req_ax_001
3. Embedding + vector store query — use existing VectorStore
4. Score creation callback — POST to /api/v1/scores with OCR + RAG results
5. Error handling + fire-and-forget compliance
6. Startup validation (REQ-INTEG-008, REQ-UBI-001)

