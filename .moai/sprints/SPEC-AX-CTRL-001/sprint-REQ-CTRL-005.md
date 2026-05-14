# Sprint Contract — Sprint 6: REQ-CTRL-005 Celery Dispatch

> SPEC: SPEC-AX-CTRL-001 v0.1.2
> Sprint: 6
> REQ: REQ-CTRL-005
> Phase: RED (2026-05-14)
> Harness: thorough
> Evaluator priority: Originality 35% / Functionality 35% / Completeness 20% / Security 10%

---

## 1. Sprint 목표

Celery protocol v2 envelope (Kombu 호환) 직렬화 + Redis RPUSH 기반 dispatch 구현.
R-CTRL-001 mitigation을 위한 golden file 비교 테스트가 핵심 계약.

---

## 2. Priority Dimension

**Originality + Functionality** (전략 문서 4.7 기준)

- Celery protocol v2 envelope byte-exact match는 Go → Python Celery worker 통신의 핵심
- Python Kombu가 생성하는 JSON envelope과 Go가 생성하는 JSON envelope이 동일해야 함
- Dispatch failure → workflow FAILED 전이는 R-CTRL-003 PENDING 고아 mitigation의 핵심

---

## 3. Acceptance Checklist

| ID | 기준 | 검증 방법 | Priority |
|----|------|----------|----------|
| AC-CTRL-005-1 | Envelope golden file byte match | TestCeleryDispatcher_BuildEnvelope_GoldenFileMatch | P0 (blocking) |
| AC-CTRL-005-1 | Kombu v2 필수 필드 완전성 | TestCeleryDispatcher_BuildEnvelope_RequiredFields | P0 |
| AC-CTRL-005-1 | Body base64 디코드 후 [args, kwargs, embed] | TestCeleryDispatcher_BuildEnvelope_BodyBase64Decodable | P0 |
| AC-CTRL-005-1 | argsrepr/kwargsrepr Python repr 형식 | TestCeleryDispatcher_BuildEnvelope_ArgsKwargsRepr | P1 |
| AC-CTRL-005-1 | pythonReprList 함수 변환 | TestPythonReprList | P1 |
| AC-CTRL-005-1 | pythonReprDict 함수 변환 | TestPythonReprDict | P1 |
| AC-CTRL-005-2 | 정상 dispatch → Redis RPUSH 1회 | TestCeleryDispatcher_Dispatch_RedisRPUSH | P0 |
| AC-CTRL-005-2 | 커스텀 queue 이름 RPUSH | TestCeleryDispatcher_Dispatch_CustomQueue | P1 |
| AC-CTRL-005-2 | Redis 불가 → ErrDispatchFailed | TestCeleryDispatcher_Dispatch_RedisUnavailable_ReturnsError | P0 |
| AC-CTRL-005-2 | RPUSH 실패 → ErrDispatchFailed (R-CTRL-003) | TestCeleryDispatcher_Dispatch_FailureMarksWorkflowFailed | P0 |
| AC-CTRL-005-2 | context 취소 → context 에러 | TestCeleryDispatcher_Dispatch_ContextCancelled | P1 |
| AC-CTRL-005-3 | envelope 직렬화 실패 → RPUSH 0건 | TestCeleryDispatcher_Dispatch_NoRPUSH_WhenEnvelopeFails | P0 |
| (구조 검증) | 골든 파일 유효한 JSON + 필수 키 | TestCeleryEnvelopeGoldenFile_StructureValid | P0 |
| (구조 검증) | 골든 파일 round-trip idempotent | TestCeleryEnvelopeGoldenFile_KeysAlphabeticalOrder | P1 |
| AC-CTRL-005-4 | dispatch p99 < 100ms (레이턴시) | TestCeleryDispatcher_Latency_P99Under100ms | P1 |

---

## 4. Test Scenarios (Sprint 6 RED 완료 기준)

### 4.1 RED Phase 완료 기준 (현재)

- **TestPythonReprList**: FAIL (stub 빈 문자열 반환) — RED 정상
- **TestPythonReprDict**: FAIL (stub 빈 문자열 반환) — RED 정상
- 그 외 13개 테스트: PASS (ErrNotImplemented 반환을 assert)
  - 이는 올바른 RED 설계 — GREEN 구현 시 ErrNotImplemented 제거 → 실제 로직 검증으로 전환

### 4.2 GREEN Phase 목표 (다음 Sprint)

