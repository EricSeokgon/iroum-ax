---
engine: PostgreSQL 16
orm: pgx/v5 (raw SQL, no ORM)
last_synced_at: 2026-05-20
manifest_hash: _TBD_
---

# Database Schema

수동 SQL 마이그레이션 기반 스키마 (마이그레이션 도구 미사용). 각 마이그레이션 파일은
`.moai/db/schema/migrations/` 에 위치하며 `0001 → 0002 → 0003 → 0004 → 0005` 순서로 적용한다.

---

## Tables

| 테이블 | 설명 | 마이그레이션 |
|--------|------|-------------|
| `documents` | 워크플로우 문서 엔티티 (SPEC-AX-CTRL-001) | `0001_initial.sql` |
| `audit_logs` | 모든 도메인의 감사 이벤트 단일 테이블 (SPEC-AX-001) | `0001_initial.sql` |
| `evidences` | 경영평가 증빙 자료 (SPEC-AX-EVID-001) | `0002_evidence_tables.sql` |
| `evaluation_items` | 경영평가 평가항목 taxonomy (SPEC-AX-EVAL-ITEM-001) | `0003_eval_item_tables.sql` |
| `scores` | 경영평가 점수 (SPEC-AX-SCORE-001) | `0004_score_tables.sql` |
| `grade_thresholds` | 등급 산출 임계값 (SPEC-AX-SCORE-001) | `0004_score_tables.sql` |
| `score_review_requests` | 평가 점수 제출/검토/승인 워크플로우 (SPEC-AX-REVIEW-001) | `0005_score_review_request_tables.sql` |

---

## 테이블 상세

### `scores` (SPEC-AX-SCORE-001, `0004_score_tables.sql` L11-24)

경영평가 점수 단일 테이블. `level` discriminator(`raw`/`item`/`category`)로 평가지표·평가항목·평가범주
점수를 구분한다 (Decision 1 Option A). 물리 삭제 없음 — status state-machine(D4) 으로 이력 보존.

| 컬럼 | 타입 | NULL | 기본값 | 설명 |
|------|------|------|--------|------|
| `id` | `UUID` | NOT NULL | `uuid_generate_v4()` | PK — audit `resource_id` 직접 대입 (Decision 2, D2) |
| `evaluation_item_id` | `VARCHAR(64)` | NOT NULL | — | FK 없는 stub; `evaluation_items.id VARCHAR(64)` 타입 호환 (§1.4 HARD) |
| `evidence_id` | `UUID` | NULL | — | FK 없는 stub; `evidences.id UUID` 타입 호환 (§1.4 HARD) |
| `level` | `VARCHAR(16)` | NOT NULL | `'raw'` | D1 discriminator: `raw`\|`item`\|`category` |
| `score_value` | `DECIMAL(6,2)` | NULL | — | 점수 값 (NaN/Inf 거부 — store 계층 검증) |
| `weight` | `DECIMAL(5,4)` | NULL | — | 가중치 (NULL = exclude 정책, GAP-01) |
| `grade` | `VARCHAR(2)` | NULL | — | 등급 문자 `S`\|`A`\|`B`\|`C`\|`D`; 산출 전 NULL |
| `status` | `VARCHAR(32)` | NOT NULL | `'DRAFT'` | D4 state-machine: `DRAFT`→`CONFIRMED`→`SUPERSEDED` |
| `metadata` | `JSONB` | NULL | — | 불투명 메타데이터 (미해석, REQ-SCORE-001-O1) |
| `created_at` | `TIMESTAMP WITH TIME ZONE` | NOT NULL | `now()` | 생성 시각 |
| `created_by` | `VARCHAR(64)` | NOT NULL | `'cli-anonymous'` | 생성자 (인증 비활성 기본값, REQ-SCORE-UBI-003) |
| `updated_at` | `TIMESTAMP WITH TIME ZONE` | NOT NULL | `now()` | 최종 수정 시각 |

**CHECK 제약** (`0004_score_tables.sql` L27-42):
- `scores_level_chk`: `level IN ('raw', 'item', 'category')`
- `scores_status_chk`: `status IN ('DRAFT', 'CONFIRMED', 'SUPERSEDED')`
- `scores_grade_chk`: `grade IS NULL OR grade IN ('S', 'A', 'B', 'C', 'D')`

