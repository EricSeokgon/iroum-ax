# SPEC-AX-SERVER-001 — Acceptance Criteria

> SPEC: `SPEC-AX-SERVER-001 v0.1.2`
> Format: Given / When / Then (G/W/T)
> Coverage: 5 REQ modules × ≥ 2 AC = 14 AC (target ≥ 12 충족) + S0 helper/어댑터 3 AC
> Edge cases: port conflict, DB failure, signal race, listener death asymmetry, probe during shutdown
> iteration 2~3: 모든 AC가 spec.md §2.0 검증된 실제 API만 단언 (phantom API 제거 — plan-audit D1~D6 대응; D9 redis 어댑터 경유 반영)

---

## 0. S0 Pre-req Helper 메서드 AC (iteration 2 신규 — D1/D4 해소 검증)

### AC-SERVER-S0-PgPing — `PgWorkflowStore.Ping(ctx) error` 동작

- **Given**:
  - testcontainers postgres 정상 부팅 → `store.NewPgWorkflowStore(ctx, dsn, logger)` 성공
- **When**:
  - `err := pgStore.Ping(ctx)` 호출 (정상 상태)
  - postgres container stop 후 동일 호출 (`err2 := pgStore.Ping(ctx)`)
- **Then**:
  - `err == nil` (정상 연결)
  - `err2 != nil` AND `err2.Error()`에 `"postgres ping 실패"` 포함 (`s.pool.Ping(ctx)` 래핑 검증)
  - 기존 메서드(`Close`/`BeginTx`/`PoolStats`/`ListWorkflows`) 시그니처/동작 불변 (기존 store 테스트 모두 통과 — regression 0)

### AC-SERVER-S0-JWKSReachable — `JWKSCache.Reachable(ctx) bool` 동작

- **Given**:
  - 정적 JWKS HTTP server 부팅 → `jc := auth.NewJWKSCache(jwksURI)`
- **When**:
  - fetch 전: `r0 := jc.Reachable(ctx)`
  - `jc.GetKey(ctx, validKid)` 1회 호출(첫 fetch 강제) 후: `r1 := jc.Reachable(ctx)`
  - JWKS server stop + `staleMaxAge` 경과 시뮬레이션(테스트에서 `lastSuccessAt`를 과거로 hook 또는 짧은 `WithStaleMaxAge`) 후: `r2 := jc.Reachable(ctx)`
- **Then**:
  - `r0 == false` (한 번도 fetch 성공 안 함 — `lastSuccessAt` zero)
  - `r1 == true` (fetch 성공 + stale 유효 기간 내)
  - `r2 == false` (`cacheAge >= staleMaxAge`)
  - `Reachable`은 `c.mu.RLock()` 보유 하에 `lastSuccessAt`/`cacheAge()`를 평가 (D11: `cacheAge()`의 "호출자 mu.RLock 보유" 동시성 계약 준수). `go test -race`로 `GetKey`(write lock)와 `Reachable`(read lock) 동시 호출 시 data race 0건 검증
  - 기존 `GetKey`/`refresh`/`JWKSProvider` 인터페이스 동작 불변 (기존 auth 테스트 모두 통과 — regression 0)

### AC-SERVER-S0-RedisAdapter — `scheduler.RedisClientAdapter` 인터페이스 충족 (iteration 3 신규 — D9 해소 검증)

- **Given**:
  - `internal/scheduler/redis_adapter.go` production 파일 존재 (e2e_test.go:199~209 `goRedisAdapter`에서 promote)
  - testcontainers redis 부팅 → `rc := redis.NewClient(&redis.Options{Addr: <container>})`
- **When**:
  - compile-time assertion `var _ scheduler.RedisClient = (*scheduler.RedisClientAdapter)(nil)` 컴파일
  - `ad := scheduler.NewRedisClientAdapter(rc)` 후 `n, err := ad.RPush(ctx, "q", []byte("payload"))` 및 `perr := ad.Ping(ctx)` 호출
