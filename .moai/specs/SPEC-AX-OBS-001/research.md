# SPEC-AX-OBS-001 — 연구 노트 (research.md)

본 문서는 SPEC 작성 전 코드베이스 분석 결과 + 핵심 아키텍처 결정의 근거를 보존한다. lessons #9(phantom API 금지) 적용 — 모든 참조 API는 Read/Grep으로 사전 검증됨.

## 1. 검증된 API (lessons #9, spec.md §2.0 단일 진실 요약)

| 심볼 | 시그니처 | 출처 |
|------|----------|------|
| `auth.LookupRESTPermission` | `(method, path string) (perm string, bypass bool, found bool)` | `authz_mapping.go:81` |
| `auth.restPermissionTable` | private `[]restEntry{method,pathPrefix,perm,isPrefix,bypass}` | `authz_mapping.go:27` |
| `auth.permissionMatrix` | private `map[Role][]Permission` (AUTH-001 frozen, `read:metrics` 미정의) | `rbac.go:39` |
| `auth.RoleAdmin`/`ParseRolesFromScope`/`EffectivePermissions`/`UserFromContext` | `rbac.go:21,68,85`; `middleware.go` | 검증됨 |
| `auth.RESTAuthzMiddleware`/`UnaryAuthzInterceptor`/`BuildGRPCInterceptorChain`/`BuildRESTChain` | `authz_middleware.go:90,163`; `chain.go:43,86` | 검증됨 |
| `store.PgWorkflowStore.PoolStats()` | `*pgxpool.Stat` (AcquiredConns/IdleConns/TotalConns/MaxConns int32) | `pg_store.go:92` |
| `scheduler.CeleryDispatcher.Dispatch` | `(ctx,workflowID,documentID string) error` (nil/`ErrDispatchFailed`) | `dispatcher.go:70` |
| `server.RESTHandler.Mux()` / `WorkflowService` | `http.Handler` / `proto.WorkflowServiceServer` | `rest_handler.go:94`, `grpc_server.go:36` |
| `cmd/server` `Server.Run()` | `package main`, `outerMux` (`/health`·`/ready` chain 외부 mount, L184~195) | `server.go:171` |
| `auth.TokenValidator.Verify` reject 분기 | `auth_rejections_total` source — `ErrTokenInvalidIssuer`:279 / `ErrAlgorithmKeyMismatch`:370 / `ErrTokenExpired`:248,257,264 / `ErrTokenBlacklisted`:291; `New`/options additive 패턴(L115~176) | `validator.go:197~314` |
| `auth` 패키지 import | `context/crypto/encoding/errors/fmt/strings/time/golang-jwt` — **metrics 미import**(순환 회피 DI 전제) | `validator.go:1~17`, `middleware.go:1~16` |
| `workflow.StateMachine` | `Start`:82 / `Complete`:123 / `Fail`:163 (각 `tx.Commit` 성공 후 nil); 패키지 import `context/fmt/sync/cperrors/types/zap`만 — **auth/metrics 미import** | `workflow/state_machine.go:82~204` |
| `config.Config` | OTel/metrics 필드 **부재** (S0 추가 대상) | `config.go:12~99` |
| `go.mod` | `prometheus/client_golang` **부재**; `go.opentelemetry.io/otel v1.43.0` family **indirect** | `go.mod:9,65~69` |

핵심: AUTH-002 §5 Exclusion #13이 지적한 정확한 gap이 코드로 재확인됨 — `rbac.go` `permissionMatrix`에 `read:metrics` 없음, `rest_handler.go` `Mux()`에 `/metrics` route 없음. SERVER-001 `outerMux`가 `/health`/`/ready`를 chain 외부 mount하는 패턴이 `/metrics` 처리의 직접 선례.

## 2. Decision Matrix — `read:metrics` 권한 처리 (3 옵션, lesson #9)

AUTH-002 §5 Exclusion #13의 핵심 모순: "`/metrics` 권한 매핑만 추가하면 cross-SPEC 변경(rbac.go matrix + rest_handler.go handler 동시 추가)이 필요하여 명세-코드 모순". 본 SPEC은 `rbac.go`가 AUTH-001 frozen 자산임을 전제로 3개 옵션을 평가했다.

