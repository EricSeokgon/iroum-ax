# Evaluation Report
SPEC: SPEC-AX-SERVER-001 v0.1.2
Evaluator: evaluator-active (independent, post plan-auditor iter 3 PASS 0.92)
Date: 2026-05-15
Overall Verdict: CONFIRM

---

## Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 85/100 | PASS | 모든 생성자 시그니처 코드 검증 완료. auth.New(ctx,issuer,audience,opts) validator.go:157 ✓, NewOIDCClient(ctx,issuerURL,opts) oidc.go:68 ✓, NewRedisRefreshStore(addr) refresh.go:406 ✓, NewTxCoordinator(store,recorder) transaction.go:26 ✓, NewStateMachine(coordinator,logger) state_machine.go:63 ✓, NewWorkflowService(store,sm,logger) grpc_server.go:51 ✓, NewRESTHandler(svc,logger) rest_handler.go:33 ✓, NewRecorder(authEnabled) recorder.go:46 ✓, BuildRESTChain(handler,validator,recorder,authEnabled,opts) chain.go:43 ✓, BuildGRPCInterceptorChain(validator,recorder,authEnabled) chain.go:86 ✓. S0 deliverables 모두 코드에서 ABSENT 확인 (Ping 미존재 pg_store.go:38~81, Reachable 미존재 jwks_cache.go:118~218, redis_adapter 미존재 scheduler/). RedisClient 인터페이스 dispatcher.go:24~29 정확. 17 AC 모두 검증 가능한 형태. |
| Security (25%) | 82/100 | PASS | G112 Slowloris: REQ-SERVER-001-E2 ReadHeaderTimeout:10s 명시. 미인증 포트 :50051/:8080. 프로브 인증 chain 외부 mount 정확. startup/shutdown audit trail 완비. 시크릿 미포함. gRPC Stop/GracefulStop race는 Risk Register에 명시 + 대응책(time.AfterFunc timer.Stop() + done chan) 기술. OWASP 위반 없음. |
| Craft (20%) | 80/100 | PASS | 17 테스트(S0 3 + S1 6 + S2 4 + S3 E2E 4) + 2 벤치마크 명세. goleak.VerifyNone + -race 지정. 역순 cleanup 정확(redisClient.Close → pgStore.Close만). fmt.Errorf("%w") error chain 전체. 85% coverage DoD에 명시. |
| Consistency (15%) | 83/100 | PASS | zap 구조화 로그, interface-based mock(RedisClient), testcontainers 패턴 모두 기존 codebase와 일치. @MX tags 4개 지정(ANCHOR×2, WARN+REASON, NOTE). stepName enum 패턴이 기존 audit.Action 상수 패턴과 정합. |

**Weighted Score**: 0.40×85 + 0.25×82 + 0.20×80 + 0.15×83 = 34.0 + 20.5 + 16.0 + 12.45 = **82.95 / 100 (0.83)**

---

## Findings

### plan-auditor 미발견 항목 (독립 검증)

- [Info] `apps/control-plane/cmd/server/server.go:3` — 현재 `package server`. 동일 디렉토리에 `main.go`(package main) 신규 생성 시 Go 규칙상 모든 파일이 동일 package name을 가져야 하므로 server.go도 `package main`으로 전환 필수. SPEC은 "전면 재작성"으로 처리하나 명시적 언급 없음. Run phase 구현자가 인지 필요 (빌드 오류 발생 조건).

- [Info] `apps/control-plane/internal/server/rest_handler.go:107` — `RESTHandler.Mux()`가 이미 `"GET /health"` 핸들러를 등록. 본 SPEC probes.go의 외부 mux도 동일 경로 등록 → 외부 mux가 더 구체적 패턴으로 inner 핸들러를 shadow. 기능 오류 없음(의도된 동작). 단, `/health` 응답 body가 두 핸들러 간 불일치 가능 — rest_handler.go의 handleHealth 구현 검증 불가(미공개). Run phase에서 응답 format 일치 확인 필요.

- [Verified] `apps/control-plane/internal/auth/oidc.go:68` — `NewOIDCClient(ctx context.Context, issuerURL string, opts ...OIDCClientOption) (*OIDCClient, error)`. plan-auditor가 이 파일을 직접 읽지 않았으나(검증 파일 목록: e2e_test.go, dispatcher.go, jwks_cache.go) 독립 검증 결과 spec.md §2.0 wiring step (e) `auth.NewOIDCClient(ctx, cfg.OIDCIssuerURL)` 와 MATCH. research.md v0.1.0의 구 시그니처(`issuerURL, audience, cacheTTL` 3-param) 는 stale이었으나 spec.md v0.1.2 §2.0은 올바름. 결함 없음.

### 전체 spec-to-code 정합 결과

