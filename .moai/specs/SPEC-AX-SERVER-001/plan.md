# SPEC-AX-SERVER-001 — Implementation Plan

> SPEC: `SPEC-AX-SERVER-001 v0.1.0`
> Methodology: DDD (testcontainers 무거움 + brownfield `server.go` stub 분석 우선 → ANALYZE-PRESERVE-IMPROVE 친화). 단, 신규 파일 3개는 TDD RED-GREEN-REFACTOR로 진행.
> Harness level: `thorough` (full-stack E2E + 4개 SPEC의 통합 진입점, KEPCO 운영 배포 blocker — Sprint Contract Protocol 필수)
> Cross-SPEC: AUTH-002 §5 Exclusion #12 정식 해소; CTRL-001 Sprint 7 (`cmd/server/server.go` 실제 부팅) 미완성 gap 흡수

---

## 1. 전체 전략

본 SPEC은 4개 선행 SPEC(AX-001 + CTRL-001 + AUTH-001 + AUTH-002 v0.1.2)이 정의한 components를 조립·부팅·종료하는 calling code만 작성한다. 비즈니스 로직은 0줄 — 모두 dependency wiring + lifecycle 관리. 따라서 위험은 (a) 부팅 순서 violation으로 인한 nil pointer panic, (b) goroutine leak / shutdown race, (c) testcontainers 환경 무거움으로 인한 CI 느림. 이를 다음 3 Sprint로 분해한다.

DAG 구성:

```
S0 (Pre-req chores)
  │  ├── audit/actions.go에 SERVER_STARTUP/SHUTDOWN_INITIATED/SHUTDOWN_COMPLETED 3 const 추가
  │  ├── config/config.go에 ShutdownTimeoutSeconds + ReadyProbeTimeoutSeconds 2 field 추가 (env: SHUTDOWN_TIMEOUT_SECONDS=30, READY_PROBE_TIMEOUT_SECONDS=5)
  │  └── go.mod: golang.org/x/sync/errgroup require 추가
  ▼
S1 (Core bootstrap + dual listener) ←─── 본 SPEC의 60% 작업
  │  ├── cmd/server/server.go 전면 재작성: New() + Run(ctx) + shutdown(ctx)
  │  ├── cmd/server/main.go 신규: os.Args 무관, signal.NotifyContext + Server.Run(ctx) + os.Exit
  │  ├── REQ-SERVER-001 E1/E2 dual listener (errgroup)
  │  ├── REQ-SERVER-002 E1/E2/E3 dependency wiring + reverse cleanup
  │  └── 단위 테스트: TestServerNew_DependencyOrder + TestServerNew_PgPingFailure + TestServerRun_PortConflict
  ▼ (parallel branches possible — S2 + S3 partially parallelizable)
S2 (Graceful shutdown + signal handling)
  │  ├── REQ-SERVER-003 E1/E2/S1/U1 signal handling + idempotency + force-kill
  │  ├── REQ-SERVER-UBI-001-a/c audit row 3종
  │  └── 단위 테스트: TestServerShutdown_DoubleSignal + TestServerShutdown_TimeoutForceKill
  ▼
S3 (Health/Readiness probes + full-stack E2E + AUTH-002 chain mount 검증)
   ├── cmd/server/probes.go: GET /health + GET /ready + grpc_health_v1.HealthServer
   ├── REQ-SERVER-004 E1/E2/E3/S1/U1
   ├── E2E: testcontainers postgres + redis + static JWKS HTTP server → REST + gRPC + ready + SIGTERM 통합
   └── benchmark: BenchmarkServerStartup + BenchmarkReadyProbe
```

`server.go` 전면 재작성은 S1에 집중하여 SAS(Server Anti-corruption Surface)를 한 번에 그리고, S2/S3는 그 위에 부가하므로 file write 충돌 없음. S3 E2E는 S1/S2 결과를 모두 검증하므로 마지막에 배치.

---

## 2. Sprint 상세 (priority 기반, time estimate 미사용)

### S0 — Pre-requisite Chores (Priority: High, 선행 필수)

