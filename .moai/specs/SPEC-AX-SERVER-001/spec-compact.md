# SPEC-AX-SERVER-001 — Compact Summary

> Compact reference for agent context. Full spec: `spec.md`.

---

## Identity

- **ID**: SPEC-AX-SERVER-001
- **Version**: 0.1.0
- **Status**: draft
- **Priority**: high
- **Composite Domain**: AX + SERVER (서버 부팅/인프라 sub-domain)
- **Methodology**: TDD (권장) — 모든 deliverable이 신규 + 외부 동작 zero
- **Harness**: thorough

## Mission

4개 선행 SPEC(AX-001 + CTRL-001 + AUTH-001 + AUTH-002 v0.1.2)이 정의한 components를 조립·부팅·종료하는 calling code를 작성한다. 비즈니스 로직 0줄.

## Scope

- 신규 3개 파일: `cmd/server/main.go`, `cmd/server/probes.go`, `cmd/server/server_test.go`
- 전면 재작성 1개 파일: `cmd/server/server.go` (40-line stub → ~250 LOC)
- 사전 의존 (S0 deliverable): `audit/actions.go` 3 const + `config/config.go` 2 field + `go.mod` errgroup require

## EARS Requirements (5 modules)

1. **REQ-SERVER-UBI-001** (Ubiquitous, 3 sub-clauses):
   - (a) 모든 startup/shutdown audit (`SERVER_STARTUP` / `SERVER_SHUTDOWN_INITIATED` / `SERVER_SHUTDOWN_COMPLETED`)
   - (b) Dependency 순서 강제 (config → store → scheduler → auth → handler → server, 11 단계)
   - (c) Graceful guarantee (in-flight 요청 30s 대기, AuthEnabled 무관)

2. **REQ-SERVER-001** (Dual Listener):
   - S1 gRPC :50051 + REST :8080 동시 listen
   - E1/E2 listener bind via errgroup
   - U1 port conflict graceful error
   - U2 one listener dies → 전체 종료

3. **REQ-SERVER-002** (Dependency Wiring):
   - E1 sequential init with error wrapping (`fmt.Errorf("init step %s failed: %w", ...)`)
   - E2 partial cleanup on failure (reverse order)
   - E3 auth chain composition (BuildRESTChain + BuildGRPCInterceptorChain 호출)
   - U1 DB Ping 실패 시 startup abort
   - U2 JWKS fetch 실패 시 abort (AuthEnabled=true만)

4. **REQ-SERVER-003** (Graceful Shutdown):
   - E1 SIGTERM/SIGINT 처리 + 5단계 shutdown 순서
   - E2 in-flight HTTP 요청 완료 대기 (`ShutdownTimeoutSeconds`, default 30s)
   - S1 shutdown idempotency (`sync.Once`)
   - S1-DoubleSignal 두 번째 signal force-kill
   - U1 timeout 초과 force-kill (`details.exit_reason=force_kill_timeout`)

5. **REQ-SERVER-004** (Health & Readiness):
   - E1 `/health` 항상 200 (liveness)
   - E2 `/ready` DB+Redis+JWKS ping → 200/503
   - E3 gRPC `/grpc.health.v1.Health/Check` 동일 로직
   - S1 shutdown 중 `/ready` → 503 (`shutting_down`)
   - U1 probe timeout → check 별 `failed: timeout`

## Exclusions (10)

