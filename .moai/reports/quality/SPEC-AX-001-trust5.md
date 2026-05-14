# SPEC-AX-001 TRUST 5 품질 보고서

> 생성일: 2026-05-14
> 검증 단계: Phase 2.5 (Sprint 6 GREEN 완료 후 sync 이전)
> 검증자: manager-quality
> 결론: **WARNING** (Critical 0건, Warning 다수)

---

## 전체 판정: WARNING

| 판정 기준 | 상태 |
|-----------|------|
| Critical (0건 필요) | 0건 — 충족 |
| Warning (5건 이하) | 3개 주요 경고 영역 — 임계 초과 |
| sync 진행 권고 | 수정 후 진행 권장 |

---

## TRUST 5 차원별 평가

### 1. Tested (테스트 완전성)

**판정: WARNING**

| 지표 | 결과 |
|------|------|
| 전체 테스트 수 | 177 통과 / 8 deselected (integration, gpu) |
| 실패 테스트 수 | 0 |
| 전체 커버리지 | **82%** (목표: 85%) |

**커버리지 미달 모듈 (80% 미만):**

| 모듈 | 커버리지 | 미달 이유 |
|------|---------|-----------|
| `pipelines/config/models.py` | **0%** | FastAPI Request/Response 모델 — 테스트 없음 |
| `pipelines/main.py` | **0%** | FastAPI 진입점 — 통합 테스트로 분류, unit 제외됨 |
| `pipelines/ingestion/pdf_parser.py` | **61%** | 회전 PDF 경로, 예외 처리 분기 미커버 |
| `pipelines/ingestion/vlm_processor.py` | **51%** | GPU 경로 분기 (`@pytest.mark.gpu` 제외) |
| `pipelines/mapping/vector_store.py` | **55%** | psycopg2 실제 연결 경로 미커버 |
| `pipelines/scoring/scenario_simulator.py` | **60%** | 다중 시나리오 분기 미커버 |
| `pkg/logging/logger.py` | **67%** | 구조화 로거 핸들러 설정 분기 미커버 |

**분석**: 전체 커버리지 82%는 목표 85%에 3%p 미달. `pipelines/main.py`와 `pipelines/config/models.py`는 integration 테스트 범위이므로 unit 미커버는 의도적이나 TOTAL에 반영됨. 해당 모듈 제외 시 실질 커버리지는 더 높을 것으로 추정되나, 공식 측정값 기준으로 WARNING 처리.

---

### 2. Readable (가독성)

**판정: PASS (단, 소규모 개선 권장)**

| 항목 | 결과 |
|------|------|
| 식별자 언어 | 영어 100% (클래스, 함수, 변수 모두 영어) |
| 코드 주석 언어 | 한국어 준수 (language.yaml `code_comments: ko` 충족) |
| 최대 모듈 크기 | `pipelines/mapping/vector_store.py` — 216 lines (300 한계 미달) |
| 명명 일관성 | snake_case (함수/변수), PascalCase (클래스) — 일관됨 |
| docstring 완전성 | 주요 public 함수 docstring 보유 (Args/Returns 형식) |

**관찰 사항:**
- @MX 태그 설명이 한국어로 작성되어 `code_comments: ko` 정책 준수
- 모든 모듈 300행 미만 — 분해 불필요
- 일부 테스트 파일에 한국어 인라인 주석 사용 — 일관성 정상

---

### 3. Unified (통합성 / 코드 스타일)

**판정: CRITICAL — 즉각 수정 필요**

**소스 코드 (`pipelines/` + `pkg/`) ruff 결과:**

```
Found 14 errors (E501 8건, UP042 4건, 기타 2건)
```

| 오류 유형 | 건수 | 심각도 | 내용 |
|-----------|------|--------|------|
| E501 (줄 길이 초과, >100자) | 8 | WARNING | @MX:ANCHOR 주석 문자열 + TODO 줄 |
| UP042 (StrEnum 사용 권장) | 4 | INFO | `class FileType(str, Enum)` → `StrEnum` 권장 |
| 기타 | 2 | INFO | E501 변형 |