| 테스트 | 현재 (RED) | GREEN 후 목표 |
|--------|-----------|--------------|
| TestCeleryDispatcher_BuildEnvelope_GoldenFileMatch | PASS (ErrNotImplemented assert) | PASS (actual byte match) |
| TestCeleryDispatcher_BuildEnvelope_RequiredFields | PASS (ErrNotImplemented assert) | PASS (field validation) |
| TestPythonReprList | FAIL | PASS |
| TestPythonReprDict | FAIL | PASS |
| TestCeleryDispatcher_Dispatch_RedisRPUSH | PASS (ErrNotImplemented assert) | PASS (actual RPUSH) |
| TestCeleryDispatcher_Dispatch_RedisUnavailable_ReturnsError | PASS (ErrNotImplemented assert) | PASS (ErrDispatchFailed) |

---

## 5. Pass Conditions

### 5.1 GREEN Phase Pass Conditions

- [ ] 15개 테스트 모두 PASS
- [ ] `go test -race ./apps/control-plane/internal/scheduler/...` 0 FAIL
- [ ] `golangci-lint run ./apps/control-plane/...` 0 error
- [ ] `go test -cover ./apps/control-plane/internal/scheduler/...` coverage ≥ 85%
- [ ] `goleak.VerifyNone` 모든 테스트 통과
- [ ] Sprint 1-5 회귀 없음 (audit/server/store/workflow 패키지 all PASS)

### 5.2 Evaluator 4-dim 목표 점수

| Dimension | Target | Description |
|-----------|--------|-------------|
| Originality | ≥ 0.75 | Celery protocol v2 envelope 정확성, golden file match |
| Functionality | ≥ 0.75 | Redis RPUSH, retry, failure handling |
| Completeness | ≥ 0.70 | 모든 AC 커버 |
| Security | ≥ 0.65 | 에러 메시지 노출 최소화, context 취소 처리 |

---

## 6. Implementation Notes (GREEN Phase 가이드)

### 6.1 BuildEnvelope 구현 포인트

- `encoding/json` stdlib만 사용 (외부 의존 없음)
- map key 알파벳 정렬: Go `encoding/json`이 자동 처리 (`map[string]interface{}`)
- body: `[args, kwargs, embed]` JSON 직렬화 후 base64 인코딩
- argsrepr: `pythonReprList([]string{documentID})` → `"['d-uuid']"`
- kwargsrepr: `pythonReprDict(map[string]string{"workflow_id": workflowID})` → `"{'workflow_id': 'uuid'}"`
- headers.id = workflowID
- headers.root_id = workflowID
- properties.correlation_id = workflowID
- properties.delivery_tag = deliveryTag (주입 파라미터)
- properties.reply_to = replyTo (주입 파라미터)
- headers.origin = `"go-control-plane@" + hostname`

### 6.2 pythonReprList 구현

```go
// 예: ["a", "b"] → "['a', 'b']"
func pythonReprList(args []string) string {
    // strings.Builder + fmt.Sprintf 사용
}
```

### 6.3 Dispatch 구현 포인트

- builder가 nil이면 기본 BuildEnvelope 호출
- envelope 생성 실패 → 즉시 ErrEnvelopeSerializationFailed 반환 (RPUSH 없음)
- context.Done() 체크 → context 에러 반환
- RPUSH 실패 → ErrDispatchFailed 래핑 반환
- 3회 exponential backoff는 GREEN에서 추가 가능 (AC-CTRL-005-2 50/200/800ms)

---

## 7. Golden File 관리

골든 파일 경로: `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json`

현재 상태:
- 수동 생성 (hand-crafted) — Python Kombu 스크립트 미실행 환경에서 research.md §3.1 기반 구성
- body base64: `W1siZC1maXhlZC0wMDUtMDAxIl0seyJ3b3JrZmxvd19pZCI6ImZpeGVkLXRlc3QtdXVpZC0wMDUtMDAxIn0seyJjYWxsYmFja3MiOm51bGwsImNoYWluIjpudWxsLCJjaG9yZCI6bnVsbCwiZXJyYmFja3MiOm51bGx9XQ==`
- 이 값은 `[["d-fixed-005-001"],{"workflow_id":"fixed-test-uuid-005-001"},{"callbacks":null,"chain":null,"chord":null,"errbacks":null}]` (compact JSON)을 base64 인코딩한 것

Python Kombu 검증 절차 (Sprint 6 GREEN 후 1회 선택적 실행):
```bash
cd pipelines
python3 scripts/generate_celery_golden.py
# 출력을 apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json과 비교
```

---

## 8. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| R-CTRL-001: Go envelope != Kombu envelope | Low (golden file 있음) | High | golden file round-trip 테스트 |
| Body JSON key 순서 불일치 | Low (encoding/json sort_keys 보장) | Medium | TestCeleryEnvelopeGoldenFile_KeysAlphabeticalOrder |
| Python repr 형식 불일치 | Low | Medium | TestPythonReprList/Dict |

---

Version: 1.0.0
Created: 2026-05-14
Sprint Phase: RED COMPLETE
