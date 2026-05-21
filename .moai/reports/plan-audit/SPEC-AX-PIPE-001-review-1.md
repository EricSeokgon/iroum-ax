# SPEC Review Report: SPEC-AX-PIPE-001
Iteration: 1/3
Verdict: CONDITIONAL_PASS
Overall Score: 0.68

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ-PIPE-001 ~ REQ-PIPE-009, 9개 순차적, 갭·중복 없음 (spec.md:L80, L95, L108, L118, L133, L148, L162, L173, L183)
- [PASS] MP-2 EARS format compliance: REQ-PIPE-001~007은 Event-driven ("When ... 때, the system SHALL ..."), REQ-PIPE-008~009는 Ubiquitous ("The system SHALL ...") 형식 준수 (다만 REQ-PIPE-008의 내재 트리거 조건 문제는 MINOR 결함으로 별도 기재)
- [PASS] MP-3 YAML frontmatter validity: id, version, status, created, updated, author, priority, issue_number 8개 필드 전부 존재 (spec.md:L1-10)
- [N/A] MP-4 Language neutrality: 본 SPEC은 Python AI 파이프라인 단일 스택 대상. 16개 언어 다중 LSP 구성 SPEC이 아니므로 N/A

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 | 대부분 요구사항이 단일 해석 가능. Coverage 수치 충돌(85% vs 80%, spec.md:L188/L190) 및 "@MX:ANCHOR 함수" 기준 불명확(spec.md:L188)으로 일부 모호성 존재 |
| Completeness | 0.75 | 0.75 | HISTORY/개요/파일 목록/요구사항/AC/제약사항/Exclusions 모두 존재. REQ-UBI-002(Korean-first)의 추천 출력(REQ-PIPE-006) AC 누락, REQ-UBI-003 필수 필드 검증 누락 |
| Testability | 0.50 | 0.50 | Coverage 수치 모순(85% vs 80%), "@MX:ANCHOR 함수" 기준이 외부 코드 상태 의존, AC-PIPE-003-2 p99 측정의 단위/통합 테스트 경계 불명확 등 복수 결함 |
| Traceability | 0.75 | 0.75 | 9개 REQ 모두 AC 존재. §6.4 검증 매트릭스로 단위/통합 테스트 매핑 제공. REQ-PIPE-007은 단위 테스트 없이 통합 테스트에만 배정됨 |

---

## Defects Found

### MAJOR 결함

**D1. spec.md:L188 vs L190 — Coverage 수치 충돌 — Severity: MAJOR**

AC-PIPE-009-1(L188): "모듈당 분기 커버리지 ≥ 85%"
AC-PIPE-009-3(L190): "`pytest tests/ -v --cov=pipelines --cov-fail-under=80`"

두 AC가 동일한 REQ(REQ-PIPE-009) 내에서 85%와 80%라는 서로 다른 pass threshold를 제시한다. 구현자는 어느 기준을 따라야 하는지 알 수 없다.

**D2. spec.md:L179 — REQ-UBI-003 필수 필드 누락 — Severity: MAJOR**

SPEC-AX-001(parent) REQ-UBI-003: "record an audit log entry ... with **user_id, action, resource_id, and timestamp**"

AC-PIPE-008-2(L179): "audit_logs.details JSONB 는 요청 메타데이터(`file_type`, `endpoint`, `status_code`)를 포함한다"

`action` 필드와 `resource_id` 필드에 대한 검증 AC가 없다. parent SPEC이 명시한 필수 필드 4개 중 2개(action, resource_id)가 AC에서 누락됨. REQ-UBI-003 준수를 보장할 수 없다.

**D3. spec.md:L150-158 — REQ-UBI-002 추천 출력 한국어 AC 없음 — Severity: MAJOR**

SPEC-AX-001 REQ-UBI-002: "The system SHALL process Korean text as the primary language for all input parsing, RAG retrieval, draft generation, and **recommendation output**"

REQ-PIPE-006(Gap Recommendation API)의 AC-PIPE-006-1~006-4 어디에도 `RankedSuggestion`의 출력이 한국어여야 한다는 검증 조건이 없다. REQ-UBI-002의 "recommendation output" 범주가 이 SPEC에서 누락되었다.

**D4. spec.md:L188 — AC-PIPE-009-1의 "@MX:ANCHOR 함수" 기준 외부 의존 — Severity: MAJOR**

"모든 `@MX:ANCHOR` 함수에 대해 단위 테스트가 작성"이라는 조건은 런타임 코드에 MX 태그가 실제로 부착되어 있어야 테스트 가능하다. 현재 스켈레톤에 어떤 함수에 `@MX:ANCHOR`가 붙어있는지 이 SPEC 내에서 정의되지 않았다. 테스터가 어떤 함수를 반드시 테스트해야 하는지 알 수 없어 AC가 binary-testable하지 않다.

---

### MINOR 결함

**D5. spec.md:L175 — REQ-PIPE-008 EARS 패턴 미스매치 — Severity: MINOR**

"The system SHALL 7개 엔드포인트 전부에 대해 매 요청마다 audit_logs 테이블에 감사 기록을 남긴다"는 Ubiquitous 형식을 사용하지만 내재 조건("엔드포인트에 요청이 발생할 때")이 존재하므로 엄밀히는 Event-driven이어야 한다. Ubiquitous는 트리거가 없는 상수 조건에 적용된다. 의미는 명확하나 EARS 형식 위반이다.

