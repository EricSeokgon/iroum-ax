# SPEC-AX-SERVER-001 — Compact Summary

> Compact reference for agent context. Full spec: `spec.md`.

---

## Identity

- **ID**: SPEC-AX-SERVER-001
- **Version**: 0.1.2
- **Status**: draft
- **Priority**: high
- **Composite Domain**: AX + SERVER (서버 부팅/인프라 sub-domain)
- **Methodology**: TDD (권장) — 모든 deliverable이 신규 + 외부 동작 zero
- **Harness**: thorough

## Mission

4개 선행 SPEC(AX-001 + CTRL-001 + AUTH-001 + AUTH-002 v0.1.2)이 정의한 components를 조립·부팅·종료하는 calling code를 작성한다. 비즈니스 로직 0줄.

## iteration 2~3 핵심 변경 (plan-audit FAIL 대응)

근본 원인(iter1): spec.md가 존재하지 않는 phantom API에 wiring spine을 의존. 9개 source file 정독 후 실제 API로 전면 재작성. spec.md §2.0 표가 wiring 단일 진실.

| phantom (iter1) | 실제 API (검증됨) |
|-----------------|-------------------|
| `auth.NewTokenValidator()` | `auth.New(ctx, oidcIssuer, audience, opts...)` |
| `auth.NewRefreshTokenStore()` | `auth.NewRedisRefreshStore(addr)` (+ `auth.NewRefreshService`) |
| `pgStore.Ping(ctx)` | 미존재 → **S0 추가** (`pg_store.go`) |
| `oidcClient.JWKSReachable()` | 미존재 → JWKS readiness는 **S0 `JWKSCache.Reachable(ctx) bool`** (`jwks_cache.go`, mu.RLock 보유 — D11) |
| `oidcClient.Close()` | 미존재 → OIDCClient stateless, cleanup 제외 |
| raw `*redis.Client` → `scheduler.RedisClient` | **직접 충족 불가** (go-redis v9 command 타입 반환) → **S0 어댑터 promote** (`scheduler/redis_adapter.go` `NewRedisClientAdapter`, e2e_test.go:199~209 goRedisAdapter에서 — D9) |
| `celeryDispatcher.Close()` / `tokenValidator.Close()` | 미존재 → cleanup에서 제거 (redis client `Close()`만) |
| `workflow.NewStateMachine()` (선행 없음) | `NewStateMachine(coord, logger)`; `recorder→tx_coordinator→state_machine` 3단계 선행 |

iteration 3 (FINAL, FAIL 0.78 → D9 Major/D11 Minor/D12 Minor 대응):
- **D9**: raw `*redis.Client`는 `scheduler.RedisClient` 인터페이스 직접 충족 불가 → S0에서 `internal/scheduler/redis_adapter.go` production 어댑터 promote, wiring 단계 (h) `scheduler.NewRedisClientAdapter(redisClient)` 경유 (어댑터 infallible — captured slice 15-element 유지)
- **D11**: `JWKSCache.Reachable`는 `c.mu.RLock()` 보유 후 `lastSuccessAt`/`cacheAge()` 평가 (`cacheAge()` 동시성 계약 준수, `-race` 검증)
- **D12**: acceptance.md §8을 EARS 4-pattern (Optional 미해당)으로 정정 — spec.md §3:L129와 정합

## Scope

- 신규 4개 파일: `cmd/server/main.go`, `cmd/server/probes.go`, `cmd/server/server_test.go`, `internal/scheduler/redis_adapter.go` (D9 어댑터 promote)
- 전면 재작성 1개 파일: `cmd/server/server.go` (40-line stub → ~220~270 LOC)
- **메서드 추가 2개 파일 (S0)**: `internal/store/pg_store.go`에 `Ping(ctx) error`, `internal/auth/jwks_cache.go`에 `Reachable(ctx) bool` (mu.RLock 보유, 기존 시그니처 불변)
- 사전 의존 (S0): `audit/actions.go` 3 const + `config/config.go` 2 field + `scheduler/redis_adapter.go` 신규 어댑터 + go.mod errgroup/redis require

