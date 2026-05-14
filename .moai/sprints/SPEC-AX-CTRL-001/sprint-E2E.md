# Sprint Contract: SPEC-AX-CTRL-001 Sprint 7 — E2E Integration

**Sprint ID**: Sprint 7 / sprint-E2E
**SPEC**: SPEC-AX-CTRL-001
**Date**: 2026-05-14
**Priority Dimension**: Completeness — 최종 사용자 관점의 전체 파이프라인 검증
**Harness Level**: thorough

---

## Acceptance Checklist

| # | Criterion | Verifiable Signal | Status |
|---|-----------|-------------------|--------|
| 1 | POST /api/v1/workflows → HTTP 201 Created | httptest + assert.Equal(201) | PASS |
| 2 | PostgreSQL workflows row: status=PENDING | pgxpool QueryRow 직접 조회 | PASS |
| 3 | PostgreSQL audit_logs row: action=WORKFLOW_CREATED, user_id=cli-anonymous | pgxpool QueryRow 직접 조회 | PASS |
| 4 | Redis celery queue: 1 envelope, headers.id == workflow_id | redis.LRange + json 파싱 | PASS |
| 5 | PENDING → RUNNING: StateMachine.Start 성공 + workflows.status=RUNNING | pgxpool 직접 조회 | PASS |
| 6 | PENDING → RUNNING: audit_logs WORKFLOW_TRANSITIONED_TO_RUNNING 1건 | pgxpool COUNT 조회 | PASS |
| 7 | Redis 닫힌 후 Dispatch → ErrDispatchFailed 래핑 에러 반환 | errors.Is(err, ErrDispatchFailed) | PASS |
| 8 | 3개 생성 후 GET /api/v1/workflows → workflows 배열 3개 이상 | json 응답 배열 길이 확인 | PASS |
| 9 | 5개 동시 POST → 모두 HTTP 201 + workflow_id 중복 없음 | sync.WaitGroup + map seen | PASS |
| 10 | PostgreSQL: 5개 동시 생성 후 DB 행 5개 | pgxpool COUNT 조회 | PASS |

---

## Priority Dimension: Completeness

Sprint 7은 Sprint 1-6에서 검증된 개별 계층들이 실제 인프라 위에서 end-to-end로 동작하는지 검증한다.

- **Completeness 우선**: 단위 테스트에서 검증된 각 계층(store, dispatcher, state machine, handler)이 실제 Postgres + Redis와 정상 통합되는지 확인
- **Functionality 보조**: REST API 계약(HTTP 상태 코드, JSON 형식, Location 헤더)이 실제 DB 쓰기와 일치하는지 확인

---

## Test Scenarios

| Test | Description | Infrastructure |
|------|-------------|----------------|
| TestE2E_FullWorkflowCreationFlow | AC-CTRL-E2E-1 Happy Path | Postgres + Redis |
| TestE2E_StateTransitionWithAudit | PENDING→RUNNING + audit 검증 | Postgres |
| TestE2E_DispatchFailure_HandledGracefully | Redis 장애 시 ErrDispatchFailed | 연결 불가 주소 |
| TestE2E_ListWorkflows_ReturnsAll | 3개 생성 후 LIST | Postgres |
| TestE2E_ConcurrentCreationFromREST | 5개 동시 POST | Postgres |

---

## Pass Conditions

- 5개 E2E 테스트 모두 PASS
- `goleak.VerifyNone(t)` 모든 테스트에서 통과
- go vet + golangci-lint 0 issue
- 단위 테스트 79개 회귀 없음

---

## Infrastructure Notes

- testcontainers-go v0.42.0: postgres:16-alpine + redis:7-alpine
- 컨테이너 기동 타임아웃: 120초 (첫 pull 포함)
- 스키마: `apps/control-plane/internal/store/testdata/schema.sql`
- Redis 어댑터: goRedisAdapter (go-redis v9 → scheduler.RedisClient 인터페이스)
- 빌드 태그: `//go:build integration`

---

Version: 1.0.0
Source: SPEC-AX-CTRL-001 Sprint 7 (manager-tdd)