| 검증 항목 | 코드 위치 | SPEC 주장 | 결과 |
|-----------|-----------|-----------|------|
| auth.New 시그니처 | validator.go:157 | `(ctx, issuer, audience, opts...)` | MATCH |
| auth.NewOIDCClient 시그니처 | oidc.go:68 | `(ctx, issuerURL, opts...)` | MATCH |
| auth.NewRedisRefreshStore 시그니처 | refresh.go:406 | `(addr string)` | MATCH |
| audit.NewRecorder 시그니처 | recorder.go:46 | `(authEnabled bool)` | MATCH |
| workflow.NewTxCoordinator 시그니처 | transaction.go:26 | `(store, recorder)` | MATCH |
| workflow.NewStateMachine 시그니처 | state_machine.go:63 | `(coordinator, logger)` | MATCH |
| server.NewWorkflowService 시그니처 | grpc_server.go:51 | `(store, sm, logger)` | MATCH |
| server.NewRESTHandler 시그니처 | rest_handler.go:33 | `(svc, logger)` | MATCH |
| RESTHandler.Mux() 시그니처 | rest_handler.go:94 | `() http.Handler` | MATCH |
| auth.BuildRESTChain 시그니처 | chain.go:43 | `(handler, validator, recorder, authEnabled)` | MATCH |
| auth.BuildGRPCInterceptorChain 시그니처 | chain.go:86 | `(validator, recorder, authEnabled)` | MATCH |
| scheduler.RedisClient 인터페이스 | dispatcher.go:24~29 | `RPush(int64,error) + Ping(error)` | MATCH |
| goRedisAdapter 존재(test-only) | e2e_test.go:199~209 | S0 promote 대상 | CONFIRMED |
| PgWorkflowStore.Ping 부재 | pg_store.go:38~81 | S0 추가 대상 | CONFIRMED |
| JWKSCache.Reachable 부재 | jwks_cache.go:118~218 | S0 추가 대상 | CONFIRMED |
| cacheAge() mu.RLock 계약 | jwks_cache.go:212 | `"호출자가 mu.RLock을 보유해야 한다"` | CONFIRMED |
| audit 신규 action const 부재 | audit.go:13~41 | S0 추가 대상 | CONFIRMED |
| cmd/server/server.go stub | server.go:34~40 | 전면 재작성 대상 | CONFIRMED |

---

## Recommendations

1. **[Run phase 진입 전 필수]** `cmd/server/server.go` 전면 재작성 시 첫 번째 변경으로 `package server` → `package main` 전환. `main.go`와 동일 디렉토리이므로 Go 빌드 규칙상 필수. 빌드 실패 조건.

2. **[S3 구현 시 확인]** `probes.go` livenessHandler 응답 body가 `rest_handler.go`의 기존 `handleHealth` 응답과 format 일치하는지 확인. 두 핸들러가 동일 경로(`/health`)에 등록되며 outer mux shadow 동작으로 probes.go 버전이 우선하나, 테스트가 어느 핸들러를 호출하는지에 따라 format 불일치 버그 가능.

3. **[S0 구현 시 확인]** `go.mod`에서 `github.com/redis/go-redis/v9`가 현재 indirect인지 direct인지 확인 후 `redis_adapter.go` production import 시 direct require 승격 필요 (spec.md §6 / plan.md S0 step 6 명시).

4. **[S2 구현 시 확인]** double-signal force-kill 구현 시 `grpcServer.Stop()` 과 `grpcServer.GracefulStop()` 동시 호출 가능성 — plan.md Risk Register 대응책(`time.AfterFunc` timer.Stop() + done chan) 반드시 적용. panic 위험.

---

## Variance from plan-auditor (0.92)

| 항목 | plan-auditor 0.92 | evaluator-active 0.83 | 차이 원인 |
|------|-------------------|-----------------------|-----------|
| 평가 기준 | Spec 문서 품질 (Clarity/Completeness/Testability/Traceability) | 구현 준비도 (Functionality/Security/Craft/Consistency) | 차원 다름 |
| Craft 80점 | 미반영 | 85% coverage 달성 불확실성 반영 | -4pt |
| 신규 Info 발견 2건 | 미포함 | pkg 이름 변경, /health 중복 | 점수 미반영(Info급) |
| 전체 | 0.92 | 0.83 | -0.09 |

차이는 Info급 발견 2건과 평가 차원 차이에 기인. SPEC 자체의 정합성은 confirmed.

---

## Run Phase 진입 권고

**APPROVED** — spec-to-code 모순 없음. 모든 검증된 API가 실제 코드와 일치. S0 deliverables 정확히 식별. 발견된 2건은 Info급으로 blocking 없음. Run phase 진입 가능.

단, 구현자는 위 Recommendations 4항을 인지하고 진입할 것.