| 옵션 | 설명 | 장점 | 단점 | 판정 |
|------|------|------|------|------|
| **A. AUTH-001 minor amendment** | `rbac.go` `permissionMatrix[RoleAdmin]`에 `read:metrics` 추가 + AUTH-001 SPEC 파일 amendment | RBAC 단일 진실 유지, `auth.Authorize` 재사용 | AUTH-001 frozen 위반; AUTH-002 §13이 명시한 "명세-코드 모순" 정확히 재발; cross-SPEC frozen 자산 수정 — plan-auditor/evaluator 하드 결함 위험 (lessons #9 phantom 유사 — frozen 침범) | **기각** |
| **B. OBS 자체 metrics permission registry** | `internal/metrics/permission.go`에 `read:metrics` 상수 + `IsMetricsAuthorized(roles []auth.Role) bool`(RoleAdmin만). `rbac.go` 미수정. `authz_mapping.go`엔 default-deny(503) 회피용 매핑 1행만 추가 | `rbac.go` frozen 보존; AUTH-002 §13 모순 회피; metrics 권한이 OBS 도메인에 응집(관측성 권한은 관측성 SPEC 소유); 검증 로직 단위 테스트 격리 용이 | RBAC 진실이 2곳(rbac.go + metrics registry)으로 분산 — 단, metrics 권한은 OBS 전용이므로 도메인 경계상 합리적; `auth.Authorize` 직접 재사용 불가(IsMetricsAuthorized로 대체) | **채택** |
| **C. config flag (RBAC 우회)** | `cfg.MetricsRequireAdmin` 플래그로 admin 체크를 config 분기로만 처리, 권한 모델 미사용 | 구현 최소 | RBAC 모델과 분리된 ad-hoc 보안 — 일관성 붕괴, 감사 추적 약화, "권한"이 아닌 "토글"로 퇴화; PIPA 추적성 약화 | **기각** |

**채택: Option B.** 근거: (1) AUTH-001 `rbac.go` frozen 보존이 AUTH-002 §13이 명시적으로 요구한 제약. (2) 관측성 권한(`read:metrics`)은 관측성 도메인(OBS)에 응집하는 것이 도메인 경계상 자연스럽다(SPEC-AX-OBS-001 = OBS sub-domain). (3) `authz_mapping.go`엔 default-deny 회피용 1행만 추가하므로 AUTH-002 wiring 계층은 변경 최소(`LookupRESTPermission` 시그니처 불변). (4) `IsMetricsAuthorized(roles)`는 `auth.ParseRolesFromScope` + `RoleAdmin` 비교만 — `auth.permissionMatrix`를 읽지 않으므로 frozen 의존성 0.

향후 RBAC 통합이 필요하면(예: `analyst`에도 read:metrics 부여) AUTH-001 amendment SPEC을 별도로 발행하여 Option B → Option A 마이그레이션 (본 SPEC §7 Out of Scope 명시).

## 3. Chain Order Decision — Metrics 최외곽 vs auth 이후

| 위치 | 설명 | 결과 | 판정 |
|------|------|------|------|
| **Metrics 최외곽** (auth 이전) | `InstrumentHTTP(BuildRESTChain(...))`, gRPC `ChainUnaryInterceptor(metrics, authChain)` | 인증 실패(401)/인가 실패(403) 요청도 계측 → 에러율/공격 트래픽 가시성 확보. SLA의 "전체 요청 대비 실패율" 정확 | **채택** |
| Metrics auth 이후 | `BuildRESTChain(InstrumentHTTP(...))` | 인증 통과 요청만 계측 → 401 폭주(brute force)·인가 거부가 메트릭에서 불가시 → 보안 인시던트 관측 불가, SLA 분모 왜곡 | 기각 |

**채택: metrics 최외곽.** REQ-OBS-003-E1/E2로 코드 강제 + 단위 테스트로 회귀 가드. `/metrics`/`/health`/`/ready`는 instrumentation 제외(REQ-OBS-003-S1)하여 self-scrape recursion + probe self-traffic의 SLA 오염 방지.

## 4. 참고 라이브러리 (Context7/공식 문서 기반 best practice)

- **`github.com/prometheus/client_golang`**: `prometheus.NewRegistry()` + `promhttp.HandlerFor(reg, promhttp.HandlerOpts{})`. `promauto`/`DefaultRegisterer` 전역 회피 → 명시적 registry(테스트 격리, REQ-OBS-001-U1). Histogram은 `prometheus.NewHistogramVec` + 명시적 bucket(latency: default 또는 `prometheus.DefBuckets`).
- **`github.com/grpc-ecosystem/go-grpc-prometheus` (참고만)**: 표준 gRPC 메트릭 패턴 레퍼런스. 단, 신규 의존성 추가 최소화를 위해 본 SPEC은 경량 자체 `UnaryMetricsInterceptor`(duration + `status.Code`)로 구현 — go-grpc-prometheus 직접 의존은 도입하지 않음(라벨 cardinality 직접 통제 + 의존성 슬림 — go.md MUST NOT 전역 상태). 결정: 자체 인터셉터.
- **`go.opentelemetry.io/otel` SDK (v1.43.0 family, 이미 indirect)**: `sdktrace.NewTracerProvider` + `sdktrace.NewBatchSpanProcessor`. default는 exporter 없는 noop-equivalent(span drop) — 망분리 정합. `otel/propagation.TraceContext{}` W3C propagator. `contrib/instrumentation/net/http/otelhttp v0.60.0`(indirect 존재)는 HTTP propagation 헬퍼로 활용 가능. OTLP는 `otlptracegrpc.New` (opt-in).

## 5. lesson #11 적용 — /metrics polling-safe

K8s/Prometheus가 `/metrics`를 ~10s 간격으로 scrape한다. SERVER-001 lesson #11(probe deps)과 동일 원칙: `/metrics` 핸들러는 registry `Gather()`(in-memory snapshot)만 수행하고, credential 로딩·DB 조회·외부 호출을 하지 않는다. RBAC wrapper도 `UserFromContext`(in-context) + `ParseRolesFromScope`(string ops)만 — polling-safe. PgPool gauge는 scrape 시점이 아닌 별도 15s tick goroutine에서 갱신하므로 scrape가 DB를 두드리지 않는다.

## 6. lesson #12 적용 — shutdown race

PgPool 갱신 goroutine + OTel tracer flush가 SERVER-001 `Server.shutdown`(sync.Once)과 race하지 않도록: goroutine은 SERVER-001 root context 파생 ctx를 구독(`<-ctx.Done()` 종료), tracer shutdown은 SERVER-001 graceful shutdown 시퀀스(redis→pg cleanup 사이 또는 직후)에 명시적 추가. `go.uber.org/goleak`(go.mod 존재)로 누수 0 단언(AC-OBS-EDGE-2).

## 7. Risk Register (요약 — 상세 plan.md §3)

| 위험 | 완화 |
|------|------|
| AUTH-001 rbac.go frozen 침범 유혹 | Option B 채택, §2.1/§6/§7 3중 명시, plan-auditor 결함 시 본 §2 인용 거부 |
| Metric cardinality explosion | path 정규화 라우트만, 정적 라벨 집합, AC-OBS-EDGE-1 상한 단언 |
| Chain order 오류 | metrics 최외곽 코드 강제 + 단위 테스트 (§3) |
| `/metrics` self-scrape recursion | REQ-OBS-003-S1 probe/scrape 경로 제외 |
| OTel indirect→direct 버전 충돌 | S0 `go mod tidy` + `go build ./...` 회귀, otel core v1.43.0 ↔ sdk major 정합 |
| prometheus 신규 의존성 transitive 충돌 | S0 `go mod verify` + 전체 build |
| shutdown goroutine leak | root ctx 파생 + goleak (§6) |
| 계측 overhead > 1ms | atomic-only hot path, `BenchmarkInstrumentHTTP` 게이트 |
| `auth → metrics → auth` 순환 import | Dependency Inversion — `RejectionObserver` auth 내 정의, metrics 구현, server.go DI (§9.2). `go list -deps` 단언 (AC-OBS-001-4) |
| workflow observer 과적용 | 순환 부재 검증 후 직접 import 채택 (§9.3) — over-engineering 회피 |

## 9. Circular Import Decision — auth_rejections_total 수집 경로 (evaluator iter 3 Moderate)

### 9.1 문제 (코드로 재확인)

`iroum_ax_auth_rejections_total`의 유일한 source는 `auth.TokenValidator.Verify`의 reject 분기다 (검증: `internal/auth/validator.go:197~314` — `ErrTokenInvalidIssuer`:279, `ErrAlgorithmKeyMismatch`:370, `ErrTokenExpired`:248/257/264, `ErrTokenBlacklisted`:291). 이 counter를 증가시키려면 auth가 `metrics.IncAuthRejection(reason)`를 호출해야 한다 → `auth → metrics`.

그런데 `internal/metrics/permission.go`가 `MetricsAuthMiddleware`를 위해 이미 `auth`를 import한다 → `metrics → auth`. 따라서 직접 호출은 **`auth → metrics → auth` compile-time 순환 import**(Go 금지). v0.1.1 §2.1에 `internal/auth/middleware.go`가 affected files에 없던 것은 이 제약의 결과였으나, SPEC이 해결 방안을 명시하지 않아 `auth_rejections_total`이 dead counter(운영 brute-force 미탐지)로 남는 명세-코드 gap이었다.

### 9.2 옵션 평가

| 옵션 | 설명 | 단점 | 판정 |
|------|------|------|------|
| **A. auth가 metrics 직접 import** | `Verify`에서 `metrics.IncAuthRejection` 직접 호출 | compile-time 순환 import (Go 빌드 불가) | **기각 (불가능)** |
| **B. Dependency Inversion** | `RejectionObserver` interface를 **auth 패키지 내** 정의, metrics가 구현, server.go가 DI 주입 | RBAC 진실 분산 없음; auth↔metrics 단방향; 추가 wire 1지점 | **채택** |
| **C. 별도 중립 패키지로 counter 이동** | `internal/obscounters` 신설 후 auth/metrics 양쪽이 import | 신규 패키지 + collector 분산 + metrics 응집 붕괴 | 기각 (과한 구조 변경) |
| **D. auth_rejections_total 삭제** | counter 자체를 제거 | 운영 brute-force 탐지 불가 — tech.md §8/§11.2 SLA 요구 위배 | 기각 |

**채택: Option B (Dependency Inversion).** 근거: (1) Go 인터페이스는 구조적 만족이므로 `RejectionObserver`를 auth 패키지에 정의하면 auth는 metrics를 import하지 않고도 hook을 호출할 수 있다. (2) metrics는 그 interface를 구현하되 interface 만족을 위해 auth를 신규 import할 필요가 없다(metrics→auth는 `MetricsAuthMiddleware`로 이미 단방향 존재, 추가 의존 0). (3) DI wire는 `cmd/server/server.go`(`package main`, auth+metrics 모두 import) 단일 지점 — 의존 그래프상 자연스럽다. (4) `WithRejectionObserver`는 기존 `WithIssuer/WithAudience/...`와 동일한 additive ValidatorOption 패턴이라 `auth.New`/`Verify` 시그니처가 불변 → AUTH-001 기존 호출부(RESTMiddleware:182, UnaryServerInterceptor:131, 테스트) 전부 무영향, frozen `rbac.go`도 미접촉. 최종 의존 방향: **auth(interface 정의) ← metrics(구현) ← server.go(wire)**, `auth → metrics` 간선 부재 → no cycle. `go list -deps`로 단언(AC-OBS-001-4).

### 9.3 workflow_state_transitions_total — 직접 import 결정 (over-engineering 회피)

`internal/workflow/state_machine.go`의 import는 `context/fmt/sync/cperrors(internal/errors)/types(internal/types)/zap`만이다(검증됨 — auth/metrics 미import). `metrics`도 `auth`도 `workflow`를 import하지 않으므로 `workflow → metrics` 직접 import는 어떤 순환도 만들지 않는다.

**결정: workflow는 metrics를 직접 import (observer 패턴 미적용).** 근거: 순환이 실재하지 않는데 auth와 "일관성"을 명목으로 동일 DI 패턴을 적용하면 zero-benefit 추상화 — Agent Core Behavior #4 (Enforce Simplicity) + lesson(단순성) 위배. observer/DI는 **실재 순환을 푸는 도구**이지 기본 패턴이 아니다. `StateMachine.Start`(L117 Commit 성공 후)/`Complete`(L156)/`Fail`(L202)에서 `metrics.IncWorkflowTransition(from, to)`를 직접 호출하며 from/to는 bounded `types.WorkflowState`(PENDING/RUNNING/COMPLETED/FAILED)로 cardinality-safe다. (REQ-OBS-001-S3, AC-OBS-001-5.)

## 8. Cross-SPEC artifacts affected (lessons #5)

- `internal/auth/authz_mapping.go` (+1 entry) — AUTH-002 자산이나 lookup 시그니처 불변, AUTH-002 SPEC 파일 미수정.
- `internal/auth/authz_mapping_test.go` (+1 test) — AUTH-002 테스트 파일에 1건 추가.
- `cmd/server/server.go` (wiring) — SERVER-001 자산, 부팅 시퀀스 구조 불변(hook 줄 추가만).
- `internal/config/config.go` (+3 fields, S0) — 기존 필드/시그니처 불변.
- `internal/scheduler/dispatcher.go` (+계측 hook) — CTRL-001 자산, 비즈니스 로직 불변.
- upstream SPEC 파일(AUTH-002 §13 / SERVER-001 §4·§5): 본 SPEC GREEN 후 별도 chore commit으로 `RESOLVED by SPEC-AX-OBS-001 v0.1.0` 주석 추가 가능(범위 외, fact만 보장).
