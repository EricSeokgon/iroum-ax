# Plan Audit Report — SPEC-AX-INGEST-001 — Iteration 2

Reasoning context ignored per M1 Context Isolation. Project canonical schema confirmed: 8-field frontmatter uses `created` (not `created_at`), no `labels` field. acceptance.md GWT format is project standard — not an EARS violation.

---

## Verdict: FAIL

Two MP-2 EARS violations remain in the revised spec.md v0.1.1. Additional critical defects include a persistent SLO contradiction in acceptance.md, an outdated REQ count in the acceptance.md Definition of Done, and a plan.md implementation conflict on HTTP response code interpretation for `score_triggered`. The SPEC cannot proceed to Run phase until these are resolved.

---

## iter1 Fix Verification

| Fix | Applied Correctly? | Notes |
|---|---|---|
| REQ-003 EARS split (003 + 003b) | Partial | REQ-003 (Event-driven) and REQ-003b (Unwanted) are correctly separated. spec.md:218 and L234. |
| REQ-004 EARS split (004 + 004b) | FAIL | REQ-004 (State-driven) correctly separated at spec.md:249. REQ-004b (spec.md:275) contains TWO distinct IF/THEN Unwanted blocks — two triggers in one REQ. Not fixed. |
| REQ-005 EARS restructure (005 + 005b + 005c) | FAIL | REQ-005b (Unwanted) and REQ-005c (State-driven) correctly separated. REQ-005 itself (spec.md:292) is still labeled "Ubiquitous" but retains temporal condition "Celery worker 시작 시" — should be Event-driven "WHEN Celery worker starts." Not fixed. |
| OPEN #1 RESOLVED (POST /api/v1/scores) | Yes | spec.md:362 and REQ-INGEST-003:224 confirm endpoint with Go source file references. |
| OPEN #3 RESOLVED (EmbeddingService) | Yes | spec.md:372 confirms `pipelines/mapping/embedding_service.py` existence and 768-dim interface. |
| SLO fixed (30s → 200s/document CPU) | Partial | spec.md:403 correctly states "p99 < 200s/문서 (CPU 환경, 10페이지 PDF 순차 처리 기준)." However acceptance.md EC-2 (L268) still says "p99 < 30s/문서 (CPU) 초과 가능" — the old contradictory figure. The fix was applied in spec.md but NOT propagated to acceptance.md. |
| summary parameter schema defined | Yes | spec.md:229 defines `{"pages_processed": int, "chunk_count": int, "tokens": int, "ocr_backend": str}` inline in REQ-INGEST-003 constraints. |
| EC-10 chunk failure behavior defined | Yes | spec.md:280-286 (REQ-INGEST-004b) defines skip behavior with WARNING log and `failed_chunks` field. acceptance.md EC-10 (L299-301) updated to reflect Option A/B with current OPEN notation replaced by "옵션 A 권장" with "Plan Phase B/C에서 결정" — partially resolved (behavior direction established). |

---

## Critical Defects (must fix before Run)

**[C1] spec.md:275–287 — REQ-INGEST-004b contains two distinct IF/THEN Unwanted triggers in one block**

REQ-INGEST-004b opens: "IF `_execute()` 실행 중 부분 실패가 발생하면, THEN THE SYSTEM SHALL..."
Then continues: "IF 임베딩 단계에서 특정 청크가 실패하면 (EC-10 시나리오), THEN THE SYSTEM SHALL..."

These are two separate trigger conditions (general partial failure vs. chunk-specific embedding failure) within one REQ block. EARS requires one trigger per requirement. Both triggers are Unwanted type — the type does not mix, but there are two distinct triggers producing two distinct system responses. The iter1 recommendation was to split, but this block was created with two IFs bundled.

