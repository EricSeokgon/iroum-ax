# SPEC Review Report: SPEC-AX-INTEG-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.42

---

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency**: REQ-INTEG-001 ~ REQ-INTEG-008 순번이 연속적이고 갭/중복 없음. zero-padding 일관 (3자리). spec.md:L176~L223 전수 확인.

- [FAIL] **MP-2 EARS format compliance**: 인수 조건 8개 전체(AC-INTEG-001-1 ~ AC-INTEG-001-8, spec.md:L354~L408)가 `Given / When / Then / Test` BDD 형식으로 작성되어 있으며, EARS 5개 패턴(Ubiquitous, Event-driven, State-driven, Optional, Unwanted) 중 어느 것도 사용하지 않는다. 요구사항 섹션(sec 4)의 REQ들이 EARS 패턴을 올바르게 사용한 것과 달리, 인수 조건 섹션(sec 7)은 BDD 시나리오로 대체되어 있다. M3 루브릭 기준 "Given/When/Then test scenarios mislabeled as EARS" = 0.25 밴드.

- [FAIL] **MP-3 YAML frontmatter validity**: 필수 필드 3개 누락.
  - `priority` 필드 없음 (required: critical/high/medium/low) — spec.md:L1-12 전체
  - `labels` 필드 없음 (`tags` 필드는 존재하나 `labels` 키가 아님) — spec.md:L10
  - `created_at` 필드 없음 (`created: "2026-05-21"` 으로 오기재, ISO date 키 불일치) — spec.md:L6

- [N/A] **MP-4 Section 22 language neutrality**: 이 SPEC은 Python↔Go 특정 통합을 다루는 단일 도메인 SPEC으로, 다국어 LSP 도구 적용 범위가 아님. Auto-pass.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 | REQ 대부분 단일 해석 가능. REQ-INTEG-002 L184 "handle the HTTP 204" 불명확. REQ-INTEG-006 L211 비규범 default 텍스트 모호. |
| Completeness | 0.50 | 0.50 | 문서 구조 섹션(HISTORY/WHY/WHAT/REQ/AC/Exclusions) 모두 존재. 단 YAML frontmatter 3개 필수 필드 누락. |
| Testability | 0.75 | 0.75 | 모든 AC에 구체적 테스트 파일 및 함수명 참조. AC-INTEG-001-8 "30초 이내" 경계 조건이 CI 환경에서 flaky 가능성 있으나 측정 가능한 기준임. |
| Traceability | 0.75 | 0.75 | REQ-INTEG-001~008 각각 ≥1 AC 명시. AC-INTEG-001-8(L403)만 명시적 REQ 참조 없음(고아 AC). |

---

## Defects Found

**D1.** spec.md:L6 — `created` 키가 `created_at`이어야 함. 필수 frontmatter 필드 오기재. — **Severity: CRITICAL** (MP-3 위반)

**D2.** spec.md:L1-12 — `priority` 필드 없음 (required field: critical/high/medium/low). — **Severity: CRITICAL** (MP-3 위반)

**D3.** spec.md:L1-12 — `labels` 필드 없음 (`tags` 필드가 있으나 키 이름 불일치). — **Severity: CRITICAL** (MP-3 위반)

**D4.** spec.md:L354~L408 — 인수 조건(AC) 8개 전체가 `Given/When/Then/Test` BDD 형식 사용. EARS 5가지 패턴(Ubiquitous/Event-driven/State-driven/Optional/Unwanted) 미사용. MP-2 직접 위반. — **Severity: CRITICAL** (MP-2 위반)

**D5.** spec.md:L211 — `Local development default: \`http://localhost:8080\` (development docker-compose에서 주입).` 이 줄이 REQ-INTEG-006 규범적 요구사항 본문 안에 포함되어 있으나 EARS 패턴을 사용하지 않음. 규범적이라면 `WHERE local development, the worker SHALL default GO_CONTROL_PLANE_URL to http://localhost:8080` 형식이어야 함. 설명 텍스트라면 요구사항 섹션 외부로 이동해야 함. — **Severity: MAJOR**

