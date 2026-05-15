---
id: SPEC-AX-SERVER-001
version: 0.1.0
status: draft
created: 2026-05-15
updated: 2026-05-15
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.0 (2026-05-15): SPEC-AX-AUTH-002 v0.1.2 GREEN 후속. AUTH-002 §5 Exclusion #12("cmd/server/server.go 부트스트랩 — 본 SPEC 범위 외, 후속 SPEC `SPEC-AX-SERVER-001`")를 정식 후속 SPEC으로 해소한다. 앞서 4개 SPEC(SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 / SPEC-AX-AUTH-002)은 모두 `cmd/server/server.go`가 모든 components를 wire 완료된 상태를 전제했으나 실제 파일은 `Server.Run() {logger.Info("server stub")}`로 끝나는 40-line stub(`grpc_server.go:34~40`)이다. 본 SPEC은 (a) `cmd/server/main.go` 신규 진입점 작성, (b) `cmd/server/server.go` 전면 재작성으로 dual listener(gRPC :50051 + REST :8080) + 명시적 dependency injection(config → store → scheduler → auth → handler → server) + graceful shutdown(SIGTERM/SIGINT, 30s timeout) + health/readiness probes 도입, (c) testcontainers 기반 full-stack E2E로 검증한다. AUTH-002 `chain.go.BuildRESTChain` / `BuildGRPCInterceptorChain` 헬퍼를 한 줄 mount하여 AUTH-001 + AUTH-002 RBAC wiring을 완성한다. CTRL-001 Sprint 7 미완성 gap(operational deployment blocker)을 정식 해소. Composite domain: AX + SERVER(인프라/부팅 sub-domain). Sprint Contract Exclusion 10개 명시. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 / SPEC-AX-AUTH-002와 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 같은 변형 필드는 canonical schema에 없으므로 plan-auditor 결함 제기 시 본 메모와 `plan.md` 마지막 섹션을 출처로 거부한다. (lessons_session_2026_05_14 #1 적용)

---

# SPEC-AX-SERVER-001 — 서버 부트스트랩 & Dual Listener

## 1. 개요

`apps/control-plane/cmd/server/`에 실제 서버 부팅 진입점을 도입한다. SPEC-AX-001 ~ SPEC-AX-AUTH-002 4개 SPEC이 정의한 components(`config.Config`, `store.PgWorkflowStore`, `workflow.StateMachine`, `scheduler.CeleryDispatcher`, `auth.TokenValidator`, `auth.RefreshTokenStore`, `auth.BuildRESTChain`, `auth.BuildGRPCInterceptorChain`, `server.WorkflowService`, `server.RESTHandler`)는 모두 모듈 단위 GREEN이지만, 이를 조립·부팅·종료하는 calling code가 존재하지 않아 운영 배포가 불가능하다. 본 SPEC은 다음을 보장한다:

1. **Dual listener**: gRPC `:50051` + REST `:8080` 동시 listen (errgroup 기반)
2. **명시적 dependency injection**: 6단계 순서(config → store → scheduler → auth → handler → server) 코드로 강제, 각 단계 실패 시 명확한 error wrapping + 부분 cleanup
3. **Graceful shutdown**: SIGTERM/SIGINT 수신 시 진행 중 요청 30s timeout 내 완료, DB/Redis connection close
4. **Health/Readiness probes**: `/health`(liveness, 항상 200), `/ready`(readiness, DB+Redis+JWKS reachable 통과 시 200), gRPC health check
5. **Audit trail**: startup/shutdown 전 단계 `audit_logs` 기록 (SERVER_STARTUP, SERVER_SHUTDOWN_INITIATED, SERVER_SHUTDOWN_COMPLETED)

### 1.1 본 SPEC의 위상

- SPEC-AX-AUTH-002 §5 Exclusion #12에 본 SPEC 후속 처리가 명시되어 있다 → 정식 해소
- SPEC-AX-CTRL-001 Sprint 7(T-AX-006) 미완성 gap을 본 SPEC이 떠안는다
- AUTH-001 `auth_e2e_test.go` 이미 `httptest.NewServer` 기반으로 우회 검증 중 → 본 SPEC GREEN 후 full-stack E2E도 가능해진다 (단, AUTH-001/AUTH-002 SPEC 자체 수정은 본 SPEC 범위 외 — 기존 SKIP unblock은 AUTH-002 책임)

### 1.2 운영 컨텍스트 (Why now)

