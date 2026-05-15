# SPEC-AX-OBS-001 — 재검증 보고서 (R2)

**SPEC:** SPEC-AX-OBS-001 (Observability — Prometheus /metrics + OpenTelemetry Tracing Skeleton)
**Overall Verdict:** DISPUTE (FIX-THEN-REVALIDATE)
**Score:** 79.0/100 (1차 67.8 → +11.2)
**Harness:** thorough | Stance: adversarial | Date: 2026-05-15

---

## Dimension Scores

| Dimension | Score | Verdict | 1차 | 변화 | Evidence |
|-----------|-------|---------|-----|------|----------|
| Functionality (40%) | 76/100 | FAIL | 62 | +14 | 2 ACs 잔여 PARTIAL: AC-OBS-002-3/4 body exact match 불일치 (message text, 403 code, details 필드) |
| Security (25%) | 90/100 | PASS | 80 | +10 | circular 0 확인, noop default 확인, rbac.go frozen 확인, data race 0 |
| Craft (20%) | 80/100 | PASS | 58 | +22 | coverage 85.3% (threshold ≥85% 충족), race PASS 전 패키지, go vet clean |
| Consistency (15%) | 75/100 | PASS | 75 | 0 | server.go 패턴 일관, gRPC option 순서 정상 |

**Hard Threshold:** Security PASS → overall FAIL 방지. Functionality 76/100 FAIL로 전체 DISPUTE 유지.

---

## 8건 항목별 RESOLVED/PARTIAL/UNRESOLVED

### #1 gRPC interceptor phantom — RESOLVED

- `cmd/server/server.go:209` `grpc.ChainUnaryInterceptor(metrics.UnaryMetricsInterceptor(metrics.GlobalMetrics()))` 확인
- `server.go:214` `grpc.NewServer(grpcMetricsInterceptor, grpcAuthOption)` — metrics가 auth ServerOption보다 앞에 전달됨
- `interceptor_test.go` TestUnaryMetricsInterceptor_SuccessObserved, _ErrorObserved, _MultipleRequests 3건 → 모두 PASS (SampleCount≥1 assert, 레이블 검증)
- `iroum_ax_grpc_request_duration_seconds` 런타임 샘플 관측 확인

### #2 HTTP instrumentation phantom — RESOLVED (wiring), 신규 결함 발견

- `server.go:245` `instrumentedHandler := metrics.HTTPInstrumentationMiddleware(metrics.GlobalMetrics())(outerMux)` — outerMux 전체 wrap 확인
- `middleware_test.go:TestInstrumentationMiddleware_RecordsHistogram` PASS — 히스토그램 관측 확인
- **신규 결함**: `HTTPInstrumentationMiddleware`에 path 필터링 없음 → `/health`, `/ready`, `/metrics` 경로도 계측됨
  - spec §3.4 REQ-OBS-003-S1: "WHILE the HTTP path is /health, /ready, or /metrics, the system SHALL NOT record iroum_ax_http_request_duration_seconds"
  - server.go:243 comment도 "probe 포함 계측"으로 명시 — spec 요구사항과 정반대
  - AC-OBS-003-3 테스트 없음 (검색 결과 0건)
  - Severity: **Major** (REQ-OBS-003-S1 직접 위반, AC-OBS-003-3 MISSING)

### #3 DATA RACE dispatcher_test.go — RESOLVED

- `dispatcher_test.go:43` `mu sync.Mutex` 필드 추가 확인
- `RPush()`/`Ping()` 메서드에 `m.mu.Lock()/Unlock()` 적용 확인
- `getRpushCalls()` helper: mutex 보호 하에 슬라이스 복사 반환 (line 75-81)
- `go test -race ./apps/control-plane/internal/scheduler/... -count=1` → `ok ... 1.149s` DATA RACE 0건
- AC-OBS-EDGE-3 RESOLVED

### #4 401/403 body flat→nested — PARTIAL

- nested JSON 구조(`metricsErrorBody`, `metricsErrorDetail`) 구현 확인 ✓
- `middleware.go:156-163` 구조체 정의, `writeMetricsError:170` 사용 확인
- **잔여 불일치 1 (401 message)**:
  - SPEC AC-OBS-002-3 required: `"authentication required for metrics"` (단일 고정 영어 메시지)
  - 구현: `"Authorization 헤더가 없습니다"` / `"Bearer 접두사가 없습니다"` / `"토큰 검증에 실패했습니다"` (케이스별 한국어)
  - 테스트 `TestMetricsAuthMiddleware_NoToken_NestedErrorBody:197` — `"UNAUTHENTICATED"`만 assert, message 정확값 미검증
