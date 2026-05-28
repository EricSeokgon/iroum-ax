# Plan Audit Report — SPEC-AX-INGEST-001 — Iteration 1

Reasoning context ignored per M1 Context Isolation.

---

## Verdict: FAIL

**Overall Score: 0.53**

Two must-pass criteria failed (MP-2 EARS, MP-3 YAML). Additional critical defects found in unresolved blocking OPENs. Cannot proceed to Run phase until critical defects are resolved.

---

## Must-Pass Results

- **[FAIL] MP-1 REQ Number Consistency**: REQ-INGEST-001 through REQ-INGEST-005 are sequential with no gaps or duplicates. Zero-padded to 3 digits consistently. PASS — evidence: spec.md:178, 197, 217, 235, 264.

- **[FAIL] MP-2 EARS Format Compliance**: Four violations found across REQ section and Acceptance Criteria section:

  1. **REQ-INGEST-003 (spec.md:217-232)** — Mixes Event-driven and Unwanted EARS types within one REQ block. Opens with "WHEN REQ-INGEST-002가 VectorStore.upsert()를 성공 반환하면, THE SYSTEM SHALL..." (Event-driven), then adds "IF Go 채점 API가 503/504/네트워크 오류를 반환하면, THEN THE SYSTEM SHALL..." (Unwanted). Two distinct EARS patterns in one requirement = FAIL.

  2. **REQ-INGEST-004 (spec.md:235-260)** — Mixes State-driven and Unwanted EARS types. Opens with "WHILE _execute()가 정상 실행 경로를 따르는 동안, THE SYSTEM SHALL..." (State-driven), then adds "IF 부분 실패가 발생하면, THEN THE SYSTEM SHALL..." (Unwanted). Two distinct EARS patterns in one requirement = FAIL.

  3. **REQ-INGEST-005 (spec.md:264-279)** — Mislabeled as Ubiquitous but mixes three EARS types. The main body "THE SYSTEM SHALL Celery worker 시작 시..." contains a temporal condition ("시작 시" = at startup), which disqualifies it as Ubiquitous (Ubiquitous means universal, no conditions). Then adds IF/Unwanted and WHERE/Optional patterns. Three EARS types in one block = FAIL.

  4. **Acceptance Criteria (acceptance.md:1-376)** — All 8 ACs use Given-When-Then (BDD/Gherkin) format, not any of the five EARS patterns. Per MP-2, "Given/When/Then test scenarios mislabeled as EARS = FAIL." The spec.md §5 brief AC summaries (spec.md:287-294) are also non-EARS descriptive phrases, not EARS-structured criteria.

- **[FAIL] MP-3 YAML Frontmatter Validity**: Two required fields are absent:

  1. `created_at` — ABSENT. The frontmatter uses `created: "2026-05-21"` (spec.md:6) but the required field name is `created_at`. A different field name does not satisfy the requirement.
  2. `labels` — ABSENT. No `labels` key exists anywhere in the frontmatter (spec.md:1-12). Field entirely missing.

  Both fields are required per MP-3. Two missing required fields = FAIL.

- **[N/A] MP-4 Section 22 Language Neutrality**: N/A — Python-only scope. All implementation changes are confined to `pipelines/`. Go code is consumer-only 0-diff. Single-language scope auto-passes.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 — Minor ambiguity in one or two requirements | Performance SLO contradiction (spec.md:356 vs research.md:539); ScoreTrigger `summary={...}` undefined at acceptance.md:32 |
| Completeness | 0.50 | 0.50 — Multiple sections missing or substantively empty; YAML missing two required fields | YAML `created_at` and `labels` missing; 6 OPEN items unresolved within the SPEC itself; EmbeddingService confirmed unread (research.md:264) |
| Testability | 0.50 | 0.50 — Several ACs contain weasel words or require judgment calls | EC-10 (acceptance.md:300-301) has explicitly undecided behavior; ACs use GWT not binary-testable EARS; `summary={...}` vague parameter |
| Traceability | 0.75 | 0.75 — One AC references a REQ that exists but mapping is indirect | AC-INGEST-001-8 (acceptance.md:228) traces to REQ-UBI-002, which is a cross-SPEC constraint not defined in spec.md Section 4; all 5 in-SPEC REQs have at least one AC |