| 동인 | 출처 | 본 SPEC 대응 |
|------|------|-------------|
| KEPCO E&C 운영 배포 시 실제 바이너리가 부팅되지 않아 prod 환경 작동 불가 | `product.md` §6.1 Go-Live + `tech.md` §9 컨테이너 배포 | REQ-SERVER-001 dual listener |
| 4개 SPEC 모듈 GREEN이지만 통합 진입점 부재로 PR로 합쳐도 동작하지 않음 | 코드베이스 cmd/server/server.go:34~40 stub | REQ-SERVER-002 dependency wiring |
| K8s liveness/readiness probe 필수 (Helm 배포 시) | `tech.md` §9 K8s | REQ-SERVER-004 health/ready |
| SIGTERM 처리 미흡 시 in-flight 요청 손실 (PISA audit_logs 중복 가능) | AUTH-001 §3.4 PIPA 추적성 | REQ-SERVER-003 graceful shutdown |
| AUTH-002 §6 의존성 "SPEC-AX-SERVER-001 사후 책임" 명시 | AUTH-002 §6 L239 | REQ-SERVER-002 chain.go mount |

### 1.3 Composite Domain

- 1차 도메인: `AX` (iroum-ax 프로젝트 전체)
- 2차 도메인: `SERVER` (서버 부팅/인프라 sub-domain — 신규 sub-domain)
- SPEC ID: `SPEC-AX-SERVER-001` (도메인 카디널리티 2, plan.md 권장 범위 내)

### 1.4 한국 공공 도메인 6 제약 영향 평가 (lessons #8 적용)

- 망분리: 본 SPEC은 외부 API 호출 0건 (OIDC JWKS만 inbound 응답 캐시, 부팅 시 1회 fetch). KEPCO 내부망에서 Keycloak도 internal network. 영향 없음.
- PIPA audit_logs: startup/shutdown 모두 audit 기록 (REQ-SERVER-UBI-001) — 강화.
- 합니다체: 부팅 로그는 zap structured logging(영문 key + 영문 description). 합니다체 미해당.
- HWP / 한자한글 / 등급 시뮬레이션: 본 SPEC 무관 (인프라 계층).

---

## 2. 영향받는 파일 (Affected Files)

`structure.md` §2 디렉토리 트리를 따른다. 본 SPEC은 **신규 파일 3개**(`apps/control-plane/cmd/server/main.go`, `apps/control-plane/cmd/server/probes.go`, `apps/control-plane/cmd/server/server_test.go`)를 추가하고, **기존 파일 1개**(`apps/control-plane/cmd/server/server.go`)를 전면 재작성한다. Delta 마커는 Run phase에서 정확한 라인 단위로 결정.

### Scope Boundary

본 SPEC은 **부팅·종료·헬스체크 코드 그 자체**(`main.go`, `server.go`, `probes.go`)와 그에 대한 **testcontainers 기반 E2E**(`server_test.go`)로 한정한다. 비즈니스 핸들러 코드(`WorkflowService`, `RESTHandler`, `RESTAuthzMiddleware`, `UnaryAuthzInterceptor`), RBAC 매트릭스(`rbac.go`), TokenValidator 자체 로직, store/scheduler 내부 구현 등은 본 SPEC 범위 외이다. 본 SPEC은 그들을 조립·호출하는 calling code만 작성한다.

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | 모듈 | 신규/수정 |
|------|------|------|---------|
| `apps/control-plane/cmd/server/main.go` | OS 진입점. `os.Args` 파싱(현재 없음, 모두 env), 로거 초기화, `config.Load()` 호출, `Server` 인스턴스 생성, `server.Run(ctx)` 호출, exit code 반환 | REQ-SERVER-UBI-001 + REQ-SERVER-002 | 신규 |
| `apps/control-plane/cmd/server/server.go` | `Server` struct + `New()` + `Run(ctx)` + `Shutdown(ctx)` 전면 재작성. dependency wiring 6단계 + dual listener errgroup + graceful shutdown signal handling | REQ-SERVER-001 ~ 003 | 전면 재작성 |
| `apps/control-plane/cmd/server/probes.go` | `/health` / `/ready` HTTP 핸들러 + gRPC health.v1.Health 구현 (`Check` method). DB ping + Redis ping + JWKS reachable 검증 헬퍼 | REQ-SERVER-004 | 신규 |
| `apps/control-plane/cmd/server/server_test.go` | 단위 테스트: dependency wiring 순서 검증, port conflict 시 graceful error, DB 실패 시 startup abort, SIGTERM 처리 race | REQ-SERVER-001 ~ 004 | 신규 |
| `apps/control-plane/internal/server/grpc_server.go` | 본 SPEC 범위 외. 본 SPEC은 `WorkflowService`를 호출자로 사용만 함 | - | 미수정 |
| `apps/control-plane/internal/server/rest_handler.go` | 본 SPEC 범위 외. 본 SPEC은 `RESTHandler.Mux()`를 호출하여 `chain.go.BuildRESTChain`으로 wrap만 함 | - | 미수정 |
| `apps/control-plane/internal/auth/chain.go` | 본 SPEC 범위 외. AUTH-002 GREEN 산출물을 신뢰. 본 SPEC은 `BuildRESTChain` / `BuildGRPCInterceptorChain`을 호출만 함 | - | 미수정 |
| `apps/control-plane/internal/config/config.go` | 본 SPEC 범위 외. `config.Load()` 그대로 사용. 단, REQ-SERVER-002에서 신규 env 2개(`SHUTDOWN_TIMEOUT_SECONDS`, `READY_PROBE_TIMEOUT_SECONDS`)는 본 SPEC iter 1 후 별도 chore PR로 config.go에 추가됨(본 SPEC plan.md S1에서 정확한 시점 명시) | - | 사전 의존 (S1 deliverable) |

