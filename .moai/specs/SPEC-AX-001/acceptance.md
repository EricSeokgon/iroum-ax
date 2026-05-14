# SPEC-AX-001 Acceptance Criteria — 안전보건 PoC Walking Skeleton

> 본 문서는 `spec.md` EARS 요구사항에 대응하는 Given/When/Then 시나리오와 품질 게이트를 정의한다.
> 각 REQ 모듈은 최소 2개 시나리오(happy + edge)를 포함하며, REQ-UBI 3개 sub-requirement 각각에도 dedicated AC를 부여한다.
> Total: 24개 시나리오 (REQ-UBI 4개, REQ-AX-001 5개, REQ-AX-002 6개, REQ-AX-003 3개, REQ-AX-004 3개, REQ-AX-005 3개) — iteration 3 (evaluator-active cross-validation 후속, +1 신규 시나리오: AC-UBI-004).

- 작성일: 2026-05-14 (iteration 3: 2026-05-14)
- 작성자: ircp
- 대상 SPEC: SPEC-AX-001

---

## 1. Ubiquitous 시나리오

### AC-UBI-001 (Edge: 외부 API 호출 시도 차단)

- **Given**: pipelines/generation/llm_client.py가 환경 변수 `LLM_ENDPOINT`를 통해 추론 엔드포인트를 설정한다.
- **When**: 악의적이거나 잘못된 설정으로 `LLM_ENDPOINT=https://api.openai.com/v1/chat/completions`이 주입되어 LLM 호출이 시도된다.
- **Then**: 시스템은 endpoint allowlist 검증(`pipelines/config/settings.py`)에서 외부 도메인을 거부하고, `ExternalLLMBlockedError`를 발생시키며 audit_logs에 `external_llm_blocked` 이벤트를 기록한다.
- **검증**: REQ-UBI-001 (데이터 주권 invariant)

### AC-UBI-002 (Edge: 한국어 우선 처리 — 혼합 언어 입력)

- **Given**: HWP 문서 1개에 한국어 본문 90% + 영문 약어/표 헤더 10%가 혼재되어 있다 (실제 KEPCO E&C 보고서 패턴).
- **When**: 사용자가 `POST /api/documents/upload`로 해당 문서를 업로드하고 후속 RAG 검색·초안 생성을 수행한다.
- **Then**: (1) `pipelines/ingestion`이 한국어 본문을 정상 파싱하여 documents 레코드에 `language: "ko"`로 기록하고, (2) `pipelines/mapping/embedding_service`가 ko-sroberta-multitask로 한국어 텍스트 임베딩을 생성하며, (3) `pipelines/generation`이 한국어 합니다체로 초안을 생성한다. 영문 약어(예: KOSHA, ISO)는 원형 보존된다. **순수 영문(한국어 0%) 또는 한국어 비율 < 20% 입력에 대해서는** documents 레코드의 `parse_quality_flag` 필드에 `low_korean_ratio`가 기록되고, audit_logs에 `language_warning` 이벤트가 남는다 (거부하지 않음, degraded confidence flag 처리).
- **검증**: REQ-UBI-002 (Korean primary language)

### AC-UBI-003 (Happy: 4종 감사 이벤트 기록 완전성)

- **Given**: PostgreSQL에 audit_logs 테이블이 생성되어 있고, KEPCO E&C 자사 보고서 1개로 PoC E2E 워크플로우를 시작한다.
- **When**: (a) `POST /api/documents/upload`로 문서 업로드, (b) `POST /api/workflows`로 워크플로우 생성, (c) `POST /api/reports/generate`로 초안 생성, (d) `POST /api/simulations/predict`로 등급 예측 — 총 4종 액션이 순차 수행된다.
- **Then**: audit_logs 테이블에 정확히 4개의 신규 레코드가 INSERT 되며, 각 레코드는 `user_id`, `action ∈ {document_upload, workflow_create, draft_generate, prediction}`, `resource_id` (각각 document_id/workflow_id/report_id/simulation_id), `timestamp` (ISO 8601 with timezone) 4개 필드를 모두 포함한다. 누락 필드 0개. 4종 액션이 모두 발생했음에도 레코드 수 < 4이면 본 AC는 실패한다.
- **검증**: REQ-UBI-003 (audit log entry 완전성)

### AC-UBI-004 (Happy: sandbox 환경 user_id 기본값)