**테스트 코드 (`tests/`) ruff 결과:**

```
Found 106 errors (E501 41건, I001 27건, F401 12건, 기타)
```

| 오류 유형 | 건수 | 심각도 | 내용 |
|-----------|------|--------|------|
| E501 | 41 | WARNING | 테스트 파일 내 긴 줄 |
| I001 | 27 | WARNING | import 블록 정렬 불일치 (`from __future__ import annotations` 순서) |
| F401 | 12 | ERROR | 사용되지 않는 import |
| B017 | 2 | WARNING | `pytest.raises(Exception)` — 과도하게 광범위한 예외 캐치 |
| SIM105 | 1 | INFO | `contextlib.suppress` 사용 권장 |

**판정 이유**: quality.yaml `lsp_quality_gates.run.max_lint_errors: 0` 정책상 lint 오류 0건이 요구됨. 소스 코드 14건, 테스트 코드 106건(F401 12건 포함) 모두 위반. `F401` (미사용 import)은 기능 오류로 이어질 수 있어 **CRITICAL** 상향.

---

### 4. Secured (보안)

**판정: WARNING (1건 주의)**

**REQ-UBI 불변식 검증:**

| 불변식 | 구현 위치 | 상태 |
|--------|-----------|------|
| REQ-UBI-001: 외부 LLM 차단 | `settings.py:26` `validate_llm_endpoint()` — allowlist frozenset 검증, `ExternalLLMBlockedError` 발생 | PASS |
| REQ-UBI-002: 한국어 우선 | `language_detector.py:23` `detect()` — 한국어 비율 0.2 임계값 기반, `low_korean_ratio` 플래그 | PASS |
| REQ-UBI-003: 감사 로깅 | `logger.py:44` `audit_event()` — user_id, action, resource_id, timestamp 4개 필드 반환 | PASS |
| REQ-UBI-003 (sandbox): cli-anonymous 기본값 | `logger.py:74` AUTH_ENABLED=false 시 `cli-anonymous` 적용 | PASS |

**하드코딩 시크릿 점검:**

| 항목 | 결과 |
|------|------|
| `postgres_password: "devpass"` | **WARNING** — `settings.py:69` Pydantic Field 기본값으로 존재. 개발용 기본값이나 .env 오버라이드 구조는 정상. 운영 배포 시 환경변수 주입 필수 문서화 필요. |
| API 키 / 토큰 하드코딩 | 미발견 |
| OpenAI / Anthropic 키 | 미발견 |

**위험 함수 점검:**

| 패턴 | 결과 |
|------|------|
| `eval(` | 미발견 |
| `exec(` | 미발견 |
| `os.system(` | 미발견 |
| `subprocess.*shell=True` | 미발견 |

**Pydantic 경계 검증**: `Settings`, `ParsedDocument`, `Criterion`, `GradeDistribution`, `DraftSection` 등 주요 모델 모두 Pydantic v2 `BaseModel` / `BaseSettings` 기반. 입력 경계 검증 구조 정상.

---

### 5. Trackable (추적 가능성)

**판정: PASS**

**@MX 태그 현황 (`pipelines/` + `pkg/`):**

| 태그 유형 | 건수 |
|----------|------|
| @MX:ANCHOR | 26건 |
| @MX:NOTE | 29건 |
| @MX:WARN | 0건 |
| @MX:TODO | 0건 |
| **합계** | **55건** |

**ANCHOR 분포 검증 (스프린트별 주요 진입점):**
- REQ-UBI: `validate_llm_endpoint`, `audit_event`, `detect` — ANCHOR 보유
- REQ-AX-001: `HWPParser.parse`, `PDFParser.parse`, `VLMProcessor.ocr` — ANCHOR 보유
- REQ-AX-002: `CriterionParser.parse`, `EmbeddingService.encode`, `Retriever.search`, `VectorStore.query` — ANCHOR 보유
- REQ-AX-003: `BenchmarkLearner.learn`, `GradePredictor.predict` — ANCHOR 보유
- REQ-AX-004: `LLMClient.generate`, `PromptBuilder.build`, `StyleApplier.validate`, `ReportDrafter.draft_section` — ANCHOR 보유
- REQ-AX-005: `GapAnalyzer.analyze`, `ContentSuggester.suggest`, `Prioritizer.prioritize` — ANCHOR 보유

