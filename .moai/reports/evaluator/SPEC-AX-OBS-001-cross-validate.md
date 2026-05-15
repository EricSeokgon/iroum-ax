# Evaluator-Active Cross-Validation Report
SPEC: SPEC-AX-OBS-001 v0.1.1
Date: 2026-05-15
Harness: standard (plan-auditor iter 2 post)
Context: Independent of plan-auditor 0.97 (M1 Context Isolation)

---

## Evaluation Report

**SPEC: SPEC-AX-OBS-001**
**Overall Verdict: CONFIRM (PASS)**

### Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 75/100 | PASS | 5/7 collectors have verified wiring plans; 2 collectors (auth_rejections, workflow_transitions) lack explicit wiring in §2.1 affected files. Core design (MetricsAuthMiddleware, gRPC ServerOption ordering) verified against chain.go:86-101, middleware.go:149-198, server.go:185-195. |
| Security (25%) | 85/100 | PASS | rbac.go permissionMatrix verified frozen (no read:metrics); MetricsAuthMiddleware 401/403 separation is architecturally sound; OTel default noop 망분리 정합; REQ-OBS-UBI-001-d cardinality bound explicit. |
| Craft (20%) | 75/100 | PASS | 22 AC binary-testable, goleak, bench p99 gate. 2 unwired collectors will emit 0 at run time — tests validating non-zero counts for iroum_ax_auth_rejections_total will mislead. |
| Consistency (15%) | 85/100 | PASS | Mirrors SERVER-001 outerMux chain-external mount pattern; preserves AUTH-001/AUTH-002 frozen assets; authz_mapping.go 1-row addition consistent with existing lookup contract. |

**Weighted Total: 79.0/100**
**Plan-auditor 0.97 대비 variance: −0.18 (spec document quality vs implementation feasibility 측정 차이)**

---

### Findings

**[Moderate] spec.md §3.2 / §2.1 — iroum_ax_auth_rejections_total: circular import 불가**

`internal/metrics/permission.go`는 `auth.TokenValidator`, `auth.Role`, `auth.UserFromContext`, `auth.ParseRolesFromScope`를 import한다 (metrics → auth). 반면 `iroum_ax_auth_rejections_total` 소스는 "auth validator reject hook"으로 지정되며, 이를 구현하려면 `internal/auth/middleware.go`(또는 `validator.go`)에서 `metrics.IncAuthRejection(reason)`을 호출해야 한다 (auth → metrics). 이 방향은 **Go circular import**를 생성한다: `auth/middleware.go` → `metrics` → `auth`. §2.1 affected files에 `internal/auth/middleware.go`가 없는 것은 우연이 아니라 구조적 제약의 반영이나 SPEC은 이를 명시하지 않는다. 결과: 이 counter는 SPEC 범위 내에서 정의되나 **increment 지점이 없다**. Run phase 진입 전 해결 옵션을 SPEC에 명시해야 한다.

- 해결 옵션 A: `auth` 패키지에 rejection counter를 두고 `metrics` 패키지는 이를 등록(소유)만 하지 않는다 — 단 단일 레지스트리 원칙(REQ-OBS-001-U1) 위반 위험
- 해결 옵션 B: dependency inversion — `auth` 패키지에 `RejectionObserver interface`를 두고 metrics가 구현체를 주입 (server.go wiring)
- 해결 옵션 C: MetricsAuthMiddleware가 `/metrics` 경로의 인증 실패만 카운트 (scope 축소 — 전체 API auth 실패 불가시화)

**[Minor] spec.md §3.2 / §2.1 — iroum_ax_workflow_state_transitions_total: wiring 미계획**

소스는 "StateMachine 전이 hook"이나 §2.1 affected files에 `internal/workflow/` 패키지 파일이 없다. `workflow` 패키지가 `metrics`를 import하면 `metrics → auth → (no workflow)` 구조이므로 circular import는 발생하지 않으나, Run phase 구현자가 scope creep 없이 이 파일을 수정해야 한다. 명시적 affected file 누락은 Sprint S2 계획에서 혼란을 야기할 수 있다.

**[Info] authz_mapping.go — /metrics 엔트리 현재 부재 (예상된 상태)**

`restPermissionTable`에 `GET /metrics → read:metrics` 엔트리가 없다. 이는 pre-implementation 상태로 정상. REQ-OBS-002-E2의 self-disclosure("실제 production `/metrics`는 chain 외부이므로 `LookupRESTPermission`을 거치지 않는다")는 정직한 caveat이며 결함이 아니다.