- **Given**: SPEC-AX-001 sandbox 환경에서 SSO/JWT 인증 시스템이 비활성화되어 있고 (Exclusion §12), `pipelines/config/settings.py`의 `AUTH_ENABLED=false` 플래그가 적용되며, CLI/API 호출은 식별된 사용자 컨텍스트 없이 수행된다.
- **When**: 사용자가 인증 헤더 없이 `POST /api/documents/upload`, `POST /api/workflows`, `POST /api/reports/generate`, `POST /api/simulations/predict` 4종 액션을 순차 수행한다.
- **Then**: audit_logs 테이블에 INSERT된 모든 신규 레코드의 `user_id` 필드 값은 `'cli-anonymous'` 문자열이며, NULL 값이나 빈 문자열은 허용되지 않는다. `action`, `resource_id`, `timestamp` 필드는 AC-UBI-003과 동일한 완전성 기준을 만족한다. 인증 시스템이 활성화되면(향후 SPEC) 본 기본값은 실제 user_id로 자동 대체되어야 한다.
- **검증**: REQ-UBI-003 (sandbox 환경 user_id 기본값 State-driven 절)

---

## 2. REQ-AX-001 — Document Ingestion

### AC-001-1 (Happy: 정상 HWP 파싱)

- **Given**: KEPCO E&C 참조 안전보건 실적보고서 HWP 1개(50페이지)와 운영 중인 hwp-converter, Qwen2-VL 7B vLLM 엔드포인트.
- **When**: 사용자가 `POST /api/documents/upload`로 HWP 파일을 업로드한다.
- **Then**: 시스템은 30초 이내에 응답하며, documents 테이블에 1개 레코드가 생성되고, parsed_text·tables·metadata(작성자/작성일/부서)가 추출되며, 파싱 정확도가 95% 이상이다.
- **검증**: REQ-AX-001-E1

### AC-001-2 (Edge: HWP OLE 구조 손상 → VLM OCR 폴백)

- **Given**: 헤더 일부가 손상된 HWP 파일 1개(hwp-converter가 OLECompoundError 발생).
- **When**: 동일 업로드 엔드포인트로 업로드한다.
- **Then**: 시스템은 자동으로 VLM OCR 경로(Qwen2-VL 7B)로 폴백하며, documents 레코드의 status 필드가 `ocr_fallback`으로 표시되고, 사용자 개입 없이 처리가 계속된다. 만약 OCR도 실패하면 status `parse_failed`로 기록되고 HTTP 422를 반환한다.
- **검증**: REQ-AX-001-U1, REQ-AX-001-U2

### AC-001-3 (Edge: 회전된 PDF 테이블 — OCR 품질 검증)

- **Given**: 90도 회전된 표 페이지를 포함하는 PDF 파일 1개.
- **When**: PDF 업로드 후 Qwen2-VL이 OCR을 수행한다.
- **Then**: 테이블 셀이 논리적 행/열 순서로 추출되고, table_extractor가 회전 메타데이터(`rotation: 90`)를 기록하며, 셀 인식 정확도가 90% 이상이다.
- **검증**: REQ-AX-001-E1 (OCR 결과 정확도) — 본 AC는 OCR 출력 품질만 검증하며, GPU/CPU 추론 경로 분기는 AC-001-5에서 별도 검증한다.

### AC-001-4 (Edge: 동일 문서 OCR 동시 요청 큐잉)

- **Given**: document_id=`doc-101`에 대한 OCR 작업이 vlm_processor에서 진행 중이며 (`status: ocr_in_progress`), Celery `ingestion_queue`에 작업이 1건 큐잉되어 있다.
- **When**: 사용자가 동일 document_id에 대해 `POST /api/documents/doc-101/ocr/retry` (또는 동일 결과를 유발하는 OCR 재요청 엔드포인트)를 호출한다.
- **Then**: 시스템은 즉시 OCR을 시작하지 않고 (a) 두 번째 요청을 Celery 큐에 enqueue하여 HTTP 202 + `{"status": "queued", "position": 1}`을 반환하거나, (b) 같은 페이지에 대해 중복 OCR을 허용하지 않고 HTTP 409 `{"status": "ocr_already_in_progress"}`를 반환한다. 두 경우 모두 vlm_processor가 동일 페이지에 대해 동시 추론을 실행하지 않는다 (GPU memory race 방지). audit_logs에 `ocr_concurrent_request_queued` 이벤트가 기록된다.
- **검증**: REQ-AX-001-S1 (OCR 진행 중 동시 요청 큐잉 via Celery)