**SPEC 추적 가능성:**
- 24개 AC 중 REQ-UBI (4개), REQ-AX-001 (5개), REQ-AX-002 (6개), REQ-AX-003 (3개), REQ-AX-004 (3개), REQ-AX-005 (3개) 각 모듈에 `@MX:SPEC: SPEC-AX-001 REQ-*` 어노테이션 존재
- 28개 테스트 파일이 AC별로 명명됨 (`test_req_{모듈}_{컴포넌트}.py`)

---

## LSP 상태

**판정: CRITICAL (lint 오류 존재)**

| 게이트 | 목표 | 실측값 |
|--------|------|--------|
| max_errors | 0 | 0 (런타임 오류 없음) |
| max_type_errors | 0 | mypy 미설치 — UNVERIFIED |
| max_lint_errors | 0 | **120건** (소스 14 + 테스트 106) |

**mypy**: `.venv`에 미설치. 타입 오류 검증 불가 — UNVERIFIED 처리.

---

## 주요 이슈 (우선순위 순)

### 이슈 1 (CRITICAL): ruff lint 오류 120건 — LSP 게이트 위반

- **위치**: `pipelines/`, `pkg/` 14건 / `tests/` 106건
- **핵심**: F401 (미사용 import) 12건은 잠재적 기능 오류, I001 (import 정렬) 27건은 `ruff --fix`로 즉시 해결 가능
- **해결**: `cd iroum-ax && .venv/bin/ruff check tests --fix` 실행 후 F401 수동 정리
- **블로킹 여부**: quality.yaml `max_lint_errors: 0` 정책 위반 → sync 이전 수정 필요

### 이슈 2 (WARNING): 전체 커버리지 82% — 목표 85% 미달

- **위치**: `pipelines/ingestion/vlm_processor.py` (51%), `pipelines/mapping/vector_store.py` (55%), `pipelines/scoring/scenario_simulator.py` (60%)
- **원인**: GPU 의존 경로 (`@pytest.mark.gpu` 제외), psycopg2 실제 DB 연결 경로, 복잡한 분기 경로 미커버
- **해결**: `vlm_processor` CPU 경로 mock 테스트 추가, `vector_store` 예외 분기 테스트 추가, `scenario_simulator` 경계값 시나리오 추가
- **블로킹 여부**: sync 전 개선 권장 (85% 미달)

### 이슈 3 (WARNING): `postgres_password: "devpass"` 하드코딩 기본값

- **위치**: `pipelines/config/settings.py:69`
- **내용**: Pydantic BaseSettings Field 기본값으로 개발용 패스워드 노출. .env 파일로 오버라이드 가능하나 `.env.example` 또는 운영 배포 가이드에 재정의 의무 문서화 필요
- **해결**: README 또는 deployments/ 문서에 운영 환경변수 `POSTGRES_PASSWORD` 필수 설정 명시
- **블로킹 여부**: 운영 미배포 단계이므로 blocking은 아니나 sync 전 문서화 권장

---

## 권고사항

1. **즉시 수정 (sync 이전 필수)**: `ruff check --fix tests/` 실행으로 I001, E501 자동 수정, F401 미사용 import 수동 정리
2. **커버리지 개선 (sync 이전 권장)**: `vlm_processor`, `vector_store`, `scenario_simulator` 3개 모듈 테스트 보강으로 85% 달성
3. **문서화 (sync 시 처리)**: `postgres_password` 운영 환경변수 오버라이드 가이드 추가
4. **mypy 설치 (선택)**: `uv add mypy` 후 타입 오류 검증 추가

---

## Sign-Off

- 날짜: 2026-05-14
- 상태: **needs_fixes** (ruff lint 해결 + 커버리지 개선 후 sync 진행)
- 검증자: manager-quality (SPEC-AX-001 v0.1.2)