**[Info] go.mod — prometheus/client_golang, otel/sdk 부재 (예상된 상태)**

`prometheus/client_golang` absent, `otel v1.43.0` indirect → S0 deliverable. Spec §2.0 correctly documents this.

**[Info] MetricsAuthMiddleware 401 body format — auth.RESTMiddleware와 상이 (의도적)**

`auth.RESTMiddleware` writeAuthError → `{"error": "missing_authorization"}` (string value)  
MetricsAuthMiddleware REQ-OBS-002-U1 → `{"error":{"code":"UNAUTHENTICATED","message":"..."}}` (object)  
Spec §2.0 및 §3.3 design note에서 명시적으로 인지됨. 의도적 차이이며 결함 아님.

---

### Spec-to-Code 정합 결과 (plan-auditor 미발견 항목)

| 검증 항목 | 결과 | 근거 파일:라인 |
|-----------|------|----------------|
| rbac.go permissionMatrix — read:metrics 미존재 (frozen 확인) | PASS (spec 정합) | rbac.go:39-60 |
| auth.BuildGRPCInterceptorChain — grpc.ServerOption 반환 확인 | PASS | chain.go:86-101 |
| auth.RESTMiddleware — /health bypass only, /metrics 미커버 확인 | PASS | middleware.go:149-156 |
| auth.WithUser / UserFromContext / TokenValidator.Verify 시그니처 | PASS | middleware.go:40-52, 131, 182 |
| server.go outerMux — /health, /ready chain 외부 mount 패턴 | PASS | server.go:185-195 |
| server.go s.tokenValidator 필드 및 wiring | PASS | server.go:47, 135 |
| PgWorkflowStore.PoolStats() — *pgxpool.Stat 반환 | PASS | pg_store.go:92-94 |
| CeleryDispatcher.Dispatch — (ctx,workflowID,documentID) error | PASS | dispatcher.go:70 |
| go.mod — prometheus/client_golang 부재 (S0 추가 대상) | PASS (spec 정합) | go.mod:5-17 |
| go.mod — otel v1.43.0 indirect 존재 | PASS (spec 정합) | go.mod:67 |
| **iroum_ax_auth_rejections_total 계측 지점** | **FAIL — circular import** | §2.1 affected files 누락 |
| **iroum_ax_workflow_state_transitions_total 계측 지점** | **UNVERIFIED — no wiring plan** | §2.1 affected files 누락 |

---

### Recommendations

1. **[Run 진입 전 Required] auth_rejections_total 아키텍처 결정**: metrics → auth circular import 해결 방안을 SPEC amendment 또는 plan.md S2 주석으로 명시할 것. 권장: Option B (RejectionObserver interface DI via server.go wiring) — auth 패키지가 `auth.RejectObserver interface { IncAuthRejection(reason string) }` 를 선언하고 server.go가 metrics 구현체를 주입. metrics → auth 의존 방향을 역전.

2. **[Run 진입 전 Recommended] §2.1 affected files에 workflow/state_machine.go 추가**: `IncWorkflowTransition(from, to)` 계측 지점을 명시해야 Run phase 구현자가 scope boundary를 명확히 알 수 있다.

3. **[Run 진입 가능] 나머지 설계 결정 모두 검증 완료**: MetricsAuthMiddleware(Option A) 실제 구현 가능성, gRPC 최외곽 chain 조합, PoolStats API, CeleryDispatcher.Dispatch API 모두 실제 코드와 정합.

---

## Run Phase 진입 가능 여부

**CONDITIONAL PASS — Run 진입 가능, 단 auth_rejections_total 아키텍처 결정 필요**

7개 core collector 중 5개(ObserveHTTP, ObserveGRPC, IncCeleryDispatch, SetPgPoolConns, IncAuthzForbidden)는 명확한 wiring plan을 보유하며 실제 코드 API와 정합. 핵심 보안 설계(MetricsAuthMiddleware authn/authz, gRPC ServerOption 순서, rbac.go frozen 보존)는 실제 코드 검증 완료.

2개 collector의 wiring gap은 Sprint S2 착수 전 별도 resolution이 권장된다. circular import 미해결 시 `iroum_ax_auth_rejections_total`은 정의만 존재하는 dead counter가 된다. 이는 §4 비기능 요구사항("테스트 커버리지 ≥ 85%") 달성에는 영향이 없으나, 운영 가시성(brute-force 탐지, auth 에러율 SLA)에 직접 영향을 준다.