### AC-001-5 (Edge: GPU 가용 vs CPU 폴백 분기)

- **Given**: 동일한 KEPCO E&C 참조 HWP 1개와 두 가지 배포 환경 구성: (env-A) `CUDA_VISIBLE_DEVICES=0` + nvidia-smi가 1매 GPU를 보고, (env-B) `CUDA_VISIBLE_DEVICES=""` (GPU 미가용).
- **When**: 두 환경에서 각각 `POST /api/documents/upload`를 호출하고, vlm_processor가 추론 경로를 결정한다.
- **Then**: (env-A) 시스템은 vLLM-accelerated Qwen2-VL 7B 경로를 선택하여 응답 메타데이터 `{"inference_backend": "vllm_gpu", "gpu_device": 0}`를 기록하며 페이지당 OCR 응답 시간이 p99 < 2초이다. (env-B) 시스템은 CPU 추론 경로 (transformers `device_map: "cpu"` 또는 동등 백엔드)를 선택하여 `{"inference_backend": "transformers_cpu"}`를 기록하며 페이지당 응답 시간이 p99 < 20초 (2초의 5-10배 완화, `tech.md` §6.1 명시 budget) 내에 완료된다. README/운영 문서에 CPU 환경 budget 완화가 명시되어야 한다.
- **검증**: REQ-AX-001-O1 (WHERE GPU available vs unavailable 분기 로직)

---

## 3. REQ-AX-002 — Criterion Mapping

### AC-002-1 (Happy: top-3 검색 정확도)

- **Given**: 안전보건 평가편람 PDF가 인덱싱되어 pgvector HNSW에 청크 단위로 저장되어 있다.
- **When**: 사용자가 `GET /api/criteria/search?q=안전교육 이수율 평가기준`을 호출한다.
- **Then**: 시스템은 100ms 이내(p99)에 top-3 결과를 반환하며, 평균 relevance score가 0.8 이상이고, 항목→지표→배점 계층 정보가 응답에 포함된다.
- **검증**: REQ-AX-002-E1, REQ-AX-002-E2

### AC-002-2 (Edge: 검색 결과 부족)

- **Given**: 인덱싱된 평가편람에 "재해 통계 분석 방법론"에 대한 청크가 존재하지 않는다.
- **When**: 사용자가 해당 키워드로 검색한다.
- **Then**: 시스템은 0.7 이상 relevance 결과가 3개 미만임을 감지하고, status `insufficient_context`와 함께 검색된 후보(0-2개)를 반환하며, 해당 쿼리를 사람 검수 대상으로 플래그한다. 빈 응답을 silently 반환하지 않는다.
- **검증**: REQ-AX-002-U1

### AC-002-3 (Happy: 평가편람 계층 구조 보존)

- **Given**: "안전보건 → 안전교육 → 5점" 구조의 평가편람 leaf criterion 1개.
- **When**: criterion_parser가 PDF를 파싱한다.
- **Then**: criteria 테이블에 parent_criterion_id가 올바르게 연결되며, max_points 필드에 5가 저장되고, 한자/한글 혼용 텍스트는 정규화 후 임베딩된다.
- **검증**: REQ-AX-002-E1 (계층 파싱), R-AX-008 (한자/한글 혼용)

### AC-002-4 (Edge: HNSW 인덱스 재구축 중 검색 큐잉)

- **Given**: pgvector HNSW 인덱스가 신규 평가편람 추가로 재구축 중 (`reindex_status: rebuilding`)이며, 부분 인덱스 상태이다.
- **When**: 사용자가 `GET /api/criteria/search?q=안전교육`을 호출한다.
- **Then**: 시스템은 stale 또는 partial 결과를 반환하지 않는다. (a) 재구축 ETA가 짧으면 (< 10초) 요청을 internal queue에 넣어 재구축 완료 후 처리하고, (b) ETA가 길면 HTTP 503 + `{"status": "index_rebuilding", "retry_after_seconds": <int>}`을 반환한다. 두 경우 모두 부분 인덱스 검색 결과가 응답 본문에 포함되지 않으며, audit_logs에 `retrieval_blocked_during_reindex` 이벤트가 기록된다.
- **검증**: REQ-AX-002-S1 (HNSW 재구축 중 retrieval 차단)