## EARS Requirements (5 modules, 4 patterns — Optional 미해당, D7 정정)

1. **REQ-SERVER-UBI-001** (Ubiquitous, 3 sub-clauses):
   - (a) 모든 startup/shutdown audit
   - (b) Dependency 순서 강제: config→logger→pg_store(+Ping)→redis(+Ping)→[AuthEnabled]oidc/jwks→token_validator/refresh_store→recorder/tx_coordinator/state_machine→celery_dispatcher→workflow_service/rest_handler→auth_chain→listeners
   - (c) Graceful guarantee (in-flight 30s, AuthEnabled 무관)

2. **REQ-SERVER-001** (Dual Listener): S1 gRPC+REST 동시 listen / E1·E2 errgroup bind / U1 port conflict / U2 한쪽 죽으면 전체 종료

3. **REQ-SERVER-002** (Dependency Wiring):
   - E1 sequential init + error wrapping. stepName ∈ `{pg_store, pg_ping, redis_client, redis_ping, oidc_client, jwks_warmup, token_validator, refresh_store, recorder, tx_coordinator, state_machine, celery_dispatcher, workflow_service, rest_handler, auth_chain}` (config/logger infallible → enum 밖, D8). 단계 (h)는 `scheduler.NewRedisClientAdapter(redisClient)`(infallible struct 래핑, D9) → `scheduler.NewCeleryDispatcher(adapter, ...)` — 어댑터는 `celery_dispatcher`에 내재, 별도 stepName 미추가
   - E2 partial cleanup 역순: **`redisClient.Close()` → `pgStore.Close()`만** (나머지 Close 없음, D5)
   - E3 `auth.BuildRESTChain(mux, validator, recorder, authEnabled)` + `auth.BuildGRPCInterceptorChain(validator, recorder, authEnabled)`
   - U1 pg/redis Ping 실패 시 abort
   - U2 OIDC/JWKS 실패 시 abort (AuthEnabled=true만; false 시 단계 (e) 전체 skip)

4. **REQ-SERVER-003** (Graceful Shutdown): E1 SIGTERM/SIGINT + 5단계(httpShutdown→grpcGracefulStop→redisClient.Close→pgStore.Close→audit) / E2 in-flight 대기 / S1 sync.Once idempotency / S1-DoubleSignal force-kill / U1 timeout force-kill

5. **REQ-SERVER-004** (Health & Readiness): E1 `/health` 항상 200 / E2 `/ready` = `pgStore.Ping(ctx)` + `redisClient.Ping(ctx).Err()` + (AuthEnabled) `jwksCache.Reachable(ctx)` → 200/503 / E3 gRPC Health.Check 동일 / S1 shutdown 중 503 / U1 probe timeout

## Exclusions (10)

1. 다중 인스턴스 부하 분산 2. Hot reload 3. TLS termination 4. Distributed tracing 5. Prometheus `/metrics` 6. 다중 OIDC provider 7. API rate limiting 8. WebSocket/SSE 9. Server-side caching 10. Custom error pages

## NFR Highlights

- Startup p95 < 5s / Shutdown p95 < 30s / `/health` p99 < 5ms / `/ready` p95 < 500ms
- Coverage ≥ 85% / gosec G112 ReadHeaderTimeout / 망분리 정합 (외부 API 0건)

## Acceptance Criteria (17 = 14 + S0 helper/어댑터 3)