**D6.** spec.md:L282~L302, L184, L445~L451 — callback 엔드포인트(`POST /api/v1/workflows/{id}/callback`)에 대한 출처 인증 요구사항 없음. 어떤 클라이언트든 RUNNING 상태의 workflow_id를 알면 임의로 COMPLETED/FAILED 전이를 유발할 수 있음. D4(L446)에서 "spoofing 가능성 축소"를 언급하지만 shared secret, mTLS, IP whitelist 등 실제 인증 메커니즘이 어느 REQ에도 정의되지 않음. — **Severity: MAJOR**

**D7.** spec.md:L184 — `"handle the HTTP 204 No Content response"` — "handle"의 의미가 불명확. "204 수신 시 성공으로 간주하고 재시도하지 않는다"는 행동이 명시되어야 함. 현재 표현으로는 구현자가 다양하게 해석 가능. — **Severity: MINOR**

**D8.** spec.md:L403~L408 — AC-INTEG-001-8이 특정 REQ-INTEG-XXX를 참조하지 않음 (`(Full E2E)` 레이블만 있음). 고아 AC. — **Severity: MINOR**

---

## Chain-of-Verification Pass

첫 번째 패스 이후 재검토:

- REQ 번호 전수 확인 완료 (001~008, 갭 없음) — MP-1 확인
- AC 전체(L354~L408) 재검토: 8개 모두 `- **Given**`, `- **When**`, `- **Then**`, `- **Test**` 구조. EARS 패턴 없음 확인 — MP-2 FAIL 확인
- YAML frontmatter 필드별 재확인: `id`(✓), `version`(✓), `status`(✓), `created`(≠`created_at`), `priority`(없음), `labels`(없음) — MP-3 FAIL 3건 확인
- Exclusions 섹션(sec 11, L523~L537): 10개 항목 구체적으로 명시. 적절함.
- REQ-INTEG-004 이중 bullet 재확인: 성공/실패 두 케이스를 단일 REQ로 표현. EARS 패턴 자체는 준수(두 WHEN...THEN 절). 비EARS 위반 아님. 단 단일 REQ 내 분기로 Clarity 경계.
- 보안 결함(D6) 재확인: D4(L445-451)에서 "user_id를 callback body에 포함하지 않음"이 보안 설계 근거로 제시되나, 이는 body 스키마 최소화이지 endpoint 인증이 아님. 완전히 별개 문제임. D6 유지.

신규 발견: 없음 — 첫 번째 패스가 주요 결함을 포괄적으로 적발함.

---

## Recommendation

### FAIL 요인 및 필수 수정 지시

**[수정 1] MP-3 YAML frontmatter 교정 (CRITICAL)**

spec.md:L1-12 frontmatter에 다음 3개 필드를 추가/수정:

```yaml
created_at: "2026-05-21"   # "created" → "created_at"으로 키 변경
priority: "high"           # 통합 레이어는 critical path — "high" 권장
labels: ["integration", "celery", "grpc", "rest", "workflow"]
```

기존 `tags` 필드는 `labels`로 키 이름 변경. `created` 키 삭제.

---

**[수정 2] MP-2 인수 조건 전체 EARS 패턴으로 재작성 (CRITICAL)**

현재 AC 8개 모두 Given/When/Then BDD 형식이므로 EARS 패턴으로 전환해야 한다. 각 AC를 다음 중 하나로 변환:

예시 — AC-INTEG-001-1 현재:
```
Given/When/Then 형식 (현재)
```

EARS 변환 예:
```
WHEN a Kombu v2 Celery envelope with task name `pipelines.workers.ingestion_worker.run`
is dequeued from the Redis `celery` queue,
THEN the Python Celery worker SHALL invoke the task function
with positional argument `document_id` and keyword argument `workflow_id`
matching the values in the envelope payload.
Test: tests/unit/test_integ_001_celery_envelope.py::test_envelope_deserialization
```

