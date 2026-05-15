---
id: SPEC-AX-OBS-001
version: 0.1.0
status: draft
created: 2026-05-15
updated: 2026-05-15
author: ircp
priority: medium
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-15): SPEC-AX-SERVER-001 v0.1.2 GREEN 후속 (6번째 SPEC). 두 개의 명시적 cross-SPEC unblock을 정식 해소한다: (1) **SPEC-AX-AUTH-002 v0.1.2 §5 Exclusion #13** — `/metrics` Prometheus endpoint 권한 매핑(`read:metrics`) + handler 등록을 후속 SPEC `SPEC-AX-OBS-001`로 분리하라는 지시. (2) **SPEC-AX-SERVER-001 v0.1.2 §5 Exclusion #4·#5** — Distributed tracing(OpenTelemetry) + Prometheus `/metrics` endpoint를 `SPEC-AX-OBS-001`로 deferred. 추가로 `tech.md` §8(Prometheus + Grafana + Loki 모니터링 스택)이 메트릭 정의만 문서로 존재하고 코드 미구현 상태인 gap을 해소한다 (KEPCO E&C 운영 배포 시 SLA 모니터링 필수). 본 SPEC은 (a) `prometheus/client_golang` 도입 + 메트릭 레지스트리 + 7개 core collector, (b) `read:metrics` 권한 처리(research.md decision matrix 3 옵션 중 **Option B: OBS 자체 metrics permission registry** 채택 — AUTH-001 `rbac.go` frozen 회피), (c) `/metrics` endpoint + RBAC 통합, (d) HTTP/gRPC instrumentation middleware/interceptor (metrics 최외곽), (e) OpenTelemetry tracing skeleton(default noop exporter, 망분리 정합)을 보장한다. Composite domain: AX + OBS(관측성 sub-domain — 신규 sub-domain). Sprint Contract Exclusion 10개 명시. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 / SPEC-AX-AUTH-002 / SPEC-AX-SERVER-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 같은 변형 필드는 canonical schema에 없으므로 plan-auditor 결함 제기 시 본 메모와 `plan.md` 마지막 섹션, `lessons_session_2026_05_14 #1`을 출처로 거부한다.

---

# SPEC-AX-OBS-001 — Observability (Prometheus Metrics + OpenTelemetry Tracing Skeleton)

## 1. 개요

`apps/control-plane/`에 운영 관측성 계층을 도입한다. 5개 선행 SPEC(AX-001 / CTRL-001 / AUTH-001 / AUTH-002 / SERVER-001)이 부팅 가능한 dual-listener 서버를 완성했으나, `tech.md` §8이 명시한 Prometheus + Grafana + Loki 모니터링 스택 중 메트릭 노출(`/metrics` endpoint)과 OpenTelemetry trace 골격이 미구현이다. 본 SPEC은 다음을 보장한다:

1. **Metrics Registry & Collectors**: `prometheus/client_golang` 기반 레지스트리 싱글톤 + 7개 core collector
2. **`/metrics` Endpoint + RBAC**: `read:metrics` 권한(OBS 자체 registry) + `authz_mapping.go` 매핑 추가 + `RoleAdmin` only + `promhttp.Handler()` wrapping
3. **HTTP/gRPC Instrumentation**: REST latency/status middleware + gRPC metrics interceptor (둘 다 chain 최외곽 — 인증 실패도 계측)
4. **OpenTelemetry Tracing Skeleton**: `go.opentelemetry.io/otel` SDK + trace context propagation(HTTP+gRPC) + request→workflow→celery span. exporter default noop(망분리 정합)

### 1.1 본 SPEC의 위상

- **SPEC-AX-AUTH-002 v0.1.2 §5 Exclusion #13 정식 해소**: AUTH-002가 "`/metrics` 권한 매핑 + handler 등록 모두 본 SPEC 범위 외, 후속 SPEC `SPEC-AX-OBS-001` 또는 `SPEC-AX-METRICS-001`로 분리"를 명시했다. 본 SPEC이 `SPEC-AX-OBS-001`로 정식 해소 (lessons #5 + #10 — named follow-up).
- **SPEC-AX-SERVER-001 v0.1.2 §5 Exclusion #4(Distributed tracing) + #5(Prometheus `/metrics`) 정식 해소**: SERVER-001이 두 항목을 "후속 SPEC `SPEC-AX-OBS-001`"로 명시했다. 본 SPEC이 정식 해소.
- `tech.md` §8 메트릭 정의(문서)를 코드 collector로 구현 — 단, 정확한 메트릭 이름은 본 SPEC §3.2가 canonical source (tech.md §8 예시는 문서 sketch이며 충돌 시 본 SPEC §3.2 우선).
- SERVER-001 GREEN 산출물(`cmd/server/server.go` `Server.Run()`의 `outerMux` — `/health`/`/ready` chain 외부 mount)을 신뢰하며 동일 패턴으로 `/metrics`를 추가한다.

### 1.2 운영 컨텍스트 (Why now)

