# SPEC-AX-OBS-001 — 구현 계획 (plan.md)

본 계획은 우선순위 기반 Sprint DAG로 구성한다. 시간 추정은 사용하지 않으며(에이전트 공통 프로토콜 — Time Estimation 금지), 의존성 순서와 우선순위 라벨만 사용한다. 개발 방법론은 `quality.yaml` development_mode=`tdd` (brownfield enhancement: 기존 코드 이해 후 RED→GREEN→REFACTOR).

## 1. Sprint DAG 개요

```
S0 (Foundation) ──► S1 (/metrics + RBAC) ──► S2 (Instrumentation) ──► S3 (OTel skeleton + E2E)
   deps + registry        endpoint + collectors      HTTP/gRPC middleware     tracing + full E2E
   + read:metrics 처리
```

- S0 → S1: registry/permission이 있어야 endpoint가 동작
- S1 → S2: collector가 있어야 instrumentation이 관측 대상 보유
- S2 → S3: instrumentation propagation hook이 있어야 OTel span 연결 검증 가능
- S0~S3 순차 (각 Sprint는 직전 Sprint GREEN 전제). 토큰 효율을 위해 S0+S1 결합 가능(lessons 메타 — Sprint combine).

## 2. Sprint 상세

### S0 — Foundation (우선순위: High)

목표: 의존성 + 메트릭 레지스트리 + read:metrics 권한 처리 기반 마련.

Deliverables:
- `go.mod`에 `github.com/prometheus/client_golang` direct require 추가; `go.opentelemetry.io/otel` family를 indirect → direct 승격 + `otel/sdk`, `otel/sdk/trace`, `otel/exporters/otlp/otlptrace/otlptracegrpc` require 추가; `go mod tidy` + `go mod verify`.
- `internal/config/config.go`: `OTelEnabled bool`(`OTEL_ENABLED`, default false) + `OTLPEndpoint string`(`OTEL_EXPORTER_OTLP_ENDPOINT`, default "") + `MetricsEnabled bool`(`METRICS_ENABLED`, default true) 필드 추가 (기존 `getEnv`/`getBoolEnv` 패턴 재사용, 기존 필드/시그니처 미변경).
- `internal/metrics/registry.go`: `sync.Once` 기반 싱글톤 `*prometheus.Registry` + `Handler() http.Handler`(promhttp wrapping). default registry 미사용 (REQ-OBS-001-U1).
- `internal/metrics/collectors.go`: 7개 collector 정의 + 관측 헬퍼 (REQ-OBS-001-S1 canonical 이름).
- `internal/metrics/permission.go`: `read:metrics` 상수 + `IsMetricsAuthorized(roles []auth.Role) bool`(RoleAdmin만 true) — research.md Option B (AUTH-001 `rbac.go` frozen 회피).
- 단위 테스트(RED→GREEN): collector 등록/관측, 중복 등록 panic 회피, default registry 비어있음, permission 분기(admin/viewer/anonymous).

검증: `go test ./internal/metrics/... ./internal/config/...`, registry gather 시 7 collector 노출.

### S1 — /metrics Endpoint + RBAC (우선순위: High)

목표: `/metrics` HTTP 노출 + RBAC 통합 + authz_mapping 등록.

Deliverables:
- `internal/auth/authz_mapping.go`: `restPermissionTable`에 `{method:"GET", pathPrefix:"/metrics", perm:"read:metrics"}` 정확 매칭 엔트리 1줄 추가(bypass=false). `LookupRESTPermission` 시그니처/로직 미변경. **`rbac.go` permissionMatrix 미수정(frozen)**.
- `internal/auth/authz_mapping_test.go`: `GET /metrics → (read:metrics, false, true)` 매핑 테스트 1건 추가.
- `internal/metrics`에 `/metrics` RBAC wrapper handler: `cfg.AuthEnabled=false` → 통과; true → `auth.UserFromContext` 부재 시 401(REQ-OBS-002-U1), `IsMetricsAuthorized` false 시 403 + `iroum_ax_authz_forbidden_total` 증가(REQ-OBS-002-U2), 통과 시 promhttp exposition(REQ-OBS-002-E1).
- `cmd/server/server.go`: `Server.Run()` `outerMux`에 `GET /metrics` mount (auth chain 외부, `/health`/`/ready`와 동일 패턴, RBAC wrapper로 감쌈).
- 단위/통합 테스트(RED→GREEN): no-auth→401, viewer→403, admin→200, AuthEnabled=false→200.