Fix required: Split REQ-INGEST-004b into:
- REQ-INGEST-004b: "IF `_execute()` 실행 중 VLM/VectorStore 단계에서 처리 불가 오류가 발생하면, THEN THE SYSTEM SHALL `result_json`에 `error` 필드를 추가하고 callback `status='failed'`로 호출한다." (Unwanted)
- REQ-INGEST-004c (new): "IF 임베딩 단계에서 특정 청크가 실패하면, THEN THE SYSTEM SHALL 해당 청크를 skip하고 WARNING 로그를 기록하며 `result_json`에 `failed_chunks` 필드를 포함한다." (Unwanted)

Severity: critical (MP-2)

---

**[C2] spec.md:292–300 — REQ-INGEST-005 still labeled Ubiquitous but has temporal condition**

spec.md:292: "### REQ-INGEST-005: 부팅 시 LLM 엔드포인트 검증 강제 (Ubiquitous)"
spec.md:294: "**THE SYSTEM SHALL** Celery worker 시작 시 `validate_llm_endpoint(settings.vlm_endpoint)`를 호출하여..."

The Ubiquitous EARS pattern requires "The [system] shall [response]" with no conditions, triggers, or qualifiers. The phrase "Celery worker 시작 시" is a temporal trigger that makes this Event-driven, not Ubiquitous. Iter1 explicitly required this to be changed to Event-driven. The v0.1.1 left the section header as "(Ubiquitous)" and retained the temporal qualifier in the body.

Fix required: Change section header to "(Event-driven)" and rewrite the requirement as:
"WHEN Celery worker가 시작될 때, THE SYSTEM SHALL `validate_llm_endpoint(settings.vlm_endpoint)`를 호출하여 REQ-UBI-001 준수를 강제한다."

Severity: critical (MP-2)

---

**[C3] acceptance.md:268–269 — EC-2 SLO still contradicts spec.md §7.1**

acceptance.md EC-2: "정상 처리, chunks 다수, p99 < 30s/문서 (CPU) 초과 가능 — timeout 적용"
spec.md:403: "전체 _execute() 처리: p99 < 200s/문서 (CPU 환경, 10페이지 PDF 순차 처리 기준)"

The SLO fix in spec.md (30s → 200s CPU) was NOT applied to acceptance.md EC-2. EC-2 still references the old 30s figure. A tester reading EC-2 would apply the wrong performance criterion (30s instead of 200s per document). This is a testability defect — different parts of the same SPEC contradict each other on the primary performance bound.

Fix required: Update acceptance.md EC-2 to read: "p99 < 200s/문서 (CPU 기준, 10페이지 PDF 기준; GPU p99 < 20s)" and align with spec.md §7.1.

Severity: critical (contradicts fixed SLO; tester will use wrong number)

---

**[C4] plan.md:89 vs spec.md:229 — HTTP 200 vs HTTP 2xx for score_triggered=True**

plan.md Phase C.2 (ScoreTrigger tests, L89): "HTTP 200 → `score_triggered=True`"
spec.md REQ-INGEST-003 constraint (L229): "`score_triggered=true` 조건: HTTP 2xx 응답만 (3xx/4xx/5xx 모두 false)"

The Go scoring API returns HTTP 201 Created per spec.md:225: "응답 성공 형식: 201 + `{'score_id': '...', 'status': 'DRAFT'}`". If the implementation follows plan.md (HTTP 200 only → True), then a 201 response from Go API would incorrectly set `score_triggered=False`. This is a functional defect seeded in plan.md that will cause a bug in Phase C.2 implementation.

Fix required: Correct plan.md:89 from "HTTP 200 → score_triggered=True" to "HTTP 2xx (200, 201, 202...) → score_triggered=True". Also update plan.md Phase C.2 test case 1 to explicitly include 201 response as a success case.

Severity: critical (implementation will produce wrong result for the primary success path — Go returns 201)

---

## Minor Issues (fix recommended)

**[M1] REQ count inconsistency across documents**