### 2.2 E2E 테스트 (`apps/control-plane/internal/server/`)

| 경로 | 책임 | 모듈 |
|------|------|------|
| `apps/control-plane/cmd/server/server_test.go` | testcontainers 기반 full-stack E2E: postgres + redis + keycloak(or static JWKS HTTP server) + actual `server.Run()` 호출 → REST POST /workflows + gRPC CreateWorkflow + /ready + SIGTERM 시나리오 | REQ-SERVER-001 ~ 004 |

### 2.3 Python Pipelines (`pipelines/`)

본 SPEC은 Go control-plane 부팅에 한정한다. FastAPI 측 부팅(`uvicorn` + lifecycle hook)은 후속 SPEC(`SPEC-AX-SERVER-PY-001`)으로 분리.

### 2.4 Shared (`pkg/`, `schemas/`)

| 경로 | 책임 | 신규/수정 |
|------|------|---------|
| `schemas/openapi/openapi.yaml` | `/ready` endpoint 추가 (200 ready / 503 not_ready). `/health`는 이미 명세 존재 (CTRL-001) | 수정 (소규모) |

### 2.5 Deployments / Database

본 SPEC은 schema 변경 없음. 단, K8s manifest 측에서 livenessProbe path를 `/health`, readinessProbe path를 `/ready`로 설정해야 하며, 이는 인프라 PR로 별도 처리(본 SPEC scope 외 — Helm chart 변경은 후속 chore).

---

## 3. EARS 요구사항

EARS 5개 패턴(Ubiquitous / Event-driven / State-driven / Optional / Unwanted) 모두 본 SPEC 내 포함.

### 3.1 Ubiquitous Requirements (시스템 전반 불변 조건)

**REQ-SERVER-UBI-001 (부팅·종료 추적성 + 의존성 순서 강제 + Graceful guarantee)**