**Deliverables**:

1. `apps/control-plane/internal/audit/actions.go`에 3 const 추가:
   - `ActionServerStartup = "SERVER_STARTUP"`
   - `ActionServerShutdownInitiated = "SERVER_SHUTDOWN_INITIATED"`
   - `ActionServerShutdownCompleted = "SERVER_SHUTDOWN_COMPLETED"`
2. `apps/control-plane/internal/config/config.go`에 2 field 추가:
   - `ShutdownTimeoutSeconds int` (env: `SHUTDOWN_TIMEOUT_SECONDS`, default 30)
   - `ReadyProbeTimeoutSeconds int` (env: `READY_PROBE_TIMEOUT_SECONDS`, default 5)
3. `go.mod`: `require golang.org/x/sync v0.11.0` (or 현재 최신 안정). `go mod tidy` 실행.

**Verification**:
- `go build ./...` 성공
- 기존 SPEC AC 모두 그대로 통과 (regression 없음)

**Risk**: 매우 낮음. const 추가 + config field 추가 + 표준 의존성.

**Cross-SPEC impact**: 
- `audit/actions.go` 추가 const는 AUTH-001 + AUTH-002가 사용 중인 `ActionAuthForbidden` 등과 동일 패턴이므로 충돌 없음.
- `config.go` field 추가는 CTRL-001 / AUTH-001이 사용 중인 `cfg.AuthEnabled` 등과 동일 패턴.

---

### S1 — Core Bootstrap + Dual Listener (Priority: High, 본 SPEC의 핵심)

**Deliverables**:

1. **`apps/control-plane/cmd/server/server.go` 전면 재작성** (현재 40-line stub → 약 200~250 LOC):
   - struct `Server { logger, cfg, pgStore, redisClient, oidcClient, tokenValidator, refreshTokenStore, sm, dispatcher, workflowSvc, restHandler, grpcServer, httpServer, shutdownOnce sync.Once, shutdownDone chan struct{} }`
   - `New(cfg *config.Config, logger *zap.Logger) (*Server, error)`: REQ-SERVER-UBI-001-b 단계 (a)~(j) sequential init. 각 단계 실패 시 `fmt.Errorf("init step %s failed: %w", stepName, err)` + `partialCleanup(initialized)` 역순 호출.
   - `Run(ctx context.Context) error`: REQ-SERVER-001 dual listener (errgroup), REQ-SERVER-UBI-001-a `SERVER_STARTUP` audit row, REQ-SERVER-UBI-001-c shutdown 대기.
   - `shutdown(ctx context.Context, reason string)`: REQ-SERVER-003 (S2에서 본격 구현; S1에서는 stub).
2. **`apps/control-plane/cmd/server/main.go` 신규** (약 50 LOC):
   - `func main()`: logger 생성 → `config.Load()` → `signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)` → `srv, err := server.New(cfg, logger)` → `if err != nil { os.Exit(1) }` → `if err := srv.Run(ctx); err != nil { os.Exit(1) }` → `os.Exit(0)`.
3. **`apps/control-plane/cmd/server/server_test.go` (S1 portion)**:
   - `TestServerNew_DependencyOrder`: mock store + mock redis + mock OIDC; 각 단계 호출 순서를 captured slice로 검증.
   - `TestServerNew_PgPingFailure`: `pgStore.Ping` returns error → `Server.New()` returns wrapped error, `redisClient` 미생성 확인.
   - `TestServerNew_RedisPingFailure`: redis Ping 실패 → pg store close 확인 (reverse cleanup).
   - `TestServerNew_AuthDisabledSkipsOIDC`: `cfg.AuthEnabled=false` → OIDCClient 생성 skip, `srv.oidcClient == nil` 확인.
   - `TestServerRun_PortConflict`: pre-bound listener로 :50051 점유 → `Server.Run()` returns wrapped error.
   - `TestServerRun_BothListenersBind`: 임의 free port 2개로 errgroup 동작 검증 + immediate `cancel(ctx)` 호출 후 errgroup return.

