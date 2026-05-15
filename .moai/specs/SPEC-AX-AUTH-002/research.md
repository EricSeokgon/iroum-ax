# SPEC-AX-AUTH-002 Research Notes

> Phase: Plan (Research sub-phase)
> Author: manager-spec (Opus 4.7, extended reasoning applied to 3 architectural decisions)
> Created: 2026-05-15

본 문서는 SPEC-AX-AUTH-002 spec.md / plan.md 작성을 뒷받침하는 외부 참조 자료, 의사결정 근거, AUTH-001과의 관계, 그리고 default-deny 패턴의 출처를 담는다.

---

## 1. Reference Implementations

### 1.1 Kubernetes RBAC — Verb/Resource 매트릭스 패턴

- 출처: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
- 핵심 패턴:
  - Verb(get/list/create/delete) + Resource(pods/services) 조합 → Role
  - RoleBinding으로 user/group ↔ Role 매핑
  - Admission webhook이 핸들러 진입 전 권한 결정 (사전 차단)
- 본 SPEC과의 매핑:
  - HTTP Method(POST/GET/DELETE) + Path(/api/v1/workflows) → Permission 매트릭스
  - 미들웨어가 admission webhook 역할 (REQ-AUTH2-UBI-001-c 사전 차단)
- 차이점: Kubernetes는 dynamic RBAC (etcd에 저장된 RoleBinding 수정 가능), 본 SPEC은 코드 명시(immutable source-of-truth, REQ-AUTH2-001-U1).

### 1.2 AWS IAM Policy — Action 매핑 + Default-Deny