- spec.md has 9 REQs: REQ-INGEST-001, 002, 003, 003b, 004, 004b, 005, 005b, 005c (counting C1's split, will become 10)
- plan.md DoD (L360): "8개 REQ 모두 EARS 형식으로 구현 (v0.1.1: 5→8 EARS 분리)" — says 8, lists "003/003b/004/004b/005/005b/005c" (7 split REQs) + 001 + 002 = 9 total, but text says 8
- acceptance.md DoD (L367): "5개 REQ 모두 EARS 형식으로 구현" — still the original v0.1.0 count; NOT updated to reflect splits

The acceptance.md DoD is particularly problematic: it says "5개 REQ" while the spec now has 9. A developer completing implementation against the acceptance.md DoD would believe they only need to cover 5 REQs.

Fix recommended: Update plan.md DoD to "9개 REQ" (or 10 after C1 split fix). Update acceptance.md DoD to match.

Severity: major (misleading DoD)

**[M2] REQ-INGEST-004b general partial failure has no dedicated numbered AC**

REQ-INGEST-004b's first IF condition (general partial failure at VLM/VectorStore step → status=failed + error field) is not covered by a dedicated numbered AC. EC-10 in acceptance.md covers only the chunk embedding partial failure scenario (second IF in 004b). AC-INGEST-001-2 covers VLM timeout failure, but that is mapped to REQ-INGEST-001 and REQ-INGEST-004, not REQ-INGEST-004b specifically.

Fix recommended: Add AC-INGEST-001-9 or remap an existing AC to cover the first IF condition of REQ-INGEST-004b explicitly: general pipeline step failure → callback status=failed with Korean error message.

Severity: major (traceability gap — acceptance.md REQ 매핑 for REQ-INGEST-004b is only via EC-10, not a numbered AC)

**[M3] AC-INGEST-001-8 traces to cross-SPEC REQ-UBI-002 (iter1 M2 unresolved)**

acceptance.md:109: "REQ 매핑: REQ-UBI-002 (횡단 제약)"
REQ-UBI-002 is not defined in spec.md Section 4. It is referenced as a cross-cutting constraint in spec.md §1.3 but has no REQ entry in this SPEC.

Iter1 recommended either defining a local REQ for Korean-language error messages or remapping AC-INGEST-001-8 to REQ-INGEST-004b (which already states "오류 메시지는 한국어 우선 (REQ-UBI-002)"). Neither fix was applied.

Fix recommended: In the REQ 매핑 for AC-INGEST-001-8, change "REQ-UBI-002 (횡단 제약)" to "REQ-INGEST-004b (부분 실패 처리 — 한국어 오류 메시지 포함)" to establish a traceable link within this SPEC.

Severity: major (traceability convention violation)

**[M4] REQ-INGEST-001 item 4: embedded Unwanted condition inside Event-driven block**

spec.md:187: Item 4 in REQ-INGEST-001's numbered list: "OCR 텍스트가 빈 문자열이면 `IngestionEmptyError` 발생"

This is an implicit IF/Unwanted condition embedded as a numbered step within an Event-driven requirement. Iter1 flagged this in the chain-of-verification pass. It was not addressed in v0.1.1. The strict EARS standard requires separation, but this is a lower-severity issue since the enclosing REQ structure is Event-driven and the embedded condition is expressed as a numbered step, not a full IF/THEN EARS block.

Fix recommended: Extract to a standalone REQ: "IF `VLMProcessor.ocr(file_path)` 빈 문자열을 반환하면, THEN THE SYSTEM SHALL `IngestionEmptyError`를 발생시키고 callback `status='failed'`로 호출한다." (Unwanted pattern)

Severity: minor (EARS mixing at sub-clause level, not full block level)

**[M5] "b"/"c" suffix REQ ID convention not sequential**

The split REQs use non-standard suffixes (003b, 003c, 004b, 005b, 005c) instead of sequential numbers (006, 007, 008, 009) as recommended in iter1. Per MP-1, "REQ numbers must be sequential (REQ-001, REQ-002, ... REQ-N)." REQ-INGEST-003 and REQ-INGEST-003b share the same numeric stem (003), which is at minimum ambiguous about ordering.

Iter1 Fix 2 explicitly recommended renaming to sequential IDs (REQ-INGEST-006 through REQ-INGEST-009). This was not followed.

While the "b/c" suffix pattern is used internally in some requirements engineering practices, it conflicts with the strict sequential numbering requirement in the plan-auditor MP-1 criterion.

Fix recommended: Rename splits to sequential numbers. After C1 is resolved (adding 004c), the full list would be: 001, 002, 003, 004, 005, 006 (former 003b), 007 (former 004b first IF), 008 (former 004b second IF / new 004c), 009 (former 005b), 010 (former 005c). Update all cross-references in plan.md, acceptance.md, and spec.md §5.

Severity: minor (convention; functional meaning is unambiguous, but fails strict MP-1 sequentiality test)

---

## Remaining OPENs Assessment

| OPEN | Status | Run-blocker? | Rationale |
|---|---|---|---|
| #1 채점 트리거 엔드포인트 | RESOLVED v0.1.1 | No | `POST /api/v1/scores` confirmed with Go source file reference. |
| #2 Python→Go 인증 토큰 전달 방식 | Open | Conditional | Must be resolved in Phase A before Phase C.2 (ScoreTrigger) begins. Not a Run-entry blocker if Phase A is gate-controlled. C candidate (gRPC bypass) would violate [HARD] 0-diff. Candidate A (SCORE_API_TOKEN env var) or B (envelope header injection) must be selected. Risk: if unresolved at Phase C.2 start, ScoreTrigger will be implemented incorrectly. |
| #3 EmbeddingService 구현 상태 | RESOLVED v0.1.1 | No | `pipelines/mapping/embedding_service.py` confirmed, `encode()` 768-dim. |
| #4 DocumentMetadataClient 데이터 소스 | Open | Conditional | Must be resolved in Phase A before Phase C.3 (DocumentMetadataClient) begins. Candidate C (Celery envelope modification) would violate [HARD] 0-diff if it requires Go dispatcher changes. Candidate B (file path in envelope) is safest. Not a Run-entry blocker if Phase A gates it. |
| #5 VLM 타임아웃 설정 위치 | Open | No | Implementation detail. Phase E resolves. All candidates are Python-only. |
| #6 TextChunker 알고리즘 선택 | Open | No | Implementation detail. Phase C.1 resolves. All candidates are Python-only. |

**Assessment:** OPEN #2 and #4 are conditionally acceptable for Run phase entry IF Phase A is treated as a mandatory gate that resolves both before Phases C.2 and C.3 commence. The plan.md dependency graph (A → B/C/E) already enforces this. However, the Plan document should make explicit that Phase C.2 and C.3 cannot begin until OPEN #2 and #4 are recorded in spec.md §6.2 as RESOLVED. This should be added to the plan.md Phase A completion criteria.

---

## Checklist Results

| Category | Status | Notes |
|---|---|---|
| EARS Compliance (MP-2) | FAIL | REQ-INGEST-004b has two IF/THEN triggers in one block; REQ-INGEST-005 labeled Ubiquitous with temporal condition. REQ-INGEST-001 has embedded Unwanted in item 4 (minor). |
| AC Testability | PASS | All 8 numbered ACs are binary-testable via GWT scenarios. EC-10 behavior is now defined (skip + WARNING + failed_chunks). EC-2 SLO number incorrect (see C3) but structure is testable. |
| Scope Integrity | PASS | consumer-only [HARD] explicitly stated (spec.md:43, L68; plan.md:318-325). Python-only delta markers correct. No Go modifications planned. |
| Constraint Completeness | PASS | REQ-UBI-001/002/003 all referenced in spec.md §1.3 and §7.2. fire-and-forget constraint explicit in REQ-INGEST-003b. VLM localhost-only constraint in REQ-INGEST-005b. |
| Plan Feasibility | PARTIAL | Phase A→B/C/E→D→F dependency graph is sound. OPEN #2/#4 must gate Phases C.2/C.3 explicitly. HTTP 200 vs 2xx defect (C4) will cause a bug in Phase C.2 if not corrected before implementation. |
| DoD Completeness | FAIL | acceptance.md DoD (L367) says "5개 REQ" — outdated; spec now has 9 REQs. plan.md DoD says "8개 REQ" — also wrong. REQ count inconsistency across all three documents. |
| Cross-SPEC Consistency | PASS | INTEG-001, PIPE-001, SCORE-API-001 referenced consistently. consumer-only [HARD] correctly enumerates frozen Go files in plan.md:318-325. Callback interface preserved per INTEG-001 contract. |
| YAML Frontmatter (MP-3) | PASS | Using project canonical 8-field schema (`created`, no `labels`). All fields present with correct types: id (string), title (string), version (string), status (string), created (ISO date), updated (ISO date), author (string), priority (string). |
| REQ Numbering (MP-1) | CONDITIONAL | No gaps or strict duplicates in numeric sense, but "b/c" suffix pattern conflicts with MP-1 sequential ordering requirement. REQ count in plan.md DoD is wrong (8 vs actual 9). Flagged as M5 (minor). |

---

## Chain-of-Verification Pass

Second-look findings:

1. **Re-read REQ-INGEST-002 end-to-end**: spec.md:198-213. "WHEN REQ-INGEST-001이 OCR 텍스트를 성공 반환하면, THE SYSTEM SHALL..." — Event-driven, clean. Constraint: "VectorStore가 `IndexRebuildingError` 발생 시 최대 3회 재시도 (지수 백오프 2s/4s/8s)" — this is an embedded recovery behavior for an error condition. The recovery behavior is part of the SHALL steps (not a separate IF/THEN block). This is a judgment call: I consider this acceptable as a behavioral constraint within an Event-driven REQ rather than an EARS type mix, since it describes the system's response within the SHALL clause rather than introducing a new trigger.

2. **Traceability for REQ-INGEST-005c (CPU fallback)**: spec.md:319-328. AC-INGEST-001-3 in acceptance.md (L99-103) includes a sub-case "vlm_endpoint = '' → 예외 없음 (CPU fallback 허용)". This is a sub-case within AC-3, not a dedicated AC. REQ-INGEST-005c has no standalone numbered AC. This is a traceability gap but acceptable given the coverage via sub-case. Not escalated to critical.

3. **plan.md Phase B lists "8개 단위 테스트 GREEN" for VLMProcessor**: plan.md:64. Cross-checking: plan.md §6.2 table (L340) shows "test_req_ax_001_vlm_processor.py (기존) | 8 (기존) + 4 (신규)". The "8개" in Phase B refers to the 8 existing tests, and "4 (신규)" are additional. This is consistent. No defect.

4. **Verified REQ ID sequencing end-to-end**: REQ-INGEST-001 (L179), REQ-INGEST-002 (L198), REQ-INGEST-003 (L218), REQ-INGEST-003b (L234), REQ-INGEST-004 (L249), REQ-INGEST-004b (L275), REQ-INGEST-005 (L292), REQ-INGEST-005b (L305), REQ-INGEST-005c (L319). 9 REQs total. No gaps between base numbers. No pure duplicates. The "b/c" suffix issue is captured in M5.

5. **Verified that plan.md HTTP 200 vs spec 2xx conflict (C4) is specific to Phase C.2**: plan.md:89 is explicitly in the test case list for ScoreTrigger. The single test "HTTP 200 → score_triggered=True" will NOT test the 201 case. Since Go API returns 201 per spec.md:225, the implementation will have score_triggered=False for successful triggers unless corrected. Confirmed as critical.

6. **acceptance.md EC-10 resolution check**: acceptance.md L299-301: "**결정 필요**: Plan Phase B/C에서 결정 (현재 OPEN, 옵션 A 권장)". The text "결정 필요" and "현재 OPEN" remain. Spec.md §4 REQ-INGEST-004b defines the behavior (skip chunks, WARNING log, failed_chunks field). But acceptance.md EC-10 still says "결정 필요." The behavior is now defined in spec.md but the acceptance.md edge case text was not updated to remove "결정 필요." This is a minor inconsistency — the spec defines the behavior but acceptance.md still marks it as open. Not critical, but the implementer reading acceptance.md EC-10 will see "결정 필요" and be confused.

   Adding this as minor finding: acceptance.md EC-10 (L301) "결정 필요" language is outdated — REQ-INGEST-004b now defines the behavior. EC-10 should be updated to reflect the resolved decision.

7. **Checked for contradictions between REQ-INGEST-003b (fire-and-forget) and REQ-INGEST-004 (result schema)**: REQ-INGEST-003b says callback `status="completed"` even when score_triggered=false. REQ-INGEST-004 shows result schema includes `score_triggered: bool`. These are consistent — no contradiction.

8. **Verified spec.md Exclusions (§1.2 OUT of scope)**: 12 specific, concrete exclusions listed (L41-52). Not vague. SC-6 PASS confirmed.

**Additional defect found in second pass:**

[M6] acceptance.md EC-10 (L299-301): The edge case text still says "결정 필요: Plan Phase B/C에서 결정 (현재 OPEN, 옵션 A 권장)" — but REQ-INGEST-004b in spec.md v0.1.1 now defines the behavior (skip chunk, WARNING log, failed_chunks field). The EC-10 acceptance criterion was not updated to remove the "결정 필요" deferral marker. A tester or implementer reading EC-10 will incorrectly believe the behavior is still undecided.

Severity: minor (spec defines behavior; acceptance.md EC-10 just needs deferral text removed)

---

## Regression Check (Iteration 2)

Defects from iter1:

| iter1 Defect | iter1 Severity | Status | Evidence |
|---|---|---|---|
| C1: YAML `created_at` missing (uses `created`) | critical (MP-3) | NOT APPLICABLE — False positive | Project canonical schema uses `created` not `created_at`. Confirmed per project context. |
| C2: YAML `labels` entirely absent | critical (MP-3) | NOT APPLICABLE — False positive | Project canonical schema has no `labels` field. Confirmed per project context. |
| C3: REQ-INGEST-003 mixed EARS | critical (MP-2) | RESOLVED | spec.md:218 (Event-driven) and L234 (Unwanted 003b) correctly split. |
| C4: REQ-INGEST-004 mixed EARS | critical (MP-2) | PARTIAL — New defect C1 this report | spec.md:249 (State-driven 004) correctly split but 004b (L275) still has two IF/THEN triggers. |
| C5: REQ-INGEST-005 mislabeled + mixed | critical (MP-2) | PARTIAL — New defect C2 this report | 005b and 005c correctly split; REQ-INGEST-005 label still says "Ubiquitous" with temporal condition. |
| C6: ACs use GWT not EARS | critical (MP-2) | NOT APPLICABLE — False positive | acceptance.md GWT format is project standard. Not an EARS violation for this project. |
| C7: OPEN #3 EmbeddingService unverified | critical (blocking) | RESOLVED | spec.md:372 confirms existence. |
| C8: OPEN #1 endpoint undecided | critical (blocking) | RESOLVED | spec.md:362, REQ-INGEST-003:224. |
| M1: SLO contradiction (30s vs 200s) | major | PARTIAL — New defect C3 this report | spec.md §7.1 fixed to 200s. acceptance.md EC-2 still says 30s. |
| M2: AC-INGEST-001-8 cross-SPEC REQ trace | major | UNRESOLVED | acceptance.md:109 still maps to "REQ-UBI-002 (횡단 제약)". Captured as M3 this report. |
| M3: EC-10 undecided behavior | major | PARTIALLY RESOLVED | REQ-INGEST-004b defines behavior, but acceptance.md EC-10 still says "결정 필요." Captured as M6 this report. |
| M4: summary parameter undefined | major | RESOLVED | spec.md:229 defines summary schema explicitly. |

---

## Recommendation

This SPEC FAILS iteration 2. Two MP-2 EARS violations remain unresolved (C1, C2). Additionally, three critical-severity defects were found in this iteration (C3: EC-2 SLO mismatch, C4: HTTP 2xx vs 200 conflict in plan.md) that will produce wrong behavior or wrong test results if not corrected before implementation.

### Fix 1 (C1 — MP-2): Split REQ-INGEST-004b into two separate Unwanted REQs

spec.md:275-287. Divide into:
- REQ-INGEST-004b: covers only the general partial failure path (VLM raise, VectorStore terminal failure) → status="failed" + error field
- REQ-INGEST-004c (new): covers chunk-specific embedding failure → skip + WARNING + failed_chunks field

Update all cross-references: spec.md §5 AC summary, plan.md DoD REQ count, acceptance.md DoD REQ count, acceptance.md EC-10 REQ mapping.

### Fix 2 (C2 — MP-2): Correct REQ-INGEST-005 to Event-driven

spec.md:292-300. Change section header from "(Ubiquitous)" to "(Event-driven)". Rewrite the SHALL statement as: "WHEN Celery worker가 시작될 때, THE SYSTEM SHALL `validate_llm_endpoint(settings.vlm_endpoint)`를 호출하여 REQ-UBI-001 준수를 강제한다."

### Fix 3 (C3): Propagate SLO fix to acceptance.md EC-2

acceptance.md:268-269. Replace "p99 < 30s/문서 (CPU) 초과 가능" with "p99 < 200s/문서 (CPU 기준, 10페이지 PDF; GPU p99 < 20s/문서)" to match spec.md:403.

### Fix 4 (C4): Correct plan.md HTTP response code for score_triggered

plan.md:89. Change test case from "HTTP 200 → score_triggered=True" to "HTTP 2xx (특히 201 Created) → score_triggered=True". Add explicit test: "HTTP 201 → score_triggered=True" since that is the Go API's actual success response (spec.md:225).

### Fix 5 (M1): Update REQ counts in plan.md and acceptance.md DoD

plan.md:360: Change "8개 REQ" to correct count (9, or 10 after C1 split).
acceptance.md:367: Change "5개 REQ 모두 EARS 형식으로 구현" to correct count.

### Fix 6 (M3): Remap AC-INGEST-001-8 to in-SPEC REQ

acceptance.md:109. Change "REQ-UBI-002 (횡단 제약)" to "REQ-INGEST-004b (한국어 오류 메시지 — REQ-UBI-002 횡단 제약 포함)". This establishes traceability to an in-SPEC REQ while acknowledging the cross-SPEC constraint.

### Fix 7 (M6): Remove "결정 필요" from acceptance.md EC-10

acceptance.md:301. Remove the line "**결정 필요**: Plan Phase B/C에서 결정 (현재 OPEN, 옵션 A 권장)" and replace with the decided behavior: "**결정**: 성공한 청크는 upsert 진행, 실패한 청크는 skip (REQ-INGEST-004c — 옵션 A 확정)."

### Fix 8 (OPEN gate): Make plan.md Phase A completion criteria explicit for OPEN #2/#4

Add to plan.md Phase A산출물: "OPEN #2 및 OPEN #4 결정 완료 기록 (spec.md §6.2 업데이트) — Phase C.2 및 C.3 진입 전 강제 선행 조건" to ensure these OPENs cannot be deferred past Phase A.