**인덱스** (`0004_score_tables.sql` L45-47):
- `scores_evaluation_item_id_idx` ON `scores(evaluation_item_id)` — `SumWeightedByEvaluationItem` Index Scan 보장
- `scores_evidence_id_idx` ON `scores(evidence_id)` WHERE `evidence_id IS NOT NULL`
- `scores_level_idx` ON `scores(level)`

---

### `grade_thresholds` (SPEC-AX-SCORE-001, `0004_score_tables.sql` L52-70)

등급 산출 임계값 테이블. scope별 letter↔min_value 매핑. S→D 내림차순 `min_value` 스캔,
`boundary_rule`(gte/gt) 기준 첫 번째 일치 등급 반환 (D3, Decision 3).

| 컬럼 | 타입 | NULL | 기본값 | 설명 |
|------|------|------|--------|------|
| `scope` | `VARCHAR(64)` | NOT NULL | — | 등급 기준 범위 식별자 (예: `'default'`, `'kepco-safety'`) |
| `letter` | `VARCHAR(2)` | NOT NULL | — | 등급 문자: `S`\|`A`\|`B`\|`C`\|`D` |
| `min_value` | `DECIMAL(6,2)` | NOT NULL | — | 해당 등급의 최소 점수 |
| `boundary_rule` | `VARCHAR(8)` | NOT NULL | `'gte'` | `gte`: `score >= min_value`, `gt`: `score > min_value` |

**PK**: `(scope, letter)` 복합 기본키

**CHECK 제약** (`0004_score_tables.sql` L61-70):
- `grade_thresholds_letter_chk`: `letter IN ('S', 'A', 'B', 'C', 'D')`
- `grade_thresholds_boundary_rule_chk`: `boundary_rule IN ('gte', 'gt')`

scope 0행 시 `DetermineGrade`는 `ErrGradeThresholdsUnavailable` 반환 (등급 fabricate 금지, fail-closed, D3).

---

### `score_review_requests` (SPEC-AX-REVIEW-001, `0005_score_review_request_tables.sql` L10-22)

평가 점수 제출/검토/승인 워크플로우 단일 테이블. 4-상태 생명주기(`SUBMITTED` → `UNDER_REVIEW` →
`APPROVED`/`REJECTED`)를 status 컬럼으로 관리한다. `APPROVED`/`REJECTED`는 terminal(되돌리기 불가).
물리 삭제 없음 — 정정 경로는 신규 `SUBMITTED` row INSERT만 (REQ-REVIEW-UBI-004).

| 컬럼 | 타입 | NULL | 기본값 | 설명 |
|------|------|------|--------|------|
| `id` | `UUID` | NOT NULL | `uuid_generate_v4()` | PK — audit `resource_id` 직접 대입 (D2 미러, surrogate 미사용) |
| `score_id` | `UUID` | NOT NULL | — | FK 없는 stub; `scores.id UUID` 타입 호환 (§1.4 HARD — handler-compose 2-TX 검증) |
| `status` | `VARCHAR(32)` | NOT NULL | `'SUBMITTED'` | 상태 머신: `SUBMITTED`\|`UNDER_REVIEW`\|`APPROVED`\|`REJECTED` |
| `assigned_reviewer_id` | `VARCHAR(128)` | NULL | — | nullable: 정보/감사 추적용 (ABAC 결정과 무관) |
| `rejection_reason` | `TEXT` | NULL | — | nullable: REJECTED 상태 시 non-empty 필수 (§A.5 Layer 2 CHECK 강제) |
| `comment` | `TEXT` | NULL | — | nullable: 모든 상태에서 optional (opaque) |
| `metadata` | `JSONB` | NULL | — | 불투명 메타데이터 placeholder (미해석) |
| `created_at` | `TIMESTAMP WITH TIME ZONE` | NOT NULL | `now()` | 생성 시각 |
| `created_by` | `VARCHAR(128)` | NOT NULL | `'cli-anonymous'` | 생성자 (인증 비활성 기본값, REQ-REVIEW-UBI-003 정합) |
| `updated_at` | `TIMESTAMP WITH TIME ZONE` | NOT NULL | `now()` | 최종 수정 시각 |
| `updated_by` | `VARCHAR(128)` | NOT NULL | `'cli-anonymous'` | 최종 수정자 (인증 비활성 기본값) |