**Verification**:
- `go test ./cmd/server/... -race -count=1` 6개 단위 테스트 통과
- `golangci-lint run ./cmd/server/...` 0 issue
- `gosec ./cmd/server/...` G112 0건

**Risk**:
- **Goroutine leak**: errgroup goroutine 누수 가능. 대응: 모든 테스트에 `defer goleak.VerifyNone(t)` 적용 (go.uber.org/goleak — 기존 SPEC에서 사용 중인지 확인 필요. 미사용 시 본 SPEC에서 도입).
- **Mock 복잡도**: 6개 dependency 모두 mock 필요. 대응: `interfaces.go` 신규 도입 대신 기존 interface(`store.WorkflowStore`, `auth.Validator` 등) 재사용 + `gomock` 또는 hand-rolled fake로 처리.
- **`net.Listen` test flakiness**: free port 충돌. 대응: `:0` (random port assignment) 사용 + `listener.Addr().String()` 후속 참조.

**Cross-SPEC impact**:
- AUTH-002 `chain.BuildRESTChain` / `BuildGRPCInterceptorChain`을 호출하는 첫 client code. AUTH-002 단위 테스트(`TestBuildRESTChain_Order`)가 보장하는 invariant를 신뢰.

---

### S2 — Graceful Shutdown + Signal Handling (Priority: High)

**Deliverables**:

1. **`apps/control-plane/cmd/server/server.go`** `shutdown(ctx, reason string)` 본격 구현:
   - `sync.Once` guard로 idempotency 강제 (REQ-SERVER-003-S1)
   - `httpServer.Shutdown(shutdownCtx)` 호출 (deadline `cfg.ShutdownTimeoutSeconds`)
   - `time.AfterFunc(timeout, grpcServer.Stop)` 으로 gRPC force-stop race + `grpcServer.GracefulStop()` 호출
   - `redisClient.Close()` + `pgStore.Close()`
   - `SERVER_SHUTDOWN_INITIATED` audit row insert (signal 수신 직후)
   - `SERVER_SHUTDOWN_COMPLETED` audit row insert (모든 cleanup 후, `details.exit_reason` = `"signal_sigterm"` / `"signal_sigint"` / `"force_kill_timeout"` / `"fatal_error_<step>"`)
2. **`apps/control-plane/cmd/server/server.go`** `Run(ctx)` 내부 select-loop 정교화:
   - `select { case <-ctx.Done(): s.shutdown(ctx, ctxCancelReason(ctx.Err())) case err := <-errgroupErrCh: s.shutdown(ctx, "fatal_error") return err }`
3. **`apps/control-plane/cmd/server/server_test.go` (S2 portion)**:
   - `TestServerShutdown_GracefulSIGTERM`: server 부팅 후 `SIGTERM` 전송 → 30s 이내 종료, 3개 audit row 검증.
   - `TestServerShutdown_DoubleSignal`: 첫 SIGTERM 후 1s 대기, 두 번째 SIGINT 전송 → 즉시 force-kill (`grpcServer.Stop()` + `httpServer.Close()`) 호출 확인.
   - `TestServerShutdown_TimeoutForceKill`: in-flight HTTP request를 인위적 sleep으로 60s 만들고 ShutdownTimeoutSeconds=2 설정 → 2s 후 force-kill + audit row `details.exit_reason="force_kill_timeout"` 확인.
   - `TestServerShutdown_Idempotency`: `s.shutdown()` 직접 2번 호출 → 두 번째 호출은 no-op 확인 (`sync.Once` 검증).

**Verification**:
- `go test ./cmd/server/... -race -count=3` 4개 단위 테스트 통과 (race detector + 3회 반복으로 flakiness 검출)
- `goleak.VerifyNone(t)` 모든 테스트 종료 시 goroutine 0개