- **Then**:
  - compile-time assertion 통과 (어댑터가 `scheduler.RedisClient` 인터페이스 충족 — raw `*redis.Client`는 충족 못 함을 검증: `var _ scheduler.RedisClient = (*redis.Client)(nil)`는 컴파일 실패해야 함)
  - `err == nil` AND `n >= 1` (`RPush → .Result()` 변환 검증)
  - `perr == nil` (`Ping → .Err()` 변환 검증)
  - `scheduler.NewCeleryDispatcher(ad, "celery", "host")`가 컴파일·동작 (wiring 단계 (h) 경로 검증)
  - 기존 `scheduler.RedisClient` 인터페이스/`CeleryDispatcher`/test fake 동작 불변 (regression 0); e2e_test.go test-only `goRedisAdapter`는 미수정

---

## 1. REQ-SERVER-UBI-001 — Ubiquitous (3 sub-clauses, 각 dedicated AC)

### AC-SERVER-UBI-001-a — 모든 startup/shutdown 단계 audit (lessons #2)

- **Given**:
  - testcontainers postgres + redis 부팅 완료
  - 정적 JWKS HTTP server 부팅 완료
  - `cfg.AuthEnabled=true` + `cfg.OIDCIssuerURL=<jwks_test_server>`
  - audit_logs 테이블 비어있음 (truncate)
- **When**:
  - `srv, err := server.New(cfg, logger)` 호출 → `err == nil` 확인
  - 별도 goroutine에서 `srv.Run(ctx)` 호출 (ctx는 cancellable)
  - `GET /ready` 200 응답 대기 (server fully up)
  - `cancel()` 호출로 ctx 종료 (graceful shutdown 트리거)
  - `<-srv.Done()` 또는 `Run()` 종료 대기
- **Then**:
  - audit_logs에 정확히 3건 row 존재:
    - `(action=SERVER_STARTUP, user_id=system, details->>'grpc_addr'=':<port>', details->>'rest_addr'=':<port>')`
    - `(action=SERVER_SHUTDOWN_INITIATED, user_id=system, details->>'signal'='context_canceled')`
    - `(action=SERVER_SHUTDOWN_COMPLETED, user_id=system, details->>'exit_reason'='graceful', details->>'uptime_seconds'>=0)`
  - `created_at` 순서: STARTUP < SHUTDOWN_INITIATED < SHUTDOWN_COMPLETED

### AC-SERVER-UBI-001-b — Dependency 순서 강제

- **Given**:
  - `server.New()` 내부 단계 진입을 추적하는 captured slice (테스트 hook — 각 wiring 단계 시작 시 stepName append)
  - testcontainers postgres + redis + 정적 JWKS HTTP server (실제 API 호출 — phantom mock 금지, spec.md §2.0 검증 API만 사용)
  - `cfg.AuthEnabled=true`
- **When**:
  - `srv, err := server.New(cfg, logger)` 호출 → `err == nil`
- **Then**:
  - captured slice 순서가 정확히 spec.md REQ-SERVER-002-E1 enum과 동일: `["pg_store", "pg_ping", "redis_client", "redis_ping", "oidc_client", "jwks_warmup", "token_validator", "refresh_store", "recorder", "tx_coordinator", "state_machine", "celery_dispatcher", "workflow_service", "rest_handler", "auth_chain"]`
  - 특히 `recorder` → `tx_coordinator` → `state_machine` 순서가 보장됨 (D6: `workflow.NewStateMachine(txCoord, logger)`가 `workflow.NewTxCoordinator(pgStore, recorder)` 선행 의존; `recorder`는 `audit.NewRecorder(cfg.AuthEnabled)`)
  - `celery_dispatcher` 단계는 `scheduler.NewRedisClientAdapter(redisClient)`(S0 promote, infallible struct 래핑) → `scheduler.NewCeleryDispatcher(redisAdapter, ...)` 순으로 진행 (D9: raw `*redis.Client`는 `scheduler.RedisClient` 인터페이스를 직접 충족 못 함). 어댑터 생성은 추적 항목이 아니므로 captured slice는 `celery_dispatcher` 단일 항목으로 15-element 유지(어댑터 별도 stepName 미추가)
  - `token_validator`는 `auth.New(ctx, issuer, audience, auth.WithJWKSProvider(jwksCache))` 호출, `refresh_store`는 `auth.NewRedisRefreshStore(addr)` 호출에 대응 (D3: phantom `NewTokenValidator`/`NewRefreshTokenStore` 아님)
  - 어떤 단계도 직전 단계의 captured 항목 추가 전에 호출되지 않음
  - `cfg.AuthEnabled=false` 변형: captured slice에서 `oidc_client`/`jwks_warmup` 두 항목이 제거된 13-element slice

