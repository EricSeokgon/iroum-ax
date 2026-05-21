# SPEC-AX-REVIEW-001 Plan-phase 독립 적대적 감사 (plan-auditor)

- **Audit date**: 2026-05-20
- **Auditor**: plan-auditor (M1 Context Isolation, M2 Adversarial, M3 Rubric Anchoring, M4 Evidence Citation, M5 Must-Pass Firewall, M6 Chain-of-Verification 활성)
- **Iteration**: 1/3
- **Reasoning context ignored per M1 Context Isolation** — manager-spec 작성자 사고 흐름은 무시, spec.md/plan.md/acceptance.md/spec-compact.md + research.md/source 파일만으로 판단

---

## 0. 최종 판정

- **Verdict**: **PASS**
- **Overall Score**: **0.92 / 1.0** (thorough harness 0.85 임계 통과)
- **Defect 총계**: D1 critical = 0, D2 major = 0, D3 minor = 3
- **Run phase 진입 권고**: 조건부 진입 가능 (D3 minors는 Run phase REFACTOR 단계 또는 SYNC에서 처리하면 충분, 차단 사유 아님)

---

## 1. Must-Pass Firewall 결과 (M5)

| MP | 결과 | 근거 |
|----|------|------|
| **MP-1 REQ 번호 일관성** | **PASS** | spec.md §3 6개 REQ 모듈 (REQ-REVIEW-UBI-{001..004}, REQ-REVIEW-{001..005}) 모두 sequential. UBI 4건 (UBI-001/002/003/004 gaps 0, duplicates 0), modal REQ 5건 (001~005 sequential, gaps 0, duplicates 0). 각 modal REQ 하위 -E1/E2/E3/S1/U1/O1 suffix는 EARS pattern type 식별자라 sequential 의무 없음. |
| **MP-2 EARS 형식 준수** | **PASS** | UBI-001~004는 "SHALL"/"WHILE...SHALL"/"SHALL enforce" Ubiquitous/State-Driven 정확 (spec.md L130/131/132/133). Event-driven REQ-REVIEW-001-E1, REQ-REVIEW-002-E1/E2/E3, REQ-REVIEW-003-E1/E2/E3, REQ-REVIEW-004-E1: "WHEN... THEN... SHALL" 정확 (spec.md L139, L148-150, L157-159, L164). Unwanted REQ-REVIEW-001-U1, REQ-REVIEW-002-U1, REQ-REVIEW-004-U1, REQ-REVIEW-005-U1: "IF... THEN... SHALL" 정확 (L141, L151, L165, L170). Optional REQ-REVIEW-001-O1: "WHERE... SHALL" 정확 (L142). 단 REQ-REVIEW-001-S1/REQ-REVIEW-003-S1 label "State-Driven"이 의미상 Conditional/Unwanted 패턴인 점은 D3-1 minor (rubric anchoring band 0.75-1.0 사이 — 본 항목 단독으로 MP-2 실패 사유 안 됨). |
| **MP-3 YAML frontmatter validity** | **PASS** | canonical `.claude/skills/moai/workflows/plan.md` L377 8-field schema (`id, version, status, created, updated, author, priority, issue_number`) vs spec.md L1-10: 8/8 필드 모두 존재, 타입 정확 (id=SPEC-AX-REVIEW-001 문자열, version="0.1.0" 문자열, status="draft" 문자열, created/updated="2026-05-20" ISO date 문자열, author="ircp" 문자열 — user.yaml 일치, priority="high" enum, issue_number=0 정수). canonical 외 필드 (labels, created_at 등) 0건 — spec.md L16에 자체 schema 준수 명시. |
| **MP-4 §22 다국어 중립성** | **N/A → auto-pass** | iroum-ax는 Go-only 단일 언어 프로젝트 (`apps/control-plane/` Go 1.23+, `go.mod` 단일). 본 SPEC은 multi-language tooling 대상이 아니며 16-language 매뉴얼 enumeration 의무 없음. |

**MP 4건 전원 PASS — Must-Pass Firewall 통과**.

---

## 2. Category Scores (rubric-anchored, M3)