**Risk**:
- **Signal handling race**: `signal.NotifyContext` + errgroup + sync.Once 3 component race. 대응: `TestServerShutdown_DoubleSignal`에서 실제 OS signal 전송 대신 `cancel()` 호출로 시뮬레이션 + 별도 `TestServerShutdown_RealSIGTERM`에서 `syscall.Kill(os.Getpid(), syscall.SIGTERM)` 사용 (린트로 인해 OS-specific build tag 필요할 수 있음).
- **gRPC Stop() vs GracefulStop() race**: 둘 다 호출되면 panic 가능. 대응: `time.AfterFunc` cancel via `timer.Stop()` 호출 + `done chan struct{}`로 GracefulStop 완료 시그널.

**Cross-SPEC impact**: 없음. shutdown은 모든 SPEC 공통 invariant.

---

### S3 — Health/Readiness Probes + Full-stack E2E (Priority: High)

**Deliverables**:

1. **`apps/control-plane/cmd/server/probes.go` 신규** (약 100 LOC):
   - `func livenessHandler(w http.ResponseWriter, r *http.Request)`: REQ-SERVER-004-E1 정적 응답.
   - `func readinessHandler(s *Server) http.HandlerFunc`: REQ-SERVER-004-E2 3 ping 병렬 (`errgroup` 또는 `sync.WaitGroup`), REQ-SERVER-004-U1 timeout, REQ-SERVER-004-S1 shutdown 중 503.
   - `type grpcHealthServer struct { srv *Server }`: REQ-SERVER-004-E3 implements `grpc_health_v1.HealthServer.Check(ctx, req) (*HealthCheckResponse, error)`. Watch는 unimplemented 반환.
2. **`apps/control-plane/cmd/server/server.go`** 통합:
   - `restMux := http.NewServeMux(); restMux.HandleFunc("GET /health", livenessHandler); restMux.HandleFunc("GET /ready", readinessHandler(s)); restMux.Handle("/", chain.BuildRESTChain(s.restHandler.Mux(), s.tokenValidator, s.auditRecorder, cfg.AuthEnabled))` — `/health` / `/ready`는 chain 외부에 등록하여 인증 bypass.
   - gRPC: `grpc_health_v1.RegisterHealthServer(grpcServer, &grpcHealthServer{srv: s})`.
3. **`apps/control-plane/cmd/server/server_e2e_test.go` 신규** (build tag `//go:build integration`, 약 150 LOC):
   - testcontainers-go 사용: `postgres:15-alpine` + `redis:7-alpine` + 정적 JWKS HTTP server(httptest)
   - `TestE2E_FullStack_REST`: 실제 `server.Run(ctx)` 부팅 → `GET /ready` 200 대기 → `POST /api/v1/workflows` (admin token) → `GET /health` 200 → SIGTERM 전송 (`cancel(ctx)`) → 종료 후 audit row 4건(STARTUP + WORKFLOW_CREATED + SHUTDOWN_INITIATED + SHUTDOWN_COMPLETED) 검증.
   - `TestE2E_FullStack_GRPC`: 동일 부팅 → `bufconn` 대신 실제 :50051 → `grpc.Dial` + `WorkflowService.CreateWorkflow` (admin token) → `grpc_health_v1.HealthClient.Check` SERVING 응답 → SIGTERM.
   - `TestE2E_ReadyProbe_DBDown`: postgres container 강제 stop → `GET /ready` 503 + body `{"status":"not_ready","checks":{"postgres":"failed: ..."}}` 검증.
   - `TestE2E_ReadyProbe_ShuttingDown`: SIGTERM 직후 (shutdown 진행 중) `GET /ready` 503 + body `{"status":"shutting_down"}` 검증.
4. **`apps/control-plane/cmd/server/server_bench_test.go` 신규** (약 50 LOC):
   - `BenchmarkServerStartup`: `server.New()` ~ `/health` 첫 200 응답까지 측정. Target p95 < 5s.
   - `BenchmarkReadyProbe`: 동일 부팅 후 `GET /ready` 1000회. Target p95 < 500ms.

**Verification**:
- `go test ./cmd/server/... -tags=integration -race -count=1 -timeout=300s` 4개 E2E 통과
- `go test -bench=. ./cmd/server/...` benchmark 결과 NFR 만족
- 통합 테스트는 testcontainers 의존이므로 별도 CI job (PR 머지 전 필수 통과)