### AC-002-5 (Edge: 한자/한글 정규화 실패 시 graceful degradation)

- **Given**: 평가편람 PDF에 한자 → 한글 변환 사전(`pipelines/mapping/criterion_parser.py`의 hanja→hangul mapping)에 등재되지 않은 희귀 한자(예: 古文書용 한자)가 포함된 leaf criterion 1개.
- **When**: criterion_parser가 정규화 시도 후 임베딩을 수행한다.
- **Then**: 시스템은 (a) 정규화 실패한 토큰을 그대로 보존하여 임베딩에 전달하고 (raw fallback), (b) 해당 chunk의 metadata에 `normalization_warning: "unresolved_hanja", unresolved_chars: ["<해당 한자 list>"]`를 기록하며, (c) 검색 시 해당 chunk의 confidence를 0.8 곱하여 reweight한다. 정규화 실패가 발생해도 임베딩 자체는 fail하지 않으며, audit_logs에 `embedding_normalization_warning`이 기록된다. 시스템 크래시·HTTP 500 발생 시 본 AC는 실패한다.
- **검증**: REQ-AX-002-E1 (실패 경로), R-AX-008 (한자/한글 정규화 실패 모드)

### AC-002-6 (Edge: RAG cold-start — 인덱스 비어 있음)

- **Given**: pgvector criteria 테이블이 비어 있고 (`SELECT COUNT(*) FROM criteria WHERE embedding IS NOT NULL` returns 0), HNSW 인덱스는 생성되었으나 데이터가 0건이다.
- **When**: 사용자가 `GET /api/criteria/search?q=<any>`를 호출한다.
- **Then**: 시스템은 HTTP 503 또는 HTTP 200 + `{"status": "index_not_bootstrapped", "indexed_chunks": 0, "next_step": "POST /api/criteria/index 평가편람 PDF 업로드"}` 응답을 반환한다. HTTP 500 (unhandled exception) 또는 빈 array (`[]`)를 silent 반환하지 않는다. 본 AC는 AC-002-2 (검색 결과 부족)와 구분되며, AC-002-2는 인덱스에 데이터는 있으나 임계값 통과 결과가 부족한 경우를, 본 AC는 인덱스 자체가 비어 있는 cold-start 경우를 검증한다.
- **검증**: REQ-AX-002-U1 (확장 — bootstrap state), R-AX-003 (RAG 콜드스타트 위험 대응)

---

## 4. REQ-AX-003 — Grade Simulation

### AC-003-1 (Happy: A vs B 확률 분포 산출)

- **Given**: A 등급 벤치마크 보고서 1개와 B 등급 벤치마크 보고서 1개로 훈련된 grade_predictor 모델, 그리고 KEPCO E&C 자사 보고서 1개.
- **When**: 자사 보고서에 대해 `POST /api/simulations/predict`를 호출한다.
- **Then**: 시스템은 1초 이내에 응답하며, probability 분포가 `{A: p_a, B: p_b}`이고 `|p_a + p_b - 1.0| < 0.001`, 가장 높은 확률의 등급이 prediction 필드에 기록되고, simulations 테이블에 저장된다.
- **검증**: REQ-AX-003-E1

### AC-003-2 (Edge: 낮은 신뢰도 — 두 확률 모두 0.5 미만)

- **Given**: 자사 보고서의 특징이 A·B 어느 등급과도 명확히 일치하지 않아 분류기 출력이 평탄(flat distribution)한 경계 케이스.
- **When**: grade_predictor가 추론하여 `{A: 0.42, B: 0.45, abstain: 0.13}` (또는 동등하게 sum=1.0 ± 0.001을 만족하는 분포로 max(p_a, p_b) < 0.5)를 산출한다.
- **Then**: 시스템은 REQ-AX-003-U1 조건 (both A and B below 0.5)을 만족함을 감지하여 status `low_confidence`를 반환하고, response body에 `{prediction: null, candidates: {A: 0.42, B: 0.45}, reason: "both_below_0.5", recommend_human_review: true}`를 포함하며, simulations 테이블에 prediction=NULL + status=low_confidence로 저장한다. 자동 downstream 소비(예: gap recommendation)는 차단된다.
- **검증**: REQ-AX-003-U1 (predicted probability for both A and B grades below 0.5 — strict interpretation)
- **Note**: 0.55 boundary와 같은 추가 threshold는 본 SPEC에 도입하지 않는다. iteration 1에서 등장한 `{A: 0.48, B: 0.52}` 시나리오는 max > 0.5이므로 REQ-AX-003-U1의 trigger 조건이 아니며, 이는 SPEC-AX-002에서 "near-tie majority" 정책으로 별도 도입 후보이다 (D6 reconcile: Option B 채택).