**CHECK 제약** (`0005_score_review_request_tables.sql` L25-35):
- `score_review_requests_status_chk`: `status IN ('SUBMITTED', 'UNDER_REVIEW', 'APPROVED', 'REJECTED')`
- `score_review_requests_reject_reason_chk`: `(status != 'REJECTED') OR (rejection_reason IS NOT NULL AND length(rejection_reason) > 0)` — §A.5 Layer 2 fail-safe (handler validation과 이중 방어)

**인덱스** (`0005_score_review_request_tables.sql` L38-40):
- `score_review_requests_score_id_idx` ON `score_review_requests(score_id)` — score_id 기반 조회
- `score_review_requests_status_idx` ON `score_review_requests(status)` — 상태 필터 조회
- `score_review_requests_created_at_idx` ON `score_review_requests(created_at DESC)` — 시간 정렬 조회

---

## Relationships

FK 제약은 현재 stub 단계로 존재하지 않음. 타입 호환성만 보장.

| From | To | 카디널리티 | 컬럼 | 비고 |
|------|----|-----------|------|------|
| `scores` | `evaluation_items` | N:1 | `scores.evaluation_item_id` | FK 없는 stub (`VARCHAR(64)` 타입 호환) — 미래 FK 하드닝 SPEC 대상 |
| `scores` | `evidences` | N:1 | `scores.evidence_id` | FK 없는 stub (`UUID` 타입 호환, nullable) — 미래 FK 하드닝 SPEC 대상 |
| `score_review_requests` | `scores` | N:1 | `score_review_requests.score_id` | FK 없는 stub (`UUID` 타입 호환) — handler-compose 2-TX로 존재 검증 |
| `audit_logs` | `scores` | N:1 | `audit_logs.resource_id` | `resource_id = scores.id` 직접 UUID 대입 (FK 없음, D2 Decision 2) |
| `audit_logs` | `score_review_requests` | N:1 | `audit_logs.resource_id` | `resource_id = score_review_requests.id` 직접 UUID 대입 (D2 미러) |

---

## Indexes

| 테이블 | 컬럼 | 타입 | 목적 |
|--------|------|------|------|
| `scores` | `evaluation_item_id` | BTREE | `SumWeightedByEvaluationItem`, `GetScoresByEvaluationItem` 인덱스 스캔 |
| `scores` | `evidence_id` (WHERE IS NOT NULL) | PARTIAL BTREE | evidence_id 기반 조회 |
| `scores` | `level` | BTREE | level discriminator 필터 |
| `score_review_requests` | `score_id` | BTREE | score_id 기반 검토 요청 조회 |
| `score_review_requests` | `status` | BTREE | 상태 필터 목록 조회 |
| `score_review_requests` | `created_at DESC` | BTREE | 시간 역순 정렬 조회 |

---

## Constraints

| 테이블 | 제약명 | 타입 | 정의 |
|--------|--------|------|------|
| `scores` | `scores_level_chk` | CHECK | `level IN ('raw', 'item', 'category')` |
| `scores` | `scores_status_chk` | CHECK | `status IN ('DRAFT', 'CONFIRMED', 'SUPERSEDED')` |
| `scores` | `scores_grade_chk` | CHECK | `grade IS NULL OR grade IN ('S', 'A', 'B', 'C', 'D')` |
| `grade_thresholds` | `grade_thresholds_letter_chk` | CHECK | `letter IN ('S', 'A', 'B', 'C', 'D')` |
| `grade_thresholds` | `grade_thresholds_boundary_rule_chk` | CHECK | `boundary_rule IN ('gte', 'gt')` |
| `grade_thresholds` | (PK) | PRIMARY KEY | `(scope, letter)` 복합 PK |
| `score_review_requests` | `score_review_requests_status_chk` | CHECK | `status IN ('SUBMITTED', 'UNDER_REVIEW', 'APPROVED', 'REJECTED')` |
| `score_review_requests` | `score_review_requests_reject_reason_chk` | CHECK | `(status != 'REJECTED') OR (rejection_reason IS NOT NULL AND length(rejection_reason) > 0)` |

> **참고**: `scores.evaluation_item_id` / `scores.evidence_id` / `score_review_requests.score_id` 는 FK 제약 없는 stub 컬럼이다.
> FK 하드닝은 미래 별도 SPEC 대상이며 본 문서 범위 밖이다.
