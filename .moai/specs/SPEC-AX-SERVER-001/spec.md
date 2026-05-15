---
id: SPEC-AX-SERVER-001
version: 0.1.2
status: draft
created: 2026-05-15
updated: 2026-05-15
author: ircp
priority: high
issue_number: 0
---

# HISTORY

- 0.1.2 (2026-05-15): plan-auditor iteration 2 FAIL(0.78, 7/8 resolved + 3 new: D9 Major / D11 Minor / D12 Minor) 대응 — iteration 3(FINAL). **D9**(redis adapter, Major): iteration 2 spec.md §2.0:L79가 raw `*redis.Client`가 `scheduler.RedisClient` 인터페이스를 직접 충족한다고 잘못 주장. 실제로 go-redis v9 raw client의 `RPush`는 `*redis.IntCmd`, `Ping`은 `*redis.StatusCmd`를 반환하여 `scheduler.RedisClient`(`RPush(ctx,key,...) (int64,error)` + `Ping(ctx) error`, dispatcher.go:24~29)를 충족하지 못함 → 어댑터 경유 필수. 코드베이스에 이미 `internal/server/e2e_test.go:199~209`에 test-only `goRedisAdapter{client *redis.Client}`(`RPush→.Result()`, `Ping→.Err()`) 존재. **해소**: §2.0 redis row 정정, wiring 단계 (h)를 `scheduler.NewRedisClientAdapter(redisClient)` 경유로 변경, S0 deliverable에 "goRedisAdapter를 e2e_test.go에서 `internal/scheduler/redis_adapter.go` production code로 promote" task 추가, §2.1 affected files에 신규 파일 명시, §6 의존성/database 정정. 어댑터 생성은 infallible이므로 captured-slice는 15-element 유지(추적 항목 아님). **D11**(JWKSCache.Reachable mu.RLock, Minor): `cacheAge()`(jwks_cache.go:212)가 "호출자가 `mu.RLock` 보유" 동시성 계약을 가지므로 §2.1 및 plan.md S0 deliverable에 `Reachable`이 `c.mu.RLock()` 획득 후 `lastSuccessAt`/`cacheAge()`를 읽어야 함을 명시(concurrent-safe). **D12**(acceptance.md §8 stale 5-pattern, Minor): acceptance.md:L390 §8이 "EARS 5-pattern"을 참조(iteration 2에서 spec.md:L129는 4-pattern으로 정정됨, Optional 미해당) → acceptance.md §8을 "EARS 4-pattern (Optional 미해당 — 서버 부팅은 조건부 기능 없음)"으로 정정, 관련 AC 정합. Schema canonical 8 fields 보존, EARS 4-pattern(Optional 미해당) 보존. (작성자: ircp)
- 0.1.1 (2026-05-15): plan-auditor iteration 1 FAIL(0.62, 8 defects: Critical 4 / Major 2 / Minor 2) 대응. **근본 원인**: spec.md가 존재하지 않는 phantom API에 wiring spine을 의존시켰음. iteration 2에서 9개 실제 source file을 정독하여 wiring 시퀀스를 실제 생성자/메서드로 전면 재작성. **D1**(pgStore.Ping 미존재) → S0 deliverable로 `PgWorkflowStore.Ping(ctx) error` 추가(scope-added, affected files에 pg_store.go 포함). **D3**(auth 생성자 이름 오류) → `auth.NewTokenValidator()`→실제 `auth.New(ctx, oidcIssuer, audience, opts...)`, `auth.NewRefreshTokenStore()`→실제 `auth.NewRedisRefreshStore(addr)` + `auth.NewRefreshService(...)`로 정정. **D4**(oidc.JWKSReachable/Close 미존재) → JWKS readiness는 S0 deliverable `JWKSCache.Reachable(ctx) bool`로 해소(jwks_cache.go 추가); OIDCClient는 stateless이므로 Close 불필요 → cleanup에서 oidcClient 제외. **D2**(redis client 미정의) → 기존 e2e_test가 사용 중인 `github.com/redis/go-redis/v9` `redis.NewClient(&redis.Options{Addr})` 직접 생성(`*redis.Client`는 `scheduler.RedisClient` 인터페이스 충족, 실제 `Ping(ctx).Err()` / `Close() error` 보유). **D5**(celeryDispatcher.Close/tokenValidator.Close 미존재) → 두 타입 모두 Close 없음 → reverse cleanup에서 제거(redis client `Close()`만 수행). **D6**(TxCoordinator 누락) → init order에 `audit.NewRecorder` + `workflow.NewTxCoordinator`를 `NewStateMachine` 이전 단계로 추가. **D7**(EARS 5패턴 주장 부정확) → 실제 4패턴(Optional 미해당)으로 정정. **D8**(narrative vs enum scope 불일치) → config/logger infallible 명시 + stepName enum을 pg_store부터 시작함을 narrative와 정렬. stepName enum + AC captured-slice를 실제 심볼로 재작성. Schema canonical 8 fields 보존, EARS 5 type 분류 유지. (작성자: ircp)
- 0.1.0 (2026-05-15): SPEC-AX-AUTH-002 v0.1.2 GREEN 후속. AUTH-002 §5 Exclusion #12("cmd/server/server.go 부트스트랩 — 본 SPEC 범위 외, 후속 SPEC `SPEC-AX-SERVER-001`")를 정식 후속 SPEC으로 해소한다. 앞서 4개 SPEC(SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 / SPEC-AX-AUTH-002)은 모두 `cmd/server/server.go`가 모든 components를 wire 완료된 상태를 전제했으나 실제 파일은 `Server.Run() {logger.Info("server stub")}`로 끝나는 40-line stub(`grpc_server.go:34~40`)이다. 본 SPEC은 (a) `cmd/server/main.go` 신규 진입점 작성, (b) `cmd/server/server.go` 전면 재작성으로 dual listener(gRPC :50051 + REST :8080) + 명시적 dependency injection(config → store → scheduler → auth → handler → server) + graceful shutdown(SIGTERM/SIGINT, 30s timeout) + health/readiness probes 도입, (c) testcontainers 기반 full-stack E2E로 검증한다. AUTH-002 `chain.go.BuildRESTChain` / `BuildGRPCInterceptorChain` 헬퍼를 한 줄 mount하여 AUTH-001 + AUTH-002 RBAC wiring을 완성한다. CTRL-001 Sprint 7 미완성 gap(operational deployment blocker)을 정식 해소. Composite domain: AX + SERVER(인프라/부팅 sub-domain). Sprint Contract Exclusion 10개 명시. (작성자: ircp)