### AC-003-3 (Edge: 학습 중 예측 차단)

- **Given**: benchmark_learner가 신규 벤치마크 보고서로 재훈련 중인 상태(`state: training`).
- **When**: 사용자가 prediction 엔드포인트를 호출한다.
- **Then**: 시스템은 HTTP 503과 `model_training` 상태를 반환하고, 이전 모델의 stale prediction을 제공하지 않는다.
- **검증**: REQ-AX-003-S1

---

## 5. REQ-AX-004 — Report Draft Generation

### AC-004-1 (Happy: 평가지표 1개 초안 생성)

- **Given**: 안전교육 이수율 평가지표에 대한 retrieved context 5개 청크, EXAONE 3.5 7B vLLM 엔드포인트 가용.
- **When**: `POST /api/reports/generate`로 해당 지표의 초안을 요청한다.
- **Then**: 시스템은 5초 이내에 응답하며, 출력은 합니다체로 작성되고(예: "안전교육을 실시하였습니다"), style_applier 검증을 통과하고, reports 테이블에 저장된다. 주관 평가에서 4/5 이상을 받는다.
- **검증**: REQ-AX-004-E1

### AC-004-2 (Edge: 공문 스타일 위반 → 재생성 루프)

- **Given**: LLM이 1차 응답으로 합니다체와 해체를 혼용한 출력을 생성한다(예: "실시했어요", "수행했습니다" 혼재).
- **When**: style_applier가 honorific 일관성 검증을 수행한다.
- **Then**: 시스템은 초안을 폐기하고 style reinforcement 프롬프트로 재생성을 시도한다. 최대 3회 재시도 후에도 통과 못 하면 status `style_violation`을 반환한다.
- **검증**: REQ-AX-004-U1, R-AX-006

### AC-004-3 (Edge: EXAONE 미가용 → Qwen 2.5 fallback)

- **Given**: EXAONE 3.5 vLLM 엔드포인트가 3회 연속 timeout 또는 5xx 응답을 반환한다.
- **When**: llm_client가 generate()를 시도한다.
- **Then**: 시스템은 자동으로 Qwen 2.5 7B fallback endpoint로 전환하고, 동일 프롬프트 템플릿으로 초안을 생성하며, response metadata에 `model_used: "qwen2.5-7b"`를 기록한다. 외부 API(OpenAI 등)로 escalate하지 않는다.
- **검증**: REQ-AX-004-O1, REQ-AX-004-U2, R-AX-004

---

## 6. REQ-AX-005 — Gap Recommendation

### AC-005-1 (Happy: B→A 3-5개 우선순위 제안)

- **Given**: 자사 워크플로우가 현재 B 등급으로 예측되고, 타겟 등급이 A로 지정되며, A 등급 벤치마크 콘텐츠가 인덱싱되어 있다.
- **When**: `POST /api/recommendations/generate`를 호출한다.
- **Then**: 시스템은 3초 이내에 응답하며, 3-5개의 prioritized recommendation 항목을 반환한다. 각 항목은 `{content, expected_score_delta, feasibility_score, source_benchmark_id}` 구조를 가지며 feasibility_score 내림차순으로 정렬된다.
- **검증**: REQ-AX-005-E1

### AC-005-2 (Edge: 벤치마크 데이터 부족)

- **Given**: A 등급 벤치마크 콘텐츠가 안전교육 지표에 대해 인덱싱되어 있지 않다.
- **When**: 동일 엔드포인트를 호출한다.
- **Then**: 시스템은 빈 recommendation 리스트와 status `benchmark_not_available`을 반환하며, 추측에 의한 fabricated 콘텐츠를 생성하지 않는다.
- **검증**: REQ-AX-005-S1

### AC-005-3 (Edge: 사용자가 "not feasible"로 표시)