본 UBI는 3개 sub-clause를 가지며, 각 sub-clause는 acceptance.md에서 dedicated AC를 가진다(lessons #2 적용).

- **REQ-SERVER-UBI-001-a (모든 startup/shutdown 단계 audit)**: The system SHALL record every server lifecycle transition to `audit_logs` with action type `SERVER_STARTUP` (after all listeners bound successfully), `SERVER_SHUTDOWN_INITIATED` (signal received), `SERVER_SHUTDOWN_COMPLETED` (all listeners drained + connections closed). 각 audit row는 `details.grpc_addr`, `details.rest_addr`, `details.uptime_seconds`(shutdown 시), `details.exit_reason`(signal / fatal_error) 필드를 포함한다. `userID`는 `system` 고정(인간 사용자가 아닌 시스템 이벤트).
- **REQ-SERVER-UBI-001-b (Dependency 순서 강제)**: WHILE server is in startup phase, the system SHALL initialize dependencies in this exact order: (a) `config.Load()`, (b) `logger.New()`, (c) `store.NewPgWorkflowStore()` + `Ping()`, (d) `redis.NewClient()` + `Ping()`, (e) `auth.NewOIDCClient()` + JWKS warm-up fetch, (f) `auth.NewTokenValidator()` + `auth.NewRefreshTokenStore()`, (g) `workflow.NewStateMachine()`, (h) `scheduler.NewCeleryDispatcher()`, (i) `server.NewWorkflowService()` + `server.NewRESTHandler()`, (j) `auth.BuildRESTChain()` + `auth.BuildGRPCInterceptorChain()`, (k) gRPC `Listen` + REST `Listen` (errgroup). 각 단계는 직전 단계의 성공을 전제하며, 단계 (a)~(j) 중 어느 하나라도 실패 시 후속 단계 진입 금지 + 이미 완료된 단계의 cleanup(역순) 수행.
- **REQ-SERVER-UBI-001-c (Graceful guarantee — in-flight 요청 완료)**: WHILE server is in shutdown phase, the system SHALL wait for in-flight requests to complete up to `SHUTDOWN_TIMEOUT_SECONDS` (default: 30s) before force-killing. Timeout 초과 시 `SERVER_SHUTDOWN_COMPLETED` audit row에 `details.exit_reason=force_kill_timeout` 기록. AuthEnabled=false 환경에서도 동일하게 적용 (graceful shutdown은 인증과 무관한 인프라 invariant).

### 3.2 REQ-SERVER-001 — Dual Listener

#### Ubiquitous

- **REQ-SERVER-001-S1**: The system SHALL listen on two TCP ports simultaneously: gRPC on `cfg.GRPCAddr` (default `:50051`, configurable via `GRPC_ADDR` env) and REST/HTTP on `cfg.RESTAddr` (default `:8080`, configurable via `REST_ADDR` env). 두 listener는 goroutine으로 분리되며 errgroup으로 묶여 어느 하나가 fatal error 반환 시 다른 하나도 graceful shutdown 절차로 종료한다.

#### Event-driven

- **REQ-SERVER-001-E1 (gRPC listener bind)**: WHEN `Server.Run(ctx)` enters listener phase, THEN the system SHALL call `net.Listen("tcp", cfg.GRPCAddr)`, create `grpc.NewServer(authChainOption, healthServiceRegistration)`, register `WorkflowService` via `proto.RegisterWorkflowServiceServer(s, workflowSvc)` + `grpc_health_v1.RegisterHealthServer(s, healthServer)`, AND invoke `grpcServer.Serve(listener)` in a goroutine within `errgroup.Group`.

- **REQ-SERVER-001-E2 (REST listener bind)**: WHEN `Server.Run(ctx)` enters listener phase (after gRPC bind succeeds), THEN the system SHALL create `http.Server{Addr: cfg.RESTAddr, Handler: chain.BuildRESTChain(restHandler.Mux(), validator, recorder, cfg.AuthEnabled), ReadHeaderTimeout: 10s, ReadTimeout: 30s, WriteTimeout: 30s, IdleTimeout: 60s}` and invoke `httpServer.ListenAndServe()` in a goroutine within the same `errgroup.Group`. `ReadHeaderTimeout` 명시는 G112 Slowloris 방어.

#### Unwanted

- **REQ-SERVER-001-U1 (Port conflict graceful error)**: IF `net.Listen` returns `syscall.EADDRINUSE` (포트 점유) OR `syscall.EACCES` (권한 부족) for either gRPC or REST address, THEN the system SHALL return an error from `Server.Run()` wrapped as `fmt.Errorf("listener bind failed on %s: %w", addr, err)`, NOT proceed with the other listener, log structured error with `zap.String("phase", "listener_bind")`, AND NOT insert `SERVER_STARTUP` audit row (실패한 startup은 audit row 미생성으로 false positive 방지).

- **REQ-SERVER-001-U2 (One listener dies → full shutdown)**: IF either `grpcServer.Serve()` OR `httpServer.ListenAndServe()` returns a non-nil error after successful startup (excluding `http.ErrServerClosed` / `grpc.ErrServerStopped`), THEN the errgroup SHALL propagate the error, cancel the shared context, AND trigger graceful shutdown of the surviving listener via `httpServer.Shutdown(shutdownCtx)` / `grpcServer.GracefulStop()`. 즉, 비대칭 부분 가동 상태는 금지(한쪽만 살아있으면 routing 오류 + RBAC bypass 위험).

### 3.3 REQ-SERVER-002 — Dependency Wiring

#### Event-driven

- **REQ-SERVER-002-E1 (Sequential init with explicit error wrapping)**: WHEN `Server.New(cfg, logger)` is called, THEN the system SHALL initialize dependencies in the order defined by REQ-SERVER-UBI-001-b, AND each initialization step that returns an error SHALL be wrapped as `fmt.Errorf("init step %s failed: %w", stepName, err)` with `stepName` ∈ `{"pg_store", "redis_client", "oidc_client", "jwks_fetch", "token_validator", "refresh_token_store", "state_machine", "celery_dispatcher", "workflow_service", "rest_handler", "auth_chain"}`. 각 단계 실패 시 후속 단계 진입 금지.

- **REQ-SERVER-002-E2 (Partial cleanup on failure)**: WHEN any init step (a)~(j) in REQ-SERVER-UBI-001-b returns an error, THEN the system SHALL invoke cleanup of previously-initialized resources in REVERSE order: gRPC server stop, HTTP server close (해당 시점에는 미생성이므로 no-op), `celeryDispatcher.Close()`, `tokenValidator.Close()`, `oidcClient.Close()`, `redisClient.Close()`, `pgStore.Close()`. Cleanup 자체의 error는 logging만 하고 무시(이미 부팅 실패 상태에서 cleanup 실패 추가 wrap은 root cause를 가림).

- **REQ-SERVER-002-E3 (Auth chain composition — single line mount)**: WHEN dependency wiring reaches step (j), THEN the system SHALL invoke `auth.BuildRESTChain(restHandler.Mux(), tokenValidator, auditRecorder, cfg.AuthEnabled)` to obtain the wrapped REST handler AND `auth.BuildGRPCInterceptorChain(tokenValidator, auditRecorder, cfg.AuthEnabled)` to obtain the gRPC `ServerOption`. 두 헬퍼는 AUTH-002 GREEN 산출물이며 본 SPEC은 호출만 한다 (체인 순서·white-list bypass·default-deny 등 모든 wiring 정책은 AUTH-002 책임).

#### Unwanted

- **REQ-SERVER-002-U1 (DB connection failure aborts startup)**: IF step (c) `pgStore.Ping()` returns an error OR step (d) `redisClient.Ping()` returns an error, THEN the system SHALL abort startup with error wrapping per REQ-SERVER-002-E1, SHALL NOT proceed to listener bind, AND SHALL NOT insert `SERVER_STARTUP` audit row. DB 미가용 상태에서 server가 부팅되면 모든 요청이 503을 반환하므로 K8s가 unhealthy pod로 인식하여 restart 유도하는 것이 안전.

- **REQ-SERVER-002-U2 (JWKS fetch failure aborts startup when AuthEnabled=true)**: IF `cfg.AuthEnabled=true` AND step (e) JWKS warm-up fetch returns an error (network timeout / DNS failure / 5xx), THEN the system SHALL abort startup with error wrapping. AuthEnabled=false 환경에서는 JWKS fetch 자체를 skip(REQ-SERVER-UBI-001-b 단계 (e)에서 `cfg.AuthEnabled=false`이면 OIDC client 생성도 skip).

### 3.4 REQ-SERVER-003 — Graceful Shutdown

#### Event-driven

- **REQ-SERVER-003-E1 (Signal handling)**: WHEN the process receives `syscall.SIGTERM` OR `syscall.SIGINT`, THEN `signal.NotifyContext(parentCtx, syscall.SIGTERM, syscall.SIGINT)` shall cancel the shared `ctx`, the errgroup `g.Wait()` shall observe `ctx.Done()`, AND the system SHALL invoke shutdown in this order: (i) `httpServer.Shutdown(shutdownCtx)` with `shutdownCtx` deadline `cfg.ShutdownTimeoutSeconds` (default 30s), (ii) `grpcServer.GracefulStop()` (no timeout API in grpc-go; relies on shutdownCtx via goroutine race with `grpcServer.Stop()`), (iii) `redisClient.Close()`, (iv) `pgStore.Close()`, (v) insert `SERVER_SHUTDOWN_COMPLETED` audit row, (vi) return from `Server.Run()`.

- **REQ-SERVER-003-E2 (In-flight request completion)**: WHEN `httpServer.Shutdown(shutdownCtx)` is invoked, THEN the system SHALL allow currently-processing HTTP requests to complete naturally up to the shutdownCtx deadline. Idle connections are closed immediately. New incoming connections are refused with TCP RST. gRPC side: `grpcServer.GracefulStop()` blocks until all pending RPCs complete, but is raced with `time.AfterFunc(shutdownTimeout, grpcServer.Stop)` to enforce the timeout.

#### State-driven

- **REQ-SERVER-003-S1 (Shutdown idempotency)**: WHILE `Server.shutdown()` has already been invoked once, the system SHALL be idempotent — subsequent signals (SIGINT 두 번 등) are observed but do not re-enter shutdown procedure. 단, 두 번째 신호가 도착하면 즉시 `grpcServer.Stop()` + `httpServer.Close()` 강제 종료 호출 (force-kill escape hatch). 첫 SIGTERM 30초 대기 중 두 번째 SIGINT가 오면 사용자가 강제 종료를 원하는 의도로 간주.

#### Unwanted

- **REQ-SERVER-003-U1 (Shutdown timeout force-kill)**: IF `httpServer.Shutdown(shutdownCtx)` returns `context.DeadlineExceeded` after `cfg.ShutdownTimeoutSeconds`, THEN the system SHALL call `httpServer.Close()` for immediate force-close, AND log structured warning `zap.String("phase", "shutdown"), zap.String("reason", "timeout_force_kill")`, AND the `SERVER_SHUTDOWN_COMPLETED` audit row SHALL include `details.exit_reason="force_kill_timeout"`. 동일 시점에 gRPC side는 `grpcServer.Stop()`(non-graceful)가 호출됨.

### 3.5 REQ-SERVER-004 — Health & Readiness Probes

#### Event-driven

- **REQ-SERVER-004-E1 (Liveness probe)**: WHEN an HTTP request `GET /health` arrives, THEN the REST handler SHALL return HTTP 200 with body `{"status":"ok","service":"iroum-ax-control-plane","version":"<build_version>"}` regardless of dependency state. 본 endpoint는 K8s livenessProbe 용으로 "프로세스가 살아있고 HTTP listener가 동작 중인가"만 검증한다. 본 endpoint는 AUTH-002 §3.2 REST mapping table에서 `bypass`로 명시되어 인증/인가를 통과하지 않는다.

- **REQ-SERVER-004-E2 (Readiness probe)**: WHEN an HTTP request `GET /ready` arrives, THEN the REST handler SHALL execute three checks within `cfg.ReadyProbeTimeoutSeconds` (default 5s): (i) `pgStore.Ping(ctx)`, (ii) `redisClient.Ping(ctx).Err()`, (iii) `cfg.AuthEnabled=true`인 경우만 `oidcClient.JWKSReachable(ctx)` (cached JWKS의 fetch 가능 여부). 세 검사 모두 nil error 반환 시 HTTP 200 + body `{"status":"ready","checks":{"postgres":"ok","redis":"ok","oidc":"ok"}}`. 하나라도 실패 시 HTTP 503 + body `{"status":"not_ready","checks":{...}}` (실패한 check만 `"failed: <error message>"`로 표시). 본 endpoint도 AUTH-002 매핑 테이블에서 `bypass`로 추가되어야 함(AUTH-002 후속 chore, 본 SPEC plan.md S2 deliverable에서 명시).

- **REQ-SERVER-004-E3 (gRPC health check)**: WHEN a gRPC RPC `/grpc.health.v1.Health/Check` arrives, THEN the gRPC `HealthServer` SHALL execute the same three checks as REQ-SERVER-004-E2 AND return `&grpc_health_v1.HealthCheckResponse{Status: SERVING}` on all-pass OR `Status: NOT_SERVING` on any failure. gRPC health check는 AUTH-002 §3.2 gRPC mapping table에서 이미 `bypass`로 정의되어 있어 본 SPEC 범위 외 wiring 변경 불필요.

#### State-driven

- **REQ-SERVER-004-S1 (Probe during shutdown)**: WHILE server is in shutdown phase (after SIGTERM received), `/ready` SHALL return HTTP 503 with `{"status":"shutting_down"}` regardless of dependency state. `/health` continues to return 200 until the HTTP listener is closed (K8s가 readinessProbe 503 보고 트래픽을 끊은 후에야 liveness가 종료되는 정상 흐름).

#### Unwanted

- **REQ-SERVER-004-U1 (Probe timeout)**: IF any of the three ready-check operations exceeds `cfg.ReadyProbeTimeoutSeconds`, THEN the corresponding check SHALL be marked as `"failed: timeout exceeded <N>s"` in the response body, AND the overall response SHALL be HTTP 503. 본 timeout은 K8s readinessProbe의 `timeoutSeconds`(default 1s)보다 작아야 하나, 본 SPEC은 application timeout 5s를 사용하고 K8s probe timeout은 10s로 권장 (helm chart 책임).

---

## 4. 비기능 요구사항

| 영역 | 요구사항 | 출처 |
|------|----------|------|
| 성능 — Startup latency | `Server.New()` ~ 첫 `/health` 200 응답까지 p95 < 5s (testcontainers 환경; prod K8s에서는 image pull 제외 5s) | K8s `initialDelaySeconds` 권장 |
| 성능 — Shutdown latency | SIGTERM 수신 ~ process exit 까지 p95 < 30s (default timeout); in-flight 요청 없으면 p95 < 1s | REQ-SERVER-003-E1 |
| 성능 — /health 응답 | p99 < 5ms (dependency 무관 정적 응답) | K8s probe 효율 |
| 성능 — /ready 응답 | p95 < 500ms (3 ping 병렬 실행); p99 < 5s | REQ-SERVER-004-E2 |
| 보안 — Port binding | gRPC :50051 / REST :8080은 모두 unprivileged port (>= 1024); 80/443 binding은 K8s Service / Ingress 책임 | TLS termination Exclusion |
| 보안 — Slowloris 방어 | `http.Server.ReadHeaderTimeout=10s` 명시 (G112 lint) | gosec G112 |
| 보안 — Audit completeness | startup/shutdown 양 끝 단계 100% audit row 생성 (REQ-SERVER-UBI-001-a) | PIPA |
| 백워드 호환성 | AuthEnabled=false 시 SPEC-AX-001 / CTRL-001 모든 AC가 unchanged 통과 | regression invariant |
| 망분리 정합 | 외부 API 호출 0건 (JWKS는 internal Keycloak만) | `tech.md` §9.1 |
| 테스트 커버리지 | >= 85% (`quality.yaml` test_coverage_target) | `quality.yaml` |
| 개발 방법론 | DDD 또는 TDD (quality.yaml development_mode 따름; 본 SPEC은 brownfield — testcontainers 무거우므로 DDD 친화) | `quality.yaml` |

---

## 5. Exclusions (What NOT to Build)

본 SPEC에서 의도적으로 제외한 범위. 후속 SPEC에서 다룬다 (target ≥ 7 충족: 10개 명시).

1. **다중 인스턴스 부하 분산 (replicaCount >= 2)** — Helm chart에서 `replicaCount: 3` 설정 시 여러 pod가 부팅되어 K8s Service의 round-robin이 트래픽을 분배하지만, 본 SPEC은 단일 인스턴스 부팅·종료 정합성만 보장한다. session affinity, leader election, Redis 분산 lock 등은 후속 SPEC `SPEC-AX-HA-001`(High Availability).
2. **Hot reload / Dynamic config** — 운영 중 환경 변수 변경 후 재부팅 없이 반영. 본 SPEC은 `config.Load()` 1회 호출 후 immutable. SIGHUP signal로 config reload는 후속 SPEC.
3. **TLS termination (HTTPS / mTLS)** — gRPC `:50051` 및 REST `:8080`은 plain HTTP. TLS 종료는 K8s Ingress(REST) / Istio sidecar(gRPC mTLS) 레이어 책임. `tech.md` §9.1 망분리 환경에서 외부 노출 ingress가 TLS termination 담당. 본 SPEC은 HTTP/2 cleartext + HTTP/1.1 plaintext만 처리.
4. **Distributed tracing (OpenTelemetry)** — Trace span propagation, OTLP exporter, Jaeger/Tempo 연동. AUTH-001 / CTRL-001에서 audit_logs는 있지만 분산 trace는 미도입. 후속 SPEC `SPEC-AX-OBS-001`(Observability + Monitoring).
5. **Prometheus `/metrics` endpoint** — AUTH-002 v0.1.2 Exclusion #13에서 분리된 `/metrics`는 본 SPEC도 미해소. 후속 SPEC `SPEC-AX-OBS-001` 또는 `SPEC-AX-METRICS-001`.
6. **다중 OIDC provider** — 현재 단일 Keycloak realm (`iroum-ax`). 복수 IdP(Azure AD + Keycloak 동시) 지원은 후속 SPEC `SPEC-AX-AUTH-MULTI-IDP-001`. AUTH-001 §5 Exclusion 재인용.
7. **API rate limiting** — 사용자당/IP당 RPS 제한, token bucket 알고리즘. 본 SPEC은 시계열 throttling 없음. 후속 SPEC `SPEC-AX-RATELIMIT-001`. 임시 대응은 K8s Ingress NGINX `nginx.ingress.kubernetes.io/limit-rps` annotation으로 가능.
8. **WebSocket / Server-Sent Events (SSE)** — 현재 모든 RPC는 unary (gRPC unary + REST request-response). long-poll / streaming / pub-sub는 후속 SPEC. 본 SPEC `http.Server` 설정의 `IdleTimeout: 60s`는 long connection이 60초 후 강제 종료됨.
9. **Server-side caching (in-process or Redis-backed)** — 본 SPEC은 per-request 처리만. 응답 caching, 세션 caching 등 후속 SPEC. (RBAC `Authorize()` 결과 caching은 AUTH-002 Exclusion #3과 동일.)
10. **Custom error pages (HTML)** — 404/500 등 에러 응답은 모두 JSON body (`{"error":{"code":"...","message":"..."}}` 패턴, AUTH-002 §3.3 REQ-AUTH2-002-U1과 일치). HTML 페이지 렌더링 없음. 본 control-plane은 백엔드 API만 제공하며 프론트엔드 자산은 별도 서비스.

---

## 6. 의존성 및 전제

- **SPEC-AX-001 GREEN 가정**: `audit_logs` schema + `cli-anonymous` 폴백 + `audit.Recorder` 인터페이스. 본 SPEC은 `SERVER_STARTUP` / `SERVER_SHUTDOWN_INITIATED` / `SERVER_SHUTDOWN_COMPLETED` 3개 신규 action을 `audit/actions.go`에 추가해야 함 (S1 deliverable 사전 의존 — 본 SPEC plan.md S1에서 명시).
- **SPEC-AX-CTRL-001 GREEN 가정 (부분)**: `WorkflowService` + `RESTHandler` + `TxCoordinator` + `audit.Recorder` 모듈 GREEN. 단, Sprint 7 (`cmd/server/server.go` 실제 부팅)은 본 SPEC이 해소한다 — CTRL-001 Sprint 7 미완성 gap을 본 SPEC이 흡수.
- **SPEC-AX-AUTH-001 GREEN 가정**: `rbac.go` 매트릭스 + `TokenValidator` + `RefreshTokenStore` + `OIDCClient` + `UnaryServerInterceptor` + `RESTMiddleware`. 본 SPEC은 이들을 wiring 단계 (e)~(f)에서 호출만 함.
- **SPEC-AX-AUTH-002 v0.1.2 GREEN 가정**: `chain.go.BuildRESTChain` / `BuildGRPCInterceptorChain` 헬퍼. 본 SPEC은 step (j)에서 두 함수를 호출하여 한 줄로 미들웨어 chain mount. AUTH-002 §6 "SPEC-AX-SERVER-001 사후 책임"이 본 SPEC GREEN으로 해소됨.
- **AUTH-002 §5 Exclusion #12 unblock**: 본 SPEC GREEN 후 AUTH-002 Exclusion #12는 historical only(이미 처리됨). AUTH-002 SPEC 파일 자체 수정은 본 SPEC 범위 외 (별도 chore commit으로 `AUTH-002 Exclusion #12: RESOLVED by SPEC-AX-SERVER-001 v0.1.0` 추가 가능, 또는 미수정 — 본 SPEC은 unblock fact만 보장).
- **Go 의존성**: 신규 의존성 없음 (`google.golang.org/grpc/health` 이미 transitively 포함, `golang.org/x/sync/errgroup` 신규 require 필요 — go.sum에 자동 추가, S1 deliverable).
- **Database**: schema 변경 없음. `audit/actions.go`에 3개 const 추가만.
- **Helm chart**: `livenessProbe.path=/health`, `readinessProbe.path=/ready` 설정은 본 SPEC scope 외 (인프라 chore PR).
- **MX tags**:
  - `cmd/server/main.go` `main()` 함수에 `@MX:NOTE`(엔트리 포인트 명시).
  - `cmd/server/server.go` `Server.Run(ctx)` 함수에 `@MX:ANCHOR`(fan_in 예상 ≥ 2: main.go + server_test.go) + `@MX:REASON: dependency wiring + dual listener의 단일 진입점`.
  - `Server.shutdown(ctx)` 함수에 `@MX:WARN` + `@MX:REASON: goroutine 종료 race, idempotency 강제, force-kill 분기 — 변경 시 SIGTERM 처리 깨질 위험`.
  - `cmd/server/probes.go` `readyHandler` 함수에 `@MX:ANCHOR`(fan_in: REST GET /ready + gRPC Health.Check, 헬퍼 공유) + `@MX:REASON: liveness/readiness 검사 로직의 단일 출처`.
  - 모두 `code_comments: ko` 적용 (mx-tag-protocol.md 적용).

---

## 7. Out of Scope (참고)

본 SPEC을 받은 구현자가 혼동할 수 있는 인접 영역:

- AUTH-001 `auth_e2e_test.go` SKIP 마커 unblock: AUTH-002 책임 (이미 v0.1.2에서 처리). 본 SPEC GREEN 후 추가 E2E 활성화는 별도 chore.
- DELETE /api/v1/workflows/{id} REST 핸들러: 후속 SPEC `SPEC-AX-WF-DELETE-001`. 본 SPEC은 mux mount만 보장하며 핸들러 자체는 변경 없음.
- Python FastAPI 부팅(`pipelines/api/main.py`의 `uvicorn` lifecycle): 후속 SPEC `SPEC-AX-SERVER-PY-001`.
- gRPC streaming RPC 부팅: 본 SPEC은 unary만 지원 (현재 streaming endpoint 없음). 도입 시 후속 SPEC.
- Helm chart variable / K8s manifest: 별도 chore PR. 본 SPEC은 `/health` + `/ready` HTTP endpoint를 제공할 뿐, K8s probe path 설정은 인프라 책임.

---

## 8. 검증 방법 요약 (상세는 `acceptance.md`)

- 단위 테스트: `apps/control-plane/cmd/server/server_test.go` — dependency wiring 순서(`TestServerNew_DependencyOrder`), port conflict(`TestServerRun_PortConflict`), DB 실패 시 abort(`TestServerNew_PgPingFailure`), SIGTERM idempotency(`TestServerShutdown_DoubleSignal`), force-kill timeout(`TestServerShutdown_TimeoutForceKill`).
- 통합 테스트: testcontainers 기반 full-stack E2E — postgres + redis + static JWKS HTTP server → 실제 `server.Run()` 호출 → REST POST /api/v1/workflows + gRPC `WorkflowService.CreateWorkflow` + `GET /ready` 200 + SIGTERM 전송 → audit row 3개 검증.
- 백워드 호환성 테스트: AuthEnabled=false 환경에서 모든 startup 단계가 진행되며 OIDC step (e) skip 검증.
- 성능 측정: `go test -bench=BenchmarkServerStartup` (startup p95 < 5s), `go test -bench=BenchmarkReadyProbe` (p95 < 500ms).
- 보안 검증: `gosec ./cmd/server/...` G112 ReadHeaderTimeout 미명시 경고 없음.

상세 Given/When/Then 시나리오는 `acceptance.md`를 참조한다.