| 동인 | 출처 | 본 SPEC 대응 |
|------|------|-------------|
| KEPCO E&C 운영 배포 시 SLA 모니터링 부재 (응답시간/에러율 관측 불가) | `tech.md` §8 + `product.md` §6.1 Go-Live | REQ-OBS-001 collectors + REQ-OBS-003 instrumentation |
| `/metrics` endpoint 미등록 + `read:metrics` 권한 부재 (cross-SPEC gap) | AUTH-002 §5 Exclusion #13 (evaluator-active iter 3 검증) | REQ-OBS-002 |
| 분산 추적 부재로 request→workflow→celery latency 분해 불가 | SERVER-001 §5 Exclusion #4 | REQ-OBS-004 |
| 망분리 환경에서 push gateway / OTLP 외부 collector 금지 | `tech.md` §9.1 망분리 정책 | REQ-OBS-UBI-001-a |
| Prometheus scrape가 request latency에 영향 주면 SLA 측정 왜곡 | `tech.md` §11.2 API 응답 목표 | REQ-OBS-UBI-001-b |

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `OBS` (관측성/모니터링 sub-domain — 신규 sub-domain)
- SPEC ID: `SPEC-AX-OBS-001` (도메인 카디널리티 2, plan.md 권장 범위 내)

### 1.4 한국 공공 도메인 6 제약 영향 평가 (lessons #8 적용)

- **망분리**: 본 SPEC은 외부 API 호출 0건. Prometheus는 **pull 모델**(내부 scrape only) — push gateway 금지(REQ-OBS-UBI-001-a). OTel exporter는 default **noop**이며 OTLP collector는 K8s 내부 endpoint만 환경변수로 활성(외부 인터넷 노출 불가). 영향 없음(정합 강화).
- **PIPA audit_logs**: 메트릭은 집계 수치(label cardinality 제한)만 노출하며 PII(사용자 식별자/문서 내용)를 label로 사용하지 않는다(REQ-OBS-UBI-001-d). 별도 audit 영향 없음.
- **합니다체**: 메트릭/trace는 영문 식별자(Prometheus 규약). 사용자 노출 한국어 미해당.
- **HWP / 한자한글 / 등급 시뮬레이션**: 본 SPEC 무관 (관측성 횡단 계층).

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 디렉토리 트리를 따른다. 본 SPEC은 **신규 파일 6개**를 추가하고, **기존 파일 3개**에 wiring/매핑 줄을 추가한다. Delta 마커는 Run phase에서 정확한 라인 단위로 결정한다.

### 2.0 실제 API 검증 결과 (lessons #9 — phantom API 금지, 본 표가 단일 진실)

본 SPEC 작성 전 9개 source file을 Read/Grep으로 정독하여 다음을 확정한다. 모든 wiring/계측 지점은 이 표의 실제 API만 참조한다.

| 본 SPEC 참조 API | 실제 시그니처 (검증됨) | 출처 파일 | 신규/기존 |
|------------------|------------------------|-----------|-----------|
| `auth.LookupRESTPermission` | `func LookupRESTPermission(method, path string) (perm string, bypass bool, found bool)` | `internal/auth/authz_mapping.go:81` | 기존 (매핑 추가 대상) |
| `auth.LookupGRPCPermission` | `func LookupGRPCPermission(fullMethod string) (perm string, bypass bool, found bool)` | `internal/auth/authz_mapping.go:121` | 기존 |
| `auth.restPermissionTable` / `grpcBypassMethods` | 패키지 private `[]restEntry` / `map[string]bool` (`restEntry`: method/pathPrefix/perm/isPrefix/bypass) | `internal/auth/authz_mapping.go:27,64` | 기존 (`/metrics` row 추가) |
| `auth.permissionMatrix` | `map[Role][]Permission` (package **private**, AUTH-001 §3.5 frozen, `read:metrics` 미정의) | `internal/auth/rbac.go:39` | 기존 — **수정 금지(frozen)** |
| `auth.RoleAdmin` / `auth.Role` / `auth.ParseRolesFromScope` | `RoleAdmin Role = "admin"`; `func ParseRolesFromScope(scope string) []Role`; `func EffectivePermissions(roles []Role) map[Permission]bool` | `internal/auth/rbac.go:21,68,85` | 기존 |
| `auth.UserFromContext` | `func UserFromContext(ctx context.Context) (*User, bool)` (User.Scopes 보유) | `internal/auth/middleware.go` (검증됨, AUTH-001) | 기존 |
| `auth.RESTAuthzMiddleware` | `func RESTAuthzMiddleware(recorder auditRecorder, authEnabled bool) func(http.Handler) http.Handler` | `internal/auth/authz_middleware.go:90` | 기존 (재사용, `/metrics`는 별도 처리) |
| `store.PgWorkflowStore.PoolStats` | `func (s *PgWorkflowStore) PoolStats() *pgxpool.Stat` (`*pgxpool.Stat`은 `AcquiredConns()`/`IdleConns()`/`TotalConns()`/`MaxConns()` int32 메서드 보유) | `internal/store/pg_store.go:92` | 기존 (DB gauge 소스) |
| `scheduler.CeleryDispatcher.Dispatch` | `func (d *CeleryDispatcher) Dispatch(ctx, workflowID, documentID string) error` (성공 nil / 실패 `ErrDispatchFailed` 래핑) | `internal/scheduler/dispatcher.go:70` | 기존 (dispatch counter 계측 지점) |
| `server.RESTHandler.Mux` | `func (h *RESTHandler) Mux() http.Handler` (시그니처 변경 없음) | `internal/server/rest_handler.go:94` | 기존 (instrument 대상) |
| `server.WorkflowService` | `proto.WorkflowServiceServer` 구현 (`CreateWorkflow`/`GetWorkflow`/`ListWorkflows`) | `internal/server/grpc_server.go:36` | 기존 |
| `cmd/server` package | `package main`; `Server.Run(ctx)`가 `outerMux := http.NewServeMux()` 생성 후 `GET /health`·`GET /ready`를 auth chain 외부 mount, 나머지는 `auth.BuildRESTChain(...)` wrapping (L184~195) | `cmd/server/server.go:171~269` | 기존 (`/metrics` mount + interceptor wiring) |
| `audit.Action` 상수 | `audit.go`에 `ActionAuthForbidden`/`ActionServerStartup` 등 존재; metrics 관련 action 미정의 (본 SPEC은 audit row 미생성 — 메트릭은 audit와 독립) | `internal/audit/audit.go:13~50` | 기존 (변경 없음) |
| `config.Config` | `MetricsEnabled`/`OTelEnabled`/`OTLPEndpoint` 등 관측성 필드 **미존재** → **S0에서 추가** (기존 `getEnv`/`getBoolEnv` 패턴 재사용, 기존 필드 미변경) | `internal/config/config.go:12~99` | 메서드/필드 추가 (S0) |
| `prometheus/client_golang` | go.mod에 **미존재** → **S0에서 require 추가** | `go.mod` (검증됨, 부재) | 신규 의존성 (S0) |
| `go.opentelemetry.io/otel v1.43.0` | go.mod에 **indirect로 존재** (`otel`, `otel/metric`, `otel/trace`, `contrib/.../otelhttp v0.60.0`, `auto/sdk` 모두 indirect) → **S0에서 direct require로 승격** + `otel/sdk` + `otel/sdk/trace` require 추가 | `go.mod:65~69` (검증됨, indirect) | 의존성 승격 (S0) |