### AC-SERVER-UBI-001-c — Graceful guarantee (in-flight 요청 30s 대기)

- **Given**:
  - 실제 server 부팅 (testcontainers full stack)
  - in-flight 시뮬레이션: `POST /api/v1/workflows`를 처리하는 핸들러를 인위적으로 5s sleep (test hook via slow store mock)
  - `cfg.ShutdownTimeoutSeconds=30` (default)
- **When**:
  - 클라이언트가 `POST /api/v1/workflows` 시작 (t=0)
  - t=1s 시점에 `SIGTERM` 전송 (`cancel(ctx)`)
- **Then**:
  - `POST /api/v1/workflows` 응답은 `201 Created` 정상 반환 (t=5s 시점)
  - server는 t=5s 이후에야 graceful shutdown 완료 (force-kill 미발생)
  - `SERVER_SHUTDOWN_COMPLETED` audit row의 `details.exit_reason='signal_sigterm'`
  - `details.uptime_seconds >= 5` 검증

---

## 2. REQ-SERVER-001 — Dual Listener

### AC-SERVER-001-E1 — gRPC listener bind on configured address

- **Given**:
  - `cfg.GRPCAddr=":0"` (random port)
- **When**:
  - `srv.Run(ctx)` 호출 (별도 goroutine)
  - `srv.GRPCAddr()` getter로 실제 바인딩된 주소 획득 (test helper)
- **Then**:
  - `net.Dial("tcp", srv.GRPCAddr())` 연결 성공
  - `grpc.Dial(srv.GRPCAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))` + `WorkflowService.GetWorkflow(non-existent-id)` 호출 → `codes.NotFound` 응답 (서비스 등록 확인)

### AC-SERVER-001-E2 — REST listener bind on configured address

- **Given**:
  - `cfg.RESTAddr=":0"` (random port)
- **When**:
  - `srv.Run(ctx)` 호출
  - `srv.RESTAddr()` getter
- **Then**:
  - `http.Get("http://" + srv.RESTAddr() + "/health")` → HTTP 200 + body `{"status":"ok",...}`
  - `http.Server`의 `ReadHeaderTimeout`은 10s로 명시되어 있음 (G112 회피, reflection 또는 spy로 검증)

### AC-SERVER-001-U1 — Port conflict graceful error

- **Given**:
  - 사전 점유: `pre, _ := net.Listen("tcp", "127.0.0.1:0")` → `cfg.GRPCAddr = pre.Addr().String()`
  - `cfg.RESTAddr=":0"` (REST는 정상)
- **When**:
  - `srv, err := server.New(cfg, logger)` (`New`는 listener bind 단계 없으므로 성공)
  - `err = srv.Run(ctx)` 호출
- **Then**:
  - `err != nil` 반환
  - `err.Error()` 에 `"listener bind failed on <addr>"` 포함
  - `errors.Is(err, syscall.EADDRINUSE)` true
  - audit_logs에 `SERVER_STARTUP` row 미생성 (failed startup은 audit 미기록)

### AC-SERVER-001-U2 — One listener dies, the other shuts down

- **Given**:
  - 서버 부팅 완료 (`GET /ready` 200 확인)
- **When**:
  - 테스트 hook으로 `srv.grpcServer.Stop()` 강제 호출 (gRPC listener 비정상 종료 시뮬레이션)
