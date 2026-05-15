# SPEC-AX-OBS-001 — Final Cross-Validation Report

**Evaluator:** evaluator-active  
**Harness:** thorough (standard harness, final-pass mode)  
**Date:** 2026-05-15  
**SPEC version:** 0.1.2  

---

## Evaluation Report

SPEC: SPEC-AX-OBS-001  
Overall Verdict: **DISPUTE**

### Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 62/100 | FAIL | 5개 AC 미충족: AC-OBS-003-1/2 (HTTP/gRPC instrumentation 미wire), AC-OBS-002-3/4 (401/403 body 포맷 불일치), AC-OBS-EDGE-3 (`go test -race ./...` FAIL) |
| Security (25%) | 80/100 | PASS | 순환 import 0 확인, OTLP default noop, push gateway 0건, /metrics admin-only 로직 존재. IncAuthzForbidden 미호출은 감사 gap(Minor) |
| Craft (20%) | 58/100 | FAIL | metrics 패키지 coverage 65.3%, scheduler `-race` 데이터레이스 존재, 401/403 body 포맷이 spec과 불일치 |
| Consistency (15%) | 75/100 | PASS | server.go 패턴(outerMux 외부 mount) 일관. gRPC interceptor ServerOption 순서 누락은 패턴 위반 |

**Weighted Score:** (62×0.40) + (80×0.25) + (58×0.20) + (75×0.15) = **67.8/100**

---

### Findings

#### [DISPUTE] Missing — gRPC 메트릭 인터셉터 미wire (phantom collector)
**파일:** `apps/control-plane/cmd/server/server.go:205-208`  
`grpcServerOption := auth.BuildGRPCInterceptorChain(s.tokenValidator, nil, s.cfg.AuthEnabled)`  
`s.grpcServer = grpc.NewServer(grpcServerOption)`

SPEC REQ-OBS-003-E2 / AC-OBS-003-2 요구사항: `metrics.UnaryMetricsInterceptor()` ServerOption을 auth ServerOption보다 **앞** 인자로 `grpc.NewServer()`에 전달해야 한다. 현재 server.go에는 `grpc.ChainUnaryInterceptor(metrics.UnaryMetricsInterceptor())` 호출이 전혀 없다. `iroum_ax_grpc_request_duration_seconds` 컬렉터는 registry.go에 정의되어 있으나 **어떤 경로에서도 ObserveGRPCDuration이 호출되지 않는다** — phantom collector. Run agent가 S2 deliverable로 wiring을 명시했으나 실제 server.go에 미반영. AC-OBS-003-2 MISSING.

#### [DISPUTE] Missing — HTTP InstrumentHTTP 래핑 미wire
**파일:** `apps/control-plane/cmd/server/server.go:230-235`

server.go의 `outerMux.Handle("/", auth.BuildRESTChain(...))` 에 `metrics.HTTPInstrumentationMiddleware` 또는 동등한 래핑이 없다. `HTTPInstrumentationMiddleware`는 `internal/metrics/middleware.go:99`에 정의되어 있으나 server.go 어디에도 호출되지 않는다. InstrumentHTTP 계열 함수를 grep해도 server.go에서 발견 없음. `iroum_ax_http_request_duration_seconds`는 middleware_test가 직접 호출 시 동작하지만, **서버 런타임에서 REST 요청이 계측되지 않는다.** AC-OBS-003-1 MISSING.

#### [DISPUTE] Race condition — scheduler 테스트 데이터레이스
**파일:** `apps/control-plane/internal/scheduler/dispatcher_test.go:55`

`go test -race ./...` 실행 시 `TestCeleryDispatcher_Latency_P99Under100ms`가 FAIL (DATA RACE). `mockRedisClient.rpushCalls` 슬라이스를 뮤텍스 없이 병렬 goroutine에서 read/write. AC-OBS-EDGE-3은 "go test -race ./... 전체 통과"를 DoD 조건으로 명시한다. 데이터레이스는 scheduler 패키지 테스트에 국한되나, 전체 `-race` 실행이 FAIL이므로 AC-OBS-EDGE-3 MISSING.

#### [Major] 401/403 body 포맷 불일치
**파일:** `apps/control-plane/internal/metrics/middleware.go:50,56,62,69`

SPEC AC-OBS-002-3: 401 body는 `{"error":{"code":"UNAUTHENTICATED","message":"authentication required for metrics"}}` 형식.  
SPEC AC-OBS-002-4: 403 body는 `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"read:metrics"}}}` 형식.

실제 구현: `writeMetricsError`는 `{"error": "missing_authorization"}`, `{"error": "invalid_request"}`, `{"error": "invalid_token"}`, `{"error": "forbidden"}` 형식을 반환한다. 포맷이 다르다 — `error`가 nested object가 아닌 string. 테스트(`TestMetricsAuthMiddleware_NoToken_Returns401` 등)도 실제 body 포맷을 assert하지 않아 이 불일치를 감지하지 못함. AC-OBS-002-3, AC-OBS-002-4 PARTIAL.

