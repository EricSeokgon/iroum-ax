# SPEC-AX-OBS-001 (compact)

id=SPEC-AX-OBS-001 · v0.1.2 · draft · 2026-05-15 · ircp · priority=medium · issue=0
6번째 SPEC · SERVER-001 v0.1.2 GREEN 후속 · thorough harness · TDD
v0.1.2: evaluator-active iter3 CONFIRM(0.79, plan-auditor PASS 0.97) — circular import(Moderate) + workflow wiring(Minor) 해소.
v0.1.1: plan-auditor iter1 FAIL(0.78, 1 Major+2 Minor) — D1(MetricsAuthMiddleware) + D2(gRPC ServerOption 정정) + D3(AC count 정합).

## 목적
운영 관측성 계층 도입 (Prometheus metrics + OTel tracing skeleton). Cross-SPEC unblock:
AUTH-002 §5 Excl #13 (`/metrics` 권한+handler) + SERVER-001 §5 Excl #4(OTel)·#5(metrics) 정식 해소.
tech.md §8 모니터링 스택 코드 미구현 gap 해소.

## REQ 요약 (5 + UBI)
- **REQ-OBS-UBI-001** (Ubiquitous, 4 sub): a=망분리 pull only/push gateway 금지, b=계측 overhead <1ms p99, c=read:metrics RBAC 필수, d=PII 미노출/cardinality bound
- **REQ-OBS-001** (Metrics Registry): client_golang 싱글톤 registry + 7 core collector (http/grpc duration histogram, workflow transition/celery dispatch/auth rejection/authz forbidden counter, pg_pool gauge). U1=default registry 미사용. **S2=auth_rejections_total은 RejectionObserver DI로 수집(circular import 회피)**. **S3=workflow_state_transitions_total은 직접 import hook(순환 없음, observer 미적용)**
- **REQ-OBS-002** (/metrics + RBAC via MetricsAuthMiddleware): authz_mapping `/metrics→read:metrics` 1행(test 검증용), `MetricsAuthMiddleware` 전용 미들웨어가 authn+authz 자체 수행. E1=200 exposition, E2=authz_mapping, E3=authn(Verify+WithUser), S1=admin-only authz, U1=401(authn 실패), U2=403(authz 실패)
- **REQ-OBS-003** (Instrumentation): REST middleware + gRPC interceptor, **chain 최외곽**(인증실패도 계측), probe 제외, panic re-raise. E2=metrics ServerOption을 auth ServerOption보다 앞 인자로 grpc.NewServer 전달
- **REQ-OBS-004** (OTel skeleton): otel SDK, noop default(망분리), OTLP opt-in, init 실패 non-fatal, request→workflow→celery span

EARS 5 패턴 전부: Ubiquitous(UBI/001-S1·S2·S3), Event(001-E1/002-E1·E2·E3/003-E1·E2/004-E1), State(002-S1/003-S1/004-S1), Optional(004-O1), Unwanted(001-U1/002-U1·U2/003-U1/004-U1).

## v0.1.2 — Circular import 해소 (Dependency Inversion, REQ-OBS-001-S2)
`auth_rejections_total` source = `auth.TokenValidator.Verify` reject 분기(validator.go:248~291: invalid_issuer/alg_mismatch/expired/blacklist). auth→metrics 직접 호출 필요하나 metrics→auth 이미 존재(MetricsAuthMiddleware) → `auth→metrics→auth` compile 순환. **FIX**: `internal/auth/observer.go`(신규) `RejectionObserver interface{IncAuthRejection(reason string)}`를 **auth 패키지 내** 정의(auth는 metrics import 0). `validator.go`(수정, additive) `WithRejectionObserver` ValidatorOption + optional `rejectionObs` 필드(nil-safe) + Verify reject 분기 `recordRejection(reason)` hook. `New`/`Verify` 시그니처 불변(AUTH-001 backward-compat, rbac.go 미접촉). metrics가 interface 구조적 만족 구현(auth 신규 import 0). server.go(package main, auth+metrics import)가 `auth.WithRejectionObserver(obs)` DI wire. **의존 방향: auth(interface) ← metrics(구현) ← server.go(wire). auth→metrics 간선 부재 → no cycle.** `go list -deps` 단언(AC-OBS-001-4).