**Risk**:
- **testcontainers CI 느림**: 3 container 부팅 ~30~60s. 대응: `t.Parallel()` 활용 + container reuse (testcontainers의 `Reuse: true`) + CI job 별도 분리.
- **Keycloak vs static JWKS**: Keycloak testcontainer는 startup 30s+. 대응: 정적 JWKS HTTP server(`httptest.NewServer` + RSA keypair pre-generated)로 대체. AUTH-001 E2E와 동일 패턴 재활용.
- **AUTH-002 `/ready` 매핑 미정의**: AUTH-002 §3.2 REST mapping table에 `/ready`가 없음 → default-deny 503 발동 가능. 대응: 본 SPEC S3에서 `/ready`를 chain 외부에 mount (REQ-SERVER-004-E1과 동일 패턴) → AUTH-002 chain 진입 자체 없음. 별도 chore로 AUTH-002 mapping table에 `/ready: bypass` 행 추가는 후속 (본 SPEC 범위 외 — chain 외부 mount로 우회 가능).

**Cross-SPEC impact**:
- 본 SPEC GREEN 후 AUTH-002 Exclusion #12 historical. AUTH-002 spec.md 자체 수정은 불필요(unblock fact만 보장).
- CTRL-001 Sprint 7 T-AX-006 closed (`cmd/server/server.go` 실제 부팅 완료). CTRL-001 spec.md 수정은 불필요.

---

## 3. Risk Register

| 위험 | 가능성 | 영향 | 대응 |
|------|--------|------|------|
| 포트 점유 (개발자 환경에서 `:8080` 또는 `:50051` 사용 중) | 중 | 중 | REQ-SERVER-001-U1 wrapped error + 명확한 stderr 메시지 + `:0` random assignment 옵션 (env `RANDOM_PORT=true`) |
| Dependency startup order violation | 낮 | 높 | REQ-SERVER-UBI-001-b 코드 강제 + `TestServerNew_DependencyOrder` 단위 테스트 |
| Signal handling race (SIGTERM + errgroup error 동시 발생) | 중 | 높 | `sync.Once` shutdown guard + `select` 명시적 case 분기 + `TestServerShutdown_DoubleSignal` 검증 |
| Goroutine leak (errgroup goroutine 미종료) | 중 | 중 | `go.uber.org/goleak`을 모든 server_test에 적용 + race detector |
| testcontainers CI 느림 → flaky | 중 | 중 | `Reuse: true` + 별도 CI job + `-timeout=300s` |
| gRPC `Stop()` vs `GracefulStop()` race → panic | 낮 | 높 | `time.AfterFunc` timer 명시적 `Stop()` + `done chan struct{}` 시그널 |
| `/ready` endpoint를 AUTH-002 default-deny가 차단 | 중 | 중 | `/ready`를 chain 외부에 mount (chain 진입 자체 없음). 별도 chore로 AUTH-002 매핑 추가 가능 (본 SPEC 범위 외) |
| Helm chart livenessProbe path 미설정 → K8s 운영 시 적용 안 됨 | 낮 | 중 | 본 SPEC 범위 외 (인프라 chore PR), 단 README에 명시 |
| `/health`가 의존성 실패에도 200 반환 → 잘못된 healthy 신호 | 의도된 동작 | - | REQ-SERVER-004-E1 의도. K8s liveness 의도는 "프로세스가 살아있나" — readiness가 의존성 검사 담당 |
| `cmd/server/server.go` 전면 재작성으로 기존 stub의 import 깨짐 | 중 | 낮 | 본 stub은 어떤 client code에도 호출되지 않음 (`Server.Run`은 main이 없어 호출자 없음). 안전. |

---

## 4. Cross-SPEC Coordination

### AUTH-002 §5 Exclusion #12 정식 해소