검증: `go test ./internal/metrics/... ./internal/auth/...`, `/metrics` exposition format 단언.

### S2 — HTTP/gRPC Instrumentation (우선순위: High)

목표: REST/gRPC 계측 미들웨어/인터셉터 (chain 최외곽).

Deliverables:
- `internal/metrics/http_middleware.go`: `InstrumentHTTP(next) http.Handler` — duration histogram + status capture wrapper, `/health`/`/ready`/`/metrics` 제외(REQ-OBS-003-S1), panic re-raise(REQ-OBS-003-U1).
- `internal/metrics/grpc_interceptor.go`: `UnaryMetricsInterceptor()` — duration histogram + `status.Code(err)` 매핑.
- `cmd/server/server.go`: REST `outerMux` 비-probe handler를 `InstrumentHTTP`로 **최외곽** wrapping; gRPC `grpc.NewServer`에 `grpc.ChainUnaryInterceptor(metricsInterceptor, <auth chain>)` 최외곽 추가; `metrics.SetPgPoolConns` 주기 갱신 goroutine(context-aware, shutdown 정리 — lessons #12) 추가; `CeleryDispatcher.Dispatch` 계측 hook(`scheduler/dispatcher.go` 성공/실패 분기에 `metrics.IncCeleryDispatch`).
- 단위/벤치 테스트: status capture, code 매핑, overhead p99 < 1ms(`BenchmarkInstrumentHTTP`), probe 제외.

검증: `go test -race ./internal/metrics/...`, 요청 1건 후 histogram count 증가.

### S3 — OpenTelemetry Skeleton + E2E (우선순위: Medium)

목표: OTel tracing 골격 + full E2E.

Deliverables:
- `internal/metrics/tracing.go`: `InitTracing(cfg) (shutdown func(context.Context) error, error)` — noop default(REQ-OBS-004-S1), OTLP opt-in(REQ-OBS-004-O1), init 실패 시 noop fallback non-fatal(REQ-OBS-004-U1), W3C propagator 등록.
- `cmd/server/server.go`: `InitTracing` 호출 + shutdown 시퀀스에 tracer flush 추가(SERVER-001 graceful shutdown sync.Once 정합 — lessons #12).
- HTTP/gRPC instrumentation에 trace context propagate/extract; `workflow.create`/`celery.dispatch` child span hook.
- `internal/server/metrics_e2e_test.go`: `httptest.NewServer` 조합 — `/metrics` 401/403/200, request 후 histogram count, OTel noop default 검증.
- `schemas/openapi/openapi.yaml`: `/metrics` endpoint 추가.

검증: `go test ./internal/server/... -run Metrics`, OTel noop 시 outbound 0건 단언, 전체 회귀(`go test ./...` SERVER-001 regression).

## 3. Risk Register

| 위험 | 영향 | 완화 |
|------|------|------|
| AUTH-001 `rbac.go` frozen amendment 유혹 | `read:metrics`를 permissionMatrix에 넣으면 frozen 위반 + AUTH-002 §13이 명시한 명세-코드 모순 재발 | research.md Option B 채택 — OBS 자체 permission registry. `rbac.go` 미수정을 §2.1/§6/§7에 3중 명시. plan-auditor가 "rbac.go 수정 누락" 제기 시 본 결정 인용하여 거부 |
| Metric cardinality explosion | path/method/status 자유 조합 시 시계열 폭발 → Prometheus OOM | REQ-OBS-UBI-001-d: path는 정규화 라우트 패턴(`/api/v1/workflows/{id}`)만, 라벨 집합 정적 상수화. 단위 테스트로 라벨 카디널리티 상한 단언 |
| Chain order 오류 (metrics가 auth 내부) | 인증 실패 요청이 미계측 → 에러율 SLA 왜곡 | REQ-OBS-003-E1/E2: metrics 최외곽 강제. `grpc.ChainUnaryInterceptor(metrics, authChain)` 순서 + REST `InstrumentHTTP(BuildRESTChain(...))` 순서를 코드로 강제 + 단위 테스트 |
| `/metrics` self-scrape recursion | scrape가 자신을 계측 → 무한 증가 | REQ-OBS-003-S1: `/metrics`/`/health`/`/ready` instrumentation 제외 |
| OTel indirect→direct 승격 시 버전 충돌 | go.mod indirect v1.43.0과 sdk 버전 mismatch | S0에서 `go mod tidy` 후 `go build ./...` 검증. otel core(v1.43.0)와 sdk 버전 정합 확인(동일 major 라인) |
| Prometheus 신규 의존성 transitive 충돌 | client_golang이 기존 grpc/protobuf와 충돌 | S0에서 `go mod verify` + 전체 `go build ./...` 회귀 |
| shutdown 시 PgPool goroutine leak | context cancel 누락 시 goroutine 누수 (lessons #12) | goroutine은 SERVER-001 root context 파생 ctx 구독, `Server.shutdown` 경로에서 자연 종료. `@MX:WARN` + goleak 테스트 |
| 계측 overhead가 1ms 초과 | SLA 측정 자체가 SLA를 침해 | REQ-OBS-UBI-001-b: atomic in-memory만, hot path I/O 금지. `BenchmarkInstrumentHTTP`로 p99 < 1ms 게이트 |

## 4. Cross-SPEC 추적 사슬 (lessons #5 + #10)

- **AUTH-002 §5 Exclusion #13** (`/metrics` 권한+handler 분리) → **본 SPEC `SPEC-AX-OBS-001` REQ-OBS-002**가 정식 해소. 추적: Exclusion #13 → SPEC-AX-OBS-001 → REQ-OBS-002 → AC-OBS-002-*.
- **SERVER-001 §5 Exclusion #4** (Distributed tracing/OTel) → **본 SPEC REQ-OBS-004**가 해소.
- **SERVER-001 §5 Exclusion #5** (Prometheus `/metrics`) → **본 SPEC REQ-OBS-001/002/003**가 해소.
- 본 SPEC GREEN 후 upstream SPEC 파일(AUTH-002/SERVER-001) 자체 수정은 본 SPEC 범위 외 — 별도 chore commit으로 `RESOLVED by SPEC-AX-OBS-001 v0.1.0` 주석 추가 가능 (lessons #5 — cross-SPEC artifact regeneration 명시).

## 5. MX Tag 계획

| 파일/심볼 | 태그 | 사유 |
|-----------|------|------|
| `metrics/registry.go` 싱글톤 | `@MX:ANCHOR` | fan_in ≥ 4 (collectors/http/grpc/endpoint) — 중복 등록 panic 위험 |
| `metrics/http_middleware.go` `InstrumentHTTP` | `@MX:NOTE` | 최외곽 wrapper 의미 + probe 제외 규칙 |
| PgPool 갱신 goroutine | `@MX:WARN` + `@MX:REASON` | context cancel 누락 시 goroutine leak (lessons #12) |
| `cmd/server/server.go` metrics wiring | `@MX:NOTE` | SERVER-001 부팅 시퀀스에 관측성 hook 삽입 지점 |

모두 `code_comments: ko` 적용.

## 6. 완료 조건 (Definition of Done 요약, 상세 acceptance.md)

- 14+ AC 전부 GREEN, 테스트 커버리지 ≥ 85%
- `go test -race ./...` 전체 통과 (SERVER-001/AUTH-002 regression 포함)
- `BenchmarkInstrumentHTTP` p99 < 1ms, `BenchmarkMetricsScrape` p99 < 50ms
- push gateway 코드 부재(grep) + OTLP default noop 단언
- AUTH-002 §13 / SERVER-001 §4·§5 cross-SPEC unblock fact 확인

## 7. Schema note 출처 (plan-auditor 대응)

YAML frontmatter 8-field canonical schema는 `.claude/skills/moai/workflows/plan.md` Phase 2(~L378)에 정의됨 (`id, version, status, created, updated, author, priority, issue_number`). `labels`/`created_at` 변형 필드는 canonical에 없으므로 plan-auditor 결함 제기 시 본 절 + spec.md HISTORY Schema note + `lessons_session_2026_05_14 #1`을 출처로 거부한다.
