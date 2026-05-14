# Sprint Contract — REQ-AX-004 Report Draft Generation

- SPEC: SPEC-AX-001 v0.1.2
- Sprint: 5
- 작성일: 2026-05-14
- Harness level: thorough (sprint contract 필수)
- Priority dimension: **Functionality + Originality** (한국어 공문 합니다체 enforcement)

---

## 1. 수락 기준 체크리스트

| AC | 설명 | 검증 방법 |
|----|------|----------|
| AC-004-1 | 합니다체 초안 생성 + style_applier 통과 + 5초 이내 응답 | `test_req_ax_004_report_drafter.py` — 합니다체 단언 |
| AC-004-2 | 스타일 위반 → 최대 3회 재시도 → 3회 후 `style_violation` | `test_req_ax_004_report_drafter.py` — retry 최대 횟수 경계 |
| AC-004-3 | EXAONE 3회 실패 → Qwen 2.5 자동 fallback + `model_used` 메타데이터 | `test_req_ax_004_llm_client_generate.py` — fallback 분기 |

---

## 2. 우선순위 차원

### 2.1 Primary: Functionality (30%)

- `PromptBuilder.build()` — 평가편람 context + 고객 콘텐츠 + 한국어 공문 스타일 지시문이 프롬프트에 포함되어야 함
- `LLMClient.generate()` — `GenerationResult` (text, model_used, tokens, latency_ms) 반환 계약
- `ReportDrafter.draft_section()` — `DraftSection` 반환, prompt_builder → llm_client → style_applier 순서 보장

### 2.2 Secondary: Originality (35%) — 한국어 공문 합니다체 enforcement

- `StyleApplier.validate()` — 합니다체 종결어미 (`습니다`, `입니다`, `됩니다`, `합니다`) 감지
- 해체(`해`, `이야`) 또는 구어체(`했어요`) 단독 사용 시 `is_official=False` 반환
- 영어 단독 텍스트(한국어 0%) 거부 — `is_official=False`, `violations`에 `no_korean` 포함
- `honorific_score` — 합니다체 종결어미 비율 (0.0~1.0)
- 재시도 로직 — max 2 재시도 (v0.1.2 Unwanted 절: 최대 3회 total = 1초 + 2재시도)

### 2.3 Security (25%)

- 외부 LLM API 호출 차단 (REQ-UBI-001 상속): allowlist 검증은 기존 `validate_llm_endpoint`가 담당
- Qwen 2.5 fallback도 내부 엔드포인트만 허용

### 2.4 Completeness (10%)

- `model_used` 메타데이터 (`"qwen2.5-7b"` 또는 주 모델명) 추적
- `DraftSection.status` 필드: `"ok"` | `"style_violation"`

---

## 3. 테스트 시나리오 (단위 테스트 — httpx/FastAPI testclient 대체)

### 3.1 PromptBuilder 시나리오

| # | 입력 | 기대 출력 |
|---|------|----------|
| PB-1 | Criterion + customer_content | 프롬프트에 criterion_name 포함 |
| PB-2 | Criterion + customer_content | 프롬프트에 customer_content 포함 |
| PB-3 | 한국어 공문 스타일 지시 확인 | "합니다체" 또는 "공문" 키워드 포함 |
| PB-4 | `criterion_detail` 비어 있는 경우 | 예외 없이 프롬프트 생성 |
| PB-5 | customer_content 빈 문자열 | 예외 없이 프롬프트 생성 |

### 3.2 StyleApplier 시나리오

| # | 입력 텍스트 | 기대 `is_official` | 기대 `honorific_score` |
|---|-------------|-------------------|----------------------|
| SA-1 | "안전교육을 실시하였습니다." | True | ≥ 0.8 |
| SA-2 | "안전보건 관리체계를 도입하였습니다." | True | ≥ 0.8 |
| SA-3 | "실시했어요, 수행했습니다 혼재." | False | 0.5 미만 가능 |
| SA-4 | "안전교육 실시했어 이번에." | False | 낮음 |
| SA-5 | "Safety training was conducted." (영어 단독) | False | violations에 `no_korean` |
| SA-6 | "합니다체 문장입니다. 이렇게 씁니다." (다중 문장) | True | ≥ 0.8 |
| SA-7 | "" 빈 문자열 | False | 0.0, violations에 `empty_text` |
| SA-8 | "안전교육을 실시하였습니다. 됩니다. 입니다." | True | 1.0 |