- 출처: https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_evaluation-logic.html
- 핵심 패턴:
  - Action(s3:GetObject) → Resource(arn:aws:s3:::bucket/*) 매핑
  - Default-deny: 명시적 Allow 없으면 거부 (본 SPEC REQ-AUTH2-001-U1과 동일 철학)
  - Explicit Deny가 Allow를 override
- 본 SPEC 영향:
  - "매핑 미정의 = 503" 결정의 근거 (default-deny safety net)
  - AUTH-001 매트릭스가 explicit allow 역할; unknown method/path는 implicit deny → 503

### 1.3 OPA (Open Policy Agent) — 검토 후 거부

- 출처: https://www.openpolicyagent.org/
- 검토 사유: declarative policy engine, Rego 언어로 권한 규칙 표현
- 본 SPEC 거부 사유:
  - 매핑이 코드 명시(immutable)로 충분 — Rego의 dynamic capability 불필요
  - 운영 hot-reload는 본 SPEC 의도적 거부 (REQ-AUTH2-001-U1)
  - 추가 의존성(OPA sidecar 또는 lib) — 망분리 + Go-only 단순성 우선
  - 후속 SPEC에서 ABAC 도입 시 재검토 가능
- 결정: code-as-config (Go source 매핑 table) 채택

### 1.4 gRPC Interceptor Chaining — go-grpc-middleware

- 출처: https://github.com/grpc-ecosystem/go-grpc-middleware
- 핵심 패턴: `grpc.ChainUnaryInterceptor(auth, ratelimit, logging, handler)`
- 본 SPEC 활용:
  - `grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor(...), server.UnaryAuthzInterceptor(...))` 순서
  - chain order = 권한 결정의 사전 차단 보장 (REQ-AUTH2-UBI-001-c)
- 표준 패턴이므로 외부 lib 추가 없이 `google.golang.org/grpc`의 ChainUnaryInterceptor 사용

### 1.5 chi/middleware (Go HTTP middleware reference)

- 출처: https://github.com/go-chi/chi
- 본 SPEC 직접 의존 없음 (현재 stdlib `net/http` 사용)
- 참조: middleware decorator 패턴 (`func(http.Handler) http.Handler`)이 본 SPEC `RESTAuthzMiddleware` 시그니처와 동일

### 1.6 AUTH-001 rbac.go — 본 SPEC의 직접 의존

- 출처: `apps/control-plane/internal/auth/rbac.go` (SPEC-AX-AUTH-001 v0.1.1 GREEN)
- Exports 활용:
  - `auth.Authorize(ctx, requiredPerm Permission) error` — 단일 호출점
  - `auth.LogForbidden(ctx, tx, method, path, userID, grantedRoles)` — audit 기록
  - `auth.ParseRolesFromScope(scope string) []Role` — 본 SPEC에서 직접 호출 안 함 (Authorize 내부에서 호출)
  - `auth.EffectivePermissions(roles []Role) map[Permission]bool` — 동일
- 본 SPEC은 위 함수들을 wiring 계층에서 호출만 함 (rbac.go 변경 0건).

---

## 2. Default-Deny Pattern Justification

### 2.1 Why Default-Deny?

본 SPEC의 핵심 보안 결정 중 하나. 매핑 미정의 method/path를 어떻게 처리할지의 결정.

**옵션 비교**:

| 옵션 | 응답 | Pro | Con |
|------|------|-----|-----|
| A: Default-deny 503 (본 SPEC 채택) | 503 + AUTHZ_MAPPING_MISSING | 운영 안전(unknown 차단), 개발 단계 즉시 발견 | 정상 endpoint 매핑 누락 시 503 무한 |
| B: Default-allow (요청 통과) | 200 OK | 미정의 path도 동작 | 보안 결함 (open by default) |
| C: Default-deny 403 | 403 Forbidden | 사용자에게 권한 부족 시사 | 매핑 누락이 권한 부족으로 오인 |
| D: 빌드 타임 검증 (라우터 등록 시 매핑 강제) | N/A | 누락 원천 차단 | Go의 `http.ServeMux` 동적 등록과 호환 어려움 |

**선택: A (503 + AUTHZ_MAPPING_MISSING)**

**근거**:
1. **보안 우선**: open-by-default(옵션 B)는 KEPCO 운영 배포 시 보안 결함 직결. 절대 거부.
2. **운영 명확성**: 503은 "서버 일시 문제"를 시사하여 클라이언트가 retry하지 않도록 함. 403은 사용자 권한 issue로 오인되어 디버깅 어려움.
3. **개발 시점 발견**: 503 발생 = 매핑 누락 = 개발자가 즉시 인지. SPEC 변경 트리거.
4. **AWS IAM 패턴 일치**: explicit allow 없으면 deny, 단 본 SPEC은 더 보수적으로 "deny + 개발 결함 시그널".

**Trade-off 수용**: 정상 endpoint 매핑 누락 시 503 발생. mitigation: spec.md §3.2 매트릭스가 single source-of-truth, 새 endpoint SPEC은 본 매트릭스 업데이트 의무.

### 2.2 Why 503, not 500?

- 500 Internal Server Error: "서버 내부 버그" 의미. 매핑 누락은 설계 단계 누락이므로 503이 더 정확.
- 503 Service Unavailable: "일시적 미가용" 의미. Retry-After 헤더로 운영자 alert 트리거 가능.
- 본 SPEC: `Retry-After: 0` 또는 미설정 (즉시 retry 무의미; 운영자 개입 필요).

---

## 3. Method-Permission Mapping Placement Decision

### 3.1 Architectural Decision: Middleware Lookup Table vs Handler-Embedded

본 SPEC의 두 번째 architectural decision.

**옵션 비교**:

| 옵션 | 위치 | Pro | Con |
|------|------|-----|-----|
| A: Middleware lookup table (본 SPEC 채택) | `authz.go` 중앙 매핑 | 단일 source-of-truth, 테스트 용이, 변경 추적 명확 | wiring 추가 1 layer |
| B: Handler-embedded (`handler.go` 각 핸들러 진입 시 `auth.Authorize` 호출) | 각 핸들러 내부 | 핸들러 단위 fine-grained 가능 | 누락 위험 (개발자가 각 핸들러마다 호출 잊음), 테스트 분산 |
| C: Annotation/Decorator (`@RequiresPermission("write:workflow")` 형태) | 핸들러 메타데이터 | Java/C# 스타일 명확성 | Go에는 표준 annotation 없음, struct tag로 우회 가능하나 reflection 비용 |

**선택: A (Middleware lookup table)**

**근거**:
1. **누락 차단**: 옵션 B는 개발자가 각 핸들러에 `auth.Authorize` 호출을 직접 추가해야 하며, 새 핸들러 추가 시 잊을 위험. 미들웨어 일괄 적용은 누락 방지.
2. **테스트 용이성**: 단위 테스트(매핑 lookup) + 미들웨어 통합 테스트로 분리하여 검증 — lessons #4 (stub-assert anti-pattern 회피)와 부합.
3. **REQ-AUTH2-UBI-001-c 보장**: 미들웨어가 next 호출 전에 Authorize 실행 → 핸들러 진입 전 차단 invariant 자동 보장.
4. **REQ-AUTH2-001-U1 default-deny 자연 구현**: lookup miss = default-deny → 503 분기가 미들웨어 내 단일 지점.
5. **Go idiom 부합**: stdlib `net/http`와 `google.golang.org/grpc` 모두 middleware decorator + interceptor chain 패턴 권장.

**거부 사유**:
- 옵션 B: 누락 위험. 본 SPEC이 보안 critical이므로 누락 = 실패.
- 옵션 C: Go에 annotation 표준 없음. Struct tag + reflection은 성능(NFR p99 < 100µs) 위협.

---

## 4. Cross-SPEC Artifact Impact (lessons #5 적용)

본 SPEC이 SPEC-AX-AUTH-001의 artifact에 영향을 미치는 부분을 명시.

### 4.1 SPEC-AX-AUTH-001 영향

| Artifact | 변경 내용 | 변경 책임 SPEC |
|----------|----------|---------------|
| `apps/control-plane/internal/server/auth_e2e_test.go` `TestE2E_Auth_RBACForbidden` | `t.Skip(...)` 제거 + 3 시나리오 활성화 | SPEC-AX-AUTH-002 S3 |
| `.moai/specs/SPEC-AX-AUTH-001/acceptance.md` §6 AC-AUTH-E2E-3 | status 마커 `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)` 추가 | SPEC-AX-AUTH-002 S3 (acceptance.md edit) |

### 4.2 SPEC-AX-CTRL-001 영향

본 SPEC은 핸들러 비즈니스 로직 변경 없음(미들웨어만 추가). CTRL-001 AC 영향 없음 — 단, AuthEnabled=false 시 모든 CTRL-001 AC unchanged 통과 invariant 필수(REQ-AUTH2-UBI-001-b).

### 4.3 SPEC-AX-001 영향

audit_logs schema 변경 없음. `cli-anonymous` 폴백 보존(REQ-AUTH2-UBI-001-b로 자동 보장).

### 4.4 OpenAPI Schema (schemas/openapi/openapi.yaml)

- AUTH-001에서 401 응답 정의됨. 본 SPEC에서 403 + `WWW-Authenticate: insufficient_scope` 헤더 정의 추가.
- 503 응답(default-deny mapping missing) 별도 정의 추가.
- 본 변경은 sync phase에서 manager-docs가 처리(본 SPEC S3 deliverable).

---

## 5. Architectural Decision Log (Top 3)

### Decision 1: Mapping Placement — Middleware Lookup Table (vs Handler-Embedded vs Annotation)

상세는 §3 참조. 채택: **Middleware lookup table** (authz.go 중앙 집중).

### Decision 2: Default-Deny Response Code — 503 (vs 500 vs 403 vs Build-time check)

상세는 §2 참조. 채택: **503 Service Unavailable + AUTHZ_MAPPING_MISSING**.

### Decision 3: AuthEnabled=false Handling — Skip via Constructor Flag (vs validator-nil check vs Middleware exclusion)

본 SPEC의 세 번째 decision.

**옵션 비교**:

| 옵션 | 구현 방식 | Pro | Con |
|------|----------|-----|-----|
| A: Constructor flag `authEnabled bool` (본 SPEC 채택) | `RESTAuthzMiddleware(recorder, authEnabled bool)` 명시 인자 | 명확성, 테스트 용이 (mock 가능) | 호출 site 변경 |
| B: `validator == nil` 체크로 추론 | middleware 내부에서 user context 부재 시 skip | 코드 변경 최소 | 의도 불명확 (validator nil 의미 모호) |
| C: Wiring 단계에서 middleware chain에서 제외 | `cmd/server/server.go`에서 if문 | middleware 자체가 unaware, 단순 | wiring 누락 위험 |

**선택: A + C 하이브리드**

**근거**:
- A 옵션의 explicit flag로 의도 명확화 + 단위 테스트에서 두 모드 모두 검증 가능
- C 옵션을 보조적으로 활용: `cmd/server/server.go`에서 `if cfg.AuthEnabled { chain authz middleware }` 패턴으로 production wiring 단순화
- AuthEnabled=false 시 middleware 자체 진입조차 안 함 → 성능 최적 + REQ-AUTH2-UBI-001-b 자연 보장
- 단위 테스트는 middleware 직접 호출하여 `authEnabled=false` 동작도 검증 (defense in depth)

**거부 사유**:
- 옵션 B 단독: validator nil 의미가 "auth disabled"인지 "validator 초기화 실패"인지 모호. explicit flag가 안전.

---

## 6. Risk Register Detail

### R-AUTH2-001 — 매핑 누락 default-deny 미동작

**Likelihood**: Low (S0 단위 테스트로 강제 검증)
**Impact**: Critical (open-by-default 시 보안 결함)
**Detection**: S0 `authz_test.go` unknown method/path 명시 테스트 + S1/S2 E2E `AC-AUTH2-001-2/5`
**Mitigation**:
- 단위 테스트가 unknown 경로 명시 검증
- `resolveRESTPermission` / `resolveGRPCPermission` 반환값에 `mapped bool` 명시 → caller가 분기 강제
- `// @MX:WARN` + `// @MX:REASON: 매핑 미정의 시 fail-closed 보장` 코드 마커
**Residual Risk**: Very Low

### R-AUTH2-002 — Interceptor chain 순서 오류

**Likelihood**: Medium (개발자 wiring 실수)
**Impact**: High (authz가 먼저면 user context 미주입 → 500 무한)
**Detection**: AC-AUTH2-003-6 (chain order 검증), S2 통합 테스트
**Mitigation**:
- `cmd/server/server.go`에 `@MX:NOTE: chain order: auth → authz → handler`
- 빌드 시 lint check (별도 검증 가능)
- bufconn 통합 테스트가 잘못된 토큰 시 `codes.Unauthenticated` 확정 (authz 전 차단 검증)
**Residual Risk**: Low

### R-AUTH2-003 — AuthEnabled=false 백워드 호환 회귀

**Likelihood**: Medium (wiring 추가 시 의도치 않은 경로 변경)
**Impact**: High (SPEC-AX-CTRL-001 + SPEC-AX-AUTH-001 회귀 → 본 SPEC 차단)
**Detection**:
- `TestE2E_Auth_AnonymousFallback` regression 가드
- AC-AUTH2-UBI-001-b dedicated AC
- AC-AUTH2-003-5 gRPC 측 검증
**Mitigation**:
- `cmd/server/server.go`에서 `cfg.AuthEnabled=false` 시 middleware chain에서 제외 (옵션 C)
- 미들웨어 자체에 `authEnabled bool` explicit flag (옵션 A) — defense in depth
- CI에서 본 SPEC 진행 중에도 SPEC-AX-AUTH-001 전체 테스트 통과 의무
**Residual Risk**: Low

### R-AUTH2-004 — Audit row 중복 (grant + handler audit)

**Likelihood**: Medium (REQ-AUTH2-UBI-001-a 해석 모호 시)
**Impact**: Medium (audit log noise, PISA 컴플라이언스 영향 미미)
**Detection**: AC-AUTH2-UBI-001-a2 (grant 시 별도 row 미생성)
**Mitigation**:
- REQ-AUTH2-UBI-001-a 명시: grant는 context annotation만, deny만 별도 row
- 핸들러 audit row에 `details.granted_permission` 통합
- 단위 테스트가 grant 시 audit row count 검증
**Residual Risk**: Low

### R-AUTH2-005 — 핸들러 진입 후 차단 (사전 차단 invariant 위반)

**Likelihood**: Low (middleware decorator 패턴 자체가 사전 차단)
**Impact**: Critical (비즈니스 부작용 발생 후 차단 → transactional integrity 위협)
**Detection**: AC-AUTH2-UBI-001-c (workflows count 변화 0건 검증)
**Mitigation**:
- middleware/interceptor가 next/handler 호출 전에 Authorize 명시
- E2E 테스트가 forbidden 시 DB 변화 없음 검증
**Residual Risk**: Very Low

### R-AUTH2-006 — TestE2E_Auth_ConcurrentRequests race

**Likelihood**: Low (lookup table은 read-only)
**Impact**: Medium (race detector positive 시 PR 차단)
**Detection**: `go test -race` 강제
**Mitigation**:
- 매핑 테이블은 init 시 1회 생성 후 read-only (mutation 없음)
- middleware/interceptor는 stateless decorator
- AC-AUTH2-004-4가 명시적으로 동시성 검증
**Residual Risk**: Very Low

### R-AUTH2-007 — DELETE 핸들러 부재로 admin 시나리오 모호

**Likelihood**: Medium (DELETE 핸들러는 후속 SPEC 범위)
**Impact**: Low (테스트 단언 명확성만 영향)
**Detection**: AC-AUTH2-004-3 정의 시 발견
**Mitigation**:
- AC 단언을 "403이 아니다" + "AUTH_FORBIDDEN row 미생성"으로 제한
- 절대값(예: "200 OK + WORKFLOW_DELETED row") 회피 — lessons #4 stub-assert anti-pattern 회피 원칙 적용
**Residual Risk**: Low

### R-AUTH2-008 — evaluator-active 보수성 score 편차

**Likelihood**: High (lessons #7 일관 -0.05~-0.10 편차 관찰)
**Impact**: Medium (thorough harness 충족 위협)
**Detection**: plan-auditor + evaluator-active 교차 검증
**Mitigation**:
- thorough harness 적용 (security-critical SPEC)
- plan-auditor 0.85+ 목표 → evaluator -0.10 buffer 후 ≥ 0.75 보존
- Security dimension 강화 (default-deny + 사전 차단 + 백워드 호환 invariant 3중 명시)
- Must-pass criteria: AC-AUTH2-UBI-001-b 백워드 호환 + AC-AUTH2-004-1 SKIP unblock
**Residual Risk**: Medium (수용, 보정 가능)

---

## 7. Cognitive Bias Check

- **Anchoring bias**: AUTH-001 패턴(verifier/middleware/RBAC lib)이 익숙해서 "wiring 추가만" 단순 결론으로 끝낼 위험. **Mitigation**: §3 default-deny + middleware vs handler-embedded decision을 명시 평가.
- **Confirmation bias**: "AUTH-001 GREEN이니 통합도 쉽다" 가정 → AuthEnabled=false 백워드 호환 회귀 위험 누락 위험. **Mitigation**: R-AUTH2-003 명시 + AC-AUTH2-UBI-001-b dedicated AC.
- **Sunk cost bias**: AUTH-001 Sprint 7에서 RBAC E2E SKIP한 결정을 부정하려는 충동 → 본 SPEC scope 과확장 위험. **Mitigation**: §5 Exclusion 9개 명시(ABAC/per-resource/cell-level/위임 등), spec.md §7 Out of Scope에서 DELETE 핸들러/Python wiring 명시 분리.
- **Overconfidence**: "in-process wiring이라 token budget 작다" → AUTH-001(1.2M)과 동일 4-sprint 패턴이라 ~800K 예상하지만 evaluator iter 추가 시 1M 가능. **Mitigation**: plan.md §7 보수적 추정.

**This option might fail because**:
- (a) AuthEnabled=false 시 middleware exclusion 패턴(Decision 3 옵션 C)에서 `cmd/server/server.go` wiring 누락 → mitigation: AC-AUTH2-UBI-001-b가 E2E 가드로 강제 검증.
- (b) DELETE 핸들러가 본 SPEC 범위 외인데 E2E E3 시나리오가 admin DELETE를 검증 → mitigation: AC를 "403 아님 + AUTH_FORBIDDEN 미생성"으로 제한, 절대값 회피.
- (c) AUTH-001 acceptance.md edit이 cross-SPEC artifact 변경 → mitigation: lessons #5 적용으로 plan.md §6 cross-SPEC impact 섹션에 명시.

---

## 8. References (External)

| Source | Topic | URL/Path |
|--------|-------|----------|
| `tech.md` §9.1, §9.4 | 망분리, PISA, 감사 로그 | `.moai/project/tech.md` |
| `product.md` §4.1, §6.1 | 페르소나(경영평가팀 5-10명), KEPCO E&C Go-Live | `.moai/project/product.md` |
| SPEC-AX-AUTH-001 §3.5 REQ-AUTH-004 | 3-role 매트릭스 (본 SPEC 신뢰) | `.moai/specs/SPEC-AX-AUTH-001/spec.md` |
| SPEC-AX-AUTH-001 acceptance.md §6 AC-AUTH-E2E-3 | SKIP unblock 대상 | `.moai/specs/SPEC-AX-AUTH-001/acceptance.md` |
| SPEC-AX-AUTH-001 auth_e2e_test.go:354~371 | TestE2E_Auth_RBACForbidden SKIP marker | `apps/control-plane/internal/server/auth_e2e_test.go` |
| SPEC-AX-AUTH-001 rbac.go | Authorize / LogForbidden / ParseRolesFromScope exports | `apps/control-plane/internal/auth/rbac.go` |
| SPEC-AX-AUTH-001 middleware.go | UnaryServerInterceptor / RESTMiddleware | `apps/control-plane/internal/auth/middleware.go` |
| SPEC-AX-CTRL-001 §3.1 | gRPC + REST handler 구조 | `.moai/specs/SPEC-AX-CTRL-001/spec.md` |
| SPEC-AX-001 §3.1 REQ-UBI-003 | cli-anonymous fallback | `.moai/specs/SPEC-AX-001/spec.md` |
| Kubernetes RBAC docs | Verb/Resource 매트릭스 + Admission webhook | https://kubernetes.io/docs/reference/access-authn-authz/rbac/ |
| AWS IAM Policy Evaluation Logic | Default-deny pattern | https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_evaluation-logic.html |
| OPA (Open Policy Agent) | Declarative policy engine (rejected) | https://www.openpolicyagent.org/ |
| go-grpc-middleware | gRPC interceptor chaining patterns | https://github.com/grpc-ecosystem/go-grpc-middleware |
| RFC 6750 §3 | WWW-Authenticate header for insufficient_scope | https://datatracker.ietf.org/doc/html/rfc6750 |
| OAuth 2.0 BCP (RFC 9700) §4.3 | RBAC + scope-based authz patterns | https://datatracker.ietf.org/doc/html/rfc9700 |
| lessons_session_2026_05_14.md | 8 patterns from previous 3 SPECs | `~/.claude/projects/-home-sklee-moai-iroum-ax/memory/` |

---

## 9. Open Questions (sensible default 있음 — autonomous 진행 가능)

> [HARD] manager-spec(본 agent)은 AskUserQuestion 호출 금지. 본 SPEC은 autonomous mode이므로 sensible default로 진행.

### Q1: `/metrics` endpoint 인증 정책 — **DEFERRED to SPEC-AX-OBS-001** (v0.1.2 Option C)

- 질문: Prometheus scrape endpoint `/metrics`를 완전 bypass할 것인가, admin-only로 제한할 것인가?
- ~~옵션 A: bypass — Prometheus는 K8s 내부망에서 scrape하며 별도 network policy로 차단됨~~
- ~~옵션 B: admin only — service account token으로 인증~~ (iter 2 D4에서 일시 적용했으나 iter 3에서 분리)
- **v0.1.2 (iter 3) 결정**: **DEFERRED** — 본 SPEC 범위에서 `/metrics` 완전 제거. 사유: AUTH-001 `rbac.go` permissionMatrix에 `read:metrics`가 미정의이며 `rest_handler.go`에도 `/metrics` 핸들러가 미등록 상태로(evaluator-active iter 3 검증 결과), 본 SPEC scope에서 `/metrics` 권한 매핑만 추가하면 cross-SPEC 변경(rbac.go + rest_handler.go 동시) 필요 → 명세-코드 모순. 후속 SPEC **`SPEC-AX-OBS-001`** (Observability + Monitoring) 또는 **`SPEC-AX-METRICS-001`**(Prometheus exposition 전용)에서 (a) rbac.go permissionMatrix에 `read:metrics` 추가, (b) rest_handler.go에 `/metrics` handler 등록(promhttp.Handler), (c) authz mapping에 `/metrics → read:metrics` 추가를 한 SPEC scope으로 처리. 임시 운영 보호: K8s NetworkPolicy / Docker network isolation / Helm values 차원 (KEPCO 망분리 환경에서 internal network ACL이 일차 방어선).

### Q2: DELETE 핸들러 구현 시점

- 질문: DELETE /api/v1/workflows/{id} 핸들러를 본 SPEC에서 함께 구현할 것인가?
- 옵션 A (권장 default): NO — 본 SPEC은 RBAC wiring만, DELETE 핸들러는 후속 SPEC `SPEC-AX-WF-DELETE-001`
- 옵션 B: YES — admin DELETE 시나리오 완결성 위해 핸들러도 구현
- 결정: **옵션 A** — scope discipline 유지(lessons #4 적용), AC-AUTH2-004-E3는 RBAC 통과만 검증

### Q3: Python FastAPI RBAC wiring 포함 여부

- 질문: 본 SPEC에서 Python pipelines FastAPI 측 RBAC wiring도 함께?
- 옵션 A (권장 default): NO — 후속 SPEC `SPEC-AX-AUTH-PY-001`로 분리
- 옵션 B: YES — 통합 SPEC
- 결정: **옵션 A** — Go control-plane 우선(KEPCO 운영 진입점), Python은 후속

### Q4: 503 vs 500 for mapping missing

- 질문: default-deny 응답 코드 선택
- 옵션 A (권장 default): 503 + AUTHZ_MAPPING_MISSING
- 옵션 B: 500 Internal Server Error
- 결정: **옵션 A** — §2.2 근거 (운영 명확성)

### Q5: Multiple roles union 처리 (analyst+viewer)

- 질문: 다중 role 토큰 시 권한 결정 알고리즘
- 옵션 A (권장 default): Union (가장 강한 권한 적용) — AUTH-001 REQ-AUTH-004-S1과 일관
- 옵션 B: Intersection (모든 role이 가진 권한만)
- 결정: **옵션 A** — AUTH-001 매트릭스 신뢰, 본 SPEC은 wiring만

---

## 10. Definition of Done (Research Phase)

- [x] 6개 reference implementation 문서화 (K8s RBAC, AWS IAM, OPA-rejected, gRPC interceptor, chi middleware, AUTH-001 rbac.go)
- [x] Default-deny pattern justification (4-way 비교 + 503 vs 500 결정)
- [x] Method-permission mapping placement decision (middleware vs handler vs annotation 3-way)
- [x] AuthEnabled=false 처리 hybrid decision (constructor flag + middleware exclusion)
- [x] Cross-SPEC artifact impact 명시 (AUTH-001 acceptance + e2e_test 변경 책임)
- [x] 8개 Risk(R-AUTH2-001~008) detail + likelihood/impact/mitigation/residual
- [x] Cognitive bias check 4종 (anchoring/confirmation/sunk cost/overconfidence) + "This option might fail because" 3종
- [x] 5개 open question with sensible defaults (autonomous 진행 가능)
- [x] External references table (16 sources)
