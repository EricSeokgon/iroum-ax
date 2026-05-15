# SPEC-AX-SERVER-001 — Research

> SPEC: `SPEC-AX-SERVER-001 v0.1.0`
> Purpose: Server bootstrap pattern selection + dual listener strategy + graceful shutdown comparison

---

## 1. Codebase Analysis

### 1.1 현재 상태 (`cmd/server/server.go:1~40`)

40-line stub. `Server` struct는 `logger *zap.Logger` + `cfg Config` (cfg는 자체 정의 — `LogLevel`, `GRPCPort`, `RESTPort`만 보유, `config.Config`와 별개). `Run()`은 `logger.Info("server stub — not yet implemented", ...)` 후 nil 반환. main 함수 부재 (`cmd/server/main.go` 미존재). 호출자 없음 — 어떤 test도 `Server.Run()`을 호출하지 않음 → 안전하게 전면 재작성 가능.

### 1.2 호환 components (4개 SPEC GREEN 산출물)

| Module | Path | API | 본 SPEC 사용 단계 |
|--------|------|-----|------------------|
| `config.Config` + `Load()` | `internal/config/config.go` | 12 fields incl `AuthEnabled`, `GRPCAddr`, `RESTAddr`, `OIDCIssuerURL`, `JWKSCacheTTLSeconds` | step (a) |
| `store.PgWorkflowStore` | `internal/store/pg_store.go` (CTRL-001 Sprint 3) | `NewPgWorkflowStore(dsn) (*PgWorkflowStore, error)`, `Ping(ctx) error`, `Close() error` | step (c) |
| Redis client | `github.com/redis/go-redis/v9` (CTRL-001 Sprint 6 dispatcher) | `redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})`, `.Ping(ctx)`, `.Close()` | step (d) |
| `auth.OIDCClient` | `internal/auth/oidc_client.go` (AUTH-001) | `NewOIDCClient(issuerURL, audience, cacheTTL)`, JWKS 초기 fetch | step (e) |
| `auth.TokenValidator` | `internal/auth/token_validator.go` (AUTH-001) | `NewTokenValidator(oidcClient, clockSkew)` | step (f) |
| `auth.RefreshTokenStore` | `internal/auth/refresh_token_store.go` (AUTH-001) | `NewRefreshTokenStore(redisClient)` | step (f) |
| `workflow.StateMachine` | `internal/workflow/state_machine.go` (CTRL-001 Sprint 2) | `NewStateMachine(store, dispatcher, logger)`, `Coordinator() *TxCoordinator` | step (g) |
| `scheduler.CeleryDispatcher` | `internal/scheduler/dispatcher.go` (CTRL-001 Sprint 6) | `NewCeleryDispatcher(redisClient, logger)` | step (h) |
| `server.WorkflowService` | `internal/server/grpc_server.go` (CTRL-001 Sprint 4) | `NewWorkflowService(store, sm, logger)` | step (i) |
| `server.RESTHandler` | `internal/server/rest_handler.go` (CTRL-001 Sprint 5) | `NewRESTHandler(svc, logger)`, `Mux() http.Handler` | step (i) |
| `auth.BuildRESTChain` | `internal/auth/chain.go` (AUTH-002 Sprint 0) | `BuildRESTChain(handler, validator, recorder, authEnabled) http.Handler` | step (j) |
| `auth.BuildGRPCInterceptorChain` | `internal/auth/chain.go` (AUTH-002 Sprint 0) | `BuildGRPCInterceptorChain(validator, recorder, authEnabled) grpc.ServerOption` | step (j) |
| `audit.Recorder` | `internal/audit/recorder.go` (AX-001) | `recorder.Record(ctx, AuditEntry{Action, UserID, Details})` | UBI-001-a |

### 1.3 신규 추가 필요 (S0 deliverable)

- `internal/audit/actions.go`: 3 const (`ActionServerStartup`, `ActionServerShutdownInitiated`, `ActionServerShutdownCompleted`)
- `internal/config/config.go`: 2 fields (`ShutdownTimeoutSeconds`, `ReadyProbeTimeoutSeconds`)
- `go.mod`: `golang.org/x/sync v0.11.0` require

---

## 2. Pattern Comparison: Dual Listener Strategies

### Option A — `errgroup.WithContext` (선택)

```
ctx, cancel := signal.NotifyContext(parent, SIGTERM, SIGINT)
defer cancel()
g, gCtx := errgroup.WithContext(ctx)
g.Go(func() error { return grpcServer.Serve(grpcListener) })
g.Go(func() error { return httpServer.ListenAndServe() })
g.Go(func() error {
    <-gCtx.Done()
    return s.shutdown(gCtx)
})
return g.Wait()
```

**Pros**:
- 표준 패턴 (Google CodeLab + `crush` + `goctl` 다수 채택)
- 한쪽 listener fatal error 시 자동으로 다른 쪽 cancel 전파
- `g.Wait()`가 모든 goroutine 완료 보장

