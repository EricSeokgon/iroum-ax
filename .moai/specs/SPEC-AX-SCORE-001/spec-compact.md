# SPEC-AX-SCORE-001 (Compact) — 경영평가 점수 산출/집계 (Evaluation Scoring & Aggregation)

> v0.1.2 · status draft · TDD · thorough · brownfield. 상세는 spec.md / plan.md / acceptance.md / strategy.md. (AC=17, §7 edge case=16, §6 4건 RESOLVED)

## 핵심 stub 계약 [HARD]

- `scores.evaluation_item_id` = `VARCHAR(64)` — **FK 없는 stub** (SPEC-AX-EVAL-ITEM-001 §1.4 `evaluation_items.id VARCHAR(64)` 계층코드 타입 호환)
- `scores.evidence_id` = `UUID` nullable — **FK 없는 stub** (SPEC-AX-EVID-001 `evidences.id UUID` 타입 호환)
- EVID-001/EVAL-ITEM-001 코드·마이그레이션(`0002`/`0003`)·FK 하드닝 = **범위 밖** (미래 별도 SPEC)
- 1차 산출물 = 점수 데이터 모델 + store + audit Walking Skeleton. 가중 롤업/등급 = 모델링 + 최소 구현

## REQ 모듈 (5: 1 Ubiquitous 묶음 + 4 modal)

| REQ-ID | EARS | 요지 |
|--------|------|------|
| REQ-SCORE-UBI-001 (데이터 주권) | Ubiquitous | 점수 생성/조회/수정/롤업/등급 외부 호출 0건 (LLM 시뮬레이션 범위 밖, 내부 결정적 threshold만) |
| REQ-SCORE-UBI-002 (감사 가능성) | Ubiquitous | 모든 create/update → 동일 TX `audit_logs` 1건 |
| REQ-SCORE-UBI-003 (cli-anonymous) | Ubiquitous(State) | AuthN off 시 created_by/user_id='cli-anonymous' literal |
| REQ-SCORE-UBI-004 (점수 불변, Decision 4) | Ubiquitous | `scores` 물리 삭제 0건 + status state-machine (DRAFT 가변/CONFIRMED 불변/정정=신규행+SUPERSEDED) |
| REQ-SCORE-001-E1/S1/S2/O1/U1 | E/S/S/O/U | 점수 데이터 모델 & Store: 생성+audit 원자(E1), FK 없는 stub(S1), status state-machine(S2, Decision 4), metadata opaque(O1), 입력 검증(U1) |
| REQ-SCORE-002-E1/U1 | E/U | 가중 롤업(최소): 단일 레벨 Σ(score×weight)(E1), NULL weight 조용한 왜곡 0건(U1) |
| REQ-SCORE-003-E1/S1/U1 | E/S/U | 등급 산출: `grade_thresholds` 테이블 결정적 lookup(E1, Decision 3), 0행 결정적 실패(S1), boundary_rule 경계 결정성(U1) |
| REQ-SCORE-004-E1/U1 | E/U | 감사 연계: RecordScore* 동일 TX, resource_id=score.id 직접 UUID(E1, Decision 2), audit fail 양방향 rollback(U1) |

## AC (17: AC-SCORE-{REQ}-{N})

- §0 UBI: AC-SCORE-UBI-001(외부호출 0), -002(audit 원자), -003(cli-anonymous), -004(무삭제+status state-machine)
- §1 REQ-SCORE-001: AC-SCORE-001-1(생성+audit), -2(evidence_id UUID stub FK 없음), -3(evaluation_item_id VARCHAR(64) stub FK 없음), -4(입력 검증), -S2(DRAFT 가변/CONFIRMED 불변, Decision 4), -O1-1(metadata opaque)
- §2 REQ-SCORE-002: AC-SCORE-002-1(Σ(score×weight) 정확), -2(NULL weight 결정적 단일 정책)
- §3 REQ-SCORE-003: AC-SCORE-003-1(grade_thresholds letter 결정적), -2(boundary_rule 경계+0행 미설정 실패)
- §4 REQ-SCORE-004: AC-SCORE-004-1(RecordScore* audit row, resource_id=score.id 직접), -2(audit fail 양방향 rollback)
- §5 경계: AC-SCORE-BOUNDARY-1(evidences/evaluation_items FK 부재 + 미수정)
- 분해 합 = 4+6+2+2+2+1 = 17 = 물리 heading 17. §7 edge=16. 각 modal REQ ≥2 AC. §6 4건 RESOLVED 반영 완료