- **잔여 불일치 2 (403 code)**:
  - SPEC AC-OBS-002-4 required: `code: "PERMISSION_DENIED"`
  - 구현: `code: "FORBIDDEN"` (`middleware.go:82, 117`)
  - 테스트 `TestMetricsAuthMiddleware_Forbidden_NestedErrorBody:219` — `"FORBIDDEN"`으로 assert (spec과 다른 값을 PASS 판정)
- **잔여 불일치 3 (403 details 필드 누락)**:
  - SPEC required: `{"details":{"required":"read:metrics"}}`
  - 구현: `metricsErrorDetail` 구조체에 `Details` 필드 없음
- AC-OBS-002-3/4 PARTIAL → 테스트가 틀린 값을 PASS 처리 중

### #5 IncAuthzForbidden dead counter — RESOLVED

- `middleware.go:76-82` 403 브랜치에서 `m.IncAuthzForbidden(role, "/metrics")` 호출 확인
- `TestMetricsAuthMiddleware_Forbidden_IncAuthzForbiddenCalled` — registry.Gather로 counter=1.0 assert → PASS
- AC-OBS-002-4 (IncAuthzForbidden 부분) RESOLVED

### #6 /metrics restPermissionTable 부재 — RESOLVED, 이중 auth 회귀 없음

- `authz_mapping.go:56` `{method:"GET", pathPrefix:"/metrics", perm:"read:metrics"}` 행 확인
- `TestLookupRESTPermission_Metrics` PASS — `perm="read:metrics", bypass=false, found=true`
- **이중 auth 회귀 없음**: Go 1.22+ ServeMux — `"GET /metrics"` exact pattern이 `"/"` catch-all보다 우선 매칭
  → /metrics 요청은 BuildRESTChain(authz_middleware.go)으로 절대 라우팅되지 않음
  → authz_mapping.go 행은 안전망 용도, 실제 평가 미발생, 회귀 0

### #7 reason "algorithm_key_mismatch" — RESOLVED

- `validator.go:230` `v.recordRejection("alg_mismatch")` 확인
- `TestVerify_AlgMismatch_RecordsAlgMismatchReason` PASS — `obs.reasons[0] == "alg_mismatch"` 단언
- `ErrAlgorithmKeyMismatch` 이름/메시지 불변 (line 400: sentinel error 유지)

### #8 coverage 65.3% → 85.3% — RESOLVED

- `go test ./apps/control-plane/internal/metrics/... -cover -count=1` → `coverage: 85.3% of statements`
- 임계값 ≥85% 충족
- 테스트가 실제 동작 검증 (stub-assert 아님): `registry.Gather()` SampleCount assert, counter 증가 assert

---

## AC Coverage

**1차:** 19/24 (5건 MISSING/PARTIAL)

**R2:**
- AC-OBS-002-3: PARTIAL (code 구조 맞음, message/details spec 불일치)
- AC-OBS-002-4: PARTIAL (code 구조 + IncAuthzForbidden 맞음, code "PERMISSION_DENIED" vs "FORBIDDEN", details 누락)
- AC-OBS-003-3: MISSING (probe 경로 계측 제외 미구현, 테스트 없음) — **신규 발견**
- AC-OBS-EDGE-3: RESOLVED (race 0)
- 기타 gRPC/HTTP phantom, alg_mismatch, coverage: RESOLVED

**R2 AC Coverage:** 21/24 (3건 PARTIAL/MISSING)

---

## Must-pass 재확인 (독립 실행 결과)

| 항목 | 결과 | 증거 |
|------|------|------|
| `go list -deps ./apps/control-plane/internal/auth/ \| grep -c internal/metrics` | **0** | 직접 실행 확인 |
| `git diff --stat HEAD~3 -- apps/control-plane/internal/auth/rbac.go` | **출력 없음 (frozen)** | permissionMatrix 불변 |
| OTel noop default | **PASS** | `tracer.go:40` `trace.NeverSample()`, OTEL_EXPORTER_OTLP_ENDPOINT 미설정 시 외부 egress 0 |
| /metrics admin-only | **PASS** | TestMetricsAuthMiddleware_NoToken→401, ViewerToken→403, AdminToken→200 |
| `go test -race ./apps/control-plane/... -count=1` | **전 패키지 PASS, race 0** | 14개 패키지 모두 ok |
| `go vet ./apps/control-plane/...` | **clean (no output)** | 직접 실행 확인 |
| `golangci-lint run ./apps/control-plane/...` | **clean (no output)** | 직접 실행 확인 |

