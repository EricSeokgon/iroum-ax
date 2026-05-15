# SPEC-AX-OBS-001 독립 교차검증 R3 보고서

SPEC: SPEC-AX-OBS-001 (Observability — Prometheus /metrics + OpenTelemetry Tracing Skeleton)
Round: R3 (3차 재검증)
Evaluator: evaluator-active (독립, 적대적 스탠스)
Date: 2026-05-15
Verdict: **CONFIRM**
Overall Score: 89/100

---

## Evaluation Report

### Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 95/100 | PASS | AC 24/24 GREEN; 7 collector 전부 runtime 경로 실재; probe skip 3 subtest PASS |
| Security (25%) | 90/100 | PASS | circular import = 0; rbac.go frozen(git diff empty); 401/403 spec oracle 일치; race = 0 |
| Craft (20%) | 88/100 | PASS | coverage 87.2% (threshold 85%); false-green 교정; exact-map overbloking 방지 |
| Consistency (15%) | 80/100 | PASS | acceptance.md 리터럴 추종; nested error body 통일; omitempty Details 설계 일관성 |

---

## R2 잔여 3건 해소 확인

### 잔여 1: AC-OBS-003-3 probe path skip (REQ-OBS-003-S1)

**주장:** `middleware.go:124-130` `probePaths` map + `HTTPInstrumentationMiddleware` skip 로직 추가.

**검증:**
- `middleware.go:124-130` 확인: `probePaths = map[string]struct{}{"/health":{}, "/ready":{}, "/metrics":{}}` — exact match map (prefix 아님)
- `middleware.go:141`: `if _, isProbe := probePaths[r.URL.Path]; isProbe { next.ServeHTTP(w, r); return }` — 조기 반환으로 histogram 미기록
- `middleware_test.go:156-201`: `TestHTTPInstrumentationMiddleware_ProbePathsSkipped` — 3 subtest (/health, /ready, /metrics) 각각 (a) 200 OK 통과, (b) histogram 미기록 단언
- `go test ./metrics/... -run TestHTTPInstrumentationMiddleware_ProbePathsSkipped -v` → 3 subtest PASS
- `TestHTTPInstrumentationMiddleware_NonProbePathRecorded` (line 204): 일반 경로(`/api/v1/foo`)는 정상 계측 유지 확인
- **오버블로킹 점검:** `probePaths[r.URL.Path]` = exact lookup. `/metricsX`, `/healthcheck` 등은 map miss → 정상 계측됨. prefix 오판 없음.

**결론: RESOLVED**

---

### 잔여 2: AC-OBS-002-3 401 단일 리터럴 (수정 주장)

**주장:** 모든 401 케이스(헤더 없음/non-Bearer/검증 실패)를 단일 고정 메시지 `"authentication required for metrics"`로 통일.

**검증:**
- acceptance.md L87 oracle: `{"error":{"code":"UNAUTHENTICATED","message":"authentication required for metrics"}}`
- `middleware.go:59`: no-header/non-Bearer → `writeMetricsError(w, 401, "UNAUTHENTICATED", "authentication required for metrics")`
- `middleware.go:65`: Verify 실패 → 동일 메시지
- `middleware.go:105`: authz-only middleware 401 경로 → 동일
- `middleware_test.go:287-288`: `TestMetricsAuthMiddleware_NoToken_NestedErrorBody` — `assert.Contains(t, body, '"authentication required for metrics"')` (spec literal)
- `middleware_test.go:309-310`: non-Bearer 케이스 — 동일 spec literal assert
- `middleware_test.go:329-330`: invalid token 케이스 — 동일 spec literal assert
- **authn 누락 점검:** malformed token 케이스는 `v.Verify()` 오류 분기 처리 — 동일 401 반환. 계측/로깅 누락 없음(counter 호출 경로 별도).

**결론: RESOLVED**

---

### 잔여 3: AC-OBS-002-4 403 PERMISSION_DENIED + false-green 교정

**주장:** `middleware.go:72-79` PERMISSION_DENIED + insufficient scope + details, false-green test 교정.

**검증:**
- acceptance.md L93 oracle: `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"read:metrics"}}}`
- `middleware.go:72-79`:
  ```go
  writeMetricsErrorWithDetails(w, http.StatusForbidden, "PERMISSION_DENIED", "insufficient scope",
      map[string]string{"required": "read:metrics"})
  ```
- `metricsErrorDetail.Details map[string]string` — `json:"details,omitempty"` (401 경우 nil → 필드 생략, 403 경우 non-nil → 포함)
- `writeMetricsErrorWithDetails` — `middleware.go:192-197` 확인
- `middleware_test.go:334-363`: `TestMetricsAuthMiddleware_Forbidden_NestedErrorBody`
  - 주석: "구 false-green: code='FORBIDDEN' assert → spec oracle: code='PERMISSION_DENIED' + details 검증"
  - 단언: `"PERMISSION_DENIED"`, `"insufficient scope"`, `"required"`, `"read:metrics"` (모두 acceptance.md oracle)
