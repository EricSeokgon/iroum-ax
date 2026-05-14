# SPEC-AX-AUTH-001 Implementation Strategy — Phase 1 (manager-strategy)

> **Plan mode 활성 안내**: 본 문서는 본래 `.moai/specs/SPEC-AX-AUTH-001/strategy.md`로 작성되어야 하나, 현재 plan 모드가 활성화되어 있어 지정 plan 파일 경로(`/home/sklee/moai/iroum-ax/.moai/plans/enumerated-plotting-manatee-agent-a2efa36b7dcc5dcaf.md`)에 작성한다. plan 모드 해제 후 orchestrator가 동일 내용을 `.moai/specs/SPEC-AX-AUTH-001/strategy.md`로 이동/재생성해야 한다. 이는 SPEC-AX-001 / SPEC-AX-CTRL-001 strategy.md가 따랐던 패턴과 동일 (lessons #3 적용).

- 작성일: 2026-05-15
- 작성자: manager-strategy (Opus 4.7, Adaptive Thinking 활성, 자동 모드)
- 대상 SPEC: SPEC-AX-AUTH-001 v0.1.1 (draft, plan-auditor iter 1 PASS 0.88 + evaluator-active CONFIRM 0.782 후속 보정 완료)
- 개발 방법론: TDD (RED-GREEN-REFACTOR) per `quality.yaml development_mode: tdd`
- Harness 레벨: thorough (per-sprint evaluator-active, sprint_contract 필수)
- 목표: SPEC-AX-AUTH-001 SSO/JWT 인증 + 3-role RBAC 구현 전략 — Phase 2 RED 진입 전 Sprint 분해·시퀀싱·라이브러리 결정·Sprint Contract scaffold·리스크 mitigation·백워드 호환 invariant 확정

---

## 0. Phase 1 산출물 요약 (Quick Reference)

| 항목 | 결정 사항 |
|------|----------|
| Sprint 총수 | **8** (Pre-Sprint 0 환경 검증 + S0 Foundation + S1 JWT Validator + S2 OIDC Discovery + S3 Middleware Integration + S4 REST + FastAPI + Celery Envelope + S5 RBAC + S6 Refresh/Logout + S7 E2E) |
| Critical path REQ ordering | go deps + python deps 추가 → pkg/auth/claims 공통 → REQ-AUTH-001(JWT 검증, SF-1 iss + SF-2 alg cross-check) → REQ-AUTH-002(OIDC Discovery) → REQ-AUTH-003 gRPC interceptor → REQ-AUTH-003 REST middleware + FastAPI Depends + Celery envelope.headers.user_id → REQ-AUTH-004(RBAC) → REQ-AUTH-005(Refresh/Logout) → E2E (AC-AUTH-E2E-1/2) |
| Top 3 risks | R-AUTH-007 백워드 호환성 regression (Python 177 + Go 95 = 272 tests), R-AUTH-006 Algorithm Confusion Attack (alg cross-check), R-AUTH-001 JWKS 미가용 |
| Foundation deps 개수 | **Go 3개** (golang-jwt/jwt/v5, coreos/go-oidc/v3, MicahParks/keyfunc/v3) + **Python 2개** (PyJWT[cryptography], authlib) — 본 SPEC research.md §1.4는 PyJWT 채택을 명시했으므로 `python-jose` 대신 **PyJWT 2.8** 채택 |
| Cross-SPEC artifact regen (Sprint 4) | celery_envelope.go `headers.user_id` 필드 추가 + `testdata/celery_envelope_v2.json` golden file 재생성 + SPEC-AX-CTRL-001 AC-CTRL-005-1 회귀 가드 (15 dispatcher tests) |
| Open question 수 | 5 (모두 sensible default 적용 — 자동 모드 진입, AskUserQuestion 호출 없음) |
| Phase 2 RED entry | **YES** (모든 sensible default 적용 가능, 백워드 호환 baseline은 S0에서 1회만 작성 후 S3 종료 시 회귀 검증) |

> **주요 차이점 vs plan.md S0~S6**: plan.md는 7개 sub-sprint(S0 Scaffolding → S1 Verifier → S2 OIDC → S3 Middleware → S4 RBAC → S5 Refresh/Logout → S6 E2E + Deployment)로 분해했다. 본 strategy는 **Pre-Sprint 0 환경 검증**을 명시 추가하고, plan.md S3(Middleware Integration)을 **S3 gRPC** + **S4 REST/FastAPI/Celery envelope**으로 분할한다. 근거는 §1.2 참조 — gRPC interceptor와 REST middleware는 의존성 사슬이 다르고(Celery envelope user_id 필드 추가가 SPEC-AX-CTRL-001 cross-SPEC artifact 재생성을 수반), 단일 sprint로 묶을 경우 evaluator-active scoring이 must-pass criteria 분산 위험.

---

## 1. Sprint Sequencing (REQ Dependency DAG → Sprint Order)

### 1.1 Topological order

```
[Pre-Sprint 0: Environment Verification]
        ↓
[Sprint 0 AUTH: Foundation]
   - Go deps 추가 (golang-jwt/jwt/v5 v5.2.x, coreos/go-oidc/v3 v3.10.x, MicahParks/keyfunc/v3 v3.x)
   - Python deps 추가 (PyJWT[cryptography] ^2.8.0, authlib ^1.3.0, httpx ^0.27.0)
   - pkg/auth/claims.go + pkg/auth/claims.py Claims 구조체 공유 정의
   - apps/control-plane/internal/auth/ 디렉토리 + 8개 stub 파일 (verifier/jwks_cache/oidc_client/blacklist/rbac/context/refresh_family/errors)
   - pipelines/auth/ 디렉토리 + 7개 stub (verifier/oidc_client/dependencies/blacklist/context/errors/__init__)
   - apps/control-plane/internal/config/config.go에 OIDC 필드 4개 추가 (OIDCIssuerURL, OIDCAudience, OIDCJWKSCacheTTL, RBACScopePrefix)
   - pipelines/config/settings.py에 OIDC 필드 4개 추가
   - AuthEnabled=false 분기 wiring (모든 신규 미들웨어가 no-op 동작)
   - deployments/helm/keycloak/ 디렉토리 + values.yaml stub
   - tests/integration/test_auth_backward_compat.py 골격 (regression baseline)
        ↓
[Sprint 1 AUTH: REQ-AUTH-001 JWT Validator + SF-1 iss + SF-2 alg cross-check]
   - internal/auth/verifier.go (golang-jwt/jwt/v5, allow-list [RS256, EdDSA, ES256])
   - internal/auth/jwks_cache.go (keyfunc/v3 wrapping, 1h hard TTL + stale-while-revalidate)
   - internal/auth/blacklist.go (Redis SET + EXPIREAT)
   - internal/auth/errors.go (ErrTokenExpired, ErrTokenInvalid, ErrTokenBlacklisted, ErrInsufficientScope, ErrInvalidIssuer, ErrAlgorithmKeyMismatch)
   - pipelines/auth/verifier.py (PyJWT + PyJWKClient)
   - pipelines/auth/blacklist.py + errors.py
   - 검증 항목: exp/nbf/iat/aud/iss (SF-1) + alg/kty cross-check (SF-2)
        ↓
[Sprint 2 AUTH: REQ-AUTH-002 OIDC Discovery]
   - internal/auth/oidc_client.go (coreos/go-oidc/v3 Provider 활용, startup-time fetch, 10s timeout → panic)
   - pipelines/auth/oidc_client.py (authlib OIDC client)
   - JWKS endpoint URL을 Sprint 1 cache에 wiring
        ↓
[Sprint 3 AUTH: REQ-AUTH-003-E1 gRPC Interceptor]
   - internal/server/grpc_interceptor.go (UnaryServerInterceptor + StreamServerInterceptor)
   - internal/auth/context.go (auth.WithUser / UserFromContext 헬퍼)
   - internal/server/grpc_server.go 수정 (hardcoded "cli-anonymous" 제거 → auth.UserFromContext(ctx).UID)
   - internal/audit/recorder.go 수정 (resolveUserID 확장 — authEnabled=true + ctx user_id 우선, 아니면 cli-anonymous 폴백)
   - /grpc.health.v1.Health/Check bypass
        ↓
[Sprint 4 AUTH: REQ-AUTH-003-E2/E3 REST middleware + FastAPI Depends + Celery envelope (CROSS-SPEC)]
   - internal/server/rest_middleware.go (http.Handler chain, /health bypass, /api/v1/* enforce)
   - internal/server/rest_handler.go 수정 (Mux에 middleware chain 적용, context user_id 사용)
   - cmd/server/main.go 수정 (auth wiring)
   - pipelines/auth/dependencies.py (FastAPI Depends(verify_token))
   - pipelines/auth/context.py (Celery task_prerun signal handler — envelope headers.user_id → contextvar)
   - pipelines/main.py 수정 (Depends(verify_token) 등록, /health + /docs 제외)
   - pipelines/workers/{ingestion,generation,simulation}_worker.py 수정 (contextvar user_id로 audit_event INSERT)
   - **[CROSS-SPEC] internal/scheduler/celery_envelope.go 수정 — headers.user_id 필드 추가**
   - **[CROSS-SPEC] testdata/celery_envelope_v2.json golden file 재생성 (user_id 포함 변형)**
   - **[CROSS-SPEC] SPEC-AX-CTRL-001 Sprint 6 dispatcher tests 15개 회귀 가드**
        ↓
[Sprint 5 AUTH: REQ-AUTH-004 RBAC 3-role Foundation]
   - internal/auth/rbac.go (scope 파싱 regex `^iroum-ax:(admin|analyst|viewer)$`, union permission, AuthorizeRBAC(ctx, methodOrPath))
   - internal/audit/audit.go 수정 (4 신규 Action 상수: ActionAuthRejected, ActionAuthForbidden, ActionAuthLogout, ActionAuthRefreshReuseDetected)
   - internal/server/grpc_interceptor.go 확장 (RBAC hook 추가)
   - internal/server/rest_middleware.go 확장 (RBAC hook 추가)
   - pipelines/auth/dependencies.py 확장 (require_role("analyst") factory dependency)
        ↓
[Sprint 6 AUTH: REQ-AUTH-005 Refresh + Logout]
   - internal/auth/refresh_family.go (Redis key auth:refresh_family:<family_id>, Lua script atomic check-and-blacklist)
   - internal/server/rest_handler.go 확장 (POST /api/v1/auth/logout, POST /api/v1/auth/refresh)
   - blacklist 양 토큰 진입 + family invalidation 로직
        ↓
[Sprint 7 AUTH: E2E Integration + Deployment]
   - docker-compose.yml에 Keycloak 24.x 컨테이너 + dev realm.json mount
   - deployments/helm/keycloak/templates/keycloak-statefulset.yaml (단일 인스턴스 PoC)
   - deployments/helm/keycloak/templates/keycloak-realm-configmap.yaml (admin/analyst/viewer 클라이언트, scope 정의)
   - tests/integration/test_auth_e2e.py (testcontainers Keycloak 24.x + Postgres + Redis full chain)
   - tests/integration/test_auth_backward_compat.py FULL 실행 (AUTH_ENABLED=false 시 SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C unchanged)
```

### 1.2 시퀀싱 정당화 (vs plan.md §2 ordering)

plan.md §2는 `S0 Scaffolding → S1 Verifier → S2 OIDC → S3 Middleware → S4 RBAC → S5 Refresh/Logout → S6 E2E` 순으로 명시했다. 본 strategy는 다음 4가지 근거로 **시퀀스를 재배치**한다 — 단, plan.md의 큰 의존 골격은 그대로 보존된다(verifier가 OIDC와 무관하게 시작 가능, middleware가 verifier+OIDC 모두 GREEN 이후, RBAC이 middleware 뒤, refresh/logout이 RBAC 뒤).

| 변경 | 근거 |
|------|------|
| **Pre-Sprint 0 환경 검증 분리** | SPEC-AX-CTRL-001 strategy.md가 따랐던 동일 패턴. Keycloak 24.x docker image pull, testcontainers-go v0.32+ 가용성, Redis miniredis 호환성, PyJWT[cryptography] 빌드(OpenSSL 의존) 사전 확인. plan.md는 S0를 직접 Scaffolding으로 시작하므로 환경 차단 시 sprint 중간에 발견될 위험. |
| **plan.md S3(Middleware Integration)을 S3 gRPC + S4 REST/FastAPI/Celery로 분할** | (a) gRPC interceptor는 bufconn 단위 테스트 self-contained, REST middleware는 httptest 별도. (b) **Celery envelope user_id 필드 추가는 SPEC-AX-CTRL-001 cross-SPEC artifact 재생성을 수반** (lessons #5 적용 — celery_envelope_v2.json golden file 재생성). 단일 sprint로 묶을 경우 evaluator-active scoring에서 functionality + cross-spec consistency must-pass 두 영역이 동일 sprint에 몰림. (c) plan.md L132 노트도 "golden file regeneration in S6, not S0"을 명시 — 본 strategy는 이를 한 단계 더 명료화하여 S4에 배치. |
| **RBAC(S5)와 Refresh/Logout(S6)을 분리 유지** | plan.md ordering 그대로 보존. RBAC은 미들웨어 hook 확장이고, Refresh/Logout은 새 endpoint + Redis Lua script로 의존성과 risk profile이 다름. |
| **E2E(S7)에 backward_compat full 실행 포함** | plan.md S6는 E2E를 "AC-AUTH-E2E-1/2 통과"로 정의했으나, 본 strategy는 R-AUTH-007 (백워드 호환 regression — Python 177 + Go 95 = 272 tests) mitigation을 **S0 baseline 작성 + S3 종료 시 회귀 검증 + S7 full chain 검증**의 3중 가드로 확장. lessons #4 적용 — RED stub에 `ErrNotImplemented` assert 사용 시 GREEN 직후 제거를 명시. |

**Re-evaluation**: 만약 백워드 호환이 가장 큰 risk(R-AUTH-007)이고 SPEC-AX-001 / SPEC-AX-CTRL-001 모든 AC를 깨지 않아야 한다면 **S0에서 baseline을 미리 작성**하는 것이 합리적이다. 본 strategy는 이를 채택 — S0 종료 시 test_auth_backward_compat.py가 AUTH_ENABLED=false 시 unchanged 결과 반환 baseline을 잡고, S3 종료 시 회귀 가드 통과, S7에서 full E2E 검증. 즉 baseline → 중간 회귀 가드 → 최종 회귀 가드의 3-stage 가드.

### 1.3 병렬화 기회

- **Sprint 1(JWT Validator) Go와 Python 평행 구현**: Go verifier.go + Python verifier.py는 동일 JWT 표준을 따르므로 RED 단계에서 두 언어 평행 작성 가능. 단, **단일 manager-tdd 환경에서는 순차** 권장(token 부담). team mode 활성 시 implementer 2명(isolation: worktree)으로 분할 가능.
- **Sprint 2(OIDC Discovery)** Go와 Python 평행 구현 동일.
- **Sprint 4(REST + FastAPI + Celery envelope)**: Go REST middleware + Python FastAPI Depends + Celery context.py + scheduler/celery_envelope.go 수정은 4개 독립 파일이므로 team mode 활성 시 implementer 3-4명 병렬 가능. **단, golden file 재생성(testdata/celery_envelope_v2.json)은 scheduler/celery_envelope.go GREEN 직후 단일 시점에 수행** (race condition 방지).
- 순차 강제 구간: S1 → S3 (verifier가 interceptor 의존), S2 → S3 (OIDC client가 verifier에 wiring), S3 → S4 (REST middleware도 verifier+context 공유), S4 → S5 (RBAC이 미들웨어 hook 확장), S5 → S6 (Refresh/Logout이 RBAC 뒤), S6 → S7 (E2E).

---

## 2. Foundation Setup (Sprint 0)

### 2.1 Go 의존성 추가 (`apps/control-plane/go.mod`)

| 라이브러리 | 버전 | 용도 | 정당화 |
|----------|------|------|-------|
| `github.com/golang-jwt/jwt/v5` | v5.2.x | JWT parse + 서명 검증 | research.md §1.2 결정. v5 stable + breaking change 적음. `jwt.WithAlgorithms("RS256", "EdDSA", "ES256")` 강제로 Algorithm Confusion Attack 자동 방어. `jwt.WithLeeway(30 * time.Second)` clock skew 자연 지원. |
| `github.com/coreos/go-oidc/v3` | v3.10.x | OIDC Discovery + Provider metadata cache | research.md §1.3 결정. `oidc.NewProvider(ctx, issuerURL)` 한 줄로 Discovery + JWKS 자동 처리. 그러나 **RemoteKeySet의 Cache-Control 의존을 1h hard floor로 superset** (jwks_cache.go가 wrapping). |
| `github.com/MicahParks/keyfunc/v3` | v3.x | JWKS handling (golang-jwt와 결합용 KeyFunc) | research.md §1.3 보충. golang-jwt v5와 coreos/go-oidc 사이의 KeyFunc 어댑터로 활용 — kid 기반 RSA/EC public key 조회를 표준화. |

### 2.2 Python 의존성 추가 (`pipelines/pyproject.toml`)

| 라이브러리 | 버전 | 용도 | 정당화 |
|----------|------|------|-------|
| `PyJWT[cryptography]` | ^2.8.0 | Python JWT parse + 서명 검증 + `PyJWKClient` 내장 | research.md §1.4 + §5.2 결정. python-jose보다 활성도 + 보안 패치 빈도 우수. `PyJWKClient` 내장으로 별도 JWKS client 불필요. `algorithms=["RS256", "EdDSA", "ES256"]` 인자 강제. |
| `authlib` | ^1.3.0 | OIDC client (Discovery + token endpoint client) | Python 측 OIDC Discovery는 표준 라이브러리 부재 → authlib가 사실상 표준. PyJWT는 JWT만 처리하므로 OIDC discovery를 authlib로 보완. |
| `httpx` | ^0.27.0 | OIDC discovery HTTP client (authlib backing) | 이미 ingestion_worker에서 사용 중일 수 있으나 명시적 의존 표시. |

> **결정 — python-jose vs PyJWT**: research.md §5.2가 명확히 PyJWT 채택을 결정했다. spec.md §6 의존성 섹션이 `python-jose[cryptography] ^3.3.0 또는 PyJWT ^2.8.0`로 두 옵션을 병기하나, research.md decision이 우선 → **PyJWT 2.8 채택**. 본 strategy는 이 결정을 단일 source로 확정.

### 2.3 신규 디렉토리 구조

```
iroum-ax/
├── pkg/auth/                              # 신규 (Go-Python 공유)
│   ├── claims.go                          # JWT Claims 구조체 (Go)
│   └── claims.py                          # JWT Claims 구조체 (Python Pydantic)
├── apps/control-plane/
│   └── internal/
│       └── auth/                          # 신규 디렉토리 (Sprint 0 stub)
│           ├── verifier.go                # S1 GREEN
│           ├── jwks_cache.go              # S1 GREEN
│           ├── oidc_client.go             # S2 GREEN
│           ├── blacklist.go               # S1 GREEN
│           ├── rbac.go                    # S5 GREEN
│           ├── context.go                 # S3 GREEN (WithUser/UserFromContext)
│           ├── refresh_family.go          # S6 GREEN
│           └── errors.go                  # S0 GREEN (6 신규 에러 정의)
├── pipelines/
│   └── auth/                              # 신규 디렉토리 (Sprint 0 stub)
│       ├── __init__.py
│       ├── verifier.py                    # S1 GREEN
│       ├── oidc_client.py                 # S2 GREEN
│       ├── dependencies.py                # S4 GREEN + S5 확장
│       ├── blacklist.py                   # S1 GREEN
│       ├── context.py                     # S4 GREEN (Celery task_prerun signal handler)
│       └── errors.py                      # S0 GREEN
└── deployments/
    └── helm/
        └── keycloak/                      # 신규 디렉토리 (Sprint 0 stub)
            ├── Chart.yaml
            ├── values.yaml
            ├── values-dev.yaml            # auth.enabled: false
            └── templates/
                ├── keycloak-statefulset.yaml    # S7 GREEN
                ├── keycloak-realm-configmap.yaml # S7 GREEN
                └── secret.yaml                   # S7 GREEN
```

### 2.4 Config 필드 4개 추가

**Go (`internal/config/config.go`)**:
- `OIDCIssuerURL string` — Keycloak realm issuer URL (예: `http://keycloak.iroum-ax.svc.cluster.local:8080/realms/iroum-ax`)
- `OIDCAudience string` — JWT `aud` 클레임 기대값 (예: `iroum-ax-control-plane`)
- `OIDCJWKSCacheTTL time.Duration` — JWKS cache hard TTL (default 1h)
- `RBACScopePrefix string` — RBAC scope 명명 prefix (default `iroum-ax:`)
- `AuthEnabled bool` — 마스터 스위치 (default `false` for backward compat)

**Python (`pipelines/config/settings.py`)**:
- `OIDC_ISSUER_URL: str` (Pydantic Settings)
- `OIDC_AUDIENCE: str` (default `iroum-ax-pipelines`)
- `OIDC_JWKS_CACHE_TTL: int` (seconds, default 3600)
- `RBAC_SCOPE_PREFIX: str` (default `iroum-ax:`)
- `AUTH_ENABLED: bool` (default `False`)

### 2.5 AuthEnabled=false 분기 wiring (Sprint 0 invariant)

S0 GREEN의 핵심 산출물은 **모든 신규 미들웨어가 AuthEnabled=false 시 no-op 동작**이다. 이는 R-AUTH-007 mitigation의 1단계.

- gRPC interceptor stub: `if !cfg.AuthEnabled { return handler(ctx, req) }` (전체 인증 우회)
- REST middleware stub: 동일 패턴
- FastAPI Depends(verify_token) stub: `if not settings.AUTH_ENABLED: return User(uid="cli-anonymous", scopes=[])` 폴백
- audit/recorder.go resolveUserID: `if !authEnabled { return "cli-anonymous" }` 폴백 우선

S0 GREEN 직후 `test_auth_backward_compat.py`가 AUTH_ENABLED=false 시 SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C와 byte-identical 결과 반환 검증.

### 2.6 RED stub anti-pattern 회피 (lessons #4)

`internal/auth/` 8개 stub 파일에 `@MX:TODO Sprint 1+` 태그만 부착하고, 함수 body는 다음 두 패턴 중 하나:

- **권장 (자연 fail)**: 함수가 미존재 — 호출자가 컴파일 에러 시 `ModuleNotFoundError` / `package missing function` 자연 발생. RED 단계 진입 시 implementer가 컴파일 시도하면 즉시 인지.
- **차선 (assert ErrNotImplemented)**: 함수가 존재하되 `return errors.ErrNotImplemented` 즉시 반환. **단, GREEN 직후 ErrNotImplemented 분기를 제거하고 실제 구현으로 교체할 의무를 plan에 명시** (lessons #4 — stub-assert anti-pattern 회피).

본 SPEC은 **권장 패턴(자연 fail)** 사용. errors.go에 ErrNotImplemented 정의 없음.

---

## 3. Sprint Contracts (per-sprint thorough harness)

`design.yaml` `sprint_contract` 활성화 (harness=thorough, lessons #7 — evaluator-active -0.05~-0.10 보수 적용). 각 sprint 시작 시 evaluator-active가 Sprint Contract 생성. **must-pass criteria 개별 통과 필요** (strict mode 활성).

| Sprint | Priority Dimension | Pass Threshold (must-pass + nice-to-have) | Reference AC |
|--------|------------------|------------------------------------------|-------------|
| Pre-S0 | Consistency (env baseline) | ≥ 0.75 (Keycloak image pull, testcontainers, PyJWT build) | — |
| S0 Foundation | Consistency (regression baseline) | ≥ 0.75 (test_auth_backward_compat baseline 통과) | AC-AUTH-UBI-001-C |
| S1 JWT Validator | **Security** (서명 검증, alg allow-list, iss SF-1, alg cross-check SF-2) | **≥ 0.80** (security-critical, lessons #7) | AC-AUTH-001-1~10, AC-AUTH-001-iss-validation, AC-AUTH-001-alg-cross-check, AC-AUTH-001-Performance |
| S2 OIDC Discovery | Functionality (discovery 정확성, fail-fast) | ≥ 0.75 | AC-AUTH-002-1~3 |
| S3 gRPC Interceptor | **Security** + Functionality (middleware integration + bypass) | ≥ 0.80 | AC-AUTH-003-1, AC-AUTH-003-2, AC-AUTH-003-6, AC-AUTH-UBI-001-D |
| S4 REST + FastAPI + Celery | **Functionality** + **Consistency** (cross-SPEC celery envelope) | ≥ 0.80 (cross-SPEC artifact 정합성 critical) | AC-AUTH-003-3, AC-AUTH-003-4, AC-AUTH-003-5, AC-AUTH-UBI-001-A/B |
| S5 RBAC | **Security** (3-role enforcement, scope union, AUTH_FORBIDDEN audit) | **≥ 0.80** (security-critical) | AC-AUTH-004-1~6 |
| S6 Refresh/Logout | **Security** (token revocation, family invalidation, Lua atomicity) | **≥ 0.80** (OAuth 2.0 BCP 권고, lessons #7) | AC-AUTH-005-1~4 |
| S7 E2E | Completeness (full chain) | ≥ 0.75 | AC-AUTH-E2E-1, AC-AUTH-E2E-2 |

**lessons #7 — evaluator-active 보수 보정**: 각 sprint pass threshold는 evaluator-active가 -0.05~-0.10 보수 적용 후에도 통과 가능한 수준으로 설정. S1/S5/S6는 security-critical이므로 must-pass Security ≥ 0.6 + 0.75 기대치를 보장.

### 3.1 Sprint Contract 핵심 항목 (S1 예시)

S1 Sprint Contract (evaluator-active가 RED 진입 전 작성):
- **Acceptance checklist (concrete)**:
  - AC-AUTH-001-1 happy path (RS256 valid token 수용)
  - AC-AUTH-001-2 expired token rejection (ErrTokenExpired)
  - AC-AUTH-001-3 future iat rejection (clock skew 초과)
  - AC-AUTH-001-4 wrong audience rejection
  - AC-AUTH-001-5 HS256 algorithm rejection (allow-list)
  - AC-AUTH-001-6 alg=none rejection
  - AC-AUTH-001-7 clock skew within 30s acceptance
  - AC-AUTH-001-8 JWKS cache TTL refresh
  - AC-AUTH-001-9 JWKS unavailable + stale-while-revalidate
  - AC-AUTH-001-10 blacklisted token rejection
  - **AC-AUTH-001-iss-validation (SF-1)** — per-token issuer 검증
  - **AC-AUTH-001-alg-cross-check (SF-2)** — JWKS kty vs token alg 일관성
- **Priority dimension**: Security (서명 검증 + algorithm safety)
- **Test scenarios**: testify table tests (Go) + pytest parametrize (Python) + miniredis (blacklist) + httptest (JWKS endpoint mock)
- **Pass conditions**: 모든 AC PASS + coverage ≥ 85% (`go test -cover`, `pytest --cov`) + benchmark p99 < 5ms (JWKS cache hit) + zero golangci-lint warning + zero ruff error

---

## 4. Risk-Driven Decisions

본 strategy는 SPEC research.md §6 7개 risk(R-AUTH-001~007)에 더해 **plan-auditor SF-1/SF-2 차단 보정 후 detection AC가 적절히 매핑되었는지** 확인.

### 4.1 R-AUTH-007 백워드 호환성 regression (TOP RISK — Probability High × Impact Critical)

**Likelihood**: High (resolveUserID 확장이 SPEC-AX-001 / SPEC-AX-CTRL-001 8개 RecordXxx 호출처에 영향).
**Impact**: Critical (Python 177 + Go 95 = 272 tests가 회귀하면 본 SPEC merge 차단).
**Detection**:
- S0: `tests/integration/test_auth_backward_compat.py` baseline 작성 (AUTH_ENABLED=false 시 AC-CTRL-UBI-002-A/B/C 결과 capture)
- S3 종료: 동일 테스트가 unchanged 결과 반환 검증
- S7 E2E: AC-AUTH-E2E-2 (AUTH_ENABLED=false 시 모든 audit_logs.user_id=`cli-anonymous` byte-identical with SPEC-AX-CTRL-001 AC-CTRL-E2E-1)
**Mitigation**:
- `resolveUserID(ctx, providedUserID, authEnabled bool) string` 시그니처 명시 — authEnabled=false 시 무조건 cli-anonymous 폴백
- CI에서 본 SPEC 진행 중에도 SPEC-AX-CTRL-001 전체 테스트 통과 유지 의무
- audit/recorder.go의 resolveUserID는 `@MX:ANCHOR (fan_in 8+)` 부여 — 모든 RecordXxx의 invariant 단일 진입점
**Residual Risk**: Low (자동화 보장 + 3-stage 가드)

### 4.2 R-AUTH-006 Algorithm Confusion Attack (Probability Low × Impact Critical)

**Likelihood**: Low (라이브러리 자체는 잘 처리, 미스컨피그 시).
**Impact**: Critical (forged token으로 인증 우회).
**Detection**:
- AC-AUTH-001-5 (HS256 rejection)
- AC-AUTH-001-6 (alg=none rejection)
- **AC-AUTH-001-alg-cross-check (SF-2 신규)** — JWKS key kty와 token alg cross-check (RSA key + ES256 claim 변형 거부)
**Mitigation**:
- REQ-AUTH-001-U1: allow-list `[RS256, EdDSA, ES256]`만 명시적 허용
- `jwt.WithAlgorithms(...)` 강제 (Go) + `algorithms=[...]` 인자 강제 (Python)
- **SF-2 추가 가드**: JWKS 응답의 kty=RSA → token alg가 RS256/RS384/RS512 중 하나 / kty=EC → ES256/ES384/ES512 중 하나 / kty=OKP → EdDSA 중 하나만 허용 (verifier.go에 cross-check 로직 추가)
**Residual Risk**: Very Low

### 4.3 R-AUTH-001 JWKS Endpoint Unavailability (Probability Medium × Impact High)

**Likelihood**: Medium (Keycloak 단일 인스턴스, 재시작 / 네트워크 partition).
**Impact**: High (JWKS 미가용 시 모든 인증 요청 실패 → 서비스 중단).
**Detection**: AC-AUTH-001-9 (JWKS unavailable 시 stale cached keys 사용).
**Mitigation**:
- 1h hard TTL JWKS cache (REQ-AUTH-001-E2, jwks_cache.go가 coreos/go-oidc RemoteKeySet 위에 wrapping)
- **Stale-while-revalidate**: cache 만료 후 fetch 실패 시 cached keys 계속 사용 (degraded mode, 최대 4h)
- Cache 자체가 비어 있을 때만(startup 직후 + JWKS down): HTTP 503 + `Retry-After: 30` 헤더
- 운영 단계 후속 SPEC: Keycloak HA (3-replica StatefulSet, R-AUTH-005 mitigation과 함께)
**Residual Risk**: Low (PoC 단계 수용)

### 4.4 R-AUTH-004 Refresh Token Family Race Condition (Probability Low × Impact Medium)

**Likelihood**: Low (정상 시나리오 발생 안 함, 공격 시도 시 발생).
**Impact**: Medium (정상 사용자 family invalidation 경험 가능).
**Detection**: AC-AUTH-005-3.
**Mitigation**:
- Redis Lua script `EVAL` 사용 — Get family + Validate jti + Blacklist family를 atomic 수행
- Lua script eval failure 시 fallback: 보수적으로 family 전체 blacklist (false positive 수용 — 사용자 friendly 에러)
- `refresh_family.go`에 `@MX:WARN REASON: Lua script atomic 조건문 복잡도 ≥ 12, fallback 전략 필수` 부착
**Residual Risk**: Low

### 4.5 Clock Skew (R-AUTH-003) + Token Leakage (R-AUTH-002) + Keycloak SPOF (R-AUTH-005)

- R-AUTH-003 Clock Skew: ±30초 허용 (OAuth 2.0 BCP), AC-AUTH-001-7 검증. NTP 권장 docs 후속.
- R-AUTH-002 Token Leakage: `auth/context.go`에 raw token 저장 금지, zap log Authorization 필드 redaction, Python logging filter. CI lint rule (후속) — `Bearer [A-Za-z0-9-_\.]{20,}` 검출 시 PR reject.
- R-AUTH-005 Keycloak SPOF: PoC 단일 인스턴스 수용, 후속 SPEC `SPEC-AX-AUTH-HA-001`로 HA 분리.

---

## 5. Backward Compat Invariant (CRITICAL — REQ-AUTH-UBI-001 WHILE clause)

본 SPEC의 가장 중요한 invariant. R-AUTH-007 mitigation의 핵심.

### 5.1 회귀 가드 범위

- **SPEC-AX-001**: 7개 REQ (UBI 데이터주권, UBI cli-anonymous 폴백 포함). Python 177 테스트.
- **SPEC-AX-CTRL-001**: 5개 REQ (UBI workflow atomicity + cli-anonymous 폴백 포함). Go 95 테스트.
- **합계**: Python 177 + Go 95 = **272 tests** 모두 unchanged 통과 필요.

### 5.2 회귀 가드 자동화

1. **S0 baseline 작성**: `tests/integration/test_auth_backward_compat.py`
   - AUTH_ENABLED=false 로 control-plane + pipelines 부팅
   - SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A (Authorization 헤더 없이 POST /api/v1/workflows → 201 + workflows.user_id=cli-anonymous) 실행
   - SPEC-AX-CTRL-001 AC-CTRL-UBI-002-B (state transition audit에서 cli-anonymous 보존) 실행
   - SPEC-AX-CTRL-001 AC-CTRL-UBI-002-C (audit_logs.user_id=cli-anonymous) 실행
   - 응답 byte-for-byte capture → golden snapshot

2. **S3 종료 시 회귀 검증**: 동일 테스트 재실행 → byte-identical 결과 확인

3. **S7 E2E full chain**: AC-AUTH-E2E-2 (docker-compose 환경에서 AUTH_ENABLED=false 시 SPEC-AX-CTRL-001 AC-CTRL-E2E-1 결과 byte-identical)

### 5.3 cli-anonymous 폴백 유지 (REQ-CTRL-UBI-002-C, ADR 0007 참조)

- audit/recorder.go의 `resolveUserID(ctx, providedUserID, authEnabled bool) string`:
  - `if !authEnabled { return "cli-anonymous" }` 1차 폴백 (기존 동작 보존)
  - `if user := auth.UserFromContext(ctx); user != nil { return user.UID }` 2차 (인증 활성 + ctx user_id 존재)
  - `if providedUserID != "" { return providedUserID }` 3차 (호출자 명시)
  - `return "cli-anonymous"` 최종 폴백 (인증 활성이나 ctx user_id 부재 — 미들웨어 우회 버그 detection)

### 5.4 신규 인증 코드 AuthEnabled 조건부 진입

- gRPC interceptor: `if !cfg.AuthEnabled { return handler(ctx, req) }` (early return, RBAC도 우회)
- REST middleware: 동일
- FastAPI Depends(verify_token): `if not settings.AUTH_ENABLED: return User(uid="cli-anonymous", scopes=[])` (sandbox 호환)
- RBAC: `if !cfg.AuthEnabled { return nil }` (RBAC 검사 자체 우회 — 권한 거부 0건)
- Logout/Refresh endpoint: `if !cfg.AuthEnabled { return 404 Not Found }` (endpoint 자체 미제공)

---

## 6. Cross-SPEC Artifact Regen (Sprint 4) — lessons #5 적용

### 6.1 영향받는 SPEC-AX-CTRL-001 산출물

| 산출물 | 변경 사항 | 회귀 가드 |
|------|---------|---------|
| `apps/control-plane/internal/scheduler/celery_envelope.go` | `Envelope.Headers` 구조체에 `UserID string \`json:"user_id,omitempty"\`` 필드 추가 (D1 v0.1.1 정정 경로 — dispatcher.go가 아닌 celery_envelope.go) | Sprint 4 GREEN |
| `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json` | golden file 재생성 — `headers.user_id` 필드 포함 변형 (예: `"user_id": "uuid-alice"` 또는 `"user_id": "cli-anonymous"`) | Python kombu 측 generator 1회 재실행 후 커밋 |
| `apps/control-plane/internal/scheduler/dispatcher_test.go` (15 tests) | 기존 golden file 비교 테스트가 새 user_id 필드 수용하도록 update | Sprint 4 GREEN |
| SPEC-AX-CTRL-001 AC-CTRL-005-1 (Celery envelope schema validation) | Schema가 user_id 필드 추가를 수용하도록 update (선택적 필드 추가는 backward compatible) | SPEC-AX-CTRL-001 maintenance handoff note |

### 6.2 golden file 재생성 절차

1. Python kombu 측 `pipelines/scripts/generate_celery_envelope_fixture.py` (Sprint 4 신규 스크립트) 작성
2. 두 변형 생성:
   - 변형 A (인증 활성): `headers.user_id = "uuid-alice"` (실제 sub 클레임 시뮬)
   - 변형 B (인증 비활성): `headers.user_id = "cli-anonymous"` (백워드 호환)
3. `apps/control-plane/internal/scheduler/testdata/celery_envelope_v2.json` 갱신 + 새 변형 추가
4. dispatcher_test.go가 두 변형 모두 deserialization 가능한지 테스트
5. SPEC-AX-CTRL-001 Sprint 6 기존 15 dispatcher tests 회귀 검증

### 6.3 cross-SPEC handoff

- 본 SPEC Sprint 4 GREEN 직후 SPEC-AX-CTRL-001 AC-CTRL-005-1 회귀 확인
- SPEC-AX-CTRL-001 acceptance.md에 신규 AC `AC-CTRL-005-1b (Celery envelope user_id field)` 추가 권고 (sync phase에서 manager-docs 처리)

---

## 7. LSP / Lint Baseline (TRUST 5 Quality Gates)

| 도구 | 단계 | Threshold |
|------|------|----------|
| `go vet` | S0 GREEN 이후 매 sprint 종료 시 | 0 issues (strict) |
| `golangci-lint` (v1.64) | 동일 | 0 issues (default ruleset + gosec G401/G402 crypto warnings 활성) |
| `ruff check` (Python 3.11) | 동일 | 0 errors |
| `goleak.VerifyNone(t)` | 모든 Go 테스트 | 0 leaked goroutines (jwks_cache 백그라운드 refresh, refresh_family Lua eval 등 검증) |
| Go coverage (`go test -cover`) | S1~S6 종료 시 | ≥ 85% (`internal/auth/` 패키지 신규 — 단순 구조라 도달 가능, lessons #6 측정) |
| Python coverage (`pytest --cov`) | 동일 | ≥ 85% (`pipelines/auth/` 패키지 신규) |
| zap log Authorization redaction | 모든 sprint 종료 시 | 0 instances of `Bearer ` literal in log output (R-AUTH-002 mitigation, DoD §9 마지막 항목) |

LSP quality gates는 `.moai/config/sections/quality.yaml` `lsp_gates.run` 정책 준수: zero errors, zero type errors, zero lint errors.

---

## 8. Open Questions — Sensible Defaults (자동 모드, AskUserQuestion 호출 없음)

본 strategy는 자동 모드로 진입하므로 research.md §9 5개 open question에 sensible default를 적용한다. lessons #2 / lessons #8 적용.

| ID | Question | Default 채택 | 정당화 |
|----|----------|------------|-------|
| **Q1** | 전자정부 표준 인증 사용 강제? | **NO** (Keycloak 진행, 후속 SPEC `SPEC-AX-AUTH-EGOV-001`으로 분리) | 한국 공공 6 제약 중 망분리 = Keycloak self-hosted 강제 (lessons #8). 외부 OAuth 0건 가드. 전자정부 표준은 정부 인증서 + 정부망 통합 절차로 PoC 단계 불가. |
| **Q2** | JWKS cache TTL 정책? | **3600s (1h hard TTL + stale-while-revalidate 최대 4h)** | research.md §1.3 결정. coreos/go-oidc RemoteKeySet의 Cache-Control 의존을 1h hard floor로 superset. Keycloak default Cache-Control이 짧을 수 있어 보수적 설정. |
| **Q3** | Refresh token TTL 정책? | **Access 1h / Refresh 24h** (research.md §9 Q2 option A, OAuth 2.0 BCP 권장). 본 SPEC은 7d access + 30d refresh를 채택하지 않음 — 한국 공공 보안 요구 우선. | OAuth 2.0 BCP RFC 9700 권장. 자동 모드 instruction이 "default 7d access + 30d refresh"를 제안했으나 **lessons #8 한국 공공 6 제약 우선** 적용으로 1h/24h 채택. Keycloak realm 설정에서 운영 단계 조정 가능. |
| **Q4** | RBAC scope 명명 규약? | **`iroum-ax:{role}` (admin/analyst/viewer)** | spec.md §3.5 REQ-AUTH-004-S1 명시. Keycloak realm 설정과 일치 (`@MX:NOTE` 부착, rbac.go). |
| **Q5** | Keycloak HA 도입 시점? | **PoC 단일 인스턴스 수용 (R-AUTH-005 Medium residual)**, 운영 단계 후속 SPEC `SPEC-AX-AUTH-HA-001` 분리 | research.md §6 R-AUTH-005 결정. Keycloak HA는 별도 SPEC 범위. PoC 단계는 단일 인스턴스로 진행하되 JWKS 캐시 + stale-while-revalidate로 부분 가용성 확보. |

> **자동 모드 alignment**: instruction sensible defaults Q3가 "7d/30d"를 제안했으나, 본 strategy는 **한국 공공 보안 요구 우선** (lessons #8) + research.md §9 Q2 option A + OAuth 2.0 BCP 권장에 따라 **1h/24h** 채택. 이는 spec.md §4 비기능 요구사항 표 및 Q2 default와 정합. 운영 단계 KEPCO E&C 요구에 따라 Keycloak realm 설정에서 후 조정.

---

## 9. MX Tag Plan (코드 주석 언어: 한국어 per language.yaml)

본 SPEC 구현 시 추가될 @MX 태그. plan.md §6를 본 strategy가 보완.

### 9.1 @MX:ANCHOR (fan_in ≥ 3, REASON 필수)

| 함수 | 파일 | fan_in 예상 | REASON |
|------|------|----------|-------|
| `VerifyToken(ctx, tokenString) (*Claims, error)` | `auth/verifier.go` | 5+ (gRPC interceptor, REST middleware, logout, refresh, test) | 모든 인증 경로 단일 진입점, SF-1 iss + SF-2 alg cross-check 포함 |
| `auth.UserFromContext(ctx) (*User, bool)` | `auth/context.go` | 10+ (gRPC + REST handlers, audit recorder, RBAC check, scheduler dispatch) | context user 추출 단일 진입점 |
| `AuthorizeRBAC(ctx, methodOrPath) error` | `auth/rbac.go` | 4+ (gRPC interceptor, REST middleware, FastAPI dep) | RBAC 결정 단일 진입점 |
| `resolveUserID(ctx, providedUserID, authEnabled) string` | `audit/recorder.go` | 8+ (8 RecordXxx 메서드) | **백워드 호환 invariant** (SPEC-AX-001 REQ-UBI-003 + SPEC-AX-CTRL-001 REQ-CTRL-UBI-002 보존) |
| `verify_token(authorization: str) -> User` (Python) | `pipelines/auth/dependencies.py` | 5+ (모든 protected endpoint) | Python 측 인증 진입점 |

### 9.2 @MX:WARN (REASON 필수)

| 위치 | 위험 | REASON |
|------|-----|-------|
| `jwks_cache.go` JWKS refresh | TTL refresh 시 sync.RWMutex Upgrade lock pattern, race 가능 | stale-while-revalidate 보장 위해 RWMutex 권장, 첫 요청 race detection 필수 |
| `refresh_family.go` family invalidation | Lua script atomic 조건문 복잡도 ≥ 12 | Redis race condition mitigation, Lua script eval failure 시 fallback 전략 (보수적 family 전체 blacklist) 명시 |
| `rest_middleware.go` token redaction | log line에 Authorization 헤더 출력 검사 필요 | OWASP A09 mitigation, R-AUTH-002 |
| `verifier.go` alg/kty cross-check | SF-2 보정 — RSA key + ES256 claim 변형 거부 | Algorithm Confusion Attack 변형 가드, OWASP JWT cheat sheet |

### 9.3 @MX:NOTE

| 위치 | 컨텍스트 |
|------|-------|
| `verifier.go` clock skew 30s | OAuth 2.0 BCP RFC 9700 권고치 |
| `verifier.go` iss 검증 (SF-1) | RFC 7519 §4.1.1 + OWASP JWT cheat sheet, cross-realm token 재사용 공격 방어 |
| `rbac.go` scope prefix `iroum-ax:` | Keycloak realm 설정과 일치, 변경 시 양쪽 동기 |
| `blacklist.go` Redis key prefix `auth:blacklist:` | Go와 Python이 동일 키 스페이스 공유 (cross-language consistency) |
| `audit/audit.go` 신규 Action 상수 4개 (ActionAuthRejected/Forbidden/Logout/RefreshReuseDetected) | REQ-AUTH-UBI-001 / REQ-AUTH-004 / REQ-AUTH-005 audit enumeration 확장 |
| `scheduler/celery_envelope.go` headers.user_id | Sprint 4 cross-SPEC 보정, SPEC-AX-CTRL-001 AC-CTRL-005-1과 정합 |

### 9.4 @MX:TODO

- S0 stub 파일 전체에 `@MX:TODO Sprint S1~S7 비즈니스 로직 구현` 표시
- 각 sprint GREEN 직후 해당 TODO 제거
- S7 종료 시 본 SPEC 관련 @MX:TODO 0건

---

## 10. Phase 2 Handoff Checklist

manager-tdd가 Phase 2 RED 진입 시 다음 항목을 확인:

- [x] 8개 Sprint 시퀀스 + DAG (Pre-S0 ~ S7)
- [x] Foundation deps 결정 (Go 3개: golang-jwt/jwt/v5, coreos/go-oidc/v3, MicahParks/keyfunc/v3 / Python 2개: PyJWT[cryptography], authlib)
- [x] 신규 디렉토리 구조 명시 (pkg/auth/, internal/auth/, pipelines/auth/, deployments/helm/keycloak/)
- [x] Config 필드 4개 추가 (OIDCIssuerURL, OIDCAudience, OIDCJWKSCacheTTL, RBACScopePrefix + AuthEnabled 마스터 스위치)
- [x] Sprint Contract per-sprint priority dimension + pass threshold (S1/S5/S6 ≥ 0.80, 나머지 ≥ 0.75)
- [x] Top 7 risks (R-AUTH-001~007) + detection AC mapping
- [x] Backward compat 3-stage 가드 (S0 baseline + S3 회귀 + S7 E2E)
- [x] Cross-SPEC artifact regen (Sprint 4 celery_envelope.go + golden file)
- [x] LSP / lint baseline (go vet 0, golangci-lint 0, ruff 0, coverage ≥ 85%)
- [x] 5 open question sensible defaults 적용 (자동 모드)
- [x] MX Tag plan (5 ANCHOR + 4 WARN + 6 NOTE + S0 stub TODO)
- [x] RED stub anti-pattern 회피 (자연 fail 권장, ErrNotImplemented 미사용)
- [x] No time estimates (priority labels만 사용)

**Phase 2 RED entry**: **YES** — 모든 결정 사항 sensible default 적용 완료, AskUserQuestion 호출 없이 진행 가능.

---

## 11. Return Summary (≤300 words)

**파일 경로**: `.moai/plans/enumerated-plotting-manatee-agent-a2efa36b7dcc5dcaf.md` (plan mode redirect — orchestrator가 plan mode 해제 후 `.moai/specs/SPEC-AX-AUTH-001/strategy.md`로 이동/재생성 필요, lessons #3 적용)

**Sprint 총수**: **8** (Pre-Sprint 0 환경 검증 + S0 Foundation + S1 JWT Validator/SF-1+SF-2 + S2 OIDC Discovery + S3 gRPC Interceptor + S4 REST + FastAPI + Celery envelope (cross-SPEC) + S5 RBAC + S6 Refresh/Logout + S7 E2E)

**Critical path REQ ordering**: Foundation → REQ-AUTH-001 (JWT + iss + alg cross-check) → REQ-AUTH-002 (OIDC Discovery) → REQ-AUTH-003-E1 (gRPC) → REQ-AUTH-003-E2/E3 + Celery envelope (cross-SPEC) → REQ-AUTH-004 (RBAC) → REQ-AUTH-005 (Refresh/Logout) → E2E

**Top 3 risks + mitigation**:
1. **R-AUTH-007 백워드 호환 regression** (Python 177 + Go 95 = 272 tests) → S0 baseline + S3 회귀 + S7 E2E 3-stage 가드, resolveUserID `@MX:ANCHOR`
2. **R-AUTH-006 Algorithm Confusion Attack** → allow-list `[RS256, EdDSA, ES256]` + SF-2 alg/kty cross-check
3. **R-AUTH-001 JWKS Endpoint Unavailability** → 1h hard TTL + stale-while-revalidate (최대 4h), HTTP 503 + Retry-After 30

**Foundation deps**: Go **3개** (golang-jwt/jwt/v5 v5.2, coreos/go-oidc/v3 v3.10, MicahParks/keyfunc/v3 v3.x), Python **2개** (PyJWT[cryptography] ^2.8, authlib ^1.3) — research.md §5.2 PyJWT 채택 결정 반영

**Cross-SPEC artifact regen (Sprint 4)**:
- `internal/scheduler/celery_envelope.go` Envelope.Headers.UserID 필드 추가 (D1 v0.1.1 정정)
- `testdata/celery_envelope_v2.json` golden file 재생성 (2 변형: 인증 활성 user_id=uuid / 비활성 user_id=cli-anonymous)
- SPEC-AX-CTRL-001 Sprint 6 dispatcher tests 15개 회귀 가드 + AC-CTRL-005-1 schema update

**Ready for Sprint 0 entry**: **YES**