---

## Defects Found

### Critical Defects (must fix before Run)

**C1. spec.md:6 — YAML field name `created` must be `created_at`**
The frontmatter uses `created: "2026-05-21"` but the required field name is `created_at`. This is a field name mismatch, not a value error. The required field is absent.
Severity: critical (MP-3)

**C2. spec.md:1-12 — YAML field `labels` is entirely absent**
No `labels` key exists in the frontmatter. Required field completely missing.
Severity: critical (MP-3)

**C3. spec.md:217-232 — REQ-INGEST-003 mixes Event-driven and Unwanted EARS patterns**
The requirement opens with Event-driven ("WHEN...THE SYSTEM SHALL") and then appends an Unwanted condition ("IF Go 채점 API가 503/504/네트워크 오류를 반환하면, THEN THE SYSTEM SHALL"). These must be split into two separate REQs: one Event-driven for the happy path trigger, one Unwanted for the failure handling.
Severity: critical (MP-2)

**C4. spec.md:235-260 — REQ-INGEST-004 mixes State-driven and Unwanted EARS patterns**
Opens with State-driven ("WHILE _execute()가 정상 실행 경로를 따르는 동안, THE SYSTEM SHALL") and adds an Unwanted condition ("IF 부분 실패가 발생하면, THEN THE SYSTEM SHALL"). Must be split into two separate REQs with separate REQ IDs.
Severity: critical (MP-2)

**C5. spec.md:264-279 — REQ-INGEST-005 is mislabeled Ubiquitous and mixes three EARS types**
Labeled "Ubiquitous" but contains a temporal condition ("Celery worker 시작 시"). Ubiquitous means the system shall always do this — no conditions. Additionally mixes IF/Unwanted and WHERE/Optional sub-clauses in the same block. Correct pattern: split into (a) Event-driven "WHEN Celery worker starts, THE SYSTEM SHALL call validate_llm_endpoint()", (b) Unwanted "IF vlm_endpoint specifies a non-allowlist host, THE SYSTEM SHALL raise ExternalLLMBlockedError", (c) State-driven "WHILE vlm_endpoint is empty string, THE SYSTEM SHALL operate in CPU fallback mode."
Severity: critical (MP-2)

**C6. acceptance.md:1-376 — All ACs use Given-When-Then format, not EARS**
The acceptance criteria throughout acceptance.md use Gherkin-style Given/When/Then format. Per MP-2, "Given/When/Then test scenarios mislabeled as EARS = FAIL." EARS acceptance criteria must use one of the five EARS patterns: Ubiquitous, Event-driven, State-driven, Optional, or Unwanted. The spec.md §5 summaries (spec.md:287-294) are also informal descriptive phrases without EARS structure.
Severity: critical (MP-2)

**C7. spec.md:327-329 / research.md:264 — OPEN #3: EmbeddingService existence unverified, blocks REQ-INGEST-002**
REQ-INGEST-002 (spec.md:197-213) requires calling `EmbeddingService.embed()`. Research.md §5.3 explicitly states: "STATUS: Not read. Assumed to provide" for `pipelines/mapping/embedding_service.py`. This means the core module required by a must-implement REQ was never verified to exist. OPEN #3 acknowledges this and proposes resolution "Plan 초반 (Phase A 진입 전)" — but OPEN items should be resolved before the SPEC exits planning. If EmbeddingService does not exist, REQ-INGEST-002 cannot be implemented as written and a new SPEC scope decision is required.
Severity: critical (blocking)

**C8. spec.md:311-316 — OPEN #1: Scoring trigger endpoint undecided, blocks REQ-INGEST-003**
REQ-INGEST-003 requires `POST {go_control_plane_url}/api/v1/scores` or "동등 trigger 엔드포인트." Three candidate endpoints are listed (spec.md:313-316), one of which ("POST /api/v1/workflows/{workflow_id}/trigger-scoring") would require Go changes and violate consumer-only [HARD]. The endpoint is not decided, meaning ScoreTrigger.fire() cannot be correctly implemented. Resolution deferred to "Plan 후반 (annotation cycle)" — but planning should be complete before Run.
Severity: critical (blocking)

---

### Major Issues (fix recommended)

