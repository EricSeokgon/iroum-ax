# SPEC-AX-OBS-001 (compact)

id=SPEC-AX-OBS-001 · v0.1.0 · draft · 2026-05-15 · ircp · priority=medium · issue=0
6번째 SPEC · SERVER-001 v0.1.2 GREEN 후속 · thorough harness · TDD

## 목적
운영 관측성 계층 도입 (Prometheus metrics + OTel tracing skeleton). Cross-SPEC unblock:
AUTH-002 §5 Excl #13 (`/metrics` 권한+handler) + SERVER-001 §5 Excl #4(OTel)·#5(metrics) 정식 해소.
tech.md §8 모니터링 스택 코드 미구현 gap 해소.

## REQ 요약 (5 + UBI)
- **REQ-OBS-UBI-001** (Ubiquitous, 4 sub): a=망분리 pull only/push gateway 금지, b=계측 overhead <1ms p99, c=read:metrics RBAC 필수, d=PII 미노출/cardinality bound
- **REQ-OBS-001** (Metrics Registry): client_golang 싱글톤 registry + 7 core collector (http/grpc duration histogram, workflow transition/celery dispatch/auth rejection/authz forbidden counter, pg_pool gauge). U1=default registry 미사용
- **REQ-OBS-002** (/metrics + RBAC): authz_mapping `/metrics→read:metrics` 1행, RoleAdmin only, promhttp wrapping. U1=401, U2=403
- **REQ-OBS-003** (Instrumentation): REST middleware + gRPC interceptor, **chain 최외곽**(인증실패도 계측), probe 제외, panic re-raise
- **REQ-OBS-004** (OTel skeleton): otel SDK, noop default(망분리), OTLP opt-in, init 실패 non-fatal, request→workflow→celery span

EARS 5 패턴 전부: Ubiquitous(UBI), Event(001-E1/002-E1/003-E1/004-E1), State(002-S1/003-S1/004-S1), Optional(004-O1), Unwanted(001-U1/002-U1·U2/003-U1/004-U1).

## read:metrics 권한 결정 (research.md decision matrix)
**Option B 채택**: OBS 자체 metrics permission registry (`internal/metrics/permission.go`, `IsMetricsAuthorized` RoleAdmin only). AUTH-001 `rbac.go` permissionMatrix **frozen 보존**(미수정). Option A(rbac amendment)=frozen 위반·AUTH-002 §13 모순 재발로 기각. Option C(config flag)=RBAC 모델 붕괴로 기각.

## Chain Order 결정
Metrics **최외곽**(auth 이전): `InstrumentHTTP(BuildRESTChain(...))`, `ChainUnaryInterceptor(metrics, authChain)`. 인증/인가 실패도 계측(SLA 실패율·brute force 가시성). `/metrics`·`/health`·`/ready` 계측 제외(self-scrape recursion 방지).

## 핵심 검증된 API (lesson #9, spec.md §2.0 단일 진실)
`auth.LookupRESTPermission`(authz_mapping.go:81), `auth.permissionMatrix` private/frozen(rbac.go:39), `auth.RoleAdmin`/`ParseRolesFromScope`(rbac.go), `store.PgWorkflowStore.PoolStats() *pgxpool.Stat`(pg_store.go:92), `scheduler.CeleryDispatcher.Dispatch`(dispatcher.go:70), `cmd/server` package main `Server.Run()` outerMux(server.go:171~269), config.Config OTel 필드 부재(S0 추가), go.mod prometheus 부재/otel v1.43.0 indirect(S0 승격).

## Sprint DAG
S0 deps+registry+permission(High) → S1 /metrics+RBAC+authz_mapping(High) → S2 HTTP/gRPC instrumentation(High) → S3 OTel skeleton+E2E(Medium). 순차, S0+S1 결합 가능.

## Affected files
신규6: metrics/{registry,collectors,permission,http_middleware,grpc_interceptor,tracing}.go + tests.
수정3: authz_mapping.go(+1행), cmd/server/server.go(wiring), config.go(+3 field S0), dispatcher.go(+hook). rbac.go **미수정(frozen)**.

## Exclusions (10, named follow-up — lesson #10)
1.Grafana 대시보드(SPEC-AX-DASH-001) 2.Loki(SPEC-AX-LOG-001) 3.Alertmanager(운영) 4.Jaeger/Tempo backend(운영) 5.SLO/SLI(SPEC-AX-SLO-001) 6.Push gateway(망분리 영구금지) 7.Thanos/Cortex(운영) 8.Exemplars(SPEC-AX-OBS-002) 9.pprof(SPEC-AX-PPROF-001) 10.RED/USE 자동 대시보드(후속).

## AC (18, lesson #2 — UBI 4 sub 각 dedicated)
UBI-001-a/b/c/d, 001-1/2/3, 002-1/2/3/4/5, 003-1/2/3/4, 004-1/2/3, EDGE-1/2/3.
Edge: /metrics no-auth→401, viewer→403, cardinality bound, OTel noop default, overhead<1ms p99, goleak shutdown(lesson #12), SERVER-001/AUTH-002 regression.

## Cross-SPEC unblock (lesson #5+#10)
AUTH-002 §13 → REQ-OBS-002 RESOLVED. SERVER-001 §4 → REQ-OBS-004, §5 → REQ-OBS-001/002/003.

## Lessons 적용
#1 Schema note · #2 UBI sub dedicated AC · #4 stub-assert 회피(TDD RED 자연 fail) · #5 cross-SPEC unblock 명시 · #9 실제 API 사전 Read/Grep(9 file) · #10 named follow-up Exclusion · #11 /metrics polling-safe · #12 shutdown goroutine race(goleak).

## DoD
18 AC GREEN · ≥85% cov · `go test -race ./...` 전체(regression) · overhead<1ms · push gateway grep 0 · rbac.go git diff 빈 결과 · MX 4종(ko).