> Schema note: YAML frontmatter는 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 / SPEC-AX-AUTH-002와 동일하게 `.claude/skills/moai/workflows/plan.md` Phase 2의 8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`)를 따른다. `labels`, `created_at` 같은 변형 필드는 canonical schema에 없으므로 plan-auditor 결함 제기 시 본 메모와 `plan.md` 마지막 섹션을 출처로 거부한다. (lessons_session_2026_05_14 #1 적용)

---

# SPEC-AX-SERVER-001 — 서버 부트스트랩 & Dual Listener

## 1. 개요

`apps/control-plane/cmd/server/`에 실제 서버 부팅 진입점을 도입한다. SPEC-AX-001 ~ SPEC-AX-AUTH-002 4개 SPEC이 정의한 components(`config.Config`, `store.PgWorkflowStore`, `workflow.TxCoordinator`, `workflow.StateMachine`, `scheduler.CeleryDispatcher`, `auth.TokenValidator`(생성자 `auth.New`), `auth.RedisRefreshStore`(생성자 `auth.NewRedisRefreshStore`), `auth.OIDCClient`, `auth.JWKSCache`, `auth.BuildRESTChain`, `auth.BuildGRPCInterceptorChain`, `audit.Recorder`, `server.WorkflowService`, `server.RESTHandler`)는 모두 모듈 단위 GREEN이지만, 이를 조립·부팅·종료하는 calling code가 존재하지 않아 운영 배포가 불가능하다. 본 SPEC은 다음을 보장한다:

1. **Dual listener**: gRPC `:50051` + REST `:8080` 동시 listen (errgroup 기반)
2. **명시적 dependency injection**: REQ-SERVER-UBI-001-b의 단계 순서(config → logger → store → redis → oidc/jwks → auth → recorder/coordinator/state-machine → dispatcher → handler → chain → listeners) 코드로 강제, 각 단계 실패 시 명확한 error wrapping + 부분 cleanup
3. **Graceful shutdown**: SIGTERM/SIGINT 수신 시 진행 중 요청 30s timeout 내 완료, DB/Redis connection close
4. **Health/Readiness probes**: `/health`(liveness, 항상 200), `/ready`(readiness, DB ping + Redis ping + JWKS reachable 통과 시 200), gRPC health check
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

`structure.md` §2 디렉토리 트리를 따른다. 본 SPEC은 **신규 파일 4개**(`apps/control-plane/cmd/server/main.go`, `apps/control-plane/cmd/server/probes.go`, `apps/control-plane/cmd/server/server_test.go`, `apps/control-plane/internal/scheduler/redis_adapter.go`)를 추가하고, **기존 파일 1개**(`apps/control-plane/cmd/server/server.go`)를 전면 재작성하며, **기존 파일 2개에 readiness helper 메서드를 추가**(`internal/store/pg_store.go`에 `Ping(ctx) error`, `internal/auth/jwks_cache.go`에 `Reachable(ctx) bool`)한다. 후자 2개 및 신규 `redis_adapter.go`는 Sprint 0 pre-req chore(spec.md §6 패턴)로 처리하며 기존 메서드/시그니처는 변경하지 않고 메서드/파일만 추가한다. `redis_adapter.go`는 현재 `internal/server/e2e_test.go`에 test-only로 존재하는 `goRedisAdapter`를 production code로 promote한 것이다(D9 해소 — 아래 §2.0 / §2.1 / plan.md S0 참조). Delta 마커는 Run phase에서 정확한 라인 단위로 결정.

### 2.0 실제 API 검증 결과 (iteration 2, plan-audit FAIL 대응)

iteration 1 spec.md는 존재하지 않는 phantom API를 참조했다. iteration 2에서 9개 source file을 정독하여 다음을 확정한다 (이 표가 wiring spine의 단일 진실):

| SPEC iter1 가정 (phantom) | 실제 API (검증됨) | 출처 파일 |
|---------------------------|-------------------|-----------|
| `auth.NewTokenValidator()` | `auth.New(ctx, oidcIssuer, audience, opts ...ValidatorOption) (*TokenValidator, error)` | `internal/auth/validator.go:157` |
| `auth.NewRefreshTokenStore()` | `auth.NewRedisRefreshStore(addr string) *RedisRefreshStore` (+ 선택적 `auth.NewRefreshService(store, validator, issuer, auditLogger)`) | `internal/auth/refresh.go:406,96` |
| `pgStore.Ping(ctx)` | 미존재 → **S0에서 추가**. 현재 `*PgWorkflowStore`는 `Close()`/`BeginTx`/`PoolStats() *pgxpool.Stat`/`ListWorkflows`만 보유 (`pool.Ping`은 `NewPgWorkflowStore` 생성자 내부에서만 호출) | `internal/store/pg_store.go:38~81` |
| `oidcClient.JWKSReachable(ctx)` | 미존재. `*OIDCClient`는 `GetMetadata() *Metadata`만 보유. JWKS readiness는 `*JWKSCache`에 **S0에서 `Reachable(ctx) bool` 추가**로 해소 (현재 `JWKSCache`는 `lastSuccessAt`/`ttl`/`staleMaxAge`/`cacheAge()` 보유, public reachability getter 부재) | `internal/auth/oidc.go:137`, `internal/auth/jwks_cache.go:118~218` |
| `oidcClient.Close()` | 미존재. `OIDCClient`는 stateless(httpClient + 캐시된 metadata만) → **Close 불필요, cleanup에서 제외** | `internal/auth/oidc.go:37~139` |
| `redis.NewClient()` + `redisClient.Ping()`/`Close()` | `github.com/redis/go-redis/v9` `redis.NewClient(&redis.Options{Addr}) *redis.Client` 직접 생성. `*redis.Client`는 `Ping(ctx) *StatusCmd`(`.Err()`) + `Close() error`를 보유하므로 readiness/cleanup 용도로는 직접 사용 가능. **단 `scheduler.RedisClient` 인터페이스는 직접 충족하지 못한다** (D9 정정): `scheduler.RedisClient`(dispatcher.go:24~29)는 `RPush(ctx,key,values...) (int64,error)` + `Ping(ctx) error`를 요구하나 go-redis v9 raw client의 `RPush`는 `*redis.IntCmd`, `Ping`은 `*redis.StatusCmd`를 반환하여 시그니처 불일치 → **어댑터 경유 필수**. 코드베이스에 이미 어댑터가 존재: `internal/server/e2e_test.go:199~209` `goRedisAdapter{client *redis.Client}` (`RPush → a.client.RPush(...).Result()`, `Ping → a.client.Ping(...).Err()`). 현재 test-only이므로 production wiring을 위해 S0에서 production code로 promote 필요(아래 §2.1 참조) | `internal/scheduler/dispatcher.go:24~29,44`, `internal/server/e2e_test.go:199~209` |
| `celeryDispatcher.Close()` | 미존재. `*CeleryDispatcher`는 `SetBuilder`/`Dispatch`/`BuildEnvelope`만 보유 → cleanup은 dispatcher 대신 보유한 `*redis.Client.Close()`로 수행 | `internal/scheduler/dispatcher.go:36~99` |
| `tokenValidator.Close()` | 미존재. `*TokenValidator`는 `Verify`만 보유 → cleanup 불필요 | `internal/auth/validator.go:197` |
| `workflow.NewStateMachine()` (선행 단계 없음) | `workflow.NewStateMachine(coordinator *TxCoordinator, logger *zap.Logger) *StateMachine`. `TxCoordinator`는 `workflow.NewTxCoordinator(s store.WorkflowStore, r *audit.Recorder) *TxCoordinator`, `Recorder`는 `audit.NewRecorder(authEnabled bool) *Recorder`. 즉 wiring에 `recorder → tx_coordinator → state_machine` 3단계 필요 | `internal/workflow/state_machine.go:63`, `internal/workflow/transaction.go:26`, `internal/audit/recorder.go:46` |
| `server.NewWorkflowService` / `server.NewRESTHandler` | `server.NewWorkflowService(s store.WorkflowStore, sm *workflow.StateMachine, logger) *WorkflowService`; `server.NewRESTHandler(svc *WorkflowService, logger) *RESTHandler`; `RESTHandler.Mux() http.Handler` (시그니처 변경 없음, 검증됨) | `internal/server/grpc_server.go:51`, `internal/server/rest_handler.go:33,94` |

### Scope Boundary

본 SPEC은 **부팅·종료·헬스체크 코드 그 자체**(`main.go`, `server.go`, `probes.go`)와 그에 대한 **testcontainers 기반 E2E**(`server_test.go`)로 한정한다. 비즈니스 핸들러 코드(`WorkflowService`, `RESTHandler`, `RESTAuthzMiddleware`, `UnaryAuthzInterceptor`), RBAC 매트릭스(`rbac.go`), TokenValidator 자체 로직, store/scheduler 내부 구현 등은 본 SPEC 범위 외이다. 본 SPEC은 그들을 조립·호출하는 calling code만 작성한다.

### 2.1 Go Control Plane (`apps/control-plane/`)

| 경로 | 책임 | 모듈 | 신규/수정 |
|------|------|------|---------|
| `apps/control-plane/cmd/server/main.go` | OS 진입점. `os.Args` 파싱(현재 없음, 모두 env), 로거 초기화, `config.Load()` 호출, `Server` 인스턴스 생성, `server.Run(ctx)` 호출, exit code 반환 | REQ-SERVER-UBI-001 + REQ-SERVER-002 | 신규 |
| `apps/control-plane/cmd/server/server.go` | `Server` struct + `New()` + `Run(ctx)` + `Shutdown(ctx)` 전면 재작성. dependency wiring 6단계 + dual listener errgroup + graceful shutdown signal handling | REQ-SERVER-001 ~ 003 | 전면 재작성 |
| `apps/control-plane/cmd/server/probes.go` | `/health` / `/ready` HTTP 핸들러 + gRPC health.v1.Health 구현 (`Check` method). DB ping + Redis ping + JWKS reachable 검증 헬퍼 | REQ-SERVER-004 | 신규 |
| `apps/control-plane/cmd/server/server_test.go` | 단위 테스트: dependency wiring 순서 검증, port conflict 시 graceful error, DB 실패 시 startup abort, SIGTERM 처리 race | REQ-SERVER-001 ~ 004 | 신규 |
| `apps/control-plane/internal/store/pg_store.go` | **S0 deliverable (D1 해소)**: `func (s *PgWorkflowStore) Ping(ctx context.Context) error` 메서드 신규 추가. 내부적으로 `s.pool.Ping(ctx)`를 호출하여 `fmt.Errorf("postgres ping 실패: %w", err)` 래핑 반환. 기존 메서드(`Close`/`BeginTx`/`PoolStats`/`ListWorkflows`)는 미변경. readiness probe(REQ-SERVER-004-E2)와 startup abort(REQ-SERVER-002-U1)가 정당하게 요구하는 helper | REQ-SERVER-002-U1 + REQ-SERVER-004-E2 | 메서드 추가 (S0) |
| `apps/control-plane/internal/auth/jwks_cache.go` | **S0 deliverable (D4 해소)**: `func (c *JWKSCache) Reachable(ctx context.Context) bool` 메서드 신규 추가. `c.mu.RLock()`을 보유한 채 `c.lastSuccessAt`가 zero가 아니고 `c.cacheAge() < c.staleMaxAge`이면 true(=마지막 fetch 성공 + stale 유효 기간 내). `cacheAge()`(jwks_cache.go:212)는 "호출자가 `mu.RLock`을 보유해야 한다"는 동시성 계약을 가지므로 `Reachable`은 `c.mu.RLock()` 획득 후 `lastSuccessAt`/`cacheAge()`를 읽어야 한다(D11 정정 — concurrent-safe). 기존 `GetKey`/`refresh`/필드는 미변경. JWKS readiness(REQ-SERVER-004-E2 (iii))가 정당하게 요구하는 helper. OIDCClient는 stateless이므로 별도 변경 없음 | REQ-SERVER-004-E2 | 메서드 추가 (S0) |
| `apps/control-plane/internal/scheduler/redis_adapter.go` | **S0 deliverable (D9 해소)**: 신규 파일. `internal/server/e2e_test.go:199~209`의 test-only `goRedisAdapter`(`type goRedisAdapter struct { client *redis.Client }`, `RPush → a.client.RPush(ctx,key,values...).Result()`, `Ping → a.client.Ping(ctx).Err()`)를 production code로 promote한다. `scheduler` 패키지에 exported 타입(예: `RedisClientAdapter`)으로 정의하여 `scheduler.RedisClient` 인터페이스를 충족시킨다. raw `*redis.Client`는 인터페이스를 직접 충족하지 못하므로(go-redis v9가 command 타입 반환) wiring 단계 (h)에서 이 어댑터를 경유해야 한다. e2e_test.go의 기존 `goRedisAdapter`는 본 SPEC 범위 외(테스트가 production 어댑터로 전환할지는 후속 cleanup; 본 SPEC은 production 어댑터 추가만 보장) | REQ-SERVER-002-E1 (step h) | 신규 (S0) |
| `apps/control-plane/internal/server/grpc_server.go` | 본 SPEC 범위 외. 본 SPEC은 `server.NewWorkflowService(store, sm, logger)`를 호출자로 사용만 함 (시그니처 검증됨) | - | 미수정 |
| `apps/control-plane/internal/server/rest_handler.go` | 본 SPEC 범위 외. 본 SPEC은 `server.NewRESTHandler(svc, logger)` 후 `RESTHandler.Mux()`를 호출하여 `auth.BuildRESTChain`으로 wrap만 함 (시그니처 검증됨) | - | 미수정 |
| `apps/control-plane/internal/auth/chain.go` | 본 SPEC 범위 외. AUTH-002 GREEN 산출물을 신뢰. 본 SPEC은 `auth.BuildRESTChain(mux, validator, recorder, authEnabled)` / `auth.BuildGRPCInterceptorChain(validator, recorder, authEnabled)`을 호출만 함 | - | 미수정 |
| `apps/control-plane/internal/auth/validator.go` / `oidc.go` / `refresh.go` | 본 SPEC 범위 외. 본 SPEC은 `auth.New(ctx, issuer, audience, opts...)` / `auth.NewOIDCClient(ctx, issuer, opts...)` / `auth.NewRedisRefreshStore(addr)`를 호출만 함. 이들 타입에 Close 메서드 추가 안 함(stateless / cleanup 불필요) | - | 미수정 |
| `apps/control-plane/internal/config/config.go` | 본 SPEC 범위 외. `config.Load()` 그대로 사용. 단, 신규 env 2개(`SHUTDOWN_TIMEOUT_SECONDS`, `READY_PROBE_TIMEOUT_SECONDS`) + 기존 env(`REDIS_ADDR`/`OIDC_ISSUER_URL`/`OIDC_AUDIENCE`/`CELERY_QUEUE`)는 S0 chore에서 config.go에 추가/확인(plan.md S0에서 명시) | - | 사전 의존 (S0 deliverable) |

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

본 SPEC은 EARS 4개 패턴(Ubiquitous / Event-driven / State-driven / Unwanted)을 사용한다. **Optional(`Where the [feature] ..., the system SHALL ...`) 패턴은 미해당** — 본 SPEC의 모든 요구사항은 인프라 불변 조건(부팅·종료·헬스체크)으로 선택적 기능이 존재하지 않는다. `cfg.AuthEnabled` 조건부 동작은 Optional이 아니라 Unwanted(U2) / State-driven(UBI-001-c) 절 안에 내재된다 (D7 정정: iteration 1의 "5패턴 모두 포함" 주장은 부정확했으며 실제 4패턴이다).

### 3.1 Ubiquitous Requirements (시스템 전반 불변 조건)

**REQ-SERVER-UBI-001 (부팅·종료 추적성 + 의존성 순서 강제 + Graceful guarantee)**

본 UBI는 3개 sub-clause를 가지며, 각 sub-clause는 acceptance.md에서 dedicated AC를 가진다(lessons #2 적용).

- **REQ-SERVER-UBI-001-a (모든 startup/shutdown 단계 audit)**: The system SHALL record every server lifecycle transition to `audit_logs` with action type `SERVER_STARTUP` (after all listeners bound successfully), `SERVER_SHUTDOWN_INITIATED` (signal received), `SERVER_SHUTDOWN_COMPLETED` (all listeners drained + connections closed). 각 audit row는 `details.grpc_addr`, `details.rest_addr`, `details.uptime_seconds`(shutdown 시), `details.exit_reason`(signal / fatal_error) 필드를 포함한다. `userID`는 `system` 고정(인간 사용자가 아닌 시스템 이벤트).
- **REQ-SERVER-UBI-001-b (Dependency 순서 강제)**: WHILE server is in startup phase, the system SHALL initialize dependencies in this exact order, using only the verified APIs of §2.0:
  - (a) `config.Load()` — `*config.Config` 반환, error 없음(infallible, D8 정정)
  - (b) `logger.New()` (zap 로거 초기화) — error 없음(infallible)
  - (c) `store.NewPgWorkflowStore(ctx, cfg.PostgresDSN, logger)` 호출 → 그 다음 신규 S0 메서드 `pgStore.Ping(ctx)` 호출 (생성자 내부 `pool.Ping`과 별개로 readiness 재확인용)
  - (d) `redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})`(`github.com/redis/go-redis/v9`) → 그 다음 `redisClient.Ping(ctx).Err()` 호출
  - (e) `cfg.AuthEnabled=true`인 경우만: `auth.NewOIDCClient(ctx, cfg.OIDCIssuerURL)` (생성자가 OIDC Discovery 수행) → `jwksCache := auth.NewJWKSCache(oidcClient.GetMetadata().JWKSUri)` → JWKS warm-up(`jwksCache`의 첫 fetch를 강제하여 키 캐시 채움). `cfg.AuthEnabled=false`이면 (e) 전체 skip(`oidcClient`/`jwksCache` 모두 nil)
  - (f) `auth.New(ctx, cfg.OIDCIssuerURL, cfg.OIDCAudience, auth.WithJWKSProvider(jwksCache))` → `tokenValidator`; `auth.NewRedisRefreshStore(cfg.RedisAddr)` → `refreshStore` (`cfg.AuthEnabled=false`이면 tokenValidator는 chain에서 미사용이나 생성 자체는 가능 — chain이 authEnabled=false 시 bypass)
  - (g) `audit.NewRecorder(cfg.AuthEnabled)` → `recorder`; `workflow.NewTxCoordinator(pgStore, recorder)` → `txCoord`; `workflow.NewStateMachine(txCoord, logger)` → `sm` (D6 정정: TxCoordinator/Recorder가 StateMachine 선행 의존)
  - (h) `redisAdapter := scheduler.NewRedisClientAdapter(redisClient)` (S0에서 production code로 promote된 어댑터 — D9 정정: raw `*redis.Client`는 `scheduler.RedisClient` 인터페이스를 직접 충족하지 못함) → `scheduler.NewCeleryDispatcher(redisAdapter, cfg.CeleryQueue, hostname)` → `dispatcher` (생성자가 `scheduler.RedisClient` 인터페이스를 받으며 어댑터가 이를 충족: `RPush → .Result()`, `Ping → .Err()`)
  - (i) `server.NewWorkflowService(pgStore, sm, logger)` → `workflowSvc`; `server.NewRESTHandler(workflowSvc, logger)` → `restHandler`
  - (j) `auth.BuildRESTChain(restHandler.Mux(), tokenValidator, recorder, cfg.AuthEnabled)` + `auth.BuildGRPCInterceptorChain(tokenValidator, recorder, cfg.AuthEnabled)`
  - (k) gRPC `Listen` + REST `Listen` (errgroup)

  각 단계는 직전 단계의 성공을 전제하며, 단계 (c)~(j) 중 어느 하나라도 실패 시 후속 단계 진입 금지 + 이미 완료된 단계의 cleanup(역순) 수행. 단계 (a)/(b)는 infallible이므로 wrapping 대상이 아니다.
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

- **REQ-SERVER-002-E1 (Sequential init with explicit error wrapping)**: WHEN `Server.New(cfg, logger)` is called, THEN the system SHALL initialize dependencies in the order defined by REQ-SERVER-UBI-001-b, AND each fallible initialization step that returns an error SHALL be wrapped as `fmt.Errorf("init step %s failed: %w", stepName, err)` with `stepName` ∈ `{"pg_store", "pg_ping", "redis_client", "redis_ping", "oidc_client", "jwks_warmup", "token_validator", "refresh_store", "recorder", "tx_coordinator", "state_machine", "celery_dispatcher", "workflow_service", "rest_handler", "auth_chain"}`. enum은 단계 (c)의 `pg_store`부터 시작한다 — 단계 (a)`config.Load()`/(b)`logger.New()`는 error를 반환하지 않으므로(infallible) enum 범위 밖이다(D8 정정: narrative와 enum scope 정렬). `recorder`/`tx_coordinator`/`state_machine`/`celery_dispatcher`/`workflow_service`/`rest_handler`는 현재 생성자가 error를 반환하지 않으나(`audit.NewRecorder`/`workflow.NewTxCoordinator`/`workflow.NewStateMachine`/`scheduler.NewCeleryDispatcher`/`server.NewWorkflowService`/`server.NewRESTHandler` 모두 단일 반환값), nil 입력 방어 검사 실패 시 동일 wrapping 형식을 사용한다. 단계 (h)의 `scheduler.NewRedisClientAdapter(redisClient)` 호출(D9 해소 — raw `*redis.Client`를 `scheduler.RedisClient`로 래핑)은 단순 struct 래핑으로 infallible이며 별도 stepName 없이 `celery_dispatcher` 단계에 내재한다(captured slice는 15-element 유지 — 어댑터 생성은 추적 항목 아님). 각 단계 실패 시 후속 단계 진입 금지.

- **REQ-SERVER-002-E2 (Partial cleanup on failure)**: WHEN any fallible init step (c)~(j) in REQ-SERVER-UBI-001-b returns an error, THEN the system SHALL invoke cleanup of previously-initialized closable resources in REVERSE order. 실제로 명시적 정리가 필요한 자원은 두 개뿐이다: `redisClient.Close()` (go-redis `*redis.Client.Close() error`) → `pgStore.Close()` (`*PgWorkflowStore.Close()`, 반환값 없음). `tokenValidator`/`oidcClient`/`jwksCache`/`celeryDispatcher`/`refreshStore`는 Close 메서드가 없고 stateless이거나 GC로 회수되므로 cleanup 대상이 아니다(D5 정정: phantom `celeryDispatcher.Close()`/`tokenValidator.Close()`/`oidcClient.Close()` 제거). gRPC/HTTP server는 New 단계에서 미생성이므로 cleanup no-op. Cleanup 자체의 error(예: `redisClient.Close()`)는 logging만 하고 무시(이미 부팅 실패 상태에서 cleanup 실패 추가 wrap은 root cause를 가림).

- **REQ-SERVER-002-E3 (Auth chain composition — single line mount)**: WHEN dependency wiring reaches step (j), THEN the system SHALL invoke `auth.BuildRESTChain(restHandler.Mux(), tokenValidator, auditRecorder, cfg.AuthEnabled)` to obtain the wrapped REST handler AND `auth.BuildGRPCInterceptorChain(tokenValidator, auditRecorder, cfg.AuthEnabled)` to obtain the gRPC `ServerOption`. 두 헬퍼는 AUTH-002 GREEN 산출물이며 본 SPEC은 호출만 한다 (체인 순서·white-list bypass·default-deny 등 모든 wiring 정책은 AUTH-002 책임).

#### Unwanted

- **REQ-SERVER-002-U1 (DB connection failure aborts startup)**: IF step (c) `pgStore.Ping(ctx)` (S0에서 추가되는 `*PgWorkflowStore.Ping(ctx) error` 메서드) returns an error (stepName `pg_ping`) OR step (d) `redisClient.Ping(ctx).Err()` returns an error (stepName `redis_ping`), THEN the system SHALL abort startup with error wrapping per REQ-SERVER-002-E1, SHALL NOT proceed to listener bind, AND SHALL NOT insert `SERVER_STARTUP` audit row. `store.NewPgWorkflowStore` 생성자도 내부에서 `pool.Ping`을 호출하므로 잘못된 DSN은 단계 (c) `pg_store`에서 이미 실패할 수 있다; 단계 (c) 이후의 명시적 `pgStore.Ping(ctx)`는 생성 성공 후 런타임 연결 단절을 추가로 검출한다(readiness probe와 동일 메서드 재사용). DB 미가용 상태에서 server가 부팅되면 모든 요청이 503을 반환하므로 K8s가 unhealthy pod로 인식하여 restart 유도하는 것이 안전.

- **REQ-SERVER-002-U2 (JWKS fetch failure aborts startup when AuthEnabled=true)**: IF `cfg.AuthEnabled=true` AND step (e) JWKS warm-up(`jwksCache` 첫 fetch) returns an error (network timeout / DNS failure / 5xx) OR `auth.NewOIDCClient(ctx, cfg.OIDCIssuerURL)` returns an error (Discovery 실패), THEN the system SHALL abort startup with error wrapping (stepName `oidc_client` 또는 `jwks_warmup`). AuthEnabled=false 환경에서는 REQ-SERVER-UBI-001-b 단계 (e) 전체(OIDC client + JWKSCache 생성 + warm-up)를 skip하므로 JWKS 관련 실패가 발생하지 않는다.

### 3.4 REQ-SERVER-003 — Graceful Shutdown

#### Event-driven

- **REQ-SERVER-003-E1 (Signal handling)**: WHEN the process receives `syscall.SIGTERM` OR `syscall.SIGINT`, THEN `signal.NotifyContext(parentCtx, syscall.SIGTERM, syscall.SIGINT)` shall cancel the shared `ctx`, the errgroup `g.Wait()` shall observe `ctx.Done()`, AND the system SHALL invoke shutdown in this order: (i) `httpServer.Shutdown(shutdownCtx)` with `shutdownCtx` deadline `cfg.ShutdownTimeoutSeconds` (default 30s), (ii) `grpcServer.GracefulStop()` (no timeout API in grpc-go; relies on shutdownCtx via goroutine race with `grpcServer.Stop()`), (iii) `redisClient.Close()` (go-redis `*redis.Client.Close() error`), (iv) `pgStore.Close()` (`*PgWorkflowStore.Close()`, 반환값 없음), (v) insert `SERVER_SHUTDOWN_COMPLETED` audit row, (vi) return from `Server.Run()`. `tokenValidator`/`oidcClient`/`jwksCache`/`celeryDispatcher`는 Close 메서드가 없으므로 shutdown 시 별도 정리하지 않는다.

- **REQ-SERVER-003-E2 (In-flight request completion)**: WHEN `httpServer.Shutdown(shutdownCtx)` is invoked, THEN the system SHALL allow currently-processing HTTP requests to complete naturally up to the shutdownCtx deadline. Idle connections are closed immediately. New incoming connections are refused with TCP RST. gRPC side: `grpcServer.GracefulStop()` blocks until all pending RPCs complete, but is raced with `time.AfterFunc(shutdownTimeout, grpcServer.Stop)` to enforce the timeout.

#### State-driven

- **REQ-SERVER-003-S1 (Shutdown idempotency)**: WHILE `Server.shutdown()` has already been invoked once, the system SHALL be idempotent — subsequent signals (SIGINT 두 번 등) are observed but do not re-enter shutdown procedure. 단, 두 번째 신호가 도착하면 즉시 `grpcServer.Stop()` + `httpServer.Close()` 강제 종료 호출 (force-kill escape hatch). 첫 SIGTERM 30초 대기 중 두 번째 SIGINT가 오면 사용자가 강제 종료를 원하는 의도로 간주.

#### Unwanted

- **REQ-SERVER-003-U1 (Shutdown timeout force-kill)**: IF `httpServer.Shutdown(shutdownCtx)` returns `context.DeadlineExceeded` after `cfg.ShutdownTimeoutSeconds`, THEN the system SHALL call `httpServer.Close()` for immediate force-close, AND log structured warning `zap.String("phase", "shutdown"), zap.String("reason", "timeout_force_kill")`, AND the `SERVER_SHUTDOWN_COMPLETED` audit row SHALL include `details.exit_reason="force_kill_timeout"`. 동일 시점에 gRPC side는 `grpcServer.Stop()`(non-graceful)가 호출됨.

### 3.5 REQ-SERVER-004 — Health & Readiness Probes

#### Event-driven

- **REQ-SERVER-004-E1 (Liveness probe)**: WHEN an HTTP request `GET /health` arrives, THEN the REST handler SHALL return HTTP 200 with body `{"status":"ok","service":"iroum-ax-control-plane","version":"<build_version>"}` regardless of dependency state. 본 endpoint는 K8s livenessProbe 용으로 "프로세스가 살아있고 HTTP listener가 동작 중인가"만 검증한다. 본 endpoint는 AUTH-002 §3.2 REST mapping table에서 `bypass`로 명시되어 인증/인가를 통과하지 않는다.

- **REQ-SERVER-004-E2 (Readiness probe)**: WHEN an HTTP request `GET /ready` arrives, THEN the REST handler SHALL execute up to three checks within `cfg.ReadyProbeTimeoutSeconds` (default 5s): (i) `pgStore.Ping(ctx)` (S0에서 추가되는 `*PgWorkflowStore.Ping(ctx) error` 메서드, error nil이면 ok), (ii) `redisClient.Ping(ctx).Err()` (go-redis StatusCmd, nil이면 ok), (iii) `cfg.AuthEnabled=true`인 경우만 `jwksCache.Reachable(ctx)` (S0에서 추가되는 `*JWKSCache.Reachable(ctx) bool` 메서드 — 마지막 JWKS fetch 성공 여부 + staleMaxAge 내 staleness 여부; OIDCClient는 stateless이므로 별도 검사 안 함, D4 정정). 세 검사(AuthEnabled=false 시 두 검사) 모두 통과 시 HTTP 200 + body `{"status":"ready","checks":{"postgres":"ok","redis":"ok","oidc":"ok"}}` (AuthEnabled=false 시 `oidc` 키 생략 또는 `"skipped"`). 하나라도 실패 시 HTTP 503 + body `{"status":"not_ready","checks":{...}}` (실패한 check만 `"failed: <error message>"` 또는 JWKS의 경우 `"failed: jwks unreachable"`로 표시). 본 endpoint도 AUTH-002 매핑 테이블에서 `bypass`로 추가되어야 하나, 본 SPEC은 `/ready`를 AUTH-002 chain 외부에 mount하여 진입 자체를 회피한다(plan.md S3 + Risk Register 참조).

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
- **SPEC-AX-CTRL-001 GREEN 가정 (부분)**: `server.NewWorkflowService(store, sm, logger)` + `server.NewRESTHandler(svc, logger)` + `RESTHandler.Mux()` + `workflow.NewTxCoordinator(store, recorder)` + `workflow.NewStateMachine(coordinator, logger)` + `sm.Coordinator()` + `audit.NewRecorder(authEnabled)` 모듈 GREEN (모두 §2.0에서 시그니처 검증됨). 단, Sprint 7 (`cmd/server/server.go` 실제 부팅)은 본 SPEC이 해소한다 — CTRL-001 Sprint 7 미완성 gap을 본 SPEC이 흡수.
- **SPEC-AX-AUTH-001 GREEN 가정**: `rbac.go` 매트릭스 + `auth.New(ctx, issuer, audience, opts...)` (`*TokenValidator`) + `auth.NewRedisRefreshStore(addr)` (`*RedisRefreshStore`) + `auth.NewOIDCClient(ctx, issuer, opts...)` (`*OIDCClient`, Discovery 수행) + `auth.NewJWKSCache(jwksURI, opts...)` (`*JWKSCache`, JWKSProvider). 본 SPEC은 이들을 wiring 단계 (e)~(f)에서 호출만 하며, 단 readiness를 위해 `jwks_cache.go`에 `Reachable(ctx) bool` 메서드를 S0에서 추가한다(§2.1).
- **SPEC-AX-AUTH-002 v0.1.2 GREEN 가정**: `auth.BuildRESTChain(mux, validator, recorder, authEnabled)` / `auth.BuildGRPCInterceptorChain(validator, recorder, authEnabled)` 헬퍼 (시그니처 검증됨). 본 SPEC은 step (j)에서 두 함수를 호출하여 한 줄로 미들웨어 chain mount. AUTH-002 §6 "SPEC-AX-SERVER-001 사후 책임"이 본 SPEC GREEN으로 해소됨.
- **AUTH-002 §5 Exclusion #12 unblock**: 본 SPEC GREEN 후 AUTH-002 Exclusion #12는 historical only(이미 처리됨). AUTH-002 SPEC 파일 자체 수정은 본 SPEC 범위 외 (별도 chore commit으로 `AUTH-002 Exclusion #12: RESOLVED by SPEC-AX-SERVER-001 v0.1.1` 추가 가능, 또는 미수정 — 본 SPEC은 unblock fact만 보장).
- **Go 의존성**: `golang.org/x/sync/errgroup` 신규 require 필요(S0 deliverable). `github.com/redis/go-redis/v9`는 이미 `internal/server/e2e_test.go`가 사용 중인 기존 의존성 — 신규 require 아님(`go.mod` 확인 후 누락 시 S0에서 직접 require로 승격). 단 raw `*redis.Client`는 `scheduler.RedisClient` 인터페이스를 직접 충족하지 못하므로(go-redis v9가 `*redis.IntCmd`/`*redis.StatusCmd` 등 command 타입 반환) S0에서 `internal/scheduler/redis_adapter.go`에 어댑터(`goRedisAdapter` test-only → production `scheduler.NewRedisClientAdapter`)를 promote하여 wiring 단계 (h)에서 경유한다(D9 해소). `google.golang.org/grpc/health`는 이미 transitively 포함.
- **Database**: schema 변경 없음. `audit/actions.go`에 3개 const 추가, `pg_store.go`에 `Ping(ctx) error` 메서드 추가, `jwks_cache.go`에 `Reachable(ctx) bool` 메서드 추가, `scheduler/redis_adapter.go` 신규 파일(어댑터 promote)만 (모두 S0).
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