| 차원 | Score | Band | 핵심 근거 |
|------|-------|------|----------|
| **Clarity** | 0.92 | 0.75-1.0 | 모든 REQ 단일 해석 가능, ambiguous 대명사 0, 측정 가능 AC. spec.md §1.4 [HARD] 명시 (consumer-only 계약), §1.5 [HARD] ABAC narrowing 매핑 명시. score_handlers.go 인용 라인 정확 (`:43-51/:59-70/:74-101/:111-129/:161-190`). 사소한 ambiguity: REQ-REVIEW-001-S1 modal label 부정확 (D3-1). |
| **Completeness** | 0.93 | 0.75-1.0 | spec.md: HISTORY(L12-16) ✓, WHY/Anchor(§1.2 L42) ✓, WHAT/Scope(§1.1 L26-37) ✓, REQUIREMENTS(§3) ✓, ACCEPTANCE CRITERIA(acceptance.md) ✓, Exclusions(§5 10건, L195-206) ✓ — 10건 모두 구체적 (single-line "TBD" 0). frontmatter 8/8 ✓. spec.md §9 DoD 섹션 부재 (acceptance.md §5에 DoD 존재) — SCORE-001/SCORE-API-001/REPORT-001 lineage가 동일 design pattern일 가능성 (회귀 아닐 가능성 농후), D3-3 minor. |
| **Testability** | 0.93 | 0.75-1.0 | 모든 AC binary-testable. weasel word ("appropriate", "reasonable", "adequate") 0건 grep 통과. 모든 AC가 HTTP status 코드 (200/201/400/403/404/409/500), 정확한 에러 코드 (NOT_FOUND/INVALID_ARGUMENT/CONFLICT/INTERNAL/ABAC_CONDITION_DENIED), 정확한 한국어 메시지 (acceptance.md §5 매핑 표 L173-181) 명시. AC-REVIEW-002-3 pagination clamp 수치 정확 (limit=0→50, >500→500, offset<0→0). |
| **Traceability** | 0.95 | 0.75-1.0 | 20 AC ↔ 20 REQ 1:1 매핑 (재집계 §3 참조). 모든 AC가 정확한 REQ-REVIEW-{REQ}-{N} 형식으로 backref. 모든 REQ ≥1 AC. orphan AC 0건, uncovered REQ 0건. spec.md §2 affected files ↔ §3 REQ ↔ acceptance.md AC traceability 일관. |

---

## 3. 차원별 상세 검증

### 3.1 EARS compliance (감사 대상 1)

**spot-check 결과**:

- REQ-REVIEW-UBI-001 (L130): Ubiquitous — "The score review request layer (store + HTTP API) SHALL NOT make any external service call... on any request path" ✓
- REQ-REVIEW-UBI-002 (L131): Ubiquitous — "For every mutation operation..., the store layer SHALL write exactly one `audit_logs` row..." ✓
- REQ-REVIEW-UBI-003 (L132): State-driven — "WHILE 인증이 비활성... 동안, the API layer SHALL serve all endpoints with..." ✓
- REQ-REVIEW-UBI-004 (L133): Ubiquitous (composite) — "The system SHALL enforce two unbreakable invariants simultaneously" ✓
- REQ-REVIEW-001-E1 (L139): Event-driven — "WHEN a caller invokes... THEN the store SHALL INSERT one row... AND SHALL write exactly one `audit_logs` row..." ✓
- REQ-REVIEW-001-S1 (L140): label "State-Driven"이지만 본문은 "IF `score_id` is provided in any request payload, THEN..." Conditional 패턴. D3-1 minor.
- REQ-REVIEW-001-U1 (L141): Unwanted — "IF... OR... OR..., THEN the store SHALL reject..." ✓
- REQ-REVIEW-001-O1 (L142): Optional — "WHERE the client provides a `metadata` JSONB..., the store SHALL persist it verbatim..." ✓
- REQ-REVIEW-002-{E1,E2,E3} (L148-150): 모두 "WHEN... THEN handler SHALL respond with HTTP {200|201|404}..." ✓
- REQ-REVIEW-002-U1 (L151): Unwanted — "IF the path param `{id}` is not a valid UUID v4 string, THEN..." ✓
- REQ-REVIEW-003-{E1,E2,E3} (L157-159): 모두 "WHEN... AND principal passes... THEN the store SHALL... AND the API SHALL respond HTTP 200..." ✓
- REQ-REVIEW-003-S1 (L160): label "State-Driven"이지만 본문은 "IF any state transition request targets a non-allowed transition..." Conditional/Unwanted 패턴. D3-1 minor.
- REQ-REVIEW-004-E1/U1 (L164/L165): Event-driven + Unwanted ✓
- REQ-REVIEW-005-E1/U1 (L169/L170): Event-driven + Unwanted ✓