**Cons**:
- gRPC `Serve()` 정상 종료(`GracefulStop()` 호출 후)는 nil 반환이지만 `ErrServerStopped` 반환하는 경우 있음 → wrap 필요
- HTTP `ListenAndServe()` 정상 종료(`Shutdown()` 호출 후)는 `http.ErrServerClosed` 반환 → wrap 필요

### Option B — Manual goroutine + sync.WaitGroup + done channel

**Pros**: 명시적 제어
**Cons**: error propagation 수동, race 위험 ↑, boilerplate ↑

### Option C — gRPC `WithGRPCWeb` + gRPC-only (REST 제거)

**Pros**: 단일 listener, 코드 단순
**Cons**: REST API 별도 명세(CTRL-001 §3 REQ-CTRL-003)와 충돌. 기각.

### Option D — `oklog/run.Group` (선택지에서 제외)

**Pros**: errgroup과 유사하나 더 풍부한 interrupt 처리
**Cons**: 추가 의존성. errgroup으로 충분.

**선택: Option A (errgroup + signal.NotifyContext + shutdown goroutine)**. 표준 + 검증된 패턴 + 추가 의존성 1개(`golang.org/x/sync`, 표준-인근).

---

## 3. Reference Implementations

### 3.1 grpc-go server pattern (official)

`grpc-go/examples/features/multiplex/server/main.go`:
- gRPC + REST 동시 listen 예제
- `grpc.NewServer(opts...)`, `grpcServer.Serve(grpcListener)`, `http.ListenAndServe(restAddr, mux)`
- shutdown: `grpcServer.GracefulStop()` + `httpServer.Shutdown(ctx)`

### 3.2 charmbracelet/crush

`internal/lsp/transport/` (powernap 기반) — JSON-RPC 서브프로세스 lifecycle을 errgroup으로 관리. 본 SPEC과 유사한 graceful shutdown 패턴 사용. `signal.NotifyContext` + `g.Wait()` + `defer cancel()`. 검증된 production 코드 (23k+ stars).

### 3.3 Go standard library

- `net/http`: `http.Server.Shutdown(ctx)` — context-aware graceful shutdown. `IdleTimeout` / `ReadHeaderTimeout` 명시 필수 (G112 lint).
- `os/signal`: `signal.NotifyContext(parent, sig...)` — Go 1.16+ idiomatic. 이전 `signal.Notify(ch, ...)` + manual goroutine은 deprecated 권장 패턴.
- `golang.org/x/sync/errgroup`: 표준-인근. `g.Wait()` blocks until all goroutines return.

### 3.4 grpc-go health protocol

`google.golang.org/grpc/health` package:
- `health.NewServer()` 만들면 `Check(ctx, req)` 자동 구현 (단순 status setting).
- 본 SPEC은 dependency check 로직이 readiness probe와 동일해야 하므로 custom `grpcHealthServer{srv: s}`로 직접 구현.

---

## 4. Decision: Architecture Choices

### Decision 1 — Dual listener with errgroup + signal.NotifyContext

선택. §2 Option A 사유.

### Decision 2 — `/health` + `/ready` mounted outside auth chain

`/health` / `/ready`는 AUTH-002 §3.2 매핑 테이블에서 bypass가 아닌 "미정의" 상태 (현재). default-deny가 발동하면 503 반환 — readiness probe 자체가 default-deny에 막혀 503이 나오는 동시 의도된 503과 구별 불가. 해결책 2가지:
- (a) AUTH-002 매핑 테이블에 `GET /health: bypass` + `GET /ready: bypass` 행 추가 (cross-SPEC 변경, AUTH-002 chore PR 필요)
- (b) `/health` + `/ready`를 chain 외부에 mount → chain 진입 자체 없음

선택: (b). 이유: cross-SPEC 변경 회피 + chain 외부 mount는 표준 패턴 (K8s probe는 인증 없이 호출되어야 정상). 구현: `restMux := http.NewServeMux(); restMux.HandleFunc("GET /health", ...); restMux.HandleFunc("GET /ready", ...); restMux.Handle("/", chain.BuildRESTChain(restHandler.Mux(), ...))`. `restMux`가 외부 wrapper, chain wrap된 핸들러는 `/` prefix로 catch-all.

### Decision 3 — `sync.Once` for shutdown idempotency

선택. signal handler가 중복 호출되거나, errgroup error + signal이 동시 발생할 수 있음. `sync.Once` guard로 첫 호출만 실제 shutdown 수행, 후속 호출은 no-op. 단, "두 번째 signal는 force-kill 트리거"라는 별도 path는 `sync.Once`와 별개로 `secondSignalCh chan struct{}` + `select` 분기.

### Decision 4 — Reverse cleanup on init failure

선택. init step (c)~(j) 중 어느 하나 실패 시 이미 초기화된 리소스를 역순 close. 표준 Go pattern (`defer` 누적 또는 명시적 slice). 본 SPEC은 명시적 slice (`initialized []func()`)로 구현 — defer는 함수 return 시점 일괄 호출이라 init step 사이 실패에 부적합.

