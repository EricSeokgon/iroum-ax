# Research: SPEC-AX-SCORE-001 경영평가 점수 산출/집계 (Evaluation Scoring & Aggregation)

Phase: 0.5 Deep Research
Generated: 2026-05-19
Agent: Explore (read-only deep codebase analysis)
Status: complete

> SPEC-AX-SCORE-001 EARS 요구사항 설계 근거. 모든 주장 file:line 근거 포함.

---

## 1. Architecture Analysis — Store/Audit 재사용 맵

- 2계층 패턴: 공개 `XxxStore`(TX 진입점만) + 내부 `XxxTx`(단일 pgx TX 내 read/write) — `store.go:16-54`(WorkflowStore), `:56-106`(EvidenceStore), `:108-212`(EvalItemStore) 본보기.
- `PgWorkflowStore{pool, logger}` 단일 풀 전 도메인 공유. `BeginTx`/`BeginEvidenceTx`/`BeginEvalItemTx` 모두 `s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: ReadCommitted})` (`pg_store.go:84/104/119`). 신규 풀 금지.
- `postgres.go` Sprint-0 死 스텁 — 비대상. 실 구현 `pg_store.go`만.
- SCORE 통합: `store.go`에 `ScoreStore`(EvalItemStore 패턴), `pg_store.go`에 `BeginScoreTx`, 신규 `score.go`에 `PgScoreTx`(`eval_item.go:31-36` 미러), `recorder.go`에 `RecordScore*`(local AuditTx, `recorder.go` RecordEvalItem* 미러).

## 2. Dependency Stub 계약 — evaluation_item_id / evidence_id 타입

- `0002_evidence_tables.sql:8`: `evaluation_item_id VARCHAR(64) NOT NULL` FK 없는 stub.
- `0003_eval_item_tables.sql:8`: `evaluation_items.id VARCHAR(64) PRIMARY KEY` 계층코드 (UUID 아님). SPEC-AX-EVAL-ITEM-001 §1.4 [HARD] load-bearing.
- evidences.id = UUID (`0002`).
- **[HARD] SCORE 계약**: `scores.evaluation_item_id` = `VARCHAR(64)` (EVAL-ITEM-001 §1.4 호환), `scores.evidence_id` = `UUID` (evidences.id 호환). FK 제약 없음(EVID-001 stub 동일 패턴). EVID-001/EVAL-ITEM-001 코드·마이그레이션·FK 변경 범위 밖. FK 하드닝 미래 별도 SPEC.

## 3. 경영평가 Scoring/Grade Semantics (product.md)

- `product.md:160` 4계층: 평가범주 → 평가항목 → 평가지표 → 배점/가중치.
- 점수 흐름: 지표별 raw score → EVAL-ITEM weight 가중 롤업(Σ score×weight) → 항목 → 범주 집계 → 등급(S/A/B/C/D) threshold.
- 등급: 기획재정부 경영평가 편람(`product.md:67`), A/B/C/D 벤치마크(`:70`). 정확 임계값은 PoC 의존(evaluation_items.metadata 또는 별도 grade_rubric 이연).
- **[HARD] 경계**: LLM "등급 시뮬레이션"(`product.md:171-183`)·"Recommendation 엔진"(`:177`)은 **후속 Python/AI 파이프라인 — SCORE-001 범위 밖**. SCORE-001 = 기반 점수 데이터 모델 + 최소 롤업/threshold만. 풀 rubric 시스템도 §Out of Scope (EVAL-ITEM-001 §5 이연).

## 4. Migration & Schema 규약 (0004_score_tables.sql)