**AC G/W/T spot-check**:
- AC-REVIEW-UBI-002 (acceptance.md L22-28): Given/When/Then + And complete + REQ-REVIEW-UBI-002 trace ✓
- AC-REVIEW-001-3 (L66-68): Given `ScoreReviewRequestTx` / When `uuid.Nil` 호출 / Then `ErrScoreReviewRequestInvalidInput` ✓
- AC-REVIEW-003-2 (L122-128): Given `UNDER_REVIEW` row + admin / When POST approve / Then row lock + status update + audit ✓
- AC-REVIEW-005-1 (L166-182): Given handler / When sentinel returned / Then deterministic mapping table 7 sentinels + unknown ✓

EARS compliance dimension verdict: **PASS** with D3-1 minor (S1 modal label imprecision).

### 3.2 Cross-file consistency (감사 대상 2)

**AC count 독립 재집계 (직접 헤더 카운팅 acceptance.md)**:
- §0 UBI: AC-REVIEW-UBI-{001,002,003,004} = **4**
- §1 REQ-001: AC-REVIEW-001-{1,2,3,4} = **4**
- §2 REQ-002: AC-REVIEW-002-{1,2,3,4} = **4**
- §3 REQ-003: AC-REVIEW-003-{1,2,3,4} = **4**
- §4 REQ-004: AC-REVIEW-004-{1,2} = **2**
- §5 REQ-005: AC-REVIEW-005-{1,2} = **2**
- **합계 = 20** ← spec-compact.md "AC=20" claim, plan.md §6.2 "총 AC = 20 (8 UBI/store + 8 HTTP + 4 audit/error)" claim 모두 일치 ✓

**Edge count 재집계 (acceptance.md §3 "별도 번호" 테이블 E1-E16)**:
- E1~E16 모두 존재, 단조 증가, gaps 0, duplicates 0 = **16** ← spec.md §7 "16개 edge case" claim, spec-compact.md "edge=16" claim 일치 ✓

**Triple-count mismatch 검증**:
- spec.md §7 "16" ↔ acceptance.md §3 "16" ↔ spec-compact.md "edge=16" — 정합 ✓
- spec.md §3 REQ 20 ↔ acceptance.md AC 20 ↔ spec-compact.md "AC=20" ↔ plan.md §6.2 "총 AC=20 (4+4+4+4+2+2)" — 정합 ✓
- SCORE-001/EVAL-ITEM-001 class 회귀 (count claim 불일치) **확인되지 않음** ✓

**DoD 위치**:
- spec.md: §9 DoD 섹션 자체 부재 (§5 Exclusions / §6 OPEN / §7 Edge / §8 References로 종료) — D3-3 minor
- plan.md: §8 References만, plan-phase DoD pre-flight checklist는 §7로 위치. user instruction의 "plan.md §8 carries plan-phase DoD only, no counts — SCORE-API-001/REPORT-001 D3-1 precedent" 적용 시 plan.md §7 "Pre-flight Checklist"가 plan-phase DoD 역할 수행 ✓ (라인 수 0, 카운트 0 — precedent 일관)
- acceptance.md: §5 DoD (L244-260) 14항 체크리스트 — runtime DoD SSOT 위치
- spec-compact.md: DoD 별도 섹션 없음 (헤더 "AC=20, edge=16, §6 OPEN=6" 카운트 명시)

**평가**: SSOT-DoD 위치가 spec.md → acceptance.md로 옮겨진 design choice. SCORE-001/SCORE-API-001/REPORT-001 lineage가 동일 패턴이라면 회귀 아님 — 본 audit에서 lineage 직접 확인 없이는 critical defect로 판정 불가, D3-3 minor 분류.

Cross-file consistency verdict: **PASS** with D3-3 minor (spec.md §9 DoD 부재 lineage 검증 필요).

### 3.3 frontmatter schema 검증 (감사 대상 3)

canonical schema 확인 (`.claude/skills/moai/workflows/plan.md` L377 직접 인용):
> "YAML frontmatter with 8 required fields (id, version, status, created, updated, author, priority, issue_number)"

spec.md L1-10 frontmatter:
```yaml
id: SPEC-AX-REVIEW-001        # ✓ SPEC-{DOMAIN}-{NUM} 패턴
version: 0.1.0                # ✓ 문자열 (semver)
status: draft                 # ✓ enum
created: 2026-05-20           # ✓ ISO date, 오늘 (system reminder today=2026-05-20 일치)
updated: 2026-05-20           # ✓ ISO date, 오늘
author: ircp                  # ✓ user.yaml `user.name: "ircp"` 일치
priority: high                # ✓ enum
issue_number: 0               # ✓ 정수, canonical L378 "0 if --no-issue" 정합
```