- **Then**:
  - REST `httpServer.Shutdown()`이 5s 이내 호출됨 (graceful 절차 진입 확인)
  - `srv.Run(ctx)`이 `grpc.ErrServerStopped` 또는 wrapped error 반환 (전체 종료)
  - `http.Get("http://" + srv.RESTAddr() + "/health")` → connection refused 또는 timeout

---

## 3. REQ-SERVER-002 — Dependency Wiring

### AC-SERVER-002-E1 — Sequential init with explicit error wrapping

- **Given**:
  - 도달 불가능한 `cfg.PostgresDSN` (예: 잘못된 포트) → `store.NewPgWorkflowStore` 생성자 내부 `pool.Ping` 또는 후속 명시적 `pgStore.Ping(ctx)`가 실패
- **When**:
  - `srv, err := server.New(cfg, logger)`
- **Then**:
  - `srv == nil`
  - `err != nil`
  - `err.Error()`가 `"init step pg_store failed: ..."` 또는 `"init step pg_ping failed: ..."` 형태 (단계 (c) 생성자 실패 vs 후속 명시 Ping 실패에 따라 stepName이 `pg_store` 또는 `pg_ping`)
  - `errors.Unwrap(err)`가 nil이 아니며 원인 에러 보존 (`%w` wrapping 확인)
  - stepName은 spec.md REQ-SERVER-002-E1 enum의 원소이며 enum은 `pg_store`부터 시작(config/logger는 infallible이므로 enum 밖, D8)

### AC-SERVER-002-E2 — Partial cleanup on failure (reverse order)

- **Given**:
  - testcontainers postgres 정상 부팅 (`*PgWorkflowStore.Close()` 호출 여부를 wrapper spy로 관측 — concrete type 그대로, phantom 메서드 미사용)
  - redis 미가용 주소 → 단계 (d) `redisClient.Ping(ctx).Err()` 실패
- **When**:
  - `srv, err := server.New(cfg, logger)` → `err != nil` (`"init step redis_ping failed"`) 확인
- **Then**:
  - `pgStore.Close()`가 정확히 1회 호출됨 (역순 cleanup — 명시적 정리 대상은 `pgStore.Close()` 뿐, 단계 (d) redis client는 Ping 실패했어도 `redisClient.Close()`로 정리)
  - `tokenValidator`/`oidcClient`/`jwksCache`/`celeryDispatcher`에 대한 Close 호출은 존재하지 않음 (해당 타입에 Close 메서드 없음 — D5 정정, phantom `celeryDispatcher.Close()`/`tokenValidator.Close()` 단언 제거)
  - `srv == nil` 반환

### AC-SERVER-002-E3 — Auth chain composition single line mount

- **Given**:
  - 정상 부팅 환경 + `cfg.AuthEnabled=true`
- **When**:
  - `srv, _ := server.New(cfg, logger)` 후 reflection 또는 test hook으로 `srv.httpServer.Handler` 검증
- **Then**:
  - `srv.httpServer.Handler`는 `auth.BuildRESTChain()`의 반환 타입과 일치 (체인 wrapper 확인)
  - `srv.grpcServerOptions`에 `auth.BuildGRPCInterceptorChain()` 결과가 포함됨
  - 직접 chain 함수 호출 단언: `viewer DELETE /api/v1/workflows/<id>` → HTTP 403 (AUTH-002 chain 정상 wired)

### AC-SERVER-002-U1 — DB connection failure aborts startup

- **Given**:
  - testcontainers postgres container를 부팅 전에 stop
  - `cfg.PostgresDSN`은 유효하나 connection refused
- **When**:
  - `srv, err := server.New(cfg, logger)`
- **Then**:
  - `err != nil`, `err.Error()` includes `"init step pg_store failed"` 또는 `"init step pg_ping failed"` (생성자 내부 ping 실패 vs 후속 명시적 `pgStore.Ping(ctx)` 실패)
  - listener bind 단계 진입하지 않음 (`net.Listen` 호출 없음)
  - audit_logs `SERVER_STARTUP` 미생성