#### [Major] IncAuthzForbidden 미호출 — authz_forbidden_total dead counter
**파일:** `apps/control-plane/internal/metrics/middleware.go:68-70`

```go
if !IsMetricsAuthorized(u) {
    writeMetricsError(w, http.StatusForbidden, "forbidden")
    return
}
```

REQ-OBS-002-U2 / AC-OBS-002-4: 403 반환 시 `iroum_ax_authz_forbidden_total{role=<role>,method="GET /metrics"}`를 1 증가시켜야 한다. 현재 `IncAuthzForbidden` 호출이 없다. `IncAuthzForbidden`은 `registry.go:160`에 정의되어 있으나 `middleware.go`에서 호출되지 않는다. 컬렉터는 정의되어 있으나 authz deny 경로에서 **절대 증가하지 않는다** — dead counter.

#### [Minor] AC-OBS-001-4 reason 불일치 — "algorithm_key_mismatch" vs 스펙 "alg_mismatch"
**파일:** `apps/control-plane/internal/auth/validator.go:229`

SPEC은 `ErrAlgorithmKeyMismatch` 분기의 reason을 `"alg_mismatch"`로 명시한다. 구현은 `v.recordRejection("algorithm_key_mismatch")`를 호출한다. `iroum_ax_auth_rejections_total{reason="alg_mismatch"}` 시계열이 생성되지 않는다. AC-OBS-001-4 PARTIAL.

#### [Minor] permission.go IsMetricsAuthorized 시그니처 — `[]auth.Role` 대신 `*auth.User` 수용
**파일:** `apps/control-plane/internal/metrics/permission.go:18`

SPEC §3.3 / AC-OBS-UBI-001-c: `IsMetricsAuthorized(roles []auth.Role) bool` 시그니처를 명시한다. 구현은 `IsMetricsAuthorized(u *auth.User) bool`이다. 내부 동작은 동일하나 테스트에서 `IsMetricsAuthorized([]auth.Role{auth.RoleAdmin})` 형식으로 호출할 수 없다 — acceptance.md의 Given/When 시나리오와 불일치. Minor(기능은 동작하지만 spec 계약 위반).

#### [Minor] AC-OBS-002-3 서브케이스 누락 — "non-Bearer scheme" 테스트
**파일:** `apps/control-plane/internal/metrics/middleware_test.go`

AC-OBS-002-3은 3개 sub-case를 요구: (i) 헤더 없음, (ii) Bearer scheme 아님, (iii) Verify 실패. 현재 테스트는 (i)와 (iii)만 확인. (ii) `Authorization: Token xyz` 케이스 테스트 누락.

#### [Info] metrics 패키지 coverage 65.3% — 85% threshold 미달
**파일:** `apps/control-plane/internal/metrics/` (coverage: 65.3%)

DoD는 테스트 커버리지 ≥ 85%를 요구한다. metrics 패키지가 65.3%로 가장 낮다. HTTPInstrumentationMiddleware 오류 경로, `MetricsAuthMiddlewareWithUserInjector`, PgPool gauge 등 미테스트 코드가 존재.

---

### Must-Pass Status

| Must-Pass 기준 | 상태 | 근거 |
|----------------|------|------|
| AC-OBS-001-4 순환 import 0 (`auth → metrics` 0건) | PASS | `go list -deps ./internal/auth/ | grep internal/metrics` = 0. DI 정상. |
| REQ-OBS-UBI-001 noop default (외부 egress 0) | PASS | `OTEL_EXPORTER_OTLP_ENDPOINT` 미설정 시 `NeverSample()`. push gateway grep 0건. |
| /metrics admin-only auth (401/403 분기 동작) | PARTIAL-FAIL | 401/403 HTTP 상태 코드는 정확하나 body JSON 포맷이 SPEC과 불일치. IncAuthzForbidden 미호출. 기능 의미상 미충족. |

---

### AC Coverage Table (24건)