## Files to Modify

| 경로 | Delta |
|------|-------|
| `internal/store/store.go` | [MODIFY] ScoreStore/ScoreTx 인터페이스 |
| `internal/store/score.go` | [NEW] PgScoreTx pgx (eval_item.go 미러) + validateScoreStatusTransition (Decision 4) |
| `internal/store/pg_store.go` | [MODIFY] BeginScoreTx (PgWorkflowStore.pool 재사용 — postgres.go 死 스텁 비대상) |
| `internal/audit/audit.go` | [MODIFY] ActionScoreCreated/Updated **상수 2개만 (namespace 0, Decision 2)** |
| `internal/audit/recorder.go` | [MODIFY] RecordScoreCreated/Updated (로컬 AuditTx, resource_id=score.id 직접) |
| `.moai/db/schema/migrations/0004_score_tables.sql` | [NEW] **2테이블** `scores` + `grade_thresholds` 멱등 SQL (Decision 1+3 RESOLVED) |
| `.moai/db/schema/initial.sql` / `0002` / `0003` | [EXISTING] 참조 only, 미수정 (FK 하드닝 범위 밖) |
| `internal/store/score_test.go` / `internal/audit/recorder_score_test.go` | [NEW] |
| `internal/store/pg_store_test.go` | [EXISTING] Workflow/Evidence/EvalItem 회귀 검증 |

## Exclusions (What NOT to Build)

1. 풀 집계 엔진 / REST API / Console UI
2. 풀 등급기준(scoring rubric) 시스템 — 룰 엔진/가점·감점/계층 (SPEC-AX-EVAL-ITEM-001 §5 #2 이연). **IN SCOPE**: 최소 `grade_thresholds` 4컬럼 테이블(Decision 3, 풀 rubric 아님)
3. `evidences`/`evaluation_items` FK 하드닝 (EVID-001/EVAL-ITEM-001 코드·마이그레이션 변경 포함)
4. 깊은 재귀 롤업 / 집계 snapshot 영속 / incremental 집계 (Option B 2-테이블 감사 원자성 위반 기각)
5. **LLM 등급 시뮬레이션 / Recommendation 엔진** (`product.md:171-183`/`:177` — 후속 Python/AI 파이프라인)
6. 다중 테넌시 / 조직 격리
7. 마이그레이션 도구 통합 (alembic/golang-migrate)

## §6 RESOLVED (plan.md §6 — Run Phase 1 strategy.md §A + Human Gate sign-off, v0.1.2)

1. **Decision 1 — Score 테이블 = Option A** (단일 `scores` + level discriminator ∈ {raw,item,category}). B 기각(2-테이블 집계 saga → 1 TX=1 entity=1 audit 위반, EVAL-ITEM Option B 동일 선례 독립 재도출), C는 named post-PoC 전환 경로(Option A가 PoC 동작 superset 포함)
2. **Decision 2 — Audit resource_id = Option 1 직접 UUID** (`Event.ResourceID = score.id`, surrogate/namespace 상수 0). 🔑 `scores.id`=UUID PK → EVAL-ITEM-001 audit 타입 불일치(VARCHAR PK→uuid.Nil) **구조적 부재** → AUD-1 불필요, EVAL-ITEM보다 단순. Option 2(surrogate)=비-UUID PK dead path. D1=A ⟹ D2=Option1 필연
3. **Decision 3 — Grade-threshold = 최소 `grade_thresholds` 테이블** (scope, letter, min_value, boundary_rule, PK(scope,letter); metadata JSONB 아님 — S1 scope 결정성 + O1 opacity 자기모순 해소). boundary_rule 기본 `gte`. 풀 rubric 아닌 최소 floor
4. **Decision 4 — 불변/정정 = status state-machine + append-only** (DRAFT in-place 가변 / CONFIRMED 불변 / 정정=신규행 INSERT+구행 CONFIRMED→SUPERSEDED / 물리삭제 0). 전이표: DRAFT→DRAFT, DRAFT→CONFIRMED, CONFIRMED→SUPERSEDED(terminal). EVID-001 append-only 선례

> `0004` 2테이블 확정. issue_number 0 (gh unavailable). Migration 0004 비충돌 (0001/0002/0003 디스크 확인, strategy.md §0). phantom 0 (BeginScoreTx→pg_store.go PgWorkflowStore.pool). SPEC-AX-EVAL-ITEM-001 §6 OPEN→RESOLVED 흐름 동위상.