- 멱등성(`0002:26-29`, `0003:24-27`): `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `DO $$ BEGIN ALTER...ADD CONSTRAINT...EXCEPTION WHEN duplicate_object THEN NULL; END $$`.
- 파일번호: `0004_score_tables.sql` (0001/0002/0003 비충돌 확인).
- 타입: UUID PK `DEFAULT uuid_generate_v4()`, VARCHAR(64) 계층코드, `DECIMAL` 점수/가중치(weight DECIMAL(5,4), score DECIMAL(6,2)), TIMESTAMPTZ audit, JSONB metadata, VARCHAR+CHECK enum. 인덱스 `{table}_{cols}_idx`. [HARD] `initial.sql` 미수정 (additive only).

## 5. Score 테이블 구조 옵션 (strategy 단계 결정 이연)

| Option | 구조 | audit 원자성 | Pros | Cons |
|--------|------|-------------|------|------|
| **A 단일 scores + level discriminator** (잠정 권고) | 1 테이블 (level: raw/item/category) | 1 entity 1 Recorder 1 audit — atomic ✓ | 최단순, cascade 없음 | 런타임 집계(깊은 계층 느림), 집계 audit 이력 부재 |
| B scores + score_aggregates | 2 테이블 | 집계가 별도 TX → **audit 위반** (EVAL-ITEM Option B saga 기각 동일) | 집계 빠름, 집계 audit | saga 복잡, orphan |
| C raw scores + on-the-fly 계산 | 1 테이블 + 읽기시 계산 | 1 audit ✓ | 최단순 audit | 읽기 집계 느림, 집계 snapshot audit 부재 |

strategy 결정 의존: grade threshold가 집계 참조하는지, 집계 snapshot audit 필요 여부, PoC가 raw 점수만인지.

## 6. AUD-1 Audit-ID 선례 & 결정 옵션

- EVAL-ITEM AUD-1(`audit.go:70-82` `EvalItemAuditNamespace = uuid.MustParse("a7f3c2e1-...")`, `recorder.go:303-305` `uuid.NewSHA1(ns,[]byte(code))`): VARCHAR 계층코드 → 결정적 UUIDv5 surrogate. 고정 컴파일타임 상수(SEC-05/TH-12), RFC4122 v5.
- `audit.Event.ResourceID uuid.UUID`(`audit.go:72`), `audit_logs.resource_id UUID NOT NULL`(`initial.sql:119`), `parseResourceID` 비-UUID→uuid.Nil.
- **SCORE 옵션**: Option 1(권고) — `scores.id UUID PK` → `Event.ResourceID = score.id` 직접(evidences 동일, surrogate 불필요, 신규 namespace 상수 없음). Option 2 — 비-UUID PK 시 EVAL-ITEM AUD-1 surrogate 선례. strategy가 테이블 구조(§5)와 함께 확정.

## 7. SPEC 문서 규약 (EVAL-ITEM-001 v0.1.3 최신 템플릿)

- frontmatter 8필드(`SPEC-AX-EVAL-ITEM-001/spec.md:1-10`): id SPEC-AX-SCORE-001, version 0.1.0, status draft, created/updated 2026-05-19, author ircp, priority high, issue_number 0. `labels`/`created_at` 미사용.
- 섹션: HISTORY + Schema note → §1 개요(Walking Skeleton/Anchor/Composite/HARD 계약) §2 영향파일([DELTA]) §3 EARS(UBI+E/S/O/U) §4 구현방법 §5 Exclusions §6 관계 SPEC §7 Edge Cases §8 RED/GREEN §9 DoD.
- EARS: `REQ-SCORE-UBI-NNN (한글)`; `REQ-SCORE-NNN-E/S/O/U`. AC `AC-SCORE-{REQ}-{N}`. spec-compact.md 자동 생성. 모든 주장 file:line.

## 8. REQ-UBI 패턴 & 한국 공공 6제약

- canonical `REQ-SCORE-UBI-NNN (한글 제목)` dual-track:
  - REQ-SCORE-UBI-001 (데이터 주권): 점수 계산 내부 PostgreSQL만, 외부 호출 0.
  - REQ-SCORE-UBI-002 (감사 가능성): 모든 score create/update → 동일 TX `audit_logs` 1건.
  - REQ-SCORE-UBI-003 (cli-anonymous 기본값): AUTH 비활성 시 literal 'cli-anonymous'(NULL 금지).
  - REQ-SCORE-UBI-004 (불변 — strategy 확정): 예 "grade 확정 후 점수 불변" 등.
- 6제약: 데이터 주권 / 한국어 / 감사 가능성 / 망분리 / 조직 격리 / 시간 제약(TIMESTAMPTZ UTC).

## 9. Risks, Constraints, Implicit Contracts

- HARD: scores.evaluation_item_id VARCHAR(64)(EVAL-ITEM 호환), 단일 pgx 풀 재사용, 1 TX=1 entity=1 audit(Option B saga 기각), AUD namespace 고정 상수, initial.sql 미수정.
- 암묵: Recorder.clock 주입(`recorder.go:42-76`), 에러 wrapping+stderrors 매핑(`eval_item.go:74-86`), nullIfEmpty NULL/empty 구분(`eval_item.go:439-443`), 멱등 DO$$, @MX 태그(fan_in≥3 ANCHOR / 복잡도 WARN).
- strategy 불확실성: 테이블 구조 A/B/C, grade threshold 모델, 점수 가변성(정정 vs 불변), 집계 빈도(요청시 vs 점진 영속).

## 10. Recommended Implementation Approach

Score 엔티티(최소): `id UUID PK, evaluation_item_id VARCHAR(64), evidence_id UUID, score_value DECIMAL(6,2), weight DECIMAL(5,4), created_at, created_by DEFAULT 'cli-anonymous', updated_at`. ScoreStore.BeginScoreTx + ScoreTx{InsertScore, GetScoreByID, GetScoresByEvaluationItem, InsertAuditLog, Commit, Rollback}. audit.go에 `ActionScoreCreated` + (필요시 grade), recorder.go에 `RecordScoreCreated` (AUD-1 선례, scores.id UUID면 직접). grade-threshold 최소 모델(metadata JSONB 또는 별도 grade_thresholds 테이블 — strategy). EVID/EVAL-ITEM 경량 stub(FK 없음). `0004_score_tables.sql` 멱등(Option A 잠정). TDD RED-GREEN-REFACTOR, 기존 Workflow/Evidence/EvalItem 특성화 회귀 0.

복합 도메인 `SPEC-AX-SCORE-001`. 의존: SPEC-AX-CTRL-001(pgx pool/audit) + SPEC-AX-EVID-001(evidences) + SPEC-AX-EVAL-ITEM-001(evaluation_items) GREEN. thorough harness.

strategy 결정 항목: (1) 테이블 구조 A/B/C, (2) 가중치 롤업 위치(store-layer vs 별도 함수), (3) audit-id Option 1/2, (4) grade-threshold 모델 구조.