**M1. spec.md:356 vs research.md:539 — Performance SLO contradiction**
spec.md §7.1 states: "전체 _execute() 처리: p99 < 30s/문서 (CPU 환경, 10페이지 PDF 기준)." research.md §10.4 states: "CPU environment: p99 < 20s per page." If p99 < 20s/page on CPU, a 10-page document requires up to 200s, which is irreconcilable with a 30s total SLO. One of these numbers is wrong. The SPEC and its own research document are contradictory on the primary performance constraint.
Severity: major

**M2. acceptance.md:228 — AC-INGEST-001-8 traces to REQ-UBI-002 which is not defined in spec.md Section 4**
AC-INGEST-001-8 has "REQ 매핑: REQ-UBI-002 (횡단 제약)". REQ-UBI-002 is referenced in spec.md §1.3 as a cross-cutting constraint but is not defined in the REQUIREMENTS section (spec.md:176-280). Acceptance criteria must trace to REQs defined in the document. REQ-UBI-002 is a cross-SPEC constraint. Either define REQ-UBI-002 explicitly in this SPEC's requirements section or remap AC-INGEST-001-8 to an INGEST REQ that encompasses the Korean-language requirement.
Severity: major

**M3. acceptance.md:300-301 — EC-10 has undecided behavior**
Edge case EC-10 ("부분 chunks 실패") explicitly states "결정 필요: Plan Phase B/C에서 결정 (현재 OPEN, 옵션 A 권장)." An acceptance criterion that has two mutually exclusive options and deferred decision cannot be verified binary-pass/fail. A tester cannot determine the expected outcome. This edge case must be decided before Run.
Severity: major

**M4. acceptance.md:32 — ScoreTrigger.fire() `summary` parameter undefined**
AC-INGEST-001-1 shows `ScoreTrigger.fire(workflow_id="wf-uuid", document_id="doc-uuid", summary={...})` but `summary={...}` is never defined. What fields does summary contain? This leaves the API contract for ScoreTrigger ambiguous between acceptance.md and spec.md §3.2, which shows `ScoreTrigger.fire(workflow_id, document_id, summary)` without specifying the summary structure.
Severity: major

---

### Observations (no action required)

**O1. spec.md:248 / research.md:467-469 — result_json schema minor inconsistency with research.md guidance**
spec.md REQ-INGEST-004 defines result_json as `{document_id, chunks, tokens, score_triggered, ocr_backend, pages_processed, spec}`. research.md §9.3 says "Always include in result_json: document_id, pages_processed, ocr_confidence, criterion_matches, error." Fields `ocr_confidence` and `criterion_matches` appear in research guidance but not in REQ-INGEST-004 schema. The SPEC supersedes research, so this is not a defect — but the implementer should be aware that research.md §9.3 guidance is partially overridden.

**O2. spec.md:62-67 — OPEN #2 and OPEN #4 are secondary to OPEN #1**
OPEN #2 (Python→Go auth token) and OPEN #4 (DocumentMetadataClient data source) are interdependent with OPEN #1. These cannot be decided until OPEN #1 determines the trigger endpoint. The dependency chain OPEN #1 → OPEN #2 is implicit but correct given the spec structure.

**O3. plan.md:349 — Test count estimate of 48 tests appears achievable**
The test count breakdown (plan.md §6.2) shows 48 tests across 8 test files. Given the scope (4 new modules + 1 modified module), this is plausible for TDD approach. No inflation detected.

**O4. plan.md:319-325 — Delta markers are complete and accurate**
[MODIFY], [NEW], [EXISTING], [FROZEN] markers are consistently applied. Go frozen scope exactly matches the consumer-only [HARD] requirement. The `server.go` mount route is correctly added to the frozen list (plan.md:325).

---

## Chain-of-Verification Pass

Second-look findings:

1. **REQ-INGEST-001 embedded Unwanted condition** — Re-read spec.md:180-193. Line 186-187: "OCR 텍스트가 빈 문자열이면 IngestionEmptyError 발생" is embedded as item 4 of a numbered list under an Event-driven REQ. This is a sub-clause Unwanted condition inside an Event-driven block. While less severe than C3/C4/C5 (it's a sub-item, not a full EARS block), it still represents an EARS type mixing. However, the primary REQ structure is Event-driven and this sub-item is expressed as a constraint inline. Flagged but counted within C3/C4/C5 analysis — the auditor notes this pattern appears across multiple REQs consistently, suggesting the author's intent was to bundle related behaviors. All such bundles must be split per EARS rules.

2. **REQ number sequencing verified end-to-end** — Confirmed all 5 REQ IDs: REQ-INGEST-001 (L178), REQ-INGEST-002 (L197), REQ-INGEST-003 (L217), REQ-INGEST-004 (L235), REQ-INGEST-005 (L264). No gaps. No duplicates. MP-1 PASS confirmed.

3. **Exclusions section specificity** — spec.md §1.2 OUT of scope list (L41-52) contains 12 specific items, all concrete and unambiguous. No vague exclusions like "out of scope items." SC-6 PASS confirmed.

4. **Cross-SPEC parent ref consistency** — SPEC-AX-INTEG-001, SPEC-AX-PIPE-001, SPEC-AX-SCORE-API-001 are referenced consistently across spec.md, plan.md, and research.md. No naming inconsistency found.

5. **OPEN #2 auth method concern re: REQ-UBI-003** — The auth token delivery for ScoreTrigger (OPEN #2) must preserve user_id='cli-anonymous' per REQ-UBI-003. The spec acknowledges this at spec.md:324. No contradiction, but the resolution of OPEN #2 must verify REQ-UBI-003 compatibility — candidate C (gRPC bypass) is already flagged as [HARD] violation risk. Noted but not a new defect.

6. **DoD performance SLO gap** — plan.md §7 DoD (plan.md:361-376) does not include the p99 < 30s performance SLO as a verifiable gate. Combined with M1 (SLO contradiction), this means neither the spec nor the DoD would catch a performance regression. New defect confirmed as M1 (major).

No additional critical defects found beyond the 8 already catalogued.

---

## Checklist Results

| Category | Status | Notes |
|---|---|---|
| EARS Compliance (MP-2) | FAIL | REQ-INGEST-003/004/005 mix EARS types; all ACs in GWT format, not EARS; REQ-INGEST-005 mislabeled Ubiquitous |
| AC Testability | FAIL | EC-10 undecided behavior; `summary={...}` undefined; GWT format not binary-testable per EARS standard |
| Scope Integrity | PASS | consumer-only [HARD] explicitly stated; Go frozen scope in plan.md:319-325 exact; Python-only delta markers complete |
| Constraint Completeness | PARTIAL | REQ-UBI-001/002/003 explicitly stated (spec.md:62-67); OPEN #1 and OPEN #3 leave REQ-INGEST-002 and REQ-INGEST-003 unimplementable without further decisions |
| Plan Feasibility | PARTIAL | Phase sequence A→B/C/E→D→F is logical; 48 test estimate plausible; OPEN #1/#2/#3/#4 mean Phase A is a prerequisite gate, not optional; if OPEN #3 spawns separate SPEC, scope shifts |
| DoD Completeness | PARTIAL | consumer-only 0-diff check present; performance SLO not in DoD; OPEN resolution documented but no gate on EmbeddingService existence |
| Cross-SPEC Consistency | PASS | Parent SPEC references consistent; SCORE-API-001 endpoint trigger correctly identified as OPEN; INTEG-001 callback contract correctly preserved |
| YAML Frontmatter (MP-3) | FAIL | `created_at` missing (uses `created`); `labels` missing entirely |

---

## Recommendation

**This SPEC must NOT proceed to Run phase until the following are resolved:**

### Fix 1 (MP-3): YAML Frontmatter correction
Add the two missing required fields to spec.md frontmatter:
```yaml
created_at: "2026-05-21"   # rename from `created`
labels: ["ingestion", "vlm", "rag", "python"]
```
Remove the `created` field. The `updated` field may be retained as supplementary.

### Fix 2 (MP-2): Split mixed-EARS REQs into separate requirements

**REQ-INGEST-003** must be split into:
- REQ-INGEST-003: "WHEN REQ-INGEST-002가 VectorStore.upsert()를 성공 반환하면, THE SYSTEM SHALL ScoreTrigger.fire()를 호출한다." (Event-driven, happy path)
- REQ-INGEST-006 (new): "IF ScoreTrigger.fire()가 503/504/네트워크 오류를 반환하면, THEN THE SYSTEM SHALL ERROR 로그를 기록하고 raise하지 않으며 score_triggered=false로 표시한다." (Unwanted)

**REQ-INGEST-004** must be split into:
- REQ-INGEST-004: "WHILE _execute()가 정상 실행 경로를 따르는 동안, THE SYSTEM SHALL 정의된 스키마의 result_json을 반환한다." (State-driven)
- REQ-INGEST-007 (new): "IF _execute() 중 부분 실패가 발생하면, THEN THE SYSTEM SHALL result_json에 error 필드를 추가하고 callback status='failed'로 호출한다." (Unwanted)

**REQ-INGEST-005** must be restructured as:
- REQ-INGEST-005: "WHEN Celery worker가 시작될 때, THE SYSTEM SHALL validate_llm_endpoint(settings.vlm_endpoint)를 호출한다." (Event-driven)
- REQ-INGEST-008 (new): "IF vlm_endpoint이 allowlist 밖의 호스트를 지정하면, THEN THE SYSTEM SHALL ExternalLLMBlockedError를 발생시키고 worker를 시작하지 않는다." (Unwanted)
- REQ-INGEST-009 (new): "WHILE vlm_endpoint이 빈 문자열인 동안, THE SYSTEM SHALL transformers CPU fallback 모드로 동작한다." (State-driven)

After splitting, REQ numbers must remain sequential (001-009 with no gaps).

### Fix 3 (MP-2): Convert ACs from Given-When-Then to EARS format
Each of the 8 ACs in acceptance.md and their summaries in spec.md §5 must be expressed using EARS patterns. Example conversion for AC-INGEST-001-1:

Before (GWT): `Given... When... Then...`

After (EARS): "WHEN _execute('doc-uuid', workflow_id='wf-uuid')가 호출되고 DocumentMetadataClient가 {file_type: PDF}를 반환하면, THE SYSTEM SHALL chunks > 0인 result_json을 반환하고 VectorStore.upsert()를 호출하고 post_callback(status='completed')를 호출한다."

### Fix 4 (C7): Resolve OPEN #3 before Run phase
Verify that `pipelines/mapping/embedding_service.py` exists and provides `EmbeddingService.embed(text: str) -> list[float]` returning 768-dim vector. If it does not exist, either:
(a) Include EmbeddingService implementation in this SPEC's scope (expand spec.md §1.2 IN scope and add a REQ), or
(b) Create a dependency SPEC-AX-EMBED-001 and mark SPEC-AX-INGEST-001 as blocked until SPEC-AX-EMBED-001 Run completes.
Document the resolution in spec.md §6.2 OPEN #3.

### Fix 5 (C8): Resolve OPEN #1 before Run phase
Decide the scoring trigger endpoint. Only options compatible with consumer-only [HARD] are eligible. If `POST /api/v1/scores` (score creation) is chosen, document the exact request body schema and auth mechanism. Remove all non-viable candidates from the OPEN item and close it.

### Fix 6 (M1): Resolve performance SLO contradiction
Choose one consistent SLO statement:
- Either: "p99 < 20s per page, no total document SLO" (consistent with research.md §10.4)
- Or: "p99 < 30s per document for <= 5 pages on CPU" (revise the 10-page scenario)
Update both spec.md §7.1 and the DoD to include the SLO as a verifiable gate.

### Fix 7 (M2): Remap AC-INGEST-001-8 to an in-SPEC REQ
Either define REQ-UBI-002 content explicitly in spec.md Section 4 (as REQ-INGEST-00X covering Korean-language error messages), or remap AC-INGEST-001-8 to REQ-INGEST-004 which already references "오류 메시지는 한국어 우선 (REQ-UBI-002)."

### Fix 8 (M3): Decide EC-10 behavior
Decide between Option A (fail all chunks if any fail) and Option B (upsert successful chunks only). Remove the "결정 필요" deferral from acceptance.md EC-10. The accepted behavior must be deterministic before testing can verify it.

### Fix 9 (M4): Define `summary` parameter for ScoreTrigger.fire()
Add a concrete schema for the `summary` dict parameter in ScoreTrigger.fire(). At minimum specify: field names, types, and whether any fields are optional. Update acceptance.md AC-INGEST-001-1:32 and spec.md §3.2.