- AC-SERVER-S0-PgPing / AC-SERVER-S0-JWKSReachable (`-race`) / AC-SERVER-S0-RedisAdapter (D9, iter3 신규)
- AC-SERVER-UBI-001-a/b/c (b: captured slice 15-element, AuthEnabled=false 시 13)
- AC-SERVER-001-E1/E2/U1/U2
- AC-SERVER-002-E1/E2/E3/U1/U2 (실제 stepName + redisClient/pgStore Close만)
- AC-SERVER-003-E1/S1/S1-DoubleSignal/U1
- AC-SERVER-004-E1/E2/E2-DBDown/E3/S1/U1
- AC-SERVER-Edge-PortZero / AuthDisabledFullPath / FatalErrorDuringRun

## Dependencies (실제 시그니처 검증, spec.md §2.0)

- SPEC-AX-001 GREEN (`audit_logs`, `audit.NewRecorder(authEnabled)`, `cli-anonymous`)
- SPEC-AX-CTRL-001 부분 GREEN (`server.NewWorkflowService(store,sm,logger)`, `server.NewRESTHandler(svc,logger)`, `RESTHandler.Mux()`, `workflow.NewTxCoordinator(store,recorder)`, `workflow.NewStateMachine(coord,logger)`, `sm.Coordinator()`); Sprint 7 흡수
- SPEC-AX-AUTH-001 GREEN (`auth.New(ctx,issuer,audience,opts...)`, `auth.NewRedisRefreshStore(addr)`, `auth.NewOIDCClient(ctx,issuer,opts...)`, `auth.NewJWKSCache(uri,opts...)`)
- SPEC-AX-AUTH-002 v0.1.2 GREEN (`auth.BuildRESTChain(mux,validator,recorder,authEnabled)`, `auth.BuildGRPCInterceptorChain(validator,recorder,authEnabled)`)
- Go 의존성: `golang.org/x/sync/errgroup` 신규 require + `github.com/redis/go-redis/v9` (e2e_test 기존 사용, production `redis_adapter.go` direct import → direct 승격 필수) + `go.uber.org/goleak`(테스트)

## Cross-SPEC Resolution

- **AUTH-002 §5 Exclusion #12** → 본 SPEC GREEN으로 historical
- **CTRL-001 Sprint 7 T-AX-006** → 본 SPEC 흡수

## Sprint DAG (4 sprints incl. S0)

```
S0 (audit const 3 + config field 2 + pg_store.Ping + jwks_cache.Reachable[mu.RLock] + scheduler/redis_adapter.go + go.mod)
  ▼
S1 (server.go 전면 재작성[step h: NewRedisClientAdapter 경유] + main.go + 6 unit tests)
  ▼
S2 (graceful shutdown: sync.Once + signal + 4 unit tests)
  ▼
S3 (probes.go + testcontainers E2E 4 + benchmark 2)
```

## Top 3 Architectural Decisions

1. **errgroup + signal.NotifyContext** dual listener — 한쪽 실패 시 자동 cancel 전파.
2. **`/health` + `/ready` chain 외부 mount** — AUTH-002 chain 진입 회피, cross-SPEC 매핑 변경 불필요.
3. **`sync.Once` shutdown guard + double-signal force-kill** — 첫 signal graceful, 두 번째 force-kill.

## Schema Note

8-field canonical schema(`id, version, status, created, updated, author, priority, issue_number`) 준수. `labels`/`created_at` 등 변형 필드는 plan-auditor 결함 시 본 문서 + spec.md "Schema note" + plan.md §7 + acceptance.md §8을 출처로 거부.

## Lessons Applied

- #1 Schema note 포함
- #2 UBI sub-clauses 각 dedicated AC
- #4 stub-assert anti-pattern 회피 — testcontainers full-stack 실제 동작 검증; iter2에서 모든 AC를 실제 API로 정정; iter3에서 D9 redis 어댑터 경유 명시(raw client 인터페이스 미충족)
- #5 Cross-SPEC: AUTH-002 §12 + CTRL-001 Sprint 7 unblock 명시
- #6 ~600-800K tokens 예상
- #8 한국 공공 도메인 6 제약: 망분리 정합 / PIPA audit / 합니다체 미해당 / HWP 무관
