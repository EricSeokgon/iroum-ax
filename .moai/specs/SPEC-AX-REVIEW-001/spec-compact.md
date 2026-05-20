# SPEC-AX-REVIEW-001 (Compact) — 평가 제출/승인 워크플로우 저장소 + HTTP API 계층 (Score Review Request Store + HTTP API Layer)

> v0.1.0 · status draft · TDD · thorough harness · sub-agent · brownfield. 상세는 spec.md / plan.md / acceptance.md / research.md. (AC=20, edge=16, §6 OPEN=6, 본 SPEC=수직 슬라이스 store+audit+HTTP+ABAC 단일 SPEC)

## 핵심 stub 계약 [HARD]

- `score_review_requests.score_id` = `UUID NOT NULL` — **FK 없는 stub** (SCORE-001 `scores.id` UUID 참조, SCORE-001 §1.4 동형 — 점수 존재 검증은 핸들러 단계 cross-store 2-TX, REPORT-001 §6#1 Option A 정합)
- `score_review_requests.status` = state-machine enum `SUBMITTED|UNDER_REVIEW|APPROVED|REJECTED` (DB CHECK + store-level validateReviewStatusTransition 이중 방어)
- SCORE-001/EVAL-ITEM-001/EVID-001/AUTH-003 코드·스키마·FK·permissionMatrix·핸들러 = **0-diff [HARD] consumer-only**
- 신규 마이그레이션 = `0005_score_review_request_tables.sql` 1건 (0001/0002/0003/0004 디스크 확인 후 비충돌, 0004 멱등 패턴 정확 미러)
- 동일-TX audit 있음 (REPORT-001과 다름) — SCORE-001 D2/D4 패턴 미러: resource_id=`score_review_requests.id` UUID 직접 (surrogate 미사용)
- frozen `rbac.go` 3-role (`admin`/`analyst`/`viewer`) **0-diff** — 신규 역할 추가 0 (SCORE-API-001 §6 OPEN #4 같은 충돌 不發生 because 본 SPEC 매핑 명확)
- 1차 산출물 = store 도메인 신설 + 동일-TX audit + 6 HTTP 엔드포인트 + 핸들러-로컬 ABAC narrowing

## REQ 모듈 (6: 4 UBI + 5 modal × 6 endpoints)

| REQ-ID | EARS | 요지 |
|--------|------|------|
| REQ-REVIEW-UBI-001 (데이터 주권) | Ubiquitous | 외부 호출 0건 (외부 LLM/SaaS/CDN/secrets manager 모두 0), 단일 내부 PostgreSQL pgx pool만 |
| REQ-REVIEW-UBI-002 (감사 가능성) | Ubiquitous | 모든 mutation(생성/할당/승인/반려) → 동일 TX `audit_logs` 1건, resource_id=UUID 직접, 핸들러 self-audit 0 (이중 감사 금지) |
| REQ-REVIEW-UBI-003 (cli-anonymous + auth-disabled) | Ubiquitous(State) | AuthN off 시 `created_by`/`updated_by`/`audit_logs.user_id` = `'cli-anonymous'` literal (NULL 금지), 미들웨어 자동 투과 |
| REQ-REVIEW-UBI-004 (권한 narrowing + 4-상태 불변식) | Ubiquitous | (1) ABAC narrowing: analyst=제출/admin=승인·반려·할당/viewer+모든=조회, frozen rbac.go 0-diff; (2) 4-state allowed transitions only (`SUBMITTED→UNDER_REVIEW→APPROVED|REJECTED`, terminal 불변), 물리 DELETE 0 |
| REQ-REVIEW-001-E1/S1/U1/O1 | E/S/U/O | Store + 데이터 모델 + 마이그레이션: 동일-TX entity+audit(E1), score_id FK-less stub(S1), blank/invalid 거부(U1), metadata opaque(O1) |
| REQ-REVIEW-002-E1/E2/E3/U1 | E/E/E/U | HTTP create/get/list: analyst 생성(E1), 단건 조회(E2), 목록 + filter + pagination clamp(E3), malformed UUID 거부(U1) |
| REQ-REVIEW-003-E1/E2/E3/S1 | E/E/E/S | HTTP transitions: assign-reviewer SUBMITTED→UNDER_REVIEW(E1), approve UNDER_REVIEW→APPROVED terminal(E2), reject UNDER_REVIEW→REJECTED with reason(E3), 불법 전이 fail-closed(S1) |
| REQ-REVIEW-004-E1/U1 | E/U | Audit 연계: RecordScoreReviewRequest* 동일 TX(E1), audit fail 양방향 rollback(U1) |
| REQ-REVIEW-005-E1/U1 | E/U | 에러 매핑: 결정적 store→HTTP 매핑(E1, 7 센티넬 + unknown), raw pgx 누출 금지(U1) |

## AC (20: AC-REVIEW-{REQ}-{N})

- §0 UBI: AC-REVIEW-UBI-001(외부호출 0), -002(audit 동일 TX 원자), -003(cli-anonymous + auth-disabled 투과), -004(권한 narrowing + 4-state 불변식 + 물리 DELETE 0)
- §1 REQ-REVIEW-001: AC-REVIEW-001-1(entity+audit 원자 생성), -2(score_id FK-less stub), -3(blank/invalid 거부), -4(metadata opaque round-trip)
- §2 REQ-REVIEW-002: AC-REVIEW-002-1(analyst create 201), -2(단건 조회), -3(목록 + filter + pagination + 빈 결과 200), -4(malformed UUID 400)
- §3 REQ-REVIEW-003: AC-REVIEW-003-1(assign SUBMITTED→UNDER_REVIEW), -2(approve UNDER_REVIEW→APPROVED terminal), -3(reject + reason → REJECTED terminal), -4(불법 전이 모두 거부 fail-closed)
- §4 REQ-REVIEW-004: AC-REVIEW-004-1(RecordScoreReviewRequest* 동일 TX), -2(audit fail 양방향 rollback)
- §5 REQ-REVIEW-005: AC-REVIEW-005-1(7 센티넬 + unknown 결정적 매핑 + 한국어 메시지), -2(raw pgx.ErrNoRows 누출 0)
- 분해 합 = 4+4+4+4+2+2 = 20 = 물리 heading 20. §3 edge=16. 각 modal REQ ≥2 AC.

## Files

| 경로 | Delta |
|------|-------|
| `internal/store/score_review_request.go` | [NEW] PgScoreReviewRequestTx pgx (eval_item.go/score.go 미러) + validateReviewRequestInput + validateReviewStatusTransition (SCORE-001 D4 동형) |
| `cmd/server/review_handlers.go` | [NEW] ReviewHandler + Routes() + 6 핸들러 + 핸들러-로컬 ABAC (score_handlers.go:43-190 동형) |
| `cmd/server/review_handlers_test.go` | [NEW] httptest 20+ test (생명주기/ABAC/cross-store/edge/audit 원자) |
| `.moai/db/schema/migrations/0005_score_review_request_tables.sql` | [NEW] 멱등 SQL (0004 미러), CHECK 상태 enum + REJECTED reason CHECK, 인덱스 3개 |
| `internal/store/store.go` | [MODIFY] ScoreReviewRequestStore/ScoreReviewRequestTx 인터페이스 + ScoreReviewRequest struct (EvalItemStore/Tx 패턴 미러) |
| `internal/store/pg_store.go` | [MODIFY] BeginScoreReviewRequestTx (PgWorkflowStore.pool 재사용 + Recorder 주입, pg_store.go:143-147 동형) |
| `internal/audit/audit.go` | [MODIFY] ActionScoreReviewRequest{Created,ReviewerAssigned,Approved,Rejected} 상수 **4개만** (namespace 0, D2) |
| `internal/audit/recorder.go` | [MODIFY] RecordScoreReviewRequest{Created,ReviewerAssigned,Approved,Rejected} 메서드 **4개만** (local AuditTx, resource_id=UUID 직접) |
| `internal/errors/errors.go` | [MODIFY] 센티넬 **6개만** 추가 (SCORE-API-001 errors.go drift 교훈 — manifest에 명시 부착) |
| `cmd/server/server.go` | [MODIFY] **≈7줄** (필드+생성+innerMux.Handle 2줄+ko 주석, SCORE-API-001 :55/:210/:266-267 + REPORT-001 :56/:212/:269-270 정확 미러, "1줄" 잘못 기술 0) |
| `internal/store/score.go` / `eval_item.go` / `evidence.go` / `cmd/server/score_handlers.go` / `report_handlers.go` / `evidence_handlers.go` / `internal/auth/**/*.go` / `0001`–`0004` / `go.mod` | [EXISTING] 0-diff [HARD] |

## Exclusions (What NOT to Build)

1. **SCORE-001/EVAL-ITEM-001/EVID-001/AUTH-003 코드·스키마 수정** — consumer-only [HARD]
2. **`score_id` → `scores.id` 외래키 강제** — FK-less stub 유지 (SCORE-001 §1.4 동형)
3. **요청당 다중 검토자 (multi-reviewer)** — PoC = 검토자 1명, 다중/투표/합의는 post-PoC
4. **에스컬레이션 / 타임아웃 / SLA 워크플로우** — 시한 알람, 자동 에스컬레이션은 post-PoC
5. **알림 / 이메일 / 메신저 통합** — 외부 호출 0 (REQ-REVIEW-UBI-001 정합)
6. **AUTH-003 모델 초과 풀 org-unit 속성 ABAC** — 조직단위 narrowing 미구현
7. **검토 이력 시계열 조회 API / 변경 감사 UI** — 별도 audit-query SPEC 이연
8. **물리 삭제 / hard delete** — append-only (UBI-004 정합)
9. **6번째 한국 공공 제약 (시간 제약 — KST 업무시간 한정 승인)** — 의도적 제외 (SCORE-API-001/REPORT-001 동형)
10. **LLM 기반 자동 검토 / 추천 엔진** — 외부 호출 0, 결정적 처리만 (UBI-001 정합)

## §6 OPEN (strategy phase 결정 — sub-agent + Human Gate, 6건)

1. **검토자 할당 엔드포인트 형태**: Option A `POST /reviews/{id}/assign-reviewer` (별도 sub-resource, SCORE-API-001 supersede 선례) vs Option B `PUT /reviews/{id}` body embedded. **권장**: A (비-멱등 상태 전이는 sub-resource 명시).
2. **승인/반려 엔드포인트 형태**: Option A 별도 `POST /reviews/{id}/approve` + `/reject` vs Option B `PUT /reviews/{id}/status` 단일 + body status. **권장**: A (SCORE-API-001 supersede 선례, 의도 명확, 핸들러 분리로 테스트/감사 단순).
3. **Cross-store 점수 존재 검증**: Option A handler-compose 2-TX(첫 TX `BeginScoreTx` → `GetScoreByID`, 두 번째 TX `BeginScoreReviewRequestTx`, REPORT-001 §6#1 Option A 선례) vs Option B store-layer 단일 TX 통합(불가능 — cross-store consumer-only 위반). **권장**: A.
4. **검토자 역할 매핑**: Option A `admin-only` 승인/반려/할당 (frozen rbac.go `RoleAdmin` 사용, RoleReviewer 신설 금지) vs Option B `assigned_reviewer_id` 일치 시만 승인 (`abac.go` narrowing 정책 주입). **권장**: A (frozen RBAC 0-diff, SCORE-API-001 §6#4 같은 충돌 不發生 because admin은 rbac.go에 존재). 단점: 모든 admin이 모든 검토 승인 가능 — PoC 수용.
5. **Reason/Comment 작성 조건**: Option A DB CHECK constraint (`status != 'REJECTED' OR rejection_reason IS NOT NULL`) vs Option B pre-store validation. **권장**: A + B 이중 방어 (B로 한국어 에러, A로 fail-safe).
6. **Concurrent transition handling**: Option A `SELECT ... FOR UPDATE` (EVID-001 패턴) + validateStatusTransition 이중 vs Option B optimistic version 컬럼 단독. **권장**: A.

> 본 OPEN 6건은 **SCORE-API-001 §6 OPEN #4(write 역할 매핑 `evaluator` 부재) 상속하지 않음** — 본 SPEC의 역할 매핑(analyst/admin/viewer)은 frozen rbac.go에 모두 존재(`rbac.go:19-26`)하므로 0-diff 자연 성립.

## Drift-Guard Manifest Highlights (REPORT-001/SCORE-API-001 lessons 적용)

- **errors.go 부착 정확**: SCORE-API-001 교훈 — [MODIFY] 6 센티넬 추가를 manifest §2.1/§2.3 모두 명시 부착 (drift detection 분실 방지)
- **server.go ≈7줄 정확**: REPORT-001 교훈 — "1줄"이 아닌 ≈7줄(필드+생성+마운트 2줄+ko 주석, SCORE-API-001 :55/:210/:266-267 + REPORT-001 :56/:212/:269-270 정확 라인 인용)
- **frozen rbac.go 0-diff**: SCORE-API-001 §6 OPEN #4 충돌 회피 — 본 SPEC은 RoleReviewer 신설 금지, admin/analyst 기존 사용으로 매핑 자연 성립
- **phantom API 0**: 모든 [NEW] 메서드 spec.md §2.1 명시, 모든 [EXISTING] 호출 source-verified 라인 인용 (`BeginScoreTx` `pg_store.go:134-148`, `GetScoreByID` `store.go:275-276`, `RoleAdmin`/`RoleAnalyst` `rbac.go:19-26`, `ErrCodeABACDenied` `abac.go:24` 등)
- **consumer-only [HARD]**: SCORE-001/EVAL-ITEM-001/EVID-001/AUTH-003 모두 0-diff 검증 (M5 git diff)

> phantom 0 (lesson #9), errors.go drift 부착(SCORE-API-001 lesson), server.go ≈7줄 정확(REPORT-001 lesson), frozen rbac.go 0-diff(자연 성립, SCORE-API-001 §6#4 충돌 불상속). SSOT: research.md(665줄).