## v0.1.2 — workflow_state_transitions wiring (REQ-OBS-001-S3, 직접 import)
`workflow/state_machine.go` import = context/fmt/sync/cperrors/types/zap만(auth/metrics 미import 검증). metrics/auth→workflow 간선 부재 → `workflow→metrics` 직접 import 순환 없음. **결정: 직접 import, observer 미적용** — 순환 없는데 일관성 명목 DI는 over-engineering(Enforce Simplicity). `Start`(L117 Commit 후)/`Complete`(L156)/`Fail`(L202)에 `metrics.IncWorkflowTransition(from,to)` 1줄(commit된 전이만, from/to bounded WorkflowState → cardinality-safe). AC-OBS-001-5.

## D1 해소 — MetricsAuthMiddleware (Option A, v0.1.1)
`/metrics`는 `auth.BuildRESTChain` 외부 mount(REQ-OBS-003 최외곽 정합) → `auth.RESTMiddleware`(WithUser populate 유일 지점, chain.go:71) 우회 → `internal/metrics/permission.go`에 `MetricsAuthMiddleware(validator *auth.TokenValidator, authEnabled bool) func(http.Handler) http.Handler` 신설. authn(Bearer→`Verify`→`WithUser`) + authz(`UserFromContext`→`ParseRolesFromScope`→`IsMetricsAuthorized`). RESTMiddleware/RESTAuthzMiddleware와 독립. Option B(BuildRESTChain 내부+rbac.go read:metrics)=frozen 위반·AUTH-002 §13 모순 재발로 기각.

## D2 해소 — gRPC ServerOption 정정 (v0.1.1)
`auth.BuildGRPCInterceptorChain`은 `grpc.ServerOption` 반환(chain.go:86-101). `grpc.ChainUnaryInterceptor(metrics.UnaryMetricsInterceptor())` ServerOption을 `auth.BuildGRPCInterceptorChain(...)` ServerOption보다 **앞 인자**로 `grpc.NewServer(...)` 전달(순서대로 chainUnaryInts 누적 → [metrics,authn,authz,handler]).

## read:metrics 권한 결정 (research.md §2)
**Option B 채택**: OBS 자체 metrics permission registry(`permission.go`, `IsMetricsAuthorized` RoleAdmin only). AUTH-001 `rbac.go` permissionMatrix **frozen 보존**. Option A(rbac amendment)·C(config flag) 기각.

## Chain Order 결정
Metrics **최외곽**: REST `InstrumentHTTP(outerMux non-probe)`, gRPC `grpc.NewServer(ChainUnaryInterceptor(metrics), authServerOption)`. 인증/인가 실패도 계측. `/metrics`·`/health`·`/ready` 제외(self-scrape 방지).

## 핵심 검증된 API (lesson #9, spec.md §2.0 단일 진실)
`auth.LookupRESTPermission`(authz_mapping.go:81), `auth.permissionMatrix` private/frozen(rbac.go:39), `auth.RoleAdmin`/`ParseRolesFromScope`(rbac.go), `auth.UserFromContext`(middleware.go:49), `auth.WithUser`(middleware.go:40), `auth.TokenValidator.Verify`+reject 분기(validator.go:197~314: ErrTokenInvalidIssuer:279/ErrAlgorithmKeyMismatch:370/ErrTokenExpired:248,257,264/ErrTokenBlacklisted:291), `auth.New`+ValidatorOption additive 패턴(validator.go:115~176), auth 패키지 metrics 미import(validator.go·middleware.go), `auth.RESTMiddleware`(chain.go:71 재사용 안 함), `auth.BuildGRPCInterceptorChain → grpc.ServerOption`(chain.go:86-101), `Server.tokenValidator`(server.go:44·135·175·192), `store.PgWorkflowStore.PoolStats()`(pg_store.go:92), `scheduler.CeleryDispatcher.Dispatch`(dispatcher.go:70), `workflow.StateMachine.Start/Complete/Fail`(state_machine.go:82~204, Commit 후 nil, auth/metrics 미import), `cmd/server` package main Server.Run() outerMux(server.go:171~214), config.Config OTel 필드 부재(S0), go.mod prometheus 부재/otel v1.43.0 indirect(S0).