- **Given**: 이전에 반환된 recommendation 항목 ID `rec-123`에 대해 사용자가 "not_feasible" 피드백을 등록한다.
- **When**: `POST /api/recommendations/rec-123/feedback`을 호출하고, 이후 동일 워크플로우의 후속 추천 호출이 발생한다.
- **Then**: 시스템은 해당 항목의 우선순위를 내리고, 가능 시 alternative content item을 제안하며, recommendations 테이블의 feedback 필드에 사용자 입력을 보존한다.
- **검증**: REQ-AX-005-U1

---

## 7. 성능 기준 (Performance Criteria)

`spec.md` §4 비기능 요구사항을 acceptance 단계에서 측정 가능한 형태로 재정의.

| 기준 | 측정 도구 | 목표 | 측정 위치 |
|------|-----------|------|-----------|
| OCR 페이지당 처리 시간 | `iroum_ax_document_processing_duration_seconds` (Prometheus histogram) | p99 < 2초 (Qwen2-VL 7B 단일 GPU) | `pipelines/ingestion/vlm_processor.py` |
| RAG 검색 응답 시간 | `iroum_ax_api_request_latency_seconds{endpoint="/api/criteria/search"}` | p99 < 100ms | `pipelines/mapping/retriever.py` |
| 초안 생성 응답 시간 | `iroum_ax_vllm_inference_latency_seconds{model="exaone-3.5-7b"}` | p99 < 5초 | `pipelines/generation/llm_client.py` |
| 등급 예측 응답 시간 | `iroum_ax_api_request_latency_seconds{endpoint="/api/simulations/predict"}` | p99 < 1초 | `pipelines/scoring/grade_predictor.py` |

GPU 미가용 환경(CPU 추론)에서는 본 기준이 5-10배 완화되며 README에 명시한다 (AC-001-5에서 검증).

### 7.1 Korean-specific 품질 기준 (D14 보강)

본 SPEC은 한국어 공공기관 보고서 도메인에 특화되므로 일반 NFR과 별도로 한국어 edge case 품질 기준을 명시한다.

| 기준 | 측정 방법 | 목표 | 관련 AC |
|------|----------|------|---------|
| HWP OLE 손상 복원율 | 손상 HWP 10건 × 자동 VLM 폴백 성공 비율 | ≥ 80% | AC-001-2 |
| 공문 합니다체 일관성 | style_applier 통과율 (재시도 후 최종) | ≥ 95% | AC-004-1, AC-004-2 |
| 한자/한글 혼용 정규화율 | 평가편람 한자 토큰 자동 변환 성공 비율 | ≥ 90% | AC-002-3 |
| 한자 정규화 실패 graceful 처리 | 미지 한자 발생 시 시스템 크래시 0건 | 100% (no 500) | AC-002-5 |
| 한국어 비율 < 20% 입력 경고 | parse_quality_flag 기록률 | 100% (모든 해당 케이스) | AC-UBI-002 |
| RAG cold-start 응답 명확성 | 빈 인덱스 검색 시 명시적 status 반환 | 100% (no silent empty / no 500) | AC-002-6 |

---

## 8. 품질 게이트 (Quality Gates)

본 SPEC의 Definition of Done 통과 기준.

### 8.1 기능 품질

- 문서 파싱 정확도 ≥ 95% (KEPCO E&C 참조 HWP 기준)
- RAG top-3 평균 relevance ≥ 0.8 (안전보건 쿼리 셋 10개 기준)
- 등급 예측 일치율 ≥ 80% (사람 검수 벤치마크 대비, A/B 2-class)
- 공문 스타일 주관 평가 ≥ 4/5 (KEPCO E&C 담당자 5명 이상)
- Recommendation 실현율 ≥ 70% (사용자 피드백 기반, PoC 운영 중 측정)

### 8.2 코드 품질 (`quality.yaml` 기반)

- 단위 + 통합 테스트 커버리지 ≥ 85% (`test_coverage_target`)
- TDD 사이클 준수: 모든 commit에서 test_first_required 위반 없음
- LSP zero-error: `max_errors=0`, `max_type_errors=0`, `max_lint_errors=0` (Run phase)
- ast-grep 게이트: `ast_grep_gate.block_on_error=true`, 에러 매치 0개
- TRUST 5 모든 차원 통과 (Tested, Readable, Unified, Secured, Trackable)