### Scope Boundary

본 SPEC은 **관측성 횡단 계층 코드 그 자체**(메트릭 레지스트리 + collector + `/metrics` 핸들러 + instrumentation middleware/interceptor + OTel skeleton)와 그에 대한 단위/E2E 테스트로 한정한다. 비즈니스 핸들러 로직(`WorkflowService`/`RESTHandler`), RBAC 매트릭스(`rbac.go` — frozen), TokenValidator, store/scheduler 내부 구현 등은 본 SPEC 범위 외이며 계측 hook만 추가한다. `cmd/server/server.go`는 본 SPEC이 신규 헬퍼를 호출하는 wiring 줄(수~십 줄)만 추가하며 SERVER-001 부팅 시퀀스 구조는 변경하지 않는다.

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | 모듈 | 신규/수정 |
|------|------|------|---------|
| `apps/control-plane/internal/metrics/registry.go` | `prometheus.Registry` 싱글톤 + 7개 collector 정의/등록 + `Handler() http.Handler`(promhttp wrapping). label cardinality 제한 헬퍼 | REQ-OBS-001 | 신규 |
| `apps/control-plane/internal/metrics/collectors.go` | 7개 collector 인스턴스(Histogram/Counter/Gauge) + 관측 헬퍼(`ObserveHTTP`/`ObserveGRPC`/`IncWorkflowTransition`/`IncCeleryDispatch`/`SetPgPoolConns`/`IncAuthRejection`/`IncAuthzForbidden`) | REQ-OBS-001 | 신규 |
| `apps/control-plane/internal/metrics/permission.go` | OBS 자체 metrics permission registry (`read:metrics` 상수 + `IsMetricsAuthorized(roles []auth.Role) bool` — `RoleAdmin`만 true). AUTH-001 `rbac.go` frozen 회피 (research.md Option B) | REQ-OBS-002 | 신규 |
| `apps/control-plane/internal/metrics/http_middleware.go` | REST `InstrumentHTTP(next http.Handler) http.Handler`(latency histogram + status counter, chain 최외곽). `responseWriter` status capture wrapper | REQ-OBS-003 | 신규 |
| `apps/control-plane/internal/metrics/grpc_interceptor.go` | gRPC `UnaryMetricsInterceptor() grpc.UnaryServerInterceptor`(latency histogram + code counter, chain 최외곽) | REQ-OBS-003 | 신규 |
| `apps/control-plane/internal/metrics/tracing.go` | OTel `TracerProvider` 초기화(`InitTracing(cfg) (shutdown func(context.Context) error, error)`) — default noop exporter, `OTelEnabled=true`+`OTLPEndpoint` 설정 시만 OTLP. HTTP/gRPC propagator 등록 헬퍼 | REQ-OBS-004 | 신규 |
| `apps/control-plane/internal/metrics/metrics_test.go` + `tracing_test.go` | collector 등록/관측, permission 분기, middleware status capture, interceptor code 매핑, OTel noop default, label cardinality bound 단위 테스트 | REQ-OBS-001~004 + UBI | 신규 |
| `apps/control-plane/internal/auth/authz_mapping.go` | `restPermissionTable`에 `GET /metrics → read:metrics` 엔트리 1줄 추가(정확 매칭, bypass=false). `LookupRESTPermission`/시그니처 미변경. **`rbac.go` permissionMatrix는 미수정(frozen)** — `read:metrics`는 OBS metrics permission registry가 검증 | REQ-OBS-002 | 수정 (소규모, 1행) |
| `apps/control-plane/cmd/server/server.go` | (a) `Server.Run()` `outerMux`에 `GET /metrics`를 `/health`/`/ready`와 동일하게 auth chain 외부 mount하되 OBS metrics RBAC wrapper로 감싼다, (b) gRPC `grpc.NewServer` 옵션에 metrics interceptor를 **최외곽**으로 추가(`auth.BuildGRPCInterceptorChain` 결과 앞), (c) REST `outerMux`의 비-probe handler를 `metrics.InstrumentHTTP`로 최외곽 wrapping, (d) `metrics.InitTracing` 호출 + shutdown 시퀀스에 tracer flush 추가, (e) `metrics.SetPgPoolConns` 주기 갱신 goroutine(context-aware, shutdown 시 정리 — lessons #12) | REQ-OBS-002~004 | 수정 (wiring) |
| `apps/control-plane/internal/config/config.go` | **S0 deliverable**: `OTelEnabled bool`(`OTEL_ENABLED`, default false) + `OTLPEndpoint string`(`OTEL_EXPORTER_OTLP_ENDPOINT`, default "") + `MetricsEnabled bool`(`METRICS_ENABLED`, default true) 필드 추가. 기존 `getEnv`/`getBoolEnv` 패턴 재사용, 기존 필드/시그니처 미변경 | REQ-OBS-002 + REQ-OBS-004 | 필드 추가 (S0) |
| `apps/control-plane/internal/scheduler/dispatcher.go` | `Dispatch` 성공/실패 분기에 `metrics.IncCeleryDispatch(status)` 호출 1~2줄 추가(계측 hook). 비즈니스 로직 미변경 | REQ-OBS-001 | 수정 (소규모) |

### 2.2 E2E 테스트 (`apps/control-plane/internal/server/`)

| 경로 | 책임 | 모듈 |
|------|------|------|
| `apps/control-plane/internal/server/metrics_e2e_test.go` | `httptest.NewServer` + `cmd/server` 헬퍼 조합: `/metrics` no-auth → 401, viewer 토큰 → 403, admin 토큰 → 200 + Prometheus exposition format 검증, request 1건 후 `iroum_ax_http_request_duration_seconds` 카운트 증가 검증, OTel noop default 검증 | REQ-OBS-002~004 |

### 2.3 Python Pipelines (`pipelines/`)

본 SPEC은 Go control-plane 관측성에 한정한다. FastAPI 측 Prometheus instrumentation(`prometheus-fastapi-instrumentator`)은 후속 SPEC(`SPEC-AX-OBS-PY-001`)으로 분리하여 scope discipline 유지.

### 2.4 Shared (`pkg/`, `schemas/`)

| 경로 | 책임 | 신규/수정 |
|------|------|---------|
| `schemas/openapi/openapi.yaml` | `/metrics` endpoint 추가 (200 text/plain Prometheus exposition / 401 / 403) | 수정 (소규모) |

### 2.5 Deployments / Database

본 SPEC은 schema 변경 없음. K8s ServiceMonitor / Prometheus scrape config는 인프라 PR(별도 chore — Exclusion #1 참조). 본 SPEC은 `/metrics` HTTP endpoint를 제공할 뿐이다.

### 2.6 Tests

| 경로 | 책임 | 모듈 |
|------|------|------|
| `apps/control-plane/internal/metrics/metrics_test.go` | collector 등록 중복 panic 회피, 7개 collector 관측 후 registry gather 값 검증, label cardinality 상한, permission 분기(admin/viewer/anonymous) | REQ-OBS-001/002 + UBI |
| `apps/control-plane/internal/metrics/tracing_test.go` | `OTelEnabled=false` 시 noop TracerProvider, `OTelEnabled=true`+endpoint 시 OTLP provider, shutdown 멱등 | REQ-OBS-004 + UBI |
| `apps/control-plane/internal/server/metrics_e2e_test.go` | RBAC 통합 E2E + instrumentation E2E (위 §2.2) | REQ-OBS-002/003 |
| `apps/control-plane/internal/auth/authz_mapping_test.go` | 기존 파일에 `GET /metrics → read:metrics, bypass=false, found=true` 매핑 테스트 1건 추가 | REQ-OBS-002 |

---

## 3. EARS 요구사항

EARS 5개 패턴(Ubiquitous / Event-driven / State-driven / Optional / Unwanted) 모두 본 SPEC 내 포함.

### 3.1 Ubiquitous Requirements (시스템 전반 불변 조건)

**REQ-OBS-UBI-001 (망분리 정합 + 무영향 계측 + RBAC 보호 + PII 미노출)**

본 UBI는 4개 sub-clause를 가지며, 각 sub-clause는 acceptance.md에서 dedicated AC를 가진다 (lessons #2 적용).

- **REQ-OBS-UBI-001-a (망분리 — pull only, push gateway 금지)**: The system SHALL expose metrics exclusively via a Prometheus pull endpoint (`GET /metrics`, internal scrape) AND SHALL NOT initiate any outbound connection to a Prometheus push gateway or external metrics backend. OTel exporter는 default noop이며, OTLP exporter는 `cfg.OTelEnabled=true` AND `cfg.OTLPEndpoint != ""`일 때만 활성화되고 그 endpoint는 K8s 내부 주소만 허용(외부 인터넷 endpoint 검증은 인프라/NetworkPolicy 책임이나 default가 noop이므로 망분리 기본 정합).
- **REQ-OBS-UBI-001-b (무영향 계측 — < 1ms overhead)**: The system SHALL ensure metric collection adds no measurable impact to request latency — instrumentation overhead p99 < 1ms per request. Collector 갱신은 in-memory atomic 연산만 사용하고, blocking I/O(DB/network)를 instrumentation hot path에서 수행하지 않는다. PgPool gauge는 요청 경로가 아닌 별도 주기 goroutine(default 15s)에서 갱신한다.
- **REQ-OBS-UBI-001-c (RBAC 보호 — read:metrics 권한 필수)**: WHILE `cfg.AuthEnabled=true`, the system SHALL require the `read:metrics` permission for `GET /metrics` access. `read:metrics`는 OBS 자체 metrics permission registry(`internal/metrics/permission.go`)가 `RoleAdmin`에만 부여하며, AUTH-001 `rbac.go` `permissionMatrix`는 frozen으로 수정하지 않는다(research.md Option B). `cfg.AuthEnabled=false`이면 `/metrics`는 인증 없이 통과(SERVER-001 backward-compat 정합).
- **REQ-OBS-UBI-001-d (PII 미노출 — label cardinality bound)**: The system SHALL NOT use unbounded or PII-bearing values (user ID, document content, raw request body, full URL path with IDs) as metric label values. Path label은 정규화된 라우트 패턴(예: `/api/v1/workflows/{id}`)만 사용하며, 라벨 조합 카디널리티는 정적 상수 집합으로 제한한다.

### 3.2 REQ-OBS-001 — Metrics Registry & Collectors

#### Ubiquitous

- **REQ-OBS-001-S1**: The system SHALL define a single `prometheus.Registry` instance (singleton in `internal/metrics`) registering exactly the following 7 core collectors with the canonical names below (tech.md §8 정합; 충돌 시 본 표가 canonical source):

| 메트릭 이름 | 타입 | 라벨 | 소스 |
|-------------|------|------|------|
| `iroum_ax_http_request_duration_seconds` | Histogram | `method`, `path`(정규화 라우트), `status` | REST instrumentation |
| `iroum_ax_grpc_request_duration_seconds` | Histogram | `method`(FullMethod), `code`(gRPC code 문자열) | gRPC interceptor |
| `iroum_ax_workflow_state_transitions_total` | Counter | `from`, `to` | StateMachine 전이 hook |
| `iroum_ax_celery_dispatch_total` | Counter | `status`(success/failed) | `CeleryDispatcher.Dispatch` |
| `iroum_ax_pg_pool_connections` | Gauge | `state`(acquired/idle/total/max) | `PgWorkflowStore.PoolStats()` |
| `iroum_ax_auth_rejections_total` | Counter | `reason`(invalid_issuer/alg_mismatch/expired/blacklist) | auth validator reject hook |
| `iroum_ax_authz_forbidden_total` | Counter | `role`, `method` | RBAC deny hook |

#### Event-driven

- **REQ-OBS-001-E1 (Collector registration)**: WHEN the metrics package is initialized (lazy singleton via `sync.Once`), THEN the system SHALL register all 7 collectors into the singleton registry exactly once AND subsequent initialization calls SHALL return the same registry instance without re-registering (no `prometheus.AlreadyRegisteredError` panic).

#### Unwanted

- **REQ-OBS-001-U1 (Default registry 미사용)**: The system SHALL NOT use `prometheus.DefaultRegisterer` / `promauto` global default. IF any code attempts to register a collector outside the singleton registry, THEN the build/test SHALL surface it (단위 테스트가 default registry가 비어 있음을 단언). (글로벌 전역 상태 회피 — go.md MUST NOT, 테스트 격리.)

### 3.3 REQ-OBS-002 — /metrics Endpoint + RBAC

#### Event-driven

- **REQ-OBS-002-E1 (Endpoint exposure)**: WHEN an HTTP request `GET /metrics` arrives AND authorization passes (or `cfg.AuthEnabled=false`), THEN the system SHALL respond HTTP 200 with `Content-Type: text/plain; version=0.0.4` body containing the Prometheus exposition of the singleton registry (`promhttp.HandlerFor(registry, ...)`). The endpoint is mounted on the SERVER-001 `outerMux` outside `auth.BuildRESTChain` (mirrors `/health`/`/ready` pattern), wrapped instead by the OBS metrics RBAC wrapper.

- **REQ-OBS-002-E2 (authz_mapping registration)**: WHEN `auth.LookupRESTPermission("GET", "/metrics")` is called, THEN it SHALL return `(perm="read:metrics", bypass=false, found=true)` via a new exact-match entry in `restPermissionTable`. 이는 default-deny(503) 회피를 위한 매핑 등록이며, 실제 `read:metrics` 검증은 OBS metrics permission registry가 수행한다(`rbac.go` permissionMatrix 미수정 — frozen).

#### State-driven

- **REQ-OBS-002-S1 (Admin-only)**: WHILE `cfg.AuthEnabled=true`, THE `/metrics` RBAC wrapper SHALL extract `*auth.User` via `auth.UserFromContext`, derive roles via `auth.ParseRolesFromScope`, AND grant access only when `metrics.IsMetricsAuthorized(roles)` returns true (true iff roles contains `auth.RoleAdmin`).

#### Unwanted

- **REQ-OBS-002-U1 (Unauthenticated → 401)**: IF `cfg.AuthEnabled=true` AND no valid authenticated user is present in context for `GET /metrics`, THEN the system SHALL return HTTP 401 Unauthorized with body `{"error":{"code":"UNAUTHENTICATED","message":"authentication required for metrics"}}` AND SHALL NOT emit the registry exposition. (`/metrics`는 chain 외부 mount이므로 RBAC wrapper가 인증 부재를 직접 401 처리 — `auth.RESTMiddleware` 우회 경로 방어.)

- **REQ-OBS-002-U2 (Insufficient role → 403)**: IF the authenticated user lacks `auth.RoleAdmin` (e.g. viewer/analyst) for `GET /metrics` AND `cfg.AuthEnabled=true`, THEN the system SHALL return HTTP 403 Forbidden with body `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"read:metrics"}}}`, increment `iroum_ax_authz_forbidden_total{role=<role>,method="GET /metrics"}`, AND SHALL NOT emit the registry exposition.

### 3.4 REQ-OBS-003 — HTTP/gRPC Instrumentation Middleware

#### Event-driven

- **REQ-OBS-003-E1 (REST instrumentation — outermost)**: WHEN any HTTP request enters the REST `outerMux` non-probe path, THEN `metrics.InstrumentHTTP` SHALL be the **outermost** wrapper (executes before `auth.BuildRESTChain`), measure wall-clock duration, capture the final response status code via a `ResponseWriter` wrapper, AND observe `iroum_ax_http_request_duration_seconds{method,path,status}` with the normalized route pattern. 인증 실패(401)·인가 실패(403) 요청도 계측된다(최외곽이므로).

- **REQ-OBS-003-E2 (gRPC instrumentation — outermost)**: WHEN any gRPC unary RPC arrives, THEN `metrics.UnaryMetricsInterceptor` SHALL be composed as the **outermost** interceptor (registered before `auth.BuildGRPCInterceptorChain`'s chain via `grpc.ChainUnaryInterceptor(metricsInterceptor, authChain...)`), measure duration, derive the gRPC code from the handler error via `status.Code(err)`, AND observe `iroum_ax_grpc_request_duration_seconds{method,code}`. 인증 실패(`codes.Unauthenticated`)·인가 실패(`codes.PermissionDenied`)도 계측된다.

#### State-driven

- **REQ-OBS-003-S1 (Probe paths excluded)**: WHILE the HTTP path is `/health`, `/ready`, or `/metrics`, the system SHALL NOT record `iroum_ax_http_request_duration_seconds` (probe/scrape self-traffic는 SLA 메트릭을 오염시키므로 제외 — REQ-OBS-UBI-001-d cardinality 정합). `/metrics` self-scrape recursion 방지.

#### Unwanted

- **REQ-OBS-003-U1 (No panic propagation suppression)**: IF the wrapped handler panics, THEN the instrumentation wrapper SHALL still observe the request with `status="500"` (or gRPC code `Internal`) via deferred recovery-aware observation AND SHALL re-panic so existing recovery middleware/behavior is unchanged (instrumentation은 관측만 — 기존 패닉 처리 동작을 삼키지 않는다).

### 3.5 REQ-OBS-004 — OpenTelemetry Tracing Skeleton

#### Optional

- **REQ-OBS-004-O1 (OTLP exporter — opt-in)**: WHERE `cfg.OTelEnabled=true` AND `cfg.OTLPEndpoint != ""`, the system SHALL configure an OTLP trace exporter targeting that internal endpoint with a batch span processor. (Optional 패턴 — OTLP는 선택적 기능이며 default는 미활성.)

#### Event-driven

- **REQ-OBS-004-E1 (Tracer init + propagation)**: WHEN `metrics.InitTracing(cfg)` is called during server bootstrap, THEN it SHALL return a configured `*sdktrace.TracerProvider` (noop/stdout default) and a `shutdown func(context.Context) error`, register a W3C TraceContext propagator, AND the HTTP/gRPC instrumentation SHALL propagate/extract trace context so that a request span has child spans for `workflow.create` and `celery.dispatch` when those operations execute within the request.

#### State-driven

- **REQ-OBS-004-S1 (Noop default — 망분리)**: WHILE `cfg.OTelEnabled=false` (default), THE tracer provider SHALL be a noop (no exporter, zero outbound network) AND `InitTracing` SHALL still return a valid non-nil shutdown function (idempotent no-op) so bootstrap/shutdown wiring is uniform regardless of config.

#### Unwanted

- **REQ-OBS-004-U1 (Init failure non-fatal)**: IF OTLP exporter initialization fails (endpoint unreachable / config error) WHILE `cfg.OTelEnabled=true`, THEN the system SHALL log a structured warning, fall back to a noop tracer provider, AND SHALL NOT abort server startup (관측성 실패가 운영 서비스 가용성을 깨뜨려서는 안 됨 — graceful degradation).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 성능 — 계측 overhead | HTTP/gRPC instrumentation 추가 latency p99 < 1ms/request | REQ-OBS-UBI-001-b |
| 성능 — /metrics scrape | `/metrics` 응답 p99 < 50ms (registry gather, 7 collector) | Prometheus scrape 효율 |
| 성능 — PgPool gauge | 요청 경로가 아닌 별도 goroutine(default 15s tick)에서 갱신 — hot path 무영향 | REQ-OBS-UBI-001-b |
| 보안 — RBAC | `/metrics`는 AuthEnabled=true 시 admin only, 401/403 정확 분기 | REQ-OBS-002 |
| 보안 — 망분리 | push gateway 0건, OTLP default noop, 외부 API 호출 0건 | `tech.md` §9.1 |
| 보안 — PII | metric label에 사용자 식별자/문서 내용/raw path 미포함 | REQ-OBS-UBI-001-d, PIPA |
| 가용성 — 관측성 격리 | OTel/metrics 초기화 실패가 server startup을 abort하지 않음 | REQ-OBS-004-U1 |
| polling-safe | `/metrics`는 K8s/Prometheus 10s 간격 scrape에 안전 (credential 로딩/비싼 호출 없음, registry gather만) | lessons #11 |
| shutdown race | PgPool 갱신 goroutine + tracer flush가 SERVER-001 graceful shutdown에 race 없이 통합(context cancel + sync.Once 정합) | lessons #12 |
| 백워드 호환성 | AuthEnabled=false 시 SERVER-001 / CTRL-001 모든 AC unchanged 통과, `/metrics`는 인증 없이 200 | regression invariant |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | TDD (`quality.yaml` development_mode=tdd; brownfield enhancement — 기존 코드 이해 후 RED) | `quality.yaml` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC/운영 레포에서 다룬다 (target ≥ 8 충족: 10개 명시, lessons #10 — named follow-up).

1. **Grafana 대시보드 정의** — System Health / API Latency / VLM Performance 대시보드 JSON. 본 SPEC은 메트릭 노출만. 후속 SPEC **`SPEC-AX-DASH-001`** 또는 운영 인프라 레포(`deployments/grafana/`).
2. **Loki 로그 집계** — JSON 구조화 로그의 Loki 수집/쿼리 (tech.md §8.2). 본 SPEC은 zap 로깅 변경 없음(SERVER-001 신뢰). 후속 SPEC **`SPEC-AX-LOG-001`**.
3. **Alertmanager 규칙** — SLA 위반 알림 규칙(`PrometheusRule` CRD). 운영 인프라 레포 책임. 후속 chore.
4. **분산 추적 backend (Jaeger / Tempo)** — trace 저장/조회 인프라. 본 SPEC은 OTel SDK skeleton + propagation만, exporter default noop. backend 배포는 운영 인프라 (Exclusion #1 family).
5. **커스텀 SLO/SLI 정의** — error budget, burn-rate 계산. 본 SPEC은 raw 메트릭만. 후속 SPEC **`SPEC-AX-SLO-001`**.
6. **Push gateway** — 망분리 정합 위반(REQ-OBS-UBI-001-a). pull only. 영구 제외 (후속 SPEC에서도 도입 금지).
7. **메트릭 장기 보관 (Thanos / Cortex)** — long-term storage / downsampling. 운영 인프라 책임. 본 SPEC 범위 영구 외.
8. **Exemplars / trace-metric correlation** — metric에 trace_id exemplar 첨부. OTel skeleton 안정화 후 후속 SPEC **`SPEC-AX-OBS-002`**.
9. **Profiling (pprof endpoint)** — `/debug/pprof` CPU/heap 프로파일. 보안 표면 추가로 별도 SPEC **`SPEC-AX-PPROF-001`**(admin only + 망분리 검토).
10. **RED/USE 자동 대시보드 생성** — 메트릭으로부터 대시보드 자동 생성. 후속(Exclusion #1 + #5 family).

---

## 6. 의존성 및 전제

- **SPEC-AX-SERVER-001 v0.1.2 GREEN 가정**: `cmd/server/server.go` `package main`, `Server.Run()`의 `outerMux`(`/health`/`/ready` chain 외부 mount 패턴), `grpc.NewServer(grpcServerOption)` 호출 지점이 모두 §2.0에서 검증됨. 본 SPEC은 동일 패턴으로 `/metrics` mount + interceptor 최외곽 추가.
- **SPEC-AX-AUTH-002 v0.1.2 GREEN 가정**: `auth.LookupRESTPermission`/`LookupGRPCPermission`/`restPermissionTable`/`auth.RESTAuthzMiddleware`/`auth.BuildGRPCInterceptorChain` 시그니처 §2.0 검증됨. 본 SPEC은 `restPermissionTable`에 `/metrics` 행 1줄만 추가하고 lookup 시그니처는 미변경.
- **SPEC-AX-AUTH-001 GREEN 가정 (frozen)**: `rbac.go` `permissionMatrix`(private), `RoleAdmin`/`ParseRolesFromScope`/`EffectivePermissions`/`UserFromContext` §2.0 검증됨. **`rbac.go`는 frozen — 본 SPEC은 수정하지 않으며 `read:metrics`는 OBS 자체 registry가 검증** (research.md Option B; AUTH-002 §5 Exclusion #13이 명시한 "rbac.go matrix + rest_handler.go handler 동시 추가 시 명세-코드 모순"을 회피).
- **SPEC-AX-CTRL-001 GREEN 가정**: `store.PgWorkflowStore.PoolStats()`, `scheduler.CeleryDispatcher.Dispatch`, `server.RESTHandler.Mux()`, `server.WorkflowService` §2.0 검증됨 (계측 hook 대상).
- **Cross-SPEC unblock (lessons #5 + #10)**:
  - **AUTH-002 §5 Exclusion #13 RESOLVED by SPEC-AX-OBS-001**: 본 SPEC GREEN 후 AUTH-002 Exclusion #13은 historical only. AUTH-002 SPEC 파일 자체 수정은 본 SPEC 범위 외(별도 chore commit으로 `RESOLVED by SPEC-AX-OBS-001 v0.1.0` 추가 가능, 또는 미수정 — 본 SPEC은 unblock fact만 보장).
  - **SERVER-001 §5 Exclusion #4 + #5 RESOLVED by SPEC-AX-OBS-001**: 동일 패턴 (historical only, SERVER-001 파일 미수정).
- **Go 의존성**: `github.com/prometheus/client_golang`(promhttp 포함) 신규 require(S0). `go.opentelemetry.io/otel` family는 go.mod에 indirect로 존재 → S0에서 direct 승격 + `otel/sdk` + `otel/sdk/trace` + `otel/exporters/otlp/otlptrace/otlptracegrpc` require 추가. 모두 K8s 내부 동작(외부 fetch 없음).
- **Database**: schema 변경 없음. metric은 audit_logs와 독립(별도 row 미생성).
- **MX tags**:
  - `internal/metrics/registry.go` 싱글톤 함수에 `@MX:ANCHOR`(fan_in ≥ 4: collectors/http_middleware/grpc_interceptor/`/metrics` 핸들러) + `@MX:REASON: 7개 collector 등록 단일 진입점 — 중복 등록 시 panic`.
  - `internal/metrics/http_middleware.go` `InstrumentHTTP`에 `@MX:NOTE`(최외곽 wrapper 의미).
  - PgPool 갱신 goroutine에 `@MX:WARN` + `@MX:REASON: shutdown 시 context cancel 누락하면 goroutine leak — lessons #12 race`.
  - `cmd/server/server.go` metrics wiring 지점에 `@MX:NOTE`(SERVER-001 부팅 시퀀스에 관측성 hook 삽입).
  - 모두 `code_comments: ko` 적용 (mx-tag-protocol.md).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- AUTH-001 `rbac.go` `permissionMatrix`에 `read:metrics` 추가: **금지(frozen)**. 본 SPEC은 OBS 자체 metrics permission registry로 검증 (research.md Option B). `permissionMatrix` 수정은 AUTH-001 amendment SPEC 책임이며 본 SPEC 범위 영구 외.
- Python FastAPI Prometheus instrumentation: 후속 SPEC `SPEC-AX-OBS-PY-001`. 본 SPEC은 Go control-plane만.
- gRPC streaming RPC 계측: 본 SPEC은 unary만 (현재 streaming endpoint 없음). 도입 시 후속 SPEC.
- K8s `ServiceMonitor` / Prometheus scrape config / Helm values: 인프라 chore PR. 본 SPEC은 `/metrics` HTTP endpoint 제공만.
- 메트릭 기반 자동 스케일링(HPA custom metrics adapter): 운영 인프라. 본 SPEC 무관.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `internal/metrics/metrics_test.go` — 7 collector 등록/관측, 중복 등록 회피, default registry 비어있음, label cardinality 상한, permission 분기. `tracing_test.go` — noop default / OTLP opt-in / shutdown 멱등.
- 통합 테스트: `internal/auth/authz_mapping_test.go`에 `/metrics → read:metrics` 매핑 1건 추가.
- E2E 테스트: `internal/server/metrics_e2e_test.go` — `/metrics` no-auth → 401, viewer → 403, admin → 200 + exposition format, 요청 1건 후 histogram count 증가, OTel noop default.
- 성능 측정: `go test -bench=BenchmarkInstrumentHTTP` (overhead p99 < 1ms), `go test -bench=BenchmarkMetricsScrape` (`/metrics` p99 < 50ms).
- 백워드 호환성: AuthEnabled=false 시 `/metrics` 인증 없이 200, SERVER-001 `TestServer*` regression 보존.
- 보안 검증: push gateway 코드 부재(grep), OTLP default noop 단언, metric label에 PII 부재 단언.

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
