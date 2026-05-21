# SPEC-AX-INGEST-001 Plan Audit — Iteration 3

Iteration: 3/3
Verdict: PASS
Overall Score: 0.87

---

## Critical Defects (C-level) — blocks PASS

None.

All four MUST-PASS checks clear:
- MP-1 REQ numbering: 001, 002, 003, 003b, 004, 004b, 004c, 005, 005b, 005c — sequential with established b/c suffix pattern, no gaps, no duplicates. (spec.md L179-338)
- MP-2 EARS format: all 10 REQs carry exactly one EARS trigger type (verified below).
- MP-3 YAML frontmatter: id, version, status, created, updated, author, priority, issue_number all present; matches project-canonical 8-field schema confirmed from SPEC-AX-AUDIT-QUERY-001. (spec.md L1-11)
- MP-4 Language neutrality: N/A — SPEC is scoped to Python-only ingestion worker (not multi-language tooling).

---

## Minor Issues (M-level) — does not block

**M1. plan.md:L360 — DoD REQ count claims 9 but enumerates 10.**

"9개 REQ 모두 EARS 형식으로 구현 (v0.1.2: 004b/004c 분리 포함; 001/002/003/003b/004/004b/004c/005/005b/005c 중 핵심 9개)"

The parenthetical enumerates exactly 10 REQ IDs (001, 002, 003, 003b, 004, 004b, 004c, 005, 005b, 005c). The qualifier "핵심 9개" does not identify which REQ is excluded. The iter2 fix updated 8→9, but the C1 split (004b→004b+004c) added one more, making the correct count 10. Severity: minor.

Fix: Change "9개 REQ" to "10개 REQ" and remove or justify the "핵심 9개" qualifier.

**M2. acceptance.md:L6 — version reference is stale (v0.1.0 vs spec.md v0.1.2).**

"**관련 SPEC:** SPEC-AX-INGEST-001 v0.1.0"

The spec is at v0.1.2. The acceptance.md header was not updated across v0.1.1 or v0.1.2 revisions. Severity: minor.

Fix: Update to `SPEC-AX-INGEST-001 v0.1.2`.

**M3. acceptance.md:L366 — DoD says "5개 REQ" but spec.md has 10 REQs.**

"1. **기능 완성**: 5개 REQ 모두 EARS 형식으로 구현 + 8개 AC 모두 단위 테스트 GREEN"