### 3.3 LLMClient.generate() 시나리오

| # | 설정 | 기대 결과 |
|---|------|----------|
| LC-1 | 정상 내부 엔드포인트 mock | `GenerationResult` 반환 |
| LC-2 | `use_gpu=False` (default) | `model_used`에 모델명 포함 |
| LC-3 | `use_gpu=True` (@pytest.mark.gpu) | GPU device routing 경로 실행 |
| LC-4 | `text` 필드 비어 있지 않음 | text len > 0 |
| LC-5 | `model_used` 필드 존재 | "qwen" 포함 문자열 |
| LC-6 | `tokens` 필드 int > 0 | 정수형 |
| LC-7 | `latency_ms` 필드 int > 0 | 정수형 |

### 3.4 ReportDrafter 시나리오

| # | 설정 | 기대 결과 |
|---|------|----------|
| RD-1 | 정상 LLM 응답 + style 통과 | `DraftSection.status == "ok"` |
| RD-2 | 첫 응답 style 실패 → 재시도 → 통과 | status "ok", retry_count == 1 |
| RD-3 | 모든 시도 style 실패 (max 2 재시도) | status "style_violation", retry_count == 2 |
| RD-4 | `model_used` 필드 추적 | DraftSection에 model_used 포함 |
| RD-5 | LLM 예외 → 재시도 안 하고 예외 전파 | 예외 발생 (retry는 style 실패 전용) |

### 3.5 통합 시나리오

| # | 설정 | 기대 결과 |
|---|------|----------|
| INT-1 | 전체 파이프라인 (mock LLM + 합니다체 응답) | DraftSection.status "ok" + model_used 포함 |
| INT-2 | Qwen fallback 경로 (EXAONE mock 5xx + Qwen mock 정상) | model_used "qwen2.5-7b" |
| INT-3 | max retry exhausted (mock LLM 해체 응답 반복) | status "style_violation" |

---

## 4. Pass 조건

- 3 AC (AC-004-1, AC-004-2, AC-004-3) 자동화 테스트 통과
- 공문 합니다체 일관성 StyleApplier 통과율 ≥ 95% (단위 테스트 기준)
- 외부 API 호출 시도 0건 (기존 REQ-UBI-001 테스트 회귀 없음)
- EXAONE 미가용 시 Qwen-only 경로로 AC-004-1 검증 가능 (strategy §4.1 결정)
- 단위 테스트 coverage ≥ 85%
- LSP errors=0, type_errors=0, lint_errors=0

---

## 5. Retry 정책 (v0.1.2 Unwanted 절 해석)

- AC-004-2 원문: "최대 3회 재시도 후에도 통과 못 하면 status `style_violation`"
- 해석: total 3회 시도 = 1초 시도 + 2재시도 → `max_retries=2` (인자)
- `ReportDrafter.draft_section()` — style 실패 시에만 재시도, LLM 예외는 재시도 없이 전파

---

## 6. 모의 전략

| 컴포넌트 | 모의 방법 | 비고 |
|---------|----------|------|
| Qwen 2.5 7B 모델 로딩 | `unittest.mock.patch` + `MagicMock` | 실제 모델 로딩 불가 (단위 테스트) |
| LLMClient.generate (내부) | `MagicMock` return_value | GenerationResult 픽스처 반환 |
| 한국어 응답 (합니다체) | 픽스처 문자열 | `"안전교육을 실시하였습니다."` |
| 해체 응답 (스타일 위반) | 픽스처 문자열 | `"안전교육 실시했어. 이번에."` |
| EXAONE 5xx | `side_effect=Exception("5xx")` | 3회 연속 실패 시나리오 |

---

## 7. Evaluator 4-dim 가중치

| 차원 | 가중치 |
|------|-------|
| Originality (한국어 공문) | 35% |
| Functionality | 30% |
| Security (LLM allowlist) | 25% |
| Completeness | 10% |

---

Version: 1.0.0
Created: 2026-05-14
Sprint: 5 (REQ-AX-004)