### AC-SERVER-002-U2 — JWKS fetch failure aborts startup when AuthEnabled=true

- **Given**:
  - `cfg.AuthEnabled=true`
  - `cfg.OIDCIssuerURL="http://nonexistent.invalid:9999/realms/iroum-ax"` (DNS failure)
- **When**:
  - `srv, err := server.New(cfg, logger)`
- **Then**:
  - `err != nil`, `err.Error()` includes `"init step oidc_client failed"` (Discovery 실패) 또는 `"init step jwks_warmup failed"` (JWKS 첫 fetch 실패)
  - 역순 cleanup: `redisClient.Close()` + `pgStore.Close()` 각 1회 호출 (이미 init된 단계 (c)/(d)의 closable만; tokenValidator/oidcClient/jwksCache는 Close 없으므로 정리 대상 아님)
  - 동일 시나리오에서 `cfg.AuthEnabled=false`이면 `err == nil` (단계 (e) OIDC client + JWKSCache + warm-up 전체 skip)

---

## 4. REQ-SERVER-003 — Graceful Shutdown

### AC-SERVER-003-E1 — Signal handling triggers shutdown sequence

- **Given**:
  - 실제 server 부팅 (`GET /ready` 200)
- **When**:
  - `syscall.Kill(os.Getpid(), syscall.SIGTERM)` 호출 (또는 testable equivalent `cancel(ctx)`)
- **Then**:
  - shutdown 순서 검증: `httpServer.Shutdown()` 호출 시점 < `grpcServer.GracefulStop()` 호출 시점 < `redisClient.Close()` 호출 시점 (go-redis `*redis.Client.Close() error`) < `pgStore.Close()` 호출 시점 (`*PgWorkflowStore.Close()`, 반환값 없음) < `SERVER_SHUTDOWN_COMPLETED` audit row insert 시점
  - `tokenValidator`/`oidcClient`/`jwksCache`/`celeryDispatcher`에 대한 Close 호출 없음 (해당 타입에 Close 메서드 없음)
  - 모든 단계가 30s 이내 완료
  - `srv.Run()` returns nil (정상 종료)

### AC-SERVER-003-S1 — Shutdown idempotency

- **Given**:
  - 서버 부팅 완료
- **When**:
  - `srv.shutdown(ctx, "test")` 직접 호출 (test hook)
  - 1s 대기
  - `srv.shutdown(ctx, "test_second")` 두 번째 호출
- **Then**:
  - 두 번째 호출은 즉시 return (no-op, sync.Once 보장)
  - audit_logs `SERVER_SHUTDOWN_COMPLETED` row 정확히 1건 (중복 없음)
  - `httpServer.Shutdown()` mock 호출 횟수 = 1

### AC-SERVER-003-S1-DoubleSignal — Second signal forces kill

- **Given**:
  - 서버 부팅 완료
  - in-flight 요청 60s sleep 진행 중
  - `cfg.ShutdownTimeoutSeconds=30`
- **When**:
  - 첫 SIGTERM (t=0) → graceful 시도
  - 1s 대기
  - 두 번째 SIGINT (t=1s) → force-kill 시도
- **Then**:
  - 두 번째 신호 직후 `grpcServer.Stop()` + `httpServer.Close()` 호출 (force)
  - in-flight 요청은 connection reset (501 또는 client error)
  - `SERVER_SHUTDOWN_COMPLETED` audit row `details.exit_reason='double_signal_force'`

### AC-SERVER-003-U1 — Shutdown timeout force-kill

- **Given**:
  - 서버 부팅 완료
  - in-flight 요청 60s sleep
  - `cfg.ShutdownTimeoutSeconds=2`
- **When**:
  - `cancel(ctx)` 호출 (t=0)
- **Then**:
  - t=2s 시점에 `httpServer.Close()` 강제 호출
  - in-flight 요청 connection reset
  - audit_logs `SERVER_SHUTDOWN_COMPLETED.details.exit_reason='force_kill_timeout'`
  - log에 `"shutdown timeout exceeded, force kill"` warning 기록