각 AC에 대해:
- AC-INTEG-001-1: Event-driven (`WHEN ... THEN ...`)
- AC-INTEG-001-2: Event-driven (`WHEN ... THEN ...`)
- AC-INTEG-001-3: Event-driven (`WHEN ... THEN ...`)
- AC-INTEG-001-4: Unwanted (`IF ... THEN ...`)
- AC-INTEG-001-5: Unwanted (`IF ... THEN ...`)
- AC-INTEG-001-6: Event-driven (`WHEN ... THEN ...`)
- AC-INTEG-001-7: Unwanted (`IF ... THEN ...`)
- AC-INTEG-001-8: Event-driven (`WHEN ... THEN ...`)

테스트 파일 참조는 EARS 본문 이후 `- Verification: <test_path>` 형식으로 분리 기재.

---

**[수정 3] REQ-INTEG-006 L211 비EARS 텍스트 처리 (MAJOR)**

다음 중 하나를 선택:
- 옵션 A (규범적으로 유지): `WHERE local development environment, the Celery app SHALL default GO_CONTROL_PLANE_URL to http://localhost:8080.`
- 옵션 B (설명 텍스트로 격하): 섹션 5.3 환경변수 표의 Default 컬럼에 이미 기재되어 있으므로 REQ-INTEG-006 본문에서 삭제.

---

**[수정 4] Callback 엔드포인트 출처 인증 REQ 추가 (MAJOR)**

보안 설계 결정(D4)에서 callback body 스키마 최소화를 결정했으나, callback 자체의 인증이 누락되어 있다. 다음 중 하나의 접근을 REQ로 명시:

- 옵션 A (shared secret header): `IF a callback POST arrives at /api/v1/workflows/{id}/callback without a valid X-Callback-Secret header matching the configured secret, THEN Go SHALL return HTTP 401 Unauthorized and SHALL NOT modify workflow state.`
- 옵션 B (localhost-only 바인딩): `WHILE Go callback handler is running, it SHALL only accept callback requests originating from localhost (127.0.0.1 or ::1).`
- 옵션 C (내부 네트워크 가정으로 Out-of-Scope 명시): sandbox/PoC 범위라면 Exclusions에 "callback 엔드포인트 출처 인증(향후 production 전환 시 추가)"을 명시적으로 추가하여 의도적 제외임을 문서화.

---

**[수정 5] AC-INTEG-001-8 REQ 참조 추가 (MINOR)**

spec.md:L403: `### AC-INTEG-001-8 (Full E2E)` → `### AC-INTEG-001-8 (REQ-INTEG-001, REQ-INTEG-002, REQ-INTEG-004, REQ-INTEG-007)` 로 변경하여 고아 AC 해소.

---

**[수정 6] REQ-INTEG-002 "handle" 모호성 해소 (MINOR)**

spec.md:L184: "handle the HTTP 204 No Content response" → "treat HTTP 204 No Content as callback success and SHALL NOT retry the callback POST"로 명확화.

---

## 강점 (Strengths)

1. **REQ 섹션 EARS 품질 우수**: REQ-INTEG-001~008의 요구사항 본문 자체는 WHEN/THEN/IF/WHILE 키워드를 일관되게 사용하여 EARS 패턴을 잘 준수함 (spec.md:L178, L184, L190, L196-198, L202, L209-211, L217, L223).

2. **Exclusions 섹션 구체적**: sec 11(L523~L537)에 10개 항목이 구체적으로 열거되어 있으며 각 항목이 후속 SPEC 또는 설계 결정(D2)을 참조함.

3. **Cross-SPEC traceability 명시적**: sec 1.3(L50~L63) 선행 SPEC 결정 사항 표에서 종속 관계와 제약을 명시적으로 기록. 후속 구현자가 부모 SPEC 위반을 실수할 위험을 낮춤.

4. **인터페이스 명세 정밀**: sec 5.1/5.2(L232~L302)에서 Celery envelope 스키마와 callback REST 스키마가 구체적인 JSON 예시와 함께 정의되어 있어 구현 모호성 최소화.

5. **테스트 전략 구체성**: sec 9(L456~L483)에서 단위/통합 테스트 파일별 검증 대상과 커버리지 목표(85%/90%)를 명시.

6. **설계 결정 문서화**: D1~D4가 결정 사유, 대안 검토, 위험을 명시적으로 기록하여 향후 재논의 방지.