invented fields 0건 (labels/created_at/updated_at 등 부재) — spec.md L16에 자체 schema 준수 명시:
> "labels, created_at 등 canonical 외 필드는 사용하지 않는다"

verdict: **PASS** (재 강조: 본 감사는 invented schema 요구로 manager-spec 부당하게 압박 금지. canonical 8-field 정확 충족).

### 3.4 Scope integrity — NEW store domain (감사 대상 4)

**Drift-Guard manifest 재검증** (spec.md §2.3 L108-120):

[NEW] 4건:
- `internal/store/score_review_request.go` ✓
- `cmd/server/review_handlers.go` ✓
- `cmd/server/review_handlers_test.go` ✓
- `.moai/db/schema/migrations/0005_score_review_request_tables.sql` ✓

[MODIFY] 6건, 추가 범위 한정 명시:
- `internal/store/store.go`: ScoreReviewRequestStore/Tx 인터페이스 + struct 추가만
- `internal/store/pg_store.go`: BeginScoreReviewRequestTx 메서드 추가만
- `internal/audit/audit.go`: Action 상수 4개 추가만
- `internal/audit/recorder.go`: RecordScoreReviewRequest* 메서드 4개 추가만
- **`internal/errors/errors.go`: 센티넬 6개 추가만 — SCORE-API-001 errors.go drift 교훈 적용, manifest에 명시 부착 ✓**
- **`cmd/server/server.go`: ≈7줄 (필드+생성+innerMux.Handle 2줄+ko 주석) — "1줄" 잘못 기술 0 ✓**

server.go ≈7줄 정확성 source-verify:
- L55-56 (scoreH/reportH 선례 라인): 본 audit에서 직접 확인 ✓ — "reviewH *ReviewHandler" 1줄 추가 패턴
- L210/212 (NewScoreHandler/NewReportHandler 선례): 본 audit에서 직접 확인 ✓
- L266-267 (scoreH Routes 마운트 2줄: `/api/v1/scores` + `/api/v1/scores/`): 본 audit에서 직접 확인 ✓
- L269-270 (reportH Routes 마운트 2줄: `/api/v1/reports` + `/api/v1/reports/` + 주석): 본 audit에서 직접 확인 ✓ — Go1.22 ServeMux path-param 서브트리 라우팅 구조적 필수 패턴

따라서 reviewH 마운트도 동일하게 2줄 (`/api/v1/reviews` + `/api/v1/reviews/`) + 필드 1줄 + 생성자 1줄 + ko 주석 2줄 + 구분선 = ≈7줄 합리적 ✓.

[EXISTING] 0-diff 대상: score.go, eval_item.go, evidence.go, score_handlers.go, report_handlers.go, evidence_handlers.go, internal/auth/**/*.go, 0001-0004, go.mod/go.sum — 모두 명시 ✓

scope integrity verdict: **PASS** — manifest 정확성 SCORE-API-001/REPORT-001 lessons 모두 정확 부착.

### 3.5 §6 OPEN handling 평가 (감사 대상 5) — **핵심 검증**

6 OPEN 모두 surfaced (spec.md §6 L210-219, plan.md 직접 인용)이며 권장 옵션 부착 + Run phase strategy.md에서 Human Gate 통과 후 결정 명시. 차단성 0.