### Decision 5 — `http.Server` 모든 timeout 명시

선택. `ReadHeaderTimeout=10s` (G112 Slowloris 방어 — gosec lint 필수), `ReadTimeout=30s`, `WriteTimeout=30s`, `IdleTimeout=60s`. CTRL-001 / AUTH-002의 기존 `rest_handler_test.go`는 `httptest.NewServer`를 사용하므로 본 SPEC의 timeout 정책에 영향 없음. 단, in-flight 5s sleep AC(`AC-SERVER-UBI-001-c`)는 `WriteTimeout=30s`을 넘지 않으므로 안전.

### Decision 6 — testcontainers vs minimal mock

E2E S3는 testcontainers (postgres + redis + static JWKS HTTP server). 단위 테스트 S1/S2는 hand-rolled mock (interface 기반). 이유: testcontainers는 startup ~30s — 단위 테스트마다 부팅 시 CI 30분+ 폭증. E2E 4 케이스만 testcontainer 부담.

---

## 5. Risk Register

| 위험 | 가능성 | 영향 | 완화 |
|------|--------|------|------|
| `signal.NotifyContext` Go 1.16+ 필요 | 낮 | - | `go.mod` `go 1.23` 명시 (이미) |
| `golang.org/x/sync/errgroup` 의존성 추가 | 낮 | 낮 | `go mod tidy` |
| gRPC `Serve()` ↔ `GracefulStop()` race | 중 | 중 | `errgroup.WithContext`로 cancel propagation + `time.AfterFunc` force-stop fallback |
| `http.Server.Shutdown(ctx)` deadline 초과 후 in-flight | 중 | 중 | REQ-SERVER-003-U1 `httpServer.Close()` force-kill + audit `force_kill_timeout` |
| Random port (`:0`) 테스트 flaky | 중 | 낮 | `listener.Addr().String()` 후속 참조 + retry 1회 |
| testcontainers postgres 부팅 60s+ | 중 | 중 | `Reuse: true` + 별도 CI job + `-timeout=300s` |
| Goroutine leak (errgroup 미종료) | 중 | 중 | `go.uber.org/goleak` 모든 test에 `defer goleak.VerifyNone(t)` |
| `signal.NotifyContext` parent ctx not Background → cancel chain 손상 | 낮 | 중 | `signal.NotifyContext(context.Background(), ...)` 명시 |
| `/health` + `/ready`를 AUTH-002 chain 외부 mount → URL routing 충돌 | 낮 | 낮 | `http.ServeMux`의 longest-prefix match로 `/health` / `/ready` 우선, 나머지는 catch-all `/` chain |
| AUTH-002 v0.1.2에서 `/metrics` 분리 → 본 SPEC도 `/metrics` 미해소 | 의도 | - | Exclusion #5 명시. K8s NetworkPolicy 임시 보호 |
| K8s livenessProbe `/health`가 의존성 실패에도 200 → false healthy | 의도된 동작 | - | K8s convention: liveness="process alive", readiness="dependencies ok". 분리 정상 |

---

## 6. External References

- [Google CodeLabs — graceful shutdown](https://cloud.google.com/run/docs/tips/general#graceful_shutdown) — `signal.NotifyContext` + `http.Server.Shutdown` 패턴
- [grpc-go health checking protocol](https://github.com/grpc/grpc-go/blob/master/Documentation/health-checking.md)
- [charmbracelet/crush](https://github.com/charmbracelet/crush) — production-grade Go server with errgroup + LSP subprocess lifecycle
- [gosec G112](https://github.com/securego/gosec) — `http.Server.ReadHeaderTimeout` 미명시 Slowloris 취약점
- [go.uber.org/goleak](https://github.com/uber-go/goleak) — goroutine leak detection in tests
- [testcontainers-go](https://github.com/testcontainers/testcontainers-go) — postgres + redis container 부팅

---

## 7. Conclusion

본 SPEC은 표준 Go 서버 부팅 패턴 (`signal.NotifyContext` + `errgroup` + `http.Server.Shutdown` + `grpcServer.GracefulStop`)을 충실히 적용한다. 추가 의존성은 `golang.org/x/sync/errgroup` 1건 + `go.uber.org/goleak` (테스트 only) 1건. 비즈니스 로직 0줄 — 4개 SPEC GREEN 산출물을 조립하는 calling code만 작성. 위험은 (a) goroutine leak, (b) signal race, (c) testcontainers CI 느림 3종이며 모두 검증된 완화책 존재.

KEPCO 운영 배포 prerequisite로서 본 SPEC GREEN이 없으면 4개 SPEC 모두 PR merge되어도 실제 서버 부팅 불가. AUTH-002 §5 Exclusion #12 정식 해소 + CTRL-001 Sprint 7 미완성 gap 흡수로 통합 진입점 완성.
