# SPEC-AX-SCORE-001 (Compact) — 경영평가 점수 산출/집계 (Evaluation Scoring & Aggregation)

> v0.1.1 · status draft · TDD · thorough · brownfield. 상세는 spec.md / plan.md / acceptance.md. (AC=16, §7 edge case=15)

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
| REQ-SCORE-UBI-004 (점수 불변) | Ubiquitous | `scores` 물리 삭제 0건, 확정 점수 불변 (정확 규칙 plan.md §6 OPEN) |
| REQ-SCORE-001-E1/S1/O1/U1 | E/S/O/U | 점수 데이터 모델 & Store: 생성+audit 원자(E1), evaluation_item_id/evidence_id FK 없는 stub(S1), metadata opaque(O1), 입력 검증(U1) |
| REQ-SCORE-002-E1/U1 | E/U | 가중 롤업(최소): 단일 레벨 Σ(score×weight)(E1), NULL weight 조용한 왜곡 0건(U1) |
| REQ-SCORE-003-E1/S1/U1 | E/S/U | 등급 산출(최소 threshold): score→letter 결정적(E1), 미설정 결정적 실패(S1), 경계값 결정성(U1) |
| REQ-SCORE-004-E1/U1 | E/U | 감사 연계: RecordScoreCreated/Updated 동일 TX(E1), audit fail 양방향 rollback(U1) |

## AC (16: AC-SCORE-{REQ}-{N})

- §0 UBI: AC-SCORE-UBI-001(외부호출 0), -002(audit 원자), -003(cli-anonymous), -004(무삭제)
- §1 REQ-SCORE-001: AC-SCORE-001-1(생성+audit), -2(evidence_id UUID stub FK 없음), -3(evaluation_item_id VARCHAR(64) stub FK 없음), -4(입력 검증), -O1-1(metadata opaque)
- §2 REQ-SCORE-002: AC-SCORE-002-1(Σ(score×weight) 정확), -2(NULL weight 결정적)
- §3 REQ-SCORE-003: AC-SCORE-003-1(letter 결정적), -2(경계 결정성+미설정 실패)
- §4 REQ-SCORE-004: AC-SCORE-004-1(RecordScore* audit row), -2(audit fail 양방향 rollback)
- §5 경계: AC-SCORE-BOUNDARY-1(evidences/evaluation_items FK 부재 + 미수정)
- 각 modal REQ ≥2 AC. §6 OPEN 의존 AC(UBI-004, 002-2, 003-1/2, 004-1)는 Run Phase 1 strategy 확정 후 정정

## Files to Modify

| 경로 | Delta |
|------|-------|
| `internal/store/store.go` | [MODIFY] ScoreStore/ScoreTx 인터페이스 |
| `internal/store/score.go` | [NEW] PgScoreTx pgx (eval_item.go 미러) |
| `internal/store/pg_store.go` | [MODIFY] BeginScoreTx (PgWorkflowStore.pool 재사용 — postgres.go 死 스텁 비대상) |
| `internal/audit/audit.go` | [MODIFY] ActionScoreCreated/Updated |
| `internal/audit/recorder.go` | [MODIFY] RecordScoreCreated/Updated (로컬 AuditTx) |
| `.moai/db/schema/migrations/0004_score_tables.sql` | [NEW] scores 멱등 SQL (Option A 잠정) |
| `.moai/db/schema/initial.sql` / `0002` / `0003` | [EXISTING] 참조 only, 미수정 (FK 하드닝 범위 밖) |
| `internal/store/score_test.go` / `internal/audit/recorder_score_test.go` | [NEW] |
| `internal/store/pg_store_test.go` | [EXISTING] Workflow/Evidence/EvalItem 회귀 검증 |

## Exclusions (What NOT to Build)

1. 풀 집계 엔진 / REST API / Console UI
2. 풀 등급기준(scoring rubric) 시스템 (SPEC-AX-EVAL-ITEM-001 §5 #2 이연 — 최소 threshold만)
3. `evidences`/`evaluation_items` FK 하드닝 (EVID-001/EVAL-ITEM-001 코드·마이그레이션 변경 포함)
4. 깊은 재귀 롤업 / 집계 snapshot 영속 / incremental 집계 (Option B 2-테이블 감사 원자성 위반 기각)
5. **LLM 등급 시뮬레이션 / Recommendation 엔진** (`product.md:171-183`/`:177` — 후속 Python/AI 파이프라인)
6. 다중 테넌시 / 조직 격리
7. 마이그레이션 도구 통합 (alembic/golang-migrate)

## OPEN (plan.md §6 — Run Phase 1 strategy + Human Gate 확정, 본 SPEC 미해결)

1. Score 테이블 구조 A(단일+level discriminator, 잠정)/B(2-테이블, 감사 위반 기각)/C(on-the-fly, post-PoC)
2. Audit resource_id Option 1(직접 UUID, scores.id UUID PK 시 잠정)/Option 2(EVAL-ITEM AUD-1 식 surrogate, 비-UUID PK fallback)
3. Grade-threshold 모델 위치 (metadata JSONB vs 최소 grade_thresholds 테이블)
4. 점수 불변/정정 규칙 (status 전이 vs 신규 행, 전이표)

> EVAL-ITEM-001 §6 pre-Run OPEN→RESOLVED 흐름 동위상. issue_number 0 (gh unavailable). Migration 0004 비충돌 (0001/0002/0003 디스크 확인).