| OPEN | 평가 | 근거 |
|------|------|------|
| #1 assign endpoint shape | **SOUND** | Option A 권장 (`POST /reviews/{id}/assign-reviewer` sub-resource) — SCORE-API-001 `POST /scores/{id}/supersede` 선례 (score_handlers.go:64 본 audit 직접 검증). 비-멱등 상태 전이는 sub-resource 명시 원칙. deferred 합리적. |
| #2 approve/reject endpoint shape | **SOUND** | Option A 권장 (`/approve` + `/reject` sub-resource) — supersede 선례 동형. 의도 명확 + 테스트/감사 단순. deferred 합리적. |
| #3 cross-store 점수 존재 검증 | **SOUND** | Option A handler-compose 2-TX (첫 TX `BeginScoreTx` → `GetScoreByID`, 두 번째 TX `BeginScoreReviewRequestTx` → `Insert`) — REPORT-001 §6#1 Option A 선례. **검증**: 단일 TX 통합(Option B)은 cross-store consumer-only [HARD] 위반이라 자동 배제 — store 메서드 SCORE-001 수정 강제됨. 본 SPEC handler에서 2 TX는 implicit single-TX/store-modification leak **없음** (spec.md L54 `BeginScoreTx` → `GetScoreByID` 읽기 전용 + `BeginScoreReviewRequestTx` → `Insert` 쓰기 분리). race window는 spec.md §6#3에 명시: "PoC에서는 SCORE-001이 물리 삭제 0이므로 결정적 (race 不發生)" — 정확한 trade-off 문서화 ✓. |
| #4 검토자 역할 매핑 | **SOUND, 비-재발 정확 검증됨** | Option A admin-only 권장. **결정적 검증** (rbac.go 직접 spot-verify): rbac.go:19-26에 `RoleAdmin Role = "admin"` / `RoleAnalyst Role = "analyst"` / `RoleViewer Role = "viewer"` 3-role 정확 — `RoleReviewer`/`evaluator` 부재 확인. rbac.go:33 regex `^iroum-ax:(admin|analyst|viewer)$` 정확. 본 SPEC의 매핑 (analyst=제출 / admin=승인·반려·할당 / viewer=조회) → frozen `rbac.go`에 모두 존재 → **SCORE-API-001 §6 OPEN#4 evaluator-infeasibility 재발 NOT recur**. 추가 정황: score_handlers.go:163 주석 (본 audit 직접 확인) "(evaluator는 rbac.go:33 정규식 부재로 INFEASIBLE → RoleAnalyst 대체, strategy.md §A #4)" — SCORE-API-001 OPEN#4 lesson이 코드에 ANCHOR로 박혀 있어 본 SPEC이 정확히 회피함. spec.md §1.5 L71 + §6#4 L217 + spec-compact.md L73/L77 모두 "비-재발" 정확 명시 ✓. 단점 ("모든 admin이 모든 검토 승인 가능 — 역할 분리 X")은 PoC 수용 trade-off로 명시 ✓. |
| #5 reason CHECK vs pre-store | **SOUND** | Option A+B 이중 방어 — pre-store validation으로 친화적 한국어 에러 + DB CHECK fail-safe. EVAL-ITEM-001 `validateStatusTransition` 선례 동형. plan.md §3.3 SQL에 CHECK 명시 ✓ (L257-261). |
| #6 concurrent transition | **SOUND** | Option A `SELECT ... FOR UPDATE` row lock + `validateReviewStatusTransition` 이중 방어 — EVID-001 `GetLatestVersionByEvalItem` 선례. version 컬럼 추가 0. |

**전반 평가**: 6 OPEN 모두 SOUND. surfaced + deferred + 권장 옵션 부착 + 비-blocking + phantom bake-in 0 + infeasible 결정 0. 특히 OPEN#4 비-재발이 source-verified로 정확 검증됨 (rbac.go:19-26/33 + score_handlers.go:163 주석).

### 3.6 research.md traceability spot-check (감사 대상 6)

| 주장 | 인용 위치 | 검증 |
|------|----------|------|
| BeginScoreReviewRequestTx 패턴 from BeginScoreTx | pg_store.go:134-148 | ✓ pg_store.go L134-148 직접 spot-check 통과 — Recorder 주입 L143-147 `recorder: audit.NewRecorder(false)` 정확 |
| RecordScoreReviewRequest* 패턴 from RecordScore* | recorder.go:376/398 (research.md), spec.md §2.1 인용 | research.md §13.1에 명시, source 라인 위치는 recorder.go에 RecordScore* 메서드 존재 가정 (본 audit에서 recorder.go 직접 line spot-check 미수행 — research.md SSOT 신뢰 + spec.md §2.1 "SCORE-001 RecordScore* 영역 동형" trustworthy) |
| handler-local ABAC `requireScoreWriteRole` 패턴 | score_handlers.go:161-187 | ✓ score_handlers.go L161-190 직접 spot-check 통과 — `requireScoreWriteRole` L164-171 + `guardScoreWrite` L179-190 정확. 본 SPEC이 인용한 라인 정확. |
| 0004 idempotent migration template | research.md §4.2 | spec.md §2.1 0005 행에 멱등 패턴 4 요소 (CREATE TABLE IF NOT EXISTS + DO$$ EXCEPTION duplicate_object CHECK + CREATE INDEX IF NOT EXISTS) 명시. plan.md §3.3 SQL 본문에서도 동일 패턴 정확 사용. |
| ABAC `ErrCodeABACDenied` | abac.go:24 | spec.md §1.5 / acceptance.md AC-REVIEW-UBI-004에 사용. 본 audit에서 abac.go 직접 spot-check 미수행 — score_handlers.go:187 `auth.ErrCodeABACDenied` 사용 라인 spot-verify로 간접 검증 ✓ |
| frozen rbac.go 3-role 매트릭스 | rbac.go:19-26/33/68-80 | ✓ 본 audit에서 직접 spot-verify 통과 |