| AC ID | 상태 | 비고 |
|-------|------|------|
| AC-OBS-UBI-001-a | IMPLEMENTED | push gateway 0건, noop default |
| AC-OBS-UBI-001-b | IMPLEMENTED | in-memory atomic, hot path I/O 없음 |
| AC-OBS-UBI-001-c | PARTIAL | IsMetricsAuthorized 동작하나 시그니처 다름 |
| AC-OBS-UBI-001-d | IMPLEMENTED | 정규화 route label 확인됨 |
| AC-OBS-001-1 | IMPLEMENTED | 7 collector 등록 (단 pg_pool은 GaugeFunc 별도) |
| AC-OBS-001-2 | IMPLEMENTED | sync.Once 멱등 |
| AC-OBS-001-3 | IMPLEMENTED | default registry에 없음 |
| AC-OBS-001-4 | PARTIAL | DI 동작하나 reason "alg_mismatch" → "algorithm_key_mismatch" 불일치 |
| AC-OBS-001-5 | IMPLEMENTED | workflow 직접 import, Commit 후 hook 확인 |
| AC-OBS-002-1 | IMPLEMENTED | admin → 200 + exposition |
| AC-OBS-002-2 | MISSING | authz_mapping.go에 `/metrics` 행 없음 (grep 결과 없음) |
| AC-OBS-002-3 | PARTIAL | HTTP 상태 401 정확, body 포맷 불일치, sub-case (ii) 테스트 누락 |
| AC-OBS-002-4 | PARTIAL | HTTP 상태 403 정확, body 포맷 불일치, IncAuthzForbidden 미호출 |
| AC-OBS-002-5 | IMPLEMENTED | authEnabled=false → 200 |
| AC-OBS-003-1 | MISSING | HTTPInstrumentationMiddleware 서버 미wire |
| AC-OBS-003-2 | MISSING | gRPC 메트릭 인터셉터 서버 미wire |
| AC-OBS-003-3 | UNVERIFIED | probe 제외 로직 코드 없음 (미wire이므로 irrelevant) |
| AC-OBS-003-4 | UNVERIFIED | panic re-raise 구현 코드 없음 |
| AC-OBS-004-1 | IMPLEMENTED | noop default, shutdown non-nil |
| AC-OBS-004-2 | PARTIAL | OTLP sampler만 설정, exporter 미연결 (Sprint 3 deferral 명시되어 있으나 AC는 그것을 허용하지 않음) |
| AC-OBS-004-3 | IMPLEMENTED | noop fallback 동작 |
| AC-OBS-EDGE-1 | IMPLEMENTED | label cardinality 정적 |
| AC-OBS-EDGE-2 | UNVERIFIED | goleak 테스트 없음, goroutine 정리는 server shutdown에 의존 |
| AC-OBS-EDGE-3 | MISSING | `go test -race ./...` FAIL (scheduler 데이터레이스) |

**Summary: 10 IMPLEMENTED / 5 PARTIAL / 4 MISSING / 3 UNVERIFIED / 2 PARTIAL(authz)**

---

### Recommendations

1. **[DISPUTE-blocking, Priority High]** `cmd/server/server.go`에 gRPC 메트릭 인터셉터 wire 추가:
   ```go
   metricsInterceptor := metrics.UnaryMetricsInterceptor()
   s.grpcServer = grpc.NewServer(
       grpc.ChainUnaryInterceptor(metricsInterceptor),
       grpcServerOption,
   )
   ```
   단, `metrics.UnaryMetricsInterceptor()`가 아직 정의되지 않았을 수 있으므로 `grpc_interceptor.go` 파일 존재 여부도 확인할 것.

2. **[DISPUTE-blocking, Priority High]** `cmd/server/server.go` REST outerMux에 HTTPInstrumentationMiddleware 최외곽 래핑 추가. `/metrics`, `/health`, `/ready` 제외 로직 포함.

3. **[DISPUTE-blocking, Priority High]** `internal/scheduler/dispatcher_test.go` `mockRedisClient`에 `sync.Mutex` 추가하여 `rpushCalls` 슬라이스 동시 접근 보호. `-race` PASS 달성.

4. **[Major, Priority High]** `internal/metrics/middleware.go` `writeMetricsError` 반환 body를 spec 형식으로 수정:
   - 401: `{"error":{"code":"UNAUTHENTICATED","message":"authentication required for metrics"}}`
   - 403: `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"read:metrics"}}}`

5. **[Major, Priority High]** `internal/metrics/middleware.go` authz 실패 분기에 `global().IncAuthzForbidden(role, "GET /metrics")` 호출 추가. role 추출은 `u.Roles` 또는 `u.Scopes`에서.

6. **[Minor, Priority Medium]** `internal/auth/validator.go:229` reason을 `"algorithm_key_mismatch"` → `"alg_mismatch"`로 수정하여 spec 명세와 정합.

7. **[Minor, Priority Medium]** `internal/auth/authz_mapping.go` `restPermissionTable`에 `{method:"GET", pathPrefix:"/metrics", perm:"read:metrics", bypass:false}` 행 추가. AC-OBS-002-2 충족.

8. **[Info, Priority Low]** metrics 패키지 coverage를 85% 이상으로 올리기 위한 추가 테스트 작성.

---

### Overall Recommendation

**FIX-THEN-REVALIDATE**

필수 수정 항목 (DISPUTE 해소 전 진행 불가):
- gRPC metrics interceptor server wiring
- HTTP instrumentation middleware server wiring  
- scheduler 데이터레이스 수정 (`-race` PASS)
- 401/403 body 포맷 수정
- IncAuthzForbidden 호출 추가

위 5건 수정 완료 후 재평가 시 CONFIRM 가능성 높음. OTel OTLP exporter 미연결은 plan.md S3에서 "Sprint 3에서 연결"로 명시되어 있으나 AC-OBS-004-2 자체가 OTLP child span을 요구한다 — 이 항목은 scope 협의가 필요하다.

---

Version: 1.0.0  
Evaluator: evaluator-active  
Profile: default (Functionality 40%, Security 25%, Craft 20%, Consistency 15%)