The original count was 5 (v0.1.0). After v0.1.1 expansion and v0.1.2 C1 split, spec.md now contains 10 REQs. The plan.md DoD was updated (iter2 M1 fix) but acceptance.md DoD was not touched. A developer using acceptance.md as the completion gate could declare Done prematurely. Severity: minor (plan.md DoD is more authoritative for implementation, and spec.md's 10 REQs are unambiguous, but the discrepancy creates confusion).

Fix: Update acceptance.md DoD item 1 to "10개 REQ" and note the REQ list explicitly.

**M4. acceptance.md:L24 — AC-INGEST-001-1 Given uses "HTTP 200" but spec.md REQ-INGEST-003:L225 specifies 201.**

acceptance.md AC-INGEST-001-1 Given: "ScoreTrigger.fire()가 HTTP 200 반환"
spec.md REQ-INGEST-003 (L225): "응답 성공 형식: 201 + `{\"score_id\": \"...\", \"status\": \"DRAFT\"}`"

The golden path test fixture uses HTTP 200 while the actual Go API is specified to respond with 201. HTTP 200 is a valid 2xx and would set `score_triggered=True` correctly, but the test mock does not reflect the documented API contract. Severity: minor (functional behavior is unaffected, but test fixture diverges from API spec).

Fix: Change AC-INGEST-001-1 Given to "ScoreTrigger.fire()가 HTTP 201 반환" to match REQ-INGEST-003 L225.

**M5. spec.md §6.2 — OPEN #2, #4, #5, #6 unresolved; OPEN #2 is referenced in a normative REQ constraint.**

spec.md REQ-INGEST-003 (L227): "인증: SPEC-AX-SCORE-API-001 write-role guard 통과 필요 — Python worker의 인증 토큰 전달 방식은 §6 OPEN #2"

OPEN #2 (auth token method) is named in the constraint section of REQ-INGEST-003, making the REQ's implementation incomplete until resolved. OPEN #4, #5, #6 similarly affect implementation decisions. All four OPENs are explicitly scheduled for plan.md Phase A, so they are tracked deferred decisions, not overlooked gaps. Severity: minor (draft-status SPEC with documented resolution plan is acceptable; risk is noted in spec.md §9).

No fix required before implementation begins; Phase A resolution is mandatory before Phase B entry.

---

## Verification of iter2 Fixes

**C1 — REQ-INGEST-004b dual-trigger split: CONFIRMED.**

spec.md REQ-INGEST-004b (L275): "IF `_execute()` 실행 중 VLM OCR 또는 VectorStore.upsert() 단계에서 복구 불가능한 오류가 발생하면, THEN THE SYSTEM SHALL..." — single IF trigger, general failure only.

spec.md REQ-INGEST-004c (L288): "IF 임베딩 단계에서 특정 청크의 임베딩이 실패하면 (EC-10 시나리오), THEN THE SYSTEM SHALL..." — single IF trigger, chunk-level failure only.

Both are now pure Unwanted Behavior REQs with one trigger each.

**C2 — REQ-INGEST-005 label and wording changed to Event-driven: CONFIRMED.**

spec.md L301: "### REQ-INGEST-005: 부팅 시 LLM 엔드포인트 검증 강제 (Event-driven)"
spec.md L303: "**WHEN** Celery worker가 시작될 때, **THE SYSTEM SHALL**..."

Previously labeled (Ubiquitous) with incorrect trigger structure. Now correctly Event-driven.

**C3 — acceptance.md EC-2 SLO updated to 200s: CONFIRMED.**

acceptance.md EC-2 (L267-268): "정상 처리, chunks 다수, p99 < 200s/문서 (CPU, 10페이지 PDF 순차 처리) 초과 가능 — timeout 적용"

Consistent with spec.md §7.1 (L412): "p99 < 200s/문서 (CPU 환경, 10페이지 PDF 순차 처리 기준; GPU 환경 p99 < 20s/문서)". The "초과 가능" note correctly explains that a 100-page PDF can exceed the 10-page baseline SLO.

**C4 — plan.md HTTP 200 → HTTP 2xx: CONFIRMED.**

plan.md Phase C ScoreTrigger (L89): "- HTTP 2xx → `score_triggered=True`"

Consistent with spec.md REQ-INGEST-003 (L228): "`score_triggered=true` 조건: HTTP 2xx 응답만 (3xx/4xx/5xx 모두 false)".

**M1 (iter2) — plan.md DoD REQ count 8→9: PARTIALLY CONFIRMED.**

plan.md L360: "9개 REQ 모두 EARS 형식으로 구현" — the fix changed 8→9. However, v0.1.2 introduced REQ-INGEST-004c (C1 fix), bringing the actual count to 10. The fix was applied but is now one count behind. This is the root cause of the new M1 defect above.

**M6 (iter2) — acceptance.md EC-10 defined behavior: CONFIRMED.**

acceptance.md EC-10 (L298-300): "실패 청크 skip + WARNING 로그 (청크 인덱스 포함) + `failed_chunks=2` (result_json); 성공한 3개 청크만 upsert; callback `status=\"completed\"` (REQ-INGEST-004c)"

EC-10 now has complete, testable behavior referencing REQ-INGEST-004c.

---

## EARS Format Detail — All 10 REQs

| REQ | Declared Type | Trigger Keyword | Single Trigger | Verdict |
|-----|--------------|-----------------|---------------|---------|
| REQ-INGEST-001 | Event-driven | WHEN (L181) | Yes | PASS |
| REQ-INGEST-002 | Event-driven | WHEN (L200) | Yes | PASS |
| REQ-INGEST-003 | Event-driven | WHEN (L220) | Yes | PASS |
| REQ-INGEST-003b | Unwanted | IF (L237) | Yes | PASS |
| REQ-INGEST-004 | State-driven | WHILE (L251) | Yes | PASS |
| REQ-INGEST-004b | Unwanted | IF (L277) | Yes | PASS |
| REQ-INGEST-004c | Unwanted | IF (L290) | Yes | PASS |
| REQ-INGEST-005 | Event-driven | WHEN (L303) | Yes | PASS |
| REQ-INGEST-005b | Unwanted | IF (L316) | Yes (compound AND condition within single IF) | PASS |
| REQ-INGEST-005c | State-driven | WHILE (L330) | Yes | PASS |

Note on REQ-INGEST-005b (L316): "비어 있지 않고 allowlist 밖의 호스트를 지정하면" is a compound AND condition within a single IF trigger. Compound conditions within a single EARS trigger are permitted; this is not a multi-trigger violation.

---

## Chain-of-Verification Pass

Second-look findings: two additional minor issues found beyond first pass.

Verified by re-reading the following sections:
- All 10 REQ entries in full (not skimmed) — no additional EARS violations found
- acceptance.md DoD section (L362-375) — discovered M3 (stale "5개 REQ" count)
- acceptance.md AC-INGEST-001-1 Given section — discovered M4 (HTTP 200 vs 201 mismatch)
- plan.md DoD (L360) — confirmed M1 (9 vs 10 REQ count)
- §6.2 OPEN items — all 4 unresolved OPENs noted as M5
- spec.md §7 non-functional requirements cross-checked against acceptance.md EC-2 — consistent

No C-level defects found in second pass.

---

## Regression Check (Iteration 3)

Defects from iteration 2 verified:

- **C1 (iter2)**: REQ-INGEST-004b dual-trigger — **RESOLVED** (split into 004b + 004c, each with single trigger)
- **C2 (iter2)**: REQ-INGEST-005 Ubiquitous/EARS mismatch — **RESOLVED** (now Event-driven with WHEN)
- **C3 (iter2)**: acceptance.md EC-2 SLO inconsistency — **RESOLVED** (200s/문서 CPU consistent)
- **C4 (iter2)**: plan.md HTTP 200 vs 2xx — **RESOLVED** (HTTP 2xx throughout plan.md)
- **M1 (iter2)**: plan.md DoD count 8→9 — **PARTIALLY RESOLVED** (changed to 9, but C1 split created a 10th REQ; count is still incorrect by one)
- **M6 (iter2)**: acceptance.md EC-10 undefined behavior — **RESOLVED** (behavior defined, references REQ-INGEST-004c)

All C-level defects from iterations 1 and 2 are resolved. No C-level defect recurs.

---

## Summary

All C-level defects from iterations 1 and 2 are confirmed resolved. The 10 REQs in spec.md v0.1.2 each carry exactly one EARS trigger, YAML frontmatter satisfies the project-canonical schema, and the Python-only consumer-only scope is enforced consistently across all three documents. Five minor (M-level) issues remain: a stale REQ count in plan.md DoD (9 vs actual 10), a stale REQ count in acceptance.md DoD (5 vs actual 10), an outdated version header in acceptance.md, an HTTP 200/201 fixture mismatch in AC-INGEST-001-1, and four unresolved OPEN design decisions in spec.md §6.2 that are explicitly planned for Phase A resolution. None of these M-level issues prevent implementation from proceeding correctly when plan.md and spec.md are read together.