- **잠복 false-green 점검:** auth/middleware_test.go — metrics 도메인 code 문자열 assert 없음. "FORBIDDEN"은 테스트 주석과 auth 도메인 상수에만 존재, spec oracle로 사용된 case 없음.

**결론: RESOLVED + false-green 교정 확인**

---

## 무회귀 점검 (R1+R2 RESOLVED 8건)

| 항목 | 검증 방법 | 결과 |
|------|---------|------|
| #1 gRPC interceptor wire | server.go:209-214 `grpcMetricsInterceptor` 확인 | PASS |
| #2 HTTP instrumentation wire | server.go:243-245 `HTTPInstrumentationMiddleware` wrap 확인 | PASS |
| #3 DATA RACE dispatcher | `go test -race ./...` → race 0 | PASS |
| #4 401/403 body format | nested JSON + spec literal (잔여 2/3에서 확인) | PASS |
| #5 IncAuthzForbidden dead | middleware.go:75 `m.IncAuthzForbidden(role, "/metrics")` | PASS |
| #6 /metrics authz_mapping | authz_mapping.go:56 `{GET, /metrics, read:metrics}` | PASS |
| #7 alg_mismatch 문자열 | validator.go:230 `"alg_mismatch"` | PASS |
| #8 coverage 65.3% | `go test ./metrics/... -cover` → 87.2% | PASS |

---

## Must-pass 항목

| 항목 | 명령 / 검증 | 결과 |
|------|------------|------|
| circular import | `go list -deps .../internal/auth/ \| grep -c internal/metrics` = 0 | PASS (0) |
| rbac.go frozen | `git diff --stat HEAD~5 -- .../auth/rbac.go` = empty | PASS |
| OTel noop default | `OTEL_EXPORTER_OTLP_ENDPOINT` 미설정 시 외부 egress 0 (REQ-OBS-UBI-001) | PASS |
| /metrics admin-only | 401 no-token / 403 non-admin / 200 admin 테스트 PASS | PASS |
| race 0 | `go test -race ./apps/control-plane/... -count=1` → ALL PASS | PASS |
| vet/lint clean | `go vet` + `golangci-lint run ./apps/control-plane/...` | PASS |

---

## 적대적 추가 점검 결과

| 점검 항목 | 결과 |
|---------|------|
| probe skip 오버블로킹 (`/metricsX` prefix 오판) | CLEAN — exact map lookup, prefix 불일치는 정상 계측 |
| 401 단일화로 authn 분기 계측/로깅 누락 | CLEAN — Verify 오류 분기도 동일 401 경로 처리 |
| false-green 잠복 (타 테스트에서 구현값 oracle) | CLEAN — "FORBIDDEN"을 spec oracle로 쓰는 assert 없음 |
| 7 collector runtime 증가 경로 phantom | CLEAN — 모든 7 collector에 실제 호출 경로 확인 |

---

## AC 커버리지 요약

**전체 24/24 GREEN** (REQ-OBS-UBI-001×4 + OBS-001×5 + OBS-002×5 + OBS-003×4 + OBS-004×3)

R1 MISSING/PARTIAL → R2 PARTIAL 잔류 → R3 모두 RESOLVED.

---

## Findings

없음. 신규 결함 및 잠복 false-green 미발견.

---

## Recommendations

1. `probePaths` map을 패키지 수준 변수로 노출 (현재 패키지 내부) — 향후 probe path 추가 시 단일 위치 수정 용이. 현재 구현은 기능 요건 충족.
2. Coverage 87.2%는 임계치(85%)를 충분히 초과하나, `metricsAuthzOnlyMiddleware` 경로 추가 커버리지로 90% 이상 도달 가능.

---

## Round Summary

| Round | Verdict | Score | Key Finding |
|-------|---------|-------|-------------|
| R1 | DISPUTE | 67.8 | 8건 결함 (gRPC phantom, HTTP phantom, race, 401/403 format, dead counter, authz mapping, alg_mismatch, coverage 65%) |
| R2 | DISPUTE | 79.0 | 6건 RESOLVED, 2건 PARTIAL (401 메시지/403 code+details), 신규 발견 AC-OBS-003-3 probe skip MISSING |
| R3 | CONFIRM | 89.0 | 3건 잔여 전부 RESOLVED, false-green 교정, overbloking 방지 확인, coverage 87.2% |

---

Version: R3-final
Source: evaluator-active autonomous execution
