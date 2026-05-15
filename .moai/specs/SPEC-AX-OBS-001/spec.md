---
id: SPEC-AX-OBS-001
version: 0.1.2
status: draft
created: 2026-05-15
updated: 2026-05-15
author: ircp
priority: medium
issue_number: 0
---

# HISTORY

- 0.1.2 (2026-05-15): evaluator-active iteration 3 CONFIRM(0.79, plan-auditor PASS 0.97) — genuine spec-to-code 결함 2건 대응. **[Moderate] Circular import 해소 (Run 진입 전 Required)**: `iroum_ax_auth_rejections_total` counter의 source는 `auth.TokenValidator.Verify`(검증 `internal/auth/validator.go:197`, reject 분기: `ErrTokenInvalidIssuer`:279 / `ErrAlgorithmKeyMismatch`:370 / `ErrTokenExpired`:248,257,264 / `ErrTokenBlacklisted`:291)이므로 auth가 `metrics.IncAuthRejection(reason)`를 호출해야 한다(auth → metrics). 그러나 `metrics/permission.go`가 이미 `auth.TokenValidator`/`auth.Role`/`auth.UserFromContext` import(metrics → auth) → **auth → metrics → auth 순환 import**(Go compile error). §2.1에 `internal/auth/middleware.go`가 없던 것은 이 제약의 결과였으나 SPEC이 해결책 미명시 → `auth_rejections_total`이 dead counter(운영 brute-force 탐지 불가)였다. **FIX — Dependency Inversion**: (1) `internal/auth/observer.go`(신규) — `RejectionObserver interface { IncAuthRejection(reason string) }`를 **auth 패키지 내** 선언(auth는 metrics를 import하지 않음). (2) `internal/auth/validator.go`(수정) — `WithRejectionObserver(obs RejectionObserver) ValidatorOption` 추가 + `TokenValidator`에 optional `rejectionObs RejectionObserver` 필드(nil-safe, 미설정 시 no-op) + `Verify` reject 분기에 `recordRejection(reason)` 헬퍼 호출(기존 `New`/`Verify` 시그니처 불변 — additive option pattern, AUTH-001 backward-compat 보존). (3) `internal/metrics/collectors.go`(수정) — `auth.RejectionObserver`를 구조적으로 만족하는 구현체(`IncAuthRejection(reason string)` 메서드 → `iroum_ax_auth_rejections_total{reason}` 증가) 제공. metrics는 interface 만족을 위해 auth를 새로 import하지 않는다(Go 구조적 만족; metrics → auth는 `MetricsAuthMiddleware`로 이미 단방향 존재, 허용). (4) `cmd/server/server.go`(수정) — server.go가 metrics observer를 생성하여 `auth.New(..., auth.WithRejectionObserver(obs))`로 주입(DI wire point; server.go는 auth+metrics 모두 import하는 `package main`). **의존 방향(단방향, no cycle)**: auth는 자체 `RejectionObserver` interface만 정의(metrics import 0). metrics는 그 interface 구현(auth 신규 import 0). server.go가 wire. auth → metrics 간선이 영구 제거됨. **[Minor] workflow_state_transitions_total wiring**: `internal/workflow/state_machine.go`는 import가 `context/fmt/sync/cperrors/types/zap`만(검증 — auth/metrics 모두 import 안 함). `workflow → metrics` 직접 import는 순환 없음(metrics/auth → workflow 간선 부재). **결정: workflow는 metrics를 직접 import**(observer 패턴 미적용 — auth만 순환이 실재하므로 DI 필요; workflow에 동일 패턴을 "일관성"으로 적용하면 zero-benefit over-engineering, Agent Core Behavior #4 Enforce Simplicity). `Start`(L117 Commit 성공 후)/`Complete`(L156)/`Fail`(L202) 각 transition에 `metrics.IncWorkflowTransition(from, to)` hook(commit된 전이만 카운트, from/to는 bounded `types.WorkflowState` → cardinality-safe). §2.1 affected files + §3.2 + §6 + AC(AC-OBS-001-4/5 신규: 두 counter 실제 increment 검증) + §8.1 카운트(REQ-OBS-001 3→5, REQ-mapped 19→21, 총 22→24) 반영. plan.md S2 + research.md circular-import decision + spec-compact 재생성. (작성자: ircp)
- 0.1.1 (2026-05-15): plan-auditor iteration 1 FAIL(0.78, 1 Major + 2 Minor) 대응. **D1(Major) 해소**: `/metrics`가 `auth.BuildRESTChain` 외부 mount이므로 `auth.RESTMiddleware`(chain 내부, `WithUser` populate 유일 지점)를 우회한다 → Bearer token을 `*auth.User`로 파싱하는 컴포넌트 부재로 AC-OBS-002-1/4 구현 불가했던 명세-코드 gap을 **Option A: 전용 `MetricsAuthMiddleware`** 명세로 해소(probes.go의 `/health`·`/ready` bypass와 동일 레벨에서 `/metrics`는 자체 auth 미들웨어 경유 — authn: `auth.TokenValidator.Verify` + `auth.WithUser`, authz: `metrics.IsMetricsAuthorized`). §2.1 affected files + §3.3 REQ-OBS-002 재작성, AC-OBS-002-1/3/4를 MetricsAuthMiddleware 기준(인증 실패 401 + authz 실패 403 분리 검증)으로 재작성. Option B(`/metrics`를 BuildRESTChain 내부 이동 + `read:metrics`를 rbac.go에 추가)는 frozen 위반 + AUTH-002 §13 명세-코드 모순 재발로 **기각**. **D2(Minor) 해소**: §3.4 REQ-OBS-003-E2의 `grpc.ChainUnaryInterceptor(metricsInterceptor, authChain...)`가 타입 불일치(`auth.BuildGRPCInterceptorChain`은 `grpc.ServerOption` 반환, 검증 `internal/auth/chain.go:86-101`) — §2.1과 일치시켜 "metrics `grpc.ChainUnaryInterceptor(...)` ServerOption을 `auth.BuildGRPCInterceptorChain(...)` ServerOption보다 **앞에** `grpc.NewServer(...)`에 전달(gRPC가 option 순서대로 `chainUnaryInts` 누적)"로 정정. **D3(Minor) 해소**: AC count를 실제 enumeration과 정합 — REQ-mapped 19건 + EDGE 3건 = 총 22건으로 통일, disambiguation note 명시. (작성자: ircp)
- 0.1.0 (2026-05-15): SPEC-AX-SERVER-001 v0.1.2 GREEN 후속 (6번째 SPEC). 두 개의 명시적 cross-SPEC unblock을 정식 해소한다: (1) **SPEC-AX-AUTH-002 v0.1.2 §5 Exclusion #13** — `/metrics` Prometheus endpoint 권한 매핑(`read:metrics`) + handler 등록을 후속 SPEC `SPEC-AX-OBS-001`로 분리하라는 지시. (2) **SPEC-AX-SERVER-001 v0.1.2 §5 Exclusion #4·#5** — Distributed tracing(OpenTelemetry) + Prometheus `/metrics` endpoint를 `SPEC-AX-OBS-001`로 deferred. 추가로 `tech.md` §8(Prometheus + Grafana + Loki 모니터링 스택)이 메트릭 정의만 문서로 존재하고 코드 미구현 상태인 gap을 해소한다 (KEPCO E&C 운영 배포 시 SLA 모니터링 필수). 본 SPEC은 (a) `prometheus/client_golang` 도입 + 메트릭 레지스트리 + 7개 core collector, (b) `read:metrics` 권한 처리(research.md decision matrix 3 옵션 중 **Option B: OBS 자체 metrics permission registry** 채택 — AUTH-001 `rbac.go` frozen 회피), (c) `/metrics` endpoint + RBAC 통합, (d) HTTP/gRPC instrumentation middleware/interceptor (metrics 최외곽), (e) OpenTelemetry tracing skeleton(default noop exporter, 망분리 정합)을 보장한다. Composite domain: AX + OBS(관측성 sub-domain — 신규 sub-domain). Sprint Contract Exclusion 10개 명시. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 / SPEC-AX-AUTH-002 / SPEC-AX-SERVER-001과 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 같은 변형 필드는 canonical schema에 없으므로 plan-auditor 결함 제기 시 본 메모와 `plan.md` 마지막 섹션, `lessons_session_2026_05_14 #1`을 출처로 거부한다.

---

# SPEC-AX-OBS-001 — Observability (Prometheus Metrics + OpenTelemetry Tracing Skeleton)

## 1. 개요

`apps/control-plane/`에 운영 관측성 계층을 도입한다. 5개 선행 SPEC(AX-001 / CTRL-001 / AUTH-001 / AUTH-002 / SERVER-001)이 부팅 가능한 dual-listener 서버를 완성했으나, `tech.md` §8이 명시한 Prometheus + Grafana + Loki 모니터링 스택 중 메트릭 노출(`/metrics` endpoint)과 OpenTelemetry trace 골격이 미구현이다. 본 SPEC은 다음을 보장한다:

1. **Metrics Registry & Collectors**: `prometheus/client_golang` 기반 레지스트리 싱글톤 + 7개 core collector
2. **`/metrics` Endpoint + RBAC**: `read:metrics` 권한(OBS 자체 registry) + `authz_mapping.go` 매핑 추가 + `RoleAdmin` only + `promhttp.Handler()` wrapping
3. **HTTP/gRPC Instrumentation**: REST latency/status middleware + gRPC metrics interceptor (둘 다 chain 최외곽 — 인증 실패도 계측). auth reject는 RejectionObserver DI(circular import 회피), workflow 전이는 직접 import hook으로 계측
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

`structure.md` §2 디렉토리 트리를 따른다. 본 SPEC은 **신규 파일 7개**(metrics 6 + `internal/auth/observer.go` 1)를 추가하고, **기존 파일 6개**(`authz_mapping.go` / `cmd/server/server.go` / `config.go` / `scheduler/dispatcher.go` / `auth/validator.go` / `workflow/state_machine.go`)에 wiring/매핑/계측 hook 줄을 추가한다. Delta 마커는 Run phase에서 정확한 라인 단위로 결정한다.

### 2.0 실제 API 검증 결과 (lessons #9 — phantom API 금지, 본 표가 단일 진실)

본 SPEC 작성 전 9개 source file을 Read/Grep으로 정독하여 다음을 확정한다. 모든 wiring/계측 지점은 이 표의 실제 API만 참조한다.

| 본 SPEC 참조 API | 실제 시그니처 (검증됨) | 출처 파일 | 신규/기존 |
|------------------|------------------------|-----------|-----------|
| `auth.LookupRESTPermission` | `func LookupRESTPermission(method, path string) (perm string, bypass bool, found bool)` | `internal/auth/authz_mapping.go:81` | 기존 (매핑 추가 대상) |
| `auth.LookupGRPCPermission` | `func LookupGRPCPermission(fullMethod string) (perm string, bypass bool, found bool)` | `internal/auth/authz_mapping.go:121` | 기존 |
| `auth.restPermissionTable` / `grpcBypassMethods` | 패키지 private `[]restEntry` / `map[string]bool` (`restEntry`: method/pathPrefix/perm/isPrefix/bypass) | `internal/auth/authz_mapping.go:27,64` | 기존 (`/metrics` row 추가) |
| `auth.permissionMatrix` | `map[Role][]Permission` (package **private**, AUTH-001 §3.5 frozen, `read:metrics` 미정의) | `internal/auth/rbac.go:39` | 기존 — **수정 금지(frozen)** |
| `auth.RoleAdmin` / `auth.Role` / `auth.ParseRolesFromScope` | `RoleAdmin Role = "admin"`; `func ParseRolesFromScope(scope string) []Role`; `func EffectivePermissions(roles []Role) map[Permission]bool` | `internal/auth/rbac.go:21,68,85` | 기존 |
| `auth.UserFromContext` | `func UserFromContext(ctx context.Context) (*User, bool)` (`User.Roles []string` + `User.Scopes []string` 보유) | `internal/auth/middleware.go:49` (검증됨, AUTH-001) | 기존 |
| `auth.WithUser` | `func WithUser(ctx context.Context, u *User) context.Context` (context user 주입 단일 진입점, `@MX:ANCHOR`) | `internal/auth/middleware.go:40` (검증됨) | 기존 (MetricsAuthMiddleware authn 단계가 호출) |
| `auth.TokenValidator.Verify` | `func (v *TokenValidator) Verify(ctx context.Context, token string) (*ValidatedToken, error)` (RESTMiddleware:182 / UnaryServerInterceptor:131이 동일 호출; `validatedTokenToUser(vt)`로 `*User` 변환). **reject 분기(= `auth_rejections_total` source)**: `ErrTokenInvalidIssuer`(validator.go:279, reason=`invalid_issuer`) / `ErrAlgorithmKeyMismatch`(:370, `alg_mismatch`) / `ErrTokenExpired`(:248,257,264, `expired`) / `ErrTokenBlacklisted`(:291, `blacklist`) | `internal/auth/middleware.go:131,182` + `internal/auth/validator.go:197~314` (검증됨) | 기존 (MetricsAuthMiddleware authn 단계 + RejectionObserver hook 대상) |
| `auth.TokenValidator` 생성/옵션 | `func New(_ context.Context, oidcIssuer, audience string, opts ...ValidatorOption) (*TokenValidator, error)`; `type ValidatorOption func(*TokenValidator)`; 기존 옵션 `WithIssuer/WithAudience/WithAllowedAlgs/WithClockSkew/WithJWKSProvider/WithBlacklistChecker` (additive option 패턴 확립됨) | `internal/auth/validator.go:115~176` (검증됨) | 기존 — **본 SPEC이 `WithRejectionObserver` 옵션을 동일 패턴으로 additive 추가**(`New`/`Verify` 시그니처 불변) |
| `auth.RejectionObserver` | **본 SPEC S0 신규** — `internal/auth/observer.go`에 `type RejectionObserver interface { IncAuthRejection(reason string) }` 선언. auth 패키지 내 정의(auth는 metrics를 import하지 않음 — circular import 회피 DI). `TokenValidator`가 optional 보유(nil-safe) | `internal/auth/observer.go` (신규, 본 SPEC) | 신규 (auth 패키지, S0) |
| `workflow.StateMachine.Start/Complete/Fail` | `func (sm *StateMachine) Start/Complete/Fail(ctx, workflowID[, ...]) error` — 각 메서드는 `tx.Commit(ctx)` 성공 후 nil 반환(Start:117 / Complete:156 / Fail:202). 패키지 import는 `context/fmt/sync/cperrors/types/zap`만(**auth/metrics 미import — 검증됨**) → `workflow → metrics` 직접 import 순환 없음 | `internal/workflow/state_machine.go:82~204` (검증됨) | 기존 (Commit 성공 후 `metrics.IncWorkflowTransition(from,to)` hook 대상) |
| `auth.RESTMiddleware` | `func RESTMiddleware(validator *TokenValidator) func(http.Handler) http.Handler` — Bearer 파싱→`Verify`→`WithUser`. 단 `/health`만 bypass하고 401 body는 `{"error":"missing_authorization"}` 형식(RFC6750 WWW-Authenticate) → REQ-OBS-002-U1 지정 body와 상이하므로 **`/metrics`는 RESTMiddleware를 재사용하지 않고 MetricsAuthMiddleware가 동일 Verify 흐름을 OBS 지정 401/403 body로 재구현** | `internal/auth/chain.go:71` + `internal/auth/middleware.go:149` (검증됨) | 기존 (참조만 — 재사용 안 함) |
| `auth.RESTAuthzMiddleware` | `func RESTAuthzMiddleware(recorder auditRecorder, authEnabled bool) func(http.Handler) http.Handler` | `internal/auth/authz_middleware.go:90` | 기존 (재사용 안 함, `/metrics`는 MetricsAuthMiddleware 별도 처리) |
| `auth.BuildGRPCInterceptorChain` | `func BuildGRPCInterceptorChain(validator *TokenValidator, recorder auditRecorder, authEnabled bool) grpc.ServerOption` — **`grpc.ServerOption` 반환(인터셉터 아님)**. 내부적으로 `grpc.ChainUnaryInterceptor(UnaryServerInterceptor, UnaryAuthzInterceptor)` ServerOption 생성. `authEnabled=false`면 빈 `grpc.ChainUnaryInterceptor()` ServerOption | `internal/auth/chain.go:86-101` (검증됨) | 기존 (재사용, metrics는 별도 ServerOption 선행) |
| `cmd/server.Server.tokenValidator` | `Server` 구조체 필드 `tokenValidator *auth.TokenValidator` (`auth.New(...)` 결과, server.go:135 set). `Run()`에서 `auth.BuildGRPCInterceptorChain(s.tokenValidator,...)` / `auth.BuildRESTChain(...,s.tokenValidator,...)`에 전달됨(server.go:175,192) | `cmd/server/server.go:44,135,175,192` (검증됨) | 기존 (MetricsAuthMiddleware에 동일 인스턴스 주입) |
| `store.PgWorkflowStore.PoolStats` | `func (s *PgWorkflowStore) PoolStats() *pgxpool.Stat` (`*pgxpool.Stat`은 `AcquiredConns()`/`IdleConns()`/`TotalConns()`/`MaxConns()` int32 메서드 보유) | `internal/store/pg_store.go:92` | 기존 (DB gauge 소스) |
| `scheduler.CeleryDispatcher.Dispatch` | `func (d *CeleryDispatcher) Dispatch(ctx, workflowID, documentID string) error` (성공 nil / 실패 `ErrDispatchFailed` 래핑) | `internal/scheduler/dispatcher.go:70` | 기존 (dispatch counter 계측 지점) |
| `server.RESTHandler.Mux` | `func (h *RESTHandler) Mux() http.Handler` (시그니처 변경 없음) | `internal/server/rest_handler.go:94` | 기존 (instrument 대상) |
| `server.WorkflowService` | `proto.WorkflowServiceServer` 구현 (`CreateWorkflow`/`GetWorkflow`/`ListWorkflows`) | `internal/server/grpc_server.go:36` | 기존 |
| `cmd/server` package | `package main`; `Server.Run(ctx)`가 `outerMux := http.NewServeMux()` 생성(L185) 후 `GET /health`(L186)·`GET /ready`(L187)를 auth chain 외부 mount, 나머지(`/`)는 `auth.BuildRESTChain(s.restHandler.Mux(), s.tokenValidator, nil, s.cfg.AuthEnabled)` wrapping(L190~195). gRPC: `grpcServerOption := auth.BuildGRPCInterceptorChain(s.tokenValidator, nil, s.cfg.AuthEnabled)`(L175) → `s.grpcServer = grpc.NewServer(grpcServerOption)`(L178) | `cmd/server/server.go:171~214` (검증됨) | 기존 (`/metrics` mount + metrics ServerOption 선행 wiring) |
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
| `apps/control-plane/internal/metrics/collectors.go` | 7개 collector 인스턴스(Histogram/Counter/Gauge) + 관측 헬퍼(`ObserveHTTP`/`ObserveGRPC`/`IncWorkflowTransition(from,to)`/`IncCeleryDispatch`/`SetPgPoolConns`/`IncAuthRejection(reason)`/`IncAuthzForbidden`). **추가**: `auth.RejectionObserver` interface를 구조적으로 만족하는 구현체(메서드 `IncAuthRejection(reason string)` → `iroum_ax_auth_rejections_total{reason}` 증가) + 그 인스턴스를 반환하는 생성자(`NewRejectionObserver() *RejectionObserver` 또는 함수 어댑터). metrics는 interface 만족을 위해 auth를 신규 import하지 않음(Go 구조적 만족; metrics → auth는 `MetricsAuthMiddleware` 경유로 이미 단방향 존재) | REQ-OBS-001 | 신규 |
| `apps/control-plane/internal/metrics/permission.go` | OBS 자체 metrics permission registry: (1) `read:metrics` 상수 + `IsMetricsAuthorized(roles []auth.Role) bool`(`RoleAdmin`만 true, AUTH-001 `rbac.go` frozen 회피 — research.md Option B). (2) **`MetricsAuthMiddleware(validator *auth.TokenValidator, authEnabled bool) func(http.Handler) http.Handler`** — `/metrics` 전용 경량 auth 미들웨어. `auth.RESTMiddleware`/`auth.RESTAuthzMiddleware`와 독립(chain 외부 mount 경로 방어). authn 단계: `authEnabled=false`면 통과 / Bearer 부재·파싱 실패·`validator.Verify` 실패 시 REQ-OBS-002-U1 지정 401 body. authz 단계: `auth.UserFromContext`→`auth.ParseRolesFromScope`→`IsMetricsAuthorized` false 시 REQ-OBS-002-U2 지정 403 body + `iroum_ax_authz_forbidden_total` 증가. 통과 시 inner handler(promhttp) 호출 | REQ-OBS-002 | 신규 |
| `apps/control-plane/internal/metrics/http_middleware.go` | REST `InstrumentHTTP(next http.Handler) http.Handler`(latency histogram + status counter, chain 최외곽). `responseWriter` status capture wrapper | REQ-OBS-003 | 신규 |
| `apps/control-plane/internal/metrics/grpc_interceptor.go` | gRPC `UnaryMetricsInterceptor() grpc.UnaryServerInterceptor`(latency histogram + code counter, chain 최외곽) | REQ-OBS-003 | 신규 |
| `apps/control-plane/internal/metrics/tracing.go` | OTel `TracerProvider` 초기화(`InitTracing(cfg) (shutdown func(context.Context) error, error)`) — default noop exporter, `OTelEnabled=true`+`OTLPEndpoint` 설정 시만 OTLP. HTTP/gRPC propagator 등록 헬퍼 | REQ-OBS-004 | 신규 |
| `apps/control-plane/internal/metrics/metrics_test.go` + `tracing_test.go` | collector 등록/관측, permission 분기, middleware status capture, interceptor code 매핑, OTel noop default, label cardinality bound 단위 테스트 | REQ-OBS-001~004 + UBI | 신규 |
| `apps/control-plane/internal/auth/authz_mapping.go` | `restPermissionTable`에 `GET /metrics → read:metrics` 엔트리 1줄 추가(정확 매칭, bypass=false). `LookupRESTPermission`/시그니처 미변경. **`rbac.go` permissionMatrix는 미수정(frozen)** — `read:metrics`는 OBS metrics permission registry가 검증 | REQ-OBS-002 | 수정 (소규모, 1행) |
| `apps/control-plane/internal/auth/observer.go` | **S0 deliverable (circular import 회피 DI)**: `type RejectionObserver interface { IncAuthRejection(reason string) }`를 **auth 패키지 내** 선언. auth는 metrics를 import하지 않으므로 `auth → metrics` 간선이 생기지 않는다. 본 interface는 server.go가 metrics 구현체를 주입하는 DI 경계 | REQ-OBS-001 | 신규 (auth 패키지, S0) |
| `apps/control-plane/internal/auth/validator.go` | **수정 (additive, AUTH-001 backward-compat 보존)**: (1) `WithRejectionObserver(obs RejectionObserver) ValidatorOption` 추가(기존 `WithIssuer`/... 와 동일 패턴), (2) `TokenValidator`에 optional `rejectionObs RejectionObserver` 필드(nil 시 no-op), (3) `Verify`의 reject 분기(`ErrTokenInvalidIssuer`/`ErrAlgorithmKeyMismatch`/`ErrTokenExpired`/`ErrTokenBlacklisted`, 검증 L248~291)에서 nil-safe `recordRejection(reason)` 헬퍼 호출(reason ∈ {invalid_issuer, alg_mismatch, expired, blacklist}). `New`/`Verify` 시그니처 불변 — 기존 호출부(RESTMiddleware/UnaryServerInterceptor/test) 무영향. **`rbac.go`는 미수정(frozen)** — 본 변경은 `validator.go`만 (AUTH-001 frozen 자산이 아님) | REQ-OBS-001 | 수정 (additive option + Verify hook) |
| `apps/control-plane/cmd/server/server.go` | (a) `Server.Run()` `outerMux`에 `GET /metrics`를 `/health`(L186)/`/ready`(L187)와 동일하게 auth chain **외부** mount하되 `metrics.MetricsAuthMiddleware(s.tokenValidator, s.cfg.AuthEnabled)(metrics.Handler())`로 감싼다 (검증된 `s.tokenValidator *auth.TokenValidator` 동일 인스턴스 주입 — wiring 1줄: `outerMux.Handle("GET /metrics", metrics.MetricsAuthMiddleware(s.tokenValidator, s.cfg.AuthEnabled)(metrics.Handler()))`), (b) gRPC `grpc.NewServer(...)`에 metrics interceptor ServerOption을 **최외곽**으로 추가 — `grpc.ChainUnaryInterceptor(metrics.UnaryMetricsInterceptor())` ServerOption을 `auth.BuildGRPCInterceptorChain(...)` ServerOption보다 **앞** 인자로 전달(`grpc.NewServer(grpc.ChainUnaryInterceptor(metricsInterceptor), authServerOption)` — gRPC가 ServerOption 순서대로 `chainUnaryInts` 누적하여 `[metrics, authn, authz]` 보장; 검증 chain.go:90 ServerOption 반환), (c) REST `outerMux`의 비-probe handler를 `metrics.InstrumentHTTP`로 최외곽 wrapping(`/metrics`는 자체 mount이므로 InstrumentHTTP 대상 아님, REQ-OBS-003-S1 제외 정합), (d) `metrics.InitTracing` 호출 + shutdown 시퀀스에 tracer flush 추가, (e) `metrics.SetPgPoolConns` 주기 갱신 goroutine(context-aware, shutdown 시 정리 — lessons #12), (f) **RejectionObserver DI wire (circular import 회피)**: metrics observer 구현체를 생성하여 `auth.New(...)` 호출 시 `auth.WithRejectionObserver(obs)` 옵션으로 주입(server.go는 auth+metrics 모두 import하는 `package main`이므로 DI 단일 지점으로 적합 — `auth → metrics` 간선 회피). `s.tokenValidator`가 검증 실패 시 observer를 통해 `iroum_ax_auth_rejections_total` 증가 | REQ-OBS-001~004 | 수정 (wiring + DI) |
| `apps/control-plane/internal/config/config.go` | **S0 deliverable**: `OTelEnabled bool`(`OTEL_ENABLED`, default false) + `OTLPEndpoint string`(`OTEL_EXPORTER_OTLP_ENDPOINT`, default "") + `MetricsEnabled bool`(`METRICS_ENABLED`, default true) 필드 추가. 기존 `getEnv`/`getBoolEnv` 패턴 재사용, 기존 필드/시그니처 미변경 | REQ-OBS-002 + REQ-OBS-004 | 필드 추가 (S0) |
| `apps/control-plane/internal/scheduler/dispatcher.go` | `Dispatch` 성공/실패 분기에 `metrics.IncCeleryDispatch(status)` 호출 1~2줄 추가(계측 hook). 비즈니스 로직 미변경 | REQ-OBS-001 | 수정 (소규모) |
| `apps/control-plane/internal/workflow/state_machine.go` | **수정 (계측 hook, 직접 import — 순환 없음)**: `Start`(L117 `tx.Commit` 성공 후)·`Complete`(L156)·`Fail`(L202) 각 transition에 `metrics.IncWorkflowTransition(from, to)` 호출 1줄 추가(commit된 전이만 카운트). `from`/`to`는 bounded `types.WorkflowState`(PENDING/RUNNING/COMPLETED/FAILED) → cardinality-safe(REQ-OBS-UBI-001-d 정합). `workflow` 패키지는 auth/metrics 미import이므로 `workflow → metrics` 직접 import는 순환 없음(observer 패턴 미적용 — auth만 실재 순환, 일관성 명목 적용은 over-engineering: research.md §9). 비즈니스 로직/시그니처 미변경 | REQ-OBS-001 | 수정 (계측 hook 3지점) |

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
| `apps/control-plane/internal/metrics/metrics_test.go` | collector 등록 중복 panic 회피, 7개 collector 관측 후 registry gather 값 검증, label cardinality 상한, permission 분기(admin/viewer/anonymous), `MetricsAuthMiddleware` authn/authz 분리 검증(`httptest` + stub `*auth.TokenValidator`: no-auth/non-Bearer/invalid→401, viewer→403, admin→pass, authEnabled=false→pass) | REQ-OBS-001/002 + UBI |
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
| `iroum_ax_auth_rejections_total` | Counter | `reason`(invalid_issuer/alg_mismatch/expired/blacklist) | auth validator reject hook (RejectionObserver DI — REQ-OBS-001-S2) |
| `iroum_ax_authz_forbidden_total` | Counter | `role`, `method` | RBAC deny hook (MetricsAuthMiddleware authz 단계, REQ-OBS-002-U2) |

- **REQ-OBS-001-S2 (auth_rejections_total 수집 아키텍처 — circular import 회피 Dependency Inversion)**: The system SHALL collect `iroum_ax_auth_rejections_total` via a Dependency-Inversion seam, NOT via a direct `auth → metrics` import (which would create a compile-time import cycle `auth → metrics → auth`, because `internal/metrics/permission.go` already imports `auth` for `MetricsAuthMiddleware`). 구체:
  - `internal/auth/observer.go`(신규)가 `RejectionObserver interface { IncAuthRejection(reason string) }`를 **auth 패키지 내** 선언한다. auth 패키지는 metrics를 import하지 않는다(단방향 보존).
  - `auth.TokenValidator`는 optional `RejectionObserver`를 보유하며 `WithRejectionObserver` ValidatorOption으로 주입된다. 미설정(nil) 시 no-op(기존 동작 불변 — AUTH-001 backward-compat).
  - `TokenValidator.Verify`의 4개 reject 분기(`ErrTokenInvalidIssuer`→`invalid_issuer`, `ErrAlgorithmKeyMismatch`→`alg_mismatch`, `ErrTokenExpired`→`expired`, `ErrTokenBlacklisted`→`blacklist`)에서 nil-safe하게 `IncAuthRejection(reason)`를 호출한다.
  - `internal/metrics`가 `auth.RejectionObserver`를 **구조적으로 만족**하는 구현체(메서드 `IncAuthRejection(reason string)` → counter 증가)를 제공한다. metrics는 interface 만족을 위해 auth를 신규 import하지 않는다.
  - `cmd/server/server.go`(`package main`, auth+metrics 모두 import)가 metrics 구현체를 생성하여 `auth.New(..., auth.WithRejectionObserver(obs))`로 주입(DI wire). **최종 의존 방향: auth(interface 정의, metrics import 0) ← metrics(interface 구현) ← server.go(wire). `auth → metrics` 간선 부재 → no cycle.**

- **REQ-OBS-001-S3 (workflow_state_transitions_total 수집 — 직접 import, 순환 없음)**: The system SHALL collect `iroum_ax_workflow_state_transitions_total` via a direct `workflow → metrics` import (no observer indirection). `internal/workflow/state_machine.go`는 auth/metrics를 import하지 않으며 metrics/auth → workflow 간선도 없으므로 `workflow → metrics` 직접 import는 순환을 만들지 않는다. `StateMachine.Start`/`Complete`/`Fail`이 각각 `tx.Commit` 성공 직후 `metrics.IncWorkflowTransition(from, to)`를 호출한다(롤백된 전이는 미카운트). observer DI 패턴은 auth(실재 순환)에만 적용하고 workflow에는 적용하지 않는다 — 순환이 없는데 동일 패턴을 일관성 명목으로 적용하면 zero-benefit over-engineering이다(research.md §9 결정 근거).

#### Event-driven

- **REQ-OBS-001-E1 (Collector registration)**: WHEN the metrics package is initialized (lazy singleton via `sync.Once`), THEN the system SHALL register all 7 collectors into the singleton registry exactly once AND subsequent initialization calls SHALL return the same registry instance without re-registering (no `prometheus.AlreadyRegisteredError` panic).

#### Unwanted

- **REQ-OBS-001-U1 (Default registry 미사용)**: The system SHALL NOT use `prometheus.DefaultRegisterer` / `promauto` global default. IF any code attempts to register a collector outside the singleton registry, THEN the build/test SHALL surface it (단위 테스트가 default registry가 비어 있음을 단언). (글로벌 전역 상태 회피 — go.md MUST NOT, 테스트 격리.)

### 3.3 REQ-OBS-002 — /metrics Endpoint + RBAC (MetricsAuthMiddleware)

> **D1 해소 설계 결정 (Option A)**: `/metrics`는 `/health`·`/ready`와 동일하게 SERVER-001 `outerMux`에 `auth.BuildRESTChain` **외부** mount된다(REQ-OBS-003 instrumentation 최외곽 결정과 정합). `auth.RESTMiddleware`(`*auth.User`를 context에 populate하는 유일 지점, `BuildRESTChain` 내부 — 검증 chain.go:71, middleware.go:195)가 이 경로를 거치지 않으므로, `/metrics`는 전용 경량 미들웨어 **`metrics.MetricsAuthMiddleware`**를 경유한다. 이 미들웨어는 **authentication(token→User)** 와 **authorization(metrics permission)** 을 자체적으로 수행하며, `auth.RESTMiddleware`/`RESTAuthzMiddleware`와 독립이다. Option B(`/metrics`를 `BuildRESTChain` 내부로 이동 + `read:metrics`를 `rbac.go` `permissionMatrix`에 추가)는 (1) `rbac.go` frozen 위반, (2) `authz_middleware`가 `rbac.Authorize` 호출 시 `read:metrics` 미정의로 `ErrInsufficientPermission` 발생(AUTH-002 evaluator가 잡은 명세-코드 모순 패턴 재현)으로 **기각**.

#### Event-driven

- **REQ-OBS-002-E1 (Endpoint exposure via MetricsAuthMiddleware)**: WHEN an HTTP request `GET /metrics` arrives AND `MetricsAuthMiddleware`의 authn+authz 단계를 모두 통과 (또는 `cfg.AuthEnabled=false`), THEN the system SHALL respond HTTP 200 with `Content-Type: text/plain; version=0.0.4` body containing the Prometheus exposition of the singleton registry (`promhttp.HandlerFor(registry, ...)`). The endpoint is mounted on the SERVER-001 `outerMux` outside `auth.BuildRESTChain` (mirrors `/health`/`/ready` chain-external pattern), wrapped instead by `metrics.MetricsAuthMiddleware(s.tokenValidator, s.cfg.AuthEnabled)`.

- **REQ-OBS-002-E2 (authz_mapping registration)**: WHEN `auth.LookupRESTPermission("GET", "/metrics")` is called, THEN it SHALL return `(perm="read:metrics", bypass=false, found=true)` via a new exact-match entry in `restPermissionTable`. 이는 default-deny(503) 회피를 위한 매핑 등록(`authz_mapping_test.go` 검증 대상)이며, 실제 production `/metrics` 경로는 chain 외부이므로 `LookupRESTPermission`을 거치지 않고 `MetricsAuthMiddleware` + OBS metrics permission registry가 검증한다(`rbac.go` permissionMatrix 미수정 — frozen).

#### Event-driven (authentication 단계 — D1)

- **REQ-OBS-002-E3 (MetricsAuthMiddleware authentication)**: WHEN `GET /metrics` 요청이 `MetricsAuthMiddleware`에 진입 AND `cfg.AuthEnabled=true`, THEN the middleware SHALL: (1) `Authorization` 헤더에서 `Bearer <token>`을 파싱, (2) 주입된 `*auth.TokenValidator.Verify(ctx, token)`를 호출하여 토큰을 검증, (3) 성공 시 `auth.WithUser`로 검증된 `*auth.User`를 request context에 주입한 뒤 authorization 단계로 진행. 이 단계는 `auth.RESTMiddleware`와 동일한 `Verify`→User 흐름이되 `/health` bypass가 없고 401 body는 REQ-OBS-002-U1 형식을 따른다(RESTMiddleware의 `{"error":"missing_authorization"}` 형식과 상이하므로 재사용 불가 — §2.0 검증 표 참조).

#### State-driven

- **REQ-OBS-002-S1 (Admin-only authorization)**: WHILE `cfg.AuthEnabled=true` AND authentication 단계(REQ-OBS-002-E3) 성공, THE `MetricsAuthMiddleware` authorization 단계 SHALL extract `*auth.User` via `auth.UserFromContext`, derive roles via `auth.ParseRolesFromScope`, AND grant access (inner promhttp handler 호출) only when `metrics.IsMetricsAuthorized(roles)` returns true (true iff roles contains `auth.RoleAdmin`).

#### Unwanted

- **REQ-OBS-002-U1 (Unauthenticated → 401)**: IF `cfg.AuthEnabled=true` AND (Authorization 헤더 부재 OR Bearer 파싱 실패 OR `TokenValidator.Verify` 실패) for `GET /metrics`, THEN `MetricsAuthMiddleware` SHALL return HTTP 401 Unauthorized with body `{"error":{"code":"UNAUTHENTICATED","message":"authentication required for metrics"}}` AND SHALL NOT invoke the inner promhttp handler (registry exposition 미노출). (`/metrics`는 chain 외부 mount이므로 `MetricsAuthMiddleware`가 인증 부재를 직접 401 처리 — `auth.RESTMiddleware` 우회 경로 방어.)

- **REQ-OBS-002-U2 (Insufficient role → 403)**: IF the authenticated user lacks `auth.RoleAdmin` (e.g. viewer/analyst) for `GET /metrics` AND `cfg.AuthEnabled=true` (즉 authentication은 성공했으나 `metrics.IsMetricsAuthorized(roles)`가 false), THEN `MetricsAuthMiddleware` SHALL return HTTP 403 Forbidden with body `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"read:metrics"}}}`, increment `iroum_ax_authz_forbidden_total{role=<role>,method="GET /metrics"}`, AND SHALL NOT invoke the inner promhttp handler (registry exposition 미노출).

### 3.4 REQ-OBS-003 — HTTP/gRPC Instrumentation Middleware

#### Event-driven

- **REQ-OBS-003-E1 (REST instrumentation — outermost)**: WHEN any HTTP request enters the REST `outerMux` non-probe path, THEN `metrics.InstrumentHTTP` SHALL be the **outermost** wrapper (executes before `auth.BuildRESTChain`), measure wall-clock duration, capture the final response status code via a `ResponseWriter` wrapper, AND observe `iroum_ax_http_request_duration_seconds{method,path,status}` with the normalized route pattern. 인증 실패(401)·인가 실패(403) 요청도 계측된다(최외곽이므로).

- **REQ-OBS-003-E2 (gRPC instrumentation — outermost)**: WHEN any gRPC unary RPC arrives, THEN `metrics.UnaryMetricsInterceptor()` SHALL be composed as the **outermost** interceptor. 실제 wiring: `auth.BuildGRPCInterceptorChain(...)`은 인터셉터가 아닌 `grpc.ServerOption`을 반환하므로(검증 `internal/auth/chain.go:86-101`), metrics 인터셉터는 별도 `grpc.ChainUnaryInterceptor(metrics.UnaryMetricsInterceptor())` ServerOption으로 만들어 `grpc.NewServer(...)` 호출 시 `auth.BuildGRPCInterceptorChain(...)`의 ServerOption보다 **앞 인자**로 전달한다(예: `grpc.NewServer(grpc.ChainUnaryInterceptor(metricsInterceptor), authServerOption)`). gRPC는 복수 `ChainUnaryInterceptor` ServerOption을 전달 순서대로 `chainUnaryInts`에 누적하므로 실행 순서는 `[metrics, authn, authz, handler]`가 보장된다(metrics 최외곽). 인터셉터는 duration을 측정하고 handler error에서 `status.Code(err)`로 gRPC code를 도출하여 `iroum_ax_grpc_request_duration_seconds{method,code}`를 관측한다. 인증 실패(`codes.Unauthenticated`)·인가 실패(`codes.PermissionDenied`)도 metrics가 최외곽이므로 계측된다.

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
- **SPEC-AX-AUTH-001 GREEN 가정 (frozen)**: `rbac.go` `permissionMatrix`(private), `RoleAdmin`/`ParseRolesFromScope`/`EffectivePermissions`/`UserFromContext`/`WithUser`/`TokenValidator.Verify` §2.0 검증됨. **`rbac.go`는 frozen — 본 SPEC은 수정하지 않으며 `read:metrics`는 OBS 자체 registry가 검증** (research.md Option B; AUTH-002 §5 Exclusion #13이 명시한 "rbac.go matrix + rest_handler.go handler 동시 추가 시 명세-코드 모순"을 회피). **D1 정합**: `/metrics`는 `auth.BuildRESTChain` 외부 mount이므로 `auth.RESTMiddleware`(`WithUser` populate 유일 지점, chain.go:71)를 거치지 않는다 → `MetricsAuthMiddleware`가 동일한 `TokenValidator.Verify`→`WithUser` 흐름을 자체 수행하여 authentication을 보장(`auth.RESTMiddleware`는 `/health`-only bypass + 상이한 401 body 형식으로 재사용 불가, §2.0 검증 표 참조). `s.tokenValidator`는 SERVER-001 `Server.New`의 `auth.New(...)` 산출물(server.go:135)이며 OBS는 그 동일 인스턴스를 wiring으로 주입받는다(server.go에 metrics 미들웨어 mount 1줄 추가).
- **SPEC-AX-CTRL-001 GREEN 가정**: `store.PgWorkflowStore.PoolStats()`, `scheduler.CeleryDispatcher.Dispatch`, `server.RESTHandler.Mux()`, `server.WorkflowService` §2.0 검증됨 (계측 hook 대상). `workflow.StateMachine.Start/Complete/Fail`(state_machine.go:82~204, import `context/fmt/sync/cperrors/types/zap`만 — auth/metrics 미import 검증됨) Commit 성공 후 `metrics.IncWorkflowTransition` hook 대상.
- **Circular import 회피 (REQ-OBS-001-S2, evaluator iter 3 Moderate 해소)**: `auth.TokenValidator.Verify` reject 분기가 `auth_rejections_total`의 유일 source이나 `metrics`가 이미 `auth`를 import(`MetricsAuthMiddleware`)하므로 직접 `auth → metrics` 호출은 compile 순환. **Dependency Inversion으로 차단**: `internal/auth/observer.go`(신규)가 `RejectionObserver` interface를 auth 패키지 내 정의 → auth는 metrics를 import하지 않음. metrics가 interface 구현(auth 신규 import 0), server.go가 `auth.WithRejectionObserver`로 DI 주입. 단방향 보존(auth ← metrics ← server.go). `workflow → metrics`는 순환이 없으므로 직접 import(REQ-OBS-001-S3, observer 미적용 — 단순성).
- **Cross-SPEC unblock (lessons #5 + #10)**:
  - **AUTH-002 §5 Exclusion #13 RESOLVED by SPEC-AX-OBS-001**: 본 SPEC GREEN 후 AUTH-002 Exclusion #13은 historical only. AUTH-002 SPEC 파일 자체 수정은 본 SPEC 범위 외(별도 chore commit으로 `RESOLVED by SPEC-AX-OBS-001 v0.1.1` 추가 가능, 또는 미수정 — 본 SPEC은 unblock fact만 보장).
  - **SERVER-001 §5 Exclusion #4 + #5 RESOLVED by SPEC-AX-OBS-001**: 동일 패턴 (historical only, SERVER-001 파일 미수정).
- **Go 의존성**: `github.com/prometheus/client_golang`(promhttp 포함) 신규 require(S0). `go.opentelemetry.io/otel` family는 go.mod에 indirect로 존재 → S0에서 direct 승격 + `otel/sdk` + `otel/sdk/trace` + `otel/exporters/otlp/otlptrace/otlptracegrpc` require 추가. 모두 K8s 내부 동작(외부 fetch 없음).
- **Database**: schema 변경 없음. metric은 audit_logs와 독립(별도 row 미생성).
- **MX tags**:
  - `internal/metrics/registry.go` 싱글톤 함수에 `@MX:ANCHOR`(fan_in ≥ 4: collectors/http_middleware/grpc_interceptor/`/metrics` 핸들러) + `@MX:REASON: 7개 collector 등록 단일 진입점 — 중복 등록 시 panic`.
  - `internal/metrics/http_middleware.go` `InstrumentHTTP`에 `@MX:NOTE`(최외곽 wrapper 의미).
  - `internal/metrics/permission.go` `MetricsAuthMiddleware`에 `@MX:ANCHOR`(`/metrics` 인증/인가 단일 경계 — chain 외부 mount 경로의 유일 방어선) + `@MX:REASON: auth.RESTMiddleware 우회 경로이므로 token→User authn을 자체 수행하지 않으면 인증 우회 발생`.
  - `internal/auth/observer.go` `RejectionObserver` interface에 `@MX:ANCHOR`(circular import 회피 DI 경계 — auth↔metrics 단방향 계약) + `@MX:REASON: 이 interface를 auth 패키지 밖으로 옮기거나 auth가 metrics를 직접 import하면 auth→metrics→auth 순환 import compile error 재발`.
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

- 단위 테스트: `internal/metrics/metrics_test.go` — 7 collector 등록/관측, 중복 등록 회피, default registry 비어있음, label cardinality 상한, permission 분기. `tracing_test.go` — noop default / OTLP opt-in / shutdown 멱등. `internal/auth/validator_test.go`(또는 신규 observer 테스트) — stub `RejectionObserver` 주입 후 `Verify` reject 분기에서 `IncAuthRejection(reason)`가 정확한 reason으로 호출됨 단언(circular import 없이 빌드됨도 암묵 검증). `internal/workflow/state_machine_test.go` — Commit 성공 전이 후 `IncWorkflowTransition(from,to)` 호출 단언(stub/spy collector).
- 통합 테스트: `internal/auth/authz_mapping_test.go`에 `/metrics → read:metrics` 매핑 1건 추가.
- E2E 테스트: `internal/server/metrics_e2e_test.go` — `/metrics` no-auth → 401, viewer → 403, admin → 200 + exposition format, 요청 1건 후 histogram count 증가, OTel noop default.
- 성능 측정: `go test -bench=BenchmarkInstrumentHTTP` (overhead p99 < 1ms), `go test -bench=BenchmarkMetricsScrape` (`/metrics` p99 < 50ms).
- 백워드 호환성: AuthEnabled=false 시 `/metrics` 인증 없이 200, SERVER-001 `TestServer*` regression 보존.
- 보안 검증: push gateway 코드 부재(grep), OTLP default noop 단언, metric label에 PII 부재 단언.

### 8.1 AC 카운트 (D3 정합 — 단일 진실)

`acceptance.md` enumeration과 정확히 일치한다. 카운트 disambiguation:

| 그룹 | AC ID | 건수 |
|------|-------|------|
| REQ-OBS-UBI-001 (4 sub-clause) | AC-OBS-UBI-001-a/b/c/d | 4 |
| REQ-OBS-001 (S2/S3 counter increment 검증 AC-4/5 추가) | AC-OBS-001-1/2/3/4/5 | 5 |
| REQ-OBS-002 (D1로 401/403 분리 검증 추가) | AC-OBS-002-1/2/3/4/5 | 5 |
| REQ-OBS-003 | AC-OBS-003-1/2/3/4 | 4 |
| REQ-OBS-004 | AC-OBS-004-1/2/3 | 3 |
| **소계 (REQ-mapped)** | | **21** |
| Edge/회귀 | AC-OBS-EDGE-1/2/3 | 3 |
| **총계** | | **24** |

> Disambiguation note: "REQ-mapped 21건"은 REQ에 직접 매핑된 AC만 카운트하며, "총 24건"은 EDGE 3건을 포함한다. DoD 및 plan.md는 **총 24건**(REQ-mapped 21 + EDGE 3)을 기준으로 한다. iteration 3(v0.1.2)에서 REQ-OBS-001에 AC-OBS-001-4(`auth_rejections_total`이 RejectionObserver DI를 통해 실제 increment — circular import 없이) + AC-OBS-001-5(`workflow_state_transitions_total`이 직접 import hook으로 실제 increment) 2건을 추가하여 3→5. D1(MetricsAuthMiddleware)으로 AC-OBS-002-1/3/4가 authn/authz 분리 기준으로 재작성된 것은 v0.1.1에서 처리됨(AC ID 개수 5건 불변).

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