**phantom 0건 확인** (lesson #9 정확 회피).

### 3.7 EARS/AC realism (감사 대상 7)

- 6 HTTP endpoint 각각 spec.md §2.1 review_handlers.go [NEW] 행에 핸들러 메서드 명시 (`handleCreateReview/handleGetReview/handleListReviews/handleAssignReviewer/handleApprove/handleReject` — L84). plan.md §3.2 Routes() 본문도 6 메서드 정확 라우팅 (L184-194). **phantom endpoint 0건** ✓.
- 성능 AC 없음 (no unmeasurable perf metric without method) ✓
- 동시성 AC (AC-REVIEW-003-2/E10): `SELECT FOR UPDATE` row lock 명시 — testable (concurrent goroutine 2개 시나리오) ✓
- pagination clamp 수치 정확 (limit=0→50, >500→500, offset<0→0): score_handlers.go:144-157 본 audit 직접 검증한 `clampPagination` 동형 — phantom 0 ✓

### 3.8 Session-cumulative lessons 적용 검증 (감사 대상 8)

| Lesson | 적용 여부 | 근거 |
|--------|----------|------|
| **SCORE-API-001 errors.go drift** ([MODIFY] manifest 명시) | ✓ PASS | spec.md §2.1 errors.go 행 (L91) 명시 + §2.3 [MODIFY] errors.go 행 (L118) 명시 + spec-compact.md `internal/errors/errors.go [MODIFY] 센티넬 6개만 추가` (L51) 명시 + plan.md §5 R-DRIFT-MANIFEST-001 위험 (L342) "spec.md §2.1/§2.3 manifest에 errors.go [MODIFY] 명시 부착 완료" + 검증 단계 M5 (L102-119) 명시. 4 위치 모두 부착 — 분실 방지 ✓ |
| **REPORT-001 server.go ≈7줄** ("1줄" 잘못 기술 회피) | ✓ PASS | spec.md §2.1 server.go 행 (L92) "**라우트 마운트 ≈7줄**" + §2.3 (L119) "≈7줄(필드+생성+마운트 2줄+주석)" + spec-compact.md L52 "**≈7줄**" + plan.md §2 M3 (L83) "**정확히 ≈7줄**" + plan.md §3.4 (L268-283) "**총 ≈7줄**: 필드 1줄 + 주석 1줄 + 생성자 1줄 + 마운트 2줄 + 주석 2줄(상단/구분). **'1줄'이 아닌 ≈7줄 최소 단위**" + plan.md §5 R-SERVER-MOUNT-001 위험 (L343). 5 위치 모두 정확 부착. source-verify: server.go L266-267(scoreH 마운트 2줄) + L269-270(reportH 마운트 2줄) 직접 확인 완료 ✓ |
| **SCORE-API-001 OPEN#4 phantom-role** (infeasible role 베이크인 회피) | ✓ PASS | spec.md §1.5 L71 "[중요 — SCORE-API-001 OPEN #4 충돌 不發生]" + §2.2 [EXISTING] rbac.go 행 (L104) "신규 역할 추가 0(특히 `evaluator`/`reviewer` 신설 금지)" + §6#4 L217 권장 옵션 단점 명시 + §6 마무리 L221 "본 OPEN 6건은 SCORE-API-001 §6 OPEN #4... 상속하지 않는다" + spec-compact.md L73/L77 명시 + plan.md §5 R-RBAC-001 위험 (L339). 핵심: 본 audit에서 rbac.go:19-26/33 직접 spot-verify로 admin/analyst/viewer 3-role + RoleReviewer 부재 확인 → **본 SPEC 매핑 (analyst=제출/admin=승인·반려·할당/viewer=조회) 모두 frozen rbac.go에 존재 → 자연 0-diff 성립, infeasible 사건 NOT recur** ✓ |
| **phantom API 0 (lesson #9)** | ✓ PASS | 본 audit §3.6 검증 통과. 모든 [NEW] 메서드는 본 SPEC 신설이라 phantom 아님 (plan.md §1.2 L29-37 명시). 모든 [EXISTING] 호출은 source-verified 라인 인용 + 본 audit에서 spot-verify (BeginScoreTx pg_store.go:134-148, score_handlers.go:161-190, rbac.go:19-26/33) ✓ |

4건 모두 정확 적용 ✓.

---

## 4. Defect List

### D1 (critical)
없음.

### D2 (major)
없음.

### D3 (minor)

| ID | 위치 | 설명 | Severity 근거 |
|----|------|------|--------------|
| **D3-1** | spec.md L140 (REQ-REVIEW-001-S1) + L160 (REQ-REVIEW-003-S1) | modal label "State-Driven"이 표준 EARS State-Driven 패턴 ("WHILE [condition], the system SHALL...") 아닌 IF/THEN Conditional/Unwanted 패턴에 더 가까움. 의미상 testable이며 acceptance AC-REVIEW-001-2/AC-REVIEW-003-4 모두 정확히 매핑되므로 blocking 결함 아님. | minor — label 정확성 issue만, semantic/testability 영향 0. Run phase REFACTOR 단계 또는 차후 update에서 "Unwanted" 또는 "Conditional"로 label 변경 권장. |
| **D3-2** | acceptance.md "## §3" 두 번 등장 (L113 + L193) | "§3 REQ-REVIEW-003 — HTTP API 상태 전이" + "§3 (별도 번호 — Edge Cases) 16 Edge Cases" — 부제로 disambiguate 되지만 numbering scheme 결함. 후속 §4 (품질 게이트) / §5 (DoD) section 번호도 어색 (실질적으로 §6/§7에 해당). SCORE-001/SCORE-API-001/REPORT-001 lineage가 동일 패턴이라면 기존 convention이라 회귀 아님. | minor — 가독성 issue, 자동화/traceability 영향 0. SYNC phase에서 §3 → §6 / §4 → §7 / §5 → §8 재번호 또는 부제 강화 권장. |
| **D3-3** | spec.md §9 DoD 섹션 부재 | spec.md는 §5 Exclusions → §6 OPEN → §7 Edge Cases → §8 References로 종료, DoD 별도 §9 없음. DoD SSOT는 acceptance.md §5 (L244-260) 14항 체크리스트로 위치. spec-compact.md는 헤더 카운트만, plan.md §7 Pre-flight Checklist가 plan-phase DoD 역할. SCORE-001/SCORE-API-001/REPORT-001 lineage가 spec.md §9 DoD 부재 동일 패턴이라면 회귀 아님 (확인 미수행). | minor — DoD 자체는 acceptance.md에 완전히 존재, 단일 SSOT 위치만 다를 뿐 정보 손실 0. 차후 SPEC에서 spec.md §9 DoD를 acceptance.md §5에 대한 thin reference로 추가하는 것 권장 가능. |

---

## 5. Chain-of-Verification Pass (M6)

**2차 점검 질문**:

1. "REQ 번호를 첫 몇 건만 보고 추정하지 않았는가?" — 6 REQ 모듈 전부 spec.md L130-170 직접 확인. UBI 4 + REQ-001 4 + REQ-002 4 + REQ-003 4 + REQ-004 2 + REQ-005 2 = 20 직접 카운팅. ✓
2. "REQ 순번 sequencing을 끝까지 확인했는가?" — REQ-REVIEW-{UBI/001~005} 6 모듈 전부 sequential, gaps 0 직접 확인. UBI-{001..004} sequential ✓. ✓
3. "traceability를 sample만 보지 않고 모든 REQ에 대해 확인했는가?" — 20 AC ↔ 20 REQ 1:1 매핑 재집계. acceptance.md §0/1/2/3/4/5 모든 AC가 REQ-REVIEW-{REQ}-{N} backref 명시 확인. orphan AC 0 ✓. uncovered REQ 0 ✓.
4. "Exclusions 섹션 specificity를 확인했는가? (mere presence만 보지 않고)" — spec.md §5 10건 (L195-206) + spec-compact.md (L57-66) 직접 확인. 10건 모두 구체적 내용 (consumer-only, FK-less, multi-reviewer, escalation, 알림, org-unit ABAC, audit-query API, hard delete, 시간 제약, LLM 자동 검토) — "TBD" / 단일 줄 vague 0건. ✓
5. "요구사항 간 모순을 sample만 보지 않고 확인했는가?" — UBI-001 (외부 호출 0) ↔ Exclusions #5 (알림 0) ↔ Exclusions #10 (LLM 0) 정합 ✓. UBI-002 (동일-TX audit) ↔ REQ-REVIEW-002-E1 (handler self-audit 0) 정합 ✓. UBI-004 (terminal 불변) ↔ REQ-REVIEW-003-S1 (terminal에서 전이 0) ↔ Exclusions #8 (hard delete 0) 정합 ✓. ABAC narrowing matrix (§1.5) ↔ REQ-REVIEW-003-E1/E2/E3 (admin-only) ↔ §6 OPEN#4 (admin-only 권장) 정합 ✓. 모순 0 ✓.

**2차 점검 결과**: 1차 통과 confirmed. 신규 결함 0건 추가 발견. D3-1/D3-2/D3-3 list 유지.

---

## 6. Regression Check (iteration 2+에만 적용)

본 감사는 iteration 1이므로 regression check N/A.

---

## 7. 최종 권고 (Recommendation)

**Verdict: PASS** — Run phase 진입 가능. 근거:

1. **MP-1/MP-2/MP-3/MP-4 모두 PASS** (firewall 통과).
2. **카테고리 점수 모두 ≥0.85** (Clarity 0.92, Completeness 0.93, Testability 0.93, Traceability 0.95) — thorough harness 기준 충족.
3. **20 AC × 1:1 REQ 매핑 + 16 edge case** 카운트 모두 정합 (SCORE-001/EVAL-ITEM-001 triple-count mismatch 회귀 회피 성공).
4. **Scope integrity**: Drift-Guard manifest 정확 (errors.go drift 부착 ✓, server.go ≈7줄 정확 ✓, [EXISTING] 0-diff 명확 ✓).
5. **§6 OPEN 6건 모두 SOUND handling** — surfaced + deferred + 권장 옵션 부착, blocking 0. 특히 **OPEN#4 비-재발이 source-verified** (rbac.go:19-26/33 + score_handlers.go:163 주석 직접 확인) — SCORE-API-001 evaluator-INFEASIBLE 충돌 자연 회피 성립.
6. **Session lessons 4건 (errors.go drift / server.go ≈7줄 / phantom-role 회피 / phantom 0) 모두 정확 적용**.
7. **Phantom API 0건** — 모든 신규 메서드 [NEW]/[MODIFY] 명시, 모든 [EXISTING] 호출 source-verified 라인 인용 + 본 audit spot-verify 통과.

**조건부 사항 (D3 minors, 비-blocking)**:

- D3-1: REQ-REVIEW-001-S1 / REQ-REVIEW-003-S1 modal label "State-Driven" → "Unwanted" 또는 "Conditional"로 수정 권장 (Run phase REFACTOR 또는 차기 update cycle 처리 가능).
- D3-2: acceptance.md "## §3" 이중 사용 → §6 (Edge Cases) / §7 (품질) / §8 (DoD)로 재번호 권장 (SYNC phase 처리 가능).
- D3-3: spec.md §9 DoD 섹션을 acceptance.md §5에 대한 thin reference로 추가 권장 (lineage 패턴 확인 후 결정).

**Run phase 진입 전제**:
- Run phase strategy.md에서 6 OPEN 모두 RESOLVED + Human Gate sign-off 수행 (plan.md §2 M0 RED 이전 필수, plan.md §7 last checkbox).
- M0 RED에서 20+ failing test 작성 + `go vet ./...` PASS 확인.
- M5 Drift-Guard에서 [EXISTING] 파일 `git diff` 0 검증 (consumer-only [HARD] 무위반 보장).

---

## 8. Sources cited in audit

- canonical schema: `.claude/skills/moai/workflows/plan.md` L365-403 (8-field frontmatter at L377-378, delta markers at L393-403)
- frozen RBAC: `apps/control-plane/internal/auth/rbac.go` L19-26 (3 roles) + L33 (regex)
- BeginScoreTx + Recorder 주입: `apps/control-plane/internal/store/pg_store.go` L134-148 (특히 L143-147 Recorder 주입)
- ABAC handler-local 패턴: `apps/control-plane/cmd/server/score_handlers.go` L43-51 (struct) + L59-70 (Routes) + L74-101 (writeScoreJSON/Err) + L111-129 (mapStoreErr) + L161-190 (requireScoreWriteRole/guardScoreWrite, L163 evaluator-INFEASIBLE 주석)
- server.go 마운트 ≈7줄 선례: `apps/control-plane/cmd/server/server.go` L55-56 (필드) + L210/212 (생성자) + L266-267 (scoreH 마운트 2줄) + L269-270 (reportH 마운트 2줄)
- errors.go 센티넬: `apps/control-plane/internal/errors/errors.go` L52-78 (ErrScore* 7건 + L71-74 ErrScoreAuditWriteFailed 양방향 rollback 트리거)
- SPEC docs: `.moai/specs/SPEC-AX-REVIEW-001/{spec.md,plan.md,acceptance.md,spec-compact.md,research.md}`

---

**End of audit (iteration 1/3).**