본 SPEC GREEN 종료 시 AUTH-002 §5 Exclusion #12("cmd/server/server.go 부트스트랩")는 **historical only** 상태로 전환된다. AUTH-002 spec.md 자체는 수정하지 않으며(frozen), 본 SPEC HISTORY 0.1.0에 unblock fact를 명시한다. 추후 AUTH-002 chore commit 가능: `## HISTORY` 항목으로 `0.1.3 (TBD): SPEC-AX-SERVER-001 v0.1.0 GREEN으로 §5 Exclusion #12 RESOLVED 메모 추가`. 본 SPEC 책임 외.

### CTRL-001 Sprint 7 T-AX-006 closed

CTRL-001 plan.md S7(T-AX-006 `cmd/server/server.go` 실제 부팅)을 본 SPEC이 흡수. CTRL-001 자체 수정은 불필요하나, `progress.md` 업데이트 시 "Sprint 7 absorbed by SPEC-AX-SERVER-001" 메모 권장 (본 SPEC 범위 외 chore).

### AUTH-001 SKIP unblock — 본 SPEC 범위 외

AUTH-001 `auth_e2e_test.go` `TestE2E_Auth_RBACForbidden`의 `t.Skip` 제거는 AUTH-002 책임 (AUTH-002 §3.5 REQ-AUTH2-004-U1 + plan.md S3 명시). 본 SPEC은 그것과 무관. 단 본 SPEC E2E(`TestE2E_FullStack_REST`)는 admin DELETE까지 검증할 수도 있으나, DELETE 핸들러 자체가 SPEC-AX-WF-DELETE-001 후속이므로 본 SPEC E2E는 POST + GET만 검증한다.

---

## 5. Methodology Selection (DDD vs TDD)

`.moai/config/sections/quality.yaml` `development_mode`에 따라 본 plan을 두 방식으로 적용 가능:

| Mode | S1 접근 | S2 접근 | S3 접근 |
|------|---------|---------|---------|
| **DDD** (현재 brownfield 권장) | ANALYZE: 현재 stub 분석 + 4개 SPEC GREEN 산출물 확인 → PRESERVE: 기존 stub은 호출자 없음 확인 → IMPROVE: 전면 재작성 (characterization test 불필요 — 기존 동작 zero) | 동일 패턴: 동작 zero 상태에서 새로 작성 | 동일 |
| **TDD** | RED: server_test.go 6 테스트 작성 → GREEN: server.go 작성 → REFACTOR | RED: shutdown 4 테스트 → GREEN → REFACTOR | RED: probes 4 E2E → GREEN → REFACTOR |

**권장**: TDD. 본 SPEC의 모든 deliverable은 신규이고 외부 동작 zero(기존 stub은 logger.Info만 호출하며 client 없음) — characterization test가 무의미하다. 단위 테스트가 풍부하므로 TDD가 더 자연스럽다.

---

## 6. Definition of Done

- [ ] S0 deliverable 3건 모두 PR merged + `go build ./...` 성공
- [ ] S1 단위 테스트 6개 통과 (`go test ./cmd/server/... -race`)
- [ ] S2 단위 테스트 4개 통과 + `goleak` 0 goroutine leak
- [ ] S3 E2E 4개 통과 (`go test -tags=integration ./cmd/server/...`)
- [ ] benchmark 결과 NFR 만족: startup p95 < 5s, /ready p95 < 500ms
- [ ] `golangci-lint run ./cmd/server/...` 0 issue
- [ ] `gosec ./cmd/server/...` G112 0건
- [ ] coverage ≥ 85% on `cmd/server/` package
- [ ] @MX tags 추가: `Server.Run` ANCHOR, `Server.shutdown` WARN+REASON, `main()` NOTE, `readinessHandler` ANCHOR
- [ ] acceptance.md 12+ AC 모두 PASS
- [ ] AUTH-002 §5 Exclusion #12 historical 메모 본 SPEC HISTORY 0.1.0에 명시 (이미 명시됨)

---

## 7. Schema Note 재확인

본 plan.md는 spec.md frontmatter의 8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`)를 그대로 참조한다. plan-auditor가 `labels` 또는 `created_at` 같은 변형 필드를 결함으로 제기 시, spec.md "Schema note" 섹션과 본 §7을 출처로 거부한다. (lessons_session_2026_05_14 #1 적용)