### 8.3 보안 및 컴플라이언스

- 외부 LLM API 호출 0건 (AC-UBI-001로 검증)
- PII 기본 마스킹(`tech.md` §9.2) 적용 — 전화번호, 한글 인명 2-4자
- audit_logs에 문서 업로드, 워크플로우 생성, 초안 생성, 예측 이벤트 100% 기록

### 8.4 운영성

- Prometheus 메트릭 4종(`tech.md` §11.1) 모두 노출
- `iroum-ax` Helm 차트 dev values로 sandbox 단일 노드 배포 성공
- README에 빠른 시작 (docker-compose up) 시나리오 검증

---

## 9. Definition of Done (DoD)

다음 모든 조건이 충족되면 SPEC-AX-001을 완료로 표시한다.

- [ ] §1-6 모든 acceptance 시나리오(24개)에 대응하는 자동화된 테스트가 통과 (REQ-UBI 4 + REQ-AX-001 5 + REQ-AX-002 6 + REQ-AX-003 3 + REQ-AX-004 3 + REQ-AX-005 3)
- [ ] §7 성능 기준 4종 + §7.1 Korean-specific 품질 기준 6종이 sandbox 환경에서 측정·기록됨 (CPU 추론 환경에서는 완화 기준 명시)
- [ ] §8.1 기능 품질 기준 5종이 모두 통과 (사용자 만족도 항목 포함)
- [ ] §8.2 코드 품질 게이트 모두 통과 (커버리지·TDD·LSP·ast-grep·TRUST 5)
- [ ] §8.3 보안 컴플라이언스 항목 검증
- [ ] §8.4 운영성 항목 검증 (Prometheus, Helm, README)
- [ ] E2E 테스트 `tests/e2e/test_document_to_report.py` 통과
- [ ] KEPCO E&C 담당자 데모 1회 완료 및 피드백 수렴
- [ ] @MX:TODO 태그 잔존 0건 (모든 RED→GREEN 사이클 완료)
- [ ] @MX:ANCHOR 태그가 plan.md §5.1 함수들에 부착됨
- [ ] @MX:WARN 태그가 plan.md §5.2 위험 패턴에 @MX:REASON과 함께 부착됨
- [ ] `.moai/specs/SPEC-AX-001/spec-compact.md` Run phase 로딩용 압축본 존재

---

## 10. 검수 절차 (Review Procedure)

1. **자동 검증**: CI 파이프라인이 §8 품질 게이트를 모두 통과해야 한다.
2. **plan-auditor 검토**: thorough harness 레벨에서 본 SPEC의 EARS 준수·완전성·일관성을 적대적으로 감사한다.
3. **evaluator-active 교차 검증**: 4차원 채점(Functionality/Security/Craft/Consistency)에서 must-pass 항목 통과 필수.
4. **사람 데모**: KEPCO E&C 담당자(또는 대리 도메인 전문가)에 의한 §8.1 주관 평가 수행.
5. **승인**: 위 4단계 모두 통과 시 SPEC status를 `draft` → `approved`로 변경하고 Run phase로 진행.

---

## Iteration History

- iteration 1 (2026-05-14): 초안 작성, 16개 시나리오. plan-auditor 리뷰 결과: FAIL (D1-D14, MP-3 firewall + Traceability gap).
- iteration 2 (2026-05-14): D1/D2/D3/D6/D7/D8/D9/D10/D11/D13/D14 ACCEPTED. D4/D5 REJECTED (canonical schema 유지, 자세한 사유는 spec.md HISTORY). D12 ACCEPTED (structure.md 수정 사항을 spec.md §6 의존성에 handoff note로 명시). 시나리오 16 → 23, 신규: AC-UBI-002, AC-UBI-003, AC-001-4, AC-001-5, AC-002-4, AC-002-5, AC-002-6. 수정: AC-003-2 (D6 reconcile, primary scenario both < 0.5).
- iteration 3 (2026-05-14): evaluator-active cross-validation (0.813 PASS) 후속 findings 반영. NEW-F1: REQ-AX-003-E1에 abstain 분기 명시 (mathematical contradiction 해소). NEW-F2: REQ-UBI-003에 sandbox user_id 기본값 'cli-anonymous' State-driven 절 추가. 신규 AC-UBI-004 1개. 시나리오 23 → 24.