---

## Findings

### 신규 결함

**[Major] `cmd/server/server.go:243-245` — probe 경로 계측 미제외 (AC-OBS-003-3 MISSING)**

`HTTPInstrumentationMiddleware`가 `outerMux` 전체를 래핑하며 path 필터링이 없음.
`/health`, `/ready`, `/metrics` 요청도 `iroum_ax_http_request_duration_seconds`에 기록됨.
`spec.md` REQ-OBS-003-S1 직접 위반. server.go comment도 "probe 포함 계측"으로 명시 — spec과 역방향.
AC-OBS-003-3 테스트 전무. 이 결함은 R1에서 미발견.

수정 방향: `HTTPInstrumentationMiddleware`에 path skip 로직 추가 (`/health`, `/ready`, `/metrics` 제외),
또는 server.go에서 probe 핸들러를 `instrumentedHandler` 외부에 별도 마운트.

### 잔여 결함 (1차 #4 PARTIAL)

**[Major] `internal/metrics/middleware.go:57,63,69,82,117` — 401/403 body spec exact 불일치**

- 401 message: spec `"authentication required for metrics"` vs 구현 케이스별 한국어 문자열 3종
- 403 code: spec `"PERMISSION_DENIED"` vs 구현 `"FORBIDDEN"`
- 403 details: spec `{"details":{"required":"read:metrics"}}` vs 구현 details 필드 없음
- 테스트 `TestMetricsAuthMiddleware_Forbidden_NestedErrorBody:219`가 `"FORBIDDEN"` assert — 틀린 값으로 PASS

---

## 7 Collector Runtime 증가 경로 확인

| Collector | 런타임 경로 | 상태 |
|-----------|-------------|------|
| `iroum_ax_http_request_duration_seconds` | `HTTPInstrumentationMiddleware` → `ObserveHTTPDuration` | WIRED (probe 제외 미구현) |
| `iroum_ax_grpc_request_duration_seconds` | `UnaryMetricsInterceptor` → `ObserveGRPCDuration` | WIRED (RESOLVED) |
| `iroum_ax_workflow_state_transitions_total` | `state_machine.go` Start/Complete/Fail → `IncWorkflowTransition` | WIRED |
| `iroum_ax_celery_dispatch_total` | `dispatcher.go:98,103` → `IncCeleryDispatch` | WIRED |
| `iroum_ax_pg_pool_connections` | `server.go:102` `RegisterPgPoolGauge` GaugeFunc | WIRED |
| `iroum_ax_auth_rejections_total` | `validator.go:recordRejection` → `RejectionObserver.IncAuthRejection` | WIRED |
| `iroum_ax_authz_forbidden_total` | `middleware.go:81,116` → `IncAuthzForbidden` | WIRED (RESOLVED) |

---

## Recommendations

### FIX-THEN-REVALIDATE (잔여 수정 목록)

1. **[Must] AC-OBS-003-3 — probe 경로 계측 제외 구현**
   `HTTPInstrumentationMiddleware` 또는 server.go 마운트 구조 수정:
   path가 `/health`, `/ready`, `/metrics`인 경우 `ObserveHTTPDuration` 호출 생략.
   `TestHTTPInstrumentationMiddleware_ProbePathExcluded` 테스트 추가 필수.

2. **[Must] AC-OBS-002-3 — 401 message 정확 일치**
   모든 401 케이스를 단일 고정 메시지 `"authentication required for metrics"`로 통일.
   `TestMetricsAuthMiddleware_NoToken_NestedErrorBody`에 message 정확값 assert 추가.

3. **[Must] AC-OBS-002-4 — 403 code/details 수정**
   - `code: "FORBIDDEN"` → `code: "PERMISSION_DENIED"`
   - `metricsErrorDetail`에 `Details map[string]any` 필드 추가
   - `writeMetricsError` 확장 또는 403 전용 writer 분리
   - `TestMetricsAuthMiddleware_Forbidden_NestedErrorBody`에 `"PERMISSION_DENIED"` + details assert 추가

---

**Report file:** `/home/sklee/moai/iroum-ax/.moai/reports/evaluator/SPEC-AX-OBS-001-final-crossval-r2.md`