---

## 5. REQ-SERVER-004 — Health & Readiness

### AC-SERVER-004-E1 — Liveness probe always returns 200

- **Given**:
  - 서버 부팅 완료 + `cfg.AuthEnabled=true`
- **When**:
  - `GET /health` (인증 없음) 호출
- **Then**:
  - HTTP 200
  - body 정확히 `{"status":"ok","service":"iroum-ax-control-plane","version":"<version>"}`
  - 응답 시간 p99 < 5ms (1000회 측정)
  - 인증 헤더 없어도 통과 (AUTH-002 chain 외부 mount 확인)

### AC-SERVER-004-E2 — Readiness probe all-pass returns 200

- **Given**:
  - postgres, redis, JWKS HTTP server 모두 정상
  - `cfg.AuthEnabled=true`
- **When**:
  - `GET /ready` 호출
- **Then**:
  - HTTP 200 (내부적으로 `pgStore.Ping(ctx)` nil + `redisClient.Ping(ctx).Err()` nil + `jwksCache.Reachable(ctx) == true` — 모두 S0/검증 API)
  - body 정확히 `{"status":"ready","checks":{"postgres":"ok","redis":"ok","oidc":"ok"}}`
  - `cfg.AuthEnabled=false` 변형: `oidc` 검사 skip, body `checks`에 `oidc` 키 생략 또는 `"skipped"`
  - 응답 시간 p95 < 500ms

### AC-SERVER-004-E2-DBDown — Readiness probe DB failure returns 503

- **Given**:
  - testcontainers postgres 강제 stop
  - redis 정상, JWKS 정상
- **When**:
  - `GET /ready`
- **Then**:
  - HTTP 503 (`pgStore.Ping(ctx)`가 non-nil error 반환 — S0 메서드, `"postgres ping 실패: ..."` 래핑)
  - body `{"status":"not_ready","checks":{"postgres":"failed: <postgres ping 실패 ...>","redis":"ok","oidc":"ok"}}`
  - `Content-Type: application/json`

### AC-SERVER-004-E3 — gRPC health check SERVING

- **Given**:
  - 정상 부팅 + 모든 dependency healthy
- **When**:
  - `grpc_health_v1.NewHealthClient(conn).Check(ctx, &HealthCheckRequest{Service: ""})`
- **Then**:
  - response `Status == SERVING`
  - 인증 없이 호출 가능 (AUTH-002 매핑 `/grpc.health.v1.Health/Check` → bypass)

### AC-SERVER-004-S1 — Probe during shutdown returns 503

- **Given**:
  - 정상 부팅
  - shutdown 시작 (`cancel(ctx)` 호출 직후, t=0)
- **When**:
  - t=100ms 시점에 `GET /ready`
- **Then**:
  - HTTP 503
  - body `{"status":"shutting_down"}`
  - `GET /health` 동일 시점에는 200 (liveness는 process down 직전까지 200 유지)

### AC-SERVER-004-U1 — Probe timeout marks failed

- **Given**:
  - `pgStore.Ping(ctx)` 호출이 `ctx` deadline까지 블록되도록 구성 (예: 응답 없는 postgres 또는 매우 짧은 `cfg.ReadyProbeTimeoutSeconds`로 ctx deadline 유도 — `Ping`은 ctx-aware이므로 deadline 시 `context.DeadlineExceeded` 계열 error 반환)
  - `cfg.ReadyProbeTimeoutSeconds=2`
- **When**:
  - `GET /ready`
- **Then**:
  - 2s 이내 응답 도착 (probe handler가 `context.WithTimeout(cfg.ReadyProbeTimeoutSeconds)` 적용)
  - HTTP 503
  - body `{"status":"not_ready","checks":{"postgres":"failed: timeout exceeded 2s","redis":"ok","oidc":"ok"}}`

---

## 6. Edge Cases & Cross-cutting AC

### AC-SERVER-Edge-PortZero — `:0` random port assignment