1. 다중 인스턴스 부하 분산 (`replicaCount >= 2`)
2. Hot reload / dynamic config
3. TLS termination (Ingress 책임)
4. Distributed tracing (OpenTelemetry → SPEC-AX-OBS-001)
5. Prometheus `/metrics` (AUTH-002 #13 + 본 SPEC #5)
6. 다중 OIDC provider
7. API rate limiting (Ingress + 후속 SPEC)
8. WebSocket / SSE
9. Server-side caching
10. Custom error pages (HTML)

## NFR Highlights

- Startup p95 < 5s (testcontainers)
- Shutdown p95 < 30s (default timeout)
- `/health` p99 < 5ms
- `/ready` p95 < 500ms
- Coverage ≥ 85%
- `gosec` G112 ReadHeaderTimeout 명시
- 망분리 정합 (외부 API 0건)

## Acceptance Criteria (14)

- AC-SERVER-UBI-001-a/b/c (audit + order + graceful)
- AC-SERVER-001-E1/E2/U1/U2 (listener)
- AC-SERVER-002-E1/E2/E3/U1/U2 (wiring)
- AC-SERVER-003-E1/S1/S1-DoubleSignal/U1 (shutdown)
- AC-SERVER-004-E1/E2/E2-DBDown/E3/S1/U1 (probes)
- AC-SERVER-Edge-PortZero / AuthDisabledFullPath / FatalErrorDuringRun

## Dependencies

- SPEC-AX-001 GREEN (`audit_logs`, `cli-anonymous`)
- SPEC-AX-CTRL-001 부분 GREEN (`WorkflowService`, `RESTHandler`, `TxCoordinator`); Sprint 7 흡수
- SPEC-AX-AUTH-001 GREEN (`rbac.go`, `TokenValidator`, `RefreshTokenStore`, OIDC)
- SPEC-AX-AUTH-002 v0.1.2 GREEN (`chain.go.BuildRESTChain` / `BuildGRPCInterceptorChain`)
- Go 의존성 신규: `golang.org/x/sync/errgroup` + `go.uber.org/goleak` (테스트)

## Cross-SPEC Resolution

- **AUTH-002 §5 Exclusion #12** "cmd/server/server.go 부트스트랩" → 본 SPEC GREEN으로 historical
- **CTRL-001 Sprint 7 T-AX-006** 미완성 gap → 본 SPEC 흡수

## Sprint DAG (3 sprints)

```
S0 (Pre-req chores: audit actions + config fields + go.mod)
  ▼
S1 (Core bootstrap + dual listener: server.go 전면 재작성 + main.go + 6 unit tests)
  ▼
S2 (Graceful shutdown: sync.Once + signal handling + 4 unit tests)
  ▼
S3 (Health/Readiness + testcontainers E2E + benchmark: probes.go + 4 E2E + 2 benchmark)
```

## Top 3 Architectural Decisions

1. **errgroup + signal.NotifyContext for dual listener**: 표준 Go 패턴, `grpc-go/examples/features/multiplex` + charmbracelet/crush 검증. 한쪽 listener 실패 시 자동 cancel 전파.
2. **`/health` + `/ready` chain 외부 mount**: AUTH-002 chain 진입 자체 회피 → cross-SPEC 매핑 변경 불필요. `http.ServeMux` longest-prefix match로 `/health` + `/ready` 우선 분리.
3. **`sync.Once` shutdown guard + double-signal force-kill**: 첫 signal는 graceful, 두 번째 signal는 즉시 force-kill 의도로 해석. errgroup error + signal 동시 발생 race를 sync.Once로 해소.

## Schema Note

8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`) 준수. `labels` / `created_at` 등 변형 필드는 plan-auditor 결함 시 본 문서 + spec.md "Schema note" + plan.md §7 + acceptance.md §8을 출처로 거부.

## Lessons Applied

- #1 Schema note 포함 (이 §Schema Note)
- #2 UBI sub-clauses 각 dedicated AC (AC-SERVER-UBI-001-a/b/c)
- #4 stub-assert anti-pattern 회피 — testcontainers full-stack로 실제 동작 검증
- #5 Cross-SPEC: AUTH-002 §12 + CTRL-001 Sprint 7 unblock 명시
- #6 ~600-800K tokens 예상 (3 sprints, testcontainers E2E 무거움)
- #8 한국 공공 도메인 6 제약: 망분리 정합 / PIPA audit / 합니다체 미해당 / HWP 무관