**D6. spec.md:L112 / L289 — AC-PIPE-003-2 p99 측정의 테스트 유형 모호성 — Severity: MINOR**

AC-PIPE-003-2: "top_k=3, 50개 색인 criteria 기준 **p99 < 100ms**"가 REQ-PIPE-003의 AC로 정의되어 있다. §6.4 검증 매트릭스(L289)에서 "E2E search + p99 측정"으로 통합 테스트에 배정됨으로써 단위 테스트 vs 통합 테스트 경계가 혼재된다. 이 AC가 단위 테스트 대상인지 통합 테스트 대상인지 명확히 구분해야 한다.

**D7. spec.md:L290 — REQ-PIPE-007 단위 테스트 없이 통합 테스트에만 배정 — Severity: MINOR**

§6.4 검증 매트릭스(L290): "REQ-PIPE-007 | tests/integration/test_pipeline_e2e.py | feedback step"으로 단위 테스트 위치가 비어있다. AC-PIPE-007-1(JSONB 저장), AC-PIPE-007-2(HTTP 404), AC-PIPE-007-3(audit_log 생성)은 모두 단위 테스트로 독립 검증 가능하나 이 배정이 누락되었다.

---

## Chain-of-Verification Pass

첫 번째 패스 후 재검토한 결과:

추가 발견된 사항:
- REQ 번호 연속성(REQ-PIPE-001~009) 전수 재확인: 이상 없음
- Phantom API 전수 재확인: PDFParser/HWPParser/VLMProcessor/EmbeddingService/VectorStore/Retriever/BenchmarkLearner/GradePredictor/ScenarioSimulator/LLMClient/PromptBuilder/ReportDrafter/StyleApplier/GapAnalyzer/ContentSuggester/Prioritizer 16개 모두 `pipelines/` 디렉토리에 실존 확인됨
- SLA 숫자 크로스-참조(30s/100ms/1s/5s/3s) 5개 전수 확인: SPEC-AX-001과 완전히 일치
- Exclusions 10개 항목 실질성 확인: "Celery+Redis", "VLM/LLM 모델 파일 다운로드", "Multi-document 배치", "C/D 등급", "Console UI", "JWT 강제", "TableExtractor CV 로직", "외부 LLM 호출", "운영 모니터링", "국제화" — 모두 구체적이고 구현 경계가 명확함
- D1-D7 결정사항 7개 근거 전수 확인: 모두 rationale 존재
- AC-PIPE-006-1 "3-5건" 범위 표현은 `3 <= len(result) <= 5` assert로 binary-testable 확인
- D4의 재확인: Coverage 불일치(85%/80%)는 독립적으로 두 번 확인, D1로 유지
- REQ-UBI-001 allowlist: LLMClient 외 다른 모듈(EmbeddingService는 로컬 ko-sroberta-multitask 사용이므로 외부 API 아님) — REQ-UBI-001 위반 위험 없음. AC-PIPE-005-2 단일 AC로 충분 판단

첫 번째 패스에서 D1~D4(MAJOR), D5~D7(MINOR)를 식별하였으며 두 번째 패스에서 추가 결함 없음. 7개 결함으로 확정.

---

## Recommendation

본 SPEC은 전반적으로 구조가 탄탄하고 Phantom API 없음, SLA 수치 상속 정확, Exclusions 구체적이나, 아래 4개 항목을 수정하여 재제출해야 한다.

### 필수 수정 사항 (CONDITIONAL_PASS 해제 조건)

**Fix-1: Coverage 수치 통일 (D1)**

AC-PIPE-009-1과 AC-PIPE-009-3의 coverage 임계값을 통일한다. 권장 방안:
- AC-PIPE-009-1을 "Phase별 단위 테스트 실행 시 해당 도메인 모듈 분기 커버리지 ≥ 85%" (Phase A~E 범위)
- AC-PIPE-009-3을 "전체(단위+통합) 테스트 실행 시 `pytest tests/ --cov-fail-under=85`" (Phase F 종료 시)
- 또는 전체를 80%로 통일하되, AC-PIPE-009-1에서 "85%"를 "80%"로 수정

**Fix-2: audit_log 필수 필드 검증 추가 (D2)**

AC-PIPE-008-2를 다음과 같이 수정:
"audit_logs 레코드는 user_id, action, resource_id, timestamp 필드를 포함하며, details JSONB 에는 file_type, endpoint, status_code 요청 메타데이터가 저장된다"

**Fix-3: 추천 출력 한국어 AC 추가 (D3)**

REQ-PIPE-006에 다음 AC를 추가:
"AC-PIPE-006-5: RankedSuggestion 의 suggestion_text 및 rationale 필드는 한국어로 반환된다 (REQ-UBI-002)"

**Fix-4: MX:ANCHOR 기준 명확화 (D4)**

AC-PIPE-009-1의 "@MX:ANCHOR 함수에 대해" 기준을 다음 중 하나로 교체:
- 옵션 A: "모든 public 메서드(parse, encode, search, predict, draft_section, analyze, prioritize 등)에 대해 단위 테스트가 작성"
- 옵션 B: SPEC 내에 `@MX:ANCHOR` 대상 함수 목록을 명시하는 보조 테이블 추가