## Sprint DAG
S0 deps+registry+permission+MetricsAuthMiddleware+observer.go+validator.go DI(High) → S1 /metrics+RBAC+authz_mapping(High) → S2 HTTP/gRPC instrumentation+workflow hook+observer wire(High) → S3 OTel skeleton+E2E(Medium). 순차, S0+S1 결합 가능.

## Affected files
신규7: metrics/{registry,collectors(+RejectionObserver 구현체),permission(+MetricsAuthMiddleware),http_middleware,grpc_interceptor,tracing}.go + tests, **auth/observer.go(RejectionObserver interface)**.
수정6: authz_mapping.go(+1행), cmd/server/server.go(/metrics mount + gRPC ServerOption 선행 + **RejectionObserver DI wire**), config.go(+3 field S0), dispatcher.go(+hook), **auth/validator.go(+WithRejectionObserver option+Verify hook, additive)**, **workflow/state_machine.go(+IncWorkflowTransition 3지점, 직접 import)**. rbac.go **미수정(frozen)**.

## Exclusions (10, named follow-up — lesson #10)
1.Grafana(SPEC-AX-DASH-001) 2.Loki(SPEC-AX-LOG-001) 3.Alertmanager(운영) 4.Jaeger/Tempo(운영) 5.SLO/SLI(SPEC-AX-SLO-001) 6.Push gateway(망분리 영구금지) 7.Thanos/Cortex(운영) 8.Exemplars(SPEC-AX-OBS-002) 9.pprof(SPEC-AX-PPROF-001) 10.RED/USE 자동 대시보드.

## AC (총 24 = REQ-mapped 21 + EDGE 3, spec.md §8.1 단일 진실 — lesson #2)
REQ-mapped 21: UBI-001-a/b/c/d(4) · 001-1/2/3/4/5(5) · 002-1/2/3/4/5(5) · 003-1/2/3/4(4) · 004-1/2/3(3).
EDGE 3: EDGE-1(cardinality)/EDGE-2(goleak shutdown, lesson #12)/EDGE-3(SERVER-001·AUTH-002 regression).
v0.1.2 추가: AC-OBS-001-4(auth_rejections_total RejectionObserver DI로 실제 increment + auth가 metrics 미import import-graph 단언) + AC-OBS-001-5(workflow_state_transitions_total 직접 import hook으로 실제 increment, commit된 전이만). D1로 AC-OBS-002-1/3/4 authn/authz 분리(v0.1.1).

## Cross-SPEC unblock (lesson #5+#10)
AUTH-002 §13 → REQ-OBS-002 RESOLVED. SERVER-001 §4 → REQ-OBS-004, §5 → REQ-OBS-001/002/003. upstream 파일 미수정(별도 chore `RESOLVED by SPEC-AX-OBS-001 v0.1.2`).

## Lessons 적용
#1 Schema note · #2 UBI sub dedicated AC · #4 stub-assert 회피 · #5 cross-SPEC unblock · #9 실제 API 사전 Read/Grep(validator.go·middleware.go·state_machine.go 추가 검증) · #10 named follow-up Exclusion · #11 /metrics polling-safe · #12 shutdown goroutine race(goleak) · Enforce Simplicity(workflow observer 미적용).

## DoD
총 24 AC GREEN(REQ-mapped 21 + EDGE 3) · ≥85% cov · `go test -race ./...` 전체 · `go list -deps`로 auth가 metrics 미import 단언(순환 부재) · auth_rejections/workflow_transitions 실제 increment · overhead<1ms · push gateway grep 0 · rbac.go git diff 빈 결과 · MX 6종(registry ANCHOR / MetricsAuthMiddleware ANCHOR+REASON / **RejectionObserver ANCHOR+REASON** / InstrumentHTTP NOTE / goroutine WARN+REASON / wiring NOTE, ko).