- **Given**: `cfg.GRPCAddr=":0"` + `cfg.RESTAddr=":0"`
- **When**: `srv.Run(ctx)` 부팅 후 `srv.GRPCAddr()` / `srv.RESTAddr()` 호출
- **Then**: 둘 다 `:0`이 아닌 실제 할당된 포트 반환 (e.g., `127.0.0.1:54321`)

### AC-SERVER-Edge-AuthDisabledFullPath — AuthEnabled=false backward compat

- **Given**: `cfg.AuthEnabled=false`
- **When**: `srv.Run(ctx)` 부팅 후 `POST /api/v1/workflows` (헤더 없음)
- **Then**:
  - HTTP 201 응답 (AUTH-002 chain 외부 mount 또는 chain 내 bypass)
  - audit_logs `WORKFLOW_CREATED.user_id='cli-anonymous'` (SPEC-AX-001 / CTRL-001 / AUTH-001 invariant 보존)
  - JWKS warm-up step (e)는 skip되었음 (audit log 또는 spy 확인)

### AC-SERVER-Edge-FatalErrorDuringRun — fatal error triggers shutdown

- **Given**: 정상 부팅
- **When**: 테스트 hook으로 errgroup에 fatal error 주입 (e.g., listener accept loop에서 panic recover)
- **Then**:
  - shutdown 자동 트리거
  - audit_logs `SERVER_SHUTDOWN_COMPLETED.details.exit_reason='fatal_error_grpc_serve'` (또는 적절한 phase)
  - `srv.Run(ctx)` 반환 error에 wrapped fatal error 포함

---

## 7. Quality Gate (Definition of Done — Verification subset)

- [ ] 모든 단위 테스트 통과 (`go test ./cmd/server/... -race -count=1`)
- [ ] 모든 E2E 통과 (`go test -tags=integration ./cmd/server/... -timeout=300s`)
- [ ] coverage ≥ 85% (`go test -cover ./cmd/server/...`)
- [ ] `golangci-lint run ./cmd/server/...` 0 issue
- [ ] `gosec ./cmd/server/...` G112 0건 (ReadHeaderTimeout 명시 확인)
- [ ] `goleak.VerifyNone(t)` 모든 server_test에서 0 goroutine leak
- [ ] benchmark: startup p95 < 5s, /ready p95 < 500ms
- [ ] @MX tags 4개 추가 확인 (`Server.Run` ANCHOR, `Server.shutdown` WARN+REASON, `main()` NOTE, `readinessHandler` ANCHOR)
- [ ] AUTH-002 §5 Exclusion #12 historical 메모 본 SPEC HISTORY에 명시 (이미 명시됨)
- [ ] S0 helper/어댑터 AC 3개 PASS (`AC-SERVER-S0-PgPing` + `AC-SERVER-S0-JWKSReachable`(`-race`) + `AC-SERVER-S0-RedisAdapter`)
- [ ] AC 14개 + S0 helper/어댑터 3개 = 17개 모두 PASS
- [ ] 모든 AC가 spec.md §2.0 검증 API만 단언 (phantom API 단언 0건 — plan-audit iteration 1 D1~D6 회귀 방지; iteration 2 D9 redis 어댑터 경유 반영 — raw `*redis.Client` 직접 dispatcher 주입 단언 0건)

---

## 8. Schema Note 재확인

본 acceptance.md는 spec.md frontmatter의 8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`)와 EARS 4-pattern 분류(Ubiquitous + Event-driven + State-driven + Unwanted; **Optional 미해당** — 서버 부팅·종료·헬스체크는 인프라 불변 조건으로 조건부/선택적 기능이 존재하지 않음. spec.md §3:L129와 정합, D12 정정 — iteration 2의 acceptance.md "5-pattern" 표현은 부정확했으며 실제 4-pattern)를 그대로 참조한다. plan-auditor가 변형 필드 또는 패턴 개수 결함을 제기 시 spec.md "Schema note" + spec.md §3:L129 + plan.md §7 + 본 §8을 출처로 거부한다. (lessons_session_2026_05_14 #1 적용)
